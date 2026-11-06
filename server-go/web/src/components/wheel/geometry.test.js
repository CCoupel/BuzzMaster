import { describe, it, expect } from 'vitest'
import * as geometry from './geometry'

// #282 (v11.4.0, LOT-2) — contracts/wheel-component.md §3/§4 : fonctions pures, testées sans DOM.
// Écrit contre le contrat seul (le module n'existe pas encore : RED attendu).
const { indexToOffset, offsetToIndex, angleFor, positionAt } = geometry

describe('geometry — indexToOffset / offsetToIndex (normalisation modulo n, §4)', () => {
  it('offsetToIndex ramène toute position continue dans [0, n)', () => {
    for (const n of [1, 5, 24, 26, 64]) {
      for (const off of [0, 0.4, n - 0.0001, n, n + 3.7, 4 * n + 1, 100 * n + 0.5, -1, -n - 2.5, -100 * n - 0.25]) {
        const idx = offsetToIndex(off, n)
        expect(idx).toBeGreaterThanOrEqual(0)
        expect(idx).toBeLessThan(n)
        expect(Number.isFinite(idx)).toBe(true)
      }
    }
  })

  it('indexToOffset puis offsetToIndex est l\'identité pour un index entier valide', () => {
    for (let i = 0; i < 26; i++) {
      expect(offsetToIndex(indexToOffset(i, 26), 26)).toBe(i)
    }
  })

  it('1 000 mouvements aléatoires enchaînés : la position normalisée reste dans [0, n) (critère §4)', () => {
    const n = 26
    let pos = 0
    let seed = 12345
    const rnd = () => { seed = (seed * 1664525 + 1013904223) % 4294967296; return seed / 4294967296 }
    for (let k = 0; k < 1000; k++) {
      const amplitude = Math.floor(rnd() * 4 * n) // amplitude bornée ≤ 4n
      pos = offsetToIndex(pos + amplitude, n)
      expect(pos).toBeGreaterThanOrEqual(0)
      expect(pos).toBeLessThan(n)
    }
  })
})

describe('geometry — angleFor (arc : secteur i centré sur -90° + i × 360/n)', () => {
  it('index 0 est au sommet (-90°), pas de 360/n', () => {
    expect(angleFor(0, 26)).toBeCloseTo(-90, 5)
    expect(angleFor(1, 26) - angleFor(0, 26)).toBeCloseTo(360 / 26, 5)
    expect(angleFor(13, 26)).toBeCloseTo(-90 + 180, 5)
  })
  it('24 segments : pas de 15°', () => {
    expect(angleFor(1, 24) - angleFor(0, 24)).toBeCloseTo(15, 5)
  })
})

describe('geometry — positionAt (mouvement long, réservé v12.0)', () => {
  const motion = { from: 3, to: 10, turns: 0, durationMs: 1000, startedAt: 5000 }
  it('au départ : from ; à la fin : to', () => {
    expect(positionAt(5000, motion)).toBeCloseTo(3, 5)
    expect(positionAt(6000, motion)).toBeCloseTo(10, 5)
  })
  it('monotone et borné entre from et to (ease-out), au-delà de la fin : to', () => {
    let prev = positionAt(5000, motion)
    for (let t = 5100; t <= 6000; t += 100) {
      const p = positionAt(t, motion)
      expect(p).toBeGreaterThanOrEqual(prev - 1e-9)
      expect(p).toBeLessThanOrEqual(10 + 1e-9)
      prev = p
    }
    expect(positionAt(9000, motion)).toBeCloseTo(10, 5)
  })
})
