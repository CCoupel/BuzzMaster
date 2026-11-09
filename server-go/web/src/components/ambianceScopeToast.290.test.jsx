import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

// ---------------------------------------------------------------------------
// #290 (v11.6.0, LOT-4) — retour visuel d'un geste direct retenu pour la manche seulement : la réponse porte
// `scope: "round"` ⇒ message « Appliqué à la manche en cours seulement (la manche impose ce réglage). » ; `scope:
// "global"` (ou absent, ou corps non JSON) ⇒ AUCUN message et aucune exception. Trois hôtes : popover Son
// (SoundTogglePanel), popover Éclairage (LightingModePanel), page Ambiance › Son (SoundsManager : cues + décompte).
// Contrat http-endpoints.md (`scope`), ambiance-settings.md §4.2. Écrit depuis le contrat.
// ---------------------------------------------------------------------------

vi.mock('./LightingModePanel.css', () => ({}))
vi.mock('../pages/AmbiancePage.css', () => ({}))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(() => ({ teams: {} })), GameProvider: ({ children }) => children }))
vi.mock('../hooks/useSoundStatus', async (orig) => ({ ...(await orig()), notifySoundChanged: vi.fn() }))

import SoundTogglePanel from './SoundTogglePanel'
import LightingModePanel from './LightingModePanel'
import SoundsManager from './SoundsManager'
import AmbiancePage from '../pages/AmbiancePage'

const MSG = /Appliqué à la manche en cours seulement/

// Réponse fetch : `body` = objet JSON, ou `null` pour un corps NON JSON (json() lève).
const reply = (body, status = 200) => {
  const make = () => ({
    ok: status < 400, status,
    json: async () => { if (body === null) throw new SyntaxError('not json'); return body },
    text: async () => (body === null ? 'OK' : JSON.stringify(body)),
  })
  const r = make()
  r.clone = make
  return r
}

afterEach(() => vi.restoreAllMocks())

describe('SoundTogglePanel — geste « Son » (#290)', () => {
  const setup = (postBody) => {
    global.fetch = vi.fn(async () => reply(postBody))
    const refresh = vi.fn().mockResolvedValue(null)
    render(<SoundTogglePanel muted={false} refresh={refresh} />)
    return refresh
  }

  it('scope "round" ⇒ message affiché', async () => {
    setup({ status: 'ok', scope: 'round' })
    fireEvent.click(screen.getByRole('radio', { name: 'OFF' }))
    expect(await screen.findByText(MSG)).toBeInTheDocument()
  })

  it('scope "global" ⇒ aucun message', async () => {
    const refresh = setup({ status: 'ok', scope: 'global' })
    fireEvent.click(screen.getByRole('radio', { name: 'OFF' }))
    await waitFor(() => expect(refresh).toHaveBeenCalled())
    expect(screen.queryByText(MSG)).toBeNull()
  })

  it('corps non JSON ⇒ aucun message, aucune erreur affichée, le rafraîchissement a lieu', async () => {
    const refresh = setup(null)
    fireEvent.click(screen.getByRole('radio', { name: 'OFF' }))
    await waitFor(() => expect(refresh).toHaveBeenCalled())
    expect(screen.queryByText(MSG)).toBeNull()
    expect(screen.queryByText(/^Erreur : /)).toBeNull()
  })

  it('le message est effacé au geste suivant', async () => {
    global.fetch = vi.fn()
      .mockResolvedValueOnce(reply({ status: 'ok', scope: 'round' }))
      .mockResolvedValueOnce(reply({ status: 'ok', scope: 'global' }))
    const { rerender } = render(<SoundTogglePanel muted={false} refresh={vi.fn().mockResolvedValue(null)} />)
    fireEvent.click(screen.getByRole('radio', { name: 'OFF' }))
    await screen.findByText(MSG)
    rerender(<SoundTogglePanel muted={true} refresh={vi.fn().mockResolvedValue(null)} />)
    fireEvent.click(screen.getByRole('radio', { name: 'ON' }))
    await waitFor(() => expect(screen.queryByText(MSG)).toBeNull())
  })
})

describe('LightingModePanel — geste « Éclairage général » (#290)', () => {
  const setup = (modeBody) => {
    global.fetch = vi.fn(async (url, opts = {}) => {
      if (url === '/api/lighting/status') return reply({ state: 'ok', mode: 'AUTO', flash: false })
      if (url === '/api/lighting/mode' && opts.method === 'POST') return reply(modeBody)
      return reply({})
    })
    render(<LightingModePanel />)
  }
  const clickOn = async () => fireEvent.click(await screen.findByRole('radio', { name: 'ON' }))

  it('scope "round" ⇒ toast affiché', async () => {
    setup({ result: 'ok', mode: 'ON', scope: 'round' })
    await clickOn()
    expect(await screen.findByText(MSG)).toBeInTheDocument()
  })

  it('scope "global" ⇒ aucun message', async () => {
    setup({ result: 'ok', mode: 'ON', scope: 'global' })
    await clickOn()
    await waitFor(() => expect(global.fetch.mock.calls.some((c) => c[0] === '/api/lighting/mode')).toBe(true))
    expect(screen.queryByText(MSG)).toBeNull()
  })

  it('corps non JSON ⇒ aucun message ni erreur', async () => {
    setup(null)
    await clickOn()
    await waitFor(() => expect(global.fetch.mock.calls.some((c) => c[0] === '/api/lighting/mode')).toBe(true))
    expect(screen.queryByText(MSG)).toBeNull()
    expect(screen.queryByText(/^Erreur : /)).toBeNull()
  })
})

