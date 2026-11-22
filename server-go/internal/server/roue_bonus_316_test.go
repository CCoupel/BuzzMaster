// #316 / #317 (v12.2.0, LOT-1) — HTTP : BONUS posé sur une case à points et mode éphémère / permanent.
// Source : contracts/roue.md §17.3 (limites et refus), §17.5 (mode), §17.6 (wheel-preview), §17.7 (migration).
// Frontière HTTP uniquement ; réutilise postRoue145, questionDirCount145, postWheelPreview144, wheelBody144,
// postGameConfig144 / getGameConfig144 des tests #144 / #145. Attendu ROUGE tant que les tâches 2 et 5 du plan ne sont pas livrées.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// ---- aides --------------------------------------------------------------------------------------------------------

type previewSeg316 struct {
	ID    string `json:"ID"`
	Kind  string `json:"KIND"`
	Value int    `json:"VALUE"`
	Bonus *bool  `json:"BONUS"`
}

type preview316 struct {
	Count    int             `json:"COUNT"`
	Segments []previewSeg316 `json:"SEGMENTS"`
}

func bonusBody316(max, step, b, p int, extra string) string {
	return strings.TrimSuffix(wheelBody144(max, step, b, p), "}") + extra + `}`
}

func preview316Of(t *testing.T, s *HTTPServer, body string) preview316 {
	t.Helper()
	w := postWheelPreview144(t, s, body)
	if w.Code != http.StatusOK {
		t.Fatalf("wheel-preview %s : %d %s", body, w.Code, w.Body.String())
	}
	var out preview316
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func (p preview316) marks() []int {
	var out []int
	for i, s := range p.Segments {
		if s.Bonus != nil && *s.Bonus {
			out = append(out, i)
		}
	}
	return out
}

func (p preview316) render() string {
	parts := make([]string, 0, len(p.Segments))
	for _, s := range p.Segments {
		switch s.Kind {
		case "bankrupt":
			parts = append(parts, "BANQ")
		case "lose_turn":
			parts = append(parts, "PERDU")
		default:
			v := fmt.Sprint(s.Value)
			if s.Bonus != nil && *s.Bonus {
				v += "+"
			}
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, " ")
}

func refusalCode316(t *testing.T, body []byte) string {
	t.Helper()
	var m map[string]string
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("corps de refus non JSON : %s", body)
	}
	return m["CODE"]
}

// ---- POST /api/roue/wheel-preview ---------------------------------------------------------------------------------

// critical — l'aperçu renvoie les marques d'UN tirage : COUNT = V + B + P (les BONUS ne comptent plus), chaque case
// porte BONUS (bool, toujours présent), `bonus_count` marques sur des cases à points, aucune voisine d'une case
// spéciale, aucun KIND « bonus ».
func TestRoueBonus316_Preview_MarksAndCount(t *testing.T) {
	for _, c := range []struct {
		max, step, b, p, bonus, cells int
	}{
		{1000, 50, 2, 1, 4, 22}, {1000, 50, 2, 1, 2, 22}, {1000, 100, 2, 1, 2, 12}, {100, 20, 1, 1, 1, 7},
		{1000, 100, 3, 2, 1, 14}, {100, 10, 0, 0, 3, 10}, {64, 1, 0, 0, 4, 64},
	} {
		t.Run(fmt.Sprintf("%d/%d/%d/%d+%d", c.max, c.step, c.b, c.p, c.bonus), func(t *testing.T) {
			s, _ := setupTestHTTPServer(t)
			for call := 0; call < 20; call++ {
				got := preview316Of(t, s, bonusBody316(c.max, c.step, c.b, c.p, fmt.Sprintf(`,"bonus_count":%d`, c.bonus)))
				if got.Count != c.cells || len(got.Segments) != c.cells {
					t.Fatalf("COUNT %d / %d cases, attendu %d (BONUS exclus)", got.Count, len(got.Segments), c.cells)
				}
				for i, sg := range got.Segments {
					if sg.Bonus == nil {
						t.Fatalf("la case %d n'a pas le champ BONUS : toujours présent, même à false", i)
					}
					if sg.Kind == "bonus" {
						t.Fatalf("KIND « bonus » retiré en v12.2.0")
					}
					if *sg.Bonus && sg.Kind != "value" {
						t.Fatalf("marque sur une case %q", sg.Kind)
					}
				}
				m := got.marks()
				if len(m) != c.bonus {
					t.Fatalf("%d marques, attendu %d\n%s", len(m), c.bonus, got.render())
				}
				n := len(got.Segments)
				for _, i := range m {
					for _, d := range []int{-1, 1} {
						nb := got.Segments[(i+d+n)%n]
						if nb.Kind != "value" || (nb.Bonus != nil && *nb.Bonus) {
							t.Fatalf("la marque de la case %d touche une case spéciale\n%s", i, got.render())
						}
					}
				}
			}
		})
	}
}

// Sans bonus_count (ou 0) : aperçu identique à v12.0 / v12.1, case pour case ; aucune marque.
func TestRoueBonus316_Preview_WithoutBonusIsTheV120Wheel(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	const want = "100 BANQ 90 10 80 PERDU 20 70 30 60 40"
	for _, body := range []string{wheelBody144(100, 10, 1, 1), bonusBody316(100, 10, 1, 1, `,"bonus_count":0`)} {
		got := preview316Of(t, s, body)
		if got.render() != want || len(got.marks()) != 0 || got.Count != 11 {
			t.Fatalf("%s : %q (COUNT %d, %d marques), attendu %q", body, got.render(), got.Count, len(got.marks()), want)
		}
	}
}

// Les cases BANQUEROUTE / PERDU et les valeurs sont identiques d'un appel à l'autre ; les marques changent de place
// (« 🎲 Autre tirage ») ; un nouvel appel peut les poser ailleurs.
func TestRoueBonus316_Preview_SpecialsStableMarksMove(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	plain := func(p preview316) string {
		q := p
		q.Segments = append([]previewSeg316(nil), p.Segments...)
		f := false
		for i := range q.Segments {
			q.Segments[i].Bonus = &f
		}
		return q.render()
	}
	body := bonusBody316(1000, 50, 2, 1, `,"bonus_count":2`)
	first := plain(preview316Of(t, s, body))
	layouts := map[string]bool{}
	for i := 0; i < 60; i++ {
		got := preview316Of(t, s, body)
		if plain(got) != first {
			t.Fatalf("valeurs / BANQUEROUTE / PERDU modifiées entre deux tirages :\n%s\n%s", plain(got), first)
		}
		layouts[fmt.Sprint(got.marks())] = true
	}
	if len(layouts) < 5 {
		t.Fatalf("60 appels ne donnent que %d disposition(s) de marques", len(layouts))
	}
}

// Refus : trop de BONUS pour la roue → 400 ROUE_BONUS_COUNT_INVALID (jamais réduit dans l'aperçu), hors 0–4 aussi ;
// une roue invalide même sans BONUS reste ROUE_WHEEL_INVALID ; la limite de 64 cases s'applique à N = V + B + P seulement.
func TestRoueBonus316_Preview_Refusals(t *testing.T) {
	cases := []struct {
		name, body, code string
	}{
		{"3 BONUS sur 1000/100/2/1 (max 2)", bonusBody316(1000, 100, 2, 1, `,"bonus_count":3`), "ROUE_BONUS_COUNT_INVALID"},
		{"1 BONUS sur 100/30/1/1 (max 0)", bonusBody316(100, 30, 1, 1, `,"bonus_count":1`), "ROUE_BONUS_COUNT_INVALID"},
		{"2 BONUS sur 1000/100/3/2 (max 1)", bonusBody316(1000, 100, 3, 2, `,"bonus_count":2`), "ROUE_BONUS_COUNT_INVALID"},
		{"5 BONUS (hors 0–4)", bonusBody316(1000, 50, 2, 1, `,"bonus_count":5`), "ROUE_BONUS_COUNT_INVALID"},
		{"-1 BONUS", bonusBody316(1000, 50, 2, 1, `,"bonus_count":-1`), "ROUE_BONUS_COUNT_INVALID"},
		{"roue invalide (65 cases)", bonusBody316(65, 1, 0, 0, `,"bonus_count":0`), "ROUE_WHEEL_INVALID"},
		{"roue invalide + BONUS : la roue d'abord", bonusBody316(65, 1, 0, 0, `,"bonus_count":2`), "ROUE_WHEEL_INVALID"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, _ := setupTestHTTPServer(t)
			w := postWheelPreview144(t, s, c.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("attendu 400, got %d : %s", w.Code, w.Body.String())
			}
			if got := refusalCode316(t, w.Body.Bytes()); got != c.code {
				t.Fatalf("CODE = %q, attendu %q (%s)", got, c.code, w.Body.String())
			}
		})
	}
	// Le message de refus donne le maximum (« Trop de BONUS : N au plus »).
	s, _ := setupTestHTTPServer(t)
	w := postWheelPreview144(t, s, bonusBody316(1000, 100, 2, 1, `,"bonus_count":3`))
	if !strings.Contains(w.Body.String(), "2") || !strings.Contains(strings.ToLower(w.Body.String()), "bonus") {
		t.Errorf("le message doit nommer le maximum permis (2) : %s", w.Body.String())
	}
}

