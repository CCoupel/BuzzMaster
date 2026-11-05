// #285 (v11.4.0, LOT-4) — gestion des catégories par la frontière HTTP : GET enrichi, POST (+color, validation d'icône),
// PUT, POST …/icon, DELETE (reassign / delete, cartes MEMOTION comprises), 409 « partie en cours » sur les 4 écritures,
// fusion de l'historique dans « Sans catégorie » et sauvegarde .bak. Source normative : contracts/http-endpoints.md
// §Catégories + §Historique (cf8c69d5).
//
// RÈGLE docs/TEST_PROCEDURE.md §2.3 : aucun symbole du dev — uniquement HTTP + JSON brut + méthodes EXISTANTES du
// moteur (SetPhase, AddGameEvent, SetHistoryPath, SaveHistory, UpsertRafaleQuestion, SnapshotRafaleReservoir…) ;
// le fichier compile avant le code et échoue à l'exécution (RED). Fixtures sous t.TempDir() uniquement. Préfixe tw285.
package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"buzzcontrol/internal/game"
)

// ---------- fixtures ----------

var tw285PNG = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...)
var tw285JPG = append([]byte("\xff\xd8\xff\xe0\x00\x10JFIF"), bytes.Repeat([]byte{0}, 64)...)
var tw285WEBP = append([]byte("RIFF\x24\x00\x00\x00WEBPVP8 "), bytes.Repeat([]byte{0}, 64)...)

type tw285Env struct {
	srv     *HTTPServer
	dataDir string
}

func (e tw285Env) catDir() string   { return filepath.Join(e.dataDir, "files", "categories") }
func (e tw285Env) qDir() string     { return filepath.Join(e.dataDir, "files", "questions") }
func (e tw285Env) histPath() string { return filepath.Join(e.dataDir, "config", "history.json") }

func tw285New(t *testing.T) tw285Env {
	t.Helper()
	srv, dataDir := setupTestHTTPServer(t)
	e := tw285Env{srv, dataDir}
	_ = os.MkdirAll(e.catDir(), 0o755)
	_ = os.MkdirAll(e.qDir(), 0o755)
	srv.engine.SetHistoryPath(e.histPath())
	srv.engine.SetRafalePath(filepath.Join(dataDir, "files", "rafale", "reservoir.json"))
	srv.engine.SetRafaleUsedPath(filepath.Join(dataDir, "config", "rafale_used.json"))
	srv.engine.SetPhase(game.PhaseStopped)
	return e
}

