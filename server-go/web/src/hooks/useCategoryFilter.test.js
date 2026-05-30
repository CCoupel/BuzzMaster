/**
 * Tests for useCategoryFilter — issue #95 (custom categories extension)
 *
 * Validates:
 * 1. Custom categories (isCustom=true) present in questions appear in availableCategories
 * 2. Backward compat: calling without customCategories param behaves identically to before
 */
import { describe, it, expect, vi } from 'vitest'
import { renderHook } from '@testing-library/react'
import { useCategoryFilter } from './useCategoryFilter'

vi.mock('../components/QuestionCard', () => ({
  CATEGORIES: {
    GEOGRAPHY: { label: 'Geographie', icon: '🌍', color: '#3b82f6' },
    SCIENCE: { label: 'Sciences', icon: '🔬', color: '#22c55e' },
  },
}))

describe('useCategoryFilter — custom categories (#95)', () => {
  it('includes custom categories in availableCategories when questions reference them', () => {
    const questions = [
      { ID: '1', CATEGORY: 'GEOGRAPHY' },
      { ID: '2', CATEGORY: 'SPORT_EXTREME' }, // custom
    ]
    const customCategories = [
      { key: 'SPORT_EXTREME', name: 'Sport Extreme', imageURL: '/files/categories/Sport%20Extreme.png', isCustom: true },
    ]

    const { result } = renderHook(() => useCategoryFilter(questions, customCategories))

    expect(result.current.availableCategories).toContain('GEOGRAPHY')
    expect(result.current.availableCategories).toContain('SPORT_EXTREME')
  })

  it('backward compat: calling without customCategories works as before', () => {
    const questions = [
      { ID: '1', CATEGORY: 'GEOGRAPHY' },
      { ID: '2', CATEGORY: 'SCIENCE' },
    ]

    const { result } = renderHook(() => useCategoryFilter(questions))

    expect(result.current.availableCategories).toContain('GEOGRAPHY')
    expect(result.current.availableCategories).toContain('SCIENCE')
    expect(result.current.filteredQuestions).toHaveLength(2)
    expect(typeof result.current.toggleCategoryFilter).toBe('function')
    expect(typeof result.current.clearCategoryFilters).toBe('function')
  })
})
