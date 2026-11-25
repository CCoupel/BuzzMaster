package server

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
	"time"

	"buzzcontrol/internal/game"
)

// LOT-2 (v13.1.0, issue fille #324) — T16 : SQUELETTE des tests du Pool Picto (contracts/picto-pool.md rév. 2, §2 à §5, §7).
// Écrits contract-first, avant le code (T7 à T12 en Batch 2) : ils sont ROUGES tant que `/api/picto/*` n'existe pas
// et ne font appel qu'à HTTP + à l'API moteur déjà existante (Ready/SetPhase), jamais à un symbole à venir.
// Les codes d'erreur sont cherchés dans le corps de la réponse (WORD_INVALID, CATEGORY_UNKNOWN, ...).

func pictoBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("corps non JSON : %q (%v)", w.Body.String(), err)
	}
	return m
}

func addWord(t *testing.T, s *HTTPServer, word, cat string, diff int) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"QUESTION": word, "CATEGORY": cat, "DIFFICULTY": diff})
	return nrDo(t, s, "POST", "/api/picto/words", string(b))
}

func TestPoolPicto_List_EmptyShape_AndIdFormat(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w := nrDo(t, s, "GET", "/api/picto/words", "")
	if w.Code != 200 || w.Body.String() != "{\"QUESTIONS\":[],\"TOTAL\":0}\n" {
		t.Fatalf("liste vide : %d %q", w.Code, w.Body.String())
	}
	w = addWord(t, s, "Éléphant", "HISTORY", 2)
	if w.Code != 200 || w.Body.String() != "{\"ID\":\"p-001\"}\n" {
		t.Fatalf("création : %d %q (attendu ID p-001)", w.Code, w.Body.String())
	}
	w = nrDo(t, s, "GET", "/api/picto/words", "")
	want := `{"QUESTIONS":[{"ID":"p-001","QUESTION":"Éléphant","ANSWER":"","CATEGORY":"HISTORY","DIFFICULTY":2,"USED":false}],"TOTAL":1}` + "\n"
	if w.Body.String() != want {
		t.Errorf("le mot est dans QUESTION, ANSWER toujours vide.\n got  %q\n want %q", w.Body.String(), want)
	}
}

func TestPoolPicto_Create_Refusals_WithContractCodes(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	if w := addWord(t, s, "Éléphant", "HISTORY", 1); w.Code != 200 {
		t.Fatalf("seed : %d %s", w.Code, w.Body.String())
	}
	cases := []struct {
		name, word, cat string
		diff            int
		code            string
	}{
		{"vide", "  ", "HISTORY", 1, "WORD_INVALID"},
		{"> 40 caractères", strings.Repeat("a", 41), "HISTORY", 1, "WORD_INVALID"},
		{"caractère interdit", "chat<3", "HISTORY", 1, "WORD_INVALID"},
		{"catégorie inconnue", "chien", "NOPE", 1, "CATEGORY_UNKNOWN"},
		{"difficulté 0", "chien", "HISTORY", 0, "DIFFICULTY_INVALID"},
		{"difficulté 4", "chien", "HISTORY", 4, "DIFFICULTY_INVALID"},
		{"doublon replié dans la même catégorie", "ELEPHANT", "HISTORY", 3, "DUPLICATE"},
	}
	for _, c := range cases {
		w := addWord(t, s, c.word, c.cat, c.diff)
		if w.Code != 400 || !strings.Contains(w.Body.String(), c.code) {
			t.Errorf("%s : %d %q, attendu 400 avec %s", c.name, w.Code, w.Body.String(), c.code)
		}
	}
}

func TestPoolPicto_Create_AcceptsSpaceApostropheHyphenDigitsAccents_AndSameWordOtherCategory(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	for _, wd := range []string{"pont-levis", "l'avion", "Tour Eiffel", "R2D2", "château"} {
		if w := addWord(t, s, wd, "HISTORY", 1); w.Code != 200 {
			t.Errorf("%q refusé : %d %s", wd, w.Code, w.Body.String())
		}
	}
	// le doublon n'est interdit que PAR catégorie
	if w := addWord(t, s, "château", "SCIENCE", 1); w.Code != 200 {
		t.Errorf("même mot dans une autre catégorie : %d %s", w.Code, w.Body.String())
	}
}

