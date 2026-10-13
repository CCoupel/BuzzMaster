import { useEffect, useRef } from 'react'
import './TvPointsAward.css'
import { FLIGHT } from '../hooks/usePointsAward'

// #265 — rendu de l'animation d'attribution de points (maquettes tv_points_award__multi_team et
// vplayer_points_award__own_team) : fusée à trajectoire courbe (départ et arrivée aléatoires), étincelles,
// carte « équipe + +X pts » avec zoom. Couche fixe, overflow hidden, pointer-events none (vue TV statique :
// aucun scroll). transform/opacity seulement. Le calendrier (cartes, étapes) vient de usePointsAward.

const SAMPLES = 14

function bezier(c, e) {
  const x = (1 - e) ** 2 * c.start.x + 2 * (1 - e) * e * c.ctrl.x + e * e * c.end.x
  const y = (1 - e) ** 2 * c.start.y + 2 * (1 - e) * e * c.ctrl.y + e * e * c.end.y
  return { x, y }
}

function Rocket({ card, durationMs }) {
  const ref = useRef(null)
  // Dépendances PRIMITIVES (jamais l'objet carte, recréé à chaque rendu) : l'animation ne repart pas
  // quand le parent se re-rend (tick du chrono, autre carte qui change d'étape...).
  const { key, color } = card
  const sx = card.start.x, sy = card.start.y, cx = card.ctrl.x, cy = card.ctrl.y, ex = card.end.x, ey = card.end.y
  useEffect(() => {
    const el = ref.current
    if (!el || typeof el.animate !== 'function') return undefined
    const frames = Array.from({ length: SAMPLES + 1 }, (_, i) => {
      const k = i / SAMPLES
      const { x, y } = bezier({ start: { x: sx, y: sy }, ctrl: { x: cx, y: cy }, end: { x: ex, y: ey } }, 1 - (1 - k) ** 2)
      return { transform: `translate(${x}vw, ${y}vh)`, offset: k }
    })
    const anim = el.animate(frames, { duration: durationMs * FLIGHT, easing: 'linear', fill: 'forwards' })
    return () => anim.cancel()
  }, [key, sx, sy, cx, cy, ex, ey, durationMs])
  return <div ref={ref} className="tv-award-rocket" style={{ '--c': color }} aria-hidden="true" />
}

function Sparks({ card }) {
  const ref = useRef(null)
  const { key, color, own } = card
  const ex = card.end.x, ey = card.end.y
  useEffect(() => {
    const kids = ref.current ? [...ref.current.children] : []
    if (!kids.length || typeof kids[0].animate !== 'function') return undefined
    const anims = kids.map((el, i) => {
      const a = (i / kids.length) * Math.PI * 2
      const d = own ? 18 + Math.random() * 12 : 6 + Math.random() * 6
      const tx = Math.max(-ex + 1, Math.min(99 - ex, Math.cos(a) * d * 0.56))
      const ty = Math.max(-ey + 1, Math.min(99 - ey, Math.sin(a) * d + 4))
      return el.animate(
        [{ transform: 'translate(0vw, 0vh)', opacity: 1 }, { transform: `translate(${tx}vw, ${ty}vh)`, opacity: 0 }],
        { duration: 900, easing: 'cubic-bezier(.1,.7,.3,1)', fill: 'forwards' },
      )
    })
    return () => anims.forEach((a) => a.cancel())
  }, [key, own, ex, ey])
  const cols = [color, '#ffffff', '#ffd700']
  return (
    <div ref={ref} className="tv-award-sparks" style={{ left: `${ex}%`, top: `${ey}%` }} aria-hidden="true">
      {Array.from({ length: 18 }, (_, i) => <span key={i} className="tv-award-spark" style={{ '--c': cols[i % 3] }} />)}
    </div>
  )
}

/** @param {{lots: Array}} props lots issus de usePointsAward */
export default function TvPointsAward({ lots }) {
  if (!lots || lots.length === 0) return null
  const multi = lots.find((l) => !l.own && l.count > 1)
  return (
    <div className="tv-award-layer" aria-hidden={lots.some((l) => l.own) ? undefined : 'true'} data-testid="points-award-layer">
      {lots.flatMap((lot) => lot.cards.map((c) => {
        const card = { ...c, own: lot.own }
        if (c.stage === 'wait') return null
        if (c.stage === 'flying') return <Rocket key={c.key} card={card} durationMs={lot.durationMs} />
        const motion = c.reduced ? 'calm' : 'zoom'
        return (
          <div key={c.key}>
            {!c.reduced && <Sparks card={card} />}
            <div
              className={`tv-award-card ${lot.own ? 'own' : ''} ${c.stage === 'out' ? 'out' : motion}`}
              style={{ '--c': c.color, '--zoom': `${Math.round(lot.durationMs * 0.1)}ms`, '--fade': `${Math.round(lot.durationMs * 0.14)}ms`, left: `${c.end.x}%`, top: `${c.end.y}%` }}
              title={c.team}
              role={lot.own ? 'status' : undefined}
              aria-live={lot.own ? 'polite' : undefined}
            >
              <span className="tv-award-team">{c.team}</span>
              <span className="tv-award-pts">+{c.points} pts</span>
              {lot.own && <span className="tv-award-sub">pour votre équipe</span>}
            </div>
          </div>
        )
      }))}
      {multi && <div className="tv-award-lot-tag">Lot {multi.index + 1}/{multi.count}</div>}
    </div>
  )
}
