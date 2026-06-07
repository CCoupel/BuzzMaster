/**
 * Tests for QuestionsPage — v5.7.1 features (issues #97 #98)
 *
 * #97 — Bouton + catégorie : création inline depuis QuestionsPage
 * #98 — Type "Speedy" (anciennement "Normal") : bouton et état par défaut
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import QuestionsPage from './QuestionsPage'

// ---------------------------------------------------------------------------
// Mocks — suivre le même pattern que QuestionsPage.ardoise.test.jsx
// ---------------------------------------------------------------------------

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
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
    <div
      data-testid={`qcard-${question.ID}`}
      onClick={() => onClick && onClick(question)}
    />
  ),
  CATEGORIES: {
    GEOGRAPHY: { label: 'Geographie', icon: '🌍', color: '#3b82f6' },
  },
}))

vi.mock('./QuestionsPage.css', () => ({}))
vi.mock('./ConfigPage.css', () => ({}))

// ---------------------------------------------------------------------------
// Imports après mocks
// ---------------------------------------------------------------------------
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'

// ---------------------------------------------------------------------------
// Helpers
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

// ---------------------------------------------------------------------------
// #98 — Type "Speedy" (anciennement "Normal")
// ---------------------------------------------------------------------------

describe('QuestionsPage — Type Speedy (#98)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useGame.mockReturnValue(makeQPageMock())
    useCategories.mockReturnValue(makeCategoriesMock())
    global.fetch = vi.fn()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('le bouton "Speedy" est présent dans le sélecteur de type', () => {
    render(<QuestionsPage />)
    expect(screen.getByText('Speedy')).toBeInTheDocument()
  })

  it('le bouton Speedy est actif par défaut (type initial = SPEEDY)', () => {
    render(<QuestionsPage />)
    const speedyBtn = screen.getByText('Speedy').closest('button')
    expect(speedyBtn).toHaveClass('active')
  })

  it('le bouton Speedy porte les classes attendues', () => {
    render(<QuestionsPage />)
    const speedyBtn = screen.getByText('Speedy').closest('button')
    expect(speedyBtn).toHaveClass('type-btn')
  })

  it('cliquer sur QCM retire la classe active du bouton Speedy', () => {
    render(<QuestionsPage />)
    const qcmBtn = screen.getByText(/qcm/i).closest('button')
    fireEvent.click(qcmBtn)

    const speedyBtn = screen.getByText('Speedy').closest('button')
    expect(speedyBtn).not.toHaveClass('active')
  })

  it('cliquer à nouveau sur Speedy le remet actif', () => {
    render(<QuestionsPage />)
    // Passer sur QCM d'abord
    const qcmBtn = screen.getByText(/qcm/i).closest('button')
    fireEvent.click(qcmBtn)

    // Revenir sur Speedy
    const speedyBtn = screen.getByText('Speedy').closest('button')
    fireEvent.click(speedyBtn)
    expect(speedyBtn).toHaveClass('active')
  })
})

// ---------------------------------------------------------------------------
// #97 — Bouton + catégorie (création inline)
// ---------------------------------------------------------------------------

describe('QuestionsPage — Bouton + catégorie (#97)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useGame.mockReturnValue(makeQPageMock())
    useCategories.mockReturnValue(makeCategoriesMock())
    global.fetch = vi.fn()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('le bouton "+" est présent dans le sélecteur de catégorie', () => {
    const { container } = render(<QuestionsPage />)
    const addBtn = container.querySelector('.category-btn--add')
    expect(addBtn).not.toBeNull()
  })

  it('cliquer sur "+" affiche le formulaire inline d\'ajout de catégorie', () => {
    const { container } = render(<QuestionsPage />)
    const addBtn = container.querySelector('.category-btn--add')
    fireEvent.click(addBtn)

    const form = container.querySelector('.add-category-inline')
    expect(form).not.toBeNull()
  })

  it('le formulaire inline contient un champ texte et les boutons Valider/Annuler', () => {
    const { container } = render(<QuestionsPage />)
    const addBtn = container.querySelector('.category-btn--add')
    fireEvent.click(addBtn)

    const input = container.querySelector('.add-category-input')
    expect(input).not.toBeNull()

    const validateBtn = container.querySelector('.add-category-validate')
    expect(validateBtn).not.toBeNull()

    const cancelBtn = container.querySelector('.add-category-cancel')
    expect(cancelBtn).not.toBeNull()
  })

  it('cliquer sur "Annuler" masque le formulaire inline', () => {
    const { container } = render(<QuestionsPage />)
    const addBtn = container.querySelector('.category-btn--add')
    fireEvent.click(addBtn)

    const cancelBtn = container.querySelector('.add-category-cancel')
    fireEvent.click(cancelBtn)

    const form = container.querySelector('.add-category-inline')
    expect(form).toBeNull()

    // Le bouton "+" est de nouveau visible
    const addBtnAfter = container.querySelector('.category-btn--add')
    expect(addBtnAfter).not.toBeNull()
  })

  it('une erreur 400 (nom invalide) affiche le message d\'erreur attendu', async () => {
    global.fetch = vi.fn().mockResolvedValueOnce({
      ok: false,
      status: 400,
    })

    const { container } = render(<QuestionsPage />)
    const addBtn = container.querySelector('.category-btn--add')
    fireEvent.click(addBtn)

    const input = container.querySelector('.add-category-input')
    fireEvent.change(input, { target: { value: '' } }) // nom vide

    const validateBtn = container.querySelector('.add-category-validate')
    fireEvent.click(validateBtn)

    // Le message d'erreur "Nom invalide" doit apparaître
    await waitFor(() => {
      const errorEl = container.querySelector('.add-category-error')
      expect(errorEl).not.toBeNull()
      expect(errorEl.textContent).toMatch(/nom invalide/i)
    })
  })

  it('une erreur 409 (conflit) affiche "Cette catégorie existe déjà"', async () => {
    global.fetch = vi.fn().mockResolvedValueOnce({
      ok: false,
      status: 409,
    })

    const { container } = render(<QuestionsPage />)
    const addBtn = container.querySelector('.category-btn--add')
    fireEvent.click(addBtn)

    const input = container.querySelector('.add-category-input')
    fireEvent.change(input, { target: { value: 'Geography' } })

    const validateBtn = container.querySelector('.add-category-validate')
    fireEvent.click(validateBtn)

    await waitFor(() => {
      const errorEl = container.querySelector('.add-category-error')
      expect(errorEl).not.toBeNull()
      expect(errorEl.textContent).toMatch(/existe déjà/i)
    })
  })

  it('un POST réussi ferme le formulaire et appelle refetch', async () => {
    const mockRefetch = vi.fn()
    useCategories.mockReturnValue(makeCategoriesMock({ refetch: mockRefetch }))

    global.fetch = vi.fn().mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({
        key: 'MA_CATEGORIE',
        name: 'Ma Categorie',
        imageURL: '',
        isCustom: true,
      }),
    })

    const { container } = render(<QuestionsPage />)
    const addBtn = container.querySelector('.category-btn--add')
    fireEvent.click(addBtn)

    const input = container.querySelector('.add-category-input')
    fireEvent.change(input, { target: { value: 'Ma Categorie' } })

    const validateBtn = container.querySelector('.add-category-validate')
    fireEvent.click(validateBtn)

    await waitFor(() => {
      // Le formulaire doit être fermé
      const form = container.querySelector('.add-category-inline')
      expect(form).toBeNull()
    })

    expect(mockRefetch).toHaveBeenCalledTimes(1)
  })
})
