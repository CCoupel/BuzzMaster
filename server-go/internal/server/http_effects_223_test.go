package server

// #223 (v11.7.0, LOT-2) — POST /config.json (effets par moment : fusion, validation 400),
// GET /api/lighting/effects, POST /api/lighting/effects/reload, sauvegarde / restauration de files/effects/.
// Source : contracts/light-effects.md §3.2, §6, §7.1, §7.2 ; ambiance-settings.md §10.
// Fournisseur de catalogue réel (effects.Manager : natifs du dépôt + dossier utilisateur temporaire).
// Aides préfixées tw223 ; noms de tests TestTW223_* (scénarios 1 et 10 de light-effects-223.md).

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// tw223Hosts crée deux groupes animateur d'id « ha » et « hb » (et leurs ampoules) pour les réglages par groupe.
func tw223Hosts(t *testing.T, srv *HTTPServer) {
	t.Helper()
	code, out := tw223Post(t, srv, `{"lighting":{"lights":[{"name":"P","role":"general"},{"name":"R","role":"general"}],`+
		`"groups":[{"id":"ha","role":"animateur","lights":["P"]},{"id":"hb","role":"animateur","lights":["R"]}]}}`)
	if code != 200 {
		t.Fatalf("groupes animateur : %d %s", code, out)
	}
}

