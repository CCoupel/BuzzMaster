package server

// Tests dérivés de contracts/ai-key-validation.md (v6.0.3, #9) et de
// _work/reports/plan-20260809-104602.md (Batch 1, tâche 6) — suite Go
// d'intégration pour POST /api/ai/validate-key : dev-backend et test-writer
// tournent en parallèle sur le même contrat (même motif que
// ai_generation_test.go en tête de fichier lors de #8).
//
// Réutilise les helpers déjà en place dans ce package (même fichier de test
// binaire) : setupTestHTTPServer/setAIConfig/decodeJSONBody
// (ai_generation_test.go), validAIConfig. Les mocks/helpers ci-dessous sont
// nouveaux et nommés distinctement (mockValidateUpstream*, postValidateKey,
// resetAIValidateCooldown) pour ne rien redéclarer.
//
// ⚠️ Comme le reste du package (cf. commentaire d'ai_batching_test.go),
// config.SetInstance mute un singleton global — PAS de t.Parallel() ici.
//
// ⚠️ globalAIValidateCooldown (ai_validate.go) est LUI AUSSI un singleton
// global, non réinitialisé entre tests par le code de production (il n'y a
// aucune raison qu'il le soit en dehors des tests — le cooldown DOIT
// survivre entre deux requêtes HTTP réelles). resetAIValidateCooldown()
// ci-dessous le remet à zéro en tête de chaque test qui n'exerce PAS
// explicitly le cooldown lui-même, pour rendre chaque test indépendant de
// l'ordre d'exécution — seule la section Cooldown s'appuie sur son état
// naturel.
//
// ⚠️ Coût de suite : TestValidateAPIKey_Anthropic_Timeout_Unreachable attend
// réellement le plafond de 10 s codé en dur par le contrat §2 (délibérément
// PAS dérivé de ai.timeout_seconds, donc pas raccourcissable côté test comme
// le fait ai_generation_test.go pour le timeout de génération). ~10 s pour
// cette seule table — accepté comme le prix honnête de vérifier un vrai
// plafond, gardé néanmoins derrière testing.Short() pour qui itère avec
// `go test -short` (le testCmd du projet, `go test ./...`, ne passe pas
// -short et l'exécutera donc en entier).
//
// Signalé au CDP/dev-backend en cours de rédaction : ai_validate.go déclare
// `const anthropicBaseURLEnvVar = "ANTHROPIC_BASE_URL"` au niveau paquet,
// qui entre en collision avec la même constante déjà déclarée dans
// ai_generation_test.go:47 — erreur de compilation tant que ce n'est pas
// corrigé côté dev-backend. Ce fichier n'y touche pas : les helpers
// withValidateAnthropicBaseURL/withValidateGroqBaseURL ci-dessous posent la
// variable d'environnement par chaîne littérale, sans référencer aucune des
// deux constantes en collision.

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"buzzcontrol/internal/config"
)

// ================================================================================
// Helpers
// ================================================================================

func postValidateKey(server *HTTPServer, payload map[string]interface{}) *httptest.ResponseRecorder {
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/ai/validate-key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	return w
}

// resetAIValidateCooldown gives every test below (except the dedicated
// Cooldown section) a clean slate, independent of whatever a previous test
// in this binary already did to the package-level singleton.
func resetAIValidateCooldown() {
	globalAIValidateCooldown = &aiValidateCooldownState{}
}

func withValidateAnthropicBaseURL(t *testing.T, url string) {
	t.Helper()
	t.Setenv("ANTHROPIC_BASE_URL", url)
}

func withValidateGroqBaseURL(t *testing.T, url string) {
	t.Helper()
	t.Setenv("GROQ_BASE_URL", url)
}

// capturedUpstreamRequest snapshots what the server-under-test actually sent
// upstream — used to assert the exact endpoint/method/headers contract §2
// mandates, and (Section D) that it is the models-list endpoint, never the
// generation endpoint.
type capturedUpstreamRequest struct {
	Method string
	Path   string // r.URL.RequestURI() — includes the query string (?limit=1)
	Header http.Header
}

