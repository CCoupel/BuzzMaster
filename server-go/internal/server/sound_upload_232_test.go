// #232 (v11.6.0, LOT-1) — conversion automatique des uploads sonores, sur les TROIS chemins :
// POST /api/sounds/{cue} (bruitage), POST /api/sounds/decompte, POST /questions (champ `sound`).
// Source normative : contracts/http-endpoints.md « Uploads sonores — conversion automatique »
// + contracts/sound.md §3 amendement #232. Écrit depuis le contrat avant le code (rouge attendu).
//
// Vérifié côté frontière HTTP uniquement : statut, champs additifs `converted` / `source_format`
// (jamais omis : `null` quand déjà canonique), fichier STOCKÉ strictement canonique, refus nommés,
// plafond d'entrée brute 4 × canonique (413). Préfixe tw232u pour les helpers.
package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"buzzcontrol/internal/audio"
)

// tw232uWAV : WAV PCM entier (bits 16/24) ou float32 (tag 3), sinus 440 Hz amplitude 0,4.
func tw232uWAV(tag uint16, channels, rate, bits int, seconds float64) []byte {
	frames := int(seconds * float64(rate))
	var data bytes.Buffer
	for i := 0; i < frames; i++ {
		v := 0.4 * math.Sin(2*math.Pi*440*float64(i)/float64(rate))
		for c := 0; c < channels; c++ {
			switch {
			case tag == 3:
				_ = binary.Write(&data, binary.LittleEndian, float32(v))
			case bits == 24:
				x := int32(math.Round(v * 8388607))
				data.Write([]byte{byte(x), byte(x >> 8), byte(x >> 16)})
			default:
				_ = binary.Write(&data, binary.LittleEndian, int16(math.Round(v*32767)))
			}
		}
	}
	return tw232uAssemble(tag, channels, rate, bits, data.Bytes())
}

func tw232uAssemble(tag uint16, channels, rate, bits int, data []byte) []byte {
	le := binary.LittleEndian
	ba := channels * bits / 8
	var b bytes.Buffer
	b.WriteString("RIFF")
	_ = binary.Write(&b, le, uint32(36+len(data)))
	b.WriteString("WAVEfmt ")
	_ = binary.Write(&b, le, uint32(16))
	_ = binary.Write(&b, le, tag)
	_ = binary.Write(&b, le, uint16(channels))
	_ = binary.Write(&b, le, uint32(rate))
	_ = binary.Write(&b, le, uint32(rate*ba))
	_ = binary.Write(&b, le, uint16(ba))
	_ = binary.Write(&b, le, uint16(bits))
	b.WriteString("data")
	_ = binary.Write(&b, le, uint32(len(data)))
	b.Write(data)
	return b.Bytes()
}

// tw232uAssertStoredCanonical : le fichier stocké doit être un WAV strictement canonique
// (44 100 Hz, 2 canaux, 16 bits, tag 1) — « seul le résultat canonique est stocké ».
func tw232uAssertStoredCanonical(t *testing.T, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("fichier stocké illisible (%s) : %v", path, err)
	}
	if _, err := audio.ValidateQuestionSound(raw); err != nil {
		t.Fatalf("le fichier stocké doit rester valide : %v", err)
	}
	if len(raw) < 36 || string(raw[0:4]) != "RIFF" || string(raw[8:12]) != "WAVE" {
		t.Fatalf("fichier stocké : pas un RIFF/WAVE")
	}
	le := binary.LittleEndian
	tag, ch := le.Uint16(raw[20:]), le.Uint16(raw[22:])
	rate, bits := le.Uint32(raw[24:]), le.Uint16(raw[34:])
	if tag != 1 || int(ch) != audio.ChannelCount || int(rate) != audio.SampleRate || int(bits) != audio.BitsPerSample {
		t.Errorf("fichier stocké non canonique : tag=%d ch=%d rate=%d bits=%d", tag, ch, rate, bits)
	}
}

