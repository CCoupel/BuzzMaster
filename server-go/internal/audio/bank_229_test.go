// Suite test-writer pour #229 (milestone v11.0 — plan de dev §2.8) :
// FileBank (bank.go), qui remplace le paravent de #227/#228 par de vrais
// octets PCM lus depuis data/files/sounds/. Complémentaire de
// internal/audio/synth's propres tests (le générateur qui ÉCRIT les
// fichiers que FileBank lit) — ce fichier couvre la LECTURE, y compris les
// cas de dégradation silencieuse (contract §5.5) que le générateur seul ne
// peut pas produire (fichier absent, corrompu, hors format canonique).
//
// Paquet de test EXTERNE (audio_test, pas audio) : ce fichier importe
// internal/audio/synth pour comparer FileBank à de vrais fichiers générés,
// et internal/audio/synth importe déjà internal/audio (pour ses constantes
// de format) — package audio le ferait entrer en cycle d'import. Même
// contrainte que toute suite qui a besoin des deux côtés à la fois.
//
// Convention de collision : préfixe tw229b pour ne jamais entrer en
// collision avec un éventuel fichier de tests dev-backend du même paquet.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package audio_test

import (
	"os"
	"path/filepath"
	"testing"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/audio/synth"
)

// TestFileBank_ReadsRealGeneratedFile_ForEveryCue proves the two halves —
// synth.WriteAll (writer) and FileBank.PCM (reader) — actually agree on
// disk format: every cue's generated file, once written by the REAL
// generator, must be readable back through the REAL bank, and the PCM
// bytes returned must exactly equal what synth.Generate produced minus its
// 44-byte header (WriteWAV/extractCanonicalPCM's own contract).
func TestFileBank_ReadsRealGeneratedFile_ForEveryCue(t *testing.T) {
	dir := t.TempDir()
	if _, err := synth.WriteAll(dir, false); err != nil {
		t.Fatalf("setup invalide : synth.WriteAll a échoué : %v", err)
	}
	bank := audio.NewFileBank(dir)

	for _, c := range synth.Cues {
		t.Run(string(c), func(t *testing.T) {
			wav, ok := synth.Generate(c)
			if !ok {
				t.Fatalf("setup invalide : synth.Generate(%s) a renvoyé ok=false", c)
			}
			wantPCM := wav[44:] // même convention d'en-tête que synth.WriteWAV

			gotPCM, ok := bank.PCM(c)
			if !ok {
				t.Fatalf("FileBank.PCM(%s) a renvoyé ok=false pour un fichier réellement généré", c)
			}
			if len(gotPCM) != len(wantPCM) {
				t.Fatalf("%s : PCM lu (%d octets) ne correspond pas en taille au PCM généré (%d octets)", c, len(gotPCM), len(wantPCM))
			}
			for i := range wantPCM {
				if gotPCM[i] != wantPCM[i] {
					t.Fatalf("%s : PCM lu diffère du PCM généré à l'octet %d", c, i)
					break
				}
			}
		})
	}
}

// TestFileBank_MissingFile_DegradesSilently is contract §5.5's "dégradation
// silencieuse" applied to la banque : un cue jamais généré/téléversé ne
// doit produire aucune erreur, seulement ok=false.
func TestFileBank_MissingFile_DegradesSilently(t *testing.T) {
	bank := audio.NewFileBank(t.TempDir()) // répertoire vide : aucun fichier
	data, ok := bank.PCM(audio.CueDepart)
	if ok {
		t.Fatal("PCM sur un répertoire vide doit renvoyer ok=false")
	}
	if data != nil {
		t.Errorf("data doit être nil quand ok=false, got %v", data)
	}
}

// TestFileBank_CorruptFile_DegradesSilently proves a file that exists but
// is not a valid RIFF/WAVE stream at all degrades the same way as a
// missing one — never a panic, never a hard error surfaced to the engine.
func TestFileBank_CorruptFile_DegradesSilently(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "depart.wav"), []byte("ceci n'est pas un fichier WAV"), 0644); err != nil {
		t.Fatalf("setup invalide : %v", err)
	}
	bank := audio.NewFileBank(dir)
	if _, ok := bank.PCM(audio.CueDepart); ok {
		t.Fatal("un fichier corrompu (pas un RIFF/WAVE valide) doit dégrader silencieusement, ok=false")
	}
}

