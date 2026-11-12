package effects

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v4"
)

// document.go — the JSON exchange form of an effect (contract light-effects.md §11.1) and the
// editor grid view (§11.2, E5). The server is the only parser, validator and interpreter:
// a Document is turned into a file text (serialize.go) and checked by the very same parser as
// the files.

// DocAction is one action of a step: `target: [colour, intensity]` or `[eteint]`.
type DocAction struct {
	Target    string `json:"target"`
	Color     string `json:"color,omitempty"`     // "#RRGGBB" or theme | equipe | urgence | scene
	Intensity *int   `json:"intensity,omitempty"` // 0-100
	Off       bool   `json:"off,omitempty"`
}

// DocVariable is one declared variable. Kind "progression": Start and Step are expressions in
// text (Step nil = « suit »); kind "list": Items.
type DocVariable struct {
	Name  string   `json:"name"`
	Kind  string   `json:"kind"`
	Start string   `json:"start,omitempty"`
	Step  *string  `json:"step"`
	Items []string `json:"items,omitempty"`
}

// MarshalJSON omits start and step of a list.
func (v DocVariable) MarshalJSON() ([]byte, error) {
	if v.Kind == "list" {
		items := v.Items
		if items == nil {
			items = []string{}
		}
		return json.Marshal(struct {
			Name  string   `json:"name"`
			Kind  string   `json:"kind"`
			Items []string `json:"items"`
		}{v.Name, v.Kind, items})
	}
	return json.Marshal(struct {
		Name  string  `json:"name"`
		Kind  string  `json:"kind"`
		Start string  `json:"start"`
		Step  *string `json:"step"`
	}{v.Name, v.Kind, v.Start, v.Step})
}

// Document is the JSON form of an effect (§11.1). pre, last and post are nil when the step
// is absent; every field is always present in a JSON answer.
type Document struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Moments     []string      `json:"moments"`
	MinGroups   int           `json:"min_groups"`
	TickMs      int           `json:"tick_ms"`
	Loop        bool          `json:"loop"`
	Variables   []DocVariable `json:"variables"`
	Pre         []DocAction   `json:"pre"`
	Steps       [][]DocAction `json:"steps"`
	Last        []DocAction   `json:"last"`
	Post        []DocAction   `json:"post"`
}

// MarshalJSON never writes null for moments, variables or steps.
func (d Document) MarshalJSON() ([]byte, error) {
	type plain Document
	p := plain(d)
	if p.Moments == nil {
		p.Moments = []string{}
	}
	if p.Variables == nil {
		p.Variables = []DocVariable{}
	}
	if p.Steps == nil {
		p.Steps = [][]DocAction{}
	}
	return json.Marshal(p)
}

// UnmarshalJSON: a document that omits `loop` loops, as a file does.
func (d *Document) UnmarshalJSON(b []byte) error {
	type plain Document
	p := plain{Loop: true}
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	*d = Document(p)
	return nil
}

// TargetString writes a target as it is written in a file (g0, g3, g*, gA, gA+1, gA-2).
func TargetString(t Target) string {
	switch t.Kind {
	case TargetG0:
		return "g0"
	case TargetGn:
		return "g" + strconv.Itoa(t.N)
	case TargetGN:
		return "g*"
	case TargetVar:
		switch {
		case t.Offset > 0:
			return t.Var + "+" + strconv.Itoa(t.Offset)
		case t.Offset < 0:
			return t.Var + strconv.Itoa(t.Offset)
		}
		return t.Var
	}
	return ""
}

func docActions(s Step) []DocAction {
	out := make([]DocAction, 0, len(s))
	for _, a := range s {
		da := DocAction{Target: TargetString(a.Target)}
		if a.Off {
			da.Off = true
		} else {
			if a.Color.Name != "" {
				da.Color = a.Color.Name
			} else {
				da.Color = fmt.Sprintf("#%02X%02X%02X", a.Color.RGB[0], a.Color.RGB[1], a.Color.RGB[2])
			}
			p := a.Pct
			da.Intensity = &p
		}
		out = append(out, da)
	}
	return out
}

