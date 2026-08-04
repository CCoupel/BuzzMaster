package server

import (
	"buzzcontrol/internal/protocol"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// Tests : HEARTBEAT applicatif (#118, B1)
//
// Plan : _work/reports/plan-20260729-190000.md — tâche B1.
// Cause racine (_work/reports/plan-analysis-20260729-190000-118-findings.md) :
// le serveur détecte déjà une liaison morte (ping/pong protocolaire + read
// deadline 5s côté serveur), mais le CLIENT n'a aucune preuve de vie
// exploitable — le navigateur répond aux trames ping au niveau protocole sans
// jamais exposer d'événement JavaScript pour elles. Le correctif ajoute, sur
// le ticker déjà présent dans writePump (3s, qui émet déjà la trame ping),
// un message applicatif HEARTBEAT{INTERVAL_MS} que le client PEUT observer.
//
// Ces tests attendent un tick réel (~3.5s) — même convention que les autres
// tests de ce paquet exerçant des délais réels (ex. websocket_buzzer_test.go).
// ---------------------------------------------------------------------------

// readHeartbeatPayload unmarshals a HEARTBEAT message's MSG into its
// INTERVAL_MS and DEAD_LINK_TIMEOUT_MS fields (#130).
func readHeartbeatPayload(t *testing.T, msg *protocol.Message) (intervalMs, deadLinkTimeoutMs int) {
	t.Helper()
	var payload struct {
		IntervalMs        int `json:"INTERVAL_MS"`
		DeadLinkTimeoutMs int `json:"DEAD_LINK_TIMEOUT_MS"`
	}
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		t.Fatalf("failed to unmarshal HEARTBEAT payload: %v (raw: %s)", err, msg.Msg)
	}
	return payload.IntervalMs, payload.DeadLinkTimeoutMs
}

// collectMessagesAndPings runs a background read loop on conn for `window`,
// recording every parsed application message (HEARTBEAT, UPDATE, ...) AND
// whether at least one protocol-level ping frame was received (via
// SetPingHandler — control frames only surface during a call to
// ReadMessage, hence the need for a continuous loop rather than a single
// blocking read).
func collectMessagesAndPings(t *testing.T, conn *websocket.Conn, window time.Duration) (messages []*protocol.Message, pingReceived bool) {
	t.Helper()
	var mu sync.Mutex

	conn.SetPingHandler(func(appData string) error {
		mu.Lock()
		pingReceived = true
		mu.Unlock()
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			msg, parseErr := protocol.ParseSingle(data)
			if parseErr != nil {
				continue
			}
			mu.Lock()
			messages = append(messages, msg)
			mu.Unlock()
		}
	}()

	time.Sleep(window)
	conn.Close()
	<-done

	mu.Lock()
	defer mu.Unlock()
	return messages, pingReceived
}

func findAction(messages []*protocol.Message, action string) *protocol.Message {
	for _, m := range messages {
		if m.Action == action {
			return m
		}
	}
	return nil
}

// TestWritePump_EmitsHeartbeatWithCorrectInterval is the core B1 test: a
// HEARTBEAT{INTERVAL_MS} message must arrive within one ticker period,
// carrying the ticker's REAL cadence — read from writePumpTickPeriod itself
// (#130: 2000ms, down from 3000ms), never a hardcoded/duplicated literal
// that could drift from the actual ticker.
func TestWritePump_EmitsHeartbeatWithCorrectInterval(t *testing.T) {
	srv, _, cleanup := startTestWSServer(t)
	defer cleanup()

	conn := dialWSPath(t, srv, "/ws/player")
	messages, _ := collectMessagesAndPings(t, conn, 2500*time.Millisecond)

	heartbeat := findAction(messages, "HEARTBEAT")
	if heartbeat == nil {
		t.Fatalf("expected at least one HEARTBEAT message within 2.5s, got actions: %v", actionsOf(messages))
	}
	intervalMs, _ := readHeartbeatPayload(t, heartbeat)
	if want := int(writePumpTickPeriod.Milliseconds()); intervalMs != want {
		t.Errorf("expected INTERVAL_MS=%d (writePumpTickPeriod, the ticker's actual cadence), got %d", want, intervalMs)
	}
}

