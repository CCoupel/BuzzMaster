// #144 (v12.0.0, LOT-1.2) — moteur de manche ROUE (contracts/roue.md §4 à §7) : les 7 étapes, la rotation des
// 3 modes (table §5.2), lettres, cases BANQUEROUTE / TOUR PERDU, crédit, arrêt (buzz / forcer / durée max),
// minuteries (annulation, pause), tirage à source fixée. Boîte noire sur l'API publique de l'Engine ; seules les
// durées sont raccourcies via le runtime interne (même paquet) pour ne pas attendre 1,5 s par freinage.
package game

import (
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---- harnais ----------------------------------------------------------------------------------

// Roue 100 / 10 / B=1 / P=1 : « 100 BANQ 90 10 80 PERDU 20 70 30 60 40 » (exemple du contrat §10.1).
const (
	wIdx100   = 0
	wIdxBanq  = 1
	wIdx90    = 2
	wIdxPerdu = 5
)

type roueRig struct {
	t       *testing.T
	e       *Engine
	idx     atomic.Int64 // case tirée par la source d'aléa injectée
	draws   atomic.Int64
	lastN   atomic.Int64
	changes atomic.Int64
	notices []RoueNotice
	// initialSpinUp / initialPeriod : SPIN_UP_MS et SPIN_PERIOD_MS tels que posés au START, avant raccourcissement.
	initialSpinUp, initialPeriod int
	nmu     sync.Mutex
}

func newRoueRig(t *testing.T, phrase, mode, gains string, teams ...string) *roueRig {
	t.Helper()
	r := &roueRig{t: t}
	e := NewEngine()
	r.e = e
	tm := map[string]*Team{}
	bm := map[string]*Bumper{}
	colors := [][]int{{255, 0, 0}, {0, 0, 255}, {0, 200, 0}}
	for i, name := range teams {
		tm[name] = &Team{Name: name, Color: colors[i%3], Ready: true}
		bm["b-"+name] = &Bumper{Name: "P-" + name, Team: name, Connected: true, Ready: true}
	}
	e.SetTeams(tm)
	e.SetBumpers(bm)
	e.SetRoueWheelParams(RoueWheelParams{MaxPoints: 100, Step: 10, BankruptCount: 1, LoseTurnCount: 1})
	e.SetRoueRand(func(n int) int { r.draws.Add(1); r.lastN.Store(int64(n)); return int(r.idx.Load()) })
	e.OnRoueChange = func() { r.changes.Add(1) }
	e.OnRoueNotice = func(n RoueNotice) { r.nmu.Lock(); r.notices = append(r.notices, n); r.nmu.Unlock() }
	q := &Question{ID: "rq", Question: "Roue", Type: QuestionTypeRoue, Category: "ANIMALS",
		TypedContent: TypedContent{RouePhrase: phrase, RoueMode: mode, RoueGainsRule: gains}}
	e.Ready("rq", q)
	if err := e.RoueSetTeams(teams); err != nil {
		t.Fatalf("RoueSetTeams(%v) : %v", teams, err)
	}
	e.StartImmediate(0)
	t.Cleanup(func() { e.Stop() })
	if st := e.GetState(); st.Phase != PhaseStarted || st.RoueSubPhase != RoueSubTurn {
		t.Fatalf("la manche ROUE doit démarrer en TURN, phase=%s sous-phase=%q", st.Phase, st.RoueSubPhase)
	}
	// Durées raccourcies (le contrat borne à 1,5 s / 200 ms / 3 s : on n'attend pas ça en test).
	e.mu.Lock()
	r.initialSpinUp, r.initialPeriod = e.state.RoueWheel.SpinUpMs, e.state.RoueWheel.SpinPeriodMs
	e.state.RoueWheel.SpinUpMs = 40 // montée raccourcie (contrat : 2 × SPIN_PERIOD_MS)
	e.roue.stopDuration = 15 * time.Millisecond
	e.state.RoueWheel.FlipIntervalMs = 5
	e.state.RoueWheel.SpinMaxMs = 60
	e.mu.Unlock()
	return r
}

func (r *roueRig) st() GameState { return r.e.GetState() }

func (r *roueRig) mustOK(err error) {
	r.t.Helper()
	if err != nil {
		r.t.Fatalf("action refusée : %v", err)
	}
}

func (r *roueRig) wantErr(err error, code string) {
	r.t.Helper()
	re, ok := err.(*RoueError)
	if !ok || re.Code != code {
		r.t.Fatalf("erreur attendue %s, got %v", code, err)
	}
}

func (r *roueRig) waitSub(sub string) {
	r.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if r.st().RoueSubPhase == sub {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	r.t.Fatalf("sous-phase %s non atteinte (actuelle : %q)", sub, r.st().RoueSubPhase)
}

// land lance la roue, force l'arrêt sur la case idx, attend la fin du freinage.
func (r *roueRig) land(idx int) {
	r.t.Helper()
	r.idx.Store(int64(idx))
	r.mustOK(r.e.RoueSpin())
	r.waitClimbOver()
	r.mustOK(r.e.RoueStop())
	r.waitSub(afterLanding(r, idx))
}

func afterLanding(r *roueRig, idx int) string {
	seg := r.st().RoueWheel.Segments[idx]
	if seg.Kind == RoueKindValue {
		return RoueSubLetter
	}
	return RoueSubTurnEnd
}

// playHit joue un tour complet jusqu'à ANSWER avec une consonne présente.
func (r *roueRig) playHit(idx int, letter string) {
	r.t.Helper()
	r.land(idx)
	r.mustOK(r.e.RoueLetter(letter))
	r.mustOK(r.e.RoueFlip())
	r.waitSub(RoueSubAnswer)
}

func (r *roueRig) teamScore(name string) int {
	r.e.mu.Lock()
	defer r.e.mu.Unlock()
	return r.e.data.Teams[name].Score
}

const roueTestPhrase = "BONJOUR LE MONDE" // consonnes B N J R L M D ; N ×2, O ×3

// ---- démarrage et état initial ---------------------------------------------------------------------

func TestRoue144_Start_InitialState(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeChacunSonTour, "", "red", "blue")
	s := r.st()
	if s.RoueStep != 1 || s.RoueCurrentTeam != "red" {
		t.Errorf("étape/main = %d/%q, attendu 1/red", s.RoueStep, s.RoueCurrentTeam)
	}
	if s.RoueTeamBank["red"] != 0 || s.RoueTeamBank["blue"] != 0 || len(s.RoueTeamBank) != 2 {
		t.Errorf("cagnottes à 0 pour chaque équipe participante : %v", s.RoueTeamBank)
	}
	if s.RoueWheel.Current != -1 || s.RoueWheel.Stop != nil || len(s.RoueWheel.Segments) != 11 {
		t.Errorf("roue : Current=%d Stop=%v segments=%d (attendu -1, nil, 11)", s.RoueWheel.Current, s.RoueWheel.Stop, len(s.RoueWheel.Segments))
	}
	if !s.RoueConsonantsLeft {
		t.Errorf("ROUE_CONSONANTS_LEFT doit valoir true au départ")
	}
	if !s.RoueShowLetters {
		t.Errorf("ROUE_SHOW_LETTERS : true par défaut")
	}
	if len(s.RoueBoard.Rows) == 0 || s.RoueBoard.Cols != 14 {
		t.Errorf("plateau : %+v", s.RoueBoard)
	}
	if len(s.RoueLetters) != 0 || s.RoueTurnGain != 0 || s.RoueWinner != "" || len(s.RoueAwards) != 0 {
		t.Errorf("lettres/gain/gagnant/crédits vides au départ : %+v", s)
	}
}

// Contrat §9 : à chaque instant, une lettre non révélée n'est que `_` (ou `*` si éclairée), jamais la lettre.
func TestRoue144_Board_NeverLeaksUnrevealedLetters(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	check := func(step string) {
		t.Helper()
		for _, row := range r.st().RoueBoard.Rows {
			for _, ch := range row {
				if ch >= 'A' && ch <= 'Z' && !strings.ContainsRune("N", ch) {
					t.Fatalf("%s : lettre non révélée %q dans ROUE_BOARD %v", step, ch, r.st().RoueBoard.Rows)
				}
			}
		}
	}
	check("TURN")
	r.land(wIdx100)
	check("LETTER")
	r.mustOK(r.e.RoueLetter("N"))
	for _, row := range r.st().RoueBoard.Rows {
		if strings.ContainsRune(row, 'N') {
			t.Fatalf("LIT : la lettre éclairée doit être `*`, pas N : %v", r.st().RoueBoard.Rows)
		}
	}
	if got := strings.Count(strings.Join(r.st().RoueBoard.Rows, ""), "*"); got != 2 {
		t.Errorf("LIT : 2 cases N éclairées attendues, got %d", got)
	}
	r.mustOK(r.e.RoueFlip())
	check("REVEALING") // N est désormais révélée (autorisée)
	r.waitSub(RoueSubAnswer)
	check("ANSWER")
}

// ---- les 7 étapes -----------------------------------------------------------------------------------

func TestRoue144_FullTurn_SevenSteps(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeTantQueJeGagne, "", "red", "blue")
	r.idx.Store(wIdx100)

	r.mustOK(r.e.RoueSpin())
	if s := r.st(); s.RoueSubPhase != RoueSubSpinning || s.RoueStep != 2 || s.RoueLastEvent.Kind != RoueEvSpin {
		t.Fatalf("après ROUE_SPIN : %q step %d event %s", s.RoueSubPhase, s.RoueStep, s.RoueLastEvent.Kind)
	}
	r.waitClimbOver()
	r.mustOK(r.e.RoueStop())
	s := r.st()
	if s.RoueSubPhase != RoueSubStopping || s.RoueStep != 3 || s.RoueWheel.Stop == nil || s.RoueWheel.Stop.To != wIdx100 || s.RoueWheel.Stop.DurationMs <= 0 {
		t.Fatalf("STOPPING : %+v stop=%+v", s.RoueSubPhase, s.RoueWheel.Stop)
	}
	// l'effet de la case n'est appliqué qu'à la FIN du freinage (§6)
	if s.RoueWheel.Current != -1 {
		t.Errorf("Current doit rester -1 pendant le freinage, got %d", s.RoueWheel.Current)
	}
	r.waitSub(RoueSubLetter)
	s = r.st()
	if s.RoueStep != 4 || s.RoueWheel.Current != wIdx100 || s.RoueWheel.Stop != nil || s.RoueLastEvent.Kind != RoueEvLandValue || s.RoueLastEvent.Value != 100 {
		t.Fatalf("LETTER : step %d current %d stop %v event %+v", s.RoueStep, s.RoueWheel.Current, s.RoueWheel.Stop, s.RoueLastEvent)
	}

	r.mustOK(r.e.RoueLetter("N"))
	s = r.st()
	if s.RoueSubPhase != RoueSubLit || s.RoueStep != 5 || s.RoueTurnGain != 200 {
		t.Fatalf("LIT : %q step %d gain %d (attendu 2 × 100 = 200)", s.RoueSubPhase, s.RoueStep, s.RoueTurnGain)
	}
	if ev := s.RoueLastEvent; ev.Kind != RoueEvLetterHit || ev.Letter != "N" || ev.Count != 2 || ev.Value != 100 || ev.Gain != 200 {
		t.Errorf("LETTER_HIT : %+v", ev)
	}
	if len(s.RoueLetters) != 1 || s.RoueLetters[0] != (RoueLetter{Letter: "N", Team: "red", Hit: true}) {
		t.Errorf("ROUE_LETTERS : %+v", s.RoueLetters)
	}
	if s.RoueTeamBank["red"] != 0 {
		t.Errorf("le gain du tour ne rejoint la cagnotte qu'à l'étape 7, bank=%d", s.RoueTeamBank["red"])
	}

	r.mustOK(r.e.RoueFlip())
	if s := r.st(); s.RoueSubPhase != RoueSubRevealing || s.RoueLastEvent.Kind != RoueEvFlip || s.RoueLastEvent.Count != 2 {
		t.Fatalf("REVEALING : %q event %+v", s.RoueSubPhase, s.RoueLastEvent)
	}
	r.waitSub(RoueSubAnswer)
	s = r.st()
	if s.RoueStep != 6 || s.RoueNextTeam != "blue" || s.RoueNextDefault != RoueHandSame {
		t.Fatalf("ANSWER : step %d next %q conseillé %q (TANT_QUE_JE_GAGNE + lettre trouvée → SAME)", s.RoueStep, s.RoueNextTeam, s.RoueNextDefault)
	}

	// pas de réponse validée : « Tour suivant » (même équipe) — le gain du tour rejoint la cagnotte
	r.mustOK(r.e.RoueNext(RoueHandSame))
	s = r.st()
	if s.RoueSubPhase != RoueSubTurn || s.RoueStep != 1 || s.RoueCurrentTeam != "red" || s.RoueTeamBank["red"] != 200 || s.RoueTurnGain != 0 {
		t.Fatalf("après ROUE_NEXT SAME : %q main=%s bank=%d gain=%d", s.RoueSubPhase, s.RoueCurrentTeam, s.RoueTeamBank["red"], s.RoueTurnGain)
	}
	if s.RoueLastEvent.Kind != RoueEvTurnEnd || s.RoueLastEvent.Gain != 200 {
		t.Errorf("événement TURN_END attendu avec le gain : %+v", s.RoueLastEvent)
	}
	// aucun score modifié pendant la manche (§7)
	if r.teamScore("red") != 0 || r.teamScore("blue") != 0 {
		t.Errorf("aucun score ne change pendant la manche : red=%d blue=%d", r.teamScore("red"), r.teamScore("blue"))
	}
}

