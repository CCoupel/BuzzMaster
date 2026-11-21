// #147 (v12.1.0, LOT-1.2) — achat de voyelles côté HTTP et liste blanche : POST /questions (6 champs voyelles),
// ROUE_BUY_VOWEL (qui peut l'envoyer). Source : contracts/roue.md §15.1, §15.3. Réutilise postRoue145 / questionDirCount145.
package server

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"buzzcontrol/internal/protocol"
)

// critical — question v12.0 (aucun champ voyelle) : les réglages par défaut sont écrits, achat activé (champ non écrit).
func TestRoueVowels147_Upload_DefaultsWritten(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := postRoue145(t, s, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	want := map[string]interface{}{"ROUE_VOWEL_TIMING": "BEFORE", "ROUE_VOWELS_BEFORE": "MANY", "ROUE_VOWELS_AFTER": "MANY", "ROUE_VOWEL_PRICE": float64(250), "ROUE_VOWEL_GAIN": "NONE"}
	for k, v := range want {
		if saved[k] != v {
			t.Errorf("%s = %v, attendu %v", k, saved[k], v)
		}
	}
	if _, present := saved["ROUE_BUY_VOWELS"]; present {
		t.Errorf("ROUE_BUY_VOWELS n'est écrit que pour false : %v", saved["ROUE_BUY_VOWELS"])
	}
}

func TestRoueVowels147_Upload_ValuesPersistedAndRelu(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := postRoue145(t, s, map[string]string{
		"roue_buy_vowels": "true", "roue_vowel_timing": "BOTH", "roue_vowels_before": "ONE", "roue_vowels_after": "ONE",
		"roue_vowel_price": "400", "roue_vowel_gain": "PER_OCCURRENCE",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	want := map[string]interface{}{"ROUE_VOWEL_TIMING": "BOTH", "ROUE_VOWELS_BEFORE": "ONE", "ROUE_VOWELS_AFTER": "ONE", "ROUE_VOWEL_PRICE": float64(400), "ROUE_VOWEL_GAIN": "PER_OCCURRENCE"}
	for k, v := range want {
		if saved[k] != v {
			t.Errorf("%s = %v, attendu %v", k, saved[k], v)
		}
	}
	if _, present := saved["ROUE_BUY_VOWELS"]; present {
		t.Errorf("roue_buy_vowels=true : le champ n'est pas écrit (absent = true)")
	}
}

// Achat désactivé : ROUE_BUY_VOWELS = false écrit, et les autres réglages sont CONSERVÉS pour une réactivation.
func TestRoueVowels147_Upload_DisabledWrittenAndSettingsKept(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := postRoue145(t, s, map[string]string{"roue_buy_vowels": "false", "roue_vowel_timing": "AFTER", "roue_vowel_price": "500", "roue_vowel_gain": "REFUND", "roue_vowels_after": "ONE"})
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if saved["ROUE_BUY_VOWELS"] != false {
		t.Errorf("ROUE_BUY_VOWELS = %v, attendu false", saved["ROUE_BUY_VOWELS"])
	}
	if saved["ROUE_VOWEL_TIMING"] != "AFTER" || saved["ROUE_VOWEL_PRICE"] != float64(500) || saved["ROUE_VOWEL_GAIN"] != "REFUND" || saved["ROUE_VOWELS_AFTER"] != "ONE" {
		t.Errorf("réglages non conservés : %v", saved)
	}
}

func TestRoueVowels147_Upload_PriceBounds(t *testing.T) {
	for _, p := range []string{"1", "250", "1000000"} {
		s, _ := setupTestHTTPServer(t)
		if w, _ := postRoue145(t, s, map[string]string{"roue_vowel_price": p}); w.Code != http.StatusOK {
			t.Errorf("prix %s accepté attendu, got %d %s", p, w.Code, w.Body.String())
		}
	}
}

// Refus 400 préfixés par leur code, rien d'écrit, dossier de création supprimé.
func TestRoueVowels147_Upload_RefusalCodes(t *testing.T) {
	cases := []struct{ field, value, code string }{
		{"roue_vowel_timing", "NEVER", "ROUE_VOWEL_TIMING_INVALID"},
		{"roue_vowel_timing", "before", "ROUE_VOWEL_TIMING_INVALID"},
		{"roue_vowels_before", "TWO", "ROUE_VOWELS_BEFORE_INVALID"},
		{"roue_vowels_after", "many", "ROUE_VOWELS_AFTER_INVALID"},
		{"roue_vowel_price", "0", "ROUE_VOWEL_PRICE_INVALID"},
		{"roue_vowel_price", "-5", "ROUE_VOWEL_PRICE_INVALID"},
		{"roue_vowel_price", "1000001", "ROUE_VOWEL_PRICE_INVALID"},
		{"roue_vowel_price", "abc", "ROUE_VOWEL_PRICE_INVALID"},
		{"roue_vowel_price", "2.5", "ROUE_VOWEL_PRICE_INVALID"},
		{"roue_vowel_gain", "DOUBLE", "ROUE_VOWEL_GAIN_INVALID"},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s=%s", c.field, c.value), func(t *testing.T) {
			s, _ := setupTestHTTPServer(t)
			w, saved := postRoue145(t, s, map[string]string{c.field: c.value})
			if w.Code != http.StatusBadRequest || !strings.HasPrefix(w.Body.String(), c.code) {
				t.Fatalf("attendu 400 %s, got %d %q", c.code, w.Code, w.Body.String())
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

// Les autres champs ROUE_* (dont BONUS) restent relus à l'identique avec les champs voyelles (leçon #107).
func TestRoueVowels147_Upload_OtherFieldsStillRoundTrip(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := postRoue145(t, s, map[string]string{"roue_vowel_timing": "AFTER", "roue_bonus_count": "2", "ROUE_MODE": "SOLO", "roue_gains_rule": "ALL_TEAMS", "roue_show_letters": "false"})
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if saved["ROUE_MODE"] != "SOLO" || saved["ROUE_GAINS_RULE"] != "ALL_TEAMS" || saved["ROUE_SHOW_LETTERS"] != false || saved["ROUE_BONUS_COUNT"] != float64(2) || saved["ROUE_VOWEL_TIMING"] != "AFTER" {
		t.Errorf("champs altérés : %v", saved)
	}
}

// ---- liste blanche (§8.2) ----------------------------------------------------------------------------------------

// ROUE_BUY_VOWEL : admin et animateur seulement ; jamais TV ni tablette ; refusée pendant un entracte.
func TestRoueVowels147_AllowlistBuyVowel(t *testing.T) {
	a := protocol.ActionRoueBuyVowel
	if a != "ROUE_BUY_VOWEL" {
		t.Fatalf("nom de l'action = %q, attendu ROUE_BUY_VOWEL", a)
	}
	if !IsActionAllowed(a, ClientTypeAdmin) || !IsActionAllowed(a, ClientTypeAnim) {
		t.Errorf("admin et animateur doivent pouvoir envoyer %s", a)
	}
	for _, ct := range []ClientType{ClientTypeTV, ClientTypeVPlayer} {
		if IsActionAllowed(a, ct) {
			t.Errorf("%s doit être refusée au type de client %v", a, ct)
		}
	}
	if IsActionAllowedDuringEntracte(a) {
		t.Errorf("%s doit être refusée pendant un entracte", a)
	}
}
