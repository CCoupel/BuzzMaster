import { useEffect, useState } from 'react'

// Suit une media query. Repli sûr (jsdom, navigateurs anciens) : `fallback`.
export default function useMediaQuery(query, fallback = false) {
  const get = () =>
    typeof window !== 'undefined' && typeof window.matchMedia === 'function'
      ? window.matchMedia(query).matches
      : fallback
  const [matches, setMatches] = useState(get)

  useEffect(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return undefined
    const mql = window.matchMedia(query)
    const onChange = () => setMatches(mql.matches)
    onChange()
    if (mql.addEventListener) {
      mql.addEventListener('change', onChange)
      return () => mql.removeEventListener('change', onChange)
    }
    mql.addListener?.(onChange)
    return () => mql.removeListener?.(onChange)
  }, [query])

  return matches
}
