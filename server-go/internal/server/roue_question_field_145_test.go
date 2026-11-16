package server

import (
	"net/http"
	"testing"
)

// #145 — l'éditeur envoie `question` = phrase de présentation, ou « Roue de la Fortune » si vide :
// le serveur l'accepte tel quel, et la phrase à deviner ne passe jamais par QUESTION.
func TestRoueUpload145_QuestionFieldIsPresentationOrDefaultTitle(t *testing.T) {
	for _, q := range []string{"Roue de la Fortune", "Chanson de Line Renaud"} {
		s, _ := setupTestHTTPServer(t)
		w, saved := postRoue145(t, s, map[string]string{"question": q, "roue_presentation": q})
		if w.Code != http.StatusOK {
			t.Fatalf("question=%q : attendu 200, got %d : %s", q, w.Code, w.Body.String())
		}
		if saved["QUESTION"] != q {
			t.Errorf("QUESTION = %v, attendu %q", saved["QUESTION"], q)
		}
		if saved["QUESTION"] == saved["ROUE_PHRASE"] || saved["ANSWER"] != "" {
			t.Errorf("la phrase ne doit pas fuir dans QUESTION/ANSWER : %v", saved)
		}
	}
}
