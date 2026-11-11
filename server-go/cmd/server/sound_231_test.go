// #231 (v11.6.0, LOT-3) — bruitages fins par type de question : UNE cue par site d'émission (D4), jamais de doublon
// avec une cue globale, filtrage `cues_disabled` / `muted` sur les nouvelles cues.
// Source normative : contracts/sound.md §2.1 « Amendement v11.6.0 (#231) » (table des 8 cues et de leurs sites),
// §6.2/§6.3 (fan-out unique, CuesDisabled). Écrit depuis le contrat avant le code (rouge attendu).
//
// Frontière : les cues sont désignées par leur NOM de contrat (audio.Cue("qcm-indice")…) — le fichier compile
// avant le dev. On pilote les vrais sites (broadcastQCMHint, handleFlipMemoryCard, handlers MEMOTION/RAFALE via
// handleWebMessage) avec le moteur son sur un FakeOutput (twa227WireSound) et on lit les cues JOUÉES.
// Préfixe tw231.
package main

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
)

const (
	tw231QCMIndice         = audio.Cue("qcm-indice")
	tw231MemoryCarte       = audio.Cue("memory-carte")
	tw231MemoryPaire       = audio.Cue("memory-paire")
	tw231MemotionSelection = audio.Cue("memotion-selection")
	tw231MemotionFinCarte  = audio.Cue("memotion-fin-carte")
	tw231RafaleBonne       = audio.Cue("rafale-bonne")
	tw231RafaleMauvaise    = audio.Cue("rafale-mauvaise")
	tw231RafaleFinLot      = audio.Cue("rafale-fin-lot")
)

