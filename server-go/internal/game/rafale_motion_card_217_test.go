package game

// Tests for #217 — RAFALE nestable dans une carte MEMOTION (milestone
// v9.0.0, Batch 4). Written contract-first from the plan
// (_work/reports/plan-v900-consolide-20260904-150000.md §4/§6 Lot 4) in
// parallel with dev-backend's implementation.
//
// C'est le lot le plus lourd techniquement du milestone (dev-backend's own
// handoff) : porter l'état RAFALE (13 champs GameState globaux aujourd'hui)
// vers MotionActive.State scopé par CARD_ID, jamais les champs globaux
// (qui restent réservés aux manches RAFALE classiques d'une Question de
// TYPE RAFALE, contract §2.1). Étant donné l'incertitude sur le point
// d'entrée exact du tirage/de la validation pour une carte (les fonctions
// existantes RafaleValidate/RafaleInvalidate/DrawRafaleQuestion pourraient
// être réutilisées telles quelles avec un contexte MotionActive implicite,
// ou gagner un paramètre cardID explicite comme FlipMotionMemoryCard(cardID,
// ...) l'a fait pour #187) — chaque test ci-dessous documente explicitement
// l'hypothèse qu'il retient, pour faciliter l'ajustement mécanique une fois
// l'implémentation réelle disponible (même discipline que pour #216/#214).
//
// Ce qui est certain (repris tel quel du registre, contrat rafale.md et des
// handoffs dev-backend/dev-frontend, déjà confirmés en lisant
// question_types.go et questionTypeMeta.js au moment de l'écriture) :
//   - QuestionTypeRafale.NestableInMotionCard devient true.
//   - QuestionTypeRafale.DefaultPointsRule devient PointsRuleModeStarsProrata.
//   - Mode SOLO forcé pour une carte RAFALE (217-Q3).
//   - Le réservoir (e.rafaleQuestions/e.rafaleUsed) est partagé, jamais
//     dupliqué — une carte et une manche classique de la même partie
//     marquent "déjà utilisée" au même endroit.
//   - Les champs GameState globaux RAFALE_* restent réservés aux manches
//     classiques — une carte ne doit jamais les toucher.

import (
	"testing"
)

// rafaleMotionCard builds a single MEMOTION card of TYPE=RAFALE — mirrors
// memoryMotionCard's own doc comment/shape (engine_memory_card_187_test.go).
func rafaleMotionCard(cardID string, categories []string, difficulties []int, questionTime, maxQuestions int) MotionCard {
	return MotionCard{
		ID:         cardID,
		Type:       QuestionTypeRafale,
		RectoTheme: "Rafale éclair",
		TypedContent: TypedContent{
			RafaleCategories:   categories,
			RafaleDifficulties: difficulties,
			RafaleMode:         string(RafaleModeSolo), // 217-Q3 — SOLO forcé, jamais un autre mode pour une carte
			RafaleQuestionTime: questionTime,
			RafaleMaxQuestions: maxQuestions,
		},
	}
}

// ---------------------------------------------------------------------------
// Registre : RAFALE devient nestable, barème STARS_PRORATA (comme MEMORY).
// ---------------------------------------------------------------------------

func TestQuestionTypeRafale_NestableWithStarsProrataDefault(t *testing.T) {
	def, ok := questionTypeRegistry[QuestionTypeRafale]
	if !ok {
		t.Fatal("QuestionTypeRafale missing from questionTypeRegistry")
	}
	if !def.NestableInMotionCard {
		t.Error("QuestionTypeRafale.NestableInMotionCard = false, want true (#217)")
	}
	if def.DefaultPointsRule != PointsRuleModeStarsProrata {
		t.Errorf("QuestionTypeRafale.DefaultPointsRule = %q, want %q (217-Q4, même barème que MEMORY)", def.DefaultPointsRule, PointsRuleModeStarsProrata)
	}
}

// ---------------------------------------------------------------------------
// Scoping : une carte RAFALE ne doit JAMAIS toucher les champs GameState
// globaux réservés aux manches RAFALE classiques.
// ---------------------------------------------------------------------------

