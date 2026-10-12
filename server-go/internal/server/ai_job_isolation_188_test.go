package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// #188 — flakiness du paquet : globalAIJobRegistry n'autorise qu'un job RUNNING
// à la fois. Un test qui rend la main alors que son job asynchrone tourne encore
// fait répondre 409 aux tests suivants du même binaire (TestGenerateQuestions_*,
// ..._OnlyOneStarts_Race). Contrat de test : à la fin de tout test qui a utilisé
// setupTestHTTPServer, plus aucun job n'est RUNNING (drainAIJobRegistry).

// aiJobRunning reporte si le job courant du registre global est RUNNING.
func aiJobRunning() bool {
	j := globalAIJobRegistry.current()
	if j == nil {
		return false
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.State == "RUNNING"
}

// drainAIJobRegistry demande l'annulation du job courant s'il tourne et attend
// son état terminal (échéance large, 30 s ; attente par sondage court, sans
// durée fixe). À appeler dans un t.Cleanup enregistré AVANT les mocks amont :
// les cleanups s'exécutent en ordre inverse, donc les mocks sont fermés d'abord.
func drainAIJobRegistry(t *testing.T) {
	t.Helper()
	if j := globalAIJobRegistry.current(); j != nil && aiJobRunning() {
		j.requestCancel()
	}
	deadline := time.Now().Add(30 * time.Second)
	for aiJobRunning() {
		if time.Now().After(deadline) {
			t.Errorf("#188 : un job IA est resté RUNNING à la fin du test (échéance 30 s) — il ferait répondre 409 aux tests suivants")
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Reproduction déterministe : le sous-test lance un job dont l'amont est
// bloqué, puis rend la main SANS l'attendre. Une fois le sous-test terminé
// (cleanups compris), aucun job RUNNING ne doit subsister, et un nouveau
// POST ne doit pas recevoir de 409.
func TestAIJob_NoRunningJobLeaksBetweenTests_188(t *testing.T) {
	t.Run("laisse_un_job_en_cours", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		setAIConfig(batchingAIConfig("anthropic", 1, 10, 2))

		release := make(chan struct{})
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-release:
			case <-r.Context().Done():
			}
			w.WriteHeader(http.StatusInternalServerError)
		}))
		withAnthropicBaseURL(t, upstream.URL)
		t.Cleanup(upstream.Close)
		t.Cleanup(func() { close(release) }) // s'exécute avant Close : débloque le handler

		expectAccepted(t, postGenerateQuestions(server, baseGenerateRequest()))
		if !aiJobRunning() {
			t.Fatal("précondition : le job doit être RUNNING (amont bloqué)")
		}
		// le sous-test rend la main sans attendre le job
	})

	if aiJobRunning() {
		t.Fatal("#188 : un job IA est resté RUNNING après la fin du test précédent — fuite vers les tests suivants (409)")
	}

	t.Run("test_suivant_n_est_pas_en_409", func(t *testing.T) {
		server, _ := setupTestHTTPServer(t)
		setAIConfig(batchingAIConfig("anthropic", 1, 10, 2))
		upstream := mockAnthropicSSEServer(t, []map[string]interface{}{
			llmQuestion(map[string]interface{}{"TYPE": "SPEEDY", "ANSWER": "ok"}),
		})
		withAnthropicBaseURL(t, upstream.URL)
		w := postGenerateQuestions(server, baseGenerateRequest())
		if w.Code == http.StatusConflict {
			t.Fatalf("#188 : 409 sur le premier POST du test suivant (job résiduel)")
		}
	})
}
