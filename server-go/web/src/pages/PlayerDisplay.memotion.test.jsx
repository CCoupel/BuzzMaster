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
  // #185 (correction ponctuelle) — hintMarkers exposé via un attribut data-*
  // pour vérifier son câblage sans dépendre du rendu réel de Timer.jsx
  // (déjà testé en isolation) ; rétrocompatible avec les tests existants qui
  // ne lisent que le textContent (currentTime).
  default: ({ currentTime, hintMarkers }) => (
    <div data-testid="timer" data-hint-markers={hintMarkers ? JSON.stringify(hintMarkers) : ''}>
      {currentTime}
    </div>
  ),
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

// ---------------------------------------------------------------------------
// #185/C-F2 — carte MEMOTION de type QCM sur /tv. Point de délégation posé
// en B-F5 (#184) : `cardType === 'SPEEDY'` gardait le contenu SPEEDY, ce lot
// ajoute la branche QCM sans y toucher (test d'agnosticité, contrat §10).
// Affichage seul (contrat §7.1) : les 4 réponses de LA CARTE, invalidations
// progressives (via MEMOTION_ACTIVE.STATE, contrat §5.3), bonne réponse en
// couleur au REVEAL — aucune action entrante nouvelle.
// ---------------------------------------------------------------------------

const CARD_QCM = {
  ID: 'card-3',
  RECTO_THEME: 'Capitales d\'Europe',
  DIFFICULTY: 2,
  TYPE: 'QCM',
  QUESTION_TEXT: 'Quelle est la capitale de la Slovénie ?',
  QCM_ANSWERS: { RED: 'Bratislava', GREEN: 'Ljubljana', YELLOW: 'Zagreb', BLUE: 'Sarajevo' },
  QCM_CORRECT: 'GREEN',
}

/** Variante de makeMemotionMock incluant MEMOTION_ACTIVE (état d'indices carte-scopé). */
const makeMemotionQcmMock = (subphase, invalidated = []) => {
  const base = makeMemotionMock(subphase, 'card-3', [CARD_WITH_IMG, CARD_QCM])
  base.gameState.MEMOTION_ACTIVE = {
    CARD_ID: 'card-3',
    TYPE: 'QCM',
    STATE: { QCM_INVALIDATED: invalidated },
  }
  return base
}

