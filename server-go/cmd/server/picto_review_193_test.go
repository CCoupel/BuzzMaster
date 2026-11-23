package main

// #193 (v13.0.0, LOT-2.1) — corrections de revue (code-review-20261123-171000 M1/M3/M4, audit m2), d'après
// contracts/picto.md §8.3 (journal borné, CANVAS_FULL, vidé à NEW_GAME), §8.5 (PICTO_CANVAS_SYNC), §9.1 (émetteurs :
// tv, admin, anim, vplayer DESSINATEUR seulement). Vraie WebSocket, harnais de picto_wire_192_193_test.go.

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"buzzcontrol/internal/protocol"
)

func canvasStrokeCount(t *testing.T, frame string) (strokes int, points int) {
	t.Helper()
	var p struct {
		Msg struct {
			Strokes []struct {
				Pts [][2]int `json:"PTS"`
			} `json:"STROKES"`
		} `json:"MSG"`
	}
	if err := json.Unmarshal([]byte(frame), &p); err != nil {
		t.Fatal(err)
	}
	for _, s := range p.Msg.Strokes {
		points += len(s.Pts)
	}
	return len(p.Msg.Strokes), points
}

// §8.3 : au-delà des bornes (60 000 points par époque), PICTO_STROKES est refusé (CANVAS_FULL) ; la feuille reste intacte.
func TestPictoWire193_CanvasFull_RefusedExplicitly_SheetIntact(t *testing.T) {
	if testing.Short() {
		t.Skip("remplit la feuille (~300 trames cadencées sous la limite de débit, ≈ 10 s) : exclu de -short")
	}
	w := newPictoWire(t)
	w.chooseTeamAndDrawer(t, w.aliceID)
	w.startRound(t, w.alice)
	ep := int(gameField(t, w.admin, "PICTO_CANVAS_EPOCH").(float64))

	pts := make([][2]int, 200)
	for i := range pts {
		pts[i] = [2]int{i * 40, i * 40}
	}
	full := false
	accepted := 0
	for i := 0; i < 400 && !full; i++ { // 300 trames de 200 points = 60 000 points = la borne
		before := len(w.alice.of(pictoActError))
		w.send(t, w.alice, pictoActStrokes, map[string]interface{}{
			"EPOCH":   ep,
			"STROKES": []map[string]interface{}{{"ID": i + 1, "PTS": pts, "END": true}},
		})
		time.Sleep(28 * time.Millisecond) // ≈ 35 trames/s : sous la limite de 40/s (§8.6)
		for _, f := range w.alice.of(pictoActError)[before:] {
			switch errorCode(f) {
			case "CANVAS_FULL":
				full = true
			case "RATE_LIMITED":
				time.Sleep(1100 * time.Millisecond) // on repart plus lentement : cette trame n'a pas été acceptée
				i--
			}
		}
		if !full {
			accepted = i + 1
		}
	}
	if !full {
		t.Fatalf("après 400 trames de 200 points, PICTO_ERROR CANVAS_FULL attendu (borne 60 000 points, §8.3)")
	}
	if accepted < 250 || accepted > 301 {
		t.Errorf("la borne de 60 000 points doit être atteinte vers la 300e trame, atteinte après %d trames acceptées", accepted)
	}
	// Une trame de plus : refusée explicitement, jamais relayée.
	relayed := len(w.tv.of(pictoActStrokes))
	time.Sleep(100 * time.Millisecond)
	w.send(t, w.alice, pictoActStrokes, strokesPayload(ep, 9999, [2]int{1, 1}))
	time.Sleep(300 * time.Millisecond)
	if got := len(w.tv.of(pictoActStrokes)); got != relayed {
		t.Errorf("une trame refusée (feuille pleine) n'est jamais relayée (TV : %d → %d)", relayed, got)
	}
	// La feuille déjà dessinée reste intacte et rattrapable : borne respectée, rien d'écrasé.
	w.send(t, w.tv, pictoActSync, struct{}{})
	cv := w.tv.waitAction(t, pictoActCanvas, 1, 5*time.Second)
	_, points := canvasStrokeCount(t, cv[len(cv)-1])
	if points > 60000 || points < 50000 {
		t.Errorf("la feuille rattrapée doit contenir ≈ 60 000 points (≤ borne), got %d", points)
	}
	if ph := gameField(t, w.admin, "PHASE"); ph != "STARTED" {
		t.Errorf("une feuille pleine ne coupe pas la manche, phase = %v", ph)
	}
}

