import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// ---------------------------------------------------------------------------
// Mock WebSocket global — capture l'instance pour simuler les messages
// (même pattern que useWebSocket.ardoise.test.js)
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
// Tests : useWebSocket — CREDIT_POINTS (MAJEUR-1, revue de code #155/#156)
// ---------------------------------------------------------------------------

describe('useWebSocket — CREDIT_POINTS mapping (MAJEUR-1)', () => {
  beforeEach(() => {
    wsInstance = null
    vi.stubGlobal('WebSocket', MockWebSocket)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.clearAllMocks()
  })

  it('initialise creditPoints à 0 dans le state initial', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    expect(result.current.creditPoints).toBe(0)
  })

  it('met à jour creditPoints quand un message CREDIT_POINTS arrive', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    expect(wsInstance).not.toBeNull()

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'CREDIT_POINTS', MSG: { POINTS: 20 } }) })
    })

    expect(result.current.creditPoints).toBe(20)
  })

  it('un CREDIT_POINTS ultérieur écrase la valeur précédente (pas de fusion)', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'CREDIT_POINTS', MSG: { POINTS: 10 } }) })
    })
    expect(result.current.creditPoints).toBe(10)

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'CREDIT_POINTS', MSG: { POINTS: 20 } }) })
    })
    expect(result.current.creditPoints).toBe(20)
  })

  it('CREDIT_POINTS { POINTS: 0 } (NEW_GAME, aucune question courante) remet à zéro', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'CREDIT_POINTS', MSG: { POINTS: 15 } }) })
    })
    expect(result.current.creditPoints).toBe(15)

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'CREDIT_POINTS', MSG: { POINTS: 0 } }) })
    })
    expect(result.current.creditPoints).toBe(0)
  })

  it('MSG absent/POINTS absent replie sur 0', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'CREDIT_POINTS', MSG: {} }) })
    })
    expect(result.current.creditPoints).toBe(0)
  })
})
