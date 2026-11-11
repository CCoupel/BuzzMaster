// #290 (v11.6.0, LOT-4) — effet de la résolution sur les TROIS chemins sonores : bruitages (notifySound), son de
// question / carte / entracte (LOT-2) et décompte. « Son coupé » imposé par la manche ⇒ tout est muet ; une cue
// coupée/réactivée par la manche est respectée (nouvelles cues du LOT-3 comprises) ; `sound.enabled` (matériel)
// reste une garde en amont, jamais surchargeable. Contrat : ambiance-settings.md §4.3–§4.4, §5.2 ; sound.md §11.2,
// §12.4, §14. Écrit depuis le contrat avant le code (rouge attendu). Réutilise tw290New / tw235Player / tw287Fake.
// Préfixe tw290s.
package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/config"
	"buzzcontrol/internal/game"
)

type tw290sEnv struct {
	*tw290Env
	cues      *audio.FakeOutput
	media     *tw235Player
	countdown *tw287Fake
}

func tw290sNew(t *testing.T, cfgJSON string) *tw290sEnv {
	t.Helper()
	e := tw290New(t)
	if err := config.PatchJSON([]byte(cfgJSON)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { config.Mutate(func(c *config.Config) { c.Sound = config.SoundConfig{} }) })
	s := &tw290sEnv{tw290Env: e, cues: audio.NewFakeOutput(), media: &tw235Player{}, countdown: &tw287Fake{}}
	e.app.soundEngine.Store(e.app.newSoundEngine(s.cues))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go e.app.sound().Start(ctx)
	e.app.qsound = &questionSoundAdapter{app: e.app, player: s.media}
	tw219qSetSoundEnabledFor(e.app, true)
	t.Cleanup(func() { tw257CancelWatchdog(e.app) })
	tw287InstallCountdown(t, e.app, s.countdown)
	return s
}

const tw290sActive = `{"sound":{"enabled":true,"muted":false,"countdown_disabled":false,"countdown_once":false,"cues_disabled":{}}}`

func tw290sAmb(sound string) interface{} {
	var v interface{}
	_ = json.Unmarshal([]byte(`{"sound":`+sound+`}`), &v)
	return v
}

// attend que le moteur de bruitages ait joué au moins n cues (ou 500 ms) puis renvoie la liste.
func (s *tw290sEnv) played(n int) []audio.Cue {
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && s.cues.Count() < n {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(60 * time.Millisecond)
	return s.cues.Played()
}

// ---------------------------------------------------------------------------
// Son coupé par la manche : bruitages, question, décompte.
// ---------------------------------------------------------------------------

func TestRoundAmbiance290Sound_RoundMuted_CuesAreSilent_OutsideRoundTheyPlay(t *testing.T) {
	s := tw290sNew(t, tw290sActive)
	s.twoRounds(t, tw290sAmb(`{"muted":true}`))

	s.ready(t, "4") // hors manche
	s.app.notifySound(audio.CueGagne)
	if got := s.played(1); len(got) != 1 || got[0] != audio.CueGagne {
		t.Fatalf("hors manche, gagne doit jouer (non-régression), got %v", got)
	}

	s.ready(t, "1") // manche « son coupé »
	before := s.cues.Count()
	s.app.notifySound(audio.CuePerdu)
	s.app.notifySound(audio.Cue("rafale-bonne"))
	time.Sleep(150 * time.Millisecond)
	if s.cues.Count() != before {
		t.Errorf("manche « son coupé » : aucune cue ne doit jouer, got %v", s.cues.Played())
	}
}

func TestRoundAmbiance290Sound_RoundMuted_QuestionMediaDoesNotPlay(t *testing.T) {
	s := tw290sNew(t, tw290sActive)
	s.twoRounds(t, tw290sAmb(`{"muted":true}`))
	s.app.config.Storage.QuestionsDir = s.dir
	dir := filepath.Join(s.dir, "1")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "snd.wav"), audio.BuildCanonicalWAV(make([]byte, audio.SampleRate*audio.FrameSize)), 0o644)
	s.ready(t, "1")

	q := &game.Question{ID: "1", Type: game.QuestionTypeSpeedy, Sound: "/question/1/snd.wav"}
	s.app.qsound.Start(q)
	time.Sleep(100 * time.Millisecond)
	if n := s.media.count("play:") + s.media.count("loop:"); n != 0 {
		t.Errorf("« son coupé » par la manche : le média de question ne doit pas jouer (T1 comme §11.2), ops=%v", s.media.snapshot())
	}
}

func TestRoundAmbiance290Sound_RoundMuted_CountdownDoesNotPlay(t *testing.T) {
	s := tw290sNew(t, tw290sActive)
	s.twoRounds(t, tw290sAmb(`{"muted":true}`))
	s.ready(t, "1")
	s.app.countdownSound().OnAnswerTimerStarted(game.AnswerTimerStart{})
	time.Sleep(100 * time.Millisecond)
	if n := tw235rPlayedCountdown(s.countdown); n != 0 {
		t.Errorf("« son coupé » par la manche : le décompte ne joue pas, journal=%v", s.countdown.journal())
	}
}

// ---------------------------------------------------------------------------
// Décompte : actif / boucle surchargés par la manche.
// ---------------------------------------------------------------------------

