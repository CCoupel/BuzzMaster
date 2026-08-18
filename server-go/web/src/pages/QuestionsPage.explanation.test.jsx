import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import QuestionsPage from './QuestionsPage'

// ---------------------------------------------------------------------------
// QuestionsPage — note d'explication (v6.4.x, #168, tâche F8).
//
// Plan : _work/reports/plan-20260818-121500.md. Contrat : contracts/
// models.md §EXPLANATION, contracts/http-endpoints.md POST /questions.
// Patron d'assertion FormData : QuestionsPage.ardoise.test.jsx (sérialisation
// ARDOISE_KEYBOARD_TYPE), mocks identiques.
//
// Le piège du plan (F8) : la liste des champs de formData est dupliquée
// TROIS fois dans QuestionsPage.jsx (état initial useState, peuplement à
// l'édition, handleNewQuestion) — ce fichier couvre les trois.
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
  },
}))

vi.mock('./QuestionsPage.css', () => ({}))
vi.mock('./ConfigPage.css', () => ({}))

import { useGame } from '../hooks/GameContext'

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

const EXPLANATION_LABEL = "Note d'explication (animateur seul)"

function getNoteTextarea() {
  return screen.getByLabelText(EXPLANATION_LABEL)
}

function fillMinimalSpeedyForm() {
  fireEvent.change(screen.getAllByPlaceholderText(/question/i)[0], { target: { value: 'Une question' } })
  fireEvent.change(screen.getByPlaceholderText(/entrez la reponse/i), { target: { value: 'Une réponse' } })
}

function submit(container) {
  const submitBtn = container.querySelector('.submit-btn')
  fireEvent.click(submitBtn)
}

// QuestionsPage fetches /config.json on mount (AI status, useEffect) — that
// call lands in global.fetch.mock.calls BEFORE the submit's POST /questions,
// so calls[0] is not reliable. Find the /questions call explicitly.
function getSubmittedFormData() {
  const call = global.fetch.mock.calls.find(c => c[0] === '/questions')
  return call?.[1]?.body
}

beforeEach(() => {
  vi.clearAllMocks()
  useGame.mockReturnValue(makeQPageMock())
})

// ---------------------------------------------------------------------------
// Présence et absence de contrainte de longueur
// ---------------------------------------------------------------------------

describe('QuestionsPage — champ note d\'explication', () => {
  it('la textarea est présente, libellée, vide par défaut', () => {
    render(<QuestionsPage />)
    const textarea = getNoteTextarea()
    expect(textarea).toBeInTheDocument()
    expect(textarea.tagName).toBe('TEXTAREA')
    expect(textarea).toHaveValue('')
  })

  it('aucune limite de longueur (maxLength) — la note n\'est pas bornée (contrat §EXPLANATION)', () => {
    render(<QuestionsPage />)
    expect(getNoteTextarea()).not.toHaveAttribute('maxLength')
  })

  it('saisie mise à jour dans le champ', () => {
    render(<QuestionsPage />)
    fireEvent.change(getNoteTextarea(), { target: { value: 'Contexte historique.' } })
    expect(getNoteTextarea()).toHaveValue('Contexte historique.')
  })
})

// ---------------------------------------------------------------------------
// Soumission — FormData (patron QuestionsPage.ardoise.test.jsx)
// ---------------------------------------------------------------------------

