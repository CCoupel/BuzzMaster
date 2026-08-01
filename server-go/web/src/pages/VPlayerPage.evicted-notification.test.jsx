import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, act } from '@testing-library/react'
import VPlayerPage from './VPlayerPage'
import PlayerDisplay from './PlayerDisplay'
import { useGame } from '../hooks/GameContext'

// ---------------------------------------------------------------------------
// Tests : VPlayerPage — remise à zéro du bumper disparu + filet
// SESSION_EXPIRED redevenu atteignable (#123, F2, cause A2)
//
// Plan : _work/reports/plan-20260730-094500.md — cause A2.
//
// Avant ce fix, `setBumper(...)` n'apparaissait que dans la branche "trouvé"
// de l'effet de matching — la branche "disparu" ne remettait à zéro que
// `bumperRef.current` (une ref, jamais l'état React `bumper`). Deux
// conséquences : l'écran de jeu continuait d'afficher un joueur périmé (le
// symptôme rapporté), et le filet de sécurité SESSION_EXPIRED de #118 — qui
// garde sur `bumper || reconnectError || evictedReason !== null` — ne
// pouvait plus jamais se déclencher puisque `bumper` restait éternellement
// truthy. Ce fichier verrouille les deux : la remise à zéro elle-même, et le
// fait que le filet, désormais débloqué, se déclenche bien après son délai.
//
// Les scénarios de la maquette où le motif provient du serveur (PLAYER_EVICTED
// immédiat, PLAYER_CONNECT rejeté avec PLAYER_REMOVED/GAME_RESET/ENROLLMENT_
// CLOSED littéral) sont déjà couverts par VPlayerPage.enroll-race.test.jsx
// (#120) et VPlayerPage.reconnect.test.jsx (#118/#123, section F6/F3) — non
// dupliqués ici.
// ---------------------------------------------------------------------------

const mockNavigate = vi.hoisted(() => vi.fn())

vi.mock('react-router-dom', () => ({
  useNavigate: () => mockNavigate,
}))

vi.mock('../hooks/GameContext', () => ({
  useGame: vi.fn(),
}))

vi.mock('./VPlayerPage.css', () => ({}))

vi.mock('nosleep.js', () => ({
  default: class NoSleep {
    enable() { return Promise.resolve() }
    disable() {}
  },
}))

vi.mock('./PlayerDisplay', () => ({ default: vi.fn(() => null) }))
vi.mock('../components/ArdoiseKeyboard', () => ({ default: () => null }))

function createStorageMock() {
  let store = {}
  return {
    getItem: (k) => (Object.prototype.hasOwnProperty.call(store, k) ? store[k] : null),
    setItem: (k, v) => { store[k] = String(v) },
    removeItem: (k) => { delete store[k] },
    clear: () => { store = {} },
  }
}

const BUMPER_ID = 'vjoueur_alice_1'

const makeGameMock = (overrides = {}) => ({
  sendMessage: vi.fn(),
  gameState: { phase: 'STARTED', question: { ID: 'q1' }, enrollmentActive: true },
  bumpers: {},
  teams: {},
  status: 'connected',
  playerConnectStatus: null,
  clearPlayerConnectStatus: vi.fn(),
  playerEvictedStatus: null,
  clearPlayerEvictedStatus: vi.fn(),
  ...overrides,
})

