package server

import (
	"testing"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
)

// #193 — journal et garde de PICTO_STROKES, au niveau unitaire (contracts/picto.md §8.3 à §8.6).

func pictoCtx(now time.Time) PictoStrokeContext {
	return PictoStrokeContext{ClientType: ClientTypeVPlayer, PlayerID: "alice", Drawer: "alice", Phase: game.PhaseStarted, Epoch: 3, IsPicto: true, Now: now}
}

func pictoFrame(epoch, id int, pts ...[]int) protocol.PictoStrokesPayload {
	return protocol.PictoStrokesPayload{Epoch: epoch, Strokes: []protocol.PictoStroke{{ID: id, Pts: pts, End: true}}}
}

func wantPErr(t *testing.T, perr *game.PictoError, code string) {
	t.Helper()
	if perr == nil || perr.Code != code {
		t.Fatalf("erreur %s attendue, got %v", code, perr)
	}
}

func TestPictoCanvasDev_AcceptRelaysWithIncreasingSeq(t *testing.T) {
	c := NewPictoCanvas()
	now := time.Now()
	for want := 1; want <= 3; want++ {
		relay, perr := c.Accept("c1", pictoFrame(3, want, []int{10, 20}), pictoCtx(now))
		if perr != nil || relay == nil || relay.Seq != want || relay.Epoch != 3 {
			t.Fatalf("trame %d : relay=%+v err=%v", want, relay, perr)
		}
	}
	if s := c.Snapshot(); s.Seq != 3 || s.Epoch != 3 || len(s.Strokes) != 3 {
		t.Errorf("snapshot : %+v", s)
	}
}

func TestPictoCanvasDev_StrokeContinuedByLaterFrame(t *testing.T) {
	c := NewPictoCanvas()
	now := time.Now()
	f1 := protocol.PictoStrokesPayload{Epoch: 3, Strokes: []protocol.PictoStroke{{ID: 1, Pts: [][]int{{1, 1}, {2, 2}}}}}
	f2 := protocol.PictoStrokesPayload{Epoch: 3, Strokes: []protocol.PictoStroke{{ID: 1, Pts: [][]int{{3, 3}}, End: true}}}
	c.Accept("c1", f1, pictoCtx(now))
	c.Accept("c1", f2, pictoCtx(now))
	s := c.Snapshot()
	if len(s.Strokes) != 1 || len(s.Strokes[0].Pts) != 3 || !s.Strokes[0].End {
		t.Errorf("le trait 1 doit être poursuivi : %+v", s.Strokes)
	}
}

func TestPictoCanvasDev_IdentityComesFromTheConnection(t *testing.T) {
	c := NewPictoCanvas()
	now := time.Now()
	for name, mod := range map[string]func(*PictoStrokeContext){
		"autre joueur":     func(x *PictoStrokeContext) { x.PlayerID = "bob" },
		"non lié":          func(x *PictoStrokeContext) { x.PlayerID = "" },
		"type tv":          func(x *PictoStrokeContext) { x.ClientType = ClientTypeTV },
		"type admin":       func(x *PictoStrokeContext) { x.ClientType = ClientTypeAdmin },
		"sans dessinateur": func(x *PictoStrokeContext) { x.Drawer, x.PlayerID = "", "" },
	} {
		ctx := pictoCtx(now)
		mod(&ctx)
		relay, perr := c.Accept("c-"+name, pictoFrame(3, 1, []int{1, 1}), ctx)
		if relay != nil {
			t.Errorf("%s : jamais relayé", name)
		}
		wantPErr(t, perr, game.PictoErrNotDrawer)
	}
	if s := c.Snapshot(); len(s.Strokes) != 0 {
		t.Error("une trame refusée n'a jamais d'effet")
	}
}

func TestPictoCanvasDev_PhaseEpochAndShape(t *testing.T) {
	now := time.Now()
	c := NewPictoCanvas()
	for _, ph := range []game.GamePhase{game.PhasePaused, game.PhaseReady, game.PhaseStopped, game.PhaseRevealed} {
		ctx := pictoCtx(now)
		ctx.Phase = ph
		_, perr := c.Accept("c1", pictoFrame(3, 1, []int{1, 1}), ctx)
		wantPErr(t, perr, game.PictoErrWrongPhase)
	}
	ctx := pictoCtx(now)
	ctx.IsPicto = false
	_, perr := c.Accept("c1", pictoFrame(3, 1, []int{1, 1}), ctx)
	wantPErr(t, perr, game.PictoErrWrongPhase)
	// Époque périmée : ignorée en silence (ni erreur, ni relais).
	if relay, perr := c.Accept("c1", pictoFrame(2, 1, []int{1, 1}), pictoCtx(now)); relay != nil || perr != nil {
		t.Errorf("époque périmée : silence attendu, got %+v %v", relay, perr)
	}
	bad := []protocol.PictoStrokesPayload{
		pictoFrame(3, 1, []int{-1, 5}), pictoFrame(3, 1, []int{5, 10001}), pictoFrame(3, 0, []int{1, 1}),
		pictoFrame(3, 1, []int{1}), pictoFrame(3, 1), {Epoch: 3},
	}
	for i, f := range bad {
		_, perr := c.Accept("c1", f, pictoCtx(now))
		if perr == nil || perr.Code != game.PictoErrInvalidStrokes {
			t.Errorf("trame invalide %d : INVALID_STROKES attendu, got %v", i, perr)
		}
	}
	tooMany := protocol.PictoStrokesPayload{Epoch: 3}
	for i := 1; i <= 21; i++ {
		tooMany.Strokes = append(tooMany.Strokes, protocol.PictoStroke{ID: i, Pts: [][]int{{1, 1}}})
	}
	_, perr = c.Accept("c1", tooMany, pictoCtx(now))
	wantPErr(t, perr, game.PictoErrInvalidStrokes)
	pts := make([][]int, 201)
	for i := range pts {
		pts[i] = []int{1, 1}
	}
	_, perr = c.Accept("c1", pictoFrame(3, 1, pts...), pictoCtx(now))
	wantPErr(t, perr, game.PictoErrInvalidStrokes)
}

