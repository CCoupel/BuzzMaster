import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'

// ---------------------------------------------------------------------------
// #230 — onglet Son de /admin/ambiance (SoundsManager.jsx, SoundTestModal.jsx).
// Maquette validée rév. 4 : docs/mockups/sound-config-230.html §02/§03.
// Contrat : contracts/http-endpoints.md §Sound, contracts/sound.md §6.3.
//
// Les SEPT tests qui comptent (handoff test-writer #230 §1) sont couverts
// ici nommément dans les commentaires de chaque `it`/`describe` : T1 (rien
// ne se joue à l'ouverture de la modale — LE test le plus important de
// cette issue), T2 (aucune réponse serveur ne positionne le verdict), T3
// est backend (cmd/server/sound_cuesdisabled_230_test.go), T4/T5 déjà
// vérifiés par les 5 fichiers AmbiancePage.*.test.jsx existants (inchangés
// hors le sous-titre, modification assumée — voir AmbiancePage.test.jsx),
// T6 est backend (internal/server/sound_admin_230_test.go), T7 (remplacer/
// restaurer efface les deux verdicts de la ligne).
//
// Rendu via <AmbiancePage /> complet (onglet Son), pas un import direct de
// SoundsManager/SoundTestModal — teste depuis le contrat/la maquette, pas
// depuis les noms de fichiers internes de dev-frontend, et exerce le VRAI
// câblage (bascule d'onglet, montage conditionnel).
// ---------------------------------------------------------------------------

vi.mock('./AmbiancePage.css', () => ({}))
vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

import AmbiancePage from './AmbiancePage'
import { useGame } from '../hooks/GameContext'

beforeEach(() => {
  useGame.mockReturnValue({ teams: {} })
  // jsdom n'implémente pas la lecture audio — sans ce mock,
  // HTMLMediaElement.prototype.play lève "Not implemented" à chaque appel,
  // ce qui rendrait TOUT test de l'écoute locale impossible à distinguer
  // d'un vrai échec de lecture.
  window.HTMLMediaElement.prototype.play = vi.fn().mockResolvedValue(undefined)
  window.HTMLMediaElement.prototype.pause = vi.fn()
})

afterEach(() => {
  vi.restoreAllMocks()
})

const SEVEN_CUES = [
  { cue: 'depart', enabled: true, custom: false, duration_seconds: 0.42, path: '/files/sounds/depart.wav' },
  { cue: 'temps-ecoule', enabled: true, custom: true, duration_seconds: 0.88, path: '/files/sounds/temps-ecoule.wav' },
  { cue: 'gagne', enabled: true, custom: false, duration_seconds: 0.61, path: '/files/sounds/gagne.wav' },
  { cue: 'perdu', enabled: false, custom: false, duration_seconds: 0.35, path: '/files/sounds/perdu.wav' },
  { cue: 'reveal', enabled: true, custom: false, duration_seconds: 0.74, path: '/files/sounds/reveal.wav' },
  { cue: 'entracte-debut', enabled: true, custom: false, duration_seconds: 0.95, path: '/files/sounds/entracte-debut.wav' },
  { cue: 'entracte-fin', enabled: true, custom: false, duration_seconds: 0.95, path: '/files/sounds/entracte-fin.wav' },
]