// TestFileBank_NonCanonicalFormat_Refused proves a STRUCTURALLY valid WAV
// (real RIFF/fmt/data chunks) that simply uses a different sample rate/
// channel count/bit depth than the canonical format (contract §3) is
// REFUSED, never resampled or reinterpreted — exactly extractCanonicalPCM's
// own documented contract ("any other value is REFUSED, never resampled").
func TestFileBank_NonCanonicalFormat_Refused(t *testing.T) {
	dir := t.TempDir()
	// En-tête WAV minimal fait main : mono au lieu de stéréo (hors format
	// canonique), 100 octets de "data" arbitraires.
	wav := tw229bBuildWAVHeader(t, 1 /* mono, non conforme */, audio.SampleRate, audio.BitsPerSample, make([]byte, 100))
	if err := os.WriteFile(filepath.Join(dir, "depart.wav"), wav, 0644); err != nil {
		t.Fatalf("setup invalide : %v", err)
	}
	bank := audio.NewFileBank(dir)
	if _, ok := bank.PCM(audio.CueDepart); ok {
		t.Fatal("un WAV structurellement valide mais hors format canonique (mono ici) doit être refusé, jamais ré-échantillonné ni réinterprété (contract §3)")
	}
}

// TestFileBank_DiskIsAuthoritative_NoCaching proves "le disque fait foi"
// (plan de cadrage §2.2) literally: replacing a cue's file between two PCM
// calls is picked up immediately, with no restart and no explicit
// invalidation — bank.go's own doc comment states this explicitly; this
// test verifies it rather than trusting the comment.
func TestFileBank_DiskIsAuthoritative_NoCaching(t *testing.T) {
	dir := t.TempDir()
	first := tw229bBuildWAVHeader(t, audio.ChannelCount, audio.SampleRate, audio.BitsPerSample, []byte{0x01, 0x02, 0x03, 0x04})
	if err := os.WriteFile(filepath.Join(dir, "depart.wav"), first, 0644); err != nil {
		t.Fatalf("setup invalide : %v", err)
	}
	bank := audio.NewFileBank(dir)

	got1, ok := bank.PCM(audio.CueDepart)
	if !ok || len(got1) != 4 || got1[0] != 0x01 {
		t.Fatalf("premier PCM inattendu : ok=%v, got=%v", ok, got1)
	}

	second := tw229bBuildWAVHeader(t, audio.ChannelCount, audio.SampleRate, audio.BitsPerSample, []byte{0xAA, 0xBB, 0xCC, 0xDD})
	if err := os.WriteFile(filepath.Join(dir, "depart.wav"), second, 0644); err != nil {
		t.Fatalf("setup invalide (remplacement) : %v", err)
	}

	got2, ok := bank.PCM(audio.CueDepart)
	if !ok || len(got2) != 4 || got2[0] != 0xAA {
		t.Fatalf("le remplacement du fichier sur disque n'a pas été pris en compte immédiatement (cache suspecté) : ok=%v, got=%v", ok, got2)
	}
}

// tw229bBuildWAVHeader hand-builds a minimal, structurally valid RIFF/WAVE
// byte stream with the given fmt-chunk fields and data payload — used to
// exercise extractCanonicalPCM's format-validation branch independently of
// synth.WriteWAV (which always writes the canonical format by
// construction, so it alone could never produce a non-canonical fixture).
func tw229bBuildWAVHeader(t *testing.T, channels, sampleRate, bitsPerSample int, data []byte) []byte {
	t.Helper()
	byteRate := sampleRate * channels * (bitsPerSample / 8)
	blockAlign := channels * (bitsPerSample / 8)

	buf := make([]byte, 44+len(data))
	copy(buf[0:4], "RIFF")
	tw229bPutU32(buf[4:8], uint32(36+len(data)))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	tw229bPutU32(buf[16:20], 16)
	tw229bPutU16(buf[20:22], 1)
	tw229bPutU16(buf[22:24], uint16(channels))
	tw229bPutU32(buf[24:28], uint32(sampleRate))
	tw229bPutU32(buf[28:32], uint32(byteRate))
	tw229bPutU16(buf[32:34], uint16(blockAlign))
	tw229bPutU16(buf[34:36], uint16(bitsPerSample))
	copy(buf[36:40], "data")
	tw229bPutU32(buf[40:44], uint32(len(data)))
	copy(buf[44:], data)
	return buf
}

func tw229bPutU32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func tw229bPutU16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}