// tw231Settle renvoie les cues jouées SANS `depart`. Attente SUR CONDITION : si des cues sont attendues (expect),
// on sonde (≤ 5 s) jusqu'à ce que chacune ait été jouée, puis on laisse une courte grâce pour qu'une cue en trop
// (doublon, D4) ait le temps de se manifester. Sans attente (cas « silence »), simple fenêtre d'observation.
func tw231Settle(fake *audio.FakeOutput, expect ...audio.Cue) []audio.Cue {
	played := func() []audio.Cue {
		var out []audio.Cue
		for _, c := range fake.Played() {
			if c != audio.CueDepart {
				out = append(out, c)
			}
		}
		return out
	}
	if len(expect) == 0 {
		time.Sleep(300 * time.Millisecond)
		return played()
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got := played()
		all := true
		for _, e := range expect {
			if tw231Count(got, e) == 0 {
				all = false
				break
			}
		}
		if all {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(150 * time.Millisecond)
	return played()
}

func tw231Count(cues []audio.Cue, want audio.Cue) int {
	n := 0
	for _, c := range cues {
		if c == want {
			n++
		}
	}
	return n
}

// tw231Wire : twa227WireSound + hub de logs (le callback du logger diffuse vers ce hub pendant un dispatch).
func tw231Wire(t *testing.T) (*App, *audio.FakeOutput) {
	t.Helper()
	app, fake := twa227WireSound(t)
	app.logsHub = server.NewLogsWebSocketHub(10)
	return app, fake
}

func tw231SetCuesDisabled(t *testing.T, names ...string) {
	t.Helper()
	m := map[string]bool{}
	for _, n := range names {
		m[n] = true
	}
	raw, _ := json.Marshal(map[string]interface{}{"sound": map[string]interface{}{"cues_disabled": m}})
	if err := config.PatchJSON(raw); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { config.Mutate(func(c *config.Config) { c.Sound.CuesDisabled = nil }) })
}

// ---------------------------------------------------------------------------
// QCM — un indice barre une réponse : `qcm-indice`, une seule fois.
// ---------------------------------------------------------------------------

func TestSounds231_QCMHint_PlaysQcmIndiceOnce(t *testing.T) {
	config.Mutate(func(c *config.Config) { c.Sound.CuesDisabled = nil })
	app, fake := tw231Wire(t)
	app.broadcastQCMHint("RED", 2)
	got := tw231Settle(fake, tw231QCMIndice)
	if len(got) != 1 || got[0] != tw231QCMIndice {
		t.Fatalf("un indice QCM doit jouer exactement [qcm-indice], got %v", got)
	}
}

func TestSounds231_QCMHint_CueDisabled_Silent_OthersUnaffected(t *testing.T) {
	app, fake := tw231Wire(t)
	tw231SetCuesDisabled(t, "qcm-indice")
	app.broadcastQCMHint("RED", 2)
	if got := tw231Settle(fake); len(got) != 0 {
		t.Fatalf("qcm-indice désactivée par cues_disabled : aucune cue attendue, got %v", got)
	}
}

func TestSounds231_QCMHint_Muted_Silent(t *testing.T) {
	config.Mutate(func(c *config.Config) { c.Sound.CuesDisabled = nil })
	app, fake := tw231Wire(t)
	if err := config.PatchJSON([]byte(`{"sound":{"muted":true}}`)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = config.PatchJSON([]byte(`{"sound":{"muted":false}}`)) })
	app.broadcastQCMHint("RED", 2)
	if got := tw231Settle(fake); len(got) != 0 {
		t.Fatalf("son coupé : aucune cue attendue, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// MEMORY — carte retournée / paire trouvée / paire ratée garde `perdu`.
// ---------------------------------------------------------------------------

func tw231MemoryApp(t *testing.T) (*App, *audio.FakeOutput) {
	t.Helper()
	config.Mutate(func(c *config.Config) { c.Sound.CuesDisabled = nil })
	app, fake := tw231Wire(t)
	q := &game.Question{
		ID: "q1", Type: game.QuestionTypeMemory,
		TypedContent: game.TypedContent{
			MemoryPairs: []game.MemoryPair{
				{ID: 1, Card1: game.MemoryCard{Text: "A"}, Card2: game.MemoryCard{Text: "A"}},
				{ID: 2, Card1: game.MemoryCard{Text: "B"}, Card2: game.MemoryCard{Text: "B"}},
				{ID: 3, Card1: game.MemoryCard{Text: "C"}, Card2: game.MemoryCard{Text: "C"}},
			},
			MemoryConfig: &game.MemoryConfig{FlipDelay: 0.05},
		},
	}
	app.engine.Ready("q1", q)
	app.engine.SetTeams(map[string]*game.Team{"TeamA": {Name: "TeamA"}})
	if err := app.engine.SetMemoryParticipatingTeams([]string{"TeamA"}); err != nil {
		t.Fatalf("setup: %v", err)
	}
	app.engine.SetPhase(game.PhaseStarted)
	return app, fake
}

func tw231Flip(t *testing.T, app *App, cardID string) {
	t.Helper()
	msg, err := protocol.NewMessage(protocol.ActionFlipMemoryCard, protocol.FlipMemoryCardPayload{CardID: cardID})
	if err != nil {
		t.Fatal(err)
	}
	app.handleFlipMemoryCard("test-admin", server.ClientTypeAdmin, msg)
}

func TestSounds231_MemoryFirstCardFlip_PlaysMemoireCarteOnly(t *testing.T) {
	app, fake := tw231MemoryApp(t)
	tw231Flip(t, app, "1-1")
	got := tw231Settle(fake, tw231MemoryCarte)
	if len(got) != 1 || got[0] != tw231MemoryCarte {
		t.Fatalf("une carte retournée (1re de la paire) doit jouer exactement [memory-carte], got %v", got)
	}
}

func TestSounds231_MemoryPairFound_PlaysMemoryPaire_NeverPerdu(t *testing.T) {
	app, fake := tw231MemoryApp(t)
	tw231Flip(t, app, "1-1")
	tw231Flip(t, app, "1-2")
	got := tw231Settle(fake, tw231MemoryCarte, tw231MemoryPaire)
	if tw231Count(got, tw231MemoryPaire) != 1 {
		t.Errorf("une paire trouvée doit jouer memory-paire UNE fois, got %v", got)
	}
	if tw231Count(got, audio.CuePerdu) != 0 {
		t.Errorf("une paire trouvée ne joue jamais perdu, got %v", got)
	}
}

// D4 : le site « carte retournée » n'émet qu'UNE cue par retournement — la 2e carte d'une paire trouvée ne doit
// pas jouer à la fois memory-carte ET memory-paire.
func TestSounds231_MemoryPairFound_OneCuePerFlip(t *testing.T) {
	app, fake := tw231MemoryApp(t)
	tw231Flip(t, app, "1-1")
	tw231Flip(t, app, "1-2")
	got := tw231Settle(fake, tw231MemoryCarte, tw231MemoryPaire)
	if len(got) != 2 {
		t.Fatalf("deux retournements ⇒ exactement 2 cues (une par site/événement, D4), got %v", got)
	}
	if got[0] != tw231MemoryCarte {
		t.Errorf("la 1re cue (1re carte) doit être memory-carte, got %v", got[0])
	}
}

func TestSounds231_MemoryPairMissed_KeepsPerdu_NoMemoryPaire(t *testing.T) {
	app, fake := tw231MemoryApp(t)
	tw231Flip(t, app, "1-1")
	tw231Flip(t, app, "2-1") // paires différentes ⇒ raté
	got := tw231Settle(fake, audio.CuePerdu)
	if tw231Count(got, audio.CuePerdu) != 1 {
		t.Errorf("une paire ratée garde perdu (une fois), got %v", got)
	}
	if tw231Count(got, tw231MemoryPaire) != 0 {
		t.Errorf("une paire ratée ne joue pas memory-paire, got %v", got)
	}
}

func TestSounds231_Memory_CueDisabled_Silent(t *testing.T) {
	app, fake := tw231MemoryApp(t)
	tw231SetCuesDisabled(t, "memory-carte", "memory-paire")
	tw231Flip(t, app, "1-1")
	tw231Flip(t, app, "1-2")
	if got := tw231Settle(fake); len(got) != 0 {
		t.Fatalf("memory-carte et memory-paire désactivées : aucune cue, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// MEMOTION — carte choisie / carte fermée sans gagnant ; avec gagnant : `gagne` inchangé (D4).
// ---------------------------------------------------------------------------

func tw231MemotionApp(t *testing.T) (*App, *audio.FakeOutput) {
	t.Helper()
	config.Mutate(func(c *config.Config) { c.Sound.CuesDisabled = nil })
	app, fake := tw231Wire(t)
	app.engine.SetTeams(map[string]*game.Team{"red": {Name: "red", Color: []int{255, 0, 0}}})
	card := func(id string) game.MotionCard { return game.MotionCard{ID: id, RectoTheme: id, Difficulty: 1} }
	q := &game.Question{
		ID: "mq1", Question: "MEMOTION", Type: game.QuestionTypeMemotion,
		MotionCards: []game.MotionCard{card("c1"), card("c2")}, MotionMode: string(game.MemoryModeSolo),
		Points: "10", Time: "0",
	}
	app.engine.Ready(q.ID, q)
	if err := app.engine.SetMotionParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("setup: SetMotionParticipatingTeams : %v", err)
	}
	app.engine.StartImmediate(0)
	t.Cleanup(func() { app.engine.Stop() })
	return app, fake
}

func TestSounds231_MotionSelect_PlaysMemotionSelection(t *testing.T) {
	app, fake := tw231MemotionApp(t)
	dispatchAs(t, app, server.ClientTypeAnim, protocol.ActionMotionSelect, protocol.MotionSelectPayload{CardID: "c1"})
	got := tw231Settle(fake, tw231MemotionSelection)
	if len(got) != 1 || got[0] != tw231MemotionSelection {
		t.Fatalf("choisir une carte doit jouer exactement [memotion-selection], got %v", got)
	}
}

func TestSounds231_MotionDone_NoWinner_PlaysMemotionFinCarte_NotGagne(t *testing.T) {
	app, fake := tw231MemotionApp(t)
	dispatchAs(t, app, server.ClientTypeAnim, protocol.ActionMotionSelect, protocol.MotionSelectPayload{CardID: "c1"})
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionMotionFlip, nil)
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionMotionReveal, nil)
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionMotionDone, protocol.MotionDonePayload{CardID: "c1"})
	got := tw231Settle(fake, tw231MemotionFinCarte)
	if tw231Count(got, tw231MemotionFinCarte) != 1 {
		t.Errorf("fermeture de carte SANS gagnant : memotion-fin-carte attendue une fois, got %v", got)
	}
	if tw231Count(got, audio.CueGagne) != 0 {
		t.Errorf("sans gagnant, jamais gagne, got %v", got)
	}
}

func TestSounds231_MotionDone_WithWinner_PlaysGagne_NotFinCarte(t *testing.T) {
	app, fake := tw231MemotionApp(t)
	dispatchAs(t, app, server.ClientTypeAnim, protocol.ActionMotionSelect, protocol.MotionSelectPayload{CardID: "c1"})
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionMotionFlip, nil)
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionMotionReveal, nil)
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionMotionDone, protocol.MotionDonePayload{CardID: "c1", WinnerTeam: "red"})
	got := tw231Settle(fake, audio.CueGagne)
	if tw231Count(got, audio.CueGagne) != 1 {
		t.Errorf("avec gagnant : gagne inchangé (une fois), got %v", got)
	}
	if tw231Count(got, tw231MemotionFinCarte) != 0 {
		t.Errorf("avec gagnant : memotion-fin-carte ne joue pas (D4, une cue par site), got %v", got)
	}
}

