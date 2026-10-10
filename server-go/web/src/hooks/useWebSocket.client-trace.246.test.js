import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #246 (v11.2) — CLIENT_TRACE émis par le hook : cycle de vie de la page (VISIBILITY, PAGESHOW,
// FREEZE/RESUME), coupures (WS_GAP avec envois abandonnés), remise à vide vue par l'admin
// (ARDOISE_CLEARED_SEEN). Contrat : contracts/websocket-actions.md §CLIENT_TRACE.

let wsInstances = []
class MockWebSocket {
  constructor(url) { this.url = url; this.readyState = 0; this.sentMessages = []; wsInstances.push(this) }
  send(d) { this.sentMessages.push(d) }
  close() { if (this.readyState === 3) return; this.readyState = 3; this.onclose && this.onclose() }
  simulateOpen() { this.readyState = 1; this.onopen && this.onopen() }
  simulateMessage(d) { this.onmessage && this.onmessage({ data: JSON.stringify(d) }) }
}
MockWebSocket.CONNECTING = 0; MockWebSocket.OPEN = 1; MockWebSocket.CLOSING = 2; MockWebSocket.CLOSED = 3

const traces = (ws) => ws.sentMessages.map((m) => JSON.parse(m)).filter((m) => m.ACTION === 'CLIENT_TRACE').map((m) => m.MSG)
const setVisibility = (state) => {
  Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => state })
  document.dispatchEvent(new Event('visibilitychange'))
}

