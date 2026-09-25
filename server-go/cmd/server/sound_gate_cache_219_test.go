// Suite test-writer pour l'addendum v11.1 « Média indisponible = lancement
// bloqué » (#219/#236/#237, plan _work/reports/plan-20260923-101500.md
// rév. 8 §6 tâche 11, contract sound.md §10.8.3) : le cache de validation
// de questionSoundAdapter.validateSoundFile (cmd/server/question_sound.go)
// — empreinte mtime+taille, hit sans relecture, verdict négatif mis en
// cache, entrée supprimée si le fichier disparaît puis revalidée s'il
// revient.
//
// Technique de preuve (sans compteur intrusif ni modification de
// question_sound.go) : après une première validation, le fichier est
// RÉÉCRIT avec un contenu DIFFÉRENT (valide↔corrompu) tout en forçant
// délibérément la MÊME empreinte (mtime+taille, via os.Chtimes) ou une
// empreinte DIFFÉRENTE selon ce que le test veut démontrer. Si le verdict
// renvoyé par le second appel correspond au contenu D'ORIGINE malgré le
// changement, c'est la preuve comportementale qu'aucune relecture n'a eu
// lieu — plus robuste qu'un compteur d'appels, qui ne prouverait que
// l'implémentation actuelle, pas la garantie observable.
//
// Convention de collision : préfixe tw219c pour ne jamais entrer en
// collision avec un helper d'un autre fichier de ce paquet.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"buzzcontrol/internal/audio"
	"buzzcontrol/internal/game"
)

// tw219cBuildWAV hand-builds a RIFF/WAVE byte stream — même technique que
// les autres suites test-writer du son (ex. internal/server/
// question_sound_upload_219_test.go), dupliquée localement pour ne pas
// entrer en collision avec elles (paquet différent de toute façon).
func tw219cBuildWAV(channels, sampleRate, bitsPerSample, dataLen int) []byte {
	byteRate := sampleRate * channels * (bitsPerSample / 8)
	blockAlign := channels * (bitsPerSample / 8)
	buf := make([]byte, 44+dataLen)
	copy(buf[0:4], "RIFF")
	tw219cPutU32(buf[4:8], uint32(36+dataLen))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	tw219cPutU32(buf[16:20], 16)
	tw219cPutU16(buf[20:22], 1)
	tw219cPutU16(buf[22:24], uint16(channels))
	tw219cPutU32(buf[24:28], uint32(sampleRate))
	tw219cPutU32(buf[28:32], uint32(byteRate))
	tw219cPutU16(buf[32:34], uint16(blockAlign))
	tw219cPutU16(buf[34:36], uint16(bitsPerSample))
	copy(buf[36:40], "data")
	tw219cPutU32(buf[40:44], uint32(dataLen))
	return buf
}

func tw219cPutU32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func tw219cPutU16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

// tw219cValidWAV returns a canonical, VALID WAV of the given duration.
func tw219cValidWAV(seconds int) []byte {
	dataLen := seconds * audio.SampleRate * audio.ChannelCount * audio.BitsPerSample / 8
	return tw219cBuildWAV(audio.ChannelCount, audio.SampleRate, audio.BitsPerSample, dataLen)
}

// tw219cCorruptCopy returns a byte-for-byte copy of valid EXCEPT its RIFF
// magic is scrambled — same LENGTH (so a size-based fingerprint alone
// cannot distinguish the two), but extractCanonicalPCM/ValidateQuestionSound
// refuses it outright.
func tw219cCorruptCopy(valid []byte) []byte {
	corrupt := append([]byte(nil), valid...)
	copy(corrupt[0:4], "XXXX")
	return corrupt
}

// tw219cWriteFileAt writes data to path and forces its mtime to exactly
// mtime (os.Chtimes) — the deterministic control needed to isolate
// "fingerprint unchanged" from "fingerprint changed" scenarios, since two
// writes in quick succession could otherwise land on the same real-clock
// mtime by accident (or a different one, depending on filesystem
// granularity) — never left to chance here.
func tw219cWriteFileAt(t *testing.T, path string, data []byte, mtime time.Time) {
	t.Helper()
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("setup invalide : écriture de %s a échoué : %v", path, err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("setup invalide : os.Chtimes(%s) a échoué : %v", path, err)
	}
}

// ---------------------------------------------------------------------------
// Hit — verdict POSITIF réutilisé sans relecture du contenu.
// ---------------------------------------------------------------------------

