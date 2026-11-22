package game

// #147 (v12.1.0, LOT-1.2) — achat de voyelles : spécification (contracts/roue.md §15.1, rév. 7.2).
//
// Les questions sont construites par JSON (champs ROUE_* du contrat) et l'état est lu par son JSON : les tests ne
// dépendent donc que du fil (noms JSON du contrat) et d'une seule signature Go, `Engine.RoueBuyVowel(letter string) error`.
// Phrase de référence « TOTO TATA » : voyelles présentes O (2 cases) et A (2 cases) ; E, I, U, Y absentes.

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
)

type vopt map[string]interface{}

// vEngine147 monte une manche ROUE démarrée (TURN, étape 1). Équipes red / blue, cagnotte de red = 1000, de blue = 300.
// Le tirage est figé sur `*draw` (index de case), modifiable par le test.
func vEngine147(t *testing.T, over vopt, teams ...string) (*Engine, *int) {
	t.Helper()
	q := map[string]interface{}{"ID": "v147", "QUESTION": "Roue de la Fortune", "TYPE": "ROUE", "ROUE_PHRASE": "TOTO TATA", "ROUE_MODE": RoueModeTantQueJeGagne}
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
	e.Ready("v147", &question)
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

func vState147(t *testing.T, e *Engine) map[string]interface{} {
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
func vSub(t *testing.T, e *Engine) string { return vState147(t, e)["ROUE_SUBPHASE"].(string) }
func vBool(t *testing.T, e *Engine, k string) bool {
	t.Helper()
	b, ok := vState147(t, e)[k].(bool)
	if !ok {
		t.Fatalf("champ %s absent ou non booléen", k)
	}
	return b
}
func vBank(t *testing.T, e *Engine, team string) int {
	t.Helper()
	bank := vState147(t, e)["ROUE_TEAM_BANK"].(map[string]interface{})
	return int(bank[team].(float64))
}
func vEvent(t *testing.T, e *Engine) map[string]interface{} {
	return vState147(t, e)["ROUE_LAST_EVENT"].(map[string]interface{})
}
func vSeq(t *testing.T, e *Engine) int { return int(vEvent(t, e)["SEQ"].(float64)) }
func vKind(t *testing.T, e *Engine) string {
	return vEvent(t, e)["KIND"].(string)
}
func vBoardHasStar(t *testing.T, e *Engine) bool {
	rows := vState147(t, e)["ROUE_BOARD"].(map[string]interface{})["ROWS"].([]interface{})
	for _, r := range rows {
		if strings.Contains(r.(string), "*") {
			return true
		}
	}
	return false
}

func vCode(err error) string {
	if err == nil {
		return ""
	}
	if re, ok := err.(*RoueError); ok {
		return re.Code
	}
	return "ERR:" + err.Error()
}

// vBuy147 achète et exige un succès.
func vBuy147(t *testing.T, e *Engine, l string) {
	t.Helper()
	if err := e.RoueBuyVowel(l); err != nil {
		t.Fatalf("RoueBuyVowel(%s) refusé : %v", l, err)
	}
}

// vRefused147 exige un refus portant `code`, sans débit ni changement de sous-phase ni d'événement.
func vRefused147(t *testing.T, e *Engine, l, code string) {
	t.Helper()
	sub, bank, seq := vSub(t, e), vBank(t, e, "red"), vSeq(t, e)
	if got := vCode(e.RoueBuyVowel(l)); got != code {
		t.Fatalf("RoueBuyVowel(%s) : code %q, attendu %q", l, got, code)
	}
	if vSub(t, e) != sub || vBank(t, e, "red") != bank || vSeq(t, e) != seq {
		t.Fatalf("un refus %s doit laisser l'état intact (sous-phase %s→%s, cagnotte %d→%d)", code, sub, vSub(t, e), bank, vBank(t, e, "red"))
	}
}

// vFlip147 retourne les cases au bouton et laisse finir la minuterie.
func vFlip147(t *testing.T, e *Engine) {
	t.Helper()
	if err := e.RoueFlip(); err != nil {
		t.Fatalf("RoueFlip : %v", err)
	}
	fireRoueTimer144(e)
}

// vToLetter147 lance la roue et l'arrête sur la case `idx` ; vaut LETTER pour une case valeur.
func vToLetter147(t *testing.T, e *Engine, draw *int, idx int) {
	t.Helper()
	*draw = idx
	if err := e.RoueSpin(); err != nil {
		t.Fatalf("RoueSpin : %v", err)
	}
	fireRoueTimer144(e)
	if err := e.RoueStop(); err != nil {
		t.Fatal(err)
	}
	fireRoueTimer144(e)
}

func vSegIndex147(t *testing.T, e *Engine, kind string) int {
	t.Helper()
	for i, s := range e.GetState().RoueWheel.Segments {
		if s.Kind == kind {
			return i
		}
	}
	t.Fatalf("aucune case %s", kind)
	return -1
}

// ---- réglages : défauts, copie au START, désactivation ----------------------------------------------------------------

// critical — question v12.0 (aucun champ voyelle) : Avant de tourner, Plusieurs / Plusieurs, 250, Sans gain, achat activé.
func TestRoueVowel147_DefaultsCopiedAtStart(t *testing.T) {
	e, _ := vEngine147(t, nil)
	m := vState147(t, e)
	want := map[string]interface{}{
		"ROUE_BUY_VOWELS": true, "ROUE_VOWEL_TIMING": "BEFORE", "ROUE_VOWELS_BEFORE": "MANY", "ROUE_VOWELS_AFTER": "MANY",
		"ROUE_VOWEL_PRICE": float64(250), "ROUE_VOWEL_GAIN": "NONE", "ROUE_VOWELS_LEFT": true, "ROUE_TURN_VOWEL": false, "ROUE_STOP_VOWEL": false,
	}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("%s = %v, attendu %v", k, m[k], v)
		}
	}
}