// custom crée une catégorie personnalisée sur disque (image + sidecar).
func (e tw285Env) custom(t *testing.T, key, name, color string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(e.catDir(), key+".png"), tw285PNG, 0o644); err != nil {
		t.Fatal(err)
	}
	sc := map[string]string{"name": name}
	if color != "" {
		sc["color"] = color
	}
	raw, _ := json.Marshal(sc)
	if err := os.WriteFile(filepath.Join(e.catDir(), key+".json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

// question écrit un question.json brut.
func (e tw285Env) question(t *testing.T, id string, fields map[string]interface{}) {
	t.Helper()
	q := map[string]interface{}{"ID": id, "NUMBER": id, "QUESTION": "Q" + id, "ANSWER": "A", "TYPE": "SPEEDY", "POINTS": "1", "TIME": "20", "ORDER": 1}
	for k, v := range fields {
		q[k] = v
	}
	dir := filepath.Join(e.qDir(), id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(q)
	if err := os.WriteFile(filepath.Join(dir, "question.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func (e tw285Env) readQuestion(t *testing.T, id string) map[string]interface{} {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(e.qDir(), id, "question.json"))
	if err != nil {
		t.Fatalf("question %s illisible : %v", id, err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func (e tw285Env) rafale(t *testing.T, id, cat string) {
	t.Helper()
	if _, err := e.srv.engine.UpsertRafaleQuestion(game.RafaleQuestion{ID: id, Question: "Q" + id, Answer: "A", Category: game.QuestionCategory(cat), Difficulty: 1}); err != nil {
		t.Fatalf("UpsertRafaleQuestion : %v", err)
	}
}

func (e tw285Env) reservoirCategories(t *testing.T) map[string]string {
	t.Helper()
	qs, _ := e.srv.engine.SnapshotRafaleReservoir()
	out := map[string]string{}
	for _, q := range qs {
		out[q.ID] = string(q.Category)
	}
	return out
}

func (e tw285Env) do(method, url string, body []byte, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	e.srv.mux.ServeHTTP(w, req)
	return w
}

func (e tw285Env) putJSON(key string, body string) *httptest.ResponseRecorder {
	return e.do("PUT", "/api/categories/"+key, []byte(body), "application/json")
}

func (e tw285Env) del(key, body string) *httptest.ResponseRecorder {
	return e.do("DELETE", "/api/categories/"+key, []byte(body), "application/json")
}

// multipart construit un corps multipart (champs + fichier `file` de nom filename).
func tw285Multipart(fields map[string]string, filename string, content []byte) ([]byte, string) {
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	if filename != "" {
		part, _ := mw.CreateFormFile("file", filename)
		_, _ = part.Write(content)
	}
	_ = mw.Close()
	return buf.Bytes(), mw.FormDataContentType()
}

func (e tw285Env) post(fields map[string]string, filename string, content []byte) *httptest.ResponseRecorder {
	body, ct := tw285Multipart(fields, filename, content)
	return e.do("POST", "/api/categories", body, ct)
}

func (e tw285Env) icon(key, filename string, content []byte) *httptest.ResponseRecorder {
	body, ct := tw285Multipart(nil, filename, content)
	return e.do("POST", "/api/categories/"+key+"/icon", body, ct)
}

type tw285Cat struct {
	Key           string `json:"key"`
	Name          string `json:"name"`
	ImageURL      string `json:"imageURL"`
	Color         string `json:"color"`
	IsCustom      bool   `json:"isCustom"`
	QuestionCount int    `json:"questionCount"`
	RafaleCount   int    `json:"rafaleCount"`
}

func (e tw285Env) list(t *testing.T) map[string]tw285Cat {
	t.Helper()
	w := e.do("GET", "/api/categories", nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/categories : %d %s", w.Code, w.Body.String())
	}
	var cats []tw285Cat
	if err := json.Unmarshal(w.Body.Bytes(), &cats); err != nil {
		t.Fatal(err)
	}
	out := map[string]tw285Cat{}
	for _, c := range cats {
		out[c.Key] = c
	}
	return out
}

func tw285Err(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var m map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	s, _ := m["error"].(string)
	return s
}

func tw285Files(t *testing.T, dir string) []string {
	t.Helper()
	ents, _ := os.ReadDir(dir)
	var out []string
	for _, en := range ents {
		out = append(out, en.Name())
	}
	sort.Strings(out)
	return out
}

// ---------- GET ----------

func TestCategories285_GET_BuiltinDefaults_CustomWithoutColorIsEmpty_NoGrayFallback(t *testing.T) {
	e := tw285New(t)
	e.custom(t, "MA_CATEGORIE", "Ma Catégorie", "")
	cats := e.list(t)
	if g := cats["GEOGRAPHY"]; g.Color != "#3b82f6" || g.IsCustom || g.ImageURL != "" {
		t.Errorf("GEOGRAPHY = %+v, attendu couleur d'origine, intégrée, sans image", g)
	}
	m := cats["MA_CATEGORIE"]
	if !m.IsCustom || m.Name != "Ma Catégorie" || m.Color != "" {
		t.Errorf("custom sans couleur = %+v, attendu color \"\" (le repli #6b7280 est retiré de l'API)", m)
	}
	if !strings.Contains(e.do("GET", "/api/categories", nil, "").Body.String(), `"color":""`) {
		t.Error(`la clé "color" doit être présente avec la valeur "" (jamais omise)`)
	}
}

func TestCategories285_GET_BuiltinSidecarOverridesNameAndColor_EmptyColorMeansNone(t *testing.T) {
	e := tw285New(t)
	_ = os.WriteFile(filepath.Join(e.catDir(), "GEOGRAPHY.json"), []byte(`{"name":"Géo","color":"#112233"}`), 0o644)
	_ = os.WriteFile(filepath.Join(e.catDir(), "HISTORY.json"), []byte(`{"color":""}`), 0o644)
	cats := e.list(t)
	if g := cats["GEOGRAPHY"]; g.Name != "Géo" || g.Color != "#112233" {
		t.Errorf("GEOGRAPHY surchargée = %+v", g)
	}
	if h := cats["HISTORY"]; h.Color != "" {
		t.Errorf("HISTORY color \"\" explicite ⇒ aucune couleur, got %q", h.Color)
	}
}

func TestCategories285_GET_BuiltinImageStillIgnored(t *testing.T) {
	e := tw285New(t)
	_ = os.WriteFile(filepath.Join(e.catDir(), "GEOGRAPHY.png"), tw285PNG, 0o644)
	if g := e.list(t)["GEOGRAPHY"]; g.ImageURL != "" || g.IsCustom {
		t.Errorf("une image à clé intégrée est ignorée : %+v", g)
	}
}

func TestCategories285_GET_Counts_QuestionsCardsAndReservoir(t *testing.T) {
	e := tw285New(t)
	e.custom(t, "CINEMA", "Cinéma", "#0ea5e9")
	e.question(t, "1", map[string]interface{}{"CATEGORY": "CINEMA"})
	e.question(t, "2", map[string]interface{}{"CATEGORY": "CINEMA"})
	e.question(t, "3", map[string]interface{}{"CATEGORY": "GEOGRAPHY"})
	e.question(t, "4", map[string]interface{}{"TYPE": "MEMOTION", "MOTION_CARDS": []interface{}{
		map[string]interface{}{"ID": "c1", "RECTO_THEME": "t", "DIFFICULTY": 1, "CATEGORY": "CINEMA"},
		map[string]interface{}{"ID": "c2", "RECTO_THEME": "t", "DIFFICULTY": 1, "CATEGORY": "GEOGRAPHY"},
	}})
	e.rafale(t, "r1", "CINEMA")
	e.rafale(t, "r2", "CINEMA")
	e.rafale(t, "r3", "GEOGRAPHY")
	c := e.list(t)["CINEMA"]
	if c.QuestionCount != 3 || c.RafaleCount != 2 {
		t.Errorf("CINEMA : questionCount=%d (2 questions + 1 carte), rafaleCount=%d (2) ; got %+v", c.QuestionCount, c.RafaleCount, c)
	}
	if g := e.list(t)["GEOGRAPHY"]; g.QuestionCount != 2 || g.RafaleCount != 1 {
		t.Errorf("GEOGRAPHY : %+v, attendu 2 (1 question + 1 carte) / 1", g)
	}
}

// ---------- POST ----------

func TestCategories285_POST_ColorStored_ResponseIsFullInfo(t *testing.T) {
	e := tw285New(t)
	w := e.post(map[string]string{"name": "Cinéma", "color": "#0ea5e9"}, "icon.png", tw285PNG)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	var c tw285Cat
	_ = json.Unmarshal(w.Body.Bytes(), &c)
	if c.Color != "#0ea5e9" || !c.IsCustom || c.QuestionCount != 0 || c.RafaleCount != 0 || c.ImageURL == "" {
		t.Errorf("réponse = %+v", c)
	}
	if got := e.list(t)[c.Key]; got.Color != "#0ea5e9" {
		t.Errorf("GET ensuite : %+v", got)
	}
}

func TestCategories285_POST_InvalidColor_400(t *testing.T) {
	e := tw285New(t)
	for _, col := range []string{"red", "#12345", "#gggggg", "0ea5e9", "#0ea5e9ff"} {
		if w := e.post(map[string]string{"name": "X" + strings.ReplaceAll(col, "#", ""), "color": col}, "i.png", tw285PNG); w.Code != http.StatusBadRequest {
			t.Errorf("color %q : attendu 400, got %d", col, w.Code)
		}
	}
}

func TestCategories285_POST_IconValidation(t *testing.T) {
	e := tw285New(t)
	cases := []struct {
		name, file string
		content    []byte
		want       int
	}{
		{"HTML renommé .png", "x.png", []byte("<html><script>alert(1)</script></html>"), 400},
		{"PNG renommé .webp", "x.webp", tw285PNG, 400},
		{"JPEG renommé .png", "x.png", tw285JPG, 400},
		{"extension hors allowlist", "x.gif", []byte("GIF89a......"), 400},
		{"> 2 Mio", "x.png", append(append([]byte{}, tw285PNG...), make([]byte, 2<<20)...), 413},
	}
	for i, c := range cases {
		w := e.post(map[string]string{"name": "Cat" + string(rune('A'+i))}, c.file, c.content)
		if w.Code != c.want {
			t.Errorf("%s : attendu %d, got %d : %s", c.name, c.want, w.Code, w.Body.String())
		}
	}
	if left := tw285Files(t, e.catDir()); len(left) != 0 {
		t.Errorf("un refus ne doit laisser aucun fichier : %v", left)
	}
}

func TestCategories285_POST_FilenameTraversalNeverUsed_NameDerivedFromKey(t *testing.T) {
	e := tw285New(t)
	w := e.post(map[string]string{"name": "Evil"}, "../../x.png", tw285PNG)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200 (le nom de fichier client est ignoré), got %d : %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(e.dataDir, "x.png")); err == nil {
		t.Error("le nom du client a été utilisé pour écrire hors du dossier des catégories")
	}
	if _, err := os.Stat(filepath.Join(e.dataDir, "files", "x.png")); err == nil {
		t.Error("le nom du client a été utilisé pour écrire hors du dossier des catégories")
	}
	if _, err := os.Stat(filepath.Join(e.catDir(), "EVIL.png")); err != nil {
		t.Errorf("nom disque attendu <KEY>.png : %v (dossier : %v)", err, tw285Files(t, e.catDir()))
	}
}

func TestCategories285_POST_JpegExtensionNormalizedToJpg(t *testing.T) {
	e := tw285New(t)
	if w := e.post(map[string]string{"name": "Photo"}, "p.JPEG", tw285JPG); w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(e.catDir(), "PHOTO.jpg")); err != nil {
		t.Errorf(".jpeg ⇒ .jpg attendu : %v", tw285Files(t, e.catDir()))
	}
}

// ---------- 409 « partie en cours » ----------

func TestCategories285_Locked409_OnTheFourWrites_InLivePhases(t *testing.T) {
	for _, phase := range []game.GamePhase{game.PhaseStarted, game.PhasePaused, game.PhaseReady, game.PhaseEnroll, game.PhasePrepare, game.PhaseCountdown} {
		t.Run(string(phase), func(t *testing.T) {
			e := tw285New(t)
			e.custom(t, "CINEMA", "Cinéma", "")
			e.srv.engine.SetPhase(phase)
			calls := map[string]*httptest.ResponseRecorder{
				"POST":      e.post(map[string]string{"name": "Nouvelle"}, "i.png", tw285PNG),
				"PUT":       e.putJSON("CINEMA", `{"name":"Cine"}`),
				"POST icon": e.icon("CINEMA", "i.png", tw285PNG),
				"DELETE":    e.del("CINEMA", `{}`),
			}
			for name, w := range calls {
				if w.Code != http.StatusConflict || tw285Err(t, w) != "game in progress" {
					t.Errorf("%s en phase %s : attendu 409 game in progress, got %d %s", name, phase, w.Code, w.Body.String())
				}
			}
			if got := e.list(t)["CINEMA"]; got.Name != "Cinéma" {
				t.Errorf("rien ne doit avoir été écrit : %+v", got)
			}
		})
	}
}

func TestCategories285_Locked409_EvaluatedBeforeAnyValidation(t *testing.T) {
	e := tw285New(t)
	e.srv.engine.SetPhase(game.PhaseStarted)
	if w := e.putJSON("CINEMA", `{pas du json`); w.Code != http.StatusConflict {
		t.Errorf("PUT corps invalide en partie : 409 attendu (pas 400), got %d", w.Code)
	}
	if w := e.putJSON("pas-une-cle", `{}`); w.Code != http.StatusConflict {
		t.Errorf("PUT clé invalide (routable) en partie : 409 attendu avant la validation de clé, got %d", w.Code)
	}
	if w := e.post(nil, "", nil); w.Code != http.StatusConflict {
		t.Errorf("POST sans nom ni fichier en partie : 409 attendu, got %d", w.Code)
	}
	if w := e.icon("GEOGRAPHY", "x.png", []byte("pas une image")); w.Code != http.StatusConflict {
		t.Errorf("icône invalide sur intégrée en partie : 409 attendu, got %d", w.Code)
	}
	if w := e.del("GEOGRAPHY", `{}`); w.Code != http.StatusConflict {
		t.Errorf("DELETE intégrée en partie : 409 (garde en premier), got %d", w.Code)
	}
}

func TestCategories285_NotLocked_InStoppedNewGameRevealed_AndGETAlwaysFree(t *testing.T) {
	for _, phase := range []game.GamePhase{game.PhaseStopped, game.PhaseNewGame, game.PhaseRevealed} {
		t.Run(string(phase), func(t *testing.T) {
			e := tw285New(t)
			e.custom(t, "CINEMA", "Cinéma", "")
			e.srv.engine.SetPhase(phase)
			if w := e.putJSON("CINEMA", `{"name":"Cine"}`); w.Code != http.StatusOK {
				t.Errorf("PUT en phase %s : 200 attendu, got %d %s", phase, w.Code, w.Body.String())
			}
			if w := e.icon("CINEMA", "i.png", tw285PNG); w.Code != http.StatusOK {
				t.Errorf("icône en phase %s : 200 attendu, got %d %s", phase, w.Code, w.Body.String())
			}
			if w := e.post(map[string]string{"name": "Nouvelle"}, "i.png", tw285PNG); w.Code != http.StatusOK {
				t.Errorf("POST en phase %s : 200 attendu, got %d %s", phase, w.Code, w.Body.String())
			}
			if w := e.del("CINEMA", `{}`); w.Code != http.StatusOK {
				t.Errorf("DELETE en phase %s : 200 attendu (inutilisée), got %d %s", phase, w.Code, w.Body.String())
			}
		})
	}
	e := tw285New(t)
	e.srv.engine.SetPhase(game.PhaseStarted)
	if w := e.do("GET", "/api/categories", nil, ""); w.Code != http.StatusOK {
		t.Errorf("GET reste libre en partie, got %d", w.Code)
	}
}

// ---------- PUT ----------

func TestCategories285_PUT_RenameBuiltin_AndColor_ReflectedByGET(t *testing.T) {
	e := tw285New(t)
	w := e.putJSON("GEOGRAPHY", `{"name":"Géo du monde","color":"#ff00aa"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	var c tw285Cat
	_ = json.Unmarshal(w.Body.Bytes(), &c)
	if c.Key != "GEOGRAPHY" || c.Name != "Géo du monde" || c.Color != "#ff00aa" || c.IsCustom {
		t.Errorf("réponse = %+v", c)
	}
	if g := e.list(t)["GEOGRAPHY"]; g.Name != "Géo du monde" || g.Color != "#ff00aa" {
		t.Errorf("GET = %+v", g)
	}
}

func TestCategories285_PUT_MergesFieldByField_KeyImmutable(t *testing.T) {
	e := tw285New(t)
	e.custom(t, "CINEMA", "Cinéma", "#0ea5e9")
	if w := e.putJSON("CINEMA", `{"name":"Septième art"}`); w.Code != http.StatusOK {
		t.Fatalf("rename : %d %s", w.Code, w.Body.String())
	}
	if c := e.list(t)["CINEMA"]; c.Name != "Septième art" || c.Color != "#0ea5e9" {
		t.Errorf("le champ absent doit être conservé : %+v", c)
	}
	if w := e.putJSON("CINEMA", `{"color":""}`); w.Code != http.StatusOK {
		t.Fatalf("color \"\" : %d %s", w.Code, w.Body.String())
	}
	if c := e.list(t)["CINEMA"]; c.Name != "Septième art" || c.Color != "" {
		t.Errorf("color \"\" = aucune, nom conservé : %+v", c)
	}
	e.putJSON("CINEMA", `{"key":"AUTRE","name":"Ciné"}`)
	cats := e.list(t)
	if _, ok := cats["AUTRE"]; ok {
		t.Error("la clé est immuable : AUTRE ne doit pas exister")
	}
	if cats["CINEMA"].Name != "Ciné" {
		t.Errorf("CINEMA = %+v", cats["CINEMA"])
	}
}

func TestCategories285_PUT_DoesNotRewriteQuestionsReservoirOrHistory(t *testing.T) {
	e := tw285New(t)
	e.custom(t, "CINEMA", "Cinéma", "")
	e.question(t, "1", map[string]interface{}{"CATEGORY": "CINEMA"})
	before, _ := os.ReadFile(filepath.Join(e.qDir(), "1", "question.json"))
	e.rafale(t, "r1", "CINEMA")
	e.srv.engine.AddGameEvent(game.GameEvent{Timestamp: 1, QuestionID: "1", QuestionCategory: "CINEMA", CategoryDisplayName: "Cinéma", EventType: "POINTS_AWARDED", Points: 5, WinnerType: "TEAM", TeamName: "A"})
	e.putJSON("CINEMA", `{"name":"Septième art"}`)
	after, _ := os.ReadFile(filepath.Join(e.qDir(), "1", "question.json"))
	if !bytes.Equal(before, after) {
		t.Error("PUT ne doit pas réécrire les question.json")
	}
	if e.reservoirCategories(t)["r1"] != "CINEMA" {
		t.Error("PUT ne doit pas toucher le réservoir")
	}
	if h := e.srv.engine.GetHistory(); len(h) != 1 || h[0].QuestionCategory != "CINEMA" || h[0].CategoryDisplayName != "Cinéma" {
		t.Errorf("l'historique garde ses snapshots : %+v", h)
	}
}

func TestCategories285_PUT_Invalid_400_Unknown_404_Method_405(t *testing.T) {
	e := tw285New(t)
	e.custom(t, "CINEMA", "Cinéma", "")
	bad := []string{`{}`, `{"name":""}`, `{"name":"` + strings.Repeat("a", 51) + `"}`, `{"name":"a<b>"}`, `{"color":"rouge"}`, `{"color":"#12345"}`, `{"name":1}`, `pas du json`}
	for _, b := range bad {
		if w := e.putJSON("CINEMA", b); w.Code != http.StatusBadRequest {
			t.Errorf("PUT %s : attendu 400, got %d", b, w.Code)
		}
	}
	if w := e.putJSON("INCONNUE", `{"name":"x"}`); w.Code != http.StatusNotFound {
		t.Errorf("clé inconnue : 404 attendu, got %d", w.Code)
	}
	if w := e.do("PATCH", "/api/categories/CINEMA", []byte(`{}`), "application/json"); w.Code != http.StatusMethodNotAllowed {
		t.Errorf("PATCH : 405 attendu, got %d", w.Code)
	}
}

func TestCategories285_PUT_TraversalKeys_Rejected_NothingWritten(t *testing.T) {
	e := tw285New(t)
	for _, key := range []string{"..%2F..%2Fetc", "cinema", "A%2Fb", "..", "A.B", strings.Repeat("A", 51)} {
		w := e.putJSON(key, `{"name":"x"}`)
		if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound && w.Code != http.StatusTemporaryRedirect {
			t.Errorf("clé %q : 400/404 (ou 307 de nettoyage de chemin) attendu, got %d", key, w.Code)
		}
	}
	if left := tw285Files(t, e.catDir()); len(left) != 0 {
		t.Errorf("aucun fichier ne doit être créé par une clé invalide : %v", left)
	}
}

// ---------- POST …/icon ----------

func TestCategories285_Icon_ReplaceChangesExtension_OldImagePurged_URLChanges(t *testing.T) {
	e := tw285New(t)
	e.custom(t, "CINEMA", "Cinéma", "#0ea5e9")
	before := e.list(t)["CINEMA"].ImageURL
	w := e.icon("CINEMA", "new.webp", tw285WEBP)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	var c tw285Cat
	_ = json.Unmarshal(w.Body.Bytes(), &c)
	if !strings.HasSuffix(c.ImageURL, ".webp") || c.ImageURL == before {
		t.Errorf("imageURL = %q (avant %q)", c.ImageURL, before)
	}
	files := tw285Files(t, e.catDir())
	if len(files) != 2 || files[0] != "CINEMA.json" || files[1] != "CINEMA.webp" {
		t.Errorf("l'ancien CINEMA.png doit disparaître, jamais deux images ; dossier : %v", files)
	}
	if c.Name != "Cinéma" || c.Color != "#0ea5e9" {
		t.Errorf("le sidecar est conservé : %+v", c)
	}
}

func TestCategories285_Icon_BuiltinRefused400_NothingWritten(t *testing.T) {
	e := tw285New(t)
	w := e.icon("GEOGRAPHY", "i.png", tw285PNG)
	if w.Code != http.StatusBadRequest || tw285Err(t, w) != "built-in categories keep their emoji" {
		t.Errorf("attendu 400 « built-in categories keep their emoji », got %d %s", w.Code, w.Body.String())
	}
	if left := tw285Files(t, e.catDir()); len(left) != 0 {
		t.Errorf("rien ne doit être écrit : %v", left)
	}
}

func TestCategories285_Icon_Security(t *testing.T) {
	e := tw285New(t)
	e.custom(t, "CINEMA", "Cinéma", "")
	cases := []struct {
		name, file string
		content    []byte
		want       int
	}{
		{"> 2 Mio", "i.png", append(append([]byte{}, tw285PNG...), make([]byte, 2<<20)...), 413},
		{"PNG renommé .webp", "i.webp", tw285PNG, 400},
		{"HTML renommé .png", "i.png", []byte("<html>x</html>"), 400},
		{"extension .svg", "i.svg", []byte("<svg/>"), 400},
		{"extension absente", "icon", tw285PNG, 400},
	}
	for _, c := range cases {
		if w := e.icon("CINEMA", c.file, c.content); w.Code != c.want {
			t.Errorf("%s : attendu %d, got %d : %s", c.name, c.want, w.Code, w.Body.String())
		}
	}
	if w := e.icon("CINEMA", "../../evil.png", tw285PNG); w.Code != http.StatusOK {
		t.Errorf("nom client traversal : le nom est ignoré, 200 attendu, got %d", w.Code)
	}
	if _, err := os.Stat(filepath.Join(e.dataDir, "evil.png")); err == nil {
		t.Error("écriture hors dossier par le nom du client")
	}
	if got := tw285Files(t, e.catDir()); len(got) != 2 {
		t.Errorf("après refus puis un remplacement valide : une image + un sidecar, got %v", got)
	}
	if w := e.icon("INCONNUE", "i.png", tw285PNG); w.Code != http.StatusNotFound {
		t.Errorf("clé inconnue : 404, got %d", w.Code)
	}
	if w := e.icon("..%2Fetc", "i.png", tw285PNG); w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("clé traversal : 400/404, got %d", w.Code)
	}
}

// ---------- DELETE ----------

func TestCategories285_DELETE_Builtin400_Unknown404_BadMethod(t *testing.T) {
	e := tw285New(t)
	if w := e.del("GEOGRAPHY", `{}`); w.Code != http.StatusBadRequest || tw285Err(t, w) != "built-in categories cannot be deleted" {
		t.Errorf("intégrée : 400 « built-in categories cannot be deleted », got %d %s", w.Code, w.Body.String())
	}
	if w := e.del("INCONNUE", `{}`); w.Code != http.StatusNotFound {
		t.Errorf("inconnue : 404, got %d", w.Code)
	}
	if w := e.del("..%2Fetc", `{}`); w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("traversal : 400/404, got %d", w.Code)
	}
}

func (e tw285Env) seedCinema(t *testing.T) {
	t.Helper()
	e.custom(t, "CINEMA", "Cinéma", "#0ea5e9")
	e.custom(t, "AUTRE", "Autre", "")
	e.question(t, "1", map[string]interface{}{"CATEGORY": "CINEMA"})
	e.question(t, "2", map[string]interface{}{"CATEGORY": "CINEMA", "TYPE": "RAFALE", "RAFALE_CATEGORIES": []string{"CINEMA", "GEOGRAPHY"}})
	e.question(t, "3", map[string]interface{}{"CATEGORY": "GEOGRAPHY", "RAFALE_CATEGORIES": []string{"CINEMA"}})
	e.question(t, "4", map[string]interface{}{"CATEGORY": "AUTRE"})
	e.question(t, "5", map[string]interface{}{"TYPE": "MEMOTION", "CATEGORY": "GEOGRAPHY", "MOTION_CARDS": []interface{}{
		map[string]interface{}{"ID": "c1", "RECTO_THEME": "t", "DIFFICULTY": 1, "CATEGORY": "CINEMA"},
		map[string]interface{}{"ID": "c2", "RECTO_THEME": "t", "DIFFICULTY": 1, "CATEGORY": "GEOGRAPHY"},
	}})
	e.rafale(t, "r1", "CINEMA")
	e.rafale(t, "r2", "CINEMA")
	e.rafale(t, "r3", "GEOGRAPHY")
}

func TestCategories285_DELETE_InUse_NoMode_409_WithCountsPayload(t *testing.T) {
	e := tw285New(t)
	e.seedCinema(t)
	e.srv.engine.AddGameEvent(game.GameEvent{Timestamp: 1, QuestionID: "1", QuestionCategory: "CINEMA", EventType: "POINTS_AWARDED", Points: 10, WinnerType: "TEAM", TeamName: "A"})
	e.srv.engine.AddGameEvent(game.GameEvent{Timestamp: 2, QuestionID: "x", EventType: "POINTS_AWARDED", Points: 4, WinnerType: "TEAM", TeamName: "A", CategoryBreakdown: map[string]int{"CINEMA": 3, "HISTORY": 1}})
	for _, body := range []string{``, `{}`, `{"mode":"nimportequoi"}`} {
		w := e.del("CINEMA", body)
		if w.Code != http.StatusConflict || tw285Err(t, w) != "category in use" {
			t.Fatalf("corps %q : 409 category in use attendu, got %d %s", body, w.Code, w.Body.String())
		}
	}
	var p struct {
		Questions int `json:"questions"`
		Rafale    int `json:"rafale"`
		History   struct {
			Events int `json:"events"`
			Points int `json:"points"`
		} `json:"history"`
	}
	_ = json.Unmarshal(e.del("CINEMA", `{}`).Body.Bytes(), &p)
	if p.Questions != 3 || p.Rafale != 2 || p.History.Events != 2 || p.History.Points < 13 {
		t.Errorf("payload = %+v, attendu questions=3 (2 questions + 1 carte), rafale=2, history.events=2 (QUESTION_CATEGORY ou BREAKDOWN), points ≥ 13", p)
	}
	if _, ok := e.list(t)["CINEMA"]; !ok {
		t.Error("un 409 ne supprime rien")
	}
}

func TestCategories285_DELETE_Reassign_QuestionsCardsReservoirFilters(t *testing.T) {
	e := tw285New(t)
	e.seedCinema(t)
	w := e.del("CINEMA", `{"mode":"reassign"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	var r struct {
		Deleted   string `json:"deleted"`
		Mode      string `json:"mode"`
		Questions struct {
			Reassigned, Deleted, CardsReassigned, CardsDeleted, RafaleFiltersCleaned int
		} `json:"questions"`
		Rafale  struct{ Reassigned, Deleted int } `json:"rafale"`
		History struct {
			Events, Points int
			Backup         string
		} `json:"history"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &r)
	if r.Deleted != "CINEMA" || r.Mode != "reassign" || r.Questions.Reassigned != 2 || r.Questions.Deleted != 0 ||
		r.Questions.CardsReassigned != 1 || r.Questions.CardsDeleted != 0 || r.Rafale.Reassigned != 2 || r.Rafale.Deleted != 0 {
		t.Errorf("réponse = %s", w.Body.String())
	}
	if r.Questions.RafaleFiltersCleaned < 2 {
		t.Errorf("rafaleFiltersCleaned = %d, attendu ≥ 2 (questions 2 et 3)", r.Questions.RafaleFiltersCleaned)
	}
	if r.History.Backup != "" {
		t.Errorf("aucun événement d'historique concerné ⇒ backup \"\" (got %q)", r.History.Backup)
	}
	// questions réaffectées, toujours là
	for _, id := range []string{"1", "2"} {
		if c, _ := e.readQuestion(t, id)["CATEGORY"].(string); c != "" {
			t.Errorf("question %s : CATEGORY = %q, attendu \"\"", id, c)
		}
	}
	if c, _ := e.readQuestion(t, "3")["CATEGORY"].(string); c != "GEOGRAPHY" {
		t.Errorf("une autre catégorie reste intacte : %q", c)
	}
	// RAFALE_CATEGORIES nettoyées dans les deux questions
	for id, want := range map[string][]interface{}{"2": {"GEOGRAPHY"}, "3": nil} {
		got, _ := e.readQuestion(t, id)["RAFALE_CATEGORIES"].([]interface{})
		if len(got) != len(want) || (len(want) == 1 && got[0] != want[0]) {
			t.Errorf("question %s RAFALE_CATEGORIES = %v, attendu %v (clé retirée, liste vide = toutes)", id, got, want)
		}
	}
	// carte MEMOTION réaffectée, hôte et autre carte intactes
	cards, _ := e.readQuestion(t, "5")["MOTION_CARDS"].([]interface{})
	if len(cards) != 2 {
		t.Fatalf("reassign : les 2 cartes restent, got %d", len(cards))
	}
	if c0, _ := cards[0].(map[string]interface{})["CATEGORY"].(string); c0 != "" {
		t.Errorf("carte c1 : CATEGORY = %q, attendu \"\"", c0)
	}
	if c1, _ := cards[1].(map[string]interface{})["CATEGORY"].(string); c1 != "GEOGRAPHY" {
		t.Errorf("carte c2 intacte : %q", c1)
	}
	// réservoir
	res := e.reservoirCategories(t)
	if res["r1"] != "" || res["r2"] != "" || res["r3"] != "GEOGRAPHY" {
		t.Errorf("réservoir = %v, attendu r1/r2 sans catégorie, r3 intacte", res)
	}
	// fichiers et liste
	if _, ok := e.list(t)["CINEMA"]; ok {
		t.Error("CINEMA ne doit plus être listée")
	}
	for _, f := range []string{"CINEMA.png", "CINEMA.json"} {
		if _, err := os.Stat(filepath.Join(e.catDir(), f)); err == nil {
			t.Errorf("%s doit être supprimé", f)
		}
	}
	if _, err := os.Stat(filepath.Join(e.catDir(), "AUTRE.png")); err != nil {
		t.Error("une autre catégorie personnalisée est intacte")
	}
}

func TestCategories285_DELETE_DeleteMode_RemovesQuestionsCardsReservoir_KeepsHostAndOthers(t *testing.T) {
	e := tw285New(t)
	e.seedCinema(t)
	// média de la carte c1 dans le dossier de l'hôte
	_ = os.WriteFile(filepath.Join(e.qDir(), "5", "card_c1.png"), tw285PNG, 0o644)
	w := e.del("CINEMA", `{"mode":"delete"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	var r struct {
		Questions struct{ Reassigned, Deleted, CardsReassigned, CardsDeleted int } `json:"questions"`
		Rafale    struct{ Reassigned, Deleted int }                                `json:"rafale"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &r)
	if r.Questions.Deleted != 2 || r.Questions.CardsDeleted != 1 || r.Questions.Reassigned != 0 || r.Rafale.Deleted != 2 {
		t.Errorf("réponse = %s", w.Body.String())
	}
	for _, id := range []string{"1", "2"} {
		if _, err := os.Stat(filepath.Join(e.qDir(), id)); err == nil {
			t.Errorf("le dossier de la question %s doit être supprimé", id)
		}
	}
	for _, id := range []string{"3", "4", "5"} {
		if _, err := os.Stat(filepath.Join(e.qDir(), id, "question.json")); err != nil {
			t.Errorf("la question %s doit être conservée : %v", id, err)
		}
	}
	// hôte MEMOTION conservée, carte c1 retirée, c2 gardée
	cards, _ := e.readQuestion(t, "5")["MOTION_CARDS"].([]interface{})
	if len(cards) != 1 || cards[0].(map[string]interface{})["ID"] != "c2" {
		t.Errorf("la carte de la catégorie est retirée de son hôte (conservée) : %v", cards)
	}
	res := e.reservoirCategories(t)
	if _, ok := res["r1"]; ok {
		t.Error("r1 doit être purgée du réservoir")
	}
	if _, ok := res["r2"]; ok {
		t.Error("r2 doit être purgée du réservoir")
	}
	if res["r3"] != "GEOGRAPHY" {
		t.Errorf("r3 intacte : %v", res)
	}
	// RAFALE_CATEGORIES nettoyées aussi en mode delete (question 3 reste)
	if got, _ := e.readQuestion(t, "3")["RAFALE_CATEGORIES"].([]interface{}); len(got) != 0 {
		t.Errorf("RAFALE_CATEGORIES de la question 3 : %v, attendu vide", got)
	}
}

func TestCategories285_DELETE_Unused_NoMode_200(t *testing.T) {
	e := tw285New(t)
	e.custom(t, "VIDE", "Vide", "")
	w := e.del("VIDE", ``)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	if _, ok := e.list(t)["VIDE"]; ok {
		t.Error("VIDE doit avoir disparu")
	}
	if got := tw285Files(t, e.catDir()); len(got) != 0 {
		t.Errorf("dossier attendu vide : %v", got)
	}
}

// ---------- historique : fusion dans « Sans catégorie » + .bak ----------

// settleHistory laisse finir les sauvegardes asynchrones d'AddGameEvent (safeGo) puis fige un état disque stable.
func (e tw285Env) settleHistory(t *testing.T) {
	t.Helper()
	time.Sleep(250 * time.Millisecond)
	if err := e.srv.engine.SaveHistory(); err != nil {
		t.Fatalf("setup : %v", err)
	}
}

func (e tw285Env) seedHistory(t *testing.T) {
	t.Helper()
	e.custom(t, "CINEMA", "Cinéma", "#0ea5e9")
	ev := func(ts int64, cat string, pts int, bd map[string]int) game.GameEvent {
		return game.GameEvent{Timestamp: ts, QuestionID: "q", QuestionCategory: cat, CategoryDisplayName: map[bool]string{true: "Cinéma"}[cat == "CINEMA"],
			CategoryImageURL: map[bool]string{true: "/files/categories/CINEMA.png"}[cat == "CINEMA"], CategoryColor: map[bool]string{true: "#0ea5e9"}[cat == "CINEMA"],
			EventType: "POINTS_AWARDED", WinnerType: "TEAM", TeamName: "A", Points: pts, CategoryBreakdown: bd}
	}
	e.srv.engine.AddGameEvent(ev(1, "CINEMA", 10, nil))
	e.srv.engine.AddGameEvent(ev(2, "GEOGRAPHY", 7, nil))
	e.srv.engine.AddGameEvent(ev(3, "", 5, map[string]int{"CINEMA": 3, "HISTORY": 2}))
	e.srv.engine.AddGameEvent(ev(4, "", 6, map[string]int{"": 1, "CINEMA": 5}))
	e.settleHistory(t)
}

func (e tw285Env) palmaresTotal(t *testing.T) (byCat map[string]int, sum int) {
	t.Helper()
	byCat = map[string]int{}
	for _, p := range decodePalmares(t, e.srv) {
		byCat[p.Category] = p.TotalPoints
		sum += p.TotalPoints
	}
	return
}

func TestCategories285_History_Merge_PreservesSum_CreditsUnknown_Snapshots(t *testing.T) {
	e := tw285New(t)
	e.seedHistory(t)
	_, sumBefore := e.palmaresTotal(t)
	w := e.del("CINEMA", `{"mode":"reassign"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	var r struct {
		History struct {
			Events, Points int
			Backup         string
		} `json:"history"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &r)
	if r.History.Events != 3 || !strings.HasPrefix(r.History.Backup, "history.json.bak-") {
		t.Errorf("history = %+v, attendu 3 événements (1 + 2 par BREAKDOWN) et un backup horodaté", r.History)
	}
	byCat, sumAfter := e.palmaresTotal(t)
	if sumAfter != sumBefore {
		t.Errorf("la somme du palmarès doit être conservée : %d → %d", sumBefore, sumAfter)
	}
	if _, ok := byCat["CINEMA"]; ok {
		t.Errorf("plus aucun crédit CINEMA : %v", byCat)
	}
	if byCat["UNKNOWN"] != 10+3+5+1 {
		t.Errorf("UNKNOWN (« Sans catégorie ») = %d, attendu %d (10 + 3 + 5 + 1) ; palmarès %v", byCat["UNKNOWN"], 19, byCat)
	}
	if byCat["GEOGRAPHY"] != 7 || byCat["HISTORY"] != 2 {
		t.Errorf("les autres catégories sont intactes : %v", byCat)
	}
	for _, ev := range e.srv.engine.GetHistory() {
		if ev.QuestionCategory == "CINEMA" || ev.CategoryDisplayName == "Cinéma" || ev.CategoryColor == "#0ea5e9" || ev.CategoryImageURL != "" && strings.Contains(ev.CategoryImageURL, "CINEMA") {
			t.Errorf("snapshot de catégorie supprimée à vider : %+v", ev)
		}
		if ev.CategoryBreakdown != nil {
			s := 0
			for k, v := range ev.CategoryBreakdown {
				if k == "CINEMA" {
					t.Errorf("clé CINEMA retirée du BREAKDOWN : %+v", ev.CategoryBreakdown)
				}
				s += v
			}
			if s != ev.Points {
				t.Errorf("la somme du BREAKDOWN (%d) doit rester égale à POINTS (%d)", s, ev.Points)
			}
		}
	}
}

func TestCategories285_History_Backup_ByteIdentical_AtomicWrite_ValidJSON(t *testing.T) {
	e := tw285New(t)
	e.seedHistory(t)
	before, _ := os.ReadFile(e.histPath())
	w := e.del("CINEMA", `{"mode":"reassign"}`)
	var r struct {
		History struct{ Backup string } `json:"history"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &r)
	bak, err := os.ReadFile(filepath.Join(filepath.Dir(e.histPath()), r.History.Backup))
	if err != nil {
		t.Fatalf("backup %q illisible : %v", r.History.Backup, err)
	}
	if !bytes.Equal(bak, before) {
		t.Error("le .bak doit être la copie octet pour octet de l'historique AVANT fusion")
	}
	after, _ := os.ReadFile(e.histPath())
	var events []map[string]interface{}
	if err := json.Unmarshal(after, &events); err != nil {
		t.Fatalf("history.json invalide après fusion : %v", err)
	}
	if bytes.Equal(after, before) {
		t.Error("history.json doit avoir été réécrit")
	}
	for _, f := range tw285Files(t, filepath.Dir(e.histPath())) {
		if strings.HasSuffix(f, ".tmp") {
			t.Errorf("fichier temporaire laissé : %s", f)
		}
	}
}

func TestCategories285_History_NoConcernedEvent_NoBackup_NoRewrite(t *testing.T) {
	e := tw285New(t)
	e.custom(t, "CINEMA", "Cinéma", "")
	e.question(t, "1", map[string]interface{}{"CATEGORY": "CINEMA"})
	e.srv.engine.AddGameEvent(game.GameEvent{Timestamp: 1, QuestionCategory: "GEOGRAPHY", EventType: "POINTS_AWARDED", Points: 3, WinnerType: "TEAM", TeamName: "A"})
	e.settleHistory(t)
	before, _ := os.ReadFile(e.histPath())
	if w := e.del("CINEMA", `{"mode":"reassign"}`); w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	after, _ := os.ReadFile(e.histPath())
	if !bytes.Equal(before, after) {
		t.Error("aucun événement concerné ⇒ pas de réécriture")
	}
	for _, f := range tw285Files(t, filepath.Dir(e.histPath())) {
		if strings.Contains(f, ".bak") {
			t.Errorf("aucun événement concerné ⇒ pas de .bak (vu %s)", f)
		}
	}
}

func TestCategories285_History_BackupRotation_KeepsFiveMostRecent(t *testing.T) {
	e := tw285New(t)
	e.seedHistory(t)
	dir := filepath.Dir(e.histPath())
	old := []string{
		"history.json.bak-20200101-000001", "history.json.bak-20200101-000002", "history.json.bak-20200101-000003",
		"history.json.bak-20200101-000004", "history.json.bak-20200101-000005", "history.json.bak-20200101-000006",
	}
	for _, n := range old {
		_ = os.WriteFile(filepath.Join(dir, n), []byte("[]"), 0o644)
	}
	if w := e.del("CINEMA", `{"mode":"reassign"}`); w.Code != http.StatusOK {
		t.Fatalf("attendu 200, got %d : %s", w.Code, w.Body.String())
	}
	var baks []string
	for _, f := range tw285Files(t, dir) {
		if strings.HasPrefix(f, "history.json.bak-") {
			baks = append(baks, f)
		}
	}
	if len(baks) != 5 {
		t.Fatalf("5 sauvegardes conservées, got %d : %v", len(baks), baks)
	}
	for _, gone := range old[:2] {
		for _, b := range baks {
			if b == gone {
				t.Errorf("la plus ancienne %s doit être supprimée", gone)
			}
		}
	}
}

func TestCategories285_History_BackupNotInTarBackup(t *testing.T) {
	e := tw285New(t)
	e.seedHistory(t)
	e.del("CINEMA", `{"mode":"reassign"}`)
	w := e.do("GET", "/game-backup", nil, "")
	if w.Code != http.StatusOK {
		t.Skipf("GET /game-backup : %d (route non exploitable ici)", w.Code)
	}
	if strings.Contains(w.Body.String(), "history.json.bak") {
		t.Error("les .bak ne doivent jamais entrer dans le .tar de sauvegarde (seul history.json y est ajouté)")
	}
}
