package game

// #317 (v12.2.0, LOT-1) — BONUS éphémère / permanent : mode effectif de la manche et consommation de la marque
// (contracts/roue.md §17.5, §17.6). Réutilise les aides de roue_bonus_316_test.go (même paquet).
//
// Identifiants Go attendus : RoueWheelParams.BonusMode (global, "" = EPHEMERAL), GameState.RoueBonusMode (JSON
// ROUE_BONUS_MODE, sans omitempty), TypedContent.RoueBonusMode (JSON ROUE_BONUS_MODE).
// Attendu ROUGE tant que les tâches 5 et 6 du plan ne sont pas livrées.

import (
	"encoding/json"
	"testing"
)

func modeOf317(t *testing.T, e *Engine) interface{} {
	t.Helper()
	v, ok := stateMap316(t, e)["ROUE_BONUS_MODE"]
	if !ok {
		t.Fatalf("ROUE_BONUS_MODE absent de l'état : le champ est sérialisé même vide (pas d'omitempty)")
	}
	return v
}

// segsSnapshot317 : copie des cases, pour vérifier que seule une marque peut changer.
func segsSnapshot317(e *Engine) []RoueSegment {
	return append([]RoueSegment(nil), e.GetState().RoueWheel.Segments...)
}

// onlyThisMarkCleared317 : exactement la case `idx` a perdu sa marque ; ID, KIND, VALUE et nombre de cases intacts ;
// aucune autre marque ne change.
func onlyThisMarkCleared317(t *testing.T, before []RoueSegment, e *Engine, idx int) {
	t.Helper()
	after := e.GetState().RoueWheel.Segments
	if len(after) != len(before) {
		t.Fatalf("le nombre de cases change : %d → %d", len(before), len(after))
	}
	for i := range before {
		if after[i].ID != before[i].ID || after[i].Kind != before[i].Kind || after[i].Value != before[i].Value {
			t.Fatalf("la case %d change d'identité : %+v → %+v", i, before[i], after[i])
		}
		wantBonus := before[i].Bonus && i != idx
		if after[i].Bonus != wantBonus {
			t.Fatalf("case %d : BONUS = %v, attendu %v (seule la case jouée perd sa marque)\navant %s\naprès %s", i, after[i].Bonus, wantBonus, render316(before), render316(after))
		}
	}
}

// ---- mode effectif de la manche ---------------------------------------------------------------------------------

// Sans réglage global ni question : éphémère. Hors manche : "".
func TestRoueMode317_DefaultIsEphemeralAndEmptyOutsideARound(t *testing.T) {
	e := NewEngine()
	if got := modeOf317(t, e); got != "" {
		t.Fatalf("NewEngine : ROUE_BONUS_MODE = %v, attendu \"\"", got)
	}
	e2, _ := engine316(t, nil, RoueWheelParams{})
	if got := modeOf317(t, e2); got != "EPHEMERAL" {
		t.Fatalf("manche démarrée sans réglage : %v, attendu EPHEMERAL (défaut décidé)", got)
	}
}

// La question prime (présent = personnalisé), le réglage global sinon ; copié au START.
func TestRoueMode317_QuestionOverridesGlobalOtherwiseGlobal(t *testing.T) {
	perm := RoueWheelParams{MaxPoints: 1000, Step: 50, BankruptCount: 2, LoseTurnCount: 1, BonusMode: "PERMANENT",
		SpinPeriodMs: 2500, SpinMaxMs: 10000, StopDurationMs: 4000, FlipIntervalMs: 600}
	eph := perm
	eph.BonusMode = "EPHEMERAL"
	cases := []struct {
		name   string
		global RoueWheelParams
		q      string // ROUE_BONUS_MODE de la question ("" = absent)
		want   string
	}{
		{"global permanent, question absente", perm, "", "PERMANENT"},
		{"global éphémère, question absente", eph, "", "EPHEMERAL"},
		{"global permanent, question éphémère", perm, "EPHEMERAL", "EPHEMERAL"},
		{"global éphémère, question permanente", eph, "PERMANENT", "PERMANENT"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			over := q316{}
			if c.q != "" {
				over["ROUE_BONUS_MODE"] = c.q
			}
			e, _ := engine316(t, over, c.global)
			if got := modeOf317(t, e); got != c.want {
				t.Fatalf("mode effectif %v, attendu %s", got, c.want)
			}
		})
	}
}

