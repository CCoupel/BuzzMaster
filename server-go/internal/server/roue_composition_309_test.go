// #309 (v12.1.0, LOT-2) — composition de la roue par question, côté HTTP : POST /questions (5 champs, champ vide = retiré, 0 = enregistré,
// pas corrigé, refus 400), game-config.json `wheel.bonus_count`, relecture d'une question du LOT-1. Source : contracts/roue.md §16.
// Réutilise postRoue145 / questionDirCount145 / postGameConfig144 / getGameConfig144.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"buzzcontrol/internal/config"
)

// Champs de composition de la question, et leur nom de formulaire.
var compoFields309 = [][2]string{
	{"roue_wheel_max_points", "ROUE_WHEEL_MAX_POINTS"}, {"roue_wheel_step", "ROUE_WHEEL_STEP"},
	{"roue_wheel_bankrupt_count", "ROUE_WHEEL_BANKRUPT_COUNT"}, {"roue_wheel_lose_turn_count", "ROUE_WHEEL_LOSE_TURN_COUNT"},
	{"roue_bonus_count", "ROUE_BONUS_COUNT"},
}

func setGlobalWheel309(t *testing.T, s *HTTPServer, body string) {
	t.Helper()
	if w := postGameConfig144(t, s, body); w.Code != http.StatusOK {
		t.Fatalf("POST /game-config.json %s : %d %s", body, w.Code, w.Body.String())
	}
}