describe('useWebSocket — CLIENT_TRACE (#246)', () => {
  beforeEach(() => { wsInstances = []; vi.useFakeTimers(); vi.stubGlobal('WebSocket', MockWebSocket) })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals(); vi.restoreAllMocks() })

  function open(endpoint = '/ws/player') {
    const hook = renderHook(() => useWebSocket(endpoint))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() })
    return { hook, ws }
  }

  it('VISIBILITY hidden puis visible (avec HIDDEN_MS), horodatés AT', () => {
    const { ws } = open()
    act(() => { setVisibility('hidden') })
    act(() => { vi.advanceTimersByTime(1500) })
    act(() => { setVisibility('visible') })

    const v = traces(ws).filter((t) => t.KIND === 'VISIBILITY')
    expect(v.map((t) => t.DETAIL.STATE)).toEqual(['hidden', 'visible'])
    expect(v[1].DETAIL.HIDDEN_MS).toBeGreaterThanOrEqual(1400)
    for (const t of v) expect(typeof t.AT).toBe('number')
  })

  it('PAGESHOW porte DETAIL.PERSISTED (restauration bfcache)', () => {
    const { ws } = open()
    act(() => {
      const e = new Event('pageshow'); e.persisted = true
      window.dispatchEvent(e)
    })
    const pageshow = traces(ws).filter((t) => t.KIND === 'PAGESHOW')
    expect(pageshow.length, 'un CLIENT_TRACE de KIND PAGESHOW doit être émis sur l’événement pageshow').toBe(1)
    expect(pageshow[0].DETAIL?.PERSISTED, 'DETAIL.PERSISTED (restauration bfcache)').toBe(true)
  })

  it('FREEZE puis RESUME', () => {
    const { ws } = open()
    act(() => { document.dispatchEvent(new Event('freeze')) })
    act(() => { document.dispatchEvent(new Event('resume')) })
    expect(traces(ws).map((t) => t.KIND)).toEqual(expect.arrayContaining(['FREEZE', 'RESUME']))
  })

  it('WS_GAP à la reconnexion : durée de la coupure et envois abandonnés pendant celle-ci', () => {
    const { hook, ws } = open()
    act(() => { ws.close() }) // coupure
    act(() => { vi.advanceTimersByTime(1000) })
    act(() => { hook.result.current.sendMessage('ARDOISE_INPUT', { TEXT: 'A', ID: 'x' }) })
    act(() => { hook.result.current.sendMessage('ARDOISE_INPUT', { TEXT: 'AB', ID: 'x' }) })
    act(() => { vi.advanceTimersByTime(8000) }) // reconnexion automatique
    const ws2 = wsInstances[wsInstances.length - 1]
    expect(ws2).not.toBe(ws)
    act(() => { ws2.simulateOpen() })

    const gap = traces(ws2).find((t) => t.KIND === 'WS_GAP')
    expect(gap).toBeTruthy()
    expect(gap.DETAIL.DROPPED_SENDS).toBe(2)
    expect(gap.DETAIL.DOWN_MS).toBeGreaterThanOrEqual(1000)
  })

  it('WS_GAP sans envoi abandonné : DROPPED_SENDS = 0', () => {
    const { ws } = open()
    act(() => { ws.close() })
    act(() => { vi.advanceTimersByTime(8000) })
    const ws2 = wsInstances[wsInstances.length - 1]
    act(() => { ws2.simulateOpen() })
    const gap = traces(ws2).find((t) => t.KIND === 'WS_GAP')
    expect(gap, 'un CLIENT_TRACE WS_GAP doit être émis à la reconnexion même sans envoi abandonné').toBeTruthy()
    expect(gap.DETAIL?.DROPPED_SENDS, 'DETAIL.DROPPED_SENDS').toBe(0)
  })

  describe('ARDOISE_CLEARED_SEEN (admin / anim)', () => {
    const update = (qid, answers, phase = 'STARTED') => ({
      ACTION: 'UPDATE',
      MSG: { GAME: { PHASE: phase, QUESTION: { ID: qid, TYPE: 'ARDOISE' }, ARDOISE_ANSWERS: answers } },
    })
    const ans = (text) => ({ 'Les Verts': { TEXT: text, STARTED_AT: 1, SUBMITTED_AT: 2 } })

    for (const endpoint of ['/ws/admin', '/ws/anim']) {
      it(`${endpoint} : texte non vide → vide pour la même question = trace avec TEAM, PREV_LEN, CAUSE_ACTION`, () => {
        const { ws } = open(endpoint)
        act(() => { ws.simulateMessage(update('q1', ans('Paris la capitale'))) })
        act(() => { ws.simulateMessage(update('q1', ans(''))) })

        const t = traces(ws).filter((x) => x.KIND === 'ARDOISE_CLEARED_SEEN')
        expect(t.length).toBe(1)
        expect(t[0]).toMatchObject({ TEAM: 'Les Verts', PREV_LEN: 17, CAUSE_ACTION: 'UPDATE' })
      })
    }

    it('une équipe qui disparaît de l’état est aussi tracée', () => {
      const { ws } = open('/ws/admin')
      act(() => { ws.simulateMessage(update('q1', ans('Paris'))) })
      act(() => { ws.simulateMessage(update('q1', {})) })
      expect(traces(ws).filter((x) => x.KIND === 'ARDOISE_CLEARED_SEEN').length).toBe(1)
    })

    it('pas de trace pour une saisie normale, ni pour une nouvelle question, ni pour PREPARE', () => {
      const { ws } = open('/ws/admin')
      act(() => { ws.simulateMessage(update('q1', ans('P'))) })
      act(() => { ws.simulateMessage(update('q1', ans('Pa'))) })
      act(() => { ws.simulateMessage(update('q2', ans(''))) })  // nouvelle question : remise à zéro légitime
      act(() => { ws.simulateMessage(update('q2', ans('X'))) })
      act(() => { ws.simulateMessage(update('q2', ans(''), 'PREPARE')) })
      expect(traces(ws).filter((x) => x.KIND === 'ARDOISE_CLEARED_SEEN')).toEqual([])
    })

    it('un écran TV ou une tablette n’émet pas ARDOISE_CLEARED_SEEN', () => {
      const { ws } = open('/ws/tv')
      act(() => { ws.simulateMessage(update('q1', ans('Paris'))) })
      act(() => { ws.simulateMessage(update('q1', ans(''))) })
      expect(traces(ws).filter((x) => x.KIND === 'ARDOISE_CLEARED_SEEN')).toEqual([])
    })
  })
})
