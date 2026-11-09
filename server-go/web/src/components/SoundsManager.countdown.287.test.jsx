import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'

// ---------------------------------------------------------------------------
// #287/#288 (v11.4.0, LOT-3) — SoundsManager : section « Sons longs » et ligne « Décompte du chronomètre »
// (maquette docs/mockup/v11.4.0/ui/admin_sounds_manager__countdown_loop.html §01 ; contrats http-endpoints.md §Sound,
// sound.md §12.3). Le composant est monté SEUL (aucune prop), serveur simulé sur fetch.
// Tests existants des 7 bruitages (AmbiancePage.sound.test.jsx) : inchangés, doivent rester verts.
// ---------------------------------------------------------------------------

vi.mock('../pages/AmbiancePage.css', () => ({}))
import SoundsManager from './SoundsManager'

const SEVEN = ['depart', 'temps-ecoule', 'gagne', 'perdu', 'reveal', 'entracte-debut', 'entracte-fin'].map((cue) => ({
  cue, enabled: true, custom: false, duration_seconds: 0.5, path: `/files/sounds/${cue}.wav`,
}))

const decompte = (over = {}) => ({
  name: 'decompte', kind: 'countdown', enabled: true, loop: true, custom: false, duration_seconds: 2.0,
  path: '/files/sounds/decompte.wav', ...over,
})

function makeServer({ long = [decompte()], testResult = { result: 'played' } } = {}) {
  const server = { long: long.map((l) => ({ ...l })), sound: { enabled: true, cues_disabled: {} }, calls: [] }
  const respond = (status, body = {}) => ({ ok: status < 400, status, json: async () => body, text: async () => JSON.stringify(body) })
  global.fetch = vi.fn(async (url, opts = {}) => {
    const method = opts.method || 'GET'
    let body = null
    if (opts.body && typeof opts.body === 'string') { try { body = JSON.parse(opts.body) } catch { body = null } }
    server.calls.push({ method, url, body, rawBody: opts.body })
    if (method === 'GET' && url === '/config.json') return respond(200, { sound: server.sound, lighting: {} })
    if (method === 'POST' && url === '/config.json') {
      if (body?.sound) {
        server.sound = { ...server.sound, ...body.sound }
        server.long = server.long.map((l) => ({
          ...l,
          enabled: body.sound.countdown_disabled === undefined ? l.enabled : !body.sound.countdown_disabled,
          loop: body.sound.countdown_once === undefined ? l.loop : !body.sound.countdown_once,
        }))
      }
      return respond(200, { ok: true })
    }
    if (method === 'GET' && url === '/api/sounds') return respond(200, { cues: SEVEN, long: server.long })
    if (method === 'GET' && url === '/api/sound/status') return respond(200, { active: true })
    if (method === 'POST' && url === '/api/sounds/decompte') {
      server.long = server.long.map((l) => ({ ...l, custom: true, duration_seconds: 4.8 }))
      return respond(200, { status: 'ok', custom: true, duration_seconds: 4.8 })
    }
    if (method === 'POST' && url === '/api/sounds/decompte/restore') {
      server.long = server.long.map((l) => ({ ...l, custom: false, duration_seconds: 2.0 }))
      return respond(200, { status: 'ok', custom: false, duration_seconds: 2.0 })
    }
    if (method === 'POST' && url === '/api/sounds/decompte/test') return respond(200, testResult)
    if (method === 'POST' && url === '/api/sounds/restore-defaults') return respond(200, { status: 'ok', restored: [] })
    if (method === 'POST' && /^\/api\/sounds\/[^/]+\/test$/.test(url)) return respond(200, { result: 'played' })
    return respond(404, { status: 'error', message: `non mockée ${method} ${url}` })
  })
  return server
}

const callsTo = (s, m, u) => s.calls.filter((c) => c.method === m && c.url === u)

async function openRow() {
  render(<SoundsManager />)
  const label = await screen.findByText('Décompte du chronomètre')
  return label.closest('tr')
}

beforeEach(() => {
  window.HTMLMediaElement.prototype.play = vi.fn().mockResolvedValue(undefined)
  window.HTMLMediaElement.prototype.pause = vi.fn()
})
afterEach(() => vi.restoreAllMocks())

