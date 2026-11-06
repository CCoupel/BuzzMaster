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
// #97 → #285 — la création inline (« + ») a été retirée du sélecteur de catégorie par décision utilisateur
// (retour QUALIF 11.4.0.5) : la création vit dans CategoryManagerDialog (CategoryManagerDialog.create285.test.jsx).
// Les 9 anciens cas du bloc « Bouton + catégorie (#97) » sont supprimés ; il ne reste que l'absence du bouton.
// ---------------------------------------------------------------------------

describe('QuestionsPage — pas de création de catégorie dans l\'éditeur (#285)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useGame.mockReturnValue(makeQPageMock())
    useCategories.mockReturnValue(makeCategoriesMock())
    global.fetch = vi.fn()
  })

  it('aucun bouton « Créer une catégorie » dans le sélecteur de l\'éditeur', () => {
    render(<QuestionsPage />)
    expect(screen.queryByTitle('Créer une catégorie')).toBeNull()
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
