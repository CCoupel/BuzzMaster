import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// ---------------------------------------------------------------------------
// useWebSocket — son de la question (v11.1, #219, contrat
// contracts/game-state.md GAME.QUESTION_SOUND_STATE/ANSWER_TIMER_WAITING,
// contracts/websocket-actions.md §QUESTION_SOUND), patron
// useWebSocket.rafale.test.js / useWebSocket.entracte.test.js.
//
// Dispatché en correctif de revue (finding MAJEUR,
// _work/reports/code-review-frontend-20260922-150000.md) — le plan initial
// ne listait aucun test JS pour ce lot.
//
// Périmètre : capture des deux champs GameState dans le UPDATE (jamais
// omitempty côté serveur, CLAUDE.md — donc `?? prev`, pas `|| defaut`), et
// questionSound(command) qui émet {ACTION:'QUESTION_SOUND', MSG:{COMMAND}}.
// ---------------------------------------------------------------------------

let wsInstance = null

class MockWebSocket {
  constructor(url) {
    wsInstance = this
    this.url = url
    this.readyState = MockWebSocket.CONNECTING
    this.send = vi.fn()
  }
  close() {
    this.readyState = MockWebSocket.CLOSED
    this.onclose && this.onclose()
  }
}
MockWebSocket.CONNECTING = 0
MockWebSocket.OPEN = 1
MockWebSocket.CLOSING = 2
MockWebSocket.CLOSED = 3

beforeEach(() => {
  wsInstance = null
  vi.stubGlobal('WebSocket', MockWebSocket)
})

afterEach(() => {
  vi.unstubAllGlobals()
  vi.clearAllMocks()
})

// ---------------------------------------------------------------------------
// État initial — aucun omitempty côté serveur, donc une valeur de repli
// définie et non-nil dès le montage (règle projet, CLAUDE.md).
// ---------------------------------------------------------------------------

describe('useWebSocket — QUESTION_SOUND_STATE/ANSWER_TIMER_WAITING, état initial', () => {
  it("gameState.QUESTION_SOUND_STATE démarre à 'IDLE'", () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    expect(result.current.gameState.QUESTION_SOUND_STATE).toBe('IDLE')
  })

  it('gameState.ANSWER_TIMER_WAITING démarre à false', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    expect(result.current.gameState.ANSWER_TIMER_WAITING).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// UPDATE — capture depuis MSG.GAME (contrat game-state.md, tableau
// ws-payload-serialization.md : diffusé même vide/false).
// ---------------------------------------------------------------------------

describe('useWebSocket — capture de QUESTION_SOUND_STATE/ANSWER_TIMER_WAITING depuis UPDATE', () => {
  it('met à jour les deux champs depuis un UPDATE', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'UPDATE',
          MSG: { GAME: { PHASE: 'STARTED', QUESTION_SOUND_STATE: 'PLAYING', ANSWER_TIMER_WAITING: true } },
        }),
      })
    })

    expect(result.current.gameState.QUESTION_SOUND_STATE).toBe('PLAYING')
    expect(result.current.gameState.ANSWER_TIMER_WAITING).toBe(true)
  })

  it('une transition PLAYING → PAUSED → IDLE est reflétée à chaque UPDATE (pas figée à la première valeur)', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    for (const state of ['PLAYING', 'PAUSED', 'IDLE']) {
      await act(async () => {
        wsInstance.onmessage({
          data: JSON.stringify({ ACTION: 'UPDATE', MSG: { GAME: { QUESTION_SOUND_STATE: state } } }),
        })
      })
      expect(result.current.gameState.QUESTION_SOUND_STATE).toBe(state)
    }
  })

  it("ANSWER_TIMER_WAITING true → false (libération du chronomètre différé, CA15) est bien reflété", async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'UPDATE', MSG: { GAME: { ANSWER_TIMER_WAITING: true } } }) })
    })
    expect(result.current.gameState.ANSWER_TIMER_WAITING).toBe(true)

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'UPDATE', MSG: { GAME: { ANSWER_TIMER_WAITING: false } } }) })
    })
    expect(result.current.gameState.ANSWER_TIMER_WAITING).toBe(false)
  })

  it('UPDATE sans ces deux clés : conserve la valeur précédente (repli `?? prev`, jamais réinitialisé à IDLE/false par erreur)', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({ ACTION: 'UPDATE', MSG: { GAME: { QUESTION_SOUND_STATE: 'PLAYING', ANSWER_TIMER_WAITING: true } } }),
      })
    })
    expect(result.current.gameState.QUESTION_SOUND_STATE).toBe('PLAYING')
    expect(result.current.gameState.ANSWER_TIMER_WAITING).toBe(true)

    // UPDATE ultérieur sur un champ sans rapport (PHASE) — les deux champs
    // son doivent survivre intacts, jamais réinitialisés silencieusement.
    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'UPDATE', MSG: { GAME: { PHASE: 'STARTED' } } }) })
    })
    expect(result.current.gameState.QUESTION_SOUND_STATE).toBe('PLAYING')
    expect(result.current.gameState.ANSWER_TIMER_WAITING).toBe(true)
  })

  it("ANSWER_TIMER_WAITING: false explicite est honoré (pas confondu avec une absence — `?? prev`, jamais `|| prev`)", async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'UPDATE', MSG: { GAME: { ANSWER_TIMER_WAITING: true } } }) })
    })
    expect(result.current.gameState.ANSWER_TIMER_WAITING).toBe(true)

    // false est une valeur MEANINGFUL (jamais omitempty côté serveur) — un
    // `||` au lieu de `??` la confondrait avec "absent" et garderait `true`.
    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'UPDATE', MSG: { GAME: { ANSWER_TIMER_WAITING: false } } }) })
    })
    expect(result.current.gameState.ANSWER_TIMER_WAITING).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// questionSound(command) — émission WS (contrat websocket-actions.md
// §QUESTION_SOUND), patron setEntracte (useWebSocket.entracte.test.js).
// ---------------------------------------------------------------------------

describe('useWebSocket — questionSound(command) émet ACTION:QUESTION_SOUND', () => {
  it.each(['PLAY', 'PAUSE', 'RESUME', 'STOP'])("questionSound('%s') envoie {ACTION:'QUESTION_SOUND', MSG:{COMMAND:'%s'}}", async (command) => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.readyState = MockWebSocket.OPEN
      wsInstance.onopen && wsInstance.onopen()
    })

    act(() => {
      result.current.questionSound(command)
    })

    const call = wsInstance.send.mock.calls
      .map(([payload]) => JSON.parse(payload))
      .find((msg) => msg.ACTION === 'QUESTION_SOUND')
    expect(call).toEqual({ ACTION: 'QUESTION_SOUND', MSG: { COMMAND: command } })
  })

  it('questionSound est exposé par le hook (disponible via useGame())', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    expect(typeof result.current.questionSound).toBe('function')
  })
})
