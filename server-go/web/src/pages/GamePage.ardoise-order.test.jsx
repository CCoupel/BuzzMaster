import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, fireEvent } from '@testing-library/react'
import GamePage from './GamePage'

// ---------------------------------------------------------------------------
// Tests : tri chronologique + délai du panneau admin ARDOISE (#117)
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

// Reads the panel rows in DOM order and returns [{ teamName, rank, delayText }].
// The team name lives in a plain <span> alongside the rank/delay spans inside
// `.ardoise-answer-team-name` — excluded explicitly so its text isn't polluted
// by an adjacent "1" (rank) or "4.732 s" (delay).
function readPanelRows(container) {
  const rows = container.querySelectorAll('.ardoise-answer-row')
  return Array.from(rows).map((row) => {
    const nameEl = row.querySelector(
      '.ardoise-answer-team-name span:not(.ardoise-team-dot):not(.ardoise-answer-rank):not(.ardoise-answer-delay)'
    )
    const rankEl = row.querySelector('.ardoise-answer-rank')
    const delayEl = row.querySelector('.ardoise-answer-delay')
    return {
      teamName: nameEl ? nameEl.textContent : '',
      hasAnswer: row.classList.contains('has-answer'),
      rank: rankEl ? rankEl.textContent : null,
      delayText: delayEl ? delayEl.textContent : null,
    }
  })
}