// Le mode est enregistré et copié même quand le nombre de BONUS effectif est 0.
func TestRoueMode317_CopiedEvenWithZeroBonus(t *testing.T) {
	e, _ := engine316(t, q316{"ROUE_BONUS_COUNT": 0, "ROUE_BONUS_MODE": "PERMANENT"}, RoueWheelParams{})
	if got := modeOf317(t, e); got != "PERMANENT" {
		t.Fatalf("mode %v, attendu PERMANENT avec 0 BONUS", got)
	}
}

// Un mode inconnu dans la question (donnée corrompue) ne casse pas le départ : le réglage global s'applique.
func TestRoueMode317_UnknownQuestionModeFallsBackToGlobal(t *testing.T) {
	e, _ := engine316(t, q316{"ROUE_BONUS_MODE": "FOREVER"}, RoueWheelParams{})
	if e.GetState().RoueSubPhase != RoueSubTurn {
		t.Fatalf("départ refusé : %s", e.GetState().RoueSubPhase)
	}
	if got := modeOf317(t, e); got != "EPHEMERAL" {
		t.Fatalf("mode %v, attendu EPHEMERAL (global)", got)
	}
}

// Remise à "" : Ready (nouvelle question), Stop, InitGameWith (NEW_GAME) — mêmes règles que les autres champs ROUE_*.
func TestRoueMode317_ResetOutsideARound(t *testing.T) {
	t.Run("Stop", func(t *testing.T) {
		e, _ := engine316(t, q316{"ROUE_BONUS_MODE": "PERMANENT"}, RoueWheelParams{})
		e.Stop()
		if got := modeOf317(t, e); got != "" {
			t.Fatalf("après Stop : %v", got)
		}
	})
	t.Run("Ready", func(t *testing.T) {
		e, _ := engine316(t, q316{"ROUE_BONUS_MODE": "PERMANENT"}, RoueWheelParams{})
		e.Stop()
		e.Ready("autre", &Question{ID: "autre", Question: "Quiz", Type: QuestionTypeQCM})
		if got := modeOf317(t, e); got != "" {
			t.Fatalf("après Ready d'une autre question : %v", got)
		}
	})
	t.Run("InitGameWith", func(t *testing.T) {
		e, _ := engine316(t, q316{"ROUE_BONUS_MODE": "PERMANENT"}, RoueWheelParams{})
		e.InitGameWith(NewGameOptions{PurgeScores: true, ResetQuestions: true})
		if got := modeOf317(t, e); got != "" {
			t.Fatalf("après NEW_GAME : %v", got)
		}
	})
}

// ---- éphémère : consommation de la marque -----------------------------------------------------------------------

// critical — éphémère : la première action ACCEPTÉE qui quitte LETTER consomme la marque de la case jouée, dans la
// même action ; consonne présente, absente, déjà proposée, voyelle ; seule cette case change.
func TestRoueMode317_EphemeralConsumedByEveryAcceptedAction(t *testing.T) {
	cases := []struct {
		name string
		act  func(e *Engine) error
		pre  func(e *Engine)
	}{
		{"consonne présente", func(e *Engine) error { return e.RoueLetter("T") }, nil},
		{"consonne absente", func(e *Engine) error { return e.RoueLetter("Z") }, nil},
		{"consonne déjà proposée", func(e *Engine) error { return e.RoueLetter("T") }, func(e *Engine) {
			e.mu.Lock()
			e.state.RoueLetters = append(e.state.RoueLetters, RoueLetter{Letter: "T", Team: "blue", Hit: true})
			e.mu.Unlock()
		}},
		{"voyelle présente", func(e *Engine) error { return e.RoueBuyVowel("O") }, nil},
		{"voyelle absente", func(e *Engine) error { return e.RoueBuyVowel("E") }, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, draw := engine316(t, q316{"ROUE_BONUS_MODE": "EPHEMERAL"}, RoueWheelParams{})
			idx := markedIdx316(t, e)[0]
			stopOn316(t, e, draw, idx)
			if c.pre != nil {
				c.pre(e)
			}
			before := segsSnapshot317(e)
			if !before[idx].Bonus {
				t.Fatalf("la marque ne doit pas disparaître avant le choix de la lettre")
			}
			if err := c.act(e); err != nil {
				t.Fatalf("action refusée : %v", err)
			}
			onlyThisMarkCleared317(t, before, e, idx)
			if e.GetState().RoueSubPhase == RoueSubLetter {
				t.Fatalf("l'action doit quitter LETTER")
			}
		})
	}
}

