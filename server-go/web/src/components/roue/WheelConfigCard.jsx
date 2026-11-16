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
import { useEffect, useRef, useState } from 'react'
import Button from '../Button'
import FortuneWheel, { buildFortuneSegments } from './FortuneWheel'
import './WheelConfigCard.css'

const PARAMS = [
  { key: 'max_points', label: 'Points maximum', hint: 'valeur de la meilleure case', min: 2 },
  { key: 'step', label: 'Pas', hint: 'écart entre deux valeurs ; aussi la plus petite valeur', min: 1 },
  { key: 'bankrupt_count', label: '💸 Banqueroute', hint: 'cases qui vident la cagnotte', min: 0 },
  { key: 'lose_turn_count', label: '⏭ Perdu', hint: 'cases qui font passer le tour', min: 0 },
]
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
    spin_period_ms: x.spin_period_ms, spin_max_ms: x.spin_max_ms, stop_duration_ms: x.stop_duration_ms,
    flip_interval_ms: x.flip_interval_ms,
  }
}

function Ring({ segments }) {
  // Vue de réglage : dessin des cases telles que renvoyées par le serveur (aucune règle ici).
  const n = segments.length
  const a = 360 / n
  const R = 480
  const r = 300
  const pt = (rad, deg) => { const t = (deg * Math.PI) / 180; return [(500 + rad * Math.cos(t)).toFixed(1), (500 + rad * Math.sin(t)).toFixed(1)] }
  return (
    <svg viewBox="0 0 1000 1000" className="wheel-cfg-ring" role="img" aria-label={`Roue de ${n} cases`}>
      <circle cx="500" cy="500" r="290" fill="#f9fafb" />
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
            <path d={`M${x0} ${y0} A${R} ${R} 0 0 1 ${x1} ${y1} L${x2} ${y2} A${r} ${r} 0 0 0 ${x3} ${y3} Z`} fill={s.color || PALETTE[0]} stroke="#0b1020" strokeWidth="3" />
            <text x={lx} y={ly} transform={`rotate(${mid + 90} ${lx} ${ly})`} textAnchor="middle" dominantBaseline="central" fontSize={fs} fontWeight="900" fill={s.kind === 'special' ? '#fff' : '#111827'}>
              {s.icon || s.label}
            </text>
          </g>
        )
      })}
      <path d="M500 46 L478 6 L522 6 Z" fill="#ef4444" />
      <text x="500" y="490" textAnchor="middle" fontSize="34" fontWeight="800" fill="#374151">{n} cases</text>
      <text x="500" y="535" textAnchor="middle" fontSize="24" fill="#6b7280">case 1 en haut, lecture ↻</text>
    </svg>
  )
}

export default function WheelConfigCard({ wheel, wheelDefault, onSaved }) {
  const [form, setForm] = useState(() => fromServer(wheel))
  const [preview, setPreview] = useState(null) // { COUNT, step, SEGMENTS, step_adjusted_from }
  const [error, setError] = useState('')
  const [adjusted, setAdjusted] = useState(null) // { from, to }
  const [saving, setSaving] = useState(false)
  const [toast, setToast] = useState(null)
  const seq = useRef(0)

  // Les 4 paramètres qui déterminent la roue (les durées ne changent pas son dessin).
  const { max_points: mp, step: st, bankrupt_count: bc, lose_turn_count: lc } = form
  useEffect(() => {
    const body = { max_points: num(mp), step: num(st), bankrupt_count: num(bc), lose_turn_count: num(lc) }
    if (![body.max_points, body.step, body.bankrupt_count, body.lose_turn_count].every(Number.isFinite)) {
      setPreview(null)
      setError('les quatre réglages sont des nombres entiers')
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
  }, [mp, st, bc, lc])

  const edit = (key, value) => {
    setAdjusted(null)
    setToast(null)
    setForm(f => ({ ...f, [key]: value === '' ? '' : Number(value) }))
  }
  const editSeconds = (key, value) => {
    setToast(null)
    setForm(f => ({ ...f, [key]: value === '' ? '' : Math.round(Number(value) * 1000) }))
  }
  const resetDefault = () => {
    if (!wheelDefault) return
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
        setToast({ type: 'error', message: text || `Échec de l'enregistrement (HTTP ${res.status}).` })
      }
    } catch (e) {
      setToast({ type: 'error', message: e.message })
    } finally {
      setSaving(false)
    }
  }

  const segments = preview ? buildFortuneSegments(preview.SEGMENTS || []) : []
  const values = segments.filter(s => s.kind === 'value').map(s => Number(s.label))
  const special = segments.length - values.length
  const total = values.reduce((a, b) => a + b, 0)
  const defLabel = wheelDefault ? ` (${wheelDefault.max_points} · ${wheelDefault.step} · ${wheelDefault.bankrupt_count} · ${wheelDefault.lose_turn_count})` : ''

  return (
    <div className="config-section wheel-cfg" data-testid="wheel-config-card">
      <h3 className="config-section-title">🎡 Roue de la Fortune</h3>
      <p className="config-section-hint">
        La roue du mode ROUE est construite à partir de ces réglages. Elle est commune à toutes les questions ROUE ; sa forme (bandeau ou arc) se choisit sur chaque question.
      </p>

      <div className="wheel-cfg-params">
        {PARAMS.map(p => (
          <div key={p.key} className="wheel-cfg-param">
            <label htmlFor={`wheel-${p.key}`}>{p.label}</label>
            <input id={`wheel-${p.key}`} type="number" min={p.min} step="1" value={form[p.key] ?? ''} onChange={(e) => edit(p.key, e.target.value)} />
            <small>{p.hint}</small>
          </div>
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
            <b>{form.bankrupt_count}</b> Banqueroute · <b>{form.lose_turn_count}</b> Perdu —{' '}
            {special ? `une chance sur ${fmt(Number((segments.length / special).toFixed(1)))} de tomber sur une case spéciale` : 'aucune case spéciale'}
            {' '}· gain moyen d'une case : {Math.round(total / segments.length)}.
          </div>
          <div className="wheel-cfg-views">
            <div>
              <h4>Roue générée, dans l'ordre (la dernière case rejoint la première)</h4>
              <div className="wheel-cfg-strip">
                {segments.map((s, i) => (
                  <div key={s.id} className={`wheel-cfg-pc${s.kind === 'special' ? ' sp' : ''}`} style={s.kind === 'special' ? undefined : { background: s.color }}>
                    <i>{i + 1}</i>
                    {s.kind === 'special' ? <>{s.icon}<small>{s.label.split('\n').map((l, k) => <span key={k}>{k > 0 && <br />}{l}</span>)}</small></> : s.label}
                  </div>
                ))}
              </div>
              <h4>Sur la TV — forme « bandeau » (autour de la case 1)</h4>
              <div className="wheel-cfg-tv"><FortuneWheel wheel={{ LAYOUT: 'strip', SEGMENTS: preview.SEGMENTS, CURRENT: 0 }} size="compact" /></div>
              <p className="config-section-hint">Forme « arc » : la partie haute de la roue ci-contre émerge du bas de l'écran, la case tirée au sommet.</p>
            </div>
            <div>
              <h4>Vue d'ensemble de la roue</h4>
              <Ring segments={segments} />
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
