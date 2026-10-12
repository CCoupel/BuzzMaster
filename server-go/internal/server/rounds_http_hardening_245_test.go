package server

// #245 (v11.2) — re-revue (code-reviewer-20261013-130000.md, M1-bis) : les chemins HTTP qui construisent un chemin de fichier à
// partir d'une valeur du client sont sécurisés comme ROUNDS_UPDATE : POST /questions (champ `number`) et DELETE /files/...
// ROUGES avant les corrections. `critical` : aucun dossier créé hors de files/questions, aucune suppression de la racine des questions.
//
// SÉCURITÉ DU TEST : le répertoire de données est placé 2 niveaux sous un répertoire temporaire dédié ; un `number` du type
// « ../../x » ne peut sortir que dans ce répertoire dédié, jamais dans le dossier temporaire système.

import (
	"buzzcontrol/internal/game"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sandbox245 : serveur HTTP dont dataDir = <racine>/a/b ; « files/questions/../.. » = <racine>/a/b … ; « ../../x » = <racine>/a/b/x.
func sandbox245(t *testing.T) (s *HTTPServer, root, dataDir string) {
	t.Helper()
	s, _ = setupTestHTTPServer(t)
	root = t.TempDir()
	dataDir = filepath.Join(root, "a", "b")
	if err := os.MkdirAll(filepath.Join(dataDir, "files", "questions"), 0o755); err != nil {
		t.Fatal(err)
	}
	s.dataDir = dataDir
	return s, root, dataDir
}

func tree245(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if info.IsDir() {
			out[rel] = "<dir>"
		} else {
			b, _ := os.ReadFile(p)
			out[rel] = string(b)
		}
		return nil
	})
	return out
}

func diff245(a, b map[string]string) string {
	for k, v := range a {
		if bv, ok := b[k]; !ok {
			return "disparu : " + k
		} else if bv != v {
			return "modifié : " + k
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			return "apparu : " + k
		}
	}
	return ""
}

// --- (1) POST /questions : le champ `number` ---------------------------------------------------------------------------------

func TestRoundsHTTPHardening245_Critical_UploadWithAForgedOrInvalidNumberIsRefusedWithNoFolderCreated(t *testing.T) {
	for _, number := range []string{"../../x", "../x", "..", ".", "a/b", "a b", "x\\y", "  ", "é", strings.Repeat("9", 65)} {
		t.Run(number, func(t *testing.T) {
			s, root, _ := sandbox245(t)
			before := tree245(t, root)

			code := postQuestion254(t, s, map[string]string{"number": number, "type": "SPEEDY", "question": "Q", "answer": "a", "points": "1", "time": "30", "category": "CULTURE"})

			if code != http.StatusBadRequest {
				t.Errorf("number %q : 400 attendu, obtenu %d", number, code)
			}
			if d := diff245(before, tree245(t, root)); d != "" {
				t.Errorf("number %q : aucun dossier ni fichier créé, nulle part (%s)", number, d)
			}
		})
	}
}

func TestRoundsHTTPHardening245_UploadWithAValidNumberOrNoNumberStillWorks(t *testing.T) {
	s, _, dataDir := sandbox245(t)
	if code := postQuestion254(t, s, map[string]string{"number": "12", "type": "SPEEDY", "question": "Q", "answer": "a", "points": "1", "time": "30", "category": "CULTURE"}); code != http.StatusOK {
		t.Fatalf("number valide : %d", code)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "files", "questions", "12", "question.json")); err != nil {
		t.Errorf("la question 12 est créée : %v", err)
	}
	if code := postQuestion254(t, s, map[string]string{"type": "SPEEDY", "question": "Auto", "answer": "a", "points": "1", "time": "30", "category": "CULTURE"}); code != http.StatusOK {
		t.Fatalf("sans number : attribution automatique : %d", code)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "files", "questions", "1", "question.json")); err != nil {
		t.Errorf("attribution automatique du premier ID libre : %v", err)
	}
}

// --- (2) DELETE /files/... ---------------------------------------------------------------------------------------------------

