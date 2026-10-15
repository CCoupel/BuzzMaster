package server

import (
	"buzzcontrol/internal/game"
	"io"
	"log"
	"strings"
	"sync"
	"testing"
)

// #269 — data race sur BroadcastLogger.debugEnabled.
//
// Cause racine (voir _work/handoff/planner-20261013-102100.md §#269) : un timer
// résiduel d'un test précédent (coalescer ARDOISE) appelle LogDebug sur le
// logger GLOBAL pendant que le test suivant (Instrumentation246) appelle
// SetDebugEnabled sur ce même logger. `debugEnabled` est un bool nu lu dans
// log() et écrit dans SetDebugEnabled : data race détectée par `-race`.
//
// Contrat vérifié ici, uniquement par l'API EXISTANTE (aucun symbole nouveau) :
//   - lecture (Debug / LogDebug) et écriture (SetDebugEnabled) concurrentes du
//     niveau debug : AUCUNE data race — RED sous `-race` tant que le champ n'est
//     pas protégé (atomic.Bool), vert après le fix ;
//   - la sémantique du niveau est inchangée (désactivé par défaut, activable,
//     désactivable, les autres niveaux jamais filtrés) — vert avant et après.
//
// Commande : CGO_ENABLED=1 go test -race -count=5 -run 'Debug269' ./internal/server

// silenceStdLog évite que log.Printf (appelé à chaque entrée acceptée) noie la sortie.
func silenceStdLog(t *testing.T) {
	t.Helper()
	prev := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(prev) })
}

// swapGlobalLogger installe bl comme logger global le temps du test et restaure l'ancien.
func swapGlobalLogger(t *testing.T, bl *BroadcastLogger) {
	t.Helper()
	prev := GetGlobalLogger()
	SetGlobalLogger(bl)
	t.Cleanup(func() { SetGlobalLogger(prev) })
}

func countMessages(bl *BroadcastLogger, level game.LogLevel, sub string) int {
	n := 0
	for _, e := range bl.GetHistory() {
		if e.Level == level && strings.Contains(e.Message, sub) {
			n++
		}
	}
	return n
}

// --- Reproduction de la data race (RED sous -race avant le fix) -------------

// Un écrivain bascule le niveau debug pendant que des lecteurs journalisent en DEBUG.
func TestDebug269_ConcurrentToggleAndDebugLogIsRaceFree(t *testing.T) {
	silenceStdLog(t)
	bl := NewBroadcastLogger(64)

	const iterations = 500
	const readers = 4
	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(1)
	go func() { // écrivain : équivalent du SetDebugEnabled de Instrumentation246
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			bl.SetDebugEnabled(i%2 == 0)
		}
	}()
	for r := 0; r < readers; r++ { // lecteurs : équivalent du timer résiduel du coalescer
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				bl.Debug(game.LogComponentApp, "residual timer %d", i)
			}
		}()
	}
	close(start)
	wg.Wait()
}

// Même scénario par le chemin réel de l'issue : le logger GLOBAL, via LogDebug.
func TestDebug269_GlobalLoggerLogDebugVersusSetDebugEnabledIsRaceFree(t *testing.T) {
	silenceStdLog(t)
	bl := NewBroadcastLogger(64)
	swapGlobalLogger(t, bl)

	const iterations = 500
	var wg sync.WaitGroup
	start := make(chan struct{})

	wg.Add(2)
	go func() { // « test suivant »
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			GetGlobalLogger().SetDebugEnabled(i%3 != 0)
		}
	}()
	go func() { // « timer survivant du test précédent »
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			LogDebug(game.LogComponentWebSocket, "ardoise coalescer flush %d", i)
		}
	}()
	close(start)
	wg.Wait()
}

// Écritures concurrentes entre elles (deux tests/goroutines qui basculent le niveau) + lecteurs mixtes.
func TestDebug269_ConcurrentWritersAndMixedLevelsAreRaceFree(t *testing.T) {
	silenceStdLog(t)
	bl := NewBroadcastLogger(64)

	const iterations = 300
	var wg sync.WaitGroup
	start := make(chan struct{})

	for w := 0; w < 3; w++ {
		w := w
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				bl.SetDebugEnabled((i+w)%2 == 0)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			bl.Info(game.LogComponentApp, "info %d", i)
			bl.Warn(game.LogComponentApp, "warn %d", i)
			bl.Error(game.LogComponentApp, "error %d", i)
			bl.Debug(game.LogComponentApp, "debug %d", i)
		}
	}()
	close(start)
	wg.Wait()
}

