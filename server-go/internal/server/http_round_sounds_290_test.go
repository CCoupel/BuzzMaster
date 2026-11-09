// #290 (v11.6.0, LOT-4 Batch 2, révision 3) — tests UNITAIRES de internal/server/http_round_sounds.go (couverture du
// paquet : le fichier n'était jusqu'ici exercé que depuis cmd/server). Contrat : ambiance-settings.md §9.3,
// http-endpoints.md « Rounds sounds » : liste, envoi validé/refusé, chemin déterministe, DELETE, test, 404/405, erreurs
// d'écriture. Réutilise setupTestHTTPServer, multipartImageBody, tw230Post, tw232uWAV, tw219hCanonicalWAV. Préfixe tw290r.
package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/game"
)

// tw290rAmbiance : AmbianceProvider factice (les méthodes non surchargées paniqueraient : elles ne sont pas appelées ici).
type tw290rAmbiance struct {
	AmbianceProvider
	changed int
	testOK  bool
	tested  []string
}

func (f *tw290rAmbiance) AmbianceRoundsChanged() { f.changed++ }
func (f *tw290rAmbiance) AmbianceTestFile(key, rel string) bool {
	f.tested = append(f.tested, key+"|"+rel)
	return f.testOK
}

func tw290rSetup(t *testing.T) (*HTTPServer, string, *tw290rAmbiance) {
	t.Helper()
	s, dataDir := setupTestHTTPServer(t)
	qdir := filepath.Join(dataDir, "files", "questions")
	if err := os.MkdirAll(qdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := SaveRoundsFile(qdir, RoundsFile{Rounds: []game.Round{{ID: "r1", Name: "Blind-test"}, {ID: "r2", Name: "Finale"}}, NextID: 3}); err != nil {
		t.Fatal(err)
	}
	f := &tw290rAmbiance{testOK: true}
	s.Ambiance = f
	return s, dataDir, f
}

func tw290rReq(t *testing.T, s *HTTPServer, method, path string, body []byte, ct string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	var req *http.Request
	if rdr != nil {
		req = httptest.NewRequest(method, path, rdr)
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	return w
}

func tw290rUpload(t *testing.T, s *HTTPServer, round, key, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	body, ct := multipartImageBody(t, filename, content)
	data := body.Bytes()
	return tw290rReq(t, s, "POST", "/api/rounds/"+round+"/sounds/"+key, data, ct)
}

func tw290rJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("réponse non JSON (%d) : %v — %s", w.Code, err, w.Body.String())
	}
	return m
}

func tw290rRounds(t *testing.T, dataDir string) RoundsFile {
	t.Helper()
	return LoadRoundsFile(filepath.Join(dataDir, "files", "questions"))
}

// ---------------------------------------------------------------------------
// Routage : 404 / 405
// ---------------------------------------------------------------------------

func TestRoundSounds290_Router_NotFoundAndMethodNotAllowed(t *testing.T) {
	s, _, _ := tw290rSetup(t)
	for _, p := range []string{
		"/api/rounds/", "/api/rounds/r1", "/api/rounds/r1/other", "/api/rounds/r1/sounds/depart/extra/more",
		"/api/rounds/x1/sounds", "/api/rounds/R1/sounds", "/api/rounds/r1/sounds/inconnue", "/api/rounds/r1/sounds/inconnue/test",
		"/api/rounds/r1/sounds/depart/notest",
	} {
		if w := tw290rReq(t, s, "GET", p, nil, ""); w.Code != http.StatusNotFound {
			t.Errorf("GET %s : 404 attendu, got %d", p, w.Code)
		}
	}
	for _, c := range []struct{ m, p string }{
		{"POST", "/api/rounds/r1/sounds"}, {"DELETE", "/api/rounds/r1/sounds"},
		{"PUT", "/api/rounds/r1/sounds/depart"}, {"GET", "/api/rounds/r1/sounds/depart/test"}, {"DELETE", "/api/rounds/r1/sounds/depart/test"},
	} {
		if w := tw290rReq(t, s, c.m, c.p, nil, ""); w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s : 405 attendu, got %d", c.m, c.p, w.Code)
		}
	}
}

// ---------------------------------------------------------------------------
// GET /api/rounds/{id}/sounds
// ---------------------------------------------------------------------------

