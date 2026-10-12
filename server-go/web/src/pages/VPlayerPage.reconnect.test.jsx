import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, act } from '@testing-library/react'
import VPlayerPage from './VPlayerPage'
import PlayerDisplay from './PlayerDisplay'
import { useGame } from '../hooks/GameContext'
import { REDIRECT_MESSAGES } from '../utils/playerConnectMessages'

// ---------------------------------------------------------------------------
// Tests : VPlayerPage — reconnexion applicative après coupure réseau (#118)
//
// Plan : _work/reports/plan-20260729-190000.md — tâches F2 (R4), F3 (R1),
// F4, F6 (R3). F1/F5 (useWebSocket) sont couverts par
// useWebSocket.heartbeat.test.js ; F7 (file du buzz) par
// VPlayerPage.buzz-queue.test.jsx.
//
// F2 (R4) — le minuteur de 2s qui précédait tout PLAYER_CONNECT de
// reconnexion n'avait pas de justification une fois l'ID connu : le serveur
// envoie déjà l'état au HELLO. Sur un lien instable (le terrain exact de
// #118), ces 2s pouvaient ne jamais s'écouler à l'intérieur d'une seule
// fenêtre 'connected' ininterrompue, et aucun PLAYER_CONNECT ne partait
// jamais. Le fix émet immédiatement dès que `status` passe à 'connected'
// avec un ID connu, avec une reprise à 2s en filet de sécurité.
//
// F3 (R1) — le minuteur de reconnexion ignorait `reconnectError` et ne
// remettait jamais `playerSession` à `null` : un basculement de `status`
// pendant les 3s de lecture d'un rejet pouvait réarmer un PLAYER_CONNECT
// sans ID, provoquant NAME_TAKEN ou un bumper fantôme.
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

describe('VPlayerPage — F2 : PLAYER_CONNECT immédiat à la reconnexion (#118, R4)', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.stubGlobal('localStorage', createStorageMock())
    vi.stubGlobal('sessionStorage', createStorageMock())
    useGame.mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('session avec ID + bumper non confirmé : PLAYER_CONNECT émis immédiatement, sans attendre 2s', async () => {
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    localStorage.setItem('vplayer_id', 'vjoueur_alice_1')

    const mock = makeGameMock({ bumpers: {} }) // bumper not yet in the roster
    useGame.mockReturnValue(mock)
    render(<VPlayerPage />)

    // No timer advance at all — the send must be synchronous with mount/effect.
    await act(async () => {})

    expect(mock.sendMessage).toHaveBeenCalledWith('PLAYER_CONNECT', { NAME: 'Alice', ID: 'vjoueur_alice_1' })
  })

  it('lien instable : des cycles connected/disconnected de moins de 2s finissent quand même par émettre PLAYER_CONNECT', async () => {
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    localStorage.setItem('vplayer_id', 'vjoueur_alice_1')

    const mock = makeGameMock({ bumpers: {}, status: 'disconnected' })
    useGame.mockReturnValue(mock)
    const { rerender } = render(<VPlayerPage />)
    await act(async () => {})
    expect(mock.sendMessage).not.toHaveBeenCalledWith('PLAYER_CONNECT', expect.anything())

    // Flicker connected for under 2s, then drop again — under the OLD 2s
    // stability timer this could repeat forever without ever sending.
    useGame.mockReturnValue(makeGameMock({ ...mock, status: 'connected' }))
    rerender(<VPlayerPage />)
    await act(async () => { vi.advanceTimersByTime(500) })

    useGame.mockReturnValue(makeGameMock({ ...mock, status: 'disconnected' }))
    rerender(<VPlayerPage />)
    await act(async () => { vi.advanceTimersByTime(500) })

    useGame.mockReturnValue(makeGameMock({ ...mock, status: 'connected' }))
    rerender(<VPlayerPage />)
    await act(async () => {})

    expect(mock.sendMessage).toHaveBeenCalledWith('PLAYER_CONNECT', { NAME: 'Alice', ID: 'vjoueur_alice_1' })
  })

  it('non-régression : premier enrôlement sans ID conserve le comportement #109 (attente de 2s)', async () => {
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    // No vplayer_id — never enrolled from this device before.

    const mock = makeGameMock({ bumpers: {} })
    useGame.mockReturnValue(mock)
    render(<VPlayerPage />)

    await act(async () => {})
    expect(mock.sendMessage).not.toHaveBeenCalledWith('PLAYER_CONNECT', expect.anything())

    await act(async () => { vi.advanceTimersByTime(1999) })
    expect(mock.sendMessage).not.toHaveBeenCalledWith('PLAYER_CONNECT', expect.anything())

    await act(async () => { vi.advanceTimersByTime(2) })
    expect(mock.sendMessage).toHaveBeenCalledWith('PLAYER_CONNECT', { NAME: 'Alice' })
  })
})

