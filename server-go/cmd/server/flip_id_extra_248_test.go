package main

import (
	"strings"
	"testing"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// #248 lot 5b — compléments des tests FLIP_ID (flip_id_dedupe_248_test.go) : (1) purge à la suppression du bumper,
// (2) hôte MEMOTION (carte de type MEMORY) côté Go, (3) les 3 événements de journal de contracts/logging.md
// (FLIP_DUPLICATE INFO, FLIP_IGNORED INFO, FLIP_ID_INVALID WARN). Mêmes règles : tout est piloté par JSON / messages
// bruts, donc ces tests compilent avant l'implémentation et échouent sur le comportement.

// --- (1) Suppression du bumper : la mémoire des FLIP_ID part avec lui ------------------------------

func mustMsg248(t *testing.T, action string, payload interface{}) *protocol.Message {
	t.Helper()
	msg, err := protocol.NewMessage(action, payload)
	if err != nil {
		t.Fatalf("message %s : %v", action, err)
	}
	return msg
}

func TestFlipID248_MemoryPurgedWhenTheBumperIsDeleted(t *testing.T) {
	r := newFlipRoom248(t)
	r.flip(t, "p1", "1-1", "f-del", nil)
	r.observe(t)

	// suppression par le vrai chemin DELETE_BUMPER
	delMsg := mustMsg248(t, "DELETE_BUMPER", map[string]interface{}{"ID": r.ids["p1"]})
	r.app.handleDeleteBumper(delMsg)
	if r.app.engine.GetBumper(r.ids["p1"]) != nil {
		t.Fatal("précondition : le bumper p1 doit être supprimé")
	}
	// le joueur revient avec le MÊME identifiant (même équipe, et la main reste à TeamA : on n'appelle surtout pas
	// ClearMemoryFlippedCards, qui ferait passer la main à TeamB et produirait un NOT_TURN sans rapport avec la purge)
	tb := r.app.engine.GetTeamsAndBumpers()
	bumpers := map[string]*game.Bumper{}
	for id, b := range tb.Bumpers {
		bumpers[id] = b
	}
	bumpers[r.ids["p1"]] = &game.Bumper{Name: "P1", Team: "TeamA", IsVirtual: true, IsVPlayer: true, Connected: true}
	r.app.engine.SetBumpers(bumpers)
	r.observe(t)

	// La carte 1-1 est toujours face visible. Mémoire INTACTE => APPLIED + DUPLICATE=true (réponse rejouée).
	// Mémoire PURGÉE => geste neuf, évalué : carte déjà visible => IGNORED NOT_FLIPPABLE + DUPLICATE=false.
	r.flip(t, "p1", "1-1", "f-del", nil)
	acks := acksOf248(r.observe(t)["p1"])
	if len(acks) != 1 {
		t.Fatalf("attendu 1 ACK, a %+v", acks)
	}
	if acks[0].Duplicate || acks[0].Status == "APPLIED" {
		t.Errorf("la mémoire du bumper supprimé doit être purgée : le FLIP_ID est évalué comme neuf, pas rejoué (a %+v)", acks[0])
	}
	if acks[0].Status != "IGNORED" || acks[0].Reason != "NOT_FLIPPABLE" {
		t.Errorf("évalué comme neuf, le geste est refusé (carte déjà visible) : attendu IGNORED NOT_FLIPPABLE, a %+v", acks[0])
	}
}

// --- (2) Hôte MEMOTION : carte de type MEMORY -----------------------------------------------

func configureMotion248(t *testing.T, r *room247) {
	t.Helper()
	e := r.app.engine
	card := game.MotionCard{
		ID: "mc-mem", Type: game.QuestionTypeMemory, RectoTheme: "T", Difficulty: 1,
		TypedContent: game.TypedContent{
			MemoryConfig: &game.MemoryConfig{FlipDelay: 0.05},
			MemoryPairs:  []game.MemoryPair{{ID: 1}, {ID: 2}},
		},
	}
	e.SetPhase(game.PhaseStopped)
	e.Ready("mq-248", &game.Question{
		ID: "mq-248", Type: game.QuestionTypeMemotion, MotionCards: []game.MotionCard{card},
		MotionMode: "SOLO", Points: "10", Time: "0",
	})
	if err := e.SetMotionParticipatingTeams([]string{"TeamA", "TeamB"}); err != nil {
		t.Fatalf("SetMotionParticipatingTeams : %v", err)
	}
	e.StartImmediate(0)
	e.InitMotionState()
	if err := e.SelectMotionCard("mc-mem"); err != nil {
		t.Fatalf("SelectMotionCard : %v", err)
	}
	if err := e.FlipMotionCard(); err != nil {
		t.Fatalf("FlipMotionCard : %v", err)
	}
}

func newMotionRoom248(t *testing.T) *room247 {
	t.Helper()
	r := newRoom247(t)
	configureMotion248(t, r)
	r.observe(t)
	return r
}

var motionScope248 = map[string]interface{}{"MOTION_CARD_ID": "mc-mem"}

func TestFlipID248_Motion_AppliedFlip_AckEchoesTheMotionCardAndGoesToTheSenderOnly(t *testing.T) {
	r := newMotionRoom248(t)
	r.flip(t, "p1", "1-1", "f-mot-1", motionScope248)
	got := r.observe(t)
	acks := acksOf248(got["p1"])
	if len(acks) != 1 || acks[0].Status != "APPLIED" || acks[0].Duplicate || acks[0].MotionCardID != "mc-mem" || acks[0].CardID != "1-1" {
		t.Fatalf("attendu 1 ACK APPLIED avec MOTION_CARD_ID=mc-mem, a %+v", acks)
	}
	for _, n := range []string{"p2", "p3", "admin", "tv", "anim"} {
		if len(acksOf248(got[n])) != 0 {
			t.Errorf("%s ne doit jamais recevoir l'ACK : %v", n, got[n])
		}
	}
	if f := motionActiveMemoryFlippedCardsForTest(r.app); len(f) != 1 {
		t.Errorf("une carte retournée dans la carte MEMOTION, a %v", f)
	}
}

func TestFlipID248_Motion_ResendAfterMissedPair_DoesNotFlipASecondTime(t *testing.T) {
	r := newMotionRoom248(t)
	signal := flipBackSignal248(t)
	r.flip(t, "p1", "1-1", "f-mot-a", motionScope248)
	r.observe(t)
	r.flip(t, "p1", "2-1", "f-mot-b", motionScope248) // paire ratée
	r.observe(t)
	waitFlipBack248(t, r, signal)
	if f := motionActiveMemoryFlippedCardsForTest(r.app); len(f) != 0 {
		t.Fatalf("précondition : cartes remises face cachée, a %v", f)
	}

	r.flip(t, "p1", "2-1", "f-mot-b", motionScope248) // renvoi
	got := r.observe(t)
	if f := motionActiveMemoryFlippedCardsForTest(r.app); len(f) != 0 {
		t.Errorf("le renvoi ne doit PAS retourner la carte une seconde fois : %v", f)
	}
	acks := acksOf248(got["p1"])
	if len(acks) != 1 || acks[0].Status != "APPLIED" || !acks[0].Duplicate || acks[0].MotionCardID != "mc-mem" {
		t.Fatalf("attendu ACK APPLIED DUPLICATE=true MOTION_CARD_ID=mc-mem, a %+v", acks)
	}
	requireQuiet248(t, got, "p2", "p3", "admin", "tv", "anim")
}

func TestFlipID248_Motion_NotTheTeamsTurn_IsIgnoredNotTurn(t *testing.T) {
	r := newMotionRoom248(t) // MotionCurrentTeam = TeamA ; p3 est TeamB
	r.flip(t, "p3", "1-1", "f-mot-oop", motionScope248)
	got := r.observe(t)
	acks := acksOf248(got["p3"])
	if len(acks) != 1 || acks[0].Status != "IGNORED" || acks[0].Reason != "NOT_TURN" || acks[0].MotionCardID != "mc-mem" {
		t.Fatalf("attendu 1 ACK IGNORED NOT_TURN avec MOTION_CARD_ID, a %+v", acks)
	}
	requireQuiet248(t, got, "p1", "p2", "admin", "tv", "anim")
	if f := motionActiveMemoryFlippedCardsForTest(r.app); len(f) != 0 {
		t.Errorf("aucun retournement attendu : %v", f)
	}
}

func TestFlipID248_Motion_NotFlippable_IsIgnoredWithReason(t *testing.T) {
	r := newMotionRoom248(t)
	r.flip(t, "p1", "1-1", "f-mot-ok", motionScope248)
	r.observe(t)
	r.flip(t, "p1", "1-1", "f-mot-again", motionScope248)
	got := r.observe(t)
	acks := acksOf248(got["p1"])
	if len(acks) != 1 || acks[0].Status != "IGNORED" || acks[0].Reason != "NOT_FLIPPABLE" || acks[0].Duplicate {
		t.Fatalf("attendu 1 ACK IGNORED NOT_FLIPPABLE, a %+v", acks)
	}
	requireQuiet248(t, got, "p2", "p3", "admin", "tv", "anim")
}

func TestFlipID248_Motion_CardNoLongerPlayable_IsIgnoredNotPlayable(t *testing.T) {
	r := newMotionRoom248(t)
	if err := r.app.engine.RevealMotionCard(); err != nil { // la carte passe en REVEAL : plus jouable
		t.Fatalf("RevealMotionCard : %v", err)
	}
	r.observe(t)
	r.flip(t, "p1", "1-1", "f-mot-late", motionScope248)
	acks := acksOf248(r.observe(t)["p1"])
	if len(acks) != 1 || acks[0].Status != "IGNORED" || acks[0].Reason != "NOT_PLAYABLE" {
		t.Fatalf("attendu 1 ACK IGNORED NOT_PLAYABLE (carte plus en QUESTION), a %+v", acks)
	}
}

func TestFlipID248_Motion_WrongCardScope_IsIgnoredCardScope(t *testing.T) {
	r := newMotionRoom248(t)
	r.flip(t, "p1", "1-1", "f-mot-scope", map[string]interface{}{"MOTION_CARD_ID": "autre-carte"})
	acks := acksOf248(r.observe(t)["p1"])
	if len(acks) != 1 || acks[0].Status != "IGNORED" || acks[0].Reason != "CARD_SCOPE" || acks[0].MotionCardID != "autre-carte" {
		t.Fatalf("attendu 1 ACK IGNORED CARD_SCOPE (carte active ≠ carte demandée), a %+v", acks)
	}
}

func TestFlipID248_Motion_WithoutFlipID_BehaviourUnchanged(t *testing.T) {
	r := newMotionRoom248(t)
	r.flip(t, "p1", "1-1", "", motionScope248)
	got := r.observe(t)
	if len(acksOf248(got["p1"])) != 0 {
		t.Errorf("sans FLIP_ID : aucun ACK : %v", got["p1"])
	}
	if indexOfAction248(got["admin"], "UPDATE") < 0 {
		t.Error("sans FLIP_ID : l'UPDATE général actuel doit toujours atteindre l'admin")
	}
	if f := motionActiveMemoryFlippedCardsForTest(r.app); len(f) != 1 {
		t.Errorf("la carte doit être retournée comme avant : %v", f)
	}
}

// --- (3) Journal : FLIP_DUPLICATE / FLIP_IGNORED (INFO), FLIP_ID_INVALID (WARN) -----------------

func logEntries248(r *room247, level game.LogLevel, event string) []string {
	var out []string
	for _, en := range r.app.logger.GetHistory() {
		if en.Level == level && strings.Contains(en.Message, event) {
			out = append(out, en.Message)
		}
	}
	return out
}

func newLoggedFlipRoom248(t *testing.T) *room247 {
	t.Helper()
	r := newFlipRoom248(t)
	server.SetGlobalLogger(r.app.logger) // les handlers journalisent via server.LogX
	t.Cleanup(func() { server.SetGlobalLogger(nil) })
	return r
}

func TestFlipID248_Log_DuplicateIsInfoWithItsFields(t *testing.T) {
	r := newLoggedFlipRoom248(t)
	r.flip(t, "p1", "1-1", "f-log-dup", nil)
	r.observe(t)
	if n := len(logEntries248(r, game.LogLevelInfo, "FLIP_DUPLICATE")); n != 0 {
		t.Fatalf("précondition : aucun FLIP_DUPLICATE avant le renvoi, a %d", n)
	}
	r.flip(t, "p1", "1-1", "f-log-dup", nil)
	r.observe(t)

	lines := logEntries248(r, game.LogLevelInfo, "FLIP_DUPLICATE")
	if len(lines) != 1 {
		t.Fatalf("attendu 1 entrée INFO FLIP_DUPLICATE, a %d : %v", len(lines), lines)
	}
	for _, want := range []string{"client=", "player_id=", "flip_id=", "card=", "motion_card=", "status=", "reason=", "f-log-dup", r.ids["p1"], "1-1", "APPLIED"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("FLIP_DUPLICATE doit contenir %q : %s", want, lines[0])
		}
	}
}