// Petit serveur simulé — même discipline que AmbiancePage.test.jsx::makeServer
// (throw sur toute route non mockée), étendu aux routes son du contrat.
function makeSoundServer({ cues = SEVEN_CUES, soundStatus = { active: true }, testResult = { result: 'played' } } = {}) {
  const server = {
    lighting: { enabled: false, bridge_ip: '', bridge_id: '', lights: [] },
    sound: { enabled: true, cues_disabled: {} },
    cues: cues.map(c => ({ ...c })),
    calls: [],
  }

  const respond = (status, body = {}) => ({
    ok: status < 400,
    status,
    json: async () => body,
    text: async () => JSON.stringify(body),
  })

  global.fetch = vi.fn(async (url, opts = {}) => {
    const method = opts.method || 'GET'
    let body = null
    if (opts.body && typeof opts.body === 'string') {
      try { body = JSON.parse(opts.body) } catch { body = null }
    }
    server.calls.push({ method, url, body })

    if (method === 'GET' && url === '/config.json') {
      return respond(200, { lighting: server.lighting, sound: server.sound })
    }
    if (method === 'POST' && url === '/config.json') {
      if (body.sound) server.sound = { ...server.sound, ...body.sound }
      return respond(200, { ok: true })
    }
    if (method === 'GET' && url === '/api/lighting/status') {
      return respond(200, { state: 'disabled' })
    }
    if (method === 'GET' && url === '/api/sound/status') {
      return respond(200, soundStatus)
    }
    if (method === 'GET' && url === '/api/sounds') {
      return respond(200, { cues: server.cues })
    }
    const testMatch = /^\/api\/sounds\/([^/]+)\/test$/.exec(url)
    if (method === 'POST' && testMatch) {
      return respond(200, typeof testResult === 'function' ? testResult(testMatch[1]) : testResult)
    }
    const restoreMatch = /^\/api\/sounds\/([^/]+)\/restore$/.exec(url)
    if (method === 'POST' && restoreMatch) {
      const cue = restoreMatch[1]
      server.cues = server.cues.map(c => (c.cue === cue ? { ...c, custom: false } : c))
      return respond(200, { status: 'ok', cue, custom: false, duration_seconds: 0.5 })
    }
    if (method === 'POST' && url === '/api/sounds/restore-defaults') {
      server.cues = server.cues.map(c => ({ ...c, custom: false }))
      return respond(200, { status: 'ok', written: server.cues.map(c => c.cue) })
    }
    const replaceMatch = /^\/api\/sounds\/([^/]+)$/.exec(url)
    if (method === 'POST' && replaceMatch) {
      const cue = replaceMatch[1]
      server.cues = server.cues.map(c => (c.cue === cue ? { ...c, custom: true, duration_seconds: 0.9 } : c))
      return respond(200, { status: 'ok', cue, custom: true, duration_seconds: 0.9, warning: null })
    }
    throw new Error(`Route non mockée : ${method} ${url}`)
  })

  return server
}

const callsTo = (server, method, url) => server.calls.filter(c => c.method === method && c.url === url)

/** Bascule vers l'onglet Son et attend que le tableau des sept sons soit rendu. */
async function openSoundTab() {
  const { container } = render(<AmbiancePage />)
  await screen.findByText('Rechercher un pont') // attend la fin du chargement initial (onglet Lumière)
  fireEvent.click(screen.getByRole('tab', { name: /Son/i }))
  await screen.findByText('Départ')
  return { container }
}

async function openTestModalFor(cueLabel) {
  const row = screen.getByText(cueLabel).closest('tr')
  fireEvent.click(within(row).getByRole('button', { name: '▶ Tester' }))
  return screen.findByRole('dialog', { name: /Tester le son/i })
}

// ===========================================================================
// Onglets — l'assistant Hue reste intact, bascule vers Son.
// ===========================================================================

describe('AmbiancePage — onglets Lumière/Son', () => {
  it("l'onglet Lumière est actif par défaut, l'onglet Son n'est pas rendu (T4/R1 : aucun <input> avant ouverture)", async () => {
    makeSoundServer()
    const { container } = render(<AmbiancePage />)
    await screen.findByText('Rechercher un pont')
    expect(screen.getByRole('tab', { name: /Lumière/i })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('tab', { name: /Son/i })).toHaveAttribute('aria-selected', 'false')
    expect(screen.queryByText('Sortie audio')).toBeNull()
    expect(container.querySelector('table.sound-table')).toBeNull()
  })

  it('bascule vers Son : affiche les sept lignes, revient à Lumière sans perte', async () => {
    makeSoundServer()
    await openSoundTab()
    for (const label of ['Départ', 'Temps écoulé', 'Points gagnés', 'Erreur', 'Révélation', "Début d'entracte", "Fin d'entracte"]) {
      expect(screen.getByText(label)).toBeInTheDocument()
    }
    fireEvent.click(screen.getByRole('tab', { name: /Lumière/i }))
    expect(screen.getByText('Trouver le pont')).toBeInTheDocument()
  })
})

