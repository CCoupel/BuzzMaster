// #285 (v11.4.0, LOT-4, révision 2 du contrat — 0ee50546) — complément de spécification par la frontière HTTP :
// DELETE refusé en 409 quand SEUL l'historique référence la catégorie, noms réservés Windows refusés en 400,
// clé Unicode (« CINÉMA ») acceptée de bout en bout. Source : contracts/http-endpoints.md §POST/DELETE /api/categories.
// Aucun symbole du dev : HTTP + JSON brut + méthodes existantes du moteur. Préfixe tw285. Fixtures sous t.TempDir().
package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"buzzcontrol/internal/game"
)

func TestCategories285Rev2_DELETE_OnlyHistory_409_WithZeroCountsAndHistory_ThenModeOK_WithBak(t *testing.T) {
	e := tw285New(t)
	e.custom(t, "CINEMA", "Cinéma", "#0ea5e9")
	// aucune question, aucune carte, aucune question RAFALE : seul l'historique référence CINEMA
	e.srv.engine.AddGameEvent(game.GameEvent{Timestamp: 1, QuestionID: "q", QuestionCategory: "CINEMA", EventType: "POINTS_AWARDED", Points: 10, WinnerType: "TEAM", TeamName: "A"})
	e.srv.engine.AddGameEvent(game.GameEvent{Timestamp: 2, QuestionID: "q2", EventType: "POINTS_AWARDED", Points: 4, WinnerType: "TEAM", TeamName: "A", CategoryBreakdown: map[string]int{"CINEMA": 4}})
	e.settleHistory(t)
	before, _ := os.ReadFile(e.histPath())

	for _, body := range []string{``, `{}`, `{"mode":"nimportequoi"}`} {
		w := e.del("CINEMA", body)
		if w.Code != http.StatusConflict || tw285Err(t, w) != "category in use" {
			t.Fatalf("corps %q : 409 category in use attendu, got %d %s", body, w.Code, w.Body.String())
		}
	}
	var p struct {
		Questions int `json:"questions"`
		Rafale    int `json:"rafale"`
		History   struct {
			Events int `json:"events"`
			Points int `json:"points"`
		} `json:"history"`
	}
	if err := json.Unmarshal(e.del("CINEMA", `{}`).Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Questions != 0 || p.Rafale != 0 || p.History.Events != 2 || p.History.Points < 10 {
		t.Errorf("payload = %+v, attendu questions=0, rafale=0, history.events=2, history.points ≥ 10", p)
	}
	// un 409 ne touche à rien : catégorie présente, historique intact, aucune sauvegarde
	if _, ok := e.list(t)["CINEMA"]; !ok {
		t.Error("un 409 ne supprime pas la catégorie")
	}
	if after, _ := os.ReadFile(e.histPath()); string(after) != string(before) {
		t.Error("un 409 ne réécrit pas l'historique")
	}
	if baks, _ := filepath.Glob(e.histPath() + ".bak*"); len(baks) != 0 {
		t.Errorf("aucune sauvegarde sur un 409 : %v", baks)
	}

	// avec un mode valide : 200, catégorie supprimée, historique fusionné, .bak créé
	w := e.del("CINEMA", `{"mode":"reassign"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("DELETE avec mode : %d %s", w.Code, w.Body.String())
	}
	if _, ok := e.list(t)["CINEMA"]; ok {
		t.Error("CINEMA doit avoir disparu")
	}
	if baks, _ := filepath.Glob(e.histPath() + ".bak*"); len(baks) != 1 {
		t.Errorf("une sauvegarde .bak attendue après fusion : %v", baks)
	}
	for _, ev := range e.srv.engine.GetHistory() {
		if ev.QuestionCategory == "CINEMA" {
			t.Errorf("l'historique ne référence plus CINEMA : %+v", ev)
		}
	}
}

func TestCategories285Rev2_POST_ReservedWindowsNames_400_NothingWritten(t *testing.T) {
	e := tw285New(t)
	for _, name := range []string{"CON", "con", "Con", "PRN", "Aux", "NUL", "COM1", "com9", "LPT1", "lpt9"} {
		w := e.post(map[string]string{"name": name}, "i.png", tw285PNG)
		if w.Code != http.StatusBadRequest || tw285Err(t, w) != "reserved name" {
			t.Errorf("%q : 400 reserved name attendu, got %d %s", name, w.Code, w.Body.String())
		}
	}
	if got := tw285Files(t, e.catDir()); len(got) != 0 {
		t.Errorf("un nom réservé n'écrit rien sur disque : %v", got)
	}
	for _, name := range []string{"Console", "COM10", "Lpt", "Auxiliaire"} {
		if w := e.post(map[string]string{"name": name}, "i.png", tw285PNG); w.Code != http.StatusOK {
			t.Errorf("%q n'est pas un nom réservé : 200 attendu, got %d %s", name, w.Code, w.Body.String())
		}
	}
}

func TestCategories285Rev2_UnicodeKey_Cinema_AcceptedEndToEnd(t *testing.T) {
	e := tw285New(t)
	w := e.post(map[string]string{"name": "Cinéma", "color": "#0ea5e9"}, "i.png", tw285PNG)
	if w.Code != http.StatusOK {
		t.Fatalf("POST « Cinéma » : 200 attendu, got %d %s", w.Code, w.Body.String())
	}
	var created tw285Cat
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if created.Key != "CINÉMA" || created.Name != "Cinéma" || !created.IsCustom {
		t.Errorf("CategoryInfo = %+v, attendu key=CINÉMA name=Cinéma isCustom", created)
	}
	if _, ok := e.list(t)["CINÉMA"]; !ok {
		t.Fatal("GET /api/categories doit lister CINÉMA")
	}
	// la clé Unicode traverse PUT et DELETE (URL encodée en UTF-8)
	if w := e.do("PUT", "/api/categories/CIN%C3%89MA", []byte(`{"color":"#ff0000"}`), "application/json"); w.Code != http.StatusOK {
		t.Fatalf("PUT CINÉMA : %d %s", w.Code, w.Body.String())
	}
	if got := e.list(t)["CINÉMA"].Color; got != "#ff0000" {
		t.Errorf("couleur = %q, attendu #ff0000", got)
	}
	if w := e.do("DELETE", "/api/categories/CIN%C3%89MA", nil, ""); w.Code != http.StatusOK {
		t.Fatalf("DELETE CINÉMA : %d %s", w.Code, w.Body.String())
	}
	if _, ok := e.list(t)["CINÉMA"]; ok {
		t.Error("CINÉMA doit avoir disparu")
	}
	// une clé qui sort de l'alphabet autorisé reste refusée (point, séparateur de chemin)
	for _, name := range []string{"a.b", "a/b", "../x"} {
		if w := e.post(map[string]string{"name": name}, "i.png", tw285PNG); w.Code != http.StatusBadRequest {
			t.Errorf("%q : 400 attendu, got %d", name, w.Code)
		}
	}
	for _, f := range tw285Files(t, e.catDir()) {
		if strings.Contains(f, "..") || strings.Contains(f, "/") {
			t.Errorf("fichier suspect : %s", f)
		}
	}
}
