/**
 * Tests for PlayerDisplay — dispatch positif du type de contenu (#183, A-T1
 * — filet de non-régression du refactor A-F1).
 *
 * A-F1 remplace, à 3 endroits (`:1438`, `:1502`, `:2413` avant refactor), la
 * garde par négation `!isQcm && !isMemory && !isMemotion` par un dispatch
 * positif (`isSpeedy || isArdoise`). Ce fichier vérifie :
 *
 *   1. Équivalence stricte pour les 5 types connus — le bloc de mise en page
 *      par défaut (SPEEDY/ARDOISE) s'affiche/ne s'affiche pas exactement
 *      comme avant, sur les 4 phases où la garde s'applique (COUNTDOWN,
 *      READY, STARTED, REVEALED côté TV).
 *   2. Le SEUL changement de comportement voulu par #183 : un TYPE renseigné
 *      mais inconnu des 5 types gérés ne retombe plus silencieusement sur la
 *      mise en page SPEEDY (avant A-F1, la négation le laissait passer).
 *   3. Plus aucune garde exprimée en liste de négations dans le source
 *      (critère d'acceptance #183) — scan de source, même famille de test que
 *      AnimMotionActions.test.jsx (fs.readFileSync + assertion d'absence).
 *
 * Note méthodologique : les 3 sites de garde partagent la classe CSS bare
 * "game-content-zones" avec le bloc QCM (`:1538`, positif, hors scope A-F1,
 * non touché) — mais PAS avec les blocs MEMORY/MEMOTION, qui portent une
 * classe composée ("game-content-zones memory-game[...]"). Compter les
 * éléments à classe EXACTEMENT "game-content-zones" est donc un signal fiable
 * pour distinguer "un bloc générique/QCM s'affiche" (1) de "rien ne
 * s'affiche" (0) — sans avoir à réimplémenter la logique de rendu ici.
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render } from '@testing-library/react'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import PlayerDisplay from './PlayerDisplay'

// ---------------------------------------------------------------------------
// Mocks — PlayerDisplay imports (pattern identique à PlayerDisplay.memotion.test.jsx
// et PlayerDisplay.palmares.test.jsx)
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

vi.mock('../hooks/useCategories', () => ({
  useCategories: vi.fn(),
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
// Import après les mocks
// ---------------------------------------------------------------------------
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

// ---------------------------------------------------------------------------
// Fixtures — un gameState minimal par (type, phase), sans crash pour aucun
// des 5 types connus ni pour un type inconnu.
// ---------------------------------------------------------------------------

/** Construit gameState.question pour un TYPE donné (null si type est null). */
function questionFor(type) {
  if (!type) return null
  const base = {
    TYPE: type,
    CATEGORY: 'CULTURE',
    QUESTION: 'Question générique ?',
    ANSWER: 'Réponse générique',
    MEDIA: null,
  }
  if (type === 'MEMORY') return { ...base, MEMORY_PAIRS: [], MEMORY_CONFIG: {} }
  if (type === 'MEMOTION') return { ...base, MOTION_CARDS: [], MOTION_CONFIG: {} }
  return base
}

/** Construit un gameState complet pour (phase, type). */
function makeGameState(phase, type, overrides = {}) {
  return {
    phase,
    remote: 'GAME',
    timer: 10,
    totalTime: 30,
    countdownTime: 3,
    question: questionFor(type),
    newGameBackgrounds: [],
    MEMORY_PARTICIPATING_TEAMS: [],
    MEMOTION_PARTICIPATING_TEAMS: [],
    MEMOTION_SUBPHASE: null,
    MEMOTION_CARD_STATES: {},
    MEMOTION_CARD_TEAMS: {},
    MEMOTION_SELECTED: null,
    MEMOTION_CURRENT_TEAM: '',
    MEMOTION_CURRENT_TEAM_COLOR: null,
    ...overrides,
  }
}

function mockUseGame(gameState) {
  useGame.mockReturnValue({
    gameState,
    teams: {},
    bumpers: {},
    flipMemoryCard: vi.fn(),
    showQRCode: false,
    selectMotionCard: vi.fn(),
  })
}

/** render PlayerDisplay en mode TV (isVPlayer=false, défaut du composant). */
function renderTV(phase, type, overrides = {}) {
  mockUseGame(makeGameState(phase, type, overrides))
  return render(<PlayerDisplay />)
}

/**
 * Éléments dont la classe est EXACTEMENT "game-content-zones" (bare) — exclut
 * les blocs MEMORY/MEMOTION dont la classe est composée. Voir note
 * méthodologique en tête de fichier.
 */
