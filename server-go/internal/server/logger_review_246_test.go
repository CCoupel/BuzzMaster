package server

import (
	"buzzcontrol/internal/game"
	"strings"
	"testing"
)

// #246 review MAJEUR-1: an entry with embedded line breaks is ONE line in the log file.
func TestFileLogging246_EmbeddedNewlinesNeverSplitALine(t *testing.T) {
	bl, path := newFileLogger246(t, `{"file_path":"@PATH@"}`)
	bl.Warn(game.LogComponentApp, "a\n[ERROR] [App] forged\r\nb")

	lines := strings.Split(strings.TrimRight(readLog246(t, path), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("une entrée = une ligne, obtenu %d : %q", len(lines), lines)
	}
}
