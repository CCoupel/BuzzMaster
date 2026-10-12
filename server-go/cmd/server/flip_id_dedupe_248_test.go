package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

// #248 lot 5b — FLIP_MEMORY_CARD.FLIP_ID (retournement idempotent) et FLIP_MEMORY_CARD_ACK.
// Contrat : contracts/websocket-actions.md §FLIP_MEMORY_CARD (« Idempotence — FLIP_ID ») et §FLIP_MEMORY_CARD_ACK ;
// contracts/logging.md (FLIP_DUPLICATE / FLIP_IGNORED / FLIP_ID_INVALID).
//   - FLIP_ID optionnel, 1–64 caractères [A-Za-z0-9_-], honoré pour un vplayer résolu, indexé par bumper ;
//   - déduplication AVANT la vérification du tour, sous le même verrou que le retournement ;
//   - ACK vers l'émetteur seul (STATUS APPLIED/IGNORED, REASON, DUPLICATE), suivi d'un UPDATE ciblé ;
//   - un doublon ou un geste ignoré ne diffuse RIEN à tous ; un geste appliqué garde les diffusions actuelles ;
//   - 16 FLIP_ID par bumper (le plus ancien oublié), purgés au changement de question, sur NEW_GAME, à la
//     suppression du bumper ; sans FLIP_ID : comportement strictement inchangé (hors tour : silence).
// Les tests passent par des JSON / messages bruts et des littéraux : ils COMPILENT avant le correctif (le champ
// FlipID et l'action n'existent pas encore) et échouent sur le comportement. Harnais : newRoom247
// (speedy_refused_echo_247_test.go), reconfiguré sur une question MEMORY à 2 équipes (CHACUN_SON_TOUR).

const ackAction248 = "FLIP_MEMORY_CARD_ACK"

type ack248 struct {
	FlipID       string
	CardID       string
	MotionCardID string
	Status       string
	Reason       string
	Duplicate    bool
	hasReason    bool
	hasMotion    bool
}

func acksOf248(fs []frame247) []ack248 {
	var out []ack248
	for _, f := range fs {
		if f.Action != ackAction248 {
			continue
		}
		var env struct {
			Msg map[string]json.RawMessage `json:"MSG"`
		}
		if json.Unmarshal(f.Raw, &env) != nil {
			continue
		}
		a := ack248{}
		_ = json.Unmarshal(env.Msg["FLIP_ID"], &a.FlipID)
		_ = json.Unmarshal(env.Msg["CARD_ID"], &a.CardID)
		_, a.hasMotion = env.Msg["MOTION_CARD_ID"]
		_ = json.Unmarshal(env.Msg["MOTION_CARD_ID"], &a.MotionCardID)
		_ = json.Unmarshal(env.Msg["STATUS"], &a.Status)
		_, a.hasReason = env.Msg["REASON"]
		_ = json.Unmarshal(env.Msg["REASON"], &a.Reason)
		_ = json.Unmarshal(env.Msg["DUPLICATE"], &a.Duplicate)
		out = append(out, a)
	}
	return out
}

func indexOfAction248(fs []frame247, action string) int {
	for i, f := range fs {
		if f.Action == action {
			return i
		}
	}
	return -1
}

func lastIndexOfAction248(fs []frame247, action string) int {
	for i := len(fs) - 1; i >= 0; i-- {
		if fs[i].Action == action {
			return i
		}
	}
	return -1
}

// newFlipRoom248 : question MEMORY (3 paires), équipes TeamA (p1, p2) puis TeamB (p3), la main à TeamA,
// délai de retour face cachée court (50 ms), phase STARTED.
func newFlipRoom248(t *testing.T) *room247 {
	t.Helper()
	r := newRoom247(t)
	configureMemory248(t, r, "qmem")
	r.observe(t)
	return r
}