func TestRoundSounds290_List_EmptyUnknownAndOrdered(t *testing.T) {
	s, dataDir, _ := tw290rSetup(t)
	w := tw290rReq(t, s, "GET", "/api/rounds/r1/sounds", nil, "")
	if m := tw290rJSON(t, w); w.Code != 200 || m["round_id"] != "r1" || len(m["files"].([]interface{})) != 0 {
		t.Fatalf("liste vide attendue, got %d %v", w.Code, m)
	}
	if w := tw290rReq(t, s, "GET", "/api/rounds/r9/sounds", nil, ""); w.Code != http.StatusNotFound {
		t.Errorf("manche inconnue : 404, got %d", w.Code)
	}
	// ordre normatif : cues du registre puis decompte, quel que soit l'ordre d'envoi
	for _, k := range []string{"decompte", "rafale-fin-lot", "depart"} {
		content := tw219hCanonicalWAV(1)
		if k != "decompte" {
			content = tw230CanonicalWAV(0.5)
		}
		if w := tw290rUpload(t, s, "r1", k, "x.wav", content); w.Code != 200 {
			t.Fatalf("envoi %s : %d %s", k, w.Code, w.Body.String())
		}
	}
	m := tw290rJSON(t, tw290rReq(t, s, "GET", "/api/rounds/r1/sounds", nil, ""))
	files := m["files"].([]interface{})
	var order []string
	for _, f := range files {
		fm := f.(map[string]interface{})
		order = append(order, fm["cue"].(string))
		if fm["present"] != true || fm["duration_seconds"] == nil {
			t.Errorf("%v : present:true et durée attendus", fm)
		}
	}
	if len(order) != 3 || order[0] != "depart" || order[1] != "rafale-fin-lot" || order[2] != "decompte" {
		t.Errorf("ordre normatif attendu [depart rafale-fin-lot decompte], got %v", order)
	}
	_ = dataDir
}

func TestRoundSounds290_List_OrphanReference_PresentFalse_Untouched(t *testing.T) {
	s, dataDir, _ := tw290rSetup(t)
	tw290rUpload(t, s, "r1", "gagne", "g.wav", tw230CanonicalWAV(0.5))
	before, _ := os.ReadFile(filepath.Join(dataDir, "files", "questions", "rounds.json"))
	_ = os.Remove(filepath.Join(dataDir, "files", "sounds", "rounds", "r1", "gagne.wav"))
	m := tw290rJSON(t, tw290rReq(t, s, "GET", "/api/rounds/r1/sounds", nil, ""))
	f := m["files"].([]interface{})[0].(map[string]interface{})
	if f["present"] != false || f["duration_seconds"] != nil {
		t.Errorf("référence orpheline : present:false / durée null, got %v", f)
	}
	if after, _ := os.ReadFile(filepath.Join(dataDir, "files", "questions", "rounds.json")); !bytes.Equal(before, after) {
		t.Error("D12 : rounds.json n'est jamais corrigé en silence")
	}
}

// Une référence forgée (traversée / absolue) n'est jamais suivie : listée absente, aucun accès hors du dossier des sons.
func TestRoundSounds290_List_ForgedReference_NeverFollowed(t *testing.T) {
	s, dataDir, _ := tw290rSetup(t)
	outside := filepath.Join(dataDir, "outside.wav")
	_ = os.WriteFile(outside, tw230CanonicalWAV(0.5), 0o644)
	qdir := filepath.Join(dataDir, "files", "questions")
	rf := LoadRoundsFile(qdir)
	amb := rf.Rounds[0].Ambiance.Clone()
	amb.SoundFiles = map[string]string{"depart": "../../outside.wav", "gagne": outside, "perdu": ".."}
	rf.Rounds[0].Ambiance = amb
	if err := SaveRoundsFile(qdir, rf); err != nil {
		t.Fatal(err)
	}
	m := tw290rJSON(t, tw290rReq(t, s, "GET", "/api/rounds/r1/sounds", nil, ""))
	for _, f := range m["files"].([]interface{}) {
		if fm := f.(map[string]interface{}); fm["present"] != false {
			t.Errorf("référence forgée %v : jamais présente (jamais lue)", fm)
		}
	}
}

// ---------------------------------------------------------------------------
// POST — envoi validé / refusé
// ---------------------------------------------------------------------------

