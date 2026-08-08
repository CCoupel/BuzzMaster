import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import ConfigPage from './ConfigPage'

// Tests dérivés de bugfix/config-api-key-help — handoff dev-frontend
// (_work/handoff/dev-frontend-20260808-164252.md, commits 38164db + 5d29182) :
//   1. Popup d'aide par fournisseur (bouton "?" → ApiKeyHelpModal)
//   2. Auto-sélection / désactivation du sélecteur Claude/Groq selon la
//      présence de chaque clé API (helper `pickAutoProvider`, non exporté —
//      testé ici par comportement, pas en unité)
//
// Fichier NEUF, isolé de ConfigPage.test.jsx / ConfigPage.ai.test.jsx à la
// demande du CDP (ces 2 fichiers restent bloqués indéfiniment dans certains
// environnements WSL — cf. rapport _work/reports/test-writer-*.md). Reprend
// les mêmes conventions : GameContext mocké, USBConfigModal/OtaAllModal
// mockés, chaque requête `fetch` explicitement gérée (aucun appel non mocké
// ne doit atteindre le réseau réel).
//
// ⚠️ ConfigPage rend PLUSIEURS boutons "Enregistrer" — chaque test scope ses
// requêtes avec `within()` à partir de la section IA (repérée par le label
// "Clé API Claude" ou "Clé API Groq"), jamais par texte global.

