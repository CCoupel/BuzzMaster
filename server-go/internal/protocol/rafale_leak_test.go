package protocol

import (
	"encoding/json"
	"testing"

	"buzzcontrol/internal/game"
)

// ---------------------------------------------------------------------------
// Tests : RAFALE — la réponse attendue ne doit JAMAIS transiter par
// GameState (milestone v8.0.0 #16, contrat contracts/rafale.md §2.3, §5.2) —
// patron ardoise_leak_128_test.go (#128).
//
// ⚠️ Écrit en Batch 1 (test-writer), à partir du contrat SEUL. Les
// constantes protocol.ActionRafaleAnswer/ActionRafaleTick et le payload
// RafaleAnswerPayload n'existent pas encore au moment où ce fichier est
// écrit — prévus tâche 18 du plan (#107, Batch 2, dev-backend). Ce fichier
// NE COMPILERA PAS tant que ces symboles ne sont pas ajoutés — attendu
// (TDD contract-first). game.RafaleCurrent, en revanche, existe déjà
// (models.go, livré en avance par dev-backend en Batch 1) : les tests qui
// ne dépendent que de lui tournent dès aujourd'hui.
//
// ⚠️ Architecture DIFFÉRENTE du précédent #128/ardoise : ARDOISE_ANSWERS
// est diffusé à TOUT LE MONDE puis retiré par client type (liste
// d'exclusion — VPlayerOnlyGameFields). RAFALE_ANSWER n'est PAS diffusé du
// tout à TV/VPlayer (contrat §2.3 : "aucune liste d'exclusion ne peut les
// séparer" puisque SerializeForWebClient sert TV ET anim identiquement) —
// la protection est au niveau du SITE D'APPEL (BroadcastToTypes(msg,
// ClientTypeAdmin, ClientTypeAnim), internal/server, jamais dans ce
// fichier). Ce fichier ne peut donc PROUVER que deux choses au niveau
// protocole :
//   1. La réponse n'existe structurellement PAS dans GameState (RafaleCurrent
//      n'a pas de champ ANSWER) — la vraie garantie architecturale du
//      contrat §2.3, testée ci-dessous sur toutes les actions de diffusion
//      partagées TV/anim/VPlayer.
//   2. Le payload RAFALE_ANSWER lui-même est bien formé pour ses
//      destinataires légitimes (admin/anim).
// La couverture "delivré à admin+anim SEULEMENT" (site d'appel) appelle un
// test complémentaire côté cmd/server une fois le handler livré (#107,
// Batch 2/3) — hors périmètre protocole, non fait ici.
// ---------------------------------------------------------------------------

// buildRafaleLeakMsg builds a realistic GameData-shaped message for the
// given action, with a GAME node carrying RAFALE_CURRENT_QUESTION exactly
// as game.RafaleCurrent serializes it — no ANSWER field, matching the real,
// already-committed struct (models.go) — plus PHASE/TIME/CURRENT_TIME so
// the payload has the same general shape ardoise_leak_128_test.go exercises.
func buildRafaleLeakMsg(t *testing.T, action string, phase string) *Message {
	t.Helper()
	current := game.RafaleCurrent{
		ID:         "r-042",
		Question:   "Capitale de l'Italie ?",
		Category:   "GEOGRAPHY",
		Difficulty: 2,
	}
	payload := map[string]interface{}{
		"GAME": map[string]interface{}{
			"PHASE":                   phase,
			"TIME":                    int64(1234567890),
			"CURRENT_TIME":            12,
			"RAFALE_CURRENT_QUESTION": current,
			"RAFALE_SUBPHASE":         "QUESTION",
		},
		"bumpers": map[string]interface{}{},
		"teams":   map[string]interface{}{},
	}
	rawMsg, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("buildRafaleLeakMsg: marshal failed: %v", err)
	}
	msg, err := NewMessage(action, nil)
	if err != nil {
		t.Fatalf("buildRafaleLeakMsg: NewMessage failed: %v", err)
	}
	msg.Msg = rawMsg
	return msg
}

