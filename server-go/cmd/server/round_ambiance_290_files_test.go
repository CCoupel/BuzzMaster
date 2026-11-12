// #290 (v11.6.0, LOT-4 Batch 2, révision 3) — fichiers de son PAR MANCHE : API REST `/api/rounds/{id}/sounds[/{cue}[/test]]`,
// stockage `sounds/rounds/<id>/<cue>.wav`, `rounds.json`, ROUNDS_UPDATE qui ignore `sound.files`, nettoyage à la suppression de la
// manche, lecture (bruitages + décompte) à l'entrée de la manche, repli et signalement d'un fichier manquant, indépendance du
// catalogue global. Source normative : contracts/ambiance-settings.md §9 (§9.1–§9.6), http-endpoints.md « Rounds sounds »,
// models.md §Manches, websocket-actions.md (ROUNDS_UPDATE), sound.md (PlayCueAt). Écrit depuis le contrat avant le code
// (rouge attendu pour l'API). Réutilise tw290New (serveur HTTP réel) et ses helpers. Préfixe tw290f.
package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"buzzcontrol/internal/rounds"
	"buzzcontrol/internal/server"
)

// ---------------------------------------------------------------------------
// Outils
// ---------------------------------------------------------------------------

func (e *tw290Env) soundsDir() string { return filepath.Join(e.dir, "..", "sounds") }

func tw290fCanonical(frames int) []byte {
	pcm := make([]byte, frames*audio.FrameSize)
	for i := range pcm {
		pcm[i] = byte(i%251 + 1)
	}
	return audio.BuildCanonicalWAV(pcm)
}

// tw290fPCM : WAV PCM entier quelconque (rate/canaux/bits), silence.
func tw290fPCM(rate, ch, bits, frames int) []byte {
	data := make([]byte, frames*ch*bits/8)
	var b bytes.Buffer
	le := binary.LittleEndian
	b.WriteString("RIFF")
	_ = binary.Write(&b, le, uint32(36+len(data)))
	b.WriteString("WAVEfmt ")
	_ = binary.Write(&b, le, uint32(16))
	_ = binary.Write(&b, le, uint16(1))
	_ = binary.Write(&b, le, uint16(ch))
	_ = binary.Write(&b, le, uint32(rate))
	_ = binary.Write(&b, le, uint32(rate*ch*bits/8))
	_ = binary.Write(&b, le, uint16(ch*bits/8))
	_ = binary.Write(&b, le, uint16(bits))
	b.WriteString("data")
	_ = binary.Write(&b, le, uint32(len(data)))
	b.Write(data)
	return b.Bytes()
}

func tw290fSeconds(s float64) int { return int(s * audio.SampleRate) }

func (e *tw290Env) do(t *testing.T, method, path string, body io.Reader, contentType string) (int, map[string]interface{}) {
	t.Helper()
	req, _ := http.NewRequest(method, e.base+path, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s : %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	return resp.StatusCode, m
}

func (e *tw290Env) upload(t *testing.T, roundID, cue, filename string, content []byte) (int, map[string]interface{}) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if filename != "" {
		part, _ := mw.CreateFormFile("file", filename)
		_, _ = part.Write(content)
	}
	_ = mw.Close()
	return e.do(t, "POST", "/api/rounds/"+roundID+"/sounds/"+cue, &buf, mw.FormDataContentType())
}

func (e *tw290Env) list(t *testing.T, roundID string) (int, map[string]interface{}) {
	t.Helper()
	return e.do(t, "GET", "/api/rounds/"+roundID+"/sounds", nil, "")
}

func (e *tw290Env) roundFile(roundID, cue string) string {
	return filepath.Join(e.soundsDir(), "rounds", roundID, cue+".wav")
}

func (e *tw290Env) roundAmbiance(t *testing.T, roundID string) map[string]interface{} {
	t.Helper()
	for _, r := range e.roundsFile(t) {
		if r["ID"] == roundID {
			a, _ := r["AMBIANCE"].(map[string]interface{})
			return a
		}
	}
	t.Fatalf("manche %s absente de rounds.json", roundID)
	return nil
}

func (e *tw290Env) roundsJSONBytes(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(e.dir, "rounds.json"))
	if err != nil {
		t.Fatalf("rounds.json : %v", err)
	}
	return b
}

func tw290fFiles(m map[string]interface{}) map[string]interface{} {
	f, _ := tw290Path(m, "sound", "files").(map[string]interface{})
	return f
}