// ===========================================================================
// Tableau — origine, durée, actions conditionnelles.
// ===========================================================================

describe('SoundsManager — tableau', () => {
  it('badge « Défaut »/« Personnalisé », jamais « livré »', async () => {
    makeSoundServer()
    await openSoundTab()
    const defaultRow = screen.getByText('Départ').closest('tr')
    expect(within(defaultRow).getByText('Défaut')).toBeInTheDocument()
    const customRow = screen.getByText('Temps écoulé').closest('tr')
    expect(within(customRow).getByText('Personnalisé')).toBeInTheDocument()
    expect(screen.queryByText('Livré', { exact: false })).toBeNull()
  })

  it('« Restaurer » n\'apparaît que sur les lignes personnalisées', async () => {
    makeSoundServer()
    await openSoundTab()
    const defaultRow = screen.getByText('Départ').closest('tr')
    expect(within(defaultRow).queryByRole('button', { name: /Restaurer/ })).toBeNull()
    const customRow = screen.getByText('Temps écoulé').closest('tr')
    expect(within(customRow).getByRole('button', { name: /Restaurer/ })).toBeInTheDocument()
  })

  it('ligne éteinte : classe is-off, mais le bouton Tester reste actif (mockup §02)', async () => {
    makeSoundServer()
    await openSoundTab()
    const offRow = screen.getByText('Erreur').closest('tr')
    expect(offRow.className).toContain('is-off')
    expect(within(offRow).getByRole('button', { name: '▶ Tester' })).not.toBeDisabled()
  })

  it('un seul GET /config.json au montage (T5, section sound comprise)', async () => {
    const server = makeSoundServer()
    await openSoundTab()
    expect(callsTo(server, 'GET', '/config.json')).toHaveLength(1)
  })
})

// ===========================================================================
// T1 — LA modale ne joue RIEN à l'ouverture. Le test le plus important.
// ===========================================================================

describe('SoundTestModal — T1 : aucune lecture automatique à l\'ouverture', () => {
  it("ouvrir la modale ne déclenche NI POST .../test NI lecture locale", async () => {
    const server = makeSoundServer()
    await openSoundTab()
    await openTestModalFor('Départ')

    expect(callsTo(server, 'POST', '/api/sounds/depart/test')).toHaveLength(0)
    expect(window.HTMLMediaElement.prototype.play).not.toHaveBeenCalled()
  })

  it("l'élément <audio> a preload=\"none\" (aucun chargement avant le clic)", async () => {
    makeSoundServer()
    await openSoundTab()
    const modal = await openTestModalFor('Départ')
    const audioEl = modal.querySelector('audio')
    expect(audioEl).not.toBeNull()
    expect(audioEl).toHaveAttribute('preload', 'none')
  })
})

// ===========================================================================
// Chaque bouton ne déclenche que sa propre écoute.
// ===========================================================================

describe('SoundTestModal — deux écoutes indépendantes', () => {
  it('« Jouer dans ce navigateur » ne déclenche AUCUN appel serveur', async () => {
    const server = makeSoundServer()
    await openSoundTab()
    const modal = await openTestModalFor('Départ')
    fireEvent.click(within(modal).getByRole('button', { name: /Jouer dans ce navigateur/ }))

    await waitFor(() => expect(window.HTMLMediaElement.prototype.play).toHaveBeenCalledTimes(1))
    expect(callsTo(server, 'POST', '/api/sounds/depart/test')).toHaveLength(0)
  })

  it('« Jouer sur l\'enceinte » ne déclenche AUCUNE lecture locale', async () => {
    const server = makeSoundServer()
    await openSoundTab()
    const modal = await openTestModalFor('Départ')
    fireEvent.click(within(modal).getByRole('button', { name: /Jouer sur l.?enceinte/ }))

    await waitFor(() => expect(callsTo(server, 'POST', '/api/sounds/depart/test')).toHaveLength(1))
    expect(window.HTMLMediaElement.prototype.play).not.toHaveBeenCalled()
  })
})

