// #287 (v11.4.0, LOT-3) — HTTP du son de décompte : GET /api/sounds (tableau additif `long`), POST
// /api/sounds/decompte[/restore|/test], restore-defaults (7 + 1), réglages countdown_* via /config.json.
// Sources normatives : contracts/http-endpoints.md §Sound (LOT-3), contracts/sound.md §12. Boîte noire HTTP,
// réutilise tw230*/tw286*/setupTestHTTPServer (même paquet). Préfixe tw287.
//
// Écrit AVANT le code (TDD), compile avant lui (aucun symbole nouveau typé ; le rappel est cherché par réflexion).
package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/audio/synth"
)

// tw287Provider : SoundProvider + extension décompte (duck-typing : CountdownOutputAvailable / TestCountdown).
type tw287Provider struct {
	enabled, outputAvailable, testAccepted bool
	countdownTests                         int
	cueTests                               []audio.Cue
}

func (p *tw287Provider) SoundEnabled() bool         { return p.enabled }
func (p *tw287Provider) SoundOutputAvailable() bool { return p.outputAvailable }
func (p *tw287Provider) TestSoundCue(c audio.Cue) bool {
	p.cueTests = append(p.cueTests, c)
	return true
}
func (p *tw287Provider) CountdownOutputAvailable() bool { return p.outputAvailable }
func (p *tw287Provider) TestCountdown() bool            { p.countdownTests++; return p.testAccepted }

type tw287LongEntry struct {
	Name            string   `json:"name"`
	Kind            string   `json:"kind"`
	Enabled         bool     `json:"enabled"`
	Loop            bool     `json:"loop"`
	Custom          bool     `json:"custom"`
	DurationSeconds *float64 `json:"duration_seconds"`
	Path            string   `json:"path"`
}

