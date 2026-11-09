// Package diag holds the diagnostic helpers of the WebSocket handlers: the log
// rendering of client-supplied values (a client can never forge a log line,
// #246 review MAJEUR-1) and the ARDOISE_INPUT sequence check (#246).
// Extracted from cmd/server (#291, Phase B).
package diag

import (
	"encoding/json"
	"fmt"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/server"
)

// ArdoiseSeqState tracks the last applied ARDOISE_INPUT SEQ for one connection
// and one question (#246).
type ArdoiseSeqState struct {
	question string
	last     int
}

// CheckArdoiseSeq returns false when the message is stale (SEQ <= last applied,
// same connection and question) and must be ignored; a gap is only logged.
func CheckArdoiseSeq(sq map[string]*ArdoiseSeqState, clientID, bumperID, questionID string, seq int) bool {
	st := sq[clientID]
	if st == nil || st.question != questionID {
		st = &ArdoiseSeqState{question: questionID}
		sq[clientID] = st
	}
	if seq <= st.last {
		server.LogWarn(game.LogComponentApp, "ARDOISE_SEQ_STALE bumper=%s client=%s last=%d got=%d", bumperID, clientID, st.last, seq)
		return false
	}
	if seq != st.last+1 {
		server.LogWarn(game.LogComponentApp, "ARDOISE_SEQ_GAP bumper=%s client=%s expected=%d got=%d", bumperID, clientID, st.last+1, seq)
	}
	st.last = seq
	return true
}

// LogToken renders a client-supplied string for a log line: bare when it is a
// harmless token, quoted (%q, control characters escaped) otherwise, so a
// client can never inject a forged log line (#246 review MAJEUR-1).
func LogToken(s string) string {
	if s == "" {
		return `""`
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '.' || r == ':' || r == '-') {
			return fmt.Sprintf("%q", s)
		}
	}
	return s
}

// LogValue renders a CLIENT_TRACE DETAIL value: scalars through logToken /
// number / bool formatting, anything else re-serialised as quoted JSON.
func LogValue(v interface{}) string {
	switch x := v.(type) {
	case string:
		return LogToken(x)
	case bool, float64, int, int64, nil:
		return fmt.Sprint(x)
	default:
		b, _ := json.Marshal(x)
		return fmt.Sprintf("%q", string(b))
	}
}
