package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/protocol"
)

// ============================================================================
// Job model (contract ai-multi-provider.md §12) — in-memory only. A server
// restart loses the job; the questions already written to disk survive
// (documented MVP limitation, contract §12).
// ============================================================================

// aiJob tracks one AI generation run. Mutable fields are guarded by mu so the
// running goroutine (writer) and WS/HTTP handlers (readers, e.g. a client
// reconnecting mid-job) never race (security finding M1 extends to this
// struct's own fields, not just the registry's single-job invariant).
type aiJob struct {
	mu sync.Mutex

	ID           string
	Provider     string
	State        string // "RUNNING" | "DONE" | "FAILED" | "CANCELLED"
	BatchesDone  int
	BatchesTotal int
	CreatedCount int
	SkippedCount int
	ErrorCode    string
	// ErrorMessage is the sanitized admin-facing detail behind ErrorCode
	// (protocol.AIGenerationProgressPayload.ErrorMessage, #142-adjacent
	// verbosity fix) — "" unless State is FAILED.
	ErrorMessage string

	// cancelRequested is observed BETWEEN batches only (contract §11:
	// "l'annulation prend effet entre deux lots — jamais au milieu d'un
	// appel provider"). It is intentionally NOT the context passed to the
	// provider call itself — see runAIJob.
	cancelRequested chan struct{}
	cancelOnce      sync.Once
}

func (j *aiJob) requestCancel() {
	j.cancelOnce.Do(func() { close(j.cancelRequested) })
}

func (j *aiJob) isCancelRequested() bool {
	select {
	case <-j.cancelRequested:
		return true
	default:
		return false
	}
}

// snapshot returns a copy of the job's current state as the WS payload
// shape, safe to send without holding the lock any longer than this call.
func (j *aiJob) snapshot() protocol.AIGenerationProgressPayload {
	j.mu.Lock()
	defer j.mu.Unlock()
	return protocol.AIGenerationProgressPayload{
		JobID:        j.ID,
		State:        j.State,
		BatchesDone:  j.BatchesDone,
		BatchesTotal: j.BatchesTotal,
		CreatedCount: j.CreatedCount,
		SkippedCount: j.SkippedCount,
		ErrorCode:    j.ErrorCode,
		ErrorMessage: j.ErrorMessage,
		Provider:     j.Provider,
	}
}

// aiJobRegistry enforces "un seul job à la fois" (contract §12). tryStart is
// the ONLY way to install a new job, and it does the "is one running?" check
// and the "install this one" write under the SAME lock acquisition — no gap
// between them for a second concurrent POST to slip through (security
// finding M1: the naive "check, then separately create" pattern has a TOCTOU
// window; this doesn't).
type aiJobRegistry struct {
	mu  sync.Mutex
	job *aiJob
}

var globalAIJobRegistry = &aiJobRegistry{}

func (r *aiJobRegistry) tryStart(provider string, batchesTotal int) (*aiJob, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.job != nil {
		// r.job.State is written under job.mu by the running goroutine
		// (broadcastAIJobProgress) — r.mu only protects the registry's OWN
		// pointer, not the job struct's fields, so this read must go through
		// the job's own lock too (caught by -race: a direct r.job.State
		// read here raced against a concurrent job's State write).
		r.job.mu.Lock()
		running := r.job.State == "RUNNING"
		r.job.mu.Unlock()
		if running {
			return nil, false
		}
	}
	if provider == "" {
		provider = "anthropic" // contract §5 default — safe to set here: j isn't published (r.job = j) until below, so no reader can observe it yet
	}
	j := &aiJob{
		ID:              newAIJobID(),
		Provider:        provider,
		State:           "RUNNING",
		BatchesTotal:    batchesTotal,
		cancelRequested: make(chan struct{}),
	}
	r.job = j
	return j, true
}

func (r *aiJobRegistry) current() *aiJob {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.job
}

// newAIJobID builds an unguessable job id (security: "job_id non devinable")
// — crypto/rand, not math/rand or a predictable counter — prefixed with a
// timestamp purely for human readability in logs.
func newAIJobID() string {
	var buf [6]byte
	_, _ = rand.Read(buf[:]) // crypto/rand.Read only errors if the OS CSPRNG is unavailable; on that failure buf stays zero, which is degraded but never blocks job creation
	return fmt.Sprintf("gen-%s-%s", time.Now().UTC().Format("20060102-150405"), hex.EncodeToString(buf[:]))
}

