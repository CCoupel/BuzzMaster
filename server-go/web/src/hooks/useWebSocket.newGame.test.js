import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #243 — newGame(options) et NEW_GAME_REFUSED.

let ws
class MockWebSocket {
  constructor() { ws = this; this.readyState = 0; this.sent = [] }
  send(m) { this.sent.push(JSON.parse(m)) }
  close() { this.readyState = 3 }
}
MockWebSocket.OPEN = 1

describe('useWebSocket — NEW_GAME (#243)', () => {
  beforeEach(() => vi.stubGlobal('WebSocket', MockWebSocket))
  afterEach(() => vi.unstubAllGlobals())
  const lastNewGame = () => ws.sent.filter((m) => m.ACTION === 'NEW_GAME').at(-1).MSG

  it('options → les quatre champs toujours envoyés', () => {
    const { result } = renderHook(() => useWebSocket('/ws/admin'))
    ws.readyState = 1
    act(() => result.current.newGame({ purgeScores: true, purgePlayers: false, purgeTeams: true, resetQuestions: false }))
    expect(lastNewGame()).toEqual({ PURGE_SCORES: true, PURGE_PLAYERS: false, PURGE_TEAMS: true, RESET_QUESTIONS: false })
  })

  it('sans options (ou événement de clic) → payload vide (comportement v11.1)', () => {
    const { result } = renderHook(() => useWebSocket('/ws/admin'))
    ws.readyState = 1
    act(() => result.current.newGame())
    expect(lastNewGame()).toEqual({})
    act(() => result.current.newGame({ type: 'click' }))
    expect(lastNewGame()).toEqual({})
  })

  it('NEW_GAME_REFUSED exposé puis consommé', () => {
    const { result } = renderHook(() => useWebSocket('/ws/admin'))
    act(() => { ws.onmessage({ data: JSON.stringify({ ACTION: 'NEW_GAME_REFUSED', MSG: { REASON: 'PLAYERS_REQUIRE_SCORES' } }) }) })
    expect(result.current.newGameRefused).toEqual({ reason: 'PLAYERS_REQUIRE_SCORES' })
    act(() => result.current.clearNewGameRefused())
    expect(result.current.newGameRefused).toBeNull()
  })
})