// Non-régression : l'expiration du chrono d'une carte reste SILENCIEUSE (TestSoundChain_MotionCardTimerExpiry).
func TestSounds231_Memotion_CueDisabled_Silent(t *testing.T) {
	app, fake := tw231MemotionApp(t)
	tw231SetCuesDisabled(t, "memotion-selection", "memotion-fin-carte")
	dispatchAs(t, app, server.ClientTypeAnim, protocol.ActionMotionSelect, protocol.MotionSelectPayload{CardID: "c1"})
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionMotionFlip, nil)
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionMotionReveal, nil)
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionMotionDone, protocol.MotionDonePayload{CardID: "c1"})
	if got := tw231Settle(fake); len(got) != 0 {
		t.Fatalf("cues MEMOTION désactivées : aucune cue, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// RAFALE — bonne / mauvaise (remplace perdu) / fin de lot (temps-ecoule reste global).
// ---------------------------------------------------------------------------

func tw231RafaleApp(t *testing.T, maxQuestions int, startDelay ...int) (*App, *audio.FakeOutput) {
	t.Helper()
	config.Mutate(func(c *config.Config) { c.Sound.CuesDisabled = nil })
	app, fake := tw231Wire(t)
	app.engine.SetTeams(map[string]*game.Team{"red": {Name: "red", Color: []int{255, 0, 0}}})
	for i := 1; i <= 10; i++ {
		if _, err := app.engine.UpsertRafaleQuestion(game.RafaleQuestion{
			ID: fmt.Sprintf("r-%d", i), Question: fmt.Sprintf("Q%d", i), Answer: fmt.Sprintf("A%d", i),
			Category: game.CategoryHistory, Difficulty: 1,
		}); err != nil {
			t.Fatalf("seed reservoir : %v", err)
		}
	}
	q := &game.Question{
		ID: "rq1", Question: "RAFALE", Type: game.QuestionTypeRafale, Category: game.CategoryHistory,
		Points: "10", Time: "120",
		TypedContent: game.TypedContent{
			RafaleDifficulty: 1, RafaleMode: string(game.RafaleModeChacunSonTour),
			RafaleQuestionTime: 30, RafaleMaxQuestions: maxQuestions,
		},
	}
	app.engine.Ready("rq1", q)
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionRafaleSetTeams, protocol.RafaleSetTeamsPayload{Teams: []string{"red"}})
	delay := 0
	if len(startDelay) > 0 {
		delay = startDelay[0]
	}
	app.engine.StartImmediate(delay)
	t.Cleanup(func() { app.engine.Stop() })
	twa227WaitForPlayed(t, fake, 1, time.Second) // depart
	return app, fake
}

func TestSounds231_RafaleValidate_PlaysRafaleBonne(t *testing.T) {
	app, fake := tw231RafaleApp(t, 100)
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionRafaleValidate, nil)
	got := tw231Settle(fake, tw231RafaleBonne)
	if tw231Count(got, tw231RafaleBonne) != 1 {
		t.Errorf("RAFALE_VALIDATE : rafale-bonne attendue une fois, got %v", got)
	}
	if tw231Count(got, audio.CueGagne) > 1 {
		t.Errorf("gagne ne doit pas être doublé par rafale-bonne, got %v", got)
	}
}

