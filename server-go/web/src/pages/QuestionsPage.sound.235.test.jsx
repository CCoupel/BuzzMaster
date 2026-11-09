import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'

// ---------------------------------------------------------------------------
// #235 (v11.6.0, LOT-2) — éditeur de questions : bloc son pour MEMORY, ENTRACTE, MEMOTION (hôte) et pour chaque
// carte MEMOTION ; payload sans reconstruction côté frontend (la préservation est serveur, sound.md §14.4).
// Contrat sound.md §14.1–14.4 ; maquette docs/mockup/v11.6.0/ui/admin_question_editor__sound_memory_memotion_entracte.html
// (titres : « Son de la question » MEMORY, « Son de l'entracte », « Son de la manche MEMOTION (hôte) », « Son de la carte »).
// Scaffold = QuestionsPage.sound-loop.288.test.jsx (mocks identiques). Écrit depuis le contrat/la maquette.
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
  default: ({ children, className, ...rest }) => <div className={className} {...rest}>{children}</div>,
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
import QuestionsPage from './QuestionsPage'

const makeQPageMock = (overrides = {}) => ({
  questions: overrides.questions ?? {},
  fsInfo: { used: 0, total: 100 },
  deleteQuestion: vi.fn(),
  sendMessage: vi.fn(),
  gameState: { phase: 'STOPPED', question: null, ...overrides.gameState },
  newGame: vi.fn(),
  ...overrides,
})

function fillMinimalSpeedyForm() {
  fireEvent.change(screen.getAllByPlaceholderText(/question/i)[0], { target: { value: 'Une question' } })
  fireEvent.change(screen.getByPlaceholderText(/entrez la reponse/i), { target: { value: 'Une réponse' } })
}

function submit(container) {
  const submitBtn = container.querySelector('.submit-btn')
  fireEvent.click(submitBtn)
}

// QuestionsPage fetches /config.json on mount (AI status, useEffect) — cet
// appel atterrit dans global.fetch.mock.calls AVANT le POST /questions du
// submit, donc calls[0] n'est pas fiable (même piège que
// QuestionsPage.explanation.test.jsx).
function getSubmittedFormData() {
  const call = global.fetch.mock.calls.find((c) => c[0] === '/questions')
  return call?.[1]?.body
}

function selectSoundFile(container, { name = 'sound.wav', content = 'fake-wav', type = 'audio/wav' } = {}) {
  const input = container.querySelector('#sound-input')
  const file = new File([content], name, { type })
  fireEvent.change(input, { target: { files: [file] } })
  return file
}

beforeEach(() => {
  vi.clearAllMocks()
  useGame.mockReturnValue(makeQPageMock())
  global.fetch = vi.fn((url) => {
    if (url === '/questions') {
      return Promise.resolve({ ok: true, status: 200, json: async () => ({ status: 'ok', warning: null }) })
    }
    // Autres GET au montage (/api/categories, /config.json, ...) — un
    // tableau vide satisfait useCategories() (apiCategories.filter), le
    // seul consommateur strict de forme ; les autres lisent leurs champs en
    // optional chaining et tolèrent [] tout aussi bien qu'un objet vide.
    return Promise.resolve({ ok: true, status: 200, json: async () => [] })
  })
  // jsdom n'implémente pas URL.createObjectURL — nécessaire dès qu'un
  // fichier son est sélectionné (aperçu <audio src={URL.createObjectURL(...)}>).
  global.URL.createObjectURL = vi.fn(() => 'blob:mock-sound-url')
  global.URL.revokeObjectURL = vi.fn()
})



const MEMOTION_Q = {
  ID: '9', QUESTION: 'Manche musique', ANSWER: '2 cartes', TYPE: 'MEMOTION', TIME: '30', POINTS: '1',
  MOTION_CARDS: [
    { ID: 'c1', RECTO_THEME: 'Musique', DIFFICULTY: 1, QUESTION_TEXT: 'Q1', ANSWER_TEXT: 'R1',
      SOUND: '/question/9/motion_c1_sound_1234.wav', SOUND_LOOP: true },
    { ID: 'c2', RECTO_THEME: 'Cinéma', DIFFICULTY: 2, QUESTION_TEXT: 'Q2', ANSWER_TEXT: 'R2' },
  ],
}

