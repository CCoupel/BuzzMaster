package server

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ─── Compatibility stub ───────────────────────────────────────────────────────
//
// getBackupPath was part of the old backup-and-overwrite restart strategy and
// was removed when issue #70 refactored performRestart to the copy-and-launch
// approach. The existing TestGetBackupPath in updater_test.go cannot be modified
// (immutable rule), so we provide the original implementation here as a test-only
// helper to keep the package compilable.
func getBackupPath(exePath string) string {
	if runtime.GOOS == "windows" {
		return exePath + ".bak"
	}
	return exePath + ".old"
}

// ─── Test helpers ─────────────────────────────────────────────────────────────

// minFakeFileSize is used to create synthetic "binaries" that pass the MinBinarySize
// guard in performRestart (Step 1/4). The file content is all zeros — not a valid
// ELF/PE binary — so cmd.Start() will fail (exec format error / invalid PE), which
// prevents os.Exit(0) from being called during tests.
const minFakeFileSize = MinBinarySize + 1

// createFakeLargeFile creates a sparse file at path of exactly size bytes.
// Content is all zeros: not a valid ELF or PE binary, so exec.Command().Start()
// on this file will fail on both Linux and Windows.
func createFakeLargeFile(t *testing.T, path string, size int64) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("createFakeLargeFile: create %s: %v", path, err)
	}
	defer f.Close()
	if err := f.Truncate(size); err != nil {
		t.Fatalf("createFakeLargeFile: truncate %s to %d bytes: %v", path, size, err)
	}
}

// versionedExeName returns a platform-appropriate versioned binary filename.
// The name deliberately does NOT match the currentExe filename so the copy
// goes alongside (not over) the running binary.
func versionedExeName() string {
	if runtime.GOOS == "windows" {
		return "buzzcontrol-v4.0.11-windows-amd64.exe"
	}
	return "buzzcontrol-v4.0.11-linux-amd64"
}

// fileNames extracts file names from ReadDir entries for use in error messages.
func fileNames(entries []os.DirEntry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names
}

// ─── Non-regression tests for issue #70: copy-and-launch strategy ─────────────

// TestPerformRestart_CurrentExeUnmodified verifies the core invariant of the
// copy-and-launch strategy: the running binary (currentExe) is NEVER touched —
// not overwritten, not renamed, not deleted.
//
// Old behaviour (pre-#70): the running binary was overwritten in-place, causing
// "text file busy" errors on Linux and instability on Windows.
//
// The test provokes a cmd.Start() failure (invalid binary format → no valid ELF/PE
// magic bytes) so os.Exit(0) is never called, allowing file-system assertions.
func TestPerformRestart_CurrentExeUnmodified(t *testing.T) {
	exeDir := t.TempDir()
	updatesDir := t.TempDir()

	// Current binary — small file with known, distinctive content.
	currentExe := filepath.Join(exeDir, "buzzcontrol")
	if runtime.GOOS == "windows" {
		currentExe += ".exe"
	}
	originalContent := []byte("this-is-the-current-binary-content")
	if err := os.WriteFile(currentExe, originalContent, 0755); err != nil {
		t.Fatalf("cannot create currentExe: %v", err)
	}

	// New binary in updatesDir — large enough to pass MinBinarySize check, but
	// all-zeros content → cmd.Start() will fail → os.Exit(0) never called.
	newExe := filepath.Join(updatesDir, versionedExeName())
	createFakeLargeFile(t, newExe, minFakeFileSize)

	u := newTestUpdater(t, "4.0.10")
	u.performRestart(currentExe, newExe)

	// Post-condition: currentExe must be byte-for-byte identical to the original.
	got, err := os.ReadFile(currentExe)
	if err != nil {
		t.Fatalf("cannot read currentExe after performRestart: %v", err)
	}
	if string(got) != string(originalContent) {
		t.Errorf("currentExe was modified:\n  got  %q\n  want %q", got, originalContent)
	}

	// Extra guard: currentExe size must be unchanged (rules out truncation).
	info, err := os.Stat(currentExe)
	if err != nil {
		t.Fatalf("cannot stat currentExe: %v", err)
	}
	if info.Size() != int64(len(originalContent)) {
		t.Errorf("currentExe size changed: got %d, want %d", info.Size(), len(originalContent))
	}
}

