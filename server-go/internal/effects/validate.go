package effects

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v4"
)

var (
	reID      = regexp.MustCompile(`^[a-z0-9-]{1,40}$`)
	reHex     = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	reGn      = regexp.MustCompile(`^g([1-9][0-9]?)$`)
	reVarName = regexp.MustCompile(`^g[A-Z][A-Za-z0-9]*$`)
	reVarRef  = regexp.MustCompile(`^(g[A-Z][A-Za-z0-9]*)([+-][0-9]+)?$`)
)

var namedColors = map[string]bool{"theme": true, "equipe": true, "urgence": true, "scene": true}

type validator struct {
	file string
	errs []Error
	vars map[string]bool
}

func (v *validator) add(line int, format string, args ...any) {
	v.errs = append(v.errs, Error{File: v.file, Line: line, Message: fmt.Sprintf(format, args...)})
}

// addPart is add for a problem of the start or the step of a variable (`variables[i].start`).
func (v *validator) addPart(line int, part, format string, args ...any) {
	v.errs = append(v.errs, Error{File: v.file, Line: line, Message: fmt.Sprintf(format, args...), part: part})
}

// build checks the decoded file against the schema of §4 and returns the effect.
func (v *validator) build(r *rawEffect, present map[string]int) Effect {
	e := Effect{ID: r.ID, Name: r.Nom, Description: r.Description, Version: r.Version, Moments: r.Moments,
		MinGroups: r.GroupesMin, TickMs: r.DureeTickMs, Loop: true}
	for _, k := range []string{"id", "nom", "version", "moments", "duree_tick_ms", "pas"} {
		if _, ok := present[k]; !ok {
			v.add(0, "la clé « %s » est obligatoire", k)
		}
	}
	if line, ok := present["id"]; ok && !reID.MatchString(e.ID) {
		v.add(line, "l'identifiant « %s » est invalide : 1 à %d caractères parmi a-z, 0-9 et -", e.ID, MaxIDLen)
	}
	if line, ok := present["nom"]; ok {
		if n := utf8.RuneCountInString(e.Name); n < 1 || n > MaxNameLen {
			v.add(line, "« nom » compte de 1 à %d caractères", MaxNameLen)
		}
	}
	if utf8.RuneCountInString(e.Description) > MaxDescLen {
		v.add(present["description"], "la description dépasse %d caractères", MaxDescLen)
	}
	if line, ok := present["version"]; ok && e.Version != SchemaVersion {
		v.add(line, "la version %d du format n'est pas prise en charge (seule la version %d l'est)", e.Version, SchemaVersion)
	}
	if line, ok := present["moments"]; ok {
		if len(e.Moments) == 0 {
			v.add(line, "« moments » ne peut pas être vide (chrono, calm, score)")
		}
		for _, m := range e.Moments {
			if !Moment(m).Valid() {
				v.add(line, "le moment « %s » est inconnu (chrono, calm ou score)", m)
			}
		}
	}
	if line, ok := present["groupes_min"]; ok && (e.MinGroups < 0 || e.MinGroups > MaxGroups) {
		v.add(line, "groupes_min doit être compris entre 0 et %d", MaxGroups)
	}
	if line, ok := present["duree_tick_ms"]; ok && (e.TickMs < MinTickMs || e.TickMs > MaxTickMs) {
		v.add(line, "duree_tick_ms doit être compris entre %d et %d ms", MinTickMs, MaxTickMs)
	}
	if r.Boucle.Kind != 0 {
		switch strings.ToLower(r.Boucle.Value) {
		case "oui", "true", "yes":
			e.Loop = true
		case "non", "false", "no":
			e.Loop = false
		default:
			v.add(r.Boucle.Line, "« boucle » doit valoir oui ou non")
		}
	}
	v.vars = map[string]bool{}
	if r.Variables.Kind != 0 {
		e.Vars = v.variables(&r.Variables)
	}
	if r.Pre.Kind != 0 {
		e.Pre, e.HasPre = v.step(&r.Pre, "pre"), true
	}
	if r.Pas.Kind != 0 {
		if r.Pas.Kind != yaml.SequenceNode {
			v.add(r.Pas.Line, "« pas » doit être une liste de pas")
		} else {
			if n := len(r.Pas.Content); n < 1 || n > MaxSteps {
				v.add(r.Pas.Line, "« pas » compte de 1 à %d pas", MaxSteps)
			}
			for _, n := range r.Pas.Content {
				e.Steps = append(e.Steps, v.step(n, "pas"))
			}
		}
	}
	if r.Dernier.Kind != 0 {
		e.Last, e.HasLast = v.step(&r.Dernier, "dernier"), true
	}
	if r.Post.Kind != 0 {
		e.Post, e.HasPost = v.step(&r.Post, "post"), true
	}
	return e
}