// ---- lettres ----------------------------------------------------------------------------------------

func TestRoue144_Letter_AbsentEndsTheTurn(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeTantQueJeGagne, "", "red", "blue")
	r.land(wIdx100)
	seq := r.st().RoueLastEvent.Seq
	r.mustOK(r.e.RoueLetter("Z"))
	s := r.st()
	if s.RoueLastEvent.Seq != seq+1 {
		t.Errorf("un événement par action : SEQ %d → %d (attendu +1, pas de TURN_END en plus)", seq, s.RoueLastEvent.Seq)
	}
	if s.RoueSubPhase != RoueSubTurnEnd || s.RoueStep != 7 || s.RoueLastEvent.Kind != RoueEvLetterMiss || s.RoueLastEvent.Letter != "Z" {
		t.Fatalf("lettre absente : %q step %d event %+v", s.RoueSubPhase, s.RoueStep, s.RoueLastEvent)
	}
	if len(s.RoueLetters) != 1 || s.RoueLetters[0].Hit {
		t.Errorf("la lettre absente est enregistrée avec HIT=false : %+v", s.RoueLetters)
	}
	if s.RoueTurnGain != 0 || s.RoueTeamBank["red"] != 0 {
		t.Errorf("aucun gain sur une lettre absente")
	}
	r.nmu.Lock()
	defer r.nmu.Unlock()
	if len(r.notices) == 0 || r.notices[len(r.notices)-1].Kind != RoueNoticeBad {
		t.Errorf("une lettre absente déclenche la notice « mauvaise réponse » : %+v", r.notices)
	}
}

func TestRoue144_Letter_RepeatedIsTreatedAsAbsent(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.playHit(wIdx100, "N")
	r.mustOK(r.e.RoueNext(RoueHandSame))
	r.land(wIdx90)
	seq := r.st().RoueLastEvent.Seq
	r.mustOK(r.e.RoueLetter("N")) // déjà proposée
	s := r.st()
	if s.RoueLastEvent.Seq != seq+1 {
		t.Errorf("un événement par action : SEQ %d → %d", seq, s.RoueLastEvent.Seq)
	}
	if s.RoueSubPhase != RoueSubTurnEnd || s.RoueLastEvent.Kind != RoueEvLetterRepeat {
		t.Fatalf("lettre répétée : %q %+v", s.RoueSubPhase, s.RoueLastEvent)
	}
	if len(s.RoueLetters) != 1 {
		t.Errorf("une lettre répétée ne crée pas de nouvelle entrée : %+v", s.RoueLetters)
	}
}

func TestRoue144_Letter_RefusalsLeaveStateUntouched(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.land(wIdx100)
	before := r.st()
	for _, v := range []string{"A", "E", "I", "O", "U", "Y", "a", "é"} {
		r.wantErr(r.e.RoueLetter(v), RoueErrVowelNotAvailable)
	}
	for _, v := range []string{"", "1", "!", " ", "AB", "-"} {
		r.wantErr(r.e.RoueLetter(v), RoueErrNotALetter)
	}
	after := r.st()
	if after.RoueSubPhase != RoueSubLetter || len(after.RoueLetters) != 0 || after.RoueLastEvent != before.RoueLastEvent {
		t.Errorf("un refus ne change pas l'état ni n'inflige de pénalité : %q %+v", after.RoueSubPhase, after.RoueLetters)
	}
}

