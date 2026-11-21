package game

// #308 (v12.1.0, LOT-1.1) — case BONUS de la roue : spécification (contracts/roue.md §15.2, §15.4, §15.5).
//
// Identifiants Go attendus du dev-backend (le contrat fixe les noms JSON, pas les noms Go) :
//   - TypedContent.RoueBonusCount (int, JSON ROUE_BONUS_COUNT, omitempty)
//   - RoueKindBonus = "bonus", RoueEvBonus = "BONUS", RoueNoticeBonus = "BONUS"
// L'API publique du moteur (START, RoueSpin / RoueStop / RoueLetter / RoucePass, GetState) est inchangée.

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

// roueBonusEngine308 monte une manche ROUE démarrée avec `bonus` cases BONUS et des paramètres de roue donnés
// (zéro valeur = roue par défaut 1000/50/2/1). Le tirage est figé : `draw` renvoie l'index de la case tirée.
func roueBonusEngine308(t *testing.T, bonus int, mode string, p RoueWheelParams, draw func(n int) int, teams ...string) *Engine {
	t.Helper()
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red", Color: []int{255, 0, 0}}, "blue": {Name: "blue", Color: []int{0, 0, 255}}})
	q := &Question{ID: "roue308", Question: "Roue de la Fortune", Type: QuestionTypeRoue,
		TypedContent: TypedContent{RouePhrase: "Toto tata", RoueMode: mode, RoueBonusCount: bonus}}
	e.Ready("roue308", q)
	if len(teams) == 0 {
		teams = []string{"red", "blue"}
	}
	if err := e.RoueSetTeams(teams); err != nil {
		t.Fatalf("RoueSetTeams: %v", err)
	}
	if p.MaxPoints != 0 {
		e.SetRoueWheelParams(p)
	}
	if draw == nil {
		draw = func(int) int { return 0 }
	}
	e.SetRoueRand(draw)
	e.StartImmediate(0)
	t.Cleanup(e.Stop)
	return e
}

func kindIndexes308(segs []RoueSegment, kind string) []int {
	var out []int
	for i, s := range segs {
		if s.Kind == kind {
			out = append(out, i)
		}
	}
	return out
}

func countKind308(segs []RoueSegment, kind string) int { return len(kindIndexes308(segs, kind)) }

// ---- roue générée --------------------------------------------------------------------------------------------

// critical — avec 0 BONUS (champ absent ou 0), la roue de la manche est identique case pour case à celle de v12.0
// (même ordre, mêmes ID), pour plusieurs configurations (contrat §15.2.2, §15.5).
func TestRoueBonus308_ZeroBonusWheelIsIdenticalToV120(t *testing.T) {
	configs := []RoueWheelParams{
		DefaultRoueWheelParams(),
		{MaxPoints: 100, Step: 10, BankruptCount: 0, LoseTurnCount: 0},
		{MaxPoints: 100, Step: 10, BankruptCount: 1, LoseTurnCount: 1},
		{MaxPoints: 1000, Step: 100, BankruptCount: 3, LoseTurnCount: 2},
		{MaxPoints: 100, Step: 10, BankruptCount: 0, LoseTurnCount: 2},
		{MaxPoints: 100, Step: 30, BankruptCount: 1, LoseTurnCount: 1}, // pas corrigé à 25
	}
	for _, p := range configs {
		t.Run(fmt.Sprintf("%d/%d/%d/%d", p.MaxPoints, p.Step, p.BankruptCount, p.LoseTurnCount), func(t *testing.T) {
			np, _, err := NormalizeRoueWheelParams(p)
			if err != nil {
				t.Fatal(err)
			}
			want := BuildRoueWheel(np)
			e := roueBonusEngine308(t, 0, RoueModeTantQueJeGagne, p, nil)
			got := e.GetState().RoueWheel.Segments
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("roue différente de v12.0 avec 0 BONUS\n got  %+v\n want %+v", got, want)
			}
			if n := countKind308(got, RoueKindBonus); n != 0 {
				t.Fatalf("%d case(s) BONUS avec ROUE_BONUS_COUNT = 0", n)
			}
		})
	}
}