describe('PlayerDisplay — carte MEMOTION de type QCM (#185/C-F2)', () => {
  describe('Face VERSO (QUESTION) — 4 réponses affichées, pas de reveal', () => {
    beforeEach(() => {
      useGame.mockReturnValue(makeMemotionQcmMock('QUESTION'))
    })

    it('affiche les 4 réponses de la carte QCM active', () => {
      const { container } = renderTV()
      const grid = container.querySelector('.memotion-tv-qcm-grid')
      expect(grid).not.toBeNull()
      expect(grid.textContent).toContain('Bratislava')
      expect(grid.textContent).toContain('Ljubljana')
      expect(grid.textContent).toContain('Zagreb')
      expect(grid.textContent).toContain('Sarajevo')
    })

    it('ne marque aucune réponse "correct" avant REVEAL, même si QCM_CORRECT est connue', () => {
      const { container } = renderTV()
      expect(container.querySelectorAll('.memotion-tv-qcm-item.correct').length).toBe(0)
    })

    it("n'affiche PAS le contenu SPEEDY (ANSWER_TEXT/ANSWER_IMAGE) pour une carte QCM", () => {
      const { container } = renderTV()
      const overlay = container.querySelector('.memotion-tv-fullscreen')
      expect(overlay.textContent).not.toContain('ANSWER_TEXT')
    })
  })

  // #185 — correction ponctuelle (détectée par test-writer en C-T1) :
  // qcmHintMarkers (repères visuels de seuils sur la barre de chrono,
  // PlayerDisplay.jsx:667) était gated sur `isQcm` (question-scopée) et
  // n'apparaissait jamais pour une carte QCM active, alors que
  // l'invalidation elle-même fonctionne déjà côté carte (C-B1). Étendu au
  // contexte carte sans nouvelle donnée serveur — juste le déclenchement
  // visuel client.
  describe('Repères visuels de seuils d\'indices (qcmHintMarkers) — même comportement observable qu\'une question QCM classique', () => {
    const CARD_QCM_WITH_HINTS = {
      ...CARD_QCM,
      QCM_HINTS_ENABLED: true,
      QCM_HINT_THRESHOLD_1: 0.25,
      QCM_HINT_THRESHOLD_2: 0.125,
    }

    it('affiche les repères de seuils sur le chrono quand la carte active est QCM avec indices activés', () => {
      const mock = makeMemotionMock('QUESTION', 'card-3', [CARD_WITH_IMG, CARD_QCM_WITH_HINTS])
      useGame.mockReturnValue(mock)
      const { container } = renderTV()
      const timer = container.querySelector('[data-testid="timer"]')
      expect(timer.getAttribute('data-hint-markers')).not.toBe('')
      const markers = JSON.parse(timer.getAttribute('data-hint-markers'))
      expect(markers.length).toBe(2)
    })

    it('ne rend aucun repère quand la carte QCM active n\'a pas les indices activés', () => {
      const mock = makeMemotionMock('QUESTION', 'card-3', [CARD_WITH_IMG, CARD_QCM])
      useGame.mockReturnValue(mock)
      const { container } = renderTV()
      const timer = container.querySelector('[data-testid="timer"]')
      expect(timer.getAttribute('data-hint-markers')).toBe('')
    })

    it('ne rend aucun repère quand la carte active est SPEEDY (non-régression)', () => {
      const mock = makeMemotionMock('QUESTION', 'card-1', [CARD_WITH_IMG, CARD_QCM_WITH_HINTS])
      useGame.mockReturnValue(mock)
      const { container } = renderTV()
      const timer = container.querySelector('[data-testid="timer"]')
      expect(timer.getAttribute('data-hint-markers')).toBe('')
    })

    it('marque un seuil "triggered" une fois la réponse correspondante invalidée (MEMOTION_ACTIVE.STATE)', () => {
      const mock = makeMemotionMock('QUESTION', 'card-3', [CARD_WITH_IMG, CARD_QCM_WITH_HINTS])
      mock.gameState.MEMOTION_ACTIVE = { CARD_ID: 'card-3', TYPE: 'QCM', STATE: { QCM_INVALIDATED: ['RED'] } }
      useGame.mockReturnValue(mock)
      const { container } = renderTV()
      const timer = container.querySelector('[data-testid="timer"]')
      const markers = JSON.parse(timer.getAttribute('data-hint-markers'))
      expect(markers[0].triggered).toBe(true)
    })
  })

  describe('Face VERSO (QUESTION) — indices progressifs (invalidation)', () => {
    it('marque une réponse invalidée (issue de MEMOTION_ACTIVE.STATE.QCM_INVALIDATED)', () => {
      useGame.mockReturnValue(makeMemotionQcmMock('QUESTION', ['RED']))
      const { container } = renderTV()
      const items = container.querySelectorAll('.memotion-tv-qcm-item')
      const redItem = Array.from(items).find(el => el.textContent.includes('Bratislava'))
      expect(redItem.classList.contains('invalidated')).toBe(true)
    })

    it('ne marque aucune réponse invalidée quand la liste est vide', () => {
      useGame.mockReturnValue(makeMemotionQcmMock('QUESTION', []))
      const { container } = renderTV()
      expect(container.querySelectorAll('.memotion-tv-qcm-item.invalidated').length).toBe(0)
    })
  })

  describe('Face REVEAL — bonne réponse en couleur, grille conservée', () => {
    beforeEach(() => {
      useGame.mockReturnValue(makeMemotionQcmMock('REVEAL'))
    })

    it('conserve la grille des 4 réponses (pas de saut visuel entre VERSO et REVEAL)', () => {
      const { container } = renderTV()
      expect(container.querySelector('.memotion-tv-qcm-grid')).not.toBeNull()
      expect(container.querySelectorAll('.memotion-tv-qcm-item').length).toBe(4)
    })

    it('marque UNIQUEMENT la bonne réponse (QCM_CORRECT) comme "correct"', () => {
      const { container } = renderTV()
      const items = container.querySelectorAll('.memotion-tv-qcm-item')
      const correctItems = Array.from(items).filter(el => el.classList.contains('correct'))
      expect(correctItems).toHaveLength(1)
      expect(correctItems[0].textContent).toContain('Ljubljana')
    })
  })

  describe('Non-régression — carte SPEEDY de la même manche', () => {
    it('une carte SPEEDY continue d\'afficher son propre contenu, pas de grille QCM', () => {
      const mock = makeMemotionMock('QUESTION', 'card-1', [CARD_WITH_IMG, CARD_QCM])
      useGame.mockReturnValue(mock)
      const { container } = renderTV()
      expect(container.querySelector('.memotion-tv-qcm-grid')).toBeNull()
      expect(container.querySelector('.memotion-tv-fs-question-text').textContent)
        .toBe('Dans quel épisode Dark Vador révèle-t-il sa filiation ?')
    })
  })

  describe('Aucune action entrante nouvelle (contrat §7.1)', () => {
    it('la grille QCM en carte ne rend aucun élément interactif (bouton/input)', () => {
      useGame.mockReturnValue(makeMemotionQcmMock('QUESTION'))
      const { container } = renderTV()
      const grid = container.querySelector('.memotion-tv-qcm-grid')
      expect(grid.querySelectorAll('button, input').length).toBe(0)
    })
  })
})

