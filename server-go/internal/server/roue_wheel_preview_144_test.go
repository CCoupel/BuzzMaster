// #144 (v12.0.0, LOT-1.2) — construction de la roue : POST /api/roue/wheel-preview (contracts/roue.md §10.1, §11.3).
// Les 10 exemples du contrat sont des cas de test EXIGÉS (résultat identique, case pour case), ainsi que les 5
// corrections de pas et les refus. Frontière HTTP uniquement : la règle de construction est unique, côté serveur.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type wheelPreview144 struct {
	Count    int  `json:"COUNT"`
	Step     int  `json:"step"`
	Adjusted *int `json:"step_adjusted_from"`
	Segments []struct {
		ID    string `json:"ID"`
		Kind  string `json:"KIND"`
		Value int    `json:"VALUE"`
	} `json:"SEGMENTS"`
}

func postWheelPreview144(t *testing.T, s *HTTPServer, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/roue/wheel-preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	return w
}

func wheelBody144(max, step, b, p int) string {
	return fmt.Sprintf(`{"max_points":%d,"step":%d,"bankrupt_count":%d,"lose_turn_count":%d}`, max, step, b, p)
}

// render met la roue sous la forme du contrat : « 100 BANQ 90 … PERDU … ».
func (w wheelPreview144) render() string {
	parts := make([]string, 0, len(w.Segments))
	for _, sg := range w.Segments {
		switch sg.Kind {
		case "bankrupt":
			parts = append(parts, "BANQ")
		case "lose_turn":
			parts = append(parts, "PERDU")
		default:
			parts = append(parts, fmt.Sprint(sg.Value))
		}
	}
	return strings.Join(parts, " ")
}

func TestRoueWheel144_ContractExamples(t *testing.T) {
	cases := []struct {
		max, step, b, p int
		want            string
	}{
		{100, 10, 0, 0, "100 10 90 20 80 30 70 40 60 50"},
		{100, 10, 1, 0, "100 BANQ 90 10 80 20 70 30 60 40 50"},
		{100, 10, 1, 1, "100 BANQ 90 10 80 PERDU 20 70 30 60 40"},
		{100, 10, 0, 1, "100 10 90 20 80 PERDU 30 70 40 60"},
		{1000, 100, 2, 1, "1000 BANQ 900 100 PERDU 800 200 700 BANQ 300 600 400"},
		{1000, 50, 2, 1, "1000 BANQ 950 50 900 100 850 PERDU 150 800 200 750 250 700 BANQ 300 650 350 600 400 550 450"},
		{100, 20, 1, 1, "100 BANQ 80 PERDU 20 60 40"},
		{1000, 100, 3, 2, "1000 BANQ PERDU 900 100 BANQ 800 200 PERDU 700 300 BANQ 600 400"},
		{100, 10, 0, 2, "100 10 PERDU 90 20 80 30 70 PERDU 40 60"},
		{100, 30, 1, 1, "100 BANQ PERDU 75 25"}, // pas corrigé à 25
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%d/%d/%d/%d", c.max, c.step, c.b, c.p), func(t *testing.T) {
			s, _ := setupTestHTTPServer(t)
			w := postWheelPreview144(t, s, wheelBody144(c.max, c.step, c.b, c.p))
			if w.Code != http.StatusOK {
				t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
			}
			var got wheelPreview144
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("JSON invalide : %v", err)
			}
			if got.render() != c.want {
				t.Errorf("roue = %q\nattendu = %q", got.render(), c.want)
			}
			if got.Count != len(got.Segments) {
				t.Errorf("COUNT = %d, %d cases", got.Count, len(got.Segments))
			}
		})
	}
}

// Roue par défaut 1000 / 50 / 2 / 1 : 22 cases (validée par l'utilisateur).
func TestRoueWheel144_DefaultWheelHas22Segments(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	var got wheelPreview144
	w := postWheelPreview144(t, s, wheelBody144(1000, 50, 2, 1))
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Count != 22 {
		t.Errorf("COUNT = %d, attendu 22", got.Count)
	}
}

func TestRoueWheel144_SegmentIDsAreUniqueAndKindsValid(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	var got wheelPreview144
	_ = json.Unmarshal(postWheelPreview144(t, s, wheelBody144(1000, 50, 2, 1)).Body.Bytes(), &got)
	seen := map[string]bool{}
	for _, sg := range got.Segments {
		if sg.ID == "" || seen[sg.ID] {
			t.Errorf("ID de case vide ou dupliqué : %q", sg.ID)
		}
		seen[sg.ID] = true
		if sg.Kind != "value" && sg.Kind != "bankrupt" && sg.Kind != "lose_turn" {
			t.Errorf("KIND invalide %q", sg.Kind)
		}
		if sg.Kind != "value" && sg.Value != 0 {
			t.Errorf("une case spéciale a VALUE=0, got %d", sg.Value)
		}
	}
}

