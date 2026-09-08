// Suite d'acceptance test-writer pour POST /api/lighting/preview (P2c),
// milestone v10.0.0, planner-v10-general-theme-toggle-20260908-114420.md
// §Partie 2. Contre l'implémentation réelle de dev-backend (P2a, commit
// 611ee24c) et dev-frontend (P2b, commit e10d6c31).
//
// Complémentaire de :
//   - http_lighting_dev_test.go (dev-backend) :
//     TestDevLightingPreview_TurnsOnAndOffEvenUnconfigured (le cas des
//     rounds 1-5 — résolution live d'une ampoule non enregistrée, réponses
//     HTTP), TestDevLightingPreview_NoDriverIsRefused,
//     TestDevLightingPreview_BusyGuard (garde busy, mais SIMULÉE : le flag
//     est positionné à la main, jamais deux requêtes réellement
//     concurrentes) ;
//   - driver_dev_test.go (dev-backend, internal/lighting/hue) :
//     TestDevSetLightDirect_ResolvesLiveUnconfiguredLight (même cas au
//     niveau pilote, avec vérification du corps physique des PUT),
//     TestDevSetLightDirect_UnknownNameIsRefused,
//     TestDevSetLightDirect_InvalidatesWriterDedupCache ;
//   - AmbiancePage.preview.test.jsx (dev-frontend, 8 tests) : bascule
//     immédiate on:true/false côté JS, désactivation croisée des cases
//     pendant l'appel en vol, pont injoignable n'annule PAS la bascule
//     (côté état React), busy/refused, mention de l'état "pas encore
//     enregistré".
//
// Aucun de ces tests n'exerce deux appels RÉELLEMENT concurrents à
// /preview (TestDevLightingPreview_BusyGuard positionne le flag à la main,
// exactement comme TestDevLightingOneInFlightPerOperation le fait pour
// /register et /discover) — alors qu'un tel test existe déjà pour
// /register (TestDevLightingRegisterConcurrentIsSerialised,
// http_lighting_sec_dev_test.go). Ce fichier applique le même patron à
// /preview (item 5 de la tâche) et ajoute la moitié BACKEND du pont
// injoignable — la taxonomie d'erreur HTTP elle-même (item 4 ; sa moitié
// FRONTEND, "la case bascule quand même", est déjà couverte par
// AmbiancePage.preview.test.jsx).
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
// Aides préfixées twP (P2c) pour ne jamais entrer en collision.
package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"buzzcontrol/internal/lighting/hue"
)

// twPSlowHueBridge answers /config, /lights (GET) and /lights/<id>/state
// (PUT) like a real Hue v1 bridge, but the FIRST request it receives blocks
// until released — letting a test observe "the request is genuinely in
// flight" before firing a second, concurrent call. Every request after the
// first proceeds immediately (the gate channel is closed, not reset): the
// driver issues its 2-3 requests for one SetLightDirect call strictly in
// sequence, so blocking only the first is enough to hold the WHOLE
// operation in flight for as long as the test needs.
type twPSlowHueBridge struct {
	hits    atomic.Int32
	entered chan struct{}
	block   chan struct{}
	once    sync.Once
	srv     *httptest.Server
}

