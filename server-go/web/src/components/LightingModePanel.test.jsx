import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import LightingModePanel from './LightingModePanel'

// ---------------------------------------------------------------------------
// LightingModePanel — #208 (milestone v10.0.0). Composant AUTONOME (aucune
// prop requise, gère son propre useLightingStatus()) — d'abord monté sur
// AmbiancePage.jsx (Batch 3), puis extrait ici et déplacé vers GamePage.jsx
// (correction utilisateur du 2026-09-07, voir en-tête du .jsx). Ces tests
// remplacent le describe "#208 conduite en direct" qui vivait dans
// AmbiancePage.batch3.test.jsx.
//
// Contrat : contracts/lighting.md §10.1 (SHA df448318) — le mode TIENT
// indéfiniment (pas d'écrasement automatique), seul un retour manuel sur
// AUTO relâche. État lu depuis GET /api/lighting/status, jamais déduit côté
// client.
// ---------------------------------------------------------------------------

vi.mock('./LightingModePanel.css', () => ({}))

function makeServer({ mode = 'AUTO', flash = false, state = 'ok' } = {}) {
  const server = { mode, flash, state, calls: [] }
  const respond = (status, body = {}) => ({
    ok: status < 400,
    status,
    json: async () => body,
    text: async () => JSON.stringify(body),
  })

  global.fetch = vi.fn(async (url, opts = {}) => {
    const method = opts.method || 'GET'
    const body = opts.body ? JSON.parse(opts.body) : null
    server.calls.push({ method, url, body })

    if (method === 'GET' && url === '/api/lighting/status') {
      return respond(200, { state: server.state, mode: server.mode, flash: server.flash })
    }
    if (method === 'POST' && url === '/api/lighting/mode') {
      if (!['ON', 'AUTO', 'OFF'].includes(body?.mode)) return respond(400, { result: 'error' })
      server.mode = body.mode
      return respond(200, { result: 'ok', mode: server.mode })
    }
    if (method === 'POST' && url === '/api/lighting/flash') {
      server.flash = !!body?.on
      return respond(200, { result: 'ok', flash: server.flash })
    }
    throw new Error(`Route non mockée : ${method} ${url}`)
  })

  return server
}

const callsTo = (server, method, url) => server.calls.filter(c => c.method === method && c.url === url)

afterEach(() => {
  vi.restoreAllMocks()
})

describe('LightingModePanel', () => {
  it('reste invisible tant que l\'éclairage n\'est pas configuré (state disabled)', async () => {
    makeServer({ state: 'disabled' })
    render(<LightingModePanel />)
    await waitFor(() => expect(global.fetch).toHaveBeenCalled())
    expect(screen.queryByRole('radiogroup')).toBeNull()
  })

  it('affiche la position AUTO par défaut, sans bandeau d\'avertissement', async () => {
    makeServer({ mode: 'AUTO' })
    render(<LightingModePanel />)

    const auto = await screen.findByRole('radio', { name: 'AUTO' })
    expect(auto).toHaveAttribute('aria-checked', 'true')
    expect(auto).toBeDisabled() // reclic sur la position déjà active : sans effet
    expect(screen.getByRole('radio', { name: 'ON' })).not.toBeDisabled()
    expect(screen.getByRole('radio', { name: 'OFF' })).not.toBeDisabled()
    expect(screen.queryByText(/engagé/)).toBeNull()
  })

  it('cliquer OFF appelle POST /api/lighting/mode {mode:"OFF"} et met à jour le sélecteur affiché', async () => {
    const server = makeServer({ mode: 'AUTO' })
    render(<LightingModePanel />)
    await screen.findByRole('radio', { name: 'AUTO' })

    fireEvent.click(screen.getByRole('radio', { name: 'OFF' }))

    await waitFor(() => expect(screen.getByRole('radio', { name: 'OFF' })).toHaveAttribute('aria-checked', 'true'))
    const calls = callsTo(server, 'POST', '/api/lighting/mode')
    expect(calls).toHaveLength(1)
    expect(calls[0].body).toEqual({ mode: 'OFF' })
    // La position affichée vient du GET /api/lighting/status relu après le
    // POST — pas d'état optimiste côté client (contrat §10.1.1 pt.6).
    expect(callsTo(server, 'GET', '/api/lighting/status').length).toBeGreaterThanOrEqual(2)
  })

  it('le mode ON/OFF affiche un bandeau d\'avertissement permanent (garde-fou contre l\'oubli)', async () => {
    makeServer({ mode: 'OFF' })
    render(<LightingModePanel />)
    await screen.findByRole('radio', { name: 'AUTO' })

    expect(screen.getByRole('radio', { name: 'OFF' })).toHaveAttribute('aria-checked', 'true')
    // Le texte est scindé entre plusieurs nœuds (le mode est dans un
    // <strong>) : comparer le textContent complet plutôt qu'un getByText.
    const warning = document.querySelector('.lighting-mode-warning')
    expect(warning).not.toBeNull()
    expect(warning.textContent).toMatch(/Mode\s*OFF\s*engagé/)
    expect(warning.textContent).toMatch(/restera\s*éteint indéfiniment/)
  })

  it('cliquer la position déjà active n\'émet aucune requête', async () => {
    const server = makeServer({ mode: 'ON' })
    render(<LightingModePanel />)
    await screen.findByRole('radio', { name: 'AUTO' })

    fireEvent.click(screen.getByRole('radio', { name: 'ON' }))
    expect(callsTo(server, 'POST', '/api/lighting/mode')).toHaveLength(0)
  })

  it('Flash est une bascule séparée : l\'activer puis le désactiver n\'affecte jamais le sélecteur', async () => {
    const server = makeServer({ mode: 'OFF', flash: false })
    render(<LightingModePanel />)
    await screen.findByRole('radio', { name: 'AUTO' })

    const flashBtn = screen.getByRole('button', { name: /Flash/ })
    expect(flashBtn).toHaveAttribute('aria-pressed', 'false')

    fireEvent.click(flashBtn)
    await waitFor(() => expect(screen.getByRole('button', { name: /Flash/ })).toHaveAttribute('aria-pressed', 'true'))
    expect(callsTo(server, 'POST', '/api/lighting/flash')[0].body).toEqual({ on: true })
    // Le sélecteur reste sur OFF — Flash ne le déplace jamais (contrat §10.1.2).
    expect(screen.getByRole('radio', { name: 'OFF' })).toHaveAttribute('aria-checked', 'true')

    fireEvent.click(screen.getByRole('button', { name: /Flash/ }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Flash/ })).toHaveAttribute('aria-pressed', 'false'))
    expect(callsTo(server, 'POST', '/api/lighting/flash')[1].body).toEqual({ on: false })
  })

  it('une erreur serveur sur /api/lighting/mode affiche un toast, sans faire bouger le sélecteur', async () => {
    makeServer({ mode: 'AUTO' })
    global.fetch = vi.fn(async (url, opts = {}) => {
      if (url === '/api/lighting/mode') return { ok: false, status: 500, text: async () => 'boom' }
      if (url === '/api/lighting/status') return { ok: true, status: 200, json: async () => ({ state: 'ok', mode: 'AUTO', flash: false }) }
      throw new Error(`Route non mockée : ${url}`)
    })
    render(<LightingModePanel />)
    await screen.findByRole('radio', { name: 'AUTO' })

    fireEvent.click(screen.getByRole('radio', { name: 'OFF' }))
    await screen.findByText(/Erreur/)
    expect(screen.getByRole('radio', { name: 'AUTO' })).toHaveAttribute('aria-checked', 'true')
  })
})
