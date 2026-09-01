package server

// ---------------------------------------------------------------------------
// Régression — retour QUALIF 8.0.0.14 (#199) : "je peux encore lancer un
// START alors que je n'ai pas défini d'équipe pour la manche RAFALE",
// malgré la garde participantsConform (SHA 393c6dc7, reviewée, QA validée).
//
// Investigation exhaustive (dev-backend) de TOUS les chemins backend
// pouvant amener Phase=STARTED sans passer par la garde :
//   - Engine.Start() (handleStart, ADMIN et ANIM) : refuse hors READY. Testé.
//   - Engine.ForceReady() (debug, ADMIN uniquement) : vérifie
//     participantsConform en interne. Testé.
//   - Engine.TransitionToReady() (le VRAI chemin PONG, seul appelant :
//     handlePong, main.go:1608) : gardé par le SITE D'APPEL
//     (`if a.engine.AreAllTeamsReady() && a.engine.ParticipantsConform()`),
//     PONG n'est de toute façon jamais autorisé depuis ANIM (allow-list).
//   - Engine.StartImmediate() : jamais appelé depuis un chemin WS réel
//     (grep négatif sur cmd/server/main.go et internal/server/*.go hors
//     tests) — accessible uniquement aux tests.
// Les TROIS sites qui font `Phase = PhaseReady` dans engine.go sont donc
// TOUS gardés, directement ou par leur appelant. Aucun contournement de
// code trouvé.
//
// Hypothèse retenue (cohérente avec le PATRON DÉJÀ VU deux fois cette
// semaine — CATEGORY vide, cycle 1 ; RAFALE_DIFFICULTY jamais persisté,
// cycle 2) : RAFALE_MODE lui-même a TOUJOURS eu un repli "vide ⇒ SOLO"
// documenté comme "harmless" au moment du fix DIFFICULTY (2026-08-31,
// http.go) — vrai À CETTE DATE (#199 n'existait pas encore), FAUX depuis
// que #199 fait dépendre l'exigence d'équipe de RAFALE_MODE. Une question
// de configuration de manche sauvegardée AVANT le fix http.go (8.0.0.11,
// avant lequel handleUploadQuestion n'avait AUCUN bloc RAFALE — RAFALE_MODE
// inclus) et jamais RE-sauvegardée depuis aurait RAFALE_MODE="" encore
// aujourd'hui, donc traitée comme SOLO par la garde — un comportement
// CORRECT du code vis-à-vis d'une donnée PÉRIMÉE, pas un bug de la garde
// elle-même. Pas une preuve directe (accès aux données QUALIF de
// l'utilisateur impossible depuis ici), mais l'explication la plus
// cohérente avec l'historique de cette fonctionnalité.
//
// Ce fichier prouve, à travers le VRAI chemin HTTP d'upload (pas une
// construction directe de game.Question en mémoire, contrairement à
// internal/game/rafale_modes_test.go) qu'une question RAFALE
// correctement configurée en mode multi AUJOURD'HUI (sauvegardée via le
// endpoint réel, sous le code actuel) exige bien une équipe — la garde
// fonctionne de bout en bout sur le chemin que l'utilisateur emprunte
// réellement en pratique (éditeur -> POST /questions -> READY -> START).
// ---------------------------------------------------------------------------

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"buzzcontrol/internal/game"
)

// uploadRafaleQuestionViaHTTP POSTs a RAFALE round-config question through
// the REAL multipart handler (handleUploadQuestion) and returns it
// unmarshaled into game.Question — the exact same path
// TestHTTPServer_RafaleQuestionUpload already covers for persistence, reused
// here to feed the engine's real Ready()/ParticipantsConform() gate. Reads
// the created question.json directly from disk (dataDir) rather than
// through main.go's App.loadQuestion — that helper lives in package main,
// not reachable from this package — same net effect (a fresh unmarshal of
// the persisted file, not an in-memory copy).
func uploadRafaleQuestionViaHTTP(t *testing.T, server *HTTPServer, dataDir string, mode string) *game.Question {
	t.Helper()
	body := strings.NewReader("--boundary\r\n" +
		"Content-Disposition: form-data; name=\"question\"\r\n\r\n" +
		"Manche Histoire\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"type\"\r\n\r\n" +
		"RAFALE\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"category\"\r\n\r\n" +
		"HISTORY\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"points\"\r\n\r\n" +
		"10\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"RAFALE_DIFFICULTY\"\r\n\r\n" +
		"1\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"RAFALE_MODE\"\r\n\r\n" +
		mode + "\r\n" +
		"--boundary\r\n" +
		"Content-Disposition: form-data; name=\"RAFALE_QUESTION_TIME\"\r\n\r\n" +
		"3\r\n" +
		"--boundary--\r\n")

	req := httptest.NewRequest(http.MethodPost, "/questions", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("upload failed: %d: %s", w.Code, w.Body.String())
	}

	// handleUploadQuestion's response body is just {"status":"ok"} — no ID
	// (see TestHTTPServer_MemoryQuestionUpload's own pattern) — the created
	// question's directory is found by listing questionsDir instead.
	questionsDir := filepath.Join(dataDir, "files", "questions")
	entries, err := os.ReadDir(questionsDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected a question directory to be created under %s: %v", questionsDir, err)
	}
	id := entries[len(entries)-1].Name() // most recently created

	questionFile := filepath.Join(questionsDir, id, "question.json")
	data, err := os.ReadFile(questionFile)
	if err != nil {
		t.Fatalf("failed to read back question.json for %s: %v", id, err)
	}
	var loaded game.Question
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("failed to parse question.json for %s: %v (raw: %s)", id, err, data)
	}
	return &loaded
}

