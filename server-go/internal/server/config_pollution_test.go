package server

// Regression test for #143 (plan _work/reports/plan-20260811-105859.md §7 task 4,
// §1.1 security finding).
//
// Root cause (confirmed by the planner): config.Get()/config.Save()
// (internal/config/config.go:348,386) hard-code a path relative to the
// process's current working directory ("config.json"). `go test` runs each
// package with CWD == that package's directory, so any test in
// internal/server that exercises handleConfig's POST path
// (TestHTTPServer_Config_POST*, http_test.go:342,396,452) writes straight
// into the tracked fixture internal/server/config.json — including the fake
// API keys those tests plant to verify masking/preservation behaviour
// (sk-ant-original, sk-ant-new-value, gsk_original, gsk_new-value, see
// TestHTTPServer_Config_POST_APIKeyPreservation, http_test.go:452-800).
//
// This is not just dev-loop hygiene: it is the exact incident already
// documented in internal/config/config.go:81-91 ("a Groq key committed in
// cleartext to a tracked config.json blocked the PROD deployment") and
// docs/ADMIN_GUIDE.md:1240-1284, reproduced automatically by `go test ./...`
// itself.
//
// This file fails RED until #143 lands: config path indirection
// (SetConfigPath) in internal/config/config.go + t.Chdir(t.TempDir())
// isolation in the internal/server tests that call config.Save() indirectly
// via handleConfig (plan tasks 1-2). Once #143 lands, `internal/server/
// config.json` is never written by the suite and this passes cleanly.
//
// The guard is split in two layers on purpose:
//   - TestMain hashes the fixture before/after the *entire* package's test
//     run, because task 4 explicitly asks for a before/after comparison
//     across the whole suite, not just around one test. A test-scoped
//     t.Chdir in one file cannot detect a regression introduced by some
//     *other* test in the same package that forgets to isolate itself.
//   - TestConfigJSONFixture_RemainsTrackedAndReadOnly is a normal named test
//     that also asserts "no secret pattern", so `go test -run
//     TestConfigJSONFixture` in isolation (or a CI summarizer that only
//     surfaces named test failures, not TestMain's stderr + exit code)
//     still gets a clear, attributable failure.
//
// Non-regression rule: this file is additive only — it never touches
// http_test.go or any other existing test.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"testing"
)

// trackedFixturePath is internal/server/config.json, relative to this
// package's CWD under `go test` — the same relative path config.Get()/
// config.Save() resolve today (the bug), and the path this guard watches.
const trackedFixturePath = "config.json"

// secretPatternRe matches the two fake/real API key prefixes named
// explicitly in the acceptance criterion (plan §5: "Aucune clé API, même
// factice, n'apparaît dans un fichier tracké après exécution de la suite de
// tests") and used verbatim by TestHTTPServer_Config_POST_APIKeyPreservation.
var secretPatternRe = regexp.MustCompile(`sk-ant-|gsk_`)

var fixtureHashBeforeSuite string

// TestMain wraps the whole internal/server package's test run with the
// before/after hash comparison. No other _test.go in this package declares
// TestMain (verified: `grep -rn "func TestMain" internal/server` was empty
// before this file was added) — Go allows exactly one per package, so this
// is safe to add without touching existing files.
func TestMain(m *testing.M) {
	before, err := hashFile(trackedFixturePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "anti-pollution guard: cannot read tracked fixture %s before suite: %v\n", trackedFixturePath, err)
		os.Exit(1)
	}
	fixtureHashBeforeSuite = before

	if msg := secretPatternMessage(trackedFixturePath, "before"); msg != "" {
		// A dirty fixture before the suite even starts means a previous,
		// uncleaned `go test` run already leaked keys into it — fail loudly
		// rather than silently comparing "polluted" to "still polluted".
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}

	code := m.Run()

	after, err := hashFile(trackedFixturePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "anti-pollution guard: cannot read tracked fixture %s after suite: %v\n", trackedFixturePath, err)
		os.Exit(1)
	}

	if after != fixtureHashBeforeSuite {
		fmt.Fprintf(os.Stderr,
			"anti-pollution guard: %s was modified by the test suite (sha256 %s -> %s). "+
				"config.Get()/Save() must resolve a path independent of the process CWD (#143), "+
				"and internal/server tests that reach handleConfig's POST path must isolate "+
				"themselves with t.Chdir(t.TempDir()) (plan tasks 1-2).\n",
			trackedFixturePath, fixtureHashBeforeSuite, after)
		if code == 0 {
			code = 1
		}
	}

	if msg := secretPatternMessage(trackedFixturePath, "after"); msg != "" {
		fmt.Fprintln(os.Stderr, msg)
		if code == 0 {
			code = 1
		}
	}

	os.Exit(code)
}

func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// secretPatternMessage returns a non-empty diagnostic if path contains a
// forbidden secret pattern, "" otherwise.
func secretPatternMessage(path, when string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("anti-pollution guard: cannot read %s (%s suite): %v", path, when, err)
	}
	if secretPatternRe.Match(data) {
		return fmt.Sprintf(
			"anti-pollution guard: %s contains a fake or real API key pattern (\"sk-ant-\" or \"gsk_\") %s the test suite — "+
				"a test wrote secrets to a tracked file instead of an isolated temp config",
			path, when)
	}
	return ""
}

// TestConfigJSONFixture_RemainsTrackedAndReadOnly is the named-test companion
// to TestMain's guard (see file header for why both exist). It only checks
// the "no secret pattern" half — the hash comparison is inherently a
// whole-suite, before/after concern that a single test running in isolation
// cannot reproduce on its own.
func TestConfigJSONFixture_RemainsTrackedAndReadOnly(t *testing.T) {
	info, err := os.Stat(trackedFixturePath)
	if err != nil {
		t.Fatalf("tracked fixture %s must exist: %v", trackedFixturePath, err)
	}
	if info.IsDir() {
		t.Fatalf("%s must be a file, not a directory", trackedFixturePath)
	}

	data, err := os.ReadFile(trackedFixturePath)
	if err != nil {
		t.Fatalf("cannot read %s: %v", trackedFixturePath, err)
	}
	if secretPatternRe.Match(data) {
		t.Errorf("%s must never contain a real or fake API key pattern (\"sk-ant-\" / \"gsk_\") — "+
			"see plan §1.1 (security finding) and internal/config/config.go:81-91", trackedFixturePath)
	}
}
