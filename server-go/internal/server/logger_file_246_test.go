package server

// #246 (v11.2) — journalisation persistante : section `logging` de config.json, écriture
// fichier (niveau minimal, format), rotation bornée, défauts, erreur disque non bloquante.
// Contrat : contracts/logging.md §1.
//
// Symboles attendus (voir message au dev-backend) : config.Config.Logging,
// (*BroadcastLogger).SetFileOutput(config.LoggingConfig) error — écriture synchrone.
// Les valeurs de configuration sont lues via un re-marshal JSON (indépendant du type des champs).

import (
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func loadConfig246(t *testing.T, loggingJSON string) *config.Config {
	t.Helper()
	body := `{"server":{"http_port":0}}`
	if loggingJSON != "" {
		body = fmt.Sprintf(`{"server":{"http_port":0},"logging":%s}`, loggingJSON)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

func loggingKeys246(t *testing.T, cfg *config.Config) map[string]interface{} {
	t.Helper()
	raw, _ := json.Marshal(cfg)
	var all map[string]json.RawMessage
	_ = json.Unmarshal(raw, &all)
	section, ok := all["logging"]
	if !ok {
		t.Fatalf("la configuration doit exposer la section `logging` (tag json) : %s", raw)
	}
	out := map[string]interface{}{}
	_ = json.Unmarshal(section, &out)
	return out
}

// --- Défauts -----------------------------------------------------------------------------

func TestLoggingConfig246_AbsentSectionYieldsTheDocumentedDefaults(t *testing.T) {
	got := loggingKeys246(t, loadConfig246(t, ""))

	want := map[string]interface{}{
		"file_enabled":     true,
		"file_path":        "./data/logs/buzzcontrol.log",
		"file_min_level":   "WARN",
		"file_max_size_mb": float64(10),
		"file_max_files":   float64(5),
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("logging.%s = %v (%T), contrat = %v", k, got[k], got[k], v)
		}
	}
}

func TestLoggingConfig246_ExplicitValuesAreKept_FalseIncluded(t *testing.T) {
	got := loggingKeys246(t, loadConfig246(t, `{"file_enabled":false,"file_path":"/var/log/bz.log","file_min_level":"INFO","file_max_size_mb":2,"file_max_files":3}`))

	want := map[string]interface{}{
		"file_enabled": false, "file_path": "/var/log/bz.log", "file_min_level": "INFO",
		"file_max_size_mb": float64(2), "file_max_files": float64(3),
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("logging.%s = %v, attendu %v (un `false` explicite ne doit pas être remplacé par le défaut)", k, got[k], v)
		}
	}
}

func TestLoggingConfig246_PartialSectionFillsTheMissingKeys(t *testing.T) {
	got := loggingKeys246(t, loadConfig246(t, `{"file_min_level":"ERROR"}`))

	if got["file_min_level"] != "ERROR" || got["file_enabled"] != true || got["file_max_files"] != float64(5) {
		t.Errorf("section partielle : %v", got)
	}
}

// --- Écriture fichier ------------------------------------------------------------------------

func newFileLogger246(t *testing.T, loggingJSON string) (*BroadcastLogger, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "logs", "sub", "buzzcontrol.log") // le répertoire n'existe pas : créé au démarrage
	cfgJSON := strings.Replace(loggingJSON, "@PATH@", filepath.ToSlash(path), 1)
	cfg := loadConfig246(t, cfgJSON)
	bl := NewBroadcastLogger(1000)
	if err := bl.SetFileOutput(cfg.Logging); err != nil {
		t.Fatalf("SetFileOutput: %v", err)
	}
	return bl, path
}

func readLog246(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("le fichier de logs %s doit exister : %v", path, err)
	}
	return string(b)
}

func TestFileLogging246_DefaultMinLevelWritesWarnAndErrorOnly(t *testing.T) {
	bl, path := newFileLogger246(t, `{"file_path":"@PATH@"}`)

	bl.Debug(game.LogComponentApp, "ligne debug")
	bl.Info(game.LogComponentApp, "ligne info")
	bl.Warn(game.LogComponentApp, "ligne warn %d", 1)
	bl.Error(game.LogComponentApp, "ligne error")

	content := readLog246(t, path)
	if strings.Contains(content, "ligne debug") || strings.Contains(content, "ligne info") {
		t.Errorf("WARN par défaut : DEBUG/INFO ne sont pas écrits sur disque :\n%s", content)
	}
	if !strings.Contains(content, "ligne warn 1") || !strings.Contains(content, "ligne error") {
		t.Errorf("WARN et ERROR sont écrits :\n%s", content)
	}
	// Le tampon mémoire (et /ws/logs) reste inchangé : INFO y est.
	found := false
	for _, e := range bl.GetHistory() {
		if strings.Contains(e.Message, "ligne info") {
			found = true
		}
	}
	if !found {
		t.Error("le tampon mémoire doit continuer à recevoir les entrées INFO")
	}
}

