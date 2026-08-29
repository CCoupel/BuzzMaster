package main

import (
	"encoding/json"
	"testing"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// Tests : garde-fou anti-fuite RAFALE_ANSWER au site d'appel cmd/server
// (milestone v8.0.0 #16, review code-review-20260828-182815.md, MAJEUR-1).
//
// internal/protocol/rafale_leak_test.go prouve que game.RafaleCurrent (le
// seul type portant la question courante dans GameState) n'a
// STRUCTURELLEMENT pas de champ ANSWER — mais documente explicitement (§2.3,
// lignes 24-41/203-212 de ce fichier) que rien au niveau protocole ne peut
// prouver que RAFALE_ANSWER lui-même n'atteint QUE admin+anim, puisque
// SerializeForWebClient sert un payload identique à TV et /anim : la seule
// protection réelle est la liste de ClientType passée à BroadcastToTypes au
// SITE D'APPEL, broadcastRafaleAnswer (main.go). Ce fichier verrouille
// EXACTEMENT ce site d'appel, à travers le vrai chemin de dispatch
// (RAFALE_VALIDATE/RAFALE_INVALIDATE → handleWebMessage →
// Engine.RafaleValidate/Invalidate → OnRafaleAnswer callback →
// broadcastRafaleAnswer), pas un appel direct à la fonction — un futur
// copier-coller depuis broadcastRafaleTick (qui liste bien
// ClientTypeTV/ClientTypeVPlayer, à raison : RAFALE_TICK est public) vers
// broadcastRafaleAnswer élargirait silencieusement la diffusion sans qu'
// aucun test ne le détecte autrement — reproduction exacte de la classe de
// bug ardoise_leak_128 (#128) que rafale_leak_test.go cite comme précédent.
//
// Patron de harnais : credit_points_test.go's TestBroadcastCreditPoints_
// OnlyReachesAnim (même structure "admin+anim reçoivent, TV ne reçoit
// jamais"), étendu à 4 types de client (+VPlayer) et déclenché via le vrai
// dispatch WS plutôt qu'un appel direct à la fonction de broadcast, comme
// demandé par la revue.
// ---------------------------------------------------------------------------

const rafaleAnswerLeakReservoirCategory = game.CategoryHistory
const rafaleAnswerLeakReservoirDifficulty = 1

// newRafaleAnswerLeakTestApp builds a test App with a live RAFALE round
// already in its QUESTION sub-phase (SOLO mode — team selection is
// irrelevant to this leak guard) and OnRafaleAnswer wired to
// broadcastRafaleAnswer, exactly as production's setupCallbacks does. This
// harness (like inbound_allowlist_test.go's sendAction) does NOT run the
// full setupCallbacks background dispatch loop — only the one callback this
// test actually needs is wired, same discipline as main_broadcast_127_test.go/
// vplayer_broadcast_integration_test.go's own manual OnStateChange wiring.
func newRafaleAnswerLeakTestApp(t *testing.T) *App {
	t.Helper()
	app := newAnimTestApp(t)

	app.engine.OnRafaleAnswer = func(id, answer string) {
		app.broadcastRafaleAnswer(id, answer)
	}

	// Seed the reservoir with 2 questions matching one category/difficulty —
	// one is drawn automatically at round start (StartImmediate), a second
	// one must still be available for the RAFALE_VALIDATE/INVALIDATE-driven
	// advance this test actually exercises (contract §7 — DrawRafaleQuestion
	// never reproposes an already-used question).
	for _, rq := range []game.RafaleQuestion{
		{ID: "r-1", Question: "Q1", Answer: "SECRET-ANSWER-1", Category: rafaleAnswerLeakReservoirCategory, Difficulty: rafaleAnswerLeakReservoirDifficulty},
		{ID: "r-2", Question: "Q2", Answer: "SECRET-ANSWER-2", Category: rafaleAnswerLeakReservoirCategory, Difficulty: rafaleAnswerLeakReservoirDifficulty},
	} {
		if _, err := app.engine.UpsertRafaleQuestion(rq); err != nil {
			t.Fatalf("UpsertRafaleQuestion(%q) failed: %v", rq.ID, err)
		}
	}

	q := &game.Question{
		ID:       "rafale-leak-q",
		Question: "RAFALE round",
		Type:     game.QuestionTypeRafale,
		Category: rafaleAnswerLeakReservoirCategory,
		Points:   "10",
		Time:     "120",
		TypedContent: game.TypedContent{
			RafaleDifficulty:   rafaleAnswerLeakReservoirDifficulty,
			RafaleMode:         string(game.RafaleModeSolo),
			RafaleQuestionTime: 3,
			RafaleMaxQuestions: 100,
		},
	}
	app.engine.Ready(q.ID, q)
	app.engine.StartImmediate(0) // draws r-1 (or r-2, uniform) — no listener connected yet, harmless

	return app
}

// assertRafaleAnswerNeverReceived drains conn for up to 300ms and fails the
// test if a RAFALE_ANSWER frame ever arrives — same idle-drain shape as
// TestBroadcastCreditPoints_OnlyReachesAnim.
func assertRafaleAnswerNeverReceived(t *testing.T, name string, conn interface {
	SetReadDeadline(time.Time) error
	ReadMessage() (int, []byte, error)
}) {
	t.Helper()
	deadline := time.Now().Add(300 * time.Millisecond)
	for {
		conn.SetReadDeadline(deadline)
		_, data, err := conn.ReadMessage()
		if err != nil {
			return // timeout (or closed) — nothing (more) to read, as required
		}
		var envelope struct {
			Action string `json:"ACTION"`
		}
		if json.Unmarshal(data, &envelope) == nil && envelope.Action == protocol.ActionRafaleAnswer {
			t.Errorf("%s must never receive RAFALE_ANSWER (contract §2.3) — got frame: %s", name, data)
			return
		}
		// Some other, legitimate frame for this client type — keep draining
		// until the deadline in case RAFALE_ANSWER arrives after it.
	}
}

// TestRafaleAnswer_ValidateReachesAdminAndAnim_NeverTVOrVPlayer is the
// review's exact scenario for RAFALE_VALIDATE: admin sends it, admin+anim
// must receive RAFALE_ANSWER (with the real answer), TV and VPlayer must
// never receive it — dispatched through the genuine WS path, not a direct
// function call.
func TestRafaleAnswer_ValidateReachesAdminAndAnim_NeverTVOrVPlayer(t *testing.T) {
	app := newRafaleAnswerLeakTestApp(t)

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	anim := dialWS(t, baseURL, "/ws/anim")
	tv := dialWS(t, baseURL, "/ws/tv")
	vplayer := dialWS(t, baseURL, "/ws/player") // no explicit case in the routing switch → falls through to ClientTypeVPlayer, matching production's real /ws/player route
	learnClientID(t, app, admin)
	learnClientID(t, app, anim)
	learnClientID(t, app, tv)
	learnClientID(t, app, vplayer)

	sendAction(t, app, admin, protocol.ActionRafaleValidate, struct{}{})

	for _, recipient := range []struct {
		name string
		conn *websocket.Conn
	}{{"admin", admin}, {"anim", anim}} {
		_, raw := readActionMatching(t, recipient.conn, protocol.ActionRafaleAnswer)
		var parsed struct {
			Msg protocol.RafaleAnswerPayload `json:"MSG"`
		}
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			t.Fatalf("%s: failed to unmarshal RAFALE_ANSWER: %v (raw: %s)", recipient.name, err, raw)
		}
		if parsed.Msg.Answer == "" || parsed.Msg.ID == "" {
			t.Errorf("%s: expected a non-empty RAFALE_ANSWER payload, got %+v", recipient.name, parsed.Msg)
		}
	}

	assertRafaleAnswerNeverReceived(t, "tv", tv)
	assertRafaleAnswerNeverReceived(t, "vplayer", vplayer)
}

