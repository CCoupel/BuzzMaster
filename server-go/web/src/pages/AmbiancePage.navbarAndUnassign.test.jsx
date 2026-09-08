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
// Point 3 — REMISE À L'ÉTAT LIBRE (rôle/équipe, groupé), 3 boutons à
// portées disjointes. Corrigé le même jour (précision utilisateur sur
// SHA b35fddbf) : le modèle a TROIS états — Libre (absente de
// `lighting.lights[]`), Général (`role:"general"`), Équipe
// (`role:"team"`) — pas deux. Les boutons remettent à LIBRE (retirent
// l'entrée), jamais à "general" (ce serait encore un rôle explicite).
// Distincts de « Dissocier ce pont » (qui efface la clé et le pont
// entier) — ceux-ci ne touchent que les rôles.
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
// Config déjà PERSISTÉE avec les deux rôles réels (pas construite par des
// clics dans le test) — reflète l'état "au rechargement de la page", la
// seule source de vérité que les boutons "Libre" doivent consulter (revue
// code-reviewer MAJEUR, currentlyAssignedNames dans AmbiancePage.jsx).
const CONFIGURED_BOTH_ROLES = {
  ...CONFIGURED,
  lights: [
    { name: 'Salle gauche', role: 'team', team: 'Rouges' },
    { name: 'Salle droite', role: 'general' },
  ],
}
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
    // P2b (2026-09-08) — allumage/extinction immédiat au coché/décoché.
    if (method === 'POST' && url === '/api/lighting/preview') return respond(200, { result: 'ok' })
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

