package effects

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v4"
)

var (
	reUnknownField = regexp.MustCompile(`field (\S+) not found`)
	reCannot       = regexp.MustCompile(`cannot unmarshal (\S+)(?: ` + "`" + `[^` + "`" + `]*` + "`" + `)? into (\S+)`)
	reLine         = regexp.MustCompile(`line (\d+)`)
)

// Parse decodes and validates one effect (contract §5.1); errors carry no file name. See
// ParseFile.
func Parse(data []byte) (Effect, []Error) { return ParseFile("", data) }

// rawEffect is the strict top-level decoding: unknown keys are refused; the step tables
// stay yaml nodes so that their order and their lines are kept.
type rawEffect struct {
	ID          string    `yaml:"id"`
	Nom         string    `yaml:"nom"`
	Description string    `yaml:"description"`
	Version     int       `yaml:"version"`
	Moments     []string  `yaml:"moments"`
	GroupesMin  int       `yaml:"groupes_min"`
	DureeTickMs int       `yaml:"duree_tick_ms"`
	Boucle      yaml.Node `yaml:"boucle"`
	Variables   yaml.Node `yaml:"variables"`
	Pre         yaml.Node `yaml:"pre"`
	Pas         yaml.Node `yaml:"pas"`
	Dernier     yaml.Node `yaml:"dernier"`
	Post        yaml.Node `yaml:"post"`
}

// ParseFile decodes and validates one effect file. file only labels the errors. On any
// error the returned Effect must not be used.
func ParseFile(file string, data []byte) (Effect, []Error) {
	if len(data) > MaxFileBytes {
		return Effect{}, []Error{{File: file, Message: fmt.Sprintf("fichier trop gros (%d Ko au plus)", MaxFileBytes>>10)}}
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return Effect{}, []Error{syntaxError(file, err)}
	}
	if root.Kind == 0 || len(root.Content) == 0 {
		return Effect{}, []Error{{File: file, Message: "fichier vide"}}
	}
	top := root.Content[0]
	if top.Kind != yaml.MappingNode {
		return Effect{}, []Error{{File: file, Line: top.Line, Message: "le fichier doit être une suite de clés (id, nom, version, pas…)"}}
	}
	present := map[string]int{}
	for i := 0; i+1 < len(top.Content); i += 2 {
		present[top.Content[i].Value] = top.Content[i].Line
		if top.Content[i].Value == "version" && top.Content[i+1].Value == "1" {
			return Effect{}, []Error{{File: file, Line: top.Content[i].Line, Message: "format d'effet de la version précédente : réécrire en pas à pas"}}
		}
	}
	var raw rawEffect
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil && !errors.Is(err, io.EOF) {
		return Effect{}, decodeErrors(file, err)
	}
	v := &validator{file: file}
	e := v.build(&raw, present)
	if len(v.errs) > 0 {
		return Effect{}, v.errs
	}
	return e, nil
}

func syntaxError(file string, err error) Error {
	line := 0
	if m := reLine.FindStringSubmatch(err.Error()); m != nil {
		line, _ = strconv.Atoi(m[1])
	}
	msg := strings.TrimPrefix(err.Error(), "yaml: ")
	return Error{File: file, Line: line, Message: "YAML illisible : " + msg}
}

func decodeErrors(file string, err error) []Error {
	var te *yaml.TypeError
	if !errors.As(err, &te) {
		return []Error{syntaxError(file, err)}
	}
	out := make([]Error, 0, len(te.Errors))
	for _, ue := range te.Errors {
		msg := ue.Err.Error()
		switch {
		case reUnknownField.MatchString(msg):
			out = append(out, Error{File: file, Line: ue.Line, Message: fmt.Sprintf("clé inconnue « %s »", reUnknownField.FindStringSubmatch(msg)[1])})
		case reCannot.MatchString(msg):
			out = append(out, Error{File: file, Line: ue.Line, Message: "valeur d'un type inattendu (" + expected(reCannot.FindStringSubmatch(msg)[2]) + ")"})
		default:
			out = append(out, Error{File: file, Line: ue.Line, Message: "valeur invalide : " + msg})
		}
	}
	return out
}

func expected(goType string) string {
	switch {
	case strings.HasPrefix(goType, "[]"):
		return "une liste attendue"
	case strings.HasPrefix(goType, "map"), strings.HasPrefix(goType, "effects."):
		return "un bloc de clés attendu"
	case goType == "string":
		return "un texte attendu"
	case strings.Contains(goType, "int"), strings.Contains(goType, "float"):
		return "un nombre attendu"
	case goType == "bool":
		return "vrai ou faux attendu"
	}
	return goType + " attendu"
}

// indexLines maps a dotted key path ("pattern.cells", "looks.tete",
// "stages.0.pattern.step_ms") to the line of its key in the file.
func indexLines(root *yaml.Node) map[string]int {
	out := map[string]int{}
	var walk func(n *yaml.Node, prefix string)
	walk = func(n *yaml.Node, prefix string) {
		switch n.Kind {
		case yaml.DocumentNode:
			for _, c := range n.Content {
				walk(c, prefix)
			}
		case yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				k := n.Content[i]
				p := k.Value
				if prefix != "" {
					p = prefix + "." + k.Value
				}
				out[p] = k.Line
				walk(n.Content[i+1], p)
			}
		case yaml.SequenceNode:
			for i, c := range n.Content {
				p := strconv.Itoa(i)
				if prefix != "" {
					p = prefix + "." + p
				}
				out[p] = c.Line
				walk(c, p)
			}
		}
	}
	walk(root, "")
	return out
}

// lineOf returns the line of the deepest known prefix of path, 0 if none.
func lineOf(lines map[string]int, path string) int {
	for {
		if l, ok := lines[path]; ok {
			return l
		}
		i := strings.LastIndex(path, ".")
		if i < 0 {
			return 0
		}
		path = path[:i]
	}
}
