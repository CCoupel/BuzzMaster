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
//
// MISE À JOUR (bugfix/config-api-key-help, sur demande explicite du CDP) :
// ce fichier datait de #8 (fournisseur Claude unique) et n'avait jamais pu
// tourner jusqu'ici (blocage vitest/WSL, cf. `_work/reports/test-writer-*.md`),
// ce qui a laissé passer sa désynchronisation avec le libellé UI introduit par
// #137 (fournisseurs multiples Claude/Groq, contracts/CHANGELOG.md
// [20260805b]/[20260806] — `provider`/`groq_api_key_configured` ajoutés,
// badges/labels raccourcis) : les textes attendus ("Clé API Anthropic
// (Claude)", "⚠️ Aucune clé configurée") ne correspondent plus au JSX actuel
// ("Clé API Claude", "⚠️ Aucune clé"), et `getAiSection()` scopait sur
// `.config-section` — qui contient désormais AUSSI le bloc Groq introduit par
// #137, donc DEUX boutons "Enregistrer"/"Supprimer la clé" (un par
// fournisseur) sous ce même scope, faisant échouer `getByText` (élément
// ambigu) dès que ces tests s'exécuteraient. Corrigé ci-dessous : libellés
// alignés sur le JSX courant, scope resserré sur `.ai-provider-block` (bloc
// Claude uniquement).
//
// MISE À JOUR 2 (tâche #13, contracts/ai-key-validation.md, sur demande
// explicite du CDP — signalé par dev-frontend dans
// `_work/handoff/dev-frontend-20260809-115500.md`, non traité par eux pour
// éviter un conflit d'édition concurrente) : le badge passe de binaire
// (`✅ Clé configurée` / `⚠️ Aucune clé`) à **tri-état**
// (`✅ Clé vérifiée` / `⚠️ Clé non vérifiée` / `⚠️ Aucune clé`), lu depuis
// `api_key_configured` ET le nouveau `anthropic_api_key_verified`. Plus
// profond qu'un simple changement de libellé : `handleSaveAiKey` route
// désormais TOUJOURS par `POST /api/ai/validate-key` avant d'écrire
// `/config.json` dès qu'un champ est rempli OU qu'une clé est déjà stockée
// (contrat §9) — les tests de sauvegarde ci-dessous, qui ne mockaient que
// `/config.json`, échoueraient silencieusement contre le code actuel
// (`validateAndProceed` prend la branche d'erreur générique faute de mock
// pour `/api/ai/validate-key`, et n'atteint jamais le POST `/config.json`).
// `mockFetchImplementation` gagne donc un verdict de validation par défaut
// (`valid`), et les payloads persistés incluent désormais
// `anthropic_api_key_verified`. `handleClearAiKey` reste hors de ce chemin
// (inchangé, tâche 7 du plan) — ses tests n'ont besoin que du correctif de
// libellé. Suit le même correctif déjà appliqué par dev-frontend à 3 tests
// similaires dans `ConfigPage.apikeyhelp.test.jsx` (même commit).

// #136 — `useGame` en `vi.fn()` + `mockReturnValue(obj)` (identité stable
// entre rendus), pas un littéral fléché qui recrée `firmwareInfo` à chaque
// appel (cause de la boucle de rendu synchrone sous RTL, cf.
// ConfigPage.jsx:262-266 corrigé dans le même commit que le durcissement des
// 4 mocks de ce milestone).
vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

import { useGame } from '../hooks/GameContext'

const makeConfigPageMock = (overrides = {}) => ({
  teams: {},
  bumpers: {},
  gameState: { backgrounds: [] },
  updateConfig: vi.fn(),
  sendMessage: vi.fn(),
  version: '6.0.0',
  firmwareInfo: { EXISTS: true, IS_MERGED: true, VERSION: '3.1.1', FILENAME: 'buzzclick-v3.1.1.bin', SIZE: 512000 },
  ...overrides,
})

beforeEach(() => {
  useGame.mockReturnValue(makeConfigPageMock())
})

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

// Locates the Claude-specific "ai-provider-block" (NOT the wider IA
// ".config-section", which also holds the Groq block since #137 — scoping
// there would make "Enregistrer"/"Supprimer la clé" ambiguous, one pair per
// provider), so every query below it targets the Claude fields only.
function getAiSection() {
  const label = screen.getByText('Clé API Claude')
  return label.closest('.ai-provider-block')
}