func TestRoueVowel147_ConfiguredSettingsCopiedAtStart(t *testing.T) {
	e, _ := vEngine147(t, vopt{"ROUE_BUY_VOWELS": false, "ROUE_VOWEL_TIMING": "BOTH", "ROUE_VOWELS_BEFORE": "ONE", "ROUE_VOWELS_AFTER": "ONE", "ROUE_VOWEL_PRICE": 400, "ROUE_VOWEL_GAIN": "REFUND"})
	m := vState147(t, e)
	want := map[string]interface{}{"ROUE_BUY_VOWELS": false, "ROUE_VOWEL_TIMING": "BOTH", "ROUE_VOWELS_BEFORE": "ONE", "ROUE_VOWELS_AFTER": "ONE", "ROUE_VOWEL_PRICE": float64(400), "ROUE_VOWEL_GAIN": "REFUND"}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("%s = %v, attendu %v", k, m[k], v)
		}
	}
}

// Les 9 champs sont toujours sérialisés (pas d'omitempty), même hors manche.
func TestRoueVowel147_NineFieldsAlwaysSerialised(t *testing.T) {
	e := NewEngine()
	t.Cleanup(e.Stop)
	m := vState147(t, e)
	for _, k := range []string{"ROUE_BUY_VOWELS", "ROUE_VOWEL_TIMING", "ROUE_VOWELS_BEFORE", "ROUE_VOWELS_AFTER", "ROUE_VOWEL_PRICE", "ROUE_VOWEL_GAIN", "ROUE_VOWELS_LEFT", "ROUE_TURN_VOWEL", "ROUE_STOP_VOWEL"} {
		if _, ok := m[k]; !ok {
			t.Errorf("%s absent de GameState hors manche (omitempty interdit)", k)
		}
	}
	if m["ROUE_BUY_VOWELS"] != false || m["ROUE_VOWEL_TIMING"] != "" || m["ROUE_VOWEL_PRICE"] != float64(0) {
		t.Errorf("hors manche : valeurs vides attendues, got %v %v %v", m["ROUE_BUY_VOWELS"], m["ROUE_VOWEL_TIMING"], m["ROUE_VOWEL_PRICE"])
	}
}

// Achat désactivé : manche strictement v12.0 — refus VOWELS_DISABLED, ROUE_LETTER refuse toujours une voyelle, ROUE_SPIN refusé sans consonne.
func TestRoueVowel147_DisabledIsStrictlyV120(t *testing.T) {
	e, draw := vEngine147(t, vopt{"ROUE_BUY_VOWELS": false, "ROUE_VOWEL_TIMING": "BOTH"})
	vRefused147(t, e, "O", "VOWELS_DISABLED")
	vToLetter147(t, e, draw, 0)
	vRefused147(t, e, "O", "VOWELS_DISABLED")
	if got := vCode(e.RoueLetter("O")); got != RoueErrVowelNotAvailable {
		t.Fatalf("ROUE_LETTER d'une voyelle : %q, attendu VOWEL_NOT_AVAILABLE", got)
	}
}

func TestRoueVowel147_DisabledSpinWithoutConsonantRefused(t *testing.T) {
	e, _ := vEngine147(t, vopt{"ROUE_PHRASE": "OUI", "ROUE_BUY_VOWELS": false, "ROUE_VOWEL_TIMING": "AFTER"})
	if got := vCode(e.RoueSpin()); got != RoueErrWrongSubPhase {
		t.Fatalf("ROUE_SPIN sans consonne, achat désactivé : %q, attendu WRONG_SUBPHASE", got)
	}
}

// ---- avant de tourner : présente / absente ---------------------------------------------------------------------

// critical — achat d'une voyelle présente : débit de la cagnotte, aucun score de partie modifié, VOWEL_LIT à l'étape 1,
// cases éclairées, événement VOWEL_HIT { TEAM, LETTER, COUNT, VALUE = prix }, retournement au bouton.
func TestRoueVowel147_Before_PresentDebitsAndLights(t *testing.T) {
	e, _ := vEngine147(t, nil)
	seq := vSeq(t, e)
	vBuy147(t, e, "o")
	m := vState147(t, e)
	if m["ROUE_SUBPHASE"] != "VOWEL_LIT" || m["ROUE_STEP"] != float64(1) {
		t.Fatalf("attendu VOWEL_LIT / étape 1, got %v / %v", m["ROUE_SUBPHASE"], m["ROUE_STEP"])
	}
	if vBank(t, e, "red") != 750 {
		t.Fatalf("cagnotte = %d, attendu 750", vBank(t, e, "red"))
	}
	if !vBoardHasStar(t, e) {
		t.Errorf("les cases de la voyelle doivent être éclairées (*) en VOWEL_LIT")
	}
	ev := vEvent(t, e)
	if ev["KIND"] != "VOWEL_HIT" || ev["TEAM"] != "red" || ev["LETTER"] != "O" || ev["COUNT"] != float64(2) || ev["VALUE"] != float64(250) || ev["SEQ"] != float64(seq+1) {
		t.Fatalf("événement = %v", ev)
	}
	for name, tm := range e.GetTeamsAndBumpers().Teams {
		if tm.TeamPoints != 0 || tm.Score != 0 {
			t.Errorf("score de partie de %s modifié avant la résolution", name)
		}
	}
	letters := vState147(t, e)["ROUE_LETTERS"].([]interface{})
	if len(letters) != 1 || letters[0].(map[string]interface{})["LETTER"] != "O" || letters[0].(map[string]interface{})["HIT"] != true {
		t.Errorf("ROUE_LETTERS = %v", letters)
	}
	if vBool(t, e, "ROUE_TURN_VOWEL") {
		t.Errorf("ROUE_TURN_VOWEL ne passe à vrai qu'à la fin du retournement")
	}
}

