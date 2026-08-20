import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import AnimPage from './AnimPage'
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

// ---------------------------------------------------------------------------
// AnimPage — mode ENTRACTE (#119), tâches F7/T3 du plan
// _work/reports/plan-entracte-119-20260820-140825.md.
//
// Contrat : `anim` n'a AUCUN droit de contrôle sur l'entracte (ENTRACTE_SET
// est admin-only, contracts/websocket-actions.md) — cette page ne doit
// exposer aucun bouton, seulement un indicateur textuel non filtré.
//
// Mêmes mocks de câblage que AnimPage.test.jsx (#166) : AnimConductPanel /
// AnimAnswerZone / AnimArdoiseList ont leur propre couverture exhaustive
// ailleurs, ils sont réduits à des stubs ici.
// ---------------------------------------------------------------------------

vi.mock('nosleep.js', () => ({
  default: class NoSleep {
    constructor() { this.isEnabled = false }
    enable() { this.isEnabled = true; return Promise.resolve() }
    disable() { this.isEnabled = false }
  },
}))

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('../hooks/useCategories', () => ({
  useCategories: vi.fn(),
}))

vi.mock('../components/Timer', () => ({
  default: ({ currentTime, phase }) => <div data-testid="timer" data-phase={phase}>{currentTime}</div>,
}))

vi.mock('../components/AnimConductPanel', () => ({
  default: () => <div data-testid="conduct-panel" />,
}))

vi.mock('../components/AnimAnswerZone', () => ({
  default: () => <div data-testid="answer-zone" />,
}))

vi.mock('../components/AnimArdoiseList', () => ({
  default: () => <div data-testid="ardoise-list" />,
}))

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

beforeEach(() => {
  useGame.mockReturnValue(makeGameMock())
  useCategories.mockReturnValue({ categories: [] })
})

describe('AnimPage — filtre .entracte-dim (T3)', () => {
  it('aucun filtre quand entracte est false', () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { entracte: false } }))
    const { container } = render(<AnimPage />)
    expect(container.querySelectorAll('.entracte-dim')).toHaveLength(0)
  })

  // `.anim-page` est une grille CSS à placement explicite (F7 du plan) :
  // la classe est appliquée à CHAQUE zone séparément (context/conduct/
  // teams/regie), pas un seul wrapper — seule l'existence d'au moins un
  // nœud filtré est contractuelle, pas le compte exact.
  it('au moins un nœud filtré apparaît quand entracte est true', () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { entracte: true } }))
    const { container } = render(<AnimPage />)
    expect(container.querySelectorAll('.entracte-dim').length).toBeGreaterThan(0)
  })
})

describe('AnimPage — indicateur "entracte en cours", aucun contrôle (T3)', () => {
  it("n'affiche pas l'indicateur quand entracte est false", () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { entracte: false } }))
    render(<AnimPage />)
    expect(screen.queryByText(/Entracte en cours/i)).toBeNull()
  })

  it('affiche l\'indicateur "Entracte en cours — contrôle réservé à l\'admin" quand entracte est true, non filtré', () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { entracte: true } }))
    const { container } = render(<AnimPage />)
    const indicator = screen.getByText(/Entracte en cours.*contrôle réservé à l.admin/i)
    expect(indicator).toBeInTheDocument()
    const dimNodes = container.querySelectorAll('.entracte-dim')
    // L'indicateur ne doit être descendant d'AUCUN nœud filtré.
    dimNodes.forEach(dim => {
      expect(dim.contains(indicator)).toBe(false)
    })
  })

  it("n'expose AUCUN bouton de contrôle de l'entracte (admin uniquement, contracts/websocket-actions.md)", () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { entracte: true } }))
    render(<AnimPage />)
    const entracteButtons = screen.queryAllByRole('button', { name: /ENTRACTE/i })
    expect(entracteButtons).toHaveLength(0)
  })
})
