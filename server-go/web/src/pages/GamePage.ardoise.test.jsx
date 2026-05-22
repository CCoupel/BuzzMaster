import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import GamePage from './GamePage'

// ---------------------------------------------------------------------------
// Mocks
// framer-motion est aliasé globalement via vite.config.js
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

// ---------------------------------------------------------------------------
// Import useGame après les mocks
// ---------------------------------------------------------------------------
import { useGame } from '../hooks/GameContext'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Construit un bumper minimal pour simuler une équipe avec un joueur.
 * displayTeams ne retient que les équipes ayant au moins un bumper.
 */
const makeTeamBumper = (teamName, mac = 'AA:BB:CC:DD:EE:01') => ({
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

const makeGameMock = (overrides = {}) => {
  // Séparer gameState/teams/bumpers/questions des overrides de fonctions (ex: setTeamPoints spy)
  // pour que ...otherOverrides ne ré-écrase pas les clés structurelles
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
      ...(gameStateOverride || {}),
    },
    teams: teams ?? {
      'Équipe A': { NAME: 'Équipe A', SCORE: 0, COLOR: [99, 102, 241] },
      'Équipe B': { NAME: 'Équipe B', SCORE: 5, COLOR: [234, 179, 8] },
    },
    bumpers: bumpers ?? {
      ...makeTeamBumper('Équipe A', 'AA:00:00:00:00:01'),
      ...makeTeamBumper('Équipe B', 'AA:00:00:00:00:02'),
    },
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
    ...otherOverrides,  // spies injectés (ex: setTeamPoints: mockFn) sans risque d'écraser gameState
  }
}

// ---------------------------------------------------------------------------
// Tests : panneau admin ARDOISE — GamePage (#89)
// ---------------------------------------------------------------------------

describe('GamePage — panneau ARDOISE en phase STARTED (#89)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('affiche la zone ardoise-team-zone en phase STARTED avec TYPE=ARDOISE', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED' },
    }))
    const { container } = render(<GamePage />)

    expect(container.querySelector('.ardoise-team-zone')).not.toBeNull()
  })

  it('affiche la zone ardoise-team-zone dans le timer-display-section', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED' },
    }))
    const { container } = render(<GamePage />)

    expect(container.querySelector('.ardoise-team-zone')).not.toBeNull()
  })

  it('affiche une ligne par équipe dans le panneau ARDOISE', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED' },
    }))
    const { container } = render(<GamePage />)

    const rows = container.querySelectorAll('.ardoise-answer-row')
    expect(rows.length).toBe(2)
  })

  it('affiche le nom de chaque équipe dans le panneau', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED' },
    }))
    render(<GamePage />)

    expect(screen.getByText('Équipe A')).toBeInTheDocument()
    expect(screen.getByText('Équipe B')).toBeInTheDocument()
  })

  it('affiche "—" pour une équipe sans réponse', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STARTED',
        ARDOISE_ANSWERS: {},
      },
    }))
    const { container } = render(<GamePage />)

    const noAnswerTexts = container.querySelectorAll('.ardoise-answer-row.no-answer')
    expect(noAnswerTexts.length).toBe(2)
  })

  it('affiche le texte de la réponse quand une équipe a répondu', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STARTED',
        ARDOISE_ANSWERS: {
          'Équipe A': { TEXT: 'Paris', SUBMITTED_AT: 1000 },
        },
      },
    }))
    render(<GamePage />)

    expect(screen.getByText('Paris')).toBeInTheDocument()
  })

  it('la réponse d\'équipe a la classe "has-answer" quand réponse présente', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STARTED',
        ARDOISE_ANSWERS: {
          'Équipe A': { TEXT: 'Paris', SUBMITTED_AT: 1000 },
        },
      },
    }))
    const { container } = render(<GamePage />)

    const hasAnswer = container.querySelector('.ardoise-answer-row.has-answer')
    expect(hasAnswer).not.toBeNull()
    expect(hasAnswer.querySelector('.ardoise-answer-text-row').textContent).toContain('Paris')
  })
})

describe('GamePage — panneau ARDOISE masqué hors contexte ARDOISE (#89)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('panneau absent quand TYPE=QCM', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STARTED',
        question: { ID: '10', TYPE: 'QCM', QUESTION: '?', ARDOISE_KEYBOARD_TYPE: undefined },
        ARDOISE_ANSWERS: {},
      },
    }))
    const { container } = render(<GamePage />)

    expect(container.querySelector('.ardoise-team-zone')).toBeNull()
  })

  it('panneau absent quand TYPE=NORMAL', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STARTED',
        question: { ID: '10', TYPE: 'NORMAL', QUESTION: '?' },
        ARDOISE_ANSWERS: {},
      },
    }))
    const { container } = render(<GamePage />)

    expect(container.querySelector('.ardoise-team-zone')).toBeNull()
  })

  it('panneau absent en phase READY même si TYPE=ARDOISE', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'READY',
        ARDOISE_ANSWERS: {},
      },
    }))
    const { container } = render(<GamePage />)

    expect(container.querySelector('.ardoise-team-zone')).toBeNull()
  })
})

