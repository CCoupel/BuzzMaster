/**
 * WheelConfigCard — carte « 🎡 Roue de la Fortune » de la page Paramètres (v12.0.0, #144, maquette M4,
 * contrat roue.md §10 / §11.3).
 *
 * La roue est GÉNÉRÉE PAR LE SERVEUR à partir de 4 paramètres : l'aperçu vient de
 * `POST /api/roue/wheel-preview` — aucune copie de la règle de construction ni de la correction du pas ici.
 * Les durées sont saisies en secondes, enregistrées en millisecondes (`flip_interval_ms` n'est pas exposé :
 * conservé tel quel).
 *
 * @param {Object} props.wheel - section `wheel` effective de GET /game-config.json
 * @param {Object} props.wheelDefault - `wheel_default` (bouton « Roue par défaut »)
 * @param {Function} [props.onSaved] - appelée avec la section `wheel` retenue après enregistrement
 */
import { useCallback, useEffect, useRef, useState } from 'react'
import Button from '../Button'
import FortuneWheel, { buildFortuneSegments } from './FortuneWheel'
import './WheelConfigCard.css'

// Deux groupes (maquette retenue B, #309) : Points / Cases spéciales ; « 🎁 BONUS » = nombre global de cases BONUS (0 à 4, défaut 0).
const PARAMS = [
  { key: 'max_points', label: 'Points maximum', hint: 'valeur de la meilleure case', min: 2, group: 'Points' },
  { key: 'step', label: 'Pas', hint: 'écart entre deux valeurs ; aussi la plus petite valeur', min: 1, group: 'Points' },
  { key: 'bankrupt_count', label: '💸 Banqueroute', hint: 'cases qui vident la cagnotte', min: 0, group: 'Cases spéciales' },
  { key: 'lose_turn_count', label: '⏭ Perdu', hint: 'cases qui font passer le tour', min: 0, group: 'Cases spéciales' },
  { key: 'bonus_count', label: '🎁 BONUS', hint: "0 à 4 (moins si la roue est petite) — marque posée sur une case à points ; cadeau géré par l'animateur, aucun point", min: 0, max: 4, group: 'Cases spéciales', isNew: true },
]
const GROUPS = ['Points', 'Cases spéciales']
const DURATIONS = [
  { key: 'spin_period_ms', label: "Durée d'un tour de roue", hint: 'secondes (1 à 6) — vitesse de défilement', step: 0.5 },
  { key: 'spin_max_ms', label: 'Durée maximale de rotation', hint: "secondes (3 à 30) — sans buzz de l'équipe, la roue s'arrête d'elle-même", step: 1 },
  { key: 'stop_duration_ms', label: 'Durée du freinage', hint: "secondes (1,5 à 8) — entre l'arrêt demandé et l'arrêt sur la case", step: 0.5 },
]
const PREVIEW_DELAY_MS = 300
const PALETTE = ['#fca5a5', '#93c5fd', '#86efac', '#fde68a', '#d8b4fe', '#fdba74']

const num = (v) => (v === '' || v == null ? NaN : Number(v))
const fmt = (n) => String(n).replace('.', ',')

function fromServer(w) {
  const x = w || {}
  return {
    max_points: x.max_points, step: x.step, bankrupt_count: x.bankrupt_count, lose_turn_count: x.lose_turn_count,
    bonus_count: x.bonus_count ?? 0, // absent (config v12.0 / v12.1 LOT-1) = 0 BONUS
    bonus_mode: x.bonus_mode === 'PERMANENT' ? 'PERMANENT' : 'EPHEMERAL', // absent = éphémère (§17.5)
    spin_period_ms: x.spin_period_ms, spin_max_ms: x.spin_max_ms, stop_duration_ms: x.stop_duration_ms,
    flip_interval_ms: x.flip_interval_ms,
  }
}

/**
 * Position (segments) du bandeau de l'essai, lue à chaque image : l'anneau tourne EXACTEMENT comme lui (même position, même
 * instant). Ne calcule rien de la roue : il copie `data-pos` du composant `Wheel`.
 */
