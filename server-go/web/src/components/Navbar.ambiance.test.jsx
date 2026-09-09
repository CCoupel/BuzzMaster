import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import Navbar from './Navbar'

// ---------------------------------------------------------------------------
// #207 — entrée « Ambiance » du menu abeille, placée juste après Config, avec
// une ampoule dont la FORME dit l'état (contrat hue-bridge.md §7.1, maquette
// §01 rév. 4) et un `title` en toutes lettres.
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

const lightingMock = { status: { state: 'disabled' }, refresh: vi.fn() }
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
  lightingMock.status = { state: 'disabled' }
})

afterEach(() => {
  vi.restoreAllMocks()
  document.documentElement.style.removeProperty('--navbar-h')
})

const renderNavbar = () =>
  render(
    <MemoryRouter initialEntries={['/admin']}>
      <Navbar connectionStatus="connected" clientCounts={{ admin: 1, tv: 0, vplayer: 0, anim: 0 }} serverVersion="10.0.0" bumpers={{}} />
    </MemoryRouter>
  )

function openMenu() {
  fireEvent.click(screen.getByLabelText('Menu de navigation'))
}

function getAmbianceLink(container) {
  return Array.from(container.querySelectorAll('.navbar-menu-dropdown a')).find(a => a.textContent.includes('Ambiance'))
}

describe('#207 — entrée « Ambiance » du menu', () => {
  it('est un lien vers /admin/ambiance, placé JUSTE APRÈS Config', () => {
    const { container } = renderNavbar()
    openMenu()
    const labels = Array.from(container.querySelectorAll('.navbar-menu-dropdown .menu-label')).map(el => el.textContent)
    expect(labels.indexOf('Ambiance')).toBe(labels.indexOf('Config') + 1)
    expect(labels[labels.length - 1]).toBe('Quitter')

    const link = getAmbianceLink(container)
    expect(link).toHaveAttribute('href', '/admin/ambiance')
  })

  it('porte un SVG aria-hidden dans .menu-icon — pas un emoji', () => {
    const { container } = renderNavbar()
    openMenu()
    const icon = getAmbianceLink(container).querySelector('.menu-icon')
    const svg = icon.querySelector('svg.lighting-bulb-icon')
    expect(svg).not.toBeNull()
    expect(svg).toHaveAttribute('aria-hidden', 'true')
    expect(icon.textContent).not.toContain('💡')
  })

  it('les autres entrées ne portent pas de title (comportement inchangé)', () => {
    const { container } = renderNavbar()
    openMenu()
    const config = Array.from(container.querySelectorAll('.navbar-menu-dropdown a')).find(a => a.textContent.includes('Config'))
    expect(config).not.toHaveAttribute('title')
  })

  describe('glyphe et title selon l\'état', () => {
    it('ok → ampoule pleine avec rayons, « Éclairage : pont connecté »', () => {
      lightingMock.status = { state: 'ok' }
      const { container } = renderNavbar()
      openMenu()
      const link = getAmbianceLink(container)
      expect(link).toHaveAttribute('title', 'Éclairage : pont connecté')
      expect(link.querySelector('svg').dataset.glyph).toBe('lit')
    })

    it('unreachable → contour + pastille, « Éclairage : pont injoignable »', () => {
      lightingMock.status = { state: 'unreachable' }
      const { container } = renderNavbar()
      openMenu()
      const link = getAmbianceLink(container)
      expect(link).toHaveAttribute('title', 'Éclairage : pont injoignable')
      expect(link.querySelector('svg').dataset.glyph).toBe('alert')
      expect(link.querySelector('svg circle')).not.toBeNull()
    })

    it('refused → même glyphe d\'alerte, title distinct', () => {
      lightingMock.status = { state: 'refused' }
      const { container } = renderNavbar()
      openMenu()
      const link = getAmbianceLink(container)
      expect(link).toHaveAttribute('title', 'Éclairage : association refusée')
      expect(link.querySelector('svg').dataset.glyph).toBe('alert')
    })

    it('disabled → contour nu sans pastille, « Éclairage : non configuré »', () => {
      lightingMock.status = { state: 'disabled' }
      const { container } = renderNavbar()
      openMenu()
      const link = getAmbianceLink(container)
      expect(link).toHaveAttribute('title', 'Éclairage : non configuré')
      expect(link.querySelector('svg').dataset.glyph).toBe('off')
      expect(link.querySelector('svg circle')).toBeNull()
    })
  })
})
