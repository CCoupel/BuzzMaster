import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// ---------------------------------------------------------------------------
// Mock WebSocket global — même pattern que useWebSocket.creditPoints.test.js
// ---------------------------------------------------------------------------

let wsInstance = null

class MockWebSocket {
  constructor(url) {
    wsInstance = this
    this.url = url
    this.readyState = MockWebSocket.CONNECTING
  }
  send() {}
  close() {
    this.readyState = MockWebSocket.CLOSED
    this.onclose && this.onclose()
  }
}
MockWebSocket.CONNECTING = 0
MockWebSocket.OPEN      = 1
MockWebSocket.CLOSING   = 2
MockWebSocket.CLOSED    = 3

// ---------------------------------------------------------------------------
// Tests : useWebSocket — questionPosition (#166/F1, T4)
//
// contracts/CHANGELOG.md [20260815-1] : NEXT_QUESTION.CURRENT_POSITION /
// TOTAL_QUESTIONS décrivent la question COURANTE et le quiz, pas la
// suivante — volontairement INDÉPENDANTS de `nextQuestion` : le payload
// n'est plus "tout ou rien" depuis #166 (contracts/CHANGELOG.md
// [20260815-2]) : sur la dernière question du quiz, `nextQuestion` devient
// `null` (pas d'ID) alors que `questionPosition` continue de refléter
// "12/12". C'est le point central testé ici.
// ---------------------------------------------------------------------------

describe('useWebSocket — questionPosition (#166/F1)', () => {
  beforeEach(() => {
    wsInstance = null
    vi.stubGlobal('WebSocket', MockWebSocket)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.clearAllMocks()
  })

  it('initialise questionPosition à { position: 0, total: 0 }', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    expect(result.current.questionPosition).toEqual({ position: 0, total: 0 })
  })

  it('met à jour questionPosition ET nextQuestion quand NEXT_QUESTION arrive avec une question suivante', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'NEXT_QUESTION',
          MSG: { ID: '2', QUESTION: 'Q2', CURRENT_POSITION: 1, TOTAL_QUESTIONS: 12 },
        }),
      })
    })

    expect(result.current.questionPosition).toEqual({ position: 1, total: 12 })
    expect(result.current.nextQuestion).toMatchObject({ ID: '2' })
  })

  // Cas central (#166/GATE 2 D2, contracts/CHANGELOG.md [20260815-2]) : plus
  // aucune question suivante (dernière question du quiz) — MSG n'a pas d'ID,
  // nextQuestion redevient null, MAIS CURRENT_POSITION/TOTAL_QUESTIONS
  // restent renseignés : "12/12" doit continuer de s'afficher.
  it("sur la dernière question (MSG sans ID) : nextQuestion devient null MAIS questionPosition reste renseignée", async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'NEXT_QUESTION',
          MSG: { CURRENT_POSITION: 12, TOTAL_QUESTIONS: 12 },
        }),
      })
    })

    expect(result.current.nextQuestion).toBeNull()
    expect(result.current.questionPosition).toEqual({ position: 12, total: 12 })
  })

  it('MSG entièrement absent (payload vide) : nextQuestion null, questionPosition replie sur { 0, 0 }', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'NEXT_QUESTION', MSG: {} }) })
    })

    expect(result.current.nextQuestion).toBeNull()
    expect(result.current.questionPosition).toEqual({ position: 0, total: 0 })
  })

  it('un NEXT_QUESTION ultérieur écrase la position précédente (pas de fusion)', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({ ACTION: 'NEXT_QUESTION', MSG: { ID: '2', CURRENT_POSITION: 1, TOTAL_QUESTIONS: 12 } }),
      })
    })
    expect(result.current.questionPosition).toEqual({ position: 1, total: 12 })

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({ ACTION: 'NEXT_QUESTION', MSG: { ID: '3', CURRENT_POSITION: 2, TOTAL_QUESTIONS: 12 } }),
      })
    })
    expect(result.current.questionPosition).toEqual({ position: 2, total: 12 })
  })

  it('un CURRENT_POSITION à 0 (aucune question courante trouvée) est distinct de "jamais reçu" côté valeur, mais se lit pareil', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({ ACTION: 'NEXT_QUESTION', MSG: { ID: '1', CURRENT_POSITION: 0, TOTAL_QUESTIONS: 5 } }),
      })
    })

    expect(result.current.questionPosition).toEqual({ position: 0, total: 5 })
    expect(result.current.nextQuestion).toMatchObject({ ID: '1' })
  })
})
