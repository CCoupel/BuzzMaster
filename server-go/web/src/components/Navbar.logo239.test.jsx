import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import Navbar from './Navbar'

// ---------------------------------------------------------------------------
// #239 — le bouton d'ouverture du menu de la Navbar affiche le logo A1 à la
// place de l'abeille 🐝 animée ; le texte « BuzzControl » adjacent disparaît.
// Le comportement du menu (ouverture/fermeture, aria-label, title) est
// INCHANGÉ. Plan : _work/handoff/plan-239-v7-20260925-160000.md (partie A).
// Couvre AC1, AC2, AC3, AC9, AC10.
// ---------------------------------------------------------------------------

vi.mock('./Navbar.css', () => ({}))
vi.mock('./BrandLogo.css', () => ({}))
vi.mock('./LightingBulbIcon.css', () => ({}))
vi.mock('./SoundSpeakerIcon.css', () => ({}))
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
vi.mock('../hooks/useLightingStatus', () => ({
  useLightingStatus: () => ({ status: { state: 'disabled', mode: 'AUTO', flash: false }, refresh: vi.fn() }),
}))
vi.mock('../hooks/useSoundStatus', () => ({
  useSoundStatus: () => ({ status: { active: false }, refresh: vi.fn() }),
}))

class ResizeObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
}

beforeEach(() => {
  global.fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) })
  global.ResizeObserver = ResizeObserverMock
})

afterEach(() => {
  vi.restoreAllMocks()
  document.documentElement.style.removeProperty('--navbar-h')
})

const renderNavbar = (initialEntry = '/admin/quiz') =>
  render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <Navbar connectionStatus="connected" clientCounts={{ admin: 1, tv: 0, vplayer: 0, anim: 0 }} serverVersion="11.1.0" bumpers={{}} />
    </MemoryRouter>
  )

const getButton = () => screen.getByLabelText('Menu de navigation')
const getDropdown = (container) => container.querySelector('.navbar-menu-dropdown')

describe('Navbar — bouton de menu = logo A1 (#239 AC1, AC9)', () => {
  it('le bouton contient le logo (Buzz / Control / ⚡) et l\'indicateur ▼', () => {
    renderNavbar()
    const btn = getButton()
    expect(btn.querySelector('.brand-logo-wordmark')).not.toBeNull()
    expect(btn.querySelector('.brand-logo-buzz').textContent).toBe('Buzz')
    expect(btn.querySelector('.brand-logo-control').textContent).toBe('Control')
    expect(btn.querySelector('.brand-logo-bolt').textContent).toBe('⚡')
    expect(btn.querySelector('.menu-indicator').textContent).toBe('▼')
  })

  it('le bouton ne contient plus l\'abeille 🐝', () => {
    renderNavbar()
    expect(getButton().textContent).not.toContain('🐝')
    expect(document.querySelector('.brand-logo')).toBeNull()
  })

  it('le contenu du logo est aria-hidden ; le bouton reste annoncé « Menu de navigation »', () => {
    renderNavbar()
    const btn = getButton()
    expect(btn.querySelector('.brand-logo-wordmark').getAttribute('aria-hidden')).toBe('true')
    expect(btn.getAttribute('aria-label')).toBe('Menu de navigation')
    expect(screen.getByRole('button', { name: 'Menu de navigation' })).toBe(btn)
  })

  it('AC2 — le title du bouton est conservé (« Menu »)', () => {
    renderNavbar()
    expect(getButton().getAttribute('title')).toBe('Menu')
  })
})

