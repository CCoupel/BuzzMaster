package server

// #246 (v11.2) — diagnostic de diffusion (contracts/logging.md §3.3) : un client dont la file d'envoi
// (256) se remplit est coupé ET journalisé (CLIENT_EVICTED_SLOW : client, type, queue) ; une écriture qui
// dépasse son délai de 3 s est journalisée (CLIENT_WRITE_TIMEOUT) ; une file d'entrée pleine (100) journalise
// INCOMING_DROPPED (client, type, action). Vrai hub, vraies WebSockets, tampon de logs réel.

import (
	"buzzcontrol/internal/protocol"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func newHub246(t *testing.T, ct ClientType) (*WebSocketHub, *BroadcastLogger, string) {
	t.Helper()
	bl := NewBroadcastLogger(5000)
	SetGlobalLogger(bl)
	t.Cleanup(func() { SetGlobalLogger(nil) })
	hub := NewWebSocketHub()
	go hub.Run()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleConnectionWithType(w, r, ct)
	}))
	t.Cleanup(srv.Close)
	return hub, bl, "ws" + strings.TrimPrefix(srv.URL, "http")
}

func logHas246(bl *BroadcastLogger, parts ...string) bool {
	for _, e := range bl.GetHistory() {
		ok := true
		for _, p := range parts {
			if !strings.Contains(e.Message, p) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func waitFor246(d time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return cond()
}

// dialStalled ouvre une connexion qui ne lira JAMAIS (tampon de réception minuscule : la file se remplit vite).
func dialStalled246(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	d := websocket.Dialer{NetDial: func(network, addr string) (net.Conn, error) {
		c, err := net.Dial(network, addr)
		if tc, ok := c.(*net.TCPConn); ok && err == nil {
			_ = tc.SetReadBuffer(4096)
		}
		return c, err
	}}
	c, _, err := d.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestSlowClient246_FullSendQueueEvictsAndIsLogged(t *testing.T) {
	hub, bl, url := newHub246(t, ClientTypeAdmin)
	dialStalled246(t, url)
	if !waitFor246(2*time.Second, func() bool { return hub.ClientCount() == 1 }) {
		t.Fatal("précondition : le client doit être enregistré")
	}
	big, err := protocol.NewMessage("UPDATE", map[string]string{"X": strings.Repeat("x", 64*1024)})
	if err != nil {
		t.Fatal(err)
	}

	evicted := false
	for i := 0; i < 3000 && !evicted; i++ {
		hub.BroadcastToTypes(big, ClientTypeAdmin)
		evicted = logHas246(bl, "CLIENT_EVICTED_SLOW")
	}

	if !evicted {
		t.Fatal("un client qui ne lit plus doit être coupé et journalisé CLIENT_EVICTED_SLOW")
	}
	if !logHas246(bl, "CLIENT_EVICTED_SLOW", "client=", "type=admin", "queue=") {
		t.Errorf("l'événement doit porter client, type et queue ; logs=%v", bl.GetRecent(5))
	}
	if !waitFor246(2*time.Second, func() bool { return hub.ClientCount() == 0 }) {
		t.Errorf("le client lent doit être retiré du hub (%d restant)", hub.ClientCount())
	}
}

func TestSlowClient246_BlockedWriteBeyondItsDeadlineIsLoggedAsWriteTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("attend le délai d'écriture réel de 3 s, exclu de la boucle rapide (-short)")
	}
	hub, bl, url := newHub246(t, ClientTypeAdmin)
	dialStalled246(t, url)
	waitFor246(2*time.Second, func() bool { return hub.ClientCount() == 1 })
	big, _ := protocol.NewMessage("UPDATE", map[string]string{"X": strings.Repeat("x", 64*1024)})

	for i := 0; i < 3000 && !logHas246(bl, "CLIENT_EVICTED_SLOW"); i++ {
		hub.BroadcastToTypes(big, ClientTypeAdmin) // l'écriture bloquée de writePump atteint son délai de 3 s
	}

	if !waitFor246(6*time.Second, func() bool { return logHas246(bl, "CLIENT_WRITE_TIMEOUT", "client=", "type=admin") }) {
		t.Fatalf("une écriture bloquée au-delà de 3 s doit journaliser CLIENT_WRITE_TIMEOUT ; logs=%v", bl.GetRecent(10))
	}
}

func TestSlowClient246_HealthyClientIsNeverEvicted(t *testing.T) {
	hub, bl, url := newHub246(t, ClientTypeAdmin)
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	go func() {
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}()
	waitFor246(2*time.Second, func() bool { return hub.ClientCount() == 1 })
	msg, _ := protocol.NewMessage("UPDATE", map[string]string{"X": strings.Repeat("x", 4096)})

	for i := 0; i < 500; i++ {
		hub.BroadcastToTypes(msg, ClientTypeAdmin)
		time.Sleep(time.Millisecond)
	}

	if logHas246(bl, "CLIENT_EVICTED_SLOW") || hub.ClientCount() != 1 {
		t.Errorf("un client qui lit n'est jamais coupé (clients=%d)", hub.ClientCount())
	}
}

func TestIncomingDropped246_FullInboundQueueIsLoggedWithTheAction(t *testing.T) {
	hub, bl, url := newHub246(t, ClientTypeVPlayer)
	_ = hub // aucun consommateur de hub.Incoming : la file (100) se remplit
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	msg, _ := protocol.NewMessage("PONG", map[string]interface{}{})
	data, _ := msg.SerializeForWebSocket()

	for i := 0; i < 160; i++ {
		if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}

	if !waitFor246(3*time.Second, func() bool { return logHas246(bl, "INCOMING_DROPPED", "client=", "type=vplayer", "action=PONG") }) {
		t.Fatalf("file d'entrée pleine : INCOMING_DROPPED client/type/action attendu ; logs=%v", bl.GetRecent(5))
	}
}