func configureMemory248(t *testing.T, r *room247, questionID string) {
	t.Helper()
	e := r.app.engine
	e.SetPhase(game.PhaseStopped)
	e.Ready(questionID, &game.Question{
		ID: questionID, Question: "Memory ?", Type: game.QuestionTypeMemory,
		TypedContent: game.TypedContent{
			MemoryMode:   string(game.MemoryModeChacunSonTour),
			MemoryConfig: &game.MemoryConfig{FlipDelay: 0.05},
			MemoryPairs:  []game.MemoryPair{{ID: 1}, {ID: 2}, {ID: 3}},
		},
	})
	if err := e.SetMemoryParticipatingTeams([]string{"TeamA", "TeamB"}); err != nil {
		t.Fatalf("SetMemoryParticipatingTeams : %v", err)
	}
	e.SetPhase(game.PhaseStarted)
}

func (r *room247) flip(t *testing.T, who, cardID, flipID string, extra map[string]interface{}) {
	t.Helper()
	payload := map[string]interface{}{"CARD_ID": cardID, "ID": r.ids[who]}
	if flipID != "" {
		payload["FLIP_ID"] = flipID
	}
	for k, v := range extra {
		payload[k] = v
	}
	msg, err := protocol.NewMessage(protocol.ActionFlipMemoryCard, payload)
	if err != nil {
		t.Fatalf("message FLIP_MEMORY_CARD : %v", err)
	}
	r.app.handleFlipMemoryCard(r.cids[who], server.ClientTypeVPlayer, msg)
}

// flipBackSignal248 : événement « cartes remises face cachée » (goroutine de retour), sans Sleep.
func flipBackSignal248(t *testing.T) chan struct{} {
	t.Helper()
	ch := make(chan struct{}, 16)
	setTestInjectMemoryFlipBackPanic(func() {
		select {
		case ch <- struct{}{}:
		default:
		}
	})
	t.Cleanup(clearTestInjectMemoryFlipBackPanic)
	return ch
}

// waitFlipBack248 attend le retour face cachée PUIS l'UPDATE qu'il diffuse (lu sur toutes les connexions, pour
// qu'il ne pollue pas l'observation suivante).
func waitFlipBack248(t *testing.T, r *room247, signal chan struct{}) map[string][]frame247 {
	t.Helper()
	consumed := map[string][]frame247{}
	select {
	case <-signal:
	case <-time.After(10 * time.Second):
		t.Fatal("la paire ratée n'est jamais retournée face cachée (échéance 10 s)")
	}
	deadline := time.After(10 * time.Second)
	for name, ch := range r.msgs {
		for done := false; !done; {
			select {
			case f, ok := <-ch:
				if !ok {
					t.Fatalf("%s : connexion fermée", name)
				}
				consumed[name] = append(consumed[name], f)
				if f.Action != "UPDATE" {
					continue
				}
				var env struct {
					Msg struct {
						Game struct {
							Flipped []string `json:"MEMORY_FLIPPED_CARDS"`
							Active  *struct {
								State struct {
									Flipped []string `json:"MEMORY_FLIPPED_CARDS"`
								} `json:"STATE"`
							} `json:"MEMOTION_ACTIVE"`
						} `json:"GAME"`
					} `json:"MSG"`
				}
				if json.Unmarshal(f.Raw, &env) != nil {
					continue
				}
				motionFlipped := 0
				if env.Msg.Game.Active != nil {
					motionFlipped = len(env.Msg.Game.Active.State.Flipped)
				}
				if len(env.Msg.Game.Flipped) == 0 && motionFlipped == 0 {
					done = true
				}
			case <-deadline:
				t.Fatalf("%s : UPDATE du retour face cachée non reçu", name)
			}
		}
	}
	return consumed
}

func flipped248(r *room247) []string { return r.app.engine.GetState().MemoryFlippedCards }

