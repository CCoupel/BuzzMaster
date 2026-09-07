import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'

// ---------------------------------------------------------------------------
// AmbiancePage — Batch 3 (#213 + #208, milestone v10.0.0, reprise post-#207).
//
// #213 — colonne « Rôle » par ampoule (Éclairage général / Équipe X), même
// schéma role/team déjà figé par #207 (config.go). La liste d'équipes vient
// de l'état de jeu courant (useGame().teams), jamais d'un endpoint dédié.
//
// #208 — panneau « Éclairage général — conduite en direct » : sélecteur
// ON/AUTO/OFF + bascule Flash, scopés à la zone `general` uniquement.
// Contrat : contracts/lighting.md §10.1 (SHA df448318) — le mode TIENT
// indéfiniment (pas d'écrasement automatique), seul un retour manuel sur
// AUTO relâche. État lu depuis GET /api/lighting/status (mode + flash),
// jamais déduit côté client.
//
// Maquettes de référence : docs/mockups/lighting-team-assignment-213.html
// (rev6), docs/mockups/lighting-priority-208.md (rev3).
//
// Fichier séparé de AmbiancePage.test.jsx (convention du repo, cf.
// BackstagePage.entracte.test.jsx) : mock GameContext local, propre serveur
// simulé étendu avec /api/lighting/mode et /api/lighting/flash.
// ---------------------------------------------------------------------------

vi.mock('./AmbiancePage.css', () => ({}))
vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

import AmbiancePage from './AmbiancePage'
import { useGame } from '../hooks/GameContext'

const TEAMS = {
  Rouges: { COLOR: [255, 26, 26], COLOR_NAME: 'rouge', SCORE: 0 },
  Bleus: { COLOR: [26, 94, 255], COLOR_NAME: 'bleu', SCORE: 0 },
}

const BRIDGE = { ip: '192.168.1.101', id: '001788fffea0591e', model: 'BSB002' }
const CONFIGURED = { enabled: true, bridge_ip: BRIDGE.ip, bridge_id: BRIDGE.id, api_key_configured: true, lights: [] }
const INVENTORY = {
  status: 200,
  body: {
    lights: [
      { id: '8', name: 'Salle gauche', reachable: true, on: false },
      { id: '9', name: 'Salle droite', reachable: true, on: true },
    ],
  },
}

// Serveur simulé — même patron que AmbiancePage.test.jsx, étendu avec le
// mode/flash server-side (#208) : `mode`/`flash` sont un état MUTABLE porté
// par le serveur simulé, jamais déduit côté client, exactement comme le
// contrat l'exige (§10.1.1 pt.6).
function makeServer({ lighting = {}, statusExtra = {}, lights, mode = 'AUTO', flash = false } = {}) {
  const server = {
    keyStored: !!lighting.api_key_configured,
    lighting: { enabled: false, bridge_ip: '', bridge_id: '', lights: [], ...lighting },
    mode,
    flash,
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
      const { clear_api_key, api_key_configured, ...section } = body.lighting
      if (clear_api_key) server.keyStored = false
      server.lighting = { ...server.lighting, ...section }
      return respond(200, { ok: true })
    }
    if (method === 'GET' && url === '/api/lighting/status') {
      return respond(200, { state: 'ok', mode: server.mode, flash: server.flash, ...statusExtra })
    }
    if (method === 'GET' && url === '/api/lighting/lights') {
      const r = typeof lights === 'function' ? lights() : (lights ?? { status: 200, body: { lights: [] } })
      return respond(r.status, r.body)
    }
    if (method === 'POST' && url === '/api/lighting/test') {
      return respond(200, { result: 'ok' })
    }
    // #208 — POST /api/lighting/mode {"mode":"ON"|"AUTO"|"OFF"} (contrat §10.1).
    if (method === 'POST' && url === '/api/lighting/mode') {
      if (!['ON', 'AUTO', 'OFF'].includes(body?.mode)) return respond(400, { result: 'error' })
      server.mode = body.mode
      return respond(200, { result: 'ok', mode: server.mode })
    }
    // #208 — POST /api/lighting/flash {"on":true|false} (contrat §10.1.2).
    if (method === 'POST' && url === '/api/lighting/flash') {
      server.flash = !!body?.on
      return respond(200, { result: 'ok', flash: server.flash })
    }
    throw new Error(`Route non mockée : ${method} ${url}`)
  })

  return server
}

const callsTo = (server, method, url) => server.calls.filter(c => c.method === method && c.url === url)

beforeEach(() => {
  useGame.mockReturnValue({ teams: TEAMS })
})

afterEach(() => {
  vi.restoreAllMocks()
})

// ===========================================================================
// #213 — colonne d'affectation équipe → ampoule
// ===========================================================================

