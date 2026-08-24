// Tests for #184 — réenregistrement sans erreur des 9 questions MEMOTION
// EXISTANTES au travers du vrai chemin d'upload HTTP (handleUploadQuestion),
// donc contre la validation NEUVE de B-B2 (CARD_TYPE_NOT_NESTABLE /
// CARD_TYPE_CONTENT_MISMATCH). C'est un critère de fait explicite du plan
// (#184) : « Les 9 questions MEMOTION existantes s'ouvrent verrouillées sur
// SPEEDY, se modifient et se réenregistrent sans erreur » — dev-backend l'a
// coché dans son handoff comme "vérifié conforme au contrat, rétrocompatible"
// mais sans test dédié rejouant les VRAIS fichiers du dépôt : ses tests
// existants (motion_card_type_lock_184_test.go) construisent des cartes
// synthétiques de forme équivalente, jamais les 9 vrais `question.json`.
// TestQuestionFixtures_RoundTrip_TypedContent (B-B1, models_roundtrip_test.go)
// couvre le round-trip JSON générique (Unmarshal/Marshal du struct Go), pas
// le chemin `handleUploadQuestion` ni sa validation B-B2 — les deux tests
// sont complémentaires, pas redondants.
//
// Run: go test ./internal/server/... -run 184_ExistingMEMOTIONFixtures -v
package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// existingMEMOTIONFixtureIDs — les 9 questions MEMOTION réelles du dépôt
// (`grep -l '"TYPE": *"MEMOTION"' data/files/questions/*/question.json`).
// Liste figée délibérément (comme wantFixtureCount dans
// models_roundtrip_test.go) : si ce nombre change, le test doit être mis à
// jour en connaissance de cause plutôt que de silencieusement tester moins
// de fixtures.
var existingMEMOTIONFixtureIDs184 = []string{"14", "15", "16", "36", "37", "38", "83", "84", "85"}

func loadRawFixtureQuestion184(t *testing.T, id string) map[string]interface{} {
	t.Helper()
	// internal/server → ../../data/files/questions (même relatif que
	// internal/game/models_roundtrip_test.go, deux niveaux plus haut).
	path := filepath.Join("..", "..", "data", "files", "questions", id, "question.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("impossible de lire la fixture %s : %v", path, err)
	}
	var q map[string]interface{}
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatalf("impossible de décoder la fixture %s : %v", path, err)
	}
	return q
}