func TestPoolPicto_Delete_Reset_ResetAll_Pool_Shapes(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	addWord(t, s, "chat", "HISTORY", 1)
	addWord(t, s, "chien", "HISTORY", 1)
	if w := nrDo(t, s, "POST", "/api/picto/words/p-001/reset", ""); w.Code != 200 || w.Body.String() != "{\"AVAILABLE\":true,\"ID\":\"p-001\"}\n" {
		t.Errorf("reset : %d %q", w.Code, w.Body.String())
	}
	if w := nrDo(t, s, "POST", "/api/picto/words/reset-all", ""); w.Code != 200 || !strings.HasPrefix(w.Body.String(), "{\"RESET\":") {
		t.Errorf("reset-all : %d %q", w.Code, w.Body.String())
	}
	w := nrDo(t, s, "GET", "/api/picto/pool?categories=HISTORY&difficulties=1", "")
	b := pictoBody(t, w)
	if w.Code != 200 || b["AVAILABLE"] != float64(2) || b["TOTAL"] != float64(2) || b["USED"] != float64(0) {
		t.Errorf("pool : %d %v", w.Code, b)
	}
	if w := nrDo(t, s, "DELETE", "/api/picto/words/p-001", ""); w.Code != 200 || w.Body.String() != "{\"DELETED\":\"p-001\"}\n" {
		t.Errorf("delete : %d %q", w.Code, w.Body.String())
	}
	if w := nrDo(t, s, "DELETE", "/api/picto/words/p-999", ""); w.Code != 404 {
		t.Errorf("delete inconnu : %d", w.Code)
	}
	if w := nrDo(t, s, "PUT", "/api/picto/words", ""); w.Code != 405 {
		t.Errorf("méthode non permise : %d", w.Code)
	}
}

func TestPoolPicto_Batch_CreatesValid_RefusesListed_InternalDuplicateFromSecondOccurrence(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	body := `{"CATEGORY":"HISTORY","DIFFICULTY":2,"WORDS":["chat","chien","CHAT","chat<3","  "]}`
	w := nrDo(t, s, "POST", "/api/picto/words/batch", body)
	if w.Code != 200 {
		t.Fatalf("batch : %d %s", w.Code, w.Body.String())
	}
	b := pictoBody(t, w)
	created, _ := b["CREATED"].([]any)
	refused, _ := b["REFUSED"].([]any)
	if len(created) != 2 || len(refused) != 3 {
		t.Fatalf("CREATED=%d (attendu 2) REFUSED=%d (attendu 3) : %v", len(created), len(refused), b)
	}
	codes := map[string]int{}
	for _, r := range refused {
		m := r.(map[string]any)
		codes[m["CODE"].(string)]++
	}
	if codes["DUPLICATE"] != 1 || codes["WORD_INVALID"] != 2 {
		t.Errorf("codes de refus = %v", codes)
	}
	// une seule sauvegarde : le fichier contient les 2 mots créés
	list := pictoBody(t, nrDo(t, s, "GET", "/api/picto/words", ""))
	if list["TOTAL"] != float64(2) {
		t.Errorf("TOTAL = %v", list["TOTAL"])
	}
}

func TestPoolPicto_Batch_Bounds_1To200Words(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	if w := nrDo(t, s, "POST", "/api/picto/words/batch", `{"CATEGORY":"HISTORY","DIFFICULTY":1,"WORDS":[]}`); w.Code != 400 {
		t.Errorf("0 mot : %d", w.Code)
	}
	words := make([]string, 201)
	for i := range words {
		words[i] = "mot" + string(rune('a'+i%26)) + strings.Repeat("x", i/26)
	}
	b, _ := json.Marshal(map[string]any{"CATEGORY": "HISTORY", "DIFFICULTY": 1, "WORDS": words})
	if w := nrDo(t, s, "POST", "/api/picto/words/batch", string(b)); w.Code != 400 {
		t.Errorf("201 mots : %d (attendu 400, bornes 1 à 200)", w.Code)
	}
}

func TestPoolPicto_Batch_NotAvailableForRafalePool(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w := nrDo(t, s, "POST", "/api/rafale/questions/batch", `{"CATEGORY":"HISTORY","DIFFICULTY":1,"WORDS":["chat"]}`)
	if w.Code == 200 {
		t.Errorf("le collage est désactivé pour le Pool Speedy, reçu 200")
	}
}

