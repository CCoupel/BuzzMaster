package game

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// VJoueur session tokens (v13.0.0, contracts/vplayer-session.md): the bumper ID is a public identifier, the
// secret proving possession of a seat is a random token handed to the tablet at enrolment. Only the SHA-256
// FINGERPRINT is kept, in its own file (data/config/vplayer_sessions.json) — never in Bumper, so no serializer
// can leak it — and the plaintext never reaches a log, a GameState, an UPDATE or a file.
//
// The store is always active, in memory; SetVPlayerSessionsPath adds the persistence (the application layer
// calls it at startup). The legacy ReconnectOrCreateVirtualPlayer (no token argument) does not verify the proof
// of possession — it is kept for tests and tools; every real connection goes through
// ReconnectOrCreateVirtualPlayerWithToken.

// vplayerSessionsFile is the on-disk shape: {"TOKENS": {"<bumperID>": "<sha256 hex>"}}.
type vplayerSessionsFile struct {
	Tokens map[string]string `json:"TOKENS"`
}

// vplayerSessionStore maps a bumper ID to the fingerprint of its token. Guarded by the engine lock.
type vplayerSessionStore struct {
	path   string
	tokens map[string]string
}

// ErrInvalidSession is the reason of the PLAYER_REJECTED sent when a seat exists, has a token, and the
// connecting tablet does not present it (matrix case 1b).
const ErrInvalidSession = "INVALID_SESSION"

func tokenFingerprint(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// newSessionToken returns 32 random bytes from crypto/rand, base64url without padding (43 characters).
func newSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// verify compares, in constant time, the fingerprint of the presented token with the stored one.
// has=false: no fingerprint for this seat (created before v13.0.0). ok: the token matches.
func (s *vplayerSessionStore) verify(id, token string) (has, ok bool) {
	fp, exists := s.tokens[id]
	if !exists {
		return false, false
	}
	got := tokenFingerprint(token)
	return true, subtle.ConstantTimeCompare([]byte(got), []byte(fp)) == 1
}

// SetVPlayerSessionsPath sets the persistence file and loads the fingerprints from it. A missing or unreadable
// file gives an empty store AND a non-nil error (the caller journals a WARN: every seat is back to "first to
// arrive", matrix case 1c). Never carries a token or a fingerprint.
func (e *Engine) SetVPlayerSessionsPath(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.sessions == nil {
		e.sessions = &vplayerSessionStore{tokens: map[string]string{}}
	}
	e.sessions.path = path
	return e.loadVPlayerSessionsUnsafe()
}

// LoadVPlayerSessions reloads the fingerprints from disk (restoration of a backup).
func (e *Engine) LoadVPlayerSessions() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.sessions == nil {
		return nil
	}
	err := e.loadVPlayerSessionsUnsafe()
	e.sessionPruneUnsafe() // the restored roster is the reference: no fingerprint for a seat it does not hold
	return err
}

// loadVPlayerSessionsUnsafe fills the store from the file; the error (missing or unreadable file) never
// contains the file's content.
func (e *Engine) loadVPlayerSessionsUnsafe() error {
	s := e.sessions
	s.tokens = map[string]string{}
	if s.path == "" {
		return nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	var f vplayerSessionsFile
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("vplayer_sessions.json illisible: %w", err)
	}
	for id, fp := range f.Tokens {
		s.tokens[id] = fp
	}
	log.Printf("[Engine] vplayer sessions loaded: %d seat(s)", len(s.tokens))
	return nil
}

// SaveVPlayerSessions writes the fingerprints atomically (temp file + rename).
func (e *Engine) SaveVPlayerSessions() error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := e.sessions
	if s == nil || s.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(vplayerSessionsFile{Tokens: s.tokens}, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(s.path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, 0600); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

// VPlayerSeatsWithoutFingerprint counts the VJoueur seats that hold no token fingerprint: the seats of the
// "first to arrive" window (matrix case 1c). Used for the WARN journal of contracts/vplayer-session.md §6.
func (e *Engine) VPlayerSeatsWithoutFingerprint() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	n := 0
	for id, b := range e.data.Bumpers {
		if b == nil || !b.IsVirtual {
			continue
		}
		if e.sessions == nil {
			n++
		} else if _, ok := e.sessions.tokens[id]; !ok {
			n++
		}
	}
	return n
}

// sessionIssueUnsafe generates a token for the seat, stores its fingerprint (replacing any previous one) and
// returns the plaintext, to be handed to the bound tablet only. "" when the feature is off.
func (e *Engine) sessionIssueUnsafe(id string) string {
	if e.sessions == nil {
		return ""
	}
	tok, err := newSessionToken()
	if err != nil {
		log.Printf("[Engine] VPLAYER_TOKEN_ISSUE_FAILED id=%s: %v", id, err)
		return ""
	}
	e.sessions.tokens[id] = tokenFingerprint(tok)
	safeGo("SaveVPlayerSessions", e.SaveVPlayerSessions)
	return tok
}

// sessionRemoveUnsafe forgets the fingerprint of a seat that no longer exists under this ID.
func (e *Engine) sessionRemoveUnsafe(id string) {
	if e.sessions == nil {
		return
	}
	if _, ok := e.sessions.tokens[id]; ok {
		delete(e.sessions.tokens, id)
		safeGo("SaveVPlayerSessions", e.SaveVPlayerSessions)
	}
}

// sessionPruneUnsafe drops the fingerprints of every ID that is no longer a bumper (deletion, NEW_GAME purge,
// reset, restoration of another roster).
func (e *Engine) sessionPruneUnsafe() {
	if e.sessions == nil {
		return
	}
	changed := false
	for id := range e.sessions.tokens {
		if _, ok := e.data.Bumpers[id]; !ok {
			delete(e.sessions.tokens, id)
			changed = true
		}
	}
	if changed {
		safeGo("SaveVPlayerSessions", e.SaveVPlayerSessions)
	}
}
