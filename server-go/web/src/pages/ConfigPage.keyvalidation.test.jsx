import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import ConfigPage from './ConfigPage'

// Tests dérivés des 11 scénarios de la maquette (validation de clé API par appel
// réel au fournisseur, contracts/ai-key-validation.md, tâche #13 du plan
// _work/reports/plan-20260809-104602.md, /api/ai/validate-key mocké).
//
// ⚠️ Comme ConfigPage.test.jsx/.ai.test.jsx/.apikeyhelp.test.jsx, ce fichier n'a
// pas pu être exécuté avec succès dans l'environnement WSL de développement
// (render() de ConfigPage bloque indéfiniment, y compris avec un `fetch`
// intégralement mocké — cf. handoffs dev-frontend précédents). Écrit et
// syntaxiquement validé (esbuild) mais NON vérifié à l'exécution dans cette
// session ; à faire tourner sur un environnement Windows natif (QA).

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

// La seule `.ai-provider-block` montée (Claude XOR Groq, jamais les deux —
// simplification #7, commit 1d5de9f).
function visibleBlock() {
  return screen.getByText(/^Clé API (Claude|Groq)$/).closest('.ai-provider-block')
}

// `validateResponses` : un verdict, ou une file de verdicts consommés dans
// l'ordre à chaque appel successif à /api/ai/validate-key (scénario "Réessayer
// qui réussit" : 1er appel unreachable, 2e appel valid). `validateStatus`
// permet de simuler une erreur de NOTRE serveur (400 préfixe invalide, 429
// cooldown) plutôt qu'un verdict fournisseur (§5).
function mockFetch({
  claudeOk = false,
  groqOk = false,
  claudeVerified = false,
  groqVerified = false,
  provider = 'anthropic',
  validateResponses = [{ result: 'valid', http_status: 200 }],
  validateStatus = 200,
} = {}) {
  const postSpy = vi.fn()
  const validateSpy = vi.fn()
  let validateCallIndex = 0
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
            anthropic_api_key_verified: claudeVerified,
            groq_api_key_verified: groqVerified,
            provider,
          },
        }),
      })
    }
    if (url === '/config.json' && options?.method === 'POST') {
      postSpy(JSON.parse(options.body))
      return Promise.resolve({ ok: true, json: async () => ({}) })
    }
    if (url === '/api/ai/validate-key') {
      const body = JSON.parse(options.body)
      validateSpy(body)
      if (validateStatus !== 200) {
        return Promise.resolve({ ok: false, status: validateStatus, text: async () => 'Format de clé API invalide' })
      }
      const verdict = validateResponses[Math.min(validateCallIndex, validateResponses.length - 1)]
      validateCallIndex += 1
      return Promise.resolve({ ok: true, json: async () => ({ provider: body.provider, ...verdict }) })
    }
    if (url === '/api/wifi/defaults') {
      return Promise.resolve({ ok: true, json: async () => ({ ssid: '', password: '', server_ip: '', server_port: 80 }) })
    }
    if (url === '/api/firmware/buzzclick/version') {
      return Promise.resolve({ ok: true, json: async () => ({ EXISTS: false }) })
    }
    return Promise.resolve({ ok: false, json: async () => ({}), text: async () => '' })
  })
  return { postSpy, validateSpy }
}

async function fillAndSaveClaudeKey(value) {
  const section = visibleBlock()
  fireEvent.change(within(section).getByPlaceholderText('sk-ant-...'), { target: { value } })
  fireEvent.click(within(section).getByText('Enregistrer'))
}

