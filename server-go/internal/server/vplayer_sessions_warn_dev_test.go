package server

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"buzzcontrol/internal/game"
)

// Contrat vplayer-session.md §6 : règle commune du WARN des places sans empreinte (démarrage et restauration).
func TestVPlayerSessionsWarnDev_RuleTable(t *testing.T) {
	absent := &fs.PathError{Op: "open", Path: "vplayer_sessions.json", Err: fs.ErrNotExist}
	corrupt := errors.New("vplayer_sessions.json illisible: invalid json")
	cases := []struct {
		name  string
		seats int
		err   error
		warn  bool
		cause string
	}{
		{"fichier absent, 0 place : silence", 0, absent, false, ""},
		{"fichier chargé, 0 place : silence", 0, nil, false, ""},
		{"fichier corrompu, 0 place : WARN", 0, corrupt, true, "illisible ou corrompu"},
		{"fichier absent, 2 places : WARN", 2, absent, true, "absent"},
		{"fichier chargé, 1 place (sauvegarde ancienne) : WARN", 1, nil, true, "sauvegarde d'avant v13.0.0"},
		{"fichier corrompu, 3 places : WARN", 3, corrupt, true, "illisible ou corrompu"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bl := NewBroadcastLogger(100)
			SetGlobalLogger(bl)
			t.Cleanup(func() { SetGlobalLogger(nil) })
			got := LogVPlayerSessionsWarn(game.LogComponentHTTP, "restauration", c.seats, c.err, "sauvegarde d'avant v13.0.0 ou sans vplayer_sessions.json")
			if got != c.warn {
				t.Fatalf("WARN = %v, attendu %v", got, c.warn)
			}
			var line string
			for _, e := range bl.GetHistory() {
				if strings.Contains(e.Message, "VPLAYER_SESSIONS") {
					line = e.Message
				}
			}
			if (line != "") != c.warn {
				t.Fatalf("ligne de journal = %q, WARN attendu %v", line, c.warn)
			}
			if c.warn && (!strings.Contains(line, c.cause) || !strings.Contains(line, "place(s)")) {
				t.Errorf("cause %q et nombre de places attendus : %s", c.cause, line)
			}
		})
	}
}
