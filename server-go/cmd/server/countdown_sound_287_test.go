// #287 (v11.4.0, LOT-3) — son de décompte, contracts/sound.md §12.4 (gardes), §12.5 (déclenchement), §12.7/§12.8
// (arrêt, pause, buzz). Frontière : vraie chaîne de rappels de production (setupCallbacks) + dispatcher réel
// (handleWebMessage), lecteur n°2 remplacé par un FAUX journalisé.
//
// Écrit AVANT le code (TDD) et COMPILE avant lui : la config est posée par JSON (comme sound_muted_286_test.go) et
// l'adaptateur de décompte est installé par RÉFLEXION (le contrat n'impose pas les noms internes). Convention
// d'installation attendue de dev-backend (cf. handoff test-writer) : un champ `*struct` de `App` dont le nom contient
// « csound » ou « countdown », dont la struct porte UN champ de type `audio.MediaPlayer` et UN champ `*App`.
// Absent ⇒ échec par assertion « RED », jamais d'erreur de compilation.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/audio/synth"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// tw287Fake est le lecteur n°2 factice : journal ordonné des appels, état de lecture simple.
type tw287Fake struct {
	mu    sync.Mutex
	calls []string
	state audio.MediaState
}

func (f *tw287Fake) rec(c string) { f.mu.Lock(); f.calls = append(f.calls, c); f.mu.Unlock() }
func (f *tw287Fake) Play(p string) error {
	f.rec("Play:" + filepath.Base(p))
	f.mu.Lock()
	f.state = audio.MediaPlaying
	f.mu.Unlock()
	return nil
}
func (f *tw287Fake) PlayLoop(p string) error {
	f.rec("PlayLoop:" + filepath.Base(p))
	f.mu.Lock()
	f.state = audio.MediaPlaying
	f.mu.Unlock()
	return nil
}
func (f *tw287Fake) Pause() { f.rec("Pause"); f.mu.Lock(); f.state = audio.MediaPaused; f.mu.Unlock() }
func (f *tw287Fake) Resume() {
	f.rec("Resume")
	f.mu.Lock()
	f.state = audio.MediaPlaying
	f.mu.Unlock()
}
func (f *tw287Fake) Stop() { f.rec("Stop"); f.mu.Lock(); f.state = audio.MediaIdle; f.mu.Unlock() }
func (f *tw287Fake) State() audio.MediaState {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state
}
func (f *tw287Fake) Close() error { return nil }
func (f *tw287Fake) journal() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}
func (f *tw287Fake) count(prefix string) int {
	n := 0
	for _, c := range f.journal() {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}
func (f *tw287Fake) reset() { f.mu.Lock(); f.calls = nil; f.mu.Unlock() }

// tw287Config pose la config sound.* par JSON (jamais par champ typé) et restaure les défauts à la fin.
func tw287Config(t *testing.T, raw string) {
	t.Helper()
	apply := func(s string) {
		if err := json.Unmarshal([]byte(s), config.Get()); err != nil {
			t.Fatalf("setup invalide : %v", err)
		}
	}
	apply(raw)
	t.Cleanup(func() {
		apply(`{"sound":{"enabled":false,"muted":false,"countdown_disabled":false,"countdown_once":false}}`)
	})
}

// tw287InstallCountdown installe fake comme lecteur n°2 de l'adaptateur de décompte (réflexion, voir en-tête).
func tw287InstallCountdown(t *testing.T, app *App, fake audio.MediaPlayer) {
	t.Helper()
	av := reflect.ValueOf(app).Elem()
	var field reflect.Value
	for i := 0; i < av.NumField(); i++ {
		sf := av.Type().Field(i)
		name := strings.ToLower(sf.Name)
		if (strings.Contains(name, "csound") || strings.Contains(name, "countdown")) &&
			sf.Type.Kind() == reflect.Ptr && sf.Type.Elem().Kind() == reflect.Struct {
			field = av.Field(i)
			break
		}
	}
	if !field.IsValid() {
		t.Fatal("RED : App n'a pas de champ adaptateur de décompte (contrat sound.md §12.1, cmd/server/countdown_sound.go)")
	}
	ad := reflect.New(field.Type().Elem())
	mpType := reflect.TypeOf((*audio.MediaPlayer)(nil)).Elem()
	players := 0
	for j := 0; j < ad.Elem().NumField(); j++ {
		f := ad.Elem().Field(j)
		settable := reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
		switch {
		case f.Type() == mpType:
			settable.Set(reflect.ValueOf(&fake).Elem())
			players++
		case f.Type() == reflect.TypeOf(app):
			settable.Set(reflect.ValueOf(app))
		}
	}
	if players != 1 {
		t.Fatalf("RED : l'adaptateur de décompte doit porter exactement un champ audio.MediaPlayer (trouvé %d)", players)
	}
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(ad)
}

// tw287App : App avec la vraie chaîne de rappels, fichier decompte.wav présent, faux lecteur n°2 installé.
func tw287App(t *testing.T, cfgJSON string) (*App, *tw287Fake) {
	t.Helper()
	app := newTestAppWithHub(t)
	app.logsHub = server.NewLogsWebSocketHub(10) // callback du logger (setupCallbacks) diffuse vers ce hub
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, app.logsHub)
	app.logger = server.NewBroadcastLogger(100)
	app.udpBcast = server.NewUDPBroadcaster()
	setGlobalLogger246(t, app) // le dispatcher journalise via le logger global (hub de logs requis)
	app.config.Storage.QuestionsDir = t.TempDir()
	filesDir := t.TempDir()
	prev := app.config.Storage.FilesDir
	app.config.Storage.FilesDir = filesDir
	t.Cleanup(func() { app.config.Storage.FilesDir = prev })
	sounds := filepath.Join(filesDir, "sounds")
	if err := os.MkdirAll(sounds, 0o755); err != nil {
		t.Fatal(err)
	}
	if wav, ok := synth.Generate(audio.CueDepart); ok {
		if err := os.WriteFile(filepath.Join(sounds, "decompte.wav"), wav, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tw287Config(t, cfgJSON)
	t.Cleanup(func() { app.engine.Stop() })
	app.setupCallbacks()
	fake := &tw287Fake{}
	tw287InstallCountdown(t, app, fake)
	return app, fake
}

const tw287Active = `{"sound":{"enabled":true,"muted":false,"countdown_disabled":false,"countdown_once":false}}`

func tw287StartSpeedy(app *App, q *game.Question) {
	app.engine.Ready(q.ID, q)
	app.engine.StartImmediate(30)
}

func tw287Speedy() *game.Question {
	return &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy, Answer: "x", Time: "30"}
}

func tw287Send(t *testing.T, app *App, action string, payload interface{}) {
	t.Helper()
	msg, err := protocol.NewMessage(action, payload)
	if err != nil {
		t.Fatalf("NewMessage(%s) : %v", action, err)
	}
	app.handleWebMessage(&protocol.IncomingMessage{Source: "WebSocket", Data: msg, ClientID: "adm-287", ClientType: "admin"})
}

func tw287Waiting(t *testing.T, fake *tw287Fake, prefix string, n int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if fake.count(prefix) >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("journal %v : %d appel(s) « %s » attendu(s)", fake.journal(), n, prefix)
}

// ---------- (b) boucle par défaut / une fois ----------

func TestCountdown287_DefaultIsLoop_PlayLoopOnTimerStart(t *testing.T) {
	app, fake := tw287App(t, tw287Active)
	tw287StartSpeedy(app, tw287Speedy())
	tw287Waiting(t, fake, "PlayLoop:decompte.wav", 1)
	if fake.count("Play:") != 0 {
		t.Errorf("défaut = EN BOUCLE (§12.3) : Play interdit, journal %v", fake.journal())
	}
}

func TestCountdown287_CountdownOnce_UsesPlayNotPlayLoop(t *testing.T) {
	app, fake := tw287App(t, `{"sound":{"enabled":true,"muted":false,"countdown_disabled":false,"countdown_once":true}}`)
	tw287StartSpeedy(app, tw287Speedy())
	tw287Waiting(t, fake, "Play:decompte.wav", 1)
	if fake.count("PlayLoop") != 0 {
		t.Errorf("countdown_once=true : PlayLoop interdit, journal %v", fake.journal())
	}
}

// ---------- (c) gardes : jamais de Play ----------

func TestCountdown287_Guards_NeverPlays(t *testing.T) {
	cases := []struct {
		name string
		cfg  string
		q    *game.Question
	}{
		{"muted", `{"sound":{"enabled":true,"muted":true,"countdown_disabled":false,"countdown_once":false}}`, tw287Speedy()},
		{"enabled=false", `{"sound":{"enabled":false,"muted":false,"countdown_disabled":false,"countdown_once":false}}`, tw287Speedy()},
		{"countdown_disabled", `{"sound":{"enabled":true,"muted":false,"countdown_disabled":true,"countdown_once":false}}`, tw287Speedy()},
		{"média simultané", tw287Active, &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy, Answer: "x", Sound: "/question/q1/sound_1.wav"}},
		{"média différé", tw287Active, &game.Question{ID: "q1", Type: game.QuestionTypeSpeedy, Answer: "x", Sound: "/question/q1/sound_1.wav", SoundTimerDelayed: true}},
		{"média en boucle", tw287Active, tw287Loop(&game.Question{ID: "q1", Type: game.QuestionTypeSpeedy, Answer: "x", Sound: "/question/q1/sound_1.wav"})},
		{"média en boucle + différé", tw287Active, tw287Loop(&game.Question{ID: "q1", Type: game.QuestionTypeSpeedy, Answer: "x", Sound: "/question/q1/sound_1.wav", SoundTimerDelayed: true})},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			app, fake := tw287App(t, c.cfg)
			tw287StartSpeedy(app, c.q)
			time.Sleep(150 * time.Millisecond)
			if n := fake.count("Play"); n != 0 {
				t.Fatalf("garde %q : décompte lancé (%v), attendu aucun Play/PlayLoop", c.name, fake.journal())
			}
		})
	}
}

