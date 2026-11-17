/**
 * #144 (v12.0.0, LOT-1.2) — géométrie du mouvement long, tests de SPÉCIFICATION (contracts/wheel-component.md §4/§5) :
 * position toujours normalisée, amplitude ≤ 4n, arrivée exacte sur `to`, aucune dérive. Complète les tests
 * du développeur (Wheel.motion.144.test.jsx) par des propriétés sur de nombreux cas.
 */
import { describe, it, expect } from 'vitest'
import { spinPosition, landTotal, landPosition, MAX_LAND_TURNS } from './geometry'

const SIZES = [2, 10, 22, 26, 64]

describe('spinPosition — défilement continu', () => {
  it('un tour exact en periodMs revient à la position de départ', () => {
    for (const n of SIZES) expect(spinPosition(3 % n, 2500, 2500, n)).toBeCloseTo(3 % n, 6)
  })

  it('demi-tour = n/2 segments', () => {
    expect(spinPosition(0, 1250, 2500, 22)).toBeCloseTo(11, 6)
  })

  it('reste dans [0, n) pour des durées arbitrairement longues (aucune dérive)', () => {
    for (const n of SIZES) {
      for (const t of [0, 1, 999, 2500, 10_000, 123_456_789, 9e12]) {
        const p = spinPosition(n - 0.5, t, 2500, n)
        expect(p).toBeGreaterThanOrEqual(0)
        expect(p).toBeLessThan(n)
      }
    }
  })

  it('avance toujours vers l\'avant (monotone modulo n sur un tour)', () => {
    const n = 22
    let prev = spinPosition(0, 0, 2500, n)
    for (let t = 100; t < 2500; t += 100) {
      const p = spinPosition(0, t, 2500, n)
      expect(p).toBeGreaterThan(prev)
      prev = p
    }
  })

  it('durée négative ou nulle : pas de défilement négatif', () => {
    expect(spinPosition(5, -1000, 2500, 22)).toBeCloseTo(5, 6)
  })

  it('n invalide : renvoie 0 sans lever', () => {
    expect(spinPosition(3, 1000, 2500, 0)).toBe(0)
  })
})

describe('landTotal — amplitude d\'un freinage', () => {
  it('toujours dans [0, 4n] pour toute paire from / to / minTurns', () => {
    for (const n of SIZES) {
      for (let from = 0; from < n; from += Math.max(1, Math.floor(n / 7))) {
        for (let to = 0; to < n; to++) {
          for (const mt of [0, 1, 2, 3, 10, 100]) {
            const total = landTotal(from, to, n, mt)
            expect(total).toBeGreaterThanOrEqual(0)
            expect(total).toBeLessThanOrEqual(4 * n)
          }
        }
      }
    }
  })

  it('au moins un tour complet quand minTurns = 1, même si from = to', () => {
    for (const n of SIZES) expect(landTotal(2 % n, 2 % n, n, 1)).toBeGreaterThanOrEqual(n)
  })

  it('l\'amplitude arrive exactement sur `to` modulo n', () => {
    for (const n of SIZES) {
      for (let to = 0; to < n; to++) {
        const from = (to * 7 + 3) % n
        const total = landTotal(from, to, n, 1)
        expect((((from + total) % n) + n) % n).toBeCloseTo(to, 6)
      }
    }
  })

  it('plafonne minTurns : MAX_LAND_TURNS tours au plus', () => {
    expect(MAX_LAND_TURNS).toBeLessThanOrEqual(3)
    expect(landTotal(0, 0, 22, 1000)).toBeLessThanOrEqual(4 * 22)
  })

  it('n invalide : 0', () => {
    expect(landTotal(0, 1, 0, 1)).toBe(0)
  })
})

describe('landPosition — avancement', () => {
  const n = 22, from = 5, to = 17
  const total = landTotal(from, to, n, 1)

  it('p = 0 : position de départ', () => {
    expect(landPosition(from, to, total, 0, n)).toBeCloseTo(from, 6)
  })

  it('p ≥ 1 : exactement `to` (aucun arrondi flottant résiduel)', () => {
    expect(landPosition(from, to, total, 1, n)).toBe(to)
    expect(landPosition(from, to, total, 1.5, n)).toBe(to)
  })

  it('reste dans [0, n) sur tout l\'avancement', () => {
    for (let p = 0; p <= 1; p += 0.01) {
      const pos = landPosition(from, to, total, p, n)
      expect(pos).toBeGreaterThanOrEqual(0)
      expect(pos).toBeLessThan(n)
    }
  })

  it('100 freinages enchaînés depuis la position d\'arrivée précédente : aucune dérive', () => {
    let pos = 0
    for (let i = 0; i < 100; i++) {
      const target = (i * 13 + 5) % n
      const tot = landTotal(pos, target, n, 1)
      pos = landPosition(pos, target, tot, 1, n)
      expect(pos).toBe(target)
    }
  })
})
