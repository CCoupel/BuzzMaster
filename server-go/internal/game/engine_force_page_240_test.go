package game

// #240 — « Forcer l'affichage Jeu lors du lancement d'une manche ».
// Plan : _work/handoff/plan-240-20260925-171000.md.
//
// Décisions utilisateur testées :
//   - state.Page (sélecteur TV Jeu/Equipes/Joueurs/Palmarès, action REMOTE) est
//     forcé à "GAME" à la SÉLECTION de la question (entrée en PREPARE, Ready()) ET
//     au START (Start / StartImmediate / actualStart) ;
//   - depuis les 3 vues Equipes (SCORE), Joueurs (PLAYERS), Palmarès (PALMARES) ;
//   - le VPlayer suit le même champ (state.Page → GAME.REMOTE) ;
//   - CONTINUE après PAUSE force aussi Jeu ;
//   - HYPOTHÈSE (proposition du plan, NON explicitement confirmée par
//     l'utilisateur) : le départ d'une carte MEMOTION (SelectMotionCard) et le
//     tirage d'une carte RAFALE (StartRafaleMotionCardRound) forcent aussi Jeu ;
//   - le forçage n'est PAS un verrou : un REMOTE manuel ensuite est conservé.
// Hors forçage (AC4 du plan) : PAUSE, REVEAL, STOP, retour READY → PREPARE (#172),
// NEW_GAME, transitions refusées par leur garde de phase.
//
// Fichier additif ; les helpers sont préfixés fp240 (le fichier
// force_page_240_test.go de dev-backend a ses propres tests, sans recouvrement de noms).

import (
	"encoding/json"
	"sync"
	"testing"
)

var fp240Views = []string{"SCORE", "PLAYERS", "PALMARES"}

func fp240Page(e *Engine) string { s := e.GetState(); return s.Page }

func fp240Question(id string, qt QuestionType) *Question {
	q := &Question{ID: id, Type: qt, Question: "Q " + id, Answer: "42", Points: "10", Time: "30"}
	switch qt {
	case QuestionTypeMemory:
		q.TypedContent = TypedContent{MemoryMode: string(MemoryModeSolo)}
	case QuestionTypeMemotion:
		q.MotionCards = defaultMotionCards()
		q.MotionMode = string(MemoryModeSolo)
	}
	return q
}

// fp240Ready brings a fresh engine to READY on question q1 of the given type.
func fp240Ready(t *testing.T, qt QuestionType) *Engine {
	t.Helper()
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}, "blue": {Name: "blue"}})
	e.Ready("q1", fp240Question("q1", qt))
	e.ForceReady()
	if e.GetPhase() != PhaseReady {
		t.Fatalf("setup: expected READY, got %s", e.GetPhase())
	}
	return e
}

// fp240Started brings an engine to STARTED (QCM) with the view forced to `view`
// AFTER the start, so later transitions can be observed against a non-GAME view.
func fp240Started(t *testing.T) *Engine {
	t.Helper()
	e := fp240Ready(t, QuestionTypeQCM)
	e.StartImmediate(0)
	if e.GetPhase() != PhaseStarted {
		t.Fatalf("setup: expected STARTED, got %s", e.GetPhase())
	}
	return e
}

// fp240Revealed brings an engine to REVEALED (Reveal requires STOPPED or PAUSED).
func fp240Revealed(t *testing.T) *Engine {
	t.Helper()
	e := fp240Started(t)
	e.Stop()
	e.Reveal()
	if e.GetPhase() != PhaseRevealed {
		t.Fatalf("setup: expected REVEALED, got %s", e.GetPhase())
	}
	return e
}

// ---------------------------------------------------------------------------
// AC1/AC5 — sélection de la question (Ready → PREPARE), 3 vues × tous les types
// ---------------------------------------------------------------------------

func TestForcePage240_Selection_FromEachView_AllQuestionTypes(t *testing.T) {
	types := []QuestionType{
		QuestionTypeQCM, QuestionTypeSpeedy, QuestionTypeMemory,
		QuestionTypeMemotion, QuestionTypeRafale,
	}
	for _, qt := range types {
		for _, view := range fp240Views {
			t.Run(string(qt)+"/"+view, func(t *testing.T) {
				e := NewEngine()
				e.SetPage(view)
				e.Ready("q1", fp240Question("q1", qt))
				if e.GetPhase() != PhasePrepare {
					t.Fatalf("expected PREPARE, got %s", e.GetPhase())
				}
				if got := fp240Page(e); got != "GAME" {
					t.Errorf("selection of a %s question from view %s: Page=%q, want GAME", qt, view, got)
				}
			})
		}
	}
}

