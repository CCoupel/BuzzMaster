package game

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Jeton de session des tablettes (contracts/vplayer-session.md) — tests du moteur : matrice §4.2, cycle de vie §7.

func newSessionEngine(t *testing.T) (*Engine, string) {
	t.Helper()
	e := NewEngine()
	path := filepath.Join(t.TempDir(), "vplayer_sessions.json")
	t.Cleanup(func() { time.Sleep(150 * time.Millisecond) }) // let the asynchronous saves finish before the TempDir is removed
	e.SetVPlayerSessionsPath(path)
	e.SetPhase(PhaseEnroll)
	return e, path
}

func enrol(t *testing.T, e *Engine, name string) (id, token string) {
	t.Helper()
	id, _, reconnected, token, err := e.ReconnectOrCreateVirtualPlayerWithToken("", name, "")
	if err != nil || reconnected || token == "" {
		t.Fatalf("enrôlement %s : id=%q reconnected=%v token=%q err=%v", name, id, reconnected, token, err)
	}
	return id, token
}

func wantInvalidSession(t *testing.T, err error) {
	t.Helper()
	if ee, ok := err.(*EnrollmentError); !ok || ee.Reason != ErrInvalidSession {
		t.Fatalf("INVALID_SESSION attendu, got %v", err)
	}
}

// L'entrée historique (sans jeton) ne vérifie pas la preuve de possession : réservée aux tests et outils.
func TestSessionToken_LegacyEntryPointDoesNotEnforce(t *testing.T) {
	e := NewEngine()
	e.SetPhase(PhaseEnroll)
	id, _, _, err := e.ReconnectOrCreateVirtualPlayer("", "Alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, rec, err := e.ReconnectOrCreateVirtualPlayer(id, "Alice"); err != nil || !rec {
		t.Errorf("l'ancienne entrée accepte la reconnexion par ID seul : %v %v", rec, err)
	}
	if _, _, _, _, err := e.ReconnectOrCreateVirtualPlayerWithToken(id, "Alice", ""); err == nil {
		t.Error("l'entrée avec jeton, elle, refuse l'ID seul")
	}
}

func TestSessionToken_EnrolIssuesTokenAndStoresFingerprintOnly(t *testing.T) {
	e, path := newSessionEngine(t)
	_, tok := enrol(t, e, "Alice")
	raw, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil || len(raw) != 32 || len(tok) != 43 {
		t.Fatalf("jeton : 32 octets base64url sans remplissage attendus, got %q (%v)", tok, err)
	}
	time.Sleep(50 * time.Millisecond) // sauvegarde asynchrone
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("fichier des empreintes absent : %v", err)
	}
	if strings.Contains(string(data), tok) {
		t.Error("le jeton en clair ne doit jamais figurer dans le fichier")
	}
	if !strings.Contains(string(data), tokenFingerprint(tok)) || !strings.Contains(string(data), `"TOKENS"`) {
		t.Errorf("l'empreinte SHA-256 doit figurer sous TOKENS : %s", data)
	}
	if info, _ := os.Stat(path); info != nil && info.Mode().Perm() != 0600 {
		t.Errorf("droits du fichier = %v, attendu 0600", info.Mode().Perm())
	}
}

func TestSessionToken_Matrix_1a_1b(t *testing.T) {
	e, _ := newSessionEngine(t)
	id, tok := enrol(t, e, "Alice")
	otherID, otherTok := enrol(t, e, "Bob")
	_ = otherID

	// 1a : jeton valable → reconnexion, le même jeton est renvoyé.
	if gid, _, rec, got, err := e.ReconnectOrCreateVirtualPlayerWithToken(id, "Alice", tok); err != nil || !rec || gid != id || got != tok {
		t.Fatalf("1a : %v %v %q %v", rec, gid, got, err)
	}
	// 1b : absent, faux, jeton d'une autre place.
	for name, bad := range map[string]string{"absent": "", "faux": "x" + tok[1:], "autre place": otherTok} {
		before := *e.GetBumper(id)
		_, b, rec, got, err := e.ReconnectOrCreateVirtualPlayerWithToken(id, "Alice", bad)
		wantInvalidSession(t, err)
		if b != nil || rec || got != "" {
			t.Errorf("1b (%s) : rien ne doit être renvoyé : %v %v %q", name, b, rec, got)
		}
		if after := *e.GetBumper(id); after.Connected != before.Connected || after.ReclaimRequested != before.ReclaimRequested {
			t.Errorf("1b (%s) : aucun effet d'état attendu", name)
		}
	}
	// Après tous ces refus, le jeton légitime fonctionne toujours (rien n'a tourné).
	if _, _, rec, _, err := e.ReconnectOrCreateVirtualPlayerWithToken(id, "Alice", tok); err != nil || !rec {
		t.Errorf("le jeton légitime doit rester valable : %v", err)
	}
}

