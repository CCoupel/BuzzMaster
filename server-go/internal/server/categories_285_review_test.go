// #285 (v11.4.0, LOT-4) — correctifs de la revue : 409 historique seul, noms réservés Windows,
// .bak sans écrasement, sérialisation des écritures, échec sidecar, erreurs 500 génériques.
package server

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"buzzcontrol/internal/game"
)

func TestCategories285Review_Delete409WhenOnlyHistoryReferencesCategory(t *testing.T) {
	s, _ := dev285Server(t)
	hp := filepath.Join(t.TempDir(), "history.json")
	s.engine.SetHistoryPath(hp)
	dev285Create(t, s, "Cinema", "")
	s.engine.AddGameEvent(game.GameEvent{QuestionCategory: "CINEMA", EventType: "POINTS_AWARDED", Points: 10})
	s.engine.SaveHistory()

	rec := dev285Do(s, "DELETE", "/api/categories/CINEMA", nil, "")
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), "category in use") || !strings.Contains(rec.Body.String(), `"events":1`) ||
		!strings.Contains(rec.Body.String(), `"questions":0`) || !strings.Contains(rec.Body.String(), `"rafale":0`) || !strings.Contains(rec.Body.String(), `"points":10`) {
		t.Fatalf("DELETE sans mode, historique seul : %d %s", rec.Code, rec.Body.String())
	}
	if h := s.engine.GetHistory(); len(h) != 1 || h[0].QuestionCategory != "CINEMA" {
		t.Error("l'historique ne doit pas être réécrit sans confirmation")
	}
	if baks, _ := filepath.Glob(hp + ".bak-*"); len(baks) != 0 {
		t.Errorf("aucune sauvegarde attendue sans confirmation : %v", baks)
	}
	if _, ok := dev285Get(t, s)["CINEMA"]; !ok {
		t.Error("la catégorie ne doit pas être supprimée")
	}
	if rec := dev285JSON(s, "DELETE", "/api/categories/CINEMA", `{"mode":"reassign"}`); rec.Code != 200 {
		t.Fatalf("avec mode : %d %s", rec.Code, rec.Body.String())
	}
}

func TestCategories285Review_ReservedWindowsNamesRejected(t *testing.T) {
	s, _ := dev285Server(t)
	for _, name := range []string{"CON", "prn", "Aux", "NUL", "COM1", "com9", "Lpt1", "LPT9"} {
		body, ct := dev285Multipart(t, map[string]string{"name": name}, "i.png", dev285PNG)
		if rec := dev285Do(s, "POST", "/api/categories", body, ct); rec.Code != 400 || !strings.Contains(rec.Body.String(), `"error":"reserved name"`) {
			t.Errorf("%q : %d %s, attendu 400", name, rec.Code, rec.Body.String())
		}
	}
	dev285Create(t, s, "Console", "") // préfixe seulement : accepté
	dev285Create(t, s, "Cinéma", "")  // clé Unicode CINÉMA : acceptée
}

func TestCategories285Review_HistoryBackupNeverOverwritten(t *testing.T) {
	e := game.NewEngine()
	dir := t.TempDir()
	e.SetHistoryPath(filepath.Join(dir, "history.json"))
	seen := map[string]bool{}
	for i := 0; i < 3; i++ {
		e.AddGameEvent(game.GameEvent{QuestionCategory: "K", EventType: "POINTS_AWARDED", Points: 5})
		e.SaveHistory()
		_, _, b, err := e.MergeHistoryCategoryWithBackup("K")
		if err != nil || b == "" {
			t.Fatalf("merge %d : %q %v", i, b, err)
		}
		if seen[b] {
			t.Fatalf("sauvegarde %q réutilisée (écrasement)", b)
		}
		seen[b] = true
	}
	n := 0
	entries, _ := os.ReadDir(dir)
	for _, en := range entries {
		if strings.HasPrefix(en.Name(), "history.json.bak-") {
			n++
		}
	}
	if n != 3 {
		t.Errorf("%d sauvegardes, attendu 3", n)
	}
}

func TestCategories285Review_ConcurrentCreatesSerialised(t *testing.T) {
	s, _ := dev285Server(t)
	var wg sync.WaitGroup
	codes := make([]int, 8)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body, ct := dev285Multipart(t, map[string]string{"name": "Meme"}, "i.png", dev285PNG)
			codes[i] = dev285Do(s, "POST", "/api/categories", body, ct).Code
		}(i)
	}
	wg.Wait()
	ok, conflict := 0, 0
	for _, c := range codes {
		switch c {
		case 200:
			ok++
		case 409:
			conflict++
		}
	}
	if ok != 1 || conflict != len(codes)-1 {
		t.Errorf("codes = %v, attendu un seul 200 et le reste 409", codes)
	}
}

func TestCategories285Review_SidecarFailure_500AndImagePurged(t *testing.T) {
	s, dataDir := dev285Server(t)
	catDir := filepath.Join(dataDir, "files", "categories")
	_ = os.MkdirAll(filepath.Join(catDir, "ZED.json"), 0755) // le renommage du sidecar échoue
	body, ct := dev285Multipart(t, map[string]string{"name": "Zed"}, "i.png", dev285PNG)
	rec := dev285Do(s, "POST", "/api/categories", body, ct)
	if rec.Code != 500 {
		t.Fatalf("code = %d %s, attendu 500", rec.Code, rec.Body.String())
	}
	entries, _ := os.ReadDir(catDir)
	for _, en := range entries {
		if !en.IsDir() {
			t.Errorf("fichier résiduel %q après échec", en.Name())
		}
	}
}

func TestCategories285Review_500DoesNotLeakDiskPaths(t *testing.T) {
	s, _ := dev285Server(t)
	dev285Create(t, s, "Cinema", "")
	blockerDir := t.TempDir()
	blocker := filepath.Join(blockerDir, "blocker")
	_ = os.WriteFile(blocker, []byte("x"), 0644)
	s.engine.SetHistoryPath(filepath.Join(blocker, "sub", "history.json"))
	s.engine.AddGameEvent(game.GameEvent{QuestionCategory: "CINEMA", EventType: "POINTS_AWARDED", Points: 5})

	rec := dev285JSON(s, "DELETE", "/api/categories/CINEMA", `{"mode":"reassign"}`)
	if rec.Code != 500 {
		t.Fatalf("code = %d %s, attendu 500", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), blockerDir) || strings.Contains(rec.Body.String(), "blocker") {
		t.Errorf("chemin disque dans la réponse : %s", rec.Body.String())
	}
}
