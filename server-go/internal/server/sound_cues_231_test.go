// #231 (v11.6.0, LOT-3) — API des bruitages à 15 cues : GET /api/sounds (champ `type`, ordre global puis par type),
// POST /api/sounds/{cue}, /restore, /test sur une nouvelle cue, restore-defaults (15 + décompte), cues_disabled,
// cue inconnue toujours refusée. Source : contracts/http-endpoints.md « GET /api/sounds — cues par type (#231, LOT-3) »,
// sound.md §2.1/§6.3. Réutilise les helpers tw230*. Préfixe tw231h.
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/audio/synth"
	"buzzcontrol/internal/config"
)

var tw231hOrder = []struct{ cue, typ string }{
	{"depart", ""}, {"temps-ecoule", ""}, {"gagne", ""}, {"perdu", ""}, {"reveal", ""}, {"entracte-debut", ""}, {"entracte-fin", ""},
	{"qcm-indice", "QCM"}, {"memory-carte", "MEMORY"}, {"memory-paire", "MEMORY"},
	{"memotion-selection", "MEMOTION"}, {"memotion-fin-carte", "MEMOTION"},
	{"rafale-bonne", "RAFALE"}, {"rafale-mauvaise", "RAFALE"}, {"rafale-fin-lot", "RAFALE"},
}

