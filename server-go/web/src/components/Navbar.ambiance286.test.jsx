import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, within, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import Navbar from './Navbar'

// #284 / #286 — bouton et popover « Ambiance » : éclairage + son coupé (sound.muted).
// Contrat : contracts/sound.md §11.2-§11.4.

vi.mock('./Navbar.css', () => ({}))
vi.mock('./LightingBulbIcon.css', () => ({}))
vi.mock('./SoundSpeakerIcon.css', () => ({}))
vi.mock('./LightingModePanel.css', () => ({}))
vi.mock('../styles/entracte.css', () => ({}))
vi.mock('../hooks/useUpdates', () => ({ useUpdates: () => ({ updateInfo: null, checkForUpdates: vi.fn() }) }))
vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(() => ({ gameState: { phase: 'STOPPED', entracte: false }, setEntracte: vi.fn() })),
}))

const lightingMock = { status: {}, refresh: vi.fn() }
vi.mock('../hooks/useLightingStatus', () => ({ useLightingStatus: () => lightingMock }))
const soundMock = { status: {}, refresh: vi.fn() }
vi.mock('../hooks/useSoundStatus', () => ({
  useSoundStatus: () => soundMock,
  notifySoundChanged: vi.fn(),
}))

class ResizeObserverMock { observe() {} unobserve() {} disconnect() {} }

beforeEach(() => {
  global.fetch = vi.fn().mockResolvedValue({ ok: true, text: async () => '', json: async () => ({}) })
  global.ResizeObserver = ResizeObserverMock
  lightingMock.status = { state: 'ok', mode: 'AUTO', flash: false, enabled: true, bridge_ip: '10.0.0.2' }
  soundMock.status = { active: true, muted: false }
  soundMock.refresh = vi.fn().mockResolvedValue(null)
})
afterEach(() => { vi.restoreAllMocks() })

const renderNavbar = () =>
  render(
    <MemoryRouter initialEntries={['/admin/quiz']}>
      <Navbar connectionStatus="connected" clientCounts={{ admin: 1, tv: 0, vplayer: 0, anim: 0 }} serverVersion="11.4.0" bumpers={{}} />
    </MemoryRouter>
  )
const btn = () => document.querySelector('.lighting-mode-nav-badge')
const popover = () => document.querySelector('.navbar-popover')

describe('bouton « Ambiance » (#286)', () => {
  it('repos : libellé « Ambiance », ton neutre, glyphes 💡 et 🔊', () => {
    renderNavbar()
    expect(btn().textContent).toContain('Ambiance')
    expect(btn()).toHaveClass('is-auto')
    expect(btn().textContent).toContain('💡')
    expect(btn().textContent).toContain('🔊')
  })

  it('son coupé : « Son coupé », 🔇, badge orange (is-on)', () => {
    soundMock.status = { active: true, muted: true }
    renderNavbar()
    expect(btn().textContent).toContain('Son coupé')
    expect(btn().textContent).toContain('🔇')
    expect(btn()).toHaveClass('is-on')
  })

  it('mode OFF + son coupé : libellé composé et badge rouge', () => {
    lightingMock.status = { ...lightingMock.status, mode: 'OFF' }
    soundMock.status = { active: true, muted: true }
    renderNavbar()
    expect(btn().textContent).toContain('Mode OFF engagé · Son coupé')
    expect(btn()).toHaveClass('is-off')
  })

  it('visible avec le seul son actif (aucun pont Hue) : section Son uniquement', () => {
    lightingMock.status = { state: 'disabled', mode: 'AUTO', enabled: false, bridge_ip: '' }
    renderNavbar()
    expect(btn()).not.toBeNull()
    expect(btn().textContent).not.toContain('💡')
    fireEvent.click(btn())
    expect(within(popover()).queryByText('Éclairage général')).toBeNull()
    expect(within(popover()).getByRole('radiogroup', { name: 'Son' })).toBeInTheDocument()
  })

  it('absent sans pont ni son', () => {
    lightingMock.status = { state: 'disabled', mode: 'AUTO', enabled: false, bridge_ip: '' }
    soundMock.status = { active: false, muted: false }
    renderNavbar()
    expect(btn()).toBeNull()
  })

  it('son inactif mais coupé : le bouton reste visible (garde-fou « soirée muette »)', () => {
    lightingMock.status = { state: 'disabled', mode: 'AUTO', enabled: false, bridge_ip: '' }
    soundMock.status = { active: false, muted: true }
    renderNavbar()
    expect(btn()).not.toBeNull()
  })
})

