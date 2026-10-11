import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import PlayerDisplay from './PlayerDisplay'

// #245 (lot V3-3) — TV : pastille du nom de la manche (ROUND_INDEX > 0) et titre de manche sur l'écran d'attente de la
// première question (FIRST_OF_ROUND), rien sans manche. Maquette (g‴).

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

const TEAMS = { A: { NAME: 'A', SCORE: 0, COLOR: [0, 0, 255] } }
const QUESTION = { ID: '9', TYPE: 'QCM', QUESTION: 'Fleuve ?', ANSWER: 'La Seine', POINTS: '3', QCM_ANSWERS: { RED: 'a', GREEN: 'b', YELLOW: 'c', BLUE: 'd' }, QCM_CORRECT: 'GREEN', CATEGORY: 'GEOGRAPHY' }
const ROUNDS = [{ ID: 'r1', NAME: 'Échauffement' }, { ID: 'r2', NAME: 'Culture générale' }]
const ctx = (over = {}) => ({ ROUND_ID: 'r2', ROUND_INDEX: 2, ROUND_COUNT: 2, POSITION: 1, TOTAL: 5, FIRST_OF_ROUND: true, ...over })
const mount = ({ phase = 'READY', question = QUESTION, rounds = ROUNDS, roundContext = ctx(), isVPlayer = false } = {}) => {
  useGame.mockReturnValue({
    gameState: {
      phase, remote: 'GAME', timer: 0, totalTime: 30, question, qcmInvalidated: [], ARDOISE_ANSWERS: {},
      MEMORY_PARTICIPATING_TEAMS: [], MEMOTION_PARTICIPATING_TEAMS: [], MEMOTION_CARD_STATES: {}, MEMOTION_CARD_TEAMS: {},
      MEMOTION_CURRENT_TEAM: null, MEMOTION_SELECTED: null, newGameBackgrounds: [], SCORE_CHANGE: { SEQ: 0, DISPLAY: true, TEAMS: [], PLAYERS: [] },
      ROUNDS: rounds, ROUND_CONTEXT: roundContext,
    },
    teams: TEAMS, bumpers: {}, flipMemoryCard: vi.fn(), showQRCode: false, selectMotionCard: vi.fn(),
  })
  return render(<PlayerDisplay isVPlayer={isVPlayer} />)
}

describe('TV — manche (#245, lot V3-3)', () => {
  beforeEach(() => { useGame.mockReset() })

  it('Critical — pastille « Manche 2 · nom » et titre de manche sur l’attente de la première question', () => {
    mount()
    expect(screen.getByTestId('tv-round-pill').textContent).toBe('Manche 2 · Culture générale')
    expect(screen.getByTestId('tv-round-title').textContent).toBe('Manche 2 — Culture générale')
  })

  it('pas de titre quand la question n’est pas la première de la manche (la pastille reste)', () => {
    mount({ roundContext: ctx({ FIRST_OF_ROUND: false, POSITION: 2 }) })
    expect(screen.getByTestId('tv-round-pill')).toBeTruthy()
    expect(screen.queryByTestId('tv-round-title')).toBeNull()
  })

  it('pas de titre hors écran d’attente (STARTED) ; pastille conservée', () => {
    mount({ phase: 'STARTED' })
    expect(screen.queryByTestId('tv-round-title')).toBeNull()
    expect(screen.getByTestId('tv-round-pill')).toBeTruthy()
  })

  it('Critical — rien sans manche : ROUND_INDEX 0 (Sans manche) ou aucune manche', () => {
    mount({ roundContext: ctx({ ROUND_ID: '', ROUND_INDEX: 0, FIRST_OF_ROUND: false }) })
    expect(screen.queryByTestId('tv-round-pill')).toBeNull()
    expect(screen.queryByTestId('tv-round-title')).toBeNull()
  })

  it('rien sans le contexte (serveur plus ancien)', () => {
    mount({ rounds: [], roundContext: undefined })
    expect(screen.queryByTestId('tv-round-pill')).toBeNull()
  })

  it('pas sur la tablette joueur (VJoueur) ni pour une entracte', () => {
    mount({ isVPlayer: true })
    expect(screen.queryByTestId('tv-round-pill')).toBeNull()
  })

  it('TV statique : le CSS de la pastille et du titre n’utilise que des unités d’écran, sans défilement', async () => {
    const { readFileSync } = await import('node:fs')
    const { resolve } = await import('node:path')
    const css = readFileSync(resolve(process.cwd(), 'src/pages/PlayerDisplay.css'), 'utf8')
    const block = css.slice(css.indexOf('.tv-round-pill'))
    expect(block).toMatch(/top:\s*1\.5vh/)
    expect(block).toMatch(/overflow:\s*hidden/)
    expect(block).not.toMatch(/overflow:\s*(auto|scroll)/)
  })
})
