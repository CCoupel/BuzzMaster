// Suite test-writer pour #228 (milestone v11.0 — plan de dev §1.6) :
// initialisation et dégradation du pilote réel (`NewOutput`,
// internal/audio/output.go/output_oto.go/output_other.go).
//
// Contrainte de conception assumée ici : `oto.Context` est un type concret
// de la bibliothèque, pas une interface — `otoOutput` (output_oto.go) ne
// l'enveloppe donc derrière AUCUN seam injectable (décision de dev-backend,
// confirmée en coordination directe : la protection "un seul contexte par
// processus" est déjà assurée par la bibliothèque elle-même, qui refuse
// tout second appel et fait dégrader `newOtoOutput` par le même chemin
// d'erreur qu'un échec matériel ordinaire — contract §5.5 s'applique à la
// construction, pas seulement à `Play`). Ces tests appellent donc `NewOutput`
// EN VRAI, sans double — ce qui est possible PARTOUT, y compris sur une
// machine de CI sans aucun périphérique audio, précisément PARCE QUE la
// dégradation silencieuse est la garantie que #228 doit tenir. Sur une
// machine de CI/sandbox typique (confirmé empiriquement ici : PulseAudio/
// ALSA absents), `NewOutput` dégrade réellement vers `noopOutput` — ce
// fichier vérifie alors les invariants de CETTE dégradation. Sur une
// machine avec un vrai périphérique, `NewOutput` réussirait à la place —
// les mêmes invariants (jamais nil, jamais de blocage indéfini, jamais de
// panique, lecteur toujours drainé, Close idempotent) doivent tenir dans
// les DEUX cas, donc aucun test ci-dessous ne suppose lequel des deux se
// produit.
//
// Convention de collision : préfixe tw228o pour ne jamais entrer en
// collision avec un helper d'un autre fichier de ce paquet.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package audio

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"time"
)

// tw228oGenerousTimeout bounds every construction/Play call in this file —
// generous relative to otoOutput's own internal bounds (otoContextReadyTimeout
// = 5s, otoPlayMaxWait = 10s, output_oto.go) so a real, working device is
// never mistaken for a hang, while still catching an actual deadlock.
const tw228oGenerousTimeout = 15 * time.Second

// tw228oSilence returns n bytes of digital silence, frame-aligned — used as
// Play's payload in these tests so that on a machine with REAL working
// audio hardware, running this suite produces no audible click (same
// technique as otoOutput.prime's own pre-arming silence, output_oto.go).
func tw228oSilence(frames int) []byte {
	return make([]byte, frames*FrameSize)
}

// ---------------------------------------------------------------------------
// Initialisation (plan §1.6, point 2) : jamais nil, jamais de blocage
// indéfini — que la construction réussisse (vrai matériel) ou dégrade
// (aucun matériel, contract §5.5).
// ---------------------------------------------------------------------------

func TestNewOutput_NeverNilAndReturnsWithinBoundedTime(t *testing.T) {
	done := make(chan Output, 1)
	go func() { done <- NewOutput(OutputConfig{}) }()

	select {
	case out := <-done:
		if out == nil {
			t.Fatal("NewOutput a renvoyé nil — contract §5.5 : dégradation silencieuse, jamais un Output absent")
		}
		t.Logf("NewOutput() -> %T (succès matériel réel ou dégradation silencieuse, les deux sont valides ici)", out)
		_ = out.Close()
	case <-time.After(tw228oGenerousTimeout):
		t.Fatalf("NewOutput() n'a pas retourné après %s — le canal de disponibilité du contexte doit être BORNÉ (contract §5.5, plan #228 risque R.2), jamais attendu indéfiniment", tw228oGenerousTimeout)
	}
}

// TestNewOutput_ConcurrentConstruction_NoPanicNoDeadlock is plan §1.6 point
// 2's "même sous test concurrent" : N constructions simultanées ne doivent
// ni paniquer ni se bloquer. La bibliothèque sous-jacente refuse un second
// contexte par processus (plan Partie 1.2) — la garantie attendue ici n'est
// pas "un seul appel physique à NewContext réussit", c'est que CHAQUE appel
// concurrent se termine proprement (succès ou dégradation), sans jamais
// faire s'effondrer le processus ni geler un appelant.
func TestNewOutput_ConcurrentConstruction_NoPanicNoDeadlock(t *testing.T) {
	const n = 5
	results := make(chan Output, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("NewOutput a paniqué sous construction concurrente : %v", r)
					results <- nil
					return
				}
			}()
			results <- NewOutput(OutputConfig{})
		}()
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(tw228oGenerousTimeout):
		t.Fatalf("au moins une construction concurrente ne s'est pas terminée après %s — deadlock probable", tw228oGenerousTimeout)
	}
	close(results)

	got := 0
	for out := range results {
		got++
		if out == nil {
			continue // déjà rapporté par le recover ci-dessus, si c'est la cause
		}
		_ = out.Close()
	}
	if got != n {
		t.Fatalf("attendu %d résultats, got %d", n, got)
	}
}

