package game

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Tests : SaveBumpers persistance atomique (#120 B2)
//
// SaveBumpers écrivait via os.WriteFile, qui tronque la destination en place.
// SaveBumpers est déclenchée en goroutine à chaque inscription/reconnexion
// VJoueur (go e.SaveBumpers()) et prend un RLock partagé : plusieurs sauvegardes
// peuvent se chevaucher, et un lecteur (ou l'autre sauvegarde) pouvait observer
// un fichier vide ou partiel. Correctif : fichier temporaire + os.Rename
// atomique, reprenant à l'identique le pattern déjà validé sur SaveTeams
// (#113 B4, engine.go:2028-2050).
// ---------------------------------------------------------------------------

// setBumpersNoAutoSave sets the in-memory bumper map directly, bypassing
// SetBumpers' own "go e.SaveBumpers()" auto-save side effect (engine.go:490).
// Tests that want to control exactly when/how many SaveBumpers calls happen
// use this instead of SetBumpers, otherwise an unawaited background save
// races the test's own explicit call and its assertions (flaky: the
// directory/file check can run before that background goroutine's rename
// completes).
func setBumpersNoAutoSave(t *testing.T, e *Engine, bumpers map[string]*Bumper) {
	t.Helper()
	e.mu.Lock()
	e.data.Bumpers = bumpers
	e.mu.Unlock()
}

// TestSaveBumpers_WritesViaTempFileThenRename verifies the happy path leaves
// no leftover temp file next to the destination — the write goes through
// os.CreateTemp + os.Rename, not a direct os.WriteFile truncation.
func TestSaveBumpers_WritesViaTempFileThenRename(t *testing.T) {
	e := NewEngine()
	dir := t.TempDir()
	path := filepath.Join(dir, "bumpers.json")
	e.SetBumpersPath(path)
	setBumpersNoAutoSave(t, e, map[string]*Bumper{
		"b1": {Name: "Alice", IsVirtual: true},
	})

	if err := e.SaveBumpers(); err != nil {
		t.Fatalf("SaveBumpers failed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read temp dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "bumpers.json" {
		t.Fatalf("expected exactly bumpers.json in %s, got %v", dir, entries)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("bumpers.json should be readable: %v", err)
	}
	var bumpers map[string]*Bumper
	if err := json.Unmarshal(data, &bumpers); err != nil {
		t.Fatalf("bumpers.json should be valid JSON: %v", err)
	}
	if bumpers["b1"] == nil || bumpers["b1"].Name != "Alice" {
		t.Errorf("expected b1=Alice in saved bumpers, got %v", bumpers)
	}
}

// TestSaveBumpers_ConcurrentSaves_AlwaysReadable is the core B2 regression
// test: N goroutines calling SaveBumpers concurrently must never leave
// bumpers.json empty, truncated, or otherwise invalid — run under -race to
// also confirm no data race on the shared map/file.
func TestSaveBumpers_ConcurrentSaves_AlwaysReadable(t *testing.T) {
	e := NewEngine()
	dir := t.TempDir()
	path := filepath.Join(dir, "bumpers.json")
	e.SetBumpersPath(path)

	const n = 20
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			setBumpersNoAutoSave(t, e, map[string]*Bumper{
				fmt.Sprintf("b%d", i): {Name: fmt.Sprintf("Player%d", i), IsVirtual: true},
			})
			if err := e.SaveBumpers(); err != nil {
				t.Errorf("SaveBumpers failed under concurrency: %v", err)
			}
		}(i)
	}
	wg.Wait()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("bumpers.json should be readable after concurrent saves: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("bumpers.json should never be empty after concurrent saves")
	}
	var bumpers map[string]*Bumper
	if err := json.Unmarshal(data, &bumpers); err != nil {
		t.Fatalf("bumpers.json should always be complete, valid JSON — got parse error: %v (content: %s)", err, data)
	}
	if len(bumpers) != 1 {
		t.Errorf("expected exactly 1 bumper (last writer wins on the in-memory map), got %d: %v", len(bumpers), bumpers)
	}
}

// TestSaveBumpers_WriteFailure_CleansUpTempAndPreservesDestination forces the
// final os.Rename to fail (destination path is an existing directory, not a
// file — rename-onto-directory fails on both Linux and Windows) and verifies:
// the temp file is removed (no leftover .tmp-* next to the destination), an
// error is returned, and the destination itself is left untouched.
func TestSaveBumpers_WriteFailure_CleansUpTempAndPreservesDestination(t *testing.T) {
	e := NewEngine()
	dir := t.TempDir()
	destPath := filepath.Join(dir, "bumpers.json")
	if err := os.Mkdir(destPath, 0755); err != nil {
		t.Fatalf("setup: failed to create directory standing in for the destination: %v", err)
	}
	e.SetBumpersPath(destPath)
	setBumpersNoAutoSave(t, e, map[string]*Bumper{"b1": {Name: "Alice", IsVirtual: true}})

	err := e.SaveBumpers()
	if err == nil {
		t.Fatal("expected SaveBumpers to fail when the destination path is a directory")
	}

	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatalf("failed to read temp dir: %v", readErr)
	}
	for _, entry := range entries {
		if entry.Name() != "bumpers.json" && strings.Contains(entry.Name(), ".tmp-") {
			t.Errorf("leftover temp file after a failed save: %s", entry.Name())
		}
	}

	info, statErr := os.Stat(destPath)
	if statErr != nil {
		t.Fatalf("destination should still exist untouched: %v", statErr)
	}
	if !info.IsDir() {
		t.Error("destination should remain a directory (untouched) after a failed rename — a partial write would have replaced it")
	}
}
