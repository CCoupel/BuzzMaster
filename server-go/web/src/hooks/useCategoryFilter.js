import { useState, useMemo } from 'react'
import { CATEGORIES } from '../utils/categoryUtils'
import { effectiveRafaleCategories } from '../utils/rafaleEffective'

// 🔴 Retour QUALIF v9.0.0.4 (point A+2) — depuis #216 une manche RAFALE porte
// une LISTE de catégories (RAFALE_CATEGORIES), q.CATEGORY (singulier) est
// vide : ce filtre l'ignorait entièrement pour ce type — une manche RAFALE
// n'apparaissait ni dans les catégories disponibles, ni dans un filtre
// actif, quelle que soit la sélection. `effectiveRafaleCategories` (#216,
// utils/rafaleEffective.js) porte le repli mono automatique pour une manche
// enregistrée avant #216. Utilisé ici par QuestionsPage.jsx ET GamePage.jsx
// (sélecteur de question admin, la surface la plus impactante des deux).
function questionCategories(q) {
  if (q.TYPE === 'RAFALE') return effectiveRafaleCategories(q)
  return q.CATEGORY ? [q.CATEGORY] : []
}

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
      questionCategories(q).forEach(c => seen.add(c))
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
    return sortedQuestions.filter(q => questionCategories(q).some(c => selectedCategories.has(c)))
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
