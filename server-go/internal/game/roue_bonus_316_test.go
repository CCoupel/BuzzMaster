package game

// #316 (v12.2.0, LOT-1) — moteur : arrêt sur une case marquée BONUS et départ de manche
// (contracts/roue.md §17.3 « START », §17.4, §17.6). Une marque est un attribut d'une case à points : l'arrêt est un tour
// NORMAL au montant de la case, SANS relance, avec l'événement BONUS (VALUE = valeur de la case) à la place de LAND_VALUE.
//
// Identifiants Go attendus du dev-backend : RoueSegment.Bonus ; BuildRoueWheel(p, randN) ; aucun RoueKindBonus.
// Les questions sont construites par JSON (noms du contrat) ; l'aléa est injecté par Engine.SetRoueRand.
// Attendu ROUGE tant que les tâches 1, 3, 4 et 6 du plan ne sont pas livrées.

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// phrase de référence « TOTO TATA » : T ×4, O ×2, A ×2 ; consonne absente : Z ; voyelle absente : E.
type q316 map[string]interface{}

// engine316 monte une manche ROUE démarrée (TURN, étape 1), roue par défaut 1000/50/2/1 sauf `p` non nul.
// Cagnotte de red = 1000 (assez pour acheter une voyelle). L'aléa renvoie `*draw`.
func engine316(t *testing.T, over q316, p RoueWheelParams, teams ...string) (*Engine, *int) {
	t.Helper()
	q := map[string]interface{}{"ID": "b316", "QUESTION": "Roue de la Fortune", "TYPE": "ROUE", "ROUE_PHRASE": "TOTO TATA",
		"ROUE_MODE": RoueModeTantQueJeGagne, "ROUE_BUY_VOWELS": true, "ROUE_VOWEL_TIMING": "BOTH", "ROUE_BONUS_COUNT": 2}
	for k, v := range over {
		q[k] = v
	}
	raw, _ := json.Marshal(q)
	var question Question
	if err := json.Unmarshal(raw, &question); err != nil {
		t.Fatal(err)
	}
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red", Color: []int{255, 0, 0}}, "blue": {Name: "blue", Color: []int{0, 0, 255}}})
	if p.MaxPoints != 0 {
		e.SetRoueWheelParams(p)
	}
	e.Ready("b316", &question)
	if len(teams) == 0 {
		teams = []string{"red", "blue"}
	}
	if err := e.RoueSetTeams(teams); err != nil {
		t.Fatal(err)
	}
	draw := new(int)
	e.SetRoueRand(func(int) int { return *draw })
	e.StartImmediate(0)
	t.Cleanup(e.Stop)
	e.mu.Lock()
	e.state.RoueTeamBank["red"] = 1000
	if _, ok := e.state.RoueTeamBank["blue"]; ok {
		e.state.RoueTeamBank["blue"] = 300
	}
	e.mu.Unlock()
	return e, draw
}

func markedIdx316(t *testing.T, e *Engine) []int {
	t.Helper()
	var out []int
	for i, s := range e.GetState().RoueWheel.Segments {
		if s.Bonus {
			out = append(out, i)
		}
	}
	return out
}

// stopOn316 lance la roue et l'arrête sur la case idx ; en retour, la fin du freinage a été jouée.
func stopOn316(t *testing.T, e *Engine, draw *int, idx int) {
	t.Helper()
	*draw = idx
	if err := e.RoueSpin(); err != nil {
		t.Fatalf("RoueSpin : %v", err)
	}
	fireRoueTimer144(e) // fin de la montée
	if err := e.RoueStop(); err != nil {
		t.Fatalf("RoueStop : %v", err)
	}
	fireRoueTimer144(e) // fin du freinage : l'effet de la case s'applique
}

func firstIdx316(t *testing.T, e *Engine, pred func(RoueSegment) bool) int {
	t.Helper()
	for i, s := range e.GetState().RoueWheel.Segments {
		if pred(s) {
			return i
		}
	}
	t.Fatalf("aucune case ne convient")
	return -1
}