describe('VPlayerPage — F3 : gardes du minuteur de reconnexion après un rejet (#118, R1)', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.stubGlobal('localStorage', createStorageMock())
    vi.stubGlobal('sessionStorage', createStorageMock())
    useGame.mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('après un rejet (NAME_TAKEN), un basculement de status ne réémet plus PLAYER_CONNECT sans ID', async () => {
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    localStorage.setItem('vplayer_id', 'vjoueur_alice_stale')

    const mock = makeGameMock({ bumpers: {} })
    useGame.mockReturnValue(mock)
    const { container, rerender } = render(<VPlayerPage />)
    await act(async () => {})
    mock.sendMessage.mockClear()

    useGame.mockReturnValue(makeGameMock({
      ...mock,
      playerConnectStatus: { status: 'rejected', reason: 'NAME_TAKEN' },
    }))
    rerender(<VPlayerPage />)

    expect(container.querySelector('.vplayer-reconnect-error-text')).not.toBeNull()

    // status flickers during the 3s read delay — the OLD bug re-armed the
    // reconnect timer here and sent PLAYER_CONNECT without an ID (localStorage
    // no longer has one after clearVPlayerSession — see F4/#120).
    // Reuse mock.sendMessage (not a fresh vi.fn()) on every subsequent
    // mockReturnValue: the assertion below checks THAT specific spy, so a
    // fresh one here would make it pass trivially regardless of the bug.
    useGame.mockReturnValue(makeGameMock({ ...mock, bumpers: {}, status: 'disconnected' }))
    rerender(<VPlayerPage />)
    useGame.mockReturnValue(makeGameMock({ ...mock, bumpers: {}, status: 'connected' }))
    rerender(<VPlayerPage />)

    await act(async () => { vi.advanceTimersByTime(3000) })

    expect(mock.sendMessage).not.toHaveBeenCalledWith('PLAYER_CONNECT', expect.anything())
  })
})

describe('VPlayerPage — F4 : bandeau de connexion (#118)', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.stubGlobal('localStorage', createStorageMock())
    vi.stubGlobal('sessionStorage', createStorageMock())
    useGame.mockReset()
    PlayerDisplay.mockClear()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  function renderInGame(overrides = {}) {
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    localStorage.setItem('vplayer_id', 'vjoueur_alice_1')

    const mock = makeGameMock({
      bumpers: {
        vjoueur_alice_1: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true },
      },
      ...overrides,
    })
    useGame.mockReturnValue(mock)
    return { mock, ...render(<VPlayerPage />) }
  }

  it('aucun bandeau en fonctionnement normal', async () => {
    const { container } = renderInGame()
    await act(async () => {})
    expect(container.querySelector('.vplayer-connection-banner')).toBeNull()
  })

  it('bandeau orange puis vert 2s au retour, sans jamais désactiver le buzzer', async () => {
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    localStorage.setItem('vplayer_id', 'vjoueur_alice_1')

    const bumpers = {
      vjoueur_alice_1: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true },
    }
    const mock = makeGameMock({ bumpers })
    useGame.mockReturnValue(mock)
    const { container, rerender } = render(<VPlayerPage />)
    await act(async () => {})
    expect(container.querySelector('.vplayer-connection-banner')).toBeNull()

    // Guard-fail test (explicit, per plan): the buzzer must NEVER carry a
    // `disabled` prop/attribute, in ANY of the three connection states — this
    // locks against a future reintroduction of the disabled button the user
    // explicitly ruled out (arbitrage: the banner informs, it never blocks).
    const assertBuzzerNeverDisabled = () => {
      const lastCall = PlayerDisplay.mock.calls[PlayerDisplay.mock.calls.length - 1]
      expect(lastCall[0].onMediaClick).toBeTypeOf('function')
      expect(lastCall[0]).not.toHaveProperty('disabled', true)
      expect(Object.keys(lastCall[0])).not.toContain('disabled')
    }
    assertBuzzerNeverDisabled() // normal state

    // Link drops.
    useGame.mockReturnValue(makeGameMock({ bumpers, status: 'disconnected' }))
    rerender(<VPlayerPage />)
    const lostBanner = container.querySelector('.vplayer-connection-banner')
    expect(lostBanner).not.toBeNull()
    expect(lostBanner.className).toContain('lost')
    assertBuzzerNeverDisabled() // lost-connection state

    // Link comes back.
    useGame.mockReturnValue(makeGameMock({ bumpers, status: 'connected' }))
    rerender(<VPlayerPage />)
    const restoredBanner = container.querySelector('.vplayer-connection-banner')
    expect(restoredBanner).not.toBeNull()
    expect(restoredBanner.className).toContain('restored')
    assertBuzzerNeverDisabled() // restored state

    await act(async () => { vi.advanceTimersByTime(2000) })
    expect(container.querySelector('.vplayer-connection-banner')).toBeNull()
    assertBuzzerNeverDisabled() // back to normal
  })
})

