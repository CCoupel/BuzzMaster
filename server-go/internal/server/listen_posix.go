//go:build !windows

package server

// newReuseAddrListener creates a TCP listener with SO_REUSEADDR explicitly
// set on the underlying socket.
//
// On Linux/macOS, net.Listen already calls setDefaultListenerSockopts which
// sets SO_REUSEADDR, but we set it explicitly here as belt-and-suspenders so
// the server can immediately rebind even if a previous socket is still in
// TIME_WAIT (e.g. after a forceful kill of the previous process).

import (
	"context"
	"net"
	"syscall"
)

func newReuseAddrListener(network, addr string) (net.Listener, error) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				// SO_REUSEADDR allows immediate rebind after a process crash.
				_ = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			})
		},
	}
	return lc.Listen(context.Background(), network, addr)
}
