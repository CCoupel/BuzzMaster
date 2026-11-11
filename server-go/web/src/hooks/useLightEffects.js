import { useState, useEffect, useCallback } from 'react'
import { normalizeCatalog, EMPTY_CATALOG, EMPTY_GROUPS, zoneGroupCounts } from '../utils/lightEffects'

// #223 (v11.7.0, LOT-2) — catalogue d'effets lumière : GET /api/lighting/effects (contracts/light-effects.md §7.1,
// groupes et applicabilité par zone). Aucune E/S vers le pont côté serveur : lecture au montage + à la demande.
// `withGroups` : lit aussi `lighting.groups` dans GET /config.json pour connaître les zones qui existent (un bloc
// d'effets par zone) — pour les parents qui ne portent pas déjà la configuration (fenêtre « Ambiance de la manche »).
export function useLightEffects({ withGroups = false } = {}) {
  const [catalog, setCatalog] = useState(EMPTY_CATALOG)
  const [zoneGroups, setZoneGroups] = useState(EMPTY_GROUPS)
  const [loaded, setLoaded] = useState(false)

  const refresh = useCallback(async () => {
    try {
      const res = await fetch('/api/lighting/effects')
      setCatalog(res.ok ? normalizeCatalog(await res.json()) : EMPTY_CATALOG)
    } catch {
      setCatalog(EMPTY_CATALOG)
    }
    if (withGroups) {
      try {
        const res = await fetch('/config.json')
        const data = res.ok ? await res.json() : null
        setZoneGroups(zoneGroupCounts(data?.lighting?.groups))
      } catch {
        setZoneGroups(EMPTY_GROUPS)
      }
    }
    setLoaded(true)
  }, [withGroups])

  useEffect(() => { refresh() }, [refresh])

  return { catalog, zoneGroups, loaded, refresh }
}