// variables parses the `variables` table (§4.4, §12.2). Two passes: declarations and
// expression syntax, then the references between variables (declared, no loop).
func (v *validator) variables(n *yaml.Node) []Var {
	if n.Kind != yaml.MappingNode {
		v.add(n.Line, "« variables » doit être une table « gA: g1; +1 »")
		return nil
	}
	if len(n.Content)/2 > MaxVars {
		v.add(n.Line, "%d variables au plus", MaxVars)
	}
	var out []Var
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, val := n.Content[i], n.Content[i+1]
		if !reVarName.MatchString(k.Value) {
			v.add(k.Line, "le nom de variable « %s » est invalide : « g » suivi d'une lettre majuscule, puis de lettres ou de chiffres", k.Value)
			continue
		}
		if v.vars[k.Value] {
			v.add(k.Line, "la variable « %s » est déclarée deux fois", k.Value)
			continue
		}
		v.vars[k.Value] = true // registered at once: a bad declaration does not make its uses "undeclared"
		vr := Var{Name: k.Value, Line: k.Line}
		switch val.Kind {
		case yaml.ScalarNode:
			if !v.progression(&vr, val) {
				continue
			}
		case yaml.SequenceNode:
			vr.IsList = true
			if len(val.Content) == 0 {
				v.add(val.Line, "la liste de « %s » est vide", k.Value)
				continue
			}
			bad := false
			for _, it := range val.Content {
				m := reGn.FindStringSubmatch(it.Value)
				if it.Kind != yaml.ScalarNode || m == nil {
					v.add(it.Line, "l'élément « %s » de la liste « %s » doit être un groupe g1, g2…", it.Value, k.Value)
					bad = true
					continue
				}
				nn, _ := strconv.Atoi(m[1])
				vr.List = append(vr.List, Target{Kind: TargetGn, N: nn})
			}
			if bad {
				continue
			}
		default:
			v.add(val.Line, "la déclaration de « %s » est mal écrite : « g1; +1 », « N/2 » ou une liste [g1, g3]", k.Value)
			continue
		}
		out = append(out, vr)
	}
	v.references(out)
	return out
}

// progression parses "start" or "start; step" into vr; false when an error was reported.
func (v *validator) progression(vr *Var, val *yaml.Node) bool {
	parts := strings.Split(val.Value, ";")
	if len(parts) > 2 {
		v.add(val.Line, "la déclaration de « %s » est mal écrite : « départ » ou « départ; pas » (un seul « ; »)", vr.Name)
		return false
	}
	start, err := ParseExpr(parts[0])
	if err != nil {
		v.addPart(val.Line, "start", "la déclaration de « %s » : %s", vr.Name, err.Error())
		return false
	}
	vr.Start = start
	if len(parts) == 2 {
		step, err := ParseExpr(parts[1])
		if err != nil {
			v.addPart(val.Line, "step", "la déclaration de « %s » : %s", vr.Name, err.Error())
			return false
		}
		if refs := step.Refs(); len(refs) > 0 {
			v.addPart(val.Line, "step", "le pas ne peut pas citer une variable (%s)", refs[0])
			return false
		}
		vr.Step = step
	}
	return true
}

// undeclared reports a variable used in a target or an expression without a declaration.
func (v *validator) undeclared(line int, name, part string) {
	if name == "gN" {
		v.addPart(line, part, "« gN » n'est pas déclarée dans variables. gN est désormais une variable comme les autres : pour le dernier groupe, écrivez g*")
		return
	}
	v.addPart(line, part, "la variable « %s » n'est pas déclarée dans variables", name)
}

