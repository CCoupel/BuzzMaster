// #145 (v12.0.0, LOT-1.1) — plateau ROUE : repli, mise en lignes 14 × 4, décompte des lettres.
// Source : contracts/roue.md §3.1 (règle unique côté serveur). Tests de spécification (boîte noire),
// via l'adaptateur roue_board_adapter_145_test.go.
package game

import (
	"reflect"
	"strings"
	"testing"
)

func eqRows(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lignes = %q, attendu %q", got, want)
	}
}

func mustOK(t *testing.T, phrase string) roueLayoutResult {
	t.Helper()
	r := roueLayout(phrase)
	if r.ErrCode != "" {
		t.Fatalf("phrase %q : refus inattendu %s (%s)", phrase, r.ErrCode, r.Description)
	}
	return r
}

func mustFail(t *testing.T, phrase, code string) {
	t.Helper()
	r := roueLayout(phrase)
	if r.ErrCode != code {
		t.Fatalf("phrase %q : code = %q, attendu %q", phrase, r.ErrCode, code)
	}
}

// Exemple de référence du contrat : 3 lignes, 12 consonnes, 17 lettres.
func TestRoueBoard145_ReferenceExample(t *testing.T) {
	r := mustOK(t, "Combien pour ce chien dans la vitrine")
	eqRows(t, r.Rows, []string{"COMBIEN POUR", "CE CHIEN DANS", "LA VITRINE"})
	if r.Consonants != 12 || r.Letters != 17 {
		t.Errorf("consonnes/lettres = %d/%d, attendu 12/17", r.Consonants, r.Letters)
	}
}

func TestRoueBoard145_Layout_Boundaries(t *testing.T) {
	cases := []struct {
		name   string
		phrase string
		rows   []string
	}{
		{"un mot de 14 pile", "ABCDEFGHIJKLMN", []string{"ABCDEFGHIJKLMN"}},
		{"ligne de 14 pile avec espace (7+1+6)", "AAAAAAA BBBBBB", []string{"AAAAAAA BBBBBB"}},
		{"15 avec espace (7+1+7) passe à la ligne", "AAAAAAA BBBBBBB", []string{"AAAAAAA", "BBBBBBB"}},
		{"remplissage glouton, pas d'équilibrage", "AA BBBBBBBBBBBB CC", []string{"AA", "BBBBBBBBBBBB", "CC"}},
		{"4 lignes pile", "AAAAAAAAAAAAAA BBBBBBBBBBBBBB CCCCCCCCCCCCCC DDDDDDDDDDDDDD",
			[]string{"AAAAAAAAAAAAAA", "BBBBBBBBBBBBBB", "CCCCCCCCCCCCCC", "DDDDDDDDDDDDDD"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := mustOK(t, c.phrase)
			if len(r.Rows) > 4 {
				t.Fatalf("plus de 4 lignes : %q", r.Rows)
			}
			for _, row := range r.Rows {
				if len(row) > 14 {
					t.Errorf("ligne de %d colonnes (> 14) : %q", len(row), row)
				}
			}
			eqRows(t, r.Rows, c.rows)
		})
	}
}

func TestRoueBoard145_Refusals(t *testing.T) {
	t.Run("mot de 15 caractères", func(t *testing.T) { mustFail(t, "ABCDEFGHIJKLMNO", "ROUE_PHRASE_WORD_TOO_LONG") })
	t.Run("mot trop long au milieu", func(t *testing.T) { mustFail(t, "BON ABCDEFGHIJKLMNO BON", "ROUE_PHRASE_WORD_TOO_LONG") })
	t.Run("5 lignes", func(t *testing.T) {
		mustFail(t, "AAAAAAAAAAAAAA BBBBBBBBBBBBBB CCCCCCCCCCCCCC DDDDDDDDDDDDDD EEEEEEEEEEEEEE", "ROUE_PHRASE_TOO_LONG")
	})
	t.Run("phrase de la maquette (5 lignes)", func(t *testing.T) {
		mustFail(t, "Il ne faut pas vendre la peau de l'ours avant de l'avoir tué", "ROUE_PHRASE_TOO_LONG")
	})
	t.Run("vide", func(t *testing.T) { mustFail(t, "", "ROUE_PHRASE_EMPTY") })
	t.Run("espaces seuls", func(t *testing.T) { mustFail(t, "    ", "ROUE_PHRASE_EMPTY") })
	for _, bad := range []string{"a_b", "a*b", "a@b", "a/b", "a#b", "a(b)"} {
		bad := bad
		t.Run("caractère interdit "+bad, func(t *testing.T) { mustFail(t, bad, "ROUE_PHRASE_INVALID_CHAR") })
	}
}