func tw287GetSounds(t *testing.T, s *HTTPServer) (raw []byte, cues []map[string]interface{}, long []tw287LongEntry) {
	t.Helper()
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/sounds", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/sounds : attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	var resp struct {
		Cues []map[string]interface{} `json:"cues"`
		Long []tw287LongEntry         `json:"long"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("réponse non JSON : %v", err)
	}
	return w.Body.Bytes(), resp.Cues, resp.Long
}

func tw287Seeded(t *testing.T) (*HTTPServer, string) {
	t.Helper()
	s, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	return s, dataDir
}

func tw287DecompteWAV(t *testing.T, dataDir string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(tw230SoundsDir(dataDir), "decompte.wav"))
	if err != nil {
		t.Fatalf("decompte.wav illisible : %v", err)
	}
	return b
}

// ---------- GET /api/sounds ----------

func TestSoundsList287_LongArray_DefaultDecompteEntry(t *testing.T) {
	s, _ := tw287Seeded(t)
	_, cues, long := tw287GetSounds(t, s)

	if len(cues) != len(synth.Cues) {
		t.Fatalf("`cues` doit rester à %d entrées (byte-identique à LOT-2), got %d", len(synth.Cues), len(cues))
	}
	for i, c := range cues {
		if len(c) != 5 {
			t.Errorf("cue %d : %d clés (%v), attendu exactement cue/enabled/custom/duration_seconds/path", i, len(c), c)
		}
	}
	if len(long) != 1 {
		t.Fatalf("`long` doit contenir exactement 1 entrée (decompte), got %d", len(long))
	}
	d := long[0]
	if d.Name != "decompte" || d.Kind != "countdown" || d.Path != "/files/sounds/decompte.wav" {
		t.Errorf("entrée long[0] = %+v, attendu name=decompte kind=countdown path=/files/sounds/decompte.wav", d)
	}
	if !d.Enabled || !d.Loop || d.Custom {
		t.Errorf("défaut : enabled=true, loop=true, custom=false ; got %+v", d)
	}
	if d.DurationSeconds == nil || *d.DurationSeconds < 1.99 || *d.DurationSeconds > 2.01 {
		t.Errorf("duration_seconds = %v, attendu 2.0 (tic-tac par défaut)", d.DurationSeconds)
	}
}

func TestSoundsList287_LongEnabledAndLoopFollowConfig(t *testing.T) {
	s, _ := tw287Seeded(t)
	tw286PostConfig(t, s, `{"sound":{"countdown_disabled":true,"countdown_once":true}}`)
	_, _, long := tw287GetSounds(t, s)
	if len(long) != 1 || long[0].Enabled || long[0].Loop {
		t.Fatalf("countdown_disabled+countdown_once ⇒ enabled=false, loop=false ; got %+v", long)
	}
	tw286PostConfig(t, s, `{"sound":{"countdown_disabled":false,"countdown_once":false}}`)
	_, _, long = tw287GetSounds(t, s)
	if !long[0].Enabled || !long[0].Loop {
		t.Fatalf("retour aux défauts : enabled=true, loop=true ; got %+v", long)
	}
}

func TestSoundsList287_MissingFile_DurationNull_StillListed(t *testing.T) {
	s, dataDir := tw287Seeded(t)
	if err := os.Remove(filepath.Join(tw230SoundsDir(dataDir), "decompte.wav")); err != nil {
		t.Fatalf("setup invalide : %v", err)
	}
	_, _, long := tw287GetSounds(t, s)
	if len(long) != 1 || long[0].Custom || long[0].DurationSeconds != nil {
		t.Fatalf("fichier absent : entrée listée, custom=false, duration_seconds=null ; got %+v", long)
	}
}

// ---------- POST /api/sounds/decompte ----------

func TestDecompteUpload287_Valid3sStereo_Accepted_CustomTrue(t *testing.T) {
	s, dataDir := tw287Seeded(t)
	body, ct := multipartImageBody(t, "d.wav", tw230CanonicalWAV(3))
	w := tw230Post(t, s, "/api/sounds/decompte", body, ct)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	var resp struct {
		Status          string  `json:"status"`
		Custom          bool    `json:"custom"`
		DurationSeconds float64 `json:"duration_seconds"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Status != "ok" || !resp.Custom || resp.DurationSeconds < 2.9 || resp.DurationSeconds > 3.1 {
		t.Errorf("réponse = %+v, attendu ok/custom=true/~3 s", resp)
	}
	_, _, long := tw287GetSounds(t, s)
	if !long[0].Custom {
		t.Errorf("GET /api/sounds : custom=true attendu après remplacement, got %+v", long[0])
	}
	if got := tw287DecompteWAV(t, dataDir); !bytes.Equal(got[:4], []byte("RIFF")) {
		t.Errorf("decompte.wav stocké n'est pas un WAV")
	}
}

func TestDecompteUpload287_30sLimit_31sRejected400_30sAccepted(t *testing.T) {
	s, _ := tw287Seeded(t)
	body, ct := multipartImageBody(t, "d.wav", tw230CanonicalWAV(31))
	if w := tw230Post(t, s, "/api/sounds/decompte", body, ct); w.Code != http.StatusBadRequest {
		t.Errorf("31 s : attendu 400 (limite 30 s, pas 5 s des cues), got %d : %s", w.Code, w.Body.String())
	}
	body, ct = multipartImageBody(t, "d.wav", tw230CanonicalWAV(29))
	if w := tw230Post(t, s, "/api/sounds/decompte", body, ct); w.Code != http.StatusOK {
		t.Errorf("29 s : attendu 200 (au-delà des 5 s des cues, ValidateQuestionSound), got %d : %s", w.Code, w.Body.String())
	}
}

func TestDecompteUpload287_Over6MiB_Returns413(t *testing.T) {
	s, _ := tw287Seeded(t)
	// #232 : plafond d'entrée brute = 4 × 6 Mio = 24 Mio ; un vrai WAV de 25 Mio ⇒ 413.
	body, ct := multipartImageBody(t, "d.wav", tw232uAssemble(1, 2, 44100, 16, make([]byte, 25<<20)))
	if w := tw230Post(t, s, "/api/sounds/decompte", body, ct); w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("> 4 × 6 Mio : attendu 413, got %d : %s", w.Code, w.Body.String())
	}
}

