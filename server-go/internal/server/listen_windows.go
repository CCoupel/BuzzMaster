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
	"errors"
	"io/fs"
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

// isPortInUse reports whether err is a genuine "address already in use" bind
// failure (#220). Checks the real WSA error code (WSAEADDRINUSE) via
// errors.Is instead of matching on err.Error() text — Go's raw Windows
// syscall errors are NOT translated to the generic syscall.EADDRINUSE the
// posix build uses (that constant is an invented value for package os, never
// actually returned by a Winsock bind() failure), and the previous
// string-based check additionally broke on any non-English Windows locale.
func isPortInUse(err error) bool {
	return errors.Is(err, windows.WSAEADDRINUSE)
}

// isPermissionDenied reports whether err is a permission-denied bind failure
// (e.g. a URL ACL reservation or a restricted low port). Checked two ways:
// fs.ErrPermission catches the generic ERROR_ACCESS_DENIED case (which Go's
// syscall.Errno.Is maps to it on Windows too), and WSAEACCES catches a raw
// Winsock-level access-denied that generic check does not cover. Deliberately
// NOT folded into isPortInUse: #220 requires it to retry at its own, slower,
// actionable cadence rather than the fast EADDRINUSE backoff.
func isPermissionDenied(err error) bool {
	return errors.Is(err, fs.ErrPermission) || errors.Is(err, windows.WSAEACCES)
}
