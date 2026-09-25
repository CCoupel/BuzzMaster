// Suite test-writer pour #219 (milestone v11.1 — plan
// _work/reports/plan-20260922-103848.md Phase 0 tâche 4, contract §10.4) :
// ValidateQuestionSound — le validateur DÉDIÉ au média sonore de question
// (validate_media.go), distinct du validateur des cues (validate.go,
// #230) et réutilisant extractCanonicalPCM (bank.go) exactement comme
// ValidateUpload le fait déjà, jamais un second parseur WAV écrit
// indépendamment (contract §10.4 : "Le validateur du média réutilise
// extractCanonicalPCM").
//
// Signature réelle (validate_media.go, livré par dev-backend en parallèle
// de ce fichier) : ValidateQuestionSound(raw []byte) (QuestionSoundValidation,
// error) — QuestionSoundValidation{PCM, Duration}, sans champ Warning
// (l'avertissement contextuel contre Question.TIME est du ressort de
// l'appelant, Batch 1). Ce fichier ne nomme jamais le type de retour
// explicitement (`result, err := ValidateQuestionSound(...)`), donc il
// reste indifférent à son nom exact.
//
// Convention de collision : préfixe tw219v pour ne jamais entrer en
// collision avec un helper d'un autre fichier de ce paquet.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun ;
// il ne touche à aucune ligne de validate.go — c'est précisément ce que
// vérifie son test-sentinelle ci-dessous.
package audio

import (
	"testing"
	"time"
)

// tw219vBuildWAV hand-builds a RIFF/WAVE byte stream with the given
// fmt-chunk fields, a "data" chunk of dataLen canonical-silence bytes, and
// an optional extra "LIST" chunk of padBytes bytes BEFORE the data chunk —
// used to inflate the RAW FILE size independently of the audio duration
// (contract §10.4 : le plafond de 6 Mio laisse de la place "pour l'en-tête
// RIFF et d'éventuels chunks LIST/INFO... sans autoriser un fichier
// abusif" — un vrai éditeur WAV tiers ajoute parfois de tels chunks).
// Mirrors bank_229_test.go's tw229bBuildWAVHeader, mais local à ce fichier/
// paquet (audio, pas audio_test) pour ne jamais entrer en collision avec
// lui.
func tw219vBuildWAV(t *testing.T, channels, sampleRate, bitsPerSample, dataLen, padBytes int) []byte {
	t.Helper()
	byteRate := sampleRate * channels * (bitsPerSample / 8)
	blockAlign := channels * (bitsPerSample / 8)

	padChunk := 0
	if padBytes > 0 {
		padChunk = 8 + padBytes // en-tête de chunk "LIST" (id+taille) + charge utile
	}
	total := 44 + padChunk + dataLen
	buf := make([]byte, total)
	copy(buf[0:4], "RIFF")
	tw219vPutU32(buf[4:8], uint32(total-8))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	tw219vPutU32(buf[16:20], 16)
	tw219vPutU16(buf[20:22], 1)
	tw219vPutU16(buf[22:24], uint16(channels))
	tw219vPutU32(buf[24:28], uint32(sampleRate))
	tw219vPutU32(buf[28:32], uint32(byteRate))
	tw219vPutU16(buf[32:34], uint16(blockAlign))
	tw219vPutU16(buf[34:36], uint16(bitsPerSample))

	pos := 36
	if padBytes > 0 {
		copy(buf[pos:pos+4], "LIST")
		tw219vPutU32(buf[pos+4:pos+8], uint32(padBytes))
		// charge utile laissée à zéro — seule sa TAILLE compte ici.
		pos += 8 + padBytes
	}
	copy(buf[pos:pos+4], "data")
	tw219vPutU32(buf[pos+4:pos+8], uint32(dataLen))
	// le contenu du data chunk est laissé à zéro (silence numérique) — seule
	// sa TAILLE compte pour ces tests de validation de format/durée.
	return buf
}

func tw219vPutU32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func tw219vPutU16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

// tw219vCanonicalDataLen returns how many canonical-format PCM bytes a clip
// of the given duration weighs — the same arithmetic validate.go's own
// pcmDuration inverts.
func tw219vCanonicalDataLen(d time.Duration) int {
	return int(d.Seconds() * float64(SampleRate) * float64(FrameSize))
}

