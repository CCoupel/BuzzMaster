package server

// Tests dérivés de contracts/ai-generation.md (#8, v6.0.0) et de
// _work/reports/planner-20260805-121900.md, phases 2 à 4.
//
// Ce fichier est un ajout NEUF (aucune collision avec http_test.go, étendu par
// dev-backend pour le correctif générique de POST /config.json — cf.
// internal/config/config_merge_test.go pour le détail du découpage).
//
// ⚠️ config.SetInstance mute un global : PAS de t.Parallel() dans ce fichier.
//
// --------------------------------------------------------------------------------
// SECTION C — hypothèse d'intégration à faire valider par dev-backend/code-reviewer
// --------------------------------------------------------------------------------
// Le contrat (§4) exige un client Anthropic "mocké (httptest)". Au moment où ces
// tests sont écrits, `internal/server/ai_client.go` n'existe pas encore (Batch 1 :
// dev-backend et test-writer tournent en parallèle sur les mêmes contrats). Pour
// rester mockable sans dépendre du SDK officiel (dont le point d'entrée réel du
// service Anthropic n'est pas modifiable en dur), ces tests supposent que le client
// respecte la variable d'environnement ANTHROPIC_BASE_URL (comportement standard des
// SDK officiels Anthropic — l'équivalent de OPENAI_BASE_URL) pour rediriger ses
// appels vers le httptest.Server local défini ci-dessous.
//
// Si dev-backend a choisi le repli stdlib net/http (contrat §4, risque R7) sans
// respecter cette variable, `code-reviewer`/`qa` doivent soit l'ajouter (une ligne :
// lire ANTHROPIC_BASE_URL avec repli sur "https://api.anthropic.com"), soit adapter
// la constante `anthropicBaseURLEnvVar` ci-dessous au mécanisme réellement choisi.
// Les tests de la Section A (validation) et B (allocation d'ID) ne dépendent PAS de
// cette hypothèse — ils passent aussi bien avec le SDK officiel qu'avec le repli.

import (
	"buzzcontrol/internal/config"
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const anthropicBaseURLEnvVar = "ANTHROPIC_BASE_URL"

// ================================================================================
// Helpers
// ================================================================================

// setAIConfig mutates the singleton's AI section in place (config.Get() returns the
// same *Config pointer that setupTestHTTPServer registered via SetInstance).
func setAIConfig(ai config.AIConfig) {
	cfg := config.Get()
	cfg.AI = ai
	config.SetInstance(cfg)
}

func validAIConfig() config.AIConfig {
	return config.AIConfig{
		AnthropicAPIKey: "sk-ant-test-key-0001",
		Model:           "claude-opus-5",
		TimeoutSeconds:  5,
		MaxQuestions:    200,
	}
}

// baseGenerateRequest returns a fresh, fully valid request body per contract §3.
// Tests mutate the returned map before marshaling to exercise specific fields.
func baseGenerateRequest() map[string]interface{} {
	return map[string]interface{}{
		"theme":        "Cinéma français des années 80",
		"populations":  []string{"Adulte (18-64 ans)"},
		"language":     "Français",
		"difficulties": []string{"Moyen"},
		"objectives":   "",
		"instructions": "",
		"categories":   []string{"ENTERTAINMENT"},
		"volume":       map[string]interface{}{"mode": "count", "value": 5},
		"distribution": map[string]interface{}{"SPEEDY": 100, "QCM": 0, "MEMORY": 0, "MEMOTION": 0},
	}
}

func postGenerateQuestions(server *HTTPServer, payload map[string]interface{}) *httptest.ResponseRecorder {
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/generate-questions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	return w
}

func decodeJSONBody(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("Response body is not valid JSON: %v — body: %s", err, w.Body.String())
	}
	return out
}

func readQuestionFile(t *testing.T, dataDir, id string) map[string]interface{} {
	t.Helper()
	path := filepath.Join(dataDir, "files", "questions", id, "question.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Could not read question.json for id %s: %v", id, err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("question.json for id %s is not valid JSON: %v", id, err)
	}
	return out
}

func countQuestionDirs(t *testing.T, dataDir string) int {
	t.Helper()
	dir := filepath.Join(dataDir, "files", "questions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("ReadDir failed: %v", err)
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			n++
		}
	}
	return n
}

// llmQuestion builds one raw LLM-schema question object (contract §5) as JSON-ready map.
func llmQuestion(fields map[string]interface{}) map[string]interface{} {
	base := map[string]interface{}{
		"CATEGORY":   "ENTERTAINMENT",
		"QUESTION":   "Question générée ?",
		"TIME":       20,
		"DIFFICULTY": "Moyen",
	}
	for k, v := range fields {
		base[k] = v
	}
	return base
}

// writeSSEQuestions streams a minimal but well-formed Anthropic Messages API SSE
// response (contract §4: "Streaming obligatoire") whose single text content block
// is the given JSON-encoded LLM payload (`{"questions": [...]}`, contract §5 root
// schema). Shared by mockAnthropicSSEServer (fixed content) and
// mockAnthropicSSEServerPerCall (content computed fresh on every request).
func writeSSEQuestions(t *testing.T, w http.ResponseWriter, questions []map[string]interface{}) {
	t.Helper()
	payload, err := json.Marshal(map[string]interface{}{"questions": questions})
	if err != nil {
		t.Fatalf("Failed to marshal mock LLM payload: %v", err)
	}
	escaped, _ := json.Marshal(string(payload)) // JSON-escape the text delta content

	w.Header().Set("Content-Type", "text/event-stream")
	flusher, ok := w.(http.Flusher)
	if !ok {
		t.Fatal("ResponseWriter does not support flushing (required for SSE)")
	}
	events := []string{
		`event: message_start` + "\n" +
			`data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","model":"claude-opus-5","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":0}}}` + "\n\n",
		`event: content_block_start` + "\n" +
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` + "\n\n",
		`event: content_block_delta` + "\n" +
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":` + string(escaped) + `}}` + "\n\n",
		`event: content_block_stop` + "\n" +
			`data: {"type":"content_block_stop","index":0}` + "\n\n",
		`event: message_delta` + "\n" +
			`data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":42}}` + "\n\n",
		`event: message_stop` + "\n" +
			`data: {"type":"message_stop"}` + "\n\n",
	}
	for _, e := range events {
		fmt.Fprint(w, e)
		flusher.Flush()
	}
}

