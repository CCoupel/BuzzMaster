import { useState, useMemo } from 'react'
import { CATEGORIES } from '../components/QuestionCard'

/**
 * useCategoryFilter — shared category filter logic for question lists.
 *
 * @param {Array} sortedQuestions - Pre-sorted question array
 * @param {Array} customCategories - Optional custom categories from API
 * @returns {{ selectedCategories, availableCategories, filteredQuestions, toggleCategoryFilter, clearCategoryFilters }}
 */
export function useCategoryFilter(sortedQuestions, customCategories = []) {
  const [selectedCategories, setSelectedCategories] = useState(new Set())

  const availableCategories = useMemo(() => {
    const seen = new Set()
    sortedQuestions.forEach(q => {
      if (q.CATEGORY) seen.add(q.CATEGORY)
    })
    // Hardcoded categories in definition order
    const hardcoded = Object.keys(CATEGORIES).filter(k => seen.has(k))
    // Custom categories (isCustom=true) present in questions
    const custom = customCategories
      .filter(c => c.isCustom && seen.has(c.key))
      .map(c => c.key)
    return [...hardcoded, ...custom]
  }, [sortedQuestions, customCategories])

  const filteredQuestions = useMemo(() => {
    if (selectedCategories.size === 0) return sortedQuestions
    return sortedQuestions.filter(q => q.CATEGORY && selectedCategories.has(q.CATEGORY))
  }, [sortedQuestions, selectedCategories])

  const toggleCategoryFilter = (catKey) => {
    setSelectedCategories(prev => {
      const next = new Set(prev)
      if (next.has(catKey)) next.delete(catKey)
      else next.add(catKey)
      return next
    })
  }

  const clearCategoryFilters = () => setSelectedCategories(new Set())

  return { selectedCategories, availableCategories, filteredQuestions, toggleCategoryFilter, clearCategoryFilters }
}
