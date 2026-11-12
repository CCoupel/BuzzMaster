import { useEffect, useMemo, useRef, useState } from 'react'

// #299 (v11.7.0, LOT-3) — aperçu animé d'un effet : le navigateur REJOUE les trames calculées par le serveur
// (GET /api/lighting/effects/{id}/preview, contracts/light-effects.md §7.3), aucun second interpréteur.
// Ampoules dessinées, une par groupe, étiquetées g1, g2… ; sous elles le numéro du tick et le pas joué
// (« PRE », « pas 0 », « dernier »…), puis, pour un effet de chronomètre, un compte à rebours simulé de 20 s.
//
// Une trame par tick (`tick`, `step` : « pre » | « pas:<n> » | « dernier », §7.3) ; durée de l'aperçu (`duration_ms`) et
// compte à rebours (`chrono_s`, 0 = pas de chronomètre) lus dans la réponse — l'interface ne fixe rien elle-même.

const DARK = [30, 41, 59]

export function paintStyle(color, intensity) {
  const k = Math.max(0, Math.min(255, intensity ?? 0)) / 255
  const rgb = (color || [0, 0, 0]).map((v, i) => Math.round(DARK[i] + (v - DARK[i]) * k))
  return {
    backgroundColor: `rgb(${rgb.join(',')})`,
    boxShadow: k > 0.15 ? `0 0 ${4 + 14 * k}px rgba(${(color || [0, 0, 0]).join(',')},${0.75 * k})` : 'none',
  }
}

// Pas joué : « pre » → « PRE », « pas:0 » → « pas 0 », « dernier » → « dernier ».
export function stepLabel(step) {
  if (step === 'pre') return 'PRE'
  const m = /^pas:(\d+)$/.exec(step || '')
  return m ? `pas ${m[1]}` : (step || '')
}

// Temps restant du chronomètre simulé (s) : chrono_s − at_ms / 1000, jamais négatif ; null sans chronomètre.
export const chronoRemaining = (preview, atMs) => (preview?.chrono_s > 0 ? Math.max(0, Math.ceil(preview.chrono_s - atMs / 1000)) : null)

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

// Rangée d'ampoules (une par groupe, étiquetées g1, g2…) peinte d'après une trame du serveur ; partagée avec le simulateur de l'éditeur.
export function BulbRow({ groups, frame, className = 'lfx' }) {
  const bulbs = Array.from({ length: groups }, (_, i) => {
    const g = frame?.groups?.[i]
    return g ? paintStyle(g.color, g.intensity) : undefined
  })
  return (
    <>
      <div className={`${className}-bulbs`}>
        {bulbs.map((style, i) => <div key={i} className={`${className}-bulb`} style={style} />)}
      </div>
      <div className={`${className}-nums`}>{bulbs.map((_, i) => <span key={i}>g{i + 1}</span>)}</div>
    </>
  )
}

// `version` : change à chaque rechargement du catalogue (fichier édité, ajouté, retiré) et invalide l'aperçu mis en cache.
export default function LightEffectPreview({ id, groups, moment, version = 0, playing, onUnavailable = () => {} }) {
  const [preview, setPreview] = useState(null)
  const [error, setError] = useState(false)
  const [now, setNow] = useState(0) // ms écoulées depuis le début de la lecture
  const loaded = useRef(null) // clé (id|groups|moment|version) de l'aperçu CHARGÉ (réponse reçue), jamais d'une requête en vol

  const key = `${id}|${groups}|${moment}|${version}`
  useEffect(() => {
    if (!playing || loaded.current === key) return undefined
    let cancelled = false
    setError(false)
    fetch(`/api/lighting/effects/${encodeURIComponent(id)}/preview?groups=${groups}&moment=${encodeURIComponent(moment)}`)
      .then(async (res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        return res.json()
      })
      .then((data) => { if (!cancelled) { loaded.current = key; setPreview(data) } })
      .catch(() => { if (!cancelled) { setError(true); onUnavailable() } })
    // Nettoyage (arrêt avant la réponse, double montage de React.StrictMode) : la réponse en vol est ignorée et rien
    // n'est marqué « chargé » — la prochaine lecture redemande l'aperçu.
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
  const rem = current ? chronoRemaining(preview, current.at) : null

  return (
    <div className="lfx-stage" data-testid={`preview-${id}`}>
      <BulbRow groups={n} frame={current?.frame} />
      <div className="lfx-clock" aria-live="off">
        {error ? 'Aperçu indisponible'
          : current
            ? `tick ${current.frame.tick ?? 0}${current.frame.step ? ` · ${stepLabel(current.frame.step)}` : ''}${rem !== null ? ` · chronomètre simulé : ${rem} s` : ''}`
            : ''}
      </div>
    </div>
  )
}
