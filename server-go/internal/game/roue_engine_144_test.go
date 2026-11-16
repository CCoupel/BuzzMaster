package game

import (
	"strings"
	"sync"
	"testing"
)

func roueEngine144(t *testing.T, mode string, teams ...string) *Engine {
	t.Helper()
	e := NewEngine()
	all := map[string]*Team{"red": {Name: "red", Color: []int{255, 0, 0}}, "blue": {Name: "blue", Color: []int{0, 0, 255}}}
	e.SetTeams(all)
	q := &Question{ID: "roue1", Question: "Roue de la Fortune", Type: QuestionTypeRoue,
		TypedContent: TypedContent{RouePhrase: "Toto tata", RoueMode: mode}}
	e.Ready("roue1", q)
	if err := e.RoueSetTeams(teams); err != nil {
		t.Fatalf("RoueSetTeams: %v", err)
	}
	e.SetRoueRand(func(int) int { return 0 }) // case 0 = 1000
	e.StartImmediate(0)
	t.Cleanup(e.Stop)
	return e
}

// fireRoueTimer144 fires the armed timer synchronously (no real sleep).
func fireRoueTimer144(e *Engine) {
	e.mu.Lock()
	gen, kind := e.roue.gen, e.roue.timerKind
	e.mu.Unlock()
	e.roueTimerFired(gen, kind)
}

func TestRoueEngine144_FullTurnAndCredit(t *testing.T) {
	e := roueEngine144(t, RoueModeTantQueJeGagne, "red", "blue")
	st := e.GetState()
	if st.RoueSubPhase != RoueSubTurn || st.RoueStep != 1 || st.RoueCurrentTeam != "red" {
		t.Fatalf("start: %+v", st.RoueSubPhase)
	}
	if strings.ContainsAny(strings.Join(st.RoueBoard.Rows, ""), "TOA") {
		t.Fatalf("a hidden letter leaks in the board: %v", st.RoueBoard.Rows)
	}
	if err := e.RoueLetter("T"); err == nil {
		t.Fatal("ROUE_LETTER accepted outside LETTER")
	}
	steps := []struct {
		do   func() error
		want string
	}{
		{e.RoueSpin, RoueSubSpinning},
		{e.RoueStop, RoueSubStopping},
	}
	for _, s := range steps {
		if err := s.do(); err != nil {
			t.Fatal(err)
		}
		if got := e.GetState().RoueSubPhase; got != s.want {
			t.Fatalf("want %s got %s", s.want, got)
		}
	}
	if stop := e.GetState().RoueWheel.Stop; stop == nil || stop.To != 0 {
		t.Fatalf("STOP = %+v", stop)
	}
	fireRoueTimer144(e)
	st = e.GetState()
	if st.RoueSubPhase != RoueSubLetter || st.RoueWheel.Current != 0 || st.RoueWheel.Stop != nil {
		t.Fatalf("after braking: %s current=%d", st.RoueSubPhase, st.RoueWheel.Current)
	}
	if err := e.RoueLetter("A"); err == nil || err.(*RoueError).Code != RoueErrVowelNotAvailable {
		t.Fatalf("vowel: %v", err)
	}
	if e.GetState().RoueSubPhase != RoueSubLetter {
		t.Fatal("a refused letter must leave the state unchanged")
	}
	if err := e.RoueLetter("t"); err != nil {
		t.Fatal(err)
	}
	st = e.GetState()
	if st.RoueSubPhase != RoueSubLit || st.RoueTurnGain != 4000 || !strings.Contains(strings.Join(st.RoueBoard.Rows, "|"), "*") {
		t.Fatalf("LIT: %s gain=%d %v", st.RoueSubPhase, st.RoueTurnGain, st.RoueBoard.Rows)
	}
	if err := e.RoueFlip(); err != nil {
		t.Fatal(err)
	}
	fireRoueTimer144(e)
	st = e.GetState()
	if st.RoueSubPhase != RoueSubAnswer || st.RoueNextDefault != "" && st.RoueNextDefault != RoueHandSame {
		t.Fatalf("ANSWER: %s default=%s", st.RoueSubPhase, st.RoueNextDefault)
	}
	if st.RoueNextDefault != RoueHandSame {
		t.Fatalf("a found letter in TANT_QUE_JE_GAGNE advises SAME, got %q", st.RoueNextDefault)
	}
	if err := e.RoueAnswer(); err != nil {
		t.Fatal(err)
	}
	st = e.GetState()
	if st.RoueSubPhase != RoueSubSolved || st.RoueWinner != "red" || st.RoueAwards["red"] != 4000 {
		t.Fatalf("SOLVED: %+v %v", st.RoueWinner, st.RoueAwards)
	}
	if got := e.GetTeamsAndBumpers().Teams["red"].TeamPoints; got != 4000 {
		t.Fatalf("red credited %d", got)
	}
	if strings.Contains(strings.Join(st.RoueBoard.Rows, ""), "_") {
		t.Fatalf("board not fully visible: %v", st.RoueBoard.Rows)
	}
}

