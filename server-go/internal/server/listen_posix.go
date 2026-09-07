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
	"errors"
	"io/fs"
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

// isPortInUse reports whether err is a genuine "address already in use"
// bind failure (#220). Uses errors.Is against the real syscall errno instead
// of matching on err.Error() text — the previous string-based check silently
// broke on any non-English OS locale that translates the error message.
func isPortInUse(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE)
}

// isPermissionDenied reports whether err is a permission-denied bind failure
// (EACCES — typically a port <1024 without elevated privileges). Deliberately
// NOT folded into isPortInUse: #220 requires it to retry at its own, slower,
// actionable cadence rather than the fast EADDRINUSE backoff.
func isPermissionDenied(err error) bool {
	return errors.Is(err, fs.ErrPermission)
}