// Retournement au bouton : VOWEL_REVEALING, événement FLIP, puis TURN, même équipe, ROUE_TURN_VOWEL vrai, aucun nouvel événement.
func TestRoueVowel147_Before_FlipThenBackToTurn(t *testing.T) {
	e, _ := vEngine147(t, nil)
	vBuy147(t, e, "O")
	if err := e.RoueFlip(); err != nil {
		t.Fatal(err)
	}
	if vSub(t, e) != "VOWEL_REVEALING" || vKind(t, e) != "FLIP" {
		t.Fatalf("après ROUE_FLIP : %s / %s", vSub(t, e), vKind(t, e))
	}
	seq := vSeq(t, e)
	fireRoueTimer144(e)
	m := vState147(t, e)
	if m["ROUE_SUBPHASE"] != "TURN" || m["ROUE_STEP"] != float64(1) || m["ROUE_CURRENT_TEAM"] != "red" || m["ROUE_TURN_VOWEL"] != true {
		t.Fatalf("fin du retournement : %v / %v / %v / %v", m["ROUE_SUBPHASE"], m["ROUE_STEP"], m["ROUE_CURRENT_TEAM"], m["ROUE_TURN_VOWEL"])
	}
	if vSeq(t, e) != seq {
		t.Errorf("la fin du retournement ne doit émettre aucun événement")
	}
}

// Voyelle absente (avant) : payée, « mauvaise réponse », tour perdu ; événement VOWEL_MISS, pas d'événement TURN_END.
func TestRoueVowel147_Before_AbsentPaysAndEndsTurn(t *testing.T) {
	e, _ := vEngine147(t, nil)
	var notices []RoueNotice
	e.OnRoueNotice = func(n RoueNotice) { notices = append(notices, n) }
	seq := vSeq(t, e)
	vBuy147(t, e, "E")
	m := vState147(t, e)
	if m["ROUE_SUBPHASE"] != "TURN_END" || vBank(t, e, "red") != 750 {
		t.Fatalf("attendu TURN_END et cagnotte 750, got %v / %d", m["ROUE_SUBPHASE"], vBank(t, e, "red"))
	}
	ev := vEvent(t, e)
	if ev["KIND"] != "VOWEL_MISS" || ev["TEAM"] != "red" || ev["LETTER"] != "E" || ev["VALUE"] != float64(250) || ev["SEQ"] != float64(seq+1) {
		t.Fatalf("événement = %v", ev)
	}
	if len(notices) != 1 || notices[0].Kind != RoueNoticeBad {
		t.Fatalf("notices = %+v, attendu une seule notice BAD", notices)
	}
	if m["ROUE_NEXT_DEFAULT"] != "NEXT" {
		t.Errorf("conseil de fin de tour = %v, attendu NEXT (lettre absente)", m["ROUE_NEXT_DEFAULT"])
	}
}

// Une voyelle présente ne produit aucune notice (aucun bruitage, contrat §15.4).
func TestRoueVowel147_PresentVowelMakesNoSound(t *testing.T) {
	e, _ := vEngine147(t, nil)
	var notices []RoueNotice
	e.OnRoueNotice = func(n RoueNotice) { notices = append(notices, n) }
	vBuy147(t, e, "O")
	vFlip147(t, e)
	if len(notices) != 0 {
		t.Fatalf("notices = %+v, attendu aucune", notices)
	}
}

// MANY : de retour en TURN, une autre voyelle s'achète ; la roue peut ensuite être lancée.
func TestRoueVowel147_Before_ManyChainsVowels(t *testing.T) {
	e, _ := vEngine147(t, nil)
	vBuy147(t, e, "O")
	vFlip147(t, e)
	vBuy147(t, e, "A")
	vFlip147(t, e)
	if vBank(t, e, "red") != 500 || vSub(t, e) != "TURN" {
		t.Fatalf("après 2 voyelles : cagnotte %d, sous-phase %s", vBank(t, e, "red"), vSub(t, e))
	}
	if err := e.RoueSpin(); err != nil {
		t.Fatalf("lancer la roue après les voyelles : %v", err)
	}
}

// ONE : une seule voyelle avant de tourner par tour, ensuite VOWEL_LIMIT_REACHED sans débit.
func TestRoueVowel147_Before_OneLimit(t *testing.T) {
	e, _ := vEngine147(t, vopt{"ROUE_VOWELS_BEFORE": "ONE"})
	vBuy147(t, e, "O")
	vFlip147(t, e)
	vRefused147(t, e, "A", "VOWEL_LIMIT_REACHED")
}

// Le plafond « 1 » se renouvelle au tour suivant (ROUE_NEXT).
func TestRoueVowel147_Before_OneLimitRenewedNextTurn(t *testing.T) {
	e, _ := vEngine147(t, vopt{"ROUE_VOWELS_BEFORE": "ONE"})
	vBuy147(t, e, "E") // absente : TURN_END
	if err := e.RoueNext(RoueHandSame); err != nil {
		t.Fatal(err)
	}
	if vBool(t, e, "ROUE_TURN_VOWEL") {
		t.Fatalf("ROUE_TURN_VOWEL doit être remis à faux à l'entrée dans un tour")
	}
	vBuy147(t, e, "O")
}

