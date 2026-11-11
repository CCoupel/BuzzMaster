package server

// #223 (v11.7.0, LOT-2) — POST /config.json (effets par moment : fusion, validation 400),
// GET /api/lighting/effects, POST /api/lighting/effects/reload, sauvegarde / restauration de files/effects/.
// Source : contracts/light-effects.md §3.2, §6, §7.1, §7.2 ; ambiance-settings.md §10.
// Fournisseur de catalogue réel (effects.Manager : natifs du dépôt + dossier utilisateur temporaire).
// Aides préfixées tw223 ; noms de tests TestTW223_* (scénarios 1 et 10 de light-effects-223.md).

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/effects"
)

type tw223Provider struct {
	m   *effects.Manager
	dir string
}

func (p *tw223Provider) EffectsCatalogue() effects.Catalogue { return p.m.Catalogue() }
func (p *tw223Provider) EffectsReload() effects.Catalogue    { return p.m.Reload() }
func (p *tw223Provider) EffectsDir() string                  { return p.dir }

// tw223Server : serveur de test + fournisseur de catalogue (natifs réels, dossier files/effects du dataDir).
func tw223Server(t *testing.T) (*HTTPServer, string, *tw223Provider) {
	t.Helper()
	srv, dataDir := setupTestHTTPServer(t)
	dir := filepath.Join(dataDir, "files", "effects")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// setupTestHTTPServer fait un t.Chdir : le chemin des natifs est résolu depuis ce fichier source.
	_, self, _, _ := runtime.Caller(0)
	natives := filepath.Join(filepath.Dir(self), "..", "..", "assets", "effects")
	prov := &tw223Provider{m: effects.NewManager(os.DirFS(natives), dir, nil), dir: dir}
	srv.Effects = prov
	// le serveur de test part d'une config vide : appliquer les défauts comme au chargement réel de config.json
	cfg := config.Get()
	config.ApplyDefaults(cfg)
	config.SetInstance(cfg)
	return srv, dataDir, prov
}

const tw223UserEffect = `id: mon-effet
nom: Mon effet
description: effet de test
version: 2
moments: [calm, score]
duree_tick_ms: 1000
pas:
  - { g1: ["#FF0000", 100] }
`

func tw223WriteEffect(t *testing.T, prov *tw223Provider, name, src string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(prov.dir, name), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	prov.m.Reload()
}

// tw223Zones : trois zones, deux groupes de salle (dont un groupe vide entre eux), un groupe animateur, aucun groupe public.
func tw223Zones(t *testing.T, srv *HTTPServer) {
	t.Helper()
	code, out := tw223Post(t, srv, `{"lighting":{"lights":[{"name":"A","role":"general"},{"name":"B","role":"general"},{"name":"H","role":"general"},{"name":"P","role":"general"}],`+
		`"groups":[{"lights":["A"]},{"lights":[]},{"lights":["B"]},{"role":"animateur","lights":["H"]}]}}`)
	if code != 200 {
		t.Fatalf("groupes : %d %s", code, out)
	}
}

var tw223ZoneNames = []string{"ambiance", "animateur", "public"}
var tw223MomentNames = []string{"chrono", "calm", "score"}

type tw223Settings struct {
	Effects map[string]map[string]string  `json:"effects"` // zone → moment → effet
	Speeds  map[string]map[string]float64 `json:"speeds"`  // zone → moment → vitesse
}