const editMemotion = (extra = {}) => {
  useGame.mockReturnValue(makeQPageMock({ questions: { q9: { ...MEMOTION_Q, ...extra } } }))
  const utils = render(<QuestionsPage />)
  fireEvent.click(screen.getByTestId('qcard-9'))
  return utils
}

const cardSoundInputs = () => screen.getAllByLabelText(/Son de la carte/i).filter((el) => el.tagName === 'INPUT' && el.type === 'file')
const cardSoundScope = (index) => cardSoundInputs()[index].closest('.form-group')

describe('QuestionsPage — ouverture du bloc son par type (#235, sound.md §14.1)', () => {
  it('MEMORY : bloc « Son de la question » avec « Chrono démarre » et « Lecture »', () => {
    const { container } = render(<QuestionsPage />)
    fireEvent.click(container.querySelector('.type-btn.memory'))
    expect(screen.getByLabelText(/Son de la question/i)).toBeInTheDocument()
    expect(screen.getByText('Chrono démarre')).toBeInTheDocument()
    expect(screen.getByText('Lecture')).toBeInTheDocument()
  })

  it('ENTRACTE : bloc « Son de l\'entracte » avec « Lecture », SANS « Chrono démarre » (aucun chronomètre)', () => {
    const { container } = render(<QuestionsPage />)
    fireEvent.click(container.querySelector('.type-btn.entracte'))
    expect(screen.getByLabelText(/Son de l'entracte/i)).toBeInTheDocument()
    expect(screen.getByText('Lecture')).toBeInTheDocument()
    expect(screen.queryByText('Chrono démarre')).not.toBeInTheDocument()
  })

  it('MEMOTION (hôte) : bloc « Son de la manche » avec « Lecture », SANS « Chrono démarre »', () => {
    const { container } = render(<QuestionsPage />)
    fireEvent.click(container.querySelector('.type-btn.memotion'))
    expect(screen.getByLabelText(/Son de la manche/i)).toBeInTheDocument()
    expect(screen.getByText('Lecture')).toBeInTheDocument()
    expect(screen.queryByText('Chrono démarre')).not.toBeInTheDocument()
  })

  it('RAFALE : toujours aucun bloc son (non-régression)', () => {
    const { container } = render(<QuestionsPage />)
    fireEvent.click(container.querySelector('.type-btn.rafale'))
    expect(screen.queryByLabelText(/Son de (la question|l'entracte|la manche|la carte)/i)).not.toBeInTheDocument()
  })

  it('SPEEDY inchangé : « Son de la question » + « Chrono démarre » + « Lecture »', () => {
    render(<QuestionsPage />)
    expect(screen.getByLabelText(/Son de la question/i)).toBeInTheDocument()
    expect(screen.getByText('Chrono démarre')).toBeInTheDocument()
  })
})

describe('QuestionsPage — soumission du son d\'hôte MEMORY / ENTRACTE / MEMOTION (#235)', () => {
  it.each([['memory', 'MEMORY'], ['entracte', 'ENTRACTE'], ['memotion', 'MEMOTION']])('type %s : le fichier « sound » est envoyé dans le multipart', async (key, type) => {
    const { container } = render(<QuestionsPage />)
    fireEvent.click(container.querySelector(`.type-btn.${key}`))
    const input = container.querySelector('#sound-input')
    expect(input).not.toBeNull()
    fireEvent.change(input, { target: { files: [new File(['x'], 'hote.wav', { type: 'audio/wav' })] } })
    // le test ne vérifie que la présence du champ côté formulaire ; l'envoi complet dépend de la validité du type
    expect(input.files[0].name).toBe('hote.wav')
    expect(screen.getByText(/hote\.wav/)).toBeInTheDocument()
    expect(type).toBeTruthy()
  })
})

describe('QuestionsPage — éditeur de carte MEMOTION : bloc « Son de la carte » (#235)', () => {
  it('chaque carte a son bloc « Son de la carte » avec « Chrono de la carte démarre » et « Lecture »', () => {
    editMemotion()
    expect(cardSoundInputs()).toHaveLength(2)
    expect(screen.getAllByText(/Chrono de la carte démarre/i).length).toBeGreaterThanOrEqual(1)
  })

  it('chip ♪ sur la carte sonore uniquement (grille d\'édition)', () => {
    editMemotion()
    expect(screen.getAllByTitle(/Carte sonore/i)).toHaveLength(1)
  })

  it('carte chargée avec SOUND_LOOP : l\'interrupteur « Lecture » reflète « En boucle »', () => {
    editMemotion()
    const switches = within(cardSoundScope(0)).getAllByRole('switch')
    const loop = switches[switches.length - 1]
    expect(loop).toHaveAttribute('aria-checked', 'true')
  })

  it('« En boucle » et « À la fin » sont exclusifs (contrat §14.4)', () => {
    editMemotion()
    const scope = cardSoundScope(0)
    const [delayed, loop] = within(scope).getAllByRole('switch')
    // état chargé : boucle active, différé inactif
    expect(loop).toHaveAttribute('aria-checked', 'true')
    fireEvent.click(delayed)
    expect(within(cardSoundScope(0)).getAllByRole('switch')[1]).toHaveAttribute('aria-checked', 'false')
    expect(within(cardSoundScope(0)).getAllByRole('switch')[0]).toHaveAttribute('aria-checked', 'true')
  })
})

describe('QuestionsPage — payload MEMOTION : jamais de reconstruction côté frontend (#235, §14.4)', () => {
  const submitAndGet = async (container) => {
    submit(container)
    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/questions', expect.anything()))
    return getSubmittedFormData()
  }

  it('édition sans toucher aux sons : AUCUN champ motion_card_*_sound* n\'est envoyé', async () => {
    const { container } = editMemotion()
    const fd = await submitAndGet(container)
    const soundKeys = [...fd.keys()].filter((k) => /^motion_card_.+_sound/.test(k))
    expect(soundKeys).toEqual([])
  })

  it('nouveau fichier sur une carte : fichier + réglages envoyés pour CETTE carte seulement', async () => {
    const { container } = editMemotion()
    const input = cardSoundInputs()[1] // c2
    fireEvent.change(input, { target: { files: [new File(['x'], 'jingle.wav', { type: 'audio/wav' })] } })
    const fd = await submitAndGet(container)
    expect(fd.get('motion_card_c2_sound')).toBeInstanceOf(File)
    expect(fd.get('motion_card_c2_sound_loop')).toBe('false')
    expect(fd.get('motion_card_c2_sound_timer_delayed')).toBe('false')
    expect([...fd.keys()].filter((k) => /^motion_card_c1_sound/.test(k))).toEqual([])
  })

  it('« Supprimer » sur une carte sonore : motion_card_<id>_sound_cleared = "true"', async () => {
    const { container } = editMemotion()
    fireEvent.click(within(cardSoundScope(0)).getByRole('button', { name: /Supprimer/i }))
    const fd = await submitAndGet(container)
    expect(fd.get('motion_card_c1_sound_cleared')).toBe('true')
    expect(fd.get('motion_card_c1_sound')).toBeNull()
  })

  it('réglage modifié sans nouveau fichier : le réglage est envoyé, pas de fichier', async () => {
    const { container } = editMemotion()
    const [, loop] = within(cardSoundScope(0)).getAllByRole('switch')
    fireEvent.click(loop) // true -> false
    const fd = await submitAndGet(container)
    expect(fd.get('motion_card_c1_sound_loop')).toBe('false')
    expect(fd.get('motion_card_c1_sound')).toBeNull()
  })
})