// Une action REFUSÉE ne consomme rien : lettre invalide, voyelle par ROUE_LETTER, voyelle sans cagnotte, voyelle
// refusée par le réglage ; la marque et la sous-phase restent.
func TestRoueMode317_RefusedActionConsumesNothing(t *testing.T) {
	cases := []struct {
		name string
		over q316
		pre  func(e *Engine)
		act  func(e *Engine) error
	}{
		{"pas une lettre", nil, nil, func(e *Engine) error { return e.RoueLetter("1") }},
		{"voyelle par ROUE_LETTER", nil, nil, func(e *Engine) error { return e.RoueLetter("A") }},
		{"cagnotte insuffisante", nil, func(e *Engine) {
			e.mu.Lock()
			e.state.RoueTeamBank["red"] = 0
			e.mu.Unlock()
		}, func(e *Engine) error { return e.RoueBuyVowel("O") }},
		{"voyelle refusée par le réglage", q316{"ROUE_VOWEL_TIMING": "BEFORE"}, nil, func(e *Engine) error { return e.RoueBuyVowel("O") }},
		{"pas une voyelle", nil, nil, func(e *Engine) error { return e.RoueBuyVowel("T") }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			over := q316{"ROUE_BONUS_MODE": "EPHEMERAL"}
			for k, v := range c.over {
				over[k] = v
			}
			e, draw := engine316(t, over, RoueWheelParams{})
			idx := markedIdx316(t, e)[0]
			stopOn316(t, e, draw, idx)
			if c.pre != nil {
				c.pre(e)
			}
			before := segsSnapshot317(e)
			seq := e.GetState().RoueLastEvent.Seq
			if err := c.act(e); err == nil {
				t.Fatalf("l'action devait être refusée")
			}
			after := e.GetState()
			if after.RoueSubPhase != RoueSubLetter || after.RoueLastEvent.Seq != seq || !after.RoueWheel.Segments[idx].Bonus {
				t.Fatalf("un refus ne consomme rien : sous-phase %s, SEQ %d→%d, marque %v", after.RoueSubPhase, seq, after.RoueLastEvent.Seq, after.RoueWheel.Segments[idx].Bonus)
			}
			for i := range before {
				if before[i] != after.RoueWheel.Segments[i] {
					t.Fatalf("la case %d a changé malgré le refus", i)
				}
			}
		})
	}
}

// Éphémère : la case redevient une case à points normale jusqu'à la fin de la manche — un nouvel arrêt dessus est un
// LAND_VALUE (plus de BONUS, plus de bandeau).
func TestRoueMode317_EphemeralCaseIsAPlainValueCaseAfterwards(t *testing.T) {
	e, draw := engine316(t, q316{"ROUE_BONUS_MODE": "EPHEMERAL"}, RoueWheelParams{})
	idx := markedIdx316(t, e)[0]
	stopOn316(t, e, draw, idx)
	if err := e.RoueLetter("Z"); err != nil { // absente : fin de tour
		t.Fatal(err)
	}
	if err := e.RoueNext(RoueHandSame); err != nil {
		t.Fatalf("ROUE_NEXT : %v", err)
	}
	stopOn316(t, e, draw, idx)
	st := e.GetState()
	if st.RoueLastEvent.Kind != RoueEvLandValue || st.RoueSubPhase != RoueSubLetter {
		t.Fatalf("second arrêt sur la case consommée : %s / %s, attendu LAND_VALUE / LETTER", st.RoueLastEvent.Kind, st.RoueSubPhase)
	}
	if st.RoueWheel.Segments[idx].Bonus {
		t.Fatalf("la marque ne revient jamais (jamais false → true)")
	}
}

// Les autres marques restent jouables après la consommation d'une.
func TestRoueMode317_OtherMarksStayPlayable(t *testing.T) {
	e, draw := engine316(t, q316{"ROUE_BONUS_MODE": "EPHEMERAL", "ROUE_BONUS_COUNT": 2}, RoueWheelParams{})
	idxs := markedIdx316(t, e)
	stopOn316(t, e, draw, idxs[0])
	_ = e.RoueLetter("Z")
	_ = e.RoueNext(RoueHandSame)
	stopOn316(t, e, draw, idxs[1])
	if got := e.GetState().RoueLastEvent.Kind; got != RoueEvBonus {
		t.Fatalf("la seconde marque doit encore déclencher un BONUS, got %s", got)
	}
}

