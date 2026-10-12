// Suite test-writer pour #219 (milestone v11.1 — plan
// _work/reports/plan-20260922-103848.md §9 "Non-blocage — cmd/server" et
// "Frontière des chemins", contract sound.md §10.5/§10.7) :
// questionSoundAdapter (cmd/server/question_sound.go) — les trois
// dégradations CA12, le chien de garde, l'idempotence de la libération, la
// règle CA13 ("Rejouer" ne touche jamais le chronomètre), la sûreté sur
// receveur nil (bug réel trouvé par dev-backend, re-vérifiée ici), le
// risque R5 (RAFALE/reprise-pause ne doivent jamais redéclencher le son),
// et la frontière normative avec le moteur de cues (jamais notifySound/
// PlayCue dans ce fichier).
//
// Les scénarios CA12/watchdog/idempotence exigent un engine RÉELLEMENT en
// différé — obtenu via le VRAI Start()+countdown (internal/game n'expose
// aucun raccourci équivalent pour ce cas, voir
// internal/game/deferred_timer_219_test.go pour le détail du trou de
// testabilité de StartImmediate()). Exécutés en t.Parallel() pour amortir
// ce coût réel.
//
// Convention de collision : préfixe tw219q pour ne jamais entrer en
// collision avec un helper d'un autre fichier de ce paquet.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
)

const tw219qCountdownTimeout = 8 * time.Second

// tw219qFakeMediaPlayer implements audio.MediaPlayer with full
// instrumentation — never touches real hardware, deterministic for CA12's
// "fichier illisible" case (PlayErr) and for proving voix-unique/observable
// state without needing a real oto backend.
type tw219qFakeMediaPlayer struct {
	mu      sync.Mutex
	plays   []string
	playErr error
	state   audio.MediaState
	closed  bool
}

func (f *tw219qFakeMediaPlayer) Play(path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.plays = append(f.plays, path)
	if f.playErr != nil {
		return f.playErr
	}
	f.state = audio.MediaPlaying
	return nil
}
func (f *tw219qFakeMediaPlayer) Pause() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state = audio.MediaPaused
}
func (f *tw219qFakeMediaPlayer) Resume() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state = audio.MediaPlaying
}
func (f *tw219qFakeMediaPlayer) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state = audio.MediaIdle
}
func (f *tw219qFakeMediaPlayer) State() audio.MediaState {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state
}
func (f *tw219qFakeMediaPlayer) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}
func (f *tw219qFakeMediaPlayer) playCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.plays)
}

// tw219qSetSoundEnabled resets the GLOBAL config singleton — play() reads
// config.Get().Sound.Enabled directly, never qs.app.config (question_sound.go's
// own design) — so every CA12/watchdog test must set this explicitly rather
// than relying on newTestApp's captured snapshot.
func tw219qSetSoundEnabled(enabled bool) {
	config.SetInstance(&config.Config{Sound: config.SoundConfig{Enabled: enabled}})
}

// tw219qDeferredApp builds a minimal App (newTestApp) whose engine has
// genuinely reached PhaseStarted in DEFERRED mode for a SPEEDY question
// carrying a sound — via the REAL Start()+countdown, never StartImmediate
// (see file header). qsound is wired to player, which the caller controls.
func tw219qDeferredApp(t *testing.T, player audio.MediaPlayer) (*App, *game.Question) {
	t.Helper()
	app := newTestApp(t)
	app.qsound = &questionSoundAdapter{app: app, player: player}

	q := &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy, Answer: "x",
		Sound: "/question/q1/sound_1234.wav", SoundTimerDelayed: true}
	app.engine.Ready(q.ID, q)
	app.engine.TransitionToReady()

	var once sync.Once
	started := make(chan struct{})
	app.engine.OnStateChange = func(phase game.GamePhase) {
		if phase == game.PhaseStarted {
			once.Do(func() { close(started) })
		}
	}
	app.engine.Start(20)
	select {
	case <-started:
	case <-time.After(tw219qCountdownTimeout):
		t.Fatalf("setup invalide : PhaseStarted jamais atteint après %s", tw219qCountdownTimeout)
	}
	if !app.engine.GetState().AnswerTimerWaiting {
		t.Fatal("setup invalide : ANSWER_TIMER_WAITING devrait être true dès → STARTED en mode différé")
	}
	return app, q
}