function bareGameContentZones(container) {
  return Array.from(container.querySelectorAll('.game-content-zones'))
    .filter(el => el.className === 'game-content-zones')
}

const KNOWN_TYPES = ['SPEEDY', 'QCM', 'MEMORY', 'MEMOTION', 'ARDOISE']

beforeEach(() => {
  vi.clearAllMocks()
  useCategories.mockReturnValue({ categories: [], loading: false, error: null, refetch: vi.fn() })
  // Évite "Not implemented: requestFullscreen" dans jsdom (même pattern que
  // PlayerDisplay.memotion.test.jsx)
  Object.defineProperty(document.documentElement, 'requestFullscreen', {
    value: vi.fn().mockResolvedValue(undefined),
    writable: true,
    configurable: true,
  })
})

// ---------------------------------------------------------------------------
// 1. Équivalence des gardes — les 5 types connus, avant/après A-F1.
//
// Table de vérité (dérivée du guard `showXxx && (isSpeedy || isArdoise)` /
// `(isSpeedy || isArdoise) && !(isArdoise && showAnswer && !isVPlayer) &&
// showGameContent && question`, identique à la négation qu'il remplace pour
// les 5 types connus) :
//   - SPEEDY, ARDOISE, QCM → count == 1 (SPEEDY/ARDOISE via le bloc générique
//     ou son variant ARDOISE-reveal ; QCM via son propre bloc, non affecté)
//   - MEMORY, MEMOTION     → count == 0 (leur bloc porte une classe composée)
// ---------------------------------------------------------------------------

describe('PlayerDisplay — équivalence des gardes de dispatch (#183/A-F1), COUNTDOWN', () => {
  it.each([
    ['SPEEDY', 1], ['ARDOISE', 1], ['QCM', 1], ['MEMORY', 0], ['MEMOTION', 0],
  ])('type=%s → %i bloc "game-content-zones" (bare) affiché', (type, expectedCount) => {
    const { container } = renderTV('COUNTDOWN', type)
    expect(bareGameContentZones(container)).toHaveLength(expectedCount)
  })
})

describe('PlayerDisplay — équivalence des gardes de dispatch (#183/A-F1), READY', () => {
  it.each([
    ['SPEEDY', 1], ['ARDOISE', 1], ['QCM', 1], ['MEMORY', 0], ['MEMOTION', 0],
  ])('type=%s → %i bloc "game-content-zones" (bare) affiché', (type, expectedCount) => {
    const { container } = renderTV('READY', type)
    expect(bareGameContentZones(container)).toHaveLength(expectedCount)
  })
})

describe('PlayerDisplay — équivalence des gardes de dispatch (#183/A-F1), STARTED', () => {
  it.each([
    ['SPEEDY', 1], ['ARDOISE', 1], ['QCM', 1], ['MEMORY', 0], ['MEMOTION', 0],
  ])('type=%s → %i bloc "game-content-zones" (bare) affiché', (type, expectedCount) => {
    const { container } = renderTV('STARTED', type)
    expect(bareGameContentZones(container)).toHaveLength(expectedCount)
  })
})

describe('PlayerDisplay — équivalence des gardes de dispatch (#183/A-F1), REVEALED (TV, non-VPlayer)', () => {
  // ARDOISE bascule sur son bloc dédié "ARDOISE TV Reveal" (`isArdoise &&
  // showAnswer && !isVPlayer`, positif, hors scope A-F1) plutôt que sur le
  // bloc générique — mais celui-ci porte AUSSI la classe bare
  // "game-content-zones" : le compte reste 1, seul le bloc source diffère.
  it.each([
    ['SPEEDY', 1], ['ARDOISE', 1], ['QCM', 1], ['MEMORY', 0], ['MEMOTION', 0],
  ])('type=%s → %i bloc "game-content-zones" (bare) affiché', (type, expectedCount) => {
    const { container } = renderTV('REVEALED', type)
    expect(bareGameContentZones(container)).toHaveLength(expectedCount)
  })

  it("SPEEDY : le bloc générique affiche l'ANSWER via .answer-text (bare, pas .ardoise-correct-answer)", () => {
    const { container } = renderTV('REVEALED', 'SPEEDY')
    const bare = Array.from(container.querySelectorAll('.answer-text'))
      .find(el => el.className === 'answer-text')
    expect(bare).toBeDefined()
    expect(bare.textContent).toBe('Réponse générique')
  })

  it("ARDOISE : PAS de .answer-text bare — la réponse passe par le bloc ARDOISE-reveal dédié (.ardoise-correct-answer)", () => {
    const { container } = renderTV('REVEALED', 'ARDOISE')
    const bare = Array.from(container.querySelectorAll('.answer-text'))
      .find(el => el.className === 'answer-text')
    expect(bare).toBeUndefined()
    expect(container.querySelector('.answer-text.ardoise-correct-answer')).not.toBeNull()
  })
})

