import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #282 (v11.4.0, LOT-2) — contracts/rafale.md §15.7 : défauts ET fusion explicite de
// RAFALE_CURRENT_LETTER / RAFALE_LETTERS (le hook fusionne champ par champ : un champ non listé n'arrive jamais).
let wsInstance = null
class MockWebSocket {
  constructor(url) { wsInstance = this; this.url = url; this.readyState = 0 }
  send() {}
  close() { this.readyState = 3; this.onclose && this.onclose() }
}
MockWebSocket.CONNECTING = 0; MockWebSocket.OPEN = 1; MockWebSocket.CLOSING = 2; MockWebSocket.CLOSED = 3

const push = async (game) => {
  await act(async () => {
    wsInstance.onmessage({ data: JSON.stringify({ ACTION: 'UPDATE', MSG: { GAME: { PHASE: 'STARTED', ...game } } }) })
  })
}

describe('useWebSocket — RAFALE_CURRENT_LETTER / RAFALE_LETTERS (#282)', () => {
  beforeEach(() => { wsInstance = null; vi.stubGlobal('WebSocket', MockWebSocket) })
  afterEach(() => { vi.unstubAllGlobals(); vi.clearAllMocks() })

  it('défauts : RAFALE_CURRENT_LETTER = \'\' et RAFALE_LETTERS = []', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    expect(result.current.gameState.RAFALE_CURRENT_LETTER).toBe('')
    expect(result.current.gameState.RAFALE_LETTERS).toEqual([])
  })

  it('fusionne les deux champs reçus dans un UPDATE', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    await push({ RAFALE_CURRENT_LETTER: 'C', RAFALE_LETTERS: ['D', 'M', 'S'] })
    expect(result.current.gameState.RAFALE_CURRENT_LETTER).toBe('C')
    expect(result.current.gameState.RAFALE_LETTERS).toEqual(['D', 'M', 'S'])
  })

  it('un UPDATE sans ces champs conserve les valeurs précédentes (fusion champ par champ)', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    await push({ RAFALE_CURRENT_LETTER: 'C', RAFALE_LETTERS: ['D'] })
    await push({ TIME: 12 })
    expect(result.current.gameState.RAFALE_CURRENT_LETTER).toBe('C')
    expect(result.current.gameState.RAFALE_LETTERS).toEqual(['D'])
  })

  it('un UPDATE avec lettre vide et liste vide remet à zéro (fin de manche)', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    await push({ RAFALE_CURRENT_LETTER: 'C', RAFALE_LETTERS: ['D'] })
    await push({ RAFALE_CURRENT_LETTER: '', RAFALE_LETTERS: [] })
    expect(result.current.gameState.RAFALE_CURRENT_LETTER).toBe('')
    expect(result.current.gameState.RAFALE_LETTERS).toEqual([])
  })
})
