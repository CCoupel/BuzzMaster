import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #254 — BUMPER_POINTS_BATCH, son résultat et AWARDED_TEAMS.PLAYERS.

let ws
class MockWebSocket {
  constructor() { ws = this; this.readyState = 0; this.sent = [] }
  send(m) { this.sent.push(JSON.parse(m)) }
  close() { this.readyState = 3 }
}
MockWebSocket.OPEN = 1
const msg = (ACTION, MSG) => act(() => { ws.onmessage({ data: JSON.stringify({ ACTION, MSG }) }) })

describe('useWebSocket — attribution groupée joueurs (#254)', () => {
  beforeEach(() => vi.stubGlobal('WebSocket', MockWebSocket))
  afterEach(() => vi.unstubAllGlobals())

  it('bumperPointsBatch envoie QUESTION_ID + ENTRIES {ID, POINTS} et retourne le booléen d\'envoi', () => {
    const { result } = renderHook(() => useWebSocket('/ws/admin'))
    let ok
    act(() => { ok = result.current.bumperPointsBatch('12', [{ id: 'b1', points: 2 }]) })
    expect(ok).toBe(false)
    ws.readyState = 1
    act(() => { ok = result.current.bumperPointsBatch(12, [{ id: 'b1', points: 2 }, { id: 'b2', points: -1 }]) })
    expect(ok).toBe(true)
    expect(ws.sent.at(-1)).toEqual({ ACTION: 'BUMPER_POINTS_BATCH', MSG: { QUESTION_ID: '12', ENTRIES: [{ ID: 'b1', POINTS: 2 }, { ID: 'b2', POINTS: -1 }] } })
  })

  it('BUMPER_POINTS_BATCH_RESULT exposé puis consommé', () => {
    const { result } = renderHook(() => useWebSocket('/ws/admin'))
    msg('BUMPER_POINTS_BATCH_RESULT', { STATUS: 'APPLIED', QUESTION_ID: '12', APPLIED: [{ ID: 'b1', POINTS: 2 }], SKIPPED: [{ ID: 'b2', REASON: 'ALREADY_AWARDED' }], PLAYERS: [] })
    expect(result.current.bumperPointsBatchResult).toEqual({
      status: 'APPLIED', reason: undefined, questionId: '12',
      applied: [{ id: 'b1', points: 2 }], skipped: [{ id: 'b2', reason: 'ALREADY_AWARDED' }], players: [],
    })
    act(() => result.current.clearBumperPointsBatchResult())
    expect(result.current.bumperPointsBatchResult).toBeNull()
  })

  it('AWARDED_TEAMS.PLAYERS → awardedPlayers indexé par ID (un crédit à 0 est un verrou valide)', () => {
    const { result } = renderHook(() => useWebSocket('/ws/admin'))
    msg('AWARDED_TEAMS', { QUESTION_ID: '', TEAMS: [], PLAYERS: [{ ID: 'b1', TEAM: 'A', POINTS: 0, TIMESTAMP: 5 }, { ID: 'b2', TEAM: 'A', POINTS: 3, TIMESTAMP: 6 }] })
    expect(Object.keys(result.current.awardedPlayers)).toEqual(['b1', 'b2'])
    expect('b1' in result.current.awardedPlayers).toBe(true)
    expect(result.current.awardedPlayers.b1.POINTS).toBe(0)
    msg('AWARDED_TEAMS', { QUESTION_ID: '', TEAMS: [] })
    expect(result.current.awardedPlayers).toEqual({})
  })
})
