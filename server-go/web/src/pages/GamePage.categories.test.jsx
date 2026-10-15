/**
 * Tests for issue #40 — Category filters in GamePage
 *
 * These tests require a non-empty CATEGORIES mock so that category pills
 * are actually rendered (unlike GamePage.test.jsx which uses CATEGORIES: {}).
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'

// ---------------------------------------------------------------------------
// Mocks — same as GamePage.test.jsx but with real CATEGORIES entries
// ---------------------------------------------------------------------------

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
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

// CATEGORIES non vide pour que la barre de filtres s'affiche
vi.mock('../components/QuestionCard', () => ({
  default: ({ question, onClick }) => (
    <div
      data-testid={`question-card-${question.ID}`}
      data-category={question.CATEGORY || ''}
      onClick={() => onClick && onClick(question)}
    />
  ),
  CATEGORIES: {
    GEOGRAPHY: { label: 'Geographie', icon: '🌍', color: '#3b82f6' },
    SCIENCE: { label: 'Sciences', icon: '🔬', color: '#22c55e' },
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

vi.mock('./GamePage.css', () => ({}))

import { useGame } from '../hooks/GameContext'
import GamePage from './GamePage'

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

const makeGameMock = (overrides = {}) => ({
  gameState: {
    phase: 'STOPPED',
    question: { ID: '1', STATUS: 'STOPPED' },
    remote: 'GAME',
    timer: 0,
    totalTime: 30,
    MEMORY_PARTICIPATING_TEAMS: [],
    ...overrides.gameState,
  },
  teams: overrides.teams ?? {},
  bumpers: overrides.bumpers ?? {},
  questions: overrides.questions ?? {
    '1': { ID: '1', STATUS: 'STOPPED', ORDER: 1, CATEGORY: 'GEOGRAPHY' },
    '2': { ID: '2', STATUS: 'AVAILABLE', ORDER: 2, CATEGORY: 'SCIENCE' },
    '3': { ID: '3', STATUS: 'AVAILABLE', ORDER: 3, CATEGORY: 'GEOGRAPHY' },
    '4': { ID: '4', STATUS: 'AVAILABLE', ORDER: 4 },
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
  ...overrides,
})

// ---------------------------------------------------------------------------
// Tests — issue #40 : filtres catégories
// ---------------------------------------------------------------------------

describe('GamePage - Filtres catégories (#40)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('toutes les questions s\'affichent quand aucun filtre n\'est actif', () => {
    useGame.mockReturnValue(makeGameMock())
    render(<GamePage />)

    expect(screen.getByTestId('question-card-1')).toBeInTheDocument()
    expect(screen.getByTestId('question-card-2')).toBeInTheDocument()
    expect(screen.getByTestId('question-card-3')).toBeInTheDocument()
    expect(screen.getByTestId('question-card-4')).toBeInTheDocument()
  })

  it('la barre de filtres s\'affiche si des catégories existent dans les questions', () => {
    useGame.mockReturnValue(makeGameMock())
    render(<GamePage />)

    // #221 : le filtre est le CategoryPicker variant="filter" — on cible ses
    // pastilles par `title` (libellé) et `aria-pressed`, jamais par classe CSS.
    expect(screen.getByTitle('Geographie')).toBeInTheDocument()
    expect(screen.getByTitle('Sciences & Nature')).toBeInTheDocument()
  })

  it('la barre de filtres est absente si aucune question n\'a de catégorie', () => {
    useGame.mockReturnValue(makeGameMock({
      questions: {
        '1': { ID: '1', STATUS: 'AVAILABLE', ORDER: 1 },
        '2': { ID: '2', STATUS: 'AVAILABLE', ORDER: 2 },
      },
    }))
    render(<GamePage />)

    // Aucune pastille de catégorie, aucun bouton de réinitialisation.
    for (const label of ['Geographie', 'Divertissement', 'Histoire', 'Arts & Litterature', 'Sciences & Nature', 'Sports & Loisirs', 'Gastronomie', 'Animaux']) {
      expect(screen.queryByTitle(label)).toBeNull()
    }
    expect(screen.queryByTitle('Réinitialiser les filtres')).toBeNull()
  })

  it('cliquer sur une catégorie filtre les questions : seules GEOGRAPHY restent visibles', () => {
    useGame.mockReturnValue(makeGameMock())
    render(<GamePage />)

    // Trouver et cliquer sur la pastille GEOGRAPHY (title = libellé)
    const geoPill = screen.getByTitle('Geographie')
    expect(geoPill).toHaveAttribute('aria-pressed', 'false')
    fireEvent.click(geoPill)
    expect(screen.getByTitle('Geographie')).toHaveAttribute('aria-pressed', 'true')

    // Questions GEOGRAPHY visibles
    expect(screen.getByTestId('question-card-1')).toBeInTheDocument()
    expect(screen.getByTestId('question-card-3')).toBeInTheDocument()

    // Question SCIENCE masquée
    expect(screen.queryByTestId('question-card-2')).toBeNull()

    // Question sans catégorie masquée
    expect(screen.queryByTestId('question-card-4')).toBeNull()
  })

  it('le bouton reset (×) apparaît après activation d\'un filtre', () => {
    useGame.mockReturnValue(makeGameMock())
    render(<GamePage />)

    // Avant : pas de bouton reset
    expect(screen.queryByTitle('Réinitialiser les filtres')).toBeNull()

    // Activer un filtre
    fireEvent.click(screen.getByTitle('Geographie'))

    // Après : bouton reset visible
    expect(screen.getByTitle('Réinitialiser les filtres')).toBeInTheDocument()
  })

  it('cliquer sur reset supprime les filtres et réaffiche toutes les questions', () => {
    useGame.mockReturnValue(makeGameMock())
    render(<GamePage />)

    // Activer filtre
    fireEvent.click(screen.getByTitle('Geographie'))

    // Vérifier filtre actif (question-card-2 masquée)
    expect(screen.queryByTestId('question-card-2')).toBeNull()

    // Cliquer reset
    fireEvent.click(screen.getByTitle('Réinitialiser les filtres'))

    // Toutes questions affichées
    expect(screen.getByTestId('question-card-2')).toBeInTheDocument()
    expect(screen.getByTestId('question-card-4')).toBeInTheDocument()
  })

  it('cliquer deux fois sur la même catégorie désactive le filtre', () => {
    useGame.mockReturnValue(makeGameMock())
    render(<GamePage />)

    // Activer
    fireEvent.click(screen.getByTitle('Geographie'))
    expect(screen.queryByTestId('question-card-2')).toBeNull()

    // Désactiver
    fireEvent.click(screen.getByTitle('Geographie'))
    expect(screen.getByTestId('question-card-2')).toBeInTheDocument()
  })

  it('message vide s\'affiche quand le filtre ne correspond à aucune question', () => {
    // Scénario : on rend avec des questions GEOGRAPHY + SCIENCE (les 2 pills visibles),
    // on active le filtre GEOGRAPHY, puis on re-rend avec uniquement des questions SCIENCE.
    // Le filtre GEOGRAPHY reste actif dans le state React mais aucune question ne correspond
    // => le composant affiche "Aucune question dans cette categorie."
    useGame.mockReturnValue(makeGameMock({
      questions: {
        '1': { ID: '1', STATUS: 'AVAILABLE', ORDER: 1, CATEGORY: 'GEOGRAPHY' },
        '2': { ID: '2', STATUS: 'AVAILABLE', ORDER: 2, CATEGORY: 'SCIENCE' },
      },
    }))
    const { rerender } = render(<GamePage />)

    // Activer la pastille GEOGRAPHY (title="Geographie")
    const geoPill = screen.getByTitle('Geographie')
    expect(geoPill).toBeInTheDocument()
    fireEvent.click(geoPill)

    // Re-render avec uniquement des questions SCIENCE — filtre GEOGRAPHY toujours actif
    useGame.mockReturnValue(makeGameMock({
      questions: {
        '2': { ID: '2', STATUS: 'AVAILABLE', ORDER: 2, CATEGORY: 'SCIENCE' },
      },
    }))
    rerender(<GamePage />)

    // Message "Aucune question dans cette categorie." visible
    expect(screen.getByText(/aucune question dans cette cat/i)).toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// Tests — issue #43 : badge version cliquable (logique pure)
// ---------------------------------------------------------------------------

describe('Navbar — badge version cliquable (#43) — logique', () => {
  it('badge avec version vide affiche "v..."', () => {
    const serverVersion = ''
    const display = `v${serverVersion || '...'}`
    expect(display).toBe('v...')
  })

  it('badge avec version réelle affiche "v3.7.0"', () => {
    const serverVersion = '3.7.0'
    const display = `v${serverVersion || '...'}`
    expect(display).toBe('v3.7.0')
  })

  // #155 (F2, BREAKING) — currentPrefix is a constant '/admin' now, not
  // derived from the URL anymore (Navbar.jsx only ever renders on /admin/*,
  // /anim is its own page — AnimPage — that never shows this navbar). The
  // sibling case that used to test a '/anim' prefix here no longer applies
  // and was removed rather than adapted.
  it('la destination navigate est construite avec le bon chemin', () => {
    const currentPrefix = '/admin'
    const getFullPath = (path) => path ? `${currentPrefix}/${path}` : currentPrefix
    expect(getFullPath('updates')).toBe('/admin/updates')
  })
})
