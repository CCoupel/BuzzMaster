package effects

// SURFACE D'API ATTENDUE par les tests du magasin de fichiers (v11.8.0, LOT-1, #305 — contrat light-effects.md §11.4 à §11.6).
// Le contrat fixe les règles et les codes (`reason`), pas les signatures Go de B1.1 (store.go) : tout appel dépendant de la forme
// est isolé ICI. Si dev-backend nomme autrement, il n'adapte que ce fichier ; store_spec_test.go ne change pas.
//
// Forme supposée (méthodes de *Manager, sérialisées par son verrou, suivies d'un rechargement) :
//
//	func (m *Manager) WriteEffect(id, text string, create bool) (file string, err error) // §11.4 : nom de fichier, atomique
//	func (m *Manager) DeleteEffect(id string) error                                      // §11.5
//	func (m *Manager) EffectText(id string) (string, error)                              // §11.2 / §11.6 : texte du fichier
//	func RewriteID(text, newID string) (string, error)                                   // §11.7 « Garder les deux » : seule la valeur de `id`
//
// Les erreurs portent leur `reason` : `interface{ Reason() string }` avec « native », « exists », « limit », « reserved »,
// « too_big », « invalid », « unknown » (404) — l'adaptateur ci-dessous les lit.

import (
	"errors"
	"fmt"
)

type reasoned interface{ Reason() string }

// reasonOf : "" si err == nil ; sinon le `reason` de l'erreur ("?" si elle n'en porte pas).
func reasonOf(err error) string {
	if err == nil {
		return ""
	}
	var r reasoned
	if errors.As(err, &r) {
		return r.Reason()
	}
	return "?:" + fmt.Sprint(err)
}

func storeWrite(m *Manager, id, text string, create bool) (file, reason string) {
	f, err := m.WriteEffect(id, text, create)
	return f, reasonOf(err)
}

func storeDelete(m *Manager, id string) string { return reasonOf(m.DeleteEffect(id)) }

func storeText(m *Manager, id string) (string, string) {
	s, err := m.EffectText(id)
	return s, reasonOf(err)
}

func storeRewriteID(text, newID string) (string, error) { return RewriteID(text, newID) }
