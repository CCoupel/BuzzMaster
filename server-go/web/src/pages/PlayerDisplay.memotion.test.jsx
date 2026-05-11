import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render } from '@testing-library/react'
import PlayerDisplay from './PlayerDisplay'

// ---------------------------------------------------------------------------
// Mocks — PlayerDisplay imports
// framer-motion est auto-mocké via l'alias vite.config.js (src/mocks/framer-motion.jsx)
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
// Données de test MEMOTION
// ---------------------------------------------------------------------------

/** Carte avec image recto, question, réponse (Carte A) */
const CARD_WITH_IMG = {
  ID: 'card-1',
  RECTO_THEME: 'Thème Star Wars',
  RECTO_IMAGE: '/files/card1.jpg',
  DIFFICULTY: 2,
  QUESTION_TEXT: 'Dans quel épisode Dark Vador révèle-t-il sa filiation ?',
  QUESTION_IMAGE: '/files/q1.jpg',
  ANSWER_TEXT: "L'Empire contre-attaque (Épisode V)",
  ANSWER_IMAGE: '/files/a1.jpg',
}

/** Carte sans image recto (Carte B) */
const CARD_NO_IMG = {
  ID: 'card-2',
  RECTO_THEME: 'Thème Littérature',
  DIFFICULTY: 1,
  QUESTION_TEXT: "Qui a écrit Les Misérables ?",
  ANSWER_TEXT: 'Victor Hugo',
}

/**
 * Construit un mock useGame pour une question MEMOTION.
 * @param {string|null} subphase   - 'GRID' | 'SELECTED' | 'QUESTION' | 'REVEAL' | null
 * @param {string|null} selectedId - ID de la carte sélectionnée
 * @param {object[]}    cards      - tableau de MotionCards à utiliser
 */