describe('SoundsManager — section « Sons longs » (#287)', () => {
  it('deux séparateurs : « Bruitages (impulsions, ≤ 5 s) » puis « Sons longs (≤ 30 s) »', async () => {
    makeServer()
    await openRow()
    expect(screen.getByText(/Bruitages \(impulsions, ≤ 5 s\)/)).toBeInTheDocument()
    expect(screen.getByText(/Sons longs \(≤ 30 s\)/)).toBeInTheDocument()
  })

  it('la ligne Décompte porte son libellé, son sous-texte et sa durée « 2,00 s »', async () => {
    makeServer()
    const row = await openRow()
    expect(within(row).getByText(/Pendant que le chronomètre décompte/)).toBeInTheDocument()
    expect(within(row).getByText('2,00 s')).toBeInTheDocument()
    expect(within(row).getByText('Défaut')).toBeInTheDocument()
  })

  it('les 7 bruitages restent rendus, inchangés (la colonne Lecture vaut « — » sur les cues)', async () => {
    makeServer()
    await openRow()
    for (const label of ['Départ', 'Temps écoulé', 'Points gagnés', 'Erreur', 'Révélation', "Début d'entracte", "Fin d'entracte"]) {
      const row = screen.getByText(label).closest('tr')
      expect(within(row).getAllByText('—').length).toBeGreaterThanOrEqual(1)
    }
  })

  it('le décompte n\'apparaît jamais comme une cue de la table des bruitages (CUE_LABELS inchangé)', async () => {
    makeServer()
    await openRow()
    expect(screen.getAllByText('Décompte du chronomètre')).toHaveLength(1)
  })
})

// #290 (v11.6.0, LOT-4) — les commandes « Actif » et « Lecture » du décompte vivent désormais dans la section
// « Décompte » du composant mutualisé AmbianceSettingsPanel (maquette ambiance_settings_panel__global_per_type_tabs :
// « Décompte du chronomètre  Actif / Inactif  Une fois / En boucle »), plus dans la ligne « Sons longs » (qui garde
// fichier, durée, Remplacer / Restaurer / Tester). L'intention des tests d'origine est conservée : mêmes patchs
// `countdown_disabled` / `countdown_once`, champ par champ.
const cdActive = () => screen.getByRole('radiogroup', { name: /décompte actif/i })
const cdLoop = () => screen.getByRole('radiogroup', { name: /lecture du décompte/i })

describe('SoundsManager — Décompte : réglages (#287/#288 → section « Décompte » du panneau #290)', () => {
  it('« Actif » sélectionné par défaut ; « Inactif » ⇒ POST /config.json {sound:{countdown_disabled:true}}', async () => {
    const server = makeServer()
    await openRow()
    expect(within(cdActive()).getByRole('radio', { name: 'Actif' })).toHaveAttribute('aria-checked', 'true')
    fireEvent.click(within(cdActive()).getByRole('radio', { name: 'Inactif' }))
    await waitFor(() => expect(callsTo(server, 'POST', '/config.json')).toHaveLength(1))
    expect(callsTo(server, 'POST', '/config.json')[0].body).toEqual({ sound: { countdown_disabled: true } })
  })

  it('décompte désactivé côté serveur : « Inactif » sélectionné ; « Tester » reste actif (geste explicite)', async () => {
    makeServer({ long: [decompte({ enabled: false })] })
    const row = await openRow()
    await waitFor(() => expect(within(cdActive()).getByRole('radio', { name: 'Inactif' })).toHaveAttribute('aria-checked', 'true'))
    expect(within(row).getByRole('button', { name: /Tester/ })).not.toBeDisabled()
  })

  it('Lecture : « En boucle » sélectionné par défaut ; « Une fois » ⇒ POST {sound:{countdown_once:true}}', async () => {
    const server = makeServer()
    await openRow()
    expect(within(cdLoop()).getByRole('radio', { name: 'En boucle' })).toHaveAttribute('aria-checked', 'true')
    fireEvent.click(within(cdLoop()).getByRole('radio', { name: 'Une fois' }))
    await waitFor(() => expect(callsTo(server, 'POST', '/config.json')).toHaveLength(1))
    expect(callsTo(server, 'POST', '/config.json')[0].body).toEqual({ sound: { countdown_once: true } })
  })

  it('Lecture « Une fois » côté serveur (loop=false) puis « En boucle » ⇒ POST {sound:{countdown_once:false}}', async () => {
    const server = makeServer({ long: [decompte({ loop: false })] })
    await openRow()
    await waitFor(() => expect(within(cdLoop()).getByRole('radio', { name: 'Une fois' })).toHaveAttribute('aria-checked', 'true'))
    fireEvent.click(within(cdLoop()).getByRole('radio', { name: 'En boucle' }))
    await waitFor(() => expect(callsTo(server, 'POST', '/config.json')).toHaveLength(1))
    expect(callsTo(server, 'POST', '/config.json')[0].body).toEqual({ sound: { countdown_once: false } })
  })

  it('changer la boucle n\'écrase pas countdown_disabled (patch champ par champ)', async () => {
    const server = makeServer()
    await openRow()
    fireEvent.click(within(cdLoop()).getByRole('radio', { name: 'Une fois' }))
    await waitFor(() => expect(callsTo(server, 'POST', '/config.json')).toHaveLength(1))
    expect(Object.keys(callsTo(server, 'POST', '/config.json')[0].body.sound)).toEqual(['countdown_once'])
  })
})

