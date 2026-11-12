package effects

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// serialize.go — Document → file text (contract light-effects.md §11.4): keys in the order of
// §4.1, one step per line written `{ target: [colour, intensity] }`, `description` in a `>`
// block, no comment, `\n` line ends, UTF-8. The text is then parsed by the same parser as the
// files: one set of rules, and a `path` for every error (the line written → the element).

var (
	rePlainWord = regexp.MustCompile(`^[\p{L}][\p{L}\p{N} _.,()'’-]*$`)
	rePlainKey  = regexp.MustCompile(`^[A-Za-z0-9*+-]+$`)
	rePlainExpr = regexp.MustCompile(`^[A-Za-z0-9(+-][A-Za-z0-9*+\-/(); ]*$`)
	yamlWords   = map[string]bool{"true": true, "false": true, "yes": true, "no": true, "null": true, "on": true, "off": true, "oui": true, "non": true}
)

// scalar writes a text value: plain when it is safe, else double-quoted.
func scalar(s string) string {
	if rePlainWord.MatchString(s) && s == strings.TrimSpace(s) && !yamlWords[strings.ToLower(s)] {
		return s
	}
	return strconv.Quote(s)
}

func keyText(s string) string {
	if rePlainKey.MatchString(s) && !strings.HasPrefix(s, "*") || s == "g*" {
		return s
	}
	return strconv.Quote(s)
}

func exprText(s string) string {
	if rePlainExpr.MatchString(s) && s == strings.TrimSpace(s) {
		return s
	}
	return strconv.Quote(s)
}

type textWriter struct {
	lines []string
	paths []string // path of each line (same index)
}

func (w *textWriter) add(path, line string) {
	w.lines = append(w.lines, line)
	w.paths = append(w.paths, path)
}

func actionText(a DocAction) (string, bool) {
	if a.Off {
		return keyText(a.Target) + ": [eteint]", true
	}
	if a.Color == "" || a.Intensity == nil {
		return "", false
	}
	col := a.Color
	if namedColors[col] {
		// plain
	} else {
		col = strconv.Quote(col)
	}
	return keyText(a.Target) + ": [" + col + ", " + strconv.Itoa(*a.Intensity) + "]", true
}

// stepText writes `{ a: [..], b: [..] }`; ok=false when an action has neither colour nor off.
func stepText(acts []DocAction) (string, bool) {
	parts := make([]string, 0, len(acts))
	for _, a := range acts {
		t, ok := actionText(a)
		if !ok {
			return "", false
		}
		parts = append(parts, t)
	}
	if len(parts) == 0 {
		return "{}", true
	}
	return "{ " + strings.Join(parts, ", ") + " }", true
}

func descriptionLines(d string) []string {
	body := strings.TrimSuffix(d, "\n")
	simple := body != "" && body == strings.TrimSpace(body) && !strings.ContainsAny(body, "\n\r\t") && strings.IndexFunc(body, unicode.IsControl) < 0
	switch {
	case simple && d == body:
		return []string{"description: >-", "  " + body}
	case simple && d == body+"\n":
		return []string{"description: >", "  " + body}
	}
	return []string{"description: " + strconv.Quote(d)}
}

// Serialize writes the file text of a document. Any problem — the document's own, or the
// validator's — comes back as errors with a `path` and line 0, and an empty text.
func Serialize(d Document) (string, []Error) {
	var errs []Error
	bad := func(path, format string, args ...any) {
		errs = append(errs, Error{Path: path, Message: fmt.Sprintf(format, args...)})
	}
	if IsReserved(d.ID) {
		bad("id", "« %s » est un mot réservé : changez l'identifiant", d.ID)
	} else if IsWindowsDevice(d.ID) {
		bad("id", "« %s » est un nom réservé par Windows : changez l'identifiant", d.ID)
	}
	w := &textWriter{}
	w.add("id", "id: "+scalar(d.ID))
	w.add("name", "nom: "+scalar(d.Name))
	if d.Description != "" {
		for _, l := range descriptionLines(d.Description) {
			w.add("description", l)
		}
	}
	w.add("", "version: "+strconv.Itoa(SchemaVersion))
	ms := make([]string, len(d.Moments))
	for i, m := range d.Moments {
		ms[i] = scalar(m)
	}
	w.add("moments", "moments: ["+strings.Join(ms, ", ")+"]")
	if d.MinGroups != 0 {
		w.add("min_groups", "groupes_min: "+strconv.Itoa(d.MinGroups))
	}
	w.add("tick_ms", "duree_tick_ms: "+strconv.Itoa(d.TickMs))
	if !d.Loop {
		w.add("loop", "boucle: non")
	}
	if len(d.Variables) > 0 {
		w.add("", "")
		w.add("variables", "variables:")
		for i, v := range d.Variables {
			path := "variables[" + strconv.Itoa(i) + "]"
			switch v.Kind {
			case "progression":
				val := exprText(v.Start)
				if v.Step != nil {
					val = exprText(v.Start + "; " + *v.Step)
				}
				w.add(path, "  "+keyText(v.Name)+": "+val)
			case "list":
				items := make([]string, len(v.Items))
				for j, it := range v.Items {
					items[j] = scalar(it)
				}
				w.add(path, "  "+keyText(v.Name)+": ["+strings.Join(items, ", ")+"]")
			default:
				bad(path, "le type de la variable « %s » doit être « progression » ou « list »", v.Name)
			}
		}
	}
	flow := func(key, path string, acts []DocAction) {
		t, ok := stepText(acts)
		if !ok {
			bad(path, "une action doit avoir une couleur et une intensité, ou être éteinte")
			return
		}
		w.add("", "")
		w.add(path, key+":")
		w.add(path, "  "+t)
	}
	if d.Pre != nil {
		flow("pre", "pre", d.Pre)
	}
	w.add("", "")
	w.add("steps", "pas:")
	if len(d.Steps) == 0 {
		bad("steps", "« pas » compte de 1 à %d pas", MaxSteps)
	}
	for i, s := range d.Steps {
		path := "steps[" + strconv.Itoa(i) + "]"
		t, ok := stepText(s)
		if !ok {
			bad(path, "une action doit avoir une couleur et une intensité, ou être éteinte")
			continue
		}
		w.add(path, "  - "+t)
	}
	if d.Last != nil {
		flow("dernier", "last", d.Last)
	}
	if d.Post != nil {
		flow("post", "post", d.Post)
	}
	if len(errs) > 0 {
		return "", errs
	}
	text := strings.Join(w.lines, "\n") + "\n"
	if _, perrs := Parse([]byte(text)); len(perrs) > 0 {
		for _, pe := range perrs {
			p := ""
			if pe.Line >= 1 && pe.Line <= len(w.paths) {
				p = w.paths[pe.Line-1]
			}
			if pe.part != "" && strings.HasPrefix(p, "variables[") {
				p += "." + pe.part
			}
			errs = append(errs, Error{Path: p, Message: pe.Message})
		}
		return "", errs
	}
	return text, nil
}
