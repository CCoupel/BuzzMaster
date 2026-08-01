import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// ---------------------------------------------------------------------------
// Tests : useWebSocket — surveillance de liaison morte + reconnexion
// autonome (#118, F1/F5, R6/R5)
//
// Cause racine (_work/reports/plan-20260729-190000.md) : le serveur détecte
// déjà une liaison morte (ping/pong + read deadline 5s), mais le CLIENT n'a
// aucune preuve de vie exploitable — sur une vraie coupure réseau, la trame
// de fermeture du serveur ne traverse jamais un lien coupé : `onclose` ne se
// déclenche pas, `status` reste 'connected', et le socket devient un zombie
// (`readyState === OPEN`) qui bloque `connect()` pour de bon. Seul un
// rechargement de page recrée un socket — le contournement observé.
//
// Le fix : le serveur émet HEARTBEAT{INTERVAL_MS} sur son ticker existant
// (voir heartbeat_test.go, package server) ; le client surveille l'arrivée de
// N'IMPORTE QUEL message (HEARTBEAT compris) et, au-delà de 3× la cadence
// annoncée sans nouvelle, force la fermeture du socket zombie AVANT de
// remettre la référence à null — ce qui est précisément ce qui empêche la
// garde de `connect()` de rester verrouillée.
// ---------------------------------------------------------------------------

let wsInstances = []

class MockWebSocket {
  constructor(url) {
    this.url = url
    this.readyState = MockWebSocket.CONNECTING
    this.sentMessages = []
    wsInstances.push(this)
  }
  send(data) { this.sentMessages.push(data) }
  close() {
    if (this.readyState === MockWebSocket.CLOSED) return
    this.readyState = MockWebSocket.CLOSED
    this.onclose && this.onclose()
  }
  // --- test-only helpers, not part of the real WebSocket API ---
  simulateOpen() {
    this.readyState = MockWebSocket.OPEN
    this.onopen && this.onopen()
  }
  simulateMessage(data) {
    this.onmessage && this.onmessage({ data: JSON.stringify(data) })
  }
}
MockWebSocket.CONNECTING = 0
MockWebSocket.OPEN = 1
MockWebSocket.CLOSING = 2
MockWebSocket.CLOSED = 3

function heartbeat(intervalMs) {
  return { ACTION: 'HEARTBEAT', MSG: { INTERVAL_MS: intervalMs } }
}

function sentActions(ws) {
  return ws.sentMessages.map(m => JSON.parse(m).ACTION)
}