// tw232uDecode exige que `converted` ET `source_format` soient PRÉSENTS (jamais omis).
func tw232uDecode(t *testing.T, body []byte) (converted bool, source interface{}) {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("réponse non JSON : %v — %s", err, body)
	}
	c, ok := m["converted"]
	if !ok {
		t.Fatalf("champ `converted` absent de la réponse (additif, jamais omis) : %s", body)
	}
	if _, ok := m["source_format"]; !ok {
		t.Fatalf("champ `source_format` absent (doit valoir null si déjà canonique, jamais omis) : %s", body)
	}
	b, isBool := c.(bool)
	if !isBool {
		t.Fatalf("`converted` doit être un booléen, got %T", c)
	}
	return b, m["source_format"]
}

// ---------------------------------------------------------------------------
// Chemin 1 — bruitage : POST /api/sounds/{cue}
// ---------------------------------------------------------------------------

func TestSoundUpload232_Cue_NonCanonical_ConvertedAndStoredCanonical(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)

	body, ct := multipartImageBody(t, "bip.wav", tw232uWAV(1, 1, 48000, 24, 1.0))
	w := tw230Post(t, s, "/api/sounds/depart", body, ct)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200 (WAV PCM converti, plus refusé), got %d : %s", w.Code, w.Body.String())
	}
	converted, src := tw232uDecode(t, w.Body.Bytes())
	if !converted {
		t.Error("converted=true attendu pour un 48 kHz/24 bits/mono")
	}
	if src != "48000Hz/1ch/24bit" {
		t.Errorf("source_format attendu \"48000Hz/1ch/24bit\" (contrat), got %v", src)
	}
	tw232uAssertStoredCanonical(t, filepath.Join(tw230SoundsDir(dataDir), "depart.wav"))
}

func TestSoundUpload232_Cue_Float32_Converted(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	body, ct := multipartImageBody(t, "f.wav", tw232uWAV(3, 2, 48000, 32, 0.5))
	w := tw230Post(t, s, "/api/sounds/gagne", body, ct)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if converted, src := tw232uDecode(t, w.Body.Bytes()); !converted || src != "48000Hz/2ch/32bit" {
		t.Errorf("converted=true et source_format=\"48000Hz/2ch/32bit\" attendus, got %v / %v", converted, src)
	}
	tw232uAssertStoredCanonical(t, filepath.Join(tw230SoundsDir(dataDir), "gagne.wav"))
}

func TestSoundUpload232_Cue_AlreadyCanonical_NotConverted_NullSource_BytesUntouched(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	canonical := tw230CanonicalWAV(0.5)

	body, ct := multipartImageBody(t, "ok.wav", canonical)
	w := tw230Post(t, s, "/api/sounds/depart", body, ct)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	converted, src := tw232uDecode(t, w.Body.Bytes())
	if converted || src != nil {
		t.Errorf("déjà canonique ⇒ converted=false et source_format=null, got %v / %v", converted, src)
	}
	stored, err := os.ReadFile(filepath.Join(tw230SoundsDir(dataDir), "depart.wav"))
	if err != nil {
		t.Fatalf("fichier stocké illisible : %v", err)
	}
	if !bytes.Equal(stored, canonical) {
		t.Error("non-régression : un fichier déjà canonique doit être stocké octet pour octet")
	}
}

func TestSoundUpload232_Cue_MP3Refused400_NamedCause(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	before, _ := os.ReadFile(filepath.Join(tw230SoundsDir(dataDir), "depart.wav"))

	// (a) extension .mp3
	body, ct := multipartImageBody(t, "musique.mp3", []byte("ID3\x03\x00\x00\x00\x00\x00\x21fake mp3"))
	if w := tw230Post(t, s, "/api/sounds/depart", body, ct); w.Code != http.StatusBadRequest {
		t.Errorf(".mp3 : attendu 400, got %d : %s", w.Code, w.Body.String())
	}
	// (b) MP3 déguisé en .wav (tag 0x55)
	body, ct = multipartImageBody(t, "faux.wav", tw232uAssemble(0x55, 2, 44100, 16, make([]byte, 4000)))
	w := tw230Post(t, s, "/api/sounds/depart", body, ct)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("WAV compressé (tag 0x55) : attendu 400, got %d : %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "error") && strings.TrimSpace(w.Body.String()) == "" {
		t.Error("le refus doit porter un message nommant la cause")
	}
	after, _ := os.ReadFile(filepath.Join(tw230SoundsDir(dataDir), "depart.wav"))
	if !bytes.Equal(before, after) {
		t.Error("un refus ne doit pas modifier le fichier existant")
	}
}

