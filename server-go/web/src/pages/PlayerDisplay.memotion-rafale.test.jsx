import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, cleanup } from '@testing-library/react'
import PlayerDisplay from './PlayerDisplay'

// ---------------------------------------------------------------------------
// Retour QUALIF v9.0.0.4 (plan-v900-correctifs-qualif-20260906-104500.md
// §5 Lot C, §7 "dette de test #217") : "AUCUN test de rendu TV n'existe
// aujourd'hui pour une carte RAFALE — c'est pourquoi le chrono figé (C1)
// est passé en QUALIF sans être détecté." Ce fichier comble ce trou.
//
// Mocks calqués sur PlayerDisplay.memotion.test.jsx (patron déjà établi
// pour une carte MEMOTION en TV) — RAFALE_CURRENT_QUESTION/chips/chrono
// vivent dans MEMOTION_ACTIVE.STATE (getTypeState, contrat rafale.md §14.2),
// jamais dans les champs globaux RAFALE_* (réservés à la manche classique,
// #216) : le test central ci-dessous peuple délibérément les DEUX avec des
// valeurs distinctes pour prouver l'absence de confusion.
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

vi.mock('../components/Podium', () => ({ default: () => <div data-testid="podium" /> }))
vi.mock('../components/QRCodeOverlay', () => ({ default: () => null }))
vi.mock('../components/QRCodeDisplay', () => ({ default: () => null }))
vi.mock('./QuestionsPage', () => ({ CATEGORIES: [] }))
vi.mock('../constants/colors', () => ({ getCategoryColor: vi.fn(() => '#8b5cf6') }))
vi.mock('../utils/colorUtils', () => ({
  getRgbColor: vi.fn((color) => (Array.isArray(color) ? `rgb(${color.join(',')})` : color)),
}))
vi.mock('./PlayerDisplay.css', () => ({}))
vi.mock('../styles/neon.css', () => ({}))

import { useGame } from '../hooks/GameContext'

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

const RAFALE_CARD = {
  ID: 'mc-rafale-1',
  TYPE: 'RAFALE',
  RECTO_THEME: 'Manche éclair',
  RAFALE_CATEGORIES: ['HISTORY'],
  RAFALE_DIFFICULTIES: [2],
}

/**
 * @param {object} overrides.motionActiveState - MEMOTION_ACTIVE.STATE (carte)
 * @param {object} overrides.globalRafale - champs globaux RAFALE_* (manche classique, témoin de non-confusion)
 */
const makeRafaleCardMock = ({ motionActiveState = {}, globalRafale = {} } = {}) => ({
  gameState: {
    phase: 'STARTED',
    remote: 'GAME',
    timer: 30,
    totalTime: 60,
    question: { TYPE: 'MEMOTION', MOTION_CARDS: [RAFALE_CARD] },
    MEMOTION_SUBPHASE: 'QUESTION',
    MEMOTION_SELECTED: RAFALE_CARD.ID,
    MEMOTION_CARD_STATES: {},
    MEMOTION_CARD_TEAMS: {},
    MEMOTION_CURRENT_TEAM: 'Équipe A',
    MEMOTION_CURRENT_TEAM_COLOR: [99, 102, 241],
    MEMOTION_PARTICIPATING_TEAMS: ['Équipe A'],
    MEMORY_PARTICIPATING_TEAMS: [],
    MEMOTION_ACTIVE: {
      CARD_ID: RAFALE_CARD.ID,
      TYPE: 'RAFALE',
      STATE: {
        RAFALE_SUBPHASE: 'QUESTION',
        RAFALE_CURRENT_QUESTION: { ID: 'r-9', QUESTION: 'Quelle est la capitale de la Norvège ?', CATEGORY: 'HISTORY', DIFFICULTY: 2 },
        RAFALE_ASKED_COUNT: 1,
        RAFALE_CORRECT_COUNT: 0,
        ...motionActiveState,
      },
    },
    // Témoin de non-confusion : une manche RAFALE CLASSIQUE (hors carte)
    // pourrait avoir laissé ces champs globaux peuplés plus tôt dans la
    // même partie (#216) — un rendu de carte ne doit JAMAIS s'y alimenter.
    RAFALE_CURRENT_QUESTION: { ID: 'classic-1', QUESTION: 'CE TEXTE NE DOIT JAMAIS APPARAÎTRE', CATEGORY: 'SCIENCE', DIFFICULTY: 3 },
    RAFALE_QUESTION_TIME: 99,
    ...globalRafale,
    newGameBackgrounds: [],
  },
  teams: { 'Équipe A': { SCORE: 10, COLOR: [99, 102, 241] } },
  bumpers: {},
  flipMemoryCard: vi.fn(),
  showQRCode: false,
  selectMotionCard: vi.fn(),
})

const renderTV = (overrides) => {
  useGame.mockReturnValue(makeRafaleCardMock(overrides))
  return render(<PlayerDisplay />)
}

describe('PlayerDisplay TV — carte RAFALE en MEMOTION+ (#217, comble le trou de couverture révélé par C1)', () => {
  it('affiche la question TIRÉE de la carte (MEMOTION_ACTIVE.STATE), jamais un résidu de manche classique', () => {
    const { container } = renderTV()
    expect(container.textContent).toContain('Quelle est la capitale de la Norvège ?')
    expect(container.textContent).not.toContain('CE TEXTE NE DOIT JAMAIS APPARAÎTRE')
  })

  it('⚠️ C1 (chrono figé) : le décompte de la carte suit MEMOTION_ACTIVE.STATE.RAFALE_QUESTION_TIME, PAS le champ global RAFALE_QUESTION_TIME', () => {
    const { container, rerender } = renderTV({
      motionActiveState: { RAFALE_QUESTION_TIME: 3 },
      globalRafale: { RAFALE_QUESTION_TIME: 99 }, // sentinelle globale distincte, ne doit jamais apparaître
    })
    expect(container.textContent).toContain('3')
    expect(container.textContent).not.toContain('99')

    // Un tick réel change UNIQUEMENT la valeur scopée carte — le rendu doit
    // suivre, exactement le symptôme "chrono figé" que ce fichier existe
    // pour détecter (avant #217, aucun test ne rendait ce chemin du tout).
    useGame.mockReturnValue(makeRafaleCardMock({
      motionActiveState: { RAFALE_QUESTION_TIME: 2 },
      globalRafale: { RAFALE_QUESTION_TIME: 99 },
    }))
    rerender(<PlayerDisplay />)
    expect(container.textContent).toContain('2')
  })

  it('chips catégorie et difficulté de la carte affichés (contrat rafale.md §14.2)', () => {
    const { container } = renderTV()
    expect(container.textContent).toMatch(/HISTOIRE|HISTORY/i)
    expect(container.textContent).toContain('★★')
  })

  it('aucun défilement introduit (contrainte TV statique, CLAUDE.md)', () => {
    const { container } = renderTV()
    const overlay = container.querySelector('.memotion-tv-fullscreen')
    expect(overlay).not.toBeNull()
    const all = [overlay, ...overlay.querySelectorAll('*')]
    for (const el of all) {
      const overflow = el.style?.overflow
      expect(overflow).not.toBe('auto')
      expect(overflow).not.toBe('scroll')
    }
  })
})
