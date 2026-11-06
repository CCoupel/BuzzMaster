// #288 (v11.4.0, LOT-3) — contracts/sound.md §13.1 : MediaPlayer.PlayLoop. Même discipline que media_219_test.go :
// le constructeur RÉEL est appelé sans double ; sans matériel audio (CI/sandbox) la construction dégrade vers le
// lecteur neutre, où PlayLoop est un no-op — la sémantique « 3 passages ⇒ 1 seul onNaturalEnd » n'est alors vérifiée
// que sur machine équipée (QUALIF enceinte, procédure tests/procedures/lot-3). Réutilise tw219mShortWAV.
// Compile avant le code : PlayLoop est cherchée par réflexion.
package audio

import (
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func TestMediaPlayerLoop288_InterfaceDeclaresPlayLoop(t *testing.T) {
	it := reflect.TypeOf((*MediaPlayer)(nil)).Elem()
	m, ok := it.MethodByName("PlayLoop")
	if !ok {
		t.Fatal("MediaPlayer.PlayLoop(path string) error absent de l'interface (sound.md §13.1)")
	}
	if m.Type.NumIn() != 1 || m.Type.NumOut() != 1 {
		t.Errorf("signature PlayLoop = %s, attendu func(string) error", m.Type)
	}
}

func tw288callLoop(t *testing.T, p MediaPlayer, path string) error {
	t.Helper()
	m := reflect.ValueOf(p).MethodByName("PlayLoop")
	if !m.IsValid() {
		t.Fatal("le lecteur n'implémente pas PlayLoop (sound.md §13.1)")
	}
	out := m.Call([]reflect.Value{reflect.ValueOf(path)})
	if err, _ := out[0].Interface().(error); err != nil {
		return err
	}
	return nil
}

func TestMediaPlayerLoop288_Neutral_PlayLoopIsNoOp_NeverEndsNaturally(t *testing.T) {
	var ends atomic.Int32
	p := NewMediaPlayer(OutputConfig{}, func() { ends.Add(1) })
	defer p.Close()
	if !IsNeutralMedia(p) {
		t.Skip("lecteur réel disponible : couvert par TestMediaPlayerLoop288_Real_*")
	}
	if err := tw288callLoop(t, p, "/chemin/inexistant.wav"); err != nil {
		t.Fatalf("lecteur neutre : PlayLoop doit être un no-op sans erreur, got %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if p.State() == MediaPlaying {
		t.Error("lecteur neutre : jamais PLAYING")
	}
	if ends.Load() != 0 {
		t.Error("lecteur neutre : onNaturalEnd ne doit jamais être appelé")
	}
}

// Machine équipée : une boucle de ~45 ms rejouée 3+ fois ⇒ onNaturalEnd EXACTEMENT une fois (fin du 1er passage),
// State() reste PLAYING d'un tour à l'autre, Stop met fin à la boucle sans nouvel appel.
func TestMediaPlayerLoop288_Real_ManyPasses_OneNaturalEnd_StaysPlaying_StopEnds(t *testing.T) {
	var ends atomic.Int32
	p := NewMediaPlayer(OutputConfig{}, func() { ends.Add(1) })
	defer p.Close()
	if IsNeutralMedia(p) {
		t.Skip("pas de matériel audio : sémantique vérifiée en QUALIF enceinte")
	}
	path := tw219mShortWAV(t, t.TempDir(), "loop.wav", 2000)
	if err := tw288callLoop(t, p, path); err != nil {
		t.Fatalf("PlayLoop : %v", err)
	}
	deadline := time.Now().Add(tw219mGenerousTimeout)
	for ends.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if ends.Load() != 1 {
		t.Fatalf("onNaturalEnd = %d à la fin du 1er passage, attendu 1", ends.Load())
	}
	time.Sleep(400 * time.Millisecond) // ≥ 3 passages de plus
	if ends.Load() != 1 {
		t.Errorf("onNaturalEnd = %d après plusieurs passages, attendu toujours 1 (§13.1)", ends.Load())
	}
	if p.State() != MediaPlaying {
		t.Errorf("State() = %v pendant la boucle, attendu PLAYING", p.State())
	}
	p.Stop()
	time.Sleep(150 * time.Millisecond)
	if p.State() != MediaIdle || ends.Load() != 1 {
		t.Errorf("après Stop : State=%v ends=%d, attendu IDLE / 1", p.State(), ends.Load())
	}
}

// Stop pendant le 1er passage ⇒ jamais d'onNaturalEnd (règle Play inchangée, §13.1).
func TestMediaPlayerLoop288_Real_StopBeforeFirstPassEnds_NoNaturalEnd(t *testing.T) {
	var ends atomic.Int32
	p := NewMediaPlayer(OutputConfig{}, func() { ends.Add(1) })
	defer p.Close()
	if IsNeutralMedia(p) {
		t.Skip("pas de matériel audio")
	}
	path := tw219mShortWAV(t, t.TempDir(), "long.wav", 44100*3)
	if err := tw288callLoop(t, p, path); err != nil {
		t.Fatalf("PlayLoop : %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	p.Stop()
	time.Sleep(300 * time.Millisecond)
	if ends.Load() != 0 {
		t.Errorf("onNaturalEnd = %d après Stop anticipé, attendu 0", ends.Load())
	}
}
