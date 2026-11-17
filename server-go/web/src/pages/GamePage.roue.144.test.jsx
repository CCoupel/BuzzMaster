import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, fireEvent } from '@testing-library/react'
import GamePage from './GamePage'

// ---------------------------------------------------------------------------
// #144 : GamePage monte RouePanel pour une question ROUE
//
// Contrat : contracts/models.md — ArdoiseAnswer.STARTED_AT
// Plan    : _work/reports/plan-20260727-093000.md
// Décision GATE 2 : le délai est affiché avec 3 décimales (ex: "4.732 s"),
// même convention que les temps de réaction au buzzer.
//
// Ces tests couvrent uniquement le tri et l'affichage du délai — les tests
// de rendu de base du panneau (visibilité, boutons, filtre VJoueur) sont
// dans GamePage.ardoise.test.jsx (#89/#93) et restent inchangés.
// ---------------------------------------------------------------------------

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('../hooks/useCategoryFilter', () => ({
  useCategoryFilter: vi.fn((questions) => ({
    selectedCategories: new Set(),
    availableCategories: [],
    filteredQuestions: questions ? Object.values(questions) : [],
    toggleCategoryFilter: vi.fn(),
    clearCategoryFilters: vi.fn(),
  })),
}))

vi.mock('../components/Timer', () => ({
  default: ({ phase }) => <div data-testid="timer" data-phase={phase} />,
}))

vi.mock('../components/QuestionPreview', () => ({
  default: () => <div data-testid="question-preview" />,
}))

vi.mock('../components/TeamCard', () => ({
  default: ({ name }) => <div data-testid={`team-card-${name}`} />,
  OtaAllModal: () => null,
}))

vi.mock('../components/QuestionCard', () => ({
  default: ({ question, onClick }) => (
    <div
      data-testid={`question-card-${question.ID}`}
      onClick={() => onClick && onClick(question)}
    />
  ),
  CATEGORIES: {},
}))

vi.mock('../components/Card', () => ({
  default: ({ children, className, padding, variant, ...rest }) => (
    <div className={className} {...rest}>{children}</div>
  ),
}))

vi.mock('../components/Button', () => ({
  default: ({ children, onClick, disabled, variant, size, ...rest }) => (
    <button onClick={onClick} disabled={disabled} {...rest}>{children}</button>
  ),
}))

vi.mock('../utils/colorUtils', () => ({
  getRgbColor: vi.fn((color) => (Array.isArray(color) ? `rgb(${color.join(',')})` : color || '#888')),
}))

vi.mock('./GamePage.css', () => ({}))

import { useGame } from '../hooks/GameContext'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const makeTeamBumper = (teamName, mac) => ({
  [mac]: {
    TEAM: teamName,
    NAME: `Buzzer-${teamName}`,
    SCORE: 0,
    IS_VIRTUAL: false,
    IS_VPLAYER: true,
    CONNECTED: true,
    COLOR: [99, 102, 241],
  },
})

// Trois équipes A/B/C insérées dans cet ordre — les tests vérifient que
// l'ordre affiché ne dépend PAS de cet ordre d'insertion mais de STARTED_AT.
const THREE_TEAMS = {
  'Équipe A': { NAME: 'Équipe A', SCORE: 0, COLOR: [255, 0, 0] },
  'Équipe B': { NAME: 'Équipe B', SCORE: 0, COLOR: [0, 255, 0] },
  'Équipe C': { NAME: 'Équipe C', SCORE: 0, COLOR: [0, 0, 255] },
}

const THREE_BUMPERS = {
  ...makeTeamBumper('Équipe A', 'AA:00:00:00:00:01'),
  ...makeTeamBumper('Équipe B', 'AA:00:00:00:00:02'),
  ...makeTeamBumper('Équipe C', 'AA:00:00:00:00:03'),
}

const makeGameMock = (overrides = {}) => {
  const { gameState: gameStateOverride, teams, bumpers, questions, ...otherOverrides } = overrides
  return {
    gameState: {
      phase: 'STARTED',
      question: {
        ID: '10',
        TYPE: 'ARDOISE',
        QUESTION: 'Quelle est la capitale de la France ?',
        ANSWER: 'Paris',
        POINTS: '2',
        ARDOISE_KEYBOARD_TYPE: 'AZERTY',
      },
      remote: 'GAME',
      timer: 15,
      totalTime: 30,
      MEMORY_PARTICIPATING_TEAMS: [],
      ARDOISE_ANSWERS: {},
      gameTime: 1000000, // question start, in microseconds
      ...(gameStateOverride || {}),
    },
    teams: teams ?? THREE_TEAMS,
    bumpers: bumpers ?? THREE_BUMPERS,
    questions: questions ?? {
      '10': { ID: '10', STATUS: 'STARTED', ORDER: 1 },
    },
    startGame: vi.fn(),
    stopGame: vi.fn(),
    pauseGame: vi.fn(),
    continueGame: vi.fn(),
    revealAnswer: vi.fn(),
    selectQuestion: vi.fn(),
    setRemoteDisplay: vi.fn(),
    setBumperPoints: vi.fn(),
    setTeamPoints: vi.fn(),
    forceReady: vi.fn(),
    simulateButton: vi.fn(),
    simulatePong: vi.fn(),
    sendMessage: vi.fn(),
    ...otherOverrides,
  }
}