func TestRoundAmbiance290Sound_RoundDisablesCountdown_ThenLoopOverride(t *testing.T) {
	s := tw290sNew(t, tw290sActive)
	s.twoRounds(t, tw290sAmb(`{"countdown":{"enabled":false}}`))
	s.ready(t, "1")
	s.app.countdownSound().OnAnswerTimerStarted(game.AnswerTimerStart{})
	time.Sleep(80 * time.Millisecond)
	if tw235rPlayedCountdown(s.countdown) != 0 {
		t.Errorf("décompte désactivé par la manche : silence, journal=%v", s.countdown.journal())
	}
	s.ready(t, "4") // hors manche : le décompte est actif (global)
	s.app.countdownSound().OnAnswerTimerStarted(game.AnswerTimerStart{})
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && tw235rPlayedCountdown(s.countdown) == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if tw235rPlayedCountdown(s.countdown) == 0 {
		t.Error("hors manche, le décompte (global actif) doit jouer")
	}
}

func TestRoundAmbiance290Sound_RoundCountdownOnce_UsesPlayNotLoop(t *testing.T) {
	s := tw290sNew(t, tw290sActive) // global : en boucle
	s.twoRounds(t, tw290sAmb(`{"countdown":{"loop":false}}`))
	s.ready(t, "1")
	s.app.countdownSound().OnAnswerTimerStarted(game.AnswerTimerStart{})
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && tw235rPlayedCountdown(s.countdown) == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if s.countdown.count("Play:") != 1 || s.countdown.count("PlayLoop:") != 0 {
		t.Errorf("la manche impose « une fois » : Play et non PlayLoop, journal=%v", s.countdown.journal())
	}
}

// ---------------------------------------------------------------------------
// Cues par manche (nouvelles cues du LOT-3 comprises).
// ---------------------------------------------------------------------------

func TestRoundAmbiance290Sound_RoundCutsAndReenablesCues(t *testing.T) {
	s := tw290sNew(t, `{"sound":{"enabled":true,"muted":false,"cues_disabled":{"reveal":true}}}`) // reveal coupée au global
	s.twoRounds(t, tw290sAmb(`{"cues":{"gagne":false,"reveal":true,"qcm-indice":false}}`))

	s.ready(t, "1")
	for _, c := range []audio.Cue{audio.CueGagne, audio.CueReveal, audio.CueDepart, audio.Cue("qcm-indice"), audio.Cue("memory-carte")} {
		s.app.notifySound(c)
	}
	got := s.played(3)
	want := map[audio.Cue]bool{audio.CueReveal: true, audio.CueDepart: true, audio.Cue("memory-carte"): true}
	if len(got) != 3 {
		t.Fatalf("3 cues attendues (reveal réactivée par la manche, depart, memory-carte), got %v", got)
	}
	for _, c := range got {
		if !want[c] {
			t.Errorf("cue %q ne devait pas jouer en manche r1 (gagne et qcm-indice coupées par la manche), got %v", c, got)
		}
	}

	// hors manche : le global reprend (gagne active, reveal coupée)
	s.ready(t, "4")
	n := s.cues.Count()
	s.app.notifySound(audio.CueReveal)
	s.app.notifySound(audio.CueGagne)
	after := s.played(n + 1)
	if len(after) != n+1 || after[n] != audio.CueGagne {
		t.Errorf("hors manche : gagne joue, reveal reste coupée (global), got %v", after)
	}
}

// Garde matérielle en amont : sound.enabled=false ⇒ rien, même si la manche active tout (jamais surchargeable).
func TestRoundAmbiance290Sound_HardwareGuard_EnabledFalse_NotOverridable(t *testing.T) {
	s := tw290sNew(t, `{"sound":{"enabled":false,"muted":true,"cues_disabled":{"gagne":true}}}`)
	s.twoRounds(t, tw290sAmb(`{"muted":false,"cues":{"gagne":true}}`))
	s.ready(t, "1")
	s.app.notifySound(audio.CueGagne)
	time.Sleep(150 * time.Millisecond)
	if s.cues.Count() != 0 {
		t.Errorf("sound.enabled=false : aucune cue, la manche ne peut pas rouvrir la sortie, got %v", s.cues.Played())
	}
	if tw290Path(s.settings(t), "global", "sound", "muted") != true {
		t.Error("sanity : le global (muted=true) reste lu")
	}
}

// Effets de bord de « son coupé » déclenchés par la RÉSOLUTION (§4.3) : entrer dans une manche muette coupe le média en cours.
func TestRoundAmbiance290Sound_EnteringMutedRound_StopsRunningMedia(t *testing.T) {
	s := tw290sNew(t, tw290sActive)
	s.twoRounds(t, tw290sAmb(`{"muted":true}`))
	dir := filepath.Join(s.dir, "4")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "snd.wav"), audio.BuildCanonicalWAV(make([]byte, audio.SampleRate*audio.FrameSize)), 0o644)
	s.ready(t, "4")
	s.app.qsound.Start(&game.Question{ID: "4", Type: game.QuestionTypeSpeedy, Sound: "/question/4/snd.wav", SoundLoop: true})
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && s.media.State() != audio.MediaPlaying {
		time.Sleep(5 * time.Millisecond)
	}
	if s.media.State() != audio.MediaPlaying {
		t.Fatalf("setup invalide : le média devrait jouer, ops=%v", s.media.snapshot())
	}
	s.ready(t, "1") // entrée dans la manche « son coupé »
	if s.media.State() != audio.MediaIdle {
		t.Errorf("entrée dans une manche « son coupé » : le média en cours doit être coupé (effets §11.2 via la résolution), state=%v", s.media.State())
	}
}