func TestRoue144_Letter_NormalizedLowercaseAndAccents(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.land(wIdx100)
	r.mustOK(r.e.RoueLetter(" n "))
	if r.st().RoueLastEvent.Letter != "N" {
		t.Errorf("la lettre est normalisée en majuscule : %+v", r.st().RoueLastEvent)
	}
}

// ---- cases spéciales --------------------------------------------------------------------------------

func TestRoue144_Bankrupt_ZeroesThePotAndEndsTheTurn(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeChacunSonTour, "", "red", "blue")
	r.playHit(wIdx100, "N")
	r.mustOK(r.e.RoueNext(RoueHandSame)) // red encaisse 200, garde la main
	if r.st().RoueTeamBank["red"] != 200 {
		t.Fatalf("préparation : cagnotte 200 attendue, got %d", r.st().RoueTeamBank["red"])
	}
	r.idx.Store(wIdxBanq)
	r.mustOK(r.e.RoueSpin())
	r.waitClimbOver()
	r.mustOK(r.e.RoueStop())
	seq := r.st().RoueLastEvent.Seq // SPIN posé ; le freinage n'a pas encore produit d'événement
	r.waitSub(RoueSubTurnEnd)
	s := r.st()
	if s.RoueLastEvent.Seq != seq+1 {
		t.Errorf("fin du freinage sur BANQUEROUTE : un seul événement (SEQ +1), %d → %d", seq, s.RoueLastEvent.Seq)
	}
	if s.RoueSubPhase != RoueSubTurnEnd || s.RoueTeamBank["red"] != 0 || s.RoueLastEvent.Kind != RoueEvBankrupt {
		t.Fatalf("BANQUEROUTE : %q bank=%d event=%s", s.RoueSubPhase, s.RoueTeamBank["red"], s.RoueLastEvent.Kind)
	}
	if s.RoueTeamBank["blue"] != 0 {
		t.Errorf("seule la cagnotte de l'équipe qui a la main est vidée")
	}
	r.nmu.Lock()
	defer r.nmu.Unlock()
	bad := 0
	for _, n := range r.notices {
		if n.Kind == RoueNoticeBad {
			bad++
		}
	}
	if bad == 0 {
		t.Errorf("la BANQUEROUTE déclenche la notice « mauvaise réponse »")
	}
}

func TestRoue144_LoseTurn_KeepsThePotAndEndsTheTurn(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeChacunSonTour, "", "red", "blue")
	r.playHit(wIdx100, "N")
	r.mustOK(r.e.RoueNext(RoueHandSame))
	r.idx.Store(wIdxPerdu)
	r.mustOK(r.e.RoueSpin())
	r.waitClimbOver()
	r.mustOK(r.e.RoueStop())
	seq := r.st().RoueLastEvent.Seq
	r.waitSub(RoueSubTurnEnd)
	s := r.st()
	if s.RoueLastEvent.Seq != seq+1 {
		t.Errorf("fin du freinage sur TOUR PERDU : un seul événement (SEQ +1), %d → %d", seq, s.RoueLastEvent.Seq)
	}
	if s.RoueSubPhase != RoueSubTurnEnd || s.RoueTeamBank["red"] != 200 || s.RoueLastEvent.Kind != RoueEvLoseTurn {
		t.Fatalf("TOUR PERDU : %q bank=%d event=%s", s.RoueSubPhase, s.RoueTeamBank["red"], s.RoueLastEvent.Kind)
	}
}

// ---- rotation : table §5.2 ----------------------------------------------------------------------------

func TestRoue144_Advice_Table(t *testing.T) {
	// mode × (lettre trouvée | pas de lettre trouvée) → bouton conseillé
	type row struct {
		mode  string
		teams []string
		found string
		miss  string
	}
	cases := []row{
		{RoueModeSolo, []string{"red"}, RoueHandSame, RoueHandSame},
		{RoueModeChacunSonTour, []string{"red", "blue"}, RoueHandNext, RoueHandNext},
		{RoueModeTantQueJeGagne, []string{"red", "blue"}, RoueHandSame, RoueHandNext},
	}
	for _, c := range cases {
		t.Run(c.mode+"/trouvée", func(t *testing.T) {
			r := newRoueRig(t, roueTestPhrase, c.mode, "", c.teams...)
			r.playHit(wIdx100, "N")
			if got := r.st().RoueNextDefault; got != c.found {
				t.Errorf("lettre trouvée : conseillé %q, attendu %q", got, c.found)
			}
		})
		t.Run(c.mode+"/lettre absente", func(t *testing.T) {
			r := newRoueRig(t, roueTestPhrase, c.mode, "", c.teams...)
			r.land(wIdx100)
			r.mustOK(r.e.RoueLetter("Z"))
			if got := r.st().RoueNextDefault; got != c.miss {
				t.Errorf("lettre absente : conseillé %q, attendu %q", got, c.miss)
			}
		})
		t.Run(c.mode+"/banqueroute", func(t *testing.T) {
			r := newRoueRig(t, roueTestPhrase, c.mode, "", c.teams...)
			r.land(wIdxBanq)
			if got := r.st().RoueNextDefault; got != c.miss {
				t.Errorf("banqueroute : conseillé %q, attendu %q", got, c.miss)
			}
		})
		t.Run(c.mode+"/tour perdu", func(t *testing.T) {
			r := newRoueRig(t, roueTestPhrase, c.mode, "", c.teams...)
			r.land(wIdxPerdu)
			if got := r.st().RoueNextDefault; got != c.miss {
				t.Errorf("tour perdu : conseillé %q, attendu %q", got, c.miss)
			}
		})
	}
}

// Principe : la réponse est un bonus — une réponse fausse ou absente ne change pas le bouton conseillé
// (le serveur ne distingue pas les deux : ROUE_NEXT depuis ANSWER).
func TestRoue144_Advice_UnchangedByAWrongAnswer(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeTantQueJeGagne, "", "red", "blue")
	r.playHit(wIdx100, "N")
	if r.st().RoueNextDefault != RoueHandSame {
		t.Fatalf("préparation")
	}
	r.mustOK(r.e.RoueNext(RoueHandSame)) // réponse fausse / absente
	if r.st().RoueCurrentTeam != "red" {
		t.Errorf("le tour suivant reste à red")
	}
}

func TestRoue144_Next_SameAndNextHands(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeChacunSonTour, "", "red", "blue", "green")
	r.land(wIdx100)
	r.mustOK(r.e.RoueLetter("Z"))
	if r.st().RoueNextTeam != "blue" {
		t.Fatalf("équipe suivante = %q, attendu blue", r.st().RoueNextTeam)
	}
	r.mustOK(r.e.RoueNext(RoueHandNext))
	if s := r.st(); s.RoueCurrentTeam != "blue" || s.RoueSubPhase != RoueSubTurn || s.RoueNextTeam != "" || s.RoueNextDefault != "" {
		t.Fatalf("NEXT : main=%s sub=%s next=%q/%q", s.RoueCurrentTeam, s.RoueSubPhase, s.RoueNextTeam, s.RoueNextDefault)
	}
	// le bouton non conseillé reste utilisable : SAME alors que NEXT était conseillé
	r.land(wIdx100)
	r.mustOK(r.e.RoueLetter("Z"))
	r.mustOK(r.e.RoueNext(RoueHandSame))
	if r.st().RoueCurrentTeam != "blue" {
		t.Errorf("SAME garde la main : %s", r.st().RoueCurrentTeam)
	}
	// rotation circulaire : blue → green → red
	r.land(wIdx100)
	r.mustOK(r.e.RoueLetter("Z"))
	r.mustOK(r.e.RoueNext(RoueHandNext))
	r.land(wIdx100)
	r.mustOK(r.e.RoueLetter("Q"))
	r.mustOK(r.e.RoueNext(RoueHandNext))
	if r.st().RoueCurrentTeam != "red" {
		t.Errorf("la rotation revient à red, got %s", r.st().RoueCurrentTeam)
	}
}

