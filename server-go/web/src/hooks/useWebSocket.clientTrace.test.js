import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import useWebSocket from './useWebSocket'

// #246 (F-LOG) — CLIENT_TRACE émis par le hook : WS_GAP (durée + envois
// abandonnés), VISIBILITY, PAGESHOW, ARDOISE_CLEARED_SEEN (admin/anim).

let instances = []
class MockWebSocket {
  constructor(url) {
    this.url = url
    this.readyState = 0
    this.sent = []
    instances.push(this)
  }
  send(m) { this.sent.push(JSON.parse(m)) }
  close() { this.readyState = 3; this.onclose && this.onclose() }
}
MockWebSocket.OPEN = 1
MockWebSocket.CONNECTING = 0
MockWebSocket.CLOSED = 3

const open = (ws) => { ws.readyState = 1; ws.onopen() }
const traces = (ws) => ws.sent.filter((m) => m.ACTION === 'CLIENT_TRACE').map((m) => m.MSG)
const update = (ws, GAME) => ws.onmessage({ data: JSON.stringify({ ACTION: 'UPDATE', MSG: { GAME } }) })

describe('useWebSocket — CLIENT_TRACE (#246)', () => {
  beforeEach(() => {
    instances = []
    vi.useFakeTimers()
    vi.stubGlobal('WebSocket', MockWebSocket)
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('WS_GAP : durée de coupure et envois abandonnés, émis à la réouverture', async () => {
    const { result } = renderHook(() => useWebSocket('/ws/player'))
    open(instances[0])
    await act(async () => { instances[0].readyState = 3; instances[0].onclose() })
    await act(async () => { vi.advanceTimersByTime(500) })
    act(() => {
      result.current.sendMessage('ARDOISE_INPUT', { TEXT: 'A' })
      result.current.sendMessage('ARDOISE_INPUT', { TEXT: 'AB' })
      result.current.sendClientTrace('VISIBILITY') // une trace n'est pas un envoi abandonné
    })
    await act(async () => { vi.advanceTimersByTime(8000) })
    const ws = instances.at(-1)
    open(ws)
    const gap = traces(ws).find((t) => t.KIND === 'WS_GAP')
    expect(gap).toBeTruthy()
    expect(gap.DETAIL.DROPPED_SENDS).toBe(2)
    expect(gap.DETAIL.DOWN_MS).toBeGreaterThanOrEqual(500)
    expect(typeof gap.AT).toBe('number')
  })

  it('VISIBILITY et PAGESHOW émis', () => {
    renderHook(() => useWebSocket('/ws/player'))
    const ws = instances[0]
    open(ws)
    Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true })
    document.dispatchEvent(new Event('visibilitychange'))
    Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true })
    document.dispatchEvent(new Event('visibilitychange'))
    const pe = new Event('pageshow'); pe.persisted = true
    window.dispatchEvent(pe)
    const t = traces(ws)
    expect(t.find((x) => x.KIND === 'VISIBILITY' && x.DETAIL.STATE === 'hidden')).toBeTruthy()
    expect(t.find((x) => x.KIND === 'VISIBILITY' && x.DETAIL.STATE === 'visible')).toBeTruthy()
    expect(t.find((x) => x.KIND === 'PAGESHOW').DETAIL.PERSISTED).toBe(true)
  })

  it('ARDOISE_CLEARED_SEEN : texte non vide → vide pour la même question (admin)', () => {
    renderHook(() => useWebSocket('/ws/admin'))
    const ws = instances[0]
    open(ws)
    const q = { ID: 'q1', TYPE: 'ARDOISE' }
    act(() => update(ws, { PHASE: 'STARTED', QUESTION: q, ARDOISE_ANSWERS: { Verts: { TEXT: 'PARIS' } } }))
    expect(traces(ws).filter((t) => t.KIND === 'ARDOISE_CLEARED_SEEN')).toHaveLength(0)
    act(() => update(ws, { PHASE: 'STARTED', QUESTION: q, ARDOISE_ANSWERS: { Verts: { TEXT: '' } } }))
    const seen = traces(ws).filter((t) => t.KIND === 'ARDOISE_CLEARED_SEEN')
    expect(seen).toHaveLength(1)
    expect(seen[0]).toMatchObject({ TEAM: 'Verts', PREV_LEN: 5, CAUSE_ACTION: 'UPDATE' })
  })

  it('ARDOISE_CLEARED_SEEN : pas de trace pour une remise à zéro légitime (nouvelle question, PREPARE) ni hors admin/anim', () => {
    renderHook(() => useWebSocket('/ws/admin'))
    const ws = instances[0]
    open(ws)
    act(() => update(ws, { PHASE: 'STARTED', QUESTION: { ID: 'q1' }, ARDOISE_ANSWERS: { Verts: { TEXT: 'PARIS' } } }))
    act(() => update(ws, { PHASE: 'PREPARE', QUESTION: { ID: 'q1' }, ARDOISE_ANSWERS: { Verts: { TEXT: '' } } }))
    act(() => update(ws, { PHASE: 'STARTED', QUESTION: { ID: 'q1' }, ARDOISE_ANSWERS: { Verts: { TEXT: 'AB' } } }))
    act(() => update(ws, { PHASE: 'STARTED', QUESTION: { ID: 'q2' }, ARDOISE_ANSWERS: { Verts: { TEXT: '' } } }))
    expect(traces(ws).filter((t) => t.KIND === 'ARDOISE_CLEARED_SEEN')).toHaveLength(0)

    renderHook(() => useWebSocket('/ws/tv'))
    const tv = instances.at(-1)
    open(tv)
    act(() => update(tv, { PHASE: 'STOPPED', QUESTION: { ID: 'q1' }, ARDOISE_ANSWERS: { Verts: { TEXT: 'PARIS' } } }))
    act(() => update(tv, { PHASE: 'STOPPED', QUESTION: { ID: 'q1' }, ARDOISE_ANSWERS: { Verts: { TEXT: '' } } }))
    expect(traces(tv)).toHaveLength(0)
  })

  it.each([['freeze', 'FREEZE'], ['resume', 'RESUME']])('événement Page Lifecycle « %s » → CLIENT_TRACE %s, écouteur retiré au démontage', (evt, kind) => {
    const { unmount } = renderHook(() => useWebSocket('/ws/anim'))
    const ws = instances[0]
    open(ws)
    act(() => { document.dispatchEvent(new Event(evt)) })
    const hits = traces(ws).filter((t) => t.KIND === kind)
    expect(hits).toHaveLength(1)
    expect(typeof hits[0].AT).toBe('number')
    unmount()
    document.dispatchEvent(new Event(evt))
    expect(traces(ws).filter((t) => t.KIND === kind)).toHaveLength(1)
  })
})