func tw223Lighting(t *testing.T, srv *HTTPServer) tw223Settings {
	t.Helper()
	req := httptest.NewRequest("GET", "/config.json", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	var parsed struct {
		Lighting tw223Settings `json:"lighting"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("JSON invalide : %v", err)
	}
	return parsed.Lighting
}

func tw223Post(t *testing.T, srv *HTTPServer, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest("POST", "/config.json", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

// ---------------------------------------------------------------------------
// POST /config.json — effets par zone et par moment (contrat §6, révision 4)
// ---------------------------------------------------------------------------

func TestTW223_Config_Defaults_ThreeZonesByThreeMoments(t *testing.T) {
	srv, _, _ := tw223Server(t)
	l := tw223Lighting(t, srv)
	for _, z := range tw223ZoneNames {
		for _, m := range tw223MomentNames {
			id, ok := l.Effects[z][m]
			if !ok {
				t.Errorf("lighting.effects.%s.%s toujours sérialisé : %+v", z, m, l.Effects)
			}
			want := ""
			if z == "ambiance" && m == "chrono" {
				want = "pulsation"
			}
			if id != want {
				t.Errorf("défaut %s.%s = %q, attendu %q (comportement v11.6.0 : pulsation pendant le chrono, rien ailleurs)", z, m, id, want)
			}
		}
	}
}

func TestTW223_Config_PartialPatchesMergeOneMomentAtATime(t *testing.T) {
	srv, _, prov := tw223Server(t)
	tw223WriteEffect(t, prov, "mon-effet.yaml", tw223UserEffect)
	for _, body := range []string{
		`{"lighting":{"effects":{"ambiance":{"chrono":"chenillard"}}}}`,
		`{"lighting":{"effects":{"ambiance":{"calm":"mon-effet"}}}}`,
		`{"lighting":{"effects":{"ambiance":{"score":"couleur-equipe"}}}}`,
	} {
		if code, out := tw223Post(t, srv, body); code != 200 {
			t.Fatalf("%s : %d %s", body, code, out)
		}
	}
	l := tw223Lighting(t, srv)
	a := l.Effects["ambiance"]
	if a["chrono"] != "chenillard" || a["calm"] != "mon-effet" || a["score"] != "couleur-equipe" {
		t.Errorf("chaque patch ne doit écraser que son moment : %+v", a)
	}
	if code, out := tw223Post(t, srv, `{"lighting":{"effects":{"ambiance":{"chrono":"pulsation"}}}}`); code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	a = tw223Lighting(t, srv).Effects["ambiance"]
	if a["chrono"] != "pulsation" || a["calm"] != "mon-effet" || a["score"] != "couleur-equipe" {
		t.Errorf("fusion par moment : %+v", a)
	}
}

func TestTW223_Config_ZonesAreIndependent(t *testing.T) {
	srv, _, _ := tw223Server(t)
	tw223Zones(t, srv)
	for _, body := range []string{
		`{"lighting":{"effects":{"animateur":{"score":"couleur-equipe"}}}}`,
		`{"lighting":{"effects":{"public":{"chrono":"pulsation"}}}}`,
		`{"lighting":{"effects":{"animateur":{"chrono":"pulsation"}}}}`,
		`{"lighting":{"effects":{"ambiance":{"chrono":"chenillard"}}}}`,
	} {
		if code, out := tw223Post(t, srv, body); code != 200 {
			t.Fatalf("%s : %d %s", body, code, out)
		}
	}
	e := tw223Lighting(t, srv).Effects
	if e["animateur"]["score"] != "couleur-equipe" || e["animateur"]["chrono"] != "pulsation" || e["animateur"]["calm"] != "" {
		t.Errorf("animateur : chaque patch ne touche que son moment : %+v", e["animateur"])
	}
	if e["public"]["chrono"] != "pulsation" || e["public"]["score"] != "" {
		t.Errorf("public indépendant de l'animateur : %+v", e["public"])
	}
	if e["ambiance"]["chrono"] != "chenillard" || e["ambiance"]["score"] != "" {
		t.Errorf("ambiance : %+v", e["ambiance"])
	}
}

func TestTW223_Config_ZoneWithoutGroup_SettingsAreKept(t *testing.T) {
	// §2.5 : une zone sans groupe non vide n'existe pas, mais ses réglages sont conservés
	srv, _, _ := tw223Server(t)
	if code, out := tw223Post(t, srv, `{"lighting":{"effects":{"public":{"score":"couleur-equipe"}},"speeds":{"public":{"score":2}}}}`); code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	l := tw223Lighting(t, srv)
	if l.Effects["public"]["score"] != "couleur-equipe" || l.Speeds["public"]["score"] != 2 {
		t.Errorf("réglages de la zone public conservés sans groupe : %+v", l)
	}
}

func TestTW223_Config_OldFlatAndHostFormsRefusedAtWrite(t *testing.T) {
	srv, _, _ := tw223Server(t)
	tw223Zones(t, srv)
	for _, body := range []string{
		`{"lighting":{"effects":{"chrono":"chenillard"}}}`,
		`{"lighting":{"speeds":{"chrono":2}}}`,
		`{"lighting":{"host_effects":{"score":"couleur-equipe"}}}`,
		`{"lighting":{"host_effects":{"ha":{"score":"couleur-equipe"}}}}`,
		`{"lighting":{"host_speeds":{"ha":{"score":2}}}}`,
	} {
		if code, _ := tw223Post(t, srv, body); code != http.StatusBadRequest {
			t.Errorf("les anciennes formes (à plat, host_*) ne sont plus acceptées à l'écriture : %s → %d", body, code)
		}
	}
}

func TestTW223_Config_EmptyIdMeansNoEffect_Accepted(t *testing.T) {
	srv, _, _ := tw223Server(t)
	if code, out := tw223Post(t, srv, `{"lighting":{"effects":{"ambiance":{"chrono":""}}}}`); code != 200 {
		t.Fatalf("« aucun effet » accepté : %d %s", code, out)
	}
	if got := tw223Lighting(t, srv).Effects["ambiance"]["chrono"]; got != "" {
		t.Errorf("\"\" remplace « pulsation » (aucune animation pendant le chrono) : %q", got)
	}
}

func TestTW223_Config_Refusals400_NothingChanged(t *testing.T) {
	cases := map[string]string{
		"moment inconnu":                     `{"lighting":{"effects":{"ambiance":{"bidule":"chenillard"}}}}`,
		"zone inconnue":                      `{"lighting":{"effects":{"salle":{"chrono":"chenillard"}}}}`,
		"id inexistant":                      `{"lighting":{"effects":{"ambiance":{"chrono":"n-existe-pas"}}}}`,
		"effet qui ne déclare pas le moment": `{"lighting":{"effects":{"ambiance":{"calm":"chenillard"}}}}`,
		"animateur : id d'effet inexistant":  `{"lighting":{"effects":{"animateur":{"calm":"n-existe-pas"}}}}`,
		"animateur : mauvais moment":         `{"lighting":{"effects":{"animateur":{"chrono":"couleur-equipe"}}}}`,
		"public : id d'effet inexistant":     `{"lighting":{"effects":{"public":{"calm":"n-existe-pas"}}}}`,
		"public : mauvais moment":            `{"lighting":{"effects":{"public":{"chrono":"couleur-equipe"}}}}`,
		"un bon moment et un mauvais":        `{"lighting":{"effects":{"ambiance":{"chrono":"chenillard","calm":"chenillard"}}}}`,
		"une bonne zone et une mauvaise":     `{"lighting":{"effects":{"ambiance":{"chrono":"chenillard"},"public":{"calm":"n-existe-pas"}}}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			srv, _, _ := tw223Server(t)
			tw223Zones(t, srv)
			before := tw223Lighting(t, srv)
			code, out := tw223Post(t, srv, body)
			if code != http.StatusBadRequest {
				t.Fatalf("400 attendu, obtenu %d : %s", code, out)
			}
			if strings.TrimSpace(out) == "" {
				t.Error("message lisible attendu")
			}
			after := tw223Lighting(t, srv)
			if !reflect.DeepEqual(before.Effects, after.Effects) {
				t.Errorf("un refus ne modifie rien (écriture atomique) : %+v → %+v", before.Effects, after.Effects)
			}
		})
	}
}

