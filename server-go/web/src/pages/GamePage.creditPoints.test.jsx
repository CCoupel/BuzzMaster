import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import GamePage from './GamePage'

// ---------------------------------------------------------------------------
// GamePage — émission SET_CREDIT_POINTS sur ajustement de pointsInput
// (MAJEUR-1, revue de code #155/#156)
//
// Sans ce câblage, /anim créditait question.POINTS brut pendant que /admin
// créditait pointsInput potentiellement ajusté (ex. manche bonus) : deux
// montants différents pour la même question. SET_CREDIT_POINTS pousse
// l'ajustement au serveur, qui le rediffuse à /anim via CREDIT_POINTS
// (contrats/websocket-actions.md §SET_CREDIT_POINTS).
//
// Mocks repris de GamePage.test.jsx (même structure minimale de rendu).
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
    <div data-testid={`question-card-${question.ID}`} onClick={() => onClick && onClick(question)} />
  ),
  CATEGORIES: {},
}))

vi.mock('../components/Card', () => ({
  default: ({ children, className, ...rest }) => <div className={className} {...rest}>{children}</div>,
}))

vi.mock('../components/Button', () => ({
  default: ({ children, onClick, disabled, ...rest }) => (
    <button onClick={onClick} disabled={disabled} {...rest}>{children}</button>
  ),
}))

vi.mock('./GamePage.css', () => ({}))

import { useGame } from '../hooks/GameContext'

const makeGameMock = (overrides = {}) => ({
  gameState: {
    phase: 'STOPPED',
    question: { ID: '2', STATUS: 'STOPPED', TYPE: 'SPEEDY' },
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
  ...overrides,
})

beforeEach(() => {
  vi.useFakeTimers()
})

afterEach(() => {
  vi.runOnlyPendingTimers()
  vi.useRealTimers()
  vi.clearAllMocks()
})

describe('GamePage — SET_CREDIT_POINTS (MAJEUR-1)', () => {
  it('n\'émet rien au montage (rien n\'a encore été ajusté)', () => {
    const props = makeGameMock()
    useGame.mockReturnValue(props)
    render(<GamePage />)

    vi.advanceTimersByTime(1000)
    expect(props.sendMessage).not.toHaveBeenCalledWith('SET_CREDIT_POINTS', expect.anything())
  })

  it('émet SET_CREDIT_POINTS après un ajustement de pointsInput, debounced', () => {
    const props = makeGameMock()
    useGame.mockReturnValue(props)
    render(<GamePage />)

    const input = screen.getByLabelText('Points')
    fireEvent.change(input, { target: { value: '20' } })

    // Pas encore envoyé avant l'écoulement du debounce
    expect(props.sendMessage).not.toHaveBeenCalledWith('SET_CREDIT_POINTS', expect.anything())

    vi.advanceTimersByTime(400)
    expect(props.sendMessage).toHaveBeenCalledWith('SET_CREDIT_POINTS', { POINTS: 20 })
  })

  it('coalesce les ajustements rapprochés en un seul message (debounce)', () => {
    const props = makeGameMock()
    useGame.mockReturnValue(props)
    render(<GamePage />)

    const input = screen.getByLabelText('Points')
    fireEvent.change(input, { target: { value: '5' } })
    vi.advanceTimersByTime(100)
    fireEvent.change(input, { target: { value: '15' } })
    vi.advanceTimersByTime(100)
    fireEvent.change(input, { target: { value: '25' } })
    vi.advanceTimersByTime(400)

    const setCreditCalls = props.sendMessage.mock.calls.filter(([action]) => action === 'SET_CREDIT_POINTS')
    expect(setCreditCalls).toHaveLength(1)
    expect(setCreditCalls[0][1]).toEqual({ POINTS: 25 })
  })
})
