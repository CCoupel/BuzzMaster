import { useState, useEffect, useCallback, useContext } from 'react'
import { CategoriesPushContext } from './CategoriesPushContext'
import { bustCategoryIcon } from '../utils/categoryUtils'

export function useCategories() {
  const [categories, setCategories] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [fetchTick, setFetchTick] = useState(0)

  // #285 : CATEGORIES_CHANGED (WebSocket, via GameProvider) porte la liste complète résolue :
  // appliquée directement, sans refetch. Hors GameProvider : contexte null ⇒ GET au montage + refetch().
  const push = useContext(CategoriesPushContext)
  const pushTs = push?.ts
  useEffect(() => {
    if (!push) return
    setCategories(push.categories ?? [])
    setError(null)
    if (push.change && (push.change.TYPE === 'icon' || push.change.TYPE === 'created')) {
      bustCategoryIcon(push.change.KEY, push.ts)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pushTs])

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
