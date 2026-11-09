package audio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"
)

// Conversion WAV → format canonique à l'upload (#232, contracts/sound.md §3
// amendement v11.6.0). Point d'entrée unique : ToCanonical. Go pur, aucune
// dépendance tierce.
//
// Chaîne : parseur WAV élargi → PCM flottant interne → conversion de fréquence
// par FIR fenêtré polyphase (sinc × Kaiser, coupure min(f_src, f_dst)/2) →
// arrondi 16 bits sans dither → upmix mono (upmixMonoToStereo). Déterministe.
// Un fichier déjà canonique prend un chemin court : octets identiques.

// MaxRawUploadFactor : plafond d'entrée brute = 4 × le plafond canonique du
// chemin (borne la mémoire de la conversion).
const MaxRawUploadFactor = 4

// MaxRawUploadBytes et MaxRawQuestionSoundBytes : plafonds d'entrée BRUTE des
// chemins cue et média de question/décompte (au-delà ⇒ ErrTooLarge, HTTP 413).
const (
	MaxRawUploadBytes        = MaxRawUploadFactor * MaxUploadBytes
	MaxRawQuestionSoundBytes = MaxRawUploadFactor * MaxQuestionSoundBytes
)

// ErrTooLarge signale un dépassement de plafond d'octets (entrée brute ou
// résultat canonique) : l'appelant HTTP répond 413.
var ErrTooLarge = errors.New("fichier trop volumineux")

// ConversionInfo décrit ce que ToCanonical a fait.
type ConversionInfo struct {
	// Converted est vrai si l'entrée n'était pas déjà au format canonique
	// stéréo 16 bits 44 100 Hz (mono, autre fréquence, autre profondeur…).
	Converted bool
	// SourceFormat décrit le format d'origine (ex. "48000Hz/1ch/24bit") ;
	// "" si déjà canonique.
	SourceFormat string
}

const (
	wavTagPCM        = 1
	wavTagFloat      = 3
	wavTagExtensible = 0xFFFE

	minSourceRate = 8000
	maxSourceRate = 192000

	// firZeroCrossings : demi-largeur du noyau en passages par zéro du sinc de
	// coupure (≥ 32 taps par phase dans tous les cas) ; kaiserBeta ≈ 63 dB.
	firZeroCrossings = 96
	kaiserBeta       = 6.0
)

// tooLongError : la durée de l'entrée dépasse la limite avant même la
// conversion (évite de convertir un fichier qui sera refusé).
type tooLongError struct{ duration time.Duration }

func (e *tooLongError) Error() string { return fmt.Sprintf("durée %.1f s", e.duration.Seconds()) }

type wavFormat struct {
	float    bool
	channels int
	rate     int
	bits     int
}

func (f wavFormat) String() string {
	return fmt.Sprintf("%dHz/%dch/%dbit", f.rate, f.channels, f.bits)
}

func (f wavFormat) isCanonical() bool {
	return !f.float && f.channels == ChannelCount && f.rate == SampleRate && f.bits == BitsPerSample
}

// ToCanonical convertit un fichier WAV PCM (tags 1 PCM entier 8/16/24/32 bits,
// 3 IEEE float 32 bits, 0xFFFE extensible avec ces sous-formats ; 1 ou 2 canaux ;
// 8 000–192 000 Hz) en PCM canonique stéréo 16 bits 44 100 Hz (octets bruts,
// sans en-tête ; BuildCanonicalWAV pour l'envelopper). Aucun plafond de durée.
func ToCanonical(raw []byte) ([]byte, ConversionInfo, error) {
	return toCanonical(raw, 0)
}

// toCanonical : maxDuration > 0 refuse (tooLongError) une entrée plus longue
// avant conversion.
func toCanonical(raw []byte, maxDuration time.Duration) ([]byte, ConversionInfo, error) {
	f, data, err := parseWAV(raw)
	if err != nil {
		return nil, ConversionInfo{}, err
	}
	bytesPer := f.bits / 8
	frameBytes := bytesPer * f.channels
	frames := len(data) / frameBytes
	data = data[:frames*frameBytes]

	if f.isCanonical() {
		d := pcmDuration(data)
		if maxDuration > 0 && d > maxDuration {
			return nil, ConversionInfo{}, &tooLongError{d}
		}
		return data, ConversionInfo{}, nil // chemin court : octets identiques
	}

	srcDur := time.Duration(float64(frames) / float64(f.rate) * float64(time.Second))
	if maxDuration > 0 && srcDur > maxDuration {
		return nil, ConversionInfo{}, &tooLongError{srcDur}
	}

	chans := decodeChannels(f, data, frames)
	if f.rate != SampleRate {
		for i := range chans {
			chans[i] = resample(chans[i], f.rate, SampleRate)
		}
	}
	out := quantizeStereo(chans)
	return out, ConversionInfo{Converted: true, SourceFormat: f.String()}, nil
}

