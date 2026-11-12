import { useEffect, useRef, useState } from 'react'
import {
  TRY_EVENT, TRY_REFUSALS, TRY_SECONDS, ZONE_LABEL, readTryZone, saveTryZone, visibleZones,
} from '../utils/lightEffects'

// #210 (v11.8.0, LOT-2, F2.3) — « Essayer sur la salle » de l'éditeur : joue le BROUILLON courant (sans l'enregistrer) sur les vraies ampoules
// d'une zone, via POST /api/lighting/effects/draft/try (contrat §11.10 ; mêmes refus que §7.4). Zone choisie parmi celles qui existent ;
// la dernière zone est retenue par navigateur (partagée avec le catalogue).
//
// Props : body ({effect} | {yaml} : le brouillon courant), valid (brouillon valide et vérifié), zoneGroups { ambiance, animateur, public },
//   minGroups, seconds (durée annoncée : 5 s pour un effet de score seul, 10 s sinon ; le serveur fixe la vraie), tryUnavailable(zone) -> '' | raison (éclairage, pont, question en cours, forçage), onStopTry()

const stopRoute = '/api/lighting/effects/try/stop'

export default function EffectTryBar({ body, valid, zoneGroups, minGroups = 0, tryUnavailable = () => '', onStopTry, seconds = TRY_SECONDS }) {
  const [zone, setZone] = useState(readTryZone)
  const [trying, setTrying] = useState(false)
  const [starting, setStarting] = useState(false)
  const [message, setMessage] = useState('')
  const timer = useRef(null)
  const mounted = useRef(true)

  useEffect(() => {
    mounted.current = true
    const onOther = (e) => { if (e.detail?.source !== 'editor') { clearTimeout(timer.current); setTrying(false) } }
    window.addEventListener(TRY_EVENT, onOther)
    return () => { mounted.current = false; window.removeEventListener(TRY_EVENT, onOther); clearTimeout(timer.current) }
  }, [])

  const zones = visibleZones(zoneGroups)
  const z = zones.includes(zone) ? zone : 'ambiance'
  const n = zoneGroups?.[z] ?? 0
  const reason = trying ? ''
    : !valid ? "Corrigez les erreurs de l'effet avant de l'essayer."
    : minGroups > n ? `Cet effet demande ${minGroups} groupe${minGroups > 1 ? 's' : ''} ; la zone ${ZONE_LABEL[z]} en a ${n}.`
    : tryUnavailable(z)

  const start = async () => {
    setStarting(true); setMessage('')
    try {
      const res = await fetch('/api/lighting/effects/draft/try', {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...body, zone: z, speed: 1 }),
      })
      let data = {}
      let raw = ''
      if (typeof res.text === 'function') { raw = await res.text().catch(() => ''); try { data = raw ? JSON.parse(raw) : {} } catch { /* texte brut */ } } else { data = await res.json().catch(() => ({})) }
      if (!mounted.current) return
      if (res.ok) {
        const ms = Number.isFinite(data.duration_ms) ? data.duration_ms : TRY_SECONDS * 1000
        window.dispatchEvent(new CustomEvent(TRY_EVENT, { detail: { source: 'editor' } }))
        clearTimeout(timer.current)
        setTrying(true)
        timer.current = setTimeout(() => { if (mounted.current) setTrying(false) }, ms)
        return
      }
      setMessage(res.status === 409 ? (TRY_REFUSALS[data?.reason] || 'Essai refusé.')
        : res.status === 503 ? "Pont injoignable — impossible d'essayer l'effet."
        : res.status === 400 ? "Corrigez les erreurs de l'effet avant de l'essayer."
        : res.status === 404 ? 'Effet inconnu — rechargez le catalogue.'
        : `Essai impossible (HTTP ${res.status}).`)
    } catch (e) {
      if (mounted.current) setMessage(`Essai impossible : ${e.message}`)
    } finally { if (mounted.current) setStarting(false) }
  }
  const stop = async () => {
    clearTimeout(timer.current)
    setTrying(false)
    try { await fetch(stopRoute, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' }) } catch { /* l'essai s'arrête seul */ }
    onStopTry?.()
  }

  return (
    <div className="efx-trybar" data-testid="effect-trybar">
      <b className="efx-trybar-t">Essayer sur la salle</b>
      <span className="efx-hint efx-nomargin">Zone :</span>
      <span className="efx-chips" role="group" aria-label="Zone où jouer l'essai">
        {['ambiance', 'animateur', 'public'].map(k => {
          const exists = zones.includes(k)
          const cnt = zoneGroups?.[k] ?? 0
          return (
            <button
              key={k} type="button" className={`efx-chip${k === z ? ' is-on' : ''}`} aria-pressed={k === z}
              disabled={!exists || trying} title={exists ? undefined : `Aucun groupe dans la zone ${ZONE_LABEL[k]}`}
              onClick={() => { setZone(k); saveTryZone(k) }}
            >{ZONE_LABEL[k]} · {cnt ? `${cnt} groupe${cnt > 1 ? 's' : ''}` : 'aucun groupe'}</button>
          )
        })}
      </span>
      {trying
        ? <button type="button" className="efx-btn" onClick={stop}>■ Arrêter l'essai</button>
        : <button type="button" className="efx-btn" disabled={!!reason || starting} title={reason || undefined} onClick={start}>💡 Essayer {seconds} s</button>}
      <span className="efx-hint efx-nomargin" role="status">
        {message || reason || "Joue l'effet tel qu'il est à l'écran, sans l'enregistrer, sur les vraies ampoules de la zone choisie ; puis l'éclairage revient à son état normal."}
      </span>
    </div>
  )
}
