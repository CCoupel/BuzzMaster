import { useEffect, useRef, useState } from 'react'
import { REP, angleFor, forwardDistance, offsetToIndex } from './geometry'
import './Wheel.css'

/**
 * Roue générique (contracts/wheel-component.md) : ne sait rien du jeu.
 * Pointeur fixe, contenu mobile (un seul `transform`). Position continue
 * TOUJOURS ramenée modulo n à la fin de chaque bascule (§4) : aucune dérive
 * après 100 mouvements.
 *
 * @param {Object} props
 * @param {Array<{id:string,label:string,color?:string,kind?:'letter'|'value'|'special',state?:''|'disabled'|'played'}>} props.segments
 * @param {number|null} props.current - index du segment sous le pointeur, ou null
 * @param {'strip'|'arc'} [props.layout='strip']
 * @param {'tv'|'compact'} [props.size='tv']
 * @param {string} [props.accent] - couleur du pointeur / du halo
 * @param {Object|null} [props.motion] - réservé v12.0 : accepté et IGNORÉ en v11.4.0 (bascule courte)
 * @param {number} [props.stepMs=600] - durée de la bascule courte
 */
const PALETTE = ['#e5e7eb', '#dbeafe', '#fef3c7', '#dcfce7', '#fce7f3', '#ede9fe']

const reducedMotion = () => {
  try { return !!window.matchMedia?.('(prefers-reduced-motion: reduce)').matches } catch { return false }
}

function ariaLabelFor(segments, current) {
  const seg = current != null ? segments[current] : null
  if (!seg) return 'Roue'
  return seg.kind === 'letter' || !seg.kind ? `Lettre courante : ${seg.label}` : `Segment : ${seg.label}`
}

function wedgePath(i, n, R, r) {
  const half = Math.PI / n
  const c = (angleFor(i, n) * Math.PI) / 180
  const a0 = c - half
  const a1 = c + half
  const p = (rad, a) => `${(500 + rad * Math.cos(a)).toFixed(2)} ${(500 + rad * Math.sin(a)).toFixed(2)}`
  const large = n === 1 ? 1 : 0
  return `M ${p(R, a0)} A ${R} ${R} 0 ${large} 1 ${p(R, a1)} L ${p(r, a1)} A ${r} ${r} 0 ${large} 0 ${p(r, a0)} Z`
}

export default function Wheel({
  segments = [],
  current = null,
  layout = 'strip',
  size = 'tv',
  accent = '#3b82f6',
  // eslint-disable-next-line no-unused-vars
  motion = null,
  stepMs = 600,
}) {
  const n = segments.length
  const safeCurrent = current != null && current >= 0 && current < n ? current : null

  // pos : position affichée (continue). animate : transition CSS active pendant la bascule.
  const [view, setView] = useState({ pos: safeCurrent ?? 0, animate: false })
  const posRef = useRef(safeCurrent ?? 0) // position normalisée au repos
  const timerRef = useRef(null)

  useEffect(() => {
    if (safeCurrent == null || n === 0) return undefined
    const from = posRef.current
    const delta = forwardDistance(from, safeCurrent, n) // [0, n) : plus court chemin vers l'avant
    posRef.current = safeCurrent
    if (timerRef.current) clearTimeout(timerRef.current)
    if (delta === 0 || reducedMotion()) {
      setView({ pos: safeCurrent, animate: false })
      return undefined
    }
    // Départ depuis la position normalisée précédente (jamais une valeur cumulée), arrivée = from + delta.
    setView({ pos: from, animate: false })
    const raf = typeof requestAnimationFrame === 'function' ? requestAnimationFrame : (f) => setTimeout(f, 0)
    raf(() => setView({ pos: from + delta, animate: true }))
    // Normalisation modulo n à la fin du mouvement (§4) : repli sans transition.
    timerRef.current = setTimeout(() => {
      setView({ pos: offsetToIndex(safeCurrent, n), animate: false })
    }, stepMs + 30)
    return undefined
  }, [safeCurrent, n, stepMs])

  useEffect(() => () => { if (timerRef.current) clearTimeout(timerRef.current) }, [])

  const transition = view.animate ? `transform ${stepMs}ms cubic-bezier(.215,.61,.355,1)` : 'none'
  const cls = `wheel wheel-${layout === 'arc' ? 'arc' : 'strip'} wheel-size-${size}`
  const colorOf = (seg, i) => seg.color || PALETTE[i % PALETTE.length]
  const stateCls = (seg, i) =>
    [seg.state === 'disabled' ? 'disabled' : '', seg.state === 'played' ? 'played' : '',
      seg.kind === 'special' ? 'special' : '', i === safeCurrent ? 'is-current' : ''].filter(Boolean).join(' ')

  const common = { role: 'img', 'aria-label': ariaLabelFor(segments, safeCurrent), style: { '--wheel-accent': accent } }

  if (layout === 'arc') {
    const step = n > 0 ? 360 / n : 0
    const deg = -(view.pos / Math.max(n, 1)) * 360
    return (
      <div className={cls} {...common}>
        <svg className="wheel-svg" viewBox="0 0 1000 1000" preserveAspectRatio="xMidYMid meet" aria-hidden="true">
          <g
            className="rotor wheel-rotor"
            style={{ transform: `rotate(${deg}deg)`, transformOrigin: '500px 500px', transition, willChange: 'transform' }}
          >
            {segments.map((seg, i) => {
              const a = angleFor(i, n)
              const rad = (a * Math.PI) / 180
              const tx = 500 + 405 * Math.cos(rad)
              const ty = 500 + 405 * Math.sin(rad)
              return (
                <g
                  key={seg.id ?? i}
                  className={`wheel-sector ${stateCls(seg, i)}`}
                  data-state={seg.state || undefined}
                  data-kind={seg.kind || undefined}
                  data-current={i === safeCurrent ? 'true' : undefined}
                >
                  <path d={wedgePath(i, n, 480, 330)} fill={colorOf(seg, i)} />
                  <text x={tx} y={ty} transform={`rotate(${a + 90} ${tx} ${ty})`} textAnchor="middle" dominantBaseline="central" fontSize={Math.min(90, 1800 / Math.max(n, 1))}>
                    {seg.label}
                  </text>
                </g>
              )
            })}
          </g>
          <polygon className="wheel-pointer" points="470,4 530,4 500,64" fill={accent} />
        </svg>
        <span hidden>{step}</span>
      </div>
    )
  }

  // strip : ruban de n cellules répété REP fois, translateX en % de la largeur du ruban (aucun px).
  const base = 2 * n
  const total = n * REP
  const x = total > 0 ? ((base + view.pos + 0.5) / total) * 100 : 0
  return (
    <div className={cls} {...common}>
      <div
        className="wheel-track"
        style={{ '--wheel-cells': total, transform: `translateX(-${x}%)`, transition, willChange: 'transform' }}
      >
        {Array.from({ length: REP }, (_, rep) =>
          segments.map((seg, i) => (
            <div
              key={`${rep}-${seg.id ?? i}`}
              className={`wheel-cell ${stateCls(seg, i)}`}
              data-state={seg.state || undefined}
              data-kind={seg.kind || undefined}
              data-current={i === safeCurrent ? 'true' : undefined}
              style={{ background: colorOf(seg, i) }}
            >
              <span>{seg.label}</span>
            </div>
          )),
        )}
      </div>
      <div className="wheel-pointer" aria-hidden="true" />
    </div>
  )
}
