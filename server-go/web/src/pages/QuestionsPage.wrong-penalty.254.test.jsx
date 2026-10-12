import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import QuestionsPage from './QuestionsPage'

// #254 (v11.2, lot V2-4, lot C) — éditeur de questions : champ « Pénalité mauvaise réponse » (QCM_WRONG_PENALTY), selon
// la maquette VALIDÉE admin_question_editor__qcm_wrong_penalty_default0 : questions QCM seulement, sur la ligne
// Points / Temps, signe « − » fixe, saisie de 0 à 100, enregistrée en négatif (qcm_wrong_penalty), 0 = aucune pénalité
// = défaut (nouvelle question ET ancienne question sans le champ, « par défaut »), erreur et envoi bloqué si invalide.
// Contrats : http-endpoints.md (qcm_wrong_penalty), models.md §Question. Scénarios 20 et 21 du plan.

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
const makeQPageMock = (over = {}) => ({
  questions: over.questions ?? {},
  fsInfo: { used: 0, total: 100 },
  deleteQuestion: vi.fn(),
  sendMessage: vi.fn(),
  gameState: { phase: 'STOPPED', question: null },
  newGame: vi.fn(),
  ...over,
})
const QCM_Q = (over = {}) => ({
  ID: '9', TYPE: 'QCM', QUESTION: 'Quel fleuve traverse Paris ?', ANSWER: 'La Seine', POINTS: '3', TIME: '30', CATEGORY: 'CULTURE',
  QCM_ANSWERS: { RED: 'La Loire', GREEN: 'La Seine', YELLOW: 'Le Rhône', BLUE: 'La Garonne' }, QCM_CORRECT: 'GREEN', ...over,
})
const FIELD = /Pénalité mauvaise réponse/
const penaltyInput = () => screen.getByLabelText(FIELD)
const queryPenaltyInput = () => screen.queryByLabelText(FIELD)

let posted
beforeEach(() => {
  vi.clearAllMocks()
  posted = []
  global.fetch = vi.fn((url, opts) => {
    if (String(url).includes('/questions') && opts?.method === 'POST') posted.push(opts.body)
    // seul l'enregistrement aboutit ; les autres appels de la page (catégories, config) sont ignorés par elle
    const isSave = String(url).includes('/questions') && opts?.method === 'POST'
    return Promise.resolve({ ok: isSave, json: () => Promise.resolve({}) })
  })
})
afterEach(() => { vi.restoreAllMocks() })

const editExisting = (question) => {
  useGame.mockReturnValue(makeQPageMock({ questions: { [question.ID]: question } }))
  render(<QuestionsPage />)
  fireEvent.click(screen.getByTestId(`qcard-${question.ID}`))
}
const submit = () => fireEvent.click(screen.getByRole('button', { name: /^(Modifier|Ajouter)$/ }))

