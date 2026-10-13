import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render } from '@testing-library/react'
import TvPointsAward from './TvPointsAward'

// #265 — majeur de revue n°1 : les animations Web Animations (fusée, étincelles) ne doivent PAS être
// relancées (annulées puis recréées avec un nouvel aléa) à chaque re-rendu du composant : tick du
// chrono, UPDATE WebSocket, autre carte qui change d'étape, passage d'une carte en « out ».
// jsdom n'a pas Element.animate : on le double et on compte les lancements / annulations.
// Comportement observable (appels à animate/cancel, aléa consommé), pas le mécanisme du correctif.

vi.mock('./TvPointsAward.css', () => ({}))

let calls // { el, cancel }
beforeEach(() => {
  calls = []
  Object.defineProperty(Element.prototype, 'animate', {
    configurable: true,
    writable: true,
    value: function animate() {
      const rec = { el: this, cancel: vi.fn() }
      calls.push(rec)
      return { cancel: rec.cancel }
    },
  })
})
afterEach(() => {
  delete Element.prototype.animate
  vi.restoreAllMocks()
})

const card = (team, stage, x) => ({
  key: `1-${team}`, team, points: 3, color: 'rgb(30,120,250)', stage,
  end: { x, y: 40 }, start: { x: 10, y: 100 }, ctrl: { x: 15, y: 60 }, reduced: false,
})
const lotOf = (cards) => ({ id: 1, index: 0, count: 2, durationMs: 3200, own: false, reduced: false, cards })
// copie profonde : mêmes valeurs, objets neufs (ce que produit un nouveau rendu du parent)
const clone = (v) => JSON.parse(JSON.stringify(v))
const SPARKS = 18

const sparksOfCard = (x) => calls.filter((c) => c.el.parentElement?.style.left === `${x}%`)

describe('TvPointsAward — pas de relance des animations au re-rendu (#265)', () => {
  it('Critical — fusée en vol : un re-rendu identique ne la relance pas et ne l\'annule pas', () => {
    const lots = [lotOf([card('A', 'flying', 20)])]
    const { rerender } = render(<TvPointsAward lots={lots} />)
    expect(calls).toHaveLength(1)
    rerender(<TvPointsAward lots={lots} />)
    rerender(<TvPointsAward lots={clone(lots)} />)
    expect(calls).toHaveLength(1) // jamais relancée
    expect(calls[0].cancel).not.toHaveBeenCalled()
  })

  it('Critical — étincelles d\'une carte affichée : re-rendus identiques → aucune relance, aucune annulation, aucun nouvel aléa', () => {
    const lots = [lotOf([card('A', 'in', 20)])]
    const { rerender } = render(<TvPointsAward lots={lots} />)
    expect(calls).toHaveLength(SPARKS)
    const rnd = vi.spyOn(Math, 'random')
    rerender(<TvPointsAward lots={lots} />)
    rerender(<TvPointsAward lots={clone(lots)} />)
    expect(calls).toHaveLength(SPARKS)
    expect(calls.every((c) => c.cancel.mock.calls.length === 0)).toBe(true)
    expect(rnd).not.toHaveBeenCalled() // positions/distances aléatoires tirées une seule fois par carte
  })

  it('Critical — une autre carte change d\'étape : les étincelles déjà lancées de la 1re carte ne sont ni annulées ni relancées', () => {
    const base = (stageB) => [lotOf([card('A', 'in', 20), card('B', stageB, 60)])]
    const { rerender } = render(<TvPointsAward lots={base('wait')} />)
    expect(calls).toHaveLength(SPARKS) // étincelles de A seulement
    rerender(<TvPointsAward lots={base('flying')} />)
    expect(calls).toHaveLength(SPARKS + 1) // + la fusée de B
    rerender(<TvPointsAward lots={base('in')} />)
    expect(calls).toHaveLength(SPARKS + 1 + SPARKS) // + les étincelles de B (la fusée de B est démontée)
    const a = sparksOfCard(20)
    expect(a).toHaveLength(SPARKS) // A : lancées une seule fois
    expect(a.every((c) => c.cancel.mock.calls.length === 0)).toBe(true)
  })

  it('Critical — une carte passe en « out » : les autres cartes ne sont pas relancées', () => {
    const lots = (stageA) => [lotOf([card('A', stageA, 20), card('B', 'in', 60)])]
    const { rerender } = render(<TvPointsAward lots={lots('in')} />)
    expect(calls).toHaveLength(2 * SPARKS)
    rerender(<TvPointsAward lots={lots('out')} />)
    expect(calls).toHaveLength(2 * SPARKS) // aucun nouveau lancement
    const b = sparksOfCard(60)
    expect(b).toHaveLength(SPARKS)
    expect(b.every((c) => c.cancel.mock.calls.length === 0)).toBe(true)
  })

  it('un nouveau lot qui arrive ne relance pas les animations du lot en cours', () => {
    const first = lotOf([card('A', 'in', 20)])
    const { rerender } = render(<TvPointsAward lots={[first]} />)
    expect(calls).toHaveLength(SPARKS)
    const second = { ...lotOf([card('Z', 'wait', 70)]), id: 2, index: 1 }
    second.cards[0].key = '2-Z'
    rerender(<TvPointsAward lots={[first, second]} />)
    expect(calls).toHaveLength(SPARKS)
    expect(calls.every((c) => c.cancel.mock.calls.length === 0)).toBe(true)
  })
})
