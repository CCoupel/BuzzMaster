// #282 (v11.4.0, LOT-2) — option « Alphabétique » de la manche RAFALE : tirage par passes A→Z,
// RAFALE_CURRENT_LETTER / RAFALE_LETTERS. Contrat : contracts/rafale.md §15.
//
// Tests par la FRONTIÈRE (docs/TEST_PROCEDURE.md §2.3) : l'option est posée par JSON
// (RAFALE_ALPHABETICAL), l'état est lu par la sérialisation JSON de GameState — aucun symbole non livré
// (champ, type ou fonction) n'est référencé, le fichier compile avant le dev et échoue par assertion.
// Les ordres de tirage sont observés via RAFALE_CURRENT_QUESTION.ID (jamais la réponse).
package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// tw282Words : id → réponse. Les ids sont choisis pour lire les ordres attendus.
type tw282Word struct{ id, answer string }

// tw282AlphaQuestion construit une manche RAFALE SOLO avec l'option posée par JSON.
func tw282AlphaQuestion(t *testing.T, alphabetical bool, maxQuestions int) *Question {
	t.Helper()
	q := makeRafaleQuestion("rq-alpha", string(RafaleModeSolo), CategoryHistory, 1)
	q.RafaleMaxQuestions = maxQuestions
	raw, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("setup invalide : %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("setup invalide : %v", err)
	}
	m["RAFALE_ALPHABETICAL"] = alphabetical
	raw, _ = json.Marshal(m)
	var out Question
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("setup invalide : %v", err)
	}
	return &out
}

// tw282Engine amorce un moteur avec un réservoir et une manche prête, non démarrée.
func tw282Engine(t *testing.T, words []tw282Word, alphabetical bool, maxQuestions int) *Engine {
	t.Helper()
	e := NewEngine()
	e.SetTeams(map[string]*Team{"red": {Name: "red"}})
	rq := make([]RafaleQuestion, 0, len(words))
	for _, w := range words {
		rq = append(rq, RafaleQuestion{ID: w.id, Question: "Q " + w.id, Answer: w.answer, Category: CategoryHistory, Difficulty: 1})
	}
	seedRafaleReservoir(t, e, rq)
	e.Ready("rq-alpha", tw282AlphaQuestion(t, alphabetical, maxQuestions))
	if err := e.SetRafaleParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams : %v", err)
	}
	return e
}

// tw282State sérialise GameState et renvoie la charge utile en map (frontière JSON).
func tw282State(t *testing.T, e *Engine) map[string]interface{} {
	t.Helper()
	raw, err := json.Marshal(e.GetState())
	if err != nil {
		t.Fatalf("marshal GameState : %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal GameState : %v", err)
	}
	return m
}

func tw282CurrentID(t *testing.T, e *Engine) string {
	t.Helper()
	cur, _ := tw282State(t, e)["RAFALE_CURRENT_QUESTION"].(map[string]interface{})
	id, _ := cur["ID"].(string)
	return id
}

// tw282Letter lit RAFALE_CURRENT_LETTER ; échoue si la clé est absente (jamais omitempty, CLAUDE.md).
func tw282Letter(t *testing.T, e *Engine) string {
	t.Helper()
	v, ok := tw282State(t, e)["RAFALE_CURRENT_LETTER"]
	if !ok {
		t.Fatal("RAFALE_CURRENT_LETTER absent de GameState (jamais omitempty — contrat §15.5)")
	}
	s, _ := v.(string)
	return s
}

// tw282Letters lit RAFALE_LETTERS ; échoue si absent ou null (jamais nil — contrat §15.5).
func tw282Letters(t *testing.T, e *Engine) []string {
	t.Helper()
	v, ok := tw282State(t, e)["RAFALE_LETTERS"]
	if !ok {
		t.Fatal("RAFALE_LETTERS absent de GameState (jamais omitempty — contrat §15.5)")
	}
	arr, isArr := v.([]interface{})
	if !isArr {
		t.Fatalf("RAFALE_LETTERS doit être un tableau JSON (jamais null), got %#v", v)
	}
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		out = append(out, fmt.Sprint(x))
	}
	return out
}

// tw282Run démarre la manche et avance jusqu'à épuisement du réservoir en validant ;
// renvoie les ids et lettres courantes dans l'ordre.
func tw282Run(t *testing.T, e *Engine, n int) (ids, letters []string) {
	t.Helper()
	e.StartImmediate(0)
	t.Cleanup(e.Stop)
	for i := 0; i < n; i++ {
		ids = append(ids, tw282CurrentID(t, e))
		letters = append(letters, tw282Letter(t, e))
		if i == n-1 {
			break
		}
		if err := e.RafaleValidate(); err != nil {
			t.Fatalf("RafaleValidate à l'avance %d : %v", i, err)
		}
	}
	return ids, letters
}

