package main

// #245 (v11.2, lot V3-1) — manches : ROUNDS_UPDATE (création avec entractes automatiques, renommage, déplacements,
// suppression KEEP/DELETE, refus atomiques), rounds.json, ROUND_ID/ORDER des fichiers, allow-list et LAUNCH_LOCK.
// Contrats : websocket-actions.md §ROUNDS_UPDATE (a6aeb193, a50b9f32, b77f5314), models.md §Manches, game-state.md §ROUNDS.
// Scénarios 2, 3, 4, 11, 22-24, 26-29 (planner-20261012-200000.md, -223000.md). `critical` : 2, 26, 27, 28.
// Boîte noire : actions désignées par leur nom de protocole (littéraux), état lu sur les fichiers et les trames.

import (
	"buzzcontrol/internal/game"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

type env245 struct {
	*env253
	dir string
}

func newEnv245(t *testing.T) *env245 {
	t.Helper()
	e := buildEnv253(t, false, true)
	dir := t.TempDir()
	e.app.config.Storage.QuestionsDir = dir
	return &env245{env253: e, dir: dir}
}

// writeQ245 crée un dossier de question ; qtype vide = SPEEDY ; round vide = sans ROUND_ID.
func (e *env245) writeQ(t *testing.T, id string, order int, round, qtype string) {
	t.Helper()
	if qtype == "" {
		qtype = "SPEEDY"
	}
	q := map[string]interface{}{"ID": id, "TYPE": qtype, "QUESTION": "Question " + id, "ANSWER": "A", "POINTS": "1", "TIME": "30", "CATEGORY": "CULTURE", "ORDER": order}
	if round != "" {
		q["ROUND_ID"] = round
	}
	if qtype == "ENTRACTE" {
		q["ENTRACTE_CONFIG"] = map[string]interface{}{"TITLE": "Pause " + id, "SUBTITLE": ""}
	}
	e.writeRaw(t, id, q)
}

func (e *env245) writeRaw(t *testing.T, id string, q map[string]interface{}) {
	t.Helper()
	d := filepath.Join(e.dir, id)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(q)
	if err := os.WriteFile(filepath.Join(d, "question.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// aged245 : met tous les fichiers à une date ancienne pour détecter ensuite les réécritures (sans attente).
func (e *env245) aged(t *testing.T) time.Time {
	t.Helper()
	old := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	_ = filepath.Walk(e.dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			_ = os.Chtimes(p, old, old)
		}
		return nil
	})
	return old
}

func (e *env245) question(t *testing.T, id string) map[string]interface{} {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(e.dir, id, "question.json"))
	if err != nil {
		t.Fatalf("question %s : %v", id, err)
	}
	var q map[string]interface{}
	_ = json.Unmarshal(b, &q)
	return q
}

func (e *env245) exists(id string) bool {
	_, err := os.Stat(filepath.Join(e.dir, id, "question.json"))
	return err == nil
}

func (e *env245) roundsFile(t *testing.T) (rounds []map[string]interface{}, nextID float64, present bool) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(e.dir, "rounds.json"))
	if err != nil {
		return nil, 0, false
	}
	var f struct {
		Rounds []map[string]interface{} `json:"ROUNDS"`
		NextID float64                  `json:"NEXT_ID"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("rounds.json illisible : %v", err)
	}
	return f.Rounds, f.NextID, true
}

// snapshot245 : contenu de tous les fichiers du dossier des questions (chemin → contenu).
func (e *env245) snapshot(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	_ = filepath.Walk(e.dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			b, _ := os.ReadFile(p)
			rel, _ := filepath.Rel(e.dir, p)
			out[rel] = string(b)
		}
		return nil
	})
	return out
}

func sameSnapshot245(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

type roundSpec245 struct {
	ID        string   `json:"ID,omitempty"`
	Name      string   `json:"NAME"`
	Questions []string `json:"QUESTIONS"`
	Entractes *bool    `json:"ENTRACTES,omitempty"`
}

type deletedRound245 struct {
	ID        string `json:"ID"`
	Questions string `json:"QUESTIONS"`
}

func (e *env245) update(t *testing.T, from string, rounds []roundSpec245, unassigned []string, deleted []deletedRound245) {
	t.Helper()
	p := map[string]interface{}{"ROUNDS": rounds, "UNASSIGNED": unassigned}
	if rounds == nil {
		p["ROUNDS"] = []roundSpec245{}
	}
	if unassigned == nil {
		p["UNASSIGNED"] = []string{}
	}
	if deleted != nil {
		p["DELETED_ROUNDS"] = deleted
	}
	e.send(t, from, "admin", "ROUNDS_UPDATE", p)
}

type roundsResult245 struct {
	Status string `json:"STATUS"`
	Reason string `json:"REASON"`
	Rounds []struct {
		ID   string `json:"ID"`
		Name string `json:"NAME"`
	} `json:"ROUNDS"`
	Created []struct {
		RoundID string `json:"ROUND_ID"`
		StartID string `json:"START_ID"`
		EndID   string `json:"END_ID"`
	} `json:"CREATED_ENTRACTES"`
	Deleted []string `json:"DELETED_QUESTIONS"`
}

func (e *env245) result(t *testing.T) roundsResult245 {
	t.Helper()
	m, ok := waitFrame253(e.a1, "ROUNDS_UPDATE_RESULT", nil)
	if !ok {
		t.Fatal("ROUNDS_UPDATE_RESULT attendu à l'émetteur")
	}
	var r roundsResult245
	_ = json.Unmarshal(m, &r)
	return r
}

func orderOf245(t *testing.T, e *env245, id string) int {
	t.Helper()
	switch v := e.question(t, id)["ORDER"].(type) {
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return -1
}

func roundOf245(q map[string]interface{}) string {
	s, _ := q["ROUND_ID"].(string)
	return s
}

func boolp(b bool) *bool { return &b }

// base245 : quatre questions sans manche (ORDER 0..3 = ids 1..4).
func (e *env245) base(t *testing.T) {
	t.Helper()
	for i, id := range []string{"1", "2", "3", "4"} {
		e.writeQ(t, id, i, "", "")
	}
}

// --- 2. ROUNDS_UPDATE valide (critical) ----------------------------------------------------------------------------------

func TestRoundsUpdate245_Critical_ValidUpdateWritesRoundsJsonAndOnlyTheChangedFiles(t *testing.T) {
	e := newEnv245(t)
	e.writeQ(t, "1", 5, "", "")
	e.writeQ(t, "2", 6, "", "")
	e.writeQ(t, "3", 0, "", "") // déjà en position 0 de « Sans manche »
	e.writeQ(t, "4", 1, "", "")
	old := e.aged(t)

	e.update(t, e.admin1, []roundSpec245{{Name: "Échauffement", Questions: []string{"1", "2"}, Entractes: boolp(false)}, {Name: "Finale", Questions: nil, Entractes: boolp(false)}}, []string{"3", "4"}, nil)

	r := e.result(t)
	if r.Status != "APPLIED" || len(r.Rounds) != 2 || r.Rounds[0].ID != "r1" || r.Rounds[1].ID != "r2" || r.Rounds[0].Name != "Échauffement" {
		t.Fatalf("RESULT = %+v", r)
	}
	rounds, next, present := e.roundsFile(t)
	if !present || len(rounds) != 2 || rounds[0]["ID"] != "r1" || rounds[1]["ID"] != "r2" || next != 3 {
		t.Errorf("rounds.json = {ROUNDS:%v, NEXT_ID:%v} (r1, r2, NEXT_ID 3) présent=%v", rounds, next, present)
	}
	for id, want := range map[string]struct {
		round string
		order int
	}{"1": {"r1", 0}, "2": {"r1", 1}, "3": {"", 0}, "4": {"", 1}} {
		q := e.question(t, id)
		if roundOf245(q) != want.round || orderOf245(t, e, id) != want.order {
			t.Errorf("question %s : ROUND_ID=%q ORDER=%d, attendu %q/%d", id, roundOf245(q), orderOf245(t, e, id), want.round, want.order)
		}
	}
	// seuls les fichiers qui changent sont réécrits
	for _, id := range []string{"3", "4"} {
		info, _ := os.Stat(filepath.Join(e.dir, id, "question.json"))
		if !info.ModTime().Equal(old) {
			t.Errorf("la question %s n'a pas changé : son fichier ne doit pas être réécrit", id)
		}
	}
	for _, id := range []string{"1", "2"} {
		info, _ := os.Stat(filepath.Join(e.dir, id, "question.json"))
		if info.ModTime().Equal(old) {
			t.Errorf("la question %s a changé : son fichier est réécrit", id)
		}
	}
	// diffusions : UPDATE (GAME.ROUNDS) à l'admin, QUESTIONS à l'admin, NEXT_QUESTION à l'anim
	time.Sleep(300 * time.Millisecond)
	adminFrames := tapOf253(e.a2).take()
	if !strings.Contains(strings.Join(adminFrames, ""), `"ROUNDS":[{`) {
		t.Errorf("UPDATE avec GAME.ROUNDS diffusé aux admins")
	}
	if countAction253(adminFrames, "QUESTIONS") == 0 {
		t.Error("QUESTIONS diffusé aux admins")
	}
	if countAction253(tapOf253(e.an).take(), "NEXT_QUESTION") == 0 {
		t.Error("NEXT_QUESTION diffusé à l'anim")
	}
	if !e.hasLog(game.LogLevelInfo, "ROUNDS_UPDATE") {
		t.Errorf("INFO ROUNDS_UPDATE ; logs=%v", e.logLines(""))
	}
}

func TestRoundsUpdate245_IdenticalSecondUpdateRewritesNothing(t *testing.T) {
	e := newEnv245(t)
	e.base(t)
	rs := []roundSpec245{{Name: "R", Questions: []string{"1", "2"}, Entractes: boolp(false)}}
	e.update(t, e.admin1, rs, []string{"3", "4"}, nil)
	_ = e.result(t)
	old := e.aged(t)

	e.update(t, e.admin1, []roundSpec245{{ID: "r1", Name: "R", Questions: []string{"1", "2"}}}, []string{"3", "4"}, nil)

	if r := e.result(t); r.Status != "APPLIED" {
		t.Fatalf("%+v", r)
	}
	for id := range e.snapshot(t) {
		info, _ := os.Stat(filepath.Join(e.dir, id))
		if id != "rounds.json" && !info.ModTime().Equal(old) {
			t.Errorf("%s réécrit alors que rien ne change", id)
		}
	}
}

// --- 3 + 28. Refus : rien n'est écrit (critical) -----------------------------------------------------------------------------

func TestRoundsUpdate245_Critical_EveryRefusalWritesAndDeletesNothing(t *testing.T) {
	long := strings.Repeat("x", 61)
	tooMany := make([]roundSpec245, 51)
	for i := range tooMany {
		tooMany[i] = roundSpec245{Name: "M" + strconv.Itoa(i), Entractes: boolp(false)}
	}
	valid := func() ([]roundSpec245, []string) {
		return []roundSpec245{{ID: "r1", Name: "Un", Questions: []string{"1", "2"}}, {ID: "r2", Name: "Deux", Questions: []string{"3"}}}, []string{"4"}
	}
	cases := []struct {
		name   string
		build  func() ([]roundSpec245, []string, []deletedRound245)
		reason string
	}{
		{"nom vide", func() ([]roundSpec245, []string, []deletedRound245) {
			r, u := valid()
			r[0].Name = "  "
			return r, u, nil
		}, "INVALID_NAME"},
		{"nom de 61 caractères", func() ([]roundSpec245, []string, []deletedRound245) {
			r, u := valid()
			r[0].Name = long
			return r, u, nil
		}, "INVALID_NAME"},
		{"51 manches", func() ([]roundSpec245, []string, []deletedRound245) {
			return tooMany, []string{"1", "2", "3", "4"}, nil
		}, "TOO_MANY_ROUNDS"},
		{"manche inconnue", func() ([]roundSpec245, []string, []deletedRound245) {
			r, u := valid()
			r[1].ID = "r99"
			return r, u, nil
		}, "UNKNOWN_ROUND"},
		{"manche supprimée inconnue", func() ([]roundSpec245, []string, []deletedRound245) {
			r, u := valid()
			return r, u, []deletedRound245{{ID: "r99", Questions: "KEEP"}}
		}, "UNKNOWN_ROUND"},
		{"question en double", func() ([]roundSpec245, []string, []deletedRound245) {
			r, u := valid()
			r[1].Questions = []string{"3", "1"}
			return r, u, nil
		}, "DUPLICATE_QUESTION"},
		{"question inconnue", func() ([]roundSpec245, []string, []deletedRound245) {
			r, u := valid()
			r[1].Questions = []string{"3", "999"}
			return r, u, nil
		}, "UNKNOWN_QUESTION"},
		{"question oubliée", func() ([]roundSpec245, []string, []deletedRound245) {
			r, _ := valid()
			return r, []string{}, nil
		}, "INCOMPLETE"},
		{"manche sans décision", func() ([]roundSpec245, []string, []deletedRound245) {
			r, u := valid()
			return r[:1], append(u, "3"), nil
		}, "MISSING_ROUND_DECISION"},
		{"décision inconnue", func() ([]roundSpec245, []string, []deletedRound245) {
			r, u := valid()
			return r[:1], append(u, "3"), []deletedRound245{{ID: "r2", Questions: "MAYBE"}}
		}, "INVALID_DECISION"},
		{"manche dans les deux listes", func() ([]roundSpec245, []string, []deletedRound245) {
			r, u := valid()
			return r, u, []deletedRound245{{ID: "r2", Questions: "KEEP"}}
		}, "INVALID_DECISION"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv245(t)
			e.base(t)
			e.update(t, e.admin1, []roundSpec245{{Name: "Un", Questions: []string{"1", "2"}, Entractes: boolp(false)}, {Name: "Deux", Questions: []string{"3"}, Entractes: boolp(false)}}, []string{"4"}, nil)
			if r := e.result(t); r.Status != "APPLIED" {
				t.Fatalf("précondition : %+v", r)
			}
			before := e.snapshot(t)

			rounds, unassigned, deleted := c.build()
			e.update(t, e.admin1, rounds, unassigned, deleted)

			r := e.result(t)
			if r.Status != "REFUSED" || r.Reason != c.reason {
				t.Fatalf("RESULT = %+v, attendu REFUSED/%s", r, c.reason)
			}
			if !sameSnapshot245(before, e.snapshot(t)) {
				t.Error("un refus global n'écrit ni ne supprime rien (fichiers de questions et rounds.json inchangés)")
			}
			if !e.hasLog(game.LogLevelWarn, "ROUNDS_UPDATE_REFUSED") {
				t.Errorf("WARN ROUNDS_UPDATE_REFUSED ; logs=%v", e.logLines(game.LogLevelWarn))
			}
		})
	}
}

// --- 22-24. Entractes automatiques -----------------------------------------------------------------------------------------

func TestRoundsUpdate245_NewRoundCreatesTwoStandardEntractesByDefault(t *testing.T) {
	e := newEnv245(t)
	e.base(t)

	e.update(t, e.admin1, []roundSpec245{{Name: "Finale", Questions: []string{"1", "2"}}}, []string{"3", "4"}, nil) // ENTRACTES absent = true

	r := e.result(t)
	if r.Status != "APPLIED" || len(r.Created) != 1 || r.Created[0].RoundID != "r1" || r.Created[0].StartID == "" || r.Created[0].EndID == "" {
		t.Fatalf("RESULT = %+v (CREATED_ENTRACTES [{ROUND_ID r1, START_ID, END_ID}])", r)
	}
	start, end := e.question(t, r.Created[0].StartID), e.question(t, r.Created[0].EndID)
	for name, c := range map[string]struct {
		q     map[string]interface{}
		title string
	}{"début": {start, "Début de la manche Finale"}, "fin": {end, "Fin de la manche Finale"}} {
		cfg, _ := c.q["ENTRACTE_CONFIG"].(map[string]interface{})
		if c.q["TYPE"] != "ENTRACTE" || c.q["QUESTION"] != c.title || cfg["TITLE"] != c.title || cfg["SUBTITLE"] != "" || roundOf245(c.q) != "r1" {
			t.Errorf("entracte de %s : ENTRACTE standard, QUESTION et TITLE « %s », SUBTITLE vide, ROUND_ID r1 : %v", name, c.title, c.q)
		}
		if _, hasMarker := c.q["ROUND_ROLE"]; hasMarker {
			t.Errorf("aucun marqueur particulier sur une entracte automatique : %v", c.q)
		}
	}
	if !(orderOf245(t, e, r.Created[0].StartID) < orderOf245(t, e, "1") && orderOf245(t, e, "1") < orderOf245(t, e, "2") && orderOf245(t, e, "2") < orderOf245(t, e, r.Created[0].EndID)) {
		t.Errorf("début en tête, fin à la fin de la manche (ORDER %d < %d < %d < %d)", orderOf245(t, e, r.Created[0].StartID), orderOf245(t, e, "1"), orderOf245(t, e, "2"), orderOf245(t, e, r.Created[0].EndID))
	}
}

func TestRoundsUpdate245_EntractesFalseCreatesNoneAndExistingRoundIgnoresTheFlag(t *testing.T) {
	e := newEnv245(t)
	e.base(t)
	e.update(t, e.admin1, []roundSpec245{{Name: "Sans", Questions: []string{"1"}, Entractes: boolp(false)}}, []string{"2", "3", "4"}, nil)
	r := e.result(t)
	if r.Status != "APPLIED" || len(r.Created) != 0 || len(e.snapshot(t)) != 5 { // 4 questions + rounds.json
		t.Fatalf("ENTRACTES:false → aucune entracte : %+v (%d fichiers)", r, len(e.snapshot(t)))
	}
	before := len(e.snapshot(t))

	e.update(t, e.admin1, []roundSpec245{{ID: "r1", Name: "Sans", Questions: []string{"1"}, Entractes: boolp(true)}}, []string{"2", "3", "4"}, nil)

	if r := e.result(t); r.Status != "APPLIED" || len(r.Created) != 0 || len(e.snapshot(t)) != before {
		t.Errorf("ENTRACTES ignoré pour une manche existante : %+v", r)
	}
}

func TestRoundsUpdate245_RenamingARoundDoesNotRenameItsEntractes(t *testing.T) {
	e := newEnv245(t)
	e.base(t)
	e.update(t, e.admin1, []roundSpec245{{Name: "Ancien", Questions: []string{"1"}}}, []string{"2", "3", "4"}, nil)
	created := e.result(t).Created[0]

	e.update(t, e.admin1, []roundSpec245{{ID: "r1", Name: "Nouveau", Questions: []string{created.StartID, "1", created.EndID}}}, []string{"2", "3", "4"}, nil)

	if r := e.result(t); r.Status != "APPLIED" || r.Rounds[0].Name != "Nouveau" {
		t.Fatalf("%+v", r)
	}
	if q := e.question(t, created.StartID); q["QUESTION"] != "Début de la manche Ancien" {
		t.Errorf("renommer la manche ne renomme pas ses entractes : %v", q["QUESTION"])
	}
}

// --- 4 + 26. Suppression KEEP (critical) ------------------------------------------------------------------------------------

func TestRoundsUpdate245_Critical_DeleteWithKeepMovesEverythingToTheEndOfNoRoundWithoutDeletingAFile(t *testing.T) {
	e := newEnv245(t)
	e.base(t)
	e.update(t, e.admin1, []roundSpec245{{Name: "A supprimer", Questions: []string{"1", "2"}}}, []string{"3", "4"}, nil)
	created := e.result(t).Created[0]
	filesBefore := len(e.snapshot(t))

	e.update(t, e.admin1, nil, []string{"3", "4"}, []deletedRound245{{ID: "r1", Questions: "KEEP"}})

	r := e.result(t)
	if r.Status != "APPLIED" || len(r.Deleted) != 0 {
		t.Fatalf("RESULT = %+v", r)
	}
	if rounds, _, _ := e.roundsFile(t); len(rounds) != 0 {
		t.Errorf("la manche est retirée de rounds.json : %v", rounds)
	}
	if len(e.snapshot(t)) != filesBefore {
		t.Error("KEEP : aucun fichier supprimé")
	}
	// ordre dans « Sans manche » : 3, 4 (en place), puis, dans leur ordre d'origine, début, 1, 2, fin
	want := []string{"3", "4", created.StartID, "1", "2", created.EndID}
	for i, id := range want {
		if roundOf245(e.question(t, id)) != "" {
			t.Errorf("question %s : ROUND_ID retiré", id)
		}
		if got := orderOf245(t, e, id); got != i {
			t.Errorf("question %s : ORDER = %d, attendu %d (fin du groupe « Sans manche », ordre d'origine)", id, got, i)
		}
	}
}

// --- 27. Suppression DELETE (critical) ---------------------------------------------------------------------------------------

func TestRoundsUpdate245_Critical_DeleteWithDeleteRemovesQuestionsFoldersAndBroadcasts(t *testing.T) {
	e := newEnv245(t)
	e.base(t)
	e.update(t, e.admin1, []roundSpec245{{Name: "A supprimer", Questions: []string{"1", "2"}}}, []string{"3", "4"}, nil)
	created := e.result(t).Created[0]
	if err := os.WriteFile(filepath.Join(e.dir, "1", "media.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	tapOf253(e.a2).take()
	tapOf253(e.an).take()

	e.update(t, e.admin1, nil, []string{"3", "4"}, []deletedRound245{{ID: "r1", Questions: "DELETE"}})

	r := e.result(t)
	sort.Strings(r.Deleted)
	wantDeleted := []string{"1", "2", created.EndID, created.StartID}
	sort.Strings(wantDeleted)
	if r.Status != "APPLIED" || strings.Join(r.Deleted, ",") != strings.Join(wantDeleted, ",") {
		t.Fatalf("DELETED_QUESTIONS = %v, attendu %v (%+v)", r.Deleted, wantDeleted, r)
	}
	for _, id := range wantDeleted {
		if _, err := os.Stat(filepath.Join(e.dir, id)); err == nil {
			t.Errorf("le dossier de la question %s (médias compris) est supprimé", id)
		}
	}
	for _, id := range []string{"3", "4"} {
		if !e.exists(id) {
			t.Errorf("la question %s hors manche est conservée", id)
		}
	}
	time.Sleep(300 * time.Millisecond)
	if countAction253(tapOf253(e.a2).take(), "QUESTIONS") == 0 || countAction253(tapOf253(e.an).take(), "NEXT_QUESTION") == 0 {
		t.Error("QUESTIONS (admin) et NEXT_QUESTION (anim) diffusés")
	}
}

// --- 29. IDs presque épuisés -----------------------------------------------------------------------------------------------------

func TestRoundsUpdate245_NearlyExhaustedIDsRefuseTheCreationAndCreateNothing(t *testing.T) {
	e := newEnv245(t)
	for i := 1; i <= 998; i++ {
		e.writeQ(t, strconv.Itoa(i), i, "", "")
	}
	before := e.snapshot(t)
	unassigned := make([]string, 0, 998)
	for i := 1; i <= 998; i++ {
		unassigned = append(unassigned, strconv.Itoa(i))
	}

	e.update(t, e.admin1, []roundSpec245{{Name: "Trop tard", Questions: nil}}, unassigned, nil) // 2 entractes à créer, 1 ID libre

	r := e.result(t)
	if r.Status != "REFUSED" || r.Reason != "QUESTION_ID_EXHAUSTED" {
		t.Fatalf("RESULT = %+v", r)
	}
	if !sameSnapshot245(before, e.snapshot(t)) {
		t.Error("aucune manche créée, aucun fichier écrit")
	}
}

// --- 11. Allow-list et LAUNCH_LOCK -------------------------------------------------------------------------------------------------

func TestRoundsUpdate245_AdminOnlyAndStillAllowedWhileTheLaunchPopupIsOpen(t *testing.T) {
	e := newEnv245(t)
	e.base(t)
	before := e.snapshot(t)
	payload := map[string]interface{}{"ROUNDS": []roundSpec245{{Name: "X", Questions: []string{"1"}, Entractes: boolp(false)}}, "UNASSIGNED": []string{"2", "3", "4"}}

	e.send(t, e.anim, "anim", "ROUNDS_UPDATE", payload)
	e.send(t, e.tv, "tv", "ROUNDS_UPDATE", payload)
	e.send(t, e.paulCID, "vplayer", "ROUNDS_UPDATE", payload)
	time.Sleep(200 * time.Millisecond)
	if !sameSnapshot245(before, e.snapshot(t)) {
		t.Error("ROUNDS_UPDATE est refusé pour anim, TV et VJoueur (admin seulement)")
	}
	if !e.hasLog(game.LogLevelWarn, "Rejected action ROUNDS_UPDATE") {
		t.Errorf("le rejet est journalisé ; logs=%v", e.logLines(game.LogLevelWarn))
	}

	e.openLock(t, e.admin1)
	e.waitLock(func(l lockState253) bool { return l.Active })
	e.send(t, e.admin2, "admin", "ROUNDS_UPDATE", payload)
	m, ok := waitFrame253(e.a2, "ROUNDS_UPDATE_RESULT", nil)
	var r roundsResult245
	_ = json.Unmarshal(m, &r)
	if !ok || r.Status != "APPLIED" {
		t.Errorf("l'édition du quiz reste permise pendant LAUNCH_LOCK : %+v", r)
	}
	if e.countLog(game.LogLevelWarn, "ACTION_REFUSED_LAUNCH") != 0 {
		t.Error("ROUNDS_UPDATE n'est pas une action suspendue par le verrou")
	}
}
