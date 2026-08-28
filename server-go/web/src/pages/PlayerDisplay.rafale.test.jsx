import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import PlayerDisplay from './PlayerDisplay'

// ---------------------------------------------------------------------------
// PlayerDisplay — TV `isRafale` (milestone v8.0.0 #16/#198, contrat
// contracts/rafale.md §2.1/§2.3/§4/§4bis/§8.2, maquette
// docs/mockups/rafale-v8.html §4/§4bis, CLAUDE.md « Contrainte Affichage TV »
// — `/tv` est STATIQUE, aucun défilement possible).
//
// Périmètre de ce fichier (batch 1, cf. tâche du CDP) : uniquement
//   (a) le rendu déclenché par isRafale, et
//   (b) l'absence de défilement introduite par le bloc RAFALE.
// L'indicateur « équipe active » VPlayer (pas TV) est hors périmètre
// (VPlayerPage.jsx, tâche 38, Batch 3) — mais TV le rend déjà ici (§4bis),
// donc couvert incidemment par les tests ci-dessous.
//
// Écrit contre l'implémentation RÉELLE : au moment où ce fichier est écrit
// (Batch 1), dev-frontend a déjà livré le bloc JSX `isRafale` complet
// (PlayerDisplay.jsx, ~L2812-2903) — double timer (RafaleTimers), question
// courante SANS réponse (RAFALE_CURRENT_QUESTION, jamais gameState.question
// directement — §3.3 : RAFALE ne porte pas d'énoncé propre), indicateur
// d'équipe active en mode multi, compteurs plafonnés à 6 équipes.
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

// Timer et RafaleTimers réels (non mockés) : RafaleTimers est le composant
// déjà testé isolément dans RafaleTimers.rafale.test.jsx — ici on vérifie
// le CÂBLAGE (props transmises) en lisant son rendu MM:SS réel, pas un
// double. framer-motion (dont Timer.jsx dépend) est auto-mocké via l'alias
// vite.config.js.

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

import { useGame } from '../hooks/GameContext'

const RAFALE_QUESTION = { ID: 'q-rafale-1', TYPE: 'RAFALE', RAFALE_QUESTION_TIME: 3 }
const SPEEDY_QUESTION = { ID: 'q-speedy-1', TYPE: 'SPEEDY', QUESTION: 'Capitale de la France ?', ANSWER: 'Paris' }

const TEAMS_2 = {
  'Équipe A': { NAME: 'Équipe A', SCORE: 10, COLOR: [99, 102, 241] },
  'Équipe B': { NAME: 'Équipe B', SCORE: 5, COLOR: [234, 179, 8] },
}

const DEFAULT_BUMPERS = {
  'AA:00:00:00:00:01': { TEAM: 'Équipe A', NAME: 'VJoueur-A', IS_VPLAYER: true, CONNECTED: true, COLOR: [99, 102, 241] },
}

function makeMock({
  phase = 'STARTED',
  question = RAFALE_QUESTION,
  teams = TEAMS_2,
  bumpers = DEFAULT_BUMPERS,
  rafaleCurrentQuestion = { ID: 'r-042', QUESTION: 'Capitale de l\'Italie ?', CATEGORY: 'GEOGRAPHY', DIFFICULTY: 2 },
  rafaleParticipatingTeams = [],
  rafaleCurrentTeam = '',
  rafaleCurrentTeamColor = [],
  rafaleTeamCounters = {},
  rafaleQuestionTime = 2,
} = {}) {
  return {
    gameState: {
      phase,
      remote: 'GAME',
      timer: 90,
      totalTime: 120,
      question,
      RAFALE_CURRENT_QUESTION: rafaleCurrentQuestion,
      RAFALE_QUESTION_TIME: rafaleQuestionTime,
      RAFALE_PARTICIPATING_TEAMS: rafaleParticipatingTeams,
      RAFALE_CURRENT_TEAM: rafaleCurrentTeam,
      RAFALE_CURRENT_TEAM_COLOR: rafaleCurrentTeamColor,
      RAFALE_TEAM_COUNTERS: rafaleTeamCounters,
      ARDOISE_ANSWERS: {},
      MEMORY_PARTICIPATING_TEAMS: [],
      MEMOTION_PARTICIPATING_TEAMS: [],
      MEMOTION_CARD_STATES: {},
      MEMOTION_CARD_TEAMS: {},
      MEMOTION_CURRENT_TEAM: null,
      MEMOTION_SELECTED: null,
      newGameBackgrounds: [],
    },
    teams,
    bumpers,
    flipMemoryCard: vi.fn(),
    showQRCode: false,
    selectMotionCard: vi.fn(),
  }
}

const renderTV = (overrides = {}) => {
  useGame.mockReturnValue(makeMock(overrides))
  return render(<PlayerDisplay />)
}

beforeEach(() => {
  vi.clearAllMocks()
  Object.defineProperty(document.documentElement, 'requestFullscreen', {
    value: vi.fn().mockResolvedValue(undefined),
    writable: true,
    configurable: true,
  })
})

// ---------------------------------------------------------------------------
// Rendu déclenché par isRafale
// ---------------------------------------------------------------------------

