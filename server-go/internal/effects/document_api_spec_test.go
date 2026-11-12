package effects

// SURFACE D'API ATTENDUE par les tests du document d'effet (v11.8.0, LOT-2, B2.1 — contrat light-effects.md §11.1, §11.2).
// Le contrat fixe la forme JSON du document, pas les signatures Go : tout appel dépendant de la forme est isolé ICI. Si dev-backend
// nomme autrement, il n'adapte que ce fichier ; document_spec_test.go ne change pas.
//
// Forme supposée (document.go, serialize.go) :
//
//	type Document struct{ … }                          // tags JSON = noms de l'API (§11.1)
//	func DocumentOf(e Effect) Document
//	func Serialize(d Document) (string, []Error)       // texte régénéré (§11.4) ; erreurs de validation (path dans Error.Path si présent)
//	func BuildGrid(d Document) Grid                    // §11.2 `grid`
//	type Grid struct { Editable bool; Effect *Document; Converted []Conversion; Blocked []string }
//	type Conversion struct { From, To string }

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type docJSON = map[string]any

func docOfEffect(e Effect) docJSON {
	b, _ := json.Marshal(DocumentOf(e))
	var m docJSON
	_ = json.Unmarshal(b, &m)
	return m
}

func docToDocument(m docJSON) Document {
	b, _ := json.Marshal(m)
	var d Document
	_ = json.Unmarshal(b, &d)
	return d
}

func serializeDoc(m docJSON) (string, []Error) { return Serialize(docToDocument(m)) }

type gridView struct {
	Editable  bool
	Effect    docJSON // nil si non éditable
	Converted [][2]string
	Blocked   []string
}

func gridOfDoc(m docJSON) gridView {
	g := BuildGrid(docToDocument(m))
	v := gridView{Editable: g.Editable, Blocked: g.Blocked}
	if g.Effect != nil {
		b, _ := json.Marshal(g.Effect)
		_ = json.Unmarshal(b, &v.Effect)
	}
	for _, c := range g.Converted {
		v.Converted = append(v.Converted, [2]string{c.From, c.To})
	}
	return v
}

// readNative : texte embarqué d'un natif du dépôt (server-go/assets/effects).
func readNative(t interface {
	Helper()
	Fatal(...any)
}, id string) ([]byte, error) {
	return os.ReadFile(filepath.Join("..", "..", "assets", "effects", id+".yaml"))
}