// CurrentAIJobProgress exposes the running/last job's snapshot to cmd/server
// (main.go), which pushes it to a newly-identified admin client (contract
// §10: "émis immédiatement à la connexion d'un client admin si un job est en
// cours" — a page reload retrieves progress without any client-side
// reconstruction logic). Returns ok=false if no job has ever run this
// process lifetime.
func CurrentAIJobProgress() (protocol.AIGenerationProgressPayload, bool) {
	job := globalAIJobRegistry.current()
	if job == nil {
		return protocol.AIGenerationProgressPayload{}, false
	}
	snapshot := job.snapshot()
	if snapshot.State != "RUNNING" {
		// Contract §10 is specific: "si un job est EN COURS" — a client
		// connecting after the last job already reached a terminal state has
		// nothing to reattach to. Pushing a stale terminal snapshot here is
		// actively wrong, not just superfluous: a client that has not yet
		// started its OWN job could misread an old DONE/FAILED/CANCELLED as
		// belonging to a job it's about to create (confirmed by a real test
		// failure during development — a fresh admin connection observed the
		// PREVIOUS test's already-terminal job and treated it as its own).
		return protocol.AIGenerationProgressPayload{}, false
	}
	return snapshot, true
}

// pushAIJobProgressToNewAdmin implements contract ai-multi-provider.md §10:
// a newly-identified admin client (fresh connection to /ws/admin, or a
// legacy /ws client that just sent SET_CLIENT_TYPE) learns about a running
// generation job immediately — targeted (SendToClient), never a broadcast,
// so TV/player/buzzer clients never see it even transiently. Wired as
// WebSocketHub.OnClientRegistered in NewHTTPServer. No-op if the client
// isn't admin or no job has run yet.
func (h *HTTPServer) pushAIJobProgressToNewAdmin(client *WebSocketClient) {
	if client == nil || client.Type != ClientTypeAdmin {
		return
	}
	progress, ok := CurrentAIJobProgress()
	if !ok {
		return
	}
	msg, err := protocol.NewMessage(protocol.ActionAIGenerationProgress, progress)
	if err != nil {
		return
	}
	h.wsHub.SendToClient(client.ID, msg)
}

// CancelAIJob requests cancellation of the job matching jobID (contract
// §11). No-op (returns false) if jobID doesn't match the current job or no
// job is running — an admin's stale/reloaded modal cancelling a job that
// already finished must not error.
func CancelAIJob(jobID string) bool {
	job := globalAIJobRegistry.current()
	if job == nil {
		return false
	}
	job.mu.Lock()
	matches := job.ID == jobID && job.State == "RUNNING"
	job.mu.Unlock()
	if !matches {
		return false
	}
	job.requestCancel()
	return true
}

// ============================================================================
// Job execution
// ============================================================================

// startAIGenerationJob validates nothing itself (the caller, handleGenerateQuestions,
// already validated the request and the provider's API key) — it computes
// batchesTotal, atomically registers the job (M1), and launches the batch
// loop in a new goroutine. Returns immediately; the goroutine drives
// everything else via WS progress messages.
func (h *HTTPServer) startAIGenerationJob(req generateQuestionsRequest, aiCfg config.AIConfig) (jobID string, batchesTotal int, ok bool) {
	total := resolveTotalQuestionTarget(req.Volume, aiCfg.MaxQuestions)
	batchSize := clampBatchSize(aiCfg.BatchSize)
	batchesTotal = (total + batchSize - 1) / batchSize
	if batchesTotal < 1 {
		batchesTotal = 1
	}

	job, started := globalAIJobRegistry.tryStart(aiCfg.Provider, batchesTotal)
	if !started {
		return "", 0, false
	}
	// NOTE: the "" -> "anthropic" default is applied INSIDE tryStart (before
	// r.job is published) rather than here — this job struct becomes visible
	// to other goroutines (via CurrentAIJobProgress, read under job.mu) the
	// instant tryStart returns it, so mutating a field here afterward without
	// job.mu is a real race with a concurrent reader (caught by -race: a
	// newly-connecting admin's OnClientRegistered hook reading job.Provider
	// via snapshot() while this line wrote it unlocked).

	go h.runAIJob(job, req, aiCfg, total, batchSize, batchesTotal)

	return job.ID, batchesTotal, true
}

func clampBatchSize(configured int) int {
	batchSize := configured
	if batchSize <= 0 {
		batchSize = 20
	}
	if batchSize > 50 {
		batchSize = 50
	}
	return batchSize
}

