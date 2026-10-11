package main

// #254 (v11.2) — garde serveur « SET_CREDIT_POINTS ≤ 0 ignoré » : un montant nul ou négatif (pénalité du champ « Points »)
// n'est jamais rediffusé aux tablettes de l'anim (leur bouton de crédit retirerait des points au lieu d'en donner) ; elles
// gardent la dernière valeur positive. Contrat : websocket-actions.md §« Montant négatif et tablettes de l'anim ».
// Réserve QA #254 (_work/reports/qa-20261011-123300.md).

import (
	"encoding/json"
	"testing"
	"time"
)

// creditValues254 : valeurs POINTS des CREDIT_POINTS reçus par la tablette anim depuis le dernier appel.
func creditValues254(tap *tap253) []int {
	var out []int
	for _, f := range tap.take() {
		a, m := envelope253(f)
		if a != "CREDIT_POINTS" {
			continue
		}
		var p struct {
			Points int `json:"POINTS"`
		}
		_ = json.Unmarshal(m, &p)
		out = append(out, p.Points)
	}
	return out
}

func TestSetCreditPoints254_Critical_NonPositiveAmountIsIgnoredAndTheAnimKeepsTheLastPositiveValue(t *testing.T) {
	e := newEnv244(t)
	startTap253(e.animConn)
	tap := tapOf253(e.animConn)
	setCredit := func(v int) {
		e.send(t, e.admin, "admin", "SET_CREDIT_POINTS", map[string]interface{}{"POINTS": v})
		time.Sleep(150 * time.Millisecond)
	}

	setCredit(5)
	if got := creditValues254(tap); len(got) == 0 || got[len(got)-1] != 5 {
		t.Fatalf("une valeur positive est rediffusée à l'anim : %v", got)
	}
	if e.app.currentCreditPoints != 5 {
		t.Fatalf("valeur courante conservée côté serveur : %d", e.app.currentCreditPoints)
	}

	for _, bad := range []int{0, -1, -3, -100} {
		setCredit(bad)
		if got := creditValues254(tap); len(got) != 0 {
			t.Errorf("SET_CREDIT_POINTS %d : aucune diffusion à l'anim (reçu %v)", bad, got)
		}
		if e.app.currentCreditPoints != 5 {
			t.Errorf("SET_CREDIT_POINTS %d ignoré : la dernière valeur positive (5) est conservée, obtenu %d", bad, e.app.currentCreditPoints)
		}
	}

	// un nouvel anim qui se connecte pendant ce temps ne voit jamais une valeur ≤ 0
	conn, id := e.dial(t, "/ws/anim")
	startTap253(conn)
	e.send(t, id, "anim", "HELLO", map[string]interface{}{})
	time.Sleep(200 * time.Millisecond)
	for _, v := range creditValues254(tapOf253(conn)) {
		if v <= 0 {
			t.Errorf("un anim qui rejoint ne reçoit jamais un crédit ≤ 0 : %d", v)
		}
	}

	setCredit(2)
	if got := creditValues254(tap); len(got) == 0 || got[len(got)-1] != 2 || e.app.currentCreditPoints != 2 {
		t.Errorf("le retour à une valeur positive est de nouveau rediffusé : %v / %d", got, e.app.currentCreditPoints)
	}
}