// Les cases BONUS s'AJOUTENT : exactement `n` cases BONUS (VALUE 0), la roue grossit de `n`, aucune valeur ni
// BANQUEROUTE / TOUR PERDU n'est retirée, le maximum reste à l'index 0, ID séquentiels.
func TestRoueBonus308_ExactCountAndNothingDisplaced(t *testing.T) {
	base := DefaultRoueWheelParams()
	np, _, _ := NormalizeRoueWheelParams(base)
	v120 := BuildRoueWheel(np)
	for n := 1; n <= 4; n++ {
		t.Run(fmt.Sprintf("bonus=%d", n), func(t *testing.T) {
			e := roueBonusEngine308(t, n, RoueModeTantQueJeGagne, base, nil)
			segs := e.GetState().RoueWheel.Segments
			if got := countKind308(segs, RoueKindBonus); got != n {
				t.Fatalf("cases BONUS = %d, attendu %d", got, n)
			}
			if len(segs) != len(v120)+n {
				t.Fatalf("la roue a %d cases, attendu %d (v12.0 + %d)", len(segs), len(v120)+n, n)
			}
			for _, s := range segs {
				if s.Kind == RoueKindBonus && s.Value != 0 {
					t.Errorf("case BONUS %s avec VALUE %d, attendu 0", s.ID, s.Value)
				}
			}
			for _, kind := range []string{RoueKindValue, RoueKindBankrupt, RoueKindLoseTurn} {
				if countKind308(segs, kind) != countKind308(v120, kind) {
					t.Errorf("le nombre de cases %q a changé : %d → %d", kind, countKind308(v120, kind), countKind308(segs, kind))
				}
			}
			valuesOf := func(ss []RoueSegment) map[int]int {
				m := map[int]int{}
				for _, s := range ss {
					if s.Kind == RoueKindValue {
						m[s.Value]++
					}
				}
				return m
			}
			if !reflect.DeepEqual(valuesOf(segs), valuesOf(v120)) {
				t.Errorf("les valeurs de la roue ont changé quand on ajoute des BONUS")
			}
			if segs[0].Kind != RoueKindValue || segs[0].Value != base.MaxPoints {
				t.Errorf("l'index 0 doit rester le maximum, got %+v", segs[0])
			}
			if segs[1].Kind != RoueKindBankrupt {
				t.Errorf("la première BANQUEROUTE reste à côté du maximum (index 1), got %+v", segs[1])
			}
			for i, s := range segs {
				if s.ID != fmt.Sprintf("s%d", i) {
					t.Errorf("ID de la case %d = %q, attendu s%d", i, s.ID, i)
				}
			}
		})
	}
}

// Placement (§15.2.2) : l'ordre des types suit « Banqueroute, puis Perdu, BONUS, Banqueroute… » ; avec 1 de chaque,
// les trois cases spéciales sont dans l'ordre BANQUEROUTE, TOUR PERDU, BONUS le long de la roue.
func TestRoueBonus308_PlacementOrderOneOfEach(t *testing.T) {
	p := RoueWheelParams{MaxPoints: 100, Step: 10, BankruptCount: 1, LoseTurnCount: 1}
	e := roueBonusEngine308(t, 1, RoueModeTantQueJeGagne, p, nil)
	var order []string
	for _, s := range e.GetState().RoueWheel.Segments {
		if s.Kind != RoueKindValue {
			order = append(order, s.Kind)
		}
	}
	want := []string{RoueKindBankrupt, RoueKindLoseTurn, RoueKindBonus}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("ordre des cases spéciales = %v, attendu %v", order, want)
	}
}