describe('ConfigPage — Validation de clé API (contracts/ai-key-validation.md, tâche #13)', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('1. clé valide, fournisseur joignable -> badge "Clé vérifiée", aucun dialogue, clé persistée avec verified:true', async () => {
    const { postSpy, validateSpy } = mockFetch({ provider: 'anthropic', validateResponses: [{ result: 'valid', http_status: 200 }] })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('Clé API Claude')).toBeInTheDocument())

    await fillAndSaveClaudeKey('sk-ant-goodkey')

    await waitFor(() => expect(within(visibleBlock()).getByText('✅ Clé vérifiée')).toBeInTheDocument())
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(validateSpy).toHaveBeenCalledWith({ provider: 'anthropic', api_key: 'sk-ant-goodkey' })
    expect(postSpy).toHaveBeenCalledWith({ ai: { ...FULL_AI_SETTINGS, anthropic_api_key: 'sk-ant-goodkey', anthropic_api_key_verified: true } })
  })

  it('2. clé refusée (invalid_key), "Corriger la clé" -> dialogue refus puis champ en erreur, RIEN n\'est écrit', async () => {
    const { postSpy } = mockFetch({
      provider: 'anthropic',
      validateResponses: [{ result: 'invalid_key', http_status: 401, detail: 'invalid x-api-key' }],
    })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('Clé API Claude')).toBeInTheDocument())

    await fillAndSaveClaudeKey('sk-ant-badkey')

    const dialog = await screen.findByRole('alertdialog')
    expect(within(dialog).getByText('Claude a refusé cette clé')).toBeInTheDocument()
    expect(within(dialog).getByText('401 · invalid x-api-key')).toBeInTheDocument()
    // invalid_key n'offre jamais "Réessayer" (contrat §9 — distinct d'unreachable).
    expect(within(dialog).queryByText('Réessayer')).not.toBeInTheDocument()

    fireEvent.click(within(dialog).getByText('Corriger la clé'))

    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(screen.getByText("Clé refusée par Claude. Rien n'a été enregistré.")).toBeInTheDocument()
    expect(within(visibleBlock()).getByPlaceholderText('sk-ant-...')).toHaveClass('invalid')
    expect(postSpy).not.toHaveBeenCalled()
  })

  it('3. clé refusée, "Enregistrer quand même" -> badge "Clé non vérifiée", clé persistée avec verified:false', async () => {
    const { postSpy } = mockFetch({
      provider: 'anthropic',
      validateResponses: [{ result: 'invalid_key', http_status: 403, detail: 'permission denied' }],
    })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('Clé API Claude')).toBeInTheDocument())

    await fillAndSaveClaudeKey('sk-ant-badkey')
    const dialog = await screen.findByRole('alertdialog')
    fireEvent.click(within(dialog).getByText('Enregistrer quand même'))

    await waitFor(() => expect(within(visibleBlock()).getByText('⚠️ Clé non vérifiée')).toBeInTheDocument())
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(postSpy).toHaveBeenCalledWith({ ai: { ...FULL_AI_SETTINGS, anthropic_api_key: 'sk-ant-badkey', anthropic_api_key_verified: false } })
  })

  it('4. fournisseur injoignable, "Réessayer" qui réussit -> dialogue se ferme, badge "Clé vérifiée", clé persistée verified:true', async () => {
    const { postSpy, validateSpy } = mockFetch({
      provider: 'anthropic',
      validateResponses: [
        { result: 'unreachable', http_status: 0 },
        { result: 'valid', http_status: 200 },
      ],
    })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('Clé API Claude')).toBeInTheDocument())

    await fillAndSaveClaudeKey('sk-ant-flakykey')
    const dialog = await screen.findByRole('alertdialog')
    expect(within(dialog).getByText('Impossible de joindre Claude')).toBeInTheDocument()
    // unreachable, contrairement à invalid_key, offre "Réessayer".
    expect(within(dialog).getByText('Réessayer')).toBeInTheDocument()

    fireEvent.click(within(dialog).getByText('Réessayer'))

    await waitFor(() => expect(within(visibleBlock()).getByText('✅ Clé vérifiée')).toBeInTheDocument())
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(validateSpy).toHaveBeenCalledTimes(2)
    expect(postSpy).toHaveBeenCalledWith({ ai: { ...FULL_AI_SETTINGS, anthropic_api_key: 'sk-ant-flakykey', anthropic_api_key_verified: true } })
  })

  it('5. le fournisseur répond 429 (upstream) -> dialogue "injoignable", jamais "refusée" (contrat §3)', async () => {
    mockFetch({
      provider: 'anthropic',
      validateResponses: [{ result: 'unreachable', http_status: 429 }],
    })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('Clé API Claude')).toBeInTheDocument())

    await fillAndSaveClaudeKey('sk-ant-somekey')

    const dialog = await screen.findByRole('alertdialog')
    expect(within(dialog).getByText('Impossible de joindre Claude')).toBeInTheDocument()
    expect(dialog.className).toContain('injoignable')
    expect(dialog.className).not.toContain('refus')
  })

  it('6. champ vide, clé déjà enregistrée -> valide la clé stockée (aucune clé envoyée), badge mis à jour', async () => {
    const { postSpy, validateSpy } = mockFetch({
      provider: 'anthropic',
      claudeOk: true,
      claudeVerified: false,
      validateResponses: [{ result: 'valid', http_status: 200 }],
    })
    render(<ConfigPage />)
    await waitFor(() => expect(within(visibleBlock()).getByText('⚠️ Clé non vérifiée')).toBeInTheDocument())

    // Champ laissé vide -> valide la clé EFFECTIVE déjà stockée côté serveur
    // (§9 D3 — c'est aussi le seul chemin de vérification d'une clé issue
    // d'une variable d'environnement, qui ne transite jamais par ce champ).
    fireEvent.click(within(visibleBlock()).getByText('Enregistrer'))

    await waitFor(() => expect(within(visibleBlock()).getByText('✅ Clé vérifiée')).toBeInTheDocument())
    expect(validateSpy).toHaveBeenCalledWith({ provider: 'anthropic', api_key: '' })
    // Aucun champ `anthropic_api_key` dans le payload : la clé stockée est préservée.
    expect(postSpy).toHaveBeenCalledWith({ ai: { ...FULL_AI_SETTINGS, anthropic_api_key_verified: true } })
  })

  it('8. champ vide, aucune clé nulle part -> aucune validation, comportement actuel (POST direct)', async () => {
    const { postSpy, validateSpy } = mockFetch({ provider: 'anthropic', claudeOk: false })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('Clé API Claude')).toBeInTheDocument())

    fireEvent.click(within(visibleBlock()).getByText('Enregistrer'))

    await waitFor(() => expect(postSpy).toHaveBeenCalledWith({ ai: FULL_AI_SETTINGS }))
    expect(validateSpy).not.toHaveBeenCalled()
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('9. "Supprimer la clé" ne déclenche aucune validation', async () => {
    const { validateSpy } = mockFetch({ provider: 'anthropic', claudeOk: true, claudeVerified: true })
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<ConfigPage />)
    await waitFor(() => expect(within(visibleBlock()).getByText('✅ Clé vérifiée')).toBeInTheDocument())

    fireEvent.click(within(visibleBlock()).getByText('Supprimer la clé'))

    await waitFor(() => expect(within(visibleBlock()).getByText('⚠️ Aucune clé')).toBeInTheDocument())
    expect(validateSpy).not.toHaveBeenCalled()
  })

  it('10. préfixe invalide (erreur de NOTRE serveur, pas un verdict fournisseur) -> toast d\'erreur, rien écrit, pas de dialogue', async () => {
    const { postSpy } = mockFetch({ provider: 'anthropic', validateStatus: 400 })
    render(<ConfigPage />)
    await waitFor(() => expect(screen.getByText('Clé API Claude')).toBeInTheDocument())

    await fillAndSaveClaudeKey('not-a-real-key')

    await waitFor(() => expect(screen.getByText('Erreur: Format de clé API invalide')).toBeInTheDocument())
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(postSpy).not.toHaveBeenCalled()
  })

  it('11. badge "non vérifiée" persiste après un remontage (rechargement) suivant un enregistrement forcé', async () => {
    // Simule le rechargement de page après un "Enregistrer quand même" d'une
    // session précédente : le GET renvoie directement verified:false, pas de
    // dérivation locale (contrat §7 D2).
    mockFetch({ provider: 'anthropic', claudeOk: true, claudeVerified: false })
    render(<ConfigPage />)

    await waitFor(() => expect(within(visibleBlock()).getByText('⚠️ Clé non vérifiée')).toBeInTheDocument())
  })
})
