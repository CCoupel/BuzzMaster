package server

// #245 (v11.2, lot V3-1) — formulaire de question et manches : `round_id` à la création (« + Question » d'une manche),
// `ROUND_ID` conservé à la modification (piège de reconstruction du JSON), ajout en fin de groupe (« Sans manche » en tête),
// ID de manche inconnu ignoré. Contrats : http-endpoints.md (round_id, ROUND_ID conservé), models.md §Manches.
// Scénarios 7 et 8 (planner-20261012-200000.md). Boîte noire par le vrai handler HTTP.

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func writeRounds245(t *testing.T, dataDir string) {
	t.Helper()
	dir := filepath.Join(dataDir, "files", "questions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rounds.json"), []byte(`{"ROUNDS":[{"ID":"r1","NAME":"Un"},{"ID":"r2","NAME":"Deux"}],"NEXT_ID":3}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func orderNum245(q map[string]interface{}) int {
	switch v := q["ORDER"].(type) {
	case float64:
		return int(v)
	case string:
		n := 0
		for _, c := range v {
			n = n*10 + int(c-'0')
		}
		return n
	}
	return -1
}

func TestRoundsForm245_NewQuestionWithRoundIDIsAttachedAndPlacedAtTheEndOfItsRound(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	writeRounds245(t, dataDir)
	writeQuestionFile254(t, dataDir, "1", map[string]interface{}{"ID": "1", "TYPE": "SPEEDY", "QUESTION": "A", "ANSWER": "a", "POINTS": "1", "TIME": "30", "ORDER": 0, "ROUND_ID": "r1"})
	writeQuestionFile254(t, dataDir, "2", map[string]interface{}{"ID": "2", "TYPE": "SPEEDY", "QUESTION": "B", "ANSWER": "b", "POINTS": "1", "TIME": "30", "ORDER": 7, "ROUND_ID": "r1"})
	writeQuestionFile254(t, dataDir, "3", map[string]interface{}{"ID": "3", "TYPE": "SPEEDY", "QUESTION": "C", "ANSWER": "c", "POINTS": "1", "TIME": "30", "ORDER": 40, "ROUND_ID": "r2"})

	code := postQuestion254(t, s, map[string]string{"number": "10", "type": "SPEEDY", "question": "Nouvelle", "answer": "n", "points": "1", "time": "30", "category": "CULTURE", "round_id": "r1"})

	if code != http.StatusOK {
		t.Fatalf("code %d", code)
	}
	q := savedQuestion254(t, dataDir, "10")
	if q["ROUND_ID"] != "r1" || orderNum245(q) != 8 {
		t.Errorf("rattachée à r1, ORDER = plus grand ORDER de la manche + 1 (8) : ROUND_ID=%v ORDER=%v", q["ROUND_ID"], q["ORDER"])
	}
}

func TestRoundsForm245_UnknownRoundIDIsIgnoredAndWithoutRoundGoesAtTheEndOfNoRound(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	writeRounds245(t, dataDir)
	writeQuestionFile254(t, dataDir, "1", map[string]interface{}{"ID": "1", "TYPE": "SPEEDY", "QUESTION": "A", "ANSWER": "a", "POINTS": "1", "TIME": "30", "ORDER": 3})
	writeQuestionFile254(t, dataDir, "2", map[string]interface{}{"ID": "2", "TYPE": "SPEEDY", "QUESTION": "B", "ANSWER": "b", "POINTS": "1", "TIME": "30", "ORDER": 90, "ROUND_ID": "r2"})

	postQuestion254(t, s, map[string]string{"number": "11", "type": "SPEEDY", "question": "X", "answer": "x", "points": "1", "time": "30", "category": "CULTURE", "round_id": "r99"})
	postQuestion254(t, s, map[string]string{"number": "12", "type": "SPEEDY", "question": "Y", "answer": "y", "points": "1", "time": "30", "category": "CULTURE"})

	for id, wantOrder := range map[string]int{"11": 4, "12": 5} {
		q := savedQuestion254(t, dataDir, id)
		if r, _ := q["ROUND_ID"].(string); r != "" {
			t.Errorf("question %s : round_id inconnu ou absent → pas de ROUND_ID (Sans manche) : %v", id, r)
		}
		if orderNum245(q) != wantOrder {
			t.Errorf("question %s : ORDER = %v, attendu %d (fin du groupe « Sans manche », qui reste en tête)", id, q["ORDER"], wantOrder)
		}
	}
}

func TestRoundsForm245_Critical_EditingAQuestionKeepsItsRoundIDWhateverRoundIDTheFormCarries(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	writeRounds245(t, dataDir)
	writeQuestionFile254(t, dataDir, "5", map[string]interface{}{"ID": "5", "TYPE": "SPEEDY", "QUESTION": "Avant", "ANSWER": "a", "POINTS": "1", "TIME": "30", "ORDER": 2, "ROUND_ID": "r2"})

	// modification sans round_id : conservé
	postQuestion254(t, s, map[string]string{"number": "5", "type": "SPEEDY", "question": "Après", "answer": "a", "points": "2", "time": "30", "category": "CULTURE"})
	if q := savedQuestion254(t, dataDir, "5"); q["ROUND_ID"] != "r2" || q["QUESTION"] != "Après" || orderNum245(q) != 2 {
		t.Errorf("ROUND_ID et ORDER conservés à la modification (le JSON est reconstruit) : %v", q)
	}
	// modification avec un autre round_id : seul ROUNDS_UPDATE change la manche d'une question existante
	postQuestion254(t, s, map[string]string{"number": "5", "type": "SPEEDY", "question": "Encore", "answer": "a", "points": "2", "time": "30", "category": "CULTURE", "round_id": "r1"})
	if q := savedQuestion254(t, dataDir, "5"); q["ROUND_ID"] != "r2" {
		t.Errorf("round_id est ignoré à la modification : ROUND_ID = %v", q["ROUND_ID"])
	}
}

func TestRoundsForm245_RoundIDIsServedByTheQuestionList(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	writeRounds245(t, dataDir)
	writeQuestionFile254(t, dataDir, "6", map[string]interface{}{"ID": "6", "TYPE": "SPEEDY", "QUESTION": "Q", "ANSWER": "a", "POINTS": "1", "TIME": "30", "ORDER": 1, "ROUND_ID": "r1"})
	writeQuestionFile254(t, dataDir, "7", map[string]interface{}{"ID": "7", "TYPE": "SPEEDY", "QUESTION": "Q", "ANSWER": "a", "POINTS": "1", "TIME": "30", "ORDER": 2, "ROUND_ID": "r9"})

	all := listQuestions254(t, s)

	if all["6"]["ROUND_ID"] != "r1" {
		t.Errorf("GET /questions porte ROUND_ID : %v", all["6"])
	}
	if _, isQuestion := all["rounds"]; isQuestion {
		t.Error("rounds.json n'est pas pris pour une question")
	}
	if all["7"] == nil {
		t.Error("une question de manche inconnue est servie sans erreur (Sans manche)")
	}
}
