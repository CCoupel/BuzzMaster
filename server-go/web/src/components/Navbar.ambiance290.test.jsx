import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import Navbar from './Navbar'

// ---------------------------------------------------------------------------
// #290 (v11.6.0, LOT-4) — popover « Ambiance » de l'en-tête : état EFFECTIF (GameState.AMBIANCE_EFFECTIVE) et étiquette
// « manche » pour chaque réglage imposé par la manche courante, ligne « Manche « X » : N réglages imposés », portée du
// geste direct. Contrat game-state.md (AMBIANCE_EFFECTIVE = { SETTINGS, SOURCES, ROUND_ID }, ROUNDS[].AMBIANCE),
// lighting.md §12 (restitution) ; maquette admin_navbar__ambiance_popover_round_source.html.
// Hors manche surchargée : rendu v11.4.0 à l'identique (non-régression). Écrit depuis le contrat/la maquette.
// ---------------------------------------------------------------------------

vi.mock('./Navbar.css', () => ({}))
vi.mock('./LightingBulbIcon.css', () => ({}))
vi.mock('./SoundSpeakerIcon.css', () => ({}))
vi.mock('./LightingModePanel.css', () => ({}))
vi.mock('../styles/entracte.css', () => ({}))
vi.mock('../hooks/useUpdates', () => ({ useUpdates: () => ({ updateInfo: null, checkForUpdates: vi.fn() }) }))

let gameStateMock = { phase: 'STOPPED', entracte: false }
vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(() => ({ gameState: gameStateMock, setEntracte: vi.fn() })),
}))
const lightingMock = { status: {}, refresh: vi.fn() }
vi.mock('../hooks/useLightingStatus', () => ({ useLightingStatus: () => lightingMock }))
const soundMock = { status: {}, refresh: vi.fn() }
vi.mock('../hooks/useSoundStatus', () => ({ useSoundStatus: () => soundMock, notifySoundChanged: vi.fn() }))

class ResizeObserverMock { observe() {} unobserve() {} disconnect() {} }

const GLOBAL_SETTINGS = { sound: { muted: false, countdown: { enabled: true, loop: true }, cues: {} }, light: { mode: 'AUTO' } }
const ALL_GLOBAL = { sound: { muted: 'global', countdown: { enabled: 'global', loop: 'global' }, cues: {} }, light: { mode: 'global' } }

const OUTSIDE_ROUND = { SETTINGS: GLOBAL_SETTINGS, SOURCES: ALL_GLOBAL, ROUND_ID: '' }
const IN_ROUND = {
  SETTINGS: { sound: { muted: false, countdown: { enabled: false, loop: true }, cues: { 'qcm-indice': false } }, light: { mode: 'ON' } },
  SOURCES: { sound: { muted: 'global', countdown: { enabled: 'round', loop: 'global' }, cues: { 'qcm-indice': 'round' } }, light: { mode: 'round' } },
  ROUND_ID: 'r2',
}
const ROUNDS = [
  { ID: 'r1', NAME: 'Cinéma', AMBIANCE: {} },
  { ID: 'r2', NAME: 'Blind-test', AMBIANCE: { sound: { countdown: { enabled: false }, cues: { 'qcm-indice': false } }, light: { mode: 'ON' } } },
]

beforeEach(() => {
  global.fetch = vi.fn().mockResolvedValue({ ok: true, text: async () => '', json: async () => ({}) })
  global.ResizeObserver = ResizeObserverMock
  lightingMock.status = { state: 'ok', mode: 'AUTO', flash: false, enabled: true, bridge_ip: '10.0.0.2' }
  soundMock.status = { active: true, muted: false }
  soundMock.refresh = vi.fn().mockResolvedValue(null)
  gameStateMock = { phase: 'STOPPED', entracte: false, ROUNDS, AMBIANCE_EFFECTIVE: OUTSIDE_ROUND }
})
afterEach(() => { vi.restoreAllMocks() })

const renderNavbar = () =>
  render(
    <MemoryRouter initialEntries={['/admin/quiz']}>
      <Navbar connectionStatus="connected" clientCounts={{ admin: 1, tv: 0, vplayer: 0, anim: 0 }} serverVersion="11.6.0" bumpers={{}} />
    </MemoryRouter>
  )
const btn = () => document.querySelector('.lighting-mode-nav-badge')
const popover = () => document.querySelector('.navbar-popover')