func TestRoue144_Next_RefusalsAndSolo(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.land(wIdx100)
	r.mustOK(r.e.RoueLetter("Z"))
	r.wantErr(r.e.RoueNext(RoueHandNext), RoueErrNotAllowed) // « Équipe suivante » refusé en SOLO
	r.wantErr(r.e.RoueNext("PERHAPS"), RoueErrNotAllowed)    // valeur inconnue
	if r.st().RoueSubPhase != RoueSubTurnEnd {
		t.Errorf("un refus ne change pas l'état")
	}
	if r.st().RoueNextTeam != "" {
		t.Errorf("ROUE_NEXT_TEAM vide en SOLO, got %q", r.st().RoueNextTeam)
	}
	r.mustOK(r.e.RoueNext(RoueHandSame))
}

// Étape 7 franchie depuis ANSWER : le gain du tour est acquis quel que soit le bouton.
func TestRoue144_Next_FromAnswerBanksTheGainWhateverTheButton(t *testing.T) {
	for _, hand := range []string{RoueHandSame, RoueHandNext} {
		t.Run(hand, func(t *testing.T) {
			r := newRoueRig(t, roueTestPhrase, RoueModeTantQueJeGagne, "", "red", "blue")
			r.playHit(wIdx100, "N")
			r.mustOK(r.e.RoueNext(hand))
			if r.st().RoueTeamBank["red"] != 200 {
				t.Errorf("le gain d'une lettre trouvée reste acquis : bank=%d", r.st().RoueTeamBank["red"])
			}
		})
	}
}

func TestRoue144_Pass(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeChacunSonTour, "", "red", "blue")
	r.playHit(wIdx100, "N")
	r.mustOK(r.e.RoueNext(RoueHandSame)) // red : cagnotte 200
	r.mustOK(r.e.RoucePass())
	s := r.st()
	if s.RoueCurrentTeam != "blue" || s.RoueSubPhase != RoueSubTurn || s.RoueTeamBank["red"] != 200 || s.RoueLastEvent.Kind != RoueEvPass {
		t.Fatalf("PASS : main=%s sub=%s bank=%d event=%s", s.RoueCurrentTeam, s.RoueSubPhase, s.RoueTeamBank["red"], s.RoueLastEvent.Kind)
	}
	// seulement au début du tour
	r.land(wIdx100)
	r.wantErr(r.e.RoucePass(), RoueErrWrongSubPhase)

	solo := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	solo.wantErr(solo.e.RoucePass(), RoueErrNotAllowed)
}

// Règle « un événement par action » (arbitrage roue.md §4/§5.1) : TURN_END n'est émis que par ROUE_NEXT depuis
// ANSWER (ou depuis TURN sans consonne) ; ROUE_NEXT depuis TURN_END n'émet rien.
func TestRoue144_Events_OnePerAction(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeChacunSonTour, "", "red", "blue")
	r.land(wIdx100)
	r.mustOK(r.e.RoueLetter("Z")) // → TURN_END (étape 7), événement LETTER_MISS
	seq := r.st().RoueLastEvent.Seq
	r.mustOK(r.e.RoueNext(RoueHandNext))
	if got := r.st().RoueLastEvent.Seq; got != seq {
		t.Errorf("ROUE_NEXT depuis TURN_END n'émet aucun événement : SEQ %d → %d", seq, got)
	}
	// depuis ANSWER : l'événement TURN_END porte le gain du tour
	r.playHit(wIdx100, "N")
	seq = r.st().RoueLastEvent.Seq
	r.mustOK(r.e.RoueNext(RoueHandSame))
	ev := r.st().RoueLastEvent
	if ev.Seq != seq+1 || ev.Kind != RoueEvTurnEnd || ev.Gain != 200 {
		t.Errorf("ROUE_NEXT depuis ANSWER émet TURN_END (gain 200, SEQ +1) : %+v (seq avant %d)", ev, seq)
	}
}

func TestRoue144_Events_SeqStrictlyIncreasesByOnePerAction(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeTantQueJeGagne, "", "red", "blue")
	prev := r.st().RoueLastEvent.Seq
	check := func(what string) {
		t.Helper()
		got := r.st().RoueLastEvent.Seq
		if got != prev+1 {
			t.Fatalf("%s : SEQ %d → %d (attendu +1)", what, prev, got)
		}
		prev = got
	}
	r.idx.Store(wIdx100)
	r.mustOK(r.e.RoueSpin())
	check("ROUE_SPIN")
	r.waitClimbOver()
	r.mustOK(r.e.RoueStop())
	r.waitSub(RoueSubLetter)
	check("fin du freinage (LAND_VALUE)")
	r.mustOK(r.e.RoueLetter("N"))
	check("ROUE_LETTER (LETTER_HIT)")
	r.mustOK(r.e.RoueFlip())
	check("ROUE_FLIP")
	r.waitSub(RoueSubAnswer) // la fin du retournement n'émet rien
	if r.st().RoueLastEvent.Seq != prev {
		t.Errorf("la fin du retournement n'émet pas d'événement : %d → %d", prev, r.st().RoueLastEvent.Seq)
	}
	r.mustOK(r.e.RoueNext(RoueHandSame))
	check("ROUE_NEXT depuis ANSWER (TURN_END)")
}

func TestRoue144_Events_NextFromTurnWithoutConsonantEmitsTurnEnd(t *testing.T) {
	r := newRoueRig(t, "OUI", RoueModeChacunSonTour, "", "red", "blue")
	seq := r.st().RoueLastEvent.Seq
	r.mustOK(r.e.RoueNext(RoueHandNext))
	if ev := r.st().RoueLastEvent; ev.Seq != seq+1 || ev.Kind != RoueEvTurnEnd {
		t.Errorf("ROUE_NEXT depuis TURN sans consonne émet TURN_END : %+v", ev)
	}
}

// ---- plus de consonne -----------------------------------------------------------------------------------

func TestRoue144_NoConsonantLeft_NoMoreSpin(t *testing.T) {
	r := newRoueRig(t, "OUI", RoueModeChacunSonTour, "", "red", "blue")
	if r.st().RoueConsonantsLeft {
		t.Fatalf("« OUI » n'a aucune consonne à trouver")
	}
	r.wantErr(r.e.RoueSpin(), RoueErrWrongSubPhase)
	// l'équipe ne peut que proposer : ROUE_NEXT = pas de réponse validée, la main passe, on reste en TURN
	r.mustOK(r.e.RoueNext(RoueHandNext))
	if s := r.st(); s.RoueSubPhase != RoueSubTurn || s.RoueCurrentTeam != "blue" {
		t.Fatalf("après ROUE_NEXT sans consonne : %s main=%s", s.RoueSubPhase, s.RoueCurrentTeam)
	}
	// ROUE_ANSWER depuis TURN : réponse validée → SOLVED
	r.mustOK(r.e.RoueAnswer())
	if s := r.st(); s.RoueSubPhase != RoueSubSolved || s.RoueWinner != "blue" {
		t.Fatalf("SOLVED attendu pour blue : %s %q", s.RoueSubPhase, s.RoueWinner)
	}
}

func TestRoue144_LastConsonantFound_ConsonantsLeftBecomesFalse(t *testing.T) {
	r := newRoueRig(t, "AMI", RoueModeSolo, "", "red")
	if !r.st().RoueConsonantsLeft {
		t.Fatalf("M reste à proposer")
	}
	r.playHit(wIdx100, "M")
	r.mustOK(r.e.RoueNext(RoueHandSame))
	if r.st().RoueConsonantsLeft {
		t.Errorf("plus aucune consonne cachée")
	}
	r.wantErr(r.e.RoueSpin(), RoueErrWrongSubPhase)
}

// ---- mauvaises sous-phases -------------------------------------------------------------------------------

func TestRoue144_ActionsRefusedInWrongSubPhase(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	// TURN : seuls SPIN / PASS (hors SOLO) / REVEAL sont valables
	r.wantErr(r.e.RoueStop(), RoueErrWrongSubPhase)
	r.wantErr(r.e.RoueLetter("N"), RoueErrWrongSubPhase)
	r.wantErr(r.e.RoueFlip(), RoueErrWrongSubPhase)
	r.wantErr(r.e.RoueAnswer(), RoueErrWrongSubPhase) // des consonnes restent
	r.wantErr(r.e.RoueNext(RoueHandSame), RoueErrWrongSubPhase)
	r.mustOK(r.e.RoueSpin())
	// SPINNING : SPIN refusé, LETTER refusé
	r.wantErr(r.e.RoueSpin(), RoueErrWrongSubPhase)
	r.wantErr(r.e.RoueLetter("N"), RoueErrWrongSubPhase)
	r.waitClimbOver()
	r.mustOK(r.e.RoueStop())
	// STOPPING : un second ROUE_STOP est refusé, sans nouveau tirage
	d := r.draws.Load()
	r.wantErr(r.e.RoueStop(), RoueErrWrongSubPhase)
	if r.draws.Load() != d {
		t.Errorf("aucun nouveau tirage pendant le freinage")
	}
}