// ---------------------------------------------------------------------------
// Limites de durée et de taille (contract §10.4 : "30 s / 6 Mio").
// ---------------------------------------------------------------------------

func TestValidateQuestionSound_Accepts30Seconds(t *testing.T) {
	data := tw219vBuildWAV(t, ChannelCount, SampleRate, BitsPerSample, tw219vCanonicalDataLen(MaxQuestionSoundDuration), 0)
	result, err := ValidateQuestionSound(data)
	if err != nil {
		t.Fatalf("un son de exactement %s (la limite) doit être accepté (contract §10.4) : %v", MaxQuestionSoundDuration, err)
	}
	if result.Duration > MaxQuestionSoundDuration+50*time.Millisecond {
		t.Errorf("durée calculée %s incohérente avec un fichier de %s", result.Duration, MaxQuestionSoundDuration)
	}
}

func TestValidateQuestionSound_Refuses31Seconds(t *testing.T) {
	over := MaxQuestionSoundDuration + time.Second
	data := tw219vBuildWAV(t, ChannelCount, SampleRate, BitsPerSample, tw219vCanonicalDataLen(over), 0)
	if len(data) >= MaxQuestionSoundBytes {
		t.Fatalf("setup invalide : le fixture de %s dépasse déjà le plafond de taille (%d octets) — ce test doit isoler le refus par DURÉE, pas par taille", over, MaxQuestionSoundBytes)
	}
	if _, err := ValidateQuestionSound(data); err == nil {
		t.Fatalf("un son de %s doit être refusé — la limite est %s (contract §10.4)", over, MaxQuestionSoundDuration)
	}
}

// ---------------------------------------------------------------------------
// Format canonique, réutilisant extractCanonicalPCM (contract §3, §10.4).
// ---------------------------------------------------------------------------

func TestValidateQuestionSound_Refuses48kHzMono(t *testing.T) {
	// Deux non-conformités à la fois (fréquence ET canaux), exactement le
	// cas illustré par la maquette rév.3 §01 ("Ce WAV est en 48 000 Hz
	// mono").
	data := tw219vBuildWAV(t, 1, 48000, BitsPerSample, tw219vCanonicalDataLen(5*time.Second), 0)
	if _, err := ValidateQuestionSound(data); err == nil {
		t.Fatal("un WAV 48 000 Hz mono doit être refusé — format non canonique (contract §3, réutilisé par §10.4)")
	}
}

