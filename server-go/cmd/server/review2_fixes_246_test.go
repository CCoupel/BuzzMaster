package main

// #246 — corrections de la revue du Batch 2 : journalisation quotée, limite de taille du texte final.

import (
	"buzzcontrol/internal/game"
	"strings"
	"testing"
	"time"
)

func TestReview2_246_HashAndReasonCannotForgeLogPairs(t *testing.T) {
	e := newEnv246(t)
	conn, cid := e.dial(t, "/ws/player")
	e.connect(t, cid, "Paul", e.paul)
	e.typeAs(t, cid, e.paul, "PARIS", map[string]interface{}{"SEQ": 1})
	e.typeAs(t, cid, e.paul, "", map[string]interface{}{"SEQ": 2, "REASON": "clear x=1\n[ERROR] faux"})
	e.stop()
	bad := finalPayload246(e.paul, "q-ardoise", "A")
	bad["HASH"] = "zz zz=1\n[ERROR] faux"
	e.send(t, cid, "vplayer", "ARDOISE_FINAL", bad)
	noAction246(t, conn, "ARDOISE_FINAL_ACK", 100*time.Millisecond)

	n := 0
	for _, en := range e.app.logger.GetHistory() {
		if strings.Contains(en.Message, "ARDOISE_CLEARED") || strings.Contains(en.Message, "ARDOISE_FINAL_HASH_MISMATCH") {
			n++
			if strings.ContainsAny(en.Message, "\r\n") {
				t.Fatalf("saut de ligne dans une entrée : %q", en.Message)
			}
			// la valeur fournie doit rester enfermée dans des guillemets (pas de paire libre)
			for _, forged := range []string{" x=1", " zz=1"} {
				if i := strings.Index(en.Message, forged); i >= 0 && strings.Count(en.Message[:i], `"`)%2 == 0 {
					t.Fatalf("fausse paire clé=valeur fabriquée hors guillemets : %q", en.Message)
				}
			}
		}
	}
	if n != 2 {
		t.Errorf("2 entrées attendues, obtenu %d ; logs=%v", n, e.logLines(""))
	}
}

func TestReview2_246_FinalTextLimitIsCountedInCharacters(t *testing.T) {
	cases := []struct {
		name   string
		text   string
		accept bool
	}{
		{"2000 caractères ASCII", strings.Repeat("a", 2000), true},
		{"2001 caractères ASCII", strings.Repeat("a", 2001), false},
		{"2000 caractères multi-octets (6000 octets)", strings.Repeat("é", 2000), true},
		{"2001 caractères multi-octets", strings.Repeat("é", 2001), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv246(t)
			conn, cid := e.dial(t, "/ws/player")
			e.connect(t, cid, "Paul", e.paul)
			e.stop()

			e.send(t, cid, "vplayer", "ARDOISE_FINAL", finalPayload246(e.paul, "q-ardoise", tc.text))

			_, received := e.answers()
			if received["Les Bleus"] != tc.accept {
				t.Fatalf("RECEIVED = %v, attendu %v", received["Les Bleus"], tc.accept)
			}
			if !tc.accept {
				if !e.hasLog(game.LogLevelWarn, "ARDOISE_FINAL_TOO_LONG") {
					t.Errorf("WARN ARDOISE_FINAL_TOO_LONG attendu ; logs=%v", e.logLines(game.LogLevelWarn))
				}
				noAction246(t, conn, "ARDOISE_FINAL_ACK", 200*time.Millisecond)
			}
		})
	}
}
