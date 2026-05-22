import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// ---------------------------------------------------------------------------
// Mock WebSocket global — capture l'instance pour simuler les messages
// ---------------------------------------------------------------------------

let wsInstance = null

class MockWebSocket {
  constructor(url) {
    wsInstance = this
    this.url = url
    this.readyState = MockWebSocket.CONNECTING
    // onopen / onclose / onerror / onmessage sont assignés par le hook après construction
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
// Tests : useWebSocket — mapping ARDOISE_ANSWERS (#91)
// ---------------------------------------------------------------------------

describe('useWebSocket — ARDOISE_ANSWERS mapping (#91)', () => {
  beforeEach(() => {
    wsInstance = null
    vi.stubGlobal('WebSocket', MockWebSocket)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.clearAllMocks()
  })

  it('initialise ARDOISE_ANSWERS à {} dans le state initial', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    // ARDOISE_ANSWERS doit exister dans le state initial (pas undefined)
    expect(result.current.gameState.ARDOISE_ANSWERS).toBeDefined()
    expect(result.current.gameState.ARDOISE_ANSWERS).toEqual({})
  })

  it('met à jour ARDOISE_ANSWERS quand un message UPDATE contient ARDOISE_ANSWERS', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    expect(wsInstance).not.toBeNull()

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'UPDATE',
          MSG: {
            GAME: {
              PHASE: 'STARTED',
              ARDOISE_ANSWERS: {
                'Équipe A': { TEXT: 'Paris',  SUBMITTED_AT: 1000 },
                'Équipe B': { TEXT: 'Berlin', SUBMITTED_AT: 1200 },
              },
            },
          },
        }),
      })
    })

    expect(result.current.gameState.ARDOISE_ANSWERS).toEqual({
      'Équipe A': { TEXT: 'Paris',  SUBMITTED_AT: 1000 },
      'Équipe B': { TEXT: 'Berlin', SUBMITTED_AT: 1200 },
    })
  })

  it('conserve ARDOISE_ANSWERS lors d\'un UPDATE ultérieur qui ne contient pas ARDOISE_ANSWERS', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    // 1er UPDATE : fixe ARDOISE_ANSWERS
    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'UPDATE',
          MSG: {
            GAME: {
              PHASE: 'STARTED',
              ARDOISE_ANSWERS: {
                'Équipe A': { TEXT: 'Paris', SUBMITTED_AT: 1000 },
              },
            },
          },
        }),
      })
    })

    // 2e UPDATE : pas d'ARDOISE_ANSWERS dans MSG.GAME
    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'UPDATE',
          MSG: {
            GAME: {
              PHASE: 'STOPPED',
              // Pas d'ARDOISE_ANSWERS ici
            },
          },
        }),
      })
    })

    // ARDOISE_ANSWERS doit être conservé depuis le 1er UPDATE
    expect(result.current.gameState.ARDOISE_ANSWERS).toEqual({
      'Équipe A': { TEXT: 'Paris', SUBMITTED_AT: 1000 },
    })
  })
})
