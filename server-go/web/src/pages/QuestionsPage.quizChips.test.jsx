/**
 * Tests — QuestionsPage : chips multi-sélection Publics/Difficultés (#137
 * Batch 2b T2.2).
 *
 * Contexte (_work/handoff/task-test-writer-review-batch2b-20260806-165427.md) :
 * dev-frontend a livré T2.1→T2.5 sans ajouter de nouveaux tests Vitest —
 * uniquement l'adaptation des fixtures existantes. Les chips (motif repris
 * des catégories d'AIGenerateModal.jsx) remplacent les <select> à valeur
 * unique v6.0.0 ; ce fichier verrouille : sélection, désélection, et le fait
 * que la sélection locale n'est envoyée au serveur qu'au clic sur
 * "Enregistrer" — jamais au clic sur un chip lui-même (contract §5bis, T2.5 :
 * la génération IA doit utiliser gameState.quiz*, pas un état de formulaire
 * qui s'auto-enverrait à chaque interaction).
 *
 * Suit le pattern de mocks de QuestionsPage.v571.test.jsx /
 * QuestionsPage.ardoise.test.jsx.
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import QuestionsPage from './QuestionsPage'

// ---------------------------------------------------------------------------
// Mocks
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
    <div data-testid={`qcard-${question.ID}`} onClick={() => onClick && onClick(question)} />
  ),
  CATEGORIES: {
    GEOGRAPHY: { label: 'Geographie', icon: '🌍', color: '#3b82f6' },
  },
}))

vi.mock('./QuestionsPage.css', () => ({}))
vi.mock('./ConfigPage.css', () => ({}))

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
    quizPopulations: [],
    quizDifficulties: [],
    quizHiddenFields: [],
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

describe('QuestionsPage — chips multi-sélection Publics/Difficultés (#137 Batch 2b T2.2)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useCategories.mockReturnValue(makeCategoriesMock())
    global.fetch = vi.fn()
  })

  it('un public non sélectionné devient actif au clic (sélection)', () => {
    useGame.mockReturnValue(makeQPageMock())
    render(<QuestionsPage />)

    const chip = screen.getByRole('button', { name: 'Ado (13-17 ans)' })
    expect(chip.className).not.toMatch(/active/)

    fireEvent.click(chip)

    expect(chip.className).toMatch(/active/)
  })

  it('un public déjà sélectionné (via gameState) redevient inactif au clic (désélection)', () => {
    useGame.mockReturnValue(makeQPageMock({ gameState: { quizPopulations: ['Ado (13-17 ans)'] } }))
    render(<QuestionsPage />)

    const chip = screen.getByRole('button', { name: 'Ado (13-17 ans)' })
    expect(chip.className).toMatch(/active/)

    fireEvent.click(chip)

    expect(chip.className).not.toMatch(/active/)
  })

  it('une difficulté suit la même logique de bascule sélection/désélection', () => {
    useGame.mockReturnValue(makeQPageMock({ gameState: { quizDifficulties: ['Moyen'] } }))
    render(<QuestionsPage />)

    const chip = screen.getByRole('button', { name: 'Moyen' })
    expect(chip.className).toMatch(/active/)

    fireEvent.click(chip)
    expect(chip.className).not.toMatch(/active/)

    fireEvent.click(chip)
    expect(chip.className).toMatch(/active/)
  })

  it('cliquer un chip ne déclenche AUCUN envoi au serveur — seule "Enregistrer" le fait', () => {
    const sendMessage = vi.fn()
    useGame.mockReturnValue(makeQPageMock({ sendMessage }))
    render(<QuestionsPage />)

    fireEvent.click(screen.getByRole('button', { name: 'Ado (13-17 ans)' }))
    fireEvent.click(screen.getByRole('button', { name: 'Moyen' }))

    expect(sendMessage).not.toHaveBeenCalledWith('UPDATE_QUIZ_META', expect.anything())
  })

  it('"Enregistrer" envoie la sélection locale de chips (POPULATIONS/DIFFICULTIES), pas seulement ce qui était déjà dans gameState', () => {
    const sendMessage = vi.fn()
    useGame.mockReturnValue(makeQPageMock({ sendMessage, gameState: { quizPopulations: [], quizDifficulties: [] } }))
    const { container } = render(<QuestionsPage />)

    // Sélectionne 2 publics et 1 difficulté avant tout enregistrement —
    // persistance dans l'état local du formulaire, pas encore dans gameState.
    fireEvent.click(screen.getByRole('button', { name: 'Ado (13-17 ans)' }))
    fireEvent.click(screen.getByRole('button', { name: 'Adulte (18-64 ans)' }))
    fireEvent.click(screen.getByRole('button', { name: 'Moyen' }))

    // Scopé au formulaire Quiz (.quiz-meta-form) : depuis #119 (corrections
    // C1), la section Entracte de cette même page a AUSSI un bouton
    // "Enregistrer" — un getByText global ne serait plus unique.
    const quizMetaForm = container.querySelector('.quiz-meta-form')
    fireEvent.click(within(quizMetaForm).getByText('Enregistrer'))

    expect(sendMessage).toHaveBeenCalledWith(
      'UPDATE_QUIZ_META',
      expect.objectContaining({
        POPULATIONS: expect.arrayContaining(['Ado (13-17 ans)', 'Adulte (18-64 ans)']),
        DIFFICULTIES: ['Moyen'],
      })
    )
    const [, payload] = sendMessage.mock.calls.find(c => c[0] === 'UPDATE_QUIZ_META')
    expect(payload.POPULATIONS).toHaveLength(2)
  })
})