func TestPoolPicto_TwoInstances_AreIsolated_FilesAndIds(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	nrDo(t, s, "POST", "/api/rafale/questions", `{"QUESTION":"Q ?","ANSWER":"A","CATEGORY":"HISTORY","DIFFICULTY":1}`)
	addWord(t, s, "chat", "HISTORY", 1)
	rafale := pictoBody(t, nrDo(t, s, "GET", "/api/rafale/questions", ""))
	picto := pictoBody(t, nrDo(t, s, "GET", "/api/picto/words", ""))
	if rafale["TOTAL"] != float64(1) || picto["TOTAL"] != float64(1) {
		t.Fatalf("rafale=%v picto=%v : chaque instance a son propre magasin", rafale["TOTAL"], picto["TOTAL"])
	}
	for _, f := range []string{"files/rafale/reservoir.json", "files/picto/reservoir.json"} {
		if _, err := os.Stat(filepath.Join(dataDir, f)); err != nil {
			t.Errorf("%s absent : %v", f, err)
		}
	}
	data, _ := os.ReadFile(filepath.Join(dataDir, "files", "picto", "reservoir.json"))
	if !strings.Contains(string(data), `"QUESTIONS"`) || !strings.Contains(string(data), `"ANSWER": ""`) || !strings.Contains(string(data), `"p-001"`) {
		t.Errorf("format disque Picto : %s", data)
	}
}