// §15.4 — « une lettre à la fois, cyclique A→Z » : passe 1 = 1er mot de chaque lettre, passe 2 = 2ᵉ…
func TestRafaleAlpha282_Draw_ByPasses_A2Z(t *testing.T) {
	e := tw282Engine(t, []tw282Word{
		{"a2", "Avion"}, {"b2", "Bateau"}, {"c2", "Citron"},
		{"a1", "Abricot"}, {"b1", "Banane"}, {"c1", "Cerise"},
	}, true, 100)
	ids, letters := tw282Run(t, e, 6)
	if want := []string{"a1", "b1", "c1", "a2", "b2", "c2"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("ordre de tirage = %v, attendu %v (passes A→B→C, tri par clé puis ID dans la lettre)", ids, want)
	}
	if want := []string{"A", "B", "C", "A", "B", "C"}; !reflect.DeepEqual(letters, want) {
		t.Errorf("RAFALE_CURRENT_LETTER = %v, attendu %v", letters, want)
	}
}

// §15.4 cas limite — une seule lettre pourvue ⇒ réponses consécutives de la même lettre.
func TestRafaleAlpha282_Draw_SingleLetter_StaysOnSameLetter(t *testing.T) {
	e := tw282Engine(t, []tw282Word{{"z1", "Zèbre"}, {"z2", "Zinc"}, {"z3", "Zoo"}}, true, 100)
	ids, letters := tw282Run(t, e, 3)
	if want := []string{"z1", "z2", "z3"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("ordre = %v, attendu %v", ids, want)
	}
	if want := []string{"Z", "Z", "Z"}; !reflect.DeepEqual(letters, want) {
		t.Errorf("lettres = %v, attendu %v (la roue ne bouge pas)", letters, want)
	}
}

// §15.4 cas limite — la passe k+1 commence à la première lettre encore pourvue, pas forcément A.
func TestRafaleAlpha282_Draw_NextPassStartsAtFirstPopulatedLetter(t *testing.T) {
	e := tw282Engine(t, []tw282Word{
		{"a1", "Ail"}, {"b1", "Banane"}, {"b2", "Bol"}, {"c1", "Cerise"}, {"c2", "Citron"},
	}, true, 100)
	ids, letters := tw282Run(t, e, 5)
	if want := []string{"a1", "b1", "c1", "b2", "c2"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("ordre = %v, attendu %v (la 2ᵉ passe commence à B, A est épuisée)", ids, want)
	}
	if want := []string{"A", "B", "C", "B", "C"}; !reflect.DeepEqual(letters, want) {
		t.Errorf("lettres = %v, attendu %v", letters, want)
	}
}

// §15.4 cas limite — `#` est tiré en fin de passe ; il n'apparaît jamais dans RAFALE_LETTERS.
func TestRafaleAlpha282_Draw_HashLetterAtEndOfPass_NeverInLetters(t *testing.T) {
	e := tw282Engine(t, []tw282Word{{"n1", "2024"}, {"z1", "Zèbre"}, {"a1", "Arbre"}}, true, 100)
	e.StartImmediate(0)
	defer e.Stop()
	var letters []string
	for i := 0; i < 3; i++ {
		letters = append(letters, tw282Letter(t, e))
		for _, l := range tw282Letters(t, e) {
			if l == "#" {
				t.Errorf("RAFALE_LETTERS ne doit jamais contenir « # » (A–Z seulement), got %v", tw282Letters(t, e))
			}
		}
		if i < 2 {
			if err := e.RafaleValidate(); err != nil {
				t.Fatalf("RafaleValidate : %v", err)
			}
		}
	}
	if want := []string{"A", "Z", "#"}; !reflect.DeepEqual(letters, want) {
		t.Errorf("lettres = %v, attendu %v (« # » en fin de passe)", letters, want)
	}
}

