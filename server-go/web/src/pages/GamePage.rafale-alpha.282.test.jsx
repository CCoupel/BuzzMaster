/**
 * #282 (v11.4.0, LOT-2) — contracts/rafale.md §15.7, maquette admin_question_editor__rafale_alphabetical :
 * GamePage (admin) — chip « 🔤 Alphabétique » dans `.rafale-admin-config`, pastille de la lettre courante
 * dans `.rafale-admin-live`, « Suivante (lettre S) : … ». Option inactive : rien de plus. Les tests de
 * fuite (TV/VPlayer) vivent côté Go. Scaffold calqué sur GamePage.rafale.test.jsx.
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

describe('GamePage — option Alphabétique RAFALE (#282)', () => {
  beforeEach(() => vi.clearAllMocks())

  it('option active : chip « 🔤 Alphabétique » dans .rafale-admin-config', () => {
    useGame.mockReturnValue(makeGameMock({ alphabetical: true }))
    const { container } = render(<GamePage />)
    const cfg = container.querySelector('.rafale-admin-config')
    expect(cfg).not.toBeNull()
    expect(cfg.textContent).toContain('Alphabétique')
  })

  it('option inactive : aucune chip Alphabétique (rendu inchangé)', () => {
    useGame.mockReturnValue(makeGameMock({ alphabetical: false }))
    const { container } = render(<GamePage />)
    expect(container.querySelector('.rafale-admin-panel').textContent).not.toContain('Alphabétique')
  })

  it('option active : RafalePoolAlert reçoit alphabetical=true ; inactive : false', () => {
    useGame.mockReturnValue(makeGameMock({ alphabetical: true }))
    const { unmount } = render(<GamePage />)
    expect(screen.getByTestId('rafale-pool-alert-stub').getAttribute('data-alphabetical')).toBe('true')
    unmount()
    useGame.mockReturnValue(makeGameMock({ alphabetical: false }))
    render(<GamePage />)
    expect(screen.getByTestId('rafale-pool-alert-stub').getAttribute('data-alphabetical')).toBe('false')
  })

  it('manche en cours, option active : la lettre courante est affichée dans .rafale-admin-live', () => {
    useGame.mockReturnValue(makeGameMock({ alphabetical: true, phase: 'STARTED' }))
    const { container } = render(<GamePage />)
    const live = container.querySelector('.rafale-admin-live')
    expect(live).not.toBeNull()
    expect(live.textContent).toMatch(/\bS\b/)
  })

  it('manche en cours, option inactive : pas de pastille de lettre', () => {
    useGame.mockReturnValue(makeGameMock({ alphabetical: false, phase: 'STARTED' }))
    const { container } = render(<GamePage />)
    const live = container.querySelector('.rafale-admin-live')
    expect(live?.querySelector('.rafale-letter-badge, [data-testid="rafale-letter"]') ?? null).toBeNull()
  })
})