// Scénario de l'issue : TV laissée sur Equipes/Palmarès après un REVEAL, puis
// sélection de la question suivante.
func TestForcePage240_Selection_AfterReveal_TVLeftOnScores(t *testing.T) {
	for _, view := range fp240Views {
		t.Run(view, func(t *testing.T) {
			e := fp240Revealed(t)
			e.SetPage(view) // l'animateur montre les scores
			if fp240Page(e) != view {
				t.Fatalf("setup: SetPage(%s) not applied", view)
			}
			e.Ready("q2", fp240Question("q2", QuestionTypeQCM)) // question suivante
			if got := fp240Page(e); got != "GAME" {
				t.Errorf("after REVEAL + view %s, selecting next question: Page=%q, want GAME", view, got)
			}
		})
	}
}

func TestForcePage240_Selection_FromStoppedAndReSelectionInReady(t *testing.T) {
	// depuis STOPPED
	e := fp240Started(t)
	e.Stop()
	e.SetPage("PALMARES")
	e.Ready("q2", fp240Question("q2", QuestionTypeQCM))
	if got := fp240Page(e); got != "GAME" {
		t.Errorf("selection from STOPPED: Page=%q, want GAME", got)
	}

	// changement de question alors qu'on est déjà en READY
	e2 := fp240Ready(t, QuestionTypeQCM)
	e2.SetPage("SCORE")
	e2.Ready("q2", fp240Question("q2", QuestionTypeQCM))
	if got := fp240Page(e2); got != "GAME" {
		t.Errorf("re-selection while READY: Page=%q, want GAME", got)
	}
}

// Ready refusé par sa garde de phase : aucune bascule (AC4).
func TestForcePage240_Selection_RejectedByPhaseGuard_NoForce(t *testing.T) {
	e := fp240Started(t)
	e.SetPage("SCORE")
	e.Ready("q2", fp240Question("q2", QuestionTypeQCM)) // STARTED : refusé
	if e.GetPhase() != PhaseStarted {
		t.Fatalf("sanity: Ready from STARTED must be refused, phase=%s", e.GetPhase())
	}
	if got := fp240Page(e); got != "SCORE" {
		t.Errorf("refused Ready() must not touch Page: got %q, want SCORE", got)
	}
}

// ---------------------------------------------------------------------------
// AC1 — START (Start avec compte à rebours, StartImmediate), 3 vues
// ---------------------------------------------------------------------------

func TestForcePage240_Start_FromEachView(t *testing.T) {
	for _, view := range fp240Views {
		t.Run("Start/"+view, func(t *testing.T) {
			e := fp240Ready(t, QuestionTypeQCM)
			e.SetPage(view) // l'animateur remet les scores PENDANT la préparation
			e.Start(0)
			defer e.Stop()
			ph := e.GetPhase()
			if ph != PhaseCountdown && ph != PhaseStarted {
				t.Fatalf("expected COUNTDOWN or STARTED after Start, got %s", ph)
			}
			if got := fp240Page(e); got != "GAME" {
				t.Errorf("START from view %s: Page=%q, want GAME", view, got)
			}
		})
		t.Run("StartImmediate/"+view, func(t *testing.T) {
			e := fp240Ready(t, QuestionTypeQCM)
			e.SetPage(view)
			e.StartImmediate(0)
			if got := fp240Page(e); got != "GAME" {
				t.Errorf("StartImmediate from view %s: Page=%q, want GAME", view, got)
			}
		})
	}
}

func TestForcePage240_Start_AllQuestionTypes(t *testing.T) {
	for _, qt := range []QuestionType{QuestionTypeSpeedy, QuestionTypeQCM, QuestionTypeMemory, QuestionTypeMemotion, QuestionTypeRafale} {
		t.Run(string(qt), func(t *testing.T) {
			// MEMORY/MEMOTION/RAFALE : READY exige une sélection de participants
			// conforme ; StartImmediate (chemin de test du moteur) ne vérifie pas la phase.
			e := NewEngine()
			e.SetTeams(map[string]*Team{"red": {Name: "red"}})
			e.Ready("q1", fp240Question("q1", qt))
			e.SetPage("PLAYERS")
			e.StartImmediate(0)
			if got := fp240Page(e); got != "GAME" {
				t.Errorf("START of a %s question: Page=%q, want GAME", qt, got)
			}
		})
	}
}

