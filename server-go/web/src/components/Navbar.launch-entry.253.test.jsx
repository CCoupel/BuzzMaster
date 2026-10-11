import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import Navbar from './Navbar'
import LaunchGameHost from './LaunchGameHost'
import { useGame } from '../hooks/GameContext'

// #253 (v11.2, lot V2-4) — entrée « 🚀 Lancer la partie » du groupe Préparation (maquette VALIDÉE
// admin_navbar__launch_game_entry) : écran large (en ligne) et étroit (menu unique), clic pendant une entracte
// (message, pas d'ouverture), popup ouverte sur un autre poste (entrée grisée). Boîte noire : libellés, rôles ARIA.

vi.mock('./Navbar.css', () => ({}))
vi.mock('./NavGroupMenu.css', () => ({}))
vi.mock('./BrandLogo.css', () => ({}))
vi.mock('./LightingBulbIcon.css', () => ({}))
vi.mock('./SoundSpeakerIcon.css', () => ({}))
vi.mock('./LightingModePanel.css', () => ({}))
vi.mock('./LaunchGameDialog.css', () => ({}))
vi.mock('./LaunchGameHost.css', () => ({}))
vi.mock('../styles/entracte.css', () => ({}))
vi.mock('../hooks/useUpdates', () => ({ useUpdates: () => ({ updateInfo: null, checkForUpdates: vi.fn() }) }))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn() }))
vi.mock('../hooks/useLightingStatus', () => ({ useLightingStatus: () => ({ status: { state: 'disabled', mode: 'AUTO', flash: false }, refresh: vi.fn() }) }))
vi.mock('../hooks/useSoundStatus', () => ({ useSoundStatus: () => ({ status: { active: false }, refresh: vi.fn() }) }))

class ResizeObserverMock { observe() {} unobserve() {} disconnect() {} }
let viewport = 1280
function setViewport(w) {
  viewport = w
  window.innerWidth = w
  window.matchMedia = (query) => ({
    get matches() {
      return query.split(/\band\b/).every((part) => {
        const min = part.match(/min-width:\s*(\d+)px/)
        const max = part.match(/max-width:\s*(\d+)px/)
        if (min) return viewport >= Number(min[1])
        if (max) return viewport <= Number(max[1])
        return true
      })
    },
    media: query, addEventListener: () => {}, removeEventListener: () => {}, addListener: () => {}, removeListener: () => {}, onchange: null, dispatchEvent: () => false,
  })
}

const LAUNCH_TITLE = "Choisir ce qu'il faut effacer et relancer la partie"
let game
const mk = (over = {}) => ({
  gameState: { phase: 'STOPPED', entracte: false, LAUNCH_LOCK: { ACTIVE: false, OWNER: '', SINCE: 0, EXPIRES_AT: 0 }, ...(over.gameState || {}) },
  setEntracte: vi.fn(), connectionId: 'me', launchLock: vi.fn(() => true), clearLaunchLockRefused: vi.fn(),
  teams: {}, bumpers: {}, questions: {}, newGame: vi.fn(), newGameRefused: null, clearNewGameRefused: vi.fn(),
  launchLockRefused: null, actionRefused: null, clearActionRefused: vi.fn(),
  ...(({ gameState, ...rest }) => rest)(over),
})

beforeEach(() => {
  global.fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) })
  global.ResizeObserver = ResizeObserverMock
  game = mk()
  useGame.mockReturnValue(game)
  setViewport(1280)
})
afterEach(() => { vi.restoreAllMocks() })

const renderNavbar = () => render(
  <MemoryRouter initialEntries={['/admin']}>
    <LaunchGameHost>
      <Navbar connectionStatus="connected" clientCounts={{ admin: 1, tv: 1, vplayer: 0, anim: 1 }} serverVersion="11.2.0" bumpers={{}} />
    </LaunchGameHost>
  </MemoryRouter>
)
const prepButton = () => screen.getByRole('button', { name: /Préparation/i })
const entry = () => {
  const el = screen.queryByText(/Lancer la partie/i)
  return el ? el.closest('button, a, [role="menuitem"]') || el : null
}
const follows = (a, b) => !!(a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING)

