// Suite test-writer pour le pilote Hue (contracts/hue-bridge.md, #206),
// contre l'implémentation réelle de driver.go (package hue, white-box).
//
// Priorités reprises de la tâche #206/#207 : ampoule renommée ⇒ introuvable
// et jamais de repli · deux homonymes ⇒ refus · pont éteint ⇒ unreachable ·
// clé révoquée ⇒ refused, jamais fondus en une « erreur » · n'écrire que ce
// qui change (§5.3) · une ampoule injoignable n'abat pas les autres (§5.5) ·
// une seule ligne de log par changement d'état, jamais par échec.
package hue

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"buzzcontrol/internal/lighting"
)

// ---------------------------------------------------------------------------
// Faux pont — httptest.NewServer, comme demandé par la tâche ("faux pont
// httptest.NewServer"). Reprend le style du spike (map dynamique modifiable
// pendant le test pour simuler un renommage) plus un compteur de requêtes.
// ---------------------------------------------------------------------------

type fakeLight struct {
	name      string
	on        bool
	bri       int
	xy        [2]float64
	reachable bool
	// putErr, si non nil, fait échouer le PUT suivant sur cette ampoule avec
	// cette hueError v1 (ex: {201, "device is off"}), puis se réinitialise.
	putErr *hueError
}

type fakeBridge struct {
	mu        sync.Mutex
	bridgeID  string
	lights    map[string]*fakeLight // id -> light
	hits      []string              // "METHOD /path", dans l'ordre
	keyWanted string                // "" = accepte toute clé
	srv       *httptest.Server
}

func newFakeBridge(t *testing.T, bridgeID string) *fakeBridge {
	t.Helper()
	fb := &fakeBridge{bridgeID: bridgeID, lights: map[string]*fakeLight{}}
	fb.srv = httptest.NewServer(http.HandlerFunc(fb.handle))
	t.Cleanup(fb.srv.Close)
	return fb
}

func (fb *fakeBridge) addLight(id, name string) *fakeLight {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	l := &fakeLight{name: name, reachable: true}
	fb.lights[id] = l
	return l
}

func (fb *fakeBridge) rename(id, newName string) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	fb.lights[id].name = newName
}

func (fb *fakeBridge) hitCount() int {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	return len(fb.hits)
}

func (fb *fakeBridge) writeCount() int {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	n := 0
	for _, h := range fb.hits {
		if len(h) >= 3 && h[:3] == "PUT" {
			n++
		}
	}
	return n
}

func (fb *fakeBridge) handle(w http.ResponseWriter, r *http.Request) {
	fb.mu.Lock()
	fb.hits = append(fb.hits, r.Method+" "+r.URL.Path)
	fb.mu.Unlock()

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/"+fb.keyOr()+"/lights":
		fb.mu.Lock()
		out := map[string]any{}
		for id, l := range fb.lights {
			out[id] = map[string]any{"name": l.name, "type": "Extended color light", "modelid": "LCT015",
				"state": map[string]any{"on": l.on, "bri": l.bri, "xy": []float64{l.xy[0], l.xy[1]}, "reachable": l.reachable}}
		}
		fb.mu.Unlock()
		_ = json.NewEncoder(w).Encode(out)
	case r.Method == http.MethodGet && r.URL.Path == "/api/"+fb.keyOr()+"/config":
		_ = json.NewEncoder(w).Encode(map[string]any{"bridgeid": fb.bridgeID, "name": "BuzzMaster Bridge", "modelid": "BSB002", "apiversion": "1.67.0"})
	case r.Method == http.MethodPut && len(r.URL.Path) > len("/api/"+fb.keyOr()+"/lights/"):
		fb.handlePut(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`[{"error":{"type":3,"address":"","description":"resource not found"}}]`))
	}
}

func (fb *fakeBridge) keyOr() string { return "u" } // ce faux pont accepte toujours la clé "u"

