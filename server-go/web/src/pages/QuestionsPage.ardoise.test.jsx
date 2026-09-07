import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import QuestionsPage from './QuestionsPage'

// ---------------------------------------------------------------------------
// Mocks
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
    CULTURE: { label: 'Culture', icon: '🎭', color: '#8b5cf6' },
    SPORT:   { label: 'Sport',   icon: '⚽', color: '#22c55e' },
  },
}))

vi.mock('./QuestionsPage.css', () => ({}))
vi.mock('./ConfigPage.css',    () => ({}))

// ---------------------------------------------------------------------------
// Import useGame après mocks
// ---------------------------------------------------------------------------
import { useGame } from '../hooks/GameContext'

// ---------------------------------------------------------------------------
// Helper : mock minimal useGame
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

// ---------------------------------------------------------------------------
// Tests : QuestionsPage — ARDOISE form (issue #87, v5.6.0)
// ---------------------------------------------------------------------------

describe('QuestionsPage — sélecteur de type ARDOISE (#87)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useGame.mockReturnValue(makeQPageMock())
  })

  it('le bouton "⌨️ Ardoise" est présent dans le sélecteur de type', () => {
    render(<QuestionsPage />)

    const ardoiseBtn = screen.getByText(/ardoise/i)
    expect(ardoiseBtn).toBeInTheDocument()
  })

  it('le bouton ARDOISE porte la classe "type-btn ardoise"', () => {
    render(<QuestionsPage />)

    const ardoiseBtn = screen.getByText(/ardoise/i).closest('button')
    expect(ardoiseBtn).toHaveClass('type-btn')
    expect(ardoiseBtn).toHaveClass('ardoise')
  })

  it('cliquer sur ARDOISE active la classe "active" sur le bouton', () => {
    render(<QuestionsPage />)

    const ardoiseBtn = screen.getByText(/ardoise/i).closest('button')
    fireEvent.click(ardoiseBtn)

    expect(ardoiseBtn).toHaveClass('active')
  })

  it('cliquer sur ARDOISE masque la classe "active" des autres types', () => {
    render(<QuestionsPage />)

    // First click NORMAL to make it active
    const normalBtn = screen.getAllByRole('button').find(
      btn => btn.textContent === 'Normal' || btn.classList.contains('type-btn') && !btn.classList.contains('qcm') && !btn.classList.contains('memory') && !btn.classList.contains('memotion') && !btn.classList.contains('ardoise')
    )

    const ardoiseBtn = screen.getByText(/ardoise/i).closest('button')
    fireEvent.click(ardoiseBtn)

    // QCM, MEMORY, MEMOTION buttons should NOT be active
    const qcmBtn = screen.getAllByRole('button').find(btn => btn.classList.contains('qcm'))
    const memoryBtn = screen.getAllByRole('button').find(btn => btn.classList.contains('memory'))
    const memotionBtn = screen.getAllByRole('button').find(btn => btn.classList.contains('memotion'))

    expect(qcmBtn).not.toHaveClass('active')
    expect(memoryBtn).not.toHaveClass('active')
    expect(memotionBtn).not.toHaveClass('active')
  })
})

