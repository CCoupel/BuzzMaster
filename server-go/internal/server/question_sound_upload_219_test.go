// Suite test-writer pour #219 (milestone v11.1 — plan
// _work/reports/plan-20260922-103848.md §9, contract sound.md §10.4 et
// http-endpoints.md §Questions) : l'upload du son de question dans
// handleUploadQuestion (internal/server/http.go, tâche 7) — préservation
// sur ré-édition, suppression réelle via sound_cleared, refus nommés (CA2),
// avertissement contextuel présent en mode simultané / absent en mode
// différé, et SOUND_TIMER_DELAYED jamais écrit sans SOUND.
//
// Convention de collision : préfixe tw219h pour ne jamais entrer en
// collision avec un helper d'un autre fichier de ce paquet.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
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

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/game"
)

// tw219hBuildWAV hand-builds a minimal canonical (or deliberately
// non-canonical, via the given fields) RIFF/WAVE byte stream — same
// technique as internal/audio's own test-writer fixtures
// (bank_229_test.go's tw229bBuildWAVHeader), duplicated locally since it
// lives in a different package/module boundary (internal/server never
// imports internal/audio's test files).
func tw219hBuildWAV(channels, sampleRate, bitsPerSample, dataLen int) []byte {
	byteRate := sampleRate * channels * (bitsPerSample / 8)
	blockAlign := channels * (bitsPerSample / 8)
	buf := make([]byte, 44+dataLen)
	copy(buf[0:4], "RIFF")
	tw219hPutU32(buf[4:8], uint32(36+dataLen))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	tw219hPutU32(buf[16:20], 16)
	tw219hPutU16(buf[20:22], 1)
	tw219hPutU16(buf[22:24], uint16(channels))
	tw219hPutU32(buf[24:28], uint32(sampleRate))
	tw219hPutU32(buf[28:32], uint32(byteRate))
	tw219hPutU16(buf[32:34], uint16(blockAlign))
	tw219hPutU16(buf[34:36], uint16(bitsPerSample))
	copy(buf[36:40], "data")
	tw219hPutU32(buf[40:44], uint32(dataLen))
	return buf
}

func tw219hPutU32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func tw219hPutU16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

// tw219hCanonicalWAV builds a valid canonical WAV of the given duration in
// seconds (whole seconds only, exact frame count).
func tw219hCanonicalWAV(seconds int) []byte {
	dataLen := seconds * audio.SampleRate * audio.ChannelCount * audio.BitsPerSample / 8
	return tw219hBuildWAV(audio.ChannelCount, audio.SampleRate, audio.BitsPerSample, dataLen)
}

// tw219hUploadFields is the base multipart form for a SPEEDY question,
// overridable per test via the fields map.
type tw219hUploadFields struct {
	number           string
	question         string
	answer           string
	points           string
	time             string
	soundTimerDelay  string // "true"/"false"/"" (omitted)
	soundCleared     string // "true"/"false"/"" (omitted)
	soundFileName    string // "" = no "sound" part at all
	soundFileContent []byte
}

// tw219hNewUploadRequest builds a POST /questions multipart request from f.
func tw219hNewUploadRequest(t *testing.T, f tw219hUploadFields) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	_ = mw.WriteField("number", f.number)
	_ = mw.WriteField("question", f.question)
	_ = mw.WriteField("answer", f.answer)
	_ = mw.WriteField("points", f.points)
	_ = mw.WriteField("time", f.time)
	_ = mw.WriteField("type", "SPEEDY")
	if f.soundTimerDelay != "" {
		_ = mw.WriteField("sound_timer_delayed", f.soundTimerDelay)
	}
	if f.soundCleared != "" {
		_ = mw.WriteField("sound_cleared", f.soundCleared)
	}
	if f.soundFileName != "" {
		part, err := mw.CreateFormFile("sound", f.soundFileName)
		if err != nil {
			t.Fatalf("CreateFormFile(sound): %v", err)
		}
		if _, err := part.Write(f.soundFileContent); err != nil {
			t.Fatalf("write sound content: %v", err)
		}
	}
	_ = mw.Close()

	req := httptest.NewRequest("POST", "/questions", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

// tw219hReadQuestion reads back question.json for the given explicit ID.
func tw219hReadQuestion(t *testing.T, dataDir, id string) game.Question {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dataDir, "files", "questions", id, "question.json"))
	if err != nil {
		t.Fatalf("failed to read written question.json for id=%s: %v", id, err)
	}
	var q game.Question
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatalf("failed to unmarshal written question.json: %v", err)
	}
	return q
}

