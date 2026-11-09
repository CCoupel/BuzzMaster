// #235 (v11.6.0, LOT-2) — sons de CARTES MEMOTION à l'upload (POST /questions) : champs multipart
// `motion_card_<cardID>_sound[_cleared|_timer_delayed|_loop]`, PRÉSERVATION serveur par ID de carte à la
// ré-édition, nettoyage des fichiers, et rangement du son d'hôte par type (MEMORY / ENTRACTE / MEMOTION hôte).
// Source normative : contracts/sound.md §14.1–14.4, contracts/http-endpoints.md « POST /questions — sons de
// cartes MEMOTION (#235) », contracts/models.md (MotionCard). Écrit depuis le contrat avant le code (rouge).
//
// ⚠️ Nom du champ de suppression : le CONTRAT dit `_sound_cleared` (le plan disait `_sound_clear`) — le contrat fait foi.
//
// LE test prioritaire est le « piège de reconstruction » (§14.4) : handleUploadQuestion reconstruit MOTION_CARDS
// depuis le payload client ; une ré-édition SANS champ son ne doit jamais détruire le son d'une carte existante.
// Frontière : HTTP + question.json relu comme map JSON (aucune dépendance aux noms de champs Go). Préfixe tw235.
package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type tw235Form struct {
	typ    string
	cards  []map[string]interface{}
	fields map[string]string
	files  map[string][]byte // champ multipart -> contenu (nom de fichier x.wav)
	title  string
	time   string // "" = 30
}

func tw235Post(t *testing.T, s *HTTPServer, f tw235Form) *httptest.ResponseRecorder {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	title := f.title
	if title == "" {
		title = "Question #235"
	}
	typ := f.typ
	if typ == "" {
		typ = "MEMOTION"
	}
	timeStr := f.time
	if timeStr == "" {
		timeStr = "30"
	}
	for k, v := range map[string]string{"number": "7", "question": title, "answer": "", "points": "1", "time": timeStr, "type": typ} {
		_ = mw.WriteField(k, v)
	}
	if f.cards != nil {
		raw, _ := json.Marshal(f.cards)
		_ = mw.WriteField("motion_cards", string(raw))
	}
	for k, v := range f.fields {
		_ = mw.WriteField(k, v)
	}
	for field, content := range f.files {
		part, err := mw.CreateFormFile(field, "x.wav")
		if err != nil {
			t.Fatalf("CreateFormFile(%s) : %v", field, err)
		}
		_, _ = part.Write(content)
	}
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/questions", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	return w
}

func tw235QDir(dataDir string) string { return filepath.Join(dataDir, "files", "questions", "7") }

func tw235ReadQuestion(t *testing.T, dataDir string) map[string]interface{} {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(tw235QDir(dataDir), "question.json"))
	if err != nil {
		t.Fatalf("question.json illisible : %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("question.json invalide : %v", err)
	}
	return m
}

func tw235Card(t *testing.T, q map[string]interface{}, id string) map[string]interface{} {
	t.Helper()
	cards, _ := q["MOTION_CARDS"].([]interface{})
	for _, c := range cards {
		if m, ok := c.(map[string]interface{}); ok && m["ID"] == id {
			return m
		}
	}
	t.Fatalf("carte %q absente de MOTION_CARDS : %v", id, q["MOTION_CARDS"])
	return nil
}

func tw235Cards(ids ...string) []map[string]interface{} {
	var out []map[string]interface{}
	for _, id := range ids {
		out = append(out, map[string]interface{}{"ID": id, "RECTO_THEME": "thème " + id, "DIFFICULTY": 1})
	}
	return out
}

var tw235SoundURL = regexp.MustCompile(`^/question/7/motion_([A-Za-z0-9_-]+)_sound_\d+\.wav$`)

