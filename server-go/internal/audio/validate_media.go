package audio

import (
	"encoding/binary"
	"fmt"
	"time"
)

// MaxQuestionSoundDuration et MaxQuestionSoundBytes sont les limites
// d'upload du son de question (contracts/sound.md §10.4) — DISTINCTES de
// MaxSoundDuration / MaxUploadBytes (validate.go), qui protègent la file
// strictement séquentielle des cues (§5.2) et ne sont PAS touchées par ce
// fichier. Les deux jeux de constantes coexistent délibérément : voir le
// test-sentinelle (test-writer) qui vérifie que les valeurs des cues
// restent inchangées — « factoriser » les deux jeux en un seul supprimerait
// silencieusement la garantie du §5.2.
const (
	// MaxQuestionSoundDuration — 30 s (arbitrage utilisateur, GATE 2 du
	// 2026-09-22).
	MaxQuestionSoundDuration = 30 * time.Second
	// MaxQuestionSoundBytes — 6 Mio (6 << 20). 30 s canoniques pèsent
	// 5,05 Mio ; 6 Mio laisse +18,9 % pour l'en-tête RIFF et d'éventuels
	// chunks LIST/INFO sans autoriser un fichier abusif. Appliqué à
	// l'upload BRUT (avant suréchantillonnage mono→stéréo éventuel, voir
	// extractQuestionSoundPCM) — un mono de 30s pèse déjà moins que son
	// équivalent stéréo, donc ce plafond reste la borne la plus stricte des
	// deux dans tous les cas.
	MaxQuestionSoundBytes = 6 << 20
)

// QuestionSoundValidation porte ce dont l'endpoint d'upload
// (internal/server/http.go, Batch 1 tâche 7) a besoin après une validation
// réussie : le PCM **canonique stéréo** — voir extractQuestionSoundPCM,
// un mono d'entrée est déjà suréchantillonné à ce stade — prêt à être
// écrit sur disque via BuildCanonicalWAV, et la durée calculée — utilisée
// par l'appelant pour construire l'avertissement CONTEXTUEL contre
// Question.TIME (§10.4). Aucun champ Warning ici, à la différence de
// ValidationResult (validate.go) pour les cues : le son de question n'a
// AUCUN seuil d'avertissement fixe (§10.4), seul l'appelant — qui connaît
// Question.TIME et le mode simultané/différé — peut décider s'il y a lieu
// d'avertir.
type QuestionSoundValidation struct {
	PCM      []byte
	Duration time.Duration
}

// ValidateQuestionSound valide les octets bruts d'un upload de son de
// question contre le format canonique **assoupli au mono** (§10.4bis ter —
// arbitrage utilisateur QUALIF v11.1, 2026-09-22 : un WAV mono doit être
// accepté) et les limites propres au son de question ci-dessus (contract
// §10.4 : 30 s / 6 Mio, AUCUN seuil fixe d'avertissement — voir
// QuestionSoundValidation).
//
// Un mono est ACCEPTÉ puis suréchantillonné en stéréo (chaque échantillon
// dupliqué sur les deux voies, extractQuestionSoundPCM/upmixMonoToStereo) —
// une opération arithmétique pure sur du PCM déjà décodé, jamais un
// transcodage de codec, cohérente avec l'esprit "refus strict plutôt que
// conversion complexe" du §3 : ce n'est PAS une exception à ce principe,
// dupliquer un échantillon n'est pas "convertir" au sens où §3 l'entend
// (rééchantillonnage de fréquence, décodage compressé). Le fichier
// EFFECTIVEMENT stocké sur disque est donc TOUJOURS stéréo canonique — le
// pilote de lecture (media_oto.go) n'a besoin d'aucune branche
// supplémentaire pour un second cas de figure.
//
// Construit sur extractQuestionSoundPCM (ce fichier) — un parseur WAV
// DÉLIBÉRÉMENT DISTINCT d'extractCanonicalPCM (bank.go), qui reste
// strictement stéréo et INCHANGÉ : cette dernière est partagée avec le
// moteur de cues (FileBank.PCM, ValidateUpload) et doit conserver son
// comportement exact — élargir SA tolérance canaux affecterait aussi
// l'upload et la lecture des bruitages d'ambiance, hors périmètre de cet
// arbitrage (limité au média de question).
//
// Chaque cause de refus est nommée dans le message d'erreur (contract CA2 :
// pas WAV / fréquence-bits / nombre de voies invalide / > 30 s / > 6 Mio),
// jamais un message générique.
func ValidateQuestionSound(raw []byte) (QuestionSoundValidation, error) {
	if len(raw) > MaxQuestionSoundBytes {
		return QuestionSoundValidation{}, fmt.Errorf(
			"ce fichier pèse %.1f Mio — la limite pour un son de question est de %d Mio",
			float64(len(raw))/(1<<20), MaxQuestionSoundBytes>>20)
	}

	pcm, channels, err := extractQuestionSoundPCM(raw)
	if err != nil {
		return QuestionSoundValidation{}, fmt.Errorf(
			"ce fichier n'est pas un WAV compatible (%d Hz, mono ou stéréo, %d bits) : %w",
			SampleRate, BitsPerSample, err)
	}
	if channels == 1 {
		pcm = upmixMonoToStereo(pcm)
	}

	duration := pcmDuration(pcm)
	if duration > MaxQuestionSoundDuration {
		return QuestionSoundValidation{}, fmt.Errorf(
			"ce son dure %.1f s — la limite pour un son de question est de %d s",
			duration.Seconds(), int(MaxQuestionSoundDuration.Seconds()))
	}

	return QuestionSoundValidation{PCM: pcm, Duration: duration}, nil
}

