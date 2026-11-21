// #308 (v12.1.0, LOT-1.1) — case BONUS côté HTTP : POST /questions (roue_bonus_count), POST /api/roue/wheel-preview
// (bonus_count). Source : contracts/roue.md §15.2, §15.3. Frontière HTTP uniquement (réutilise postRoue145,
// postWheelPreview144, wheelBody144, wheelPreview144 des tests #144 / #145).
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---- POST /questions ---------------------------------------------------------------------------------------

// critical — question v12.0 (champ absent) : ROUE_BONUS_COUNT n'est pas écrit, relecture = 0 BONUS.
func TestRoueBonus308_Upload_AbsentFieldMeansZeroAndIsNotWritten(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := postRoue145(t, s, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if _, present := saved["ROUE_BONUS_COUNT"]; present {
		t.Errorf("ROUE_BONUS_COUNT ne doit pas être écrit quand il est absent : %v", saved["ROUE_BONUS_COUNT"])
	}
}

// 0 explicite : accepté, non écrit (omitempty).
func TestRoueBonus308_Upload_ZeroIsAcceptedAndNotWritten(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := postRoue145(t, s, map[string]string{"roue_bonus_count": "0"})
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if _, present := saved["ROUE_BONUS_COUNT"]; present {
		t.Errorf("ROUE_BONUS_COUNT = 0 ne doit pas être écrit : %v", saved["ROUE_BONUS_COUNT"])
	}
}

// 1 à 4 : enregistré en entier et relu à l'identique.
func TestRoueBonus308_Upload_ValidCountsPersisted(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, _ := setupTestHTTPServer(t)
			w, saved := postRoue145(t, s, map[string]string{"roue_bonus_count": fmt.Sprint(n)})
			if w.Code != http.StatusOK {
				t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
			}
			if got, ok := saved["ROUE_BONUS_COUNT"].(float64); !ok || int(got) != n {
				t.Errorf("ROUE_BONUS_COUNT = %v, attendu %d", saved["ROUE_BONUS_COUNT"], n)
			}
		})
	}
}

