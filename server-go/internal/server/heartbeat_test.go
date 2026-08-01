package server

import (
	"buzzcontrol/internal/protocol"
	"encoding/json"
	"sync"
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

// readHeartbeatPayload unmarshals a HEARTBEAT message's MSG into its INTERVAL_MS field.
func readHeartbeatPayload(t *testing.T, msg *protocol.Message) int {
	t.Helper()
	var payload struct {
		IntervalMs int `json:"INTERVAL_MS"`
	}
	if err := json.Unmarshal(msg.Msg, &payload); err != nil {
		t.Fatalf("failed to unmarshal HEARTBEAT payload: %v (raw: %s)", err, msg.Msg)
	}
	return payload.IntervalMs
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
// carrying the ticker's REAL cadence (3000ms) — not a hardcoded/duplicated
// value that could drift from the actual ticker.
func TestWritePump_EmitsHeartbeatWithCorrectInterval(t *testing.T) {
	srv, _, cleanup := startTestWSServer(t)
	defer cleanup()

	conn := dialWSPath(t, srv, "/ws/player")
	messages, _ := collectMessagesAndPings(t, conn, 3500*time.Millisecond)

	heartbeat := findAction(messages, "HEARTBEAT")
	if heartbeat == nil {
		t.Fatalf("expected at least one HEARTBEAT message within 3.5s, got actions: %v", actionsOf(messages))
	}
	if got := readHeartbeatPayload(t, heartbeat); got != 3000 {
		t.Errorf("expected INTERVAL_MS=3000 (the ticker's actual cadence), got %d", got)
	}
}

// TestWritePump_HeartbeatCoexistsWithProtocolPing verifies the new
// application-level HEARTBEAT does NOT replace the existing protocol-level
// ping frame — non-regression on the PongHandler / 5s read deadline liveness
// mechanism the SERVER already relies on to detect a dead client.
func TestWritePump_HeartbeatCoexistsWithProtocolPing(t *testing.T) {
	srv, _, cleanup := startTestWSServer(t)
	defer cleanup()

	conn := dialWSPath(t, srv, "/ws/player")
	messages, pingReceived := collectMessagesAndPings(t, conn, 3500*time.Millisecond)

	if !pingReceived {
		t.Error("expected a protocol-level ping frame within 3.5s (non-regression: PongHandler/5s read deadline)")
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
			messages, _ := collectMessagesAndPings(t, conn, 3500*time.Millisecond)
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

	time.Sleep(3500 * time.Millisecond)

	select {
	case incoming := <-hub.Incoming:
		t.Errorf("HEARTBEAT (server-authored) must never appear on the Incoming channel, got: %+v", incoming)
	default:
		// Expected: nothing queued from the server's own heartbeat ticker.
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
