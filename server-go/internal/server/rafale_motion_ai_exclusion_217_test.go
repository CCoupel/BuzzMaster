package server

// Test for #217 — RAFALE stays excluded from AI generation even after
// becoming nestable in a MEMOTION card (decision 2026-08-28, not reopened).
// This file was referenced by a comment in test-writer's own
// internal/game/rafale_motion_card_217_test.go (first pass) before it
// actually existed — fixed here rather than left dangling.
//
// The JS-side mirror (GENERABLE_TYPES, questionTypeMeta.js) was already
// confirmed excluding RAFALE while writing this file — this is the Go-side
// companion, generableQuestionTypes (ai_generator.go), the actual schema
// source consumed by the AI provider call.
import "testing"

func TestGenerableQuestionTypes_ExcludesRafale_EvenNestable(t *testing.T) {
	for _, ty := range generableQuestionTypes {
		if ty == "RAFALE" {
			t.Fatalf("generableQuestionTypes contains RAFALE — #217 (nestable in a MEMOTION card) must NOT reopen AI generation for it (decision 2026-08-28, contracts/CHANGELOG.md)")
		}
	}
}
