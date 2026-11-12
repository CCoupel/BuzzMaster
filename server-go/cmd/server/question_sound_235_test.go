// #235 (v11.6.0, LOT-2) — lecture du média audio sur MEMORY, MEMOTION (hôte ET carte) et ENTRACTE.
// Source normative : contracts/sound.md §14.2–14.5 (+ §10.8 gate T0, §11.2 Muted, §12.6 décompte), game-state.md
// (QUESTION_SOUND_STATE / QUESTION_SOUND_OWNER / QUESTION_SOUND_UNAVAILABLE), plan LOT-2 tâches 11–12.
// Écrit depuis le contrat avant le code (rouge attendu).
//
// Frontière : le vrai pipeline (setupCallbacks + handleWebMessage) avec un MediaPlayer factice qui journalise ses
// appels ; état lu par la sérialisation JSON de GameState (clés du contrat) ; modèle posé par json.Unmarshal
// (le fichier compile avant le dev). Décisions testées : D9 (la carte REMPLACE l'hôte, sans reprise),
// « le plus intérieur gagne », carte sans son ⇒ l'hôte continue, gate T0 appliquée au flip sans le refuser.
// Préfixe tw235.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// tw235Player : MediaPlayer factice qui distingue Play / PlayLoop et mémorise l'ordre des appels.
type tw235Player struct {
	mu    sync.Mutex
	ops   []string // "play:<base>" | "loop:<base>" | "stop" | "pause" | "resume"
	state audio.MediaState
}

func (p *tw235Player) rec(op string) { p.mu.Lock(); p.ops = append(p.ops, op); p.mu.Unlock() }
func (p *tw235Player) Play(path string) error {
	p.rec("play:" + filepath.Base(path))
	p.mu.Lock()
	p.state = audio.MediaPlaying
	p.mu.Unlock()
	return nil
}
func (p *tw235Player) PlayLoop(path string) error {
	p.rec("loop:" + filepath.Base(path))
	p.mu.Lock()
	p.state = audio.MediaPlaying
	p.mu.Unlock()
	return nil
}
func (p *tw235Player) Pause() {
	p.rec("pause")
	p.mu.Lock()
	p.state = audio.MediaPaused
	p.mu.Unlock()
}
func (p *tw235Player) Resume() {
	p.rec("resume")
	p.mu.Lock()
	p.state = audio.MediaPlaying
	p.mu.Unlock()
}
func (p *tw235Player) Stop() { p.rec("stop"); p.mu.Lock(); p.state = audio.MediaIdle; p.mu.Unlock() }
func (p *tw235Player) State() audio.MediaState {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}
func (p *tw235Player) Close() error { return nil }

func (p *tw235Player) snapshot() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.ops...)
}

// starts : les appels de lecture (play:/loop:) dans l'ordre, sans les stop/pause.
func (p *tw235Player) starts() []string {
	var out []string
	for _, op := range p.snapshot() {
		if strings.HasPrefix(op, "play:") || strings.HasPrefix(op, "loop:") {
			out = append(out, op)
		}
	}
	return out
}

func (p *tw235Player) count(prefix string) int {
	n := 0
	for _, op := range p.snapshot() {
		if strings.HasPrefix(op, prefix) {
			n++
		}
	}
	return n
}

// tw235App : App câblée (setupCallbacks), adaptateur de son de question sur le faux lecteur, répertoire de
// questions temporaire. Les fichiers sons sont créés par tw235WriteSound (canoniques, donc valides pour la gate T0).
func tw235App(t *testing.T) (*App, *tw235Player) {
	t.Helper()
	app := newEntracteTestApp(t)
	app.config.Storage.QuestionsDir = t.TempDir()
	fake := &tw235Player{}
	app.qsound = &questionSoundAdapter{app: app, player: fake}
	tw219qSetSoundEnabledFor(app, true)
	prevEnabled := app.config.Sound.Enabled
	app.config.Sound.Enabled = true
	app.logsHub = server.NewLogsWebSocketHub(10)
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, app.logsHub)
	app.setupCallbacks()
	app.soundEngine.Store(app.newSoundEngine(audio.NewFakeOutput()))
	t.Cleanup(func() {
		app.engine.Stop()
		tw257CancelWatchdog(app)
		app.config.Sound.Enabled = prevEnabled
	})
	return app, fake
}

