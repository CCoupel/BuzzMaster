import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import CategoryFilterBar from './CategoryFilterBar'

// ---------------------------------------------------------------------------
// CategoryFilterBar — barre de filtre par catégorie, extraite de
// QuestionsPage.jsx (#40) pour être partagée avec RafalePage.jsx (v8.0.0,
// #16/#197, bugfix "doublon de catégories dans le pool RAFALE", SHA
// ba960bdf). Remplace l'ancienne variante `.rafale-chip` de RafalePage.jsx,
// qui divergeait sur 2 points : pas de garde `if (!meta) return null`, et
// `customCategories` reçu non filtré (isCustom brut).
//
// Composant nouvellement extrait, aucun test dédié au moment où ce fichier
// est écrit.
// ---------------------------------------------------------------------------

const CUSTOM_CATEGORIES = [
  { key: 'CUSTOM_1', name: 'Custom Un', isCustom: true, imageURL: '/img/custom1.png' },
]

function renderBar(overrides = {}) {
  const onToggle = overrides.onToggle ?? vi.fn()
  const onClear = overrides.onClear ?? vi.fn()
  const utils = render(
    <CategoryFilterBar
      availableCategories={overrides.availableCategories ?? ['HISTORY', 'SCIENCE']}
      selectedCategories={overrides.selectedCategories ?? new Set()}
      customCategories={overrides.customCategories ?? []}
      onToggle={onToggle}
      onClear={onClear}
      size={overrides.size}
      showLabel={overrides.showLabel}
    />
  )
  return { ...utils, onToggle, onClear }
}

describe('CategoryFilterBar — rendu', () => {
  it('availableCategories vide : ne rend RIEN (retourne null)', () => {
    const { container } = renderBar({ availableCategories: [] })
    expect(container.firstChild).toBeNull()
  })

  it('rend une pastille par catégorie disponible', () => {
    const { container } = renderBar({ availableCategories: ['HISTORY', 'SCIENCE', 'SPORTS'] })
    expect(container.querySelectorAll('.category-filter-pill').length).toBe(3)
  })

  it('affiche le libellé texte à côté de l\'icône par défaut (showLabel non fourni)', () => {
    renderBar({ availableCategories: ['HISTORY'] })
    expect(screen.getByText('Histoire')).toBeInTheDocument()
  })

  it('showLabel=false masque le libellé texte', () => {
    renderBar({ availableCategories: ['HISTORY'], showLabel: false })
    expect(screen.queryByText('Histoire')).not.toBeInTheDocument()
    // La pastille (icône) reste néanmoins présente.
    expect(screen.getByTitle('Histoire')).toBeInTheDocument()
  })

  it('rend aussi les catégories custom valides', () => {
    renderBar({ availableCategories: ['HISTORY', 'CUSTOM_1'], customCategories: CUSTOM_CATEGORIES })
    expect(screen.getByTitle('Custom Un')).toBeInTheDocument()
  })
})

describe('CategoryFilterBar — garde `if (!meta) return null` (cause racine du bug ba960bdf)', () => {
  it('une clé sans meta connue (categoryMeta renvoie null) est silencieusement ignorée — aucune pastille cassée', () => {
    const { container } = renderBar({ availableCategories: ['HISTORY', 'CLE_INCONNUE_XYZ'], customCategories: [] })

    // Seule HISTORY (meta connue) produit une pastille — la clé inconnue
    // n'en produit AUCUNE (ni pastille vide, ni doublon visuel).
    expect(container.querySelectorAll('.category-filter-pill').length).toBe(1)
    expect(screen.getByTitle('Histoire')).toBeInTheDocument()
    expect(screen.queryByText('CLE_INCONNUE_XYZ')).not.toBeInTheDocument()
  })

  it('toutes les clés sans meta connue : le composant ne rend que le conteneur vide (pas de crash)', () => {
    const { container } = renderBar({ availableCategories: ['INCONNUE_A', 'INCONNUE_B'], customCategories: [] })
    expect(container.querySelectorAll('.category-filter-pill').length).toBe(0)
    expect(container.querySelector('.category-filter-bar')).not.toBeNull()
  })
})

describe('CategoryFilterBar — sélection de filtre', () => {
  it('clic sur une pastille appelle onToggle avec sa clé', () => {
    const { onToggle } = renderBar({ availableCategories: ['HISTORY', 'SCIENCE'] })
    fireEvent.click(screen.getByTitle('Histoire'))
    expect(onToggle).toHaveBeenCalledWith('HISTORY')
  })

  it('une catégorie sélectionnée porte la classe "active"', () => {
    renderBar({ availableCategories: ['HISTORY'], selectedCategories: new Set(['HISTORY']) })
    expect(screen.getByTitle('Histoire').className).toMatch(/\bactive\b/)
  })

  it('une catégorie non sélectionnée ne porte pas "active"', () => {
    renderBar({ availableCategories: ['HISTORY'], selectedCategories: new Set() })
    expect(screen.getByTitle('Histoire').className).not.toMatch(/\bactive\b/)
  })

  it('aucun filtre actif : pas de bouton de réinitialisation', () => {
    const { container } = renderBar({ selectedCategories: new Set() })
    expect(container.querySelector('.category-filter-reset')).toBeNull()
  })

  it('au moins un filtre actif : le bouton de réinitialisation apparaît et appelle onClear', () => {
    const { container, onClear } = renderBar({ availableCategories: ['HISTORY'], selectedCategories: new Set(['HISTORY']) })
    const resetBtn = container.querySelector('.category-filter-reset')
    expect(resetBtn).not.toBeNull()
    fireEvent.click(resetBtn)
    expect(onClear).toHaveBeenCalledTimes(1)
  })
})
