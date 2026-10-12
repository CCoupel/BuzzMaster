import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import Navbar from './Navbar'
import { useGame } from '../hooks/GameContext'

// #242 — accessibilité de la Navbar (réserves de la review de #238) :
//  1. Label in Name (WCAG 2.5.3) : le nom accessible du bouton d'entracte
//     actif doit contenir son libellé visible « REPRISE » (commande vocale
//     « cliquer REPRISE »).
//  2. Aucun <div aria-label> sans rôle (déconseillé par ARIA) dans les menus
//     déroulants de la barre.
// Comportement observable (rôle, nom accessible), pas le mécanisme du correctif.

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
    media: query,
    addEventListener: () => {}, removeEventListener: () => {},
    addListener: () => {}, removeListener: () => {},
    onchange: null, dispatchEvent: () => false,
  })
}

const gameMock = (entracte = false) => ({
  gameState: { phase: 'STOPPED', entracte },
  setEntracte: vi.fn(),
})

function renderNavbar() {
  return render(
    <MemoryRouter initialEntries={['/admin/scoreboard']}>
      <Navbar
        connectionStatus="connected"
        clientCounts={{ admin: 1, tv: 2, vplayer: 0, anim: 1 }}
        serverVersion="11.2.1"
        bumpers={{}}
      />
    </MemoryRouter>
  )
}

beforeEach(() => {
  global.fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) })
  global.ResizeObserver = ResizeObserverMock
  useGame.mockReturnValue(gameMock())
  setViewport(1280)
})

afterEach(() => {
  vi.restoreAllMocks()
  document.documentElement.style.removeProperty('--navbar-h')
})

const orphanLabelledDivs = () =>
  Array.from(document.querySelectorAll('div[aria-label]')).filter(
    (el) => !el.getAttribute('role')
  )

describe('Navbar — a11y (#242)', () => {
  it('entracte actif : le nom accessible contient le libellé visible « REPRISE » (Label in Name)', () => {
    useGame.mockReturnValue(gameMock(true))
    const { container } = renderNavbar()
    const visible = container.querySelector('.entracte-label').textContent
    expect(visible).toBe('REPRISE')
    const btn = screen.getByRole('button', { name: new RegExp(visible, 'i') })
    expect(btn).toBe(container.querySelector('.entracte-toggle-btn'))
  })

  it('entracte inactif : le nom accessible contient le libellé visible « ENTRACTE »', () => {
    const { container } = renderNavbar()
    const visible = container.querySelector('.entracte-label').textContent
    expect(screen.getByRole('button', { name: new RegExp(visible, 'i') })).toBe(
      container.querySelector('.entracte-toggle-btn')
    )
  })

  it('menu « Préparation » ouvert (< 1685 px) : aucun div[aria-label] sans rôle', () => {
    renderNavbar()
    fireEvent.click(screen.getByRole('button', { name: /Préparation/i }))
    expect(document.querySelector('.navgroup-dropdown')).not.toBeNull()
    expect(orphanLabelledDivs()).toEqual([])
  })

  it('menu « Interface » ouvert (≥ 1685 px, mode en ligne) : aucun div[aria-label] sans rôle', () => {
    setViewport(1800)
    renderNavbar()
    fireEvent.click(screen.getByRole('button', { name: /Interface/i }))
    expect(document.querySelector('.navgroup-dropdown')).not.toBeNull()
    expect(orphanLabelledDivs()).toEqual([])
  })
})
