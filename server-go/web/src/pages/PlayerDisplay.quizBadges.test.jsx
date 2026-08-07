/**
 * Tests — PlayerDisplay : badges Publics/Difficultés/Langue de l'écran
 * NEW_GAME, masquage QUIZ_HIDDEN_FIELDS + plafonnement (#137 Batch 2b T2.4).
 *
 * Contexte (_work/handoff/task-test-writer-review-batch2b-20260806-165427.md) :
 * dev-frontend a vérifié le cas "les quatre champs masqués" par LECTURE du
 * JSX seulement, sans test automatisé (cf. son handoff, section T2.4) — ce
 * fichier comble ce trou, ainsi que le plafonnement à 2+N avec le jeu de
 * valeurs complet (5 publics + 4 difficultés) et l'ordre masquage AVANT
 * plafonnement (contract game-state.md, PlayerDisplay.jsx quizBadgeFamilies).
 *
 * Suit le pattern de mocks de PlayerDisplay.qrcode.test.jsx.
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import PlayerDisplay from './PlayerDisplay'

// ---------------------------------------------------------------------------
// Mocks — identiques à PlayerDisplay.qrcode.test.jsx
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
  default: ({ url }) => <div data-testid="qr-display" data-url={url} />,
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

// ---------------------------------------------------------------------------
// Données de test
// ---------------------------------------------------------------------------

const BASE_GAME_STATE = {
  phase: 'NEW_GAME',
  remote: null,
  question: null,
  timer: 0,
  totalTime: 0,
  backgrounds: [],
  currentBackgroundIndex: 0,
  newGameBackgrounds: [],
  memoryMatchedPairs: [],
  neonEffect: { enabled: false },
  quizName: '',
  quizTheme: '',
  quizNotes: '',
  quizPopulations: [],
  quizDifficulties: [],
  quizLanguage: '',
  quizHiddenFields: [],
}

function mockUseGame(overrides = {}) {
  useGame.mockReturnValue({
    gameState: { ...BASE_GAME_STATE, ...overrides },
    teams: {},
    bumpers: {},
    flipMemoryCard: vi.fn(),
    showQRCode: vi.fn(),
    selectMotionCard: vi.fn(),
  })
}

const ALL_POPULATIONS = ['Junior (6-12 ans)', 'Ado (13-17 ans)', 'Adulte (18-64 ans)', 'Senior (65+ ans)', 'Famille']
const ALL_DIFFICULTIES = ['Facile', 'Moyen', 'Difficile', 'Expert']

describe('PlayerDisplay — écran NEW_GAME : badges Quiz (#137 Batch 2b T2.4)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('un champ masqué (DIFFICULTIES) est totalement absent des badges, les autres champs restent', () => {
    mockUseGame({
      quizPopulations: ['Adulte (18-64 ans)'],
      quizDifficulties: ['Moyen'],
      quizLanguage: 'Français',
      quizHiddenFields: ['DIFFICULTIES'],
    })
    render(<PlayerDisplay />)

    expect(screen.getByText('Adulte (18-64 ans)')).toBeInTheDocument()
    expect(screen.getByText('Français')).toBeInTheDocument()
    expect(screen.queryByText('Moyen')).not.toBeInTheDocument()
  })

  it('les quatre champs masqués (THEME/POPULATIONS/DIFFICULTIES/LANGUAGE) : la rangée de badges disparaît entièrement, aucun bloc vide résiduel', () => {
    mockUseGame({
      quizTheme: 'Cinéma',
      quizPopulations: ['Adulte (18-64 ans)'],
      quizDifficulties: ['Moyen'],
      quizLanguage: 'Français',
      quizHiddenFields: ['THEME', 'POPULATIONS', 'DIFFICULTIES', 'LANGUAGE'],
    })
    const { container } = render(<PlayerDisplay />)

    // Aucune des valeurs masquées ne doit apparaître…
    expect(screen.queryByText('Adulte (18-64 ans)')).not.toBeInTheDocument()
    expect(screen.queryByText('Moyen')).not.toBeInTheDocument()
    expect(screen.queryByText('Français')).not.toBeInTheDocument()
    // …et le conteneur de badges lui-même ne doit pas être monté du tout —
    // pas de <div className="new-game-badges"> vide résiduel (dev-frontend
    // n'avait vérifié ce point que par lecture du JSX, pas de test).
    expect(container.querySelector('.new-game-badges')).toBeNull()
    // Le thème étant masqué, le bloc dédié ne doit pas non plus apparaître.
    expect(container.querySelector('.new-game-quiz-theme')).toBeNull()
  })

  it('plafonne à 2 par famille visible ("+N" pour le surplus) avec le jeu complet — 5 publics, 4 difficultés', () => {
    mockUseGame({
      quizPopulations: ALL_POPULATIONS,
      quizDifficulties: ALL_DIFFICULTIES,
      quizLanguage: 'Français',
      quizHiddenFields: [],
    })
    const { container } = render(<PlayerDisplay />)

    // Seules les 2 premières valeurs de chaque famille sont rendues comme
    // badge simple — le surplus est résumé par un badge "+N" agrégé.
    expect(screen.getByText('Junior (6-12 ans)')).toBeInTheDocument()
    expect(screen.getByText('Ado (13-17 ans)')).toBeInTheDocument()
    expect(screen.queryByText('Adulte (18-64 ans)')).not.toBeInTheDocument()
    expect(screen.queryByText('Senior (65+ ans)')).not.toBeInTheDocument()
    expect(screen.queryByText('Famille')).not.toBeInTheDocument()

    expect(screen.getByText('Facile')).toBeInTheDocument()
    expect(screen.getByText('Moyen')).toBeInTheDocument()
    expect(screen.queryByText('Difficile')).not.toBeInTheDocument()
    expect(screen.queryByText('Expert')).not.toBeInTheDocument()

    // 5 publics -> 2 affichés + "+3" ; 4 difficultés -> 2 affichées + "+2".
    expect(screen.getByText('+3')).toBeInTheDocument()
    expect(screen.getByText('+2')).toBeInTheDocument()

    const moreBadges = container.querySelectorAll('.new-game-badge-more')
    expect(moreBadges).toHaveLength(2)
  })

  it('masquage AVANT plafonnement : une famille masquée ne laisse RIEN passer, même plafonné, alors qu\'une famille visible avec le même volume est bien plafonnée à 2+N', () => {
    mockUseGame({
      quizPopulations: ALL_POPULATIONS, // visible, 5 valeurs -> attendu 2 + "+3"
      quizDifficulties: ALL_DIFFICULTIES, // masquée, 4 valeurs -> attendu 0 badge, pas même "2 + +2"
      quizLanguage: 'Français',
      quizHiddenFields: ['DIFFICULTIES'],
    })
    render(<PlayerDisplay />)

    // La famille visible est bien plafonnée (comportement normal, non affecté
    // par le masquage d'une AUTRE famille).
    expect(screen.getByText('Junior (6-12 ans)')).toBeInTheDocument()
    expect(screen.getByText('Ado (13-17 ans)')).toBeInTheDocument()
    expect(screen.getByText('+3')).toBeInTheDocument()

    // La famille masquée ne doit produire NI badge simple NI badge "+N" — si
    // le masquage était appliqué APRÈS un plafonnement qui, par erreur,
    // ignorerait quizHiddenFields, on verrait ici "Facile"/"Moyen" + "+2".
    for (const d of ALL_DIFFICULTIES) {
      expect(screen.queryByText(d)).not.toBeInTheDocument()
    }
    expect(screen.queryByText('+2')).not.toBeInTheDocument()
  })
})
