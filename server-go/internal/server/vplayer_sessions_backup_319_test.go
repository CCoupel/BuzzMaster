package server

// #319 (v13.0.0, LOT-2.2) — sauvegarde sélective, réinitialisation et restauration « Joueurs » : vplayer_sessions.json
// voyage avec bumpers.json (contracts/vplayer-session.md §7, §9 : « la session reste valable après une restauration »).
// Mêmes drapeaux que les joueurs (`bumpers=true`) dans handleBackupSelect, handleResetSelect, detectTARContents et
// handleRestore. Modèle : gamestate_backup_test.go.

import (
	"archive/tar"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"buzzcontrol/internal/game"
)

type sessBackup319 struct {
	srv       *HTTPServer
	cfgDir    string
	id, token string
}

func newSessBackup319(t *testing.T) *sessBackup319 {
	t.Helper()
	srv, dataDir := setupTestHTTPServer(t)
	cfg := filepath.Join(dataDir, "config")
	os.MkdirAll(cfg, 0o755)
	srv.engine.SetBumpersPath(filepath.Join(cfg, "bumpers.json"))
	srv.engine.SetVPlayerSessionsPath(filepath.Join(cfg, "vplayer_sessions.json"))
	srv.engine.SetPhase(game.PhaseEnroll)
	id, _, _, token, err := srv.engine.ReconnectOrCreateVirtualPlayerWithToken("", "Alice", "")
	if err != nil || token == "" {
		t.Fatalf("enrôlement : %v (jeton %q)", err, token)
	}
	if err := srv.engine.SaveBumpers(); err != nil {
		t.Fatal(err)
	}
	if err := srv.engine.SaveVPlayerSessions(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { time.Sleep(300 * time.Millisecond) }) // sauvegardes asynchrones du moteur
	return &sessBackup319{srv: srv, cfgDir: cfg, id: id, token: token}
}

func (b *sessBackup319) get(t *testing.T, url string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	b.srv.mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, url, nil))
	return w
}

func (b *sessBackup319) tarFile(t *testing.T, data []byte, name string) (string, bool) {
	t.Helper()
	tr := tar.NewReader(bytes.NewReader(data))
	for {
		h, err := tr.Next()
		if err != nil {
			return "", false
		}
		if h.Name == name {
			c, _ := io.ReadAll(tr)
			return string(c), true
		}
	}
}

// Sauvegarde : le drapeau « Joueurs » emporte les empreintes ; une sauvegarde d'équipes seules, non.
func TestVPlayerSessionsBackup319_BackupSelect_FollowsThePlayersFlag(t *testing.T) {
	b := newSessBackup319(t)

	w := b.get(t, "/backup-select?bumpers=true")
	if w.Code != http.StatusOK {
		t.Fatalf("200 attendu, got %d", w.Code)
	}
	content, ok := b.tarFile(t, w.Body.Bytes(), "config/vplayer_sessions.json")
	if !ok {
		t.Fatalf("la sauvegarde « Joueurs » doit contenir config/vplayer_sessions.json")
	}
	if strings.Contains(content, b.token) {
		t.Errorf("l'archive ne doit contenir que l'empreinte, jamais le jeton en clair")
	}
	if !tarContains(t, w.Body.Bytes(), "config/bumpers.json") {
		t.Errorf("précondition : bumpers.json aussi")
	}

	w = b.get(t, "/backup-select?teams=true")
	if tarContains(t, w.Body.Bytes(), "config/vplayer_sessions.json") {
		t.Errorf("une sauvegarde sans le drapeau joueurs ne doit pas contenir les empreintes")
	}
}

// Réinitialisation « Joueurs » : les empreintes partent avec les joueurs (fichier et mémoire).
func TestVPlayerSessionsBackup319_ResetSelect_RemovesFingerprintsWithThePlayers(t *testing.T) {
	b := newSessBackup319(t)
	w := httptest.NewRecorder()
	b.srv.mux.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/reset-select?bumpers=true", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("200 attendu, got %d: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(b.cfgDir, "vplayer_sessions.json")); err == nil {
		t.Errorf("vplayer_sessions.json doit être supprimé avec les joueurs")
	}
	// Plus aucune place : l'ancien ID n'ouvre rien (nouvel enrôlement, nouveau jeton).
	_, _, _, tok, err := b.srv.engine.ReconnectOrCreateVirtualPlayerWithToken(b.id, "Alice", b.token)
	if err != nil || tok == b.token {
		t.Errorf("après réinitialisation, l'ancien jeton ne ressert pas (nouvelle place) : (%q, %v)", tok, err)
	}
}

// Restauration : la session reste valable (1a) ; un client sans jeton reste refusé (1b).
func TestVPlayerSessionsBackup319_Restore_SessionStaysValid(t *testing.T) {
	b := newSessBackup319(t)

	archive := b.get(t, "/backup-select?bumpers=true").Body.Bytes()

	// On efface les joueurs (état + fichiers), comme après un incident.
	b.srv.engine.SetBumpers(make(map[string]*game.Bumper))
	os.Remove(filepath.Join(b.cfgDir, "bumpers.json"))
	os.Remove(filepath.Join(b.cfgDir, "vplayer_sessions.json"))

	body, contentType := multipartRestoreBody(t, archive)
	req := httptest.NewRequest(http.MethodPost, "/restore", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	b.srv.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("restauration : 200 attendu, got %d: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(b.cfgDir, "vplayer_sessions.json")); err != nil {
		t.Fatalf("vplayer_sessions.json doit être restauré avec les joueurs : %v", err)
	}

	id, _, reconnected, tok, err := b.srv.engine.ReconnectOrCreateVirtualPlayerWithToken(b.id, "Alice", b.token)
	if err != nil || !reconnected || id != b.id || tok != b.token {
		t.Fatalf("après restauration, le jeton doit rester valable (1a) : (%q, reconnected=%v, tok égal=%v, err=%v)", id, reconnected, tok == b.token, err)
	}
	if _, _, _, _, err := b.srv.engine.ReconnectOrCreateVirtualPlayerWithToken(b.id, "Alice", ""); err == nil {
		t.Errorf("après restauration, une place protégée refuse toujours un client sans jeton (1b)")
	}
}

// La détection d'archive : une archive qui ne contient QUE vplayer_sessions.json compte comme « Joueurs ».
func TestVPlayerSessionsBackup319_DetectTARContents_SessionsFileIsPlayers(t *testing.T) {
	b := newSessBackup319(t)
	tarData := buildTARWithSingleFile(t, "config/vplayer_sessions.json", []byte(`{"TOKENS":{}}`))
	body, contentType := multipartRestoreBody(t, tarData)
	req := httptest.NewRequest(http.MethodPost, "/restore", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	b.srv.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("200 attendu, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "bumpers") {
		t.Errorf("une archive contenant vplayer_sessions.json doit être détectée comme « Joueurs » (bumpers) : %s", w.Body.String())
	}
}
