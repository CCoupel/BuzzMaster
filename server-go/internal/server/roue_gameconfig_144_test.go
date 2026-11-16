// #144 (v12.0.0, LOT-1.2) — section `wheel` de game-config.json via GET / POST /game-config.json
// (contracts/roue.md §10) : défauts, fusion par section, validation, pas corrigé, durées bornées.
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type wheelCfg144 struct {
	MaxPoints      int `json:"max_points"`
	Step           int `json:"step"`
	BankruptCount  int `json:"bankrupt_count"`
	LoseTurnCount  int `json:"lose_turn_count"`
	SpinPeriodMs   int `json:"spin_period_ms"`
	SpinMaxMs      int `json:"spin_max_ms"`
	StopDurationMs int `json:"stop_duration_ms"`
	FlipIntervalMs int `json:"flip_interval_ms"`
}

func getGameConfig144(t *testing.T, s *HTTPServer) (wheel, def wheelCfg144, raw map[string]json.RawMessage) {
	t.Helper()
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, httptest.NewRequest("GET", "/game-config.json", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /game-config.json : %d %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("JSON invalide : %v", err)
	}
	if err := json.Unmarshal(raw["wheel"], &wheel); err != nil {
		t.Fatalf("section wheel absente ou invalide : %v (%s)", err, w.Body.String())
	}
	if err := json.Unmarshal(raw["wheel_default"], &def); err != nil {
		t.Fatalf("wheel_default absent ou invalide : %v", err)
	}
	return
}

func postGameConfig144(t *testing.T, s *HTTPServer, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/game-config.json", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	return w
}

var defaultWheel144 = wheelCfg144{1000, 50, 2, 1, 2500, 10000, 4000, 600}

// Section absente : roue par défaut 1000 / 50 / 2 / 1, durées par défaut ; wheel_default toujours présent.
func TestRoueGameConfig144_AbsentSectionYieldsDefaults(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	wheel, def, _ := getGameConfig144(t, s)
	if wheel != defaultWheel144 {
		t.Errorf("wheel par défaut = %+v, attendu %+v", wheel, defaultWheel144)
	}
	if def != defaultWheel144 {
		t.Errorf("wheel_default = %+v, attendu %+v", def, defaultWheel144)
	}
}

func TestRoueGameConfig144_PostReplacesSectionAndPersists(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w := postGameConfig144(t, s, `{"wheel":{"max_points":100,"step":10,"bankrupt_count":1,"lose_turn_count":1,"spin_period_ms":3000,"spin_max_ms":12000,"stop_duration_ms":5000,"flip_interval_ms":500}}`)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	wheel, def, _ := getGameConfig144(t, s)
	want := wheelCfg144{100, 10, 1, 1, 3000, 12000, 5000, 500}
	if wheel != want {
		t.Errorf("wheel relu = %+v, attendu %+v", wheel, want)
	}
	if def != defaultWheel144 {
		t.Errorf("wheel_default est en lecture seule : %+v", def)
	}
}

// Fusion par section : une requête sans `wheel` ne touche pas la roue enregistrée.
func TestRoueGameConfig144_PostWithoutWheelLeavesWheelUntouched(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	if w := postGameConfig144(t, s, `{"wheel":{"max_points":100,"step":10,"bankrupt_count":1,"lose_turn_count":1}}`); w.Code != http.StatusOK {
		t.Fatalf("POST wheel : %d %s", w.Code, w.Body.String())
	}
	if w := postGameConfig144(t, s, `{"neon_effect":{"enabled":true}}`); w.Code != http.StatusOK {
		t.Fatalf("POST neon_effect : %d %s", w.Code, w.Body.String())
	}
	wheel, _, _ := getGameConfig144(t, s)
	if wheel.MaxPoints != 100 || wheel.Step != 10 || wheel.BankruptCount != 1 {
		t.Errorf("la roue enregistrée ne doit pas changer, got %+v", wheel)
	}
}

// Inversement, poster `wheel` ne touche pas les autres sections.
func TestRoueGameConfig144_PostWheelLeavesOtherSectionsUntouched(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	if w := postGameConfig144(t, s, `{"game":{"default_delay":66}}`); w.Code != http.StatusOK {
		t.Fatalf("POST game : %d", w.Code)
	}
	if w := postGameConfig144(t, s, `{"wheel":{"max_points":100,"step":10,"bankrupt_count":0,"lose_turn_count":0}}`); w.Code != http.StatusOK {
		t.Fatalf("POST wheel : %d", w.Code)
	}
	_, _, raw := getGameConfig144(t, s)
	if !strings.Contains(string(raw["game"]), "66") {
		t.Errorf("la section game ne doit pas être modifiée : %s", raw["game"])
	}
}