// §9.1 : PICTO_CANVAS_SYNC — un vplayer qui n'est PAS le dessinateur est refusé ; le dessinateur, la TV, l'admin et
// l'anim sont servis ; la réponse va au demandeur seul.
func TestPictoWire193_CanvasSync_AccessControl(t *testing.T) {
	w := newPictoWire(t)
	w.chooseTeamAndDrawer(t, w.aliceID)
	w.startRound(t, w.alice)
	ep := int(gameField(t, w.admin, "PICTO_CANVAS_EPOCH").(float64))
	w.send(t, w.alice, pictoActStrokes, strokesPayload(ep, 1, [2]int{100, 100}, [2]int{200, 200}))
	w.tv.waitAction(t, pictoActStrokes, 1, 3*time.Second)

	// Autorisés : la feuille entière à chacun, à lui seul.
	for _, c := range []*wireClient{w.tv, w.admin, w.anim, w.alice} {
		n := len(c.of(pictoActCanvas))
		w.send(t, c, pictoActSync, struct{}{})
		fr := c.waitAction(t, pictoActCanvas, n+1, 3*time.Second)
		if s, p := canvasStrokeCount(t, fr[len(fr)-1]); s != 1 || p != 2 {
			t.Errorf("%s : PICTO_CANVAS = %d trait(s), %d point(s), attendu 1 / 2", c.name, s, p)
		}
		time.Sleep(550 * time.Millisecond) // sous la limite de débit (~2/s) entre deux demandes
	}

	// Refusés : un vplayer qui n'est pas le dessinateur (même équipe ou autre équipe) ne reçoit rien.
	for _, c := range []*wireClient{w.bob, w.carol} {
		n := len(c.of(pictoActCanvas))
		w.send(t, c, pictoActSync, struct{}{})
		time.Sleep(300 * time.Millisecond)
		if got := len(c.of(pictoActCanvas)); got != n {
			t.Errorf("%s (non-dessinateur) ne doit pas recevoir PICTO_CANVAS (§9.1)", c.name)
		}
		for _, f := range c.snapshot() {
			if strings.Contains(f, `"PTS"`) {
				t.Errorf("%s : la feuille ne doit jamais atteindre une tablette non dessinatrice : %s", c.name, f)
			}
		}
	}
}

// Débit de PICTO_CANVAS_SYNC (~2/s par connexion, audit m2) : une rafale de demandes ne produit que quelques réponses,
// la connexion n'est pas coupée, et le service reprend ensuite.
func TestPictoWire193_CanvasSync_RateLimited(t *testing.T) {
	w := newPictoWire(t)
	w.chooseTeamAndDrawer(t, w.aliceID)
	w.startRound(t, w.alice)
	ep := int(gameField(t, w.admin, "PICTO_CANVAS_EPOCH").(float64))
	w.send(t, w.alice, pictoActStrokes, strokesPayload(ep, 1, [2]int{100, 100}))
	w.tv.waitAction(t, pictoActStrokes, 1, 3*time.Second)

	start := time.Now()
	for i := 0; i < 30; i++ {
		w.send(t, w.tv, pictoActSync, struct{}{})
	}
	time.Sleep(500 * time.Millisecond)
	elapsed := time.Since(start).Seconds()
	got := len(w.tv.of(pictoActCanvas))
	maxExpected := int(elapsed*2) + 3 // ~2/s + une marge d'amorçage
	if got < 1 {
		t.Errorf("la première demande doit être servie")
	}
	if got > maxExpected {
		t.Errorf("30 demandes en %.1f s : %d réponses PICTO_CANVAS, au plus ~%d attendues (limite ~2/s, chaque réponse peut peser ~700 Ko)", elapsed, got, maxExpected)
	}
	// Après une pause, le service reprend.
	time.Sleep(1200 * time.Millisecond)
	n := len(w.tv.of(pictoActCanvas))
	w.send(t, w.tv, pictoActSync, struct{}{})
	w.tv.waitAction(t, pictoActCanvas, n+1, 3*time.Second)
	// La connexion n'a pas été coupée par le débit.
	if err := w.tv.conn.WriteControl(9, nil, time.Now().Add(time.Second)); err != nil {
		t.Errorf("la connexion de la TV doit rester ouverte : %v", err)
	}
}

// §8.3 : NEW_GAME vide le journal de feuille — une TV qui demande la feuille ensuite reçoit une feuille vierge.
func TestPictoWire193_NewGame_ClearsSheetJournal(t *testing.T) {
	w := newPictoWire(t)
	w.chooseTeamAndDrawer(t, w.aliceID)
	w.startRound(t, w.alice)
	ep := int(gameField(t, w.admin, "PICTO_CANVAS_EPOCH").(float64))
	w.send(t, w.alice, pictoActStrokes, strokesPayload(ep, 1, [2]int{100, 100}, [2]int{200, 200}))
	w.tv.waitAction(t, pictoActStrokes, 1, 3*time.Second)

	w.send(t, w.admin, protocol.ActionStop, struct{}{})
	waitGameField(t, w.admin, "PHASE", "STOPPED", 3*time.Second)
	w.send(t, w.admin, protocol.ActionNewGame, struct{}{})
	waitGameField(t, w.admin, "PHASE", "NEW_GAME", 5*time.Second)

	time.Sleep(1100 * time.Millisecond) // hors limite de débit éventuelle
	n := len(w.tv.of(pictoActCanvas))
	w.send(t, w.tv, pictoActSync, struct{}{})
	fr := w.tv.waitAction(t, pictoActCanvas, n+1, 3*time.Second)
	if s, p := canvasStrokeCount(t, fr[len(fr)-1]); s != 0 || p != 0 {
		t.Errorf("NEW_GAME doit vider le journal : PICTO_CANVAS contient encore %d trait(s), %d point(s)", s, p)
	}
	// Les champs PICTO du GameState sont aussi remis à vide (§5).
	for _, f := range []string{"PICTO_TEAM", "PICTO_DRAWER", "PICTO_OUTCOME", "PICTO_REVEALED_WORD"} {
		if v := gameField(t, w.admin, f); v != "" {
			t.Errorf("NEW_GAME : GAME.%s = %v, attendu vide (§5)", f, v)
		}
	}
}
