package main

// #245 (v11.2, lot V3-1) — ordre de jeu à deux niveaux (« Sans manche » en premier, puis les manches, ORDER puis ID),
// NEXT_QUESTION (ENTRACTE proposées, jouées sautées, NEW_ROUND), ROUND_CONTEXT, ROUND_ID inconnu, état sans manche (v11.1).
// Scénarios 1, 5, 6, 8, 12, 30, 33, 40. Parité JS / Go : fixture partagée server-go/tests/fixtures/rounds_order_245.json
// (EXPECTED_ORDER écrit à la main). `critical` : 1, 5, 12, 33.

import (
	"buzzcontrol/internal/game"
	"encoding/json"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"
)

type orderFixture245 struct {
	Rounds    []map[string]interface{} `json:"ROUNDS"`
	Questions []map[string]interface{} `json:"QUESTIONS"`
	Expected  []string                 `json:"EXPECTED_ORDER"`
}

func loadFixture245(t *testing.T) orderFixture245 {
	t.Helper()
	b, err := os.ReadFile("../../tests/fixtures/rounds_order_245.json")
	if err != nil {
		t.Fatal(err)
	}
	var f orderFixture245
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

// install : écrit les questions de la fixture, puis crée les manches par un ROUNDS_UPDATE… non : rounds.json direct (état de départ).
func (e *env245) installFixture(t *testing.T, f orderFixture245) {
	t.Helper()
	for _, q := range f.Questions {
		doc := map[string]interface{}{"QUESTION": "Question " + q["ID"].(string), "ANSWER": "A", "POINTS": "1", "TIME": "30", "CATEGORY": "CULTURE"}
		for k, v := range q {
			doc[k] = v
		}
		if q["TYPE"] == "ENTRACTE" {
			doc["ENTRACTE_CONFIG"] = map[string]interface{}{"TITLE": "Pause " + q["ID"].(string)}
		}
		e.writeRaw(t, q["ID"].(string), doc)
	}
	rf, _ := json.Marshal(map[string]interface{}{"ROUNDS": f.Rounds, "NEXT_ID": len(f.Rounds) + 1})
	if err := os.WriteFile(e.dir+"/rounds.json", rf, 0o644); err != nil {
		t.Fatal(err)
	}
}

// nextAfter : NEXT_QUESTION calculé par le serveur quand `current` est la question courante ("" = aucune).
func (e *env245) nextAfter(t *testing.T, current string) (id, roundID string, newRound bool, pos, total int) {
	t.Helper()
	if current == "" {
		e.app.engine.SetPhase(game.PhaseStopped)
		e.app.engine.Ready("", &game.Question{ID: "", Type: game.QuestionTypeSpeedy})
	} else {
		e.app.engine.SetPhase(game.PhaseStopped)
		e.app.engine.Ready("", &game.Question{ID: current, Type: game.QuestionTypeSpeedy})
	}
	p := e.app.getNextQuestionPayload()
	var raw map[string]interface{}
	b, _ := json.Marshal(p)
	_ = json.Unmarshal(b, &raw)
	id, _ = raw["ID"].(string)
	roundID, _ = raw["ROUND_ID"].(string)
	newRound, _ = raw["NEW_ROUND"].(bool)
	pf, _ := raw["CURRENT_POSITION"].(float64)
	tf, _ := raw["TOTAL_QUESTIONS"].(float64)
	return id, roundID, newRound, int(pf), int(tf)
}

// --- 12 + 5 + 33. Parité et ordre de jeu (critical) ---------------------------------------------------------------------------

func TestRoundsOrder245_Critical_GoPlayOrderMatchesTheSharedFixtureIncludingEntractes(t *testing.T) {
	f := loadFixture245(t)
	e := newEnv245(t)
	e.installFixture(t, f)

	// aucune question courante : la première entrée jouable
	if id, _, _, _, total := e.nextAfter(t, ""); id != f.Expected[0] || total != len(f.Expected) {
		t.Errorf("sans question courante : suivante = %q (total %d), attendu %q (%d)", id, total, f.Expected[0], len(f.Expected))
	}
	// de chaque entrée à la suivante : le déroulé complet, ENTRACTE comprises (33)
	for i := 0; i+1 < len(f.Expected); i++ {
		id, _, _, pos, _ := e.nextAfter(t, f.Expected[i])
		if id != f.Expected[i+1] {
			t.Errorf("après %s : suivante %q, attendu %q (parité avec l'ordre partagé)", f.Expected[i], id, f.Expected[i+1])
		}
		if pos != i+1 {
			t.Errorf("après %s : CURRENT_POSITION = %d (les ENTRACTE comptent comme positions du déroulé), attendu %d", f.Expected[i], pos, i+1)
		}
	}
	if id, _, _, _, _ := e.nextAfter(t, f.Expected[len(f.Expected)-1]); id != "" {
		t.Errorf("après la dernière entrée, plus de suivante : %q", id)
	}
}

func TestRoundsOrder245_NextQuestionOffersEntractesButSkipsPlayedEntries(t *testing.T) {
	f := loadFixture245(t)
	e := newEnv245(t)
	e.installFixture(t, f)

	// 31 → 30 → 32 (ENTRACTE de fin de manche 1) : proposée, avec son type
	id, _, _, _, _ := e.nextAfter(t, "30")
	if id != "32" {
		t.Fatalf("après 30, l'ENTRACTE de fin de manche est proposée (plus sautée) : %q", id)
	}
	if p := e.app.getNextQuestionPayload(); p.Type != "ENTRACTE" || p.Question != "Question 32" {
		t.Errorf("TYPE ENTRACTE et libellé : %+v", p)
	}
	// une entrée déjà jouée reste sautée
	e.app.engine.SetPhase(game.PhaseStopped)
	e.app.engine.Ready("", &game.Question{ID: "32", Type: game.QuestionTypeSpeedy})
	e.app.engine.StartImmediate(5)
	e.app.engine.Stop() // 32 est maintenant « jouée » (STOPPED)
	if id, _, _, _, _ := e.nextAfter(t, "30"); id != "21" {
		t.Errorf("l'ENTRACTE 32 est jouée : sautée, on passe à 21 (début de manche 2) : %q", id)
	}
}

func TestRoundsOrder245_NewRoundIsTrueWhenTheNextEntryOpensAnotherRound(t *testing.T) {
	f := loadFixture245(t)
	e := newEnv245(t)
	e.installFixture(t, f)

	if _, rid, nr, _, _ := e.nextAfter(t, "61"); rid != "r1" || !nr { // dernier de « Sans manche » → 31 (manche r1)
		t.Errorf("61 → 31 : ROUND_ID r1, NEW_ROUND true : %q %v", rid, nr)
	}
	if _, rid, nr, _, _ := e.nextAfter(t, "31"); rid != "r1" || nr { // même manche
		t.Errorf("31 → 30 : même manche, NEW_ROUND false : %q %v", rid, nr)
	}
	if _, rid, nr, _, _ := e.nextAfter(t, "32"); rid != "r2" || !nr { // r1 → r2 (21)
		t.Errorf("32 → 21 : ROUND_ID r2, NEW_ROUND true : %q %v", rid, nr)
	}
	if p := e.app.getNextQuestionPayload(); p == nil {
		t.Fatal("payload")
	}
	b, _ := json.Marshal(e.app.getNextQuestionPayload())
	var raw map[string]interface{}
	_ = json.Unmarshal(b, &raw)
	if raw["ROUND_NAME"] != "Deux" {
		t.Errorf("ROUND_NAME = %v, attendu « Deux »", raw["ROUND_NAME"])
	}
}

// --- 1. Sans manche : strictement v11.1 (critical) -----------------------------------------------------------------------------

func TestRoundsOrder245_Critical_WithoutRoundsFileTheOrderAndStateAreExactlyV111(t *testing.T) {
	e := newEnv245(t)
	// 30 questions : ORDER distincts, certains absents (repli sur l'ID), IDs non contigus
	type q struct {
		id    string
		order *int
	}
	var qs []q
	for i := 0; i < 30; i++ {
		id := strconv.Itoa(100 - i*3)
		if i%7 == 0 {
			qs = append(qs, q{id, nil})
		} else {
			o := 200 + (i*7)%31 // ORDER tous distincts : pas d'ambiguïté d'égalité (l'ancien tri n'en tranchait aucune)
			qs = append(qs, q{id, &o})
		}
	}
	for _, x := range qs {
		doc := map[string]interface{}{"ID": x.id, "TYPE": "SPEEDY", "QUESTION": "Q" + x.id, "POINTS": "1", "TIME": "30"}
		if x.order != nil {
			doc["ORDER"] = *x.order
		}
		e.writeRaw(t, x.id, doc)
	}
	// ordre v11.1 : ORDER (repli ID), puis ID numérique
	type key struct {
		id    string
		order int
	}
	var keys []key
	for _, x := range qs {
		o, _ := strconv.Atoi(x.id)
		if x.order != nil {
			o = *x.order
		}
		keys = append(keys, key{x.id, o})
	}
	sort.SliceStable(keys, func(i, j int) bool {
		if keys[i].order != keys[j].order {
			return keys[i].order < keys[j].order
		}
		a, _ := strconv.Atoi(keys[i].id)
		b, _ := strconv.Atoi(keys[j].id)
		return a < b
	})

	for i := 0; i+1 < len(keys); i++ {
		if id, _, _, _, total := e.nextAfter(t, keys[i].id); id != keys[i+1].id || total != len(keys) {
			t.Fatalf("après %s : %q (total %d), attendu %q — ordre v11.1 inchangé sans manche", keys[i].id, id, total, keys[i+1].id)
		}
	}

	st := e.app.engine.GetGameJSON()
	var g struct {
		Game struct {
			Rounds *[]interface{} `json:"ROUNDS"`
			Ctx    *struct {
				RoundID    string `json:"ROUND_ID"`
				RoundIndex int    `json:"ROUND_INDEX"`
				RoundCount int    `json:"ROUND_COUNT"`
				Position   int    `json:"POSITION"`
				Total      int    `json:"TOTAL"`
				First      bool   `json:"FIRST_OF_ROUND"`
			} `json:"ROUND_CONTEXT"`
		} `json:"GAME"`
	}
	_ = json.Unmarshal(st, &g)
	if g.Game.Rounds == nil || len(*g.Game.Rounds) != 0 {
		t.Errorf("GAME.ROUNDS toujours sérialisé, [] sans rounds.json : %v", g.Game.Rounds)
	}
	if g.Game.Ctx == nil || g.Game.Ctx.RoundID != "" || g.Game.Ctx.RoundIndex != 0 || g.Game.Ctx.RoundCount != 0 || g.Game.Ctx.Position != 0 || g.Game.Ctx.Total != 0 || g.Game.Ctx.First {
		t.Errorf("GAME.ROUND_CONTEXT toujours sérialisé, tout à zéro sans manche : %+v", g.Game.Ctx)
	}
}

// --- 6 + 30. ROUND_CONTEXT au READY ----------------------------------------------------------------------------------------------

func (e *env245) readyAndContext(t *testing.T, id string) (ctx struct {
	RoundID    string `json:"ROUND_ID"`
	RoundIndex int    `json:"ROUND_INDEX"`
	RoundCount int    `json:"ROUND_COUNT"`
	Position   int    `json:"POSITION"`
	Total      int    `json:"TOTAL"`
	First      bool   `json:"FIRST_OF_ROUND"`
}) {
	t.Helper()
	e.app.engine.SetPhase(game.PhaseStopped)
	e.send(t, e.admin1, "admin", "READY", map[string]interface{}{"QUESTION": id})
	time.Sleep(60 * time.Millisecond)
	var g struct {
		Game struct {
			Ctx struct {
				RoundID    string `json:"ROUND_ID"`
				RoundIndex int    `json:"ROUND_INDEX"`
				RoundCount int    `json:"ROUND_COUNT"`
				Position   int    `json:"POSITION"`
				Total      int    `json:"TOTAL"`
				First      bool   `json:"FIRST_OF_ROUND"`
			} `json:"ROUND_CONTEXT"`
		} `json:"GAME"`
	}
	_ = json.Unmarshal(e.app.engine.GetGameJSON(), &g)
	return g.Game.Ctx
}

func TestRoundsOrder245_RoundContextAtReadyCountsQuestionsOnlyAndFlagsTheFirstNonEntracte(t *testing.T) {
	f := loadFixture245(t)
	e := newEnv245(t)
	e.installFixture(t, f)
	type want struct {
		round         string
		idx, pos, tot int
		first         bool
	}
	for id, w := range map[string]want{
		"31": {"r1", 1, 1, 2, true}, // première question de r1
		"30": {"r1", 1, 2, 2, false},
		"32": {"r1", 1, 0, 2, false}, // ENTRACTE : POSITION 0, jamais FIRST_OF_ROUND
		"21": {"r2", 2, 0, 2, false}, // entracte de début : n'est pas la « première »
		"22": {"r2", 2, 1, 2, true},  // première question après l'entracte de début
		"20": {"r2", 2, 2, 2, false},
	} {
		c := e.readyAndContext(t, id)
		if c.RoundID != w.round || c.RoundIndex != w.idx || c.RoundCount != 2 || c.Position != w.pos || c.Total != w.tot || c.First != w.first {
			t.Errorf("READY %s : %+v, attendu manche %s index %d/2 position %d/%d premier=%v", id, c, w.round, w.idx, w.pos, w.tot, w.first)
		}
	}
	// « Sans manche » : index 0, ID vide
	if c := e.readyAndContext(t, "12"); c.RoundID != "" || c.RoundIndex != 0 {
		t.Errorf("question sans manche : ROUND_ID \"\" et ROUND_INDEX 0 : %+v", c)
	}
}

func TestRoundsOrder245_FirstOfRoundIsTrueOnTheFirstQuestionOfARoundWithoutEntractes(t *testing.T) {
	e := newEnv245(t)
	e.writeQ(t, "1", 0, "r1", "")
	e.writeQ(t, "2", 1, "r1", "")
	if err := os.WriteFile(e.dir+"/rounds.json", []byte(`{"ROUNDS":[{"ID":"r1","NAME":"Seule"}],"NEXT_ID":2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if c := e.readyAndContext(t, "1"); !c.First || c.Position != 1 || c.Total != 2 {
		t.Errorf("manche sans entractes : la première question est FIRST_OF_ROUND : %+v", c)
	}
}

// --- 8. ROUND_ID inconnu -----------------------------------------------------------------------------------------------------------

func TestRoundsOrder245_UnknownRoundIDIsTreatedAsNoRoundWithoutError(t *testing.T) {
	e := newEnv245(t)
	e.writeQ(t, "1", 0, "", "")
	e.writeQ(t, "2", 1, "r9", "") // manche absente (restauration partielle)
	if err := os.WriteFile(e.dir+"/rounds.json", []byte(`{"ROUNDS":[{"ID":"r1","NAME":"Un"}],"NEXT_ID":2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if id, _, nr, _, _ := e.nextAfter(t, "1"); id != "2" || nr {
		t.Errorf("la question de manche inconnue suit « Sans manche » sans changement de manche : %q %v", id, nr)
	}
	if c := e.readyAndContext(t, "2"); c.RoundID != "" || c.RoundIndex != 0 {
		t.Errorf("ROUND_CONTEXT d'une manche inconnue = Sans manche : %+v", c)
	}
}
