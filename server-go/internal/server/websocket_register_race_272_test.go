package server

import (
	"buzzcontrol/internal/protocol"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// #272 — course à la connexion WebSocket (test de REPRODUCTION, nature regression).
//
// Bug : HandleConnectionWithType lance `readPump` juste après le rendez-vous sur
// h.register (non bufferisé), mais la boucle Run n'inscrit le client dans
// h.clients (`h.clients[client] = true`) qu'APRÈS ce rendez-vous. Un message
// envoyé par le client dans cette fenêtre (quelques µs, élargie sous charge CI ou
// -race) est lu, mis dans Incoming et traité AVANT que le client soit inscrit :
//   - SetClientPlayerID / BindClientPlayerID ne trouvent personne → le PlayerID
//     n'est jamais lié (pour un VJoueur : PLAYER_CONNECT « perdu ») ;
//   - l'écho ciblé SendRawToPlayerID (filtré sur client.PlayerID) ne vise personne.
// Observé en CI : TestBroadcast129_QCMAnswer_EachPlayerReceivesOnlyOwnEchoZeroFromOthers
// (job 115287415370 ; _work/handoff/dev-backend-20261013-143500.md).
//
// Contrat vérifié (invariant du hub) :
//   « Tout message lu depuis une connexion et remis dans Incoming vient d'un client
//     DÉJÀ inscrit : ClientBinding(msg.ClientID) répond connected=true. »
// Conséquence : un PLAYER_CONNECT envoyé dès l'ouverture de la connexion est toujours
// lié, et l'écho ciblé est toujours livré.
//
// Déterminisme : SetRegisterHook (test-only, websocket.go) fait attendre la boucle Run
// AVANT l'inscription. Le hook attend soit que le message du client ait été traité
// (cas du bug : il l'est pendant la fenêtre), soit l'expiration d'une fenêtre bornée
// (cas corrigé : le message est retenu tant que le client n'est pas inscrit). Aucun
// délai n'est une mesure d'horloge : le RED ne dépend pas de la charge machine, et le
// GREEN ne dépend d'aucune course.
//
// RED attendu avant le correctif ; GREEN après.
// Le hook est global au processus : ces tests ne sont donc pas parallèles.
// Commande : CGO_ENABLED=1 go test -race -count=3 -run RegisterRace272 ./internal/server
// ---------------------------------------------------------------------------

const (
	// Fenêtre pendant laquelle Run retarde l'inscription quand rien n'arrive (cas corrigé).
	registerWindow272 = 250 * time.Millisecond
	// Échéance large des attentes par événement (jamais une durée d'observation).
	eventDeadline272 = 3 * time.Second
	echoMarker272    = "RACE_272_TARGETED_ECHO"
)

// holdRegistration272 retarde l'inscription du prochain client jusqu'à ce que `early`
// soit fermé ou signalé, ou jusqu'à registerWindow272. Hook posé AVANT go hub.Run().
func holdRegistration272(t *testing.T, early <-chan struct{}) {
	t.Helper()
	restore := SetRegisterHook(func() {
		select {
		case <-early:
		case <-time.After(registerWindow272):
		}
	})
	t.Cleanup(restore)
}

// raceServer272 : hub + serveur /ws/player + pompe Incoming qui joue le rôle du
// dispatcher de cmd/server pour PLAYER_CONNECT (BindClientPlayerID puis écho ciblé).
type raceServer272 struct {
	srv *httptest.Server
	hub *WebSocketHub

	mu                sync.Mutex
	registeredAtRead  map[string]bool // clientID → inscrit au moment où son message est sorti d'Incoming
	actions           []string        // actions reçues, dans l'ordre d'arrivée dans Incoming
	processedFirstMsg chan struct{}   // signalé après le traitement du 1er message
	once              sync.Once
}

func startRaceServer272(t *testing.T, playerID string) *raceServer272 {
	t.Helper()
	hub := NewWebSocketHub()
	rs := &raceServer272{
		hub:               hub,
		registeredAtRead:  map[string]bool{},
		processedFirstMsg: make(chan struct{}),
	}
	go hub.Run()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleConnectionWithType(w, r, ClientTypeVPlayer)
	}))
	rs.srv = srv

	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			case in := <-hub.Incoming:
				_, registered := hub.ClientBinding(in.ClientID)
				rs.mu.Lock()
				if _, seen := rs.registeredAtRead[in.ClientID]; !seen {
					rs.registeredAtRead[in.ClientID] = registered
				}
				rs.actions = append(rs.actions, in.Data.Action)
				rs.mu.Unlock()

				if in.Data.Action == protocol.ActionPlayerConnect {
					// Même ordre que handlePlayerConnect (cmd/server/main.go) : liaison, puis écho ciblé.
					hub.BindClientPlayerID(in.ClientID, playerID)
					hub.SendRawToPlayerID(playerID, []byte(`{"ACTION":"`+echoMarker272+`"}`))
				}
				rs.once.Do(func() { close(rs.processedFirstMsg) })
			}
		}
	}()

	t.Cleanup(func() {
		close(stop)
		srv.Close()
	})
	return rs
}

