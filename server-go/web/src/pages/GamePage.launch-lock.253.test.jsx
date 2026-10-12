import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import GamePage from './GamePage'
import { useGame } from '../hooks/GameContext'

// #253 (v11.2, lot V2-4) — page de jeu admin : plus de bouton « LANCER LA PARTIE » (une seule entrée, dans le menu
// Préparation) ; quand la popup est ouverte sur un AUTRE poste, les commandes de jeu sont grisées avec le motif.

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


const TEAMS = { 'Les Bleus': { NAME: 'Les Bleus', SCORE: 0, COLOR: [10, 50, 90] } }
const idle = { ACTIVE: false, OWNER: '', SINCE: 0, EXPIRES_AT: 0 }
const mk = (lock, phase = 'REVEALED') => ({
  gameState: { phase, question: { ID: '2', STATUS: 'REVEALED' }, remote: 'GAME', timer: 0, totalTime: 30, MEMORY_PARTICIPATING_TEAMS: [], LAUNCH_LOCK: lock },
  connectionId: 'me', teams: TEAMS, bumpers: {}, questions: { 2: { ID: '2', STATUS: 'REVEALED', ORDER: 1 } },
  startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(), revealAnswer: vi.fn(), selectQuestion: vi.fn(),
  setRemoteDisplay: vi.fn(), setBumperPoints: vi.fn(), setTeamPoints: vi.fn(), forceReady: vi.fn(), simulateButton: vi.fn(), simulatePong: vi.fn(),
  sendMessage: vi.fn(), newGame: vi.fn(), newGameRefused: null, clearNewGameRefused: vi.fn(),
})

describe('GamePage — « Lancer la partie » (#253)', () => {
  beforeEach(() => { useGame.mockReset() })

  it('Critical — la page de jeu n’a plus de bouton « LANCER LA PARTIE » ni de popup', () => {
    useGame.mockReturnValue(mk(idle))
    render(<GamePage />)
    expect(screen.queryByText(/LANCER LA PARTIE/i)).toBeNull()
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('Critical — popup ouverte sur un autre poste : START / STOP désactivés avec le motif', () => {
    useGame.mockReturnValue(mk({ ACTIVE: true, OWNER: 'other', SINCE: 1, EXPIRES_AT: 46000 }, 'STARTED'))
    render(<GamePage />)
    const stop = screen.getAllByRole('button').filter((b) => /^\s*(⏹|■)?\s*STOP/i.test(b.textContent) || /^STOP/i.test(b.textContent))
    expect(stop.length).toBeGreaterThan(0)
    stop.forEach((b) => expect(b.disabled).toBe(true))
    expect(stop.some((b) => /Lancement d.une nouvelle partie en cours sur un autre poste/.test(b.title))).toBe(true)
  })

  it('verrou à notre nom ou au repos : les commandes ne sont pas désactivées par le verrou', () => {
    useGame.mockReturnValue(mk({ ACTIVE: true, OWNER: 'me', SINCE: 1, EXPIRES_AT: 46000 }, 'STARTED'))
    render(<GamePage />)
    const stop = screen.getAllByRole('button').filter((b) => /^STOP/i.test(b.textContent.trim()))
    expect(stop.length).toBeGreaterThan(0)
    expect(stop.some((b) => !b.disabled)).toBe(true)
  })
})