// runAIJob is the batch loop, adapted from #8/#137-Phase-1's synchronous
// runGenerationBatches to run as a background goroutine with WS progress and
// cooperative cancellation (contract §2, §10, §11). Design notes:
//
//   - Provider calls use their OWN short-lived context (aiCfg.TimeoutSeconds),
//     never the job's cancellation signal — an in-flight call always
//     completes or times out on its own; cancellation is only checked
//     BETWEEN batches (contract §11 "jamais au milieu d'un appel provider").
//   - A batch that fails to generate/parse retries at the SAME batch index,
//     same "don't retry on validation attrition alone" rule as Phase 1.
//   - A rate-limit error (429/413) waits Retry-After (or exponential backoff,
//     capped at 120s) before retrying — contract §3. It ALSO counts toward
//     consecutiveFailures like any other batch failure, so persistent
//     rate-limiting still terminates the job (FAILED, provider_quota) rather
//     than retrying forever.
//   - The inter-batch delay (ai.inter_batch_delay_ms) is applied after a
//     SUCCESSFUL batch, before starting the next one, and is itself
//     cancellable.
func (h *HTTPServer) runAIJob(job *aiJob, req generateQuestionsRequest, aiCfg config.AIConfig, total, batchSize, batchesTotal int) {
	provider := h.selectProvider(aiCfg)
	maxFailures := aiCfg.MaxConsecutiveFailures
	if maxFailures <= 0 {
		maxFailures = 2
	}
	interBatchDelay := time.Duration(aiCfg.InterBatchDelayMs) * time.Millisecond
	if interBatchDelay <= 0 {
		interBatchDelay = 60 * time.Second
	}

	baseOrder := h.maxQuestionOrder()
	var extraContext []existingQuestionContext
	consecutiveFailures := 0
	var lastErr error

	finalState := "DONE"
	finalErrorCode := ""

batchLoop:
	for batchIndex := 0; batchIndex < batchesTotal; {
		if job.isCancelRequested() {
			finalState = "CANCELLED"
			finalErrorCode = ""
			break batchLoop
		}

		thisBatch := batchSize
		if batchIndex == batchesTotal-1 {
			thisBatch = total - batchSize*(batchesTotal-1)
		}
		if thisBatch <= 0 {
			thisBatch = 1
		}

		prompt := h.buildGenerationPrompt(req, thisBatch, extraContext)
		schema := provider.AdaptSchema(buildQuestionSchema(req.Categories, req.Difficulties))

		callCtx, cancel := context.WithTimeout(context.Background(), timeoutOrDefault(aiCfg.TimeoutSeconds))
		rawJSON, err := provider.Generate(callCtx, aiCfg, prompt, schema)
		cancel()

		if err != nil {
			lastErr = err
			consecutiveFailures++
			LogWarn(game.LogComponentHTTP, "AI generation job %s: batch %d/%d failed (%d/%d consecutive failures): %v",
				job.ID, batchIndex+1, batchesTotal, consecutiveFailures, maxFailures, err)

			if consecutiveFailures >= maxFailures {
				finalState = "FAILED"
				finalErrorCode = classifyJobErrorCode(lastErr)
				break batchLoop
			}

			var rateLimitErr *aiRateLimitError
			if errors.As(err, &rateLimitErr) {
				// Rate limit (429/413): retry the SAME batch after waiting —
				// moving on to the "next" batch would likely hit the exact
				// same per-minute limit immediately, silently dropping
				// content the admin asked for (contract §3; ai_batching_test.go
				// assumption #3: "un 429/413 avec retry-after déclenche une
				// NOUVELLE tentative du MÊME lot, pas un abandon vers le lot
				// suivant").
				wait := rateLimitErr.retryAfter
				if wait <= 0 {
					wait = backoffDelay(consecutiveFailures)
				}
				if cancelled := interruptibleSleep(job.cancelRequested, wait); cancelled {
					finalState = "CANCELLED"
					finalErrorCode = ""
					break batchLoop
				}
				continue // retry the SAME batchIndex — do not advance
			}

			// Any other failure (network, 5xx, auth, ...): this batch
			// contributes 0 questions and the job moves on to the NEXT
			// batch — a single bad batch must not stall the whole run
			// (contract §2 "Reprise": previous AND subsequent batches are
			// unaffected by one isolated failure, up to
			// max_consecutive_failures).
			batchIndex++
			continue
		}

		llmQuestions, parseErr := parseLLMResponse(rawJSON)
		if parseErr != nil {
			lastErr = parseErr
			consecutiveFailures++
			LogWarn(game.LogComponentHTTP, "AI generation job %s: batch %d/%d response schema mismatch (%d/%d consecutive failures): %v",
				job.ID, batchIndex+1, batchesTotal, consecutiveFailures, maxFailures, parseErr)
			if consecutiveFailures >= maxFailures {
				finalState = "FAILED"
				finalErrorCode = classifyJobErrorCode(lastErr)
				break batchLoop
			}
			// A malformed/non-conforming response is not a rate-limit
			// condition — same "move on, don't retry" rule as a generic
			// provider failure above.
			batchIndex++
			continue
		}
		consecutiveFailures = 0

		validQuestions, skipped := validateGeneratedQuestions(llmQuestions, req.Categories, req.Difficulties, aiCfg.MaxQuestions)

		var batchCreatedCount int
		var writeErr error
		for _, vq := range validQuestions {
			id, dir, allocErr := h.resolveQuestionDir("")
			if allocErr != nil {
				writeErr = allocErr
				break
			}
			job.mu.Lock()
			order := baseOrder + job.CreatedCount + batchCreatedCount + 1
			job.mu.Unlock()
			question := mapGeneratedQuestion(vq, id, order)
			data, marshalErr := json.MarshalIndent(question, "", "  ")
			if marshalErr != nil {
				writeErr = marshalErr
				break
			}
			if err := os.WriteFile(filepath.Join(dir, "question.json"), data, 0644); err != nil {
				writeErr = err
				break
			}
			batchCreatedCount++
			// #196 — feed back the PERSISTED type (question["TYPE"], already
			// normalized MEMOTION_PLUS→MEMOTION by mapGeneratedQuestion), not
			// the raw vq.Type: the anti-duplicate context describes what's
			// actually on disk, and the pseudo-type has no reason to appear
			// in a later batch's prompt either — same invariant as
			// question.json, applied to the one other surface TYPE reaches.
			persistedType, _ := question["TYPE"].(string)
			extraContext = append(extraContext, existingQuestionContext{Type: persistedType, Category: vq.Category, Question: vq.Question})
		}

		job.mu.Lock()
		job.CreatedCount += batchCreatedCount
		job.SkippedCount += len(skipped)
		if batchCreatedCount > 0 {
			job.BatchesDone = batchIndex + 1
		}
		job.mu.Unlock()

		// Broadcast questions FIRST, progress SECOND (contract §10: "Émis
		// après le broadcast des questions du lot, pour que la liste soit
		// déjà à jour côté client").
		if batchCreatedCount > 0 && h.OnQuestionUpload != nil {
			h.OnQuestionUpload()
		}

		if writeErr != nil {
			finalState = "FAILED"
			if errors.Is(writeErr, ErrQuestionIDExhausted) {
				finalErrorCode = "id_exhausted"
			} else {
				LogError(game.LogComponentHTTP, "AI generation job %s: failed to write question: %v", job.ID, writeErr)
				finalErrorCode = "internal_error"
			}
			h.broadcastAIJobProgress(job, "FAILED", finalErrorCode, sanitizeUpstreamMessage(writeErr.Error()))
			return
		}

		job.mu.Lock()
		job.BatchesDone = batchIndex + 1
		job.mu.Unlock()
		h.broadcastAIJobProgress(job, "RUNNING", "", "")

		batchIndex++
		if batchIndex < batchesTotal {
			if cancelled := interruptibleSleep(job.cancelRequested, interBatchDelay); cancelled {
				finalState = "CANCELLED"
				finalErrorCode = ""
				break batchLoop
			}
		}
	}

	// The loop can end "naturally" (batchIndex reaches batchesTotal without
	// ever hitting break batchLoop) after a batch failed WITHOUT tripping
	// consecutiveFailures >= maxFailures — trivially true whenever
	// batchesTotal is small (a single-batch job has nowhere to "advance" to
	// after its one and only batch fails once). Left as-is this reports
	// DONE with CREATED_COUNT=0, which is a false success: the job really
	// did fail to produce anything, for a real reason (lastErr). Same
	// principle as #8's R9 ("never a false 200"), carried into the job
	// model: zero questions AND a generation-level error on record means
	// FAILED, not DONE — caught by a test asserting a single-batch job's
	// lone 401/timeout must surface as FAILED, not silently succeed empty.
	if finalState == "DONE" {
		job.mu.Lock()
		created := job.CreatedCount
		job.mu.Unlock()
		if created == 0 && lastErr != nil {
			finalState = "FAILED"
			finalErrorCode = classifyJobErrorCode(lastErr)
		}
	}

	// finalErrorMessage (#142-adjacent verbosity fix): lastErr's own Error()
	// already carries the sanitized upstream detail when it originated from
	// classifyGroqError/classifyAnthropicError (aiUpstreamError.message) —
	// sanitizeUpstreamMessage is applied again here regardless of origin
	// (parseLLMResponse's schema-mismatch error, a local error, is not
	// upstream text but running it through the same filter is harmless and
	// keeps this call site's behavior independent of exactly which error type
	// lastErr holds). Only meaningful when the job actually ended FAILED.
	finalErrorMessage := ""
	if finalState == "FAILED" && lastErr != nil {
		finalErrorMessage = sanitizeUpstreamMessage(lastErr.Error())
	}
	h.broadcastAIJobProgress(job, finalState, finalErrorCode, finalErrorMessage)
}

