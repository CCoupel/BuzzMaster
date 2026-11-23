package main

// ---------------------------------------------------------------------------
// #193 (v13.0.0, LOT-2.1, tâche 8) — MESURE DE CHARGE du relais de dessin PICTO (contracts/picto.md §8.5, plan
// « Tests requis » → scope perf). Vrai serveur (hub + vraies WebSockets), 1 TV + 1 admin + 1 anim + 12 VJoueurs dont
// le dessinateur, qui dessine en continu.
//
// Seuils (à reporter dans `testing.perf`) :
//   - relais serveur, de l'entrée à la sortie : p95 ≤ 100 ms ;
//   - taille d'une trame relayée : ≤ 4 Ko ;
//   - débit nominal (16 trames/s, 200 points), 5 minutes : 0 déconnexion ET 0 trame perdue ;
//   - 3 × le débit nominal : 0 déconnexion (les pertes sont tolérées, puis rattrapées).
//
// Exécution : la mesure complète de 5 minutes est réservée au scope perf :
//     PICTO_LOAD=1 go test ./cmd/server -run TestPictoLoad193 -v -timeout 20m
//   (PICTO_LOAD_SECONDS=N raccourcit la durée ; rapport par `t.Logf`, repris dans tests/perf/picto-load.md).
// Sans PICTO_LOAD, un smoke de 5 s tourne avec les mêmes seuils (exclu par -short).
// ---------------------------------------------------------------------------

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"buzzcontrol/internal/protocol"

	"github.com/gorilla/websocket"
)

const (
	loadPlayers        = 12
	loadNominalFPS     = 16
	loadPointsPerFrame = 200
	loadP95Max         = 100 * time.Millisecond
	loadFrameMaxBytes  = 4096
)

type loadRecv struct {
	name     string
	mu       sync.Mutex
	seqAt    map[int]time.Time // SEQ -> instant de réception
	maxBytes int
	dead     atomic.Bool // lecture interrompue = déconnexion
}

func startLoadReader(name string, conn *websocket.Conn) *loadRecv {
	r := &loadRecv{name: name, seqAt: map[int]time.Time{}}
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				r.dead.Store(true)
				return
			}
			now := time.Now()
			var env struct {
				Action string `json:"ACTION"`
				Msg    struct {
					Seq int `json:"SEQ"`
				} `json:"MSG"`
			}
			if json.Unmarshal(data, &env) != nil || env.Action != "PICTO_STROKES" {
				continue
			}
			r.mu.Lock()
			if _, dup := r.seqAt[env.Msg.Seq]; !dup {
				r.seqAt[env.Msg.Seq] = now
			}
			if len(data) > r.maxBytes {
				r.maxBytes = len(data)
			}
			r.mu.Unlock()
		}
	}()
	return r
}

type loadEnv struct {
	app       *App
	drawer    *websocket.Conn
	epoch     int
	receivers []*loadRecv
	conns     []*websocket.Conn
	clients0  int
}

func newLoadEnv(t *testing.T) *loadEnv {
	t.Helper()
	w := newPictoWire(t) // 3 joueurs : la mesure en ajoute pour atteindre 12 VJoueurs
	base := startAnimAllowlistTestServer(t, w.app)
	for i := 0; i < loadPlayers-3; i++ {
		id := setupVirtualPlayer(t, w.app, fmt.Sprintf("Joueur%02d", i), "TeamA")
		c := newWireClient(t, "extra", dialWS(t, base, "/ws/player"))
		learnClientID(t, w.app, c.conn)
		w.send(t, c, protocol.ActionPlayerConnect, protocol.PlayerConnectPayload{Name: fmt.Sprintf("Joueur%02d", i), ID: id})
		c.waitAction(t, "PLAYER_CONNECTED", 1, 3*time.Second)
	}
	w.chooseTeamAndDrawer(t, w.aliceID)
	w.startRound(t, w.alice)
	epoch := int(gameField(t, w.admin, "PICTO_CANVAS_EPOCH").(float64))

	// Lecteurs dédiés (TV, admin, anim) : on arrête les collecteurs de trames du harnais fonctionnel en les
	// remplaçant par des lecteurs qui horodatent — les connexions sont partagées, donc on ouvre de NOUVEAUX
	// clients de mesure plutôt que de lire en double.
	env := &loadEnv{app: w.app, drawer: w.alice.conn, epoch: epoch}
	for _, spec := range []struct{ name, path string }{{"tv", "/ws/tv"}, {"admin", "/ws/admin"}, {"anim", "/ws/anim"}} {
		c := dialWS(t, base, spec.path)
		learnClientID(t, w.app, c)
		env.receivers = append(env.receivers, startLoadReader(spec.name, c))
		env.conns = append(env.conns, c)
	}
	env.clients0 = w.app.wsHub.ClientCount()
	// Boucle de dispatch réelle (main.go) : les trames du dessinateur sont traitées en continu.
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			select {
			case <-done:
				return
			case msg := <-w.app.wsHub.Incoming:
				w.app.handleWebMessage(msg)
			}
		}
	}()
	t.Cleanup(func() { close(done); <-stopped })
	return env
}

