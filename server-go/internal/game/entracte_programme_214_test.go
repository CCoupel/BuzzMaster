package game

// Tests for #214 — entracte programmée (milestone v9.0.0, Batch 3, Lot 3).
// Written contract-first from the plan
// (_work/reports/plan-v900-consolide-20260904-150000.md §4/§6 Lot 3) and the
// maquette (docs/mockups/entracte-programme-214.html §01/§02/§03/§04) in
// parallel with dev-backend's implementation.
//
// Nature de la feature (maquette §01) : PAS un nouveau mécanisme — un second
// DÉCLENCHEUR du mécanisme ENTRACTE existant (#119). Deux déclencheurs, une
// seule diffusion :
//   - Bouton ENTRACTE navbar (existant, inchangé) → configuration globale
//     (ENTRACTE_CONFIG_SAVED).
//   - Entrée ENTRACTE du déroulé, cycle START normal (nouveau) →
//     configuration PORTÉE PAR CETTE ENTRÉE, le temps de son passage — à la
//     sortie, la configuration globale reprend la main (règle de
//     restauration, décision utilisateur 214-Q9).
//
// Naming: TypedContent gagne EntracteConfig *EntracteConfig (json
// ENTRACTE_CONFIG,omitempty) — réutilise le type EntracteConfig existant
// (GameState.EntracteConfig/.EntracteConfigSaved), précédent exact :
// TypedContent.MemoryConfig *MemoryConfig (json MEMORY_CONFIG,omitempty).
// QuestionTypeEntracte QuestionType = "ENTRACTE", même convention que
// QuestionTypeRafale/QuestionTypeArdoise.
//
// Chaque test pilote UNIQUEMENT l'API publique existante (Ready,
// StartImmediate, SetEntracte, SetEntracteConfig, GetState) — le geste de
// sortie EST SetEntracte(false)/ENTRACTE_SET, confirmé identique à
// l'entracte manuel par la maquette §03 ("Sortie — Même geste que pour
// l'entracte manuel") — donc reste valide quel que soit le mécanisme
// interne exact déclenchant Entracte=true depuis le cycle START.

import (
	"testing"
)

func makeEntracteQuestion(id string, cfg EntracteConfig) *Question {
	return &Question{
		ID:       id,
		Question: "Entracte programmée " + id,
		Type:     QuestionTypeEntracte,
		TypedContent: TypedContent{
			EntracteConfig: &cfg,
		},
	}
}

// ---------------------------------------------------------------------------
// Unitaire : source de configuration — occurrence pendant la pause, jamais
// la globale (maquette §01, tableau "DÉCLENCHEUR/CONFIGURATION/EFFET").
// ---------------------------------------------------------------------------

func TestEntracteProgrammed_UsesOccurrenceConfig_NotGlobalSaved(t *testing.T) {
	e := NewEngine()

	globalCfg := EntracteConfig{Title: "ENTRACTE", Subtitle: "Retour dans 20mn", PanelSize: 65, AnimPeriod: 10, AnimIntensity: 20, TransitionMs: 2000}
	e.SetEntracteConfig(globalCfg)

	occurrenceCfg := EntracteConfig{Title: "PAUSE DÉJEUNER", Subtitle: "Retour à 14h00", PanelSize: 70, AnimPeriod: 8, AnimIntensity: 30, TransitionMs: 1500}
	q := makeEntracteQuestion("e1", occurrenceCfg)
	e.Ready(q.ID, q)
	e.StartImmediate(0)
	defer e.Stop()

	state := e.GetState()
	if !state.Entracte {
		t.Fatal("Entracte flag not raised after starting an ENTRACTE question — the panel must appear (maquette §03 START)")
	}
	if state.EntracteConfig != occurrenceCfg {
		t.Errorf("EntracteConfig = %+v, want the OCCURRENCE's own config %+v — must never be the global ENTRACTE_CONFIG_SAVED during a programmed pause", state.EntracteConfig, occurrenceCfg)
	}
	if state.EntracteConfigSaved != globalCfg {
		t.Errorf("EntracteConfigSaved = %+v, want unchanged global config %+v — a programmed pause must never overwrite the global saved config", state.EntracteConfigSaved, globalCfg)
	}
}

// ---------------------------------------------------------------------------
// Unitaire : restauration à la sortie — LE test explicitement demandé par le
// handoff (règle de restauration, décision 214-Q9, maquette §01).
// ---------------------------------------------------------------------------

func TestEntracteProgrammed_RestoresGlobalConfigOnExit(t *testing.T) {
	e := NewEngine()

	globalCfg := EntracteConfig{Title: "ENTRACTE", Subtitle: "Retour dans 20mn", PanelSize: 65, AnimPeriod: 10, AnimIntensity: 20, TransitionMs: 2000}
	e.SetEntracteConfig(globalCfg)

	occurrenceCfg := EntracteConfig{Title: "PAUSE DÉJEUNER", Subtitle: "Retour à 14h00", PanelSize: 70, AnimPeriod: 8, AnimIntensity: 30, TransitionMs: 1500}
	q := makeEntracteQuestion("e1", occurrenceCfg)
	e.Ready(q.ID, q)
	e.StartImmediate(0)
	defer e.Stop()

	if state := e.GetState(); state.EntracteConfig != occurrenceCfg {
		t.Fatalf("setup failed: EntracteConfig = %+v, want occurrence config %+v before testing exit", state.EntracteConfig, occurrenceCfg)
	}

	// Geste de sortie — même geste que l'entracte manuel (maquette §03).
	if ok := e.SetEntracte(false); !ok {
		t.Fatal("SetEntracte(false) refused — deactivation must ALWAYS succeed, from any phase (contract: jamais un entracte sans issue)")
	}

	state := e.GetState()
	if state.Entracte {
		t.Error("Entracte still true after the exit gesture")
	}
	if state.EntracteConfig != globalCfg {
		t.Errorf("EntracteConfig after exit = %+v, want the GLOBAL config %+v restored — sans cette règle, le bouton manuel afficherait le texte de la dernière pause programmée (maquette §01, \"la règle de restauration\")", state.EntracteConfig, globalCfg)
	}
}