// Refus 400 préfixé par le code, rien d'écrit, dossier de création supprimé : hors 0–4 ou valeur non entière.
func TestRoueBonus308_Upload_InvalidCountRefused(t *testing.T) {
	for _, v := range []string{"5", "-1", "10", "abc", "1.5", "2x"} {
		t.Run(fmt.Sprintf("%q", v), func(t *testing.T) {
			s, _ := setupTestHTTPServer(t)
			w, saved := postRoue145(t, s, map[string]string{"roue_bonus_count": v})
			if w.Code != http.StatusBadRequest || !strings.HasPrefix(w.Body.String(), "ROUE_BONUS_COUNT_INVALID") {
				t.Fatalf("attendu 400 ROUE_BONUS_COUNT_INVALID, got %d %q", w.Code, w.Body.String())
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

// Un champ vide (formulaire sans valeur) = absent = 0, comme les autres champs ROUE_* facultatifs.
func TestRoueBonus308_Upload_EmptyFieldIsTreatedAsAbsent(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := postRoue145(t, s, map[string]string{"roue_bonus_count": ""})
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if _, present := saved["ROUE_BONUS_COUNT"]; present {
		t.Errorf("champ vide : ROUE_BONUS_COUNT ne doit pas être écrit")
	}
}

// Les autres champs ROUE_* sont relus à l'identique quand roue_bonus_count est présent (leçon #107 : aucun champ perdu).
func TestRoueBonus308_Upload_OtherRoueFieldsStillRoundTrip(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := postRoue145(t, s, map[string]string{
		"roue_bonus_count": "2", "ROUE_MODE": "CHACUN_SON_TOUR", "roue_gains_rule": "ALL_TEAMS",
		"roue_wheel_layout": "arc", "roue_show_letters": "false",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if saved["ROUE_MODE"] != "CHACUN_SON_TOUR" || saved["ROUE_GAINS_RULE"] != "ALL_TEAMS" ||
		saved["ROUE_WHEEL_LAYOUT"] != "arc" || saved["ROUE_SHOW_LETTERS"] != false {
		t.Errorf("champs ROUE_* altérés : %v", saved)
	}
	if got, _ := saved["ROUE_BONUS_COUNT"].(float64); int(got) != 2 {
		t.Errorf("ROUE_BONUS_COUNT = %v, attendu 2", saved["ROUE_BONUS_COUNT"])
	}
}

// Limite de 64 cases : la roue courante (game-config) plus les BONUS dépasserait 64 → refus ROUE_BONUS_COUNT_INVALID.
// On règle la configuration à 63 cases (max 63 / pas 1) : 1 BONUS tient (64), 2 dépassent.
func TestRoueBonus308_Upload_CountThatWouldExceed64CasesRefused(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	cfg := strings.NewReader(`{"wheel":{"max_points":63,"step":1,"bankrupt_count":0,"lose_turn_count":0}}`)
	req, _ := http.NewRequest("POST", "/game-config.json", cfg)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Skipf("configuration de la roue à 63 cases non acceptée (%d %s) : cas non réalisable dans ce harnais", rec.Code, rec.Body.String())
	}
	w, saved := postRoue145(t, s, map[string]string{"roue_bonus_count": "2"})
	if w.Code != http.StatusBadRequest || !strings.HasPrefix(w.Body.String(), "ROUE_BONUS_COUNT_INVALID") {
		t.Fatalf("63 cases + 2 BONUS : attendu 400 ROUE_BONUS_COUNT_INVALID, got %d %q", w.Code, w.Body.String())
	}
	if saved != nil {
		t.Errorf("rien ne doit être écrit : %v", saved)
	}
	w, _ = postRoue145(t, s, map[string]string{"roue_bonus_count": "1"})
	if w.Code != http.StatusOK {
		t.Fatalf("63 cases + 1 BONUS = 64 : attendu 200, got %d %s", w.Code, w.Body.String())
	}
}

// ---- POST /api/roue/wheel-preview --------------------------------------------------------------------------

func bonusBody308(max, step, b, p int, bonus string) string {
	return strings.TrimSuffix(wheelBody144(max, step, b, p), "}") + `,"bonus_count":` + bonus + `}`
}

// Additif : sans bonus_count, l'aperçu reste identique aux 10 exemples du contrat (champ facultatif, absent = 0).
func TestRoueBonus308_WheelPreview_AbsentBonusCountKeepsV120Wheel(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	var without, zero wheelPreview144
	for _, c := range []struct {
		body string
		out  *wheelPreview144
	}{{wheelBody144(100, 10, 1, 1), &without}, {bonusBody308(100, 10, 1, 1, "0"), &zero}} {
		w := postWheelPreview144(t, s, c.body)
		if w.Code != http.StatusOK {
			t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
		}
		if err := json.Unmarshal(w.Body.Bytes(), c.out); err != nil {
			t.Fatal(err)
		}
	}
	const want = "100 BANQ 90 10 80 PERDU 20 70 30 60 40"
	if without.render() != want || zero.render() != want {
		t.Fatalf("sans BONUS : %q / %q, attendu %q", without.render(), zero.render(), want)
	}
}

// bonus_count de 1 à 4 : cases KIND "bonus" en plus, COUNT augmenté, même placement qu'au START.
func TestRoueBonus308_WheelPreview_CountsAndKinds(t *testing.T) {
	for n := 1; n <= 4; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, _ := setupTestHTTPServer(t)
			w := postWheelPreview144(t, s, bonusBody308(100, 10, 1, 1, fmt.Sprint(n)))
			if w.Code != http.StatusOK {
				t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
			}
			var got wheelPreview144
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			bonus := 0
			for _, sg := range got.Segments {
				if sg.Kind == "bonus" {
					bonus++
					if sg.Value != 0 {
						t.Errorf("case BONUS %s : VALUE %d, attendu 0", sg.ID, sg.Value)
					}
				}
			}
			if bonus != n || got.Count != 11+n || len(got.Segments) != 11+n {
				t.Fatalf("bonus=%d : %d BONUS, COUNT %d, %d cases (attendu %d BONUS et %d cases)", n, bonus, got.Count, len(got.Segments), n, 11+n)
			}
			if got.Segments[0].Value != 100 || got.Segments[1].Kind != "bankrupt" {
				t.Errorf("maximum à l'index 0 et BANQUEROUTE à l'index 1 attendus : %+v %+v", got.Segments[0], got.Segments[1])
			}
		})
	}
}

// Valeur hors 0–4 ou qui ferait dépasser 64 cases : refus 400 ; (code non fixé par le contrat pour l'aperçu :
// ROUE_WHEEL_INVALID, comme les autres refus de wheel-preview, ou ROUE_BONUS_COUNT_INVALID — l'un des deux).
func TestRoueBonus308_WheelPreview_InvalidBonusCountRefused(t *testing.T) {
	cases := map[string]string{
		"hors bornes (5)":  bonusBody308(100, 10, 1, 1, "5"),
		"négatif":          bonusBody308(100, 10, 1, 1, "-1"),
		"dépasse 64 cases": bonusBody308(63, 1, 0, 0, "2"),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			s, _ := setupTestHTTPServer(t)
			w := postWheelPreview144(t, s, body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("attendu 400, got %d : %s", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), "ROUE_WHEEL_INVALID") && !strings.Contains(w.Body.String(), "ROUE_BONUS_COUNT_INVALID") {
				t.Errorf("code de refus inattendu : %s", w.Body.String())
			}
		})
	}
}
