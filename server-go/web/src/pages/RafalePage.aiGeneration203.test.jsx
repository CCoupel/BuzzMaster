import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import RafalePage from './RafalePage'

// Tests pour les tâches 12 (bouton d'entrée + refetch piloté par la
// progression) et 13 (compteurs de caractères) du plan
// _work/reports/plan-20260901-162105.md — issue #203, milestone v8.1.0,
// contrat contracts/rafale-ai-generation.md §5.3/§6.
//
// Fichier NOUVEAU, compagnon de RafalePage.rafale.test.jsx (27 tests
// pré-#203, non modifiés) — même motif que AIGenerateModal.progress.test.jsx
// vis-à-vis de AIGenerateModal.test.jsx.
//
// `useOptionalGame` (hooks/GameContext.jsx) est mocké ici plutôt que
// wrappé dans un vrai GameProvider : ce dernier ouvrirait une vraie
// WebSocket (useWebSocket.js) dans un environnement jsdom sans serveur —
// mocker le hook est le point d'injection le plus direct pour piloter
// `aiJob`/`cancelAiGeneration` depuis chaque test.

vi.mock('../hooks/GameContext', () => ({
  useOptionalGame: () => globalThis.__mockGameValue ?? null,
}))

const QUESTIONS_FIXTURE = [
  { ID: 'r-001', QUESTION: 'Capitale de l\'Italie ?', ANSWER: 'Rome', CATEGORY: 'GEOGRAPHY', DIFFICULTY: 1, USED: false },
]

function jsonResponse(body, ok = true, status = 200) {
  return Promise.resolve({
    ok,
    status,
    json: () => Promise.resolve(body),
    text: () => Promise.resolve(typeof body === 'string' ? body : JSON.stringify(body)),
  })
}

function mockFetchRouter({ questions = QUESTIONS_FIXTURE, categories = [], config = {} } = {}) {
  const defaultConfig = {
    ai: {
      provider: 'anthropic',
      api_key_configured: false,
      groq_api_key_configured: false,
      inter_batch_delay_ms: 60000,
      max_consecutive_failures: 2,
      max_questions: 200,
    },
    ...config,
  }
  return vi.fn((url, options = {}) => {
    const method = options.method || 'GET'
    if (url === '/api/rafale/questions' && method === 'GET') {
      return jsonResponse({ QUESTIONS: questions, TOTAL: questions.length })
    }
    if (url === '/api/rafale/questions' && method === 'POST') {
      const body = JSON.parse(options.body)
      return jsonResponse({ ID: body.ID || 'r-new' })
    }
    if (url === '/api/categories') {
      return jsonResponse(categories)
    }
    if (url === '/config.json') {
      return jsonResponse(defaultConfig)
    }
    return jsonResponse({}, false, 404)
  })
}

function renderRafalePage(fetchOpts, { withRouter = false, gameValue = null } = {}) {
  global.fetch = mockFetchRouter(fetchOpts)
  globalThis.__mockGameValue = gameValue
  const tree = <RafalePage />
  return render(withRouter ? <MemoryRouter>{tree}</MemoryRouter> : tree)
}

beforeEach(() => {
  vi.clearAllMocks()
  globalThis.__mockGameValue = null
})

describe('RafalePage — bouton "✨ Générer via IA" (contrat §2, tâche 12)', () => {
  it('is disabled and shows the config hint when no provider key is configured', async () => {
    renderRafalePage({ config: { ai: { provider: 'anthropic', api_key_configured: false } } })
    await waitFor(() => expect(screen.getByText('✨ Générer via IA')).toBeInTheDocument())
    expect(screen.getByText('✨ Générer via IA').closest('button')).toBeDisabled()
    expect(screen.getAllByText(/Configurer une clé API dans Paramètres/).length).toBeGreaterThan(0)
  })

  it('is enabled once the ANTHROPIC key is configured and the provider is anthropic', async () => {
    renderRafalePage({ config: { ai: { provider: 'anthropic', api_key_configured: true } } })
    await waitFor(() => expect(screen.getByText('✨ Générer via IA')).toBeInTheDocument())
    expect(screen.getByText('✨ Générer via IA').closest('button')).not.toBeDisabled()
  })

  it('is enabled with the GROQ key when the selected provider is groq, even if the Anthropic key is absent', async () => {
    renderRafalePage({ config: { ai: { provider: 'groq', api_key_configured: false, groq_api_key_configured: true } } })
    await waitFor(() => expect(screen.getByText('✨ Générer via IA')).toBeInTheDocument())
    expect(screen.getByText('✨ Générer via IA').closest('button')).not.toBeDisabled()
  })

  it('stays clickable while a RAFALE job is already RUNNING (re-attachment), even without a configured key', async () => {
    renderRafalePage(
      { config: { ai: { provider: 'anthropic', api_key_configured: false } } },
      { gameValue: { aiJob: { jobId: 'gen-1', state: 'RUNNING', target: 'RAFALE' }, cancelAiGeneration: vi.fn() } },
    )
    await waitFor(() => expect(screen.getByText('✨ Générer via IA')).toBeInTheDocument())
    expect(screen.getByText('✨ Générer via IA').closest('button')).not.toBeDisabled()
  })

  it('stays DISABLED while a QUIZ job is running elsewhere — a Quiz job must not enable the Rafale button (contrat §6 TARGET filter)', async () => {
    renderRafalePage(
      { config: { ai: { provider: 'anthropic', api_key_configured: false } } },
      { gameValue: { aiJob: { jobId: 'gen-1', state: 'RUNNING', target: 'QUIZ' }, cancelAiGeneration: vi.fn() } },
    )
    await waitFor(() => expect(screen.getByText('✨ Générer via IA')).toBeInTheDocument())
    expect(screen.getByText('✨ Générer via IA').closest('button')).toBeDisabled()
  })

  it('clicking the button opens the RafaleAIGenerateModal', async () => {
    renderRafalePage({ config: { ai: { provider: 'anthropic', api_key_configured: true } } }, { withRouter: true })
    await waitFor(() => expect(screen.getByText('✨ Générer via IA')).toBeInTheDocument())
    fireEvent.click(screen.getByText('✨ Générer via IA'))
    expect(screen.getByText('✨ Générer des questions RAFALE via IA')).toBeInTheDocument()
  })
})