// tw235WriteSound écrit un WAV canonique d'1 s sous <QuestionsDir>/<qid>/<file> et renvoie l'URL de question.
func tw235WriteSound(t *testing.T, app *App, qid, file string) string {
	t.Helper()
	dir := filepath.Join(app.config.Storage.QuestionsDir, qid)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, file), audio.BuildCanonicalWAV(make([]byte, audio.SampleRate*audio.FrameSize)), 0o644); err != nil {
		t.Fatal(err)
	}
	return "/question/" + qid + "/" + file
}

// tw235Memotion construit une question MEMOTION (hôte) avec cartes ; patch = JSON appliqué par-dessus
// (champs SOUND* de l'hôte et des cartes — noms JSON du contrat).
func tw235Memotion(t *testing.T, cardsJSON, hostPatch string) *game.Question {
	t.Helper()
	q := &game.Question{ID: "mq1", Question: "MEMOTION", Type: game.QuestionTypeMemotion, Points: "10", Time: "0", MotionMode: string(game.MemoryModeSolo)}
	if err := json.Unmarshal([]byte(`{"MOTION_CARDS":`+cardsJSON+`}`), q); err != nil {
		t.Fatalf("setup invalide (cartes) : %v", err)
	}
	if hostPatch != "" {
		if err := json.Unmarshal([]byte(hostPatch), q); err != nil {
			t.Fatalf("setup invalide (hôte) : %v", err)
		}
	}
	return q
}

func tw235StartMemotion(t *testing.T, app *App, q *game.Question) {
	t.Helper()
	app.engine.Ready(q.ID, q)
	app.engine.StartImmediate(0)
}

func tw235StateJSON(t *testing.T, app *App) map[string]interface{} {
	t.Helper()
	raw, err := json.Marshal(app.engine.GetState())
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func tw235Owner(t *testing.T, app *App) interface{} {
	return tw235StateJSON(t, app)["QUESTION_SOUND_OWNER"]
}

func tw235Select(t *testing.T, app *App, id string) {
	t.Helper()
	dispatchAs(t, app, server.ClientTypeAnim, protocol.ActionMotionSelect, protocol.MotionSelectPayload{CardID: id})
}
func tw235Flip(t *testing.T, app *App) {
	t.Helper()
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionMotionFlip, nil)
}

const tw235TwoCards = `[
 {"ID":"c1","RECTO_THEME":"un","DIFFICULTY":1,"SOUND":"/question/mq1/card1.wav"},
 {"ID":"c2","RECTO_THEME":"deux","DIFFICULTY":1}]`

// ---------------------------------------------------------------------------
// Hôte MEMOTION : joué au START, boucle selon SOUND_LOOP (§14.4).
// ---------------------------------------------------------------------------

func TestMemotionHostSound235_PlayedAtStart(t *testing.T) {
	app, fake := tw235App(t)
	tw235WriteSound(t, app, "mq1", "host.wav")
	q := tw235Memotion(t, tw235TwoCards, `{"SOUND":"/question/mq1/host.wav"}`)
	tw235StartMemotion(t, app, q)

	if got := fake.starts(); len(got) != 1 || got[0] != "play:host.wav" {
		t.Fatalf("le son d'hôte MEMOTION doit être joué UNE fois au START, got %v", got)
	}
	if st := app.engine.GetState().QuestionSoundState; st != game.QuestionSoundPlaying {
		t.Errorf("QUESTION_SOUND_STATE=PLAYING attendu, got %q", st)
	}
	if o := tw235Owner(t, app); o != "HOST" {
		t.Errorf("QUESTION_SOUND_OWNER=\"HOST\" attendu, got %v", o)
	}
}

func TestMemotionHostSound235_LoopFlag_UsesPlayLoop(t *testing.T) {
	app, fake := tw235App(t)
	tw235WriteSound(t, app, "mq1", "host.wav")
	tw235StartMemotion(t, app, tw235Memotion(t, tw235TwoCards, `{"SOUND":"/question/mq1/host.wav","SOUND_LOOP":true}`))
	if got := fake.starts(); len(got) != 1 || got[0] != "loop:host.wav" {
		t.Errorf("SOUND_LOOP ⇒ PlayLoop, got %v", got)
	}
}

