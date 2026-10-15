import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import QuestionsPage from './QuestionsPage'

// ---------------------------------------------------------------------------
// QuestionsPage — éditeur de carte MEMOTION : ligne « Points : » (POINTS_RULE),
// #190 (LOT-5, v11.2.2, contrat question-types.md §6.2, maquette
// admin_question_editor__memotion_card_points_rule).
//
// Règles couvertes (plan #190, « Critères d'acceptation ») :
//  - la ligne « Points : » est présente sous « Valeur de la carte : » pour chaque type
//  - options filtrées par type ; choisir le défaut n'écrit AUCUN POINTS_RULE
//  - un autre mode écrit {MODE} ou {MODE, VALUE} (VALUE seulement FIXED / PER_UNIT)
//  - Montant fixe / Par unité sans VALUE entière 1–999 : enregistrement bloqué
//    côté client (message « Indiquez un montant… », aria-invalid, aucun POST)
//  - changement de type : règle inapplicable → défaut + avertissement ; applicable → conservée
//  - chargement : règle explicite restaurée, défaut du type = rien d'écrit
//  - refus serveur (HTTP 400 POINTS_RULE_*) affiché, formulaire conservé
//
// Mêmes conventions de mock que QuestionsPage.motionMemory.test.jsx /
// QuestionsPage.wrong-penalty.254.test.jsx. Cible rôles / aria-* / libellés.
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

// Manche MEMOTION valide (≥ 2 cartes avec thème) dont les deux cartes SPEEDY
// ne portent aucun contenu propre ⇒ type NON verrouillé (on peut le changer).
const memotionQuestion = (cards, over = {}) => ({
  ID: '30', TYPE: 'MEMOTION', QUESTION: 'Manche points', ANSWER: '—', POINTS: '1', TIME: '30',
  MOTION_MODE: 'SOLO', MOTION_CONFIG,
  MOTION_CARDS: cards ?? [
    { ID: 'c1', RECTO_THEME: 'Capitales', DIFFICULTY: 2 },
    { ID: 'c2', RECTO_THEME: 'Fleuves', DIFFICULTY: 1 },
  ],
  ...over,
})