func TestRoue144_ActionsRefusedOutsideARoundOrOnAnotherType(t *testing.T) {
	e := NewEngine()
	if err := e.RoueSpin(); err == nil {
		t.Errorf("ROUE_SPIN hors manche doit être refusé")
	} else if re, ok := err.(*RoueError); !ok || re.Code != RoueErrNotAllowed {
		t.Errorf("NOT_ALLOWED attendu, got %v", err)
	}
	e.SetTeams(map[string]*Team{"red": {Name: "red", Ready: true}})
	e.Ready("sp", &Question{ID: "sp", Question: "Q", Answer: "A", Type: QuestionTypeSpeedy, Points: "10", Time: "30"})
	e.StartImmediate(0)
	defer e.Stop()
	if err := e.RoueReveal(); err == nil {
		t.Errorf("ROUE_REVEAL sur une question SPEEDY doit être refusé")
	}
}

// ---- arrêt de la roue : buzz / forcer / durée maximale ------------------------------------------------------

func TestRoue144_Stop_BuzzOfTheTeamWithTheHand(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeChacunSonTour, "", "red", "blue")
	r.idx.Store(wIdx90)
	r.mustOK(r.e.RoueSpin())

	// appui d'une AUTRE équipe : ignoré, sans effet
	if r.e.ProcessButtonPress("b-blue", time.Now().UnixMicro(), "A") {
		t.Errorf("l'appui d'une autre équipe n'est pas un buzz accepté")
	}
	if r.st().RoueSubPhase != RoueSubSpinning || r.draws.Load() != 0 {
		t.Fatalf("appui d'une autre équipe ignoré : sous-phase=%s tirages=%d", r.st().RoueSubPhase, r.draws.Load())
	}
	// appui de l'équipe qui a la main — acceptée seulement à pleine vitesse (§5.1, montée d'un tour)
	r.waitClimbOver()
	r.e.ProcessButtonPress("b-red", time.Now().UnixMicro(), "A")
	s := r.st()
	if s.RoueSubPhase != RoueSubStopping || s.RoueWheel.Stop == nil || s.RoueWheel.Stop.To != wIdx90 {
		t.Fatalf("le buzz doit arrêter la roue sur la case tirée : %s %+v", s.RoueSubPhase, s.RoueWheel.Stop)
	}
	// second appui pendant le freinage : ignoré (pas de nouveau tirage)
	r.idx.Store(wIdx100)
	r.e.ProcessButtonPress("b-red", time.Now().UnixMicro(), "A")
	if r.st().RoueWheel.Stop == nil || r.st().RoueWheel.Stop.To != wIdx90 || r.draws.Load() != 1 {
		t.Errorf("second appui ignoré : Stop=%+v tirages=%d", r.st().RoueWheel.Stop, r.draws.Load())
	}
	r.waitSub(RoueSubLetter)
	if r.st().RoueWheel.Current != wIdx90 {
		t.Errorf("CURRENT = %d, attendu %d", r.st().RoueWheel.Current, wIdx90)
	}
}

func TestRoue144_Buzz_OutsideSpinningIsIgnored(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.e.ProcessButtonPress("b-red", time.Now().UnixMicro(), "A") // sous-phase TURN
	if r.st().RoueSubPhase != RoueSubTurn || r.draws.Load() != 0 {
		t.Errorf("un appui hors SPINNING est ignoré")
	}
	r.land(wIdx100)
	r.e.ProcessButtonPress("b-red", time.Now().UnixMicro(), "A") // sous-phase LETTER
	if r.st().RoueSubPhase != RoueSubLetter {
		t.Errorf("un appui en LETTER est ignoré")
	}
}

// En mode ROUE un appui ne pose jamais le statut PAUSE d'un buzzer (§5.1).
func TestRoue144_Buzz_NeverSetsPauseStatus(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.mustOK(r.e.RoueSpin())
	r.e.ProcessButtonPress("b-red", time.Now().UnixMicro(), "A")
	for id, b := range r.e.GetTeamsAndBumpersSnapshot().Bumpers {
		if b.Status == "PAUSE" {
			t.Errorf("le buzzer %s ne doit pas passer en PAUSE", id)
		}
	}
}

func TestRoue144_Stop_MaxSpinDurationStopsTheWheelByItself(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.idx.Store(wIdx90)
	r.mustOK(r.e.RoueSpin()) // SpinMaxMs raccourci à 60 ms par le harnais
	r.waitSub(RoueSubStopping)
	if r.st().RoueWheel.Stop == nil || r.st().RoueWheel.Stop.To != wIdx90 {
		t.Errorf("la durée maximale déclenche le même arrêt : %+v", r.st().RoueWheel.Stop)
	}
	r.waitSub(RoueSubLetter)
}

func TestRoue144_Draw_UsesTheSourceOnceOverAllSegmentsAtTheStopRequest(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.idx.Store(wIdx90)
	r.mustOK(r.e.RoueSpin())
	if r.draws.Load() != 0 {
		t.Fatalf("aucun tirage au lancer")
	}
	r.waitClimbOver()
	r.mustOK(r.e.RoueStop())
	if r.draws.Load() != 1 || r.lastN.Load() != 11 {
		t.Errorf("un tirage sur 11 cases attendu, got %d tirage(s) sur %d", r.draws.Load(), r.lastN.Load())
	}
}

// Tirage uniforme (source réelle) : sur 3300 tirages, chaque case de la roue est tirée.
func TestRoue144_Draw_RealSourceCoversEverySegment(t *testing.T) {
	e := NewEngine()
	e.SetRoueRand(nil)
	seen := map[int]bool{}
	for i := 0; i < 3300; i++ {
		seen[e.roueRandN(11)] = true
	}
	if len(seen) != 11 {
		t.Errorf("toutes les cases doivent pouvoir sortir : %d/11", len(seen))
	}
}

// ---- crédit -----------------------------------------------------------------------------------------------

// credited : red 200 (N à 100), blue 90 (R à 90), green 0 ; red a la main et gagne avec M à 100 (+100 → 300).
func playToAnswer(t *testing.T, gains string) *roueRig {
	t.Helper()
	r := newRoueRig(t, roueTestPhrase, RoueModeChacunSonTour, gains, "red", "blue", "green")
	r.playHit(wIdx100, "N")
	r.mustOK(r.e.RoueNext(RoueHandNext)) // red encaisse 200 → blue
	r.playHit(wIdx90, "R")
	r.mustOK(r.e.RoueNext(RoueHandNext)) // blue encaisse 90 → green
	r.mustOK(r.e.RoucePass())            // green passe → red (cagnotte 0 pour green)
	if r.st().RoueCurrentTeam != "red" {
		t.Fatalf("préparation : red doit avoir la main, got %s", r.st().RoueCurrentTeam)
	}
	r.playHit(wIdx100, "M") // gain du tour 100 pas encore en cagnotte
	return r
}

func countAwarded(r *roueRig) int {
	n := 0
	for _, ev := range r.e.GetHistory() {
		if ev.EventType == "POINTS_AWARDED" {
			n++
		}
	}
	return n
}

func TestRoue144_Answer_WinnerOnlyCreditsOnlyTheWinner(t *testing.T) {
	r := playToAnswer(t, RoueGainsWinnerOnly)
	r.mustOK(r.e.RoueAnswer())
	s := r.st()
	if s.RoueSubPhase != RoueSubSolved || s.RoueStep != 0 || s.RoueWinner != "red" {
		t.Fatalf("SOLVED : %s step %d winner %q", s.RoueSubPhase, s.RoueStep, s.RoueWinner)
	}
	if got := s.RoueAwards; len(got) != 1 || got["red"] != 300 {
		t.Errorf("ROUE_AWARDS = %v, attendu {red:300} (200 + 100 du tour validé)", got)
	}
	if r.teamScore("red") != 300 || r.teamScore("blue") != 0 || r.teamScore("green") != 0 {
		t.Errorf("scores red/blue/green = %d/%d/%d, attendu 300/0/0", r.teamScore("red"), r.teamScore("blue"), r.teamScore("green"))
	}
	if n := countAwarded(r); n != 1 {
		t.Errorf("1 événement POINTS_AWARDED attendu, got %d", n)
	}
	if s.RoueNextTeam != "" || s.RoueNextDefault != "" {
		t.Errorf("plus d'équipe suivante après la résolution")
	}
}