func tw231hGetSounds(t *testing.T, s *HTTPServer) (cues []map[string]interface{}, long []interface{}) {
	t.Helper()
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/sounds", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	var resp struct {
		Cues []map[string]interface{} `json:"cues"`
		Long []interface{}            `json:"long"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("réponse non JSON : %v", err)
	}
	return resp.Cues, resp.Long
}

func TestSoundsAPI231_List_FifteenCuesWithType_InNormativeOrder(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	cues, _ := tw231hGetSounds(t, s)
	if len(cues) != 15 {
		t.Fatalf("15 cues attendues, got %d", len(cues))
	}
	for i, w := range tw231hOrder {
		if cues[i]["cue"] != w.cue {
			t.Errorf("position %d : attendu %q, got %v (global puis QCM, MEMORY, MEMOTION, RAFALE)", i, w.cue, cues[i]["cue"])
		}
		typ, present := cues[i]["type"]
		if !present {
			t.Errorf("%s : champ `type` absent (jamais omis — \"\" pour les cues globales)", w.cue)
			continue
		}
		if typ != w.typ {
			t.Errorf("%s : type attendu %q, got %v", w.cue, w.typ, typ)
		}
	}
}

func TestSoundsAPI231_List_NewCuesHaveDefaultFilesPathAndDuration(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	cues, _ := tw231hGetSounds(t, s)
	for _, c := range cues[7:] {
		name := c["cue"].(string)
		if c["custom"] != false {
			t.Errorf("%s : custom=false attendu (son par défaut), got %v", name, c["custom"])
		}
		if c["enabled"] != true {
			t.Errorf("%s : enabled=true attendu, got %v", name, c["enabled"])
		}
		if c["path"] != "/files/sounds/"+name+".wav" {
			t.Errorf("%s : path inattendu %v", name, c["path"])
		}
		if d, _ := c["duration_seconds"].(float64); d <= 0 {
			t.Errorf("%s : duration_seconds > 0 attendu, got %v", name, c["duration_seconds"])
		}
	}
}

func TestSoundsAPI231_List_DecompteStaysInLong_NotInCues(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	cues, long := tw231hGetSounds(t, s)
	for _, c := range cues {
		if c["cue"] == "decompte" {
			t.Error("le décompte reste dans `long[]`, jamais dans `cues[]`")
		}
	}
	if len(long) != 1 {
		t.Errorf("`long` inchangé : 1 entrée (decompte), got %d", len(long))
	}
}

func TestSoundsAPI231_CuesDisabled_FiltersNewCues(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	if err := json.Unmarshal([]byte(`{"sound":{"cues_disabled":{"rafale-bonne":true,"memory-carte":true}}}`), config.Get()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { config.Get().Sound.CuesDisabled = nil })
	cues, _ := tw231hGetSounds(t, s)
	for _, c := range cues {
		want := !(c["cue"] == "rafale-bonne" || c["cue"] == "memory-carte")
		if c["enabled"] != want {
			t.Errorf("%v : enabled=%v attendu, got %v", c["cue"], want, c["enabled"])
		}
	}
}

// ---------------------------------------------------------------------------
// POST /api/sounds/{cue}, /restore, /test sur une nouvelle cue — mêmes règles que les 7 historiques.
// ---------------------------------------------------------------------------

func TestSoundsAPI231_Replace_NewCue_WritesFile_AndListSaysCustom(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	body, ct := multipartImageBody(t, "bonne.wav", tw230CanonicalWAV(0.6))
	w := tw230Post(t, s, "/api/sounds/rafale-bonne", body, ct)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(tw230SoundsDir(dataDir), "rafale-bonne.wav")); err != nil {
		t.Errorf("fichier absent : %v", err)
	}
	cues, _ := tw231hGetSounds(t, s)
	for _, c := range cues {
		if c["cue"] == "rafale-bonne" && c["custom"] != true {
			t.Errorf("rafale-bonne : custom=true attendu après remplacement, got %v", c["custom"])
		}
		if c["cue"] == "rafale-mauvaise" && c["custom"] != false {
			t.Errorf("rafale-mauvaise ne doit pas être touchée, custom=%v", c["custom"])
		}
	}
}

func TestSoundsAPI231_RestoreOne_NewCue_RegeneratesDefault(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	body, ct := multipartImageBody(t, "x.wav", tw230CanonicalWAV(0.7))
	tw230Post(t, s, "/api/sounds/memory-paire", body, ct)
	w := tw230Post(t, s, "/api/sounds/memory-paire/restore", nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	got, _ := os.ReadFile(filepath.Join(tw230SoundsDir(dataDir), "memory-paire.wav"))
	want, _ := synth.Generate("memory-paire")
	if string(got) != string(want) {
		t.Error("restore : le son par défaut synthétisé doit être réécrit")
	}
}

func TestSoundsAPI231_TestEndpoint_NewCue_Played(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	fake := &tw230eFakeSoundProvider{enabled: true, outputAvailable: true, testAccepted: true}
	s.Sound = fake
	w := tw230Post(t, s, "/api/sounds/qcm-indice/test", nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if len(fake.testCalls) != 1 || fake.testCalls[0] != audio.Cue("qcm-indice") {
		t.Errorf("TestSoundCue doit être appelé une fois avec qcm-indice, got %v", fake.testCalls)
	}
}

// Tester joue même si la cue est désactivée (geste explicite, §6.3) — vaut aussi pour les nouvelles cues.
func TestSoundsAPI231_TestEndpoint_NewCueDisabled_StillTestable(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	_ = json.Unmarshal([]byte(`{"sound":{"cues_disabled":{"rafale-fin-lot":true}}}`), config.Get())
	t.Cleanup(func() { config.Get().Sound.CuesDisabled = nil })
	fake := &tw230eFakeSoundProvider{enabled: true, outputAvailable: true, testAccepted: true}
	s.Sound = fake
	if w := tw230Post(t, s, "/api/sounds/rafale-fin-lot/test", nil, ""); w.Code != http.StatusOK || len(fake.testCalls) != 1 {
		t.Errorf("cue désactivée mais testable : 200 + 1 appel attendus, got %d / %v", w.Code, fake.testCalls)
	}
}

func TestSoundsAPI231_UnknownCue_StillRefused404(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	for _, cue := range []string{"qcm", "memory", "rafale-bonne2", "RAFALE-BONNE", "memory-carte.wav", "decompte2"} {
		body, ct := multipartImageBody(t, "x.wav", tw230CanonicalWAV(0.5))
		if w := tw230Post(t, s, "/api/sounds/"+cue, body, ct); w.Code != http.StatusNotFound {
			t.Errorf("cue %q hors catalogue : 404 attendu, got %d", cue, w.Code)
		}
	}
}

func TestSoundsAPI231_RestoreDefaults_RestoresAllFifteenAndCountdown(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw230SeedDefaultSounds(t, dataDir)
	for _, cue := range []string{"qcm-indice", "rafale-fin-lot", "depart"} {
		body, ct := multipartImageBody(t, "x.wav", tw230CanonicalWAV(0.7))
		if w := tw230Post(t, s, "/api/sounds/"+cue, body, ct); w.Code != http.StatusOK {
			t.Fatalf("setup invalide (%s) : %d", cue, w.Code)
		}
	}
	w := tw230Post(t, s, "/api/sounds/restore-defaults", nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	cues, _ := tw231hGetSounds(t, s)
	if len(cues) != 15 {
		t.Fatalf("15 cues attendues, got %d", len(cues))
	}
	for _, c := range cues {
		if c["custom"] != false {
			t.Errorf("%v : custom=false attendu après restore-defaults", c["cue"])
		}
	}
	for _, c := range audio.AllCues() {
		got, err := os.ReadFile(filepath.Join(tw230SoundsDir(dataDir), synth.Filename(c)))
		want, _ := synth.Generate(c)
		if err != nil || string(got) != string(want) {
			t.Errorf("%s : fichier non restauré (err=%v)", c, err)
		}
	}
}