func tw290fWriteGlobal(t *testing.T, e *tw290Env, cue string, frames int) {
	t.Helper()
	if err := os.MkdirAll(e.soundsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.soundsDir(), cue+".wav"), tw290fCanonical(frames), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// GET /api/rounds/{id}/sounds
// ---------------------------------------------------------------------------

func TestRoundFiles290_List_EmptyWhenNoFile_FilesIsArrayNotNull(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	code, m := e.list(t, "r1")
	if code != http.StatusOK {
		t.Fatalf("200 attendu, got %d %v", code, m)
	}
	if m["round_id"] != "r1" {
		t.Errorf("round_id=\"r1\" attendu, got %v", m["round_id"])
	}
	files, ok := m["files"].([]interface{})
	if !ok || len(files) != 0 {
		t.Errorf("`files` doit être un tableau vide [] (jamais null/absent), got %v", m["files"])
	}
}

func TestRoundFiles290_List_UnknownOrMalformedRound_404(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	for _, id := range []string{"r99", "x", "r1%2F..%2Fr2", "R1", "r1;rm", "%2E%2E"} { // « .. » nu est normalisé par le client HTTP (chemin /api/sounds)
		if code, _ := e.do(t, "GET", "/api/rounds/"+id+"/sounds", nil, ""); code != http.StatusNotFound {
			t.Errorf("manche %q : 404 attendu, got %d", id, code)
		}
	}
}

// ---------------------------------------------------------------------------
// POST /api/rounds/{id}/sounds/{cue} — Remplacer…
// ---------------------------------------------------------------------------

func TestRoundFiles290_Upload_Cue_WritesFile_RoundsJson_AndAnswersContractFields(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	code, m := e.upload(t, "r1", "depart", "mon-depart.wav", tw290fCanonical(tw290fSeconds(0.8)))
	if code != http.StatusOK {
		t.Fatalf("200 attendu, got %d %v", code, m)
	}
	for _, k := range []string{"status", "round_id", "cue", "path", "duration_seconds", "converted", "source_format", "warning"} {
		if _, ok := m[k]; !ok {
			t.Errorf("champ %q absent de la réponse (contrat §9.3) : %v", k, m)
		}
	}
	if m["path"] != "rounds/r1/depart.wav" || m["cue"] != "depart" || m["round_id"] != "r1" {
		t.Errorf("path/cue/round_id inattendus : %v", m)
	}
	if m["converted"] != false || m["source_format"] != nil {
		t.Errorf("fichier déjà canonique ⇒ converted=false / source_format=null, got %v / %v", m["converted"], m["source_format"])
	}
	if _, err := os.Stat(e.roundFile("r1", "depart")); err != nil {
		t.Fatalf("fichier attendu sous sounds/rounds/r1/depart.wav : %v", err)
	}
	if got := tw290fFiles(e.roundAmbiance(t, "r1"))["depart"]; got != "rounds/r1/depart.wav" {
		t.Errorf("rounds.json : AMBIANCE.sound.files.depart = %v, attendu \"rounds/r1/depart.wav\"", got)
	}
}

func TestRoundFiles290_Upload_ListsPresentWithDuration_AndAppearsInSettings(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	e.upload(t, "r1", "gagne", "g.wav", tw290fCanonical(tw290fSeconds(1.5)))
	_, m := e.list(t, "r1")
	files, _ := m["files"].([]interface{})
	if len(files) != 1 {
		t.Fatalf("1 fichier attendu, got %v", m["files"])
	}
	f, _ := files[0].(map[string]interface{})
	if f["cue"] != "gagne" || f["path"] != "rounds/r1/gagne.wav" || f["present"] != true {
		t.Errorf("entrée attendue {gagne, rounds/r1/gagne.wav, present:true}, got %v", f)
	}
	if d, _ := f["duration_seconds"].(float64); d < 1.4 || d > 1.6 {
		t.Errorf("duration_seconds ≈ 1,5 attendu, got %v", f["duration_seconds"])
	}
	s := e.settings(t)
	if tw290Path(s, "round_override", "sound", "files") != nil {
		// hors manche effective : round_override = {} ; le fichier n'apparaît qu'en manche
		t.Errorf("hors manche, round_override reste {}, got %v", s["round_override"])
	}
	e.ready(t, "1")
	s = e.settings(t)
	if tw290fFiles(map[string]interface{}{"sound": tw290Path(s, "round_override", "sound")})["gagne"] != "rounds/r1/gagne.wav" {
		t.Errorf("round_override.sound.files.gagne attendu en manche, got %v", s["round_override"])
	}
}

func TestRoundFiles290_Upload_NonCanonicalWAV_ConvertedAndStoredCanonical(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	code, m := e.upload(t, "r1", "reveal", "r.wav", tw290fPCM(48000, 1, 16, 48000))
	if code != http.StatusOK {
		t.Fatalf("200 attendu (conversion #232 identique au catalogue), got %d %v", code, m)
	}
	if m["converted"] != true || m["source_format"] != "48000Hz/1ch/16bit" {
		t.Errorf("converted=true / source_format=\"48000Hz/1ch/16bit\" attendus, got %v / %v", m["converted"], m["source_format"])
	}
	raw, err := os.ReadFile(e.roundFile("r1", "reveal"))
	if err != nil {
		t.Fatal(err)
	}
	le := binary.LittleEndian
	if le.Uint16(raw[22:]) != 2 || le.Uint32(raw[24:]) != 44100 || le.Uint16(raw[34:]) != 16 {
		t.Errorf("seul le résultat canonique est stocké (44100 Hz, 2 canaux, 16 bits), got ch=%d rate=%d bits=%d", le.Uint16(raw[22:]), le.Uint32(raw[24:]), le.Uint16(raw[34:]))
	}
}

func TestRoundFiles290_Upload_ValidationIdenticalToCatalogue(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	before := e.roundsJSONBytes(t)
	cases := []struct {
		name, cue, filename string
		content             []byte
		want                int
	}{
		{"bruitage de 6 s (> 5 s)", "depart", "long.wav", tw290fCanonical(tw290fSeconds(6)), http.StatusBadRequest},
		{"décompte de 31 s (> 30 s)", "decompte", "long.wav", tw290fCanonical(tw290fSeconds(31)), http.StatusBadRequest},
		{"MP3 renommé .wav (tag 0x55)", "depart", "x.wav", func() []byte {
			b := tw290fPCM(44100, 2, 16, 2000)
			binary.LittleEndian.PutUint16(b[20:], 0x55)
			return b
		}(), http.StatusBadRequest},
		{"extension .mp3", "depart", "x.mp3", []byte("ID3fake"), http.StatusBadRequest},
		{"octets quelconques", "depart", "x.wav", []byte("pas un wav"), http.StatusBadRequest},
		{"fichier trop volumineux (> 4 × plafond)", "depart", "enorme.wav", tw290fPCM(44100, 2, 16, 9<<20/4), http.StatusRequestEntityTooLarge},
	}
	for _, c := range cases {
		code, m := e.upload(t, "r1", c.cue, c.filename, c.content)
		if code != c.want {
			t.Errorf("%s : %d attendu, got %d %v", c.name, c.want, code, m)
		}
	}
	if code, _ := e.upload(t, "r1", "depart", "", nil); code != http.StatusBadRequest {
		t.Errorf("sans champ `file` : 400 attendu, got %d", code)
	}
	if !bytes.Equal(before, e.roundsJSONBytes(t)) {
		t.Error("un refus ne modifie pas rounds.json")
	}
	if entries, _ := os.ReadDir(filepath.Join(e.soundsDir(), "rounds", "r1")); len(entries) != 0 {
		t.Errorf("un refus n'écrit aucun fichier, got %v", entries)
	}
}

func TestRoundFiles290_Upload_Countdown_Accepts30s_AndStoresDecompteWav(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	code, m := e.upload(t, "r2", "decompte", "tic.wav", tw290fCanonical(tw290fSeconds(29)))
	if code != http.StatusOK {
		t.Fatalf("décompte de 29 s : 200 attendu (validateur du décompte, ≤ 30 s), got %d %v", code, m)
	}
	if m["path"] != "rounds/r2/decompte.wav" {
		t.Errorf("path attendu rounds/r2/decompte.wav, got %v", m["path"])
	}
	if _, err := os.Stat(e.roundFile("r2", "decompte")); err != nil {
		t.Errorf("fichier du décompte absent : %v", err)
	}
}

func TestRoundFiles290_Upload_UnknownRoundOrCue_404_NothingWritten(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	good := tw290fCanonical(2000)
	for _, tc := range []struct{ round, cue string }{
		{"r99", "depart"}, {"r1", "inconnue"}, {"r1", "DEPART"}, {"r1", "depart.wav"}, {"r1", "..%2F..%2Fconfig"},
		{"%2E%2E", "depart"}, {"r1", "depart%2F..%2F..%2Fx"}, {"r1", "decompte2"},
	} {
		if code, _ := e.upload(t, tc.round, tc.cue, "x.wav", good); code != http.StatusNotFound {
			t.Errorf("manche %q cue %q : 404 attendu, got %d", tc.round, tc.cue, code)
		}
	}
	if entries, _ := os.ReadDir(filepath.Join(e.soundsDir(), "rounds")); len(entries) != 0 {
		t.Errorf("aucune écriture pour une requête refusée, got %v", entries)
	}
	if _, err := os.Stat(filepath.Join(e.soundsDir(), "..", "config.json")); err == nil {
		t.Error("aucun fichier ne doit être créé hors du dossier des sons (traversée)")
	}
}

func TestRoundFiles290_Upload_MethodNotAllowed(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	for _, m := range []string{"PUT", "PATCH"} {
		if code, _ := e.do(t, m, "/api/rounds/r1/sounds/depart", bytes.NewBufferString("x"), ""); code != http.StatusMethodNotAllowed {
			t.Errorf("%s : 405 attendu, got %d", m, code)
		}
	}
}

// Nom déterministe : remplacer réécrit le MÊME fichier (temp + rename), aucun orphelin ni fichier temporaire.
func TestRoundFiles290_Replace_SameDeterministicPath_NoOrphanNoTemp(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	e.upload(t, "r1", "depart", "a.wav", tw290fCanonical(tw290fSeconds(0.5)))
	first, _ := os.ReadFile(e.roundFile("r1", "depart"))
	for i := 0; i < 3; i++ {
		if code, _ := e.upload(t, "r1", "depart", "b.wav", tw290fCanonical(tw290fSeconds(1.0+float64(i)*0.1))); code != http.StatusOK {
			t.Fatalf("remplacement %d : 200 attendu, got %d", i, code)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(e.soundsDir(), "rounds", "r1"))
	if len(entries) != 1 || entries[0].Name() != "depart.wav" {
		var names []string
		for _, en := range entries {
			names = append(names, en.Name())
		}
		t.Errorf("un seul fichier depart.wav attendu (pas de suffixe aléatoire, pas de .tmp), got %v", names)
	}
	last, _ := os.ReadFile(e.roundFile("r1", "depart"))
	if bytes.Equal(first, last) || len(last) <= len(first) {
		t.Error("le contenu doit avoir été remplacé par le dernier envoi")
	}
}

// ---------------------------------------------------------------------------
// DELETE /api/rounds/{id}/sounds/{cue} — Revenir au fichier global
// ---------------------------------------------------------------------------

func TestRoundFiles290_Delete_RemovesFileAndKey_KeepsOtherOverrides(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, map[string]interface{}{"sound": map[string]interface{}{"muted": true}, "light": map[string]interface{}{"mode": "ON"}})
	e.upload(t, "r1", "depart", "a.wav", tw290fCanonical(2000))
	e.upload(t, "r1", "gagne", "b.wav", tw290fCanonical(2000))

	code, m := e.do(t, "DELETE", "/api/rounds/r1/sounds/depart", nil, "")
	if code != http.StatusOK || m["status"] != "ok" {
		t.Fatalf("200 {status:ok} attendu, got %d %v", code, m)
	}
	if _, err := os.Stat(e.roundFile("r1", "depart")); !os.IsNotExist(err) {
		t.Error("le fichier doit être supprimé du disque")
	}
	amb := e.roundAmbiance(t, "r1")
	files := tw290fFiles(amb)
	if _, still := files["depart"]; still {
		t.Error("la clé depart doit être retirée de AMBIANCE.sound.files")
	}
	if files["gagne"] != "rounds/r1/gagne.wav" {
		t.Errorf("les autres fichiers sont conservés, got %v", files)
	}
	if tw290Path(amb, "sound", "muted") != true || tw290Path(amb, "light", "mode") != "ON" {
		t.Errorf("les autres surcharges de la manche sont conservées, got %v", amb)
	}
	if code, _ := e.do(t, "DELETE", "/api/rounds/r1/sounds/depart", nil, ""); code != http.StatusNotFound {
		t.Errorf("DELETE d'un fichier que la manche n'a pas : 404 attendu, got %d", code)
	}
	if code, _ := e.do(t, "DELETE", "/api/rounds/r99/sounds/depart", nil, ""); code != http.StatusNotFound {
		t.Errorf("manche inconnue : 404 attendu, got %d", code)
	}
}

func TestRoundFiles290_Delete_LastFile_LeavesEmptyFilesCleanly(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	e.upload(t, "r1", "reveal", "a.wav", tw290fCanonical(2000))
	e.do(t, "DELETE", "/api/rounds/r1/sounds/reveal", nil, "")
	if files := tw290fFiles(e.roundAmbiance(t, "r1")); len(files) != 0 {
		t.Errorf("plus aucun fichier de manche, got %v", files)
	}
	if _, m := e.list(t, "r1"); len(m["files"].([]interface{})) != 0 {
		t.Errorf("la liste est vide, got %v", m["files"])
	}
}

// ---------------------------------------------------------------------------
// ROUNDS_UPDATE ignore `sound.files` (propriété exclusive de l'API §9.3).
// ---------------------------------------------------------------------------

func TestRoundFiles290_RoundsUpdate_IgnoresAndKeepsSoundFiles(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	e.upload(t, "r1", "depart", "a.wav", tw290fCanonical(2000))

	// un client tente d'écrire des fichiers (dont un chemin d'une AUTRE manche et une traversée) via ROUNDS_UPDATE
	e.roundsUpdate(t, []map[string]interface{}{
		{"ID": "r1", "NAME": "Blind-test", "QUESTIONS": []string{"1", "2"}, "AMBIANCE": map[string]interface{}{"sound": map[string]interface{}{
			"muted": true,
			"files": map[string]interface{}{"gagne": "rounds/r2/gagne.wav", "depart": "../../config.json", "reveal": "rounds/r1/reveal.wav"},
		}}},
		{"ID": "r2", "NAME": "Finale", "QUESTIONS": []string{"3"}},
	})
	amb := e.roundAmbiance(t, "r1")
	if tw290Path(amb, "sound", "muted") != true {
		t.Errorf("les autres champs de l'AMBIANCE sont appliqués, got %v", amb)
	}
	files := tw290fFiles(amb)
	if len(files) != 1 || files["depart"] != "rounds/r1/depart.wav" {
		t.Errorf("`sound.files` ignoré et recopié depuis l'existant : {depart: rounds/r1/depart.wav} attendu, got %v", files)
	}
}

func TestRoundFiles290_RoundsUpdate_ExplicitEmptyAmbiance_ClearsSettingsButKeepsFiles(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, map[string]interface{}{"light": map[string]interface{}{"mode": "ON"}})
	e.upload(t, "r1", "gagne", "a.wav", tw290fCanonical(2000))
	e.roundsUpdate(t, []map[string]interface{}{
		{"ID": "r1", "NAME": "Blind-test", "QUESTIONS": []string{"1", "2"}, "AMBIANCE": map[string]interface{}{}},
		{"ID": "r2", "NAME": "Finale", "QUESTIONS": []string{"3"}},
	})
	amb := e.roundAmbiance(t, "r1")
	if tw290Path(amb, "light", "mode") != nil {
		t.Errorf("`{}` efface les réglages, got %v", amb)
	}
	if tw290fFiles(amb)["gagne"] != "rounds/r1/gagne.wav" {
		t.Errorf("`{}` n'efface JAMAIS les fichiers (propriété de l'API), got %v", amb)
	}
	if _, err := os.Stat(e.roundFile("r1", "gagne")); err != nil {
		t.Errorf("le fichier reste sur le disque : %v", err)
	}
}

