/**
 * #282 (v11.4.0, LOT-2) — contracts/rafale.md §15.10 / §15.7 : GamePage (admin) affiche la lettre de la
 * question SUIVANTE, « Suivante (lettre S) : … », lue dans rafaleAnswer.NEXT.LETTER (jamais normalisée
 * côté frontend). LETTER vide (option inactive) : pas de libellé « lettre ». Scaffold = GamePage.rafale-alpha.282.test.jsx.
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'

vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))
vi.mock('../hooks/useCategories', () => ({
  useCategories: vi.fn(() => ({ categories: [], loading: false, error: null, refetch: vi.fn() })),
}))
vi.mock('../components/Timer', () => ({ default: () => <div data-testid="timer" /> }))
vi.mock('../components/QuestionPreview', () => ({ default: () => <div data-testid="question-preview" /> }))
vi.mock('../components/TeamCard', () => ({ default: ({ name }) => <div data-testid={`team-card-${name}`} />, OtaAllModal: () => null }))
vi.mock('../components/QuestionCard', () => ({
  default: ({ question }) => <div data-testid={`question-card-${question.ID}`} />,
  CATEGORIES: { HISTORY: { label: 'Histoire', icon: '📜', color: '#eab308' } },
}))
vi.mock('../components/Card', () => ({ default: ({ children, className }) => <div className={className}>{children}</div> }))
vi.mock('../components/Button', () => ({
  default: ({ children, onClick, disabled }) => <button onClick={onClick} disabled={disabled}>{children}</button>,
}))
vi.mock('../components/RafalePoolAlert', () => ({
  default: ({ alphabetical }) => <div data-testid="rafale-pool-alert-stub" data-alphabetical={String(!!alphabetical)} />,
}))
vi.mock('./GamePage.css', () => ({}))

import { useGame } from '../hooks/GameContext'
import GamePage from './GamePage'

const makeGameMock = ({ alphabetical = false, phase = 'STOPPED', gameExtra = {}, rafaleAnswer = null } = {}) => ({
  gameState: {
    phase,
    question: {
      ID: '1', TYPE: 'RAFALE', STATUS: phase === 'STARTED' ? 'STARTED' : 'STOPPED',
      RAFALE_CATEGORIES: ['HISTORY'], RAFALE_DIFFICULTIES: [2], RAFALE_MODE: 'SOLO', RAFALE_QUESTION_TIME: 3,
      ...(alphabetical ? { RAFALE_ALPHABETICAL: true } : {}),
    },
    remote: 'GAME', timer: 0, totalTime: 120, MEMORY_PARTICIPATING_TEAMS: [],
    RAFALE_SUBPHASE: phase === 'STARTED' ? 'QUESTION' : '',
    RAFALE_CURRENT_QUESTION: phase === 'STARTED' ? { ID: 'r-1', QUESTION: 'Fleuve de Paris ?', CATEGORY: 'HISTORY', DIFFICULTY: 1 } : { ID: '', QUESTION: '' },
    RAFALE_CURRENT_LETTER: alphabetical && phase === 'STARTED' ? 'S' : '',
    RAFALE_LETTERS: alphabetical && phase === 'STARTED' ? ['T', 'V'] : [],
    ...gameExtra,
  },
  teams: {}, bumpers: {},
  questions: { '1': { ID: '1', TYPE: 'RAFALE', STATUS: 'STOPPED', ORDER: 1, CATEGORY: 'HISTORY' } },
  startGame: vi.fn(), stopGame: vi.fn(), pauseGame: vi.fn(), continueGame: vi.fn(), revealAnswer: vi.fn(),
  selectQuestion: vi.fn(), setRemoteDisplay: vi.fn(), setBumperPoints: vi.fn(), setTeamPoints: vi.fn(),
  forceReady: vi.fn(), simulateButton: vi.fn(), simulatePong: vi.fn(), sendMessage: vi.fn(), rafaleSetTeams: vi.fn(),
  rafaleAnswer, rafaleValidate: vi.fn(), rafaleInvalidate: vi.fn(),
})

const NEXT_Q = (letter) => ({
  ID: 'r-1', ANSWER: 'Seine',
  NEXT: { ID: 'r-2', QUESTION: 'Fleuve de Rome ?', CATEGORY: 'HISTORY', DIFFICULTY: 1, LETTER: letter },
})

describe('GamePage — lettre de la question suivante (#282, §15.10)', () => {
  beforeEach(() => vi.clearAllMocks())

  it('NEXT.LETTER = "T" : affiche « Suivante (lettre T) : <énoncé> » dans .rafale-admin-live', () => {
    useGame.mockReturnValue(makeGameMock({ alphabetical: true, phase: 'STARTED', rafaleAnswer: NEXT_Q('T') }))
    const { container } = render(<GamePage />)
    const live = container.querySelector('.rafale-admin-live')
    expect(live.textContent).toMatch(/Suivante\s*\(lettre T\)\s*:\s*Fleuve de Rome \?/)
    expect(live.textContent).toContain('Fleuve de Rome ?')
  })

  it('la lettre affichée vient de NEXT.LETTER, pas de la lettre courante ni d\'une normalisation locale', () => {
    useGame.mockReturnValue(makeGameMock({ alphabetical: true, phase: 'STARTED', rafaleAnswer: NEXT_Q('#') }))
    const { container } = render(<GamePage />)
    const live = container.querySelector('.rafale-admin-live')
    expect(live.textContent).toMatch(/\(lettre #\)/)
    expect(live.textContent).not.toMatch(/\(lettre S\)/)
  })

  it('NEXT.LETTER = "" (sans l\'option) : « Suivante : <énoncé> », sans « lettre »', () => {
    useGame.mockReturnValue(makeGameMock({ alphabetical: false, phase: 'STARTED', rafaleAnswer: NEXT_Q('') }))
    const { container } = render(<GamePage />)
    const txt = container.querySelector('.rafale-admin-live').textContent
    expect(txt).toMatch(/Suivante\s*:\s*Fleuve de Rome \?/)
    expect(txt).not.toMatch(/lettre/i)
  })

  it('NEXT = null : « Dernière question du réservoir », sans « lettre »', () => {
    useGame.mockReturnValue(makeGameMock({ alphabetical: true, phase: 'STARTED', rafaleAnswer: { ID: 'r-1', ANSWER: 'Seine', NEXT: null } }))
    const { container } = render(<GamePage />)
    const txt = container.querySelector('.rafale-admin-live').textContent
    expect(txt).toContain('Dernière question du réservoir')
    expect(txt).not.toMatch(/\(lettre/)
  })

  it('la réponse de la question suivante n\'est jamais affichée', () => {
    const ra = NEXT_Q('T'); ra.NEXT.ANSWER = 'Tibre-secret'
    useGame.mockReturnValue(makeGameMock({ alphabetical: true, phase: 'STARTED', rafaleAnswer: ra }))
    const { container } = render(<GamePage />)
    expect(container.querySelector('.rafale-admin-live').textContent).not.toContain('Tibre-secret')
  })
})
