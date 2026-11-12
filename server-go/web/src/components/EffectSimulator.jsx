import { useEffect, useMemo, useRef, useState } from 'react'
import { BulbRow, chronoRemaining, stepLabel } from './LightEffectPreview'

// #210 (v11.8.0, LOT-2, F2.3) — simulateur multi-salles pas à pas (maquette effect_editor_r4 « Simulateur », contrat §11.3, §11.8).
// Une rangée par taille de salle (1, 3, 6, 12 groupes + zones réelles de taille différente), TOUTES jouées sur la même horloge :
// le navigateur ne fait que REJOUER les trames reçues de `draft` (`previews[].frames[]` : `tick`, `step`, `groups`, `vars`) — il ne
// calcule aucune trame. Pas à pas : lecture / pause, tick précédent / suivant, retour au début, réglette ; un clic sur l'en-tête
// d'un pas de la grille (prop `seek`) place le simulateur sur le premier tick où ce pas est joué. À l'arrêt, tick, pas joué et
// `vars` restent affichés.
//
// Props :
//   previews    aperçus du dernier brouillon valide (un par taille demandée, `groups` = la taille)
//   rows        [{ n, label, real }] rangées à afficher, dans l'ordre
//   minGroups   groupes demandés par l'effet (« sous le minimum : non joué en salle »)
//   tickMs      durée d'un tick de l'effet (lecture ×1, plancher 250 ms comme le lecteur)
//   moments, moment, onMoment   moments déclarés et moment simulé
//   seek        { column, nonce } : place la lecture sur le premier tick du pas `column` ('pre' | 'last' | indice) ; met en pause
//   onColumn(column | null)     colonne jouée dans la rangée de référence (surlignée dans la grille)
//   stale       le brouillon courant est en erreur : le simulateur joue le dernier brouillon valide

const MOMENT_LABEL = { chrono: 'Chronomètre', calm: 'Phases calmes', score: 'Équipe qui marque' }
export const MIN_PLAY_MS = 250

// Colonne de la grille correspondant au pas d'une trame (§7.3 : « pre » | « pas:<n> » | « dernier »).
export function columnOfStep(step) {
  if (step === 'pre') return 'pre'
  if (step === 'dernier') return 'last'
  const m = /^pas:(\d+)$/.exec(step || '')
  return m ? Number(m[1]) : null
}
const stepOfColumn = (c) => (c === 'pre' ? 'pre' : c === 'last' ? 'dernier' : `pas:${c}`)

// Trame d'une rangée au tick t : la trame de ce tick, sinon la dernière avant (tenue) ; `skipped` quand le tick n'existe pas
// pour cette rangée alors que l'horloge est encore dans l'aperçu (pas sauté dans cette salle).
export function frameAtTick(preview, t) {
  const frames = preview?.frames
  if (!Array.isArray(frames) || frames.length === 0) return { frame: null, skipped: false }
  let held = null
  for (const f of frames) {
    if (f.tick === t) return { frame: f, skipped: false }
    if (f.tick < t) held = f
    else break
  }
  return { frame: held, skipped: held !== null && t <= frames[frames.length - 1].tick }
}