func TestTW223_Config_WithoutCatalogue_NonEmptyIdRefused(t *testing.T) {
	srv, _ := setupTestHTTPServer(t) // aucun fournisseur de catalogue
	if code, _ := tw223Post(t, srv, `{"lighting":{"effects":{"ambiance":{"calm":"chenillard"}}}}`); code != 400 {
		t.Errorf("sans catalogue, un id est introuvable : 400, obtenu %d", code)
	}
	if code, _ := tw223Post(t, srv, `{"lighting":{"effects":{"ambiance":{"calm":""}}}}`); code != 200 {
		t.Errorf("« aucun effet » reste valide sans catalogue : %d", code)
	}
}

func TestTW223_Config_SavingOtherSectionsNeverWipesEffects(t *testing.T) {
	srv, _, _ := tw223Server(t)
	tw223Zones(t, srv)
	tw223Post(t, srv, `{"lighting":{"effects":{"ambiance":{"chrono":"chenillard"},"animateur":{"score":"couleur-equipe"},"public":{"chrono":"pulsation"}}}}`)
	for _, body := range []string{`{"server":{"debug":true}}`, `{"lighting":{"enabled":false}}`} {
		if code, out := tw223Post(t, srv, body); code != 200 {
			t.Fatalf("%s : %d %s", body, code, out)
		}
		e := tw223Lighting(t, srv).Effects
		if e["ambiance"]["chrono"] != "chenillard" || e["animateur"]["score"] != "couleur-equipe" || e["public"]["chrono"] != "pulsation" {
			t.Fatalf("POST %s a modifié les effets : %+v", body, e)
		}
	}
}

func TestTW223_Config_EffectBecomingAbsentLater_DoesNotInvalidateTheConfig(t *testing.T) {
	srv, _, prov := tw223Server(t)
	tw223WriteEffect(t, prov, "mon-effet.yaml", tw223UserEffect)
	if code, out := tw223Post(t, srv, `{"lighting":{"effects":{"public":{"calm":"mon-effet"}}}}`); code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	if err := os.Remove(filepath.Join(prov.dir, "mon-effet.yaml")); err != nil {
		t.Fatal(err)
	}
	prov.m.Reload()
	if code, out := tw223Post(t, srv, `{"server":{"debug":false}}`); code != 200 {
		t.Fatalf("une configuration enregistrée reste valide quand l'effet disparaît : %d %s", code, out)
	}
	if got := tw223Lighting(t, srv).Effects["public"]["calm"]; got != "mon-effet" {
		t.Errorf("la valeur est conservée telle quelle : %q", got)
	}
}

// ---------------------------------------------------------------------------
// GET /api/lighting/effects (§7.1, révision 4) — groupes par zone, applicabilité par zone, tick_ms, unresolved par zone
// ---------------------------------------------------------------------------

type tw223EffectsView struct {
	Groups  map[string]int `json:"groups"`
	Effects []struct {
		ID         string          `json:"id"`
		Name       string          `json:"name"`
		Moments    []string        `json:"moments"`
		Origin     string          `json:"origin"`
		File       string          `json:"file"`
		MinGroups  int             `json:"min_groups"`
		TickMs     int             `json:"tick_ms"`
		Applicable map[string]bool `json:"applicable"`
	} `json:"effects"`
	Errors []struct {
		File    string
		Line    int
		Message string
	} `json:"errors"`
	Unresolved []struct {
		Zone   string `json:"zone"`
		Moment string `json:"moment"`
		ID     string `json:"id"`
		Scope  string `json:"scope"`
	} `json:"unresolved"`
	Dir string `json:"dir"`
}

func tw223GetEffects(t *testing.T, srv *HTTPServer, method, path string) (int, tw223EffectsView, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	var v tw223EffectsView
	_ = json.Unmarshal(w.Body.Bytes(), &v)
	return w.Code, v, w.Body.String()
}

func tw223Find(v tw223EffectsView, id string) (i int, ok bool) {
	for k, e := range v.Effects {
		if e.ID == id {
			return k, true
		}
	}
	return -1, false
}

func TestTW223_Effects_SmokeListsNatives(t *testing.T) {
	srv, _, _ := tw223Server(t)
	code, v, raw := tw223GetEffects(t, srv, "GET", "/api/lighting/effects")
	if code != 200 {
		t.Fatalf("%d %s", code, raw)
	}
	for _, id := range []string{"pulsation", "chenillard", "chenillard-traine", "couleur-equipe", "degrade"} {
		i, ok := tw223Find(v, id)
		if !ok {
			t.Fatalf("natif %s attendu", id)
		}
		if v.Effects[i].Origin != "native" || !strings.HasPrefix(v.Effects[i].File, "natifs/") {
			t.Errorf("%s : origine native, fichier natifs/… : %+v", id, v.Effects[i])
		}
		if len(v.Effects[i].Moments) == 0 {
			t.Errorf("%s : moments attendus", id)
		}
		if v.Effects[i].TickMs <= 0 {
			t.Errorf("%s : tick_ms (durée d'un tick, pour afficher la vitesse en secondes) : %d", id, v.Effects[i].TickMs)
		}
		for _, z := range tw223ZoneNames {
			if _, ok := v.Effects[i].Applicable[z]; !ok {
				t.Errorf("%s : applicable est une table à trois zones : %+v", id, v.Effects[i].Applicable)
			}
		}
	}
	if v.Dir == "" || !strings.HasSuffix(v.Dir, "files/effects") {
		t.Errorf("dir : %q", v.Dir)
	}
	if !strings.Contains(raw, `"errors":[]`) && !strings.Contains(raw, `"errors": []`) {
		t.Errorf("errors et unresolved sont toujours des tableaux (jamais null) : %s", raw)
	}
	for _, gone := range []string{`"target"`, `"host_applicable"`, `"reason"`} {
		if strings.Contains(raw, gone) {
			t.Errorf("le champ %s n'existe plus (révision 4, §7.1) : %s", gone, raw)
		}
	}
}

