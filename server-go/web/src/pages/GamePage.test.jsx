import { describe, it, test, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import GamePage from './GamePage'

// ---------------------------------------------------------------------------
// Mocks nécessaires au rendu de GamePage
// framer-motion est déjà aliasé globalement via vite.config.js
// ---------------------------------------------------------------------------

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
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

vi.mock('./GamePage.css', () => ({}))

// ---------------------------------------------------------------------------
// Helper : construit un mock useGame minimal
// ---------------------------------------------------------------------------
import { useGame } from '../hooks/GameContext'

const makeGameMock = (overrides = {}) => ({
  gameState: {
    phase: 'STOPPED',
    question: { ID: '2', STATUS: 'STOPPED' },
    remote: 'GAME',
    timer: 0,
    totalTime: 30,
    MEMORY_PARTICIPATING_TEAMS: [],
    ...overrides.gameState,
  },
  teams: overrides.teams ?? {},
  bumpers: overrides.bumpers ?? {},
  questions: overrides.questions ?? {
    '1': { ID: '1', STATUS: 'PLAYED', ORDER: 1 },
    '2': { ID: '2', STATUS: 'STOPPED', ORDER: 2 },
    '3': { ID: '3', STATUS: 'AVAILABLE', ORDER: 3 },
    '4': { ID: '4', STATUS: 'AVAILABLE', ORDER: 4 },
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
  ...overrides,
})

// ---------------------------------------------------------------------------
// Describe : bouton "Question suivante" (nextUnplayedQuestion)
// ---------------------------------------------------------------------------

describe('GamePage - Bouton "Question suivante"', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  // Test 1 : phase STARTED → nextUnplayedQuestion = null → bouton absent
  it('bouton absent quand phase est STARTED', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STARTED',
        question: { ID: '2', STATUS: 'STOPPED' },
        remote: 'GAME',
        timer: 0,
        totalTime: 30,
        MEMORY_PARTICIPATING_TEAMS: [],
      },
    }))

    render(<GamePage />)
    expect(screen.queryByText(/Question suivante/i)).toBeNull()
  })

  // Test 2 : phase STOPPED mais toutes les questions suivantes sont jouées → bouton absent
  it('bouton absent en phase STOPPED si toutes les questions suivantes sont jouées', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STOPPED',
        question: { ID: '3', STATUS: 'STOPPED' },
        remote: 'GAME',
        timer: 0,
        totalTime: 30,
        MEMORY_PARTICIPATING_TEAMS: [],
      },
      questions: {
        '1': { ID: '1', STATUS: 'PLAYED', ORDER: 1 },
        '2': { ID: '2', STATUS: 'PLAYED', ORDER: 2 },
        '3': { ID: '3', STATUS: 'STOPPED', ORDER: 3 },
        '4': { ID: '4', STATUS: 'PLAYED', ORDER: 4 },
      },
    }))

    render(<GamePage />)
    expect(screen.queryByText(/Question suivante/i)).toBeNull()
  })

  // Test 3 : phase STOPPED avec question suivante disponible → bouton visible avec "#ID"
  it('bouton visible en phase STOPPED si question suivante disponible', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STOPPED',
        question: { ID: '2', STATUS: 'STOPPED' },
        remote: 'GAME',
        timer: 0,
        totalTime: 30,
        MEMORY_PARTICIPATING_TEAMS: [],
      },
      // questions par défaut : ID 3 et 4 sont AVAILABLE après ID 2
    }))

    render(<GamePage />)
    expect(screen.getByText(/Question suivante.*#3/i)).toBeInTheDocument()
  })

  // Test 4 : phase REVEALED avec question suivante disponible → bouton visible
  it('bouton visible en phase REVEALED si question suivante disponible', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'REVEALED',
        question: { ID: '2', STATUS: 'STOPPED' },
        remote: 'GAME',
        timer: 0,
        totalTime: 30,
        MEMORY_PARTICIPATING_TEAMS: [],
      },
    }))

    render(<GamePage />)
    expect(screen.getByText(/Question suivante.*#3/i)).toBeInTheDocument()
  })

  // Test 5 : clic sur le bouton → selectQuestion appelé avec le bon ID
  it('clic sur bouton appelle selectQuestion avec l\'ID de la prochaine question', () => {
    const mockSelectQuestion = vi.fn()
    useGame.mockReturnValue(makeGameMock({
      selectQuestion: mockSelectQuestion,
      gameState: {
        phase: 'STOPPED',
        question: { ID: '2', STATUS: 'STOPPED' },
        remote: 'GAME',
        timer: 0,
        totalTime: 30,
        MEMORY_PARTICIPATING_TEAMS: [],
      },
    }))

    render(<GamePage />)
    const btn = screen.getByText(/Question suivante.*#3/i)
    fireEvent.click(btn)

    expect(mockSelectQuestion).toHaveBeenCalledWith('3')
  })

  // Test 6 : question avec STATUS='PREPARE' → considérée non jouée → bouton visible
  it('bouton visible si prochaine question a STATUS=PREPARE', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STOPPED',
        question: { ID: '1', STATUS: 'STOPPED' },
        remote: 'GAME',
        timer: 0,
        totalTime: 30,
        MEMORY_PARTICIPATING_TEAMS: [],
      },
      questions: {
        '1': { ID: '1', STATUS: 'STOPPED', ORDER: 1 },
        '2': { ID: '2', STATUS: 'PREPARE', ORDER: 2 },
      },
    }))

    render(<GamePage />)
    expect(screen.getByText(/Question suivante.*#2/i)).toBeInTheDocument()
  })

  // Test 7 : question avec STATUS='READY' → considérée non jouée → bouton visible
  it('bouton visible si prochaine question a STATUS=READY', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STOPPED',
        question: { ID: '1', STATUS: 'STOPPED' },
        remote: 'GAME',
        timer: 0,
        totalTime: 30,
        MEMORY_PARTICIPATING_TEAMS: [],
      },
      questions: {
        '1': { ID: '1', STATUS: 'STOPPED', ORDER: 1 },
        '2': { ID: '2', STATUS: 'READY', ORDER: 2 },
      },
    }))

    render(<GamePage />)
    expect(screen.getByText(/Question suivante.*#2/i)).toBeInTheDocument()
  })

  // Test 8 : saute les questions jouées pour trouver la vraie prochaine disponible
  it('prochaine question est la première AVAILABLE après la courante (saute les PLAYED)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STOPPED',
        question: { ID: '1', STATUS: 'STOPPED' },
        remote: 'GAME',
        timer: 0,
        totalTime: 30,
        MEMORY_PARTICIPATING_TEAMS: [],
      },
      questions: {
        '1': { ID: '1', STATUS: 'STOPPED', ORDER: 1 },
        '2': { ID: '2', STATUS: 'PLAYED', ORDER: 2 },
        '3': { ID: '3', STATUS: 'AVAILABLE', ORDER: 3 },
      },
    }))

    render(<GamePage />)
    // Doit pointer vers #3 (saute #2 qui est PLAYED)
    expect(screen.getByText(/Question suivante.*#3/i)).toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// Describe : opacité des questions non jouées dans la liste admin
// ---------------------------------------------------------------------------

describe('GamePage - Opacité questions non jouées', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  // Test 7 : opacité 0.5 sur question non jouée en phase STARTED
  it('question non jouée a opacity 0.5 en phase STARTED (hors question courante)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STARTED',
        question: { ID: '1', STATUS: 'PLAYED' },
        remote: 'GAME',
        timer: 15,
        totalTime: 30,
        MEMORY_PARTICIPATING_TEAMS: [],
      },
      questions: {
        '1': { ID: '1', STATUS: 'PLAYED', ORDER: 1 },
        '2': { ID: '2', STATUS: 'AVAILABLE', ORDER: 2 },
      },
    }))

    const { container } = render(<GamePage />)
    // Le wrapper de la question #2 (non jouée, non courante) doit avoir opacity:0.5
    const questionCard2 = container.querySelector('[data-testid="question-card-2"]')
    expect(questionCard2).not.toBeNull()
    const wrapper = questionCard2.parentElement
    expect(wrapper.style.opacity).toBe('0.5')
  })

  // Test 8 : opacité normale (pas de style inline opacity) sur question non jouée en phase STOPPED
  it('question non jouée a opacity normale (non dimmed) en phase STOPPED', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STOPPED',
        question: { ID: '1', STATUS: 'STOPPED' },
        remote: 'GAME',
        timer: 0,
        totalTime: 30,
        MEMORY_PARTICIPATING_TEAMS: [],
      },
      questions: {
        '1': { ID: '1', STATUS: 'STOPPED', ORDER: 1 },
        '2': { ID: '2', STATUS: 'AVAILABLE', ORDER: 2 },
      },
    }))

    const { container } = render(<GamePage />)
    const questionCard2 = container.querySelector('[data-testid="question-card-2"]')
    expect(questionCard2).not.toBeNull()
    const wrapper = questionCard2.parentElement
    // En phase STOPPED, isNextButtonActive=true → dimmed=false → pas de style opacity
    expect(wrapper.style.opacity).toBe('')
  })
})

