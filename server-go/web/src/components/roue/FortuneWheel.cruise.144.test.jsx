import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import { render, cleanup, act } from '@testing-library/react'
import FortuneWheel from './FortuneWheel'

// #144 (v12.0.0) — FULL_SPEED reçu du serveur EN COURS de rampe (écran en avance ou en retard sur la montée locale) :
// la roue passe aussitôt à la vitesse de croisière, sans saut de position, et sans relancer de montée (wheel-component.md §5, critère 4).
afterEach(() => { cleanup(); vi.useRealTimers() })
beforeEach(() => { vi.useFakeTimers() })

const N = 24
const wheel = (over = {}) => ({
  LAYOUT: 'strip', SPIN_PERIOD_MS: 2500, SPIN_MAX_MS: 10000, SPIN_UP_MS: 10000, FULL_SPEED: false, CURRENT: 0, STOP: null,
  SEGMENTS: Array.from({ length: N }, (_, i) => ({ ID: `s${i}`, KIND: 'value', VALUE: 100 * (i + 1) })), ...over,
})
const run = async (ms) => { await act(async () => { vi.advanceTimersByTime(ms) }) }
const pos = (c) => Number(c.querySelector('[role="img"]').getAttribute('data-pos'))
const wrap = (a, b) => ((b - a + N / 2) % N + N) % N - N / 2
const VC_100MS = N * 100 / 2500 // 0,96 segment

describe('FortuneWheel — FULL_SPEED reçu en cours de rampe', () => {
  it('pas de saut de position, vitesse de croisière dès l\'image suivante', async () => {
    const { container, rerender } = render(<FortuneWheel wheel={wheel({ CURRENT: -1 })} subphase="TURN" />)
    rerender(<FortuneWheel wheel={wheel({ CURRENT: -1 })} subphase="SPINNING" />)
    await run(3000) // rampe : vitesse ≈ 0,3 vc
    const before = pos(container)
    rerender(<FortuneWheel wheel={wheel({ CURRENT: -1, FULL_SPEED: true })} subphase="SPINNING" />)
    await run(20)
    expect(Math.abs(wrap(before, pos(container)))).toBeLessThan(VC_100MS * 0.2 + 0.05) // pas de saut de position
    const mid = pos(container)
    await run(100)
    expect(wrap(mid, pos(container))).toBeGreaterThan(VC_100MS * 0.8) // vc, pas la vitesse de rampe (≈ 0,3 vc)
    expect(wrap(mid, pos(container))).toBeLessThan(VC_100MS * 1.2)
  })

  it('FULL_SPEED reçu juste après le début : jamais de marche arrière, pas de nouvelle montée ensuite', async () => {
    const { container, rerender } = render(<FortuneWheel wheel={wheel({ CURRENT: -1 })} subphase="TURN" />)
    rerender(<FortuneWheel wheel={wheel({ CURRENT: -1 })} subphase="SPINNING" />)
    await run(100)
    rerender(<FortuneWheel wheel={wheel({ CURRENT: -1, FULL_SPEED: true })} subphase="SPINNING" />)
    let prev = pos(container)
    for (let i = 0; i < 10; i++) {
      await run(100)
      const p = pos(container)
      expect(wrap(prev, p)).toBeGreaterThanOrEqual(0)
      prev = p
    }
    // UPDATE intermédiaire (objet neuf, mêmes valeurs) : toujours à vc
    const before = pos(container)
    rerender(<FortuneWheel wheel={{ ...wheel({ CURRENT: -1, FULL_SPEED: true }) }} subphase="SPINNING" />)
    await run(100)
    expect(wrap(before, pos(container))).toBeGreaterThan(VC_100MS * 0.8)
  })
})
