package synth

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"buzzcontrol/internal/audio"
)

// #287 (contract sound.md §12.2) — le tic-tac par défaut : 2,00 s, stéréo canonique, amplitude <= 0,5,
// silence aux deux extrémités (boucle sans clic), déterministe.
func TestGenerateCountdown_Canonical2sQuietDeterministic(t *testing.T) {
	data := GenerateCountdown()
	res, err := audio.ValidateQuestionSound(data)
	if err != nil {
		t.Fatalf("le décompte par défaut doit passer ValidateQuestionSound : %v", err)
	}
	if d := res.Duration.Seconds(); d < 1.999 || d > 2.001 {
		t.Errorf("durée = %.4f s, attendu 2,00 s", d)
	}
	if !bytes.Equal(data, GenerateCountdown()) {
		t.Error("GenerateCountdown doit être déterministe")
	}
	pcm := res.PCM
	var peak int
	for i := 0; i+1 < len(pcm); i += 2 {
		v := int(int16(binary.LittleEndian.Uint16(pcm[i:])))
		if v < 0 {
			v = -v
		}
		if v > peak {
			peak = v
		}
	}
	if float64(peak) > 0.5*32767+1 {
		t.Errorf("amplitude crête = %d, attendu <= 0,5 × pleine échelle", peak)
	}
	if pcm[0] != 0 || pcm[1] != 0 || pcm[len(pcm)-1] != 0 || pcm[len(pcm)-2] != 0 {
		t.Error("le fichier doit commencer et finir au silence (boucle sans clic)")
	}
}

func TestWriteAll_WritesCountdownOnlyIfAbsent_AndOverwriteRestores(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteAll(dir, false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, CountdownFilename())
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("decompte.wav absent après WriteAll : %v", err)
	}
	if err := os.WriteFile(path, []byte("perso"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteAll(dir, false); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "perso" {
		t.Error("WriteAll(overwrite=false) ne doit jamais écraser un décompte existant")
	}
	m, err := ReconcileManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if e := m[CountdownName]; !e.Present || !e.Custom {
		t.Errorf("manifeste decompte = %+v, attendu present+custom", e)
	}
	if _, err := WriteAll(dir, true); err != nil {
		t.Fatal(err)
	}
	m, _ = ReconcileManifest(dir)
	if e := m[CountdownName]; !e.Present || e.Custom {
		t.Errorf("après restore, manifeste decompte = %+v, attendu present, non custom", e)
	}
}