describe('PlayerDisplay — RAFALE : rendu déclenché par isRafale', () => {
  it('TYPE=RAFALE, phase STARTED : rend EXACTEMENT un bloc .game-content-zones, marqué .rafale-tv', () => {
    const { container } = renderTV({ phase: 'STARTED' })
    const zones = container.querySelectorAll('.game-content-zones')
    expect(zones).toHaveLength(1)
    expect(zones[0].classList.contains('rafale-tv')).toBe(true)
  })

  it('TYPE=SPEEDY : le bloc RAFALE (.rafale-tv) n\'est pas rendu — dispatch positif, aucune fuite croisée', () => {
    const { container } = renderTV({ phase: 'STARTED', question: SPEEDY_QUESTION })
    expect(container.querySelector('.game-content-zones.rafale-tv')).toBeNull()
    // Témoin : le bloc générique SPEEDY, lui, est bien là.
    expect(container.querySelector('.game-content-zones')).not.toBeNull()
  })

  it('affiche l\'énoncé depuis RAFALE_CURRENT_QUESTION — JAMAIS depuis gameState.question (§3.3 : RAFALE ne porte pas d\'énoncé propre)', () => {
    renderTV({
      phase: 'STARTED',
      question: RAFALE_QUESTION, // pas de QUESTION ici, volontairement (contrat §3.3)
      rafaleCurrentQuestion: { ID: 'r-1', QUESTION: 'Plus long fleuve du monde ?', CATEGORY: 'GEOGRAPHY', DIFFICULTY: 1 },
    })
    expect(screen.getByText('Plus long fleuve du monde ?')).toBeInTheDocument()
  })

  it('câble RafaleTimers avec le timer de manche (gameState.timer/totalTime) et le timer de question (RAFALE_QUESTION_TIME)', () => {
    const { container } = renderTV({ phase: 'STARTED', rafaleQuestionTime: 2 })
    // RafaleTimers monte 2 vrais Timer (non mockés) — 2 affichages MM:SS.
    const displays = Array.from(container.querySelectorAll('.timer-display')).map((el) => el.textContent)
    expect(displays).toContain('01:30') // 90s de manche restant
    expect(displays).toContain('00:02') // 2s de question restant
  })

  it('la réponse (ANSWER) n\'apparaît JAMAIS dans le DOM, même si présente par erreur dans RAFALE_CURRENT_QUESTION (défense en profondeur, contrat §2.3)', () => {
    const { container } = renderTV({
      phase: 'STARTED',
      // Simule la fuite que le contrat interdit structurellement : un
      // ANSWER glissé dans RAFALE_CURRENT_QUESTION malgré tout.
      rafaleCurrentQuestion: { ID: 'r-1', QUESTION: 'Q?', CATEGORY: 'HISTORY', DIFFICULTY: 1, ANSWER: 'NE_DOIT_JAMAIS_S_AFFICHER' },
    })
    expect(container.textContent).not.toContain('NE_DOIT_JAMAIS_S_AFFICHER')
  })
})

// ---------------------------------------------------------------------------
// Indicateur d'équipe active (§8.2) — mode multi uniquement
// ---------------------------------------------------------------------------

describe('PlayerDisplay — RAFALE : indicateur équipe active (mode multi)', () => {
  it('mode SOLO (RAFALE_PARTICIPATING_TEAMS vide) : aucun indicateur d\'équipe active', () => {
    const { container } = renderTV({ phase: 'STARTED', rafaleParticipatingTeams: [], rafaleCurrentTeam: '' })
    expect(container.querySelector('.rafale-tv-active-team')).toBeNull()
  })

  it('mode multi, équipe active définie : le bandeau affiche son nom', () => {
    const { container } = renderTV({
      phase: 'STARTED',
      rafaleParticipatingTeams: ['Équipe A', 'Équipe B'],
      rafaleCurrentTeam: 'Équipe A',
      rafaleCurrentTeamColor: [99, 102, 241],
    })
    const banner = container.querySelector('.rafale-tv-active-team')
    expect(banner).not.toBeNull()
    expect(banner.textContent).toContain('Équipe A')
  })

  it('compteurs par équipe plafonnés à 6 (règle TV statique, CLAUDE.md)', () => {
    const eightTeams = Object.fromEntries(Array.from({ length: 8 }, (_, i) => [`Team${i}`, i]))
    const { container } = renderTV({
      phase: 'STARTED',
      rafaleParticipatingTeams: Object.keys(eightTeams),
      rafaleTeamCounters: eightTeams,
    })
    expect(container.querySelectorAll('.rafale-tv-team').length).toBeLessThanOrEqual(6)
  })
})

// ---------------------------------------------------------------------------
// Absence de défilement (CLAUDE.md, /tv STATIQUE) — garde-fou générique :
// aucun élément du sous-arbre RAFALE ne doit porter un style INLINE
// overflow:auto/scroll. Ce n'est PAS la protection principale (celle-ci est
// la convention CSS externe overflow:hidden + unités viewport, revue par
// code-reviewer et vérifiée manuellement en QA — tests/procedures/
// rafale-v8.md) : ce test attrape uniquement le contournement le plus
// direct (un style inline ajouté à la va-vite).
// ---------------------------------------------------------------------------

describe('PlayerDisplay — RAFALE : absence de défilement introduit (garde-fou style inline)', () => {
  it('aucun élément rendu pour TYPE=RAFALE ne porte overflow:auto ou overflow:scroll en style inline', () => {
    const { container } = renderTV({ phase: 'STARTED' })
    const offenders = Array.from(container.querySelectorAll('*')).filter((el) => {
      const overflow = el.style?.overflow
      return overflow === 'auto' || overflow === 'scroll'
    })
    expect(offenders).toHaveLength(0)
  })
})