func TestSoundUpload232_Cue_DurationLimitApplied_AfterConversion(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	// 6 s, 48 kHz/16 bits/mono = 576 Kio brut (sous tous les plafonds d'octets) mais > 5 s.
	body, ct := multipartImageBody(t, "long.wav", tw232uWAV(1, 1, 48000, 16, 6))
	if w := tw230Post(t, s, "/api/sounds/depart", body, ct); w.Code != http.StatusBadRequest {
		t.Errorf("6 s > limite de 5 s (inchangée) : attendu 400, got %d : %s", w.Code, w.Body.String())
	}
}

// Plafond brut = 4 × 2 Mio. 2,9 Mio brut > 2 Mio canonique mais ≤ 8 Mio ⇒ accepté ; 9 Mio ⇒ 413.
func TestSoundUpload232_Cue_RawCap_4xCanonical(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)

	mid := tw232uWAV(1, 2, 192000, 24, 2.5) // ≈ 2,9 Mio
	if len(mid) <= audio.MaxUploadBytes || len(mid) > 4*audio.MaxUploadBytes {
		t.Fatalf("setup invalide : %d octets", len(mid))
	}
	body, ct := multipartImageBody(t, "hd.wav", mid)
	if w := tw230Post(t, s, "/api/sounds/depart", body, ct); w.Code != http.StatusOK {
		t.Errorf("brut %d octets (≤ 4 × plafond) : attendu 200, got %d : %s", len(mid), w.Code, w.Body.String())
	}

	huge := tw232uAssemble(1, 2, 44100, 16, make([]byte, 9<<20))
	body, ct = multipartImageBody(t, "enorme.wav", huge)
	if w := tw230Post(t, s, "/api/sounds/depart", body, ct); w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("brut %d octets (> 4 × plafond) : attendu 413, got %d", len(huge), w.Code)
	}
}

// ---------------------------------------------------------------------------
// Chemin 2 — décompte : POST /api/sounds/decompte
// ---------------------------------------------------------------------------

func TestSoundUpload232_Countdown_NonCanonical_Converted(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)

	body, ct := multipartImageBody(t, "decompte.wav", tw232uWAV(1, 1, 48000, 24, 2.0))
	w := tw230Post(t, s, "/api/sounds/decompte", body, ct)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	converted, src := tw232uDecode(t, w.Body.Bytes())
	if !converted || src != "48000Hz/1ch/24bit" {
		t.Errorf("converted=true / source_format=\"48000Hz/1ch/24bit\" attendus, got %v / %v", converted, src)
	}
	tw232uAssertStoredCanonical(t, filepath.Join(tw230SoundsDir(dataDir), "decompte.wav"))
}

func TestSoundUpload232_Countdown_AlreadyCanonical_NotConverted(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	body, ct := multipartImageBody(t, "decompte.wav", tw230CanonicalWAV(1))
	w := tw230Post(t, s, "/api/sounds/decompte", body, ct)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if converted, src := tw232uDecode(t, w.Body.Bytes()); converted || src != nil {
		t.Errorf("déjà canonique ⇒ converted=false / source_format=null, got %v / %v", converted, src)
	}
}

func TestSoundUpload232_Countdown_MP3Refused(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	body, ct := multipartImageBody(t, "d.wav", tw232uAssemble(0x55, 2, 44100, 16, make([]byte, 4000)))
	if w := tw230Post(t, s, "/api/sounds/decompte", body, ct); w.Code != http.StatusBadRequest {
		t.Errorf("attendu 400, got %d : %s", w.Code, w.Body.String())
	}
}