const MEMORY_CARD = (over = {}) => ({
  ID: 'cm', TYPE: 'MEMORY', RECTO_THEME: 'Paires', DIFFICULTY: 3,
  MEMORY_PAIRS: [
    { ID: 1, CARD1: { TEXT: 'A' }, CARD2: { TEXT: 'A' } },
    { ID: 2, CARD1: { TEXT: 'B' }, CARD2: { TEXT: 'B' } },
  ],
  ...over,
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

const renderNewMemotion = () => {
  useGame.mockReturnValue(makeQPageMock())
  const utils = render(<QuestionsPage />)
  fireEvent.click(screen.getByText(/memotion/i).closest('button'))
  return utils
}

const cardAt = (container, index) => container.querySelectorAll('.memotion-card-item')[index]
const pointsGroup = (card) => within(card).getByRole('group', { name: 'Barème de points de la carte' })
const modeSelect = (card) => within(card).getByRole('combobox', { name: 'Mode de barème' })
const optionLabels = (card) => within(modeSelect(card)).getAllByRole('option').map(o => o.textContent)
const chooseMode = (card, value) => fireEvent.change(modeSelect(card), { target: { value } })
const setAmount = (card, label, value) => fireEvent.change(within(card).getByRole('spinbutton', { name: label }), { target: { value: String(value) } })
const chooseType = (card, label) => fireEvent.click(within(card).getByRole('button', { name: label }))
const submit = () => fireEvent.click(screen.getByRole('button', { name: /^(Modifier|Ajouter)$/ }))
const savedCards = async () => {
  await waitFor(() => expect(posted).toHaveLength(1))
  return JSON.parse(posted[0].get('motion_cards'))
}
const savedCard = async (id) => (await savedCards()).find(c => c.ID === id)

// ---------------------------------------------------------------------------
// Présence et options
// ---------------------------------------------------------------------------

describe('Éditeur MEMOTION — ligne « Points » de chaque carte (#190)', () => {
  it('chaque carte (nouvelle manche, 2 cartes par défaut) porte son groupe « Barème de points de la carte »', () => {
    renderNewMemotion()
    expect(screen.getAllByRole('group', { name: 'Barème de points de la carte' })).toHaveLength(2)
  })

  it('la ligne est SOUS « Valeur de la carte : » dans la face RECTO', () => {
    const { container } = renderNewMemotion()
    const card = cardAt(container, 0)
    const label = within(card).getByText('Valeur de la carte :')
    expect(label.compareDocumentPosition(pointsGroup(card)) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('carte SPEEDY (type par défaut) : « Étoiles (défaut) » et « Montant fixe », défaut sélectionné', () => {
    const { container } = renderNewMemotion()
    const card = cardAt(container, 0)
    expect(optionLabels(card)).toEqual(['Étoiles (défaut)', 'Montant fixe'])
    expect(modeSelect(card).selectedOptions[0].textContent).toBe('Étoiles (défaut)')
  })

  it('carte QCM : mêmes 2 options', () => {
    const { container } = renderNewMemotion()
    const card = cardAt(container, 0)
    chooseType(card, 'QCM')
    expect(optionLabels(card)).toEqual(['Étoiles (défaut)', 'Montant fixe'])
  })

  it.each(['Memory', 'Rafale'])('carte %s : 4 options, « Prorata des étoiles (défaut) » en premier', (label) => {
    const { container } = renderNewMemotion()
    const card = cardAt(container, 0)
    chooseType(card, label)
    expect(optionLabels(card)).toEqual([
      'Prorata des étoiles (défaut)',
      'Étoiles pleines (tout ou rien)',
      'Montant fixe',
      'Par unité',
    ])
    expect(modeSelect(card).selectedOptions[0].textContent).toBe('Prorata des étoiles (défaut)')
  })

  it('l\'aide « → N pts » suit le barème de la manche (MOTION_CONFIG) et la valeur ★ de la carte', () => {
    const { container } = editExisting(memotionQuestion(undefined, { MOTION_CONFIG: { POINTS_1_STAR: 1, POINTS_2_STAR: 4, POINTS_3_STAR: 9 } }))
    expect(within(pointsGroup(cardAt(container, 0))).getByText('→ 4 pts (★★)')).toBeInTheDocument()
    expect(within(pointsGroup(cardAt(container, 1))).getByText('→ 1 pt (★)')).toBeInTheDocument()
  })

  it('MEMORY : l\'aide mentionne le nombre de paires de la carte', () => {
    const { container } = editExisting(memotionQuestion([MEMORY_CARD(), { ID: 'c2', RECTO_THEME: 'Autre', DIFFICULTY: 1 }]))
    expect(within(pointsGroup(cardAt(container, 0))).getByText('→ jusqu\'à 5 pts, au prorata des paires trouvées (2 paires)')).toBeInTheDocument()
  })

  it('le champ de montant n\'apparaît que pour Montant fixe / Par unité', () => {
    const { container } = renderNewMemotion()
    const card = cardAt(container, 0)
    expect(within(card).queryByRole('spinbutton', { name: 'Montant' })).not.toBeInTheDocument()
    chooseMode(card, 'FIXED')
    expect(within(card).getByRole('spinbutton', { name: 'Montant' })).toBeInTheDocument()
    chooseMode(card, '')
    expect(within(card).queryByRole('spinbutton', { name: 'Montant' })).not.toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// Sérialisation (payload multipart motion_cards)
// ---------------------------------------------------------------------------

describe('Éditeur MEMOTION — POINTS_RULE dans le payload motion_cards (#190)', () => {
  it('défauts inchangés : AUCUNE carte n\'envoie POINTS_RULE (la clé est absente, pas null)', async () => {
    editExisting(memotionQuestion())
    submit()
    const cards = await savedCards()
    expect(cards).toHaveLength(2)
    for (const card of cards) {
      expect('POINTS_RULE' in card).toBe(false)
    }
  })

  it('Montant fixe 10 sur une carte QCM : { MODE: \'FIXED\', VALUE: 10 }', async () => {
    const { container } = editExisting(memotionQuestion())
    const card = cardAt(container, 0)
    chooseType(card, 'QCM')
    chooseMode(card, 'FIXED')
    setAmount(card, 'Montant', 10)
    submit()
    const saved = await savedCard('c1')
    expect(saved.TYPE).toBe('QCM')
    expect(saved.POINTS_RULE).toEqual({ MODE: 'FIXED', VALUE: 10 })
  })

  it('Montant fixe sur une carte SPEEDY : { MODE: \'FIXED\', VALUE } (VALUE nombre, pas chaîne)', async () => {
    const { container } = editExisting(memotionQuestion())
    const card = cardAt(container, 0)
    chooseMode(card, 'FIXED')
    setAmount(card, 'Montant', 7)
    submit()
    const saved = await savedCard('c1')
    expect(saved.POINTS_RULE).toEqual({ MODE: 'FIXED', VALUE: 7 })
    expect(typeof saved.POINTS_RULE.VALUE).toBe('number')
  })

  it('Étoiles pleines sur MEMORY : { MODE: \'STARS\' } sans VALUE', async () => {
    const { container } = editExisting(memotionQuestion([MEMORY_CARD(), { ID: 'c2', RECTO_THEME: 'Autre', DIFFICULTY: 1 }]))
    chooseMode(cardAt(container, 0), 'STARS')
    submit()
    const saved = await savedCard('cm')
    expect(saved.POINTS_RULE).toEqual({ MODE: 'STARS' })
    expect('VALUE' in saved.POINTS_RULE).toBe(false)
  })

  it('Par unité 2 sur une carte RAFALE : { MODE: \'PER_UNIT\', VALUE: 2 }', async () => {
    const { container } = editExisting(memotionQuestion())
    const card = cardAt(container, 0)
    chooseType(card, 'Rafale')
    chooseMode(card, 'PER_UNIT')
    setAmount(card, 'Points par unité', 2)
    submit()
    const saved = await savedCard('c1')
    expect(saved.TYPE).toBe('RAFALE')
    expect(saved.POINTS_RULE).toEqual({ MODE: 'PER_UNIT', VALUE: 2 })
  })

  it('revenir au défaut après un autre choix : POINTS_RULE de nouveau ABSENT (jamais écrit)', async () => {
    const { container } = editExisting(memotionQuestion())
    const card = cardAt(container, 0)
    chooseMode(card, 'FIXED')
    setAmount(card, 'Montant', 10)
    chooseMode(card, '')
    submit()
    const saved = await savedCard('c1')
    expect('POINTS_RULE' in saved).toBe(false)
  })

  it('le choix d\'une carte n\'affecte pas les autres : seule la carte modifiée porte POINTS_RULE', async () => {
    const { container } = editExisting(memotionQuestion())
    const first = cardAt(container, 0)
    chooseMode(first, 'FIXED')
    setAmount(first, 'Montant', 4)
    submit()
    const cards = await savedCards()
    expect(cards.find(c => c.ID === 'c1').POINTS_RULE).toEqual({ MODE: 'FIXED', VALUE: 4 })
    expect('POINTS_RULE' in cards.find(c => c.ID === 'c2')).toBe(false)
  })

  it('« Valeur de la carte » (DIFFICULTY) reste indépendante de la règle : FIXED 10 conserve la difficulté choisie', async () => {
    const { container } = editExisting(memotionQuestion())
    const card = cardAt(container, 0)
    chooseMode(card, 'FIXED')
    setAmount(card, 'Montant', 10)
    submit()
    const saved = await savedCard('c1')
    expect(saved.DIFFICULTY).toBe(2)
  })
})

// ---------------------------------------------------------------------------
// Blocage côté client
// ---------------------------------------------------------------------------

describe('Éditeur MEMOTION — Montant fixe / Par unité sans montant valide : enregistrement bloqué (#190)', () => {
  const settle = () => new Promise(r => setTimeout(r, 30))

  it('Montant fixe sans montant : message « Indiquez un montant… », champ aria-invalid, aucun POST', async () => {
    const { container } = editExisting(memotionQuestion())
    const card = cardAt(container, 0)
    chooseMode(card, 'FIXED')
    submit()
    await settle()
    expect(posted).toHaveLength(0)
    expect(within(card).getByRole('alert')).toHaveTextContent(/Indiquez un montant/)
    expect(within(card).getByRole('spinbutton', { name: 'Montant' })).toHaveAttribute('aria-invalid', 'true')
  })

  it('Par unité sans montant : message « Indiquez un montant par unité (1 à 999) », aucun POST', async () => {
    const { container } = editExisting(memotionQuestion())
    const card = cardAt(container, 0)
    chooseType(card, 'Rafale')
    chooseMode(card, 'PER_UNIT')
    submit()
    await settle()
    expect(posted).toHaveLength(0)
    expect(within(card).getByRole('alert')).toHaveTextContent(/Indiquez un montant par unité \(1 à 999\)/)
  })

  it.each(['0', '1000', '-3'])('montant %s hors bornes : aucun POST', async (value) => {
    const { container } = editExisting(memotionQuestion())
    const card = cardAt(container, 0)
    chooseMode(card, 'FIXED')
    setAmount(card, 'Montant', value)
    submit()
    await settle()
    expect(posted).toHaveLength(0)
    expect(within(card).getByRole('alert')).toBeInTheDocument()
  })

  it('l\'erreur ne touche que la carte concernée', async () => {
    const { container } = editExisting(memotionQuestion())
    chooseMode(cardAt(container, 0), 'FIXED')
    submit()
    await settle()
    expect(within(cardAt(container, 1)).queryByRole('alert')).not.toBeInTheDocument()
  })

  it('corriger le montant puis ré-enregistrer : le POST part avec la valeur corrigée', async () => {
    const { container } = editExisting(memotionQuestion())
    const card = cardAt(container, 0)
    chooseMode(card, 'FIXED')
    submit()
    await settle()
    expect(posted).toHaveLength(0)
    setAmount(card, 'Montant', 12)
    submit()
    const saved = await savedCard('c1')
    expect(saved.POINTS_RULE).toEqual({ MODE: 'FIXED', VALUE: 12 })
  })

  it('montants limites 1 et 999 acceptés', async () => {
    const { container } = editExisting(memotionQuestion())
    chooseMode(cardAt(container, 0), 'FIXED')
    setAmount(cardAt(container, 0), 'Montant', 1)
    chooseMode(cardAt(container, 1), 'FIXED')
    setAmount(cardAt(container, 1), 'Montant', 999)
    submit()
    const cards = await savedCards()
    expect(cards.find(c => c.ID === 'c1').POINTS_RULE.VALUE).toBe(1)
    expect(cards.find(c => c.ID === 'c2').POINTS_RULE.VALUE).toBe(999)
  })
})

// ---------------------------------------------------------------------------
// Changement de type
// ---------------------------------------------------------------------------

describe('Éditeur MEMOTION — changement de type de carte (#190)', () => {
  it('RAFALE « Par unité » → SPEEDY : règle inapplicable remise au défaut + avertissement bref', () => {
    const { container } = editExisting(memotionQuestion())
    const card = cardAt(container, 0)
    chooseType(card, 'Rafale')
    chooseMode(card, 'PER_UNIT')
    setAmount(card, 'Points par unité', 2)
    chooseType(card, 'Speedy')

    expect(modeSelect(card).selectedOptions[0].textContent).toBe('Étoiles (défaut)')
    expect(within(card).queryByRole('spinbutton')).not.toBeInTheDocument()
    expect(within(card).getByRole('status')).toBeInTheDocument()
  })

  it('RAFALE « Par unité » → SPEEDY : POINTS_RULE absent du payload enregistré', async () => {
    const { container } = editExisting(memotionQuestion())
    const card = cardAt(container, 0)
    chooseType(card, 'Rafale')
    chooseMode(card, 'PER_UNIT')
    setAmount(card, 'Points par unité', 2)
    chooseType(card, 'Speedy')
    submit()
    const saved = await savedCard('c1')
    expect(saved.TYPE).toBe('SPEEDY')
    expect('POINTS_RULE' in saved).toBe(false)
  })

  it('MEMORY « Montant fixe 10 » → QCM : règle encore applicable, CONSERVÉE (mode et montant), sans avertissement', () => {
    const { container } = editExisting(memotionQuestion())
    const card = cardAt(container, 0)
    chooseType(card, 'Memory')
    chooseMode(card, 'FIXED')
    setAmount(card, 'Montant', 10)
    chooseType(card, 'QCM')

    expect(modeSelect(card).selectedOptions[0].textContent).toBe('Montant fixe')
    expect(within(card).getByRole('spinbutton', { name: 'Montant' }).value).toBe('10')
    expect(within(card).queryByRole('status')).not.toBeInTheDocument()
  })

  it('MEMORY « Étoiles pleines » → SPEEDY : STARS = défaut du binaire, normalisé en défaut sans avertissement', () => {
    const { container } = editExisting(memotionQuestion())
    const card = cardAt(container, 0)
    chooseType(card, 'Memory')
    chooseMode(card, 'STARS')
    chooseType(card, 'Speedy')

    expect(modeSelect(card).selectedOptions[0].textContent).toBe('Étoiles (défaut)')
    expect(within(card).queryByRole('status')).not.toBeInTheDocument()
  })

  it('règle par défaut + changement de type : le défaut suit le nouveau type (« Prorata » sur Memory/Rafale)', () => {
    const { container } = editExisting(memotionQuestion())
    const card = cardAt(container, 0)
    chooseType(card, 'Memory')
    expect(modeSelect(card).selectedOptions[0].textContent).toBe('Prorata des étoiles (défaut)')
    chooseType(card, 'Speedy')
    expect(modeSelect(card).selectedOptions[0].textContent).toBe('Étoiles (défaut)')
  })

  it('l\'avertissement disparaît au prochain changement de barème', () => {
    const { container } = editExisting(memotionQuestion())
    const card = cardAt(container, 0)
    chooseType(card, 'Rafale')
    chooseMode(card, 'PER_UNIT')
    setAmount(card, 'Points par unité', 2)
    chooseType(card, 'Speedy')
    expect(within(card).getByRole('status')).toBeInTheDocument()
    chooseMode(card, 'FIXED')
    expect(within(card).queryByRole('status')).not.toBeInTheDocument()
  })

  it('la règle est un champ de CARTE : elle ne verrouille jamais le type (le type reste modifiable après un choix de barème)', () => {
    const { container } = editExisting(memotionQuestion())
    const card = cardAt(container, 0)
    chooseMode(card, 'FIXED')
    setAmount(card, 'Montant', 10)
    expect(within(card).getByRole('button', { name: 'QCM' })).not.toBeDisabled()
    expect(within(card).getByRole('button', { name: 'Memory' })).not.toBeDisabled()
  })
})

// ---------------------------------------------------------------------------
// Chargement d'une carte existante
// ---------------------------------------------------------------------------

describe('Éditeur MEMOTION — chargement des cartes existantes (#190)', () => {
  it('carte QCM avec { MODE: FIXED, VALUE: 10 } : « Montant fixe » sélectionné, montant 10', () => {
    const { container } = editExisting(memotionQuestion([
      {
        ID: 'c1', TYPE: 'QCM', RECTO_THEME: 'Capitales', DIFFICULTY: 1, POINTS_RULE: { MODE: 'FIXED', VALUE: 10 },
        QCM_ANSWERS: { RED: 'a', GREEN: 'b', YELLOW: 'c', BLUE: 'd' }, QCM_CORRECT: 'RED',
      },
      { ID: 'c2', RECTO_THEME: 'Fleuves', DIFFICULTY: 1 },
    ]))
    const card = cardAt(container, 0)
    expect(modeSelect(card).selectedOptions[0].textContent).toBe('Montant fixe')
    expect(within(card).getByRole('spinbutton', { name: 'Montant' }).value).toBe('10')
    expect(within(card).getByText('→ 10 pts, quelle que soit la valeur ★')).toBeInTheDocument()
  })

  it('carte MEMORY avec { MODE: PER_UNIT, VALUE: 3 } : « Par unité », « Points par unité » = 3', () => {
    const { container } = editExisting(memotionQuestion([
      MEMORY_CARD({ POINTS_RULE: { MODE: 'PER_UNIT', VALUE: 3 } }),
      { ID: 'c2', RECTO_THEME: 'Autre', DIFFICULTY: 1 },
    ]))
    const card = cardAt(container, 0)
    expect(modeSelect(card).selectedOptions[0].textContent).toBe('Par unité')
    expect(within(card).getByRole('spinbutton', { name: 'Points par unité' }).value).toBe('3')
  })

  it('carte MEMORY avec { MODE: STARS } (tout-ou-rien) : « Étoiles pleines (tout ou rien) » conservé', () => {
    const { container } = editExisting(memotionQuestion([
      MEMORY_CARD({ POINTS_RULE: { MODE: 'STARS' } }),
      { ID: 'c2', RECTO_THEME: 'Autre', DIFFICULTY: 1 },
    ]))
    expect(modeSelect(cardAt(container, 0)).selectedOptions[0].textContent).toBe('Étoiles pleines (tout ou rien)')
  })

  it('carte sans POINTS_RULE : défaut de son type (« Prorata » pour MEMORY, « Étoiles » pour SPEEDY)', () => {
    const { container } = editExisting(memotionQuestion([
      MEMORY_CARD(),
      { ID: 'c2', RECTO_THEME: 'Fleuves', DIFFICULTY: 1 },
    ]))
    expect(modeSelect(cardAt(container, 0)).selectedOptions[0].textContent).toBe('Prorata des étoiles (défaut)')
    expect(modeSelect(cardAt(container, 1)).selectedOptions[0].textContent).toBe('Étoiles (défaut)')
  })

  it('carte MEMORY avec { MODE: STARS_PRORATA } explicite (idempotence) : affichée comme le défaut', () => {
    const { container } = editExisting(memotionQuestion([
      MEMORY_CARD({ POINTS_RULE: { MODE: 'STARS_PRORATA' } }),
      { ID: 'c2', RECTO_THEME: 'Autre', DIFFICULTY: 1 },
    ]))
    expect(modeSelect(cardAt(container, 0)).selectedOptions[0].textContent).toBe('Prorata des étoiles (défaut)')
  })

  it('carte SPEEDY avec { MODE: STARS } (= défaut explicite) : normalisée en défaut, ré-enregistrée SANS POINTS_RULE', async () => {
    const { container } = editExisting(memotionQuestion([
      { ID: 'c1', RECTO_THEME: 'Capitales', DIFFICULTY: 2, POINTS_RULE: { MODE: 'STARS' } },
      { ID: 'c2', RECTO_THEME: 'Fleuves', DIFFICULTY: 1 },
    ]))
    expect(modeSelect(cardAt(container, 0)).selectedOptions[0].textContent).toBe('Étoiles (défaut)')
    submit()
    const saved = await savedCard('c1')
    expect('POINTS_RULE' in saved).toBe(false)
  })

  it('règle inapplicable stockée à la main (SPEEDY + PER_UNIT) : jamais refusée à la lecture, remise au défaut + avertissement', () => {
    const { container } = editExisting(memotionQuestion([
      { ID: 'c1', RECTO_THEME: 'Capitales', DIFFICULTY: 2, POINTS_RULE: { MODE: 'PER_UNIT', VALUE: 2 } },
      { ID: 'c2', RECTO_THEME: 'Fleuves', DIFFICULTY: 1 },
    ]))
    const card = cardAt(container, 0)
    expect(modeSelect(card).selectedOptions[0].textContent).toBe('Étoiles (défaut)')
    expect(within(card).getByRole('status')).toBeInTheDocument()
  })

  it('ré-enregistrement sans modification : une règle explicite applicable est renvoyée à l\'identique', async () => {
    editExisting(memotionQuestion([
      { ID: 'c1', RECTO_THEME: 'Capitales', DIFFICULTY: 1, POINTS_RULE: { MODE: 'FIXED', VALUE: 10 } },
      { ID: 'c2', RECTO_THEME: 'Fleuves', DIFFICULTY: 1 },
    ]))
    submit()
    const cards = await savedCards()
    expect(cards.find(c => c.ID === 'c1').POINTS_RULE).toEqual({ MODE: 'FIXED', VALUE: 10 })
    expect('POINTS_RULE' in cards.find(c => c.ID === 'c2')).toBe(false)
  })

  it('non-régression : une question MEMOTION historique (cartes sans TYPE ni POINTS_RULE) s\'ouvre et se ré-enregistre sans POINTS_RULE', async () => {
    editExisting(memotionQuestion([
      { ID: 'c1', RECTO_THEME: 'Histoire', DIFFICULTY: 1, QUESTION_TEXT: 'Q ?', ANSWER_TEXT: 'R' },
      { ID: 'c2', RECTO_THEME: 'Sport', DIFFICULTY: 3, QUESTION_TEXT: 'Q2 ?', ANSWER_TEXT: 'R2' },
    ]))
    submit()
    const cards = await savedCards()
    expect(cards).toHaveLength(2)
    for (const card of cards) {
      expect('POINTS_RULE' in card).toBe(false)
    }
  })
})

// ---------------------------------------------------------------------------
// Refus serveur
// ---------------------------------------------------------------------------

describe('Éditeur MEMOTION — refus serveur POINTS_RULE_* (#190)', () => {
  const SERVER_MESSAGE = 'POINTS_RULE_MODE_NOT_APPLICABLE: card c1 (TYPE=SPEEDY) refuse le mode PER_UNIT'

  beforeEach(() => {
    global.fetch = vi.fn((url, opts) => {
      const isSave = String(url).includes('/questions') && opts?.method === 'POST'
      if (isSave) {
        posted.push(opts.body)
        return Promise.resolve({ ok: false, status: 400, text: () => Promise.resolve(SERVER_MESSAGE), json: () => Promise.resolve({}) })
      }
      return Promise.resolve({ ok: false, json: () => Promise.resolve({}), text: () => Promise.resolve('') })
    })
  })

  it('HTTP 400 : le message du serveur (code nommé) est affiché dans l\'éditeur', async () => {
    editExisting(memotionQuestion())
    submit()
    expect(await screen.findByText(/POINTS_RULE_MODE_NOT_APPLICABLE/)).toBeInTheDocument()
  })

  it('HTTP 400 : le formulaire est CONSERVÉ (règle choisie toujours là, bouton d\'enregistrement toujours proposé)', async () => {
    const { container } = editExisting(memotionQuestion())
    const card = cardAt(container, 0)
    chooseMode(card, 'FIXED')
    setAmount(card, 'Montant', 10)
    submit()
    await screen.findByText(/POINTS_RULE_MODE_NOT_APPLICABLE/)

    expect(modeSelect(cardAt(container, 0)).selectedOptions[0].textContent).toBe('Montant fixe')
    expect(within(cardAt(container, 0)).getByRole('spinbutton', { name: 'Montant' }).value).toBe('10')
    expect(screen.getByRole('button', { name: /^(Modifier|Ajouter)$/ })).toBeInTheDocument()
  })
})
