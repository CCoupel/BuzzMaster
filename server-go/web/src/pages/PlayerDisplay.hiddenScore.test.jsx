import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, act } from '@testing-library/react'
import PlayerDisplay from './PlayerDisplay'
import confetti from 'canvas-confetti'

// #252 (v11.2) — SCORE_CHANGE : une hausse de score avec DISPLAY=false (correction cachée) n'anime
// RIEN (ni « +X pts », ni célébration) ; une hausse visible garde le comportement actuel et
// n'anime que sa propre différence.

vi.mock('nosleep.js', () => ({
  default: class NoSleep {
    enable() { return Promise.resolve() }
    disable() {}
  },
}))

vi.mock('canvas-confetti', () => ({ default: vi.fn() }))

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('../components/Timer', () => ({
  default: ({ currentTime }) => <div data-testid="timer">{currentTime}</div>,
}))

vi.mock('../components/Podium', () => ({
  default: () => <div data-testid="podium" />,
}))

vi.mock('../components/QRCodeOverlay', () => ({
  default: () => null,
}))

vi.mock('../components/QRCodeDisplay', () => ({
  default: () => null,
}))

vi.mock('./QuestionsPage', () => ({
  CATEGORIES: [],
}))

vi.mock('../constants/colors', () => ({
  getCategoryColor: vi.fn(() => '#8b5cf6'),
}))

vi.mock('../utils/colorUtils', () => ({
  getRgbColor: vi.fn((color) => (Array.isArray(color) ? `rgb(${color.join(',')})` : color)),
}))

vi.mock('./PlayerDisplay.css', () => ({}))
vi.mock('../styles/neon.css', () => ({}))

import { useGame } from '../hooks/GameContext'

const COLOR = [99, 102, 241]
const mk = (scoreA, scoreB, sc) => ({
  gameState: {
    phase: 'STOPPED', remote: 'GAME', timer: 0, totalTime: 30, question: null,
    MEMORY_PARTICIPATING_TEAMS: [], MEMOTION_PARTICIPATING_TEAMS: [], MEMOTION_CARD_STATES: {},
    MEMOTION_CARD_TEAMS: {}, MEMOTION_CURRENT_TEAM: null, MEMOTION_SELECTED: null, newGameBackgrounds: [],
    SCORE_CHANGE: sc,
  },
  teams: { A: { NAME: 'A', SCORE: scoreA, COLOR }, B: { NAME: 'B', SCORE: scoreB, COLOR: [234, 179, 8] } },
  bumpers: {}, flipMemoryCard: vi.fn(), showQRCode: false, selectMotionCard: vi.fn(),
})
const SC = (SEQ, DISPLAY, TEAMS = [], PLAYERS = []) => ({ SEQ, DISPLAY, TEAMS, PLAYERS })

describe('PlayerDisplay — correction cachée (#252, SCORE_CHANGE)', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    Object.defineProperty(document.documentElement, 'requestFullscreen', { value: vi.fn().mockResolvedValue(undefined), writable: true, configurable: true })
    confetti.mockClear?.()
  })
  afterEach(() => vi.useRealTimers())

  // #265 : l'annonce « +X pts » est une carte qui apparaît à l'arrivée de la fusée (≈ 0,3 × 3,2 s) ;
  // `step` laisse passer 1 s simulée, la carte est alors visible (fenêtre de lecture jusqu'à 2,75 s).
  const step = (view, game) => { useGame.mockReturnValue(game); act(() => { view.rerender(<PlayerDisplay />) }); act(() => { vi.advanceTimersByTime(1000) }) }
  const award = (view) => view.container.querySelector('.tv-award-layer')
  const start = () => {
    useGame.mockReturnValue(mk(10, 5, SC(0, true)))
    return render(<PlayerDisplay />)
  }

  it('hausse visible (DISPLAY=true) : « +X pts » et célébration (comportement actuel)', () => {
    const view = start()
    step(view, mk(13, 5, SC(1, true, ['A'])))
    expect(award(view)?.textContent).toMatch(/\+3 pts/) // #265 : feu d'artifice (fusée + étincelles) à la place du confetti
  })

  it('hausse cachée (DISPLAY=false) : aucun « +X pts », aucune célébration, nouvelle valeur affichée', () => {
    const view = start()
    confetti.mockClear()
    step(view, mk(13, 5, SC(1, false, ['A'])))
    expect(award(view)).toBeNull()
    expect(confetti).not.toHaveBeenCalled()
  })

  it('hausse visible suivante : n\'anime que SA différence (pas celle de la correction cachée)', () => {
    const view = start()
    step(view, mk(13, 5, SC(1, false, ['A'])))
    confetti.mockClear()
    step(view, mk(15, 5, SC(2, true, ['A'])))
    expect(award(view)?.textContent).toMatch(/\+2 pts/)
    expect(award(view)?.textContent).not.toMatch(/\+5 pts/)
  })

  it('correction cachée sur l\'équipe A : une hausse visible de B simultanée n\'est pas masquée', () => {
    const view = start()
    step(view, mk(13, 8, SC(1, false, ['A'])))
    expect(award(view)?.textContent).toMatch(/B/)
  })

  it('même SEQ rejoué (UPDATE sans nouvelle modification) : aucun effet de bord', () => {
    const view = start()
    step(view, mk(13, 5, SC(1, false, ['A'])))
    step(view, mk(13, 5, SC(1, false, ['A'])))
    expect(award(view)).toBeNull()
  })
})