func TestRoundFiles290_RoundsUpdate_RenameAndReorder_KeepFiles(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	e.upload(t, "r1", "depart", "a.wav", tw290fCanonical(2000))
	e.roundsUpdate(t, []map[string]interface{}{
		{"ID": "r2", "NAME": "Finale", "QUESTIONS": []string{"3"}},
		{"ID": "r1", "NAME": "Renommée", "QUESTIONS": []string{"1", "2"}},
	})
	if tw290fFiles(e.roundAmbiance(t, "r1"))["depart"] != "rounds/r1/depart.wav" {
		t.Error("renommer / réordonner une manche conserve ses fichiers")
	}
}

// ---------------------------------------------------------------------------
// Nettoyage : suppression de manche.
// ---------------------------------------------------------------------------

func TestRoundFiles290_DeleteRound_RemovesItsSoundFolder_Only(t *testing.T) {
	for _, choice := range []string{"KEEP", "DELETE"} {
		t.Run(choice, func(t *testing.T) {
			e := tw290New(t)
			e.twoRounds(t, nil)
			e.upload(t, "r1", "depart", "a.wav", tw290fCanonical(2000))
			e.upload(t, "r2", "depart", "b.wav", tw290fCanonical(2000))
			p := map[string]interface{}{
				"ROUNDS":         []map[string]interface{}{{"ID": "r2", "NAME": "Finale", "QUESTIONS": []string{"3"}}},
				"UNASSIGNED":     []string{"4"},
				"DELETED_ROUNDS": []map[string]interface{}{{"ID": "r1", "QUESTIONS": choice}},
			}
			if choice == "KEEP" {
				p["UNASSIGNED"] = []string{"4", "1", "2"}
			}
			dispatchAs(t, e.app, server.ClientTypeAdmin, "ROUNDS_UPDATE", p)
			if _, err := os.Stat(filepath.Join(e.soundsDir(), "rounds", "r1")); !os.IsNotExist(err) {
				t.Errorf("%s : le dossier sounds/rounds/r1 doit être supprimé avec la manche", choice)
			}
			if _, err := os.Stat(e.roundFile("r2", "depart")); err != nil {
				t.Errorf("%s : les fichiers des autres manches ne sont pas touchés : %v", choice, err)
			}
		})
	}
}