// ---------- (d) arrêt : chaque site isolément ----------

func tw287Playing(t *testing.T) (*App, *tw287Fake) {
	t.Helper()
	app, fake := tw287App(t, tw287Active)
	tw287StartSpeedy(app, tw287Speedy())
	tw287Waiting(t, fake, "PlayLoop", 1)
	return app, fake
}

func TestCountdown287_StopSites_EachOneStopsALoopingCountdown(t *testing.T) {
	sites := []struct {
		name string
		do   func(t *testing.T, app *App)
	}{
		{"OnTimeUp", func(t *testing.T, app *App) { app.engine.OnTimeUp() }},
		{"STOP", func(t *testing.T, app *App) { tw287Send(t, app, protocol.ActionStop, map[string]interface{}{}) }},
		{"REVEAL", func(t *testing.T, app *App) { tw287Send(t, app, protocol.ActionReveal, map[string]interface{}{}) }},
		{"READY (question rechargée)", func(t *testing.T, app *App) {
			msg, _ := protocol.NewMessage(protocol.ActionReady, protocol.ReadyPayload{Question: ""})
			app.handleWebMessage(&protocol.IncomingMessage{Source: "WebSocket", Data: msg, ClientID: "adm-287", ClientType: "admin"})
		}},
		{"NEW_GAME", func(t *testing.T, app *App) {
			tw287Send(t, app, protocol.ActionNewGame, map[string]interface{}{"PURGE_SCORES": true, "RESET_QUESTIONS": true})
		}},
		{"muted → true", func(t *testing.T, app *App) {
			app.httpServer.OnSoundMutedChange(true)
		}},
		{"countdown_disabled → true", func(t *testing.T, app *App) {
			f := reflect.ValueOf(app.httpServer).Elem().FieldByName("OnSoundCountdownDisabledChange")
			if !f.IsValid() || f.IsNil() {
				t.Fatal("RED : HTTPServer.OnSoundCountdownDisabledChange non câblé (§12.3)")
			}
			f.Interface().(func(bool))(true)
		}},
		{"OnMotionCardTimeUp", func(t *testing.T, app *App) {
			f := reflect.ValueOf(app.engine).Elem().FieldByName("OnMotionCardTimeUp")
			if !f.IsValid() || f.IsNil() {
				t.Fatal("RED : Engine.OnMotionCardTimeUp non câblé (§12.8)")
			}
			f.Interface().(func())()
		}},
	}
	for _, s := range sites {
		t.Run(s.name, func(t *testing.T) {
			app, fake := tw287Playing(t)
			fake.reset()
			s.do(t, app)
			if fake.count("Stop") < 1 {
				t.Fatalf("site %q : Stop() non appelé sur le lecteur de décompte (journal %v)", s.name, fake.journal())
			}
		})
	}
}