// TestValidateQuestionSound_AcceptsMono_UpmixedToStereo verrouille
// l'arbitrage QUALIF v11.1 (2026-09-22, retour utilisateur) : à la
// différence des cues (contract §3, INCHANGÉ — extractCanonicalPCM,
// bank.go, reste strictement stéréo), un WAV mono DOIT être accepté pour
// le son de question, puis suréchantillonné en stéréo (chaque échantillon
// dupliqué sur les deux voies) — jamais refusé, jamais stocké mono.
//
// Remplace l'ancien TestValidateQuestionSound_RefusesMono (le mono passe
// désormais du côté accepté).
func TestValidateQuestionSound_AcceptsMono_UpmixedToStereo(t *testing.T) {
	const duration = 5 * time.Second
	// ⚠️ Longueur MONO, pas tw219vCanonicalDataLen (qui calcule une taille
	// STÉRÉO, FrameSize = 2×BytesPerSample) : un mono de la même durée pèse
	// moitié moins d'octets bruts.
	monoDataLen := int(duration.Seconds() * float64(SampleRate) * float64(BytesPerSample))
	data := tw219vBuildWAV(t, 1, SampleRate, BitsPerSample, monoDataLen, 0)

	result, err := ValidateQuestionSound(data)
	if err != nil {
		t.Fatalf("un WAV mono (1 canal, %d Hz, %d bits) doit être accepté (arbitrage QUALIF v11.1, 2026-09-22) : %v", SampleRate, BitsPerSample, err)
	}

	// Le PCM renvoyé doit être STÉRÉO — deux fois plus d'octets que le mono
	// d'origine, jamais renvoyé mono tel quel (le pilote de lecture,
	// media_oto.go, n'accepte que du stéréo canonique).
	if got, want := len(result.PCM), monoDataLen*2; got != want {
		t.Fatalf("PCM suréchantillonné de taille inattendue : got %d octets, attendu %d (2× le mono d'origine)", got, want)
	}
	// La durée calculée doit rester celle du clip d'ORIGINE — le
	// suréchantillonnage double le nombre d'octets ET le nombre de voies en
	// même temps, donc ne doit PAS changer la durée perçue.
	if diff := result.Duration - duration; diff < -10*time.Millisecond || diff > 10*time.Millisecond {
		t.Fatalf("durée calculée %s incohérente avec un mono de %s — le suréchantillonnage ne doit jamais changer la durée perçue", result.Duration, duration)
	}

	// Le fichier RÉELLEMENT STOCKÉ (BuildCanonicalWAV, ce que
	// internal/server/http.go écrit sur disque) doit être un WAV canonique
	// stéréo valide — relisible par extractCanonicalPCM (bank.go),
	// EXACTEMENT le parseur qu'utilisent FileBank et le pilote de lecture
	// des cues : preuve que le fichier écrit pour un média de question
	// d'origine mono est indiscernable d'un WAV stéréo natif à la lecture.
	stored := BuildCanonicalWAV(result.PCM)
	replayed, err := extractCanonicalPCM(stored)
	if err != nil {
		t.Fatalf("le fichier reconstruit par BuildCanonicalWAV n'est pas relisible comme un WAV canonique stéréo : %v", err)
	}
	if len(replayed) != len(result.PCM) {
		t.Fatalf("le PCM relu depuis le fichier stocké diffère en taille : got %d, attendu %d", len(replayed), len(result.PCM))
	}
}

// TestUpmixMonoToStereo_DuplicatesEachSampleOntoBothChannels est le test
// UNITAIRE de la primitive elle-même (package audio, accès direct à la
// fonction non exportée) : sur un motif de 3 échantillons distincts et non
// nuls, vérifie que chaque échantillon mono se retrouve identique sur les
// DEUX voies, dans l'ordre, sans permutation ni décalage d'alignement — un
// test sur du silence (comme tw219vBuildWAV le génère par défaut) ne
// pourrait pas distinguer une duplication correcte d'un bug d'alignement,
// tout étant zéro des deux côtés.
func TestUpmixMonoToStereo_DuplicatesEachSampleOntoBothChannels(t *testing.T) {
	mono := []byte{
		0x01, 0x02, // échantillon 0
		0x03, 0x04, // échantillon 1
		0x05, 0x06, // échantillon 2
	}
	want := []byte{
		0x01, 0x02, 0x01, 0x02, // échantillon 0 : gauche puis droite
		0x03, 0x04, 0x03, 0x04, // échantillon 1
		0x05, 0x06, 0x05, 0x06, // échantillon 2
	}

	got := upmixMonoToStereo(mono)
	if len(got) != len(want) {
		t.Fatalf("longueur inattendue : got %d octets, attendu %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("octet %d : got 0x%02x, attendu 0x%02x — got=%v want=%v", i, got[i], want[i], got, want)
		}
	}
}

func TestValidateQuestionSound_Refuses24Bits(t *testing.T) {
	data := tw219vBuildWAV(t, ChannelCount, SampleRate, 24, tw219vCanonicalDataLen(5*time.Second), 0)
	if _, err := ValidateQuestionSound(data); err == nil {
		t.Fatal("un WAV 24 bits doit être refusé — le format canonique exige 16 bits (contract §3)")
	}
}

func TestValidateQuestionSound_RefusesNonRIFF(t *testing.T) {
	if _, err := ValidateQuestionSound([]byte("ceci n'est pas un fichier WAV")); err == nil {
		t.Fatal("un flux qui n'est pas un RIFF/WAVE valide doit être refusé")
	}
}