function useFollowWheel(sourceRef, active, restIndex, n) {
  const gRef = useRef(null)
  const rot = (idx) => (n > 0 ? -(idx * 360) / n : 0)
  useEffect(() => {
    const g = gRef.current
    if (!g) return undefined
    if (!active) { g.setAttribute('transform', `rotate(${rot(restIndex)} 500 500)`); return undefined }
    let raf = null
    const tick = () => {
      const el = sourceRef.current?.querySelector('[role="img"]')
      const p = Number(el?.getAttribute('data-pos'))
      if (Number.isFinite(p)) g.setAttribute('transform', `rotate(${rot(p)} 500 500)`)
      raf = requestAnimationFrame(tick)
    }
    raf = requestAnimationFrame(tick)
    return () => { if (raf != null) cancelAnimationFrame(raf) }
  }, [active, restIndex, n]) // eslint-disable-line react-hooks/exhaustive-deps
  return gRef
}

function Ring({ segments, sourceRef, active = false, restIndex = 0 }) {
  // Vue de réglage : dessin des cases telles que renvoyées par le serveur (aucune règle ici). L'essai fait tourner le groupe des
  // cases ; le pointeur et les textes du centre restent fixes.
  const n = segments.length
  const gRef = useFollowWheel(sourceRef, active, restIndex, n)
  const a = 360 / n
  const R = 480
  const r = 300
  const pt = (rad, deg) => { const t = (deg * Math.PI) / 180; return [(500 + rad * Math.cos(t)).toFixed(1), (500 + rad * Math.sin(t)).toFixed(1)] }
  return (
    <svg viewBox="0 0 1000 1000" className="wheel-cfg-ring" role="img" aria-label={`Roue de ${n} cases`}>
      <g ref={gRef} data-testid="wheel-cfg-ring-rotor">
      {segments.map((s, i) => {
        const mid = -90 + i * a
        const [x0, y0] = pt(R, mid - a / 2)
        const [x1, y1] = pt(R, mid + a / 2)
        const [x2, y2] = pt(r, mid + a / 2)
        const [x3, y3] = pt(r, mid - a / 2)
        const [lx, ly] = pt(390, mid)
        const fs = Math.min(46, 1000 / n)
        return (
          <g key={s.id}>
            <path d={`M${x0} ${y0} A${R} ${R} 0 0 1 ${x1} ${y1} L${x2} ${y2} A${r} ${r} 0 0 0 ${x3} ${y3} Z`} fill={s.mark === 'bonus' ? '#fbbf24' : (s.color || PALETTE[0])} stroke="#0b1020" strokeWidth="3" />
            {s.mark === 'bonus' && <line x1={pt(r, mid)[0]} y1={pt(r, mid)[1]} x2={pt(R, mid)[0]} y2={pt(R, mid)[1]} stroke="#b45309" strokeWidth={Math.max(8, 300 / n)} />}
            <text x={lx} y={ly} transform={`rotate(${mid + 90} ${lx} ${ly})`} textAnchor="middle" dominantBaseline="central" fontSize={fs} fontWeight="900" fill={s.kind === 'special' ? '#fff' : '#111827'}>
              {s.mark === 'bonus' ? '🎁' : (s.icon || s.label)}
              {s.mark === 'bonus' && <title>{`${s.label}, BONUS`}</title>}
            </text>
          </g>
        )
      })}
      </g>
      <circle cx="500" cy="500" r="290" fill="#f9fafb" />
      <path d="M500 46 L478 6 L522 6 Z" fill="#ef4444" />
      <text x="500" y="490" textAnchor="middle" fontSize="34" fontWeight="800" fill="#374151">{n} cases</text>
      <text x="500" y="535" textAnchor="middle" fontSize="24" fill="#6b7280">case 1 en haut, lecture ↻</text>
    </svg>
  )
}

