package main

// ---------------------------------------------------------------------------
// #193 (v13.0.0, LOT-2.1, tâche 8) — MESURE DE CHARGE du relais de dessin PICTO (contracts/picto.md §8.5, plan
// « Tests requis » → scope perf). Vrai serveur (hub + vraies WebSockets), 1 TV + 1 admin + 1 anim + 12 VJoueurs dont
// le dessinateur, qui dessine en continu.
//
// Seuils (à reporter dans `testing.perf`) :
//   - relais serveur, de l'entrée à la sortie : p95 ≤ 100 ms ;
//   - taille d'une trame relayée : ≤ 4 Ko ;
//   - nominal : 16 trames/s × 8 points × 280 s (35 840 points ; manche de 300 s au maximum, §4) : 0 déconnexion ET 0 trame perdue ;
//   - trame maximale : 16 trames/s × 200 points × 15 s (48 000 points) : 0 perte, trame ≤ 4 096 o ;
//   - 3 × le débit : 48 trames/s × 200 points × 60 s : 0 déconnexion ; pertes et CANVAS_FULL tolérés (comptés).
//   Garde de cohérence : le nominal et la trame maximale restent SOUS la borne de 60 000 points par époque (§8.3).
//   Arbitrage : _work/reports/plan-arbitrage-perf-picto-20261123.md ; seuils : testing.perf.picto_relay.
//
// Exécution : la mesure complète de 5 minutes est réservée au scope perf :
//     PICTO_LOAD=1 go test ./cmd/server -run TestPictoLoad193 -v -timeout 20m
//   (PICTO_LOAD_SECONDS=N raccourcit la durée ; rapport par `t.Logf`, repris dans tests/perf/picto-load.md).
// Sans PICTO_LOAD, un smoke de 5 s tourne avec les mêmes seuils (exclu par -short).
// ---------------------------------------------------------------------------

import (
	"buzzcontrol/internal/game"
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
	loadNominalPoints  = 8
	loadMaxFramePoints = 200
	loadCanvasMax      = 60000
	loadRoundSeconds   = 300 // DELAY du START = durée de la manche de la fixture (la question reste à 90 s : TIME n'est pas la durée du chrono)
	loadRoundMargin    = 15  // marge entre la fin du dessin et la fin de la manche (le chrono démarre au START)
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
	drawerC   *wireClient
	epoch     int
	receivers []*loadRecv
	conns     []*websocket.Conn
	clients0  int
}