// Placement sans BANQUEROUTE : les BONUS n'occupent jamais l'index 0 (réservé au maximum) et ne se superposent pas.
func TestRoueBonus308_NeverOnIndexZeroNorOverlapping(t *testing.T) {
	for _, p := range []RoueWheelParams{
		{MaxPoints: 100, Step: 10, BankruptCount: 0, LoseTurnCount: 0},
		{MaxPoints: 100, Step: 50, BankruptCount: 0, LoseTurnCount: 1},
		DefaultRoueWheelParams(),
	} {
		for n := 1; n <= 4; n++ {
			e := roueBonusEngine308(t, n, RoueModeTantQueJeGagne, p, nil)
			segs := e.GetState().RoueWheel.Segments
			if segs[0].Kind != RoueKindValue {
				t.Errorf("%+v bonus=%d : l'index 0 n'est plus une valeur (%+v)", p, n, segs[0])
			}
			if got := countKind308(segs, RoueKindBonus); got != n {
				t.Errorf("%+v bonus=%d : %d cases BONUS", p, n, got)
			}
		}
	}
}

// Limite de 64 cases : au START, un nombre de BONUS qui ferait dépasser 64 est RÉDUIT, le départ n'est jamais refusé.
func TestRoueBonus308_ReducedAtStartToFitIn64Cases(t *testing.T) {
	p := RoueWheelParams{MaxPoints: 63, Step: 1} // 63 cases : une seule place libre
	e := roueBonusEngine308(t, 4, RoueModeTantQueJeGagne, p, nil)
	st := e.GetState()
	if st.Phase != PhaseStarted || st.RoueSubPhase != RoueSubTurn {
		t.Fatalf("le départ ne doit jamais être refusé : phase %s / %s", st.Phase, st.RoueSubPhase)
	}
	segs := st.RoueWheel.Segments
	if len(segs) > RoueMaxSegments {
		t.Fatalf("la roue a %d cases, limite %d", len(segs), RoueMaxSegments)
	}
	if got := countKind308(segs, RoueKindBonus); got != 1 || len(segs) != 64 {
		t.Fatalf("attendu 1 case BONUS (réduction de 4 à 1) pour 64 cases, got %d BONUS / %d cases", got, len(segs))
	}
}

// Roue déjà à 64 cases : tous les BONUS sont retirés (roue v12.0), départ accepté.
func TestRoueBonus308_ReducedToZeroWhenWheelIsFull(t *testing.T) {
	p := RoueWheelParams{MaxPoints: 64, Step: 1}
	e := roueBonusEngine308(t, 3, RoueModeTantQueJeGagne, p, nil)
	segs := e.GetState().RoueWheel.Segments
	if len(segs) != 64 || countKind308(segs, RoueKindBonus) != 0 {
		t.Fatalf("attendu 64 cases sans BONUS, got %d cases dont %d BONUS", len(segs), countKind308(segs, RoueKindBonus))
	}
	if e.GetState().RoueSubPhase != RoueSubTurn {
		t.Fatalf("départ refusé : %s", e.GetState().RoueSubPhase)
	}
}

// ---- arrêt sur BONUS -----------------------------------------------------------------------------------------

// spinAndStop308 lance la roue, termine la montée, force l'arrêt ; la roue freine vers l'index tiré (non encore appliqué).
func spinAndStop308(t *testing.T, e *Engine) {
	t.Helper()
	if err := e.RoueSpin(); err != nil {
		t.Fatal(err)
	}
	fireRoueTimer144(e) // fin de la montée (FULL_SPEED)
	if err := e.RoueStop(); err != nil {
		t.Fatal(err)
	}
}

type bonusStop308 struct {
	e        *Engine
	bonusIdx int
	seqPre   int
}

