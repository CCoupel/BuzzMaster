/**
 * PictoCanvasInput — feuille de dessin de la tablette du dessinateur (contrat picto.md §8.5, maquette M4).
 *
 * Capture `pointer` (doigt) sur une feuille carrée. Aucun outil : trait noir, jamais d'effacement. Le trait est
 * dessiné localement tout de suite (le store local), puis envoyé en trames `PICTO_STROKES` au plus toutes les
 * 60 ms, 200 points au plus, 20 traits au plus par trame ; un point n'est gardé que s'il est à ≥ 15 unités du
 * précédent. `PICTO_STROKES` ne porte ni identité ni autre champ que `EPOCH` et `STROKES` (garde serveur §8.6).
 * Verrouillée (`locked`) en pause, à l'arrêt et en révélation : aucun trait n'est capté.
 */
import { useCallback, useEffect, useRef } from 'react'
import PictoCanvasView from './PictoCanvasView'
import {
  PICTO_FRAME_MS, PICTO_MAX_POINTS_PER_FRAME, PICTO_MAX_STROKES_PER_FRAME,
  keepPoint, normalizePoint,
} from './pictoGeometry'
import './PictoCanvas.css'

/** Découpe les points en attente en trames ≤ 20 traits et ≤ 200 points (pur, testable). */
export function buildFrames(pending) {
  const frames = []
  let cur = []
  let pts = 0
  for (const s of pending) {
    let rest = s.PTS
    let first = true
    while (rest.length || first) {
      if (cur.length >= PICTO_MAX_STROKES_PER_FRAME || pts >= PICTO_MAX_POINTS_PER_FRAME) { frames.push(cur); cur = []; pts = 0 }
      const room = PICTO_MAX_POINTS_PER_FRAME - pts
      const take = rest.slice(0, room)
      rest = rest.slice(take.length)
      cur.push({ ID: s.ID, PTS: take, END: rest.length === 0 ? s.END : false })
      pts += take.length
      first = false
      if (!rest.length) break
    }
  }
  if (cur.length) frames.push(cur)
  return frames
}

export default function PictoCanvasInput({ store, epoch, locked = false, sendMessage, lockLabel = '', className = '' }) {
  const wrapRef = useRef(null)
  const draw = useRef({ id: 0, last: null, active: false, pointerId: null })
  const pending = useRef([]) // [{ID, PTS, END}] en attente d'envoi
  const timer = useRef(0)
  const lastSent = useRef(0)

  const flush = useCallback(() => {
    timer.current = 0
    if (!pending.current.length) return
    const batch = pending.current
    pending.current = []
    for (const strokes of buildFrames(batch)) {
      sendMessage?.('PICTO_STROKES', { EPOCH: epoch, STROKES: strokes })
    }
    lastSent.current = Date.now()
  }, [epoch, sendMessage])

  const schedule = useCallback(() => {
    if (timer.current) return
    const wait = Math.max(0, PICTO_FRAME_MS - (Date.now() - lastSent.current))
    timer.current = setTimeout(flush, wait)
  }, [flush])

  // Nouvelle feuille (autre époque) : numérotation des traits et file d'envoi repartent à zéro.
  useEffect(() => {
    draw.current = { id: 0, last: null, active: false, pointerId: null }
    pending.current = []
    return () => { if (timer.current) { clearTimeout(timer.current); timer.current = 0 } }
  }, [epoch])

  const point = (e) => {
    const r = wrapRef.current.getBoundingClientRect()
    return normalizePoint(e.clientX - r.left, e.clientY - r.top, r.width, r.height)
  }
  const add = (id, p, end) => {
    store.addLocal(id, [p], end)
    const last = pending.current[pending.current.length - 1]
    if (last && last.ID === id) { last.PTS.push(p); last.END = end } else pending.current.push({ ID: id, PTS: [p], END: end })
    schedule()
  }

  const onDown = (e) => {
    if (locked || !store) return
    if (e.button !== undefined && e.button > 0) return
    try { wrapRef.current.setPointerCapture?.(e.pointerId) } catch { /* sans capture : le trait suit quand même */ }
    const id = Math.max(draw.current.id, store.maxStrokeId()) + 1
    const p = point(e)
    draw.current = { id, last: p, active: true, pointerId: e.pointerId }
    add(id, p, false)
  }
  const onMove = (e) => {
    const d = draw.current
    if (!d.active || locked || e.pointerId !== d.pointerId) return
    const p = point(e)
    if (!keepPoint(d.last, p)) return
    d.last = p
    add(d.id, p, false)
  }
  const onUp = (e) => {
    const d = draw.current
    if (!d.active || e.pointerId !== d.pointerId) return
    d.active = false
    // Fin du trait : le dernier point est gardé même s'il est proche (le doigt s'arrête là). Un trait ne part jamais
    // sans point (une trame a 1 à 200 points) : sans point nouveau, le dernier est renvoyé avec END = true.
    const p = point(e)
    add(d.id, keepPoint(d.last, p) ? p : d.last, true)
  }

  return (
    <div
      ref={wrapRef}
      className={`picto-sheet picto-sheet-fit ${className}`.trim()}
      data-testid="picto-input"
      style={{ touchAction: 'none' }}
      onPointerDown={onDown}
      onPointerMove={onMove}
      onPointerUp={onUp}
      onPointerCancel={onUp}
    >
      <PictoCanvasView store={store} />
      {locked && lockLabel && <div className="picto-sheet-lock">{lockLabel}</div>}
    </div>
  )
}