vi.mock('../hooks/GameContext', () => ({
  useGame: () => ({
    teams: {},
    bumpers: {},
    gameState: { backgrounds: [] },
    updateConfig: vi.fn(),
    sendMessage: vi.fn(),
    version: '6.1.1',
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

function claudeSection() {
  return screen.getByText('Clé API Claude').closest('.ai-provider-block')
}
function groqSection() {
  return screen.getByText('Clé API Groq').closest('.ai-provider-block')
}
function providerButtons() {
  const row = screen.getByText('Fournisseur').closest('.ai-provider-row')
  return {
    claudeBtn: within(row).getByRole('button', { name: 'Claude (Anthropic)' }),
    groqBtn: within(row).getByRole('button', { name: 'Groq' }),
  }
}

// `postSpy` lets assertions inspect every POST /config.json body (including
// the silent mount-time re-persist) without caring about GET calls.
function mockFetch({ claudeOk = false, groqOk = false, provider = 'anthropic', postOk = true } = {}) {
  const postSpy = vi.fn()
  global.fetch = vi.fn((url, options) => {
    if (url === '/config.json' && (!options || !options.method)) {
      return Promise.resolve({
        ok: true,
        json: async () => ({
          server: { auto_open_browsers: false, debug: false },
          ai: {
            // Spread FIRST, then override — FULL_AI_SETTINGS also carries a
            // `provider` field (default 'anthropic') which must NOT clobber
            // the `provider` param under test here.
            ...FULL_AI_SETTINGS,
            api_key_configured: claudeOk,
            groq_api_key_configured: groqOk,
            provider,
          },
        }),
      })
    }
    if (url === '/config.json' && options?.method === 'POST') {
      postSpy(JSON.parse(options.body))
      if (postOk) return Promise.resolve({ ok: true, json: async () => ({}) })
      return Promise.resolve({ ok: false, text: async () => 'Erreur serveur' })
    }
    if (url === '/api/wifi/defaults') {
      return Promise.resolve({ ok: true, json: async () => ({ ssid: '', password: '', server_ip: '', server_port: 80 }) })
    }
    if (url === '/api/firmware/buzzclick/version') {
      return Promise.resolve({ ok: true, json: async () => ({ EXISTS: false }) })
    }
    return Promise.resolve({ ok: false, json: async () => ({}), text: async () => '' })
  })
  return postSpy
}

describe('ConfigPage — Popup d\'aide clé API (bugfix/config-api-key-help, commit 38164db)', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('renders a "?" help button next to the Claude API key field', async () => {
    mockFetch()
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    const btn = within(claudeSection()).getByRole('button', { name: 'Comment obtenir une clé API Anthropic ?' })
    expect(btn).toBeInTheDocument()
    expect(btn).toHaveTextContent('?')
  })

  it('renders a "?" help button next to the Groq API key field', async () => {
    mockFetch()
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    const btn = within(groqSection()).getByRole('button', { name: 'Comment obtenir une clé API Groq ?' })
    expect(btn).toBeInTheDocument()
  })

  it('no help modal is rendered before any "?" button is clicked', async () => {
    mockFetch()
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('clicking the Claude "?" button opens the help modal with Anthropic content', async () => {
    mockFetch()
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    fireEvent.click(within(claudeSection()).getByRole('button', { name: 'Comment obtenir une clé API Anthropic ?' }))

    expect(screen.getByRole('heading', { name: 'Obtenir une clé API Claude (Anthropic)' })).toBeInTheDocument()
  })

  it('clicking the Groq "?" button opens the help modal with Groq content', async () => {
    mockFetch()
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    fireEvent.click(within(groqSection()).getByRole('button', { name: 'Comment obtenir une clé API Groq ?' }))

    expect(screen.getByRole('heading', { name: 'Obtenir une clé API Groq' })).toBeInTheDocument()
  })

  it('switches modal content when closing Claude help and opening Groq help', async () => {
    mockFetch()
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    fireEvent.click(within(claudeSection()).getByRole('button', { name: 'Comment obtenir une clé API Anthropic ?' }))
    expect(screen.getByRole('heading', { name: 'Obtenir une clé API Claude (Anthropic)' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Fermer' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()

    fireEvent.click(within(groqSection()).getByRole('button', { name: 'Comment obtenir une clé API Groq ?' }))
    expect(screen.getByRole('heading', { name: 'Obtenir une clé API Groq' })).toBeInTheDocument()
  })

  it('closing the help modal does not trigger any extra /config.json POST', async () => {
    const postSpy = mockFetch()
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    fireEvent.click(within(claudeSection()).getByRole('button', { name: 'Comment obtenir une clé API Anthropic ?' }))
    fireEvent.click(screen.getByRole('button', { name: 'Fermer' }))

    expect(postSpy).not.toHaveBeenCalled()
  })
})

describe('ConfigPage — Auto-sélection fournisseur IA (bugfix/config-api-key-help, commit 5d29182)', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('no key at all: both provider buttons are disabled, no silent re-POST fired', async () => {
    const postSpy = mockFetch({ claudeOk: false, groqOk: false, provider: 'anthropic' })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    const { claudeBtn, groqBtn } = providerButtons()
    expect(claudeBtn).toBeDisabled()
    expect(groqBtn).toBeDisabled()
    expect(postSpy).not.toHaveBeenCalled()
  })

  it('only Claude configured: Claude button active & enabled, Groq disabled with an explanatory title', async () => {
    mockFetch({ claudeOk: true, groqOk: false, provider: 'anthropic' })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    const { claudeBtn, groqBtn } = providerButtons()
    expect(claudeBtn).not.toBeDisabled()
    expect(claudeBtn).toHaveClass('active')
    expect(groqBtn).toBeDisabled()
    expect(groqBtn).toHaveAttribute('title', 'Enregistrez une clé API Groq pour sélectionner ce fournisseur')
  })

  it('only Groq configured: Groq auto-selected & enabled, Claude disabled', async () => {
    mockFetch({ claudeOk: false, groqOk: true, provider: 'anthropic' })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    const { claudeBtn, groqBtn } = providerButtons()
    await waitFor(() => expect(groqBtn).toHaveClass('active'))
    expect(groqBtn).not.toBeDisabled()
    expect(claudeBtn).toBeDisabled()
  })

  it('both keys configured: Claude wins the auto-selection over Groq even if Groq was persisted as provider', async () => {
    mockFetch({ claudeOk: true, groqOk: true, provider: 'groq' })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    const { claudeBtn, groqBtn } = providerButtons()
    await waitFor(() => expect(claudeBtn).toHaveClass('active'))
    expect(claudeBtn).not.toBeDisabled()
    expect(groqBtn).not.toBeDisabled()
  })

  it('mount auto-repersist: resolved provider diverging from the server value silently re-POSTs the corrected provider', async () => {
    const postSpy = mockFetch({ claudeOk: true, groqOk: true, provider: 'groq' })
    render(<ConfigPage />)

    await waitFor(() => {
      expect(postSpy).toHaveBeenCalledWith({ ai: { ...FULL_AI_SETTINGS, provider: 'anthropic' } })
    })
  })

  it('mount: no re-POST when the resolved provider already matches the server value', async () => {
    const postSpy = mockFetch({ claudeOk: true, groqOk: false, provider: 'anthropic' })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    expect(postSpy).not.toHaveBeenCalled()
  })

  it('saving a Claude key while Groq was selected auto-switches the active provider to Claude', async () => {
    const postSpy = mockFetch({ claudeOk: false, groqOk: true, provider: 'groq' })
    render(<ConfigPage />)
    await waitFor(() => expect(providerButtons().groqBtn).toHaveClass('active'))

    const section = claudeSection()
    fireEvent.change(within(section).getByPlaceholderText('sk-ant-...'), { target: { value: 'sk-ant-newkey' } })
    fireEvent.click(within(section).getByText('Enregistrer'))

    await waitFor(() => expect(providerButtons().claudeBtn).toHaveClass('active'))
    await waitFor(() => {
      expect(postSpy).toHaveBeenCalledWith(
        expect.objectContaining({ ai: expect.objectContaining({ provider: 'anthropic' }) })
      )
    })
  })

  it('saving a Groq key while no Claude key exists auto-switches the active provider to Groq', async () => {
    mockFetch({ claudeOk: false, groqOk: false, provider: 'anthropic' })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    const section = groqSection()
    fireEvent.change(within(section).getByPlaceholderText('gsk_...'), { target: { value: 'gsk_newkey' } })
    fireEvent.click(within(section).getByText('Enregistrer'))

    await waitFor(() => expect(providerButtons().groqBtn).toHaveClass('active'))
  })

  it('saving a Groq key while a Claude key already exists does NOT switch the active provider away from Claude', async () => {
    const postSpy = mockFetch({ claudeOk: true, groqOk: false, provider: 'anthropic' })
    render(<ConfigPage />)
    await waitFor(() => expect(providerButtons().claudeBtn).toHaveClass('active'))
    postSpy.mockClear()

    const section = groqSection()
    fireEvent.change(within(section).getByPlaceholderText('gsk_...'), { target: { value: 'gsk_newkey' } })
    fireEvent.click(within(section).getByText('Enregistrer'))

    await waitFor(() => expect(within(section).getByText('✅ Clé configurée')).toBeInTheDocument())
    expect(providerButtons().claudeBtn).toHaveClass('active')
    // Only the key-save POST fires — no extra provider-switch POST.
    expect(postSpy).toHaveBeenCalledTimes(1)
  })

  it('clearing the Claude key while Groq is configured auto-switches to Groq', async () => {
    mockFetch({ claudeOk: true, groqOk: true, provider: 'anthropic' })
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<ConfigPage />)
    await waitFor(() => expect(providerButtons().claudeBtn).toHaveClass('active'))

    fireEvent.click(within(claudeSection()).getByText('Supprimer la clé'))

    await waitFor(() => expect(providerButtons().groqBtn).toHaveClass('active'))
  })

  it('clearing the Claude key while Groq has no key leaves the selection unchanged (both end up disabled)', async () => {
    mockFetch({ claudeOk: true, groqOk: false, provider: 'anthropic' })
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<ConfigPage />)
    await waitFor(() => expect(providerButtons().claudeBtn).toHaveClass('active'))

    fireEvent.click(within(claudeSection()).getByText('Supprimer la clé'))

    await waitFor(() => expect(within(claudeSection()).getByText('⚠️ Aucune clé')).toBeInTheDocument())
    const { claudeBtn, groqBtn } = providerButtons()
    expect(claudeBtn).toHaveClass('active')
    expect(claudeBtn).toBeDisabled()
    expect(groqBtn).toBeDisabled()
  })

  it('clearing the Groq key while Claude is configured auto-switches to Claude', async () => {
    mockFetch({ claudeOk: true, groqOk: true, provider: 'groq' })
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<ConfigPage />)
    await waitFor(() => expect(providerButtons().claudeBtn).toHaveClass('active'))
    // Both keys present → Claude auto-selected on mount regardless of persisted 'groq' (see test above).

    fireEvent.click(within(groqSection()).getByText('Supprimer la clé'))

    await waitFor(() => expect(within(groqSection()).getByText('⚠️ Aucune clé')).toBeInTheDocument())
    expect(providerButtons().claudeBtn).toHaveClass('active')
  })
})
