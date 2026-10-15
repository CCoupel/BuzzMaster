package server

// Tests du helper waitRegistered (#274) — voir testhelpers_register_274_test.go.
//
// Contrat convenu : waitRegistered(t, hub, want) attend PAR CONDITION (jamais par Sleep fixe)
// que hub.ClientCount() >= want, avec une échéance de 3 s ; l'échéance dépassée échoue le test
// de façon explicite (t.Fatalf). Un compteur déjà atteint rend la main sans attendre.

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// dialRaw274 ouvre une connexion SANS passer par dialWSPath : le test du helper
// ne doit pas dépendre de l'attente d'inscription que ce même helper alimente.
// Sans appel à t.Fatalf : utilisable depuis une goroutine ; l'erreur est renvoyée.
func dialRaw274(t *testing.T, srv *httptest.Server, path string) error {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + path
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("dial %s : %w", path, err)
	}
	t.Cleanup(func() { conn.Close() })
	return nil
}

// Cas 1 — succès : après un dial concurrent de N clients, waitRegistered ne rend la main
// que lorsque les N clients sont inscrits dans le hub.
func TestWaitRegistered274_ReturnsOnceAllClientsAreRegistered(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	const want = 3
	var wg sync.WaitGroup
	for i := 0; i < want; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := dialRaw274(t, srv, "/ws/admin"); err != nil {
				t.Errorf("%v", err)
			}
		}()
	}
	wg.Wait() // upgrades terminés : l'inscription, elle, peut encore être en cours

	waitRegistered(t, hub, want)

	if got := hub.ClientCount(); got < want {
		t.Fatalf("waitRegistered est revenu avec %d client(s) inscrit(s), attendu au moins %d", got, want)
	}
}

// Cas 2 — compteur déjà atteint : retour immédiat, sans consommer l'échéance.
func TestWaitRegistered274_AlreadyReachedReturnsImmediately(t *testing.T) {
	srv, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	if err := dialRaw274(t, srv, "/ws/tv"); err != nil {
		t.Fatal(err)
	}
	// Première attente : on établit l'état « déjà atteint » avec le helper lui-même.
	waitRegistered(t, hub, 1)

	start := time.Now()
	waitRegistered(t, hub, 1) // déjà atteint
	waitRegistered(t, hub, 0) // trivialement atteint
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("compteur déjà atteint : retour en %v, attendu quasi immédiat (< 500 ms)", elapsed)
	}
}

// Cas 3 — échéance dépassée : le helper échoue explicitement (il ne rend pas la main
// silencieusement, ne bloque pas indéfiniment). Le *testing.T du helper est un T isolé,
// exécuté dans sa propre goroutine, pour observer l'échec sans faire échouer ce test.
func TestWaitRegistered274_DeadlineExceededFailsExplicitly(t *testing.T) {
	_, hub, cleanup := startTestWSServer(t)
	defer cleanup()

	probe := &testing.T{}
	done := make(chan struct{})
	start := time.Now()
	go func() {
		defer close(done)             // t.Fatalf termine la goroutine par runtime.Goexit : le defer s'exécute
		waitRegistered(probe, hub, 1) // aucun client ne s'inscrira
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("waitRegistered ne rend pas la main : l'échéance (3 s) n'est pas appliquée")
	}

	if !probe.Failed() {
		t.Fatal("échéance dépassée : waitRegistered aurait dû échouer le test (t.Fatalf), il a rendu la main sans erreur")
	}
	if elapsed := time.Since(start); elapsed < 2*time.Second {
		t.Fatalf("waitRegistered a échoué après %v : échéance attendue ≈ 3 s (pas d'abandon prématuré)", elapsed)
	}
}