// DocumentOf is the document of a valid effect.
func DocumentOf(e Effect) Document {
	d := Document{ID: e.ID, Name: e.Name, Description: e.Description, MinGroups: e.MinGroups, TickMs: e.TickMs, Loop: e.Loop,
		Moments: append([]string{}, e.Moments...), Variables: []DocVariable{}, Steps: [][]DocAction{}}
	for _, v := range e.Vars {
		dv := DocVariable{Name: v.Name}
		if v.IsList {
			dv.Kind = "list"
			dv.Items = []string{}
			for _, t := range v.List {
				dv.Items = append(dv.Items, TargetString(t))
			}
		} else {
			dv.Kind = "progression"
			dv.Start = v.Start.Src
			if v.Step != nil {
				s := v.Step.Src
				dv.Step = &s
			}
		}
		d.Variables = append(d.Variables, dv)
	}
	if e.HasPre {
		d.Pre = docActions(e.Pre)
	}
	for _, s := range e.Steps {
		d.Steps = append(d.Steps, docActions(s))
	}
	if e.HasLast {
		d.Last = docActions(e.Last)
	}
	if e.HasPost {
		d.Post = docActions(e.Post)
	}
	return d
}

// ParseDocument validates a document with the very rules of the files: it is serialized, then
// parsed. The errors carry their `path` and line 0.
func ParseDocument(d Document) (Effect, []Error) {
	text, errs := Serialize(d)
	if len(errs) > 0 {
		return Effect{}, errs
	}
	e, perrs := Parse([]byte(text))
	if len(perrs) > 0 {
		return Effect{}, perrs // cannot happen: Serialize has validated the same text
	}
	return e, nil
}

// ---------------------------------------------------------------------------------------------
// Grid (§11.2): the document as the editor grid shows it, without neighbour targets.
// ---------------------------------------------------------------------------------------------

// Conversion is one neighbour replaced by a variable that follows (`gA-1` ⇒ `gB`).
type Conversion struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Grid is the `grid` object of §11.2 / §11.3.
type Grid struct {
	Editable  bool         `json:"editable"`
	Effect    *Document    `json:"effect"`
	Converted []Conversion `json:"converted"`
	Blocked   []string     `json:"blocked"`
}

// BuildGrid converts every neighbour target (gA+1, gA-2) of d into a variable that follows the
// origin (`gB: gA-1`, no step), inserted just before it (by increasing offset), so that the
// frames are the same for any N (§12.2). A conversion that would exceed MaxVars blocks the grid.
func BuildGrid(d Document) Grid {
	g := Grid{Converted: []Conversion{}, Blocked: []string{}}
	type nb struct {
		from, origin string
		offset       int
	}
	var found []nb
	seen := map[string]bool{}
	scan := func(acts []DocAction) {
		for _, a := range acts {
			m := reVarRef.FindStringSubmatch(a.Target)
			if m == nil || m[2] == "" || seen[a.Target] {
				continue
			}
			off, _ := strconv.Atoi(m[2])
			seen[a.Target] = true
			found = append(found, nb{from: a.Target, origin: m[1], offset: off})
		}
	}
	scan(d.Pre)
	for _, s := range d.Steps {
		scan(s)
	}
	scan(d.Last)
	scan(d.Post)
	cp := cloneDocument(d)
	if len(found) == 0 {
		g.Editable, g.Effect = true, &cp
		return g
	}
	if len(d.Variables)+len(found) > MaxVars {
		for _, f := range found {
			g.Blocked = append(g.Blocked, f.from)
		}
		return g
	}
	used := map[string]bool{}
	for _, v := range d.Variables {
		used[v.Name] = true
	}
	newName := func() string {
		for c := 'A'; c <= 'Z'; c++ {
			if n := "g" + string(c); !used[n] {
				used[n] = true
				return n
			}
		}
		return ""
	}
	to := map[string]string{}
	created := map[string][]nb{} // origin → neighbours
	for _, f := range found {
		n := newName()
		if n == "" { // cannot happen under MaxVars ≤ 26
			g.Blocked = append(g.Blocked, f.from)
			continue
		}
		to[f.from] = n
		created[f.origin] = append(created[f.origin], f)
		g.Converted = append(g.Converted, Conversion{From: f.from, To: n})
	}
	if len(g.Blocked) > 0 {
		g.Converted = []Conversion{}
		return g
	}
	var vars []DocVariable
	for _, v := range cp.Variables {
		if list := created[v.Name]; len(list) > 0 {
			sort.SliceStable(list, func(i, j int) bool { return list[i].offset < list[j].offset })
			for _, f := range list {
				vars = append(vars, DocVariable{Name: to[f.from], Kind: "progression", Start: f.from})
			}
		}
		vars = append(vars, v)
	}
	// A neighbour whose origin is not declared cannot occur in a valid document.
	cp.Variables = vars
	rewrite := func(acts []DocAction) {
		for i := range acts {
			if n, ok := to[acts[i].Target]; ok {
				acts[i].Target = n
			}
		}
	}
	rewrite(cp.Pre)
	for _, s := range cp.Steps {
		rewrite(s)
	}
	rewrite(cp.Last)
	rewrite(cp.Post)
	g.Editable, g.Effect = true, &cp
	return g
}

