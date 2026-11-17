import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import { render, cleanup, act } from '@testing-library/react'
import RoueTv from './RoueTv'

/**
 * Bug QUALIF 12.0.0.8 : sur la tablette l'arrêt de la roue était immédiat (pas de freinage). Cause : la zone passait de
 * `<button>` (tapable) à `<div>` au passage SPINNING → STOPPING ; React démontait puis remontait `Wheel`, qui, monté
 * directement en `land`, se place aussitôt sur `to` (roue.md §6 / wheel-component §5). La roue doit rester MONTÉE.
 */
afterEach(() => { cleanup(); vi.useRealTimers() })
beforeEach(() => { vi.useFakeTimers() })

const segs = Array.from({ length: 24 }, (_, i) => ({ ID: `s${i}`, KIND: 'value', VALUE: 100 * (i + 1) }))
const gs = (over) => ({
  phase: 'STARTED', question: { TYPE: 'ROUE', ROUE_MODE: 'TANT_QUE_JE_GAGNE' },
  ROUE_SUBPHASE: 'SPINNING', ROUE_TEAMS: ['Rouges', 'Bleus'], ROUE_CURRENT_TEAM: 'Rouges', ROUE_CURRENT_TEAM_COLOR: [239, 68, 68],
  ROUE_TEAM_BANK: {}, ROUE_CONSONANTS_LEFT: true, ROUE_SHOW_LETTERS: true, ROUE_BOARD: { COLS: 14, ROWS: ['_____'] }, ROUE_LETTERS: [],
  ROUE_WHEEL: { LAYOUT: 'strip', SPIN_PERIOD_MS: 2500, SPIN_UP_MS: 10000, FULL_SPEED: true, SPIN_MAX_MS: 10000, FLIP_INTERVAL_MS: 600, CURRENT: 0, STOP: null, SEGMENTS: segs },
  ROUE_LAST_EVENT: { SEQ: 0, KIND: '', TEAM: '', LETTER: '', COUNT: 0, VALUE: 0, GAIN: 0 }, ROUE_WINNER: '', ROUE_AWARDS: {},
  ...over,
})
const teams = { Rouges: { COLOR: [239, 68, 68] }, Bleus: { COLOR: [59, 130, 246] } }
const run = async (ms) => { await act(async () => { vi.advanceTimersByTime(ms) }) }
const pos = (c) => Number(c.querySelector('[data-testid="fortune-wheel"] [role="img"]').getAttribute('data-pos'))

describe('tablette — SPINNING → STOPPING : la roue freine au lieu de se placer d\'un coup', () => {
  it('la roue n\'est pas démontée/remontée au passage tapable → non tapable', async () => {
    const { container, rerender } = render(<RoueTv gameState={gs({})} teams={teams} isVPlayer teamName="Rouges" onStop={() => {}} />)
    await run(500)
    const before = container.querySelector('[data-testid="fortune-wheel"]')
    rerender(<RoueTv gameState={gs({ ROUE_SUBPHASE: 'STOPPING', ROUE_WHEEL: { ...gs().ROUE_WHEEL, FULL_SPEED: false, STOP: { TO: 7, DURATION_MS: 4000 } } })} teams={teams} isVPlayer teamName="Rouges" onStop={() => {}} />)
    expect(container.querySelector('[data-testid="fortune-wheel"]')).toBe(before)
  })

  it('1 s après le début du freinage la roue roule encore ; elle n\'est sur la case tirée qu\'à la fin', async () => {
    const { container, rerender } = render(<RoueTv gameState={gs({})} teams={teams} isVPlayer teamName="Rouges" onStop={() => {}} />)
    await run(500)
    rerender(<RoueTv gameState={gs({ ROUE_SUBPHASE: 'STOPPING', ROUE_WHEEL: { ...gs().ROUE_WHEEL, FULL_SPEED: false, STOP: { TO: 7, DURATION_MS: 4000 } } })} teams={teams} isVPlayer teamName="Rouges" onStop={() => {}} />)
    await run(1000)
    expect(pos(container)).not.toBe(7)
    await run(3100)
    expect(pos(container)).toBe(7)
  })

  it('même chose pour l\'autre équipe (zone jamais tapable)', async () => {
    const { container, rerender } = render(<RoueTv gameState={gs({})} teams={teams} isVPlayer teamName="Bleus" onStop={() => {}} />)
    await run(500)
    const before = container.querySelector('[data-testid="fortune-wheel"]')
    rerender(<RoueTv gameState={gs({ ROUE_SUBPHASE: 'STOPPING', ROUE_WHEEL: { ...gs().ROUE_WHEEL, FULL_SPEED: false, STOP: { TO: 7, DURATION_MS: 4000 } } })} teams={teams} isVPlayer teamName="Bleus" onStop={() => {}} />)
    expect(container.querySelector('[data-testid="fortune-wheel"]')).toBe(before)
    await run(1000)
    expect(pos(container)).not.toBe(7)
  })
})
