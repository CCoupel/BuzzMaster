//go:build linux || windows

// Instrumentation ciblée pour le retour QUALIF v11.1.0.4 (#219) : « le
// fichier mono est accepté à l'upload mais aucun son n'est audible en
// jeu ». Le round-trip de TestValidateQuestionSound_AcceptsMono_
// UpmixedToStereo (validate_media_219_test.go) ne prouve la validité du
// fichier stocké qu'à travers extractCanonicalPCM (bank.go) — le parseur
// des CUES, jamais celui réellement emprunté à la LECTURE d'un média de
// question. Ce fichier exerce le VRAI parseur de lecture,
// wavDataSection (media_oto.go, package-privé — d'où le build tag
// linux||windows, identique à oto_singleton_219_test.go), sur un fichier
// RÉELLEMENT ÉCRIT SUR DISQUE par BuildCanonicalWAV à partir d'un mono
// suréchantillonné — le chemin complet upload→stockage→lecture, pas
// seulement la validation.
//
// Convention de collision : préfixe tw219p pour ne jamais entrer en
// collision avec un helper d'un autre fichier de ce paquet.
package audio

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// tw219pBuildMonoWAV construit un WAV mono RIFF/WAVE valide et non
// silencieux (motif d'échantillons croissants, jamais des zéros — un
// bug d'alignement serait invisible sur du silence pur) pour la durée
// donnée.
func tw219pBuildMonoWAV(t *testing.T, seconds float64) []byte {
	t.Helper()
	frames := int(seconds * float64(SampleRate))
	data := make([]byte, frames*BytesPerSample)
	for i := 0; i < frames; i++ {
		v := uint16(i % 30000)
		data[i*2] = byte(v)
		data[i*2+1] = byte(v >> 8)
	}

	buf := make([]byte, 44+len(data))
	copy(buf[0:4], "RIFF")
	tw219pPutU32(buf[4:8], uint32(36+len(data)))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	tw219pPutU32(buf[16:20], 16)
	tw219pPutU16(buf[20:22], 1) // PCM
	tw219pPutU16(buf[22:24], 1) // MONO
	tw219pPutU32(buf[24:28], uint32(SampleRate))
	tw219pPutU32(buf[28:32], uint32(SampleRate*1*BytesPerSample)) // ByteRate MONO
	tw219pPutU16(buf[32:34], uint16(1*BytesPerSample))            // BlockAlign MONO
	tw219pPutU16(buf[34:36], uint16(BitsPerSample))
	copy(buf[36:40], "data")
	tw219pPutU32(buf[40:44], uint32(len(data)))
	copy(buf[44:], data)
	return buf
}

func tw219pPutU32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func tw219pPutU16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

// TestMonoUploadPipeline_WavDataSectionLocatesCorrectDataChunk exerce le
// chemin COMPLET upload→stockage→lecture pour un WAV mono, avec le VRAI
// parseur de lecture (wavDataSection), pas extractCanonicalPCM (retour
// QUALIF v11.1.0.4, #219) :
//  1. ValidateQuestionSound (le validateur d'upload réel) sur un mono.
//  2. BuildCanonicalWAV (ce qu'internal/server/http.go écrit sur disque)
//     sur le PCM validé.
//  3. Écriture RÉELLE sur disque, dans un fichier temporaire.
//  4. wavDataSection (media_oto.go) OUVRE ce fichier exactement comme
//     otoMediaPlayer.Play le fait, et doit localiser un offset/une
//     taille qui, une fois lus via io.NewSectionReader, reproduisent
//     EXACTEMENT le PCM stéréo validé à l'étape 1 — pas un octet de
//     plus, pas un octet de moins, pas décalé.
func TestMonoUploadPipeline_WavDataSectionLocatesCorrectDataChunk(t *testing.T) {
	monoWAV := tw219pBuildMonoWAV(t, 0.5) // 0.5s — court, largement sous 30s/6Mio

	result, err := ValidateQuestionSound(monoWAV)
	if err != nil {
		t.Fatalf("ValidateQuestionSound a refusé un mono valide : %v", err)
	}

	stored := BuildCanonicalWAV(result.PCM)

	dir := t.TempDir()
	path := filepath.Join(dir, "sound_mono.wav")
	if err := os.WriteFile(path, stored, 0644); err != nil {
		t.Fatalf("setup invalide : écriture du fichier a échoué : %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("setup invalide : ouverture du fichier a échoué : %v", err)
	}
	defer f.Close()

	offset, size, err := wavDataSection(f)
	if err != nil {
		t.Fatalf("wavDataSection (le VRAI parseur de lecture, media_oto.go) refuse le fichier écrit par BuildCanonicalWAV à partir d'un mono suréchantillonné : %v — c'est exactement la cause possible du retour QUALIF v11.1.0.4 (\"mono accepté mais silencieux\")", err)
	}
	if size != int64(len(result.PCM)) {
		t.Fatalf("wavDataSection a localisé une taille de %d octets, attendu %d (len(result.PCM)) — le lecteur lirait une plage tronquée ou surdimensionnée", size, len(result.PCM))
	}

	section := io.NewSectionReader(f, offset, size)
	replayed := make([]byte, size)
	if _, err := io.ReadFull(section, replayed); err != nil {
		t.Fatalf("lecture de la section localisée par wavDataSection a échoué : %v", err)
	}
	for i := range result.PCM {
		if replayed[i] != result.PCM[i] {
			t.Fatalf("octet %d diffère : lu %#02x, attendu %#02x (PCM validé) — le fichier stocké et ce que wavDataSection localise ne correspondent pas", i, replayed[i], result.PCM[i])
		}
	}
}

// TestMonoUploadPipeline_RealPlaybackIfHardwareAvailable va plus loin que
// la localisation d'octets : si cette machine dispose d'un backend audio
// RÉEL (voir media_219_test.go's même discipline gated sur
// IsNeutralMedia), elle fait effectivement JOUER le fichier mono
// upmixé via le VRAI MediaPlayer (Play/State), jusqu'à MediaIdle — la
// preuve la plus proche d'« audible » qu'un test automatisé puisse
// apporter sans oreille humaine ni matériel garanti en CI.
func TestMonoUploadPipeline_RealPlaybackIfHardwareAvailable(t *testing.T) {
	monoWAV := tw219pBuildMonoWAV(t, 0.3)
	result, err := ValidateQuestionSound(monoWAV)
	if err != nil {
		t.Fatalf("setup invalide : %v", err)
	}
	stored := BuildCanonicalWAV(result.PCM)
	dir := t.TempDir()
	path := filepath.Join(dir, "sound_mono.wav")
	if err := os.WriteFile(path, stored, 0644); err != nil {
		t.Fatalf("setup invalide : %v", err)
	}

	mp := NewMediaPlayer(OutputConfig{}, nil)
	if mp == nil {
		t.Fatal("setup invalide : NewMediaPlayer a renvoyé nil")
	}
	defer mp.Close()
	if IsNeutralMedia(mp) {
		t.Skip("aucun backend audio réel disponible ici (dégradation neutre, contract §5.5) — la localisation d'octets est couverte par TestMonoUploadPipeline_WavDataSectionLocatesCorrectDataChunk")
	}

	if err := mp.Play(path); err != nil {
		t.Fatalf("Play() a échoué sur un backend réel pour le fichier mono upmixé : %v", err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if mp.State() == MediaIdle {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("la lecture du mono upmixé n'a jamais atteint MediaIdle après 15s, état final %q", mp.State())
}