describe('SoundsManager — cues et décompte (#290)', () => {
  const CUES = ['depart', 'temps-ecoule', 'gagne', 'perdu', 'reveal', 'entracte-debut', 'entracte-fin'].map((cue) => ({
    cue, type: '', enabled: true, custom: false, duration_seconds: 0.5, path: `/files/sounds/${cue}.wav`,
  }))
  const DECOMPTE = { name: 'decompte', kind: 'countdown', enabled: true, loop: true, custom: false, duration_seconds: 2, path: '/files/sounds/decompte.wav' }
  const setup = (configBody) => {
    global.fetch = vi.fn(async (url, opts = {}) => {
      const method = opts.method || 'GET'
      if (method === 'GET' && url === '/api/sounds') return reply({ cues: CUES, long: [DECOMPTE] })
      if (method === 'GET' && url === '/config.json') return reply({ sound: { enabled: true, cues_disabled: {} }, lighting: {} })
      if (method === 'GET' && url === '/api/sound/status') return reply({ active: true })
      if (method === 'POST' && url === '/config.json') return reply(configBody)
      return reply({})
    })
    window.HTMLMediaElement.prototype.play = vi.fn().mockResolvedValue(undefined)
    window.HTMLMediaElement.prototype.pause = vi.fn()
    render(<SoundsManager />)
  }
  const toggleDepart = async () => fireEvent.click(await screen.findByRole('checkbox', { name: /Départ/ }))

  it('interrupteur d\'une cue, scope "round" ⇒ toast', async () => {
    setup({ status: 'ok', scope: 'round' })
    await toggleDepart()
    expect(await screen.findByText(MSG)).toBeInTheDocument()
  })

  it('interrupteur d\'une cue, scope "global" ⇒ aucun message', async () => {
    setup({ status: 'ok', scope: 'global' })
    await toggleDepart()
    await waitFor(() => expect(global.fetch.mock.calls.some((c) => c[0] === '/config.json' && c[1]?.method === 'POST')).toBe(true))
    expect(screen.queryByText(MSG)).toBeNull()
  })

  it('décompte « Inactif », scope "round" ⇒ toast', async () => {
    setup({ status: 'ok', scope: 'round' })
    await screen.findByText('Départ')
    fireEvent.click(screen.getByRole('radio', { name: 'Inactif' }))
    expect(await screen.findByText(MSG)).toBeInTheDocument()
  })

  it('corps non JSON ⇒ aucun message, aucun toast d\'erreur', async () => {
    setup(null)
    await toggleDepart()
    await waitFor(() => expect(global.fetch.mock.calls.some((c) => c[0] === '/config.json' && c[1]?.method === 'POST')).toBe(true))
    expect(screen.queryByText(MSG)).toBeNull()
    expect(screen.queryByText(/^Erreur : /)).toBeNull()
  })
})

describe('AmbiancePage › Lumière — mode d\'éclairage par défaut (#290)', () => {
  const CFG = { enabled: true, bridge_ip: '10.0.0.2', bridge_id: 'x', api_key_configured: true, lights: [] }
  const setup = (postBody) => {
    global.fetch = vi.fn(async (url, opts = {}) => {
      const method = opts.method || 'GET'
      if (method === 'POST' && url === '/config.json') return reply(postBody)
      if (url === '/config.json') return reply({ lighting: CFG, sound: { enabled: true } })
      if (url === '/api/lighting/status') return reply({ state: 'ok', enabled: true })
      if (url === '/api/sound/status') return reply({ active: true, muted: false })
      if (url === '/api/lighting/lights') return reply({ lights: [] })
      return reply({})
    })
    render(<AmbiancePage />)
  }
  const clickOn = async () => {
    const group = await screen.findByRole('radiogroup', { name: "Mode d'éclairage" })
    fireEvent.click(Array.from(group.querySelectorAll('button')).find((b) => b.textContent === 'ON'))
  }
  const posted = () => global.fetch.mock.calls.some((c) => c[0] === '/config.json' && c[1]?.method === 'POST')

  it('scope "round" ⇒ toast affiché', async () => {
    setup({ status: 'ok', scope: 'round' })
    await clickOn()
    expect(await screen.findByText(MSG)).toBeInTheDocument()
  })

  it('scope "global" ⇒ aucun message', async () => {
    setup({ status: 'ok', scope: 'global' })
    await clickOn()
    await waitFor(() => expect(posted()).toBe(true))
    expect(screen.queryByText(MSG)).toBeNull()
  })

  it('corps non JSON ⇒ aucun message ni erreur', async () => {
    setup(null)
    await clickOn()
    await waitFor(() => expect(posted()).toBe(true))
    expect(screen.queryByText(MSG)).toBeNull()
    expect(screen.queryByText(/^Erreur : /)).toBeNull()
  })
})