// mockValidateUpstream serves a fixed status+body for every request and,
// when capture is non-nil, records the request's method/path/headers before
// responding.
func mockValidateUpstream(t *testing.T, statusCode int, body string, capture *capturedUpstreamRequest) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if capture != nil {
			capture.Method = r.Method
			capture.Path = r.URL.RequestURI()
			capture.Header = r.Header.Clone()
		}
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// mockValidateUpstreamCountingCalls never actually gets hit in a well-behaved
// prefix-rejection test — it exists precisely so those tests can PROVE that,
// by asserting the counter stayed at 0.
func mockValidateUpstreamCountingCalls(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

// mockValidateUpstreamHanging blocks until the request's context is
// cancelled — the client-side counterpart of a provider that never answers.
// Used by the timeout test: the SERVER never decides to give up, only the
// CLIENT's context deadline (aiValidateTimeout, 10s) does — same motif as
// mockAnthropicSlowServer (ai_generation_test.go) but open-ended rather than
// a fixed delay, since aiValidateTimeout isn't configurable from a test.
func mockValidateUpstreamHanging(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	return srv
}

// downUpstreamURL returns a URL nothing is listening on — the fastest,
// deterministic way to exercise "connexion coupée" (contract §3: any
// network/DNS/TLS failure classifies as unreachable, exactly like a
// timeout — doKeyValidationRequest's `err != nil` branch doesn't
// distinguish the two).
func downUpstreamURL(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing listens at `url` from this point on
	return url
}

// captureLogOutput redirects the standard `log` package's output (what both
// LogInfo's global-logger path AND its no-logger fallback ultimately write
// through, cf. logger.go) into a buffer for the duration of the test, so
// tests can assert on log hygiene without any project-specific log-capture
// plumbing.
func captureLogOutput(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(orig) })
	return &buf
}

// ================================================================================
// Section A — validation de la requête, avant tout appel réseau (contrat §5)
// ================================================================================

func TestValidateAPIKey_MethodNotAllowed(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()

	req := httptest.NewRequest("GET", "/api/ai/validate-key", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for GET /api/ai/validate-key, got %d: %s", w.Code, w.Body.String())
	}
}

func TestValidateAPIKey_MalformedJSON(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()

	req := httptest.NewRequest("POST", "/api/ai/validate-key", bytes.NewReader([]byte("{not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for malformed JSON, got %d: %s", w.Code, w.Body.String())
	}
}

func TestValidateAPIKey_UnknownProvider(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()

	w := postValidateKey(server, map[string]interface{}{"provider": "openai", "api_key": "whatever"})

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for an unrecognized provider, got %d: %s", w.Code, w.Body.String())
	}
}

func TestValidateAPIKey_ProviderAbsent(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()

	// No "provider" key at all — same code path as an unrecognized value
	// (Go's zero value for a missing string field is "", which the handler's
	// switch's default case rejects exactly like "openai" above).
	w := postValidateKey(server, map[string]interface{}{"api_key": "sk-ant-whatever"})

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 when \"provider\" is absent, got %d: %s", w.Code, w.Body.String())
	}
}

func TestValidateAPIKey_BodyTooLarge(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()

	// 64 KB ceiling (contract §5) — pad well past it with an otherwise
	// well-formed JSON body so MaxBytesReader, not the JSON parser, is what
	// trips.
	huge := `{"provider":"anthropic","api_key":"sk-ant-` + strings.Repeat("A", 70*1024) + `"}`
	req := httptest.NewRequest("POST", "/api/ai/validate-key", bytes.NewReader([]byte(huge)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected 413 for a body over 64KB, got %d: %s", w.Code, w.Body.String())
	}
}

func TestValidateAPIKey_InvalidPrefix_Anthropic_NoNetworkCall(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	upstream, calls := mockValidateUpstreamCountingCalls(t)
	withValidateAnthropicBaseURL(t, upstream.URL)

	w := postValidateKey(server, map[string]interface{}{"provider": "anthropic", "api_key": "foo-not-a-real-prefix"})

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for a malformed key prefix, got %d: %s", w.Code, w.Body.String())
	}
	if got := atomic.LoadInt32(calls); got != 0 {
		t.Errorf("Expected ZERO upstream calls for a prefix rejection (contract §5: checked BEFORE any network call), got %d", got)
	}
}