describe('AmbiancePage — #213 rôle par ampoule', () => {
  it('propose Éclairage général + une option par équipe de la partie courante', async () => {
    makeServer({ lighting: CONFIGURED, lights: INVENTORY })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    const rows = document.querySelectorAll('.ambiance-light')
    const select = within(rows[0]).getByLabelText('Rôle de Salle gauche')
    const optionLabels = Array.from(select.options).map(o => o.textContent)
    expect(optionLabels).toEqual(['Éclairage général', 'Équipe — Rouges', 'Équipe — Bleus'])
  })

  it('une équipe littéralement nommée "general" reste sélectionnable — pas de collision de value (revue code-reviewer)', async () => {
    // Sans le préfixe de value (roleSelectValue/TEAM_SELECT_PREFIX), les deux
    // options porteraient value="general" et onChange ne pourrait jamais
    // distinguer laquelle a été cliquée — pendant côté React du garde-fou
    // déjà posé côté backend (ambiance.go, "a team literally named general
    // must never shadow it").
    useGame.mockReturnValue({ teams: { general: { COLOR: [1, 2, 3], COLOR_NAME: 'rouge', SCORE: 0 } } })
    const server = makeServer({ lighting: CONFIGURED, lights: INVENTORY })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    const select = screen.getByLabelText('Rôle de Salle gauche')
    const values = Array.from(select.options).map(o => o.value)
    expect(values).toEqual(['general', 'team:general']) // distincts, jamais deux fois "general"

    fireEvent.change(select, { target: { value: 'team:general' } })
    fireEvent.click(screen.getByText('Enregistrer'))
    await screen.findByText('Ampoules enregistrées.')

    const saves = callsTo(server, 'POST', '/config.json')
    expect(saves[0].body.lighting.lights).toContainEqual({ name: 'Salle gauche', role: 'team', team: 'general' })
  })

  it('sans équipe configurée, seule « Éclairage général » est proposée', async () => {
    useGame.mockReturnValue({ teams: {} })
    makeServer({ lighting: CONFIGURED, lights: INVENTORY })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    const select = screen.getByLabelText('Rôle de Salle gauche')
    expect(Array.from(select.options).map(o => o.textContent)).toEqual(['Éclairage général'])
  })

  it('une ampoule déjà affectée à une équipe affiche cette équipe au chargement', async () => {
    makeServer({
      lighting: { ...CONFIGURED, lights: [{ name: 'Salle gauche', role: 'team', team: 'Rouges' }] },
      lights: INVENTORY,
    })
    render(<AmbiancePage />)
    // « Salle droite » n'existe QUE dans l'inventaire (jamais dans la config
    // de ce test) : l'attendre garantit que le second fetch (les ampoules du
    // pont) a bien résolu — contrairement à « Salle gauche », qui apparaît
    // déjà comme ligne « introuvable » dès la config seule chargée.
    await screen.findByText('Salle droite')

    expect(screen.getByLabelText('Rôle de Salle gauche').value).toBe('team:Rouges')
    expect(screen.getByLabelText('Rôle de Salle droite').value).toBe('general')
  })

  it('« Enregistrer » persiste le rôle équipe choisi, sans toucher aux ampoules non modifiées', async () => {
    const server = makeServer({ lighting: CONFIGURED, lights: INVENTORY })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    fireEvent.change(screen.getByLabelText('Rôle de Salle gauche'), { target: { value: 'team:Rouges' } })
    fireEvent.click(screen.getByText('Enregistrer'))

    await screen.findByText('Ampoules enregistrées.')
    const saves = callsTo(server, 'POST', '/config.json')
    expect(saves).toHaveLength(1)
    expect(saves[0].body.lighting.lights).toEqual([
      { name: 'Salle gauche', role: 'team', team: 'Rouges' },
      { name: 'Salle droite', role: 'general' },
    ])
  })

  it('repasser une ampoule d\'équipe en « Éclairage général » retire team au moment de l\'enregistrement', async () => {
    const server = makeServer({
      lighting: { ...CONFIGURED, lights: [{ name: 'Salle gauche', role: 'team', team: 'Rouges' }] },
      lights: INVENTORY,
    })
    render(<AmbiancePage />)
    await screen.findByText('Salle droite') // même précaution que le test précédent

    fireEvent.change(screen.getByLabelText('Rôle de Salle gauche'), { target: { value: 'general' } })
    fireEvent.click(screen.getByText('Enregistrer'))

    await screen.findByText('Ampoules enregistrées.')
    const saves = callsTo(server, 'POST', '/config.json')
    expect(saves[0].body.lighting.lights[0]).toEqual({ name: 'Salle gauche', role: 'general' })
  })

  it('le menu de rôle est figé quand le pont est injoignable (même garde que la case à cocher)', async () => {
    makeServer({ lighting: CONFIGURED, statusExtra: { state: 'unreachable' }, lights: { status: 503, body: { result: 'unreachable' } } })
    render(<AmbiancePage />)
    await screen.findByText('Pont injoignable.')
    // Aucune ampoule affichée sans sélection préalable côté config : rien à
    // vérifier de plus ici que l'absence de crash — couvert par les tests
    // de dégradation existants (AmbiancePage.test.jsx).
  })
})

// ===========================================================================
// #208 — conduite en direct : sélecteur ON/AUTO/OFF + Flash
// ===========================================================================