// ---------------------------------------------------------------------------
// Describe : filtre équipes sans joueurs (displayTeams — issue #45)
// ---------------------------------------------------------------------------

describe('GamePage - Filtre équipes sans joueurs (displayTeams)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  // Test 1 : Une équipe sans bumper ne doit PAS être affichée
  it('équipe avec buzzers vides ([]) n\'est pas affichée', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STOPPED',
        question: { ID: '1', STATUS: 'STOPPED' },
        remote: 'GAME',
        timer: 0,
        totalTime: 30,
        MEMORY_PARTICIPATING_TEAMS: [],
      },
      teams: { 'Equipe Vide': { SCORE: 0 } },
      bumpers: {}, // Aucun bumper → teamBumpers['Equipe Vide'] = undefined → buzzers: []
    }))

    render(<GamePage />)
    // La TeamCard pour "Equipe Vide" ne doit pas être rendue
    expect(screen.queryByTestId('team-card-Equipe Vide')).toBeNull()
  })

  // Test 2 : Une équipe avec au moins un bumper DOIT être affichée
  it('équipe avec un bumper assigné est affichée normalement', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STOPPED',
        question: { ID: '1', STATUS: 'STOPPED' },
        remote: 'GAME',
        timer: 0,
        totalTime: 30,
        MEMORY_PARTICIPATING_TEAMS: [],
      },
      teams: { 'Equipe Pleine': { SCORE: 10 } },
      bumpers: {
        'AA:BB:CC:DD:EE:FF': { TEAM: 'Equipe Pleine', NAME: 'Player1', SCORE: 0, READY: false },
      },
    }))

    render(<GamePage />)
    expect(screen.getByTestId('team-card-Equipe Pleine')).toBeInTheDocument()
  })

  // Test 3 : Filtre sélectif — une équipe avec joueurs visible, une sans joueurs masquée
  it('filtre sélectif : équipe avec joueurs visible, sans joueurs masquée', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STOPPED',
        question: { ID: '1', STATUS: 'STOPPED' },
        remote: 'GAME',
        timer: 0,
        totalTime: 30,
        MEMORY_PARTICIPATING_TEAMS: [],
      },
      teams: {
        'Equipe Avec': { SCORE: 5 },
        'Equipe Sans': { SCORE: 0 },
      },
      bumpers: {
        'AA:BB:CC:DD:EE:FF': { TEAM: 'Equipe Avec', NAME: 'Player1', SCORE: 0, READY: false },
        // Aucun bumper pour 'Equipe Sans'
      },
    }))

    render(<GamePage />)
    expect(screen.getByTestId('team-card-Equipe Avec')).toBeInTheDocument()
    expect(screen.queryByTestId('team-card-Equipe Sans')).toBeNull()
  })

  // Test 4 : Toutes les équipes ont des joueurs → toutes affichées, pas de crash
  it('toutes les équipes avec joueurs sont affichées sans crash', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STOPPED',
        question: { ID: '1', STATUS: 'STOPPED' },
        remote: 'GAME',
        timer: 0,
        totalTime: 30,
        MEMORY_PARTICIPATING_TEAMS: [],
      },
      teams: {
        'Rouge': { SCORE: 10 },
        'Bleu': { SCORE: 8 },
        'Vert': { SCORE: 6 },
      },
      bumpers: {
        'AA:BB:CC:DD:EE:FF': { TEAM: 'Rouge', NAME: 'Player1', SCORE: 0, READY: false },
        'BB:CC:DD:EE:FF:00': { TEAM: 'Bleu', NAME: 'Player2', SCORE: 0, READY: false },
        'CC:DD:EE:FF:00:11': { TEAM: 'Vert', NAME: 'Player3', SCORE: 0, READY: false },
      },
    }))

    render(<GamePage />)
    expect(screen.getByTestId('team-card-Rouge')).toBeInTheDocument()
    expect(screen.getByTestId('team-card-Bleu')).toBeInTheDocument()
    expect(screen.getByTestId('team-card-Vert')).toBeInTheDocument()
  })

  // Test 5 : Aucune équipe du tout → rendu sans erreur (teams vide)
  it('teams vide → aucune TeamCard rendue, pas de crash', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STOPPED',
        question: { ID: '1', STATUS: 'STOPPED' },
        remote: 'GAME',
        timer: 0,
        totalTime: 30,
        MEMORY_PARTICIPATING_TEAMS: [],
      },
      teams: {},
      bumpers: {},
    }))

    expect(() => render(<GamePage />)).not.toThrow()
  })
})

