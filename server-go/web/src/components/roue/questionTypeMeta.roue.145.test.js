/**
 * #145 (v12.0.0, LOT-1.1) — registre des types côté éditeur : entrée ROUE (contracts/question-types.md §7,
 * contracts/roue.md §2). Fichier distinct : questionTypeMeta.test.js existant reste immuable.
 */
import { describe, it, expect } from 'vitest'
import { QUESTION_TYPES, QUESTION_TYPE_META, GENERABLE_TYPES, getQuestionTypeMeta } from '../../utils/questionTypeMeta'

describe('questionTypeMeta — ROUE (#145)', () => {
  it('ROUE est un type du registre avec libellé et icône', () => {
    const t = QUESTION_TYPES.find(x => x.key === 'ROUE')
    expect(t).toBeTruthy()
    expect(t.label).toBe('Roue')
    expect(t.icon).toBeTruthy()
    expect(QUESTION_TYPE_META.ROUE).toBeTruthy()
    expect(getQuestionTypeMeta('ROUE').label).toBe('Roue')
  })

  it('ROUE n\'est pas imbriquable dans une carte MEMOTION', () => {
    expect(QUESTION_TYPES.find(x => x.key === 'ROUE').nestable).toBe(false)
  })

  it('ROUE n\'est pas générable par IA', () => {
    expect(GENERABLE_TYPES.map(t => t.key)).not.toContain('ROUE')
  })

  it('non-régression : les types existants restent présents', () => {
    const keys = QUESTION_TYPES.map(t => t.key)
    for (const k of ['SPEEDY', 'QCM', 'MEMORY', 'MEMOTION', 'ARDOISE', 'RAFALE', 'ENTRACTE']) expect(keys).toContain(k)
  })
})
