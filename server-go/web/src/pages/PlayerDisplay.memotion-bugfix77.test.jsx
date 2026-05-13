import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render } from '@testing-library/react'
import PlayerDisplay from './PlayerDisplay'

// ---------------------------------------------------------------------------
// Mocks — identiques à PlayerDisplay.memotion.test.jsx
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

/** Carte complète : image recto + question texte+image + réponse texte+image */
const CARD_FULL = {
  ID: 'card-1',
  RECTO_THEME: 'Thème Star Wars',
  RECTO_IMAGE: '/files/card1.jpg',
  DIFFICULTY: 2,
  QUESTION_TEXT: 'Dans quel épisode Dark Vador révèle-t-il sa filiation ?',
  QUESTION_IMAGE: '/files/q1.jpg',
  ANSWER_TEXT: "L'Empire contre-attaque (Épisode V)",
  ANSWER_IMAGE: '/files/a1.jpg',
}

/** Carte sans image recto — texte question + réponse texte seule */
const CARD_NO_RECTO_IMG = {
  ID: 'card-2',
  RECTO_THEME: 'Thème Littérature',
  DIFFICULTY: 1,
  QUESTION_TEXT: "Qui a écrit Les Misérables ?",
  ANSWER_TEXT: 'Victor Hugo',
}

/** Carte sans texte question — image question + réponse mixte */
const CARD_NO_QUESTION_TEXT = {
  ID: 'card-4',
  RECTO_THEME: 'Thème Art',
  RECTO_IMAGE: '/files/card4.jpg',
  DIFFICULTY: 1,
  QUESTION_IMAGE: '/files/q4.jpg',
  ANSWER_TEXT: 'La Joconde',
}

/**
 * Construit un mock useGame pour une question MEMOTION.
 *
 * @param {string}      subphase           - 'GRID' | 'SELECTED' | 'QUESTION' | 'REVEAL'
 * @param {string|null} selectedId         - ID de la carte sélectionnée (overlay fullscreen)
 * @param {object[]}    cards              - tableau de MotionCards
 * @param {object}      cardStatesOverride - ex: { 'card-1': 'DONE' }
 * @param {object}      cardTeamsOverride  - ex: { 'card-1': 'Équipe A' }
 */