func TestRoundSounds290_Upload_OK_WritesDeterministicFile_RoundsJson_NotifiesAmbiance(t *testing.T) {
	s, dataDir, f := tw290rSetup(t)
	w := tw290rUpload(t, s, "r1", "depart", "x.wav", tw230CanonicalWAV(0.5))
	m := tw290rJSON(t, w)
	if w.Code != 200 || m["path"] != "rounds/r1/depart.wav" || m["converted"] != false || m["source_format"] != nil || m["warning"] != nil {
		t.Fatalf("200 + path attendus, got %d %v", w.Code, m)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "files", "sounds", "rounds", "r1", "depart.wav")); err != nil {
		t.Errorf("fichier absent : %v", err)
	}
	if got := tw290rRounds(t, dataDir).Rounds[0].Ambiance.SoundFiles["depart"]; got != "rounds/r1/depart.wav" {
		t.Errorf("rounds.json : %q", got)
	}
	if tw290rRounds(t, dataDir).Rounds[1].Ambiance.SoundFiles != nil && len(tw290rRounds(t, dataDir).Rounds[1].Ambiance.SoundFiles) != 0 {
		t.Error("l'autre manche n'est pas touchée")
	}
	if f.changed != 1 {
		t.Errorf("AmbianceRoundsChanged attendu une fois, got %d", f.changed)
	}
}

func TestRoundSounds290_Upload_NoAmbianceProvider_StillWorks(t *testing.T) {
	s, _, _ := tw290rSetup(t)
	s.Ambiance = nil
	if w := tw290rUpload(t, s, "r1", "reveal", "x.wav", tw230CanonicalWAV(0.5)); w.Code != 200 {
		t.Errorf("sans fournisseur d'ambiance : 200 attendu, got %d", w.Code)
	}
	if w := tw290rReq(t, s, "DELETE", "/api/rounds/r1/sounds/reveal", nil, ""); w.Code != 200 {
		t.Errorf("DELETE sans fournisseur : 200 attendu, got %d", w.Code)
	}
}

func TestRoundSounds290_Upload_NonCanonical_Converted_Countdown_And_Warning(t *testing.T) {
	s, dataDir, _ := tw290rSetup(t)
	m := tw290rJSON(t, tw290rUpload(t, s, "r1", "decompte", "d.wav", tw232uWAV(1, 1, 48000, 24, 2)))
	if m["converted"] != true || m["source_format"] != "48000Hz/1ch/24bit" || m["path"] != "rounds/r1/decompte.wav" {
		t.Errorf("décompte converti attendu, got %v", m)
	}
	raw, _ := os.ReadFile(filepath.Join(dataDir, "files", "sounds", "rounds", "r1", "decompte.wav"))
	if _, err := audio.ValidateQuestionSound(raw); err != nil {
		t.Errorf("fichier stocké canonique attendu : %v", err)
	}
	w := tw290rUpload(t, s, "r1", "gagne", "long.wav", tw230CanonicalWAV(3.2)) // > 2 s : accepté avec avertissement
	if m := tw290rJSON(t, w); w.Code != 200 || m["warning"] == nil {
		t.Errorf("bruitage de 3,2 s : accepté avec `warning`, got %d %v", w.Code, m)
	}
}

func TestRoundSounds290_Upload_Refusals(t *testing.T) {
	s, dataDir, f := tw290rSetup(t)
	before, _ := os.ReadFile(filepath.Join(dataDir, "files", "questions", "rounds.json"))
	cases := []struct {
		name, round, key, filename string
		content                    []byte
		want                       int
	}{
		{"manche inconnue", "r9", "depart", "x.wav", tw230CanonicalWAV(0.5), 404},
		{"extension .mp3", "r1", "depart", "x.mp3", []byte("ID3"), 400},
		{"octets quelconques", "r1", "depart", "x.wav", []byte("pas un wav"), 400},
		{"bruitage de 6 s", "r1", "depart", "x.wav", tw230CanonicalWAV(6), 400},
		{"décompte de 31 s", "r1", "decompte", "x.wav", tw219hCanonicalWAV(31), 400},
		{"trop volumineux (bruitage)", "r1", "depart", "x.wav", tw232uAssemble(1, 2, 44100, 16, make([]byte, 9<<20)), 413},
		{"trop volumineux (décompte)", "r1", "decompte", "x.wav", tw232uAssemble(1, 2, 44100, 16, make([]byte, 25<<20)), 413},
	}
	for _, c := range cases {
		if w := tw290rUpload(t, s, c.round, c.key, c.filename, c.content); w.Code != c.want {
			t.Errorf("%s : %d attendu, got %d %s", c.name, c.want, w.Code, w.Body.String())
		}
	}
	if w := tw290rReq(t, s, "POST", "/api/rounds/r1/sounds/depart", []byte("pas du multipart"), "text/plain"); w.Code != 400 {
		t.Errorf("corps non multipart : 400 attendu, got %d", w.Code)
	}
	body, ct := multipartImageBody(t, "", nil) // aucun fichier
	if w := tw290rReq(t, s, "POST", "/api/rounds/r1/sounds/depart", body.Bytes(), ct); w.Code != 400 {
		t.Errorf("sans champ file : 400 attendu, got %d", w.Code)
	}
	if after, _ := os.ReadFile(filepath.Join(dataDir, "files", "questions", "rounds.json")); !bytes.Equal(before, after) {
		t.Error("un refus ne modifie pas rounds.json")
	}
	if entries, _ := os.ReadDir(filepath.Join(dataDir, "files", "sounds", "rounds")); len(entries) != 0 {
		t.Errorf("un refus n'écrit aucun fichier, got %v", entries)
	}
	if f.changed != 0 {
		t.Error("un refus ne notifie pas l'ambiance")
	}
}

