//go:build windows

package server

// newReuseAddrListener creates a TCP listener that can immediately rebind a
// port in TIME_WAIT state — the Windows equivalent of Linux SO_REUSEADDR.
//
// # Why the two-step setsockopt sequence
//
// Go's net.Listen calls setDefaultListenerSockopts which sets
// SO_EXCLUSIVEADDRUSE=1 after socket() but before bind(). This flag prevents
// binding to a port that another socket holds exclusively, even in TIME_WAIT.
//
// The Control callback is invoked after socket() but BEFORE bind(), so it can
// override those options while the socket is still unbound:
//
//  1. Clear SO_EXCLUSIVEADDRUSE (set to 0) — removes Go's exclusive lock.
//  2. Set SO_REUSEADDR (set to 1) — allows rebind on TIME_WAIT sockets.
//
// Without step 1, setting SO_REUSEADDR while SO_EXCLUSIVEADDRUSE=1 is active
// causes WSAEACCES (bind forbidden) even on a completely free port.
//
// SO_EXCLUSIVEADDRUSE is not exported by golang.org/x/sys/windows; its value
// is ((int)(~SO_REUSEADDR)) per ws2def.h — 0xFFFFFFFB = -5 as int32.

import (
	"context"
	"net"
	"syscall"

	"golang.org/x/sys/windows"
)

// soExclusiveAddrUse is SO_EXCLUSIVEADDRUSE from ws2def.h.
// Defined as ((int)(~SO_REUSEADDR)) = ~4 = 0xFFFFFFFB = -5 as int32.
// Not exported by golang.org/x/sys/windows, so we define it locally.
// The literal -5 equals int32(0xFFFFFFFB), the correct Windows bit pattern.
const soExclusiveAddrUse = -5

func newReuseAddrListener(network, addr string) (net.Listener, error) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				h := windows.Handle(fd)
				// Step 1: clear SO_EXCLUSIVEADDRUSE set internally by Go.
				_ = windows.SetsockoptInt(h, windows.SOL_SOCKET, soExclusiveAddrUse, 0)
				// Step 2: allow rebind on TIME_WAIT sockets.
				_ = windows.SetsockoptInt(h, windows.SOL_SOCKET, windows.SO_REUSEADDR, 1)
			})
		},
	}
	return lc.Listen(context.Background(), network, addr)
}
