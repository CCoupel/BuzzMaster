package protocol

import (
	"encoding/json"
	"testing"
)

// ---------------------------------------------------------------------------
// Tests : traversée VPlayer des champs RAFALE_CURRENT_TEAM/_COLOR/
// _PARTICIPATING_TEAMS (milestone v8.0.0 #16, issue #199 task 37,
// contrats/rafale.md §8.2).
//
// Le contrat §8.2 signale un risque explicite : SerializeForVPlayer
// (cmd/server/main.go: buildVPlayerPayloads, sa réimplémentation "hot path")
// construit sa propre carte de champs — sans vérification, l'indicateur
// « équipe active » fonctionnerait sur /tv et /anim mais resterait vide sur
// VPlayer (panne asymétrique, difficile à diagnostiquer).
//
// Analyse (dev-backend, #199) : SerializeForWebClient/SerializeForVPlayerCommon/
// SerializeForVPlayer (ce fichier) sont tous les trois des listes
// D'EXCLUSION (AdminOnlyGameFields ∪ VPlayerOnlyGameFields, retirés du noeud
// GAME déjà complet), jamais des listes D'INCLUSION — voir leurs propres
// commentaires de code. RAFALE_CURRENT_TEAM/_CURRENT_TEAM_COLOR/
// _PARTICIPATING_TEAMS ne figurent dans AUCUNE des deux listes (contrat §4 :
// "aucun de ces champs ne rejoint AdminOnlyGameFields ni
// VPlayerOnlyGameFields — ils sont diffusés à tous les types de clients par
// défaut"). Ils traversent donc déjà les trois chemins par construction,
// sans qu'aucun code n'ait besoin d'être modifié ici — ce fichier est le
// test bloquant qui le PROUVE plutôt que de le supposer, couvrant le chemin
// de référence (ce package) ET, séparément, le hot path
// (cmd/server/rafale_vplayer_fanout_test.go).
// ---------------------------------------------------------------------------

// buildRafaleTeamFieldsMsg builds a realistic GameData-shaped UPDATE message
// carrying the 3 RAFALE team-indicator fields (contract §8.2) alongside a
// named bumper/team pair, so the reduced VPlayer path (own-bumper-entry) has
// something real to key off.
func buildRafaleTeamFieldsMsg(t *testing.T, phase string) *Message {
	t.Helper()
	payload := map[string]interface{}{
		"GAME": map[string]interface{}{
			"PHASE":                      phase,
			"TIME":                       int64(1234567890),
			"CURRENT_TIME":               12,
			"RAFALE_CURRENT_TEAM":        "TeamA",
			"RAFALE_CURRENT_TEAM_COLOR":  []interface{}{255.0, 0.0, 0.0},
			"RAFALE_PARTICIPATING_TEAMS": []interface{}{"TeamA", "TeamB"},
		},
		"bumpers": map[string]interface{}{
			vplayerTestPlayerID: map[string]interface{}{
				"NAME": "Alice",
				"TEAM": "TeamA",
			},
		},
		"teams": map[string]interface{}{
			"TeamA": map[string]interface{}{"NAME": "TeamA"},
			"TeamB": map[string]interface{}{"NAME": "TeamB"},
		},
	}
	rawMsg, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("buildRafaleTeamFieldsMsg: marshal failed: %v", err)
	}
	msg, err := NewMessage(ActionUpdate, nil)
	if err != nil {
		t.Fatalf("buildRafaleTeamFieldsMsg: NewMessage failed: %v", err)
	}
	msg.Msg = rawMsg
	return msg
}

// assertRafaleTeamFieldsPresent fails the test unless all 3 fields are
// present in the given GAME node with their expected values.
func assertRafaleTeamFieldsPresent(t *testing.T, label string, gameNode map[string]interface{}) {
	t.Helper()
	if gameNode["RAFALE_CURRENT_TEAM"] != "TeamA" {
		t.Errorf("%s: RAFALE_CURRENT_TEAM missing or wrong, got %v (node: %v)", label, gameNode["RAFALE_CURRENT_TEAM"], gameNode)
	}
	if _, ok := gameNode["RAFALE_CURRENT_TEAM_COLOR"]; !ok {
		t.Errorf("%s: RAFALE_CURRENT_TEAM_COLOR missing (node: %v)", label, gameNode)
	}
	if _, ok := gameNode["RAFALE_PARTICIPATING_TEAMS"]; !ok {
		t.Errorf("%s: RAFALE_PARTICIPATING_TEAMS missing (node: %v)", label, gameNode)
	}
}

func TestSerializeForWebClient_RafaleTeamFields_Survive(t *testing.T) {
	msg := buildRafaleTeamFieldsMsg(t, "STARTED")
	data, err := msg.SerializeForWebClient()
	if err != nil {
		t.Fatalf("SerializeForWebClient failed: %v", err)
	}
	assertRafaleTeamFieldsPresent(t, "SerializeForWebClient (TV/anim)", gameNodeOf(t, data))
}

func TestSerializeForVPlayerCommon_RafaleTeamFields_Survive(t *testing.T) {
	msg := buildRafaleTeamFieldsMsg(t, "STARTED")
	data, err := msg.SerializeForVPlayerCommon()
	if err != nil {
		t.Fatalf("SerializeForVPlayerCommon failed: %v", err)
	}
	assertRafaleTeamFieldsPresent(t, "SerializeForVPlayerCommon", gameNodeOf(t, data))
}

// TestSerializeForVPlayer_RafaleTeamFields_Survive_UnreducedPath covers a
// phase OUTSIDE PREPARE/READY (STARTED — the only phase a RAFALE round is
// actually live in), which falls back to SerializeForVPlayerCommon inside
// SerializeForVPlayer itself.
func TestSerializeForVPlayer_RafaleTeamFields_Survive_UnreducedPath(t *testing.T) {
	msg := buildRafaleTeamFieldsMsg(t, "STARTED")
	data, err := msg.SerializeForVPlayer(vplayerTestPlayerID)
	if err != nil {
		t.Fatalf("SerializeForVPlayer failed: %v", err)
	}
	assertRafaleTeamFieldsPresent(t, "SerializeForVPlayer (STARTED, unreduced)", gameNodeOf(t, data))
}

// TestSerializeForVPlayer_RafaleTeamFields_Survive_ReducedPath covers the
// PREPARE/READY per-recipient reduction path (contract vplayer-payload-filter.md
// §2) — the ONE path that rebuilds its own "raw" map independently of the
// common serializeFiltered helper (see SerializeForVPlayer's own doc
// comment). GAME itself is left untouched by the reduction (only the
// admin-only/vplayer-excluded lists are applied, and "bumpers" is reduced to
// one entry) — this proves the 3 team fields survive THIS specific code
// path too, not just the common one.
func TestSerializeForVPlayer_RafaleTeamFields_Survive_ReducedPath(t *testing.T) {
	msg := buildRafaleTeamFieldsMsg(t, "PREPARE")
	data, err := msg.SerializeForVPlayer(vplayerTestPlayerID)
	if err != nil {
		t.Fatalf("SerializeForVPlayer failed: %v", err)
	}
	assertRafaleTeamFieldsPresent(t, "SerializeForVPlayer (PREPARE, reduced)", gameNodeOf(t, data))
}