// requireQuiet248 : aucun message pour les clients listés (diffusion générale interdite).
func requireQuiet248(t *testing.T, got map[string][]frame247, names ...string) {
	t.Helper()
	for _, n := range names {
		if len(got[n]) != 0 {
			t.Errorf("%s ne doit recevoir AUCUN message, a reçu %v", n, got[n])
		}
	}
}

// --- ACK vers l'émetteur seul, puis UPDATE ciblé ---------------------------------------------

func TestFlipID248_AppliedFlip_AckToSenderOnlyThenTargetedUpdate(t *testing.T) {
	r := newFlipRoom248(t)
	r.flip(t, "p1", "1-1", "f-first-1", nil)
	got := r.observe(t)

	acks := acksOf248(got["p1"])
	if len(acks) != 1 {
		t.Fatalf("p1 doit recevoir exactement 1 FLIP_MEMORY_CARD_ACK, a reçu %d (trames : %v)", len(acks), got["p1"])
	}
	a := acks[0]
	if a.FlipID != "f-first-1" || a.CardID != "1-1" || a.Status != "APPLIED" || a.Reason != "" || a.Duplicate {
		t.Errorf("ACK = %+v, attendu FLIP_ID=f-first-1 CARD_ID=1-1 APPLIED REASON='' DUPLICATE=false", a)
	}
	if !a.hasReason || !a.hasMotion {
		t.Errorf("REASON et MOTION_CARD_ID doivent être TOUJOURS sérialisés (reason=%v motion=%v)", a.hasReason, a.hasMotion)
	}
	if ackAt, updAt := indexOfAction248(got["p1"], ackAction248), lastIndexOfAction248(got["p1"], "UPDATE"); updAt < ackAt || updAt < 0 {
		t.Errorf("l'ACK doit être SUIVI d'un UPDATE ciblé (ACK à %d, dernier UPDATE à %d) : %v", ackAt, updAt, got["p1"])
	}
	for _, n := range []string{"p2", "p3", "admin", "tv", "anim"} {
		if len(acksOf248(got[n])) != 0 {
			t.Errorf("%s ne doit JAMAIS recevoir l'ACK (émetteur seul) : %v", n, got[n])
		}
	}
	if len(flipped248(r)) != 1 {
		t.Errorf("la carte doit être retournée une fois : %v", flipped248(r))
	}
}

// --- Idempotence -----------------------------------------------------------------------------

func TestFlipID248_ResendAfterMissedPair_DoesNotFlipTheCardASecondTime(t *testing.T) {
	r := newFlipRoom248(t)
	signal := flipBackSignal248(t)
	r.flip(t, "p1", "1-1", "f-a", nil)
	r.observe(t)
	r.flip(t, "p1", "2-1", "f-b", nil) // paire ratée : retour face cachée, la main passe à TeamB
	r.observe(t)
	waitFlipBack248(t, r, signal)
	if len(flipped248(r)) != 0 {
		t.Fatalf("précondition : cartes remises face cachée, a %v", flipped248(r))
	}

	r.flip(t, "p1", "2-1", "f-b", nil) // renvoi (accusé perdu) : la main est passée, la carte est face cachée
	got := r.observe(t)

	if len(flipped248(r)) != 0 {
		t.Errorf("le renvoi ne doit PAS retourner la carte une seconde fois : %v", flipped248(r))
	}
	acks := acksOf248(got["p1"])
	if len(acks) != 1 || acks[0].Status != "APPLIED" || !acks[0].Duplicate || acks[0].Reason != "" {
		t.Fatalf("attendu 1 ACK APPLIED DUPLICATE=true (la déduplication passe avant le tour, pas de NOT_TURN à tort), a %+v (trames : %v)", acks, got["p1"])
	}
	requireQuiet248(t, got, "p2", "p3", "admin", "tv", "anim") // doublon : aucune diffusion générale
	if lastIndexOfAction248(got["p1"], "UPDATE") < indexOfAction248(got["p1"], ackAction248) {
		t.Errorf("l'ACK doit être suivi de l'UPDATE ciblé : %v", got["p1"])
	}
	if indexOfAction248(got["p1"], "PAUSE") >= 0 {
		t.Error("jamais de PAUSE pour un doublon")
	}
}