// TestNewOutput_DeviceConfigured_StillReturnsUsableOutput proves
// OutputConfig.Device (reserved, not yet honoured — output.go's own doc
// comment) never turns construction into a hard failure — only a log line,
// per contracts/sound.md §9 and output_oto.go's own comment on newOtoOutput.
func TestNewOutput_DeviceConfigured_StillReturnsUsableOutput(t *testing.T) {
	out := NewOutput(OutputConfig{Device: "some-nonexistent-sink"})
	if out == nil {
		t.Fatal("un Device non honoré ne doit jamais faire échouer la construction — seulement être journalisé (contract §9)")
	}
	defer out.Close()
}

// ---------------------------------------------------------------------------
// Dégradation (plan §1.6, point 3) : Play ne doit jamais bloquer le jeu ni
// remonter une erreur inattendue, que la sortie soit réelle ou dégradée.
// ---------------------------------------------------------------------------

// tw228oCountingReader wraps a reader and records whether it was read to
// EOF — proves Play (real or degraded) is a well-behaved io.Reader
// consumer (noopOutput's own doc comment: "Draining pcm costs nothing and
// keeps this a well-behaved io.Reader consumer, matching what a real
// Output would do before playing").
type tw228oCountingReader struct {
	r        io.Reader
	mu       sync.Mutex
	readToEOF bool
}

func (c *tw228oCountingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if err == io.EOF {
		c.mu.Lock()
		c.readToEOF = true
		c.mu.Unlock()
	}
	return n, err
}

func (c *tw228oCountingReader) drained() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.readToEOF
}

func TestNewOutput_Play_DrainsReaderAndReturnsWithinBoundedTime(t *testing.T) {
	out := NewOutput(OutputConfig{})
	if out == nil {
		t.Fatal("setup invalide : NewOutput a renvoyé nil")
	}
	defer out.Close()

	// ~50ms de silence — assez court pour ne jamais gêner sur une machine
	// avec un vrai périphérique, assez long pour exercer un vrai backend
	// s'il existe.
	payload := tw228oSilence(int(0.05 * float64(SampleRate)))
	reader := &tw228oCountingReader{r: bytes.NewReader(payload)}

	ctx, cancel := context.WithTimeout(context.Background(), tw228oGenerousTimeout)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- out.Play(ctx, reader) }()

	select {
	case err := <-done:
		// Aucune erreur n'est ATTENDUE ici (silence valide, contexte non
		// annulé) — mais même une erreur de dégradation ne doit jamais
		// paniquer ni remonter comme un panic ; on la journalise seulement.
		if err != nil {
			t.Logf("Play a retourné une erreur (acceptable en dégradation, jamais fatal côté jeu) : %v", err)
		}
	case <-time.After(tw228oGenerousTimeout):
		t.Fatalf("Play() n'a pas retourné après %s — contract §5.5/§5.3 : ne doit jamais bloquer indéfiniment, même en dégradation", tw228oGenerousTimeout)
	}
	if !reader.drained() {
		t.Error("Play n'a pas lu le pcm jusqu'à EOF — doit rester un consommateur io.Reader bien élevé (même contrat que noopOutput, output_other.go)")
	}
}

// TestNewOutput_Close_Idempotent proves Close never errors nor panics when
// called more than once — contract §4: "Close releases resources.
// Idempotent, callable even if Play never succeeded."
func TestNewOutput_Close_Idempotent(t *testing.T) {
	out := NewOutput(OutputConfig{})
	if out == nil {
		t.Fatal("setup invalide : NewOutput a renvoyé nil")
	}
	for i := 0; i < 3; i++ {
		if err := out.Close(); err != nil {
			t.Errorf("Close() #%d a renvoyé une erreur, attendu nil (idempotent) : %v", i, err)
		}
	}
}
