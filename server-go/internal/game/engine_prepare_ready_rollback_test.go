package game

// #172 — D4 (test-writer, plan `_work/reports/plan-20260817-122307.md` §7 Bloc D).
//
// Couvre le retour arrière READY -> PREPARE déclenché par reevaluatePrepareReadyUnsafe
// (#172 B3) lorsque la conformité des participants cesse d'être vraie EN PHASE READY :
//   - retirer une équipe casse la conformité -> retour en PREPARE
//   - la rétablir fait repasser en READY, sans repasser par un nouveau PONG ("sans geste
//     supplémentaire")
//   - AUCUN retour arrière depuis STARTED ou une phase de jeu ultérieure (risque R2, plan
//     §8) : ce fichier vérifie explicitement ce cas négatif, comme demandé par le CDP.
//
// Fichier additif — n'existait pas avant #172, ne modifie aucun test existant (règle de
// non-régression du test-writer).
//
// NOTE (à l'attention du CDP, cf. D8) : au moment de l'écriture de ces tests, la fonction
// reevaluatePrepareReadyUnsafe existe déjà dans engine.go (#172 B3, documentée) mais n'est
// PAS ENCORE appelée depuis SetMemoryParticipatingTeams / SetMotionParticipatingTeams —
// ces tests sont donc rouges jusqu'à ce que dev-backend termine le branchement. C'est le
// comportement TDD attendu, pas une erreur d'écriture.

import "testing"

// setupReadyMemory amène l'engine en phase READY sur une question MEMORY avec les
// équipes actives données, buzzers tous prêts, et la sélection `participating` déjà
// conforme. Retourne l'engine prêt pour le scénario de retour arrière.
func setupReadyMemory(t *testing.T, mode string, teamBumpers map[string]string, participating []string) *Engine {
	t.Helper()
	e := NewEngine()

	teams := make(map[string]*Team, len(teamBumpers))
	for _, teamID := range teamBumpers {
		teams[teamID] = &Team{Name: teamID}
	}
	e.SetTeams(teams)
	for bumperID, teamID := range teamBumpers {
		e.UpdateBumper(bumperID, map[string]interface{}{"TEAM": teamID})
	}

	q := &Question{ID: "q1", Type: QuestionTypeMemory, MemoryMode: mode}
	e.Ready("q1", q)

	for bumperID := range teamBumpers {
		e.SetBumperReady(bumperID)
	}
	if !e.AreAllTeamsReady() {
		t.Fatal("setup: all active teams should be ready after every bumper answered")
	}

	if err := e.SetMemoryParticipatingTeams(participating); err != nil {
		t.Fatalf("setup: SetMemoryParticipatingTeams(%v) should succeed in PREPARE, got %v", participating, err)
	}
	if !e.ParticipantsConform() {
		t.Fatalf("setup: selection %v should already be conform for mode %s", participating, mode)
	}

	e.TransitionToReady()
	if e.GetPhase() != PhaseReady {
		t.Fatalf("setup: expected phase READY, got %s", e.GetPhase())
	}
	return e
}

// setupReadyMotion — équivalent de setupReadyMemory pour MEMOTION.
func setupReadyMotion(t *testing.T, teamBumpers map[string]string, participating []string) *Engine {
	t.Helper()
	e := NewEngine()

	teams := make(map[string]*Team, len(teamBumpers))
	for _, teamID := range teamBumpers {
		teams[teamID] = &Team{Name: teamID}
	}
	e.SetTeams(teams)
	for bumperID, teamID := range teamBumpers {
		e.UpdateBumper(bumperID, map[string]interface{}{"TEAM": teamID})
	}

	q := &Question{ID: "q1", Type: QuestionTypeMemotion}
	e.Ready("q1", q)

	for bumperID := range teamBumpers {
		e.SetBumperReady(bumperID)
	}
	if !e.AreAllTeamsReady() {
		t.Fatal("setup: all active teams should be ready after every bumper answered")
	}

	if err := e.SetMotionParticipatingTeams(participating); err != nil {
		t.Fatalf("setup: SetMotionParticipatingTeams(%v) should succeed in PREPARE, got %v", participating, err)
	}
	if !e.ParticipantsConform() {
		t.Fatalf("setup: selection %v should already be conform for MEMOTION", participating)
	}

	e.TransitionToReady()
	if e.GetPhase() != PhaseReady {
		t.Fatalf("setup: expected phase READY, got %s", e.GetPhase())
	}
	return e
}

// D4 — MEMORY SOLO : retirer l'unique équipe sélectionnée (READY -> non conforme)
// doit ramener la question en PREPARE.
func TestPrepareReadyRollback_MemorySolo_TeamRemoved_RevertsToPrepare(t *testing.T) {
	e := setupReadyMemory(t, string(MemoryModeSolo),
		map[string]string{"b1": "red", "b2": "blue"}, []string{"red"})

	if err := e.SetMemoryParticipatingTeams([]string{}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams([]) should be accepted in READY, got %v", err)
	}

	if e.GetPhase() != PhasePrepare {
		t.Errorf("Expected phase PREPARE after breaking conformity in READY, got %s", e.GetPhase())
	}
}

