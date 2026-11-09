// #290 (v11.6.0, LOT-4 Batch 2, révision 3) — lecture d'un fichier de son PAR MANCHE : `FileBank.PCMAt(rel)` et
// `Engine.PlayCueAt(c, rel)` (file `{cue, rel}`, garanties §5 inchangées ; `PlayCue(c)` ≡ `PlayCueAt(c, "<cue>.wav")`) ;
// repli sur `<cue>.wav` avec un WARN dédoublonné par `rel` jusqu'au prochain succès ; un chemin qui sort du dossier est
// traité comme absent. Source : contracts/ambiance-settings.md §9.4, contracts/sound.md §15 (révision 3).
// Frontière : fichiers WAV réels dans un dossier temporaire ; le contenu joué est identifié par sa LONGUEUR (chaque fichier
// a une durée distincte). Préfixe tw290p.
package audio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// tw290pRecorder : Output qui journalise la longueur (en octets) du PCM de chaque Play.
type tw290pRecorder struct {
	mu   sync.Mutex
	lens []int
}

func (r *tw290pRecorder) Play(ctx context.Context, pcm io.Reader) error {
	b, _ := io.ReadAll(pcm)
	r.mu.Lock()
	r.lens = append(r.lens, len(b))
	r.mu.Unlock()
	return nil
}
func (r *tw290pRecorder) Close() error { return nil }
func (r *tw290pRecorder) played() []int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]int(nil), r.lens...)
}

// tw290pWAV écrit un WAV canonique de `frames` trames sous dir/rel et renvoie la longueur du PCM.
func tw290pWAV(t *testing.T, dir, rel string, frames int) int {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	pcm := make([]byte, frames*FrameSize)
	for i := range pcm {
		pcm[i] = byte(i%251 + 1)
	}
	if err := os.WriteFile(path, BuildCanonicalWAV(pcm), 0o644); err != nil {
		t.Fatal(err)
	}
	return len(pcm)
}

// tw290pEngine : moteur réel (FileBank) sur un enregistreur, démarré pour la durée du test.
func tw290pEngine(t *testing.T, dir string) (*Engine, *tw290pRecorder) {
	t.Helper()
	rec := &tw290pRecorder{}
	e := NewEngine(Config{Output: rec, Bank: NewFileBank(dir)})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go e.Start(ctx)
	return e, rec
}

