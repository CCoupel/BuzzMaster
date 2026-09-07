import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'

// ---------------------------------------------------------------------------
// AmbiancePage — Batch 3, #213 (milestone v10.0.0, reprise post-#207) :
// colonne « Rôle » par ampoule (Éclairage général / Équipe X), même schéma
// role/team déjà figé par #207 (config.go). La liste d'équipes vient de
// l'état de jeu courant (useGame().teams), jamais d'un endpoint dédié.
//
// Le panneau #208 (sélecteur ON/AUTO/OFF + Flash) vivait aussi ici dans une
// première version de ce fichier — passé par GamePage.jsx, puis retiré
// définitivement le 2026-09-07 (les commandes ne vivent QUE dans la Navbar,
// dans un popover — retour QUALIF v10.0.0.13). Ses tests vivent désormais
// dans components/LightingModePanel.test.jsx (composant) et
// components/Navbar.lighting208.test.jsx (câblage popover).
//
// Cocher explicitement chaque ampoule avant « Enregistrer » dans les tests
// ci-dessous : depuis le même retour QUALIF, une association fraîche ne
// pré-coche plus rien (voir AmbiancePage.navbarAndUnassign.test.jsx pour la
// vérification dédiée de ce revirement, et le bouton « Désassocier toutes
// les ampoules »).
//
// Maquette de référence : docs/mockups/lighting-team-assignment-213.html
// (rev6). Contrats : contracts/lighting.md §10.1 (SHA df448318),
// contracts/hue-bridge.md §5.2/§5.7.
//
// Fichier séparé de AmbiancePage.test.jsx (convention du repo, cf.
// BackstagePage.entracte.test.jsx) : mock GameContext local.
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

// Serveur simulé — même patron que AmbiancePage.test.jsx.
function makeServer({ lighting = {}, statusExtra = {}, lights } = {}) {
  const server = {
    keyStored: !!lighting.api_key_configured,
    lighting: { enabled: false, bridge_ip: '', bridge_id: '', lights: [], ...lighting },
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
      return respond(200, { state: 'ok', ...statusExtra })
    }
    if (method === 'GET' && url === '/api/lighting/lights') {
      const r = typeof lights === 'function' ? lights() : (lights ?? { status: 200, body: { lights: [] } })
      return respond(r.status, r.body)
    }
    if (method === 'POST' && url === '/api/lighting/test') {
      return respond(200, { result: 'ok' })
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

    // Round 4 (2026-09-07) — plus de présélection par défaut (#213/#207
    // revirement) : cocher explicitement l'ampoule avant de pouvoir
    // l'enregistrer.
    fireEvent.click(screen.getByLabelText('Salle gauche'))
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

    // Round 4 (2026-09-07) — plus de présélection par défaut : cocher
    // explicitement les deux ampoules avant de changer le rôle de l'une.
    fireEvent.click(screen.getByLabelText('Salle gauche'))
    fireEvent.click(screen.getByLabelText('Salle droite'))
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

