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
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
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
		{"cinq groupes animateur", `{"lighting":{"groups":[` + strings.Join(tw225Hosts(5), ",") + `]}}`},
		{"id mal formé (majuscules)", `{"lighting":{"groups":[{"id":"ABC","name":"A","lights":["L1"]}]}}`},
		{"id mal formé (trop long)", `{"lighting":{"groups":[{"id":"abcdefghijklm","name":"A","lights":["L1"]}]}}`},
		{"id mal formé (caractères)", `{"lighting":{"groups":[{"id":"a-b","name":"A","lights":["L1"]}]}}`},
		{"id en double", `{"lighting":{"groups":[{"id":"x1","name":"A","lights":["L1"]},{"id":"x1","name":"B","lights":["L2"]}]}}`},
		{"id en double entre salle et animateur", `{"lighting":{"groups":[{"id":"x1","name":"A","lights":["L1"]},{"id":"x1","name":"B","role":"animateur","lights":["L2"]}]}}`},
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

// tw225Hosts fabrique n groupes animateur vides (noms distincts).
func tw225Hosts(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = `{"name":"Anim` + string(rune('A'+i)) + `","role":"animateur","lights":[]}`
	}
	return out
}

func tw225GetGroupsRaw(t *testing.T, srv *HTTPServer) []map[string]any {
	t.Helper()
	req := httptest.NewRequest("GET", "/config.json", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	var parsed struct {
		Lighting struct {
			Groups []map[string]any `json:"groups"`
		} `json:"lighting"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	return parsed.Lighting.Groups
}

// ---------------------------------------------------------------------------
// Révision du GATE LOT-1 : plusieurs groupes animateur, plafonds 12 / 4, `id` stable
// ---------------------------------------------------------------------------

func TestTW225_Config_SeveralHostGroups_UpToFourAccepted(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	tw225Seed(t, srv)
	for n := 1; n <= 4; n++ {
		if code, out := tw225Post(t, srv, `{"lighting":{"groups":[`+strings.Join(tw225Hosts(n), ",")+`]}}`); code != 200 {
			t.Fatalf("%d groupes animateur acceptés : %d %s", n, code, out)
		}
	}
	if got := tw225GetGroups(t, srv); len(got) != 4 {
		t.Fatalf("4 groupes animateur conservés : %+v", got)
	}
}

func TestTW225_Config_Caps_TwelveRoomAndFourHostIndependent(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	tw225Seed(t, srv)
	room := make([]string, 12)
	for i := range room {
		room[i] = `{"name":"S` + string(rune('A'+i)) + `","lights":[]}`
	}
	all := append(append([]string{}, room...), tw225Hosts(4)...)
	if code, out := tw225Post(t, srv, `{"lighting":{"groups":[`+strings.Join(all, ",")+`]}}`); code != 200 {
		t.Fatalf("12 groupes de salle + 4 groupes animateur : les deux plafonds sont indépendants : %d %s", code, out)
	}
	over := append(append([]string{}, room...), `{"name":"Trop","lights":[]}`)
	if code, _ := tw225Post(t, srv, `{"lighting":{"groups":[`+strings.Join(over, ",")+`]}}`); code != 400 {
		t.Errorf("13 groupes de salle refusés : %d", code)
	}
}

func TestTW225_Config_ID_AssignedByServerWhenAbsent_UniqueAndWellFormed(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	tw225Seed(t, srv)
	if code, out := tw225Post(t, srv, `{"lighting":{"groups":[{"name":"A","lights":["L1"]},{"name":"P","role":"animateur","lights":["L2"]},{"name":"Q","role":"animateur","lights":[]}]}}`); code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	re := regexp.MustCompile(`^[a-z0-9]{1,12}$`)
	seen := map[string]bool{}
	for _, g := range tw225GetGroupsRaw(t, srv) {
		id, _ := g["id"].(string)
		if !re.MatchString(id) {
			t.Errorf("id attribué [a-z0-9]{1,12} : %q (%v)", id, g)
		}
		if seen[id] {
			t.Errorf("ids uniques : %q en double", id)
		}
		seen[id] = true
	}
	if len(seen) != 3 {
		t.Errorf("un id par groupe : %v", seen)
	}
}

func TestTW225_Config_ID_StableAcrossRenameAndReorder_ClientSendsItBack(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	tw225Seed(t, srv)
	tw225Post(t, srv, `{"lighting":{"groups":[{"name":"A","lights":["L1"]},{"name":"B","lights":["L2"]},{"name":"P","role":"animateur","lights":["L3"]}]}}`)
	first := tw225GetGroupsRaw(t, srv)
	idOf := map[string]string{}
	for _, g := range first {
		idOf[g["name"].(string)], _ = g["id"].(string)
	}
	// renommer + réordonner + déplacer une ampoule, en renvoyant les id
	body := `{"lighting":{"groups":[` +
		`{"id":"` + idOf["B"] + `","name":"B renommé","lights":["L1"]},` +
		`{"id":"` + idOf["A"] + `","name":"A","lights":["L2"]},` +
		`{"id":"` + idOf["P"] + `","name":"Pupitre","role":"animateur","lights":["L3"]}]}}`
	if code, out := tw225Post(t, srv, body); code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	for _, g := range tw225GetGroupsRaw(t, srv) {
		name := g["name"].(string)
		switch name {
		case "B renommé":
			if g["id"] != idOf["B"] {
				t.Errorf("renommer et déplacer ne change pas l'id : %v", g)
			}
		case "A":
			if g["id"] != idOf["A"] {
				t.Errorf("réordonner ne change pas l'id : %v", g)
			}
		case "Pupitre":
			if g["id"] != idOf["P"] {
				t.Errorf("l'id d'un groupe animateur est stable : %v", g)
			}
		}
	}
}

func TestTW225_Config_ID_KeptWhenSavingOtherSections(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	tw225Seed(t, srv)
	tw225Post(t, srv, `{"lighting":{"groups":[{"name":"P","role":"animateur","lights":["L1"]}]}}`)
	before := tw225GetGroupsRaw(t, srv)[0]["id"]
	tw225Post(t, srv, `{"server":{"debug":true}}`)
	if after := tw225GetGroupsRaw(t, srv)[0]["id"]; after != before || before == "" {
		t.Errorf("id conservé : %v → %v", before, after)
	}
}

func TestTW225_Config_SavingGroups_PurgesHostEffectsOfVanishedIds(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	tw225Seed(t, srv)
	tw225Post(t, srv, `{"lighting":{"groups":[{"id":"ga1","name":"P","role":"animateur","lights":["L1"]},{"id":"gb2","name":"Q","role":"animateur","lights":["L2"]}]}}`)
	cfg := config.Get()
	cfg.Lighting.HostEffects = map[string]map[string]string{
		"ga1": {"chrono": "", "calm": "", "score": ""}, "gb2": {"chrono": "", "calm": "", "score": ""},
	}
	config.SetInstance(cfg)
	// on supprime le groupe gb2
	if code, out := tw225Post(t, srv, `{"lighting":{"groups":[{"id":"ga1","name":"P","role":"animateur","lights":["L1"]}]}}`); code != 200 {
		t.Fatalf("%d %s", code, out)
	}
	he := config.Get().Lighting.HostEffects
	if _, ok := he["gb2"]; ok {
		t.Errorf("enregistrer lighting.groups purge host_effects des id disparus : %v", he)
	}
	if _, ok := he["ga1"]; !ok {
		t.Errorf("l'entrée du groupe conservé reste : %v", he)
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

// tw225Provider : le fournisseur de développement + l'essai de groupe (GroupTester) simulé, qui reproduit la résolution
// de référence de la production (n = salle, "host<k>" = animateur de rang k ; inconnu ou vide ⇒ erreur) et enregistre les appels.
type tw225Provider struct {
	*devProvider
	started []string
	stops   int
}

func (p *tw225Provider) CheckGroup(ref string) error {
	lc := config.Get().Lighting
	if k, ok := strings.CutPrefix(ref, "host"); ok {
		rank, err := strconv.Atoi(k)
		if err != nil || rank < 1 {
			return errors.New("groupe inconnu")
		}
		hosts := lc.HostGroups()
		if rank > len(hosts) {
			return errors.New("groupe inconnu")
		}
		if len(hosts[rank-1].Lights) == 0 {
			return errors.New("groupe vide")
		}
		return nil
	}
	n, err := strconv.Atoi(ref)
	if err != nil || n < 1 {
		return errors.New("groupe inconnu")
	}
	rank := 0
	for _, g := range lc.Groups {
		if g.Role == config.GroupRoleAnimateur {
			continue
		}
		rank++
		if rank == n {
			if len(g.Lights) == 0 {
				return errors.New("groupe vide")
			}
			return nil
		}
	}
	return errors.New("groupe inconnu")
}
func (p *tw225Provider) StartGroupTest(ref string) error {
	if err := p.CheckGroup(ref); err != nil {
		return err
	}
	p.started = append(p.started, ref)
	return nil
}
func (p *tw225Provider) GroupTestDurationMs() int { return 6000 }
func (p *tw225Provider) StopGroupTest()           { p.stops++ }

// tw225ServerWithTester : serveur + pilote (pont factice) + essai de groupe simulé.
func tw225ServerWithTester(t *testing.T) (*HTTPServer, *devHueBridge, *tw225Provider) {
	t.Helper()
	srv, bridge := tw225ServerWithDriver(t)
	prov := &tw225Provider{devProvider: srv.Lighting.(*devProvider)}
	srv.Lighting = prov
	// le pilote ne connaît l'état du pont (ok) qu'après un premier échange, comme en production après le démarrage
	if err := prov.LightingDriver().RefreshInventory(context.Background()); err != nil {
		t.Fatalf("pont factice : %v", err)
	}
	return srv, bridge, prov
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

func TestTW225_Lights_GroupField_RankAmongSalleGroups_HostIsStringWithRank(t *testing.T) {
	srv, _ := tw225ServerWithDriver(t)
	// Les groupes animateur ne comptent pas dans la numérotation de salle : « Salle » est le groupe 1.
	cfg := config.Get()
	cfg.Lighting.Lights = []config.LightingLightEntry{{Name: "BuzzHue1", Role: "general"}}
	cfg.Lighting.Groups = []config.LightingGroupEntry{
		{ID: "ha", Name: "Pupitre", Role: "animateur", Lights: []string{}, NearTeams: []string{}},
		{ID: "sa", Name: "Salle", Role: "salle", Lights: []string{"BuzzHue1"}, NearTeams: []string{}},
	}
	config.SetInstance(cfg)
	if v := tw225LightsGroupField(t, srv); v != float64(1) {
		t.Fatalf("numéro = rang parmi les groupes de salle (animateur exclu) : attendu 1, obtenu %v", v)
	}
	// Premier groupe animateur : "host1".
	cfg = config.Get()
	cfg.Lighting.Groups = []config.LightingGroupEntry{
		{ID: "ha", Name: "Pupitre", Role: "animateur", Lights: []string{"BuzzHue1"}, NearTeams: []string{}},
	}
	config.SetInstance(cfg)
	if v := tw225LightsGroupField(t, srv); v != "host1" {
		t.Fatalf("ampoule du groupe animateur de rang 1 ⇒ group = \"host1\", obtenu %v", v)
	}
	// Second groupe animateur (le premier, vide, garde son rang) : "host2".
	cfg = config.Get()
	cfg.Lighting.Groups = []config.LightingGroupEntry{
		{ID: "ha", Name: "Pupitre", Role: "animateur", Lights: []string{}, NearTeams: []string{}},
		{ID: "hb", Name: "Régie", Role: "animateur", Lights: []string{"BuzzHue1"}, NearTeams: []string{}},
	}
	config.SetInstance(cfg)
	if v := tw225LightsGroupField(t, srv); v != "host2" {
		t.Fatalf("rang propre des animateurs, vides compris : \"host2\", obtenu %v", v)
	}
}

func TestTW225_Lights_TeamBulbCarriesZero(t *testing.T) {
	srv, _ := tw225ServerWithDriver(t)
	cfg := config.Get()
	cfg.Lighting.Lights = []config.LightingLightEntry{{Name: "BuzzHue1", Role: "team", Team: "Rouges"}}
	cfg.Lighting.Groups = []config.LightingGroupEntry{}
	config.SetInstance(cfg)
	if v := tw225LightsGroupField(t, srv); v != float64(0) {
		t.Fatalf("une ampoule d'équipe porte 0 : %v", v)
	}
}

// ---------------------------------------------------------------------------
// §2.4 — POST /api/lighting/groups/test
// ---------------------------------------------------------------------------

func tw225ConfigureGroups(groups ...config.LightingGroupEntry) {
	cfg := config.Get()
	cfg.Lighting.Enabled = true
	cfg.Lighting.Lights = []config.LightingLightEntry{{Name: "BuzzHue1", Role: "general"}}
	cfg.Lighting.Groups = groups
	config.SetInstance(cfg)
}

func tw225Salle(lights ...string) config.LightingGroupEntry {
	return config.LightingGroupEntry{ID: "s1", Name: "Salle", Role: "salle", Lights: lights, NearTeams: []string{}}
}

func tw225Anim(id string, lights ...string) config.LightingGroupEntry {
	return config.LightingGroupEntry{ID: id, Name: "Anim " + id, Role: "animateur", Lights: lights, NearTeams: []string{}}
}

func TestTW225_GroupsTest_MethodNotAllowed(t *testing.T) {
	srv, _ := tw225ServerWithDriver(t)
	for _, path := range []string{"/api/lighting/groups/test", "/api/lighting/groups/test/stop"} {
		if code, _ := devDo(t, srv, "GET", path, ""); code != http.StatusMethodNotAllowed {
			t.Fatalf("GET %s refusé en 405, obtenu %d", path, code)
		}
	}
}

func TestTW225_GroupsTest_Refusals400(t *testing.T) {
	srv, _, prov := tw225ServerWithTester(t)
	tw225ConfigureGroups(tw225Salle("BuzzHue1"),
		config.LightingGroupEntry{ID: "s2", Name: "Vide", Role: "salle", Lights: []string{}, NearTeams: []string{}},
		tw225Anim("h1", "BuzzHue1x"), tw225Anim("h2"))
	for name, body := range map[string]string{
		"numéro inconnu":                    `{"group": 9}`,
		"numéro 0":                          `{"group": 0}`,
		"ancienne forme \"host\" sans rang": `{"group": "host"}`,
		"rang animateur inexistant":         `{"group": "host3"}`,
		"rang animateur 0":                  `{"group": "host0"}`,
		"groupe animateur vide":             `{"group": "host2"}`,
		"corps sans groupe":                 `{}`,
		"corps illisible":                   `pas du json`,
		"groupe de salle vide":              `{"group": 2}`,
		"chaîne autre que host<k>":          `{"group": "salle"}`,
	} {
		if code, out := devDo(t, srv, "POST", "/api/lighting/groups/test", body); code != http.StatusBadRequest {
			t.Errorf("%s : 400 attendu, obtenu %d %v", name, code, out)
		}
	}
	if len(prov.started) != 0 {
		t.Errorf("un refus ne démarre rien : %v", prov.started)
	}
}

func TestTW225_GroupsTest_NoLongerAnswers429_NewTestReplacesPrevious(t *testing.T) {
	srv, _, prov := tw225ServerWithTester(t)
	tw225ConfigureGroups(tw225Salle("BuzzHue1"), tw225Anim("h1", "BuzzHue1x"))
	for _, body := range []string{`{"group": 1}`, `{"group": 1}`, `{"group": "host1"}`} {
		code, out := devDo(t, srv, "POST", "/api/lighting/groups/test", body)
		if code == http.StatusTooManyRequests {
			t.Fatalf("un nouvel essai remplace le précédent, jamais 429 : %v", out)
		}
		if code != 200 {
			t.Fatalf("%s : %d %v", body, code, out)
		}
	}
	if strings.Join(prov.started, ",") != "1,1,host1" {
		t.Errorf("chaque appel démarre un essai (le précédent est remplacé) : %v", prov.started)
	}
}

func TestTW225_GroupsTest_LightingDisabled_NoDriver_409(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	tw225ConfigureGroups(tw225Salle("BuzzHue1"))
	srv.Lighting = &tw225Provider{devProvider: &devProvider{}} // éclairage désactivé : aucun pilote vivant
	if code, _ := devDo(t, srv, "POST", "/api/lighting/groups/test", `{"group": 1}`); code != http.StatusConflict {
		t.Fatalf("éclairage désactivé ⇒ 409, obtenu %d", code)
	}
}

func TestTW225_GroupsTest_NoProvider_409(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	tw225ConfigureGroups(tw225Salle("BuzzHue1"))
	code, _ := devDo(t, srv, "POST", "/api/lighting/groups/test", `{"group": 1}`)
	if code != http.StatusConflict {
		t.Fatalf("sans pilote ⇒ 409, obtenu %d", code)
	}
}

func TestTW225_GroupsTest_BridgeUnreachable_503(t *testing.T) {
	srv, bridge, prov := tw225ServerWithTester(t)
	tw225ConfigureGroups(tw225Salle("BuzzHue1"))
	bridge.srv.Close() // pont éteint : le pilote passe à « unreachable » dès qu'il essaie
	_ = prov.LightingDriver().RefreshInventory(context.Background())
	code, out := devDo(t, srv, "POST", "/api/lighting/groups/test", `{"group": 1}`)
	if code != http.StatusServiceUnavailable {
		t.Fatalf("pont injoignable ⇒ 503 (état connu du pilote, sans E/S), obtenu %d %v", code, out)
	}
	if len(prov.started) != 0 {
		t.Error("rien n'est démarré")
	}
}

func TestTW225_GroupsTest_Immediate200_NoDirectBridgeWrite(t *testing.T) {
	srv, bridge, prov := tw225ServerWithTester(t)
	tw225ConfigureGroups(tw225Salle("BuzzHue1"), tw225Anim("h1", "BuzzHue1x"))
	for name, body := range map[string]string{"salle": `{"group": 1}`, "animateur": `{"group": "host1"}`} {
		code, out := devDo(t, srv, "POST", "/api/lighting/groups/test", body)
		if code != 200 || out["ok"] != true || out["duration_ms"] != float64(6000) {
			t.Fatalf("%s : réponse immédiate {\"ok\":true,\"duration_ms\":6000} : %d %v", name, code, out)
		}
	}
	if len(prov.started) != 2 {
		t.Errorf("la séquence est confiée au contrôleur : %v", prov.started)
	}
	// La séquence est portée par l'écrivain via Scene() : le gestionnaire HTTP n'écrit jamais au pont lui-même.
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	for _, h := range bridge.hits {
		if strings.HasPrefix(h, "PUT ") && strings.Contains(h, "/state") {
			t.Errorf("aucune écriture directe du gestionnaire (séquence via l'écrivain) : %s", h)
		}
	}
}

func TestTW225_GroupsTest_Stop_Always200_AndStopsTheTest(t *testing.T) {
	srv, _, prov := tw225ServerWithTester(t)
	tw225ConfigureGroups(tw225Salle("BuzzHue1"))
	for i := 0; i < 2; i++ { // sans essai en cours, puis après un essai
		code, out := devDo(t, srv, "POST", "/api/lighting/groups/test/stop", "")
		if code != 200 || out["ok"] != true {
			t.Fatalf("stop : 200 {\"ok\":true} même sans essai en cours : %d %v", code, out)
		}
		devDo(t, srv, "POST", "/api/lighting/groups/test", `{"group": 1}`)
	}
	if prov.stops != 2 {
		t.Errorf("chaque appel de stop arrête l'essai : %d", prov.stops)
	}
}

func TestTW225_GroupsTest_Stop_WithoutProvider_Still200(t *testing.T) {
	srv, _ := setupTestHTTPServer(t)
	if code, out := devDo(t, srv, "POST", "/api/lighting/groups/test/stop", ""); code != 200 || out["ok"] != true {
		t.Fatalf("%d %v", code, out)
	}
}
