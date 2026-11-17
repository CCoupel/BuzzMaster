import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render } from '@testing-library/react'
import PlayerDisplay from './PlayerDisplay'

// #144 (v12.0.0, LOT-1.2) — PlayerDisplay : branchement du type ROUE (contracts/roue.md §9, maquettes M1 / M5).
// Dispatch positif sur question.TYPE === 'ROUE', jamais de repli sur le bloc SPEEDY générique (qui afficherait
// l'énoncé et la réponse). Mocks identiques à PlayerDisplay.entracte.test.jsx.

vi.mock('nosleep.js', () => ({ default: class NoSleep { enable() { return Promise.resolve() } disable() {} } }))
vi.mock('canvas-confetti', () => ({ default: vi.fn() }))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))
vi.mock('../components/Timer', () => ({ default: ({ currentTime }) => <div data-testid="timer">{currentTime}</div> }))
vi.mock('../components/Podium', () => ({ default: () => <div data-testid="podium" /> }))
vi.mock('../components/QRCodeOverlay', () => ({ default: () => <div data-testid="qrcode-overlay" /> }))
vi.mock('../components/QRCodeDisplay', () => ({ default: () => <div data-testid="qr-display" /> }))
vi.mock('../components/EntractePanel', () => ({ default: () => <div data-testid="entracte-panel" /> }))
vi.mock('./QuestionsPage', () => ({ CATEGORIES: [] }))
vi.mock('../constants/colors', () => ({ getCategoryColor: vi.fn(() => '#8b5cf6') }))
vi.mock('../utils/colorUtils', () => ({ getRgbColor: vi.fn((c) => (Array.isArray(c) ? `rgb(${c.join(',')})` : c)) }))
vi.mock('./PlayerDisplay.css', () => ({}))
vi.mock('../styles/neon.css', () => ({}))
vi.mock('../styles/entracte.css', () => ({}))

import { useGame } from '../hooks/GameContext'

const BASE = {
  phase: 'STARTED', remote: null, timer: 0, totalTime: 0, virtualPlayerCount: 0, virtualPlayerLimit: 20,
  backgrounds: [], currentBackgroundIndex: 0, memoryMatchedPairs: [], neonEffect: { enabled: false }, entracte: false,
  entracteConfig: { TITLE: 'ENTRACTE', SUBTITLE: '', IMAGE_IS_CUSTOM: false, PANEL_SIZE: 65, ANIM_PERIOD: 10, ANIM_INTENSITY: 20 },
  question: { ID: 'rq', TYPE: 'ROUE', QUESTION: 'Roue de la Fortune', ANSWER: '', CATEGORY: 'ANIMALS', ROUE_PRESENTATION: 'Chanson de Line Renaud', ROUE_MODE: 'TANT_QUE_JE_GAGNE' },
  ROUE_SUBPHASE: 'TURN', ROUE_TEAMS: ['Rouges', 'Bleus'], ROUE_CURRENT_TEAM: 'Rouges', ROUE_CURRENT_TEAM_COLOR: [239, 68, 68],
  ROUE_TEAM_BANK: { Rouges: 0, Bleus: 0 }, ROUE_CONSONANTS_LEFT: true, ROUE_SHOW_LETTERS: true,
  ROUE_BOARD: { COLS: 14, ROWS: ['_____ __'] }, ROUE_LETTERS: [],
  ROUE_WHEEL: { LAYOUT: 'strip', SPIN_PERIOD_MS: 2500, SPIN_MAX_MS: 10000, FLIP_INTERVAL_MS: 600, CURRENT: -1, STOP: null,
    SEGMENTS: [{ ID: 's0', KIND: 'value', VALUE: 100 }, { ID: 's1', KIND: 'bankrupt', VALUE: 0 }] },
  ROUE_LAST_EVENT: { SEQ: 0, KIND: '', TEAM: '', LETTER: '', COUNT: 0, VALUE: 0, GAIN: 0 }, ROUE_WINNER: '', ROUE_AWARDS: {},
}
const TEAMS = { Rouges: { NAME: 'Rouges', COLOR: [239, 68, 68] }, Bleus: { NAME: 'Bleus', COLOR: [59, 130, 246] } }

const FULL = { ROUE_SUBPHASE: 'SPINNING', ROUE_WHEEL: { ...BASE.ROUE_WHEEL, SPIN_UP_MS: 10000, FULL_SPEED: true } }
const CLIMB = { ROUE_SUBPHASE: 'SPINNING', ROUE_WHEEL: { ...BASE.ROUE_WHEEL, SPIN_UP_MS: 10000, FULL_SPEED: false } }

function mockGame(overrides = {}) {
  useGame.mockReturnValue({
    gameState: { ...BASE, ...overrides }, teams: TEAMS, bumpers: {},
    flipMemoryCard: vi.fn(), showQRCode: vi.fn(), selectMotionCard: vi.fn(),
  })
}

beforeEach(() => {
  global.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({}) })
})
afterEach(() => { vi.clearAllMocks() })

