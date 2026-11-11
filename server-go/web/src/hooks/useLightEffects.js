import { useState, useEffect, useCallback } from 'react'
import { normalizeCatalog, EMPTY_CATALOG, findHostGroup } from '../utils/lightEffects'

// #223 (v11.7.0, LOT-2) — catalogue d'effets lumière : GET /api/lighting/effects
// (contracts/light-effects.md §7.1). Aucune E/S vers le pont côté serveur : lecture au montage + à la demande.
// `withHost` : lit aussi `lighting.groups` dans GET /config.json pour savoir si un groupe animateur existe
// (pour les parents qui ne portent pas déjà la configuration, ex. la fenêtre « Ambiance de la manche »).
export function useLightEffects({ withHost = false } = {}) {
  const [catalog, setCatalog] = useState(EMPTY_CATALOG)
  const [hostGroup, setHostGroup] = useState(null)
  const [loaded, setLoaded] = useState(false)

  const refresh = useCallback(async () => {
    try {
      const res = await fetch('/api/lighting/effects')
      setCatalog(res.ok ? normalizeCatalog(await res.json()) : EMPTY_CATALOG)
    } catch {
      setCatalog(EMPTY_CATALOG)
    }
    if (withHost) {
      try {
        const res = await fetch('/config.json')
        const data = res.ok ? await res.json() : null
        setHostGroup(findHostGroup(data?.lighting?.groups))
      } catch {
        setHostGroup(null)
      }
    }
    setLoaded(true)
  }, [withHost])

  useEffect(() => { refresh() }, [refresh])

  return { catalog, hostGroup, loaded, refresh }
}
