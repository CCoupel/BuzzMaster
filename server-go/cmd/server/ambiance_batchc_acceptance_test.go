// Suite d'acceptance test-writer pour Batch C (C1b + C3), milestone
// v10.0.0, retour QUALIF round 6. Référence :
// _work/reports/planner-v10-teamcolor-changes-20260908-092100.md. Contre
// l'implémentation réelle de dev-backend (ambiance.go/ambiance_override.go/
// main.go, commits a1da174a + 97e60c58).
//
// Complémentaire de :
//   - ambiance_team_color_acceptance_test.go (test-writer, Batch A) et
//     TestDevAmbianceTeamZones213 (dev-backend, mis à jour dans 97e60c58)
//     pour C3 points 1-3 (couleur/intensité hors partie et selon distinction)
//     — déjà exhaustifs après leurs propres mises à jour C1a, NON répétés
//     ici (vérifié par exécution, voir le rapport DONE) ;
//   - TestDevLightingMode_ScopedToGeneralZoneOnly (dev-backend, OFF en
//     partie) et TestCA208_Flash_NeverAffectsAnActiveTeamZone (test-writer,
//     Flash en partie) pour C3 point 4 — ce fichier-ci étend cette garantie
//     au cas HORS PARTIE (KindIdle), qu'aucun des deux ne couvre ;
//   - TestDevShutdownExtinguishHueLighting_AppliesOffToEveryConfiguredZone
//     (dev-backend) pour C1b — ce test-ci compte des PUT sur une
//     installation jamais allumée au préalable. Ce fichier-ci allume
//     RÉELLEMENT l'ampoule d'équipe via le vrai chemin d'écriture (C1a
//     l'y engage désormais en permanence) puis vérifie l'ÉTAT PHYSIQUE
//     final après extinction — pas seulement un compte de PUT.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
// Aides préfixées twC (Batch C) pour ne jamais entrer en collision.
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/lighting"
)

// ---------------------------------------------------------------------------
// C1b — extinction à l'arrêt : preuve empirique sur une ampoule d'équipe
// RÉELLEMENT allumée au préalable (pas seulement un compte de PUT sur une
// installation jamais allumée).
// ---------------------------------------------------------------------------

type twCFakeLight struct {
	on  bool
	bri int
}

// twCFakeBridge is a minimal Hue v1 fake tracking PHYSICAL per-light state
// (unlike the dev tests' bridges here, which only count PUTs) — the shape
// C1b's task explicitly asks for ("prouver empiriquement").
type twCFakeBridge struct {
	mu     sync.Mutex
	lights map[string]*twCFakeLight // id -> state
	names  map[string]string        // id -> name
}

func newTwCFakeBridge(names map[string]string) *twCFakeBridge {
	lights := make(map[string]*twCFakeLight, len(names))
	for id := range names {
		lights[id] = &twCFakeLight{on: true, bri: 200} // starts lit — extinction must actually change this
	}
	return &twCFakeBridge{lights: lights, names: names}
}

func (b *twCFakeBridge) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/config"):
			_, _ = w.Write([]byte(`{"bridgeid":"fffe0000deadbeef","modelid":"BSB002"}`))
		case strings.HasSuffix(r.URL.Path, "/lights") && r.Method == http.MethodGet:
			b.mu.Lock()
			body := `{`
			first := true
			for id, name := range b.names {
				if !first {
					body += ","
				}
				first = false
				l := b.lights[id]
				body += `"` + id + `":{"name":"` + name + `","state":{"on":` + boolStr(l.on) + `,"bri":` + itoa(l.bri) + `,"xy":[0.3,0.3],"reachable":true}}`
			}
			body += `}`
			b.mu.Unlock()
			_, _ = w.Write([]byte(body))
		case strings.Contains(r.URL.Path, "/lights/") && strings.HasSuffix(r.URL.Path, "/state") && r.Method == http.MethodPut:
			parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/state"), "/")
			id := parts[len(parts)-1]
			b.mu.Lock()
			l, ok := b.lights[id]
			b.mu.Unlock()
			if !ok {
				w.WriteHeader(404)
				return
			}
			var body struct {
				On  *bool `json:"on"`
				Bri *int  `json:"bri"`
			}
			_ = jsonDecode(r, &body)
			b.mu.Lock()
			if body.On != nil {
				l.on = *body.On
			}
			if body.Bri != nil {
				l.bri = *body.Bri
			}
			b.mu.Unlock()
			_, _ = w.Write([]byte(`[{"success":{"/lights/` + id + `/state/on":true}}]`))
		default:
			w.WriteHeader(404)
		}
	}
}

func (b *twCFakeBridge) state(id string) (on bool, bri int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	l := b.lights[id]
	return l.on, l.bri
}

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
func itoa(v int) string { return strconv.Itoa(v) }