// extractQuestionSoundPCM parse un flux RIFF/WAVE et valide son chunk
// "fmt " contre le format canonique DU SON DE QUESTION — 44 100 Hz,
// 16 bits, **1 OU 2 voies** (arbitrage QUALIF v11.1 ci-dessus) — puis
// renvoie les octets bruts du chunk "data" ainsi que le nombre de voies
// effectif, pour qu'ValidateQuestionSound sache s'il doit suréchantillonner.
//
// Duplique DÉLIBÉRÉMENT la marche des chunks d'extractCanonicalPCM
// (bank.go) plutôt que de la paramétrer ou de la réutiliser : cette
// dernière est partagée avec le moteur de cues (§10.4's propre
// avertissement le rappelle) et DOIT rester à l'identique — un paramètre
// "canaux tolérés" y introduirait un risque de régression sur la validation
// ET la lecture des bruitages pour un gain de code minime. Les deux
// parseurs partagent uniquement les CONSTANTES de format (format.go), donc
// ne peuvent pas diverger sur ce qui est SampleRate/BitsPerSample.
func extractQuestionSoundPCM(raw []byte) (data []byte, channels int, err error) {
	if len(raw) < 12 || string(raw[0:4]) != "RIFF" || string(raw[8:12]) != "WAVE" {
		return nil, 0, fmt.Errorf("not a RIFF/WAVE file")
	}

	var (
		haveFmt                bool
		chCount, bitsPerSample uint16
		sampleRate             uint32
		haveData               bool
	)

	pos := 12
	for pos+8 <= len(raw) {
		id := string(raw[pos : pos+4])
		size := binary.LittleEndian.Uint32(raw[pos+4 : pos+8])
		body := pos + 8
		if body+int(size) > len(raw) {
			return nil, 0, fmt.Errorf("chunk %q overruns file (size=%d)", id, size)
		}
		switch id {
		case "fmt ":
			if size < 16 {
				return nil, 0, fmt.Errorf("fmt chunk too short (%d bytes)", size)
			}
			format := binary.LittleEndian.Uint16(raw[body : body+2])
			if format != 1 {
				return nil, 0, fmt.Errorf("unsupported WAV format tag %d (only PCM=1)", format)
			}
			chCount = binary.LittleEndian.Uint16(raw[body+2 : body+4])
			sampleRate = binary.LittleEndian.Uint32(raw[body+4 : body+8])
			bitsPerSample = binary.LittleEndian.Uint16(raw[body+14 : body+16])
			haveFmt = true
		case "data":
			data = raw[body : body+int(size)]
			haveData = true
		}
		pos = body + int(size)
		if size%2 == 1 {
			pos++
		}
	}

	if !haveFmt {
		return nil, 0, fmt.Errorf("no fmt chunk")
	}
	if !haveData {
		return nil, 0, fmt.Errorf("no data chunk")
	}
	if chCount != 1 && chCount != 2 {
		return nil, 0, fmt.Errorf("unsupported channel count %d (expected 1 or 2)", chCount)
	}
	if sampleRate != uint32(SampleRate) || int(bitsPerSample) != BitsPerSample {
		return nil, 0, fmt.Errorf("non-canonical format: %d ch, %d Hz, %d-bit (expected %d Hz, %d-bit, 1 or 2 ch)",
			chCount, sampleRate, bitsPerSample, SampleRate, BitsPerSample)
	}
	return data, int(chCount), nil
}

// upmixMonoToStereo duplique chaque échantillon 16 bits mono sur les deux
// voies d'un tampon stéréo entrelacé — une opération arithmétique pure
// (aucun décodage, aucun rééchantillonnage de fréquence), jamais un
// "transcodage" au sens que §3 exclut. Tout octet de fin non aligné sur
// BytesPerSample (fichier malformé qu'extractQuestionSoundPCM aurait dû
// rejeter par ailleurs) est silencieusement ignoré plutôt que de paniquer.
func upmixMonoToStereo(mono []byte) []byte {
	stereo := make([]byte, 0, len(mono)*2)
	for i := 0; i+BytesPerSample <= len(mono); i += BytesPerSample {
		sample := mono[i : i+BytesPerSample]
		stereo = append(stereo, sample...) // gauche
		stereo = append(stereo, sample...) // droite
	}
	return stereo
}

// BuildCanonicalWAV enveloppe pcm (échantillons stéréo entrelacés 16 bits,
// déjà au format canonique — ChannelCount voies, SampleRate, BitsPerSample)
// dans un en-tête RIFF/WAVE minimal neuf. Utilisé par l'endpoint d'upload
// (internal/server/http.go) pour écrire sur disque le résultat de
// ValidateQuestionSound — TOUJOURS stéréo, qu'il vienne d'un WAV stéréo
// d'origine ou d'un mono suréchantillonné : le fichier stocké est donc
// toujours directement lisible par le pilote de lecture (media_oto.go)
// sans branche supplémentaire pour un second cas de figure.
func BuildCanonicalWAV(pcm []byte) []byte {
	buf := make([]byte, 44+len(pcm))
	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(36+len(pcm)))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1) // PCM
	binary.LittleEndian.PutUint16(buf[22:24], uint16(ChannelCount))
	binary.LittleEndian.PutUint32(buf[24:28], uint32(SampleRate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(SampleRate*ChannelCount*BytesPerSample))
	binary.LittleEndian.PutUint16(buf[32:34], uint16(ChannelCount*BytesPerSample))
	binary.LittleEndian.PutUint16(buf[34:36], uint16(BitsPerSample))
	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(len(pcm)))
	copy(buf[44:], pcm)
	return buf
}
