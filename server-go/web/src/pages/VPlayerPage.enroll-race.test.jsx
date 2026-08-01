import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, act, fireEvent } from '@testing-library/react'
import VPlayerPage from './VPlayerPage'
import { useGame } from '../hooks/GameContext'
import { REDIRECT_MESSAGES, DEFAULT_REDIRECT_MESSAGE } from '../utils/playerConnectMessages'

// ---------------------------------------------------------------------------
// Tests : VPlayerPage — fermeture de la course d'inscription VJoueur (#120)
//
// Cause racine A du plan (_work/reports/plan-20260728-101500.md) : la page
// montait AVANT l'arrivée de l'UPDATE listant le bumper fraîchement créé — le
// roster local était par construction encore celui d'avant l'inscription. La
// détection par balayage de roster ("le bumper n'y est pas -> j'ai été
// supprimé") était donc systématiquement fausse pendant cette fenêtre, dès
// qu'un autre VJoueur ou buzzer physique était déjà présent (seul le tout
// premier inscrit, avec un roster vide, échappait à la course).
//
// Le fix (F1/F2/F3/F5) : plus aucune déduction depuis `bumpers` — seule
// l'action serveur PLAYER_EVICTED fait autorité, l'identité se vérifie par ID
// (repli sur le nom pour une session antérieure), et l'absence du bumper
// affiche un état d'attente purement visuel (F5) au lieu de l'écran de jeu
// incomplet ou d'un renvoi.
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
  gameState: { phase: 'STOPPED', question: null, enrollmentActive: true },
  bumpers: {},
  teams: {},
  status: 'connected',
  playerConnectStatus: null,
  clearPlayerConnectStatus: vi.fn(),
  playerEvictedStatus: null,
  clearPlayerEvictedStatus: vi.fn(),
  ...overrides,
})