describe('PlayerDisplay — type ROUE (#144)', () => {
  it('TV : monte l\'écran ROUE (équipes, plateau, roue) pendant la manche', () => {
    mockGame()
    const { container } = render(<PlayerDisplay />)
    expect(container.querySelector('[data-testid="roue-tv"]')).not.toBeNull()
    expect(container.querySelectorAll('.roue-bank')).toHaveLength(2)
    expect(container.querySelector('[data-testid="fortune-wheel"]')).not.toBeNull()
  })

  it('TV : jamais de repli sur le bloc générique — ni énoncé « Roue de la Fortune » ni champ réponse', () => {
    mockGame()
    const { container } = render(<PlayerDisplay />)
    expect(container.textContent).not.toMatch(/Roue de la Fortune/)
    expect(container.querySelector('.answer, .question-answer')).toBeNull()
  })

  it('TV : la phrase éventuellement présente dans le payload n\'est jamais affichée', () => {
    mockGame({ question: { ...BASE.question, ROUE_PHRASE: 'COMBIEN POUR CE CHIEN' } })
    const { container } = render(<PlayerDisplay />)
    expect(container.textContent).not.toMatch(/COMBIEN POUR CE CHIEN/)
  })

  it('tablette (isVPlayer) : monte la vue tablette, la roue tapable quand elle tourne pour son équipe, sans bouton STOP', () => {
    mockGame(FULL)
    const { container, getByRole } = render(<PlayerDisplay isVPlayer teamName="Rouges" onMediaClick={vi.fn()} />)
    expect(container.querySelector('[data-testid="roue-vp"]')).not.toBeNull()
    expect(getByRole('button', { name: /Arrêter la roue/ }).querySelector('[data-testid="fortune-wheel"]')).not.toBeNull()
    expect(container.querySelector('.roue-vp-stop')).toBeNull()
  })

  it('tablette : un tap sur la roue appelle onMediaClick une seule fois (envoi BUTTON)', () => {
    mockGame(FULL)
    const onMediaClick = vi.fn()
    const { getByRole } = render(<PlayerDisplay isVPlayer teamName="Rouges" onMediaClick={onMediaClick} />)
    getByRole('button', { name: /Arrêter la roue/ }).click()
    expect(onMediaClick).toHaveBeenCalledTimes(1)
  })

  it('tablette d\'une autre équipe : roue affichée, non tapable, aucun envoi', () => {
    mockGame(FULL)
    const onMediaClick = vi.fn()
    const { container, queryByRole } = render(<PlayerDisplay isVPlayer teamName="Bleus" onMediaClick={onMediaClick} />)
    expect(queryByRole('button', { name: /Arrêter la roue/ })).toBeNull()
    expect(container.querySelector('[data-testid="fortune-wheel"]')).not.toBeNull()
    container.querySelector('[data-testid="fortune-wheel"]').click()
    expect(onMediaClick).not.toHaveBeenCalled()
  })

  it('tablette pendant la montée (FULL_SPEED=false) : roue affichée, non tapable, aucun envoi', () => {
    mockGame(CLIMB)
    const onMediaClick = vi.fn()
    const { container, queryByRole } = render(<PlayerDisplay isVPlayer teamName="Rouges" onMediaClick={onMediaClick} />)
    expect(queryByRole('button', { name: /Arrêter la roue/ })).toBeNull()
    expect(container.querySelector('[data-testid="fortune-wheel"]')).not.toBeNull()
    container.querySelector('[data-testid="fortune-wheel"]').click()
    expect(onMediaClick).not.toHaveBeenCalled()
    expect(container.textContent).toMatch(/prend de la vitesse/i)
  })

  it('tablette : devient tapable quand l\'UPDATE apporte FULL_SPEED=true', () => {
    mockGame(CLIMB)
    const { queryByRole, rerender } = render(<PlayerDisplay isVPlayer teamName="Rouges" onMediaClick={vi.fn()} />)
    expect(queryByRole('button', { name: /Arrêter la roue/ })).toBeNull()
    mockGame(FULL)
    rerender(<PlayerDisplay isVPlayer teamName="Rouges" onMediaClick={vi.fn()} />)
    expect(queryByRole('button', { name: /Arrêter la roue/ })).not.toBeNull()
  })

  it('tablette hors SPINNING : roue affichée, non tapable, y compris SOLVED', () => {
    for (const sub of ['TURN', 'STOPPING', 'LETTER', 'SOLVED']) {
      mockGame({ ROUE_SUBPHASE: sub })
      const { container, queryByRole, unmount } = render(<PlayerDisplay isVPlayer teamName="Rouges" onMediaClick={vi.fn()} />)
      expect(queryByRole('button', { name: /Arrêter la roue/ }), sub).toBeNull()
      expect(container.querySelector('[data-testid="fortune-wheel"]'), sub).not.toBeNull()
      unmount()
    }
  })

  it('tablette : la présentation est affichée et la phrase jamais', () => {
    mockGame({ ...FULL, question: { ...BASE.question, ROUE_PHRASE: 'COMBIEN POUR CE CHIEN' } })
    const { container } = render(<PlayerDisplay isVPlayer teamName="Rouges" onMediaClick={vi.fn()} />)
    expect(container.textContent).toMatch(/Chanson de Line Renaud/)
    expect(container.textContent).not.toMatch(/COMBIEN POUR CE CHIEN/)
  })

  it('non-régression : une question SPEEDY ne monte pas l\'écran ROUE', () => {
    mockGame({ question: { ID: 's1', TYPE: 'SPEEDY', QUESTION: 'Capitale ?', ANSWER: 'Paris' } })
    const { container } = render(<PlayerDisplay />)
    expect(container.querySelector('[data-testid="roue-tv"]')).toBeNull()
  })
})