func TestValidateAPIKey_InvalidPrefix_Groq_NoNetworkCall(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	upstream, calls := mockValidateUpstreamCountingCalls(t)
	withValidateGroqBaseURL(t, upstream.URL)

	w := postValidateKey(server, map[string]interface{}{"provider": "groq", "api_key": "sk-ant-wrong-family"})

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for a malformed key prefix, got %d: %s", w.Code, w.Body.String())
	}
	if got := atomic.LoadInt32(calls); got != 0 {
		t.Errorf("Expected ZERO upstream calls for a prefix rejection, got %d", got)
	}
}

// ================================================================================
// Section B — taxonomie du résultat (contrat §3), fournisseur Anthropic
// ================================================================================

func TestValidateAPIKey_Anthropic_Valid200(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	upstream := mockValidateUpstream(t, http.StatusOK, `{"data":[{"id":"claude-opus-5"}]}`, nil)
	withValidateAnthropicBaseURL(t, upstream.URL)

	w := postValidateKey(server, map[string]interface{}{"provider": "anthropic", "api_key": "sk-ant-a-valid-looking-key"})

	if w.Code != http.StatusOK {
		t.Fatalf("Expected HTTP 200 envelope (contract §5: verdict lives in the body, not the status), got %d: %s", w.Code, w.Body.String())
	}
	resp := decodeJSONBody(t, w)
	if resp["result"] != "valid" {
		t.Errorf("Expected result=valid, got %v", resp["result"])
	}
	if resp["provider"] != "anthropic" {
		t.Errorf("Expected provider echoed back as \"anthropic\", got %v", resp["provider"])
	}
	if resp["http_status"] != float64(200) {
		t.Errorf("Expected http_status=200, got %v", resp["http_status"])
	}
}

