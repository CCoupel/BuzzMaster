import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter, useLocation } from 'react-router-dom'
import Navbar from './Navbar'

// ---------------------------------------------------------------------------
// #208 (v10.0.0, correction utilisateur du 2026-09-07) — garde-fou global
// "mode éclairage engagé", élargi de GamePage.jsx (où vit le seul contrôle
// réel, components/LightingModePanel.jsx) vers la Navbar, même schéma que
// le bouton ENTRACTE (Navbar.entracte.test.jsx) : visible sur TOUT /admin/*,
// pas seulement l'écran où se trouve le panneau. Contrat : lighting.md
// §10.1.1 encart (R8, planner-v10-etat-courant-20260907.md §3).
//
// Réutilise la même instance useLightingStatus() que #207 (ampoule du menu
// Ambiance) — mocké ici comme dans Navbar.ambiance.test.jsx, étendu avec
// mode/flash.
// ---------------------------------------------------------------------------

vi.mock('./Navbar.css', () => ({}))
vi.mock('./LightingBulbIcon.css', () => ({}))
vi.mock('../styles/entracte.css', () => ({}))

vi.mock('../hooks/useUpdates', () => ({
  useUpdates: () => ({ updateInfo: null, checkForUpdates: vi.fn() }),
}))

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(() => ({
    gameState: { phase: 'STOPPED', entracte: false },
    setEntracte: vi.fn(),
  })),
}))

const lightingMock = { status: { state: 'disabled', mode: 'AUTO', flash: false }, refresh: vi.fn() }
vi.mock('../hooks/useLightingStatus', () => ({
  useLightingStatus: () => lightingMock,
}))

class ResizeObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
}

beforeEach(() => {
  global.fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) })
  global.ResizeObserver = ResizeObserverMock
  lightingMock.status = { state: 'disabled', mode: 'AUTO', flash: false }
})

afterEach(() => {
  vi.restoreAllMocks()
  document.documentElement.style.removeProperty('--navbar-h')
})

// Capture la route courante en direct — permet de vérifier qu'un clic sur
// le badge ramène bien vers /admin (GamePage), sans mocker react-router.
let lastPathname = null
function LocationSpy() {
  lastPathname = useLocation().pathname
  return null
}

const renderNavbar = (initialEntry = '/admin/quiz') => {
  lastPathname = null
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <LocationSpy />
      <Navbar connectionStatus="connected" clientCounts={{ admin: 1, tv: 0, vplayer: 0, anim: 0 }} serverVersion="10.0.0" bumpers={{}} />
    </MemoryRouter>
  )
}

const getBadge = () => document.querySelector('.lighting-mode-nav-badge')

describe('Navbar — garde-fou global "mode éclairage engagé" (#208)', () => {
  it('absent quand l\'éclairage n\'est pas configuré (state disabled)', () => {
    lightingMock.status = { state: 'disabled', mode: 'AUTO', flash: false }
    renderNavbar()
    expect(getBadge()).toBeNull()
  })

  it('absent quand le mode est AUTO, même éclairage configuré (position de repos)', () => {
    lightingMock.status = { state: 'ok', mode: 'AUTO', flash: false }
    renderNavbar()
    expect(getBadge()).toBeNull()
  })

  it('présent et coloré "on" quand le mode est ON', () => {
    lightingMock.status = { state: 'ok', mode: 'ON', flash: false }
    renderNavbar()
    const badge = getBadge()
    expect(badge).not.toBeNull()
    expect(badge).toHaveClass('is-on')
    expect(badge.textContent).toContain('Mode ON engagé')
  })

  it('présent et coloré "off" quand le mode est OFF', () => {
    lightingMock.status = { state: 'ok', mode: 'OFF', flash: false }
    renderNavbar()
    const badge = getBadge()
    expect(badge).not.toBeNull()
    expect(badge).toHaveClass('is-off')
    expect(badge.textContent).toContain('Mode OFF engagé')
  })

  it('visible quelle que soit la page admin courante (même garantie que le bouton ENTRACTE)', () => {
    lightingMock.status = { state: 'ok', mode: 'OFF', flash: false }
    renderNavbar('/admin/teams')
    expect(getBadge()).not.toBeNull()
  })

  it('cliquer le badge ramène sur /admin (GamePage), là où vit le seul contrôle réel', () => {
    lightingMock.status = { state: 'ok', mode: 'OFF', flash: false }
    renderNavbar('/admin/quiz')
    expect(lastPathname).toBe('/admin/quiz')

    fireEvent.click(getBadge())

    expect(lastPathname).toBe('/admin')
  })

  it('se situe dans .navbar-brand, après le bouton ENTRACTE (même zone, même schéma)', () => {
    lightingMock.status = { state: 'ok', mode: 'OFF', flash: false }
    const { container } = renderNavbar()
    const brand = container.querySelector('.navbar-brand')
    const entracteBtn = screen.getByRole('button', { name: /ENTRACTE/i })
    const badge = getBadge()

    expect(brand.contains(badge)).toBe(true)
    expect(entracteBtn.compareDocumentPosition(badge) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })
})
