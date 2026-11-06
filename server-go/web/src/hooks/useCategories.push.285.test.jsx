// #285 (LOT-4 rév. 2) — useCategories applique CATEGORIES_CHANGED directement (sans refetch), cache-buster d'icône
// `?v=<TS>` pour icon/created, et reste utilisable hors GameProvider (contexte null ⇒ GET au montage + refetch()).
// Contrat : contracts/websocket-actions.md §CATEGORIES_CHANGED « Règles côté client ».
import React from 'react'
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useCategories } from './useCategories'
import { CategoriesPushContext } from './CategoriesPushContext'
import { categoryMeta } from '../utils/categoryUtils'

const INITIAL = [{ key: 'GEOGRAPHY', name: 'Géo', imageURL: '', color: '#3b82f6', isCustom: false }]

function withPush(initial) {
  let current = initial
  const listeners = new Set()
  const Provider = ({ children }) => {
    const [, force] = React.useReducer(x => x + 1, 0)
    React.useEffect(() => { listeners.add(force); return () => listeners.delete(force) }, [])
    return <CategoriesPushContext.Provider value={current}>{children}</CategoriesPushContext.Provider>
  }
  const push = value => { current = value; listeners.forEach(f => f()) }
  return { Provider, push }
}

describe('useCategories — CATEGORIES_CHANGED (#285)', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({ ok: true, json: () => Promise.resolve(INITIAL) })))
  })
  afterEach(() => { vi.unstubAllGlobals(); vi.clearAllMocks() })

  it('applique la liste poussée sans refaire de GET', async () => {
    const { Provider, push } = withPush(null)
    const { result } = renderHook(() => useCategories(), { wrapper: Provider })
    await act(async () => {})
    expect(fetch).toHaveBeenCalledTimes(1) // GET au montage seulement
    const pushed = [{ key: 'GEOGRAPHY', name: 'Géographie', imageURL: '', color: '#ff0000', isCustom: false }]
    act(() => { push({ categories: pushed, change: { TYPE: 'renamed', KEY: 'GEOGRAPHY' }, ts: 1000 }) })
    expect(result.current.categories).toEqual(pushed)
    expect(fetch).toHaveBeenCalledTimes(1)
  })

  it('une catégorie supprimée disparaît de la liste', async () => {
    const { Provider, push } = withPush(null)
    const { result } = renderHook(() => useCategories(), { wrapper: Provider })
    await act(async () => {})
    act(() => { push({ categories: [], change: { TYPE: 'deleted', KEY: 'CINEMA' }, ts: 2000 }) })
    expect(result.current.categories).toEqual([])
  })

  it('icon et created : l\'imageURL de la clé touchée porte ?v=<TS> ; renamed/colored ne changent pas l\'URL', async () => {
    const { Provider, push } = withPush(null)
    const list = [{ key: 'CINEMA', name: 'Cinéma', imageURL: '/files/categories/CINEMA.png', color: '#0ea5e9', isCustom: true }]
    const { result } = renderHook(() => useCategories(), { wrapper: Provider })
    await act(async () => {})
    act(() => { push({ categories: list, change: { TYPE: 'colored', KEY: 'CINEMA' }, ts: 3000 }) })
    expect(categoryMeta('CINEMA', result.current.categories).imageURL).toBe('/files/categories/CINEMA.png')
    act(() => { push({ categories: list, change: { TYPE: 'icon', KEY: 'CINEMA' }, ts: 4242 }) })
    expect(categoryMeta('CINEMA', result.current.categories).imageURL).toBe('/files/categories/CINEMA.png?v=4242')
    act(() => { push({ categories: list, change: { TYPE: 'created', KEY: 'CINEMA' }, ts: 5151 }) })
    expect(categoryMeta('CINEMA', result.current.categories).imageURL).toBe('/files/categories/CINEMA.png?v=5151')
  })

  it('hors GameProvider (contexte null) : GET au montage et refetch() fonctionnent', async () => {
    const { result } = renderHook(() => useCategories())
    await act(async () => {})
    expect(fetch).toHaveBeenCalledTimes(1)
    expect(result.current.categories).toEqual(INITIAL)
    await act(async () => { result.current.refetch() })
    expect(fetch).toHaveBeenCalledTimes(2)
  })
})