// broadcastAIJobProgress updates the job's terminal fields (when state/code/
// message are non-empty overrides) and pushes AI_GENERATION_PROGRESS to every
// /ws/admin client (contract §10 — never /ws/tv, /ws/player, /ws/buzzer).
// errorMessage is the sanitized admin-facing detail behind errorCode
// (#142-adjacent verbosity fix) — callers pass "" for non-error broadcasts
// (RUNNING) same as they already do for errorCode.
func (h *HTTPServer) broadcastAIJobProgress(job *aiJob, state, errorCode, errorMessage string) {
	job.mu.Lock()
	if state != "" {
		job.State = state
	}
	if errorCode != "" {
		job.ErrorCode = errorCode
	}
	if errorMessage != "" {
		job.ErrorMessage = errorMessage
	}
	payload := protocol.AIGenerationProgressPayload{
		JobID: job.ID, State: job.State, BatchesDone: job.BatchesDone, BatchesTotal: job.BatchesTotal,
		CreatedCount: job.CreatedCount, SkippedCount: job.SkippedCount, ErrorCode: job.ErrorCode,
		ErrorMessage: job.ErrorMessage, Provider: job.Provider,
	}
	job.mu.Unlock()

	if h.wsHub == nil {
		return // unit tests constructing a minimal HTTPServer without a wsHub
	}
	msg, err := protocol.NewMessage(protocol.ActionAIGenerationProgress, payload)
	if err != nil {
		return
	}
	h.wsHub.BroadcastToTypes(msg, ClientTypeAdmin)
}