// Une fois la roue lancée : plus d'achat avant le prochain TURN.
func TestRoueVowel147_Before_NoPurchaseOnceSpun(t *testing.T) {
	e, _ := vEngine147(t, nil)
	if err := e.RoueSpin(); err != nil {
		t.Fatal(err)
	}
	vRefused147(t, e, "O", RoueErrWrongSubPhase)
}

// « Réponse validée » ouverte à l'étape 1 seulement après une voyelle présente (ROUE_TURN_VOWEL) ; sinon refusée.
func TestRoueVowel147_Before_AnswerOpenedAfterPresentVowelOnly(t *testing.T) {
	e, _ := vEngine147(t, nil)
	if err := e.RoueAnswer(); err == nil {
		t.Fatalf("ROUE_ANSWER en TURN sans voyelle achetée : refusé")
	}
	vBuy147(t, e, "O")
	vFlip147(t, e)
	if err := e.RoueAnswer(); err != nil {
		t.Fatalf("ROUE_ANSWER en TURN après une voyelle : %v", err)
	}
	m := vState147(t, e)
	if m["ROUE_SUBPHASE"] != "SOLVED" || m["ROUE_WINNER"] != "red" {
		t.Fatalf("attendu SOLVED pour red, got %v / %v", m["ROUE_SUBPHASE"], m["ROUE_WINNER"])
	}
}

// Les drapeaux sont remis à faux à ROUE_PASS.
func TestRoueVowel147_FlagsResetOnPass(t *testing.T) {
	e, _ := vEngine147(t, nil)
	vBuy147(t, e, "O")
	vFlip147(t, e)
	if !vBool(t, e, "ROUE_TURN_VOWEL") {
		t.Fatal("ROUE_TURN_VOWEL attendu vrai")
	}
	if err := e.RoucePass(); err != nil {
		t.Fatal(err)
	}
	if vBool(t, e, "ROUE_TURN_VOWEL") || vBool(t, e, "ROUE_STOP_VOWEL") {
		t.Fatalf("drapeaux voyelle non remis à faux après ROUE_PASS")
	}
}

// ---- gain ------------------------------------------------------------------------------------------------------

// Les 3 options : événement VOWEL_HIT { GAIN = gain à venir }, cagnotte inchangée jusqu'au retournement, crédit au retournement.
func TestRoueVowel147_GainOptions(t *testing.T) {
	cases := []struct {
		gain          string
		wantEventGain float64
		afterBuy      int
		afterFlip     int
	}{
		{"NONE", 0, 750, 750},
		{"REFUND", 250, 750, 1000},
		{"PER_OCCURRENCE", 500, 750, 1250}, // O × 2 cases × 250
	}
	for _, c := range cases {
		t.Run(c.gain, func(t *testing.T) {
			e, _ := vEngine147(t, vopt{"ROUE_VOWEL_GAIN": c.gain})
			vBuy147(t, e, "O")
			if got := vEvent(t, e)["GAIN"]; got != c.wantEventGain {
				t.Errorf("VOWEL_HIT.GAIN = %v, attendu %v", got, c.wantEventGain)
			}
			if vBank(t, e, "red") != c.afterBuy {
				t.Errorf("cagnotte à l'achat = %d, attendu %d (le gain n'est pas crédité avant le retournement)", vBank(t, e, "red"), c.afterBuy)
			}
			if err := e.RoueFlip(); err != nil {
				t.Fatal(err)
			}
			if vBank(t, e, "red") != c.afterFlip {
				t.Errorf("cagnotte au retournement = %d, attendu %d", vBank(t, e, "red"), c.afterFlip)
			}
			if got := vEvent(t, e); got["KIND"] != "FLIP" || got["GAIN"] != c.wantEventGain {
				t.Errorf("événement FLIP = %v", got)
			}
		})
	}
}

// ROUE_TURN_GAIN ne porte jamais le gain d'une voyelle.
func TestRoueVowel147_GainNeverGoesToTurnGain(t *testing.T) {
	e, _ := vEngine147(t, vopt{"ROUE_VOWEL_GAIN": "PER_OCCURRENCE"})
	vBuy147(t, e, "O")
	vFlip147(t, e)
	if g := vState147(t, e)["ROUE_TURN_GAIN"]; g != float64(0) {
		t.Fatalf("ROUE_TURN_GAIN = %v, attendu 0", g)
	}
}

// Le gain crédité est aussitôt disponible pour la voyelle suivante (cagnotte 250 : REFUND permet d'enchaîner).
func TestRoueVowel147_GainAvailableForNextVowel(t *testing.T) {
	e, _ := vEngine147(t, vopt{"ROUE_VOWEL_GAIN": "REFUND"})
	e.mu.Lock()
	e.state.RoueTeamBank["red"] = 250
	e.mu.Unlock()
	vBuy147(t, e, "O")
	vFlip147(t, e)
	if vBank(t, e, "red") != 250 {
		t.Fatalf("cagnotte = %d, attendu 250 (mise remboursée)", vBank(t, e, "red"))
	}
	vBuy147(t, e, "A")
}

// Une voyelle absente n'est jamais remboursée, quelle que soit l'option.
func TestRoueVowel147_AbsentNeverRefunded(t *testing.T) {
	for _, g := range []string{"NONE", "REFUND", "PER_OCCURRENCE"} {
		e, _ := vEngine147(t, vopt{"ROUE_VOWEL_GAIN": g})
		vBuy147(t, e, "E")
		if vBank(t, e, "red") != 750 {
			t.Errorf("gain %s : voyelle absente, cagnotte %d, attendu 750", g, vBank(t, e, "red"))
		}
	}
}

// ---- contrôles de refus, dans l'ordre -----------------------------------------------------------------------------