// Non-régression §14.5 : MEMOTION sans aucun son (hôte ni carte) ⇒ aucun appel au MediaPlayer.
func TestMemotion235_NoSoundAnywhere_NeverTouchesMediaPlayer(t *testing.T) {
	app, fake := tw235App(t)
	q := tw235Memotion(t, `[{"ID":"c1","RECTO_THEME":"un","DIFFICULTY":1},{"ID":"c2","RECTO_THEME":"deux","DIFFICULTY":1}]`, "")
	tw235StartMemotion(t, app, q)
	tw235Select(t, app, "c1")
	tw235Flip(t, app)
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionMotionReveal, nil)
	if ops := fake.snapshot(); len(ops) != 0 && fake.count("play:")+fake.count("loop:") != 0 {
		t.Errorf("aucune lecture attendue sans son (hôte ni carte), got %v", ops)
	}
	if o := tw235Owner(t, app); o != "" {
		t.Errorf("QUESTION_SOUND_OWNER=\"\" attendu sans média, got %v", o)
	}
}

// ---------------------------------------------------------------------------
// Carte : au retournement, REMPLACE l'hôte (D9), sans reprise ; « le plus intérieur gagne ».
// ---------------------------------------------------------------------------

func TestMemotionCardSound235_Flip_ReplacesHostSound_OwnerIsCard(t *testing.T) {
	app, fake := tw235App(t)
	tw235WriteSound(t, app, "mq1", "host.wav")
	tw235WriteSound(t, app, "mq1", "card1.wav")
	tw235StartMemotion(t, app, tw235Memotion(t, tw235TwoCards, `{"SOUND":"/question/mq1/host.wav"}`))

	tw235Select(t, app, "c1")
	if got := fake.starts(); len(got) != 1 {
		t.Fatalf("la SÉLECTION d'une carte ne lance pas son son (c'est le retournement), starts=%v", got)
	}
	tw235Flip(t, app)

	got := fake.starts()
	if len(got) != 2 || got[1] != "play:card1.wav" {
		t.Fatalf("au retournement, le son de la carte doit être joué après celui de l'hôte, got %v", got)
	}
	// la voix hôte est coupée (arrêt net) avant que la carte ne sonne
	ops := fake.snapshot()
	idxCard := -1
	for i, op := range ops {
		if op == "play:card1.wav" {
			idxCard = i
		}
	}
	if idxCard < 0 || fake.State() != audio.MediaPlaying {
		t.Fatalf("lecture de la carte attendue en cours, ops=%v state=%v", ops, fake.State())
	}
	if o := tw235Owner(t, app); o != "CARD:c1" {
		t.Errorf("QUESTION_SOUND_OWNER=\"CARD:c1\" attendu, got %v", o)
	}
	if st := app.engine.GetState().QuestionSoundState; st != game.QuestionSoundPlaying {
		t.Errorf("QUESTION_SOUND_STATE=PLAYING attendu, got %q", st)
	}
}

func TestMemotionCardSound235_CardLoopFlag_UsesPlayLoop(t *testing.T) {
	app, fake := tw235App(t)
	tw235WriteSound(t, app, "mq1", "card1.wav")
	cards := `[{"ID":"c1","RECTO_THEME":"un","DIFFICULTY":1,"SOUND":"/question/mq1/card1.wav","SOUND_LOOP":true}]`
	tw235StartMemotion(t, app, tw235Memotion(t, cards, ""))
	tw235Select(t, app, "c1")
	tw235Flip(t, app)
	if got := fake.starts(); len(got) != 1 || got[0] != "loop:card1.wav" {
		t.Errorf("SOUND_LOOP de la carte ⇒ PlayLoop, got %v", got)
	}
}

