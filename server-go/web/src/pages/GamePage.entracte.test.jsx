import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render } from '@testing-library/react'
import GamePage from './GamePage'

// ---------------------------------------------------------------------------
// GamePage — estompage pendant l'entracte (#119, T3, révisé par le delta C2
// du plan _work/reports/plan-entracte-119-fixes-20260820-155123.md).
//
// C2-F2 déplace le bouton ENTRACTE / FIN D'ENTRACTE vers la Navbar (couvert
// par Navbar.entracte.test.jsx) — GamePage ne porte donc plus AUCUNE
// assertion de bouton, seulement le filtre `.entracte-dim` qu'elle
// conserve (`entracteDim` reste appliqué sur `.game-page`, C2-F2 : « seul
// le bouton déménage »).
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
    entracte: false,
    ...overrides.gameState,
  },
  teams: overrides.teams ?? {},
  bumpers: overrides.bumpers ?? {},
  questions: overrides.questions ?? {
    '1': { ID: '1', STATUS: 'PLAYED', ORDER: 1 },
    '2': { ID: '2', STATUS: 'STOPPED', ORDER: 2 },
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
  setEntracte: vi.fn(),
  ...overrides,
})

beforeEach(() => {
  vi.clearAllMocks()
})

describe('GamePage — filtre .entracte-dim (T3, conservé après le déménagement du bouton vers la Navbar — C2)', () => {
  it('aucun filtre quand entracte est false', () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { phase: 'READY', entracte: false } }))
    const { container } = render(<GamePage />)
    expect(container.querySelectorAll('.entracte-dim')).toHaveLength(0)
  })

  it('au moins un nœud filtré apparaît quand entracte est true', () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { phase: 'STOPPED', entracte: true } }))
    const { container } = render(<GamePage />)
    expect(container.querySelectorAll('.entracte-dim').length).toBeGreaterThan(0)
  })

  it("n'expose plus aucun bouton ENTRACTE / FIN D'ENTRACTE (déménagé vers la Navbar, C2-F2 — non-régression)", () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { phase: 'STOPPED', entracte: true } }))
    const { queryByRole } = render(<GamePage />)
    expect(queryByRole('button', { name: /ENTRACTE/i })).toBeNull()
  })
})
