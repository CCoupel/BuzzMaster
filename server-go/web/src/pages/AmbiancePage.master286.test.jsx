import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

// #286 — interrupteur général « Éclairage d'ambiance activé » (lighting.enabled),
// onglet Lumière de /admin/ambiance. Contrat : contracts/sound.md §11.4.

vi.mock('./AmbiancePage.css', () => ({}))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))

import AmbiancePage from './AmbiancePage'
import { useGame } from '../hooks/GameContext'

const json = (body, status = 200) => ({ ok: status < 400, status, json: async () => body, text: async () => JSON.stringify(body) })

function makeServer(lighting) {
  const posts = []
  global.fetch = vi.fn(async (url, opts = {}) => {
    if (opts.method === 'POST' && url === '/config.json') {
      const body = JSON.parse(opts.body)
      posts.push(body)
      return json({})
    }
    if (url === '/config.json') return json({ lighting, sound: { enabled: true } })
    if (url === '/api/lighting/status') return json({ state: 'ok', enabled: lighting.enabled })
    if (url === '/api/sound/status') return json({ active: true, muted: false })
    if (url === '/api/lighting/lights') return json({ lights: [] })
    return json({})
  })
  return posts
}
const CFG = { bridge_ip: '10.0.0.2', bridge_id: 'x', api_key_configured: true, lights: [] }

beforeEach(() => { useGame.mockReturnValue({ teams: {} }) })
afterEach(() => { vi.restoreAllMocks() })

describe('AmbiancePage › Lumière — interrupteur général (#286)', () => {
  it('pont associé + activé : bouton pressé, étapes non grisées', async () => {
    makeServer({ ...CFG, enabled: true })
    render(<AmbiancePage />)
    const sw = await screen.findByRole('button', { name: /Éclairage d'ambiance activé/ })
    expect(sw).toHaveAttribute('aria-pressed', 'true')
    expect(document.querySelector('.ambiance-step-lights')).not.toHaveClass('is-dimmed')
  })

  it('désactivé : non pressé, étapes grisées, pastille « désactivé »', async () => {
    makeServer({ ...CFG, enabled: false })
    render(<AmbiancePage />)
    const sw = await screen.findByRole('button', { name: /Éclairage d'ambiance activé/ })
    expect(sw).toHaveAttribute('aria-pressed', 'false')
    expect(document.querySelector('.ambiance-step-lights')).toHaveClass('is-dimmed')
    expect(document.querySelector('.ambiance-status-badge').dataset.state).toBe('disabled')
  })

  it('clic → POST /config.json {lighting:{enabled:false}}', async () => {
    const posts = makeServer({ ...CFG, enabled: true })
    render(<AmbiancePage />)
    fireEvent.click(await screen.findByRole('button', { name: /Éclairage d'ambiance activé/ }))
    await waitFor(() => expect(posts).toEqual([{ lighting: { enabled: false } }]))
  })

  it('sans pont associé : aucun interrupteur (assistant étape 1)', async () => {
    makeServer({ enabled: false, bridge_ip: '', api_key_configured: false, lights: [] })
    render(<AmbiancePage />)
    await screen.findByText('Trouver le pont')
    expect(screen.queryByRole('button', { name: /Éclairage d'ambiance activé/ })).toBeNull()
  })
})
