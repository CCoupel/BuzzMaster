package effects

import (
	"strings"
	"testing"
)

// B2.1 (#210) — dev tests: error paths, description forms, hostile texts in a document.

func devDoc(t *testing.T, body string) Document {
	t.Helper()
	return DocumentOf(mustParse(t, fx(body)))
}

func TestSerialize_ErrorPaths_Dev(t *testing.T) {
	t.Parallel()
	d := devDoc(t, "variables:\n  gA: 1; +1\npas:\n  - { gA: [theme, 100] }\n  - { g1: [theme, 100] }\n")
	bad := func(mut func(*Document), wantPath string) {
		t.Helper()
		c := cloneDocument(d)
		mut(&c)
		_, errs := Serialize(c)
		if len(errs) == 0 || errs[0].Path != wantPath || errs[0].Line != 0 {
			t.Errorf("attendu path %q, obtenu %+v", wantPath, errs)
		}
	}
	bad(func(c *Document) { c.Variables[0].Start = "N//2" }, "variables[0].start")
	bad(func(c *Document) { s := "gA"; c.Variables[0].Step = &s }, "variables[0].step")
	bad(func(c *Document) { c.TickMs = 10 }, "tick_ms")
	bad(func(c *Document) { c.Steps[1][0].Color = "rose" }, "steps[1]")
	bad(func(c *Document) { c.Steps[1][0].Intensity = nil }, "steps[1]")
	bad(func(c *Document) { c.Steps[0][0].Target = "gZ" }, "steps[0]")
	bad(func(c *Document) { c.Moments = nil }, "moments")
	bad(func(c *Document) { c.ID = "nul" }, "id")
	bad(func(c *Document) { c.ID = "draft" }, "id")
	bad(func(c *Document) { c.Steps = nil }, "steps")
	bad(func(c *Document) { c.Variables[0].Kind = "autre" }, "variables[0]")
}

func TestSerialize_DescriptionAndHostileText_Dev(t *testing.T) {
	t.Parallel()
	d := devDoc(t, "pas:\n  - { g1: [theme, 100] }\n")
	for _, desc := range []string{"", "une ligne", "une ligne\n", "deux\nlignes", "  espaces autour  ", "avec: deux-points # et dièse", "\"guillemets\" et 'apostrophes'", "tab\there", "- tiret", strings.Repeat("é", 400)} {
		c := cloneDocument(d)
		c.Description = desc
		c.Name = "Nom: avec # piège"
		text, errs := Serialize(c)
		if len(errs) > 0 {
			t.Errorf("%q : %+v", desc, errs)
			continue
		}
		e, perrs := Parse([]byte(text))
		if len(perrs) > 0 || e.Description != desc || e.Name != c.Name {
			t.Errorf("%q : relu %q / %q (%+v)\n%s", desc, e.Description, e.Name, perrs, text)
		}
	}
}

func TestCustomOrderAndComments_Dev(t *testing.T) {
	t.Parallel()
	ordered := mustParse(t, fx("variables:\n  gA: 1; +1\npas:\n  - { g0: [theme, 100], g1: [theme, 100], g*: [theme, 100], gA-1: [theme, 50], gA: [urgence, 100] }\n"))
	if CustomOrder(ordered) {
		t.Error("ordre canonique : custom_order faux")
	}
	if !CustomOrder(mustParse(t, fx("pas:\n  - { g2: [theme, 100], g1: [theme, 100] }\n"))) {
		t.Error("g2 avant g1 : custom_order vrai")
	}
	if !CustomOrder(mustParse(t, fx("variables:\n  gA: 1; +1\npas:\n  - { gA: [theme, 100], g0: [theme, 100] }\n"))) {
		t.Error("une variable avant g0 : custom_order vrai")
	}
	if !HasComments([]byte("# c\nid: x\n")) || !HasComments([]byte("id: x # c\n")) || HasComments([]byte("id: x\nnom: \"a # b\"\n")) {
		t.Error("has_comments")
	}
}

func TestGridBlockedAndOrder_Dev(t *testing.T) {
	t.Parallel()
	g := BuildGrid(devDoc(t, "variables:\n  gA: 1; +1\npas:\n  - { gA+1: [theme, 50], gA-1: [theme, 50], gA: [urgence, 100] }\n"))
	if !g.Editable || len(g.Converted) != 2 || g.Converted[0].To != "gB" || g.Converted[1].To != "gC" {
		t.Fatalf("%+v", g)
	}
	var names []string
	for _, v := range g.Effect.Variables {
		names = append(names, v.Name+"="+v.Start)
	}
	// inserted before gA by increasing offset: gA-1 (gC) then gA+1 (gB)
	if strings.Join(names, ",") != "gC=gA-1,gB=gA+1,gA=1" {
		t.Errorf("ordre %v", names)
	}
}