func TestPictoCanvasDev_JournalBounds(t *testing.T) {
	c := NewPictoCanvas()
	now := time.Now()
	c.mu.Lock()
	c.epoch, c.points = 3, PictoMaxPoints-1
	c.mu.Unlock()
	if _, perr := c.Accept("c1", pictoFrame(3, 1, []int{1, 1}), pictoCtx(now)); perr != nil {
		t.Fatalf("le dernier point doit passer : %v", perr)
	}
	_, perr := c.Accept("c1", pictoFrame(3, 2, []int{1, 1}), pictoCtx(now))
	wantPErr(t, perr, game.PictoErrCanvasFull)

	c2 := NewPictoCanvas()
	c2.mu.Lock()
	c2.epoch = 3
	for i := 1; i <= PictoMaxStrokes; i++ {
		c2.index[i] = len(c2.strokes)
		c2.strokes = append(c2.strokes, protocol.PictoStroke{ID: i})
	}
	c2.mu.Unlock()
	_, perr = c2.Accept("c1", pictoFrame(3, PictoMaxStrokes+1, []int{1, 1}), pictoCtx(now))
	wantPErr(t, perr, game.PictoErrCanvasFull)
	if _, perr = c2.Accept("c1", pictoFrame(3, 5, []int{1, 1}), pictoCtx(now)); perr != nil { // poursuivre un trait existant reste permis
		t.Errorf("poursuivre un trait existant : %v", perr)
	}
}

func TestPictoCanvasDev_RateLimitOneErrorPerSecond(t *testing.T) {
	c := NewPictoCanvas()
	now := time.Now()
	for i := 1; i <= PictoMaxFramesPerS; i++ {
		if _, perr := c.Accept("c1", pictoFrame(3, i, []int{1, 1}), pictoCtx(now)); perr != nil {
			t.Fatalf("trame %d refusée trop tôt : %v", i, perr)
		}
	}
	_, perr := c.Accept("c1", pictoFrame(3, 99, []int{1, 1}), pictoCtx(now))
	wantPErr(t, perr, game.PictoErrRateLimited)
	if relay, perr := c.Accept("c1", pictoFrame(3, 100, []int{1, 1}), pictoCtx(now)); relay != nil || perr != nil {
		t.Errorf("une seule erreur par seconde : silence attendu ensuite, got %+v %v", relay, perr)
	}
	// Une autre connexion n'est pas limitée ; la seconde suivante repart.
	if _, perr := c.Accept("c2", pictoFrame(3, 101, []int{1, 1}), pictoCtx(now)); perr != nil {
		t.Errorf("limite par connexion : %v", perr)
	}
	if _, perr := c.Accept("c1", pictoFrame(3, 102, []int{1, 1}), pictoCtx(now.Add(1100*time.Millisecond))); perr != nil {
		t.Errorf("la fenêtre suivante doit repartir : %v", perr)
	}
}

func TestPictoCanvasDev_ResetAndSnapshotIsACopy(t *testing.T) {
	c := NewPictoCanvas()
	now := time.Now()
	c.Accept("c1", pictoFrame(3, 1, []int{1, 1}), pictoCtx(now))
	s := c.Snapshot()
	s.Strokes[0].Pts[0][0] = 9999
	if got := c.Snapshot().Strokes[0].Pts[0][0]; got != 1 {
		t.Error("Snapshot doit être une copie")
	}
	c.Reset(4)
	if s := c.Snapshot(); s.Epoch != 4 || s.Seq != 0 || len(s.Strokes) != 0 {
		t.Errorf("après Reset(4) : %+v", s)
	}
}

