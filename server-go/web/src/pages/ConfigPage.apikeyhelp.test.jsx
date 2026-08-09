import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import ConfigPage from './ConfigPage'

// Tests dérivés de bugfix/config-api-key-help :
//   1. Popup d'aide par fournisseur (bouton "?" → ApiKeyHelpModal, commit 38164db)
//   2. Sélecteur de fournisseur Claude/Groq (handoff dev-frontend
//      dev-frontend-20260809-105500.md, tâche #7, commit 1d5de9f)
//
// RÉÉCRITURE (sur demande explicite du CDP, suite au commit 1d5de9f) : la
// version précédente de ce fichier (commit 1241802) testait l'auto-sélection
// de fournisseur (`pickAutoProvider`, boutons `disabled` selon présence de
// clé, bascule automatique on save/clear) — cette logique a été entièrement
// supprimée sur décision CDP/utilisateur suite à un bug critique trouvé en
// revue (voir commits 5d29182 → 3a595c3 → 1d5de9f). Le nouveau comportement
// est strictement manuel et beaucoup plus simple :
//   - les 2 boutons du sélecteur "Fournisseur" sont **toujours cliquables**,
//     quelle que soit la présence d'une clé ;
//   - **une seule carte fournisseur est montée dans le DOM** à la fois
//     (Claude XOR Groq, jamais les deux) — enregistrer/supprimer une clé ne
//     change JAMAIS quelle carte est affichée, ni le fournisseur actif ;
//   - la sélection n'est modifiée QUE par un clic explicite sur le
//     sélecteur, persisté immédiatement via `handleProviderChange` (POST
//     /config.json) — aucun POST silencieux au montage ni ailleurs.
//
// ⚠️ Une seule carte `.ai-provider-block` existe dans le DOM à un instant
// donné (contrairement à l'ancienne version où les 2 étaient toujours
// montées) — `visibleBlock()` ci-dessous la retrouve sans avoir besoin de
// désambiguïser Claude/Groq comme le faisait l'ancien fichier.

