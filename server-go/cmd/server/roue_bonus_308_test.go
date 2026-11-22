package main

// #308 (v12.1.0, LOT-1.1) — câblage de la notice BONUS (contracts/roue.md §15.4) : « bonne réponse » (CueRafaleBonne)
// + comète LED sur les buzzers de l'équipe ; ni flash de score, ni pulsation d'ambiance, ni crédit (aucun point).
// Test de câblage statique : le gestionnaire OnRoueNotice appelle des effets de bord matériels (audio, LED) qu'aucun
// double du harnais ne capture ; on vérifie donc le corps du `case` dans les sources (même approche que sound_sites_test.go).

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func bonusCaseBody308(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("roue.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	i := strings.Index(src, "case game.RoueNoticeBonus:")
	if i < 0 {
		t.Fatal("roue.go : `case game.RoueNoticeBonus:` absent de OnRoueNotice")
	}
	rest := src[i+len("case game.RoueNoticeBonus:"):]
	if j := regexp.MustCompile(`\n\t\tcase |\n\t\tdefault:|\n\t}\n`).FindStringIndex(rest); j != nil {
		rest = rest[:j[0]]
	}
	return rest
}

func TestRoueBonus308_NoticeBonusPlaysBonneReponseAndComet(t *testing.T) {
	body := bonusCaseBody308(t)
	for _, want := range []string{"audio.CueRafaleBonne", "sendLEDSetComet(n.Team)"} {
		if !strings.Contains(body, want) {
			t.Errorf("le traitement de la notice BONUS doit appeler %s :\n%s", want, body)
		}
	}
}

func TestRoueBonus308_NoticeBonusNeverFlashesScoreNorPulsesAmbiance(t *testing.T) {
	body := bonusCaseBody308(t)
	for _, forbidden := range []string{"startScoreFlash", "NotifyPulse", "broadcastAwardedTeams", "CueGagne"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("la notice BONUS ne doit jamais appeler %s (aucun point) :\n%s", forbidden, body)
		}
	}
}

// Le site de son reste inscrit au registre fermé (setupRoueCallbacks documente désormais la cue BONUS).
func TestRoueBonus308_SoundRegistryMentionsBonus(t *testing.T) {
	d, ok := soundSiteRegistry[soundSite{Func: "setupRoueCallbacks"}]
	if !ok {
		t.Fatal("setupRoueCallbacks absent de soundSiteRegistry")
	}
	if !strings.Contains(strings.ToUpper(d.Reason), "BONUS") {
		t.Errorf("l'entrée de registre de setupRoueCallbacks doit documenter la cue BONUS")
	}
}
