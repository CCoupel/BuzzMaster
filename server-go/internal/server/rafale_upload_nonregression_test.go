package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Non-régression handleUploadQuestion (v8.0.0, #107, SHA 4374ac08) — le
// bugfix RAFALE_DIFFICULTY a ajouté un bloc `if questionType == "RAFALE"`
// APRÈS les blocs existants QCM/MEMORY/MEMOTION/ARDOISE, sans y toucher.
// TestHTTPServer_QuestionUpload (SPEEDY) et TestHTTPServer_MemoryQuestionUpload
// (MEMORY) existaient déjà et restent verts (déjà vérifié). QCM et ARDOISE
// n'avaient AUCUN test à ce niveau HTTP avant ce fichier — gap préexistant,
// sans rapport avec le bugfix RAFALE, mais l'occasion de fermer la question
// "les autres types persistent-ils toujours correctement" avec une preuve
// directe plutôt qu'une lecture de code. MEMOTION (media uploads, JSON
// imbriqué bien plus complexe) est hors périmètre ici — surface
// structurellement isolée du bloc RAFALE de la même façon que les autres,
// et déjà exercée par ses propres tests dédiés dans ce paquet
// (motion_card_*_test.go).
// ---------------------------------------------------------------------------

func TestHTTPServer_QCMQuestionUpload_UnaffectedByRafaleBlock(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	questionsDir := filepath.Join(dataDir, "files", "questions")
	os.MkdirAll(questionsDir, 0755)

	body := strings.NewReader("--boundary\r\n" +
		"Content-Disposition: form-data; name=\"question\"\r\n\r\n" +
		"Capitale de la France ?\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"answer\"\r\n\r\n" +
		"Paris\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"type\"\r\n\r\n" +
		"QCM\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"points\"\r\n\r\n" +
		"10\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"time\"\r\n\r\n" +
		"30\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"qcm_answers\"\r\n\r\n" +
		`{"RED":"Paris","GREEN":"Lyon","YELLOW":"Marseille","BLUE":"Nice"}` + "\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"qcm_correct\"\r\n\r\n" +
		"RED\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"qcm_hints_enabled\"\r\n\r\n" +
		"true\r\n" +
		"--boundary--\r\n")

	req := httptest.NewRequest("POST", "/questions", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		bodyBytes, _ := io.ReadAll(w.Body)
		t.Fatalf("Expected 200, got %d: %s", w.Code, string(bodyBytes))
	}

	entries, _ := os.ReadDir(questionsDir)
	if len(entries) == 0 {
		t.Fatal("Expected question directory to be created")
	}
	data, err := os.ReadFile(filepath.Join(questionsDir, entries[0].Name(), "question.json"))
	if err != nil {
		t.Fatalf("Failed to read question.json: %v", err)
	}
	var question map[string]interface{}
	if err := json.Unmarshal(data, &question); err != nil {
		t.Fatalf("Failed to parse question.json: %v", err)
	}

	if question["TYPE"] != "QCM" {
		t.Errorf("Expected TYPE 'QCM', got %v", question["TYPE"])
	}
	answers, ok := question["QCM_ANSWERS"].(map[string]interface{})
	if !ok {
		t.Fatal("QCM_ANSWERS should be an object")
	}
	if answers["RED"] != "Paris" {
		t.Errorf("Expected QCM_ANSWERS.RED='Paris', got %v", answers["RED"])
	}
	if question["QCM_CORRECT"] != "RED" {
		t.Errorf("Expected QCM_CORRECT='RED', got %v", question["QCM_CORRECT"])
	}
	if question["QCM_HINTS_ENABLED"] != true {
		t.Errorf("Expected QCM_HINTS_ENABLED=true, got %v", question["QCM_HINTS_ENABLED"])
	}
	// Sanity: no RAFALE_* field leaked into a QCM question (the RAFALE
	// block is guarded by questionType == "RAFALE", never reached here).
	for _, key := range []string{"RAFALE_DIFFICULTY", "RAFALE_MODE", "RAFALE_QUESTION_TIME", "RAFALE_MAX_QUESTIONS"} {
		if _, present := question[key]; present {
			t.Errorf("QCM question must never carry %s, got %v", key, question[key])
		}
	}
}

func TestHTTPServer_ArdoiseQuestionUpload_UnaffectedByRafaleBlock(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	questionsDir := filepath.Join(dataDir, "files", "questions")
	os.MkdirAll(questionsDir, 0755)

	body := strings.NewReader("--boundary\r\n" +
		"Content-Disposition: form-data; name=\"question\"\r\n\r\n" +
		"Plus long fleuve du monde ?\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"answer\"\r\n\r\n" +
		"Le Nil\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"type\"\r\n\r\n" +
		"ARDOISE\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"points\"\r\n\r\n" +
		"15\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"time\"\r\n\r\n" +
		"45\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"ardoise_keyboard_type\"\r\n\r\n" +
		"NUMPAD\r\n" +
		"--boundary--\r\n")

	req := httptest.NewRequest("POST", "/questions", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		bodyBytes, _ := io.ReadAll(w.Body)
		t.Fatalf("Expected 200, got %d: %s", w.Code, string(bodyBytes))
	}

	entries, _ := os.ReadDir(questionsDir)
	if len(entries) == 0 {
		t.Fatal("Expected question directory to be created")
	}
	data, err := os.ReadFile(filepath.Join(questionsDir, entries[0].Name(), "question.json"))
	if err != nil {
		t.Fatalf("Failed to read question.json: %v", err)
	}
	var question map[string]interface{}
	if err := json.Unmarshal(data, &question); err != nil {
		t.Fatalf("Failed to parse question.json: %v", err)
	}

	if question["TYPE"] != "ARDOISE" {
		t.Errorf("Expected TYPE 'ARDOISE', got %v", question["TYPE"])
	}
	if question["ARDOISE_KEYBOARD_TYPE"] != "NUMPAD" {
		t.Errorf("Expected ARDOISE_KEYBOARD_TYPE='NUMPAD', got %v", question["ARDOISE_KEYBOARD_TYPE"])
	}
	for _, key := range []string{"RAFALE_DIFFICULTY", "RAFALE_MODE", "RAFALE_QUESTION_TIME", "RAFALE_MAX_QUESTIONS"} {
		if _, present := question[key]; present {
			t.Errorf("ARDOISE question must never carry %s, got %v", key, question[key])
		}
	}
}