const makeMemotionMock = (
  subphase = 'GRID',
  selectedId = null,
  cards = [CARD_FULL, CARD_NO_RECTO_IMG],
  cardStatesOverride = {},
  cardTeamsOverride = {},
) => ({
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
    MEMOTION_CARD_STATES: cardStatesOverride,
    MEMOTION_CARD_TEAMS: cardTeamsOverride,
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
// Helper render
// ---------------------------------------------------------------------------
const renderTV = () => render(<PlayerDisplay />)

// ---------------------------------------------------------------------------
// TESTS — États visuels MEMOTION (bugfix #77)
// Complètent PlayerDisplay.memotion.test.jsx (SC1–SC8) sans les modifier.
// Ces tests documentent les comportements APRÈS le fix complet.
// ---------------------------------------------------------------------------

describe('PlayerDisplay — MEMOTION états visuels (bugfix #77)', () => {

  beforeEach(() => {
    vi.clearAllMocks()
    Object.defineProperty(document.documentElement, 'requestFullscreen', {
      value: vi.fn().mockResolvedValue(undefined),
      writable: true,
      configurable: true,
    })
  })

  // =========================================================================
  // A — Carte UNPLAYED dans la grille : éléments additionnels
  //     Complète SC1/SC7 de PlayerDisplay.memotion.test.jsx
  // =========================================================================

  describe('A — UNPLAYED (grille) : structure 1/6-4/6-1/6 additionnelle', () => {

    beforeEach(() => {
      useGame.mockReturnValue(makeMemotionMock('GRID'))
    })

    it('PAS de .memory-card-letter sur les cartes MEMOTION (exclusif dos MEMORY)', () => {
      // .memory-card-letter est la classe des cartes MEMORY (dos avec lettre A/B/C...).
      // Son apparition sur .memotion-card indiquerait une régression CSS de scoping.
      const { container } = renderTV()
      expect(container.querySelector('.memotion-card .memory-card-letter')).toBeNull()
    })

    it('PAS de .memory-card-image sur les cartes MEMOTION (exclusif face avant MEMORY)', () => {
      const { container } = renderTV()
      expect(container.querySelector('.memotion-card .memory-card-image')).toBeNull()
    })

    it('footer des cartes UNPLAYED ne contient PAS de .memotion-card-done-team', () => {
      const { container } = renderTV()
      expect(container.querySelector('.memotion-card-done-team')).toBeNull()
    })

    it('body des cartes UNPLAYED vide quand RECTO_IMAGE absent (card-2)', () => {
      const { container } = renderTV()
      const cardBacks = container.querySelectorAll('.memotion-card .memory-card-back')
      // card-2 (CARD_NO_RECTO_IMG) est à l'index 1
      expect(cardBacks[1].querySelector('.memotion-card-body img')).toBeNull()
    })
  })

  // =========================================================================
  // B — Carte SELECTED dans la grille : classe selected + pas de matched
  // =========================================================================

  describe('B — SELECTED (grille) : classe selected + pas de matched', () => {

    beforeEach(() => {
      useGame.mockReturnValue(makeMemotionMock('SELECTED', 'card-1'))
    })

    it('la carte selectedId (card-1) a la classe selected', () => {
      const { container } = renderTV()
      const cards = container.querySelectorAll('.memotion-card')
      expect(cards[0].classList.contains('selected')).toBe(true)
    })

    it('la carte selectedId (card-1) n\'a PAS la classe matched (SELECTED ≠ DONE)', () => {
      const { container } = renderTV()
      const cards = container.querySelectorAll('.memotion-card')
      expect(cards[0].classList.contains('matched')).toBe(false)
    })

    it('les cartes non-sélectionnées n\'ont PAS la classe selected', () => {
      const { container } = renderTV()
      const cards = container.querySelectorAll('.memotion-card')
      // card-2 (index 1) n'est pas le selectedId
      expect(cards[1].classList.contains('selected')).toBe(false)
    })

    it('PAS de .memotion-card-done-team dans la grille (SELECTED n\'est pas DONE)', () => {
      const { container } = renderTV()
      expect(container.querySelector('.memotion-card-done-team')).toBeNull()
    })
  })

  // =========================================================================
  // C — Overlay SELECTED fullscreen (bugfix #77) : 3 rows, PAS de pts/équipe
  //     SELECTED est partiellement corrigé dans le code ; ces tests valident
  //     le layout complet post-fix.
  // =========================================================================

  describe('C — Overlay SELECTED fullscreen : layout RECTO 3 rows (bugfix #77)', () => {

    describe('C1 — avec RECTO_IMAGE (card-1)', () => {

      beforeEach(() => {
        useGame.mockReturnValue(makeMemotionMock('SELECTED', 'card-1'))
      })

      it('le fullscreen a exactement 3 enfants directs (header + body + footer)', () => {
        // Régression : avant fix → 2 enfants (header + body). Après fix → 3 (+footer étoiles).
        const { container } = renderTV()
        const overlay = container.querySelector('.memotion-tv-fullscreen')
        expect(overlay).not.toBeNull()
        expect(overlay.children.length).toBe(3)
      })

      it('row1 (header) a la classe memotion-tv-fs-recto-zone', () => {
        const { container } = renderTV()
        const header = container.querySelector('.memotion-tv-fs-header')
        expect(header.classList.contains('memotion-tv-fs-recto-zone')).toBe(true)
      })

      it('row1 (header) contient .memotion-tv-fs-theme avec le RECTO_THEME', () => {
        const { container } = renderTV()
        const header = container.querySelector('.memotion-tv-fs-header')
        const theme = header.querySelector('.memotion-tv-fs-theme')
        expect(theme).not.toBeNull()
        expect(theme.textContent).toBe('Thème Star Wars')
      })

      it('row1 (header) NE contient PAS .memotion-tv-fs-pts (éliminé bugfix #77)', () => {
        // Avant fix : header contenait les points (ex: "3pt"). Après fix : absent.
        const { container } = renderTV()
        expect(container.querySelector('.memotion-tv-fs-header .memotion-tv-fs-pts')).toBeNull()
      })

      it('row1 (header) NE contient PAS .memotion-tv-fs-team (éliminé bugfix #77)', () => {
        // Avant fix : le nom de l'équipe courante apparaissait dans le header.
        const { container } = renderTV()
        expect(container.querySelector('.memotion-tv-fs-header .memotion-tv-fs-team')).toBeNull()
      })

      it('row1 (header) NE contient PAS .memotion-tv-fs-diff (étoiles déplacées en row3)', () => {
        // Avant fix : étoiles dans le header. Après fix : déplacées dans le footer.
        const { container } = renderTV()
        expect(container.querySelector('.memotion-tv-fs-header .memotion-tv-fs-diff')).toBeNull()
      })

      it('row2 (body) contient .memotion-tv-fs-img avec src=RECTO_IMAGE', () => {
        const { container } = renderTV()
        const overlay = container.querySelector('.memotion-tv-fullscreen')
        const row2 = overlay.children[1]
        expect(row2.classList.contains('memotion-tv-fs-body')).toBe(true)
        const img = row2.querySelector('img.memotion-tv-fs-img')
        expect(img).not.toBeNull()
        expect(img.getAttribute('src')).toBe('/files/card1.jpg')
      })

      it('row3 (footer) a la classe memotion-tv-fs-recto-zone', () => {
        const { container } = renderTV()
        const overlay = container.querySelector('.memotion-tv-fullscreen')
        const row3 = overlay.children[2]
        expect(row3.classList.contains('memotion-tv-fs-footer')).toBe(true)
        expect(row3.classList.contains('memotion-tv-fs-recto-zone')).toBe(true)
      })

      it('row3 (footer) contient .memotion-tv-fs-diff avec les étoiles (DIFFICULTY 2)', () => {
        // Après fix : les étoiles de difficulté sont dans le footer (row3), pas dans le header.
        const { container } = renderTV()
        const footer = container.querySelector('.memotion-tv-fs-footer')
        expect(footer).not.toBeNull()
        const diff = footer.querySelector('.memotion-tv-fs-diff')
        expect(diff).not.toBeNull()
        expect(diff.textContent).toBe('★★')
      })

      it('.memotion-tv-fs-diff est dans le footer, PAS dans le header', () => {
        const { container } = renderTV()
        expect(container.querySelector('.memotion-tv-fs-header .memotion-tv-fs-diff')).toBeNull()
        expect(container.querySelector('.memotion-tv-fs-footer .memotion-tv-fs-diff')).not.toBeNull()
      })
    })

    describe('C2 — sans RECTO_IMAGE (card-2)', () => {

      beforeEach(() => {
        useGame.mockReturnValue(makeMemotionMock('SELECTED', 'card-2'))
      })

      it('row2 (body) affiche .memotion-tv-fs-text avec RECTO_THEME quand pas d\'image', () => {
        const { container } = renderTV()
        const body = container.querySelector('.memotion-tv-fs-body')
        expect(body.querySelector('img')).toBeNull()
        const text = body.querySelector('.memotion-tv-fs-text')
        expect(text).not.toBeNull()
        expect(text.textContent).toBe('Thème Littérature')
      })

      it('row3 (footer) contient quand même les étoiles (DIFFICULTY 1)', () => {
        const { container } = renderTV()
        const footer = container.querySelector('.memotion-tv-fs-footer')
        const diff = footer.querySelector('.memotion-tv-fs-diff')
        expect(diff).not.toBeNull()
        expect(diff.textContent).toBe('★')
      })
    })
  })

  // =========================================================================
  // D — Overlay QUESTION (bugfix #77) : QUESTION_TEXT en row1, 3 rows
  //     Comportement ATTENDU après le fix (tests failleront avant le fix)
  // =========================================================================

  describe('D — Overlay QUESTION : QUESTION_TEXT en row1 (bugfix #77)', () => {

    describe('D1 — avec QUESTION_IMAGE et QUESTION_TEXT (card-1)', () => {

      beforeEach(() => {
        useGame.mockReturnValue(makeMemotionMock('QUESTION', 'card-1'))
      })

      it('le fullscreen a exactement 3 enfants directs (header + body + footer vide)', () => {
        // Avant fix → 2 enfants (header + body). Après fix → 3 (+ footer vide).
        const { container } = renderTV()
        const overlay = container.querySelector('.memotion-tv-fullscreen')
        expect(overlay).not.toBeNull()
        expect(overlay.children.length).toBe(3)
      })

      it('row1 (header) contient .memotion-tv-fs-question-text avec QUESTION_TEXT', () => {
        // Après fix : QUESTION_TEXT est en row1 via .memotion-tv-fs-question-text.
        // Avant fix : row1 affichait RECTO_THEME + étoiles + pts + équipe.
        const { container } = renderTV()
        const header = container.querySelector('.memotion-tv-fs-header')
        const qText = header.querySelector('.memotion-tv-fs-question-text')
        expect(qText).not.toBeNull()
        expect(qText.textContent).toBe('Dans quel épisode Dark Vador révèle-t-il sa filiation ?')
      })

      it('row1 (header) NE contient PAS .memotion-tv-fs-theme (RECTO_THEME éliminé)', () => {
        // Avant fix : header affichait RECTO_THEME. Après fix : seul QUESTION_TEXT.
        const { container } = renderTV()
        expect(container.querySelector('.memotion-tv-fs-header .memotion-tv-fs-theme')).toBeNull()
      })

      it('row1 (header) NE contient PAS .memotion-tv-fs-pts (éliminé bugfix #77)', () => {
        const { container } = renderTV()
        expect(container.querySelector('.memotion-tv-fs-header .memotion-tv-fs-pts')).toBeNull()
      })

      it('row1 (header) NE contient PAS .memotion-tv-fs-team (éliminé bugfix #77)', () => {
        const { container } = renderTV()
        expect(container.querySelector('.memotion-tv-fs-header .memotion-tv-fs-team')).toBeNull()
      })

      it('row1 (header) NE contient PAS .memotion-tv-fs-diff (éliminé bugfix #77)', () => {
        const { container } = renderTV()
        expect(container.querySelector('.memotion-tv-fs-header .memotion-tv-fs-diff')).toBeNull()
      })

      it('row2 (body) contient QUESTION_IMAGE via .memotion-tv-fs-img', () => {
        const { container } = renderTV()
        const body = container.querySelector('.memotion-tv-fs-body')
        const img = body.querySelector('img.memotion-tv-fs-img')
        expect(img).not.toBeNull()
        expect(img.getAttribute('src')).toBe('/files/q1.jpg')
      })

      it('row2 (body) NE contient PAS .memotion-tv-fs-text (QUESTION_TEXT déplacé en row1)', () => {
        // Avant fix : QUESTION_TEXT était dans le body. Après fix : dans le header.
        const { container } = renderTV()
        const body = container.querySelector('.memotion-tv-fs-body')
        expect(body.querySelector('.memotion-tv-fs-text')).toBeNull()
      })

      it('row3 (footer) est présent et vide (zone libre)', () => {
        // Après fix : footer vide ajouté pour respecter la grille CSS 1fr 4fr 1fr.
        const { container } = renderTV()
        const overlay = container.querySelector('.memotion-tv-fullscreen')
        const row3 = overlay.children[2]
        expect(row3.classList.contains('memotion-tv-fs-footer')).toBe(true)
        expect(row3.textContent.trim()).toBe('')
      })
    })

    describe('D2 — QUESTION sans image : body vide', () => {

      beforeEach(() => {
        // card-2 a QUESTION_TEXT mais pas QUESTION_IMAGE
        useGame.mockReturnValue(makeMemotionMock('QUESTION', 'card-2'))
      })

      it('row1 (header) contient QUESTION_TEXT', () => {
        const { container } = renderTV()
        const header = container.querySelector('.memotion-tv-fs-header')
        const qText = header.querySelector('.memotion-tv-fs-question-text')
        expect(qText).not.toBeNull()
        expect(qText.textContent).toBe("Qui a écrit Les Misérables ?")
      })

      it('row2 (body) est vide — pas d\'image (QUESTION_IMAGE absent)', () => {
        const { container } = renderTV()
        const body = container.querySelector('.memotion-tv-fs-body')
        expect(body.querySelector('img')).toBeNull()
      })

      it('row3 (footer) est présent et vide', () => {
        const { container } = renderTV()
        const overlay = container.querySelector('.memotion-tv-fullscreen')
        expect(overlay.children[2].textContent.trim()).toBe('')
      })
    })

    describe('D3 — QUESTION sans texte : header vide, body=image', () => {

      beforeEach(() => {
        // CARD_NO_QUESTION_TEXT a QUESTION_IMAGE mais pas QUESTION_TEXT
        useGame.mockReturnValue(
          makeMemotionMock('QUESTION', 'card-4', [CARD_FULL, CARD_NO_QUESTION_TEXT])
        )
      })

      it('row1 (header) ne contient PAS .memotion-tv-fs-question-text quand QUESTION_TEXT absent', () => {
        const { container } = renderTV()
        expect(container.querySelector('.memotion-tv-fs-header .memotion-tv-fs-question-text')).toBeNull()
      })

      it('row2 (body) contient QUESTION_IMAGE', () => {
        const { container } = renderTV()
        const body = container.querySelector('.memotion-tv-fs-body')
        const img = body.querySelector('img.memotion-tv-fs-img')
        expect(img).not.toBeNull()
        expect(img.getAttribute('src')).toBe('/files/q4.jpg')
      })
    })
  })

  // =========================================================================
  // G — Overlay REVEAL (bugfix #77) : QUESTION_TEXT rappel en row1 (classe recall)
  //     Comportement ATTENDU après le fix (tests failleront avant le fix)
  // =========================================================================

  describe('G — Overlay REVEAL : QUESTION_TEXT rappel en row1 (bugfix #77)', () => {

    describe('G1 — avec ANSWER_IMAGE et ANSWER_TEXT (card-1)', () => {

      beforeEach(() => {
        useGame.mockReturnValue(makeMemotionMock('REVEAL', 'card-1'))
      })

      it('le fullscreen a exactement 3 enfants directs (header + body + footer TOUJOURS)', () => {
        // Après fix : footer toujours présent (structure CSS 1fr 4fr 1fr garantie).
        const { container } = renderTV()
        const overlay = container.querySelector('.memotion-tv-fullscreen')
        expect(overlay).not.toBeNull()
        expect(overlay.children.length).toBe(3)
      })

      it('row1 (header) contient .memotion-tv-fs-recall avec QUESTION_TEXT', () => {
        // Après fix : header affiche QUESTION_TEXT avec classe recall (opacity réduite).
        // Avant fix : header affichait RECTO_THEME + étoiles + pts.
        const { container } = renderTV()
        const header = container.querySelector('.memotion-tv-fs-header')
        const recall = header.querySelector('.memotion-tv-fs-recall')
        expect(recall).not.toBeNull()
        expect(recall.textContent).toBe('Dans quel épisode Dark Vador révèle-t-il sa filiation ?')
      })

      it('l\'élément recall porte aussi la classe .memotion-tv-fs-question-text', () => {
        const { container } = renderTV()
        const qText = container.querySelector('.memotion-tv-fs-header .memotion-tv-fs-question-text')
        expect(qText).not.toBeNull()
        expect(qText.classList.contains('memotion-tv-fs-recall')).toBe(true)
      })

      it('row1 (header) a la classe memotion-tv-fs-recto-zone', () => {
        const { container } = renderTV()
        expect(container.querySelector('.memotion-tv-fs-header').classList.contains('memotion-tv-fs-recto-zone')).toBe(true)
      })

      it('row1 (header) NE contient PAS .memotion-tv-fs-theme (RECTO_THEME éliminé bugfix #77)', () => {
        // Avant fix : header affichait RECTO_THEME. Après fix : seul QUESTION_TEXT rappel.
        const { container } = renderTV()
        expect(container.querySelector('.memotion-tv-fs-header .memotion-tv-fs-theme')).toBeNull()
      })

      it('row1 (header) NE contient PAS .memotion-tv-fs-pts (éliminé bugfix #77)', () => {
        const { container } = renderTV()
        expect(container.querySelector('.memotion-tv-fs-header .memotion-tv-fs-pts')).toBeNull()
      })

      it('row1 (header) NE contient PAS .memotion-tv-fs-diff (éliminé bugfix #77)', () => {
        const { container } = renderTV()
        expect(container.querySelector('.memotion-tv-fs-header .memotion-tv-fs-diff')).toBeNull()
      })

      it('row2 (body) contient ANSWER_IMAGE via .memotion-tv-fs-img', () => {
        const { container } = renderTV()
        const body = container.querySelector('.memotion-tv-fs-body')
        const img = body.querySelector('img.memotion-tv-fs-img')
        expect(img).not.toBeNull()
        expect(img.getAttribute('src')).toBe('/files/a1.jpg')
      })

      it('row3 (footer) contient ANSWER_TEXT quand image + texte coexistent', () => {
        // card-1 a ANSWER_IMAGE && ANSWER_TEXT → footer affiche ANSWER_TEXT.
        const { container } = renderTV()
        const overlay = container.querySelector('.memotion-tv-fullscreen')
        const footer = overlay.children[2]
        expect(footer.classList.contains('memotion-tv-fs-footer')).toBe(true)
        expect(footer.textContent).toBe("L'Empire contre-attaque (Épisode V)")
      })
    })

    describe('G2 — REVEAL sans image : ANSWER_TEXT dans body (fallback)', () => {

      beforeEach(() => {
        // card-2 a ANSWER_TEXT mais pas ANSWER_IMAGE
        useGame.mockReturnValue(makeMemotionMock('REVEAL', 'card-2'))
      })

      it('row1 (header) contient .memotion-tv-fs-recall avec QUESTION_TEXT', () => {
        const { container } = renderTV()
        const recall = container.querySelector('.memotion-tv-fs-header .memotion-tv-fs-recall')
        expect(recall).not.toBeNull()
        expect(recall.textContent).toBe("Qui a écrit Les Misérables ?")
      })

      it('row2 (body) contient .memotion-tv-answer-text avec ANSWER_TEXT', () => {
        // Fallback : quand pas d'image réponse, le texte est affiché dans le body.
        const { container } = renderTV()
        const body = container.querySelector('.memotion-tv-fs-body')
        const answerText = body.querySelector('.memotion-tv-answer-text')
        expect(answerText).not.toBeNull()
        expect(answerText.textContent).toBe('Victor Hugo')
      })

      it('row3 (footer) est présent et vide (pas de combinaison image+texte)', () => {
        // Après fix : footer toujours présent, vide si pas de combinaison image+texte.
        const { container } = renderTV()
        const overlay = container.querySelector('.memotion-tv-fullscreen')
        const row3 = overlay.children[2]
        expect(row3.classList.contains('memotion-tv-fs-footer')).toBe(true)
        expect(row3.textContent.trim()).toBe('')
      })

      it('le fullscreen a 3 enfants directs (footer vide compté)', () => {
        const { container } = renderTV()
        const overlay = container.querySelector('.memotion-tv-fullscreen')
        expect(overlay.children.length).toBe(3)
      })
    })
  })

  // =========================================================================
  // I — Carte DONE : classe matched, fond équipe gagnante, footer avec nom
  // =========================================================================

  describe('I — Carte DONE : fond équipe + footer avec nom équipe', () => {

    beforeEach(() => {
      useGame.mockReturnValue(
        makeMemotionMock(
          'GRID',
          null,
          [CARD_FULL, CARD_NO_RECTO_IMG],
          { 'card-1': 'DONE' },
          { 'card-1': 'Équipe A' },
        )
      )
    })

    it('la carte DONE (card-1) a la classe matched', () => {
      // La classe matched conditionne le CSS .memotion-card.matched .memory-card-back
      // (fond couleur équipe). C'est l'indicateur DOM de l'état DONE.
      const { container } = renderTV()
      const cards = container.querySelectorAll('.memotion-card')
      expect(cards[0].classList.contains('matched')).toBe(true)
    })

    it('la carte DONE a --matched-team-color dans son style inline', () => {
      // La CSS variable est injectée pour coloriser le fond via CSS.
      const { container } = renderTV()
      const card = container.querySelector('.memotion-card.matched')
      const style = card.getAttribute('style') || ''
      expect(style).toContain('--matched-team-color')
    })

    it('la carte non-DONE (card-2) n\'a PAS la classe matched', () => {
      const { container } = renderTV()
      const cards = container.querySelectorAll('.memotion-card')
      expect(cards[1].classList.contains('matched')).toBe(false)
    })

    it('footer de la carte DONE contient .memotion-card-done-team avec le nom de l\'équipe', () => {
      const { container } = renderTV()
      const doneTeam = container.querySelector('.memotion-card.matched .memotion-card-done-team')
      expect(doneTeam).not.toBeNull()
      expect(doneTeam.textContent).toBe('Équipe A')
    })

    it('footer de la carte DONE contient .memotion-card-stars', () => {
      const { container } = renderTV()
      const stars = container.querySelector('.memotion-card.matched .memotion-card-stars')
      expect(stars).not.toBeNull()
      expect(stars.textContent).toBe('★★') // DIFFICULTY 2
    })

    it('footer de la carte DONE a exactement 2 enfants (stars + done-team)', () => {
      const { container } = renderTV()
      const footer = container.querySelector('.memotion-card.matched .memotion-card-footer')
      expect(footer.children.length).toBe(2)
    })

    it('PAS de .memotion-card-pts dans le footer DONE', () => {
      const { container } = renderTV()
      const footer = container.querySelector('.memotion-card.matched .memotion-card-footer')
      expect(footer.querySelector('.memotion-card-pts')).toBeNull()
    })

    it('PAS d\'overlay fullscreen quand DONE (état dans grille seulement)', () => {
      const { container } = renderTV()
      expect(container.querySelector('.memotion-tv-fullscreen')).toBeNull()
    })
  })

  // =========================================================================
  // J — Carte DONE sans winnerTeam : fond neutre, pas de nom d'équipe
  // =========================================================================

  describe('J — Carte DONE sans winnerTeam : fond neutre, pas de nom équipe', () => {

    beforeEach(() => {
      useGame.mockReturnValue(
        makeMemotionMock(
          'GRID',
          null,
          [CARD_FULL, CARD_NO_RECTO_IMG],
          { 'card-1': 'DONE' },
          {},  // aucun winner pour card-1
        )
      )
    })

    it('la carte DONE (card-1) a quand même la classe matched', () => {
      const { container } = renderTV()
      expect(container.querySelector('.memotion-card').classList.contains('matched')).toBe(true)
    })

    it('PAS de .memotion-card-done-team dans le footer (winnerTeam null)', () => {
      // Sans équipe gagnante assignée, le span done-team ne doit pas apparaître.
      const { container } = renderTV()
      expect(container.querySelector('.memotion-card-done-team')).toBeNull()
    })

    it('footer contient uniquement .memotion-card-stars (1 seul enfant)', () => {
      const { container } = renderTV()
      const footer = container.querySelector('.memotion-card.matched .memotion-card-footer')
      expect(footer).not.toBeNull()
      expect(footer.querySelector('.memotion-card-stars')).not.toBeNull()
      expect(footer.children.length).toBe(1)
    })

    it('footer textContent = uniquement des étoiles (pas de texte équipe)', () => {
      const { container } = renderTV()
      const footer = container.querySelector('.memotion-card.matched .memotion-card-footer')
      expect(footer.textContent).toBe('★★')
    })
  })

  // =========================================================================
  // K — Non-régression MEMORY : éléments MEMOTION non propagés aux cartes MEMORY
  // =========================================================================

  describe('K — Non-régression MEMORY : MEMOTION scoping sans impact MEMORY', () => {

    it('les cartes MEMOTION n\'ont PAS .memory-card-letter (classe exclusive MEMORY)', () => {
      useGame.mockReturnValue(makeMemotionMock('GRID'))
      const { container } = renderTV()
      expect(container.querySelector('.memotion-card .memory-card-letter')).toBeNull()
    })

    it('les cartes MEMOTION n\'ont PAS .memory-card-text (classe exclusive MEMORY)', () => {
      useGame.mockReturnValue(makeMemotionMock('GRID'))
      const { container } = renderTV()
      expect(container.querySelector('.memotion-card .memory-card-text')).toBeNull()
    })

    it('les cartes MEMOTION back ont les classes MEMOTION mais pas les classes MEMORY internes', () => {
      useGame.mockReturnValue(makeMemotionMock('GRID'))
      const { container } = renderTV()
      const cardBack = container.querySelector('.memotion-card .memory-card-back')
      // Structure MEMOTION présente
      expect(cardBack.querySelector('.memotion-card-header')).not.toBeNull()
      expect(cardBack.querySelector('.memotion-card-body')).not.toBeNull()
      expect(cardBack.querySelector('.memotion-card-footer')).not.toBeNull()
      // Pas de classes MEMORY internes
      expect(cardBack.querySelector('.memory-card-letter')).toBeNull()
    })

    it('pas d\'overlay MEMOTION quand TYPE=MEMORY', () => {
      // Les overlays MEMOTION (.memotion-tv-fullscreen) ne doivent JAMAIS
      // s'afficher pour un jeu MEMORY.
      useGame.mockReturnValue({
        gameState: {
          phase: 'STARTED',
          remote: 'GAME',
          timer: 15,
          totalTime: 30,
          question: {
            TYPE: 'MEMORY',
            MEMORY_PAIRS: [],
            MEMORY_CONFIG: { COLS: 4, ROWS: 4 },
          },
          memoryMatchedPairs: [],
          MEMORY_PARTICIPATING_TEAMS: ['Équipe A'],
          newGameBackgrounds: [],
        },
        teams: { 'Équipe A': { SCORE: 0, COLOR: [99, 102, 241] } },
        bumpers: {},
        flipMemoryCard: vi.fn(),
        showQRCode: false,
        selectMotionCard: vi.fn(),
      })
      const { container } = renderTV()
      expect(container.querySelector('.memotion-tv-fullscreen')).toBeNull()
    })

    it('pas de .memotion-card ni .memotion-game quand TYPE=MEMORY', () => {
      useGame.mockReturnValue({
        gameState: {
          phase: 'STARTED',
          remote: 'GAME',
          timer: 15,
          totalTime: 30,
          question: {
            TYPE: 'MEMORY',
            MEMORY_PAIRS: [],
            MEMORY_CONFIG: { COLS: 4, ROWS: 4 },
          },
          memoryMatchedPairs: [],
          MEMORY_PARTICIPATING_TEAMS: ['Équipe A'],
          newGameBackgrounds: [],
        },
        teams: { 'Équipe A': { SCORE: 0, COLOR: [99, 102, 241] } },
        bumpers: {},
        flipMemoryCard: vi.fn(),
        showQRCode: false,
        selectMotionCard: vi.fn(),
      })
      const { container } = renderTV()
      expect(container.querySelector('.memotion-card')).toBeNull()
      expect(container.querySelector('.memotion-game')).toBeNull()
    })
  })

})