func TestFlipID248_ResendBeforeAnythingChanges_IsADuplicateWithoutBroadcast(t *testing.T) {
	r := newFlipRoom248(t)
	r.flip(t, "p1", "1-1", "f-dup", nil)
	r.observe(t)
	r.flip(t, "p1", "1-1", "f-dup", nil)
	got := r.observe(t)

	acks := acksOf248(got["p1"])
	if len(acks) != 1 || acks[0].Status != "APPLIED" || !acks[0].Duplicate {
		t.Fatalf("attendu ACK APPLIED DUPLICATE=true, a %+v", acks)
	}
	if n := len(flipped248(r)); n != 1 {
		t.Errorf("une seule carte retournée (pas de double retournement) : %v", flipped248(r))
	}
	requireQuiet248(t, got, "p2", "p3", "admin", "tv", "anim")
}

func TestFlipID248_ReconnectionSameBumperNewConnection_IsDeduplicated(t *testing.T) {
	// mémoire indexée par BUMPER, pas par connexion : le renvoi par une autre connexion est dédupliqué
	r := newFlipRoom248(t)
	r.flip(t, "p1", "1-1", "f-conn", nil)
	r.observe(t)
	msg, _ := protocol.NewMessage(protocol.ActionFlipMemoryCard, map[string]interface{}{"CARD_ID": "1-1", "ID": r.ids["p1"], "FLIP_ID": "f-conn"})
	r.app.handleFlipMemoryCard(r.cids["p2"], server.ClientTypeVPlayer, msg) // autre clientID, même bumper p1
	got := r.observe(t)
	if n := len(flipped248(r)); n != 1 {
		t.Errorf("renvoi par une autre connexion : la carte ne doit pas être retournée à nouveau : %v", flipped248(r))
	}
	// l'ACK va à la connexion émettrice (celle du renvoi), dédupliqué parce que la mémoire est celle du BUMPER
	acks := acksOf248(got["p2"])
	if len(acks) != 1 || acks[0].Status != "APPLIED" || !acks[0].Duplicate {
		t.Errorf("renvoi par une autre connexion : attendu 1 ACK APPLIED DUPLICATE=true sur la connexion émettrice, a %+v", acks)
	}
}

// --- Refus réels : IGNORED + REASON ----------------------------------------------------------

func TestFlipID248_NotFlippable_IsIgnoredWithReason_AndTheAnswerIsRemembered(t *testing.T) {
	r := newFlipRoom248(t)
	r.flip(t, "p1", "1-1", "f-ok", nil)
	r.observe(t)

	r.flip(t, "p1", "1-1", "f-again", nil) // carte déjà face visible
	got := r.observe(t)
	acks := acksOf248(got["p1"])
	if len(acks) != 1 || acks[0].Status != "IGNORED" || acks[0].Reason != "NOT_FLIPPABLE" || acks[0].Duplicate {
		t.Fatalf("attendu 1 ACK IGNORED NOT_FLIPPABLE DUPLICATE=false, a %+v", acks)
	}
	requireQuiet248(t, got, "p2", "p3", "admin", "tv", "anim") // geste ignoré : aucune diffusion générale
	if n := len(flipped248(r)); n != 1 {
		t.Errorf("l'état ne doit pas changer : %v", flipped248(r))
	}

	// la réponse d'un geste ignoré est mémorisée : son renvoi donne la même réponse, marquée DUPLICATE
	r.flip(t, "p1", "1-1", "f-again", nil)
	again := acksOf248(r.observe(t)["p1"])
	if len(again) != 1 || again[0].Status != "IGNORED" || again[0].Reason != "NOT_FLIPPABLE" || !again[0].Duplicate {
		t.Errorf("renvoi d'un geste ignoré : attendu IGNORED NOT_FLIPPABLE DUPLICATE=true, a %+v", again)
	}
}

