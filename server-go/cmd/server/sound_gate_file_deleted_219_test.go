// Diagnostic pour le retour QUALIF v11.1.0.5 (#219/#236/#237) : « le
// fichier .wav est supprimé du disque, puis la question est sélectionnée —
// aucun motif de blocage ne s'affiche ». Ce fichier verrouille, de bout en
// bout et avec du VRAI disque, que le serveur calcule et diffuse
// correctement `GAME.QUESTION_SOUND_UNAVAILABLE="FILE"` dans cette
// séquence exacte — la cause racine réelle du symptôme signalé s'est
// révélée être ailleurs (`web/src/utils/prepareWaitReason.js` : la
// vérification `buzzersWaiting` s'exécute AVANT la branche son et la
// masque tant qu'un buzzer n'a pas répondu — l'état normal juste après une
// sélection de question, `Ready()` remettant `bumper.Ready` à `false`).
// Le blocage réel (bouton désactivé via `canStart(phase)`, refus
// `Engine.Start()`) reste correct et n'est PAS affecté — ce test le
// prouve pour la partie serveur.
//
// Convention de collision : préfixe tw219g pour ne jamais entrer en
// collision avec un helper d'un autre fichier de ce paquet.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// tw219gBuildValidWAV returns a minimal canonical WAV (44100/stereo/16-bit,
// short silence) — same shape as the other #219 test helpers.
func tw219gBuildValidWAV() []byte {
	frames := int(0.2 * float64(audio.SampleRate))
	data := make([]byte, frames*audio.FrameSize)
	buf := make([]byte, 44+len(data))
	copy(buf[0:4], "RIFF")
	tw219gPutU32(buf[4:8], uint32(36+len(data)))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	tw219gPutU32(buf[16:20], 16)
	tw219gPutU16(buf[20:22], 1)
	tw219gPutU16(buf[22:24], uint16(audio.ChannelCount))
	tw219gPutU32(buf[24:28], uint32(audio.SampleRate))
	tw219gPutU32(buf[28:32], uint32(audio.SampleRate*audio.ChannelCount*audio.BytesPerSample))
	tw219gPutU16(buf[32:34], uint16(audio.ChannelCount*audio.BytesPerSample))
	tw219gPutU16(buf[34:36], uint16(audio.BitsPerSample))
	copy(buf[36:40], "data")
	tw219gPutU32(buf[40:44], uint32(len(data)))
	copy(buf[44:], data)
	return buf
}

func tw219gPutU32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func tw219gPutU16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

// TestSoundGate_FileDeletedBeforeSelection_ReportsFILE_EndToEnd reproduces
// the exact QUALIF v11.1.0.5 sequence via the REAL dispatch path: a real
// question.json + a real, valid .wav on disk, the file deleted OUT OF BAND
// (not via sound_cleared — the field in question.json still points at it,
// exactly like a manual filesystem deletion would), then a REAL READY
// message through the real handleReady handler — and inspects the ACTUAL
// bytes SerializeForAdmin would send, not just the in-memory GameState.
func TestSoundGate_FileDeletedBeforeSelection_ReportsFILE_EndToEnd(t *testing.T) {
	app := newTestAppWithHub(t)
	dir := t.TempDir()
	app.config.Storage.QuestionsDir = dir
	app.config.Sound.Enabled = true
	app.logger = server.NewBroadcastLogger(100)

	fake := &tw219dFakeMediaPlayer{}
	app.qsound = &questionSoundAdapter{app: app, player: fake, validationCache: make(map[string]soundValidationEntry)}

	qDir := filepath.Join(dir, "1")
	if err := os.MkdirAll(qDir, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	soundPath := filepath.Join(qDir, "sound_9897.wav")
	if err := os.WriteFile(soundPath, tw219gBuildValidWAV(), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	qjson := map[string]interface{}{
		"ID": "1", "QUESTION": "q", "ANSWER": "a", "TYPE": "SPEEDY",
		"POINTS": "10", "TIME": "30", "SOUND": "/question/1/sound_9897.wav",
	}
	data, _ := json.MarshalIndent(qjson, "", "  ")
	if err := os.WriteFile(filepath.Join(qDir, "question.json"), data, 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// The exact reported sequence: delete the file BEFORE selecting the
	// question (question.json's SOUND field still references it).
	if err := os.Remove(soundPath); err != nil {
		t.Fatalf("setup: %v", err)
	}

	msg, err := protocol.NewMessage(protocol.ActionReady, protocol.ReadyPayload{Question: "1"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	app.handleReady(msg)

	state := app.engine.GetState()
	if state.Question == nil || state.Question.Sound == "" {
		t.Fatalf("setup invalide : question.json non chargée correctement, Question=%+v", state.Question)
	}
	if state.QuestionSoundUnavailable != "FILE" {
		t.Errorf("GameState.QuestionSoundUnavailable = %q après handleReady (fichier supprimé avant sélection), attendu \"FILE\"", state.QuestionSoundUnavailable)
	}

	// The button's actual gate is phase-driven (web/src/utils/phaseRules.js
	// canStart: phase === 'READY') — confirm the phase itself stayed in
	// PREPARE, which is what really blocks the launch client-side AND
	// server-side (Engine.Start()'s own phase guard, CA18).
	if state.Phase != "PREPARE" {
		t.Errorf("GameState.Phase = %q, attendu PREPARE — la question ne doit jamais atteindre READY avec un son FILE non contourné", state.Phase)
	}

	fullMsg := &protocol.Message{Action: protocol.ActionUpdate, Msg: app.engine.GetGameJSON()}
	raw, err := fullMsg.SerializeForAdmin()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if !strings.Contains(string(raw), `"QUESTION_SOUND_UNAVAILABLE":"FILE"`) {
		t.Errorf("le JSON réellement envoyé à /admin ne contient pas QUESTION_SOUND_UNAVAILABLE:\"FILE\" — got: %s", string(raw))
	}
}
