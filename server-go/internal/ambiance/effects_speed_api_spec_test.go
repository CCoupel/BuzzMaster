package ambiance

// SURFACE D'API ATTENDUE pour la vitesse réglable (contrat light-effects.md §6, tâche R13) côté lecteur : la source des
// réglages (EffectsSource) fournit en plus les multiplicateurs RÉSOLUS (global ← manche) :
//
//	EffectSpeeds() (room map[string]float64, host map[string]map[string]float64)   // moment → vitesse ; id de groupe animateur → moment → vitesse
//
// Absent d'une table ⇒ vitesse 1. Si dev-backend choisit une autre forme (méthode, interface optionnelle), il n'adapte
// que ce fichier (et, si l'interface EffectsSource l'exige, les méthodes de tw223Source).

func (s *tw223Source) EffectSpeeds() (map[string]float64, map[string]map[string]float64) {
	return s.speeds, s.hostSpeeds
}

// setSpeed règle la vitesse de la salle pour un moment.
func (e *tw223Env) setSpeed(moment string, v float64) {
	if e.src.speeds == nil {
		e.src.speeds = map[string]float64{}
	}
	e.src.speeds[moment] = v
}

// setHostSpeed règle la vitesse d'un groupe animateur pour un moment.
func (e *tw223Env) setHostSpeed(id, moment string, v float64) {
	if e.src.hostSpeeds == nil {
		e.src.hostSpeeds = map[string]map[string]float64{}
	}
	if e.src.hostSpeeds[id] == nil {
		e.src.hostSpeeds[id] = map[string]float64{}
	}
	e.src.hostSpeeds[id][moment] = v
}
