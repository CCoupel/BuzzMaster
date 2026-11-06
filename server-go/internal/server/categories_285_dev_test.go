// #285 (v11.4.0, LOT-4) — tests dev-backend de la gestion des catégories (contrat http-endpoints.md §Catégories).
package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"buzzcontrol/internal/game"
)

var (
	dev285PNG  = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...)
	dev285WEBP = append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), bytes.Repeat([]byte{0}, 64)...)
)

func dev285Server(t *testing.T) (*HTTPServer, string) {
	t.Helper()
	s, dataDir := setupTestHTTPServer(t)
	s.engine.SetPhase(game.PhaseStopped)
	return s, dataDir
}

func dev285Multipart(t *testing.T, fields map[string]string, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	if filename != "" {
		fw, _ := mw.CreateFormFile("file", filename)
		_, _ = fw.Write(content)
	}
	_ = mw.Close()
	return &buf, mw.FormDataContentType()
}

func dev285Do(s *HTTPServer, method, path string, body *bytes.Buffer, ct string) *httptest.ResponseRecorder {
	if body == nil {
		body = &bytes.Buffer{}
	}
	req := httptest.NewRequest(method, path, body)
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	return rec
}

func dev285JSON(s *HTTPServer, method, path, body string) *httptest.ResponseRecorder {
	return dev285Do(s, method, path, bytes.NewBufferString(body), "application/json")
}

func dev285Create(t *testing.T, s *HTTPServer, name, color string) {
	t.Helper()
	f := map[string]string{"name": name}
	if color != "" {
		f["color"] = color
	}
	body, ct := dev285Multipart(t, f, "i.png", dev285PNG)
	if rec := dev285Do(s, "POST", "/api/categories", body, ct); rec.Code != 200 {
		t.Fatalf("création %q : %d %s", name, rec.Code, rec.Body.String())
	}
}

func dev285Get(t *testing.T, s *HTTPServer) map[string]CategoryInfo {
	t.Helper()
	rec := dev285Do(s, "GET", "/api/categories", nil, "")
	var list []CategoryInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	m := map[string]CategoryInfo{}
	for _, c := range list {
		m[c.Key] = c
	}
	return m
}

func TestCategories285_AllWritesBlockedDuringGame_BeforeValidation(t *testing.T) {
	s, _ := dev285Server(t)
	dev285Create(t, s, "Cinema", "#0ea5e9")
	for _, ph := range []game.GamePhase{game.PhaseStarted, game.PhasePaused, game.PhaseReady, game.PhaseEnroll, game.PhasePrepare} {
		s.engine.SetPhase(ph)
		body, ct := dev285Multipart(t, nil, "", nil) // invalide ET partie en cours ⇒ 409, pas 400
		for _, rq := range []struct {
			m, p string
			b    *bytes.Buffer
			ct   string
		}{
			{"POST", "/api/categories", body, ct},
			{"PUT", "/api/categories/CINEMA", bytes.NewBufferString("{}"), "application/json"},
			{"POST", "/api/categories/CINEMA/icon", body, ct},
			{"DELETE", "/api/categories/CINEMA", nil, ""},
			{"PUT", "/api/categories/..BAD", bytes.NewBufferString("{}"), "application/json"},
		} {
			if rec := dev285Do(s, rq.m, rq.p, rq.b, rq.ct); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "game in progress") {
				t.Errorf("%s %s en phase %s : %d %s, attendu 409 game in progress", rq.m, rq.p, ph, rec.Code, rec.Body.String())
			}
		}
		if rec := dev285Do(s, "GET", "/api/categories", nil, ""); rec.Code != 200 {
			t.Errorf("GET doit rester libre en phase %s", ph)
		}
	}
	for _, ph := range []game.GamePhase{game.PhaseStopped, game.PhaseNewGame, game.PhaseRevealed} {
		s.engine.SetPhase(ph)
		if rec := dev285JSON(s, "PUT", "/api/categories/CINEMA", `{"color":"#112233"}`); rec.Code != 200 {
			t.Errorf("PUT en phase %s : %d %s", ph, rec.Code, rec.Body.String())
		}
	}
}