func TestSessionToken_Matrix_1c_LegacySeatGetsTokenOnFirstArrival(t *testing.T) {
	e, _ := newSessionEngine(t)
	e.SetBumpers(map[string]*Bumper{"vjoueur_Old_1": {Name: "Old", IsVirtual: true, IsVPlayer: true}}) // sans empreinte
	_, _, rec, tok, err := e.ReconnectOrCreateVirtualPlayerWithToken("vjoueur_Old_1", "Old", "")
	if err != nil || !rec || tok == "" {
		t.Fatalf("1c : reconnexion acceptée et jeton émis attendus : %v %v %q", rec, err, tok)
	}
	// Premier arrivé : le suivant sans jeton est maintenant refusé.
	_, _, _, _, err = e.ReconnectOrCreateVirtualPlayerWithToken("vjoueur_Old_1", "Old", "")
	wantInvalidSession(t, err)
}

func TestSessionToken_ReclaimRekeysAndIssuesNewToken(t *testing.T) {
	e, _ := newSessionEngine(t)
	id, tok := enrol(t, e, "Alice")
	e.UpdateBumper(id, map[string]interface{}{"CONNECTED": false})
	if !e.ReleaseBumperName(id) {
		t.Fatal("libération refusée")
	}
	newID, _, rec, newTok, err := e.ReconnectOrCreateVirtualPlayerWithToken("", "Alice", "")
	if err != nil || !rec || newTok == "" || newTok == tok {
		t.Fatalf("reprise #122 : nouveau jeton attendu : %v %v %q", rec, err, newTok)
	}
	if newID != id {
		// re-clé : l'ancien ID n'existe plus, son empreinte non plus
		e.mu.RLock()
		_, stale := e.sessions.tokens[id]
		e.mu.RUnlock()
		if stale {
			t.Error("l'ancienne empreinte doit être supprimée à la re-clé")
		}
	}
	if _, _, _, _, err := e.ReconnectOrCreateVirtualPlayerWithToken(newID, "Alice", tok); err == nil {
		t.Error("l'ancien jeton ne doit plus ouvrir la nouvelle place")
	}
	if _, _, rec, _, err := e.ReconnectOrCreateVirtualPlayerWithToken(newID, "Alice", newTok); err != nil || !rec {
		t.Errorf("le nouveau jeton doit ouvrir la place : %v", err)
	}
}

func TestSessionToken_ConnectedRelease134_NobodyHoldsTheNewToken(t *testing.T) {
	e, _ := newSessionEngine(t)
	id, tok := enrol(t, e, "Alice")
	newID, wasConnected, ok := e.ReleaseSeat(id)
	if !ok || !wasConnected || newID == "" {
		t.Fatalf("libération d'une place connectée : %q %v %v", newID, wasConnected, ok)
	}
	for name, try := range map[string]string{"ancien jeton": tok, "sans jeton": ""} {
		if _, _, _, _, err := e.ReconnectOrCreateVirtualPlayerWithToken(newID, "Alice", try); err == nil {
			t.Errorf("%s : la place libérée ne doit pas s'ouvrir par ID", name)
		}
	}
	// Le propriétaire reprend par le nom (#122).
	if _, _, rec, tk, err := e.ReconnectOrCreateVirtualPlayerWithToken("", "Alice", ""); err != nil || !rec || tk == "" {
		t.Errorf("reprise par le nom après libération : %v %v %q", rec, err, tk)
	}
}

func TestSessionToken_Lifecycle_DeleteAndNewGamePurge(t *testing.T) {
	e, _ := newSessionEngine(t)
	a, _ := enrol(t, e, "Alice")
	time.Sleep(1100 * time.Millisecond) // ID horodaté à la seconde
	b, _ := enrol(t, e, "Bob")
	e.DeleteBumper(a)
	e.mu.RLock()
	_, hasA := e.sessions.tokens[a]
	_, hasB := e.sessions.tokens[b]
	e.mu.RUnlock()
	if hasA || !hasB {
		t.Errorf("suppression : empreinte de A retirée, celle de B gardée (A=%v B=%v)", hasA, hasB)
	}
	e.InitGameWith(NewGameOptions{PurgePlayers: true})
	e.mu.RLock()
	n := len(e.sessions.tokens)
	e.mu.RUnlock()
	if n != 0 {
		t.Errorf("NEW_GAME avec purge des joueurs : toutes les empreintes retirées, reste %d", n)
	}
}