// Erreurs d'écriture : dossier des sons impossible à créer ⇒ 500 sans toucher à rounds.json.
func TestRoundSounds290_Upload_WriteErrors_500(t *testing.T) {
	s, dataDir, f := tw290rSetup(t)
	soundsRounds := filepath.Join(dataDir, "files", "sounds")
	if err := os.MkdirAll(soundsRounds, 0o755); err != nil {
		t.Fatal(err)
	}
	// « rounds » est un FICHIER : MkdirAll(sounds/rounds/r1) échoue
	if err := os.WriteFile(filepath.Join(soundsRounds, "rounds"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if w := tw290rUpload(t, s, "r1", "depart", "x.wav", tw230CanonicalWAV(0.5)); w.Code != http.StatusInternalServerError {
		t.Errorf("création du dossier impossible : 500 attendu, got %d", w.Code)
	}
	_ = os.Remove(filepath.Join(soundsRounds, "rounds"))

	// le fichier cible est un DOSSIER : l'écriture atomique (rename) échoue
	target := filepath.Join(soundsRounds, "rounds", "r1", "depart.wav")
	if err := os.MkdirAll(filepath.Join(target, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if w := tw290rUpload(t, s, "r1", "depart", "x.wav", tw230CanonicalWAV(0.5)); w.Code != http.StatusInternalServerError {
		t.Errorf("écriture du fichier impossible : 500 attendu, got %d", w.Code)
	}
	if got := tw290rRounds(t, dataDir).Rounds[0].Ambiance.SoundFiles["depart"]; got != "" {
		t.Errorf("échec d'écriture : rounds.json inchangé, got %q", got)
	}
	if f.changed != 0 {
		t.Error("échec : aucune notification")
	}
}

// rounds.json impossible à réécrire ⇒ 500 et un NOUVEAU fichier n'est pas laissé orphelin.
func TestRoundSounds290_Upload_RoundsJsonWriteFails_500_RemovesNewFile(t *testing.T) {
	s, dataDir, _ := tw290rSetup(t)
	qdir := filepath.Join(dataDir, "files", "questions")
	// un dossier à la place du fichier temporaire/cible rend SaveRoundsFile impossible sans casser la lecture :
	// on remplace rounds.json par un lien vers un dossier APRÈS la lecture n'est pas faisable ; on bloque donc l'écriture
	// en rendant le DOSSIER des questions non inscriptible quand l'OS le permet (sinon le cas est ignoré : root).
	if err := os.Chmod(qdir, 0o555); err != nil {
		t.Skip("chmod indisponible")
	}
	defer os.Chmod(qdir, 0o755)
	if f, err := os.CreateTemp(qdir, "probe"); err == nil {
		f.Close()
		os.Remove(f.Name())
		t.Skip("le dossier reste inscriptible (exécution en root) — cas non simulable")
	}
	w := tw290rUpload(t, s, "r1", "depart", "x.wav", tw230CanonicalWAV(0.5))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("rounds.json non inscriptible : 500 attendu, got %d", w.Code)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "files", "sounds", "rounds", "r1", "depart.wav")); !os.IsNotExist(err) {
		t.Error("le nouveau fichier ne doit pas rester orphelin quand rounds.json n'a pas pu être écrit")
	}
}

func TestRoundSounds290_Upload_ReplaceKeepsSinglePath(t *testing.T) {
	s, dataDir, _ := tw290rSetup(t)
	for i := 0; i < 3; i++ {
		if w := tw290rUpload(t, s, "r1", "perdu", "x.wav", tw230CanonicalWAV(0.4+0.1*float64(i))); w.Code != 200 {
			t.Fatalf("envoi %d : %d", i, w.Code)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(dataDir, "files", "sounds", "rounds", "r1"))
	if len(entries) != 1 || entries[0].Name() != "perdu.wav" {
		t.Errorf("un seul perdu.wav attendu, got %v", entries)
	}
}

// ---------------------------------------------------------------------------
// DELETE
// ---------------------------------------------------------------------------

func TestRoundSounds290_Delete_RemovesFileAndKey_404s(t *testing.T) {
	s, dataDir, f := tw290rSetup(t)
	tw290rUpload(t, s, "r1", "depart", "x.wav", tw230CanonicalWAV(0.5))
	tw290rUpload(t, s, "r1", "gagne", "x.wav", tw230CanonicalWAV(0.5))
	f.changed = 0
	w := tw290rReq(t, s, "DELETE", "/api/rounds/r1/sounds/depart", nil, "")
	if m := tw290rJSON(t, w); w.Code != 200 || m["status"] != "ok" {
		t.Fatalf("200 {status:ok} attendu, got %d %v", w.Code, m)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "files", "sounds", "rounds", "r1", "depart.wav")); !os.IsNotExist(err) {
		t.Error("fichier supprimé du disque")
	}
	files := tw290rRounds(t, dataDir).Rounds[0].Ambiance.SoundFiles
	if _, still := files["depart"]; still || files["gagne"] == "" {
		t.Errorf("clé depart retirée, gagne conservée : %v", files)
	}
	if f.changed != 1 {
		t.Errorf("AmbianceRoundsChanged attendu une fois, got %d", f.changed)
	}
	if w := tw290rReq(t, s, "DELETE", "/api/rounds/r1/sounds/depart", nil, ""); w.Code != http.StatusNotFound {
		t.Errorf("second DELETE : 404, got %d", w.Code)
	}
	if w := tw290rReq(t, s, "DELETE", "/api/rounds/r9/sounds/depart", nil, ""); w.Code != http.StatusNotFound {
		t.Errorf("manche inconnue : 404, got %d", w.Code)
	}
}

func TestRoundSounds290_Delete_OrphanReference_And_ForgedPath_AreSafe(t *testing.T) {
	s, dataDir, _ := tw290rSetup(t)
	outside := filepath.Join(dataDir, "outside.wav")
	_ = os.WriteFile(outside, []byte("secret"), 0o644)
	qdir := filepath.Join(dataDir, "files", "questions")
	rf := LoadRoundsFile(qdir)
	amb := rf.Rounds[0].Ambiance.Clone()
	amb.SoundFiles = map[string]string{"depart": "../../outside.wav", "gagne": "rounds/r1/gagne.wav"}
	rf.Rounds[0].Ambiance = amb
	_ = SaveRoundsFile(qdir, rf)
	for _, k := range []string{"depart", "gagne"} { // orpheline et forgée
		if w := tw290rReq(t, s, "DELETE", "/api/rounds/r1/sounds/"+k, nil, ""); w.Code != 200 {
			t.Errorf("DELETE %s : 200 attendu, got %d", k, w.Code)
		}
	}
	if b, err := os.ReadFile(outside); err != nil || string(b) != "secret" {
		t.Error("un chemin forgé ne doit JAMAIS faire supprimer un fichier hors du dossier des sons")
	}
}

// ---------------------------------------------------------------------------
// POST …/test
// ---------------------------------------------------------------------------

func TestRoundSounds290_Test_PlayedRoundThenGlobal_AndUnavailable(t *testing.T) {
	s, _, f := tw290rSetup(t)
	tw290rUpload(t, s, "r1", "depart", "x.wav", tw230CanonicalWAV(0.5))

	m := tw290rJSON(t, tw290rReq(t, s, "POST", "/api/rounds/r1/sounds/depart/test", nil, ""))
	if m["status"] != "ok" || m["played"] != "round" {
		t.Errorf("fichier de manche : {ok, round} attendu, got %v", m)
	}
	m = tw290rJSON(t, tw290rReq(t, s, "POST", "/api/rounds/r1/sounds/gagne/test", nil, ""))
	if m["status"] != "ok" || m["played"] != "global" {
		t.Errorf("sans fichier de manche : {ok, global} attendu, got %v", m)
	}
	m = tw290rJSON(t, tw290rReq(t, s, "POST", "/api/rounds/r1/sounds/decompte/test", nil, ""))
	if m["played"] != "global" {
		t.Errorf("décompte sans fichier : global, got %v", m)
	}
	if len(f.tested) != 3 || f.tested[0] != "depart|rounds/r1/depart.wav" || f.tested[1] != "gagne|gagne.wav" || f.tested[2] != "decompte|decompte.wav" {
		t.Errorf("fichiers joués attendus [rounds/r1/depart.wav gagne.wav decompte.wav], got %v", f.tested)
	}
	f.testOK = false
	if m := tw290rJSON(t, tw290rReq(t, s, "POST", "/api/rounds/r1/sounds/depart/test", nil, "")); m["status"] != "unavailable" {
		t.Errorf("sortie indisponible : status=unavailable, got %v", m)
	}
	s.Ambiance = nil
	if m := tw290rJSON(t, tw290rReq(t, s, "POST", "/api/rounds/r1/sounds/depart/test", nil, "")); m["status"] != "unavailable" {
		t.Errorf("sans fournisseur : status=unavailable, got %v", m)
	}
	if w := tw290rReq(t, s, "POST", "/api/rounds/r9/sounds/depart/test", nil, ""); w.Code != http.StatusNotFound {
		t.Errorf("manche inconnue : 404, got %d", w.Code)
	}
}

func TestRoundSounds290_Test_MissingRoundFile_FallsBackToGlobalLabel(t *testing.T) {
	s, dataDir, f := tw290rSetup(t)
	tw290rUpload(t, s, "r1", "reveal", "x.wav", tw230CanonicalWAV(0.5))
	_ = os.Remove(filepath.Join(dataDir, "files", "sounds", "rounds", "r1", "reveal.wav"))
	m := tw290rJSON(t, tw290rReq(t, s, "POST", "/api/rounds/r1/sounds/reveal/test", nil, ""))
	if m["played"] != "global" || f.tested[len(f.tested)-1] != "reveal|reveal.wav" {
		t.Errorf("fichier de manche manquant ⇒ le global est joué, got %v / %v", m, f.tested)
	}
}

// ---------------------------------------------------------------------------
// Fonctions pures
// ---------------------------------------------------------------------------

func TestRoundSounds290_RoundSoundKey_ClosedCatalogueUnionCountdown(t *testing.T) {
	for _, k := range []string{"depart", "rafale-fin-lot", "decompte", "qcm-indice"} {
		if got, ok := roundSoundKey(k); !ok || got != k {
			t.Errorf("%s doit être accepté", k)
		}
	}
	for _, k := range []string{"", "Depart", "depart.wav", "../depart", "decompte2", "inconnue"} {
		if _, ok := roundSoundKey(k); ok {
			t.Errorf("%q doit être refusé", k)
		}
	}
}

func TestRoundSounds290_RoundFilePath_StaysInsideSoundsDir(t *testing.T) {
	s, dataDir, _ := tw290rSetup(t)
	root := filepath.Join(dataDir, "files", "sounds")
	if got := s.roundFilePath("rounds/r1/depart.wav"); got != filepath.Join(root, "rounds", "r1", "depart.wav") {
		t.Errorf("chemin valide : %q", got)
	}
	for _, rel := range []string{"", "..", "../x.wav", "rounds/../../x.wav", filepath.Join(dataDir, "x.wav")} {
		if got := s.roundFilePath(rel); got != "" {
			t.Errorf("rel %q : chemin refusé attendu, got %q", rel, got)
		}
	}
}

func TestRoundSounds290_DurationOf_InvalidOrMissing_Nil(t *testing.T) {
	s, dataDir, _ := tw290rSetup(t)
	bad := filepath.Join(dataDir, "bad.wav")
	_ = os.WriteFile(bad, []byte("nope"), 0o644)
	if s.durationOf("depart", bad) != nil || s.durationOf("decompte", bad) != nil || s.durationOf("depart", filepath.Join(dataDir, "absent.wav")) != nil {
		t.Error("fichier absent ou invalide ⇒ durée nil")
	}
	good := filepath.Join(dataDir, "good.wav")
	_ = os.WriteFile(good, tw230CanonicalWAV(1), 0o644)
	if d, _ := s.durationOf("depart", good).(float64); d < 0.9 || d > 1.1 {
		t.Errorf("durée ≈ 1 s attendue, got %v", d)
	}
	if d, _ := s.durationOf("decompte", good).(float64); d < 0.9 || d > 1.1 {
		t.Errorf("durée du décompte ≈ 1 s attendue, got %v", d)
	}
}