// ---------------------------------------------------------------------------
// Sûreté sur receveur nil — re-vérifie le bugfix réel de dev-backend
// (handoff dev-backend-20260922-122419.md : panic SIGSEGV sur un *App{}
// construit sans setup(), détecté par TestHandlePong_StillExcludesAnim).
// ---------------------------------------------------------------------------

func TestQuestionSoundAdapter_NilReceiver_NeverPanics(t *testing.T) {
	var qs *questionSoundAdapter
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("un questionSoundAdapter nil ne doit jamais paniquer, got: %v", r)
		}
	}()
	qs.Start(&game.Question{ID: "q1", Sound: "/x.wav"})
	qs.HandleCommand(protocol.QuestionSoundCommandPlay)
	qs.Pause()
	qs.Resume()
	qs.Stop()
}

// ---------------------------------------------------------------------------
// CA9 — pas de son attaché : aucun effet.
// ---------------------------------------------------------------------------

func TestQuestionSoundAdapter_Start_NoSound_IsNoOp(t *testing.T) {
	app := newTestApp(t)
	fake := &tw219qFakeMediaPlayer{}
	app.qsound = &questionSoundAdapter{app: app, player: fake}

	app.qsound.Start(&game.Question{ID: "q1", Sound: ""})

	if fake.playCount() != 0 {
		t.Error("Start() sur une question sans son ne doit jamais appeler Play()")
	}
	if app.engine.GetState().QuestionSoundState != game.QuestionSoundIdle {
		t.Error("Start() sur une question sans son ne doit jamais changer QUESTION_SOUND_STATE")
	}
}

// ---------------------------------------------------------------------------
// CA12 (×3) — la lecture ne démarre pas réellement ⇒ le chronomètre
// démarre immédiatement. Le point de revue n°1 du lot (R10).
// ---------------------------------------------------------------------------

func TestQuestionSoundAdapter_CA12_SoundDisabled_ReleasesImmediately(t *testing.T) {
	t.Parallel()
	fake := &tw219qFakeMediaPlayer{}
	app, q := tw219qDeferredApp(t, fake)
	defer app.engine.Stop()

	tw219qSetSoundEnabled(false)
	app.qsound.Start(q)

	if fake.playCount() != 0 {
		t.Error("CA12 (son désactivé) : Play() ne doit JAMAIS être appelé — vérifié avant tout accès au player")
	}
	if app.engine.GetState().AnswerTimerWaiting {
		t.Error("CA12 (son désactivé) : le chronomètre doit démarrer immédiatement — ANSWER_TIMER_WAITING doit repasser à false")
	}
}

func TestQuestionSoundAdapter_CA12_NeutralPlayer_ReleasesImmediately(t *testing.T) {
	t.Parallel()
	// Vrai constructeur audio.NewMediaPlayer, sans double : dans ce
	// sandbox (confirmé par la suite Batch 0, internal/audio) il dégrade de
	// façon fiable vers l'implémentation neutre — exactement le cas "enceinte
	// indisponible" de CA12, testé avec le VRAI chemin de dégradation plutôt
	// qu'un faux-semblant.
	neutral := audio.NewMediaPlayer(audio.OutputConfig{}, nil)
	if !audio.IsNeutralMedia(neutral) {
		t.Skip("cette machine dispose d'un vrai backend audio — le cas neutre n'est pas exerçable ici (voir CA12/sound-disabled et CA12/unreadable-file pour les deux autres dégradations, couvertes indépendamment du matériel)")
	}
	app, q := tw219qDeferredApp(t, neutral)
	defer app.engine.Stop()

	tw219qSetSoundEnabled(true)
	app.qsound.Start(q)

	if app.engine.GetState().AnswerTimerWaiting {
		t.Error("CA12 (lecteur neutre) : le chronomètre doit démarrer immédiatement — ANSWER_TIMER_WAITING doit repasser à false")
	}
}

func TestQuestionSoundAdapter_CA12_UnreadableFile_ReleasesImmediately(t *testing.T) {
	t.Parallel()
	fake := &tw219qFakeMediaPlayer{playErr: assertErr("fichier illisible")}
	app, q := tw219qDeferredApp(t, fake)
	defer app.engine.Stop()

	tw219qSetSoundEnabled(true)
	app.qsound.Start(q)

	if fake.playCount() != 1 {
		t.Errorf("CA12 (fichier illisible) : Play() doit être tenté une fois, got %d appels", fake.playCount())
	}
	if app.engine.GetState().AnswerTimerWaiting {
		t.Error("CA12 (fichier illisible) : le chronomètre doit démarrer immédiatement — ANSWER_TIMER_WAITING doit repasser à false")
	}
	if app.engine.GetState().QuestionSoundState != game.QuestionSoundIdle {
		t.Errorf("CA12 (fichier illisible) : QUESTION_SOUND_STATE doit revenir à IDLE après l'échec, got %q", app.engine.GetState().QuestionSoundState)
	}
}