func TestFlipID248_Log_FirstIgnoredIsInfoOnce_ItsResendIsADuplicateNotAnotherIgnored(t *testing.T) {
	r := newLoggedFlipRoom248(t)
	r.flip(t, "p3", "1-1", "f-log-oop", nil) // hors tour avec FLIP_ID : IGNORED NOT_TURN
	r.observe(t)

	ign := logEntries248(r, game.LogLevelInfo, "FLIP_IGNORED")
	if len(ign) != 1 {
		t.Fatalf("attendu 1 entrée INFO FLIP_IGNORED au premier envoi, a %d : %v", len(ign), ign)
	}
	for _, want := range []string{"client=", "player_id=", "flip_id=", "card=", "motion_card=", "reason=", "NOT_TURN", "f-log-oop", r.ids["p3"]} {
		if !strings.Contains(ign[0], want) {
			t.Errorf("FLIP_IGNORED doit contenir %q : %s", want, ign[0])
		}
	}

	r.flip(t, "p3", "1-1", "f-log-oop", nil) // renvoi : réponse mémorisée
	r.observe(t)
	if n := len(logEntries248(r, game.LogLevelInfo, "FLIP_IGNORED")); n != 1 {
		t.Errorf("le renvoi ne journalise pas un second FLIP_IGNORED (a %d)", n)
	}
	dup := logEntries248(r, game.LogLevelInfo, "FLIP_DUPLICATE")
	if len(dup) != 1 || !strings.Contains(dup[0], "NOT_TURN") || !strings.Contains(dup[0], "IGNORED") {
		t.Errorf("le renvoi est un FLIP_DUPLICATE portant status IGNORED et reason NOT_TURN : %v", dup)
	}
}