function trialLabel(st, segments) {
  switch (st.phase) {
    case 'up': return 'Montée…'
    case 'full': return `Pleine vitesse — arrêt automatique dans ${Math.ceil(st.remainingMs / 1000)} s`
    case 'brake': return 'Freinage'
    case 'stopped': { const seg = segments[st.rest]; return seg ? `Arrêt sur ${seg.kind === 'special' ? seg.label.replace('\n', ' ') : seg.label}${seg.mark === 'bonus' ? ' + 🎁' : ''}` : '' }
    default: return ''
  }
}

/**
 * Essai de la roue (roue.md §10.2) : états repos → montée → pleine vitesse → freinage → arrêt, SANS effet sur le jeu (aucun
 * message WebSocket). Les durées viennent de `TIMING` (aperçu serveur), jamais d'un calcul de la page ; le tirage de la case est
 * local (uniforme, sans valeur de jeu). Interrompu (`interrupt`) par tout changement de réglage et au démontage.
 */
function useWheelTrial(timing, count) {
  const [st, setSt] = useState({ phase: 'idle', stop: null, rest: 0, remainingMs: 0 })
  const timers = useRef([])
  const clear = () => { timers.current.forEach(t => { clearTimeout(t); clearInterval(t) }); timers.current = [] }
  const interrupt = useCallback(() => { clear(); setSt({ phase: 'idle', stop: null, rest: 0, remainingMs: 0 }) }, [])
  useEffect(() => () => clear(), [])
  const brake = useCallback((to, durMs) => {
    clear()
    setSt({ phase: 'brake', stop: { TO: to, DURATION_MS: durMs }, rest: to, remainingMs: 0 })
    timers.current.push(setTimeout(() => setSt({ phase: 'stopped', stop: null, rest: to, remainingMs: 0 }), durMs + 50))
  }, [])
  const start = useCallback(() => {
    if (!timing || !(count > 0)) return
    clear()
    const drawAndBrake = () => brake(Math.floor(Math.random() * count), timing.STOP_DURATION_MS)
    setSt(s => ({ phase: 'up', stop: null, rest: s.rest, remainingMs: 0 }))
    timers.current.push(setTimeout(() => {
      const t0 = Date.now()
      setSt(s => ({ ...s, phase: 'full', remainingMs: timing.SPIN_MAX_MS }))
      timers.current.push(setInterval(() => setSt(s => (s.phase === 'full' ? { ...s, remainingMs: Math.max(0, timing.SPIN_MAX_MS - (Date.now() - t0)) } : s)), 200))
      timers.current.push(setTimeout(drawAndBrake, timing.SPIN_MAX_MS)) // arrêt automatique à la durée maximale
    }, timing.SPIN_UP_MS))
  }, [timing, count, brake])
  const stop = useCallback(() => { if (timing && count > 0) brake(Math.floor(Math.random() * count), timing.STOP_DURATION_MS) }, [timing, count, brake])
  return { st, start, stop, interrupt }
}

