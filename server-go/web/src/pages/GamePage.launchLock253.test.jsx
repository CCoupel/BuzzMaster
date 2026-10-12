import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/react'
import GamePage from './GamePage'
import { useGame } from '../hooks/GameContext'

// #253 — page de jeu : commandes désactivées (avec motif) quand la popup « Lancer la partie » est ouverte
// sur un AUTRE poste ; bouton « Lancer la partie » retiré de la page.

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

vi.mock('../components/TeamCard', async () => {
  const actual = await vi.importActual('../components/TeamCard')
  return actual
})

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

vi.mock('../utils/colorUtils', async () => {
  const actual = await vi.importActual('../utils/colorUtils')
  return actual
})

vi.mock('./GamePage.css', () => ({}))


const TEAMS = { 'Les Bleus': { NAME: 'Les Bleus', SCORE: 0, COLOR: [0, 0, 255] } }
const BUMPERS = { b1: { TEAM: 'Les Bleus', NAME: 'VJ', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true, COLOR: [1, 2, 3] } }
const mkGame = (phase, lock) => ({
  gameState: {
    phase, question: { ID: '9', TYPE: 'NORMAL', QUESTION: 'Q ?', ANSWER: 'R', POINTS: '2', STATUS: phase },
    remote: 'GAME', timer: 0, totalTime: 30, MEMORY_PARTICIPATING_TEAMS: [], gameTime: 1000000,
    LAUNCH_LOCK: lock,
  },
  teams: TEAMS, bumpers: BUMPERS, questions: { '9': { ID: '9', STATUS: phase, ORDER: 1 } },
  connectionId: 'me',
  startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(), revealAnswer: vi.fn(),
  selectQuestion: vi.fn(), setRemoteDisplay: vi.fn(), setBumperPoints: vi.fn(), setTeamPoints: vi.fn(),
  forceReady: vi.fn(), simulateButton: vi.fn(), simulatePong: vi.fn(), sendMessage: vi.fn(),
  teamPointsBatch: vi.fn(() => true), awardedTeams: {},
})
const OFF = { ACTIVE: false, OWNER: '', SINCE: 0, EXPIRES_AT: 0 }
const OTHER = { ACTIVE: true, OWNER: 'other', SINCE: 1, EXPIRES_AT: 2 }
const btn = (name) => screen.getByRole('button', { name })

describe('GamePage — verrou « Lancer la partie » (#253)', () => {
  beforeEach(() => useGame.mockReset())

  it('plus de bouton « Lancer la partie » ni « Nouvelle partie » dans l\'en-tête', () => {
    useGame.mockReturnValue(mkGame('STOPPED', OFF))
    render(<GamePage />)
    expect(screen.queryByText('LANCER LA PARTIE')).toBeNull()
    expect(screen.queryByText('NOUVELLE PARTIE')).toBeNull()
  })

  it('sans verrou : REPONSE actif et le clic équipe crédite', () => {
    const g = mkGame('STOPPED', OFF); useGame.mockReturnValue(g)
    render(<GamePage />)
    expect(btn('REPONSE').disabled).toBe(false)
    fireEvent.click(screen.getByText('Les Bleus', { selector: 'h3' }))
    expect(g.setTeamPoints).toHaveBeenCalled()
  })

  it('verrou tenu par un autre poste : REPONSE désactivé avec le motif, clic équipe sans effet', () => {
    const g = mkGame('STOPPED', OTHER); useGame.mockReturnValue(g)
    render(<GamePage />)
    const reveal = btn('REPONSE')
    expect(reveal.disabled).toBe(true)
    expect(reveal.getAttribute('title')).toMatch(/Lancement d’une nouvelle partie en cours sur un autre poste/)
    fireEvent.click(screen.getByText('Les Bleus', { selector: 'h3' }))
    expect(g.setTeamPoints).not.toHaveBeenCalled()
  })

  it('verrou tenu par un autre poste : STOP / PAUSE désactivés avec le motif', () => {
    const g = mkGame('STARTED', OTHER); useGame.mockReturnValue(g)
    render(<GamePage />)
    expect(btn('STOP').disabled).toBe(true)
    expect(btn('STOP').getAttribute('title')).toMatch(/action suspendue/)
    expect(btn('PAUSE').disabled).toBe(true)
  })

  it('verrou à NOTRE nom (propriétaire) : la page n\'est pas désactivée par ce mécanisme (la popup couvre l\'écran)', () => {
    const g = mkGame('STOPPED', { ACTIVE: true, OWNER: 'me', SINCE: 1, EXPIRES_AT: 2 }); useGame.mockReturnValue(g)
    render(<GamePage />)
    expect(btn('REPONSE').disabled).toBe(false)
  })
})
