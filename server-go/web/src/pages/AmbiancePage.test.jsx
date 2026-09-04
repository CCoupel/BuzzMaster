import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, act, within } from '@testing-library/react'
import AmbiancePage, { REGISTER_RETRY_MS, REGISTER_TIMEOUT_S } from './AmbiancePage'
import { LIGHTING_CHANGED_EVENT } from '../hooks/useLightingStatus'

// ---------------------------------------------------------------------------
// #207 — /admin/ambiance. Maquette validée rév. 4 :
// docs/mockups/lighting-hue-config-207.html. Contrat : contracts/hue-bridge.md
// §6 (config `lighting`), §7 (endpoints), §5.6 (taxonomie à trois issues).
//
// Le backend (#206/#207) n'existe pas encore : `global.fetch` est mocké par
// route, avec les formes de réponse EXACTES du contrat §7.
// ---------------------------------------------------------------------------

vi.mock('./AmbiancePage.css', () => ({}))

// Petit serveur simulé : la section `lighting` de config.json évolue avec les
// POST, comme le ferait handleConfig (remplacement de section, clé préservée
// si absente, effacée si clear_api_key). `keyStored` = clé obtenue par register.
function makeServer({ lighting = {}, status = { state: 'disabled' }, register, lights, discover, test } = {}) {
  const server = {
    keyStored: !!lighting.api_key_configured,
    lighting: { enabled: false, bridge_ip: '', bridge_id: '', lights: [], ...lighting },
    status,
    calls: [],
  }
  delete server.lighting.api_key_configured

  const respond = (status, body = {}) => ({
    ok: status < 400,
    status,
    json: async () => body,
    text: async () => JSON.stringify(body),
  })

  global.fetch = vi.fn(async (url, opts = {}) => {
    const method = opts.method || 'GET'
    const body = opts.body ? JSON.parse(opts.body) : null
    server.calls.push({ method, url, body })

    if (method === 'GET' && url === '/config.json') {
      return respond(200, { lighting: { ...server.lighting, api_key_configured: server.keyStored } })
    }
    if (method === 'POST' && url === '/config.json') {
      // Merge champ par champ, comme handleConfig (http.go) : seules les clés
      // PRÉSENTES sont écrites — y compris une chaîne vide.
      const { clear_api_key, api_key_configured, ...section } = body.lighting
      if (clear_api_key) server.keyStored = false
      server.lighting = { ...server.lighting, ...section }
      return respond(200, { ok: true })
    }
    if (method === 'GET' && url === '/api/lighting/status') {
      return respond(200, typeof server.status === 'function' ? server.status() : server.status)
    }
    if (method === 'POST' && url === '/api/lighting/discover') {
      const r = typeof discover === 'function' ? discover() : (discover ?? { bridges: [] })
      return respond(200, r)
    }
    if (method === 'POST' && url === '/api/lighting/register') {
      const r = typeof register === 'function' ? register(body) : (register ?? { status: 409, body: { result: 'refused', reason: 'link_button_not_pressed' } })
      if (r.status === 200) {
        // Comme http_lighting.go : clé + bridge_ip + bridge_id lu sur le pont
        // sont persistés côté serveur AVANT la réponse 200.
        server.keyStored = true
        server.lighting.bridge_ip = body.bridge_ip
        server.lighting.bridge_id = r.body?.bridge_id ?? ''
      }
      return respond(r.status, r.body)
    }
    if (method === 'GET' && url === '/api/lighting/lights') {
      const r = typeof lights === 'function' ? lights() : (lights ?? { status: 200, body: { lights: [] } })
      return respond(r.status, r.body)
    }
    if (method === 'POST' && url === '/api/lighting/test') {
      const r = typeof test === 'function' ? test(body) : (test ?? { status: 200, body: { result: 'ok' } })
      return respond(r.status, r.body)
    }
    throw new Error(`Route non mockée : ${method} ${url}`)
  })

  return server
}

const callsTo = (server, method, url) => server.calls.filter(c => c.method === method && c.url === url)

const BRIDGE = { ip: '192.168.1.101', id: '001788fffea0591e', model: 'BSB002' }
const INVENTORY = {
  status: 200,
  body: {
    lights: [
      { id: '8', name: 'Salle gauche', reachable: true, on: false },
      { id: '9', name: 'Salle droite', reachable: true, on: true },
      { id: '11', name: 'Scène', reachable: false, on: false },
    ],
  },
}
const CONFIGURED = { enabled: true, bridge_ip: BRIDGE.ip, bridge_id: BRIDGE.id, api_key_configured: true, lights: [] }

