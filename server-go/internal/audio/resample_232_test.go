// #232 (v11.6.0, LOT-1) — conversion à l'upload de tout WAV PCM vers le format canonique.
// Source normative : contracts/sound.md §3 « Amendement v11.6.0 (#232) » + contracts/http-endpoints.md
// « Uploads sonores — conversion automatique ». Écrit depuis le contrat, avant le code : rouge (ToCanonical
// n'existe pas) tant que dev-backend n'a pas livré internal/audio/resample.go.
//
// Frontière uniquement : seul le point d'entrée ToCanonical(raw) (pcm, ConversionInfo, error) et les
// validateurs existants sont appelés ; les champs de ConversionInfo sont vérifiés côté HTTP
// (internal/server/sound_upload_232_test.go : converted / source_format).
package audio

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Fabrique de WAV synthétiques — préfixe tw232.
// ---------------------------------------------------------------------------

type tw232Spec struct {
	tag        uint16 // 1 PCM, 3 float, 0x55 MP3…
	extensible bool   // enveloppe WAVE_FORMAT_EXTENSIBLE (sous-format = tag)
	channels   int
	rate       int
	bits       int
}

// tw232Encode écrit un échantillon x ∈ [-1,1] sur `bits` bits (8 = non signé).
func tw232Encode(buf *bytes.Buffer, x float64, spec tw232Spec) {
	if x > 1 {
		x = 1
	}
	if x < -1 {
		x = -1
	}
	switch {
	case spec.tag == 3:
		_ = binary.Write(buf, binary.LittleEndian, float32(x))
	case spec.bits == 8:
		buf.WriteByte(byte(math.Round(x*127) + 128))
	case spec.bits == 16:
		_ = binary.Write(buf, binary.LittleEndian, int16(math.Round(x*32767)))
	case spec.bits == 24:
		v := int32(math.Round(x * 8388607))
		buf.Write([]byte{byte(v), byte(v >> 8), byte(v >> 16)})
	case spec.bits == 32:
		_ = binary.Write(buf, binary.LittleEndian, int32(math.Round(x*2147483647)))
	}
}

// tw232WAV construit un WAV dont chaque trame est fournie par frame(i) (une valeur par canal).
func tw232WAV(spec tw232Spec, frames int, frame func(i int) []float64) []byte {
	var data bytes.Buffer
	for i := 0; i < frames; i++ {
		for _, x := range frame(i) {
			tw232Encode(&data, x, spec)
		}
	}
	blockAlign := spec.channels * spec.bits / 8
	var fmtc bytes.Buffer
	le := binary.LittleEndian
	if spec.extensible {
		_ = binary.Write(&fmtc, le, uint16(0xFFFE))
	} else {
		_ = binary.Write(&fmtc, le, spec.tag)
	}
	_ = binary.Write(&fmtc, le, uint16(spec.channels))
	_ = binary.Write(&fmtc, le, uint32(spec.rate))
	_ = binary.Write(&fmtc, le, uint32(spec.rate*blockAlign))
	_ = binary.Write(&fmtc, le, uint16(blockAlign))
	_ = binary.Write(&fmtc, le, uint16(spec.bits))
	if spec.extensible {
		_ = binary.Write(&fmtc, le, uint16(22))        // cbSize
		_ = binary.Write(&fmtc, le, uint16(spec.bits)) // validBitsPerSample
		_ = binary.Write(&fmtc, le, uint32(3))         // channelMask (FL|FR)
		_ = binary.Write(&fmtc, le, spec.tag)          // sous-format = tag réel
		fmtc.Write([]byte{0, 0, 0, 0, 0x10, 0, 0x80, 0, 0, 0xAA, 0, 0x38, 0x9B, 0x71})
	}
	var out bytes.Buffer
	out.WriteString("RIFF")
	_ = binary.Write(&out, le, uint32(4+8+fmtc.Len()+8+data.Len()))
	out.WriteString("WAVE")
	out.WriteString("fmt ")
	_ = binary.Write(&out, le, uint32(fmtc.Len()))
	out.Write(fmtc.Bytes())
	out.WriteString("data")
	_ = binary.Write(&out, le, uint32(data.Len()))
	out.Write(data.Bytes())
	return out.Bytes()
}