// rafaleCurrentQuestionNodeOf extracts GAME.RAFALE_CURRENT_QUESTION as a raw
// map, for asserting on ITS keys specifically — the leak this test guards
// against (a stray ANSWER key) would live INSIDE this nested object, not at
// the top level of GAME, so gameNodeOf alone isn't enough here.
func rafaleCurrentQuestionNodeOf(t *testing.T, data []byte) map[string]interface{} {
	t.Helper()
	gameNode := gameNodeOf(t, data)
	current, ok := gameNode["RAFALE_CURRENT_QUESTION"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected GAME.RAFALE_CURRENT_QUESTION to be an object, got %v", gameNode["RAFALE_CURRENT_QUESTION"])
	}
	return current
}

// ---------------------------------------------------------------------------
// Garantie structurelle (le cœur du §2.3) : game.RafaleCurrent — le SEUL
// type qui porte la question courante dans GameState — n'a et ne doit
// jamais avoir de champ ANSWER. Casse à la compilation ou au premier
// json.Marshal si un futur dev ajoutait `Answer string` à ce struct (par
// exemple en réutilisant RafaleQuestion à la place de RafaleCurrent par
// erreur) : la reproduction exacte de la faille qu'ardoise_leak_128 a
// documentée, mais rendue impossible ici par construction plutôt que
// filtrée après coup.
// ---------------------------------------------------------------------------

func TestRafaleCurrent_NeverSerializesAnAnswerField(t *testing.T) {
	current := game.RafaleCurrent{
		ID: "r-1", Question: "Q?", Category: "HISTORY", Difficulty: 1,
	}
	data, err := json.Marshal(current)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, present := raw["ANSWER"]; present {
		t.Fatalf("game.RafaleCurrent must NEVER carry ANSWER (contract §2.3) — got %v", raw)
	}
	// Sanity: the type still round-trips its legitimate fields — proves
	// this isn't an accidentally-empty struct passing the check above for
	// the wrong reason.
	if raw["ID"] != "r-1" || raw["QUESTION"] != "Q?" {
		t.Errorf("sanity check failed — RafaleCurrent's own fields missing: %v", raw)
	}
}

// TestSerializeForWebClient_RafaleCurrentQuestion_NeverCarriesAnswer is the
// #128-style multi-action sweep: across every broadcast action known to
// carry a full GameState (buildArdoiseLeakMsg's own list — START/STOP/
// PAUSE/CONTINUE/UPDATE_TIMER/UPDATE), RAFALE_CURRENT_QUESTION must never
// gain an ANSWER key on the TV/anim path (SerializeForWebClient, shared by
// both — contract §2.3's own reasoning for why this can't be a filter
// list).
func TestSerializeForWebClient_RafaleCurrentQuestion_NeverCarriesAnswer(t *testing.T) {
	actions := []string{
		ActionUpdate, ActionStart, ActionStop, ActionPause, ActionContinue, ActionUpdateTimer,
	}
	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			msg := buildRafaleLeakMsg(t, action, "STARTED")

			data, err := msg.SerializeForWebClient()
			if err != nil {
				t.Fatalf("SerializeForWebClient(%s) failed: %v", action, err)
			}

			current := rafaleCurrentQuestionNodeOf(t, data)
			if _, present := current["ANSWER"]; present {
				t.Errorf("action %s: RAFALE_CURRENT_QUESTION.ANSWER must never reach TV/anim — this IS the #2.3 regression: %v", action, current)
			}
			// Sanity: the legitimate fields DO survive — proves this isn't
			// a coincidental empty-node pass.
			if current["ID"] != "r-042" {
				t.Errorf("action %s: sanity check failed — RAFALE_CURRENT_QUESTION.ID unexpectedly absent/wrong: %v", action, current)
			}
		})
	}
}