// critical — question v12.0 (aucun champ de composition) : AUCUN des 5 champs n'est écrit (tout suit le global).
func TestRoueCompo309_Upload_NoFieldWritesNothing(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := postRoue145(t, s, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	for _, f := range compoFields309 {
		if _, present := saved[f[1]]; present {
			t.Errorf("%s ne doit pas être écrit quand le champ du formulaire est absent : %v", f[1], saved[f[1]])
		}
	}
}

// Champ vide = retiré (retour au global), pour chacun des 5.
func TestRoueCompo309_Upload_EmptyFieldIsRemoved(t *testing.T) {
	for _, f := range compoFields309 {
		t.Run(f[0], func(t *testing.T) {
			s, _ := setupTestHTTPServer(t)
			w, saved := postRoue145(t, s, map[string]string{f[0]: ""})
			if w.Code != http.StatusOK {
				t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
			}
			if _, present := saved[f[1]]; present {
				t.Errorf("%s = %v alors que le champ est vide", f[1], saved[f[1]])
			}
		})
	}
}

// Valeur présente = personnalisée, enregistrée telle quelle, y compris 0 ; relecture complète des autres champs.
func TestRoueCompo309_Upload_CustomValuesPersisted(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := postRoue145(t, s, map[string]string{
		"roue_wheel_max_points": "500", "roue_wheel_step": "100", "roue_wheel_bankrupt_count": "0", "roue_wheel_lose_turn_count": "3", "roue_bonus_count": "0",
		"ROUE_MODE": "SOLO", "roue_vowel_timing": "BOTH",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	want := map[string]float64{"ROUE_WHEEL_MAX_POINTS": 500, "ROUE_WHEEL_STEP": 100, "ROUE_WHEEL_BANKRUPT_COUNT": 0, "ROUE_WHEEL_LOSE_TURN_COUNT": 3, "ROUE_BONUS_COUNT": 0}
	for k, v := range want {
		got, present := saved[k]
		if !present || got != v {
			t.Errorf("%s = %v (présent : %v), attendu %v (0 personnalisé conservé)", k, got, present, v)
		}
	}
	if saved["ROUE_MODE"] != "SOLO" || saved["ROUE_VOWEL_TIMING"] != "BOTH" {
		t.Errorf("autres champs ROUE_* altérés : %v", saved)
	}
}

// Un champ personnalisé égal à la valeur globale reste enregistré (il ne suit plus le global).
func TestRoueCompo309_Upload_CustomEqualToGlobalIsKept(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := postRoue145(t, s, map[string]string{"roue_wheel_max_points": "1000", "roue_wheel_bankrupt_count": "2"}) // = roue par défaut
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if saved["ROUE_WHEEL_MAX_POINTS"] != float64(1000) || saved["ROUE_WHEEL_BANKRUPT_COUNT"] != float64(2) {
		t.Errorf("valeurs égales au global non enregistrées : %v", saved)
	}
}

// Pas corrigé jamais refusé : la valeur enregistrée est la valeur corrigée, selon le maximum EFFECTIF.
func TestRoueCompo309_Upload_StepCorrectedAgainstEffectiveMaximum(t *testing.T) {
	cases := []struct {
		name  string
		form  map[string]string
		saved float64
	}{
		{"max perso 100, pas 30 → 25", map[string]string{"roue_wheel_max_points": "100", "roue_wheel_step": "30"}, 25},
		{"max global 1000, pas 70 → 50", map[string]string{"roue_wheel_step": "70"}, 50},
		{"max perso 1000, pas 300 → 250", map[string]string{"roue_wheel_max_points": "1000", "roue_wheel_step": "300"}, 250},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, _ := setupTestHTTPServer(t)
			w, saved := postRoue145(t, s, c.form)
			if w.Code != http.StatusOK {
				t.Fatalf("un pas à corriger n'est jamais refusé : %d %s", w.Code, w.Body.String())
			}
			if saved["ROUE_WHEEL_STEP"] != c.saved {
				t.Errorf("ROUE_WHEEL_STEP enregistré = %v, attendu la valeur corrigée %v", saved["ROUE_WHEEL_STEP"], c.saved)
			}
		})
	}
}

// Refus 400 ROUE_WHEEL_INVALID sur la composition effective ; rien d'écrit, dossier de création supprimé.
func TestRoueCompo309_Upload_InvalidCompositionRefused(t *testing.T) {
	cases := []struct {
		name string
		form map[string]string
	}{
		{"pas ≥ maximum", map[string]string{"roue_wheel_max_points": "100", "roue_wheel_step": "100"}},
		{"pas > maximum global", map[string]string{"roue_wheel_step": "1000"}},
		{"maximum perso ≤ pas global", map[string]string{"roue_wheel_max_points": "50"}},
		{"maximum < 2", map[string]string{"roue_wheel_max_points": "1"}},
		{"maximum hors limite", map[string]string{"roue_wheel_max_points": "1000001"}},
		{"plus de 64 cases", map[string]string{"roue_wheel_bankrupt_count": "70"}},
		{"BANQUEROUTE négative", map[string]string{"roue_wheel_bankrupt_count": "-1"}},
		{"TOUR PERDU négatif", map[string]string{"roue_wheel_lose_turn_count": "-2"}},
		{"valeur non entière", map[string]string{"roue_wheel_max_points": "abc"}},
		{"décimale", map[string]string{"roue_wheel_step": "2.5"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, _ := setupTestHTTPServer(t)
			w, saved := postRoue145(t, s, c.form)
			if w.Code != http.StatusBadRequest || !strings.HasPrefix(w.Body.String(), "ROUE_WHEEL_INVALID") {
				t.Fatalf("attendu 400 ROUE_WHEEL_INVALID, got %d %q", w.Code, w.Body.String())
			}
			if saved != nil {
				t.Errorf("rien ne doit être écrit : %v", saved)
			}
			if n := questionDirCount145(t); n != 0 {
				t.Errorf("le dossier d'une création refusée doit être supprimé (%d restant)", n)
			}
		})
	}
}

// BONUS hors 0–4, ou qui dépasse le maximum permis (§17.3) par la composition EFFECTIVE (champs de la question + global) : ROUE_BONUS_COUNT_INVALID.
func TestRoueCompo309_Upload_BonusCountRefusals(t *testing.T) {
	cases := []struct {
		name string
		form map[string]string
	}{
		{"5 BONUS", map[string]string{"roue_bonus_count": "5"}},
		{"négatif", map[string]string{"roue_bonus_count": "-1"}},
		{"roue perso 1000/100/3/2 + 2 BONUS", map[string]string{"roue_wheel_max_points": "1000", "roue_wheel_step": "100", "roue_wheel_bankrupt_count": "3", "roue_wheel_lose_turn_count": "2", "roue_bonus_count": "2"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, _ := setupTestHTTPServer(t)
			w, saved := postRoue145(t, s, c.form)
			if w.Code != http.StatusBadRequest || !strings.HasPrefix(w.Body.String(), "ROUE_BONUS_COUNT_INVALID") {
				t.Fatalf("attendu 400 ROUE_BONUS_COUNT_INVALID, got %d %q", w.Code, w.Body.String())
			}
			if saved != nil {
				t.Errorf("rien ne doit être écrit : %v", saved)
			}
		})
	}
}

// La validation lit le réglage global COURANT : avec une roue globale 1000/100/3/2 (1 BONUS au plus), 1 BONUS tient, 2 non ;
// une composition personnalisée qui libère de la place (sans BANQUEROUTE ni PERDU) accepte 4 BONUS.
func TestRoueCompo309_Upload_ValidatesAgainstCurrentGlobal(t *testing.T) {
	const global = `{"wheel":{"max_points":1000,"step":100,"bankrupt_count":3,"lose_turn_count":2}}`
	s, _ := setupTestHTTPServer(t)
	setGlobalWheel309(t, s, global)
	if w, _ := postRoue145(t, s, map[string]string{"roue_bonus_count": "2"}); w.Code != http.StatusBadRequest {
		t.Fatalf("roue globale 14 cases + 2 BONUS : attendu 400, got %d", w.Code)
	}
	if w, _ := postRoue145(t, s, map[string]string{"roue_bonus_count": "1"}); w.Code != http.StatusOK {
		t.Fatalf("roue globale 14 cases + 1 BONUS : attendu 200, got %d %s", w.Code, w.Body.String())
	}
	s2, _ := setupTestHTTPServer(t)
	setGlobalWheel309(t, s2, global)
	if w, _ := postRoue145(t, s2, map[string]string{"roue_wheel_bankrupt_count": "0", "roue_wheel_lose_turn_count": "0", "roue_bonus_count": "4"}); w.Code != http.StatusOK {
		t.Fatalf("composition perso sans case spéciale + 4 BONUS : attendu 200, got %d %s", w.Code, w.Body.String())
	}
}

// ---- game-config.json : wheel.bonus_count ---------------------------------------------------------------------------

// GET : bonus_count présent, 0 par défaut ; wheel_default à 0 BONUS.
func TestRoueCompo309_GameConfig_BonusCountDefaultsAndWheelDefault(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	var raw map[string]map[string]interface{}
	w := getRaw309(t, s)
	if err := json.Unmarshal(w, &raw); err != nil {
		t.Fatal(err)
	}
	for _, sec := range []string{"wheel", "wheel_default"} {
		if v, ok := raw[sec]["bonus_count"]; !ok || v != float64(0) {
			t.Errorf("%s.bonus_count = %v (présent : %v), attendu 0", sec, v, ok)
		}
	}
}

func getRaw309(t *testing.T, s *HTTPServer) []byte {
	t.Helper()
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, httptest.NewRequest("GET", "/game-config.json", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /game-config.json : %d %s", w.Code, w.Body.String())
	}
	return w.Body.Bytes()
}

// POST : BONUS global de 0 à 4 accepté, relu ; absent de la section = 0 ; hors 0–4 ou > 64 cases : 400, rien d'enregistré.
func TestRoueCompo309_GameConfig_BonusCountAcceptedAndReread(t *testing.T) {
	for n := 0; n <= 4; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, _ := setupTestHTTPServer(t)
			w := postGameConfig144(t, s, fmt.Sprintf(`{"wheel":{"max_points":1000,"step":50,"bankrupt_count":2,"lose_turn_count":1,"bonus_count":%d}}`, n))
			if w.Code != http.StatusOK {
				t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
			}
			var raw map[string]map[string]interface{}
			_ = json.Unmarshal(getRaw309(t, s), &raw)
			if raw["wheel"]["bonus_count"] != float64(n) {
				t.Errorf("bonus_count relu = %v, attendu %d", raw["wheel"]["bonus_count"], n)
			}
		})
	}
}

