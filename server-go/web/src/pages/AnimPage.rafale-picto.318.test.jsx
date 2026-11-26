import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import AnimPage from './AnimPage'
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

// ---------------------------------------------------------------------------
// (gabarit AnimPage.picto.194) — AnimPage : crédit d'une manche PICTO (contrat picto.md §7, maquette M1) : « +N pts »
// pré-rempli aux points de la question pour l'équipe qui A TROUVÉ (issue FOUND) seulement, « 0 pt » pour les autres
// équipes ; pas de motif « pas de buzz » ; verrou d'attribution (#170) ; aucune ligne de crédit hors REVEALED.
// Scaffold : AnimPage.rafale.test.jsx (AnimTeamCard réel).
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

// v13.1.0 (#318) — AnimConductPanel RÉEL : les boutons « Trouvé » / « Passer » de la conduite Rafale Picto vivent dans le
// panneau de conduite (L3, AnimPicto), pas dans AnimPage ; l'ancien mock de câblage RAFALE les masquait.

vi.mock('../components/AnimAnswerZone', () => ({
  default: (props) => (
    <div data-testid="answer-zone" data-question-id={props.question?.ID ?? ''} data-revealed={String(!!props.revealed)} />
  ),
}))

vi.mock('../components/AnimArdoiseList', () => ({
  default: () => <div data-testid="ardoise-list" />,
}))

// ⚠️ `...overrides` spreadé EN PREMIER — voir le piège documenté dans
// GamePage.rafaleStartGate.test.jsx : un `overrides.gameState` partiel ne
// doit jamais écraser silencieusement la clé `gameState` fusionnée juste
// après.

function makeGameMock(overrides = {}) {
  return {
    ...overrides,
    status: 'connected',
    gameState: {
      phase: 'STARTED', timer: 100, totalTime: 120, gameTime: 0,
      question: { ID: 'rp1', TYPE: 'RAFALE_PICTO', CATEGORY: 'ANIMALS', POINTS: 10, TIME: 120, RAFALE_PICTO_PASS_COST: 5, RAFALE_PICTO_PASS_LIMIT: 1 },
      PICTO_TEAM: 'Bleue', PICTO_TEAM_COLOR: [0, 0, 255], PICTO_DRAWER: 'b1', PICTO_DRAWER_NAME: 'Alice', PICTO_DRAWER_CONNECTED: true,
      PICTO_CANDIDATES: [], PICTO_WORDS_COUNT: 5, PICTO_CANVAS_EPOCH: 7, PICTO_OUTCOME: '', PICTO_REVEALED_WORD: '', PICTO_REVEALED_CATEGORY: '',
      PICTO_FOUND_COUNT: 2, PICTO_FOUND_WORDS: ['Chat', 'Chien'], PICTO_PASS_COUNT: 0, PICTO_PASSES_LEFT: 1, PICTO_POOL_AVAILABLE: 2, PICTO_ROUND_POINTS: 20, PICTO_WORD_CATEGORY: 'ANIMALS',
      ...overrides.gameState,
    },
    teams: overrides.teams ?? { Bleue: { NAME: 'Bleue', COLOR: [0, 0, 255] }, Rouge: { NAME: 'Rouge', COLOR: [255, 0, 0] } },
    bumpers: overrides.bumpers ?? { b1: { TEAM: 'Bleue', NAME: 'Alice' }, r1: { TEAM: 'Rouge', NAME: 'Rita' } },
    nextQuestion: null, questionPosition: { position: 0, total: 0 },
    awardedTeams: overrides.awardedTeams ?? {},
    creditPoints: overrides.creditPoints ?? 10,
    startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(), revealAnswer: vi.fn(), selectQuestion: vi.fn(),
    setTeamPoints: overrides.setTeamPoints ?? vi.fn(), setBumperPoints: vi.fn(), flipMemoryCard: vi.fn(),
    regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: '' }, clearRegieMessage: vi.fn(),
    selectMotionCard: vi.fn(), flipMotionCard: vi.fn(), stopMotionTimer: vi.fn(), revealMotionCard: vi.fn(), doneMotionCard: vi.fn(),
    rafaleAnswer: null, rafaleValidate: vi.fn(), rafaleInvalidate: vi.fn(),
    pictoWord: { WORD: 'Maison', EPOCH: 7, CATEGORY: 'ANIMALS' }, pictoError: null, pictoCanvasStore: null,
    pictoFound: overrides.pictoFound ?? vi.fn(), pictoPass: overrides.pictoPass ?? vi.fn(), pictoSetTeam: vi.fn(), pictoSetDrawer: vi.fn(), pictoDrawDrawer: vi.fn(),
  }
}

