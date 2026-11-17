import { describe, it, expect, afterEach } from 'vitest'
import { render, cleanup } from '@testing-library/react'
import FortuneWheel, { buildFortuneSegments, fortuneMotion } from './FortuneWheel'

afterEach(() => cleanup())

const wheel = (over = {}) => ({
  LAYOUT: 'strip', SPIN_PERIOD_MS: 2500, SPIN_MAX_MS: 10000, SPIN_UP_MS: 5000, FULL_SPEED: false, CURRENT: -1, STOP: null,
  SEGMENTS: [{ ID: 's0', KIND: 'value', VALUE: 100 }, { ID: 's1', KIND: 'bankrupt', VALUE: 0 }, { ID: 's2', KIND: 'value', VALUE: 90 }, { ID: 's3', KIND: 'lose_turn', VALUE: 0 }],
  ...over,
})

describe('FortuneWheel — adaptateur (#144, wheel-component.md §8)', () => {
  it('traduit KIND en segments value / special', () => {
    const s = buildFortuneSegments(wheel().SEGMENTS)
    expect(s.map(x => x.kind)).toEqual(['value', 'special', 'value', 'special'])
    expect(s[0].label).toBe('100')
    expect(s[1]).toMatchObject({ label: 'BANQUE\nROUTE', icon: '💸' })
    expect(s[3]).toMatchObject({ label: 'TOUR\nPERDU', icon: '⏭' })
  })
  it('motion selon la sous-phase', () => {
    expect(fortuneMotion(wheel(), 'SPINNING')).toMatchObject({ mode: 'spin', periodMs: 2500, accelTurns: 1 })
    expect(fortuneMotion(wheel({ STOP: { TO: 2, DURATION_MS: 4000 } }), 'STOPPING')).toEqual({ mode: 'land', to: 2, durationMs: 4000, minTurns: 1, periodMs: 2500 }) // freinage toujours depuis la croisière
    expect(fortuneMotion(wheel(), 'TURN')).toBeNull()
  })
  it('layout arc / strip et aria-label du segment courant', () => {
    const { container, rerender } = render(<FortuneWheel wheel={wheel({ CURRENT: 2 })} subphase="TURN" />)
    expect(container.querySelector('.wheel-strip')).not.toBeNull()
    expect(container.querySelector('[role="img"]').getAttribute('aria-label')).toBe('Segment : 90')
    rerender(<FortuneWheel wheel={wheel({ LAYOUT: 'arc', CURRENT: 1 })} subphase="TURN" />)
    expect(container.querySelector('.wheel-arc')).not.toBeNull()
    expect(container.querySelector('[role="img"]').getAttribute('aria-label')).toBe('Segment : BANQUE ROUTE')
  })
  it('SPINNING à pleine vitesse : barre de durée maximale, aucune case en évidence', () => {
    const { container, getByRole } = render(<FortuneWheel wheel={wheel({ CURRENT: 2, FULL_SPEED: true })} subphase="SPINNING" />)
    expect(getByRole('progressbar')).not.toBeNull()
    expect(container.querySelector('[data-current="true"]')).toBeNull()
  })
  it('couleur d\'équipe [r,g,b] → accent', () => {
    const { container } = render(<FortuneWheel wheel={wheel()} subphase="TURN" teamColor={[255, 0, 0]} />)
    expect(container.querySelector('.wheel').style.getPropertyValue('--wheel-accent')).toBe('rgb(255,0,0)')
  })
  it('sans roue : rien', () => {
    const { container } = render(<FortuneWheel wheel={null} />)
    expect(container.firstChild).toBeNull()
  })
})
