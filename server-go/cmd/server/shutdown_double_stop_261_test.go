package main

import (
	"net"
	"testing"
	"time"

	"buzzcontrol/internal/server"
)

// #261 — chemin réel de l'arrêt : /shutdown déclenche a.stop() (httpServer.OnShutdown),
// puis main() rappelle app.stop() une fois appCtx annulé. Le second appel
// paniquait (« close of closed channel », broadcaster.go). Le test vérifie le
// comportement — deux (*App).stop() consécutifs ne paniquent pas — pas le
// mécanisme du correctif.
func TestAppStop_CalledTwice_NoPanic_261(t *testing.T) {
	app := newTestApp(t)
	app.udpBcast = server.NewUDPBroadcaster()
	app.broadcaster = server.NewBroadcasterManager(app.udpBcast, 80)
	app.dnsServer = server.NewDNSServer(0, net.IPv4(127, 0, 0, 1))
	app.mdnsServer = server.NewMDNSServer("buzz-261", 0, 0)
	app.httpServer = server.NewHTTPServer(0, app.engine, server.NewWebSocketHub(), app.buzzerHub, server.NewLogsWebSocketHub(10))
	app.broadcaster.Start()

	done := make(chan interface{}, 1)
	go func() {
		defer func() { done <- recover() }()
		app.stop() // 1er appel : hook OnShutdown de /shutdown
		app.stop() // 2e appel : fin de main()
	}()
	select {
	case r := <-done:
		if r != nil {
			t.Fatalf("double (*App).stop() a paniqué : %v", r)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("double (*App).stop() ne rend pas la main (échéance 30 s)")
	}
}
