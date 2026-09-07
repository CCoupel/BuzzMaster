import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'

// ---------------------------------------------------------------------------
// AmbiancePage — retours QUALIF v10.0.0.13 (2026-09-07), points 2 et 3 de
// task-dev-frontend-navbar-and-unassign-20260907.md (le point 1, panneau
// #208 dans la Navbar, est couvert par components/Navbar.lighting208.test.jsx
// et components/LightingModePanel.test.jsx — inchangé, seul son point de
// montage a bougé).
//
// Point 2 — plus d'affectation par défaut : une association fraîche (config
// vide) ne pré-coche plus aucune ampoule. Revirement de #207
// (defaultSelectionFor, AmbiancePage.jsx) — déjà couvert dans le détail par
// AmbiancePage.test.jsx (étape 3, tests mis à jour) ; ce fichier ajoute une
// vérification dédiée et sans ambiguïté.
//
// Point 3 — bouton « Désassocier toutes les ampoules » (rôle/équipe,
// groupé) : distinct de « Dissocier ce pont » (qui efface la clé et le
// pont entier) — celui-ci ne touche que les rôles, les ampoules restent
// détectées et pilotées.
// ---------------------------------------------------------------------------

vi.mock('./AmbiancePage.css', () => ({}))
vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

import AmbiancePage from './AmbiancePage'
import { useGame } from '../hooks/GameContext'

const TEAMS = { Rouges: { COLOR: [255, 26, 26], COLOR_NAME: 'rouge', SCORE: 0 } }
const BRIDGE = { ip: '192.168.1.101', id: '001788fffea0591e' }
const CONFIGURED = { enabled: true, bridge_ip: BRIDGE.ip, bridge_id: BRIDGE.id, api_key_configured: true, lights: [] }
const INVENTORY = {
  status: 200,
  body: {
    lights: [
      { id: '8', name: 'Salle gauche', reachable: true },
      { id: '9', name: 'Salle droite', reachable: true },
    ],
  },
}

function makeServer({ lighting, lights }) {
  const server = { lighting: { ...lighting }, calls: [] }
  const respond = (status, body = {}) => ({
    ok: status < 400, status, json: async () => body, text: async () => JSON.stringify(body),
  })

  global.fetch = vi.fn(async (url, opts = {}) => {
    const method = opts.method || 'GET'
    const body = opts.body ? JSON.parse(opts.body) : null
    server.calls.push({ method, url, body })

    if (method === 'GET' && url === '/config.json') return respond(200, { lighting: server.lighting })
    if (method === 'POST' && url === '/config.json') {
      const { clear_api_key, api_key_configured, ...section } = body.lighting
      server.lighting = { ...server.lighting, ...section }
      return respond(200, { ok: true })
    }
    if (method === 'GET' && url === '/api/lighting/status') return respond(200, { state: 'ok' })
    if (method === 'GET' && url === '/api/lighting/lights') return respond(lights.status, lights.body)
    if (method === 'POST' && url === '/api/lighting/test') return respond(200, { result: 'ok' })
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

describe('AmbiancePage — point 2 : plus d\'affectation par défaut à l\'association', () => {
  it('association fraîche (config vide) : les ampoules détectées démarrent NON cochées, rôle "Éclairage général" affiché sans être enregistré', async () => {
    makeServer({ lighting: CONFIGURED, lights: INVENTORY })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    expect(screen.getByLabelText('Salle gauche')).not.toBeChecked()
    expect(screen.getByLabelText('Salle droite')).not.toBeChecked()
    // Le sélecteur de rôle affiche la valeur de repli (aucune valeur
    // enregistrée) — ne pas confondre "valeur affichée" et "affectation
    // persistée" : rien n'est envoyé au serveur tant que la case n'est pas
    // cochée ET « Enregistrer » cliqué.
    expect(screen.getByLabelText('Rôle de Salle gauche').value).toBe('general')
  })

  it('cocher puis Enregistrer ne persiste QUE les ampoules explicitement cochées', async () => {
    const server = makeServer({ lighting: CONFIGURED, lights: INVENTORY })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    fireEvent.click(screen.getByLabelText('Salle gauche')) // Salle droite reste décochée
    fireEvent.click(screen.getByText('Enregistrer'))
    await screen.findByText('Ampoules enregistrées.')

    const saves = callsTo(server, 'POST', '/config.json')
    expect(saves[0].body.lighting.lights).toEqual([{ name: 'Salle gauche', role: 'general' }])
  })
})

describe('AmbiancePage — point 3 : « Désassocier toutes les ampoules » (groupé)', () => {
  const unassignBtn = () => screen.getByText('Désassocier toutes les ampoules').closest('button')

  it('désactivé quand aucune ampoule sélectionnée ne porte de rôle équipe', async () => {
    makeServer({ lighting: CONFIGURED, lights: INVENTORY })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    expect(unassignBtn()).toBeDisabled() // rien de coché, a fortiori aucune équipe

    fireEvent.click(screen.getByLabelText('Salle gauche')) // cochée mais rôle general
    expect(unassignBtn()).toBeDisabled()
  })

  it('activé dès qu\'une ampoule sélectionnée porte un rôle équipe ; distinct de « Dissocier ce pont »', async () => {
    makeServer({ lighting: CONFIGURED, lights: INVENTORY })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    fireEvent.click(screen.getByLabelText('Salle gauche'))
    fireEvent.change(screen.getByLabelText('Rôle de Salle gauche'), { target: { value: 'team:Rouges' } })

    expect(unassignBtn()).not.toBeDisabled()
    expect(screen.getByText('Dissocier ce pont')).toBeInTheDocument() // toujours présent, inchangé
    expect(unassignBtn()).not.toBe(screen.getByText('Dissocier ce pont').closest('button'))
  })

  it('confirmation refusée : aucun POST /config.json envoyé', async () => {
    const server = makeServer({ lighting: CONFIGURED, lights: INVENTORY })
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    fireEvent.click(screen.getByLabelText('Salle gauche'))
    fireEvent.change(screen.getByLabelText('Rôle de Salle gauche'), { target: { value: 'team:Rouges' } })
    fireEvent.click(unassignBtn())

    expect(window.confirm).toHaveBeenCalledTimes(1)
    expect(callsTo(server, 'POST', '/config.json')).toHaveLength(0)
  })

  it('confirmé : réinitialise le rôle de TOUTES les ampoules sélectionnées à "general" en un seul POST, ampoules toujours pilotées', async () => {
    const server = makeServer({ lighting: CONFIGURED, lights: INVENTORY })
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    // Deux ampoules cochées, une seule affectée à une équipe — la
    // désassociation groupée doit quand même renvoyer LES DEUX (elles
    // restent pilotées), toutes deux en rôle "general".
    fireEvent.click(screen.getByLabelText('Salle gauche'))
    fireEvent.click(screen.getByLabelText('Salle droite'))
    fireEvent.change(screen.getByLabelText('Rôle de Salle gauche'), { target: { value: 'team:Rouges' } })

    fireEvent.click(unassignBtn())
    await screen.findByText('Affectations retirées.')

    const saves = callsTo(server, 'POST', '/config.json')
    expect(saves).toHaveLength(1)
    expect(saves[0].body.lighting.lights).toEqual([
      { name: 'Salle gauche', role: 'general' },
      { name: 'Salle droite', role: 'general' },
    ])
    // Après rechargement de la config, le sélecteur reflète bien "general".
    expect(screen.getByLabelText('Rôle de Salle gauche').value).toBe('general')
    expect(screen.getByLabelText('Salle gauche')).toBeChecked() // toujours pilotée, pas décochée
  })
})
