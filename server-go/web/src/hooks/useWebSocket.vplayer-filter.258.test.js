import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #258 (v11.2, scénario 10) — contrat vplayer-payload-filter.md §7.3 : les messages que la tablette reçoit pour START / STOP /
// PAUSE / CONTINUE / UPDATE_TIMER peuvent être sérialisés une fois pour toutes les tablettes avec `bumpers`/`teams` NEUTRALISÉS.
// Le client ne doit donc JAMAIS en tirer l'état des bumpers/équipes : seuls UPDATE (par destinataire) et l'état initial le font.
let wsInstances = []
class MockWebSocket {
  constructor(url) { this.url = url; this.readyState = 0; this.sentMessages = []; wsInstances.push(this) }
  send(d) { this.sentMessages.push(d) }
  close() { if (this.readyState === 3) return; this.readyState = 3; this.onclose && this.onclose() }
  simulateOpen() { this.readyState = 1; this.onopen && this.onopen() }
  simulateMessage(d) { this.onmessage && this.onmessage({ data: JSON.stringify(d) }) }
}
MockWebSocket.CONNECTING = 0; MockWebSocket.OPEN = 1; MockWebSocket.CLOSING = 2; MockWebSocket.CLOSED = 3

const GAME = { PHASE: 'STARTED', QUESTION: { ID: '9', TYPE: 'QCM' } }
const full = { GAME, teams: { A: { NAME: 'A', SCORE: 4, TIME: 99, BUMPER: 'x', STATUS: 'PAUSE' } }, bumpers: { x: { NAME: 'Paul', TEAM: 'A', TIME: 5, ANSWERED_COLOR: 'RED' } } }
const neutral = { GAME, teams: { A: { NAME: 'A', SCORE: 4, TIME: 0, BUMPER: '', STATUS: '' } }, bumpers: { x: { NAME: 'Paul', TEAM: 'A', TIME: 0, ANSWERED_COLOR: '' } } }

describe('useWebSocket — les actions START/STOP/PAUSE/CONTINUE/UPDATE_TIMER n’écrasent pas bumpers/teams (#258)', () => {
  beforeEach(() => { wsInstances = []; vi.useFakeTimers(); vi.stubGlobal('WebSocket', MockWebSocket) })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

  it.each(['START', 'STOP', 'PAUSE', 'CONTINUE', 'UPDATE_TIMER'])('Critical — %s portant des bumpers/équipes neutralisés : l’état reçu par UPDATE est conservé', (action) => {
    const hook = renderHook(() => useWebSocket('/ws/player'))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() })
    act(() => { ws.simulateMessage({ ACTION: 'UPDATE', MSG: full }) })
    expect(hook.result.current.bumpers.x.TIME).toBe(5)

    act(() => { ws.simulateMessage({ ACTION: action, MSG: neutral }) })

    expect(hook.result.current.bumpers.x.TIME, `${action} ne doit pas écraser bumpers`).toBe(5)
    expect(hook.result.current.bumpers.x.ANSWERED_COLOR).toBe('RED')
    expect(hook.result.current.teams.A.TIME, `${action} ne doit pas écraser teams`).toBe(99)
  })

  it('UPDATE (par destinataire) reste la source : un UPDATE neutralisé met bien à jour l’état', () => {
    const hook = renderHook(() => useWebSocket('/ws/player'))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() })
    act(() => { ws.simulateMessage({ ACTION: 'UPDATE', MSG: full }) })
    act(() => { ws.simulateMessage({ ACTION: 'UPDATE', MSG: neutral }) })
    expect(hook.result.current.bumpers.x.TIME).toBe(0)
  })
})
