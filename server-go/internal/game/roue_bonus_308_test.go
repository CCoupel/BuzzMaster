package game

// #308 (v12.1.0) réécrit pour v12.2.0 (#316/#317, contracts/roue.md §17) : un BONUS n'est plus une case de la roue mais une
// MARQUE posée sur une case à points (`RoueSegment.Bonus`) ; l'arrêt dessus ouvre un tour normal sur la valeur de la case
// (sous-phase LETTER), sans relance. RoueEvBonus = "BONUS" (VALUE = valeur de la case), RoueNoticeBonus = "BONUS".

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// roueBonusEngine308 monte une manche ROUE démarrée avec `bonus` cases BONUS et des paramètres de roue donnés
// (zéro valeur = roue par défaut 1000/50/2/1). Le tirage est figé : `draw` renvoie l'index de la case tirée.
func roueBonusEngine308(t *testing.T, bonus int, mode string, p RoueWheelParams, draw func(n int) int, teams ...string) *Engine {
	t.Helper()
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red", Color: []int{255, 0, 0}}, "blue": {Name: "blue", Color: []int{0, 0, 255}}})
	q := &Question{ID: "roue308", Question: "Roue de la Fortune", Type: QuestionTypeRoue,
		TypedContent: TypedContent{RouePhrase: "Toto tata", RoueMode: mode, RoueBonusCount: &bonus}}
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

// markIndexes308 : index des cases portant la marque BONUS.
func markIndexes308(segs []RoueSegment) []int {
	var out []int
	for i, s := range segs {
		if s.Bonus {
			out = append(out, i)
		}
	}
	return out
}

// ---- roue générée --------------------------------------------------------------------------------------------

// critical — avec 0 BONUS (champ absent ou 0), la roue de la manche est identique case pour case à celle de v12.0
// (même ordre, mêmes ID), pour plusieurs configurations (contrat §17.2, §17.7).
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
			want := BuildRoueWheel(np, nil)
			e := roueBonusEngine308(t, 0, RoueModeTantQueJeGagne, p, nil)
			got := e.GetState().RoueWheel.Segments
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("roue différente de v12.0 avec 0 BONUS\n got  %+v\n want %+v", got, want)
			}
			if n := len(markIndexes308(got)); n != 0 {
				t.Fatalf("%d marque(s) BONUS avec ROUE_BONUS_COUNT = 0", n)
			}
		})
	}
}

// Les BONUS sont des MARQUES : exactement `n` cases à points marquées, le nombre de cases ne change pas, rien n'est déplacé
// (valeurs, BANQUEROUTE, TOUR PERDU identiques à la roue sans BONUS), le maximum reste à l'index 0, ID séquentiels.
func TestRoueBonus308_ExactCountAndNothingDisplaced(t *testing.T) {
	base := DefaultRoueWheelParams()
	np, _, _ := NormalizeRoueWheelParams(base)
	v120 := BuildRoueWheel(np, nil)
	for n := 1; n <= 4; n++ {
		t.Run(fmt.Sprintf("bonus=%d", n), func(t *testing.T) {
			e := roueBonusEngine308(t, n, RoueModeTantQueJeGagne, base, nil)
			segs := e.GetState().RoueWheel.Segments
			marks := markIndexes308(segs)
			if len(marks) != n {
				t.Fatalf("cases marquées = %d, attendu %d", len(marks), n)
			}
			if len(segs) != len(v120) {
				t.Fatalf("la roue a %d cases, attendu %d (les BONUS n'ajoutent plus de case)", len(segs), len(v120))
			}
			for _, i := range marks {
				if segs[i].Kind != RoueKindValue || segs[i].Value == 0 {
					t.Errorf("la marque %s doit porter sur une case à points : %+v", segs[i].ID, segs[i])
				}
			}
			for i := range segs {
				a, b := segs[i], v120[i]
				a.Bonus = false
				if a != b {
					t.Errorf("case %d déplacée ou modifiée : %+v au lieu de %+v", i, segs[i], v120[i])
				}
			}
		})
	}
}

