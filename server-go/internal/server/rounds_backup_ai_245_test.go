package server

// #245 (v11.2) — scénario 9 : la sauvegarde « questions » emporte rounds.json et la restauration le remet en place ET le recharge
// dans l'état de jeu (GameState.ROUNDS) ; scénario 10 : les questions générées par l'IA n'ont pas de ROUND_ID et arrivent à la FIN du
// groupe « Sans manche » (en tête du jeu, contrat b77f5314), c'est-à-dire ORDER = plus grand ORDER de ce groupe + 1, indépendamment des
// questions de manche. Contrats : models.md §Manches, http-endpoints.md (§Génération IA / sauvegarde), ai-generation.md §5.2.

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tarNames245(t *testing.T, data []byte) []string {
	t.Helper()
	var names []string
	tr := tar.NewReader(bytes.NewReader(data))
	for {
		h, err := tr.Next()
		if err != nil {
			return names
		}
		names = append(names, h.Name)
	}
}

func engineRounds245(t *testing.T, s *HTTPServer) []map[string]interface{} {
	t.Helper()
	var g struct {
		Game struct {
			Rounds []map[string]interface{} `json:"ROUNDS"`
		} `json:"GAME"`
	}
	_ = json.Unmarshal(s.engine.GetGameJSON(), &g)
	return g.Game.Rounds
}

// --- 9. Sauvegarde / restauration -----------------------------------------------------------------------------------------------

func TestRoundsBackup245_QuestionsBackupCarriesRoundsJson(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	writeRounds245(t, dataDir)
	writeQuestionFile254(t, dataDir, "1", map[string]interface{}{"ID": "1", "TYPE": "SPEEDY", "QUESTION": "Q", "ANSWER": "a", "POINTS": "1", "TIME": "30", "ORDER": 0, "ROUND_ID": "r1"})

	req := httptest.NewRequest("GET", "/backup-select?questions=true", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("%d", w.Code)
	}
	names := tarNames245(t, w.Body.Bytes())
	if !tarContains(t, w.Body.Bytes(), "files/questions/rounds.json") {
		t.Errorf("la sauvegarde « questions » emporte files/questions/rounds.json ; archive : %v", names)
	}
	if !tarContains(t, w.Body.Bytes(), "files/questions/1/question.json") {
		t.Errorf("et la question avec son ROUND_ID : %v", names)
	}
}

func TestRoundsBackup245_Critical_RestoreWritesRoundsJsonAndReloadsTheRoundsIntoTheGameState(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	if rounds := engineRounds245(t, s); len(rounds) != 0 {
		t.Fatalf("précondition : aucune manche au départ : %v", rounds)
	}
	roundsJSON := `{"ROUNDS":[{"ID":"r1","NAME":"Un"},{"ID":"r2","NAME":"Deux"}],"NEXT_ID":3}`
	question := `{"ID":"1","TYPE":"SPEEDY","QUESTION":"Q","ANSWER":"a","POINTS":"1","TIME":"30","ORDER":0,"ROUND_ID":"r2"}`
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range map[string]string{"files/questions/rounds.json": roundsJSON, "files/questions/1/question.json": question} {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}); err != nil {
			t.Fatal(err)
		}
		_, _ = tw.Write([]byte(content))
	}
	_ = tw.Close()
	body, contentType := multipartRestoreBody(t, buf.Bytes())

	req := httptest.NewRequest("POST", "/restore", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("%d : %s", w.Code, w.Body.String())
	}
	raw, err := os.ReadFile(filepath.Join(dataDir, "files", "questions", "rounds.json"))
	if err != nil {
		t.Fatalf("rounds.json restauré sur disque : %v", err)
	}
	if !strings.Contains(string(raw), `"Deux"`) {
		t.Errorf("contenu restauré : %s", raw)
	}
	rounds := engineRounds245(t, s)
	if len(rounds) != 2 || rounds[0]["ID"] != "r1" || rounds[1]["NAME"] != "Deux" {
		t.Errorf("GameState.ROUNDS rechargé après la restauration : %v", rounds)
	}
	// la question restaurée est servie avec son ROUND_ID
	if all := listQuestions254(t, s); all["1"]["ROUND_ID"] != "r2" {
		t.Errorf("question restaurée avec son ROUND_ID : %v", all["1"])
	}
}