// mockAnthropicSSEServer stands in for the Anthropic Messages API with a FIXED
// response: every request (however many) gets the same questions back. Fine for
// single-request tests, but unsuitable when several concurrent requests must be
// distinguishable afterward (see mockAnthropicSSEServerPerCall below).
func mockAnthropicSSEServer(t *testing.T, questions []map[string]interface{}) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEQuestions(t, w, questions)
	}))
	t.Cleanup(server.Close)
	return server
}

// mockAnthropicSSEServerPerCall calls genQuestions() fresh on every incoming
// request, so each call can return distinguishable content. This matters for
// concurrency tests: ANTHROPIC_BASE_URL is a single process-wide env var, so
// concurrent goroutines necessarily hit the very same mock server — there is no
// way to route "batch A" and "batch B" to two different upstreams. genQuestions
// must be safe for concurrent invocation (e.g. via sync/atomic).
func mockAnthropicSSEServerPerCall(t *testing.T, genQuestions func() []map[string]interface{}) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSSEQuestions(t, w, genQuestions())
	}))
	t.Cleanup(server.Close)
	return server
}

// mockAnthropicErrorServer simulates an upstream failure (401/429/5xx) using
// Anthropic's documented error envelope shape.
func mockAnthropicErrorServer(t *testing.T, statusCode int, errType string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"type": "error",
			"error": map[string]string{
				"type":    errType,
				"message": "mock upstream error — this text must never be relayed verbatim if it embeds a secret",
			},
		})
	}))
	t.Cleanup(server.Close)
	return server
}

// mockAnthropicSlowServer never responds fast enough — used for the timeout test.
func mockAnthropicSlowServer(t *testing.T, delay time.Duration) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func withAnthropicBaseURL(t *testing.T, url string) {
	t.Helper()
	t.Setenv(anthropicBaseURLEnvVar, url)
}

// ================================================================================
// Section A — validation de la requête (contrat §3), sans dépendance au client IA :
// la validation doit intervenir avant tout appel amont.
// ================================================================================

func TestGenerateQuestions_MethodNotAllowed(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(validAIConfig())

	req := httptest.NewRequest("GET", "/api/generate-questions", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for GET /api/generate-questions, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGenerateQuestions_NoAPIKey_Returns409(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(config.AIConfig{AnthropicAPIKey: "", Model: "claude-opus-5", TimeoutSeconds: 300, MaxQuestions: 200})

	w := postGenerateQuestions(server, baseGenerateRequest())

	if w.Code != http.StatusConflict {
		t.Fatalf("Expected 409 when no API key configured, got %d: %s", w.Code, w.Body.String())
	}
	body := decodeJSONBody(t, w)
	if body["code"] != "no_api_key" {
		t.Errorf("Expected code=no_api_key, got %v", body["code"])
	}
}

func TestGenerateQuestions_PayloadValidation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(m map[string]interface{})
		wantErr string // substring expected nowhere in particular — we just assert 400 + code
	}{
		{
			name:   "missing theme",
			mutate: func(m map[string]interface{}) { m["theme"] = "" },
		},
		{
			name:   "missing categories",
			mutate: func(m map[string]interface{}) { m["categories"] = []string{} },
		},
		{
			name:   "unknown category",
			mutate: func(m map[string]interface{}) { m["categories"] = []string{"NOT_A_REAL_CATEGORY"} },
		},
		{
			name:   "missing difficulties",
			mutate: func(m map[string]interface{}) { m["difficulties"] = []string{} },
		},
		// v6.1.0 (#137 Batch 2b, contract ai-generation.md §3bis) — populations
		// replaces the singular population (string); same validation shape as
		// difficulties (≥1, enum, no duplicate), tested explicitly rather than
		// assumed identical because it's a distinct code path
		// (validateGenerateRequest, ai_generator.go).
		{
			name:   "missing populations (empty array)",
			mutate: func(m map[string]interface{}) { m["populations"] = []string{} },
		},
		{
			name:   "populations contains an unknown value",
			mutate: func(m map[string]interface{}) { m["populations"] = []string{"Extraterrestre (âge inconnu)"} },
		},
		{
			name: "populations contains a duplicate value",
			mutate: func(m map[string]interface{}) {
				m["populations"] = []string{"Adulte (18-64 ans)", "Adulte (18-64 ans)"}
			},
		},
		// ⚠️ BREAKING (contract §3bis) — a v6.0.0 caller still posting the old
		// singular "population" field gets a 400: the key is unrecognized by
		// generateQuestionsRequest, so "populations" is left at its zero value
		// (empty slice) and fails the same "≥1 element" rule as if omitted
		// entirely. No silent fallback to the old field name — a client that
		// hasn't been redeployed must be visible, not silently rescued
		// (contract §3ter).
		{
			name: "old singular 'population' field is not recognized (BREAKING v6.1.0)",
			mutate: func(m map[string]interface{}) {
				delete(m, "populations")
				m["population"] = "Adulte (18-64 ans)"
			},
		},
		// objectives is new and optional (contract §3bis) — only its length
		// cap is validated; empty/absent is explicitly accepted (covered
		// implicitly by every other case in this table, which all leave
		// baseGenerateRequest()'s objectives:"" untouched).
		{
			name:   "objectives exceeds 2000 characters",
			mutate: func(m map[string]interface{}) { m["objectives"] = strings.Repeat("x", 2001) },
		},
		{
			name: "distribution sum != 100",
			mutate: func(m map[string]interface{}) {
				m["distribution"] = map[string]interface{}{"SPEEDY": 50, "QCM": 40, "MEMORY": 0, "MEMOTION": 0}
			},
		},
		{
			name: "distribution all zero",
			mutate: func(m map[string]interface{}) {
				m["distribution"] = map[string]interface{}{"SPEEDY": 0, "QCM": 0, "MEMORY": 0, "MEMOTION": 0}
			},
		},
		{
			name:   "volume count over max_questions",
			mutate: func(m map[string]interface{}) { m["volume"] = map[string]interface{}{"mode": "count", "value": 500} },
		},
		{
			name:   "volume count zero",
			mutate: func(m map[string]interface{}) { m["volume"] = map[string]interface{}{"mode": "count", "value": 0} },
		},
		{
			name:   "volume duration under 5 minutes",
			mutate: func(m map[string]interface{}) { m["volume"] = map[string]interface{}{"mode": "duration", "value": 2} },
		},
		{
			name:   "volume duration over 240 minutes",
			mutate: func(m map[string]interface{}) { m["volume"] = map[string]interface{}{"mode": "duration", "value": 300} },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, _ := setupTestHTTPServer(t)
			setAIConfig(validAIConfig())

			req := baseGenerateRequest()
			tt.mutate(req)

			w := postGenerateQuestions(server, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("Expected 400 for %q, got %d: %s", tt.name, w.Code, w.Body.String())
			}
			body := decodeJSONBody(t, w)
			if body["code"] != "invalid_request" {
				t.Errorf("Expected code=invalid_request for %q, got %v", tt.name, body["code"])
			}
		})
	}
}