func TestCategories285_RenameBuiltIn_ColorAndKeyValidation(t *testing.T) {
	s, _ := dev285Server(t)
	if rec := dev285JSON(s, "PUT", "/api/categories/GEOGRAPHY", `{"name":"Monde","color":""}`); rec.Code != 200 {
		t.Fatalf("PUT intégrée : %d %s", rec.Code, rec.Body.String())
	}
	g := dev285Get(t, s)["GEOGRAPHY"]
	if g.Name != "Monde" || g.Color != "" || g.IsCustom || g.ImageURL != "" {
		t.Errorf("GEOGRAPHY après PUT = %+v", g)
	}
	if n, _, c := s.ResolveCategoryMeta("GEOGRAPHY"); n != "Monde" || c != "" {
		t.Errorf("ResolveCategoryMeta = %q %q, attendu la surcharge", n, c)
	}
	for _, tc := range []struct{ path, body string }{
		{"/api/categories/GEOGRAPHY", `{}`},
		{"/api/categories/GEOGRAPHY", `{"color":"red"}`},
		{"/api/categories/GEOGRAPHY", `{"name":"a/b"}`},
		{"/api/categories/A.B", `{"name":"x"}`},
	} {
		if rec := dev285JSON(s, "PUT", tc.path, tc.body); rec.Code != 400 {
			t.Errorf("PUT %s %s : %d, attendu 400", tc.path, tc.body, rec.Code)
		}
	}
	if rec := dev285JSON(s, "PUT", "/api/categories/NOPE", `{"name":"x"}`); rec.Code != 404 {
		t.Errorf("PUT inconnue : %d, attendu 404", rec.Code)
	}
	if rec := dev285Do(s, "DELETE", "/api/categories/GEOGRAPHY", nil, ""); rec.Code != 400 {
		t.Errorf("DELETE intégrée : %d, attendu 400", rec.Code)
	}
	body, ct := dev285Multipart(t, nil, "i.png", dev285PNG)
	if rec := dev285Do(s, "POST", "/api/categories/GEOGRAPHY/icon", body, ct); rec.Code != 400 || !strings.Contains(rec.Body.String(), "emoji") {
		t.Errorf("icône sur intégrée : %d %s, attendu 400", rec.Code, rec.Body.String())
	}
}

