// Suite test-writer pour #219 (milestone v11.1 — plan
// _work/reports/plan-20260922-103848.md Phase 0 tâches 2/3, contract
// §10.2) : MediaPlayer — le second chemin de lecture asynchrone, distinct
// du moteur de cues (§5), qui rend possibles les gestes de conduite
// (rejouer, pause, stop) que #219 introduit.
//
// Comme output_228_test.go (même paquet), ce fichier appelle le
// CONSTRUCTEUR RÉEL (NewMediaPlayer) sans double : sur une machine de
// CI/sandbox typique — confirmée sans PulseAudio/ALSA réels par
// output_228_test.go — la construction dégrade vers l'implémentation
// neutre PARCE QUE le contexte oto singleton (§10.3, partagé avec
// NewOutput) est déjà refusé dans cet environnement. Les tests qui
// exercent un VRAI cycle de lecture (Play/Pause/Resume/Stop) sont donc
// gated sur IsNeutralMedia et s'auto-limitent à vérifier la dégradation
// silencieuse dans ce cas — même discipline que isneutral_230_test.go et
// output_228_test.go ("les deux issues sont valides selon l'environnement,
// seule leur COHÉRENCE mutuelle est vérifiée").
//
// Constructeur réel (media.go, livré par dev-backend en parallèle de ce
// fichier) : NewMediaPlayer(cfg OutputConfig, onNaturalEnd func()) MediaPlayer
// — réutilise OutputConfig (pas de config média séparée, cohérent avec
// contract §10.6 : la sélection de périphérique reste portée par
// OutputConfig) et prend un rappel de fin naturelle, invoqué au plus une
// fois par Play() SI ET SEULEMENT SI la lecture se termine TOUTE SEULE
// (jamais sur Pause/Stop/Close/remplacement, contract §10.7). Les tests
// ci-dessous passent `nil` quand ce rappel n'est pas nécessaire à
// l'assertion (State() suffit à observer la fin naturelle par polling).
//
// Convention de collision : préfixe tw219m pour ne jamais entrer en
// collision avec un helper d'un autre fichier de ce paquet.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package audio

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

const tw219mGenerousTimeout = 15 * time.Second

// tw219mShortWAV builds a short, structurally valid canonical WAV file on
// disk — digital silence, frame-aligned, so running this suite on a
// machine with REAL working audio hardware produces no audible click (same
// technique as otoOutput.prime's silence, output_oto.go).
func tw219mShortWAV(t *testing.T, dir, name string, frames int) string {
	t.Helper()
	data := make([]byte, frames*FrameSize)
	buf := make([]byte, 44+len(data))
	copy(buf[0:4], "RIFF")
	tw219mPutU32(buf[4:8], uint32(36+len(data)))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	tw219mPutU32(buf[16:20], 16)
	tw219mPutU16(buf[20:22], 1)
	tw219mPutU16(buf[22:24], uint16(ChannelCount))
	tw219mPutU32(buf[24:28], uint32(SampleRate))
	tw219mPutU32(buf[28:32], uint32(SampleRate*ChannelCount*BytesPerSample))
	tw219mPutU16(buf[32:34], uint16(ChannelCount*BytesPerSample))
	tw219mPutU16(buf[34:36], uint16(BitsPerSample))
	copy(buf[36:40], "data")
	tw219mPutU32(buf[40:44], uint32(len(data)))
	copy(buf[44:], data)

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf, 0644); err != nil {
		t.Fatalf("setup invalide : écriture du WAV de test a échoué : %v", err)
	}
	return path
}

func tw219mPutU32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func tw219mPutU16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

