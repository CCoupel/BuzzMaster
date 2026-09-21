// Suite test-writer pour #229 (milestone v11.0 — plan de dev §2.8) : le
// générateur des 7 timbres par défaut (Generate/Cues, cues.go).
//
// Convention de collision : préfixe tw229 (partagé avec wav_229_test.go,
// même fichier d'aides n'existe pas — chaque assertion redéfinit ce dont
// elle a besoin localement pour rester lisible seule).
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package synth

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"buzzcontrol/internal/audio"
)

// tw229Envelope reads the 16-bit signed samples at the very start and very
// end of a WAV's data section (skipping the 44-byte header) — used to
// verify the attack/release envelope (plan §2.3: "amplitude nulle aux deux
// extrémités").
func tw229FirstAndLastSample(t *testing.T, wav []byte) (first, last int16) {
	t.Helper()
	if len(wav) < 44+4 {
		t.Fatalf("fichier WAV trop court pour contenir au moins 2 échantillons de données : %d octets", len(wav))
	}
	data := wav[44:]
	first = int16(binary.LittleEndian.Uint16(data[0:2]))
	last = int16(binary.LittleEndian.Uint16(data[len(data)-2:]))
	return first, last
}

// TestGenerate_EveryClosedCatalogueCueProducesAValidWAV is §2.8 point 1's
// "en-tête WAV conforme" applied to EVERY cue of the closed v11.0
// catalogue, not just an arbitrary sample — a defect specific to one
// timbre (e.g. a duration computation only wrong for a 2-note sequence)
// would otherwise slip through.
func TestGenerate_EveryClosedCatalogueCueProducesAValidWAV(t *testing.T) {
	if len(Cues) != 7 {
		t.Fatalf("Cues doit lister exactement les 7 cues du catalogue fermé v11.0 (contract §2.1), got %d: %v", len(Cues), Cues)
	}
	for _, c := range Cues {
		t.Run(string(c), func(t *testing.T) {
			wav, ok := Generate(c)
			if !ok {
				t.Fatalf("Generate(%s) a renvoyé ok=false — chaque cue du catalogue fermé doit avoir un timbre", c)
			}
			h := tw229ParseWAVHeader(t, wav)
			if h.NumChannels != audio.ChannelCount || h.SampleRate != audio.SampleRate || h.BitsPerSample != audio.BitsPerSample {
				t.Errorf("Generate(%s) : en-tête non conforme au format canonique — %+v", c, h)
			}
		})
	}
}

// TestGenerate_UnknownCue_ReturnsNotOK proves the catalogue stays CLOSED
// (contract §2.1) at the generator level too: an arbitrary Cue value
// outside the 7 known constants must be refused explicitly, never silently
// synthesise something.
func TestGenerate_UnknownCue_ReturnsNotOK(t *testing.T) {
	if _, ok := Generate(audio.Cue("not-a-real-cue")); ok {
		t.Fatal("Generate a accepté une cue hors catalogue fermé — attendu ok=false (contract §2.1)")
	}
}

// TestGenerate_Deterministic_AllCues is §2.8 point 1's exigence de
// déterminisme strict, à l'échelle du GÉNÉRATEUR COMPLET (le paravent de
// clic — WriteWAV — est déjà couvert isolément par
// TestWriteWAV_Deterministic ; ce test-ci couvre en plus la synthèse de
// forme d'onde elle-même, pour chacun des 7 timbres).
func TestGenerate_Deterministic_AllCues(t *testing.T) {
	for _, c := range Cues {
		t.Run(string(c), func(t *testing.T) {
			a, okA := Generate(c)
			b, okB := Generate(c)
			if !okA || !okB {
				t.Fatalf("setup invalide : Generate(%s) a renvoyé ok=false", c)
			}
			if !bytes.Equal(a, b) {
				t.Fatalf("deux générations de %s ont produit des octets différents — contract §229 §2.1 : aucun aléa, aucun horodatage autorisé", c)
			}
		})
	}
}

// TestGenerate_EnvelopePresent_AllCues is §2.8 point 1's "enveloppe
// présente (amplitude nulle aux deux extrémités)" — plan §2.3: "une onde
// qui démarre ou s'arrête à amplitude non nulle produit un clic audible."
// The FIRST sample must be exactly 0 for every cue (attack always ramps
// from silence, by construction in renderNote/decayingTone). The LAST
// sample is checked with a small tolerance: decayingTone's release is a
// continuous decay reaching (frames-1-last)/(frames-1) = 0 exactly at the
// final frame algebraically, but quantisation to int16 could in principle
// leave an off-by-one rounding artifact — this test catches a REAL
// regression (a forgotten envelope entirely, amplitude in the thousands)
// without being flaky over exact rounding.
func TestGenerate_EnvelopePresent_AllCues(t *testing.T) {
	const tolerance = 4 // int16 quantisation slack, nowhere close to an audible click
	for _, c := range Cues {
		t.Run(string(c), func(t *testing.T) {
			wav, ok := Generate(c)
			if !ok {
				t.Fatalf("setup invalide : Generate(%s) a renvoyé ok=false", c)
			}
			first, last := tw229FirstAndLastSample(t, wav)
			if first != 0 {
				t.Errorf("%s : premier échantillon = %d, attendu 0 (attaque doit partir du silence, sans quoi un clic est audible)", c, first)
			}
			if last < -tolerance || last > tolerance {
				t.Errorf("%s : dernier échantillon = %d, attendu proche de 0 (extinction doit revenir au silence, tolérance ±%d) — clic audible probable", c, last, tolerance)
			}
		})
	}
}

// TestGenerate_DurationUnderTarget_AllCues is §2.8 point 1's "durée sous la
// cible (<1s)" — plan §2.3: "sons courts — cible : moins d'une seconde,"
// une contrainte d'architecture (#228's Play bloque, la lecture est
// strictement séquentielle), pas une préférence esthétique.
func TestGenerate_DurationUnderTarget_AllCues(t *testing.T) {
	const target = 1 * time.Second
	for _, c := range Cues {
		t.Run(string(c), func(t *testing.T) {
			wav, ok := Generate(c)
			if !ok {
				t.Fatalf("setup invalide : Generate(%s) a renvoyé ok=false", c)
			}
			h := tw229ParseWAVHeader(t, wav)
			frames := int(h.DataSize) / int(h.BlockAlign)
			duration := time.Duration(float64(frames) / float64(h.SampleRate) * float64(time.Second))
			if duration >= target {
				t.Errorf("%s dure %s, attendu strictement sous %s (plan §2.3, contrainte d'architecture #228)", c, duration, target)
			}
			if duration <= 0 {
				t.Errorf("%s : durée calculée nulle ou négative (%s) — setup invalide ou générateur cassé", c, duration)
			}
		})
	}
}

// TestFilename_MatchesCueNamePlusExtension pins the naming convention
// (bank.go/disk.go/cmd/server's manifest reconciliation all depend on this
// staying in lockstep — Filename is their single source of truth,
// disk.go's own doc comment).
func TestFilename_MatchesCueNamePlusExtension(t *testing.T) {
	for _, c := range Cues {
		want := string(c) + ".wav"
		if got := Filename(c); got != want {
			t.Errorf("Filename(%s) = %q, attendu %q", c, got, want)
		}
	}
}