// tw232Sine : sinus de fréquence f, amplitude a, `seconds` secondes, dupliqué sur `channels`.
func tw232Sine(spec tw232Spec, f, a, seconds float64) []byte {
	frames := int(seconds * float64(spec.rate))
	return tw232WAV(spec, frames, func(i int) []float64 {
		v := a * math.Sin(2*math.Pi*f*float64(i)/float64(spec.rate))
		out := make([]float64, spec.channels)
		for c := range out {
			out[c] = v
		}
		return out
	})
}

// tw232Stereo16 relit un PCM canonique : trames (L,R) en int16.
func tw232Stereo16(t *testing.T, pcm []byte) (l, r []float64) {
	t.Helper()
	if len(pcm)%FrameSize != 0 {
		t.Fatalf("PCM non aligné sur %d octets (len=%d)", FrameSize, len(pcm))
	}
	for i := 0; i+FrameSize <= len(pcm); i += FrameSize {
		l = append(l, float64(int16(binary.LittleEndian.Uint16(pcm[i:])))/32768)
		r = append(r, float64(int16(binary.LittleEndian.Uint16(pcm[i+2:])))/32768)
	}
	return
}

func tw232Peak(x []float64) float64 {
	p := 0.0
	for _, v := range x {
		p = math.Max(p, math.Abs(v))
	}
	return p
}

// tw232RMS ignore `skip` trames à chaque bout (régime transitoire du filtre).
func tw232RMS(x []float64, skip int) float64 {
	if len(x) <= 2*skip {
		return 0
	}
	s := 0.0
	for _, v := range x[skip : len(x)-skip] {
		s += v * v
	}
	return math.Sqrt(s / float64(len(x)-2*skip))
}

// ---------------------------------------------------------------------------
// D2 — entrées acceptées : tags 1 / 3 / 0xFFFE, 8–32 bits, 1–2 canaux, 8–192 kHz.
// ---------------------------------------------------------------------------

func TestToCanonical232_AcceptedFormats_ProduceCanonicalStereoAtSameDuration(t *testing.T) {
	cases := []struct {
		name string
		spec tw232Spec
	}{
		{"48k_24bit_mono (cas réel #283)", tw232Spec{tag: 1, channels: 1, rate: 48000, bits: 24}},
		{"48k_16bit_stereo", tw232Spec{tag: 1, channels: 2, rate: 48000, bits: 16}},
		{"44k1_24bit_stereo (même fréquence)", tw232Spec{tag: 1, channels: 2, rate: 44100, bits: 24}},
		{"96k_32bit_int_stereo", tw232Spec{tag: 1, channels: 2, rate: 96000, bits: 32}},
		{"48k_float32_mono (tag 3)", tw232Spec{tag: 3, channels: 1, rate: 48000, bits: 32}},
		{"48k_float32_stereo (tag 3)", tw232Spec{tag: 3, channels: 2, rate: 48000, bits: 32}},
		{"48k_24bit_stereo_EXTENSIBLE_pcm", tw232Spec{tag: 1, extensible: true, channels: 2, rate: 48000, bits: 24}},
		{"48k_float32_EXTENSIBLE_float", tw232Spec{tag: 3, extensible: true, channels: 2, rate: 48000, bits: 32}},
		{"22k05_8bit_mono", tw232Spec{tag: 1, channels: 1, rate: 22050, bits: 8}},
		{"8k_16bit_mono (borne basse)", tw232Spec{tag: 1, channels: 1, rate: 8000, bits: 16}},
		{"192k_24bit_stereo (borne haute)", tw232Spec{tag: 1, channels: 2, rate: 192000, bits: 24}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw := tw232Sine(c.spec, 440, 0.5, 0.5)
			pcm, _, err := ToCanonical(raw)
			if err != nil {
				t.Fatalf("un WAV PCM valide doit être converti, pas refusé : %v", err)
			}
			wantFrames := float64(SampleRate) * 0.5
			gotFrames := float64(len(pcm) / FrameSize)
			if math.Abs(gotFrames-wantFrames) > 4 {
				t.Errorf("durée conservée attendue (~%.0f trames), got %.0f", wantFrames, gotFrames)
			}
			l, r := tw232Stereo16(t, pcm)
			if p := tw232Peak(l); p < 0.45 || p > 0.55 {
				t.Errorf("amplitude conservée attendue (crête ≈ 0,5), got %.3f", p)
			}
			if c.spec.channels == 1 {
				for i := range l {
					if l[i] != r[i] {
						t.Fatalf("mono ⇒ upmix L==R attendu, trame %d : L=%v R=%v", i, l[i], r[i])
					}
				}
			}
		})
	}
}

