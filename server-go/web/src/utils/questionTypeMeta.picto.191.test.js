// @vitest-environment node
/**
 * PICTO (v13.0.0, #191) — contracts/picto.md §4 : entrée de `questionTypeMeta.js`
 * `{ key: 'PICTO', label: 'Picto', icon: '✏️', nestable: false }`, et type non
 * générable par l'IA (génération reportée en v13.1.0).
 */
import { describe, it, expect } from 'vitest'
import {
  QUESTION_TYPES,
  QUESTION_TYPE_META,
  GENERABLE_TYPES,
  getQuestionTypeMeta,
} from './questionTypeMeta'

describe('questionTypeMeta — type PICTO (#191)', () => {
  it('PICTO est dans QUESTION_TYPES, une seule fois, libellé « Picto » et icône ✏️', () => {
    const entries = QUESTION_TYPES.filter(t => t.key === 'PICTO')
    expect(entries).toHaveLength(1)
    expect(entries[0].label).toBe('Picto')
    expect(entries[0].icon).toBe('✏️')
  })

  it('PICTO n’est pas imbriquable dans une carte MEMOTION', () => {
    expect(QUESTION_TYPES.find(t => t.key === 'PICTO').nestable).toBe(false)
    expect(QUESTION_TYPE_META.PICTO.nestable).toBe(false)
  })

  it('PICTO porte une couleur (comme tous les types)', () => {
    expect(QUESTION_TYPES.find(t => t.key === 'PICTO').color).toMatch(/^#[0-9a-fA-F]{3,8}$/)
  })

  it('PICTO n’est pas dans GENERABLE_TYPES (pas de génération IA en v13.0.0)', () => {
    expect(GENERABLE_TYPES.some(t => t.key === 'PICTO')).toBe(false)
  })

  it('getQuestionTypeMeta("PICTO") renvoie sa propre meta, pas le repli « Type inconnu » ni SPEEDY', () => {
    const meta = getQuestionTypeMeta('PICTO')
    expect(meta.key).toBe('PICTO')
    expect(meta.label).toBe('Picto')
  })

  it('le nom produit ne contient jamais « Pictionary » ni « PICSONARY »', () => {
    const json = JSON.stringify(QUESTION_TYPES)
    expect(json).not.toMatch(/pictionary|picsonary/i)
  })
})