func cloneDocument(d Document) Document {
	c := d
	c.Moments = append([]string(nil), d.Moments...)
	c.Variables = make([]DocVariable, len(d.Variables))
	for i, v := range d.Variables {
		v.Items = append([]string(nil), v.Items...)
		if v.Step != nil {
			s := *v.Step
			v.Step = &s
		}
		c.Variables[i] = v
	}
	cp := func(a []DocAction) []DocAction {
		if a == nil {
			return nil
		}
		out := make([]DocAction, len(a))
		copy(out, a)
		return out
	}
	c.Pre, c.Last, c.Post = cp(d.Pre), cp(d.Last), cp(d.Post)
	c.Steps = make([][]DocAction, len(d.Steps))
	for i, s := range d.Steps {
		c.Steps[i] = cp(s)
	}
	return c
}

// ---------------------------------------------------------------------------------------------
// Text inspection: comments and action order (§11.2 `has_comments`, `custom_order`).
// ---------------------------------------------------------------------------------------------

// HasComments reports whether the text carries at least one `#` comment.
func HasComments(data []byte) bool {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return strings.Contains(string(data), "#") && hasCommentLine(string(data))
	}
	var walk func(n *yaml.Node) bool
	walk = func(n *yaml.Node) bool {
		if n.HeadComment != "" || n.LineComment != "" || n.FootComment != "" {
			return true
		}
		for _, c := range n.Content {
			if walk(c) {
				return true
			}
		}
		return false
	}
	return walk(&root)
}

// hasCommentLine is the fallback for an unparsable text: a `#` that starts a word outside quotes.
func hasCommentLine(s string) bool {
	for _, ln := range strings.Split(s, "\n") {
		inS, inD := false, false
		for i := 0; i < len(ln); i++ {
			switch c := ln[i]; {
			case c == '\'' && !inD:
				inS = !inS
			case c == '"' && !inS:
				inD = !inD
			case c == '#' && !inS && !inD && (i == 0 || ln[i-1] == ' ' || ln[i-1] == '\t'):
				return true
			}
		}
	}
	return false
}

// CustomOrder reports whether at least one step of the effect is not in the canonical order
// of §11.1 (g0, g1…g12, g*, then the variables in declaration order, a neighbour just before
// its variable by increasing offset).
func CustomOrder(e Effect) bool {
	idx := map[string]int{}
	for i, v := range e.Vars {
		idx[v.Name] = i
	}
	key := func(t Target) [3]int {
		switch t.Kind {
		case TargetG0:
			return [3]int{0, 0, 0}
		case TargetGn:
			return [3]int{1, t.N, 0}
		case TargetGN:
			return [3]int{2, 0, 0}
		}
		sub := 1 << 20 // the variable itself, after its neighbours
		if t.Offset != 0 {
			sub = t.Offset
		}
		return [3]int{3, idx[t.Var], sub}
	}
	inOrder := func(s Step) bool {
		for i := 1; i < len(s); i++ {
			a, b := key(s[i-1].Target), key(s[i].Target)
			for j := 0; j < 3; j++ {
				if a[j] < b[j] {
					break
				}
				if a[j] > b[j] {
					return false
				}
			}
		}
		return true
	}
	steps := append([]Step{}, e.Steps...)
	if e.HasPre {
		steps = append(steps, e.Pre)
	}
	if e.HasLast {
		steps = append(steps, e.Last)
	}
	if e.HasPost {
		steps = append(steps, e.Post)
	}
	for _, s := range steps {
		if !inOrder(s) {
			return true
		}
	}
	return false
}