// ================================================================================
// Section B — allocation d'ID (contrat §5.1), sans dépendance au client IA.
// ================================================================================

// newUploadQuestionRequest builds a multipart POST to the existing (pre-#8) manual
// creation endpoint, matching the fields read by handleUploadQuestion.
func newUploadQuestionRequest(question, answer string) *http.Request {
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	mw.WriteField("question", question)
	mw.WriteField("answer", answer)
	mw.WriteField("points", "10")
	mw.WriteField("time", "20")
	mw.WriteField("type", "SPEEDY")
	mw.Close()
	req := httptest.NewRequest("POST", "/questions", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

// TestConcurrentQuestionUpload_UniqueIDs is the regression test for R2 (course
// d'allocation d'ID). Run with `go test -race` (qa/CI responsibility, cf. plan
// risk R2) — without the mutex + os.Mkdir exclusif required by contract §5.1, two
// goroutines can compute the same free ID and one silently overwrites the other's
// question.json, which this test also detects functionally (missing distinct
// question text) even without -race.
func TestConcurrentQuestionUpload_UniqueIDs(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	const n = 20
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := newUploadQuestionRequest(fmt.Sprintf("Concurrent question #%d", i), "reponse")
			w := httptest.NewRecorder()
			server.mux.ServeHTTP(w, req)
		}(i)
	}
	wg.Wait()

	// Every one of the N concurrent uploads must have landed in its own directory,
	// with its own distinct QUESTION text intact — no collision, no overwrite.
	seenQuestions := map[string]bool{}
	dir := filepath.Join(dataDir, "files", "questions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		q := readQuestionFile(t, dataDir, e.Name())
		if text, ok := q["QUESTION"].(string); ok {
			seenQuestions[text] = true
		}
	}
	if len(seenQuestions) != n {
		t.Errorf("Expected %d distinct questions to survive %d concurrent uploads, got %d distinct texts (some were overwritten by an ID collision)", n, n, len(seenQuestions))
	}
}

// TestGenerateQuestions_IDExhausted_Returns507 saturates IDs 1..999 then requests one
// more question via the batch endpoint. The pre-existing buggy fallback
// (`return "999"`) would silently overwrite question 999; contract §5.1 requires a
// hard 507 instead.
// TestGenerateQuestions_IDExhausted_Returns507 — updated for the async job
// model (#137, contract ai-multi-provider.md §9): id_exhausted no longer
// comes back as an HTTP response (the endpoint is 202-only); it surfaces as
// AI_GENERATION_PROGRESS STATE=FAILED, ERROR_CODE=id_exhausted.
func TestGenerateQuestions_IDExhausted_Returns507(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	setAIConfig(validAIConfig())

	questionsDir := filepath.Join(dataDir, "files", "questions")
	for i := 1; i <= 999; i++ {
		id := fmt.Sprintf("%d", i)
		qDir := filepath.Join(questionsDir, id)
		if err := os.MkdirAll(qDir, 0755); err != nil {
			t.Fatalf("Failed to pre-create question dir %s: %v", id, err)
		}
		marker := fmt.Sprintf(`{"ID":"%s","QUESTION":"pre-existing #%s","ANSWER":"x","POINTS":"10","TIME":"20"}`, id, id)
		if err := os.WriteFile(filepath.Join(qDir, "question.json"), []byte(marker), 0644); err != nil {
			t.Fatalf("Failed to write marker for %s: %v", id, err)
		}
	}

	upstream := mockAnthropicSSEServer(t, []map[string]interface{}{
		llmQuestion(map[string]interface{}{"TYPE": "SPEEDY", "ANSWER": "peu importe"}),
	})
	withAnthropicBaseURL(t, upstream.URL)

	conn := dialAdminWS(t, server)
	expectAccepted(t, postGenerateQuestions(server, baseGenerateRequest()))

	final := waitForJobTerminalState(t, conn, 5*time.Second)
	if final["STATE"] != "FAILED" {
		t.Fatalf("Expected STATE=FAILED when no free ID remains, got %v (msg=%v)", final["STATE"], final)
	}
	if final["ERROR_CODE"] != "id_exhausted" {
		t.Errorf("Expected ERROR_CODE=id_exhausted, got %v", final["ERROR_CODE"])
	}

	// Question 999 must be untouched — the old silent-overwrite fallback is the bug
	// this endpoint must not reintroduce.
	q999 := readQuestionFile(t, dataDir, "999")
	if q999["QUESTION"] != "pre-existing #999" {
		t.Errorf("Question 999 was overwritten on id_exhausted — expected it untouched, got QUESTION=%v", q999["QUESTION"])
	}
}

// ================================================================================
// Section C — intégration avec client Anthropic mocké (contrat §3, §4, §5).
// Voir le bloc de commentaire en tête de fichier pour l'hypothèse ANTHROPIC_BASE_URL.
// ================================================================================