describe('RafalePage — compteurs de caractères de l\'éditeur manuel (contrat §5.3, tâche 13)', () => {
  it('shows n/100 under the QUESTION field and n/40 under the ANSWER field', async () => {
    renderRafalePage()
    await waitFor(() => expect(screen.getByText(/1 question/)).toBeInTheDocument())
    expect(screen.getByText('0/100')).toBeInTheDocument()
    expect(screen.getByText('0/40')).toBeInTheDocument()

    fireEvent.change(document.querySelector('textarea'), { target: { value: 'Une question' } })
    expect(screen.getByText('12/100')).toBeInTheDocument()
  })

  it('flags the counter as over-limit past 100 runes for QUESTION and disables submit', async () => {
    renderRafalePage()
    await waitFor(() => expect(screen.getByText(/1 question/)).toBeInTheDocument())
    const textarea = document.querySelector('textarea')
    fireEvent.change(textarea, { target: { value: 'a'.repeat(101) } })

    expect(screen.getByText('101/100')).toHaveClass('over')
    expect(screen.getByText('Ajouter').closest('button')).toBeDisabled()
  })

  it('flags the counter as over-limit past 40 runes for ANSWER and disables submit', async () => {
    renderRafalePage()
    await waitFor(() => expect(screen.getByText(/1 question/)).toBeInTheDocument())
    const answerInput = document.querySelectorAll('.rafale-form input[type="text"]')[0]
    fireEvent.change(answerInput, { target: { value: 'a'.repeat(41) } })

    expect(screen.getByText('41/40')).toHaveClass('over')
    expect(screen.getByText('Ajouter').closest('button')).toBeDisabled()
  })

  it('counts runes, not UTF-16 code units — an accented/emoji-heavy answer at exactly 40 runes is NOT flagged', async () => {
    renderRafalePage()
    await waitFor(() => expect(screen.getByText(/1 question/)).toBeInTheDocument())
    const answerInput = document.querySelectorAll('.rafale-form input[type="text"]')[0]
    fireEvent.change(answerInput, { target: { value: 'é'.repeat(40) } })

    expect(screen.getByText('40/40')).not.toHaveClass('over')
    expect(screen.getByText('Ajouter').closest('button')).not.toBeDisabled()
  })

  it('re-enables submit once the over-limit field is shortened back under the cap', async () => {
    renderRafalePage()
    await waitFor(() => expect(screen.getByText(/1 question/)).toBeInTheDocument())
    const textarea = document.querySelector('textarea')
    fireEvent.change(textarea, { target: { value: 'a'.repeat(101) } })
    expect(screen.getByText('Ajouter').closest('button')).toBeDisabled()

    fireEvent.change(textarea, { target: { value: 'a'.repeat(50) } })
    expect(screen.getByText('50/100')).not.toHaveClass('over')
  })
})

describe('RafalePage — refetch piloté par la progression (contrat §6, tâche 12)', () => {
  it('refetches the reservoir when a RAFALE job\'s CREATED_COUNT increases', async () => {
    const fetchMock = mockFetchRouter()
    global.fetch = fetchMock
    globalThis.__mockGameValue = { aiJob: null, cancelAiGeneration: vi.fn() }
    const { rerender } = render(<RafalePage />)
    await waitFor(() => expect(screen.getByText(/1 question/)).toBeInTheDocument())
    const callsBefore = fetchMock.mock.calls.filter(c => c[0] === '/api/rafale/questions').length

    globalThis.__mockGameValue = { aiJob: { jobId: 'gen-1', state: 'RUNNING', target: 'RAFALE', createdCount: 3 }, cancelAiGeneration: vi.fn() }
    rerender(<RafalePage />)

    await waitFor(() => {
      const callsAfter = fetchMock.mock.calls.filter(c => c[0] === '/api/rafale/questions').length
      expect(callsAfter).toBeGreaterThan(callsBefore)
    })
  })

  it('does NOT refetch on a QUIZ job\'s progress (TARGET filter applies here too, independently of the modal)', async () => {
    const fetchMock = mockFetchRouter()
    global.fetch = fetchMock
    globalThis.__mockGameValue = { aiJob: null, cancelAiGeneration: vi.fn() }
    const { rerender } = render(<RafalePage />)
    await waitFor(() => expect(screen.getByText(/1 question/)).toBeInTheDocument())
    const callsBefore = fetchMock.mock.calls.filter(c => c[0] === '/api/rafale/questions').length

    globalThis.__mockGameValue = { aiJob: { jobId: 'gen-quiz', state: 'RUNNING', target: 'QUIZ', createdCount: 3 }, cancelAiGeneration: vi.fn() }
    rerender(<RafalePage />)

    // Give any (incorrect) effect a chance to fire, then assert it didn't.
    await new Promise(r => setTimeout(r, 20))
    const callsAfter = fetchMock.mock.calls.filter(c => c[0] === '/api/rafale/questions').length
    expect(callsAfter).toBe(callsBefore)
  })
})
