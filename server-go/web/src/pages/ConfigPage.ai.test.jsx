import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import ConfigPage from './ConfigPage'

// Tests dérivés de contracts/ai-generation.md (#8, v6.0.0) §2 et §9, et de
// _work/mockups/8-generateur-ia.md §9 ("Section IA — ConfigPage").
//
// Fichier NEUF — pas de collision avec ConfigPage.test.jsx (dev-frontend n'écrit
// pas de tests, cf. répartition du plan). Reprend les conventions de
// ConfigPage.test.jsx : GameContext mocké minimalement, Button/Card réels,
// USBConfigModal/OtaAllModal mockés (hors scope IA, coûteux à monter).
//
// ⚠️ ConfigPage rend PLUSIEURS boutons "Enregistrer" (params serveur, WiFi,
// IA, néon) — chaque test scope ses requêtes avec `within()` à partir de la
// section IA (repérée par le label "Clé API Claude"), jamais par texte global.

vi.mock('../hooks/GameContext', () => ({
  useGame: () => ({
    teams: {},
    bumpers: {},
    gameState: { backgrounds: [] },
    updateConfig: vi.fn(),
    sendMessage: vi.fn(),
    version: '6.0.0',
    firmwareInfo: { EXISTS: true, IS_MERGED: true, VERSION: '3.1.1', FILENAME: 'buzzclick-v3.1.1.bin', SIZE: 512000 },
  })
}))

vi.mock('../components/USBConfigModal', () => ({
  default: ({ onClose }) => (
    <div data-testid="usb-modal"><button onClick={onClose}>Close USB Modal</button></div>
  )
}))

vi.mock('../components/TeamCard', () => ({
  OtaAllModal: ({ onClose }) => (
    <div data-testid="ota-all-modal"><button onClick={onClose}>Close OTA Modal</button></div>
  )
}))

// Locates the "IA" config-section card, scoping every query below it so the
// multiple other "Enregistrer" buttons on the page never collide.
function getAiSection() {
  const label = screen.getByText('Clé API Claude')
  return label.closest('.config-section')
}

function mockFetchImplementation({ apiKeyConfigured = false, postOk = true, postBody = null } = {}) {
  global.fetch = vi.fn((url, options) => {
    if (url === '/config.json' && (!options || options.method === undefined)) {
      return Promise.resolve({
        ok: true,
        json: async () => ({
          server: { auto_open_browsers: false, debug: false },
          ai: { api_key_configured: apiKeyConfigured },
        }),
      })
    }
    if (url === '/config.json' && options?.method === 'POST') {
      if (postOk) {
        return Promise.resolve({ ok: true, json: async () => (postBody || {}) })
      }
      return Promise.resolve({ ok: false, text: async () => 'Format de clé API invalide (attendu : sk-ant-...)' })
    }
    if (url === '/api/wifi/defaults') {
      return Promise.resolve({ ok: true, json: async () => ({ ssid: '', password: '', server_ip: '', server_port: 80 }) })
    }
    if (url === '/api/firmware/buzzclick/version') {
      return Promise.resolve({ ok: true, json: async () => ({ EXISTS: false }) })
    }
    return Promise.resolve({ ok: false })
  })
}

