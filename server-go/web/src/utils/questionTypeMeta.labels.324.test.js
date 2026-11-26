import { describe, it, expect } from 'vitest'
import { QUESTION_TYPES, QUESTION_TYPE_META, getQuestionTypeMeta, GENERABLE_TYPES } from './questionTypeMeta'

// LOT-2 (v13.1.0, issue fille #324) — libellés (picto.md §17.13, plan « Renommage d'affichage ») : seul le libellé
// change, JAMAIS un identifiant. Source unique : questionTypeMeta.js (sélecteur de type, cartes, page Jeu, anim, TV).

describe('questionTypeMeta — renommage d’affichage Rafale → Rafale Speedy (#324)', () => {
  it('le type RAFALE s’affiche « Rafale Speedy » avec la même icône 🌀', () => {
    expect(QUESTION_TYPE_META.RAFALE.label).toBe('Rafale Speedy')
    expect(QUESTION_TYPE_META.RAFALE.icon).toBe('🌀')
    expect(getQuestionTypeMeta('RAFALE').label).toBe('Rafale Speedy')
  })

  it('l’identifiant technique reste RAFALE (clé, nestable, règle de points inchangés)', () => {
    const t = QUESTION_TYPES.find((x) => x.key === 'RAFALE')
    expect(t).toBeTruthy()
    expect(t.nestable).toBe(true)
    expect(t.defaultPointsRule).toBe('STARS_PRORATA')
    expect(t.color).toBe('#f97316')
  })

  it('le type PICTO garde son libellé « Picto » et son icône ✏️', () => {
    expect(QUESTION_TYPE_META.PICTO.label).toBe('Picto')
    expect(QUESTION_TYPE_META.PICTO.icon).toBe('✏️')
  })

  it('aucun libellé de type ne contient plus « Rafale » seul (ni « RAFALE » en majuscules d’affichage)', () => {
    for (const t of QUESTION_TYPES) {
      if (/rafale/i.test(t.label)) expect(t.label).toMatch(/^Rafale (Speedy|Picto)$/)
    }
  })

  it('RAFALE reste exclu de la génération IA par types (le Pool Speedy a sa propre modale)', () => {
    expect(GENERABLE_TYPES.map((t) => t.key)).not.toContain('RAFALE')
  })
})
