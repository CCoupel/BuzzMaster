package server

// Tests dérivés de contracts/ai-multi-provider.md (#137, v6.1.0) et de
// _work/reports/planner-20260805-204318-plan-137.md, phases 1 et 3.
//
// Ce fichier est un ajout NEUF, complémentaire à internal/server/ai_generation_test.go
// (#8) et internal/server/ai_generator_m2_test.go (dev-backend) — même paquet, réutilise
// leurs helpers (setupTestHTTPServer, setAIConfig, validAIConfig, llmQuestion,
// baseGenerateRequest, postGenerateQuestions, decodeJSONBody, readQuestionFile,
// countQuestionDirs, withAnthropicBaseURL, writeSSEQuestions, newUploadQuestionRequest).
// AUCUNE de ces définitions n'est dupliquée ici.
//
// ⚠️ config.SetInstance mute un global : PAS de t.Parallel() dans ce fichier.
//
// --------------------------------------------------------------------------------
// Hypothèses d'intégration à faire valider par dev-backend/code-reviewer
// --------------------------------------------------------------------------------
// Au moment où ces tests sont écrits, AUCUN fichier #137 n'existe encore côté backend
// (ni ai_provider.go, ni ai_groq.go, ni ai_job.go — vérifié) : Phase 0 (calibration
// empirique T0.1) n'a pas démarré. Ces tests posent donc, en plus du seam
// ANTHROPIC_BASE_URL déjà documenté dans ai_generation_test.go :
//
//  1. GROQ_BASE_URL (env var) — même motif, pour rediriger le client Groq stdlib vers
//     un httptest.Server local. Le contrat §6 impose net/http stdlib (motif
//     github_client.go), donc aucun SDK ne fournit ce comportement "gratuitement" comme
//     pour Anthropic — dev-backend doit l'ajouter explicitement (une ligne : lire
//     GROQ_BASE_URL avec repli sur "https://api.groq.com").
//  2. La réponse Groq réussie est mockée au format standard "OpenAI chat completions"
//     (`{"choices":[{"message":{"content": "<json questions>"}}]}`), cohérent avec
//     "endpoint compatible OpenAI" (contrat §6) — pas de streaming (contrat confirmé).
//  3. Sémantique retry-after / échecs consécutifs (contrat §3) : ces tests supposent
//     qu'un 429/413 avec retry-after déclenche une NOUVELLE tentative du MÊME lot
//     (pas un abandon vers le lot suivant) — sinon le volume demandé ne serait jamais
//     atteint sur une erreur transitoire. Un 429 persistant est supposé compter comme
//     échec (consécutif) au sens de max_consecutive_failures. Si l'implémentation
//     retenue diffère, seuls TestAIBatching_Groq429_RespectsRetryAfter et
//     TestAIBatching_Groq429Persistent_ReturnsProviderQuotaCode sont concernés — le
//     reste du fichier n'en dépend pas.
//  4. Un lot dont TOUTES les questions sont rejetées par la validation serveur (§5.1
//     de ai-generation.md — ex. ANSWER manquant) est traité comme un lot à 0 question
//     créée, PAS comme un échec transport comptant vers max_consecutive_failures — ce
//     compteur ne sanctionne que les échecs provider (réseau/HTTP), pas le contenu.
//     Voir TestAIBatching_SequentialIDAllocation_NoGapMisuse.

