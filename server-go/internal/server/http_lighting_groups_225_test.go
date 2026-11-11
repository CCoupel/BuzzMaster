package server

// Tests de spécification v11.7.0 LOT-1 (#225) — groupes d'ambiance : validation
// de lighting.groups via POST /config.json, champ `group` de
// GET /api/lighting/lights, POST /api/lighting/groups/test.
// Écrits depuis contracts/light-effects.md §2.1, §2.4, §2.5 et
// contracts/http-endpoints.md ; ils ne dépendent d'aucun nom interne de
// dev-backend (tout passe par le JSON HTTP), sauf config.Get().Lighting pour
// l'état persisté. Aides préfixées tw225, réutilise devHueBridge/devProvider
// de http_lighting_dev_test.go.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/lighting/hue"
)

// tw225Post envoie un POST /config.json et rend (code, corps).
func tw225Post(t *testing.T, srv *HTTPServer, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest("POST", "/config.json", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

// tw225Seed enregistre quatre ampoules (2 générales de salle, 1 d'équipe, 1 générale libre).
func tw225Seed(t *testing.T, srv *HTTPServer) {
	t.Helper()
	code, out := tw225Post(t, srv, `{"lighting":{"enabled":false,"lights":[`+
		`{"name":"L1","role":"general"},{"name":"L2","role":"general"},{"name":"L3","role":"general"},`+
		`{"name":"T1","role":"team","team":"Rouges"}]}}`)
	if code != http.StatusOK {
		t.Fatalf("seed lighting : %d %s", code, out)
	}
}

type tw225Group struct {
	Name      string   `json:"name"`
	Role      string   `json:"role"`
	Lights    []string `json:"lights"`
	NearTeams []string `json:"near_teams"`
}

func tw225GetGroups(t *testing.T, srv *HTTPServer) []tw225Group {
	t.Helper()
	req := httptest.NewRequest("GET", "/config.json", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /config.json : %d", w.Code)
	}
	var parsed struct {
		Lighting struct {
			Groups *[]tw225Group `json:"groups"`
		} `json:"lighting"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("JSON invalide : %v", err)
	}
	if parsed.Lighting.Groups == nil {
		t.Fatal("lighting.groups doit toujours être sérialisé (jamais absent ni null) — règle « pas d'omitempty »")
	}
	return *parsed.Lighting.Groups
}

// ---------------------------------------------------------------------------
// §2.1 — valeurs par défaut et aller-retour
// ---------------------------------------------------------------------------

func TestTW225_Config_NoGroups_SerialisedAsEmptyArray(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	if got := tw225GetGroups(t, srv); len(got) != 0 {
		t.Fatalf("sans groupe : tableau vide attendu, obtenu %+v", got)
	}
}

func TestTW225_Config_GroupsRoundTrip_OrderRoleDefaultAndNearTeams(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	tw225Seed(t, srv)
	code, out := tw225Post(t, srv, `{"lighting":{"groups":[`+
		`{"name":"Scène gauche","lights":["L1"],"near_teams":["Rouges"]},`+
		`{"name":"Pupitre","role":"animateur","lights":["L2"]},`+
		`{"name":"Bar","role":"salle","lights":["L3"]}]}}`)
	if code != http.StatusOK {
		t.Fatalf("POST groups : %d %s", code, out)
	}
	got := tw225GetGroups(t, srv)
	if len(got) != 3 {
		t.Fatalf("3 groupes attendus : %+v", got)
	}
	if got[0].Name != "Scène gauche" || got[1].Name != "Pupitre" || got[2].Name != "Bar" {
		t.Errorf("l'ordre du tableau doit être conservé : %+v", got)
	}
	if got[0].Role != "salle" {
		t.Errorf("rôle absent ⇒ \"salle\", obtenu %q", got[0].Role)
	}
	if got[1].Role != "animateur" {
		t.Errorf("rôle animateur perdu : %q", got[1].Role)
	}
	if len(got[0].NearTeams) != 1 || got[0].NearTeams[0] != "Rouges" {
		t.Errorf("near_teams perdu : %+v", got[0].NearTeams)
	}
	if got[1].NearTeams == nil || got[2].NearTeams == nil {
		t.Error("near_teams doit être sérialisé en [] et non null")
	}
}

func TestTW225_Config_GroupsPersistedInMemoryConfig(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	tw225Seed(t, srv)
	if code, out := tw225Post(t, srv, `{"lighting":{"groups":[{"name":"G","lights":["L1","L2"]}]}}`); code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	g := config.Get().Lighting.Groups
	if len(g) != 1 || len(g[0].Lights) != 2 {
		t.Fatalf("groupes non persistés dans la config : %+v", g)
	}
}

func TestTW225_Config_EmptyGroup_Accepted(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	tw225Seed(t, srv)
	if code, out := tw225Post(t, srv, `{"lighting":{"groups":[{"name":"Vide","lights":[]}]}}`); code != 200 {
		t.Fatalf("un groupe vide est accepté (§2.1) : %d %s", code, out)
	}
}

func TestTW225_Config_ReplacementIsComplete(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	tw225Seed(t, srv)
	tw225Post(t, srv, `{"lighting":{"groups":[{"name":"A","lights":["L1"]},{"name":"B","lights":["L2"]}]}}`)
	if code, out := tw225Post(t, srv, `{"lighting":{"groups":[{"name":"C","lights":["L3"]}]}}`); code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	got := tw225GetGroups(t, srv)
	if len(got) != 1 || got[0].Name != "C" {
		t.Fatalf("remplacement complet du tableau attendu : %+v", got)
	}
	if code, _ := tw225Post(t, srv, `{"lighting":{"groups":[]}}`); code != 200 {
		t.Fatal("groups: [] doit être accepté (suppression de tous les groupes)")
	}
	if got := tw225GetGroups(t, srv); len(got) != 0 {
		t.Fatalf("groups: [] doit tout supprimer : %+v", got)
	}
}

func TestTW225_Config_SavingOtherSectionsNeverWipesGroups(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	tw225Seed(t, srv)
	tw225Post(t, srv, `{"lighting":{"groups":[{"name":"A","lights":["L1"]}]}}`)
	for _, body := range []string{`{"server":{"debug":true}}`, `{"lighting":{"enabled":false}}`, `{"lighting":{"lights":[{"name":"L1","role":"general"},{"name":"L2","role":"general"},{"name":"L3","role":"general"},{"name":"T1","role":"team","team":"Rouges"}]}}`} {
		if code, out := tw225Post(t, srv, body); code != 200 {
			t.Fatalf("%s : %d %s", body, code, out)
		}
		if got := tw225GetGroups(t, srv); len(got) != 1 || len(got[0].Lights) != 1 {
			t.Fatalf("POST %s a modifié lighting.groups : %+v", body, got)
		}
	}
}

// ---------------------------------------------------------------------------
// §2.1 — refus 400 lisibles, et rien n'est modifié en cas de refus
// ---------------------------------------------------------------------------

func TestTW225_Config_GroupsValidation_Refusals(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"ampoule inconnue", `{"lighting":{"groups":[{"name":"A","lights":["Inconnue"]}]}}`},
		{"ampoule de rôle équipe", `{"lighting":{"groups":[{"name":"A","lights":["T1"]}]}}`},
		{"ampoule dans deux groupes", `{"lighting":{"groups":[{"name":"A","lights":["L1"]},{"name":"B","lights":["L1"]}]}}`},
		{"nom en double", `{"lighting":{"groups":[{"name":"A","lights":["L1"]},{"name":"A","lights":["L2"]}]}}`},
		{"nom en double insensible à la casse", `{"lighting":{"groups":[{"name":"Bar","lights":["L1"]},{"name":"BAR","lights":["L2"]}]}}`},
		{"nom vide", `{"lighting":{"groups":[{"name":"","lights":["L1"]}]}}`},
		{"nom de plus de 40 caractères", `{"lighting":{"groups":[{"name":"` + strings.Repeat("x", 41) + `","lights":["L1"]}]}}`},
		{"rôle inconnu", `{"lighting":{"groups":[{"name":"A","role":"vip","lights":["L1"]}]}}`},
		{"deux groupes animateur", `{"lighting":{"groups":[{"name":"A","role":"animateur","lights":["L1"]},{"name":"B","role":"animateur","lights":["L2"]}]}}`},
		{"near_teams sur l'animateur", `{"lighting":{"groups":[{"name":"A","role":"animateur","lights":["L1"],"near_teams":["Rouges"]}]}}`},
		{"équipe proche de deux groupes", `{"lighting":{"groups":[{"name":"A","lights":["L1"],"near_teams":["Rouges"]},{"name":"B","lights":["L2"],"near_teams":["Rouges"]}]}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, _ := setupTestHTTPServer(t)
			tw225Seed(t, srv)
			tw225Post(t, srv, `{"lighting":{"groups":[{"name":"Avant","lights":["L3"]}]}}`)
			code, out := tw225Post(t, srv, c.body)
			if code != http.StatusBadRequest {
				t.Fatalf("400 attendu, obtenu %d : %s", code, out)
			}
			if strings.TrimSpace(out) == "" {
				t.Error("le refus doit porter un message lisible")
			}
			got := tw225GetGroups(t, srv)
			if len(got) != 1 || got[0].Name != "Avant" {
				t.Errorf("un refus ne doit rien modifier : %+v", got)
			}
		})
	}
}

func TestTW225_Config_GroupsLimit_TwelveAcceptedThirteenRefused(t *testing.T) {
	build := func(n int) string {
		parts := make([]string, n)
		for i := range parts {
			parts[i] = `{"name":"G` + string(rune('A'+i)) + `","lights":[]}`
		}
		return `{"lighting":{"groups":[` + strings.Join(parts, ",") + `]}}`
	}
	srv, _ := setupTestHTTPServer(t)
	tw225Seed(t, srv)
	if code, out := tw225Post(t, srv, build(12)); code != 200 {
		t.Fatalf("12 groupes acceptés : %d %s", code, out)
	}
	if code, _ := tw225Post(t, srv, build(13)); code != 400 {
		t.Fatalf("13 groupes refusés avec 400, obtenu %d", code)
	}
}

func TestTW225_Config_OneHostGroupAccepted_WithoutNumbering(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	tw225Seed(t, srv)
	if code, out := tw225Post(t, srv, `{"lighting":{"groups":[{"name":"Pupitre","role":"animateur","lights":["L1"]}]}}`); code != 200 {
		t.Fatalf("un groupe animateur seul est valide : %d %s", code, out)
	}
}

// ---------------------------------------------------------------------------
// §2.1 — retrait automatique : ampoule passée en rôle team, ou retirée
// ---------------------------------------------------------------------------

func TestTW225_Config_LightSwitchedToTeam_LeavesItsGroupInSameWrite(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	tw225Seed(t, srv)
	tw225Post(t, srv, `{"lighting":{"groups":[{"name":"A","lights":["L1","L2"]}]}}`)
	code, out := tw225Post(t, srv, `{"lighting":{"lights":[{"name":"L1","role":"team","team":"Rouges"},`+
		`{"name":"L2","role":"general"},{"name":"L3","role":"general"},{"name":"T1","role":"team","team":"Rouges"}]}}`)
	if code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	got := tw225GetGroups(t, srv)
	if len(got) != 1 || len(got[0].Lights) != 1 || got[0].Lights[0] != "L2" {
		t.Fatalf("L1 passée en équipe doit quitter son groupe : %+v", got)
	}
}

func TestTW225_Config_LightRemoved_LeavesItsGroup_GroupKept(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	tw225Seed(t, srv)
	tw225Post(t, srv, `{"lighting":{"groups":[{"name":"A","lights":["L1"]},{"name":"B","lights":["L2"]}]}}`)
	code, out := tw225Post(t, srv, `{"lighting":{"lights":[{"name":"L2","role":"general"},{"name":"L3","role":"general"}]}}`)
	if code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	got := tw225GetGroups(t, srv)
	if len(got) != 2 || len(got[0].Lights) != 0 || len(got[1].Lights) != 1 {
		t.Fatalf("L1 retirée disparaît de A ; le groupe A (vide) est conservé : %+v", got)
	}
}

func TestTW225_Config_LightsAndGroupsInSameWrite_GroupsValidatedAgainstNewLights(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	code, out := tw225Post(t, srv, `{"lighting":{"lights":[{"name":"N1","role":"general"},{"name":"N2","role":"general"}],`+
		`"groups":[{"name":"A","lights":["N1","N2"]}]}}`)
	if code != 200 {
		t.Fatalf("groupes validés contre la liste d'ampoules de la même écriture : %d %s", code, out)
	}
}

// ---------------------------------------------------------------------------
// §2.4 — GET /api/lighting/lights : champ `group`
// ---------------------------------------------------------------------------

func tw225ServerWithDriver(t *testing.T) (*HTTPServer, *devHueBridge) {
	t.Helper()
	srv, _ := setupTestHTTPServer(t)
	bridge := newDevHueBridge(t) // une ampoule : BuzzHue1
	d, err := hue.New(hue.Config{BridgeIP: bridge.srv.URL, BridgeID: bridge.bridgeID, APIKey: bridge.key,
		Lights: []hue.LightSpec{{Name: "BuzzHue1"}},
		FindBridge: func(_ context.Context, _ string, _ time.Duration) (hue.Bridge, bool, error) {
			return hue.Bridge{}, false, nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	srv.Lighting = &devProvider{d: d}
	return srv, bridge
}

func tw225LightsGroupField(t *testing.T, srv *HTTPServer) any {
	t.Helper()
	code, out := devDo(t, srv, "GET", "/api/lighting/lights", "")
	if code != 200 {
		t.Fatalf("GET /api/lighting/lights : %d %v", code, out)
	}
	lights, _ := out["lights"].([]any)
	if len(lights) != 1 {
		t.Fatalf("1 ampoule attendue : %v", out)
	}
	l := lights[0].(map[string]any)
	v, ok := l["group"]
	if !ok {
		t.Fatalf("chaque ampoule doit porter `group` (0 = hors groupe) : %v", l)
	}
	return v
}

func TestTW225_Lights_GroupField_ZeroWhenUngrouped(t *testing.T) {
	srv, _ := tw225ServerWithDriver(t)
	cfg := config.Get()
	cfg.Lighting.Lights = []config.LightingLightEntry{{Name: "BuzzHue1", Role: "general"}}
	config.SetInstance(cfg)
	if v := tw225LightsGroupField(t, srv); v != float64(0) {
		t.Fatalf("hors groupe ⇒ group = 0, obtenu %v", v)
	}
}

func TestTW225_Lights_GroupField_RankAmongSalleGroups_HostIsString(t *testing.T) {
	srv, _ := tw225ServerWithDriver(t)
	// Le groupe animateur en tête ne compte pas dans la numérotation : « Salle » est le groupe 1.
	cfg := config.Get()
	cfg.Lighting.Lights = []config.LightingLightEntry{{Name: "BuzzHue1", Role: "general"}}
	cfg.Lighting.Groups = []config.LightingGroupEntry{
		{Name: "Pupitre", Role: "animateur", Lights: []string{}, NearTeams: []string{}},
		{Name: "Salle", Role: "salle", Lights: []string{"BuzzHue1"}, NearTeams: []string{}},
	}
	config.SetInstance(cfg)
	if v := tw225LightsGroupField(t, srv); v != float64(1) {
		t.Fatalf("numéro = rang parmi les groupes de salle (animateur exclu) : attendu 1, obtenu %v", v)
	}

	cfg = config.Get()
	cfg.Lighting.Groups = []config.LightingGroupEntry{
		{Name: "Pupitre", Role: "animateur", Lights: []string{"BuzzHue1"}, NearTeams: []string{}},
	}
	config.SetInstance(cfg)
	if v := tw225LightsGroupField(t, srv); v != "host" {
		t.Fatalf("ampoule du groupe animateur ⇒ group = \"host\", obtenu %v", v)
	}
}

// ---------------------------------------------------------------------------
// §2.4 — POST /api/lighting/groups/test
// ---------------------------------------------------------------------------

func tw225ConfigureGroups(groups ...config.LightingGroupEntry) {
	cfg := config.Get()
	cfg.Lighting.Lights = []config.LightingLightEntry{{Name: "BuzzHue1", Role: "general"}}
	cfg.Lighting.Groups = groups
	config.SetInstance(cfg)
}

func TestTW225_GroupsTest_MethodNotAllowed(t *testing.T) {
	srv, _ := tw225ServerWithDriver(t)
	if code, _ := devDo(t, srv, "GET", "/api/lighting/groups/test", ""); code != http.StatusMethodNotAllowed {
		t.Fatalf("GET refusé en 405, obtenu %d", code)
	}
}

func TestTW225_GroupsTest_Refusals400(t *testing.T) {
	srv, _ := tw225ServerWithDriver(t)
	tw225ConfigureGroups(
		config.LightingGroupEntry{Name: "Salle", Role: "salle", Lights: []string{"BuzzHue1"}, NearTeams: []string{}},
		config.LightingGroupEntry{Name: "Vide", Role: "salle", Lights: []string{}, NearTeams: []string{}},
	)
	for name, body := range map[string]string{
		"numéro inconnu":               `{"group": 9}`,
		"numéro 0":                     `{"group": 0}`,
		"host sans groupe animateur":   `{"group": "host"}`,
		"corps sans groupe":            `{}`,
		"corps illisible":              `pas du json`,
		"groupe vide (aucune ampoule)": `{"group": 2}`,
		"chaîne autre que host":        `{"group": "salle"}`,
	} {
		if code, out := devDo(t, srv, "POST", "/api/lighting/groups/test", body); code != http.StatusBadRequest {
			t.Errorf("%s : 400 attendu, obtenu %d %v", name, code, out)
		}
	}
}

func TestTW225_GroupsTest_NoDriver_409(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	tw225ConfigureGroups(config.LightingGroupEntry{Name: "Salle", Role: "salle", Lights: []string{"BuzzHue1"}, NearTeams: []string{}})
	code, _ := devDo(t, srv, "POST", "/api/lighting/groups/test", `{"group": 1}`)
	if code != http.StatusConflict {
		t.Fatalf("éclairage désactivé / sans pilote ⇒ 409, obtenu %d", code)
	}
}

func TestTW225_GroupsTest_OK_NumberAndHost_OnlyGuardedPaths(t *testing.T) {
	srv, bridge := tw225ServerWithDriver(t)
	tw225ConfigureGroups(
		config.LightingGroupEntry{Name: "Salle", Role: "salle", Lights: []string{"BuzzHue1"}, NearTeams: []string{}},
	)
	code, out := devDo(t, srv, "POST", "/api/lighting/groups/test", `{"group": 1}`)
	if code != 200 || out["ok"] != true {
		t.Fatalf("essai du groupe 1 : %d %v", code, out)
	}
	tw225ConfigureGroups(
		config.LightingGroupEntry{Name: "Pupitre", Role: "animateur", Lights: []string{"BuzzHue1"}, NearTeams: []string{}},
	)
	code, out = devDo(t, srv, "POST", "/api/lighting/groups/test", `{"group": "host"}`)
	if code != 200 || out["ok"] != true {
		t.Fatalf("essai du groupe animateur : %d %v", code, out)
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	puts := 0
	for _, h := range bridge.hits {
		method, path, _ := strings.Cut(h, " ")
		if err := hue.GuardRequest(method, path); err != nil {
			t.Errorf("requête refusée par la garde : %s (%v)", h, err)
		}
		if method == "PUT" && strings.Contains(path, "/lights/8/state") {
			puts++
		}
	}
	if puts < 2 {
		t.Errorf("chaque essai écrit l'ampoule (clignotement puis restauration), écritures observées : %d", puts)
	}
}