type tw223Settings struct {
	Effects     map[string]string            `json:"effects"`
	HostEffects map[string]map[string]string `json:"host_effects"` // id de groupe animateur → moment → effet
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
// POST /config.json — effets par moment
// ---------------------------------------------------------------------------

func TestTW223_Config_Defaults_V1160Behaviour(t *testing.T) {
	srv, _, _ := tw223Server(t)
	l := tw223Lighting(t, srv)
	if l.Effects["chrono"] != "pulsation" || l.Effects["calm"] != "" || l.Effects["score"] != "" {
		t.Errorf("défauts salle : chrono=pulsation, calm et score vides : %+v", l.Effects)
	}
	if len(l.HostEffects) != 0 {
		t.Errorf("aucun groupe animateur ⇒ host_effects = {} : %+v", l.HostEffects)
	}
	if len(l.Effects) != 3 {
		t.Errorf("les trois clés de effects sont toujours sérialisées : %+v", l)
	}
}

func TestTW223_Config_PartialPatchesMergeOneMomentAtATime(t *testing.T) {
	srv, _, prov := tw223Server(t)
	tw223WriteEffect(t, prov, "mon-effet.yaml", tw223UserEffect)
	for _, body := range []string{
		`{"lighting":{"effects":{"chrono":"chenillard"}}}`,
		`{"lighting":{"effects":{"calm":"mon-effet"}}}`,
		`{"lighting":{"effects":{"score":"couleur-equipe"}}}`,
	} {
		if code, out := tw223Post(t, srv, body); code != 200 {
			t.Fatalf("%s : %d %s", body, code, out)
		}
	}
	l := tw223Lighting(t, srv)
	if l.Effects["chrono"] != "chenillard" || l.Effects["calm"] != "mon-effet" || l.Effects["score"] != "couleur-equipe" {
		t.Errorf("chaque patch ne doit écraser que son moment : %+v", l.Effects)
	}
	// et un patch ultérieur d'un seul moment garde les deux autres
	if code, out := tw223Post(t, srv, `{"lighting":{"effects":{"chrono":"pulsation"}}}`); code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	l = tw223Lighting(t, srv)
	if l.Effects["chrono"] != "pulsation" || l.Effects["calm"] != "mon-effet" || l.Effects["score"] != "couleur-equipe" {
		t.Errorf("fusion par moment : %+v", l.Effects)
	}
}

func TestTW223_Config_HostEffectsMergeByGroupIdAndMoment(t *testing.T) {
	srv, _, _ := tw223Server(t)
	tw223Hosts(t, srv)
	for _, body := range []string{
		`{"lighting":{"host_effects":{"ha":{"score":"couleur-equipe"}}}}`,
		`{"lighting":{"host_effects":{"hb":{"chrono":"pulsation"}}}}`,
		`{"lighting":{"host_effects":{"ha":{"chrono":"pulsation"}}}}`,
		`{"lighting":{"effects":{"chrono":"chenillard"}}}`,
	} {
		if code, out := tw223Post(t, srv, body); code != 200 {
			t.Fatalf("%s : %d %s", body, code, out)
		}
	}
	l := tw223Lighting(t, srv)
	if l.HostEffects["ha"]["score"] != "couleur-equipe" || l.HostEffects["ha"]["chrono"] != "pulsation" {
		t.Errorf("groupe ha : chaque patch ne touche que son moment : %+v", l.HostEffects["ha"])
	}
	if l.HostEffects["hb"]["chrono"] != "pulsation" || l.HostEffects["hb"]["score"] == "couleur-equipe" {
		t.Errorf("groupe hb indépendant de ha : %+v", l.HostEffects["hb"])
	}
	if l.Effects["chrono"] != "chenillard" {
		t.Errorf("effects (salle) : %+v", l.Effects)
	}
}

func TestTW223_Config_HostEffects_UnknownGroupId_DroppedWithoutError(t *testing.T) {
	srv, _, _ := tw223Server(t)
	tw223Hosts(t, srv)
	code, out := tw223Post(t, srv, `{"lighting":{"host_effects":{"disparu":{"score":"couleur-equipe"},"ha":{"calm":""}}}}`)
	if code != 200 {
		t.Fatalf("un id de groupe inconnu est écarté sans erreur (exception voulue §6) : %d %s", code, out)
	}
	l := tw223Lighting(t, srv)
	if _, ok := l.HostEffects["disparu"]; ok {
		t.Errorf("l'entrée d'un groupe inexistant n'est pas conservée : %+v", l.HostEffects)
	}
}

func TestTW223_Config_HostEffects_OldFlatFormRefusedAtWrite(t *testing.T) {
	srv, _, _ := tw223Server(t)
	tw223Hosts(t, srv)
	for _, body := range []string{
		`{"lighting":{"host_effects":{"score":"couleur-equipe"}}}`,
		`{"lighting":{"host_effects":{"chrono":"","calm":"","score":""}}}`,
	} {
		if code, _ := tw223Post(t, srv, body); code != http.StatusBadRequest {
			t.Errorf("l'ancienne forme à plat n'est plus acceptée à l'écriture : %s → %d", body, code)
		}
	}
}

func TestTW223_Config_HostEffects_FullFormHasOneEntryPerHostGroup_AfterSavingGroupsPurgesOthers(t *testing.T) {
	srv, _, _ := tw223Server(t)
	tw223Hosts(t, srv)
	tw223Post(t, srv, `{"lighting":{"host_effects":{"ha":{"score":"couleur-equipe"},"hb":{"score":"couleur-equipe"}}}}`)
	// on supprime le groupe hb : sa clé disparaît de host_effects, celle de ha reste
	if code, out := tw223Post(t, srv, `{"lighting":{"groups":[{"id":"ha","role":"animateur","lights":["P"]}]}}`); code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	l := tw223Lighting(t, srv)
	if _, ok := l.HostEffects["hb"]; ok {
		t.Errorf("enregistrer lighting.groups purge host_effects des id disparus : %+v", l.HostEffects)
	}
	if l.HostEffects["ha"]["score"] != "couleur-equipe" {
		t.Errorf("l'entrée du groupe conservé reste : %+v", l.HostEffects)
	}
}

func TestTW223_Config_EmptyIdMeansNoEffect_Accepted(t *testing.T) {
	srv, _, _ := tw223Server(t)
	if code, out := tw223Post(t, srv, `{"lighting":{"effects":{"chrono":""}}}`); code != 200 {
		t.Fatalf("« aucun effet » accepté : %d %s", code, out)
	}
	if got := tw223Lighting(t, srv).Effects["chrono"]; got != "" {
		t.Errorf("\"\" remplace « pulsation » (aucune animation pendant le chrono) : %q", got)
	}
}

func TestTW223_Config_Refusals400_NothingChanged(t *testing.T) {
	cases := map[string]string{
		"moment inconnu":                     `{"lighting":{"effects":{"bidule":"chenillard"}}}`,
		"id inexistant":                      `{"lighting":{"effects":{"chrono":"n-existe-pas"}}}`,
		"effet qui ne déclare pas le moment": `{"lighting":{"effects":{"calm":"chenillard"}}}`,
		"animateur : moment inconnu":         `{"lighting":{"host_effects":{"ha":{"bidule":"couleur-equipe"}}}}`,
		"animateur : id d'effet inexistant":  `{"lighting":{"host_effects":{"ha":{"calm":"n-existe-pas"}}}}`,
		"animateur : mauvais moment":         `{"lighting":{"host_effects":{"ha":{"chrono":"couleur-equipe"}}}}`,
		"un bon moment et un mauvais":        `{"lighting":{"effects":{"chrono":"chenillard","calm":"chenillard"}}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			srv, _, _ := tw223Server(t)
			tw223Hosts(t, srv)
			before := tw223Lighting(t, srv)
			code, out := tw223Post(t, srv, body)
			if code != http.StatusBadRequest {
				t.Fatalf("400 attendu, obtenu %d : %s", code, out)
			}
			if strings.TrimSpace(out) == "" {
				t.Error("message lisible attendu")
			}
			after := tw223Lighting(t, srv)
			if before.Effects["chrono"] != after.Effects["chrono"] || before.Effects["calm"] != after.Effects["calm"] {
				t.Errorf("un refus ne modifie rien (écriture atomique) : %+v → %+v", before, after)
			}
		})
	}
}

func TestTW223_Config_WithoutCatalogue_NonEmptyIdRefused(t *testing.T) {
	srv, _ := setupTestHTTPServer(t) // aucun fournisseur de catalogue
	if code, _ := tw223Post(t, srv, `{"lighting":{"effects":{"calm":"chenillard"}}}`); code != 400 {
		t.Errorf("sans catalogue, un id est introuvable : 400, obtenu %d", code)
	}
	if code, _ := tw223Post(t, srv, `{"lighting":{"effects":{"calm":""}}}`); code != 200 {
		t.Errorf("« aucun effet » reste valide sans catalogue : %d", code)
	}
}

func TestTW223_Config_SavingOtherSectionsNeverWipesEffects(t *testing.T) {
	srv, _, _ := tw223Server(t)
	tw223Hosts(t, srv)
	tw223Post(t, srv, `{"lighting":{"effects":{"chrono":"chenillard"},"host_effects":{"ha":{"score":"couleur-equipe"}}}}`)
	for _, body := range []string{`{"server":{"debug":true}}`, `{"lighting":{"enabled":false}}`} {
		if code, out := tw223Post(t, srv, body); code != 200 {
			t.Fatalf("%s : %d %s", body, code, out)
		}
		l := tw223Lighting(t, srv)
		if l.Effects["chrono"] != "chenillard" || l.HostEffects["ha"]["score"] != "couleur-equipe" {
			t.Fatalf("POST %s a modifié les effets : %+v", body, l)
		}
	}
}

func TestTW223_Config_EffectBecomingAbsentLater_DoesNotInvalidateTheConfig(t *testing.T) {
	srv, _, prov := tw223Server(t)
	tw223WriteEffect(t, prov, "mon-effet.yaml", tw223UserEffect)
	if code, out := tw223Post(t, srv, `{"lighting":{"effects":{"calm":"mon-effet"}}}`); code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	if err := os.Remove(filepath.Join(prov.dir, "mon-effet.yaml")); err != nil {
		t.Fatal(err)
	}
	prov.m.Reload()
	if code, out := tw223Post(t, srv, `{"server":{"debug":false}}`); code != 200 {
		t.Fatalf("une configuration enregistrée reste valide quand l'effet disparaît : %d %s", code, out)
	}
	if got := tw223Lighting(t, srv).Effects["calm"]; got != "mon-effet" {
		t.Errorf("la valeur est conservée telle quelle : %q", got)
	}
}

// ---------------------------------------------------------------------------
// GET /api/lighting/effects (§7.1)
// ---------------------------------------------------------------------------

type tw223EffectsView struct {
	Groups  int `json:"groups"`
	Effects []struct {
		ID             string   `json:"id"`
		Name           string   `json:"name"`
		Moments        []string `json:"moments"`
		Origin         string   `json:"origin"`
		File           string   `json:"file"`
		MinGroups      int      `json:"min_groups"`
		Applicable     bool     `json:"applicable"`
		Reason         string   `json:"reason"`
		HostApplicable bool     `json:"host_applicable"`
	} `json:"effects"`
	Errors []struct {
		File    string
		Line    int
		Message string
	} `json:"errors"`
	Unresolved []struct {
		Moment string `json:"moment"`
		ID     string `json:"id"`
		Scope  string `json:"scope"`
		Host   bool   `json:"host"`
		Group  string `json:"group"`
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
	}
	if v.Dir == "" || !strings.HasSuffix(v.Dir, "files/effects") {
		t.Errorf("dir : %q", v.Dir)
	}
	if !strings.Contains(raw, `"errors":[]`) && !strings.Contains(raw, `"errors": []`) {
		t.Errorf("errors et unresolved sont toujours des tableaux (jamais null) : %s", raw)
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

func TestTW223_Effects_MinGroupsHostApplicableAndNoTargetField(t *testing.T) {
	srv, _, _ := tw223Server(t)
	_, v, raw := tw223GetEffects(t, srv, "GET", "/api/lighting/effects")
	if strings.Contains(raw, `"target"`) {
		t.Errorf("le champ `target` est supprimé (révision 3, §7.1) : %s", raw)
	}
	want := map[string]struct {
		min  int
		host bool
	}{
		"pulsation": {0, true}, "chenillard": {3, false}, "chenillard-traine": {5, false}, "couleur-equipe": {0, true}, "degrade": {2, false},
	}
	for id, w := range want {
		i, ok := tw223Find(v, id)
		if !ok {
			t.Fatalf("natif %s attendu", id)
		}
		if e := v.Effects[i]; e.MinGroups != w.min || e.HostApplicable != w.host {
			t.Errorf("%s : min_groups = groupes_min (%d), host_applicable = n'écrit que g0 (%v) : %+v", id, w.min, w.host, e)
		}
	}
	pu, _ := tw223Find(v, "pulsation")
	if e := v.Effects[pu]; !e.Applicable {
		t.Errorf("pulsation applicable sans groupe : %+v", e)
	}
}

func TestTW223_Effects_ApplicabilityFollowsNonEmptyRoomGroups(t *testing.T) {
	srv, _, _ := tw223Server(t)
	if code, out := tw223Post(t, srv, `{"lighting":{"lights":[{"name":"A","role":"general"},{"name":"B","role":"general"},{"name":"C","role":"general"},{"name":"H","role":"general"}],`+
		`"groups":[{"lights":["A"]},{"lights":[]},{"lights":["B"]},{"role":"animateur","lights":["H"]}]}}`); code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	_, v, _ := tw223GetEffects(t, srv, "GET", "/api/lighting/effects")
	if v.Groups != 2 {
		t.Fatalf("N = groupes de salle non vides (le groupe vide et l'animateur ne comptent pas) : %d", v.Groups)
	}
	ch, _ := tw223Find(v, "chenillard")
	if e := v.Effects[ch]; e.Applicable || e.Reason == "" {
		t.Errorf("chenillard (3 groupes) avec N = 2 : inapplicable avec une raison : %+v", e)
	}
	if code, out := tw223Post(t, srv, `{"lighting":{"groups":[{"lights":["A"]},{"lights":["B"]},{"lights":["C"]}]}}`); code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	_, v, _ = tw223GetEffects(t, srv, "GET", "/api/lighting/effects")
	ch, _ = tw223Find(v, "chenillard")
	if v.Groups != 3 || !v.Effects[ch].Applicable {
		t.Errorf("N = 3 : chenillard applicable : %+v", v)
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

func TestTW223_Effects_UnresolvedListsGlobalSettingsOnly(t *testing.T) {
	srv, _, prov := tw223Server(t)
	tw223WriteEffect(t, prov, "mon-effet.yaml", tw223UserEffect)
	if code, out := tw223Post(t, srv, `{"lighting":{"lights":[{"name":"A","role":"general"},{"name":"P","role":"general"}],`+
		`"groups":[{"lights":["A"]},{"id":"ha","role":"animateur","lights":["P"]}]}}`); code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	tw223Post(t, srv, `{"lighting":{"effects":{"calm":"mon-effet"},"host_effects":{"ha":{"score":"couleur-equipe"}}}}`)
	_, v, _ := tw223GetEffects(t, srv, "GET", "/api/lighting/effects")
	if len(v.Unresolved) != 0 {
		t.Fatalf("tous les réglages se résolvent : %+v", v.Unresolved)
	}
	if err := os.Remove(filepath.Join(prov.dir, "mon-effet.yaml")); err != nil {
		t.Fatal(err)
	}
	prov.m.Reload()
	_, v, _ = tw223GetEffects(t, srv, "GET", "/api/lighting/effects")
	if len(v.Unresolved) != 1 || v.Unresolved[0].Moment != "calm" || v.Unresolved[0].ID != "mon-effet" || v.Unresolved[0].Scope != "global" || v.Unresolved[0].Host {
		t.Errorf("unresolved : le réglage global dont le fichier a disparu : %+v", v.Unresolved)
	}
}

func TestTW223_Effects_UnresolvedHostEntryCarriesGroupId(t *testing.T) {
	srv, _, prov := tw223Server(t)
	tw223WriteEffect(t, prov, "mon-effet.yaml", tw223UserEffect)
	tw223Hosts(t, srv)
	if code, out := tw223Post(t, srv, `{"lighting":{"host_effects":{"hb":{"calm":"mon-effet"}}}}`); code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	if err := os.Remove(filepath.Join(prov.dir, "mon-effet.yaml")); err != nil {
		t.Fatal(err)
	}
	prov.m.Reload()
	_, v, _ := tw223GetEffects(t, srv, "GET", "/api/lighting/effects")
	if len(v.Unresolved) != 1 || !v.Unresolved[0].Host || v.Unresolved[0].Group != "hb" || v.Unresolved[0].Moment != "calm" {
		t.Errorf("unresolved d'un groupe animateur : host=true et group = id du groupe : %+v", v.Unresolved)
	}
}

func TestTW223_Effects_UnresolvedInapplicableHostSetting(t *testing.T) {
	srv, _, _ := tw223Server(t)
	// chenillard (3 groupes) réglé pour le chrono salle sans groupe : joue la valeur par défaut → à signaler
	tw223Post(t, srv, `{"lighting":{"effects":{"chrono":"chenillard"}}}`)
	_, v, _ := tw223GetEffects(t, srv, "GET", "/api/lighting/effects")
	found := false
	for _, u := range v.Unresolved {
		if u.Moment == "chrono" && u.ID == "chenillard" && !u.Host {
			found = true
		}
	}
	if !found {
		t.Errorf("un effet inapplicable (N=0 < 3) figure dans unresolved : %+v", v.Unresolved)
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
// Vitesse réglable (§6, R13) : lighting.speeds.<moment> et lighting.host_speeds.<id>.<moment>
// ---------------------------------------------------------------------------

type tw223Speeds struct {
	Speeds     map[string]float64            `json:"speeds"`
	HostSpeeds map[string]map[string]float64 `json:"host_speeds"`
}

func tw223GetSpeeds(t *testing.T, srv *HTTPServer) tw223Speeds {
	t.Helper()
	req := httptest.NewRequest("GET", "/config.json", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	var parsed struct {
		Lighting tw223Speeds `json:"lighting"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("JSON invalide : %v", err)
	}
	return parsed.Lighting
}

func TestTW223_Speeds_DefaultIsOne_HostSpeedsEmptyWithoutHostGroup(t *testing.T) {
	srv, _, _ := tw223Server(t)
	sp := tw223GetSpeeds(t, srv)
	for _, m := range []string{"chrono", "calm", "score"} {
		// config.json : une vitesse absente vaut 1 (« absents de config.json ⇒ vitesse 1 partout », §6) ; la forme complète
		// (trois moments toujours présents) est celle de l'API des réglages (AMBIANCE_EFFECTIVE), testée dans cmd/server.
		if v, ok := sp.Speeds[m]; ok && v != 1 {
			t.Errorf("speeds.%s par défaut = 1 : %v", m, v)
		}
	}
	if sp.HostSpeeds == nil || len(sp.HostSpeeds) != 0 {
		t.Errorf("aucun groupe animateur ⇒ host_speeds = {} : %v", sp.HostSpeeds)
	}
}

func TestTW223_Speeds_AllFiveValuesAccepted(t *testing.T) {
	for _, v := range []string{"0.5", "0.75", "1", "1.5", "2"} {
		srv, _, _ := tw223Server(t)
		if code, out := tw223Post(t, srv, `{"lighting":{"speeds":{"chrono":`+v+`}}}`); code != 200 {
			t.Errorf("vitesse %s acceptée : %d %s", v, code, out)
		}
	}
}

func TestTW223_Speeds_OtherValuesRefused400_NothingChanged(t *testing.T) {
	for _, v := range []string{"0", "0.25", "3", "-1", "1.25", `"2"`, "null", `"rapide"`, "true"} {
		srv, _, _ := tw223Server(t)
		tw223Post(t, srv, `{"lighting":{"speeds":{"calm":2}}}`)
		code, out := tw223Post(t, srv, `{"lighting":{"speeds":{"chrono":`+v+`,"calm":0.5}}}`)
		if code != http.StatusBadRequest || strings.TrimSpace(out) == "" {
			t.Errorf("vitesse %s : 400 avec message attendu, obtenu %d %q", v, code, out)
		}
		if got := tw223GetSpeeds(t, srv).Speeds; got["chrono"] != 1 || got["calm"] != 2 {
			t.Errorf("vitesse %s : un refus ne change rien (écriture atomique) : %v", v, got)
		}
	}
}

func TestTW223_Speeds_UnknownMomentRefused(t *testing.T) {
	srv, _, _ := tw223Server(t)
	if code, _ := tw223Post(t, srv, `{"lighting":{"speeds":{"bidule":2}}}`); code != 400 {
		t.Errorf("moment inconnu : 400, obtenu %d", code)
	}
}

func TestTW223_Speeds_PartialPatchesMergeOneMomentAtATime(t *testing.T) {
	srv, _, _ := tw223Server(t)
	for _, body := range []string{`{"lighting":{"speeds":{"chrono":2}}}`, `{"lighting":{"speeds":{"calm":0.5}}}`, `{"lighting":{"speeds":{"chrono":1.5}}}`} {
		if code, out := tw223Post(t, srv, body); code != 200 {
			t.Fatalf("%s : %d %s", body, code, out)
		}
	}
	sp := tw223GetSpeeds(t, srv).Speeds
	if sp["chrono"] != 1.5 || sp["calm"] != 0.5 || sp["score"] != 1 {
		t.Errorf("fusion par moment : %v", sp)
	}
}

func TestTW223_HostSpeeds_ByGroupId_MergedPerMoment_UnknownIdDroppedWithoutError(t *testing.T) {
	srv, _, _ := tw223Server(t)
	tw223Hosts(t, srv)
	for _, body := range []string{
		`{"lighting":{"host_speeds":{"ha":{"score":2}}}}`,
		`{"lighting":{"host_speeds":{"hb":{"chrono":0.75}}}}`,
		`{"lighting":{"host_speeds":{"ha":{"chrono":1.5},"disparu":{"calm":2}}}}`,
	} {
		if code, out := tw223Post(t, srv, body); code != 200 {
			t.Fatalf("%s : %d %s", body, code, out)
		}
	}
	hs := tw223GetSpeeds(t, srv).HostSpeeds
	if hs["ha"]["score"] != 2 || hs["ha"]["chrono"] != 1.5 || hs["hb"]["chrono"] != 0.75 {
		t.Errorf("host_speeds par id et par moment : %v", hs)
	}
	if _, ok := hs["disparu"]; ok {
		t.Errorf("l'entrée d'un groupe inexistant est écartée sans erreur : %v", hs)
	}
}

func TestTW223_HostSpeeds_InvalidValueRefused_And_PurgedWithTheGroup(t *testing.T) {
	srv, _, _ := tw223Server(t)
	tw223Hosts(t, srv)
	if code, _ := tw223Post(t, srv, `{"lighting":{"host_speeds":{"ha":{"chrono":3}}}}`); code != 400 {
		t.Errorf("vitesse hors liste pour un groupe animateur : 400, obtenu %d", code)
	}
	tw223Post(t, srv, `{"lighting":{"host_speeds":{"ha":{"chrono":2},"hb":{"chrono":2}}}}`)
	if code, out := tw223Post(t, srv, `{"lighting":{"groups":[{"id":"ha","role":"animateur","lights":["P"]}]}}`); code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	hs := tw223GetSpeeds(t, srv).HostSpeeds
	if _, ok := hs["hb"]; ok {
		t.Errorf("enregistrer lighting.groups purge host_speeds des id disparus : %v", hs)
	}
	if hs["ha"]["chrono"] != 2 {
		t.Errorf("l'entrée du groupe conservé reste : %v", hs)
	}
}

func TestTW223_Speeds_NeverWipedBySavingOtherSections(t *testing.T) {
	srv, _, _ := tw223Server(t)
	tw223Hosts(t, srv)
	tw223Post(t, srv, `{"lighting":{"speeds":{"chrono":2},"host_speeds":{"ha":{"calm":0.5}}}}`)
	for _, body := range []string{`{"server":{"debug":true}}`, `{"lighting":{"enabled":false}}`} {
		if code, out := tw223Post(t, srv, body); code != 200 {
			t.Fatalf("%s : %d %s", body, code, out)
		}
		sp := tw223GetSpeeds(t, srv)
		if sp.Speeds["chrono"] != 2 || sp.HostSpeeds["ha"]["calm"] != 0.5 {
			t.Fatalf("POST %s a modifié les vitesses : %+v", body, sp)
		}
	}
}

func TestTW223_Speeds_IndependentOfTheEffectChoice(t *testing.T) {
	// la vitesse vaut pour le moment, quel que soit l'effet choisi — et reste enregistrée quand aucun effet n'est joué
	srv, _, _ := tw223Server(t)
	tw223Post(t, srv, `{"lighting":{"effects":{"chrono":""}}}`) // aucun effet pendant le chrono
	if code, out := tw223Post(t, srv, `{"lighting":{"speeds":{"chrono":2}}}`); code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	if got := tw223GetSpeeds(t, srv).Speeds["chrono"]; got != 2 {
		t.Errorf("la vitesse est conservée sans effet : %v", got)
	}
}
