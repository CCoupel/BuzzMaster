import { useState, useEffect, useCallback } from 'react'
import { normalizeCatalog, EMPTY_CATALOG, EMPTY_GROUPS, zoneGroupCounts } from '../utils/lightEffects'

// #223 (v11.7.0, LOT-2) — catalogue d'effets lumière : GET /api/lighting/effects (contracts/light-effects.md §7.1,
// groupes et applicabilité par zone). Aucune E/S vers le pont côté serveur : lecture au montage + à la demande.
// `withGroups` : lit aussi `lighting.groups` dans GET /config.json pour connaître les zones qui existent (un bloc
// d'effets par zone) — pour les parents qui ne portent pas déjà la configuration (fenêtre « Ambiance de la manche »).
const IMPORT_REFUSALS = {
  400: "Ce fichier n'est pas un fichier d'effet (.yaml), ou l'envoi est illisible (20 fichiers au plus).",
  413: 'Envoi trop gros (2 Mo au plus).',
}

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

  // POST /api/lighting/effects/reload (§7.2) : relit le dossier ; la réponse est celle du catalogue (§7.1).
  const reload = useCallback(async () => {
    try {
      const res = await fetch('/api/lighting/effects/reload', { method: 'POST' })
      if (res.ok) { setCatalog(normalizeCatalog(await res.json())); return true }
    } catch { /* repli ci-dessous */ }
    await refresh()
    return false
  }, [refresh])

  // POST /api/lighting/effects/import (§11.7) : multipart, champ `files` répété, `on_conflict` = ask | replace | keep_both.
  // Renvoie { ok, results[] } (la réponse porte le catalogue rechargé, appliqué ici) ou { ok: false, message }.
  const importFiles = useCallback(async (files, onConflict = 'ask') => {
    const form = new FormData()
    for (const f of files) form.append('files', f, f.name)
    form.append('on_conflict', onConflict)
    try {
      const res = await fetch('/api/lighting/effects/import', { method: 'POST', body: form })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) return { ok: false, status: res.status, message: IMPORT_REFUSALS[res.status] || `Import impossible (HTTP ${res.status}).` }
      if (data?.catalogue) setCatalog(normalizeCatalog(data.catalogue))
      return { ok: true, results: Array.isArray(data?.results) ? data.results : [] }
    } catch (e) {
      return { ok: false, message: `Import impossible : ${e.message}` }
    }
  }, [])

  // DELETE /api/lighting/effects/{id} (§11.5) : supprime l'effet utilisateur, ou le fichier qui remplace un natif.
  const deleteEffect = useCallback(async (id) => {
    try {
      const res = await fetch(`/api/lighting/effects/${encodeURIComponent(id)}`, { method: 'DELETE' })
      const data = await res.json().catch(() => ({}))
      if (res.ok) {
        if (data?.catalogue) setCatalog(normalizeCatalog(data.catalogue)); else await refresh()
        return { ok: true }
      }
      const message = res.status === 409 ? 'Un effet livré ne peut pas être supprimé.'
        : res.status === 404 ? 'Effet inconnu — rechargez le catalogue.'
        : `Suppression impossible (HTTP ${res.status}).`
      if (res.status === 404) await refresh()
      return { ok: false, message }
    } catch (e) {
      return { ok: false, message: `Suppression impossible : ${e.message}` }
    }
  }, [refresh])

  return { catalog, zoneGroups, loaded, refresh, reload, importFiles, deleteEffect }
}
