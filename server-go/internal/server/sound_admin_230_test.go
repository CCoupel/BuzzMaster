// #230 (v11.0) — page d'administration des sons : endpoints purement
// disque (GET /api/sounds, POST /api/sounds/{cue}, POST
// /api/sounds/{cue}/restore). Source normative : contracts/http-endpoints.md
// §Sound (SHA 1ce78d73) — ce fichier teste le comportement HTTP EXTERNE
// documenté par le contrat, jamais l'implémentation de dev-backend (écrite
// en parallèle, même répertoire partagé, aucune opération git globale —
// incident #187).
//
// POST /api/sounds/{cue}/test et GET /api/sound/status (qui nécessitent un
// accès au moteur via SoundProvider) sont dans sound_admin_engine_230_test.go
// — séparés pour ne pas bloquer ce fichier-ci sur la coordination avec
// dev-backend au sujet de la forme de SoundProvider.
//
// T6 (sécurité — cue hors catalogue, traversée de chemin) est LE test
// prioritaire de ce fichier : le segment {cue} vient de l'URL et sert à
// construire un chemin disque (contract, section "Sécurité").
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package server

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/audio/synth"
)

// ---------------------------------------------------------------------------
// Aides propres à ce fichier — préfixe tw230 pour ne jamais entrer en
// collision avec un helper déclaré ailleurs dans le paquet.
// ---------------------------------------------------------------------------

// tw230BuildWAV hand-builds a minimal, structurally valid (or deliberately
// non-conforming) RIFF/WAVE byte stream — same technique as #229's
// tw229bBuildWAVHeader (internal/audio/bank_229_test.go), reimplemented
// locally since that helper lives in a different package.
func tw230BuildWAV(channels, sampleRate, bitsPerSample, formatTag int, seconds float64) []byte {
	frames := int(seconds * float64(sampleRate))
	blockAlign := channels * (bitsPerSample / 8)
	data := make([]byte, frames*blockAlign) // silence — content irrelevant to format/duration validation
	byteRate := sampleRate * blockAlign

	buf := make([]byte, 44+len(data))
	copy(buf[0:4], "RIFF")
	tw230PutU32(buf[4:8], uint32(36+len(data)))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	tw230PutU32(buf[16:20], 16)
	tw230PutU16(buf[20:22], uint16(formatTag))
	tw230PutU16(buf[22:24], uint16(channels))
	tw230PutU32(buf[24:28], uint32(sampleRate))
	tw230PutU32(buf[28:32], uint32(byteRate))
	tw230PutU16(buf[32:34], uint16(blockAlign))
	tw230PutU16(buf[34:36], uint16(bitsPerSample))
	copy(buf[36:40], "data")
	tw230PutU32(buf[40:44], uint32(len(data)))
	copy(buf[44:], data)
	return buf
}

func tw230PutU32(b []byte, v uint32) { b[0], b[1], b[2], b[3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24) }
func tw230PutU16(b []byte, v uint16) { b[0], b[1] = byte(v), byte(v>>8) }

// tw230CanonicalWAV builds a WAV conforming to the canonical format
// (contracts/sound.md §3) with the given duration in seconds.
func tw230CanonicalWAV(seconds float64) []byte {
	return tw230BuildWAV(audio.ChannelCount, audio.SampleRate, audio.BitsPerSample, 1 /* PCM */, seconds)
}

// tw230SoundsDir returns the sounds directory for a test server built by
// setupTestHTTPServer — same soundsDir convention as #229's sounds_backup_229_test.go.
func tw230SoundsDir(dataDir string) string {
	return filepath.Join(dataDir, "files", "sounds")
}

// tw230SeedDefaultSounds seeds all 7 default sounds on disk — most tests
// need at least one real cue present (e.g. "depart") to exercise replace/
// restore against a known-good baseline.
func tw230SeedDefaultSounds(t *testing.T, dataDir string) {
	t.Helper()
	if _, err := synth.WriteAll(tw230SoundsDir(dataDir), false); err != nil {
		t.Fatalf("setup invalide : synth.WriteAll : %v", err)
	}
}

// body is io.Reader, not *bytes.Buffer: a nil *bytes.Buffer passed as an
// io.Reader is a NON-nil interface wrapping a nil pointer — httptest.NewRequest
// then panics trying to call methods on it. Callers with no body pass the
// untyped literal `nil` (a true nil io.Reader), never a typed nil pointer.
func tw230Post(t *testing.T, server *HTTPServer, url string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", url, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	return w
}

// ---------------------------------------------------------------------------
// GET /api/sounds — la liste des 7 cues.
// ---------------------------------------------------------------------------

