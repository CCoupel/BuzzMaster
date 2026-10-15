import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import AnimPage from './AnimPage'
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

// ---------------------------------------------------------------------------
// AnimPage — bandeau « zone contexte » en sous-phase SELECTED d'une manche
// MEMOTION : « Carte « <thème> » — ★ · <montant> » (#190, LOT-5, v11.2.2,
// contrat question-types.md §6.2 « Parité d'affichage »). Le montant suit la
// règle POINTS_RULE de la carte sélectionnée (fonction JS unique, miroir de
// `motionCardPointsForOutcome`) :
//   Étoiles (défaut) → « 3 pts » — inchangé
//   Montant fixe 10  → « 10 pts »
//   Par unité 2      → « 2 pts / bonne rép. » (RAFALE) · « 2 pts / paire » (MEMORY)
//
// Même scaffold que AnimPage.memotion-reveal.189.test.jsx. Le texte du bandeau
// est lu par son libellé « Carte « … » », jamais par classe CSS.
// ---------------------------------------------------------------------------

vi.mock('nosleep.js', () => ({
  default: class NoSleep { constructor() { this.isEnabled = false } enable() { this.isEnabled = true; return Promise.resolve() } disable() { this.isEnabled = false } },
}))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))
vi.mock('../hooks/useCategories', () => ({ useCategories: vi.fn() }))
vi.mock('../components/Timer', () => ({ default: () => <div data-testid="timer" /> }))
vi.mock('../components/AnimArdoiseList', () => ({ default: () => <div data-testid="ardoise-list" /> }))
vi.mock('../components/AnimAnswerZone', () => ({ default: () => <div data-testid="answer-zone" /> }))
vi.mock('./AnimPage.css', () => ({}))
vi.mock('../components/AnimConductPanel.css', () => ({}))
vi.mock('../components/LaunchGameHost.css', () => ({}))
vi.mock('../styles/entracte.css', () => ({}))

const THEME = 'Question bonus'

function makeGameMock(gameState) {
  return {
    status: 'connected',
    gameState: {
      phase: 'STARTED',
      timer: 30,
      totalTime: 30,
      gameTime: 0,
      entracte: false,
      entracteConfig: { TITLE: 'ENTRACTE', SUBTITLE: '', IMAGE_IS_CUSTOM: false, PANEL_SIZE: 65, ANIM_PERIOD: 10, ANIM_INTENSITY: 20 },
      ...gameState,
    },
    teams: {},
    bumpers: {},
    nextQuestion: null,
    questionPosition: { position: 0, total: 0 },
    awardedTeams: {},
    creditPoints: 0,
    startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(),
    revealAnswer: vi.fn(), selectQuestion: vi.fn(), setTeamPoints: vi.fn(), setBumperPoints: vi.fn(),
    flipMemoryCard: vi.fn(),
    regieMessage: { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: '' },
    clearRegieMessage: vi.fn(),
    selectMotionCard: vi.fn(), flipMotionCard: vi.fn(), stopMotionTimer: vi.fn(),
    revealMotionCard: vi.fn(), doneMotionCard: vi.fn(),
  }
}

const memotionSelected = (card, motionConfig) => ({
  phase: 'STARTED',
  question: {
    ID: 'q1', TYPE: 'MEMOTION', STATUS: 'READY',
    ...(motionConfig ? { MOTION_CONFIG: motionConfig } : {}),
    MOTION_CARDS: [
      { ID: 'c0', RECTO_THEME: 'Autre', DIFFICULTY: 1 },
      { ID: 'c1', RECTO_THEME: THEME, DIFFICULTY: 1, ...card },
    ],
  },
  MEMOTION_SUBPHASE: 'SELECTED',
  MEMOTION_SELECTED: 'c1',
  MEMOTION_CARD_STATES: { c1: 'FLIPPED' },
  MEMOTION_CARD_TEAMS: {},
  MEMOTION_CURRENT_TEAM: '',
  MEMOTION_PARTICIPATING_TEAMS: [],
})

/** Textes du bandeau « Carte « thème » — ★ · montant » (toutes occurrences). */
function statements(card, motionConfig) {
  useGame.mockReturnValue(makeGameMock(memotionSelected(card, motionConfig)))
  render(<AnimPage />)
  return screen.getAllByText(/Carte « Question bonus »/).map(el => el.textContent)
}

beforeEach(() => {
  vi.clearAllMocks()
  useCategories.mockReturnValue({ categories: [] })
})

describe('AnimPage — bandeau SELECTED d\'une carte MEMOTION : montant selon POINTS_RULE (#190)', () => {
  it('Montant fixe 10 sur une carte ★ : « 10 pts », pas le barème étoiles (1 pt)', () => {
    const texts = statements({ TYPE: 'QCM', POINTS_RULE: { MODE: 'FIXED', VALUE: 10 } })
    expect(texts.length).toBeGreaterThan(0)
    for (const text of texts) {
      expect(text).toMatch(/\b10\s?pts?\b/)
      expect(text).not.toMatch(/\b1\s?pt\b/)
      expect(text).toContain('★')
    }
  })

  it('Montant fixe ignore MOTION_CONFIG de la manche', () => {
    const texts = statements(
      { TYPE: 'SPEEDY', DIFFICULTY: 2, POINTS_RULE: { MODE: 'FIXED', VALUE: 4 } },
      { POINTS_1_STAR: 2, POINTS_2_STAR: 8, POINTS_3_STAR: 15 },
    )
    for (const text of texts) {
      expect(text).toMatch(/\b4\s?pts?\b/)
      expect(text).not.toMatch(/\b8\s?pts?\b/)
    }
  })

  it('Par unité 2 sur une carte RAFALE : « 2 pts / bonne rép. »', () => {
    const texts = statements({ TYPE: 'RAFALE', DIFFICULTY: 2, POINTS_RULE: { MODE: 'PER_UNIT', VALUE: 2 } })
    for (const text of texts) {
      expect(text).toMatch(/\b2\s?pts?\s?\/\s?bonne rép\./)
    }
  })

  it('Par unité 2 sur une carte MEMORY : « 2 pts / paire »', () => {
    const texts = statements({ TYPE: 'MEMORY', DIFFICULTY: 2, POINTS_RULE: { MODE: 'PER_UNIT', VALUE: 2 } })
    for (const text of texts) {
      expect(text).toMatch(/\b2\s?pts?\s?\/\s?paire/)
    }
  })

  it('NR — carte SPEEDY sans règle (★★) : « 3 pts », inchangé', () => {
    const texts = statements({ DIFFICULTY: 2 })
    for (const text of texts) {
      expect(text).toMatch(/\b3\s?pts?\b/)
      expect(text).not.toContain('/')
    }
  })

  it('NR — carte MEMORY par défaut (prorata, ★★★) : valeur étoiles 5 pts, pas 0 (aucune paire encore trouvée)', () => {
    const texts = statements({ TYPE: 'MEMORY', DIFFICULTY: 3 })
    for (const text of texts) {
      expect(text).toMatch(/\b5\s?pts?\b/)
    }
  })

  it('NR — STARS explicite sur une carte RAFALE (★★) : valeur étoiles 3 pts', () => {
    const texts = statements({ TYPE: 'RAFALE', DIFFICULTY: 2, POINTS_RULE: { MODE: 'STARS' } })
    for (const text of texts) {
      expect(text).toMatch(/\b3\s?pts?\b/)
    }
  })
})