func TestValidateQuestionSound_RefusesOver6MioEvenWithinDurationLimit(t *testing.T) {
	// Contract §10.4 : "6 Mio laisse +18,9 % pour l'en-tête RIFF et
	// d'éventuels chunks LIST/INFO... sans autoriser un fichier abusif" — le
	// plafond de taille protège contre les MÉTADONNÉES, pas seulement
	// contre une durée excessive. Ce fixture reste à EXACTEMENT 30 s de PCM
	// (accepté par la seule règle de durée) mais un chunk LIST de
	// remplissage fait dépasser le plafond de taille du fichier BRUT —
	// isolant le refus par TAILLE de celui par DURÉE, déjà couvert par
	// TestValidateQuestionSound_Refuses31Seconds.
	dataLen := tw219vCanonicalDataLen(MaxQuestionSoundDuration)
	pad := MaxQuestionSoundBytes - dataLen // amène juste au-dessus du plafond, en-têtes comprises
	data := tw219vBuildWAV(t, ChannelCount, SampleRate, BitsPerSample, dataLen, pad)
	if len(data) <= MaxQuestionSoundBytes {
		t.Fatalf("setup invalide : fixture de %d octets ne dépasse pas le plafond de %d", len(data), MaxQuestionSoundBytes)
	}
	if _, err := ValidateQuestionSound(data); err == nil {
		t.Fatalf("un fichier de %d octets (> %d, le plafond) doit être refusé même si son extrait audio ne dure que %s — le plafond de taille couvre l'en-tête/les métadonnées, pas seulement la durée (contract §10.4)", len(data), MaxQuestionSoundBytes, MaxQuestionSoundDuration)
	}
}

// ---------------------------------------------------------------------------
// Les deux jeux de constantes NE DOIVENT JAMAIS être confondus (contract
// §10.4, avertissement normatif).
// ---------------------------------------------------------------------------

// TestSentinel_CueSoundConstants_UnchangedByQuestionSoundLimits est le
// test-sentinelle explicitement exigé par contract §10.4 : « MaxSoundDuration
// (5 s) et MaxUploadBytes (2 Mio) ne sont PAS modifiées [...] Un
// test-sentinelle vérifie que les valeurs des cues sont inchangées :
// "factoriser" les deux jeux en un seul est le défaut le plus probable de ce
// lot, et il supprimerait silencieusement la garantie du §5.2. » Ce test ne
// prouve rien sur le nouveau code — il verrouille l'ANCIEN, contre une
// régression que ce lot est structurellement susceptible d'introduire.
func TestSentinel_CueSoundConstants_UnchangedByQuestionSoundLimits(t *testing.T) {
	if MaxSoundDuration != 5*time.Second {
		t.Fatalf("MaxSoundDuration (limite des CUES, validate.go) a changé : %s — attendu 5s. Les deux jeux de constantes cues/média doivent rester DISTINCTS (contract §10.4)", MaxSoundDuration)
	}
	if MaxUploadBytes != 2<<20 {
		t.Fatalf("MaxUploadBytes (limite des CUES, validate.go) a changé : %d — attendu %d (2 Mio). Les deux jeux de constantes cues/média doivent rester DISTINCTS (contract §10.4)", MaxUploadBytes, 2<<20)
	}
	if MaxQuestionSoundDuration == MaxSoundDuration {
		t.Fatal("MaxQuestionSoundDuration ne doit JAMAIS être égale à MaxSoundDuration (cues) — si c'est le cas, les deux jeux de constantes ont probablement été fusionnés par erreur (contract §10.4)")
	}
	if MaxQuestionSoundBytes == MaxUploadBytes {
		t.Fatal("MaxQuestionSoundBytes ne doit JAMAIS être égal à MaxUploadBytes (cues) — si c'est le cas, les deux jeux de constantes ont probablement été fusionnés par erreur (contract §10.4)")
	}
}

func TestQuestionSoundLimits_MatchContractValues(t *testing.T) {
	if MaxQuestionSoundDuration != 30*time.Second {
		t.Errorf("MaxQuestionSoundDuration = %s, attendu 30s (contract §10.4, arbitrage GATE 2 du 2026-09-22)", MaxQuestionSoundDuration)
	}
	if MaxQuestionSoundBytes != 6<<20 {
		t.Errorf("MaxQuestionSoundBytes = %d, attendu %d (6 Mio, contract §10.4)", MaxQuestionSoundBytes, 6<<20)
	}
}
