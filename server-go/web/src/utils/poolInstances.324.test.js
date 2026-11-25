import { describe, it, expect } from 'vitest'
import { POOL_INSTANCES, getPoolInstance, poolUrls } from './poolInstances'

// #324 (v13.1.0, LOT-2 T13 générique) — paramétrage par instance, contrat picto-pool.md §1, §4, §6.
describe('poolInstances — chemins HTTP', () => {
  it('rafale : chemins inchangés', () => {
    const u = poolUrls(POOL_INSTANCES.rafale)
    expect(u.list).toBe('/api/rafale/questions')
    expect(u.item('r 1')).toBe('/api/rafale/questions/r%201')
    expect(u.reset('r-1')).toBe('/api/rafale/questions/r-1/reset')
    expect(u.resetAll).toBe('/api/rafale/questions/reset-all')
    expect(u.pool).toBe('/api/rafale/pool')
    expect(u.generate).toBe('/api/rafale/generate-questions')
  })
  it('picto : /api/picto/words*, /pool, /generate-words, /words/batch', () => {
    const u = poolUrls(POOL_INSTANCES.picto)
    expect(u.list).toBe('/api/picto/words')
    expect(u.item('p-001')).toBe('/api/picto/words/p-001')
    expect(u.reset('p-001')).toBe('/api/picto/words/p-001/reset')
    expect(u.resetAll).toBe('/api/picto/words/reset-all')
    expect(u.batch).toBe('/api/picto/words/batch')
    expect(u.pool).toBe('/api/picto/pool')
    expect(u.generate).toBe('/api/picto/generate-words')
  })
})

describe('poolInstances — paramètres', () => {
  it('rafale : réponse, plafonds 100/40, paliers 10..200, pas de collage, TARGET RAFALE', () => {
    const i = POOL_INSTANCES.rafale
    expect([i.hasAnswer, i.maxEntryRunes, i.maxAnswerRunes, i.pasteList, i.target]).toEqual([true, 100, 40, false, 'RAFALE'])
    expect(i.generationPresets).toEqual([10, 20, 50, 100, 200])
  })
  it('picto : pas de réponse, mot ≤ 40, paliers 10/20/50, collage, TARGET PICTO', () => {
    const i = POOL_INSTANCES.picto
    expect([i.hasAnswer, i.maxEntryRunes, i.pasteList, i.target]).toEqual([false, 40, true, 'PICTO'])
    expect(i.generationPresets).toEqual([10, 20, 50])
    expect(i.labels.entryColumn).toBe('Mot')
  })
  it('clé absente ou inconnue : retombe sur rafale', () => {
    expect(getPoolInstance(undefined)).toBe(POOL_INSTANCES.rafale)
    expect(getPoolInstance('nimporte')).toBe(POOL_INSTANCES.rafale)
  })
})