// Le nettoyage n'a lieu QU'APRÈS une écriture réussie : un ROUNDS_UPDATE refusé ne supprime rien.
func TestRoundFiles290_RefusedRoundsUpdate_DeletesNothing(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	e.upload(t, "r1", "depart", "a.wav", tw290fCanonical(2000))
	// r1 n'est ni conservée ni supprimée : MISSING_ROUND_DECISION (refus atomique)
	dispatchAs(t, e.app, server.ClientTypeAdmin, "ROUNDS_UPDATE", map[string]interface{}{
		"ROUNDS":     []map[string]interface{}{{"ID": "r2", "NAME": "Finale", "QUESTIONS": []string{"3"}}},
		"UNASSIGNED": []string{"1", "2", "4"},
	})
	if _, err := os.Stat(e.roundFile("r1", "depart")); err != nil {
		t.Errorf("refus ⇒ aucun fichier supprimé : %v", err)
	}
	if tw290fFiles(e.roundAmbiance(t, "r1"))["depart"] == nil {
		t.Error("refus ⇒ rounds.json inchangé")
	}
}

// ---------------------------------------------------------------------------
// Résolution : AMBIANCE_EFFECTIVE / GET /api/ambiance/settings exposent `sound.files` et ses sources.
// ---------------------------------------------------------------------------