// TestWritePump_HeartbeatCarriesDeadLinkTimeout is the #130 CA4 test: the
// HEARTBEAT payload must also carry DEAD_LINK_TIMEOUT_MS, read from
// deadLinkTimeout itself (not a duplicated literal) — the GATE-2-adjusted
// value of 4000ms (contract's own recommendation was 5000ms; the user chose
// the more reactive variant, see the constant's doc comment).
func TestWritePump_HeartbeatCarriesDeadLinkTimeout(t *testing.T) {
	srv, _, cleanup := startTestWSServer(t)
	defer cleanup()

	conn := dialWSPath(t, srv, "/ws/player")
	messages, _ := collectMessagesAndPings(t, conn, 2500*time.Millisecond)

	heartbeat := findAction(messages, "HEARTBEAT")
	if heartbeat == nil {
		t.Fatalf("expected at least one HEARTBEAT message within 2.5s, got actions: %v", actionsOf(messages))
	}
	_, deadLinkTimeoutMs := readHeartbeatPayload(t, heartbeat)
	if want := int(deadLinkTimeout.Milliseconds()); deadLinkTimeoutMs != want {
		t.Errorf("expected DEAD_LINK_TIMEOUT_MS=%d (deadLinkTimeout), got %d", want, deadLinkTimeoutMs)
	}
}

// TestDeadLinkTimeout_LessThanReadDeadlineTimeout locks in the load-bearing
// invariant documented on both constants (websocket.go): the client must be
// positioned to detect and reconnect BEFORE the server gives up on the same
// dead link — a deliberate inversion from the pre-#130 order (contract §4).
// A future change to either constant that collapses or reverses this
// inequality must fail this test rather than pass silently.
func TestDeadLinkTimeout_LessThanReadDeadlineTimeout(t *testing.T) {
	if !(deadLinkTimeout < readDeadlineTimeout) {
		t.Fatalf("invariant violated: deadLinkTimeout (%s) must be strictly less than readDeadlineTimeout (%s)", deadLinkTimeout, readDeadlineTimeout)
	}
}

// TestWritePump_HeartbeatCoexistsWithProtocolPing verifies the new
// application-level HEARTBEAT does NOT replace the existing protocol-level
// ping frame — non-regression on the PongHandler / readDeadlineTimeout (#130:
// 7s, was 5s) liveness mechanism the SERVER already relies on to detect a
// dead client.
func TestWritePump_HeartbeatCoexistsWithProtocolPing(t *testing.T) {
	srv, _, cleanup := startTestWSServer(t)
	defer cleanup()

	conn := dialWSPath(t, srv, "/ws/player")
	messages, pingReceived := collectMessagesAndPings(t, conn, 2500*time.Millisecond)

	if !pingReceived {
		t.Error("expected a protocol-level ping frame within 2.5s (non-regression: PongHandler/readDeadlineTimeout)")
	}
	if findAction(messages, "HEARTBEAT") == nil {
		t.Errorf("expected a HEARTBEAT message alongside the protocol ping, got actions: %v", actionsOf(messages))
	}
}

// TestWritePump_HeartbeatDeliveredToAllWebClientTypes verifies the heartbeat
// reaches admin, TV and VPlayer alike — it is written unconditionally on
// writePump's shared ticker, common to all three (per the plan: "les trois
// pages bénéficient de la détection").
func TestWritePump_HeartbeatDeliveredToAllWebClientTypes(t *testing.T) {
	srv, _, cleanup := startTestWSServer(t)
	defer cleanup()

	for _, path := range []string{"/ws/admin", "/ws/tv", "/ws/player"} {
		t.Run(path, func(t *testing.T) {
			conn := dialWSPath(t, srv, path)
			messages, _ := collectMessagesAndPings(t, conn, 2500*time.Millisecond)
			if findAction(messages, "HEARTBEAT") == nil {
				t.Errorf("expected HEARTBEAT on %s, got actions: %v", path, actionsOf(messages))
			}
		})
	}
}

// TestWritePump_HeartbeatNeverEntersIncomingChannel verifies the heartbeat is
// purely server-authored (written directly in writePump) and never flows
// through the Incoming channel — the single-consumer pipeline that also
// drives ConfirmDelivery/the #109 connection-badge green window. A client
// never sends HEARTBEAT, so this also guarantees no spurious
// ConfirmDelivery/broadcastUpdate is triggered by it.
func TestWritePump_HeartbeatNeverEntersIncomingChannel(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	conn := dialWSPath(t, srv, "/ws/player")
	defer conn.Close()
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	time.Sleep(2500 * time.Millisecond)

	select {
	case incoming := <-hub.Incoming:
		t.Errorf("HEARTBEAT (server-authored) must never appear on the Incoming channel, got: %+v", incoming)
	default:
		// Expected: nothing queued from the server's own heartbeat ticker.
	}
}

