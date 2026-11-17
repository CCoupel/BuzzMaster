import { useEffect, useRef, useState } from 'react'
import { REP, angleFor, forwardDistance, brakePlan, brakePosition, landPosition, landTotal, offsetToIndex, spinPosition, spinRampPosition } from './geometry'
import './Wheel.css'

/**
 * Roue générique (contracts/wheel-component.md) : ne sait rien du jeu.
 * Pointeur fixe, contenu mobile (un seul `transform`). Position continue
 * TOUJOURS ramenée modulo n à la fin de chaque bascule (§4) : aucune dérive
 * après 100 mouvements.
 *
 * @param {Object} props
 * Un `label` peut contenir `\n` (libellé sur plusieurs lignes) ; `icon` (optionnel) s'affiche au-dessus.
 * @param {Array<{id:string,label:string,icon?:string,color?:string,kind?:'letter'|'value'|'special',state?:''|'disabled'|'played'}>} props.segments
 * @param {number|null} props.current - index du segment sous le pointeur, ou null
 * @param {'strip'|'arc'} [props.layout='strip']
 * @param {'tv'|'compact'} [props.size='tv']
 * @param {string} [props.accent] - couleur du pointeur / du halo
 * @param {Object|null} [props.motion] - null = bascule courte vers `current` ; `{mode:'spin', periodMs}` défilement continu ;
 *   `{mode:'spin', periodMs[, accelTurns, cruise]}` (avec `accelTurns` : départ à l'arrêt, montée à accélération constante, pleine vitesse atteinte après `accelTurns` tours ; `cruise` : croisière immédiate) ; `{mode:'land', to, durationMs, minTurns[, periodMs]}` freinage jusqu'à l'index `to` (avec `periodMs` : vitesse continue depuis la croisière du `spin`, sans `minTurns`) (v12.0.0, §5). Sans horloge serveur.
 * @param {number} [props.stepMs=600] - durée de la bascule courte
 */
const PALETTE = ['#e5e7eb', '#dbeafe', '#fef3c7', '#dcfce7', '#fce7f3', '#ede9fe']

const reducedMotion = () => {
  try { return !!window.matchMedia?.('(prefers-reduced-motion: reduce)').matches } catch { return false }
}

function ariaLabelFor(segments, current) {
  const seg = current != null ? segments[current] : null
  if (!seg) return 'Roue'
  const label = String(seg.label).replace(/\n/g, ' ')
  return seg.kind === 'letter' || !seg.kind ? `Lettre courante : ${label}` : `Segment : ${label}`
}