func TestFlipID248_Log_AppliedFlipLogsNeitherDuplicateNorIgnored(t *testing.T) {
	r := newLoggedFlipRoom248(t)
	r.flip(t, "p1", "1-1", "f-log-ok", nil)
	r.observe(t)
	if n := len(logEntries248(r, game.LogLevelInfo, "FLIP_DUPLICATE")) + len(logEntries248(r, game.LogLevelInfo, "FLIP_IGNORED")); n != 0 {
		t.Errorf("un premier envoi appliqué ne journalise ni FLIP_DUPLICATE ni FLIP_IGNORED (a %d)", n)
	}
}

func TestFlipID248_Log_InvalidFlipIDIsWarnWithTypeAndLen(t *testing.T) {
	r := newLoggedFlipRoom248(t)
	bad := strings.Repeat("a", 65)
	r.flip(t, "p1", "1-1", bad, nil)
	r.observe(t)

	lines := logEntries248(r, game.LogLevelWarn, "FLIP_ID_INVALID")
	if len(lines) != 1 {
		t.Fatalf("attendu 1 entrée WARN FLIP_ID_INVALID, a %d : %v", len(lines), lines)
	}
	for _, want := range []string{"client=", "type=", "len=", "len=65"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("FLIP_ID_INVALID doit contenir %q : %s", want, lines[0])
		}
	}
	if n := len(logEntries248(r, game.LogLevelInfo, "FLIP_IGNORED")) + len(logEntries248(r, game.LogLevelInfo, "FLIP_DUPLICATE")); n != 0 {
		t.Errorf("un FLIP_ID invalide est traité comme absent : aucun FLIP_IGNORED/FLIP_DUPLICATE (a %d)", n)
	}
}

