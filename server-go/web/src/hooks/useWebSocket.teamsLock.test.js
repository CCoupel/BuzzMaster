import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #249 — TEAMS_LOCKED (réducteur UPDATE) et TEAMS_UPDATE_REFUSED.

let ws
class MockWebSocket {
  constructor() { ws = this; this.readyState = 0 }
  send() {}
  close() { this.readyState = 3 }
}
MockWebSocket.OPEN = 1

const msg = (ACTION, MSG) => act(() => { ws.onmessage({ data: JSON.stringify({ ACTION, MSG }) }) })

describe('useWebSocket — verrou des équipes (#249)', () => {
  beforeEach(() => vi.stubGlobal('WebSocket', MockWebSocket))
  afterEach(() => vi.unstubAllGlobals())

  it('TEAMS_LOCKED : faux par défaut, suit chaque UPDATE, conservé s\'il est absent', () => {
    const { result } = renderHook(() => useWebSocket('/ws/admin'))
    expect(result.current.gameState.TEAMS_LOCKED).toBe(false)
    msg('UPDATE', { GAME: { PHASE: 'STARTED', TEAMS_LOCKED: true } })
    expect(result.current.gameState.TEAMS_LOCKED).toBe(true)
    msg('UPDATE', { GAME: { PHASE: 'STARTED' } })
    expect(result.current.gameState.TEAMS_LOCKED).toBe(true)
    msg('UPDATE', { GAME: { PHASE: 'REVEALED', TEAMS_LOCKED: false } })
    expect(result.current.gameState.TEAMS_LOCKED).toBe(false)
  })

  it('TEAMS_UPDATE_REFUSED : exposé puis consommé', () => {
    const { result } = renderHook(() => useWebSocket('/ws/admin'))
    expect(result.current.teamsUpdateRefused).toBeNull()
    msg('TEAMS_UPDATE_REFUSED', { REASON: 'TEAMS_LOCKED_DURING_QUESTION', TEAMS: ['Les Bleus'] })
    expect(result.current.teamsUpdateRefused).toEqual({ reason: 'TEAMS_LOCKED_DURING_QUESTION', teams: ['Les Bleus'] })
    act(() => result.current.clearTeamsUpdateRefused())
    expect(result.current.teamsUpdateRefused).toBeNull()
  })
})