describe('Entrée « Lancer la partie » du menu Préparation (#253)', () => {
  it('Critical — écran étroit : absente tant que le menu est fermé ; ouverte, elle suit Backstage et précède la section Interface', () => {
    renderNavbar()
    expect(entry()).toBeNull()
    fireEvent.click(prepButton())
    const e = entry()
    expect(e).not.toBeNull()
    expect(e.textContent).toMatch(/🚀/)
    const backstage = document.querySelector('a[href="/admin/backstage"]')
    expect(follows(backstage, e)).toBe(true)
    expect(follows(e, screen.getByText(/^Interface$/i))).toBe(true)
  })

  it('écran large (≥ 1685 px) : l’entrée est visible en ligne dans le groupe Préparation, sans ouvrir de menu', () => {
    setViewport(1700)
    renderNavbar()
    const e = entry()
    expect(e).not.toBeNull()
    const backstage = document.querySelector('a[href="/admin/backstage"]')
    expect(follows(backstage, e)).toBe(true)
  })

  it('l’entrée porte l’info-bulle de la maquette', () => {
    renderNavbar()
    fireEvent.click(prepButton())
    expect(entry().getAttribute('title') || entry().closest('[title]')?.getAttribute('title')).toBe(LAUNCH_TITLE)
  })

  it('Critical — un clic envoie LAUNCH_LOCK OPEN (jamais NEW_GAME) ; la popup attend l’état verrouillé à notre nom', () => {
    renderNavbar()
    fireEvent.click(prepButton())
    fireEvent.click(entry())
    expect(game.launchLock).toHaveBeenCalledWith('OPEN')
    expect(game.newGame).not.toHaveBeenCalled()
    expect(screen.queryByRole('dialog')).toBeNull() // pas d'état verrouillé à notre nom : pas d'affichage
  })

  it('verrou à notre nom reçu → la popup s’affiche', () => {
    const { rerender } = renderNavbar()
    fireEvent.click(prepButton())
    fireEvent.click(entry())
    useGame.mockReturnValue(mk({ gameState: { LAUNCH_LOCK: { ACTIVE: true, OWNER: 'me', SINCE: 1, EXPIRES_AT: 46000 } } }))
    rerender(
      <MemoryRouter initialEntries={['/admin']}>
        <LaunchGameHost><Navbar connectionStatus="connected" clientCounts={{ admin: 1, tv: 1, vplayer: 0, anim: 1 }} serverVersion="11.2.0" bumpers={{}} /></LaunchGameHost>
      </MemoryRouter>
    )
    expect(screen.getByRole('dialog')).toBeTruthy()
  })

  it('Critical — pendant une entracte : message, aucune ouverture, aucun OPEN envoyé', () => {
    useGame.mockReturnValue(mk({ gameState: { entracte: true } }))
    renderNavbar()
    fireEvent.click(prepButton())
    fireEvent.click(entry())
    expect(game.launchLock).not.toHaveBeenCalled()
    expect(screen.getByRole('alert').textContent).toMatch(/Impossible pendant une entracte : sortez d'abord de l'entracte\./)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('Critical — popup ouverte sur un autre poste : entrée grisée (aria-disabled), clic sans effet', () => {
    const locked = mk({ gameState: { LAUNCH_LOCK: { ACTIVE: true, OWNER: 'other-conn', SINCE: 1, EXPIRES_AT: 46000 } } })
    useGame.mockReturnValue(locked)
    renderNavbar()
    fireEvent.click(prepButton())
    const e = entry()
    expect(e.getAttribute('aria-disabled')).toBe('true')
    fireEvent.click(e)
    expect(locked.launchLock).not.toHaveBeenCalled()
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('sans verrou ailleurs, l’entrée n’est pas grisée', () => {
    renderNavbar()
    fireEvent.click(prepButton())
    expect(entry().getAttribute('aria-disabled')).not.toBe('true')
  })

  it('refus serveur LOCKED_BY_OTHER à l’ouverture : message court, pas de popup', () => {
    const refused = mk({ launchLockRefused: { reason: 'LOCKED_BY_OTHER' } })
    useGame.mockReturnValue(refused)
    renderNavbar()
    expect(within(screen.getByRole('alert')).getByText(/déjà ouverte sur un autre poste/i)).toBeTruthy()
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})
