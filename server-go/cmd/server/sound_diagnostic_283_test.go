// #283 (v11.6.0, LOT-1) — diagnostic de la sortie audio côté App : ligne de démarrage unique routée vers le
// logger serveur (composant AUDIO) et état servi par GET /api/sound/status.
// Contrat : contracts/http-endpoints.md « GET /api/sound/status — diagnostic » + « Journal de démarrage ».
//
// Frontière : on appelle le vrai (*App).setupSound() ; le journal est lu via le logger global serveur ;
// l'état de diagnostic est lu par assertion d'interface locale (SoundDiagnostic() — signature à confirmer
// avec dev-backend), de sorte que le fichier COMPILE avant le dev et échoue par assertion.
// Aucune sortie audio réelle n'est exigée : sur un poste/CI sans carte son, le cas `unavailable` est
// accepté au même titre que `ok` pour sound.enabled=true (dépend de l'environnement).
package main

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"buzzcontrol/internal/game"
	"buzzcontrol/internal/server"
)

type tw283Diagnostic interface {
	SoundDiagnostic() (enabled bool, output, reason string, restartRequired bool)
}

// tw283BootApp prépare une App, capture le logger global et pose sound.enabled (config process-wide,
// restaurée en fin de test).
func tw283BootApp(t *testing.T, enabledAtBoot bool) (*App, *server.BroadcastLogger) {
	t.Helper()
	app := newTestApp(t)
	app.config.Storage.FilesDir = t.TempDir()
	prevEnabled := app.config.Sound.Enabled
	app.config.Sound.Enabled = enabledAtBoot
	prevLogger := server.GetGlobalLogger()
	bl := server.InitLogger(200)
	t.Cleanup(func() {
		app.config.Sound.Enabled = prevEnabled
		server.SetGlobalLogger(prevLogger)
	})
	app.setupSound()
	return app, bl
}

func tw283StartupLines(bl *server.BroadcastLogger) []game.LogEntry {
	var out []game.LogEntry
	for _, e := range bl.GetHistory() {
		if e.Component == game.LogComponentAudio && strings.HasPrefix(e.Message, "sound: enabled=") {
			out = append(out, e)
		}
	}
	return out
}

func tw283Diag(t *testing.T, app *App) (bool, string, string, bool) {
	t.Helper()
	d, ok := any(app).(tw283Diagnostic)
	if !ok {
		t.Fatal("(*App) doit exposer SoundDiagnostic() (enabled, output, reason, restartRequired) pour GET /api/sound/status")
	}
	return d.SoundDiagnostic()
}

// P1 — une seule ligne de démarrage, composant AUDIO, format du contrat.
func TestSoundBoot283_StartupLine_Disabled_SingleLineContractFormat(t *testing.T) {
	_, bl := tw283BootApp(t, false)
	lines := tw283StartupLines(bl)
	if len(lines) != 1 {
		t.Fatalf("exactement UNE ligne de démarrage `sound: enabled=…` attendue (composant AUDIO), got %d", len(lines))
	}
	msg := lines[0].Message
	re := regexp.MustCompile(`^sound: enabled=false output=disabled reason="sound\.enabled=false" device=\S+ format=44100Hz/2ch/16bit \(config: .+\)$`)
	if !re.MatchString(msg) {
		t.Errorf("format du contrat non respecté :\n  got  %q\n  want sound: enabled=<b> output=<…> reason=\"…\" device=<…> format=44100Hz/2ch/16bit (config: <chemin>)", msg)
	}
	if lines[0].Level != game.LogLevelInfo {
		t.Errorf("sound.enabled=false est un choix, pas une panne : niveau INFO attendu, got %v", lines[0].Level)
	}
}