// D4 : sur le site d'invalidation, `perdu` est REMPLACÉ par `rafale-mauvaise` (une seule cue par site).
func TestSounds231_RafaleInvalidate_PlaysRafaleMauvaise_InsteadOfPerdu(t *testing.T) {
	app, fake := tw231RafaleApp(t, 100)
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionRafaleInvalidate, nil)
	got := tw231Settle(fake, tw231RafaleMauvaise)
	if tw231Count(got, tw231RafaleMauvaise) != 1 {
		t.Errorf("RAFALE_INVALIDATE : rafale-mauvaise attendue une fois, got %v", got)
	}
	if tw231Count(got, audio.CuePerdu) != 0 {
		t.Errorf("rafale-mauvaise REMPLACE perdu sur ce site (D4), got %v", got)
	}
}

// Fin du lot (réservoir épuisé : dernière question jugée) : rafale-fin-lot ; temps-ecoule reste la cue du chrono global.
func TestSounds231_RafaleLastQuestion_PlaysRafaleFinLot_NotTempsEcoule(t *testing.T) {
	app, fake := tw231RafaleApp(t, 1)
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionRafaleValidate, nil)
	got := tw231Settle(fake, tw231RafaleBonne, tw231RafaleFinLot)
	if tw231Count(got, tw231RafaleFinLot) != 1 {
		t.Errorf("dernière question du lot : rafale-fin-lot attendue une fois, got %v", got)
	}
	if tw231Count(got, audio.CueTempsEcoule) != 0 {
		t.Errorf("temps-ecoule reste la cue du chrono GLOBAL (pas de la fin de lot), got %v", got)
	}
	// Ordre (revue LOT-3, m1) : la bonne réponse d'abord, puis la fin de lot — « bonne » ne doit pas être couverte par le jingle de fin.
	if len(got) != 2 || got[0] != tw231RafaleBonne || got[1] != tw231RafaleFinLot {
		t.Errorf("ordre attendu [rafale-bonne, rafale-fin-lot] à la dernière bonne réponse, got %v", got)
	}
}

