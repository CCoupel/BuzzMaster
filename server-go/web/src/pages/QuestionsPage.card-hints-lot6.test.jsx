import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import QuestionsPage from './QuestionsPage'

// ---------------------------------------------------------------------------
// QuestionsPage — plus d'indices sur les cartes QCM d'une question MEMOTION
// (#279, LOT-6, v11.2.2, contrat question-types.md §7.1 / §3.2, décision
// utilisateur 2026-10-15 « option B »).
//
// Règles couvertes (plan #279, « Critères d'acceptation » et « Compatibilité ») :
//  - le sous-éditeur QCM d'une CARTE n'affiche ni la case « Activer les indices
//    progressifs » ni seuils / pénalités ; celui d'une QUESTION QCM les affiche
//    comme avant (NR hôte, #254)
//  - payload `motion_cards` d'une carte QCM : aucune des 5 clés d'indices
//    (QCM_HINTS_ENABLED, QCM_HINT_THRESHOLD_1/2, QCM_PENALTY_1/2)
//  - carte héritée (enregistrée avant LOT-6 avec QCM_HINTS_ENABLED:true) :
//    ligne d'information « Indices retirés… » sur la carte, réponses et
//    désignation intactes, ré-enregistrement SANS les 5 clés
//  - verrou de type : une carte QCM neuve reste déverrouillée ; réponses et
//    désignation verrouillent toujours ; les indices n'interviennent plus
//
// Mêmes conventions de mock que QuestionsPage.points-rule.190.test.jsx.
// Cible rôles / libellés ; la carte est repérée par `.memotion-card-item`
// (classe structurelle existante, comme QuestionsPage.motionMemory.test.jsx).
// ---------------------------------------------------------------------------

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

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
    <div data-testid={`qcard-${question.ID}`} onClick={() => onClick && onClick(question)} />
  ),
  CATEGORIES: {
    CULTURE: { label: 'Culture', icon: '🎭', color: '#8b5cf6' },
  },
}))

vi.mock('./QuestionsPage.css', () => ({}))
vi.mock('./ConfigPage.css', () => ({}))

import { useGame } from '../hooks/GameContext'

const makeQPageMock = (over = {}) => ({
  questions: over.questions ?? {},
  fsInfo: { used: 0, total: 100 },
  deleteQuestion: vi.fn(),
  sendMessage: vi.fn(),
  gameState: { phase: 'STOPPED', question: null },
  newGame: vi.fn(),
  ...over,
})

const MOTION_CONFIG = { POINTS_1_STAR: 1, POINTS_2_STAR: 3, POINTS_3_STAR: 5 }
const HINT_KEYS = ['QCM_HINTS_ENABLED', 'QCM_HINT_THRESHOLD_1', 'QCM_HINT_THRESHOLD_2', 'QCM_PENALTY_1', 'QCM_PENALTY_2']
const LEGACY_HINTS = {
  QCM_HINTS_ENABLED: true,
  QCM_HINT_THRESHOLD_1: 0.4,
  QCM_HINT_THRESHOLD_2: 0.2,
  QCM_PENALTY_1: 0.5,
  QCM_PENALTY_2: 0.25,
}
const ANSWERS = { RED: 'Paris', GREEN: 'Lyon', YELLOW: 'Nice', BLUE: 'Metz' }

const qcmCard = (over = {}) => ({
  ID: 'cq', TYPE: 'QCM', RECTO_THEME: 'Capitales', DIFFICULTY: 2,
  QUESTION_TEXT: 'Capitale de la France ?', QCM_ANSWERS: ANSWERS, QCM_CORRECT: 'RED', ...over,
})
const speedyCard = (over = {}) => ({ ID: 'cs', RECTO_THEME: 'Fleuves', DIFFICULTY: 1, ...over })

const memotionQuestion = (cards) => ({
  ID: '30', TYPE: 'MEMOTION', QUESTION: 'Manche indices', ANSWER: '—', POINTS: '1', TIME: '30',
  MOTION_MODE: 'SOLO', MOTION_CONFIG,
  MOTION_CARDS: cards ?? [qcmCard(), speedyCard()],
})

const hostQcm = (over = {}) => ({
  ID: '9', TYPE: 'QCM', QUESTION: 'Quel fleuve traverse Paris ?', ANSWER: 'La Seine', POINTS: '3', TIME: '30', CATEGORY: 'CULTURE',
  QCM_ANSWERS: { RED: 'La Loire', GREEN: 'La Seine', YELLOW: 'Le Rhône', BLUE: 'La Garonne' }, QCM_CORRECT: 'GREEN', ...over,
})

