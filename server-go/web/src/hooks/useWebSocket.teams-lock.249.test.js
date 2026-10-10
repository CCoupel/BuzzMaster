import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #249 (v11.2) — TEAMS_LOCKED dans l'état reçu, TEAMS_UPDATE_REFUSED ciblé.
// Contrats : game-state.md §TEAMS_LOCKED, websocket-actions.md §TEAMS_UPDATE_REFUSED.
let wsInstances = []
class MockWebSocket {
  constructor(url) { this.url = url; this.readyState = 0; this.sentMessages = []; wsInstances.push(this) }
  send(d) { this.sentMessages.push(d) }
  close() { if (this.readyState === 3) return; this.readyState = 3; this.onclose && this.onclose() }
  simulateOpen() { this.readyState = 1; this.onopen && this.onopen() }
  simulateMessage(d) { this.onmessage && this.onmessage({ data: JSON.stringify(d) }) }
}
MockWebSocket.CONNECTING = 0; MockWebSocket.OPEN = 1; MockWebSocket.CLOSING = 2; MockWebSocket.CLOSED = 3

const update = (locked) => ({ ACTION: 'UPDATE', MSG: { GAME: { PHASE: 'STARTED', QUESTION: { ID: 'q1', TYPE: 'ARDOISE' }, TEAMS_LOCKED: locked }, teams: {}, bumpers: {} } })

describe('useWebSocket — TEAMS_LOCKED / TEAMS_UPDATE_REFUSED (#249)', () => {
  beforeEach(() => { wsInstances = []; vi.useFakeTimers(); vi.stubGlobal('WebSocket', MockWebSocket) })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

  function open() {
    const hook = renderHook(() => useWebSocket('/ws/admin'))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() })
    return { hook, ws }
  }

  it('gameState.TEAMS_LOCKED vaut false au départ puis suit chaque UPDATE (true puis false)', () => {
    const { hook, ws } = open()
    expect(hook.result.current.gameState.TEAMS_LOCKED).toBe(false)
    act(() => { ws.simulateMessage(update(true)) })
    expect(hook.result.current.gameState.TEAMS_LOCKED).toBe(true)
    act(() => { ws.simulateMessage(update(false)) })
    expect(hook.result.current.gameState.TEAMS_LOCKED).toBe(false)
  })

  it('TEAMS_UPDATE_REFUSED expose le motif et les équipes, et peut être consommé', () => {
    const { hook, ws } = open()
    act(() => { ws.simulateMessage({ ACTION: 'TEAMS_UPDATE_REFUSED', MSG: { REASON: 'TEAMS_LOCKED_DURING_QUESTION', TEAMS: ['Les Bleus'] } }) })
    expect(hook.result.current.teamsUpdateRefused).toEqual({ reason: 'TEAMS_LOCKED_DURING_QUESTION', teams: ['Les Bleus'] })
    act(() => { hook.result.current.clearTeamsUpdateRefused() })
    expect(hook.result.current.teamsUpdateRefused).toBeNull()
  })
})
