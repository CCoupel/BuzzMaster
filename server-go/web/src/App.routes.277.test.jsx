import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import { MemoryRouter, useLocation } from 'react-router-dom'
import Navbar from './components/Navbar'
import { useGame } from './hooks/GameContext'

// ---------------------------------------------------------------------------
// #277 — montée de react-router-dom 6.30.6 → 7.18.4.
//
// App.routes.268.test.jsx couvre déjà : chaque route rend la bonne page, la redirection
// /admin/rafale → /admin/quiz?tab=rafale (Navigate replace, pas de boucle au retour), la coque
// et la navigation par <Link>. Mais il REMPLACE la Navbar par un stub : l'état actif des vrais
// <NavLink> (aria-current posé par le routeur, classe `active` de la barre) n'était testé nulle
// part sous le routeur réel. Ce fichier comble ce seul trou : la VRAIE Navbar sous MemoryRouter.
//
// Attentes par événement (waitFor) : v7 enveloppe les mises à jour de navigation dans
// React.startTransition, le DOM n'est donc plus garanti synchrone juste après un clic.
// ---------------------------------------------------------------------------

vi.mock('./components/Navbar.css', () => ({}))
vi.mock('./components/NavGroupMenu.css', () => ({}))
vi.mock('./components/BrandLogo.css', () => ({}))
vi.mock('./components/LightingBulbIcon.css', () => ({}))
vi.mock('./components/SoundSpeakerIcon.css', () => ({}))
vi.mock('./components/LightingModePanel.css', () => ({}))
vi.mock('./styles/entracte.css', () => ({}))
vi.mock('./hooks/useUpdates', () => ({
  useUpdates: () => ({ updateInfo: null, checkForUpdates: vi.fn() }),
}))
vi.mock('./hooks/GameContext', () => ({ useGame: vi.fn() }))
vi.mock('./hooks/useLightingStatus', () => ({
  useLightingStatus: () => ({ status: { state: 'disabled', mode: 'AUTO', flash: false }, refresh: vi.fn() }),
}))
vi.mock('./hooks/useSoundStatus', () => ({
  useSoundStatus: () => ({ status: { active: false }, refresh: vi.fn() }),
}))

class ResizeObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
}

// Mode « en ligne » (≥ 1685 px) : toutes les entrées de la barre sont des <a> directs.
function setViewport(w) {
  window.innerWidth = w
  window.matchMedia = (query) => ({
    get matches() {
      return query.split(/\band\b/).every((part) => {
        const min = part.match(/min-width:\s*(\d+)px/)
        const max = part.match(/max-width:\s*(\d+)px/)
        if (min) return w >= Number(min[1])
        if (max) return w <= Number(max[1])
        return true
      })
    },
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    onchange: null,
    dispatchEvent: () => false,
  })
}

beforeEach(() => {
  global.fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) })
  global.ResizeObserver = ResizeObserverMock
  useGame.mockReturnValue({ gameState: { phase: 'STOPPED', entracte: false }, setEntracte: vi.fn() })
  setViewport(1920)
})

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

function LocationProbe() {
  const location = useLocation()
  return <div data-testid="location" data-path={location.pathname + location.search} />
}

function renderNavbar(path) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Navbar
        connectionStatus="connected"
        clientCounts={{ admin: 1, tv: 0, vplayer: 0, anim: 0 }}
        serverVersion="11.2.3"
        bumpers={{}}
      />
      <LocationProbe />
    </MemoryRouter>
  )
}

const link = (href) => document.querySelector(`a[href="${href}"]`)
// Entrées portant la classe `active` de la barre (état visuel : égalité EXACTE de pathname).
const activeHrefs = () =>
  [...document.querySelectorAll('a.nav-link.active, a.menu-item.active')].map((a) => a.getAttribute('href'))

// Deux mécanismes distincts, tous deux sous le routeur : la classe `active` est calculée par la
// Navbar (location.pathname === chemin) ; aria-current="page" est posé par <NavLink> lui-même
// (correspondance par préfixe : « Jeu » = /admin le porte aussi sur une sous-page — comportement
// du routeur, non asserté ici pour ne pas figer un détail de version).
describe('#277 — Navbar réelle : état actif des NavLink sous le routeur', () => {
  it.each([
    ['/admin/scoreboard'],
    ['/admin/history'],
    ['/admin/palmares'],
  ])('sur %s, seule cette entrée porte la classe active, et <NavLink> pose aria-current="page"', (path) => {
    renderNavbar(path)
    const entry = link(path)
    expect(entry, `lien ${path} absent de la barre`).not.toBeNull()
    expect(entry).toHaveAttribute('aria-current', 'page')
    expect(activeHrefs()).toEqual([path])
  })

  it('sur /admin (page Jeu), « Jeu » est la seule entrée active', () => {
    renderNavbar('/admin')
    expect(link('/admin')).toHaveAttribute('aria-current', 'page')
    expect(activeHrefs()).toEqual(['/admin'])
  })

  it('une query string ne change pas l\'entrée active (/admin/history?x=1)', () => {
    renderNavbar('/admin/history?x=1')
    expect(link('/admin/history')).toHaveAttribute('aria-current', 'page')
  })

  it('cliquer un lien déplace l\'état actif et l\'URL (sans recharger)', async () => {
    renderNavbar('/admin')
    fireEvent.click(link('/admin/scoreboard'))

    await waitFor(() => {
      expect(screen.getByTestId('location').dataset.path).toBe('/admin/scoreboard')
      expect(activeHrefs()).toEqual(['/admin/scoreboard'])
    })
    expect(link('/admin/scoreboard')).toHaveAttribute('aria-current', 'page')
  })
})