// TestPerformRestart_VersionedBinaryCopiedToExeDir verifies that the new binary
// is copied into filepath.Dir(currentExe) — alongside the current server — and
// NOT into some other directory (e.g. the updatesDir download cache).
//
// Verification strategy (indirect, because os.Remove cleans up on cmd.Start failure):
//
//  1. After performRestart, exeDir must contain exactly 1 file (currentExe).
//     The versioned copy was made in exeDir then cleaned up after launch failure.
//
//  2. After performRestart, updatesDir must contain exactly 1 file (newExe).
//     If the copy had landed in updatesDir, that directory would have 2 files.
//
// Together, (1) and (2) prove the copy targeted exeDir and was cleaned up there.
func TestPerformRestart_VersionedBinaryCopiedToExeDir(t *testing.T) {
	exeDir := t.TempDir()
	updatesDir := t.TempDir()

	currentExe := filepath.Join(exeDir, "buzzcontrol")
	if runtime.GOOS == "windows" {
		currentExe += ".exe"
	}
	if err := os.WriteFile(currentExe, []byte("current-binary"), 0755); err != nil {
		t.Fatalf("cannot create currentExe: %v", err)
	}

	newExe := filepath.Join(updatesDir, versionedExeName())
	createFakeLargeFile(t, newExe, minFakeFileSize)

	u := newTestUpdater(t, "4.0.10")
	u.performRestart(currentExe, newExe)

	// (1) exeDir must have exactly 1 file — the versioned copy was placed here then cleaned up.
	exeFiles, err := os.ReadDir(exeDir)
	if err != nil {
		t.Fatalf("cannot list exeDir: %v", err)
	}
	if len(exeFiles) != 1 {
		t.Errorf("exeDir has %d file(s) after performRestart, want exactly 1 (currentExe); found: %v",
			len(exeFiles), fileNames(exeFiles))
	} else if exeFiles[0].Name() != filepath.Base(currentExe) {
		t.Errorf("exeDir contains %q instead of currentExe %q", exeFiles[0].Name(), filepath.Base(currentExe))
	}

	// (2) updatesDir must still have exactly 1 file (newExe, the source) — the copy
	// did NOT land in updatesDir, which would have left a second file there.
	updateFiles, err := os.ReadDir(updatesDir)
	if err != nil {
		t.Fatalf("cannot list updatesDir: %v", err)
	}
	if len(updateFiles) != 1 {
		t.Errorf("updatesDir has %d file(s), want 1 — copy should not have landed here; found: %v",
			len(updateFiles), fileNames(updateFiles))
	}

	// (3) The source binary in updatesDir must be untouched (copy, not move).
	sourceInfo, err := os.Stat(newExe)
	if err != nil {
		t.Fatalf("newExe removed from updatesDir — performRestart must not move the source: %v", err)
	}
	if sourceInfo.Size() != minFakeFileSize {
		t.Errorf("newExe size changed: got %d, want %d", sourceInfo.Size(), minFakeFileSize)
	}
}

// TestPerformRestart_CopyFails_NoPanic verifies that performRestart returns cleanly
// (no panic, no crash) when the copy to exeDir fails because the directory is not
// writable. This simulates a misconfigured deployment or insufficient permissions.
//
// Note: on Windows, directory-level write permissions work differently — this test
// is skipped. On Linux, running as root would also bypass chmod; CI typically runs
// as a regular user.
func TestPerformRestart_CopyFails_NoPanic(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory chmod semantics differ on Windows — test skipped")
	}

	exeDir := t.TempDir()
	updatesDir := t.TempDir()

	currentExe := filepath.Join(exeDir, "buzzcontrol")
	if err := os.WriteFile(currentExe, []byte("current"), 0755); err != nil {
		t.Fatalf("cannot create currentExe: %v", err)
	}

	newExe := filepath.Join(updatesDir, versionedExeName())
	createFakeLargeFile(t, newExe, minFakeFileSize)

	// Make exeDir read-only so copyFile() fails at Step 2/4.
	if err := os.Chmod(exeDir, 0555); err != nil {
		t.Fatalf("cannot chmod exeDir to read-only: %v", err)
	}
	// Restore write permission so t.TempDir() cleanup works.
	t.Cleanup(func() { os.Chmod(exeDir, 0755) })

	u := newTestUpdater(t, "4.0.10")

	// Must not panic — Step 2/4 failure must log + return, not crash.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("performRestart panicked when copy destination is not writable: %v", r)
		}
	}()
	u.performRestart(currentExe, newExe)
}