// bonusStopEngine308 : roue 100/10/0/0 + 1 BONUS ; le tirage tombe sur la case BONUS.
func bonusStopEngine308(t *testing.T, mode string, teams ...string) bonusStop308 {
	t.Helper()
	p := RoueWheelParams{MaxPoints: 100, Step: 10}
	idx := -1
	e := roueBonusEngine308(t, 1, mode, p, func(int) int { return idx }, teams...)
	idx = kindIndexes308(e.GetState().RoueWheel.Segments, RoueKindBonus)[0]
	// Cagnotte non nulle pour prouver qu'elle ne bouge pas.
	e.mu.Lock()
	e.state.RoueTeamBank["red"] = 500
	e.state.RoueTeamBank["blue"] = 300
	e.mu.Unlock()
	spinAndStop308(t, e)
	if got := e.GetState().RoueSubPhase; got != RoueSubStopping {
		t.Fatalf("attendu STOPPING, got %s", got)
	}
	return bonusStop308{e: e, bonusIdx: idx, seqPre: e.GetState().RoueLastEvent.Seq}
}

// critical — l'arrêt sur BONUS ne modifie ni cagnotte, ni gain du tour, ni lettres, ni score de partie ; la même
// équipe rejoue (nouveau TURN, étape 1) ; un seul événement BONUS (§15.2.3).
func TestRoueBonus308_StopOnBonusChangesNothingAndSameTeamPlaysAgain(t *testing.T) {
	for _, mode := range []string{RoueModeTantQueJeGagne, RoueModeChacunSonTour} {
		t.Run(mode, func(t *testing.T) {
			s := bonusStopEngine308(t, mode)
			fireRoueTimer144(s.e) // fin du freinage : l'effet de la case s'applique
			st := s.e.GetState()
			if st.RoueSubPhase != RoueSubTurn || st.RoueStep != 1 {
				t.Fatalf("attendu TURN / étape 1, got %s / %d", st.RoueSubPhase, st.RoueStep)
			}
			if st.RoueCurrentTeam != "red" {
				t.Fatalf("l'équipe garde la main (même en %s) : got %q", mode, st.RoueCurrentTeam)
			}
			if st.RoueWheel.Current != s.bonusIdx || st.RoueWheel.Stop != nil {
				t.Fatalf("CURRENT = %d (attendu %d), STOP = %+v", st.RoueWheel.Current, s.bonusIdx, st.RoueWheel.Stop)
			}
			if st.RoueTeamBank["red"] != 500 || st.RoueTeamBank["blue"] != 300 {
				t.Fatalf("cagnottes modifiées : %v", st.RoueTeamBank)
			}
			if st.RoueTurnGain != 0 || len(st.RoueLetters) != 0 || !st.RoueConsonantsLeft {
				t.Fatalf("gain %d / lettres %v / consonnes restantes %v", st.RoueTurnGain, st.RoueLetters, st.RoueConsonantsLeft)
			}
			for name, tm := range s.e.GetTeamsAndBumpers().Teams {
				if tm.TeamPoints != 0 {
					t.Errorf("score de partie de %s modifié : %d", name, tm.TeamPoints)
				}
			}
			if st.RoueNextTeam != "" || st.RoueNextDefault != "" {
				t.Errorf("aucun bouton de fin de tour conseillé : next=%q default=%q", st.RoueNextTeam, st.RoueNextDefault)
			}
			if ev := st.RoueLastEvent; ev.Kind != RoueEvBonus || ev.Team != "red" || ev.Seq != s.seqPre+1 {
				t.Fatalf("événement = %+v, attendu BONUS / red / SEQ %d (un seul événement)", ev, s.seqPre+1)
			}
		})
	}
}

// SOLO : l'unique équipe rejoue aussi.
func TestRoueBonus308_StopOnBonusSolo(t *testing.T) {
	s := bonusStopEngine308(t, RoueModeSolo, "red")
	fireRoueTimer144(s.e)
	st := s.e.GetState()
	if st.RoueSubPhase != RoueSubTurn || st.RoueCurrentTeam != "red" || st.RoueLastEvent.Kind != RoueEvBonus {
		t.Fatalf("SOLO : %s / %s / %s", st.RoueSubPhase, st.RoueCurrentTeam, st.RoueLastEvent.Kind)
	}
}

