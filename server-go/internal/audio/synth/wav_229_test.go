// Suite test-writer pour #229 (milestone v11.0 — plan de dev §2.8,
// _work/reports/plan-dev-228-229-20260921-114500.md Partie 2) : conformité
// de l'en-tête WAV canonique produit par WriteWAV (contracts/sound.md §3 —
// PCM 16 bits, 44100 Hz, stéréo).
//
// Convention de collision : préfixe tw229 pour ne jamais entrer en
// collision avec un éventuel fichier de tests dev-backend du même paquet.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package synth

import (
	"bytes"
	"encoding/binary"
	"testing"

	"buzzcontrol/internal/audio"
)

// tw229ParseWAVHeader decodes the fixed 44-byte canonical header this
// package writes, failing the test immediately on any structural surprise
// (wrong size, wrong magic markers) rather than silently reading garbage.
type tw229WAVHeader struct {
	ChunkSize     uint32
	NumChannels   uint16
	SampleRate    uint32
	ByteRate      uint32
	BlockAlign    uint16
	BitsPerSample uint16
	DataSize      uint32
}

func tw229ParseWAVHeader(t *testing.T, wav []byte) tw229WAVHeader {
	t.Helper()
	if len(wav) < 44 {
		t.Fatalf("fichier WAV trop court pour contenir l'en-tête canonique : %d octets", len(wav))
	}
	if string(wav[0:4]) != "RIFF" {
		t.Fatalf("marqueur RIFF absent, got %q", wav[0:4])
	}
	if string(wav[8:12]) != "WAVE" {
		t.Fatalf("marqueur WAVE absent, got %q", wav[8:12])
	}
	if string(wav[12:16]) != "fmt " {
		t.Fatalf("marqueur 'fmt ' absent, got %q", wav[12:16])
	}
	if string(wav[36:40]) != "data" {
		t.Fatalf("marqueur 'data' absent, got %q", wav[36:40])
	}
	audioFormat := binary.LittleEndian.Uint16(wav[20:22])
	if audioFormat != 1 {
		t.Fatalf("audio format = %d, attendu 1 (PCM non compressé)", audioFormat)
	}
	fmtChunkSize := binary.LittleEndian.Uint32(wav[16:20])
	if fmtChunkSize != 16 {
		t.Fatalf("taille du chunk fmt = %d, attendu 16 (PCM sans extension)", fmtChunkSize)
	}
	return tw229WAVHeader{
		ChunkSize:     binary.LittleEndian.Uint32(wav[4:8]),
		NumChannels:   binary.LittleEndian.Uint16(wav[22:24]),
		SampleRate:    binary.LittleEndian.Uint32(wav[24:28]),
		ByteRate:      binary.LittleEndian.Uint32(wav[28:32]),
		BlockAlign:    binary.LittleEndian.Uint16(wav[32:34]),
		BitsPerSample: binary.LittleEndian.Uint16(wav[34:36]),
		DataSize:      binary.LittleEndian.Uint32(wav[40:44]),
	}
}

// tw229SilentPCM returns n frames (audio.ChannelCount samples each) of
// all-zero PCM — sufficient payload for header-shape tests, which don't
// care about waveform content.
func tw229SilentPCM(frames int) []byte {
	return make([]byte, frames*audio.FrameSize)
}