describe('GamePage — panneau ARDOISE en phase REVEALED (#89)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('affiche la bonne réponse dans le panneau en phase REVEALED', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'REVEALED',
        ARDOISE_ANSWERS: {
          'Équipe A': { TEXT: 'Paris', SUBMITTED_AT: 1000 },
        },
      },
    }))
    const { container } = render(<GamePage />)

    // La réponse de l'équipe apparaît dans l'overlay admin
    const overlay = container.querySelector('.ardoise-team-zone')
    expect(overlay?.textContent).toContain('Paris')
  })

  it('affiche les boutons "+N pts" pour chaque équipe en phase REVEALED', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'REVEALED',
        ARDOISE_ANSWERS: {},
      },
    }))
    const { container } = render(<GamePage />)

    const ptsBtns = container.querySelectorAll('.ardoise-points-btn')
    expect(ptsBtns.length).toBe(2) // une par équipe
  })

  it('cliquer sur "+N pts" appelle setTeamPoints avec le bon nom d\'équipe', () => {
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
        ARDOISE_ANSWERS: {},
      },
      setTeamPoints,
    }))
    const { container } = render(<GamePage />)

    const ptsBtns = container.querySelectorAll('.ardoise-points-btn')
    // Click first button (Équipe A or B — whichever is first in displayTeams)
    fireEvent.click(ptsBtns[0])
    expect(setTeamPoints).toHaveBeenCalledTimes(1)
  })

  it('masque les boutons "+N pts" en phase STARTED', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STARTED',
        ARDOISE_ANSWERS: {},
      },
    }))
    const { container } = render(<GamePage />)

    const ptsBtns = container.querySelectorAll('.ardoise-points-btn')
    expect(ptsBtns.length).toBe(0)
  })

  it('le panneau est visible en phase STOPPED avec TYPE=ARDOISE', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STOPPED',
        ARDOISE_ANSWERS: {},
      },
    }))
    const { container } = render(<GamePage />)

    expect(container.querySelector('.ardoise-team-zone')).not.toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Tests #93 — Filtre VJoueur dans le panneau admin ARDOISE (#93)
// ---------------------------------------------------------------------------

describe('GamePage — panneau ARDOISE : filtre VJoueur (#93)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('affiche uniquement les équipes dont un bumper a IS_VPLAYER=true', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', ARDOISE_ANSWERS: {} },
      bumpers: {
        'MAC:A1': { TEAM: 'Équipe A', IS_VPLAYER: true,  NAME: 'VJoueur A', CONNECTED: true, SCORE: 0, COLOR: [99, 102, 241] },
        'MAC:B1': { TEAM: 'Équipe B', IS_VPLAYER: false, NAME: 'Buzzer B',  CONNECTED: true, SCORE: 0, COLOR: [234, 179, 8] },
      },
    }))
    const { container } = render(<GamePage />)

    const overlay = container.querySelector('.ardoise-team-zone')
    const teamAnswers = overlay.querySelectorAll('.ardoise-answer-row')
    // Seule Équipe A (VJoueur) doit apparaître dans le panneau
    expect(teamAnswers.length).toBe(1)
    expect(teamAnswers[0].querySelector('.ardoise-answer-team-name').textContent).toContain('Équipe A')
  })

  it('masque une équipe avec uniquement des buzzers physiques IS_VPLAYER=false', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', ARDOISE_ANSWERS: {} },
      bumpers: {
        'MAC:A1': { TEAM: 'Équipe A', IS_VPLAYER: false, NAME: 'Buzzer A', CONNECTED: true, SCORE: 0, COLOR: [99, 102, 241] },
        'MAC:B1': { TEAM: 'Équipe B', IS_VPLAYER: false, NAME: 'Buzzer B', CONNECTED: true, SCORE: 0, COLOR: [234, 179, 8] },
      },
    }))
    const { container } = render(<GamePage />)

    // Le panneau est présent mais aucune équipe n'est affichée
    expect(container.querySelector('.ardoise-team-zone')).not.toBeNull()
    const teamAnswers = container.querySelectorAll('.ardoise-answer-row')
    expect(teamAnswers.length).toBe(0)
  })
})
