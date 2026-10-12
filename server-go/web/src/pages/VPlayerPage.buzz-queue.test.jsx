import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, act, fireEvent } from '@testing-library/react'
import VPlayerPage from './VPlayerPage'
import { useGame } from '../hooks/GameContext'

// ---------------------------------------------------------------------------
// Tests : VPlayerPage — file d'attente du buzz pendant une coupure (#118, F7)
//
// Plan : _work/reports/plan-20260729-190000.md — tâche F7. Arbitrage
// utilisateur : le buzzer reste actif pendant la coupure (jamais désactivé,
// cf. VPlayerPage.reconnect.test.jsx pour le garde-fou F4), mais l'appui ne
// peut évidemment pas atteindre le serveur. Il est mémorisé (un seul, le
// premier) et envoyé à la reconnexion — SAUF si le contexte a changé.
//
// **Le point de vigilance n'est pas la file elle-même, c'est la validation de
// contexte au moment du vidage** : un client hors ligne pendant TOUT un
// changement de question n'observe jamais la phase PREPARE — le déclencheur
// (a) (purge sur PREPARE observé) ne peut donc pas s'appliquer. Seul le
// déclencheur (b) (comparaison de l'identité de question au moment du
// vidage) couvre ce cas, qui est le scénario central de cette suite.
//
// Convention importante : toute mise à jour du mock via
// useGame.mockReturnValue(...) après le rendu initial part de `{ ...mock,
// ... }` — jamais un makeGameMock() nu — pour conserver LE MÊME spy
// `sendMessage` d'un rerender à l'autre. Un spy fraîchement recréé rendrait
// les assertions sur `mock.sendMessage` aveugles à l'envoi réel (elles
// vérifieraient un objet différent de celui que le composant appelle).
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

// Interactive stub: exposes a clickable element invoking onMediaClick, so
// tests can simulate a buzz press without depending on PlayerDisplay's own
// internals (mirrors the ArdoiseKeyboard interactive-stub technique from
// VPlayerPage.ardoise-immediate.test.jsx, #117).
vi.mock('./PlayerDisplay', () => ({
  default: ({ onMediaClick }) => (
    <div data-testid="buzz-target" onClick={() => onMediaClick && onMediaClick()} />
  ),
}))
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
  gameState: { phase: 'STARTED', question: { ID: 'q1', TYPE: 'NORMAL' }, enrollmentActive: true },
  bumpers: {
    [BUMPER_ID]: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true },
  },
  teams: {},
  status: 'connected',
  playerConnectStatus: null,
  clearPlayerConnectStatus: vi.fn(),
  playerEvictedStatus: null,
  clearPlayerEvictedStatus: vi.fn(),
  ...overrides,
})

function pressBuzz(container) {
  fireEvent.click(container.querySelector('[data-testid="buzz-target"]'))
}

