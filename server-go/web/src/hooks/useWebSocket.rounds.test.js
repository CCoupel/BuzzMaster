import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #245 — ROUNDS_UPDATE (admin), ROUNDS_UPDATE_RESULT, GameState.ROUNDS / ROUND_CONTEXT.
let wsInstances = []
class MockWebSocket {
  constructor(url) { this.url = url; this.readyState = 0; this.sentMessages = []; wsInstances.push(this) }
  send(d) { this.sentMessages.push(d) }
  close() { this.readyState = 3; this.onclose && this.onclose() }
  simulateOpen() { this.readyState = 1; this.onopen && this.onopen() }
  simulateMessage(d) { this.onmessage && this.onmessage({ data: JSON.stringify(d) }) }
}
MockWebSocket.CONNECTING = 0; MockWebSocket.OPEN = 1; MockWebSocket.CLOSING = 2; MockWebSocket.CLOSED = 3
const sentOf = (ws, a) => ws.sentMessages.map((m) => JSON.parse(m)).filter((m) => m.ACTION === a).map((m) => m.MSG)

describe('useWebSocket — manches (#245)', () => {
  beforeEach(() => { wsInstances = []; vi.useFakeTimers(); vi.stubGlobal('WebSocket', MockWebSocket) })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })
  const open = () => { const hook = renderHook(() => useWebSocket('/ws/admin')); const ws = wsInstances[0]; act(() => { ws.simulateOpen() }); return { hook, ws } }

  it('défauts : ROUNDS [] et ROUND_CONTEXT à zéro', () => {
    const { hook } = open()
    expect(hook.result.current.gameState.ROUNDS).toEqual([])
    expect(hook.result.current.gameState.ROUND_CONTEXT).toMatchObject({ ROUND_INDEX: 0, FIRST_OF_ROUND: false })
  })

  it('UPDATE expose ROUNDS et ROUND_CONTEXT', () => {
    const { hook, ws } = open()
    act(() => { ws.simulateMessage({ ACTION: 'UPDATE', MSG: { GAME: { PHASE: 'STOPPED', ROUNDS: [{ ID: 'r1', NAME: 'A' }], ROUND_CONTEXT: { ROUND_ID: 'r1', ROUND_INDEX: 1, ROUND_COUNT: 1, POSITION: 2, TOTAL: 5, FIRST_OF_ROUND: false } }, teams: {}, bumpers: {} } }) })
    expect(hook.result.current.gameState.ROUNDS).toEqual([{ ID: 'r1', NAME: 'A' }])
    expect(hook.result.current.gameState.ROUND_CONTEXT.POSITION).toBe(2)
  })

  it('roundsUpdate envoie ROUNDS_UPDATE au format du contrat (ENTRACTES seulement si fourni, DELETED_ROUNDS si présent)', () => {
    const { hook, ws } = open()
    let ok
    act(() => { ok = hook.result.current.roundsUpdate({ rounds: [{ id: 'r1', name: 'A', questions: ['1', 2] }, { name: 'B', questions: [], entractes: false }], unassigned: ['3'], deletedRounds: [{ id: 'r2', questions: 'KEEP' }] }) })
    expect(ok).toBe(true)
    expect(sentOf(ws, 'ROUNDS_UPDATE')).toEqual([{
      ROUNDS: [{ ID: 'r1', NAME: 'A', QUESTIONS: ['1', '2'] }, { NAME: 'B', QUESTIONS: [], ENTRACTES: false }],
      UNASSIGNED: ['3'],
      DELETED_ROUNDS: [{ ID: 'r2', QUESTIONS: 'KEEP' }],
    }])
  })

  it('ROUNDS_UPDATE_RESULT est exposé et se consomme', () => {
    const { hook, ws } = open()
    act(() => { ws.simulateMessage({ ACTION: 'ROUNDS_UPDATE_RESULT', MSG: { STATUS: 'APPLIED', ROUNDS: [{ ID: 'r3', NAME: 'C' }], CREATED_ENTRACTES: [{ ROUND_ID: 'r3', START_ID: '86', END_ID: '87' }], DELETED_QUESTIONS: ['9'] } }) })
    expect(hook.result.current.roundsUpdateResult).toMatchObject({ status: 'APPLIED', rounds: [{ id: 'r3', name: 'C' }], createdEntractes: [{ roundId: 'r3', startId: '86', endId: '87' }], deletedQuestions: ['9'] })
    act(() => { hook.result.current.clearRoundsUpdateResult() })
    expect(hook.result.current.roundsUpdateResult).toBeNull()
  })
})