func TestRoundFiles290_Settings_GlobalExposesAllSixteenFiles(t *testing.T) {
	e := tw290New(t)
	files := tw290fFiles(map[string]interface{}{"sound": tw290Path(e.settings(t), "global", "sound")})
	if len(files) != 16 {
		t.Fatalf("global.sound.files : 15 cues + decompte attendus, got %d", len(files))
	}
	if files["depart"] != "depart.wav" || files["decompte"] != "decompte.wav" || files["rafale-fin-lot"] != "rafale-fin-lot.wav" {
		t.Errorf("fichiers du catalogue attendus, got %v", files)
	}
}

func TestRoundFiles290_Effective_FileOfRound_InRound_GlobalOutside(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	e.upload(t, "r1", "depart", "a.wav", tw290fCanonical(2000))
	e.upload(t, "r1", "decompte", "c.wav", tw290fCanonical(2000))
	e.ready(t, "1")
	s := e.settings(t)
	if tw290Path(s, "effective", "sound", "files", "depart") != "rounds/r1/depart.wav" || tw290Path(s, "sources", "sound", "files", "depart") != "round" {
		t.Errorf("en manche : fichier et source \"round\" attendus, got %v / %v", tw290Path(s, "effective", "sound", "files", "depart"), tw290Path(s, "sources", "sound", "files", "depart"))
	}
	if tw290Path(s, "effective", "sound", "files", "decompte") != "rounds/r1/decompte.wav" {
		t.Error("le décompte résout aussi le fichier de la manche")
	}
	if tw290Path(s, "effective", "sound", "files", "gagne") != "gagne.wav" || tw290Path(s, "sources", "sound", "files", "gagne") != "global" {
		t.Error("une cue sans fichier de manche reste globale")
	}
	game := e.gameJSON(t)
	if tw290Path(game, "AMBIANCE_EFFECTIVE", "SETTINGS", "sound", "files", "depart") != "rounds/r1/depart.wav" ||
		tw290Path(game, "AMBIANCE_EFFECTIVE", "SOURCES", "sound", "files", "depart") != "round" {
		t.Errorf("GameState.AMBIANCE_EFFECTIVE reflète le fichier de manche, got %v", game["AMBIANCE_EFFECTIVE"])
	}
	e.ready(t, "4") // hors manche
	s = e.settings(t)
	if tw290Path(s, "effective", "sound", "files", "depart") != "depart.wav" || tw290Path(s, "sources", "sound", "files", "depart") != "global" {
		t.Error("hors manche : fichier global")
	}
}

// L'upload pendant la manche effective s'applique IMMÉDIATEMENT (refreshAmbiance + UPDATE), sans changer de manche.
func TestRoundFiles290_Upload_InEffectiveRound_AppliesImmediately(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	e.ready(t, "1")
	if tw290Path(e.settings(t), "effective", "sound", "files", "reveal") != "reveal.wav" {
		t.Fatal("setup invalide")
	}
	e.upload(t, "r1", "reveal", "a.wav", tw290fCanonical(2000))
	eventually264(t, "fichier de manche appliqué sans attendre", func() bool {
		return tw290Path(e.settings(t), "effective", "sound", "files", "reveal") == "rounds/r1/reveal.wav"
	})
	e.do(t, "DELETE", "/api/rounds/r1/sounds/reveal", nil, "")
	eventually264(t, "retour immédiat au fichier global", func() bool {
		return tw290Path(e.settings(t), "effective", "sound", "files", "reveal") == "reveal.wav"
	})
}

// D1′ point 4 : un geste direct ne change aucun fichier.
func TestRoundFiles290_DirectGesture_NeverChangesFiles(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	e.upload(t, "r1", "depart", "a.wav", tw290fCanonical(2000))
	e.ready(t, "1")
	e.post(t, "/config.json", `{"sound":{"muted":true,"cues_disabled":{"depart":true},"countdown_disabled":true}}`)
	s := e.settings(t)
	if tw290Path(s, "effective", "sound", "files", "depart") != "rounds/r1/depart.wav" || tw290Path(s, "sources", "sound", "files", "depart") != "round" {
		t.Errorf("un geste direct ne touche pas aux fichiers (jamais source \"direct\"), got %v", tw290Path(s, "sources", "sound", "files", "depart"))
	}
}

// ---------------------------------------------------------------------------
// Catalogue global indépendant (C3, C9).
// ---------------------------------------------------------------------------

