package game

import (
	"os"
	"path/filepath"
	"testing"
)

// #249 review mineur 4: first normalisation at load keeps teams.json.bak, never overwrites it.
func TestLoadTeams_NormalisationKeepsAOneShotBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "teams.json")
	orig := `{"Les Bleus":{"NAME":"Ancien nom"}}`
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	e := NewEngine()
	e.SetTeamsPath(path)
	if err := e.LoadTeams(); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(path + ".bak"); err != nil || string(b) != orig {
		t.Fatalf("sauvegarde .bak du fichier d'origine attendue : %v %q", err, b)
	}
	if e.GetTeamsAndBumpers().Teams["Les Bleus"].Name != "Les Bleus" {
		t.Error("NAME normalisé")
	}
	// second repair with another bad file: the existing .bak is kept
	if err := os.WriteFile(path, []byte(`{"Les Rouges":{"NAME":"X"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.LoadTeams(); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path + ".bak"); string(b) != orig {
		t.Errorf(".bak existant non écrasé : %q", b)
	}
}

func TestLoadTeams_CleanFileCreatesNoBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "teams.json")
	_ = os.WriteFile(path, []byte(`{"A":{"NAME":"A"}}`), 0o644)
	e := NewEngine()
	e.SetTeamsPath(path)
	_ = e.LoadTeams()
	if _, err := os.Stat(path + ".bak"); err == nil {
		t.Error("aucune sauvegarde si rien n'est corrigé")
	}
}