func TestGenerateQuestions_MappingPerType(t *testing.T) {
	tests := []struct {
		name       string
		llm        map[string]interface{}
		wantType   string
		wantPoints string
		wantTarget string
		checkExtra func(t *testing.T, q map[string]interface{})
	}{
		{
			name: "SPEEDY",
			llm: llmQuestion(map[string]interface{}{
				"TYPE": "SPEEDY", "ANSWER": "Le Grand Bleu",
			}),
			wantType:   "SPEEDY",
			wantPoints: "20", // DIFFICULTY=Moyen
			wantTarget: "PLAYER",
			checkExtra: func(t *testing.T, q map[string]interface{}) {
				if q["ANSWER"] != "Le Grand Bleu" {
					t.Errorf("Expected ANSWER=Le Grand Bleu, got %v", q["ANSWER"])
				}
			},
		},
		{
			name: "QCM",
			llm: llmQuestion(map[string]interface{}{
				"TYPE": "QCM",
				"QCM_ANSWERS": map[string]string{
					"RED": "Paris", "GREEN": "Lyon", "YELLOW": "Nice", "BLUE": "Lille",
				},
				"QCM_CORRECT": "RED",
			}),
			wantType:   "QCM",
			wantPoints: "20",
			wantTarget: "TEAM",
			checkExtra: func(t *testing.T, q map[string]interface{}) {
				if hints, ok := q["QCM_HINTS_ENABLED"]; !ok || hints != false {
					t.Errorf("Expected QCM_HINTS_ENABLED=false, got %v", hints)
				}
				qcmAnswers, ok := q["QCM_ANSWERS"].(map[string]interface{})
				if !ok || qcmAnswers["RED"] != "Paris" {
					t.Errorf("Expected QCM_ANSWERS.RED=Paris, got %v", q["QCM_ANSWERS"])
				}
			},
		},
		{
			name: "MEMORY",
			llm: llmQuestion(map[string]interface{}{
				"TYPE": "MEMORY",
				"MEMORY_PAIRS": []map[string]string{
					{"LEFT": "France", "RIGHT": "Paris"},
					{"LEFT": "Italie", "RIGHT": "Rome"},
				},
			}),
			wantType:   "MEMORY",
			wantPoints: "0",
			wantTarget: "TEAM",
			checkExtra: func(t *testing.T, q map[string]interface{}) {
				if q["ANSWER"] != "2 paires" {
					t.Errorf("Expected ANSWER=\"2 paires\", got %v", q["ANSWER"])
				}
				if q["MEMORY_MODE"] != "SOLO" {
					t.Errorf("Expected MEMORY_MODE=SOLO, got %v", q["MEMORY_MODE"])
				}
				pairs, ok := q["MEMORY_PAIRS"].([]interface{})
				if !ok || len(pairs) != 2 {
					t.Fatalf("Expected 2 MEMORY_PAIRS, got %v", q["MEMORY_PAIRS"])
				}
				p0 := pairs[0].(map[string]interface{})
				if p0["ID"] != float64(1) {
					t.Errorf("Expected MEMORY_PAIRS[0].ID=1, got %v", p0["ID"])
				}
				card1, _ := p0["CARD1"].(map[string]interface{})
				if card1["TEXT"] != "France" || card1["IS_IMAGE"] != false {
					t.Errorf("Expected CARD1={TEXT:France,IS_IMAGE:false}, got %v", card1)
				}
				card2, _ := p0["CARD2"].(map[string]interface{})
				if card2["TEXT"] != "Paris" {
					t.Errorf("Expected CARD2.TEXT=Paris, got %v", card2)
				}
				cfg, ok := q["MEMORY_CONFIG"].(map[string]interface{})
				if !ok || cfg["POINTS_PER_PAIR"] != float64(10) {
					t.Errorf("Expected MEMORY_CONFIG.POINTS_PER_PAIR=10, got %v", q["MEMORY_CONFIG"])
				}
			},
		},
		{
			name: "MEMOTION",
			llm: llmQuestion(map[string]interface{}{
				"TYPE": "MEMOTION",
				"MOTION_CARDS": []map[string]interface{}{
					{"RECTO_THEME": "Sport", "QUESTION_TEXT": "Quel sport ?", "ANSWER_TEXT": "Football", "DIFFICULTY": 1},
					{"RECTO_THEME": "Sport", "QUESTION_TEXT": "Quel pays ?", "ANSWER_TEXT": "France", "DIFFICULTY": 2},
					{"RECTO_THEME": "Sport", "QUESTION_TEXT": "Quelle année ?", "ANSWER_TEXT": "1998", "DIFFICULTY": 3},
					{"RECTO_THEME": "Sport", "QUESTION_TEXT": "Quel stade ?", "ANSWER_TEXT": "Stade de France", "DIFFICULTY": 1},
				},
			}),
			wantType:   "MEMOTION",
			wantPoints: "1",
			wantTarget: "TEAM",
			checkExtra: func(t *testing.T, q map[string]interface{}) {
				if q["ANSWER"] != "4 cartes" {
					t.Errorf("Expected ANSWER=\"4 cartes\", got %v", q["ANSWER"])
				}
				if q["MOTION_MODE"] != "CHACUN_SON_TOUR" {
					t.Errorf("Expected MOTION_MODE=CHACUN_SON_TOUR, got %v", q["MOTION_MODE"])
				}
				cards, ok := q["MOTION_CARDS"].([]interface{})
				if !ok || len(cards) != 4 {
					t.Fatalf("Expected 4 MOTION_CARDS, got %v", q["MOTION_CARDS"])
				}
				c0 := cards[0].(map[string]interface{})
				if c0["ID"] != "mc-1" {
					t.Errorf("Expected MOTION_CARDS[0].ID=mc-1, got %v", c0["ID"])
				}
				cfg, ok := q["MOTION_CONFIG"].(map[string]interface{})
				if !ok || cfg["POINTS_1_STAR"] != float64(1) || cfg["POINTS_3_STAR"] != float64(5) {
					t.Errorf("Expected MOTION_CONFIG {1:1,3:5}, got %v", q["MOTION_CONFIG"])
				}
			},
		},
		{
			// #137 Batch 2a (planner-20260806-121743-qualif-137.md §2):
			// ARDOISE joined generableQuestionTypes — structurally SPEEDY +
			// ARDOISE_KEYBOARD_TYPE, POINTS_TARGET=TEAM (not PLAYER, unlike
			// SPEEDY — matches the real sample question.json cited in the
			// plan and the Q2.2 arbitrage).
			name: "ARDOISE",
			llm: llmQuestion(map[string]interface{}{
				"TYPE": "ARDOISE", "ANSWER": "Tokyo", "ARDOISE_KEYBOARD_TYPE": "AZERTY",
			}),
			wantType:   "ARDOISE",
			wantPoints: "20", // DIFFICULTY=Moyen
			wantTarget: "TEAM",
			checkExtra: func(t *testing.T, q map[string]interface{}) {
				if q["ANSWER"] != "Tokyo" {
					t.Errorf("Expected ANSWER=Tokyo, got %v", q["ANSWER"])
				}
				if q["ARDOISE_KEYBOARD_TYPE"] != "AZERTY" {
					t.Errorf("Expected ARDOISE_KEYBOARD_TYPE=AZERTY, got %v", q["ARDOISE_KEYBOARD_TYPE"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, dataDir := setupTestHTTPServer(t)
			setAIConfig(validAIConfig())

			upstream := mockAnthropicSSEServer(t, []map[string]interface{}{tt.llm})
			withAnthropicBaseURL(t, upstream.URL)

			req := baseGenerateRequest()
			req["distribution"] = map[string]interface{}{"SPEEDY": 0, "QCM": 0, "MEMORY": 0, "MEMOTION": 0}
			req["distribution"].(map[string]interface{})[tt.wantType] = 100

			// Async job model (#137): the endpoint only accepts the request;
			// the result is observed via AI_GENERATION_PROGRESS, then read
			// back from disk (the response body no longer carries created[]).
			conn := dialAdminWS(t, server)
			expectAccepted(t, postGenerateQuestions(server, req))
			final := waitForJobTerminalState(t, conn, 5*time.Second)
			if final["STATE"] != "DONE" {
				t.Fatalf("Expected STATE=DONE, got %v (msg=%v)", final["STATE"], final)
			}
			if final["CREATED_COUNT"] != float64(1) {
				t.Fatalf("Expected CREATED_COUNT=1, got %v (msg=%v)", final["CREATED_COUNT"], final)
			}

			ids := collectQuestionIDs(t, dataDir)
			if len(ids) != 1 {
				t.Fatalf("Expected 1 question file, got %d", len(ids))
			}
			id := ids[0]

			q := readQuestionFile(t, dataDir, id)
			if q["TYPE"] != tt.wantType {
				t.Errorf("Expected TYPE=%s, got %v", tt.wantType, q["TYPE"])
			}
			if q["POINTS"] != tt.wantPoints {
				t.Errorf("Expected POINTS=%q (string), got %v (%T)", tt.wantPoints, q["POINTS"], q["POINTS"])
			}
			if q["POINTS_TARGET"] != tt.wantTarget {
				t.Errorf("Expected POINTS_TARGET=%s, got %v", tt.wantTarget, q["POINTS_TARGET"])
			}
			if _, isString := q["TIME"].(string); !isString {
				t.Errorf("Expected TIME as a string (contract §5.2), got %T: %v", q["TIME"], q["TIME"])
			}
			if q["ORDER"] != float64(1) {
				t.Errorf("Expected ORDER=1 (first question, max+1+0), got %v", q["ORDER"])
			}
			if _, hasMedia := q["MEDIA"]; hasMedia {
				t.Errorf("Expected no MEDIA field on a generated question, got %v", q["MEDIA"])
			}
			tt.checkExtra(t, q)
		})
	}
}

func TestGenerateQuestions_ValidationDropsInvalidQuestions(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	setAIConfig(validAIConfig())

	valid := llmQuestion(map[string]interface{}{"TYPE": "SPEEDY", "QUESTION": "Valide", "ANSWER": "ok"})
	missingAnswer := llmQuestion(map[string]interface{}{"TYPE": "SPEEDY", "QUESTION": "Sans reponse", "ANSWER": ""})
	qcmThreeAnswers := llmQuestion(map[string]interface{}{
		"TYPE": "QCM", "QUESTION": "QCM incomplet",
		"QCM_ANSWERS": map[string]string{"RED": "A", "GREEN": "B", "YELLOW": "C", "BLUE": ""},
		"QCM_CORRECT": "RED",
	})
	memoryOnePair := llmQuestion(map[string]interface{}{
		"TYPE": "MEMORY", "QUESTION": "Memory incomplet",
		"MEMORY_PAIRS": []map[string]string{{"LEFT": "A", "RIGHT": "B"}},
	})
	unknownCategory := llmQuestion(map[string]interface{}{
		"TYPE": "SPEEDY", "QUESTION": "Mauvaise categorie", "ANSWER": "x", "CATEGORY": "SCIENCE_FICTION_NOT_REQUESTED",
	})

	upstream := mockAnthropicSSEServer(t, []map[string]interface{}{
		valid, missingAnswer, qcmThreeAnswers, memoryOnePair, unknownCategory,
	})
	withAnthropicBaseURL(t, upstream.URL)

	req := baseGenerateRequest()
	req["distribution"] = map[string]interface{}{"SPEEDY": 40, "QCM": 30, "MEMORY": 30, "MEMOTION": 0}
	req["volume"] = map[string]interface{}{"mode": "count", "value": 5}

	// Async job model (#137): a batch with some invalid questions is still a
	// successful batch — the job reaches DONE, not an HTTP-level error
	// (contract ai-generation.md §3 "skipped questions are not an error").
	conn := dialAdminWS(t, server)
	expectAccepted(t, postGenerateQuestions(server, req))
	final := waitForJobTerminalState(t, conn, 5*time.Second)
	if final["STATE"] != "DONE" {
		t.Fatalf("Expected STATE=DONE (skipped questions are not an error, contract §3), got %v (msg=%v)", final["STATE"], final)
	}
	if final["CREATED_COUNT"] != float64(1) {
		t.Errorf("Expected CREATED_COUNT=1, got %v", final["CREATED_COUNT"])
	}
	if final["SKIPPED_COUNT"] != float64(4) {
		t.Errorf("Expected SKIPPED_COUNT=4, got %v", final["SKIPPED_COUNT"])
	}
	if countQuestionDirs(t, dataDir) != 1 {
		t.Errorf("Expected exactly 1 question directory on disk, got %d", countQuestionDirs(t, dataDir))
	}
}

// TestGenerateQuestions_ZeroValidQuestions_Returns502 — updated for the async
// job model (#137): a batch producing zero usable questions is content
// attrition, not a provider/transport failure — it does NOT count toward
// max_consecutive_failures (ai_batching_test.go assumption #4, matched by
// TestAIBatching_SequentialIDAllocation_NoGapMisuse), so with a single
// requested batch the job still reaches DONE, just with CREATED_COUNT=0. The
// #8-era "never a false 200" concern (R9) was about a synchronous response
// claiming success while silently returning nothing; the async progress
// message here is not misleading — it accurately reports 0 created, which is
// exactly what a reader of AI_GENERATION_PROGRESS needs to see. What DOES
// still matter, and what this test verifies, is CA10: never a question
// directory left behind for content that was never usable.
func TestGenerateQuestions_ZeroValidQuestions_Returns502(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	setAIConfig(validAIConfig())

	onlyInvalid := llmQuestion(map[string]interface{}{"TYPE": "SPEEDY", "ANSWER": ""})
	upstream := mockAnthropicSSEServer(t, []map[string]interface{}{onlyInvalid})
	withAnthropicBaseURL(t, upstream.URL)

	conn := dialAdminWS(t, server)
	expectAccepted(t, postGenerateQuestions(server, baseGenerateRequest()))

	final := waitForJobTerminalState(t, conn, 5*time.Second)
	if final["STATE"] != "DONE" {
		t.Fatalf("Expected STATE=DONE (content attrition alone never fails the job), got %v (msg=%v)", final["STATE"], final)
	}
	if final["CREATED_COUNT"] != float64(0) {
		t.Errorf("Expected CREATED_COUNT=0, got %v", final["CREATED_COUNT"])
	}
	if countQuestionDirs(t, dataDir) != 0 {
		t.Errorf("Expected no question directory created when nothing was ever usable, got %d", countQuestionDirs(t, dataDir))
	}
}

func TestGenerateQuestions_Success_BroadcastsAndPreservesExistingQuestions(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	setAIConfig(validAIConfig())

	// Pre-existing manual questions must survive untouched (contract CA7).
	server.mux.ServeHTTP(httptest.NewRecorder(), newUploadQuestionRequest("Question manuelle 1", "Reponse 1"))
	server.mux.ServeHTTP(httptest.NewRecorder(), newUploadQuestionRequest("Question manuelle 2", "Reponse 2"))

	before := map[string]map[string]interface{}{
		"1": readQuestionFile(t, dataDir, "1"),
		"2": readQuestionFile(t, dataDir, "2"),
	}

	broadcastCalled := 0
	server.OnQuestionUpload = func() { broadcastCalled++ }

	upstream := mockAnthropicSSEServer(t, []map[string]interface{}{
		llmQuestion(map[string]interface{}{"TYPE": "SPEEDY", "QUESTION": "Generee 1", "ANSWER": "ok1"}),
		llmQuestion(map[string]interface{}{"TYPE": "SPEEDY", "QUESTION": "Generee 2", "ANSWER": "ok2"}),
	})
	withAnthropicBaseURL(t, upstream.URL)

	conn := dialAdminWS(t, server)
	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 2}

	expectAccepted(t, postGenerateQuestions(server, req))
	final := waitForJobTerminalState(t, conn, 5*time.Second)
	if final["STATE"] != "DONE" {
		t.Fatalf("Expected STATE=DONE, got %v (msg=%v)", final["STATE"], final)
	}
	if final["CREATED_COUNT"] != float64(2) {
		t.Fatalf("Expected CREATED_COUNT=2, got %v", final["CREATED_COUNT"])
	}

	if broadcastCalled == 0 {
		t.Error("Expected h.OnQuestionUpload() to be called after a successful generation (contract §3 'Effet de bord obligatoire') — without it, QuestionsPage only refreshes after a page reload")
	}

	// Snapshot check: pre-existing questions 1 and 2 must be byte-identical.
	for id, snapshot := range before {
		after := readQuestionFile(t, dataDir, id)
		snapshotJSON, _ := json.Marshal(snapshot)
		afterJSON, _ := json.Marshal(after)
		if string(snapshotJSON) != string(afterJSON) {
			t.Errorf("Pre-existing question %s was modified by generation.\nBefore: %s\nAfter:  %s", id, snapshotJSON, afterJSON)
		}
	}
}

// TestGenerateQuestions_ConcurrentBatches_NoIDCollision (#8) is superseded by
// #137's single-job-at-a-time invariant (contract ai-multi-provider.md §9,
// §12): two concurrent POST /api/generate-questions no longer both proceed —
// the second one now correctly gets 409 generation_in_progress, which is
// exactly the behavior this old test's premise (both succeeding
// concurrently) would now contradict. The scenario it originally guarded
// (concurrent callers never colliding on IDs) is covered two ways now:
//   - TestAIJob_SecondGenerateWhileRunning_Returns409 (ai_job_test.go):
//     a second POST while a job runs is rejected, not raced.
//   - TestAIBatching_ConcurrentManualUpload_NoIDCollision_Race
//     (ai_batching_test.go) and TestConcurrentQuestionResolveDir_UniqueIDs
//     (http_test.go): the underlying ID-allocation lock itself is still
//     exercised under real concurrency (a running job's batches racing a
//     manual upload), just not via two concurrent generate-questions calls,
//     which the new contract makes impossible by construction.

// TestGenerateQuestions_UpstreamErrors — updated for the async job model
// (#137): 502/504 no longer come back as the HTTP response to POST
// /api/generate-questions (which is 202-only, contract ai-multi-provider.md
// §9 — "Les erreurs 502/504 de #8 ne sont plus renvoyées par cet endpoint :
// elles surviennent pendant le job et transitent désormais par la
// progression"); they surface as AI_GENERATION_PROGRESS STATE=FAILED with
// the matching ERROR_CODE. maxConsecutiveFailures defaults to 2
// (validAIConfig doesn't set it), so a single-batch job (volume=5 <=
// batch_size=20) exhausts its retries after 2 failed attempts against the
// same persistently-failing mock.
func TestGenerateQuestions_UpstreamErrors(t *testing.T) {
	t.Run("401 invalid key maps to upstream_error", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		setAIConfig(validAIConfig())
		upstream := mockAnthropicErrorServer(t, http.StatusUnauthorized, "authentication_error")
		withAnthropicBaseURL(t, upstream.URL)

		conn := dialAdminWS(t, server)
		expectAccepted(t, postGenerateQuestions(server, baseGenerateRequest()))

		final := waitForJobTerminalState(t, conn, 5*time.Second)
		if final["STATE"] != "FAILED" {
			t.Fatalf("Expected STATE=FAILED on persistent upstream 401, got %v (msg=%v)", final["STATE"], final)
		}
		if final["ERROR_CODE"] != "upstream_error" {
			t.Errorf("Expected ERROR_CODE=upstream_error, got %v", final["ERROR_CODE"])
		}
		if strings.Contains(fmt.Sprintf("%v", final), "sk-ant-test-key-0001") {
			t.Error("Progress message must never leak the configured API key (contract S2/CA12)")
		}
	})

	t.Run("429 rate limit maps to provider_quota", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		setAIConfig(validAIConfig())
		upstream := mockAnthropicErrorServer(t, http.StatusTooManyRequests, "rate_limit_error")
		withAnthropicBaseURL(t, upstream.URL)

		conn := dialAdminWS(t, server)
		expectAccepted(t, postGenerateQuestions(server, baseGenerateRequest()))

		final := waitForJobTerminalState(t, conn, 10*time.Second)
		if final["STATE"] != "FAILED" {
			t.Fatalf("Expected STATE=FAILED on persistent upstream 429, got %v (msg=%v)", final["STATE"], final)
		}
		if final["ERROR_CODE"] != "provider_quota" {
			t.Errorf("Expected ERROR_CODE=provider_quota on persistent 429 (contract ai-multi-provider.md §3), got %v", final["ERROR_CODE"])
		}
	})

	t.Run("upstream timeout maps to timeout", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		ai := validAIConfig()
		ai.TimeoutSeconds = 1
		setAIConfig(ai)
		upstream := mockAnthropicSlowServer(t, 3*time.Second)
		withAnthropicBaseURL(t, upstream.URL)

		conn := dialAdminWS(t, server)
		expectAccepted(t, postGenerateQuestions(server, baseGenerateRequest()))

		final := waitForJobTerminalState(t, conn, 10*time.Second)
		if final["STATE"] != "FAILED" {
			t.Fatalf("Expected STATE=FAILED when upstream exceeds ai.timeout_seconds, got %v (msg=%v)", final["STATE"], final)
		}
		if final["ERROR_CODE"] != "timeout" {
			t.Errorf("Expected ERROR_CODE=timeout, got %v", final["ERROR_CODE"])
		}
	})
}