func TestFlipID248_NotTheTeamsTurn_FirstSendIsIgnoredNotTurn(t *testing.T) {
	r := newFlipRoom248(t) // la main est à TeamA ; p3 est TeamB
	r.flip(t, "p3", "1-1", "f-oop", nil)
	got := r.observe(t)
	acks := acksOf248(got["p3"])
	if len(acks) != 1 || acks[0].Status != "IGNORED" || acks[0].Reason != "NOT_TURN" || acks[0].Duplicate {
		t.Fatalf("attendu 1 ACK IGNORED NOT_TURN, a %+v", acks)
	}
	requireQuiet248(t, got, "p1", "p2", "admin", "tv", "anim")
	if n := len(flipped248(r)); n != 0 {
		t.Errorf("aucun retournement attendu : %v", flipped248(r))
	}
}

func TestFlipID248_NotPlayablePhase_IsIgnoredNotPlayable(t *testing.T) {
	r := newFlipRoom248(t)
	r.app.engine.SetPhase(game.PhaseStopped)
	r.flip(t, "p1", "1-1", "f-stopped", nil)
	got := r.observe(t)
	acks := acksOf248(got["p1"])
	if len(acks) != 1 || acks[0].Status != "IGNORED" || acks[0].Reason != "NOT_PLAYABLE" {
		t.Fatalf("attendu 1 ACK IGNORED NOT_PLAYABLE (phase ≠ STARTED), a %+v", acks)
	}
	requireQuiet248(t, got, "p2", "p3", "admin", "tv", "anim")
}

func TestFlipID248_InvalidCardScope_IsIgnoredCardScope(t *testing.T) {
	r := newFlipRoom248(t) // question MEMORY : aucune carte MEMOTION active
	r.flip(t, "p1", "1-1", "f-scope", map[string]interface{}{"MOTION_CARD_ID": "card-inconnue"})
	acks := acksOf248(r.observe(t)["p1"])
	if len(acks) != 1 || acks[0].Status != "IGNORED" || acks[0].Reason != "CARD_SCOPE" || acks[0].MotionCardID != "card-inconnue" {
		t.Fatalf("attendu 1 ACK IGNORED CARD_SCOPE avec MOTION_CARD_ID repris, a %+v", acks)
	}
	if n := len(flipped248(r)); n != 0 {
		t.Errorf("portée invalide : aucun retournement : %v", flipped248(r))
	}
}

// --- Sans FLIP_ID / FLIP_ID invalide : comportement actuel, gardes de non-régression -------------

func TestFlipID248_WithoutFlipID_BehaviourUnchanged_NoAck_GeneralBroadcast(t *testing.T) {
	r := newFlipRoom248(t)
	r.flip(t, "p1", "1-1", "", nil)
	got := r.observe(t)
	if len(acksOf248(got["p1"])) != 0 {
		t.Errorf("sans FLIP_ID : aucun ACK : %v", got["p1"])
	}
	for _, n := range []string{"admin", "tv", "anim", "p2", "p3"} {
		if indexOfAction248(got[n], "UPDATE") < 0 {
			t.Errorf("sans FLIP_ID : l'UPDATE général actuel doit toujours atteindre %s : %v", n, got[n])
		}
	}
	if n := len(flipped248(r)); n != 1 {
		t.Errorf("la carte doit être retournée : %v", flipped248(r))
	}
}