// ---------------------------------------------------------------------------
// #187 (v7.1.0) — carte MEMOTION de type MEMORY. La grille occupe Row 2
// (body) des faces QUESTION/REVEAL — MEMORY n'a pas de `question` MediaSlot
// (contrat §7 : "recto + N paires"), la grille EST le contenu média de la
// carte. Task 14 du plan : la révélation est routée par
// `cardHostContext.revealed` (jamais `gameState.phase === 'REVEALED'`,
// inatteignable en MEMOTION). Aucune restriction par équipe (task 2.1, même
// règle que la grille MEMORY question-scopée) : le serveur reste seule
// autorité sur le tour (contrat websocket-actions.md, FLIP_MEMORY_CARD).
// ---------------------------------------------------------------------------

const CARD_MEMORY = {
  ID: 'card-3',
  RECTO_THEME: 'Paires historiques',
  DIFFICULTY: 2,
  TYPE: 'MEMORY',
  QUESTION_TEXT: 'Retrouvez les paires',
  MEMORY_PAIRS: [
    { ID: 1, CARD1: { TEXT: 'Napoléon' }, CARD2: { TEXT: '1804' } },
    { ID: 2, CARD1: { TEXT: 'De Gaulle' }, CARD2: { TEXT: '1958' } },
  ],
}

/** Variante de makeMemotionMock incluant MEMOTION_ACTIVE (état MEMORY carte-scopé). */
const makeMemotionMemoryMock = (subphase, state = {}) => {
  const base = makeMemotionMock(subphase, 'card-3', [CARD_WITH_IMG, CARD_MEMORY])
  base.gameState.MEMOTION_ACTIVE = {
    CARD_ID: 'card-3',
    TYPE: 'MEMORY',
    STATE: {
      MEMORY_FLIPPED_CARDS: state.flippedCards || [],
      MEMORY_MATCHED_PAIRS: state.matchedPairs || [],
      MEMORY_ERRORS: state.errors || 0,
    },
  }
  return base
}