export default function WheelConfigCard({ wheel, wheelDefault, onSaved }) {
  const [form, setForm] = useState(() => fromServer(wheel))
  const [preview, setPreview] = useState(null) // { COUNT, step, SEGMENTS, step_adjusted_from }
  const [error, setError] = useState('')
  const [adjusted, setAdjusted] = useState(null) // { from, to }
  const [saving, setSaving] = useState(false)
  const [toast, setToast] = useState(null)
  const seq = useRef(0)
  const tvRef = useRef(null)
  const trial = useWheelTrial(preview?.TIMING || null, preview?.SEGMENTS?.length || 0)

  // Les 4 paramètres qui déterminent la roue (les durées ne changent pas son dessin).
  // Les trois durées partent aussi (facultatives) : le serveur renvoie `TIMING` (bornes appliquées, montée) pour l'essai (§10.2, §11.3).
  const { max_points: mp, step: st, bankrupt_count: bc, lose_turn_count: lc, bonus_count: bn, spin_period_ms: dp, spin_max_ms: dm, stop_duration_ms: ds } = form
  useEffect(() => {
    const body = { max_points: num(mp), step: num(st), bankrupt_count: num(bc), lose_turn_count: num(lc), bonus_count: Number.isFinite(num(bn)) ? num(bn) : 0 }
    if (Number.isFinite(num(dp))) body.spin_period_ms = num(dp)
    if (Number.isFinite(num(dm))) body.spin_max_ms = num(dm)
    if (Number.isFinite(num(ds))) body.stop_duration_ms = num(ds)
    if (![body.max_points, body.step, body.bankrupt_count, body.lose_turn_count].every(Number.isFinite)) {
      setPreview(null)
      setError('les réglages sont des nombres entiers')
      return undefined
    }
    const id = ++seq.current
    const ctrl = typeof AbortController !== 'undefined' ? new AbortController() : null
    const timer = setTimeout(async () => {
      try {
        const res = await fetch('/api/roue/wheel-preview', {
          method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body), signal: ctrl?.signal,
        })
        if (id !== seq.current) return
        if (res.ok) {
          const data = await res.json()
          if (id !== seq.current) return
          setPreview(data)
          setError('')
          const kept = data.step
          if (Number.isFinite(kept) && kept !== body.step) {
            setAdjusted({ from: data.step_adjusted_from ?? body.step, to: kept })
            setForm(f => ({ ...f, step: kept })) // le champ prend la valeur retenue ; nouvel aperçu sans correction
          }
        } else {
          let data = null
          try { data = await res.json() } catch { /* corps non JSON */ }
          setPreview(null)
          setError(data?.MESSAGE || data?.CODE || `refusé par le serveur (HTTP ${res.status})`)
        }
      } catch (e) {
        if (e?.name === 'AbortError' || id !== seq.current) return
        setPreview(null)
        setError('aperçu indisponible (serveur injoignable)')
      }
    }, PREVIEW_DELAY_MS)
    return () => { clearTimeout(timer); ctrl?.abort() }
  }, [mp, st, bc, lc, bn, dp, dm, ds])

  useEffect(() => { trial.interrupt() }, [preview]) // eslint-disable-line react-hooks/exhaustive-deps

  const edit = (key, value) => {
    trial.interrupt()
    setAdjusted(null)
    setToast(null)
    setForm(f => ({ ...f, [key]: value === '' ? '' : Number(value) }))
  }
  const editSeconds = (key, value) => {
    trial.interrupt()
    setToast(null)
    setForm(f => ({ ...f, [key]: value === '' ? '' : Math.round(Number(value) * 1000) }))
  }
  const resetDefault = () => {
    if (!wheelDefault) return
    trial.interrupt()
    setAdjusted(null)
    setToast(null)
    setForm(fromServer(wheelDefault))
  }

  const save = async () => {
    setSaving(true)
    setToast(null)
    try {
      const res = await fetch('/game-config.json', {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ wheel: form }),
      })
      if (res.ok) {
        let data = null
        try { data = await res.json() } catch { /* corps vide */ }
        const kept = data?.wheel ? fromServer(data.wheel) : form
        setForm(kept)
        if (data?.step_adjusted_from != null && kept.step !== data.step_adjusted_from) setAdjusted({ from: data.step_adjusted_from, to: kept.step })
        setToast({ type: 'success', message: 'Roue enregistrée.' })
        onSaved?.(kept)
      } else {
        const text = await res.text().catch(() => '')
        // Corps texte « ROUE_BONUS_COUNT_INVALID: Trop de BONUS : N au plus… » : sans le préfixe technique (§17.3).
        const msg = text.replace(/^ROUE_[A-Z_]+:\s*/, '').trim()
        setToast({ type: 'error', message: msg || `Échec de l'enregistrement (HTTP ${res.status}).` })
      }
    } catch (e) {
      setToast({ type: 'error', message: e.message })
    } finally {
      setSaving(false)
    }
  }

  const segments = preview ? buildFortuneSegments(preview.SEGMENTS || []) : []
  const timing = preview?.TIMING || null
  // Essai : même adaptateur que le jeu (`FortuneWheel`), alimenté par un ROUE_WHEEL SIMULÉ (jamais envoyé nulle part).
  const trialSub = trial.st.phase === 'up' || trial.st.phase === 'full' ? 'SPINNING' : trial.st.phase === 'brake' ? 'STOPPING' : ''
  const trialActive = trial.st.phase !== 'idle' && trial.st.phase !== 'stopped'
  const trialWheel = {
    LAYOUT: 'strip', SEGMENTS: preview?.SEGMENTS, CURRENT: trial.st.rest, STOP: trial.st.stop,
    SPIN_PERIOD_MS: timing?.SPIN_PERIOD_MS, SPIN_UP_MS: timing?.SPIN_UP_MS, SPIN_MAX_MS: timing?.SPIN_MAX_MS, STOP_DURATION_MS: timing?.STOP_DURATION_MS,
    FULL_SPEED: trial.st.phase === 'full',
  }
  const values = segments.filter(s => s.kind === 'value').map(s => Number(s.label))
  const special = segments.length - values.length
  const total = values.reduce((a, b) => a + b, 0)
  const defLabel = wheelDefault ? ` (${wheelDefault.max_points} · ${wheelDefault.step} · ${wheelDefault.bankrupt_count} · ${wheelDefault.lose_turn_count} · ${wheelDefault.bonus_count ?? 0})` : ''

  return (
    <div className="config-section wheel-cfg" data-testid="wheel-config-card">
      <h3 className="config-section-title">🎡 Roue de la Fortune</h3>
      <p className="config-section-hint">
        <b>Réglages par défaut des questions ROUE</b> ; chaque question peut en personnaliser un ou plusieurs (éditeur de question). La forme de la roue (bandeau ou arc) se choisit sur chaque question.
      </p>

      <div className="wheel-cfg-groups">
        {GROUPS.map(g => (
          <fieldset key={g} className="wheel-cfg-group" data-testid={`wheel-group-${g}`}>
            <legend>{g}</legend>
            <div className={`wheel-cfg-params wheel-cfg-params-${g === 'Points' ? 2 : 3}`}>
              {PARAMS.filter(p => p.group === g).map(p => (
                <div key={p.key} className={`wheel-cfg-param${p.isNew ? ' new' : ''}`}>
                  <label htmlFor={`wheel-${p.key}`}>{p.label}</label>
                  <input id={`wheel-${p.key}`} type="number" min={p.min} max={p.max} step="1" value={form[p.key] ?? ''} onChange={(e) => edit(p.key, e.target.value)} />
                  <small>{p.hint}</small>
                  {p.key === 'bonus_count' && (
                    <label className={`wheel-cfg-switch${(form.bonus_count ?? 0) === 0 ? ' off' : ''}`} htmlFor="wheel-bonus_mode" title="Éphémère : la marque disparaît après son premier tirage, jusqu'à la fin de la manche">
                      <input id="wheel-bonus_mode" type="checkbox" role="switch" checked={form.bonus_mode !== 'PERMANENT'} disabled={(form.bonus_count ?? 0) === 0}
                        onChange={(e) => { trial.interrupt(); setToast(null); setForm(f => ({ ...f, bonus_mode: e.target.checked ? 'EPHEMERAL' : 'PERMANENT' })) }} />
                      <span>une seule fois par manche</span>
                    </label>
                  )}
                </div>
              ))}
            </div>
          </fieldset>
        ))}
      </div>

      {adjusted && (
        <div className="wheel-cfg-fix" role="status">
          ℹ Pas ajusté à <b>{adjusted.to}</b> : le pas doit diviser les points maximum ; {adjusted.from} ne le divise pas, {adjusted.to} est le diviseur le plus proche.
        </div>
      )}
      {error && <div className="wheel-cfg-err" role="alert">✗ Réglage refusé : {error}. Rien n'est enregistré.</div>}

      {preview && !error && (
        <>
          <div className="wheel-cfg-sum">
            <b>{segments.length} cases</b> : {values.length} gagnantes de <b>{Math.min(...values)}</b> à <b>{Math.max(...values)}</b> par pas de {preview.step} ·{' '}
            <b>{form.bankrupt_count}</b> Banqueroute · <b>{form.lose_turn_count}</b> Perdu · <b>{form.bonus_count ?? 0}</b> BONUS —{' '}
            {special ? `une chance sur ${fmt(Number((segments.length / special).toFixed(1)))} de tomber sur une case spéciale` : 'aucune case spéciale'}
            {' '}· gain moyen d'une case : {Math.round(total / segments.length)}.
          </div>
          <div className="wheel-cfg-trial" data-testid="wheel-trial">
            <Button variant="secondary" onClick={trial.start} disabled={!timing || !['idle', 'stopped'].includes(trial.st.phase)}>🎡 Tester</Button>
            <Button variant="secondary" onClick={trial.stop} disabled={trial.st.phase !== 'full'}>✋ Arrêter</Button>
            <span className="wheel-cfg-trial-state" aria-live="polite" data-testid="wheel-trial-state">{trialLabel(trial.st, segments)}</span>
          </div>
          <div className="wheel-cfg-views">
            <div>
              <h4>Roue générée, dans l'ordre (la dernière case rejoint la première)</h4>
              <div className="wheel-cfg-strip">
                {segments.map((s, i) => (
                  <div key={s.id} className={`wheel-cfg-pc${s.kind === 'special' ? ' sp' : ''}${s.mark === 'bonus' ? ' bonus' : ''}`} style={s.kind === 'special' ? undefined : s.mark === 'bonus' ? undefined : { background: s.color }}
                    title={s.mark === 'bonus' ? `${s.label}, BONUS` : undefined}>
                    <i>{i + 1}</i>
                    {s.mark === 'bonus' ? '🎁' : s.kind === 'special' ? <>{s.icon}<small>{s.label.split('\n').map((l, k) => <span key={k}>{k > 0 && <br />}{l}</span>)}</small></> : s.label}
                  </div>
                ))}
              </div>
              <h4>Sur la TV — forme « bandeau » (autour de la case 1)</h4>
              <div className="wheel-cfg-tv" ref={tvRef}><FortuneWheel wheel={trialWheel} subphase={trialSub} size="compact" /></div>
              <p className="config-section-hint">Forme « arc » : la partie haute de la roue ci-contre émerge du bas de l'écran, la case tirée au sommet.</p>
            </div>
            <div>
              <h4>Vue d'ensemble de la roue</h4>
              <Ring segments={segments} sourceRef={tvRef} active={trialActive} restIndex={trial.st.rest} />
              <p className="config-section-hint">Vue de réglage : permet de vérifier la place des cases spéciales. La TV n'affiche jamais la roue entière.</p>
            </div>
          </div>
        </>
      )}

      <h4>Durées</h4>
      {DURATIONS.map(d => (
        <div key={d.key} className="wheel-cfg-row">
          <label className="wheel-cfg-lbl" htmlFor={`wheel-${d.key}`}>{d.label}</label>
          <input id={`wheel-${d.key}`} type="number" step={d.step} value={form[d.key] === '' || form[d.key] == null ? '' : form[d.key] / 1000} onChange={(e) => editSeconds(d.key, e.target.value)} />
          <span className="config-section-hint">{d.hint}</span>
        </div>
      ))}

      {toast && <p className={`wheel-cfg-toast ${toast.type}`} role={toast.type === 'error' ? 'alert' : 'status'}>{toast.message}</p>}
      <div className="config-section-actions wheel-cfg-actions">
        <Button variant="secondary" onClick={resetDefault} disabled={!wheelDefault}>↺ Roue par défaut{defLabel}</Button>
        <Button variant="primary" onClick={save} loading={saving} disabled={!!error || !preview}>Enregistrer</Button>
      </div>
    </div>
  )
}
