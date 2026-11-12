package server

// SURFACE D'API ATTENDUE par les tests des routes de l'éditeur (v11.8.0, LOT-2 — contrat light-effects.md §11.10).
// Le contrat fixe la route `POST /api/lighting/effects/draft/try` et ses réponses, pas l'interface Go entre le serveur HTTP et le
// contrôleur d'ambiance. Tout ce qui en dépend est isolé ICI ; si dev-backend nomme autrement, il n'adapte que ce fichier.
//
// Forme (confirmée par dev-backend, DraftTrialer) : le contrôleur (srv.Lighting) implémente en plus de EffectTrialer
//
//	StartDraftTrial(e effects.Effect, zone string, speed float64, seconds int) (int, error)
//
// avec les mêmes erreurs de refus (TrialReason()) que StartEffectTrial.
//
// Le fournisseur de catalogue réutilise tw305Provider (http_effects_import_305_spec_test.go) : si l'interface EffectsStore gagne des
// méthodes pour PUT / source, c'est lui qu'il faut compléter.

import (
	"buzzcontrol/internal/effects"
)

type tw210DraftCall struct {
	e       effects.Effect
	zone    string
	speed   float64
	seconds int
}

type tw210Trialer struct {
	*tw223TrialProvider
	drafts []tw210DraftCall
}

func (p *tw210Trialer) StartDraftTrial(e effects.Effect, zone string, speed float64, seconds int) (int, error) {
	if p.err != nil {
		return 0, p.err
	}
	p.drafts = append(p.drafts, tw210DraftCall{e, zone, speed, seconds})
	return seconds * 1000, nil
}

// Rapproché de l'interface réelle (http_effects_editor.go, DraftTrialer) : le double de test l'implémente.
var _ DraftTrialer = (*tw210Trialer)(nil)
