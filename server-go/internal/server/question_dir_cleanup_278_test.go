package server

// #278 (v11.2.3) — un POST /questions refusé ne laisse aucune trace sur disque.
// Contrat : contracts/http-endpoints.md §POST /questions « Atomicité du refus — aucune trace sur disque ».
//
//   - question nouvelle (identifiant libre ou alloué automatiquement) : aucun dossier après un refus,
//     et l'identifiant redevient disponible pour l'allocation automatique suivante ;
//   - question existante : dossier, question.json et média jamais supprimés ni modifiés.
//
// Frontière publique uniquement : requêtes HTTP (mux) puis état du disque sous data/files/questions.

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// post278 envoie un POST /questions multipart : champs libres + fichier optionnel (champ, nom, contenu).
func post278(t *testing.T, server *HTTPServer, fields map[string]string, fileField, fileName string, fileContent []byte) *httptest.ResponseRecorder {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatalf("WriteField(%s): %v", k, err)
		}
	}
	if fileField != "" {
		part, err := mw.CreateFormFile(fileField, fileName)
		if err != nil {
			t.Fatalf("CreateFormFile(%s): %v", fileField, err)
		}
		if _, err := part.Write(fileContent); err != nil {
			t.Fatalf("write %s: %v", fileField, err)
		}
	}
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/questions", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	return w
}

// speedyFields278 : formulaire valide d'une question SPEEDY simple ; number "" = allocation automatique.
func speedyFields278(number string) map[string]string {
	f := map[string]string{"question": "Capitale de la France ?", "answer": "Paris", "type": "SPEEDY", "points": "10", "time": "30"}
	if number != "" {
		f["number"] = number
	}
	return f
}

// memotionFields278 : formulaire MEMOTION portant les cartes données (JSON) ; number "" = allocation automatique.
func memotionFields278(t *testing.T, number string, cards []map[string]interface{}) map[string]string {
	t.Helper()
	raw, err := json.Marshal(cards)
	if err != nil {
		t.Fatalf("marshal cards: %v", err)
	}
	f := map[string]string{"question": "MEMOTION #278", "answer": "", "type": "MEMOTION", "points": "1", "time": "30", "motion_cards": string(raw)}
	if number != "" {
		f["number"] = number
	}
	return f
}

// questionDirs278 liste les dossiers présents sous data/files/questions (vide si la racine n'existe pas).
func questionDirs278(t *testing.T, dataDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dataDir, "files", "questions"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("lecture de data/files/questions : %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func assertNoQuestionDir278(t *testing.T, dataDir, why string) {
	t.Helper()
	if dirs := questionDirs278(t, dataDir); len(dirs) != 0 {
		t.Errorf("%s : aucun dossier de question ne doit subsister après un refus, trouvé %v", why, dirs)
	}
}

// ── 1. Refus POINTS_RULE_MODE_NOT_APPLICABLE sur identifiant libre ─────────────────────────

func TestQuestionDirCleanup278_PointsRuleRefusal_FreeID_LeavesNoDirectory(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	card := pointsRuleCard("mc-1", "SPEEDY")
	withRule(card, rule("STARS_PRORATA")) // non applicable à SPEEDY

	w := post278(t, server, memotionFields278(t, "42", []map[string]interface{}{card}), "", "", nil)

	if w.Code != http.StatusBadRequest || !bytes.Contains(w.Body.Bytes(), []byte(codeNotApplicable)) {
		t.Fatalf("attendu 400 %s, obtenu %d : %s", codeNotApplicable, w.Code, w.Body.String())
	}
	assertNoQuestionDir278(t, dataDir, "refus POINTS_RULE (id 42 libre)")
}

// ── 2. Refus CARD_TYPE_CONTENT_MISMATCH en allocation auto : dossier absent, id réutilisé ───

func TestQuestionDirCleanup278_CardMismatchRefusal_AutoAllocation_ReleasesTheID(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	mismatch := map[string]interface{}{
		"ID": "mc-1", "RECTO_THEME": "x", "DIFFICULTY": 1, "TYPE": "SPEEDY",
		"QCM_PENALTY_1": 0.5, // champ QCM sur une carte SPEEDY
	}

	w := post278(t, server, memotionFields278(t, "", []map[string]interface{}{mismatch}), "", "", nil)

	if w.Code != http.StatusBadRequest || !bytes.Contains(w.Body.Bytes(), []byte("CARD_TYPE_CONTENT_MISMATCH")) {
		t.Fatalf("attendu 400 CARD_TYPE_CONTENT_MISMATCH, obtenu %d : %s", w.Code, w.Body.String())
	}
	assertNoQuestionDir278(t, dataDir, "refus CARD_TYPE_CONTENT_MISMATCH (allocation automatique)")

	// L'identifiant libéré est le premier candidat de l'allocation suivante : un seul dossier, « 1 ».
	ok := post278(t, server, speedyFields278(""), "", "", nil)
	if ok.Code != http.StatusOK {
		t.Fatalf("POST valide suivant : attendu 200, obtenu %d : %s", ok.Code, ok.Body.String())
	}
	if dirs := questionDirs278(t, dataDir); len(dirs) != 1 || dirs[0] != "1" {
		t.Errorf("l'identifiant du refus doit être réutilisé : attendu exactement [1], trouvé %v", dirs)
	}
}

// ── 3. Refus sound (.mp3 puis taille) sur identifiant libre ─────────────────────────────────

func TestQuestionDirCleanup278_SoundRefusal_FreeID_LeavesNoDirectory(t *testing.T) {
	for _, tc := range []struct {
		name     string
		fileName string
		content  []byte
		wantCode int
	}{
		{"extension .mp3", "son.mp3", []byte("ID3-pas-un-wav"), http.StatusBadRequest},
		{"fichier trop lourd (413)", "gros.wav", make([]byte, 64<<20), http.StatusRequestEntityTooLarge},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			server, dataDir := setupTestHTTPServer(t)

			w := post278(t, server, speedyFields278("43"), "sound", tc.fileName, tc.content)

			if w.Code != tc.wantCode {
				t.Fatalf("attendu %d, obtenu %d : %s", tc.wantCode, w.Code, w.Body.String())
			}
			assertNoQuestionDir278(t, dataDir, "refus sound "+tc.name+" (id 43 libre)")
		})
	}
}

