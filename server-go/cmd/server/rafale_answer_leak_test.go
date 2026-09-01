package main

import (
	"encoding/json"
	"strings"
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

	app.engine.OnRafaleAnswer = func(id, answer string, next *game.RafaleCurrent) {
		app.broadcastRafaleAnswer(id, answer, next)
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

// ---------------------------------------------------------------------------
// #202, contract §13.2/§13.8 — NEXT (the pre-fetched next question's
// statement) extends this SAME restricted channel and the SAME leak-guard
// discipline as ANSWER above. newRafaleAnswerLeakTestApp's 2-question
// reservoir is deliberately too small for these tests (round start already
// consumes 1 + pre-fetches the other, leaving the pool empty — NEXT would
// read nil, nothing to assert a leak of) — a dedicated 3-question harness
// below keeps at least one real NEXT value in flight throughout.
// ---------------------------------------------------------------------------

const (
	rafaleNextLeakStatement1 = "SECRET-NEXT-STATEMENT-Q1-#202"
	rafaleNextLeakStatement2 = "SECRET-NEXT-STATEMENT-Q2-#202"
	rafaleNextLeakStatement3 = "SECRET-NEXT-STATEMENT-Q3-#202"
)

// newRafaleNextLeakTestApp mirrors newRafaleAnswerLeakTestApp above but with
// 3 reservoir questions (round start consumes 1 + pre-fetches a 2nd,
// leaving exactly 1 still in the pool — enough for RAFALE_VALIDATE below to
// still observe a REAL, non-nil NEXT rather than the pool having already
// gone dry).
func newRafaleNextLeakTestApp(t *testing.T) *App {
	t.Helper()
	app := newAnimTestApp(t)

	app.engine.OnRafaleAnswer = func(id, answer string, next *game.RafaleCurrent) {
		app.broadcastRafaleAnswer(id, answer, next)
	}

	for _, rq := range []game.RafaleQuestion{
		{ID: "rn-1", Question: rafaleNextLeakStatement1, Answer: "A1", Category: rafaleAnswerLeakReservoirCategory, Difficulty: rafaleAnswerLeakReservoirDifficulty},
		{ID: "rn-2", Question: rafaleNextLeakStatement2, Answer: "A2", Category: rafaleAnswerLeakReservoirCategory, Difficulty: rafaleAnswerLeakReservoirDifficulty},
		{ID: "rn-3", Question: rafaleNextLeakStatement3, Answer: "A3", Category: rafaleAnswerLeakReservoirCategory, Difficulty: rafaleAnswerLeakReservoirDifficulty},
	} {
		if _, err := app.engine.UpsertRafaleQuestion(rq); err != nil {
			t.Fatalf("UpsertRafaleQuestion(%q) failed: %v", rq.ID, err)
		}
	}

	q := &game.Question{
		ID:       "rafale-next-leak-q",
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
	app.engine.StartImmediate(0) // draws 1 as current, pre-fetches a 2nd — 1 left in the pool

	return app
}

// assertNoFrameContainsSecret drains conn for up to 300ms and fails the
// test if any frame's raw bytes contain secret anywhere — a substring scan
// across EVERY action/field, not just RAFALE_ANSWER's own absence, closing
// the door on the statement leaking through some other field entirely
// (defense in depth beyond assertRafaleAnswerNeverReceived above).
func assertNoFrameContainsSecret(t *testing.T, name string, conn interface {
	SetReadDeadline(time.Time) error
	ReadMessage() (int, []byte, error)
}, secret string) {
	t.Helper()
	if secret == "" {
		t.Fatal("assertNoFrameContainsSecret: empty secret — this would trivially pass for the wrong reason")
	}
	deadline := time.Now().Add(300 * time.Millisecond)
	for {
		conn.SetReadDeadline(deadline)
		_, data, err := conn.ReadMessage()
		if err != nil {
			return // timeout (or closed) — nothing (more) to read
		}
		if strings.Contains(string(data), secret) {
			t.Errorf("BUG (#202, contract §13.2): %s received a frame containing the NEXT question's statement %q — %s", name, secret, data)
			return
		}
	}
}

// TestRafaleAnswerNext_ReachesAdminAndAnim_ConsistentBetweenBoth is the
// positive control: RAFALE_ANSWER's own NEXT field carries a real,
// non-nil, IDENTICAL preview to both admin and anim (contract §13.3) —
// proven before the negative (leak) assertions below, so a failure there
// can't be misread as "NEXT just isn't wired up at all".
func TestRafaleAnswerNext_ReachesAdminAndAnim_ConsistentBetweenBoth(t *testing.T) {
	app := newRafaleNextLeakTestApp(t)

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	anim := dialWS(t, baseURL, "/ws/anim")
	learnClientID(t, app, admin)
	learnClientID(t, app, anim)

	sendAction(t, app, admin, protocol.ActionRafaleValidate, struct{}{})

	var statements []string
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
		if parsed.Msg.Next == nil {
			t.Fatalf("%s: expected a non-nil NEXT (1 question still in the pool), got nil", recipient.name)
		}
		if parsed.Msg.Next.ID == "" || parsed.Msg.Next.Question == "" {
			t.Errorf("%s: expected a fully-populated NEXT, got %+v", recipient.name, parsed.Msg.Next)
		}
		statements = append(statements, parsed.Msg.Next.Question)
	}
	if statements[0] != statements[1] {
		t.Errorf("admin and anim disagree on the NEXT statement: %q vs %q — both must see the exact same pre-fetch", statements[0], statements[1])
	}
}

// TestRafaleAnswerNext_NeverLeaksToTVOrPlayer is the critical, BLOCKING
// test (plan task 11, contract §13.8): the pre-fetched next question's
// statement must never reach /ws/tv or /ws/player — neither as
// RAFALE_ANSWER itself (assertRafaleAnswerNeverReceived, same as the
// existing ANSWER-only tests above) NOR embedded in any other frame
// (assertNoFrameContainsSecret's raw substring scan across every action).
func TestRafaleAnswerNext_NeverLeaksToTVOrPlayer(t *testing.T) {
	app := newRafaleNextLeakTestApp(t)

	baseURL := startAnimAllowlistTestServer(t, app)
	admin := dialWS(t, baseURL, "/ws/admin")
	tv := dialWS(t, baseURL, "/ws/tv")
	vplayer := dialWS(t, baseURL, "/ws/player")
	learnClientID(t, app, admin)
	learnClientID(t, app, tv)
	learnClientID(t, app, vplayer)

	sendAction(t, app, admin, protocol.ActionRafaleValidate, struct{}{})

	// Drain+consume the admin frame first so we know the EXACT statement
	// text to search for on tv/vplayer (avoids hard-coding which of the 2
	// remaining reservoir questions the uniform draw happened to pick).
	_, raw := readActionMatching(t, admin, protocol.ActionRafaleAnswer)
	var parsed struct {
		Msg protocol.RafaleAnswerPayload `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("admin: failed to unmarshal RAFALE_ANSWER: %v (raw: %s)", err, raw)
	}
	if parsed.Msg.Next == nil || parsed.Msg.Next.Question == "" {
		t.Fatalf("sanity: expected a non-nil, non-empty NEXT statement to search for, got %+v", parsed.Msg.Next)
	}

	assertRafaleAnswerNeverReceived(t, "tv", tv)
	assertRafaleAnswerNeverReceived(t, "vplayer", vplayer)
	assertNoFrameContainsSecret(t, "tv", tv, parsed.Msg.Next.Question)
	assertNoFrameContainsSecret(t, "vplayer", vplayer, parsed.Msg.Next.Question)
}
