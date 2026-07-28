import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, act, fireEvent } from '@testing-library/react'
import VPlayerPage from './VPlayerPage'
import { useGame } from '../hooks/GameContext'

// ---------------------------------------------------------------------------
// Tests : VPlayerPage — handleArdoiseChange, premier envoi immédiat (#117)
//
// Cause racine #3 du plan (_work/reports/plan-20260727-093000.md) : l'envoi
// ARDOISE_INPUT était un DEBOUNCE de 200ms — chaque frappe annulait l'envoi
// programmé, donc une équipe écrivant sans pause n'envoyait rien tant
// qu'elle ne s'arrêtait pas. Le correctif fait partir le tout premier
// caractère non vide immédiatement (synchrone) pour chaque question ; les
// frappes suivantes restent régulées à ~200ms.
//
// Point de non-régression le plus important (cause racine #3, cf. handoff
// team-lead) : une équipe qui écrit une longue réponse SANS PAUSE doit être
// datée à son premier caractère, pas à sa première pause — d'où le test
// dédié "rafale sans pause" ci-dessous.
// ---------------------------------------------------------------------------

// vi.hoisted: a plain `useNavigate: () => vi.fn()` recreates a new function on
// every render, which retriggers effects depending on `navigate` and can spin
// the component into a render loop (observed as a vitest worker OOM in QA —
// _work/reports/qa-20260727-193400.md). Stabilized the same way as
// VPlayerPage.test.jsx.
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

// Interactive stub: exposes onChange so tests can simulate keystrokes, and
// mirrors `value`/`disabled` as data-attributes for assertions if needed.
vi.mock('../components/ArdoiseKeyboard', () => ({
  default: ({ value, onChange, disabled }) => (
    <input
      data-testid="ardoise-input"
      value={value}
      disabled={disabled}
      onChange={(e) => onChange(e.target.value)}
      readOnly={false}
    />
  ),
}))

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
  gameState: {
    phase: 'STARTED',
    question: { ID: 'aq1', TYPE: 'ARDOISE', ARDOISE_KEYBOARD_TYPE: 'AZERTY' },
  },
  bumpers: {
    vjoueur_alice: { NAME: 'Alice', IS_VIRTUAL: true, IS_VPLAYER: true, TEAM: 'Équipe A', CONNECTED: true },
  },
  teams: { 'Équipe A': { NAME: 'Équipe A', COLOR: [255, 0, 0] } },
  status: 'connected',
  playerConnectStatus: null,
  clearPlayerConnectStatus: vi.fn(),
  ...overrides,
})

function typeChar(input, text) {
  fireEvent.change(input, { target: { value: text } })
}

describe('VPlayerPage — ARDOISE : premier caractère envoyé immédiatement (#117)', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.stubGlobal('localStorage', createLocalStorageMock())
    localStorage.setItem('vplayer_name', 'Alice')
    localStorage.setItem('vplayer_session', '1234567890')
    useGame.mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('envoie ARDOISE_INPUT de façon synchrone dès le premier caractère non vide, sans attendre le debounce', async () => {
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)

    const { getByTestId } = render(<VPlayerPage />)
    const input = getByTestId('ardoise-input')

    await act(async () => {
      typeChar(input, 'P')
    })

    // Sent right away — no timer advance needed at all.
    expect(mock.sendMessage).toHaveBeenCalledWith('ARDOISE_INPUT', { TEXT: 'P', ID: 'vjoueur_alice' })
    expect(mock.sendMessage).toHaveBeenCalledTimes(1)
  })

  it('régule les frappes suivantes à ~200ms (debounce), sans envoi supplémentaire avant le délai', async () => {
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)

    const { getByTestId } = render(<VPlayerPage />)
    const input = getByTestId('ardoise-input')

    await act(async () => {
      typeChar(input, 'P') // immediate — call #1
    })
    expect(mock.sendMessage).toHaveBeenCalledTimes(1)

    await act(async () => {
      typeChar(input, 'Pa') // debounced
    })
    await act(async () => {
      vi.advanceTimersByTime(199)
    })
    // Still just the one immediate call — debounce hasn't fired yet.
    expect(mock.sendMessage).toHaveBeenCalledTimes(1)

    await act(async () => {
      vi.advanceTimersByTime(1)
    })
    expect(mock.sendMessage).toHaveBeenCalledTimes(2)
    expect(mock.sendMessage).toHaveBeenNthCalledWith(2, 'ARDOISE_INPUT', { TEXT: 'Pa', ID: 'vjoueur_alice' })
  })

  it("régression cause racine #3 : une rafale de frappes SANS PAUSE n'envoie qu'un seul message immédiat sur le premier caractère, pas un par lettre", async () => {
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)

    const { getByTestId } = render(<VPlayerPage />)
    const input = getByTestId('ardoise-input')

    // Simulate a fast, uninterrupted burst of keystrokes (no pause between them —
    // each subsequent keystroke re-arms the 200ms debounce timer before it fires).
    const burst = ['P', 'Pa', 'Par', 'Pari', 'Paris']
    for (const text of burst) {
      await act(async () => {
        typeChar(input, text)
        vi.advanceTimersByTime(50) // well under the 200ms debounce window
      })
    }

    // Only the very first keystroke was sent synchronously; the rest are still
    // pending in the debounce timer (no pause long enough for it to fire).
    expect(mock.sendMessage).toHaveBeenCalledTimes(1)
    expect(mock.sendMessage).toHaveBeenCalledWith('ARDOISE_INPUT', { TEXT: 'P', ID: 'vjoueur_alice' })

    // Once the team finally pauses, the debounce flushes the latest full text.
    await act(async () => {
      vi.advanceTimersByTime(200)
    })
    expect(mock.sendMessage).toHaveBeenCalledTimes(2)
    expect(mock.sendMessage).toHaveBeenNthCalledWith(2, 'ARDOISE_INPUT', { TEXT: 'Paris', ID: 'vjoueur_alice' })
  })

  it('le mécanisme se réarme au changement de question : la question suivante envoie de nouveau son premier caractère immédiatement', async () => {
    const mock = makeGameMock()
    useGame.mockReturnValue(mock)

    const { getByTestId, rerender } = render(<VPlayerPage />)
    let input = getByTestId('ardoise-input')

    await act(async () => {
      typeChar(input, 'P')
    })
    expect(mock.sendMessage).toHaveBeenCalledTimes(1)

    // New question — server clears ARDOISE_ANSWERS, VPlayer resets local ardoise state.
    // Reuse the SAME sendMessage spy (mock.sendMessage) so the call count below reflects
    // both questions — a fresh vi.fn() here would silently start back at 0 and the
    // assertion would pass against the wrong spy regardless of the actual behavior.
    useGame.mockReturnValue(makeGameMock({
      gameState: {
        phase: 'STARTED',
        question: { ID: 'aq2', TYPE: 'ARDOISE', ARDOISE_KEYBOARD_TYPE: 'AZERTY' },
      },
      sendMessage: mock.sendMessage,
    }))
    rerender(<VPlayerPage />)
    input = getByTestId('ardoise-input')

    await act(async () => {
      typeChar(input, 'L')
    })

    // Second call overall, but it's the FIRST (immediate) send for question aq2 —
    // proves ardoiseFirstSentRef was rearmed by the question-ID change.
    expect(mock.sendMessage).toHaveBeenCalledTimes(2)
    expect(mock.sendMessage).toHaveBeenNthCalledWith(2, 'ARDOISE_INPUT', { TEXT: 'L', ID: 'vjoueur_alice' })
  })
})
