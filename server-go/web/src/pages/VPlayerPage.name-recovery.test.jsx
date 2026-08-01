import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, act } from '@testing-library/react'
import VPlayerPage from './VPlayerPage'
import { useGame } from '../hooks/GameContext'
import { REJECTION_MESSAGES } from '../utils/playerConnectMessages'

// ---------------------------------------------------------------------------
// Tests : VPlayerPage — NAME_TAKEN_OFFLINE sur un rejet de reconnexion
// (#122, F2)
//
// NAME_TAKEN_OFFLINE n'est pas dans REDIRECT_MESSAGES (#120/#123) : il ne
// signale pas une éviction, mais un rejet à la reprise — il doit donc suivre
// le même chemin que NAME_TAKEN/ENROLLMENT_CLOSED (reconnectError, écran
// "Rejoindre à nouveau"), pas le chemin evictedReason. Ce test verrouille ce
// routage, distinct de celui de PLAYER_REMOVED/GAME_RESET (#123 F3).
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

describe('VPlayerPage — NAME_TAKEN_OFFLINE sur un rejet de reconnexion (#122, F2)', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.stubGlobal('localStorage', createStorageMock())
    vi.stubGlobal('sessionStorage', createStorageMock())
    localStorage.setItem('vplayer_name', 'Emma')
    localStorage.setItem('vplayer_session', '1234567890')
    localStorage.setItem('vplayer_id', 'vjoueur_emma_stale')
    useGame.mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('affiche l\'invitation à demander la place, via l\'écran de rejet (pas le motif d\'éviction)', async () => {
    const mock = makeGameMock({ bumpers: {} })
    useGame.mockReturnValue(mock)
    const { container, rerender } = render(<VPlayerPage />)
    await act(async () => {})

    useGame.mockReturnValue(makeGameMock({
      bumpers: {},
      playerConnectStatus: { status: 'rejected', reason: 'NAME_TAKEN_OFFLINE' },
    }))
    rerender(<VPlayerPage />)

    expect(container.querySelector('.vplayer-reconnect-error-text').textContent)
      .toBe(REJECTION_MESSAGES.NAME_TAKEN_OFFLINE)
  })

  it('non-régression : NAME_TAKEN affiche toujours son texte inchangé sur ce même écran', async () => {
    const mock = makeGameMock({ bumpers: {} })
    useGame.mockReturnValue(mock)
    const { container, rerender } = render(<VPlayerPage />)
    await act(async () => {})

    useGame.mockReturnValue(makeGameMock({
      bumpers: {},
      playerConnectStatus: { status: 'rejected', reason: 'NAME_TAKEN' },
    }))
    rerender(<VPlayerPage />)

    expect(container.querySelector('.vplayer-reconnect-error-text').textContent)
      .toBe('Ce pseudo est déjà utilisé, choisis-en un autre')
  })
})
