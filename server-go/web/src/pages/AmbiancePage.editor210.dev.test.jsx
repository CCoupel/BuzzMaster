import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'

// #210 (F2.3) — page Ambiance › Effets : « Modifier » ouvre l'éditeur à la place du catalogue ; Retour ramène au catalogue ; « Nouvel effet »
// ouvre un effet neuf (PUT create:true à l'enregistrement) ; « Dupliquer » depuis le catalogue ouvre une copie.
vi.mock('./AmbiancePage.css', () => ({}))
vi.mock('../components/LightGroupsManager.css', () => ({}))
vi.mock('../components/LightEffectsCatalogue.css', () => ({}))
vi.mock('../components/LightEffectEditor.css', () => ({}))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))

import AmbiancePage from './AmbiancePage'
import { useGame } from '../hooks/GameContext'

const json = (body, status = 200) => ({ ok: status < 400, status, json: async () => body, text: async () => JSON.stringify(body) })
const A = (target, color = 'theme', intensity = 100) => ({ target, color, intensity })
const DOC = { id: 'perso', name: 'Perso', description: '', moments: ['chrono'], min_groups: 0, tick_ms: 1000, loop: true, variables: [], pre: null, steps: [[A('g0')]], last: null, post: null }
const CATALOG = { groups: { ambiance: 1, animateur: 1, public: 0 }, effects: [
  { id: 'pulsation', name: 'Pulsation', moments: ['chrono'], origin: 'native', file: 'natifs/pulsation.yaml', min_groups: 0, tick_ms: 1000, applicable: { ambiance: true, animateur: true, public: false } },
  { id: 'perso', name: 'Perso', moments: ['chrono'], origin: 'user', file: 'perso.yaml', min_groups: 0, tick_ms: 1000, applicable: { ambiance: true, animateur: true, public: false } },
], errors: [], unresolved: [], dir: '' }
const SRC = (id, origin) => ({ id, origin, file: `${id}.yaml`, read_only: origin !== 'user', yaml: `id: ${id}\n`, effect: { ...DOC, id }, has_comments: false, custom_order: false, grid: { editable: true, effect: { ...DOC, id }, converted: [], blocked: [] } })

let calls
function server() {
  calls = []
  global.fetch = vi.fn(async (url, opts = {}) => {
    const method = opts.method || 'GET'
    calls.push({ url, method, body: opts.body && typeof opts.body === 'string' ? JSON.parse(opts.body) : null })
    if (url === '/config.json') return json({ lighting: { enabled: true, bridge_ip: '10.0.0.2', bridge_id: 'abc', api_key_configured: true, lights: [{ name: 'A', role: 'general' }], groups: [{ id: 'g1', role: 'salle', lights: ['A'] }] }, sound: { enabled: true } })
    if (url === '/api/lighting/status') return json({ state: 'ok', enabled: true, mode: 'AUTO', flash: false })
    if (url === '/api/sound/status') return json({ active: true })
    if (url === '/api/lighting/lights') return json({ lights: [{ id: '1', name: 'A', reachable: true }] })
    if (url === '/api/lighting/effects') return json(CATALOG)
    if (/\/source$/.test(url)) return json(SRC(url.split('/').at(-2), url.includes('pulsation') ? 'native' : 'user'))
    if (/\/draft$/.test(url)) { const b = JSON.parse(opts.body); return json({ ok: true, errors: [], effect: b.effect || DOC, yaml: 'id: x\n', has_comments: false, custom_order: false, grid: { editable: true, effect: b.effect || DOC, converted: [], blocked: [] }, moment: 'chrono', previews: (b.groups || []).map(n => ({ id: 'x', groups: n, frames: [{ tick: 0, step: 'pas:0', at_ms: 0, groups: [], vars: {} }] })) }) }
    if (method === 'PUT') return json({ ok: true, catalogue: CATALOG })
    return json({})
  })
}
const openEffects = async () => {
  render(<AmbiancePage />)
  fireEvent.click(await screen.findByRole('tab', { name: /Effets/ }))
  await screen.findByTestId('fx-perso')
}
const settled = async () => { await screen.findByTestId('effect-editor'); await screen.findByTestId('effect-grid'); await waitFor(() => expect(screen.queryByText('Vérification par le serveur…')).toBeNull()) }

beforeEach(() => { useGame.mockReturnValue({ teams: {}, gameState: { phase: 'STOPPED' } }) })
afterEach(() => { vi.restoreAllMocks() })

describe('AmbiancePage › éditeur d\'effet (#210)', () => {
  it('« Modifier » ouvre l\'éditeur à la place du catalogue ; « Retour au catalogue » le ramène', async () => {
    server()
    await openEffects()
    fireEvent.click(within(screen.getByTestId('fx-perso')).getByRole('button', { name: /Modifier/ }))
    await settled()
    expect(screen.queryByTestId('fx-perso')).toBeNull()
    expect(calls.some(c => /perso\/source$/.test(c.url))).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: /Retour au catalogue/ }))
    expect(await screen.findByTestId('fx-perso')).toBeInTheDocument()
  })

  it('« Ouvrir » (livré) : éditeur en lecture seule', async () => {
    server()
    await openEffects()
    fireEvent.click(within(screen.getByTestId('fx-pulsation')).getByRole('button', { name: /Ouvrir/ }))
    await settled()
    expect(screen.getByText(/est un effet livré avec BuzzControl/)).toBeInTheDocument()
  })

  it('« Nouvel effet » : fenêtre nom + identifiant, puis éditeur sur un effet neuf ; Enregistrer = PUT create:true', async () => {
    server()
    await openEffects()
    fireEvent.click(screen.getByRole('button', { name: /Nouvel effet/ }))
    fireEvent.change(screen.getByLabelText('Nom'), { target: { value: 'Mon essai' } })
    expect(screen.getByLabelText('Identifiant')).toHaveValue('mon-essai')
    fireEvent.click(screen.getByRole('button', { name: 'Créer' }))
    await settled()
    const save = screen.getByRole('button', { name: 'Enregistrer' })
    await waitFor(() => expect(save).not.toBeDisabled())
    fireEvent.click(save)
    await waitFor(() => expect(calls.some(c => c.method === 'PUT')).toBe(true))
    const put = calls.find(c => c.method === 'PUT')
    expect(put.url).toBe('/api/lighting/effects/mon-essai')
    expect(put.body.create).toBe(true)
    expect(put.body.effect.steps).toHaveLength(1)
  })

  it('« Dupliquer » depuis le catalogue : copie de la source sous un autre identifiant', async () => {
    server()
    await openEffects()
    fireEvent.click(within(screen.getByTestId('fx-pulsation')).getByRole('button', { name: /Dupliquer/ }))
    fireEvent.click(screen.getByRole('button', { name: 'Créer' }))
    await settled()
    expect(screen.getByDisplayValue('pulsation-copie')).toBeInTheDocument()
    expect(calls.some(c => /pulsation\/source$/.test(c.url))).toBe(true)
  })
})