// La manche suivante tire une nouvelle roue : les marques consommées sont reposées (nouveau tirage, §17.2).
func TestRoueMode317_NextRoundDrawsAFreshWheel(t *testing.T) {
	e, draw := engine316(t, q316{"ROUE_BONUS_MODE": "EPHEMERAL", "ROUE_BONUS_COUNT": 2}, RoueWheelParams{})
	idx := markedIdx316(t, e)[0]
	stopOn316(t, e, draw, idx)
	_ = e.RoueLetter("T")
	if got := len(markedIdx316(t, e)); got != 1 {
		t.Fatalf("après consommation : %d marque(s), attendu 1", got)
	}
	e.Stop()
	raw := []byte(`{"ID":"b316","QUESTION":"Roue","TYPE":"ROUE","ROUE_PHRASE":"TOTO TATA","ROUE_BONUS_COUNT":2,"ROUE_BONUS_MODE":"EPHEMERAL"}`)
	var q Question
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatal(err)
	}
	e.Ready("b316", &q)
	_ = e.RoueSetTeams([]string{"red", "blue"})
	e.StartImmediate(0)
	if got := len(markedIdx316(t, e)); got != 2 {
		t.Fatalf("nouvelle manche : %d marque(s), attendu 2 (nouveau tirage)", got)
	}
}

// ---- permanent ---------------------------------------------------------------------------------------------------

// critical — permanent : la marque reste après la lettre (toutes les actions) ; un nouvel arrêt sur la case redonne un
// BONUS (événement BONUS, SEQ +1, comme la première fois).
func TestRoueMode317_PermanentKeepsTheMarkAndPlaysItAgain(t *testing.T) {
	for _, name := range []string{"consonne présente", "consonne absente", "voyelle absente"} {
		t.Run(name, func(t *testing.T) {
			e, draw := engine316(t, q316{"ROUE_BONUS_MODE": "PERMANENT"}, RoueWheelParams{})
			idx := markedIdx316(t, e)[0]
			stopOn316(t, e, draw, idx)
			before := segsSnapshot317(e)
			var err error
			switch name {
			case "consonne présente":
				err = e.RoueLetter("T")
			case "consonne absente":
				err = e.RoueLetter("Z")
			default:
				err = e.RoueBuyVowel("E")
			}
			if err != nil {
				t.Fatal(err)
			}
			after := e.GetState().RoueWheel.Segments
			for i := range before {
				if before[i] != after[i] {
					t.Fatalf("permanent : la case %d change (%+v → %+v)", i, before[i], after[i])
				}
			}
			// Fin de tour (lettre ou voyelle absente) puis nouvel arrêt sur la même case. Après une consonne
			// présente le tour continue par d'autres étapes : on s'en tient à la marque conservée.
			if e.GetState().RoueSubPhase != RoueSubTurnEnd {
				return
			}
			if err := e.RoueNext(RoueHandSame); err != nil {
				t.Fatalf("ROUE_NEXT : %v", err)
			}
			seq := e.GetState().RoueLastEvent.Seq
			stopOn316(t, e, draw, idx)
			st := e.GetState()
			if st.RoueLastEvent.Kind != RoueEvBonus || st.RoueLastEvent.Seq != seq+2 || st.RoueSubPhase != RoueSubLetter {
				t.Fatalf("second arrêt : %+v / %s, attendu BONUS (SEQ %d) / LETTER", st.RoueLastEvent, st.RoueSubPhase, seq+2)
			}
			if st.RoueLastEvent.Value != before[idx].Value {
				t.Fatalf("VALUE = %d, attendu %d", st.RoueLastEvent.Value, before[idx].Value)
			}
		})
	}
}

// Le mode PERMANENT global suit le même chemin quand la question ne personnalise rien.
func TestRoueMode317_GlobalPermanentKeepsTheMark(t *testing.T) {
	p := DefaultRoueWheelParams()
	p.BonusMode = "PERMANENT"
	e, draw := engine316(t, nil, p)
	idx := markedIdx316(t, e)[0]
	stopOn316(t, e, draw, idx)
	if err := e.RoueLetter("T"); err != nil {
		t.Fatal(err)
	}
	if !e.GetState().RoueWheel.Segments[idx].Bonus {
		t.Fatalf("global permanent : la marque doit rester")
	}
}