// TestSerializeForVPlayerCommon_RafaleCurrentQuestion_NeverCarriesAnswer is
// the VPlayer-path half — same guarantee, same actions, VPlayer being an
// even MORE restricted recipient than TV/anim (contract §8.1: VPlayer stays
// strictly passive during RAFALE).
func TestSerializeForVPlayerCommon_RafaleCurrentQuestion_NeverCarriesAnswer(t *testing.T) {
	actions := []string{
		ActionUpdate, ActionStart, ActionStop, ActionPause, ActionContinue, ActionUpdateTimer,
	}
	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			msg := buildRafaleLeakMsg(t, action, "STARTED")

			data, err := msg.SerializeForVPlayerCommon()
			if err != nil {
				t.Fatalf("SerializeForVPlayerCommon(%s) failed: %v", action, err)
			}

			current := rafaleCurrentQuestionNodeOf(t, data)
			if _, present := current["ANSWER"]; present {
				t.Errorf("action %s: RAFALE_CURRENT_QUESTION.ANSWER must never reach VPlayer: %v", action, current)
			}
		})
	}
}

// TestSerializeForVPlayer_RafaleCurrentQuestion_NeverCarriesAnswer mirrors
// ardoise_leak_128's identified-VPlayer-path test: SerializeForVPlayer
// (post-PLAYER_CONNECT) must share the same guarantee as the generic
// SerializeForVPlayerCommon it's built on top of.
func TestSerializeForVPlayer_RafaleCurrentQuestion_NeverCarriesAnswer(t *testing.T) {
	msg := buildRafaleLeakMsg(t, ActionUpdate, "STARTED")

	data, err := msg.SerializeForVPlayer("buzzer-1")
	if err != nil {
		t.Fatalf("SerializeForVPlayer failed: %v", err)
	}
	current := rafaleCurrentQuestionNodeOf(t, data)
	if _, present := current["ANSWER"]; present {
		t.Errorf("SerializeForVPlayer must also never carry RAFALE_CURRENT_QUESTION.ANSWER: %v", current)
	}
}

// ---------------------------------------------------------------------------
// RAFALE_ANSWER — le payload dédié (contrat §5.2 : {"ID":..., "ANSWER":...},
// destinataires admin+anim UNIQUEMENT via BroadcastToTypes). Ce que ce
// fichier PEUT prouver au niveau protocole : le payload est bien formé et
// traverse intact les chemins admin/anim (D1 fallback, comme REVEAL — pas
// de nœud GAME, rien à filtrer). Ce qu'il NE PEUT PAS prouver : que TV/
// VPlayer ne le reçoivent jamais — cette garantie est au site d'appel
// (internal/server, BroadcastToTypes), à couvrir par un test cmd/server
// dédié une fois le handler livré (#107, Batch 2/3).
// ---------------------------------------------------------------------------

func TestRafaleAnswerPayload_SurvivesAdminAndAnimSerialization(t *testing.T) {
	msg, err := NewMessage(ActionRafaleAnswer, RafaleAnswerPayload{ID: "r-042", Answer: "Rome"})
	if err != nil {
		t.Fatalf("NewMessage(RAFALE_ANSWER) failed: %v", err)
	}

	adminData, err := msg.SerializeForAdmin()
	if err != nil {
		t.Fatalf("SerializeForAdmin(RAFALE_ANSWER) failed: %v", err)
	}
	animData, err := msg.SerializeForWebClient() // shared TV/anim path (serializeForClientType)
	if err != nil {
		t.Fatalf("SerializeForWebClient(RAFALE_ANSWER) failed: %v", err)
	}

	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"admin", adminData},
		{"anim (SerializeForWebClient)", animData},
	} {
		m := parseMsgMap(t, tc.data)
		if m["ID"] != "r-042" || m["ANSWER"] != "Rome" {
			t.Errorf("%s: RAFALE_ANSWER payload must survive intact (no GAME node to filter), got %v", tc.name, m)
		}
	}
}