describe('AmbiancePage — point 3 : 3 boutons « Remettre à Libre » à portées disjointes', () => {
  const allBtn = () => screen.getByText('Remettre à Libre toutes les ampoules').closest('button')
  const generalBtn = () => screen.getByText('Remettre à Libre les ampoules de rôle Général').closest('button')
  const teamBtn = () => screen.getByText('Remettre à Libre les ampoules de rôle Équipe').closest('button')

  // Config déjà PERSISTÉE (CONFIGURED_BOTH_ROLES) : les deux ampoules sont
  // cochées par défaut ET portent chacune un rôle réel — rien à cliquer.
  // Revue code-reviewer (MAJEUR) : une ampoule seulement COCHÉE, sans rôle
  // jamais enregistré ni modifié cette session, n'est PLUS considérée
  // "assignée" par les boutons Libre (voir le describe de non-régression
  // dédié plus bas) — ce scénario de test ne peut donc plus se construire
  // par de simples clics sur les cases, il lui faut une config persistée
  // (ou un changement explicite de rôle, voir le test juste après).
  async function renderWithBothRoles() {
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')
  }

  it('les 3 boutons sont désactivés tant que rien n\'est assigné (config vide, rien coché)', async () => {
    makeServer({ lighting: CONFIGURED, lights: INVENTORY })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    expect(allBtn()).toBeDisabled()
    expect(generalBtn()).toBeDisabled()
    expect(teamBtn()).toBeDisabled()
  })

  it('cocher une ampoule SANS lui choisir de rôle ne l\'assigne à rien : les 3 boutons restent désactivés', async () => {
    // Non-régression directe du MAJEUR : avant le fix, cocher suffisait à
    // rendre "Général" cliquable (la case ET le rôle de repli visuel
    // suffisaient) — désormais il faut un choix explicite (ou une config
    // déjà persistée).
    makeServer({ lighting: CONFIGURED, lights: INVENTORY })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    fireEvent.click(screen.getByLabelText('Salle gauche')) // cochée, rôle jamais choisi
    // Les 3 boutons sont désactivés : rien n'est réellement "assigné"
    // (ni persisté, ni modifié cette session) — une case cochée seule ne
    // suffit plus, y compris pour "Toutes" (même correction de fond).
    expect(allBtn()).toBeDisabled()
    expect(generalBtn()).toBeDisabled()
    expect(teamBtn()).toBeDisabled()
  })

  it('« Général » s\'active dès qu\'un rôle "général" est explicitement choisi (même sans jamais enregistrer)', async () => {
    makeServer({ lighting: CONFIGURED, lights: INVENTORY })
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    fireEvent.click(screen.getByLabelText('Salle gauche'))
    // Choix EXPLICITE, même si "Éclairage général" était déjà la valeur
    // affichée par défaut — c'est ce choix qui la fait compter, pas la case.
    fireEvent.change(screen.getByLabelText('Rôle de Salle gauche'), { target: { value: 'general' } })
    expect(generalBtn()).not.toBeDisabled()
    expect(teamBtn()).toBeDisabled()
  })

  it('confirmation refusée sur n\'importe lequel des 3 boutons : aucun POST /config.json envoyé', async () => {
    const server = makeServer({ lighting: CONFIGURED_BOTH_ROLES, lights: INVENTORY })
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    await renderWithBothRoles()

    fireEvent.click(teamBtn())
    expect(window.confirm).toHaveBeenCalledTimes(1)
    expect(callsTo(server, 'POST', '/config.json')).toHaveLength(0)
  })

  it('« Équipe » confirmé : retire UNIQUEMENT l\'ampoule d\'équipe (devient libre), l\'ampoule générale garde EXACTEMENT son rôle', async () => {
    const server = makeServer({ lighting: CONFIGURED_BOTH_ROLES, lights: INVENTORY })
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    await renderWithBothRoles()

    fireEvent.click(teamBtn())
    await screen.findByText('Ampoules remises à l\'état libre.')

    const saves = callsTo(server, 'POST', '/config.json')
    expect(saves).toHaveLength(1)
    // "Salle gauche" (équipe) a disparu du tableau envoyé = libre. "Salle
    // droite" (général) est réenvoyée à l'IDENTIQUE, jamais réécrite en
    // "general" par accident si elle avait porté un autre rôle.
    expect(saves[0].body.lighting.lights).toEqual([{ name: 'Salle droite', role: 'general' }])
  })

  it('« Général » confirmé : retire UNIQUEMENT l\'ampoule générale (devient libre), l\'ampoule d\'équipe n\'est pas touchée', async () => {
    const server = makeServer({ lighting: CONFIGURED_BOTH_ROLES, lights: INVENTORY })
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    await renderWithBothRoles()

    fireEvent.click(generalBtn())
    await screen.findByText('Ampoules remises à l\'état libre.')

    const saves = callsTo(server, 'POST', '/config.json')
    expect(saves[0].body.lighting.lights).toEqual([{ name: 'Salle gauche', role: 'team', team: 'Rouges' }])
  })

  it('« Toutes » confirmé : aucune entrée envoyée, quelle que soit la catégorie de chacune — LIBRE, jamais "general"', async () => {
    const server = makeServer({ lighting: CONFIGURED_BOTH_ROLES, lights: INVENTORY })
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    await renderWithBothRoles()

    fireEvent.click(allBtn())
    await screen.findByText('Ampoules remises à l\'état libre.')

    const saves = callsTo(server, 'POST', '/config.json')
    expect(saves[0].body.lighting.lights).toEqual([]) // pas [{role:"general"}, ...] — LIBRE = absent
    // Après rechargement, les deux redeviennent décochées (libres), pas
    // cochées avec un rôle "general" résiduel.
    expect(screen.getByLabelText('Salle gauche')).not.toBeChecked()
    expect(screen.getByLabelText('Salle droite')).not.toBeChecked()
  })

  it('distinct de « Dissocier ce pont », toujours présent et inchangé', async () => {
    makeServer({ lighting: CONFIGURED_BOTH_ROLES, lights: INVENTORY })
    await renderWithBothRoles()

    const dissocier = screen.getByText('Dissocier ce pont').closest('button')
    expect(dissocier).not.toBe(allBtn())
    expect(dissocier).not.toBe(generalBtn())
    expect(dissocier).not.toBe(teamBtn())
  })
})

