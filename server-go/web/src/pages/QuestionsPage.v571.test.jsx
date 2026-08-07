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

  // Helper : sélectionner un fichier image dans le formulaire inline
  // Nécessaire depuis v5.7.2 — le champ file est obligatoire (BREAKING #100)
  const selectCategoryFile = (container, filename = 'photo.png') => {
    const fileInput = container.querySelector('.add-category-inline input[type="file"]')
    if (!fileInput) return
    const file = new File(['fake-image'], filename, { type: 'image/png' })
    fireEvent.change(fileInput, { target: { files: [file] } })
  }

  it('sans image sélectionnée, le formulaire affiche "Image requise" (v5.7.2 #100)', async () => {
    const { container } = render(<QuestionsPage />)
    const addBtn = container.querySelector('.category-btn--add')
    fireEvent.click(addBtn)

    const input = container.querySelector('.add-category-input')
    fireEvent.change(input, { target: { value: 'Bonne Catégorie' } })

    // Cliquer Valider sans sélectionner de fichier
    const validateBtn = container.querySelector('.add-category-validate')
    fireEvent.click(validateBtn)

    await waitFor(() => {
      const errorEl = container.querySelector('.add-category-error')
      expect(errorEl).not.toBeNull()
      expect(errorEl.textContent).toMatch(/image requise/i)
    })
  })

  it('le formulaire contient un input file acceptant les images (v5.7.2 #100)', () => {
    const { container } = render(<QuestionsPage />)
    const addBtn = container.querySelector('.category-btn--add')
    fireEvent.click(addBtn)

    const fileInput = container.querySelector('.add-category-inline input[type="file"]')
    expect(fileInput).not.toBeNull()
    expect(fileInput.getAttribute('accept')).toContain('.png')
    expect(fileInput.getAttribute('accept')).toContain('.jpg')
  })

  it('une erreur 409 (conflit) affiche "Cette catégorie existe déjà"', async () => {
    // v5.7.2 BREAKING: POST /api/categories requiert multipart + fichier image obligatoire
    // v6.0.0 (#8): QuestionsPage appelle aussi GET /config.json au montage
    // (état de la clé IA) — router par URL plutôt que mockResolvedValueOnce
    // pour ne pas dépendre de l'ordre des appels fetch.
    global.fetch = vi.fn((url) => {
      if (url === '/config.json') {
        return Promise.resolve({ ok: true, json: () => Promise.resolve({ ai: { api_key_configured: false } }) })
      }
      return Promise.resolve({ ok: false, status: 409 })
    })

    const { container } = render(<QuestionsPage />)
    const addBtn = container.querySelector('.category-btn--add')
    fireEvent.click(addBtn)

    const input = container.querySelector('.add-category-input')
    fireEvent.change(input, { target: { value: 'Geography' } })

    // Sélectionner un fichier (obligatoire depuis v5.7.2)
    selectCategoryFile(container)

    const validateBtn = container.querySelector('.add-category-validate')
    fireEvent.click(validateBtn)

    await waitFor(() => {
      const errorEl = container.querySelector('.add-category-error')
      expect(errorEl).not.toBeNull()
      expect(errorEl.textContent).toMatch(/existe déjà/i)
    })
  })

  it('un POST réussi ferme le formulaire et appelle refetch', async () => {
    // v5.7.2 BREAKING: POST /api/categories requiert multipart + fichier image obligatoire
    const mockRefetch = vi.fn()
    useCategories.mockReturnValue(makeCategoriesMock({ refetch: mockRefetch }))

    // v6.0.0 (#8): QuestionsPage appelle aussi GET /config.json au montage
    // (état de la clé IA) — router par URL plutôt que mockResolvedValueOnce
    // pour ne pas dépendre de l'ordre des appels fetch.
    global.fetch = vi.fn((url) => {
      if (url === '/config.json') {
        return Promise.resolve({ ok: true, json: () => Promise.resolve({ ai: { api_key_configured: false } }) })
      }
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve({
          key: 'MA_CATEGORIE',
          name: 'Ma Categorie',
          imageURL: '/files/categories/MA_CATEGORIE.png',
          isCustom: true,
        }),
      })
    })

    const { container } = render(<QuestionsPage />)
    const addBtn = container.querySelector('.category-btn--add')
    fireEvent.click(addBtn)

    const input = container.querySelector('.add-category-input')
    fireEvent.change(input, { target: { value: 'Ma Categorie' } })

    // Sélectionner un fichier (obligatoire depuis v5.7.2)
    selectCategoryFile(container, 'ma-categorie.png')

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