// §15.3 — normalisation observée : accents, casse, articles initiaux, ponctuation de tête, chiffre, vide.
func TestRafaleAlpha282_Normalization_ObservedLetters(t *testing.T) {
	cases := []struct{ answer, want string }{
		{"Élan", "E"}, {"élan", "E"}, {"L'Égypte", "E"}, {"L’Égypte", "E"}, {"La Seine", "S"},
		{"Les Misérables", "M"}, {"Lapin", "L"}, {"Un chat", "C"}, {"Une vache", "V"}, {"Des fleurs", "F"},
		{"Du pain", "P"}, {"De la Tour", "T"}, {"De l'eau", "E"}, {"« Ça va »", "C"}, {"(Zèbre)", "Z"},
		{"Œuf", "O"}, {"Ågren", "A"}, {"ñandu", "N"}, {"1984", "#"}, {"", "#"}, {"Ωmega", "#"},
		{"le", "L"}, // « le » seul : pas d'espace après l'article ⇒ non retiré
	}
	for i, c := range cases {
		c := c
		t.Run(fmt.Sprintf("%02d_%s", i, c.answer), func(t *testing.T) {
			e := tw282Engine(t, []tw282Word{{"w", c.answer}}, true, 100)
			e.StartImmediate(0)
			defer e.Stop()
			if got := tw282Letter(t, e); got != c.want {
				t.Errorf("lettre de %q = %q, attendu %q", c.answer, got, c.want)
			}
		})
	}
}

// §15.5 — RAFALE_LETTERS = lettres A–Z des réponses encore à jouer (file + pré-tirée), hors courante ;
// une lettre se grise dès que sa dernière réponse est devenue courante ; jamais null.
func TestRafaleAlpha282_Letters_ShrinkAsWordsArePlayed(t *testing.T) {
	e := tw282Engine(t, []tw282Word{{"a1", "Ail"}, {"b1", "Banane"}, {"c1", "Cerise"}}, true, 100)
	e.StartImmediate(0)
	defer e.Stop()

	steps := []struct {
		current string
		letters []string
	}{{"A", []string{"B", "C"}}, {"B", []string{"C"}}, {"C", []string{}}}
	for i, s := range steps {
		if got := tw282Letter(t, e); got != s.current {
			t.Errorf("pas %d : lettre courante = %q, attendu %q", i, got, s.current)
		}
		got := tw282Letters(t, e)
		sorted := append([]string(nil), got...)
		if len(sorted) != len(s.letters) || (len(sorted) > 0 && !reflect.DeepEqual(sorted, s.letters)) {
			t.Errorf("pas %d : RAFALE_LETTERS = %v, attendu %v", i, got, s.letters)
		}
		if i < len(steps)-1 {
			if err := e.RafaleValidate(); err != nil {
				t.Fatalf("RafaleValidate : %v", err)
			}
		}
	}
}

// §15.4 — `NEXT` annoncé à l'anim est exactement la prochaine entrée de la file (même fonction que le tirage).
func TestRafaleAlpha282_Next_MatchesNextQueueEntry(t *testing.T) {
	e := tw282Engine(t, []tw282Word{
		{"a1", "Ail"}, {"b1", "Banane"}, {"c1", "Cerise"}, {"a2", "Avion"},
	}, true, 100)
	var lastNext *RafaleNext
	e.OnRafaleAnswer = func(id, answer string, next *RafaleNext) { lastNext = next }
	e.StartImmediate(0)
	defer e.Stop()

	for i, wantNext := range []string{"b1", "c1", "a2"} {
		if lastNext == nil {
			t.Fatalf("pas %d : NEXT nil alors que la file contient encore %q", i, wantNext)
		}
		if lastNext.ID != wantNext {
			t.Fatalf("pas %d : NEXT annoncé = %q, attendu %q", i, lastNext.ID, wantNext)
		}
		if err := e.RafaleValidate(); err != nil {
			t.Fatalf("RafaleValidate : %v", err)
		}
		if got := tw282CurrentID(t, e); got != wantNext {
			t.Errorf("pas %d : la question courante (%q) doit être celle annoncée en NEXT (%q)", i, got, wantNext)
		}
	}
}

// §15.2/§15.4 — option désactivée : aucune lettre, tableau vide, comportement inchangé.
func TestRafaleAlpha282_OptionOff_NoLetters(t *testing.T) {
	e := tw282Engine(t, []tw282Word{{"a1", "Ail"}, {"b1", "Banane"}, {"c1", "Cerise"}}, false, 100)
	e.StartImmediate(0)
	defer e.Stop()
	if got := tw282Letter(t, e); got != "" {
		t.Errorf("option désactivée : RAFALE_CURRENT_LETTER doit être vide, got %q", got)
	}
	if got := tw282Letters(t, e); len(got) != 0 {
		t.Errorf("option désactivée : RAFALE_LETTERS doit être [], got %v", got)
	}
}