func TestRoueVowel147_RefusalsLeaveStateUntouched(t *testing.T) {
	cases := []struct {
		name   string
		over   vopt
		bank   int
		letter string
		code   string
	}{
		{"pas une lettre", nil, 1000, "1", "NOT_A_LETTER"},
		{"consonne", nil, 1000, "T", "NOT_A_VOWEL"},
		{"cagnotte insuffisante", nil, 100, "O", "INSUFFICIENT_BANK"},
		{"cagnotte juste au prix", nil, 250, "O", ""},
		{"désactivé", vopt{"ROUE_BUY_VOWELS": false}, 1000, "O", "VOWELS_DISABLED"},
		{"AFTER en TURN", vopt{"ROUE_VOWEL_TIMING": "AFTER"}, 1000, "O", "VOWEL_NOT_NOW"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, _ := vEngine147(t, c.over)
			e.mu.Lock()
			e.state.RoueTeamBank["red"] = c.bank
			e.mu.Unlock()
			if c.code == "" {
				vBuy147(t, e, c.letter)
				if vBank(t, e, "red") != 0 {
					t.Errorf("cagnotte = %d, attendu 0", vBank(t, e, "red"))
				}
				return
			}
			vRefused147(t, e, c.letter, c.code)
		})
	}
}

// Voyelle déjà proposée : refusée sans débit (décision 2), à tous les points d'achat.
func TestRoueVowel147_AlreadyProposedRefusedWithoutDebit(t *testing.T) {
	e, _ := vEngine147(t, nil)
	vBuy147(t, e, "O")
	vFlip147(t, e)
	vRefused147(t, e, "O", "LETTER_ALREADY_PROPOSED")
	e2, _ := vEngine147(t, nil)
	vBuy147(t, e2, "E") // absente, tour perdu : TURN_END → tour suivant, E déjà proposée
	_ = e2.RoueNext(RoueHandSame)
	vRefused147(t, e2, "E", "LETTER_ALREADY_PROPOSED")
}

// Ordre des contrôles : désactivé avant moment ; moment avant « pas une voyelle » ; limite avant « pas une voyelle ».
func TestRoueVowel147_RefusalOrder(t *testing.T) {
	e, _ := vEngine147(t, vopt{"ROUE_BUY_VOWELS": false, "ROUE_VOWEL_TIMING": "AFTER"})
	vRefused147(t, e, "T", "VOWELS_DISABLED")
	e, _ = vEngine147(t, vopt{"ROUE_VOWEL_TIMING": "AFTER"})
	vRefused147(t, e, "T", "VOWEL_NOT_NOW")
	e, _ = vEngine147(t, vopt{"ROUE_VOWELS_BEFORE": "ONE"})
	vBuy147(t, e, "O")
	vFlip147(t, e)
	vRefused147(t, e, "T", "VOWEL_LIMIT_REACHED")
	e, _ = vEngine147(t, nil)
	vRefused147(t, e, "T", "NOT_A_VOWEL") // consonne avant cagnotte : ordre 5 avant 8
	e.mu.Lock()
	e.state.RoueTeamBank["red"] = 0
	e.mu.Unlock()
	vRefused147(t, e, "T", "NOT_A_VOWEL")
}

// Plus de voyelle à trouver : ROUE_VOWELS_LEFT faux, refus NO_VOWEL_LEFT, même pour une voyelle absente.
func TestRoueVowel147_NoVowelLeft(t *testing.T) {
	e, _ := vEngine147(t, vopt{"ROUE_PHRASE": "TOTO"})
	if !vBool(t, e, "ROUE_VOWELS_LEFT") {
		t.Fatal("ROUE_VOWELS_LEFT attendu vrai au départ")
	}
	vBuy147(t, e, "O")
	vFlip147(t, e)
	if vBool(t, e, "ROUE_VOWELS_LEFT") {
		t.Fatal("ROUE_VOWELS_LEFT attendu faux : la seule voyelle est révélée")
	}
	vRefused147(t, e, "A", "NO_VOWEL_LEFT")
}

// ROUE_VOWELS_LEFT retombe aussi quand les voyelles ont été proposées (même absentes de la phrase, il en reste d'autres).
func TestRoueVowel147_VowelsLeftOnlyCountsPhraseVowels(t *testing.T) {
	e, _ := vEngine147(t, vopt{"ROUE_PHRASE": "TOTO"})
	vBuy147(t, e, "E") // absente, tour perdu
	if !vBool(t, e, "ROUE_VOWELS_LEFT") {
		t.Fatal("proposer une voyelle absente ne consomme pas les voyelles de la phrase")
	}
}

// ---- après l'arrêt de la roue ----------------------------------------------------------------------------------

// Voyelle présente à la place de la consonne : LIT (étape 5), ROUE_STOP_VOWEL vrai, consonne refusée, retournement → ANSWER (étape 6).
func TestRoueVowel147_After_PresentReplacesConsonant(t *testing.T) {
	e, draw := vEngine147(t, vopt{"ROUE_VOWEL_TIMING": "AFTER"})
	vToLetter147(t, e, draw, 0)
	if vSub(t, e) != "LETTER" {
		t.Fatalf("attendu LETTER, got %s", vSub(t, e))
	}
	vBuy147(t, e, "O")
	m := vState147(t, e)
	if m["ROUE_SUBPHASE"] != "LIT" || m["ROUE_STEP"] != float64(5) || m["ROUE_STOP_VOWEL"] != true {
		t.Fatalf("après achat : %v / étape %v / STOP_VOWEL %v", m["ROUE_SUBPHASE"], m["ROUE_STEP"], m["ROUE_STOP_VOWEL"])
	}
	if vKind(t, e) != "VOWEL_HIT" || vBank(t, e, "red") != 750 {
		t.Fatalf("événement %s, cagnotte %d", vKind(t, e), vBank(t, e, "red"))
	}
	vFlip147(t, e)
	m = vState147(t, e)
	if m["ROUE_SUBPHASE"] != "ANSWER" || m["ROUE_STEP"] != float64(6) || m["ROUE_NEXT_DEFAULT"] != "SAME" {
		t.Fatalf("après retournement : %v / %v / %v", m["ROUE_SUBPHASE"], m["ROUE_STEP"], m["ROUE_NEXT_DEFAULT"])
	}
	if got := vCode(e.RoueLetter("T")); got != RoueErrWrongSubPhase {
		t.Fatalf("plus aucune consonne après une voyelle achetée à cet arrêt : %q", got)
	}
	if m["ROUE_TURN_GAIN"] != float64(0) {
		t.Errorf("la valeur de la case tirée n'est pas jouée : ROUE_TURN_GAIN = %v", m["ROUE_TURN_GAIN"])
	}
}