// Géométrie (§8.2) : cas nommés, mêmes valeurs côté JS.
func TestPictoGeometryDev_NamedCases(t *testing.T) {
	check := func(name string, px, py, w, h float64, wx, wy int) {
		if x, y := PictoNormalize(px, py, w, h); x != wx || y != wy {
			t.Errorf("%s : (%d,%d), attendu (%d,%d)", name, x, y, wx, wy)
		}
	}
	check("normalize_center", 150, 150, 300, 300, 5000, 5000)
	check("normalize_corners", 0, 0, 300, 300, 0, 0)
	check("normalize_corners", 300, 300, 300, 300, 10000, 10000)
	check("clamp_outside", -10, 400, 300, 300, 0, 10000)
	check("round_half", 0.5, 0.5, 10000, 10000, 1, 1)     // 0.5 → 1 (demi arrondi vers le haut)
	if PictoLineWidth != 80 || PictoDotDiameter() != 80 { // line_width_units, single_point_dot
		t.Error("épaisseur fixe de 80 unités, un point seul = disque de ce diamètre")
	}
}

// Multiplier d'abord : (3/20000)*10000 = 1.4999999999999998 en flottant, alors que 3*10000/20000 = 1.5 → 2.
func TestPictoGeometryDev_RoundHalfMultipliesFirst(t *testing.T) {
	if x, y := PictoNormalize(3, 3, 20000, 20000); x != 2 || y != 2 {
		t.Errorf("(%d,%d), attendu (2,2)", x, y)
	}
}

// Revue M1 (majeur) : PICTO_CANVAS_SYNC limité à PictoMaxSyncsPerS par connexion, une erreur au plus par seconde.
func TestPictoCanvasDev_AllowSyncRateLimit(t *testing.T) {
	c := NewPictoCanvas()
	now := time.Now()
	for i := 0; i < PictoMaxSyncsPerS; i++ {
		if ok, rep := c.AllowSync("c1", now); !ok || rep {
			t.Fatalf("demande %d : acceptée attendue", i+1)
		}
	}
	if ok, rep := c.AllowSync("c1", now); ok || !rep {
		t.Errorf("au-delà : refusée avec une erreur, got ok=%v report=%v", ok, rep)
	}
	if ok, rep := c.AllowSync("c1", now); ok || rep {
		t.Errorf("une seule erreur par seconde : refusée en silence ensuite, got ok=%v report=%v", ok, rep)
	}
	if ok, _ := c.AllowSync("c2", now); !ok {
		t.Error("la limite est par connexion")
	}
	if ok, _ := c.AllowSync("c1", now.Add(1100*time.Millisecond)); !ok {
		t.Error("la fenêtre suivante doit repartir")
	}
	c.ForgetClient("c1") // revue M2 : l'état de la connexion fermée est libéré
	c.mu.Lock()
	_, a := c.syncs["c1"]
	_, b := c.rate["c1"]
	c.mu.Unlock()
	if a || b {
		t.Error("ForgetClient doit oublier les deux compteurs")
	}
}

// TrySendToTypes sérialise PICTO_STROKES une seule fois pour tous les destinataires (rien de propre au type) :
// mêmes octets pour TV, admin et anim, jamais pour une tablette ; un tampon plein perd la trame sans éviction.
func TestPictoTrySendDev_SingleMarshalSameBytesForAllTypes(t *testing.T) {
	hub := NewWebSocketHub()
	mk := func(id string, ct ClientType, buf int) *WebSocketClient {
		c := &WebSocketClient{ID: id, Type: ct, Send: make(chan []byte, buf)}
		hub.clients[c] = true
		return c
	}
	tv, admin, anim, tablet, full := mk("tv", ClientTypeTV, 4), mk("adm", ClientTypeAdmin, 4), mk("anim", ClientTypeAnim, 4), mk("tab", ClientTypeVPlayer, 4), mk("tv-full", ClientTypeTV, 1)
	full.Send <- []byte("occupé")
	msg, err := protocol.NewMessage(protocol.ActionPictoStrokes, protocol.PictoStrokesPayload{Epoch: 3, Seq: 7, Strokes: []protocol.PictoStroke{{ID: 1, Pts: [][]int{{1, 2}}, End: true}}})
	if err != nil {
		t.Fatal(err)
	}
	hub.TrySendToTypes(msg, ClientTypeTV, ClientTypeAdmin, ClientTypeAnim)
	want, _ := msg.SerializeForWebSocket()
	for name, c := range map[string]*WebSocketClient{"tv": tv, "admin": admin, "anim": anim} {
		select {
		case got := <-c.Send:
			if string(got) != string(want) {
				t.Errorf("%s : octets différents", name)
			}
		default:
			t.Errorf("%s : trame non reçue", name)
		}
	}
	if len(tablet.Send) != 0 {
		t.Error("une tablette ne reçoit jamais PICTO_STROKES")
	}
	if len(full.Send) != 1 {
		t.Error("tampon plein : la trame est perdue, rien d'autre ne change")
	}
	if _, still := hub.clients[full]; !still {
		t.Error("le client au tampon plein reste enregistré")
	}
}
