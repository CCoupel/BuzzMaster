//go:build linux || windows

// Suite test-writer pour #219 (milestone v11.1 — plan
// _work/reports/plan-20260922-103848.md Phase 0 tâche 1, contract §10.3) :
// le contexte `oto` extrait en SINGLETON DE PACKAGE — exigence normative
// pour que newOtoOutput (le pilote de bruitages, #228) et le backend oto du
// MediaPlayer (#219) partagent le MÊME contexte, jamais deux appels
// distincts à oto.NewContext, que la bibliothèque documente refuser (§1 /
// spike #226 §2 — un seul contexte audio par processus).
//
// sharedOtoContext() (*oto.Context, error), sous sync.Once — exactement la
// forme nommée par le plan de dev (§7, tâche 1) et livrée par dev-backend
// dans output_oto.go.
//
// Build tag identique à output_oto.go et cross_compile_228_test.go :
// sharedOtoContext n'existe que sur les deux cibles réelles du pipeline de
// release.
//
// Convention de collision : préfixe tw219s pour ne jamais entrer en
// collision avec un helper d'un autre fichier de ce paquet.
//
// Non-régression : ce fichier n'ajoute que des tests, n'en modifie aucun ;
// il ne touche à aucune ligne de output_oto.go.
package audio

import (
	"sync"
	"testing"
	"time"

	"github.com/ebitengine/oto/v3"
)

const tw219sGenerousTimeout = 15 * time.Second

// TestSharedOtoContext_TwoCalls_ReturnSamePointer verrouille l'exigence
// normative du contract §10.3 : deux appels rendent le MÊME pointeur de
// contexte (succès ou échec — sync.Once mémorise le résultat, il ne relance
// jamais oto.NewContext une seconde fois), jamais deux instances
// indépendantes.
func TestSharedOtoContext_TwoCalls_ReturnSamePointer(t *testing.T) {
	ctx1, err1 := sharedOtoContext()
	ctx2, err2 := sharedOtoContext()

	if ctx1 != ctx2 {
		t.Fatalf("sharedOtoContext() a renvoyé deux pointeurs DIFFÉRENTS (%p puis %p) — le contexte oto n'est pas un vrai singleton de package (contract §10.3), risque direct de second oto.NewContext refusé par la bibliothèque", ctx1, ctx2)
	}
	if (err1 == nil) != (err2 == nil) {
		t.Fatalf("sharedOtoContext() est incohérent entre deux appels : err1=%v, err2=%v — sync.Once doit mémoriser le MÊME résultat (succès ou échec), jamais retenter", err1, err2)
	}
	t.Logf("sharedOtoContext() -> ctx=%p, err=%v (succès matériel réel ou dégradation, les deux sont valides ici — voir output_228_test.go)", ctx1, err1)
}

// TestSharedOtoContext_ConcurrentCalls_NoPanicNoDeadlock mirrors
// TestNewOutput_ConcurrentConstruction_NoPanicNoDeadlock (output_228_test.go)
// pour le nouveau point d'entrée singleton lui-même : N appels concurrents
// ne doivent ni paniquer ni se bloquer, et doivent tous converger vers le
// MÊME résultat — preuve, au niveau concurrence, de la même exigence que le
// test précédent (sync.Once doit rester correct sous contention, pas
// seulement en séquentiel).
func TestSharedOtoContext_ConcurrentCalls_NoPanicNoDeadlock(t *testing.T) {
	type result struct {
		ctx *oto.Context
		err error
	}
	const n = 8
	results := make(chan result, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("sharedOtoContext a paniqué sous appel concurrent : %v", r)
				}
			}()
			ctx, err := sharedOtoContext()
			results <- result{ctx: ctx, err: err}
		}()
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(tw219sGenerousTimeout):
		t.Fatalf("au moins un appel concurrent à sharedOtoContext ne s'est pas terminé après %s — deadlock probable", tw219sGenerousTimeout)
	}
	close(results)

	var first *result
	got := 0
	for r := range results {
		r := r
		got++
		if first == nil {
			first = &r
			continue
		}
		if r.ctx != first.ctx {
			t.Error("des appels concurrents à sharedOtoContext ont convergé vers des pointeurs DIFFÉRENTS — sync.Once n'a pas fait son office sous contention (contract §10.3)")
		}
	}
	if got != n {
		t.Fatalf("attendu %d résultats, got %d", n, got)
	}
}

// TestOutputAndMediaPlayer_ShareSingletonContext_AgreeOnDegradation est la
// conséquence BLACK-BOX du singleton (contract §10.3) : ce test ne touche
// PAS sharedOtoContext directement, seulement les deux constructeurs
// PUBLICS qui doivent désormais en dépendre tous les deux. Avant ce
// refactor, un second appel à oto.NewContext (aujourd'hui caché dans
// otoOutput) pouvait se comporter différemment du premier — exactement le
// bug que #219 doit éviter pour le nouveau chemin média. Après le
// refactor, Output et MediaPlayer partagent UN contexte : soit les DEUX
// obtiennent un backend réel, soit les DEUX dégradent — jamais l'un sans
// l'autre.
func TestOutputAndMediaPlayer_ShareSingletonContext_AgreeOnDegradation(t *testing.T) {
	out := NewOutput(OutputConfig{})
	if out == nil {
		t.Fatal("setup invalide : NewOutput a renvoyé nil")
	}
	defer out.Close()

	mp := NewMediaPlayer(OutputConfig{}, nil)
	if mp == nil {
		t.Fatal("setup invalide : NewMediaPlayer a renvoyé nil")
	}
	defer mp.Close()

	outNeutral := IsNeutral(out)
	mpNeutral := IsNeutralMedia(mp)
	if outNeutral != mpNeutral {
		t.Fatalf("Output (IsNeutral=%v) et MediaPlayer (IsNeutralMedia=%v) divergent sur la disponibilité du contexte oto — ils doivent partager le MÊME contexte singleton (contract §10.3) et donc dégrader ENSEMBLE, jamais l'un sans l'autre", outNeutral, mpNeutral)
	}
	t.Logf("Output et MediaPlayer s'accordent : neutral=%v (les deux issues sont valides selon l'environnement)", outNeutral)
}
