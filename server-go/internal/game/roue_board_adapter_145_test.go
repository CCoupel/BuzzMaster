// #145 (v12.0.0, LOT-1.1) — adaptateur unique entre les tests du plateau ROUE et l'API Go réelle.
// Écrit AVANT le code (contract-first, contracts/roue.md §3.1) : si dev-backend nomme autrement la
// fonction de mise en lignes, SEUL ce fichier est à réaligner (une ligne), pas les cas de test.
package game

// roueLayoutResult est la vue minimale dont les tests ont besoin (contrat §3.1 / §11.2).
type roueLayoutResult struct {
	Rows        []string // lignes affichées, sans remplissage, majuscules sans accent
	Consonants  int      // consonnes distinctes à trouver
	Letters     int      // lettres distinctes à trouver
	ErrCode     string   // "" si OK, sinon ROUE_PHRASE_EMPTY | _INVALID_CHAR | _WORD_TOO_LONG | _TOO_LONG
	Description string
}

// roueLayout est le SEUL point d'appel vers l'implémentation (roue_board.go, tâche 2).
// Aligné sur les signatures livrées par dev-backend.
func roueLayout(phrase string) roueLayoutResult {
	// Signature réelle (roue_board.go) : RoueBuildBoard(string) (RoueBoardLayout, error), erreur *RoueError.
	l, err := RoueBuildBoard(phrase)
	if err != nil {
		if re, ok := err.(*RoueError); ok {
			return roueLayoutResult{ErrCode: re.Code, Description: re.Message}
		}
		return roueLayoutResult{ErrCode: "UNTYPED_ERROR", Description: err.Error()}
	}
	return roueLayoutResult{Rows: l.Rows, Consonants: l.Consonants, Letters: l.Letters}
}