func TestValidateAPIKey_Anthropic_401_InvalidKey(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	upstream := mockValidateUpstream(t, http.StatusUnauthorized, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`, nil)
	withValidateAnthropicBaseURL(t, upstream.URL)

	w := postValidateKey(server, map[string]interface{}{"provider": "anthropic", "api_key": "sk-ant-revoked-key"})

	resp := decodeJSONBody(t, w)
	if resp["result"] != "invalid_key" {
		t.Errorf("Expected result=invalid_key for a 401, got %v", resp["result"])
	}
	if resp["http_status"] != float64(401) {
		t.Errorf("Expected http_status=401, got %v", resp["http_status"])
	}
	if !strings.Contains(resp["detail"].(string), "invalid x-api-key") {
		t.Errorf("Expected the upstream detail to surface (sanitized), got %v", resp["detail"])
	}
}

func TestValidateAPIKey_Anthropic_403_InvalidKey(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	upstream := mockValidateUpstream(t, http.StatusForbidden, `{"error":{"message":"forbidden"}}`, nil)
	withValidateAnthropicBaseURL(t, upstream.URL)

	w := postValidateKey(server, map[string]interface{}{"provider": "anthropic", "api_key": "sk-ant-forbidden-key"})

	resp := decodeJSONBody(t, w)
	if resp["result"] != "invalid_key" {
		t.Errorf("Expected result=invalid_key for a 403, got %v", resp["result"])
	}
}

// Contract §3: "429 → unreachable, délibérément" — the single most
// consequential classification rule of this feature. A rate-limited call
// likely means auth passed, but nothing is confirmed; classifying it
// invalid_key would make an operator "correct" a perfectly good key.
func TestValidateAPIKey_Anthropic_429_IsUnreachable_NeverInvalidKey(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	upstream := mockValidateUpstream(t, http.StatusTooManyRequests, `{"error":{"message":"rate limited"}}`, nil)
	withValidateAnthropicBaseURL(t, upstream.URL)

	w := postValidateKey(server, map[string]interface{}{"provider": "anthropic", "api_key": "sk-ant-a-fine-key"})

	resp := decodeJSONBody(t, w)
	if resp["result"] != "unreachable" {
		t.Fatalf("Contract §3 violation: a 429 MUST classify as unreachable, got %v (http_status=%v)", resp["result"], resp["http_status"])
	}
	if resp["http_status"] != float64(429) {
		t.Errorf("Expected http_status=429, got %v", resp["http_status"])
	}
}

func TestValidateAPIKey_Anthropic_500_Unreachable(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	upstream := mockValidateUpstream(t, http.StatusInternalServerError, `{"error":{"message":"internal error"}}`, nil)
	withValidateAnthropicBaseURL(t, upstream.URL)

	w := postValidateKey(server, map[string]interface{}{"provider": "anthropic", "api_key": "sk-ant-a-fine-key"})

	resp := decodeJSONBody(t, w)
	if resp["result"] != "unreachable" {
		t.Errorf("Expected result=unreachable for a 500, got %v", resp["result"])
	}
	if resp["http_status"] != float64(500) {
		t.Errorf("Expected http_status=500, got %v", resp["http_status"])
	}
}

func TestValidateAPIKey_Anthropic_NetworkDown_Unreachable(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	withValidateAnthropicBaseURL(t, downUpstreamURL(t))

	w := postValidateKey(server, map[string]interface{}{"provider": "anthropic", "api_key": "sk-ant-a-fine-key"})

	if w.Code != http.StatusOK {
		t.Fatalf("A network failure is still a completed validation (contract §5: \"aucun 5xx pour un échec fournisseur\"), expected 200, got %d", w.Code)
	}
	resp := decodeJSONBody(t, w)
	if resp["result"] != "unreachable" {
		t.Errorf("Expected result=unreachable when the upstream is unreachable, got %v", resp["result"])
	}
	if resp["http_status"] != float64(0) {
		t.Errorf("Expected http_status=0 (no HTTP response was ever received), got %v", resp["http_status"])
	}
}

// Slow (~10s wall clock) — see file header. Exercises the hard-coded
// aiValidateTimeout ceiling (contract §2), not the generation path's
// configurable ai.timeout_seconds.
func TestValidateAPIKey_Anthropic_Timeout_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ~10s hard-timeout test in -short mode")
	}
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	upstream := mockValidateUpstreamHanging(t)
	withValidateAnthropicBaseURL(t, upstream.URL)

	start := time.Now()
	w := postValidateKey(server, map[string]interface{}{"provider": "anthropic", "api_key": "sk-ant-a-fine-key"})
	elapsed := time.Since(start)

	resp := decodeJSONBody(t, w)
	if resp["result"] != "unreachable" {
		t.Errorf("Expected result=unreachable on timeout, got %v", resp["result"])
	}
	if resp["http_status"] != float64(0) {
		t.Errorf("Expected http_status=0 on timeout (no response ever received), got %v", resp["http_status"])
	}
	// Loose bounds: must not fire noticeably before 10s (proves the ceiling
	// isn't shorter than contracted) nor take drastically longer (proves the
	// context deadline is actually enforced, not silently ignored).
	if elapsed < 9*time.Second {
		t.Errorf("Timeout fired too early (%v) — expected ~10s per contract §2", elapsed)
	}
	if elapsed > 15*time.Second {
		t.Errorf("Timeout took too long (%v) — the 10s ceiling doesn't seem enforced", elapsed)
	}
}

// Same 2s-under-the-ceiling check as a fast sanity test: a merely slow (but
// still well within 10s) upstream must NOT be cut off prematurely.
func TestValidateAPIKey_Anthropic_SlowButWithinTimeout_StillValid(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(upstream.Close)
	withValidateAnthropicBaseURL(t, upstream.URL)

	w := postValidateKey(server, map[string]interface{}{"provider": "anthropic", "api_key": "sk-ant-a-fine-key"})

	resp := decodeJSONBody(t, w)
	if resp["result"] != "valid" {
		t.Errorf("A 2s-slow-but-successful upstream must not be treated as a timeout, got result=%v", resp["result"])
	}
}

// ================================================================================
// Section C — fournisseur Groq (le code de classification est partagé avec
// Anthropic via doKeyValidationRequest — on ne re-couvre pas les 7 statuts,
// seulement le routage/les en-têtes propres à Groq et un échantillon de la
// taxonomie).
// ================================================================================

func TestValidateAPIKey_Groq_Valid200(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	upstream := mockValidateUpstream(t, http.StatusOK, `{"data":[{"id":"openai/gpt-oss-120b"}]}`, nil)
	withValidateGroqBaseURL(t, upstream.URL)

	w := postValidateKey(server, map[string]interface{}{"provider": "groq", "api_key": "gsk_a_valid_looking_key"})

	resp := decodeJSONBody(t, w)
	if resp["result"] != "valid" {
		t.Errorf("Expected result=valid, got %v", resp["result"])
	}
	if resp["provider"] != "groq" {
		t.Errorf("Expected provider echoed back as \"groq\", got %v", resp["provider"])
	}
}

func TestValidateAPIKey_Groq_401_InvalidKey(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	upstream := mockValidateUpstream(t, http.StatusUnauthorized, `{"error":{"message":"Invalid API Key"}}`, nil)
	withValidateGroqBaseURL(t, upstream.URL)

	w := postValidateKey(server, map[string]interface{}{"provider": "groq", "api_key": "gsk_revoked_key"})

	resp := decodeJSONBody(t, w)
	if resp["result"] != "invalid_key" {
		t.Errorf("Expected result=invalid_key for a 401, got %v", resp["result"])
	}
}

func TestValidateAPIKey_Groq_429_IsUnreachable(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	upstream := mockValidateUpstream(t, http.StatusTooManyRequests, `{"error":{"message":"rate limited"}}`, nil)
	withValidateGroqBaseURL(t, upstream.URL)

	w := postValidateKey(server, map[string]interface{}{"provider": "groq", "api_key": "gsk_a_fine_key"})

	resp := decodeJSONBody(t, w)
	if resp["result"] != "unreachable" {
		t.Errorf("Contract §3: a 429 MUST classify as unreachable, got %v", resp["result"])
	}
}

// ================================================================================
// Section D — non-régression : c'est bien l'endpoint de LISTE DE MODÈLES qui
// est appelé, jamais celui de génération (contrat §6: "Aucune modification
// du chemin de génération... Generate n'est jamais appelé sur ce chemin").
// ================================================================================

func TestValidateAPIKey_Anthropic_HitsModelsEndpoint_NotGenerate(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	var captured capturedUpstreamRequest
	upstream := mockValidateUpstream(t, http.StatusOK, `{"data":[]}`, &captured)
	withValidateAnthropicBaseURL(t, upstream.URL)

	postValidateKey(server, map[string]interface{}{"provider": "anthropic", "api_key": "sk-ant-a-fine-key"})

	if captured.Method != http.MethodGet {
		t.Errorf("Expected GET (contract §2), got %s", captured.Method)
	}
	if captured.Path != "/v1/models?limit=1" {
		t.Errorf("Expected the models-list endpoint /v1/models?limit=1 (contract §2) — NOT the generation endpoint (/v1/messages) — got %q", captured.Path)
	}
	if got := captured.Header.Get("x-api-key"); got != "sk-ant-a-fine-key" {
		t.Errorf("Expected x-api-key header carrying the key under test, got %q", got)
	}
	if got := captured.Header.Get("anthropic-version"); got != "2023-06-01" {
		t.Errorf("Expected anthropic-version: 2023-06-01 (contract §2), got %q", got)
	}
}

func TestValidateAPIKey_Groq_HitsModelsEndpoint_NotGenerate(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	var captured capturedUpstreamRequest
	upstream := mockValidateUpstream(t, http.StatusOK, `{"data":[]}`, &captured)
	withValidateGroqBaseURL(t, upstream.URL)

	postValidateKey(server, map[string]interface{}{"provider": "groq", "api_key": "gsk_a_fine_key"})

	if captured.Method != http.MethodGet {
		t.Errorf("Expected GET (contract §2), got %s", captured.Method)
	}
	if captured.Path != "/openai/v1/models" {
		t.Errorf("Expected the models-list endpoint /openai/v1/models (contract §2) — NOT the generation endpoint (/openai/v1/chat/completions) — got %q", captured.Path)
	}
	if got := captured.Header.Get("Authorization"); got != "Bearer gsk_a_fine_key" {
		t.Errorf("Expected \"Authorization: Bearer <key>\" (contract §2), got %q", got)
	}
}

// ================================================================================
// Section E — résolution de la clé effective (contrat §5 "api_key absent ou
// vide", §7 "cas variable d'environnement")
// ================================================================================

func TestValidateAPIKey_EmptyAPIKey_UsesStoredKey(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	ai := validAIConfig()
	ai.AnthropicAPIKey = "sk-ant-stored-in-config-json"
	setAIConfig(ai)
	var captured capturedUpstreamRequest
	upstream := mockValidateUpstream(t, http.StatusOK, `{"data":[]}`, &captured)
	withValidateAnthropicBaseURL(t, upstream.URL)

	w := postValidateKey(server, map[string]interface{}{"provider": "anthropic", "api_key": ""})

	resp := decodeJSONBody(t, w)
	if resp["result"] != "valid" {
		t.Fatalf("Expected result=valid, got %v (body=%s)", resp["result"], w.Body.String())
	}
	if got := captured.Header.Get("x-api-key"); got != "sk-ant-stored-in-config-json" {
		t.Errorf("Expected the STORED key to be validated when api_key is empty (contract §5), got x-api-key=%q", got)
	}
}

func TestValidateAPIKey_EmptyAPIKey_EnvVarTakesPriorityOverStored(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	ai := validAIConfig()
	ai.AnthropicAPIKey = "sk-ant-stored-in-config-json"
	setAIConfig(ai)
	t.Setenv(config.EnvAnthropicAPIKey, "sk-ant-from-environment")
	var captured capturedUpstreamRequest
	upstream := mockValidateUpstream(t, http.StatusOK, `{"data":[]}`, &captured)
	withValidateAnthropicBaseURL(t, upstream.URL)

	postValidateKey(server, map[string]interface{}{"provider": "anthropic", "api_key": ""})

	if got := captured.Header.Get("x-api-key"); got != "sk-ant-from-environment" {
		t.Errorf("Expected the ENVIRONMENT key to win over the stored one (contract §7), got x-api-key=%q", got)
	}
}

func TestValidateAPIKey_EmptyAPIKey_GroqEnvVar(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	t.Setenv(config.EnvGroqAPIKey, "gsk_from_environment")
	var captured capturedUpstreamRequest
	upstream := mockValidateUpstream(t, http.StatusOK, `{"data":[]}`, &captured)
	withValidateGroqBaseURL(t, upstream.URL)

	w := postValidateKey(server, map[string]interface{}{"provider": "groq", "api_key": ""})

	resp := decodeJSONBody(t, w)
	if resp["result"] != "valid" {
		t.Fatalf("Expected result=valid, got %v (body=%s)", resp["result"], w.Body.String())
	}
	if got := captured.Header.Get("Authorization"); got != "Bearer gsk_from_environment" {
		t.Errorf("Expected the environment Groq key to be validated, got Authorization=%q", got)
	}
}

// Contract §5 "Effet de bord : aucun. N'écrit ni config.json ni le
// singleton." — even on a successful "valid" verdict, this endpoint alone
// must never mutate the stored config. Persisting *_api_key_verified is
// POST /config.json's job (contract §7), triggered separately by the
// frontend after reading this response — never by this handler itself.
func TestValidateAPIKey_NoSideEffect_DoesNotMutateStoredConfig(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	ai := validAIConfig()
	ai.AnthropicAPIKey = "sk-ant-original-stored-key"
	ai.AnthropicAPIKeyVerified = false
	setAIConfig(ai)
	upstream := mockValidateUpstream(t, http.StatusOK, `{"data":[]}`, nil)
	withValidateAnthropicBaseURL(t, upstream.URL)

	postValidateKey(server, map[string]interface{}{"provider": "anthropic", "api_key": "sk-ant-a-different-key-being-tested"})

	after := config.Get().AI
	if after.AnthropicAPIKey != "sk-ant-original-stored-key" {
		t.Errorf("POST /api/ai/validate-key must never write config.json's stored key — got AnthropicAPIKey=%q", after.AnthropicAPIKey)
	}
	if after.AnthropicAPIKeyVerified {
		t.Errorf("POST /api/ai/validate-key must never flip AnthropicAPIKeyVerified itself (that's POST /config.json's job) — got true")
	}
}

// ================================================================================
// Section F — hygiène de sécurité (contrat §8)
// ================================================================================

func TestValidateAPIKey_ResponseNeverContainsTheKey(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	const secretKey = "sk-ant-super-secret-value-0001"
	upstream := mockValidateUpstream(t, http.StatusUnauthorized, `{"error":{"message":"invalid key`+secretKey+`"}}`, nil)
	withValidateAnthropicBaseURL(t, upstream.URL)

	w := postValidateKey(server, map[string]interface{}{"provider": "anthropic", "api_key": secretKey})

	if strings.Contains(w.Body.String(), secretKey) {
		t.Fatalf("Security violation: the API key leaked into the HTTP response body: %s", w.Body.String())
	}
}

// sanitizeUpstreamMessage (ai_client.go) redacts API-key-shaped substrings
// from any upstream-supplied text before it reaches `detail` — this test
// proves that redaction is actually wired into THIS new path, not just the
// generation path it was originally built for.
func TestValidateAPIKey_DetailIsSanitized_KeyShapedSubstringRedacted(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	// The upstream error body itself echoes something key-shaped — a
	// provider misbehaving, or an overly chatty error message. Whatever the
	// cause, it must never survive into `detail` verbatim.
	upstream := mockValidateUpstream(t, http.StatusUnauthorized,
		`{"error":{"message":"key sk-ant-leaked-in-upstream-body-should-be-redacted was rejected"}}`, nil)
	withValidateAnthropicBaseURL(t, upstream.URL)

	w := postValidateKey(server, map[string]interface{}{"provider": "anthropic", "api_key": "sk-ant-a-fine-key"})

	resp := decodeJSONBody(t, w)
	detail, _ := resp["detail"].(string)
	if strings.Contains(detail, "sk-ant-leaked-in-upstream-body-should-be-redacted") {
		t.Errorf("sanitizeUpstreamMessage did not redact a key-shaped substring from detail: %q", detail)
	}
	if !strings.Contains(detail, "[redacted]") {
		t.Errorf("Expected the redaction marker \"[redacted]\" in detail, got %q", detail)
	}
}

func TestValidateAPIKey_LogsNeverContainTheKey(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	logs := captureLogOutput(t)
	const secretKey = "sk-ant-must-never-appear-in-logs"
	upstream := mockValidateUpstream(t, http.StatusOK, `{"data":[]}`, nil)
	withValidateAnthropicBaseURL(t, upstream.URL)

	postValidateKey(server, map[string]interface{}{"provider": "anthropic", "api_key": secretKey})

	if strings.Contains(logs.String(), secretKey) {
		t.Fatalf("Security violation (contract §8, motif M4): the API key leaked into the logs: %s", logs.String())
	}
}

func TestValidateAPIKey_ValidResult_DetailFieldOmittedFromJSON(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	upstream := mockValidateUpstream(t, http.StatusOK, `{"data":[]}`, nil)
	withValidateAnthropicBaseURL(t, upstream.URL)

	w := postValidateKey(server, map[string]interface{}{"provider": "anthropic", "api_key": "sk-ant-a-fine-key"})

	resp := decodeJSONBody(t, w)
	if _, present := resp["detail"]; present {
		t.Errorf(`Expected "detail" to be entirely absent from the JSON body on a valid result (json:"detail,omitempty"), got %v`, resp["detail"])
	}
}

// ================================================================================
// Section G — cooldown (contrat §8: "1 validation / 2 s, global au serveur")
// ================================================================================

func TestValidateAPIKey_Cooldown_SecondCallWithin2s_Gets429(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	upstream := mockValidateUpstream(t, http.StatusOK, `{"data":[]}`, nil)
	withValidateAnthropicBaseURL(t, upstream.URL)

	first := postValidateKey(server, map[string]interface{}{"provider": "anthropic", "api_key": "sk-ant-a-fine-key"})
	if first.Code != http.StatusOK {
		t.Fatalf("Expected the first call to succeed, got %d: %s", first.Code, first.Body.String())
	}

	second := postValidateKey(server, map[string]interface{}{"provider": "anthropic", "api_key": "sk-ant-a-fine-key"})
	if second.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 for a second validation within the 2s cooldown window, got %d: %s", second.Code, second.Body.String())
	}
}

func TestValidateAPIKey_Cooldown_IsGlobal_NotPerProvider(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	anthropicUpstream := mockValidateUpstream(t, http.StatusOK, `{"data":[]}`, nil)
	withValidateAnthropicBaseURL(t, anthropicUpstream.URL)
	groqUpstream := mockValidateUpstream(t, http.StatusOK, `{"data":[]}`, nil)
	withValidateGroqBaseURL(t, groqUpstream.URL)

	postValidateKey(server, map[string]interface{}{"provider": "anthropic", "api_key": "sk-ant-a-fine-key"})
	// Contract §8: "global au serveur" — a Claude validation must also
	// spend the cooldown slot for a following Groq validation, not just for
	// another Claude one.
	second := postValidateKey(server, map[string]interface{}{"provider": "groq", "api_key": "gsk_a_fine_key"})

	if second.Code != http.StatusTooManyRequests {
		t.Errorf("Expected the cooldown to be global across providers (contract §8), got %d for the immediate Groq call", second.Code)
	}
}

func TestValidateAPIKey_Cooldown_AfterWindowElapses_Succeeds(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	resetAIValidateCooldown()
	upstream := mockValidateUpstream(t, http.StatusOK, `{"data":[]}`, nil)
	withValidateAnthropicBaseURL(t, upstream.URL)

	first := postValidateKey(server, map[string]interface{}{"provider": "anthropic", "api_key": "sk-ant-a-fine-key"})
	if first.Code != http.StatusOK {
		t.Fatalf("Expected the first call to succeed, got %d", first.Code)
	}

	time.Sleep(2100 * time.Millisecond) // just over the 2s window
	second := postValidateKey(server, map[string]interface{}{"provider": "anthropic", "api_key": "sk-ant-a-fine-key"})
	if second.Code != http.StatusOK {
		t.Errorf("Expected a call after the cooldown window to succeed, got %d: %s", second.Code, second.Body.String())
	}
}
