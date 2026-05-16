import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render } from '@testing-library/react'
import PlayerDisplay from './PlayerDisplay'

// ---------------------------------------------------------------------------
// Mocks — same pattern as PlayerDisplay.memotion.test.jsx
// ---------------------------------------------------------------------------

vi.mock('nosleep.js', () => ({
  default: class NoSleep {
    enable() { return Promise.resolve() }
    disable() {}
  },
}))

vi.mock('canvas-confetti', () => ({ default: vi.fn() }))

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('../components/Timer', () => ({
  default: ({ currentTime }) => <div data-testid="timer">{currentTime}</div>,
}))

vi.mock('../components/Podium', () => ({
  default: () => <div data-testid="podium" />,
}))

vi.mock('../components/QRCodeOverlay', () => ({
  default: () => null,
}))

vi.mock('../components/QRCodeDisplay', () => ({
  default: () => null,
}))

vi.mock('./QuestionsPage', () => ({
  CATEGORIES: [],
}))

vi.mock('../constants/colors', () => ({
  getCategoryColor: vi.fn(() => '#8b5cf6'),
}))

vi.mock('../utils/colorUtils', () => ({
  getRgbColor: vi.fn((color) => (Array.isArray(color) ? `rgb(${color.join(',')})` : color)),
}))

vi.mock('./PlayerDisplay.css', () => ({}))
vi.mock('../styles/neon.css', () => ({}))

// ---------------------------------------------------------------------------
// Import useGame après les mocks
// ---------------------------------------------------------------------------
import { useGame } from '../hooks/GameContext'

// ---------------------------------------------------------------------------
// Données de test
// ---------------------------------------------------------------------------

const CARDS_4 = [
  { ID: 'c1', RECTO_THEME: 'Thème A', DIFFICULTY: 1 },
  { ID: 'c2', RECTO_THEME: 'Thème B', DIFFICULTY: 2 },
  { ID: 'c3', RECTO_THEME: 'Thème C', DIFFICULTY: 1 },
  { ID: 'c4', RECTO_THEME: 'Thème D', DIFFICULTY: 3 },
]

/**
 * Builds a mock for a MEMOTION question.
 * @param {string} subphase     - 'MEMORIZE' | 'GRID' | 'SELECTED' | ...
 * @param {boolean} secretMode  - if true, sets MOTION_MEMORIZE_DURATION > 0
 * @param {object[]} cards      - motion cards array
 */
const makeSecretMock = (subphase = 'GRID', secretMode = false, cards = CARDS_4) => ({
  gameState: {
    phase: 'STARTED',
    remote: 'GAME',
    timer: 10,
    totalTime: 30,
    question: {
      TYPE: 'MEMOTION',
      MOTION_CARDS: cards,
      MOTION_CONFIG: {
        POINTS_1_STAR: 1,
        POINTS_2_STAR: 3,
        POINTS_3_STAR: 5,
      },
      MOTION_MEMORIZE_DURATION: secretMode ? 15 : 0,
    },
    MEMOTION_SUBPHASE: subphase,
    MEMOTION_CARD_STATES: {},
    MEMOTION_CARD_TEAMS: {},
    MEMOTION_CURRENT_TEAM: 'Équipe A',
    MEMOTION_CURRENT_TEAM_COLOR: [99, 102, 241],
    MEMOTION_SELECTED: null,
    MEMOTION_PARTICIPATING_TEAMS: ['Équipe A', 'Équipe B'],
    MEMORY_PARTICIPATING_TEAMS: [],
    newGameBackgrounds: [],
  },
  teams: {
    'Équipe A': { SCORE: 10, COLOR: [99, 102, 241] },
    'Équipe B': { SCORE: 5, COLOR: [234, 179, 8] },
  },
  bumpers: {},
  flipMemoryCard: vi.fn(),
  showQRCode: false,
  selectMotionCard: vi.fn(),
})