func TestRoueEngine144_MissEndsTurnAndAdvisesNext(t *testing.T) {
	e := roueEngine144(t, RoueModeTantQueJeGagne, "red", "blue")
	_ = e.RoueSpin()
	_ = e.RoueStop()
	fireRoueTimer144(e)
	if err := e.RoueLetter("Z"); err != nil {
		t.Fatal(err)
	}
	st := e.GetState()
	if st.RoueSubPhase != RoueSubTurnEnd || st.RoueNextDefault != RoueHandNext || st.RoueNextTeam != "blue" {
		t.Fatalf("%s %s %s", st.RoueSubPhase, st.RoueNextDefault, st.RoueNextTeam)
	}
	if err := e.RoueNext(RoueHandNext); err != nil {
		t.Fatal(err)
	}
	st = e.GetState()
	if st.RoueSubPhase != RoueSubTurn || st.RoueCurrentTeam != "blue" {
		t.Fatalf("%s %s", st.RoueSubPhase, st.RoueCurrentTeam)
	}
	if err := e.RoueNext(RoueHandSame); err == nil {
		t.Fatal("second command in the same sub-phase must be refused")
	}
}

func TestRoueEngine144_SoloRefusesNextAndPass(t *testing.T) {
	e := roueEngine144(t, RoueModeSolo, "red")
	if err := e.RoucePass(); err == nil {
		t.Fatal("pass accepted in SOLO")
	}
	_ = e.RoueSpin()
	_ = e.RoueStop()
	fireRoueTimer144(e)
	_ = e.RoueLetter("Z")
	if err := e.RoueNext(RoueHandNext); err == nil {
		t.Fatal("NEXT accepted in SOLO")
	}
	if err := e.RoueNext(RoueHandSame); err != nil {
		t.Fatal(err)
	}
}

func TestRoueEngine144_BuzzOfOtherTeamIgnoredAndPauseSuspendsTimer(t *testing.T) {
	e := roueEngine144(t, RoueModeTantQueJeGagne, "red", "blue")
	e.SetBumpers(map[string]*Bumper{"b1": {Team: "red"}, "b2": {Team: "blue"}})
	_ = e.RoueSpin()
	if e.ProcessButtonPress("b2", 1, "A") || e.GetState().RoueSubPhase != RoueSubSpinning {
		t.Fatal("buzz of the other team must be ignored")
	}
	e.Pause()
	if err := e.RoueStop(); err == nil {
		t.Fatal("ROUE_* must be refused in PAUSED")
	}
	e.Continue()
	if e.ProcessButtonPress("b1", 2, "A") {
		t.Fatal("a ROUE buzz is never recorded")
	}
	if e.GetState().RoueSubPhase != RoueSubStopping {
		t.Fatal("the buzz of the team with the hand must stop the wheel")
	}
}

func TestRoueEngine144_ConcurrentCommandsSingleWinner(t *testing.T) {
	e := roueEngine144(t, RoueModeTantQueJeGagne, "red", "blue")
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if e.RoueSpin() == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if ok != 1 {
		t.Fatalf("%d concurrent ROUE_SPIN applied, want exactly 1", ok)
	}
}

func TestRoueEngine144_StopResetsState(t *testing.T) {
	e := roueEngine144(t, RoueModeTantQueJeGagne, "red", "blue")
	_ = e.RoueSpin()
	e.Stop()
	st := e.GetState()
	if st.RoueSubPhase != "" || len(st.RoueTeams) != 0 || st.RoueBoard.Rows == nil || len(st.RoueBoard.Rows) != 0 || st.RoueWheel.Current != -1 {
		t.Fatalf("state not reset: %+v", st.RoueSubPhase)
	}
}

// Un buzz de TABLETTE (VPlayer, action BUTTON) de l'équipe qui a la main arrête la roue ; jamais enregistré comme buzz
// (accepted=false : pas de PAUSE), l'UPDATE part par OnRoueChange.
func TestRoueEngine144_VPlayerBuzzStopsTheWheel(t *testing.T) {
	e := roueEngine144(t, RoueModeTantQueJeGagne, "red", "blue")
	e.SetBumpers(map[string]*Bumper{"p1": {Team: "red", IsVPlayer: true}, "p2": {Team: "blue", IsVPlayer: true}})
	changed := make(chan struct{}, 4)
	e.OnRoueChange = func() { changed <- struct{}{} }
	_ = e.RoueSpin()
	if e.ProcessButtonPress("p2", 1, "A") || e.GetState().RoueSubPhase != RoueSubSpinning {
		t.Fatal("tablet of the other team ignored")
	}
	if e.ProcessButtonPress("p1", 2, "A") {
		t.Fatal("a ROUE buzz is never a recorded buzz")
	}
	if e.GetState().RoueSubPhase != RoueSubStopping {
		t.Fatal("tablet of the team with the hand must stop the wheel")
	}
	select {
	case <-changed:
	default:
		t.Fatal("OnRoueChange must fire so the UPDATE is broadcast")
	}
	if e.GetState().Phase != PhaseStarted {
		t.Fatal("no PAUSE")
	}
}