func (fb *fakeBridge) handlePut(w http.ResponseWriter, r *http.Request) {
	// /api/u/lights/<id>/state
	path := r.URL.Path
	prefix := "/api/" + fb.keyOr() + "/lights/"
	id := path[len(prefix):]
	if len(id) > len("/state") {
		id = id[:len(id)-len("/state")]
	}
	fb.mu.Lock()
	l, ok := fb.lights[id]
	fb.mu.Unlock()
	if !ok {
		_, _ = w.Write([]byte(`[{"error":{"type":3,"address":"","description":"resource not found"}}]`))
		return
	}
	if l.putErr != nil {
		e := *l.putErr
		l.putErr = nil
		b, _ := json.Marshal([]map[string]any{{"error": map[string]any{"type": e.Type, "address": e.Address, "description": e.Description}}})
		_, _ = w.Write(b)
		return
	}
	var body map[string]json.RawMessage
	_ = json.NewDecoder(r.Body).Decode(&body)
	fb.mu.Lock()
	if raw, ok := body["on"]; ok {
		var v bool
		_ = json.Unmarshal(raw, &v)
		l.on = v
	}
	if raw, ok := body["bri"]; ok {
		var v int
		_ = json.Unmarshal(raw, &v)
		l.bri = v
	}
	if raw, ok := body["xy"]; ok {
		var v [2]float64
		_ = json.Unmarshal(raw, &v)
		l.xy = v
	}
	fb.mu.Unlock()
	_, _ = w.Write([]byte(`[{"success":{"/lights/` + id + `/state/on":true}}]`))
}

// authError makes the fake bridge answer "unauthorized" (type 1) to every
// request — simulates a revoked key.
func (fb *fakeBridge) authError() {
	fb.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fb.mu.Lock()
		fb.hits = append(fb.hits, r.Method+" "+r.URL.Path)
		fb.mu.Unlock()
		_, _ = w.Write([]byte(`[{"error":{"type":1,"address":"","description":"unauthorized user"}}]`))
	})
}

