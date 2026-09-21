// Suite test-writer pour #229 (milestone v11.0 — plan de dev §2.5 B.7) :
// ReconcileManifest (manifest.go) — le manifeste data/files/sounds/sounds.json,
// réconcilié au scan disque, jamais une source de vérité indépendante (plan
// de cadrage §2.2 : "le disque fait foi").
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package synth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestReconcileManifest_AllDefaultFiles_PresentNotCustom proves a freshly
// generated set of default sounds is reported Present=true, Custom=false
// for all 7 cues — the ordinary, most common state.
func TestReconcileManifest_AllDefaultFiles_PresentNotCustom(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteAll(dir, false); err != nil {
		t.Fatalf("setup invalide : %v", err)
	}

	manifest, err := ReconcileManifest(dir)
	if err != nil {
		t.Fatalf("ReconcileManifest a échoué : %v", err)
	}
	if len(manifest) != len(Cues) {
		t.Fatalf("manifeste couvre %d cue(s), attendu %d", len(manifest), len(Cues))
	}
	for _, c := range Cues {
		entry, ok := manifest[c]
		if !ok {
			t.Errorf("%s absent du manifeste", c)
			continue
		}
		if !entry.Present {
			t.Errorf("%s : Present=false pour un fichier réellement généré", c)
		}
		if entry.Custom {
			t.Errorf("%s : Custom=true pour un fichier généré jamais modifié", c)
		}
	}
}

// TestReconcileManifest_CustomizedFile_ReportedCustom proves a file whose
// on-disk bytes differ from what Generate(c) would produce right now is
// reported Custom=true — DERIVED from a byte comparison, never a stored
// flag (manifest.go's own doc comment: "never trusted from a previous
// manifest").
func TestReconcileManifest_CustomizedFile_ReportedCustom(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteAll(dir, false); err != nil {
		t.Fatalf("setup invalide : %v", err)
	}
	customized := Cues[0]
	if err := os.WriteFile(filepath.Join(dir, Filename(customized)), []byte("contenu personnalisé, différent du généré"), 0644); err != nil {
		t.Fatalf("setup invalide : %v", err)
	}

	manifest, err := ReconcileManifest(dir)
	if err != nil {
		t.Fatalf("ReconcileManifest a échoué : %v", err)
	}
	entry := manifest[customized]
	if !entry.Present {
		t.Errorf("%s : Present=false, attendu true (le fichier existe)", customized)
	}
	if !entry.Custom {
		t.Errorf("%s : Custom=false, attendu true (contenu différent du généré)", customized)
	}
}

// TestReconcileManifest_MissingFile_ReportedAbsent proves a cue with no
// file on disk at all is reported Present=false — never a panic, never a
// synthetic "empty" entry pretending the file exists.
func TestReconcileManifest_MissingFile_ReportedAbsent(t *testing.T) {
	dir := t.TempDir() // vide, aucun WriteAll

	manifest, err := ReconcileManifest(dir)
	if err != nil {
		t.Fatalf("ReconcileManifest a échoué sur un répertoire vide : %v", err)
	}
	for _, c := range Cues {
		if manifest[c].Present {
			t.Errorf("%s : Present=true sur un répertoire vide, attendu false", c)
		}
		if manifest[c].Custom {
			t.Errorf("%s : Custom=true sur un fichier absent — n'a de sens que si Present=true", c)
		}
	}
}

// TestReconcileManifest_WritesValidJSONToDisk proves sounds.json is
// actually written to <dir>/sounds.json, and is valid, parseable JSON
// matching the returned map — not just an in-memory result.
func TestReconcileManifest_WritesValidJSONToDisk(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteAll(dir, false); err != nil {
		t.Fatalf("setup invalide : %v", err)
	}
	inMemory, err := ReconcileManifest(dir)
	if err != nil {
		t.Fatalf("ReconcileManifest a échoué : %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "sounds.json"))
	if err != nil {
		t.Fatalf("sounds.json n'a pas été écrit sur disque : %v", err)
	}
	var onDisk map[string]ManifestEntry
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("sounds.json n'est pas un JSON valide : %v", err)
	}
	if len(onDisk) != len(inMemory) {
		t.Fatalf("sounds.json sur disque contient %d entrée(s), attendu %d (résultat en mémoire)", len(onDisk), len(inMemory))
	}
	for c, entry := range inMemory {
		diskEntry, ok := onDisk[string(c)]
		if !ok {
			t.Errorf("%s absent de sounds.json sur disque", c)
			continue
		}
		if diskEntry.Present != entry.Present || diskEntry.Custom != entry.Custom {
			t.Errorf("%s : sounds.json sur disque (%+v) diffère du résultat en mémoire (%+v)", c, diskEntry, entry)
		}
	}
}

// TestReconcileManifest_IdempotentWhenNothingChanges proves calling it
// twice, with no change to the sounds directory in between, produces
// byte-identical sounds.json content — a stale-looking diff on every
// restart (even with nothing customised) would be a real regression, since
// the manifest is meant to be a pure reconciliation of disk state.
func TestReconcileManifest_IdempotentWhenNothingChanges(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteAll(dir, false); err != nil {
		t.Fatalf("setup invalide : %v", err)
	}
	if _, err := ReconcileManifest(dir); err != nil {
		t.Fatalf("première réconciliation a échoué : %v", err)
	}
	first, err := os.ReadFile(filepath.Join(dir, "sounds.json"))
	if err != nil {
		t.Fatalf("%v", err)
	}

	if _, err := ReconcileManifest(dir); err != nil {
		t.Fatalf("seconde réconciliation a échoué : %v", err)
	}
	second, err := os.ReadFile(filepath.Join(dir, "sounds.json"))
	if err != nil {
		t.Fatalf("%v", err)
	}

	if string(first) != string(second) {
		t.Errorf("sounds.json a changé entre deux réconciliations sans aucune modification du répertoire — attendu identique\navant : %s\naprès : %s", first, second)
	}
}