describe('Éditeur — champ « Pénalité mauvaise réponse » (#254, scénario 20)', () => {
  it('Critical — nouvelle question QCM : champ présent, valeur 0 « par défaut », signe « − » fixe', () => {
    useGame.mockReturnValue(makeQPageMock())
    render(<QuestionsPage />)
    fireEvent.click(screen.getAllByRole('button').find((b) => b.classList.contains('qcm')))
    expect(penaltyInput().value).toBe('0')
    const group = penaltyInput().closest('.form-group') || penaltyInput().parentElement
    expect(group.textContent).toMatch(/par défaut/)
    expect(group.textContent).toMatch(/[−-]/)
  })

  it('ancienne question sans le champ : 0 « par défaut » (aucune réécriture)', () => {
    editExisting(QCM_Q())
    expect(penaltyInput().value).toBe('0')
    expect((penaltyInput().closest('.form-group') || penaltyInput().parentElement).textContent).toMatch(/par défaut/)
  })

  it('question avec QCM_WRONG_PENALTY = −2 : le champ affiche 2 (le signe − est fixe), plus de mention « par défaut »', () => {
    editExisting(QCM_Q({ QCM_WRONG_PENALTY: -2 }))
    expect(penaltyInput().value).toBe('2')
    expect((penaltyInput().closest('.form-group') || penaltyInput().parentElement).textContent).not.toMatch(/par défaut/)
  })

  it('Critical — saisie 150 : message d’erreur « Entier de 0 à 100 », bouton d’enregistrement désactivé, aucun envoi', async () => {
    editExisting(QCM_Q())
    fireEvent.change(penaltyInput(), { target: { value: '150' } })
    const group = penaltyInput().closest('.form-group') || penaltyInput().parentElement
    expect(group.textContent).toMatch(/Entier de 0 à 100/)
    expect(screen.getByRole('button', { name: /^(Modifier|Ajouter)$/ }).disabled).toBe(true)
    submit()
    await new Promise((r) => setTimeout(r, 30))
    expect(posted).toHaveLength(0)
  })

  it.each(['-1', '2,5', 'abc'])('saisie invalide %s : erreur et envoi bloqué', (v) => {
    editExisting(QCM_Q())
    fireEvent.change(penaltyInput(), { target: { value: v } })
    expect(screen.getByRole('button', { name: /^(Modifier|Ajouter)$/ }).disabled).toBe(true)
  })

  it('Critical — saisie 0 : valide, envoi de qcm_wrong_penalty=0', async () => {
    editExisting(QCM_Q({ QCM_WRONG_PENALTY: -2 }))
    fireEvent.change(penaltyInput(), { target: { value: '0' } })
    expect(screen.getByRole('button', { name: /^(Modifier|Ajouter)$/ }).disabled).toBe(false)
    submit()
    await waitFor(() => expect(posted).toHaveLength(1))
    expect(posted[0].get('qcm_wrong_penalty')).toBe('0')
  })

  it('Critical — saisie 2 : envoi de qcm_wrong_penalty=-2 (enregistré en négatif)', async () => {
    editExisting(QCM_Q())
    fireEvent.change(penaltyInput(), { target: { value: '2' } })
    submit()
    await waitFor(() => expect(posted).toHaveLength(1))
    expect(posted[0].get('qcm_wrong_penalty')).toBe('-2')
  })

  it('ancienne question enregistrée sans toucher au champ : envoie qcm_wrong_penalty=0 (le fichier reçoit le champ à la prochaine sauvegarde)', async () => {
    editExisting(QCM_Q())
    submit()
    await waitFor(() => expect(posted).toHaveLength(1))
    expect(posted[0].get('qcm_wrong_penalty')).toBe('0')
  })

  it.each(['SPEEDY', 'ARDOISE'])('type %s : champ absent et qcm_wrong_penalty non envoyé', async (type) => {
    editExisting({ ID: '5', TYPE: type, QUESTION: 'Q ?', ANSWER: 'A', POINTS: '2', TIME: '30', CATEGORY: 'CULTURE', ...(type === 'ARDOISE' ? { ARDOISE_KEYBOARD_TYPE: 'AZERTY' } : {}) })
    expect(queryPenaltyInput()).toBeNull()
    submit()
    await waitFor(() => expect(posted).toHaveLength(1))
    expect(posted[0].has('qcm_wrong_penalty')).toBe(false)
  })

  it('passer d’un QCM à un autre type masque le champ ; revenir au QCM le réaffiche', () => {
    useGame.mockReturnValue(makeQPageMock())
    render(<QuestionsPage />)
    const qcmBtn = screen.getAllByRole('button').find((b) => b.classList.contains('qcm'))
    fireEvent.click(qcmBtn)
    expect(queryPenaltyInput()).not.toBeNull()
    fireEvent.click(screen.getByText(/ardoise/i).closest('button'))
    expect(queryPenaltyInput()).toBeNull()
    fireEvent.click(qcmBtn)
    expect(queryPenaltyInput()).not.toBeNull()
  })

  it('le champ est distinct des indices progressifs : il reste visible indices activés ou non', () => {
    editExisting(QCM_Q({ QCM_HINTS_ENABLED: true }))
    expect(queryPenaltyInput()).not.toBeNull()
    expect(screen.queryAllByText(/indices progressifs/i).length).toBeGreaterThan(0)
  })
})

describe('Éditeur — cartes MEMOTION de type QCM (#254, scénario 21)', () => {
  it('Critical — le type MEMOTION n’expose aucun champ de pénalité de mauvaise réponse', () => {
    useGame.mockReturnValue(makeQPageMock())
    render(<QuestionsPage />)
    fireEvent.click(screen.getAllByRole('button').find((b) => b.classList.contains('memotion')))
    expect(queryPenaltyInput()).toBeNull()
  })

  it('une carte MEMOTION de type QCM existante n’a pas de champ non plus', () => {
    editExisting({
      ID: '30', TYPE: 'MEMOTION', QUESTION: 'Manche', ANSWER: '—', MOTION_MODE: 'SOLO', MOTION_CONFIG: { POINTS_1_STAR: 1, POINTS_2_STAR: 3, POINTS_3_STAR: 5 },
      MOTION_CARDS: [{ ID: 'c1', TYPE: 'QCM', QUESTION: 'Q ?', QCM_ANSWERS: { RED: 'a', GREEN: 'b', YELLOW: 'c', BLUE: 'd' }, QCM_CORRECT: 'RED', DIFFICULTY: 1 }],
    })
    expect(queryPenaltyInput()).toBeNull()
  })
})
