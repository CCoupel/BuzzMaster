package game

// #246 (v11.2) — identité des VJoueurs au niveau du moteur : le NAME envoyé avec un ID
// résolu ne renomme jamais le bumper (cas 1), les noms restent uniques (sans casse,
// espaces retirés). Contrat : contracts/websocket-actions.md §PLAYER_CONNECT.
// Reproduction harness246_identity_server (statut `regression`).

import (
	"encoding/json"
	"strings"
	"testing"
)

func newEnrollEngine246(t *testing.T) *Engine {
	t.Helper()
	e := NewEngine()
	e.SetPhase(PhaseEnroll)
	e.SetVirtualPlayerLimit(50)
	return e
}

func TestIdentity246_Case1NeverRenamesTheBumper(t *testing.T) {
	e := newEnrollEngine246(t)
	paul, _, _, err := e.ReconnectOrCreateVirtualPlayer("", "Paul")
	if err != nil {
		t.Fatal(err)
	}
	mamie, _, _, err := e.ReconnectOrCreateVirtualPlayer("", "Mamie")
	if err != nil {
		t.Fatal(err)
	}

	// onglet périmé : PLAYER_CONNECT {ID: Paul, NAME: "Mamie"}
	id, b, reconnected, err := e.ReconnectOrCreateVirtualPlayer(paul, "Mamie")

	if err != nil || !reconnected || id != paul {
		t.Fatalf("cas 1 = reconnexion du bumper de Paul : id=%s reconnected=%v err=%v", id, reconnected, err)
	}
	if b.Name != "Paul" || e.GetBumper(paul).Name != "Paul" {
		t.Errorf("le nom serveur fait foi, jamais celui du client : %q", b.Name)
	}
	if e.GetBumper(mamie).Name != "Mamie" {
		t.Errorf("le bumper de Mamie est intact : %q", e.GetBumper(mamie).Name)
	}
}

func TestIdentity246_NamesStayUniqueWhateverTheCaseAndSpacing(t *testing.T) {
	e := newEnrollEngine246(t)
	if _, _, _, err := e.ReconnectOrCreateVirtualPlayer("", "Paul"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Paul", "paul", "PAUL", "  Paul  ", " pAuL"} {
		_, _, _, err := e.ReconnectOrCreateVirtualPlayer("", name)
		ee, ok := err.(*EnrollmentError)
		if !ok || !strings.HasPrefix(ee.Reason, "NAME_TAKEN") {
			t.Errorf("%q : NAME_TAKEN* attendu, obtenu %v", name, err)
		}
	}
	var bd struct {
		Bumpers map[string]struct {
			Name string `json:"NAME"`
		} `json:"bumpers"`
	}
	_ = json.Unmarshal(e.GetGameJSON(), &bd)
	n := len(bd.Bumpers)
	if n != 1 {
		t.Errorf("un seul VJoueur doit subsister : %d", n)
	}
}

func TestIdentity246_StaleIDDoesNotBypassNameUniqueness(t *testing.T) {
	e := newEnrollEngine246(t)
	if _, _, _, err := e.ReconnectOrCreateVirtualPlayer("", "Mamie"); err != nil {
		t.Fatal(err)
	}

	_, _, _, err := e.ReconnectOrCreateVirtualPlayer("vjoueur_Mamie_ancienne_partie", "mamie")

	if err == nil {
		t.Fatal("ID inconnu + nom déjà pris : rejet, jamais de fusion ni de doublon")
	}
}