// TestHTTPServer_184_ExistingMEMOTIONFixtures_ReUploadAccepted rejoue chacune
// des 9 questions MEMOTION réelles au travers de handleUploadQuestion, avec
// EXACTEMENT le contenu de carte du fichier sur disque (aucune carte ne porte
// TYPE ⇒ toutes verrouillées sur SPEEDY implicite, contrat §3.2 — « les 9
// questions MEMOTION existantes s'ouvrent verrouillées sur SPEEDY... et se
// réenregistrent sans erreur »). Un échec ici serait une régression de
// compatibilité réelle, pas hypothétique : ces 9 fichiers sont ceux qui
// existent aujourd'hui en production.
func TestHTTPServer_184_ExistingMEMOTIONFixtures_ReUploadAccepted(t *testing.T) {
	for _, id := range existingMEMOTIONFixtureIDs184 {
		t.Run("fixture_"+id, func(t *testing.T) {
			fixture := loadRawFixtureQuestion184(t, id)

			cards, ok := fixture["MOTION_CARDS"]
			if !ok {
				t.Fatalf("fixture %s : pas de MOTION_CARDS", id)
			}
			// Aucune carte de ces 9 fixtures ne doit porter de clé TYPE — sinon
			// ce test ne prouverait pas ce qu'il prétend (rétrocompatibilité du
			// cas SANS TYPE). Vérifié explicitement plutôt que supposé.
			cardList, ok := cards.([]interface{})
			if !ok {
				t.Fatalf("fixture %s : MOTION_CARDS n'est pas un tableau", id)
			}
			for i, c := range cardList {
				card, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				if _, hasType := card["TYPE"]; hasType {
					t.Fatalf("fixture %s : carte %d porte déjà TYPE=%v — ce test suppose les 9 "+
						"fixtures existantes sans TYPE (rétrocompatibilité SPEEDY implicite)", id, i, card["TYPE"])
				}
			}

			cardsJSON, err := json.Marshal(cardList)
			if err != nil {
				t.Fatalf("fixture %s : marshal MOTION_CARDS : %v", id, err)
			}

			server, dataDir := setupTestHTTPServer(t)

			body := &bytes.Buffer{}
			mw := multipart.NewWriter(body)
			_ = mw.WriteField("question", stringField184(fixture["QUESTION"]))
			_ = mw.WriteField("answer", stringField184(fixture["ANSWER"]))
			_ = mw.WriteField("points", stringField184(fixture["POINTS"]))
			_ = mw.WriteField("time", stringField184(fixture["TIME"]))
			_ = mw.WriteField("type", "MEMOTION")
			_ = mw.WriteField("category", stringField184(fixture["CATEGORY"]))
			_ = mw.WriteField("MOTION_MODE", stringField184(fixture["MOTION_MODE"]))
			_ = mw.WriteField("motion_cards", string(cardsJSON))
			if cfg, ok := fixture["MOTION_CONFIG"]; ok {
				cfgJSON, _ := json.Marshal(cfg)
				_ = mw.WriteField("motion_config", string(cfgJSON))
			}
			_ = mw.Close()

			req := httptest.NewRequest("POST", "/questions", body)
			req.Header.Set("Content-Type", mw.FormDataContentType())
			w := httptest.NewRecorder()
			server.mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("fixture %s : réenregistrement refusé, attendu 200, got %d: %s",
					id, w.Code, w.Body.String())
			}

			// Le fichier réenregistré doit porter des cartes toujours sans TYPE
			// explicite (SPEEDY implicite préservé, pas de migration forcée) et
			// garder leur ANSWER_TEXT — le champ dont la présence verrouille
			// SPEEDY côté UI (contrat §3.2, table des OwnedFields).
			questionsDir := filepath.Join(dataDir, "files", "questions")
			entries, err := os.ReadDir(questionsDir)
			if err != nil || len(entries) != 1 {
				t.Fatalf("fixture %s : attendu exactement un dossier de question écrit, got %v (err=%v)", id, entries, err)
			}
			raw, err := os.ReadFile(filepath.Join(questionsDir, entries[0].Name(), "question.json"))
			if err != nil {
				t.Fatalf("fixture %s : lecture du question.json réenregistré : %v", id, err)
			}
			var written map[string]interface{}
			if err := json.Unmarshal(raw, &written); err != nil {
				t.Fatalf("fixture %s : question.json réenregistré invalide : %v", id, err)
			}
			writtenCards, _ := written["MOTION_CARDS"].([]interface{})
			if len(writtenCards) != len(cardList) {
				t.Fatalf("fixture %s : %d cartes réenregistrées, attendu %d", id, len(writtenCards), len(cardList))
			}
			for i, c := range writtenCards {
				card, _ := c.(map[string]interface{})
				if _, hasType := card["TYPE"]; hasType {
					t.Errorf("fixture %s : carte %d a gagné un TYPE explicite au réenregistrement, "+
						"attendu absent (SPEEDY implicite préservé)", id, i)
				}
				if _, hasAnswerText := card["ANSWER_TEXT"]; !hasAnswerText {
					t.Errorf("fixture %s : carte %d a perdu son ANSWER_TEXT au réenregistrement", id, i)
				}
			}
		})
	}
}

// stringField184 convertit une valeur décodée JSON générique (string déjà,
// dans les 9 fixtures — POINTS/TIME sont des chaînes, pas des nombres, voir
// data/files/questions/14/question.json) en chaîne de formulaire, sans
// paniquer si le type diffère (défensif — pas une conversion numérique).
func stringField184(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