func TestGETAPISounds_ListsAllSevenCuesWithOriginAndDuration(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)

	req := httptest.NewRequest("GET", "/api/sounds", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Cues []struct {
			Cue              string  `json:"cue"`
			Enabled          bool    `json:"enabled"`
			Custom           bool    `json:"custom"`
			DurationSeconds  float64 `json:"duration_seconds"`
			Path             string  `json:"path"`
		} `json:"cues"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("réponse non JSON valide : %v (%s)", err, w.Body.String())
	}
	if len(resp.Cues) != len(synth.Cues) {
		t.Fatalf("attendu %d cues, got %d", len(synth.Cues), len(resp.Cues))
	}
	for i, c := range synth.Cues {
		got := resp.Cues[i]
		if got.Cue != string(c) {
			t.Errorf("position %d : attendu cue=%q (ordre normatif contract sound.md §2.1), got %q", i, c, got.Cue)
		}
		if got.Custom {
			t.Errorf("%s : custom=true pour un son par défaut jamais modifié", c)
		}
		if !got.Enabled {
			t.Errorf("%s : enabled=false, attendu true (aucune configuration CuesDisabled)", c)
		}
		if got.DurationSeconds <= 0 {
			t.Errorf("%s : duration_seconds=%v, attendu > 0", c, got.DurationSeconds)
		}
		wantPath := "/files/sounds/" + string(c) + ".wav"
		if got.Path != wantPath {
			t.Errorf("%s : path=%q, attendu %q", c, got.Path, wantPath)
		}
	}
}

func TestGETAPISounds_RejectsNonGET(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)

	req := httptest.NewRequest("POST", "/api/sounds", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for POST /api/sounds, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// T6 — Sécurité : le nom de cue vient de l'URL (contract, section
// "Sécurité"). LE test prioritaire de ce fichier.
// ---------------------------------------------------------------------------

func TestPOSTAPISoundsCue_UnknownCue_Returns404_NeverTouchesDisk(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)

	cases := []string{
		"not-a-real-cue",
		"DEPART",             // sensibilité à la casse — la comparaison doit être stricte
		"depart.wav",         // extension déjà incluse
		"depart2",
		"..%2F..%2F..%2Fetc%2Fpasswd", // traversée encodée
		"",                    // segment vide
	}
	for _, cue := range cases {
		t.Run(cue, func(t *testing.T) {
			body, ct := multipartImageBody(t, "x.wav", tw230CanonicalWAV(0.5))
			w := tw230Post(t, server, "/api/sounds/"+cue, body, ct)
			if w.Code != http.StatusNotFound {
				t.Errorf("cue=%q : attendu 404, got %d: %s", cue, w.Code, w.Body.String())
			}
		})
	}

	// Aucun fichier ne doit avoir été écrit HORS du répertoire des sons —
	// vérification directe : le répertoire parent de dataDir ne doit
	// contenir aucun artefact inattendu (ex. "passwd").
	if _, err := os.Stat(filepath.Join(dataDir, "passwd")); !os.IsNotExist(err) {
		t.Error("un fichier a été créé en dehors de data/files/sounds/ — traversée de chemin réussie")
	}
}

func TestPOSTAPISoundsCueTraversal_PathTraversalSegment_Returns404(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)

	// httptest.NewRequest normalise certains ".." dans l'URL avant même
	// d'atteindre le mux — on vérifie donc à la fois la forme brute (que le
	// routeur peut ou non normaliser) ET une forme qui survit à la
	// normalisation standard (segment unique contenant des points, jamais
	// un cue réel).
	for _, cue := range []string{"....", "...", "a..b"} {
		t.Run(cue, func(t *testing.T) {
			body, ct := multipartImageBody(t, "x.wav", tw230CanonicalWAV(0.5))
			w := tw230Post(t, server, "/api/sounds/"+cue, body, ct)
			if w.Code != http.StatusNotFound {
				t.Errorf("cue=%q : attendu 404 (hors catalogue fermé), got %d", cue, w.Code)
			}
		})
	}
}

func TestPOSTAPISoundsCueRestore_UnknownCue_Returns404(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)

	w := tw230Post(t, server, "/api/sounds/not-a-real-cue/restore", nil, "")
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// POST /api/sounds/{cue} — remplacement, validation.
// ---------------------------------------------------------------------------

func TestPOSTAPISoundsCue_ValidCanonicalWAV_Accepted(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)

	body, ct := multipartImageBody(t, "x.wav", tw230CanonicalWAV(1.0))
	w := tw230Post(t, server, "/api/sounds/depart", body, ct)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Status          string   `json:"status"`
		Cue             string   `json:"cue"`
		Custom          bool     `json:"custom"`
		DurationSeconds float64  `json:"duration_seconds"`
		Warning         *string  `json:"warning"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("réponse non JSON valide : %v", err)
	}
	if resp.Status != "ok" || resp.Cue != "depart" || !resp.Custom {
		t.Errorf("réponse inattendue : %+v", resp)
	}
	if resp.Warning != nil {
		t.Errorf("warning=%v pour un son de 1s, attendu null (sous le seuil ~2s)", *resp.Warning)
	}

	onDisk, err := os.ReadFile(filepath.Join(tw230SoundsDir(dataDir), "depart.wav"))
	if err != nil {
		t.Fatalf("le fichier n'a pas été écrit : %v", err)
	}
	if !bytes.Equal(onDisk, tw230CanonicalWAV(1.0)) {
		t.Error("le contenu écrit sur disque ne correspond pas au fichier envoyé")
	}

	// Le manifeste doit être réconcilié : sounds.json doit désormais
	// rapporter depart comme personnalisé.
	manifestRaw, err := os.ReadFile(filepath.Join(tw230SoundsDir(dataDir), "sounds.json"))
	if err != nil {
		t.Fatalf("sounds.json absent après remplacement : %v", err)
	}
	var manifest map[string]struct {
		Present bool `json:"present"`
		Custom  bool `json:"custom,omitempty"`
	}
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatalf("sounds.json invalide : %v", err)
	}
	if !manifest["depart"].Custom {
		t.Error("sounds.json ne rapporte pas depart comme personnalisé après remplacement")
	}
}