// Les BONUS ne comptent plus dans les 64 cases : une roue de 64 cases accepte ses 4 BONUS.
func TestRoueBonus316_Preview_BonusDoNotCountInto64Cases(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	got := preview316Of(t, s, bonusBody316(64, 1, 0, 0, `,"bonus_count":4`))
	if got.Count != 64 || len(got.marks()) != 4 {
		t.Fatalf("64 cases + 4 marques attendues : COUNT %d, %d marques", got.Count, len(got.marks()))
	}
}

// `bonus_mode` accepté et ignoré : même réponse (hors positions des marques) avec ou sans.
func TestRoueBonus316_Preview_BonusModeAcceptedAndIgnored(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	for _, mode := range []string{"EPHEMERAL", "PERMANENT"} {
		got := preview316Of(t, s, bonusBody316(1000, 50, 2, 1, `,"bonus_count":2,"bonus_mode":"`+mode+`"`))
		if got.Count != 22 || len(got.marks()) != 2 {
			t.Fatalf("bonus_mode %s : COUNT %d, %d marques", mode, got.Count, len(got.marks()))
		}
	}
}

// ---- POST /questions ----------------------------------------------------------------------------------------------

// critical — roue_bonus_mode : absent ou vide = global (champ NON écrit) ; EPHEMERAL / PERMANENT enregistrés tels
// quels ; relus à l'identique, même avec 0 BONUS.
func TestRoueBonus317_Upload_ModeAbsentEmptyAndValues(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := postRoue145(t, s, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if _, present := saved["ROUE_BONUS_MODE"]; present {
		t.Errorf("champ absent : ROUE_BONUS_MODE ne doit pas être écrit (%v)", saved["ROUE_BONUS_MODE"])
	}
	s2, _ := setupTestHTTPServer(t)
	_, saved = postRoue145(t, s2, map[string]string{"roue_bonus_mode": ""})
	if _, present := saved["ROUE_BONUS_MODE"]; present {
		t.Errorf("champ vide = retiré (suit le global) : %v", saved["ROUE_BONUS_MODE"])
	}
	for _, mode := range []string{"EPHEMERAL", "PERMANENT"} {
		for _, bonus := range []string{"0", "2"} {
			t.Run(mode+"/bonus="+bonus, func(t *testing.T) {
				s3, _ := setupTestHTTPServer(t)
				w, saved := postRoue145(t, s3, map[string]string{"roue_bonus_mode": mode, "roue_bonus_count": bonus})
				if w.Code != http.StatusOK {
					t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
				}
				if saved["ROUE_BONUS_MODE"] != mode {
					t.Errorf("ROUE_BONUS_MODE = %v, attendu %s", saved["ROUE_BONUS_MODE"], mode)
				}
			})
		}
	}
}

// Relu à chaque enregistrement (leçon #107) : un second enregistrement sans le champ retire le mode ; les autres champs
// ROUE_* sont relus à l'identique quand roue_bonus_mode est présent.
func TestRoueBonus317_Upload_ModeIsRereadAndOtherFieldsRoundTrip(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := postRoue145(t, s, map[string]string{
		"roue_bonus_mode": "PERMANENT", "roue_bonus_count": "2", "ROUE_MODE": "CHACUN_SON_TOUR", "roue_gains_rule": "ALL_TEAMS",
		"roue_wheel_layout": "arc", "roue_show_letters": "false",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if saved["ROUE_BONUS_MODE"] != "PERMANENT" || saved["ROUE_MODE"] != "CHACUN_SON_TOUR" || saved["ROUE_GAINS_RULE"] != "ALL_TEAMS" ||
		saved["ROUE_WHEEL_LAYOUT"] != "arc" || saved["ROUE_SHOW_LETTERS"] != false {
		t.Errorf("champs ROUE_* altérés : %v", saved)
	}
	if got, _ := saved["ROUE_BONUS_COUNT"].(float64); int(got) != 2 {
		t.Errorf("ROUE_BONUS_COUNT = %v, attendu 2", saved["ROUE_BONUS_COUNT"])
	}
}

// Refus 400 préfixé par le code, rien d'écrit, dossier de création supprimé.
func TestRoueBonus317_Upload_InvalidModeRefused(t *testing.T) {
	for _, v := range []string{"FOREVER", "ephemeral", "1", "TRUE", "PERMANENT "} {
		t.Run(fmt.Sprintf("%q", v), func(t *testing.T) {
			s, _ := setupTestHTTPServer(t)
			w, saved := postRoue145(t, s, map[string]string{"roue_bonus_mode": v})
			code := w.Body.String()
			if v == "PERMANENT " {
				// espaces de fin : tolérés (rognés) ou refusés, jamais une écriture altérée
				if w.Code == http.StatusOK {
					if saved["ROUE_BONUS_MODE"] != "PERMANENT" {
						t.Fatalf("valeur rognée attendue : %v", saved["ROUE_BONUS_MODE"])
					}
					return
				}
			}
			if w.Code != http.StatusBadRequest || !strings.HasPrefix(code, "ROUE_BONUS_MODE_INVALID") {
				t.Fatalf("attendu 400 ROUE_BONUS_MODE_INVALID, got %d %q", w.Code, code)
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

// §17.3 — BONUS au plus : un nombre envoyé au-dessus du maximum de la roue effective est refusé
// ROUE_BONUS_COUNT_INVALID ; au maximum, accepté ; rien n'est écrit sur un refus.
func TestRoueBonus316_Upload_CountAboveTheWheelMaximumRefused(t *testing.T) {
	roue := map[string]string{"roue_wheel_max_points": "1000", "roue_wheel_step": "100", "roue_wheel_bankrupt_count": "2", "roue_wheel_lose_turn_count": "1"}
	with := func(bonus string) map[string]string {
		m := map[string]string{"roue_bonus_count": bonus}
		for k, v := range roue {
			m[k] = v
		}
		return m
	}
	s, _ := setupTestHTTPServer(t)
	w, saved := postRoue145(t, s, with("3")) // 1000/100/2/1 : 2 au plus
	if w.Code != http.StatusBadRequest || !strings.HasPrefix(w.Body.String(), "ROUE_BONUS_COUNT_INVALID") {
		t.Fatalf("3 BONUS sur 12 cases : attendu 400 ROUE_BONUS_COUNT_INVALID, got %d %q", w.Code, w.Body.String())
	}
	if saved != nil {
		t.Errorf("rien ne doit être écrit : %v", saved)
	}
	if n := questionDirCount145(t); n != 0 {
		t.Errorf("dossier de création refusée non supprimé (%d)", n)
	}
	if w, _ := postRoue145(t, s, with("2")); w.Code != http.StatusOK {
		t.Fatalf("2 BONUS (le maximum) : attendu 200, got %d %s", w.Code, w.Body.String())
	}
}

// BONUS hérité du réglage global (règle §16.5) : la roue personnalisée de la question réduit la place, le BONUS global
// hérité dépasse → refus ROUE_BONUS_COUNT_INVALID.
func TestRoueBonus316_Upload_InheritedGlobalCountAboveMaximumRefused(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	if w := postGameConfig144(t, s, `{"wheel":{"max_points":1000,"step":50,"bankrupt_count":2,"lose_turn_count":1,"bonus_count":4}}`); w.Code != http.StatusOK {
		t.Fatalf("réglage global à 4 BONUS refusé : %d %s", w.Code, w.Body.String())
	}
	w, saved := postRoue145(t, s, map[string]string{"roue_wheel_step": "100"}) // 12 cases : 2 au plus, 4 hérités
	if w.Code != http.StatusBadRequest || !strings.HasPrefix(w.Body.String(), "ROUE_BONUS_COUNT_INVALID") {
		t.Fatalf("attendu 400 ROUE_BONUS_COUNT_INVALID (BONUS hérité), got %d %q", w.Code, w.Body.String())
	}
	if saved != nil {
		t.Errorf("rien ne doit être écrit : %v", saved)
	}
}

// Une roue invalide même sans BONUS est ROUE_WHEEL_INVALID (la roue d'abord, puis les BONUS).
func TestRoueBonus316_Upload_InvalidWheelIsReportedBeforeBonus(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := postRoue145(t, s, map[string]string{"roue_wheel_max_points": "1000", "roue_wheel_step": "1", "roue_bonus_count": "2"})
	if w.Code != http.StatusBadRequest || !strings.HasPrefix(w.Body.String(), "ROUE_WHEEL_INVALID") {
		t.Fatalf("attendu 400 ROUE_WHEEL_INVALID, got %d %q", w.Code, w.Body.String())
	}
	if saved != nil {
		t.Errorf("rien ne doit être écrit : %v", saved)
	}
}

// Les BONUS ne comptent plus dans les 64 cases (v12.1 : « ferait dépasser 64 cases ») : une roue à 63 cases accepte 2 BONUS.
func TestRoueBonus316_Upload_BonusNoLongerCountedInto64Cases(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	if w := postGameConfig144(t, s, `{"wheel":{"max_points":63,"step":1,"bankrupt_count":0,"lose_turn_count":0}}`); w.Code != http.StatusOK {
		t.Fatalf("roue globale à 63 cases refusée : %d %s", w.Code, w.Body.String())
	}
	w, saved := postRoue145(t, s, map[string]string{"roue_bonus_count": "2"})
	if w.Code != http.StatusOK {
		t.Fatalf("63 cases + 2 BONUS : attendu 200 (les BONUS ne comptent plus), got %d %q", w.Code, w.Body.String())
	}
	if got, _ := saved["ROUE_BONUS_COUNT"].(float64); int(got) != 2 {
		t.Errorf("ROUE_BONUS_COUNT = %v, attendu 2", saved["ROUE_BONUS_COUNT"])
	}
}

// Migration (§17.7) : une question v12.1 (ROUE_BONUS_COUNT = 2, sans mode) se ré-enregistre sans erreur et reste sans mode.
func TestRoueBonus316_Upload_V121QuestionResavesWithoutError(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w, saved := postRoue145(t, s, map[string]string{"roue_bonus_count": "2"})
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if _, present := saved["ROUE_BONUS_MODE"]; present {
		t.Errorf("question v12.1 : aucun mode écrit, elle suit le global (%v)", saved["ROUE_BONUS_MODE"])
	}
	if got, _ := saved["ROUE_BONUS_COUNT"].(float64); int(got) != 2 {
		t.Errorf("ROUE_BONUS_COUNT = %v", saved["ROUE_BONUS_COUNT"])
	}
}

// ---- game-config.json : wheel.bonus_mode et bonus_count ------------------------------------------------------------

func wheelSection316(t *testing.T, s *HTTPServer) (wheel, def map[string]interface{}) {
	t.Helper()
	_, _, raw := getGameConfig144(t, s)
	if err := json.Unmarshal(raw["wheel"], &wheel); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw["wheel_default"], &def); err != nil {
		t.Fatal(err)
	}
	return
}

// critical — défaut éphémère : section absente ou sans `bonus_mode` = EPHEMERAL ; wheel_default.bonus_mode = EPHEMERAL.
func TestRoueBonus317_GameConfig_DefaultIsEphemeral(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	wheel, def := wheelSection316(t, s)
	if wheel["bonus_mode"] != "EPHEMERAL" || def["bonus_mode"] != "EPHEMERAL" {
		t.Fatalf("bonus_mode : wheel %v / wheel_default %v, attendu EPHEMERAL des deux côtés", wheel["bonus_mode"], def["bonus_mode"])
	}
	// Une configuration enregistrée sans bonus_mode (v12.1) se charge sans erreur, éphémère.
	if w := postGameConfig144(t, s, `{"wheel":{"max_points":500,"step":100,"bankrupt_count":1,"lose_turn_count":1,"bonus_count":1}}`); w.Code != http.StatusOK {
		t.Fatalf("POST sans bonus_mode : %d %s", w.Code, w.Body.String())
	}
	wheel, _ = wheelSection316(t, s)
	if wheel["bonus_mode"] != "EPHEMERAL" {
		t.Fatalf("sans bonus_mode envoyé : %v, attendu EPHEMERAL", wheel["bonus_mode"])
	}
}

// POST + relecture : PERMANENT, puis EPHEMERAL, enregistrés tels quels ; les autres champs de la roue ne bougent pas.
func TestRoueBonus317_GameConfig_ModeSavedAndReread(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	for _, mode := range []string{"PERMANENT", "EPHEMERAL"} {
		body := fmt.Sprintf(`{"wheel":{"max_points":1000,"step":50,"bankrupt_count":2,"lose_turn_count":1,"bonus_count":2,"bonus_mode":%q}}`, mode)
		if w := postGameConfig144(t, s, body); w.Code != http.StatusOK {
			t.Fatalf("POST %s : %d %s", mode, w.Code, w.Body.String())
		}
		wheel, _ := wheelSection316(t, s)
		if wheel["bonus_mode"] != mode || wheel["bonus_count"] != float64(2) || wheel["max_points"] != float64(1000) {
			t.Fatalf("relecture : %v, attendu mode %s, 2 BONUS, 1000", wheel, mode)
		}
	}
}

// Valeur inconnue : 400 et rien n'est écrit (la configuration précédente est intacte).
func TestRoueBonus317_GameConfig_InvalidModeRefusedNothingWritten(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	if w := postGameConfig144(t, s, `{"wheel":{"max_points":1000,"step":50,"bankrupt_count":2,"lose_turn_count":1,"bonus_count":1,"bonus_mode":"PERMANENT"}}`); w.Code != http.StatusOK {
		t.Fatalf("préparation : %d %s", w.Code, w.Body.String())
	}
	for _, v := range []string{"FOREVER", "permanent", "1"} {
		w := postGameConfig144(t, s, fmt.Sprintf(`{"wheel":{"max_points":500,"step":100,"bankrupt_count":1,"lose_turn_count":1,"bonus_count":1,"bonus_mode":%q}}`, v))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("bonus_mode %q : attendu 400, got %d %s", v, w.Code, w.Body.String())
		}
		wheel, _ := wheelSection316(t, s)
		if wheel["bonus_mode"] != "PERMANENT" || wheel["max_points"] != float64(1000) {
			t.Fatalf("un refus ne doit rien écrire : %v", wheel)
		}
	}
}

// §17.3 : `bonus_count` au-dessus du maximum de la roue → 400 et rien n'est écrit ; au maximum accepté ; hors 0–4 refusé.
func TestRoueBonus316_GameConfig_BonusCountAboveMaximumRefused(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	if w := postGameConfig144(t, s, `{"wheel":{"max_points":1000,"step":50,"bankrupt_count":2,"lose_turn_count":1,"bonus_count":0}}`); w.Code != http.StatusOK {
		t.Fatalf("préparation : %d %s", w.Code, w.Body.String())
	}
	refused := []string{
		`{"wheel":{"max_points":1000,"step":100,"bankrupt_count":2,"lose_turn_count":1,"bonus_count":3}}`, // max 2
		`{"wheel":{"max_points":100,"step":30,"bankrupt_count":1,"lose_turn_count":1,"bonus_count":1}}`,   // max 0
		`{"wheel":{"max_points":1000,"step":100,"bankrupt_count":3,"lose_turn_count":2,"bonus_count":2}}`, // max 1
		`{"wheel":{"max_points":1000,"step":50,"bankrupt_count":2,"lose_turn_count":1,"bonus_count":5}}`,
		`{"wheel":{"max_points":1000,"step":50,"bankrupt_count":2,"lose_turn_count":1,"bonus_count":-1}}`,
	}
	for _, body := range refused {
		if w := postGameConfig144(t, s, body); w.Code != http.StatusBadRequest {
			t.Fatalf("%s : attendu 400, got %d %s", body, w.Code, w.Body.String())
		}
		if wheel, _ := wheelSection316(t, s); wheel["step"] != float64(50) || wheel["bonus_count"] != float64(0) {
			t.Fatalf("un refus ne doit rien écrire : %v", wheel)
		}
	}
	for _, body := range []string{
		`{"wheel":{"max_points":1000,"step":100,"bankrupt_count":2,"lose_turn_count":1,"bonus_count":2}}`,
		`{"wheel":{"max_points":1000,"step":50,"bankrupt_count":2,"lose_turn_count":1,"bonus_count":4}}`,
		`{"wheel":{"max_points":64,"step":1,"bankrupt_count":0,"lose_turn_count":0,"bonus_count":4}}`, // 64 cases : BONUS non comptés
	} {
		if w := postGameConfig144(t, s, body); w.Code != http.StatusOK {
			t.Fatalf("%s : attendu 200, got %d %s", body, w.Code, w.Body.String())
		}
	}
}

// Une roue sans BONUS valable en v12.1 le reste : aucune règle nouvelle ne la refuse.
func TestRoueBonus316_GameConfig_WheelWithoutBonusStaysValid(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	for _, body := range []string{
		`{"wheel":{"max_points":100,"step":30,"bankrupt_count":1,"lose_turn_count":1}}`,
		`{"wheel":{"max_points":1000,"step":100,"bankrupt_count":3,"lose_turn_count":2}}`,
		`{"wheel":{"max_points":64,"step":1,"bankrupt_count":0,"lose_turn_count":0}}`,
		`{"wheel":{"max_points":100,"step":10,"bankrupt_count":0,"lose_turn_count":0}}`,
	} {
		if w := postGameConfig144(t, s, body); w.Code != http.StatusOK {
			t.Fatalf("%s : une roue sans BONUS valable en v12.1 doit le rester, got %d %s", body, w.Code, w.Body.String())
		}
	}
}
