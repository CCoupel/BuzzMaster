package game

import "testing"

func TestRoueBonusDev308_ZeroBonusIsV120Wheel(t *testing.T) {
	for _, p := range []RoueWheelParams{
		DefaultRoueWheelParams(),
		{MaxPoints: 1000, Step: 50, BankruptCount: 0, LoseTurnCount: 0},
		{MaxPoints: 500, Step: 100, BankruptCount: 3, LoseTurnCount: 2},
	} {
		a := BuildRoueWheel(p)
		p.BonusCount = 0
		b := BuildRoueWheel(p)
		if len(a) != len(b) {
			t.Fatal("size differs")
		}
		for i := range a {
			if a[i] != b[i] {
				t.Fatalf("case %d differs", i)
			}
		}
	}
}

func TestRoueBonusDev308_ExactCountAndSize(t *testing.T) {
	for n := 0; n <= RoueMaxBonusCount; n++ {
		p := DefaultRoueWheelParams()
		p.BonusCount = n
		segs := BuildRoueWheel(p)
		got := 0
		for _, s := range segs {
			if s.Kind == RoueKindBonus {
				got++
				if s.Value != 0 {
					t.Fatal("BONUS must have VALUE 0")
				}
			}
		}
		if got != n || len(segs) != 22+n {
			t.Fatalf("n=%d got %d bonus, %d cases", n, got, len(segs))
		}
	}
}

func TestRoueBonusDev308_Validate(t *testing.T) {
	p := DefaultRoueWheelParams()
	if ValidateRoueBonusCount(4, p) != nil || ValidateRoueBonusCount(5, p) == nil || ValidateRoueBonusCount(-1, p) == nil {
		t.Fatal("0..4 expected")
	}
	big := RoueWheelParams{MaxPoints: 3100, Step: 50, BankruptCount: 0, LoseTurnCount: 0} // 62 cases
	if ValidateRoueBonusCount(2, big) != nil || ValidateRoueBonusCount(3, big) == nil {
		t.Fatal("64-case limit expected")
	}
}

func TestRoueBonusDev308_StopOnBonus(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red", Color: []int{255, 0, 0}}, "blue": {Name: "blue", Color: []int{0, 0, 255}}})
	q := &Question{ID: "r", Question: "Roue", Type: QuestionTypeRoue,
		TypedContent: TypedContent{RouePhrase: "Toto tata", RoueBonusCount: 1}}
	e.Ready("r", q)
	if err := e.RoueSetTeams([]string{"red", "blue"}); err != nil {
		t.Fatal(err)
	}
	var notices []RoueNotice
	e.OnRoueNotice = func(n RoueNotice) { notices = append(notices, n) }
	e.StartImmediate(0)
	t.Cleanup(e.Stop)
	st := e.GetState()
	bi := -1
	for i, s := range st.RoueWheel.Segments {
		if s.Kind == RoueKindBonus {
			bi = i
		}
	}
	if bi < 0 {
		t.Fatal("no BONUS on the wheel")
	}
	e.SetRoueRand(func(int) int { return bi })
	if err := e.RoueSpin(); err != nil {
		t.Fatal(err)
	}
	fireRoueTimer144(e)
	if err := e.RoueStop(); err != nil {
		t.Fatal(err)
	}
	seqBefore := e.GetState().RoueLastEvent.Seq
	fireRoueTimer144(e)
	st = e.GetState()
	if st.RoueSubPhase != RoueSubTurn || st.RoueCurrentTeam != "red" || st.RoueTurnGain != 0 ||
		st.RoueLastEvent.Kind != RoueEvBonus || st.RoueLastEvent.Seq != seqBefore+1 || st.RoueWheel.Current != bi {
		t.Fatalf("bad state after BONUS: sub=%s team=%s ev=%+v", st.RoueSubPhase, st.RoueCurrentTeam, st.RoueLastEvent)
	}
	if st.RoueTeamBank["red"] != 0 || len(st.RoueLetters) != 0 {
		t.Fatal("BONUS must change neither pot nor letters")
	}
	if len(notices) != 1 || notices[0].Kind != RoueNoticeBonus || notices[0].Team != "red" {
		t.Fatalf("notices: %+v", notices)
	}
}

func TestRoueBonusDev308_ReducedAtStart(t *testing.T) {
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red", Color: []int{255, 0, 0}}})
	e.RoueWheelParamsFn = func() RoueWheelParams {
		return RoueWheelParams{MaxPoints: 3100, Step: 50}
	}
	e.Ready("r", &Question{ID: "r", Question: "Roue", Type: QuestionTypeRoue, TypedContent: TypedContent{RouePhrase: "Toto", RoueBonusCount: 4}})
	_ = e.RoueSetTeams([]string{"red"})
	e.StartImmediate(0)
	t.Cleanup(e.Stop)
	if n := len(e.GetState().RoueWheel.Segments); n > RoueMaxSegments {
		t.Fatalf("%d cases", n)
	}
}
