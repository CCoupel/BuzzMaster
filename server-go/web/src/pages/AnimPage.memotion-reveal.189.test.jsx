import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render } from '@testing-library/react'
import AnimPage from './AnimPage'
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

// #189 — pendant une carte MEMOTION en sous-phase REVEAL, la zone réponse
// (AnimAnswerZone) doit se comporter comme « révélée ». gameState.phase reste
// STARTED pendant toute la manche MEMOTION : la révélation est portée par
// MEMOTION_SUBPHASE === 'REVEAL' (utils/hostContext.js). On observe la valeur
// `revealed` réellement reçue par la zone réponse.

const zoneProps = vi.fn()

vi.mock('nosleep.js', () => ({
  default: class NoSleep { constructor() { this.isEnabled = false } enable() { this.isEnabled = true; return Promise.resolve() } disable() { this.isEnabled = false } },
}))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))
vi.mock('../hooks/useCategories', () => ({ useCategories: vi.fn() }))
vi.mock('../components/Timer', () => ({ default: () => <div data-testid="timer" /> }))
vi.mock('../components/AnimArdoiseList', () => ({ default: () => <div data-testid="ardoise-list" /> }))
vi.mock('../components/AnimAnswerZone', () => ({
  default: (props) => {
    zoneProps(props)
    return <div data-testid="answer-zone" />
  },
}))
vi.mock('./AnimPage.css', () => ({}))
vi.mock('../components/AnimConductPanel.css', () => ({}))
vi.mock('../components/LaunchGameHost.css', () => ({}))
vi.mock('../styles/entracte.css', () => ({}))

function makeGameMock(gameState) {
  return {
    status: 'connected',
    gameState: {
      phase: 'STARTED',
      timer: 30,
      totalTime: 30,
      gameTime: 0,
      entracte: false,
      entracteConfig: { TITLE: 'ENTRACTE', SUBTITLE: '', IMAGE_IS_CUSTOM: false, PANEL_SIZE: 65, ANIM_PERIOD: 10, ANIM_INTENSITY: 20 },
      ...gameState,
    },
    teams: {},
    bumpers: {},
    nextQuestion: null,
    questionPosition: { position: 0, total: 0 },
    awardedTeams: {},
    creditPoints: 0,
    startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(),
    revealAnswer: vi.fn(), selectQuestion: vi.fn(), setTeamPoints: vi.fn(), setBumperPoints: vi.fn(),
    flipMemoryCard: vi.fn(),
    regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: '' },
    clearRegieMessage: vi.fn(),
    selectMotionCard: vi.fn(), flipMotionCard: vi.fn(), stopMotionTimer: vi.fn(),
    revealMotionCard: vi.fn(), doneMotionCard: vi.fn(),
  }
}

const memotion = (subphase) => ({
  phase: 'STARTED',
  question: {
    ID: 'q1', TYPE: 'MEMOTION', STATUS: 'READY',
    MOTION_CARDS: [{ ID: 'c1', RECTO_THEME: 'Thème', ANSWER_TEXT: 'Réponse', DIFFICULTY: 1 }],
  },
  MEMOTION_SUBPHASE: subphase,
  MEMOTION_SELECTED: 'c1',
  MEMOTION_CARD_STATES: { c1: 'FLIPPED' },
  MEMOTION_CARD_TEAMS: {},
  MEMOTION_CURRENT_TEAM: '',
  MEMOTION_PARTICIPATING_TEAMS: [],
})

const lastRevealed = () => {
  const calls = zoneProps.mock.calls
  expect(calls.length).toBeGreaterThan(0)
  return calls[calls.length - 1][0].revealed
}

beforeEach(() => {
  zoneProps.mockClear()
  useCategories.mockReturnValue({ categories: [] })
})

describe('AnimPage — zone réponse et révélation MEMOTION (#189)', () => {
  it('carte MEMOTION en REVEAL (phase STARTED) : la zone réponse est « révélée »', () => {
    useGame.mockReturnValue(makeGameMock(memotion('REVEAL')))
    render(<AnimPage />)
    expect(lastRevealed()).toBe(true)
  })

  it('carte MEMOTION en QUESTION : la zone réponse n\'est pas révélée', () => {
    useGame.mockReturnValue(makeGameMock(memotion('QUESTION')))
    render(<AnimPage />)
    expect(lastRevealed()).toBe(false)
  })

  it('non-régression hors MEMOTION : question classique REVEALED -> révélée, STARTED -> non', () => {
    const q = { ID: '1', TYPE: 'NORMAL', STATUS: 'READY', ANSWER: 'x' }
    useGame.mockReturnValue(makeGameMock({ phase: 'REVEALED', question: q }))
    const { unmount } = render(<AnimPage />)
    expect(lastRevealed()).toBe(true)
    unmount()
    zoneProps.mockClear()
    useGame.mockReturnValue(makeGameMock({ phase: 'STARTED', question: q }))
    render(<AnimPage />)
    expect(lastRevealed()).toBe(false)
  })
})
