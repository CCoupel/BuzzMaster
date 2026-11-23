package led

// #194 (v13.0.0, LOT-2.3) — LED des buzzers pendant une question PICTO (contracts/picto.md §15) : dès que l'équipe qui
// dessine est choisie, et pendant la manche, ses buzzers sont SOLID à la couleur d'équipe et ceux des autres équipes
// DIM ; pause : atténués ; révélation, arrêt : règles existantes (couleur d'équipe pleine) ; appui ignoré. Les grilles
// MEMORY / RAFALE / ROUE ne changent pas (couvertes par leurs tests). Modèle : TestLEDMemory_Solo_ActiveTeam.

import (
	"testing"

	"buzzcontrol/internal/game"
)

func newPictoLEDApp194(t *testing.T) *App {
	t.Helper()
	app := newTestApp(t)
	q := &game.Question{
		ID: "pq1", Type: game.QuestionTypePicto, Category: game.CategoryHistory, Time: "90",
		TypedContent: game.TypedContent{PictoWords: []string{"chat", "chien"}},
	}
	app.engine.Ready(q.ID, q) // PREPARE
	app.engine.SetBumpers(map[string]*game.Bumper{
		"MAC:A1": {Team: "TeamA"},
		"MAC:A2": {Team: "TeamA"},
		"MAC:B1": {Team: "TeamB"},
		"MAC:C1": {Team: "TeamC"},
	})
	return app
}

func assertLED194(t *testing.T, app *App, mac, effect string, intensity int, why string) {
	t.Helper()
	s := app.bumperLEDState[mac]
	if s.Effect != effect || (intensity >= 0 && s.Intensity != intensity) {
		t.Errorf("%s %s : attendu %s %d, got %s %d", why, mac, effect, intensity, s.Effect, s.Intensity)
	}
}

func TestLEDPicto194_TeamChosen_ItsBuzzersSolid_OthersDim_FromPrepare(t *testing.T) {
	app := newPictoLEDApp194(t)
	if err := app.engine.PictoSetTeam("TeamA"); err != nil {
		t.Fatalf("PictoSetTeam : %v", err)
	}
	app.ctl.PictoTeams() // câblé à Engine.OnPictoChange : à chaque changement d'équipe choisie

	for _, mac := range []string{"MAC:A1", "MAC:A2"} {
		assertLED194(t, app, mac, "SOLID", 255, "équipe qui dessine (PREPARE)")
	}
	for _, mac := range []string{"MAC:B1", "MAC:C1"} {
		assertLED194(t, app, mac, "DIM", -1, "autre équipe (PREPARE)")
	}
	// La couleur reste celle de l'équipe (même conversion que le reste du jeu) : jamais éteinte, jamais blanche.
	if got := app.bumperLEDState["MAC:A1"].Color; got != app.teamNameToRGB("TeamA") {
		t.Errorf("la couleur d'équipe doit être conservée : %v", got)
	}
}

func TestLEDPicto194_Started_SameGrid_ViaAllBuzzers(t *testing.T) {
	app := newPictoLEDApp194(t)
	_ = app.engine.PictoSetTeam("TeamA")
	app.engine.SetPhase(game.PhaseStarted)
	app.sendLEDSetAllBuzzers()
	assertLED194(t, app, "MAC:A1", "SOLID", 255, "STARTED équipe qui dessine")
	assertLED194(t, app, "MAC:B1", "DIM", -1, "STARTED autre équipe")
	assertLED194(t, app, "MAC:C1", "DIM", -1, "STARTED autre équipe")
}

func TestLEDPicto194_Paused_EveryoneDimmed_ThenBackOnContinue(t *testing.T) {
	app := newPictoLEDApp194(t)
	_ = app.engine.PictoSetTeam("TeamA")
	app.engine.SetPhase(game.PhasePaused)
	app.sendLEDSetAllBuzzers()
	for _, mac := range []string{"MAC:A1", "MAC:B1", "MAC:C1"} {
		assertLED194(t, app, mac, "DIM", -1, "PAUSED")
	}
	app.engine.SetPhase(game.PhaseStarted)
	app.sendLEDSetAllBuzzers()
	assertLED194(t, app, "MAC:A1", "SOLID", 255, "reprise : l'équipe qui dessine retrouve son éclairage")
	assertLED194(t, app, "MAC:B1", "DIM", -1, "reprise : les autres restent atténuées")
}

func TestLEDPicto194_NoTeamYet_UsualTeamColors(t *testing.T) {
	app := newPictoLEDApp194(t) // aucune équipe choisie : pas de mise en avant
	app.sendLEDSetAllBuzzers()
	for _, mac := range []string{"MAC:A1", "MAC:B1", "MAC:C1"} {
		assertLED194(t, app, mac, "SOLID", 255, "aucune équipe choisie")
	}
}

func TestLEDPicto194_StoppedAndRevealed_UsualRules(t *testing.T) {
	for _, phase := range []game.GamePhase{game.PhaseStopped, game.PhaseRevealed} {
		t.Run(string(phase), func(t *testing.T) {
			app := newPictoLEDApp194(t)
			_ = app.engine.PictoSetTeam("TeamA")
			app.engine.SetPhase(phase)
			app.sendLEDSetAllBuzzers()
			for _, mac := range []string{"MAC:A1", "MAC:B1", "MAC:C1"} {
				if s := app.bumperLEDState[mac]; s.Effect == "OFF" || s.Intensity == 0 {
					t.Errorf("%s %s : les règles existantes de fin de manche gardent la couleur d'équipe allumée, got %s %d", phase, mac, s.Effect, s.Intensity)
				}
			}
		})
	}
}

// Les VJoueurs n'ont pas de LED : seules les places à adresse MAC (buzzers physiques) sont pilotées.
func TestLEDPicto194_VPlayersAreNotDriven(t *testing.T) {
	app := newPictoLEDApp194(t)
	app.engine.SetBumpers(map[string]*game.Bumper{
		"MAC:A1":          {Team: "TeamA"},
		"vjoueur_Alice_1": {Team: "TeamA", IsVirtual: true, IsVPlayer: true},
	})
	_ = app.engine.PictoSetTeam("TeamA")
	app.ctl.PictoTeams()
	if _, ok := app.bumperLEDState["vjoueur_Alice_1"]; ok {
		t.Errorf("un VJoueur ne reçoit pas de LED_SET")
	}
	assertLED194(t, app, "MAC:A1", "SOLID", 255, "buzzer physique de l'équipe")
}

// Non-régression : une question d'un autre type n'est jamais touchée par la grille PICTO.
func TestLEDPicto194_NR_OtherTypesUnaffected(t *testing.T) {
	app := newTestApp(t)
	app.engine.Ready("s1", &game.Question{ID: "s1", Type: game.QuestionTypeSpeedy, Time: "30"})
	app.engine.SetBumpers(map[string]*game.Bumper{"MAC:A1": {Team: "TeamA"}, "MAC:B1": {Team: "TeamB"}})
	app.engine.SetPhase(game.PhaseStarted)
	app.ctl.PictoTeams() // sans effet hors PICTO
	if _, ok := app.bumperLEDState["MAC:B1"]; ok && app.bumperLEDState["MAC:B1"].Effect == "DIM" {
		t.Errorf("PictoTeams ne doit rien envoyer pour une question non PICTO")
	}
}