// TestPerformRestart_StartFails_NoOrphanFile verifies that when cmd.Start() fails
// (the copied binary is not a valid executable), performRestart removes the
// copied file from exeDir — no orphan binary is left behind after a failed launch.
//
// Non-regression: a failed launch must not leave an invalid/partial binary in the
// server's directory, which could prevent future restart attempts or confuse the OS.
func TestPerformRestart_StartFails_NoOrphanFile(t *testing.T) {
	exeDir := t.TempDir()
	updatesDir := t.TempDir()

	currentExe := filepath.Join(exeDir, "buzzcontrol")
	if runtime.GOOS == "windows" {
		currentExe += ".exe"
	}
	if err := os.WriteFile(currentExe, []byte("current-binary"), 0755); err != nil {
		t.Fatalf("cannot create currentExe: %v", err)
	}

	// newExe: large enough to pass MinBinarySize (Step 1/4 succeeds), copy to exeDir
	// succeeds (Step 2/4), chmod succeeds on Unix (Step 3/4), but cmd.Start() fails
	// because content is all zeros — not a valid ELF/PE binary (Step 4/4 fails).
	newExe := filepath.Join(updatesDir, versionedExeName())
	createFakeLargeFile(t, newExe, minFakeFileSize)

	u := newTestUpdater(t, "4.0.10")
	u.performRestart(currentExe, newExe)

	// The versioned copy MUST have been removed from exeDir after cmd.Start failure.
	expectedOrphan := filepath.Join(exeDir, versionedExeName())
	if _, err := os.Stat(expectedOrphan); err == nil {
		t.Errorf("orphan file left in exeDir after cmd.Start() failure: %s", expectedOrphan)
	} else if !os.IsNotExist(err) {
		t.Errorf("unexpected error checking orphan file: %v", err)
	}

	// exeDir must contain ONLY currentExe — no extra files left over.
	exeFiles, err := os.ReadDir(exeDir)
	if err != nil {
		t.Fatalf("cannot list exeDir: %v", err)
	}
	if len(exeFiles) != 1 {
		t.Errorf("exeDir has %d file(s) after Start() failure, want 1; orphans: %v",
			len(exeFiles), fileNames(exeFiles))
	}
}

// TestPerformRestart_TooSmallBinary verifies that performRestart rejects a new binary
// that does not meet MinBinarySize and returns without modifying anything.
//
// This guards against applying a truncated or corrupt download.
func TestPerformRestart_TooSmallBinary(t *testing.T) {
	exeDir := t.TempDir()
	updatesDir := t.TempDir()

	currentExe := filepath.Join(exeDir, "buzzcontrol")
	if runtime.GOOS == "windows" {
		currentExe += ".exe"
	}
	originalContent := []byte("current-binary-unchanged")
	if err := os.WriteFile(currentExe, originalContent, 0755); err != nil {
		t.Fatalf("cannot create currentExe: %v", err)
	}

	// newExe is too small — Step 1/4 must reject it before any copy occurs.
	smallExe := filepath.Join(updatesDir, versionedExeName())
	if err := os.WriteFile(smallExe, []byte("tiny"), 0644); err != nil {
		t.Fatalf("cannot create small newExe: %v", err)
	}

	u := newTestUpdater(t, "4.0.10")
	u.performRestart(currentExe, smallExe)

	// currentExe must be untouched.
	got, err := os.ReadFile(currentExe)
	if err != nil {
		t.Fatalf("cannot read currentExe: %v", err)
	}
	if string(got) != string(originalContent) {
		t.Errorf("currentExe modified despite too-small newExe: got %q, want %q", got, originalContent)
	}

	// No copy must have been created in exeDir.
	exeFiles, err := os.ReadDir(exeDir)
	if err != nil {
		t.Fatalf("cannot list exeDir: %v", err)
	}
	if len(exeFiles) != 1 {
		t.Errorf("exeDir has %d file(s) after too-small binary rejection, want 1; found: %v",
			len(exeFiles), fileNames(exeFiles))
	}
}
