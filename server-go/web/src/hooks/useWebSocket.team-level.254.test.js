import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #254 (revue) — AWARDED_TEAMS.TEAMS[].TEAM_LEVEL conservé dans awardedTeams (grisage au niveau joueur).
let wsInstances = []
class MockWebSocket {
  constructor(url) { this.url = url; this.readyState = 0; this.sentMessages = []; wsInstances.push(this) }
  send(d) { this.sentMessages.push(d) }
  close() { this.readyState = 3; this.onclose && this.onclose() }
  simulateOpen() { this.readyState = 1; this.onopen && this.onopen() }
  simulateMessage(d) { this.onmessage && this.onmessage({ data: JSON.stringify(d) }) }
}
MockWebSocket.CONNECTING = 0; MockWebSocket.OPEN = 1; MockWebSocket.CLOSING = 2; MockWebSocket.CLOSED = 3

describe('useWebSocket — AWARDED_TEAMS.TEAM_LEVEL (#254)', () => {
  beforeEach(() => { wsInstances = []; vi.useFakeTimers(); vi.stubGlobal('WebSocket', MockWebSocket) })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

  it('TEAM_LEVEL est exposé par équipe ; absent → non défini (traité comme faux)', () => {
    const hook = renderHook(() => useWebSocket('/ws/admin'))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() })
    act(() => { ws.simulateMessage({ ACTION: 'UPDATE', MSG: { GAME: { PHASE: 'REVEALED', QUESTION: { ID: '9', TYPE: 'QCM' } }, teams: {}, bumpers: {} } }) })
    act(() => { ws.simulateMessage({ ACTION: 'AWARDED_TEAMS', MSG: { QUESTION_ID: '9', TEAMS: [
      { TEAM: 'A', POINTS: 3, TIMESTAMP: 1, TEAM_LEVEL: true }, { TEAM: 'B', POINTS: 2, TIMESTAMP: 2, TEAM_LEVEL: false }, { TEAM: 'C', POINTS: 1, TIMESTAMP: 3 },
    ], PLAYERS: [] } }) })
    const t = hook.result.current.awardedTeams
    expect([t.A.TEAM_LEVEL, t.B.TEAM_LEVEL, !!t.C.TEAM_LEVEL]).toEqual([true, false, false])
  })
})