// MANY : à l'étape 6, une autre voyelle s'achète (LIT → REVEALING → ANSWER) ; ONE : VOWEL_LIMIT_REACHED.
func TestRoueVowel147_After_ManyChainsFromAnswer(t *testing.T) {
	e, draw := vEngine147(t, vopt{"ROUE_VOWEL_TIMING": "AFTER"})
	vToLetter147(t, e, draw, 0)
	vBuy147(t, e, "O")
	vFlip147(t, e)
	vBuy147(t, e, "A")
	if vSub(t, e) != "LIT" || vBank(t, e, "red") != 500 {
		t.Fatalf("2ᵉ voyelle : %s / cagnotte %d", vSub(t, e), vBank(t, e, "red"))
	}
	vFlip147(t, e)
	if vSub(t, e) != "ANSWER" {
		t.Fatalf("attendu ANSWER, got %s", vSub(t, e))
	}
}

func TestRoueVowel147_After_OneStopsAtFirstVowel(t *testing.T) {
	e, draw := vEngine147(t, vopt{"ROUE_VOWEL_TIMING": "AFTER", "ROUE_VOWELS_AFTER": "ONE"})
	vToLetter147(t, e, draw, 0)
	vBuy147(t, e, "O")
	vFlip147(t, e)
	vRefused147(t, e, "A", "VOWEL_LIMIT_REACHED")
}

// Absente après l'arrêt : TURN_END, VOWEL_MISS, tour perdu — depuis LETTER et depuis ANSWER (arrête l'enchaînement).
func TestRoueVowel147_After_AbsentEndsTurn(t *testing.T) {
	e, draw := vEngine147(t, vopt{"ROUE_VOWEL_TIMING": "AFTER"})
	vToLetter147(t, e, draw, 0)
	vBuy147(t, e, "E")
	if vSub(t, e) != "TURN_END" || vKind(t, e) != "VOWEL_MISS" || vBank(t, e, "red") != 750 {
		t.Fatalf("depuis LETTER : %s / %s / %d", vSub(t, e), vKind(t, e), vBank(t, e, "red"))
	}
	e, draw = vEngine147(t, vopt{"ROUE_VOWEL_TIMING": "AFTER"})
	vToLetter147(t, e, draw, 0)
	vBuy147(t, e, "O")
	vFlip147(t, e)
	vBuy147(t, e, "E")
	if vSub(t, e) != "TURN_END" || vKind(t, e) != "VOWEL_MISS" || vBank(t, e, "red") != 500 {
		t.Fatalf("depuis ANSWER : %s / %s / %d", vSub(t, e), vKind(t, e), vBank(t, e, "red"))
	}
}

// Moments : BEFORE refuse LETTER ; ANSWER après une consonne refuse l'achat ; BEFORE refuse ANSWER.
func TestRoueVowel147_After_NotNowCases(t *testing.T) {
	e, draw := vEngine147(t, nil) // BEFORE
	vToLetter147(t, e, draw, 0)
	vRefused147(t, e, "O", "VOWEL_NOT_NOW")

	e, draw = vEngine147(t, vopt{"ROUE_VOWEL_TIMING": "BOTH"})
	vToLetter147(t, e, draw, 0)
	if err := e.RoueLetter("T"); err != nil {
		t.Fatal(err)
	}
	vFlip147(t, e)
	if vSub(t, e) != "ANSWER" {
		t.Fatalf("attendu ANSWER, got %s", vSub(t, e))
	}
	vRefused147(t, e, "O", "VOWEL_NOT_NOW") // ANSWER sans ROUE_STOP_VOWEL
}

// BOTH : l'achat est possible avant de tourner puis après l'arrêt ; l'achat « avant » redevient possible au TURN suivant.
func TestRoueVowel147_Both_BeforeAndAfter(t *testing.T) {
	e, draw := vEngine147(t, vopt{"ROUE_VOWEL_TIMING": "BOTH"})
	vBuy147(t, e, "O")
	vFlip147(t, e)
	vToLetter147(t, e, draw, 0)
	vBuy147(t, e, "A")
	if vSub(t, e) != "LIT" || !vBool(t, e, "ROUE_STOP_VOWEL") {
		t.Fatalf("achat après l'arrêt en BOTH : %s", vSub(t, e))
	}
	vFlip147(t, e)
	if err := e.RoueNext(RoueHandSame); err != nil {
		t.Fatal(err)
	}
	if vBool(t, e, "ROUE_STOP_VOWEL") || vBool(t, e, "ROUE_TURN_VOWEL") {
		t.Fatal("drapeaux non remis à faux au nouveau tour")
	}
}