describe('QuestionsPage — soumission de la note (POST /questions)', () => {
  beforeEach(() => {
    global.fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ ID: '1' }) })
  })

  it('data.append("explanation", ...) porte le texte saisi', async () => {
    const { container } = render(<QuestionsPage />)
    fillMinimalSpeedyForm()
    fireEvent.change(getNoteTextarea(), { target: { value: 'Une note utile.' } })
    submit(container)

    expect(global.fetch).toHaveBeenCalled()
    const formData = getSubmittedFormData()
    expect(formData).toBeInstanceOf(FormData)
    expect(formData.get('explanation')).toBe('Une note utile.')
  })

  it('AC15 : une note explicitement vidée est soumise comme chaîne vide (mécanisme d\'effacement)', async () => {
    useGame.mockReturnValue(makeQPageMock({
      questions: { q1: { ID: '1', QUESTION: 'Q', ANSWER: 'A', TYPE: 'SPEEDY', EXPLANATION: 'Ancienne note' } },
    }))
    const { container } = render(<QuestionsPage />)
    fireEvent.click(screen.getByTestId('qcard-1')) // édite la question existante
    expect(getNoteTextarea()).toHaveValue('Ancienne note')

    fireEvent.change(getNoteTextarea(), { target: { value: '' } })
    submit(container)

    const formData = getSubmittedFormData()
    expect(formData.get('explanation')).toBe('')
  })

  it('question sans note : "explanation" toujours présent dans FormData, vide (jamais omis)', async () => {
    const { container } = render(<QuestionsPage />)
    fillMinimalSpeedyForm()
    submit(container)

    const formData = getSubmittedFormData()
    expect(formData.get('explanation')).toBe('')
  })
})

// ---------------------------------------------------------------------------
// AC14 — édition : la note est pré-remplie et SURVIT à une réédition qui ne
// la touche pas (piège handleUploadQuestion, côté frontend : le formulaire
// doit continuer à la renvoyer telle quelle).
// ---------------------------------------------------------------------------

describe('QuestionsPage — édition d\'une question existante (AC14)', () => {
  beforeEach(() => {
    global.fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ ID: '1' }) })
  })

  it('cliquer sur une question avec EXPLANATION pré-remplit la textarea', () => {
    useGame.mockReturnValue(makeQPageMock({
      questions: { q1: { ID: '1', QUESTION: 'Capitale ?', ANSWER: 'Paris', TYPE: 'SPEEDY', EXPLANATION: 'Paris depuis 508.' } },
    }))
    render(<QuestionsPage />)
    fireEvent.click(screen.getByTestId('qcard-1'))
    expect(getNoteTextarea()).toHaveValue('Paris depuis 508.')
  })

  it('cliquer sur une question SANS EXPLANATION laisse la textarea vide (pas de fuite d\'une édition précédente)', () => {
    useGame.mockReturnValue(makeQPageMock({
      questions: { q1: { ID: '1', QUESTION: 'Q1', ANSWER: 'A1', TYPE: 'SPEEDY' } },
    }))
    render(<QuestionsPage />)
    fireEvent.click(screen.getByTestId('qcard-1'))
    expect(getNoteTextarea()).toHaveValue('')
  })

  it('réédition sans toucher la note : la valeur pré-remplie est renvoyée telle quelle, pas perdue', async () => {
    useGame.mockReturnValue(makeQPageMock({
      questions: { q1: { ID: '1', QUESTION: 'Capitale ?', ANSWER: 'Paris', TYPE: 'SPEEDY', EXPLANATION: 'Paris depuis 508.' } },
    }))
    const { container } = render(<QuestionsPage />)
    fireEvent.click(screen.getByTestId('qcard-1'))

    // Seule la question elle-même est modifiée — la note n'est pas touchée.
    fireEvent.change(screen.getAllByPlaceholderText(/question/i)[0], { target: { value: 'Quelle est la capitale de la France ?' } })
    submit(container)

    const formData = getSubmittedFormData()
    expect(formData.get('number')).toBe('1')
    expect(formData.get('explanation')).toBe('Paris depuis 508.')
  })

  it('« + Nouveau » après une édition remet la note à vide (pas de fuite vers la question suivante)', () => {
    useGame.mockReturnValue(makeQPageMock({
      questions: { q1: { ID: '1', QUESTION: 'Q1', ANSWER: 'A1', TYPE: 'SPEEDY', EXPLANATION: 'Note de la question 1' } },
    }))
    render(<QuestionsPage />)
    fireEvent.click(screen.getByTestId('qcard-1'))
    expect(getNoteTextarea()).toHaveValue('Note de la question 1')

    fireEvent.click(screen.getByText('+ Nouveau'))
    expect(getNoteTextarea()).toHaveValue('')
  })
})
