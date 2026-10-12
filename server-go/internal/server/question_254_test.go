package server

// #254 (v11.2, lot V2-4) — côté questions : champ QCM_WRONG_PENALTY (pénalité de mauvaise réponse QCM) et défaut
// POINTS_TARGET au chargement. Contrats : contracts/http-endpoints.md (qcm_wrong_penalty), contracts/models.md §Question,
// contracts/question-types.md, contracts/websocket-actions.md §« Niveau d'attribution ».
// Scénarios 8, 17 et 18 du plan (_work/reports/planner-20261011-113000.md §#254). Boîte noire par le vrai handler HTTP.

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func postQuestion254(t *testing.T, s *HTTPServer, fields map[string]string) (code int) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/questions", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	return w.Code
}

// savedQuestion254 retourne le question.json écrit pour l'ID donné.
func savedQuestion254(t *testing.T, dataDir, id string) map[string]interface{} {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dataDir, "files", "questions", id, "question.json"))
	if err != nil {
		t.Fatalf("question.json de %s : %v", id, err)
	}
	var q map[string]interface{}
	if err := json.Unmarshal(b, &q); err != nil {
		t.Fatal(err)
	}
	return q
}

func qcmFields254(id string, extra map[string]string) map[string]string {
	f := map[string]string{
		"number": id, "type": "QCM", "question": "Capitale ?", "answer": "A", "points": "3", "time": "30", "category": "GEOGRAPHY",
		"qcm_correct": "A",
	}
	for k, v := range extra {
		f[k] = v
	}
	return f
}

// --- 17. Sauvegarde de qcm_wrong_penalty ----------------------------------------------------------------

func TestQuestion254_Critical_WrongPenaltyIsSavedOnlyWhenAValidIntegerBetweenMinus100And0(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  interface{} // nil = champ non écrit
	}{
		{"-2", "-2", float64(-2)},
		{"0", "0", float64(0)},
		{"borne -100", "-100", float64(-100)},
		{"-150 hors bornes", "-150", nil},
		{"3 positif", "3", nil},
		{"texte", "abc", nil},
		{"décimal", "-1.5", nil},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, dataDir := setupTestHTTPServer(t)
			id := string(rune('1' + i))
			if code := postQuestion254(t, s, qcmFields254(id, map[string]string{"qcm_wrong_penalty": c.value})); code != http.StatusOK {
				t.Fatalf("la sauvegarde réussit toujours (valeur invalide ignorée sans erreur) : code %d", code)
			}
			got, present := savedQuestion254(t, dataDir, id)["QCM_WRONG_PENALTY"]
			if c.want == nil {
				if present {
					t.Errorf("QCM_WRONG_PENALTY ne doit pas être écrit pour %q (obtenu %v)", c.value, got)
				}
				return
			}
			if !present || got != c.want {
				t.Errorf("QCM_WRONG_PENALTY = %v (présent=%v), attendu %v", got, present, c.want)
			}
		})
	}
}

func TestQuestion254_WrongPenaltyIsIgnoredForEveryTypeExceptQcm(t *testing.T) {
	for _, typ := range []string{"SPEEDY", "ARDOISE", "MEMORY", "RAFALE"} {
		t.Run(typ, func(t *testing.T) {
			s, dataDir := setupTestHTTPServer(t)
			fields := map[string]string{"number": "7", "type": typ, "question": "Q", "answer": "A", "points": "3", "time": "30", "category": "GEOGRAPHY", "qcm_wrong_penalty": "-2"}
			if code := postQuestion254(t, s, fields); code != http.StatusOK {
				t.Fatalf("code %d", code)
			}
			if _, present := savedQuestion254(t, dataDir, "7")["QCM_WRONG_PENALTY"]; present {
				t.Errorf("%s : QCM_WRONG_PENALTY non écrit (réservé au type QCM)", typ)
			}
		})
	}
}

func TestQuestion254_ResavingWithAnotherValueReplacesThePenalty(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	postQuestion254(t, s, qcmFields254("5", map[string]string{"qcm_wrong_penalty": "-4"}))
	if v := savedQuestion254(t, dataDir, "5")["QCM_WRONG_PENALTY"]; v != float64(-4) {
		t.Fatalf("précondition : -4, obtenu %v", v)
	}
	postQuestion254(t, s, qcmFields254("5", map[string]string{"qcm_wrong_penalty": "-1"}))
	if v := savedQuestion254(t, dataDir, "5")["QCM_WRONG_PENALTY"]; v != float64(-1) {
		t.Errorf("la nouvelle valeur remplace l'ancienne : %v", v)
	}
}

// --- 18. Questions existantes sans le champ ---------------------------------------------------------------