// TestWriteWAV_HeaderMatchesCanonicalFormat is #229's §2.8 point 1 — en-tête
// WAV conforme au format canonique (contracts/sound.md §3).
func TestWriteWAV_HeaderMatchesCanonicalFormat(t *testing.T) {
	pcm := tw229SilentPCM(100)
	wav := WriteWAV(pcm)
	h := tw229ParseWAVHeader(t, wav)

	if h.NumChannels != audio.ChannelCount {
		t.Errorf("NumChannels = %d, attendu %d (contract §3)", h.NumChannels, audio.ChannelCount)
	}
	if h.SampleRate != audio.SampleRate {
		t.Errorf("SampleRate = %d, attendu %d (contract §3)", h.SampleRate, audio.SampleRate)
	}
	if h.BitsPerSample != audio.BitsPerSample {
		t.Errorf("BitsPerSample = %d, attendu %d (contract §3)", h.BitsPerSample, audio.BitsPerSample)
	}
	wantByteRate := uint32(audio.SampleRate * audio.ChannelCount * (audio.BitsPerSample / 8))
	if h.ByteRate != wantByteRate {
		t.Errorf("ByteRate = %d, attendu %d (SampleRate × ChannelCount × BitsPerSample/8)", h.ByteRate, wantByteRate)
	}
	wantBlockAlign := uint16(audio.ChannelCount * (audio.BitsPerSample / 8))
	if h.BlockAlign != wantBlockAlign {
		t.Errorf("BlockAlign = %d, attendu %d (ChannelCount × BitsPerSample/8)", h.BlockAlign, wantBlockAlign)
	}
}

// TestWriteWAV_SizeFieldsMatchActualPayload proves the RIFF chunk size and
// data chunk size fields are DERIVED from len(pcm), never a fixed or
// hardcoded value — a mismatch here is exactly the class of defect that
// makes a file look valid but fail to play, or play truncated/corrupted.
func TestWriteWAV_SizeFieldsMatchActualPayload(t *testing.T) {
	for _, frames := range []int{0, 1, 100, 44099} { // 0 = degenerate edge case
		pcm := tw229SilentPCM(frames)
		wav := WriteWAV(pcm)
		h := tw229ParseWAVHeader(t, wav)

		if int(h.DataSize) != len(pcm) {
			t.Errorf("frames=%d: DataSize = %d, attendu %d (len(pcm))", frames, h.DataSize, len(pcm))
		}
		wantChunkSize := uint32(36 + len(pcm))
		if h.ChunkSize != wantChunkSize {
			t.Errorf("frames=%d: ChunkSize (RIFF) = %d, attendu %d (36 + len(pcm))", frames, h.ChunkSize, wantChunkSize)
		}
		if len(wav) != 44+len(pcm) {
			t.Errorf("frames=%d: longueur totale du fichier = %d, attendu %d (en-tête 44 + payload)", frames, len(wav), 44+len(pcm))
		}
		if !bytes.Equal(wav[44:], pcm) {
			t.Errorf("frames=%d: le payload après l'en-tête ne correspond pas exactement au pcm fourni", frames)
		}
	}
}

// TestWriteWAV_Deterministic proves two calls with the SAME input produce
// BYTE-IDENTICAL output — the narrow, low-level twin of the full-generator
// determinism test (cues_229_test.go once the seven timbres exist):
// contract §229 §2.1 requires this at every layer, not just end to end, so
// a future regression is caught at the smallest reproducible unit.
func TestWriteWAV_Deterministic(t *testing.T) {
	pcm := tw229SilentPCM(500)
	a := WriteWAV(pcm)
	b := WriteWAV(pcm)
	if !bytes.Equal(a, b) {
		t.Fatal("deux appels à WriteWAV avec le même pcm ont produit des octets différents — contract §229 §2.1 : le générateur doit être déterministe, sans aucun aléa ni horodatage")
	}
}

// TestWriteWAV_EmptyPCM_StillProducesAValidHeader is a boundary case: an
// empty payload must still produce a structurally valid (if silent, 0
// bytes of data) WAV file, never panic.
func TestWriteWAV_EmptyPCM_StillProducesAValidHeader(t *testing.T) {
	wav := WriteWAV(nil)
	h := tw229ParseWAVHeader(t, wav)
	if h.DataSize != 0 {
		t.Errorf("DataSize = %d pour un pcm vide, attendu 0", h.DataSize)
	}
	if len(wav) != 44 {
		t.Errorf("longueur totale = %d pour un pcm vide, attendu 44 (en-tête seul)", len(wav))
	}
}
