/**
 * Tests for useCategories — issue #95
 *
 * Validates loading states, success shape, and error handling for the hook
 * that fetches GET /api/categories.
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useCategories } from './useCategories'

const MOCK_CATEGORIES = [
  { key: 'GEOGRAPHY', name: 'Geographie', imageURL: '', isCustom: false },
  { key: 'SPORT_EXTREME', name: 'Sport Extreme', imageURL: '/files/categories/Sport%20Extreme.png', isCustom: true },
]

describe('useCategories — issue #95', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.clearAllMocks()
  })

  it('returns categories on success with correct shape', async () => {
    fetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(MOCK_CATEGORIES),
    })

    const { result } = renderHook(() => useCategories())

    await act(async () => {})

    expect(result.current.categories).toHaveLength(2)
    const first = result.current.categories[0]
    expect(first).toHaveProperty('key')
    expect(first).toHaveProperty('name')
    expect(first).toHaveProperty('imageURL')
    expect(first).toHaveProperty('isCustom')
    expect(result.current.error).toBeNull()
  })

  it('starts with loading: true before fetch resolves', () => {
    // Never-resolving promise keeps loading=true
    fetch.mockReturnValueOnce(new Promise(() => {}))

    const { result } = renderHook(() => useCategories())

    expect(result.current.loading).toBe(true)
    expect(result.current.categories).toEqual([])
  })

  it('sets loading: false after fetch completes', async () => {
    fetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(MOCK_CATEGORIES),
    })

    const { result } = renderHook(() => useCategories())

    await act(async () => {})

    expect(result.current.loading).toBe(false)
  })

  it('sets error and returns empty categories when fetch fails', async () => {
    fetch.mockResolvedValueOnce({ ok: false, status: 500 })

    const { result } = renderHook(() => useCategories())

    await act(async () => {})

    expect(result.current.error).not.toBeNull()
    expect(result.current.categories).toEqual([])
    expect(result.current.loading).toBe(false)
  })

  // ---------------------------------------------------------------------------
  // v5.7.8 — #108 : null guard — setCategories(data ?? [])
  // Si l'API retourne null (au lieu d'un tableau), categories doit rester []
  // et ne pas crasher les consommateurs qui font .map() ou .find().
  // ---------------------------------------------------------------------------

  it('null guard — réponse API null → categories reste [] (v5.7.8 #108)', async () => {
    // Le backend pourrait retourner null si la sérialisation JSON est absente
    fetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(null),
    })

    const { result } = renderHook(() => useCategories())

    await act(async () => {})

    // data ?? [] → categories = [] et non null
    expect(result.current.categories).toEqual([])
    expect(result.current.loading).toBe(false)
    expect(result.current.error).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// v5.7.1 — #97 : refetch après création de catégorie custom
// ---------------------------------------------------------------------------

describe('useCategories — refetch (v5.7.1 #97)', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.clearAllMocks()
  })

  it('expose une fonction refetch dans le retour du hook', async () => {
    fetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(MOCK_CATEGORIES),
    })

    const { result } = renderHook(() => useCategories())
    await act(async () => {})

    expect(typeof result.current.refetch).toBe('function')
  })

  it('appeler refetch déclenche un nouveau fetch GET /api/categories', async () => {
    fetch
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(MOCK_CATEGORIES),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve([
          ...MOCK_CATEGORIES,
          { key: 'NEW_CAT', name: 'New Cat', imageURL: '', isCustom: true },
        ]),
      })

    const { result } = renderHook(() => useCategories())
    await act(async () => {})

    expect(result.current.categories).toHaveLength(2)
    expect(fetch).toHaveBeenCalledTimes(1)

    // Déclencher refetch
    await act(async () => {
      result.current.refetch()
    })

    expect(fetch).toHaveBeenCalledTimes(2)
    expect(result.current.categories).toHaveLength(3)
  })

  it('après refetch, la nouvelle catégorie créée est dans la liste', async () => {
    const categoriesAfterPost = [
      ...MOCK_CATEGORIES,
      { key: 'MA_CATEGORIE', name: 'Ma Categorie', imageURL: '', isCustom: true },
    ]

    fetch
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(MOCK_CATEGORIES),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(categoriesAfterPost),
      })

    const { result } = renderHook(() => useCategories())
    await act(async () => {})

    expect(result.current.categories).toHaveLength(2)

    await act(async () => {
      result.current.refetch()
    })

    const keys = result.current.categories.map(c => c.key)
    expect(keys).toContain('MA_CATEGORIE')
  })
})
