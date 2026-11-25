package server

import (
	"archive/tar"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LOT-2 (v13.1.0, issue fille #324) — T16 : NON-RÉGRESSION RAFALE SPEEDY côté HTTP, IA et sauvegarde.
// Contrat contracts/picto-pool.md §1, §4, §5, §7, §9 : `/api/rafale/*` « chemins inchangés », prompt IA
// « identique », clé de sauvegarde `rafale` inchangée. Ces tests passent AVANT la refactorisation (fabrique de
// handlers, descripteur IA, liste des pools) et DOIVENT passer APRÈS, sans modification.

func nrDo(t *testing.T, s *HTTPServer, method, url, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, url, r)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	return w
}

func TestPoolRafaleAPI_NR_WireShapes_ByteForByte(t *testing.T) {
	s, _ := setupTestHTTPServer(t)

	w := nrDo(t, s, "POST", "/api/rafale/questions", `{"QUESTION":"Capitale du Pérou ?","ANSWER":"Lima","CATEGORY":"GEOGRAPHY","DIFFICULTY":2}`)
	if w.Code != 200 || w.Body.String() != "{\"ID\":\"r-001\"}\n" {
		t.Fatalf("POST create : %d %q", w.Code, w.Body.String())
	}

	w = nrDo(t, s, "GET", "/api/rafale/questions", "")
	want := `{"QUESTIONS":[{"ID":"r-001","QUESTION":"Capitale du Pérou ?","ANSWER":"Lima","CATEGORY":"GEOGRAPHY","DIFFICULTY":2,"USED":false}],"TOTAL":1}` + "\n"
	if w.Code != 200 || w.Body.String() != want {
		t.Fatalf("GET list : %d\n got  %q\n want %q", w.Code, w.Body.String(), want)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}

	if _, err := s.engine.DrawRafaleQuestion([]string{"GEOGRAPHY"}, []int{2}); err != nil {
		t.Fatal(err)
	}
	w = nrDo(t, s, "POST", "/api/rafale/questions/r-001/reset", "")
	if w.Code != 200 || w.Body.String() != "{\"AVAILABLE\":true,\"ID\":\"r-001\"}\n" {
		t.Errorf("POST reset : %d %q", w.Code, w.Body.String())
	}

	if _, err := s.engine.DrawRafaleQuestion([]string{"GEOGRAPHY"}, []int{2}); err != nil {
		t.Fatal(err)
	}
	w = nrDo(t, s, "POST", "/api/rafale/questions/reset-all", "")
	if w.Code != 200 || w.Body.String() != "{\"RESET\":1}\n" {
		t.Errorf("POST reset-all : %d %q", w.Code, w.Body.String())
	}

	w = nrDo(t, s, "GET", "/api/rafale/pool?categories=GEOGRAPHY&difficulties=2", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"AVAILABLE":1`) || !strings.Contains(w.Body.String(), `"TOTAL":1`) || !strings.Contains(w.Body.String(), `"USED":0`) {
		t.Errorf("GET pool : %d %q", w.Code, w.Body.String())
	}

	w = nrDo(t, s, "DELETE", "/api/rafale/questions/r-001", "")
	if w.Code != 200 || w.Body.String() != "{\"DELETED\":\"r-001\"}\n" {
		t.Errorf("DELETE : %d %q", w.Code, w.Body.String())
	}
}

func TestPoolRafaleAPI_NR_ErrorCodes_Unchanged(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	cases := []struct {
		name, method, url, body string
		code                    int
	}{
		{"JSON invalide", "POST", "/api/rafale/questions", `{`, 400},
		{"QUESTION vide", "POST", "/api/rafale/questions", `{"QUESTION":" ","ANSWER":"a","CATEGORY":"HISTORY","DIFFICULTY":1}`, 400},
		{"ANSWER vide (obligatoire pour RAFALE)", "POST", "/api/rafale/questions", `{"QUESTION":"q","ANSWER":"","CATEGORY":"HISTORY","DIFFICULTY":1}`, 400},
		{"QUESTION > 100", "POST", "/api/rafale/questions", `{"QUESTION":"` + strings.Repeat("x", 101) + `","ANSWER":"a","CATEGORY":"HISTORY","DIFFICULTY":1}`, 400},
		{"ANSWER > 40", "POST", "/api/rafale/questions", `{"QUESTION":"q","ANSWER":"` + strings.Repeat("x", 41) + `","CATEGORY":"HISTORY","DIFFICULTY":1}`, 400},
		{"DIFFICULTY 0", "POST", "/api/rafale/questions", `{"QUESTION":"q","ANSWER":"a","CATEGORY":"HISTORY","DIFFICULTY":0}`, 400},
		{"DIFFICULTY 4", "POST", "/api/rafale/questions", `{"QUESTION":"q","ANSWER":"a","CATEGORY":"HISTORY","DIFFICULTY":4}`, 400},
		{"CATEGORY inconnue", "POST", "/api/rafale/questions", `{"QUESTION":"q","ANSWER":"a","CATEGORY":"NOPE","DIFFICULTY":1}`, 400},
		{"difficulty de filtre invalide", "GET", "/api/rafale/questions?difficulty=x", "", 400},
		{"DELETE inconnu", "DELETE", "/api/rafale/questions/r-999", "", 404},
		{"reset inconnu", "POST", "/api/rafale/questions/r-999/reset", "", 404},
		{"méthode non permise (questions)", "PUT", "/api/rafale/questions", "", 405},
		{"méthode non permise (reset-all)", "GET", "/api/rafale/questions/reset-all", "", 405},
		{"pool sans paramètres", "GET", "/api/rafale/pool", "", 400},
	}
	for _, c := range cases {
		if w := nrDo(t, s, c.method, c.url, c.body); w.Code != c.code {
			t.Errorf("%s : %s %s → %d, attendu %d (%s)", c.name, c.method, c.url, w.Code, c.code, strings.TrimSpace(w.Body.String()))
		}
	}
}

// Prompt IA RAFALE : texte exact (contrat picto-pool.md §5 « le prompt, le schéma et les réponses du chemin RAFALE
// sont inchangés »). Toute modification, même d'un caractère, est une régression à arbitrer explicitement.
func TestPoolRafaleAPI_NR_AIPrompt_Golden(t *testing.T) {
	s, _ := setupTestHTTPServer(t)
	req := generateRafaleRequest{Theme: "Géographie", Populations: []string{"adultes", "ados"}, Language: "français", Instructions: "Pas de capitales"}
	got := s.buildRafaleGenerationPrompt(req, 10, "GEOGRAPHY", 2, []string{"Q existante ?"})
	want := "Tu es un générateur de questions pour une manche RAFALE — un mode de jeu buzzer très rapide où chaque question dispose d'environ 3 secondes de temps de réponse. " +
		"Génère des questions strictement conformes au schéma JSON fourni — n'ajoute aucun champ hors schéma.\n\n" +
		"Thème : Géographie\n" +
		"Publics cibles : adultes, ados\n" +
		"Langue de rédaction : français\n" +
		"Couple visé pour cet appel : catégorie GEOGRAPHY, difficulté " + rafaleStarsLabel(2) + "\n" +
		"Précisions pour cette génération : Pas de capitales\n" +
		"Nombre de questions à générer dans cet appel : 10\n" +
		"\n🔴 Contrainte de brièveté impérative — une manche RAFALE ne laisse qu'environ 3 secondes par question :\n" +
		"- QUESTION : au plus 80 caractères, une seule phrase interrogative courte.\n" +
		"- ANSWER : au plus 25 caractères, idéalement 1 à 3 mots.\n" +
		"- Exemple correct : {\"QUESTION\": \"Quelle planète est la plus proche du Soleil ?\", \"ANSWER\": \"Mercure\"}\n" +
		"- Contre-exemple à éviter (bien trop long pour 3 secondes de réponse) : {\"QUESTION\": \"Dans le système solaire, quelle est la planète la plus proche de notre étoile en termes de distance orbitale moyenne ?\", \"ANSWER\": \"Il s'agit de la planète Mercure\"}\n" +
		"\nQuestions déjà présentes dans le réservoir pour ces catégories/difficultés, y compris celles déjà produites plus tôt dans cette génération — NE LES DUPLIQUE PAS :\n" +
		"[\"Q existante ?\"]\n"
	if got != want {
		t.Errorf("prompt RAFALE modifié.\n--- obtenu ---\n%s\n--- attendu ---\n%s", got, want)
	}
}

func TestPoolRafaleAPI_NR_AIPresets_And_Plafonds_Unchanged(t *testing.T) {
	for _, n := range []int{10, 20, 50, 100, 200} {
		if !isRafaleGenerationPreset(n) {
			t.Errorf("palier RAFALE %d disparu", n)
		}
	}
	for _, n := range []int{0, 5, 30, 500} {
		if isRafaleGenerationPreset(n) {
			t.Errorf("%d ne doit pas être un palier RAFALE (paliers 10/20/50 = Picto seulement)", n)
		}
	}
	if rafaleMaxQuestionRunes != 100 || rafaleMaxAnswerRunes != 40 {
		t.Errorf("plafonds RAFALE = %d/%d, attendu 100/40", rafaleMaxQuestionRunes, rafaleMaxAnswerRunes)
	}
}

// Sauvegarde sélective : clé `rafale` inchangée, mêmes chemins d'archive (contrat §7, §9).
func TestPoolRafaleAPI_NR_Backup_RafaleKey_ArchivePaths(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	res := "{\n  \"QUESTIONS\": []\n}"
	used := "{\n  \"USED\": {\n    \"r-001\": true\n  }\n}"
	for path, content := range map[string]string{
		filepath.Join(dataDir, "files", "rafale", "reservoir.json"): res,
		filepath.Join(dataDir, "config", "rafale_used.json"):        used,
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w := nrDo(t, s, "GET", "/backup-select?rafale=true", "")
	if w.Code != http.StatusOK {
		t.Fatalf("backup-select : %d", w.Code)
	}
	got := map[string]string{}
	tr := tar.NewReader(w.Body)
	for {
		h, err := tr.Next()
		if err != nil {
			break
		}
		b, _ := io.ReadAll(tr)
		got[strings.TrimSuffix(h.Name, "/")] = string(b)
	}
	if got["files/rafale/reservoir.json"] != res {
		t.Errorf("files/rafale/reservoir.json absent ou modifié dans l'archive (entrées : %d)", len(got))
	}
	if got["config/rafale_used.json"] != used {
		t.Errorf("config/rafale_used.json absent ou modifié dans l'archive")
	}
	// la clé `rafale` seule n'embarque pas les autres catégories (clé picto à venir : clé distincte)
	for name := range got {
		if strings.HasPrefix(name, "files/picto") || strings.HasPrefix(name, "config/picto_used") {
			t.Errorf("rafale=true ne doit pas embarquer le Pool Picto : %s", name)
		}
	}
}