// ── 4. Refus sur une question EXISTANTE : dossier, question.json et média intacts ───────────

func TestQuestionDirCleanup278_Refusal_OnExistingQuestion_KeepsEverythingIntact(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	if w := post278(t, server, speedyFields278("7"), "", "", nil); w.Code != http.StatusOK {
		t.Fatalf("préparation : création de la question 7 : %d %s", w.Code, w.Body.String())
	}
	dir := filepath.Join(dataDir, "files", "questions", "7")
	mediaPath := filepath.Join(dir, "media_keep.jpg")
	if err := os.WriteFile(mediaPath, []byte("jpeg-bytes"), 0o644); err != nil {
		t.Fatalf("préparation : média : %v", err)
	}
	beforeJSON, err := os.ReadFile(filepath.Join(dir, "question.json"))
	if err != nil {
		t.Fatalf("préparation : question.json : %v", err)
	}

	fields := speedyFields278("7")
	fields["question"] = "Texte qui ne doit PAS être écrit"
	w := post278(t, server, fields, "sound", "son.mp3", []byte("ID3-pas-un-wav"))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("attendu 400 (sound .mp3), obtenu %d : %s", w.Code, w.Body.String())
	}
	afterJSON, err := os.ReadFile(filepath.Join(dir, "question.json"))
	if err != nil {
		t.Fatalf("le dossier ou question.json de la question existante a disparu : %v", err)
	}
	if !bytes.Equal(beforeJSON, afterJSON) {
		t.Errorf("question.json d'une question existante modifié par un refus :\navant : %s\naprès : %s", beforeJSON, afterJSON)
	}
	if got, err := os.ReadFile(mediaPath); err != nil || string(got) != "jpeg-bytes" {
		t.Errorf("le média de la question existante doit rester intact (err=%v, contenu=%q)", err, got)
	}
}

// ── 5. Succès : dossier présent (non-régression) ────────────────────────────────────────────

func TestQuestionDirCleanup278_Success_KeepsTheDirectoryWithItsQuestionJSON(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	w := post278(t, server, speedyFields278("5"), "", "", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, obtenu %d : %s", w.Code, w.Body.String())
	}
	if dirs := questionDirs278(t, dataDir); len(dirs) != 1 || dirs[0] != "5" {
		t.Errorf("attendu exactement le dossier [5], trouvé %v", dirs)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "files", "questions", "5", "question.json")); err != nil {
		t.Errorf("question.json doit exister après un succès : %v", err)
	}
}

// ── 6. number invalide : 400 sans dossier (non-régression #245) ─────────────────────────────

func TestQuestionDirCleanup278_InvalidNumber_Returns400WithoutDirectory(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	w := post278(t, server, speedyFields278("../evil"), "", "", nil)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("attendu 400 (number invalide), obtenu %d : %s", w.Code, w.Body.String())
	}
	assertNoQuestionDir278(t, dataDir, "number invalide")
	if _, err := os.Stat(filepath.Join(dataDir, "files", "evil")); err == nil {
		t.Error("un number invalide ne doit rien créer hors de data/files/questions")
	}
}
