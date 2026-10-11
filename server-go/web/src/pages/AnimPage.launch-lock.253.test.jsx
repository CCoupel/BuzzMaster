import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import AnimPage from './AnimPage'
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

// #253 (v11.2, lot V2-4) — vue de l'anim quand « Lancer la partie » est ouverte sur le poste régie (maquette VALIDÉE
// admin_new_game__launch_overlay, état « Vu depuis l'anim ») : bandeau de verrou, conduite grisée « suspendu ».
// Aucun contrôle de lancement sur l'anim. Le panneau de conduite est le vrai composant.

vi.mock('nosleep.js', () => ({
  default: class NoSleep { constructor() { this.isEnabled = false } enable() { this.isEnabled = true; return Promise.resolve() } disable() { this.isEnabled = false } },
}))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))
vi.mock('../hooks/useCategories', () => ({ useCategories: vi.fn() }))
vi.mock('../components/Timer', () => ({ default: () => <div data-testid="timer" /> }))
vi.mock('../components/AnimAnswerZone', () => ({ default: () => <div data-testid="answer-zone" /> }))
vi.mock('../components/AnimArdoiseList', () => ({ default: () => <div data-testid="ardoise-list" /> }))
vi.mock('./AnimPage.css', () => ({}))
vi.mock('../components/AnimConductPanel.css', () => ({}))
vi.mock('../components/LaunchGameHost.css', () => ({}))
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


const idle = { ACTIVE: false, OWNER: '', SINCE: 0, EXPIRES_AT: 0 }
const held = { ACTIVE: true, OWNER: 'regie-conn', SINCE: 1, EXPIRES_AT: 46000 }
const BANNER = /Lancement d.une nouvelle partie en cours sur le poste régie : conduite suspendue\./

beforeEach(() => {
  useCategories.mockReturnValue({ categories: [] })
  useGame.mockReturnValue(makeGameMock({ connectionId: 'anim-conn', gameState: { LAUNCH_LOCK: idle, phase: 'STOPPED' } }))
})

describe('AnimPage — verrou « Lancer la partie » (#253)', () => {
  it('Critical — popup ouverte sur la régie : bandeau « conduite suspendue » (role status) et conduite grisée', () => {
    useGame.mockReturnValue(makeGameMock({ connectionId: 'anim-conn', gameState: { LAUNCH_LOCK: held, phase: 'STOPPED', question: { ID: '1', TYPE: 'NORMAL', STATUS: 'READY' } } }))
    const { container } = render(<AnimPage />)
    const banner = screen.getByText(BANNER)
    expect(banner.closest('[role="status"]')).toBeTruthy()
    const panel = container.querySelector('.anim-conduct') || container
    const buttons = within(panel).queryAllByRole('button')
    expect(buttons.length).toBeGreaterThan(0)
    const conduct = buttons.filter((b) => /START|STOP|PAUSE|RÉPONSE|REVEAL|suspendu/i.test(b.textContent))
    expect(conduct.length).toBeGreaterThan(0)
    conduct.forEach((b) => expect(b.disabled || b.getAttribute('aria-disabled') === 'true').toBe(true))
    expect(panel.textContent).toMatch(/suspendu/)
  })

  it('au repos : aucun bandeau et aucune mention « suspendu »', () => {
    const { container } = render(<AnimPage />)
    expect(screen.queryByText(BANNER)).toBeNull()
    expect(container.textContent).not.toMatch(/suspendu/)
  })

  it('verrou levé : le bandeau disparaît', () => {
    useGame.mockReturnValue(makeGameMock({ connectionId: 'anim-conn', gameState: { LAUNCH_LOCK: held } }))
    const { rerender } = render(<AnimPage />)
    expect(screen.getByText(BANNER)).toBeTruthy()
    useGame.mockReturnValue(makeGameMock({ connectionId: 'anim-conn', gameState: { LAUNCH_LOCK: idle } }))
    rerender(<AnimPage />)
    expect(screen.queryByText(BANNER)).toBeNull()
  })

  it('l’anim n’a aucun bouton « Lancer la partie »', () => {
    useGame.mockReturnValue(makeGameMock({ connectionId: 'anim-conn', gameState: { LAUNCH_LOCK: held } }))
    render(<AnimPage />)
    expect(screen.queryByRole('button', { name: /Lancer la partie/i })).toBeNull()
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})