// parseWAV parcourt les chunks RIFF et renvoie le format et le chunk "data".
func parseWAV(raw []byte) (wavFormat, []byte, error) {
	var f wavFormat
	if len(raw) < 12 || string(raw[0:4]) != "RIFF" || string(raw[8:12]) != "WAVE" {
		return f, nil, fmt.Errorf("ce fichier n'est pas un WAV (en-tête RIFF/WAVE absent)")
	}
	var (
		haveFmt, haveData bool
		data              []byte
	)
	pos := 12
	for pos+8 <= len(raw) {
		id := string(raw[pos : pos+4])
		size := int(binary.LittleEndian.Uint32(raw[pos+4 : pos+8]))
		body := pos + 8
		if body+size > len(raw) || size < 0 {
			if id != "data" {
				return f, nil, fmt.Errorf("WAV tronqué ou corrompu (chunk %q dépasse le fichier)", id)
			}
			size = len(raw) - body // data tronqué (flux) : on garde ce qui est présent
		}
		switch id {
		case "fmt ":
			if size < 16 {
				return f, nil, fmt.Errorf("WAV corrompu (chunk fmt trop court : %d octets)", size)
			}
			tag := binary.LittleEndian.Uint16(raw[body:])
			f.channels = int(binary.LittleEndian.Uint16(raw[body+2:]))
			f.rate = int(binary.LittleEndian.Uint32(raw[body+4:]))
			f.bits = int(binary.LittleEndian.Uint16(raw[body+14:]))
			if tag == wavTagExtensible {
				if size < 40 {
					return f, nil, fmt.Errorf("WAV extensible corrompu (chunk fmt de %d octets)", size)
				}
				tag = binary.LittleEndian.Uint16(raw[body+24:]) // début du GUID de sous-format
			}
			switch tag {
			case wavTagPCM:
				if f.bits != 8 && f.bits != 16 && f.bits != 24 && f.bits != 32 {
					return f, nil, fmt.Errorf("profondeur PCM non prise en charge : %d bits (8, 16, 24 ou 32 attendus)", f.bits)
				}
			case wavTagFloat:
				f.float = true
				if f.bits != 32 {
					return f, nil, fmt.Errorf("WAV flottant de %d bits non pris en charge (32 bits attendus)", f.bits)
				}
			default:
				return f, nil, fmt.Errorf("format WAV non pris en charge (tag %d) : seuls le PCM et le flottant 32 bits sont acceptés — les formats compressés (MP3, ADPCM…) sont refusés", tag)
			}
			if f.channels != 1 && f.channels != 2 {
				return f, nil, fmt.Errorf("nombre de voies non pris en charge : %d (mono ou stéréo attendus)", f.channels)
			}
			if f.rate < minSourceRate || f.rate > maxSourceRate {
				return f, nil, fmt.Errorf("fréquence non prise en charge : %d Hz (%d à %d Hz attendus)", f.rate, minSourceRate, maxSourceRate)
			}
			haveFmt = true
		case "data":
			data = raw[body : body+size]
			haveData = true
		}
		pos = body + size
		if size%2 == 1 {
			pos++
		}
	}
	if !haveFmt {
		return f, nil, fmt.Errorf("WAV corrompu (chunk fmt absent)")
	}
	if !haveData {
		return f, nil, fmt.Errorf("WAV corrompu (chunk data absent)")
	}
	return f, data, nil
}

// decodeChannels : PCM → flottants dans [-1, 1], un tableau par canal.
func decodeChannels(f wavFormat, data []byte, frames int) [][]float32 {
	chans := make([][]float32, f.channels)
	for c := range chans {
		chans[c] = make([]float32, frames)
	}
	bytesPer := f.bits / 8
	for i := 0; i < frames; i++ {
		for c := 0; c < f.channels; c++ {
			p := (i*f.channels + c) * bytesPer
			var v float32
			switch {
			case f.float:
				v = math.Float32frombits(binary.LittleEndian.Uint32(data[p:]))
				if v != v { // NaN
					v = 0
				}
			case f.bits == 8:
				v = (float32(data[p]) - 128) / 128
			case f.bits == 16:
				v = float32(int16(binary.LittleEndian.Uint16(data[p:]))) / 32768
			case f.bits == 24:
				s := int32(data[p]) | int32(data[p+1])<<8 | int32(int8(data[p+2]))<<16
				v = float32(s) / 8388608
			default: // 32
				v = float32(int32(binary.LittleEndian.Uint32(data[p:]))) / 2147483648
			}
			chans[c][i] = v
		}
	}
	return chans
}