// Start refusé (pas en READY, garde #172 B4) : aucune bascule.
func TestForcePage240_Start_RejectedFromPrepare_NoForce(t *testing.T) {
	e := NewEngine()
	e.Ready("q1", fp240Question("q1", QuestionTypeQCM)) // PREPARE (forcé GAME ici)
	e.SetPage("SCORE")                                  // puis l'animateur repasse sur Equipes
	e.Start(0)                                          // refusé : il faut READY
	if e.GetPhase() != PhasePrepare {
		t.Fatalf("sanity: Start from PREPARE must be refused, phase=%s", e.GetPhase())
	}
	if got := fp240Page(e); got != "SCORE" {
		t.Errorf("refused Start() must not touch Page: got %q, want SCORE", got)
	}
}

// ---------------------------------------------------------------------------
// AC3 — le forçage n'est pas un verrou : REMOTE manuel conservé
// ---------------------------------------------------------------------------

func TestForcePage240_NotALock_ManualRemoteAfterForcing(t *testing.T) {
	e := NewEngine()
	e.SetPage("PLAYERS")
	e.Ready("q1", fp240Question("q1", QuestionTypeQCM)) // forcé GAME
	if fp240Page(e) != "GAME" {
		t.Fatalf("setup: expected forced GAME, got %q", fp240Page(e))
	}

	// PREPARE : l'animateur repasse sur Equipes → conservé
	e.SetPage("SCORE")
	if got := fp240Page(e); got != "SCORE" {
		t.Errorf("manual REMOTE in PREPARE must stick: got %q", got)
	}
	// PREPARE → READY (TransitionToReady/ForceReady) ne re-force PAS
	e.ForceReady()
	if e.GetPhase() != PhaseReady {
		t.Fatalf("setup: expected READY, got %s", e.GetPhase())
	}
	if got := fp240Page(e); got != "SCORE" {
		t.Errorf("PREPARE→READY must not re-force: got %q, want SCORE", got)
	}
	// READY : Joueurs → conservé
	e.SetPage("PLAYERS")
	if got := fp240Page(e); got != "PLAYERS" {
		t.Errorf("manual REMOTE in READY must stick: got %q", got)
	}
	// START force, puis STARTED : Palmarès manuel → conservé
	e.StartImmediate(0)
	if got := fp240Page(e); got != "GAME" {
		t.Fatalf("START must force GAME, got %q", got)
	}
	e.SetPage("PALMARES")
	if got := fp240Page(e); got != "PALMARES" {
		t.Errorf("manual REMOTE in STARTED must stick: got %q", got)
	}
	// Aucun tic ni lecture d'état ne le remet à GAME
	_ = e.GetState()
	_ = e.GetGameJSON()
	if got := fp240Page(e); got != "PALMARES" {
		t.Errorf("reading state must not alter Page: got %q", got)
	}
}

// ---------------------------------------------------------------------------
// CONTINUE après PAUSE : force Jeu (décision utilisateur)
// ---------------------------------------------------------------------------

func TestForcePage240_ContinueAfterPause_FromEachView(t *testing.T) {
	for _, view := range fp240Views {
		t.Run(view, func(t *testing.T) {
			e := fp240Started(t)
			e.Pause()
			if e.GetPhase() != PhasePaused {
				t.Fatalf("setup: expected PAUSED, got %s", e.GetPhase())
			}
			e.SetPage(view)
			e.Continue()
			if e.GetPhase() != PhaseStarted {
				t.Fatalf("expected STARTED after Continue, got %s", e.GetPhase())
			}
			if got := fp240Page(e); got != "GAME" {
				t.Errorf("CONTINUE from view %s: Page=%q, want GAME", view, got)
			}
		})
	}
}

// Continue refusé (hors PAUSED) : aucune bascule.
func TestForcePage240_Continue_RejectedOutsidePaused_NoForce(t *testing.T) {
	e := fp240Started(t)
	e.SetPage("SCORE")
	e.Continue() // STARTED : refusé
	if got := fp240Page(e); got != "SCORE" {
		t.Errorf("refused Continue() must not touch Page: got %q, want SCORE", got)
	}
}

// ---------------------------------------------------------------------------
// AC4 — transitions qui NE lancent PAS une manche : aucun forçage
// ---------------------------------------------------------------------------

