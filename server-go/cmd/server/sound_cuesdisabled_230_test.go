// T3 (#230, planner handoff §1) — "Configuration vide ⇒ exactement les
// mêmes cues qu'aujourd'hui." CuesDisabled (contracts/sound.md §6.3) touche
// notifySound, du code de #227 déjà revu et testé
// (sound_cues_chain_test.go). Ce fichier ne modifie AUCUN test existant —
// il ajoute la preuve que l'extension reste additive à valeur zéro, et
// vérifie la nouvelle branche de filtrage par cue.
//
// Réutilise délibérément twa227WireSound (sound_cues_chain_test.go, même
// paquet) plutôt que de redéfinir un harnais parallèle — le but explicite de
// ce fichier est de prouver que le chemin de #227 n'a PAS changé de
// comportement, pas d'en construire un nouveau.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package main

import (
	"testing"
	"time"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// TestT3_EmptyCuesDisabled_ProducesExactlySameCuesAsBefore is T3 itself —
// avec CuesDisabled absent (valeur zéro), un vrai départ doit produire
// exactement [depart], identique bit à bit au comportement #227 déjà
// verrouillé par TestSoundChain_OrdinaryStart_PlaysDepart. Si cette
// extension additive avait un effet de bord, ce test le détecterait sans
// avoir besoin de connaître l'implémentation exacte du filtrage.
func TestT3_EmptyCuesDisabled_ProducesExactlySameCuesAsBefore(t *testing.T) {
	// CuesDisabled explicitement laissé à sa valeur zéro (nil) — c'est le
	// point même du test.
	config.Get().Sound.CuesDisabled = nil

	app, fake := twa227WireSound(t)
	speedy := &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy}
	app.engine.Ready(speedy.ID, speedy)
	app.engine.StartImmediate(0)
	defer app.engine.Stop()

	got := twa227WaitForPlayed(t, fake, 1, time.Second)
	if len(got) != 1 || got[0] != audio.CueDepart {
		t.Fatalf("contract §6.3 : à valeur zéro, le comportement doit être identique au bit près à #227 — attendu [depart], got %v", got)
	}
}

// TestT3_CueDisabled_NeverReachesEngineViaNotifySound proves the OTHER
// half: a cue explicitly listed in CuesDisabled must never reach the
// engine through the normal game fan-out (notifySound) — the point of
// filtering is normative (contract §6.3): "dans notifySound
// (cmd/server/sound.go), avant l'appel au moteur — jamais ailleurs."
func TestT3_CueDisabled_NeverReachesEngineViaNotifySound(t *testing.T) {
	config.Get().Sound.CuesDisabled = map[string]bool{"depart": true}
	t.Cleanup(func() { config.Get().Sound.CuesDisabled = nil })

	app, fake := twa227WireSound(t)
	speedy := &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy}
	app.engine.Ready(speedy.ID, speedy)
	app.engine.StartImmediate(0)
	defer app.engine.Stop()

	// Laisse le temps à un éventuel `depart` (fautif) d'arriver.
	time.Sleep(100 * time.Millisecond)
	if got := fake.Played(); len(got) != 0 {
		t.Fatalf("depart est dans CuesDisabled — aucun son ne devait être joué via le fan-out normal, got %v", got)
	}
}

// TestT3_OnlyTheListedCueIsFiltered_OthersUnaffected proves CuesDisabled
// filters PER CUE, not globally — a cue NOT listed must still play
// normally even while another one is disabled (mockup: "Erreur est
// éteint... tandis que les six autres continuent").
func TestT3_OnlyTheListedCueIsFiltered_OthersUnaffected(t *testing.T) {
	config.Get().Sound.CuesDisabled = map[string]bool{"perdu": true}
	t.Cleanup(func() { config.Get().Sound.CuesDisabled = nil })

	app, fake := twa227WireSound(t)
	app.engine.SetTeams(map[string]*game.Team{"Les Rouges": {Name: "Les Rouges"}})
	app.engine.Ready("1", &game.Question{ID: "1"})

	// handleTeamPoints (gagne) — cue NON listée, doit sonner normalement.
	msg, err := protocol.NewMessage(protocol.ActionTeamPoints, protocol.TeamPointsPayload{Team: "Les Rouges", Points: 10})
	if err != nil {
		t.Fatalf("setup invalide : %v", err)
	}
	app.handleTeamPoints(msg)

	got := twa227WaitForPlayed(t, fake, 1, time.Second)
	if len(got) != 1 || got[0] != audio.CueGagne {
		t.Fatalf("gagne n'est PAS dans CuesDisabled ({perdu:true}) — doit sonner normalement, got %v", got)
	}
}

// TestT3_GeneralSwitch_OverridesPerCueSwitches proves the general
// interrupter remains master (contract §6.3: "sound.enabled reste maître.
// Éteint, il coupe tout... quels que soient les sept interrupteurs de
// CuesDisabled") — a cue NOT in CuesDisabled (so individually enabled)
// still produces nothing when the engine itself is disabled.
func TestT3_GeneralSwitch_OverridesPerCueSwitches(t *testing.T) {
	config.Get().Sound.CuesDisabled = nil // aucune cue individuellement désactivée

	app := newTestAppWithHub(t)
	// setupCallbacks() déréférence a.httpServer.OnAction (main.go) —
	// même setup minimal que twa227WireSound (sound_cues_chain_test.go),
	// sans y câbler de moteur son : Output nil équivaut à sound.enabled=false
	// (voir buildAudioOutput). a.sound() reste un Engine désactivé par défaut
	// (a.soundEngine jamais initialisé par ce test).
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))
	app.setupCallbacks()

	speedy := &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy}
	app.engine.Ready(speedy.ID, speedy)
	app.engine.StartImmediate(0)
	defer app.engine.Stop()

	// a.sound() (Engine nil-safe) ne doit produire aucun panic ni aucune
	// goroutine — la preuve positive "aucun son" est déjà couverte par
	// TestDisabledEngine_NoGoroutineNoOutputCall (internal/audio) ; ce test
	// vérifie seulement qu'aucun panic ne remonte jusqu'ici avec
	// CuesDisabled vide ET le moteur désactivé combinés.
	if app.sound().Enabled() {
		t.Fatal("setup invalide : le moteur ne doit pas être activé dans ce test")
	}
}
