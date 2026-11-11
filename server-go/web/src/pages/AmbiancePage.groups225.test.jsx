import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'

// ---------------------------------------------------------------------------
// #225 (v11.7.0, LOT-1) — Ambiance › Lumière : sous-onglets « Pont et ampoules » /
// « Groupes » / « Effets », colonne « Groupe » en lecture, enregistrement et essai
// des groupes. Écrit depuis contracts/light-effects.md §2.1, §2.4, §2.5 et la
// maquette docs/mockup/v11.7.0/ui/ambiance_page__light_groups.html (sections 01-04).
// ---------------------------------------------------------------------------

vi.mock('./AmbiancePage.css', () => ({}))
vi.mock('../components/LightGroupsManager.css', () => ({}))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn(), GameProvider: ({ children }) => children }))

import AmbiancePage from './AmbiancePage'
import { useGame } from '../hooks/GameContext'

const json = (body, status = 200) => ({ ok: status < 400, status, json: async () => body, text: async () => (typeof body === 'string' ? body : JSON.stringify(body)) })

const TEAMS = { Rouges: { COLOR: [255, 26, 26], SCORE: 0 }, Bleus: { COLOR: [26, 94, 255], SCORE: 0 } }
const INVENTORY = ['BuzzHue1', 'BuzzHue2', 'BuzzHue3', 'BuzzHue4'].map((name, i) => ({ id: String(i + 1), name, reachable: true, on: false }))

function makeServer({ lighting = {}, bridge = true, testStatus = 200, saveStatus = 200, saveBody = { status: 'ok' } } = {}) {
  const state = { posts: [], tests: [] }
  const base = {
    enabled: true,
    bridge_ip: bridge ? '10.0.0.2' : '',
    bridge_id: bridge ? 'abc' : '',
    api_key_configured: bridge,
    lights: INVENTORY.slice(0, 3).map(l => ({ name: l.name, role: 'general' })),
    groups: [],
    ...lighting,
  }
  global.fetch = vi.fn(async (url, opts = {}) => {
    if (opts.method === 'POST' && url === '/config.json') {
      state.posts.push(JSON.parse(opts.body))
      return json(saveBody, saveStatus)
    }
    if (opts.method === 'POST' && url === '/api/lighting/groups/test') {
      state.tests.push(JSON.parse(opts.body))
      return json({ ok: true }, testStatus)
    }
    if (url === '/config.json') return json({ lighting: base, sound: { enabled: true } })
    if (url === '/api/lighting/status') return json({ state: 'ok', enabled: true })
    if (url === '/api/sound/status') return json({ active: true, muted: false })
    if (url === '/api/lighting/lights') return json({ lights: INVENTORY })
    return json({})
  })
  return state
}

beforeEach(() => { useGame.mockReturnValue({ teams: TEAMS }) })
afterEach(() => { vi.restoreAllMocks() })

const subtab = (name) => screen.findByRole('tab', { name: new RegExp(name) })

describe('AmbiancePage › Lumière — sous-onglets (#225)', () => {
  it('pont associé : trois sous-onglets, « Pont et ampoules » sélectionné par défaut', async () => {
    makeServer()
    render(<AmbiancePage />)
    const bar = within(await screen.findByRole('tablist', { name: /Sections Lumière/ }))
    expect(bar.getByRole('tab', { name: /Pont et ampoules/ })).toHaveAttribute('aria-selected', 'true')
    expect(bar.getByRole('tab', { name: /Groupes/ })).toHaveAttribute('aria-selected', 'false')
    expect(bar.getByRole('tab', { name: /Effets/ })).toHaveAttribute('aria-selected', 'false')
  })

  it('l\'onglet Groupes porte le nombre de groupes configurés', async () => {
    makeServer({ lighting: { groups: [{ name: 'A', role: 'salle', lights: [], near_teams: [] }, { name: 'P', role: 'animateur', lights: [], near_teams: [] }] } })
    render(<AmbiancePage />)
    const tab = await subtab('Groupes')
    expect(within(tab).getByLabelText('2 groupes')).toBeInTheDocument()
  })

  it('sans pont associé (étapes 1 et 2), aucun sous-onglet', async () => {
    makeServer({ bridge: false })
    render(<AmbiancePage />)
    await screen.findByText(/Lumière|pont/i)
    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/config.json'))
    expect(screen.queryByRole('tablist', { name: /Sections Lumière/ })).toBeNull()
  })

  it('« Groupes » affiche la page de gestion ; « Pont et ampoules » masque alors la liste des groupes', async () => {
    makeServer()
    render(<AmbiancePage />)
    expect(screen.queryByText("Groupes d'ambiance")).toBeNull()
    fireEvent.click(await subtab('Groupes'))
    expect(await screen.findByText("Groupes d'ambiance")).toBeInTheDocument()
    fireEvent.click(await subtab('Pont et ampoules'))
    expect(screen.queryByText("Groupes d'ambiance")).toBeNull()
  })

  it('« Effets » (LOT-2) : sélecteurs par moment à la place de « bientôt »', async () => {
    makeServer()
    render(<AmbiancePage />)
    const tab = await subtab('Effets')
    expect(within(tab).queryByText(/bientôt/i)).toBeNull()
    fireEvent.click(tab)
    await waitFor(() => expect(document.querySelector('#asp-eff-room-chrono')).not.toBeNull())
  })
})