// ===========================================================================
// T2 — Aucune réponse serveur ne positionne le verdict, y compris "played".
// ===========================================================================

describe('SoundTestModal — T2 : le verdict reste exclusivement manuel', () => {
  it.each([
    ['played', 'Son envoyé à l’enceinte'],
    ['disabled', 'Bruitages désactivés — rien n’a été joué'],
    ['unavailable', 'Enceinte indisponible — rien n’a été joué'],
  ])('result=%s : message affiché, verdict serveur reste "Pas testé"', async (result, expectedMessage) => {
    makeSoundServer({ testResult: { result } })
    await openSoundTab()
    const modal = await openTestModalFor('Départ')

    fireEvent.click(within(modal).getByRole('button', { name: /Jouer sur l.?enceinte/ }))
    await screen.findByText(expectedMessage)

    const serverGroup = within(modal).getByRole('group', { name: /Verdict écoute serveur/i })
    expect(within(serverGroup).getByRole('button', { name: 'Pas testé' })).toHaveAttribute('aria-pressed', 'true')
    expect(within(serverGroup).getByRole('button', { name: 'Ok' })).toHaveAttribute('aria-pressed', 'false')
    expect(within(serverGroup).getByRole('button', { name: 'Ko' })).toHaveAttribute('aria-pressed', 'false')
  })

  it('poser le verdict est un geste manuel distinct, jamais déclenché par la réponse', async () => {
    makeSoundServer({ testResult: { result: 'played' } })
    await openSoundTab()
    const modal = await openTestModalFor('Départ')

    fireEvent.click(within(modal).getByRole('button', { name: /Jouer sur l.?enceinte/ }))
    await screen.findByText('Son envoyé à l’enceinte')

    const serverGroup = within(modal).getByRole('group', { name: /Verdict écoute serveur/i })
    fireEvent.click(within(serverGroup).getByRole('button', { name: 'Ok' }))
    expect(within(serverGroup).getByRole('button', { name: 'Ok' })).toHaveAttribute('aria-pressed', 'true')
  })
})

// ===========================================================================
// Les trois positions se posent indépendamment pour chaque section.
// ===========================================================================

describe('SoundTestModal — sélecteur à trois positions, par section', () => {
  it('poser le verdict local ne modifie pas le verdict serveur, et réciproquement', async () => {
    makeSoundServer()
    await openSoundTab()
    const modal = await openTestModalFor('Départ')

    const localGroup = within(modal).getByRole('group', { name: /Verdict écoute locale/i })
    const serverGroup = within(modal).getByRole('group', { name: /Verdict écoute serveur/i })

    fireEvent.click(within(localGroup).getByRole('button', { name: 'Ok' }))
    expect(within(localGroup).getByRole('button', { name: 'Ok' })).toHaveAttribute('aria-pressed', 'true')
    expect(within(serverGroup).getByRole('button', { name: 'Pas testé' })).toHaveAttribute('aria-pressed', 'true')

    fireEvent.click(within(serverGroup).getByRole('button', { name: 'Ko' }))
    expect(within(serverGroup).getByRole('button', { name: 'Ko' })).toHaveAttribute('aria-pressed', 'true')
    // Le verdict local posé plus haut doit rester "ok", inchangé.
    expect(within(localGroup).getByRole('button', { name: 'Ok' })).toHaveAttribute('aria-pressed', 'true')
  })
})

// ===========================================================================
// Les verdicts remontent dans la colonne Test du tableau.
// ===========================================================================

describe('SoundsManager — les verdicts remontent dans le tableau', () => {
  it('poser local=ok puis fermer : la ligne affiche un point vert (local) et gris (serveur)', async () => {
    makeSoundServer()
    await openSoundTab()
    const modal = await openTestModalFor('Départ')
    const localGroup = within(modal).getByRole('group', { name: /Verdict écoute locale/i })
    fireEvent.click(within(localGroup).getByRole('button', { name: 'Ok' }))
    fireEvent.click(within(modal).getByRole('button', { name: /Fermer/ }))

    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    const row = screen.getByText('Départ').closest('tr')
    const dots = row.querySelectorAll('.sound-verdict-dot')
    expect(dots[0].className).toContain('is-ok')
    expect(dots[1].className).toContain('is-untested')
  })
})

