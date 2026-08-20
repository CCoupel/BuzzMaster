import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// ---------------------------------------------------------------------------
// Mock WebSocket global — capture l'instance pour simuler les messages et
// espionner les envois sortants (setEntracte)
// ---------------------------------------------------------------------------

let wsInstance = null

class MockWebSocket {
  constructor(url) {
    wsInstance = this
    this.url = url
    this.readyState = MockWebSocket.CONNECTING
    this.send = vi.fn()
    // onopen / onclose / onerror / onmessage sont assignés par le hook après construction
  }
  close() {
    this.readyState = MockWebSocket.CLOSED
    this.onclose && this.onclose()
  }
}
MockWebSocket.CONNECTING = 0
MockWebSocket.OPEN      = 1
MockWebSocket.CLOSING   = 2
MockWebSocket.CLOSED    = 3

const DEFAULT_ENTRACTE_CONFIG = {
  TITLE: 'ENTRACTE',
  SUBTITLE: 'Retour dans 20mn',
  IMAGE_IS_CUSTOM: false,
  PANEL_SIZE: 65,
  ANIM_PERIOD: 10,
  ANIM_INTENSITY: 20,
}

// ---------------------------------------------------------------------------
// Tests : useWebSocket — mode ENTRACTE (#119, T1 du plan v6.5.2)
// ---------------------------------------------------------------------------

describe('useWebSocket — mode ENTRACTE (#119)', () => {
  beforeEach(() => {
    wsInstance = null
    vi.stubGlobal('WebSocket', MockWebSocket)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.clearAllMocks()
  })

  it('initialise entracte à false et entracteConfig aux défauts contractuels dans le state initial', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    expect(result.current.gameState.entracte).toBe(false)
    expect(result.current.gameState.entracteConfig).toEqual(DEFAULT_ENTRACTE_CONFIG)
  })

  it('met à jour entracte et entracteConfig quand un message UPDATE les contient', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    expect(wsInstance).not.toBeNull()

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'UPDATE',
          MSG: {
            GAME: {
              PHASE: 'STOPPED',
              ENTRACTE: true,
              ENTRACTE_CONFIG: {
                TITLE: 'Pause déjeuner',
                SUBTITLE: 'Retour à 13h30',
                IMAGE_IS_CUSTOM: true,
                PANEL_SIZE: 80,
                ANIM_PERIOD: 6,
                ANIM_INTENSITY: 40,
              },
            },
          },
        }),
      })
    })

    expect(result.current.gameState.entracte).toBe(true)
    expect(result.current.gameState.entracteConfig).toEqual({
      TITLE: 'Pause déjeuner',
      SUBTITLE: 'Retour à 13h30',
      IMAGE_IS_CUSTOM: true,
      PANEL_SIZE: 80,
      ANIM_PERIOD: 6,
      ANIM_INTENSITY: 40,
    })
  })

  it('reflète ENTRACTE: false (sortie d\'entracte) — jamais absorbé par omitempty côté client', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    // 1er UPDATE : entre en entracte
    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'UPDATE',
          MSG: { GAME: { PHASE: 'STOPPED', ENTRACTE: true } },
        }),
      })
    })
    expect(result.current.gameState.entracte).toBe(true)

    // 2e UPDATE : ENTRACTE explicitement false (le serveur ne l'omet jamais, contrat game-state.md)
    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'UPDATE',
          MSG: { GAME: { PHASE: 'STOPPED', ENTRACTE: false } },
        }),
      })
    })
    expect(result.current.gameState.entracte).toBe(false)
  })

  it('conserve entracte/entracteConfig lors d\'un UPDATE partiel qui ne les contient pas (repli client-side)', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'UPDATE',
          MSG: {
            GAME: {
              PHASE: 'STOPPED',
              ENTRACTE: true,
              ENTRACTE_CONFIG: { TITLE: 'X', SUBTITLE: 'Y', IMAGE_IS_CUSTOM: false, PANEL_SIZE: 50, ANIM_PERIOD: 5, ANIM_INTENSITY: 0 },
            },
          },
        }),
      })
    })

    // UPDATE ultérieur (ex. venant d'un client plus ancien ou d'un message partiel) sans ENTRACTE*
    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'UPDATE',
          MSG: { GAME: { PHASE: 'READY' } },
        }),
      })
    })

    expect(result.current.gameState.entracte).toBe(true)
    expect(result.current.gameState.entracteConfig).toEqual({
      TITLE: 'X', SUBTITLE: 'Y', IMAGE_IS_CUSTOM: false, PANEL_SIZE: 50, ANIM_PERIOD: 5, ANIM_INTENSITY: 0,
    })
  })

  it('ne plante pas quand ENTRACTE / ENTRACTE_CONFIG sont totalement absents (client ancien / non-régression)', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.onmessage({
        data: JSON.stringify({
          ACTION: 'UPDATE',
          MSG: { GAME: { PHASE: 'STARTED', TIMER: 10 } },
        }),
      })
    })

    // Aucune exception levée, valeurs par défaut toujours cohérentes
    expect(result.current.gameState.entracte).toBe(false)
    expect(result.current.gameState.entracteConfig).toEqual(DEFAULT_ENTRACTE_CONFIG)
    expect(result.current.gameState.PHASE).toBe('STARTED')
  })

  it('setEntracte(true) envoie ENTRACTE_SET avec ACTIVE: true', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.readyState = MockWebSocket.OPEN
      wsInstance.onopen && wsInstance.onopen()
    })

    act(() => {
      result.current.setEntracte(true)
    })

    expect(wsInstance.send).toHaveBeenCalledTimes(1)
    const sent = JSON.parse(wsInstance.send.mock.calls[0][0])
    expect(sent).toEqual({ ACTION: 'ENTRACTE_SET', MSG: { ACTIVE: true } })
  })

  it('setEntracte(false) envoie ENTRACTE_SET avec ACTIVE: false (commande explicite, pas un toggle — D3)', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))

    await act(async () => {
      wsInstance.readyState = MockWebSocket.OPEN
      wsInstance.onopen && wsInstance.onopen()
    })

    act(() => {
      result.current.setEntracte(false)
    })

    const sent = JSON.parse(wsInstance.send.mock.calls[0][0])
    expect(sent).toEqual({ ACTION: 'ENTRACTE_SET', MSG: { ACTIVE: false } })
  })
})
