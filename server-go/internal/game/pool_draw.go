package game

import (
	"log"
	"math/rand"
	"strings"
)

// Stratified draw over an EntryPool — contracts/rafale.md §7.2 to §7.4 (#216),
// generalised in v13.1.0 (contracts/picto-pool.md §1, §3): the state is a
// plain value and the functions take (pool, state), so the `rafale` and
// `picto` instances share one implementation. The RAFALE round's own state
// (Engine.rafaleDraw), its pre-draw (#202) and the alphabetical mode (#282)
// are unchanged and live in engine.go / rafale_alpha.go.

// poolCouple is one (category, difficulty) pair from a round's configured
// cartesian product — contracts/rafale.md §7.2 (#216). A small comparable
// value type, freely copied into permutation orderings.
type poolCouple struct {
	category   string
	difficulty int
}

// poolDrawState (#216, contract §7.2) sequences a round's stratified draw: a
// random permutation of the still-viable couples, replayed (reshuffled) once
// fully cycled. `active` only ever shrinks — a couple found empty, at
// construction or mid-round, is removed permanently (never re-added,
// contract §7.3's "au lancement OU en cours de manche, unifié"). See the
// Engine.rafaleDraw field's own doc comment for its lifecycle.
type poolDrawState struct {
	active []poolCouple // couples not yet known-exhausted this round
	order  []poolCouple // current permutation of `active`, drained by `pos`
	pos    int          // index into `order` for the NEXT draw attempt
}

// newPoolDrawState builds the initial poolDrawState for a fresh round: the
// cartesian product of the given categories × difficulties, keeping only
// couples with at least one available entry in pool RIGHT NOW — a couple
// with none is excluded upfront (contract §7.3, unified with the mid-round
// exhaustion path in next, which removes a couple the exact same way the
// moment it's discovered empty). Caller must hold the pool's mutex.
//
// (#217, contracts/rafale.md §14.7) categories/difficulties are resolved by
// the caller: a classic round from its round-config Question
// (EffectiveRafaleCategories/…Difficulties), a RAFALE-in-card round from its
// MotionCard.
func newPoolDrawState(pool *EntryPool, categories []string, difficulties []int) poolDrawState {
	var active []poolCouple
	for _, cat := range categories {
		for _, diff := range difficulties {
			if len(pool.availableUnsafe([]string{cat}, []int{diff})) > 0 {
				active = append(active, poolCouple{category: cat, difficulty: diff})
			} else {
				log.Printf("[Engine] %s: couple category=%s difficulty=%d has no available question at round start, excluded from the active set (contract §7.3)", pool.logPrefix(), cat, diff)
			}
		}
	}
	return poolDrawState{active: active}
}

// reshuffle draws a fresh random permutation of s.active into its order,
// resetting pos to 0 — contract §7.2 "nouvelle permutation une fois
// épuisée".
func (s *poolDrawState) reshuffle() {
	order := make([]poolCouple, len(s.active))
	copy(order, s.active)
	rand.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	s.order = order
	s.pos = 0
}

// removeCouple permanently drops couple from s.active (contract §7.3 — a
// couple found empty, whether at round start or mid-round, never re-enters a
// future permutation this round) and invalidates the current in-flight
// permutation: order/pos are reset so the NEXT draw attempt reshuffles from
// the now-shrunk active set, rather than keep draining a permutation built
// from a larger one (a slightly shorter/uneven current cycle is an
// acceptable one-time rebalancing blip, not a recurring bias).
func (s *poolDrawState) removeCouple(couple poolCouple) {
	for i, c := range s.active {
		if c == couple {
			s.active = append(s.active[:i], s.active[i+1:]...)
			break
		}
	}
	s.order = nil
	s.pos = 0
}

// next draws the round's next entry from pool following the stratified
// permutation sequence in s (contract §7.2, #216). Returns ErrRafalePoolEmpty
// (= ErrPoolEmpty) only when s.active is entirely empty — a couple found
// individually exhausted along the way is removed (contract §7.3) and the
// loop keeps trying the remaining couples, rebalancing; it NEVER surfaces
// that couple's own emptiness as an error to the caller. Caller must hold the
// pool's mutex (write lock) and persist the used flags afterwards.
func (s *poolDrawState) next(pool *EntryPool) (*PoolEntry, error) {
	for {
		if len(s.active) == 0 {
			return nil, ErrRafalePoolEmpty
		}
		if s.pos >= len(s.order) {
			s.reshuffle()
		}

		couple := s.order[s.pos]
		s.pos++

		drawn, err := pool.drawUnsafe([]string{couple.category}, []int{couple.difficulty})
		if err == nil {
			return drawn, nil
		}

		// Couple genuinely exhausted (found empty just now, whether it was
		// its first attempt or a later cycle) — remove permanently and keep
		// trying the remaining couples (contract §7.3: unified with the
		// pre-round exclusion in newPoolDrawState, never blocks).
		log.Printf("[Engine] %s: couple category=%s difficulty=%d exhausted mid-round, removed from the active set (contract §7.3)", pool.logPrefix(), couple.category, couple.difficulty)
		s.removeCouple(couple)
	}
}

// logPrefix is the log tag of the pool instance: "RAFALE" for the RAFALE
// reservoir (existing log texts unchanged), "PICTO" for the Pool Picto.
func (p *EntryPool) logPrefix() string { return strings.ToUpper(p.label) }