func listQuestions254(t *testing.T, s *HTTPServer) map[string]map[string]interface{} {
	t.Helper()
	req := httptest.NewRequest("GET", "/questions", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /questions : %d", w.Code)
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &all); err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]interface{}{}
	for _, raw := range all {
		var q map[string]interface{}
		if json.Unmarshal(raw, &q) == nil {
			if id, ok := q["ID"].(string); ok {
				out[id] = q
			}
		}
	}
	return out
}

func writeQuestionFile254(t *testing.T, dataDir, id string, q map[string]interface{}) string {
	t.Helper()
	dir := filepath.Join(dataDir, "files", "questions", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(q)
	p := filepath.Join(dir, "question.json")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestQuestion254_ExistingQcmWithoutTheFieldLoadsWithoutErrorAndIsNotRewritten(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	path := writeQuestionFile254(t, dataDir, "21", map[string]interface{}{"ID": "21", "TYPE": "QCM", "QUESTION": "Q", "ANSWER": "A", "POINTS": "3", "TIME": "30"})
	before, _ := os.ReadFile(path)
	info, _ := os.Stat(path)

	q, ok := listQuestions254(t, s)["21"]
	if !ok {
		t.Fatal("la question existante est servie")
	}
	if _, present := q["QCM_WRONG_PENALTY"]; present {
		t.Errorf("servie sans le champ (le client lit 0 par défaut) : %v", q["QCM_WRONG_PENALTY"])
	}
	after, _ := os.ReadFile(path)
	info2, _ := os.Stat(path)
	if !bytes.Equal(before, after) || !info.ModTime().Equal(info2.ModTime()) {
		t.Error("aucune réécriture du fichier au chargement (les fichiers existants ne sont pas migrés)")
	}
}

func TestQuestion254_ExistingPenaltyIsServedAsStored(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	writeQuestionFile254(t, dataDir, "22", map[string]interface{}{"ID": "22", "TYPE": "QCM", "QUESTION": "Q", "ANSWER": "A", "POINTS": "3", "TIME": "30", "QCM_WRONG_PENALTY": -3})
	if v := listQuestions254(t, s)["22"]["QCM_WRONG_PENALTY"]; v != float64(-3) {
		t.Errorf("QCM_WRONG_PENALTY servi tel quel : %v", v)
	}
}

// --- 8. Défaut POINTS_TARGET au chargement ----------------------------------------------------------------

func TestQuestion254_Critical_PointsTargetDefaultAtLoadFollowsTheAwardLevelTable(t *testing.T) {
	s, dataDir := setupTestHTTPServer(t)
	defaults := map[string]string{"SPEEDY": "PLAYER", "QCM": "TEAM", "ARDOISE": "TEAM", "MEMORY": "TEAM", "MEMOTION": "TEAM", "RAFALE": "TEAM"}
	ids := map[string]string{}
	n := 30
	for typ := range defaults {
		n++
		id := string(rune('0'+n/10)) + string(rune('0'+n%10))
		ids[typ] = id
		writeQuestionFile254(t, dataDir, id, map[string]interface{}{"ID": id, "TYPE": typ, "QUESTION": "Q", "ANSWER": "A", "POINTS": "3", "TIME": "30"})
	}
	// valeur présente → conservée (y compris à contre-courant du défaut)
	writeQuestionFile254(t, dataDir, "50", map[string]interface{}{"ID": "50", "TYPE": "QCM", "QUESTION": "Q", "ANSWER": "A", "POINTS": "3", "TIME": "30", "POINTS_TARGET": "PLAYER"})
	writeQuestionFile254(t, dataDir, "51", map[string]interface{}{"ID": "51", "TYPE": "SPEEDY", "QUESTION": "Q", "ANSWER": "A", "POINTS": "3", "TIME": "30", "POINTS_TARGET": "TEAM"})
	writeQuestionFile254(t, dataDir, "52", map[string]interface{}{"ID": "52", "TYPE": "ARDOISE", "QUESTION": "Q", "ANSWER": "A", "POINTS": "3", "TIME": "30", "POINTS_TARGET": "PLAYER"})
	// ancien nom de SPEEDY
	writeQuestionFile254(t, dataDir, "53", map[string]interface{}{"ID": "53", "TYPE": "NORMAL", "QUESTION": "Q", "ANSWER": "A", "POINTS": "3", "TIME": "30"})

	all := listQuestions254(t, s)
	for typ, want := range defaults {
		if got := all[ids[typ]]["POINTS_TARGET"]; got != want {
			t.Errorf("%s sans POINTS_TARGET : %v, attendu %s", typ, got, want)
		}
	}
	for id, want := range map[string]string{"50": "PLAYER", "51": "TEAM", "52": "PLAYER", "53": "PLAYER"} {
		if got := all[id]["POINTS_TARGET"]; got != want {
			t.Errorf("question %s : POINTS_TARGET = %v, attendu %s (valeur enregistrée respectée / NORMAL = SPEEDY)", id, got, want)
		}
	}
}