func TestCategories285_IconUpload_Security(t *testing.T) {
	s, dataDir := dev285Server(t)
	dev285Create(t, s, "Cinema", "")
	catDir := filepath.Join(dataDir, "files", "categories")
	post := func(filename string, content []byte) *httptest.ResponseRecorder {
		b, ct := dev285Multipart(t, nil, filename, content)
		return dev285Do(s, "POST", "/api/categories/CINEMA/icon", b, ct)
	}
	if rec := post("../x.png", dev285PNG); rec.Code != 200 {
		t.Errorf("nom de fichier hostile mais contenu valide : %d (le nom disque ne dépend pas du Filename)", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "files", "x.png")); err == nil {
		t.Error("le Filename a été utilisé pour construire un chemin")
	}
	if rec := post("a.webp", dev285PNG); rec.Code != 400 {
		t.Errorf("PNG renommé .webp : %d, attendu 400", rec.Code)
	}
	if rec := post("a.png", []byte("<html>x</html>")); rec.Code != 400 {
		t.Errorf("HTML renommé .png : %d, attendu 400", rec.Code)
	}
	if rec := post("a.gif", dev285PNG); rec.Code != 400 {
		t.Errorf("extension hors allowlist : %d, attendu 400", rec.Code)
	}
	big := append(append([]byte{}, dev285PNG...), bytes.Repeat([]byte{1}, 2<<20)...)
	if rec := post("a.png", big); rec.Code != 413 {
		t.Errorf("> 2 Mio : %d, attendu 413", rec.Code)
	}
	// remplacement .png → .webp : l'ancien .png disparaît
	if rec := post("b.webp", dev285WEBP); rec.Code != 200 {
		t.Fatalf("remplacement webp : %d %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(catDir, "CINEMA.png")); err == nil {
		t.Error("l'ancienne image .png doit être supprimée")
	}
	if _, err := os.Stat(filepath.Join(catDir, "CINEMA.webp")); err != nil {
		t.Error("CINEMA.webp doit exister")
	}
	if c := dev285Get(t, s)["CINEMA"]; !strings.HasSuffix(c.ImageURL, "CINEMA.webp") {
		t.Errorf("imageURL = %q", c.ImageURL)
	}
}

func dev285WriteQuestion(t *testing.T, dataDir, id string, q map[string]interface{}) {
	t.Helper()
	dir := filepath.Join(dataDir, "files", "questions", id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	q["ID"] = id
	data, _ := json.Marshal(q)
	if err := os.WriteFile(filepath.Join(dir, "question.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCategories285_Delete_InUse409_Reassign_Delete_HistoryMerge(t *testing.T) {
	s, dataDir := dev285Server(t)
	hp := filepath.Join(t.TempDir(), "config", "history.json")
	s.engine.SetHistoryPath(hp)
	dev285Create(t, s, "Cinema", "#0ea5e9")
	dev285WriteQuestion(t, dataDir, "1", map[string]interface{}{"CATEGORY": "CINEMA", "QUESTION": "q1"})
	dev285WriteQuestion(t, dataDir, "2", map[string]interface{}{"CATEGORY": "HISTORY", "QUESTION": "q2",
		"MOTION_CARDS": []interface{}{
			map[string]interface{}{"ID": "c1", "CATEGORY": "CINEMA", "RECTO_IMAGE": "/question/2/c1.png", "RAFALE_CATEGORIES": []interface{}{"CINEMA", "HISTORY"}},
			map[string]interface{}{"ID": "c2", "CATEGORY": "HISTORY"},
		}})
	_ = os.WriteFile(filepath.Join(dataDir, "files", "questions", "2", "c1.png"), dev285PNG, 0644)
	dev285WriteQuestion(t, dataDir, "3", map[string]interface{}{"CATEGORY": "SCIENCE", "RAFALE_CATEGORIES": []interface{}{"CINEMA"}})
	if _, err := s.engine.UpsertRafaleQuestion(game.RafaleQuestion{Question: "r", Answer: "a", Category: "CINEMA", Difficulty: 1}); err != nil {
		t.Fatal(err)
	}
	s.engine.AddGameEvent(game.GameEvent{QuestionID: "1", QuestionCategory: "CINEMA", CategoryDisplayName: "Cinema", EventType: "POINTS_AWARDED", Points: 10})
	s.engine.AddGameEvent(game.GameEvent{QuestionID: "x", EventType: "POINTS_AWARDED", Points: 30, CategoryBreakdown: map[string]int{"CINEMA": 10, "HISTORY": 20}})
	s.engine.SaveHistory()

	// 409 in use, sans mode
	rec := dev285Do(s, "DELETE", "/api/categories/CINEMA", nil, "")
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), "category in use") || !strings.Contains(rec.Body.String(), `"events":2`) {
		t.Fatalf("DELETE sans mode : %d %s", rec.Code, rec.Body.String())
	}
	if c := dev285Get(t, s)["CINEMA"]; c.QuestionCount != 2 || c.RafaleCount != 1 { // question 1 + carte c1
		t.Errorf("compteurs = %+v, attendu 2 / 1", c)
	}

	// reassign
	rec = dev285JSON(s, "DELETE", "/api/categories/CINEMA", `{"mode":"reassign"}`)
	if rec.Code != 200 {
		t.Fatalf("reassign : %d %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Questions struct{ Reassigned, CardsReassigned, RafaleFiltersCleaned int }
		History   struct {
			Events, Points int
			Backup         string
		}
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Questions.Reassigned != 1 || res.Questions.CardsReassigned != 1 || res.Questions.RafaleFiltersCleaned != 2 || res.History.Events != 2 || res.History.Backup == "" {
		t.Errorf("réponse = %s", rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(hp), res.History.Backup)); err != nil {
		t.Errorf(".bak absent : %v", err)
	}
	var sum int
	for _, ev := range s.engine.GetHistory() {
		if ev.QuestionCategory == "CINEMA" {
			t.Error("QUESTION_CATEGORY CINEMA encore présent")
		}
		if _, ok := ev.CategoryBreakdown["CINEMA"]; ok {
			t.Error("CATEGORY_BREAKDOWN CINEMA encore présent")
		}
		if ev.CategoryBreakdown != nil {
			for _, v := range ev.CategoryBreakdown {
				sum += v
			}
			if ev.CategoryBreakdown[""] != 10 || ev.CategoryBreakdown["HISTORY"] != 20 {
				t.Errorf("fusion BREAKDOWN = %v", ev.CategoryBreakdown)
			}
		}
	}
	if sum != 30 {
		t.Errorf("somme des parts = %d, attendu 30 (POINTS)", sum)
	}
	if _, ok := dev285Get(t, s)["CINEMA"]; ok {
		t.Error("CINEMA doit avoir disparu de GET")
	}
	maps := ReadQuestionMaps(filepath.Join(dataDir, "files", "questions"))
	for _, q := range maps {
		if c, _ := q["CATEGORY"].(string); c == "CINEMA" {
			t.Errorf("question %v encore en CINEMA", q["ID"])
		}
	}
	if rq, _ := s.engine.SnapshotRafaleReservoir(); len(rq) != 1 || rq[0].Category != "" {
		t.Errorf("réservoir = %+v, attendu 1 réponse sans catégorie", rq)
	}

	// mode delete sur une autre catégorie
	dev285Create(t, s, "Sport Extreme", "")
	dev285WriteQuestion(t, dataDir, "4", map[string]interface{}{"CATEGORY": "SPORT_EXTREME"})
	dev285WriteQuestion(t, dataDir, "5", map[string]interface{}{"CATEGORY": "HISTORY",
		"MOTION_CARDS": []interface{}{map[string]interface{}{"ID": "c9", "CATEGORY": "SPORT_EXTREME"}}})
	rec = dev285JSON(s, "DELETE", "/api/categories/SPORT_EXTREME", `{"mode":"delete"}`)
	if rec.Code != 200 {
		t.Fatalf("delete : %d %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dataDir, "files", "questions", "4")); err == nil {
		t.Error("le dossier de la question 4 doit être supprimé")
	}
	host := ReadQuestionMaps(filepath.Join(dataDir, "files", "questions"))
	found5 := false
	for _, q := range host {
		if q["ID"] == "5" {
			found5 = true
			if cards, _ := q["MOTION_CARDS"].([]interface{}); len(cards) != 0 {
				t.Errorf("la carte doit être retirée de l'hôte, reste %v", cards)
			}
		}
	}
	if !found5 {
		t.Error("la question hôte doit être conservée")
	}
}

func TestCategories285_HistoryBackups_RotateToFive_AndFailureLeavesEverythingUntouched(t *testing.T) {
	e := game.NewEngine()
	dir := t.TempDir()
	hp := filepath.Join(dir, "history.json")
	e.SetHistoryPath(hp)
	for i := 0; i < 7; i++ {
		e.AddGameEvent(game.GameEvent{QuestionCategory: "K", EventType: "POINTS_AWARDED", Points: 5})
		e.SaveHistory()
		if _, _, _, err := e.MergeHistoryCategoryWithBackup("K"); err != nil {
			t.Fatal(err)
		}
		_ = os.Rename(hp, hp) // no-op
		// force des horodatages distincts
		entries, _ := os.ReadDir(dir)
		for _, en := range entries {
			if strings.HasPrefix(en.Name(), "history.json.bak-") && !strings.Contains(en.Name(), "-t") {
				_ = os.Rename(filepath.Join(dir, en.Name()), filepath.Join(dir, en.Name()+"-t"+string(rune('a'+i))))
			}
		}
	}
	n := 0
	entries, _ := os.ReadDir(dir)
	for _, en := range entries {
		if strings.HasPrefix(en.Name(), "history.json.bak-") {
			n++
		}
	}
	if n > 5 {
		t.Errorf("%d sauvegardes conservées, max 5", n)
	}

	// échec d'écriture ⇒ historique en mémoire inchangé
	e2 := game.NewEngine()
	blocker := filepath.Join(t.TempDir(), "file")
	_ = os.WriteFile(blocker, []byte("x"), 0644)
	e2.SetHistoryPath(filepath.Join(blocker, "sub", "history.json")) // le « dossier » est un fichier
	e2.AddGameEvent(game.GameEvent{QuestionCategory: "K", EventType: "POINTS_AWARDED", Points: 5})
	if _, _, _, err := e2.MergeHistoryCategoryWithBackup("K"); err == nil {
		t.Fatal("erreur attendue")
	}
	if got := e2.GetHistory(); got[0].QuestionCategory != "K" {
		t.Error("l'historique en mémoire doit rester inchangé après un échec")
	}
	// aucun événement concerné ⇒ pas de .bak
	e3 := game.NewEngine()
	e3.SetHistoryPath(filepath.Join(t.TempDir(), "history.json"))
	if ev, _, b, err := e3.MergeHistoryCategoryWithBackup("K"); ev != 0 || b != "" || err != nil {
		t.Errorf("sans événement : %d %q %v", ev, b, err)
	}
}
