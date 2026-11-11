package effects

// SURFACE D'API ATTENDUE par les tests du paquet (v11.7.0 LOT-2, #223).
// Le contrat light-effects.md §5.1 fixe Parse / LoadDir / Render / Context / Frame
// mais pas leurs champs : tout appel dépendant de la forme est ISOLÉ ICI. Si
// dev-backend nomme autrement, il n'adapte que ce fichier ; les tests
// (render_test.go, validate_test.go, natives_test.go) ne changent pas.
//
// Forme supposée :
//
//	type Context struct {
//	    Groups    int           // N : groupes de salle non vides
//	    Remaining time.Duration // temps restant (moment chrono)
//	    Total     time.Duration // durée totale
//	    Scene, Theme, Team [3]int
//	    Origin    int           // groupe d'origine (1-based) pour origin: team
//	    Seed      int64
//	}
//	type GroupLook struct{ Color [3]int; Intensity, FadeMs int }   // Intensity 0-255
//	type Frame []GroupLook          // un élément par position ; target: all => 1 élément
//	type Error struct{ File string; Line int; Message string }
//	func Parse(data []byte) (Effect, []Error)
//	func LoadDir(embedded fs.FS, dir string) Catalogue
//	func (c Catalogue) Get(id string) (Entry, bool)   // Entry{Effect; Origin ("native"|"user"|"override"); File}
//	func (c Catalogue) Errors() []Error
//	func Render(e Effect, ctx Context, elapsed time.Duration) Frame
//	func (e Effect) Applicable(n int) (ok bool, reason string)
//	Effect : ID, Name string ; Moments []string ; MinGroups, DurationMs int ; Target string

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
	"time"
)

// Lk : forme neutre utilisée par les tests (indépendante de GroupLook).
type Lk struct {
	Color     [3]int
	Intensity int
	FadeMs    int
	Absent    bool // aucun état émis pour ce groupe (§4.3.1 point 3, `rest` absent)
}

// Ctx : contexte de rendu des tests.
type Ctx struct {
	N         int
	Remaining time.Duration
	Total     time.Duration
	Scene     [3]int
	Theme     [3]int
	Team      [3]int
	Origin    int
	Seed      int64
	Moment    string // vide : premier moment de l'effet
}

func (c Ctx) toContext(e Effect, at time.Duration) Context {
	m := Moment("")
	if len(e.Moments) > 0 {
		m = Moment(e.Moments[0])
	}
	if c.Moment != "" {
		m = Moment(c.Moment)
	}
	// WallMs : l'horloge murale des motifs « uniform » est le temps du test.
	return Context{Groups: c.N, Moment: m, Remaining: c.Remaining, Total: c.Total, Scene: c.Scene, Theme: c.Theme,
		Team: c.Team, Origin: c.Origin, Seed: c.Seed, WallMs: at.Milliseconds()}
}

// parseEffect décode un effet ; retourne les messages d'erreur (vide si valide).
func parseEffect(src string) (Effect, []Error) { return Parse([]byte(src)) }

func mustParse(t *testing.T, src string) Effect {
	t.Helper()
	e, errs := parseEffect(src)
	if len(errs) > 0 {
		t.Fatalf("effet de test invalide : %+v", errs)
	}
	return e
}

func render(e Effect, c Ctx, at time.Duration) []Lk {
	f := Render(e, c.toContext(e, at), at)
	out := make([]Lk, len(f))
	for i, g := range f {
		out[i] = Lk{Color: g.Color, Intensity: g.Intensity, FadeMs: g.FadeMs, Absent: isAbsent(g)}
	}
	return out
}

func applicable(e Effect, n int) (bool, string) { return e.Applicable(n) }

// catalogue décrit le contenu d'un chargement.
type catalogue struct {
	c Catalogue
}

func (k catalogue) get(id string) (origin, file string, ok bool) {
	e, ok := k.c.Get(id)
	return e.Origin, e.File, ok
}
func (k catalogue) errors() []Error { return k.c.Errors() }
func (k catalogue) effect(id string) (Effect, bool) {
	e, ok := k.c.Get(id)
	return e.Effect, ok
}

// loadFS charge `native` (effets natifs, fichiers à la racine) et le dossier utilisateur `dir`.
func loadFS(native fs.FS, dir string) catalogue { return catalogue{LoadDir(native, dir)} }

// nativeFS : natifs embarqués du dépôt (server-go/assets/effects).
func nativeFS(t *testing.T) fs.FS {
	t.Helper()
	sub, err := fs.Sub(os.DirFS(filepath.Join("..", "..", "assets")), "effects")
	if err != nil {
		t.Fatal(err)
	}
	return sub
}

// memNative : natifs de test en mémoire.
func memNative(files map[string]string) fs.FS {
	m := fstest.MapFS{}
	for n, s := range files {
		m[n] = &fstest.MapFile{Data: []byte(s)}
	}
	return m
}

// writeUser écrit un fichier dans le dossier utilisateur.
func writeUser(t *testing.T, dir, name, src string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

// isAbsent : « aucun état émis pour ce groupe » (contrat §4.3.1 point 3). La forme n'est pas fixée par le
// contrat : on accepte un booléen nommé Absent|Skip|None|Unset|Inherit sur GroupLook, sinon Intensity < 0.
// Adapter ici si dev-backend choisit une autre représentation.
func isAbsent(g GroupLook) bool {
	v := reflect.ValueOf(g)
	for _, name := range []string{"Absent", "Skip", "None", "Unset", "Inherit"} {
		if f := v.FieldByName(name); f.IsValid() && f.Kind() == reflect.Bool {
			return f.Bool()
		}
	}
	return g.Intensity < 0
}
