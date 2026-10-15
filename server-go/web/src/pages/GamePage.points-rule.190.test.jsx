/**
 * GamePage — panneau MEMOTION d'administration : le montant de la carte
 * sélectionnée suit sa règle POINTS_RULE (#190, LOT-5, v11.2.2, contrat
 * question-types.md §6.2 « Parité d'affichage »). Les trois sous-phases qui
 * affichent la carte (SELECTED, QUESTION, REVEAL) utilisent la même fonction
 * JS que l'éditeur et l'anim (utils/motionGrid.js, miroir de
 * `motionCardPointsForOutcome`) :
 *   Étoiles (défaut)  → « ★★ · 3 pts »                 — inchangé
 *   Montant fixe 10   → « ★ · 10 pts »                 — valeur de la règle
 *   Par unité 2       → « 2 pts / bonne rép. » (RAFALE) · « 2 pts / paire » (MEMORY)
 *
 * Le montant est lu dans le bloc « thème + étoiles » de la carte (parent du
 * thème), jamais par classe CSS. Mocks calqués sur GamePage.rafale.test.jsx.
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('../hooks/useCategories', () => ({
  useCategories: vi.fn(() => ({ categories: [], loading: false, error: null, refetch: vi.fn() })),
}))

vi.mock('../components/Timer', () => ({
  default: () => <div data-testid="timer" />,
}))

vi.mock('../components/QuestionPreview', () => ({
  default: () => <div data-testid="question-preview" />,
}))

vi.mock('../components/TeamCard', () => ({
  default: ({ name }) => <div data-testid={`team-card-${name}`} />,
  OtaAllModal: () => null,
}))

vi.mock('../components/QuestionCard', () => ({
  default: ({ question, onClick }) => (
    <div data-testid={`question-card-${question.ID}`} onClick={() => onClick && onClick(question)} />
  ),
  CATEGORIES: {
    GEOGRAPHY: { label: 'Geographie', icon: '🌍', color: '#3b82f6' },
  },
}))

vi.mock('../components/Card', () => ({
  default: ({ children, className, padding, variant, ...rest }) => (
    <div className={className} {...rest}>{children}</div>
  ),
}))

vi.mock('../components/Button', () => ({
  default: ({ children, onClick, disabled, variant, size, ...rest }) => (
    <button onClick={onClick} disabled={disabled} {...rest}>{children}</button>
  ),
}))

vi.mock('../components/RafalePoolAlert', () => ({
  default: () => <div data-testid="rafale-pool-alert-stub" />,
}))

vi.mock('./GamePage.css', () => ({}))

import { useGame } from '../hooks/GameContext'
import GamePage from './GamePage'

const THEME = 'Question bonus'

const makeGameMock = ({ subphase, card, motionConfig }) => ({
  gameState: {
    phase: 'STARTED',
    question: {
      ID: '1', TYPE: 'MEMOTION', STATUS: 'STARTED',
      ...(motionConfig ? { MOTION_CONFIG: motionConfig } : {}),
      MOTION_CARDS: [
        { ID: 'c1', RECTO_THEME: 'Autre carte', DIFFICULTY: 1 },
        { ID: 'c2', RECTO_THEME: THEME, DIFFICULTY: 1, QUESTION_TEXT: 'Quelle est la question ?', ANSWER_TEXT: 'La réponse', ...card },
      ],
    },
    remote: 'GAME',
    timer: 0,
    totalTime: 30,
    MEMOTION_SUBPHASE: subphase,
    MEMOTION_SELECTED: 'c2',
    MEMOTION_CARD_STATES: { c2: 'FLIPPED' },
    MEMOTION_CARD_TEAMS: {},
    MEMOTION_CURRENT_TEAM: '',
    MEMOTION_PARTICIPATING_TEAMS: [],
    MEMORY_PARTICIPATING_TEAMS: [],
  },
  teams: {},
  bumpers: {},
  questions: { '1': { ID: '1', TYPE: 'MEMOTION', STATUS: 'STARTED', ORDER: 1 } },
  startGame: vi.fn(),
  stopGame: vi.fn(),
  pauseGame: vi.fn(),
  continueGame: vi.fn(),
  revealAnswer: vi.fn(),
  selectQuestion: vi.fn(),
  setRemoteDisplay: vi.fn(),
  setBumperPoints: vi.fn(),
  setTeamPoints: vi.fn(),
  forceReady: vi.fn(),
  simulateButton: vi.fn(),
  simulatePong: vi.fn(),
  sendMessage: vi.fn(),
})

/** Texte du bloc « thème + étoiles + montant » de la carte sélectionnée. */
function cardInfoText(subphase, card, motionConfig) {
  useGame.mockReturnValue(makeGameMock({ subphase, card, motionConfig }))
  render(<GamePage />)
  return screen.getByText(THEME).parentElement.textContent
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe.each(['SELECTED', 'QUESTION', 'REVEAL'])('GamePage — panneau MEMOTION, sous-phase %s : montant selon POINTS_RULE (#190)', (subphase) => {
  it('Montant fixe 10 sur une carte ★ : « 10 pts », pas le barème étoiles (1 pt)', () => {
    const text = cardInfoText(subphase, { TYPE: 'QCM', POINTS_RULE: { MODE: 'FIXED', VALUE: 10 } })
    expect(text).toMatch(/\b10\s?pts?\b/)
    expect(text).not.toMatch(/\b1\s?pt\b/)
    expect(text).toContain('★')
  })

  it('Montant fixe ignore MOTION_CONFIG', () => {
    const text = cardInfoText(
      subphase,
      { TYPE: 'SPEEDY', DIFFICULTY: 2, POINTS_RULE: { MODE: 'FIXED', VALUE: 4 } },
      { POINTS_1_STAR: 2, POINTS_2_STAR: 8, POINTS_3_STAR: 15 },
    )
    expect(text).toMatch(/\b4\s?pts?\b/)
    expect(text).not.toMatch(/\b8\s?pts?\b/)
  })

  it('Par unité 2 sur une carte RAFALE : « 2 pts / bonne rép. »', () => {
    const text = cardInfoText(subphase, { TYPE: 'RAFALE', DIFFICULTY: 2, POINTS_RULE: { MODE: 'PER_UNIT', VALUE: 2 } })
    expect(text).toMatch(/\b2\s?pts?\s?\/\s?bonne rép\./)
  })

  it('Par unité 2 sur une carte MEMORY : « 2 pts / paire »', () => {
    const text = cardInfoText(subphase, { TYPE: 'MEMORY', DIFFICULTY: 2, POINTS_RULE: { MODE: 'PER_UNIT', VALUE: 2 } })
    expect(text).toMatch(/\b2\s?pts?\s?\/\s?paire/)
  })

  it('NR — carte SPEEDY sans règle (★★) : « 3 pts », sans unité', () => {
    const text = cardInfoText(subphase, { DIFFICULTY: 2 })
    expect(text).toMatch(/\b3\s?pts?\b/)
    expect(text).not.toContain('/')
  })

  it('NR — carte MEMORY par défaut (prorata, ★★★) : valeur étoiles 5 pts affichée, inchangée', () => {
    const text = cardInfoText(subphase, { TYPE: 'MEMORY', DIFFICULTY: 3 })
    expect(text).toMatch(/\b5\s?pts?\b/)
  })

  it('NR — STARS explicite sur MEMORY (★★★) : valeur étoiles 5 pts', () => {
    const text = cardInfoText(subphase, { TYPE: 'MEMORY', DIFFICULTY: 3, POINTS_RULE: { MODE: 'STARS' } })
    expect(text).toMatch(/\b5\s?pts?\b/)
  })
})

// ---------------------------------------------------------------------------
// Préfixe « jusqu'à » du prorata (#190, m3 de la revue) : avant le jeu, le
// montant d'une carte en STARS_PRORATA est un maximum. Format historique du
// panneau conservé : « ★★★ · jusqu'à 5pt » (sans espace avant « pt »).
// ---------------------------------------------------------------------------

describe.each(['SELECTED', 'QUESTION', 'REVEAL'])('GamePage — panneau MEMOTION, sous-phase %s : préfixe « jusqu\'à » du prorata (#190, m-B)', (subphase) => {
  it('carte MEMORY par défaut (prorata, ★★★) : « jusqu\'à 5pt »', () => {
    expect(cardInfoText(subphase, { TYPE: 'MEMORY', DIFFICULTY: 3 })).toMatch(/· jusqu'à 5pt/)
  })

  it('carte RAFALE par défaut (prorata, ★★) : « jusqu\'à 3pt »', () => {
    expect(cardInfoText(subphase, { TYPE: 'RAFALE', DIFFICULTY: 2 })).toMatch(/· jusqu'à 3pt/)
  })

  it('STARS_PRORATA explicite (MEMORY ★★★) : même préfixe que le défaut', () => {
    expect(cardInfoText(subphase, { TYPE: 'MEMORY', DIFFICULTY: 3, POINTS_RULE: { MODE: 'STARS_PRORATA' } })).toMatch(/jusqu'à 5pt/)
  })

  it('STARS explicite (tout-ou-rien, MEMORY ★★★) : « 5pt » SANS préfixe', () => {
    const text = cardInfoText(subphase, { TYPE: 'MEMORY', DIFFICULTY: 3, POINTS_RULE: { MODE: 'STARS' } })
    expect(text).toMatch(/\b5pt/)
    expect(text).not.toContain('jusqu\'à')
  })

  it('carte SPEEDY par défaut : aucun préfixe', () => {
    expect(cardInfoText(subphase, { DIFFICULTY: 2 })).not.toContain('jusqu\'à')
  })
})