func newLoadEnv(t *testing.T) *loadEnv {
	t.Helper()
	w := newPictoWire(t) // 3 joueurs : la mesure en ajoute pour atteindre 12 VJoueurs
	base := startAnimAllowlistTestServer(t, w.app)
	prevPhase := w.app.engine.GetState().Phase
	w.app.engine.SetPhase(game.PhaseEnroll) // les inscriptions ne sont ouvertes qu'en ENROLL
	for i := 0; i < loadPlayers-3; i++ {
		id := setupVirtualPlayer(t, w.app, fmt.Sprintf("Joueur%02d", i), "TeamA")
		c := newWireClient(t, "extra", dialWS(t, base, "/ws/player"))
		learnClientID(t, w.app, c.conn)
		w.send(t, c, protocol.ActionPlayerConnect, protocol.PlayerConnectPayload{Name: fmt.Sprintf("Joueur%02d", i), ID: id})
		c.waitAction(t, "PLAYER_CONNECTED", 1, 3*time.Second)
	}
	w.app.engine.SetPhase(prevPhase)
	w.chooseTeamAndDrawer(t, w.aliceID)
	// La durée de la manche est le DELAY du START (main.go handleStart : 0 = défaut serveur, 30 s), pas question.Time :
	// elle doit dépasser le dessin, sinon le chronomètre arrête la manche et les trames sont refusées hors STARTED.
	w.startRoundWithDelay(t, w.alice, loadRoundSeconds)
	if got := w.app.engine.GetState().Delay; got != loadRoundSeconds {
		t.Fatalf("durée de la manche = %d s, attendu %d s (DELAY du START)", got, loadRoundSeconds)
	}
	epoch := int(gameField(t, w.admin, "PICTO_CANVAS_EPOCH").(float64))

	// Lecteurs dédiés (TV, admin, anim) : on arrête les collecteurs de trames du harnais fonctionnel en les
	// remplaçant par des lecteurs qui horodatent — les connexions sont partagées, donc on ouvre de NOUVEAUX
	// clients de mesure plutôt que de lire en double.
	env := &loadEnv{app: w.app, drawer: w.alice.conn, drawerC: w.alice, epoch: epoch}
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

// drawAt envoie des trames de `pointsPerFrame` points à `fps` trames/s pendant d ; retourne les instants d'envoi.
func (e *loadEnv) drawAt(t *testing.T, fps, pointsPerFrame int, d time.Duration) []time.Time {
	t.Helper()
	interval := time.Second / time.Duration(fps)
	sentAt := []time.Time{}
	next := time.Now()
	end := next.Add(d)
	strokeID := 1
	for next.Before(end) {
		pts := make([][2]int, pointsPerFrame)
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

// loadScenario décrit un scénario du plan d'arbitrage (testing.perf.picto_relay).
type loadScenario struct {
	name             string
	fps, points      int
	fullSeconds      int  // durée de la mesure complète (PICTO_LOAD=1)
	smokeSeconds     int  // durée du smoke (sans PICTO_LOAD)
	zeroLoss         bool // nominal et trame maximale : 0 perte, p95 et taille contrôlés
	underCanvasBound bool // garde : fps × points × durée ≤ 60 000 (§8.3)
}

func (sc loadScenario) duration() time.Duration {
	if os.Getenv("PICTO_LOAD") != "" {
		return loadSeconds(sc.fullSeconds)
	}
	return time.Duration(sc.smokeSeconds) * time.Second
}

func runLoadScenario(t *testing.T, sc loadScenario) {
	t.Helper()
	if testing.Short() {
		t.Skip("mesure de charge : exclue de la boucle rapide (-short)")
	}
	dur := sc.duration()
	if sc.underCanvasBound {
		if total := sc.fps * sc.points * int(dur.Seconds()); total > loadCanvasMax {
			t.Fatalf("scénario au-delà de la borne : %d trames/s × %d points × %.0f s = %d points > %d par époque (§8.3) — la mesure déclencherait CANVAS_FULL", sc.fps, sc.points, dur.Seconds(), total, loadCanvasMax)
		}
	}
	if dur+loadRoundMargin*time.Second > loadRoundSeconds*time.Second {
		t.Fatalf("scénario plus long que la manche : %.0f s de dessin + %d s de marge > %d s (DELAY du START) — la manche s'arrêterait avant la fin de la mesure", dur.Seconds(), loadRoundMargin, loadRoundSeconds)
	}
	env := newLoadEnv(t)
	sent := env.drawAt(t, sc.fps, sc.points, dur)
	if ph := env.app.engine.GetState().Phase; ph != game.PhaseStarted {
		t.Errorf("%s : la manche doit être encore STARTED à la fin du dessin, phase = %s (trames refusées hors STARTED)", sc.name, ph)
	}
	time.Sleep(1500 * time.Millisecond) // laisse arriver les dernières trames

	if got := env.app.wsHub.ClientCount(); got != env.clients0 {
		t.Errorf("%s : déconnexion(s) pendant la mesure : %d clients au départ, %d à la fin", sc.name, env.clients0, got)
	}
	var canvasFull, rateLimited int
	for _, f := range env.drawerC.of(pictoActError) {
		switch errorCode(f) {
		case "CANVAS_FULL":
			canvasFull++
		case "RATE_LIMITED":
			rateLimited++
		}
	}
	t.Logf("PERF %s : %d trames/s × %d points × %.0f s, %d trames envoyées, CANVAS_FULL=%d, RATE_LIMITED=%d",
		sc.name, sc.fps, sc.points, dur.Seconds(), len(sent), canvasFull, rateLimited)
	for _, r := range env.receivers {
		if r.dead.Load() {
			t.Errorf("%s/%s : connexion coupée pendant la mesure", sc.name, r.name)
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
		t.Logf("PERF %s/%s : %d/%d trames reçues, p50=%v p95=%v p99=%v, trame max=%d o", sc.name, r.name, got, len(sent), p50, p95, p99, maxBytes)
		if !sc.zeroLoss {
			continue // ×3 : pertes et CANVAS_FULL tolérés (comptés ci-dessus)
		}
		if got != len(sent) {
			t.Errorf("%s/%s : %d trame(s) perdue(s) sur %d (seuil : 0)", sc.name, r.name, len(sent)-got, len(sent))
		}
		if p95 > loadP95Max {
			t.Errorf("%s/%s : latence p95 = %v, seuil %v", sc.name, r.name, p95, loadP95Max)
		}
		if maxBytes > loadFrameMaxBytes {
			t.Errorf("%s/%s : trame de %d o, seuil %d o", sc.name, r.name, maxBytes, loadFrameMaxBytes)
		}
	}
	if sc.zeroLoss && (canvasFull > 0 || rateLimited > 0) {
		t.Errorf("%s : CANVAS_FULL=%d / RATE_LIMITED=%d inattendus (le scénario reste sous les bornes)", sc.name, canvasFull, rateLimited)
	}
	// Rattrapage : la feuille du journal ne dépasse jamais la borne, même à 3 × le débit.
	raw, _ := json.Marshal(env.app.pictoCanvas.Snapshot())
	var snap struct {
		Strokes []struct {
			Pts [][2]int `json:"PTS"`
		} `json:"STROKES"`
	}
	_ = json.Unmarshal(raw, &snap)
	points := 0
	for _, s := range snap.Strokes {
		points += len(s.Pts)
	}
	if points > loadCanvasMax {
		t.Errorf("%s : le journal contient %d points, borne %d", sc.name, points, loadCanvasMax)
	}
}

// Nominal : 16 trames/s × 8 points × 280 s = 35 840 points (< 60 000) ; 280 s et non 300 s : la manche dure au plus
// 300 s (DELAY du START) et le dessin ne peut pas occuper toute la manche.
func TestPictoLoad193_Nominal_ZeroDisconnectZeroLoss_P95(t *testing.T) {
	runLoadScenario(t, loadScenario{name: "nominal", fps: loadNominalFPS, points: loadNominalPoints,
		fullSeconds: 280, smokeSeconds: 5, zeroLoss: true, underCanvasBound: true})
}

// Trame maximale : 16 trames/s × 200 points × 15 s = 48 000 points (< 60 000), trame ≤ 4 096 o.
func TestPictoLoad193_MaxFrame_ZeroLoss_SizeAndP95(t *testing.T) {
	runLoadScenario(t, loadScenario{name: "max_frame", fps: loadNominalFPS, points: loadMaxFramePoints,
		fullSeconds: 15, smokeSeconds: 5, zeroLoss: true, underCanvasBound: true})
}

// 3 × le débit : 48 trames/s × 200 points × 60 s — 0 déconnexion ; pertes, RATE_LIMITED et CANVAS_FULL tolérés.
func TestPictoLoad193_TripleRate_ZeroDisconnect_LossToleratedThenCaughtUp(t *testing.T) {
	runLoadScenario(t, loadScenario{name: "x3", fps: 3 * loadNominalFPS, points: loadMaxFramePoints,
		fullSeconds: 60, smokeSeconds: 5})
}