// §15.5 — hors manche (avant démarrage) : clés présentes, vides.
func TestRafaleAlpha282_BeforeStart_KeysPresentAndEmpty(t *testing.T) {
	e := NewEngine()
	if got := tw282Letter(t, e); got != "" {
		t.Errorf("RAFALE_CURRENT_LETTER initial = %q, attendu vide", got)
	}
	if got := tw282Letters(t, e); len(got) != 0 {
		t.Errorf("RAFALE_LETTERS initial = %v, attendu []", got)
	}
}

// §15.5 — vidage à l'arrêt de la manche.
func TestRafaleAlpha282_Stop_ClearsLetterState(t *testing.T) {
	e := tw282Engine(t, []tw282Word{{"a1", "Ail"}, {"b1", "Banane"}}, true, 100)
	e.StartImmediate(0)
	if tw282Letter(t, e) == "" {
		t.Fatal("setup invalide : la lettre courante devrait être posée")
	}
	e.Stop()
	if got := tw282Letter(t, e); got != "" {
		t.Errorf("après Stop : RAFALE_CURRENT_LETTER = %q, attendu vide", got)
	}
	if got := tw282Letters(t, e); len(got) != 0 {
		t.Errorf("après Stop : RAFALE_LETTERS = %v, attendu []", got)
	}
}

// §15.4 — plafond RAFALE_MAX_QUESTIONS : fin normale, la file est abandonnée (l'état est vidé).
func TestRafaleAlpha282_MaxQuestions_EndsRoundNormally_ClearsState(t *testing.T) {
	e := tw282Engine(t, []tw282Word{
		{"a1", "Ail"}, {"b1", "Banane"}, {"c1", "Cerise"}, {"d1", "Datte"}, {"e1", "Eau"},
	}, true, 2)
	e.StartImmediate(0)
	defer e.Stop()
	for i := 0; i < 2; i++ {
		if err := e.RafaleValidate(); err != nil {
			break // la manche peut se terminer à la 2ᵉ validation : c'est le comportement attendu
		}
	}
	st := tw282State(t, e)
	if sub, _ := st["RAFALE_SUBPHASE"].(string); strings.EqualFold(sub, "QUESTION") {
		t.Errorf("le plafond (2) est atteint : la manche ne doit plus être en QUESTION, got %v", sub)
	}
}

// §15.2 — carte MEMOTION RAFALE : l'option est ignorée, aucune lettre dans l'état global.
func TestRafaleAlpha282_MotionCard_IgnoresOption_NoLetter(t *testing.T) {
	e := NewEngine()
	seedRafaleReservoirCouple(t, e, "h", 5, CategoryHistory, 1)
	card := rafaleMotionCard("mc-alpha", []string{string(CategoryHistory)}, []int{1}, 3, 10)
	// L'option sur la carte est posée par JSON : elle doit être ignorée (ni exposée ni lue).
	raw, _ := json.Marshal(card)
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	m["RAFALE_ALPHABETICAL"] = true
	raw, _ = json.Marshal(m)
	var card2 MotionCard
	if err := json.Unmarshal(raw, &card2); err != nil {
		t.Fatalf("setup invalide : %v", err)
	}
	_, _, _ = startMEMOTIONAtRafaleCardQuestion(t, e, card2)
	defer e.Stop()

	if got := tw282Letter(t, e); got != "" {
		t.Errorf("carte MEMOTION RAFALE : RAFALE_CURRENT_LETTER doit rester vide (option ignorée), got %q", got)
	}
	if got := tw282Letters(t, e); len(got) != 0 {
		t.Errorf("carte MEMOTION RAFALE : RAFALE_LETTERS doit rester [], got %v", got)
	}
}

// Contrôle de harnais : l'option JSON est bien relue par le modèle (champ RAFALE_ALPHABETICAL existant).
func TestRafaleAlpha282_Model_OptionRoundTripsThroughJSON(t *testing.T) {
	q := tw282AlphaQuestion(t, true, 5)
	raw, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("marshal : %v", err)
	}
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	if m["RAFALE_ALPHABETICAL"] != true {
		t.Errorf("RAFALE_ALPHABETICAL doit survivre au round-trip JSON de Question, got %v", m["RAFALE_ALPHABETICAL"])
	}
	off := tw282AlphaQuestion(t, false, 5)
	raw, _ = json.Marshal(off)
	m = map[string]interface{}{}
	_ = json.Unmarshal(raw, &m)
	if _, present := m["RAFALE_ALPHABETICAL"]; present {
		t.Errorf("RAFALE_ALPHABETICAL=false doit être omis (omitempty §15.2), got %v", m["RAFALE_ALPHABETICAL"])
	}
}
