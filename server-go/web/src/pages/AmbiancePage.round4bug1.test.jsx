import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'

// ---------------------------------------------------------------------------
// AmbiancePage — reprise v10.0.0, round 4, Bug 1 : "Enregistrer" pouvait
// sauvegarder une liste d'ampoules VIDE.
//
// Cause confirmée (dev-backend, vérification concrète serveur réel + relec-
// ture disque) : le backend persiste fidèlement ce qu'on lui envoie — le
// vrai bug était frontend. `AmbiancePage.jsx` ne désactivait « Enregistrer »
// que sur `frozen` (pont injoignable/refusé), jamais sur une réponse
// GET /api/lighting/lights simplement VIDE sans erreur — un cas observé en
// pratique juste après association (29 ampoules réelles sur le pont), cause
// exacte encore incertaine côté timing serveur (driver Hue nouvellement
// construit ?), documentée dans le handoff de la tâche mais non
// reproductible ici sans matériel Hue réel.
//
// Ce fichier vérifie deux filets de sécurité INDÉPENDANTS, chacun testé
// isolément :
//   1. "Enregistrer" reste désactivé tant que la liste affichée est vide
//      (rows.length === 0), quelle que soit la cause.
//   2. Auto-guérison : un inventaire vide SANS erreur déclenche un unique
//      nouvel essai après 1,5 s (jamais en boucle) — si le second essai
//      retourne la vraie liste, "Enregistrer" se réactive sans action de
//      l'utilisateur. Vérifié via de vrais timers simulés (vi.useFakeTimers
//      + vi.advanceTimersByTimeAsync, même patron que AmbiancePage.test.jsx
//      §Étape 2), pas seulement par lecture de code.
//
// Aucun accès à du matériel Hue réel dans cet environnement (règle projet
// feedback_manual_qa_is_user_role.md) : la preuve de non-régression ici est
// un test d'intégration automatisé qui exerce le VRAI minuteur de
// l'auto-essai, la vérification finale sur pont physique reste à
// l'utilisateur.
// ---------------------------------------------------------------------------

vi.mock('./AmbiancePage.css', () => ({}))
vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
  GameProvider: ({ children }) => children,
}))

import AmbiancePage from './AmbiancePage'
import { useGame } from '../hooks/GameContext'

const BRIDGE = { ip: '192.168.1.101', id: '001788fffea0591e' }
const CONFIGURED_WITH_LIGHT = {
  enabled: true,
  bridge_ip: BRIDGE.ip,
  bridge_id: BRIDGE.id,
  api_key_configured: true,
  lights: [{ name: 'BuzzHue1', role: 'general' }],
}
const CONFIGURED_FRESH = {
  enabled: true,
  bridge_ip: BRIDGE.ip,
  bridge_id: BRIDGE.id,
  api_key_configured: true,
  lights: [],
}

// Serveur simulé — `lightsSequence` fournit une réponse DIFFÉRENTE à chaque
// appel de GET /api/lighting/lights (1er = vide, 2e = peuplé), pour
// reproduire fidèlement le scénario "vide puis se peuple".
function makeServer({ lighting, lightsSequence }) {
  const server = { lighting: { ...lighting }, calls: [], lightsCallCount: 0 }
  const respond = (status, body = {}) => ({
    ok: status < 400, status, json: async () => body, text: async () => JSON.stringify(body),
  })

  global.fetch = vi.fn(async (url, opts = {}) => {
    const method = opts.method || 'GET'
    const body = opts.body ? JSON.parse(opts.body) : null
    server.calls.push({ method, url, body })

    if (method === 'GET' && url === '/config.json') {
      return respond(200, { lighting: server.lighting })
    }
    if (method === 'POST' && url === '/config.json') {
      const { clear_api_key, api_key_configured, ...section } = body.lighting
      server.lighting = { ...server.lighting, ...section }
      return respond(200, { ok: true })
    }
    if (method === 'GET' && url === '/api/lighting/status') {
      return respond(200, { state: 'ok' })
    }
    if (method === 'GET' && url === '/api/lighting/lights') {
      const idx = Math.min(server.lightsCallCount, lightsSequence.length - 1)
      server.lightsCallCount += 1
      return respond(200, { lights: lightsSequence[idx] })
    }
    if (method === 'POST' && url === '/api/lighting/test') {
      return respond(200, { result: 'ok' })
    }
    throw new Error(`Route non mockée : ${method} ${url}`)
  })

  return server
}

const callsTo = (server, method, url) => server.calls.filter(c => c.method === method && c.url === url)
const enregistrerBtn = () => screen.getByText('Enregistrer').closest('button')
const tick = async (ms = 0) => { await act(async () => { await vi.advanceTimersByTimeAsync(ms) }) }

