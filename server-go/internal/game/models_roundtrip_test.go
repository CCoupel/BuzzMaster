package game

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestQuestionFixtures_RoundTrip_TypedContent is the B-B1 gate for #184: the
// TypedContent embedding (Question, MotionCard) must not add, drop, or
// rename a single JSON field for any of the existing question.json
// fixtures — contracts/question-types.md §2, §9.3, §10.3.
//
// "Octet pour octet" (contract §2) is verified here as SEMANTIC equality —
// same key set, same values — rather than a literal byte comparison, and
// that's a deliberate reading, not a relaxation: every writer of
// question.json in this codebase (handleUploadQuestion, the REORDER handler
// in main.go, the test-question seeder) marshals a map[string]interface{},
// never the Question struct directly. Go's encoding/json sorts map keys
// alphabetically on Marshal, which is exactly the key order every fixture
// on disk has today — a property of those writers, not of Question's own
// field-declaration-ordered Marshal. A literal byte comparison against
// Question's struct-order Marshal would therefore have failed even before
// this refactor (nothing in this codebase has ever round-tripped
// question.json through the Question struct back to disk) and would test a
// formatting accident, not what the contract actually cares about.
//
// What this refactor CAN genuinely break — a JSON field silently renamed,
// dropped, or gaining/losing omitempty as it moves into the shared
// TypedContent struct — is what this test catches: unmarshal the original
// bytes into Question, marshal back out, and require the two byte strings
// to decode to the identical map[string]interface{} (erasing only key
// order and int-vs-float64 formatting noise, nothing semantic) — MODULO
// stripJSONOmitemptyZeros below, which removes exactly the keys Question's
// own `omitempty` tags would legitimately drop on ANY struct-based
// round-trip, refactor or not (e.g. "QCM_HINTS_ENABLED": false,
// MemoryCard's "IMAGE": "" — both pre-existing: their omitempty tags are
// untouched copies from before #184, verifiable via `git show
// b098373:server-go/internal/game/models.go`). Driving that normalization
// off the actual struct tags via reflection — rather than a hand-maintained
// allowlist — means it self-updates if a field's omitempty status ever
// changes, and a REAL new drop/rename regression still fails loudly because
// it wouldn't match any tag-derived exemption.
func TestQuestionFixtures_RoundTrip_TypedContent(t *testing.T) {
	dir := filepath.Join("..", "..", "data", "files", "questions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cannot read fixtures dir %s: %v", dir, err)
	}

	tested := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name(), "question.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			continue // mirrors loadQuestions()'s tolerance of dirs without a question.json
		}

		var wantMap map[string]interface{}
		if err := json.Unmarshal(raw, &wantMap); err != nil {
			t.Errorf("%s: original file is not valid JSON: %v", path, err)
			continue
		}
		stripJSONOmitemptyZeros(reflect.TypeOf(Question{}), wantMap)

		var q Question
		if err := json.Unmarshal(raw, &q); err != nil {
			t.Errorf("%s: Unmarshal into Question failed: %v", path, err)
			continue
		}

		remarshaled, err := json.MarshalIndent(&q, "", "  ")
		if err != nil {
			t.Errorf("%s: Marshal(Question) failed: %v", path, err)
			continue
		}

		var gotMap map[string]interface{}
		if err := json.Unmarshal(remarshaled, &gotMap); err != nil {
			t.Errorf("%s: round-tripped bytes are not valid JSON: %v", path, err)
			continue
		}

		if !reflect.DeepEqual(wantMap, gotMap) {
			t.Errorf("%s: round-trip through Question changed the JSON content.\nBefore (normalized): %v\nAfter:  %s", path, wantMap, remarshaled)
		}
		tested++
	}

	// Fixed at the fixture count known at the time this test was written
	// (#184, B-B1) so a silently-empty glob (e.g. wrong path after a
	// directory move) fails loudly instead of reporting a vacuous pass.
	// Update this constant deliberately if fixtures are added/removed.
	const wantFixtureCount = 85
	if tested != wantFixtureCount {
		t.Errorf("expected to test %d question.json fixtures, tested %d — fixture count changed (update wantFixtureCount if deliberate) or the fixtures dir was not found as expected", wantFixtureCount, tested)
	}
}

// stripJSONOmitemptyZeros walks a decoded JSON object (m, as produced by
// json.Unmarshal into map[string]interface{} — so values are only
// string/float64/bool/nil/[]interface{}/map[string]interface{}) alongside
// the Go struct type t that will Marshal it, and deletes every key whose
// value is the JSON zero equivalent of a field tagged `omitempty` in t.
// Recurses into nested structs (embedded or not), pointers-to-struct,
// slices/arrays of struct, and map-of-struct values, using each field's own
// type — so it reaches nested omitempty like MemoryPair.Card1.Image or a
// MotionCard's own (embedded TypedContent) fields inside MOTION_CARDS[].
//
// This exists solely so models_roundtrip_test.go's fixture comparison isn't
// tripped up by pre-existing (non-#184) omitempty-zero-value fields — see
// that test's doc comment.
func stripJSONOmitemptyZeros(t reflect.Type, m map[string]interface{}) {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}

	// Two passes, mirroring encoding/json's shadowing rule (shallowest field
	// wins when names collide across an embedding boundary — see
	// TypedContent's doc comment for why this matters: Question's own
	// ANSWER, depth 0, shadows TypedContent's promoted ANSWER, depth 1).
	// Pass 1 claims every directly-declared (non-anonymous) field name at
	// THIS level; pass 2 recurses into anonymous embeds but skips any name
	// pass 1 already claimed, so a shadowed promoted field's own omitempty
	// is never consulted.
	claimed := map[string]bool{}
	var anonymous []reflect.StructField

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" && !f.Anonymous {
			continue // unexported, never reaches JSON
		}
		if f.Anonymous {
			anonymous = append(anonymous, f)
			continue
		}
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		parts := strings.Split(tag, ",")
		name := parts[0]
		if name == "" {
			continue
		}
		claimed[name] = true
		omitempty := false
		for _, p := range parts[1:] {
			if p == "omitempty" {
				omitempty = true
			}
		}

		val, exists := m[name]
		if !exists {
			continue
		}
		if omitempty && isJSONZeroValue(val) {
			delete(m, name)
			continue
		}
		recurseNormalize(f.Type, val)
	}

	for _, f := range anonymous {
		stripJSONOmitemptyZerosSkipping(f.Type, m, claimed)
	}
}