// Pendant le freinage, rien n'est encore appliqué ; après l'arrêt aucune lettre n'est acceptée (pas d'étape 4),
// aucune fin de tour n'est proposée, la roue peut être relancée, la main peut être passée (admin).
func TestRoueBonus308_AfterBonusNoLetterNoEndOfTurnButSpinAndPassWork(t *testing.T) {
	s := bonusStopEngine308(t, RoueModeTantQueJeGagne)
	if got := s.e.GetState().RoueLastEvent.Kind; got == RoueEvBonus {
		t.Fatalf("l'événement BONUS ne doit pas précéder la fin du freinage")
	}
	fireRoueTimer144(s.e)
	if err := s.e.RoueLetter("T"); err == nil || err.(*RoueError).Code != RoueErrWrongSubPhase {
		t.Fatalf("ROUE_LETTER après un BONUS : attendu WRONG_SUBPHASE, got %v", err)
	}
	if err := s.e.RoueNext(RoueHandSame); err == nil {
		t.Fatalf("ROUE_NEXT après un BONUS : refusé (pas d'étape 7), got nil")
	}
	if err := s.e.RoueFlip(); err == nil {
		t.Fatalf("ROUE_FLIP après un BONUS : refusé")
	}
	// Relance : la roue repart, le gain BONUS n'a rien laissé.
	if err := s.e.RoueSpin(); err != nil {
		t.Fatalf("relance après un BONUS : %v", err)
	}
	if got := s.e.GetState().RoueSubPhase; got != RoueSubSpinning {
		t.Fatalf("attendu SPINNING, got %s", got)
	}
}

func TestRoueBonus308_PassHandAfterBonusKeepsPotsAndMovesHand(t *testing.T) {
	s := bonusStopEngine308(t, RoueModeTantQueJeGagne)
	fireRoueTimer144(s.e)
	if err := s.e.RoucePass(); err != nil {
		t.Fatalf("ROUE_PASS après un BONUS : %v", err)
	}
	st := s.e.GetState()
	if st.RoueCurrentTeam != "blue" || st.RoueTeamBank["red"] != 500 || st.RoueSubPhase != RoueSubTurn {
		t.Fatalf("après PASS : %s / %v / %s", st.RoueCurrentTeam, st.RoueTeamBank, st.RoueSubPhase)
	}
}

// Le BONUS se joue plusieurs fois de suite : deux arrêts successifs sur BONUS, deux événements, SEQ +1 chacun.
func TestRoueBonus308_BonusTwiceInARow(t *testing.T) {
	s := bonusStopEngine308(t, RoueModeTantQueJeGagne)
	fireRoueTimer144(s.e)
	seq1 := s.e.GetState().RoueLastEvent.Seq
	spinAndStop308(t, s.e) // SPIN (+1) puis le freinage
	fireRoueTimer144(s.e)
	st := s.e.GetState()
	if st.RoueSubPhase != RoueSubTurn || st.RoueCurrentTeam != "red" || st.RoueLastEvent.Kind != RoueEvBonus || st.RoueLastEvent.Seq != seq1+2 {
		t.Fatalf("second BONUS : %s / %s / %+v (SEQ attendu %d)", st.RoueSubPhase, st.RoueCurrentTeam, st.RoueLastEvent, seq1+2)
	}
}

