/**
 * Tests for QuestionsPage — "Mélanger les questions" (#149, plan tâche 25.5)
 *
 * Maquette validée : artifact dc722290-a674-49b7-8ec4-5e36efc43903
 * (barre au-dessus de la grille, libellé "Mélanger les questions",
 * confirmation avant action, avertissement renforcé si une partie est en
 * cours, mélange portant sur TOUTES les questions même filtre actif).
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import QuestionsPage from './QuestionsPage'

// ---------------------------------------------------------------------------
// Mocks — même pattern que QuestionsPage.v571.test.jsx (vi.fn() +
// mockReturnValue : identité stable entre rendus, cf. #136)
// ---------------------------------------------------------------------------

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

vi.mock('../hooks/useCategoryFilter', () => ({
  useCategoryFilter: vi.fn(),
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
  default: ({ children, className, ...rest }) => (
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
import { useCategoryFilter } from '../hooks/useCategoryFilter'
import { useCategories } from '../hooks/useCategories'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeQuestions(n) {
  const q = {}
  for (let i = 1; i <= n; i++) {
    q[String(i)] = { ID: String(i), ORDER: i, TEXT: `Question ${i}`, CATEGORY: 'GEOGRAPHY', TYPE: 'SPEEDY' }
  }
  return q
}

const makeQPageMock = (overrides = {}) => ({
  questions: overrides.questions ?? makeQuestions(6),
  fsInfo: { used: 0, total: 100 },
  deleteQuestion: vi.fn(),
  sendMessage: overrides.sendMessage ?? vi.fn(() => true),
  gameState: { phase: 'STOPPED', question: null, ...overrides.gameState },
  newGame: vi.fn(),
  aiJob: null,
  cancelAiGeneration: vi.fn(),
})

const makeCategoryFilterMock = (questions, overrides = {}) => ({
  selectedCategories: overrides.selectedCategories ?? new Set(),
  availableCategories: overrides.availableCategories ?? [],
  filteredQuestions: overrides.filteredQuestions ?? questions,
  toggleCategoryFilter: vi.fn(),
  clearCategoryFilters: vi.fn(),
})

const makeCategoriesMock = (overrides = {}) => ({
  categories: overrides.categories ?? [],
  loading: false,
  error: null,
  refetch: vi.fn(),
})

// Renders QuestionsPage with the given questions map + optional gameState /
// category-filter overrides, and returns the useGame() mock for assertions.
function setup({ questions, gameStateOverrides = {}, sendMessage, filterOverrides = {} } = {}) {
  const q = questions ?? makeQuestions(6)
  const gameMock = makeQPageMock({ questions: q, gameState: gameStateOverrides, sendMessage })
  useGame.mockReturnValue(gameMock)
  const sorted = Object.values(q).sort((a, b) => a.ORDER - b.ORDER)
  useCategoryFilter.mockReturnValue(makeCategoryFilterMock(sorted, filterOverrides))
  useCategories.mockReturnValue(makeCategoriesMock())
  render(<QuestionsPage />)
  return gameMock
}

function reorderCall(gameMock) {
  return gameMock.sendMessage.mock.calls.find(c => c[0] === 'REORDER_QUESTIONS')
}

describe('QuestionsPage — Mélanger les questions (#149)', () => {
  let confirmSpy

  beforeEach(() => {
    vi.clearAllMocks()
    confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
  })

  afterEach(() => {
    confirmSpy.mockRestore()
    vi.clearAllMocks()
  })

  it('émet REORDER_QUESTIONS avec une permutation des IDs (même multiensemble)', () => {
    const gameMock = setup()

    fireEvent.click(screen.getByText('Mélanger les questions'))

    const call = reorderCall(gameMock)
    expect(call).toBeTruthy()
    const [, payload] = call
    expect(payload.ORDER).toHaveLength(6)
    expect([...payload.ORDER].sort()).toEqual(['1', '2', '3', '4', '5', '6'])
  })

  it('porte sur la liste complète des questions, même quand un filtre de catégorie est actif', () => {
    const questions = makeQuestions(6)
    // Filtre actif : filteredQuestions (affiché) n'a que 2 questions, mais le
    // mélange doit porter sur les 6 (sortedQuestions, jamais filteredQuestions).
    const gameMock = setup({
      questions,
      filterOverrides: {
        selectedCategories: new Set(['GEOGRAPHY']),
        filteredQuestions: Object.values(questions).slice(0, 2),
      },
    })

    fireEvent.click(screen.getByText('Mélanger les questions'))

    const [, payload] = reorderCall(gameMock)
    expect(payload.ORDER).toHaveLength(6)
  })

  it('la confirmation mentionne explicitement que le filtre actif est ignoré', () => {
    const questions = makeQuestions(6)
    setup({
      questions,
      filterOverrides: { selectedCategories: new Set(['GEOGRAPHY']) },
    })

    fireEvent.click(screen.getByText('Mélanger les questions'))

    expect(confirmSpy).toHaveBeenCalledWith(expect.stringContaining('toutes les questions'))
  })

  it('annulation de la confirmation -> rien n\'est émis', () => {
    confirmSpy.mockReturnValue(false)
    const gameMock = setup()

    fireEvent.click(screen.getByText('Mélanger les questions'))

    expect(reorderCall(gameMock)).toBeUndefined()
  })

  it('moins de 2 questions -> bouton désactivé', () => {
    setup({ questions: makeQuestions(1) })

    expect(screen.getByText('Mélanger les questions').closest('button')).toBeDisabled()
  })

  it('aucune question -> bouton désactivé', () => {
    setup({ questions: {} })

    expect(screen.getByText('Mélanger les questions').closest('button')).toBeDisabled()
  })

  it('échec réseau (WebSocket fermé) -> aucune permutation confirmée, toast d\'erreur affiché', () => {
    const gameMock = setup({ sendMessage: vi.fn(() => false) })

    fireEvent.click(screen.getByText('Mélanger les questions'))

    expect(gameMock.sendMessage).toHaveBeenCalledWith('REORDER_QUESTIONS', expect.objectContaining({ ORDER: expect.any(Array) }))
    expect(screen.getByText(/connexion perdue/)).toBeInTheDocument()
    // Aucun toast de succès concurrent
    expect(screen.queryByText(/Ordre mélangé/)).not.toBeInTheDocument()
  })

  it('affiche un toast de succès après un mélange envoyé', () => {
    setup()

    fireEvent.click(screen.getByText('Mélanger les questions'))

    expect(screen.getByText(/Ordre mélangé — 6 questions/)).toBeInTheDocument()
  })

  it('partie en cours (phase != STOPPED) -> confirmation renforcée mentionnant la partie en cours', () => {
    setup({ gameStateOverrides: { phase: 'STARTED' } })

    fireEvent.click(screen.getByText('Mélanger les questions'))

    expect(confirmSpy).toHaveBeenCalledWith(expect.stringContaining('partie est en cours'))
  })

  it('aucune partie en cours (phase STOPPED) -> confirmation standard, sans mention de partie en cours', () => {
    setup({ gameStateOverrides: { phase: 'STOPPED' } })

    fireEvent.click(screen.getByText('Mélanger les questions'))

    expect(confirmSpy).toHaveBeenCalledWith(expect.not.stringContaining('partie est en cours'))
  })
})