describe('Navbar — texte « BuzzControl » retiré (#239 AC3)', () => {
  it('.brand-text n\'est plus rendu', () => {
    const { container } = renderNavbar()
    expect(container.querySelector('.brand-text')).toBeNull()
  })

  it('le badge version suit directement le bouton de menu dans .navbar-brand', () => {
    const { container } = renderNavbar()
    const brand = container.querySelector('.navbar-brand')
    const badge = brand.querySelector('.version-badge')
    expect(badge).not.toBeNull()
    expect(badge.textContent).toContain('v11.1.0')
    // Plus aucun élément texte « BuzzControl » entre le conteneur du bouton et le badge
    expect(badge.previousElementSibling).toBe(brand.querySelector('.brand-logo-container'))
  })

  it('le badge version navigue toujours vers /admin/updates (clic + Entrée)', () => {
    // La navigation est vérifiée via la route : on observe l'absence d'erreur et
    // le rôle/tabIndex conservés (la navigation réelle est couverte par Navbar.test.jsx).
    renderNavbar()
    const badge = document.querySelector('.version-badge')
    expect(badge.getAttribute('role')).toBe('button')
    expect(badge.getAttribute('tabindex')).toBe('0')
    expect(() => fireEvent.click(badge)).not.toThrow()
    expect(() => fireEvent.keyDown(badge, { key: 'Enter' })).not.toThrow()
  })
})

describe('Navbar — ouverture/fermeture du menu inchangée (#239 AC2)', () => {
  it('fermé par défaut', () => {
    const { container } = renderNavbar()
    expect(getDropdown(container)).toBeNull()
  })

  it('un clic sur le bouton (logo) ouvre le menu, un second clic le ferme', () => {
    const { container } = renderNavbar()
    fireEvent.click(getButton())
    expect(getDropdown(container)).not.toBeNull()
    fireEvent.click(getButton())
    expect(getDropdown(container)).toBeNull()
  })

  it('un clic sur le logo lui-même (enfant du bouton) ouvre aussi le menu', () => {
    const { container } = renderNavbar()
    fireEvent.click(getButton().querySelector('.brand-logo-buzz'))
    expect(getDropdown(container)).not.toBeNull()
  })

  it('fermeture au clic extérieur', () => {
    const { container } = renderNavbar()
    fireEvent.click(getButton())
    expect(getDropdown(container)).not.toBeNull()
    fireEvent.mouseDown(document.body)
    fireEvent.click(document.body)
    expect(getDropdown(container)).toBeNull()
  })

  it('le menu ouvert contient toujours ses entrées (au moins une entrée de navigation)', () => {
    const { container } = renderNavbar()
    fireEvent.click(getButton())
    expect(getDropdown(container).querySelectorAll('.menu-item').length).toBeGreaterThan(0)
  })
})

describe('Navbar — animation de rotation retirée (#239 AC10)', () => {
  it('le bouton ne contient aucun élément animé par framer-motion (pas de transform rotate inline)', () => {
    renderNavbar()
    const animated = getButton().querySelectorAll('[style*="rotate"]')
    expect(animated.length).toBe(0)
  })

  it('Navbar.jsx n\'importe plus framer-motion pour l\'abeille (aucun motion.span logo)', () => {
    const here = dirname(fileURLToPath(import.meta.url))
    const src = readFileSync(resolve(here, './Navbar.jsx'), 'utf8')
    expect(src).not.toMatch(/animate=\{\{\s*rotate/)
    expect(src).not.toContain('🐝')
    expect(src).toMatch(/<BrandLogo\s*\/>/)
  })

  it('le hover scale(1.05) du bouton est conservé dans Navbar.css', () => {
    const here = dirname(fileURLToPath(import.meta.url))
    const css = readFileSync(resolve(here, './Navbar.css'), 'utf8')
    expect(css).toMatch(/\.brand-logo-button:hover[^{]*\{[^}]*scale\(1\.05\)/)
  })

  it('Navbar.css ne définit plus .brand-logo ni .brand-text', () => {
    const here = dirname(fileURLToPath(import.meta.url))
    const css = readFileSync(resolve(here, './Navbar.css'), 'utf8')
    expect(css).not.toMatch(/\.brand-logo\s*\{/)
    expect(css).not.toMatch(/\.brand-text/)
  })
})
