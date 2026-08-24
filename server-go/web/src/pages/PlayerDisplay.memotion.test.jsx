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
  // SC4 — QUESTION : overlay démarre à top:10vh — timer visible dans zone-timer au-dessus
  //
  // Structure JSX réelle (QUESTION) — post bugfix #77 :
  //   .memotion-tv-fullscreen  [position: fixed, top: 10vh]
  //     .memotion-tv-fs-header  ← QUESTION_TEXT via .memotion-tv-fs-question-text (row1)
  //     .memotion-tv-fs-body    ← QUESTION_IMAGE uniquement (row2)
  //     .memotion-tv-fs-footer  ← vide (row3 toujours présent — grille CSS 1fr 4fr 1fr)
  //   (timer dans .zone-timer de la grille, au-dessus de l'overlay — non dimmed)
  // -------------------------------------------------------------------------

  describe('SC4 — QUESTION subphase', () => {
    beforeEach(() => {
      useGame.mockReturnValue(makeMemotionMock('QUESTION', 'card-1'))
    })

    it('affiche le container fullscreen', () => {
      const { container } = renderTV()
      expect(container.querySelector('.memotion-tv-fullscreen')).not.toBeNull()
    })

    it("le timer n'est PAS à l'intérieur de l'overlay — il reste dans zone-timer au-dessus", () => {
      // Le timer est rendu dans .zone-timer de la grille sous-jacente (non dimmed).
      // L'overlay débute à top:10vh pour laisser le timer visible.
      const { container } = renderTV()
      const overlay = container.querySelector('.memotion-tv-fullscreen')
      expect(overlay).not.toBeNull()
      expect(overlay.querySelector('.memotion-tv-fs-timer')).toBeNull()
      // Le Timer est quand même rendu quelque part dans la page (dans zone-timer)
      expect(container.querySelector('[data-testid="timer"]')).not.toBeNull()
    })

    it('le header ne contient PAS de timer (contrairement à une version antérieure)', () => {
      const { container } = renderTV()
      const header = container.querySelector('.memotion-tv-fs-header')
      expect(header).not.toBeNull()
      expect(header.querySelector('.memotion-tv-fs-timer')).toBeNull()
    })

    it('le header fullscreen contient le QUESTION_TEXT — bugfix #77 (RECTO_THEME déplacé)', () => {
      // Avant fix : header affichait RECTO_THEME + étoiles + pts + équipe.
      // Après fix : header affiche QUESTION_TEXT via .memotion-tv-fs-question-text (row1).
      const { container } = renderTV()
      const header = container.querySelector('.memotion-tv-fs-header')
      expect(header.textContent).toContain('Dans quel épisode Dark Vador révèle-t-il sa filiation ?')
    })

    it('le body fullscreen contient la QUESTION_IMAGE', () => {
      const { container } = renderTV()
      const body = container.querySelector('.memotion-tv-fs-body')
      const img = body.querySelector('img.memotion-tv-fs-img')
      expect(img).not.toBeNull()
      expect(img.getAttribute('src')).toBe('/files/q1.jpg')
    })

    it('le header contient .memotion-tv-fs-question-text — QUESTION_TEXT en row1, bugfix #77', () => {
      // Avant fix : QUESTION_TEXT était dans le body (.memotion-tv-fs-text dans .memotion-tv-fs-body).
      // Après fix : QUESTION_TEXT est dans le header via .memotion-tv-fs-question-text (row1).
      const { container } = renderTV()
      const header = container.querySelector('.memotion-tv-fs-header')
      const text = header.querySelector('.memotion-tv-fs-question-text')
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
  // SC5 — REVEAL : rappel question en row1, image réponse en row2, texte réponse en row3
  //
  // Structure JSX réelle (REVEAL) — post bugfix #77 :
  //   .memotion-tv-fullscreen.memotion-tv-reveal  [position: fixed, top: 10vh]
  //     .memotion-tv-fs-header  ← QUESTION_TEXT rappel (.memotion-tv-fs-recall, opacity réduite)
  //     .memotion-tv-fs-body    ← ANSWER_IMAGE si dispo, sinon ANSWER_TEXT (fallback texte seul)
  //     .memotion-tv-fs-footer  ← ANSWER_TEXT si image+texte coexistent, sinon vide (toujours présent)
  // -------------------------------------------------------------------------

  describe('SC5 — REVEAL subphase', () => {
    beforeEach(() => {
      useGame.mockReturnValue(makeMemotionMock('REVEAL', 'card-1'))
    })

    it('affiche le container fullscreen avec classe memotion-tv-reveal', () => {
      const { container } = renderTV()
      expect(container.querySelector('.memotion-tv-reveal')).not.toBeNull()
    })

    it('le header fullscreen contient le QUESTION_TEXT en rappel — bugfix #77 (RECTO_THEME éliminé)', () => {
      // Avant fix : header REVEAL affichait RECTO_THEME + étoiles + pts.
      // Après fix : header affiche QUESTION_TEXT avec classe .memotion-tv-fs-recall (atténué, row1).
      const { container } = renderTV()
      const header = container.querySelector('.memotion-tv-fs-header')
      expect(header).not.toBeNull()
      const recall = header.querySelector('.memotion-tv-fs-recall')
      expect(recall).not.toBeNull()
      expect(recall.textContent).toContain('Dans quel épisode Dark Vador révèle-t-il sa filiation ?')
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

    it('quand ANSWER_IMAGE est présente, le body contient l\'image et PAS le texte réponse', () => {
      // card-1 a ANSWER_IMAGE → body = image uniquement; ANSWER_TEXT est dans le footer
      const { container } = renderTV()
      const body = container.querySelector('.memotion-tv-fs-body')
      expect(body.querySelector('img.memotion-tv-fs-img')).not.toBeNull()
      expect(body.querySelector('.memotion-tv-answer-text')).toBeNull()
    })

    it('le body ne contient pas de texte si ANSWER_TEXT est absent', () => {
      const cardNoAnswer = { ...CARD_WITH_IMG, ANSWER_TEXT: undefined }
      useGame.mockReturnValue(makeMemotionMock('REVEAL', 'card-1', [cardNoAnswer, CARD_NO_IMG]))
      const { container } = renderTV()
      const body = container.querySelector('.memotion-tv-fs-body')
      expect(body.querySelector('.memotion-tv-fs-text')).toBeNull()
    })

    // --- Footer (.memotion-tv-fs-footer) ---

    it('quand ANSWER_IMAGE && ANSWER_TEXT, le footer est présent et contient le texte réponse', () => {
      // card-1 a les deux → footer visible avec ANSWER_TEXT
      const { container } = renderTV()
      const footer = container.querySelector('.memotion-tv-fs-footer')
      expect(footer).not.toBeNull()
      expect(footer.textContent).toBe("L'Empire contre-attaque (Épisode V)")
    })

    it('quand ANSWER_IMAGE sans ANSWER_TEXT, le footer est présent mais vide — bugfix #77', () => {
      // Avant fix : footer absent si ANSWER_TEXT manquant.
      // Après fix : footer toujours présent (row3 vide — structure CSS 1fr 4fr 1fr garantie).
      const cardImgOnly = { ...CARD_WITH_IMG, ANSWER_TEXT: undefined }
      useGame.mockReturnValue(makeMemotionMock('REVEAL', 'card-1', [cardImgOnly, CARD_NO_IMG]))
      const { container } = renderTV()
      const footer = container.querySelector('.memotion-tv-fs-footer')
      expect(footer).not.toBeNull()
      expect(footer.textContent.trim()).toBe('')
    })

    it('quand ANSWER_TEXT sans ANSWER_IMAGE, le texte réponse est dans le body et le footer est vide — bugfix #77', () => {
      // card-2 n'a pas d'ANSWER_IMAGE → texte fallback dans body via .memotion-tv-answer-text
      // Avant fix : footer absent. Après fix : footer présent mais vide (row3 garantie).
      useGame.mockReturnValue(makeMemotionMock('REVEAL', 'card-2'))
      const { container } = renderTV()
      const body = container.querySelector('.memotion-tv-fs-body')
      const textEl = body.querySelector('.memotion-tv-answer-text')
      expect(textEl).not.toBeNull()
      expect(textEl.textContent).toBe('Victor Hugo')
      const footer = container.querySelector('.memotion-tv-fs-footer')
      expect(footer).not.toBeNull()
      expect(footer.textContent.trim()).toBe('')
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
  // SC8 — Non-régression bugfix layout fullscreen (1/6 header + 4/6 body + 1/6 timer)
  //
  // Bug corrigé : les cards MEMOTION plein écran ne respectaient pas le layout
  // standard 1/6 + 4/6 + 1/6. Le fix applique un CSS grid `1fr 4fr 1fr` sur
  // `.memotion-tv-fullscreen` avec header→row1, body→row2, timer→row3.
  //
  // Ces tests vérifient la structure DOM qui conditionne le bon fonctionnement
  // du CSS grid : header, body et footer doivent être des enfants directs du
  // container `.memotion-tv-fullscreen`.
  // -------------------------------------------------------------------------

  describe('SC8 — Régression layout fullscreen 1/6-4/6-1/6 (SELECTED)', () => {
    beforeEach(() => {
      useGame.mockReturnValue(makeMemotionMock('SELECTED', 'card-1'))
    })

    it('le container fullscreen a exactement 3 enfants directs (header + body + footer, bugfix #77)', () => {
      const { container } = renderTV()
      const overlay = container.querySelector('.memotion-tv-fullscreen')
      expect(overlay).not.toBeNull()
      // SELECTED = header (row1 titre) + body (row2 image) + footer (row3 étoiles) — layout 1fr 4fr 1fr
      expect(overlay.children.length).toBe(3)
    })

    it('le 1er enfant direct du fullscreen est le header (.memotion-tv-fs-header)', () => {
      const { container } = renderTV()
      const overlay = container.querySelector('.memotion-tv-fullscreen')
      expect(overlay.children[0].classList.contains('memotion-tv-fs-header')).toBe(true)
    })

    it('le 2e enfant direct du fullscreen est le body (.memotion-tv-fs-body)', () => {
      const { container } = renderTV()
      const overlay = container.querySelector('.memotion-tv-fullscreen')
      expect(overlay.children[1].classList.contains('memotion-tv-fs-body')).toBe(true)
    })

    it('.memotion-tv-fs-header est enfant DIRECT du fullscreen (pas imbriqué)', () => {
      const { container } = renderTV()
      const overlay = container.querySelector('.memotion-tv-fullscreen')
      // Sélection directe par querySelector puis vérification du parentElement
      const header = container.querySelector('.memotion-tv-fs-header')
      expect(header).not.toBeNull()
      expect(header.parentElement).toBe(overlay)
    })

    it('.memotion-tv-fs-body est enfant DIRECT du fullscreen (pas imbriqué)', () => {
      const { container } = renderTV()
      const overlay = container.querySelector('.memotion-tv-fullscreen')
      const body = container.querySelector('.memotion-tv-fs-body')
      expect(body).not.toBeNull()
      expect(body.parentElement).toBe(overlay)
    })

    it('SELECTED ne contient PAS de .memotion-tv-fs-timer (zone 1/6 timer absente)', () => {
      const { container } = renderTV()
      const overlay = container.querySelector('.memotion-tv-fullscreen')
      expect(overlay.querySelector('.memotion-tv-fs-timer')).toBeNull()
    })

    it("l'image dans le body porte la classe memotion-tv-fs-img (object-fit: contain)", () => {
      const { container } = renderTV()
      const body = container.querySelector('.memotion-tv-fs-body')
      const img = body.querySelector('img')
      expect(img).not.toBeNull()
      expect(img.classList.contains('memotion-tv-fs-img')).toBe(true)
    })
  })

  describe('SC8 — Régression layout fullscreen 1/6-4/6-1/6 (QUESTION)', () => {
    beforeEach(() => {
      useGame.mockReturnValue(makeMemotionMock('QUESTION', 'card-1'))
    })

    it('le container fullscreen a exactement 3 enfants directs (header + body + footer vide, bugfix #77)', () => {
      const { container } = renderTV()
      const overlay = container.querySelector('.memotion-tv-fullscreen')
      expect(overlay).not.toBeNull()
      // QUESTION = header (row1 question text) + body (row2 image) + footer vide (row3) — layout 1fr 4fr 1fr
      expect(overlay.children.length).toBe(3)
    })

    it('le 1er enfant DOM du fullscreen est le header (.memotion-tv-fs-header)', () => {
      const { container } = renderTV()
      const overlay = container.querySelector('.memotion-tv-fullscreen')
      expect(overlay.children[0].classList.contains('memotion-tv-fs-header')).toBe(true)
    })

    it('le 2e enfant DOM du fullscreen est le body (.memotion-tv-fs-body)', () => {
      const { container } = renderTV()
      const overlay = container.querySelector('.memotion-tv-fullscreen')
      expect(overlay.children[1].classList.contains('memotion-tv-fs-body')).toBe(true)
    })

    it("l'overlay QUESTION ne contient PAS de .memotion-tv-fs-timer", () => {
      // Le timer est dans zone-timer de la grille, au-dessus de l'overlay (top:10vh) — non dimmed.
      const { container } = renderTV()
      const overlay = container.querySelector('.memotion-tv-fullscreen')
      expect(overlay.querySelector('.memotion-tv-fs-timer')).toBeNull()
    })

    it("le timer global [data-testid='timer'] est quand même rendu dans la page (zone-timer)", () => {
      // Régression : le timer ne doit pas disparaître — juste ne plus être dans l'overlay.
      const { container } = renderTV()
      expect(container.querySelector('[data-testid="timer"]')).not.toBeNull()
    })

    it('.memotion-tv-fs-header est enfant DIRECT du fullscreen (pas imbriqué)', () => {
      const { container } = renderTV()
      const overlay = container.querySelector('.memotion-tv-fullscreen')
      const header = container.querySelector('.memotion-tv-fs-header')
      expect(header.parentElement).toBe(overlay)
    })

    it('.memotion-tv-fs-body est enfant DIRECT du fullscreen (pas imbriqué)', () => {
      const { container } = renderTV()
      const overlay = container.querySelector('.memotion-tv-fullscreen')
      const body = container.querySelector('.memotion-tv-fs-body')
      expect(body.parentElement).toBe(overlay)
    })

    it("l'image dans le body porte la classe memotion-tv-fs-img (object-fit: contain)", () => {
      const { container } = renderTV()
      const body = container.querySelector('.memotion-tv-fs-body')
      const img = body.querySelector('img')
      expect(img).not.toBeNull()
      expect(img.classList.contains('memotion-tv-fs-img')).toBe(true)
    })
  })

  describe('SC8 — Régression layout fullscreen 1/6-4/6-1/6 (REVEAL)', () => {
    beforeEach(() => {
      useGame.mockReturnValue(makeMemotionMock('REVEAL', 'card-1'))
    })

    it('le container fullscreen a exactement 3 enfants directs (header + body + footer) quand image + texte', () => {
      const { container } = renderTV()
      const overlay = container.querySelector('.memotion-tv-fullscreen')
      expect(overlay).not.toBeNull()
      // card-1 a ANSWER_IMAGE && ANSWER_TEXT → header + body (image) + footer (texte) = 3 enfants
      expect(overlay.children.length).toBe(3)
    })

    it('le 1er enfant direct du fullscreen est le header (.memotion-tv-fs-header)', () => {
      const { container } = renderTV()
      const overlay = container.querySelector('.memotion-tv-fullscreen')
      expect(overlay.children[0].classList.contains('memotion-tv-fs-header')).toBe(true)
    })

    it('le 2e enfant direct du fullscreen est le body (.memotion-tv-fs-body)', () => {
      const { container } = renderTV()
      const overlay = container.querySelector('.memotion-tv-fullscreen')
      expect(overlay.children[1].classList.contains('memotion-tv-fs-body')).toBe(true)
    })

    it('REVEAL ne contient PAS de .memotion-tv-fs-timer (zone 1/6 timer absente)', () => {
      const { container } = renderTV()
      const overlay = container.querySelector('.memotion-tv-fullscreen')
      expect(overlay.querySelector('.memotion-tv-fs-timer')).toBeNull()
    })

    it('.memotion-tv-fs-header est enfant DIRECT du fullscreen (pas imbriqué)', () => {
      const { container } = renderTV()
      const overlay = container.querySelector('.memotion-tv-fullscreen')
      const header = container.querySelector('.memotion-tv-fs-header')
      expect(header.parentElement).toBe(overlay)
    })

    it('.memotion-tv-fs-body est enfant DIRECT du fullscreen (pas imbriqué)', () => {
      const { container } = renderTV()
      const overlay = container.querySelector('.memotion-tv-fullscreen')
      const body = container.querySelector('.memotion-tv-fs-body')
      expect(body.parentElement).toBe(overlay)
    })

    it("l'image dans le body porte la classe memotion-tv-fs-img (object-fit: contain)", () => {
      const { container } = renderTV()
      const body = container.querySelector('.memotion-tv-fs-body')
      const img = body.querySelector('img')
      expect(img).not.toBeNull()
      expect(img.classList.contains('memotion-tv-fs-img')).toBe(true)
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
