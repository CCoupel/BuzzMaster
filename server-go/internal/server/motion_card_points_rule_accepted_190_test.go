package server

import (
	"net/http"
	"testing"
)

// #190 (LOT-5, v11.2.2) — POST /questions MEMOTION : règles POINTS_RULE
// ACCEPTÉES (HTTP 200) et persistées telles quelles. Pendant de
// motion_card_points_rule_190_test.go (refus 400, helpers partagés :
// pointsRuleCard, withRule, rule, ruleValue, postPointsRuleCards). Scindé pour
// rester < 50 cas par fichier. Même règle TEST_PROCEDURE.md §2.3 : frontière
// HTTP uniquement, aucun symbole nouveau du paquet game.

// ---------------------------------------------------------------------------
// 200 — règles acceptées (et persistées telles quelles)
// ---------------------------------------------------------------------------

func TestHTTPServer_MEMOTIONUpload_PointsRule_Accepted(t *testing.T) {
	tests := []struct {
		name      string
		typ       string
		rule      interface{}
		absent    bool   // POINTS_RULE absent de la carte envoyée
		wantMode  string // "" ⇒ aucune règle persistée
		wantValue int
	}{
		{name: "POINTS_RULE absent (SPEEDY) — défaut, comportement historique", typ: "SPEEDY", absent: true},
		{name: "POINTS_RULE absent (carte sans TYPE)", typ: "", absent: true},
		{name: "POINTS_RULE absent (MEMORY)", typ: "MEMORY", absent: true},
		{name: "POINTS_RULE null (QCM) — traité comme absent", typ: "QCM", rule: nil},
		{name: "MODE vide (RAFALE) — traité comme défaut", typ: "RAFALE", rule: rule("")},
		{name: "STARS sur SPEEDY", typ: "SPEEDY", rule: rule("STARS"), wantMode: "STARS"},
		{name: "STARS explicite sur QCM — idempotence du défaut", typ: "QCM", rule: rule("STARS"), wantMode: "STARS"},
		{name: "FIXED 10 sur QCM", typ: "QCM", rule: ruleValue("FIXED", 10), wantMode: "FIXED", wantValue: 10},
		{name: "FIXED 10 sur carte sans TYPE (⇒ SPEEDY)", typ: "", rule: ruleValue("FIXED", 10), wantMode: "FIXED", wantValue: 10},
		{name: "FIXED 1 (borne basse)", typ: "QCM", rule: ruleValue("FIXED", 1), wantMode: "FIXED", wantValue: 1},
		{name: "FIXED 999 (borne haute)", typ: "SPEEDY", rule: ruleValue("FIXED", 999), wantMode: "FIXED", wantValue: 999},
		{name: "STARS_PRORATA sur MEMORY — idempotence du défaut du type", typ: "MEMORY", rule: rule("STARS_PRORATA"), wantMode: "STARS_PRORATA"},
		{name: "STARS_PRORATA sur RAFALE", typ: "RAFALE", rule: rule("STARS_PRORATA"), wantMode: "STARS_PRORATA"},
		{name: "STARS sur MEMORY (étoiles pleines, tout ou rien)", typ: "MEMORY", rule: rule("STARS"), wantMode: "STARS"},
		{name: "PER_UNIT 2 sur RAFALE", typ: "RAFALE", rule: ruleValue("PER_UNIT", 2), wantMode: "PER_UNIT", wantValue: 2},
		{name: "PER_UNIT 5 sur MEMORY", typ: "MEMORY", rule: ruleValue("PER_UNIT", 5), wantMode: "PER_UNIT", wantValue: 5},
		{name: "FIXED 7 sur MEMORY", typ: "MEMORY", rule: ruleValue("FIXED", 7), wantMode: "FIXED", wantValue: 7},
		{name: "VALUE ignorée pour STARS (même hors bornes) : acceptée", typ: "SPEEDY", rule: ruleValue("STARS", 5000), wantMode: "STARS"},
		{name: "VALUE ignorée pour STARS_PRORATA (même nulle) : acceptée", typ: "MEMORY", rule: ruleValue("STARS_PRORATA", 0), wantMode: "STARS_PRORATA"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := pointsRuleCard("mc-1", tt.typ)
			if !tt.absent {
				withRule(card, tt.rule)
			}
			w, dataDir := postPointsRuleCards(t, []map[string]interface{}{card}, nil)
			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
			}

			q := readWrittenMEMOTIONQuestion(t, dataDir)
			if len(q.MotionCards) != 1 {
				t.Fatalf("expected 1 motion card, got %d", len(q.MotionCards))
			}
			got := q.MotionCards[0].PointsRule
			if tt.wantMode == "" {
				if got != nil && string(got.Mode) != "" {
					t.Errorf("expected no persisted POINTS_RULE, got MODE=%q VALUE=%d", got.Mode, got.Value)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected POINTS_RULE %s to be persisted verbatim, got none", tt.wantMode)
			}
			if string(got.Mode) != tt.wantMode {
				t.Errorf("persisted MODE = %q, want %q", got.Mode, tt.wantMode)
			}
			if tt.wantValue != 0 && got.Value != tt.wantValue {
				t.Errorf("persisted VALUE = %d, want %d", got.Value, tt.wantValue)
			}
		})
	}
}

// Non-régression : une carte valide avec image est toujours acceptée quand
// elle porte une règle valide (la validation ne casse pas le traitement des images).
func TestHTTPServer_MEMOTIONUpload_PointsRule_AcceptedWithImageStillWritesIt(t *testing.T) {
	cards := []map[string]interface{}{
		withRule(pointsRuleCard("mc-1", "QCM"), ruleValue("FIXED", 10)),
	}
	images := map[string][]byte{"motion_card_mc-1_recto": minimalPNG}
	w, dataDir := postPointsRuleCards(t, cards, images)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	q := readWrittenMEMOTIONQuestion(t, dataDir)
	if q.MotionCards[0].RectoImage == "" {
		t.Error("expected RECTO_IMAGE to be set for the accepted card")
	}
	if q.MotionCards[0].PointsRule == nil || string(q.MotionCards[0].PointsRule.Mode) != "FIXED" {
		t.Errorf("expected POINTS_RULE FIXED persisted, got %+v", q.MotionCards[0].PointsRule)
	}
}

// Non-régression : une question MEMOTION sans aucune POINTS_RULE (cas de toutes
// les questions antérieures à #190) reste acceptée à l'identique.
func TestHTTPServer_MEMOTIONUpload_PointsRule_LegacyQuestionWithoutRuleAccepted(t *testing.T) {
	cards := []map[string]interface{}{
		pointsRuleCard("mc-1", ""),
		pointsRuleCard("mc-2", "QCM"),
		pointsRuleCard("mc-3", "MEMORY"),
		pointsRuleCard("mc-4", "RAFALE"),
	}
	w, dataDir := postPointsRuleCards(t, cards, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	q := readWrittenMEMOTIONQuestion(t, dataDir)
	if len(q.MotionCards) != 4 {
		t.Fatalf("expected 4 motion cards, got %d", len(q.MotionCards))
	}
	for i, c := range q.MotionCards {
		if c.PointsRule != nil && string(c.PointsRule.Mode) != "" {
			t.Errorf("card %d: no POINTS_RULE expected, got %+v", i+1, c.PointsRule)
		}
	}
}