// references checks that the variables cited by the starts are declared and that no loop
// links them (§12.2).
func (v *validator) references(vars []Var) {
	byName := map[string]*Var{}
	for i := range vars {
		byName[vars[i].Name] = &vars[i]
	}
	ok := map[string]bool{}
	for i := range vars {
		vr := &vars[i]
		if vr.IsList {
			ok[vr.Name] = true
			continue
		}
		good := true
		for _, ref := range vr.Start.Refs() {
			if !v.vars[ref] {
				v.undeclared(vr.Line, ref, "start")
				good = false
			}
		}
		ok[vr.Name] = good
	}
	// Loops: depth-first search over the (valid) citations.
	const (
		white = iota
		grey
		black
	)
	colour := map[string]int{}
	var stack []string
	reported := map[string]bool{}
	var visit func(name string)
	visit = func(name string) {
		vr := byName[name]
		if vr == nil || vr.IsList || !ok[name] {
			return
		}
		colour[name] = grey
		stack = append(stack, name)
		for _, ref := range vr.Start.Refs() {
			switch colour[ref] {
			case grey:
				// the cycle is the stack from ref to here
				i := len(stack) - 1
				for stack[i] != ref {
					i--
				}
				cyc := append([]string(nil), stack[i:]...)
				key := strings.Join(sortedCopy(cyc), ",")
				if !reported[key] {
					reported[key] = true
					v.add(byName[cyc[0]].Line, "%s", cycleMessage(cyc))
				}
			case white:
				visit(ref)
			}
		}
		stack = stack[:len(stack)-1]
		colour[name] = black
	}
	for i := range vars {
		if colour[vars[i].Name] == white {
			visit(vars[i].Name)
		}
	}
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func cycleMessage(cyc []string) string {
	switch len(cyc) {
	case 1:
		return fmt.Sprintf("la variable %s dépend d'elle-même", cyc[0])
	case 2:
		return fmt.Sprintf("les variables %s et %s dépendent l'une de l'autre", cyc[0], cyc[1])
	}
	return fmt.Sprintf("les variables %s et %s dépendent les unes des autres", strings.Join(cyc[:len(cyc)-1], ", "), cyc[len(cyc)-1])
}

// parseTarget reads the left side of an action; ok=false on a malformed write.
func parseTarget(s string) (Target, bool, string) {
	switch {
	case s == "g0":
		return Target{Kind: TargetG0}, true, ""
	case s == "g*":
		return Target{Kind: TargetGN}, true, ""
	}
	if m := reGn.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		return Target{Kind: TargetGn, N: n}, true, ""
	}
	if m := reVarRef.FindStringSubmatch(s); m != nil {
		off := 0
		if m[2] != "" {
			off, _ = strconv.Atoi(m[2])
		}
		return Target{Kind: TargetVar, Var: m[1], Offset: off}, true, m[1]
	}
	return Target{}, false, ""
}

// step parses one step table (pre, an element of pas, dernier, post): `cible: action`.
func (v *validator) step(n *yaml.Node, where string) Step {
	if n.Kind != yaml.MappingNode {
		v.add(n.Line, "un pas de « %s » doit être une table « g0: [theme, 100] »", where)
		return nil
	}
	if c := len(n.Content) / 2; c < 1 || c > MaxActions {
		v.add(n.Line, "un pas compte de 1 à %d actions", MaxActions)
	}
	var out Step
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, val := n.Content[i], n.Content[i+1]
		tg, ok, varName := parseTarget(k.Value)
		if !ok {
			v.add(k.Line, "la cible « %s » est mal écrite (g0, g1…, g*, ou une variable comme gA)", k.Value)
			continue
		}
		if varName != "" && !v.vars[varName] {
			v.undeclared(k.Line, varName, "")
			continue
		}
		act := Action{Target: tg, Line: k.Line}
		if val.Kind != yaml.SequenceNode || len(val.Content) == 0 {
			v.add(val.Line, "l'action de « %s » doit être [couleur, intensité] ou [eteint]", k.Value)
			continue
		}
		if len(val.Content) == 1 && val.Content[0].Value == "eteint" {
			act.Off = true
			out = append(out, act)
			continue
		}
		if len(val.Content) != 2 {
			v.add(val.Line, "l'action de « %s » doit être [couleur, intensité] ou [eteint]", k.Value)
			continue
		}
		col := val.Content[0].Value
		switch {
		case namedColors[col]:
			act.Color = Color{Name: col}
		case reHex.MatchString(col):
			var r, g, b int
			_, _ = fmt.Sscanf(col, "#%02x%02x%02x", &r, &g, &b)
			act.Color = Color{RGB: [3]int{r, g, b}}
		default:
			v.add(val.Content[0].Line, "la couleur « %s » est invalide (theme, equipe, urgence, scene ou #RRGGBB)", col)
			continue
		}
		pct, err := strconv.Atoi(val.Content[1].Value)
		if err != nil || pct < 0 || pct > 100 {
			v.add(val.Content[1].Line, "l'intensité de « %s » doit être un entier de 0 à 100", k.Value)
			continue
		}
		act.Pct = pct
		out = append(out, act)
	}
	return out
}