// --- Sémantique du niveau debug (verte avant ET après le fix) ---------------

func TestDebug269_DebugDisabledByDefault(t *testing.T) {
	silenceStdLog(t)
	bl := NewBroadcastLogger(16)

	bl.Debug(game.LogComponentApp, "should-be-filtered")

	if n := countMessages(bl, game.LogLevelDebug, "should-be-filtered"); n != 0 {
		t.Fatalf("DEBUG doit être filtré par défaut, %d entrée(s) trouvée(s)", n)
	}
}

func TestDebug269_EnableThenDisableDebugIsHonoured(t *testing.T) {
	silenceStdLog(t)
	bl := NewBroadcastLogger(16)

	bl.SetDebugEnabled(true)
	bl.Debug(game.LogComponentApp, "visible-while-enabled")
	bl.SetDebugEnabled(false)
	bl.Debug(game.LogComponentApp, "hidden-after-disable")

	if n := countMessages(bl, game.LogLevelDebug, "visible-while-enabled"); n != 1 {
		t.Fatalf("DEBUG activé : 1 entrée attendue, obtenu %d", n)
	}
	if n := countMessages(bl, game.LogLevelDebug, "hidden-after-disable"); n != 0 {
		t.Fatalf("DEBUG désactivé : 0 entrée attendue, obtenu %d", n)
	}
}

func TestDebug269_NonDebugLevelsNeverFilteredWhenDebugDisabled(t *testing.T) {
	silenceStdLog(t)
	bl := NewBroadcastLogger(16)
	bl.SetDebugEnabled(false)

	bl.Info(game.LogComponentApp, "i-269")
	bl.Warn(game.LogComponentApp, "w-269")
	bl.Error(game.LogComponentApp, "e-269")

	for _, c := range []struct {
		level game.LogLevel
		msg   string
	}{
		{game.LogLevelInfo, "i-269"},
		{game.LogLevelWarn, "w-269"},
		{game.LogLevelError, "e-269"},
	} {
		if n := countMessages(bl, c.level, c.msg); n != 1 {
			t.Fatalf("niveau %s : 1 entrée attendue même sans debug, obtenu %d", c.level, n)
		}
	}
}

// Un SetDebugEnabled terminé est visible d'une autre goroutine synchronisée après lui
// (le changement ne peut pas rester « perdu » dans un cache de lecture).
func TestDebug269_SetDebugEnabledVisibleToAnotherGoroutine(t *testing.T) {
	silenceStdLog(t)
	bl := NewBroadcastLogger(16)

	bl.SetDebugEnabled(true)
	done := make(chan struct{})
	go func() {
		defer close(done)
		bl.Debug(game.LogComponentApp, "from-other-goroutine")
	}()
	<-done

	if n := countMessages(bl, game.LogLevelDebug, "from-other-goroutine"); n != 1 {
		t.Fatalf("DEBUG activé avant le lancement de la goroutine : 1 entrée attendue, obtenu %d", n)
	}
}

// Le logger global : LogDebug respecte le niveau du logger installé, sans logger il ne panique pas.
func TestDebug269_GlobalLogDebugHonoursInstalledLoggerLevel(t *testing.T) {
	silenceStdLog(t)
	bl := NewBroadcastLogger(16)
	swapGlobalLogger(t, bl)

	LogDebug(game.LogComponentApp, "global-filtered")
	bl.SetDebugEnabled(true)
	LogDebug(game.LogComponentApp, "global-visible")

	if n := countMessages(bl, game.LogLevelDebug, "global-filtered"); n != 0 {
		t.Fatalf("LogDebug avec debug désactivé : 0 entrée attendue, obtenu %d", n)
	}
	if n := countMessages(bl, game.LogLevelDebug, "global-visible"); n != 1 {
		t.Fatalf("LogDebug avec debug activé : 1 entrée attendue, obtenu %d", n)
	}
}

func TestDebug269_GlobalLogDebugWithoutLoggerDoesNotPanic(t *testing.T) {
	silenceStdLog(t)
	swapGlobalLogger(t, nil)

	LogDebug(game.LogComponentApp, "no logger installed %d", 1)
}
