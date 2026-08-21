package main

import (
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"encoding/json"
	"testing"
)

// ---------------------------------------------------------------------------
// #128 — fuite de ARDOISE_ANSWERS (et QUIZ_OBJECTIVES / champs firmware par
// buzzer) vers le VJoueur, bout-en-bout à travers les 7 sites de diffusion
// réels identifiés par le plan (_work/reports/plan-128-20260820-170433.md,
// tableau "Défaut A") : broadcastStart, broadcastStop, broadcastPause,
// broadcastPauseAll, broadcastContinue, broadcastTimerUpdate,
// broadcastCountdownUpdate.
//
// Complète internal/protocol/ardoise_leak_128_test.go (niveau sérialiseur,
// isolé du transport) par le chemin RÉEL : vraies connexions WebSocket,
// vrais appels aux méthodes App, exactement ce qu'un client reçoit sur le
// fil — même discipline que main_broadcast_127_test.go/
// main_broadcast_129_test.go, dont ce fichier réutilise le harnais
// (newBroadcast127TestApp, setupVirtualPlayer) et
// inbound_allowlist_anim_test.go (startAnimAllowlistTestServer, seul
// harnais existant à router les 4 types web — admin/tv/vplayer/anim — sur
// un même serveur de test).
// ---------------------------------------------------------------------------

// gameNodeOfRaw extracts MSG.GAME from a raw serialized WS frame — the
// cmd/server-local equivalent of internal/protocol's gameNodeOf (unexported
// there, different package).
func gameNodeOfRaw(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var envelope struct {
		Msg struct {
			Game    map[string]interface{} `json:"GAME"`
			Bumpers map[string]interface{} `json:"bumpers"`
		} `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("gameNodeOfRaw: failed to unmarshal: %v (raw: %s)", err, raw)
	}
	return envelope.Msg.Game
}

func bumpersNodeOfRaw(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var envelope struct {
		Msg struct {
			Bumpers map[string]interface{} `json:"bumpers"`
		} `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("bumpersNodeOfRaw: failed to unmarshal: %v (raw: %s)", err, raw)
	}
	return envelope.Msg.Bumpers
}

// setupArdoiseLeak128TestApp wires an app with an ARDOISE question STARTED,
// two teams with submitted answers, a QUIZ_OBJECTIVES value, and a physical
// bumper carrying an admin-only firmware field — everything #128 concerns,
// in one realistic game state.
func setupArdoiseLeak128TestApp(t *testing.T) *App {
	t.Helper()
	app := newBroadcast127TestApp(t)
	app.engine.SetTeams(map[string]*game.Team{
		"TeamA": {Name: "TeamA", Color: []int{255, 0, 0}},
		"TeamB": {Name: "TeamB", Color: []int{0, 0, 255}},
	})
	app.engine.SetBumpers(map[string]*game.Bumper{
		"buzzer-1": {Name: "Buzzer1", Team: "TeamA", Connected: true, FirmwareVersion: "3.1.1"},
	})

	app.engine.SetPhase(game.PhaseStopped)
	app.engine.Ready("", &game.Question{ID: "q-ardoise-128", Type: game.QuestionTypeArdoise})
	app.engine.SetPhase(game.PhaseStarted)

	if !app.engine.SetArdoiseAnswer("TeamA", "Paris") {
		t.Fatal("setup: SetArdoiseAnswer(TeamA) failed")
	}
	if !app.engine.SetArdoiseAnswer("TeamB", "Berlin") {
		t.Fatal("setup: SetArdoiseAnswer(TeamB) failed")
	}

	app.engine.SetQuizMeta("Quiz de test", "Thème", "Notes", nil, nil, "Français", "Objectif confidentiel #128")

	return app
}