func newTestDriver(t *testing.T, fb *fakeBridge, lights []LightSpec, mutate func(*Config)) *Driver {
	t.Helper()
	cfg := Config{
		BridgeIP: fb.srv.URL, // scheme://host — bridgeBase accepts it as-is
		BridgeID: fb.bridgeID,
		APIKey:   "u",
		Lights:   lights,
		Timeout:  2 * time.Second,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New() a échoué : %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func generalState(color [3]int, intensity int) lighting.State {
	return lighting.State{Zones: []lighting.ZoneState{{Zone: lighting.ZoneGeneral, Color: color, Intensity: intensity}}}
}

// noRediscover stubs out FindBridge so a test whose fake bridge shares a
// non-empty BridgeID doesn't fall through to a REAL mDNS/SSDP discovery
// (Discover/FindByID hit the actual network, ~3 s per attempt) when the
// fake bridge becomes unreachable. Every "bridge off"/"backoff" test below
// needs this: ensureResolved's own retry-once-by-id path would otherwise
// turn a fast, local failure into a multi-second one.
func noRediscover(c *Config) {
	c.FindBridge = func(ctx context.Context, id string, timeout time.Duration) (Bridge, bool, error) {
		return Bridge{}, false, nil
	}
}

// ---------------------------------------------------------------------------
// Résolution par nom — jamais de repli, jamais de choix arbitraire (§4.2)
// ---------------------------------------------------------------------------

func TestDriver_RenamedLight_NeverFoundNeverFallsBack(t *testing.T) {
	fb := newFakeBridge(t, "001788fffea0591e")
	fb.addLight("8", "Salle gauche")
	fb.addLight("9", "Salle droite") // pourrait être confondue à tort avec la cible

	d := newTestDriver(t, fb, []LightSpec{{Name: "BuzzHue1", Role: RoleGeneral}}, nil)
	ctx := context.Background()

	if err := d.Apply(ctx, generalState([3]int{255, 0, 0}, 200)); err != nil {
		t.Fatalf("Apply a échoué : %v", err)
	}
	report := d.Status()
	if len(report.Lights) != 1 || report.Lights[0].Resolved {
		t.Fatalf("BuzzHue1 ne doit PAS être résolue (absente du pont) : %+v", report.Lights)
	}
	if report.Lights[0].LastError != "not found" {
		t.Errorf("erreur attendue \"not found\", got %q", report.Lights[0].LastError)
	}
	// Aucune écriture n'a dû partir — ni sur "Salle gauche" ni sur "Salle droite".
	if fb.writeCount() != 0 {
		t.Errorf("aucune écriture ne doit partir quand la cible est introuvable, got %d", fb.writeCount())
	}

	// Renommer une ampoule EXISTANTE en "BuzzHue1" après coup doit la faire
	// résoudre — mais seulement à ce moment, jamais par supposition avant.
	// La ré-vérification a lieu à la résolution et PÉRIODIQUEMENT (contract
	// §4.2), jamais à chaque écriture : RefreshEvery vaut 5 min par défaut,
	// donc un simple second Apply immédiat ne relit pas l'inventaire — on
	// force ici le rafraîchissement explicite (le geste "Actualiser la
	// liste" de la maquette #207, RefreshInventory) plutôt que d'attendre.
	fb.rename("8", "BuzzHue1")
	if err := d.RefreshInventory(ctx); err != nil {
		t.Fatalf("RefreshInventory après renommage a échoué : %v", err)
	}
	if err := d.Apply(ctx, generalState([3]int{255, 0, 0}, 200)); err != nil {
		t.Fatalf("Apply après renommage a échoué : %v", err)
	}
	report = d.Status()
	if !report.Lights[0].Resolved || report.Lights[0].ID != "8" {
		t.Fatalf("BuzzHue1 doit être résolue sur l'id 8 après renommage : %+v", report.Lights[0])
	}
}

func TestDriver_AmbiguousName_RefusedNeverGuesses(t *testing.T) {
	fb := newFakeBridge(t, "001788fffea0591e")
	fb.addLight("8", "BuzzHue1")
	fb.addLight("9", "BuzzHue1") // homonyme

	d := newTestDriver(t, fb, []LightSpec{{Name: "BuzzHue1", Role: RoleGeneral}}, nil)
	ctx := context.Background()

	if err := d.Apply(ctx, generalState([3]int{0, 255, 0}, 200)); err != nil {
		t.Fatalf("Apply ne doit pas renvoyer d'erreur bloquante pour une ambiguïté locale à une ampoule : %v", err)
	}
	report := d.Status()
	if !report.Lights[0].Ambiguous || report.Lights[0].Resolved {
		t.Fatalf("BuzzHue1 doit être signalée ambiguë, jamais résolue : %+v", report.Lights[0])
	}
	if fb.writeCount() != 0 {
		t.Errorf("aucune écriture ne doit partir sur un nom ambigu (refus, jamais un choix arbitraire), got %d", fb.writeCount())
	}
}

// ---------------------------------------------------------------------------
// Taxonomie à trois issues — jamais deux (§5.6)
// ---------------------------------------------------------------------------

func TestDriver_BridgeOff_StatusUnreachable(t *testing.T) {
	fb := newFakeBridge(t, "001788fffea0591e")
	fb.addLight("8", "BuzzHue1")
	d := newTestDriver(t, fb, []LightSpec{{Name: "BuzzHue1"}}, func(c *Config) { c.Timeout = 200 * time.Millisecond; noRediscover(c) })
	fb.srv.Close() // pont "éteint"

	err := d.Apply(context.Background(), generalState([3]int{255, 0, 0}, 200))
	if err == nil {
		t.Fatal("Apply doit échouer quand le pont est injoignable")
	}
	if d.Status().State != StateUnreachable {
		t.Fatalf("état attendu unreachable, got %s (%s)", d.Status().State, d.Status().Reason)
	}
}

func TestDriver_RevokedKey_StatusRefused_NeverConfusedWithUnreachable(t *testing.T) {
	fb := newFakeBridge(t, "001788fffea0591e")
	fb.addLight("8", "BuzzHue1")
	fb.authError() // toute requête répond "unauthorized" (type 1)

	d := newTestDriver(t, fb, []LightSpec{{Name: "BuzzHue1"}}, nil)
	err := d.Apply(context.Background(), generalState([3]int{255, 0, 0}, 200))
	if err == nil {
		t.Fatal("Apply doit échouer avec une clé révoquée")
	}
	if d.Status().State != StateRefused {
		t.Fatalf("clé révoquée doit donner refused, JAMAIS unreachable ni une «erreur» générique — got %s", d.Status().State)
	}
}

func TestDriver_Refused_NeverRetriesInALoop(t *testing.T) {
	fb := newFakeBridge(t, "001788fffea0591e")
	fb.addLight("8", "BuzzHue1")
	fb.authError()
	d := newTestDriver(t, fb, []LightSpec{{Name: "BuzzHue1"}}, nil)
	ctx := context.Background()

	_ = d.Apply(ctx, generalState([3]int{255, 0, 0}, 200))
	if d.Status().State != StateRefused {
		t.Fatalf("setup invalide : état attendu refused, got %s", d.Status().State)
	}
	before := fb.hitCount()
	for i := 0; i < 20; i++ {
		if err := d.Apply(ctx, generalState([3]int{0, 0, 255}, 200)); err == nil {
			t.Fatal("Apply doit continuer à échouer tant que refused")
		}
	}
	if got := fb.hitCount(); got != before {
		t.Errorf("aucune nouvelle requête ne doit partir vers le pont en état refused (pas de relance en boucle), before=%d got=%d", before, got)
	}
}

func TestDriver_Unreachable_BackoffPreventsRequestStorm(t *testing.T) {
	fb := newFakeBridge(t, "001788fffea0591e")
	fb.addLight("8", "BuzzHue1")
	d := newTestDriver(t, fb, []LightSpec{{Name: "BuzzHue1"}}, func(c *Config) { c.Timeout = 100 * time.Millisecond; noRediscover(c) })
	fb.srv.Close()
	ctx := context.Background()

	if err := d.Apply(ctx, generalState([3]int{255, 0, 0}, 200)); err == nil {
		t.Fatal("setup invalide : le premier Apply doit échouer (pont éteint)")
	}
	before := fb.hitCount()
	// Rafale de tentatives immédiates : le retrait exponentiel doit toutes
	// les absorber sans redemander le réseau (contract §5.5 : "l'écrivain
	// n'est jamais bloqué").
	for i := 0; i < 10; i++ {
		_ = d.Apply(ctx, generalState([3]int{255, 0, 0}, 200))
	}
	if got := fb.hitCount(); got != before {
		t.Errorf("le retrait exponentiel doit empêcher toute nouvelle requête tant que la fenêtre n'est pas écoulée, before=%d got=%d", before, got)
	}
}

// ---------------------------------------------------------------------------
// N'écrire que ce qui change (§5.3) — la mitigation principale du §2
// ---------------------------------------------------------------------------

func TestDriver_WritesOnlyWhatChanges(t *testing.T) {
	fb := newFakeBridge(t, "001788fffea0591e")
	fb.addLight("8", "BuzzHue1")
	fb.addLight("9", "BuzzHue2")
	d := newTestDriver(t, fb, []LightSpec{{Name: "BuzzHue1"}, {Name: "BuzzHue2"}}, nil)
	ctx := context.Background()

	if err := d.Apply(ctx, generalState([3]int{255, 0, 0}, 200)); err != nil {
		t.Fatalf("Apply #1 a échoué : %v", err)
	}
	first := fb.writeCount()
	if first != 2 {
		t.Fatalf("le premier Apply doit écrire les 2 ampoules (aucun état connu), got %d", first)
	}

	// Même State exactement : aucune écriture supplémentaire.
	if err := d.Apply(ctx, generalState([3]int{255, 0, 0}, 200)); err != nil {
		t.Fatalf("Apply #2 a échoué : %v", err)
	}
	if got := fb.writeCount(); got != first {
		t.Errorf("un State identique ne doit produire AUCUNE écriture supplémentaire (§5.3), got %d écritures de plus", got-first)
	}
	if d.Status().Stats.Skipped == 0 {
		t.Errorf("Stats.Skipped doit refléter les écritures évitées, got %+v", d.Status().Stats)
	}

	// State différent : exactement les ampoules dont l'état cible change.
	if err := d.Apply(ctx, generalState([3]int{0, 255, 0}, 200)); err != nil {
		t.Fatalf("Apply #3 a échoué : %v", err)
	}
	if got := fb.writeCount(); got != first+2 {
		t.Errorf("un changement de couleur doit réécrire les 2 ampoules, got %d écritures depuis le début (attendu %d)", got, first+2)
	}
}

func TestDriver_WriteError_InvalidatesCacheForRetryOnNextApply(t *testing.T) {
	fb := newFakeBridge(t, "001788fffea0591e")
	l := fb.addLight("8", "BuzzHue1")
	d := newTestDriver(t, fb, []LightSpec{{Name: "BuzzHue1"}}, nil)
	ctx := context.Background()

	if err := d.Apply(ctx, generalState([3]int{255, 0, 0}, 200)); err != nil {
		t.Fatalf("Apply #1 a échoué : %v", err)
	}
	after1 := fb.writeCount()

	// Le prochain PUT sur cette ampoule échoue (ex: 201 device off) — une
	// erreur PAR AMPOULE, pas une panne du pont : Apply doit rester nil.
	l.putErr = &hueError{Type: 201, Description: "device is off"}
	if err := d.Apply(ctx, generalState([3]int{0, 0, 255}, 200)); err != nil {
		t.Fatalf("une erreur par ampoule (type 201) ne doit jamais faire échouer Apply : %v", err)
	}
	if d.Status().Lights[0].LastError == "" {
		t.Error("l'erreur par ampoule doit être visible dans Status()")
	}

	// Rejouer le MÊME State : puisque l'écriture précédente a échoué, l'état
	// mémorisé a été invalidé — le prochain Apply doit retenter, pas
	// silencieusement croire que c'est déjà fait.
	before := fb.writeCount()
	if err := d.Apply(ctx, generalState([3]int{0, 0, 255}, 200)); err != nil {
		t.Fatalf("Apply de retentative a échoué : %v", err)
	}
	if got := fb.writeCount(); got <= before {
		t.Errorf("une écriture ayant échoué doit être retentée au prochain Apply, got %d écritures (before=%d, after1=%d)", got, before, after1)
	}
}

// ---------------------------------------------------------------------------
// Une ampoule injoignable n'abat jamais les autres (§5.5)
// ---------------------------------------------------------------------------

func TestDriver_OneUnreachableLight_OthersStillWritten(t *testing.T) {
	fb := newFakeBridge(t, "001788fffea0591e")
	off := fb.addLight("8", "Scène") // "éteinte au mur"
	off.reachable = false
	off.putErr = &hueError{Type: 201, Description: "device is off"}
	fb.addLight("9", "BuzzHue2")

	d := newTestDriver(t, fb, []LightSpec{{Name: "Scène"}, {Name: "BuzzHue2"}}, nil)
	if err := d.Apply(context.Background(), generalState([3]int{255, 255, 255}, 200)); err != nil {
		t.Fatalf("une ampoule éteinte au mur ne doit jamais faire échouer Apply pour les autres : %v", err)
	}
	report := d.Status()
	var scene, buzzhue2 LightStatus
	for _, l := range report.Lights {
		if l.Name == "Scène" {
			scene = l
		}
		if l.Name == "BuzzHue2" {
			buzzhue2 = l
		}
	}
	if scene.LastError == "" {
		t.Error("l'ampoule injoignable doit porter une erreur visible")
	}
	if buzzhue2.LastError != "" {
		t.Errorf("l'ampoule saine ne doit porter aucune erreur : %+v", buzzhue2)
	}
	if fb.writeCount() < 2 {
		t.Errorf("les deux ampoules doivent avoir reçu une tentative d'écriture, got %d écritures", fb.writeCount())
	}
}

// ---------------------------------------------------------------------------
// Journalisation — une ligne au changement d'état, jamais une par échec
// ---------------------------------------------------------------------------

func TestDriver_LogsOnceOnStateChange_NeverPerFailure(t *testing.T) {
	fb := newFakeBridge(t, "001788fffea0591e")
	fb.addLight("8", "BuzzHue1")
	var mu sync.Mutex
	var lines []string
	d := newTestDriver(t, fb, []LightSpec{{Name: "BuzzHue1"}}, func(c *Config) {
		c.Timeout = 100 * time.Millisecond
		noRediscover(c)
		c.Logger = func(format string, args ...any) {
			mu.Lock()
			lines = append(lines, fmt.Sprintf(format, args...))
			mu.Unlock()
		}
	})
	fb.srv.Close()
	ctx := context.Background()

	// 15 échecs consécutifs (pont éteint) : un seul changement d'état réel
	// (unreachable→unreachable ne compte pas), donc au plus 1 ligne pour la
	// bascule initiale — jamais 15.
	for i := 0; i < 15; i++ {
		_ = d.Apply(ctx, generalState([3]int{255, 0, 0}, 200))
	}
	mu.Lock()
	got := len(lines)
	mu.Unlock()
	if got > 1 {
		t.Errorf("attendu au plus 1 ligne de log pour 15 échecs identiques (jamais une par échec) — got %d: %v", got, lines)
	}
}

// ---------------------------------------------------------------------------
// Résolution des zones (§5.2) — general vs équipe, avec repli
// ---------------------------------------------------------------------------

func TestZoneFor_GeneralLight_AlwaysFollowsGeneralZone(t *testing.T) {
	lc := LightSpec{Name: "X", Role: RoleGeneral}
	st := lighting.State{Zones: []lighting.ZoneState{{Zone: lighting.ZoneGeneral, Intensity: 100}}}
	z, ok := zoneFor(lc, st)
	if !ok || z.Intensity != 100 {
		t.Fatalf("une ampoule 'general' doit suivre la zone 'general', got %+v ok=%v", z, ok)
	}
}

func TestZoneFor_TeamLight_FollowsItsTeamZoneWhenPresent(t *testing.T) {
	lc := LightSpec{Name: "X", Role: RoleTeam, Team: "Rouges"}
	st := lighting.State{Zones: []lighting.ZoneState{
		{Zone: lighting.ZoneGeneral, Intensity: 10},
		{Zone: "Rouges", Intensity: 200},
	}}
	z, ok := zoneFor(lc, st)
	if !ok || z.Intensity != 200 {
		t.Fatalf("une ampoule d'équipe doit suivre SA zone d'équipe quand elle existe, got %+v ok=%v", z, ok)
	}
}

func TestZoneFor_TeamLight_FallsBackToGeneralWhenTeamNotNamed(t *testing.T) {
	// Contract §5.2 : "plus toute ampoule d'équipe dont l'équipe n'est pas
	// nommée dans l'état courant" doit suivre 'general'.
	lc := LightSpec{Name: "X", Role: RoleTeam, Team: "Bleus"}
	st := lighting.State{Zones: []lighting.ZoneState{{Zone: lighting.ZoneGeneral, Intensity: 77}}}
	z, ok := zoneFor(lc, st)
	if !ok || z.Intensity != 77 {
		t.Fatalf("une ampoule d'équipe non nommée dans l'état courant doit retomber sur 'general', got %+v ok=%v", z, ok)
	}
}

func TestZoneFor_NoMatchingZoneAtAll_IsASilentNonEvent(t *testing.T) {
	lc := LightSpec{Name: "X", Role: RoleGeneral}
	st := lighting.State{} // aucune zone du tout
	if _, ok := zoneFor(lc, st); ok {
		t.Fatal("aucune zone ne doit produire ok=false (non-événement silencieux, contract §5.2), pas une correspondance inventée")
	}
}

// ---------------------------------------------------------------------------
// desired() — conversion State → applied (§5.2)
// ---------------------------------------------------------------------------

func TestDesired_ZeroIntensity_IsOffNotDimZero(t *testing.T) {
	a := desired(lighting.ZoneState{Color: [3]int{255, 0, 0}, Intensity: 0})
	if a.on {
		t.Fatalf("Intensity==0 doit produire on=false, jamais une luminosité nulle allumée : %+v", a)
	}
	v1 := a.toV1()
	if v1.On == nil || *v1.On != false || v1.Bri != nil {
		t.Fatalf("toV1() pour intensity=0 doit envoyer {on:false} sans bri, got %+v", v1)
	}
	if v1.TransitionTime == nil || *v1.TransitionTime != TransitionTime {
		t.Fatalf("transitiontime doit toujours être la constante TransitionTime (0), got %+v", v1.TransitionTime)
	}
}

func TestDesired_PositiveIntensity_IsOnWithComputedBriAndXY(t *testing.T) {
	a := desired(lighting.ZoneState{Color: [3]int{0, 255, 0}, Intensity: 200})
	if !a.on {
		t.Fatal("Intensity>0 doit produire on=true")
	}
	wantBri := intensityToBri(200)
	if a.bri != wantBri {
		t.Errorf("bri = %d, attendu %d", a.bri, wantBri)
	}
	wantXY := rgbToXY(0, 255, 0)
	if a.xy != wantXY {
		t.Errorf("xy = %v, attendu %v", a.xy, wantXY)
	}
}

// ---------------------------------------------------------------------------
// New() — validation de configuration
// ---------------------------------------------------------------------------

func TestNew_Validation(t *testing.T) {
	valid := Config{BridgeIP: "192.168.1.10", APIKey: "abc123", Lights: []LightSpec{{Name: "A"}}}
	if _, err := New(valid); err != nil {
		t.Fatalf("configuration valide refusée : %v", err)
	}

	cases := []struct {
		name string
		cfg  Config
	}{
		{"clé absente", Config{BridgeIP: "192.168.1.10"}},
		{"ni ip ni id", Config{APIKey: "abc"}},
		{"clé avec caractères invalides", Config{BridgeIP: "192.168.1.10", APIKey: "abc/def"}},
		{"lumières en double", Config{BridgeIP: "192.168.1.10", APIKey: "abc", Lights: []LightSpec{{Name: "A"}, {Name: "A"}}}},
		{"nom vide", Config{BridgeIP: "192.168.1.10", APIKey: "abc", Lights: []LightSpec{{Name: "  "}}}},
		{"rôle équipe sans équipe", Config{BridgeIP: "192.168.1.10", APIKey: "abc", Lights: []LightSpec{{Name: "A", Role: RoleTeam}}}},
		{"rôle inconnu", Config{BridgeIP: "192.168.1.10", APIKey: "abc", Lights: []LightSpec{{Name: "A", Role: "bogus"}}}},
	}
	for _, c := range cases {
		if _, err := New(c.cfg); err == nil {
			t.Errorf("%s : devrait être refusé", c.name)
		}
	}
}

func TestNew_BridgeIDOnly_NoNetworkIO(t *testing.T) {
	// Ni IP ni appel réseau au moment de New() — seule Apply()/Inventory()
	// déclenchera une découverte (contract §4.1, "au démarrage, si bridge_id
	// est connu et que l'IP a changé... le pilote re-découvre").
	d, err := New(Config{BridgeID: "001788fffea0591e", APIKey: "abc", Lights: []LightSpec{{Name: "A"}}})
	if err != nil {
		t.Fatalf("bridge_id seul doit être accepté sans IP : %v", err)
	}
	if d.client != nil {
		t.Error("New() ne doit établir aucun client HTTP quand seule bridge_id est connue (pas d'I/O réseau à la construction)")
	}
}

// ---------------------------------------------------------------------------
// Re-découverte par identifiant quand l'IP a changé (§4.1)
// ---------------------------------------------------------------------------

func TestDriver_BridgeIDMismatch_Rediscovers(t *testing.T) {
	oldBridge := newFakeBridge(t, "AAAA") // un AUTRE pont répond maintenant à l'ancienne IP
	oldBridge.addLight("1", "Autre")
	newBridgeSrv := newFakeBridge(t, "001788fffea0591e")
	newBridgeSrv.addLight("8", "BuzzHue1")

	findCalls := 0
	d := newTestDriver(t, oldBridge, []LightSpec{{Name: "BuzzHue1"}}, func(c *Config) {
		c.BridgeID = "001788fffea0591e" // attendu — ne correspond pas à oldBridge
		c.FindBridge = func(ctx context.Context, id string, timeout time.Duration) (Bridge, bool, error) {
			findCalls++
			return Bridge{IP: newBridgeSrv.srv.URL, ID: id}, true, nil
		}
	})

	if err := d.Apply(context.Background(), generalState([3]int{255, 0, 0}, 200)); err != nil {
		t.Fatalf("Apply après re-découverte a échoué : %v", err)
	}
	if findCalls == 0 {
		t.Fatal("un id de pont différent de celui répondant à l'IP configurée doit déclencher une re-découverte")
	}
	if got := d.Status().BridgeIP; got != newBridgeSrv.srv.URL {
		t.Errorf("l'IP effective doit être celle du pont re-découvert, got %q", got)
	}
	if newBridgeSrv.writeCount() == 0 {
		t.Error("l'écriture doit finalement atteindre le BON pont (re-découvert), pas l'ancien")
	}
}

// ---------------------------------------------------------------------------
// Constantes normatives (contract §5.4, §5.2)
// ---------------------------------------------------------------------------

func TestDriver_NormativeConstants(t *testing.T) {
	if RecommendedMinInterval != 250*time.Millisecond {
		t.Errorf("RecommendedMinInterval = %s, attendu 250ms (contract §5.4)", RecommendedMinInterval)
	}
	if TransitionTime != 0 {
		t.Errorf("TransitionTime = %d, attendu 0 — instantané (contract §5.2)", TransitionTime)
	}
}

// Interface satisfaite — vérifiée aussi au niveau du package via
// `var _ lighting.Driver = (*Driver)(nil)` dans driver.go ; ce test la
// referme côté test-writer pour qu'elle apparaisse dans ce fichier aussi.
func TestDriver_ImplementsLightingDriver(t *testing.T) {
	var _ lighting.Driver = (*Driver)(nil)
}
