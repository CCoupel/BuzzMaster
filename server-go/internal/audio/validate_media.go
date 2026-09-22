package audio

import (
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
	// chunks LIST/INFO sans autoriser un fichier abusif.
	MaxQuestionSoundBytes = 6 << 20
)

// QuestionSoundValidation porte ce dont l'endpoint d'upload
// (internal/server/http.go, Batch 1 tâche 7) a besoin après une validation
// réussie : le PCM canonique brut, prêt à être écrit sur disque dans un
// en-tête WAV neuf, et la durée calculée — utilisée par l'appelant pour
// construire l'avertissement CONTEXTUEL contre Question.TIME (§10.4).
// Aucun champ Warning ici, à la différence de ValidationResult (validate.go)
// pour les cues : le son de question n'a AUCUN seuil d'avertissement fixe
// (§10.4), seul l'appelant — qui connaît Question.TIME et le mode
// simultané/différé — peut décider s'il y a lieu d'avertir.
type QuestionSoundValidation struct {
	PCM      []byte
	Duration time.Duration
}

// ValidateQuestionSound valide les octets bruts d'un upload de son de
// question contre le format canonique (contract §3, identique aux cues) et
// les limites propres au son de question ci-dessus (contract §10.4 : 30 s /
// 6 Mio, AUCUN seuil fixe d'avertissement — voir QuestionSoundValidation).
//
// Construit sur extractCanonicalPCM (bank.go) — jamais un second parseur
// WAV écrit indépendamment, exactement comme ValidateUpload le fait déjà
// pour les cues (§7).
//
// Chaque cause de refus est nommée dans le message d'erreur (contract CA2 :
// pas WAV / fréquence-canaux-bits / > 30 s / > 6 Mio), jamais un message
// générique.
func ValidateQuestionSound(raw []byte) (QuestionSoundValidation, error) {
	if len(raw) > MaxQuestionSoundBytes {
		return QuestionSoundValidation{}, fmt.Errorf(
			"ce fichier pèse %.1f Mio — la limite pour un son de question est de %d Mio",
			float64(len(raw))/(1<<20), MaxQuestionSoundBytes>>20)
	}

	pcm, err := extractCanonicalPCM(raw)
	if err != nil {
		return QuestionSoundValidation{}, fmt.Errorf(
			"ce fichier n'est pas un WAV canonique (%d Hz, %d voies, %d bits) : %w",
			SampleRate, ChannelCount, BitsPerSample, err)
	}

	duration := pcmDuration(pcm)
	if duration > MaxQuestionSoundDuration {
		return QuestionSoundValidation{}, fmt.Errorf(
			"ce son dure %.1f s — la limite pour un son de question est de %d s",
			duration.Seconds(), int(MaxQuestionSoundDuration.Seconds()))
	}

	return QuestionSoundValidation{PCM: pcm, Duration: duration}, nil
}
