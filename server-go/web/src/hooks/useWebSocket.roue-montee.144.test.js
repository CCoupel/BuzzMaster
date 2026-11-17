import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #144 (v12.0.0) — ROUE_WHEEL.SPIN_UP_MS et FULL_SPEED (contracts/roue.md §4) : valeurs initiales sûres, reprise d'un UPDATE.
let wsInstance = null
class MockWebSocket {
  constructor(url) { wsInstance = this; this.url = url; this.readyState = MockWebSocket.CONNECTING }
  send() {}
  close() { this.readyState = MockWebSocket.CLOSED; this.onclose && this.onclose() }
}
MockWebSocket.CONNECTING = 0; MockWebSocket.OPEN = 1; MockWebSocket.CLOSING = 2; MockWebSocket.CLOSED = 3
const push = async (msg) => { await act(async () => { wsInstance.onmessage({ data: JSON.stringify(msg) }) }) }
const update = (game) => push({ ACTION: 'UPDATE', MSG: { GAME: { PHASE: 'STARTED', ...game } } })
beforeEach(() => { wsInstance = null; vi.stubGlobal('WebSocket', MockWebSocket) })
afterEach(() => { vi.unstubAllGlobals(); vi.clearAllMocks() })

describe('useWebSocket — montée de la roue ROUE', () => {
  it('état initial : FULL_SPEED=false et SPIN_UP_MS=0 (jamais tapable avant le premier UPDATE)', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    expect(result.current.gameState.ROUE_WHEEL.FULL_SPEED).toBe(false)
    expect(result.current.gameState.ROUE_WHEEL.SPIN_UP_MS).toBe(0)
  })
  it('un UPDATE reprend SPIN_UP_MS et FULL_SPEED, y compris le passage true → false', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    const wheel = (full) => ({ LAYOUT: 'strip', SPIN_PERIOD_MS: 2500, SPIN_MAX_MS: 10000, SPIN_UP_MS: 10000, FULL_SPEED: full, SEGMENTS: [], CURRENT: -1, STOP: null })
    await update({ ROUE_SUBPHASE: 'SPINNING', ROUE_WHEEL: wheel(false) })
    expect(result.current.gameState.ROUE_WHEEL.SPIN_UP_MS).toBe(10000)
    expect(result.current.gameState.ROUE_WHEEL.FULL_SPEED).toBe(false)
    await update({ ROUE_SUBPHASE: 'SPINNING', ROUE_WHEEL: wheel(true) })
    expect(result.current.gameState.ROUE_WHEEL.FULL_SPEED).toBe(true)
    await update({ ROUE_SUBPHASE: 'STOPPING', ROUE_WHEEL: { ...wheel(false), STOP: { TO: 1, DURATION_MS: 4000 } } })
    expect(result.current.gameState.ROUE_WHEEL.FULL_SPEED).toBe(false)
  })
})
