package effects

// SURFACE D'API ATTENDUE par les tests de l'aperçu (v11.7.0, LOT-3, #299 — contrat light-effects.md §7.3).
// Le contrat fixe la forme JSON de la réponse mais pas la signature Go : tout appel dépendant de la forme est isolé ICI.
// Si dev-backend nomme autrement, il n'adapte que ce fichier ; preview_test.go ne change pas.
//
// Forme supposée (preview.go, contrat e9d86cd7) :
//
//	type PreviewFrame struct { AtMs, Tick int; Step string; Groups []<état> }  // <état> porte Color [3]int et Intensity int (plus de FadeMs)
//	type PreviewResult struct { ID, Moment string; Groups, TickMs, DurationMs, ChronoS int; Loop bool; Frames []PreviewFrame }
//	func Preview(e Effect, groups int, moment Moment) PreviewResult
//
// Le type des états de groupe n'est pas imposé : ils sont lus par réflexion sur les champs Color et Intensity.
// Contexte simulé (§7.3) : thème #4F46E5, équipe #E53935, scène blanche ; pour `chrono`, chronomètre de `chrono_s` = 20 s
// (reste = chrono_s − at_ms/1000) ; tick ×1 ; une trame PAR TICK (même identique à la précédente) plus une trame quand une
// couleur nommée change entre deux ticks (même tick, même step) ; `step` = "pre" | "pas:<n>" | "dernier" ; durée 20 s
// (chrono, calm) ou 5 s (score) ; 400 trames au plus.

import (
	"reflect"
	"time"
)

const (
	previewTotalMs   = 20000
	previewMaxFrames = 400
)

var (
	previewTheme = [3]int{0x4F, 0x46, 0xE5}
	previewTeam  = [3]int{0xE5, 0x39, 0x35}
)

type pvFrame struct {
	At, Tick int
	Step     string
	Groups   []Lk
}

type pvResult struct {
	ID, Moment                          string
	Groups, TickMs, DurationMs, ChronoS int
	Loop                                bool
	Frames                              []pvFrame
}

// lookOf lit Color et Intensity d'un état de groupe d'aperçu, quel que soit son type.
func lookOf(g interface{}) Lk {
	v := reflect.ValueOf(g)
	var l Lk
	if c := v.FieldByName("Color"); c.IsValid() {
		l.Color = c.Interface().([3]int)
	}
	if i := v.FieldByName("Intensity"); i.IsValid() {
		l.Intensity = int(i.Int())
	}
	return l
}

func preview(e Effect, groups int, moment string) pvResult {
	r := Preview(e, groups, Moment(moment))
	out := pvResult{ID: r.ID, Moment: string(r.Moment), Groups: r.Groups, TickMs: r.TickMs, DurationMs: r.DurationMs, ChronoS: r.ChronoS, Loop: r.Loop}
	for _, f := range r.Frames {
		pf := pvFrame{At: f.AtMs, Tick: f.Tick, Step: f.Step}
		for _, g := range f.Groups {
			pf.Groups = append(pf.Groups, lookOf(g))
		}
		out.Frames = append(out.Frames, pf)
	}
	return out
}

// previewCtx : le contexte simulé du contrat pour l'instant at (ms) d'un aperçu à n groupes.
func previewCtx(n int, moment string, atMs int) Ctx {
	c := Ctx{N: n, Moment: moment, Theme: previewTheme, Team: previewTeam, Scene: [3]int{255, 255, 255}}
	if moment == "chrono" {
		c.Total = previewTotalMs * time.Millisecond
		c.Remaining = time.Duration(previewTotalMs-atMs) * time.Millisecond
	}
	return c
}
