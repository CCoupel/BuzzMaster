/**
 * #193 (v13.0.0, LOT-2.1) — capture du dessinateur : lots (≤ 20 traits, ≤ 200 points par trame), cadence 60 ms,
 * simplification à 15 unités, trame sans identité, feuille verrouillée (contracts/picto.md §8.5, §8.6, maquette M4).
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, fireEvent, act } from '@testing-library/react'
import PictoCanvasInput, { buildFrames } from './PictoCanvasInput'
import { createPictoCanvasStore } from './pictoCanvasStore'

describe('buildFrames — découpe en trames', () => {
  const stroke = (id, n, end = true) => ({ ID: id, PTS: Array.from({ length: n }, (_, i) => [i, i]), END: end })

  it('une trame : 1 à 200 points au total', () => {
    const frames = buildFrames([stroke(1, 450, true)])
    expect(frames).toHaveLength(3)
    for (const f of frames) {
      const pts = f.reduce((n, s) => n + s.PTS.length, 0)
      expect(pts).toBeGreaterThanOrEqual(1)
      expect(pts).toBeLessThanOrEqual(200)
    }
    expect(frames.flat().reduce((n, s) => n + s.PTS.length, 0)).toBe(450) // aucun point perdu
  })

  it('un trait coupé garde son ID ; END n\'est posé que sur sa dernière partie', () => {
    const frames = buildFrames([stroke(7, 250, true)])
    const parts = frames.flat()
    expect(parts.every(s => s.ID === 7)).toBe(true)
    expect(parts.map(s => s.END)).toEqual([false, true])
  })

  it('au plus 20 traits par trame', () => {
    const frames = buildFrames(Array.from({ length: 45 }, (_, i) => stroke(i + 1, 1)))
    for (const f of frames) expect(f.length).toBeLessThanOrEqual(20)
    expect(frames.flat()).toHaveLength(45)
  })

  it('rien en attente : aucune trame', () => {
    expect(buildFrames([])).toEqual([])
  })
})

describe('PictoCanvasInput — capture au doigt', () => {
  let store
  let send
  beforeEach(() => {
    vi.useFakeTimers()
    store = createPictoCanvasStore()
    store.setEpoch(4)
    send = vi.fn()
    // La feuille fait 1000×1000 px à l'origine de la page.
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({ left: 0, top: 0, width: 1000, height: 1000, right: 1000, bottom: 1000 })
  })
  afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks() })

  const sheet = (utils) => utils.getByTestId('picto-input')
  const ptr = (type, x, y, id = 1) => ({ type, clientX: x, clientY: y, pointerId: id, button: 0 })
  const down = (el, x, y) => fireEvent.pointerDown(el, { clientX: x, clientY: y, pointerId: 1, button: 0 })
  const move = (el, x, y) => fireEvent.pointerMove(el, { clientX: x, clientY: y, pointerId: 1 })
  const up = (el, x, y) => fireEvent.pointerUp(el, { clientX: x, clientY: y, pointerId: 1 })

  it('un trait est envoyé en PICTO_STROKES {EPOCH, STROKES} sans identité, points normalisés', () => {
    const u = render(<PictoCanvasInput store={store} epoch={4} sendMessage={send} />)
    const el = sheet(u)
    down(el, 100, 100)
    move(el, 200, 200)
    up(el, 300, 300)
    act(() => { vi.advanceTimersByTime(200) })
    expect(send).toHaveBeenCalled()
    const [action, payload] = send.mock.calls[0]
    expect(action).toBe('PICTO_STROKES')
    expect(Object.keys(payload).sort()).toEqual(['EPOCH', 'STROKES']) // aucune identité de joueur (§8.6)
    expect(payload.EPOCH).toBe(4)
    expect(payload.STROKES[0].ID).toBeGreaterThanOrEqual(1)
    expect(payload.STROKES[0].PTS[0]).toEqual([1000, 1000]) // 100 px sur 1000 px = 1000 unités
  })

  it('au plus une trame toutes les 60 ms, les points sont regroupés', () => {
    const u = render(<PictoCanvasInput store={store} epoch={4} sendMessage={send} />)
    const el = sheet(u)
    down(el, 100, 100)
    for (let i = 1; i <= 20; i++) move(el, 100 + i * 40, 100)
    expect(send.mock.calls.length).toBeLessThanOrEqual(1) // rien ne part avant 60 ms (au plus le tout premier point)
    act(() => { vi.advanceTimersByTime(59) })
    const early = send.mock.calls.length
    act(() => { vi.advanceTimersByTime(200) })
    expect(send.mock.calls.length).toBeGreaterThan(early - 1)
    expect(send.mock.calls.length).toBeLessThanOrEqual(3)
    const pts = send.mock.calls.flatMap(([, p]) => p.STROKES.flatMap(s => s.PTS))
    expect(pts.length).toBeGreaterThanOrEqual(20)
  })

  it('simplification : un point à moins de 15 unités du précédent n\'est pas gardé', () => {
    const u = render(<PictoCanvasInput store={store} epoch={4} sendMessage={send} />)
    const el = sheet(u)
    down(el, 500, 500)
    move(el, 500.5, 500) // 5 unités : écarté
    move(el, 501, 500) // 10 unités : écarté
    move(el, 520, 500) // 200 unités : gardé
    act(() => { vi.advanceTimersByTime(200) })
    const pts = send.mock.calls.flatMap(([, p]) => p.STROKES.flatMap(s => s.PTS))
    expect(pts).toEqual([[5000, 5000], [5200, 5000]])
  })

  it('un trait envoyé a toujours au moins un point, même un simple appui (point seul)', () => {
    const u = render(<PictoCanvasInput store={store} epoch={4} sendMessage={send} />)
    const el = sheet(u)
    down(el, 400, 400)
    up(el, 400, 400)
    act(() => { vi.advanceTimersByTime(200) })
    for (const [, p] of send.mock.calls) for (const s of p.STROKES) expect(s.PTS.length).toBeGreaterThanOrEqual(1)
    const last = send.mock.calls.flatMap(([, p]) => p.STROKES).pop()
    expect(last.END).toBe(true)
  })

  it('feuille verrouillée (pause, arrêt, révélation) : aucun trait capté, rien envoyé', () => {
    const u = render(<PictoCanvasInput store={store} epoch={4} locked lockLabel="Jeu en pause" sendMessage={send} />)
    const el = sheet(u)
    down(el, 100, 100)
    move(el, 300, 300)
    up(el, 300, 300)
    act(() => { vi.advanceTimersByTime(300) })
    expect(send).not.toHaveBeenCalled()
    expect(store.getStrokes()).toEqual([])
    expect(u.getByText('Jeu en pause')).toBeTruthy()
  })

  it('la numérotation des traits est croissante et continue après une resynchronisation (maxStrokeId)', () => {
    store.applyCanvas({ EPOCH: 4, SEQ: 1, STROKES: [{ ID: 12, PTS: [[1, 1]], END: true }] })
    const u = render(<PictoCanvasInput store={store} epoch={4} sendMessage={send} />)
    const el = sheet(u)
    down(el, 100, 100)
    up(el, 200, 200)
    act(() => { vi.advanceTimersByTime(200) })
    const ids = send.mock.calls.flatMap(([, p]) => p.STROKES.map(s => s.ID))
    expect(Math.min(...ids)).toBeGreaterThan(12)
  })
})