func TestForcePage240_NoForceOn_Pause(t *testing.T) {
	e := fp240Started(t)
	e.SetPage("SCORE")
	e.Pause()
	if got := fp240Page(e); got != "SCORE" {
		t.Errorf("PAUSE must not force: Page=%q, want SCORE", got)
	}
}

func TestForcePage240_NoForceOn_Reveal(t *testing.T) {
	e := fp240Started(t)
	e.Stop()
	e.SetPage("PALMARES")
	e.Reveal()
	if e.GetPhase() != PhaseRevealed {
		t.Fatalf("sanity: expected REVEALED, got %s", e.GetPhase())
	}
	if got := fp240Page(e); got != "PALMARES" {
		t.Errorf("REVEAL must not force: Page=%q, want PALMARES", got)
	}
}

func TestForcePage240_NoForceOn_Stop(t *testing.T) {
	e := fp240Started(t)
	e.SetPage("PLAYERS")
	e.Stop()
	if e.GetPhase() != PhaseStopped {
		t.Fatalf("sanity: expected STOPPED, got %s", e.GetPhase())
	}
	if got := fp240Page(e); got != "PLAYERS" {
		t.Errorf("STOP must not force: Page=%q, want PLAYERS", got)
	}
}

// #172 : retour automatique READY → PREPARE (conformité perdue) n'est PAS une
// nouvelle sélection de question → pas de forçage.
func TestForcePage240_NoForceOn_AutoRollbackReadyToPrepare(t *testing.T) {
	e := setupReadyMemory(t, string(MemoryModeSolo),
		map[string]string{"b1": "red", "b2": "blue"}, []string{"red"})
	e.SetPage("SCORE")
	if err := e.SetMemoryParticipatingTeams([]string{}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams([]): %v", err)
	}
	if e.GetPhase() != PhasePrepare {
		t.Fatalf("sanity: expected automatic rollback to PREPARE, got %s", e.GetPhase())
	}
	if got := fp240Page(e); got != "SCORE" {
		t.Errorf("automatic READY→PREPARE rollback must not force: Page=%q, want SCORE", got)
	}
}

// NEW_GAME (InitGame) n'est pas un lancement de manche (AC4 du plan).
func TestForcePage240_NoForceOn_InitGame(t *testing.T) {
	e := NewEngine()
	e.SetPage("PALMARES")
	e.InitGame()
	if e.GetPhase() != PhaseNewGame {
		t.Fatalf("sanity: expected NEW_GAME, got %s", e.GetPhase())
	}
	if got := fp240Page(e); got != "PALMARES" {
		t.Errorf("NEW_GAME must not force Page: got %q, want PALMARES", got)
	}
}

// ---------------------------------------------------------------------------
// HYPOTHÈSE (plan Q4, non confirmée) — départ d'une carte MEMOTION / tirage RAFALE
// ---------------------------------------------------------------------------

func TestForcePage240_MemotionCardStart_FromEachView(t *testing.T) {
	for _, view := range fp240Views {
		t.Run(view, func(t *testing.T) {
			e := NewEngine()
			e.SetTeams(map[string]*Team{"red": {Name: "red"}})
			startMEMOTION(t, e, "mq1", makeMotionQuestion("mq1", defaultMotionCards(), "SOLO"))
			e.SetPage(view)
			if err := e.SelectMotionCard("mc-1"); err != nil {
				t.Fatalf("SelectMotionCard: %v", err)
			}
			if got := fp240Page(e); got != "GAME" {
				t.Errorf("MEMOTION card start from view %s: Page=%q, want GAME", view, got)
			}
		})
	}
}

func TestForcePage240_MemotionCardStart_Rejected_NoForce(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}})
	startMEMOTION(t, e, "mq1", makeMotionQuestion("mq1", defaultMotionCards(), "SOLO"))
	e.SetPage("SCORE")
	if err := e.SelectMotionCard("carte-inexistante"); err == nil {
		t.Fatal("sanity: selecting an unknown card must fail")
	}
	if got := fp240Page(e); got != "SCORE" {
		t.Errorf("rejected card selection must not force: Page=%q, want SCORE", got)
	}
}

