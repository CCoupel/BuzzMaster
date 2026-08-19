package main

import (
	"buzzcontrol/internal/server"
	"testing"
)

// ---------------------------------------------------------------------------
// Tests: #175 B1 — wiring OnShutdown to a.stop (plan tâche T5,
// _work/reports/plan-20260818-140953.md §"Ce que fait réellement /shutdown").
//
// Before B1, http.go declared OnShutdown (a callback field) and called it
// (handleShutdown, 100ms before os.Exit(0)) but nothing in the repo ever
// ASSIGNED it — /shutdown always went straight to a bare os.Exit(0): no
// a.cancelCtx(), no dnsServer/mdnsServer/broadcaster/udpBcast.Stop(), and
// critically no a.httpServer.Stop() — suspected root cause of the QUALIF
// v6.2.0.35 port-release issue (had to reboot the machine). #175 turns
// /shutdown from an occasional dev `curl` call into the everyday
// end-of-session gesture, so this latent gap needed closing.
//
// This test ONLY verifies the ASSIGNMENT happens — it deliberately never
// INVOKES the resulting OnShutdown()/a.stop(): a.stop() unconditionally
// dereferences a.dnsServer/a.mdnsServer/a.broadcaster/a.udpBcast
// (main.go:920-935), none of which a minimal test App populates (same
// reasoning newTestApp/newTestAppWithHub already apply to every other test
// in this package — setupCallbacks itself is never exercised end-to-end
// elsewhere, only its individual callback wiring is spot-checked).
// ---------------------------------------------------------------------------

// TestSetupCallbacks_WiresOnShutdownToStop is T5: after setupCallbacks runs
// (the real production wiring path, called once from main()), the HTTP
// server's OnShutdown hook must be assigned — not left nil, which is exactly
// the pre-#175-B1 state (declared and called by handleShutdown, but never
// set anywhere).
func TestSetupCallbacks_WiresOnShutdownToStop(t *testing.T) {
	app := newTestAppWithHub(t)
	// setupCallbacks assigns a.wsHub.OnPlayerDisconnected/... and
	// a.httpServer.OnLoadDemo/OnConfigUpdate/OnBuzzerWifiConfig/
	// OnPriorityMessageSent/OnShutdown — all of these are plain field
	// assignments (closures created, not called), so a.httpServer only
	// needs to be a real, non-nil *server.HTTPServer for the assignments
	// themselves not to panic on a nil dereference.
	app.httpServer = server.NewHTTPServer(0, app.engine, app.wsHub, app.buzzerHub, server.NewLogsWebSocketHub(10))

	app.setupCallbacks()

	if app.httpServer.OnShutdown == nil {
		t.Fatal("#175 B1: setupCallbacks must assign a.httpServer.OnShutdown — without it, /shutdown falls straight through to os.Exit(0) with no cleanup (leaked AckManager goroutine, ARDOISE coalescer timer, and critically no httpServer.Stop() — the listening port may not be released)")
	}
}
