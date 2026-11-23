import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, fireEvent, act } from '@testing-library/react'
import PictoCanvasInput, { buildFrames } from './PictoCanvasInput'
import { createPictoCanvasStore } from './pictoCanvasStore'

const pt = (n, base = 0) => Array.from({ length: n }, (_, i) => [base + i, i])

describe('buildFrames — trames ≤ 20 traits et ≤ 200 points (contrat §8.5)', () => {
  it('une petite file tient dans une trame', () => {
    expect(buildFrames([{ ID: 1, PTS: pt(3), END: true }])).toEqual([[{ ID: 1, PTS: pt(3), END: true }]])
  })
  it('250 points d\'un trait → 2 trames, END seulement sur la dernière', () => {
    const frames = buildFrames([{ ID: 1, PTS: pt(250), END: true }])
    expect(frames).toHaveLength(2)
    expect(frames[0][0].PTS).toHaveLength(200); expect(frames[0][0].END).toBe(false)
    expect(frames[1][0].PTS).toHaveLength(50); expect(frames[1][0].END).toBe(true)
  })
  it('25 traits d\'un point → 2 trames (20 + 5)', () => {
    const frames = buildFrames(Array.from({ length: 25 }, (_, i) => ({ ID: i + 1, PTS: [[i, i]], END: true })))
    expect(frames.map(f => f.length)).toEqual([20, 5])
  })
})

describe('PictoCanvasInput — capture', () => {
  beforeEach(() => { vi.useFakeTimers() })
  afterEach(() => { vi.useRealTimers() })
  const setup = (props = {}) => {
    const store = createPictoCanvasStore(); store.setEpoch(4)
    const send = vi.fn()
    const r = render(<PictoCanvasInput store={store} epoch={4} sendMessage={send} {...props} />)
    const el = r.getByTestId('picto-input')
    el.getBoundingClientRect = () => ({ left: 0, top: 0, width: 1000, height: 1000, right: 1000, bottom: 1000 })
    el.setPointerCapture = () => {}
    return { store, send, el }
  }
  const ev = (x, y) => ({ clientX: x, clientY: y, pointerId: 1, button: 0 })

  it('un trait est envoyé en une trame avec EPOCH et STROKES seulement (aucune identité), dessiné localement', () => {
    const { store, send, el } = setup()
    fireEvent.pointerDown(el, ev(100, 100))
    fireEvent.pointerMove(el, ev(105, 100)) // < 15 unités (150 px = 1500 unités ; 5 px = 50) : gardé
    fireEvent.pointerMove(el, ev(300, 300))
    fireEvent.pointerUp(el, ev(300, 300))
    act(() => { vi.advanceTimersByTime(100) })
    expect(send).toHaveBeenCalledTimes(1)
    const [action, payload] = send.mock.calls[0]
    expect(action).toBe('PICTO_STROKES')
    expect(Object.keys(payload).sort()).toEqual(['EPOCH', 'STROKES'])
    expect(payload.EPOCH).toBe(4)
    expect(payload.STROKES[0].ID).toBe(1)
    expect(payload.STROKES[0].END).toBe(true)
    expect(payload.STROKES[0].PTS[0]).toEqual([1000, 1000])
    expect(store.getStrokes()).toHaveLength(1)
  })
  it('au plus une trame toutes les 60 ms', () => {
    const { send, el } = setup()
    fireEvent.pointerDown(el, ev(10, 10))
    act(() => { vi.advanceTimersByTime(70) })
    fireEvent.pointerMove(el, ev(400, 400))
    fireEvent.pointerMove(el, ev(800, 800))
    expect(send).toHaveBeenCalledTimes(1)
    act(() => { vi.advanceTimersByTime(70) })
    expect(send).toHaveBeenCalledTimes(2)
  })
  it('verrouillée : aucun trait n\'est capté', () => {
    const { store, send, el } = setup({ locked: true })
    fireEvent.pointerDown(el, ev(100, 100)); fireEvent.pointerUp(el, ev(100, 100))
    act(() => { vi.advanceTimersByTime(100) })
    expect(send).not.toHaveBeenCalled()
    expect(store.getStrokes()).toHaveLength(0)
  })
  it('un simple appui envoie un point puis END (jamais une trame sans point)', () => {
    const { send, el } = setup()
    fireEvent.pointerDown(el, ev(500, 500)); fireEvent.pointerUp(el, ev(500, 500))
    act(() => { vi.advanceTimersByTime(100) })
    const strokes = send.mock.calls.flatMap(c => c[1].STROKES)
    expect(strokes.every(s => s.PTS.length >= 1)).toBe(true)
    expect(strokes[strokes.length - 1].END).toBe(true)
  })
})
