import { useState, useEffect, useCallback, useRef } from 'react'
import { normalizeSoundActive } from '../utils/soundState'

// #230 — état de la sortie audio pour la pastille de l'onglet Son.
// Contrat : contracts/http-endpoints.md §Sound, GET /api/sound/status.
//
// Jumeau volontaire de useLightingStatus.js (#207) — même mécanique
// (interrogation au montage, toutes les 30 s, et après tout enregistrement
// touchant la section `sound`) — mais PAS généralisé en un hook paramétrable
// par URL/clé : handoff #230 §5, « le hook existant n'est pas générique —
// clone-le plutôt que de le paramétrer ».
//
// Distinct de useLightingStatus sur un point : il n'y a ici qu'UNE seule
// instance utile (le badge de l'onglet Son), pas d'ampoule de menu à tenir
// synchronisée — mais l'événement `window` est conservé pour rester
// symétrique et permettre un futur second consommateur sans réécriture.

export const SOUND_STATUS_URL = '/api/sound/status'
export const SOUND_STATUS_INTERVAL_MS = 30_000
export const SOUND_CHANGED_EVENT = 'buzzcontrol:sound-changed'

export const EMPTY_SOUND_STATUS = Object.freeze({ active: false })

/** À appeler après tout enregistrement touchant la section `sound`. */
export function notifySoundChanged() {
  window.dispatchEvent(new Event(SOUND_CHANGED_EVENT))
}

export function useSoundStatus({ intervalMs = SOUND_STATUS_INTERVAL_MS } = {}) {
  const [status, setStatus] = useState(EMPTY_SOUND_STATUS)
  const mountedRef = useRef(false)

  const refresh = useCallback(async () => {
    try {
      const res = await fetch(SOUND_STATUS_URL)
      if (!res.ok) {
        // 404 = serveur sans le module (binaire antérieur) : équivaut à
        // « inactif ». Toute autre erreur HTTP : dernier état conservé.
        if (res.status === 404 && mountedRef.current) setStatus(EMPTY_SOUND_STATUS)
        return null
      }
      const data = await res.json()
      const next = { active: normalizeSoundActive(data?.active) }
      if (mountedRef.current) setStatus(next)
      return next
    } catch {
      // Notre propre serveur injoignable : dernier état conservé.
      return null
    }
  }, [])

  useEffect(() => {
    mountedRef.current = true
    refresh()
    const timer = setInterval(refresh, intervalMs)
    window.addEventListener(SOUND_CHANGED_EVENT, refresh)
    return () => {
      mountedRef.current = false
      clearInterval(timer)
      window.removeEventListener(SOUND_CHANGED_EVENT, refresh)
    }
  }, [refresh, intervalMs])

  return { status, refresh }
}
