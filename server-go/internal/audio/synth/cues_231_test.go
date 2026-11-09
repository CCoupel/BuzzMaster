// #231 (v11.6.0, LOT-3) — sons par défaut des 8 nouvelles cues : synthétisés, présents, canoniques, courts, distincts,
// déterministes ; écrits par WriteAll (15 fichiers). Source : sound.md §2.1 amendement #231 (« chacune avec un son par
// défaut synthétisé, un fichier remplaçable et un interrupteur »), plan T15 (« 8 sons par défaut, courts, distincts »).
// Cues désignées par leur NOM de contrat. Préfixe tw231s.
package synth

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"buzzcontrol/internal/audio"
)

var tw231sNew = []audio.Cue{
	"qcm-indice", "memory-carte", "memory-paire", "memotion-selection", "memotion-fin-carte",
	"rafale-bonne", "rafale-mauvaise", "rafale-fin-lot",
}

func TestDefaults231_EveryNewCueIsGenerated_Canonical_UnderCueLimit(t *testing.T) {
	for _, c := range tw231sNew {
		data, ok := Generate(c)
		if !ok || len(data) == 0 {
			t.Errorf("%s : aucun son par défaut synthétisé", c)
			continue
		}
		res, err := audio.ValidateUpload(data)
		if err != nil {
			t.Errorf("%s : le son par défaut doit passer la validation d'upload (canonique, ≤ 5 s) : %v", c, err)
			continue
		}
		// « courts » : sous la cible de 2 s du seuil d'avertissement d'un bruitage, sans avertissement.
		if res.Duration > audio.WarnSoundDuration || res.Warning != "" {
			t.Errorf("%s : durée %v — un bruitage par défaut doit rester court (≤ %v)", c, res.Duration, audio.WarnSoundDuration)
		}
		if res.Duration < 50*time.Millisecond {
			t.Errorf("%s : durée %v trop brève pour être audible", c, res.Duration)
		}
	}
}

func TestDefaults231_AllFifteenDistinct(t *testing.T) {
	seen := map[string]audio.Cue{}
	for _, c := range audio.AllCues() {
		data, ok := Generate(c)
		if !ok {
			t.Fatalf("%s : pas de son par défaut", c)
		}
		if other, dup := seen[string(data)]; dup {
			t.Errorf("%s et %s ont exactement le même son par défaut — chaque cue doit être distincte à l'oreille", c, other)
		}
		seen[string(data)] = c
	}
}

func TestDefaults231_Deterministic(t *testing.T) {
	for _, c := range tw231sNew {
		a, _ := Generate(c)
		b, _ := Generate(c)
		if !bytes.Equal(a, b) {
			t.Errorf("%s : la synthèse doit être déterministe (restauration idempotente)", c)
		}
	}
}

func TestDefaults231_Filename_And_CataloguePresent(t *testing.T) {
	if len(Cues) != 15 {
		t.Fatalf("synth.Cues doit lister les 15 cues, got %d", len(Cues))
	}
	for _, c := range tw231sNew {
		if Filename(c) != string(c)+".wav" {
			t.Errorf("Filename(%s) = %q", c, Filename(c))
		}
	}
}

func TestDefaults231_WriteAll_WritesFifteenFiles_AndNeverClobbersCustom(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteAll(dir, false); err != nil {
		t.Fatalf("WriteAll : %v", err)
	}
	for _, c := range audio.AllCues() {
		if _, err := os.Stat(filepath.Join(dir, Filename(c))); err != nil {
			t.Errorf("%s : fichier absent après WriteAll : %v", c, err)
		}
	}
	// un fichier personnalisé survit au démarrage suivant
	custom := filepath.Join(dir, Filename("memory-paire"))
	if err := os.WriteFile(custom, []byte("perso"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteAll(dir, false); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(custom); string(b) != "perso" {
		t.Error("un son personnalisé d'une nouvelle cue ne doit jamais être écrasé au démarrage")
	}
	// « restaurer » écrase, de façon idempotente
	if _, err := WriteAll(dir, true); err != nil {
		t.Fatal(err)
	}
	want, _ := Generate("memory-paire")
	if b, _ := os.ReadFile(custom); !bytes.Equal(b, want) {
		t.Error("la restauration doit réécrire le son par défaut")
	}
}