func TestRoundFiles290_Catalogue_ReplaceRestoreRestoreDefaults_NeverTouchRoundFiles(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	tw290fWriteGlobal(t, e, "depart", 100)
	e.upload(t, "r1", "depart", "a.wav", tw290fCanonical(tw290fSeconds(0.5)))
	round, _ := os.ReadFile(e.roundFile("r1", "depart"))
	global, _ := os.ReadFile(filepath.Join(e.soundsDir(), "depart.wav"))

	// remplacer le catalogue ne change pas le fichier de manche
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("file", "g.wav")
	_, _ = part.Write(tw290fCanonical(tw290fSeconds(0.3)))
	_ = mw.Close()
	if code, _ := e.do(t, "POST", "/api/sounds/depart", &buf, mw.FormDataContentType()); code != http.StatusOK {
		t.Fatalf("remplacement du catalogue : 200 attendu, got %d", code)
	}
	if after, _ := os.ReadFile(e.roundFile("r1", "depart")); !bytes.Equal(after, round) {
		t.Error("remplacer un son du catalogue ne change aucun fichier de manche")
	}
	if again, _ := os.ReadFile(filepath.Join(e.soundsDir(), "depart.wav")); bytes.Equal(again, global) {
		t.Fatal("sanity : le catalogue devait changer")
	}
	// restore / restore-defaults ne touchent pas aux fichiers de manche
	e.do(t, "POST", "/api/sounds/depart/restore", nil, "")
	e.do(t, "POST", "/api/sounds/restore-defaults", nil, "")
	if after, err := os.ReadFile(e.roundFile("r1", "depart")); err != nil || !bytes.Equal(after, round) {
		t.Errorf("restore / restore-defaults ne touchent JAMAIS aux fichiers de manche (err=%v)", err)
	}
	// et inversement : le fichier de manche n'est pas dans le manifeste du catalogue
	manifest, _ := os.ReadFile(filepath.Join(e.soundsDir(), "sounds.json"))
	if bytes.Contains(manifest, []byte("rounds/")) {
		t.Errorf("les fichiers de manche ne figurent pas dans sounds.json (D13), got %s", manifest)
	}
}

