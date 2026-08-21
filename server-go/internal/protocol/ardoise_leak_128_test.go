package protocol

import (
	"encoding/json"
	"testing"
)

// ---------------------------------------------------------------------------
// #128 — fuite de ARDOISE_ANSWERS (et, plus largement, QUIZ_OBJECTIVES et
// des champs firmware/OTA/ACK par buzzer) vers le VJoueur via des actions
// AUTRES que UPDATE.
//
// Plan : _work/reports/plan-128-20260820-170433.md (D1/D2, B1/B2/B6).
// Contrat : contracts/vplayer-payload-filter.md §6.
//
// Défaut A (B1) : SerializeForWebClient() ne filtrait qu'ActionUpdate — six
// autres actions (START/STOP/PAUSE/CONTINUE/UPDATE_TIMER, et le tic de
// COUNTDOWN, également UPDATE_TIMER) transportaient le GameState complet
// sans aucun retrait, vers TV, VPlayer ET animateur.
// Défaut B (B2) : ARDOISE_ANSWERS n'appartenait à aucune liste de retrait —
// même sur UPDATE, le VJoueur le recevait.
//
// Ce fichier teste au niveau protocole (SerializeForWebClient/
// SerializeForVPlayerCommon), indépendamment du transport WS — voir
// cmd/server/ardoise_leak_128_test.go pour la couverture bout-en-bout à
// travers les 7 sites de diffusion réels.
// ---------------------------------------------------------------------------

// buildArdoiseLeakMsg builds a realistic message for the given action, with
// a GAME node carrying PHASE/ARDOISE_ANSWERS/QUIZ_OBJECTIVES and a bumper
// carrying an admin-only firmware field — everything #128 concerns.
func buildArdoiseLeakMsg(t *testing.T, action string, phase string) *Message {
	t.Helper()
	payload := map[string]interface{}{
		"GAME": map[string]interface{}{
			"PHASE":           phase,
			"TIME":            int64(1234567890),
			"CURRENT_TIME":    12,
			"QUIZ_OBJECTIVES": "Ne doit jamais fuiter vers TV/VJoueur/anim",
			"ARDOISE_ANSWERS": map[string]interface{}{
				"TeamA": map[string]interface{}{"TEXT": "Paris", "SUBMITTED_AT": 1000},
				"TeamB": map[string]interface{}{"TEXT": "Berlin", "SUBMITTED_AT": 1200},
			},
		},
		"bumpers": map[string]interface{}{
			"buzzer-1": map[string]interface{}{
				"NAME": "Buzzer1", "TEAM": "TeamA", "CONNECTED": true,
				"FIRMWARE_VERSION": "3.1.1", "IS_OUTDATED": false,
			},
		},
		"teams": map[string]interface{}{
			"TeamA": map[string]interface{}{"NAME": "TeamA", "SCORE": 0},
		},
	}
	rawMsg, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("buildArdoiseLeakMsg: marshal failed: %v", err)
	}
	msg, err := NewMessage(action, nil)
	if err != nil {
		t.Fatalf("buildArdoiseLeakMsg: NewMessage failed: %v", err)
	}
	msg.Msg = rawMsg
	return msg
}

// TestSerializeForWebClient_ArdoiseAnswersAbsent_AcrossAllSevenBroadcastSites
// is THE central #128 test: for every action identified as a leak site
// (defect A's table), ARDOISE_ANSWERS/QUIZ_OBJECTIVES/firmware fields must
// be stripped, exactly as they already were for ActionUpdate.
func TestSerializeForWebClient_LeakFieldsAbsent_AcrossAllSevenBroadcastSites(t *testing.T) {
	// The 6 distinct action strings behind the plan's 7 sites (broadcastPause
	// and broadcastPauseAll share ActionPause; broadcastTimerUpdate and
	// broadcastCountdownUpdate share ActionUpdateTimer).
	actions := []string{
		ActionUpdate, ActionStart, ActionStop, ActionPause, ActionContinue, ActionUpdateTimer,
	}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			msg := buildArdoiseLeakMsg(t, action, "STARTED")

			data, err := msg.SerializeForWebClient()
			if err != nil {
				t.Fatalf("SerializeForWebClient(%s) failed: %v", action, err)
			}

			game := gameNodeOf(t, data)
			if _, present := game["QUIZ_OBJECTIVES"]; present {
				t.Errorf("action %s: QUIZ_OBJECTIVES must never reach TV/VJoueur/anim, got present: %v", action, game)
			}
			// SerializeForWebClient serves BOTH TV and anim — ARDOISE_ANSWERS
			// must survive HERE (only the VPlayer-specific serializer strips
			// it, see the VPlayer-only test below).
			if _, present := game["ARDOISE_ANSWERS"]; !present {
				t.Errorf("action %s: ARDOISE_ANSWERS must still reach TV/anim (SerializeForWebClient), got absent: %v", action, game)
			}

			bumpers, ok := parseMsgMap(t, data)["bumpers"].(map[string]interface{})
			if !ok {
				t.Fatalf("action %s: expected a bumpers map in the payload", action)
			}
			bumper1, ok := bumpers["buzzer-1"].(map[string]interface{})
			if !ok {
				t.Fatalf("action %s: expected bumper buzzer-1 in the payload", action)
			}
			if _, present := bumper1["FIRMWARE_VERSION"]; present {
				t.Errorf("action %s: FIRMWARE_VERSION (admin-only) must not reach TV/anim, got present: %v", action, bumper1)
			}
		})
	}
}

