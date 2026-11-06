import { describe, it, expect } from 'vitest'
import { categoryMeta } from './categoryUtils'
import { getCategoryColor, DEFAULT_NEON_COLOR } from '../constants/colors'

// #285 (v11.4.0, LOT-4) — résolution API-first (contracts/http-endpoints.md §Catégories « Frontend ») :
// nom/couleur/image viennent de GET /api/categories ; la table JS ne fournit que l'emoji des intégrées et un repli
// hors ligne. `color: ""` (= aucune) ⇒ gris #6b7280 à l'affichage.

const API = [
  { key: 'GEOGRAPHY', name: 'Géo du monde', imageURL: '', color: '#112233', isCustom: false },
  { key: 'HISTORY', name: 'Histoire', imageURL: '', color: '', isCustom: false },
  { key: 'CINEMA', name: 'Cinéma', imageURL: '/files/categories/CINEMA.png', color: '#0ea5e9', isCustom: true },
  { key: 'NUE', name: 'Nue', imageURL: '/files/categories/NUE.webp', color: '', isCustom: true },
]

describe('categoryMeta — API-first (#285)', () => {
  it('intégrée renommée + recolorée par le serveur : nom et couleur de l\'API, emoji de la table', () => {
    const m = categoryMeta('GEOGRAPHY', API)
    expect(m.label).toBe('Géo du monde')
    expect(m.color).toBe('#112233')
    expect(m.icon).toBe('🌍')
    expect(m.isCustom).toBe(false)
    expect(m.imageURL).toBeFalsy()
  })

  it('intégrée sans surcharge connue de l\'API : repli sur la table (hors ligne)', () => {
    const m = categoryMeta('SCIENCE', [])
    expect(m.label).toBe('Sciences & Nature')
    expect(m.color).toBe('#22c55e')
    expect(m.icon).toBe('🔬')
  })

  it('intégrée avec color "" côté API (aucune) : badge gris', () => {
    expect(categoryMeta('HISTORY', API).color).toBe('#6b7280')
  })

  it('personnalisée colorée : sa couleur, son nom, son image', () => {
    const m = categoryMeta('CINEMA', API)
    expect(m).toMatchObject({ label: 'Cinéma', color: '#0ea5e9', isCustom: true, imageURL: '/files/categories/CINEMA.png' })
  })

  it('personnalisée sans couleur ("") : gris #6b7280 (repli d\'affichage, retiré de l\'API)', () => {
    expect(categoryMeta('NUE', API).color).toBe('#6b7280')
  })

  it('clé inconnue : null', () => {
    expect(categoryMeta('INCONNUE', API)).toBeNull()
  })
})

describe('getCategoryColor — néon TV avec la liste API (#285)', () => {
  it('intégrée recolorée : la couleur de l\'API prime sur la table', () => {
    expect(getCategoryColor('GEOGRAPHY', API)).toBe('#112233')
  })
  it('personnalisée colorée : sa couleur', () => {
    expect(getCategoryColor('CINEMA', API)).toBe('#0ea5e9')
  })
  it('personnalisée sans couleur ("") ou catégorie absente : couleur neutre par défaut', () => {
    expect(getCategoryColor('NUE', API)).toBe(DEFAULT_NEON_COLOR)
    expect(getCategoryColor('', API)).toBe(DEFAULT_NEON_COLOR)
    expect(getCategoryColor(undefined, API)).toBe(DEFAULT_NEON_COLOR)
  })
  it('sans liste API : retombe sur la table (rétro-compatibilité des appels à un argument)', () => {
    expect(getCategoryColor('HISTORY')).toBe('#eab308')
    expect(getCategoryColor('HISTORY', [])).toBe('#eab308')
  })
})