func TestRoue144_Answer_AllTeamsCreditsEveryPotButZeroOnes(t *testing.T) {
	r := playToAnswer(t, RoueGainsAllTeams)
	r.mustOK(r.e.RoueAnswer())
	s := r.st()
	if s.RoueAwards["red"] != 300 || s.RoueAwards["blue"] != 90 {
		t.Errorf("ROUE_AWARDS = %v, attendu red:300 blue:90", s.RoueAwards)
	}
	if _, ok := s.RoueAwards["green"]; ok {
		t.Errorf("une cagnotte à 0 ne produit pas de crédit : %v", s.RoueAwards)
	}
	if r.teamScore("red") != 300 || r.teamScore("blue") != 90 || r.teamScore("green") != 0 {
		t.Errorf("scores %d/%d/%d, attendu 300/90/0", r.teamScore("red"), r.teamScore("blue"), r.teamScore("green"))
	}
	if n := countAwarded(r); n != 2 {
		t.Errorf("2 événements POINTS_AWARDED (pas de crédit pour 0), got %d", n)
	}
	r.nmu.Lock()
	defer r.nmu.Unlock()
	credits := 0
	for _, nt := range r.notices {
		if nt.Kind == RoueNoticeCredit {
			credits++
		}
	}
	if credits != 2 {
		t.Errorf("2 notices CREDIT (son de gain, comète) attendues, got %d", credits)
	}
}

func TestRoue144_Answer_BoardFullyVisibleAndPhraseRevealedAfterSolve(t *testing.T) {
	r := playToAnswer(t, "")
	r.mustOK(r.e.RoueAnswer())
	got := strings.Join(r.st().RoueBoard.Rows, " ")
	if strings.ContainsAny(got, "_*") || !strings.Contains(got, "BONJOUR") || !strings.Contains(got, "MONDE") {
		t.Errorf("plateau entièrement visible attendu : %v", r.st().RoueBoard.Rows)
	}
}

// Aucun score ne change avant la résolution, même avec des cagnottes (§7).
func TestRoue144_Scores_UntouchedBeforeSolve(t *testing.T) {
	r := playToAnswer(t, "")
	for _, tm := range []string{"red", "blue", "green"} {
		if r.teamScore(tm) != 0 {
			t.Errorf("score %s = %d avant résolution", tm, r.teamScore(tm))
		}
	}
}

func TestRoue144_Answer_DefaultGainsRuleIsWinnerOnly(t *testing.T) {
	r := playToAnswer(t, "")
	r.mustOK(r.e.RoueAnswer())
	if _, ok := r.st().RoueAwards["blue"]; ok {
		t.Errorf("règle par défaut WINNER_ONLY : blue ne reçoit rien")
	}
}

// ---- dévoilement ------------------------------------------------------------------------------------------

func TestRoue144_Reveal_NoWinnerNoCreditFromAnySubPhase(t *testing.T) {
	steps := map[string]func(r *roueRig){
		RoueSubTurn:     func(r *roueRig) {},
		RoueSubSpinning: func(r *roueRig) { r.mustOK(r.e.RoueSpin()) },
		RoueSubLetter:   func(r *roueRig) { r.land(wIdx100) },
		RoueSubLit: func(r *roueRig) {
			r.land(wIdx100)
			r.mustOK(r.e.RoueLetter("N"))
		},
		RoueSubAnswer:  func(r *roueRig) { r.playHit(wIdx100, "N") },
		RoueSubTurnEnd: func(r *roueRig) { r.land(wIdxBanq) },
	}
	for sub, prep := range steps {
		t.Run(sub, func(t *testing.T) {
			r := newRoueRig(t, roueTestPhrase, RoueModeChacunSonTour, RoueGainsAllTeams, "red", "blue")
			prep(r)
			r.mustOK(r.e.RoueReveal())
			s := r.st()
			if s.RoueSubPhase != RoueSubSolved || s.RoueWinner != "" || len(s.RoueAwards) != 0 || s.RoueLastEvent.Kind != RoueEvReveal {
				t.Fatalf("après dévoilement : %s winner=%q awards=%v event=%s", s.RoueSubPhase, s.RoueWinner, s.RoueAwards, s.RoueLastEvent.Kind)
			}
			if r.teamScore("red") != 0 || r.teamScore("blue") != 0 || countAwarded(r) != 0 {
				t.Errorf("aucun crédit au dévoilement")
			}
			if strings.ContainsAny(strings.Join(s.RoueBoard.Rows, ""), "_*") {
				t.Errorf("plateau entièrement visible : %v", s.RoueBoard.Rows)
			}
			if s.RoueWheel.Stop != nil || s.RoueTurnGain != 0 {
				t.Errorf("Stop et gain du tour remis à zéro")
			}
			r.wantErr(r.e.RoueReveal(), RoueErrWrongSubPhase) // déjà résolu
			r.wantErr(r.e.RoueSpin(), RoueErrWrongSubPhase)
		})
	}
}

// ---- minuteries : annulation et pause --------------------------------------------------------------------------

func TestRoue144_Timers_CancelledByRevealDuringBraking(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.e.mu.Lock()
	r.e.roue.stopDuration = 80 * time.Millisecond
	r.e.mu.Unlock()
	r.idx.Store(wIdx100)
	r.mustOK(r.e.RoueSpin())
	r.waitClimbOver()
	r.mustOK(r.e.RoueStop())
	r.mustOK(r.e.RoueReveal())
	time.Sleep(200 * time.Millisecond)
	if s := r.st(); s.RoueSubPhase != RoueSubSolved || s.RoueWheel.Current != -1 {
		t.Errorf("la minuterie de freinage ne doit plus agir après ROUE_REVEAL : %s current=%d", s.RoueSubPhase, s.RoueWheel.Current)
	}
}

func TestRoue144_Timers_CancelledByStopDuringBraking(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.e.mu.Lock()
	r.e.roue.stopDuration = 80 * time.Millisecond
	r.e.mu.Unlock()
	r.mustOK(r.e.RoueSpin())
	r.waitClimbOver()
	r.mustOK(r.e.RoueStop())
	r.e.Stop()
	time.Sleep(200 * time.Millisecond)
	if s := r.st(); s.RoueSubPhase != "" || s.RoueWheel.Current != -1 || len(s.RoueWheel.Segments) != 0 {
		t.Errorf("après STOP : état ROUE vidé et minuterie sans effet : %q current=%d", s.RoueSubPhase, s.RoueWheel.Current)
	}
}

func TestRoue144_Timers_CancelledByStopDuringMaxSpin(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.mustOK(r.e.RoueSpin()) // durée max 60 ms
	r.e.Stop()
	time.Sleep(150 * time.Millisecond)
	if s := r.st(); s.RoueSubPhase != "" || s.RoueWheel.Stop != nil {
		t.Errorf("la durée maximale ne doit plus agir après STOP : %q %+v", s.RoueSubPhase, s.RoueWheel.Stop)
	}
}

func TestRoue144_Timers_CancelledByRevealDuringFlip(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.e.mu.Lock()
	r.e.state.RoueWheel.FlipIntervalMs = 60
	r.e.mu.Unlock()
	r.land(wIdx100)
	r.mustOK(r.e.RoueLetter("N"))
	r.mustOK(r.e.RoueFlip())
	r.mustOK(r.e.RoueReveal())
	time.Sleep(200 * time.Millisecond)
	if r.st().RoueSubPhase != RoueSubSolved {
		t.Errorf("la fin du retournement ne doit pas relancer ANSWER après le dévoilement : %s", r.st().RoueSubPhase)
	}
}

