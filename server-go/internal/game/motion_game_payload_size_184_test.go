// Tests for #184 — mesure du nœud "GAME" (Risque R1 du plan
// `_work/reports/plan-memotion-v700-20260821.md`), assignée à B-T1
// explicitement (dev-backend l'a signalée hors périmètre B-B* dans son
// handoff, `_work/handoff/dev-backend-20260821-163800.md`).
//
// Réduction du problème "comparer à une référence v6.5.2" : un diff des
// champs de GameState entre v6.5.2 (`git show d2d8479:server-go/internal/
// game/models.go`, le tag de release juste avant ce milestone) et l'état
// courant montre EXACTEMENT un champ nouveau et toujours sérialisé —
// `MotionActive MotionActive `json:"MEMOTION_ACTIVE"``, inséré entre
// MotionCurrentTeamColor et VirtualPlayerCount. MotionSubPhase/
// MotionCardStates ont changé de TYPE Go (string → MotionSubPhase/
// MotionCardState) mais pas de FORME JSON (mêmes chaînes) ; l'embarquement
// TypedContent est prouvé octet-identique pour les données existantes par
// models_roundtrip_test.go (B-B1). Donc "comparer au v6.5.2" se réduit
// exactement à : que coûte MEMOTION_ACTIVE sur le fil — pas besoin de
// reconstruire un second binaire pour l'isoler.
//
// Run: go test ./internal/game/... -run 184_GamePayload -v
package game

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func loadFixtureQuestion184GamePayload(t *testing.T, id string) *Question {
	t.Helper()
	path := filepath.Join("..", "..", "data", "files", "questions", id, "question.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("impossible de lire la fixture %s : %v", path, err)
	}
	var q Question
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatalf("impossible de décoder la fixture %s : %v", path, err)
	}
	return &q
}

// Test184_GamePayload_MotionActiveCostConstantAcrossCardCount exerce le vrai
// chemin moteur (SelectMotionCard) sur la plus petite (4 cartes, fixture 14)
// et la plus grande (20 cartes, fixture 85) question MEMOTION réelle du
// dépôt, et exige un MEMOTION_ACTIVE de taille IDENTIQUE dans les deux cas —
// c'est la preuve exécutable de la revendication de conception de R1 : « un
// emplacement unique, pas une carte indexée par ID ». Une régression future
// qui ferait dépendre MEMOTION_ACTIVE du nombre de cartes (ex: le transformer
// en map indexée) ferait échouer ce test immédiatement.
func Test184_GamePayload_MotionActiveCostConstantAcrossCardCount(t *testing.T) {
	sizeFor := func(fixtureID string) int {
		q := loadFixtureQuestion184GamePayload(t, fixtureID)
		e := NewEngine()
		startMEMOTION(t, e, q.ID, q)
		cardID := q.MotionCards[0].ID
		if err := e.SelectMotionCard(cardID); err != nil {
			t.Fatalf("fixture %s : SelectMotionCard(%s) : %v", fixtureID, cardID, err)
		}
		defer e.Stop()

		raw, err := json.Marshal(e.state.MotionActive)
		if err != nil {
			t.Fatalf("fixture %s : marshal MotionActive : %v", fixtureID, err)
		}
		return len(raw)
	}

	size4 := sizeFor("14")  // 4 cartes
	size20 := sizeFor("85") // 20 cartes

	if size4 != size20 {
		t.Errorf("MEMOTION_ACTIVE dépend du nombre de cartes de la question (4 cartes → %d octets, "+
			"20 cartes → %d octets) — R1 exige un emplacement unique, pas un coût qui grandit avec N",
			size4, size20)
	}
}

// Test184_GamePayload_MotionActiveAbsoluteBound borne le coût absolu de
// MEMOTION_ACTIVE, y compris avec un STATE réaliste non vide (indices QCM
// invalidés) — R1 : « coût d'un seul état de type ».
func Test184_GamePayload_MotionActiveAbsoluteBound(t *testing.T) {
	active := MotionActive{
		CardID: "mc-3",
		Type:   QuestionTypeQCM,
		State:  map[string]interface{}{"QCM_INVALIDATED": []string{"RED", "YELLOW"}},
	}
	raw, err := json.Marshal(active)
	if err != nil {
		t.Fatalf("marshal MotionActive : %v", err)
	}
	const bound = 200 // mesuré ~90 octets pour ce contenu ; marge large pour un STATE réaliste
	if len(raw) > bound {
		t.Errorf("MEMOTION_ACTIVE = %d octets (%s), attendu < %d — coût d'un seul état de type censé rester petit",
			len(raw), raw, bound)
	}
}