// tw219hResponseWarning decodes the upload response body's "warning" field
// (contract http-endpoints.md §Questions: null when absent, never an
// omitted key — nullableString's own convention).
func tw219hResponseWarning(t *testing.T, body []byte) (warning string, isNull bool) {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to unmarshal response body %q: %v", body, err)
	}
	if _, ok := resp["warning"]; !ok {
		t.Fatalf(`response body is missing the "warning" key entirely — contract requires null, never an omitted key: %q`, body)
	}
	if resp["warning"] == nil {
		return "", true
	}
	s, ok := resp["warning"].(string)
	if !ok {
		t.Fatalf(`"warning" is present but not a string or null: %v`, resp["warning"])
	}
	return s, false
}

// ---------------------------------------------------------------------------
// CA1/CA8 — valid upload, then preservation and real deletion on re-edit.
// ---------------------------------------------------------------------------

func TestQuestionSoundUpload_ValidWAV_SetsSoundField(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	req := tw219hNewUploadRequest(t, tw219hUploadFields{
		number: "1", question: "Q", answer: "A", points: "1", time: "20",
		soundFileName: "sound.wav", soundFileContent: tw219hCanonicalWAV(5),
	})
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	q := tw219hReadQuestion(t, dataDir, "1")
	if q.Sound == "" {
		t.Fatal("expected SOUND to be set after a valid upload")
	}
	if filepath.Ext(q.Sound) != ".wav" {
		t.Errorf("expected SOUND to point at a .wav file, got %q", q.Sound)
	}
	if warning, isNull := tw219hResponseWarning(t, w.Body.Bytes()); !isNull {
		t.Errorf(`expected "warning": null (5s sound, 20s time, no mismatch), got %q`, warning)
	}
}

func TestQuestionSoundUpload_PreservedOnReeditWithoutSoundField(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	first := tw219hNewUploadRequest(t, tw219hUploadFields{
		number: "1", question: "Q", answer: "A", points: "1", time: "20",
		soundFileName: "sound.wav", soundFileContent: tw219hCanonicalWAV(5),
	})
	w1 := httptest.NewRecorder()
	server.mux.ServeHTTP(w1, first)
	if w1.Code != http.StatusOK {
		t.Fatalf("first upload: expected 200, got %d: %s", w1.Code, w1.Body.String())
	}
	original := tw219hReadQuestion(t, dataDir, "1").Sound
	if original == "" {
		t.Fatal("setup invalide : SOUND absent après le premier upload")
	}

	// Re-edit: same number, DIFFERENT question text, NO "sound" field, NO
	// sound_cleared — R3's exact trap (handleUploadQuestion reconstructs
	// `question` from scratch).
	second := tw219hNewUploadRequest(t, tw219hUploadFields{
		number: "1", question: "Q edited", answer: "A", points: "1", time: "20",
	})
	w2 := httptest.NewRecorder()
	server.mux.ServeHTTP(w2, second)
	if w2.Code != http.StatusOK {
		t.Fatalf("second upload: expected 200, got %d: %s", w2.Code, w2.Body.String())
	}

	q := tw219hReadQuestion(t, dataDir, "1")
	if q.Sound != original {
		t.Errorf("SOUND must survive a re-edit that sends no \"sound\" field (R3) — before=%q after=%q", original, q.Sound)
	}
	if q.Question != "Q edited" {
		t.Fatalf("setup invalide : le texte de la question n'a pas été mis à jour, got %q", q.Question)
	}
	// The underlying file itself must still exist on disk.
	oldPath := filepath.Join(dataDir, "files", "questions", "1", filepath.Base(original))
	if _, err := os.Stat(oldPath); err != nil {
		t.Errorf("le fichier son préservé doit toujours exister sur disque : %v", err)
	}
}