func tw290pWait(t *testing.T, rec *tw290pRecorder, n int) []int {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got := rec.played(); len(got) >= n {
			time.Sleep(20 * time.Millisecond)
			return rec.played()
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("%d lecture(s) attendue(s), got %v", n, rec.played())
	return nil
}

// tw290pLog capte les messages WARN du paquet (SetLogger) et restaure le défaut.
func tw290pLog(t *testing.T) func() []string {
	t.Helper()
	var mu sync.Mutex
	var warns []string
	SetLogger(func(level LogLevel, format string, args ...any) {
		if level == LogWarn {
			mu.Lock()
			warns = append(warns, fmt.Sprintf(format, args...))
			mu.Unlock()
		}
	})
	t.Cleanup(func() { SetLogger(nil) })
	return func() []string { mu.Lock(); defer mu.Unlock(); return append([]string(nil), warns...) }
}

// ---------------------------------------------------------------------------
// FileBank.PCMAt
// ---------------------------------------------------------------------------

func TestRoundFiles290_PCMAt_ReadsTheRoundFile(t *testing.T) {
	dir := t.TempDir()
	tw290pWAV(t, dir, "depart.wav", 100)
	want := tw290pWAV(t, dir, "rounds/r2/depart.wav", 250)
	pcm, ok := NewFileBank(dir).PCMAt("rounds/r2/depart.wav")
	if !ok || len(pcm) != want {
		t.Fatalf("PCMAt doit lire le fichier de manche (%d octets), got ok=%v len=%d", want, ok, len(pcm))
	}
}

func TestRoundFiles290_PCMAt_CatalogueRel_EqualsPCM(t *testing.T) {
	dir := t.TempDir()
	tw290pWAV(t, dir, "gagne.wav", 120)
	b := NewFileBank(dir)
	a, okA := b.PCMAt("gagne.wav")
	c, okC := b.PCM(Cue("gagne"))
	if !okA || !okC || !bytes.Equal(a, c) {
		t.Error("PCMAt(\"<cue>.wav\") ≡ PCM(cue) (équivalence PlayCue / PlayCueAt)")
	}
}

func TestRoundFiles290_PCMAt_MissingFile_FallsBackToCatalogue(t *testing.T) {
	dir := t.TempDir()
	want := tw290pWAV(t, dir, "depart.wav", 100)
	pcm, ok := NewFileBank(dir).PCMAt("rounds/r2/depart.wav")
	if !ok || len(pcm) != want {
		t.Errorf("fichier de manche absent ⇒ repli sur depart.wav (%d octets), got ok=%v len=%d", want, ok, len(pcm))
	}
}

func TestRoundFiles290_PCMAt_NonConformingFile_FallsBack(t *testing.T) {
	dir := t.TempDir()
	want := tw290pWAV(t, dir, "reveal.wav", 90)
	if err := os.MkdirAll(filepath.Join(dir, "rounds", "r2"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "rounds", "r2", "reveal.wav"), []byte("pas un wav"), 0o644)
	pcm, ok := NewFileBank(dir).PCMAt("rounds/r2/reveal.wav")
	if !ok || len(pcm) != want {
		t.Errorf("fichier de manche non conforme ⇒ repli sur le catalogue, got ok=%v len=%d", ok, len(pcm))
	}
}

func TestRoundFiles290_PCMAt_NothingAnywhere_NotOK(t *testing.T) {
	if pcm, ok := NewFileBank(t.TempDir()).PCMAt("rounds/r2/depart.wav"); ok || pcm != nil {
		t.Errorf("ni fichier de manche ni catalogue ⇒ ok=false (dégradation silencieuse), got ok=%v", ok)
	}
}

// Sécurité : un chemin qui sort du dossier des sons est traité comme absent — jamais lu.
func TestRoundFiles290_PCMAt_PathEscape_IsTreatedAsMissing(t *testing.T) {
	root := t.TempDir()
	sounds := filepath.Join(root, "sounds")
	secret := tw290pWAV(t, root, "secret.wav", 333) // HORS du dossier des sons
	catalogue := tw290pWAV(t, sounds, "secret.wav", 77)
	for _, rel := range []string{"../secret.wav", "rounds/../../secret.wav", "..\\secret.wav", filepath.Join(root, "secret.wav")} {
		pcm, ok := NewFileBank(sounds).PCMAt(rel)
		if ok && len(pcm) == secret {
			t.Errorf("rel %q : le fichier hors du dossier ne doit JAMAIS être lu", rel)
		}
		if ok && len(pcm) != catalogue {
			t.Errorf("rel %q : seul le repli catalogue est admis (%d octets), got %d", rel, catalogue, len(pcm))
		}
	}
}

// ---------------------------------------------------------------------------
// Engine.PlayCueAt / PlayCue
// ---------------------------------------------------------------------------

func TestRoundFiles290_PlayCueAt_PlaysTheRoundFile_PlayCuePlaysTheCatalogue(t *testing.T) {
	dir := t.TempDir()
	global := tw290pWAV(t, dir, "depart.wav", 100)
	round := tw290pWAV(t, dir, "rounds/r2/depart.wav", 250)
	e, rec := tw290pEngine(t, dir)
	if !e.PlayCueAt(Cue("depart"), "rounds/r2/depart.wav") {
		t.Fatal("PlayCueAt doit accepter (file non pleine)")
	}
	if !e.PlayCue(Cue("depart")) {
		t.Fatal("PlayCue doit accepter")
	}
	got := tw290pWait(t, rec, 2)
	if got[0] != round || got[1] != global {
		t.Errorf("FIFO attendue [fichier de manche %d, catalogue %d], got %v", round, global, got)
	}
}

func TestRoundFiles290_PlayCue_EqualsPlayCueAtCatalogueName(t *testing.T) {
	dir := t.TempDir()
	global := tw290pWAV(t, dir, "gagne.wav", 140)
	e, rec := tw290pEngine(t, dir)
	e.PlayCue(Cue("gagne"))
	e.PlayCueAt(Cue("gagne"), "gagne.wav")
	e.PlayCueAt(Cue("gagne"), "")
	got := tw290pWait(t, rec, 3)
	for i, l := range got {
		if l != global {
			t.Errorf("lecture %d : le catalogue (%d octets) attendu pour PlayCue / PlayCueAt(\"<cue>.wav\") / PlayCueAt(\"\"), got %d", i, global, l)
		}
	}
}

func TestRoundFiles290_PlayCueAt_MissingRoundFile_PlaysCatalogue_NeverSilent(t *testing.T) {
	dir := t.TempDir()
	global := tw290pWAV(t, dir, "reveal.wav", 80)
	e, rec := tw290pEngine(t, dir)
	e.PlayCueAt(Cue("reveal"), "rounds/r2/reveal.wav")
	if got := tw290pWait(t, rec, 1); got[0] != global {
		t.Errorf("repli sur le catalogue (%d octets), got %v", global, got)
	}
}

func TestRoundFiles290_PlayCueAt_OrderingAcrossCuesAndFiles_IsFIFO(t *testing.T) {
	dir := t.TempDir()
	a := tw290pWAV(t, dir, "depart.wav", 50)
	b := tw290pWAV(t, dir, "rounds/r1/gagne.wav", 60)
	c := tw290pWAV(t, dir, "reveal.wav", 70)
	tw290pWAV(t, dir, "gagne.wav", 65)
	e, rec := tw290pEngine(t, dir)
	e.PlayCue(Cue("depart"))
	e.PlayCueAt(Cue("gagne"), "rounds/r1/gagne.wav")
	e.PlayCue(Cue("reveal"))
	got := tw290pWait(t, rec, 3)
	if got[0] != a || got[1] != b || got[2] != c {
		t.Errorf("ordre FIFO non coalescé attendu [%d %d %d], got %v", a, b, c, got)
	}
}

func TestRoundFiles290_NilEngine_PlayCueAt_NoPanic(t *testing.T) {
	var e *Engine
	if e.PlayCueAt(Cue("depart"), "rounds/r1/depart.wav") {
		t.Error("un moteur nil n'accepte rien")
	}
	if NewEngine(Config{}).PlayCueAt(Cue("depart"), "x.wav") {
		t.Error("un moteur désactivé n'accepte rien (aucune goroutine, §5.5)")
	}
}

// ---------------------------------------------------------------------------
// WARN dédoublonné par rel jusqu'au prochain succès (D12 : jamais de correction silencieuse).
// ---------------------------------------------------------------------------

func TestRoundFiles290_Fallback_WarnsOncePerRel_UntilNextSuccess(t *testing.T) {
	warns := tw290pLog(t)
	dir := t.TempDir()
	tw290pWAV(t, dir, "depart.wav", 100)
	b := NewFileBank(dir)
	const rel = "rounds/r2/depart.wav"

	for i := 0; i < 5; i++ {
		b.PCMAt(rel)
	}
	count := func() int {
		n := 0
		for _, w := range warns() {
			if strings.Contains(w, rel) {
				n++
			}
		}
		return n
	}
	if n := count(); n != 1 {
		t.Fatalf("5 lectures du même fichier manquant ⇒ UN seul WARN (dédoublonné par rel), got %d : %v", n, warns())
	}

	b.PCMAt("rounds/r3/depart.wav") // un AUTRE rel manquant : son propre WARN
	if len(warns()) != 2 {
		t.Errorf("un autre rel manquant ⇒ un second WARN, got %v", warns())
	}

	tw290pWAV(t, dir, rel, 200) // le fichier réapparaît : succès, le compteur est réarmé
	if pcm, ok := b.PCMAt(rel); !ok || len(pcm) != 200*FrameSize {
		t.Fatalf("setup invalide : le fichier de manche doit être lu, ok=%v len=%d", ok, len(pcm))
	}
	_ = os.Remove(filepath.Join(dir, filepath.FromSlash(rel))) // … puis disparaît à nouveau
	b.PCMAt(rel)
	if n := count(); n != 2 {
		t.Errorf("après un succès, la prochaine absence est de nouveau signalée (WARN réarmé), got %d : %v", n, warns())
	}
}

func TestRoundFiles290_Fallback_WarnMessageNamesTheFile(t *testing.T) {
	warns := tw290pLog(t)
	dir := t.TempDir()
	tw290pWAV(t, dir, "perdu.wav", 100)
	NewFileBank(dir).PCMAt("rounds/r4/perdu.wav")
	got := warns()
	if len(got) != 1 || !strings.Contains(got[0], "rounds/r4/perdu.wav") {
		t.Errorf("le WARN doit nommer le fichier de manche manquant, got %v", got)
	}
}

// Non-régression : sans chemin de manche, aucun WARN (la lecture du catalogue ne bruite pas).
func TestRoundFiles290_PlayCue_Catalogue_NoWarn(t *testing.T) {
	warns := tw290pLog(t)
	dir := t.TempDir()
	tw290pWAV(t, dir, "depart.wav", 100)
	e, rec := tw290pEngine(t, dir)
	e.PlayCue(Cue("depart"))
	tw290pWait(t, rec, 1)
	if got := warns(); len(got) != 0 {
		t.Errorf("la lecture du catalogue ne doit produire aucun WARN, got %v", got)
	}
}