describe('VPlayerPage — F6/F3 : motif de rejet relayé tel quel, jamais déduit (#118 R3 puis #123 cause B)', () => {
  // #123 a supprimé la déduction ENROLLMENT_CLOSED → GAME_RESET introduite par
  // #118 F6 : le commentaire d'origine disait lui-même « le plus souvent »,
  // et une suppression individuelle produit exactement le même
  // ENROLLMENT_CLOSED qu'une purge NEW_GAME — affichant alors à tort « une
  // nouvelle partie a commencé ». Le registre de motifs de #123 (B3,
  // dev-backend) fait qu'un ID connu comme supprimé reçoit directement
  // PLAYER_REMOVED ou GAME_RESET du serveur ; le client se contente de
  // relayer ce motif reçu, sans plus jamais le déduire.
  beforeEach(() => {
    vi.useFakeTimers()
    vi.stubGlobal('localStorage', createStorageMock())
    vi.stubGlobal('sessionStorage', createStorageMock())
    useGame.mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('un rejet ENROLLMENT_CLOSED affiche désormais son sens littéral, jamais "nouvelle partie" (#123 F3)', async () => {
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    localStorage.setItem('vplayer_id', 'vjoueur_alice_stale')

    const mock = makeGameMock({ bumpers: {} })
    useGame.mockReturnValue(mock)
    const { container, rerender } = render(<VPlayerPage />)
    await act(async () => {})

    useGame.mockReturnValue(makeGameMock({
      bumpers: {},
      playerConnectStatus: { status: 'rejected', reason: 'ENROLLMENT_CLOSED' },
    }))
    rerender(<VPlayerPage />)

    expect(container.querySelector('.vplayer-reconnect-error-text').textContent)
      .toBe('Les inscriptions sont fermées')
    expect(container.querySelector('.vplayer-reconnect-error-text').textContent)
      .not.toBe(REDIRECT_MESSAGES.GAME_RESET)
  })

  it('un rejet PLAYER_REMOVED (registre de motifs #123) affiche "place libérée", relayé tel quel', async () => {
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    localStorage.setItem('vplayer_id', 'vjoueur_alice_stale')

    const mock = makeGameMock({ bumpers: {} })
    useGame.mockReturnValue(mock)
    const { container, rerender } = render(<VPlayerPage />)
    await act(async () => {})

    useGame.mockReturnValue(makeGameMock({
      bumpers: {},
      playerConnectStatus: { status: 'rejected', reason: 'PLAYER_REMOVED' },
    }))
    rerender(<VPlayerPage />)

    expect(container.querySelector('.vplayer-reconnect-error-text').textContent)
      .toBe(REDIRECT_MESSAGES.PLAYER_REMOVED)
  })

  it('un rejet GAME_RESET (registre de motifs #123, purge NEW_GAME réelle) affiche "nouvelle partie"', async () => {
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    localStorage.setItem('vplayer_id', 'vjoueur_alice_stale')

    const mock = makeGameMock({ bumpers: {} })
    useGame.mockReturnValue(mock)
    const { container, rerender } = render(<VPlayerPage />)
    await act(async () => {})

    useGame.mockReturnValue(makeGameMock({
      bumpers: {},
      playerConnectStatus: { status: 'rejected', reason: 'GAME_RESET' },
    }))
    rerender(<VPlayerPage />)

    expect(container.querySelector('.vplayer-reconnect-error-text').textContent)
      .toBe(REDIRECT_MESSAGES.GAME_RESET)
  })

  it('les autres motifs de rejet (NAME_TAKEN) restent affichés via le message générique existant (non-régression #109)', async () => {
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    localStorage.setItem('vplayer_id', 'vjoueur_alice_stale')

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