describe('PlayerDisplay — carte MEMOTION de type MEMORY (#187)', () => {
  describe('Face VERSO (QUESTION) — grille face cachée, cliquable', () => {
    beforeEach(() => {
      useGame.mockReturnValue(makeMemotionMemoryMock('QUESTION'))
    })

    it('affiche la grille MEMORY de la carte active (4 cartes = 2 paires)', () => {
      const { container } = renderTV()
      const grid = container.querySelector('.memotion-tv-memory-grid')
      expect(grid).not.toBeNull()
      expect(container.querySelectorAll('.memotion-tv-memory-card').length).toBe(4)
    })

    it('les cartes non retournées/trouvées sont face cachée (lettre, pas de contenu)', () => {
      const { container } = renderTV()
      const cards = container.querySelectorAll('.memotion-tv-memory-card')
      cards.forEach(card => {
        expect(card.classList.contains('up')).toBe(false)
        expect(card.querySelector('.memotion-tv-memory-card-letter')).not.toBeNull()
      })
      expect(container.textContent).not.toContain('Napoléon')
    })

    it("n'affiche PAS le contenu SPEEDY (ANSWER_TEXT/ANSWER_IMAGE) pour une carte MEMORY", () => {
      const { container } = renderTV()
      const overlay = container.querySelector('.memotion-tv-fullscreen')
      expect(overlay.textContent).not.toContain('ANSWER_TEXT')
    })

    it('une carte face cachée est cliquable (playable=true en sous-phase QUESTION) et appelle flipMemoryCard avec la portée de carte', () => {
      const mock = makeMemotionMemoryMock('QUESTION')
      useGame.mockReturnValue(mock)
      const { container } = renderTV()
      const card = container.querySelector('.memotion-tv-memory-card:not(.up)')
      expect(card.disabled).toBe(false)
      card.click()
      expect(mock.flipMemoryCard).toHaveBeenCalledTimes(1)
      expect(mock.flipMemoryCard.mock.calls[0][1]).toBe('card-3') // MOTION_CARD_ID = carte active
    })
  })

  describe('État MEMORY de LA CARTE (MEMOTION_ACTIVE.STATE), pas de la question-scopée', () => {
    it('une carte retournée (MEMORY_FLIPPED_CARDS) est affichée face visible', () => {
      useGame.mockReturnValue(makeMemotionMemoryMock('QUESTION', { flippedCards: ['1-1'] }))
      const { container } = renderTV()
      const upCards = container.querySelectorAll('.memotion-tv-memory-card.up')
      expect(upCards.length).toBe(1)
      expect(upCards[0].textContent).toContain('Napoléon')
    })

    it('une paire trouvée (MEMORY_MATCHED_PAIRS) est affichée "matched"', () => {
      useGame.mockReturnValue(makeMemotionMemoryMock('QUESTION', { matchedPairs: [2] }))
      const { container } = renderTV()
      const matched = container.querySelectorAll('.memotion-tv-memory-card.matched')
      expect(matched.length).toBe(2) // les 2 cartes de la paire 2
    })

    it('MEMOTION_ACTIVE.CARD_ID ne correspond pas à la carte active -> état vide, pas l\'ancien état', () => {
      const mock = makeMemotionMemoryMock('QUESTION', { flippedCards: ['1-1'] })
      mock.gameState.MEMOTION_ACTIVE.CARD_ID = 'card-999' // carte différente de MEMOTION_SELECTED ('card-3')
      useGame.mockReturnValue(mock)
      const { container } = renderTV()
      expect(container.querySelectorAll('.memotion-tv-memory-card.up').length).toBe(0)
    })
  })

  // Task 14 du plan #187 — la révélation ne peut pas être gatée sur
  // `gameState.phase === 'REVEALED'` (inatteignable en MEMOTION, la phase
  // reste STARTED toute la manche) : routée par `cardHostContext.revealed`
  // (dérivé de MEMOTION_SUBPHASE === 'REVEAL', utils/hostContext.js).
  describe('Face REVEAL — révélation totale, routée par hostContext.revealed (task 14)', () => {
    it('toutes les cartes sont affichées face visible en sous-phase REVEAL, même non trouvées', () => {
      useGame.mockReturnValue(makeMemotionMemoryMock('REVEAL'))
      const { container } = renderTV()
      const cards = container.querySelectorAll('.memotion-tv-memory-card')
      expect(cards.length).toBe(4)
      cards.forEach(card => expect(card.classList.contains('up')).toBe(true))
      expect(container.textContent).toContain('Napoléon')
      expect(container.textContent).toContain('1958')
    })

    it('la grille est conservée entre VERSO et REVEAL (pas de saut visuel, même carte)', () => {
      useGame.mockReturnValue(makeMemotionMemoryMock('REVEAL'))
      const { container } = renderTV()
      expect(container.querySelectorAll('.memotion-tv-memory-grid').length).toBe(1)
    })

    it('les cartes ne sont plus cliquables en REVEAL (playable=false)', () => {
      useGame.mockReturnValue(makeMemotionMemoryMock('REVEAL'))
      const { container } = renderTV()
      const cards = container.querySelectorAll('.memotion-tv-memory-card')
      cards.forEach(card => expect(card.disabled).toBe(true))
    })
  })

  describe('Non-régression — carte SPEEDY de la même manche', () => {
    it('une carte SPEEDY continue d\'afficher son propre contenu, pas de grille MEMORY', () => {
      const mock = makeMemotionMock('QUESTION', 'card-1', [CARD_WITH_IMG, CARD_MEMORY])
      useGame.mockReturnValue(mock)
      const { container } = renderTV()
      expect(container.querySelector('.memotion-tv-memory-grid')).toBeNull()
      expect(container.querySelector('.memotion-tv-fs-question-text').textContent)
        .toBe('Dans quel épisode Dark Vador révèle-t-il sa filiation ?')
    })
  })
})
