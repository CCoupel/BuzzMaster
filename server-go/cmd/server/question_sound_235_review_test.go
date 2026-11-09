// #235 (v11.6.0, LOT-2) — tests correctifs de la revue LOT-2 (_work/reports/code-reviewer-lot2-v11.6.0.md) :
// M1 (le décompte ne joue pas pendant le son d'une carte MEMOTION — sound.md §14.2/§14.4 « jamais pendant ce média »,
// §12.6) et m1 (RESTORE coupe le média en cours, §14.3). Réutilise tw287App (lecteur n°2 du décompte factice) et
// les helpers tw235*. Préfixe tw235r.
package main

import (
	"testing"
	"time"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/game"
)

// tw235rApp : App du décompte (tw287App) + adaptateur de son de question sur un faux lecteur, fichiers sons présents.
func tw235rApp(t *testing.T) (*App, *tw235Player, *tw287Fake) {
	t.Helper()
	app, countdown := tw287App(t, tw287Active)
	media := &tw235Player{}
	app.qsound = &questionSoundAdapter{app: app, player: media}
	tw219qSetSoundEnabledFor(app, true)
	t.Cleanup(func() { tw257CancelWatchdog(app) })
	return app, media, countdown
}

func tw235rPlayedCountdown(f *tw287Fake) int { return f.count("Play:") + f.count("PlayLoop:") }

// Précondition (garde-fou du harnais) : une carte SANS son, hôte sans son ⇒ le décompte joue bien au flip.
func TestCountdown235_Precondition_CardWithoutSound_CountdownPlays(t *testing.T) {
	app, _, countdown := tw235rApp(t)
	q := tw235Memotion(t, `[{"ID":"c1","RECTO_THEME":"un","DIFFICULTY":1},{"ID":"c2","RECTO_THEME":"deux","DIFFICULTY":1}]`, "")
	q.Time = "30"
	tw235StartMemotion(t, app, q)
	tw235Select(t, app, "c1")
	tw235Flip(t, app)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && tw235rPlayedCountdown(countdown) == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if tw235rPlayedCountdown(countdown) == 0 {
		t.Skip("harnais : le chrono de carte ne déclenche pas le décompte ici — M1 non vérifiable (voir tw287)")
	}
}

// M1 — carte sonore NON différée, hôte sans son : le décompte NE joue PAS pendant le son de la carte.
func TestCountdown235_M1_CardSoundNotDelayed_CountdownSilent(t *testing.T) {
	app, media, countdown := tw235rApp(t)
	tw235WriteSound(t, app, "mq1", "card1.wav")
	q := tw235Memotion(t, tw235TwoCards, "") // c1 sonore, hôte muet
	q.Time = "30"
	tw235StartMemotion(t, app, q)
	tw235Select(t, app, "c1")
	tw235Flip(t, app)

	time.Sleep(300 * time.Millisecond)
	if media.State() != audio.MediaPlaying {
		t.Fatalf("setup invalide : le son de la carte devrait jouer, ops=%v", media.snapshot())
	}
	if n := tw235rPlayedCountdown(countdown); n != 0 {
		t.Errorf("M1 : le décompte ne doit jamais jouer pendant le média d'une carte (§12.6), journal=%v", countdown.journal())
	}
}

// M1 (variante) — carte sonore différée : même exclusion.
func TestCountdown235_M1_CardSoundDelayed_CountdownSilent(t *testing.T) {
	app, _, countdown := tw235rApp(t)
	tw235WriteSound(t, app, "mq1", "card1.wav")
	cards := `[{"ID":"c1","RECTO_THEME":"un","DIFFICULTY":1,"SOUND":"/question/mq1/card1.wav","SOUND_TIMER_DELAYED":true},
 {"ID":"c2","RECTO_THEME":"deux","DIFFICULTY":1}]`
	q := tw235Memotion(t, cards, "")
	q.Time = "30"
	tw235StartMemotion(t, app, q)
	tw235Select(t, app, "c1")
	tw235Flip(t, app)
	time.Sleep(300 * time.Millisecond)
	if n := tw235rPlayedCountdown(countdown); n != 0 {
		t.Errorf("M1 (différé) : aucun décompte attendu pendant le média de la carte, journal=%v", countdown.journal())
	}
}

// M1 (hôte sonore) — comportement v11.4 conservé : l'hôte avec son exclut déjà le décompte de toutes ses cartes.
func TestCountdown235_M1_HostSound_StillExcludesCountdown(t *testing.T) {
	app, _, countdown := tw235rApp(t)
	tw235WriteSound(t, app, "mq1", "host.wav")
	q := tw235Memotion(t, `[{"ID":"c1","RECTO_THEME":"un","DIFFICULTY":1},{"ID":"c2","RECTO_THEME":"deux","DIFFICULTY":1}]`, `{"SOUND":"/question/mq1/host.wav"}`)
	q.Time = "30"
	tw235StartMemotion(t, app, q)
	tw235Select(t, app, "c1")
	tw235Flip(t, app)
	time.Sleep(300 * time.Millisecond)
	if n := tw235rPlayedCountdown(countdown); n != 0 {
		t.Errorf("hôte sonore : aucun décompte attendu, journal=%v", countdown.journal())
	}
}

// m1 — RESTORE (restauration d'une sauvegarde) coupe le média en cours (§14.3 : « au RESTORE »).
func TestRestore235_StopsRunningMedia(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, app *App)
	}{
		{"son d'entracte", func(t *testing.T, app *App) {
			tw235WriteSound(t, app, "ent1", "pause.wav")
			q := tw235Entracte(t, `{"SOUND":"/question/ent1/pause.wav","SOUND_LOOP":true}`)
			app.engine.Ready(q.ID, q)
			app.handleEntracteSet(entracteSetMsg(true))
		}},
		{"son d'hôte MEMOTION", func(t *testing.T, app *App) {
			tw235WriteSound(t, app, "mq1", "host.wav")
			tw235StartMemotion(t, app, tw235Memotion(t, tw235TwoCards, `{"SOUND":"/question/mq1/host.wav","SOUND_LOOP":true}`))
		}},
		{"son de carte MEMOTION", func(t *testing.T, app *App) {
			tw235WriteSound(t, app, "mq1", "card1.wav")
			tw235StartMemotion(t, app, tw235Memotion(t, tw235TwoCards, ""))
			tw235Select(t, app, "c1")
			tw235Flip(t, app)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, fake := tw235App(t)
			tc.setup(t, app)
			if fake.State() != audio.MediaPlaying {
				t.Fatalf("setup invalide : un média devrait jouer, ops=%v", fake.snapshot())
			}
			app.httpServer.OnAction("RESTORE", nil)
			if fake.State() != audio.MediaIdle {
				t.Errorf("RESTORE : le média doit être coupé, state=%v", fake.State())
			}
			if st := app.engine.GetState().QuestionSoundState; st != game.QuestionSoundIdle {
				t.Errorf("RESTORE : QUESTION_SOUND_STATE=IDLE attendu, got %q", st)
			}
		})
	}
}