func del245(s *HTTPServer, path string) int {
	req := httptest.NewRequest("DELETE", path, nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	return w.Code
}

func seed245(t *testing.T, dataDir string) {
	t.Helper()
	writeQuestionFile254(t, dataDir, "5", map[string]interface{}{"ID": "5", "TYPE": "SPEEDY", "QUESTION": "Q", "ANSWER": "a", "POINTS": "1", "TIME": "30", "ORDER": 0})
	if err := os.WriteFile(filepath.Join(dataDir, "files", "questions", "5", "media.png"), []byte("img"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dataDir, "config", "teams.json"), []byte("{}"), 0o644)
	_ = os.WriteFile(filepath.Join(dataDir, "files", "autre.txt"), []byte("x"), 0o644)
}

func TestRoundsHTTPHardening245_Critical_DeleteOfTheQuestionsRootOrOutsideAQuestionFolderIsRefusedAndNothingIsRemoved(t *testing.T) {
	for _, path := range []string{"/files/questions", "/files/questions/", "/files/", "/files/autre.txt", "/files/config", "/files/questions/rounds.json"} {
		t.Run(path, func(t *testing.T) {
			s, root, dataDir := sandbox245(t)
			seed245(t, dataDir)
			_ = os.WriteFile(filepath.Join(dataDir, "files", "questions", "rounds.json"), []byte(`{"ROUNDS":[],"NEXT_ID":1}`), 0o644)
			before := tree245(t, root)

			code := del245(s, path)

			if code == http.StatusOK {
				t.Errorf("DELETE %s : refusé (4xx), obtenu 200", path)
			}
			if d := diff245(before, tree245(t, root)); d != "" {
				t.Errorf("DELETE %s : rien n'est supprimé (%s)", path, d)
			}
		})
	}
}

func TestRoundsHTTPHardening245_DeleteOfAQuestionMediaIsStillAccepted(t *testing.T) {
	s, _, dataDir := sandbox245(t)
	seed245(t, dataDir)

	if code := del245(s, "/files/questions/5/media.png"); code != http.StatusOK {
		t.Fatalf("suppression légitime d'un média de question : %d", code)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "files", "questions", "5", "media.png")); err == nil {
		t.Error("le média est supprimé")
	}
	if _, err := os.Stat(filepath.Join(dataDir, "files", "questions", "5", "question.json")); err != nil {
		t.Errorf("question.json de la question est intact : %v", err)
	}
}

func TestRoundsHTTPHardening245_DeletingAWholeQuestionFolderPurgesItsPlayedStatusOrIsRefused(t *testing.T) {
	s, _, dataDir := sandbox245(t)
	seed245(t, dataDir)
	// la question 5 a été jouée (STOPPED), puis une autre est sélectionnée
	s.engine.SetPhase(game.PhaseStopped)
	s.engine.Ready("", &game.Question{ID: "5", Type: game.QuestionTypeSpeedy})
	s.engine.StartImmediate(5)
	s.engine.Stop()
	s.engine.SetPhase(game.PhaseStopped)
	s.engine.Ready("", &game.Question{ID: "6", Type: game.QuestionTypeSpeedy})
	if s.engine.GetQuestionStatus("5") != game.StatusStopped {
		t.Fatal("précondition : la question 5 est jouée")
	}

	code := del245(s, "/files/questions/5")

	_, statErr := os.Stat(filepath.Join(dataDir, "files", "questions", "5"))
	switch {
	case code == http.StatusOK && statErr != nil:
		if st := s.engine.GetQuestionStatus("5"); st == game.StatusStopped {
			t.Errorf("dossier supprimé : le statut « jouée » de la question 5 est purgé (%s), sinon un ID réutilisé apparaît jouée", st)
		}
	case code != http.StatusOK && statErr == nil:
		// refus propre : le dossier d'une question ne se supprime que par l'action DELETE (verrou, statuts, contexte de manche)
	default:
		t.Errorf("DELETE d'un dossier de question : soit supprimé (200) et statut purgé, soit refusé et conservé ; obtenu code=%d dossier présent=%v", code, statErr == nil)
	}
}