func (rs *raceServer272) dial(t *testing.T) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(rs.srv.URL, "http")+"/ws/player", nil)
	if err != nil {
		t.Fatalf("connexion WebSocket : %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func (rs *raceServer272) wasRegisteredAtRead(clientID string) (registered, seen bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	registered, seen = rs.registeredAtRead[clientID]
	return
}

func (rs *raceServer272) receivedActions() []string {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return append([]string(nil), rs.actions...)
}

func send272(t *testing.T, conn *websocket.Conn, action string, payload interface{}) {
	t.Helper()
	msg, err := protocol.NewMessage(action, payload)
	if err != nil {
		t.Fatalf("construction du message %s : %v", action, err)
	}
	data, err := msg.SerializeForWebSocket()
	if err != nil {
		t.Fatalf("sérialisation de %s : %v", action, err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("envoi de %s : %v", action, err)
	}
}

func eventually272(t *testing.T, what string, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(eventDeadline272)
	for !cond() {
		if time.Now().After(deadline) {
			t.Errorf("%s : non atteint dans %v", what, eventDeadline272)
			return false
		}
		time.Sleep(2 * time.Millisecond)
	}
	return true
}

// readUntilMarker272 lit les trames du client (heartbeats compris) jusqu'à celle de l'écho ciblé.
func readUntilMarker272(conn *websocket.Conn, marker string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn.SetReadDeadline(deadline)
		_, data, err := conn.ReadMessage()
		if err != nil {
			return false
		}
		if strings.Contains(string(data), marker) {
			return true
		}
	}
	return false
}

// --- Test principal : PLAYER_CONNECT envoyé dès l'ouverture -----------------------------

func TestRegisterRace272_PlayerConnectSentImmediatelyIsBoundAndEchoed(t *testing.T) {
	const playerID = "vjoueur_Race272"
	rs := startRaceServer272(t, playerID)
	holdRegistration272(t, rs.processedFirstMsg)

	conn := rs.dial(t)
	clientID := conn.LocalAddr().String() // ID serveur du client = r.RemoteAddr
	send272(t, conn, protocol.ActionPlayerConnect, protocol.PlayerConnectPayload{Name: "Alice"})

	// Invariant : le message n'est lu qu'une fois le client inscrit.
	if !eventually272(t, "lecture du PLAYER_CONNECT par le hub", func() bool {
		_, seen := rs.wasRegisteredAtRead(clientID)
		return seen
	}) {
		return
	}
	if registered, _ := rs.wasRegisteredAtRead(clientID); !registered {
		t.Errorf("PLAYER_CONNECT sorti d'Incoming alors que le client %s n'est PAS inscrit dans h.clients "+
			"(readPump lancé avant l'inscription) : liaison et écho ciblé seront perdus", clientID)
	}

	// Conséquence 1 : le PlayerID est lié à la connexion.
	eventually272(t, "ClientBinding("+clientID+") lié à "+playerID, func() bool {
		bound, connected := rs.hub.ClientBinding(clientID)
		return connected && bound == playerID
	})
	if !rs.hub.IsPlayerIDConnected(playerID) {
		t.Errorf("IsPlayerIDConnected(%s) = false : le joueur n'est pas considéré connecté", playerID)
	}

	// Conséquence 2 : l'écho ciblé parvient au client.
	if !readUntilMarker272(conn, echoMarker272, eventDeadline272) {
		t.Errorf("l'écho ciblé SendRawToPlayerID(%s) n'est jamais parvenu au client", playerID)
	}
}

// --- Aucun message perdu ni réordonné pendant la fenêtre ---------------------------------

func TestRegisterRace272_MessagesSentBeforeRegistrationAreDeliveredInOrder(t *testing.T) {
	rs := startRaceServer272(t, "vjoueur_Order272")
	// Aucun signal « early » : la fenêtre expire (cas corrigé) ou est levée par le 1er message (cas du bug).
	holdRegistration272(t, rs.processedFirstMsg)

	conn := rs.dial(t)
	clientID := conn.LocalAddr().String()
	send272(t, conn, protocol.ActionPlayerConnect, protocol.PlayerConnectPayload{Name: "Bob"})
	send272(t, conn, "PONG", map[string]interface{}{})
	send272(t, conn, "PONG", map[string]interface{}{})

	eventually272(t, "3 messages reçus par le hub", func() bool { return len(rs.receivedActions()) >= 3 })

	got := rs.receivedActions()
	want := []string{protocol.ActionPlayerConnect, "PONG", "PONG"}
	if len(got) < len(want) {
		t.Fatalf("messages reçus %v, attendu au moins %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ordre des messages %v, attendu %v : un message a été perdu ou réordonné", got, want)
			break
		}
	}
	if registered, _ := rs.wasRegisteredAtRead(clientID); !registered {
		t.Errorf("le 1er message du client %s est sorti d'Incoming avant son inscription", clientID)
	}
}