func newTwPSlowHueBridge(t *testing.T) *twPSlowHueBridge {
	t.Helper()
	b := &twPSlowHueBridge{entered: make(chan struct{}), block: make(chan struct{})}
	b.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.hits.Add(1)
		b.once.Do(func() { close(b.entered) })
		<-b.block
		switch {
		case strings.HasSuffix(r.URL.Path, "/config"):
			_, _ = w.Write([]byte(`{"bridgeid":"fffe0000deadbeef","modelid":"BSB002"}`))
		case strings.HasSuffix(r.URL.Path, "/lights") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"1":{"name":"BuzzHue1","state":{"on":false,"bri":1,"xy":[0.3,0.3],"reachable":true}}}`))
		case strings.Contains(r.URL.Path, "/lights/") && strings.HasSuffix(r.URL.Path, "/state") && r.Method == http.MethodPut:
			_, _ = w.Write([]byte(`[{"success":{"on":true}}]`))
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(b.srv.Close)
	return b
}

// TestPreview_ConcurrentIsSerialised is the genuine-concurrency counterpart
// of TestDevLightingPreview_BusyGuard (which only simulates the busy state
// by setting the flag directly) — modelled exactly on
// TestDevLightingRegisterConcurrentIsSerialised for /register. Two REAL
// /preview requests race; exactly one reaches the bridge while the other is
// in flight, the other is refused 429 without ever touching the network,
// and the first still succeeds once released.
func TestPreview_ConcurrentIsSerialised(t *testing.T) {
	bridge := newTwPSlowHueBridge(t)
	d, err := hue.New(hue.Config{
		BridgeIP: bridge.srv.URL, BridgeID: "fffe0000deadbeef", APIKey: "k",
		Lights: []hue.LightSpec{{Name: "BuzzHue1"}},
		FindBridge: func(context.Context, string, time.Duration) (hue.Bridge, bool, error) {
			return hue.Bridge{}, false, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	srv, _ := setupTestHTTPServer(t)
	srv.Lighting = &devProvider{d: d}

	first := make(chan int, 1)
	go func() {
		code, _ := devDo(t, srv, "POST", "/api/lighting/preview", `{"name":"BuzzHue1","on":true}`)
		first <- code
	}()
	select {
	case <-bridge.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("first /preview never reached the bridge")
	}

	code2, out2 := devDo(t, srv, "POST", "/api/lighting/preview", `{"name":"BuzzHue1","on":false}`)
	if code2 != http.StatusTooManyRequests || out2["result"] != "busy" || out2["reason"] != "preview_in_progress" {
		t.Fatalf("second concurrent /preview: want 429 busy/preview_in_progress, got %d %v", code2, out2)
	}
	if n := bridge.hits.Load(); n != 1 {
		t.Fatalf("the refused second call must never have reached the bridge: %d hit(s) recorded", n)
	}

	close(bridge.block)
	select {
	case code := <-first:
		if code != http.StatusOK {
			t.Errorf("first /preview (once released): want 200, got %d", code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("first /preview never returned after release")
	}

	// Released: a further call is now served normally, not refused.
	code3, out3 := devDo(t, srv, "POST", "/api/lighting/preview", `{"name":"BuzzHue1","on":false}`)
	if code3 != http.StatusOK || out3["result"] != "ok" {
		t.Fatalf("retry after release: want 200 ok, got %d %v", code3, out3)
	}
}

// TestPreview_UnreachableBridge_ReturnsProperErrorTaxonomy is the BACKEND
// half of item 4 ("pont injoignable ⇒ erreur propre renvoyée") — the
// FRONTEND half ("la case bascule quand même") is already covered by
// AmbiancePage.preview.test.jsx. Mirrors the dead-bridge pattern already
// used for /register (TestDevLightingRegister, http_lighting_dev_test.go)
// applied to /preview specifically, which had no such test yet — and
// checks that a dead bridge answers "unreachable", never "refused" (§5.6:
// the two taxonomies drive opposite corrective gestures and must never be
// fused into a generic "error").
func TestPreview_UnreachableBridge_ReturnsProperErrorTaxonomy(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	deadURL := dead.URL
	dead.Close() // closed before first use: every request fails to connect

	d, err := hue.New(hue.Config{
		BridgeIP: deadURL, BridgeID: "fffe0000deadbeef", APIKey: "k",
		Lights:  []hue.LightSpec{{Name: "BuzzHue1"}},
		Timeout: 300 * time.Millisecond,
		FindBridge: func(context.Context, string, time.Duration) (hue.Bridge, bool, error) {
			return hue.Bridge{}, false, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	srv, _ := setupTestHTTPServer(t)
	srv.Lighting = &devProvider{d: d}

	code, out := devDo(t, srv, "POST", "/api/lighting/preview", `{"name":"BuzzHue1","on":true}`)
	if code != http.StatusServiceUnavailable || out["result"] != "unreachable" {
		t.Fatalf("dead bridge: want 503 unreachable, got %d %v", code, out)
	}
	if out["result"] == "refused" {
		t.Fatal("a dead bridge must never be reported as 'refused' — opposite corrective gesture (§5.6)")
	}
}