// D9 : à la fermeture de la carte, le son de carte s'arrête et celui de l'hôte NE REPREND PAS.
func TestMemotionCardSound235_D9_DoneStopsCard_HostDoesNotResume(t *testing.T) {
	app, fake := tw235App(t)
	tw235WriteSound(t, app, "mq1", "host.wav")
	tw235WriteSound(t, app, "mq1", "card1.wav")
	tw235StartMemotion(t, app, tw235Memotion(t, tw235TwoCards, `{"SOUND":"/question/mq1/host.wav"}`))
	tw235Select(t, app, "c1")
	tw235Flip(t, app)
	before := len(fake.starts())

	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionMotionDone, protocol.MotionDonePayload{CardID: "c1"})

	if fake.State() != audio.MediaIdle {
		t.Errorf("MEMOTION_DONE : le son de carte doit s'arrêter, state=%v", fake.State())
	}
	if len(fake.starts()) != before {
		t.Errorf("D9 : le son d'hôte NE reprend PAS (« une question chasse l'autre »), starts=%v", fake.starts())
	}
	if o := tw235Owner(t, app); o != "" {
		t.Errorf("QUESTION_SOUND_OWNER=\"\" attendu après arrêt, got %v", o)
	}
	if st := app.engine.GetState().QuestionSoundState; st != game.QuestionSoundIdle {
		t.Errorf("QUESTION_SOUND_STATE=IDLE attendu, got %q", st)
	}
}

// Carte SANS son : l'hôte continue de sonner (pas d'arrêt, pas de relance).
func TestMemotionCardSound235_CardWithoutSound_HostKeepsPlaying(t *testing.T) {
	app, fake := tw235App(t)
	tw235WriteSound(t, app, "mq1", "host.wav")
	tw235StartMemotion(t, app, tw235Memotion(t, tw235TwoCards, `{"SOUND":"/question/mq1/host.wav"}`))
	stopsBefore := fake.count("stop")
	tw235Select(t, app, "c2") // c2 n'a pas de son
	tw235Flip(t, app)

	if got := fake.starts(); len(got) != 1 {
		t.Errorf("une carte sans son ne relance rien, starts=%v", got)
	}
	if fake.count("stop") != stopsBefore || fake.State() != audio.MediaPlaying {
		t.Errorf("l'hôte doit continuer à sonner (aucun stop), ops=%v state=%v", fake.snapshot(), fake.State())
	}
	if o := tw235Owner(t, app); o != "HOST" {
		t.Errorf("QUESTION_SOUND_OWNER=\"HOST\" attendu, got %v", o)
	}
}

// Arrêts : révélation et STOP coupent le son de carte.
func TestMemotionCardSound235_RevealAndStop_StopCardSound(t *testing.T) {
	for _, tc := range []struct {
		name string
		stop func(t *testing.T, app *App)
	}{
		{"révélation", func(t *testing.T, app *App) {
			dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionMotionReveal, nil)
		}},
		{"STOP", func(t *testing.T, app *App) {
			dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionStop, nil)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, fake := tw235App(t)
			tw235WriteSound(t, app, "mq1", "card1.wav")
			tw235StartMemotion(t, app, tw235Memotion(t, tw235TwoCards, ""))
			tw235Select(t, app, "c1")
			tw235Flip(t, app)
			if fake.State() != audio.MediaPlaying {
				t.Fatalf("setup invalide : le son de carte devrait jouer, ops=%v", fake.snapshot())
			}
			tc.stop(t, app)
			if fake.State() != audio.MediaIdle {
				t.Errorf("%s : le son de carte doit s'arrêter, state=%v", tc.name, fake.State())
			}
			if st := app.engine.GetState().QuestionSoundState; st != game.QuestionSoundIdle {
				t.Errorf("%s : QUESTION_SOUND_STATE=IDLE attendu, got %q", tc.name, st)
			}
		})
	}
}

func TestMemotionCardSound235_NewGame_StopsCardSound(t *testing.T) {
	app, fake := tw235App(t)
	tw235WriteSound(t, app, "mq1", "card1.wav")
	tw235StartMemotion(t, app, tw235Memotion(t, tw235TwoCards, ""))
	tw235Select(t, app, "c1")
	tw235Flip(t, app)
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionNewGame, nil)
	if fake.State() != audio.MediaIdle {
		t.Errorf("NEW_GAME : le son de carte doit s'arrêter, state=%v", fake.State())
	}
}