func TestToCanonical232_Stereo_ChannelsStayIndependent(t *testing.T) {
	spec := tw232Spec{tag: 1, channels: 2, rate: 48000, bits: 24}
	frames := 24000
	raw := tw232WAV(spec, frames, func(i int) []float64 {
		return []float64{0.5 * math.Sin(2*math.Pi*1000*float64(i)/48000), 0}
	})
	pcm, _, err := ToCanonical(raw)
	if err != nil {
		t.Fatalf("ToCanonical : %v", err)
	}
	l, r := tw232Stereo16(t, pcm)
	if tw232Peak(l) < 0.45 {
		t.Errorf("canal gauche attendu actif, crête=%.3f", tw232Peak(l))
	}
	if tw232Peak(r) > 0.01 {
		t.Errorf("canal droit attendu silencieux (pas de mélange), crête=%.3f", tw232Peak(r))
	}
}

func TestToCanonical232_Deterministic(t *testing.T) {
	raw := tw232Sine(tw232Spec{tag: 1, channels: 1, rate: 48000, bits: 24}, 1000, 0.7, 0.3)
	a, _, err1 := ToCanonical(raw)
	b, _, err2 := ToCanonical(raw)
	if err1 != nil || err2 != nil {
		t.Fatalf("erreurs : %v / %v", err1, err2)
	}
	if !bytes.Equal(a, b) {
		t.Error("la conversion doit être déterministe (mêmes octets en sortie pour la même entrée)")
	}
}

// ---------------------------------------------------------------------------
// Non-régression : fichier déjà canonique ⇒ chemin court, octets identiques.
// ---------------------------------------------------------------------------