// ROUE_SPIN remet ROUE_STOP_VOWEL à faux (nouveau lancer).
func TestRoueVowel147_SpinResetsStopVowel(t *testing.T) {
	e, draw := vEngine147(t, vopt{"ROUE_VOWEL_TIMING": "BOTH"})
	vToLetter147(t, e, draw, 0)
	vBuy147(t, e, "O")
	vFlip147(t, e)
	if err := e.RoueNext(RoueHandSame); err != nil {
		t.Fatal(err)
	}
	vToLetter147(t, e, draw, 0)
	if vBool(t, e, "ROUE_STOP_VOWEL") {
		t.Fatal("ROUE_STOP_VOWEL doit être faux après un nouveau lancer")
	}
}

// Cases spéciales : aucun achat « après » sur BANQUEROUTE et TOUR PERDU (TURN_END) ; BONUS → TURN, drapeaux à faux,
// achat « avant » possible si le moment le permet (décision 1 et 10).
func TestRoueVowel147_SpecialCasesNoAfterPurchase(t *testing.T) {
	for _, kind := range []string{RoueKindBankrupt, RoueKindLoseTurn} {
		t.Run(kind, func(t *testing.T) {
			e, draw := vEngine147(t, vopt{"ROUE_VOWEL_TIMING": "BOTH"})
			vToLetter147(t, e, draw, vSegIndex147(t, e, kind))
			if vSub(t, e) != "TURN_END" {
				t.Fatalf("attendu TURN_END, got %s", vSub(t, e))
			}
			vRefused147(t, e, "O", RoueErrWrongSubPhase)
		})
	}
}

func TestRoueVowel147_BonusRestartsTurnAndResetsFlags(t *testing.T) {
	over := vopt{"ROUE_VOWEL_TIMING": "BOTH", "ROUE_BONUS_COUNT": 1, "ROUE_VOWELS_BEFORE": "ONE"}
	e, draw := vEngine147(t, over)
	vBuy147(t, e, "O") // avant de tourner : ROUE_TURN_VOWEL vrai, plafond « 1 » atteint
	vFlip147(t, e)
	vRefused147(t, e, "A", "VOWEL_LIMIT_REACHED")
	vToLetter147(t, e, draw, vSegIndex147(t, e, RoueKindBonus))
	if vSub(t, e) != "TURN" || vKind(t, e) != "BONUS" {
		t.Fatalf("attendu TURN après BONUS, got %s / %s", vSub(t, e), vKind(t, e))
	}
	if vBool(t, e, "ROUE_TURN_VOWEL") || vBool(t, e, "ROUE_STOP_VOWEL") {
		t.Fatal("tour BONUS : ROUE_TURN_VOWEL et ROUE_STOP_VOWEL doivent être remis à faux")
	}
	vBuy147(t, e, "A") // le plafond « 1 » est renouvelé : achat avant de tourner possible
}

func TestRoueVowel147_BonusNoAfterPurchaseWhenTimingAfter(t *testing.T) {
	e, draw := vEngine147(t, vopt{"ROUE_VOWEL_TIMING": "AFTER", "ROUE_BONUS_COUNT": 1})
	vToLetter147(t, e, draw, vSegIndex147(t, e, RoueKindBonus))
	vRefused147(t, e, "O", "VOWEL_NOT_NOW") // TURN avec AFTER
}

// ---- ROUE_SPIN sans consonne restante ---------------------------------------------------------------------------

func TestRoueVowel147_SpinWithoutConsonant(t *testing.T) {
	cases := []struct {
		name string
		over vopt
		bank int
		ok   bool
	}{
		{"AFTER, cagnotte suffisante", vopt{"ROUE_VOWEL_TIMING": "AFTER"}, 1000, true},
		{"BOTH, cagnotte suffisante", vopt{"ROUE_VOWEL_TIMING": "BOTH"}, 1000, true},
		{"BEFORE : refusé comme v12.0", vopt{"ROUE_VOWEL_TIMING": "BEFORE"}, 1000, false},
		{"AFTER, cagnotte < prix : refusé", vopt{"ROUE_VOWEL_TIMING": "AFTER"}, 100, false},
		{"AFTER, achat désactivé : refusé", vopt{"ROUE_VOWEL_TIMING": "AFTER", "ROUE_BUY_VOWELS": false}, 1000, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			over := vopt{"ROUE_PHRASE": "OUI"}
			for k, v := range c.over {
				over[k] = v
			}
			e, _ := vEngine147(t, over)
			e.mu.Lock()
			e.state.RoueTeamBank["red"] = c.bank
			e.mu.Unlock()
			err := e.RoueSpin()
			if c.ok && err != nil {
				t.Fatalf("ROUE_SPIN refusé : %v", err)
			}
			if !c.ok && vCode(err) != RoueErrWrongSubPhase {
				t.Fatalf("ROUE_SPIN : %q, attendu WRONG_SUBPHASE", vCode(err))
			}
		})
	}
}

// Sans consonne : à l'arrêt, seules des voyelles peuvent être jouées.
func TestRoueVowel147_NoConsonantStopThenOnlyVowels(t *testing.T) {
	e, draw := vEngine147(t, vopt{"ROUE_PHRASE": "OUI", "ROUE_VOWEL_TIMING": "AFTER"})
	vToLetter147(t, e, draw, 0)
	if vSub(t, e) != "LETTER" {
		t.Fatalf("attendu LETTER, got %s", vSub(t, e))
	}
	vBuy147(t, e, "O")
	if vSub(t, e) != "LIT" {
		t.Fatalf("attendu LIT, got %s", vSub(t, e))
	}
}

// ---- pause, dévoilement, concurrence, SOLO ----------------------------------------------------------------------

