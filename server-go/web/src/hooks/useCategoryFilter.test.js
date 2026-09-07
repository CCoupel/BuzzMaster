/**
 * Tests for useCategoryFilter — issue #95 (custom categories extension)
 *
 * Validates:
 * 1. Custom categories (isCustom=true) present in questions appear in availableCategories
 * 2. Backward compat: calling without customCategories param behaves identically to before
 */
import { describe, it, expect, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'
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

// ---------------------------------------------------------------------------
// A+2 (retour QUALIF v9.0.0.4, plan-v900-correctifs-qualif-20260906-104500.md
// §4 Lot A+, réouverture #216) — une manche RAFALE n'a plus une CATEGORY
// unique depuis #216 (RAFALE_CATEGORIES, plusieurs catégories tirées) : ce
// hook ne lisait encore que `q.CATEGORY` (`:17` availableCategories, `:30`
// filteredQuestions), donc une manche RAFALE multi-catégories sans CATEGORY
// générique disparaissait purement et simplement du filtre — invisible dans
// la liste dès qu'un filtre catégorie quelconque était actif.
//
// Ces tests couvrent les DEUX surfaces consommatrices en une fois :
// QuestionsPage.jsx:360 et GamePage.jsx:245 (le sélecteur de question de
// l'admin en pleine soirée, le plus impactant des deux) appellent toutes les
// deux `useCategoryFilter(sortedQuestions, customCategories)` avec la liste
// COMPLÈTE, sans filtrage préalable propre à la page (vérifié par lecture
// des deux call sites) — un test au niveau du hook exerce donc exactement la
// même logique que l'une ou l'autre page rendrait, sans dupliquer un test de
// composant lourd deux fois pour un comportement identique.
// ---------------------------------------------------------------------------

describe('useCategoryFilter — manches RAFALE via catégories effectives (A+2, retour QUALIF v9.0.0.4)', () => {
  it('une manche RAFALE multi-catégories (#216) apparaît dans availableCategories pour CHACUNE de ses catégories effectives, même sans CATEGORY générique', () => {
    const questions = [
      { ID: 'r1', TYPE: 'RAFALE', RAFALE_CATEGORIES: ['HISTORY', 'SCIENCE'] },
    ]
    const { result } = renderHook(() => useCategoryFilter(questions))
    expect(result.current.availableCategories).toContain('HISTORY')
    expect(result.current.availableCategories).toContain('SCIENCE')
  })

  it('filtrer sur UNE des catégories effectives d\'une manche RAFALE la garde visible (ne l\'écarte plus)', () => {
    const questions = [
      { ID: 'r1', TYPE: 'RAFALE', RAFALE_CATEGORIES: ['HISTORY', 'SCIENCE'] },
      { ID: 'q1', CATEGORY: 'GEOGRAPHY' },
    ]
    const { result } = renderHook(() => useCategoryFilter(questions))
    act(() => result.current.toggleCategoryFilter('SCIENCE'))
    const ids = result.current.filteredQuestions.map(q => q.ID)
    expect(ids).toContain('r1')
    expect(ids).not.toContain('q1')
  })

  it('filtrer sur une catégorie absente des catégories effectives de la manche RAFALE l\'écarte toujours (le filtre reste sélectif, pas "toujours visible")', () => {
    const questions = [
      { ID: 'r1', TYPE: 'RAFALE', RAFALE_CATEGORIES: ['HISTORY', 'SCIENCE'] },
      { ID: 'q1', CATEGORY: 'GEOGRAPHY' },
    ]
    const { result } = renderHook(() => useCategoryFilter(questions))
    act(() => result.current.toggleCategoryFilter('GEOGRAPHY'))
    expect(result.current.filteredQuestions.map(q => q.ID)).toEqual(['q1'])
  })

  it('rétro-compatibilité mono : une manche RAFALE enregistrée avant #216 (CATEGORY seul, pas de RAFALE_CATEGORIES) continue de filtrer correctement', () => {
    const questions = [
      { ID: 'r1', TYPE: 'RAFALE', CATEGORY: 'HISTORY' },
    ]
    const { result } = renderHook(() => useCategoryFilter(questions))
    expect(result.current.availableCategories).toContain('HISTORY')
    act(() => result.current.toggleCategoryFilter('HISTORY'))
    expect(result.current.filteredQuestions.map(q => q.ID)).toEqual(['r1'])
  })

  it('non-régression — une question non-RAFALE continue de ne compter que sa CATEGORY unique', () => {
    const questions = [
      { ID: 'q1', CATEGORY: 'GEOGRAPHY' },
      { ID: 'q2', CATEGORY: 'SCIENCE' },
    ]
    const { result } = renderHook(() => useCategoryFilter(questions))
    act(() => result.current.toggleCategoryFilter('GEOGRAPHY'))
    expect(result.current.filteredQuestions.map(q => q.ID)).toEqual(['q1'])
  })
})
