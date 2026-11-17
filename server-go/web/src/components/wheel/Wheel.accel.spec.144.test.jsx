import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import { render, cleanup, act } from '@testing-library/react'
import Wheel from './Wheel'

// #144 (12.0.0) — « Forcer l'arrêt » de l'admin PENDANT la montée (wheel-component.md §5, cas d'exception, critère 3) :
// le freinage part de la vitesse atteinte sur la rampe — jamais d'un saut à la vitesse de croisière — et atterrit
// exactement sur `to`, en durationMs au plus. Complète Wheel.accel.144.test.jsx (développeur).
afterEach(() => { cleanup(); vi.useRealTimers() })
beforeEach(() => { vi.useFakeTimers() })

const segs = (n = 24) => Array.from({ length: n }, (_, i) => ({ id: `s${i}`, label: String(100 * (i + 1)), kind: 'value' }))
const SPIN = { mode: 'spin', periodMs: 2500, accelTurns: 2 } // montée : 10 s
const start = (motion) => { const r = render(<Wheel segments={segs()} current={0} motion={null} />); r.rerender(<Wheel segments={segs()} current={0} motion={motion} />); return r }
const run = async (ms) => { await act(async () => { vi.advanceTimersByTime(ms) }) }
const pos = (c) => Number(c.querySelector('[role="img"]').getAttribute('data-pos'))
const wrap = (a, b, n = 24) => ((b - a + n / 2) % n + n) % n - n / 2
const VC_PER_100MS = 24 * 100 / 2500 // 0,96 segment

describe('Wheel — arrêt forcé pendant la montée', () => {
  for (const [tStop, frac] of [[5000, 0.5], [1000, 0.1]]) {
    it(`arrêt à ${tStop} ms (vitesse ${frac} vc) : pas de saut de position, vitesse continue (pas de saut à vc), atterrissage exact`, async () => {
      const r = start(SPIN)
      await run(tStop)
      const atStop = pos(r.container)
      r.rerender(<Wheel segments={segs()} current={0} motion={{ mode: 'land', to: 11, durationMs: 4000, minTurns: 1, periodMs: 2500, accelTurns: 2 }} />)
      await run(20)
      expect(Math.abs(wrap(atStop, pos(r.container)))).toBeLessThan(VC_PER_100MS * 0.2 * Math.max(frac, 0.15) + 0.1)
      const mid = pos(r.container)
      await run(100)
      const moved = wrap(mid, pos(r.container))
      expect(moved).toBeGreaterThanOrEqual(0)
      expect(moved).toBeLessThan(VC_PER_100MS * (frac + 0.25)) // vitesse ≈ frac × vc, jamais vc
      await run(4050)
      expect(pos(r.container)).toBe(11)
    })
  }

  it('arrêt dès les premières images (vitesse ≈ 0) : jamais de marche arrière, atterrit sur `to`', async () => {
    const r = start(SPIN)
    await run(50)
    let prev = pos(r.container)
    r.rerender(<Wheel segments={segs()} current={0} motion={{ mode: 'land', to: 20, durationMs: 4000, minTurns: 1, periodMs: 2500, accelTurns: 2 }} />)
    for (let i = 0; i < 42; i++) {
      await run(100)
      const p = pos(r.container)
      expect(wrap(prev, p)).toBeGreaterThanOrEqual(-1e-6)
      prev = p
    }
    expect(pos(r.container)).toBe(20)
  })

  it('100 enchaînements spin (montée) → land : position normalisée, `to` atteint à chaque fois', async () => {
    const r = start(SPIN)
    for (let i = 0; i < 100; i++) {
      await run(400 + (i % 7) * 300) // arrêt à des vitesses variées, pendant la montée
      const to = (i * 5) % 24
      r.rerender(<Wheel segments={segs()} current={0} motion={{ mode: 'land', to, durationMs: 1500, minTurns: 1, periodMs: 2500, accelTurns: 2 }} />)
      await run(1600)
      const p = pos(r.container)
      expect(p).toBeGreaterThanOrEqual(0); expect(p).toBeLessThan(24)
      expect(p).toBe(to)
      r.rerender(<Wheel segments={segs()} current={to} motion={null} />)
      r.rerender(<Wheel segments={segs()} current={to} motion={SPIN} />)
    }
  })
})
