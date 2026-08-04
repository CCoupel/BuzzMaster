import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// ---------------------------------------------------------------------------
// Tests : useWebSocket — seuil de liaison morte transmis par le serveur (#130, T2.2)
//
// Cause racine (_work/reports/planner-20260803-214210.md) : la marge apparente
// de 2s entre le ping serveur (3s) et son ReadDeadline (5s) ne tolérait en
// réalité la perte d'AUCUN ping — un seul ping perdu reportait le pong suivant
// à 6s, au-delà du délai. #130 resserre la cadence serveur à 2s, porte le
// ReadDeadline à 7s (tolérance réelle de 2 pings perdus), et transmet
// désormais le seuil de liaison morte en VALEUR ABSOLUE côté client
// (`HEARTBEAT.DEAD_LINK_TIMEOUT_MS`) plutôt que de le faire déduire d'un
// multiplicateur codé en dur.
//
// ⚠️ Ajustement validé par l'utilisateur au GATE 2 (contrat liveness-timing.md,
// valeur serveur réelle `deadLinkTimeout = internal/server/websocket.go:601`) :
// DEAD_LINK_TIMEOUT_MS = 4000 (pas 5000 comme dans la version initiale du
// plan). Ce fichier de test n'encode PAS cette valeur en dur dans le code
// testé (elle vient du serveur) — seules les bornes horaires des tests CA6/CA7
// sont adaptées à 4000ms.
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

// heartbeat({intervalMs, deadLinkTimeoutMs}) — deadLinkTimeoutMs omis pour
// simuler un serveur antérieur à #130 (compat CA8).
function heartbeat({ intervalMs, deadLinkTimeoutMs } = {}) {
  const MSG = {}
  if (intervalMs !== undefined) MSG.INTERVAL_MS = intervalMs
  if (deadLinkTimeoutMs !== undefined) MSG.DEAD_LINK_TIMEOUT_MS = deadLinkTimeoutMs
  return { ACTION: 'HEARTBEAT', MSG }
}

describe('useWebSocket — cascade du seuil de liaison morte (#130, CA5)', () => {
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

  it('branche 1 — DEAD_LINK_TIMEOUT_MS reçu et valide : le seuil vaut exactement cette valeur', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() })
    act(() => { ws.simulateMessage(heartbeat({ intervalMs: 2000, deadLinkTimeoutMs: 4000 })) })

    // Comfortablement sous 4000ms.
    act(() => { vi.advanceTimersByTime(3500) })
    expect(result.current.status).toBe('connected')
    expect(ws.readyState).toBe(MockWebSocket.OPEN)

    // Comfortablement au-delà.
    act(() => { vi.advanceTimersByTime(1000) }) // total 4500ms
    expect(ws.readyState).toBe(MockWebSocket.CLOSED)
    expect(result.current.status).toBe('disconnected')
  })

  it('branche 2 — DEAD_LINK_TIMEOUT_MS absent, INTERVAL_MS reçu : repli sur INTERVAL_MS × 3 (serveur antérieur à #130)', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() })
    act(() => { ws.simulateMessage(heartbeat({ intervalMs: 2000 })) }) // pas de DEAD_LINK_TIMEOUT_MS -> seuil dérivé = 6000ms

    act(() => { vi.advanceTimersByTime(5500) })
    expect(result.current.status).toBe('connected')
    expect(ws.readyState).toBe(MockWebSocket.OPEN)

    act(() => { vi.advanceTimersByTime(1000) }) // total 6500ms
    expect(ws.readyState).toBe(MockWebSocket.CLOSED)
    expect(result.current.status).toBe('disconnected')
  })

  it('branche 3 — aucun HEARTBEAT jamais reçu : repli total (FALLBACK 3000ms × 3 = 9000ms)', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() }) // aucun HEARTBEAT ne parvient jamais sur ce socket

    act(() => { vi.advanceTimersByTime(8500) })
    expect(result.current.status).toBe('connected')
    expect(ws.readyState).toBe(MockWebSocket.OPEN)

    act(() => { vi.advanceTimersByTime(1000) }) // total 9500ms
    expect(ws.readyState).toBe(MockWebSocket.CLOSED)
  })
})