// Test184_GamePayload_GAMENode_MEMOTIONRoundInProgress_Bound mesure la taille
// totale du nœud "GAME" (json.Marshal(GameState), la valeur sérialisée sous
// la clé "GAME" de GameData — internal/game/models.go, GetGameJSON) pour une
// manche MEMOTION réaliste EN COURS : la plus grosse question MEMOTION réelle
// du dépôt (20 cartes, fixture 85), en sous-phase QUESTION (chrono actif,
// MEMOTION_ACTIVE peuplé), avec un mélange d'états de cartes (certaines DONE
// avec gagnant, une en QUESTION, le reste UNPLAYED) — le pire cas observable
// en v7.0.0 pour ce nœud, contrairement aux tests d'engine ci-dessus qui
// n'isolent que MEMOTION_ACTIVE.
//
// Borne : `contracts/vplayer-payload-filter.md` §4.6 documente une mesure de
// référence "~11 Ko" pour ce nœud en MEMOTION (avant #184) — mais en
// conditions réelles de jeu (quiz nommé, thème, notes, arrière-plans, équipes
// avec scores…), tous des champs de GameState sans rapport avec #184 et
// laissés à leur valeur zéro ici pour isoler spécifiquement le coût
// attribuable à MEMOTION. Mesuré ici : ~4,5 Ko pour le pire cas réel (20
// cartes, 5 DONE + 1 active). La borne (8 Ko, ~76% de marge au-dessus du
// mesuré) valide la revendication de R1 sans chasser la valeur ~11 Ko
// documentée, qui inclut des champs hors périmètre — tout en détectant
// loudly une régression qui ferait exploser la charge utile (ex: un futur
// champ indexé par carte).
func Test184_GamePayload_GAMENode_MEMOTIONRoundInProgress_Bound(t *testing.T) {
	q := loadFixtureQuestion184GamePayload(t, "85") // 20 cartes — le pire cas réel
	e := NewEngine()
	startMEMOTION(t, e, q.ID, q)

	// Mélange d'états réaliste : les 5 premières cartes DONE (avec gagnant
	// alterné), la 6e SELECTED puis flippée en QUESTION (active), le reste
	// UNPLAYED (comportement par défaut d'InitMotionState — rien à faire).
	winners := []string{"Les Bleus", "Les Rouges"}
	for i := 0; i < 5; i++ {
		cardID := q.MotionCards[i].ID
		if err := e.SelectMotionCard(cardID); err != nil {
			t.Fatalf("SelectMotionCard(%s) : %v", cardID, err)
		}
		// DoneMotionCard depuis SELECTED annule (retour UNPLAYED) — il faut
		// être en QUESTION ou REVEAL pour que la carte passe réellement DONE.
		if err := e.FlipMotionCard(); err != nil {
			t.Fatalf("FlipMotionCard(%s) : %v", cardID, err)
		}
		if _, _, err := e.DoneMotionCard(cardID, winners[i%2], 1); err != nil {
			t.Fatalf("DoneMotionCard(%s) : %v", cardID, err)
		}
	}
	activeCardID := q.MotionCards[5].ID
	if err := e.SelectMotionCard(activeCardID); err != nil {
		t.Fatalf("SelectMotionCard(%s) : %v", activeCardID, err)
	}
	if err := e.FlipMotionCard(); err != nil {
		t.Fatalf("FlipMotionCard() : %v", err)
	}
	defer e.Stop()

	raw, err := json.Marshal(&e.state)
	if err != nil {
		t.Fatalf("marshal GameState (nœud GAME) : %v", err)
	}

	const boundBytes = 8 * 1024 // 8 Ko — voir justification dans le commentaire de la fonction
	if len(raw) > boundBytes {
		t.Errorf("nœud GAME = %d octets pour une manche MEMOTION de 20 cartes en cours, "+
			"attendu < %d octets (mesuré ~4,5 Ko au moment d'écrire ce test ; contracts/"+
			"vplayer-payload-filter.md §4.6 documente ~11 Ko en conditions réelles, champs hors "+
			"périmètre MEMOTION inclus)", len(raw), boundBytes)
	}
	t.Logf("nœud GAME mesuré : %d octets (borne %d)", len(raw), boundBytes)
}
