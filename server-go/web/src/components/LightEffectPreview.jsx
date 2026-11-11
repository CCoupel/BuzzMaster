import { useEffect, useMemo, useRef, useState } from 'react'

// #299 (v11.7.0, LOT-3) — aperçu animé d'un effet : le navigateur REJOUE les trames calculées par le serveur
// (GET /api/lighting/effects/{id}/preview, contracts/light-effects.md §7.3), aucun second interpréteur.
// Ampoules dessinées, une par groupe, étiquetées g1, g2… ; sous elles le numéro du tick et le pas joué
// (« PRE », « pas 0 », « dernier »…), puis, pour un effet de chronomètre, un compte à rebours simulé de 20 s.
//
// La réponse ne liste une trame que si elle diffère de la précédente ; le tick est déduit de `at_ms` ÷ `tick_ms` et le
// pas est repris de `frame.step` s'il est fourni (champ facultatif).

const DARK = [30, 41, 59]

export function paintStyle(color, intensity) {
  const k = Math.max(0, Math.min(255, intensity ?? 0)) / 255
  const rgb = (color || [0, 0, 0]).map((v, i) => Math.round(DARK[i] + (v - DARK[i]) * k))
  return {
    backgroundColor: `rgb(${rgb.join(',')})`,
    boxShadow: k > 0.15 ? `0 0 ${4 + 14 * k}px rgba(${(color || [0, 0, 0]).join(',')},${0.75 * k})` : 'none',
  }
}

// Trame courante à l'instant t (ms) : la dernière dont `at_ms` ≤ t ; boucle sur `duration_ms` si l'effet boucle.
export function frameAt(preview, t) {
  const frames = preview?.frames
  if (!Array.isArray(frames) || frames.length === 0) return null
  const dur = preview.duration_ms > 0 ? preview.duration_ms : Infinity
  const tt = preview.loop === false ? Math.min(t, dur) : (Number.isFinite(dur) ? t % dur : t)
  let cur = frames[0]
  for (const f of frames) { if (f.at_ms <= tt) cur = f; else break }
  return { frame: cur, at: tt }
}

export default function LightEffectPreview({ id, groups, moment, tickMs, chrono, playing, onUnavailable = () => {} }) {
  const [preview, setPreview] = useState(null)
  const [error, setError] = useState(false)
  const [now, setNow] = useState(0) // ms écoulées depuis le début de la lecture
  const loaded = useRef(null) // clé (id|groups|moment) de l'aperçu chargé

  const key = `${id}|${groups}|${moment}`
  useEffect(() => {
    if (!playing || loaded.current === key) return undefined
    let cancelled = false
    loaded.current = key
    setError(false)
    fetch(`/api/lighting/effects/${encodeURIComponent(id)}/preview?groups=${groups}&moment=${encodeURIComponent(moment)}`)
      .then(async (res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        return res.json()
      })
      .then((data) => { if (!cancelled) setPreview(data) })
      .catch(() => { if (!cancelled) { loaded.current = null; setError(true); onUnavailable() } })
    return () => { cancelled = true }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [playing, key])

  useEffect(() => {
    if (!playing) { setNow(0); return undefined }
    const start = Date.now()
    const t = setInterval(() => setNow(Date.now() - start), 50)
    return () => clearInterval(t)
  }, [playing])

  const current = useMemo(() => (playing ? frameAt(preview, now) : null), [playing, preview, now])
  const n = preview?.groups ?? groups
  const bulbs = Array.from({ length: n }, (_, i) => {
    const g = current?.frame?.groups?.[i]
    return g ? paintStyle(g.color, g.intensity) : undefined
  })
  const tick = current ? (Number.isFinite(current.frame.tick) ? current.frame.tick : (tickMs > 0 ? Math.floor(current.at / tickMs) : 0)) : null
  const rem = 20 - (Math.floor((current?.at ?? 0) / 1000) % 21)

  return (
    <div className="lfx-stage" data-testid={`preview-${id}`}>
      <div className="lfx-bulbs">
        {bulbs.map((style, i) => <div key={i} className="lfx-bulb" style={style} />)}
      </div>
      <div className="lfx-nums">{bulbs.map((_, i) => <span key={i}>g{i + 1}</span>)}</div>
      <div className="lfx-clock" aria-live="off">
        {error ? 'Aperçu indisponible'
          : current
            ? `tick ${tick}${current.frame.step ? ` · ${current.frame.step}` : ''}${chrono ? ` · chronomètre simulé : ${rem} s` : ''}`
            : ''}
      </div>
    </div>
  )
}
