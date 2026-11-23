package main

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/server"
)

// contracts/vplayer-session.md §6 : WARN au chargement avec le NOMBRE de places sans empreinte, jamais d'ID.
func TestVPlayerSessionWarnDev_CountOnlyNeverTheID(t *testing.T) {
	app := newBroadcast127TestApp(t)
	server.SetGlobalLogger(app.logger)
	t.Cleanup(func() { server.SetGlobalLogger(nil) })
	app.engine.SetPhase(game.PhaseEnroll)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	id := setupVirtualPlayer(t, app, "Alice", "TeamA") // sans empreinte (entrée historique)
	app.warnSeatsWithoutFingerprint("démarrage", errors.New("fichier corrompu"))

	var line string
	for _, en := range app.logger.GetHistory() {
		if strings.Contains(en.Message, "VPLAYER_SESSIONS") {
			line = en.Message
		}
	}
	if line == "" {
		t.Fatal("WARN VPLAYER_SESSIONS attendu")
	}
	if !strings.Contains(line, "1 place(s)") || !strings.Contains(line, "absent, illisible ou corrompu") {
		t.Errorf("le WARN doit porter le nombre de places et la cause : %s", line)
	}
	if strings.Contains(line, id) || strings.Contains(line, "Alice") {
		t.Errorf("jamais d'ID ni de nom dans le WARN : %s", line)
	}
}

func TestVPlayerSessionWarnDev_SilentWhenNothingToReport(t *testing.T) {
	app := newBroadcast127TestApp(t)
	server.SetGlobalLogger(app.logger)
	t.Cleanup(func() { server.SetGlobalLogger(nil) })
	app.warnSeatsWithoutFingerprint("démarrage", nil) // aucune place, aucune erreur
	for _, en := range app.logger.GetHistory() {
		if strings.Contains(en.Message, "VPLAYER_SESSIONS") {
			t.Errorf("aucun WARN attendu : %s", en.Message)
		}
	}
}

// Revue C2 : fichier absent ET aucune place sans empreinte (installation neuve) : pas de WARN à chaque lancement.
// Un fichier illisible avertit même à 0 place ; une place sans empreinte avertit même fichier absent.
func TestVPlayerSessionWarnDev_AbsentFileAndNoSeat_Silent_Otherwise_Warns(t *testing.T) {
	count := func(app *App) int {
		n := 0
		for _, en := range app.logger.GetHistory() {
			if strings.Contains(en.Message, "VPLAYER_SESSIONS") {
				n++
			}
		}
		return n
	}
	absent := &fs.PathError{Op: "open", Path: "vplayer_sessions.json", Err: fs.ErrNotExist}

	app := newBroadcast127TestApp(t)
	server.SetGlobalLogger(app.logger)
	t.Cleanup(func() { server.SetGlobalLogger(nil) })
	app.warnSeatsWithoutFingerprint("démarrage", absent)
	if count(app) != 0 {
		t.Error("fichier absent et 0 place : aucun WARN attendu")
	}
	app.warnSeatsWithoutFingerprint("démarrage", errors.New("json: unexpected end"))
	if count(app) != 1 {
		t.Error("fichier illisible : WARN attendu même à 0 place")
	}
	app.engine.SetPhase(game.PhaseEnroll)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	setupVirtualPlayer(t, app, "Alice", "TeamA")
	app.warnSeatsWithoutFingerprint("démarrage", absent)
	if count(app) != 2 {
		t.Error("une place sans empreinte : WARN attendu même fichier absent")
	}
}
