package main

// #245 (v11.2) — durcissement de revue (code-reviewer-20261013-120000.md) : M1 (l'identité d'une question est le NOM de son dossier,
// jamais le contenu de question.json : aucun chemin construit depuis un ID forgé), M2 (application de ROUNDS_UPDATE sans effet partiel :
// rounds.json écrit d'abord, dossiers supprimés en dernier, jamais APPLIED ni SetRounds en cas d'échec, DELETED_QUESTIONS exact),
// mineur 1 (rounds.json illisible), mineur 4 (manche contenant la question en cours, statuts purgés), mineur 5 (fixture de parité).
// ROUGES avant les corrections de dev-backend. `critical` : M1, M2.
//
// SÉCURITÉ DU TEST : un ID forgé « ../.. » ferait sortir une suppression du dossier des questions. Le dossier des questions est donc
// placé 3 niveaux sous un répertoire temporaire dédié : au pire, le test détruit CE répertoire, jamais le dossier temporaire système.

import (
	"buzzcontrol/internal/game"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// newEnv245Sandboxed : dossier des questions = <racine>/a/b/questions ; « ../.. » désigne <racine>/a.
func newEnv245Sandboxed(t *testing.T) (e *env245, root string) {
	t.Helper()
	b := buildEnv253(t, false, true)
	root = t.TempDir()
	dir := filepath.Join(root, "a", "b", "questions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b.app.config.Storage.QuestionsDir = dir
	return &env245{env253: b, dir: dir}, root
}

// treeSnapshot245 : contenu de tous les fichiers ET liste des dossiers sous `root` (chemin → contenu ; dossier → "<dir>").
func treeSnapshot245(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if info.IsDir() {
			out[rel] = "<dir>"
		} else {
			b, _ := os.ReadFile(p)
			out[rel] = string(b)
		}
		return nil
	})
	return out
}

func sameTree245(a, b map[string]string) (bool, string) {
	for k, v := range a {
		if bv, ok := b[k]; !ok {
			return false, "disparu : " + k
		} else if bv != v {
			return false, "modifié : " + k
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			return false, "apparu : " + k
		}
	}
	return true, ""
}

func (e *env245) roundsAfter(t *testing.T) []map[string]interface{} {
	t.Helper()
	var g struct {
		Game struct {
			Rounds []map[string]interface{} `json:"ROUNDS"`
		} `json:"GAME"`
	}
	_ = jsonUnmarshal245(e.app.engine.GetGameJSON(), &g)
	return g.Game.Rounds
}

// =============================== M1 — identité des dossiers ===============================