func TestFlipID248_InvalidFlipID_TreatedAsAbsent(t *testing.T) {
	for _, bad := range []string{"id avec espaces", "id!", strings.Repeat("a", 65), "é-accent"} {
		r := newFlipRoom248(t)
		r.flip(t, "p1", "1-1", bad, nil)
		got := r.observe(t)
		if len(acksOf248(got["p1"])) != 0 {
			t.Errorf("FLIP_ID %q hors format : traité comme absent, aucun ACK : %v", bad, got["p1"])
		}
		if indexOfAction248(got["admin"], "UPDATE") < 0 {
			t.Errorf("FLIP_ID %q hors format : comportement actuel (UPDATE général) attendu", bad)
		}
		if n := len(flipped248(r)); n != 1 {
			t.Errorf("FLIP_ID %q hors format : la carte doit être retournée comme avant : %v", bad, flipped248(r))
		}
	}
}

func TestFlipID248_OutOfTurnWithoutFlipID_StaysSilent(t *testing.T) {
	r := newFlipRoom248(t)
	r.flip(t, "p3", "1-1", "", nil) // dérogation du 2026-08-24 : tap hors tour sans FLIP_ID = silence
	got := r.observe(t)
	for name, fs := range got {
		if len(fs) != 0 {
			t.Errorf("%s ne doit rien recevoir pour un tap hors tour sans FLIP_ID : %v", name, fs)
		}
	}
	if n := len(flipped248(r)); n != 0 {
		t.Errorf("aucun retournement : %v", flipped248(r))
	}
}

func TestFlipID248_TvAndAnim_FlipIDIgnored_NoAck(t *testing.T) {
	r := newFlipRoom248(t)
	msg, _ := protocol.NewMessage(protocol.ActionFlipMemoryCard, map[string]interface{}{"CARD_ID": "1-1", "FLIP_ID": "f-tv"})
	r.app.handleFlipMemoryCard("test-tv", server.ClientTypeTV, msg)
	got := r.observe(t)
	for name, fs := range got {
		if len(acksOf248(fs)) != 0 {
			t.Errorf("%s : un FLIP_ID de la TV est ignoré (pas d'ACK) : %v", name, fs)
		}
	}
	if n := len(flipped248(r)); n != 1 {
		t.Errorf("la TV garde son comportement actuel (retournement) : %v", flipped248(r))
	}
}

// --- Mémoire : purge et plafond --------------------------------------------------------------

func TestFlipID248_MemoryPurgedOnQuestionChange(t *testing.T) {
	r := newFlipRoom248(t)
	r.flip(t, "p1", "1-1", "f-purge", nil)
	r.observe(t)
	configureMemory248(t, r, "qmem2") // autre question : la mémoire est purgée
	r.observe(t)
	r.flip(t, "p1", "1-1", "f-purge", nil)
	acks := acksOf248(r.observe(t)["p1"])
	if len(acks) != 1 || acks[0].Duplicate || acks[0].Status != "APPLIED" {
		t.Errorf("après changement de question, le même FLIP_ID est évalué comme neuf : attendu APPLIED DUPLICATE=false, a %+v", acks)
	}
}

func TestFlipID248_MemoryPurgedOnNewGame(t *testing.T) {
	r := newFlipRoom248(t)
	r.flip(t, "p1", "1-1", "f-ng", nil)
	r.observe(t)
	r.app.engine.InitGameWith(game.NewGameOptions{PurgeScores: false, PurgePlayers: false, PurgeTeams: false, ResetQuestions: false})
	configureMemory248(t, r, "qmem") // MÊME identifiant de question : seule la purge NEW_GAME peut vider la mémoire
	r.observe(t)
	r.flip(t, "p1", "1-1", "f-ng", nil)
	acks := acksOf248(r.observe(t)["p1"])
	if len(acks) != 1 || acks[0].Duplicate || acks[0].Status != "APPLIED" {
		t.Errorf("après NEW_GAME, le FLIP_ID est évalué comme neuf : attendu APPLIED DUPLICATE=false, a %+v", acks)
	}
}