// ROUE_REVEAL pendant l'éclairage ou le retournement d'une voyelle : accepté, le prix reste débité, aucun gain.
func TestRoueVowel147_RevealDuringVowelKeepsDebitNoGain(t *testing.T) {
	for _, step := range []string{"VOWEL_LIT", "VOWEL_REVEALING"} {
		t.Run(step, func(t *testing.T) {
			e, _ := vEngine147(t, vopt{"ROUE_VOWEL_GAIN": "REFUND"})
			vBuy147(t, e, "O")
			if step == "VOWEL_REVEALING" {
				// retournement amorcé : le gain est crédité au retournement, avant le dévoilement
				if err := e.RoueFlip(); err != nil {
					t.Fatal(err)
				}
			}
			bank := vBank(t, e, "red")
			if err := e.RoueReveal(); err != nil {
				t.Fatalf("ROUE_REVEAL pendant %s : %v", step, err)
			}
			if vSub(t, e) != "SOLVED" || vBank(t, e, "red") != bank {
				t.Fatalf("après ROUE_REVEAL : %s, cagnotte %d (attendu %d)", vSub(t, e), vBank(t, e, "red"), bank)
			}
		})
	}
}

// Pause et reprise pendant le retournement d'une voyelle : la minuterie est suspendue puis reprise.
func TestRoueVowel147_PauseResumeDuringVowelFlip(t *testing.T) {
	e, _ := vEngine147(t, nil)
	vBuy147(t, e, "O")
	if err := e.RoueFlip(); err != nil {
		t.Fatal(err)
	}
	e.Pause()
	if got := vCode(e.RoueBuyVowel("A")); got == "" {
		t.Fatalf("toute action ROUE_* est refusée en PAUSED")
	}
	e.Continue()
	fireRoueTimer144(e)
	if vSub(t, e) != "TURN" || !vBool(t, e, "ROUE_TURN_VOWEL") {
		t.Fatalf("après reprise : %s", vSub(t, e))
	}
}

// Concurrence admin / animateur : deux achats simultanés de la même voyelle, un seul passe, un seul débit.
func TestRoueVowel147_ConcurrentSameVowelDebitsOnce(t *testing.T) {
	e, _ := vEngine147(t, nil)
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func(i int) { defer wg.Done(); errs[i] = e.RoueBuyVowel("O") }(i)
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		}
	}
	if ok != 1 {
		t.Fatalf("%d achats acceptés sur 8, attendu 1 (%v)", ok, errs)
	}
	if vBank(t, e, "red") != 750 {
		t.Fatalf("cagnotte = %d, attendu 750 (un seul débit)", vBank(t, e, "red"))
	}
}

// SOLO : une équipe, l'achat fonctionne ; voyelle absente → TURN_END.
func TestRoueVowel147_Solo(t *testing.T) {
	e, _ := vEngine147(t, vopt{"ROUE_MODE": RoueModeSolo}, "red")
	vBuy147(t, e, "O")
	vFlip147(t, e)
	vBuy147(t, e, "E")
	if vSub(t, e) != "TURN_END" || vBank(t, e, "red") != 500 {
		t.Fatalf("SOLO : %s / %d", vSub(t, e), vBank(t, e, "red"))
	}
}

// Achat refusé hors manche ROUE ou en phase non STARTED.
func TestRoueVowel147_NoRoundNoPurchase(t *testing.T) {
	e := NewEngine()
	t.Cleanup(e.Stop)
	if got := vCode(e.RoueBuyVowel("O")); got != RoueErrNotAllowed {
		t.Fatalf("hors manche : %q, attendu NOT_ALLOWED", got)
	}
}

// Matrice moment × nombre : combien de voyelles s'enchaînent avant (TURN) et après (ANSWER).
func TestRoueVowel147_Matrix(t *testing.T) {
	for _, timing := range []string{"BEFORE", "AFTER", "BOTH"} {
		for _, before := range []string{"ONE", "MANY"} {
			for _, after := range []string{"ONE", "MANY"} {
				name := fmt.Sprintf("%s/%s/%s", timing, before, after)
				t.Run(name, func(t *testing.T) {
					e, draw := vEngine147(t, vopt{"ROUE_VOWEL_TIMING": timing, "ROUE_VOWELS_BEFORE": before, "ROUE_VOWELS_AFTER": after})
					// Avant de tourner.
					err := e.RoueBuyVowel("O")
					if timing == "AFTER" {
						if vCode(err) != "VOWEL_NOT_NOW" {
							t.Fatalf("TURN en AFTER : %q", vCode(err))
						}
					} else {
						if err != nil {
							t.Fatalf("1ʳᵉ voyelle avant : %v", err)
						}
						vFlip147(t, e)
						second := e.RoueBuyVowel("A")
						switch before {
						case "ONE":
							if vCode(second) != "VOWEL_LIMIT_REACHED" {
								t.Fatalf("2ᵉ voyelle avant avec ONE : %q", vCode(second))
							}
						default:
							if second != nil {
								t.Fatalf("2ᵉ voyelle avant avec MANY : %v", second)
							}
							vFlip147(t, e)
						}
					}
					// Après l'arrêt (nouveau tour pour réinitialiser les plafonds).
					if vSub(t, e) == "TURN_END" {
						_ = e.RoueNext(RoueHandSame)
					} else {
						_ = e.RoueNext(RoueHandSame)
					}
					vToLetter147(t, e, draw, 0)
					first := e.RoueBuyVowel("O")
					if timing == "BEFORE" {
						if vCode(first) != "VOWEL_NOT_NOW" {
							t.Fatalf("LETTER en BEFORE : %q", vCode(first))
						}
						return
					}
					if first != nil && vCode(first) != "LETTER_ALREADY_PROPOSED" {
						t.Fatalf("1ʳᵉ voyelle après : %v", first)
					}
				})
			}
		}
	}
}