func TestQuestionSoundUpload_ClearedRemovesFieldAndFile(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	first := tw219hNewUploadRequest(t, tw219hUploadFields{
		number: "1", question: "Q", answer: "A", points: "1", time: "20",
		soundFileName: "sound.wav", soundFileContent: tw219hCanonicalWAV(5),
	})
	w1 := httptest.NewRecorder()
	server.mux.ServeHTTP(w1, first)
	if w1.Code != http.StatusOK {
		t.Fatalf("first upload: expected 200, got %d: %s", w1.Code, w1.Body.String())
	}
	original := tw219hReadQuestion(t, dataDir, "1").Sound
	oldPath := filepath.Join(dataDir, "files", "questions", "1", filepath.Base(original))
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatalf("setup invalide : le fichier son du premier upload est introuvable : %v", err)
	}

	second := tw219hNewUploadRequest(t, tw219hUploadFields{
		number: "1", question: "Q", answer: "A", points: "1", time: "20",
		soundCleared: "true",
	})
	w2 := httptest.NewRecorder()
	server.mux.ServeHTTP(w2, second)
	if w2.Code != http.StatusOK {
		t.Fatalf("second upload: expected 200, got %d: %s", w2.Code, w2.Body.String())
	}

	q := tw219hReadQuestion(t, dataDir, "1")
	if q.Sound != "" {
		t.Errorf("SOUND doit être vide après sound_cleared=true, got %q", q.Sound)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Errorf("le fichier son doit être réellement supprimé du disque après sound_cleared=true (err=%v)", err)
	}
}

// ---------------------------------------------------------------------------
// CA2 — refus nommés, sauvegarde entière rejetée (pas de save partiel).
// ---------------------------------------------------------------------------

func TestQuestionSoundUpload_RefusesNonWavExtension(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	req := tw219hNewUploadRequest(t, tw219hUploadFields{
		number: "1", question: "Q", answer: "A", points: "1", time: "20",
		soundFileName: "sound.mp3", soundFileContent: []byte("not really audio"),
	})
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a non-.wav filename, got %d: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dataDir, "files", "questions", "1", "question.json")); !os.IsNotExist(err) {
		t.Error("CA2 : un son refusé doit rejeter TOUTE la sauvegarde — question.json ne doit pas exister")
	}
}

func TestQuestionSoundUpload_RefusesNonCanonicalFormat(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	// 48kHz mono — structurally valid WAV, wrong format (contract §3).
	bad := tw219hBuildWAV(1, 48000, 16, 48000*1*2*2)
	req := tw219hNewUploadRequest(t, tw219hUploadFields{
		number: "1", question: "Q", answer: "A", points: "1", time: "20",
		soundFileName: "sound.wav", soundFileContent: bad,
	})
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a non-canonical WAV, got %d: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dataDir, "files", "questions", "1", "question.json")); !os.IsNotExist(err) {
		t.Error("CA2 : un son refusé doit rejeter TOUTE la sauvegarde — question.json ne doit pas exister")
	}
}

func TestQuestionSoundUpload_RefusesOver30Seconds(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	req := tw219hNewUploadRequest(t, tw219hUploadFields{
		number: "1", question: "Q", answer: "A", points: "1", time: "20",
		soundFileName: "sound.wav", soundFileContent: tw219hCanonicalWAV(31),
	})
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a 31s sound (limit 30s), got %d: %s", w.Code, w.Body.String())
	}
}

