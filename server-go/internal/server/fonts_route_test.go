package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// ---------------------------------------------------------------------------
// Regression tests for #115 (QA report qa-20260726-142749.md, verdict NOT
// VALIDATED): dev-frontend embedded the VPlayer @font-face files
// (fredoka-latin.woff2, inter-latin.woff2) into the React build — Vite copies
// web/public/ verbatim to the dist root, so they land at dist/fonts/*.woff2,
// and `//go:embed all:dist` (web/embed.go) picks them up like any other dist
// file. But setupRoutes() never registered a route for /fonts/, so
// GET /fonts/*.woff2 404'd in production despite the files being present in
// the embedded FS — same visual symptom as the original bug (system font
// fallback), via a different mechanism (local 404 instead of blocked external
// request).
//
// None of the existing tests caught this because they never exercised the
// real HTTP routing table for this path: frontend tests only checked dist/
// contents, and Go tests never called server.mux.ServeHTTP for /fonts/. These
// tests do — server.mux.ServeHTTP(w, req) runs the exact http.ServeMux
// built by setupRoutes(), so a missing/wrong route registration fails here.
// ---------------------------------------------------------------------------

// TestHTTPServer_FontsRoute_ServesEmbeddedFonts is the primary regression
// test: it mirrors the actual production code path (go:embed → SetEmbeddedFS)
// that QA's curl repro exercised via a real built binary.
func TestHTTPServer_FontsRoute_ServesEmbeddedFonts(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	fontBytes := []byte("fake-woff2-content-for-test")
	fsys := fstest.MapFS{
		"fonts/fredoka-latin.woff2": &fstest.MapFile{Data: fontBytes},
	}
	server.SetEmbeddedFS(fsys)

	req := httptest.NewRequest("GET", "/fonts/fredoka-latin.woff2", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /fonts/fredoka-latin.woff2: expected 200, got %d (route not registered? #115 regression)", w.Code)
	}
	if got := w.Body.String(); got != string(fontBytes) {
		t.Errorf("GET /fonts/fredoka-latin.woff2: body = %q, want %q", got, fontBytes)
	}
	if ct := w.Header().Get("Content-Type"); ct != "font/woff2" {
		t.Errorf("GET /fonts/fredoka-latin.woff2: Content-Type = %q, want %q", ct, "font/woff2")
	}
	cc := w.Header().Get("Cache-Control")
	if cc == "" {
		t.Error("GET /fonts/fredoka-latin.woff2: missing Cache-Control header")
	}
	if strings.Contains(cc, "immutable") {
		t.Errorf("GET /fonts/fredoka-latin.woff2: Cache-Control = %q — a fixed-filename font must not reuse the immutable/1-year cache reserved for hashed /assets/ files (a font swap on redeploy would never be picked up by returning clients)", cc)
	}
}

// TestHTTPServer_FontsRoute_FilesystemFallback covers the same route via the
// filesystem fallback branch (reactDir, no embeddedFS) — the dev/non-portable
// build mode of handleReactAssets.
func TestHTTPServer_FontsRoute_FilesystemFallback(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	reactDir := t.TempDir()
	fontsDir := filepath.Join(reactDir, "fonts")
	if err := os.MkdirAll(fontsDir, 0755); err != nil {
		t.Fatalf("failed to create fonts dir: %v", err)
	}
	fontBytes := []byte("fake-woff2-content-for-test")
	if err := os.WriteFile(filepath.Join(fontsDir, "inter-latin.woff2"), fontBytes, 0644); err != nil {
		t.Fatalf("failed to write fixture font: %v", err)
	}
	server.SetReactDir(reactDir)

	req := httptest.NewRequest("GET", "/fonts/inter-latin.woff2", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /fonts/inter-latin.woff2 (filesystem fallback): expected 200, got %d", w.Code)
	}
	if got := w.Body.String(); got != string(fontBytes) {
		t.Errorf("GET /fonts/inter-latin.woff2: body = %q, want %q", got, fontBytes)
	}
	if ct := w.Header().Get("Content-Type"); ct != "font/woff2" {
		t.Errorf("GET /fonts/inter-latin.woff2: Content-Type = %q, want %q", ct, "font/woff2")
	}
}

// TestHTTPServer_FontsRoute_MissingFile404s guards the flip side: registering
// the route must not turn every /fonts/ request into a 200 — a genuinely
// missing file must still 404.
func TestHTTPServer_FontsRoute_MissingFile404s(t *testing.T) {
	server, _ := setupTestHTTPServer(t)
	server.SetEmbeddedFS(fstest.MapFS{}) // present but empty

	req := httptest.NewRequest("GET", "/fonts/does-not-exist.woff2", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("GET /fonts/does-not-exist.woff2: expected 404, got %d", w.Code)
	}
}

// TestHTTPServer_AssetsRoute_StillImmutableCache is a non-regression guard:
// the #115 fix (branching Cache-Control on the /assets/ prefix) must not
// weaken the long-lived immutable cache for hashed /assets/ bundle files.
func TestHTTPServer_AssetsRoute_StillImmutableCache(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	assetBytes := []byte("console.log(1)")
	fsys := fstest.MapFS{
		"assets/index-abc123.js": &fstest.MapFile{Data: assetBytes},
	}
	server.SetEmbeddedFS(fsys)

	req := httptest.NewRequest("GET", "/assets/index-abc123.js", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /assets/index-abc123.js: expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/javascript" {
		t.Errorf("GET /assets/index-abc123.js: Content-Type = %q, want application/javascript", ct)
	}
	cc := w.Header().Get("Cache-Control")
	if !strings.Contains(cc, "immutable") {
		t.Errorf("GET /assets/index-abc123.js: Cache-Control = %q, want immutable (hashed filename)", cc)
	}
}
