/**
 * #144 (v12.0.0, LOT-1.2) — FortuneWheel : cas limites de spécification (wheel-component.md §8, roue.md §6).
 * Complète FortuneWheel.144.test.jsx (développeur).
 */
import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, cleanup, act } from '@testing-library/react'
import FortuneWheel, { buildFortuneSegments, fortuneMotion } from './FortuneWheel'

afterEach(() => cleanup())

const wheel = (over = {}) => ({
  LAYOUT: 'strip', SPIN_PERIOD_MS: 2500, SPIN_MAX_MS: 10000, SPIN_UP_MS: 10000, FULL_SPEED: false, FLIP_INTERVAL_MS: 600, CURRENT: -1, STOP: null,
  SEGMENTS: [
    { ID: 's0', KIND: 'value', VALUE: 100 }, { ID: 's1', KIND: 'bankrupt', VALUE: 0 },
    { ID: 's2', KIND: 'value', VALUE: 90 }, { ID: 's3', KIND: 'lose_turn', VALUE: 0 },
  ], ...over,
})

describe('FortuneWheel — spécification', () => {
  it('conserve l\'ordre et les identifiants des cases du serveur', () => {
    const segs = buildFortuneSegments(wheel().SEGMENTS)
    expect(segs.map(s => s.id)).toEqual(['s0', 's1', 's2', 's3'])
    expect(segs[0].label).toBe('100')
    expect(segs[1].kind).toBe('special')
    expect(segs[3].kind).toBe('special')
  })

  it('une valeur n\'est jamais confondue avec une case spéciale', () => {
    const segs = buildFortuneSegments([{ ID: 'a', KIND: 'value', VALUE: 0 }])
    expect(segs[0].kind).toBe('value')
  })

  it('motion : TURN / LETTER / ANSWER… = null (roue arrêtée, case courante)', () => {
    for (const sp of ['', 'TURN', 'LETTER', 'LIT', 'REVEALING', 'ANSWER', 'TURN_END', 'SOLVED']) {
      expect(fortuneMotion(wheel({ CURRENT: 2 }), sp)).toBeNull()
    }
  })

  it('STOPPING sans STOP (état incohérent) : pas de mouvement, pas d\'exception', () => {
    expect(fortuneMotion(wheel({ STOP: null }), 'STOPPING')).toBeNull()
  })

  it('STOPPING : freinage vers STOP.TO avec au moins un tour', () => {
    const m = fortuneMotion(wheel({ STOP: { TO: 2, DURATION_MS: 4000 } }), 'STOPPING')
    expect(m).toEqual({ mode: 'land', to: 2, durationMs: 4000, minTurns: 1, periodMs: 2500 }) // freinage toujours depuis la croisière
  })

  it('SPINNING : défilement à la période du serveur', () => {
    expect(fortuneMotion(wheel({ SPIN_PERIOD_MS: 3000, SPIN_UP_MS: 12000 }), 'SPINNING')).toMatchObject({ mode: 'spin', periodMs: 3000, accelTurns: 2 })
  })

  it('SPINNING : accelTurns = SPIN_UP_MS / (2 × SPIN_PERIOD_MS) (le serveur fait foi), plus aucune durée fixe', () => {
    const m = fortuneMotion(wheel({ SPIN_PERIOD_MS: 1000, SPIN_UP_MS: 4000 }), 'SPINNING')
    expect(m.accelTurns).toBeCloseTo(2, 9)
    expect(fortuneMotion(wheel({ SPIN_PERIOD_MS: 6000, SPIN_UP_MS: 24000 }), 'SPINNING').accelTurns).toBeCloseTo(2, 9)
  })

  it('SPINNING : cruise = FULL_SPEED (signal du serveur, pas la fin de montée calculée localement)', () => {
    expect(!!fortuneMotion(wheel({ FULL_SPEED: false }), 'SPINNING').cruise).toBe(false)
    expect(fortuneMotion(wheel({ FULL_SPEED: true }), 'SPINNING').cruise).toBe(true)
  })

  it('CURRENT = -1 : aucune case en évidence ; CURRENT = 2 : la case 2', () => {
    const { container, rerender } = render(<FortuneWheel wheel={wheel()} subphase="TURN" />)
    expect(container.querySelector('.is-current, [data-current="true"]')).toBeNull()
    rerender(<FortuneWheel wheel={wheel({ CURRENT: 2 })} subphase="LETTER" />)
    expect(container.querySelector('.is-current, [data-current="true"]')).not.toBeNull()
  })

  it('la barre de durée maximale n\'existe que pendant SPINNING, une fois la pleine vitesse atteinte', () => {
    const { container, rerender } = render(<FortuneWheel wheel={wheel({ FULL_SPEED: true })} subphase="SPINNING" />)
    expect(container.querySelector('[role="progressbar"]')).not.toBeNull()
    rerender(<FortuneWheel wheel={wheel({ STOP: { TO: 1, DURATION_MS: 4000 } })} subphase="STOPPING" />)
    expect(container.querySelector('[role="progressbar"]')).toBeNull()
  })

  it('la forme vient de la question : arc / strip', () => {
    const a = render(<FortuneWheel wheel={wheel({ LAYOUT: 'arc' })} subphase="TURN" />)
    expect(a.container.querySelector('.wheel-arc')).not.toBeNull()
    cleanup()
    const s = render(<FortuneWheel wheel={wheel({ LAYOUT: 'strip' })} subphase="TURN" />)
    expect(s.container.querySelector('.wheel-strip')).not.toBeNull()
  })

  it('layout inconnu : repli sur le bandeau', () => {
    const { container } = render(<FortuneWheel wheel={wheel({ LAYOUT: 'carre' })} subphase="TURN" />)
    expect(container.querySelector('.wheel-strip')).not.toBeNull()
  })
})