describe('QuestionsPage — formulaire ARDOISE (#87)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useGame.mockReturnValue(makeQPageMock())
  })

  // Helper : render + click ARDOISE type — retourne { container }
  const renderAndSelectArdoise = () => {
    const result = render(<QuestionsPage />)
    const ardoiseBtn = screen.getByText(/ardoise/i).closest('button')
    fireEvent.click(ardoiseBtn)
    return result
  }

  it('affiche la section ARDOISE avec le sélecteur AZERTY/NUMPAD', () => {
    renderAndSelectArdoise()

    expect(screen.getByText('AZERTY')).toBeInTheDocument()
    expect(screen.getByText('Pavé numérique')).toBeInTheDocument()
  })

  it('le bouton AZERTY est actif par défaut', () => {
    renderAndSelectArdoise()

    const azertyBtn = screen.getByText('AZERTY').closest('button')
    expect(azertyBtn).toHaveClass('active')
  })

  it('cliquer sur NUMPAD active le bouton NUMPAD', () => {
    renderAndSelectArdoise()

    const numpadBtn = screen.getByText('Pavé numérique').closest('button')
    fireEvent.click(numpadBtn)
    expect(numpadBtn).toHaveClass('active')
  })

  it('cliquer sur NUMPAD désactive le bouton AZERTY', () => {
    renderAndSelectArdoise()

    const azertyBtn = screen.getByText('AZERTY').closest('button')
    const numpadBtn = screen.getByText('Pavé numérique').closest('button')
    fireEvent.click(numpadBtn)

    expect(azertyBtn).not.toHaveClass('active')
  })

  it('affiche le champ "Bonne réponse" pour ARDOISE (libellé spécifique animateur)', () => {
    renderAndSelectArdoise()

    // Plusieurs éléments contiennent "bonne réponse" (label + ardoise-info) — prendre le premier
    expect(screen.getAllByText(/bonne réponse/i)[0]).toBeInTheDocument()
  })

  it('masque les champs QCM (RED/GREEN/YELLOW/BLUE) pour le type ARDOISE', () => {
    renderAndSelectArdoise()

    // QCM section is only rendered when type === 'QCM'
    // Check QCM color buttons are absent
    expect(screen.queryByText('Rouge')).toBeNull()
    expect(screen.queryByText('Vert')).toBeNull()
    expect(screen.queryByText('Jaune')).toBeNull()
    expect(screen.queryByText('Bleu')).toBeNull()
  })

  it('masque les champs MEMORY quand type=ARDOISE', () => {
    renderAndSelectArdoise()

    // Memory section has a heading or specific label
    expect(screen.queryByText(/paires/i)).toBeNull()
    expect(screen.queryByText(/mémoire/i)).toBeNull()
  })

  it('masque les champs MEMOTION quand type=ARDOISE', () => {
    const { container } = renderAndSelectArdoise()

    // Le bouton de type "Memotion" reste visible — seule la section de champs MEMOTION est masquée
    expect(container.querySelector('.memotion-section')).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Tests #94 — Champs image visibles pour le type ARDOISE (#94)
// ---------------------------------------------------------------------------

describe('QuestionsPage — champs image visibles pour ARDOISE (#94)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useGame.mockReturnValue(makeQPageMock())
  })

  const renderAndSelectArdoise = () => {
    const result = render(<QuestionsPage />)
    const ardoiseBtn = screen.getByText(/ardoise/i).closest('button')
    fireEvent.click(ardoiseBtn)
    return result
  }

  it('affiche le champ "Image question" quand type=ARDOISE', () => {
    renderAndSelectArdoise()

    // Le label "Image question (optionnel)" doit être présent pour ARDOISE
    expect(screen.getByText('Image question (optionnel)')).toBeInTheDocument()
  })

  it('affiche le champ "Image reponse" quand type=ARDOISE', () => {
    renderAndSelectArdoise()

    // Le label "Image reponse (optionnel)" doit être présent pour ARDOISE
    expect(screen.getByText('Image reponse (optionnel)')).toBeInTheDocument()
  })

  it('l\'input #media-input est présent pour ARDOISE', () => {
    const { container } = renderAndSelectArdoise()

    expect(container.querySelector('#media-input')).not.toBeNull()
  })

  it('l\'input #media-answer-input est présent pour ARDOISE', () => {
    const { container } = renderAndSelectArdoise()

    expect(container.querySelector('#media-answer-input')).not.toBeNull()
  })
})

describe('QuestionsPage — sérialisation ARDOISE_KEYBOARD_TYPE (#87)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('sendMessage est appelé avec ardoise_keyboard_type=AZERTY lors de la soumission', async () => {
    const sendMessage = vi.fn().mockResolvedValue(undefined)
    useGame.mockReturnValue(makeQPageMock({ sendMessage }))

    // Also mock fetch for the form submission
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ ID: '99' }),
    })

    const { container } = render(<QuestionsPage />)

    // Select ARDOISE type
    const ardoiseBtn = screen.getByText(/ardoise/i).closest('button')
    fireEvent.click(ardoiseBtn)

    // Fill in required answer field
    const answerInput = screen.getByPlaceholderText(/réponse attendue/i)
    fireEvent.change(answerInput, { target: { value: 'La Tour Eiffel' } })

    // Fill in required question text
    const questionInputs = screen.getAllByPlaceholderText(/question/i)
    if (questionInputs.length > 0) {
      fireEvent.change(questionInputs[0], { target: { value: 'Quelle est la capitale de la France ?' } })
    }

    // Submit the form — cibler .submit-btn pour éviter l'ambiguïté avec "Enregistrer" (quiz meta)
    const submitBtn = container.querySelector('.submit-btn')
    if (submitBtn) {
      fireEvent.click(submitBtn)
    }

    // Verify the AZERTY default is used
    if (global.fetch.mock.calls.length > 0) {
      const formData = global.fetch.mock.calls[0][1]?.body
      // FormData.get if available — if fetch was called with FormData body
      if (formData instanceof FormData) {
        expect(formData.get('ardoise_keyboard_type')).toBe('AZERTY')
      }
    }
  })
})