beforeEach(() => {
  useGame.mockReturnValue({ teams: {} })
  vi.useFakeTimers()
})

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('AmbiancePage — Bug 1 round 4 : liste vide sans erreur', () => {
  it('« Enregistrer » est désactivé quand le pont ne confirme AUCUNE des ampoules déjà configurées (allMissing)', async () => {
    // config non vide (BuzzHue1 déjà enregistré) + inventaire vide : toutes
    // les lignes affichées sont des placeholders "introuvable" — rows.length
    // > 0, mais aucune ne vient de l'inventaire réel. C'est exactement le
    // cas que `rows.length === 0` seul aurait manqué (une config déjà
    // existante ne produit jamais des `rows` vides, seulement "missing").
    makeServer({ lighting: CONFIGURED_WITH_LIGHT, lightsSequence: [[]] })
    render(<AmbiancePage />)
    await tick()

    expect(screen.getByText('Aucune ampoule reconnue par le pont pour l\'instant.')).toBeInTheDocument()
    expect(screen.getByText('BuzzHue1')).toBeInTheDocument() // affichée, mais en placeholder "introuvable"
    expect(document.querySelector('.ambiance-light.is-missing')).not.toBeNull()
    expect(enregistrerBtn()).toBeDisabled()
  })

  it('un clic sur « Enregistrer » pendant la fenêtre "allMissing" n\'envoie AUCUN POST /config.json', async () => {
    const server = makeServer({ lighting: CONFIGURED_WITH_LIGHT, lightsSequence: [[]] })
    render(<AmbiancePage />)
    await tick()

    fireEvent.click(enregistrerBtn()) // bouton disabled : la modification native du DOM bloque déjà le clic
    expect(callsTo(server, 'POST', '/config.json')).toHaveLength(0)
  })

  it('première association (aucune ampoule jamais configurée) : message neutre, "Enregistrer" désactivé aussi (rows.length === 0)', async () => {
    makeServer({ lighting: CONFIGURED_FRESH, lightsSequence: [[]] })
    render(<AmbiancePage />)
    await tick()

    expect(screen.getByText('Aucune ampoule sur ce pont.')).toBeInTheDocument()
    expect(screen.queryByText('Aucune ampoule reconnue par le pont pour l\'instant.')).toBeNull()
    expect(enregistrerBtn()).toBeDisabled()
  })

  it('auto-guérison : un nouvel essai après 1,5 s peuple la liste et réactive "Enregistrer" sans action utilisateur', async () => {
    const REAL_LIGHTS = [{ id: '1', name: 'BuzzHue1', reachable: true }]
    const server = makeServer({ lighting: CONFIGURED_WITH_LIGHT, lightsSequence: [[], REAL_LIGHTS] })
    render(<AmbiancePage />)
    await tick()

    expect(enregistrerBtn()).toBeDisabled()
    expect(callsTo(server, 'GET', '/api/lighting/lights')).toHaveLength(1)

    // Le seul déclencheur du second essai est le minuteur interne — aucune
    // action utilisateur entre les deux `tick`.
    await tick(1500)

    expect(callsTo(server, 'GET', '/api/lighting/lights')).toHaveLength(2)
    expect(screen.getByText('BuzzHue1')).toBeInTheDocument()
    expect(enregistrerBtn()).not.toBeDisabled()
  })

  it('auto-guérison : un seul essai supplémentaire, jamais en boucle, si la liste reste vide', async () => {
    const server = makeServer({ lighting: CONFIGURED_WITH_LIGHT, lightsSequence: [[]] }) // toujours vide
    render(<AmbiancePage />)
    await tick()
    expect(callsTo(server, 'GET', '/api/lighting/lights')).toHaveLength(1)

    await tick(1500) // le seul nouvel essai attendu
    expect(callsTo(server, 'GET', '/api/lighting/lights')).toHaveLength(2)

    await tick(10_000) // largement au-delà : aucun troisième essai spontané
    expect(callsTo(server, 'GET', '/api/lighting/lights')).toHaveLength(2)
    expect(enregistrerBtn()).toBeDisabled()
  })

  it('non-régression #207 : une ampoule PARTIELLEMENT introuvable (les autres bien présentes) n\'est PAS bloquée', async () => {
    // Cas légitime documenté par #207, distinct du bug : certaines ampoules
    // sont réellement introuvables (renommées/supprimées sur le pont) alors
    // que d'autres répondent normalement. `allMissing` doit rester FAUX ici
    // — bloquer l'enregistrement empêcherait de gérer ce cas normal.
    const lighting = {
      ...CONFIGURED_WITH_LIGHT,
      lights: [{ name: 'BuzzHue1', role: 'general' }, { name: 'Disparue', role: 'general' }],
    }
    makeServer({
      lighting,
      lightsSequence: [[{ id: '1', name: 'BuzzHue1', reachable: true }]], // "Disparue" absente, "BuzzHue1" présente
    })
    render(<AmbiancePage />)
    await tick()

    expect(screen.getByText('BuzzHue1')).toBeInTheDocument()
    expect(screen.getByText(/introuvable sur le pont/)).toBeInTheDocument() // pour "Disparue" seule
    expect(screen.queryByText('Aucune ampoule reconnue par le pont pour l\'instant.')).toBeNull()
    expect(enregistrerBtn()).not.toBeDisabled()
  })
})