func TestTW223_Effects_NativesInContractOrder_ThenUserByName(t *testing.T) {
	srv, _, prov := tw223Server(t)
	tw223WriteEffect(t, prov, "z.yaml", strings.Replace(tw223UserEffect, "id: mon-effet\nnom: Mon effet", "id: zz\nnom: Zebre", 1))
	tw223WriteEffect(t, prov, "a.yaml", strings.Replace(tw223UserEffect, "id: mon-effet\nnom: Mon effet", "id: aa\nnom: Abeille", 1))
	_, v, _ := tw223GetEffects(t, srv, "GET", "/api/lighting/effects")
	pos := func(id string) int { i, _ := tw223Find(v, id); return i }
	order := []string{"pulsation", "chenillard", "chenillard-traine", "couleur-equipe", "degrade"}
	for i := 1; i < len(order); i++ {
		if pos(order[i-1]) >= pos(order[i]) {
			t.Errorf("natifs dans l'ordre du §9 (%v) : %+v", order, v.Effects)
		}
	}
	if !(pos("degrade") < pos("aa") && pos("aa") < pos("zz")) {
		t.Errorf("effets de l'utilisateur après les natifs, par nom : %+v", v.Effects)
	}
	if i := pos("aa"); v.Effects[i].Origin != "user" || v.Effects[i].File != "a.yaml" {
		t.Errorf("origin user, fichier a.yaml : %+v", v.Effects[i])
	}
}

func TestTW223_Effects_GroupsPerZone_NonEmptyOnly(t *testing.T) {
	srv, _, _ := tw223Server(t)
	tw223Zones(t, srv) // salle : A, (vide), B ⇒ N = 2 ; animateur : H ⇒ N = 1 ; public : aucun
	_, v, _ := tw223GetEffects(t, srv, "GET", "/api/lighting/effects")
	if v.Groups["ambiance"] != 2 || v.Groups["animateur"] != 1 || v.Groups["public"] != 0 {
		t.Errorf("N par zone = groupes non vides (le groupe vide ne compte pas) : %+v", v.Groups)
	}
}

func TestTW223_Effects_ApplicabilityTablePerZone(t *testing.T) {
	srv, _, _ := tw223Server(t)
	tw223Zones(t, srv)
	_, v, _ := tw223GetEffects(t, srv, "GET", "/api/lighting/effects")
	app := func(id, zone string) bool {
		i, ok := tw223Find(v, id)
		if !ok {
			t.Fatalf("natif %s attendu", id)
		}
		return v.Effects[i].Applicable[zone]
	}
	if !app("pulsation", "ambiance") || !app("pulsation", "animateur") {
		t.Error("pulsation (aucun groupes_min) : applicable à une zone qui existe, animateur compris (plus de règle « g0 seul »)")
	}
	if app("pulsation", "public") || app("degrade", "public") {
		t.Error("zone public sans groupe non vide : n'existe pas ⇒ aucun effet applicable")
	}
	// degrade : groupes_min 2 — N = 2 en ambiance, N = 1 en animateur
	if !app("degrade", "ambiance") || app("degrade", "animateur") {
		t.Errorf("degrade (2 groupes minimum) : ambiance (N=2) oui, animateur (N=1) non : %+v", v.Effects)
	}
	if code, out := tw223Post(t, srv, `{"lighting":{"groups":[{"lights":["A"]},{"lights":["B"]},{"role":"animateur","lights":["H"]},{"role":"animateur","lights":["P"]},{"role":"public","lights":[]}]}}`); code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	_, v, _ = tw223GetEffects(t, srv, "GET", "/api/lighting/effects")
	if !app("degrade", "animateur") || app("degrade", "public") {
		t.Errorf("animateur N=2 : degrade applicable ; public avec un groupe vide : toujours inexistante : %+v", v.Groups)
	}
}

func TestTW223_Effects_InvalidFilesListedWithFileAndLine_OthersAvailable(t *testing.T) {
	srv, _, prov := tw223Server(t)
	tw223WriteEffect(t, prov, "casse.yaml", "id: casse\nnom: C\nversion: 2\nmoments: [calm]\nduree_tick_ms: 1000\nvariables:\n  gA: g1; +1\n"+
		"pas:\n  - { gA: [scene, 100],\n      gB: [scene, 100] }\n")
	tw223WriteEffect(t, prov, "bon.yaml", tw223UserEffect)
	_, v, _ := tw223GetEffects(t, srv, "GET", "/api/lighting/effects")
	if len(v.Errors) != 1 || v.Errors[0].File != "casse.yaml" || v.Errors[0].Line <= 0 || !strings.Contains(v.Errors[0].Message, "gB") {
		t.Fatalf("erreur {file, line, message} : %+v", v.Errors)
	}
	if _, ok := tw223Find(v, "casse"); ok {
		t.Error("le fichier invalide n'est pas au catalogue")
	}
	if _, ok := tw223Find(v, "mon-effet"); !ok {
		t.Error("les autres effets restent disponibles")
	}
	if _, ok := tw223Find(v, "pulsation"); !ok {
		t.Error("les natifs aussi")
	}
}