describe('FortuneWheel — barre de durée maximale : comptée depuis FULL_SPEED (roue.md §6)', () => {
  it('pas de barre pendant la montée', () => {
    const { container } = render(<FortuneWheel wheel={wheel({ FULL_SPEED: false })} subphase="SPINNING" />)
    expect(container.querySelector('[role="progressbar"]')).toBeNull()
  })

  it('la barre apparaît pleine à FULL_SPEED=true, même après une longue montée, puis se décompte', () => {
    vi.useFakeTimers()
    try {
      const { container, rerender } = render(<FortuneWheel wheel={wheel({ FULL_SPEED: false })} subphase="SPINNING" />)
      act(() => { vi.advanceTimersByTime(10000) }) // toute la montée
      rerender(<FortuneWheel wheel={wheel({ FULL_SPEED: true })} subphase="SPINNING" />)
      const bar = () => container.querySelector('[role="progressbar"]')
      expect(bar()).not.toBeNull()
      expect(Number(bar().getAttribute('aria-valuenow'))).toBeGreaterThan(9000) // le décompte démarre à la réception
      act(() => { vi.advanceTimersByTime(5000) })
      const v = Number(bar().getAttribute('aria-valuenow'))
      expect(v).toBeGreaterThan(3500); expect(v).toBeLessThan(6500)
    } finally { vi.useRealTimers() }
  })

  it('un UPDATE intermédiaire en SPINNING / FULL_SPEED ne relance pas le décompte', () => {
    vi.useFakeTimers()
    try {
      const w = wheel({ FULL_SPEED: true })
      const { container, rerender } = render(<FortuneWheel wheel={w} subphase="SPINNING" />)
      act(() => { vi.advanceTimersByTime(4000) })
      rerender(<FortuneWheel wheel={{ ...w }} subphase="SPINNING" />)
      expect(Number(container.querySelector('[role="progressbar"]').getAttribute('aria-valuenow'))).toBeLessThan(6500)
    } finally { vi.useRealTimers() }
  })
})
