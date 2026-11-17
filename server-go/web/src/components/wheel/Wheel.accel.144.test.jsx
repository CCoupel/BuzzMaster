import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import { render, cleanup, act } from '@testing-library/react'
import Wheel from './Wheel'

// #144 (12.0.0.4) — la roue ROUE accélère au lancement (accelMs) ; sans accelMs (RAFALE, anciens usages) rien ne change.
afterEach(() => { cleanup(); vi.useRealTimers() })
beforeEach(() => { vi.useFakeTimers() })

const segs = (n = 24) => Array.from({ length: n }, (_, i) => ({ id: `s${i}`, label: String(100 * (i + 1)), kind: 'value' }))
const SPIN = { mode: 'spin', periodMs: 2500, accelMs: 1500 }
// La montée ne joue que sur le passage null → spin (roue.md §6) : on monte d'abord à l'arrêt.
const start = (motion) => { const r = render(<Wheel segments={segs()} current={0} motion={null} />); r.rerender(<Wheel segments={segs()} current={0} motion={motion} />); return r }
const run = async (ms) => { await act(async () => { vi.advanceTimersByTime(ms) }) }
const pos = (c) => Number(c.querySelector('[role="img"]').getAttribute('data-pos'))
const wrap = (a, b, n = 24) => ((b - a + n / 2) % n + n) % n - n / 2

describe('Wheel — montée en vitesse au lancement', () => {
  it('montage direct en spin (client connecté en cours de rotation) : pas de montée, croisière immédiate', async () => {
    const { container } = render(<Wheel segments={segs()} current={0} motion={SPIN} />)
    await run(400)
    expect(wrap(0, pos(container))).toBeCloseTo(24 * 400 / 2500, 0)
  })

  it('avec accelMs : démarre doucement (moins de distance que le défilement uniforme sur les 400 premières ms)', async () => {
    const a = start(SPIN)
    await run(400)
    const ramp = wrap(0, pos(a.container))
    a.unmount()
    const b = start({ mode: 'spin', periodMs: 2500 })
    await run(400)
    const uniform = wrap(0, pos(b.container))
    expect(ramp).toBeGreaterThan(0)
    expect(ramp).toBeLessThan(uniform * 0.6) // montée : ¼ de la distance à t = accelMs/2 vs ½ ; à 400 ms ≈ 0,5 seg contre 3,8
    expect(uniform).toBeCloseTo(24 * 400 / 2500, 0)
  })

  it('re-rendu / UPDATE intermédiaire (même motion) : pas de nouvelle montée, la roue ne repart pas de l\'arrêt', async () => {
    const motion = { mode: 'spin', periodMs: 2500, accelMs: 1500 }
    const { container, rerender } = start(motion)
    await run(1500)
    const before = pos(container)
    rerender(<Wheel segments={segs()} current={0} motion={{ ...motion }} />) // objet neuf, mêmes valeurs
    await run(100)
    const moved = wrap(before, pos(container))
    expect(moved).toBeCloseTo(24 * 100 / 2500, 0) // vitesse de croisière (≈ 0,96 seg), pas une montée (≈ 0)
    expect(moved).toBeGreaterThan(0.7)
  })

  it('arrêt PENDANT la montée : pas de saut d\'angle, atterrit exactement sur `to`, jamais après durationMs', async () => {
    const { container, rerender } = start(SPIN)
    await run(300)
    const atStop = pos(container)
    rerender(<Wheel segments={segs()} current={0} motion={{ mode: 'land', to: 9, durationMs: 4000, minTurns: 1, periodMs: 2500, accelMs: 1500 }} />)
    await run(20)
    expect(Math.abs(wrap(atStop, pos(container)))).toBeLessThan(0.3) // continuité d'angle (≈ vitesse × 20 ms)
    await run(4030) // une image après durationMs (finish exact à la première image ≥ durationMs)
    expect(pos(container)).toBe(9)
  })

  it('arrêt en croisière : atterrit exactement sur `to` à durationMs', async () => {
    const { container, rerender } = start(SPIN)
    await run(2000)
    rerender(<Wheel segments={segs()} current={0} motion={{ mode: 'land', to: 17, durationMs: 4000, minTurns: 1, periodMs: 2500, accelMs: 1500 }} />)
    await run(4050)
    expect(pos(container)).toBe(17)
  })

  it('sans accelMs (RAFALE / usages existants) : défilement uniforme dès la première image, land cubique inchangé', async () => {
    const { container, rerender } = start({ mode: 'spin', periodMs: 2500 })
    await run(100)
    expect(pos(container)).toBeCloseTo(24 * 100 / 2500, 0)
    rerender(<Wheel segments={segs()} current={0} motion={{ mode: 'land', to: 5, durationMs: 1500, minTurns: 1 }} />)
    await run(1600)
    expect(pos(container)).toBe(5)
  })
})
