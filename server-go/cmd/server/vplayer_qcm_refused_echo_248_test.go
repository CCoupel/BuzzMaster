package main

import (
	"testing"

	"buzzcontrol/internal/game"
)

// #248 lot 5a (réserve de QA) — réponse QCM d'un VJoueur REFUSÉE par le moteur (son équipe a déjà répondu, ou renvoi de
// sa propre réponse déjà acceptée) : le VJoueur qui a renvoyé reçoit exactement UN UPDATE ciblé (TIME de son
// équipe > 0), sans PAUSE et sans aucun message pour les autres clients. Sans cet écho la tablette resterait en
// « Envoi… » puis afficherait un faux « non reçue » (l'état « équipe déjà servie » se lit dans teams[équipe].TIME).
// Jumeau QCM de speedy_refused_echo_247_test.go (même harnais room247). Le serveur contient déjà cet écho
// (handleVPlayerQCMAnswer) : ces tests sont des GARDES de non-régression et doivent passer d'emblée.

func newQCMRoom248(t *testing.T) *room247 {
	t.Helper()
	r := newRoom247(t)
	e := r.app.engine
	e.SetPhase(game.PhaseStopped)
	e.Ready("q-qcm-248", &game.Question{
		ID: "q-qcm-248", Question: "QCM ?", Type: game.QuestionTypeQCM,
		TypedContent: game.TypedContent{QCMAnswers: &game.QCMAnswers{Red: "A", Green: "B", Yellow: "C", Blue: "D"}},
	})
	e.SetPhase(game.PhaseStarted)
	r.observe(t)
	return r
}

func (r *room247) answer(t *testing.T, who, color string) {
	t.Helper()
	r.app.handleVPlayerQCMAnswer(r.cids[who], qcmAnswerMsg(t, r.ids[who], color))
}

func TestRefusedQCMAnswer248_TeammateOfServedTeam_GetsOneTargetedUpdateAndNoPause(t *testing.T) {
	r := newQCMRoom248(t)
	r.answer(t, "p1", "RED") // accepté : l'équipe A est servie
	r.observe(t)

	r.answer(t, "p2", "GREEN") // même équipe : refusé par le moteur
	got := r.observe(t)

	upd := framesOf247(got["p2"], "UPDATE")
	if len(upd) != 1 {
		t.Fatalf("p2 doit recevoir exactement 1 UPDATE ciblé, a reçu %d (trames : %v)", len(upd), got["p2"])
	}
	if tm := teamTime247(t, upd[0], "TeamA"); tm <= 0 {
		t.Errorf("l'UPDATE ciblé doit porter le TIME > 0 de son équipe, a %v", tm)
	}
	for name, fs := range got {
		if len(framesOf247(fs, "PAUSE")) != 0 {
			t.Errorf("%s ne doit recevoir aucun PAUSE : %v", name, fs)
		}
		if name != "p2" && len(fs) != 0 {
			t.Errorf("%s ne doit recevoir AUCUN message (réponse refusée de p2) : %v", name, fs)
		}
	}
}

func TestRefusedQCMAnswer248_ResendOfOwnAcceptedAnswer_GetsOneTargetedUpdate(t *testing.T) {
	r := newQCMRoom248(t)
	r.answer(t, "p1", "RED")
	r.observe(t)

	r.answer(t, "p1", "RED") // renvoi (accusé perdu) : bumper déjà appuyé
	got := r.observe(t)

	upd := framesOf247(got["p1"], "UPDATE")
	if len(upd) != 1 {
		t.Fatalf("p1 doit recevoir exactement 1 UPDATE ciblé pour son renvoi, a reçu %d (trames : %v)", len(upd), got["p1"])
	}
	for name, fs := range got {
		if len(framesOf247(fs, "PAUSE")) != 0 {
			t.Errorf("%s ne doit recevoir aucun PAUSE (renvoi) : %v", name, fs)
		}
		if name != "p1" && len(fs) != 0 {
			t.Errorf("%s ne doit recevoir AUCUN message (renvoi de p1) : %v", name, fs)
		}
	}
}

func TestAcceptedQCMAnswer248_StillOnePauseForEveryone(t *testing.T) {
	r := newQCMRoom248(t)
	r.answer(t, "p1", "BLUE")
	got := r.observe(t)
	for _, name := range []string{"admin", "tv", "anim", "p1", "p2", "p3"} {
		if n := len(framesOf247(got[name], "PAUSE")); n != 1 {
			t.Errorf("%s doit recevoir exactement 1 PAUSE sur une réponse acceptée, a reçu %d", name, n)
		}
	}
}