// ---------------------------------------------------------------------------
// Tests existants (logique pure, pas de rendu)
// ---------------------------------------------------------------------------

describe('GamePage - Tri par rapidité de réponse', () => {
  // Test 1 : Calcul temps en ms
  test('Calcul temps: (team.TIME - gameState.GAME_TIME) / 1000', () => {
    const gameTime = 1000000000
    const teamTime = 1000100000
    const expected = 100  // ms

    const result = Math.round((teamTime - gameTime) / 1000)
    expect(result).toBe(expected)
  })

  // Test 2 : Tri par temps (plus rapide en haut)
  test('Équipes triées par temps croissant (rapide → lent)', () => {
    const teams = [
      { TIME: 1000150000 },    // 150ms
      { TIME: 1000100000 },    // 100ms (plus rapide)
      { TIME: 1000200000 },    // 200ms
    ]

    const sorted = teams.sort((a, b) => a.TIME - b.TIME)

    expect(sorted[0].TIME).toBe(1000100000)  // 100ms en premier
    expect(sorted[1].TIME).toBe(1000150000)  // 150ms en second
    expect(sorted[2].TIME).toBe(1000200000)  // 200ms en dernier
  })

  // Test 3 : Équipes non buzzées en bas
  test('Équipes avec TIME=0 toujours en bas', () => {
    const buzzedTeams = [{ TIME: 1000100000 }]
    const nonBuzzedTeams = [{ TIME: 0 }]

    const result = [...buzzedTeams, ...nonBuzzedTeams]

    expect(result[0].TIME).toBeGreaterThan(0)
    expect(result[1].TIME).toBe(0)
  })

  // Test 4 : Tri stable - même temps = ordre préservé
  test('Tri stable: même temps conserve l\'ordre', () => {
    const teams = [
      { name: 'A', TIME: 1000100000 },
      { name: 'B', TIME: 1000100000 },
      { name: 'C', TIME: 1000100000 },
    ]

    const sorted = [...teams].sort((a, b) => a.TIME - b.TIME)

    expect(sorted[0].name).toBe('A')
    expect(sorted[1].name).toBe('B')
    expect(sorted[2].name).toBe('C')
  })

  // Test 5 : Phase-aware - tri uniquement en STARTED/PAUSED/REVEALED
  test('Tri actif UNIQUEMENT en STARTED/PAUSED/REVEALED', () => {
    const phases = ['STARTED', 'PAUSED', 'REVEALED']
    phases.forEach(phase => {
      expect(['STARTED', 'PAUSED', 'REVEALED'].includes(phase)).toBe(true)
    })

    const excludedPhases = ['STOP', 'PREPARE', 'READY']
    excludedPhases.forEach(phase => {
      expect(['STARTED', 'PAUSED', 'REVEALED'].includes(phase)).toBe(false)
    })
  })

  // Test 6 : Badge de classement
  test('Badge de classement: 🏆 pour rang 1, 🥈 pour rang 2, 🥉 pour rang 3', () => {
    const getRankBadge = (r) => {
      if (r === 1) return '🏆'
      if (r === 2) return '🥈'
      if (r === 3) return '🥉'
      return null
    }

    expect(getRankBadge(1)).toBe('🏆')
    expect(getRankBadge(2)).toBe('🥈')
    expect(getRankBadge(3)).toBe('🥉')
    expect(getRankBadge(4)).toBeNull()
  })

  // Test 7 : Tri joueurs - même logique que équipes
  test('Joueurs triés par timestamp croissant (rapide → lent)', () => {
    const buzzers = [
      { mac: 'a', timestamp: 1000200000 },    // 200ms
      { mac: 'b', timestamp: 1000100000 },    // 100ms (plus rapide)
      { mac: 'c', timestamp: 0 },             // Pas buzzé
    ]

    const buzzed = buzzers.filter(b => (b.timestamp ?? 0) > 0)
    const notBuzzed = buzzers.filter(b => (b.timestamp ?? 0) === 0)

    buzzed.sort((a, b) => a.timestamp - b.timestamp)

    const result = [...buzzed, ...notBuzzed]

    expect(result[0].mac).toBe('b')  // 100ms
    expect(result[1].mac).toBe('a')  // 200ms
    expect(result[2].mac).toBe('c')  // 0ms (pas buzzé)
  })
})