func TestRoue144_Pause_FreezesTheBrakingTimer(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.e.mu.Lock()
	r.e.roue.stopDuration = 120 * time.Millisecond
	r.e.mu.Unlock()
	r.idx.Store(wIdx100)
	r.mustOK(r.e.RoueSpin())
	r.waitClimbOver()
	r.mustOK(r.e.RoueStop())
	r.e.Pause()
	time.Sleep(300 * time.Millisecond) // bien plus que la durée de freinage
	if r.st().RoueSubPhase != RoueSubStopping {
		t.Fatalf("en pause le freinage ne se termine pas : %s", r.st().RoueSubPhase)
	}
	// toute action ROUE_* est refusée en PAUSED (§5.1)
	if err := r.e.RoueStop(); err == nil {
		t.Errorf("ROUE_STOP refusé en PAUSED")
	}
	r.e.Continue()
	r.waitSub(RoueSubLetter)
	if r.st().RoueWheel.Current != wIdx100 {
		t.Errorf("après reprise : CURRENT = %d", r.st().RoueWheel.Current)
	}
}

func TestRoue144_Pause_FreezesTheMaxSpinTimer(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.mustOK(r.e.RoueSpin()) // 60 ms
	r.e.Pause()
	time.Sleep(200 * time.Millisecond)
	if r.st().RoueSubPhase != RoueSubSpinning {
		t.Fatalf("en pause la roue ne s'arrête pas d'elle-même : %s", r.st().RoueSubPhase)
	}
	r.e.Continue()
	r.waitSub(RoueSubStopping)
}

func TestRoue144_Pause_FreezesTheFlipTimer(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.e.mu.Lock()
	r.e.state.RoueWheel.FlipIntervalMs = 60
	r.e.mu.Unlock()
	r.land(wIdx100)
	r.mustOK(r.e.RoueLetter("N")) // 2 cases → 120 ms
	r.mustOK(r.e.RoueFlip())
	r.e.Pause()
	time.Sleep(250 * time.Millisecond)
	if r.st().RoueSubPhase != RoueSubRevealing {
		t.Fatalf("en pause le retournement ne se termine pas : %s", r.st().RoueSubPhase)
	}
	r.e.Continue()
	r.waitSub(RoueSubAnswer)
}

// ---- remise à zéro (NEW_GAME) ---------------------------------------------------------------------------------

func TestRoue144_InitGame_ClearsEverythingAndCancelsTimers(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	r.e.mu.Lock()
	r.e.roue.stopDuration = 80 * time.Millisecond
	r.e.mu.Unlock()
	r.mustOK(r.e.RoueSpin())
	r.waitClimbOver()
	r.mustOK(r.e.RoueStop())
	r.e.InitGame()
	time.Sleep(200 * time.Millisecond)
	s := r.st()
	if s.RoueSubPhase != "" || s.RoueStep != 0 || s.RoueCurrentTeam != "" || len(s.RoueTeams) != 0 || len(s.RoueTeamBank) != 0 ||
		len(s.RoueBoard.Rows) != 0 || len(s.RoueLetters) != 0 || s.RoueWheel.Current != -1 || s.RoueWheel.Stop != nil ||
		s.RoueWinner != "" || len(s.RoueAwards) != 0 || s.RoueTurnGain != 0 {
		t.Errorf("NEW_GAME : aucun résidu ROUE attendu : %+v", s)
	}
}

// ---- JSON : jamais d'omitempty, jamais null ---------------------------------------------------------------------

func TestRoue144_GameStateJSON_AlwaysSerializesRoueFieldsNeverNull(t *testing.T) {
	e := NewEngine()
	data, err := json.Marshal(e.GetState())
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	_ = json.Unmarshal(data, &m)
	for _, k := range []string{"ROUE_SUBPHASE", "ROUE_STEP", "ROUE_TEAMS", "ROUE_CURRENT_TEAM", "ROUE_CURRENT_TEAM_COLOR", "ROUE_NEXT_TEAM",
		"ROUE_NEXT_DEFAULT", "ROUE_TEAM_BANK", "ROUE_TURN_GAIN", "ROUE_BOARD", "ROUE_LETTERS", "ROUE_SHOW_LETTERS", "ROUE_CONSONANTS_LEFT",
		"ROUE_WHEEL", "ROUE_LAST_EVENT", "ROUE_WINNER", "ROUE_AWARDS"} {
		v, ok := m[k]
		if !ok {
			t.Errorf("%s doit toujours être sérialisé (pas d'omitempty)", k)
		} else if v == nil {
			t.Errorf("%s ne doit jamais valoir null", k)
		}
	}
	w, _ := m["ROUE_WHEEL"].(map[string]interface{})
	if w["SEGMENTS"] == nil {
		t.Errorf("ROUE_WHEEL.SEGMENTS ne doit pas être null")
	}
}

// ---- renommage / suppression d'équipe ----------------------------------------------------------------------------

func TestRoue144_RenameTeam_MigratesEveryRoueField(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeChacunSonTour, RoueGainsAllTeams, "red", "blue")
	r.playHit(wIdx100, "N")
	r.mustOK(r.e.RoueNext(RoueHandSame)) // red : 200
	r.land(wIdx90)
	r.mustOK(r.e.RoueLetter("Z"))
	r.e.mu.Lock()
	r.e.data.Teams["rouge"] = r.e.data.Teams["red"]
	r.e.data.Teams["rouge"].Name = "rouge"
	delete(r.e.data.Teams, "red")
	r.e.mu.Unlock()
	migrated := r.e.RenameTeamState("red", "rouge")
	s := r.st()
	if s.RoueCurrentTeam != "rouge" || s.RoueTeams[0] != "rouge" {
		t.Errorf("main / équipes : %s %v (migré : %v)", s.RoueCurrentTeam, s.RoueTeams, migrated)
	}
	if _, old := s.RoueTeamBank["red"]; old || s.RoueTeamBank["rouge"] != 200 {
		t.Errorf("cagnotte : %v", s.RoueTeamBank)
	}
	for _, l := range s.RoueLetters {
		if l.Team == "red" {
			t.Errorf("ROUE_LETTERS garde l'ancien nom : %+v", s.RoueLetters)
		}
	}
}

func TestRoue144_RemoveTeam_DropsItFromTheRound(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeChacunSonTour, "", "red", "blue", "green")
	r.e.PurgeTeamState("blue") // suppression d'équipe
	s := r.st()
	for _, tm := range s.RoueTeams {
		if tm == "blue" {
			t.Errorf("blue doit disparaître de ROUE_TEAMS : %v", s.RoueTeams)
		}
	}
	if _, still := s.RoueTeamBank["blue"]; still {
		t.Errorf("blue doit disparaître des cagnottes : %v", s.RoueTeamBank)
	}
}

// ---- ROUE_SET_TEAMS -------------------------------------------------------------------------------------------------

func TestRoue144_SetTeams_Rules(t *testing.T) {
	mk := func(mode string) *Engine {
		e := NewEngine()
		e.SetTeams(map[string]*Team{"red": {Name: "red"}, "blue": {Name: "blue"}, "green": {Name: "green"}})
		e.Ready("rq", &Question{ID: "rq", Question: "R", Type: QuestionTypeRoue,
			TypedContent: TypedContent{RouePhrase: "BONJOUR", RoueMode: mode}})
		return e
	}
	code := func(err error) string {
		if re, ok := err.(*RoueError); ok {
			return re.Code
		}
		return ""
	}
	solo := mk(RoueModeSolo)
	if err := solo.RoueSetTeams([]string{"red"}); err != nil {
		t.Errorf("SOLO : 1 équipe acceptée : %v", err)
	}
	// Le comptage n'est plus contrôlé à la sélection (correctif 12.0.0.2) : il l'est au passage à READY.
	if err := solo.RoueSetTeams([]string{"red", "blue"}); err != nil {
		t.Errorf("SOLO : 2 équipes acceptées à la sélection (le comptage est porté par la garde de départ) : %v", err)
	}
	for _, mode := range []string{RoueModeChacunSonTour, RoueModeTantQueJeGagne} {
		e := mk(mode)
		if err := e.RoueSetTeams([]string{"red"}); err != nil {
			t.Errorf("%s : 1 équipe acceptée à la sélection (clic incrémental) : %v", mode, err)
		}
		if err := e.RoueSetTeams([]string{"blue", "red"}); err != nil {
			t.Errorf("%s : 2 équipes acceptées : %v", mode, err)
		}
		if s := e.GetState(); s.RoueCurrentTeam != "blue" || s.RoueTeams[0] != "blue" {
			t.Errorf("la première équipe de la liste a la main : %s %v", s.RoueCurrentTeam, s.RoueTeams)
		}
		if code(e.RoueSetTeams([]string{"red", "red"})) != RoueErrInvalidTeams {
			t.Errorf("%s : doublon refusé", mode)
		}
		if code(e.RoueSetTeams([]string{"red", "fantome"})) != RoueErrInvalidTeams {
			t.Errorf("%s : équipe inconnue refusée", mode)
		}
		if err := e.RoueSetTeams(nil); err != nil || len(e.GetState().RoueTeams) != 0 || e.GetState().RoueCurrentTeam != "" {
			t.Errorf("%s : une liste vide efface la sélection : %v", mode, err)
		}
	}
}