// Non-régression : une case valeur, BANQUEROUTE et TOUR PERDU gardent leur effet v12.0 sur une roue qui contient des BONUS.
func TestRoueBonus308_OtherCasesKeepV120Behaviour(t *testing.T) {
	p := RoueWheelParams{MaxPoints: 100, Step: 10, BankruptCount: 1, LoseTurnCount: 1}
	cases := []struct {
		kind    string
		wantSub string
		event   string
		bank    int
	}{
		{RoueKindValue, RoueSubLetter, RoueEvLandValue, 500},
		{RoueKindBankrupt, RoueSubTurnEnd, RoueEvBankrupt, 0},
		{RoueKindLoseTurn, RoueSubTurnEnd, RoueEvLoseTurn, 500},
	}
	for _, c := range cases {
		t.Run(c.kind, func(t *testing.T) {
			idx := -1
			e := roueBonusEngine308(t, 2, RoueModeTantQueJeGagne, p, func(int) int { return idx })
			idx = kindIndexes308(e.GetState().RoueWheel.Segments, c.kind)[0]
			e.mu.Lock()
			e.state.RoueTeamBank["red"] = 500
			e.mu.Unlock()
			spinAndStop308(t, e)
			fireRoueTimer144(e)
			st := e.GetState()
			if st.RoueSubPhase != c.wantSub || st.RoueLastEvent.Kind != c.event || st.RoueTeamBank["red"] != c.bank {
				t.Fatalf("%s : %s / %s / banque %d", c.kind, st.RoueSubPhase, st.RoueLastEvent.Kind, st.RoueTeamBank["red"])
			}
		})
	}
}

// ---- notice BONUS --------------------------------------------------------------------------------------------

// Une notice BONUS (Team, 0 point) et aucune autre ; un seul OnRoueChange pour la fin du freinage (§15.4).
func TestRoueBonus308_NoticeBonusEmittedOnceWithoutCredit(t *testing.T) {
	s := bonusStopEngine308(t, RoueModeTantQueJeGagne)
	var notices []RoueNotice
	s.e.OnRoueNotice = func(n RoueNotice) { notices = append(notices, n) }
	fireRoueTimer144(s.e)
	if len(notices) != 1 {
		t.Fatalf("attendu 1 notice, got %+v", notices)
	}
	if n := notices[0]; n.Kind != RoueNoticeBonus || n.Team != "red" || n.Points != 0 {
		t.Fatalf("notice = %+v, attendu {BONUS red 0}", n)
	}
}

// Aucune notice BONUS en dehors de la case BONUS (valeur : aucune ; BANQUEROUTE : BAD).
func TestRoueBonus308_NoBonusNoticeOnOtherCases(t *testing.T) {
	idx := 0
	e := roueBonusEngine308(t, 1, RoueModeTantQueJeGagne, RoueWheelParams{MaxPoints: 100, Step: 10}, func(int) int { return idx })
	var notices []RoueNotice
	e.OnRoueNotice = func(n RoueNotice) { notices = append(notices, n) }
	spinAndStop308(t, e)
	fireRoueTimer144(e) // case 0 = 100
	for _, n := range notices {
		if n.Kind == RoueNoticeBonus {
			t.Fatalf("notice BONUS sur une case valeur : %+v", n)
		}
	}
}

// ---- état sérialisé ------------------------------------------------------------------------------------------

// Les cases BONUS se sérialisent avec KIND "bonus" et VALUE 0 (jamais omis) dans ROUE_WHEEL.SEGMENTS.
func TestRoueBonus308_SegmentJSON(t *testing.T) {
	e := roueBonusEngine308(t, 1, RoueModeTantQueJeGagne, RoueWheelParams{MaxPoints: 100, Step: 10}, nil)
	segs := e.GetState().RoueWheel.Segments
	i := kindIndexes308(segs, "bonus")
	if len(i) != 1 {
		t.Fatalf("attendu 1 case de KIND \"bonus\", got %d", len(i))
	}
	b, err := json.Marshal(segs[i[0]])
	if err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf(`{"ID":"s%d","KIND":"bonus","VALUE":0}`, i[0]); string(b) != want {
		t.Fatalf("JSON = %s, attendu %s", b, want)
	}
}