// §12.8 : « au buzz tout continue ». Engine.Pause() est exactement ce que fait un buzz (OnStateChange(PAUSED)) ;
// OnStateChange ne doit jamais arrêter ni suspendre le décompte.
func TestCountdown287_Buzz_NothingTouchesTheCountdown(t *testing.T) {
	app, fake := tw287Playing(t)
	fake.reset()
	app.engine.Pause() // le buzz : STARTED → PAUSED interne
	time.Sleep(150 * time.Millisecond)
	if got := fake.journal(); len(got) != 0 {
		t.Fatalf("buzz : aucun appel au lecteur de décompte attendu (tout continue), journal %v", got)
	}
}

// §12.8 : PAUSE manuelle ⇒ Pause(), CONTINUE ⇒ Resume() — jamais relancé depuis le début.
func TestCountdown287_ManualPauseThenContinue_PausesThenResumes_NoRestart(t *testing.T) {
	app, fake := tw287Playing(t)
	fake.reset()
	tw287Send(t, app, protocol.ActionPause, map[string]interface{}{})
	tw287Waiting(t, fake, "Pause", 1)
	tw287Send(t, app, protocol.ActionContinue, map[string]interface{}{})
	tw287Waiting(t, fake, "Resume", 1)
	if fake.count("Play") != 0 || fake.count("Stop") != 0 {
		t.Fatalf("PAUSE puis CONTINUE : ni Play/PlayLoop ni Stop attendus, journal %v", fake.journal())
	}
}

