import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, act } from '@testing-library/react'
import VPlayerPage from './VPlayerPage'
import { useGame } from '../hooks/GameContext'

// ---------------------------------------------------------------------------
// Tests : VPlayerPage — reconnexion par ID (fix R1, #109)
//
// La reconnexion auto (2s après montage si le bumper n'est pas retrouvé par
// nom) doit désormais renvoyer l'ID stocké en localStorage (`vplayer_id`,
// capturé depuis un PLAYER_CONNECTED précédent) en plus du nom — sans ID
// stocké (jamais connecté depuis cet appareil), l'ancien comportement
// (nom seul) reste le fallback. Un rejet (`playerConnectStatus.status ===
// 'rejected'`) doit bloquer l'écran sur un message d'erreur + bouton pour
// repartir sur EnrollPage, et purger `vplayer_id` (ne jamais retenter avec
// un ID désormais invalide).
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

vi.mock('./PlayerDisplay', () => ({ default: () => null }))
vi.mock('../components/ArdoiseKeyboard', () => ({ default: () => null }))

// localStorage mock — voir EnrollPage.test.jsx pour le contexte (global
// `localStorage` inerte dans cet environnement de test).
function createLocalStorageMock() {
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
  gameState: { phase: 'STOPPED', question: null },
  bumpers: {},
  teams: {},
  status: 'connected',
  playerConnectStatus: null,
  clearPlayerConnectStatus: vi.fn(),
  ...overrides,
})

describe('VPlayerPage — reconnexion auto renvoie l\'ID stocké', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mockNavigate.mockClear()
    vi.stubGlobal('localStorage', createLocalStorageMock())
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    useGame.mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('renvoie NAME + ID quand un vplayer_id est stocké (bumper introuvable par nom)', async () => {
    localStorage.setItem('vplayer_id', 'vjoueur_alice_123')
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)

    render(<VPlayerPage />)

    await act(async () => {
      vi.advanceTimersByTime(2000)
    })

    expect(mock.sendMessage).toHaveBeenCalledWith('PLAYER_CONNECT', { NAME: 'Alice', ID: 'vjoueur_alice_123' })
  })

  it("renvoie NAME seul (pas de clé ID) quand vplayer_id est absent", async () => {
    // No vplayer_id in localStorage — never connected from this device before.
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)

    render(<VPlayerPage />)

    await act(async () => {
      vi.advanceTimersByTime(2000)
    })

    expect(mock.sendMessage).toHaveBeenCalledWith('PLAYER_CONNECT', { NAME: 'Alice' })
    const [, payload] = mock.sendMessage.mock.calls[0]
    expect(payload).not.toHaveProperty('ID')
  })

  it('ne tente pas de reconnexion si le bumper est déjà retrouvé par nom', async () => {
    const mock = makeGameMock({
      bumpers: { vjoueur_alice_123: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, TEAM: '' } },
    })
    useGame.mockReturnValue(mock)

    render(<VPlayerPage />)

    await act(async () => {
      vi.advanceTimersByTime(2000)
    })

    expect(mock.sendMessage).not.toHaveBeenCalledWith('PLAYER_CONNECT', expect.anything())
  })
})

describe('VPlayerPage — rejet de reconnexion (ID périmé / NAME_TAKEN)', () => {
  beforeEach(() => {
    mockNavigate.mockClear()
    vi.stubGlobal('localStorage', createLocalStorageMock())
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    localStorage.setItem('vplayer_id', 'vjoueur_alice_stale')
    useGame.mockReset()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("affiche un message d'erreur bloquant et purge vplayer_id sur rejet", async () => {
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)
    const { rerender } = render(<VPlayerPage />)

    useGame.mockReturnValue(makeGameMock({
      ...mock,
      playerConnectStatus: { status: 'rejected', reason: 'NAME_TAKEN' },
    }))
    rerender(<VPlayerPage />)

    await waitFor(() =>
      expect(screen.getByText('Ce pseudo est déjà utilisé, choisis-en un autre')).toBeInTheDocument()
    )
    expect(localStorage.getItem('vplayer_id')).toBeNull()
    expect(screen.getByRole('button', { name: /Rejoindre à nouveau/i })).toBeInTheDocument()
  })

  it("'Rejoindre à nouveau' nettoie toute la session locale et renvoie vers l'accueil", async () => {
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)
    const { rerender } = render(<VPlayerPage />)

    useGame.mockReturnValue(makeGameMock({
      ...mock,
      playerConnectStatus: { status: 'rejected', reason: 'NAME_TAKEN' },
    }))
    rerender(<VPlayerPage />)

    await waitFor(() => screen.getByRole('button', { name: /Rejoindre à nouveau/i }))
    screen.getByRole('button', { name: /Rejoindre à nouveau/i }).click()

    expect(localStorage.getItem('vplayer_name')).toBeNull()
    expect(localStorage.getItem('vplayer_session')).toBeNull()
    expect(localStorage.getItem('vplayer_id')).toBeNull()
    expect(mockNavigate).toHaveBeenCalledWith('/')
  })

  it('consomme playerConnectStatus (clearPlayerConnectStatus appelé) après un rejet', async () => {
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)
    const { rerender } = render(<VPlayerPage />)

    const rejectedMock = makeGameMock({
      ...mock,
      playerConnectStatus: { status: 'rejected', reason: 'NAME_TAKEN' },
    })
    useGame.mockReturnValue(rejectedMock)
    rerender(<VPlayerPage />)

    await waitFor(() => expect(rejectedMock.clearPlayerConnectStatus).toHaveBeenCalled())
  })
})
