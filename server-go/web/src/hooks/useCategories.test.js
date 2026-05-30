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
})