func TestRafaleMotionCard_NeverLeaksIntoGlobalGameStateFields(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}})
	seedRafaleReservoirCouple(t, e, "h1", 10, CategoryHistory, 1)

	card := rafaleMotionCard("mc-rafale", []string{string(CategoryHistory)}, []int{1}, 3, 5)
	q := makeMotionQuestion("mq-rafale", []MotionCard{card}, "SOLO")
	startMEMOTION(t, e, "mq-rafale", q)
	defer e.Stop()

	if err := e.SelectMotionCard(card.ID); err != nil {
		t.Fatalf("SelectMotionCard(%s): %v", card.ID, err)
	}

	state := e.GetState()
	if state.MotionActive.CardID != card.ID {
		t.Fatalf("MotionActive.CardID = %q, want %q — the card must be the active one before asserting isolation", state.MotionActive.CardID, card.ID)
	}
	if state.MotionActive.Type != QuestionTypeRafale {
		t.Errorf("MotionActive.Type = %q, want %q", state.MotionActive.Type, QuestionTypeRafale)
	}

	// Les 13 champs globaux RAFALE_* (contrat §2.1) doivent rester à leur
	// valeur de repos — une carte ne les initialise ni ne les modifie
	// jamais, ils restent réservés à une Question de TYPE RAFALE classique.
	if state.RafaleAskedCount != 0 {
		t.Errorf("GameState.RAFALE_ASKED_COUNT = %d, want 0 — a RAFALE card must never touch the global round counters", state.RafaleAskedCount)
	}
	if state.RafaleCurrentQuestion != (RafaleCurrent{}) {
		t.Errorf("GameState.RAFALE_CURRENT_QUESTION = %+v, want zero value — the card's own live question must live in MotionActive.State (CARD_ID-scoped), never here", state.RafaleCurrentQuestion)
	}
	if len(state.RafaleTeamCounters) != 0 {
		t.Errorf("GameState.RAFALE_TEAM_COUNTERS = %v, want empty — a card's own counters must be scoped by CARD_ID inside MotionActive.State, never in this global map", state.RafaleTeamCounters)
	}
}

// ---------------------------------------------------------------------------
// Réservoir partagé : une question marquée "déjà utilisée" par une manche
// RAFALE CLASSIQUE ne doit plus jamais être tirable, y compris par une carte
// RAFALE de la même partie — même pool, même marquage, jamais dupliqué.
// ---------------------------------------------------------------------------

func TestRafaleReservoir_SharedBetweenClassicRoundAndMotionCard(t *testing.T) {
	e := NewEngine()
	// Un seul couple, une seule question — épuisé après le premier tirage,
	// qu'il vienne d'une manche classique ou d'une carte.
	seedRafaleReservoirCouple(t, e, "shared", 1, CategoryHistory, 1)

	// Épuise le pool via le chemin PUBLIC déjà stable (DrawRafaleQuestion,
	// contrat §7.1) — simule "une manche classique de la même partie a déjà
	// tiré la seule question disponible".
	if _, err := e.DrawRafaleQuestion([]string{string(CategoryHistory)}, []int{1}); err != nil {
		t.Fatalf("setup draw failed: %v", err)
	}

	// Le pool doit être vide pour QUICONQUE retente le même filtre — qu'il
	// s'agisse d'une manche classique (revérifié ici) ou, structurellement,
	// d'une carte RAFALE de la même partie (aucun second magasin de
	// "used" n'existe à créer pour #217 — contrat §7.6/handoff dev-backend
	// tâche 6, "même marquage déjà utilisée").
	available, _, _ := e.CountRafalePool([]string{string(CategoryHistory)}, []int{1})
	if available != 0 {
		t.Errorf("CountRafalePool after exhausting the sole question = %d available, want 0 — the reservoir must be a SINGLE shared store, never duplicated per card", available)
	}
}

// ---------------------------------------------------------------------------
// Non-régression IA : RAFALE reste hors GENERABLE_TYPES / distribution IA
// (décision du 2026-08-28, non rouverte par #217).
// ---------------------------------------------------------------------------

func TestRafaleNestable_StillExcludedFromAIGeneration(t *testing.T) {
	// Le registre Go n'a pas de notion de "générable" séparée du côté game —
	// ce test verrouille la seule assertion valable ici : la nestabilité de
	// #217 n'implique PAS que RAFALE gagne un HasPlayerInput ou un
	// comportement de génération. La non-régression IA elle-même (registre
	// generableQuestionTypes, internal/server/ai_generator.go, déjà
	// vérifiée exclure RAFALE avant #217 et non touchée par ce lot d'après
	// le handoff dev-backend) est couverte côté package server — voir
	// rafale_motion_ai_exclusion_217_test.go.
	def := questionTypeRegistry[QuestionTypeRafale]
	if def.HasPlayerInput {
		t.Error("QuestionTypeRafale.HasPlayerInput = true — RAFALE stays judged by admin/anim only, even nested (contract §8.1), unaffected by #217's nestability change")
	}
}