// D4 — MEMORY SOLO : rétablir la conformité (re-sélectionner une équipe) doit faire
// repasser en READY automatiquement, sans qu'un nouveau PONG ne soit nécessaire — les
// deux bumpers restent déjà marqués Ready pendant tout le scénario.
func TestPrepareReadyRollback_MemorySolo_ConformityRestored_ReturnsToReadyWithoutExtraPong(t *testing.T) {
	e := setupReadyMemory(t, string(MemoryModeSolo),
		map[string]string{"b1": "red", "b2": "blue"}, []string{"red"})

	if err := e.SetMemoryParticipatingTeams([]string{}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams([]) should be accepted in READY, got %v", err)
	}
	if e.GetPhase() != PhasePrepare {
		t.Fatalf("precondition failed: expected PREPARE after breaking conformity, got %s", e.GetPhase())
	}

	// "sans geste supplémentaire" : aucun nouvel appel à SetBumperReady ici.
	if err := e.SetMemoryParticipatingTeams([]string{"blue"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams([blue]) should be accepted in PREPARE, got %v", err)
	}

	if e.GetPhase() != PhaseReady {
		t.Errorf("Expected phase READY after conformity restored (no extra PONG needed), got %s", e.GetPhase())
	}
	// Les bumpers n'ont jamais cessé d'être Ready : c'est bien la conformité seule qui a
	// piloté l'aller-retour, pas AreAllTeamsReady.
	if !e.GetBumper("b1").Ready || !e.GetBumper("b2").Ready {
		t.Error("Bumpers should have remained Ready throughout the rollback/restore cycle")
	}
}

// D4 — MEMORY multi (CHACUN_SON_TOUR) : repasser sous le seuil de deux équipes doit
// aussi déclencher le retour arrière — la règle n'est pas spécifique au mode SOLO.
func TestPrepareReadyRollback_MemoryMulti_TeamRemovedBelowTwo_RevertsToPrepare(t *testing.T) {
	e := setupReadyMemory(t, string(MemoryModeChacunSonTour),
		map[string]string{"b1": "red", "b2": "blue", "b3": "green"},
		[]string{"red", "blue"})

	if err := e.SetMemoryParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams([red]) should be accepted in READY, got %v", err)
	}

	if e.GetPhase() != PhasePrepare {
		t.Errorf("Expected phase PREPARE after dropping below 2 teams in multi mode, got %s", e.GetPhase())
	}
}

// D4 — MEMOTION : même mécanisme de retour arrière que MEMORY, sur SetMotionParticipatingTeams.
func TestPrepareReadyRollback_Motion_TeamRemoved_RevertsToPrepare(t *testing.T) {
	e := setupReadyMotion(t, map[string]string{"b1": "red", "b2": "blue"}, []string{"red"})

	if err := e.SetMotionParticipatingTeams([]string{}); err != nil {
		t.Fatalf("SetMotionParticipatingTeams([]) should be accepted in READY, got %v", err)
	}

	if e.GetPhase() != PhasePrepare {
		t.Errorf("Expected phase PREPARE after breaking MEMOTION conformity in READY, got %s", e.GetPhase())
	}
}

// D4 (cas négatif, risque R2) — une fois la partie démarrée (STARTED), toute tentative
// de modifier la sélection est refusée par le garde-fou existant de Set*ParticipatingTeams
// (PREPARE/READY uniquement) — donc AUCUN retour arrière ne peut se produire depuis
// STARTED : ni changement de phase, ni erreur silencieusement ignorée.
func TestPrepareReadyRollback_NoRollbackFromStarted(t *testing.T) {
	e := setupReadyMemory(t, string(MemoryModeSolo),
		map[string]string{"b1": "red", "b2": "blue"}, []string{"red"})

	e.StartImmediate(0)
	if e.GetPhase() != PhaseStarted {
		t.Fatalf("precondition failed: expected phase STARTED, got %s", e.GetPhase())
	}

	err := e.SetMemoryParticipatingTeams([]string{})
	if err == nil {
		t.Error("SetMemoryParticipatingTeams should be refused once the game is STARTED")
	}

	if e.GetPhase() != PhaseStarted {
		t.Errorf("Phase must NOT regress from STARTED — an in-progress round must never be interrupted by a participant-selection change (plan R2), got %s", e.GetPhase())
	}
}

// D4 (cas négatif, risque R2) — même vérification pendant COUNTDOWN, la phase de jeu
// intermédiaire entre READY et STARTED : le retour arrière ne doit pas non plus s'y
// produire.
func TestPrepareReadyRollback_NoRollbackFromCountdown(t *testing.T) {
	e := setupReadyMemory(t, string(MemoryModeSolo),
		map[string]string{"b1": "red", "b2": "blue"}, []string{"red"})

	e.Start(20)
	defer e.Stop()
	if e.GetPhase() != PhaseCountdown {
		t.Fatalf("precondition failed: expected phase COUNTDOWN, got %s", e.GetPhase())
	}

	err := e.SetMemoryParticipatingTeams([]string{})
	if err == nil {
		t.Error("SetMemoryParticipatingTeams should be refused during COUNTDOWN")
	}

	if e.GetPhase() != PhaseCountdown {
		t.Errorf("Phase must NOT regress from COUNTDOWN — no rollback outside PREPARE/READY (plan R2), got %s", e.GetPhase())
	}
}

// D4 (cas négatif, risque R2) — MEMOTION, même vérification que
// TestPrepareReadyRollback_NoRollbackFromStarted côté SetMotionParticipatingTeams.
func TestPrepareReadyRollback_Motion_NoRollbackFromStarted(t *testing.T) {
	e := setupReadyMotion(t, map[string]string{"b1": "red", "b2": "blue"}, []string{"red"})

	e.StartImmediate(0)
	if e.GetPhase() != PhaseStarted {
		t.Fatalf("precondition failed: expected phase STARTED, got %s", e.GetPhase())
	}

	err := e.SetMotionParticipatingTeams([]string{})
	if err == nil {
		t.Error("SetMotionParticipatingTeams should be refused once the game is STARTED")
	}

	if e.GetPhase() != PhaseStarted {
		t.Errorf("Phase must NOT regress from STARTED for MEMOTION either (plan R2), got %s", e.GetPhase())
	}
}