// Limite 30 s inchangée, évaluée après conversion (31 s à 48 kHz/16 bits/mono = 2,9 Mio brut < 24 Mio).
func TestSoundUpload232_Countdown_DurationLimit30s_AfterConversion(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	body, ct := multipartImageBody(t, "d.wav", tw232uWAV(1, 1, 48000, 16, 31))
	if w := tw230Post(t, s, "/api/sounds/decompte", body, ct); w.Code != http.StatusBadRequest {
		t.Errorf("31 s > 30 s : attendu 400, got %d : %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Chemin 3 — son de question : POST /questions (champ `sound`)
// ---------------------------------------------------------------------------

func TestSoundUpload232_Question_NonCanonical_ConvertedFieldsInResponse(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	req := tw219hNewUploadRequest(t, tw219hUploadFields{
		number: "1", question: "Q", answer: "A", points: "1", time: "20",
		soundFileName: "s.wav", soundFileContent: tw232uWAV(1, 1, 48000, 24, 3),
	})
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	converted, src := tw232uDecode(t, w.Body.Bytes())
	if !converted || src != "48000Hz/1ch/24bit" {
		t.Errorf("converted=true / source_format=\"48000Hz/1ch/24bit\" attendus, got %v / %v", converted, src)
	}
	q := tw219hReadQuestion(t, dataDir, "1")
	if q.Sound == "" {
		t.Fatal("SOUND attendu après upload converti")
	}
	tw232uAssertStoredCanonical(t, filepath.Join(dataDir, "files", "questions", "1", filepath.Base(q.Sound)))
}

func TestSoundUpload232_Question_AlreadyCanonical_NotConverted(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	req := tw219hNewUploadRequest(t, tw219hUploadFields{
		number: "1", question: "Q", answer: "A", points: "1", time: "20",
		soundFileName: "s.wav", soundFileContent: tw219hCanonicalWAV(2),
	})
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if converted, src := tw232uDecode(t, w.Body.Bytes()); converted || src != nil {
		t.Errorf("déjà canonique ⇒ converted=false / source_format=null, got %v / %v", converted, src)
	}
}

func TestSoundUpload232_Question_NoSoundPart_ConvertedFalse(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	req := tw219hNewUploadRequest(t, tw219hUploadFields{number: "1", question: "Q", answer: "A", points: "1", time: "20"})
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if converted, src := tw232uDecode(t, w.Body.Bytes()); converted || src != nil {
		t.Errorf("sans fichier son ⇒ converted=false / source_format=null (champs toujours présents), got %v / %v", converted, src)
	}
}

func TestSoundUpload232_Question_MP3Refused400_NothingWritten(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	req := tw219hNewUploadRequest(t, tw219hUploadFields{
		number: "1", question: "Q", answer: "A", points: "1", time: "20",
		soundFileName: "faux.wav", soundFileContent: tw232uAssemble(0x55, 2, 44100, 16, make([]byte, 4000)),
	})
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("WAV compressé : attendu 400, got %d : %s", w.Code, w.Body.String())
	}
	if matches, _ := filepath.Glob(filepath.Join(dataDir, "files", "questions", "1", "sound_*.wav")); len(matches) != 0 {
		t.Errorf("aucun fichier son ne doit être écrit sur refus, got %v", matches)
	}
}

// Plafond brut d'un son de question = 4 × 6 Mio = 24 Mio ; 413 au-delà.
func TestSoundUpload232_Question_RawCap_4xCanonical_413(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	huge := tw232uAssemble(1, 2, 44100, 16, make([]byte, 25<<20))
	req := tw219hNewUploadRequest(t, tw219hUploadFields{
		number: "1", question: "Q", answer: "A", points: "1", time: "20",
		soundFileName: "enorme.wav", soundFileContent: huge,
	})
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("brut 25 Mio (> 4 × 6 Mio) : attendu 413, got %d", w.Code)
	}
}