// ---------------------------------------------------------------------------
// #101 — Couleurs boutons type = couleurs badges (classes CSS)
// ---------------------------------------------------------------------------

describe('QuestionsPage — Couleurs boutons type (#101)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useGame.mockReturnValue(makeQPageMock())
    useCategories.mockReturnValue(makeCategoriesMock())
    global.fetch = vi.fn()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('le bouton SPEEDY possède la classe CSS "speedy" (pour ciblage couleur)', () => {
    render(<QuestionsPage />)
    const speedyBtn = screen.getByText('Speedy').closest('button')
    expect(speedyBtn).toHaveClass('speedy')
    expect(speedyBtn).toHaveClass('type-btn')
  })

  it('le bouton QCM possède la classe CSS "qcm"', () => {
    render(<QuestionsPage />)
    const qcmBtn = screen.getByText(/qcm/i).closest('button')
    expect(qcmBtn).toHaveClass('qcm')
    expect(qcmBtn).toHaveClass('type-btn')
  })

  it('le bouton MEMORY possède la classe CSS "memory"', () => {
    render(<QuestionsPage />)
    const memoryBtn = screen.getByText(/memory/i).closest('button')
    expect(memoryBtn).toHaveClass('memory')
    expect(memoryBtn).toHaveClass('type-btn')
  })

  it('le bouton MEMOTION possède la classe CSS "memotion"', () => {
    render(<QuestionsPage />)
    const memotionBtn = screen.getByText(/memotion/i).closest('button')
    expect(memotionBtn).toHaveClass('memotion')
    expect(memotionBtn).toHaveClass('type-btn')
  })

  it('le bouton ARDOISE possède la classe CSS "ardoise"', () => {
    render(<QuestionsPage />)
    const ardoiseBtn = screen.getByText(/ardoise/i).closest('button')
    expect(ardoiseBtn).toHaveClass('ardoise')
    expect(ardoiseBtn).toHaveClass('type-btn')
  })

  it('chaque bouton type actif possède uniquement sa propre classe "active"', () => {
    render(<QuestionsPage />)

    // Cliquer QCM
    const qcmBtn = screen.getByText(/qcm/i).closest('button')
    fireEvent.click(qcmBtn)
    expect(qcmBtn).toHaveClass('active')

    // Les autres ne doivent pas être actifs
    const speedyBtn = screen.getByText('Speedy').closest('button')
    expect(speedyBtn).not.toHaveClass('active')
  })
})

// ---------------------------------------------------------------------------
// #103 — État non-sélectionné = couleur badge subtile (classes CSS)
// Vérifie que les boutons MEMOTION et ARDOISE conservent leur classe de type
// même lorsqu'ils ne sont pas actifs (nécessaire pour le ciblage CSS couleur badge).
// ---------------------------------------------------------------------------

describe('QuestionsPage — État non-sélectionné (#103)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useGame.mockReturnValue(makeQPageMock())
    useCategories.mockReturnValue(makeCategoriesMock())
    global.fetch = vi.fn()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('bouton MEMOTION non-sélectionné conserve la classe "memotion" (CSS couleur badge)', () => {
    render(<QuestionsPage />)
    // Par défaut, Speedy est actif → Memotion n'est PAS actif
    const memotionBtn = screen.getByText(/memotion/i).closest('button')
    expect(memotionBtn).not.toHaveClass('active')
    expect(memotionBtn).toHaveClass('memotion')
  })

  it('bouton ARDOISE non-sélectionné conserve la classe "ardoise" (CSS couleur badge)', () => {
    render(<QuestionsPage />)
    // Par défaut, Speedy est actif → Ardoise n'est PAS actif
    const ardoiseBtn = screen.getByText(/ardoise/i).closest('button')
    expect(ardoiseBtn).not.toHaveClass('active')
    expect(ardoiseBtn).toHaveClass('ardoise')
  })
})