// quantizeStereo : arrondi 16 bits (sans dither), saturation, upmix mono.
func quantizeStereo(chans [][]float32) []byte {
	n := len(chans[0])
	mono := make([]byte, 0, n*BytesPerSample*len(chans))
	if len(chans) == 1 {
		for _, v := range chans[0] {
			mono = binary.LittleEndian.AppendUint16(mono, uint16(quantize16(v)))
		}
		return upmixMonoToStereo(mono)
	}
	for i := 0; i < n; i++ {
		mono = binary.LittleEndian.AppendUint16(mono, uint16(quantize16(chans[0][i])))
		mono = binary.LittleEndian.AppendUint16(mono, uint16(quantize16(chans[1][i])))
	}
	return mono
}

func quantize16(v float32) int16 {
	x := math.Round(float64(v) * 32768)
	if x > 32767 {
		return 32767
	}
	if x < -32768 {
		return -32768
	}
	return int16(x)
}

// canonicalBytesFor estime la taille canonique (stéréo 16 bits 44,1 kHz, en-tête
// compris) d'un son de durée d — sert à distinguer 413 (trop gros) de 400
// (trop long) quand la durée dépasse la limite.
func canonicalBytesFor(d time.Duration) int {
	return int(d.Seconds()*float64(SampleRate))*FrameSize + 44
}

// ---------------------------------------------------------------------------
// Conversion de fréquence : FIR fenêtré polyphase
// ---------------------------------------------------------------------------

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// besselI0 : fonction de Bessel modifiée d'ordre 0 (série).
func besselI0(x float64) float64 {
	sum, term := 1.0, 1.0
	for k := 1; k < 64; k++ {
		term *= (x / (2 * float64(k))) * (x / (2 * float64(k)))
		sum += term
		if term < 1e-12*sum {
			break
		}
	}
	return sum
}

// resample convertit src (fréquence rs) vers rd par sinc × Kaiser polyphase.
// Sortie : floor(len(src)·rd/rs) échantillons. Hors signal = silence.
func resample(src []float32, rs, rd int) []float32 {
	if len(src) == 0 || rs == rd {
		return src
	}
	g := gcd(rs, rd)
	L, M := rd/g, rs/g // sortie n ↔ position source n·M/L
	nOut := int(int64(len(src)) * int64(rd) / int64(rs))
	out := make([]float32, nOut)

	// Coupure normalisée (cycles par échantillon source) = min(rs, rd)/2/rs.
	fc := math.Min(float64(rs), float64(rd)) / 2 / float64(rs)
	halfWidth := float64(firZeroCrossings) / (2 * fc) // en échantillons source
	K := int(math.Ceil(halfWidth))
	i0kaiser := besselI0(kaiserBeta)

	weights := make([][]float32, L) // table de phases, calculée à la demande
	phaseWeights := func(p int) []float32 {
		if w := weights[p]; w != nil {
			return w
		}
		frac := float64(p) / float64(L)
		w := make([]float32, 2*K)
		var sum float64
		for t := 0; t < 2*K; t++ {
			d := t - K + 1
			u := frac - float64(d)
			var h float64
			if math.Abs(u) < halfWidth {
				x := 2 * fc * u
				sinc := 1.0
				if x != 0 {
					sinc = math.Sin(math.Pi*x) / (math.Pi * x)
				}
				r := u / halfWidth
				win := besselI0(kaiserBeta*math.Sqrt(1-r*r)) / i0kaiser
				h = 2 * fc * sinc * win
			}
			w[t] = float32(h)
			sum += h
		}
		if sum != 0 { // gain unitaire en continu pour chaque phase
			for t := range w {
				w[t] = float32(float64(w[t]) / sum)
			}
		}
		weights[p] = w
		return w
	}

	n := len(src)
	for o := 0; o < nOut; o++ {
		num := int64(o) * int64(M)
		i0 := int(num / int64(L))
		w := phaseWeights(int(num % int64(L)))
		var acc float64
		base := i0 - K + 1
		for t := 0; t < 2*K; t++ {
			j := base + t
			if j >= 0 && j < n {
				acc += float64(w[t]) * float64(src[j])
			}
		}
		out[o] = float32(acc)
	}
	return out
}
