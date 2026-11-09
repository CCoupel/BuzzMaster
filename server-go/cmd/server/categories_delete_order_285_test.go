// #285 (LOT-4, re-revue mineur 1) — un DELETE qui modifie des questions envoie à l'admin exactement un QUESTIONS,
// après CATEGORIES_CHANGED (plus de QUESTIONS préalable via OnQuestionUpload).
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"buzzcontrol/internal/config"
)

func TestCategoriesDelete285_Wire_SingleQuestionsAfterCategoriesChanged(t *testing.T) {
	w := tw285WireStart(t)
	w.postCategory(t, "Cinema")
	dir := filepath.Join(config.Get().Storage.DataDir, "files", "questions", "q1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	q, _ := json.Marshal(map[string]interface{}{"ID": "q1", "CATEGORY": "CINEMA", "QUESTION": "?"})
	if err := os.WriteFile(filepath.Join(dir, "question.json"), q, 0o644); err != nil {
		t.Fatal(err)
	}
	w.settle(t) // purge

	w.request(t, "DELETE", "/api/categories/CINEMA", `{"mode":"reassign"}`)
	got := w.settle(t)

	var order []string
	for _, f := range got["admin"] {
		if a := tw285Action(f); a == "CATEGORIES_CHANGED" || a == "QUESTIONS" {
			order = append(order, a)
		}
	}
	if len(order) != 2 || order[0] != "CATEGORIES_CHANGED" || order[1] != "QUESTIONS" {
		t.Errorf("ordre admin = %v, attendu exactement [CATEGORIES_CHANGED QUESTIONS]", order)
	}
}
