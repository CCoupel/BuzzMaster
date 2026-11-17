/**
 * #144 (v12.0.0) — commandes admin et animateur pendant la montée d'un tour (contracts/roue.md §5.1, §8).
 * Admin et animateur : « Forcer l'arrêt » grisé tant que
 * FULL_SPEED=false, actif à pleine vitesse ; libellé clair pour ROUE_ERROR NOT_FULL_SPEED. Le serveur reste juge.
 */
import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, cleanup, fireEvent, screen } from '@testing-library/react'
import RouePanel from './RouePanel'
import { AnimRoueView } from './AnimRoue'
import { RoueErrorLine } from './RoueControls'

afterEach(() => cleanup())

const wheel = (over = {}) => ({ LAYOUT: 'strip', SPIN_PERIOD_MS: 2500, SPIN_MAX_MS: 10000, SPIN_UP_MS: 10000, FULL_SPEED: false,
  FLIP_INTERVAL_MS: 600, CURRENT: -1, STOP: null, SEGMENTS: [{ ID: 's0', KIND: 'value', VALUE: 100 }, { ID: 's1', KIND: 'bankrupt', VALUE: 0 }], ...over })
const gs = (w = {}, over = {}) => ({
  phase: 'STARTED',
  question: { TYPE: 'ROUE', ROUE_MODE: 'TANT_QUE_JE_GAGNE', ROUE_GAINS_RULE: 'WINNER_ONLY', ROUE_PHRASE: 'BONJOUR LE MONDE', ROUE_PRESENTATION: 'Salutation' },
  ROUE_SUBPHASE: 'SPINNING', ROUE_STEP: 2, ROUE_TEAMS: ['Rouges', 'Bleus'], ROUE_CURRENT_TEAM: 'Rouges', ROUE_CURRENT_TEAM_COLOR: [239, 68, 68],
  ROUE_NEXT_TEAM: 'Bleus', ROUE_NEXT_DEFAULT: '', ROUE_TEAM_BANK: { Rouges: 0, Bleus: 0 }, ROUE_TURN_GAIN: 0,
  ROUE_BOARD: { COLS: 14, ROWS: ['_______ __'] }, ROUE_LETTERS: [], ROUE_SHOW_LETTERS: true, ROUE_CONSONANTS_LEFT: true,
  ROUE_WHEEL: wheel(w),
  ROUE_LAST_EVENT: { SEQ: 0, KIND: '', TEAM: '', LETTER: '', COUNT: 0, VALUE: 0, GAIN: 0 }, ROUE_WINNER: '', ROUE_AWARDS: {}, ...over,
})
const teams = { Rouges: { COLOR: [239, 68, 68] }, Bleus: { COLOR: [59, 130, 246] } }
const actionsMock = () => ({ setTeams: vi.fn(), spin: vi.fn(), stop: vi.fn(), letter: vi.fn(), flip: vi.fn(), answer: vi.fn(), next: vi.fn(), pass: vi.fn(), reveal: vi.fn() })
const forcer = () => screen.getByRole('button', { name: /Forcer l'arrêt/ })

describe('Admin (RouePanel) — « Forcer l\'arrêt » seulement à pleine vitesse (décision du 2026-11-17)', () => {
  it('pendant la montée : grisé, un clic n\'envoie rien', () => {
    const a = actionsMock()
    render(<RouePanel gameState={gs({ FULL_SPEED: false })} actions={a} />)
    expect(forcer().disabled).toBe(true)
    fireEvent.click(forcer())
    expect(a.stop).not.toHaveBeenCalled()
  })
  it('à pleine vitesse : actif, un clic envoie ROUE_STOP', () => {
    const a = actionsMock()
    render(<RouePanel gameState={gs({ FULL_SPEED: true })} actions={a} />)
    expect(forcer().disabled).toBe(false)
    fireEvent.click(forcer())
    expect(a.stop).toHaveBeenCalledTimes(1)
  })
  it('devient actif à l\'UPDATE FULL_SPEED=true', () => {
    const a = actionsMock()
    const { rerender } = render(<RouePanel gameState={gs({ FULL_SPEED: false })} actions={a} />)
    rerender(<RouePanel gameState={gs({ FULL_SPEED: true })} actions={a} />)
    expect(forcer().disabled).toBe(false)
  })
})

describe('Animateur (AnimRoue) — « Forcer l\'arrêt » seulement à pleine vitesse', () => {
  const stepBtn = (c) => Array.from(c.querySelectorAll('.anim-roue-steps button')).find(b => /Forcer/.test(b.textContent))
  it('pendant la montée : grisé, un clic n\'envoie rien', () => {
    const a = actionsMock()
    const { container } = render(<AnimRoueView gameState={gs({ FULL_SPEED: false })} teams={teams} actions={a} />)
    expect(stepBtn(container).disabled).toBe(true)
    fireEvent.click(stepBtn(container))
    expect(a.stop).not.toHaveBeenCalled()
    expect(Array.from(container.querySelectorAll('.anim-roue-steps button')).filter(b => !b.disabled)).toHaveLength(0)
  })
  it('à pleine vitesse : actif, un clic envoie ROUE_STOP', () => {
    const a = actionsMock()
    const { container } = render(<AnimRoueView gameState={gs({ FULL_SPEED: true })} teams={teams} actions={a} />)
    expect(stepBtn(container).disabled).toBe(false)
    fireEvent.click(stepBtn(container))
    expect(a.stop).toHaveBeenCalledTimes(1)
  })
  it('le bouton devient actif à l\'UPDATE FULL_SPEED=true', () => {
    const a = actionsMock()
    const { container, rerender } = render(<AnimRoueView gameState={gs({ FULL_SPEED: false })} teams={teams} actions={a} />)
    rerender(<AnimRoueView gameState={gs({ FULL_SPEED: true })} teams={teams} actions={a} />)
    expect(stepBtn(container).disabled).toBe(false)
  })
  it('pendant la montée : message « la roue prend de la vitesse » (et pas « la roue tourne » seul)', () => {
    const { container } = render(<AnimRoueView gameState={gs({ FULL_SPEED: false })} teams={teams} actions={actionsMock()} />)
    expect(container.textContent).toMatch(/vitesse/i)
  })
})

describe('ROUE_ERROR NOT_FULL_SPEED', () => {
  it('libellé lisible, jamais le code brut', () => {
    render(<RoueErrorLine error={{ CODE: 'NOT_FULL_SPEED', SEQ: 1 }} />)
    const el = screen.getByRole('alert')
    expect(el.textContent).toMatch(/vitesse/i)
    expect(el.textContent).not.toMatch(/NOT_FULL_SPEED/)
  })
})