// assertErr is a trivial error constructor local to this file (avoids
// importing "errors" for a single one-liner).
type simpleErr string

func (e simpleErr) Error() string { return string(e) }
func assertErr(s string) error    { return simpleErr(s) }

// ---------------------------------------------------------------------------
// Lecture réussie — pas de libération avant la fin naturelle.
// ---------------------------------------------------------------------------

func TestQuestionSoundAdapter_SuccessfulPlay_WaitsForNaturalEnd(t *testing.T) {
	t.Parallel()
	fake := &tw219qFakeMediaPlayer{}
	app, q := tw219qDeferredApp(t, fake)
	defer app.engine.Stop()

	tw219qSetSoundEnabled(true)
	app.qsound.Start(q)

	if fake.playCount() != 1 {
		t.Fatalf("Play() doit être appelé une fois, got %d", fake.playCount())
	}
	if !app.engine.GetState().AnswerTimerWaiting {
		t.Error("une lecture qui démarre réellement ne doit PAS libérer le chronomètre immédiatement")
	}
	if app.engine.GetState().QuestionSoundState != game.QuestionSoundPlaying {
		t.Errorf("QUESTION_SOUND_STATE doit passer à PLAYING, got %q", app.engine.GetState().QuestionSoundState)
	}

	// Fin naturelle simulée (le vrai backend oto appellerait ceci depuis sa
	// goroutine de surveillance — media_oto.go).
	app.qsound.onNaturalEnd()

	if app.engine.GetState().AnswerTimerWaiting {
		t.Error("onNaturalEnd doit libérer le chronomètre (contract §10.7 « fin naturelle »)")
	}
	if app.engine.GetState().QuestionSoundState != game.QuestionSoundIdle {
		t.Errorf("QUESTION_SOUND_STATE doit repasser à IDLE après la fin naturelle, got %q", app.engine.GetState().QuestionSoundState)
	}
}

// TestQuestionSoundAdapter_ManualStop_AlsoReleases proves the OTHER
// "fin du son" event (§10.7: "la fin naturelle ET l'arrêt manuel") also
// releases, and that both firing (natural end already happened, then a
// manual Stop arrives) is a harmless idempotent no-op — the engine's own
// idempotence, exercised through the adapter's real call chain.
func TestQuestionSoundAdapter_ManualStop_AlsoReleases_ThenIdempotent(t *testing.T) {
	t.Parallel()
	fake := &tw219qFakeMediaPlayer{}
	app, q := tw219qDeferredApp(t, fake)
	defer app.engine.Stop()

	tw219qSetSoundEnabled(true)
	app.qsound.Start(q)

	app.qsound.Stop() // geste manuel "Stop"
	if app.engine.GetState().AnswerTimerWaiting {
		t.Error("Stop() manuel doit libérer le chronomètre (contract §10.7 « arrêt manuel »)")
	}

	// Un second Stop() (ou une fin naturelle tardive qui arriverait après
	// coup) ne doit rien casser ni faire redémarrer quoi que ce soit deux
	// fois — idempotence de l'engine, déjà verrouillée côté moteur
	// (internal/game/deferred_timer_219_test.go), ré-exercée ici via le
	// VRAI chemin d'appel de l'adaptateur.
	app.qsound.Stop()
	app.qsound.onNaturalEnd()
	if app.engine.GetState().AnswerTimerWaiting {
		t.Error("des appels de libération redondants ne doivent jamais re-lever ANSWER_TIMER_WAITING")
	}
}

// ---------------------------------------------------------------------------
// CA13 — "Rejouer" (HandleCommand PLAY) ne touche JAMAIS le chronomètre,
// succès ou échec.
// ---------------------------------------------------------------------------

