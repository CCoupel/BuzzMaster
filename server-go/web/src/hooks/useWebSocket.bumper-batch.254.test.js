import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #254 — action BUMPER_POINTS_BATCH (entrées {id, points} → ENTRIES[{ID, POINTS}]), BUMPER_POINTS_BATCH_RESULT ciblé,
// AWARDED_TEAMS.PLAYERS exposé (grisage des lignes de joueurs). Contrat : websocket-actions.md §BUMPER_POINTS_BATCH.
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

describe('useWebSocket — attribution groupée au niveau joueur (#254)', () => {
  beforeEach(() => { wsInstances = []; vi.useFakeTimers(); vi.stubGlobal('WebSocket', MockWebSocket) })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })
  function open(endpoint = '/ws/admin') {
    const hook = renderHook(() => useWebSocket(endpoint))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() })
    return { hook, ws }
  }
  const update = (qid) => ({ ACTION: 'UPDATE', MSG: { GAME: { PHASE: 'REVEALED', QUESTION: { ID: qid, TYPE: 'QCM' } }, teams: {}, bumpers: {} } })

  it('Critical — bumperPointsBatch envoie BUMPER_POINTS_BATCH {QUESTION_ID, ENTRIES:[{ID, POINTS}]} (montant propre à chaque joueur)', () => {
    const { hook, ws } = open()
    let ok
    act(() => { ok = hook.result.current.bumperPointsBatch('9', [{ id: 'AA:01', points: 3 }, { id: 'vj2', points: -2 }]) })
    expect(ok).toBe(true)
    expect(sentOf(ws, 'BUMPER_POINTS_BATCH')).toEqual([{ QUESTION_ID: '9', ENTRIES: [{ ID: 'AA:01', POINTS: 3 }, { ID: 'vj2', POINTS: -2 }] }])
  })

  it('retourne false quand la WebSocket est fermée', () => {
    const { hook, ws } = open()
    act(() => { ws.close() })
    let ok
    act(() => { ok = hook.result.current.bumperPointsBatch('9', [{ id: 'a', points: 1 }]) })
    expect(ok).toBe(false)
  })

  it('BUMPER_POINTS_BATCH_RESULT expose statut, motif, joueurs crédités / ignorés / inconnus et se consomme', () => {
    const { hook, ws } = open()
    act(() => {
      ws.simulateMessage({ ACTION: 'BUMPER_POINTS_BATCH_RESULT', MSG: { STATUS: 'APPLIED', QUESTION_ID: '9', APPLIED: [{ ID: 'a', POINTS: 3 }], SKIPPED: [{ ID: 'b', REASON: 'ALREADY_AWARDED' }], PLAYERS: [] } })
    })
    expect(hook.result.current.bumperPointsBatchResult).toMatchObject({ status: 'APPLIED', questionId: '9', applied: [{ id: 'a', points: 3 }], skipped: [{ id: 'b', reason: 'ALREADY_AWARDED' }] })
    act(() => { ws.simulateMessage({ ACTION: 'BUMPER_POINTS_BATCH_RESULT', MSG: { STATUS: 'REFUSED', REASON: 'UNKNOWN_PLAYER', QUESTION_ID: '9', APPLIED: [], SKIPPED: [], PLAYERS: ['ZZ'] } }) })
    expect(hook.result.current.bumperPointsBatchResult).toMatchObject({ status: 'REFUSED', reason: 'UNKNOWN_PLAYER', players: ['ZZ'] })
    act(() => { hook.result.current.clearBumperPointsBatchResult() })
    expect(hook.result.current.bumperPointsBatchResult).toBeNull()
  })

  it('AWARDED_TEAMS.PLAYERS est exposé en map joueur → {POINTS, TIMESTAMP, TEAM} pour la question courante ; [] la vide', () => {
    const { hook, ws } = open()
    act(() => { ws.simulateMessage(update('9')) })
    act(() => { ws.simulateMessage({ ACTION: 'AWARDED_TEAMS', MSG: { QUESTION_ID: '9', TEAMS: [], PLAYERS: [{ ID: 'vj1', TEAM: 'Les Rouges', POINTS: 2, TIMESTAMP: 7 }] } }) })
    expect(hook.result.current.awardedPlayers).toEqual({ vj1: { POINTS: 2, TIMESTAMP: 7, TEAM: 'Les Rouges' } })
    act(() => { ws.simulateMessage({ ACTION: 'AWARDED_TEAMS', MSG: { QUESTION_ID: '9', TEAMS: [], PLAYERS: [] } }) })
    expect(hook.result.current.awardedPlayers).toEqual({})
  })

  it('un AWARDED_TEAMS d’une autre question est ignoré (garde d’obsolescence), joueurs compris', () => {
    const { hook, ws } = open()
    act(() => { ws.simulateMessage(update('9')) })
    act(() => { ws.simulateMessage({ ACTION: 'AWARDED_TEAMS', MSG: { QUESTION_ID: '8', TEAMS: [], PLAYERS: [{ ID: 'vj1', TEAM: 'A', POINTS: 2, TIMESTAMP: 7 }] } }) })
    expect(hook.result.current.awardedPlayers).toEqual({})
  })

  it('AWARDED_TEAMS.TEAMS[].TEAM_LEVEL est conservé par équipe (true crédit d\'équipe, false crédits joueurs seulement)', () => {
    const { hook, ws } = open()
    act(() => { ws.simulateMessage(update('9')) })
    act(() => { ws.simulateMessage({ ACTION: 'AWARDED_TEAMS', MSG: { QUESTION_ID: '9', TEAMS: [{ TEAM: 'A', POINTS: 2, TIMESTAMP: 1, TEAM_LEVEL: true }, { TEAM: 'B', POINTS: 1, TIMESTAMP: 2, TEAM_LEVEL: false }], PLAYERS: [] } }) })
    expect(hook.result.current.awardedTeams.A.TEAM_LEVEL).toBe(true)
    expect(hook.result.current.awardedTeams.B.TEAM_LEVEL).toBe(false)
  })
})