// §17.2 : une marque ne touche jamais une case spéciale (BANQUEROUTE, TOUR PERDU, autre BONUS), circulairement.
func TestRoueBonus308_NeverNextToASpecialCase(t *testing.T) {
	for _, p := range []RoueWheelParams{
		{MaxPoints: 100, Step: 10, BankruptCount: 0, LoseTurnCount: 0},
		{MaxPoints: 100, Step: 50, BankruptCount: 0, LoseTurnCount: 1},
		DefaultRoueWheelParams(),
	} {
		np, _, _ := NormalizeRoueWheelParams(p)
		for n := 1; n <= RoueMaxBonusFor(np); n++ {
			e := roueBonusEngine308(t, n, RoueModeTantQueJeGagne, p, nil)
			segs := e.GetState().RoueWheel.Segments
			if got := len(markIndexes308(segs)); got != n {
				t.Errorf("%+v bonus=%d : %d marques", p, n, got)
			}
			for _, i := range markIndexes308(segs) {
				for _, d := range []int{-1, 1} {
					nb := segs[(i+d+len(segs))%len(segs)]
					if nb.Kind != RoueKindValue || nb.Bonus {
						t.Errorf("%+v bonus=%d : la marque %d touche %+v", p, n, i, nb)
					}
				}
			}
		}
	}
}

// Limite : au START, un nombre de BONUS au-dessus du maximum permis est RÉDUIT, le départ n'est jamais refusé.
func TestRoueBonus308_ReducedAtStartToWhatTheWheelCanTake(t *testing.T) {
	p := RoueWheelParams{MaxPoints: 1000, Step: 100, BankruptCount: 3, LoseTurnCount: 2} // 14 cases : 1 BONUS au plus
	e := roueBonusEngine308(t, 4, RoueModeTantQueJeGagne, p, nil)
	st := e.GetState()
	if st.Phase != PhaseStarted || st.RoueSubPhase != RoueSubTurn {
		t.Fatalf("le départ ne doit jamais être refusé : phase %s / %s", st.Phase, st.RoueSubPhase)
	}
	if got := len(markIndexes308(st.RoueWheel.Segments)); got != 1 || len(st.RoueWheel.Segments) != 14 {
		t.Fatalf("attendu 1 marque (réduction de 4 à 1) pour 14 cases, got %d marque(s) / %d cases", got, len(st.RoueWheel.Segments))
	}
}

// Roue de 64 cases : les BONUS ne comptent plus dans la limite, ils se posent dans la place libre.
func TestRoueBonus308_BonusDoNotCountInto64Cases(t *testing.T) {
	p := RoueWheelParams{MaxPoints: 64, Step: 1}
	e := roueBonusEngine308(t, 3, RoueModeTantQueJeGagne, p, nil)
	segs := e.GetState().RoueWheel.Segments
	if len(segs) != 64 || len(markIndexes308(segs)) != 3 {
		t.Fatalf("attendu 64 cases dont 3 marquées, got %d cases dont %d marquées", len(segs), len(markIndexes308(segs)))
	}
	if e.GetState().RoueSubPhase != RoueSubTurn {
		t.Fatalf("départ refusé : %s", e.GetState().RoueSubPhase)
	}
}

// ---- arrêt sur une case marquée ------------------------------------------------------------------------------

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
	value    int
	seqPre   int
}

// bonusStopEngine308 : roue 100/10/0/0 + 1 BONUS (mode `bonusMode`, "" = global) ; le tirage tombe sur la case marquée.
func bonusStopEngine308(t *testing.T, mode string, teams ...string) bonusStop308 {
	return bonusStopEngineMode308(t, mode, "", teams...)
}

func bonusStopEngineMode308(t *testing.T, mode, bonusMode string, teams ...string) bonusStop308 {
	t.Helper()
	p := RoueWheelParams{MaxPoints: 100, Step: 10}
	idx := -1
	e := roueBonusEngine308(t, 1, mode, p, func(int) int { return idx }, teams...)
	if bonusMode != "" {
		e.mu.Lock()
		e.state.RoueBonusMode = bonusMode
		e.mu.Unlock()
	}
	idx = markIndexes308(e.GetState().RoueWheel.Segments)[0]
	// Cagnotte non nulle pour prouver qu'elle ne bouge pas.
	e.mu.Lock()
	e.state.RoueTeamBank["red"] = 500
	e.state.RoueTeamBank["blue"] = 300
	e.mu.Unlock()
	spinAndStop308(t, e)
	if got := e.GetState().RoueSubPhase; got != RoueSubStopping {
		t.Fatalf("attendu STOPPING, got %s", got)
	}
	return bonusStop308{e: e, bonusIdx: idx, value: e.GetState().RoueWheel.Segments[idx].Value, seqPre: e.GetState().RoueLastEvent.Seq}
}

