//go:build windows

package server

// newReuseAddrListener creates a TCP listener using plain net.Listen.
//
// # Why SO_REUSEADDR is NOT set on Windows
//
// On Linux, Go's net.Listen already calls setDefaultListenerSockopts which
// sets SO_REUSEADDR. On Windows that function sets SO_EXCLUSIVEADDRUSE=1
// instead (anti-port-hijacking). Setting SO_REUSEADDR on a socket that also
// has SO_EXCLUSIVEADDRUSE causes WSAEACCES (bind forbidden) even on a
// completely free port — so we must not call setsockopt(SO_REUSEADDR) on
// Windows at all.
//
// Port-busy situations after a forceful process kill are handled by the
// 500 ms retry loop inside HTTPServer.Start(), combined with the graceful
// Shutdown(ctx, 3 s) in Stop() which cleanly releases the listening socket
// without leaving it in TIME_WAIT.

import "net"

func newReuseAddrListener(network, addr string) (net.Listener, error) {
	return net.Listen(network, addr)
}