describe('VPlayerPage — la course d\'inscription est fermée (#120)', () => {
  let localStorageMock
  let sessionStorageMock

  beforeEach(() => {
    vi.useFakeTimers()
    localStorageMock = createStorageMock()
    sessionStorageMock = createStorageMock()
    vi.stubGlobal('localStorage', localStorageMock)
    vi.stubGlobal('sessionStorage', sessionStorageMock)
    useGame.mockReset()
    // Pre-existing gap found while adding the fast-follow race test below:
    // `mockNavigate` is module-level (vi.hoisted) and was never cleared
    // between tests in this file, unlike EnrollPage.test.jsx's equivalent
    // mock — any earlier test that navigates left calls visible to later
    // `expect(mockNavigate).not.toHaveBeenCalled()` assertions (order-
    // dependent flake, not a production bug).
    mockNavigate.mockClear()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  // -------------------------------------------------------------------------
  // Le test de la course — le plus important
  // -------------------------------------------------------------------------

  it('roster non vide ne contenant pas encore notre bumper : aucune redirection, aucune session effacée', async () => {
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    localStorage.setItem('vplayer_id', 'vjoueur_alice_1')

    // Roster already non-empty — someone else, NOT our own bumper. This is
    // exactly the scenario that used to trip the old roster-scan detection.
    const mock = makeGameMock({
      bumpers: {
        vjoueur_bob: { NAME: 'Bob', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true },
      },
    })
    useGame.mockReturnValue(mock)

    const { container } = render(<VPlayerPage />)

    // Let effects settle, but stay well under the 2s reconnect timer and the
    // 10s safety net so neither confuses this assertion.
    await act(async () => {
      vi.advanceTimersByTime(100)
    })

    expect(container.querySelector('.vplayer-connecting')).not.toBeNull()
    expect(mockNavigate).not.toHaveBeenCalled()
    expect(localStorage.getItem('vplayer_name')).toBe('Alice')
    expect(localStorage.getItem('vplayer_session')).toBe('1234567890')
    expect(localStorage.getItem('vplayer_id')).toBe('vjoueur_alice_1')
  })

  it('l\'arrivée ultérieure de l\'UPDATE porteur du bumper le fait apparaître normalement', async () => {
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    localStorage.setItem('vplayer_id', 'vjoueur_alice_1')

    const mock = makeGameMock({
      bumpers: {
        vjoueur_bob: { NAME: 'Bob', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true },
      },
    })
    useGame.mockReturnValue(mock)
    const { container, rerender } = render(<VPlayerPage />)

    await act(async () => { vi.advanceTimersByTime(100) })
    expect(container.querySelector('.vplayer-connecting')).not.toBeNull()

    // The UPDATE that follows PLAYER_CONNECTED finally lists our own bumper.
    useGame.mockReturnValue(makeGameMock({
      bumpers: {
        vjoueur_bob: { NAME: 'Bob', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true },
        vjoueur_alice_1: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true },
      },
    }))
    rerender(<VPlayerPage />)

    expect(container.querySelector('.vplayer-connecting')).toBeNull()
    expect(mockNavigate).not.toHaveBeenCalled()
  })

  // -------------------------------------------------------------------------
  // F5 — état d'attente
  // -------------------------------------------------------------------------

  it('F5 : affiche l\'état d\'attente (pas l\'écran de jeu) tant que le bumper est absent', async () => {
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')

    useGame.mockReturnValue(makeGameMock({ bumpers: {} }))
    const { container } = render(<VPlayerPage />)

    await act(async () => { vi.advanceTimersByTime(50) })

    expect(container.querySelector('.vplayer-connecting')).not.toBeNull()
    expect(container.textContent).toContain('Connexion à la partie')
  })

  it('F5 : l\'état d\'attente ne déclenche jamais de redirection ni d\'effacement de session (jusqu\'à l\'instant précédant le filet de sécurité 10s)', async () => {
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    localStorage.setItem('vplayer_id', 'vjoueur_alice_1')

    useGame.mockReturnValue(makeGameMock({ bumpers: {} }))
    const { container } = render(<VPlayerPage />)

    // Advance well past the 2s reconnect timer (harmless, unrelated to F5)
    // but stay under the 10s SESSION_EXPIRED safety net — this test is about
    // the WAITING STATE ITSELF never causing a redirect, not about disproving
    // the safety net (a distinct, intentionally-defensive escape hatch).
    await act(async () => {
      vi.advanceTimersByTime(9999)
    })

    expect(container.querySelector('.vplayer-connecting')).not.toBeNull()
    expect(mockNavigate).not.toHaveBeenCalled()
    expect(localStorage.getItem('vplayer_name')).toBe('Alice')
  })

  // -------------------------------------------------------------------------
  // PLAYER_EVICTED
  // -------------------------------------------------------------------------

  it('PLAYER_EVICTED : affiche le motif transmis, efface la session et redirige après le délai', async () => {
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    localStorage.setItem('vplayer_id', 'vjoueur_alice_1')

    const clearPlayerEvictedStatus = vi.fn()
    const mock = makeGameMock({
      bumpers: {
        vjoueur_alice_1: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true },
      },
    })
    useGame.mockReturnValue(mock)
    const { container, rerender } = render(<VPlayerPage />)
    await act(async () => { vi.advanceTimersByTime(10) })
    expect(container.querySelector('.vplayer-connecting')).toBeNull() // game screen was showing

    useGame.mockReturnValue(makeGameMock({
      ...mock,
      playerEvictedStatus: { reason: 'PLAYER_REMOVED' },
      clearPlayerEvictedStatus,
    }))
    rerender(<VPlayerPage />)

    expect(clearPlayerEvictedStatus).toHaveBeenCalled()
    expect(container.querySelector('.vplayer-reconnect-error-text').textContent)
      .toBe(REDIRECT_MESSAGES.PLAYER_REMOVED)

    // Session and navigation both wait for handleEvictedRedirect, fired either
    // by the read-delay timer or the button — neither has run yet.
    expect(localStorage.getItem('vplayer_name')).toBe('Alice')
    expect(mockNavigate).not.toHaveBeenCalled()

    await act(async () => {
      vi.advanceTimersByTime(3000)
    })

    expect(mockNavigate).toHaveBeenCalledWith('/')
    expect(sessionStorage.getItem('vplayer_redirect_reason')).toBe('PLAYER_REMOVED')
    // handleEvictedRedirect clears all 3 keys via clearVPlayerSession (F4).
    expect(localStorage.getItem('vplayer_name')).toBeNull()
    expect(localStorage.getItem('vplayer_session')).toBeNull()
    expect(localStorage.getItem('vplayer_id')).toBeNull()
  })

  it('bumper jamais confirmé + PLAYER_EVICTED reçu avant l\'échéance des 2s du minuteur de reconnexion : aucun PLAYER_CONNECT résiduel (fast-follow code-review [MAJEUR])', async () => {
    // Combinaison non couverte auparavant : le minuteur de reconnexion hérité
    // de #109 (2s, ignore evictedReason) et l'éviction (session effacée
    // seulement après le délai de lecture de 3s) sont tous deux en course.
    // Sans le fix, ce minuteur retrouve vplayer_id encore présent à 2s et
    // renvoie PLAYER_CONNECT avec un ID déjà supprimé côté serveur — qui le
    // traite comme absent et recrée un VJoueur fantôme sous le même nom.
    //
    // Mis à jour pour #118 (F2, R4) : un ID connu déclenche désormais un
    // PLAYER_CONNECT IMMÉDIAT au montage (plus d'attente de 2s) ; seule la
    // REPRISE de 2s (couvrant un message perdu) est encore en course avec
    // l'éviction — c'est elle que ce test vérifie maintenant comme annulée.
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    localStorage.setItem('vplayer_id', 'vjoueur_alice_1')

    const sendMessage = vi.fn()
    // Bumper never confirmed: absent from the roster on every render.
    const mock = makeGameMock({ sendMessage, bumpers: {} })
    useGame.mockReturnValue(mock)
    const { rerender } = render(<VPlayerPage />)

    // #118 F2 : envoi immédiat au montage, un ID étant déjà connu.
    expect(sendMessage).toHaveBeenCalledTimes(1)
    expect(sendMessage).toHaveBeenCalledWith('PLAYER_CONNECT', { NAME: 'Alice', ID: 'vjoueur_alice_1' })

    // Just before the 2000ms retry's deadline.
    await act(async () => { vi.advanceTimersByTime(1500) })
    expect(sendMessage).toHaveBeenCalledTimes(1) // still just the immediate send

    // PLAYER_EVICTED arrives with 500ms left on the retry timer.
    useGame.mockReturnValue(makeGameMock({
      sendMessage,
      bumpers: {},
      playerEvictedStatus: { reason: 'PLAYER_REMOVED' },
    }))
    rerender(<VPlayerPage />)

    // vplayer_id is cleared immediately — does not wait for the 3s read delay.
    expect(localStorage.getItem('vplayer_id')).toBeNull()

    // Past where the 2000ms retry would have fired: still just the one
    // immediate call, because evictedReason cancelled the retry.
    await act(async () => { vi.advanceTimersByTime(1000) }) // t=2500ms
    expect(sendMessage).toHaveBeenCalledTimes(1)

    // Past the eviction redirect delay too (armed at t=1500, fires at
    // t=1500+3000=4500): still nothing residual sent, and the normal
    // eviction redirect proceeds as expected.
    await act(async () => { vi.advanceTimersByTime(2000) }) // t=4500ms
    expect(sendMessage).toHaveBeenCalledTimes(1)
    expect(mockNavigate).toHaveBeenCalledWith('/')
  })

  it('PLAYER_EVICTED{GAME_RESET} affiche le motif "nouvelle partie"', async () => {
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')

    useGame.mockReturnValue(makeGameMock({
      bumpers: {},
      playerEvictedStatus: { reason: 'GAME_RESET' },
    }))
    const { container } = render(<VPlayerPage />)

    expect(container.querySelector('.vplayer-reconnect-error-text').textContent)
      .toBe(REDIRECT_MESSAGES.GAME_RESET)
  })

  it('un motif inconnu ou absent affiche le message générique (filet de sécurité, jamais d\'écran muet)', async () => {
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')

    useGame.mockReturnValue(makeGameMock({
      bumpers: {},
      playerEvictedStatus: { reason: 'SOME_FUTURE_REASON' },
    }))
    const { container } = render(<VPlayerPage />)

    expect(container.querySelector('.vplayer-reconnect-error-text').textContent)
      .toBe(DEFAULT_REDIRECT_MESSAGE)
  })

  it('le bouton "Rejoindre à nouveau" redirige immédiatement, sans attendre le délai', async () => {
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')

    useGame.mockReturnValue(makeGameMock({
      bumpers: {},
      playerEvictedStatus: { reason: 'PLAYER_REMOVED' },
    }))
    const { container } = render(<VPlayerPage />)

    fireEvent.click(container.querySelector('.vplayer-reconnect-btn'))

    expect(mockNavigate).toHaveBeenCalledWith('/')
  })

  // -------------------------------------------------------------------------
  // F2 — identité par ID, repli sur le nom
  // -------------------------------------------------------------------------

  it('recherche le bumper par ID : un changement de nom côté serveur ne provoque plus de renvoi', async () => {
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    localStorage.setItem('vplayer_id', 'vjoueur_alice_1')

    // The server-side name no longer matches the locally-stored name — under
    // the old name-based matching this would have looked like "not found".
    useGame.mockReturnValue(makeGameMock({
      bumpers: {
        vjoueur_alice_1: { NAME: 'AliceRenamed', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true },
      },
    }))
    const { container } = render(<VPlayerPage />)
    await act(async () => { vi.advanceTimersByTime(100) })

    expect(container.querySelector('.vplayer-connecting')).toBeNull() // bumper WAS found, by ID
    expect(mockNavigate).not.toHaveBeenCalled()
  })

  it('une session sans ID (antérieure à #120) retombe sur la recherche par nom', async () => {
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    // No vplayer_id stored — pre-#120 session.

    useGame.mockReturnValue(makeGameMock({
      bumpers: {
        some_legacy_key: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true },
      },
    }))
    const { container } = render(<VPlayerPage />)
    await act(async () => { vi.advanceTimersByTime(100) })

    expect(container.querySelector('.vplayer-connecting')).toBeNull() // found via name fallback
  })

  // -------------------------------------------------------------------------
  // F4 — nettoyage mutualisé de session
  // -------------------------------------------------------------------------

  it('un rejet de reconnexion efface bien les 3 clés de session (pas seulement vplayer_id)', async () => {
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    localStorage.setItem('vplayer_id', 'vjoueur_alice_stale')

    useGame.mockReturnValue(makeGameMock({ bumpers: {} }))
    const { rerender } = render(<VPlayerPage />)
    await act(async () => { vi.advanceTimersByTime(10) })

    useGame.mockReturnValue(makeGameMock({
      bumpers: {},
      playerConnectStatus: { status: 'rejected', reason: 'NAME_TAKEN' },
    }))
    rerender(<VPlayerPage />)

    expect(localStorage.getItem('vplayer_name')).toBeNull()
    expect(localStorage.getItem('vplayer_session')).toBeNull()
    expect(localStorage.getItem('vplayer_id')).toBeNull()
  })
})