const renderTV = () => render(<PlayerDisplay />)

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('PlayerDisplay — MEMOTION Secret Mode (v5.5.0 #76)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.defineProperty(document.documentElement, 'requestFullscreen', {
      value: vi.fn().mockResolvedValue(undefined),
      writable: true,
      configurable: true,
    })
  })

  // -------------------------------------------------------------------------
  // T1 — MEMORIZE subphase : cartes visibles avec RECTO_THEME
  // -------------------------------------------------------------------------

  it('renders MEMORIZE subphase with all cards visible (RECTO_THEME)', () => {
    useGame.mockReturnValue(makeSecretMock('MEMORIZE', true))
    const { container } = renderTV()

    // Les titres des cartes doivent être affichés (RECTO_THEME)
    const titles = container.querySelectorAll('.memotion-card-title')
    expect(titles.length).toBe(4)
    expect(titles[0].textContent).toBe('Thème A')
    expect(titles[1].textContent).toBe('Thème B')

    // Pas de coordonnée affichée en MEMORIZE
    const coords = container.querySelectorAll('.memotion-card-coord')
    expect(coords.length).toBe(0)
  })

  // -------------------------------------------------------------------------
  // T2 — GRID + secretMode : cartes affichent coordonnées A1/B2
  // -------------------------------------------------------------------------

  it('renders GRID subphase in secret mode with coordinates (A1, A2, B1, B2)', () => {
    useGame.mockReturnValue(makeSecretMock('GRID', true))
    const { container } = renderTV()

    // Les coordonnées doivent être affichées (4 cartes → 2 cols → A1/A2/B1/B2)
    const coords = container.querySelectorAll('.memotion-card-coord')
    expect(coords.length).toBe(4)
    expect(coords[0].textContent).toBe('A1')
    expect(coords[1].textContent).toBe('A2')
    expect(coords[2].textContent).toBe('B1')
    expect(coords[3].textContent).toBe('B2')

    // Aucun titre de thème ne doit être affiché
    const titles = container.querySelectorAll('.memotion-card-title')
    expect(titles.length).toBe(0)
  })

  // -------------------------------------------------------------------------
  // T3 — GRID + mode standard : cartes affichent les thèmes RECTO
  // -------------------------------------------------------------------------

  it('renders GRID subphase in standard mode with RECTO themes (no coordinates)', () => {
    useGame.mockReturnValue(makeSecretMock('GRID', false))
    const { container } = renderTV()

    // Les titres des cartes doivent être affichés
    const titles = container.querySelectorAll('.memotion-card-title')
    expect(titles.length).toBe(4)
    expect(titles[0].textContent).toBe('Thème A')

    // Aucune coordonnée ne doit être affichée
    const coords = container.querySelectorAll('.memotion-card-coord')
    expect(coords.length).toBe(0)
  })

  // -------------------------------------------------------------------------
  // T4 — MEMORIZE subphase : sélection de carte désactivée
  // -------------------------------------------------------------------------

  it('card click is disabled during MEMORIZE subphase', () => {
    const mockGame = makeSecretMock('MEMORIZE', true)
    useGame.mockReturnValue(mockGame)
    const { container } = renderTV()

    // Les cartes doivent avoir cursor: default (pas pointer)
    const cards = container.querySelectorAll('.memotion-card')
    expect(cards.length).toBe(4)

    // canSelectCard = isAdminPreview && subphase === 'GRID' && state === 'UNPLAYED'
    // isAdminPreview = false (TV display), donc cursor: default dans tous les cas
    // Mais plus important : selectMotionCard ne doit pas être appelé
    cards.forEach(card => {
      card.click()
    })
    expect(mockGame.selectMotionCard).not.toHaveBeenCalled()
  })

  // -------------------------------------------------------------------------
  // T5 — MEMORIZE subphase : classe memotion-memorize-active présente
  // -------------------------------------------------------------------------

  it('adds memotion-memorize-active class to container during MEMORIZE', () => {
    useGame.mockReturnValue(makeSecretMock('MEMORIZE', true))
    const { container } = renderTV()

    const memotionGame = container.querySelector('.memotion-game')
    expect(memotionGame).not.toBeNull()
    expect(memotionGame.classList.contains('memotion-memorize-active')).toBe(true)
  })

  it('does NOT add memotion-memorize-active class during GRID', () => {
    useGame.mockReturnValue(makeSecretMock('GRID', true))
    const { container } = renderTV()

    const memotionGame = container.querySelector('.memotion-game')
    expect(memotionGame).not.toBeNull()
    expect(memotionGame.classList.contains('memotion-memorize-active')).toBe(false)
  })
})