describe('popover « Ambiance » (#284 / #286)', () => {
  it('porte la classe commune .navbar-popover (correctif #284) et les deux sections', () => {
    renderNavbar()
    fireEvent.click(btn())
    expect(popover()).not.toBeNull()
    expect(popover()).toHaveClass('lighting-mode-popover')
    expect(within(popover()).getByRole('radiogroup', { name: 'Éclairage général' })).toBeInTheDocument()
    expect(within(popover()).getByRole('radiogroup', { name: 'Son' })).toBeInTheDocument()
  })

  it('le son est à DEUX positions ON / OFF (pas d\'AUTO)', () => {
    renderNavbar()
    fireEvent.click(btn())
    const sound = within(popover()).getByRole('radiogroup', { name: 'Son' })
    expect(within(sound).getAllByRole('radio').map(r => r.textContent)).toEqual(['ON', 'OFF'])
    expect(within(sound).getByRole('radio', { name: 'ON' })).toHaveAttribute('aria-checked', 'true')
  })

  it('OFF → POST /config.json {sound:{muted:true}} puis refresh (pas de mise à jour optimiste)', async () => {
    renderNavbar()
    fireEvent.click(btn())
    const sound = within(popover()).getByRole('radiogroup', { name: 'Son' })
    fireEvent.click(within(sound).getByRole('radio', { name: 'OFF' }))
    await waitFor(() => expect(soundMock.refresh).toHaveBeenCalled())
    const call = global.fetch.mock.calls.find(c => c[0] === '/config.json')
    expect(call[1].method).toBe('POST')
    expect(JSON.parse(call[1].body)).toEqual({ sound: { muted: true } })
    // la position affichée reste celle du serveur (muted=false dans le mock)
    expect(within(sound).getByRole('radio', { name: 'ON' })).toHaveAttribute('aria-checked', 'true')
  })

  it('son coupé : avertissement « après un redémarrage » ; ON renvoie {muted:false}', async () => {
    soundMock.status = { active: true, muted: true }
    renderNavbar()
    fireEvent.click(btn())
    expect(within(popover()).getByText(/après un redémarrage/)).toBeInTheDocument()
    fireEvent.click(within(within(popover()).getByRole('radiogroup', { name: 'Son' })).getByRole('radio', { name: 'ON' }))
    await waitFor(() => expect(global.fetch).toHaveBeenCalled())
    const call = global.fetch.mock.calls.find(c => c[0] === '/config.json')
    expect(JSON.parse(call[1].body)).toEqual({ sound: { muted: false } })
  })

  it('erreur serveur : message visible, refresh non appelé', async () => {
    global.fetch = vi.fn().mockResolvedValue({ ok: false, text: async () => 'boom' })
    vi.spyOn(console, 'error').mockImplementation(() => {})
    renderNavbar()
    fireEvent.click(btn())
    fireEvent.click(within(within(popover()).getByRole('radiogroup', { name: 'Son' })).getByRole('radio', { name: 'OFF' }))
    expect(await within(popover()).findByRole('alert')).toHaveTextContent('boom')
    expect(soundMock.refresh).not.toHaveBeenCalled()
  })

  it('éclairage d\'ambiance désactivé : section grisée + renvoi vers Ambiance, sans commandes ON/AUTO/OFF', () => {
    lightingMock.status = { state: 'disabled', mode: 'AUTO', flash: false, enabled: false, bridge_ip: '10.0.0.2' }
    renderNavbar()
    fireEvent.click(btn())
    expect(within(popover()).getByText(/désactivé/)).toBeInTheDocument()
    expect(within(popover()).getByRole('link', { name: /Ambiance › Lumière/ })).toHaveAttribute('href', '/admin/ambiance')
    expect(within(popover()).queryByRole('radiogroup', { name: 'Éclairage général' })).toBeNull()
    expect(within(popover()).getByRole('radiogroup', { name: 'Son' })).toBeInTheDocument()
  })
})
