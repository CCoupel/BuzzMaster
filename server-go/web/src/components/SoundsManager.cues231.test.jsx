import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'

// ---------------------------------------------------------------------------
// #231 (v11.6.0, LOT-3) — SoundsManager hôte du panneau mutualisé : 15 bruitages en onglets (GET /api/sounds typé),
// colonnes catalogue (Remplacer / Tester) pour les nouvelles cues, non-régression des 7 cues et du décompte.
// Contrats : http-endpoints.md `GET /api/sounds` (`cues[].type`), sound.md §2.1 ; maquette
// ambiance_settings_panel__global_per_type_tabs.html. Composant monté SEUL, serveur simulé sur fetch.
// ---------------------------------------------------------------------------

vi.mock('../pages/AmbiancePage.css', () => ({}))
import SoundsManager from './SoundsManager'

const T = { depart: '', 'temps-ecoule': '', gagne: '', perdu: '', reveal: '', 'entracte-debut': '', 'entracte-fin': '',
  'qcm-indice': 'QCM', 'memory-carte': 'MEMORY', 'memory-paire': 'MEMORY', 'memotion-selection': 'MEMOTION',
  'memotion-fin-carte': 'MEMOTION', 'rafale-bonne': 'RAFALE', 'rafale-mauvaise': 'RAFALE', 'rafale-fin-lot': 'RAFALE' }
const FIFTEEN = Object.entries(T).map(([cue, type]) => ({
  cue, type, enabled: true, custom: false, duration_seconds: 0.5, path: `/files/sounds/${cue}.wav`,
}))
const DECOMPTE = { name: 'decompte', kind: 'countdown', enabled: true, loop: true, custom: false, duration_seconds: 2.0, path: '/files/sounds/decompte.wav' }

function makeServer({ cues = FIFTEEN } = {}) {
  const server = { sound: { enabled: true, cues_disabled: {} }, calls: [] }
  const respond = (status, body = {}) => ({ ok: status < 400, status, json: async () => body, text: async () => JSON.stringify(body) })
  global.fetch = vi.fn(async (url, opts = {}) => {
    const method = opts.method || 'GET'
    let body = null
    if (opts.body && typeof opts.body === 'string') { try { body = JSON.parse(opts.body) } catch { body = null } }
    server.calls.push({ method, url, body })
    if (method === 'GET' && url === '/config.json') return respond(200, { sound: server.sound, lighting: {} })
    if (method === 'POST' && url === '/config.json') {
      if (body?.sound) server.sound = { ...server.sound, ...body.sound }
      return respond(200, { ok: true })
    }
    if (method === 'GET' && url === '/api/sounds') {
      return respond(200, { cues: cues.map((c) => ({ ...c, enabled: !server.sound.cues_disabled?.[c.cue] })), long: [DECOMPTE] })
    }
    if (method === 'GET' && url === '/api/sound/status') return respond(200, { active: true })
    if (method === 'POST' && /^\/api\/sounds\/[^/]+$/.test(url)) return respond(200, { status: 'ok', custom: true, duration_seconds: 0.9, warning: null, converted: false, source_format: null })
    if (method === 'POST' && /^\/api\/sounds\/[^/]+\/test$/.test(url)) return respond(200, { result: 'played' })
    if (method === 'POST' && /^\/api\/sounds\/[^/]+\/restore$/.test(url)) return respond(200, { status: 'ok', custom: false, duration_seconds: 0.5 })
    if (method === 'POST' && url === '/api/sounds/restore-defaults') return respond(200, { status: 'ok', written: [] })
    return respond(404, { status: 'error', message: `non mockée ${method} ${url}` })
  })
  return server
}

const tab = (name) => screen.getByRole('tab', { name: new RegExp(`^${name}`) })
const callsTo = (s, m, u) => s.calls.filter((c) => c.method === m && c.url === u)

beforeEach(() => {
  window.HTMLMediaElement.prototype.play = vi.fn().mockResolvedValue(undefined)
  window.HTMLMediaElement.prototype.pause = vi.fn()
})
afterEach(() => vi.restoreAllMocks())

