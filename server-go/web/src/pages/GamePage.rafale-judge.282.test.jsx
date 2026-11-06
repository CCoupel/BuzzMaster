/**
 * #282 (QUALIF 11.4.0.2) — bug « clic admin RÉPONSE VALIDE / INVALIDE sans effet » (cause racine :
 * _work/handoff/dev-backend-20261105-151500.md : `onValidate={rafaleValidate}` ⇒ l'événement React devient
 * `motionCardId` ⇒ JSON.stringify circulaire). GamePage (admin) est monté avec les VRAIS rafaleValidate /
 * rafaleInvalidate du hook useWebSocket (WebSocket mocké) ; le clic doit émettre RAFALE_VALIDATE /
 * RAFALE_INVALIDATE avec MSG sans MOTION_CARD_ID, sans exception. Scaffold = GamePage.rafale-alpha.282.test.jsx.
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'

vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))
vi.mock('../hooks/useCategories', () => ({
  useCategories: vi.fn(() => ({ categories: [], loading: false, error: null, refetch: vi.fn() })),
}))
vi.mock('../components/Timer', () => ({ default: () => <div data-testid="timer" /> }))
vi.mock('../components/QuestionPreview', () => ({ default: () => <div data-testid="question-preview" /> }))
vi.mock('../components/TeamCard', () => ({ default: ({ name }) => <div data-testid={`team-card-${name}`} />, OtaAllModal: () => null }))
vi.mock('../components/QuestionCard', () => ({
  default: ({ question }) => <div data-testid={`question-card-${question.ID}`} />,
  CATEGORIES: { HISTORY: { label: 'Histoire', icon: '📜', color: '#eab308' } },
}))
vi.mock('../components/Card', () => ({ default: ({ children, className }) => <div className={className}>{children}</div> }))
vi.mock('../components/Button', () => ({
  default: ({ children, onClick, disabled }) => <button onClick={onClick} disabled={disabled}>{children}</button>,
}))
vi.mock('../components/RafalePoolAlert', () => ({
  default: ({ alphabetical }) => <div data-testid="rafale-pool-alert-stub" data-alphabetical={String(!!alphabetical)} />,
}))
vi.mock('./GamePage.css', () => ({}))

import { useGame } from '../hooks/GameContext'
import GamePage from './GamePage'
import useWebSocket from '../hooks/useWebSocket'


// WebSocket mocké : le VRAI hook useWebSocket (rafaleValidate/rafaleInvalidate) envoie sur cette socket.
let sent = []
class MockWebSocket {
  constructor() { this.readyState = 0; MockWebSocket.last = this; MockWebSocket.all.push(this) }
  send(data) { sent.push(JSON.parse(data)) }
  close() { this.readyState = 3; this.onclose && this.onclose() }
}
MockWebSocket.all = []
MockWebSocket.CONNECTING = 0; MockWebSocket.OPEN = 1; MockWebSocket.CLOSING = 2; MockWebSocket.CLOSED = 3
const openSocket = async () => {
  await act(async () => { for (const w of MockWebSocket.all) { if (w.readyState === 3) continue; w.readyState = 1; w.onopen && w.onopen() } })
  sent = []
}
const frames = (action) => sent.filter((f) => f.ACTION === action)

const makeGameMock = ({ alphabetical = false, phase = 'STOPPED', gameExtra = {}, rafaleAnswer = null } = {}) => ({
  gameState: {
    phase,
    question: {
      ID: '1', TYPE: 'RAFALE', STATUS: phase === 'STARTED' ? 'STARTED' : 'STOPPED',
      RAFALE_CATEGORIES: ['HISTORY'], RAFALE_DIFFICULTIES: [2], RAFALE_MODE: 'SOLO', RAFALE_QUESTION_TIME: 3,
      ...(alphabetical ? { RAFALE_ALPHABETICAL: true } : {}),
    },
    remote: 'GAME', timer: 0, totalTime: 120, MEMORY_PARTICIPATING_TEAMS: [],
    RAFALE_SUBPHASE: phase === 'STARTED' ? 'QUESTION' : '',
    RAFALE_CURRENT_QUESTION: phase === 'STARTED' ? { ID: 'r-1', QUESTION: 'Fleuve de Paris ?', CATEGORY: 'HISTORY', DIFFICULTY: 1 } : { ID: '', QUESTION: '' },
    RAFALE_CURRENT_LETTER: alphabetical && phase === 'STARTED' ? 'S' : '',
    RAFALE_LETTERS: alphabetical && phase === 'STARTED' ? ['T', 'V'] : [],
    ...gameExtra,
  },
  teams: {}, bumpers: {},
  questions: { '1': { ID: '1', TYPE: 'RAFALE', STATUS: 'STOPPED', ORDER: 1, CATEGORY: 'HISTORY' } },
  startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(), revealAnswer: vi.fn(),
  selectQuestion: vi.fn(), setRemoteDisplay: vi.fn(), setBumperPoints: vi.fn(), setTeamPoints: vi.fn(),
  forceReady: vi.fn(), simulateButton: vi.fn(), simulatePong: vi.fn(), sendMessage: vi.fn(), rafaleSetTeams: vi.fn(),
  rafaleAnswer, rafaleValidate: vi.fn(), rafaleInvalidate: vi.fn(),
})


describe('GamePage (admin) — clic OK/KO RAFALE câblé sur le vrai hook (#282)', () => {
  beforeEach(() => {
    vi.clearAllMocks(); sent = []; MockWebSocket.all = []
    vi.stubGlobal('WebSocket', MockWebSocket)
    useGame.mockImplementation(() => {
      const ws = useWebSocket('/ws/admin')
      return { ...makeGameMock({ phase: 'STARTED' }), rafaleValidate: ws.rafaleValidate, rafaleInvalidate: ws.rafaleInvalidate }
    })
  })
  afterEach(() => { vi.unstubAllGlobals() })

  it('clic RÉPONSE VALIDE : RAFALE_VALIDATE avec MSG sans MOTION_CARD_ID, sans exception', async () => {
    render(<GamePage />)
    await openSocket()
    expect(() => fireEvent.click(screen.getByText(/RÉPONSE VALIDE/))).not.toThrow()
    const f = frames('RAFALE_VALIDATE')
    expect(f).toHaveLength(1)
    expect(f[0].MSG).toEqual({})
  })

  it('clic RÉPONSE INVALIDE : RAFALE_INVALIDATE avec MSG sans MOTION_CARD_ID, sans exception', async () => {
    render(<GamePage />)
    await openSocket()
    expect(() => fireEvent.click(screen.getByText(/RÉPONSE INVALIDE/))).not.toThrow()
    const f = frames('RAFALE_INVALIDATE')
    expect(f).toHaveLength(1)
    expect(f[0].MSG).toEqual({})
  })
})