describe('VPlayerPage — file d\'attente du buzz pendant une coupure (#118, F7)', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.stubGlobal('localStorage', createStorageMock())
    vi.stubGlobal('sessionStorage', createStorageMock())
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    localStorage.setItem('vplayer_id', BUMPER_ID)
    useGame.mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('un appui pendant la coupure est mémorisé et n\'est PAS envoyé immédiatement', async () => {
    const mock = makeGameMock({ status: 'disconnected' })
    useGame.mockReturnValue(mock)
    const { container } = render(<VPlayerPage />)
    await act(async () => {})

    pressBuzz(container)

    expect(mock.sendMessage).not.toHaveBeenCalledWith('BUTTON', expect.anything())
  })

  it('reconnexion sur la MÊME question toujours STARTED : le buzz mémorisé est envoyé une fois, identique à un buzz normal', async () => {
    const mock = makeGameMock({ status: 'disconnected' })
    useGame.mockReturnValue(mock)
    const { container, rerender } = render(<VPlayerPage />)
    await act(async () => {})
    pressBuzz(container)
    expect(mock.sendMessage).not.toHaveBeenCalledWith('BUTTON', expect.anything())

    // Reconnection confirmed: status connected AND bumper.CONNECTED back to
    // true — via a FRESH `bumpers` object (a new reference, exactly like a
    // real UPDATE broadcast produces via JSON.parse). Reusing the same
    // reference here would never re-trigger the flush effect: its
    // dependency is `bumper` (state), only recomputed when the `bumpers`
    // PROP reference changes — reusing `...mock`'s original bumpers object
    // would leave `bumper` untouched and the effect would never re-run.
    useGame.mockReturnValue(makeGameMock({
      ...mock,
      status: 'connected',
      bumpers: { [BUMPER_ID]: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true } },
      gameState: { phase: 'STARTED', question: { ID: 'q1', TYPE: 'NORMAL' }, enrollmentActive: true },
    }))
    rerender(<VPlayerPage />)

    expect(mock.sendMessage).toHaveBeenCalledTimes(1)
    // Strictly identical to a normal buzz — no client timestamp field added.
    expect(mock.sendMessage).toHaveBeenCalledWith('BUTTON', { ID: BUMPER_ID, button: 'A' })
  })

  it('plusieurs appuis pendant la même coupure : un seul buzz est envoyé (le premier)', async () => {
    const mock = makeGameMock({ status: 'disconnected' })
    useGame.mockReturnValue(mock)
    const { container, rerender } = render(<VPlayerPage />)
    await act(async () => {})

    pressBuzz(container)
    pressBuzz(container)
    pressBuzz(container)

    // Fresh `bumpers` reference — see comment in the test above: required for
    // the flush effect (keyed on `bumper` state) to actually re-run.
    useGame.mockReturnValue(makeGameMock({
      ...mock,
      status: 'connected',
      bumpers: { [BUMPER_ID]: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true } },
    }))
    rerender(<VPlayerPage />)

    expect(mock.sendMessage).toHaveBeenCalledTimes(1)
  })

  it('déclencheur (a) : passage observé en PREPARE (même question rejouée) vide la file SANS envoi', async () => {
    // Isolates trigger (a) from trigger (b): the question ID does NOT change
    // (a "replay" of the same question) — trigger (b)'s mismatch guard alone
    // would NOT have caught this, only the explicit PREPARE-observed purge does.
    const mock = makeGameMock({ status: 'disconnected' })
    useGame.mockReturnValue(mock)
    const { container, rerender } = render(<VPlayerPage />)
    await act(async () => {})
    pressBuzz(container)

    // Reconnects (status back), but the bumper's CONNECTED flag hasn't been
    // reconfirmed by the server yet — the pending buzz is still sitting there.
    useGame.mockReturnValue(makeGameMock({
      ...mock,
      status: 'connected',
      bumpers: { [BUMPER_ID]: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: false } },
      gameState: { phase: 'STARTED', question: { ID: 'q1', TYPE: 'NORMAL' }, enrollmentActive: true },
    }))
    rerender(<VPlayerPage />)
    expect(mock.sendMessage).not.toHaveBeenCalledWith('BUTTON', expect.anything())

    // The client is caught up enough to observe PREPARE (same question ID
    // being replayed) BEFORE reconnection ever fully completes.
    useGame.mockReturnValue(makeGameMock({
      ...mock,
      status: 'connected',
      bumpers: { [BUMPER_ID]: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: false } },
      gameState: { phase: 'PREPARE', question: { ID: 'q1', TYPE: 'NORMAL' }, enrollmentActive: true },
    }))
    rerender(<VPlayerPage />)

    // Now reconnection fully completes, on the SAME question ID — if trigger
    // (a) had not already purged the queue, trigger (b)'s ID match would
    // have let it through (ID is unchanged). It must NOT be sent.
    useGame.mockReturnValue(makeGameMock({
      ...mock,
      status: 'connected',
      gameState: { phase: 'STARTED', question: { ID: 'q1', TYPE: 'NORMAL' }, enrollmentActive: true },
    }))
    rerender(<VPlayerPage />)

    expect(mock.sendMessage).not.toHaveBeenCalledWith('BUTTON', expect.anything())
  })

  it('déclencheur (b) — scénario central : hors ligne pendant TOUT le changement de question (PREPARE jamais observé) → buzz abandonné sur la nouvelle question', async () => {
    const mock = makeGameMock({
      status: 'disconnected',
      gameState: { phase: 'STARTED', question: { ID: 'q1', TYPE: 'NORMAL' }, enrollmentActive: true },
    })
    useGame.mockReturnValue(mock)
    const { container, rerender } = render(<VPlayerPage />)
    await act(async () => {})
    pressBuzz(container)
    expect(mock.sendMessage).not.toHaveBeenCalledWith('BUTTON', expect.anything())

    // The client is offline through the ENTIRE question change — it never
    // observes PREPARE at all (no intermediate render for it, exactly as a
    // genuinely dead link would never deliver that state). It reconnects
    // directly onto question q2, already STARTED.
    useGame.mockReturnValue(makeGameMock({
      ...mock,
      status: 'connected',
      gameState: { phase: 'STARTED', question: { ID: 'q2', TYPE: 'NORMAL' }, enrollmentActive: true },
    }))
    rerender(<VPlayerPage />)

    // The stale buzz must NEVER be sent on the new question.
    expect(mock.sendMessage).not.toHaveBeenCalledWith('BUTTON', expect.anything())

    // And it must not linger either — further re-renders never resurrect it.
    await act(async () => { vi.advanceTimersByTime(5000) })
    rerender(<VPlayerPage />)
    expect(mock.sendMessage).not.toHaveBeenCalledWith('BUTTON', expect.anything())
  })

  it('reconnexion sur la même question mais phase différente de STARTED : abandon silencieux', async () => {
    const mock = makeGameMock({ status: 'disconnected' })
    useGame.mockReturnValue(mock)
    const { container, rerender } = render(<VPlayerPage />)
    await act(async () => {})
    pressBuzz(container)

    // Fresh `bumpers` reference so the flush effect (keyed on `bumper` state)
    // actually re-runs and genuinely exercises the phase guard below, rather
    // than passing vacuously because it never fired at all.
    useGame.mockReturnValue(makeGameMock({
      ...mock,
      status: 'connected',
      bumpers: { [BUMPER_ID]: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true } },
      gameState: { phase: 'REVEALED', question: { ID: 'q1', TYPE: 'NORMAL' }, enrollmentActive: true },
    }))
    rerender(<VPlayerPage />)

    expect(mock.sendMessage).not.toHaveBeenCalledWith('BUTTON', expect.anything())
  })

  it('la file est vide après un envoi réussi : aucun renvoi ultérieur', async () => {
    const mock = makeGameMock({ status: 'disconnected' })
    useGame.mockReturnValue(mock)
    const { container, rerender } = render(<VPlayerPage />)
    await act(async () => {})
    pressBuzz(container)

    useGame.mockReturnValue(makeGameMock({
      ...mock,
      status: 'connected',
      bumpers: { [BUMPER_ID]: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true } },
    }))
    rerender(<VPlayerPage />)
    expect(mock.sendMessage).toHaveBeenCalledTimes(1)

    // Further re-renders (e.g. an unrelated game-state update) must not
    // resend the already-flushed buzz. Another fresh `bumpers` reference,
    // simulating a subsequent real UPDATE broadcast.
    useGame.mockReturnValue(makeGameMock({
      ...mock,
      status: 'connected',
      bumpers: { [BUMPER_ID]: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, CONNECTED: true } },
      gameState: { phase: 'STARTED', question: { ID: 'q1', TYPE: 'NORMAL' }, enrollmentActive: false },
    }))
    rerender(<VPlayerPage />)
    expect(mock.sendMessage).toHaveBeenCalledTimes(1)
  })
})