func TestFileLogging246_MinLevelIsHonoured(t *testing.T) {
	bl, path := newFileLogger246(t, `{"file_path":"@PATH@","file_min_level":"ERROR"}`)
	bl.Warn(game.LogComponentApp, "avertissement")
	bl.Error(game.LogComponentApp, "erreur")

	content := readLog246(t, path)
	if strings.Contains(content, "avertissement") || !strings.Contains(content, "erreur") {
		t.Errorf("file_min_level=ERROR :\n%s", content)
	}
}

func TestFileLogging246_LineFormatIsRFC3339MillisLevelComponentMessage(t *testing.T) {
	bl, path := newFileLogger246(t, `{"file_path":"@PATH@"}`)
	bl.Warn(game.LogComponentApp, `ARDOISE_CLEARED team="Les Verts" prev_len=14`)

	line := strings.TrimSpace(readLog246(t, path))
	re := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}(Z|[+-]\d{2}:\d{2}) \[WARN\] \[[^\]]+\] ARDOISE_CLEARED team="Les Verts" prev_len=14$`)
	if !re.MatchString(line) {
		t.Errorf("format attendu `2026-10-10T14:03:21.123+02:00 [WARN] [App] message`, obtenu :\n%s", line)
	}
}

func TestFileLogging246_DisabledWritesNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "off.log")
	cfg := loadConfig246(t, fmt.Sprintf(`{"file_enabled":false,"file_path":%q}`, filepath.ToSlash(path)))
	bl := NewBroadcastLogger(100)
	if err := bl.SetFileOutput(cfg.Logging); err != nil {
		t.Fatalf("SetFileOutput: %v", err)
	}
	bl.Error(game.LogComponentApp, "rien sur disque")

	if _, err := os.Stat(path); err == nil {
		t.Error("file_enabled=false : comportement antérieur (mémoire + sortie standard), aucun fichier")
	}
	if len(bl.GetHistory()) != 1 {
		t.Error("le tampon mémoire continue de fonctionner")
	}
}

// --- Rotation ---------------------------------------------------------------------------------

func TestFileLogging246_RotationIsBoundedBySizeAndFileCount(t *testing.T) {
	bl, path := newFileLogger246(t, `{"file_path":"@PATH@","file_max_size_mb":1,"file_max_files":3}`)
	line := strings.Repeat("x", 1000)
	for i := 0; i < 6000; i++ { // ≈ 6 Mo écrits pour 1 Mo par fichier
		bl.Warn(game.LogComponentApp, "n=%05d %s", i, line)
	}

	dir := filepath.Dir(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		info, _ := e.Info()
		names = append(names, e.Name())
		if info.Size() > 1024*1024+4096 {
			t.Errorf("%s dépasse file_max_size_mb : %d octets", e.Name(), info.Size())
		}
	}
	if len(names) > 3 {
		t.Errorf("file_max_files=3 (courant compris), trouvé %d fichiers : %v", len(names), names)
	}
	if len(names) < 2 {
		t.Errorf("une rotation aurait dû produire buzzcontrol.log.1 : %v", names)
	}
	base := filepath.Base(path)
	if _, err := os.Stat(filepath.Join(dir, base+".1")); err != nil {
		t.Errorf("%s.1 attendu (buzzcontrol.log → buzzcontrol.log.1 → …) : %v", base, err)
	}
	if _, err := os.Stat(filepath.Join(dir, base+".3")); err == nil {
		t.Errorf("%s.3 ne doit pas exister avec file_max_files=3", base)
	}
	// Le fichier courant contient les entrées les plus récentes.
	if !strings.Contains(readLog246(t, path), "n=05999") {
		t.Error("le fichier courant doit contenir la dernière ligne écrite")
	}
}

// --- Erreur disque ------------------------------------------------------------------------------

func TestFileLogging246_DiskErrorNeverStopsTheLogger(t *testing.T) {
	// file_path sous un FICHIER ordinaire : le répertoire ne peut pas être créé.
	blocker := filepath.Join(t.TempDir(), "iamafile")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := loadConfig246(t, fmt.Sprintf(`{"file_path":%q}`, filepath.ToSlash(filepath.Join(blocker, "logs", "bz.log"))))
	bl := NewBroadcastLogger(100)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("une erreur disque ne doit jamais faire paniquer le serveur : %v", r)
		}
	}()
	_ = bl.SetFileOutput(cfg.Logging) // erreur retournée ou non : jamais bloquant
	for i := 0; i < 5; i++ {
		bl.Error(game.LogComponentApp, "toujours en mémoire %d", i)
	}

	if len(bl.GetHistory()) != 5 {
		t.Errorf("le tampon mémoire continue : %d entrées", len(bl.GetHistory()))
	}
}