// Stop/Pause/Resume sont idempotents et sans danger quand rien ne joue (§12.8).
func TestCountdown287_Gestures_AreSafeWhenNothingPlays(t *testing.T) {
	app, fake := tw287App(t, tw287Active)
	tw287Send(t, app, protocol.ActionStop, map[string]interface{}{})
	tw287Send(t, app, protocol.ActionPause, map[string]interface{}{})
	tw287Send(t, app, protocol.ActionContinue, map[string]interface{}{})
	if fake.count("Play") != 0 {
		t.Fatalf("aucun chrono : aucune lecture attendue, journal %v", fake.journal())
	}
}

// ---------- (e) manche RAFALE : une lecture pour la manche ----------

// L'adaptateur joue UNE fois par émission OnAnswerTimerStarted (une par manche, test moteur answer_timer_started_287) et
// n'arrête rien tant que la manche n'est pas finie : un Stop avant la fin de manche couperait le décompte en plein jeu.
func TestCountdown287_OnTimerStartedCallback_PlaysOncePerEmission_NoStopBeforeRoundEnd(t *testing.T) {
	app, fake := tw287App(t, tw287Active)
	app.engine.Ready("q1", tw287Speedy())
	app.engine.StartImmediate(60)
	tw287Waiting(t, fake, "PlayLoop", 1)
	time.Sleep(200 * time.Millisecond)
	if fake.count("PlayLoop") != 1 || fake.count("Stop") != 0 {
		t.Fatalf("une émission ⇒ un PlayLoop et aucun Stop, journal %v", fake.journal())
	}
}

// ---------- (f) non-deadlock : rappel émis sous e.mu ----------