describe('VPlayerPage — remise à zéro du bumper disparu (#123, F2/A2)', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.stubGlobal('localStorage', createStorageMock())
    vi.stubGlobal('sessionStorage', createStorageMock())
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    localStorage.setItem('vplayer_id', BUMPER_ID)
    useGame.mockReset()
    PlayerDisplay.mockClear()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('le bumper disparaissant du roster fait revenir l\'écran de jeu à l\'état d\'attente (F5), au lieu de rester figé', async () => {
    const bumpers = {
      [BUMPER_ID]: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true, TEAM: 'Rouges' },
    }
    const teams = { Rouges: { NAME: 'Rouges', COLOR: [255, 0, 0] } }
    const mock = makeGameMock({ bumpers, teams })
    useGame.mockReturnValue(mock)
    const { container, rerender } = render(<VPlayerPage />)
    await act(async () => {})

    // Game screen showing (bumper found) — the waiting state must be absent.
    expect(container.querySelector('.vplayer-connecting')).toBeNull()
    expect(PlayerDisplay).toHaveBeenCalled()
    const beforeCall = PlayerDisplay.mock.calls[PlayerDisplay.mock.calls.length - 1][0]
    expect(beforeCall.playerName).toBe('Alice')
    expect(beforeCall.teamName).toBe('Rouges')

    // The admin removes Alice via the real path (B1: roster diff) — her own
    // bumper simply vanishes from the next `bumpers` update, no
    // PLAYER_EVICTED simulated here (this test isolates A2 specifically;
    // the notification path itself is covered elsewhere).
    useGame.mockReturnValue(makeGameMock({ ...mock, bumpers: {}, teams: {} }))
    rerender(<VPlayerPage />)

    // #123 F2: `bumper` (and `team`) must be reset to null — the game
    // screen must NOT keep rendering the stale player. F5's waiting state
    // (already built by #120) takes over instead of a frozen display.
    expect(container.querySelector('.vplayer-connecting')).not.toBeNull()
  })

  it('le filet SESSION_EXPIRED (#118), débloqué par la remise à zéro, se déclenche bien après son délai', async () => {
    // Before #123's F2 fix, `bumper` never went back to null here, so this
    // effect's guard (`if (bumper || ...) return`) stayed permanently blocked
    // — the safety net existed but could never fire in this exact scenario
    // (a disappearance nobody explicitly notified).
    const bumpers = {
      [BUMPER_ID]: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true },
    }
    const mock = makeGameMock({ bumpers })
    useGame.mockReturnValue(mock)
    const { container, rerender } = render(<VPlayerPage />)
    await act(async () => {})
    expect(container.querySelector('.vplayer-connecting')).toBeNull()

    // Bumper disappears — no PLAYER_EVICTED ever arrives (the notification
    // path is deliberately NOT exercised here, to isolate the safety net).
    useGame.mockReturnValue(makeGameMock({ ...mock, bumpers: {} }))
    rerender(<VPlayerPage />)
    expect(container.querySelector('.vplayer-connecting')).not.toBeNull()
    expect(mockNavigate).not.toHaveBeenCalled()

    // Just under the 10s safety-net delay: still waiting, nothing fired yet.
    await act(async () => { vi.advanceTimersByTime(9000) })
    expect(container.querySelector('.vplayer-reconnect-error-text')).toBeNull()
    expect(mockNavigate).not.toHaveBeenCalled()

    // Comfortably past it: the safety net fires.
    await act(async () => { vi.advanceTimersByTime(2000) })
    expect(container.querySelector('.vplayer-reconnect-error-text')).not.toBeNull()

    // And it eventually redirects, same as any other eviction reason.
    await act(async () => { vi.advanceTimersByTime(3000) })
    expect(mockNavigate).toHaveBeenCalledWith('/')
  })

  it('la disparition du bumper n\'efface pas la session avant que le motif (notification ou filet) ne soit connu', async () => {
    const bumpers = {
      [BUMPER_ID]: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true },
    }
    const mock = makeGameMock({ bumpers })
    useGame.mockReturnValue(mock)
    const { rerender } = render(<VPlayerPage />)
    await act(async () => {})

    useGame.mockReturnValue(makeGameMock({ ...mock, bumpers: {} }))
    rerender(<VPlayerPage />)

    // Immediately after the disappearance: session must still be intact —
    // nothing should clear it before a reason (PLAYER_EVICTED or the 10s
    // safety net) is actually known.
    expect(localStorage.getItem('vplayer_name')).toBe('Alice')
    expect(localStorage.getItem('vplayer_id')).toBe(BUMPER_ID)
  })
})