// TestSerializeForVPlayerCommon_ArdoiseAnswersAbsent_AcrossAllSevenBroadcastSites
// is the VPlayer-specific half: same actions, but ARDOISE_ANSWERS must be
// gone too (VPlayerOnlyGameFields, D2) — the actual #128 regression.
func TestSerializeForVPlayerCommon_ArdoiseAnswersAbsent_AcrossAllSevenBroadcastSites(t *testing.T) {
	actions := []string{
		ActionUpdate, ActionStart, ActionStop, ActionPause, ActionContinue, ActionUpdateTimer,
	}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			msg := buildArdoiseLeakMsg(t, action, "STARTED")

			data, err := msg.SerializeForVPlayerCommon()
			if err != nil {
				t.Fatalf("SerializeForVPlayerCommon(%s) failed: %v", action, err)
			}

			game := gameNodeOf(t, data)
			if _, present := game["ARDOISE_ANSWERS"]; present {
				t.Errorf("action %s: ARDOISE_ANSWERS must NEVER reach a VJoueur — this is the #128 regression itself, got present: %v", action, game)
			}
			if _, present := game["QUIZ_OBJECTIVES"]; present {
				t.Errorf("action %s: QUIZ_OBJECTIVES must not reach a VJoueur, got present: %v", action, game)
			}
			// Sanity: PHASE (a normal, non-restricted field) DOES survive —
			// proves this isn't a coincidental empty-GAME-node pass.
			if game["PHASE"] != "STARTED" {
				t.Errorf("action %s: sanity check failed — PHASE unexpectedly absent/wrong: %v", action, game)
			}
		})
	}
}

// TestSerializeForVPlayer_ArdoiseAnswersAbsent verifies the identified
// VPlayer path (SerializeForVPlayer(playerID), used once a client has sent
// PLAYER_CONNECT) shares the same guarantee as the generic
// SerializeForVPlayerCommon — it must be BUILT ON TOP of it (B2), not a
// separate implementation that could drift.
func TestSerializeForVPlayer_ArdoiseAnswersAbsent(t *testing.T) {
	for _, action := range []string{ActionUpdate, ActionStop, ActionUpdateTimer} {
		t.Run(action, func(t *testing.T) {
			msg := buildArdoiseLeakMsg(t, action, "STARTED")

			data, err := msg.SerializeForVPlayer("buzzer-1")
			if err != nil {
				t.Fatalf("SerializeForVPlayer(%s) failed: %v", action, err)
			}

			game := gameNodeOf(t, data)
			if _, present := game["ARDOISE_ANSWERS"]; present {
				t.Errorf("action %s: SerializeForVPlayer must also strip ARDOISE_ANSWERS (built on SerializeForVPlayerCommon), got present: %v", action, game)
			}
		})
	}
}

// TestSerializeForWebClient_MultipleTimerTicks_NeverLeaksArdoiseAnswersToVPlayer
// specifically drives broadcastTimerUpdate's action (ActionUpdateTimer)
// across SEVERAL consecutive ticks — the plan's own emphasis: this is the
// most severe site (once per second for the whole question), not a single
// isolated call.
func TestSerializeForVPlayerCommon_MultipleTimerTicks_NeverLeaksArdoiseAnswers(t *testing.T) {
	for tick := 0; tick < 5; tick++ {
		msg := buildArdoiseLeakMsg(t, ActionUpdateTimer, "STARTED")
		data, err := msg.SerializeForVPlayerCommon()
		if err != nil {
			t.Fatalf("tick %d: SerializeForVPlayerCommon failed: %v", tick, err)
		}
		game := gameNodeOf(t, data)
		if _, present := game["ARDOISE_ANSWERS"]; present {
			t.Fatalf("tick %d: ARDOISE_ANSWERS leaked to VPlayer on an UPDATE_TIMER tick — this is the continuous, once-per-second leak #128 identified as the most severe: %v", tick, game)
		}
	}
}

