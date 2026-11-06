import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #285 (LOT-4 rév. 2) — CATEGORIES_CHANGED : exposé sous `categoriesPush`, plus d'évènement window sur QUESTIONS.
// Contrat : contracts/websocket-actions.md §CATEGORIES_CHANGED.

let wsInstances = []
class MockWebSocket {
  constructor(url) { this.url = url; this.readyState = 0; this.sentMessages = []; wsInstances.push(this) }
  send(d) { this.sentMessages.push(d) }
  close() { if (this.readyState === 3) return; this.readyState = 3; this.onclose && this.onclose() }
  simulateOpen() { this.readyState = 1; this.onopen && this.onopen() }
  simulateMessage(d) { this.onmessage && this.onmessage({ data: JSON.stringify(d) }) }
}
MockWebSocket.CONNECTING = 0; MockWebSocket.OPEN = 1; MockWebSocket.CLOSING = 2; MockWebSocket.CLOSED = 3

describe('useWebSocket — CATEGORIES_CHANGED (#285)', () => {
  beforeEach(() => { wsInstances = []; vi.useFakeTimers(); vi.stubGlobal('WebSocket', MockWebSocket) })
  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

  function open(endpoint = '/ws/tv') {
    const hook = renderHook(() => useWebSocket(endpoint))
    const ws = wsInstances[0]
    act(() => { ws.simulateOpen() })
    return { hook, ws }
  }

  it('expose { categories, change, ts } après CATEGORIES_CHANGED', () => {
    const { hook, ws } = open()
    expect(hook.result.current.categoriesPush).toBeNull()
    const categories = [{ key: 'GEOGRAPHY', name: 'Géo', imageURL: '', color: '#112233', isCustom: false }]
    act(() => {
      ws.simulateMessage({ ACTION: 'CATEGORIES_CHANGED', MSG: { CATEGORIES: categories, CHANGE: { TYPE: 'renamed', KEY: 'GEOGRAPHY' }, TS: 1762425548123 } })
    })
    expect(hook.result.current.categoriesPush).toEqual({ categories, change: { TYPE: 'renamed', KEY: 'GEOGRAPHY' }, ts: 1762425548123 })
  })

  it('message sans liste CATEGORIES : ignoré', () => {
    const { hook, ws } = open()
    act(() => { ws.simulateMessage({ ACTION: 'CATEGORIES_CHANGED', MSG: {} }) })
    expect(hook.result.current.categoriesPush).toBeNull()
  })

  it('QUESTIONS n\'émet plus d\'évènement window « buzz:questions-update »', () => {
    const { ws } = open('/ws/admin')
    const seen = vi.fn()
    window.addEventListener('buzz:questions-update', seen)
    act(() => { ws.simulateMessage({ ACTION: 'QUESTIONS', MSG: { q1: { ID: 'q1' } } }) })
    window.removeEventListener('buzz:questions-update', seen)
    expect(seen).not.toHaveBeenCalled()
  })
})
