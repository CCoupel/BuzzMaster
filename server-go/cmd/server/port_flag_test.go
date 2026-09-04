package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolvePort verifies that the --port CLI flag takes precedence over config.json.
func TestResolvePort(t *testing.T) {
	tests := []struct {
		name       string
		configPort int
		flagPort   int
		wantPort   int
	}{
		{
			name:       "flag overrides config",
			configPort: 8080,
			flagPort:   9090,
			wantPort:   9090,
		},
		{
			name:       "no flag — config port used",
			configPort: 8080,
			flagPort:   0,
			wantPort:   8080,
		},
		{
			name:       "flag=0 treated as unset — config port used",
			configPort: 3000,
			flagPort:   0,
			wantPort:   3000,
		},
		{
			name:       "flag and config identical — no change",
			configPort: 8080,
			flagPort:   8080,
			wantPort:   8080,
		},
		{
			name:       "flag=1 (minimal valid port) overrides config",
			configPort: 8080,
			flagPort:   1,
			wantPort:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePort(tt.configPort, tt.flagPort)
			if got != tt.wantPort {
				t.Errorf("resolvePort(%d, %d) = %d, want %d",
					tt.configPort, tt.flagPort, got, tt.wantPort)
			}
		})
	}
}

// TestResolvePortSource verifies the #220 provenance string used in
// HTTPServer's bind-wait log messages ("le message d'attente nomme le port
// résolu et sa provenance"): --port flag takes precedence, then an explicit
// config.json http_port, then falls back to describing the code default.
func TestResolvePortSource(t *testing.T) {
	writeConfig := func(t *testing.T, body string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("could not write test config.json: %v", err)
		}
		return path
	}

	t.Run("--port flag wins even with an explicit config.json value", func(t *testing.T) {
		path := writeConfig(t, `{"server":{"http_port":8080}}`)
		got := resolvePortSource(path, 9090, 9090)
		want := "--port flag (9090)"
		if got != want {
			t.Errorf("resolvePortSource() = %q, want %q", got, want)
		}
	})

	t.Run("explicit config.json http_port, no flag", func(t *testing.T) {
		path := writeConfig(t, `{"server":{"http_port":8080}}`)
		got := resolvePortSource(path, 0, 8080)
		want := "config.json (8080)"
		if got != want {
			t.Errorf("resolvePortSource() = %q, want %q", got, want)
		}
	})

	t.Run("config.json exists but omits http_port — falls back to code default", func(t *testing.T) {
		path := writeConfig(t, `{"server":{"websocket_path":"/ws"}}`)
		got := resolvePortSource(path, 0, 80)
		want := "code default (80)"
		if got != want {
			t.Errorf("resolvePortSource() = %q, want %q", got, want)
		}
	})

	t.Run("config.json missing entirely — falls back to code default", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "does-not-exist.json")
		got := resolvePortSource(missing, 0, 80)
		want := "code default (80)"
		if got != want {
			t.Errorf("resolvePortSource() = %q, want %q", got, want)
		}
	})

	t.Run("config.json is malformed — falls back to code default rather than erroring", func(t *testing.T) {
		path := writeConfig(t, `{not valid json`)
		got := resolvePortSource(path, 0, 80)
		want := "code default (80)"
		if got != want {
			t.Errorf("resolvePortSource() = %q, want %q", got, want)
		}
	})
}