// TestArdoiseLeak128_SevenBroadcastSites is THE central #128 reproduction:
// for each of the 7 real diffusion sites, a VJoueur must NEVER receive
// ARDOISE_ANSWERS, QUIZ_OBJECTIVES, or the buzzer's admin-only firmware
// field — while TV and /anim continue to receive ARDOISE_ANSWERS normally.
func TestArdoiseLeak128_SevenBroadcastSites(t *testing.T) {
	app := setupArdoiseLeak128TestApp(t)

	baseURL := startAnimAllowlistTestServer(t, app)
	adminConn := dialWS(t, baseURL, "/ws/admin")
	learnClientID(t, app, adminConn)
	tvConn := dialWS(t, baseURL, "/ws/tv")
	learnClientID(t, app, tvConn)
	vpConn := dialWS(t, baseURL, "/ws/player")
	learnClientID(t, app, vpConn)
	animConn := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, animConn)

	type site struct {
		label  string
		action string
		call   func()
	}
	sites := []site{
		{"broadcastStart", protocol.ActionStart, func() { app.broadcastStart() }},
		{"broadcastStop (l'énoncé original de #128)", protocol.ActionStop, func() { app.broadcastStop() }},
		{"broadcastPause", protocol.ActionPause, func() { app.broadcastPause("buzzer-1") }},
		{"broadcastPauseAll", protocol.ActionPause, func() { app.broadcastPauseAll() }},
		{"broadcastContinue", protocol.ActionContinue, func() { app.broadcastContinue() }},
		{"broadcastTimerUpdate (fuite continue, 1/s — le plus grave)", protocol.ActionUpdateTimer, func() { app.broadcastTimerUpdate(10) }},
		{"broadcastCountdownUpdate", protocol.ActionUpdateTimer, func() { app.broadcastCountdownUpdate(3) }},
	}

	for _, s := range sites {
		t.Run(s.label, func(t *testing.T) {
			s.call()

			_, tvRaw := readActionMatching(t, tvConn, s.action)
			_, vpRaw := readActionMatching(t, vpConn, s.action)
			_, animRaw := readActionMatching(t, animConn, s.action)
			_, adminRaw := readActionMatching(t, adminConn, s.action)

			tvGame := gameNodeOfRaw(t, tvRaw)
			vpGame := gameNodeOfRaw(t, vpRaw)
			animGame := gameNodeOfRaw(t, animRaw)
			adminGame := gameNodeOfRaw(t, adminRaw)

			// --- ARDOISE_ANSWERS ---
			if _, present := vpGame["ARDOISE_ANSWERS"]; present {
				t.Errorf("%s: VJoueur received ARDOISE_ANSWERS — the #128 leak itself: %v", s.label, vpGame)
			}
			if _, present := tvGame["ARDOISE_ANSWERS"]; !present {
				t.Errorf("%s: TV must still receive ARDOISE_ANSWERS (needed at REVEAL) — non-regression", s.label)
			}
			if _, present := animGame["ARDOISE_ANSWERS"]; !present {
				t.Errorf("%s: /anim must still receive ARDOISE_ANSWERS (#158, live list) — non-regression", s.label)
			}
			if _, present := adminGame["ARDOISE_ANSWERS"]; !present {
				t.Errorf("%s: admin must always receive ARDOISE_ANSWERS", s.label)
			}

			// --- QUIZ_OBJECTIVES (defect A, beyond ARDOISE) ---
			for name, g := range map[string]map[string]interface{}{"TV": tvGame, "VJoueur": vpGame, "anim": animGame} {
				if _, present := g["QUIZ_OBJECTIVES"]; present {
					t.Errorf("%s: %s received QUIZ_OBJECTIVES — confidentiality rule broken: %v", s.label, name, g)
				}
			}

			// --- Buzzer firmware fields (defect A, per-bumper) ---
			for name, raw := range map[string]string{"TV": tvRaw, "VJoueur": vpRaw, "anim": animRaw} {
				bumpers := bumpersNodeOfRaw(t, raw)
				b1, ok := bumpers["buzzer-1"].(map[string]interface{})
				if !ok {
					continue // some sites' payloads may omit the bumpers node entirely — not this test's concern
				}
				if _, present := b1["FIRMWARE_VERSION"]; present {
					t.Errorf("%s: %s received buzzer-1.FIRMWARE_VERSION (admin-only)", s.label, name)
				}
			}
		})
	}
}

// TestArdoiseLeak128_TimerUpdate_MultipleConsecutiveTicks drives
// broadcastTimerUpdate several times in a row — the plan's own emphasis:
// this is a continuous, once-per-second leak, not a single call.
func TestArdoiseLeak128_TimerUpdate_MultipleConsecutiveTicks(t *testing.T) {
	app := setupArdoiseLeak128TestApp(t)

	baseURL := startAnimAllowlistTestServer(t, app)
	tvConn := dialWS(t, baseURL, "/ws/tv")
	learnClientID(t, app, tvConn)
	vpConn := dialWS(t, baseURL, "/ws/player")
	learnClientID(t, app, vpConn)

	for tick := 30; tick >= 26; tick-- {
		app.broadcastTimerUpdate(tick)

		_, tvRaw := readActionMatching(t, tvConn, protocol.ActionUpdateTimer)
		_, vpRaw := readActionMatching(t, vpConn, protocol.ActionUpdateTimer)

		if _, present := gameNodeOfRaw(t, vpRaw)["ARDOISE_ANSWERS"]; present {
			t.Fatalf("tick %d: VJoueur received ARDOISE_ANSWERS on UPDATE_TIMER — the continuous leak #128 identified as most severe", tick)
		}
		if _, present := gameNodeOfRaw(t, tvRaw)["ARDOISE_ANSWERS"]; !present {
			t.Errorf("tick %d: TV should still receive ARDOISE_ANSWERS on this tick", tick)
		}
	}
}

// TestArdoiseLeak128_UpdateAction_NonRegression covers the plain ActionUpdate
// path too (defect B applied there since before this fix) — TV/anim keep
// ARDOISE_ANSWERS, VPlayer still doesn't get it, via the ordinary
// broadcastUpdate() entry point rather than the 7 sites above.
func TestArdoiseLeak128_UpdateAction_NonRegression(t *testing.T) {
	app := setupArdoiseLeak128TestApp(t)

	baseURL := startAnimAllowlistTestServer(t, app)
	tvConn := dialWS(t, baseURL, "/ws/tv")
	learnClientID(t, app, tvConn)
	vpConn := dialWS(t, baseURL, "/ws/player")
	learnClientID(t, app, vpConn)
	animConn := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, animConn)

	app.broadcastUpdate()

	_, tvRaw := readActionMatching(t, tvConn, protocol.ActionUpdate)
	_, vpRaw := readActionMatching(t, vpConn, protocol.ActionUpdate)
	_, animRaw := readActionMatching(t, animConn, protocol.ActionUpdate)

	if _, present := gameNodeOfRaw(t, vpRaw)["ARDOISE_ANSWERS"]; present {
		t.Error("ActionUpdate: VJoueur must not receive ARDOISE_ANSWERS")
	}
	if _, present := gameNodeOfRaw(t, tvRaw)["ARDOISE_ANSWERS"]; !present {
		t.Error("ActionUpdate: TV must still receive ARDOISE_ANSWERS")
	}
	if _, present := gameNodeOfRaw(t, animRaw)["ARDOISE_ANSWERS"]; !present {
		t.Error("ActionUpdate: /anim must still receive ARDOISE_ANSWERS")
	}
}
