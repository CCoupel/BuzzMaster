import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import QuestionsPage from './QuestionsPage'

// ---------------------------------------------------------------------------
// QuestionsPage — éditeur du son de question (v11.1, #219, tâche 12 du plan
// _work/reports/plan-20260922-103848.md §7). Contrat sound.md §10 /
// http-endpoints.md §Questions, maquette question-sound-219.html §01.
// Patron d'assertion FormData : QuestionsPage.explanation.test.jsx (mocks
// identiques).
//
// Dispatché en correctif de revue (finding MAJEUR,
// _work/reports/code-review-frontend-20260922-150000.md) — le plan initial
// ne listait aucun test JS pour ce lot.
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

// ---------------------------------------------------------------------------
// CA1 — champ visible, restreint à SPEEDY/QCM/ARDOISE (plan §0.2, même
// garde que les champs Image).
// ---------------------------------------------------------------------------

describe('QuestionsPage — visibilité du champ son (garde de type, plan §0.2)', () => {
  it('type SPEEDY (défaut) : le champ son est présent', () => {
    render(<QuestionsPage />)
    expect(screen.getByLabelText(/Son de la question/i)).toBeInTheDocument()
  })

  it.each(['memory', 'memotion', 'rafale', 'entracte'])('type %s : le champ son est ABSENT (aucune garde propre, réutilise la garde Image existante)', (typeKey) => {
    const { container } = render(<QuestionsPage />)
    fireEvent.click(container.querySelector(`.type-btn.${typeKey}`))
    expect(screen.queryByLabelText(/Son de la question/i)).not.toBeInTheDocument()
  })

  it('la bascule "Chronomètre de réponse" est ABSENTE tant qu\'aucun son n\'est attaché (maquette §01)', () => {
    render(<QuestionsPage />)
    expect(screen.queryByText('Chronomètre de réponse')).not.toBeInTheDocument()
  })

  it('choisir un fichier son fait apparaître la bascule "Chronomètre de réponse"', () => {
    const { container } = render(<QuestionsPage />)
    selectSoundFile(container)
    expect(screen.getByText('Chronomètre de réponse')).toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// Soumission — FormData (patron QuestionsPage.explanation.test.jsx).
// ---------------------------------------------------------------------------

describe('QuestionsPage — soumission du son (POST /questions)', () => {
  it('un nouveau fichier choisi : data.append("sound", file) porte le fichier réel', async () => {
    const { container } = render(<QuestionsPage />)
    fillMinimalSpeedyForm()
    const file = selectSoundFile(container)
    submit(container)

    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/questions', expect.anything()))
    const formData = getSubmittedFormData()
    expect(formData).toBeInstanceOf(FormData)
    expect(formData.get('sound')).toBe(file)
  })

  it('aucun fichier choisi, aucune suppression : ni "sound" ni "sound_cleared" ne sont envoyés (préservation implicite, R3)', async () => {
    const { container } = render(<QuestionsPage />)
    fillMinimalSpeedyForm()
    submit(container)

    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/questions', expect.anything()))
    const formData = getSubmittedFormData()
    expect(formData.has('sound')).toBe(false)
    expect(formData.has('sound_cleared')).toBe(false)
  })

  it('"sound_timer_delayed" est TOUJOURS envoyé, même sans son attaché (valeur zéro = comportement actuel)', async () => {
    const { container } = render(<QuestionsPage />)
    fillMinimalSpeedyForm()
    submit(container)

    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/questions', expect.anything()))
    const formData = getSubmittedFormData()
    expect(formData.get('sound_timer_delayed')).toBe('false')
  })

  it('bascule "Démarrer à la fin du son" cochée : "sound_timer_delayed" vaut "true"', async () => {
    const { container } = render(<QuestionsPage />)
    fillMinimalSpeedyForm()
    selectSoundFile(container)
    fireEvent.click(screen.getByText('Démarrer à la fin du son'))
    submit(container)

    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/questions', expect.anything()))
    const formData = getSubmittedFormData()
    expect(formData.get('sound_timer_delayed')).toBe('true')
  })
})

// ---------------------------------------------------------------------------
// Suppression réelle — "Supprimer" sur un son existant (jamais les deux
// champs sound/sound_cleared à la fois, handleSoundChange remet
// soundCleared à false dès qu'un nouveau fichier est choisi).
// ---------------------------------------------------------------------------

describe('QuestionsPage — édition et suppression d\'un son existant', () => {
  const questionsWithSound = {
    q1: { ID: '1', QUESTION: 'Capitale ?', ANSWER: 'Paris', TYPE: 'SPEEDY', SOUND: '/question/1/sound_1234.wav', SOUND_TIMER_DELAYED: true },
  }

  it('cliquer sur une question avec SOUND pré-remplit l\'aperçu (existingSound) et la bascule (soundTimerDelayed)', () => {
    useGame.mockReturnValue(makeQPageMock({ questions: questionsWithSound }))
    render(<QuestionsPage />)
    fireEvent.click(screen.getByTestId('qcard-1'))

    expect(screen.getByText('sound_1234.wav')).toBeInTheDocument()
    // La bascule doit refléter SOUND_TIMER_DELAYED=true déjà persisté.
    const delayedRadio = screen.getByText('Démarrer à la fin du son').closest('label').querySelector('input[type="radio"]')
    expect(delayedRadio.checked).toBe(true)
  })

  it('cliquer "Supprimer" sur le son existant, puis soumettre : sound_cleared="true", aucun champ "sound"', async () => {
    useGame.mockReturnValue(makeQPageMock({ questions: questionsWithSound }))
    const { container } = render(<QuestionsPage />)
    fireEvent.click(screen.getByTestId('qcard-1'))

    fireEvent.click(screen.getByText('Supprimer'))
    submit(container)

    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/questions', expect.anything()))
    const formData = getSubmittedFormData()
    expect(formData.get('sound_cleared')).toBe('true')
    expect(formData.has('sound')).toBe(false)
  })

  it('après suppression, la bascule "Chronomètre de réponse" redevient invisible (plus de son attaché)', () => {
    useGame.mockReturnValue(makeQPageMock({ questions: questionsWithSound }))
    render(<QuestionsPage />)
    fireEvent.click(screen.getByTestId('qcard-1'))
    expect(screen.getByText('Chronomètre de réponse')).toBeInTheDocument()

    fireEvent.click(screen.getByText('Supprimer'))
    expect(screen.queryByText('Chronomètre de réponse')).not.toBeInTheDocument()
  })

  it('choisir un NOUVEAU fichier après "Supprimer" annule la suppression en attente (sound_cleared repasse à false)', async () => {
    useGame.mockReturnValue(makeQPageMock({ questions: questionsWithSound }))
    const { container } = render(<QuestionsPage />)
    fireEvent.click(screen.getByTestId('qcard-1'))
    fireEvent.click(screen.getByText('Supprimer'))

    const file = selectSoundFile(container, { name: 'nouveau.wav' })
    submit(container)

    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/questions', expect.anything()))
    const formData = getSubmittedFormData()
    expect(formData.get('sound')).toBe(file)
    expect(formData.has('sound_cleared')).toBe(false)
  })

  it('réédition SANS toucher au son : "number" est envoyé, ni "sound" ni "sound_cleared" (préservation R3)', async () => {
    useGame.mockReturnValue(makeQPageMock({ questions: questionsWithSound }))
    const { container } = render(<QuestionsPage />)
    fireEvent.click(screen.getByTestId('qcard-1'))

    fireEvent.change(screen.getAllByPlaceholderText(/question/i)[0], { target: { value: 'Capitale de la France ?' } })
    submit(container)

    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/questions', expect.anything()))
    const formData = getSubmittedFormData()
    expect(formData.get('number')).toBe('1')
    expect(formData.has('sound')).toBe(false)
    expect(formData.has('sound_cleared')).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// CA2 — refus nommé (corps HTTP non-2xx affiché tel quel).
// ---------------------------------------------------------------------------

describe('QuestionsPage — refus serveur nommé (CA2)', () => {
  it('réponse 400 : le message exact du serveur est affiché sous le champ', async () => {
    global.fetch = vi.fn((url) => {
      if (url === '/questions') {
        return Promise.resolve({
          ok: false, status: 400,
          text: async () => 'ce son dure 42.0 s — la limite pour un son de question est de 30 s',
        })
      }
      return Promise.resolve({ ok: true, status: 200, json: async () => [] })
    })
    const { container } = render(<QuestionsPage />)
    fillMinimalSpeedyForm()
    selectSoundFile(container)
    submit(container)

    await waitFor(() => {
      const err = container.querySelector('.sound-error-message')
      expect(err).not.toBeNull()
      expect(err.textContent).toContain('ce son dure 42.0 s — la limite pour un son de question est de 30 s')
    })
  })

  it('un refus ne réinitialise PAS le formulaire (permet la correction)', async () => {
    global.fetch = vi.fn((url) => {
      if (url === '/questions') {
        return Promise.resolve({ ok: false, status: 400, text: async () => 'refusé' })
      }
      return Promise.resolve({ ok: true, status: 200, json: async () => [] })
    })
    const { container } = render(<QuestionsPage />)
    fillMinimalSpeedyForm()
    submit(container)

    await waitFor(() => expect(container.querySelector('.sound-error-message')).not.toBeNull())
    expect(screen.getAllByPlaceholderText(/question/i)[0]).toHaveValue('Une question')
  })
})

// ---------------------------------------------------------------------------
// Avertissement contextuel — succès (200) avec `warning` non-null (durée du
// son vs Question.TIME, mode simultané).
// ---------------------------------------------------------------------------

describe('QuestionsPage — avertissement contextuel (succès avec warning)', () => {
  it('réponse 200 avec warning non-null : affiché en toast', async () => {
    global.fetch = vi.fn((url) => {
      if (url === '/questions') {
        return Promise.resolve({
          ok: true, status: 200,
          json: async () => ({ status: 'ok', warning: 'ce son dure 28.0 s, plus longtemps que le temps de réponse (20s)' }),
        })
      }
      return Promise.resolve({ ok: true, status: 200, json: async () => [] })
    })
    const { container } = render(<QuestionsPage />)
    fillMinimalSpeedyForm()
    selectSoundFile(container)
    submit(container)

    await waitFor(() => {
      const toast = container.querySelector('.wifi-toast-warning')
      expect(toast).not.toBeNull()
      expect(toast.textContent).toContain('ce son dure 28.0 s')
    })
  })

  it('réponse 200 avec warning:null : aucun toast affiché', async () => {
    const { container } = render(<QuestionsPage />)
    fillMinimalSpeedyForm()
    selectSoundFile(container)
    submit(container)

    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/questions', expect.anything()))
    expect(container.querySelector('.wifi-toast-warning')).toBeNull()
  })
})
