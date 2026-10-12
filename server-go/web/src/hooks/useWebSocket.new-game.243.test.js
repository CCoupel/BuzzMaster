import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #243 (v11.2) — NEW_GAME avec options (quatre champs toujours envoyés) et NEW_GAME_REFUSED ciblé.
// Contrats : websocket-actions.md §NEW_GAME (payload, rétrocompatibilité) et §NEW_GAME_REFUSED.
let wsInstances = []
class MockWebSocket {
  constructor(url) { this.url = url; this.readyState = 0; this.sentMessages = []; wsInstances.push(this) }
  send(d) { this.sentMessages.push(d) }
  close() { if (this.readyState === 3) return; this.readyState = 3; this.onclose && this.onclose() }
  simulateOpen() { this.readyState = 1; this.onopen && this.onopen() }
  simulateMessage(d) { this.onmessage && this.onmessage({ data: JSON.stringify(d) }) }
}
MockWebSocket.CONNECTING = 0; MockWebSocket.OPEN = 1; MockWebSocket.CLOSING = 2; MockWebSocket.CLOSED = 3
const sentOf = (ws, action) => ws.sentMessages.map((m) => JSON.parse(m)).filter((m) => m.ACTION === action).map((m) => m.MSG)

describe('useWebSocket — newGame(options) / NEW_GAME_REFUSED (#243)', () => {
  beforeEach(() => { wsInstances = []; vi.useFakeTimers(); vi.stubGlobal('WebSocket', MockWebSocket) })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })
  function open() {
    const hook = renderHook(() => useWebSocket('/ws/admin'))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() })
    return { hook, ws }
  }

  it('Critical — newGame({…}) envoie TOUJOURS les quatre champs en MAJUSCULES', () => {
    const { hook, ws } = open()
    act(() => { hook.result.current.newGame({ purgeScores: true, purgePlayers: false, purgeTeams: true, resetQuestions: false }) })
    expect(sentOf(ws, 'NEW_GAME')).toEqual([{ PURGE_SCORES: true, PURGE_PLAYERS: false, PURGE_TEAMS: true, RESET_QUESTIONS: false }])
  })

  it('une option omise est envoyée à false (jamais absente : sinon le serveur retomberait sur le comportement v11.1)', () => {
    const { hook, ws } = open()
    act(() => { hook.result.current.newGame({ purgeScores: true }) })
    const msg = sentOf(ws, 'NEW_GAME')[0]
    expect(Object.keys(msg).sort()).toEqual(['PURGE_PLAYERS', 'PURGE_SCORES', 'PURGE_TEAMS', 'RESET_QUESTIONS'])
    expect(msg.PURGE_PLAYERS).toBe(false)
    expect(msg.RESET_QUESTIONS).toBe(false)
  })

  it('sans options (ou événement de clic passé tel quel) : payload vide = comportement v11.1 inchangé', () => {
    const { hook, ws } = open()
    act(() => { hook.result.current.newGame() })
    act(() => { hook.result.current.newGame({ type: 'click', target: {} }) })
    expect(sentOf(ws, 'NEW_GAME')).toEqual([{}, {}])
  })

  it('NEW_GAME_REFUSED expose le motif et peut être consommé', () => {
    const { hook, ws } = open()
    act(() => { ws.simulateMessage({ ACTION: 'NEW_GAME_REFUSED', MSG: { REASON: 'PLAYERS_REQUIRE_SCORES' } }) })
    expect(hook.result.current.newGameRefused).toEqual({ reason: 'PLAYERS_REQUIRE_SCORES' })
    act(() => { hook.result.current.clearNewGameRefused() })
    expect(hook.result.current.newGameRefused).toBeNull()
  })
})