func TestValidateSoundFile_Hit_PositiveVerdict_SurvivesContentChange_SameFingerprint(t *testing.T) {
	qs := &questionSoundAdapter{}
	dir := t.TempDir()
	path := filepath.Join(dir, "sound.wav")
	mtime := time.Now()

	valid := tw219cValidWAV(3)
	tw219cWriteFileAt(t, path, valid, mtime)

	if reason := qs.validateSoundFile(path); reason != "" {
		t.Fatalf("setup invalide : un WAV valide doit être accepté, got reason=%q", reason)
	}

	// Contenu remplacé par une version CORROMPUE, mais taille ET mtime
	// forcés IDENTIQUES (même empreinte) — un vrai re-parsage détecterait
	// la corruption ; un HIT ne le peut pas, par construction.
	corrupt := tw219cCorruptCopy(valid)
	tw219cWriteFileAt(t, path, corrupt, mtime)

	if reason := qs.validateSoundFile(path); reason != "" {
		t.Fatalf("HIT attendu (empreinte mtime+taille inchangée) : le verdict positif mis en cache doit être réutilisé SANS relire le contenu — got reason=%q alors que le fichier est maintenant corrompu", reason)
	}
}

// ---------------------------------------------------------------------------
// Hit — verdict NÉGATIF mis en cache lui aussi (le piège le plus probable,
// CA24).
// ---------------------------------------------------------------------------

func TestValidateSoundFile_Hit_NegativeVerdict_AlsoCached_SurvivesContentFix_SameFingerprint(t *testing.T) {
	qs := &questionSoundAdapter{}
	dir := t.TempDir()
	path := filepath.Join(dir, "sound.wav")
	mtime := time.Now()

	valid := tw219cValidWAV(3)
	corrupt := tw219cCorruptCopy(valid)
	tw219cWriteFileAt(t, path, corrupt, mtime)

	if reason := qs.validateSoundFile(path); reason != "FILE" {
		t.Fatalf("setup invalide : un WAV corrompu doit être refusé (FILE), got reason=%q", reason)
	}

	// Le fichier est "réparé" (contenu valide restauré), mais l'empreinte
	// (mtime+taille) reste IDENTIQUE — un verdict négatif non mis en cache
	// re-parserait et verrait le contenu réparé ; le cache, lui, doit
	// continuer à répondre FILE tant que l'empreinte n'a pas changé.
	tw219cWriteFileAt(t, path, valid, mtime)

	if reason := qs.validateSoundFile(path); reason != "FILE" {
		t.Fatalf("CA24 violé : le verdict NÉGATIF doit être mis en cache exactement comme un positif — un second appel à empreinte inchangée doit rester FILE, got reason=%q (le contenu a été relu alors qu'il n'aurait pas dû l'être)", reason)
	}
}

// ---------------------------------------------------------------------------
// Invalidation — mtime différent.
// ---------------------------------------------------------------------------

func TestValidateSoundFile_MtimeChanged_Invalidates(t *testing.T) {
	qs := &questionSoundAdapter{}
	dir := t.TempDir()
	path := filepath.Join(dir, "sound.wav")

	valid := tw219cValidWAV(3)
	t1 := time.Now().Add(-time.Hour)
	tw219cWriteFileAt(t, path, valid, t1)
	if reason := qs.validateSoundFile(path); reason != "" {
		t.Fatalf("setup invalide : got reason=%q", reason)
	}

	// Contenu corrompu, TAILLE identique (tw219cCorruptCopy le garantit),
	// mais mtime DIFFÉRENT — doit être traité comme un MISS.
	corrupt := tw219cCorruptCopy(valid)
	t2 := t1.Add(time.Minute)
	tw219cWriteFileAt(t, path, corrupt, t2)

	if reason := qs.validateSoundFile(path); reason != "FILE" {
		t.Errorf("un mtime différent doit invalider le cache et déclencher une revalidation — got reason=%q, attendu FILE (le contenu est maintenant corrompu)", reason)
	}
}

// ---------------------------------------------------------------------------
// Invalidation — taille différente (mtime identique).
// ---------------------------------------------------------------------------

func TestValidateSoundFile_SizeChanged_Invalidates_EvenWithSameMtime(t *testing.T) {
	qs := &questionSoundAdapter{}
	dir := t.TempDir()
	path := filepath.Join(dir, "sound.wav")
	mtime := time.Now()

	valid3s := tw219cValidWAV(3)
	tw219cWriteFileAt(t, path, valid3s, mtime)
	if reason := qs.validateSoundFile(path); reason != "" {
		t.Fatalf("setup invalide : got reason=%q", reason)
	}

	// Remplacé par un WAV plus long (taille différente), mtime forcé
	// IDENTIQUE — la taille seule doit suffire à invalider, indépendamment
	// du mtime.
	valid5s := tw219cValidWAV(5)
	if len(valid5s) == len(valid3s) {
		t.Fatal("setup invalide : les deux fixtures doivent avoir des tailles différentes")
	}
	tw219cWriteFileAt(t, path, valid5s, mtime)

	// Les deux étant valides, on ne peut pas distinguer un HIT stale d'un
	// vrai MISS par le verdict seul (les deux WAV sont acceptés) — ce test
	// vérifie donc l'invalidation avec un fichier qui DEVIENT invalide.
	corrupt5s := tw219cCorruptCopy(valid5s)
	tw219cWriteFileAt(t, path, corrupt5s, mtime)

	if reason := qs.validateSoundFile(path); reason != "FILE" {
		t.Errorf("une taille différente (même mtime que l'entrée en cache) doit invalider et déclencher une revalidation — got reason=%q, attendu FILE", reason)
	}
}

