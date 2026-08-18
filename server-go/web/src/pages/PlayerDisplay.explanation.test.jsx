import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import PlayerDisplay from './PlayerDisplay'

// ---------------------------------------------------------------------------
// PlayerDisplay — non-affichage de la note d'explication (v6.4.x, #168,
// AC18). Suit le patron de mocks de PlayerDisplay.ardoise.test.jsx.
//
// EXPLANATION voyage bien jusqu'ici dans le nœud QUESTION du payload (aucun
// filtrage réseau, décision GATE 1.5 — contracts/models.md §EXPLANATION) :
// la garantie testée ici est D'AFFICHAGE, pas de confidentialité — comme
// ANSWER (#163). Couvre TV (isVPlayer=false) ET VJoueur (isVPlayer=true),
// les deux rendus par ce même composant.
// ---------------------------------------------------------------------------

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

const NOTE_TEXT = 'Note animateur — ne doit apparaître ni sur TV ni sur VJoueur.'

const QUESTION_WITH_NOTE = {
  ID: 'q1',
  TYPE: 'SPEEDY',
  QUESTION: 'Quelle est la capitale de la France ?',
  ANSWER: 'Paris',
  EXPLANATION: NOTE_TEXT,
}

const TEAMS = {
  'Équipe A': { NAME: 'Équipe A', SCORE: 10, COLOR: [99, 102, 241] },
}

const BUMPERS = {
  'AA:00:00:00:00:01': { TEAM: 'Équipe A', NAME: 'VJoueur-A', IS_VPLAYER: true, CONNECTED: true, COLOR: [99, 102, 241] },
}

const makeMock = ({ phase = 'STARTED', question = QUESTION_WITH_NOTE } = {}) => ({
  gameState: {
    phase,
    remote: 'GAME',
    timer: 20,
    totalTime: 30,
    question,
    MEMORY_PARTICIPATING_TEAMS: [],
    MEMOTION_PARTICIPATING_TEAMS: [],
    MEMOTION_CARD_STATES: {},
    MEMOTION_CARD_TEAMS: {},
    MEMOTION_CURRENT_TEAM: null,
    MEMOTION_SELECTED: null,
    newGameBackgrounds: [],
  },
  teams: TEAMS,
  bumpers: BUMPERS,
  flipMemoryCard: vi.fn(),
  showQRCode: false,
  selectMotionCard: vi.fn(),
})

beforeEach(() => {
  vi.clearAllMocks()
})

describe('PlayerDisplay — TV, note d\'explication jamais rendue (AC18)', () => {
  it.each(['STARTED', 'STOPPED', 'REVEALED'])(
    'phase %s : EXPLANATION absente du DOM alors que la question est bien affichée',
    (phase) => {
      useGame.mockReturnValue(makeMock({ phase }))
      const { container } = render(<PlayerDisplay />)
      expect(screen.getByText('Quelle est la capitale de la France ?')).toBeInTheDocument()
      expect(container.textContent).not.toContain(NOTE_TEXT)
    }
  )

  it('même en REVEALED (ANSWER affichée), la note n\'apparaît nulle part', () => {
    useGame.mockReturnValue(makeMock({ phase: 'REVEALED' }))
    const { container } = render(<PlayerDisplay />)
    expect(screen.getByText('Paris')).toBeInTheDocument()
    expect(container.textContent).not.toContain(NOTE_TEXT)
  })
})

describe('PlayerDisplay — VJoueur (/player), note d\'explication jamais rendue (AC18)', () => {
  it('EXPLANATION absente du DOM côté VJoueur', () => {
    useGame.mockReturnValue(makeMock({ phase: 'STARTED' }))
    const { container } = render(
      <PlayerDisplay
        isVPlayer={true}
        playerName="Joueur1"
        playerNameColor={[99, 102, 241]}
        teamName="Équipe A"
        teamColor={[99, 102, 241]}
      />
    )
    expect(container.textContent).not.toContain(NOTE_TEXT)
  })
})
