// Tests for #190 LOT-5 — ValidatePointsRule / AllowedPointsRuleModes
// (contract question-types.md §6.2). Run: go test ./internal/game/... -run PointsRule.*190 -v

package game

import (
	"errors"
	"strings"
	"testing"
)

func nestableTypes() []QuestionType {
	var out []QuestionType
	for _, qt := range AllQuestionTypes() {
		if IsNestableInMotionCard(qt) {
			out = append(out, qt)
		}
	}
	return out
}

func hasPointsRuleMode(modes []PointsRuleMode, m PointsRuleMode) bool {
	for _, x := range modes {
		if x == m {
			return true
		}
	}
	return false
}

// Exhaustiveness: a nestable type added later without a sensible mode list
// breaks here rather than silently refusing/accepting everything.
func TestAllowedPointsRuleModes_EveryNestableTypeContainsItsDefault_190(t *testing.T) {
	nestable := nestableTypes()
	if len(nestable) == 0 {
		t.Fatal("no nestable type found")
	}
	for _, qt := range nestable {
		modes := AllowedPointsRuleModes(qt)
		if len(modes) == 0 {
			t.Errorf("%s: empty AllowedPointsRuleModes", qt)
			continue
		}
		def := questionTypeRegistry[qt].DefaultPointsRule
		if def == "" {
			def = PointsRuleModeStars
		}
		if !hasPointsRuleMode(modes, def) {
			t.Errorf("%s: allowed modes %v do not contain the type default %s", qt, modes, def)
		}
		if modes[0] != def {
			t.Errorf("%s: default %s should be listed first, got %v", qt, def, modes)
		}
	}
}

func TestAllowedPointsRuleModes_ByType_190(t *testing.T) {
	binary := []PointsRuleMode{PointsRuleModeStars, PointsRuleModeFixed}
	progression := []PointsRuleMode{PointsRuleModeStarsProrata, PointsRuleModeStars, PointsRuleModeFixed, PointsRuleModePerUnit}
	want := map[QuestionType][]PointsRuleMode{
		QuestionTypeSpeedy: binary, QuestionTypeQCM: binary,
		QuestionTypeMemory: progression, QuestionTypeRafale: progression,
	}
	for qt, w := range want {
		got := AllowedPointsRuleModes(qt)
		if len(got) != len(w) {
			t.Errorf("%s: got %v, want %v", qt, got, w)
			continue
		}
		for i := range w {
			if got[i] != w[i] {
				t.Errorf("%s: got %v, want %v", qt, got, w)
				break
			}
		}
	}
	if got := AllowedPointsRuleModes("NOPE"); got != nil {
		t.Errorf("unknown type: got %v, want nil", got)
	}
}

func ruleCard(rule interface{}) map[string]interface{} {
	c := map[string]interface{}{"ID": "mc-7"}
	if rule != "<absent>" {
		c["POINTS_RULE"] = rule
	}
	return c
}