// ---------------------------------------------------------------------------
// Revue code-reviewer (rapport code-reviewer-v10-round5-20260907-171136.md,
// MAJEUR) : les boutons "Général"/"Équipe" filtraient à partir
// d'`effectiveSelected` (état des cases à cocher) au lieu de
// `lighting.lights` — une ampoule décochée pour une raison SANS RAPPORT
// avec ces boutons disparaissait silencieusement de la config au clic,
// même si son rôle ne correspondait pas à la portée annoncée. Corrigé en
// itérant sur `currentlyAssignedNames` (config persistée UNION
// roleOverrides de la session), jamais sur les cases à cocher.
// ---------------------------------------------------------------------------

describe('AmbiancePage — non-régression MAJEUR (code-reviewer) : portée indépendante des cases à cocher', () => {
  const generalBtn = () => screen.getByText('Remettre à Libre les ampoules de rôle Général').closest('button')
  const teamBtn = () => screen.getByText('Remettre à Libre les ampoules de rôle Équipe').closest('button')

  // Réutilise CONFIGURED_BOTH_ROLES (module-level) — config déjà PERSISTÉE
  // reproduisant fidèlement l'état "au rechargement de la page" du scénario
  // du rapport, plutôt qu'un état construit ampoule par ampoule dans la
  // même session.

  it('décocher l\'ampoule d\'ÉQUIPE (sans rapport avec le bouton) puis cliquer "Général" ne l\'efface PAS', async () => {
    const server = makeServer({ lighting: CONFIGURED_BOTH_ROLES, lights: INVENTORY })
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')
    // Les deux sont cochées par défaut (config déjà persistée). On décoche
    // "Salle gauche" (équipe) pour une raison quelconque — AVANT de cliquer
    // un bouton "Libre" qui ne la concerne pas.
    fireEvent.click(screen.getByLabelText('Salle gauche'))
    expect(screen.getByLabelText('Salle gauche')).not.toBeChecked()

    fireEvent.click(generalBtn())
    await screen.findByText('Ampoules remises à l\'état libre.')

    const saves = callsTo(server, 'POST', '/config.json')
    // "Salle droite" (général) a bien disparu (portée du bouton). "Salle
    // gauche" (équipe) DOIT survivre malgré sa case décochée — le texte de
    // confirmation promet explicitement qu'elle n'est pas concernée.
    expect(saves[0].body.lighting.lights).toEqual([{ name: 'Salle gauche', role: 'team', team: 'Rouges' }])
  })

  it('décocher l\'ampoule GÉNÉRALE (sans rapport avec le bouton) puis cliquer "Équipe" ne l\'efface PAS', async () => {
    const server = makeServer({ lighting: CONFIGURED_BOTH_ROLES, lights: INVENTORY })
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')
    fireEvent.click(screen.getByLabelText('Salle droite'))
    expect(screen.getByLabelText('Salle droite')).not.toBeChecked()

    fireEvent.click(teamBtn())
    await screen.findByText('Ampoules remises à l\'état libre.')

    const saves = callsTo(server, 'POST', '/config.json')
    expect(saves[0].body.lighting.lights).toEqual([{ name: 'Salle droite', role: 'general' }])
  })

  it('une ampoule fraîchement cochée SANS rôle jamais enregistré n\'est écrite par AUCUN des boutons "Libre"', async () => {
    // Config vide : "Salle gauche" n'a ni entrée persistée ni roleOverride
    // — juste cochée. roleFor() la traite par défaut comme "general" pour
    // l'AFFICHAGE (repli visuel), mais elle ne doit pas être écrite par un
    // clic sur "Général" : elle n'a jamais eu de rôle réel à "libérer".
    const server = makeServer({ lighting: CONFIGURED, lights: INVENTORY })
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<AmbiancePage />)
    await screen.findByText('Salle gauche')

    fireEvent.click(screen.getByLabelText('Salle gauche')) // cochée, jamais de rôle explicite
    expect(generalBtn()).toBeDisabled() // rien n'est réellement "assigné" pour ce bouton
    expect(teamBtn()).toBeDisabled()
  })
})