func TestRoueBoard145_FoldAccentsAndCase(t *testing.T) {
	cases := map[string]string{
		"été à Noël": "ETE A NOEL",
		"Ça va":      "CA VA",
		"cœur":       "COEUR",
		"ŒUF":        "OEUF",
		"ÉCOLE":      "ECOLE",
		"naïve ÎLE":  "NAIVE ILE",
		"tHe cAsE":   "THE CASE",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) { eqRows(t, mustOK(t, in).Rows, []string{want}) })
	}
}

// Œ occupe 2 colonnes une fois replié (OE) : la limite de 14 s'applique à l'affichage.
func TestRoueBoard145_FoldedWidthCountsForTheLimit(t *testing.T) {
	// 7 × « œ » = 14 colonnes après repli : pile
	mustOK(t, strings.Repeat("œ", 7))
	// 8 × « œ » = 16 colonnes : mot trop long
	mustFail(t, strings.Repeat("œ", 8), "ROUE_PHRASE_WORD_TOO_LONG")
}

func TestRoueBoard145_ApostropheHyphenPunctuationDigits(t *testing.T) {
	eqRows(t, mustOK(t, "L'avoir tué").Rows, []string{"L'AVOIR TUE"})
	eqRows(t, mustOK(t, "Peut-être, non ?").Rows, []string{"PEUT-ETRE, NON", "?"})
	eqRows(t, mustOK(t, "Rue 21 : bis !").Rows, []string{"RUE 21 : BIS !"})
	// l'apostrophe et le tiret font partie du mot (jamais coupés) et comptent dans les 14 colonnes
	mustFail(t, "L'ABCDEFGHIJKLM", "ROUE_PHRASE_WORD_TOO_LONG")
	mustOK(t, "L'ABCDEFGHIJKL")
}

func TestRoueBoard145_SpacesAreCompacted(t *testing.T) {
	eqRows(t, mustOK(t, "   un    deux   trois  ").Rows, []string{"UN DEUX TROIS"})
}

func TestRoueBoard145_DigitsAndPunctuationAreNotLettersToFind(t *testing.T) {
	r := mustOK(t, "Rue 21 : bis !")
	// R,U,E,B,I,S = 6 lettres distinctes ; consonnes R,B,S = 3
	if r.Letters != 6 || r.Consonants != 3 {
		t.Errorf("lettres/consonnes = %d/%d, attendu 6/3", r.Letters, r.Consonants)
	}
}

func TestRoueBoard145_ConsonantsExcludeSixVowelsIncludingY(t *testing.T) {
	r := mustOK(t, "Yo-yo")
	if r.Letters != 2 || r.Consonants != 0 { // Y, O ; Y est une voyelle (jamais proposable)
		t.Errorf("lettres/consonnes = %d/%d, attendu 2/0", r.Letters, r.Consonants)
	}
}

func TestRoueBoard145_DistinctLettersAreCountedOnce(t *testing.T) {
	r := mustOK(t, "Papa maman")
	// P,A,M,N → 4 lettres distinctes ; consonnes P,M,N = 3
	if r.Letters != 4 || r.Consonants != 3 {
		t.Errorf("lettres/consonnes = %d/%d, attendu 4/3", r.Letters, r.Consonants)
	}
}

// Les accents se replient avant le décompte : É et E sont la même lettre.
func TestRoueBoard145_AccentedLettersCountAsTheirBase(t *testing.T) {
	r := mustOK(t, "été")
	if r.Letters != 2 || r.Consonants != 1 { // E, T
		t.Errorf("lettres/consonnes = %d/%d, attendu 2/1", r.Letters, r.Consonants)
	}
}

// Une phrase sans consonne (que des voyelles) est valide : rien à proposer, la manche se joue en réponse directe.
func TestRoueBoard145_VowelOnlyPhraseIsValid(t *testing.T) {
	r := mustOK(t, "Aie")
	if r.Consonants != 0 {
		t.Errorf("consonnes = %d, attendu 0", r.Consonants)
	}
}
