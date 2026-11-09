import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'

// ---------------------------------------------------------------------------
// #283 / #232 (v11.6.0, LOT-1) — onglet Son de /admin/ambiance : diagnostic de la sortie audio + toast « converti ».
// Maquette : docs/mockup/v11.6.0/ui/admin_sounds_manager__output_diagnostic.html.
// Contrat : contracts/http-endpoints.md « GET /api/sound/status — diagnostic » et « Uploads sonores — conversion ».
// Écrit depuis le contrat/la maquette (pas depuis l'implémentation) : rouge tant que dev-frontend n'a pas livré.
// Texte-pivot de la maquette : « Redémarrage requis », « Enceinte indisponible », « Bruitages désactivés ».
// ---------------------------------------------------------------------------

vi.mock('./AmbiancePage.css', () => ({}))
vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

import AmbiancePage from './AmbiancePage'
import { useGame } from '../hooks/GameContext'

const CUES = ['depart', 'temps-ecoule', 'gagne', 'perdu', 'reveal', 'entracte-debut', 'entracte-fin'].map((cue) => ({
  cue, enabled: true, custom: false, duration_seconds: 0.5, path: `/files/sounds/${cue}.wav`,
}))
const DECOMPTE = { name: 'decompte', kind: 'countdown', enabled: true, loop: true, custom: false, duration_seconds: 2.0, path: '/files/sounds/decompte.wav' }

const STATUS_OK = { active: true, muted: false, enabled: true, output: 'ok', reason: null, restart_required: false, muted_source: 'global' }

function makeServer({ status = STATUS_OK, uploadBody = { status: 'ok', custom: true, duration_seconds: 0.9, warning: null, converted: false, source_format: null } } = {}) {
  const respond = (code, body = {}) => ({ ok: code < 400, status: code, json: async () => body, text: async () => JSON.stringify(body) })
  global.fetch = vi.fn(async (url, opts = {}) => {
    const method = opts.method || 'GET'
    if (method === 'GET' && url === '/config.json') return respond(200, { lighting: {}, sound: { enabled: true, cues_disabled: {} } })
    if (method === 'GET' && url === '/api/lighting/status') return respond(200, { state: 'disabled' })
    if (method === 'GET' && url === '/api/sound/status') return respond(200, status)
    if (method === 'GET' && url === '/api/sounds') return respond(200, { cues: CUES, long: [DECOMPTE] })
    if (method === 'POST' && url === '/api/sounds/decompte') return respond(200, uploadBody)
    if (method === 'POST' && /^\/api\/sounds\/[^/]+$/.test(url)) return respond(200, uploadBody)
    throw new Error(`Route non mockée : ${method} ${url}`)
  })
}

async function openSoundTab() {
  render(<AmbiancePage />)
  await screen.findByText('Rechercher un pont')
  fireEvent.click(screen.getByRole('tab', { name: /Son/i }))
  await screen.findByText('Départ')
}

beforeEach(() => {
  useGame.mockReturnValue({ teams: {} })
  window.HTMLMediaElement.prototype.play = vi.fn().mockResolvedValue(undefined)
  window.HTMLMediaElement.prototype.pause = vi.fn()
})
afterEach(() => vi.restoreAllMocks())