// TestBuildGenerationPrompt_MEMOTIONGuidance regression-tests the fix for
// _work/handoff/task-dev-backend-20260806-112859.md (qa-20260806-111416.md
// §5.4): MEMOTION was massively under-produced by Groq (2% vs 15%
// requested) because the json_schema output has no minItems/maxItems
// support (nothing stops the model emitting fewer than the 4 cards
// validateGeneratedQuestions requires) and the RECTO_THEME/QUESTION_TEXT/
// ANSWER_TEXT three-way relationship isn't obvious from field names alone.
// The fix adds explicit prose guidance + a few-shot example to the prompt,
// gated on distribution["MEMOTION"] > 0 so batches that don't request the
// type don't pay extra prompt tokens for guidance they don't need.
func TestBuildGenerationPrompt_MEMOTIONGuidance(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	base := generateQuestionsRequest{
		Theme:        "Cinéma français des années 80",
		Populations:  []string{"Adulte (18-64 ans)"},
		Language:     "Français",
		Difficulties: []string{"Moyen"},
		Categories:   []string{"ENTERTAINMENT"},
	}

	t.Run("MEMOTION requested: guidance and few-shot example present", func(t *testing.T) {
		req := base
		req.Distribution = map[string]int{"SPEEDY": 85, "MEMOTION": 15}
		prompt := server.buildGenerationPrompt(req, 5, nil)

		for _, want := range []string{"RECTO_THEME", "QUESTION_TEXT", "ANSWER_TEXT", "ENTRE 4 ET 12"} {
			if !strings.Contains(prompt, want) {
				t.Errorf("Expected prompt to contain %q when MEMOTION is requested, got:\n%s", want, prompt)
			}
		}
	})

	t.Run("MEMOTION not requested: no extra guidance", func(t *testing.T) {
		req := base
		req.Distribution = map[string]int{"SPEEDY": 100, "MEMOTION": 0}
		prompt := server.buildGenerationPrompt(req, 5, nil)

		if strings.Contains(prompt, "RECTO_THEME") {
			t.Errorf("Expected no MEMOTION-specific guidance in prompt when MEMOTION isn't requested, got:\n%s", prompt)
		}
	})
}