// Les valeurs sont toutes différentes (et chaque valeur apparaît une fois).
func TestRoueWheel144_ValuesAreAllDistinct(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	var got wheelPreview144
	_ = json.Unmarshal(postWheelPreview144(t, s, wheelBody144(1000, 50, 2, 1)).Body.Bytes(), &got)
	seen := map[int]bool{}
	for _, sg := range got.Segments {
		if sg.Kind != "value" {
			continue
		}
		if seen[sg.Value] {
			t.Errorf("valeur %d présente deux fois", sg.Value)
		}
		seen[sg.Value] = true
	}
}

// Correction du pas : 5 exemples exigés (dont l'égalité de distance → le plus petit diviseur).
func TestRoueWheel144_StepCorrection(t *testing.T) {
	cases := []struct{ max, step, want int }{
		{100, 30, 25}, {1000, 300, 250}, {1000, 70, 50}, {100, 7, 5}, {100, 15, 10},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%d/%d", c.max, c.step), func(t *testing.T) {
			s, _ := setupTestHTTPServer(t)
			w := postWheelPreview144(t, s, wheelBody144(c.max, c.step, 1, 1))
			if w.Code != http.StatusOK {
				t.Fatalf("la correction ne refuse pas : attendu 200, got %d : %s", w.Code, w.Body.String())
			}
			var got wheelPreview144
			_ = json.Unmarshal(w.Body.Bytes(), &got)
			if got.Step != c.want {
				t.Errorf("pas retenu = %d, attendu %d", got.Step, c.want)
			}
			if got.Adjusted == nil || *got.Adjusted != c.step {
				t.Errorf("step_adjusted_from = %v, attendu %d", got.Adjusted, c.step)
			}
		})
	}
}

// Pas valide : aucune correction annoncée (clé absente).
func TestRoueWheel144_NoAdjustmentKeyWhenStepDivides(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w := postWheelPreview144(t, s, wheelBody144(100, 10, 1, 1))
	if strings.Contains(w.Body.String(), "step_adjusted_from") {
		t.Errorf("step_adjusted_from doit être absent sans correction : %s", w.Body.String())
	}
}

func TestRoueWheel144_Refusals400(t *testing.T) {
	cases := map[string]string{
		"pas égal au maximum":                      wheelBody144(100, 100, 1, 1),
		"pas supérieur au maximum":                 wheelBody144(100, 200, 1, 1),
		"pas nul":                                  wheelBody144(100, 0, 1, 1),
		"pas négatif":                              wheelBody144(100, -10, 1, 1),
		"maximum négatif":                          wheelBody144(-100, 10, 1, 1),
		"BANQUEROUTE négative":                     wheelBody144(100, 10, -1, 1),
		"PERDU négatif":                            wheelBody144(100, 10, 1, -1),
		"valeur non entière":                       `{"max_points":100.5,"step":10,"bankrupt_count":1,"lose_turn_count":1}`,
		"65 cases (1000/10/0/0 + 0 → 100 valeurs)": wheelBody144(1000, 10, 0, 0),
		"65 cases par les cases spéciales":         wheelBody144(1000, 20, 10, 6),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			s, _ := setupTestHTTPServer(t)
			w := postWheelPreview144(t, s, body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("attendu 400, got %d : %s", w.Code, w.Body.String())
			}
			var e map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil || e["CODE"] != "ROUE_WHEEL_INVALID" || e["MESSAGE"] == "" {
				t.Errorf("attendu {CODE: ROUE_WHEEL_INVALID, MESSAGE}, got %s", w.Body.String())
			}
		})
	}
}

// Limite : 64 cases pile acceptées, 65 refusées.
func TestRoueWheel144_SixtyFourSegmentsMaximum(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	// 60 valeurs − 1 (300 = max/2 cède sa place au TOUR PERDU) + 3 BANQ + 2 PERDU = 64
	if w := postWheelPreview144(t, s, wheelBody144(600, 10, 3, 2)); w.Code != http.StatusOK {
		t.Errorf("64 cases : attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	// 59 valeurs + 4 BANQ + 2 PERDU = 65
	if w := postWheelPreview144(t, s, wheelBody144(600, 10, 4, 2)); w.Code != http.StatusBadRequest {
		t.Errorf("65 cases : attendu 400, got %d", w.Code)
	}
}

// L'aperçu n'écrit rien : la configuration enregistrée n'est pas modifiée.
func TestRoueWheel144_PreviewDoesNotPersist(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	before := httptest.NewRecorder()
	s.mux.ServeHTTP(before, httptest.NewRequest("GET", "/game-config.json", nil))
	_ = postWheelPreview144(t, s, wheelBody144(100, 10, 1, 1))
	after := httptest.NewRecorder()
	s.mux.ServeHTTP(after, httptest.NewRequest("GET", "/game-config.json", nil))
	if before.Body.String() != after.Body.String() {
		t.Errorf("l'aperçu ne doit pas modifier la configuration")
	}
}

func TestRoueWheel144_MethodNotAllowed(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/roue/wheel-preview", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET : attendu 405, got %d", w.Code)
	}
}