func TestRoundsBackup245_RestoreOfARestrictedBackupWithoutRoundsJsonKeepsTheQuestionsUsable(t *testing.T) {
	// restauration partielle : la question référence une manche absente → « Sans manche », sans erreur
	s, dataDir := setupTestHTTPServer(t)
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	content := `{"ID":"1","TYPE":"SPEEDY","QUESTION":"Q","ANSWER":"a","POINTS":"1","TIME":"30","ORDER":0,"ROUND_ID":"r7"}`
	_ = tw.WriteHeader(&tar.Header{Name: "files/questions/1/question.json", Mode: 0o644, Size: int64(len(content))})
	_, _ = tw.Write([]byte(content))
	_ = tw.Close()
	body, contentType := multipartRestoreBody(t, buf.Bytes())

	req := httptest.NewRequest("POST", "/restore", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("%d : %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dataDir, "files", "questions", "1", "question.json")); err != nil {
		t.Errorf("la question est restaurée : %v", err)
	}
	if rounds := engineRounds245(t, s); len(rounds) != 0 {
		t.Errorf("aucune manche inventée : %v", rounds)
	}
}

// --- 10. Génération IA -------------------------------------------------------------------------------------------------------------

func TestRoundsAI245_GeneratedQuestionsCarryNoRoundID(t *testing.T) {
	for _, typ := range []string{"SPEEDY", "QCM", "ARDOISE"} {
		raw := llmRawQuestion{Type: typ, Question: "Q ?", Answer: "r", Time: 30, Difficulty: "Facile", ArdoiseKeyboardType: "AZERTY"}
		if typ == "QCM" {
			raw.QCMAnswers = &llmQCMAnswers{Red: "a", Green: "b", Yellow: "c", Blue: "d"}
			raw.QCMCorrect = "RED"
		}
		q := mapGeneratedQuestion(raw, "90", 3)
		if _, has := q["ROUND_ID"]; has {
			t.Errorf("%s : une question générée n'a pas de ROUND_ID (« Sans manche ») : %v", typ, q["ROUND_ID"])
		}
		if q["ORDER"] != 3 {
			t.Errorf("%s : ORDER transmis tel quel : %v", typ, q["ORDER"])
		}
	}
}

func TestRoundsAI245_Critical_GeneratedOrderStartsAfterTheLastQuestionOfNoRoundNotAfterTheRoundQuestions(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	writeRounds245(t, dataDir)
	for id, spec := range map[string]struct {
		order int
		round string
	}{"1": {0, ""}, "2": {1, ""}, "3": {2, ""}, "4": {5, "r1"}, "5": {90, "r2"}} {
		q := map[string]interface{}{"ID": id, "TYPE": "SPEEDY", "QUESTION": "Q" + id, "ANSWER": "a", "POINTS": "1", "TIME": "30", "ORDER": spec.order}
		if spec.round != "" {
			q["ROUND_ID"] = spec.round
		}
		writeQuestionFile254(t, dataDir, id, q)
	}

	// les questions générées seront ORDER = base + 1 + i ; la base est le plus grand ORDER de « Sans manche » (2), jamais 90
	if base := s.maxQuestionOrder(); base != 2 {
		t.Errorf("base d'ORDER des questions générées = %d, attendu 2 (fin du groupe « Sans manche », en tête du jeu)", base)
	}
}

func TestRoundsAI245_WithoutAnyRoundTheBaseIsTheWholeQuizAsBefore(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	for id, order := range map[string]int{"1": 4, "2": 11, "3": 7} {
		writeQuestionFile254(t, dataDir, id, map[string]interface{}{"ID": id, "TYPE": "SPEEDY", "QUESTION": "Q", "ANSWER": "a", "POINTS": "1", "TIME": "30", "ORDER": order})
	}
	if base := s.maxQuestionOrder(); base != 11 {
		t.Errorf("sans manche : plus grand ORDER du quiz (11), comme avant : %d", base)
	}
}