vi.mock('../hooks/GameContext', () => ({
  useGame: () => ({
    teams: {},
    bumpers: {},
    gameState: { backgrounds: [] },
    updateConfig: vi.fn(),
    sendMessage: vi.fn(),
    version: '6.1.2',
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

// The single mounted `.ai-provider-block` — whichever provider is currently
// selected (Claude XOR Groq, never both).
function visibleBlock() {
  return screen.getByText(/^Clé API (Claude|Groq)$/).closest('.ai-provider-block')
}
function providerButtons() {
  const row = screen.getByText('Fournisseur').closest('.ai-provider-row')
  return {
    claudeBtn: within(row).getByRole('button', { name: 'Claude (Anthropic)' }),
    groqBtn: within(row).getByRole('button', { name: 'Groq' }),
  }
}

// `postSpy` lets assertions inspect every POST /config.json body without
// caring about the GET calls.
function mockFetch({ claudeOk = false, groqOk = false, provider = 'anthropic', postOk = true } = {}) {
  const postSpy = vi.fn()
  global.fetch = vi.fn((url, options) => {
    if (url === '/config.json' && (!options || !options.method)) {
      return Promise.resolve({
        ok: true,
        json: async () => ({
          server: { auto_open_browsers: false, debug: false },
          ai: {
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

  it('renders a "?" help button next to the Claude API key field when Claude is the active provider', async () => {
    mockFetch({ provider: 'anthropic' })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    const btn = within(visibleBlock()).getByRole('button', { name: 'Comment obtenir une clé API Anthropic ?' })
    expect(btn).toBeInTheDocument()
    expect(btn).toHaveTextContent('?')
  })

  it('renders a "?" help button next to the Groq API key field when Groq is the active provider', async () => {
    mockFetch({ provider: 'groq' })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    const btn = within(visibleBlock()).getByRole('button', { name: 'Comment obtenir une clé API Groq ?' })
    expect(btn).toBeInTheDocument()
  })

  it('no help modal is rendered before any "?" button is clicked', async () => {
    mockFetch()
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('clicking the "?" button on the Claude card opens the help modal with Anthropic content', async () => {
    mockFetch({ provider: 'anthropic' })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    fireEvent.click(within(visibleBlock()).getByRole('button', { name: 'Comment obtenir une clé API Anthropic ?' }))

    expect(screen.getByRole('heading', { name: 'Obtenir une clé API Claude (Anthropic)' })).toBeInTheDocument()
  })

  it('switching to the Groq card (via the provider selector) then clicking its "?" button opens Groq help content', async () => {
    mockFetch({ provider: 'anthropic' })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    fireEvent.click(providerButtons().groqBtn)
    await waitFor(() => expect(screen.getByText('Clé API Groq')).toBeInTheDocument())

    fireEvent.click(within(visibleBlock()).getByRole('button', { name: 'Comment obtenir une clé API Groq ?' }))
    expect(screen.getByRole('heading', { name: 'Obtenir une clé API Groq' })).toBeInTheDocument()
  })

  it('closing the help modal (×) hides it, regardless of the active provider', async () => {
    mockFetch({ provider: 'groq' })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    fireEvent.click(within(visibleBlock()).getByRole('button', { name: 'Comment obtenir une clé API Groq ?' }))
    expect(screen.getByRole('dialog')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Fermer' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('closing the help modal does not trigger any /config.json POST', async () => {
    const postSpy = mockFetch()
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    fireEvent.click(within(visibleBlock()).getByRole('button', { name: 'Comment obtenir une clé API Anthropic ?' }))
    fireEvent.click(screen.getByRole('button', { name: 'Fermer' }))

    expect(postSpy).not.toHaveBeenCalled()
  })
})

describe('ConfigPage — Sélecteur de fournisseur IA, sélection manuelle uniquement (bugfix/config-api-key-help, commit 1d5de9f)', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('mounts with the Claude card only when the server-saved provider is "anthropic" — Groq fields absent from the DOM', async () => {
    mockFetch({ provider: 'anthropic' })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('Clé API Claude')).toBeInTheDocument())
    expect(screen.queryByText('Clé API Groq')).not.toBeInTheDocument()
  })

  it('mounts with the Groq card only when the server-saved provider is "groq" — Claude fields absent from the DOM', async () => {
    mockFetch({ provider: 'groq' })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('Clé API Groq')).toBeInTheDocument())
    expect(screen.queryByText('Clé API Claude')).not.toBeInTheDocument()
  })

  it('both selector buttons stay clickable (never disabled) even when NEITHER provider has a key', async () => {
    mockFetch({ claudeOk: false, groqOk: false, provider: 'anthropic' })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    const { claudeBtn, groqBtn } = providerButtons()
    expect(claudeBtn).not.toBeDisabled()
    expect(groqBtn).not.toBeDisabled()
    expect(claudeBtn).not.toHaveAttribute('title')
    expect(groqBtn).not.toHaveAttribute('title')
  })

  it('clicking "Groq" switches the visible card to Groq and persists the choice via POST /config.json', async () => {
    const postSpy = mockFetch({ provider: 'anthropic' })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('Clé API Claude')).toBeInTheDocument())

    fireEvent.click(providerButtons().groqBtn)

    await waitFor(() => expect(screen.getByText('Clé API Groq')).toBeInTheDocument())
    expect(screen.queryByText('Clé API Claude')).not.toBeInTheDocument()
    await waitFor(() => {
      expect(postSpy).toHaveBeenCalledWith({ ai: { ...FULL_AI_SETTINGS, provider: 'groq' } })
    })
  })

  it('clicking "Claude (Anthropic)" while Groq is active switches back and persists the choice', async () => {
    const postSpy = mockFetch({ provider: 'groq' })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('Clé API Groq')).toBeInTheDocument())

    fireEvent.click(providerButtons().claudeBtn)

    await waitFor(() => expect(screen.getByText('Clé API Claude')).toBeInTheDocument())
    expect(screen.queryByText('Clé API Groq')).not.toBeInTheDocument()
    await waitFor(() => {
      expect(postSpy).toHaveBeenCalledWith({ ai: { ...FULL_AI_SETTINGS, provider: 'anthropic' } })
    })
  })

  it('the active provider button carries the "active" class, the other does not', async () => {
    mockFetch({ provider: 'groq' })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('Clé API Groq')).toBeInTheDocument())

    const { claudeBtn, groqBtn } = providerButtons()
    expect(groqBtn).toHaveClass('active')
    expect(claudeBtn).not.toHaveClass('active')
  })

  it('mounting never fires a silent /config.json POST, whatever the key/provider combination', async () => {
    for (const params of [
      { claudeOk: false, groqOk: false, provider: 'anthropic' },
      { claudeOk: true, groqOk: false, provider: 'anthropic' },
      { claudeOk: false, groqOk: true, provider: 'anthropic' },
      { claudeOk: true, groqOk: true, provider: 'groq' },
    ]) {
      const postSpy = mockFetch(params)
      const { unmount } = render(<ConfigPage />)
      await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())
      expect(postSpy).not.toHaveBeenCalled()
      unmount()
    }
  })

  it('saving the Claude key does NOT switch the active provider or the visible card', async () => {
    const postSpy = mockFetch({ claudeOk: false, groqOk: true, provider: 'anthropic' })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('Clé API Claude')).toBeInTheDocument())

    const section = visibleBlock()
    fireEvent.change(within(section).getByPlaceholderText('sk-ant-...'), { target: { value: 'sk-ant-newkey' } })
    fireEvent.click(within(section).getByText('Enregistrer'))

    await waitFor(() => expect(within(visibleBlock()).getByText('✅ Clé configurée')).toBeInTheDocument())
    // Still on the Claude card — no switch to Groq even though Groq has a key.
    expect(screen.getByText('Clé API Claude')).toBeInTheDocument()
    expect(providerButtons().claudeBtn).toHaveClass('active')
    // Only the key-save POST fires — no extra provider-switch POST.
    expect(postSpy).toHaveBeenCalledTimes(1)
    expect(postSpy).toHaveBeenCalledWith({ ai: { ...FULL_AI_SETTINGS, anthropic_api_key: 'sk-ant-newkey' } })
  })

  it('clearing the only configured key does NOT switch the active provider or the visible card', async () => {
    const postSpy = mockFetch({ claudeOk: true, groqOk: false, provider: 'anthropic' })
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<ConfigPage />)
    await waitFor(() => expect(within(visibleBlock()).getByText('✅ Clé configurée')).toBeInTheDocument())

    fireEvent.click(within(visibleBlock()).getByText('Supprimer la clé'))

    await waitFor(() => expect(within(visibleBlock()).getByText('⚠️ Aucune clé')).toBeInTheDocument())
    // Still on the Claude card, still the active/highlighted selector button —
    // no auto-switch even though no provider has a key anymore.
    expect(screen.getByText('Clé API Claude')).toBeInTheDocument()
    expect(providerButtons().claudeBtn).toHaveClass('active')
    expect(providerButtons().claudeBtn).not.toBeDisabled()
    expect(postSpy).toHaveBeenCalledTimes(1)
    expect(postSpy).toHaveBeenCalledWith({ ai: { ...FULL_AI_SETTINGS, clear_api_key: true } })
  })

  it('saving the Groq key does NOT switch the active provider away from Groq even with a Claude key present', async () => {
    const postSpy = mockFetch({ claudeOk: true, groqOk: false, provider: 'groq' })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('Clé API Groq')).toBeInTheDocument())

    const section = visibleBlock()
    fireEvent.change(within(section).getByPlaceholderText('gsk_...'), { target: { value: 'gsk_newkey' } })
    fireEvent.click(within(section).getByText('Enregistrer'))

    await waitFor(() => expect(within(visibleBlock()).getByText('✅ Clé configurée')).toBeInTheDocument())
    expect(screen.getByText('Clé API Groq')).toBeInTheDocument()
    expect(providerButtons().groqBtn).toHaveClass('active')
    expect(postSpy).toHaveBeenCalledTimes(1)
  })

  it('a failed provider switch (POST rejected) rolls back to the previous card', async () => {
    const postSpy = mockFetch({ provider: 'anthropic', postOk: false })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('Clé API Claude')).toBeInTheDocument())

    fireEvent.click(providerButtons().groqBtn)

    // Optimistic switch happens first...
    await waitFor(() => expect(postSpy).toHaveBeenCalled())
    // ...then rolls back once the POST resolves with ok:false.
    await waitFor(() => expect(screen.getByText('Clé API Claude')).toBeInTheDocument())
    expect(screen.queryByText('Clé API Groq')).not.toBeInTheDocument()
    expect(providerButtons().claudeBtn).toHaveClass('active')
  })
})
