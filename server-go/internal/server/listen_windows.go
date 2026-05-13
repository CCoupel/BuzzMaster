//go:build windows

package server

// newReuseAddrListener creates a TCP listener with SO_REUSEADDR explicitly
// set on the underlying socket.
//
// On Windows, net.Listen does NOT set SO_REUSEADDR by default (unlike Linux).
// Without it, if the previous server process was forcefully terminated (e.g.
// console window closed), the port can remain bound in TIME_WAIT state for up
// to 4 minutes, causing the next launch to fail with WSAEADDRINUSE.
//
// Setting SO_REUSEADDR on Windows allows binding to a port that is in
// TIME_WAIT, so the next server launch succeeds immediately.
//
// golang.org/x/sys is already an indirect dependency of this module, so no
// new dependency is introduced.

import (
	"context"
	"net"
	"syscall"

	"golang.org/x/sys/windows"
)

func newReuseAddrListener(network, addr string) (net.Listener, error) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				// SO_REUSEADDR allows immediate rebind even on a TIME_WAIT socket.
				_ = windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_REUSEADDR, 1)
			})
		},
	}
	return lc.Listen(context.Background(), network, addr)
}