// `apiKeyVerified` (tâche #13) alimente le nouveau `anthropic_api_key_verified`
// du GET de montage. `validateOk`/`validateResult` pilotent la réponse de
// `POST /api/ai/validate-key`, désormais appelé par `handleSaveAiKey` avant
// tout `POST /config.json` (contrat §9) — `validateOk: false` simule un rejet
// de NOTRE serveur (préfixe invalide, 400, texte brut), `validateResult`
// simule le verdict fournisseur normal (`{result: 'valid', ...}` par défaut,
// pour que les tests antérieurs à #13 continuent d'exercer le chemin direct
// sans avoir à connaître le dialogue de validation — cf. `postOk` qui garde
// son rôle inchangé pour le POST /config.json qui suit).
function mockFetchImplementation({
  apiKeyConfigured = false,
  apiKeyVerified = false,
  postOk = true,
  postBody = null,
  validateOk = true,
  validateResult = { result: 'valid', http_status: 200 },
} = {}) {
  global.fetch = vi.fn((url, options) => {
    if (url === '/config.json' && (!options || options.method === undefined)) {
      return Promise.resolve({
        ok: true,
        json: async () => ({
          server: { auto_open_browsers: false, debug: false },
          ai: { api_key_configured: apiKeyConfigured, anthropic_api_key_verified: apiKeyVerified },
        }),
      })
    }
    if (url === '/api/ai/validate-key') {
      if (!validateOk) {
        return Promise.resolve({ ok: false, text: async () => 'Format de clé API invalide' })
      }
      const body = options?.body ? JSON.parse(options.body) : {}
      return Promise.resolve({
        ok: true,
        json: async () => ({ provider: body.provider, ...validateResult }),
      })
    }
    if (url === '/config.json' && options?.method === 'POST') {
      if (postOk) {
        return Promise.resolve({ ok: true, json: async () => (postBody || {}) })
      }
      return Promise.resolve({ ok: false, text: async () => 'Erreur serveur' })
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

  it('renders the "IA" section title and the Claude API key field', async () => {
    mockFetchImplementation({ apiKeyConfigured: false })
    render(<ConfigPage />)

    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())
    expect(screen.getByText('Clé API Claude')).toBeInTheDocument()
  })

  it('shows "Aucune clé" badge when ai.api_key_configured is false', async () => {
    mockFetchImplementation({ apiKeyConfigured: false })
    render(<ConfigPage />)

    await waitFor(() => {
      expect(within(getAiSection()).getByText('⚠️ Aucune clé')).toBeInTheDocument()
    })
    expect(screen.queryByText('✅ Clé vérifiée')).not.toBeInTheDocument()
    expect(screen.queryByText('⚠️ Clé non vérifiée')).not.toBeInTheDocument()
  })

  // Tri-état depuis la tâche #13 (contracts/ai-key-validation.md §7) — les 2
  // tests suivants remplacent l'ancien test binaire unique.
  it('shows "✅ Clé vérifiée" badge when configured and verified', async () => {
    mockFetchImplementation({ apiKeyConfigured: true, apiKeyVerified: true })
    render(<ConfigPage />)

    await waitFor(() => {
      expect(within(getAiSection()).getByText('✅ Clé vérifiée')).toBeInTheDocument()
    })
  })

  it('shows "⚠️ Clé non vérifiée" badge when configured but not verified', async () => {
    mockFetchImplementation({ apiKeyConfigured: true, apiKeyVerified: false })
    render(<ConfigPage />)

    await waitFor(() => {
      expect(within(getAiSection()).getByText('⚠️ Clé non vérifiée')).toBeInTheDocument()
    })
  })

  it('never receives the actual API key from the server — only api_key_configured (contract §2, CA3)', async () => {
    mockFetchImplementation({ apiKeyConfigured: true, apiKeyVerified: true })
    render(<ConfigPage />)

    await waitFor(() => expect(within(getAiSection()).getByText('✅ Clé vérifiée')).toBeInTheDocument())

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
    await waitFor(() => expect(within(getAiSection()).getByText('⚠️ Aucune clé')).toBeInTheDocument())

    const section = getAiSection()
    expect(within(section).queryByText('Supprimer la clé')).not.toBeInTheDocument()
  })

  it('renders "Supprimer la clé" when a key is configured', async () => {
    mockFetchImplementation({ apiKeyConfigured: true, apiKeyVerified: true })
    render(<ConfigPage />)
    await waitFor(() => expect(within(getAiSection()).getByText('✅ Clé vérifiée')).toBeInTheDocument())

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

  it('saving a non-empty key validates it (POST /api/ai/validate-key), then POSTs the full ai section including anthropic_api_key + anthropic_api_key_verified (contract §0; ai-key-validation.md §9)', async () => {
    mockFetchImplementation({ apiKeyConfigured: false })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    const section = getAiSection()
    const input = within(section).getByPlaceholderText('sk-ant-...')
    fireEvent.change(input, { target: { value: 'sk-ant-newkey123' } })
    fireEvent.click(within(section).getByText('Enregistrer'))

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        '/api/ai/validate-key',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ provider: 'anthropic', api_key: 'sk-ant-newkey123' }),
        })
      )
    })
    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        '/config.json',
        expect.objectContaining({
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ ai: { ...FULL_AI_SETTINGS, anthropic_api_key: 'sk-ant-newkey123', anthropic_api_key_verified: true } }),
        })
      )
    })
  })

  it('saving with an empty field validates the effective stored key, then POSTs the full ai section WITHOUT anthropic_api_key — preserves the stored key server-side (contract §0/§2, CA2; ai-key-validation.md §9 D3)', async () => {
    mockFetchImplementation({ apiKeyConfigured: true, apiKeyVerified: true })
    render(<ConfigPage />)
    await waitFor(() => expect(within(getAiSection()).getByText('✅ Clé vérifiée')).toBeInTheDocument())

    const section = getAiSection()
    fireEvent.click(within(section).getByText('Enregistrer'))

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        '/config.json',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ ai: { ...FULL_AI_SETTINGS, anthropic_api_key_verified: true } }),
        })
      )
    })
  })

  it('clears the input and flips the badge to verified after a successful non-empty save', async () => {
    mockFetchImplementation({ apiKeyConfigured: false })
    render(<ConfigPage />)
    await waitFor(() => expect(within(getAiSection()).getByText('⚠️ Aucune clé')).toBeInTheDocument())

    const section = getAiSection()
    const input = within(section).getByPlaceholderText('sk-ant-...')
    fireEvent.change(input, { target: { value: 'sk-ant-newkey123' } })
    fireEvent.click(within(section).getByText('Enregistrer'))

    await waitFor(() => {
      expect(within(getAiSection()).getByText('✅ Clé vérifiée')).toBeInTheDocument()
    })
    const refreshedSection = getAiSection()
    const refreshedInput = within(refreshedSection).getByPlaceholderText('••••••••')
    expect(refreshedInput.value).toBe('')
  })

  // Message mis à jour par la tâche #13 (ai-key-validation.md §9) — un
  // verdict "valid" nomme désormais le fournisseur, remplace l'ancien
  // "Clé API enregistrée" générique de #8.
  it('shows a success toast naming the provider after a validated save', async () => {
    mockFetchImplementation({ apiKeyConfigured: false })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('IA')).toBeInTheDocument())

    const section = getAiSection()
    fireEvent.change(within(section).getByPlaceholderText('sk-ant-...'), { target: { value: 'sk-ant-x' } })
    fireEvent.click(within(section).getByText('Enregistrer'))

    await waitFor(() => {
      expect(screen.getByText('Clé vérifiée auprès de Claude et enregistrée.')).toBeInTheDocument()
    })
  })

  // #13 déplace ce rejet de format de POST /config.json vers
  // POST /api/ai/validate-key (préfixe vérifié côté serveur AVANT tout appel
  // fournisseur, contrat ai-key-validation.md §5) — le texte d'erreur change
  // en conséquence (plus de suffixe "(attendu : sk-ant-...)").
  it('shows an error toast when the key-validation endpoint rejects the format (400)', async () => {
    mockFetchImplementation({ apiKeyConfigured: false, validateOk: false })
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
    mockFetchImplementation({ apiKeyConfigured: true, apiKeyVerified: true })
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<ConfigPage />)
    await waitFor(() => expect(within(getAiSection()).getByText('✅ Clé vérifiée')).toBeInTheDocument())

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
      expect(within(getAiSection()).getByText('⚠️ Aucune clé')).toBeInTheDocument()
    })
    confirmSpy.mockRestore()
  })

  it('does nothing when "Supprimer la clé" confirmation is declined', async () => {
    mockFetchImplementation({ apiKeyConfigured: true, apiKeyVerified: true })
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false)
    render(<ConfigPage />)
    await waitFor(() => expect(within(getAiSection()).getByText('✅ Clé vérifiée')).toBeInTheDocument())

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
