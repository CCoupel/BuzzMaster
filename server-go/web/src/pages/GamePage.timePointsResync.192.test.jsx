import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import GamePage from './GamePage'

// v13.0.0 (LOT-2.1) — « Temps » et « Points » de la page Jeu suivent la question courante, quel que soit son mode
// de sélection (anim, question suivante, rechargement) ; START envoie alors sa durée, pas 30 s par défaut.
// Mocks repris de GamePage.creditPoints.test.jsx.

vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))
vi.mock('../components/Timer', () => ({ default: () => <div data-testid="timer" /> }))
vi.mock('../components/QuestionPreview', () => ({ default: () => <div data-testid="question-preview" /> }))
vi.mock('../components/TeamCard', () => ({ default: () => <div />, OtaAllModal: () => null }))
vi.mock('../components/QuestionCard', () => ({ default: () => <div />, CATEGORIES: {} }))
vi.mock('../components/Card', () => ({ default: ({ children, className, ...rest }) => <div className={className} {...rest}>{children}</div> }))
vi.mock('../components/Button', () => ({ default: ({ children, onClick, disabled, ...rest }) => <button onClick={onClick} disabled={disabled} {...rest}>{children}</button> }))
vi.mock('./GamePage.css', () => ({}))

import { useGame } from '../hooks/GameContext'

const q = (ID, TIME, POINTS, TYPE = 'PICTO') => ({ ID, TYPE, TIME, POINTS })
const mk = (phase, question, extra = {}) => ({
  gameState: { phase, question, remote: 'GAME', timer: 0, totalTime: 30, MEMORY_PARTICIPATING_TEAMS: [], PICTO_TEAM: 'A', PICTO_DRAWER: 'b1', PICTO_DRAWER_CONNECTED: true, ...extra },
  teams: {}, bumpers: {}, questions: {},
  startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(), revealAnswer: vi.fn(),
  selectQuestion: vi.fn(), setRemoteDisplay: vi.fn(), setBumperPoints: vi.fn(), setTeamPoints: vi.fn(), forceReady: vi.fn(),
  simulateButton: vi.fn(), simulatePong: vi.fn(), sendMessage: vi.fn(),
})

beforeEach(() => { vi.useFakeTimers() })
afterEach(() => { vi.runOnlyPendingTimers(); vi.useRealTimers(); vi.clearAllMocks() })

const time = () => screen.getByLabelText('Temps (sec)').value
const points = () => screen.getByLabelText('Points').value

describe('GamePage — temps et points repris de la question courante', () => {
  it('rechargement de la page : une question PICTO déjà sélectionnée donne 90 s et 10 pts', () => {
    useGame.mockReturnValue(mk('READY', q('7', 90, 10)))
    render(<GamePage />)
    expect(time()).toBe('90')
    expect(points()).toBe('10')
  })

  it('sélection depuis l\'anim ou question suivante : le changement d\'ID reprend la nouvelle durée', () => {
    const first = mk('READY', q('7', 90, 10))
    useGame.mockReturnValue(first)
    const { rerender } = render(<GamePage />)
    expect(time()).toBe('90')
    useGame.mockReturnValue(mk('PREPARE', q('8', 120, 5)))
    rerender(<GamePage />)
    expect(time()).toBe('120')
    expect(points()).toBe('5')
  })

  it('START envoie la durée de la question (Delay), pas 30 s', () => {
    const g = mk('READY', q('7', 90, 10))
    useGame.mockReturnValue(g)
    render(<GamePage />)
    fireEvent.click(screen.getByText('START'))
    expect(g.startGame).toHaveBeenCalledWith(90, 10)
  })

  it('une saisie manuelle faite après la sélection de la même question n\'est pas écrasée', () => {
    useGame.mockReturnValue(mk('READY', q('7', 90, 10)))
    const { rerender } = render(<GamePage />)
    fireEvent.change(screen.getByLabelText('Temps (sec)'), { target: { value: '45' } })
    useGame.mockReturnValue(mk('READY', q('7', 90, 10), { PICTO_DRAWER_NAME: 'Alice' })) // nouvel UPDATE, même question
    rerender(<GamePage />)
    expect(time()).toBe('45')
  })

  it('pendant une manche ou après elle, la valeur est conservée ; la reprise a lieu à la préparation de la question suivante', () => {
    useGame.mockReturnValue(mk('READY', q('7', 90, 10)))
    const { rerender } = render(<GamePage />)
    useGame.mockReturnValue(mk('STARTED', q('9', 60, 3)))
    rerender(<GamePage />)
    expect(time()).toBe('90')
    useGame.mockReturnValue(mk('REVEALED', q('9', 60, 3)))
    rerender(<GamePage />)
    expect(time()).toBe('90')
    expect(points()).toBe('10')
    useGame.mockReturnValue(mk('PREPARE', q('9', 60, 3)))
    rerender(<GamePage />)
    expect(time()).toBe('60')
  })

  it('autre type (SPEEDY) : même comportement', () => {
    useGame.mockReturnValue(mk('READY', q('3', 45, 2, 'SPEEDY')))
    render(<GamePage />)
    expect(time()).toBe('45')
  })
})
