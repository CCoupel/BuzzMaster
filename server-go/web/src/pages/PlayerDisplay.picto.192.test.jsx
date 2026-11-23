import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render } from '@testing-library/react'
import PlayerDisplay from './PlayerDisplay'

// #192/#193 (v13.0.0, LOT-2.1) — PlayerDisplay : branchement du type PICTO (contracts/picto.md, maquettes M3 / M4).
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
  phase: 'READY', remote: null, timer: 90, totalTime: 90, virtualPlayerCount: 0, virtualPlayerLimit: 20,
  backgrounds: [], currentBackgroundIndex: 0, memoryMatchedPairs: [], neonEffect: { enabled: false }, entracte: false,
  entracteConfig: { TITLE: 'ENTRACTE', SUBTITLE: '', IMAGE_IS_CUSTOM: false, PANEL_SIZE: 65, ANIM_PERIOD: 10, ANIM_INTENSITY: 20 },
  question: { ID: 'pq', TYPE: 'PICTO', QUESTION: 'Picto — à la maison', ANSWER: '—', CATEGORY: 'ANIMALS', POINTS: 10, TIME: 90 },
  PICTO_TEAM: 'Bleus', PICTO_DRAWER: 'b1', PICTO_DRAWER_NAME: 'Alice', PICTO_DRAWER_CONNECTED: true, PICTO_CANVAS_EPOCH: 1,
}
const TEAMS = { Bleus: { NAME: 'Bleus', COLOR: [59, 130, 246] } }

function mockGame(overrides = {}) {
  useGame.mockReturnValue({
    gameState: { ...BASE, ...overrides }, teams: TEAMS, bumpers: {},
    flipMemoryCard: vi.fn(), showQRCode: vi.fn(), selectMotionCard: vi.fn(), sendMessage: vi.fn(),
  })
}

beforeEach(() => {
  global.fetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve([]) })
  HTMLCanvasElement.prototype.getContext = () => null
})
afterEach(() => { vi.clearAllMocks() })

describe('PlayerDisplay — type PICTO (#192/#193)', () => {
  it('READY (retour utilisateur GATE LOT-1) : type et catégorie, jamais « NOUVELLE QUESTION »', () => {
    mockGame()
    const { container } = render(<PlayerDisplay />)
    expect(container.textContent).toMatch(/PICTO/)
    expect(container.querySelector('.ready-category-display, .ready-state')).not.toBeNull()
    expect(container.textContent).not.toMatch(/NOUVELLE QUESTION/)
  })
  it('COUNTDOWN : chronomètre et catégorie, comme les autres types', () => {
    mockGame({ phase: 'COUNTDOWN', countdownTime: 3 })
    const { container } = render(<PlayerDisplay />)
    expect(container.querySelector('.countdown-number')).not.toBeNull()
  })
  it('STARTED TV : indice et feuille, aucun repli sur le bloc générique (ni énoncé ni réponse)', () => {
    mockGame({ phase: 'STARTED' })
    const { container } = render(<PlayerDisplay />)
    expect(container.querySelector('[data-testid="picto-tv"]')).not.toBeNull()
    expect(container.textContent).not.toMatch(/Picto — à la maison/)
  })
  it('tablette du dessinateur : « C\'est toi qui dessines ! » en READY, à la place des écrans génériques', () => {
    mockGame()
    const { container } = render(<PlayerDisplay isVPlayer playerId="b1" teamName="Bleus" />)
    expect(container.querySelector('[data-testid="picto-drawer-prep"]')).not.toBeNull()
    expect(container.querySelector('.ready-category-display')).toBeNull()
  })
  it('tablette d\'une autre équipe en READY : écran générique (type et catégorie)', () => {
    mockGame()
    const { container } = render(<PlayerDisplay isVPlayer playerId="b7" teamName="Rouges" />)
    expect(container.querySelector('[data-testid="picto-drawer-prep"]')).toBeNull()
    expect(container.textContent).toMatch(/PICTO/)
  })
})
