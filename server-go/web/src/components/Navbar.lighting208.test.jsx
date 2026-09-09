import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import Navbar from './Navbar'

// ---------------------------------------------------------------------------
// #208 — bouton d'accès aux commandes ON/AUTO/OFF/Flash dans la Navbar.
//
// Historique en une journée (2026-09-07, retours utilisateur successifs) :
//   1. bandeau d'avertissement passif, visible seulement si mode != AUTO,
//      cliquable pour REVENIR sur GamePage où vivait le panneau (SHA 110dfff3) ;
//   2. le panneau lui-même migre sur GamePage (SHA 30ff5abe) ;
//   3. retour QUALIF v10.0.0.13 : décision finale — les commandes ne vivent
//      QUE dans la Navbar. Le bouton est désormais TOUJOURS visible dès que
//      l'éclairage est configuré (pas seulement mode != AUTO) et OUVRE UN
//      POPOVER contenant LightingModePanel (composant inchangé, seul son
//      point de montage bouge) plutôt que de naviguer.
//
// Réutilise la même instance useLightingStatus() que #207 (ampoule du menu
// Ambiance) — mocké ici comme dans Navbar.ambiance.test.jsx, étendu avec
// mode/flash. LightingModePanel (monté dans le popover) consomme le MÊME
// mock, module-level — pas besoin de le mocker séparément.
// ---------------------------------------------------------------------------

vi.mock('./Navbar.css', () => ({}))
vi.mock('./LightingBulbIcon.css', () => ({}))
vi.mock('./LightingModePanel.css', () => ({}))
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

const renderNavbar = (initialEntry = '/admin/quiz') =>
  render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <Navbar connectionStatus="connected" clientCounts={{ admin: 1, tv: 0, vplayer: 0, anim: 0 }} serverVersion="10.0.0" bumpers={{}} />
    </MemoryRouter>
  )

const getButton = () => document.querySelector('.lighting-mode-nav-badge')
const getPopover = () => document.querySelector('.lighting-mode-popover')

describe('Navbar — bouton d\'accès aux commandes d\'éclairage (#208)', () => {
  it('absent quand l\'éclairage n\'est pas configuré (state disabled)', () => {
    lightingMock.status = { state: 'disabled', mode: 'AUTO', flash: false }
    renderNavbar()
    expect(getButton()).toBeNull()
  })

  it('présent même en position AUTO (repos) — ce n\'est plus un simple indicateur, c\'est le point d\'accès', () => {
    lightingMock.status = { state: 'ok', mode: 'AUTO', flash: false }
    renderNavbar()
    const btn = getButton()
    expect(btn).not.toBeNull()
    expect(btn).toHaveClass('is-auto')
    expect(btn.textContent).toContain('Éclairage')
    expect(btn.textContent).not.toContain('engagé') // AUTO n'est pas "engagé"
  })

  it('coloré "on" et libellé "Mode ON engagé" quand le mode est ON', () => {
    lightingMock.status = { state: 'ok', mode: 'ON', flash: false }
    renderNavbar()
    const btn = getButton()
    expect(btn).toHaveClass('is-on')
    expect(btn.textContent).toContain('Mode ON engagé')
  })

  it('coloré "off" et libellé "Mode OFF engagé" quand le mode est OFF', () => {
    lightingMock.status = { state: 'ok', mode: 'OFF', flash: false }
    renderNavbar()
    const btn = getButton()
    expect(btn).toHaveClass('is-off')
    expect(btn.textContent).toContain('Mode OFF engagé')
  })

  it('visible quelle que soit la page admin courante (même garantie que le bouton ENTRACTE)', () => {
    lightingMock.status = { state: 'ok', mode: 'OFF', flash: false }
    renderNavbar('/admin/teams')
    expect(getButton()).not.toBeNull()
  })

  it('aucun popover ouvert par défaut', () => {
    lightingMock.status = { state: 'ok', mode: 'AUTO', flash: false }
    renderNavbar()
    expect(getPopover()).toBeNull()
  })

  it('cliquer le bouton ouvre un popover contenant LightingModePanel (sélecteur ON/AUTO/OFF réel, pas mocké)', () => {
    lightingMock.status = { state: 'ok', mode: 'AUTO', flash: false }
    renderNavbar()

    fireEvent.click(getButton())

    const popover = getPopover()
    expect(popover).not.toBeNull()
    expect(within(popover).getByRole('radio', { name: 'AUTO' })).toBeInTheDocument()
    expect(within(popover).getByRole('radio', { name: 'ON' })).toBeInTheDocument()
    expect(within(popover).getByRole('radio', { name: 'OFF' })).toBeInTheDocument()
    expect(within(popover).getByRole('button', { name: /Flash/ })).toBeInTheDocument()
  })

  it('recliquer le bouton referme le popover', () => {
    lightingMock.status = { state: 'ok', mode: 'AUTO', flash: false }
    renderNavbar()

    fireEvent.click(getButton())
    expect(getPopover()).not.toBeNull()

    fireEvent.click(getButton())
    expect(getPopover()).toBeNull()
  })

  it('cliquer en dehors du popover le referme (même patron que le menu abeille)', () => {
    lightingMock.status = { state: 'ok', mode: 'AUTO', flash: false }
    renderNavbar()

    fireEvent.click(getButton())
    expect(getPopover()).not.toBeNull()

    fireEvent.mouseDown(document.body)
    expect(getPopover()).toBeNull()
  })

  it('se situe dans .navbar-brand, après le bouton ENTRACTE (même zone, même schéma)', () => {
    lightingMock.status = { state: 'ok', mode: 'OFF', flash: false }
    const { container } = renderNavbar()
    const brand = container.querySelector('.navbar-brand')
    const entracteBtn = screen.getByRole('button', { name: /ENTRACTE/i })
    const btn = getButton()

    expect(brand.contains(btn)).toBe(true)
    expect(entracteBtn.compareDocumentPosition(btn) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })
})