func TestToCanonical232_AlreadyCanonical_BytesIdentical(t *testing.T) {
	spec := tw232Spec{tag: 1, channels: 2, rate: SampleRate, bits: BitsPerSample}
	raw := tw232Sine(spec, 523, 0.6, 0.4)
	want, err := extractCanonicalPCM(raw)
	if err != nil {
		t.Fatalf("setup invalide : le WAV de test doit être canonique : %v", err)
	}
	got, _, err := ToCanonical(raw)
	if err != nil {
		t.Fatalf("ToCanonical : %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("fichier déjà canonique ⇒ PCM identique octet pour octet (len got=%d want=%d)", len(got), len(want))
	}
}

// ---------------------------------------------------------------------------
// Refus nommés : compressés, non-WAV, hors bornes.
// ---------------------------------------------------------------------------

func TestToCanonical232_Refusals(t *testing.T) {
	good := tw232Spec{tag: 1, channels: 1, rate: 48000, bits: 16}
	cases := []struct {
		name string
		raw  []byte
	}{
		{"MP3 (tag 0x55) refusé", tw232Sine(tw232Spec{tag: 0x55, channels: 2, rate: 44100, bits: 16}, 440, 0.5, 0.1)},
		{"ADPCM (tag 2) refusé", tw232Sine(tw232Spec{tag: 2, channels: 1, rate: 22050, bits: 4 * 2}, 440, 0.5, 0.1)},
		{"3 canaux refusé", tw232Sine(tw232Spec{tag: 1, channels: 3, rate: 48000, bits: 16}, 440, 0.5, 0.1)},
		{"7 999 Hz (sous la borne) refusé", tw232Sine(tw232Spec{tag: 1, channels: 1, rate: 7999, bits: 16}, 440, 0.5, 0.1)},
		{"192 001 Hz (au-dessus) refusé", tw232Sine(tw232Spec{tag: 1, channels: 1, rate: 192001, bits: 16}, 440, 0.5, 0.05)},
		{"octets aléatoires (pas RIFF)", []byte("ID3\x03\x00\x00\x00\x00\x00\x00 ceci est un mp3 déguisé en wav")},
		{"vide", nil},
		{"RIFF tronqué", tw232Sine(good, 440, 0.5, 0.1)[:30]},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pcm, _, err := ToCanonical(c.raw)
			if err == nil {
				t.Fatalf("refus attendu, got PCM de %d octets", len(pcm))
			}
			if strings.TrimSpace(err.Error()) == "" {
				t.Error("le refus doit porter un message qui nomme le cas")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Qualité : repliement < −40 dB (balayage sinus 48 000 → 44 100 Hz), bande passante conservée.
// ---------------------------------------------------------------------------

// Un sinus entre 22 050 Hz (Nyquist de sortie) et 24 000 Hz (Nyquist d'entrée) n'a pas de représentation
// valide à 44 100 Hz : un rééchantillonneur sans filtre le replie dans la bande audible. Le contrat exige
// que l'énergie restante soit < −40 dB de l'entrée.
func TestToCanonical232_Aliasing_Sweep48kTo44k1_Below40dB(t *testing.T) {
	spec := tw232Spec{tag: 1, channels: 1, rate: 48000, bits: 24}
	const amp = 0.5
	inRMS := amp / math.Sqrt2
	for f := 22500.0; f <= 23900.0; f += 350 {
		raw := tw232Sine(spec, f, amp, 0.4)
		pcm, _, err := ToCanonical(raw)
		if err != nil {
			t.Fatalf("f=%.0f Hz : %v", f, err)
		}
		l, _ := tw232Stereo16(t, pcm)
		rms := tw232RMS(l, 600) // on écarte le régime transitoire du filtre
		db := 20 * math.Log10(math.Max(rms, 1e-9)/inRMS)
		if db > -40 {
			t.Errorf("repliement à %.0f Hz : %.1f dB (exigé < −40 dB)", f, db)
		}
	}
}

func TestToCanonical232_Passband_1kHz_PreservedWithin1dB(t *testing.T) {
	for _, rate := range []int{48000, 96000, 22050} {
		spec := tw232Spec{tag: 1, channels: 1, rate: rate, bits: 24}
		raw := tw232Sine(spec, 1000, 0.5, 0.4)
		pcm, _, err := ToCanonical(raw)
		if err != nil {
			t.Fatalf("rate=%d : %v", rate, err)
		}
		l, _ := tw232Stereo16(t, pcm)
		rms := tw232RMS(l, 600)
		db := 20 * math.Log10(rms/(0.5/math.Sqrt2))
		if math.Abs(db) > 1 {
			t.Errorf("rate=%d : gain à 1 kHz = %.2f dB (exigé |gain| ≤ 1 dB)", rate, db)
		}
	}
}

// Montée en fréquence (22 050 → 44 100) : aucun contenu au-dessus de la source ne doit apparaître,
// la coupure est min(f_src, f_dst)/2 (sinus 5 kHz conservé).
func TestToCanonical232_Upsampling_PreservesTone(t *testing.T) {
	spec := tw232Spec{tag: 1, channels: 1, rate: 22050, bits: 16}
	pcm, _, err := ToCanonical(tw232Sine(spec, 5000, 0.5, 0.4))
	if err != nil {
		t.Fatalf("ToCanonical : %v", err)
	}
	l, _ := tw232Stereo16(t, pcm)
	if db := 20 * math.Log10(tw232RMS(l, 600)/(0.5/math.Sqrt2)); math.Abs(db) > 1 {
		t.Errorf("gain à 5 kHz après montée en fréquence = %.2f dB (exigé |gain| ≤ 1 dB)", db)
	}
}

// ---------------------------------------------------------------------------
// Plafonds : durée et octets canoniques inchangés, évalués APRÈS conversion ; entrée brute ≤ 4 × plafond.
// ---------------------------------------------------------------------------

// Un WAV non canonique plus lourd que le plafond canonique (2 Mio) mais ≤ 4 × 2 Mio est accepté si,
// converti, il respecte durée et octets canoniques.
func TestValidateUpload232_RawLargerThanCanonicalCap_ButUnder4x_Accepted(t *testing.T) {
	spec := tw232Spec{tag: 1, channels: 2, rate: 192000, bits: 24} // 1,15 Mo/s
	raw := tw232Sine(spec, 440, 0.4, 2.5)                          // ≈ 2,9 Mio brut ; 2,5 s canonique
	if len(raw) <= MaxUploadBytes || len(raw) > 4*MaxUploadBytes {
		t.Fatalf("setup invalide : taille brute %d hors de ]%d ; %d]", len(raw), MaxUploadBytes, 4*MaxUploadBytes)
	}
	res, err := ValidateUpload(raw)
	if err != nil {
		t.Fatalf("attendu accepté (brut ≤ 4 × plafond, canonique conforme) : %v", err)
	}
	if res.Duration < 2400*1e6 || res.Duration > 2600*1e6 { // ns : 2,4 s – 2,6 s
		t.Errorf("durée attendue ≈ 2,5 s, got %v", res.Duration)
	}
}

// La durée est calculée APRÈS conversion : 48 kHz mono 24 bits, 6 s = 864 Kio brut (< 2 Mio) mais > 5 s.
func TestValidateUpload232_DurationLimitAppliedAfterConversion(t *testing.T) {
	raw := tw232Sine(tw232Spec{tag: 1, channels: 1, rate: 48000, bits: 24}, 440, 0.4, 6)
	if _, err := ValidateUpload(raw); err == nil {
		t.Fatal("un cue de 6 s doit être refusé (limite 5 s inchangée), même converti")
	}
}

func TestValidateUpload232_ConvertibleWAV_AcceptedAndCanonical(t *testing.T) {
	raw := tw232Sine(tw232Spec{tag: 1, channels: 1, rate: 48000, bits: 24}, 440, 0.4, 1)
	res, err := ValidateUpload(raw)
	if err != nil {
		t.Fatalf("un WAV 48k/24b/mono doit être accepté et converti : %v", err)
	}
	if len(res.PCM)%FrameSize != 0 {
		t.Errorf("PCM de sortie non canonique (len=%d)", len(res.PCM))
	}
}

func TestValidateQuestionSound232_ConvertibleWAV_AcceptedAndStereo(t *testing.T) {
	raw := tw232Sine(tw232Spec{tag: 3, channels: 1, rate: 48000, bits: 32}, 440, 0.4, 2)
	res, err := ValidateQuestionSound(raw)
	if err != nil {
		t.Fatalf("un WAV float32 48k mono doit être accepté et converti : %v", err)
	}
	l, r := tw232Stereo16(t, res.PCM)
	if len(l) == 0 || tw232Peak(l) < 0.35 || tw232Peak(r) < 0.35 {
		t.Errorf("PCM stéréo canonique non vide attendu, crêtes L=%.2f R=%.2f", tw232Peak(l), tw232Peak(r))
	}
}

// Durée > 30 s refusée après conversion (48 kHz mono 16 bits : 31 s = 2,9 Mio brut).
func TestValidateQuestionSound232_DurationLimitAfterConversion(t *testing.T) {
	raw := tw232Sine(tw232Spec{tag: 1, channels: 1, rate: 48000, bits: 16}, 440, 0.3, 31)
	if _, err := ValidateQuestionSound(raw); err == nil {
		t.Fatal("un son de 31 s doit être refusé (limite 30 s inchangée)")
	}
}

func TestValidateQuestionSound232_RefusesMP3AndPlainBytes(t *testing.T) {
	if _, err := ValidateQuestionSound(tw232Sine(tw232Spec{tag: 0x55, channels: 2, rate: 44100, bits: 16}, 440, 0.5, 0.1)); err == nil {
		t.Error("tag MP3 (0x55) doit rester refusé")
	}
	if _, err := ValidateQuestionSound([]byte("pas un wav")); err == nil {
		t.Error("octets non RIFF doivent rester refusés")
	}
}
