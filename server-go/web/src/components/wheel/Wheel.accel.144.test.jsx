import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import { render, cleanup, act } from '@testing-library/react'
import Wheel from './Wheel'

/**
 * #144 — montée par distance (1 tour pour la Roue de la Fortune ; composant générique), composant `Wheel` (contracts/wheel-component.md §5). Critères d'acceptation :
 *   4 `cruise: true` reçu avant la fin de la montée / après ;  5 montage direct en `spin`, `accelTurns` absent ;
 *   6 normalisation après 100 enchaînements ; + arrêt pendant la montée, pas de nouvelle montée au re-rendu, sans accelTurns inchangé.
 * (Critères 1 à 3 et 5 pur : geometry.accel.144.test.js.)
 */
afterEach(() => { cleanup(); vi.useRealTimers() })
beforeEach(() => { vi.useFakeTimers() })

const segs = (n = 24) => Array.from({ length: n }, (_, i) => ({ id: `s${i}`, label: String(100 * (i + 1)), kind: 'value' }))
const run = async (ms) => { await act(async () => { vi.advanceTimersByTime(ms) }) }
const pos = (c) => Number(c.querySelector('[role="img"]').getAttribute('data-pos'))
const wrap = (a, b, n = 24) => ((b - a + n / 2) % n + n) % n - n / 2
const SPIN = { mode: 'spin', periodMs: 2500, accelTurns: 2 }
// La montée ne joue que sur le passage null → spin (roue.md §6) : on monte d'abord à l'arrêt.
const start = (motion) => { const r = render(<Wheel segments={segs()} current={0} motion={null} />); r.rerender(<Wheel segments={segs()} current={0} motion={motion} />); return r }
const Rr = (r, motion) => r.rerender(<Wheel segments={segs()} current={0} motion={motion} />)

describe('critère 5 — montage direct / accelTurns absent : vitesse vc dès la première image', () => {
  it('montage direct en spin (client connecté en cours de rotation), cruise ou non', async () => {
    for (const cruise of [false, true]) {
      const { container, unmount } = render(<Wheel segments={segs()} current={0} motion={{ ...SPIN, cruise }} />)
      await run(400)
      expect(wrap(0, pos(container))).toBeCloseTo(24 * 400 / 2500, 0)
      unmount()
    }
  })
  it('accelTurns absent : défilement uniforme dès le départ, land cubique inchangé', async () => {
    const r = start({ mode: 'spin', periodMs: 2500 })
    await run(100)
    expect(pos(r.container)).toBeCloseTo(24 * 100 / 2500, 0)
    Rr(r, { mode: 'land', to: 5, durationMs: 1500, minTurns: 1 })
    await run(1600)
    expect(pos(r.container)).toBe(5)
  })
})

describe('montée', () => {
  it('démarre doucement : à 1 s, distance ≈ vc·t²/(4·2·période) = 0,048 seg contre 9,6 en uniforme', async () => {
    const r = start(SPIN)
    await run(1000)
    expect(wrap(0, pos(r.container))).toBeCloseTo(24 * 1000 * 1000 / (4 * 2 * 2500 * 2500), 1)
  })
  it('re-rendu / UPDATE intermédiaire (même motion, objet neuf) : pas de nouvelle montée', async () => {
    const r = start(SPIN)
    await run(6000)
    const before = pos(r.container)
    Rr(r, { ...SPIN })
    await run(100)
    // à 6 s la vitesse vaut 0,6·vc (≈ 0,0058 seg/ms) : ≈ 0,58 seg en 100 ms, pas ≈ 0 (nouvelle montée)
    expect(wrap(before, pos(r.container))).toBeGreaterThan(0.4)
  })
})

describe('critère 4 — cruise', () => {
  it('reçu avant la fin de la montée : vitesse vc dès l\'image suivante, aucun saut de position', async () => {
    const r = start(SPIN)
    await run(3000)
    const before = pos(r.container)
    Rr(r, { ...SPIN, cruise: true })
    await run(20)
    const jump = wrap(before, pos(r.container))
    expect(jump).toBeLessThan(0.35) // ≈ vitesse × 20 ms (0,0096 × 20 = 0,19), pas de saut de position
    const mid = pos(r.container)
    await run(100)
    expect(wrap(mid, pos(r.container))).toBeCloseTo(24 * 100 / 2500, 0) // vc dès maintenant
  })
  it('reçu après la fin de la montée : aucun effet (position et vitesse continues)', async () => {
    const r = start(SPIN)
    await run(10500)
    const before = pos(r.container)
    Rr(r, { ...SPIN, cruise: true })
    await run(100)
    expect(wrap(before, pos(r.container))).toBeCloseTo(24 * 100 / 2500, 0)
  })
})

describe('critère 3 — `land` reçu avant la fin de la montée locale (écran en retard)', () => {
  it('traité comme cruise : aucun saut de position, vitesse vc, arrêt exact sur `to`, jamais après durationMs', async () => {
    const r = start(SPIN)
    await run(1500)
    const atStop = pos(r.container)
    Rr(r, { mode: 'land', to: 3, durationMs: 4000, minTurns: 1, periodMs: 2500 })
    await run(20)
    expect(Math.abs(wrap(atStop, pos(r.container)))).toBeLessThan(0.35) // ≈ vc × 20 ms, pas de saut de position
    const mid = pos(r.container)
    await run(100)
    expect(wrap(mid, pos(r.container))).toBeGreaterThan(0.8) // vitesse vc dès l'image suivante (0,96 seg / 100 ms)
    await run(3900)
    expect(pos(r.container)).toBe(3)
  })
  it('arrêt depuis la croisière : atterrit exactement sur `to`', async () => {
    const r = start({ ...SPIN, cruise: true })
    await run(2000)
    Rr(r, { mode: 'land', to: 17, durationMs: 4000, minTurns: 1, periodMs: 2500 })
    await run(4050)
    expect(pos(r.container)).toBe(17)
  })
})

describe('critère 6 — normalisation après 100 enchaînements spin → land', () => {
  it('position normalisée, `to` sous le pointeur, cellules intactes', async () => {
    const s = segs()
    const { container, rerender } = render(<Wheel segments={s} current={0} motion={null} />)
    const cells = container.querySelectorAll('.wheel-cell').length
    for (let k = 1; k <= 100; k++) {
      rerender(<Wheel segments={s} motion={{ mode: 'spin', periodMs: 2000, accelTurns: 2, cruise: k % 2 === 0 }} />)
      await run(500 + (k % 7) * 400)
      rerender(<Wheel segments={s} motion={{ mode: 'land', to: (k * 5) % 24, durationMs: 1500, minTurns: 1, periodMs: 2000 }} />)
      await run(1600)
      const p = pos(container)
      expect(p).toBeGreaterThanOrEqual(0); expect(p).toBeLessThan(24)
      expect(p).toBe((k * 5) % 24)
    }
    expect(container.querySelectorAll('.wheel-cell').length).toBe(cells)
  })
})