import (
	"buzzcontrol/internal/config"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// groqBaseURLEnvVar is now defined in production code (ai_groq.go) — the
// client reads GROQ_BASE_URL for real, same motif as ANTHROPIC_BASE_URL in
// ai_generation_test.go. Removed the local placeholder const here to avoid
// a redeclaration; withGroqBaseURL below resolves to the production one.

// ================================================================================
// Helpers
// ================================================================================

// batchingAIConfig extends validAIConfig() (ai_generation_test.go) with the #137
// fields (contract §8). provider ∈ {"anthropic","groq"}.
func batchingAIConfig(provider string, batchSize, interBatchDelayMs, maxConsecutiveFailures int) config.AIConfig {
	cfg := validAIConfig()
	cfg.Provider = provider
	cfg.GroqAPIKey = "gsk_test_key_0001"
	cfg.GroqModel = "openai/gpt-oss-120b"
	cfg.BatchSize = batchSize
	cfg.InterBatchDelayMs = interBatchDelayMs
	cfg.ContextTokenBudget = 1500
	cfg.MaxConsecutiveFailures = maxConsecutiveFailures
	return cfg
}

func withGroqBaseURL(t *testing.T, url string) {
	t.Helper()
	t.Setenv(groqBaseURLEnvVar, url)
}

// twoUniqueSpeedyQuestions builds 2 SPEEDY questions whose text embeds batchIdx, so
// tests can verify exactly which batches contributed which questions on disk.
func twoUniqueSpeedyQuestions(batchIdx int) []map[string]interface{} {
	return []map[string]interface{}{
		llmQuestion(map[string]interface{}{"TYPE": "SPEEDY", "QUESTION": fmt.Sprintf("Batch %d Q1", batchIdx), "ANSWER": "ok"}),
		llmQuestion(map[string]interface{}{"TYPE": "SPEEDY", "QUESTION": fmt.Sprintf("Batch %d Q2", batchIdx), "ANSWER": "ok"}),
	}
}

// batchOutcome describes what the mock provider does on a given sequential call.
type batchOutcome struct {
	questions  []map[string]interface{} // success: these questions are returned
	errStatus  int                      // >0: respond with this HTTP status instead
	errType    string                   // error envelope "type" field
	errMessage string                   // error envelope "message" field — "" defaults to "mock upstream error" (#142-adjacent verbosity fix)
	retryAfter string                   // Retry-After header value on an error response (empty = omit)
}

// sequencedAnthropicServer serves outcomes[0], outcomes[1], ... on successive calls
// (the last outcome repeats if there are more calls than outcomes), via the same SSE
// wire format as ai_generation_test.go's mockAnthropicSSEServer. Returns the server
// and an atomic call counter tests can inspect after the job finishes.
func sequencedAnthropicServer(t *testing.T, outcomes []batchOutcome) (*httptest.Server, *int32) {
	t.Helper()
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := int(atomic.AddInt32(&callCount, 1)) - 1
		outcome := pickOutcome(outcomes, idx)
		if outcome.errStatus != 0 {
			writeAnthropicError(w, outcome)
			return
		}
		writeSSEQuestions(t, w, outcome.questions)
	}))
	t.Cleanup(server.Close)
	return server, &callCount
}

func pickOutcome(outcomes []batchOutcome, idx int) batchOutcome {
	if idx < len(outcomes) {
		return outcomes[idx]
	}
	return outcomes[len(outcomes)-1]
}

func writeAnthropicError(w http.ResponseWriter, outcome batchOutcome) {
	if outcome.retryAfter != "" {
		w.Header().Set("Retry-After", outcome.retryAfter)
	}
	message := outcome.errMessage
	if message == "" {
		message = "mock upstream error"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(outcome.errStatus)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"type": "error",
		"error": map[string]string{
			"type":    outcome.errType,
			"message": message,
		},
	})
}

// groqCapturedRequest snapshots one inbound request to the Groq mock, for asserting
// the payload shape against contract §6.
type groqCapturedRequest struct {
	authHeader string
	body       map[string]interface{}
}

// sequencedGroqServer mirrors sequencedAnthropicServer for the Groq (OpenAI-compatible,
// non-streaming) wire format, and additionally records every request it receives.
func sequencedGroqServer(t *testing.T, outcomes []batchOutcome) (*httptest.Server, *int32, *[]groqCapturedRequest) {
	t.Helper()
	var callCount int32
	var mu sync.Mutex
	var captured []groqCapturedRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		captured = append(captured, groqCapturedRequest{authHeader: r.Header.Get("Authorization"), body: body})
		mu.Unlock()

		idx := int(atomic.AddInt32(&callCount, 1)) - 1
		outcome := pickOutcome(outcomes, idx)
		if outcome.errStatus != 0 {
			writeAnthropicError(w, outcome) // same envelope shape works for both mocks here
			return
		}
		writeGroqSuccess(w, outcome.questions)
	}))
	t.Cleanup(server.Close)
	return server, &callCount, &captured
}

// writeGroqSuccess mimics the standard OpenAI-compatible chat-completions response
// shape (contract §6: "endpoint compatible OpenAI", no streaming) with the LLM
// payload embedded as the message's text content.
func writeGroqSuccess(w http.ResponseWriter, questions []map[string]interface{}) {
	payload, _ := json.Marshal(map[string]interface{}{"questions": questions})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":     "chatcmpl-mock",
		"object": "chat.completion",
		"model":  "openai/gpt-oss-120b",
		"choices": []map[string]interface{}{
			{"index": 0, "message": map[string]interface{}{"role": "assistant", "content": string(payload)}, "finish_reason": "stop"},
		},
		"usage": map[string]interface{}{"prompt_tokens": 500, "completion_tokens": 300, "total_tokens": 800},
	})
}

