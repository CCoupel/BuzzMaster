import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

// ---------------------------------------------------------------------------
// AmbiancePage — P2b : allumage/extinction immédiat au coché/décoché.
// Conception : planner-v10-general-theme-toggle-20260908-114420.md §Partie 2.
//
// POST /api/lighting/preview {name, on} — écrit immédiatement l'ampoule
// nommée (blanc plein si on=true, éteinte si on=false), même taxonomie
// d'erreurs que /test (ok/busy/unreachable/refused), même garde
// `lightingBusy` (partagée avec /test — d'où la désactivation croisée
// vérifiée ci-dessous).
//
// §2.3, points normatifs testés :
//   - la case bascule TOUJOURS, même si le pont est injoignable — la
//     sélection ne dépend jamais du résultat réseau ;
//   - une ampoule déjà dans la config ENREGISTRÉE, décochée mais pas
//     encore sauvegardée, affiche une mention discrète (elle reste
//     pilotée jusqu'à « Enregistrer », donc rallumée à la prochaine scène).
// ---------------------------------------------------------------------------

vi.mock('./AmbiancePage.css', () => ({}))
vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

import AmbiancePage from './AmbiancePage'
import { useGame } from '../hooks/GameContext'

const BRIDGE = { ip: '192.168.1.101', id: '001788fffea0591e' }
const CONFIGURED = { enabled: true, bridge_ip: BRIDGE.ip, bridge_id: BRIDGE.id, api_key_configured: true, lights: [] }
const CONFIGURED_WITH_LIGHT = {
  ...CONFIGURED,
  lights: [{ name: 'Salle gauche', role: 'general' }],
}
const INVENTORY = {
  status: 200,
  body: {
    lights: [
      { id: '8', name: 'Salle gauche', reachable: true },
      { id: '9', name: 'Salle droite', reachable: true },
    ],
  },
}

function makeServer({ lighting, lights = INVENTORY, preview } = {}) {
  const server = { lighting: { ...lighting }, calls: [] }
  const respond = (status, body = {}) => ({
    ok: status < 400, status, json: async () => body, text: async () => JSON.stringify(body),
  })

  global.fetch = vi.fn(async (url, opts = {}) => {
    const method = opts.method || 'GET'
    const body = opts.body ? JSON.parse(opts.body) : null
    server.calls.push({ method, url, body })

    if (method === 'GET' && url === '/config.json') return respond(200, { lighting: server.lighting })
    if (method === 'POST' && url === '/config.json') {
      const { clear_api_key, api_key_configured, ...section } = body.lighting
      server.lighting = { ...server.lighting, ...section }
      return respond(200, { ok: true })
    }
    if (method === 'GET' && url === '/api/lighting/status') return respond(200, { state: 'ok' })
    if (method === 'GET' && url === '/api/lighting/lights') return respond(lights.status, lights.body)
    if (method === 'POST' && url === '/api/lighting/test') return respond(200, { result: 'ok' })
    if (method === 'POST' && url === '/api/lighting/preview') {
      const r = typeof preview === 'function' ? preview(body) : (preview ?? { status: 200, body: { result: 'ok' } })
      return respond(r.status, r.body)
    }
    throw new Error(`Route non mockée : ${method} ${url}`)
  })

  return server
}

const callsTo = (server, method, url) => server.calls.filter(c => c.method === method && c.url === url)

