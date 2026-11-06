// #288 (v11.4.0, LOT-3) — upload multipart `sound_loop` : contracts/http-endpoints.md §Questions, sound.md §13.2,
// models.md `SOUND_LOOP`. Écrit dans SOUND_LOOP uniquement si un son est attaché (préservé ou téléversé) ; réécrit à
// chaque enregistrement ; combinable avec sound_timer_delayed (aucun 400). Réutilise tw219h* (même paquet).
// Compile avant le code : SOUND_LOOP est lu dans le JSON écrit sur disque, pas via un champ typé.
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

// tw288Upload POSTe /questions (SPEEDY n°1) avec des champs libres et, si soundWAV != nil, une partie `sound`.
func tw288Upload(t *testing.T, s *HTTPServer, extra map[string]string, soundWAV []byte) *httptest.ResponseRecorder {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	for k, v := range map[string]string{"number": "1", "question": "Q", "answer": "A", "points": "1", "time": "20", "type": "SPEEDY"} {
		_ = mw.WriteField(k, v)
	}
	for k, v := range extra {
		_ = mw.WriteField(k, v)
	}
	if soundWAV != nil {
		part, err := mw.CreateFormFile("sound", "sound.wav")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write(soundWAV)
	}
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/questions", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	return w
}

// tw288Stored lit question.json brut : renvoie la valeur de SOUND_LOOP (false si clé absente) et SOUND_TIMER_DELAYED.
func tw288Stored(t *testing.T, dataDir string) (loop, delayed bool, sound string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dataDir, "files", "questions", "1", "question.json"))
	if err != nil {
		t.Fatalf("question.json illisible : %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("question.json invalide : %v", err)
	}
	loop, _ = m["SOUND_LOOP"].(bool)
	delayed, _ = m["SOUND_TIMER_DELAYED"].(bool)
	sound, _ = m["SOUND"].(string)
	return
}

func TestQuestionSoundLoopUpload288_WithSound_LoopTrue_Stored(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	w := tw288Upload(t, s, map[string]string{"sound_loop": "true"}, tw219hCanonicalWAV(5))
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	loop, delayed, sound := tw288Stored(t, dataDir)
	if sound == "" || !loop || delayed {
		t.Errorf("SOUND=%q SOUND_LOOP=%v DELAYED=%v, attendu son présent, boucle vraie, différé faux", sound, loop, delayed)
	}
}

func TestQuestionSoundLoopUpload288_NoSound_LoopIgnored(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	w := tw288Upload(t, s, map[string]string{"sound_loop": "true"}, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if loop, _, sound := tw288Stored(t, dataDir); sound != "" || loop {
		t.Errorf("sans son attaché SOUND_LOOP ne doit jamais être écrit (SOUND=%q LOOP=%v)", sound, loop)
	}
}

func TestQuestionSoundLoopUpload288_AbsentField_WithSound_LoopFalse(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw288Upload(t, s, nil, tw219hCanonicalWAV(5))
	if loop, _, sound := tw288Stored(t, dataDir); sound == "" || loop {
		t.Errorf("champ absent ⇒ une fois (SOUND_LOOP faux, omis) ; got SOUND=%q LOOP=%v", sound, loop)
	}
}

func TestQuestionSoundLoopUpload288_LoopAndDeferred_BothWritten_NoError(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	w := tw288Upload(t, s, map[string]string{"sound_loop": "true", "sound_timer_delayed": "true"}, tw219hCanonicalWAV(5))
	if w.Code != http.StatusOK {
		t.Fatalf("boucle + différé combinables : attendu 200 (aucun 400), got %d : %s", w.Code, w.Body.String())
	}
	if loop, delayed, sound := tw288Stored(t, dataDir); sound == "" || !loop || !delayed {
		t.Errorf("les deux champs doivent être écrits ensemble ; got SOUND=%q LOOP=%v DELAYED=%v", sound, loop, delayed)
	}
}

// Réécrit à chaque enregistrement : ré-édition sans `sound_loop` (son préservé) ⇒ SOUND_LOOP retombe à faux ;
// avec `sound_loop=true` et sans nouveau fichier ⇒ vrai, son préservé.
func TestQuestionSoundLoopUpload288_Reedit_RewrittenEachSave_SoundPreserved(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw288Upload(t, s, map[string]string{"sound_loop": "true"}, tw219hCanonicalWAV(5))
	_, _, original := tw288Stored(t, dataDir)

	tw288Upload(t, s, nil, nil) // ré-édition sans le champ
	loop, _, sound := tw288Stored(t, dataDir)
	if sound != original || loop {
		t.Errorf("re-édition sans sound_loop : son préservé (%q→%q) et SOUND_LOOP faux attendus, loop=%v", original, sound, loop)
	}

	tw288Upload(t, s, map[string]string{"sound_loop": "true"}, nil)
	loop, _, sound = tw288Stored(t, dataDir)
	if sound != original || !loop {
		t.Errorf("re-édition avec sound_loop=true : son préservé et SOUND_LOOP vrai attendus, got sound=%q loop=%v", sound, loop)
	}
}

func TestQuestionSoundLoopUpload288_SoundCleared_LoopNotKept(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw288Upload(t, s, map[string]string{"sound_loop": "true"}, tw219hCanonicalWAV(5))
	tw288Upload(t, s, map[string]string{"sound_cleared": "true", "sound_loop": "true"}, nil)
	if loop, _, sound := tw288Stored(t, dataDir); sound != "" || loop {
		t.Errorf("son supprimé ⇒ SOUND_LOOP jamais conservé ; got SOUND=%q LOOP=%v", sound, loop)
	}
}

// Valeur de sound_loop autre que "true" ⇒ faux (même lecture stricte que sound_timer_delayed).
func TestQuestionSoundLoopUpload288_NonTrueValue_IsFalse(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw288Upload(t, s, map[string]string{"sound_loop": "false"}, tw219hCanonicalWAV(5))
	if loop, _, _ := tw288Stored(t, dataDir); loop {
		t.Error(`sound_loop="false" ⇒ SOUND_LOOP faux`)
	}
}