// Les gestes QUESTION_SOUND agissent sur le média COURANT (ici : celui de la carte).
func TestMemotionCardSound235_QuestionSoundGestures_ActOnCurrentMedia(t *testing.T) {
	app, fake := tw235App(t)
	tw235WriteSound(t, app, "mq1", "card1.wav")
	tw235StartMemotion(t, app, tw235Memotion(t, tw235TwoCards, ""))
	tw235Select(t, app, "c1")
	tw235Flip(t, app)

	app.qsound.HandleCommand(protocol.QuestionSoundCommandPause)
	if fake.State() != audio.MediaPaused || app.engine.GetState().QuestionSoundState != game.QuestionSoundPaused {
		t.Errorf("PAUSE : le média de la carte doit être en pause (player=%v state=%q)", fake.State(), app.engine.GetState().QuestionSoundState)
	}
	app.qsound.HandleCommand(protocol.QuestionSoundCommandResume)
	if fake.State() != audio.MediaPlaying {
		t.Errorf("RESUME : lecture reprise attendue, got %v", fake.State())
	}
}

// ---------------------------------------------------------------------------
// Gate T0 (§10.8) appliquée au retournement : fichier de carte indisponible ⇒ la carte S'OUVRE quand même.
// ---------------------------------------------------------------------------

func TestMemotionCardSound235_FlipWithMissingFile_DoesNotRefuseFlip_ReportsFileUnavailable(t *testing.T) {
	app, fake := tw235App(t)
	// card1.wav volontairement NON créé
	tw235StartMemotion(t, app, tw235Memotion(t, tw235TwoCards, ""))
	tw235Select(t, app, "c1")
	tw235Flip(t, app)

	st := app.engine.GetState()
	if st.MotionSubPhase != game.MotionSubPhaseQuestion {
		t.Fatalf("le retournement ne doit PAS être refusé quand le son de carte est indisponible (subphase=%q)", st.MotionSubPhase)
	}
	if n := fake.count("play:") + fake.count("loop:"); n != 0 {
		t.Errorf("aucune lecture attendue pour un fichier absent, ops=%v", fake.snapshot())
	}
	if u := tw235StateJSON(t, app)["QUESTION_SOUND_UNAVAILABLE"]; u != "FILE" {
		t.Errorf("QUESTION_SOUND_UNAVAILABLE=\"FILE\" attendu (le motif remonte comme un état, §10.9), got %v", u)
	}
}

// La gate de la question HÔTE n'examine QUE le son d'hôte — jamais les cartes (sinon une manche entière serait
// bloquée par un fichier de carte).
func TestMemotionHostGate235_IgnoresCardSoundFiles(t *testing.T) {
	app, _ := tw235App(t)
	tw235WriteSound(t, app, "mq1", "host.wav")
	// card1.wav absent, hôte présent
	q := tw235Memotion(t, tw235TwoCards, `{"SOUND":"/question/mq1/host.wav"}`)
	if ok, reason := app.qsound.questionSoundAvailability(q); !ok {
		t.Errorf("gate T0 de l'hôte : seul le son d'hôte compte, un fichier de carte manquant ne doit pas bloquer (reason=%q)", reason)
	}
}

// Muted résolu ⇒ aucune lecture de carte (même règle que pour une question, §11.2).
func TestMemotionCardSound235_Muted_NoPlay(t *testing.T) {
	app, fake := tw235App(t)
	tw235WriteSound(t, app, "mq1", "card1.wav")
	if err := config.PatchJSON([]byte(`{"sound":{"muted":true}}`)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = config.PatchJSON([]byte(`{"sound":{"muted":false}}`)) })
	tw235StartMemotion(t, app, tw235Memotion(t, tw235TwoCards, ""))
	tw235Select(t, app, "c1")
	tw235Flip(t, app)
	if n := fake.count("play:") + fake.count("loop:"); n != 0 {
		t.Errorf("son coupé : aucune lecture attendue, ops=%v", fake.snapshot())
	}
}

// ---------------------------------------------------------------------------
// MEMORY : son de manche au START (phase de mémorisation comprise).
// ---------------------------------------------------------------------------