const makeMemotionMock = (subphase = 'GRID', selectedId = null, cards = [CARD_WITH_IMG, CARD_NO_IMG]) => ({
  gameState: {
    phase: 'STARTED',
    remote: 'GAME',
    timer: 15,
    totalTime: 30,
    question: {
      TYPE: 'MEMOTION',
      MOTION_CARDS: cards,
      MOTION_CONFIG: {
        POINTS_1_STAR: 1,
        POINTS_2_STAR: 3,
        POINTS_3_STAR: 5,
      },
    },
    MEMOTION_SUBPHASE: subphase,
    MEMOTION_CARD_STATES: {},
    MEMOTION_CARD_TEAMS: {},
    MEMOTION_CURRENT_TEAM: 'Équipe A',
    MEMOTION_CURRENT_TEAM_COLOR: [99, 102, 241],
    MEMOTION_SELECTED: selectedId,
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

// ---------------------------------------------------------------------------
// Helper : render PlayerDisplay avec les props TV minimales
// ---------------------------------------------------------------------------
const renderTV = () => render(<PlayerDisplay />)

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('PlayerDisplay — MEMOTION layout (bugfix SC1–SC6)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // Évite l'erreur "Not implemented: requestFullscreen" dans jsdom
    Object.defineProperty(document.documentElement, 'requestFullscreen', {
      value: vi.fn().mockResolvedValue(undefined),
      writable: true,
      configurable: true,
    })
  })

  // -------------------------------------------------------------------------
  // SC1 — Grille MEMOTION : titre et étoiles portent les bonnes classes CSS
  // -------------------------------------------------------------------------

  describe('SC1 — Grille MEMOTION (GRID subphase)', () => {
    it('affiche le RECTO_THEME dans memotion-card-title pour chaque carte', () => {
      useGame.mockReturnValue(makeMemotionMock('GRID'))
      const { container } = renderTV()

      const titles = container.querySelectorAll('.memotion-card-title')
      expect(titles.length).toBe(2)
      expect(titles[0].textContent).toBe('Thème Star Wars')
      expect(titles[1].textContent).toBe('Thème Littérature')
    })

    it('affiche les étoiles dans memotion-card-stars selon la DIFFICULTY', () => {
      useGame.mockReturnValue(makeMemotionMock('GRID'))
      const { container } = renderTV()

      const stars = container.querySelectorAll('.memotion-card-stars')
      expect(stars.length).toBe(2)
      expect(stars[0].textContent).toBe('★★')  // DIFFICULTY 2
      expect(stars[1].textContent).toBe('★')   // DIFFICULTY 1
    })

    it('la carte possède les zones header, body, footer', () => {
      useGame.mockReturnValue(makeMemotionMock('GRID'))
      const { container } = renderTV()

      const cardBacks = container.querySelectorAll('.memotion-card .memory-card-back')
      expect(cardBacks.length).toBeGreaterThan(0)
      cardBacks.forEach(card => {
        expect(card.querySelector('.memotion-card-header')).not.toBeNull()
        expect(card.querySelector('.memotion-card-body')).not.toBeNull()
        expect(card.querySelector('.memotion-card-footer')).not.toBeNull()
      })
    })

    it('affiche une image dans memotion-card-body quand RECTO_IMAGE est défini', () => {
      useGame.mockReturnValue(makeMemotionMock('GRID'))
      const { container } = renderTV()

      const cardBacks = container.querySelectorAll('.memotion-card .memory-card-back')
      const img = cardBacks[0].querySelector('img.memotion-card-img')
      expect(img).not.toBeNull()
      expect(img.getAttribute('src')).toBe('/files/card1.jpg')
    })

    it("n'affiche pas d'image dans memotion-card-body quand RECTO_IMAGE est absent", () => {
      useGame.mockReturnValue(makeMemotionMock('GRID'))
      const { container } = renderTV()

      const cardBacks = container.querySelectorAll('.memotion-card .memory-card-back')
      expect(cardBacks[1].querySelector('img.memotion-card-img')).toBeNull()
    })
  })

  // -------------------------------------------------------------------------
  // SC2 — SELECTED avec RECTO_IMAGE : image dans body, pas de texte dans body
  //
  // Structure JSX réelle (SELECTED) :
  //   .memotion-tv-fullscreen
  //     .memotion-tv-fs-header  ← thème + étoiles + équipe
  //     .memotion-tv-fs-body    ← image OU texte (ternaire)
  // -------------------------------------------------------------------------

  describe('SC2 — SELECTED avec RECTO_IMAGE', () => {
    beforeEach(() => {
      useGame.mockReturnValue(makeMemotionMock('SELECTED', 'card-1'))
    })

    it('affiche le container fullscreen (.memotion-tv-fullscreen)', () => {
      const { container } = renderTV()
      expect(container.querySelector('.memotion-tv-fullscreen')).not.toBeNull()
    })

    it('le header fullscreen contient le RECTO_THEME', () => {
      const { container } = renderTV()
      const header = container.querySelector('.memotion-tv-fs-header')
      expect(header).not.toBeNull()
      expect(header.textContent).toContain('Thème Star Wars')
    })

    it('le header fullscreen contient les étoiles de difficulté', () => {
      const { container } = renderTV()
      const diff = container.querySelector('.memotion-tv-fs-diff')
      expect(diff).not.toBeNull()
      expect(diff.textContent).toBe('★★')
    })

    it('le body fullscreen contient l\'image RECTO_IMAGE', () => {
      const { container } = renderTV()
      const body = container.querySelector('.memotion-tv-fs-body')
      expect(body).not.toBeNull()
      const img = body.querySelector('img.memotion-tv-fs-img')
      expect(img).not.toBeNull()
      expect(img.getAttribute('src')).toBe('/files/card1.jpg')
    })

    it('le body fullscreen ne contient PAS de texte quand RECTO_IMAGE est présente', () => {
      const { container } = renderTV()
      const body = container.querySelector('.memotion-tv-fs-body')
      expect(body).not.toBeNull()
      // Ternaire : image présente → pas de .memotion-tv-fs-text dans le body
      expect(body.querySelector('.memotion-tv-fs-text')).toBeNull()
    })

    it('la grille est toujours rendue avec la classe memotion-grid-dimmed', () => {
      const { container } = renderTV()
      expect(container.querySelector('.memotion-grid-dimmed')).not.toBeNull()
    })
  })

  // -------------------------------------------------------------------------
  // SC3 — SELECTED sans RECTO_IMAGE : le RECTO_THEME apparaît dans le body
  //
  // Structure JSX réelle (SELECTED sans image) :
  //   .memotion-tv-fs-body ← <p class="memotion-tv-fs-text">RECTO_THEME</p>
  // -------------------------------------------------------------------------

  describe('SC3 — SELECTED sans RECTO_IMAGE', () => {
    beforeEach(() => {
      useGame.mockReturnValue(makeMemotionMock('SELECTED', 'card-2'))
    })

    it('affiche le container fullscreen', () => {
      const { container } = renderTV()
      expect(container.querySelector('.memotion-tv-fullscreen')).not.toBeNull()
    })

    it('le body fullscreen ne contient pas d\'image', () => {
      const { container } = renderTV()
      const body = container.querySelector('.memotion-tv-fs-body')
      expect(body).not.toBeNull()
      expect(body.querySelector('img')).toBeNull()
    })

    it('le body fullscreen affiche le RECTO_THEME quand pas d\'image', () => {
      const { container } = renderTV()
      const body = container.querySelector('.memotion-tv-fs-body')
      expect(body).not.toBeNull()
      const text = body.querySelector('.memotion-tv-fs-text')
      expect(text).not.toBeNull()
      expect(text.textContent).toBe('Thème Littérature')
    })
  })

  // -------------------------------------------------------------------------
  // SC4 — QUESTION : timer sibling du header, image + texte dans body
  //
  // Structure JSX réelle (QUESTION) :
  //   .memotion-tv-fullscreen
  //     .memotion-tv-fs-timer   ← row 1 CSS grid (Timer)
  //     .memotion-tv-fs-header  ← row 2 CSS grid (thème + étoiles + équipe)
  //     .memotion-tv-fs-body    ← row 3 CSS grid (QUESTION_IMAGE + QUESTION_TEXT)
  // -------------------------------------------------------------------------

  describe('SC4 — QUESTION subphase', () => {
    beforeEach(() => {
      useGame.mockReturnValue(makeMemotionMock('QUESTION', 'card-1'))
    })

    it('affiche le container fullscreen', () => {
      const { container } = renderTV()
      expect(container.querySelector('.memotion-tv-fullscreen')).not.toBeNull()
    })

    it('le timer est un sibling direct dans .memotion-tv-fullscreen (.memotion-tv-fs-timer)', () => {
      const { container } = renderTV()
      const overlay = container.querySelector('.memotion-tv-fullscreen')
      expect(overlay).not.toBeNull()
      // .memotion-tv-fs-timer est un enfant direct du fullscreen (pas dans le header)
      const timerWrapper = overlay.querySelector('.memotion-tv-fs-timer')
      expect(timerWrapper).not.toBeNull()
      expect(timerWrapper.querySelector('[data-testid="timer"]')).not.toBeNull()
    })

    it('le header ne contient PAS de timer (contrairement à une version antérieure)', () => {
      const { container } = renderTV()
      const header = container.querySelector('.memotion-tv-fs-header')
      expect(header).not.toBeNull()
      expect(header.querySelector('.memotion-tv-fs-timer')).toBeNull()
    })

    it('le header fullscreen contient le RECTO_THEME', () => {
      const { container } = renderTV()
      const header = container.querySelector('.memotion-tv-fs-header')
      expect(header.textContent).toContain('Thème Star Wars')
    })

    it('le body fullscreen contient la QUESTION_IMAGE', () => {
      const { container } = renderTV()
      const body = container.querySelector('.memotion-tv-fs-body')
      const img = body.querySelector('img.memotion-tv-fs-img')
      expect(img).not.toBeNull()
      expect(img.getAttribute('src')).toBe('/files/q1.jpg')
    })

    it('le body fullscreen contient le QUESTION_TEXT', () => {
      const { container } = renderTV()
      const body = container.querySelector('.memotion-tv-fs-body')
      const text = body.querySelector('.memotion-tv-fs-text')
      expect(text).not.toBeNull()
      expect(text.textContent).toBe('Dans quel épisode Dark Vador révèle-t-il sa filiation ?')
    })

    it('le body ne contient pas de texte si QUESTION_TEXT est absent', () => {
      const cardNoText = { ...CARD_WITH_IMG, QUESTION_TEXT: undefined }
      useGame.mockReturnValue(makeMemotionMock('QUESTION', 'card-1', [cardNoText, CARD_NO_IMG]))
      const { container } = renderTV()
      const body = container.querySelector('.memotion-tv-fs-body')
      expect(body.querySelector('.memotion-tv-fs-text')).toBeNull()
    })
  })

  // -------------------------------------------------------------------------
  // SC5 — REVEAL : header thème sans timer, image + texte vert dans body
  //
  // Structure JSX réelle (REVEAL) :
  //   .memotion-tv-fullscreen.memotion-tv-reveal
  //     .memotion-tv-fs-header  ← thème + étoiles + pts (pas d'équipe, pas de timer)
  //     .memotion-tv-fs-body    ← ANSWER_IMAGE + ANSWER_TEXT (.memotion-tv-answer-text)
  // -------------------------------------------------------------------------

  describe('SC5 — REVEAL subphase', () => {
    beforeEach(() => {
      useGame.mockReturnValue(makeMemotionMock('REVEAL', 'card-1'))
    })

    it('affiche le container fullscreen avec classe memotion-tv-reveal', () => {
      const { container } = renderTV()
      expect(container.querySelector('.memotion-tv-reveal')).not.toBeNull()
    })

    it('le header fullscreen contient le RECTO_THEME', () => {
      const { container } = renderTV()
      const header = container.querySelector('.memotion-tv-fs-header')
      expect(header).not.toBeNull()
      expect(header.textContent).toContain('Thème Star Wars')
    })

    it('le header REVEAL ne contient PAS de timer (différence avec QUESTION)', () => {
      const { container } = renderTV()
      const overlay = container.querySelector('.memotion-tv-fullscreen')
      expect(overlay.querySelector('.memotion-tv-fs-timer')).toBeNull()
    })

    it('le body fullscreen contient l\'ANSWER_IMAGE', () => {
      const { container } = renderTV()
      const body = container.querySelector('.memotion-tv-fs-body')
      const img = body.querySelector('img.memotion-tv-fs-img')
      expect(img).not.toBeNull()
      expect(img.getAttribute('src')).toBe('/files/a1.jpg')
    })

    it('le body fullscreen affiche l\'ANSWER_TEXT avec la classe memotion-tv-answer-text (vert)', () => {
      const { container } = renderTV()
      const body = container.querySelector('.memotion-tv-fs-body')
      const answerText = body.querySelector('.memotion-tv-answer-text')
      expect(answerText).not.toBeNull()
      expect(answerText.textContent).toBe("L'Empire contre-attaque (Épisode V)")
    })

    it('le body ne contient pas de texte si ANSWER_TEXT est absent', () => {
      const cardNoAnswer = { ...CARD_WITH_IMG, ANSWER_TEXT: undefined }
      useGame.mockReturnValue(makeMemotionMock('REVEAL', 'card-1', [cardNoAnswer, CARD_NO_IMG]))
      const { container } = renderTV()
      const body = container.querySelector('.memotion-tv-fs-body')
      expect(body.querySelector('.memotion-tv-fs-text')).toBeNull()
    })
  })

  // -------------------------------------------------------------------------
  // SC6 — Non-régression : les vues non-MEMOTION ne rendent pas les classes MEMOTION
  // -------------------------------------------------------------------------

  describe('SC6 — Non-régression : vues non-MEMOTION', () => {
    it('aucun overlay MEMOTION rendu pour une question de type QUIZ', () => {
      useGame.mockReturnValue({
        gameState: {
          phase: 'STARTED',
          remote: 'GAME',
          timer: 15,
          totalTime: 30,
          question: { TYPE: 'QUIZ', QUESTION_TEXT: 'Capitale de la France ?' },
          MEMORY_PARTICIPATING_TEAMS: [],
          newGameBackgrounds: [],
        },
        teams: {},
        bumpers: {},
        flipMemoryCard: vi.fn(),
        showQRCode: false,
        selectMotionCard: vi.fn(),
      })

      const { container } = renderTV()
      expect(container.querySelector('.memotion-tv-fullscreen')).toBeNull()
      expect(container.querySelector('.memotion-card')).toBeNull()
    })

    it('aucun overlay MEMOTION rendu en phase STOPPED sans question MEMOTION', () => {
      useGame.mockReturnValue({
        gameState: {
          phase: 'STOPPED',
          remote: 'GAME',
          timer: 0,
          totalTime: 30,
          question: { TYPE: 'QCM', QUESTION_TEXT: 'Question QCM ?' },
          MEMORY_PARTICIPATING_TEAMS: [],
          newGameBackgrounds: [],
        },
        teams: {},
        bumpers: {},
        flipMemoryCard: vi.fn(),
        showQRCode: false,
        selectMotionCard: vi.fn(),
      })

      const { container } = renderTV()
      expect(container.querySelector('.memotion-tv-fullscreen')).toBeNull()
      expect(container.querySelector('.memotion-grid-dimmed')).toBeNull()
    })

    it('aucun overlay MEMOTION rendu quand question est null', () => {
      useGame.mockReturnValue({
        gameState: {
          phase: 'STARTED',
          remote: 'GAME',
          timer: 10,
          totalTime: 30,
          question: null,
          MEMORY_PARTICIPATING_TEAMS: [],
          newGameBackgrounds: [],
        },
        teams: {},
        bumpers: {},
        flipMemoryCard: vi.fn(),
        showQRCode: false,
        selectMotionCard: vi.fn(),
      })

      const { container } = renderTV()
      expect(container.querySelector('.memotion-tv-fullscreen')).toBeNull()
    })
  })

  // -------------------------------------------------------------------------
  // SC7 — Non-régression bugfix UI cartes MEMOTION
  //        • Suppression du span .memotion-card-pts dans le footer
  //        • Structure header : un seul enfant (.memotion-card-title)
  // -------------------------------------------------------------------------

  describe('SC7 — Bugfix UI carte MEMOTION (suppression pts + structure header)', () => {
    beforeEach(() => {
      useGame.mockReturnValue(makeMemotionMock('GRID'))
    })

    // --- Footer : suppression de .memotion-card-pts ---

    it('ne rend PAS .memotion-card-pts dans le footer des cartes (régression)', () => {
      const { container } = renderTV()
      // Avant le fix, un <span class="memotion-card-pts"> était présent dans chaque footer.
      // Après le fix, seul .memotion-card-stars subsiste.
      expect(container.querySelector('.memotion-card-pts')).toBeNull()
    })

    it('chaque footer ne contient que .memotion-card-stars (aucun autre span nommé)', () => {
      const { container } = renderTV()
      const footers = container.querySelectorAll('.memotion-card-footer')
      expect(footers.length).toBeGreaterThan(0)
      footers.forEach(footer => {
        expect(footer.querySelector('.memotion-card-stars')).not.toBeNull()
        expect(footer.querySelector('.memotion-card-pts')).toBeNull()
      })
    })

    it('le footer affiche les étoiles sans texte supplémentaire (pas de "pts")', () => {
      const { container } = renderTV()
      const footers = container.querySelectorAll('.memotion-card-footer')
      footers.forEach(footer => {
        // Le contenu textuel du footer ne doit contenir que des étoiles ★
        expect(footer.textContent).toMatch(/^★+$/)
      })
    })

    // --- Header : structure centrage (un seul enfant : .memotion-card-title) ---

    it('chaque header contient exactement un enfant direct (.memotion-card-title)', () => {
      const { container } = renderTV()
      const headers = container.querySelectorAll('.memotion-card-header')
      expect(headers.length).toBeGreaterThan(0)
      headers.forEach(header => {
        // Après le fix : le header ne contient que le span .memotion-card-title.
        // Un header avec plusieurs enfants aurait empêché le centrage flex correct.
        expect(header.children.length).toBe(1)
        expect(header.children[0].classList.contains('memotion-card-title')).toBe(true)
      })
    })

    it('.memotion-card-title contient le RECTO_THEME (et rien d\'autre)', () => {
      const { container } = renderTV()
      const titles = container.querySelectorAll('.memotion-card-title')
      expect(titles[0].textContent).toBe('Thème Star Wars')
      expect(titles[1].textContent).toBe('Thème Littérature')
    })
  })
})