func TestQuestionSoundAdapter_CA13_Rejouer_NeverReleasesTimer_OnSuccess(t *testing.T) {
	t.Parallel()
	fake := &tw219qFakeMediaPlayer{}
	app, _ := tw219qDeferredApp(t, fake)
	defer app.engine.Stop()

	tw219qSetSoundEnabled(true)
	// Le "Rejouer" manuel (HandleCommand PLAY), PAS Start() — c'est
	// exactement la voie releaseOnFailure=false de play().
	app.qsound.HandleCommand(protocol.QuestionSoundCommandPlay)

	if fake.playCount() != 1 {
		t.Fatalf("Rejouer doit appeler Play() une fois, got %d", fake.playCount())
	}
	if !app.engine.GetState().AnswerTimerWaiting {
		t.Error("CA13 : Rejouer ne doit jamais libérer le chronomètre, même quand la lecture réussit")
	}
}

func TestQuestionSoundAdapter_CA13_Rejouer_NeverReleasesTimer_OnFailure(t *testing.T) {
	t.Parallel()
	fake := &tw219qFakeMediaPlayer{playErr: assertErr("fichier illisible")}
	app, _ := tw219qDeferredApp(t, fake)
	defer app.engine.Stop()

	tw219qSetSoundEnabled(true)
	app.qsound.HandleCommand(protocol.QuestionSoundCommandPlay)

	if !app.engine.GetState().AnswerTimerWaiting {
		t.Error("CA13 : Rejouer ne doit jamais libérer le chronomètre, même quand la lecture ÉCHOUE (releaseOnFailure=false pour ce chemin)")
	}
}

func TestQuestionSoundAdapter_CA13_Rejouer_NoOp_WhenQuestionCarriesNoSound(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	fake := &tw219qFakeMediaPlayer{}
	app.qsound = &questionSoundAdapter{app: app, player: fake}
	app.engine.Ready("q1", &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy, Answer: "x"})

	app.qsound.HandleCommand(protocol.QuestionSoundCommandPlay)

	if fake.playCount() != 0 {
		t.Error("HandleCommand(PLAY) sur une question sans son ne doit jamais appeler Play()")
	}
}

// ---------------------------------------------------------------------------
// Chien de garde (§10.7 : "un chien de garde libère le chronomètre au plus
// tard à MaxQuestionSoundDuration + 2s") — test lent (~32s), gated
// -short, même convention que le reste du dépôt (ex.
// internal/server/ai_validate_test.go, cmd/server/http_port_retry_test.go).
// ---------------------------------------------------------------------------

