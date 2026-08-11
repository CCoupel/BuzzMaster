package server

import (
	"runtime/debug"

	"buzzcontrol/internal/game"
)

// LogRecoveredPanic logs a panic value already obtained from recover(),
// together with its stack trace and a short context label identifying which
// goroutine/message it came from.
//
// Bugfix #131: every per-connection goroutine (readPump/writePump, for the
// three WebSocket hub types — admin/TV/VPlayer, buzzer, logs) and
// cmd/server's two message-dispatch loops (handleWebMessage,
// handleBuzzerMessage) recover a panic locally instead of letting it
// terminate the whole process. This helper centralizes the log format so
// every recovered panic is grep-able the same way ("recovered panic in
// ..."), across both packages (cmd/server calls this via server.
// LogRecoveredPanic, since it already imports this package).
//
// Deliberately does NOT call recover() itself: per the Go spec, recover()
// only stops a panic when called DIRECTLY inside the deferred function that
// is unwinding — calling it one frame removed, inside a helper the deferred
// function merely invokes, returns nil and lets the panic keep propagating.
// Every call site therefore looks like:
//
//	defer func() {
//	    if r := recover(); r != nil {
//	        server.LogRecoveredPanic(game.LogComponentWebSocket, "readPump client="+c.ID, r)
//	    }
//	    // ... existing cleanup, unchanged ...
//	}()
func LogRecoveredPanic(component game.LogComponent, context string, r interface{}) {
	LogError(component, "recovered panic in %s: %v\n%s", context, r, debug.Stack())
}