// ---------------------------------------------------------------------------
// 2. Type inconnu — ne retombe plus silencieusement sur SPEEDY (#183, SEUL
//    changement de comportement du lot). Rouge avant A-F1 (négation : un type
//    inconnu n'est ni QCM ni MEMORY ni MEMOTION → passait), vert après
//    (dispatch positif : un type inconnu n'est ni SPEEDY/vide ni ARDOISE/QCM/
//    MEMORY/MEMOTION → ne matche plus aucune branche).
// ---------------------------------------------------------------------------

describe('PlayerDisplay — type inconnu ne rend plus la vue SPEEDY (#183, critère d\'acceptance)', () => {
  it.each(['COUNTDOWN', 'READY', 'STARTED', 'REVEALED'])(
    'phase=%s, TYPE="FOO" (inconnu) → aucun bloc "game-content-zones" (bare) rendu',
    (phase) => {
      const { container } = renderTV(phase, 'FOO')
      expect(bareGameContentZones(container)).toHaveLength(0)
    }
  )

  it('COUNTDOWN, type inconnu : ne rend pas non plus .countdown-number (aucune mise en page ne réclame ce type)', () => {
    const { container } = renderTV('COUNTDOWN', 'FOO')
    // Ni le bloc générique (guard désormais faux) ni le bloc QCM (isQcm faux)
    // ne peuvent produire ce marqueur pour un type inconnu.
    expect(bareGameContentZones(container)).toHaveLength(0)
    expect(container.querySelector('.countdown-number')).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// 3. Plus aucune garde par négation dans le source (#183 critère
//    d'acceptance) — scan de source, pas de rendu.
// ---------------------------------------------------------------------------

describe('PlayerDisplay.jsx — plus de garde en liste de négations (#183 critère d\'acceptance)', () => {
  const rawSource = fs.readFileSync(
    path.join(path.dirname(fileURLToPath(import.meta.url)), 'PlayerDisplay.jsx'),
    'utf-8'
  )
  // Le refactor A-F1 documente délibérément l'ancien motif de négation dans
  // des commentaires (backticks) pour expliquer l'équivalence — ces lignes ne
  // sont PAS du code et doivent être ignorées par le scan, sous peine de faux
  // positif sur la propre documentation du refactor qu'on vérifie ici.
  // Suppression volontairement minimale (lignes 100% commentaire) : suffisant
  // pour ce fichier, où ces explications sont toujours sur des lignes dédiées.
  const codeOnly = rawSource
    .split('\n')
    .filter(line => !line.trim().startsWith('//'))
    .join('\n')

  it('ne contient plus le motif `!isQcm && !isMemory && !isMemotion` en code (hors commentaires)', () => {
    expect(codeOnly).not.toMatch(/!isQcm\s*&&\s*!isMemory\s*&&\s*!isMemotion/)
  })

  it('ne contient plus de garde combinant 2+ négations des identifiants de type de contenu (isQcm/isMemory/isMemotion/isArdoise/isSpeedy)', () => {
    // Scopé aux identifiants du dispatch de contenu (#183) — délibérément
    // PAS un motif générique `!is\w+ && !is\w+`, qui matcherait des gardes
    // sans rapport et légitimes ailleurs dans le fichier (ex: mode d'affichage
    // `!isShowingScores && !isShowingPlayers && !isShowingPalmares`,
    // ou `!isVPlayer && !isAdminPreview && !isFullscreen`).
    const contentTypeNegationChain = /!(?:isQcm|isMemory|isMemotion|isArdoise|isSpeedy)\s*&&\s*!(?:isQcm|isMemory|isMemotion|isArdoise|isSpeedy)/
    expect(codeOnly).not.toMatch(contentTypeNegationChain)
  })

  it('utilise bien le dispatch positif `isSpeedy` dans les 3 gardes concernées (COUNTDOWN/READY/STARTED)', () => {
    const occurrences = codeOnly.match(/\(isSpeedy \|\| isArdoise\)/g) || []
    expect(occurrences.length).toBeGreaterThanOrEqual(3)
  })
})
