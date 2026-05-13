//go:build windows

package server

// newReuseAddrListener creates a TCP listener with SO_REUSEADDR set.
//
// On Windows, net.Listen does NOT set SO_REUSEADDR by default (unlike Linux
// where Go's setDefaultListenerSockopts does it). Without it, after a
// forceful process kill, the port may stay in TIME_WAIT and the next launch
// fails with WSAEADDRINUSE.
//
// Setting SO_REUSEADDR allows immediate rebind on a TIME_WAIT socket so the
// server restarts without waiting for the OS to release the port.
//
// Note: Go 1.24 does not set SO_EXCLUSIVEADDRUSE on Windows
// (setDefaultListenerSockopts is a no-op there), so SO_REUSEADDR can be set
// directly without any prior cleanup.

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
				_ = windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_REUSEADDR, 1)
			})
		},
	}
	return lc.Listen(context.Background(), network, addr)
}
