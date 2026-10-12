import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #244/#251 — action TEAM_POINTS_BATCH (entrées {team, points} → ENTRIES[{TEAM, POINTS}]), TEAM_POINTS_BATCH_RESULT ciblé,
// AWARDED_TEAMS reçu aussi par l'admin. Contrat : websocket-actions.md §TEAM_POINTS_BATCH / _RESULT / §AWARDED_TEAMS.
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

describe('useWebSocket — attribution groupée (#244/#251)', () => {
  beforeEach(() => { wsInstances = []; vi.useFakeTimers(); vi.stubGlobal('WebSocket', MockWebSocket) })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })
  function open(endpoint = '/ws/admin') {
    const hook = renderHook(() => useWebSocket(endpoint))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() })
    return { hook, ws }
  }
  const update = (qid) => ({ ACTION: 'UPDATE', MSG: { GAME: { PHASE: 'REVEALED', QUESTION: { ID: qid, TYPE: 'QCM' } }, teams: {}, bumpers: {} } })

  it('Critical — teamPointsBatch envoie TEAM_POINTS_BATCH {QUESTION_ID, ENTRIES:[{TEAM, POINTS}]} (une entrée par équipe, montant propre à chacune)', () => {
    const { hook, ws } = open()
    let ok
    act(() => { ok = hook.result.current.teamPointsBatch('9', [{ team: 'Les Bleus', points: 3 }, { team: 'Les Rouges', points: 2 }]) })
    expect(ok).toBe(true)
    expect(sentOf(ws, 'TEAM_POINTS_BATCH')).toEqual([{ QUESTION_ID: '9', ENTRIES: [{ TEAM: 'Les Bleus', POINTS: 3 }, { TEAM: 'Les Rouges', POINTS: 2 }] }])
  })

  it('retourne false quand la WebSocket est fermée (rien n’est parti)', () => {
    const { hook, ws } = open()
    act(() => { ws.close() })
    let ok
    act(() => { ok = hook.result.current.teamPointsBatch('9', [{ team: 'A', points: 1 }]) })
    expect(ok).toBe(false)
  })

  it('TEAM_POINTS_BATCH_RESULT est exposé (statut, motif, équipes créditées/ignorées/inconnues) et peut être consommé', () => {
    const { hook, ws } = open()
    act(() => {
      ws.simulateMessage({ ACTION: 'TEAM_POINTS_BATCH_RESULT', MSG: { STATUS: 'APPLIED', QUESTION_ID: '9', APPLIED: [{ TEAM: 'A', POINTS: 3 }], SKIPPED: [{ TEAM: 'B', REASON: 'ALREADY_AWARDED' }], TEAMS: [] } })
    })
    const r = hook.result.current.teamPointsBatchResult
    expect(r).toMatchObject({ status: 'APPLIED', questionId: '9', applied: [{ team: 'A', points: 3 }], skipped: [{ team: 'B', reason: 'ALREADY_AWARDED' }] })
    act(() => { hook.result.current.clearTeamPointsBatchResult() })
    expect(hook.result.current.teamPointsBatchResult).toBeNull()
  })

  it('un refus expose le motif et les équipes inconnues', () => {
    const { hook, ws } = open()
    act(() => { ws.simulateMessage({ ACTION: 'TEAM_POINTS_BATCH_RESULT', MSG: { STATUS: 'REFUSED', REASON: 'UNKNOWN_TEAM', QUESTION_ID: '9', APPLIED: [], SKIPPED: [], TEAMS: ['Les Bleus'] } }) })
    expect(hook.result.current.teamPointsBatchResult).toMatchObject({ status: 'REFUSED', reason: 'UNKNOWN_TEAM', teams: ['Les Bleus'] })
  })

  it('l’admin reçoit AWARDED_TEAMS comme l’anim : map équipe → {POINTS, TIMESTAMP} pour la question courante', () => {
    const { hook, ws } = open('/ws/admin')
    act(() => { ws.simulateMessage(update('9')) })
    act(() => { ws.simulateMessage({ ACTION: 'AWARDED_TEAMS', MSG: { QUESTION_ID: '9', TEAMS: [{ TEAM: 'Les Bleus', POINTS: 3, TIMESTAMP: 111 }] } }) })
    expect(hook.result.current.awardedTeams).toEqual({ 'Les Bleus': { POINTS: 3, TIMESTAMP: 111 } })
  })

  it('un AWARDED_TEAMS d’une autre question est ignoré (garde d’obsolescence)', () => {
    const { hook, ws } = open('/ws/admin')
    act(() => { ws.simulateMessage(update('9')) })
    act(() => { ws.simulateMessage({ ACTION: 'AWARDED_TEAMS', MSG: { QUESTION_ID: '8', TEAMS: [{ TEAM: 'Les Bleus', POINTS: 3, TIMESTAMP: 1 }] } }) })
    expect(hook.result.current.awardedTeams).toEqual({})
  })
})