// TestBuildQuestionSchema_ARDOISEVariant_SatisfiesGroqStrictMode (#137 Batch
// 2a T2.5, planner-20260806-121743-qualif-137.md §2): Groq's json_schema
// strict mode has 3 structural rules (ai-multi-provider.md §6/§7's comment
// above buildQuestionSchema: root not anyOf, every property listed in
// required, additionalProperties:false everywhere) — the 4 existing variants
// already satisfy them, this locks in that the 5th (ARDOISE) does too,
// rather than relying on the end-to-end Groq mock tests to notice a missing
// required entry indirectly.
func TestBuildQuestionSchema_ARDOISEVariant_SatisfiesGroqStrictMode(t *testing.T) {
	schema := buildQuestionSchema([]string{"CAT"}, []string{"Facile"})

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("Expected schema.properties to be a map, got %T", schema["properties"])
	}
	questions, ok := props["questions"].(map[string]any)
	if !ok {
		t.Fatalf("Expected properties.questions to be a map, got %T", props["questions"])
	}
	items, ok := questions["items"].(map[string]any)
	if !ok {
		t.Fatalf("Expected questions.items to be a map, got %T", questions["items"])
	}
	anyOf, ok := items["anyOf"].([]any)
	if !ok {
		t.Fatalf("Expected items.anyOf to be a slice, got %T", items["anyOf"])
	}

	var ardoise map[string]any
	for _, v := range anyOf {
		variant, ok := v.(map[string]any)
		if !ok {
			continue
		}
		p, ok := variant["properties"].(map[string]any)
		if !ok {
			continue
		}
		typeField, ok := p["TYPE"].(map[string]any)
		if !ok {
			continue
		}
		if typeField["const"] == "ARDOISE" {
			ardoise = variant
			break
		}
	}
	if ardoise == nil {
		t.Fatal("Expected an ARDOISE variant (TYPE.const=\"ARDOISE\") in the anyOf list")
	}

	if ardoise["additionalProperties"] != false {
		t.Errorf("Expected additionalProperties=false on the ARDOISE variant (Groq strict mode rule), got %v", ardoise["additionalProperties"])
	}

	propMap, ok := ardoise["properties"].(map[string]any)
	if !ok {
		t.Fatalf("Expected ARDOISE variant properties to be a map, got %T", ardoise["properties"])
	}
	required, ok := ardoise["required"].([]string)
	if !ok {
		t.Fatalf("Expected ARDOISE variant required to be []string, got %T", ardoise["required"])
	}
	requiredSet := make(map[string]bool, len(required))
	for _, r := range required {
		requiredSet[r] = true
	}
	for prop := range propMap {
		if !requiredSet[prop] {
			t.Errorf("Groq strict mode requires every property in required — %q is a property but missing from required", prop)
		}
	}
	for _, r := range required {
		if _, ok := propMap[r]; !ok {
			t.Errorf("required lists %q but it isn't a property", r)
		}
	}
	for _, want := range []string{"ANSWER", "ARDOISE_KEYBOARD_TYPE"} {
		if !requiredSet[want] {
			t.Errorf("Expected %q in the ARDOISE variant's required list, got %v", want, required)
		}
	}

	kb, ok := propMap["ARDOISE_KEYBOARD_TYPE"].(map[string]any)
	if !ok {
		t.Fatal("Expected an ARDOISE_KEYBOARD_TYPE property on the ARDOISE variant")
	}
	enumVals, ok := kb["enum"].([]string)
	if !ok || len(enumVals) != 2 || enumVals[0] != "AZERTY" || enumVals[1] != "NUMPAD" {
		t.Errorf("Expected ARDOISE_KEYBOARD_TYPE enum=[AZERTY NUMPAD], got %v", kb["enum"])
	}
}