func TestPoolPicto_NewGame_ResetsBothUsedMarks(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	addWord(t, s, "chat", "HISTORY", 1)
	if err := os.MkdirAll(filepath.Join(dataDir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	// marquage posé directement dans le fichier « utilisé » de l'instance picto puis rechargé par le moteur
	// (le moteur lit config/picto_used.json comme config/rafale_used.json)
	if err := os.WriteFile(filepath.Join(dataDir, "config", "picto_used.json"), []byte("{\n  \"USED\": {\n    \"p-001\": true\n  }\n}"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = nrDo(t, s, "POST", "/api/picto/words/reset-all", "")
	list := pictoBody(t, nrDo(t, s, "GET", "/api/picto/words", ""))
	qs, _ := list["QUESTIONS"].([]any)
	if len(qs) != 1 || qs[0].(map[string]any)["USED"] != false {
		t.Errorf("après reset-all, USED doit être false : %v", list)
	}
}

// ---- 409 pendant une manche Picto, génération IA : job unique croisé ----

func putPictoRoundInProgress(s *HTTPServer, phase game.GamePhase) {
	s.engine.Ready("picto-round-1", &game.Question{ID: "picto-round-1", Type: game.QuestionTypePicto})
	s.engine.SetPhase(phase)
}

func TestPoolPicto_WritesRefused409_DuringPictoRound(t *testing.T) {
	for _, phase := range []game.GamePhase{game.PhaseStarted, game.PhasePaused} {
		s, _ := setupTestHTTPServer(t)
		addWord(t, s, "chat", "HISTORY", 1)
		putPictoRoundInProgress(s, phase)
		for _, c := range []struct{ method, url, body string }{
			{"POST", "/api/picto/words", `{"QUESTION":"chien","CATEGORY":"HISTORY","DIFFICULTY":1}`},
			{"POST", "/api/picto/words/batch", `{"CATEGORY":"HISTORY","DIFFICULTY":1,"WORDS":["lion"]}`},
			{"DELETE", "/api/picto/words/p-001", ""},
			{"POST", "/api/picto/words/p-001/reset", ""},
			{"POST", "/api/picto/words/reset-all", ""},
		} {
			w := nrDo(t, s, c.method, c.url, c.body)
			if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "picto_round_in_progress") {
				t.Errorf("phase %s, %s %s : %d %q, attendu 409 picto_round_in_progress", phase, c.method, c.url, w.Code, w.Body.String())
			}
		}
		// la liste est aussi gardée pendant la manche (le pool ne doit pas être lisible pendant le jeu)
		if w := nrDo(t, s, "GET", "/api/picto/words", ""); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "picto_round_in_progress") {
			t.Errorf("phase %s, GET /api/picto/words : %d %q, attendu 409 picto_round_in_progress", phase, w.Code, w.Body.String())
		}
		// le Pool Speedy n'est pas concerné
		if w := nrDo(t, s, "GET", "/api/rafale/questions", ""); w.Code != 200 {
			t.Errorf("GET /api/rafale/questions pendant une manche Picto : %d", w.Code)
		}
	}
}

func TestPoolPicto_List_Returns200_OutsideAPictoRound(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	addWord(t, s, "chat", "HISTORY", 1)
	if w := nrDo(t, s, "GET", "/api/picto/words", ""); w.Code != 200 {
		t.Errorf("GET hors manche : %d", w.Code)
	}
	putPictoRoundInProgress(s, game.PhaseStopped)
	if w := nrDo(t, s, "GET", "/api/picto/words", ""); w.Code != 200 {
		t.Errorf("GET manche arrêtée (STOPPED) : %d, attendu 200 (seuls STARTED et PAUSED gardent)", w.Code)
	}
}

func TestPoolPicto_RafalePoolWrites_StayAllowed_DuringPictoRound(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	putPictoRoundInProgress(s, game.PhaseStarted)
	w := nrDo(t, s, "POST", "/api/rafale/questions", `{"QUESTION":"Q ?","ANSWER":"A","CATEGORY":"HISTORY","DIFFICULTY":1}`)
	if w.Code != 200 {
		t.Errorf("la garde Picto ne concerne pas le Pool Speedy : %d %s", w.Code, w.Body.String())
	}
}

func TestPoolPicto_Generate_RoundInProgress_409_And_ProgressTargetIsPicto(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	setAIConfig(validAIConfig())
	putPictoRoundInProgress(s, game.PhaseStarted)
	b, _ := json.Marshal(map[string]any{"theme": "Animaux", "populations": []string{"Adulte (18-64 ans)"}, "language": "Français",
		"categories": []string{"HISTORY"}, "difficulties": []int{1}, "count": 10})
	w := nrDo(t, s, "POST", "/api/picto/generate-words", string(b))
	if w.Code != http.StatusConflict || pictoBody(t, w)["code"] != "picto_round_in_progress" {
		t.Errorf("génération pendant une manche Picto : %d %q", w.Code, w.Body.String())
	}
}

func TestPoolPicto_Generate_Presets_10_20_50_Only(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	setAIConfig(validAIConfig())
	for _, n := range []int{5, 30, 100, 200} {
		b, _ := json.Marshal(map[string]any{"theme": "Animaux", "populations": []string{"Adulte (18-64 ans)"}, "language": "Français",
			"categories": []string{"HISTORY"}, "difficulties": []int{1}, "count": n})
		if w := nrDo(t, s, "POST", "/api/picto/generate-words", string(b)); w.Code != 400 {
			t.Errorf("count=%d : %d (paliers Picto = 10/20/50)", n, w.Code)
		}
	}
}

func TestPoolPicto_Generate_SingleJobAcrossRafaleAndPicto_409(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("anthropic", 2, 10, 2))
	slow := blockingAnthropicServer(t, 500*time.Millisecond, []map[string]interface{}{rafaleLLMQuestion(nil)})
	withAnthropicBaseURL(t, slow.URL)
	conn := dialAdminWS(t, s)

	expectAccepted(t, postRafaleGenerateQuestions(s, baseRafaleRequest()))
	b, _ := json.Marshal(map[string]any{"theme": "Animaux", "populations": []string{"Adulte (18-64 ans)"}, "language": "Français",
		"categories": []string{"HISTORY"}, "difficulties": []int{1}, "count": 10})
	w := nrDo(t, s, "POST", "/api/picto/generate-words", string(b))
	if w.Code != http.StatusConflict || pictoBody(t, w)["code"] != "generation_in_progress" {
		t.Errorf("job RAFALE en cours puis génération Picto : %d %q (attendu 409 generation_in_progress)", w.Code, w.Body.String())
	}
	waitForJobTerminalState(t, conn, 5*time.Second)
}

// ---- Sauvegarde : clé `picto`, archive v13.0.0 restaurée à l'identique ----

func TestPoolPicto_Backup_PictoKey_ArchivePaths_And_RafaleKeyUnchanged(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	addWord(t, s, "chat", "HISTORY", 1)
	if err := os.MkdirAll(filepath.Join(dataDir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dataDir, "config", "picto_used.json"), []byte("{\n  \"USED\": {}\n}"), 0o644)

	w := nrDo(t, s, "GET", "/backup-select?picto=true", "")
	if w.Code != 200 {
		t.Fatalf("backup-select?picto=true : %d", w.Code)
	}
	names := tarNames(t, w.Body.Bytes())
	for _, want := range []string{"files/picto/reservoir.json", "config/picto_used.json"} {
		if !names[want] {
			t.Errorf("archive sans %q : %v", want, names)
		}
	}
	for n := range names {
		if strings.HasPrefix(n, "files/rafale") || strings.HasPrefix(n, "config/rafale_used") {
			t.Errorf("picto=true ne doit pas embarquer le Pool Speedy : %s", n)
		}
	}
}

func tarNames(t *testing.T, data []byte) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	tr := tar.NewReader(bytes.NewReader(data))
	for {
		h, err := tr.Next()
		if err != nil {
			return out
		}
		out[strings.TrimSuffix(h.Name, "/")] = true
	}
}

