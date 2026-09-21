// Suite test-writer pour #229 (milestone v11.0 — plan de dev §2.8, points 2
// et 3) : WriteAll (disk.go) couvre à la fois la génération conditionnelle
// au démarrage (overwrite=false, plan §2.4/B.4) et la restauration
// idempotente (overwrite=true, plan B.5, modèle FirmwareManager.
// RestoreEmbedded) — ce fichier teste les deux régimes du même point
// d'entrée.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package synth

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// §2.8 point 2 — Démarrage (overwrite=false).
// ---------------------------------------------------------------------------

// TestWriteAll_FirstRun_CreatesAllSevenFiles is §2.8 point 2's "les 7
// fichiers sont créés au premier lancement."
func TestWriteAll_FirstRun_CreatesAllSevenFiles(t *testing.T) {
	dir := t.TempDir()
	written, err := WriteAll(dir, false)
	if err != nil {
		t.Fatalf("WriteAll a échoué : %v", err)
	}
	if len(written) != len(Cues) {
		t.Fatalf("WriteAll a écrit %d fichier(s), attendu %d (un par cue du catalogue)", len(written), len(Cues))
	}
	for _, c := range Cues {
		path := filepath.Join(dir, Filename(c))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s : fichier absent après le premier lancement : %v", Filename(c), err)
			continue
		}
		want, ok := Generate(c)
		if !ok {
			t.Fatalf("setup invalide : Generate(%s) a renvoyé ok=false", c)
		}
		if !bytes.Equal(data, want) {
			t.Errorf("%s : contenu sur disque différent de Generate(%s)", Filename(c), c)
		}
	}
}

// TestWriteAll_SecondRun_NeverOverwrites is §2.8 point 2's "un second
// lancement n'écrase rien."
func TestWriteAll_SecondRun_NeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteAll(dir, false); err != nil {
		t.Fatalf("premier WriteAll a échoué : %v", err)
	}

	// Capture les mtimes après le premier lancement.
	mtimes := map[string]int64{}
	for _, c := range Cues {
		info, err := os.Stat(filepath.Join(dir, Filename(c)))
		if err != nil {
			t.Fatalf("setup invalide : %v", err)
		}
		mtimes[Filename(c)] = info.ModTime().UnixNano()
	}

	written, err := WriteAll(dir, false)
	if err != nil {
		t.Fatalf("second WriteAll a échoué : %v", err)
	}
	if len(written) != 0 {
		t.Errorf("le second WriteAll(overwrite=false) a réécrit %d fichier(s), attendu 0 : %v", len(written), written)
	}
	for _, c := range Cues {
		info, err := os.Stat(filepath.Join(dir, Filename(c)))
		if err != nil {
			t.Fatalf("%s a disparu après le second lancement : %v", Filename(c), err)
		}
		if info.ModTime().UnixNano() != mtimes[Filename(c)] {
			t.Errorf("%s : mtime a changé après un second WriteAll(overwrite=false) — le fichier a été réécrit alors qu'il existait déjà", Filename(c))
		}
	}
}