func TestRoundFiles290_Catalogue_List_IsUnchangedByRoundFiles(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	e.upload(t, "r1", "depart", "a.wav", tw290fCanonical(2000))
	resp, err := http.Get(e.base + "/api/sounds")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var m struct {
		Cues []map[string]interface{} `json:"cues"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&m)
	for _, c := range m.Cues {
		if c["cue"] == "depart" && c["custom"] == true {
			t.Error("le catalogue global reste « défaut » : un fichier de manche n'est pas un son personnalisé du catalogue")
		}
	}
}

// ---------------------------------------------------------------------------
// Fichier manquant : repli, signalement, jamais de réconciliation silencieuse (D12).
// ---------------------------------------------------------------------------

func TestRoundFiles290_MissingFile_ReportedPresentFalse_RoundsJsonUntouched(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	e.upload(t, "r1", "depart", "a.wav", tw290fCanonical(2000))
	before := e.roundsJSONBytes(t)
	if err := os.Remove(e.roundFile("r1", "depart")); err != nil {
		t.Fatal(err)
	}
	_, m := e.list(t, "r1")
	files, _ := m["files"].([]interface{})
	if len(files) != 1 {
		t.Fatalf("la référence orpheline reste listée, got %v", m["files"])
	}
	f, _ := files[0].(map[string]interface{})
	if f["present"] != false || f["duration_seconds"] != nil {
		t.Errorf("present:false et duration_seconds:null attendus, got %v", f)
	}
	if !bytes.Equal(before, e.roundsJSONBytes(t)) {
		t.Error("D12 : aucune réconciliation automatique de rounds.json")
	}
	// remplacer répare
	if code, _ := e.upload(t, "r1", "depart", "b.wav", tw290fCanonical(2000)); code != http.StatusOK {
		t.Fatal("un nouvel envoi doit être possible")
	}
	_, m = e.list(t, "r1")
	if f := m["files"].([]interface{})[0].(map[string]interface{}); f["present"] != true {
		t.Errorf("après remplacement : present:true, got %v", f)
	}
}

func TestRoundFiles290_DeleteOrphanReference_Works(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	e.upload(t, "r1", "depart", "a.wav", tw290fCanonical(2000))
	_ = os.Remove(e.roundFile("r1", "depart"))
	if code, _ := e.do(t, "DELETE", "/api/rounds/r1/sounds/depart", nil, ""); code != http.StatusOK {
		t.Errorf("« Revenir au fichier global » sur une référence orpheline : 200 attendu, got %d", code)
	}
	if tw290fFiles(e.roundAmbiance(t, "r1"))["depart"] != nil {
		t.Error("la clé orpheline est retirée")
	}
}

// ---------------------------------------------------------------------------
// Course : upload / suppression pendant des ROUNDS_UPDATE (verrou partagé des fichiers de questions).
// ---------------------------------------------------------------------------

func TestRoundFiles290_Concurrent_UploadsAndRoundsUpdates_NeverLoseAFile(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	cues := []string{"depart", "gagne", "perdu", "reveal", "qcm-indice", "memory-carte"}
	var wg sync.WaitGroup
	for i, c := range cues {
		wg.Add(1)
		go func(i int, c string) {
			defer wg.Done()
			if code, _ := e.upload(t, "r1", c, "x.wav", tw290fCanonical(1500+i*100)); code != http.StatusOK {
				t.Errorf("upload %s : 200 attendu, got %d", c, code)
			}
		}(i, c)
	}
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			e.roundsUpdate(t, []map[string]interface{}{
				{"ID": "r1", "NAME": "Blind-test " + string(rune('A'+i)), "QUESTIONS": []string{"1", "2"}},
				{"ID": "r2", "NAME": "Finale", "QUESTIONS": []string{"3"}},
			})
		}(i)
	}
	wg.Wait()
	files := tw290fFiles(e.roundAmbiance(t, "r1"))
	for _, c := range cues {
		if files[c] != "rounds/r1/"+c+".wav" {
			t.Errorf("fichier %s perdu par une écriture concurrente de rounds.json (files=%v)", c, files)
		}
		if _, err := os.Stat(e.roundFile("r1", c)); err != nil {
			t.Errorf("fichier %s absent du disque : %v", c, err)
		}
	}
}

// ---------------------------------------------------------------------------
// POST /api/rounds/{id}/sounds/{cue}/test — joue le fichier EFFECTIF, ignore muted et interrupteurs.
// ---------------------------------------------------------------------------

// tw290fRecorder : Output qui journalise la longueur de chaque PCM joué.
type tw290fRecorder struct {
	mu   sync.Mutex
	lens []int
}

func (r *tw290fRecorder) Play(_ context.Context, pcm io.Reader) error {
	b, _ := io.ReadAll(pcm)
	r.mu.Lock()
	r.lens = append(r.lens, len(b))
	r.mu.Unlock()
	return nil
}
func (r *tw290fRecorder) Close() error { return nil }
func (r *tw290fRecorder) played() []int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]int(nil), r.lens...)
}
func (r *tw290fRecorder) waitFor(t *testing.T, n int) []int {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if g := r.played(); len(g) >= n {
			time.Sleep(60 * time.Millisecond)
			return r.played()
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("%d lecture(s) attendue(s), got %v", n, r.played())
	return nil
}

// tw290fAudio branche un moteur RÉEL (FileBank sur le dossier des sons) sur un enregistreur ; sound.enabled=true.
func tw290fAudio(t *testing.T, e *tw290Env) *tw290fRecorder {
	t.Helper()
	if err := config.PatchJSON([]byte(`{"sound":{"enabled":true,"muted":false,"cues_disabled":{}}}`)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { config.Mutate(func(c *config.Config) { c.Sound = config.SoundConfig{} }) })
	rec := &tw290fRecorder{}
	e.app.httpServer.Sound = e.app
	e.app.soundEngine.Store(audio.NewEngine(audio.Config{Output: rec, Bank: e.app.buildSoundBank()}))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go e.app.sound().Start(ctx)
	return rec
}

func TestRoundFiles290_Test_PlaysTheEffectiveFile_RoundThenGlobal(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	rec := tw290fAudio(t, e)
	tw290fWriteGlobal(t, e, "depart", 100)
	tw290fWriteGlobal(t, e, "gagne", 120)
	e.upload(t, "r1", "depart", "a.wav", tw290fCanonical(250))

	code, m := e.do(t, "POST", "/api/rounds/r1/sounds/depart/test", nil, "")
	if code != http.StatusOK || m["status"] != "ok" || m["played"] != "round" {
		t.Fatalf("200 {status:ok, played:\"round\"} attendu, got %d %v", code, m)
	}
	code, m = e.do(t, "POST", "/api/rounds/r1/sounds/gagne/test", nil, "")
	if code != http.StatusOK || m["played"] != "global" {
		t.Fatalf("cue sans fichier de manche : played=\"global\" attendu, got %d %v", code, m)
	}
	got := rec.waitFor(t, 2)
	if got[0] != 250*audio.FrameSize || got[1] != 120*audio.FrameSize {
		t.Errorf("lectures attendues [fichier de manche, catalogue] = [%d %d], got %v", 250*audio.FrameSize, 120*audio.FrameSize, got)
	}
}

func TestRoundFiles290_Test_IgnoresMutedAndSwitches_ExplicitGesture(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, map[string]interface{}{"sound": map[string]interface{}{"muted": true, "cues": map[string]interface{}{"depart": false}}})
	rec := tw290fAudio(t, e)
	tw290fWriteGlobal(t, e, "depart", 100)
	e.upload(t, "r1", "depart", "a.wav", tw290fCanonical(250))
	e.ready(t, "1") // manche « son coupé », depart désactivée
	if code, m := e.do(t, "POST", "/api/rounds/r1/sounds/depart/test", nil, ""); code != http.StatusOK || m["played"] != "round" {
		t.Fatalf("Tester joue même son coupé / cue désactivée, got %d %v", code, m)
	}
	if got := rec.waitFor(t, 1); got[0] != 250*audio.FrameSize {
		t.Errorf("le fichier de manche attendu, got %v", got)
	}
}

func TestRoundFiles290_Test_UnknownRoundOrCue_404(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	tw290fAudio(t, e)
	for _, p := range []string{"/api/rounds/r99/sounds/depart/test", "/api/rounds/r1/sounds/inconnue/test", "/api/rounds/%2E%2E/sounds/depart/test"} {
		if code, _ := e.do(t, "POST", p, nil, ""); code != http.StatusNotFound {
			t.Errorf("POST %s : 404 attendu, got %d", p, code)
		}
	}
	if code, _ := e.do(t, "GET", "/api/rounds/r1/sounds/depart/test", nil, ""); code != http.StatusMethodNotAllowed && code != http.StatusNotFound {
		t.Errorf("GET sur /test : 405 (ou 404) attendu, got %d", code)
	}
}

// ---------------------------------------------------------------------------
// Lecture réelle : bruitages et décompte jouent le fichier de la manche effective.
// ---------------------------------------------------------------------------

func TestRoundFiles290_Playback_NotifySound_PlaysRoundFileInRound_GlobalOutside(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	rec := tw290fAudio(t, e)
	tw290fWriteGlobal(t, e, "depart", 100)
	tw290fWriteGlobal(t, e, "gagne", 120)
	e.upload(t, "r1", "depart", "a.wav", tw290fCanonical(250))

	e.ready(t, "4") // hors manche
	e.app.notifySound(audio.CueDepart)
	e.ready(t, "1") // en manche
	e.app.notifySound(audio.CueDepart)
	e.app.notifySound(audio.CueGagne) // cue sans fichier de manche
	e.ready(t, "4")
	e.app.notifySound(audio.CueDepart)

	got := rec.waitFor(t, 4)
	want := []int{100, 250, 120, 100}
	for i, w := range want {
		if got[i] != w*audio.FrameSize {
			t.Errorf("lecture %d : %d trames attendues, got %d octets (journal %v)", i, w, got[i], got)
		}
	}
}

func TestRoundFiles290_Playback_RoundFileMissing_PlaysGlobal_NeverSilent(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	rec := tw290fAudio(t, e)
	tw290fWriteGlobal(t, e, "depart", 100)
	e.upload(t, "r1", "depart", "a.wav", tw290fCanonical(250))
	e.ready(t, "1")
	_ = os.Remove(e.roundFile("r1", "depart"))
	e.app.notifySound(audio.CueDepart)
	if got := rec.waitFor(t, 1); got[0] != 100*audio.FrameSize {
		t.Errorf("fichier de manche manquant ⇒ le son GLOBAL joue (jamais de silence), got %v", got)
	}
}

func TestRoundFiles290_Playback_CueDisabledByRound_StillSilent_EvenWithFile(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, map[string]interface{}{"sound": map[string]interface{}{"cues": map[string]interface{}{"depart": false}}})
	rec := tw290fAudio(t, e)
	tw290fWriteGlobal(t, e, "depart", 100)
	e.upload(t, "r1", "depart", "a.wav", tw290fCanonical(250))
	e.ready(t, "1")
	e.app.notifySound(audio.CueDepart)
	time.Sleep(200 * time.Millisecond)
	if got := rec.played(); len(got) != 0 {
		t.Errorf("l'activation et le fichier sont indépendants : cue coupée ⇒ silence même avec un fichier de manche, got %v", got)
	}
}

// Le décompte lit `soundsDir/<files["decompte"]>` (repli sur decompte.wav si le fichier de manche manque).
type tw290fPlayer struct {
	mu    sync.Mutex
	paths []string
}

func (p *tw290fPlayer) Play(path string) error     { p.rec(path); return nil }
func (p *tw290fPlayer) PlayLoop(path string) error { p.rec(path); return nil }
func (p *tw290fPlayer) Pause()                     {}
func (p *tw290fPlayer) Resume()                    {}
func (p *tw290fPlayer) Stop()                      {}
func (p *tw290fPlayer) State() audio.MediaState    { return audio.MediaIdle }
func (p *tw290fPlayer) Close() error               { return nil }
func (p *tw290fPlayer) rec(path string) {
	p.mu.Lock()
	p.paths = append(p.paths, filepath.ToSlash(path))
	p.mu.Unlock()
}
func (p *tw290fPlayer) last() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.paths) == 0 {
		return ""
	}
	return p.paths[len(p.paths)-1]
}

func TestRoundFiles290_Playback_Countdown_UsesRoundFile_GlobalOutside_FallbackWhenMissing(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	tw290fAudio(t, e)
	tw290fWriteGlobal(t, e, "decompte", 200)
	pl := &tw290fPlayer{}
	tw287InstallCountdown(t, e.app, pl)
	e.upload(t, "r1", "decompte", "c.wav", tw290fCanonical(tw290fSeconds(2)))

	e.ready(t, "4")
	e.app.countdownSound().OnAnswerTimerStarted(game.AnswerTimerStart{})
	if got := pl.last(); !strings.HasSuffix(got, "/sounds/decompte.wav") {
		t.Errorf("hors manche : decompte.wav du catalogue attendu, got %q", got)
	}
	e.ready(t, "1")
	e.app.countdownSound().OnAnswerTimerStarted(game.AnswerTimerStart{})
	if got := pl.last(); !strings.HasSuffix(got, "/sounds/rounds/r1/decompte.wav") {
		t.Errorf("en manche : le fichier de décompte de la manche attendu, got %q", got)
	}
	_ = os.Remove(e.roundFile("r1", "decompte"))
	e.app.countdownSound().OnAnswerTimerStarted(game.AnswerTimerStart{})
	if got := pl.last(); !strings.HasSuffix(got, "/sounds/decompte.wav") || strings.Contains(got, "rounds/") {
		t.Errorf("fichier de manche manquant ⇒ repli sur le catalogue, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Sécurité : une référence forgée dans rounds.json ne fait jamais sortir du dossier des sons.
// ---------------------------------------------------------------------------

func TestRoundFiles290_Security_ForgedRoundsJsonReference_NeverEscapesSoundsDir(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	rec := tw290fAudio(t, e)
	tw290fWriteGlobal(t, e, "depart", 100)
	// fichier SECRET hors du dossier des sons, de durée distincte
	secret := filepath.Join(e.dir, "..", "..", "secret.wav")
	if err := os.WriteFile(secret, tw290fCanonical(999), 0o644); err != nil {
		t.Fatal(err)
	}
	// forgeage direct de rounds.json (l'API ne produit jamais ce chemin)
	var f map[string]interface{}
	_ = json.Unmarshal(e.roundsJSONBytes(t), &f)
	rs := f["ROUNDS"].([]interface{})
	r1 := rs[0].(map[string]interface{})
	r1["AMBIANCE"] = map[string]interface{}{"sound": map[string]interface{}{"files": map[string]interface{}{"depart": "../../secret.wav"}}}
	raw, _ := json.Marshal(f)
	if err := os.WriteFile(filepath.Join(e.dir, "rounds.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	rounds.ReloadRounds(e.app) // recharge les manches depuis rounds.json comme au démarrage
	e.ready(t, "1")
	e.app.notifySound(audio.CueDepart)
	time.Sleep(300 * time.Millisecond) // le repli par nom de base (« secret ») n'a pas de fichier catalogue : silence attendu
	for _, l := range rec.played() {
		if l == 999*audio.FrameSize {
			t.Fatal("SÉCURITÉ : le fichier hors du dossier des sons a été lu")
		}
	}
}

func TestRoundFiles290_Security_UploadNeverWritesOutsideRoundsFolder(t *testing.T) {
	e := tw290New(t)
	e.twoRounds(t, nil)
	e.upload(t, "r1", "depart", "../../../evil.wav", tw290fCanonical(2000)) // nom de fichier hostile : ignoré (le chemin est déterministe)
	entries := []string{}
	_ = filepath.Walk(filepath.Join(e.dir, "..", ".."), func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(p, "evil.wav") {
			entries = append(entries, p)
		}
		return nil
	})
	if len(entries) != 0 {
		t.Errorf("le nom de fichier envoyé ne doit jamais servir de chemin, got %v", entries)
	}
	if _, err := os.Stat(e.roundFile("r1", "depart")); err != nil {
		t.Errorf("le fichier doit être écrit sous le chemin déterministe : %v", err)
	}
}