// tw219mWaitForState polls State() until it equals want or the timeout
// elapses, returning whether want was actually reached.
func tw219mWaitForState(t *testing.T, m MediaPlayer, want MediaState, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m.State() == want {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return m.State() == want
}

// ---------------------------------------------------------------------------
// Construction et dégradation (contract §10.2, §5.5 réutilisé).
// ---------------------------------------------------------------------------

func TestNewMediaPlayer_NeverNilAndConsistentWithIsNeutralMedia(t *testing.T) {
	done := make(chan MediaPlayer, 1)
	go func() { done <- NewMediaPlayer(OutputConfig{}, nil) }()

	select {
	case mp := <-done:
		if mp == nil {
			t.Fatal("NewMediaPlayer a renvoyé nil — contract §5.5/§10.2 : dégradation silencieuse, jamais un MediaPlayer absent")
		}
		defer mp.Close()
		neutral := IsNeutralMedia(mp)
		t.Logf("NewMediaPlayer() -> %T, IsNeutralMedia=%v (les deux issues sont valides selon l'environnement, même discipline que isneutral_230_test.go)", mp, neutral)
	case <-time.After(tw219mGenerousTimeout):
		t.Fatalf("NewMediaPlayer() n'a pas retourné après %s — la construction doit rester BORNÉE, comme NewOutput (contract §5.5)", tw219mGenerousTimeout)
	}
}

func TestIsNeutralMedia_Nil_IsTrue(t *testing.T) {
	if !IsNeutralMedia(nil) {
		t.Error("IsNeutralMedia(nil) doit être true — symétriquement à IsNeutral(nil) (contract §10.2 : « symétriquement à IsNeutral »)")
	}
}

func TestMediaPlayer_Close_Idempotent(t *testing.T) {
	mp := NewMediaPlayer(OutputConfig{}, nil)
	if mp == nil {
		t.Fatal("setup invalide : NewMediaPlayer a renvoyé nil")
	}
	for i := 0; i < 3; i++ {
		if err := mp.Close(); err != nil {
			t.Errorf("Close() #%d a renvoyé une erreur, attendu nil (idempotent) : %v", i, err)
		}
	}
}

func TestMediaPlayer_StopAtRest_IsNoOp(t *testing.T) {
	mp := NewMediaPlayer(OutputConfig{}, nil)
	if mp == nil {
		t.Fatal("setup invalide : NewMediaPlayer a renvoyé nil")
	}
	defer mp.Close()

	if mp.State() != MediaIdle {
		t.Fatalf("setup invalide : un MediaPlayer neuf doit démarrer à MediaIdle, got %q", mp.State())
	}
	done := make(chan struct{})
	go func() { mp.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(tw219mGenerousTimeout):
		t.Fatal("Stop() au repos ne doit jamais bloquer (plan §9 : « Stop au repos = no-op »)")
	}
	if mp.State() != MediaIdle {
		t.Errorf("Stop() au repos a changé l'état : %q, attendu MediaIdle inchangé", mp.State())
	}
}

// ---------------------------------------------------------------------------
// Dégradation neutre (plan §9 : « aucune goroutine, aucune erreur propagée,
// IsNeutralMedia() vrai »).
// ---------------------------------------------------------------------------

func TestMediaPlayer_NeutralDegradation_PlayNeverErrorsAndStaysIdle(t *testing.T) {
	mp := NewMediaPlayer(OutputConfig{}, nil)
	if mp == nil {
		t.Fatal("setup invalide : NewMediaPlayer a renvoyé nil")
	}
	defer mp.Close()
	if !IsNeutralMedia(mp) {
		t.Skip("cette machine dispose d'un vrai backend audio — la dégradation neutre est couverte ailleurs (TestOutputAndMediaPlayer_...) ; voir la branche réelle, TestMediaPlayer_RealCycle_...")
	}

	path := tw219mShortWAV(t, t.TempDir(), "sound.wav", int(0.05*float64(SampleRate)))
	if err := mp.Play(path); err != nil {
		t.Errorf("Play() sur un MediaPlayer neutre ne doit jamais propager d'erreur dans le jeu (contract §5.5 réutilisé par §10.2 : « aucune erreur propagée dans le jeu ») : %v", err)
	}
	mp.Pause()
	mp.Resume()
	mp.Stop()
	if mp.State() != MediaIdle {
		t.Errorf("un MediaPlayer neutre ne doit jamais quitter MediaIdle (aucune goroutine réelle, contract §10.2) : got %q", mp.State())
	}
}

// ---------------------------------------------------------------------------
// Cycle réel (plan §9 : « cycle Play → Pause → Resume → Stop ; Play pendant
// une lecture en cours REMPLACE (voix unique) ; Stop au repos = no-op ; fin
// naturelle ⇒ MediaIdle ») — exercé seulement quand un vrai backend est
// disponible (voir l'en-tête de fichier : neutre par défaut dans ce
// sandbox).
// ---------------------------------------------------------------------------

func TestMediaPlayer_RealCycle_PlayPauseResumeStopAndSingleVoice(t *testing.T) {
	// naturalEndCount vérifie, en plus du cycle d'état, le rappel
	// onNaturalEnd lui-même (contract §10.7) : jamais incrémenté par un
	// Stop() explicite, incrémenté UNE fois par une vraie fin naturelle.
	var naturalEndCount int32
	mp := NewMediaPlayer(OutputConfig{}, func() { atomic.AddInt32(&naturalEndCount, 1) })
	if mp == nil {
		t.Fatal("setup invalide : NewMediaPlayer a renvoyé nil")
	}
	defer mp.Close()
	if IsNeutralMedia(mp) {
		t.Skip("aucun backend audio réel disponible ici (dégradation neutre, contract §5.5) — voir TestMediaPlayer_NeutralDegradation_...")
	}

	dir := t.TempDir()
	// ~2s : assez long pour observer PLAYING/PAUSED de façon fiable sans
	// rendre la suite trop lente sur une vraie machine.
	long := tw219mShortWAV(t, dir, "long.wav", 2*SampleRate)

	if err := mp.Play(long); err != nil {
		t.Fatalf("Play() a échoué sur un backend réel : %v", err)
	}
	if !tw219mWaitForState(t, mp, MediaPlaying, tw219mGenerousTimeout) {
		t.Fatalf("Play() n'a jamais atteint MediaPlaying, état final %q", mp.State())
	}

	// Voix unique (contract §10.2 : « une question chasse l'autre ») : un
	// second Play REMPLACE la lecture en cours, il ne la superpose jamais.
	short := tw219mShortWAV(t, dir, "short.wav", int(0.3*float64(SampleRate)))
	if err := mp.Play(short); err != nil {
		t.Fatalf("Play() de remplacement a échoué : %v", err)
	}
	if mp.State() != MediaPlaying {
		t.Errorf("un Play de remplacement doit rester en MediaPlaying (voix unique, pas d'arrêt intermédiaire visible), got %q", mp.State())
	}

	mp.Pause()
	if !tw219mWaitForState(t, mp, MediaPaused, tw219mGenerousTimeout) {
		t.Fatalf("Pause() n'a jamais atteint MediaPaused, état final %q", mp.State())
	}
	mp.Resume()
	if !tw219mWaitForState(t, mp, MediaPlaying, tw219mGenerousTimeout) {
		t.Fatalf("Resume() n'a jamais atteint MediaPlaying, état final %q", mp.State())
	}
	mp.Stop()
	if !tw219mWaitForState(t, mp, MediaIdle, tw219mGenerousTimeout) {
		t.Fatalf("Stop() n'a jamais ramené à MediaIdle, état final %q", mp.State())
	}
	if atomic.LoadInt32(&naturalEndCount) != 0 {
		t.Errorf("onNaturalEnd a été appelé après un Stop() EXPLICITE — contract §10.7 : « l'appelant qui invoque Stop() sait déjà qu'il l'a fait », le rappel ne doit signaler QUE la fin naturelle")
	}

	// Fin naturelle (sans Stop explicite) ⇒ MediaIdle, sur un extrait très
	// court — la machine à états A de la maquette rév.3 §03.
	veryShort := tw219mShortWAV(t, dir, "veryshort.wav", int(0.05*float64(SampleRate)))
	if err := mp.Play(veryShort); err != nil {
		t.Fatalf("Play() a échoué : %v", err)
	}
	if !tw219mWaitForState(t, mp, MediaIdle, tw219mGenerousTimeout) {
		t.Fatalf("la fin naturelle d'un extrait de 50ms n'a jamais ramené à MediaIdle sans Stop() explicite, état final %q", mp.State())
	}
	// La transition d'état peut être observée une fraction avant que le
	// rappel (exécuté juste après, dans la même goroutine monitor) ne soit
	// invoqué — attente bornée courte plutôt qu'une lecture immédiate.
	deadline := time.Now().Add(time.Second)
	for atomic.LoadInt32(&naturalEndCount) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := atomic.LoadInt32(&naturalEndCount); got != 1 {
		t.Errorf("onNaturalEnd doit être appelé EXACTEMENT une fois après une vraie fin naturelle (contract §10.7), got %d", got)
	}
}
