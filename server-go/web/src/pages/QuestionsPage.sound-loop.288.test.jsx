import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

// ---------------------------------------------------------------------------
// #288 (v11.4.0, LOT-3) — éditeur de question : ligne « Lecture : Une fois / En boucle »
// (contracts/sound.md §13.2, maquette admin_sounds_manager__countdown_loop §02 AMENDÉE : plus d'exclusivité).
// Sélecteur normatif : role="switch", aria-label="Jouer le son en boucle jusqu'à la fin du chronomètre".
// Scaffold = QuestionsPage.sound.test.jsx (mocks identiques).
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


const LOOP_LABEL = /boucle jusqu'à la fin du chronomètre/i
const loopSwitch = () => screen.getByRole('switch', { name: LOOP_LABEL })

describe('QuestionsPage — ligne « Lecture » (#288)', () => {
  it('visible sous « Chrono démarre » mais DÉSACTIVÉE tant qu\'aucun son n\'est attaché', () => {
    render(<QuestionsPage />)
    expect(screen.getByText('Lecture')).toBeInTheDocument()
    expect(loopSwitch()).toBeDisabled()
  })

  it('défaut = « Une fois » : aria-checked=false et conseil « joué une fois, puis s\'arrête »', () => {
    const { container } = render(<QuestionsPage />)
    selectSoundFile(container)
    expect(loopSwitch()).not.toBeDisabled()
    expect(loopSwitch()).toHaveAttribute('aria-checked', 'false')
    expect(screen.getByText('Le son est joué une fois, puis s\'arrête.')).toBeInTheDocument()
  })

  it('basculer sur « En boucle » : aria-checked=true et conseil « rejoué jusqu\'à la fin du chronomètre »', () => {
    const { container } = render(<QuestionsPage />)
    selectSoundFile(container)
    fireEvent.click(loopSwitch())
    expect(loopSwitch()).toHaveAttribute('aria-checked', 'true')
    expect(screen.getByText('Le son est rejoué jusqu\'à la fin du chronomètre.')).toBeInTheDocument()
  })

  it('boucle ET « À la fin » : plus d\'exclusivité — les deux restent actifs, conseil spécifique', () => {
    const { container } = render(<QuestionsPage />)
    selectSoundFile(container)
    fireEvent.click(loopSwitch())
    fireEvent.click(screen.getByRole('switch', { name: /fin du son/i }))
    expect(loopSwitch()).not.toBeDisabled()
    expect(loopSwitch()).toHaveAttribute('aria-checked', 'true')
    expect(screen.getByRole('switch', { name: /fin du son/i })).toHaveAttribute('aria-checked', 'true')
    expect(
      screen.getByText('Le chronomètre démarre à la fin du premier passage ; le son continue en boucle.'),
    ).toBeInTheDocument()
    // Décision utilisateur du 2026-11-05 : l'avertissement d'exclusivité de la maquette est retiré.
    expect(screen.queryByText(/n'est pas disponible quand le chrono démarre à la fin/i)).not.toBeInTheDocument()
  })

  it('le conseil ne mentionne plus l\'arrêt « au buzz » (au buzz tout continue)', () => {
    const { container } = render(<QuestionsPage />)
    selectSoundFile(container)
    fireEvent.click(loopSwitch())
    expect(container.textContent).not.toMatch(/ou au buzz/i)
  })

  it.each(['memory', 'memotion', 'rafale', 'entracte'])('type %s : la ligne « Lecture » est ABSENTE (pas de section Son)', (typeKey) => {
    const { container } = render(<QuestionsPage />)
    fireEvent.click(container.querySelector(`.type-btn.${typeKey}`))
    expect(screen.queryByRole('switch', { name: LOOP_LABEL })).not.toBeInTheDocument()
  })
})

describe('QuestionsPage — SOUND_LOOP : chargement, reset, soumission (#288)', () => {
  const questionsWithLoop = {
    q1: { ID: '1', QUESTION: 'Capitale ?', ANSWER: 'Paris', TYPE: 'SPEEDY', SOUND: '/question/1/sound_1234.wav', SOUND_LOOP: true },
    q2: { ID: '2', QUESTION: 'Fleuve ?', ANSWER: 'Seine', TYPE: 'SPEEDY', SOUND: '/question/2/sound_5678.wav' },
  }

  it('édition d\'une question SOUND_LOOP=true : la ligne reflète « En boucle »', () => {
    useGame.mockReturnValue(makeQPageMock({ questions: questionsWithLoop }))
    render(<QuestionsPage />)
    fireEvent.click(screen.getByTestId('qcard-1'))
    expect(loopSwitch()).toHaveAttribute('aria-checked', 'true')
  })

  it('SOUND_LOOP absent : « Une fois »', () => {
    useGame.mockReturnValue(makeQPageMock({ questions: questionsWithLoop }))
    render(<QuestionsPage />)
    fireEvent.click(screen.getByTestId('qcard-2'))
    expect(loopSwitch()).toHaveAttribute('aria-checked', 'false')
  })

  it('charger une question en boucle puis une question sans boucle ne laisse pas l\'état précédent', () => {
    useGame.mockReturnValue(makeQPageMock({ questions: questionsWithLoop }))
    render(<QuestionsPage />)
    fireEvent.click(screen.getByTestId('qcard-1'))
    fireEvent.click(screen.getByTestId('qcard-2'))
    expect(loopSwitch()).toHaveAttribute('aria-checked', 'false')
  })

  it('"sound_loop" est TOUJOURS envoyé : "false" par défaut', async () => {
    const { container } = render(<QuestionsPage />)
    fillMinimalSpeedyForm()
    submit(container)
    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/questions', expect.anything()))
    expect(getSubmittedFormData().get('sound_loop')).toBe('false')
  })

  it('bascule « En boucle » : "sound_loop" vaut "true"', async () => {
    const { container } = render(<QuestionsPage />)
    fillMinimalSpeedyForm()
    selectSoundFile(container)
    fireEvent.click(loopSwitch())
    submit(container)
    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/questions', expect.anything()))
    const fd = getSubmittedFormData()
    expect(fd.get('sound_loop')).toBe('true')
    expect(fd.get('sound_timer_delayed')).toBe('false')
  })

  it('boucle + « À la fin » : les deux champs partent ensemble à "true"', async () => {
    const { container } = render(<QuestionsPage />)
    fillMinimalSpeedyForm()
    selectSoundFile(container)
    fireEvent.click(loopSwitch())
    fireEvent.click(screen.getByRole('switch', { name: /fin du son/i }))
    submit(container)
    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/questions', expect.anything()))
    const fd = getSubmittedFormData()
    expect(fd.get('sound_loop')).toBe('true')
    expect(fd.get('sound_timer_delayed')).toBe('true')
  })

  it('réédition sans toucher au son : la valeur chargée (SOUND_LOOP=true) est renvoyée telle quelle', async () => {
    useGame.mockReturnValue(makeQPageMock({ questions: questionsWithLoop }))
    const { container } = render(<QuestionsPage />)
    fireEvent.click(screen.getByTestId('qcard-1'))
    submit(container)
    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/questions', expect.anything()))
    expect(getSubmittedFormData().get('sound_loop')).toBe('true')
  })

  it('après « Supprimer » le son, la ligne « Lecture » redevient désactivée', () => {
    useGame.mockReturnValue(makeQPageMock({ questions: questionsWithLoop }))
    render(<QuestionsPage />)
    fireEvent.click(screen.getByTestId('qcard-1'))
    fireEvent.click(screen.getByText('Supprimer'))
    expect(loopSwitch()).toBeDisabled()
  })
})