// TestWriteAll_CustomFileSurvivesASecondRun is §2.8 point 2's "un fichier
// personnalisé survit à un redémarrage du serveur" — simulé ici par un
// contenu arbitraire (#230's future upload) à la place du son généré,
// suivi d'un second appel à WriteAll(dir, false) comme le ferait un
// redémarrage du serveur (B.4, génération conditionnelle appelée à chaque
// démarrage, jamais seulement "la première fois" au sens processus).
func TestWriteAll_CustomFileSurvivesASecondRun(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteAll(dir, false); err != nil {
		t.Fatalf("premier WriteAll a échoué : %v", err)
	}

	customized := Filename(Cues[0])
	customContent := []byte("un contenu WAV entièrement différent, simulant un fichier personnalisé par l'utilisateur (#230)")
	if err := os.WriteFile(filepath.Join(dir, customized), customContent, 0644); err != nil {
		t.Fatalf("setup invalide : %v", err)
	}

	if _, err := WriteAll(dir, false); err != nil {
		t.Fatalf("second WriteAll (simulant un redémarrage) a échoué : %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, customized))
	if err != nil {
		t.Fatalf("%s a disparu après le redémarrage simulé : %v", customized, err)
	}
	if !bytes.Equal(got, customContent) {
		t.Errorf("%s a été écrasé par un redémarrage simulé — le contenu personnalisé doit survivre (§2.8 point 2), got %q", customized, got)
	}
}

// TestWriteAll_MissingFileIsStillCreated_EvenWithOthersCustomized proves
// WriteAll(false) operates PER FILE, not as an all-or-nothing gate: if one
// of the 7 is missing (e.g. deleted) while others are present/customised,
// only the missing one gets (re)created.
func TestWriteAll_MissingFileIsStillCreated_EvenWithOthersCustomized(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteAll(dir, false); err != nil {
		t.Fatalf("premier WriteAll a échoué : %v", err)
	}
	missing := Filename(Cues[0])
	if err := os.Remove(filepath.Join(dir, missing)); err != nil {
		t.Fatalf("setup invalide : %v", err)
	}

	written, err := WriteAll(dir, false)
	if err != nil {
		t.Fatalf("WriteAll a échoué : %v", err)
	}
	if len(written) != 1 || written[0] != Cues[0] {
		t.Errorf("attendu exactement [%s] réécrit, got %v", Cues[0], written)
	}
	if _, err := os.Stat(filepath.Join(dir, missing)); err != nil {
		t.Errorf("%s aurait dû être recréé : %v", missing, err)
	}
}

// ---------------------------------------------------------------------------
// §2.8 point 3 — Restauration (overwrite=true).
// ---------------------------------------------------------------------------

// TestWriteAll_Restore_OverwritesCustomizedFiles is §2.8 point 3's
// "restauration ... écrase explicitement" (à la différence du démarrage) —
// modèle FirmwareManager.RestoreEmbedded : "Unlike InitFromEmbedded, it
// always overwrites the stored firmware regardless of version."
func TestWriteAll_Restore_OverwritesCustomizedFiles(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteAll(dir, false); err != nil {
		t.Fatalf("premier WriteAll a échoué : %v", err)
	}
	customized := Filename(Cues[0])
	if err := os.WriteFile(filepath.Join(dir, customized), []byte("personnalisé, à écraser par la restauration"), 0644); err != nil {
		t.Fatalf("setup invalide : %v", err)
	}

	written, err := WriteAll(dir, true)
	if err != nil {
		t.Fatalf("WriteAll(overwrite=true) a échoué : %v", err)
	}
	if len(written) != len(Cues) {
		t.Fatalf("la restauration doit réécrire les %d fichiers, got %d : %v", len(Cues), len(written), written)
	}

	got, err := os.ReadFile(filepath.Join(dir, customized))
	if err != nil {
		t.Fatalf("%s : %v", customized, err)
	}
	want, ok := Generate(Cues[0])
	if !ok {
		t.Fatalf("setup invalide : Generate(%s) a renvoyé ok=false", Cues[0])
	}
	if !bytes.Equal(got, want) {
		t.Errorf("la restauration n'a pas rendu exactement les octets d'origine pour %s", customized)
	}
}

// TestWriteAll_Restore_IsIdempotent is §2.8 point 3's "idempotente" —
// deux restaurations successives produisent des octets strictement
// identiques sur disque (conséquence directe du déterminisme du
// générateur, contract §229 §2.1, vérifiée ici à l'échelle du DISQUE,
// pas seulement de la fonction pure Generate).
func TestWriteAll_Restore_IsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteAll(dir, true); err != nil {
		t.Fatalf("première restauration a échoué : %v", err)
	}
	first := map[string][]byte{}
	for _, c := range Cues {
		data, err := os.ReadFile(filepath.Join(dir, Filename(c)))
		if err != nil {
			t.Fatalf("%s : %v", Filename(c), err)
		}
		first[Filename(c)] = data
	}

	if _, err := WriteAll(dir, true); err != nil {
		t.Fatalf("seconde restauration a échoué : %v", err)
	}
	for _, c := range Cues {
		data, err := os.ReadFile(filepath.Join(dir, Filename(c)))
		if err != nil {
			t.Fatalf("%s : %v", Filename(c), err)
		}
		if !bytes.Equal(data, first[Filename(c)]) {
			t.Errorf("%s : deux restaurations successives ont produit des octets différents — restauration non idempotente", Filename(c))
		}
	}
}

// TestWriteAll_EmptyDir_CreatesDirAndAllFiles proves WriteAll creates the
// target directory itself (os.MkdirAll) when it does not exist yet — the
// real first-ever-startup case, distinct from "dir exists but is empty".
func TestWriteAll_EmptyDir_CreatesDirAndAllFiles(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "sounds") // deliberately not created beforehand
	written, err := WriteAll(dir, false)
	if err != nil {
		t.Fatalf("WriteAll a échoué sur un répertoire inexistant : %v", err)
	}
	if len(written) != len(Cues) {
		t.Fatalf("attendu %d fichiers écrits, got %d", len(Cues), len(written))
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("le répertoire cible aurait dû être créé : %v", err)
	}
}