func TestQuestionSoundUpload_RefusesOverByteLimit(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	// The handler caps its read at MaxQuestionSoundBytes+1 via
	// io.LimitReader before even validating the format — any file whose
	// RAW SIZE exceeds the cap is refused with 413, regardless of content.
	huge := make([]byte, audio.MaxQuestionSoundBytes+1024)
	req := tw219hNewUploadRequest(t, tw219hUploadFields{
		number: "1", question: "Q", answer: "A", points: "1", time: "20",
		soundFileName: "sound.wav", soundFileContent: huge,
	})
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for a file over the byte cap, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Avertissement contextuel — présent en mode simultané, absent en mode
// différé (contract §10.4 : "la situation ne peut pas se produire" en
// mode différé).
// ---------------------------------------------------------------------------

func TestQuestionSoundUpload_ContextualWarning_PresentInSimultaneousMode(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	// 10s sound, 5s answer time, mode simultané (sound_timer_delayed omitted).
	req := tw219hNewUploadRequest(t, tw219hUploadFields{
		number: "1", question: "Q", answer: "A", points: "1", time: "5",
		soundFileName: "sound.wav", soundFileContent: tw219hCanonicalWAV(10),
	})
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	warning, isNull := tw219hResponseWarning(t, w.Body.Bytes())
	if isNull {
		t.Fatal(`expected a non-null "warning" (10s sound > 5s answer time, mode simultané)`)
	}
	if warning == "" {
		t.Error("warning present but empty")
	}
}

func TestQuestionSoundUpload_ContextualWarning_AbsentInDeferredMode(t *testing.T) {
	server, _ := setupTestHTTPServer(t)

	// Same mismatch as above (10s sound, 5s answer time) but mode différé —
	// the warning must NOT be emitted (contract §10.4).
	req := tw219hNewUploadRequest(t, tw219hUploadFields{
		number: "1", question: "Q", answer: "A", points: "1", time: "5",
		soundFileName: "sound.wav", soundFileContent: tw219hCanonicalWAV(10),
		soundTimerDelay: "true",
	})
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if warning, isNull := tw219hResponseWarning(t, w.Body.Bytes()); !isNull {
		t.Errorf(`expected "warning": null in deferred mode (the mismatch cannot occur, contract §10.4), got %q`, warning)
	}
}

// ---------------------------------------------------------------------------
// SOUND_TIMER_DELAYED — jamais écrit sans un son réellement attaché.
// ---------------------------------------------------------------------------

func TestQuestionSoundUpload_TimerDelayedTrue_WithSound_IsPersisted(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	req := tw219hNewUploadRequest(t, tw219hUploadFields{
		number: "1", question: "Q", answer: "A", points: "1", time: "20",
		soundFileName: "sound.wav", soundFileContent: tw219hCanonicalWAV(5),
		soundTimerDelay: "true",
	})
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	q := tw219hReadQuestion(t, dataDir, "1")
	if !q.SoundTimerDelayed {
		t.Error("SOUND_TIMER_DELAYED doit être true quand un son est réellement attaché et sound_timer_delayed=true")
	}
}

func TestQuestionSoundUpload_TimerDelayedTrue_WithoutAnySound_IsNeverWritten(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	// No "sound" field, no existing question, sound_timer_delayed=true —
	// no sound ends up attached at all, so the flag must not linger
	// dangling (handler's own doc comment: "a stale true left over after
	// sound_cleared must never linger with no sound to apply to").
	req := tw219hNewUploadRequest(t, tw219hUploadFields{
		number: "1", question: "Q", answer: "A", points: "1", time: "20",
		soundTimerDelay: "true",
	})
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	q := tw219hReadQuestion(t, dataDir, "1")
	if q.SoundTimerDelayed {
		t.Error("SOUND_TIMER_DELAYED ne doit jamais être écrit à true quand aucun son n'est attaché")
	}
}

func TestQuestionSoundUpload_ClearedAndTimerDelayedTrue_IsNeverWritten(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)

	first := tw219hNewUploadRequest(t, tw219hUploadFields{
		number: "1", question: "Q", answer: "A", points: "1", time: "20",
		soundFileName: "sound.wav", soundFileContent: tw219hCanonicalWAV(5),
		soundTimerDelay: "true",
	})
	w1 := httptest.NewRecorder()
	server.mux.ServeHTTP(w1, first)
	if w1.Code != http.StatusOK {
		t.Fatalf("first upload: expected 200, got %d: %s", w1.Code, w1.Body.String())
	}

	// Now clear the sound WHILE still resubmitting sound_timer_delayed=true
	// (a plausible stale form state) — SOUND_TIMER_DELAYED must not survive
	// with no sound left to apply to.
	second := tw219hNewUploadRequest(t, tw219hUploadFields{
		number: "1", question: "Q", answer: "A", points: "1", time: "20",
		soundCleared: "true", soundTimerDelay: "true",
	})
	w2 := httptest.NewRecorder()
	server.mux.ServeHTTP(w2, second)
	if w2.Code != http.StatusOK {
		t.Fatalf("second upload: expected 200, got %d: %s", w2.Code, w2.Body.String())
	}

	q := tw219hReadQuestion(t, dataDir, "1")
	if q.Sound != "" {
		t.Fatalf("setup invalide : SOUND devrait être vide après sound_cleared=true, got %q", q.Sound)
	}
	if q.SoundTimerDelayed {
		t.Error("SOUND_TIMER_DELAYED ne doit jamais rester à true une fois le son effacé (sound_cleared=true)")
	}
}