func TestDecompteUpload287_Mono_AcceptedAndStoredStereo(t *testing.T) {
	s, dataDir := tw287Seeded(t)
	mono := tw230BuildWAV(1, audio.SampleRate, audio.BitsPerSample, 1, 2.0)
	body, ct := multipartImageBody(t, "d.wav", mono)
	w := tw230Post(t, s, "/api/sounds/decompte", body, ct)
	if w.Code != http.StatusOK {
		t.Fatalf("mono : attendu 200 (accepté, converti), got %d : %s", w.Code, w.Body.String())
	}
	stored := tw287DecompteWAV(t, dataDir)
	if ch := binary.LittleEndian.Uint16(stored[22:24]); ch != 2 {
		t.Errorf("fichier stocké : %d canaux, attendu 2 (stéréo canonique, sound.md §10.4)", ch)
	}
}

func TestDecompteUpload287_NonWAV_Rejected400_AndFileUntouched(t *testing.T) {
	s, dataDir := tw287Seeded(t)
	before := tw287DecompteWAV(t, dataDir)
	body, ct := multipartImageBody(t, "d.wav", []byte("ID3 pas un wav du tout"))
	if w := tw230Post(t, s, "/api/sounds/decompte", body, ct); w.Code != http.StatusBadRequest {
		t.Errorf("non WAV : attendu 400, got %d", w.Code)
	}
	if !bytes.Equal(before, tw287DecompteWAV(t, dataDir)) {
		t.Error("un rejet ne doit jamais toucher le fichier existant")
	}
}

func TestDecompteUpload287_RejectsNonPOST(t *testing.T) {
	s, _ := tw287Seeded(t)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/sounds/decompte", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/sounds/decompte : attendu 405, got %d", w.Code)
	}
}

// ---------- restore ----------

