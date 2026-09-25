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
vi.mock('./SoundSpeakerIcon.css', () => ({}))
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

// #234 — Navbar consomme désormais aussi useSoundStatus (haut-parleur de
// cette même entrée « Ambiance ») : sans ce mock, le vrai hook s'exécuterait
// ici (fetch non simulé). Fixe à « inactif » pour tout ce fichier — ce
// fichier couvre les QUATRE états de l'éclairage, pas ceux du son (couverts
// par SoundSpeakerIcon.test.jsx) ; le suffixe de title en résultant est donc
// constant à travers les sous-tests ci-dessous.
const soundMock = { status: { active: false }, refresh: vi.fn() }
vi.mock('../hooks/useSoundStatus', () => ({
  useSoundStatus: () => soundMock,
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
  soundMock.status = { active: false }
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
    expect(labels.indexOf('Ambiance')).toBe(labels.indexOf('Réglages') + 1)
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

  // #234 — la page /admin/ambiance a deux onglets depuis #230 : cette entrée
  // porte désormais DEUX glyphes, l'ampoule (inchangée) et le haut-parleur
  // (nouveau), tous deux aria-hidden, tous deux dans le même .menu-icon.
  it('#234 — porte AUSSI un second SVG (haut-parleur), aria-hidden, à côté de l\'ampoule', () => {
    const { container } = renderNavbar()
    openMenu()
    const icon = getAmbianceLink(container).querySelector('.menu-icon')
    const speaker = icon.querySelector('svg.sound-speaker-icon')
    expect(speaker).not.toBeNull()
    expect(speaker).toHaveAttribute('aria-hidden', 'true')
    expect(icon.textContent).not.toContain('🔊')
  })

  it('les autres entrées ne portent pas de title (comportement inchangé)', () => {
    const { container } = renderNavbar()
    openMenu()
    const config = Array.from(container.querySelectorAll('.navbar-menu-dropdown a')).find(a => a.textContent.includes('Réglages'))
    expect(config).not.toHaveAttribute('title')
  })

  // #234 — le `title` de l'entrée porte désormais LES DEUX sens (handoff
  // §2 : « le title de l'entrée doit porter les deux sens »), concaténés
  // par « · ». Un vrai changement de comportement (pas seulement un ajout
  // de mock) : les quatre chaînes exactes ci-dessous portent donc toutes le
  // suffixe son, constant dans ce bloc (soundMock fixé à « inactif », voir
  // sa déclaration plus haut — les variations du son sont couvertes par
  // SoundSpeakerIcon.test.jsx et le bloc dédié plus bas dans ce fichier).
  describe('glyphe et title selon l\'état', () => {
    it('ok → ampoule pleine avec rayons, « Éclairage : pont connecté · Son : inactif »', () => {
      lightingMock.status = { state: 'ok' }
      const { container } = renderNavbar()
      openMenu()
      const link = getAmbianceLink(container)
      expect(link).toHaveAttribute('title', 'Éclairage : pont connecté · Son : inactif')
      expect(link.querySelector('svg').dataset.glyph).toBe('lit')
    })

    it('unreachable → contour + pastille, « Éclairage : pont injoignable · Son : inactif »', () => {
      lightingMock.status = { state: 'unreachable' }
      const { container } = renderNavbar()
      openMenu()
      const link = getAmbianceLink(container)
      expect(link).toHaveAttribute('title', 'Éclairage : pont injoignable · Son : inactif')
      expect(link.querySelector('svg').dataset.glyph).toBe('alert')
      expect(link.querySelector('svg circle')).not.toBeNull()
    })

    it('refused → même glyphe d\'alerte, title distinct', () => {
      lightingMock.status = { state: 'refused' }
      const { container } = renderNavbar()
      openMenu()
      const link = getAmbianceLink(container)
      expect(link).toHaveAttribute('title', 'Éclairage : association refusée · Son : inactif')
      expect(link.querySelector('svg').dataset.glyph).toBe('alert')
    })

    it('disabled → contour nu sans pastille, « Éclairage : non configuré · Son : inactif »', () => {
      lightingMock.status = { state: 'disabled' }
      const { container } = renderNavbar()
      openMenu()
      const link = getAmbianceLink(container)
      expect(link).toHaveAttribute('title', 'Éclairage : non configuré · Son : inactif')
      expect(link.querySelector('svg').dataset.glyph).toBe('off')
      expect(link.querySelector('svg circle')).toBeNull()
    })
  })

  // #234 — symétrique du bloc lighting ci-dessus, éclairage fixé à
  // « disabled » (constant) pendant que le son varie.
  describe('#234 — glyphe et title du son selon l\'état', () => {
    it('actif → haut-parleur avec ondes, « ... · Son : actif »', () => {
      soundMock.status = { active: true }
      const { container } = renderNavbar()
      openMenu()
      const link = getAmbianceLink(container)
      expect(link).toHaveAttribute('title', 'Éclairage : non configuré · Son : actif')
      expect(link.querySelector('svg.sound-speaker-icon').dataset.glyph).toBe('on')
      expect(link.querySelector('svg.sound-speaker-icon .sound-speaker-waves')).not.toBeNull()
    })

    it('inactif → haut-parleur nu, aucune barre oblique, « ... · Son : inactif »', () => {
      soundMock.status = { active: false }
      const { container } = renderNavbar()
      openMenu()
      const link = getAmbianceLink(container)
      expect(link).toHaveAttribute('title', 'Éclairage : non configuré · Son : inactif')
      const speaker = link.querySelector('svg.sound-speaker-icon')
      expect(speaker.dataset.glyph).toBe('off')
      expect(speaker.querySelector('.sound-speaker-waves')).toBeNull()
      expect(speaker.querySelector('line')).toBeNull()
    })
  })
})