func TestRoundsHardening245_Critical_M1_FolderNameIsTheIdentityNeverTheIDInsideTheFile(t *testing.T) {
	e, root := newEnv245Sandboxed(t)
	e.writeQ(t, "9", 0, "r1", "")                                                                                                                               // vraie question 9
	e.writeRaw(t, "5", map[string]interface{}{"ID": "9", "TYPE": "SPEEDY", "QUESTION": "Imposteur", "POINTS": "1", "TIME": "30", "ORDER": 1, "ROUND_ID": "r1"}) // dossier 5 qui se dit « 9 »
	e.writeQ(t, "7", 0, "", "")
	if err := os.WriteFile(filepath.Join(e.dir, "rounds.json"), []byte(`{"ROUNDS":[{"ID":"r1","NAME":"Un"}],"NEXT_ID":2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	before := treeSnapshot245(t, root)

	// suppression de la manche r1 avec DELETE : viserait « 9 » (deux fois) et jamais le dossier 5 réel
	e.update(t, e.admin1, nil, []string{"7"}, []deletedRound245{{ID: "r1", Questions: "DELETE"}})

	r := e.result(t)
	if r.Status != "REFUSED" {
		t.Errorf("un dossier dont l'ID ≠ son nom fait refuser le ROUNDS_UPDATE entier : %+v", r)
	}
	if ok, why := sameTree245(before, treeSnapshot245(t, root)); !ok {
		t.Errorf("aucune suppression ni écriture sur une autre question : %s", why)
	}
}

func TestRoundsHardening245_Critical_M1_ForgedPathIDNeverEscapesTheQuestionsFolder(t *testing.T) {
	e, root := newEnv245Sandboxed(t)
	outside := filepath.Join(root, "a", "precieux.txt") // = <questions>/../.. : ne doit jamais être atteint
	if err := os.WriteFile(outside, []byte("ne pas toucher"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.writeRaw(t, "3", map[string]interface{}{"ID": "../..", "TYPE": "SPEEDY", "QUESTION": "Forgée", "POINTS": "1", "TIME": "30", "ORDER": 0, "ROUND_ID": "r1"})
	e.writeQ(t, "4", 0, "", "")
	if err := os.WriteFile(filepath.Join(e.dir, "rounds.json"), []byte(`{"ROUNDS":[{"ID":"r1","NAME":"Un"}],"NEXT_ID":2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	before := treeSnapshot245(t, root)

	e.update(t, e.admin1, nil, []string{"4"}, []deletedRound245{{ID: "r1", Questions: "DELETE"}})

	if _, ok := waitFrame253(e.a1, "ROUNDS_UPDATE_RESULT", nil); !ok {
		t.Error("une réponse est envoyée à l'émetteur")
	}
	if ok, why := sameTree245(before, treeSnapshot245(t, root)); !ok {
		t.Errorf("rien ne doit sortir du dossier des questions ni être supprimé (%s)", why)
	}
	if b, err := os.ReadFile(outside); err != nil || string(b) != "ne pas toucher" {
		t.Errorf("le fichier hors du dossier des questions est intact : %v %q", err, b)
	}
}

func TestRoundsHardening245_M1_TwoFoldersDeclaringTheSameIDRefuseTheWholeUpdate(t *testing.T) {
	e, root := newEnv245Sandboxed(t)
	e.writeQ(t, "3", 0, "", "")
	e.writeRaw(t, "4", map[string]interface{}{"ID": "3", "TYPE": "SPEEDY", "QUESTION": "Doublon", "POINTS": "1", "TIME": "30", "ORDER": 1})
	before := treeSnapshot245(t, root)

	e.update(t, e.admin1, []roundSpec245{{Name: "Un", Questions: []string{"3"}, Entractes: boolp(false)}}, []string{}, nil)

	if r := e.result(t); r.Status != "REFUSED" {
		t.Errorf("deux dossiers au même ID : ROUNDS_UPDATE entier refusé : %+v", r)
	}
	if ok, why := sameTree245(before, treeSnapshot245(t, root)); !ok {
		t.Errorf("rien n'est écrit : %s", why)
	}
}

// =============================== M2 — application sans effet partiel ===============================

// readOnlyDir245 retire le droit d'écriture (impossible de créer/renommer/supprimer dans le dossier) ; ignoré si le système
// de fichiers ne l'applique pas (administrateur, Windows).
func readOnlyDir245(t *testing.T, p string) bool {
	t.Helper()
	if err := os.Chmod(p, 0o555); err != nil {
		return false
	}
	t.Cleanup(func() { _ = os.Chmod(p, 0o755) })
	probe := filepath.Join(p, ".probe")
	if f, err := os.Create(probe); err == nil {
		f.Close()
		_ = os.Remove(probe)
		_ = os.Chmod(p, 0o755)
		return false
	}
	return true
}

func TestRoundsHardening245_Critical_M2_RoundsJsonWriteFailureChangesNothingAndIsNeverApplied(t *testing.T) {
	e, root := newEnv245Sandboxed(t)
	e.base(t)
	e.writeQ(t, "10", 0, "r1", "")
	if err := os.WriteFile(filepath.Join(e.dir, "rounds.json"), []byte(`{"ROUNDS":[{"ID":"r1","NAME":"Un"}],"NEXT_ID":2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	roundsBefore := e.roundsAfter(t)
	before := treeSnapshot245(t, root)
	if !readOnlyDir245(t, e.dir) { // rounds.json ne peut plus être remplacé
		t.Skip("le système de fichiers n'applique pas les droits : injection d'échec impossible ici")
	}

	// supprime la manche r1 (DELETE de la question 10) et en crée une autre : l'écriture de rounds.json échoue
	e.update(t, e.admin1, []roundSpec245{{Name: "Neuve", Questions: []string{"1"}, Entractes: boolp(false)}}, []string{"2", "3", "4"}, []deletedRound245{{ID: "r1", Questions: "DELETE"}})

	r := e.result(t)
	if r.Status == "APPLIED" {
		t.Fatalf("jamais APPLIED quand l'écriture échoue : %+v", r)
	}
	if len(r.Deleted) != 0 {
		t.Errorf("DELETED_QUESTIONS = ce qui a réellement été supprimé (rien) : %v", r.Deleted)
	}
	_ = os.Chmod(e.dir, 0o755)
	if ok, why := sameTree245(before, treeSnapshot245(t, root)); !ok {
		t.Errorf("aucun effet : ni suppression, ni écriture (%s)", why)
	}
	after := e.roundsAfter(t)
	if len(after) != len(roundsBefore) {
		t.Errorf("SetRounds n'est pas appelé en cas d'échec : %v → %v", roundsBefore, after)
	}
}

func TestRoundsHardening245_Critical_M2_QuestionFileWriteFailureIsNeverAppliedAndNothingIsDeleted(t *testing.T) {
	e, root := newEnv245Sandboxed(t)
	e.base(t)
	e.writeQ(t, "10", 0, "r1", "")
	if err := os.WriteFile(filepath.Join(e.dir, "rounds.json"), []byte(`{"ROUNDS":[{"ID":"r1","NAME":"Un"}],"NEXT_ID":2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	roundsBefore := e.roundsAfter(t)
	// la question 2 ne peut plus être réécrite (dossier et fichier en lecture seule)
	qd := filepath.Join(e.dir, "2")
	if err := os.Chmod(filepath.Join(qd, "question.json"), 0o444); err != nil {
		t.Skip("chmod indisponible")
	}
	if !readOnlyDir245(t, qd) {
		t.Skip("le système de fichiers n'applique pas les droits : injection d'échec impossible ici")
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(qd, "question.json"), 0o644) })

	// déplace la question 2 dans r1 ET supprime ... rien d'autre : seule la réécriture de 2 échoue
	e.update(t, e.admin1, []roundSpec245{{ID: "r1", Name: "Un", Questions: []string{"10", "2"}}}, []string{"1", "3", "4"}, nil)

	r := e.result(t)
	if r.Status == "APPLIED" {
		t.Errorf("un échec d'écriture de question.json ne répond jamais APPLIED : %+v", r)
	}
	if len(r.Deleted) != 0 {
		t.Errorf("rien n'est supprimé : %v", r.Deleted)
	}
	_ = os.Chmod(qd, 0o755)
	for _, id := range []string{"1", "3", "4", "10"} {
		if !e.exists(id) {
			t.Errorf("la question %s existe toujours", id)
		}
	}
	if after := e.roundsAfter(t); len(after) != len(roundsBefore) {
		t.Errorf("SetRounds non appelé en cas d'échec : %v → %v", roundsBefore, after)
	}
	_ = root
}

func TestRoundsHardening245_Critical_M2_EntracteIDExhaustionHalfwayLeavesNoEmptyFolderAndNoOtherEffect(t *testing.T) {
	e, root := newEnv245Sandboxed(t)
	for i := 1; i <= 998; i++ {
		e.writeQ(t, itoa245(i), i, "", "")
	}
	before := treeSnapshot245(t, root)
	unassigned := make([]string, 0, 997)
	for i := 2; i <= 998; i++ { // la question 1 est déplacée dans la nouvelle manche : un effet « partiel » possible
		unassigned = append(unassigned, itoa245(i))
	}

	e.update(t, e.admin1, []roundSpec245{{Name: "Trop tard", Questions: []string{"1"}}}, unassigned, nil) // 2 entractes, 1 seul ID libre

	r := e.result(t)
	if r.Status != "REFUSED" || r.Reason != "QUESTION_ID_EXHAUSTED" {
		t.Fatalf("RESULT = %+v", r)
	}
	if ok, why := sameTree245(before, treeSnapshot245(t, root)); !ok {
		t.Errorf("aucun dossier vide laissé, aucune question réécrite (ROUND_ID/ORDER de la question 1 inchangés) : %s", why)
	}
}

func TestRoundsHardening245_M2_DeletedQuestionsListsExactlyWhatWasDeleted(t *testing.T) {
	e, _ := newEnv245Sandboxed(t)
	e.base(t)
	e.writeQ(t, "10", 0, "r1", "")
	e.writeQ(t, "11", 1, "r1", "")
	if err := os.WriteFile(filepath.Join(e.dir, "rounds.json"), []byte(`{"ROUNDS":[{"ID":"r1","NAME":"Un"}],"NEXT_ID":2}`), 0o644); err != nil {
		t.Fatal(err)
	}

	e.update(t, e.admin1, nil, []string{"1", "2", "3", "4"}, []deletedRound245{{ID: "r1", Questions: "DELETE"}})

	r := e.result(t)
	if r.Status != "APPLIED" || strings.Join(sortedCopy245(r.Deleted), ",") != "10,11" {
		t.Fatalf("DELETED_QUESTIONS = exactement les dossiers supprimés : %+v", r)
	}
	for _, id := range r.Deleted {
		if e.exists(id) {
			t.Errorf("le dossier %s annoncé supprimé existe encore", id)
		}
	}
	for _, id := range []string{"1", "2", "3", "4"} {
		if !e.exists(id) {
			t.Errorf("la question %s est conservée", id)
		}
	}
}

// =============================== mineur 1 — rounds.json illisible ===============================

func TestRoundsHardening245_Minor1_UnreadableRoundsJsonIsLoggedBackedUpAndRefusesRoundsUpdate(t *testing.T) {
	e, _ := newEnv245Sandboxed(t)
	e.base(t)
	e.writeQ(t, "10", 0, "r3", "")
	bad := `{"ROUNDS": [ {not json`
	if err := os.WriteFile(filepath.Join(e.dir, "rounds.json"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}

	e.update(t, e.admin1, []roundSpec245{{Name: "Neuve", Questions: []string{"1"}, Entractes: boolp(false)}}, []string{"2", "3", "4", "10"}, nil)

	r := e.result(t)
	if r.Status != "REFUSED" {
		t.Errorf("rounds.json illisible : ROUNDS_UPDATE refusé (il écraserait le fichier) : %+v", r)
	}
	if b, _ := os.ReadFile(filepath.Join(e.dir, "rounds.json")); string(b) != bad && !strings.Contains(strings.Join(corruptCopies245(t, e.dir), " "), "corrupt") {
		t.Error("le fichier illisible n'est pas écrasé sans copie de sauvegarde")
	}
	if len(corruptCopies245(t, e.dir)) == 0 {
		t.Error("une copie « .corrupt » du fichier illisible est conservée dans le dossier des questions")
	}
	if !e.hasLog(game.LogLevelWarn, "rounds.json") {
		t.Errorf("WARN mentionnant rounds.json ; logs=%v", e.logLines(game.LogLevelWarn))
	}
}

func TestRoundsHardening245_Minor1_NewRoundIDIsNeverAnIDAlreadyCarriedByAQuestion(t *testing.T) {
	e, _ := newEnv245Sandboxed(t)
	e.base(t)
	e.writeQ(t, "10", 0, "r3", "") // une question porte encore ROUND_ID r3 (rounds.json perdu)

	e.update(t, e.admin1, []roundSpec245{{Name: "Neuve", Questions: []string{"1"}, Entractes: boolp(false)}}, []string{"2", "3", "4", "10"}, nil)

	r := e.result(t)
	if r.Status != "APPLIED" || len(r.Rounds) != 1 {
		t.Fatalf("%+v", r)
	}
	switch r.Rounds[0].ID {
	case "r1", "r2", "r3":
		t.Errorf("l'ID %q est déjà porté par une question (r3) ou antérieur : NEXT_ID tient compte des ROUND_ID rencontrés, jamais de réutilisation", r.Rounds[0].ID)
	}
}

// =============================== mineur 4 — manche contenant la question en cours ===============================

func TestRoundsHardening245_Minor4_DeletingARoundHoldingTheCurrentQuestionIsRefusedOrForcedToKeep(t *testing.T) {
	e, _ := newEnv245Sandboxed(t)
	e.base(t)
	e.writeQ(t, "10", 0, "r1", "")
	if err := os.WriteFile(filepath.Join(e.dir, "rounds.json"), []byte(`{"ROUNDS":[{"ID":"r1","NAME":"Un"}],"NEXT_ID":2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	e.app.engine.SetPhase(game.PhaseStopped)
	e.send(t, e.admin1, "admin", "READY", map[string]interface{}{"QUESTION": "10"})
	e.until("READY ou PREPARE", func() bool { p := e.app.engine.GetPhase(); return p == game.PhaseReady || p == game.PhasePrepare })

	e.update(t, e.admin1, nil, []string{"1", "2", "3", "4"}, []deletedRound245{{ID: "r1", Questions: "DELETE"}})

	r := e.result(t)
	if r.Status == "APPLIED" && !e.exists("10") {
		t.Error("la question en cours (10) ne peut pas être supprimée : refus (QUESTION_IN_PROGRESS) ou KEEP imposé")
	}
	if r.Status == "APPLIED" {
		for _, id := range r.Deleted {
			if id == "10" {
				t.Error("DELETED_QUESTIONS ne contient pas la question en cours")
			}
		}
	}
}

func TestRoundsHardening245_Minor4_DeletedQuestionsStatusesArePurged(t *testing.T) {
	e, _ := newEnv245Sandboxed(t)
	e.base(t)
	e.writeQ(t, "10", 0, "r1", "")
	if err := os.WriteFile(filepath.Join(e.dir, "rounds.json"), []byte(`{"ROUNDS":[{"ID":"r1","NAME":"Un"}],"NEXT_ID":2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// 10 est jouée (STOPPED), puis on passe à la question 1 : plus de question courante dans la manche
	e.app.engine.SetPhase(game.PhaseStopped)
	e.app.engine.Ready("", &game.Question{ID: "10", Type: game.QuestionTypeSpeedy})
	e.app.engine.StartImmediate(5)
	e.app.engine.Stop()
	e.app.engine.SetPhase(game.PhaseStopped)
	e.app.engine.Ready("", &game.Question{ID: "1", Type: game.QuestionTypeSpeedy})
	if e.app.engine.GetQuestionStatus("10") != game.StatusStopped {
		t.Fatal("précondition : la question 10 est jouée")
	}

	e.update(t, e.admin1, nil, []string{"1", "2", "3", "4"}, []deletedRound245{{ID: "r1", Questions: "DELETE"}})

	if r := e.result(t); r.Status != "APPLIED" {
		t.Fatalf("%+v", r)
	}
	if st := e.app.engine.GetQuestionStatus("10"); st == game.StatusStopped {
		t.Errorf("le statut d'une question supprimée est purgé (%s) : un ID réutilisé ne doit pas apparaître « jouée »", st)
	}
}

func itoa245(i int) string { return strconv.Itoa(i) }

func sortedCopy245(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func jsonUnmarshal245(b []byte, v interface{}) error { return json.Unmarshal(b, v) }

func corruptCopies245(t *testing.T, dir string) []string {
	t.Helper()
	entries, _ := os.ReadDir(dir)
	var out []string
	for _, e := range entries {
		if strings.Contains(e.Name(), "corrupt") {
			out = append(out, e.Name())
		}
	}
	return out
}

// =============================== re-revue — rollback complet et WARN unique ===============================

func TestRoundsHardening245_Critical_Rollback_QuestionFilesAlreadyRewrittenGetTheirOriginalContentBack(t *testing.T) {
	e, root := newEnv245Sandboxed(t)
	ids := []string{"1", "2", "3", "4", "5", "6", "7", "8", "99"} // 99 est traitée en dernier si l'ordre est celui des IDs
	for i, id := range ids {
		e.writeQ(t, id, i, "", "")
	}
	// 99 ne peut plus être réécrite : l'échec survient APRÈS la réécriture des autres
	qd := filepath.Join(e.dir, "99")
	if err := os.Chmod(filepath.Join(qd, "question.json"), 0o444); err != nil {
		t.Skip("chmod indisponible")
	}
	if !readOnlyDir245(t, qd) {
		t.Skip("le système de fichiers n'applique pas les droits : injection d'échec impossible ici")
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(qd, "question.json"), 0o644) })
	before := treeSnapshot245(t, root)

	e.update(t, e.admin1, []roundSpec245{{Name: "Tout", Questions: ids, Entractes: boolp(false)}}, []string{}, nil)

	r := e.result(t)
	if r.Status == "APPLIED" {
		t.Fatalf("un échec d'écriture n'est jamais APPLIED : %+v", r)
	}
	_ = os.Chmod(qd, 0o755)
	if ok, why := sameTree245(before, treeSnapshot245(t, root)); !ok {
		t.Errorf("après l'échec, les question.json déjà réécrites retrouvent leur contenu d'origine, rounds.json n'apparaît pas (%s)", why)
	}
}

func TestRoundsHardening245_Minor1b_UnreadableRoundsJsonWarnsOncePerCorruptState(t *testing.T) {
	e, _ := newEnv245Sandboxed(t)
	e.base(t)
	if err := os.WriteFile(filepath.Join(e.dir, "rounds.json"), []byte(`{"ROUNDS": [ {not json`), 0o644); err != nil {
		t.Fatal(err)
	}

	// de nombreux calculs relisent rounds.json (READY, NEXT_QUESTION, ROUNDS_UPDATE refusé…)
	for i := 0; i < 5; i++ {
		_ = e.app.getNextQuestionPayload()
		e.app.engine.SetPhase(game.PhaseStopped)
		e.send(t, e.admin1, "admin", "READY", map[string]interface{}{"QUESTION": "1"})
	}
	e.update(t, e.admin1, nil, []string{"1", "2", "3", "4"}, nil)
	_ = e.result(t)
	e.update(t, e.admin1, nil, []string{"1", "2", "3", "4"}, nil)
	_ = e.result(t)

	n := 0
	for _, l := range e.logLines(game.LogLevelWarn) {
		low := strings.ToLower(l)
		if strings.Contains(low, "rounds.json") && (strings.Contains(low, "unreadable") || strings.Contains(low, "illisible") || strings.Contains(low, "corrupt")) {
			n++
		}
	}
	if n != 1 {
		t.Errorf("le WARN « rounds.json illisible » est émis UNE seule fois par état corrompu (pas à chaque lecture) : %d fois", n)
	}
}