describe('SoundsManager — 15 bruitages en onglets (#231)', () => {
  it('cinq onglets avec compteurs 7 / 1 / 2 / 2 / 3 une fois la liste typée chargée', async () => {
    makeServer()
    render(<SoundsManager />)
    await screen.findByText('Départ')
    expect(tab('Global')).toHaveTextContent('7')
    expect(tab('QCM')).toHaveTextContent('1')
    expect(tab('MEMORY')).toHaveTextContent('2')
    expect(tab('MEMOTION')).toHaveTextContent('2')
    expect(tab('RAFALE')).toHaveTextContent('3')
  })

  it('onglet Global : les 7 cues historiques ET « Décompte du chronomètre » (sons longs) — rien d\'autre', async () => {
    makeServer()
    render(<SoundsManager />)
    await screen.findByText('Départ')
    expect(screen.getByText('Décompte du chronomètre')).toBeInTheDocument()
    expect(screen.queryByText('Indice révélé')).not.toBeInTheDocument()
  })

  it('onglet MEMORY : « Carte retournée » et « Paire trouvée » avec leurs boutons Remplacer / Tester', async () => {
    makeServer()
    render(<SoundsManager />)
    await screen.findByText('Départ')
    fireEvent.click(tab('MEMORY'))
    const row = screen.getByText('Carte retournée').closest('tr')
    expect(within(row).getByRole('button', { name: '▶ Tester' })).toBeInTheDocument()
    expect(within(row).getByText(/Remplacer/)).toBeInTheDocument()
    expect(screen.getByText('Paire trouvée')).toBeInTheDocument()
    expect(screen.queryByText('Décompte du chronomètre')).not.toBeInTheDocument()
  })

  it('les huit nouveaux libellés de la maquette sont présents dans leur onglet', async () => {
    makeServer()
    render(<SoundsManager />)
    await screen.findByText('Départ')
    const expected = { QCM: ['Indice révélé'], MEMORY: ['Carte retournée', 'Paire trouvée'],
      MEMOTION: ['Carte choisie', 'Fin de carte'], RAFALE: ['Bonne réponse', 'Mauvaise réponse', 'Fin de lot'] }
    for (const [name, labels] of Object.entries(expected)) {
      fireEvent.click(tab(name))
      labels.forEach((l) => expect(screen.getByText(l)).toBeInTheDocument())
    }
  })

  it("désactiver une nouvelle cue enregistre `cues_disabled` avec son nom (forme inchangée)", async () => {
    const server = makeServer()
    render(<SoundsManager />)
    await screen.findByText('Départ')
    fireEvent.click(tab('QCM'))
    fireEvent.click(screen.getByRole('checkbox', { name: /Indice révélé/ }))
    await waitFor(() => expect(callsTo(server, 'POST', '/config.json').length).toBeGreaterThan(0))
    const body = callsTo(server, 'POST', '/config.json')[0].body
    expect(body.sound.cues_disabled['qcm-indice']).toBe(true)
  })

  it('remplacer le fichier d\'une nouvelle cue : POST /api/sounds/<cue> puis toast', async () => {
    const server = makeServer()
    render(<SoundsManager />)
    await screen.findByText('Départ')
    fireEvent.click(tab('RAFALE'))
    const row = screen.getByText('Fin de lot').closest('tr')
    const input = row.querySelector('input[type="file"]')
    fireEvent.change(input, { target: { files: [new File([new Uint8Array(8)], 'fin.wav', { type: 'audio/wav' })] } })
    await waitFor(() => expect(callsTo(server, 'POST', '/api/sounds/rafale-fin-lot')).toHaveLength(1))
    expect(await screen.findByText(/Son remplacé/)).toBeInTheDocument()
  })
})

describe('SoundsManager — non-régression (#231)', () => {
  it('serveur antérieur (cues sans `type`) : pas d\'onglets, les 7 cues et le décompte restent affichés', async () => {
    makeServer({ cues: FIFTEEN.slice(0, 7).map(({ type, ...rest }) => rest) })
    render(<SoundsManager />)
    await screen.findByText('Départ')
    expect(screen.queryByRole('tab')).toBeNull()
    expect(screen.getByText('Décompte du chronomètre')).toBeInTheDocument()
  })

  it('le diagnostic de sortie audio (#283) et les 7 cues globales restent opérationnels', async () => {
    const server = makeServer()
    render(<SoundsManager />)
    const row = (await screen.findByText('Départ')).closest('tr')
    fireEvent.click(within(row).getByRole('button', { name: '▶ Tester' }))
    expect(await screen.findByRole('dialog', { name: /Tester le son/i })).toBeInTheDocument()
    expect(callsTo(server, 'POST', '/api/sounds/depart/test')).toHaveLength(0) // rien ne se joue à l'ouverture (T1 #230)
  })
})
