/**
 * Tests — GamePage : bouton NOUVELLE PARTIE accessible depuis /admin (#215,
 * critère d'acceptation AC9, milestone v9.0.0).
 *
 * #215 déplace le formulaire Quiz (et son propre déclencheur NOUVELLE
 * PARTIE) vers /admin/backstage (QuizMetaForm.jsx) — mais NOUVELLE PARTIE
 * est aussi une action de JEU (relance en pleine soirée), pas seulement de
 * préparation : l'animateur ne doit pas avoir à changer de page pour
 * l'atteindre. GamePage rend donc son propre déclencheur, directement
 * depuis `useGame().newGame` (sans passer par QuizMetaForm — voir le
 * commentaire de QuizMetaForm.jsx sur `onNewGame` restant optionnel).
 *
 * Suit le pattern de mocks de GamePage.test.jsx.
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import GamePage from './GamePage'

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
  questions: overrides.questions ?? {},
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
  newGame: vi.fn(),
  ...overrides,
})

describe('GamePage — bouton NOUVELLE PARTIE (#215 AC9)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('le bouton NOUVELLE PARTIE est présent sur /admin (GamePage), sans passer par Backstage', () => {
    useGame.mockReturnValue(makeGameMock())
    render(<GamePage />)
    expect(screen.getByText('NOUVELLE PARTIE')).toBeInTheDocument()
  })

  it('cliquer NOUVELLE PARTIE appelle useGame().newGame directement', () => {
    const newGame = vi.fn()
    useGame.mockReturnValue(makeGameMock({ newGame }))
    render(<GamePage />)

    fireEvent.click(screen.getByText('NOUVELLE PARTIE'))

    expect(newGame).toHaveBeenCalledTimes(1)
  })
})
