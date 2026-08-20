import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import Navbar from './Navbar'

// ---------------------------------------------------------------------------
// Navbar — bouton ENTRACTE / FIN D'ENTRACTE (#119, delta C2 du plan
// _work/reports/plan-entracte-119-fixes-20260820-155123.md).
//
// C2-F1 : le bouton déménage de GamePage vers la Navbar, entre le badge de
// version (`.version-badge`) et le groupe "Jeu" (`.nav-group-game`) — donc
// visible sur TOUTES les pages admin (la Navbar est montée pour toutes les
// routes admin, App.jsx), pas seulement /admin. Règle de phase inchangée
// (`canToggleEntracte`, utils/phaseRules.js), seul le point d'usage bouge.
// ---------------------------------------------------------------------------

vi.mock('./Navbar.css', () => ({}))
vi.mock('../styles/entracte.css', () => ({}))

vi.mock('../hooks/useUpdates', () => ({
  useUpdates: () => ({
    updateInfo: null,
    checkForUpdates: vi.fn(),
  }),
}))

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
}))

import { useGame } from '../hooks/GameContext'

class ResizeObserverMock {
  constructor(callback) { this.callback = callback; this.observed = [] }
  observe(target) { this.observed.push(target) }
  unobserve() {}
  disconnect() {}
  fire() {}
}

function makeGameMock(overrides = {}) {
  return {
    gameState: { phase: 'STOPPED', entracte: false, ...overrides.gameState },
    setEntracte: vi.fn(),
    ...overrides,
  }
}

function renderNavbar(props = {}) {
  return render(
    <MemoryRouter initialEntries={['/admin']}>
      <Navbar
        connectionStatus="connected"
        clientCounts={{ admin: 1, tv: 0, vplayer: 0, anim: 0 }}
        serverVersion="6.5.2"
        bumpers={{}}
        {...props}
      />
    </MemoryRouter>
  )
}

function getEntracteButton() {
  return screen.getByRole('button', { name: /ENTRACTE/i })
}

beforeEach(() => {
  vi.clearAllMocks()
  global.fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) })
  global.ResizeObserver = ResizeObserverMock
  document.documentElement.style.removeProperty('--navbar-h')
  useGame.mockReturnValue(makeGameMock())
})

describe('Navbar — présence et emplacement du bouton ENTRACTE (C2)', () => {
  it('le bouton ENTRACTE existe et se situe entre le badge de version et le groupe "Jeu"', () => {
    const { container } = renderNavbar()
    const brand = container.querySelector('.navbar-brand')
    const versionBadge = container.querySelector('.version-badge')
    const btn = getEntracteButton()
    const gameGroup = container.querySelector('.nav-group-game')

    expect(brand.contains(btn)).toBe(true)
    expect(versionBadge).not.toBeNull()
    expect(gameGroup).not.toBeNull()

    // Ordre dans le document : version-badge avant le bouton, bouton avant
    // le groupe "Jeu" (DOCUMENT_POSITION_FOLLOWING = 4).
    expect(versionBadge.compareDocumentPosition(btn) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(btn.compareDocumentPosition(gameGroup) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it.each(['/admin', '/admin/quiz', '/admin/teams', '/admin/scoreboard'])(
    'reste visible sur %s (la Navbar est montée pour toutes les routes admin)',
    (route) => {
      const { container } = render(
        <MemoryRouter initialEntries={[route]}>
          <Navbar connectionStatus="connected" clientCounts={{ admin: 1, tv: 0, vplayer: 0, anim: 0 }} serverVersion="6.5.2" bumpers={{}} />
        </MemoryRouter>
      )
      expect(screen.getByRole('button', { name: /ENTRACTE/i })).toBeInTheDocument()
      void container
    }
  )
})

describe('Navbar — libellé du bouton dérivé de gameState.entracte (D3)', () => {
  it('affiche "ENTRACTE" quand entracte est false', () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { phase: 'READY', entracte: false } }))
    renderNavbar()
    expect(getEntracteButton()).toHaveTextContent('ENTRACTE')
    expect(screen.queryByText(/FIN D.ENTRACTE/i)).toBeNull()
  })

  it("affiche \"FIN D'ENTRACTE\" quand entracte est true", () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { phase: 'STOPPED', entracte: true } }))
    renderNavbar()
    expect(screen.getByText(/FIN D.ENTRACTE/i)).toBeInTheDocument()
  })
})

describe('Navbar — état désactivé selon la phase (D4, canToggleEntracte inchangé)', () => {
  it.each([
    ['STOPPED', false],
    ['PREPARE', false],
    ['READY', false],
    ['NEW_GAME', false],
    ['REVEALED', false],
    ['COUNTDOWN', true],
    ['STARTED', true],
    ['PAUSED', true],
    ['ENROLL', true],
  ])('phase %s → bouton disabled=%s', (phase, expectedDisabled) => {
    useGame.mockReturnValue(makeGameMock({ gameState: { phase, entracte: false } }))
    renderNavbar()
    const btn = getEntracteButton()
    if (expectedDisabled) expect(btn).toBeDisabled()
    else expect(btn).not.toBeDisabled()
  })

  it('la sortie (entracte actif) reste cliquable même dans une phase où l\'entrée serait refusée', () => {
    useGame.mockReturnValue(makeGameMock({ gameState: { phase: 'STARTED', entracte: true } }))
    renderNavbar()
    expect(getEntracteButton()).not.toBeDisabled()
  })
})

describe('Navbar — émission de ENTRACTE_SET via setEntracte (D3)', () => {
  it('clic à entracte=false appelle setEntracte(true)', () => {
    const mock = makeGameMock({ gameState: { phase: 'READY', entracte: false } })
    useGame.mockReturnValue(mock)
    renderNavbar()
    fireEvent.click(getEntracteButton())
    expect(mock.setEntracte).toHaveBeenCalledWith(true)
  })

  it('clic à entracte=true appelle setEntracte(false)', () => {
    const mock = makeGameMock({ gameState: { phase: 'STOPPED', entracte: true } })
    useGame.mockReturnValue(mock)
    renderNavbar()
    fireEvent.click(getEntracteButton())
    expect(mock.setEntracte).toHaveBeenCalledWith(false)
  })

  it('bouton désactivé (phase STARTED, hors entracte) → un clic n\'appelle pas setEntracte', () => {
    const mock = makeGameMock({ gameState: { phase: 'STARTED', entracte: false } })
    useGame.mockReturnValue(mock)
    renderNavbar()
    fireEvent.click(getEntracteButton())
    expect(mock.setEntracte).not.toHaveBeenCalled()
  })
})