// TestRafaleAnswer_InvalidateReachesAdminAndAnim_NeverTVOrVPlayer mirrors
// the VALIDATE test above for RAFALE_INVALIDATE — contract §6.1 routes both
// through the same advance/broadcast path, this proves the leak guard holds
// for both entry points, not just one.
func TestRafaleAnswer_InvalidateReachesAdminAndAnim_NeverTVOrVPlayer(t *testing.T) {
	app := newRafaleAnswerLeakTestApp(t)

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	anim := dialWS(t, baseURL, "/ws/anim")
	tv := dialWS(t, baseURL, "/ws/tv")
	vplayer := dialWS(t, baseURL, "/ws/player")
	learnClientID(t, app, admin)
	learnClientID(t, app, anim)
	learnClientID(t, app, tv)
	learnClientID(t, app, vplayer)

	sendAction(t, app, admin, protocol.ActionRafaleInvalidate, struct{}{})

	for _, recipient := range []struct {
		name string
		conn *websocket.Conn
	}{{"admin", admin}, {"anim", anim}} {
		_, raw := readActionMatching(t, recipient.conn, protocol.ActionRafaleAnswer)
		var parsed struct {
			Msg protocol.RafaleAnswerPayload `json:"MSG"`
		}
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			t.Fatalf("%s: failed to unmarshal RAFALE_ANSWER: %v (raw: %s)", recipient.name, err, raw)
		}
	}

	assertRafaleAnswerNeverReceived(t, "tv", tv)
	assertRafaleAnswerNeverReceived(t, "vplayer", vplayer)
}
