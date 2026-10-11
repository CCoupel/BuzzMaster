import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import Navbar from './Navbar'
import LaunchGameHost from './LaunchGameHost'
import { useGame } from '../hooks/GameContext'

// #253 — entrée « 🚀 Lancer la partie » du groupe Préparation (maquette admin_navbar__launch_game_entry) :
// écran large (en ligne) et étroit (menu unique), toutes phases, grisée quand la popup est ouverte ailleurs.

vi.mock('./Navbar.css', () => ({}))
vi.mock('./NavGroupMenu.css', () => ({}))
vi.mock('./BrandLogo.css', () => ({}))
vi.mock('./LightingBulbIcon.css', () => ({}))
vi.mock('./SoundSpeakerIcon.css', () => ({}))
vi.mock('./LightingModePanel.css', () => ({}))
vi.mock('../styles/entracte.css', () => ({}))

vi.mock('../hooks/useUpdates', () => ({
  useUpdates: () => ({ updateInfo: null, checkForUpdates: vi.fn() }),
}))
vi.mock('../hooks/GameContext', () => ({ useGame: vi.fn() }))
const lightingMock = { status: { state: 'disabled', mode: 'AUTO', flash: false }, refresh: vi.fn() }
vi.mock('../hooks/useLightingStatus', () => ({ useLightingStatus: () => lightingMock }))
vi.mock('../hooks/useSoundStatus', () => ({
  useSoundStatus: () => ({ status: { active: false }, refresh: vi.fn() }),
}))

class ResizeObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
}

// --- Viewport simulé : matchMedia évalue (min|max)-width contre `viewport` ---
let viewport = 1280
function setViewport(w) {
  viewport = w
  window.innerWidth = w
  window.matchMedia = (query) => {
    const evalQuery = () =>
      query.split(/\band\b/).every((part) => {
        const min = part.match(/min-width:\s*(\d+)px/)
        const max = part.match(/max-width:\s*(\d+)px/)
        if (min) return viewport >= Number(min[1])
        if (max) return viewport <= Number(max[1])
        return true
      })
    return {
      get matches() { return evalQuery() },
      media: query,
      addEventListener: () => {},
      removeEventListener: () => {},
      addListener: () => {},
      removeListener: () => {},
      onchange: null,
      dispatchEvent: () => false,
    }
  }
}


const game = (over = {}) => ({
  gameState: { phase: 'STOPPED', entracte: false, LAUNCH_LOCK: { ACTIVE: false, OWNER: '', SINCE: 0, EXPIRES_AT: 0 } },
  setEntracte: vi.fn(),
  teams: {}, bumpers: {}, questions: {}, connectionId: 'me',
  newGame: vi.fn(), newGameRefused: null, clearNewGameRefused: vi.fn(),
  launchLock: vi.fn(() => true), launchLockRefused: null, clearLaunchLockRefused: vi.fn(),
  actionRefused: null, clearActionRefused: vi.fn(),
  ...over,
})
const renderNav = () => render(
  <MemoryRouter initialEntries={['/admin']}>
    <LaunchGameHost>
      <Navbar connectionStatus="connected" clientCounts={{}} serverVersion="1" bumpers={{}} />
    </LaunchGameHost>
  </MemoryRouter>
)

describe('Navbar — Lancer la partie dans le menu Préparation (#253)', () => {
  beforeEach(() => {
    global.fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) })
    global.ResizeObserver = ResizeObserverMock
    lightingMock.status = { state: 'disabled', mode: 'AUTO', flash: false }
  })
  afterEach(() => vi.clearAllMocks())

  it('écran large : l\'entrée est dans le groupe Préparation (en ligne) et envoie OPEN au clic', () => {
    setViewport(1700)
    const g = game(); useGame.mockReturnValue(g)
    renderNav()
    const btn = screen.getByRole('button', { name: /Lancer la partie/ })
    expect(btn.getAttribute('aria-disabled')).toBe('false')
    fireEvent.click(btn)
    expect(g.launchLock).toHaveBeenCalledWith('OPEN')
  })

  it('écran étroit : l\'entrée est dans le menu unique Préparation, après Backstage', () => {
    setViewport(1280)
    const g = game(); useGame.mockReturnValue(g)
    renderNav()
    expect(screen.queryByRole('button', { name: /Lancer la partie/ })).toBeNull() // menu fermé
    fireEvent.click(screen.getByRole('button', { name: 'Préparation' }))
    const labels = [...document.querySelectorAll('.navgroup-dropdown .menu-label')].map(e => e.textContent)
    expect(labels.indexOf('Lancer la partie')).toBe(labels.indexOf('Backstage') + 1)
    fireEvent.click(screen.getByRole('button', { name: /Lancer la partie/ }))
    expect(g.launchLock).toHaveBeenCalledWith('OPEN')
  })

  it.each(['PREPARE', 'STARTED', 'PAUSED', 'STOPPED', 'REVEALED', 'NEW_GAME'])('disponible en phase %s (plus de masquage selon la phase)', (phase) => {
    setViewport(1700)
    const g = game({ gameState: { phase, entracte: false, LAUNCH_LOCK: { ACTIVE: false, OWNER: '', SINCE: 0, EXPIRES_AT: 0 } } })
    useGame.mockReturnValue(g)
    renderNav()
    const btn = screen.getByRole('button', { name: /Lancer la partie/ })
    expect(btn.getAttribute('aria-disabled')).toBe('false')
  })

  it('popup ouverte sur un autre poste : entrée grisée avec infobulle, clic sans effet', () => {
    setViewport(1700)
    const g = game({ gameState: { phase: 'STOPPED', entracte: false, LAUNCH_LOCK: { ACTIVE: true, OWNER: 'other', SINCE: 1, EXPIRES_AT: 2 } } })
    useGame.mockReturnValue(g)
    renderNav()
    const btn = screen.getByRole('button', { name: /Lancer la partie/ })
    expect(btn.getAttribute('aria-disabled')).toBe('true')
    expect(btn.getAttribute('title')).toBe('Popup « Lancer la partie » déjà ouverte sur un autre poste')
    fireEvent.click(btn)
    expect(g.launchLock).not.toHaveBeenCalled()
  })

  it('entracte : clic → message dans la barre, aucun OPEN', () => {
    setViewport(1700)
    const g = game({ gameState: { phase: 'STOPPED', entracte: true, LAUNCH_LOCK: { ACTIVE: false, OWNER: '', SINCE: 0, EXPIRES_AT: 0 } } })
    useGame.mockReturnValue(g)
    renderNav()
    fireEvent.click(screen.getByRole('button', { name: /Lancer la partie/ }))
    expect(g.launchLock).not.toHaveBeenCalled()
    expect(screen.getByRole('alert').textContent).toBe("Impossible pendant une entracte : sortez d'abord de l'entracte.")
  })
})