const badge = () => document.querySelector('.ambiance-status-badge')

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

// ===========================================================================
// Invariants transverses
// ===========================================================================

describe('AmbiancePage — invariants', () => {
  it('n\'expose JAMAIS de champ de saisie de clé (elle s\'obtient par l\'appui bouton)', async () => {
    makeServer()
    const { container } = render(<AmbiancePage />)
    await screen.findByText('Rechercher un pont')
    expect(container.querySelector('input[type="password"]')).toBeNull()
    expect(container.querySelector('input')).toBeNull()
    expect(screen.queryByText(/clé/i)).toBeNull()
  })

  it('squelette admin : page-header + page-title + page-subtitle, puis une Card', async () => {
    makeServer()
    const { container } = render(<AmbiancePage />)
    await screen.findByText('Rechercher un pont')
    expect(container.querySelector('header.page-header h1.page-title').textContent).toBe('Ambiance')
    expect(container.querySelector('header.page-header p.page-subtitle').textContent).toBe('Éclairage de la salle piloté par le jeu')
    expect(container.querySelector('.card.ambiance-card')).not.toBeNull()
  })

  it('interroge le statut au montage (ampoule/badge) et la config', async () => {
    const server = makeServer()
    render(<AmbiancePage />)
    await screen.findByText('Rechercher un pont')
    expect(callsTo(server, 'GET', '/api/lighting/status')).toHaveLength(1)
    expect(callsTo(server, 'GET', '/config.json')).toHaveLength(1)
  })
})

// ===========================================================================
// Étape 1 — trouver le pont
// ===========================================================================

describe('Étape 1 — non configuré, découverte', () => {
  it('badge « Non configuré », seul bouton « Rechercher un pont », étapes 2 et 3 absentes', async () => {
    makeServer()
    render(<AmbiancePage />)
    await screen.findByText('Rechercher un pont')
    expect(badge().textContent).toContain('Non configuré')
    expect(badge().dataset.state).toBe('disabled')
    expect(screen.queryByText(/Appuyez sur le bouton/)).toBeNull()
    expect(screen.queryByText('Enregistrer')).toBeNull()
    expect(screen.queryByText('Dissocier ce pont')).toBeNull()
    // Le champ IP n'est PAS proposé d'emblée (maquette §03).
    expect(screen.queryByLabelText('Adresse du pont')).toBeNull()
  })

  it('un pont trouvé : adresse + identifiant affichés, « Associer ce pont » et « Rechercher à nouveau »', async () => {
    const server = makeServer({ discover: { bridges: [BRIDGE] } })
    render(<AmbiancePage />)
    fireEvent.click(await screen.findByText('Rechercher un pont'))

    await screen.findByText('Associer ce pont')
    expect(callsTo(server, 'POST', '/api/lighting/discover')).toHaveLength(1)
    expect(screen.getByText('Pont détecté')).toBeInTheDocument()
    expect(screen.getByText(BRIDGE.ip)).toBeInTheDocument()
    expect(screen.getByText(BRIDGE.id)).toBeInTheDocument()
    expect(screen.getByText(/Modèle BSB002/)).toBeInTheDocument()
    expect(screen.getByText('Rechercher à nouveau')).toBeInTheDocument()
  })

  it('aucun pont : message explicatif, lien « Saisir l\'adresse manuellement » qui déplie un champ IP', async () => {
    makeServer({ discover: { bridges: [] } })
    render(<AmbiancePage />)
    fireEvent.click(await screen.findByText('Rechercher un pont'))

    await screen.findByText('Aucun pont trouvé.')
    expect(screen.getByText(/Le pont est-il allumé/)).toBeInTheDocument()
    expect(screen.queryByLabelText('Adresse du pont')).toBeNull()

    fireEvent.click(screen.getByText("Saisir l'adresse manuellement"))
    const input = screen.getByLabelText('Adresse du pont')
    expect(input).toBeInTheDocument()

    const useBtn = screen.getByText('Utiliser cette adresse').closest('button')
    expect(useBtn).toBeDisabled()
    fireEvent.change(input, { target: { value: '10.0.0.5' } })
    expect(useBtn).not.toBeDisabled()
    fireEvent.click(useBtn)

    expect(screen.getByText('Pont saisi')).toBeInTheDocument()
    expect(screen.getByText('10.0.0.5')).toBeInTheDocument()
    expect(screen.getByText('Associer ce pont')).toBeInTheDocument()
  })

  it('découverte 429 busy : « Une recherche est déjà en cours », pas une panne', async () => {
    global.fetch = vi.fn(async (url, opts = {}) => {
      if (url === '/config.json') return { ok: true, status: 200, json: async () => ({ lighting: {} }) }
      if (url === '/api/lighting/status') return { ok: true, status: 200, json: async () => ({ state: 'disabled' }) }
      if (url === '/api/lighting/discover') return { ok: false, status: 429, json: async () => ({ result: 'busy', reason: 'discover_in_progress' }) }
      throw new Error(`inattendu ${opts.method || 'GET'} ${url}`)
    })
    render(<AmbiancePage />)
    fireEvent.click(await screen.findByText('Rechercher un pont'))
    await screen.findByText('Une recherche est déjà en cours.')
    expect(screen.queryByRole('alert')).toBeNull()
    expect(screen.getByText('Rechercher à nouveau')).toBeInTheDocument()
  })

  it('plusieurs ponts : liste à choix, AUCUN présélectionné, « Associer » inactif tant qu\'aucun choix', async () => {
    const other = { ip: '192.168.1.102', id: 'aaaa0000bbbb1111', model: 'BSB002' }
    makeServer({ discover: { bridges: [BRIDGE, other] } })
    render(<AmbiancePage />)
    fireEvent.click(await screen.findByText('Rechercher un pont'))

    await screen.findByText(/Plusieurs ponts trouvés/)
    const radios = screen.getAllByRole('radio')
    expect(radios).toHaveLength(2)
    radios.forEach(r => expect(r).not.toBeChecked())
    expect(screen.getByText('Associer ce pont').closest('button')).toBeDisabled()

    fireEvent.click(radios[1])
    expect(radios[1]).toBeChecked()
    expect(screen.getByText('Associer ce pont').closest('button')).not.toBeDisabled()
  })
})

