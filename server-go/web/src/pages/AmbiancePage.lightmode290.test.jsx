import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'

// ---------------------------------------------------------------------------
// #290 (v11.6.0, LOT-4) — Ambiance › Lumière : bloc « Réglages d'ambiance — Lumière » du composant mutualisé
// (section Lumière activée en mode global) = mode d'éclairage PAR DÉFAUT persisté `lighting.default_mode`
// (ON / AUTO / OFF ; absent ⇒ AUTO). Contrat lighting.md §12, ambiance-settings.md §2/§6 ; maquette
// ambiance_settings_panel__global_per_type_tabs.html §02. Écrit depuis le contrat/la maquette.
// ---------------------------------------------------------------------------

vi.mock('./AmbiancePage.css', () => ({}))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))

import AmbiancePage from './AmbiancePage'
import { useGame } from '../hooks/GameContext'

const json = (body, status = 200) => ({ ok: status < 400, status, json: async () => body, text: async () => JSON.stringify(body) })
const CFG = { bridge_ip: '10.0.0.2', bridge_id: 'x', api_key_configured: true, enabled: true, lights: [] }

function makeServer(lighting) {
  const posts = []
  global.fetch = vi.fn(async (url, opts = {}) => {
    if (opts.method === 'POST' && url === '/config.json') { posts.push(JSON.parse(opts.body)); return json({ status: 'ok', scope: 'global' }) }
    if (url === '/config.json') return json({ lighting, sound: { enabled: true } })
    if (url === '/api/lighting/status') return json({ state: 'ok', enabled: lighting.enabled })
    if (url === '/api/sound/status') return json({ active: true, muted: false })
    if (url === '/api/lighting/lights') return json({ lights: [] })
    return json({})
  })
  return posts
}

beforeEach(() => { useGame.mockReturnValue({ teams: {} }) })
afterEach(() => { vi.restoreAllMocks() })

const modeGroup = async () => within(await screen.findByRole('radiogroup', { name: /mode d'éclairage/i }))

describe('AmbiancePage › Lumière — mode d\'éclairage par défaut (#290)', () => {
  it('le bloc « Réglages d\'ambiance — Lumière » propose ON / AUTO / OFF (sans « Hérité » : c\'est le global)', async () => {
    makeServer({ ...CFG })
    render(<AmbiancePage />)
    expect(await screen.findByText(/Réglages d'ambiance — Lumière/)).toBeInTheDocument()
    const g = await modeGroup()
    for (const n of ['ON', 'AUTO', 'OFF']) expect(g.getByRole('radio', { name: n })).toBeInTheDocument()
    expect(g.queryByRole('radio', { name: 'Hérité' })).toBeNull()
  })

  it('default_mode absent ⇒ AUTO sélectionné', async () => {
    makeServer({ ...CFG })
    render(<AmbiancePage />)
    expect((await modeGroup()).getByRole('radio', { name: 'AUTO' })).toHaveAttribute('aria-checked', 'true')
  })

  it('default_mode « ON » chargé ⇒ ON sélectionné', async () => {
    makeServer({ ...CFG, default_mode: 'ON' })
    render(<AmbiancePage />)
    await waitFor(async () => expect((await modeGroup()).getByRole('radio', { name: 'ON' })).toHaveAttribute('aria-checked', 'true'))
  })

  it('choisir OFF ⇒ POST /config.json {lighting:{default_mode:"OFF"}} (réglage persisté, pas le popover)', async () => {
    const posts = makeServer({ ...CFG })
    render(<AmbiancePage />)
    fireEvent.click((await modeGroup()).getByRole('radio', { name: 'OFF' }))
    await waitFor(() => expect(posts.some((p) => p.lighting && p.lighting.default_mode === 'OFF')).toBe(true))
    // jamais via le geste direct du popover
    expect(global.fetch.mock.calls.some((c) => String(c[0]).includes('/api/lighting/mode'))).toBe(false)
  })

  it('l\'écriture ne touche pas aux autres champs d\'éclairage (patch champ par champ)', async () => {
    const posts = makeServer({ ...CFG })
    render(<AmbiancePage />)
    fireEvent.click((await modeGroup()).getByRole('radio', { name: 'ON' }))
    await waitFor(() => expect(posts.length).toBeGreaterThan(0))
    const lightingPosts = posts.filter((p) => p.lighting && 'default_mode' in p.lighting)
    expect(Object.keys(lightingPosts[0].lighting)).toEqual(['default_mode'])
  })
})
