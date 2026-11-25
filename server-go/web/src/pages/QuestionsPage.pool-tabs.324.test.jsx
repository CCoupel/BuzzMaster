/**
 * LOT-2 (v13.1.0, #324) — onglets de QuestionsPage et libellés Pool Speedy / Pool Picto (picto-pool.md §6).
 * Squelette contract-first : ROUGE tant que T13/T15 (dev-frontend) ne sont pas livrés. Même harnais de mocks que
 * QuestionsPage.rafale.test.jsx.
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import QuestionsPage from './QuestionsPage'

// ---------------------------------------------------------------------------
// Mocks — même patron que QuestionsPage.v571.test.jsx
// ---------------------------------------------------------------------------

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

// #215 — QuestionsPage appelle désormais useNavigate()/useSearchParams()
// (onglets Questions/Rafale, lien "configurer le quiz" → navigation vers
// Backstage) : ces tests ne montent aucun Router réel, donc mock plutôt que
// de faire porter <MemoryRouter> à chacun des (nombreux) render() ci-dessous.
vi.mock('react-router-dom', () => ({
  useNavigate: () => vi.fn(),
  useSearchParams: () => [new URLSearchParams(), vi.fn()],
}))

vi.mock('../hooks/useCategoryFilter', () => ({
  useCategoryFilter: vi.fn((questions) => ({
    selectedCategories: new Set(),
    availableCategories: [],
    filteredQuestions: questions || [],
    toggleCategoryFilter: vi.fn(),
    clearCategoryFilters: vi.fn(),
  })),
}))

vi.mock('../hooks/useCategories', () => ({
  useCategories: vi.fn(),
}))

vi.mock('../components/Button', () => ({
  default: ({ children, onClick, disabled, type, ...rest }) => (
    <button onClick={onClick} disabled={disabled} type={type || 'button'} {...rest}>
      {children}
    </button>
  ),
}))

vi.mock('../components/Card', () => ({
  default: ({ children, className, padding, variant, ...rest }) => (
    <div className={className} {...rest}>{children}</div>
  ),
  CardHeader: ({ children }) => <div className="card-header">{children}</div>,
  CardBody: ({ children }) => <div className="card-body">{children}</div>,
}))

vi.mock('../components/CategoryBalance', () => ({
  default: () => null,
}))

vi.mock('../components/QuestionCard', () => ({
  default: ({ question, onClick }) => (
    <div data-testid={`qcard-${question.ID}`} onClick={() => onClick && onClick(question)} />
  ),
  CATEGORIES: {
    GEOGRAPHY: { label: 'Geographie', icon: '🌍', color: '#3b82f6' },
  },
}))

// RafalePoolAlert fait son propre fetch (GET /api/rafale/pool) — déjà
// couvert isolément par components/RafalePoolAlert.rafale.test.jsx.
// Stub léger ici pour ne pas polluer ces tests (centrés sur le sélecteur
// de mode) avec du fetch asynchrone hors-sujet.
vi.mock('../components/RafalePoolAlert', () => ({
  default: () => <div data-testid="rafale-pool-alert-stub" />,
}))

vi.mock('./QuestionsPage.css', () => ({}))
vi.mock('./ConfigPage.css', () => ({}))

import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

// ---------------------------------------------------------------------------
// Helpers — identiques à QuestionsPage.v571.test.jsx
// ---------------------------------------------------------------------------

const makeQPageMock = (overrides = {}) => ({
  questions: overrides.questions ?? {},
  fsInfo: { used: 0, total: 100 },
  deleteQuestion: vi.fn(),
  sendMessage: vi.fn(),
  gameState: {
    phase: 'STOPPED',
    question: null,
    ...overrides.gameState,
  },
  newGame: vi.fn(),
  ...overrides,
})

const makeCategoriesMock = (overrides = {}) => ({
  categories: overrides.categories ?? [
    { key: 'GEOGRAPHY', name: 'Geographie', imageURL: '', isCustom: false },
  ],
  loading: overrides.loading ?? false,
  error: overrides.error ?? null,
  refetch: overrides.refetch ?? vi.fn(),
})


describe('QuestionsPage — onglets Pool Speedy / Pool Picto (#324, picto-pool.md §6, plan « Renommage »)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useGame.mockReturnValue(makeQPageMock())
    useCategories.mockReturnValue(makeCategoriesMock())
    global.fetch = vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve({ QUESTIONS: [], TOTAL: 0 }) }))
  })

  it('trois onglets : Questions, 🌀 Pool Speedy, ✏️ Pool Picto', () => {
    render(<QuestionsPage />)
    const tabs = screen.getAllByRole('tab').map((t) => t.textContent)
    expect(tabs.some((t) => /Questions/.test(t))).toBe(true)
    expect(tabs.some((t) => /Pool Speedy/.test(t))).toBe(true)
    expect(tabs.some((t) => /Pool Picto/.test(t))).toBe(true)
  })

  it('l’ancien onglet « Rafale » seul a disparu', () => {
    render(<QuestionsPage />)
    expect(screen.queryByRole('tab', { name: /^🌀?\s*Rafale$/ })).toBeNull()
  })

  it('le bouton de type du sélecteur s’appelle « Rafale Speedy »', () => {
    render(<QuestionsPage />)
    expect(screen.getByRole('button', { name: 'Rafale Speedy' })).toBeInTheDocument()
  })

  it('cliquer l’onglet Pool Picto affiche une page de pool dont le titre est « Pool Picto »', async () => {
    render(<QuestionsPage />)
    fireEvent.click(screen.getByRole('tab', { name: /Pool Picto/ }))
    expect(await screen.findByRole('heading', { name: /Pool Picto/ })).toBeInTheDocument()
  })

  it('cliquer l’onglet Pool Speedy affiche une page de pool dont le titre est « Pool Speedy » (plus « Réservoir RAFALE »)', async () => {
    render(<QuestionsPage />)
    fireEvent.click(screen.getByRole('tab', { name: /Pool Speedy/ }))
    expect(await screen.findByRole('heading', { name: /Pool Speedy/ })).toBeInTheDocument()
    expect(screen.queryByText(/Réservoir RAFALE/)).toBeNull()
  })
})