// --- Le délai d'inscription ne bloque pas la connexion -------------------------------------

func TestRegisterRace272_ClientEventuallyRegisteredAndUsableAfterDelayedRegistration(t *testing.T) {
	rs := startRaceServer272(t, "vjoueur_Idle272")
	never := make(chan struct{}) // jamais signalé : la fenêtre expire
	holdRegistration272(t, never)

	conn := rs.dial(t)
	clientID := conn.LocalAddr().String()
	// Le client n'envoie rien : il doit quand même finir inscrit, sans interblocage.
	eventually272(t, "inscription du client inactif "+clientID, func() bool {
		_, connected := rs.hub.ClientBinding(clientID)
		return connected
	})

	// Et rester utilisable : un message envoyé APRÈS l'inscription est lié normalement.
	send272(t, conn, protocol.ActionPlayerConnect, protocol.PlayerConnectPayload{Name: "Carol"})
	eventually272(t, "liaison après inscription retardée", func() bool {
		bound, connected := rs.hub.ClientBinding(clientID)
		return connected && bound == "vjoueur_Idle272"
	})
	if !readUntilMarker272(conn, echoMarker272, eventDeadline272) {
		t.Errorf("écho ciblé non reçu après une inscription retardée")
	}
}

// --- Sans hook : l'invariant tient aussi sous connexions simultanées ------------------------
// Garde probabiliste (la fenêtre naturelle est de quelques µs) ; le test déterministe est le premier.

func TestRegisterRace272_InvariantHoldsUnderConcurrentConnections(t *testing.T) {
	rs := startRaceServer272(t, "vjoueur_Stress272") // tous liés au même PlayerID : seul l'invariant d'inscription compte ici

	const clients = 40
	ids := make([]string, clients)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < clients; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(rs.srv.URL, "http")+"/ws/player", nil)
			if err != nil {
				t.Errorf("connexion %d : %v", i, err)
				return
			}
			t.Cleanup(func() { conn.Close() })
			ids[i] = conn.LocalAddr().String()
			msg, _ := protocol.NewMessage("PONG", map[string]interface{}{})
			data, _ := msg.SerializeForWebSocket()
			_ = conn.WriteMessage(websocket.TextMessage, data)
		}()
	}
	close(start)
	wg.Wait()

	for _, id := range ids {
		if id == "" {
			continue
		}
		id := id
		eventually272(t, "lecture du 1er message de "+id, func() bool {
			_, seen := rs.wasRegisteredAtRead(id)
			return seen
		})
		if registered, seen := rs.wasRegisteredAtRead(id); seen && !registered {
			t.Errorf("message de %s lu avant son inscription dans le hub", id)
		}
	}
}