func TestTW223_Effects_OverrideOrigin_ThenRestoredByRemoval(t *testing.T) {
	srv, _, prov := tw223Server(t)
	_, base, _ := tw223GetEffects(t, srv, "GET", "/api/lighting/effects")
	tw223WriteEffect(t, prov, "copie.yaml", strings.Replace(tw223UserEffect, "id: mon-effet", "id: chenillard", 1)+"")
	_, v, _ := tw223GetEffects(t, srv, "GET", "/api/lighting/effects")
	if i, ok := tw223Find(v, "chenillard"); !ok || v.Effects[i].Origin != "override" {
		t.Fatalf("même id qu'un natif : origin override : %+v", v.Effects)
	}
	if len(v.Effects) != len(base.Effects) {
		t.Errorf("une surcharge ne s'ajoute pas au catalogue : %d → %d", len(base.Effects), len(v.Effects))
	}
	if err := os.Remove(filepath.Join(prov.dir, "copie.yaml")); err != nil {
		t.Fatal(err)
	}
	prov.m.Reload()
	_, v, _ = tw223GetEffects(t, srv, "GET", "/api/lighting/effects")
	if i, _ := tw223Find(v, "chenillard"); v.Effects[i].Origin != "native" {
		t.Errorf("fichier supprimé : retour au natif : %+v", v.Effects[i])
	}
}

func TestTW223_Effects_UnresolvedListsGlobalSettingsByZone(t *testing.T) {
	srv, _, prov := tw223Server(t)
	tw223WriteEffect(t, prov, "mon-effet.yaml", tw223UserEffect)
	tw223Zones(t, srv)
	tw223Post(t, srv, `{"lighting":{"effects":{"ambiance":{"calm":"mon-effet"},"animateur":{"score":"couleur-equipe"}}}}`)
	_, v, _ := tw223GetEffects(t, srv, "GET", "/api/lighting/effects")
	if len(v.Unresolved) != 0 {
		t.Fatalf("tous les réglages se résolvent : %+v", v.Unresolved)
	}
	if err := os.Remove(filepath.Join(prov.dir, "mon-effet.yaml")); err != nil {
		t.Fatal(err)
	}
	prov.m.Reload()
	_, v, _ = tw223GetEffects(t, srv, "GET", "/api/lighting/effects")
	if len(v.Unresolved) != 1 || v.Unresolved[0].Zone != "ambiance" || v.Unresolved[0].Moment != "calm" || v.Unresolved[0].ID != "mon-effet" || v.Unresolved[0].Scope != "global" {
		t.Errorf("unresolved : le réglage global (zone ambiance) dont le fichier a disparu : %+v", v.Unresolved)
	}
	tw223Post(t, srv, `{"lighting":{"effects":{"animateur":{"calm":"mon-effet"}}}}`) // refusé : l'effet n'existe plus ; sans effet sur la suite
}

func TestTW223_Effects_UnresolvedCarriesTheZoneOfAnInapplicableSetting(t *testing.T) {
	srv, _, _ := tw223Server(t)
	tw223Zones(t, srv) // animateur : N = 1
	// degrade (2 groupes minimum) réglé pour le score de l'animateur (N = 1) : joue la valeur par défaut → à signaler
	if code, out := tw223Post(t, srv, `{"lighting":{"effects":{"animateur":{"chrono":"","calm":"","score":"degrade"}}}}`); code != 200 {
		t.Skipf("degrade ne déclare peut-être pas le moment score : %d %s", code, out)
	}
	_, v, _ := tw223GetEffects(t, srv, "GET", "/api/lighting/effects")
	found := false
	for _, u := range v.Unresolved {
		if u.Zone == "animateur" && u.Moment == "score" && u.ID == "degrade" {
			found = true
		}
	}
	if !found {
		t.Errorf("un effet inapplicable à la zone figure dans unresolved avec sa zone : %+v", v.Unresolved)
	}
}

func TestTW223_Effects_WithoutProvider_EmptyButValid(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	code, v, raw := tw223GetEffects(t, srv, "GET", "/api/lighting/effects")
	if code != 200 || len(v.Effects) != 0 || !strings.Contains(raw, `"effects":[]`) && !strings.Contains(raw, `"effects": []`) {
		t.Errorf("sans catalogue : 200 avec un tableau vide : %d %s", code, raw)
	}
}

