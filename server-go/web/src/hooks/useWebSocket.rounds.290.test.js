import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #290 (v11.6.0, LOT-4) — ROUNDS_UPDATE côté hook : `ROUNDS[].AMBIANCE` en MAJUSCULES, absent ⇒ jamais envoyé (le serveur
// conserve), `{}` explicite ⇒ envoyé tel quel (efface) ; GameState.ROUNDS[].AMBIANCE et AMBIANCE_EFFECTIVE reçus par UPDATE.
// Contrat websocket-actions.md §ROUNDS_UPDATE, game-state.md. Même harnais que useWebSocket.rounds.245.test.js.
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

describe('useWebSocket — AMBIANCE des manches (#290)', () => {
  beforeEach(() => { wsInstances = []; vi.useFakeTimers(); vi.stubGlobal('WebSocket', MockWebSocket) })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })
  function open() {
    const hook = renderHook(() => useWebSocket('/ws/admin'))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() })
    return { hook, ws }
  }

  it('Critical — roundsUpdate transporte AMBIANCE (majuscules) ; absente ⇒ aucune clé ; {} ⇒ envoyé', () => {
    const { hook, ws } = open()
    const amb = { sound: { muted: true, cues: { 'qcm-indice': false } }, light: { mode: 'ON' } }
    act(() => {
      hook.result.current.roundsUpdate({
        rounds: [
          { id: 'r1', name: 'Un', questions: ['1'], ambiance: amb },
          { id: 'r2', name: 'Deux', questions: ['2'] },
          { id: 'r3', name: 'Trois', questions: ['3'], ambiance: {} },
        ],
        unassigned: [],
      })
    })
    const [msg] = sentOf(ws, 'ROUNDS_UPDATE')
    expect(msg.ROUNDS[0].AMBIANCE).toEqual(amb)
    expect('AMBIANCE' in msg.ROUNDS[1]).toBe(false)
    expect(msg.ROUNDS[2].AMBIANCE).toEqual({})
    expect(msg.ROUNDS[0].NAME).toBe('Un')
  })

  it('un envoi sans aucune surcharge est identique à v11.2 (aucune clé AMBIANCE)', () => {
    const { hook, ws } = open()
    act(() => { hook.result.current.roundsUpdate({ rounds: [{ id: 'r1', name: 'Un', questions: ['1'] }], unassigned: [] }) })
    expect(JSON.stringify(sentOf(ws, 'ROUNDS_UPDATE')[0])).not.toContain('AMBIANCE')
  })

  it('UPDATE : ROUNDS[].AMBIANCE et AMBIANCE_EFFECTIVE sont reçus dans l\'état du jeu', () => {
    const { hook, ws } = open()
    const eff = { SETTINGS: { sound: { muted: true }, light: { mode: 'ON' } }, SOURCES: { sound: { muted: 'round' }, light: { mode: 'round' } }, ROUND_ID: 'r1' }
    act(() => {
      ws.simulateMessage({ ACTION: 'UPDATE', MSG: { GAME: { PHASE: 'STOPPED', ROUNDS: [{ ID: 'r1', NAME: 'Un', AMBIANCE: { sound: { muted: true } } }], AMBIANCE_EFFECTIVE: eff } } })
    })
    const gs = hook.result.current.gameState
    expect(gs.AMBIANCE_EFFECTIVE).toEqual(eff)
    expect(gs.ROUNDS[0].AMBIANCE).toEqual({ sound: { muted: true } })
  })
})
