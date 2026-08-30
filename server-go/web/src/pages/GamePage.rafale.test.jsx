/**
 * Tests for GamePage — panneau de pré-lancement RAFALE, catégorie unique
 * (v8.0.0, #16/#107, bugfix 2026-08-29, contrat rafale.md §3.3, SHA 8f8ff92d).
 *
 * Le panneau affichait un compteur "N catégorie(s)" (RAFALE_CATEGORIES,
 * multi, retiré) — il affiche désormais un seul CategoryBadge, exactement
 * comme le reste du projet (nextUnplayedQuestion, gameState.question plus
 * haut dans GamePage.jsx, déjà ce même patron avant RAFALE).
 *
 * Mocks calqués sur GamePage.categories.test.jsx (même scaffold) — la
 * différence : useCategories() est mocké ici (RafalePoolAlert et le badge
 * de catégorie du panneau RAFALE en dépendent tous les deux via
 * customCategories), RafalePoolAlert lui-même est stubbé (déjà couvert
 * isolément par components/RafalePoolAlert.rafale.test.jsx — inutile de
 * dupliquer ses 3 états d'alerte/son format de requête ici).
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
    HISTORY: { label: 'Histoire', icon: '📜', color: '#eab308' },
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

// RafalePoolAlert : déjà couvert isolément (components/RafalePoolAlert.
// rafale.test.jsx) — stub léger ici, ce fichier teste UNIQUEMENT le
// panneau GamePage lui-même (badge de catégorie unique).
vi.mock('../components/RafalePoolAlert', () => ({
  default: () => <div data-testid="rafale-pool-alert-stub" />,
}))

vi.mock('./GamePage.css', () => ({}))

import { useGame } from '../hooks/GameContext'
import GamePage from './GamePage'

// ⚠️ `...overrides` spreadé EN PREMIER : sinon un `overrides.gameState`
// PARTIEL (ex. { question: {...} } seul, sans `phase`) écraserait
// entièrement la clé `gameState` fusionnée juste en dessous, faisant
// disparaître `phase: 'STOPPED'` silencieusement (bug réel identifié et
// corrigé dans GamePage.rafaleStartGate.test.jsx — inoffensif ICI
// puisqu'aucun test de ce fichier ne dépend de `phase`/`isPlaying`, mais
// corrigé par cohérence pour ne pas re-piéger un futur test ajouté ici).
const makeGameMock = (overrides = {}) => ({
  ...overrides,
  gameState: {
    phase: 'STOPPED',
    question: { ID: '1', TYPE: 'RAFALE', STATUS: 'STOPPED', CATEGORY: 'HISTORY', RAFALE_DIFFICULTY: 2, RAFALE_MODE: 'SOLO', RAFALE_QUESTION_TIME: 3 },
    remote: 'GAME',
    timer: 0,
    totalTime: 120,
    MEMORY_PARTICIPATING_TEAMS: [],
    ...overrides.gameState,
  },
  teams: overrides.teams ?? {},
  bumpers: overrides.bumpers ?? {},
  questions: overrides.questions ?? {
    '1': { ID: '1', TYPE: 'RAFALE', STATUS: 'STOPPED', ORDER: 1, CATEGORY: 'HISTORY' },
  },
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
  rafaleSetTeams: vi.fn(),
  rafaleAnswer: overrides.rafaleAnswer ?? null,
})

describe('GamePage — panneau de pré-lancement RAFALE : catégorie unique (bugfix 2026-08-29, SHA 8f8ff92d)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('question RAFALE sélectionnée, phase STOPPED : le panneau de configuration de manche est affiché', () => {
    useGame.mockReturnValue(makeGameMock())
    const { container } = render(<GamePage />)

    expect(container.querySelector('.rafale-admin-panel')).not.toBeNull()
  })

  it('CATEGORY définie : affiche un SEUL CategoryBadge (plus de compteur "N catégorie(s)")', () => {
    useGame.mockReturnValue(makeGameMock())
    const { container } = render(<GamePage />)

    const panel = container.querySelector('.rafale-admin-panel')
    expect(panel.querySelectorAll('.category-badge').length).toBe(1)
    expect(screen.getByText('Histoire')).toBeInTheDocument()
    // L'ancien libellé compteur ne doit plus jamais apparaître.
    expect(panel.textContent).not.toMatch(/catégorie\(s\)/)
    expect(panel.textContent).not.toMatch(/\d+\s*catégorie/)
  })

  it('CATEGORY absente : aucun badge de catégorie affiché (pas de repli sur un compteur à 0)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { question: { ID: '1', TYPE: 'RAFALE', STATUS: 'STOPPED', CATEGORY: '', RAFALE_DIFFICULTY: 1 } },
    }))
    const { container } = render(<GamePage />)

    const panel = container.querySelector('.rafale-admin-panel')
    expect(panel.querySelectorAll('.category-badge').length).toBe(0)
  })

  it('affiche aussi la difficulté, le mode et le temps par question (autres chips du panneau, non-régression)', () => {
    useGame.mockReturnValue(makeGameMock())
    const { container } = render(<GamePage />)

    const panel = container.querySelector('.rafale-admin-panel')
    const chips = Array.from(panel.querySelectorAll('.rafale-admin-chip')).map(el => el.textContent)
    expect(chips.some(c => c.includes('★★'))).toBe(true) // difficulté 2
    expect(chips.some(c => c === 'SOLO')).toBe(true)
    expect(chips.some(c => c.includes('3s/question'))).toBe(true)
  })

  it('transmet la CATEGORY unique à RafalePoolAlert (pas un tableau)', () => {
    useGame.mockReturnValue(makeGameMock())
    render(<GamePage />)
    // Le stub ne reçoit pas d'assertion de props directement ici — couvert
    // fonctionnellement par RafalePoolAlert.rafale.test.jsx ; on vérifie
    // seulement qu'il est bien monté dans ce contexte RAFALE.
    expect(screen.getByTestId('rafale-pool-alert-stub')).toBeInTheDocument()
  })

  it('question non-RAFALE : le panneau RAFALE n\'est jamais affiché (non-régression)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { question: { ID: '1', TYPE: 'SPEEDY', STATUS: 'STOPPED', CATEGORY: 'HISTORY' } },
    }))
    const { container } = render(<GamePage />)

    expect(container.querySelector('.rafale-admin-panel')).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// .rafale-admin-live (maquette rafale-v8.html §9.3, SHA 01eb5ce9) — /admin
// affichait RIEN pendant la manche (STARTED) avant ce lot ; le panneau de
// pré-lancement se cache dès isPlaying (voir describe précédent). Nouveau
// bloc, visible UNIQUEMENT en isPlaying, montrant le même encart coloré
// équipe + question + réponse que /anim (mêmes classes .rafale-anim-qcard*
// — pas de duplication de test de rendu détaillé, juste la présence et le
// bon contenu ici, la structure de l'encart lui-même étant déjà exhaustive
// dans AnimPage.rafale.test.jsx).
// ---------------------------------------------------------------------------

describe('GamePage — .rafale-admin-live : question+réponse pendant la manche (bugfix SHA 01eb5ce9)', () => {
  it('phase STOPPED (avant lancement) : .rafale-admin-live absent — c\'est le panneau de pré-lancement qui s\'affiche', () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { phase: 'STOPPED' } }))
    const { container } = render(<GamePage />)

    expect(container.querySelector('.rafale-admin-live')).toBeNull()
    expect(container.querySelector('.rafale-admin-panel')).not.toBeNull()
  })

  it('phase STARTED (manche en cours) : .rafale-admin-live visible, panneau de pré-lancement caché', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STARTED',
        RAFALE_CURRENT_QUESTION: { ID: 'r-042', QUESTION: 'Capitale de l\'Italie ?', CATEGORY: 'GEOGRAPHY', DIFFICULTY: 2 },
      },
    }))
    const { container } = render(<GamePage />)

    expect(container.querySelector('.rafale-admin-live')).not.toBeNull()
    expect(container.querySelector('.rafale-admin-panel')).toBeNull()
  })

  it('affiche la question courante (RAFALE_CURRENT_QUESTION)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STARTED',
        RAFALE_CURRENT_QUESTION: { ID: 'r-042', QUESTION: 'Capitale de l\'Italie ?', CATEGORY: 'GEOGRAPHY', DIFFICULTY: 2 },
      },
    }))
    render(<GamePage />)

    expect(screen.getByText('Capitale de l\'Italie ?')).toBeInTheDocument()
  })

  it('rafaleAnswer.ID correspond à la question courante : affiche la réponse (RAFALE_ANSWER déjà diffusé à admin, contrat §2.3 — simple choix d\'affichage)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STARTED',
        RAFALE_CURRENT_QUESTION: { ID: 'r-042', QUESTION: 'Capitale de l\'Italie ?', CATEGORY: 'GEOGRAPHY', DIFFICULTY: 2 },
      },
      rafaleAnswer: { ID: 'r-042', ANSWER: 'Rome' },
    }))
    render(<GamePage />)

    expect(screen.getByText('Rome')).toBeInTheDocument()
  })

  it('rafaleAnswer.ID NE correspond PAS à la question courante (pas encore reçu pour cette question) : aucune réponse affichée', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STARTED',
        RAFALE_CURRENT_QUESTION: { ID: 'r-042', QUESTION: 'Capitale de l\'Italie ?', CATEGORY: 'GEOGRAPHY', DIFFICULTY: 2 },
      },
      rafaleAnswer: { ID: 'r-999', ANSWER: 'Berlin' },
    }))
    render(<GamePage />)

    expect(screen.queryByText('Berlin')).not.toBeInTheDocument()
  })

  it('équipe active définie (mode multi) : son nom apparaît dans l\'encart', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STARTED',
        RAFALE_CURRENT_QUESTION: { ID: 'r-042', QUESTION: 'Q?', CATEGORY: 'HISTORY', DIFFICULTY: 1 },
        RAFALE_CURRENT_TEAM: 'Équipe A',
        RAFALE_CURRENT_TEAM_COLOR: [99, 102, 241],
      },
    }))
    const { container } = render(<GamePage />)

    const chip = container.querySelector('.rafale-anim-qcard-team')
    expect(chip).not.toBeNull()
    expect(chip.textContent).toContain('Équipe A')
  })

  it('question non-RAFALE, phase STARTED : .rafale-admin-live n\'est jamais affiché (non-régression)', () => {
    useGame.mockReturnValue(makeGameMock({
      gameState: { phase: 'STARTED', question: { ID: '1', TYPE: 'SPEEDY', STATUS: 'STARTED' } },
    }))
    const { container } = render(<GamePage />)

    expect(container.querySelector('.rafale-admin-live')).toBeNull()
  })
})
