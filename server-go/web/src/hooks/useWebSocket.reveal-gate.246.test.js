import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #246 (v11.2, Batch 2) — REVEAL forcé et réponses ciblées du serveur côté hook.
// Contrat : websocket-actions.md §REVEAL (FORCE), §REVEAL_REFUSED, §ARDOISE_FINAL_ACK.

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

describe('useWebSocket — REVEAL forcé / REVEAL_REFUSED / ARDOISE_FINAL_ACK (#246)', () => {
  beforeEach(() => { wsInstances = []; vi.useFakeTimers(); vi.stubGlobal('WebSocket', MockWebSocket) })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

  function open(endpoint = '/ws/admin') {
    const hook = renderHook(() => useWebSocket(endpoint))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() })
    return { hook, ws }
  }

  it('revealAnswer() envoie REVEAL sans FORCE', () => {
    const { hook, ws } = open()
    act(() => { hook.result.current.revealAnswer() })
    const m = sentOf(ws, 'REVEAL')
    expect(m.length).toBe(1)
    expect(m[0].FORCE).not.toBe(true)
  })

  it('revealAnswer(true) envoie REVEAL {FORCE: true}', () => {
    const { hook, ws } = open()
    act(() => { hook.result.current.revealAnswer(true) })
    expect(sentOf(ws, 'REVEAL')).toEqual([{ FORCE: true }])
  })

  it('Critical — un événement de clic passé tel quel (onClick={revealAnswer}) n’est JAMAIS un forçage', () => {
    const { hook, ws } = open()
    act(() => { hook.result.current.revealAnswer({ type: 'click', target: {} }) })
    expect(sentOf(ws, 'REVEAL')[0].FORCE).not.toBe(true)
  })

  it('REVEAL_REFUSED reçu expose le motif et les équipes en attente (hors ligne comprise)', () => {
    const { hook, ws } = open()
    act(() => {
      ws.simulateMessage({ ACTION: 'REVEAL_REFUSED', MSG: { REASON: 'ARDOISE_PENDING', PENDING: [{ TEAM: 'Les Verts', OFFLINE: true }, { TEAM: 'Les Gris', OFFLINE: false }] } })
    })
    const r = hook.result.current.revealRefused
    expect(r, 'le hook doit exposer revealRefused après REVEAL_REFUSED').toBeTruthy()
    expect(r.reason).toBe('ARDOISE_PENDING')
    expect(r.pending).toEqual([{ team: 'Les Verts', offline: true }, { team: 'Les Gris', offline: false }])
  })

  it('ARDOISE_FINAL_ACK reçu expose QUESTION_ID et HASH (sans texte)', () => {
    const { hook, ws } = open('/ws/player')
    act(() => { ws.simulateMessage({ ACTION: 'ARDOISE_FINAL_ACK', MSG: { QUESTION_ID: 'q42', HASH: '066a01e6' } }) })
    const a = hook.result.current.ardoiseFinalAck
    expect(a, 'le hook doit exposer ardoiseFinalAck après ARDOISE_FINAL_ACK').toBeTruthy()
    expect(a).toMatchObject({ questionId: 'q42', hash: '066a01e6' })
  })
})