// classifyJobErrorCode maps a generation-phase error (provider/parse) to the
// job's ERROR_CODE (contract §3 stable codes + "provider_quota"). Persistent
// rate-limiting (the failure that finally tripped consecutiveFailures was
// itself a rate-limit error) maps to provider_quota rather than a generic
// upstream_error (contract §3 "sur 429 persistant, remonter provider_quota").
func classifyJobErrorCode(err error) string {
	var rateLimitErr *aiRateLimitError
	if errors.As(err, &rateLimitErr) {
		return "provider_quota"
	}
	var timeoutErr *aiTimeoutError
	if errors.As(err, &timeoutErr) {
		return "timeout"
	}
	return "upstream_error"
}

func timeoutOrDefault(seconds int) time.Duration {
	if seconds <= 0 {
		return 300 * time.Second
	}
	return time.Duration(seconds) * time.Second
}

// backoffDelay is the exponential backoff used when a rate-limit error
// carries no Retry-After header (contract §3: "backoff exponentiel plafonné
// à 120 s"): 2s, 4s, 8s, 16s, ... capped at 120s.
func backoffDelay(attempt int) time.Duration {
	d := 2 * time.Second
	for i := 1; i < attempt; i++ {
		d *= 2
		if d >= 120*time.Second {
			return 120 * time.Second
		}
	}
	return d
}

// interruptibleSleep waits for d or until cancelCh is closed, whichever
// comes first. Returns true if interrupted by cancellation.
func interruptibleSleep(cancelCh <-chan struct{}, d time.Duration) bool {
	if d <= 0 {
		select {
		case <-cancelCh:
			return true
		default:
			return false
		}
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return false
	case <-cancelCh:
		return true
	}
}