describe('GamePage — tri chronologique du panneau ARDOISE (#117)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('ordonne les équipes ayant répondu par STARTED_AT croissant, indépendamment de l\'ordre de vplayerTeams', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        ARDOISE_ANSWERS: {
          'Équipe A': { TEXT: 'Réponse A', STARTED_AT: 3000000, SUBMITTED_AT: 3000000 },
          'Équipe B': { TEXT: 'Réponse B', STARTED_AT: 1000000, SUBMITTED_AT: 1000000 },
          'Équipe C': { TEXT: 'Réponse C', STARTED_AT: 2000000, SUBMITTED_AT: 2000000 },
        },
      },
    }))
    const { container } = render(<GamePage />)

    const rows = readPanelRows(container)
    // Insertion order was A, B, C — expected display order is B (1s), C (2s), A (3s)
    expect(rows.map(r => r.teamName)).toEqual(['Équipe B', 'Équipe C', 'Équipe A'])
  })

  it('affiche le rang (1, 2, 3…) sur chaque ligne ayant une réponse, dans l\'ordre trié', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        ARDOISE_ANSWERS: {
          'Équipe A': { TEXT: 'Réponse A', STARTED_AT: 3000000, SUBMITTED_AT: 3000000 },
          'Équipe B': { TEXT: 'Réponse B', STARTED_AT: 1000000, SUBMITTED_AT: 1000000 },
          'Équipe C': { TEXT: 'Réponse C', STARTED_AT: 2000000, SUBMITTED_AT: 2000000 },
        },
      },
    }))
    const { container } = render(<GamePage />)

    const rows = readPanelRows(container)
    expect(rows.map(r => r.rank)).toEqual(['1', '2', '3'])
  })

  it('place les équipes sans réponse en fin de liste', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        ARDOISE_ANSWERS: {
          'Équipe C': { TEXT: 'Réponse C', STARTED_AT: 2000000, SUBMITTED_AT: 2000000 },
        },
      },
    }))
    const { container } = render(<GamePage />)

    const rows = readPanelRows(container)
    expect(rows[0].teamName).toBe('Équipe C')
    expect(rows[0].hasAnswer).toBe(true)
    // Équipe A et B (sans réponse) suivent, sans rang ni délai
    expect(rows.slice(1).every(r => !r.hasAnswer)).toBe(true)
    expect(rows.slice(1).every(r => r.rank === null)).toBe(true)
  })

  it('formate le délai à 3 décimales (ex: "4.732 s") depuis gameState.gameTime', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        gameTime: 1000000,
        ARDOISE_ANSWERS: {
          'Équipe A': { TEXT: 'Paris', STARTED_AT: 5732000, SUBMITTED_AT: 5732000 },
        },
      },
    }))
    const { container } = render(<GamePage />)

    const delayEl = container.querySelector('.ardoise-answer-delay')
    expect(delayEl).not.toBeNull()
    expect(delayEl.textContent).toBe('4.732 s')
  })

  it('STARTED_AT=0 (réponse antérieure au correctif) : tri replié sur SUBMITTED_AT, aucun délai affiché', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        ARDOISE_ANSWERS: {
          // Legacy answer: STARTED_AT never set (0), only SUBMITTED_AT is meaningful.
          'Équipe A': { TEXT: 'Ancienne réponse', STARTED_AT: 0, SUBMITTED_AT: 9000000 },
          'Équipe B': { TEXT: 'Nouvelle réponse', STARTED_AT: 3000000, SUBMITTED_AT: 3000000 },
        },
      },
    }))
    const { container } = render(<GamePage />)

    const rows = readPanelRows(container)
    // Équipe B (STARTED_AT=3s) sorts before Équipe A (falls back to SUBMITTED_AT=9s);
    // Équipe C (default 3-team mock, no answer) legitimately trails at the end.
    expect(rows.map(r => r.teamName)).toEqual(['Équipe B', 'Équipe A', 'Équipe C'])

    // No delay must be shown for the legacy (STARTED_AT=0) row
    const legacyRow = Array.from(container.querySelectorAll('.ardoise-answer-row'))
      .find(r => r.textContent.includes('Équipe A'))
    expect(legacyRow.querySelector('.ardoise-answer-delay')).toBeNull()
  })

  it('gameTime absent : aucun délai affiché, pas de NaN à l\'écran', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        gameTime: undefined,
        ARDOISE_ANSWERS: {
          'Équipe A': { TEXT: 'Paris', STARTED_AT: 5732000, SUBMITTED_AT: 5732000 },
        },
      },
    }))
    const { container } = render(<GamePage />)

    expect(container.querySelector('.ardoise-answer-delay')).toBeNull()
    expect(container.textContent).not.toContain('NaN')
  })

  it('délai négatif (gameTime postérieur à STARTED_AT, resynchronisation) : aucun délai affiché, pas de NaN', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        gameTime: 9000000,
        ARDOISE_ANSWERS: {
          'Équipe A': { TEXT: 'Paris', STARTED_AT: 5732000, SUBMITTED_AT: 5732000 },
        },
      },
    }))
    const { container } = render(<GamePage />)

    expect(container.querySelector('.ardoise-answer-delay')).toBeNull()
    expect(container.textContent).not.toContain('NaN')
  })

  it('ordre stable : la mise à jour du texte d\'une équipe ne modifie pas les rangs', () => {
    const baseMock = makeGameMock({
      gameState: {
        ARDOISE_ANSWERS: {
          'Équipe A': { TEXT: 'Par', STARTED_AT: 3000000, SUBMITTED_AT: 4000000 },
          'Équipe B': { TEXT: 'Lo', STARTED_AT: 1000000, SUBMITTED_AT: 4100000 },
        },
      },
    })
    useGame.mockReturnValue(baseMock)
    const { container, rerender } = render(<GamePage />)

    const before = readPanelRows(container).map(r => r.teamName)

    // Équipe A finishes typing — TEXT and SUBMITTED_AT change, STARTED_AT stays frozen.
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        ARDOISE_ANSWERS: {
          'Équipe A': { TEXT: 'Paris', STARTED_AT: 3000000, SUBMITTED_AT: 4200000 },
          'Équipe B': { TEXT: 'Lo', STARTED_AT: 1000000, SUBMITTED_AT: 4100000 },
        },
      },
    }))
    rerender(<GamePage />)

    const after = readPanelRows(container).map(r => r.teamName)
    expect(after).toEqual(before)
  })

  it('le bouton d\'attribution de points reste fonctionnel en REVEALED, sur la ligne correspondant à son équipe triée', () => {
    const setTeamPoints = vi.fn()
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'REVEALED',
        question: {
          ID: '10',
          TYPE: 'ARDOISE',
          QUESTION: '?',
          ANSWER: 'Paris',
          POINTS: '3',
          ARDOISE_KEYBOARD_TYPE: 'AZERTY',
        },
        ARDOISE_ANSWERS: {
          'Équipe A': { TEXT: 'Réponse A', STARTED_AT: 3000000, SUBMITTED_AT: 3000000 },
          'Équipe B': { TEXT: 'Réponse B', STARTED_AT: 1000000, SUBMITTED_AT: 1000000 },
        },
      },
      setTeamPoints,
    }))
    const { container } = render(<GamePage />)

    // Équipe B is ranked first (STARTED_AT earlier) — its points button must be first in the DOM.
    const rows = container.querySelectorAll('.ardoise-answer-row')
    const firstRowBtn = rows[0].querySelector('.ardoise-points-btn')
    expect(rows[0].textContent).toContain('Équipe B')

    fireEvent.click(firstRowBtn)
    expect(setTeamPoints).toHaveBeenCalledWith('Équipe B', 3)
  })
})