func TestRoueCompo309_GameConfig_SectionWithoutBonusCountMeansZero(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	setGlobalWheel309(t, s, `{"wheel":{"max_points":1000,"step":50,"bankrupt_count":2,"lose_turn_count":1,"bonus_count":3}}`)
	setGlobalWheel309(t, s, `{"wheel":{"max_points":1000,"step":50,"bankrupt_count":2,"lose_turn_count":1}}`) // la section remplace l'entière
	var raw map[string]map[string]interface{}
	_ = json.Unmarshal(getRaw309(t, s), &raw)
	if raw["wheel"]["bonus_count"] != float64(0) {
		t.Errorf("bonus_count = %v, attendu 0 (section remplacée sans le champ)", raw["wheel"]["bonus_count"])
	}
}

func TestRoueCompo309_GameConfig_BonusCountRefused(t *testing.T) {
	cases := map[string]string{
		"5 BONUS":                   `{"wheel":{"max_points":1000,"step":50,"bankrupt_count":2,"lose_turn_count":1,"bonus_count":5}}`,
		"négatif":                   `{"wheel":{"max_points":1000,"step":50,"bankrupt_count":2,"lose_turn_count":1,"bonus_count":-1}}`,
		"dépasse le maximum permis": `{"wheel":{"max_points":1000,"step":100,"bankrupt_count":3,"lose_turn_count":2,"bonus_count":2}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			s, _ := setupTestHTTPServer(t)
			before := getRaw309(t, s)
			w := postGameConfig144(t, s, body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("attendu 400, got %d : %s", w.Code, w.Body.String())
			}
			if after := getRaw309(t, s); string(after) != string(before) {
				t.Errorf("un refus ne doit rien enregistrer")
			}
		})
	}
}

// Les autres champs du global restent inchangés par l'ajout de bonus_count (non-régression §10).
func TestRoueCompo309_GameConfig_ExistingWheelFieldsUnchanged(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	wheel, def, _ := getGameConfig144(t, s)
	if wheel != defaultWheel144 || def != defaultWheel144 {
		t.Fatalf("roue par défaut v12.0 altérée : %+v / %+v", wheel, def)
	}
}

// ---- lecture d'une question du LOT-1 --------------------------------------------------------------------------------

// Un question.json écrit par le LOT-1 (ROUE_BONUS_COUNT seul, aucun champ de composition) est relu sans erreur, par le moteur de
// lecture des questions : le BONUS reste personnalisé, les 4 autres paramètres absents.
func TestRoueCompo309_Lot1QuestionFileStillLoads(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, _ := postRoue145(t, s, map[string]string{"roue_bonus_count": "2"})
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	qdir := config.Get().Storage.QuestionsDir
	entries, _ := os.ReadDir(qdir)
	for _, e := range entries {
		raw, err := os.ReadFile(filepath.Join(qdir, e.Name(), "question.json"))
		if err != nil {
			continue
		}
		var q map[string]interface{}
		_ = json.Unmarshal(raw, &q)
		if q["ROUE_BONUS_COUNT"] != float64(2) {
			t.Errorf("ROUE_BONUS_COUNT = %v, attendu 2", q["ROUE_BONUS_COUNT"])
		}
		for _, k := range []string{"ROUE_WHEEL_MAX_POINTS", "ROUE_WHEEL_STEP", "ROUE_WHEEL_BANKRUPT_COUNT", "ROUE_WHEEL_LOSE_TURN_COUNT"} {
			if _, present := q[k]; present {
				t.Errorf("%s ne doit pas être écrit pour un BONUS seul", k)
			}
		}
	}
}