func TestForcePage240_RafaleCardDraw_FromEachView(t *testing.T) {
	for _, view := range fp240Views {
		t.Run(view, func(t *testing.T) {
			e := NewEngine()
			seedRafaleReservoirCouple(t, e, "h", 5, CategoryHistory, 1)
			e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}})
			card := rafaleMotionCard("mc-r1", []string{string(CategoryHistory)}, []int{1}, 3, 10)
			startMEMOTION(t, e, "mq-rafale-card", makeMotionQuestion("mq-rafale-card", []MotionCard{card}, "SOLO"))
			if err := e.SelectMotionCard(card.ID); err != nil {
				t.Fatalf("SelectMotionCard: %v", err)
			}
			if err := e.FlipMotionCard(); err != nil {
				t.Fatalf("FlipMotionCard: %v", err)
			}
			e.SetPage(view) // l'animateur a repassé la TV sur les scores entre-temps
			if _, _, _, err := e.StartRafaleMotionCardRound(card.ID); err != nil {
				t.Fatalf("StartRafaleMotionCardRound: %v", err)
			}
			defer e.Stop()
			if got := fp240Page(e); got != "GAME" {
				t.Errorf("RAFALE card draw from view %s: Page=%q, want GAME", view, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC2/AC6 — le forçage est visible dans l'état diffusé (GAME.REMOTE), donc
// vu par l'admin (bouton « Jeu »), la TV et le VPlayer
// ---------------------------------------------------------------------------

func fp240RemoteInJSON(t *testing.T, e *Engine) (string, bool) {
	t.Helper()
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(e.GetGameJSON(), &payload); err != nil {
		t.Fatalf("GetGameJSON invalid: %v", err)
	}
	var game map[string]interface{}
	if err := json.Unmarshal(payload["GAME"], &game); err != nil {
		t.Fatalf("GAME node invalid: %v", err)
	}
	v, ok := game["REMOTE"]
	if !ok {
		return "", false
	}
	s, _ := v.(string)
	return s, true
}

func TestForcePage240_Serialization_RemoteIsGameAfterForcing(t *testing.T) {
	e := NewEngine()
	e.SetPage("SCORE")
	if got, ok := fp240RemoteInJSON(t, e); !ok || got != "SCORE" {
		t.Fatalf("setup: GAME.REMOTE should be SCORE, got %q (present=%v)", got, ok)
	}
	e.Ready("q1", fp240Question("q1", QuestionTypeQCM))
	if got, ok := fp240RemoteInJSON(t, e); !ok || got != "GAME" {
		t.Errorf("after selection: GAME.REMOTE=%q (present=%v), want present GAME", got, ok)
	}
}

// Règle projet : jamais d'omission du champ — la clé REMOTE est toujours présente
// (moteur neuf, après forçage, après REMOTE manuel).
func TestForcePage240_Serialization_RemoteKeyAlwaysPresent(t *testing.T) {
	e := NewEngine()
	if _, ok := fp240RemoteInJSON(t, e); !ok {
		t.Error("fresh engine: GAME.REMOTE key must be serialized")
	}
	e.SetPage("PLAYERS")
	if got, ok := fp240RemoteInJSON(t, e); !ok || got != "PLAYERS" {
		t.Errorf("after manual REMOTE: got %q present=%v", got, ok)
	}
	e.SetPage("") // « null »/vide → GAME (comportement SetPage existant)
	if got, ok := fp240RemoteInJSON(t, e); !ok || got != "GAME" {
		t.Errorf("SetPage(\"\") must serialize GAME, got %q present=%v", got, ok)
	}
}

// Le callback OnStateChange (qui construit l'UPDATE diffusé à TV/VPlayer/admin)
// voit DÉJÀ Page=GAME quand il est invoqué à l'entrée en PREPARE, au START et au CONTINUE.
func TestForcePage240_Broadcast_StateAlreadyForcedWhenCallbackFires(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}})
	var mu sync.Mutex
	seen := map[GamePhase]string{}
	e.OnStateChange = func(phase GamePhase) {
		mu.Lock()
		defer mu.Unlock()
		if _, done := seen[phase]; !done {
			seen[phase] = e.GetState().Page
		}
	}

	e.SetPage("SCORE")
	e.Ready("q1", fp240Question("q1", QuestionTypeQCM))
	e.ForceReady()
	e.SetPage("PLAYERS")
	e.StartImmediate(0)
	e.Pause()
	e.SetPage("PALMARES")
	e.Continue()

	mu.Lock()
	defer mu.Unlock()
	for _, ph := range []GamePhase{PhasePrepare, PhaseStarted} {
		if got, ok := seen[ph]; !ok {
			t.Errorf("OnStateChange(%s) was never invoked — cannot assert broadcast state", ph)
		} else if got != "GAME" {
			t.Errorf("OnStateChange(%s) fired with Page=%q, want GAME (the UPDATE must already carry REMOTE=GAME)", ph, got)
		}
	}
}
