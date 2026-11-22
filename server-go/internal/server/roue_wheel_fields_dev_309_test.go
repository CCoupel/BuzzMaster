package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoueWheelFieldsDev309_PresentWrittenAbsentRemoved(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := postRoue145(t, s, map[string]string{
		"roue_wheel_max_points": "500", "roue_wheel_step": "100", "roue_wheel_bankrupt_count": "0", "roue_wheel_lose_turn_count": "",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	for k, want := range map[string]float64{"ROUE_WHEEL_MAX_POINTS": 500, "ROUE_WHEEL_STEP": 100, "ROUE_WHEEL_BANKRUPT_COUNT": 0} {
		if got, ok := saved[k].(float64); !ok || got != want {
			t.Errorf("%s = %v want %v", k, saved[k], want)
		}
	}
	if _, ok := saved["ROUE_WHEEL_LOSE_TURN_COUNT"]; ok {
		t.Error("empty field must be absent (follows the global)")
	}
	if _, ok := saved["ROUE_BONUS_COUNT"]; ok {
		t.Error("absent bonus follows the global")
	}
}

func TestRoueWheelFieldsDev309_StepCorrectedAndSaved(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := postRoue145(t, s, map[string]string{"roue_wheel_max_points": "1000", "roue_wheel_step": "70"})
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if st, _ := saved["ROUE_WHEEL_STEP"].(float64); st != 50 {
		t.Errorf("corrected step expected, got %v", saved["ROUE_WHEEL_STEP"])
	}
}

func TestRoueWheelFieldsDev309_Refusals(t *testing.T) {
	cases := []map[string]string{
		{"roue_wheel_max_points": "1"},
		{"roue_wheel_max_points": "abc"},
		{"roue_wheel_max_points": "1000", "roue_wheel_step": "1000"},
		{"roue_wheel_bankrupt_count": "-1"},
		{"roue_wheel_max_points": "100000", "roue_wheel_step": "1"}, // > 64 cases
	}
	for _, c := range cases {
		s, _ := setupTestHTTPServer(t)
		w, _ := postRoue145(t, s, c)
		if w.Code != http.StatusBadRequest || !strings.HasPrefix(w.Body.String(), "ROUE_WHEEL_INVALID") {
			t.Errorf("%v: %d %q", c, w.Code, w.Body.String())
		}
	}
}

func postGameConfigDev309(t *testing.T, s *HTTPServer, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, httptest.NewRequest("POST", "/game-config.json", strings.NewReader(body)))
	return w
}

func TestRoueWheelFieldsDev309_GlobalBonusCount(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	ok := postGameConfigDev309(t, s, `{"wheel":{"max_points":1000,"step":50,"bankrupt_count":2,"lose_turn_count":1,"bonus_count":2}}`)
	if ok.Code != http.StatusOK || !strings.Contains(ok.Body.String(), `"bonus_count":2`) {
		t.Fatalf("bonus_count 2: %d %s", ok.Code, ok.Body.String())
	}
	for _, bad := range []string{`"bonus_count":5`, `"bonus_count":-1`} {
		w := postGameConfigDev309(t, s, `{"wheel":{"max_points":1000,"step":50,"bankrupt_count":2,"lose_turn_count":1,`+bad+`}}`)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", bad, w.Code)
		}
	}
	if w := postGameConfigDev309(t, s, `{"wheel":{"max_points":1000,"step":100,"bankrupt_count":3,"lose_turn_count":2,"bonus_count":2}}`); w.Code != http.StatusBadRequest {
		t.Errorf("above the maximum (1 on a 14-case wheel, contract §17.3): %d", w.Code)
	}
	g := httptest.NewRecorder()
	s.mux.ServeHTTP(g, httptest.NewRequest("GET", "/game-config.json", nil))
	if !strings.Contains(g.Body.String(), `"wheel_default":{`) || !strings.Contains(g.Body.String(), `"bonus_count":0`) {
		t.Errorf("wheel_default must carry bonus_count 0: %s", g.Body.String())
	}
}

// M1 (review LOT-2): the BONUS count INHERITED from the global setting must be validated too at save time.
func TestRoueWheelFieldsDev309_InheritedBonusOverflowRefused(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	const def = `{"wheel":{"max_points":1000,"step":50,"bankrupt_count":2,"lose_turn_count":1,"bonus_count":0}}`
	t.Cleanup(func() { postGameConfigDev309(t, s, def) })
	// global: 1000/100/2/1 (12 cases) + 2 BONUS = the maximum
	if w := postGameConfigDev309(t, s, `{"wheel":{"max_points":1000,"step":100,"bankrupt_count":2,"lose_turn_count":1,"bonus_count":2}}`); w.Code != http.StatusOK {
		t.Fatalf("global: %d %s", w.Code, w.Body.String())
	}
	// the question brings the wheel to 3 BANQUEROUTE / 2 PERDU (1 BONUS at most) without sending any BONUS count: 2 inherited > 1
	w, saved := postRoue145(t, s, map[string]string{"roue_wheel_bankrupt_count": "3", "roue_wheel_lose_turn_count": "2"})
	if w.Code != http.StatusBadRequest || !strings.HasPrefix(w.Body.String(), "ROUE_BONUS_COUNT_INVALID") {
		t.Fatalf("attendu 400 ROUE_BONUS_COUNT_INVALID, got %d %q", w.Code, w.Body.String())
	}
	if saved != nil {
		t.Errorf("rien ne doit être écrit : %v", saved)
	}
	// a customised BONUS count that fits is accepted
	if w, _ := postRoue145(t, s, map[string]string{"roue_wheel_bankrupt_count": "3", "roue_wheel_lose_turn_count": "2", "roue_bonus_count": "1"}); w.Code != http.StatusOK {
		t.Errorf("1 BONUS personnalisé : %d %s", w.Code, w.Body.String())
	}
}