func TestFlipID248_Log_ValidFlipIDLogsNoInvalidWarning(t *testing.T) {
	r := newLoggedFlipRoom248(t)
	r.flip(t, "p1", "1-1", "f-valide_OK-1", nil)
	r.observe(t)
	if n := len(logEntries248(r, game.LogLevelWarn, "FLIP_ID_INVALID")); n != 0 {
		t.Errorf("FLIP_ID valide : aucun WARN FLIP_ID_INVALID (a %d)", n)
	}
}

// Retour READY → PREPARE (sélection d'équipes devenue non conforme) : la mémoire des FLIP_ID est purgée avec le
// passage en PREPARE (contrat : « purgés au changement de question (passage en PREPARE ou nouvel ID de question) »).
func TestFlipID248_MemoryPurgedWhenTheRoundGoesBackFromReadyToPrepare(t *testing.T) {
	r := newFlipRoom248(t)
	e := r.app.engine
	r.flip(t, "p1", "1-1", "f-prep", nil)
	r.observe(t)

	// STARTED -> READY (setter brut), puis une sélection non conforme ramène la manche en PREPARE
	e.SetPhase(game.PhaseReady)
	if err := e.SetMemoryParticipatingTeams([]string{"TeamA"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams : %v", err)
	}
	if e.GetPhase() != game.PhasePrepare {
		t.Skipf("précondition non atteinte : la sélection d'une seule équipe ne ramène pas en PREPARE (phase %s) ; à ajuster avec dev-backend", e.GetPhase())
	}
	// sélection conforme de nouveau, puis reprise du jeu : la carte 1-1 est toujours visible, la main est à TeamA
	if err := e.SetMemoryParticipatingTeams([]string{"TeamA", "TeamB"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams (rétablissement) : %v", err)
	}
	e.SetPhase(game.PhaseStarted)
	r.observe(t)

	// mémoire intacte => APPLIED + DUPLICATE=true ; mémoire purgée => geste neuf, refusé (carte déjà visible)
	r.flip(t, "p1", "1-1", "f-prep", nil)
	acks := acksOf248(r.observe(t)["p1"])
	if len(acks) != 1 {
		t.Fatalf("attendu 1 ACK, a %+v", acks)
	}
	if acks[0].Duplicate || acks[0].Status == "APPLIED" {
		t.Errorf("le retour en PREPARE doit purger la mémoire : FLIP_ID évalué comme neuf, pas rejoué (a %+v)", acks[0])
	}
	if acks[0].Status != "IGNORED" || acks[0].Reason != "NOT_FLIPPABLE" {
		t.Errorf("évalué comme neuf, le geste est refusé (carte déjà visible) : attendu IGNORED NOT_FLIPPABLE, a %+v", acks[0])
	}
}