func TestFlipID248_MemoryKeeps16PerBumper_OldestForgotten(t *testing.T) {
	r := newFlipRoom248(t)
	// 17 gestes distincts : le 1er est appliqué, les 16 suivants sont ignorés (carte déjà visible) mais mémorisés aussi
	for i := 1; i <= 17; i++ {
		r.flip(t, "p1", "1-1", fmt.Sprintf("f-%02d", i), nil)
	}
	r.observe(t)

	// f-02 est toujours mémorisé (parmi les 16 derniers) : sa réponse mémorisée est rejouée
	r.flip(t, "p1", "1-1", "f-02", nil)
	recent := acksOf248(r.observe(t)["p1"])
	if len(recent) != 1 || !recent[0].Duplicate || recent[0].Status != "IGNORED" {
		t.Errorf("f-02 (récent) : attendu IGNORED DUPLICATE=true, a %+v", recent)
	}
	// f-01 (le plus ancien) est oublié : réévalué comme un geste NEUF, refusé (carte déjà visible) => DUPLICATE=false
	r.flip(t, "p1", "1-1", "f-01", nil)
	oldest := acksOf248(r.observe(t)["p1"])
	if len(oldest) != 1 || oldest[0].Duplicate || oldest[0].Status != "IGNORED" {
		t.Errorf("f-01 (le plus ancien, oublié au 17e) : attendu IGNORED DUPLICATE=false, a %+v", oldest)
	}
}

// --- Concurrence : un seul retournement, un seul ACK « neuf » (lancer aussi avec -race) -------------

func TestFlipID248_ConcurrentCopiesOfTheSameFlipID_FlipOnce_Race(t *testing.T) {
	for iter := 0; iter < 20; iter++ {
		r := newFlipRoom248(t)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				r.flip(t, "p1", "1-1", "f-race", nil)
			}()
		}
		close(start)
		wg.Wait()
		got := r.observe(t)

		if n := len(flipped248(r)); n != 1 {
			t.Fatalf("itération %d : 4 copies du même FLIP_ID doivent retourner UNE carte, a %v", iter, flipped248(r))
		}
		acks := acksOf248(got["p1"])
		fresh, dup := 0, 0
		for _, a := range acks {
			if a.Status != "APPLIED" {
				t.Fatalf("itération %d : toutes les copies sont APPLIED, a %+v", iter, a)
			}
			if a.Duplicate {
				dup++
			} else {
				fresh++
			}
		}
		if len(acks) != 4 || fresh != 1 || dup != 3 {
			t.Fatalf("itération %d : attendu 4 ACK dont 1 neuf et 3 DUPLICATE, a %d ACK (neufs=%d, doublons=%d)", iter, len(acks), fresh, dup)
		}
	}
}

func TestFlipID248_ResendRacingWithTheFlipBackGoroutine_NeverFlipsTwice_Race(t *testing.T) {
	r := newFlipRoom248(t)
	signal := flipBackSignal248(t)
	r.flip(t, "p1", "1-1", "f-a", nil)
	r.flip(t, "p1", "2-1", "f-b", nil) // paire ratée : la goroutine de retour face cachée démarre
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.flip(t, "p1", "2-1", "f-b", nil) // renvois pendant / après le retour face cachée
		}()
	}
	wg.Wait()
	consumed := waitFlipBack248(t, r, signal)
	rest := r.observe(t)
	if n := len(flipped248(r)); n != 0 {
		t.Errorf("les renvois ne doivent jamais retourner de carte (état final attendu : tout face cachée), a %v", flipped248(r))
	}
	acks := acksOf248(append(append([]frame247{}, consumed["p1"]...), rest["p1"]...))
	fresh, dup := 0, 0
	for _, a := range acks {
		if a.Status != "APPLIED" {
			t.Errorf("tous les ACK de ce scénario sont APPLIED (pas de NOT_TURN à tort après le passage de main) : %+v", a)
		}
		if a.Duplicate {
			dup++
		} else {
			fresh++
		}
	}
	if fresh != 2 || dup != 6 {
		t.Errorf("attendu 2 gestes neufs (f-a, f-b) et 6 doublons, a neufs=%d doublons=%d", fresh, dup)
	}
}
