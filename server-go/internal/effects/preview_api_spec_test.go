package effects

// SURFACE D'API ATTENDUE par les tests de l'aperçu (v11.7.0, LOT-3, #299 — contrat light-effects.md §7.3).
// Le contrat fixe la forme JSON de la réponse mais pas la signature Go : tout appel dépendant de la forme est isolé ICI.
// Si dev-backend nomme autrement, il n'adapte que ce fichier ; preview_test.go ne change pas.
//
// Forme supposée (preview.go) :
//
//	type PreviewFrame struct { AtMs int; Groups []GroupLook }   // `groups` : état EFFECTIF de chaque groupe 1..N (propre, sinon g0)
//	type PreviewResult struct { ID string; Groups, DurationMs int; Loop bool; Frames []PreviewFrame }
//	func Preview(e Effect, groups int, moment Moment) PreviewResult
//
// Contexte simulé (§7.3) : chronomètre de 20 s pour `chrono` (reste = 20 s − at_ms), thème #4F46E5, équipe #E53935 ;
// tick ×1 (`duree_tick_ms`) ; une trame n'est listée que si elle diffère de la précédente ; 400 trames au plus.

import "time"

const (
	previewTotalMs   = 20000
	previewMaxFrames = 400
)

var (
	previewTheme = [3]int{0x4F, 0x46, 0xE5}
	previewTeam  = [3]int{0xE5, 0x39, 0x35}
)

type pvFrame struct {
	At     int
	Groups []Lk
}

type pvResult struct {
	Groups, DurationMs int
	Loop               bool
	Frames             []pvFrame
}

func preview(e Effect, groups int, moment string) pvResult {
	r := Preview(e, groups, Moment(moment))
	out := pvResult{Groups: r.Groups, DurationMs: r.DurationMs, Loop: r.Loop}
	for _, f := range r.Frames {
		pf := pvFrame{At: f.AtMs}
		for _, g := range f.Groups {
			pf.Groups = append(pf.Groups, toLk(g))
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