func TestQuestionSoundAdapter_Watchdog_ReleasesAfterGraceIfNoSignalEver(t *testing.T) {
	if testing.Short() {
		t.Skip("lent (~32s, MaxQuestionSoundDuration+2s) — sauté en -short")
	}
	t.Parallel()
	// Un fake dont Play() réussit mais qui ne signale JAMAIS de fin
	// (naturelle) — simule une goroutine de surveillance morte ou un pilote
	// figé (§10.7).
	fake := &tw219qFakeMediaPlayer{}
	app, q := tw219qDeferredApp(t, fake)
	defer app.engine.Stop()

	tw219qSetSoundEnabled(true)
	app.qsound.Start(q)
	if !app.engine.GetState().AnswerTimerWaiting {
		t.Fatal("setup invalide : le chronomètre devrait encore attendre juste après Start()")
	}

	deadline := time.Now().Add(audio.MaxQuestionSoundDuration + 2*time.Second + 3*time.Second)
	for time.Now().Before(deadline) {
		if !app.engine.GetState().AnswerTimerWaiting {
			return // le chien de garde a bien libéré le chronomètre
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Errorf("le chien de garde n'a pas libéré le chronomètre après %s (MaxQuestionSoundDuration+2s+marge)", audio.MaxQuestionSoundDuration+2*time.Second+3*time.Second)
}

func TestQuestionSoundAdapter_Watchdog_CancelledByStop(t *testing.T) {
	t.Parallel()
	fake := &tw219qFakeMediaPlayer{}
	app, q := tw219qDeferredApp(t, fake)
	defer app.engine.Stop()

	tw219qSetSoundEnabled(true)
	app.qsound.Start(q)
	app.qsound.mu.Lock()
	armed := app.qsound.watchdogCancel != nil
	app.qsound.mu.Unlock()
	if !armed {
		t.Fatal("setup invalide : le chien de garde devrait être armé après un Play() réussi")
	}

	app.qsound.Stop()
	app.qsound.mu.Lock()
	cancelledAfterStop := app.qsound.watchdogCancel == nil
	app.qsound.mu.Unlock()
	if !cancelledAfterStop {
		t.Error("Stop() doit annuler le chien de garde en cours (cancelWatchdogLocked)")
	}
}

// ---------------------------------------------------------------------------
// R5 — reprise après PAUSE et avance RAFALE ne doivent jamais redéclencher
// le son : onPhaseStarted réutilise EXACTEMENT la même garde isRealStart
// que le fan-out des cues (notifySound(CueDepart)) — testé ici directement,
// sans besoin de countdown réel, en appelant onPhaseStarted avec chaque
// previousPhase possible.
// ---------------------------------------------------------------------------

func TestOnPhaseStarted_QuestionSound_OnlyStartsOnRealStart(t *testing.T) {
	tests := []struct {
		name          string
		previousPhase game.GamePhase
		wantPlay      bool
	}{
		{"depuis COUNTDOWN (vrai départ)", game.PhaseCountdown, true},
		{"depuis PREPARE (vrai départ)", game.PhasePrepare, true},
		{"depuis READY (vrai départ)", game.PhaseReady, true},
		{"depuis PAUSED (reprise, pas un vrai départ)", game.PhasePaused, false},
		{"depuis STARTED (ré-émission d'avance RAFALE, R5)", game.PhaseStarted, false},
		{"depuis STOPPED (jamais un départ)", game.PhaseStopped, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newTestApp(t)
			fake := &tw219qFakeMediaPlayer{}
			app.qsound = &questionSoundAdapter{app: app, player: fake}
			tw219qSetSoundEnabled(true)
			app.engine.Ready("q1", &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy, Answer: "x", Sound: "/question/q1/sound_1.wav"})

			app.onPhaseStarted(tt.previousPhase)

			gotPlay := fake.playCount() > 0
			if gotPlay != tt.wantPlay {
				t.Errorf("onPhaseStarted(previousPhase=%s) : Play() appelé=%v, attendu %v (R5, même garde isRealStart que le fan-out des cues)", tt.previousPhase, gotPlay, tt.wantPlay)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Frontière normative (contract §10.5) — test complémentaire à
// sound_sites_test.go (#227, qui ne scanne que le sélecteur "notifySound") :
// cmd/server/question_sound.go ne doit JAMAIS appeler notifySound NI PlayCue.
// ---------------------------------------------------------------------------

func TestQuestionSoundAdapter_NeverCallsNotifySoundOrPlayCue(t *testing.T) {
	path := filepath.Join(cmdServerDir227(t), "question_sound.go")

	// notifySound — même mécanique AST que sound_sites_test.go
	// (scanSoundSitePairs227, dédié à ce seul sélecteur), appliquée au seul
	// fichier question_sound.go.
	found := scanSoundSitePairs227(t, path, nil)
	if len(found) != 0 {
		t.Errorf("contract §10.5 violé — question_sound.go appelle notifySound dans : %v (les deux chemins audio ne doivent jamais s'appeler l'un l'autre)", found)
	}

	// PlayCue — second scan, même mécanique AST générique, ciblant cet
	// autre sélecteur (audio.Engine.PlayCue) que scanSoundSitePairs227 ne
	// cherche délibérément pas (son propre commentaire : "UN SEUL nom
	// recherché").
	foundPlayCue := tw219qScanFileForSelector(t, path, "PlayCue")
	if len(foundPlayCue) != 0 {
		t.Errorf("contract §10.5 violé — question_sound.go appelle PlayCue dans : %v", foundPlayCue)
	}
}

// tw219qScanFileForSelector parses the given file and returns the set of
// enclosing-function names that call `selector` anywhere in their syntax
// tree — same AST technique as scanSoundSitePairs227
// (cmd/server/sound_sites_test.go), parameterised on the selector name
// instead of that function's hard-coded "notifySound" (its own doc comment:
// "UN SEUL nom recherché" — deliberately single-purpose, so this is a
// parallel implementation, not a modification of it).
func tw219qScanFileForSelector(t *testing.T, path, selector string) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("go/parser a échoué sur %s : %v", path, err)
	}
	found := map[string]bool{}
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		enclosing := fd.Name.Name
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			var name string
			switch fn := call.Fun.(type) {
			case *ast.SelectorExpr:
				name = fn.Sel.Name
			case *ast.Ident:
				name = fn.Name
			default:
				return true
			}
			if name == selector {
				found[enclosing] = true
			}
			return true
		})
	}
	return found
}