// TestC1b_Shutdown_ExtinguishesGeneralAndAnActuallyLitTeamBulb_Empirically
// lights the team bulb through the REAL writer path (C1a: a team on the
// board is now ALWAYS rendered, in or out of game) before calling shutdown,
// then reads back the fake bridge's own PHYSICAL light state — not a PUT
// count — for both the general and the team bulb.
func TestC1b_Shutdown_ExtinguishesGeneralAndAnActuallyLitTeamBulb_Empirically(t *testing.T) {
	bridge := newTwCFakeBridge(map[string]string{"1": "Salle", "2": "AmpouleRouge"})
	srv := httptest.NewServer(bridge.handler())
	defer srv.Close()

	app := newTestApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.ctx = ctx
	saved := *config.Get()
	t.Cleanup(func() { config.SetInstance(&saved) })
	cfg := saved
	cfg.Lighting = config.LightingConfig{
		Enabled: true, BridgeIP: srv.URL, BridgeID: "fffe0000deadbeef", APIKey: "k",
		Lights: []config.LightingLightEntry{
			{Name: "Salle", Role: "general"},
			{Name: "AmpouleRouge", Role: "team", Team: "Rouges"},
		},
	}
	config.SetInstance(&cfg)
	app.reconfigureAmbiance()
	if app.LightingDriver() == nil {
		t.Fatal("driver must be built from a valid, enabled config")
	}

	// Allume RÉELLEMENT l'ampoule d'équipe via le vrai chemin (C1a) : un
	// roster avec "Rouges" au plateau suffit, même hors partie.
	app.engine.SetTeams(map[string]*game.Team{"Rouges": {Name: "Rouges", Color: []int{255, 0, 0}}})
	app.ambiance().NotifyState()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if on, bri := bridge.state("2"); on && bri > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if on, bri := bridge.state("2"); !on || bri == 0 {
		t.Fatalf("setup invalide : AmpouleRouge devrait être allumée avant l'extinction (C1a), got on=%v bri=%d", on, bri)
	}

	app.shutdownExtinguishHueLighting()

	// L'extinction elle-même ouvre un NOUVEAU driver de courte durée
	// (voir la doc de shutdownExtinguishHueLighting) : laisser le temps à
	// sa propre requête HTTP d'aboutir sur le faux pont.
	deadline = time.Now().Add(3 * time.Second)
	var onGeneral, onTeam bool
	for time.Now().Before(deadline) {
		onGeneral, _ = bridge.state("1")
		onTeam, _ = bridge.state("2")
		if !onGeneral && !onTeam {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if onGeneral {
		t.Error("la zone general doit être physiquement éteinte après l'arrêt du serveur")
	}
	if onTeam {
		t.Error("l'ampoule d'équipe — allumée juste avant — doit être physiquement éteinte après l'arrêt du serveur (contract §10.4, réserve C1)")
	}
}

// ---------------------------------------------------------------------------
// C3 point 4 (extension) — le sélecteur général ON/AUTO/OFF n'atteint jamais
// une ampoule d'équipe, y compris HORS PARTIE (KindIdle) — le cas que ni
// TestDevLightingMode_ScopedToGeneralZoneOnly ni
// TestCA208_Flash_NeverAffectsAnActiveTeamZone (tous deux en partie,
// KindTeamTurn) ne couvrent. Pertinent depuis C1a : une ampoule d'équipe
// existe désormais aussi hors partie, donc cette garantie doit désormais
// être vraie là aussi.
// ---------------------------------------------------------------------------

func TestC3_Selector_NeverAffectsTeamZone_EvenOutOfGame(t *testing.T) {
	app := newTestApp(t)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA", Color: []int{255, 0, 0}}})
	wantTeamA := app.teamNameToRGB("TeamA")
	ev := lighting.Event{Kind: lighting.KindIdle} // hors partie

	app.setLightingMode(lightingModeOn)
	st := app.ambianceScene(ev)
	zones := map[string]lighting.ZoneState{}
	for _, z := range st.Zones {
		zones[z.Zone] = z
	}
	if zones[lighting.ZoneGeneral].Color != lightingOnColor {
		t.Fatalf("ON doit forcer general même hors partie, got %+v", zones[lighting.ZoneGeneral])
	}
	if zones["TeamA"].Color != wantTeamA || zones["TeamA"].Intensity != 255 {
		t.Errorf("ON (hors partie) ne doit jamais affecter la zone TeamA, got %+v want couleur=%v intensité=255", zones["TeamA"], wantTeamA)
	}

	app.setLightingMode(lightingModeOff)
	st = app.ambianceScene(ev)
	zones = map[string]lighting.ZoneState{}
	for _, z := range st.Zones {
		zones[z.Zone] = z
	}
	if zones[lighting.ZoneGeneral].Intensity != 0 {
		t.Fatalf("OFF doit éteindre general même hors partie, got %+v", zones[lighting.ZoneGeneral])
	}
	if zones["TeamA"].Color != wantTeamA || zones["TeamA"].Intensity != 255 {
		t.Errorf("OFF (hors partie) ne doit jamais affecter la zone TeamA, got %+v want couleur=%v intensité=255", zones["TeamA"], wantTeamA)
	}
}

// ---------------------------------------------------------------------------
// C3 point 6 / C2a — Event.Points doit être renseigné (jamais un 0 codé en
// dur par erreur) aux 4 sites NotifyPulse(KindScore, ...) de main.go.
// Garde textuelle, à l'image de TestCA2_StartAmbianceLifecycleGuard_
// IsPresentInSource et TestCA208_ShutdownExtinctionOrder_..._SourceGuard
// (déjà en place dans ce paquet) — attrape la faute d'inattention qu'un
// test purement fonctionnel ne verrait pas forcément (un site qui passerait
// une constante 0 au lieu de la variable de points collectée juste avant).
// ---------------------------------------------------------------------------

var twCNotifyPulseScoreRE = regexp.MustCompile(`NotifyPulse\(lighting\.KindScore,\s*\[\]string\{[^}]*\},\s*([^,]+),\s*lighting\.ScorePulseDuration\)`)

func TestC2a_EveryScoreNotifyPulseSite_CarriesRealPoints_NeverALiteralZero(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) a échoué")
	}
	path := filepath.Join(filepath.Dir(thisFile), "main.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lecture de %s : %v", path, err)
	}
	matches := twCNotifyPulseScoreRE.FindAllStringSubmatch(string(content), -1)
	if len(matches) != 4 {
		t.Fatalf("attendu exactement 4 sites NotifyPulse(lighting.KindScore, ...) dans main.go, got %d : %v", len(matches), matches)
	}
	for i, m := range matches {
		arg := strings.TrimSpace(m[1])
		if arg == "0" {
			t.Errorf("site %d : l'argument Points est un littéral 0 — probable oubli de branchement du vrai nombre de points", i)
		}
	}
}