describe('ConfigPage — Section IA (#8)', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('renders the "IA" section title and hint', async () => {
    mockFetchImplementation({ apiKeyConfigured: false })
    render(<ConfigPage />)

    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())
    expect(screen.getByText(/Clé API Anthropic \(Claude\)/)).toBeInTheDocument()
  })

  it('shows "Aucune clé configurée" badge when ai.api_key_configured is false', async () => {
    mockFetchImplementation({ apiKeyConfigured: false })
    render(<ConfigPage />)

    await waitFor(() => {
      expect(screen.getByText('⚠️ Aucune clé configurée')).toBeInTheDocument()
    })
    expect(screen.queryByText('✅ Clé configurée')).not.toBeInTheDocument()
  })

  it('shows "Clé configurée" badge when ai.api_key_configured is true', async () => {
    mockFetchImplementation({ apiKeyConfigured: true })
    render(<ConfigPage />)

    await waitFor(() => {
      expect(screen.getByText('✅ Clé configurée')).toBeInTheDocument()
    })
  })

  it('never receives the actual API key from the server — only api_key_configured (contract §2, CA3)', async () => {
    mockFetchImplementation({ apiKeyConfigured: true })
    render(<ConfigPage />)

    await waitFor(() => expect(screen.getByText('✅ Clé configurée')).toBeInTheDocument())

    const section = getAiSection()
    const input = within(section).getByPlaceholderText('••••••••')
    // The field must start empty — the server never sends the key back, so
    // there is nothing to pre-fill it with (contract §9: placeholder only).
    expect(input.value).toBe('')
  })

  it('shows the "sk-ant-..." placeholder when no key is configured', async () => {
    mockFetchImplementation({ apiKeyConfigured: false })
    render(<ConfigPage />)
    await waitFor(() => {
      const section = getAiSection()
      expect(within(section).getByPlaceholderText('sk-ant-...')).toBeInTheDocument()
    })
  })

  it('shows "Supprimer la clé" only when a key is configured', async () => {
    mockFetchImplementation({ apiKeyConfigured: false })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('⚠️ Aucune clé configurée')).toBeInTheDocument())

    const section = getAiSection()
    expect(within(section).queryByText('Supprimer la clé')).not.toBeInTheDocument()
  })

  it('renders "Supprimer la clé" when a key is configured', async () => {
    mockFetchImplementation({ apiKeyConfigured: true })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('✅ Clé configurée')).toBeInTheDocument())

    const section = getAiSection()
    expect(within(section).getByText('Supprimer la clé')).toBeInTheDocument()
  })

  // Fix bloquant (_work/handoff/task-dev-frontend-20260806-103759.md) : la
  // section `ai` d'un POST est remplacée INTÉGRALEMENT par le backend, sauf
  // les 2 clés API résolues individuellement (contract ai-generation.md §0,
  // même règle que neon_effect). Un payload partiel ne contenant que la clé
  // remettait provider/batch_size/etc à leurs défauts à chaque sauvegarde —
  // ConfigPage ré-émet donc désormais la section `ai` complète (aiSettings,
  // peuplée par le GET de montage) à chaque POST, jamais un payload partiel.
  const FULL_AI_SETTINGS = {
    provider: 'anthropic',
    model: 'claude-opus-5',
    timeout_seconds: 300,
    max_questions: 200,
    batch_size: 20,
    inter_batch_delay_ms: 60000,
    context_token_budget: 1500,
    max_consecutive_failures: 2,
    groq_model: 'openai/gpt-oss-120b',
  }

  it('saving a non-empty key POSTs the full ai section including anthropic_api_key (contract §0)', async () => {
    mockFetchImplementation({ apiKeyConfigured: false })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    const section = getAiSection()
    const input = within(section).getByPlaceholderText('sk-ant-...')
    fireEvent.change(input, { target: { value: 'sk-ant-newkey123' } })
    fireEvent.click(within(section).getByText('Enregistrer'))

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        '/config.json',
        expect.objectContaining({
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ ai: { ...FULL_AI_SETTINGS, anthropic_api_key: 'sk-ant-newkey123' } }),
        })
      )
    })
  })

  it('saving with an empty field POSTs the full ai section WITHOUT anthropic_api_key — preserves the stored key server-side (contract §0/§2, CA2)', async () => {
    mockFetchImplementation({ apiKeyConfigured: true })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('✅ Clé configurée')).toBeInTheDocument())

    const section = getAiSection()
    fireEvent.click(within(section).getByText('Enregistrer'))

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        '/config.json',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ ai: FULL_AI_SETTINGS }),
        })
      )
    })
  })

  it('clears the input and flips the badge to configured after a successful non-empty save', async () => {
    mockFetchImplementation({ apiKeyConfigured: false })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('⚠️ Aucune clé configurée')).toBeInTheDocument())

    const section = getAiSection()
    const input = within(section).getByPlaceholderText('sk-ant-...')
    fireEvent.change(input, { target: { value: 'sk-ant-newkey123' } })
    fireEvent.click(within(section).getByText('Enregistrer'))

    await waitFor(() => {
      expect(screen.getByText('✅ Clé configurée')).toBeInTheDocument()
    })
    const refreshedSection = getAiSection()
    const refreshedInput = within(refreshedSection).getByPlaceholderText('••••••••')
    expect(refreshedInput.value).toBe('')
  })

  it('shows a success toast after saving', async () => {
    mockFetchImplementation({ apiKeyConfigured: false })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    const section = getAiSection()
    fireEvent.change(within(section).getByPlaceholderText('sk-ant-...'), { target: { value: 'sk-ant-x' } })
    fireEvent.click(within(section).getByText('Enregistrer'))

    await waitFor(() => {
      expect(screen.getByText('Clé API enregistrée')).toBeInTheDocument()
    })
  })

  it('shows an error toast when the server rejects the key format (400)', async () => {
    mockFetchImplementation({ apiKeyConfigured: false, postOk: false })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    const section = getAiSection()
    fireEvent.change(within(section).getByPlaceholderText('sk-ant-...'), { target: { value: 'not-a-valid-key' } })
    fireEvent.click(within(section).getByText('Enregistrer'))

    await waitFor(() => {
      expect(screen.getByText(/Erreur: Format de clé API invalide/)).toBeInTheDocument()
    })
  })

  it('clicking "Supprimer la clé" asks for confirmation, then POSTs clear_api_key:true', async () => {
    mockFetchImplementation({ apiKeyConfigured: true })
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('✅ Clé configurée')).toBeInTheDocument())

    const section = getAiSection()
    fireEvent.click(within(section).getByText('Supprimer la clé'))

    expect(confirmSpy).toHaveBeenCalled()
    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        '/config.json',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ ai: { ...FULL_AI_SETTINGS, clear_api_key: true } }),
        })
      )
    })
    await waitFor(() => {
      expect(screen.getByText('⚠️ Aucune clé configurée')).toBeInTheDocument()
    })
    confirmSpy.mockRestore()
  })

  it('does nothing when "Supprimer la clé" confirmation is declined', async () => {
    mockFetchImplementation({ apiKeyConfigured: true })
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false)
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('✅ Clé configurée')).toBeInTheDocument())

    global.fetch.mockClear()
    const section = getAiSection()
    fireEvent.click(within(section).getByText('Supprimer la clé'))

    expect(confirmSpy).toHaveBeenCalled()
    // No POST /config.json should follow a declined confirmation.
    await new Promise(resolve => setTimeout(resolve, 0))
    expect(global.fetch).not.toHaveBeenCalledWith('/config.json', expect.objectContaining({ method: 'POST' }))
    confirmSpy.mockRestore()
  })
})