describe('useWebSocket — bornes horaires CA6/CA7 (#130, DEAD_LINK_TIMEOUT_MS=4000)', () => {
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

  it('CA6 — un silence de 3s ne déclenche AUCUNE reconnexion (marge réduite à 1s mais toujours positive)', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() })
    act(() => { ws.simulateMessage(heartbeat({ intervalMs: 2000, deadLinkTimeoutMs: 4000 })) })

    act(() => { vi.advanceTimersByTime(3000) })

    expect(ws.readyState).toBe(MockWebSocket.OPEN)
    expect(result.current.status).toBe('connected')
  })

  it('CA7 (valeur ajustée) — un lien mort déclenche closeZombieSocket entre 4,0 et 4,5s', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() })
    act(() => { ws.simulateMessage(heartbeat({ intervalMs: 2000, deadLinkTimeoutMs: 4000 })) })

    // Juste avant le seuil — toujours ouvert.
    act(() => { vi.advanceTimersByTime(3999) })
    expect(ws.readyState).toBe(MockWebSocket.OPEN)

    // Détection garantie au plus tard à 4500ms (granularité de vérification
    // 500ms, LIVENESS_CHECK_INTERVAL_MS).
    act(() => { vi.advanceTimersByTime(501) }) // total 4500ms
    expect(ws.readyState).toBe(MockWebSocket.CLOSED)
    expect(result.current.status).toBe('disconnected')
  })

  it('tout message reçu réarme la surveillance : un silence de 3,9s juste après un message reste sous le seuil de 4s', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() })
    act(() => { ws.simulateMessage(heartbeat({ intervalMs: 2000, deadLinkTimeoutMs: 4000 })) })

    act(() => { vi.advanceTimersByTime(3500) }) // juste sous le seuil
    act(() => { ws.simulateMessage(heartbeat({ intervalMs: 2000, deadLinkTimeoutMs: 4000 })) }) // réarme l'horloge

    act(() => { vi.advanceTimersByTime(3900) }) // total 7400ms depuis l'ouverture, mais 3900ms depuis le dernier message
    expect(result.current.status).toBe('connected')
    expect(ws.readyState).toBe(MockWebSocket.OPEN)
  })
})

describe('useWebSocket — garde de robustesse sur DEAD_LINK_TIMEOUT_MS aberrant (#130, R3)', () => {
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

  it.each([
    ['zéro', 0],
    ['négatif', -1000],
    ['non numérique (chaîne)', '4000'],
    ['non numérique (null)', null],
    ['non numérique (objet)', {}],
    ['inférieur à INTERVAL_MS', 1000], // INTERVAL_MS=2000 dans ce test
  ])('valeur aberrante (%s) ignorée : repli sur INTERVAL_MS × 3, jamais de boucle de reconnexion permanente', (_label, badValue) => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() })
    act(() => {
      ws.simulateMessage({
        ACTION: 'HEARTBEAT',
        MSG: { INTERVAL_MS: 2000, DEAD_LINK_TIMEOUT_MS: badValue },
      })
    })

    // Repli attendu : INTERVAL_MS × 3 = 6000ms — PAS 2000ms (qui produirait
    // une boucle de reconnexion permanente, la fenêtre de battement à 2000ms
    // dépassant alors systématiquement un seuil de 2000ms).
    act(() => { vi.advanceTimersByTime(5500) })
    expect(ws.readyState).toBe(MockWebSocket.OPEN)
    expect(result.current.status).toBe('connected')

    act(() => { vi.advanceTimersByTime(1000) }) // total 6500ms
    expect(ws.readyState).toBe(MockWebSocket.CLOSED)
  })

  it('une valeur invalide reçue APRÈS une valeur valide ne l\'écrase pas (pas de flap sur un message isolé aberrant)', () => {
    const { result } = renderHook(() => useWebSocket('/ws/test'))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() })
    act(() => { ws.simulateMessage(heartbeat({ intervalMs: 2000, deadLinkTimeoutMs: 4000 })) })
    act(() => { vi.advanceTimersByTime(1000) })
    // Message aberrant isolé — ne doit PAS réinitialiser le seuil valide déjà connu (4000ms).
    act(() => {
      ws.simulateMessage({ ACTION: 'HEARTBEAT', MSG: { INTERVAL_MS: 2000, DEAD_LINK_TIMEOUT_MS: -1 } })
    })

    // Le seuil reste 4000ms (dernier connu valide) : un silence de 3900ms
    // depuis ce dernier message reste sous le seuil.
    act(() => { vi.advanceTimersByTime(3900) }) // total 4900ms depuis l'ouverture, 3900ms depuis le dernier message
    expect(result.current.status).toBe('connected')
    expect(ws.readyState).toBe(MockWebSocket.OPEN)
  })
})