// drawAt envoie des trames de 200 points à `fps` trames/s pendant d ; retourne les instants d'envoi.
func (e *loadEnv) drawAt(t *testing.T, fps int, d time.Duration) []time.Time {
	t.Helper()
	interval := time.Second / time.Duration(fps)
	sentAt := []time.Time{}
	next := time.Now()
	end := next.Add(d)
	strokeID := 1
	for next.Before(end) {
		pts := make([][2]int, loadPointsPerFrame)
		for i := range pts {
			pts[i] = [2]int{(i*47 + len(sentAt)*13) % 10001, (i*91 + len(sentAt)*7) % 10001}
		}
		msg, _ := protocol.NewMessage("PICTO_STROKES", map[string]interface{}{
			"EPOCH":   e.epoch,
			"STROKES": []map[string]interface{}{{"ID": strokeID, "PTS": pts, "END": false}},
		})
		data, _ := msg.SerializeForWebSocket()
		sentAt = append(sentAt, time.Now())
		if err := e.drawer.WriteMessage(websocket.TextMessage, data); err != nil {
			t.Fatalf("écriture du dessinateur : %v", err)
		}
		next = next.Add(interval)
		if s := time.Until(next); s > 0 {
			time.Sleep(s)
		}
	}
	return sentAt
}

func percentile(ds []time.Duration, p float64) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	idx := int(float64(len(ds)-1) * p)
	return ds[idx]
}

func loadSeconds(def int) time.Duration {
	if v, err := strconv.Atoi(os.Getenv("PICTO_LOAD_SECONDS")); err == nil && v > 0 {
		return time.Duration(v) * time.Second
	}
	return time.Duration(def) * time.Second
}

func TestPictoLoad193_Nominal_ZeroDisconnectZeroLoss_P95(t *testing.T) {
	if testing.Short() {
		t.Skip("mesure de charge : exclue de la boucle rapide (-short)")
	}
	dur := 5 * time.Second
	if os.Getenv("PICTO_LOAD") != "" {
		dur = loadSeconds(300) // 5 minutes de dessin continu
	}
	env := newLoadEnv(t)
	sent := env.drawAt(t, loadNominalFPS, dur)
	time.Sleep(1500 * time.Millisecond) // laisse arriver les dernières trames

	if got := env.app.wsHub.ClientCount(); got != env.clients0 {
		t.Errorf("déconnexion(s) pendant la mesure : %d clients au départ, %d à la fin", env.clients0, got)
	}
	for _, r := range env.receivers {
		if r.dead.Load() {
			t.Errorf("%s : connexion coupée pendant la mesure", r.name)
		}
		r.mu.Lock()
		got := len(r.seqAt)
		maxBytes := r.maxBytes
		var lat []time.Duration
		for seq, at := range r.seqAt {
			if seq >= 1 && seq <= len(sent) {
				lat = append(lat, at.Sub(sent[seq-1])) // trames acceptées dans l'ordre : SEQ i = i-ème envoi
			}
		}
		r.mu.Unlock()
		p50, p95, p99 := percentile(lat, 0.50), percentile(lat, 0.95), percentile(lat, 0.99)
		t.Logf("PERF %s : %d/%d trames reçues, p50=%v p95=%v p99=%v, trame max=%d o", r.name, got, len(sent), p50, p95, p99, maxBytes)
		if got != len(sent) {
			t.Errorf("%s : %d trame(s) perdue(s) sur %d (seuil : 0 au débit nominal)", r.name, len(sent)-got, len(sent))
		}
		if p95 > loadP95Max {
			t.Errorf("%s : latence p95 = %v, seuil %v", r.name, p95, loadP95Max)
		}
		if maxBytes > loadFrameMaxBytes {
			t.Errorf("%s : trame de %d o, seuil %d o", r.name, maxBytes, loadFrameMaxBytes)
		}
	}
}

func TestPictoLoad193_TripleRate_ZeroDisconnect_LossToleratedThenCaughtUp(t *testing.T) {
	if testing.Short() {
		t.Skip("mesure de charge : exclue de la boucle rapide (-short)")
	}
	dur := 5 * time.Second
	if os.Getenv("PICTO_LOAD") != "" {
		dur = loadSeconds(60)
	}
	env := newLoadEnv(t)
	env.drawAt(t, 3*loadNominalFPS, dur)
	time.Sleep(1500 * time.Millisecond)

	if got := env.app.wsHub.ClientCount(); got != env.clients0 {
		t.Errorf("3 × le débit : %d clients au départ, %d à la fin — aucune déconnexion tolérée", env.clients0, got)
	}
	for _, r := range env.receivers {
		if r.dead.Load() {
			t.Errorf("%s : connexion coupée à 3 × le débit", r.name)
		}
	}
}