const labelLines = (seg) => String(seg.label ?? '').split('\n')
const frameNow = () => (typeof performance !== 'undefined' && performance.now ? performance.now() : Date.now())
const requestFrame = (f) => (typeof requestAnimationFrame === 'function' ? requestAnimationFrame(f) : setTimeout(() => f(frameNow()), 16))
const cancelFrame = (id) => (typeof cancelAnimationFrame === 'function' ? cancelAnimationFrame(id) : clearTimeout(id))

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
  motion = null,
  stepMs = 600,
}) {
  const n = segments.length
  const safeCurrent = current != null && current >= 0 && current < n ? current : null

  // pos : position affichée (continue). animate : transition CSS active pendant la bascule.
  const [view, setView] = useState({ pos: safeCurrent ?? 0, animate: false })
  const settledRef = useRef(safeCurrent ?? 0) // position normalisée AU REPOS (fin de la dernière bascule)
  const animatingRef = useRef(false)
  const pendingRef = useRef(null) // dernière cible reçue pendant une bascule (la plus récente gagne)
  const timerRef = useRef(null)
  const mountedRef = useRef(true)
  const latest = useRef({ n, stepMs })
  latest.current = { n, stepMs }
  const rootRef = useRef(null)
  const moverRef = useRef(null) // élément qui porte le transform (ruban ou rotor)
  const posRef = useRef(safeCurrent ?? 0) // position continue affichée, normalisée [0, n) — mise à jour à chaque image
  const firstRenderRef = useRef(true)
  const [landed, setLanded] = useState(null) // index mis en évidence à la fin d'un `land`
  const motionMode = motion?.mode === 'spin' || motion?.mode === 'land' ? motion.mode : null
  const motionTo = motion?.to
  const motionPeriod = motion?.periodMs
  const motionDuration = motion?.durationMs
  const motionTurns = motion?.minTurns
  const motionAccel = motion?.accelTurns
  const motionCruise = motion?.cruise === true
  const hasMotion = motionMode !== null
  const layoutKey = layout === 'arc' ? 'arc' : 'strip'

  // Applique une position continue directement sur le DOM (une seule propriété `transform`, aucune ré-allocation).
  const apply = useRef(null)
  apply.current = (pos) => {
    posRef.current = pos
    const el = moverRef.current
    const count = latest.current.n
    if (el) {
      if (layoutKey === 'arc') el.style.transform = `rotate(${-(pos / Math.max(count, 1)) * 360}deg)`
      else el.style.transform = `translateX(-${count > 0 ? ((2 * count + pos + 0.5) / (count * REP)) * 100 : 0}%)`
      el.style.transition = 'none'
    }
    rootRef.current?.setAttribute('data-pos', String(pos))
    rootRef.current?.setAttribute('data-animate', 'false')
  }

  // Une bascule à la fois : si `current` change pendant une bascule, la cible est mise en file et
  // enchaînée à la fin (aucune interruption ⇒ aucun saut visuel). La position est ramenée modulo n
  // à la fin de chaque bascule (§4) : aucune dérive après 100 mouvements.
  const start = useRef(null)
  start.current = (target) => {
    const { n: count, stepMs: ms } = latest.current
    const from = settledRef.current
    const delta = forwardDistance(from, target, count) // [0, n) : plus court chemin vers l'avant
    if (delta === 0 || reducedMotion()) {
      settledRef.current = target
      posRef.current = target
      setView({ pos: target, animate: false })
      return
    }
    animatingRef.current = true
    setView({ pos: from, animate: false })
    const raf = typeof requestAnimationFrame === 'function' ? requestAnimationFrame : (f) => setTimeout(f, 0)
    raf(() => { if (mountedRef.current && animatingRef.current) setView({ pos: from + delta, animate: true }) })
    timerRef.current = setTimeout(() => {
      if (!mountedRef.current) return
      settledRef.current = offsetToIndex(target, count)
      posRef.current = settledRef.current
      animatingRef.current = false
      setView({ pos: settledRef.current, animate: false })
      const next = pendingRef.current
      pendingRef.current = null
      if (next != null && next !== settledRef.current) start.current(next)
    }, ms + 30)
  }

  useEffect(() => {
    if (hasMotion || safeCurrent == null || n === 0) return
    if (animatingRef.current) pendingRef.current = safeCurrent
    else start.current(safeCurrent)
  }, [safeCurrent, n, hasMotion])

  // Mouvement long (v12.0.0) : boucle d'images, position ramenée modulo n à chaque image et à l'arrêt (§4).
  useEffect(() => {
    if (!hasMotion || n === 0) return undefined
    // La bascule courte éventuelle est abandonnée : on repart de la position réellement affichée.
    if (timerRef.current) clearTimeout(timerRef.current)
    animatingRef.current = false
    pendingRef.current = null
    const first = firstRenderRef.current
    const from = offsetToIndex(posRef.current, n)
    let raf = null
    let cancelled = false
    let accel = 0
    const finish = (to) => {
      const end = offsetToIndex(to, n)
      settledRef.current = end
      apply.current(end)
      setView({ pos: end, animate: false })
      setLanded(end)
    }
    setLanded(null)
    if (motionMode === 'land') {
      const to = Number.isInteger(motionTo) ? motionTo : 0
      if (reducedMotion() || first || !(motionDuration > 0)) {
        finish(to)
        return undefined
      }
      // Avec `periodMs` (ROUE) : freinage à vitesse continue depuis la croisière du `spin` ; sinon (RAFALE, tests) ancien profil.
      // `land` part TOUJOURS de la croisière (roue.md §5.1 : aucun arrêt avant la pleine vitesse) ; reçu pendant la montée locale
      // (écran en retard), il est traité comme `cruise: true` : passage à vc depuis la position courante, pas de saut de position.
      const plan = motionPeriod > 0 ? brakePlan(from, to, n, motionDuration, motionPeriod) : null
      const total = plan ? 0 : landTotal(from, to, n, motionTurns ?? 1)
      const t0 = frameNow()
      const tick = () => {
        if (cancelled) return
        const elapsed = frameNow() - t0
        const p = elapsed / motionDuration
        if (p >= 1) { finish(to); return }
        apply.current(plan ? brakePosition(plan, from, to, elapsed, n) : landPosition(from, to, total, p, n))
        raf = requestFrame(tick)
      }
      raf = requestFrame(tick)
    } else {
      if (reducedMotion()) return undefined // pas de défilement : la roue reste en place
      const t0 = frameNow()
      // Montage direct en `spin` (client connecté en cours de rotation) : pas de montée, croisière immédiate (roue.md §6).
      // `cruise` (FULL_SPEED du serveur) : passage immédiat à la croisière depuis la position courante, sans montée.
      accel = first || motionCruise ? 0 : motionAccel
      const tick = () => {
        if (cancelled) return
        const elapsed = frameNow() - t0
        // Avec `accelTurns` (ROUE) : la roue part de l'arrêt et accélère jusqu'à la croisière ; sinon défilement uniforme.
        apply.current(accel > 0 ? spinRampPosition(from, elapsed, motionPeriod, n, accel) : spinPosition(from, elapsed, motionPeriod, n))
        raf = requestFrame(tick)
      }
      raf = requestFrame(tick)
    }
    return () => {
      cancelled = true
      if (raf != null) cancelFrame(raf)
      settledRef.current = posRef.current // reprise sans saut (spin → land, ou retour à la bascule courte)
    }
  }, [hasMotion, motionMode, motionTo, motionPeriod, motionDuration, motionTurns, motionAccel, motionCruise, n, layoutKey]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => { firstRenderRef.current = false }, [])

  useEffect(() => {
    mountedRef.current = true
    return () => { mountedRef.current = false; if (timerRef.current) clearTimeout(timerRef.current) }
  }, [])

  // Segment mis en évidence : `spin` aucun, `land` le segment d'arrêt une fois arrivé, sinon `current`.
  const hl = motionMode === 'spin' ? null : motionMode === 'land' ? landed : safeCurrent
  const transition = view.animate ? `transform ${stepMs}ms cubic-bezier(.215,.61,.355,1)` : 'none'
  const cls = `wheel wheel-${layout === 'arc' ? 'arc' : 'strip'} wheel-size-${size}`
  const colorOf = (seg, i) => seg.color || PALETTE[i % PALETTE.length]
  const stateCls = (seg, i) =>
    [seg.state === 'disabled' ? 'disabled' : '', seg.state === 'played' ? 'played' : '',
      seg.kind === 'special' ? 'special' : '', i === hl ? 'is-current' : ''].filter(Boolean).join(' ')

  const common = { 'data-pos': view.pos, 'data-animate': view.animate ? 'true' : 'false', role: 'img', ref: rootRef, 'aria-label': ariaLabelFor(segments, hl), style: { '--wheel-accent': accent } }

  if (layout === 'arc') {
    const deg = -(view.pos / Math.max(n, 1)) * 360
    return (
      <div className={cls} {...common}>
        <svg className="wheel-svg" viewBox="0 0 1000 1000" preserveAspectRatio="xMidYMid meet" aria-hidden="true">
          <g
            ref={moverRef}
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
                  data-current={i === hl ? 'true' : undefined}
                >
                  <path d={wedgePath(i, n, 480, 330)} fill={colorOf(seg, i)} />
                  {seg.icon && (
                    <text className="wheel-icon" x={tx} y={ty} transform={`rotate(${a + 90} ${tx} ${ty}) translate(0 -38)`} textAnchor="middle" dominantBaseline="central" fontSize={Math.min(60, 1200 / Math.max(n, 1))}>
                      {seg.icon}
                    </text>
                  )}
                  <text x={tx} y={ty} transform={`rotate(${a + 90} ${tx} ${ty})`} textAnchor="middle" dominantBaseline="central" fontSize={labelLines(seg).length > 1 || seg.icon ? Math.min(40, 800 / Math.max(n, 1)) : Math.min(90, 1800 / Math.max(n, 1))}>
                    {labelLines(seg).length > 1
                      ? labelLines(seg).map((l, k) => <tspan key={k} x={tx} dy={k === 0 ? '-0.55em' : '1.1em'}>{l}</tspan>)
                      : seg.label}
                  </text>
                </g>
              )
            })}
          </g>
          <polygon className="wheel-pointer" points="470,4 530,4 500,64" fill={accent} />
        </svg>
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
        ref={moverRef}
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
              data-current={i === hl ? 'true' : undefined}
              style={{ background: colorOf(seg, i) }}
            >
              {seg.icon && <span className="wheel-icon">{seg.icon}</span>}
              <span className={`wheel-lab${labelLines(seg).length > 1 || seg.icon ? ' multi' : ''}`}>
                {labelLines(seg).map((l, k) => (k === 0 ? l : <span key={k}><br />{l}</span>))}
              </span>
            </div>
          )),
        )}
      </div>
      <div className="wheel-pointer" aria-hidden="true" />
    </div>
  )
}