// Revue LOT-3, m3 — négatif : l'expiration du chrono GLOBAL d'une manche RAFALE joue temps-ecoule et JAMAIS rafale-fin-lot
// (le chrono du lot est le chrono global : contrat sound.md §2.1 amendé).
func TestSounds231_RafaleGlobalChronoExpiry_PlaysTempsEcoule_NeverRafaleFinLot(t *testing.T) {
	app, fake := tw231RafaleApp(t, 100, 1) // CurrentTime=1 s : expire au premier tick réel
	_ = app
	got := tw231Settle(fake, audio.CueTempsEcoule)
	if tw231Count(got, audio.CueTempsEcoule) != 1 {
		t.Fatalf("l'expiration du chrono global doit jouer temps-ecoule, got %v", got)
	}
	if tw231Count(got, tw231RafaleFinLot) != 0 {
		t.Errorf("l'expiration du chrono global ne joue JAMAIS rafale-fin-lot, got %v", got)
	}
}

// Review m1 : à la dernière bonne réponse, le verdict sonne AVANT la fin de lot.
func TestSounds231_RafaleLastValidate_VerdictBeforeFinLot(t *testing.T) {
	app, fake := tw231RafaleApp(t, 1)
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionRafaleValidate, nil)
	got := tw231Settle(fake, tw231RafaleFinLot)
	if len(got) < 2 || got[len(got)-2] != tw231RafaleBonne || got[len(got)-1] != tw231RafaleFinLot {
		t.Errorf("ordre attendu ... rafale-bonne puis rafale-fin-lot, got %v", got)
	}
}

func TestSounds231_Rafale_CueDisabled_Silent(t *testing.T) {
	app, fake := tw231RafaleApp(t, 100)
	tw231SetCuesDisabled(t, "rafale-bonne", "rafale-mauvaise", "rafale-fin-lot")
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionRafaleValidate, nil)
	dispatchAs(t, app, server.ClientTypeAdmin, protocol.ActionRafaleInvalidate, nil)
	got := tw231Settle(fake)
	for _, c := range []audio.Cue{tw231RafaleBonne, tw231RafaleMauvaise, tw231RafaleFinLot} {
		if tw231Count(got, c) != 0 {
			t.Errorf("%s désactivée : ne doit pas jouer, got %v", c, got)
		}
	}
}

// Les 7 cues historiques ne changent pas de site : un indice QCM ne joue aucune cue globale en plus.
func TestSounds231_QCMHint_DoesNotAlsoPlayGlobalCue(t *testing.T) {
	config.Mutate(func(c *config.Config) { c.Sound.CuesDisabled = nil })
	app, fake := tw231Wire(t)
	app.broadcastQCMHint("BLUE", 1)
	for _, c := range tw231Settle(fake, tw231QCMIndice) {
		switch c {
		case audio.CueGagne, audio.CuePerdu, audio.CueReveal, audio.CueTempsEcoule:
			t.Errorf("un indice QCM ne doit pas jouer la cue globale %s", c)
		}
	}
}
