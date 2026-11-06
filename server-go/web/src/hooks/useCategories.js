import { useState, useEffect, useCallback } from 'react'

/** Évènement window émis par useWebSocket à chaque message QUESTIONS (#285). */
export const CATEGORIES_RELOAD_EVENT = 'buzz:questions-update'

export function useCategories() {
  const [categories, setCategories] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [fetchTick, setFetchTick] = useState(0)

  // #285 : le serveur n'a pas de message dédié — OnCategoriesChanged diffuse QUESTIONS, que
  // useWebSocket relaie en évènement window ; on recharge /api/categories à sa réception.
  useEffect(() => {
    const onUpdate = () => setFetchTick(t => t + 1)
    window.addEventListener(CATEGORIES_RELOAD_EVENT, onUpdate)
    return () => window.removeEventListener(CATEGORIES_RELOAD_EVENT, onUpdate)
  }, [])

  useEffect(() => {
    let cancelled = false
    if (fetchTick === 0) setLoading(true) // rechargements suivants : pas de clignotement
    fetch('/api/categories')
      .then(res => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        return res.json()
      })
      .then(data => {
        if (!cancelled) {
          setCategories(data ?? [])
          setLoading(false)
        }
      })
      .catch(err => {
        if (!cancelled) {
          setError(err.message)
          setLoading(false)
        }
      })
    return () => { cancelled = true }
  }, [fetchTick])

  const refetch = useCallback(() => setFetchTick(t => t + 1), [])

  return { categories, loading, error, refetch }
}