describe('AmbiancePage — #208 conduite en direct ON/AUTO/OFF + Flash', () => {
  it('affiche la position AUTO par défaut, sans bandeau d\'avertissement', async () => {
    makeServer({ lighting: CONFIGURED, lights: INVENTORY, mode: 'AUTO' })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    const auto = screen.getByRole('radio', { name: 'AUTO' })
    expect(auto).toHaveAttribute('aria-checked', 'true')
    expect(auto).toBeDisabled() // reclic sur la position déjà active : sans effet
    expect(screen.getByRole('radio', { name: 'ON' })).not.toBeDisabled()
    expect(screen.getByRole('radio', { name: 'OFF' })).not.toBeDisabled()
    expect(screen.queryByText(/engagé/)).toBeNull()
  })

  it('cliquer OFF appelle POST /api/lighting/mode {mode:"OFF"} et met à jour le sélecteur affiché', async () => {
    const server = makeServer({ lighting: CONFIGURED, lights: INVENTORY, mode: 'AUTO' })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    fireEvent.click(screen.getByRole('radio', { name: 'OFF' }))

    await waitFor(() => expect(screen.getByRole('radio', { name: 'OFF' })).toHaveAttribute('aria-checked', 'true'))
    const calls = callsTo(server, 'POST', '/api/lighting/mode')
    expect(calls).toHaveLength(1)
    expect(calls[0].body).toEqual({ mode: 'OFF' })
    // La position affichée vient du GET /api/lighting/status relu après le
    // POST — pas d'état optimiste côté client (contrat §10.1.1 pt.6).
    expect(callsTo(server, 'GET', '/api/lighting/status').length).toBeGreaterThanOrEqual(2)
  })

  it('le mode ON/OFF affiche un bandeau d\'avertissement permanent (garde-fou contre l\'oubli)', async () => {
    makeServer({ lighting: CONFIGURED, lights: INVENTORY, mode: 'OFF' })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    expect(screen.getByRole('radio', { name: 'OFF' })).toHaveAttribute('aria-checked', 'true')
    // Le texte est scindé entre plusieurs nœuds (le mode est dans un
    // <strong>) : comparer le textContent complet plutôt qu'un getByText
    // (qui ne joint jamais le texte des enfants — piège documenté de RTL).
    const warning = document.querySelector('.ambiance-mode-warning')
    expect(warning).not.toBeNull()
    expect(warning.textContent).toMatch(/Mode\s*OFF\s*engagé/)
    expect(warning.textContent).toMatch(/restera\s*éteint indéfiniment/)
  })

  it('cliquer la position déjà active n\'émet aucune requête', async () => {
    const server = makeServer({ lighting: CONFIGURED, lights: INVENTORY, mode: 'ON' })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    // Le bouton ON est `disabled` (déjà actif) : un clic ne doit rien déclencher.
    fireEvent.click(screen.getByRole('radio', { name: 'ON' }))
    expect(callsTo(server, 'POST', '/api/lighting/mode')).toHaveLength(0)
  })

  it('Flash est une bascule séparée : l\'activer puis le désactiver n\'affecte jamais le sélecteur', async () => {
    const server = makeServer({ lighting: CONFIGURED, lights: INVENTORY, mode: 'OFF', flash: false })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    const flashBtn = screen.getByRole('button', { name: /Flash/ })
    expect(flashBtn).toHaveAttribute('aria-pressed', 'false')

    fireEvent.click(flashBtn)
    await waitFor(() => expect(screen.getByRole('button', { name: /Flash/ })).toHaveAttribute('aria-pressed', 'true'))
    expect(callsTo(server, 'POST', '/api/lighting/flash')[0].body).toEqual({ on: true })
    // Le sélecteur reste sur OFF — Flash ne le déplace jamais (contrat §10.1.2).
    expect(screen.getByRole('radio', { name: 'OFF' })).toHaveAttribute('aria-checked', 'true')

    fireEvent.click(screen.getByRole('button', { name: /Flash/ }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Flash/ })).toHaveAttribute('aria-pressed', 'false'))
    expect(callsTo(server, 'POST', '/api/lighting/flash')[1].body).toEqual({ on: false })
  })

  it('une erreur serveur sur /api/lighting/mode affiche un toast, sans faire bouger le sélecteur', async () => {
    makeServer({ lighting: CONFIGURED, lights: INVENTORY, mode: 'AUTO' })
    global.fetch = vi.fn(async (url, opts = {}) => {
      if (url === '/api/lighting/mode') return { ok: false, status: 500, text: async () => 'boom' }
      if (url === '/api/lighting/status') return { ok: true, status: 200, json: async () => ({ state: 'ok', mode: 'AUTO', flash: false }) }
      if (url === '/config.json') return { ok: true, status: 200, json: async () => ({ lighting: CONFIGURED }) }
      if (url === '/api/lighting/lights') return { ok: true, status: 200, json: async () => INVENTORY.body }
      throw new Error(`Route non mockée : ${url}`)
    })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    fireEvent.click(screen.getByRole('radio', { name: 'OFF' }))
    await screen.findByText(/Erreur/)
    expect(screen.getByRole('radio', { name: 'AUTO' })).toHaveAttribute('aria-checked', 'true')
  })
})