describe('AmbiancePage › Lumière — colonne « Groupe » (#225)', () => {
  it('affiche « hors groupe », « n · nom » et « 🎤 nom (animateur) » selon la configuration', async () => {
    makeServer({ lighting: { groups: [
      { name: 'Scène', role: 'salle', lights: ['BuzzHue1'], near_teams: [] },
      { name: 'Pupitre', role: 'animateur', lights: ['BuzzHue2'], near_teams: [] },
    ] } })
    render(<AmbiancePage />)
    expect(await screen.findByLabelText('Groupe de BuzzHue1')).toHaveTextContent('1 · Scène')
    expect(screen.getByLabelText('Groupe de BuzzHue2')).toHaveTextContent('🎤 Pupitre (animateur)')
    expect(screen.getByLabelText('Groupe de BuzzHue3')).toHaveTextContent('hors groupe')
  })

  it('la numérotation ignore le groupe animateur (rang parmi les groupes de salle)', async () => {
    makeServer({ lighting: { groups: [
      { name: 'Pupitre', role: 'animateur', lights: ['BuzzHue2'], near_teams: [] },
      { name: 'Salle', role: 'salle', lights: ['BuzzHue1'], near_teams: [] },
    ] } })
    render(<AmbiancePage />)
    expect(await screen.findByLabelText('Groupe de BuzzHue1')).toHaveTextContent('1 · Salle')
  })

  it('cliquer la pastille ouvre le sous-onglet Groupes', async () => {
    makeServer()
    render(<AmbiancePage />)
    fireEvent.click(await screen.findByLabelText('Groupe de BuzzHue1'))
    expect(await screen.findByText("Groupes d'ambiance")).toBeInTheDocument()
  })
})

describe('AmbiancePage › Lumière — enregistrer et tester les groupes (#225)', () => {
  it('« Enregistrer les groupes » poste le remplacement complet dans lighting.groups', async () => {
    const srv = makeServer()
    render(<AmbiancePage />)
    fireEvent.click(await subtab('Groupes'))
    fireEvent.click(await screen.findByRole('button', { name: 'Créer un groupe par ampoule' }))
    fireEvent.click(screen.getByRole('button', { name: /Enregistrer les groupes/ }))
    await waitFor(() => expect(srv.posts.some(p => p.lighting && Array.isArray(p.lighting.groups))).toBe(true))
    const groups = srv.posts.find(p => p.lighting && p.lighting.groups).lighting.groups
    expect(groups.map(g => g.name)).toEqual(['BuzzHue1', 'BuzzHue2', 'BuzzHue3'])
    expect(groups.every(g => g.role === 'salle' && Array.isArray(g.near_teams))).toBe(true)
  })

  it('un refus 400 du serveur est montré à l\'utilisateur', async () => {
    makeServer({ saveStatus: 400, saveBody: 'lighting.groups : nom de groupe en double' })
    render(<AmbiancePage />)
    fireEvent.click(await subtab('Groupes'))
    fireEvent.click(await screen.findByRole('button', { name: '+ Nouveau groupe' }))
    fireEvent.click(screen.getByRole('button', { name: /Enregistrer les groupes/ }))
    expect(await screen.findByText(/nom de groupe en double/)).toBeInTheDocument()
  })

  it('« Tester le groupe » poste {group:n} sur /api/lighting/groups/test', async () => {
    const srv = makeServer({ lighting: { groups: [{ name: 'Scène', role: 'salle', lights: ['BuzzHue1'], near_teams: [] }] } })
    render(<AmbiancePage />)
    fireEvent.click(await subtab('Groupes'))
    fireEvent.click(await screen.findByRole('button', { name: 'Tester le groupe' }))
    await waitFor(() => expect(srv.tests).toEqual([{ group: 1 }]))
  })

  it('« Tester le groupe » du groupe animateur poste {group:"host"}', async () => {
    const srv = makeServer({ lighting: { groups: [{ name: 'Pupitre', role: 'animateur', lights: ['BuzzHue1'], near_teams: [] }] } })
    render(<AmbiancePage />)
    fireEvent.click(await subtab('Groupes'))
    fireEvent.click(await screen.findByRole('button', { name: 'Tester le groupe' }))
    await waitFor(() => expect(srv.tests).toEqual([{ group: 'host' }]))
  })

  it('un essai sur pont injoignable (503) est expliqué', async () => {
    makeServer({ testStatus: 503, lighting: { groups: [{ name: 'Scène', role: 'salle', lights: ['BuzzHue1'], near_teams: [] }] } })
    render(<AmbiancePage />)
    fireEvent.click(await subtab('Groupes'))
    fireEvent.click(await screen.findByRole('button', { name: 'Tester le groupe' }))
    expect(await screen.findByText(/Pont injoignable/)).toBeInTheDocument()
  })

  it('un essai refusé (409 éclairage désactivé) est expliqué', async () => {
    makeServer({ testStatus: 409, lighting: { groups: [{ name: 'Scène', role: 'salle', lights: ['BuzzHue1'], near_teams: [] }] } })
    render(<AmbiancePage />)
    fireEvent.click(await subtab('Groupes'))
    fireEvent.click(await screen.findByRole('button', { name: 'Tester le groupe' }))
    expect(await screen.findByText(/désactivé, ou le pont n'est pas associé ou a refusé la clé/i)).toBeInTheDocument()
  })
})
