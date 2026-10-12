import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import AnimPage from './AnimPage'
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

// #255 (v11.2) — /anim : la réponse QCM d'une équipe (couleur choisie, joueur, justesse) vient de ANSWERED_COLOR (réponse
// effective), jamais de la couleur ASSIGNÉE (ANSWER_COLOR) ; repli sans la clé : TIME > 0 && ANSWER_COLOR. Scénario 13.

vi.mock('nosleep.js', () => ({
  default: class NoSleep { constructor() { this.isEnabled = false } enable() { this.isEnabled = true; return Promise.resolve() } disable() { this.isEnabled = false } },
}))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))
vi.mock('../hooks/useCategories', () => ({ useCategories: vi.fn() }))
vi.mock('../components/Timer', () => ({ default: () => <div data-testid="timer" /> }))
vi.mock('../components/AnimConductPanel', () => ({ default: () => <div data-testid="conduct" /> }))
vi.mock('../components/AnimAnswerZone', () => ({ default: () => <div data-testid="answer-zone" /> }))
vi.mock('../components/AnimArdoiseList', () => ({ default: () => <div data-testid="ardoise-list" /> }))
vi.mock('./AnimPage.css', () => ({}))
vi.mock('../styles/entracte.css', () => ({}))

function makeGameMock(overrides = {}) {
  return {
    status: 'connected',
    gameState: {
      phase: 'STOPPED',
      timer: 30,
      totalTime: 30,
      gameTime: 0,
      question: null,
      entracte: false,
      entracteConfig: { TITLE: 'ENTRACTE', SUBTITLE: '', IMAGE_IS_CUSTOM: false, PANEL_SIZE: 65, ANIM_PERIOD: 10, ANIM_INTENSITY: 20 },
      ...overrides.gameState,
    },
    teams: {},
    bumpers: {},
    nextQuestion: null,
    questionPosition: { position: 0, total: 0 },
    awardedTeams: {},
    creditPoints: 0,
    startGame: vi.fn(),
    stopGame: vi.fn(),
    pauseGame: vi.fn(),
    continueGame: vi.fn(),
    revealAnswer: vi.fn(),
    selectQuestion: vi.fn(),
    setTeamPoints: vi.fn(),
    setBumperPoints: vi.fn(),
    flipMemoryCard: vi.fn(),
    regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: '' },
    clearRegieMessage: vi.fn(),
    selectMotionCard: vi.fn(),
    flipMotionCard: vi.fn(),
    stopMotionTimer: vi.fn(),
    revealMotionCard: vi.fn(),
    doneMotionCard: vi.fn(),
    ...overrides,
  }
}


const game = (bumpers, phase = 'REVEALED') => makeGameMock({
  gameState: { phase, question: { ID: '1', TYPE: 'QCM', QCM_CORRECT: 'RED' } },
  teams: { 'Les Rouges': { COLOR: [255, 0, 0], SCORE: 0 } },
  bumpers,
})
beforeEach(() => { useCategories.mockReturnValue({ categories: [] }) })

describe('AnimPage — réponse QCM selon ANSWERED_COLOR (#255, scénario 13)', () => {
  it('Critical — couleur assignée mais ANSWERED_COLOR vide : aucune réponse affichée (même avec un TIME > 0 résiduel)', () => {
    useGame.mockReturnValue(game({ r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'RED', ANSWERED_COLOR: '', NAME: 'Alice' } }))
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-team-qcm-answer')).toBeNull()
    expect(screen.queryByText('Alice')).toBeNull()
  })

  it('la couleur et la justesse suivent ANSWERED_COLOR (assignée RED, répondu BLUE → ✗)', () => {
    useGame.mockReturnValue(game({ r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'RED', ANSWERED_COLOR: 'BLUE', NAME: 'Alice' } }))
    const { container } = render(<AnimPage />)
    expect(screen.getByText('Alice')).toBeInTheDocument()
    expect(container.querySelector('.anim-team-qcm-correct').textContent).toBe('✗')
  })

  it('réponse correcte effective : ✓', () => {
    useGame.mockReturnValue(game({ r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'GREEN', ANSWERED_COLOR: 'RED', NAME: 'Alice' } }))
    const { container } = render(<AnimPage />)
    expect(container.querySelector('.anim-team-qcm-correct').textContent).toBe('✓')
  })

  it('repli sans la clé : TIME > 0 && ANSWER_COLOR (comportement antérieur)', () => {
    useGame.mockReturnValue(game({ r1: { TEAM: 'Les Rouges', TIME: 1000, ANSWER_COLOR: 'RED', NAME: 'Alice' } }))
    const { container } = render(<AnimPage />)
    expect(screen.getByText('Alice')).toBeInTheDocument()
    expect(container.querySelector('.anim-team-qcm-correct').textContent).toBe('✓')
    useGame.mockReturnValue(game({ r1: { TEAM: 'Les Rouges', TIME: 0, ANSWER_COLOR: 'RED', NAME: 'Alice' } }))
    const { container: c2 } = render(<AnimPage />)
    expect(c2.querySelector('.anim-team-qcm-answer')).toBeNull()
  })
})