func tw235OK(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Upload d'un son de carte (§14.4).
// ---------------------------------------------------------------------------

func TestCardSound235_Upload_WritesSoundFlagsAndCanonicalFile(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	w := tw235Post(t, s, tw235Form{
		cards: tw235Cards("c1", "c2"),
		fields: map[string]string{
			"motion_card_c1_sound_timer_delayed": "true",
			"motion_card_c1_sound_loop":          "true",
		},
		files: map[string][]byte{"motion_card_c1_sound": tw219hCanonicalWAV(2)},
	})
	tw235OK(t, w)

	q := tw235ReadQuestion(t, dataDir)
	c1 := tw235Card(t, q, "c1")
	url, _ := c1["SOUND"].(string)
	if m := tw235SoundURL.FindStringSubmatch(url); m == nil || m[1] != "c1" {
		t.Fatalf("SOUND de la carte c1 attendu /question/7/motion_c1_sound_<rand>.wav, got %q", url)
	}
	if c1["SOUND_TIMER_DELAYED"] != true || c1["SOUND_LOOP"] != true {
		t.Errorf("SOUND_TIMER_DELAYED et SOUND_LOOP attendus true, got %v / %v", c1["SOUND_TIMER_DELAYED"], c1["SOUND_LOOP"])
	}
	tw232uAssertStoredCanonical(t, filepath.Join(tw235QDir(dataDir), filepath.Base(url)))

	c2 := tw235Card(t, q, "c2")
	for _, k := range []string{"SOUND", "SOUND_TIMER_DELAYED", "SOUND_LOOP"} {
		if _, present := c2[k]; present {
			t.Errorf("carte sans son : champ %s ne doit pas être écrit (omitempty), got %v", k, c2[k])
		}
	}
}

func TestCardSound235_Upload_NonCanonicalWAV_ConvertedAndStoredCanonical(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	w := tw235Post(t, s, tw235Form{
		cards: tw235Cards("c1"),
		files: map[string][]byte{"motion_card_c1_sound": tw232uWAV(1, 1, 48000, 24, 2)},
	})
	tw235OK(t, w)
	url, _ := tw235Card(t, tw235ReadQuestion(t, dataDir), "c1")["SOUND"].(string)
	if url == "" {
		t.Fatal("SOUND attendu après upload converti")
	}
	tw232uAssertStoredCanonical(t, filepath.Join(tw235QDir(dataDir), filepath.Base(url)))
}

func TestCardSound235_Upload_FlagsWithoutSound_NotWritten(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	w := tw235Post(t, s, tw235Form{
		cards: tw235Cards("c1"),
		fields: map[string]string{
			"motion_card_c1_sound_timer_delayed": "true",
			"motion_card_c1_sound_loop":          "true",
		},
	})
	tw235OK(t, w)
	c1 := tw235Card(t, tw235ReadQuestion(t, dataDir), "c1")
	for _, k := range []string{"SOUND", "SOUND_TIMER_DELAYED", "SOUND_LOOP"} {
		if _, present := c1[k]; present {
			t.Errorf("sans son attaché, %s ne doit pas être écrit (contrat : « écrits seulement si un son est attaché »), got %v", k, c1[k])
		}
	}
}

func TestCardSound235_Upload_CompressedWAV_Refused400_NothingWritten(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	w := tw235Post(t, s, tw235Form{
		cards: tw235Cards("c1"),
		files: map[string][]byte{"motion_card_c1_sound": tw232uAssemble(0x55, 2, 44100, 16, make([]byte, 4000))},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("WAV compressé : attendu 400, got %d : %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(tw235QDir(dataDir), "question.json")); !os.IsNotExist(err) {
		t.Error("un son de carte refusé doit rejeter TOUTE la sauvegarde (question.json absent)")
	}
}

func TestCardSound235_Upload_Over30s_Refused400(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w := tw235Post(t, s, tw235Form{
		cards: tw235Cards("c1"),
		files: map[string][]byte{"motion_card_c1_sound": tw219hCanonicalWAV(31)},
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("31 s > 30 s (même limite que le son de question) : attendu 400, got %d : %s", w.Code, w.Body.String())
	}
}

// Réponse : `converted`/`source_format` (#232) et `card_warnings` (additif, `[]` sinon, jamais omis).
func TestCardSound235_Upload_Response_CardWarningsAlwaysPresent(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	w := tw235Post(t, s, tw235Form{cards: tw235Cards("c1"), files: map[string][]byte{"motion_card_c1_sound": tw219hCanonicalWAV(1)}})
	tw235OK(t, w)
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("réponse non JSON : %v", err)
	}
	cw, ok := resp["card_warnings"]
	if !ok {
		t.Fatalf("`card_warnings` absent de la réponse (additif, `[]` quand vide — jamais omis) : %s", w.Body.String())
	}
	if arr, isArr := cw.([]interface{}); !isArr || len(arr) != 0 {
		t.Errorf("card_warnings attendu [] (aucun avertissement), got %v", cw)
	}
}

// ---------------------------------------------------------------------------
// PRÉSERVATION à la ré-édition — « piège majeur » du §14.4.
// ---------------------------------------------------------------------------

func tw235SeedTwoSoundCards(t *testing.T, s *HTTPServer, dataDir string) (url1, url2 string) {
	t.Helper()
	w := tw235Post(t, s, tw235Form{
		cards: tw235Cards("c1", "c2"),
		fields: map[string]string{
			"motion_card_c1_sound_timer_delayed": "true",
			"motion_card_c2_sound_loop":          "true",
		},
		files: map[string][]byte{
			"motion_card_c1_sound": tw219hCanonicalWAV(1),
			"motion_card_c2_sound": tw219hCanonicalWAV(2),
		},
	})
	tw235OK(t, w)
	q := tw235ReadQuestion(t, dataDir)
	url1, _ = tw235Card(t, q, "c1")["SOUND"].(string)
	url2, _ = tw235Card(t, q, "c2")["SOUND"].(string)
	if url1 == "" || url2 == "" {
		t.Fatalf("setup invalide : sons de cartes absents (%q, %q)", url1, url2)
	}
	return
}

// Édition du TITRE de la question hôte, payload de cartes SANS aucun champ son ni fichier :
// sons de cartes intacts, octet pour octet dans question.json (§14.4 « Test exigé »).
func TestCardSound235_Reedit_WithoutSoundFields_PreservesEverything(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	url1, url2 := tw235SeedTwoSoundCards(t, s, dataDir)
	before := tw235ReadQuestion(t, dataDir)

	// Le frontend renvoie les cartes SANS SOUND (jamais de préservation côté client).
	w := tw235Post(t, s, tw235Form{title: "Titre modifié", cards: tw235Cards("c1", "c2")})
	tw235OK(t, w)

	after := tw235ReadQuestion(t, dataDir)
	for id, want := range map[string]string{"c1": url1, "c2": url2} {
		got := tw235Card(t, after, id)
		if got["SOUND"] != want {
			t.Errorf("carte %s : SOUND détruit par la ré-édition (got %v, want %q) — « destruction silencieuse »", id, got["SOUND"], want)
		}
		bc := tw235Card(t, before, id)
		for _, k := range []string{"SOUND_TIMER_DELAYED", "SOUND_LOOP"} {
			if got[k] != bc[k] {
				t.Errorf("carte %s : %s modifié par la ré-édition (avant %v, après %v)", id, k, bc[k], got[k])
			}
		}
		if _, err := os.Stat(filepath.Join(tw235QDir(dataDir), filepath.Base(want))); err != nil {
			t.Errorf("carte %s : le fichier son a disparu du disque : %v", id, err)
		}
	}
	if after["QUESTION"] != "Titre modifié" {
		t.Errorf("sanity : le titre aurait dû être modifié, got %v", after["QUESTION"])
	}
}

// Modifier une AUTRE carte (ou en ajouter une) ne touche pas aux sons existants.
func TestCardSound235_Reedit_OtherCardChanged_SoundsStayPerID(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	url1, url2 := tw235SeedTwoSoundCards(t, s, dataDir)

	cards := tw235Cards("c1", "c2", "c3")
	cards[1]["RECTO_THEME"] = "c2 renommée"
	tw235OK(t, tw235Post(t, s, tw235Form{cards: cards}))

	q := tw235ReadQuestion(t, dataDir)
	if tw235Card(t, q, "c1")["SOUND"] != url1 || tw235Card(t, q, "c2")["SOUND"] != url2 {
		t.Error("les sons de c1/c2 doivent être conservés par ID malgré l'ajout de c3 et le renommage de c2")
	}
	if _, present := tw235Card(t, q, "c3")["SOUND"]; present {
		t.Error("une carte NOUVELLE ne doit hériter d'aucun son")
	}
}

// L'ordre des cartes peut changer : la préservation est par ID, pas par position.
func TestCardSound235_Reedit_ReorderedCards_PreservedByID(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	url1, url2 := tw235SeedTwoSoundCards(t, s, dataDir)
	tw235OK(t, tw235Post(t, s, tw235Form{cards: tw235Cards("c2", "c1")}))
	q := tw235ReadQuestion(t, dataDir)
	if tw235Card(t, q, "c1")["SOUND"] != url1 || tw235Card(t, q, "c2")["SOUND"] != url2 {
		t.Error("la préservation doit se faire par ID de carte, pas par position")
	}
}

// Un nouveau fichier REMPLACE le son de cette carte seulement.
func TestCardSound235_Reedit_NewFile_ReplacesOnlyThatCard(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	url1, url2 := tw235SeedTwoSoundCards(t, s, dataDir)
	w := tw235Post(t, s, tw235Form{cards: tw235Cards("c1", "c2"), files: map[string][]byte{"motion_card_c1_sound": tw219hCanonicalWAV(3)}})
	tw235OK(t, w)
	q := tw235ReadQuestion(t, dataDir)
	n1, _ := tw235Card(t, q, "c1")["SOUND"].(string)
	if n1 == "" || n1 == url1 {
		t.Errorf("c1 : nouveau SOUND attendu (≠ %q), got %q", url1, n1)
	}
	if _, err := os.Stat(filepath.Join(tw235QDir(dataDir), filepath.Base(n1))); err != nil {
		t.Errorf("c1 : nouveau fichier absent : %v", err)
	}
	if tw235Card(t, q, "c2")["SOUND"] != url2 {
		t.Error("c2 ne doit pas être touchée par le remplacement du son de c1")
	}
}

// Les autres champs de la carte (cartes SANS son) restent octet pour octet : pas de clé SOUND* parasite.
func TestCardSound235_Reedit_CardsWithoutSound_NoSoundKeysAppear(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw235OK(t, tw235Post(t, s, tw235Form{cards: tw235Cards("c1")}))
	tw235OK(t, tw235Post(t, s, tw235Form{title: "autre", cards: tw235Cards("c1")}))
	c1 := tw235Card(t, tw235ReadQuestion(t, dataDir), "c1")
	for _, k := range []string{"SOUND", "SOUND_TIMER_DELAYED", "SOUND_LOOP"} {
		if _, present := c1[k]; present {
			t.Errorf("carte sans son : clé %s ne doit jamais apparaître (non-régression §14.5)", k)
		}
	}
}

// Les indicateurs d'une carte qui garde son son peuvent être modifiés sans renvoyer de fichier.
func TestCardSound235_Reedit_FlagsUpdatedWithoutNewFile(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	url1, _ := tw235SeedTwoSoundCards(t, s, dataDir)
	tw235OK(t, tw235Post(t, s, tw235Form{
		cards:  tw235Cards("c1", "c2"),
		fields: map[string]string{"motion_card_c1_sound_loop": "true", "motion_card_c1_sound_timer_delayed": "true"},
	}))
	c1 := tw235Card(t, tw235ReadQuestion(t, dataDir), "c1")
	if c1["SOUND"] != url1 {
		t.Errorf("SOUND conservé attendu %q, got %v", url1, c1["SOUND"])
	}
	if c1["SOUND_LOOP"] != true {
		t.Errorf("SOUND_LOOP=true attendu après la ré-édition, got %v", c1["SOUND_LOOP"])
	}
}

// ---------------------------------------------------------------------------
// Suppression : `_sound_cleared` et disparition de la carte retirent le fichier (§14.4).
// ---------------------------------------------------------------------------

func TestCardSound235_Cleared_RemovesFieldFlagsAndFile(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	url1, url2 := tw235SeedTwoSoundCards(t, s, dataDir)
	tw235OK(t, tw235Post(t, s, tw235Form{
		cards:  tw235Cards("c1", "c2"),
		fields: map[string]string{"motion_card_c1_sound_cleared": "true"},
	}))
	q := tw235ReadQuestion(t, dataDir)
	c1 := tw235Card(t, q, "c1")
	for _, k := range []string{"SOUND", "SOUND_TIMER_DELAYED", "SOUND_LOOP"} {
		if _, present := c1[k]; present {
			t.Errorf("c1 effacée : clé %s doit être supprimée, got %v", k, c1[k])
		}
	}
	if _, err := os.Stat(filepath.Join(tw235QDir(dataDir), filepath.Base(url1))); !os.IsNotExist(err) {
		t.Error("c1 effacée : le fichier son doit être supprimé du disque")
	}
	if tw235Card(t, q, "c2")["SOUND"] != url2 {
		t.Error("c2 ne doit pas être touchée par l'effacement du son de c1")
	}
}

func TestCardSound235_CardRemoved_FileRemovedFromDisk(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	url1, url2 := tw235SeedTwoSoundCards(t, s, dataDir)
	tw235OK(t, tw235Post(t, s, tw235Form{cards: tw235Cards("c2")}))
	if _, err := os.Stat(filepath.Join(tw235QDir(dataDir), filepath.Base(url1))); !os.IsNotExist(err) {
		t.Error("carte c1 supprimée : son fichier son doit disparaître (removeCardMedia)")
	}
	if _, err := os.Stat(filepath.Join(tw235QDir(dataDir), filepath.Base(url2))); err != nil {
		t.Errorf("carte c2 conservée : son fichier doit rester : %v", err)
	}
}

// ---------------------------------------------------------------------------
// Son d'hôte par type : MEMORY (champ commun), ENTRACTE et hôte MEMOTION (pas de « chrono démarre »).
// ---------------------------------------------------------------------------

func tw235HostSoundFields(typ string) tw235Form {
	return tw235Form{
		typ: typ,
		fields: map[string]string{
			"sound_timer_delayed": "true",
			"sound_loop":          "true",
		},
		files: map[string][]byte{"sound": tw219hCanonicalWAV(2)},
	}
}

func TestHostSound235_Memory_SoundAndFlagsWritten(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	f := tw235HostSoundFields("MEMORY")
	f.fields["memory_mode"] = "SOLO"
	tw235OK(t, tw235Post(t, s, f))
	q := tw235ReadQuestion(t, dataDir)
	if snd, _ := q["SOUND"].(string); snd == "" {
		t.Fatal("MEMORY : SOUND attendu (champ commun à tous les types, §14.1/§14.2)")
	}
	if q["SOUND_LOOP"] != true {
		t.Errorf("MEMORY : SOUND_LOOP=true attendu, got %v", q["SOUND_LOOP"])
	}
	// MEMORY : SOUND_TIMER_DELAYED écrit tel quel (sans effet si USE_TIMER est faux — pas de refus).
	if q["SOUND_TIMER_DELAYED"] != true {
		t.Errorf("MEMORY : SOUND_TIMER_DELAYED=true attendu (écrit même sans effet), got %v", q["SOUND_TIMER_DELAYED"])
	}
}

func TestHostSound235_Entracte_SoundLoopKept_TimerDelayedIgnored(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	tw235OK(t, tw235Post(t, s, tw235HostSoundFields("ENTRACTE")))
	q := tw235ReadQuestion(t, dataDir)
	if snd, _ := q["SOUND"].(string); snd == "" {
		t.Fatal("ENTRACTE : SOUND attendu")
	}
	if q["SOUND_LOOP"] != true {
		t.Errorf("ENTRACTE : SOUND_LOOP=true attendu, got %v", q["SOUND_LOOP"])
	}
	if v, present := q["SOUND_TIMER_DELAYED"]; present && v != false {
		t.Errorf("ENTRACTE : sound_timer_delayed est ignoré (aucun chronomètre) — écrit false/absent, got %v", v)
	}
}

func TestHostSound235_MemotionHost_TimerDelayedIgnored(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	f := tw235HostSoundFields("MEMOTION")
	f.cards = tw235Cards("c1")
	tw235OK(t, tw235Post(t, s, f))
	q := tw235ReadQuestion(t, dataDir)
	if snd, _ := q["SOUND"].(string); snd == "" {
		t.Fatal("hôte MEMOTION : SOUND attendu")
	}
	if v, present := q["SOUND_TIMER_DELAYED"]; present && v != false {
		t.Errorf("hôte MEMOTION : pas de chrono global ⇒ SOUND_TIMER_DELAYED ignoré (false/absent), got %v", v)
	}
	if q["SOUND_LOOP"] != true {
		t.Errorf("hôte MEMOTION : SOUND_LOOP=true attendu, got %v", q["SOUND_LOOP"])
	}
}

// Le son d'hôte MEMOTION est préservé indépendamment des sons de cartes (deux niveaux distincts).
func TestHostSound235_MemotionHostAndCardSounds_BothSurviveTitleEdit(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	f := tw235Form{
		cards: tw235Cards("c1"),
		files: map[string][]byte{"sound": tw219hCanonicalWAV(1), "motion_card_c1_sound": tw219hCanonicalWAV(1)},
	}
	tw235OK(t, tw235Post(t, s, f))
	q := tw235ReadQuestion(t, dataDir)
	host, _ := q["SOUND"].(string)
	card, _ := tw235Card(t, q, "c1")["SOUND"].(string)
	if host == "" || card == "" || host == card {
		t.Fatalf("setup invalide : sons hôte=%q carte=%q", host, card)
	}
	tw235OK(t, tw235Post(t, s, tw235Form{title: "autre titre", cards: tw235Cards("c1")}))
	q = tw235ReadQuestion(t, dataDir)
	if q["SOUND"] != host || tw235Card(t, q, "c1")["SOUND"] != card {
		t.Errorf("hôte et carte doivent survivre à la ré-édition : hôte %v (want %q), carte %v (want %q)", q["SOUND"], host, tw235Card(t, q, "c1")["SOUND"], card)
	}
	if !strings.HasPrefix(card, "/question/7/motion_c1_sound_") {
		t.Errorf("nom du son de carte inattendu : %q", card)
	}
}