// Clics incrémentaux du sélecteur : 1 puis 2 équipes hors SOLO. READY et FORCE_READY restent refusés tant que le
// compte n'est pas conforme, acceptés ensuite (comme RAFALE).
func TestRoue144_SetTeams_IncrementalClicksThenStartGuard(t *testing.T) {
	for _, mode := range []string{RoueModeChacunSonTour, RoueModeTantQueJeGagne, RoueModeSolo} {
		mk := func() *Engine {
			e := NewEngine()
			e.SetTeams(map[string]*Team{"red": {Name: "red", Ready: true}, "blue": {Name: "blue", Ready: true}})
			e.SetBumpers(map[string]*Bumper{
				"b-red":  {Name: "P-red", Team: "red", Connected: true, Ready: true},
				"b-blue": {Name: "P-blue", Team: "blue", Connected: true, Ready: true},
			})
			e.Ready("rq", &Question{ID: "rq", Question: "R", Type: QuestionTypeRoue,
				TypedContent: TypedContent{RouePhrase: "BONJOUR", RoueMode: mode}})
			return e
		}
		// Sans équipe : READY et FORCE_READY refusés.
		e := mk()
		e.ForceReady()
		if ph := e.GetState().Phase; ph != PhasePrepare {
			t.Errorf("%s : FORCE_READY sans équipe doit rester en PREPARE, got %s", mode, ph)
		}
		if mode == RoueModeSolo {
			// SOLO : 1 équipe conforme ; 2 équipes acceptées mais non conformes.
			if err := e.RoueSetTeams([]string{"red", "blue"}); err != nil {
				t.Fatalf("%s : %v", mode, err)
			}
			e.ForceReady()
			if ph := e.GetState().Phase; ph != PhasePrepare {
				t.Errorf("SOLO avec 2 équipes : reste en PREPARE, got %s", ph)
			}
			if err := e.RoueSetTeams([]string{"red"}); err != nil {
				t.Fatalf("SOLO : %v", err)
			}
			if ph := e.GetState().Phase; ph != PhaseReady {
				t.Errorf("SOLO avec 1 équipe : READY, got %s", ph)
			}
			continue
		}
		if err := e.RoueSetTeams([]string{"red"}); err != nil {
			t.Fatalf("%s : 1 équipe acceptée : %v", mode, err)
		}
		e.ForceReady()
		if ph := e.GetState().Phase; ph != PhasePrepare {
			t.Errorf("%s : 1 équipe non conforme, FORCE_READY refusé (PREPARE), got %s", mode, ph)
		}
		if err := e.RoueSetTeams([]string{"red", "blue"}); err != nil {
			t.Fatalf("%s : 2 équipes acceptées : %v", mode, err)
		}
		if ph := e.GetState().Phase; ph != PhaseReady {
			t.Errorf("%s : 2 équipes, tous prêts : READY, got %s", mode, ph)
		}
		if got := e.GetState().RoueTeams; len(got) != 2 {
			t.Errorf("%s : RoueTeams = %v", mode, got)
		}
	}
}

func TestRoue144_SetTeams_RefusedOutsidePrepareReadyOrOnAnotherType(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeChacunSonTour, "", "red", "blue")
	if code := func() string {
		if re, ok := r.e.RoueSetTeams([]string{"red", "blue"}).(*RoueError); ok {
			return re.Code
		}
		return ""
	}(); code != RoueErrNotAllowed {
		t.Errorf("STARTED : NOT_ALLOWED attendu, got %q", code)
	}
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}})
	e.Ready("sp", &Question{ID: "sp", Question: "Q", Answer: "A", Type: QuestionTypeSpeedy, Points: "10", Time: "30"})
	if err := e.RoueSetTeams([]string{"red"}); err == nil {
		t.Errorf("une question non ROUE refuse ROUE_SET_TEAMS")
	}
}

// Garde de départ : sans équipes conformes ou sans phrase, la question ne peut pas atteindre READY.
func TestRoue144_StartGuard_NeedsConformTeamsAndAPhrase(t *testing.T) {
	q := &Question{ID: "r", Type: QuestionTypeRoue, TypedContent: TypedContent{RouePhrase: "BONJOUR", RoueMode: RoueModeChacunSonTour}}
	if participantsConform(q, &GameState{RoueTeams: []string{"red"}}) {
		t.Errorf("CHACUN_SON_TOUR avec 1 équipe : non conforme")
	}
	if !participantsConform(q, &GameState{RoueTeams: []string{"red", "blue"}}) {
		t.Errorf("CHACUN_SON_TOUR avec 2 équipes : conforme")
	}
	solo := &Question{ID: "r", Type: QuestionTypeRoue, TypedContent: TypedContent{RouePhrase: "BONJOUR", RoueMode: RoueModeSolo}}
	if !participantsConform(solo, &GameState{RoueTeams: []string{"red"}}) || participantsConform(solo, &GameState{RoueTeams: []string{"red", "blue"}}) {
		t.Errorf("SOLO : exactement 1 équipe")
	}
	empty := &Question{ID: "r", Type: QuestionTypeRoue, TypedContent: TypedContent{RouePhrase: "  ", RoueMode: RoueModeSolo}}
	if participantsConform(empty, &GameState{RoueTeams: []string{"red"}}) {
		t.Errorf("phrase vide : non conforme")
	}
}

// ---- concurrence : double commande admin / animateur ------------------------------------------------------------------

// Le même geste envoyé par deux postes en même temps : appliqué une fois, l'autre refusé (WRONG_SUBPHASE).
func TestRoue144_DoubleCommand_SameGestureAppliedOnce(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	var wg sync.WaitGroup
	var ok, wrong atomic.Int64
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := r.e.RoueSpin()
			if err == nil {
				ok.Add(1)
			} else if re, isRoue := err.(*RoueError); isRoue && re.Code == RoueErrWrongSubPhase {
				wrong.Add(1)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 1 || wrong.Load() != 7 {
		t.Errorf("1 lancer accepté et 7 refusés WRONG_SUBPHASE attendus, got %d / %d", ok.Load(), wrong.Load())
	}
	if r.st().RoueLastEvent.Seq != 1 {
		t.Errorf("un seul événement SPIN : seq=%d", r.st().RoueLastEvent.Seq)
	}
}

func TestRoue144_DoubleCommand_NextAppliedOnce(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeChacunSonTour, "", "red", "blue", "green")
	r.land(wIdx100)
	r.mustOK(r.e.RoueLetter("Z"))
	var wg sync.WaitGroup
	var ok atomic.Int64
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if r.e.RoueNext(RoueHandNext) == nil {
				ok.Add(1)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 1 || r.st().RoueCurrentTeam != "blue" {
		t.Errorf("la main avance d'une équipe seulement : %d accepté(s), main=%s", ok.Load(), r.st().RoueCurrentTeam)
	}
}

// ---- callbacks ---------------------------------------------------------------------------------------------------------

// OnRoueChange est appelé hors verrou (le callback peut relire l'état sans interblocage) après chaque action.
func TestRoue144_Callbacks_FiredAfterEachActionOutsideTheLock(t *testing.T) {
	r := newRoueRig(t, roueTestPhrase, RoueModeSolo, "", "red")
	done := make(chan struct{})
	r.e.OnRoueChange = func() { _ = r.e.GetState(); r.changes.Add(1) }
	before := r.changes.Load()
	go func() { r.mustOK(r.e.RoueSpin()); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("interblocage : le callback relit l'état sous verrou")
	}
	if r.changes.Load() <= before {
		t.Errorf("OnRoueChange doit être appelé après ROUE_SPIN")
	}
	// un refus ne notifie pas
	n := r.changes.Load()
	_ = r.e.RoueLetter("N")
	if r.changes.Load() != n {
		t.Errorf("un refus ne déclenche pas de diffusion")
	}
}