func TestDecompteRestore287_BackToDefault_CustomFalse_Idempotent(t *testing.T) {
	s, dataDir := tw287Seeded(t)
	defaultBytes := tw287DecompteWAV(t, dataDir) // octets du défaut synthétisé au démarrage
	body, ct := multipartImageBody(t, "d.wav", tw230CanonicalWAV(3))
	tw230Post(t, s, "/api/sounds/decompte", body, ct)

	for i := 0; i < 2; i++ {
		w := tw230Post(t, s, "/api/sounds/decompte/restore", nil, "")
		if w.Code != http.StatusOK {
			t.Fatalf("restore #%d : attendu 200, got %d : %s", i, w.Code, w.Body.String())
		}
		var resp struct {
			Status          string  `json:"status"`
			Custom          bool    `json:"custom"`
			DurationSeconds float64 `json:"duration_seconds"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.Status != "ok" || resp.Custom || resp.DurationSeconds < 1.99 || resp.DurationSeconds > 2.01 {
			t.Errorf("restore #%d : %+v, attendu ok/custom=false/2.0", i, resp)
		}
	}
	if !bytes.Equal(tw287DecompteWAV(t, dataDir), defaultBytes) {
		t.Error("restore : octets différents du défaut synthétisé")
	}
}

// ---------- test sur l'enceinte ----------

func tw287Test(t *testing.T, s *HTTPServer) (int, string) {
	t.Helper()
	w := tw230Post(t, s, "/api/sounds/decompte/test", nil, "")
	var resp struct {
		Result string `json:"result"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return w.Code, resp.Result
}

func TestDecompteTest287_Played(t *testing.T) {
	s, _ := tw287Seeded(t)
	p := &tw287Provider{enabled: true, outputAvailable: true, testAccepted: true}
	s.Sound = p
	code, result := tw287Test(t, s)
	if code != http.StatusOK || result != "played" || p.countdownTests != 1 {
		t.Errorf("code=%d result=%q appels=%d, attendu 200/played/1", code, result, p.countdownTests)
	}
	if len(p.cueTests) != 0 {
		t.Errorf("le test du décompte ne passe jamais par TestSoundCue : %v", p.cueTests)
	}
}

func TestDecompteTest287_DisabledWhenSoundEnabledFalse_NoPlayerCall(t *testing.T) {
	s, _ := tw287Seeded(t)
	p := &tw287Provider{enabled: false, outputAvailable: true, testAccepted: true}
	s.Sound = p
	_, result := tw287Test(t, s)
	if result != "disabled" || p.countdownTests != 0 {
		t.Errorf("result=%q appels=%d, attendu disabled et aucun appel au lecteur", result, p.countdownTests)
	}
}

func TestDecompteTest287_Unavailable_WhenNoRealOutput(t *testing.T) {
	s, _ := tw287Seeded(t)
	s.Sound = &tw287Provider{enabled: true, outputAvailable: false}
	if _, result := tw287Test(t, s); result != "unavailable" {
		t.Errorf("result=%q, attendu unavailable", result)
	}
}

func TestDecompteTest287_IgnoresMutedAndCountdownDisabled(t *testing.T) {
	s, _ := tw287Seeded(t)
	p := &tw287Provider{enabled: true, outputAvailable: true, testAccepted: true}
	s.Sound = p
	tw286PostConfig(t, s, `{"sound":{"muted":true,"countdown_disabled":true}}`)
	if _, result := tw287Test(t, s); result != "played" || p.countdownTests != 1 {
		t.Errorf("geste explicite : doit jouer malgré muted/countdown_disabled (result=%q, appels=%d)", result, p.countdownTests)
	}
}

// ---------- le décompte n'est pas une cue ----------

func TestDecompte287_IsNotACue_ClosedCatalogueUnchanged(t *testing.T) {
	if _, ok := soundCueFromString("decompte"); ok {
		t.Error("soundCueFromString(\"decompte\") doit rester faux : le décompte n'est pas une cue (sound.md §12.1)")
	}
	if len(synth.Cues) != 7 {
		t.Errorf("le catalogue de cues reste fermé à 7, got %d", len(synth.Cues))
	}
}

func TestDecompte287_CuePathsStillRefuseUnknownSegments(t *testing.T) {
	s, _ := tw287Seeded(t)
	for _, u := range []string{"/api/sounds/decompte2", "/api/sounds/../decompte", "/api/sounds/decompte/inconnu"} {
		w := tw230Post(t, s, u, nil, "")
		if w.Code == http.StatusOK {
			t.Errorf("POST %s : ne doit pas réussir (got 200)", u)
		}
	}
}

// ---------- restore-defaults : 7 + 1 ----------

func TestRestoreDefaults287_RestoresEightIncludingDecompte(t *testing.T) {
	s, dataDir := tw287Seeded(t)
	defaultBytes := tw287DecompteWAV(t, dataDir)
	body, ct := multipartImageBody(t, "d.wav", tw230CanonicalWAV(3))
	tw230Post(t, s, "/api/sounds/decompte", body, ct)

	w := tw230Post(t, s, "/api/sounds/restore-defaults", nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	var resp struct {
		Written  []string `json:"written"`
		Restored []string `json:"restored"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Written) != 7 {
		t.Errorf("written = %v, attendu les 7 cues (forme inchangée)", resp.Written)
	}
	if len(resp.Restored) != 8 || !tw287Contains(resp.Restored, "decompte") {
		t.Errorf("restored = %v, attendu 8 noms dont decompte", resp.Restored)
	}
	if !bytes.Equal(tw287DecompteWAV(t, dataDir), defaultBytes) {
		t.Error("restore-defaults doit aussi réécrire decompte.wav")
	}
}

func tw287Contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// ---------- /config.json : countdown_disabled / countdown_once ----------

func TestCountdownConfig287_AbsentKeys_DefaultActiveLoop_OmittedFromSection(t *testing.T) {
	s, _ := tw287Seeded(t)
	sec := tw286GetConfigSound(t, s)
	if v, ok := sec["countdown_disabled"]; ok && v != false {
		t.Errorf("countdown_disabled = %v par défaut, attendu absent/false (actif)", v)
	}
	if v, ok := sec["countdown_once"]; ok && v != false {
		t.Errorf("countdown_once = %v par défaut, attendu absent/false (en boucle)", v)
	}
}

func TestCountdownConfig287_RoundTrip_AndMergeKeepsOtherSoundFields(t *testing.T) {
	s, _ := tw287Seeded(t)
	tw286PostConfig(t, s, `{"sound":{"muted":true,"cues_disabled":{"depart":true}}}`)
	tw286PostConfig(t, s, `{"sound":{"countdown_once":true}}`)
	sec := tw286GetConfigSound(t, s)
	if sec["countdown_once"] != true {
		t.Errorf("countdown_once non persisté : %v", sec)
	}
	if sec["muted"] != true {
		t.Errorf("countdown_once a écrasé muted : %v", sec["muted"])
	}
	if cd, _ := sec["cues_disabled"].(map[string]interface{}); cd == nil || cd["depart"] != true {
		t.Errorf("countdown_once a écrasé cues_disabled : %v", sec["cues_disabled"])
	}
	tw286PostConfig(t, s, `{"sound":{"countdown_disabled":true}}`)
	sec = tw286GetConfigSound(t, s)
	if sec["countdown_disabled"] != true || sec["countdown_once"] != true {
		t.Errorf("fusion champ par champ : les deux réglages doivent coexister, got %v", sec)
	}
}

func TestCountdownConfig287_PostConfig_CallsOnSoundCountdownDisabledChange_OnlyOnChange(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	field := reflect.ValueOf(s).Elem().FieldByName("OnSoundCountdownDisabledChange")
	if !field.IsValid() {
		t.Fatal("HTTPServer.OnSoundCountdownDisabledChange absent : rappel de sound.md §12.3 non livré")
	}
	var calls []bool
	cb := func(disabled bool) { calls = append(calls, disabled) }
	if !reflect.TypeOf(cb).AssignableTo(field.Type()) {
		t.Fatalf("OnSoundCountdownDisabledChange doit être func(bool), got %s", field.Type())
	}
	field.Set(reflect.ValueOf(cb))

	tw286PostConfig(t, s, `{"sound":{"countdown_disabled":true}}`)
	if len(calls) != 1 || !calls[0] {
		t.Fatalf("false→true : un appel (true) attendu, got %v", calls)
	}
	tw286PostConfig(t, s, `{"sound":{"countdown_disabled":true}}`)
	tw286PostConfig(t, s, `{"sound":{"countdown_once":true}}`) // un changement de boucle ne touche pas le décompte en cours
	if len(calls) != 1 {
		t.Errorf("valeur inchangée / countdown_once : aucun nouvel appel attendu, got %v", calls)
	}
	// true→false : le contrat n'exige que l'arrêt à true ; un éventuel appel (false) est toléré, jamais (true).
	tw286PostConfig(t, s, `{"sound":{"countdown_disabled":false}}`)
	for _, c := range calls[1:] {
		if c {
			t.Errorf("true→false : aucun appel (true) attendu, got %v", calls)
		}
	}
}

func TestCountdownConfig287_Persisted_SurvivesReload(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	tw286PostConfig(t, s, `{"sound":{"countdown_once":true,"countdown_disabled":true}}`)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, httptest.NewRequest("GET", "/config.json", nil))
	if !strings.Contains(w.Body.String(), "countdown_once") || !strings.Contains(w.Body.String(), "countdown_disabled") {
		t.Errorf("GET /config.json doit exposer les deux réglages une fois posés : %s", w.Body.String())
	}
}