// TestRafaleMultiTeamGate_RealHTTPUpload_ChacunSonTour_RequiresTeam is the
// exact repro requested by the CDP for the 8.0.0.14 QUALIF report: a RAFALE
// round-config question uploaded through the real editor endpoint, TODAY's
// code, with a genuinely persisted multi mode, must still require a team
// before READY — through the real Ready()/ForceReady() gate, not a direct
// in-memory Question construction.
func TestRafaleMultiTeamGate_RealHTTPUpload_ChacunSonTour_RequiresTeam(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	os.MkdirAll(filepath.Join(dataDir, "files", "questions"), 0755)
	server.engine.SetTeams(map[string]*game.Team{
		"red": {Name: "Team Red"}, "blue": {Name: "Team Blue"},
	})

	q := uploadRafaleQuestionViaHTTP(t, server, dataDir, "CHACUN_SON_TOUR")
	if q.RafaleMode != "CHACUN_SON_TOUR" {
		t.Fatalf("sanity: expected the uploaded question to round-trip RAFALE_MODE=CHACUN_SON_TOUR, got %q — persistence bug, not a gate bug", q.RafaleMode)
	}

	server.engine.Ready(q.ID, q)
	if server.engine.ParticipantsConform() {
		t.Fatal("BUG: a real HTTP-uploaded CHACUN_SON_TOUR question conforms with NO team selected")
	}

	server.engine.ForceReady()
	if state := server.engine.GetState(); state.Phase == game.PhaseReady {
		t.Fatal("BUG: reached READY via ForceReady on a real HTTP-uploaded CHACUN_SON_TOUR question with no team selected")
	}

	server.engine.Start(30)
	if state := server.engine.GetState(); state.Phase != game.PhasePrepare {
		t.Errorf("expected Start() to be refused (stuck in PREPARE), got phase=%s", state.Phase)
	}
}

// TestRafaleMultiTeamGate_RealHTTPUpload_EmptyMode_DefaultsToSOLO documents,
// as a DELIBERATE control case (not a bug), the exact mechanism behind the
// "stale pre-8.0.0.11 question" hypothesis above: a round-config question
// with NO RAFALE_MODE at all (as any RAFALE question saved before http.go's
// fix would still have, if never re-saved since) is treated EXACTLY like an
// explicit SOLO, by the SAME "empty ⇒ SOLO" convention used everywhere else
// in the engine (RafaleQuestionTime<=0⇒3, RafaleMaxQuestions<=0⇒100). This
// is why the backend cannot structurally distinguish "genuinely SOLO" from
// "stale/never-saved MODE" — see this file's header comment.
//
// #201 (2026-09-02): SOLO itself no longer means "no team required" — it
// now requires EXACTLY one, same as MEMORY SOLO (participantsCountConform,
// engine.go). Renamed from its original *_NoTeamRequired name and updated:
// still proves the empty->SOLO default, but the assertion now checks the
// current SOLO rule (no team => refused, one team => conforms) rather than
// the pre-#201 "no team required at all" behavior.
func TestRafaleMultiTeamGate_RealHTTPUpload_EmptyMode_DefaultsToSOLO(t *testing.T) {
	server, dataDir := setupTestHTTPServer(t)
	os.MkdirAll(filepath.Join(dataDir, "files", "questions"), 0755)
	server.engine.SetTeams(map[string]*game.Team{"red": {Name: "Team Red"}})

	q := uploadRafaleQuestionViaHTTP(t, server, dataDir, "") // RAFALE_MODE FormValue empty -> never written
	if q.RafaleMode != "" {
		t.Fatalf("sanity: expected RAFALE_MODE to stay empty when not sent, got %q", q.RafaleMode)
	}

	server.engine.Ready(q.ID, q)
	if server.engine.ParticipantsConform() {
		t.Error("BUG (#201): an empty RAFALE_MODE (defaults to SOLO) conforms with NO team selected — SOLO now requires exactly one")
	}

	if err := server.engine.SetRafaleParticipatingTeams([]string{"red"}); err != nil {
		t.Fatalf("SetRafaleParticipatingTeams: %v", err)
	}
	if !server.engine.ParticipantsConform() {
		t.Error("expected an empty RAFALE_MODE (defaults to SOLO) to conform with exactly one team selected — this is by-design, not a bug")
	}
}