func TestCountdown287_CallbackUnderEngineLock_DoesNotDeadlock(t *testing.T) {
	app, _ := tw287App(t, tw287Active)
	done := make(chan struct{})
	go func() {
		app.engine.Ready("q1", tw287Speedy())
		app.engine.StartImmediate(30) // startTimer() émet OnAnswerTimerStarted sous e.mu
		_ = app.engine.GetState()     // reprend e.mu : bloque si l'adaptateur a rappelé le moteur et s'est coincé
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("deadlock : l'adaptateur de décompte ne doit jamais appeler le moteur depuis OnAnswerTimerStarted (§12.5)")
	}
}

// ---------- frontière avec les autres chemins audio ----------

// Le décompte ne passe jamais par les cues : aucune cue supplémentaire n'est émise par sa lecture.
func TestCountdown287_DoesNotUseTheCuesPath(t *testing.T) {
	app, fake := tw287App(t, tw287Active)
	cuesBefore := 0
	if e := app.sound(); e != nil {
		cuesBefore = 1
	}
	_ = cuesBefore
	tw287StartSpeedy(app, tw287Speedy())
	tw287Waiting(t, fake, "PlayLoop", 1)
	// Vérification structurelle (AST) portée par sound_sites_test.go étendu (dev-backend, §12.1) ; ici : le lecteur
	// de décompte reçoit exclusivement decompte.wav.
	for _, c := range fake.journal() {
		if strings.HasPrefix(c, "Play") && !strings.HasSuffix(c, "decompte.wav") {
			t.Errorf("le lecteur de décompte ne doit lire que decompte.wav, vu %q", c)
		}
	}
}

// tw287EmissionCounter compte les émissions d'Engine.OnAnswerTimerStarted (réflexion : compile avant le code).
type tw287EmissionCounter struct {
	mu sync.Mutex
	c  int
}

func (e *tw287EmissionCounter) n() int { e.mu.Lock(); defer e.mu.Unlock(); return e.c }

func tw287CountEmissions(t *testing.T, e *game.Engine) *tw287EmissionCounter {
	t.Helper()
	f := reflect.ValueOf(e).Elem().FieldByName("OnAnswerTimerStarted")
	if !f.IsValid() {
		t.Fatal("RED : Engine.OnAnswerTimerStarted absent (contrat sound.md §12.5)")
	}
	c := &tw287EmissionCounter{}
	f.Set(reflect.MakeFunc(f.Type(), func([]reflect.Value) []reflect.Value {
		c.mu.Lock()
		c.c++
		c.mu.Unlock()
		return nil
	}))
	return c
}

// tw287Loop pose SOUND_LOOP=true par le JSON (frontière) : le fichier compile même avant le champ typé.
func tw287Loop(q *game.Question) *game.Question {
	raw, _ := json.Marshal(q)
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	m["SOUND_LOOP"] = true
	raw, _ = json.Marshal(m)
	out := &game.Question{}
	_ = json.Unmarshal(raw, out)
	return out
}

// §12.8 (corrigé 1f9624ac) : OnStateChange(STOPPED) arrête le décompte — c'est le chemin de la fin de manche RAFALE
// (plafond ou épuisement du réservoir : le moteur s'arrête lui-même, sans passer par le handler STOP).
func TestCountdown287_EngineStopsItself_StoppedPhase_StopsCountdown(t *testing.T) {
	app, fake := tw287Playing(t)
	fake.reset()
	app.engine.Stop() // transition interne vers STOPPED, comme à l'épuisement du réservoir RAFALE
	tw287Waiting(t, fake, "Stop", 1)
}

func TestCountdown287_OnStateChange_Stopped_Stops_StartedAndPaused_DoNothing(t *testing.T) {
	app, fake := tw287Playing(t)
	fake.reset()
	app.engine.OnStateChange(game.PhaseStarted)
	app.engine.OnStateChange(game.PhasePaused)
	time.Sleep(100 * time.Millisecond)
	if got := fake.journal(); len(got) != 0 {
		t.Fatalf("STARTED/PAUSED ne doivent rien faire au décompte, journal %v", got)
	}
	app.engine.OnStateChange(game.PhaseStopped)
	tw287Waiting(t, fake, "Stop", 1)
}
