import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #252 — setTeamPoints / setBumperPoints : champ DISPLAY seulement pour une correction cachée (jamais envoyé sinon :
// comportement v11.1 octet pour octet) ; GAME.SCORE_CHANGE suit les UPDATE. Contrats : websocket-actions.md §TEAM_POINTS,
// §BUMPER_POINTS ; game-state.md §SCORE_CHANGE.
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

describe('useWebSocket — DISPLAY et SCORE_CHANGE (#252)', () => {
  beforeEach(() => { wsInstances = []; vi.useFakeTimers(); vi.stubGlobal('WebSocket', MockWebSocket) })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })
  function open(ep = '/ws/admin') {
    const hook = renderHook(() => useWebSocket(ep))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() })
    return { hook, ws }
  }

  it('Critical — sans option : AUCUN champ DISPLAY (message identique à v11.1)', () => {
    const { hook, ws } = open()
    act(() => { hook.result.current.setTeamPoints('Les Bleus', 3); hook.result.current.setBumperPoints('AA:01', 2) })
    expect(sentOf(ws, 'TEAM_POINTS')).toEqual([{ TEAM: 'Les Bleus', POINTS: 3 }])
    expect(sentOf(ws, 'BUMPER_POINTS')).toEqual([{ ID: 'AA:01', POINTS: 2 }])
  })

  it('{display:true} : toujours aucun champ DISPLAY (absent = true)', () => {
    const { hook, ws } = open()
    act(() => { hook.result.current.setTeamPoints('Les Bleus', 3, { display: true }) })
    expect('DISPLAY' in sentOf(ws, 'TEAM_POINTS')[0]).toBe(false)
  })

  it('Critical — {display:false} : DISPLAY:false pour les équipes ET les joueurs', () => {
    const { hook, ws } = open()
    act(() => { hook.result.current.setTeamPoints('Les Bleus', -2, { display: false }); hook.result.current.setBumperPoints('AA:01', 5, { display: false }) })
    expect(sentOf(ws, 'TEAM_POINTS')).toEqual([{ TEAM: 'Les Bleus', POINTS: -2, DISPLAY: false }])
    expect(sentOf(ws, 'BUMPER_POINTS')).toEqual([{ ID: 'AA:01', POINTS: 5, DISPLAY: false }])
  })

  it('SCORE_CHANGE : valeur initiale {SEQ 0, DISPLAY true, listes vides}, puis suit l’UPDATE reçu', () => {
    const { hook, ws } = open()
    expect(hook.result.current.gameState.SCORE_CHANGE).toEqual({ SEQ: 0, DISPLAY: true, TEAMS: [], PLAYERS: [] })
    act(() => { ws.simulateMessage({ ACTION: 'UPDATE', MSG: { GAME: { PHASE: 'REVEALED', SCORE_CHANGE: { SEQ: 4, DISPLAY: false, TEAMS: ['Les Bleus'], PLAYERS: ['AA:01'] } }, teams: {}, bumpers: {} } }) })
    expect(hook.result.current.gameState.SCORE_CHANGE).toEqual({ SEQ: 4, DISPLAY: false, TEAMS: ['Les Bleus'], PLAYERS: ['AA:01'] })
  })

  it('un UPDATE sans SCORE_CHANGE (serveur antérieur) conserve la dernière valeur', () => {
    const { hook, ws } = open()
    act(() => { ws.simulateMessage({ ACTION: 'UPDATE', MSG: { GAME: { SCORE_CHANGE: { SEQ: 2, DISPLAY: true, TEAMS: ['A'], PLAYERS: [] } }, teams: {}, bumpers: {} } }) })
    act(() => { ws.simulateMessage({ ACTION: 'UPDATE', MSG: { GAME: { PHASE: 'STARTED' }, teams: {}, bumpers: {} } }) })
    expect(hook.result.current.gameState.SCORE_CHANGE.SEQ).toBe(2)
  })

  it('SCORE_CHANGE est reçu par tous les types de client (TV, tablette, anim)', () => {
    for (const ep of ['/ws/tv', '/ws/player', '/ws/anim']) {
      wsInstances = []
      const { hook, ws } = open(ep)
      act(() => { ws.simulateMessage({ ACTION: 'UPDATE', MSG: { GAME: { SCORE_CHANGE: { SEQ: 1, DISPLAY: false, TEAMS: ['A'], PLAYERS: [] } } } }) })
      expect(hook.result.current.gameState.SCORE_CHANGE.DISPLAY, ep).toBe(false)
    }
  })
})
