import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #252 — SCORE_CHANGE dans le réducteur UPDATE et option DISPLAY des crédits.

let ws
class MockWebSocket {
  constructor() { ws = this; this.readyState = 0; this.sent = [] }
  send(m) { this.sent.push(JSON.parse(m)) }
  close() { this.readyState = 3 }
}
MockWebSocket.OPEN = 1
const msg = (ACTION, MSG) => act(() => { ws.onmessage({ data: JSON.stringify({ ACTION, MSG }) }) })

describe('useWebSocket — SCORE_CHANGE et DISPLAY (#252)', () => {
  beforeEach(() => vi.stubGlobal('WebSocket', MockWebSocket))
  afterEach(() => vi.unstubAllGlobals())

  it('SCORE_CHANGE : valeur par défaut, suit chaque UPDATE, conservé si absent', () => {
    const { result } = renderHook(() => useWebSocket('/ws/tv'))
    expect(result.current.gameState.SCORE_CHANGE).toEqual({ SEQ: 0, DISPLAY: true, TEAMS: [], PLAYERS: [] })
    msg('UPDATE', { GAME: { PHASE: 'REVEALED', SCORE_CHANGE: { SEQ: 3, DISPLAY: false, TEAMS: ['A'], PLAYERS: [] } } })
    expect(result.current.gameState.SCORE_CHANGE).toEqual({ SEQ: 3, DISPLAY: false, TEAMS: ['A'], PLAYERS: [] })
    msg('UPDATE', { GAME: { PHASE: 'REVEALED' } })
    expect(result.current.gameState.SCORE_CHANGE.SEQ).toBe(3)
  })

  it('setTeamPoints / setBumperPoints : DISPLAY:false seulement si options.display === false', () => {
    const { result } = renderHook(() => useWebSocket('/ws/admin'))
    ws.readyState = 1
    act(() => {
      result.current.setTeamPoints('A', 3)
      result.current.setTeamPoints('A', 3, { display: true })
      result.current.setTeamPoints('A', -2, { display: false })
      result.current.setBumperPoints('b1', 5, { display: false })
      result.current.setBumperPoints('b1', 5)
    })
    const sent = ws.sent.filter(m => m.ACTION === 'TEAM_POINTS' || m.ACTION === 'BUMPER_POINTS').map(m => m.MSG)
    expect(sent[0]).toEqual({ TEAM: 'A', POINTS: 3 })
    expect(sent[1]).toEqual({ TEAM: 'A', POINTS: 3 })
    expect(sent[2]).toEqual({ TEAM: 'A', POINTS: -2, DISPLAY: false })
    expect(sent[3]).toEqual({ ID: 'b1', POINTS: 5, DISPLAY: false })
    expect(sent[4]).toEqual({ ID: 'b1', POINTS: 5 })
  })
})
