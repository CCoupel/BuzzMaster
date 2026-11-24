/**
 * PictoCanvasView — rend la feuille PICTO (trait noir sur fond blanc, carrée) depuis un `pictoCanvasStore`
 * (contrat picto.md §8.2). Aucun état React pour les traits : le store notifie, le rendu est regroupé sur
 * `requestAnimationFrame`. Segments droits entre points ; un trait d'un seul point est un disque du diamètre du trait.
 */
import { useEffect, useRef } from 'react'
import { denormalizeCoord, lineWidthPx } from './pictoGeometry'
import './PictoCanvas.css'

export function drawStrokes(ctx, strokes, size) {
  ctx.clearRect(0, 0, size, size)
  ctx.fillStyle = '#fff'
  ctx.fillRect(0, 0, size, size)
  const w = lineWidthPx(size)
  ctx.strokeStyle = '#000'
  ctx.fillStyle = '#000'
  ctx.lineWidth = w
  ctx.lineCap = 'round'
  ctx.lineJoin = 'round'
  for (const s of strokes) {
    const pts = s.PTS
    if (!pts || !pts.length) continue
    if (pts.length === 1) {
      ctx.beginPath()
      ctx.arc(denormalizeCoord(pts[0][0], size), denormalizeCoord(pts[0][1], size), w / 2, 0, Math.PI * 2)
      ctx.fill()
      continue
    }
    ctx.beginPath()
    ctx.moveTo(denormalizeCoord(pts[0][0], size), denormalizeCoord(pts[0][1], size))
    for (let i = 1; i < pts.length; i += 1) ctx.lineTo(denormalizeCoord(pts[i][0], size), denormalizeCoord(pts[i][1], size))
    ctx.stroke()
  }
}

export default function PictoCanvasView({ store, className = '' }) {
  const ref = useRef(null)
  useEffect(() => {
    const canvas = ref.current
    if (!canvas || !store) return undefined
    let raf = 0
    const paint = () => {
      raf = 0
      const rect = canvas.getBoundingClientRect()
      const size = Math.max(1, Math.round(Math.min(rect.width, rect.height) || rect.width || 1))
      const dpr = (typeof window !== 'undefined' && window.devicePixelRatio) || 1
      const px = Math.round(size * dpr)
      if (canvas.width !== px || canvas.height !== px) { canvas.width = px; canvas.height = px }
      const ctx = canvas.getContext && canvas.getContext('2d')
      if (!ctx) return
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
      drawStrokes(ctx, store.getStrokes(), size)
    }
    const schedule = () => { if (!raf) raf = requestAnimationFrame(paint) }
    const unsub = store.subscribe(schedule)
    let ro
    if (typeof ResizeObserver !== 'undefined') { ro = new ResizeObserver(schedule); ro.observe(canvas) }
    schedule()
    return () => { unsub(); if (ro) ro.disconnect(); if (raf) cancelAnimationFrame(raf) }
  }, [store])
  return <canvas ref={ref} className={`picto-canvas ${className}`.trim()} data-testid="picto-canvas" />
}