// ---------------------------------------------------------------------------
// Fichier disparu — entrée supprimée, puis revalidée s'il revient.
// ---------------------------------------------------------------------------

func TestValidateSoundFile_FileDisappears_EntryDeleted_ThenRevalidatedIfRestored(t *testing.T) {
	qs := &questionSoundAdapter{}
	dir := t.TempDir()
	path := filepath.Join(dir, "sound.wav")
	mtime := time.Now()

	valid := tw219cValidWAV(3)
	tw219cWriteFileAt(t, path, valid, mtime)
	if reason := qs.validateSoundFile(path); reason != "" {
		t.Fatalf("setup invalide : got reason=%q", reason)
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("setup invalide : suppression du fichier a échoué : %v", err)
	}
	if reason := qs.validateSoundFile(path); reason != "FILE" {
		t.Fatalf("un fichier disparu doit être refusé (FILE), got reason=%q", reason)
	}

	// Restauré à un chemin IDENTIQUE, avec un mtime naturellement différent
	// (nouvelle écriture) — l'entrée doit avoir été supprimée (pas
	// seulement marquée mauvaise), donc ce nouvel appel doit revalider
	// entièrement plutôt que de faire confiance à un verdict FILE périmé.
	tw219cWriteFileAt(t, path, valid, time.Now())
	if reason := qs.validateSoundFile(path); reason != "" {
		t.Errorf("un fichier restauré (valide) doit être revalidé et accepté — got reason=%q, attendu \"\" (l'entrée FILE périmée ne doit pas persister)", reason)
	}
}

// ---------------------------------------------------------------------------
// Nil-safety — plusieurs tests de ce paquet construisent
// questionSoundAdapter{} directement (validationCache nil).
// ---------------------------------------------------------------------------

func TestValidateSoundFile_NilCache_NeverPanics(t *testing.T) {
	qs := &questionSoundAdapter{} // validationCache est nil ici, à dessein
	dir := t.TempDir()
	path := filepath.Join(dir, "sound.wav")
	tw219cWriteFileAt(t, path, tw219cValidWAV(1), time.Now())

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("validateSoundFile ne doit jamais paniquer sur un cache nil, got: %v", r)
		}
	}()
	if reason := qs.validateSoundFile(path); reason != "" {
		t.Errorf("got reason=%q, attendu \"\"", reason)
	}
}

// ---------------------------------------------------------------------------
// questionSoundAvailability — intégration avec le cache pour le motif FILE.
// ---------------------------------------------------------------------------

func TestQuestionSoundAvailability_FILE_UsesTheCache(t *testing.T) {
	app := newTestApp(t)
	fake := &tw219qFakeMediaPlayer{}
	app.qsound = &questionSoundAdapter{app: app, player: fake, validationCache: make(map[string]soundValidationEntry)}
	tw219qSetSoundEnabled(true)

	dir := t.TempDir()
	app.config.Storage.QuestionsDir = dir
	if err := os.MkdirAll(filepath.Join(dir, "q1"), 0755); err != nil {
		t.Fatalf("setup invalide : %v", err)
	}
	soundPath := filepath.Join(dir, "q1", "sound_1234.wav")
	mtime := time.Now()
	tw219cWriteFileAt(t, soundPath, tw219cValidWAV(3), mtime)

	q := &game.Question{ID: "q1", Sound: "/question/q1/sound_1234.wav"}

	ok, reason := app.qsound.questionSoundAvailability(q)
	if !ok || reason != "" {
		t.Fatalf("un son valide doit être disponible, got ok=%v reason=%q", ok, reason)
	}

	// Corrompt le fichier, empreinte forcée identique — questionSoundAvailability
	// doit rester "disponible" (hit du cache), preuve que la gate T0
	// réutilise bien le même mécanisme de cache que validateSoundFile seul.
	corrupt := tw219cCorruptCopy(tw219cValidWAV(3))
	tw219cWriteFileAt(t, soundPath, corrupt, mtime)

	ok, reason = app.qsound.questionSoundAvailability(q)
	if !ok || reason != "" {
		t.Errorf("HIT attendu au niveau questionSoundAvailability aussi — got ok=%v reason=%q, attendu disponible malgré la corruption (empreinte inchangée)", ok, reason)
	}
}
