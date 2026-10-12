import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #245 — ROUNDS_UPDATE côté hook : message contractuel (ROUNDS[].ID/NAME/QUESTIONS/ENTRACTES, UNASSIGNED, DELETED_ROUNDS) et
// ROUNDS_UPDATE_RESULT ciblé. GameState.ROUNDS / ROUND_CONTEXT reçus par UPDATE. Contrat websocket-actions.md §ROUNDS_UPDATE.
let wsInstances = []
class MockWebSocket {
  constructor(url) { this.url = url; this.readyState = 0; this.sentMessages = []; wsInstances.push(this) }
  send(d) { this.sentMessages.push(d) }
  close() { if (this.readyState === 3) return; this.readyState = 3; this.onclose && this.onclose() }
  simulateOpen() { this.readyState = 1; this.onopen && this.onopen() }
  simulateMessage(d) { this.onmessage && this.onmessage({ data: JSON.stringify(d) }) }
}
MockWebSocket.CONNECTING = 0; MockWebSocket.OPEN = 1; MockWebSocket.CLOSING = 2; MockWebSocket.CLOSED = 3
const sentOf = (ws, a) => ws.sentMessages.map((m) => JSON.parse(m)).filter((m) => m.ACTION === a).map((m) => m.MSG)

describe('useWebSocket — manches (#245)', () => {
  beforeEach(() => { wsInstances = []; vi.useFakeTimers(); vi.stubGlobal('WebSocket', MockWebSocket) })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })
  function open(endpoint = '/ws/admin') {
    const hook = renderHook(() => useWebSocket(endpoint))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() })
    return { hook, ws }
  }

  it('Critical — roundsUpdate envoie ROUNDS_UPDATE en MAJUSCULES : manche existante (ID), nouvelle (sans ID, ENTRACTES), UNASSIGNED, DELETED_ROUNDS', () => {
    const { hook, ws } = open()
    let ok
    act(() => {
      ok = hook.result.current.roundsUpdate({
        rounds: [{ id: 'r1', name: 'Un', questions: [1, 2] }, { name: 'Neuve', questions: [], entractes: true }, { name: 'Sans', questions: ['9'], entractes: false }],
        unassigned: ['3'],
        deletedRounds: [{ id: 'r2', questions: 'DELETE' }],
      })
    })
    expect(ok).toBe(true)
    expect(sentOf(ws, 'ROUNDS_UPDATE')).toEqual([{
      ROUNDS: [{ ID: 'r1', NAME: 'Un', QUESTIONS: ['1', '2'] }, { NAME: 'Neuve', QUESTIONS: [], ENTRACTES: true }, { NAME: 'Sans', QUESTIONS: ['9'], ENTRACTES: false }],
      UNASSIGNED: ['3'],
      DELETED_ROUNDS: [{ ID: 'r2', QUESTIONS: 'DELETE' }],
    }])
  })

  it('sans manche supprimée, DELETED_ROUNDS est absent ; WebSocket fermée → false', () => {
    const { hook, ws } = open()
    act(() => { hook.result.current.roundsUpdate({ rounds: [], unassigned: ['1'] }) })
    expect(sentOf(ws, 'ROUNDS_UPDATE')[0]).not.toHaveProperty('DELETED_ROUNDS')
    act(() => { ws.close() })
    let ok
    act(() => { ok = hook.result.current.roundsUpdate({ rounds: [], unassigned: [] }) })
    expect(ok).toBe(false)
  })

  it('ROUNDS_UPDATE_RESULT : statut APPLIED avec manches créées / entractes / suppressions, ou REFUSED avec motif ; consommable', () => {
    const { hook, ws } = open()
    act(() => {
      ws.simulateMessage({ ACTION: 'ROUNDS_UPDATE_RESULT', MSG: { STATUS: 'APPLIED', ROUNDS: [{ ID: 'r1', NAME: 'Un' }], CREATED_ENTRACTES: [{ ROUND_ID: 'r1', START_ID: '8', END_ID: '9' }], DELETED_QUESTIONS: ['3'] } })
    })
    expect(hook.result.current.roundsUpdateResult).toMatchObject({ status: 'APPLIED', rounds: [{ id: 'r1', name: 'Un' }] })
    act(() => { hook.result.current.clearRoundsUpdateResult() })
    expect(hook.result.current.roundsUpdateResult).toBeNull()
    act(() => { ws.simulateMessage({ ACTION: 'ROUNDS_UPDATE_RESULT', MSG: { STATUS: 'REFUSED', REASON: 'INCOMPLETE', QUESTIONS: ['4'] } }) })
    expect(hook.result.current.roundsUpdateResult).toMatchObject({ status: 'REFUSED', reason: 'INCOMPLETE' })
  })

  it('GAME.ROUNDS et GAME.ROUND_CONTEXT reçus par UPDATE sont exposés dans gameState (jamais absents)', () => {
    const { hook, ws } = open('/ws/tv')
    act(() => {
      ws.simulateMessage({ ACTION: 'UPDATE', MSG: { GAME: { PHASE: 'STOPPED', ROUNDS: [{ ID: 'r1', NAME: 'Un' }], ROUND_CONTEXT: { ROUND_ID: 'r1', ROUND_INDEX: 1, ROUND_COUNT: 2, POSITION: 1, TOTAL: 3, FIRST_OF_ROUND: true } }, teams: {}, bumpers: {} } })
    })
    expect(hook.result.current.gameState.ROUNDS).toEqual([{ ID: 'r1', NAME: 'Un' }])
    expect(hook.result.current.gameState.ROUND_CONTEXT).toMatchObject({ ROUND_INDEX: 1, FIRST_OF_ROUND: true })
  })
})