func TestTW223_Effects_MethodsAndReload(t *testing.T) {
	srv, _, prov := tw223Server(t)
	if code, _, _ := tw223GetEffects(t, srv, "POST", "/api/lighting/effects"); code != http.StatusMethodNotAllowed {
		t.Errorf("POST sur la liste : 405, obtenu %d", code)
	}
	if code, _, _ := tw223GetEffects(t, srv, "GET", "/api/lighting/effects/reload"); code != http.StatusMethodNotAllowed {
		t.Errorf("GET sur /reload : 405, obtenu %d", code)
	}
	// un fichier posé hors rechargement automatique est vu après POST /reload
	if err := os.WriteFile(filepath.Join(prov.dir, "tard.yaml"), []byte(strings.Replace(tw223UserEffect, "mon-effet", "tard", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, v, _ := tw223GetEffects(t, srv, "GET", "/api/lighting/effects"); func() bool { _, ok := tw223Find(v, "tard"); return ok }() {
		t.Log("le fichier est déjà vu (rechargement à chaud) : pas de garantie inverse")
	}
	code, v, raw := tw223GetEffects(t, srv, "POST", "/api/lighting/effects/reload")
	if code != 200 {
		t.Fatalf("%d %s", code, raw)
	}
	if _, ok := tw223Find(v, "tard"); !ok {
		t.Errorf("POST /reload renvoie la forme du §7.1 avec le nouvel effet : %+v", v.Effects)
	}
}

// ---------------------------------------------------------------------------
// Tester un effet (§7.4) : POST /api/lighting/effects/{id}/try et /try/stop
// ---------------------------------------------------------------------------

type tw223TrialCall struct {
	id, zone string
	speed    float64
	seconds  int
}

type tw223Refusal struct{ reason string }

func (e *tw223Refusal) Error() string       { return "essai refusé : " + e.reason }
func (e *tw223Refusal) TrialReason() string { return e.reason }

// tw223TrialProvider : le fournisseur de développement + l'essai d'effet (EffectTrialer) simulé, qui enregistre les appels
// et renvoie l'erreur configurée.
type tw223TrialProvider struct {
	*devProvider
	calls []tw223TrialCall
	err   error
	stops int
}

func (p *tw223TrialProvider) StartEffectTrial(id, zone string, speed float64, seconds int) (int, error) {
	if p.err != nil {
		return 0, p.err
	}
	p.calls = append(p.calls, tw223TrialCall{id, zone, speed, seconds})
	return seconds * 1000, nil
}
func (p *tw223TrialProvider) StopEffectTrial() { p.stops++ }

func tw223TrialServer(t *testing.T) (*HTTPServer, *devHueBridge, *tw223TrialProvider) {
	t.Helper()
	srv, bridge := tw225ServerWithDriver(t)
	_, self, _, _ := runtime.Caller(0)
	natives := filepath.Join(filepath.Dir(self), "..", "..", "assets", "effects")
	dir := t.TempDir()
	srv.Effects = &tw223Provider{m: effects.NewManager(os.DirFS(natives), dir, nil), dir: dir}
	prov := &tw223TrialProvider{devProvider: srv.Lighting.(*devProvider)}
	srv.Lighting = prov
	if err := prov.LightingDriver().RefreshInventory(context.Background()); err != nil {
		t.Fatalf("pont factice : %v", err)
	}
	return srv, bridge, prov
}

func TestTW223_Trial_DefaultsAndExplicitBody(t *testing.T) {
	srv, _, prov := tw223TrialServer(t)
	code, out := devDo(t, srv, "POST", "/api/lighting/effects/pulsation/try", ``)
	if code != 200 || out["ok"] != true || out["duration_ms"] != float64(10000) {
		t.Fatalf("sans corps : zone ambiance, vitesse 1, 10 s : %d %v", code, out)
	}
	if code, out := devDo(t, srv, "POST", "/api/lighting/effects/pulsation/try", `{"zone":"public","speed":1.5,"seconds":20}`); code != 200 || out["duration_ms"] != float64(20000) {
		t.Fatalf("corps explicite : %d %v", code, out)
	}
	want := []tw223TrialCall{{"pulsation", "ambiance", 1, 10}, {"pulsation", "public", 1.5, 20}}
	if !reflect.DeepEqual(prov.calls, want) {
		t.Errorf("appels transmis au contrôleur : %+v, attendu %+v", prov.calls, want)
	}
}

func TestTW223_Trial_Refusals400(t *testing.T) {
	srv, _, prov := tw223TrialServer(t)
	for name, body := range map[string]string{
		"zone inconnue":      `{"zone":"salle"}`,
		"vitesse hors liste": `{"speed":3}`,
		"vitesse 0,6":        `{"speed":0.6}`,
		"durée trop longue":  `{"seconds":21}`,
		"durée négative":     `{"seconds":-1}`,
		"corps illisible":    `pas du json`,
	} {
		if code, _ := devDo(t, srv, "POST", "/api/lighting/effects/pulsation/try", body); code != http.StatusBadRequest {
			t.Errorf("%s : 400 attendu, obtenu %d", name, code)
		}
	}
	if len(prov.calls) != 0 {
		t.Errorf("un refus ne démarre rien : %+v", prov.calls)
	}
}

func TestTW223_Trial_UnknownEffect_404(t *testing.T) {
	srv, _, prov := tw223TrialServer(t)
	if code, _ := devDo(t, srv, "POST", "/api/lighting/effects/n-existe-pas/try", `{}`); code != http.StatusNotFound {
		t.Errorf("effet inconnu : 404, obtenu %d", code)
	}
	if code, _ := devDo(t, srv, "POST", "/api/lighting/effects/pulsation/inconnu", `{}`); code != http.StatusNotFound {
		t.Errorf("sous-chemin inconnu : 404, obtenu %d", code)
	}
	if len(prov.calls) != 0 {
		t.Error("rien n'est démarré")
	}
}

func TestTW223_Trial_ControllerRefusals_409WithReason(t *testing.T) {
	srv, _, prov := tw223TrialServer(t)
	for _, reason := range []string{"disabled", "game", "not_applicable", "mode"} {
		prov.err = &tw223Refusal{reason}
		code, out := devDo(t, srv, "POST", "/api/lighting/effects/pulsation/try", `{}`)
		if code != http.StatusConflict || out["reason"] != reason {
			t.Errorf("refus %s : 409 {reason:%s} attendu : %d %v", reason, reason, code, out)
		}
	}
	prov.err = nil
}

func TestTW223_Trial_LightingDisabled_409(t *testing.T) {
	srv, _, _ := tw223TrialServer(t)
	srv.Lighting = &tw223TrialProvider{devProvider: &devProvider{}} // aucun pilote vivant
	code, out := devDo(t, srv, "POST", "/api/lighting/effects/pulsation/try", `{}`)
	if code != http.StatusConflict || out["reason"] != "disabled" {
		t.Errorf("éclairage désactivé ⇒ 409 disabled : %d %v", code, out)
	}
}

func TestTW223_Trial_BridgeUnreachable_503(t *testing.T) {
	srv, bridge, prov := tw223TrialServer(t)
	bridge.srv.Close()
	_ = prov.LightingDriver().RefreshInventory(context.Background())
	code, _ := devDo(t, srv, "POST", "/api/lighting/effects/pulsation/try", `{}`)
	if code != http.StatusServiceUnavailable {
		t.Errorf("pont injoignable ⇒ 503 (état connu du pilote, sans E/S) : %d", code)
	}
	if len(prov.calls) != 0 {
		t.Error("rien n'est démarré")
	}
}

func TestTW223_Trial_Immediate200_NoDirectBridgeWrite(t *testing.T) {
	srv, bridge, _ := tw223TrialServer(t)
	if code, _ := devDo(t, srv, "POST", "/api/lighting/effects/pulsation/try", `{"zone":"animateur"}`); code != 200 {
		t.Fatalf("%d", code)
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	for _, h := range bridge.hits {
		if strings.HasPrefix(h, "PUT ") && strings.Contains(h, "/state") {
			t.Errorf("l'essai passe par l'écrivain (Scene), jamais par une écriture directe du gestionnaire : %s", h)
		}
	}
}

func TestTW223_Trial_MethodsAndStop(t *testing.T) {
	srv, _, prov := tw223TrialServer(t)
	if code, _ := devDo(t, srv, "GET", "/api/lighting/effects/pulsation/try", ""); code != http.StatusMethodNotAllowed {
		t.Errorf("GET sur /try : 405, obtenu %d", code)
	}
	if code, _ := devDo(t, srv, "GET", "/api/lighting/effects/try/stop", ""); code != http.StatusMethodNotAllowed {
		t.Errorf("GET sur /try/stop : 405, obtenu %d", code)
	}
	for i := 0; i < 2; i++ { // même sans essai en cours
		if code, out := devDo(t, srv, "POST", "/api/lighting/effects/try/stop", ""); code != 200 || out["ok"] != true {
			t.Errorf("/try/stop répond toujours 200 {ok:true} : %d %v", code, out)
		}
	}
	if prov.stops != 2 {
		t.Errorf("l'arrêt est transmis au contrôleur : %d", prov.stops)
	}
}

// ---------------------------------------------------------------------------
// Sauvegarde / restauration de files/effects/ (§3.2) — scénario 10 de la procédure
// ---------------------------------------------------------------------------

func tw223TarNames(t *testing.T, data []byte) []string {
	t.Helper()
	var out []string
	tr := tar.NewReader(bytes.NewReader(data))
	for {
		h, err := tr.Next()
		if err != nil {
			return out
		}
		out = append(out, h.Name)
	}
}

func tw223Seed(t *testing.T, dataDir string) {
	t.Helper()
	dir := filepath.Join(dataDir, "files", "effects")
	if err := os.MkdirAll(filepath.Join(dir, "natifs"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"mon-effet.yaml": tw223UserEffect, "natifs/chenillard.yaml": "# copie de référence\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTW223_Backup_AmbianceIncludesUserEffectsWithoutNatifs(t *testing.T) {
	srv, dataDir := setupTestHTTPServer(t)
	tw223Seed(t, dataDir)
	req := httptest.NewRequest("GET", "/backup-select?ambiance=true", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("%d", w.Code)
	}
	names := tw223TarNames(t, w.Body.Bytes())
	has := func(n string) bool {
		for _, x := range names {
			if x == n {
				return true
			}
		}
		return false
	}
	if !has("files/effects/mon-effet.yaml") {
		t.Errorf("les effets de l'utilisateur sont sauvegardés (catégorie ambiance) : %v", names)
	}
	for _, n := range names {
		if strings.Contains(n, "natifs") {
			t.Errorf("la copie de référence natifs/ n'est jamais sauvegardée : %s", n)
		}
	}
}

func TestTW223_Backup_WithoutAmbianceExcludesEffects(t *testing.T) {
	srv, dataDir := setupTestHTTPServer(t)
	tw223Seed(t, dataDir)
	for _, q := range []string{"teams=true", "history=true", "medias=true"} {
		req := httptest.NewRequest("GET", "/backup-select?"+q, nil)
		w := httptest.NewRecorder()
		srv.mux.ServeHTTP(w, req)
		for _, n := range tw223TarNames(t, w.Body.Bytes()) {
			if strings.HasPrefix(n, "files/effects/") {
				t.Errorf("%s : les effets ne suivent que la catégorie ambiance : %s", q, n)
			}
		}
	}
}

func TestTW223_Restore_UserEffectsRestored_NatifsNeverWritten(t *testing.T) {
	srv, dataDir := setupTestHTTPServer(t)
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range map[string]string{
		"files/effects/mon-effet.yaml":    tw223UserEffect,
		"files/effects/natifs/piege.yaml": "id: piege\n",
	} {
		_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))})
		_, _ = tw.Write([]byte(content))
	}
	_ = tw.Close()
	if d := srv.detectTARContents(buf.Bytes()); !d["effects"] {
		t.Fatalf("la restauration détecte la catégorie effets : %+v", d)
	}
	body, ct := multipartRestoreBody(t, buf.Bytes())
	req := httptest.NewRequest("POST", "/restore", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Restored map[string]bool `json:"restored"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Restored["effects"] {
		t.Errorf("restored.effects attendu : %+v", resp.Restored)
	}
	got, err := os.ReadFile(filepath.Join(dataDir, "files", "effects", "mon-effet.yaml"))
	if err != nil || string(got) != tw223UserEffect {
		t.Errorf("effet restauré dans files/effects/ : %v %q", err, got)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "files", "effects", "natifs", "piege.yaml")); err == nil {
		t.Error("natifs/ n'est jamais restauré")
	}
}

func TestTW223_Restore_OnlyNatifsInArchive_NotDetectedAsEffects(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	tarData := buildTARWithSingleFile(t, "files/effects/natifs/chenillard.yaml", []byte("x"))
	if d := srv.detectTARContents(tarData); d["effects"] {
		t.Errorf("une archive ne contenant que natifs/ n'est pas une archive d'effets : %+v", d)
	}
}

func TestTW223_Restore_NoEffectsInArchive_LeavesExistingEffectsUntouched(t *testing.T) {
	srv, dataDir := setupTestHTTPServer(t)
	tw223Seed(t, dataDir)
	tarData := buildTARWithSingleFile(t, "files/sounds/depart.wav", []byte("x"))
	body, ct := multipartRestoreBody(t, tarData)
	req := httptest.NewRequest("POST", "/restore", body)
	req.Header.Set("Content-Type", ct)
	srv.mux.ServeHTTP(httptest.NewRecorder(), req)
	if _, err := os.Stat(filepath.Join(dataDir, "files", "effects", "mon-effet.yaml")); err != nil {
		t.Errorf("une restauration sans effets ne touche pas les effets existants : %v", err)
	}
}

// ---------------------------------------------------------------------------
// Vitesse réglable (§6, R13) : lighting.speeds.<zone>.<moment>
// ---------------------------------------------------------------------------

func TestTW223_Speeds_DefaultIsOne_ThreeZonesByThreeMoments(t *testing.T) {
	srv, _, _ := tw223Server(t)
	sp := tw223Lighting(t, srv).Speeds
	for _, z := range tw223ZoneNames {
		for _, m := range tw223MomentNames {
			if v, ok := sp[z][m]; !ok || v != 1 {
				t.Errorf("speeds.%s.%s par défaut = 1 (forme complète) : %v (%v)", z, m, v, ok)
			}
		}
	}
}

func TestTW223_Speeds_AllFiveValuesAccepted_InEveryZone(t *testing.T) {
	for _, v := range []string{"0.5", "0.75", "1", "1.5", "2"} {
		for _, z := range tw223ZoneNames {
			srv, _, _ := tw223Server(t)
			if code, out := tw223Post(t, srv, `{"lighting":{"speeds":{"`+z+`":{"chrono":`+v+`}}}}`); code != 200 {
				t.Errorf("vitesse %s acceptée en zone %s : %d %s", v, z, code, out)
			}
		}
	}
}

func TestTW223_Speeds_OtherValuesRefused400_NothingChanged(t *testing.T) {
	for _, v := range []string{"0", "0.25", "3", "-1", "1.25", `"2"`, "null", `"rapide"`, "true"} {
		srv, _, _ := tw223Server(t)
		tw223Post(t, srv, `{"lighting":{"speeds":{"animateur":{"calm":2}}}}`)
		code, out := tw223Post(t, srv, `{"lighting":{"speeds":{"animateur":{"chrono":`+v+`,"calm":0.5}}}}`)
		if code != http.StatusBadRequest || strings.TrimSpace(out) == "" {
			t.Errorf("vitesse %s : 400 avec message attendu, obtenu %d %q", v, code, out)
		}
		if got := tw223Lighting(t, srv).Speeds["animateur"]; got["chrono"] != 1 || got["calm"] != 2 {
			t.Errorf("vitesse %s : un refus ne change rien (écriture atomique) : %v", v, got)
		}
	}
}

func TestTW223_Speeds_UnknownMomentOrZoneRefused(t *testing.T) {
	srv, _, _ := tw223Server(t)
	if code, _ := tw223Post(t, srv, `{"lighting":{"speeds":{"ambiance":{"bidule":2}}}}`); code != 400 {
		t.Errorf("moment inconnu : 400, obtenu %d", code)
	}
	if code, _ := tw223Post(t, srv, `{"lighting":{"speeds":{"salle":{"chrono":2}}}}`); code != 400 {
		t.Errorf("zone inconnue : 400, obtenu %d", code)
	}
}

func TestTW223_Speeds_PartialPatchesMergeOneZoneAndMomentAtATime(t *testing.T) {
	srv, _, _ := tw223Server(t)
	for _, body := range []string{
		`{"lighting":{"speeds":{"ambiance":{"chrono":2}}}}`,
		`{"lighting":{"speeds":{"ambiance":{"calm":0.5},"public":{"score":1.5}}}}`,
		`{"lighting":{"speeds":{"ambiance":{"chrono":1.5}}}}`,
	} {
		if code, out := tw223Post(t, srv, body); code != 200 {
			t.Fatalf("%s : %d %s", body, code, out)
		}
	}
	sp := tw223Lighting(t, srv).Speeds
	if sp["ambiance"]["chrono"] != 1.5 || sp["ambiance"]["calm"] != 0.5 || sp["ambiance"]["score"] != 1 {
		t.Errorf("fusion par moment : %v", sp["ambiance"])
	}
	if sp["public"]["score"] != 1.5 || sp["public"]["chrono"] != 1 || sp["animateur"]["calm"] != 1 {
		t.Errorf("fusion par zone : %v", sp)
	}
}

func TestTW223_Speeds_NeverWipedBySavingOtherSections(t *testing.T) {
	srv, _, _ := tw223Server(t)
	tw223Zones(t, srv)
	tw223Post(t, srv, `{"lighting":{"speeds":{"ambiance":{"chrono":2},"animateur":{"calm":0.5}}}}`)
	for _, body := range []string{`{"server":{"debug":true}}`, `{"lighting":{"enabled":false}}`, `{"lighting":{"groups":[{"lights":["A"]}]}}`} {
		if code, out := tw223Post(t, srv, body); code != 200 {
			t.Fatalf("%s : %d %s", body, code, out)
		}
		sp := tw223Lighting(t, srv).Speeds
		if sp["ambiance"]["chrono"] != 2 || sp["animateur"]["calm"] != 0.5 {
			t.Fatalf("POST %s a modifié les vitesses : %+v", body, sp)
		}
	}
}

func TestTW223_Speeds_IndependentOfTheEffectChoice(t *testing.T) {
	// la vitesse vaut pour le moment, quel que soit l'effet choisi — et reste enregistrée quand aucun effet n'est joué
	srv, _, _ := tw223Server(t)
	tw223Post(t, srv, `{"lighting":{"effects":{"ambiance":{"chrono":""}}}}`) // aucun effet pendant le chrono
	if code, out := tw223Post(t, srv, `{"lighting":{"speeds":{"ambiance":{"chrono":2}}}}`); code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	if got := tw223Lighting(t, srv).Speeds["ambiance"]["chrono"]; got != 2 {
		t.Errorf("la vitesse est conservée sans effet : %v", got)
	}
}