let posted
beforeEach(() => {
  vi.clearAllMocks()
  posted = []
  global.fetch = vi.fn((url, opts) => {
    const isSave = String(url).includes('/questions') && opts?.method === 'POST'
    if (isSave) posted.push(opts.body)
    return Promise.resolve({ ok: isSave, json: () => Promise.resolve({}), text: () => Promise.resolve('') })
  })
})
afterEach(() => { vi.restoreAllMocks() })

const editExisting = (question) => {
  useGame.mockReturnValue(makeQPageMock({ questions: { [question.ID]: question } }))
  const utils = render(<QuestionsPage />)
  fireEvent.click(screen.getByTestId(`qcard-${question.ID}`))
  return utils
}

const renderNew = () => {
  useGame.mockReturnValue(makeQPageMock())
  return render(<QuestionsPage />)
}

const renderNewMemotion = () => {
  const utils = renderNew()
  fireEvent.click(screen.getByText(/memotion/i).closest('button'))
  return utils
}

const cardAt = (container, index) => container.querySelectorAll('.memotion-card-item')[index]
const chooseType = (card, label) => fireEvent.click(within(card).getByRole('button', { name: label }))
const hintsCheckbox = (scope = screen) => scope.queryByRole('checkbox', { name: /Activer les indices progressifs/ })
const submit = () => fireEvent.click(screen.getByRole('button', { name: /^(Modifier|Ajouter)$/ }))
const savedCards = async () => {
  await waitFor(() => expect(posted).toHaveLength(1))
  return JSON.parse(posted[0].get('motion_cards'))
}
const savedCard = async (id) => (await savedCards()).find(c => c.ID === id)
const INFO_LINE = /Indices retirés/

// ---------------------------------------------------------------------------
// Éditeur : bloc d'indices absent en carte, présent en question hôte
// ---------------------------------------------------------------------------