// stripJSONOmitemptyZerosSkipping is stripJSONOmitemptyZeros restricted to
// an embedded type's own directly-declared fields, ignoring any JSON name
// already claimed by a shallower field (see the shadowing note above). It
// does not need to handle its own further embeds specially beyond passing
// `skip` through, since TypedContent (the only embed in this codebase) has
// none — but it does so anyway for robustness against a future embed of an
// embed.
func stripJSONOmitemptyZerosSkipping(t reflect.Type, m map[string]interface{}, skip map[string]bool) {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" && !f.Anonymous {
			continue
		}
		if f.Anonymous {
			stripJSONOmitemptyZerosSkipping(f.Type, m, skip)
			continue
		}
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		parts := strings.Split(tag, ",")
		name := parts[0]
		if name == "" || skip[name] {
			continue
		}
		omitempty := false
		for _, p := range parts[1:] {
			if p == "omitempty" {
				omitempty = true
			}
		}
		val, exists := m[name]
		if !exists {
			continue
		}
		if omitempty && isJSONZeroValue(val) {
			delete(m, name)
			continue
		}
		recurseNormalize(f.Type, val)
	}
}

// recurseNormalize dispatches stripJSONOmitemptyZeros across the shapes a
// decoded JSON value can take relative to a Go field type ft: a nested
// object (struct/pointer-to-struct), an array of objects, or a map whose
// values are objects (map[string]MotionCardState and friends never nest
// further, so those are no-ops here).
func recurseNormalize(ft reflect.Type, val interface{}) {
	for ft.Kind() == reflect.Ptr {
		ft = ft.Elem()
	}
	switch ft.Kind() {
	case reflect.Struct:
		if obj, ok := val.(map[string]interface{}); ok {
			stripJSONOmitemptyZeros(ft, obj)
		}
	case reflect.Slice, reflect.Array:
		elemType := ft.Elem()
		if arr, ok := val.([]interface{}); ok {
			for _, item := range arr {
				recurseNormalize(elemType, item)
			}
		}
	case reflect.Map:
		elemType := ft.Elem()
		if obj, ok := val.(map[string]interface{}); ok {
			for _, item := range obj {
				recurseNormalize(elemType, item)
			}
		}
	}
}

// isJSONZeroValue reports whether v — a value decoded by
// encoding/json.Unmarshal into interface{} — is the zero value that
// `omitempty` would have omitted, for every JSON kind Question/TypedContent/
// MotionCard actually use (string, float64, bool; nil covers omitted
// pointers/slices/maps, which never appear as an explicit JSON null in
// these fixtures but is handled for completeness).
func isJSONZeroValue(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return true
	case bool:
		return !x
	case string:
		return x == ""
	case float64:
		return x == 0
	case []interface{}:
		return len(x) == 0
	case map[string]interface{}:
		return len(x) == 0
	default:
		return false
	}
}

// TestMotionCardTypedContent_RoundTrip_NoNewKeysWhenEmpty is the MotionCard
// side of the same guarantee, exercised directly rather than only through
// the 9 MEMOTION fixtures (which never populate a card's TypedContent
// fields today): a SPEEDY-only card (no TYPE, no typed content set) must
// marshal to exactly the historical field set — no "ANSWER":"", no
// "QCM_ANSWERS", no "MEMORY_MODE" appearing out of nowhere because
// TypedContent got embedded — contracts/question-types.md §11 ("Emplacements
// média par descripteur" / "TypedContent embarqué" rows: CHANGED internally,
// unchanged on the wire).
func TestMotionCardTypedContent_RoundTrip_NoNewKeysWhenEmpty(t *testing.T) {
	card := MotionCard{
		ID:           "mc-1",
		RectoTheme:   "Théo",
		Difficulty:   1,
		QuestionText: "Q?",
		AnswerText:   "A",
	}

	data, err := json.Marshal(&card)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	want := map[string]interface{}{
		"ID":            "mc-1",
		"RECTO_THEME":   "Théo",
		"DIFFICULTY":    float64(1),
		"QUESTION_TEXT": "Q?",
		"ANSWER_TEXT":   "A",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SPEEDY MotionCard gained/lost fields via TypedContent embedding.\ngot:  %v\nwant: %v\nraw: %s", got, want, data)
	}
}