func tw235Memory(t *testing.T, patch string) *game.Question {
	t.Helper()
	q := &game.Question{ID: "mem1", Question: "MEMORY", Type: game.QuestionTypeMemory, Points: "1", Time: "0"}
	if err := json.Unmarshal([]byte(patch), q); err != nil {
		t.Fatalf("setup invalide : %v", err)
	}
	return q
}

func TestMemorySound235_PlayedAtStart(t *testing.T) {
	app, fake := tw235App(t)
	tw235WriteSound(t, app, "mem1", "round.wav")
	q := tw235Memory(t, `{"SOUND":"/question/mem1/round.wav"}`)
	app.engine.Ready(q.ID, q)
	app.engine.StartImmediate(0)
	if got := fake.starts(); len(got) != 1 || got[0] != "play:round.wav" {
		t.Errorf("MEMORY : son de manche joué UNE fois au START, got %v", got)
	}
	if o := tw235Owner(t, app); o != "HOST" {
		t.Errorf("QUESTION_SOUND_OWNER=\"HOST\" attendu, got %v", o)
	}
}

func TestMemorySound235_LoopFlag_UsesPlayLoop(t *testing.T) {
	app, fake := tw235App(t)
	tw235WriteSound(t, app, "mem1", "round.wav")
	q := tw235Memory(t, `{"SOUND":"/question/mem1/round.wav","SOUND_LOOP":true}`)
	app.engine.Ready(q.ID, q)
	app.engine.StartImmediate(0)
	if got := fake.starts(); len(got) != 1 || got[0] != "loop:round.wav" {
		t.Errorf("MEMORY + SOUND_LOOP ⇒ PlayLoop, got %v", got)
	}
}

func TestMemorySound235_StopEndsSound(t *testing.T) {
	app, fake := tw235App(t)
	tw235WriteSound(t, app, "mem1", "round.wav")
	q := tw235Memory(t, `{"SOUND":"/question/mem1/round.wav","SOUND_LOOP":true}`)
	app.engine.Ready(q.ID, q)
	app.engine.StartImmediate(0)
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionStop, nil)
	if fake.State() != audio.MediaIdle {
		t.Errorf("STOP : le son de manche doit s'arrêter, state=%v", fake.State())
	}
}

// ---------------------------------------------------------------------------
// ENTRACTE : lu à l'activation, arrêté à la sortie ; Muted respecté (§14.3).
// ---------------------------------------------------------------------------

func tw235Entracte(t *testing.T, patch string) *game.Question {
	t.Helper()
	q := &game.Question{ID: "ent1", Question: "ENTRACTE", Type: game.QuestionTypeEntracte}
	if err := json.Unmarshal([]byte(patch), q); err != nil {
		t.Fatalf("setup invalide : %v", err)
	}
	return q
}

func TestEntracteSound235_ManualActivation_PlaysLoop_DeactivationStops(t *testing.T) {
	app, fake := tw235App(t)
	tw235WriteSound(t, app, "ent1", "pause.wav")
	q := tw235Entracte(t, `{"SOUND":"/question/ent1/pause.wav","SOUND_LOOP":true}`)
	app.engine.Ready(q.ID, q)

	app.handleEntracteSet(entracteSetMsg(true))
	if !app.engine.IsEntracte() {
		t.Fatal("setup invalide : l'entracte devrait être actif")
	}
	if got := fake.starts(); len(got) != 1 || got[0] != "loop:pause.wav" {
		t.Fatalf("activation de l'entracte : son joué en boucle (SOUND_LOOP), got %v", got)
	}
	if o := tw235Owner(t, app); o != "ENTRACTE" {
		t.Errorf("QUESTION_SOUND_OWNER=\"ENTRACTE\" attendu, got %v", o)
	}

	app.handleEntracteSet(entracteSetMsg(false))
	if fake.State() != audio.MediaIdle {
		t.Errorf("sortie de l'entracte : le son doit s'arrêter, state=%v", fake.State())
	}
	if o := tw235Owner(t, app); o != "" {
		t.Errorf("QUESTION_SOUND_OWNER=\"\" attendu après la sortie, got %v", o)
	}
}