export default function EffectSimulator({
  previews = [], rows = [], minGroups = 0, tickMs = 1000, moments = [], moment = '', onMoment, seek = null, onColumn, stale = false,
}) {
  const [t, setT] = useState(0)
  const [playing, setPlaying] = useState(false)
  const [hidden, setHidden] = useState(() => new Set())

  const byGroups = useMemo(() => new Map(previews.map(p => [p.groups, p])), [previews])
  const maxTick = useMemo(() => previews.reduce((m, p) => Math.max(m, p.frames?.length ? p.frames[p.frames.length - 1].tick : 0), 0), [previews])
  const loop = previews.length ? previews[0].loop !== false : true
  const ref = useMemo(() => rows.find(r => r.n === 6 && byGroups.has(6)) || rows.find(r => byGroups.has(r.n)) || null, [rows, byGroups])
  const refPreview = ref ? byGroups.get(ref.n) : null
  const refFrame = frameAtTick(refPreview, Math.min(t, maxTick)).frame

  // Le tick reste dans l'aperçu quand celui-ci change (brouillon modifié).
  useEffect(() => { setT(x => Math.min(x, maxTick)) }, [maxTick])

  const timer = useRef(null)
  useEffect(() => {
    if (!playing) return undefined
    timer.current = setInterval(() => {
      setT(prev => {
        if (prev >= maxTick) return loop ? 0 : prev
        return prev + 1
      })
    }, Math.max(MIN_PLAY_MS, tickMs))
    return () => clearInterval(timer.current)
  }, [playing, maxTick, loop, tickMs])
  // Sans boucle : la lecture s'arrête sur le dernier tick.
  useEffect(() => { if (playing && !loop && t >= maxTick) setPlaying(false) }, [t, playing, loop, maxTick])

  // Clic sur l'en-tête d'un pas : premier tick où ce pas est joué dans la rangée de référence.
  useEffect(() => {
    if (!seek || !refPreview) return
    const want = stepOfColumn(seek.column)
    const f = refPreview.frames.find(x => x.step === want)
    if (f) { setPlaying(false); setT(f.tick) }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [seek?.nonce])

  const col = refFrame ? columnOfStep(refFrame.step) : null
  useEffect(() => { onColumn?.(col) }, [col]) // eslint-disable-line react-hooks/exhaustive-deps

  const go = (v) => { setPlaying(false); setT(Math.max(0, Math.min(maxTick, v))) }
  const toggleHidden = (n) => setHidden(h => { const x = new Set(h); if (x.has(n)) x.delete(n); else x.add(n); return x })
  const rem = refFrame ? chronoRemaining(refPreview, refFrame.at_ms) : null

  return (
    <section className="efs" aria-label="Simulateur" data-testid="effect-simulator">
      <h3 className="efx-h3">Simulateur</h3>
      <div className="efx-head">
        <span className="efs-transport">
          <button type="button" className="efx-btn" title="Revenir au début" aria-label="Revenir au début" onClick={() => go(0)}>⏮</button>
          <button type="button" className="efx-btn" title="Tick précédent" aria-label="Tick précédent" onClick={() => go(t - 1)} disabled={t <= 0}>◀ tick</button>
          <button type="button" className="efx-btn efx-btn-primary" title="Lecture / pause" aria-pressed={playing} onClick={() => setPlaying(p => !p)} disabled={maxTick <= 0 && !previews.length}>
            {playing ? '⏸ Pause' : '▶ Lecture'}
          </button>
          <button type="button" className="efx-btn" title="Tick suivant" aria-label="Tick suivant" onClick={() => go(t + 1)} disabled={t >= maxTick}>tick ▶</button>
          <input
            type="range" min="0" max={Math.max(0, maxTick)} value={Math.min(t, maxTick)} aria-label="Position dans l'aperçu (tick)"
            title="Position dans l'aperçu (tick)" onChange={(e) => go(Number(e.target.value))}
          />
        </span>
        <span className="efx-hint efs-moment">Moment simulé :</span>
        {moments.length > 1 ? (
          <span className="efx-chips" role="group" aria-label="Moment simulé">
            {moments.map(m => (
              <button
                key={m} type="button" className={`efx-chip${m === moment ? ' is-on' : ''}`} aria-pressed={m === moment}
                onClick={() => onMoment?.(m)}
              >{MOMENT_LABEL[m] || m}</button>
            ))}
          </span>
        ) : <span className="efs-moment-one">{MOMENT_LABEL[moments[0]] || moments[0] || '—'}</span>}
      </div>
      {stale && <p className="efx-warn" role="status">Le brouillon contient une erreur : le simulateur joue le dernier brouillon valide.</p>}

      <div className="efs-stage" data-testid="simulator-stage">
        {rows.map(r => {
          const p = byGroups.get(r.n)
          const { frame, skipped } = frameAtTick(p, Math.min(t, maxTick))
          const vars = frame?.vars ? Object.entries(frame.vars) : []
          return (
            <div key={`${r.n}-${r.label}`} className={`efs-row row${hidden.has(r.n) ? ' is-off off' : ''}`} data-testid={`sim-row-${r.n}`}>
              <label className={`efs-lbl${r.real ? ' is-real' : ''}`}>
                <input type="checkbox" checked={!hidden.has(r.n)} onChange={() => toggleHidden(r.n)} aria-label={`Afficher la rangée ${r.label}`} />
                {r.label}
                {r.n < minGroups && <small>sous le minimum : non joué en salle</small>}
                {skipped && <small className="efs-skip">pas sauté</small>}
              </label>
              <div className="efs-bulbs-wrap"><BulbRow groups={r.n} frame={frame} className="efs" /></div>
              <div className="efs-vv" data-testid={`sim-vars-${r.n}`}>
                {vars.map(([name, v]) => `${name} = ${v > 0 ? `g${v}` : '—'}`).join(' · ')}
              </div>
            </div>
          )
        })}
        {refFrame ? (
          <div className="efs-clock" data-testid="simulator-clock" role="status" aria-live="off">
            {`tick ${Math.min(t, maxTick)} · ${refFrame.step === 'dernier' ? 'DERNIER' : (stepLabel(refFrame.step) || '—')}${rem !== null ? ` · chronomètre simulé : ${rem} s` : ''}`}
          </div>
        ) : (
          <div className="efs-clock" role="status">Aucun aperçu : corrigez les erreurs de l'effet pour le voir jouer.</div>
        )}
      </div>
      <p className="efx-hint">
        Les rangées sont jouées <b>en même temps, sur la même horloge</b> : quatre salles types (1, 3, 6 et 12 groupes) et une rangée par zone réelle dont le
        nombre de groupes est différent. Décochez une rangée pour la masquer. À droite : le groupe que désigne chaque variable à ce tick. Le pas joué est
        surligné dans la grille. <b>Pas à pas</b> : « ◀ tick » et « tick ▶ » mettent l'aperçu en pause ; la réglette va directement à un tick ; un clic sur
        l'en-tête d'un pas de la grille place l'aperçu sur le premier tick où ce pas est joué. Vitesse d'origine (×1) ; l'aperçu suit le brouillon, sans enregistrer.
      </p>
    </section>
  )
}