// ---------------------------------------------------------------------------
// CA3 (#130) — tolerance to fully-lost pings.
//
// These simulate a lost ping/pong round trip by overriding the CLIENT's own
// PingHandler to swallow the frame instead of replying — from the SERVER's
// point of view, a client that never sends a Pong is indistinguishable from
// one whose Pong was lost in transit, so this exercises exactly the
// server-side detection logic contracts/liveness-timing.md §4 specifies.
// Real-time waits, same convention as the rest of this file and
// websocket_buzzer_test.go.
// ---------------------------------------------------------------------------

// TestReadDeadline_TwoFullyLostPings_ConnectionSurvives is the positive half
// of CA3: at writePumpTickPeriod=2s, two ENTIRELY lost ping/pong round trips
// (~4s of silence) must NOT close the connection — readDeadlineTimeout (7s)
// has real margin above that now, unlike the pre-#130 configuration where a
// single lost ping (3s) already exceeded the 5s deadline.
func TestReadDeadline_TwoFullyLostPings_ConnectionSurvives(t *testing.T) {
	srv, _, cleanup := startTestWSServer(t)
	defer cleanup()

	conn := dialWSPath(t, srv, "/ws/player")
	defer conn.Close()

	var suppress atomic.Bool
	suppress.Store(true)
	conn.SetPingHandler(func(appData string) error {
		if suppress.Load() {
			return nil // swallow: no Pong sent back — simulates a fully lost round trip
		}
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
	})

	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	// Two fully-lost ping cycles (~4s) plus a small margin, well inside the
	// 7s readDeadlineTimeout — then resume answering normally.
	time.Sleep(4300 * time.Millisecond)
	suppress.Store(false)

	// Give the connection a moment to prove it's still alive rather than
	// having been silently closed already.
	time.Sleep(500 * time.Millisecond)

	select {
	case <-closed:
		t.Fatal("CA3: connection closed after only 2 fully-lost pings — expected it to survive (readDeadlineTimeout=7s tolerates this)")
	default:
		// Still alive, as required.
	}
}

// TestReadDeadline_ThreeFullyLostPings_ConnectionCloses is the negative half
// of CA3: never answering ANY ping must still eventually close the
// connection once readDeadlineTimeout (7s) genuinely elapses — the fix
// widens tolerance, it does not remove dead-link detection altogether.
func TestReadDeadline_ThreeFullyLostPings_ConnectionCloses(t *testing.T) {
	srv, _, cleanup := startTestWSServer(t)
	defer cleanup()

	conn := dialWSPath(t, srv, "/ws/player")
	defer conn.Close()
	conn.SetPingHandler(func(string) error { return nil }) // never pong back

	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	select {
	case <-closed:
		// Expected: the server closed the connection once readDeadlineTimeout elapsed.
	case <-time.After(8500 * time.Millisecond):
		t.Fatal("CA3: expected the server to close the connection after ~7s of no Pong (3+ lost pings), it did not")
	}
}

// TestBuzzerHub_NeverEmitsHeartbeat is the non-regression counterpart for
// physical buzzers: BuzzerWebSocketHub has its own, separate writePump
// (websocket_buzzer.go) untouched by B1 — a physical buzzer's ping/pong
// mechanism must remain exactly as before, with no HEARTBEAT text message
// introduced on its connection.
func TestBuzzerHub_NeverEmitsHeartbeat(t *testing.T) {
	srv, _, cleanup := startTestBuzzerWSServer(t)
	defer cleanup()

	conn := dialBuzzerWS(t, srv)
	messages, pingReceived := collectMessagesAndPings(t, conn, 3500*time.Millisecond)

	if !pingReceived {
		t.Error("expected the buzzer hub's own protocol-level ping frame, unchanged by #118")
	}
	if hb := findAction(messages, "HEARTBEAT"); hb != nil {
		t.Errorf("physical buzzer connections must never receive HEARTBEAT, got: %+v", hb)
	}
}

func actionsOf(messages []*protocol.Message) []string {
	actions := make([]string, len(messages))
	for i, m := range messages {
		actions[i] = m.Action
	}
	return actions
}
