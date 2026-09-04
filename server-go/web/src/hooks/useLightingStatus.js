import { useState, useEffect, useCallback, useRef } from 'react'
import { normalizeLightingState } from '../utils/lightingState'

// #207 — état de l'éclairage d'ambiance pour l'ampoule du menu (Navbar) et le
// badge de la page Ambiance. Contrat : contracts/hue-bridge.md §7 + §7.1.
//
// GET /api/lighting/status est interrogé AU MONTAGE, TOUTES LES 30 s, et après
// tout enregistrement de configuration. Le précédent `useUpdates` n'appelle
// qu'au montage (Navbar.jsx) : insuffisant ici, un pont peut devenir
// injoignable PENDANT une session. L'intervalle est sans coût : l'endpoint ne
// fait aucune I/O, il lit un état déjà en mémoire côté serveur.
//
// « Après tout enregistrement » : la Navbar et la page Ambiance sont deux
// arbres React distincts, chacun avec sa propre instance du hook. La page
// appelle `notifyLightingChanged()` après un enregistrement ; toutes les
// instances montées se rafraîchissent aussitôt via un événement `window`.
// Aucun contexte nouveau, aucun canal WebSocket (contrat §7.1, candidat
// d'amélioration, pas un prérequis).

export const LIGHTING_STATUS_URL = '/api/lighting/status'
export const LIGHTING_STATUS_INTERVAL_MS = 30_000
export const LIGHTING_CHANGED_EVENT = 'buzzcontrol:lighting-changed'

export const EMPTY_LIGHTING_STATUS = Object.freeze({
  state: 'disabled',
  bridge_id: '',
  bridge_ip: '',
  lights_ok: 0,
  lights_total: 0,
})

/** À appeler après tout enregistrement touchant la section `lighting`. */
export function notifyLightingChanged() {
  window.dispatchEvent(new Event(LIGHTING_CHANGED_EVENT))
}

export function useLightingStatus({ intervalMs = LIGHTING_STATUS_INTERVAL_MS } = {}) {
  const [status, setStatus] = useState(EMPTY_LIGHTING_STATUS)
  const mountedRef = useRef(false)

  const refresh = useCallback(async () => {
    try {
      const res = await fetch(LIGHTING_STATUS_URL)
      if (!res.ok) {
        // 404 = serveur sans le module (binaire antérieur) : équivaut à
        // « non configuré ». Toute autre erreur HTTP : on garde le dernier
        // état connu plutôt que de faire clignoter l'ampoule.
        if (res.status === 404 && mountedRef.current) setStatus(EMPTY_LIGHTING_STATUS)
        return null
      }
      const data = await res.json()
      const next = {
        ...EMPTY_LIGHTING_STATUS,
        ...(data && typeof data === 'object' ? data : {}),
        state: normalizeLightingState(data?.state),
      }
      if (mountedRef.current) setStatus(next)
      return next
    } catch {
      // Notre propre serveur injoignable : la page entière est déconnectée,
      // la Navbar le dit déjà (badge Connexion). Dernier état conservé.
      return null
    }
  }, [])

  useEffect(() => {
    mountedRef.current = true
    refresh()
    const timer = setInterval(refresh, intervalMs)
    window.addEventListener(LIGHTING_CHANGED_EVENT, refresh)
    return () => {
      mountedRef.current = false
      clearInterval(timer)
      window.removeEventListener(LIGHTING_CHANGED_EVENT, refresh)
    }
  }, [refresh, intervalMs])

  return { status, refresh }
}
