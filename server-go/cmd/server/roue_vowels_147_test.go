package main

import (
	"strings"
	"testing"
	"time"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
)

// #147 (v12.1.0, LOT-1.2) — achat de voyelles par le VRAI chemin WebSocket (contracts/roue.md §8.2, §15.1.6) :
// droits par émetteur, ROUE_ERROR à l'émetteur seul, diffusion, non-fuite de la phrase.
// Phrase de la harnais : « BONJOUR LE MONDE » (voyelles O, U, E présentes ; A, I, Y absentes). Prix par défaut 250.

type buyPayload147 struct {
	Letter string `json:"LETTER"`
}

// wireWithBank147 joue un tour (N : 2 cases × 1000) puis « Tour suivant » : la cagnotte de red vaut 2000, TURN.
func wireWithBank147(t *testing.T) *roueWire {
	t.Helper()
	w := newRoueWire(t, game.RoueModeTantQueJeGagne)
	w.app.engine.SetRoueSpinUpMs(30)
	w.send(w.admin, protocol.ActionRoueSpin, struct{}{})
	w.waitFullSpeed(5 * time.Second)
	w.send(w.admin, protocol.ActionRoueStop, struct{}{})
	w.waitSub(game.RoueSubLetter, 5*time.Second)
	w.send(w.admin, protocol.ActionRoueLetter, protocol.RoueLetterPayload{Letter: "N"})
	w.send(w.admin, protocol.ActionRoueFlip, struct{}{})
	w.waitSub(game.RoueSubAnswer, 5*time.Second)
	w.send(w.admin, protocol.ActionRoueNext, protocol.RoueNextPayload{Hand: game.RoueHandSame})
	if s := w.app.engine.GetState(); s.RoueSubPhase != game.RoueSubTurn || s.RoueTeamBank["red"] != 2000 {
		t.Fatalf("préparation : %s, cagnotte red %d (attendu TURN / 2000)", s.RoueSubPhase, s.RoueTeamBank["red"])
	}
	return w
}

// L'animateur achète une voyelle présente de bout en bout : débit, éclairage, retournement, retour en TURN ; la TV et la
// tablette reçoivent les mises à jour et ne voient jamais la phrase ; les écrans ne peuvent pas acheter.
func TestRoueWire147_Anim_BuysPresentVowelEndToEnd(t *testing.T) {
	w := wireWithBank147(t)
	w.adminTap.take(30 * time.Millisecond)
	w.tvTap.take(0)
	w.vpTap.take(0)

	w.send(w.anim, protocol.ActionRoueBuyVowel, buyPayload147{Letter: "O"})
	s := w.app.engine.GetState()
	if s.RoueSubPhase != "VOWEL_LIT" || s.RoueTeamBank["red"] != 1750 || s.RoueLastEvent.Kind != "VOWEL_HIT" {
		t.Fatalf("après l'achat : %s / cagnotte %d / %s", s.RoueSubPhase, s.RoueTeamBank["red"], s.RoueLastEvent.Kind)
	}
	w.send(w.admin, protocol.ActionRoueFlip, struct{}{})
	w.waitSub(game.RoueSubTurn, 5*time.Second)
	s = w.app.engine.GetState()
	if s.RoueCurrentTeam != "red" || s.RoueStep != 1 {
		t.Fatalf("de retour en TURN, même équipe : %s étape %d", s.RoueCurrentTeam, s.RoueStep)
	}
	if got := w.app.engine.GetTeam("red").Score; got != 0 {
		t.Errorf("aucun score de partie modifié avant la résolution : %d", got)
	}
	for name, tp := range map[string]*tap{"tv": w.tvTap, "tablette": w.vpTap} {
		frames := tp.take(100 * time.Millisecond)
		if len(frames) == 0 {
			t.Errorf("%s : aucune mise à jour reçue pendant l'achat", name)
		}
		for _, f := range frames {
			if strings.Contains(f, "BONJOUR LE MONDE") || strings.Contains(f, "ROUE_PHRASE") {
				t.Fatalf("%s : la phrase fuit sur le fil : %.120s", name, f)
			}
		}
	}
}

// Refus : ROUE_ERROR à l'émetteur seul (INSUFFICIENT_BANK : cagnotte 0 au départ), l'état ne bouge pas.
func TestRoueWire147_RefusalAnswersROUE_ERROR_ToTheEmitterOnly(t *testing.T) {
	w := newRoueWire(t, game.RoueModeTantQueJeGagne)
	w.adminTap.take(30 * time.Millisecond)
	w.animTap.take(0)
	w.tvTap.take(0)
	w.send(w.admin, protocol.ActionRoueBuyVowel, buyPayload147{Letter: "O"})
	if code, ok := roueErrorOf(w.adminTap.take(200 * time.Millisecond)); !ok || code != "INSUFFICIENT_BANK" {
		t.Errorf("l'admin doit recevoir ROUE_ERROR INSUFFICIENT_BANK, got %q found=%v", code, ok)
	}
	if _, ok := roueErrorOf(w.animTap.take(50 * time.Millisecond)); ok {
		t.Errorf("l'animateur ne doit pas recevoir le ROUE_ERROR de l'admin")
	}
	if _, ok := roueErrorOf(w.tvTap.take(50 * time.Millisecond)); ok {
		t.Errorf("la TV ne doit jamais recevoir de ROUE_ERROR")
	}
	if w.sub() != game.RoueSubTurn {
		t.Errorf("un refus ne change pas l'état : %q", w.sub())
	}
	w.send(w.admin, protocol.ActionRoueBuyVowel, buyPayload147{Letter: "N"})
	if code, ok := roueErrorOf(w.adminTap.take(200 * time.Millisecond)); !ok || code == "" {
		t.Errorf("une consonne doit être refusée par un ROUE_ERROR, got %q found=%v", code, ok)
	}
}

// TV et tablette ne peuvent pas acheter : aucun effet, aucune erreur.
func TestRoueWire147_TVAndTablet_CannotBuyVowel(t *testing.T) {
	w := wireWithBank147(t)
	w.send(w.tv, protocol.ActionRoueBuyVowel, buyPayload147{Letter: "O"})
	w.send(w.vp, protocol.ActionRoueBuyVowel, buyPayload147{Letter: "O"})
	time.Sleep(100 * time.Millisecond)
	s := w.app.engine.GetState()
	if s.RoueSubPhase != game.RoueSubTurn || s.RoueTeamBank["red"] != 2000 {
		t.Errorf("aucun achat d'une TV / tablette ne doit s'exécuter : %s / %d", s.RoueSubPhase, s.RoueTeamBank["red"])
	}
}

// Double commande : le même achat par l'admin puis l'animateur — un seul passe (un seul débit), l'autre reçoit un refus.
func TestRoueWire147_DoubleCommand_OneDebitOnly(t *testing.T) {
	w := wireWithBank147(t)
	w.send(w.admin, protocol.ActionRoueBuyVowel, buyPayload147{Letter: "O"})
	w.send(w.anim, protocol.ActionRoueBuyVowel, buyPayload147{Letter: "O"})
	if got := w.app.engine.GetState().RoueTeamBank["red"]; got != 1750 {
		t.Fatalf("cagnotte = %d, attendu 1750 (un seul débit)", got)
	}
}