// TestBuildGenerationPrompt_ARDOISEGuidance (#137 Batch 2a T2.4, arbitrage
// utilisateur Q2.1): the model chooses ARDOISE_KEYBOARD_TYPE itself
// (NUMPAD for a purely numeric answer, AZERTY otherwise) — the schema's enum
// can't express a content-dependent rule, so the prompt has to state it in
// prose. Gated on distribution["ARDOISE"] > 0, same pattern as MEMOTION.
func TestBuildGenerationPrompt_ARDOISEGuidance(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	base := generateQuestionsRequest{
		Theme:        "Histoire de France",
		Populations:  []string{"Adulte (18-64 ans)"},
		Language:     "Français",
		Difficulties: []string{"Moyen"},
		Categories:   []string{"HISTORY"},
	}

	t.Run("ARDOISE requested: keyboard-choice guidance present", func(t *testing.T) {
		req := base
		req.Distribution = map[string]int{"SPEEDY": 80, "ARDOISE": 20}
		prompt := server.buildGenerationPrompt(req, 5, nil)

		for _, want := range []string{"ARDOISE_KEYBOARD_TYPE", "NUMPAD", "AZERTY"} {
			if !strings.Contains(prompt, want) {
				t.Errorf("Expected prompt to contain %q when ARDOISE is requested, got:\n%s", want, prompt)
			}
		}
	})

	t.Run("ARDOISE not requested: no extra guidance", func(t *testing.T) {
		req := base
		req.Distribution = map[string]int{"SPEEDY": 100, "ARDOISE": 0}
		prompt := server.buildGenerationPrompt(req, 5, nil)

		if strings.Contains(prompt, "ARDOISE_KEYBOARD_TYPE") {
			t.Errorf("Expected no ARDOISE-specific guidance in prompt when ARDOISE isn't requested, got:\n%s", prompt)
		}
	})
}