beforeEach(() => { useCategories.mockReturnValue({ categories: [] }) })
afterEach(() => { vi.clearAllMocks() })

const btn = (name) => screen.queryByRole('button', { name })

describe('AnimPage — Rafale Picto : « Trouvé » et « Passer » par l’animateur (#318, picto.md §17.5, maquette M6)', () => {
  it('« Trouvé » émet PICTO_FOUND avec l’époque courante', () => {
    const g = makeGameMock(); useGame.mockReturnValue(g)
    render(<AnimPage />)
    fireEvent.click(btn(/^Trouvé/))
    expect(g.pictoFound).toHaveBeenCalledTimes(1)
    expect(g.pictoFound).toHaveBeenCalledWith(7)
  })

  it('« Passer » émet PICTO_PASS avec l’époque courante', () => {
    const g = makeGameMock(); useGame.mockReturnValue(g)
    render(<AnimPage />)
    fireEvent.click(btn(/Passer/))
    expect(g.pictoPass).toHaveBeenCalledTimes(1)
    expect(g.pictoPass).toHaveBeenCalledWith(7)
  })

  it('« Passer » est grisé quand il ne reste aucune passe (PICTO_PASSES_LEFT = 0)', () => {
    const g = makeGameMock({ gameState: { PICTO_PASSES_LEFT: 0, PICTO_PASS_COUNT: 1 } }); useGame.mockReturnValue(g)
    render(<AnimPage />)
    expect(btn(/Passer/).disabled).toBe(true)
    fireEvent.click(btn(/Passer/))
    expect(g.pictoPass).not.toHaveBeenCalled()
  })

  it('passes illimitées (PICTO_PASSES_LEFT = -1) : « Passer » reste actif', () => {
    const g = makeGameMock({ gameState: { PICTO_PASSES_LEFT: -1, PICTO_PASS_COUNT: 4 } }); useGame.mockReturnValue(g)
    render(<AnimPage />)
    expect(btn(/Passer/).disabled).toBe(false)
  })

  it('en PAUSE : « Trouvé » reste proposé, « Passer » ne l’est pas (PICTO_PASS : STARTED seulement)', () => {
    const g = makeGameMock({ gameState: { phase: 'PAUSED' } }); useGame.mockReturnValue(g)
    render(<AnimPage />)
    expect(btn(/^Trouvé/)).toBeTruthy()
    const pass = btn(/Passer/)
    expect(!pass || pass.disabled).toBe(true)
  })

  it('le fil des mots montre le mot en cours et les mots trouvés, jamais les mots à venir', () => {
    useGame.mockReturnValue(makeGameMock())
    const { container } = render(<AnimPage />)
    const text = container.textContent
    expect(text).toContain('Maison') // mot en cours
    expect(text).toContain('Chat')
    expect(text).toContain('Chien')
    expect(text).not.toMatch(/à venir|prochain/i)
    expect(text).toMatch(/Rafale Picto|2 mots? trouvé/)
  })

  it('le type Picto (un mot) ne propose aucun « Passer »', () => {
    const g = makeGameMock({ gameState: { question: { ID: 'pq1', TYPE: 'PICTO', CATEGORY: 'ANIMALS', POINTS: 10, TIME: 90 }, PICTO_PASSES_LEFT: -1, PICTO_FOUND_COUNT: 0, PICTO_FOUND_WORDS: [] } })
    useGame.mockReturnValue(g)
    render(<AnimPage />)
    expect(btn(/Passer/)).toBeNull()
  })
})