const ROUE_BASE = {
  phase: 'STARTED',
  question: { ID: 'rq', TYPE: 'ROUE', QUESTION: 'Roue', ROUE_MODE: 'TANT_QUE_JE_GAGNE', ROUE_PHRASE: 'BONJOUR LE MONDE' },
  remote: 'GAME', timer: 0, totalTime: 0, MEMORY_PARTICIPATING_TEAMS: [], ARDOISE_ANSWERS: {},
  ROUE_SUBPHASE: 'TURN', ROUE_STEP: 1, ROUE_TEAMS: ['Équipe A', 'Équipe B'], ROUE_CURRENT_TEAM: 'Équipe A', ROUE_CURRENT_TEAM_COLOR: [255, 0, 0],
  ROUE_TEAM_BANK: { 'Équipe A': 0, 'Équipe B': 0 }, ROUE_CONSONANTS_LEFT: true, ROUE_LETTERS: [], ROUE_BOARD: { COLS: 14, ROWS: ['_______ __'] },
  ROUE_WHEEL: { LAYOUT: 'strip', SPIN_MAX_MS: 10000, SEGMENTS: [], CURRENT: -1, STOP: null }, ROUE_LAST_EVENT: { SEQ: 0 }, ROUE_WINNER: '', ROUE_AWARDS: {},
}

describe('GamePage — type ROUE (#144, maquette M2)', () => {
  beforeEach(() => { vi.clearAllMocks() })

  it('monte le panneau de commandes ROUE pendant la manche', () => {
    useGame.mockReturnValue(makeGameMock({ gameState: ROUE_BASE, questions: { rq: { ID: 'rq', STATUS: 'STARTED', ORDER: 1 } } }))
    const { container, getByRole } = render(<GamePage />)
    expect(container.querySelector('[data-testid="roue-panel"]')).not.toBeNull()
    expect(getByRole('button', { name: /Lancer la roue/ })).toBeTruthy()
  })

  it('« Lancer la roue » envoie ROUE_SPIN par sendMessage', () => {
    const mock = makeGameMock({ gameState: ROUE_BASE, questions: { rq: { ID: 'rq', STATUS: 'STARTED', ORDER: 1 } } })
    useGame.mockReturnValue(mock)
    const { getByRole } = render(<GamePage />)
    fireEvent.click(getByRole('button', { name: /Lancer la roue/ }))
    expect(mock.sendMessage).toHaveBeenCalledWith('ROUE_SPIN', {})
  })

  it('PREPARE : choix des équipes envoie ROUE_SET_TEAMS', () => {
    const readyTeams = Object.fromEntries(Object.entries(THREE_TEAMS).map(([k, v]) => [k, { ...v, READY: true }]))
    const mock = makeGameMock({ teams: readyTeams, gameState: { ...ROUE_BASE, phase: 'PREPARE', ROUE_TEAMS: [] }, questions: { rq: { ID: 'rq', STATUS: 'READY', ORDER: 1 } } })
    useGame.mockReturnValue(mock)
    const { container } = render(<GamePage />)
    const box = container.querySelector('.roue-pick input[type="checkbox"]:not([disabled])')
    expect(box).not.toBeNull()
    fireEvent.click(box)
    const call = mock.sendMessage.mock.calls.find(c => c[0] === 'ROUE_SET_TEAMS')
    expect(call).toBeTruthy()
    expect(call[1].TEAMS).toHaveLength(1)
  })

  it('un refus serveur (roueError) est affiché', () => {
    useGame.mockReturnValue(makeGameMock({ gameState: ROUE_BASE, roueError: { CODE: 'WRONG_SUBPHASE', MESSAGE: '', SEQ: 1 }, questions: { rq: { ID: 'rq', STATUS: 'STARTED', ORDER: 1 } } }))
    const { getByRole } = render(<GamePage />)
    expect(getByRole('alert').textContent).toMatch(/déjà prise en compte/)
  })

  it('non-régression : une question ARDOISE ne monte pas le panneau ROUE', () => {
    useGame.mockReturnValue(makeGameMock())
    const { container } = render(<GamePage />)
    expect(container.querySelector('[data-testid="roue-panel"]')).toBeNull()
  })
})