func TestSessionToken_PersistsAcrossRestartAndRestore(t *testing.T) {
	e, path := newSessionEngine(t)
	id, tok := enrol(t, e, "Alice")
	time.Sleep(100 * time.Millisecond)
	e2 := NewEngine()
	t.Cleanup(func() { time.Sleep(150 * time.Millisecond) })
	e2.SetBumpers(map[string]*Bumper{id: e.GetBumper(id)})
	e2.SetVPlayerSessionsPath(path) // redémarrage
	if _, _, rec, _, err := e2.ReconnectOrCreateVirtualPlayerWithToken(id, "Alice", tok); err != nil || !rec {
		t.Fatalf("après redémarrage, le jeton reste valable : %v", err)
	}
	if _, _, _, _, err := e2.ReconnectOrCreateVirtualPlayerWithToken(id, "Alice", ""); err == nil {
		t.Error("après redémarrage, l'absence de jeton reste refusée")
	}
	// Restauration : rechargement depuis le fichier, élagué sur le roster restauré.
	e2.SetBumpers(map[string]*Bumper{id: {Name: "Alice", IsVirtual: true, IsVPlayer: true}})
	e2.LoadVPlayerSessions()
	e2.mu.RLock()
	_, kept := e2.sessions.tokens[id]
	e2.mu.RUnlock()
	if !kept {
		t.Error("après restauration, la session doit rester valable (empreinte rechargée pour un joueur présent)")
	}
}

// Audit n3 : un fichier absent ou illisible est signalé (l'appelant journalise un WARN) sans jamais exposer son contenu.
func TestSessionToken_LoadReportsMissingOrUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	e := NewEngine()
	if err := e.SetVPlayerSessionsPath(filepath.Join(dir, "absent.json")); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Errorf("fichier absent : os.ErrNotExist attendu (distingué d'un fichier corrompu), got %v", err)
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"TOKENS": {"x": "secret-fingerprint"`), 0600); err != nil {
		t.Fatal(err)
	}
	err := e.SetVPlayerSessionsPath(bad)
	if err == nil {
		t.Fatal("fichier illisible : une erreur est attendue")
	}
	if strings.Contains(err.Error(), "secret-fingerprint") {
		t.Error("l'erreur ne doit jamais contenir le contenu du fichier")
	}
	good := filepath.Join(dir, "good.json")
	if err := os.WriteFile(good, []byte(`{"TOKENS": {}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := e.SetVPlayerSessionsPath(good); err != nil {
		t.Errorf("fichier valable : pas d'erreur attendue, got %v", err)
	}
}

// CI rouge (reset « Joueurs ») : une sauvegarde différée ne doit jamais recréer le fichier d'un store vide.
func TestSessionToken_EmptyStoreHasNoFile_DeferredSaveCannotRecreateIt(t *testing.T) {
	e, path := newSessionEngine(t)
	id, _ := enrol(t, e, "Alice")
	if err := e.SaveVPlayerSessions(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("avec une place, le fichier existe : %v", err)
	}
	e.DeleteBumper(id) // store vide
	if err := e.SaveVPlayerSessions(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("store vide : le fichier doit être supprimé, got %v", err)
	}
	// Réinitialisation : suppression explicite du fichier puis sauvegardes différées en retard.
	_ = os.Remove(path)
	for i := 0; i < 20; i++ {
		go func() { _ = e.SaveVPlayerSessions() }()
	}
	time.Sleep(100 * time.Millisecond)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("une sauvegarde différée a recréé le fichier d'un store vide : %v", err)
	}
}

// Revue de 4541fc0a : la purge NEW_GAME retire les empreintes de N joueurs en UNE seule écriture sous le verrou.
func TestSessionToken_NewGamePurgeWritesOnce(t *testing.T) {
	e, path := newSessionEngine(t)
	e.mu.Lock()
	e.state.VirtualPlayerLimit = 50
	e.mu.Unlock()
	for i := 0; i < 6; i++ {
		enrol(t, e, "Joueur"+string(rune('A'+i)))
		time.Sleep(1100 * time.Millisecond) // l'ID est horodaté à la seconde
	}
	e.mu.Lock()
	before, seats := e.sessions.writes, len(e.sessions.tokens)
	e.mu.Unlock()
	if seats != 6 {
		t.Fatalf("6 places attendues, got %d", seats)
	}
	e.InitGameWith(NewGameOptions{PurgePlayers: true})
	e.mu.RLock()
	after, left := e.sessions.writes, len(e.sessions.tokens)
	e.mu.RUnlock()
	if left != 0 {
		t.Errorf("toutes les empreintes retirées, reste %d", left)
	}
	if after-before != 1 {
		t.Errorf("la purge de 6 joueurs doit coûter UNE écriture, got %d", after-before)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("store vide : pas de fichier, got %v", err)
	}
}