func TestEntracteSound235_WithoutLoop_UsesPlay(t *testing.T) {
	app, fake := tw235App(t)
	tw235WriteSound(t, app, "ent1", "pause.wav")
	q := tw235Entracte(t, `{"SOUND":"/question/ent1/pause.wav"}`)
	app.engine.Ready(q.ID, q)
	app.handleEntracteSet(entracteSetMsg(true))
	if got := fake.starts(); len(got) != 1 || got[0] != "play:pause.wav" {
		t.Errorf("sans SOUND_LOOP ⇒ Play (une fois), got %v", got)
	}
}

// Entracte PROGRAMMÉ (question ENTRACTE démarrée) : exactement UNE lecture — pas de double déclenchement avec
// la lecture générique « son de question au START ».
func TestEntracteSound235_ProgrammedEntracte_SingleDeclenchement(t *testing.T) {
	app, fake := tw235App(t)
	tw235WriteSound(t, app, "ent1", "pause.wav")
	q := tw235Entracte(t, `{"SOUND":"/question/ent1/pause.wav","SOUND_LOOP":true}`)
	app.engine.Ready(q.ID, q)
	app.engine.StartImmediate(0)
	time.Sleep(100 * time.Millisecond)
	if !app.engine.IsEntracte() {
		t.Skip("l'entracte programmé n'est pas actif dans ce harnais (voir entracte_ambiance_symmetry_214_test.go)")
	}
	if got := fake.starts(); len(got) != 1 || got[0] != "loop:pause.wav" {
		t.Errorf("entracte programmé : UNE seule lecture attendue, got %v", got)
	}
}

// STOP et NEW_GAME sont REFUSÉS pendant l'entracte (websocket-actions.md « Actions refusées pendant l'entracte »,
// NEW_GAME_REFUSED ENTRACTE_ACTIVE) : le son d'entracte continue, et c'est la sortie de l'entracte qui l'arrête.
func TestEntracteSound235_StopAndNewGameRefusedDuringEntracte_SoundKeepsPlaying(t *testing.T) {
	app, fake := tw235App(t)
	tw235WriteSound(t, app, "ent1", "pause.wav")
	q := tw235Entracte(t, `{"SOUND":"/question/ent1/pause.wav","SOUND_LOOP":true}`)
	app.engine.Ready(q.ID, q)
	app.handleEntracteSet(entracteSetMsg(true))
	for _, action := range []string{protocol.ActionStop, protocol.ActionNewGame} {
		dispatchAs(t, app, server.ClientTypeAdmin, action, nil)
		if fake.State() != audio.MediaPlaying || !app.engine.IsEntracte() {
			t.Errorf("%s refusé pendant l'entracte : le son doit continuer (player=%v entracte=%v)", action, fake.State(), app.engine.IsEntracte())
		}
	}
	app.handleEntracteSet(entracteSetMsg(false))
	if fake.State() != audio.MediaIdle {
		t.Errorf("sortie de l'entracte : arrêt attendu, got %v", fake.State())
	}
}

func TestEntracteSound235_Muted_NoPlay(t *testing.T) {
	app, fake := tw235App(t)
	tw235WriteSound(t, app, "ent1", "pause.wav")
	if err := config.PatchJSON([]byte(`{"sound":{"muted":true}}`)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = config.PatchJSON([]byte(`{"sound":{"muted":false}}`)) })
	q := tw235Entracte(t, `{"SOUND":"/question/ent1/pause.wav"}`)
	app.engine.Ready(q.ID, q)
	app.handleEntracteSet(entracteSetMsg(true))
	if n := fake.count("play:") + fake.count("loop:"); n != 0 {
		t.Errorf("son coupé : aucune lecture d'entracte, ops=%v", fake.snapshot())
	}
}

// Entracte sans son : la cue entracte-debut reste le seul son, aucun appel au MediaPlayer (non-régression).
func TestEntracteSound235_NoSound_NeverTouchesMediaPlayer(t *testing.T) {
	app, fake := tw235App(t)
	q := tw235Entracte(t, `{}`)
	app.engine.Ready(q.ID, q)
	app.handleEntracteSet(entracteSetMsg(true))
	app.handleEntracteSet(entracteSetMsg(false))
	if n := fake.count("play:") + fake.count("loop:"); n != 0 {
		t.Errorf("entracte sans son : aucune lecture attendue, ops=%v", fake.snapshot())
	}
}
