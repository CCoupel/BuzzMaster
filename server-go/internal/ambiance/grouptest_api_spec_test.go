package ambiance

// SURFACE D'API ATTENDUE pour l'essai de groupe en séquence (contracts/light-effects.md §2.4, révision du GATE LOT-1).
// Le contrat impose le mécanisme (état d'essai porté par Controller, lu par Scene(), NotifyState à chaque pas, durée de pas
// injectable) mais pas les noms : tout appel dépendant de la forme est isolé ICI ; dev-backend n'adapte que ce fichier.
//
// Forme supposée :
//
//	func (c *Controller) StartGroupTest(ref string) error   // ref : "2" (salle n°2) ou "host1" (animateur de rang 1) ; erreur si inconnu/vide
//	func (c *Controller) StopGroupTest()
//	Controller.GroupTestStep time.Duration                   // 0 = 1000 ms (production) ; l'avancement se lit sur c.EffectsClock
//
// Le pas courant est une fonction du temps écoulé depuis le démarrage, lu à l'horloge injectée : Scene() est déterministe.

import (
	"time"
)

func tw225StartTest(c *Controller, ref string) error { return c.StartGroupTest(ref) }
func tw225StopTest(c *Controller)                    { c.StopGroupTest() }
func tw225SetStep(c *Controller, d time.Duration)    { c.GroupTestStep = d }