describe('Onglet Son — diagnostic de la sortie audio (#283)', () => {
  it('sortie OK : ni bannière « Redémarrage requis » ni « Enceinte indisponible »', async () => {
    makeServer()
    await openSoundTab()
    expect(screen.queryByText(/Redémarrage requis/)).toBeNull()
    expect(screen.queryByText(/Enceinte indisponible/)).toBeNull()
  })

  it('output=restart_required : la bannière « Redémarrage requis » est affichée', async () => {
    makeServer({ status: { ...STATUS_OK, active: false, output: 'restart_required', restart_required: true, reason: 'enabled flipped false→true after startup' } })
    await openSoundTab()
    expect(await screen.findByText(/Redémarrage requis/)).toBeInTheDocument()
  })

  it('restart_required=true seul (sans output) suffit à afficher la bannière', async () => {
    makeServer({ status: { ...STATUS_OK, active: false, restart_required: true } })
    await openSoundTab()
    expect(await screen.findByText(/Redémarrage requis/)).toBeInTheDocument()
  })

  it('output=unavailable : « Enceinte indisponible » + la cause lisible renvoyée par le serveur', async () => {
    makeServer({ status: { ...STATUS_OK, active: false, output: 'unavailable', reason: 'oto: no output device' } })
    await openSoundTab()
    expect(await screen.findByText(/Enceinte indisponible/)).toBeInTheDocument()
    expect(screen.getByText(/oto: no output device/)).toBeInTheDocument()
    expect(screen.queryByText(/Redémarrage requis/)).toBeNull()
  })

  it('output=unavailable : « Tester » reste proposé sur chaque ligne (échec motivé plutôt que bouton retiré)', async () => {
    makeServer({ status: { ...STATUS_OK, active: false, output: 'unavailable', reason: 'oto: no output device' } })
    await openSoundTab()
    const row = screen.getByText('Départ').closest('tr')
    expect(within(row).getByRole('button', { name: '▶ Tester' })).toBeEnabled()
  })

  it('output=disabled : la note « Bruitages désactivés » (config.json) est affichée', async () => {
    makeServer({ status: { ...STATUS_OK, active: false, enabled: false, output: 'disabled', reason: 'sound.enabled=false' } })
    await openSoundTab()
    expect(await screen.findByText(/Bruitages désactivés/)).toBeInTheDocument()
    expect(screen.queryByText(/Redémarrage requis/)).toBeNull()
  })

  it('serveur antérieur à v11.6.0 (aucun champ de diagnostic) : aucune bannière, aucune erreur', async () => {
    makeServer({ status: { active: true, muted: false } })
    await openSoundTab()
    expect(screen.queryByText(/Redémarrage requis/)).toBeNull()
    expect(screen.queryByText(/Enceinte indisponible/)).toBeNull()
    expect(screen.queryByText(/Bruitages désactivés/)).toBeNull()
  })
})

describe('Onglet Son — toast d\'upload « converti » (#232)', () => {
  const upload = (input) => fireEvent.change(input, { target: { files: [new File([new Uint8Array(8)], 'bip.wav', { type: 'audio/wav' })] } })

  it('bruitage converti : le toast indique « converti depuis <source_format> »', async () => {
    makeServer({ uploadBody: { status: 'ok', custom: true, duration_seconds: 0.9, warning: null, converted: true, source_format: '48000Hz/1ch/24bit' } })
    await openSoundTab()
    const row = screen.getByText('Départ').closest('tr')
    upload(row.querySelector('input[type="file"]'))
    expect(await screen.findByText(/converti depuis 48000Hz\/1ch\/24bit/)).toBeInTheDocument()
  })

  it('bruitage déjà canonique : le toast ne mentionne aucune conversion', async () => {
    makeServer()
    await openSoundTab()
    const row = screen.getByText('Départ').closest('tr')
    upload(row.querySelector('input[type="file"]'))
    await screen.findByText(/Son remplacé/)
    expect(screen.queryByText(/converti/i)).toBeNull()
  })

  it('décompte converti : même message sur la ligne « Décompte du chronomètre »', async () => {
    makeServer({ uploadBody: { status: 'ok', custom: true, duration_seconds: 4.8, converted: true, source_format: '96000Hz/2ch/32bit' } })
    await openSoundTab()
    const row = (await screen.findByText('Décompte du chronomètre')).closest('tr')
    upload(row.querySelector('input[type="file"]'))
    expect(await screen.findByText(/converti depuis 96000Hz\/2ch\/32bit/)).toBeInTheDocument()
  })

  it('conversion + avertissement de durée : les deux informations figurent dans le toast', async () => {
    makeServer({ uploadBody: { status: 'ok', custom: true, duration_seconds: 3.1, warning: 'ce son dure 3.1 s — long', converted: true, source_format: '48000Hz/1ch/24bit' } })
    await openSoundTab()
    const row = screen.getByText('Départ').closest('tr')
    upload(row.querySelector('input[type="file"]'))
    const note = await screen.findByText(/converti depuis 48000Hz\/1ch\/24bit/)
    expect(note).toHaveTextContent(/ce son dure 3\.1 s/)
  })
})