// critical — l'arrêt sur une case marquée ne modifie ni cagnotte, ni gain du tour, ni lettres, ni score de partie ; la même
// équipe garde la main et passe à l'étape 4 (LETTER, sans relance) ; un seul événement BONUS portant la valeur (§17.4).
func TestRoueBonus308_StopOnBonusIsANormalTurnOnTheCaseValueWithoutReplay(t *testing.T) {
	for _, mode := range []string{RoueModeTantQueJeGagne, RoueModeChacunSonTour} {
		t.Run(mode, func(t *testing.T) {
			s := bonusStopEngine308(t, mode)
			fireRoueTimer144(s.e) // fin du freinage : l'effet de la case s'applique
			st := s.e.GetState()
			if st.RoueSubPhase != RoueSubLetter || st.RoueStep != 4 {
				t.Fatalf("attendu LETTER / étape 4 (pas de relance), got %s / %d", st.RoueSubPhase, st.RoueStep)
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
			if ev := st.RoueLastEvent; ev.Kind != RoueEvBonus || ev.Team != "red" || ev.Seq != s.seqPre+1 || ev.Value != s.value {
				t.Fatalf("événement = %+v, attendu BONUS / red / SEQ %d / VALUE %d (un seul événement, pas de LAND_VALUE)", ev, s.seqPre+1, s.value)
			}
		})
	}
}

// SOLO : l'unique équipe joue sa consonne aussi.
func TestRoueBonus308_StopOnBonusSolo(t *testing.T) {
	s := bonusStopEngine308(t, RoueModeSolo, "red")
	fireRoueTimer144(s.e)
	st := s.e.GetState()
	if st.RoueSubPhase != RoueSubLetter || st.RoueCurrentTeam != "red" || st.RoueLastEvent.Kind != RoueEvBonus {
		t.Fatalf("SOLO : %s / %s / %s", st.RoueSubPhase, st.RoueCurrentTeam, st.RoueLastEvent.Kind)
	}
}

// Pendant le freinage rien n'est encore appliqué ; après l'arrêt la consonne se joue AU MONTANT DE LA CASE (hit : gain
// = occurrences × valeur) ; ni relance (ROUE_SPIN refusé) ni fin de tour prématurée.
func TestRoueBonus308_AfterBonusTheConsonantIsPlayedAtTheCaseValue(t *testing.T) {
	s := bonusStopEngine308(t, RoueModeTantQueJeGagne)
	if got := s.e.GetState().RoueLastEvent.Kind; got == RoueEvBonus {
		t.Fatalf("l'événement BONUS ne doit pas précéder la fin du freinage")
	}
	fireRoueTimer144(s.e)
	if err := s.e.RoueSpin(); err == nil || err.(*RoueError).Code != RoueErrWrongSubPhase {
		t.Fatalf("relance après un BONUS : attendu WRONG_SUBPHASE, got %v", err)
	}
	if err := s.e.RoueNext(RoueHandSame); err == nil {
		t.Fatalf("ROUE_NEXT avant la lettre : refusé, got nil")
	}
	if err := s.e.RoueLetter("T"); err != nil { // « Toto tata » : 4 T
		t.Fatalf("ROUE_LETTER après un BONUS : %v", err)
	}
	st := s.e.GetState()
	if st.RoueSubPhase != RoueSubLit || st.RoueTurnGain != 4*s.value {
		t.Fatalf("après T : %s / gain %d, attendu LIT / %d", st.RoueSubPhase, st.RoueTurnGain, 4*s.value)
	}
}

// Après l'arrêt sur une case marquée le tour est en cours (étape 4) : on ne passe pas la main (ROUE_PASS : début de tour
// seulement), rien ne change.
func TestRoueBonus308_PassHandAfterBonusIsRefusedTurnInProgress(t *testing.T) {
	s := bonusStopEngine308(t, RoueModeTantQueJeGagne)
	fireRoueTimer144(s.e)
	err := s.e.RoucePass()
	if err == nil || err.(*RoueError).Code != RoueErrWrongSubPhase {
		t.Fatalf("ROUE_PASS après un BONUS : attendu WRONG_SUBPHASE, got %v", err)
	}
	st := s.e.GetState()
	if st.RoueCurrentTeam != "red" || st.RoueTeamBank["red"] != 500 || st.RoueSubPhase != RoueSubLetter {
		t.Fatalf("état modifié par un refus : %s / %v / %s", st.RoueCurrentTeam, st.RoueTeamBank, st.RoueSubPhase)
	}
}

// Permanent : la marque reste après la lettre ; un nouvel arrêt sur la même case rejoue le BONUS (SEQ +1 par événement).
func TestRoueBonus308_PermanentBonusTwiceInARow(t *testing.T) {
	s := bonusStopEngineMode308(t, RoueModeTantQueJeGagne, RoueBonusPermanent)
	fireRoueTimer144(s.e)
	seq1 := s.e.GetState().RoueLastEvent.Seq
	if err := s.e.RoueLetter("Z"); err != nil { // absente : fin du tour, la marque reste
		t.Fatal(err)
	}
	if !s.e.GetState().RoueWheel.Segments[s.bonusIdx].Bonus {
		t.Fatalf("PERMANENT : la marque doit rester")
	}
	if err := s.e.RoueNext(RoueHandSame); err != nil {
		t.Fatalf("fin de tour : %v", err)
	}
	spinAndStop308(t, s.e) // SPIN (+1) puis le freinage
	fireRoueTimer144(s.e)
	st := s.e.GetState()
	if st.RoueSubPhase != RoueSubLetter || st.RoueLastEvent.Kind != RoueEvBonus || st.RoueLastEvent.Seq <= seq1 {
		t.Fatalf("second BONUS : %s / %+v (SEQ > %d attendu)", st.RoueSubPhase, st.RoueLastEvent, seq1)
	}
}

// Non-régression : une case valeur, BANQUEROUTE et TOUR PERDU gardent leur effet v12.0 sur une roue qui contient des marques.
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
			for i, s := range e.GetState().RoueWheel.Segments { // une case à points NON marquée pour le cas « value »
				if s.Kind == c.kind && !s.Bonus {
					idx = i
					break
				}
			}
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

// Une notice BONUS (Team, 0 point) et aucune autre ; un seul OnRoueChange pour la fin du freinage (§17.4).
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

// Aucune notice BONUS en dehors d'une case marquée.
func TestRoueBonus308_NoBonusNoticeOnOtherCases(t *testing.T) {
	idx := 0
	e := roueBonusEngine308(t, 1, RoueModeTantQueJeGagne, RoueWheelParams{MaxPoints: 100, Step: 10}, func(int) int { return idx })
	if e.GetState().RoueWheel.Segments[0].Bonus {
		t.Skip("la marque est tombée sur l'index 0")
	}
	var notices []RoueNotice
	e.OnRoueNotice = func(n RoueNotice) { notices = append(notices, n) }
	spinAndStop308(t, e)
	fireRoueTimer144(e) // case 0 = 100, non marquée
	for _, n := range notices {
		if n.Kind == RoueNoticeBonus {
			t.Fatalf("notice BONUS sur une case non marquée : %+v", n)
		}
	}
}

// ---- état sérialisé ------------------------------------------------------------------------------------------

// Les cases se sérialisent avec BONUS (jamais omis) : KIND "value" + BONUS true pour une case marquée.
func TestRoueBonus308_SegmentJSON(t *testing.T) {
	e := roueBonusEngine308(t, 1, RoueModeTantQueJeGagne, RoueWheelParams{MaxPoints: 100, Step: 10}, nil)
	segs := e.GetState().RoueWheel.Segments
	i := markIndexes308(segs)
	if len(i) != 1 {
		t.Fatalf("attendu 1 case marquée, got %d", len(i))
	}
	b, err := json.Marshal(segs[i[0]])
	if err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf(`{"ID":"s%d","KIND":"value","VALUE":%d,"BONUS":true}`, i[0], segs[i[0]].Value); string(b) != want {
		t.Fatalf("JSON = %s, attendu %s", b, want)
	}
	plain, _ := json.Marshal(segs[(i[0]+2)%len(segs)])
	if !strings.Contains(string(plain), `"BONUS":false`) {
		t.Fatalf("BONUS ne doit jamais être omis : %s", plain)
	}
}