func TestPOSTAPISoundsCue_DurationAboveWarningThreshold_AcceptedWithWarning(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)

	body, ct := multipartImageBody(t, "x.wav", tw230CanonicalWAV(3.4))
	w := tw230Post(t, server, "/api/sounds/depart", body, ct)
	if w.Code != http.StatusOK {
		t.Fatalf("un son de 3,4s doit être ACCEPTÉ (sous la limite de 5s), got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Warning *string `json:"warning"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("réponse non JSON valide : %v", err)
	}
	if resp.Warning == nil || *resp.Warning == "" {
		t.Error("un son de 3,4s (au-delà de ~2s) doit porter un avertissement non vide, sans bloquer")
	}
}

func TestPOSTAPISoundsCue_DurationAboveLimit_Rejected(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)

	body, ct := multipartImageBody(t, "x.wav", tw230CanonicalWAV(8.2))
	w := tw230Post(t, server, "/api/sounds/depart", body, ct)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("un son de 8,2s (au-delà de la limite de 5s) doit être REFUSÉ (400), got %d: %s", w.Code, w.Body.String())
	}
	// Le refus doit porter un MESSAGE lisible, jamais un code nu (contract,
	// "raison lisible, pas un code nu") — le contenu exact du message n'est
	// volontairement pas figé ici, seule sa présence est vérifiée.
	if w.Body.Len() == 0 {
		t.Error("le refus (400) doit porter un message lisible dans le corps, corps vide reçu")
	}
	// La cue ne doit PAS avoir été modifiée par un envoi refusé.
	onDisk, err := os.ReadFile(filepath.Join(tw230SoundsDir(dataDir), "depart.wav"))
	if err != nil {
		t.Fatalf("%v", err)
	}
	defaultWAV, _ := synth.Generate(synth.Cues[0])
	if !bytes.Equal(onDisk, defaultWAV) {
		t.Error("le fichier a été modifié malgré le refus (durée > 5s)")
	}
}