describe('popover « Ambiance » — hors manche surchargée (#290, non-régression v11.4.0)', () => {
  it('aucune ligne « réglages imposés », aucune étiquette « manche »', () => {
    renderNavbar()
    fireEvent.click(btn())
    expect(within(popover()).queryByText(/réglages? imposés?/i)).toBeNull()
    expect(popover().textContent).not.toMatch(/Son manche|Éclairage général manche/)
    expect(within(popover()).getByRole('radiogroup', { name: 'Éclairage général' })).toBeInTheDocument()
    expect(within(popover()).getByRole('radiogroup', { name: 'Son' })).toBeInTheDocument()
  })

  it('sans AMBIANCE_EFFECTIVE (serveur antérieur / pas encore reçu) : rendu v11.4.0 à l\'identique', () => {
    gameStateMock = { phase: 'STOPPED', entracte: false }
    renderNavbar()
    fireEvent.click(btn())
    expect(within(popover()).queryByText(/réglages? imposés?/i)).toBeNull()
    expect(btn().textContent).toContain('Ambiance')
  })
})

describe('popover « Ambiance » — manche « Blind-test » active (#290)', () => {
  beforeEach(() => { gameStateMock = { phase: 'STOPPED', entracte: false, ROUNDS, AMBIANCE_EFFECTIVE: IN_ROUND } })

  it('ligne « Manche « Blind-test » : 3 réglages imposés » (cues comptées une à une)', () => {
    renderNavbar()
    fireEvent.click(btn())
    expect(within(popover()).getByText(/Manche « Blind-test » : 3 réglages imposés/)).toBeInTheDocument()
  })

  it("l'éclairage affiché est l'EFFECTIF (ON, imposé par la manche) avec l'étiquette « manche »", () => {
    renderNavbar()
    fireEvent.click(btn())
    const title = within(popover()).getByText(/Éclairage général/)
    expect(title.parentElement).toHaveTextContent(/manche/i)
    expect(btn().textContent).toContain('Mode ON engagé')
  })

  it('le décompte effectif (inactif) est affiché avec l\'étiquette « manche »', () => {
    renderNavbar()
    fireEvent.click(btn())
    const line = within(popover()).getByText(/Décompte/)
    expect(line).toHaveTextContent(/inactif/i)
    expect(line).toHaveTextContent(/manche/i)
  })

  it('le texte de portée du geste direct est affiché (un geste vaut jusqu\'au prochain changement de manche)', () => {
    renderNavbar()
    fireEvent.click(btn())
    expect(popover().textContent).toMatch(/prochain changement de manche/i)
  })

  it('un champ NON imposé par la manche (son) ne porte pas l\'étiquette « manche »', () => {
    renderNavbar()
    fireEvent.click(btn())
    const soundSection = within(popover()).getByRole('radiogroup', { name: 'Son' }).parentElement
    expect(within(soundSection).queryByText('manche')).toBeNull() // étiquette seule (le texte d'aide peut citer le mot)
  })
})

describe('popover « Ambiance » — son coupé imposé par la manche ou par un geste direct (#290)', () => {
  it('son coupé imposé par la manche : « Son coupé » (effectif) et étiquette « manche » sur le son', () => {
    gameStateMock = {
      phase: 'STOPPED', entracte: false, ROUNDS,
      AMBIANCE_EFFECTIVE: {
        SETTINGS: { ...GLOBAL_SETTINGS, sound: { ...GLOBAL_SETTINGS.sound, muted: true } },
        SOURCES: { ...ALL_GLOBAL, sound: { ...ALL_GLOBAL.sound, muted: 'round' } },
        ROUND_ID: 'r2',
      },
    }
    soundMock.status = { active: true, muted: false } // le statut REST n'est pas la source : l'effectif prime
    renderNavbar()
    expect(btn().textContent).toContain('Son coupé')
    fireEvent.click(btn())
    expect(within(popover()).getByRole('radiogroup', { name: 'Son' }).parentElement).toHaveTextContent(/manche/i)
  })

  it('geste direct (source « direct ») : « Son coupé » sans étiquette « manche »', () => {
    gameStateMock = {
      phase: 'STOPPED', entracte: false, ROUNDS,
      AMBIANCE_EFFECTIVE: {
        SETTINGS: { ...IN_ROUND.SETTINGS, sound: { ...IN_ROUND.SETTINGS.sound, muted: true } },
        SOURCES: { ...IN_ROUND.SOURCES, sound: { ...IN_ROUND.SOURCES.sound, muted: 'direct' } },
        ROUND_ID: 'r2',
      },
    }
    renderNavbar()
    expect(btn().textContent).toContain('Son coupé')
    fireEvent.click(btn())
    expect(within(within(popover()).getByRole('radiogroup', { name: 'Son' }).parentElement).queryByText('manche')).toBeNull()
    expect(popover().textContent).toMatch(/Son coupé|geste direct|direct/i)
  })
})