// ===========================================================================
// Étape 2 — associer (appui bouton). Timers simulés.
// ===========================================================================

describe('Étape 2 — association par appui bouton', () => {
  const tick = async (ms = 0) => { await act(async () => { await vi.advanceTimersByTimeAsync(ms) }) }

  // Amène la page jusqu'à l'attente « appuyez sur le bouton ».
  async function startPairing(server) {
    render(<AmbiancePage />)
    await tick()
    fireEvent.click(screen.getByText('Rechercher un pont'))
    await tick()
    fireEvent.click(screen.getByText('Associer ce pont'))
    await tick()
    return server
  }

  beforeEach(() => { vi.useFakeTimers() })

  it('attente EN LIGNE (pas de modale), compte à rebours, relance toutes les 2 s, « Annuler »', async () => {
    const server = await startPairing(makeServer({ discover: { bridges: [BRIDGE] } }))

    expect(screen.getByText('Appuyez sur le bouton rond au centre du pont')).toBeInTheDocument()
    expect(screen.getByText(/réessaie automatiquement toutes les 2 secondes/)).toBeInTheDocument()
    expect(screen.getByText(`${REGISTER_TIMEOUT_S} s`)).toBeInTheDocument()
    expect(document.querySelector('[role="dialog"]')).toBeNull()
    expect(document.querySelector('.modal, .modal-overlay')).toBeNull()
    expect(screen.getByText('Annuler')).toBeInTheDocument()
    // Le 409 « bouton non pressé » est NOMINAL : aucune alarme, aucun mot « erreur ».
    expect(screen.queryByRole('alert')).toBeNull()
    expect(screen.queryByText(/erreur/i)).toBeNull()

    expect(callsTo(server, 'POST', '/api/lighting/register')).toHaveLength(1)
    expect(callsTo(server, 'POST', '/api/lighting/register')[0].body).toEqual({ bridge_ip: BRIDGE.ip })

    await tick(REGISTER_RETRY_MS)
    expect(callsTo(server, 'POST', '/api/lighting/register')).toHaveLength(2)
    await tick(REGISTER_RETRY_MS)
    expect(callsTo(server, 'POST', '/api/lighting/register')).toHaveLength(3)
    expect(screen.getByText(`${REGISTER_TIMEOUT_S - 4} s`)).toBeInTheDocument()
  })

  it('succès après deux 409 : la page n\'envoie QUE enabled:true (ip/id/clé déjà persistés par register), toast « Pont associé. », étape 3, Navbar prévenue', async () => {
    let n = 0
    const server = makeServer({
      discover: { bridges: [BRIDGE] },
      register: () => (++n < 3
        ? { status: 409, body: { result: 'refused', reason: 'link_button_not_pressed' } }
        : { status: 200, body: { result: 'ok', bridge_id: BRIDGE.id } }),
      lights: INVENTORY,
    })
    const onChanged = vi.fn()
    window.addEventListener(LIGHTING_CHANGED_EVENT, onChanged)
    await startPairing(server)

    await tick(REGISTER_RETRY_MS)
    await tick(REGISTER_RETRY_MS)
    await tick()

    const saves = callsTo(server, 'POST', '/config.json')
    expect(saves).toHaveLength(1)
    expect(saves[0].body).toEqual({ lighting: { enabled: true } })
    // Ni bridge_id ni bridge_ip ne sont renvoyés : une clé présente serait
    // écrite telle quelle par le merge serveur, même vide (bug de revue).
    expect(saves[0].body.lighting).not.toHaveProperty('bridge_id')
    expect(saves[0].body.lighting).not.toHaveProperty('bridge_ip')
    expect(JSON.stringify(saves[0].body)).not.toMatch(/api_key/)
    expect(server.lighting.bridge_id).toBe(BRIDGE.id)
    expect(screen.getByText(BRIDGE.id)).toBeInTheDocument()

    expect(screen.getByText('Pont associé.')).toBeInTheDocument()
    expect(screen.queryByText(/Appuyez sur le bouton/)).toBeNull()
    expect(screen.getByText('Ampoules du pont')).toBeInTheDocument()
    expect(onChanged).toHaveBeenCalledTimes(1)
    // Statut re-interrogé après l'enregistrement (contrat §7.1).
    expect(callsTo(server, 'GET', '/api/lighting/status').length).toBeGreaterThanOrEqual(2)
    window.removeEventListener(LIGHTING_CHANGED_EVENT, onChanged)
  })

  it('délai écoulé sans appui : « Bouton non pressé » — cas nominal, pas une panne — puis « Réessayer »', async () => {
    const server = await startPairing(makeServer({ discover: { bridges: [BRIDGE] } }))

    await tick(REGISTER_TIMEOUT_S * 1000 + REGISTER_RETRY_MS)

    expect(screen.getByText('Bouton non pressé')).toBeInTheDocument()
    expect(screen.getByText(/Le délai de 45 secondes est écoulé. Rien n'a été enregistré./)).toBeInTheDocument()
    expect(screen.queryByRole('alert')).toBeNull()
    expect(screen.queryByText(/erreur/i)).toBeNull()
    expect(callsTo(server, 'POST', '/config.json')).toHaveLength(0)

    const before = callsTo(server, 'POST', '/api/lighting/register').length
    await tick(10_000)
    expect(callsTo(server, 'POST', '/api/lighting/register')).toHaveLength(before) // plus de relance

    fireEvent.click(screen.getByText('Réessayer'))
    await tick()
    expect(screen.getByText('Appuyez sur le bouton rond au centre du pont')).toBeInTheDocument()
    expect(screen.getByText(`${REGISTER_TIMEOUT_S} s`)).toBeInTheDocument()
    expect(callsTo(server, 'POST', '/api/lighting/register')).toHaveLength(before + 1)
  })

  it('503 unreachable : « Pont injoignable » (jamais confondu avec refusé), pas de relance automatique', async () => {
    const server = await startPairing(makeServer({
      discover: { bridges: [BRIDGE] },
      register: { status: 503, body: { result: 'unreachable' } },
    }))

    expect(screen.getByRole('alert').textContent).toContain('Pont injoignable')
    expect(screen.queryByText(/refus/i)).toBeNull()
    await tick(10_000)
    expect(callsTo(server, 'POST', '/api/lighting/register')).toHaveLength(1)
    expect(screen.getByText('Réessayer')).toBeInTheDocument()
  })

  it('saisie IP manuelle (aucun pont trouvé) : le bridge_id lu par register est CONSERVÉ, jamais écrasé à vide', async () => {
    const server = makeServer({
      discover: { bridges: [] },
      register: { status: 200, body: { result: 'ok', bridge_id: '0017deadbeef0001' } },
      lights: INVENTORY,
    })
    render(<AmbiancePage />)
    await tick()
    fireEvent.click(screen.getByText('Rechercher un pont'))
    await tick()
    fireEvent.click(screen.getByText("Saisir l'adresse manuellement"))
    fireEvent.change(screen.getByLabelText('Adresse du pont'), { target: { value: '192.168.1.50' } })
    fireEvent.click(screen.getByText('Utiliser cette adresse'))
    fireEvent.click(screen.getByText('Associer ce pont'))
    await tick()
    await tick()

    expect(callsTo(server, 'POST', '/api/lighting/register')[0].body).toEqual({ bridge_ip: '192.168.1.50' })
    const saves = callsTo(server, 'POST', '/config.json')
    expect(saves).toHaveLength(1)
    expect(saves[0].body.lighting).not.toHaveProperty('bridge_id')
    expect(server.lighting.bridge_id).toBe('0017deadbeef0001')
    expect(server.lighting.bridge_ip).toBe('192.168.1.50')
    expect(screen.getByText('0017deadbeef0001')).toBeInTheDocument()
    expect(screen.getByText('Ampoules du pont')).toBeInTheDocument()
  })

  it('après le 200, l\'étape 2 reste affichée jusqu\'à l\'étape 3 — jamais de passage par « Trouver le pont »', async () => {
    makeServer({
      discover: { bridges: [BRIDGE] },
      register: { status: 200, body: { result: 'ok', bridge_id: BRIDGE.id } },
      lights: INVENTORY,
    })
    render(<AmbiancePage />)
    await tick()
    fireEvent.click(screen.getByText('Rechercher un pont'))
    await tick()
    let sawStep1 = false
    const observer = new MutationObserver(() => {
      if (document.querySelector('.ambiance-step-discover')) sawStep1 = true
    })
    observer.observe(document.body, { childList: true, subtree: true })
    fireEvent.click(screen.getByText('Associer ce pont'))
    await tick()
    await tick()
    observer.disconnect()
    expect(screen.getByText('Ampoules du pont')).toBeInTheDocument()
    expect(sawStep1).toBe(false)
  })

  it('429 busy (association déjà en cours) : traité comme « pas encore », relance dans 2 s, aucune alerte', async () => {
    let n = 0
    const server = await startPairing(makeServer({
      discover: { bridges: [BRIDGE] },
      register: () => (++n === 1
        ? { status: 429, body: { result: 'busy', reason: 'register_in_progress' } }
        : { status: 409, body: { result: 'refused', reason: 'link_button_not_pressed' } }),
    }))
    expect(screen.getByText('Appuyez sur le bouton rond au centre du pont')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).toBeNull()
    await tick(REGISTER_RETRY_MS)
    expect(callsTo(server, 'POST', '/api/lighting/register')).toHaveLength(2)
    expect(screen.getByText('Appuyez sur le bouton rond au centre du pont')).toBeInTheDocument()
  })

  it('400 bridge_ip_not_private : adresse hors réseau local — pas de Réessayer (changer l\'adresse), rien enregistré', async () => {
    const server = await startPairing(makeServer({
      discover: { bridges: [BRIDGE] },
      register: { status: 400, body: { result: 'error', reason: 'bridge_ip_not_private' } },
    }))
    expect(screen.getByRole('alert').textContent).toContain('Adresse hors du réseau local')
    expect(screen.queryByText('Réessayer')).toBeNull()
    expect(screen.getByText('Annuler')).toBeInTheDocument()
    await tick(10_000)
    expect(callsTo(server, 'POST', '/api/lighting/register')).toHaveLength(1)
    expect(callsTo(server, 'POST', '/config.json')).toHaveLength(0)
  })

  it('502 invalid_key_from_bridge : « Association impossible » avec texte fixe, Réessayer proposé', async () => {
    await startPairing(makeServer({
      discover: { bridges: [BRIDGE] },
      register: { status: 502, body: { result: 'error', reason: 'invalid_key_from_bridge' } },
    }))
    const alert = screen.getByRole('alert')
    expect(alert.textContent).toContain('Association impossible')
    expect(alert.textContent).toContain('clé inutilisable')
    expect(screen.getByText('Réessayer')).toBeInTheDocument()
  })

  it('500 {result:error} sans message : texte générique, jamais « undefined »', async () => {
    await startPairing(makeServer({
      discover: { bridges: [BRIDGE] },
      register: { status: 500, body: { result: 'error' } },
    }))
    const alert = screen.getByRole('alert')
    expect(alert.textContent).toContain('Association impossible')
    expect(alert.textContent).toContain('HTTP 500')
    expect(alert.textContent).not.toMatch(/undefined|null/)
  })

  it('409 api_key_refused (pas link_button_not_pressed) : ce n\'est PAS « pas encore » — arrêt, pas de relance', async () => {
    const server = await startPairing(makeServer({
      discover: { bridges: [BRIDGE] },
      register: { status: 409, body: { result: 'refused', reason: 'api_key_refused' } },
    }))
    expect(screen.getByRole('alert').textContent).toContain("refusé l'association")
    await tick(10_000)
    expect(callsTo(server, 'POST', '/api/lighting/register')).toHaveLength(1)
  })

  it('« Annuler » : retour à l\'étape 1, plus aucune relance', async () => {
    const server = await startPairing(makeServer({ discover: { bridges: [BRIDGE] } }))
    fireEvent.click(screen.getByText('Annuler'))
    await tick()

    expect(screen.queryByText(/Appuyez sur le bouton/)).toBeNull()
    expect(screen.getByText('Associer ce pont')).toBeInTheDocument()
    const before = callsTo(server, 'POST', '/api/lighting/register').length
    await tick(10_000)
    expect(callsTo(server, 'POST', '/api/lighting/register')).toHaveLength(before)
  })
})

// ===========================================================================
// Étape 3 — choisir et tester les ampoules
// ===========================================================================

describe('Étape 3 — pont configuré', () => {
  it('badge « Pont connecté » + compteur, toutes les ampoules AFFICHÉES et COCHÉES par défaut, « Tester » inactif pour une ampoule éteinte au mur', async () => {
    makeServer({ lighting: CONFIGURED, status: { state: 'ok', lights_ok: 2, lights_total: 3 }, lights: INVENTORY })
    render(<AmbiancePage />)

    await screen.findByText('Salle gauche')
    expect(badge().dataset.state).toBe('ok')
    expect(badge().textContent).toContain('Pont connecté')
    expect(badge().textContent).toContain('2/3 ampoules')
    expect(screen.queryByText('Rechercher un pont')).toBeNull()

    const boxes = screen.getAllByRole('checkbox')
    expect(boxes).toHaveLength(3)
    boxes.forEach(b => expect(b).toBeChecked())

    expect(screen.getByText('id 8 · joignable')).toBeInTheDocument()
    expect(screen.getByText('id 11 · éteinte au mur')).toBeInTheDocument()

    const rows = document.querySelectorAll('.ambiance-light')
    expect(within(rows[0]).getByText('Tester').closest('button')).not.toBeDisabled()
    expect(within(rows[2]).getByText('Tester').closest('button')).toBeDisabled()
    expect(screen.getByText(/bref flash puis rend l'ampoule/)).toBeInTheDocument()
  })

  it('aucun rendu intermédiaire avec des cases décochées (sélection dérivée, pas fixée par un effet)', async () => {
    makeServer({ lighting: CONFIGURED, status: { state: 'ok' }, lights: INVENTORY })
    const snapshots = []
    const observer = new MutationObserver(() => {
      const boxes = Array.from(document.querySelectorAll('input[type="checkbox"]'))
      if (boxes.length > 0) snapshots.push(boxes.map(b => b.checked))
    })
    observer.observe(document.body, { childList: true, subtree: true, attributes: true })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')
    await act(async () => { await new Promise(r => setTimeout(r, 20)) })
    observer.disconnect()
    expect(snapshots.length).toBeGreaterThan(0)
    snapshots.forEach(snap => snap.forEach(checked => expect(checked).toBe(true)))
  })

  it('inventaire en 500 {result:error} : message explicite, pas « Aucune ampoule », rien ne casse', async () => {
    makeServer({ lighting: CONFIGURED, status: { state: 'ok' }, lights: { status: 500, body: { result: 'error' } } })
    render(<AmbiancePage />)
    await screen.findByText('Lecture des ampoules impossible.')
    expect(screen.queryByText('Aucune ampoule sur ce pont.')).toBeNull()
    expect(screen.getByText('Actualiser la liste')).toBeInTheDocument()
  })

  it('« Enregistrer » persiste les noms cochés en role general, prévient la Navbar', async () => {
    const server = makeServer({ lighting: CONFIGURED, status: { state: 'ok' }, lights: INVENTORY })
    const onChanged = vi.fn()
    window.addEventListener(LIGHTING_CHANGED_EVENT, onChanged)
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    fireEvent.click(screen.getByLabelText('Scène')) // décoche
    fireEvent.click(screen.getByText('Enregistrer'))

    await screen.findByText('Ampoules enregistrées.')
    const saves = callsTo(server, 'POST', '/config.json')
    expect(saves).toHaveLength(1)
    // Seules les clés de cette étape : pont et clé restent intacts côté serveur.
    expect(saves[0].body.lighting).toEqual({
      enabled: true,
      lights: [
        { name: 'Salle gauche', role: 'general' },
        { name: 'Salle droite', role: 'general' },
      ],
    })
    expect(server.lighting.bridge_id).toBe(BRIDGE.id)
    expect(onChanged).toHaveBeenCalledTimes(1)
    window.removeEventListener(LIGHTING_CHANGED_EVENT, onChanged)
  })

  it('sélection existante : seules les ampoules configurées sont cochées ; une ampoule configurée absente du pont est signalée « introuvable », jamais remplacée', async () => {
    makeServer({
      lighting: { ...CONFIGURED, lights: [{ name: 'Salle gauche', role: 'general' }, { name: 'Disparue', role: 'general' }] },
      status: { state: 'ok' },
      lights: INVENTORY,
    })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    expect(screen.getByLabelText('Salle gauche')).toBeChecked()
    expect(screen.getByLabelText('Salle droite')).not.toBeChecked()
    expect(screen.getByLabelText('Scène')).not.toBeChecked()
    expect(screen.getByLabelText('Disparue')).toBeChecked()
    expect(screen.getByText(/introuvable sur le pont/)).toBeInTheDocument()
    expect(document.querySelector('.ambiance-light.is-missing')).not.toBeNull()
  })

  it('deux ampoules du même nom : refus explicite (ligne bloquée), pas de choix arbitraire', async () => {
    makeServer({
      lighting: CONFIGURED,
      status: { state: 'ok' },
      lights: { status: 200, body: { lights: [
        { id: '1', name: 'Jumelle', reachable: true },
        { id: '2', name: 'Jumelle', reachable: true },
      ] } },
    })
    render(<AmbiancePage />)
    await screen.findAllByText('Jumelle')
    const boxes = screen.getAllByRole('checkbox')
    boxes.forEach(b => expect(b).toBeDisabled())
    expect(screen.getAllByText(/nom en double/)).toHaveLength(2)
  })

  it('« Tester » une ampoule → POST /api/lighting/test {name} ; « Tester toutes » → {}', async () => {
    const server = makeServer({ lighting: CONFIGURED, status: { state: 'ok' }, lights: INVENTORY })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    const rows = document.querySelectorAll('.ambiance-light')
    fireEvent.click(within(rows[0]).getByText('Tester'))
    await waitFor(() => expect(callsTo(server, 'POST', '/api/lighting/test')).toHaveLength(1))
    expect(callsTo(server, 'POST', '/api/lighting/test')[0].body).toEqual({ name: 'Salle gauche' })

    fireEvent.click(screen.getByText('Tester toutes les ampoules'))
    await waitFor(() => expect(callsTo(server, 'POST', '/api/lighting/test')).toHaveLength(2))
    expect(callsTo(server, 'POST', '/api/lighting/test')[1].body).toEqual({})
  })

  it('« Dissocier ce pont » (confirmé) : clear_api_key + enabled:false, retour à l\'étape 1', async () => {
    const server = makeServer({ lighting: CONFIGURED, status: { state: 'ok' }, lights: INVENTORY })
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    fireEvent.click(screen.getByText('Dissocier ce pont'))

    await screen.findByText('Pont dissocié.')
    const saves = callsTo(server, 'POST', '/config.json')
    expect(saves).toHaveLength(1)
    expect(saves[0].body.lighting).toMatchObject({ enabled: false, clear_api_key: true, bridge_ip: '', lights: [] })
    await screen.findByText('Rechercher un pont')
    expect(badge().dataset.state).toBe('disabled')
  })

  it('« Dissocier ce pont » refusé dans la confirmation : rien n\'est envoyé', async () => {
    const server = makeServer({ lighting: CONFIGURED, status: { state: 'ok' }, lights: INVENTORY })
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')
    fireEvent.click(screen.getByText('Dissocier ce pont'))
    expect(callsTo(server, 'POST', '/config.json')).toHaveLength(0)
    expect(screen.getByText('Salle gauche')).toBeInTheDocument()
  })
})

// ===========================================================================
// Cas dégradés — « injoignable » et « refusée » ne sont JAMAIS fondus
// ===========================================================================

describe('Cas dégradés (maquette §07, contrat §5.6)', () => {
  it('pont débranché : badge « Pont injoignable », dernière sélection connue affichée et gelée, aucune perte', async () => {
    makeServer({
      lighting: { ...CONFIGURED, lights: [{ name: 'Salle gauche', role: 'general' }, { name: 'Salle droite', role: 'general' }] },
      status: { state: 'unreachable' },
      lights: { status: 503, body: { result: 'unreachable' } },
    })
    render(<AmbiancePage />)

    await screen.findByText('Salle gauche')
    expect(badge().dataset.state).toBe('unreachable')
    expect(badge().textContent).toContain('Pont injoignable')
    expect(badge().textContent).not.toContain('refusée')
    expect(screen.getByText(/La partie continue normalement/)).toBeInTheDocument()

    const list = document.querySelector('.ambiance-lights')
    expect(list.classList.contains('is-frozen')).toBe(true)
    screen.getAllByRole('checkbox').forEach(b => { expect(b).toBeDisabled(); expect(b).toBeChecked() })
    expect(screen.getByText('Enregistrer').closest('button')).toBeDisabled()
    expect(screen.queryByText('Ré-associer')).toBeNull()
  })

  it('clé révoquée : badge « Association refusée » + bouton « Ré-associer » qui relance l\'appui bouton sur le pont connu', async () => {
    const server = makeServer({
      lighting: CONFIGURED,
      status: { state: 'refused' },
      lights: { status: 401, body: { result: 'refused' } },
    })
    render(<AmbiancePage />)

    await screen.findByText('Ré-associer')
    expect(badge().dataset.state).toBe('refused')
    expect(badge().textContent).toContain('Association refusée')
    expect(badge().textContent).not.toContain('injoignable')

    fireEvent.click(screen.getByText('Ré-associer'))
    await screen.findByText('Appuyez sur le bouton rond au centre du pont')
    await waitFor(() => expect(callsTo(server, 'POST', '/api/lighting/register')).toHaveLength(1))
    expect(callsTo(server, 'POST', '/api/lighting/register')[0].body).toEqual({ bridge_ip: BRIDGE.ip })
  })

  it('les classes CSS du badge distinguent les quatre états', async () => {
    const states = ['ok', 'unreachable', 'refused']
    const seen = new Set()
    for (const state of states) {
      makeServer({ lighting: CONFIGURED, status: { state }, lights: INVENTORY })
      const { unmount } = render(<AmbiancePage />)
      await waitFor(() => expect(badge().dataset.state).toBe(state))
      seen.add(badge().className)
      unmount()
    }
    makeServer()
    render(<AmbiancePage />)
    await waitFor(() => expect(badge().dataset.state).toBe('disabled'))
    seen.add(badge().className)
    expect(seen.size).toBe(4)
  })

  it('serveur sans le module (config.json sans section lighting) : page réduite à « Rechercher un pont », rien ne casse', async () => {
    global.fetch = vi.fn(async (url) => {
      if (url === '/config.json') return { ok: true, status: 200, json: async () => ({ ai: {} }) }
      if (url === '/api/lighting/status') return { ok: false, status: 404, json: async () => ({}) }
      throw new Error(`inattendu ${url}`)
    })
    render(<AmbiancePage />)
    await screen.findByText('Rechercher un pont')
    expect(badge().dataset.state).toBe('disabled')
  })
})