// ---------------------------------------------------------------------------
// C3 point 5 / C2b — clignotement SCORE : N cycles = clamp(points, 1, 6),
// alternance or ↔ couleur d'équipe, retour à la couleur d'équipe après le
// dernier clignotement.
// ---------------------------------------------------------------------------

func twCWaitTeamZoneColor(t *testing.T, fake *lighting.FakeDriver, team string, want [3]int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		last, ok := fake.Last()
		if ok {
			for _, z := range last.Zones {
				if z.Zone == team && z.Color == want {
					return
				}
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	last, _ := fake.Last()
	t.Fatalf("timeout en attendant la zone %s à la couleur %v — dernier état observé : %+v", team, want, last)
}

func TestC2b_ScoreFlash_TwoCycles_AlternatesGoldAndTeamColour_ThenSettles(t *testing.T) {
	app, fake, cancel := tw208NewWriterApp(t)
	defer cancel()
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA", Color: []int{255, 0, 0}}})
	wantTeamA := app.teamNameToRGB("TeamA")
	app.ambiance().NotifyState() // établit un premier état stable (hors partie)
	twCWaitTeamZoneColor(t, fake, "TeamA", wantTeamA, 2*time.Second)

	// startScoreFlash seul ne suffit pas : le rendu or n'apparaît que quand
	// l'événement COURANT dérivé est KindScore (ambianceScene lit
	// currentScoreFlashTeam() uniquement dans ce cas). En production, les 4
	// sites appellent TOUJOURS NotifyPulse ET startScoreFlash ensemble
	// (main.go) — on reproduit exactement cet appariement ici, avec la VRAIE
	// durée de pulse pour ne jamais l'expirer avant la fin du clignotement.
	app.ambiance().NotifyPulse(lighting.KindScore, []string{"TeamA"}, 2, lighting.ScorePulseDuration)
	app.startScoreFlash("TeamA", 2)

	// Cycle 1 : or, puis retour à la couleur d'équipe.
	twCWaitTeamZoneColor(t, fake, "TeamA", ambianceScoreGold, 800*time.Millisecond)
	twCWaitTeamZoneColor(t, fake, "TeamA", wantTeamA, 800*time.Millisecond)
	// Cycle 2 : or, puis retour — et c'est le DERNIER cycle (points=2).
	twCWaitTeamZoneColor(t, fake, "TeamA", ambianceScoreGold, 800*time.Millisecond)
	twCWaitTeamZoneColor(t, fake, "TeamA", wantTeamA, 800*time.Millisecond)

	// Le clignotement doit s'arrêter de lui-même juste après ce 2e cycle
	// (la couleur d'équipe ci-dessus a été observée dès le DÉBUT de la
	// dernière phase "off", avant même que ses 400 ms ne soient écoulées et
	// que le nettoyage de fin de cycle ne s'exécute — on laisse donc une
	// marge avant d'exiger currentScoreFlashTeam()=="") — et surtout,
	// aucun 3e passage à l'or ne doit apparaître entre-temps.
	sawThirdGold := false
	settleDeadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(settleDeadline) {
		if last, ok := fake.Last(); ok {
			for _, z := range last.Zones {
				if z.Zone == "TeamA" && z.Color == ambianceScoreGold {
					sawThirdGold = true
				}
			}
		}
		if app.currentScoreFlashTeam() == "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if sawThirdGold {
		t.Fatal("un 3e passage à l'or a été observé alors que points=2 ne doit produire que 2 cycles")
	}
	if got := app.currentScoreFlashTeam(); got != "" {
		t.Fatalf("le clignotement devrait s'être terminé de lui-même après 2 cycles (points=2), currentScoreFlashTeam=%q", got)
	}
}

func TestC2b_ScoreFlash_ClampsAboveSixPoints(t *testing.T) {
	if testing.Short() {
		t.Skip("mesure temporelle réelle (~5 s)")
	}
	app, fake, cancel := tw208NewWriterApp(t)
	defer cancel()
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA", Color: []int{255, 0, 0}}})
	app.ambiance().NotifyState()
	twCWaitTeamZoneColor(t, fake, "TeamA", app.teamNameToRGB("TeamA"), 2*time.Second)

	app.startScoreFlash("TeamA", 9) // au-delà de 6 : doit être plafonné à 6

	// 6 cycles = 6 x 800 ms = 4800 ms. On attend un peu plus (5,3 s) et on
	// exige que le clignotement soit déjà terminé — s'il ne l'était pas
	// (9 cycles réels = 7,2 s), currentScoreFlashTeam() serait encore "TeamA"
	// à cette échéance, ce qui distingue sans ambiguïté 6 de 9.
	deadline := time.Now().Add(5300 * time.Millisecond)
	for time.Now().Before(deadline) {
		if app.currentScoreFlashTeam() == "" {
			return // terminé — plafond respecté
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("le clignotement n'était pas terminé à 5,3 s — points=9 n'a pas été plafonné à 6 cycles (6 x 800 ms = 4800 ms attendu)")
}

func TestC2b_ScoreFlashAndGeneralFlash_RunSimultaneouslyWithoutInterference(t *testing.T) {
	app, fake, cancel := tw208NewWriterApp(t)
	defer cancel()
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA", Color: []int{255, 0, 0}}})
	wantTeamA := app.teamNameToRGB("TeamA")
	app.ambiance().NotifyState()
	twCWaitTeamZoneColor(t, fake, "TeamA", wantTeamA, 2*time.Second)

	app.setLightingFlash(true) // Flash général (zone 'general')
	defer app.setLightingFlash(false)
	// Cf. le commentaire du test précédent : NotifyPulse ET startScoreFlash
	// sont TOUJOURS appariés en production (main.go).
	app.ambiance().NotifyPulse(lighting.KindScore, []string{"TeamA"}, 2, lighting.ScorePulseDuration)
	app.startScoreFlash("TeamA", 2) // clignotement SCORE (zone 'TeamA')

	sawGeneralFlashLit, sawGeneralFlashDark, sawTeamGold, sawTeamOwn := false, false, false, false
	deadline := time.Now().Add(2500 * time.Millisecond)
	for time.Now().Before(deadline) {
		last, ok := fake.Last()
		if ok {
			for _, z := range last.Zones {
				switch z.Zone {
				case lighting.ZoneGeneral:
					if z.Color == lightingFlashColor && z.Intensity == lightingOnIntensity {
						sawGeneralFlashLit = true
					}
					if z.Intensity == 0 {
						sawGeneralFlashDark = true
					}
				case "TeamA":
					if z.Color == ambianceScoreGold {
						sawTeamGold = true
					}
					if z.Color == wantTeamA {
						sawTeamOwn = true
					}
				}
			}
		}
		if sawGeneralFlashLit && sawGeneralFlashDark && sawTeamGold && sawTeamOwn {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !sawGeneralFlashLit || !sawGeneralFlashDark {
		t.Errorf("le Flash général doit continuer d'alterner pendant le clignotement SCORE : lit=%v dark=%v", sawGeneralFlashLit, sawGeneralFlashDark)
	}
	if !sawTeamGold || !sawTeamOwn {
		t.Errorf("le clignotement SCORE doit continuer d'alterner pendant le Flash général : or=%v couleur d'équipe=%v", sawTeamGold, sawTeamOwn)
	}
}

// jsonDecode is a tiny local body decoder helper for the one call site above.
func jsonDecode(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