// TestBuildGenerationPrompt_PopulationsPluralAndObjectivesOrder (#137 Batch
// 2b T1.6, contract ai-generation.md §4 "Ordre d'injection des consignes
// dans le prompt (v6.1.0)"): two distinct regressions this locks in —
//  1. `populations` (plural) must be rendered as one joined "Publics cibles"
//     line, not the v6.0.0 singular "Public cible" label — a model given
//     the old label with several values joined together would have no
//     signal that ALL of them matter, not just the first.
//  2. The global objective (rank 5) must always precede the per-generation
//     "Précisions" (rank 6) — the plan's rationale: the model needs the
//     frame (objective) before the adjustment (instructions), and the two
//     lines must use distinct labels naming their own scope, or they read
//     as one instruction repeated twice.
func TestBuildGenerationPrompt_PopulationsPluralAndObjectivesOrder(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	base := generateQuestionsRequest{
		Theme:        "Révisions de fin d'année",
		Populations:  []string{"Ado (13-17 ans)", "Adulte (18-64 ans)"},
		Language:     "Français",
		Difficulties: []string{"Moyen", "Difficile"},
		Categories:   []string{"HISTORY"},
		Distribution: map[string]int{"SPEEDY": 100},
	}

	t.Run("populations rendered as one plural, joined line", func(t *testing.T) {
		prompt := server.buildGenerationPrompt(base, 5, nil)

		if !strings.Contains(prompt, "Publics cibles : Ado (13-17 ans), Adulte (18-64 ans)") {
			t.Errorf("Expected a single 'Publics cibles' line joining all populations, got:\n%s", prompt)
		}
		if strings.Contains(prompt, "Public cible :") {
			t.Errorf("Expected the v6.0.0 singular label 'Public cible :' to be gone, got:\n%s", prompt)
		}
	})

	t.Run("objective precedes per-generation instructions when both are set", func(t *testing.T) {
		req := base
		req.Objectives = "Réviser le chapitre 3 avant le contrôle"
		req.Instructions = "Insister sur les dates clés"
		prompt := server.buildGenerationPrompt(req, 5, nil)

		objIdx := strings.Index(prompt, "Objectif de la partie : Réviser le chapitre 3 avant le contrôle")
		instrIdx := strings.Index(prompt, "Précisions pour cette génération : Insister sur les dates clés")
		if objIdx == -1 {
			t.Fatalf("Expected an 'Objectif de la partie' line, got:\n%s", prompt)
		}
		if instrIdx == -1 {
			t.Fatalf("Expected a 'Précisions pour cette génération' line, got:\n%s", prompt)
		}
		if objIdx >= instrIdx {
			t.Errorf("Expected 'Objectif de la partie' (rank 5) to precede 'Précisions pour cette génération' (rank 6), got objective at %d, instructions at %d:\n%s", objIdx, instrIdx, prompt)
		}
		// v6.0.0 label must not resurface — it didn't name its own scope,
		// exactly the ambiguity §4 corrects.
		if strings.Contains(prompt, "Instructions additionnelles de l'animateur") {
			t.Errorf("Expected the v6.0.0 label 'Instructions additionnelles de l'animateur' to be gone, got:\n%s", prompt)
		}
	})

	t.Run("empty objectives and instructions emit no line at all", func(t *testing.T) {
		req := base
		req.Objectives = "   " // whitespace-only must trim to empty, not emit a blank line
		req.Instructions = ""
		prompt := server.buildGenerationPrompt(req, 5, nil)

		if strings.Contains(prompt, "Objectif de la partie") {
			t.Errorf("Expected no 'Objectif de la partie' line when objectives is empty/whitespace, got:\n%s", prompt)
		}
		if strings.Contains(prompt, "Précisions pour cette génération") {
			t.Errorf("Expected no 'Précisions pour cette génération' line when instructions is empty, got:\n%s", prompt)
		}
	})
}
