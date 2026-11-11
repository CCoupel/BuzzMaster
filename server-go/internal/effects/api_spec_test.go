package effects

// SURFACE D'API ATTENDUE par les tests du paquet (v11.7.0, révision 3 : effets pas à pas, #223).
// Le contrat light-effects.md §5.1 fixe `Parse([]byte)`, `LoadDir`, `Render(e, ctx, tick int) Frame` mais pas la forme de
// `Frame` ni de `Context` : tout appel dépendant de la forme est isolé ICI. Si dev-backend nomme autrement, il n'adapte
// que ce fichier ; les tests (chronogram_test.go, render_test.go, validate_test.go, natives_test.go, speed_test.go) ne
// changent pas.
//
// Forme supposée :
//
//	type Context struct {
//	    Groups    int           // N : groupes de salle non vides (0 possible)
//	    Moment    Moment
//	    Remaining time.Duration // chrono : temps restant
//	    Total     time.Duration
//	    Scene, Theme, Team [3]int
//	}
//	// Frame : état des groupes APRÈS le tick `tick` (le déroulement est une fonction pure du tick).
//	type Frame struct {
//	    Base   *GroupLook  // état de g0 ; nil tant que g0 n'a pas été écrit (scène ordinaire)
//	    Groups []GroupLook // N entrées (position i+1) ; Absent=true : le groupe n'a pas d'état propre, il suit Base
//	}
//	func Render(e Effect, ctx Context, tick int) Frame
//	func RenderPost(e Effect, ctx Context, lastTick int) (Frame, bool) // POST (§4.5 point 5), variables à leur dernière valeur ; false si pas de post
//	func (e Effect) Applicable(n int) (ok bool, reason string)         // salle
//	func (e Effect) HostApplicable() bool                              // groupe animateur : n'écrit que g0
//	func TickDuration(tickMs int, speed float64) time.Duration         // tickMs ÷ speed, jamais sous 250 ms
//	Effect : ID, Name string ; Moments []string ; MinGroups int (groupes_min) ; TickMs int (duree_tick_ms)

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"
)

// Lk : forme neutre des tests (indépendante de GroupLook).
type Lk struct {
	Color     [3]int
	Intensity int
	Absent    bool // aucun état : la zone garde la scène ordinaire
}

// Ctx : contexte de rendu des tests.
type Ctx struct {
	N         int
	Moment    string
	Remaining time.Duration
	Total     time.Duration
	Scene     [3]int
	Theme     [3]int
	Team      [3]int
}

func (c Ctx) toContext(e Effect) Context {
	m := Moment("")
	if len(e.Moments) > 0 {
		m = Moment(e.Moments[0])
	}
	if c.Moment != "" {
		m = Moment(c.Moment)
	}
	return Context{Groups: c.N, Moment: m, Remaining: c.Remaining, Total: c.Total, Scene: c.Scene, Theme: c.Theme, Team: c.Team}
}

// View : ce qu'un tick montre. Base nil = g0 jamais écrit ; Groups[i] = état effectif propre du groupe i+1 (Absent = suit Base).
type View struct {
	Base   *Lk
	Groups []Lk
}

func toLk(g GroupLook) Lk { return Lk{Color: g.Color, Intensity: g.Intensity, Absent: g.Absent} }

func toView(f Frame) View {
	v := View{Groups: make([]Lk, len(f.Groups))}
	if f.Base != nil {
		b := toLk(*f.Base)
		v.Base = &b
	}
	for i, g := range f.Groups {
		v.Groups[i] = toLk(g)
	}
	return v
}

func parseEffect(src string) (Effect, []Error) { return Parse([]byte(src)) }

func mustParse(t *testing.T, src string) Effect {
	t.Helper()
	e, errs := parseEffect(src)
	if len(errs) > 0 {
		t.Fatalf("effet de test invalide : %+v", errs)
	}
	return e
}

func renderView(e Effect, c Ctx, tick int) View { return toView(Render(e, c.toContext(e), tick)) }

func renderPostView(e Effect, c Ctx, lastTick int) (View, bool) {
	f, ok := RenderPost(e, c.toContext(e), lastTick)
	return toView(f), ok
}

func applicable(e Effect, n int) (bool, string) { return e.Applicable(n) }
func hostApplicable(e Effect) bool              { return e.HostApplicable() }
func tickDuration(tickMs int, speed float64) time.Duration {
	return TickDuration(tickMs, speed)
}

// effective : état visible d'un groupe (position 1-based) : son état propre, sinon la base ; Absent si ni l'un ni l'autre.
func (v View) effective(pos int) Lk {
	if pos < 1 || pos > len(v.Groups) {
		return Lk{Absent: true}
	}
	if g := v.Groups[pos-1]; !g.Absent {
		return g
	}
	if v.Base != nil {
		return *v.Base
	}
	return Lk{Absent: true}
}

// catalogue : lecture du catalogue.
type catalogue struct{ c Catalogue }

func (k catalogue) get(id string) (origin, file string, ok bool) {
	e, ok := k.c.Get(id)
	return e.Origin, e.File, ok
}
func (k catalogue) errors() []Error { return k.c.Errors() }
func (k catalogue) effect(id string) (Effect, bool) {
	e, ok := k.c.Get(id)
	return e.Effect, ok
}

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

func memNative(files map[string]string) fs.FS {
	m := fstest.MapFS{}
	for n, s := range files {
		m[n] = &fstest.MapFile{Data: []byte(s)}
	}
	return m
}

func writeUser(t *testing.T, dir, name, src string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}