func stateMap316(t *testing.T, e *Engine) map[string]interface{} {
	t.Helper()
	raw, err := json.Marshal(e.GetState())
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]interface{}{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func render316Engine(e *Engine) string { return render316(e.GetState().RoueWheel.Segments) }

// ---- départ de manche : la roue ----------------------------------------------------------------------------------

// critical — avec N BONUS, la roue de la manche garde le nombre de cases de la roue sans BONUS ; exactement N marques,
// posées sur des cases `value` ; BANQUEROUTE, PERDU et valeurs identiques à la roue sans BONUS.
func TestRoueBonus316_StartKeepsCaseCountAndSpecials(t *testing.T) {
	base := build316(t, DefaultRoueWheelParams(), 0, seeded316(1))
	for n := 0; n <= 4; n++ {
		t.Run(fmt.Sprintf("bonus=%d", n), func(t *testing.T) {
			e, _ := engine316(t, q316{"ROUE_BONUS_COUNT": n}, RoueWheelParams{})
			segs := e.GetState().RoueWheel.Segments
			if len(segs) != len(base) {
				t.Fatalf("%d cases, attendu %d (les BONUS ne sont plus des cases)", len(segs), len(base))
			}
			if got := len(markedIdx316(t, e)); got != n {
				t.Fatalf("%d marques, attendu %d\n%s", got, n, render316(segs))
			}
			for i, s := range segs {
				if s.Kind == "bonus" {
					t.Fatalf("la case %d a le KIND retiré « bonus »", i)
				}
				if s.Kind != base[i].Kind || s.Value != base[i].Value || s.ID != base[i].ID {
					t.Fatalf("la case %d a changé : %+v → %+v", i, base[i], s)
				}
				if s.Bonus && s.Kind != RoueKindValue {
					t.Fatalf("marque sur une case %q", s.Kind)
				}
			}
		})
	}
}

// Les marques se posent au hasard de l'aléa du moteur (injectable) à chaque START : le même aléa redonne la même roue,
// des aléas différents des dispositions différentes ; les cases spéciales ne bougent jamais.
func TestRoueBonus316_MarksFollowTheInjectedRandomSource(t *testing.T) {
	startWith := func(v int) string {
		e := NewEngine()
		e.SetTeams(map[string]*Team{"red": {Name: "red"}})
		var question Question
		_ = json.Unmarshal([]byte(`{"ID":"r","QUESTION":"Roue","TYPE":"ROUE","ROUE_PHRASE":"TOTO","ROUE_BONUS_COUNT":2}`), &question)
		e.Ready("r", &question)
		_ = e.RoueSetTeams([]string{"red"})
		e.SetRoueRand(func(n int) int { return v % n })
		e.StartImmediate(0)
		defer e.Stop()
		return render316Engine(e)
	}
	layouts := map[string]bool{}
	for v := 0; v < 12; v++ {
		a := startWith(v)
		if b := startWith(v); a != b {
			t.Fatalf("même aléa, roues différentes :\n%s\n%s", a, b)
		}
		layouts[a] = true
	}
	if len(layouts) < 3 {
		t.Fatalf("12 aléas ne donnent que %d disposition(s) : les marques n'utilisent pas l'aléa du moteur", len(layouts))
	}
}

// START ne refuse jamais : BONUS au-dessus du maximum = réduits au maximum permis (1000/100/2/1 : 2 au plus).
func TestRoueBonus316_ReducedAtStartNeverRefused(t *testing.T) {
	p := RoueWheelParams{MaxPoints: 1000, Step: 100, BankruptCount: 2, LoseTurnCount: 1}
	e, _ := engine316(t, q316{"ROUE_BONUS_COUNT": 4}, p)
	st := e.GetState()
	if st.Phase != PhaseStarted || st.RoueSubPhase != RoueSubTurn {
		t.Fatalf("le départ ne doit jamais être refusé : %s / %s", st.Phase, st.RoueSubPhase)
	}
	if got := len(markedIdx316(t, e)); got != 2 {
		t.Fatalf("4 BONUS demandés, 2 permis : %d marques\n%s", got, render316Engine(e))
	}
	if len(st.RoueWheel.Segments) != 12 {
		t.Fatalf("la roue doit rester à 12 cases, got %d", len(st.RoueWheel.Segments))
	}
}

// Aucune place pour un BONUS (100/30→25/1/1 : 5 cases, 0 permis) : roue sans marque, départ accepté.
func TestRoueBonus316_NoRoomMeansNoMarkAndStillStarts(t *testing.T) {
	p := RoueWheelParams{MaxPoints: 100, Step: 30, BankruptCount: 1, LoseTurnCount: 1}
	e, _ := engine316(t, q316{"ROUE_BONUS_COUNT": 3}, p)
	if e.GetState().RoueSubPhase != RoueSubTurn {
		t.Fatalf("départ refusé : %s", e.GetState().RoueSubPhase)
	}
	if got := markedIdx316(t, e); len(got) != 0 {
		t.Fatalf("aucune marque possible, got %v", got)
	}
}

// Une question v12.1 (ROUE_BONUS_COUNT = n, sans ROUE_BONUS_MODE) s'ouvre et se joue : n marques posées sur des cases à
// points, la roue perd les n cases que les BONUS ajoutaient (migration §17.7).
func TestRoueBonus316_V121QuestionMigrates(t *testing.T) {
	e, _ := engine316(t, q316{"ROUE_BONUS_COUNT": 2}, RoueWheelParams{})
	if got := len(e.GetState().RoueWheel.Segments); got != 22 {
		t.Fatalf("v12.1 : 24 cases (22 + 2 BONUS) ; v12.2 : 22 attendues, got %d", got)
	}
	if got := len(markedIdx316(t, e)); got != 2 {
		t.Fatalf("2 marques attendues, got %d", got)
	}
}

// ---- arrêt sur une case marquée -----------------------------------------------------------------------------------

type marked316 struct {
	e     *Engine
	draw  *int
	idx   int
	value int
	seq0  int
}

// markedStop316 : manche démarrée, la roue s'arrête sur une case marquée ; la fin du freinage est jouée.
func markedStop316(t *testing.T, over q316, p RoueWheelParams, teams ...string) marked316 {
	t.Helper()
	e, draw := engine316(t, over, p, teams...)
	idxs := markedIdx316(t, e)
	if len(idxs) == 0 {
		t.Fatalf("aucune case marquée à jouer\n%s", render316Engine(e))
	}
	idx := idxs[0]
	seq0 := e.GetState().RoueLastEvent.Seq
	stopOn316(t, e, draw, idx)
	return marked316{e: e, draw: draw, idx: idx, value: e.GetState().RoueWheel.Segments[idx].Value, seq0: seq0}
}

// critical — arrêt sur une case marquée : LETTER (étape 4), événement BONUS {TEAM, VALUE = valeur de la case}, ni
// LAND_VALUE, ni relance, ni retour en TURN ; la cagnotte et le gain du tour ne bougent pas.
func TestRoueBonus316_StopOnMarkedCaseIsALetterTurnWithBonusEvent(t *testing.T) {
	for _, mode := range []string{RoueModeTantQueJeGagne, RoueModeChacunSonTour} {
		t.Run(mode, func(t *testing.T) {
			m := markedStop316(t, q316{"ROUE_MODE": mode}, RoueWheelParams{})
			st := m.e.GetState()
			if st.RoueSubPhase != RoueSubLetter {
				t.Fatalf("attendu LETTER, got %s", st.RoueSubPhase)
			}
			ev := st.RoueLastEvent
			if ev.Kind != RoueEvBonus || ev.Team != "red" || ev.Value != m.value || ev.Seq != m.seq0+2 {
				t.Fatalf("événement = %+v, attendu BONUS / red / VALUE %d / SEQ %d (SPIN puis BONUS)", ev, m.value, m.seq0+2)
			}
			if m.value <= 0 {
				t.Fatalf("la case marquée garde sa valeur (%d)", m.value)
			}
			if st.RoueWheel.Current != m.idx {
				t.Fatalf("CURRENT = %d, attendu %d", st.RoueWheel.Current, m.idx)
			}
			if st.RoueCurrentTeam != "red" {
				t.Fatalf("la main ne change pas : %q", st.RoueCurrentTeam)
			}
			if st.RoueTeamBank["red"] != 1000 || st.RoueTeamBank["blue"] != 300 || st.RoueTurnGain != 0 {
				t.Fatalf("cagnottes %v / gain %d : aucun point n'est attribué au BONUS", st.RoueTeamBank, st.RoueTurnGain)
			}
			for name, tm := range m.e.GetTeamsAndBumpers().Teams {
				if tm.TeamPoints != 0 {
					t.Errorf("score de partie de %s modifié : %d", name, tm.TeamPoints)
				}
			}
		})
	}
}

// Le même arrêt sur une case à points NON marquée reste un LAND_VALUE (non-régression), sans événement BONUS.
func TestRoueBonus316_StopOnUnmarkedValueCaseStaysLandValue(t *testing.T) {
	e, draw := engine316(t, nil, RoueWheelParams{})
	idx := firstIdx316(t, e, func(s RoueSegment) bool { return s.Kind == RoueKindValue && !s.Bonus })
	stopOn316(t, e, draw, idx)
	st := e.GetState()
	if st.RoueSubPhase != RoueSubLetter || st.RoueLastEvent.Kind != RoueEvLandValue || st.RoueLastEvent.Value != st.RoueWheel.Segments[idx].Value {
		t.Fatalf("case non marquée : %s / %+v", st.RoueSubPhase, st.RoueLastEvent)
	}
}

// BANQUEROUTE et PERDU gardent leur effet v12.0 sur une roue qui contient des marques.
func TestRoueBonus316_SpecialsKeepV120EffectOnAMarkedWheel(t *testing.T) {
	cases := []struct {
		kind, sub, ev string
		bank          int
	}{
		{RoueKindBankrupt, RoueSubTurnEnd, RoueEvBankrupt, 0},
		{RoueKindLoseTurn, RoueSubTurnEnd, RoueEvLoseTurn, 1000},
	}
	for _, c := range cases {
		t.Run(c.kind, func(t *testing.T) {
			e, draw := engine316(t, nil, RoueWheelParams{})
			stopOn316(t, e, draw, firstIdx316(t, e, func(s RoueSegment) bool { return s.Kind == c.kind }))
			st := e.GetState()
			if st.RoueSubPhase != c.sub || st.RoueLastEvent.Kind != c.ev || st.RoueTeamBank["red"] != c.bank {
				t.Fatalf("%s : %s / %s / banque %d", c.kind, st.RoueSubPhase, st.RoueLastEvent.Kind, st.RoueTeamBank["red"])
			}
		})
	}
}

// Après l'arrêt, la consonne est jouée au montant de la case : gain = nombre d'occurrences × valeur de la case.
func TestRoueBonus316_ConsonantAfterMarkedStopPaysTheCaseValue(t *testing.T) {
	m := markedStop316(t, nil, RoueWheelParams{})
	if err := m.e.RoueLetter("T"); err != nil { // T : 4 occurrences dans « TOTO TATA »
		t.Fatalf("ROUE_LETTER après un BONUS : %v", err)
	}
	st := m.e.GetState()
	if st.RoueSubPhase != RoueSubLit {
		t.Fatalf("consonne présente : attendu LIT, got %s", st.RoueSubPhase)
	}
	if ev := st.RoueLastEvent; ev.Kind != RoueEvLetterHit || ev.Count != 4 || ev.Value != m.value || ev.Gain != 4*m.value {
		t.Fatalf("événement = %+v, attendu LETTER_HIT ×4 à %d = %d", ev, m.value, 4*m.value)
	}
	if st.RoueTurnGain != 4*m.value {
		t.Fatalf("gain du tour = %d, attendu %d", st.RoueTurnGain, 4*m.value)
	}
}

// Consonne absente après le BONUS : fin de tour comme sur une case à points.
func TestRoueBonus316_AbsentConsonantEndsTheTurnLikeAValueCase(t *testing.T) {
	m := markedStop316(t, nil, RoueWheelParams{})
	if err := m.e.RoueLetter("Z"); err != nil {
		t.Fatal(err)
	}
	st := m.e.GetState()
	if st.RoueSubPhase != RoueSubTurnEnd || st.RoueLastEvent.Kind != RoueEvLetterMiss {
		t.Fatalf("consonne absente : %s / %s", st.RoueSubPhase, st.RoueLastEvent.Kind)
	}
}

// Voyelle « après l'arrêt » : permise quand le réglage de la question le permet (ROUE_VOWEL_TIMING AFTER / BOTH).
func TestRoueBonus316_VowelAfterMarkedStop(t *testing.T) {
	for _, timing := range []string{"AFTER", "BOTH"} {
		t.Run(timing, func(t *testing.T) {
			m := markedStop316(t, q316{"ROUE_VOWEL_TIMING": timing}, RoueWheelParams{})
			if err := m.e.RoueBuyVowel("O"); err != nil {
				t.Fatalf("voyelle après un BONUS (%s) refusée : %v", timing, err)
			}
			if got := m.e.GetState().RoueLastEvent.Kind; got != RoueEvVowelHit {
				t.Fatalf("événement %s, attendu VOWEL_HIT", got)
			}
		})
	}
	// Réglage « avant » : refus, rien ne change.
	m := markedStop316(t, q316{"ROUE_VOWEL_TIMING": "BEFORE"}, RoueWheelParams{})
	if err := m.e.RoueBuyVowel("O"); err == nil {
		t.Fatalf("voyelle après l'arrêt refusée par le réglage BEFORE : attendu une erreur")
	}
	if got := m.e.GetState().RoueSubPhase; got != RoueSubLetter {
		t.Fatalf("après un refus la sous-phase reste LETTER, got %s", got)
	}
}

// Pas de relance : ROUE_SPIN est refusé pendant LETTER (étape 4), la roue ne repart pas.
func TestRoueBonus316_NoRespinAfterAMarkedStop(t *testing.T) {
	m := markedStop316(t, nil, RoueWheelParams{})
	if err := m.e.RoueSpin(); err == nil {
		t.Fatalf("relance acceptée après un arrêt sur une case marquée : le tour continue par la consonne")
	}
	if got := m.e.GetState().RoueSubPhase; got != RoueSubLetter {
		t.Fatalf("sous-phase %s, attendu LETTER", got)
	}
}

// Une seule notice sonore : BONUS (« bonne réponse » + comète), 0 point, émise une fois, à la fin du freinage ;
// aucune notice BONUS sur une case non marquée.
func TestRoueBonus316_NoticeBonusOnceAndOnlyOnMarkedCases(t *testing.T) {
	e, draw := engine316(t, nil, RoueWheelParams{})
	var notices []RoueNotice
	e.OnRoueNotice = func(n RoueNotice) { notices = append(notices, n) }
	idx := markedIdx316(t, e)[0]
	stopOn316(t, e, draw, idx)
	if len(notices) != 1 || notices[0].Kind != RoueNoticeBonus || notices[0].Team != "red" || notices[0].Points != 0 {
		t.Fatalf("notices = %+v, attendu une seule {BONUS red 0}", notices)
	}
	// Case non marquée : aucune notice BONUS.
	e2, draw2 := engine316(t, nil, RoueWheelParams{})
	var n2 []RoueNotice
	e2.OnRoueNotice = func(n RoueNotice) { n2 = append(n2, n) }
	stopOn316(t, e2, draw2, firstIdx316(t, e2, func(s RoueSegment) bool { return s.Kind == RoueKindValue && !s.Bonus }))
	for _, n := range n2 {
		if n.Kind == RoueNoticeBonus {
			t.Fatalf("notice BONUS sur une case non marquée : %+v", n)
		}
	}
}

// SOLO : l'unique équipe garde la main de la même façon.
func TestRoueBonus316_StopOnMarkedCaseSolo(t *testing.T) {
	m := markedStop316(t, q316{"ROUE_MODE": RoueModeSolo}, RoueWheelParams{}, "red")
	st := m.e.GetState()
	if st.RoueSubPhase != RoueSubLetter || st.RoueLastEvent.Kind != RoueEvBonus {
		t.Fatalf("SOLO : %s / %s", st.RoueSubPhase, st.RoueLastEvent.Kind)
	}
}

// ---- état sérialisé ------------------------------------------------------------------------------------------------

// Chaque case se sérialise avec KIND, VALUE et BONUS (sans omitempty, false pour BANQUEROUTE / PERDU) ; plus aucun
// KIND « bonus » ; le champ BONUS est présent même avec 0 BONUS.
func TestRoueBonus316_SegmentJSONAlwaysCarriesBONUS(t *testing.T) {
	for _, n := range []int{0, 2} {
		e, _ := engine316(t, q316{"ROUE_BONUS_COUNT": n}, RoueWheelParams{})
		raw, _ := json.Marshal(e.GetState().RoueWheel)
		var w struct {
			Segments []map[string]interface{} `json:"SEGMENTS"`
		}
		if err := json.Unmarshal(raw, &w); err != nil {
			t.Fatal(err)
		}
		marks := 0
		for i, s := range w.Segments {
			for _, k := range []string{"ID", "KIND", "VALUE", "BONUS"} {
				if _, ok := s[k]; !ok {
					t.Fatalf("n=%d : la case %d n'a pas le champ %s : %v", n, i, k, s)
				}
			}
			if s["KIND"] == "bonus" {
				t.Fatalf("KIND « bonus » retiré en v12.2.0 : %v", s)
			}
			if b, ok := s["BONUS"].(bool); !ok {
				t.Fatalf("BONUS n'est pas un booléen : %v", s)
			} else if b {
				marks++
				if s["KIND"] != "value" {
					t.Fatalf("marque sur une case %v", s["KIND"])
				}
			}
		}
		if marks != n {
			t.Fatalf("n=%d : %d marques sérialisées", n, marks)
		}
	}
	// Forme exacte d'une case marquée.
	e, _ := engine316(t, nil, RoueWheelParams{})
	i := markedIdx316(t, e)[0]
	b, _ := json.Marshal(e.GetState().RoueWheel.Segments[i])
	want := fmt.Sprintf(`{"ID":"s%d","KIND":"value","VALUE":%d,"BONUS":true}`, i, e.GetState().RoueWheel.Segments[i].Value)
	if string(b) != want {
		t.Fatalf("JSON = %s, attendu %s", b, want)
	}
}

// Sécurité de la phrase : les marques n'ajoutent aucun champ qui expose la phrase au fil (ROUE_WHEEL ne contient jamais
// la phrase ; la non-fuite vers TV / tablettes reste couverte par les tests de fuite existants).
func TestRoueBonus316_WheelStateDoesNotCarryThePhrase(t *testing.T) {
	e, _ := engine316(t, nil, RoueWheelParams{})
	raw, _ := json.Marshal(e.GetState().RoueWheel)
	if strings.Contains(strings.ToUpper(string(raw)), "TOTO") {
		t.Fatalf("ROUE_WHEEL expose la phrase : %s", raw)
	}
}
