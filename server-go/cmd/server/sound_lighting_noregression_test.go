// Non-régression lumineuse pour #227 (milestone v11.0 — contracts/sound.md,
// amendement de contracts/lighting.md §2.5/§3.2,
// _work/reports/plan-dev-227-20260921-102500.md §3.2).
//
// Le piège que ce fichier désamorce, dans les mots du plan : "Sortir
// COUNTDOWN de KindReady change le comportement lumineux livré en v10.0.0
// [...]. Si le nouveau genre n'a pas d'entrée dans la table de scènes §8,
// l'éclairage change sans que personne ne l'ait demandé." Concrètement :
// lighting.KindCountdown existe déjà (internal/lighting/event.go, ajouté au
// Lot A) mais cmd/server/ambiance.go:ambianceSceneFor (Lot B.6) doit encore
// lui donner une entrée — tant que ce n'est pas fait, ambianceSceneFor
// retombe sur son cas par défaut (ambianceSceneIdle), PAS sur
// ambianceSceneReady. C'est exactement la régression que ce test doit
// attraper à l'exécution, avant toute revue humaine.
//
// Volontairement testé au niveau du RENDU (ambianceSceneFor), pas de la
// dérivation (deriveAmbianceEvent) : la garantie normative du contrat
// ("mappé sur exactement la même scène") porte sur la scène rendue, quelle
// que soit la façon dont Lot B partitionne PREPARE/READY/COUNTDOWN dans
// deriveAmbianceEvent. Ce test reste donc vrai indépendamment de ce choix
// d'implémentation.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun.
package main

import (
	"testing"

	"buzzcontrol/internal/lighting"
)

// TestSoundNoRegression_KindCountdownRendersIdenticallyToKindReady is C.2 —
// contract §3.2 : "KindCountdown doit être mappé sur exactement la même
// scène que KindReady, pour que le rendu lumineux reste identique au bit
// près. La scène propre au décompte appartient à #212 (v10.1), pas à #227."
func TestSoundNoRegression_KindCountdownRendersIdenticallyToKindReady(t *testing.T) {
	ready := ambianceSceneFor(lighting.Event{Kind: lighting.KindReady})
	countdown := ambianceSceneFor(lighting.Event{Kind: lighting.KindCountdown})

	if countdown != ready {
		t.Fatalf("contract §3.2 violé — KindCountdown doit rendre EXACTEMENT la même scène que KindReady (bit à bit) :\n"+
			"  KindReady     = %+v\n"+
			"  KindCountdown = %+v\n"+
			"Une scène de décompte dédiée appartient à #212 (v10.1), pas à #227 — si ambianceSceneFor est tombé sur son cas par défaut (ambianceSceneIdle) faute d'entrée pour lighting.KindCountdown, c'est précisément la régression que ce test protège.",
			ready, countdown)
	}
}

// TestSoundNoRegression_KindCountdownIsNotTheDefaultIdleFallback pins the
// EXACT failure mode described above: if ambianceSceneFor's switch has no
// case for lighting.KindCountdown at all, it silently falls through to
// ambianceSceneIdle — which happens to be a DIFFERENT struct from
// ambianceSceneReady, so the test above would already fail, but this
// second, narrower assertion names the failure unambiguously (missing
// switch case) rather than leaving a reviewer to guess between "wrong
// scene chosen" and "no case at all, fell through to idle".
func TestSoundNoRegression_KindCountdownIsNotTheDefaultIdleFallback(t *testing.T) {
	countdown := ambianceSceneFor(lighting.Event{Kind: lighting.KindCountdown})
	idle := ambianceSceneFor(lighting.Event{Kind: lighting.KindIdle})
	ready := ambianceSceneFor(lighting.Event{Kind: lighting.KindReady})

	if idle == ready {
		t.Skip("setup invalide pour cette assertion précise : ambianceSceneIdle == ambianceSceneReady dans ce build, la distinction idle-vs-ready ne permet plus de nommer le défaut — voir le test frère pour la garantie principale")
	}
	if countdown == idle {
		t.Fatalf("ambianceSceneFor(KindCountdown) est retombé sur le cas par défaut ambianceSceneIdle (%+v) — il manque `case lighting.KindCountdown: return ambianceSceneReady` dans ambianceSceneFor (cmd/server/ambiance.go, Lot B.6)", idle)
	}
}