// ---------------------------------------------------------------------------
// Intégration : cycle complet PREPARE→READY→START→sortie, avec le bouton
// manuel testé APRÈS pour prouver l'étanchéité des deux modes (maquette §01
// "Les deux modes restent étanches").
// ---------------------------------------------------------------------------

func TestEntracteProgrammed_FullCycle_ThenManualEntracteUsesGlobalAgain(t *testing.T) {
	e := NewEngine()

	globalCfg := EntracteConfig{Title: "ENTRACTE", Subtitle: "Retour dans 20mn", PanelSize: 65, AnimPeriod: 10, AnimIntensity: 20, TransitionMs: 2000}
	e.SetEntracteConfig(globalCfg)

	occurrenceCfg := EntracteConfig{Title: "PAUSE DÉJEUNER", Subtitle: "Retour à 14h00", PanelSize: 70, AnimPeriod: 8, AnimIntensity: 30, TransitionMs: 1500}
	q := makeEntracteQuestion("e1", occurrenceCfg)

	// Sélection + PREPARE→READY (Ready) — "Aucune équipe à sélectionner,
	// aucun buzzer mobilisé" (maquette §03).
	e.Ready(q.ID, q)
	if state := e.GetState(); state.Entracte {
		t.Error("Entracte must NOT be active yet at READY — only START raises the panel (maquette §03 timeline)")
	}

	// START — "Lance comme une question" (maquette §03).
	e.StartImmediate(0)
	defer e.Stop()
	if state := e.GetState(); !state.Entracte || state.EntracteConfig != occurrenceCfg {
		t.Fatalf("after START: Entracte=%v EntracteConfig=%+v, want Entracte=true with the occurrence config %+v", e.GetState().Entracte, e.GetState().EntracteConfig, occurrenceCfg)
	}

	// Sortie — retour à l'écran d'attente (maquette §03).
	if ok := e.SetEntracte(false); !ok {
		t.Fatal("exit gesture refused")
	}

	// Le bouton manuel, déclenché APRÈS une pause programmée, doit utiliser
	// la config globale — jamais celle de la pause qui vient de se
	// terminer (c'est précisément le scénario que 214-Q9 corrige).
	if ok := e.SetEntracte(true); !ok {
		t.Fatal("manual entracte activation refused after a programmed pause")
	}
	state := e.GetState()
	if state.EntracteConfig != globalCfg {
		t.Errorf("manual entracte after a programmed pause: EntracteConfig = %+v, want the GLOBAL config %+v — leaking the occurrence's config into the manual button is exactly the bug 214-Q9 guards against", state.EntracteConfig, globalCfg)
	}
}

// ---------------------------------------------------------------------------
// Non-régression : l'entracte manuel global reste identique — comportement
// et test existants inchangés (voir aussi le fichier de test dédié à
// SetEntracte, non modifié par #214).
// ---------------------------------------------------------------------------

func TestEntracteManual_UnaffectedByEntracteProgrammedFeature(t *testing.T) {
	e := NewEngine()
	globalCfg := EntracteConfig{Title: "ENTRACTE", Subtitle: "Retour dans 20mn", PanelSize: 65, AnimPeriod: 10, AnimIntensity: 20, TransitionMs: 2000}
	e.SetEntracteConfig(globalCfg)

	if ok := e.SetEntracte(true); !ok {
		t.Fatal("manual activation refused")
	}
	state := e.GetState()
	if !state.Entracte || state.EntracteConfig != globalCfg {
		t.Errorf("manual entracte: Entracte=%v EntracteConfig=%+v, want Entracte=true with the global config %+v — unchanged pre-#214 behavior", state.Entracte, state.EntracteConfig, globalCfg)
	}
}

// ---------------------------------------------------------------------------
// Scoring/palmarès : une entrée ENTRACTE ne rapporte aucun point.
// ---------------------------------------------------------------------------

func TestEntracteProgrammed_AwardsNoPoints(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "Team Red"}})

	q := makeEntracteQuestion("e1", EntracteConfig{Title: "PAUSE", PanelSize: 65, AnimPeriod: 10, AnimIntensity: 20, TransitionMs: 2000})
	e.Ready(q.ID, q)
	e.StartImmediate(0)
	defer e.Stop()

	scoreBefore := 0
	if team, ok := e.GetTeamsAndBumpers().Teams["red"]; ok {
		scoreBefore = team.Score
	}

	if ok := e.SetEntracte(false); !ok {
		t.Fatal("exit gesture refused")
	}

	team, ok := e.GetTeamsAndBumpers().Teams["red"]
	if !ok {
		t.Fatal("team 'red' missing from state")
	}
	if team.Score != scoreBefore {
		t.Errorf("team score changed from %d to %d across an ENTRACTE question's full cycle — an entracte must never award points", scoreBefore, team.Score)
	}
}