func TestSoundBoot283_StartupLine_Enabled_OkOrUnavailableWithReason(t *testing.T) {
	app, bl := tw283BootApp(t, true)
	lines := tw283StartupLines(bl)
	if len(lines) != 1 {
		t.Fatalf("exactement UNE ligne de démarrage attendue, got %d", len(lines))
	}
	_, output, reason, _ := tw283Diag(t, app)
	switch output {
	case "ok":
		if !strings.Contains(lines[0].Message, "enabled=true output=ok") {
			t.Errorf("ligne incohérente avec l'état ok : %q", lines[0].Message)
		}
		if reason != "" {
			t.Errorf("reason attendu vide si ok, got %q", reason)
		}
		if lines[0].Level != game.LogLevelInfo {
			t.Errorf("ok ⇒ niveau INFO, got %v", lines[0].Level)
		}
	case "unavailable":
		if !strings.Contains(lines[0].Message, "enabled=true output=unavailable") {
			t.Errorf("ligne incohérente avec l'état unavailable : %q", lines[0].Message)
		}
		if strings.TrimSpace(reason) == "" {
			t.Error("unavailable ⇒ une cause lisible est exigée (reason non vide)")
		}
		// Le journal écrit reason="…" avec %q (guillemets/antislashs échappés) : on compare à la forme quotée,
		// indépendante de l'environnement (la cause oto d'un runner sans carte son contient des guillemets).
		if !strings.Contains(lines[0].Message, fmt.Sprintf("reason=%q", reason)) {
			t.Errorf("la cause doit être le MÊME texte dans le journal et dans le statut : reason=%q ligne=%q", reason, lines[0].Message)
		}
		if lines[0].Level == game.LogLevelInfo {
			t.Error("une sortie demandée mais indisponible est une anomalie : niveau WARN/ERROR attendu, pas INFO")
		}
	default:
		t.Errorf("enabled=true au démarrage ⇒ output ∈ {ok, unavailable}, got %q", output)
	}
}

// P2 — état servi par le statut.
func TestSoundBoot283_Diagnostic_Disabled(t *testing.T) {
	app, _ := tw283BootApp(t, false)
	enabled, output, reason, restart := tw283Diag(t, app)
	if enabled || output != "disabled" || reason != "sound.enabled=false" || restart {
		t.Errorf("attendu (false, disabled, \"sound.enabled=false\", false), got (%v, %q, %q, %v)", enabled, output, reason, restart)
	}
}

// restart_required : interrupteur changé depuis le démarrage, dans un sens ou dans l'autre.
func TestSoundBoot283_Diagnostic_FlipFalseToTrue_RestartRequired(t *testing.T) {
	app, _ := tw283BootApp(t, false)
	app.config.Sound.Enabled = true // changé via l'interface APRÈS le démarrage
	enabled, output, reason, restart := tw283Diag(t, app)
	if enabled {
		t.Error("enabled = valeur lue AU DÉMARRAGE (false), pas la valeur courante")
	}
	if output != "restart_required" || !restart {
		t.Errorf("output=restart_required et restart_required=true attendus, got %q / %v", output, restart)
	}
	if reason != "enabled flipped false→true after startup" {
		t.Errorf("reason attendue du contrat, got %q", reason)
	}
}

func TestSoundBoot283_Diagnostic_FlipTrueToFalse_RestartRequired(t *testing.T) {
	app, _ := tw283BootApp(t, true)
	app.config.Sound.Enabled = false
	_, output, reason, restart := tw283Diag(t, app)
	if output != "restart_required" || !restart {
		t.Errorf("output=restart_required et restart_required=true attendus, got %q / %v", output, restart)
	}
	if reason != "enabled flipped true→false after startup" {
		t.Errorf("reason attendue du contrat, got %q", reason)
	}
}

// Revenir à la valeur du démarrage annule l'exigence de redémarrage (comparaison à la valeur LIVE).
func TestSoundBoot283_Diagnostic_FlipBack_ClearsRestartRequired(t *testing.T) {
	app, _ := tw283BootApp(t, false)
	app.config.Sound.Enabled = true
	if _, _, _, restart := tw283Diag(t, app); !restart {
		t.Fatal("setup invalide : restart_required attendu après l'inversion")
	}
	app.config.Sound.Enabled = false
	_, output, _, restart := tw283Diag(t, app)
	if restart || output != "disabled" {
		t.Errorf("valeur live = valeur du démarrage ⇒ output=disabled / restart_required=false, got %q / %v", output, restart)
	}
}