// ===========================================================================
// T7 — Remplacer ou restaurer efface les deux verdicts de la ligne.
// ===========================================================================

describe('SoundsManager — T7 : remplacer/restaurer efface les verdicts', () => {
  it('restaurer une ligne testée efface ses deux verdicts', async () => {
    makeSoundServer()
    await openSoundTab()

    // Pose un verdict sur "Temps écoulé" (personnalisé, a un bouton Restaurer).
    const modal = await openTestModalFor('Temps écoulé')
    const localGroup = within(modal).getByRole('group', { name: /Verdict écoute locale/i })
    fireEvent.click(within(localGroup).getByRole('button', { name: 'Ok' }))
    fireEvent.click(within(modal).getByRole('button', { name: /Fermer/ }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())

    let row = screen.getByText('Temps écoulé').closest('tr')
    expect(row.querySelectorAll('.sound-verdict-dot')[0].className).toContain('is-ok')

    fireEvent.click(within(row).getByRole('button', { name: /Restaurer/ }))
    await waitFor(() => expect(screen.getByText('Temps écoulé').closest('tr').querySelectorAll('.sound-tag')[0].textContent).toBe('Défaut'))

    row = screen.getByText('Temps écoulé').closest('tr')
    const dotsAfter = row.querySelectorAll('.sound-verdict-dot')
    expect(dotsAfter[0].className).toContain('is-untested')
    expect(dotsAfter[1].className).toContain('is-untested')
  })
})

// ===========================================================================
// Un rechargement remet tous les verdicts à « pas testé » — pas de
// persistance (plan-delta-lotC §2 : ni serveur, ni localStorage).
// ===========================================================================

describe('SoundsManager — verdicts éphémères', () => {
  it('un nouveau montage démarre toujours avec des verdicts "pas testé"', async () => {
    // Les verdicts vivent exclusivement dans l'état React de SoundsManager
    // (plan-delta-lotC §2 : "ni serveur, ni localStorage") — un nouveau
    // montage n'a donc structurellement RIEN à lire nulle part ; ce test
    // vérifie l'état initial observable, pas un mécanisme de stockage
    // (localStorage n'est pas disponible dans cet environnement jsdom).
    makeSoundServer()
    await openSoundTab()
    const row = screen.getByText('Départ').closest('tr')
    const dots = row.querySelectorAll('.sound-verdict-dot')
    expect(dots[0].className).toContain('is-untested')
    expect(dots[1].className).toContain('is-untested')
  })
})

// ===========================================================================
// Écritures de configuration — patch partiel { sound }, jamais { lighting }.
// ===========================================================================

describe('SoundsManager — interrupteur, patch partiel', () => {
  it('éteindre une ligne écrit { sound: { cues_disabled } } sans toucher lighting', async () => {
    const server = makeSoundServer()
    await openSoundTab()
    const row = screen.getByText('Départ').closest('tr')
    fireEvent.click(within(row).getByRole('checkbox'))

    await waitFor(() => expect(callsTo(server, 'POST', '/config.json')).toHaveLength(1))
    const call = callsTo(server, 'POST', '/config.json')[0]
    expect(call.body).toHaveProperty('sound')
    expect(call.body).not.toHaveProperty('lighting')
    // "perdu" (Erreur) est déjà éteint dans la fixture SEVEN_CUES — la carte
    // reconstruite doit porter les DEUX cues éteintes, pas seulement celle
    // qu'on vient de toggler (SoundsManager reconstruit `cues_disabled` en
    // entier à partir de l'état affiché, pas une fusion partielle).
    expect(call.body.sound.cues_disabled).toEqual({ depart: true, perdu: true })
  })
})