// ---------------------------------------------------------------------------
// Non-régression — actions sans nœud GAME (REVEAL, HELLO...) traversent
// inchangées : le repli de désérialisation (D1) doit renvoyer la charge
// utile intacte, jamais une erreur ni un objet vidé.
// ---------------------------------------------------------------------------

func TestSerializeForWebClient_RevealStringPayload_PassesThroughUnchanged(t *testing.T) {
	msg, err := NewMessage(ActionReveal, "La bonne réponse est Paris")
	if err != nil {
		t.Fatalf("NewMessage(REVEAL) failed: %v", err)
	}

	data, err := msg.SerializeForWebClient()
	if err != nil {
		t.Fatalf("SerializeForWebClient(REVEAL) failed: %v", err)
	}

	var envelope struct {
		Action string `json:"ACTION"`
		Msg    string `json:"MSG"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("failed to unmarshal REVEAL payload: %v (raw: %s)", err, data)
	}
	if envelope.Msg != "La bonne réponse est Paris" {
		t.Errorf("REVEAL payload must pass through unchanged (D1 fallback), got %q", envelope.Msg)
	}
}

func TestSerializeForVPlayerCommon_RevealStringPayload_PassesThroughUnchanged(t *testing.T) {
	msg, err := NewMessage(ActionReveal, "La bonne réponse est Paris")
	if err != nil {
		t.Fatalf("NewMessage(REVEAL) failed: %v", err)
	}

	data, err := msg.SerializeForVPlayerCommon()
	if err != nil {
		t.Fatalf("SerializeForVPlayerCommon(REVEAL) failed: %v", err)
	}

	var envelope struct {
		Msg string `json:"MSG"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("failed to unmarshal REVEAL payload: %v (raw: %s)", err, data)
	}
	if envelope.Msg != "La bonne réponse est Paris" {
		t.Errorf("REVEAL payload must pass through unchanged for VPlayer too, got %q", envelope.Msg)
	}
}

// ---------------------------------------------------------------------------
// D1 — le filtre doit dépendre de la FORME de la charge utile, jamais du nom
// de l'action : une action fictive, inconnue de ce fichier et du reste du
// code, transportant un nœud GAME doit être filtrée exactement comme les
// six actions connues. Garde contre la régression silencieuse identifiée
// comme risque n°1 du plan : une future action transportant GameState sans
// que personne n'ait pensé à l'ajouter à une liste.
// ---------------------------------------------------------------------------

func TestSerializeForWebClient_UnknownFutureAction_StillFiltersByPayloadShape(t *testing.T) {
	msg := buildArdoiseLeakMsg(t, "SOME_FUTURE_ACTION_NOBODY_HAS_HEARD_OF_YET", "STARTED")

	data, err := msg.SerializeForWebClient()
	if err != nil {
		t.Fatalf("SerializeForWebClient(unknown action) failed: %v", err)
	}
	game := gameNodeOf(t, data)
	if _, present := game["QUIZ_OBJECTIVES"]; present {
		t.Errorf("an unrecognized action carrying a GAME node must still be filtered by SHAPE (D1) — QUIZ_OBJECTIVES leaked: %v", game)
	}
}

func TestSerializeForVPlayerCommon_UnknownFutureAction_StillStripsArdoiseAnswers(t *testing.T) {
	msg := buildArdoiseLeakMsg(t, "SOME_FUTURE_ACTION_NOBODY_HAS_HEARD_OF_YET", "STARTED")

	data, err := msg.SerializeForVPlayerCommon()
	if err != nil {
		t.Fatalf("SerializeForVPlayerCommon(unknown action) failed: %v", err)
	}
	game := gameNodeOf(t, data)
	if _, present := game["ARDOISE_ANSWERS"]; present {
		t.Errorf("an unrecognized action carrying a GAME node must still lose ARDOISE_ANSWERS for VPlayer (D1) — the whole point of filtering by shape, not by an enumerated action list: %v", game)
	}
}

// TestSerializeForWebClient_NoGameNode_PassesThroughUnchanged covers a
// payload that parses as valid JSON but has neither "GAME" nor "bumpers" —
// the other half of the D1 fallback (not just malformed JSON).
func TestSerializeForWebClient_NoGameNode_PassesThroughUnchanged(t *testing.T) {
	msg, err := NewMessage("SOME_UNRELATED_ACTION", map[string]interface{}{"FOO": "bar"})
	if err != nil {
		t.Fatalf("NewMessage failed: %v", err)
	}

	data, err := msg.SerializeForWebClient()
	if err != nil {
		t.Fatalf("SerializeForWebClient failed: %v", err)
	}
	m := parseMsgMap(t, data)
	if m["FOO"] != "bar" {
		t.Errorf("a payload without GAME/bumpers must pass through unchanged, got %v", m)
	}
}