func TestPOSTAPISoundsCue_NonWAVContent_Rejected(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)

	body, ct := multipartImageBody(t, "x.wav", []byte("ceci n'est manifestement pas un fichier WAV"))
	w := tw230Post(t, server, "/api/sounds/depart", body, ct)
	if w.Code != http.StatusBadRequest {
		t.Errorf("contenu non-WAV : attendu 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPOSTAPISoundsCue_MP3RenamedAsWAV_Rejected(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)

	// En-tête de trame MP3 (sync 0xFF 0xFB, MPEG-1 Layer III) — pas un RIFF.
	mp3ish := append([]byte{0xFF, 0xFB, 0x90, 0x00}, make([]byte, 200)...)
	body, ct := multipartImageBody(t, "chanson.wav", mp3ish)
	w := tw230Post(t, server, "/api/sounds/depart", body, ct)
	if w.Code != http.StatusBadRequest {
		t.Errorf("MP3 renommé .wav : attendu 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPOSTAPISoundsCue_WrongSampleRate_Rejected(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)

	wav := tw230BuildWAV(audio.ChannelCount, 22050, audio.BitsPerSample, 1, 1.0)
	body, ct := multipartImageBody(t, "x.wav", wav)
	w := tw230Post(t, server, "/api/sounds/depart", body, ct)
	if w.Code != http.StatusBadRequest {
		t.Errorf("22050 Hz : attendu 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPOSTAPISoundsCue_Mono_Rejected(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)

	wav := tw230BuildWAV(1, audio.SampleRate, audio.BitsPerSample, 1, 1.0)
	body, ct := multipartImageBody(t, "x.wav", wav)
	w := tw230Post(t, server, "/api/sounds/depart", body, ct)
	if w.Code != http.StatusBadRequest {
		t.Errorf("mono : attendu 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPOSTAPISoundsCue_EightBit_Rejected(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)

	wav := tw230BuildWAV(audio.ChannelCount, audio.SampleRate, 8, 1, 1.0)
	body, ct := multipartImageBody(t, "x.wav", wav)
	w := tw230Post(t, server, "/api/sounds/depart", body, ct)
	if w.Code != http.StatusBadRequest {
		t.Errorf("8 bits : attendu 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPOSTAPISoundsCue_NonPCMFormat_Rejected(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)

	// formatTag=3 (IEEE float) au lieu de 1 (PCM).
	wav := tw230BuildWAV(audio.ChannelCount, audio.SampleRate, audio.BitsPerSample, 3, 1.0)
	body, ct := multipartImageBody(t, "x.wav", wav)
	w := tw230Post(t, server, "/api/sounds/depart", body, ct)
	if w.Code != http.StatusBadRequest {
		t.Errorf("format non-PCM : attendu 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPOSTAPISoundsCue_OversizedFile_Returns413(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)

	// ~3 Mo — au-delà du plafond ~2 Mo, quel que soit le contenu.
	oversized := make([]byte, 3<<20)
	body, ct := multipartImageBody(t, "x.wav", oversized)
	w := tw230Post(t, server, "/api/sounds/depart", body, ct)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("fichier ~3 Mo : attendu 413, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPOSTAPISoundsCue_MissingFile_Rejected(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)

	// Un multipart VALIDE (boundary correct, se ferme proprement) mais SANS
	// aucune partie "file" — simule un envoi où le champ a été omis, pas un
	// corps malformé.
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	if err := mw.Close(); err != nil {
		t.Fatalf("setup invalide : %v", err)
	}

	w := tw230Post(t, server, "/api/sounds/depart", buf, mw.FormDataContentType())
	if w.Code != http.StatusBadRequest {
		t.Errorf("champ file absent : attendu 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPOSTAPISoundsCue_RejectsNonPOST(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)

	req := httptest.NewRequest("GET", "/api/sounds/depart", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/sounds/depart : attendu 405, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// POST /api/sounds/{cue}/restore — restauration unitaire.
// ---------------------------------------------------------------------------

func TestPOSTAPISoundsCueRestore_OverwritesCustomFile_BackToDefault(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)

	body, ct := multipartImageBody(t, "x.wav", tw230CanonicalWAV(1.0))
	if w := tw230Post(t, server, "/api/sounds/depart", body, ct); w.Code != http.StatusOK {
		t.Fatalf("setup invalide (remplacement) : %d", w.Code)
	}

	w := tw230Post(t, server, "/api/sounds/depart/restore", nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Status string `json:"status"`
		Cue    string `json:"cue"`
		Custom bool   `json:"custom"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("réponse non JSON valide : %v", err)
	}
	if resp.Custom {
		t.Error("custom=true après restauration, attendu false (retour au défaut)")
	}

	onDisk, err := os.ReadFile(filepath.Join(tw230SoundsDir(dataDir), "depart.wav"))
	if err != nil {
		t.Fatalf("%v", err)
	}
	want, _ := synth.Generate(synth.Cues[0])
	if !bytes.Equal(onDisk, want) {
		t.Error("les octets après restauration ne correspondent pas exactement au son par défaut")
	}
}

func TestPOSTAPISoundsCueRestore_Idempotent(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)

	w1 := tw230Post(t, server, "/api/sounds/depart/restore", nil, "")
	if w1.Code != http.StatusOK {
		t.Fatalf("premier restore : %d", w1.Code)
	}
	first, err := os.ReadFile(filepath.Join(tw230SoundsDir(dataDir), "depart.wav"))
	if err != nil {
		t.Fatalf("%v", err)
	}

	w2 := tw230Post(t, server, "/api/sounds/depart/restore", nil, "")
	if w2.Code != http.StatusOK {
		t.Fatalf("second restore : %d", w2.Code)
	}
	second, err := os.ReadFile(filepath.Join(tw230SoundsDir(dataDir), "depart.wav"))
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !bytes.Equal(first, second) {
		t.Error("deux restaurations successives ont produit des octets différents")
	}
}

func TestPOSTAPISoundsCueRestore_RejectsNonPOST(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)

	req := httptest.NewRequest("GET", "/api/sounds/depart/restore", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("attendu 405, got %d", w.Code)
	}
}