describe('SoundsManager — Décompte : remplacer / restaurer / tester (#287)', () => {
  it('« Restaurer » n\'est visible que si le fichier est personnalisé', async () => {
    makeServer()
    const row = await openRow()
    expect(within(row).queryByRole('button', { name: /Restaurer/ })).toBeNull()
  })

  it('fichier personnalisé : badge « Personnalisé », « Restaurer » visible ; clic ⇒ POST /api/sounds/decompte/restore', async () => {
    const server = makeServer({ long: [decompte({ custom: true, duration_seconds: 4.8 })] })
    const row = await openRow()
    expect(within(row).getByText('Personnalisé')).toBeInTheDocument()
    expect(within(row).getByText('4,80 s')).toBeInTheDocument()
    fireEvent.click(within(row).getByRole('button', { name: /Restaurer/ }))
    await waitFor(() => expect(callsTo(server, 'POST', '/api/sounds/decompte/restore')).toHaveLength(1))
    await waitFor(() => expect(within(screen.getByText('Décompte du chronomètre').closest('tr')).queryByRole('button', { name: /Restaurer/ })).toBeNull())
  })

  it('« Remplacer » : POST /api/sounds/decompte en multipart avec le champ « file », puis la ligne devient personnalisée', async () => {
    const server = makeServer()
    const row = await openRow()
    const input = row.querySelector('input[type="file"]')
    expect(input).not.toBeNull()
    expect(input.getAttribute('accept')).toMatch(/wav/i)
    const file = new File(['RIFFxxxx'], 'tictac.wav', { type: 'audio/wav' })
    fireEvent.change(input, { target: { files: [file] } })
    await waitFor(() => expect(callsTo(server, 'POST', '/api/sounds/decompte')).toHaveLength(1))
    const sent = callsTo(server, 'POST', '/api/sounds/decompte')[0].rawBody
    expect(sent).toBeInstanceOf(FormData)
    expect(sent.get('file')).toBe(file)
    await waitFor(() => expect(within(screen.getByText('Décompte du chronomètre').closest('tr')).getByText('Personnalisé')).toBeInTheDocument())
  })

  it('« Tester » ouvre la modale d\'écoute SANS rien jouer ; « Jouer sur l\'enceinte » ⇒ POST /api/sounds/decompte/test', async () => {
    const server = makeServer()
    const row = await openRow()
    fireEvent.click(within(row).getByRole('button', { name: /Tester/ }))
    const modal = await screen.findByRole('dialog', { name: /Tester le son/i })
    expect(callsTo(server, 'POST', '/api/sounds/decompte/test')).toHaveLength(0)
    expect(window.HTMLMediaElement.prototype.play).not.toHaveBeenCalled()
    fireEvent.click(within(modal).getByRole('button', { name: /Jouer sur l.?enceinte/ }))
    await waitFor(() => expect(callsTo(server, 'POST', '/api/sounds/decompte/test')).toHaveLength(1))
    expect(server.calls.filter((c) => /\/api\/sounds\/(?!decompte)[^/]+\/test$/.test(c.url))).toHaveLength(0)
  })

  it('« Jouer dans ce navigateur » lit decompte.wav localement (aucun appel serveur de test)', async () => {
    const server = makeServer()
    const row = await openRow()
    fireEvent.click(within(row).getByRole('button', { name: /Tester/ }))
    const modal = await screen.findByRole('dialog', { name: /Tester le son/i })
    fireEvent.click(within(modal).getByRole('button', { name: /Jouer dans ce navigateur/ }))
    await waitFor(() => expect(window.HTMLMediaElement.prototype.play).toHaveBeenCalledTimes(1))
    expect(callsTo(server, 'POST', '/api/sounds/decompte/test')).toHaveLength(0)
  })

  it('texte d\'aide : « Restaurer tout (7 bruitages + décompte) »', async () => {
    makeServer()
    await openRow()
    expect(screen.getByText(/Restaurer tout \(7 bruitages \+ décompte\)/)).toBeInTheDocument()
  })
})