// dialAdminWS connects a real WebSocket client to /ws/admin on the given server's
// mux — same pattern as websocket_test.go's startTestWSServer/dialWSPath, adapted to
// exercise the FULL HTTPServer (not a bare WebSocketHub) so that
// handleGenerateQuestions and the admin WS share the exact same wsHub instance.
func dialAdminWS(t *testing.T, server *HTTPServer) *websocket.Conn {
	t.Helper()
	wsSrv := httptest.NewServer(server.mux)
	t.Cleanup(wsSrv.Close)
	wsURL := "ws" + strings.TrimPrefix(wsSrv.URL, "http") + "/ws/admin"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect admin WebSocket: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// dialWS connects to an arbitrary endpoint on the server's mux (e.g. /ws/tv) —
// generalizes dialAdminWS for the "never broadcast to non-admin clients" check.
func dialWS(t *testing.T, server *HTTPServer, path string) *websocket.Conn {
	t.Helper()
	wsSrv := httptest.NewServer(server.mux)
	t.Cleanup(wsSrv.Close)
	wsURL := "ws" + strings.TrimPrefix(wsSrv.URL, "http") + path
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect WebSocket at %s: %v", path, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// readNextAIProgress reads WS frames (skipping any unrelated action, e.g. an initial
// state sync) until it finds an AI_GENERATION_PROGRESS message or the timeout elapses.
func readNextAIProgress(t *testing.T, conn *websocket.Conn, timeout time.Duration) map[string]interface{} {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatal("Timed out waiting for an AI_GENERATION_PROGRESS message")
		}
		conn.SetReadDeadline(time.Now().Add(remaining))
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("Failed reading WS message while waiting for AI_GENERATION_PROGRESS: %v", err)
		}
		var envelope struct {
			Action string          `json:"ACTION"`
			Msg    json.RawMessage `json:"MSG"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			continue
		}
		if envelope.Action == "AI_GENERATION_PROGRESS" {
			var msg map[string]interface{}
			json.Unmarshal(envelope.Msg, &msg)
			return msg
		}
	}
}

// waitForJobTerminalState reads progress messages until STATE is DONE/FAILED/CANCELLED.
func waitForJobTerminalState(t *testing.T, conn *websocket.Conn, timeout time.Duration) map[string]interface{} {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatal("Timed out waiting for the job to reach a terminal state")
		}
		msg := readNextAIProgress(t, conn, remaining)
		switch msg["STATE"] {
		case "DONE", "FAILED", "CANCELLED":
			return msg
		}
	}
}

func expectAccepted(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	if w.Code != http.StatusAccepted {
		t.Fatalf("Expected 202 Accepted, got %d: %s", w.Code, w.Body.String())
	}
	return decodeJSONBody(t, w)
}

// ================================================================================
// Découpage en lots (contract §2)
// ================================================================================

func TestAIBatching_SplitsVolumeIntoSequentialBatches(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("anthropic", 2, 10, 2))

	outcomes := []batchOutcome{
		{questions: twoUniqueSpeedyQuestions(0)},
		{questions: twoUniqueSpeedyQuestions(1)},
		{questions: twoUniqueSpeedyQuestions(2)},
	}
	upstream, callCount := sequencedAnthropicServer(t, outcomes)
	withAnthropicBaseURL(t, upstream.URL)

	conn := dialAdminWS(t, server)

	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 6}
	body := expectAccepted(t, postGenerateQuestions(server, req))
	if body["batches_total"] != float64(3) {
		t.Errorf("Expected batches_total=3 for 6 questions / batch_size 2, got %v", body["batches_total"])
	}
	if jobID, _ := body["job_id"].(string); jobID == "" {
		t.Error("Expected a non-empty job_id")
	}

	final := waitForJobTerminalState(t, conn, 5*time.Second)
	if final["STATE"] != "DONE" {
		t.Fatalf("Expected STATE=DONE, got %v (msg=%v)", final["STATE"], final)
	}
	if final["BATCHES_DONE"] != float64(3) || final["BATCHES_TOTAL"] != float64(3) {
		t.Errorf("Expected 3/3 batches done, got %v/%v", final["BATCHES_DONE"], final["BATCHES_TOTAL"])
	}
	if final["CREATED_COUNT"] != float64(6) {
		t.Errorf("Expected CREATED_COUNT=6, got %v", final["CREATED_COUNT"])
	}
	if got := atomic.LoadInt32(callCount); got != 3 {
		t.Errorf("Expected exactly 3 sequential provider calls (never in parallel), got %d", got)
	}
}

func TestAIBatching_SingleFailedBatch_PreviousBatchesSurvive(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("anthropic", 2, 10, 2)) // threshold 2 — a single isolated failure must not trip it

	outcomes := []batchOutcome{
		{questions: twoUniqueSpeedyQuestions(0)},
		{questions: twoUniqueSpeedyQuestions(1)},
		{questions: twoUniqueSpeedyQuestions(2)},
		{errStatus: http.StatusBadGateway, errType: "upstream_error"}, // batch 4 (index 3) fails alone
		{questions: twoUniqueSpeedyQuestions(4)},
	}
	upstream, callCount := sequencedAnthropicServer(t, outcomes)
	withAnthropicBaseURL(t, upstream.URL)

	conn := dialAdminWS(t, server)

	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 10}
	expectAccepted(t, postGenerateQuestions(server, req))

	final := waitForJobTerminalState(t, conn, 5*time.Second)
	if final["STATE"] != "DONE" {
		t.Fatalf("A single non-consecutive batch failure must not fail the job — expected DONE, got %v (msg=%v)", final["STATE"], final)
	}
	if final["CREATED_COUNT"] != float64(8) {
		t.Errorf("Expected 8 created (4 successful batches x 2, batch 4 contributed 0), got %v", final["CREATED_COUNT"])
	}
	if got := atomic.LoadInt32(callCount); got != 5 {
		t.Errorf("Expected all 5 batches to be attempted (reprise past the isolated failure), got %d calls", got)
	}
	if countQuestionDirs(t, dataDir) != 8 {
		t.Errorf("Expected 8 question files on disk, got %d", countQuestionDirs(t, dataDir))
	}
}

func TestAIBatching_StopsAfterMaxConsecutiveFailures(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("anthropic", 2, 10, 2)) // threshold 2

	outcomes := []batchOutcome{
		{questions: twoUniqueSpeedyQuestions(0)},                       // batch 1: ok
		{errStatus: http.StatusBadGateway, errType: "upstream_error"},  // batch 2: fail (1st consecutive)
		{errStatus: http.StatusBadGateway, errType: "upstream_error"},  // batch 3: fail (2nd consecutive) -> stop
		{questions: twoUniqueSpeedyQuestions(3)},                       // batch 4: must never be called
		{questions: twoUniqueSpeedyQuestions(4)},                       // batch 5: must never be called
	}
	upstream, callCount := sequencedAnthropicServer(t, outcomes)
	withAnthropicBaseURL(t, upstream.URL)

	conn := dialAdminWS(t, server)

	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 10}
	expectAccepted(t, postGenerateQuestions(server, req))

	final := waitForJobTerminalState(t, conn, 5*time.Second)
	if final["STATE"] != "FAILED" {
		t.Fatalf("Expected STATE=FAILED after 2 consecutive batch failures, got %v (msg=%v)", final["STATE"], final)
	}
	if final["CREATED_COUNT"] != float64(2) {
		t.Errorf("Expected CREATED_COUNT=2 (batch 1 only, acquis conservé), got %v", final["CREATED_COUNT"])
	}
	if got := atomic.LoadInt32(callCount); got != 3 {
		t.Errorf("Expected exactly 3 calls (stop immediately after the 2nd consecutive failure), got %d — batches 4/5 must never be attempted", got)
	}
	if countQuestionDirs(t, dataDir) != 2 {
		t.Errorf("Expected the 2 questions from batch 1 to remain on disk, got %d files", countQuestionDirs(t, dataDir))
	}
}

func TestAIBatching_BroadcastsAfterEachSuccessfulBatch(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("anthropic", 2, 10, 2))

	outcomes := []batchOutcome{
		{questions: twoUniqueSpeedyQuestions(0)},
		{questions: twoUniqueSpeedyQuestions(1)},
		{questions: twoUniqueSpeedyQuestions(2)},
	}
	upstream, _ := sequencedAnthropicServer(t, outcomes)
	withAnthropicBaseURL(t, upstream.URL)

	var broadcastCount int32
	server.OnQuestionUpload = func() { atomic.AddInt32(&broadcastCount, 1) }

	conn := dialAdminWS(t, server)
	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 6}
	expectAccepted(t, postGenerateQuestions(server, req))
	waitForJobTerminalState(t, conn, 5*time.Second)

	if got := atomic.LoadInt32(&broadcastCount); got != 3 {
		t.Errorf("Expected OnQuestionUpload() called once per successful batch (3), got %d — contract §2 'Broadcast après chaque lot, pas seulement à la fin'", got)
	}
}

// TestAIBatching_SequentialIDAllocation_NoGapMisuse is the regression test for T1.4:
// a batch that produces 0 valid questions (content-validation rejection, NOT a
// provider/transport failure) must not "reserve" or skip any ID — the next
// successful batch's IDs pick up immediately after the previous one, with no gap.
func TestAIBatching_SequentialIDAllocation_NoGapMisuse(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("anthropic", 2, 10, 2))

	outcomes := []batchOutcome{
		{questions: twoUniqueSpeedyQuestions(0)}, // batch 1: 2 valid -> e.g. IDs 1,2
		{questions: []map[string]interface{}{ // batch 2: both invalid (no ANSWER) -> 0 created, not a "failure"
			llmQuestion(map[string]interface{}{"TYPE": "SPEEDY", "QUESTION": "Invalide 1", "ANSWER": ""}),
			llmQuestion(map[string]interface{}{"TYPE": "SPEEDY", "QUESTION": "Invalide 2", "ANSWER": ""}),
		}},
		{questions: twoUniqueSpeedyQuestions(2)}, // batch 3: 2 valid -> must land on the very next free IDs
	}
	upstream, callCount := sequencedAnthropicServer(t, outcomes)
	withAnthropicBaseURL(t, upstream.URL)

	conn := dialAdminWS(t, server)
	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 6}
	expectAccepted(t, postGenerateQuestions(server, req))

	final := waitForJobTerminalState(t, conn, 5*time.Second)
	if final["STATE"] != "DONE" {
		t.Fatalf("A batch with 0 valid questions (content rejection) must not fail the job — expected DONE, got %v (msg=%v)", final["STATE"], final)
	}
	if final["CREATED_COUNT"] != float64(4) {
		t.Errorf("Expected CREATED_COUNT=4 (batches 1 and 3 only), got %v", final["CREATED_COUNT"])
	}
	if final["SKIPPED_COUNT"] != float64(2) {
		t.Errorf("Expected SKIPPED_COUNT=2 (the 2 invalid questions from batch 2), got %v", final["SKIPPED_COUNT"])
	}
	if got := atomic.LoadInt32(callCount); got != 3 {
		t.Errorf("Expected all 3 batches attempted, got %d calls", got)
	}
	if countQuestionDirs(t, dataDir) != 4 {
		t.Fatalf("Expected exactly 4 question files, got %d", countQuestionDirs(t, dataDir))
	}

	// Collect every created ID and verify they form a contiguous run — no ID
	// silently reserved-then-abandoned for the empty batch 2.
	ids := collectQuestionIDs(t, dataDir)
	if len(ids) != 4 {
		t.Fatalf("Expected 4 IDs, got %v", ids)
	}
	sortedInts := make([]int, len(ids))
	for i, id := range ids {
		n, err := strconv.Atoi(id)
		if err != nil {
			t.Fatalf("Non-numeric question ID %q", id)
		}
		sortedInts[i] = n
	}
	sort.Ints(sortedInts)
	for i := 1; i < len(sortedInts); i++ {
		if sortedInts[i] != sortedInts[i-1]+1 {
			t.Errorf("Expected contiguous IDs, got a gap between %d and %d (ids=%v)", sortedInts[i-1], sortedInts[i], sortedInts)
		}
	}
}

func collectQuestionIDs(t *testing.T, dataDir string) []string {
	t.Helper()
	dir := filepath.Join(dataDir, "files", "questions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("Failed reading questions dir: %v", err)
	}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	return ids
}

// TestAIBatching_ConcurrentManualUpload_NoIDCollision_Race is a NEW race scenario
// introduced by #137: unlike #8 (synchronous, one HTTP request, done), the job now
// runs across a real wall-clock span (several batches with an inter-batch delay),
// during which a manual /questions upload can genuinely interleave with the job's
// ID allocation. Run with `go test -race`.
func TestAIBatching_ConcurrentManualUpload_NoIDCollision_Race(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("anthropic", 2, 20, 2)) // small delay so the job spans real time without slowing the test too much

	outcomes := []batchOutcome{
		{questions: twoUniqueSpeedyQuestions(0)},
		{questions: twoUniqueSpeedyQuestions(1)},
		{questions: twoUniqueSpeedyQuestions(2)},
	}
	upstream, _ := sequencedAnthropicServer(t, outcomes)
	withAnthropicBaseURL(t, upstream.URL)

	conn := dialAdminWS(t, server)
	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 6}
	expectAccepted(t, postGenerateQuestions(server, req))

	const manualUploads = 5
	var wg sync.WaitGroup
	for i := 0; i < manualUploads; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := newUploadQuestionRequest(fmt.Sprintf("Manual upload during job #%d", i), "reponse")
			w := httptest.NewRecorder()
			server.mux.ServeHTTP(w, r)
		}(i)
	}
	wg.Wait()

	final := waitForJobTerminalState(t, conn, 10*time.Second)
	if final["STATE"] != "DONE" {
		t.Fatalf("Expected the job to complete DONE despite concurrent manual uploads, got %v", final["STATE"])
	}

	texts := map[string]bool{}
	for _, id := range collectQuestionIDs(t, dataDir) {
		q := readQuestionFile(t, dataDir, id)
		if text, ok := q["QUESTION"].(string); ok {
			texts[text] = true
		}
	}
	wantTotal := 6 + manualUploads
	if len(texts) != wantTotal {
		t.Errorf("Expected %d distinct questions (6 from the job + %d manual), got %d — ID collision between the background job and a concurrent manual upload", wantTotal, manualUploads, len(texts))
	}
}

// ================================================================================
// Non-régression #8 : le chemin Anthropic garde le même mapping question.json
// ================================================================================

func TestAIBatching_AnthropicProvider_QuestionJSONUnchanged(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("anthropic", 5, 10, 2)) // batch_size >= volume: still a single batch

	upstream := mockAnthropicSSEServer(t, []map[string]interface{}{
		llmQuestion(map[string]interface{}{"TYPE": "SPEEDY", "QUESTION": "Non-regression #8", "ANSWER": "reponse", "DIFFICULTY": "Moyen"}),
	})
	withAnthropicBaseURL(t, upstream.URL)

	conn := dialAdminWS(t, server)
	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 1}
	body := expectAccepted(t, postGenerateQuestions(server, req))
	if body["batches_total"] != float64(1) {
		t.Errorf("Expected a single batch for volume=1, got %v", body["batches_total"])
	}

	final := waitForJobTerminalState(t, conn, 5*time.Second)
	if final["STATE"] != "DONE" || final["CREATED_COUNT"] != float64(1) {
		t.Fatalf("Expected DONE with 1 created, got %v", final)
	}

	ids := collectQuestionIDs(t, dataDir)
	if len(ids) != 1 {
		t.Fatalf("Expected exactly 1 question file, got %d", len(ids))
	}
	q := readQuestionFile(t, dataDir, ids[0])
	if q["POINTS"] != "20" { // Moyen -> "20", contract ai-generation.md §5.2, unchanged by #137
		t.Errorf("Expected POINTS=\"20\" (string) as in #8, got %v (%T)", q["POINTS"], q["POINTS"])
	}
	if q["POINTS_TARGET"] != "PLAYER" {
		t.Errorf("Expected POINTS_TARGET=PLAYER as in #8, got %v", q["POINTS_TARGET"])
	}
	if _, isString := q["TIME"].(string); !isString {
		t.Errorf("Expected TIME as a string as in #8, got %T", q["TIME"])
	}
	if _, hasMedia := q["MEDIA"]; hasMedia {
		t.Error("Expected no MEDIA field, as in #8")
	}
}

// ================================================================================
// Client Groq (contract §6)
// ================================================================================

func TestAIBatching_GroqRequestPayload_MatchesContract(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("groq", 5, 10, 2))

	upstream, _, captured := sequencedGroqServer(t, []batchOutcome{
		{questions: []map[string]interface{}{llmQuestion(map[string]interface{}{"TYPE": "SPEEDY", "ANSWER": "ok"})}},
	})
	withGroqBaseURL(t, upstream.URL)

	conn := dialAdminWS(t, server)
	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 1}
	expectAccepted(t, postGenerateQuestions(server, req))
	final := waitForJobTerminalState(t, conn, 5*time.Second)
	if final["STATE"] != "DONE" {
		t.Fatalf("Expected DONE, got %v (msg=%v)", final["STATE"], final)
	}
	if final["PROVIDER"] != "groq" {
		t.Errorf("Expected PROVIDER=groq in the progress message, got %v", final["PROVIDER"])
	}

	if len(*captured) == 0 {
		t.Fatal("Expected at least one request captured by the Groq mock")
	}
	reqCaptured := (*captured)[0]

	if reqCaptured.authHeader != "Bearer gsk_test_key_0001" {
		t.Errorf("Expected Authorization: Bearer <groq_api_key>, got %q", reqCaptured.authHeader)
	}
	if model, _ := reqCaptured.body["model"].(string); model != "openai/gpt-oss-120b" {
		t.Errorf("Expected model=openai/gpt-oss-120b, got %v", reqCaptured.body["model"])
	}
	rf, ok := reqCaptured.body["response_format"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected a response_format object in the request body, got %v", reqCaptured.body["response_format"])
	}
	if rf["type"] != "json_schema" {
		t.Errorf("Expected response_format.type=json_schema, got %v", rf["type"])
	}
	js, ok := rf["json_schema"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected response_format.json_schema object, got %v", rf["json_schema"])
	}
	if js["name"] != "buzz_questions" {
		t.Errorf("Expected json_schema.name=buzz_questions, got %v", js["name"])
	}
	if js["strict"] != true {
		t.Errorf("Expected json_schema.strict=true, got %v", js["strict"])
	}
	if js["schema"] == nil {
		t.Error("Expected a non-nil json_schema.schema")
	}
}

// TestAIBatching_Groq429_RespectsRetryAfter uses a single-batch job (volume ==
// batch_size) so the ONLY way it can end up with the requested question is by
// retrying that same batch after honoring Retry-After — see the file header's
// assumption #3.
func TestAIBatching_Groq429_RespectsRetryAfter(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("groq", 1, 50, 3)) // inter_batch_delay_ms small on purpose: Retry-After (2s) must dominate

	upstream, callCount, _ := sequencedGroqServer(t, []batchOutcome{
		{errStatus: http.StatusTooManyRequests, errType: "rate_limit_error", retryAfter: "2"},
		{questions: []map[string]interface{}{llmQuestion(map[string]interface{}{"TYPE": "SPEEDY", "ANSWER": "ok"})}},
	})
	withGroqBaseURL(t, upstream.URL)

	conn := dialAdminWS(t, server)
	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 1}
	start := time.Now()
	expectAccepted(t, postGenerateQuestions(server, req))

	final := waitForJobTerminalState(t, conn, 8*time.Second)
	elapsed := time.Since(start)

	if final["STATE"] != "DONE" {
		t.Fatalf("Expected the 429 to be retried and the job to reach DONE, got %v (msg=%v)", final["STATE"], final)
	}
	if final["CREATED_COUNT"] != float64(1) {
		t.Errorf("Expected CREATED_COUNT=1 after the retry succeeded, got %v", final["CREATED_COUNT"])
	}
	if got := atomic.LoadInt32(callCount); got != 2 {
		t.Errorf("Expected exactly 2 calls (initial 429 + 1 retry), got %d", got)
	}
	if elapsed < 1800*time.Millisecond {
		t.Errorf("Expected the retry to wait at least ~2s (Retry-After), only %v elapsed — inter_batch_delay_ms (50ms) must not be used instead of Retry-After", elapsed)
	}
}

// TestAIBatching_Groq429Persistent_ReturnsProviderQuotaCode checks the terminal
// outcome (contract §3: "sur 429 persistant, remonter code: provider_quota") without
// pinning down the exact number of internal retries before giving up (unspecified by
// the contract — see file header assumption #3).
func TestAIBatching_Groq429Persistent_ReturnsProviderQuotaCode(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("groq", 1, 10, 2))

	outcomes := make([]batchOutcome, 8)
	for i := range outcomes {
		outcomes[i] = batchOutcome{errStatus: http.StatusTooManyRequests, errType: "rate_limit_error"} // no Retry-After: exercises backoff, not the header path
	}
	upstream, _, _ := sequencedGroqServer(t, outcomes)
	withGroqBaseURL(t, upstream.URL)

	conn := dialAdminWS(t, server)
	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 1}
	expectAccepted(t, postGenerateQuestions(server, req))

	final := waitForJobTerminalState(t, conn, 15*time.Second)
	if final["STATE"] != "FAILED" {
		t.Fatalf("Expected STATE=FAILED on persistent 429, got %v (msg=%v)", final["STATE"], final)
	}
	if final["ERROR_CODE"] != "provider_quota" {
		t.Errorf("Expected ERROR_CODE=provider_quota on persistent 429 (contract §3), got %v", final["ERROR_CODE"])
	}
}

func TestAIBatching_Groq401_MapsToUpstreamError(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("groq", 1, 10, 1)) // threshold 1: a single 401 fails the job immediately

	upstream, _, _ := sequencedGroqServer(t, []batchOutcome{
		{errStatus: http.StatusUnauthorized, errType: "authentication_error"},
	})
	withGroqBaseURL(t, upstream.URL)

	conn := dialAdminWS(t, server)
	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 1}
	expectAccepted(t, postGenerateQuestions(server, req))

	final := waitForJobTerminalState(t, conn, 5*time.Second)
	if final["STATE"] != "FAILED" {
		t.Fatalf("Expected STATE=FAILED on 401, got %v", final["STATE"])
	}
	if final["ERROR_CODE"] != "upstream_error" {
		t.Errorf("Expected ERROR_CODE=upstream_error on 401 (same mapping as #8), got %v", final["ERROR_CODE"])
	}
}

// ================================================================================
// ERROR_MESSAGE verbosity (#142-adjacent fix) — contract ai-generation.md §8 S2,
// amended: the admin now sees the real provider detail (sanitized), not a fixed
// generic string. Reproduces the exact scenario that made #142 (Groq schema
// rejection) undiagnosable without a temporary debug trace.
// ================================================================================

func TestAIBatching_GroqSchemaRejection_SurfacesProviderMessage(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("groq", 1, 10, 1)) // threshold 1: a single 400 fails the job immediately

	// The exact envelope from #142's report (issue comment, QUALIF repro).
	const schemaRejectionMessage = "invalid JSON schema for response_format: 'buzz_questions': " +
		"/properties/questions/items/anyOf: anyOf disambiguation failed: anyOf: discriminator: " +
		"multiple candidate properties CATEGORY, DIFFICULTY, TYPE [discriminator_multiple_candidates]"
	upstream, _, _ := sequencedGroqServer(t, []batchOutcome{
		{errStatus: http.StatusBadRequest, errType: "invalid_request_error", errMessage: schemaRejectionMessage},
	})
	withGroqBaseURL(t, upstream.URL)

	conn := dialAdminWS(t, server)
	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 1}
	expectAccepted(t, postGenerateQuestions(server, req))

	final := waitForJobTerminalState(t, conn, 5*time.Second)
	if final["STATE"] != "FAILED" {
		t.Fatalf("Expected STATE=FAILED, got %v", final["STATE"])
	}
	if final["ERROR_CODE"] != "upstream_error" {
		t.Errorf("Expected ERROR_CODE=upstream_error, got %v", final["ERROR_CODE"])
	}
	errMsg, _ := final["ERROR_MESSAGE"].(string)
	if !strings.Contains(errMsg, "discriminator: multiple candidate properties") {
		t.Errorf("Expected ERROR_MESSAGE to surface the real Groq schema-rejection detail (the exact "+
			"text #142 needed a temporary debug trace to see), got %q", errMsg)
	}
}

func TestAIBatching_AnthropicUpstreamError_SurfacesProviderMessage(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("anthropic", 1, 10, 1))

	const detail = "messages.0: all messages must have non-empty content except for the optional final assistant message"
	upstream, _ := sequencedAnthropicServer(t, []batchOutcome{
		{errStatus: http.StatusBadRequest, errType: "invalid_request_error", errMessage: detail},
	})
	withAnthropicBaseURL(t, upstream.URL)

	conn := dialAdminWS(t, server)
	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 1}
	expectAccepted(t, postGenerateQuestions(server, req))

	final := waitForJobTerminalState(t, conn, 5*time.Second)
	if final["STATE"] != "FAILED" {
		t.Fatalf("Expected STATE=FAILED, got %v", final["STATE"])
	}
	errMsg, _ := final["ERROR_MESSAGE"].(string)
	if !strings.Contains(errMsg, "non-empty content") {
		t.Errorf("Expected ERROR_MESSAGE to surface the real Anthropic error detail, got %q", errMsg)
	}
}

// TestAIBatching_GroqUpstreamError_MessageRedactsAPIKeyShape is a defense-in-depth
// check (contract §8 S2): even if a provider's error body happened to echo back
// something matching a known API key shape, sanitizeUpstreamMessage must redact it
// before it ever reaches the admin — a provider shouldn't do this in practice (the
// key is sent via the Authorization header, never in the request body a response
// would echo), but the filter must not depend on that assumption holding.
func TestAIBatching_GroqUpstreamError_MessageRedactsAPIKeyShape(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	setAIConfig(batchingAIConfig("groq", 1, 10, 1))

	const messageWithKeyShape = "rejected request using key gsk_abcdEFGH12345678ijklMNOPqrstUVWX"
	upstream, _, _ := sequencedGroqServer(t, []batchOutcome{
		{errStatus: http.StatusBadRequest, errType: "invalid_request_error", errMessage: messageWithKeyShape},
	})
	withGroqBaseURL(t, upstream.URL)

	conn := dialAdminWS(t, server)
	req := baseGenerateRequest()
	req["volume"] = map[string]interface{}{"mode": "count", "value": 1}
	expectAccepted(t, postGenerateQuestions(server, req))

	final := waitForJobTerminalState(t, conn, 5*time.Second)
	errMsg, _ := final["ERROR_MESSAGE"].(string)
	if strings.Contains(errMsg, "gsk_abcdEFGH12345678ijklMNOPqrstUVWX") {
		t.Errorf("ERROR_MESSAGE must never contain an API-key-shaped substring verbatim, got %q", errMsg)
	}
	if !strings.Contains(errMsg, "[redacted]") {
		t.Errorf("Expected the key-shaped substring to be replaced with [redacted], got %q", errMsg)
	}
}