func buildPoolTAR(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func restoreTAR(t *testing.T, s *HTTPServer, tarData []byte) map[string]bool {
	t.Helper()
	body, ct := multipartRestoreBody(t, tarData)
	req := httptest.NewRequest("POST", "/restore", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("restore : %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Restored map[string]bool `json:"restored"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return resp.Restored
}

const v1300ReservoirArchive = "{\n  \"QUESTIONS\": [\n    {\n      \"ID\": \"r-001\",\n      \"QUESTION\": \"Q ?\",\n      \"ANSWER\": \"A\",\n      \"CATEGORY\": \"HISTORY\",\n      \"DIFFICULTY\": 1\n    }\n  ]\n}"

// Archive antérieure (v13.0.0, sans files/picto) : restaurée à l'identique, aucun Pool Picto créé (contrat §7, §9).
func TestPoolPicto_Restore_PreV1310Archive_RafaleIdentical_NoPictoSideEffect(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	restored := restoreTAR(t, s, buildPoolTAR(t, map[string]string{
		"files/rafale/reservoir.json": v1300ReservoirArchive,
		"config/rafale_used.json":     "{\n  \"USED\": {}\n}",
	}))
	if !restored["rafale"] {
		t.Errorf("restored.rafale = false : %v", restored)
	}
	if restored["picto"] {
		t.Errorf("restored.picto = true alors que l'archive n'a pas de files/picto")
	}
	got, err := os.ReadFile(filepath.Join(dataDir, "files", "rafale", "reservoir.json"))
	if err != nil || string(got) != v1300ReservoirArchive {
		t.Errorf("reservoir.json restauré ≠ archive : %q (%v)", got, err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "files", "picto")); err == nil {
		t.Errorf("files/picto ne doit pas être créé par la restauration d'une archive v13.0.0")
	}
	if w := nrDo(t, s, "GET", "/api/picto/words", ""); w.Code != 200 || w.Body.String() != "{\"QUESTIONS\":[],\"TOTAL\":0}\n" {
		t.Errorf("Pool Picto après restauration v13.0.0 : %d %q", w.Code, w.Body.String())
	}
}

func TestPoolPicto_Restore_PictoArchive_RestoresPictoKey(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	pic := "{\n  \"QUESTIONS\": [\n    {\n      \"ID\": \"p-001\",\n      \"QUESTION\": \"chat\",\n      \"ANSWER\": \"\",\n      \"CATEGORY\": \"HISTORY\",\n      \"DIFFICULTY\": 1\n    }\n  ]\n}"
	restored := restoreTAR(t, s, buildPoolTAR(t, map[string]string{
		"files/picto/reservoir.json": pic,
		"config/picto_used.json":     "{\n  \"USED\": {}\n}",
	}))
	if !restored["picto"] || restored["rafale"] {
		t.Errorf("restored = %v (attendu picto seul)", restored)
	}
	list := pictoBody(t, nrDo(t, s, "GET", "/api/picto/words", ""))
	if list["TOTAL"] != float64(1) {
		t.Errorf("mot restauré non visible : %v", list)
	}
}