// Pas corrigé : réponse 200 portant le pas retenu et step_adjusted_from ; la valeur corrigée est celle enregistrée.
func TestRoueGameConfig144_StepAdjustedIsReportedAndStored(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w := postGameConfig144(t, s, `{"wheel":{"max_points":100,"step":30,"bankrupt_count":1,"lose_turn_count":1}}`)
	if w.Code != http.StatusOK {
		t.Fatalf("la correction ne refuse pas : attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"step_adjusted_from":30`) {
		t.Errorf("la réponse doit porter step_adjusted_from: 30 : %s", w.Body.String())
	}
	wheel, _, _ := getGameConfig144(t, s)
	if wheel.Step != 25 {
		t.Errorf("pas enregistré = %d, attendu 25", wheel.Step)
	}
}

func TestRoueGameConfig144_NoAdjustmentKeyWhenStepValid(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w := postGameConfig144(t, s, `{"wheel":{"max_points":100,"step":10,"bankrupt_count":1,"lose_turn_count":1}}`)
	if strings.Contains(w.Body.String(), "step_adjusted_from") {
		t.Errorf("pas de step_adjusted_from sans correction : %s", w.Body.String())
	}
}

// Refus : 400 avec message, RIEN n'est enregistré (la roue précédente reste).
func TestRoueGameConfig144_InvalidWheelIsRefusedAndNothingSaved(t *testing.T) {
	cases := map[string]string{
		"pas = maximum": `{"wheel":{"max_points":100,"step":100,"bankrupt_count":1,"lose_turn_count":1}}`,
		"pas nul":       `{"wheel":{"max_points":100,"step":0,"bankrupt_count":1,"lose_turn_count":1}}`,
		"B négatif":     `{"wheel":{"max_points":100,"step":10,"bankrupt_count":-1,"lose_turn_count":1}}`,
		"trop de cases": `{"wheel":{"max_points":1000,"step":10,"bankrupt_count":0,"lose_turn_count":0}}`,
		"type invalide": `{"wheel":{"max_points":"cent","step":10}}`,
		"non entier":    `{"wheel":{"max_points":100.5,"step":10,"bankrupt_count":1,"lose_turn_count":1}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			s, _ := setupTestHTTPServer(t)
			if w := postGameConfig144(t, s, `{"wheel":{"max_points":200,"step":20,"bankrupt_count":1,"lose_turn_count":1}}`); w.Code != http.StatusOK {
				t.Fatalf("préparation : %d", w.Code)
			}
			w := postGameConfig144(t, s, body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("attendu 400, got %d : %s", w.Code, w.Body.String())
			}
			if strings.TrimSpace(w.Body.String()) == "" {
				t.Errorf("le refus doit porter un message")
			}
			wheel, _, _ := getGameConfig144(t, s)
			if wheel.MaxPoints != 200 || wheel.Step != 20 {
				t.Errorf("rien ne doit être enregistré sur un refus, got %+v", wheel)
			}
		})
	}
}

// Durées hors bornes : ramenées à la borne, sans refus.
func TestRoueGameConfig144_DurationsClampedNotRefused(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w := postGameConfig144(t, s, `{"wheel":{"max_points":100,"step":10,"bankrupt_count":1,"lose_turn_count":1,"spin_period_ms":10,"spin_max_ms":999999,"stop_duration_ms":1,"flip_interval_ms":99999}}`)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	wheel, _, _ := getGameConfig144(t, s)
	if wheel.SpinPeriodMs != 1000 || wheel.SpinMaxMs != 30000 || wheel.StopDurationMs != 1500 || wheel.FlipIntervalMs != 1500 {
		t.Errorf("durées ramenées à la borne : %+v", wheel)
	}
}

// Non-régression : GET /game-config.json garde ses sections existantes.
func TestRoueGameConfig144_ExistingSectionsStillServed(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, httptest.NewRequest("GET", "/game-config.json", nil))
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("JSON invalide : %v", err)
	}
	for _, k := range []string{"game", "neon_effect"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("la section %s doit rester servie", k)
		}
	}
}
