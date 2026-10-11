package main

import (
	"buzzcontrol/internal/protocol"
	"buzzcontrol/internal/server"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func roundsReq245(t *testing.T, raw string) protocol.RoundsUpdatePayload {
	t.Helper()
	var r protocol.RoundsUpdatePayload
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func questionFile245(t *testing.T, dir, id string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, id, "question.json"))
	if err != nil {
		t.Fatalf("question %s : %v", id, err)
	}
	var q map[string]interface{}
	_ = json.Unmarshal(data, &q)
	return q
}

func TestRoundsUpdate_CreateKeepDeleteAndRefusals(t *testing.T) {
	app := newAwardedTeamsTriggerTestApp(t)
	dir := app.config.Storage.QuestionsDir
	for _, id := range []string{"1", "2", "3", "4"} {
		writeQuestionFile(t, dir, id, nil, nil)
	}

	// create one round (entractes by default) holding 3 and 1; 2 and 4 stay unassigned
	res, _ := app.applyRoundsUpdate(roundsReq245(t, `{"ROUNDS":[{"NAME":"  Manche A  ","QUESTIONS":["3","1"]}],"UNASSIGNED":["2","4"]}`))
	if res.Status != "APPLIED" || len(res.Rounds) != 1 || res.Rounds[0].ID != "r1" || res.Rounds[0].Name != "Manche A" {
		t.Fatalf("création : %+v", res)
	}
	if len(res.CreatedEntractes) != 1 {
		t.Fatalf("deux entractes automatiques attendues : %+v", res.CreatedEntractes)
	}
	ce := res.CreatedEntractes[0]
	start, end := questionFile245(t, dir, ce.StartID), questionFile245(t, dir, ce.EndID)
	if start["TYPE"] != "ENTRACTE" || start["QUESTION"] != "Début de la manche Manche A" || end["QUESTION"] != "Fin de la manche Manche A" {
		t.Fatalf("entractes : %v / %v", start, end)
	}
	if rf := server.LoadRoundsFile(dir); len(rf.Rounds) != 1 || rf.NextID != 2 {
		t.Fatalf("rounds.json : %+v", rf)
	}
	q3 := questionFile245(t, dir, "3")
	if q3["ROUND_ID"] != "r1" || int(q3["ORDER"].(float64)) != 1 { // start entracte is position 0
		t.Fatalf("question 3 : %v", q3)
	}
	if _, has := questionFile245(t, dir, "2")["ROUND_ID"]; has {
		t.Error("une question sans manche n'a pas de ROUND_ID")
	}

	// refusals write nothing
	for raw, want := range map[string]string{
		`{"ROUNDS":[],"UNASSIGNED":["1","2","3","4"]}`:                                     "MISSING_ROUND_DECISION",
		`{"ROUNDS":[{"ID":"r1","NAME":"","QUESTIONS":[]}],"UNASSIGNED":[]}`:                "INVALID_NAME",
		`{"ROUNDS":[{"ID":"r9","NAME":"x","QUESTIONS":[]}],"UNASSIGNED":[]}`:               "UNKNOWN_ROUND",
		`{"ROUNDS":[{"ID":"r1","NAME":"A","QUESTIONS":["1","1"]}],"UNASSIGNED":[]}`:        "DUPLICATE_QUESTION",
		`{"ROUNDS":[{"ID":"r1","NAME":"A","QUESTIONS":["99"]}],"UNASSIGNED":[]}`:           "UNKNOWN_QUESTION",
		`{"ROUNDS":[{"ID":"r1","NAME":"A","QUESTIONS":[]}],"UNASSIGNED":["2"]}`:            "INCOMPLETE",
		`{"ROUNDS":[],"UNASSIGNED":[],"DELETED_ROUNDS":[{"ID":"r1","QUESTIONS":"MAYBE"}]}`: "INVALID_DECISION",
	} {
		res, _ := app.applyRoundsUpdate(roundsReq245(t, raw))
		if res.Status != "REFUSED" || res.Reason != want {
			t.Errorf("%s : %+v, attendu %s", raw, res, want)
		}
	}
	if rf := server.LoadRoundsFile(dir); len(rf.Rounds) != 1 {
		t.Fatal("un refus n'écrit rien")
	}

	// delete the round, KEEP its questions: they (entractes included) go to the end of Sans manche
	res, _ = app.applyRoundsUpdate(roundsReq245(t, `{"ROUNDS":[],"UNASSIGNED":["2","4"],"DELETED_ROUNDS":[{"ID":"r1","QUESTIONS":"KEEP"}]}`))
	if res.Status != "APPLIED" || len(res.DeletedQuestions) != 0 {
		t.Fatalf("KEEP : %+v", res)
	}
	if _, has := questionFile245(t, dir, "3")["ROUND_ID"]; has {
		t.Error("après KEEP, plus de ROUND_ID")
	}
	if q := questionFile245(t, dir, ce.StartID); int(q["ORDER"].(float64)) < 2 {
		t.Errorf("les questions gardées passent après les sans-manche : %v", q["ORDER"])
	}

	// new round with DELETE: its questions disappear
	res, _ = app.applyRoundsUpdate(roundsReq245(t, `{"ROUNDS":[{"NAME":"B","QUESTIONS":["1"],"ENTRACTES":false}],"UNASSIGNED":["2","3","4","`+ce.StartID+`","`+ce.EndID+`"]}`))
	if res.Status != "APPLIED" || len(res.CreatedEntractes) != 0 {
		t.Fatalf("création sans entracte : %+v", res)
	}
	res, _ = app.applyRoundsUpdate(roundsReq245(t, `{"ROUNDS":[],"UNASSIGNED":["2","3","4","`+ce.StartID+`","`+ce.EndID+`"],"DELETED_ROUNDS":[{"ID":"r2","QUESTIONS":"DELETE"}]}`))
	if res.Status != "APPLIED" || len(res.DeletedQuestions) != 1 || res.DeletedQuestions[0] != "1" {
		t.Fatalf("DELETE : %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dir, "1")); !os.IsNotExist(err) {
		t.Error("le dossier de la question supprimée a disparu")
	}
}