describe('Éditeur — le sous-éditeur QCM d\'une carte MEMOTION n\'a plus d\'indices (#279)', () => {
  it('carte passée en QCM : aucune case « Activer les indices progressifs », aucun texte d\'indice', () => {
    const { container } = renderNewMemotion()
    const card = cardAt(container, 0)
    chooseType(card, 'QCM')
    expect(hintsCheckbox(within(card))).not.toBeInTheDocument()
    expect(within(card).queryByText(/Pas d'indices automatiques/)).not.toBeInTheDocument()
    expect(within(card).queryByText(/Indice 1/)).not.toBeInTheDocument()
  })

  it('carte passée en QCM : les 4 réponses et la désignation de la bonne réponse restent proposées', () => {
    const { container } = renderNewMemotion()
    const card = cardAt(container, 0)
    chooseType(card, 'QCM')
    expect(within(card).getByText('Reponses QCM *')).toBeInTheDocument()
    expect(within(card).getAllByTitle('Marquer comme bonne reponse')).toHaveLength(4)
    expect(within(card).getByPlaceholderText('Reponse A...')).toBeInTheDocument()
  })

  it('aucune case « indices » nulle part dans la page tant que seules des cartes sont éditées', () => {
    const { container } = renderNewMemotion()
    chooseType(cardAt(container, 0), 'QCM')
    chooseType(cardAt(container, 1), 'QCM')
    expect(hintsCheckbox()).not.toBeInTheDocument()
  })

  it('carte chargée de type QCM (sans indices) : pas de bloc d\'indices', () => {
    const { container } = editExisting(memotionQuestion())
    expect(hintsCheckbox(within(cardAt(container, 0)))).not.toBeInTheDocument()
  })

  it('NR hôte — question QCM classique : la case « Activer les indices progressifs » est toujours là', () => {
    renderNew()
    fireEvent.click(screen.getAllByRole('button').find((b) => b.classList.contains('qcm')))
    expect(hintsCheckbox()).toBeInTheDocument()
    expect(screen.getByText('Pas d\'indices automatiques')).toBeInTheDocument()
  })

  it('NR hôte — cocher les indices d\'une question QCM affiche les réglages (Indice 1 / Indice 2)', () => {
    renderNew()
    fireEvent.click(screen.getAllByRole('button').find((b) => b.classList.contains('qcm')))
    fireEvent.click(hintsCheckbox())
    expect(screen.getByText('Indice 1 :')).toBeInTheDocument()
    expect(screen.getByText('Indice 2 :')).toBeInTheDocument()
  })

  it('NR hôte — question QCM existante avec indices : case cochée et réglages affichés', () => {
    editExisting(hostQcm({ QCM_HINTS_ENABLED: true, QCM_HINT_THRESHOLD_1: 0.25, QCM_HINT_THRESHOLD_2: 0.125, QCM_PENALTY_1: 0.67, QCM_PENALTY_2: 0.33 }))
    expect(hintsCheckbox()).toBeChecked()
    expect(screen.getByText('Indice 1 :')).toBeInTheDocument()
  })

  it('NR hôte — la ligne d\'information « Indices retirés » n\'existe jamais sur une question QCM', () => {
    editExisting(hostQcm({ QCM_HINTS_ENABLED: true }))
    expect(screen.queryByText(INFO_LINE)).not.toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// Payload : jamais les 5 clés d'indices sur une carte
// ---------------------------------------------------------------------------

describe('Éditeur — payload motion_cards d\'une carte QCM sans les 5 clés d\'indices (#279)', () => {
  it('carte QCM chargée sans indices, ré-enregistrée : aucune des 5 clés', async () => {
    editExisting(memotionQuestion())
    submit()
    const saved = await savedCard('cq')
    for (const key of HINT_KEYS) expect(key in saved).toBe(false)
  })

  it('carte QCM chargée sans indices : réponses, désignation et type conservés', async () => {
    editExisting(memotionQuestion())
    submit()
    const saved = await savedCard('cq')
    expect(saved.TYPE).toBe('QCM')
    expect(saved.QCM_ANSWERS).toEqual(ANSWERS)
    expect(saved.QCM_CORRECT).toBe('RED')
    expect(saved.RECTO_THEME).toBe('Capitales')
  })

  it('carte devenue QCM dans l\'éditeur puis complétée : payload QCM sans les 5 clés', async () => {
    const { container } = editExisting(memotionQuestion([speedyCard({ ID: 'c1', RECTO_THEME: 'Histoire' }), speedyCard({ ID: 'c2', RECTO_THEME: 'Sport' })]))
    const card = cardAt(container, 0)
    chooseType(card, 'QCM')
    fireEvent.change(within(card).getByPlaceholderText('Reponse A...'), { target: { value: 'Rome' } })
    fireEvent.click(within(card).getAllByTitle('Marquer comme bonne reponse')[0])
    submit()
    const saved = await savedCard('c1')
    expect(saved.TYPE).toBe('QCM')
    expect(saved.QCM_CORRECT).toBe('RED')
    for (const key of HINT_KEYS) expect(key in saved).toBe(false)
  })

  it('une carte non-QCM n\'envoie jamais de clé d\'indice non plus', async () => {
    editExisting(memotionQuestion())
    submit()
    const saved = await savedCard('cs')
    for (const key of HINT_KEYS) expect(key in saved).toBe(false)
  })

  it('NR hôte — question QCM avec indices : qcm_hints_enabled=true et réglages TOUJOURS envoyés', async () => {
    editExisting(hostQcm({ QCM_HINTS_ENABLED: true, QCM_HINT_THRESHOLD_1: 0.25, QCM_HINT_THRESHOLD_2: 0.125, QCM_PENALTY_1: 0.67, QCM_PENALTY_2: 0.33 }))
    submit()
    await waitFor(() => expect(posted).toHaveLength(1))
    expect(posted[0].get('qcm_hints_enabled')).toBe('true')
    expect(posted[0].get('qcm_hint_threshold_1')).toBe('0.25')
    expect(posted[0].get('qcm_hint_threshold_2')).toBe('0.125')
    expect(posted[0].get('qcm_penalty_1')).toBe('0.67')
    expect(posted[0].get('qcm_penalty_2')).toBe('0.33')
  })

  it('NR hôte — question QCM sans indices : qcm_hints_enabled=false, pas de réglages envoyés', async () => {
    editExisting(hostQcm())
    submit()
    await waitFor(() => expect(posted).toHaveLength(1))
    expect(posted[0].get('qcm_hints_enabled')).toBe('false')
    expect(posted[0].has('qcm_hint_threshold_1')).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// Compatibilité : carte héritée à indices
// ---------------------------------------------------------------------------

describe('Éditeur — carte QCM héritée (enregistrée avec indices avant LOT-6) (#279)', () => {
  const legacyQuestion = () => memotionQuestion([qcmCard(LEGACY_HINTS), speedyCard()])

  it('s\'ouvre sans erreur et affiche la ligne d\'information « Indices retirés » sur CETTE carte', () => {
    const { container } = editExisting(legacyQuestion())
    expect(within(cardAt(container, 0)).getByText(INFO_LINE)).toBeInTheDocument()
    expect(within(cardAt(container, 0)).getByText(/n'a plus d'indices/)).toBeInTheDocument()
  })

  it('la ligne d\'information n\'apparaît pas sur les autres cartes', () => {
    const { container } = editExisting(legacyQuestion())
    expect(within(cardAt(container, 1)).queryByText(INFO_LINE)).not.toBeInTheDocument()
  })

  it('la carte héritée est chargée SANS indices : aucune case, aucun réglage', () => {
    const { container } = editExisting(legacyQuestion())
    const card = cardAt(container, 0)
    expect(hintsCheckbox(within(card))).not.toBeInTheDocument()
    expect(within(card).queryByText('Indice 1 :')).not.toBeInTheDocument()
  })

  it('réponses et désignation de la bonne réponse de la carte héritée sont intactes à l\'écran', () => {
    const { container } = editExisting(legacyQuestion())
    const card = cardAt(container, 0)
    expect(within(card).getByPlaceholderText('Reponse A...').value).toBe('Paris')
    expect(within(card).getByPlaceholderText('Reponse D...').value).toBe('Metz')
    expect(within(card).getAllByTitle('Marquer comme bonne reponse')[0]).toHaveTextContent('✓')
  })

  it('ré-enregistrement sans modification : la carte part SANS les 5 clés, contenu conservé', async () => {
    editExisting(legacyQuestion())
    submit()
    const saved = await savedCard('cq')
    for (const key of HINT_KEYS) expect(key in saved).toBe(false)
    expect(saved.QCM_ANSWERS).toEqual(ANSWERS)
    expect(saved.QCM_CORRECT).toBe('RED')
    expect(saved.TYPE).toBe('QCM')
  })

  it('carte avec QCM_HINTS_ENABLED:false mais réglages non défaut : ouverte sans ligne d\'information, ré-enregistrée sans les clés', async () => {
    const { container } = editExisting(memotionQuestion([qcmCard({ ...LEGACY_HINTS, QCM_HINTS_ENABLED: false }), speedyCard()]))
    expect(within(cardAt(container, 0)).queryByText(INFO_LINE)).not.toBeInTheDocument()
    submit()
    const saved = await savedCard('cq')
    for (const key of HINT_KEYS) expect(key in saved).toBe(false)
  })

  it('carte QCM sans aucun indice : pas de ligne d\'information', () => {
    const { container } = editExisting(memotionQuestion())
    expect(within(cardAt(container, 0)).queryByText(INFO_LINE)).not.toBeInTheDocument()
  })

  it('carte héritée à indices ne verrouille PAS le type à cause des indices : réponses vidées ⇒ type libre', () => {
    // Les indices résiduels ne comptent plus dans le verrou : seules les réponses / la désignation verrouillent.
    const { container } = editExisting(memotionQuestion([
      qcmCard({ ...LEGACY_HINTS, QCM_ANSWERS: { RED: '', GREEN: '', YELLOW: '', BLUE: '' }, QCM_CORRECT: '' }),
      speedyCard(),
    ]))
    const card = cardAt(container, 0)
    expect(within(card).getByRole('button', { name: 'Speedy' })).not.toBeDisabled()
  })
})

// ---------------------------------------------------------------------------
// Verrou de type
// ---------------------------------------------------------------------------

describe('Éditeur — verrou de type d\'une carte QCM sans indices (#279)', () => {
  it('carte QCM neuve : déverrouillée (les boutons de type restent actifs)', () => {
    const { container } = renderNewMemotion()
    const card = cardAt(container, 0)
    chooseType(card, 'QCM')
    expect(within(card).getByRole('button', { name: 'Speedy' })).not.toBeDisabled()
    expect(within(card).getByRole('button', { name: 'Memory' })).not.toBeDisabled()
  })

  it('une réponse saisie verrouille la carte QCM', () => {
    const { container } = renderNewMemotion()
    const card = cardAt(container, 0)
    chooseType(card, 'QCM')
    fireEvent.change(within(card).getByPlaceholderText('Reponse A...'), { target: { value: 'Rome' } })
    expect(within(card).getByRole('button', { name: 'Speedy' })).toBeDisabled()
  })

  it('la désignation de la bonne réponse verrouille la carte QCM', () => {
    const { container } = renderNewMemotion()
    const card = cardAt(container, 0)
    chooseType(card, 'QCM')
    fireEvent.click(within(card).getAllByTitle('Marquer comme bonne reponse')[1])
    expect(within(card).getByRole('button', { name: 'Speedy' })).toBeDisabled()
  })
})