func TestValidatePointsRule_Table_190(t *testing.T) {
	const (
		inv = PointsRuleErrInvalidMode
		na  = PointsRuleErrModeNotApplicable
		val = PointsRuleErrValueRequired
	)
	rule := func(mode string, value interface{}) interface{} {
		m := map[string]interface{}{"MODE": mode}
		if value != "<absent>" {
			m["VALUE"] = value
		}
		return m
	}
	const abs = "<absent>"

	type tc struct {
		name string
		rule interface{}
		want string // "" = valid
	}
	common := []tc{
		{"POINTS_RULE absent", abs, ""},
		{"POINTS_RULE null", nil, ""},
		{"POINTS_RULE empty object", map[string]interface{}{}, ""},
		{"MODE empty", rule("", abs), ""},
		{"MODE null", map[string]interface{}{"MODE": nil}, ""},
		{"MODE unknown BONUS", rule("BONUS", 5.0), inv},
		{"MODE lowercase", rule("fixed", 5.0), inv},
		{"MODE not a string", map[string]interface{}{"MODE": 3.0}, inv},
		{"POINTS_RULE not an object", "FIXED", inv},
		{"STARS without VALUE", rule("STARS", abs), ""},
		{"STARS with junk VALUE ignored", rule("STARS", "abc"), ""},
		{"FIXED VALUE absent", rule("FIXED", abs), val},
		{"FIXED VALUE 0", rule("FIXED", 0.0), val},
		{"FIXED VALUE 1", rule("FIXED", 1.0), ""},
		{"FIXED VALUE 999", rule("FIXED", 999.0), ""},
		{"FIXED VALUE 1000", rule("FIXED", 1000.0), val},
		{"FIXED VALUE negative", rule("FIXED", -3.0), val},
		{"FIXED VALUE 2.5", rule("FIXED", 2.5), val},
		{"FIXED VALUE string", rule("FIXED", "10"), val},
		{"FIXED VALUE null", rule("FIXED", nil), val},
		{"FIXED VALUE int 10", rule("FIXED", 10), ""},
	}
	progressionOnly := []tc{
		{"STARS_PRORATA no VALUE", rule("STARS_PRORATA", abs), ""},
		{"PER_UNIT VALUE absent", rule("PER_UNIT", abs), val},
		{"PER_UNIT VALUE 0", rule("PER_UNIT", 0.0), val},
		{"PER_UNIT VALUE 2", rule("PER_UNIT", 2.0), ""},
		{"PER_UNIT VALUE 999", rule("PER_UNIT", 999.0), ""},
		{"PER_UNIT VALUE 1000", rule("PER_UNIT", 1000.0), val},
		{"PER_UNIT VALUE 2.5", rule("PER_UNIT", 2.5), val},
	}
	binaryOnly := []tc{
		{"STARS_PRORATA refused", rule("STARS_PRORATA", abs), na},
		{"PER_UNIT refused (VALUE ok)", rule("PER_UNIT", 2.0), na},
		{"PER_UNIT refused (VALUE absent: NOT_APPLICABLE wins)", rule("PER_UNIT", abs), na},
	}

	for _, qt := range nestableTypes() {
		cases := append([]tc{}, common...)
		if hasPointsRuleMode(AllowedPointsRuleModes(qt), PointsRuleModePerUnit) {
			cases = append(cases, progressionOnly...)
		} else {
			cases = append(cases, binaryOnly...)
		}
		for _, c := range cases {
			t.Run(string(qt)+"/"+c.name, func(t *testing.T) {
				err := ValidatePointsRule(qt, ruleCard(c.rule))
				if c.want == "" {
					if err != nil {
						t.Fatalf("want valid, got %v", err)
					}
					return
				}
				var pe *PointsRuleError
				if !errors.As(err, &pe) {
					t.Fatalf("want *PointsRuleError %s, got %v", c.want, err)
				}
				if pe.Code != c.want {
					t.Errorf("code = %s, want %s (%v)", pe.Code, c.want, err)
				}
				if !strings.HasPrefix(err.Error(), c.want+": card mc-7 (TYPE="+string(qt)+") ") {
					t.Errorf("message shape: %q", err.Error())
				}
			})
		}
	}
}

// Empty TYPE means SPEEDY (binary), as in ValidateCardTypeContent.
func TestValidatePointsRule_EmptyTypeIsSpeedy_190(t *testing.T) {
	err := ValidatePointsRule("", ruleCard(map[string]interface{}{"MODE": "STARS_PRORATA"}))
	var pe *PointsRuleError
	if !errors.As(err, &pe) || pe.Code != PointsRuleErrModeNotApplicable || pe.Type != QuestionTypeSpeedy {
		t.Fatalf("want NOT_APPLICABLE on SPEEDY, got %v", err)
	}
	if err := ValidatePointsRule("", ruleCard(map[string]interface{}{"MODE": "FIXED", "VALUE": 10.0})); err != nil {
		t.Fatalf("FIXED 10 on empty TYPE: %v", err)
	}
}

// Idempotence: an explicit mode equal to the type default is accepted.
func TestValidatePointsRule_ExplicitDefaultAccepted_190(t *testing.T) {
	if err := ValidatePointsRule(QuestionTypeMemory, ruleCard(map[string]interface{}{"MODE": "STARS_PRORATA"})); err != nil {
		t.Errorf("MEMORY STARS_PRORATA: %v", err)
	}
	if err := ValidatePointsRule(QuestionTypeSpeedy, ruleCard(map[string]interface{}{"MODE": "STARS"})); err != nil {
		t.Errorf("SPEEDY STARS: %v", err)
	}
}

// Exact message texts, documented in the handoff for the frontend/test-writer.
func TestValidatePointsRule_MessageTexts_190(t *testing.T) {
	cases := []struct {
		typ  QuestionType
		rule map[string]interface{}
		want string
	}{
		{QuestionTypeSpeedy, map[string]interface{}{"MODE": "BONUS"},
			"POINTS_RULE_INVALID_MODE: card mc-7 (TYPE=SPEEDY) declares POINTS_RULE.MODE=BONUS, which is not one of STARS, FIXED, PER_UNIT, STARS_PRORATA"},
		{QuestionTypeSpeedy, map[string]interface{}{"MODE": "PER_UNIT", "VALUE": 2.0},
			"POINTS_RULE_MODE_NOT_APPLICABLE: card mc-7 (TYPE=SPEEDY) declares POINTS_RULE.MODE=PER_UNIT, which is not applicable to this type (accepted: STARS, FIXED)"},
		{QuestionTypeRafale, map[string]interface{}{"MODE": "FIXED"},
			"POINTS_RULE_VALUE_REQUIRED: card mc-7 (TYPE=RAFALE) declares POINTS_RULE.MODE=FIXED, which requires an integer VALUE between 1 and 999"},
	}
	for _, c := range cases {
		err := ValidatePointsRule(c.typ, ruleCard(c.rule))
		if err == nil || err.Error() != c.want {
			t.Errorf("got %v\nwant %s", err, c.want)
		}
	}
}
