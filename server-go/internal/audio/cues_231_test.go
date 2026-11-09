// #231 (v11.6.0, LOT-3) — vocabulaire des 15 cues : liste fermée, ordre normatif, type par cue.
// Source : contracts/sound.md §2.1 amendement #231 (`audio.AllCues()` : global puis QCM, MEMORY, MEMOTION, RAFALE ;
// `Cue.Type()` : "" pour les 7 cues globales). Cues désignées par leur NOM de contrat. Préfixe tw231a.
package audio

import "testing"

var tw231aWant = []struct{ name, typ string }{
	{"depart", ""}, {"temps-ecoule", ""}, {"gagne", ""}, {"perdu", ""}, {"reveal", ""}, {"entracte-debut", ""}, {"entracte-fin", ""},
	{"qcm-indice", "QCM"},
	{"memory-carte", "MEMORY"}, {"memory-paire", "MEMORY"},
	{"memotion-selection", "MEMOTION"}, {"memotion-fin-carte", "MEMOTION"},
	{"rafale-bonne", "RAFALE"}, {"rafale-mauvaise", "RAFALE"}, {"rafale-fin-lot", "RAFALE"},
}

func TestCues231_AllCues_FifteenInNormativeOrder(t *testing.T) {
	got := AllCues()
	if len(got) != len(tw231aWant) {
		t.Fatalf("AllCues() doit lister 15 cues (7 globales + 8 par type), got %d : %v", len(got), got)
	}
	for i, w := range tw231aWant {
		if string(got[i]) != w.name {
			t.Errorf("position %d : attendu %q (ordre normatif global puis QCM, MEMORY, MEMOTION, RAFALE), got %q", i, w.name, got[i])
		}
	}
}

func TestCues231_Type_PerCue(t *testing.T) {
	for _, w := range tw231aWant {
		if got := Cue(w.name).Type(); got != w.typ {
			t.Errorf("Cue(%q).Type() = %q, attendu %q", w.name, got, w.typ)
		}
	}
}

func TestCues231_NoDuplicates_NoEmptyNames(t *testing.T) {
	seen := map[Cue]bool{}
	for _, c := range AllCues() {
		if c == "" || seen[c] {
			t.Errorf("cue vide ou en doublon : %q", c)
		}
		seen[c] = true
	}
}

// Les sept cues v11.0 gardent leur nom, octet pour octet (non-régression : fichiers, sites, interrupteurs).
func TestCues231_ExistingSevenUnchanged(t *testing.T) {
	for i, w := range tw231aWant[:7] {
		if string(AllCues()[i]) != w.name || w.typ != "" {
			t.Errorf("cue historique %d modifiée : %q", i, AllCues()[i])
		}
	}
	if CueDepart != "depart" || CueTempsEcoule != "temps-ecoule" || CueGagne != "gagne" || CuePerdu != "perdu" ||
		CueReveal != "reveal" || CueEntracteDebut != "entracte-debut" || CueEntracteFin != "entracte-fin" {
		t.Error("les constantes des 7 cues v11.0 ne doivent pas changer")
	}
}

// Une cue hors catalogue n'a pas de type et n'est pas dans la liste fermée.
func TestCues231_UnknownCue_NotInCatalogue(t *testing.T) {
	for _, c := range AllCues() {
		if c == Cue("inconnue") {
			t.Fatal("la liste est fermée : une cue inconnue ne doit pas y figurer")
		}
	}
	if got := Cue("inconnue").Type(); got != "" {
		t.Errorf("Type() d'une cue inconnue : \"\" attendu, got %q", got)
	}
}
