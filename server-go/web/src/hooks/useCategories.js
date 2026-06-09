import { useState, useEffect, useCallback } from 'react'

export function useCategories() {
  const [categories, setCategories] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [fetchTick, setFetchTick] = useState(0)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
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
