import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

// #223 — bouton « Tester » (page Ambiance › Lumière › Effets) : grisé pendant une question, filet 409 `game`.
vi.mock('./AmbiancePage.css', () => ({}))
vi.mock('../components/LightGroupsManager.css', () => ({}))
vi.mock('../components/LightEffectsCatalogue.css', () => ({}))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))

import AmbiancePage from './AmbiancePage'
import { useGame } from '../hooks/GameContext'

const json = (body, status = 200) => ({ ok: status < 400, status, json: async () => body, text: async () => JSON.stringify(body) })
const CATALOG = { groups: { ambiance: 1, animateur: 1, public: 0 }, effects: [
  { id: 'pulsation', name: 'Pulsation', moments: ['chrono'], origin: 'native', min_groups: 0, tick_ms: 1000, applicable: { ambiance: true, animateur: true, public: false } },
], errors: [], unresolved: [], dir: '' }

function server({ tryStatus = 200, tryBody = { ok: true, duration_ms: 10000 }, groups } = {}) {
  const calls = []
  global.fetch = vi.fn(async (url, opts = {}) => {
    if (opts.method === 'POST') { calls.push({ url, body: opts.body ? JSON.parse(opts.body) : null }); return json(tryBody, tryStatus) }
    if (url === '/config.json') {
      return json({ lighting: {
        enabled: true, bridge_ip: '10.0.0.2', bridge_id: 'abc', api_key_configured: true,
        lights: [{ name: 'A', role: 'general' }, { name: 'B', role: 'general' }],
        groups: groups ?? [{ id: 'g1', role: 'salle', lights: ['A'] }, { id: 'h1', role: 'animateur', lights: ['B'] }],
      }, sound: { enabled: true } })
    }
    if (url === '/api/lighting/status') return json({ state: 'ok', enabled: true, mode: 'AUTO', flash: false })
    if (url === '/api/sound/status') return json({ active: true })
    if (url === '/api/lighting/lights') return json({ lights: [{ id: '1', name: 'A', reachable: true }, { id: '2', name: 'B', reachable: true }] })
    if (url === '/api/lighting/effects') return json(CATALOG)
    return json({})
  })
  return calls
}
const open = async () => {
  fireEvent.click(await screen.findByRole('tab', { name: /Effets/ }))
  await screen.findByTestId('asp-zone-ambiance')
}
const tryBtn = (zone) => document.getElementById(`asp-try-${zone}-chrono`)

beforeEach(() => { useGame.mockReturnValue({ teams: {}, gameState: { phase: 'STOPPED' } }) })
afterEach(() => { vi.restoreAllMocks() })

describe('AmbiancePage › Effets — bouton « Tester »', () => {
  it('actif hors question : poste zone, vitesse réglée et 10 s', async () => {
    const calls = server()
    render(<AmbiancePage />)
    await open()
    await waitFor(() => expect(tryBtn('ambiance')).not.toBeDisabled())
    fireEvent.click(tryBtn('ambiance'))
    await waitFor(() => expect(calls.at(-1)).toEqual({ url: '/api/lighting/effects/pulsation/try', body: { zone: 'ambiance', speed: 1, seconds: 10 } }))
  })

  it.each(['STARTED', 'PAUSED', 'REVEALED', 'COUNTDOWN'])('grisé avec infobulle pendant une question (%s)', async (phase) => {
    useGame.mockReturnValue({ teams: {}, gameState: { phase } })
    server()
    render(<AmbiancePage />)
    await open()
    await waitFor(() => expect(tryBtn('ambiance')).toBeDisabled())
    expect(tryBtn('ambiance').title).toBe('Une question est en cours')
  })

  it('filet de sécurité : 409 `game` du serveur → toast explicatif', async () => {
    server({ tryStatus: 409, tryBody: { reason: 'game' } })
    render(<AmbiancePage />)
    await open()
    await waitFor(() => expect(tryBtn('ambiance')).not.toBeDisabled())
    fireEvent.click(tryBtn('ambiance'))
    expect(await screen.findByText(/question est en cours/i)).toBeInTheDocument()
  })

  it('zone sans groupe non vide : pas de bloc (groupe public vide)', async () => {
    server({ groups: [{ id: 'g1', role: 'salle', lights: ['A'] }, { id: 'p1', role: 'public', lights: [] }] })
    render(<AmbiancePage />)
    await open()
    expect(screen.queryByTestId('asp-zone-public')).toBeNull()
    expect(screen.queryByTestId('asp-zone-animateur')).toBeNull()
  })

  it('catalogue sous les réglages : « Essayer sur la salle » poste un corps sans zone ni vitesse', async () => {
    const calls = server()
    render(<AmbiancePage />)
    await open()
    const card = await screen.findByTestId('fx-pulsation')
    fireEvent.click(card.querySelector('button[title^="Joue l\'effet sur la zone Ambiance"]'))
    await waitFor(() => expect(calls.at(-1)).toEqual({ url: '/api/lighting/effects/pulsation/try', body: {} }))
  })

  it('catalogue : essai grisé pendant une question', async () => {
    useGame.mockReturnValue({ teams: {}, gameState: { phase: 'STARTED' } })
    server()
    render(<AmbiancePage />)
    await open()
    const card = await screen.findByTestId('fx-pulsation')
    await waitFor(() => expect(card.querySelector('button[title^="Une question est en cours"]')).toBeDisabled())
  })
})