beforeEach(() => {
  useGame.mockReturnValue({ teams: {} })
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('AmbiancePage — P2b : allumage/extinction immédiat', () => {
  it('cocher une ampoule appelle POST /api/lighting/preview {name, on:true}, la case bascule immédiatement', async () => {
    const server = makeServer({ lighting: CONFIGURED })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    fireEvent.click(screen.getByLabelText('Salle gauche'))

    // Bascule synchrone, avant même que la promesse ne résolve.
    expect(screen.getByLabelText('Salle gauche')).toBeChecked()
    await waitFor(() => expect(callsTo(server, 'POST', '/api/lighting/preview')).toHaveLength(1))
    expect(callsTo(server, 'POST', '/api/lighting/preview')[0].body).toEqual({ name: 'Salle gauche', on: true })
  })

  it('décocher une ampoule appelle POST .../preview {name, on:false}', async () => {
    const server = makeServer({ lighting: CONFIGURED_WITH_LIGHT })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')
    expect(screen.getByLabelText('Salle gauche')).toBeChecked() // cochée par défaut, déjà en config

    fireEvent.click(screen.getByLabelText('Salle gauche'))

    expect(screen.getByLabelText('Salle gauche')).not.toBeChecked()
    await waitFor(() => expect(callsTo(server, 'POST', '/api/lighting/preview')).toHaveLength(1))
    expect(callsTo(server, 'POST', '/api/lighting/preview')[0].body).toEqual({ name: 'Salle gauche', on: false })
  })

  it('toutes les cases sont désactivées pendant l\'appel en vol, réactivées une fois terminé', async () => {
    let resolveFetch
    const server = makeServer({
      lighting: CONFIGURED,
      preview: () => new Promise(resolve => { resolveFetch = () => resolve({ status: 200, body: { result: 'ok' } }) }),
    })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    fireEvent.click(screen.getByLabelText('Salle gauche'))
    expect(screen.getByLabelText('Salle gauche')).toBeDisabled()
    expect(screen.getByLabelText('Salle droite')).toBeDisabled() // pas seulement la ligne cliquée

    resolveFetch()
    await waitFor(() => expect(screen.getByLabelText('Salle gauche')).not.toBeDisabled())
    expect(screen.getByLabelText('Salle droite')).not.toBeDisabled()
    void server
  })

  it('pont injoignable : toast d\'erreur affiché, mais la case reste basculée (§2.3 — la config ne dépend pas du pont)', async () => {
    makeServer({ lighting: CONFIGURED, preview: { status: 503, body: { result: 'unreachable' } } })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    fireEvent.click(screen.getByLabelText('Salle gauche'))

    await screen.findByText(/Pont injoignable/)
    expect(screen.getByLabelText('Salle gauche')).toBeChecked() // pas de rollback visuel
  })

  it('busy : message dédié, distinct des autres erreurs', async () => {
    makeServer({ lighting: CONFIGURED, preview: { status: 429, body: { result: 'busy' } } })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    fireEvent.click(screen.getByLabelText('Salle gauche'))

    await screen.findByText(/prévisualisation est déjà en cours/)
  })

  it('refused (association révoquée) : message dédié', async () => {
    makeServer({ lighting: CONFIGURED, preview: { status: 401, body: { result: 'refused' } } })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    fireEvent.click(screen.getByLabelText('Salle gauche'))

    await screen.findByText(/Association refusée/)
  })

  it('cas limite §2.3 : ampoule déjà enregistrée, décochée mais pas encore sauvegardée — mention discrète affichée puis retirée', async () => {
    makeServer({ lighting: CONFIGURED_WITH_LIGHT })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    expect(screen.queryByText('sera rallumée tant que non enregistré')).toBeNull() // encore cochée

    fireEvent.click(screen.getByLabelText('Salle gauche')) // décoche

    expect(screen.getByText('sera rallumée tant que non enregistré')).toBeInTheDocument()

    fireEvent.click(screen.getByLabelText('Salle gauche')) // recoche

    expect(screen.queryByText('sera rallumée tant que non enregistré')).toBeNull()
  })

  it('cas limite : ne s\'affiche PAS pour une ampoule jamais enregistrée (rien à rallumer)', async () => {
    makeServer({ lighting: CONFIGURED }) // "Salle gauche" jamais configurée
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    // Décochée par défaut (aucune présélection, revirement v10.0.0.13) —
    // et jamais dans lighting.lights : pas de mention.
    expect(screen.getByLabelText('Salle gauche')).not.toBeChecked()
    expect(screen.queryByText('sera rallumée tant que non enregistré')).toBeNull()
  })
})
