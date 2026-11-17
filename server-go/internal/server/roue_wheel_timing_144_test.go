package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// #144 (v12.0.0) — POST /api/roue/wheel-preview : TIMING (contracts/roue.md §10.2, §11.3). Les durées de l'essai
// viennent du serveur (aucune copie de règle dans la page) : bornées comme à l'enregistrement, SPIN_UP_MS = 2 × SPIN_PERIOD_MS.

type wheelTiming144 struct {
	Timing *struct {
		SpinPeriodMs   int `json:"SPIN_PERIOD_MS"`
		SpinUpMs       int `json:"SPIN_UP_MS"`
		SpinMaxMs      int `json:"SPIN_MAX_MS"`
		StopDurationMs int `json:"STOP_DURATION_MS"`
	} `json:"TIMING"`
	Segments []json.RawMessage `json:"SEGMENTS"`
}

func previewTiming144(t *testing.T, body string) wheelTiming144 {
	t.Helper()
	s, _ := setupTestHTTPServer(t)
	w := postWheelPreview144(t, s, body)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d : %s", w.Code, w.Body.String())
	}
	var got wheelTiming144
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Timing == nil {
		t.Fatalf("TIMING est toujours présent : %s", w.Body.String())
	}
	return got
}

func TestRoueWheelTiming144_PresentWithDefaultsWhenNoDuration(t *testing.T) {
	g := previewTiming144(t, wheelBody144(1000, 50, 2, 1))
	if g.Timing.SpinPeriodMs != 2500 || g.Timing.SpinUpMs != 5000 || g.Timing.SpinMaxMs != 10000 || g.Timing.StopDurationMs != 4000 {
		t.Errorf("défauts : période 2500, montée 5000 (1 tour), max 10000, freinage 4000 — obtenu %+v", *g.Timing)
	}
}

func TestRoueWheelTiming144_SpinUpIsTwiceThePeriod(t *testing.T) {
	for _, per := range []int{1000, 1800, 2500, 4000, 6000} {
		g := previewTiming144(t, `{"max_points":100,"step":10,"bankrupt_count":1,"lose_turn_count":1,"spin_period_ms":`+itoa144(per)+`}`)
		if g.Timing.SpinPeriodMs != per || g.Timing.SpinUpMs != 2*per {
			t.Errorf("période %d : SPIN_UP_MS = 2 × période attendu, obtenu %+v", per, *g.Timing)
		}
	}
}

func TestRoueWheelTiming144_OutOfBoundsDurationsAreClamped(t *testing.T) {
	cases := []struct {
		body                  string
		period, up, max, stop int
	}{
		{`"spin_period_ms":10,"spin_max_ms":1,"stop_duration_ms":1`, 1000, 2000, 3000, 1500},
		{`"spin_period_ms":99999,"spin_max_ms":999999,"stop_duration_ms":99999`, 6000, 12000, 30000, 8000},
		{`"spin_period_ms":3000,"spin_max_ms":5000,"stop_duration_ms":2000`, 3000, 6000, 5000, 2000},
	}
	for _, c := range cases {
		g := previewTiming144(t, `{"max_points":100,"step":10,"bankrupt_count":1,"lose_turn_count":1,`+c.body+`}`)
		if g.Timing.SpinPeriodMs != c.period || g.Timing.SpinUpMs != c.up || g.Timing.SpinMaxMs != c.max || g.Timing.StopDurationMs != c.stop {
			t.Errorf("%s : attendu période=%d montée=%d max=%d freinage=%d, obtenu %+v", c.body, c.period, c.up, c.max, c.stop, *g.Timing)
		}
	}
}

// Les cases ne dépendent pas des durées : la réponse est identique pour les cases, avec ou sans durées.
func TestRoueWheelTiming144_SegmentsIdenticalWhateverTheDurations(t *testing.T) {
	a := previewTiming144(t, wheelBody144(1000, 50, 2, 1))
	b := previewTiming144(t, `{"max_points":1000,"step":50,"bankrupt_count":2,"lose_turn_count":1,"spin_period_ms":6000,"spin_max_ms":3000,"stop_duration_ms":8000}`)
	ja, _ := json.Marshal(a.Segments)
	jb, _ := json.Marshal(b.Segments)
	if string(ja) != string(jb) || len(a.Segments) != 22 {
		t.Errorf("les cases sont indépendantes des durées (22 cases)")
	}
}

// L'aperçu n'écrit rien : la configuration enregistrée est la même avant et après.
func TestRoueWheelTiming144_WritesNothing(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	get := func() string {
		req := httptest.NewRequest("GET", "/game-config.json", nil)
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, req)
		return w.Body.String()
	}
	before := get()
	w := postWheelPreview144(t, s, `{"max_points":500,"step":25,"bankrupt_count":3,"lose_turn_count":2,"spin_period_ms":1234,"spin_max_ms":4321,"stop_duration_ms":2222}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	if after := get(); after != before || strings.Contains(after, "1234") {
		t.Errorf("wheel-preview ne doit rien enregistrer")
	}
}

func itoa144(n int) string { b, _ := json.Marshal(n); return string(b) }