describe('useWebSocket — surveillance de liaison morte (#118, F1/R6)', () => {
  beforeEach(() => {
    wsInstances = []
    vi.useFakeTimers()
    vi.stubGlobal('WebSocket', MockWebSocket)
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('sans message reçu au-delà de 3× la cadence annoncée : ferme le socket, passe disconnected, programme une reconnexion', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    const ws1 = wsInstances[0]
    act(() => { ws1.simulateOpen() })
    act(() => { ws1.simulateMessage(heartbeat(3000)) }) // announced cadence: 3000ms -> threshold 9000ms

    // Comfortably below the 9s threshold.
    act(() => { vi.advanceTimersByTime(5000) })
    expect(result.current.status).toBe('connected')
    expect(ws1.readyState).toBe(MockWebSocket.OPEN)

    // Past the 9s threshold, with margin for the internal 1s check-loop
    // granularity (detection lands by t=10000 at the latest) — but
    // calibrated to stop BEFORE the dispersed reconnect (F5, jitter range
    // 3500-6500ms after detection) has any chance to fire: detection is
    // guaranteed by t=10000, so the earliest possible reconnect is
    // t=13500. Stopping at t=11000 (5000+6000) leaves a safety margin on
    // both sides without depending on the jitter draw (fix: the previous
    // 10000ms advance reached t=15000, which flakily overlapped the
    // reconnect window on a low jitter draw and intermittently left
    // status as 'connecting' instead of 'disconnected').
    act(() => { vi.advanceTimersByTime(6000) })
    expect(ws1.readyState).toBe(MockWebSocket.CLOSED)
    expect(result.current.status).toBe('disconnected')

    // A reconnection must eventually be scheduled — RECONNECT_INTERVAL is
    // dispersed (F5, up to 6500ms) so allow generous margin.
    act(() => { vi.advanceTimersByTime(10000) })
    expect(wsInstances.length).toBeGreaterThan(1)
  })

  it('tout message reçu réarme la surveillance, HEARTBEAT compris', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    const ws1 = wsInstances[0]
    act(() => { ws1.simulateOpen() })
    act(() => { ws1.simulateMessage(heartbeat(3000)) })

    act(() => { vi.advanceTimersByTime(8000) }) // just under the 9s threshold
    act(() => { ws1.simulateMessage(heartbeat(3000)) }) // any message resets the clock

    act(() => { vi.advanceTimersByTime(8000) }) // would total 16s since the FIRST message, but only 8s since the last
    expect(result.current.status).toBe('connected')
    expect(ws1.readyState).toBe(MockWebSocket.OPEN)
  })

  it('le seuil suit la cadence annoncée : un HEARTBEAT à 5000ms déplace le seuil à 15000ms', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    const ws1 = wsInstances[0]
    act(() => { ws1.simulateOpen() })
    act(() => { ws1.simulateMessage(heartbeat(5000)) })

    // Comfortably below the derived 15s threshold.
    act(() => { vi.advanceTimersByTime(9000) })
    expect(ws1.readyState).toBe(MockWebSocket.OPEN)
    expect(result.current.status).toBe('connected')

    // Comfortably past it.
    act(() => { vi.advanceTimersByTime(10000) }) // total 19s since last message
    expect(ws1.readyState).toBe(MockWebSocket.CLOSED)
    expect(result.current.status).toBe('disconnected')
  })

  it('repli raisonnable avant tout HEARTBEAT reçu : la surveillance ne bloque pas indéfiniment', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    const ws1 = wsInstances[0]
    act(() => { ws1.simulateOpen() }) // no HEARTBEAT ever arrives on this socket

    // Comfortably past the fallback threshold derived from the server's
    // known 3s cadence (i.e. past 3×3000ms=9000ms), but calibrated to stay
    // BEFORE the dispersed reconnect (F5, up to RECONNECT_INTERVAL+30% =
    // 6500ms) has a chance to fire and flip status back to 'connecting' —
    // that reconnect scheduling is checked separately by the sibling test
    // above, not here (fix: was 20000ms, which had already let the
    // reconnect fire and create a new — unopened — socket by the time of
    // the assertion below).
    act(() => { vi.advanceTimersByTime(10000) })

    expect(ws1.readyState).toBe(MockWebSocket.CLOSED)
    expect(result.current.status).toBe('disconnected')
  })

  it('le client n\'émet aucun message périodique (choix d\'architecture : surveillance passive uniquement)', () => {
    renderHook(() => useWebSocket('/ws/test'))
    const ws1 = wsInstances[0]
    act(() => { ws1.simulateOpen() })
    act(() => { ws1.simulateMessage(heartbeat(3000)) })

    act(() => { vi.advanceTimersByTime(30000) })

    // Only the initial HELLO (sent onopen) — never a client-emitted heartbeat/ping.
    expect(sentActions(ws1)).toEqual(['HELLO'])
  })

  it('socket zombie (readyState OPEN, plus aucun message) : la reconnexion aboutit malgré la garde de connect()', () => {
    renderHook(() => useWebSocket('/ws/test'))
    const ws1 = wsInstances[0]
    act(() => { ws1.simulateOpen() })
    // Network genuinely dead: no HEARTBEAT, no onclose ever fires from the
    // outside — nothing but the client's own monitor will ever close this.
    expect(ws1.readyState).toBe(MockWebSocket.OPEN)

    act(() => { vi.advanceTimersByTime(20000) })
    expect(ws1.readyState).toBe(MockWebSocket.CLOSED) // forced closed by the monitor

    act(() => { vi.advanceTimersByTime(10000) }) // let the scheduled reconnect fire
    expect(wsInstances.length).toBeGreaterThan(1)
    const ws2 = wsInstances[wsInstances.length - 1]
    expect(ws2).not.toBe(ws1)
  })

  it('aucun minuteur ne survit au démontage (battement, surveillance, reconnexion)', () => {
    const { unmount } = renderHook(() => useWebSocket('/ws/test'))
    const ws1 = wsInstances[0]
    act(() => { ws1.simulateOpen() })
    act(() => { ws1.simulateMessage(heartbeat(3000)) })

    unmount()

    expect(vi.getTimerCount()).toBe(0)
  })

  describe('F5 — dispersion de l\'intervalle de reconnexion', () => {
    // Probe how long it takes, in fake-timer terms, for a new WebSocket
    // instance to appear after `countBefore` — a black-box way to measure
    // the scheduled reconnect delay without assuming which internal timer
    // produces it.
    function timeToNextInstance(countBefore, maxMs = 20000, stepMs = 50) {
      let elapsed = 0
      while (wsInstances.length === countBefore && elapsed < maxMs) {
        act(() => { vi.advanceTimersByTime(stepMs) })
        elapsed += stepMs
      }
      return elapsed
    }

    it('deux reconnexions successives n\'utilisent pas exactement le même délai', () => {
      // Two-phase advance per cycle: first land JUST past the zombie-detection
      // point (comfortably before the earliest possible reconnect delay could
      // fire), confirm the close, THEN start the probe from there. Advancing
      // straight through both the detection AND a plausible reconnect delay
      // in one jump would let the new instance appear inside that single
      // jump, making the measured "delay" collapse to 0.
      renderHook(() => useWebSocket('/ws/test'))
      let ws = wsInstances[0]
      act(() => { ws.simulateOpen() })

      const randomSpy = vi.spyOn(Math, 'random')

      randomSpy.mockReturnValue(0) // -> RECONNECT_INTERVAL - 30% = 3500ms
      act(() => { vi.advanceTimersByTime(10100) }) // just past the 9s detection point (1s check granularity)
      expect(ws.readyState).toBe(MockWebSocket.CLOSED)
      const delay1 = timeToNextInstance(1)
      expect(wsInstances.length).toBe(2)

      ws = wsInstances[1]
      act(() => { ws.simulateOpen() })

      randomSpy.mockReturnValue(0.999) // -> RECONNECT_INTERVAL + ~30% ≈ 6500ms
      act(() => { vi.advanceTimersByTime(10100) })
      expect(ws.readyState).toBe(MockWebSocket.CLOSED)
      const delay2 = timeToNextInstance(2)
      expect(wsInstances.length).toBe(3)

      expect(delay1).not.toBe(delay2)
      // Sanity: both plausible reconnect delays, not one of them being an
      // immediate/zero retry and the other a full timeout of the probe.
      expect(delay1).toBeGreaterThan(0)
      expect(delay2).toBeGreaterThan(0)

      randomSpy.mockRestore()
    })
  })
})
