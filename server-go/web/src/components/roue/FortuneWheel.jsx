/**
 * FortuneWheel — adaptateur ROUE → Wheel (v12.0.0, #144, contrat wheel-component.md §8, maquette M1).
 *
 * Traduit `GameState.ROUE_WHEEL` en props de `Wheel` ; ne sait rien du moteur de jeu. La barre de durée
 * maximale est décomptée LOCALEMENT depuis la réception de `SPINNING` (aucune horloge serveur).
 *
 * @param {Object} props.wheel - ROUE_WHEEL { LAYOUT, SPIN_PERIOD_MS, SPIN_MAX_MS, SEGMENTS, CURRENT, STOP }
 * @param {string} props.subphase - ROUE_SUBPHASE
 * @param {number[]|string} [props.teamColor] - ROUE_CURRENT_TEAM_COLOR ([r,g,b]) ou couleur CSS
 */
import { useEffect, useMemo, useRef, useState } from 'react'
import Wheel from '../wheel/Wheel'
import './FortuneWheel.css'

const PALETTE = ['#fca5a5', '#93c5fd', '#86efac', '#fde68a', '#d8b4fe', '#fdba74']
const SPECIAL_COLOR = '#0f172a'

export function buildFortuneSegments(rawSegments = []) {
  let k = 0
  return rawSegments.map((s, i) => {
    const id = s.ID ?? `s${i}`
    if (s.KIND === 'bankrupt') return { id, kind: 'special', label: 'BANQUE\nROUTE', icon: '💸', color: SPECIAL_COLOR }
    if (s.KIND === 'lose_turn') return { id, kind: 'special', label: 'TOUR\nPERDU', icon: '⏭', color: SPECIAL_COLOR }
    return { id, kind: 'value', label: String(s.VALUE), color: PALETTE[k++ % PALETTE.length] }
  })
}

export function fortuneMotion(wheel, subphase) {
  if (subphase === 'SPINNING') return { mode: 'spin', periodMs: wheel?.SPIN_PERIOD_MS || 2500 }
  if (subphase === 'STOPPING' && wheel?.STOP) {
    return { mode: 'land', to: wheel.STOP.TO, durationMs: wheel.STOP.DURATION_MS, minTurns: 1, periodMs: wheel?.SPIN_PERIOD_MS || 2500 }
  }
  return null
}

export default function FortuneWheel({ wheel, subphase = '', teamColor, size = 'tv' }) {
  const rawSegments = wheel?.SEGMENTS
  const segments = useMemo(() => buildFortuneSegments(rawSegments || []), [rawSegments])
  const accent = Array.isArray(teamColor) && teamColor.length === 3
    ? `rgb(${teamColor.join(',')})`
    : (typeof teamColor === 'string' && teamColor ? teamColor : undefined)
  const motionKey = subphase === 'SPINNING' ? `spin:${wheel?.SPIN_PERIOD_MS}`
    : subphase === 'STOPPING' && wheel?.STOP ? `land:${wheel.STOP.TO}:${wheel.STOP.DURATION_MS}:${wheel?.SPIN_PERIOD_MS}` : ''
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const motion = useMemo(() => fortuneMotion(wheel, subphase), [motionKey])
  const current = wheel && wheel.CURRENT >= 0 ? wheel.CURRENT : null

  // Barre de durée maximale : décomptée localement depuis l'entrée en SPINNING.
  const [elapsed, setElapsed] = useState(0)
  const spinning = subphase === 'SPINNING'
  const maxMs = wheel?.SPIN_MAX_MS || 0
  const t0 = useRef(0)
  useEffect(() => {
    if (!spinning || !(maxMs > 0)) { setElapsed(0); return undefined }
    t0.current = Date.now()
    setElapsed(0)
    const id = setInterval(() => setElapsed(Math.min(maxMs, Date.now() - t0.current)), 100)
    return () => clearInterval(id)
  }, [spinning, maxMs])

  if (!wheel || segments.length === 0) return null
  const layout = wheel.LAYOUT === 'arc' ? 'arc' : 'strip'
  const remaining = Math.max(0, 1 - elapsed / maxMs)
  return (
    <div className={`fortune-wheel fortune-wheel-${layout}`} data-testid="fortune-wheel">
      {spinning && maxMs > 0 && (
        <div className="fortune-wheel-timer" role="progressbar" aria-label="Durée maximale de rotation"
          aria-valuemin={0} aria-valuemax={maxMs} aria-valuenow={Math.round(maxMs - elapsed)}>
          <div className="fortune-wheel-timer-fill" style={{ width: `${remaining * 100}%` }} />
        </div>
      )}
      <Wheel segments={segments} current={current} layout={layout} size={size} accent={accent} motion={motion} />
    </div>
  )
}
