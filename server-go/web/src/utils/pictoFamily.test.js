import { describe, it, expect } from 'vitest'
import { isPictoFamily, isRafalePicto } from './pictoFamily'

// #318 (v13.1.0, LOT-3 T18) — cas nommés à l'identique du Go (game.IsPictoFamily, picto.md §17.1).
describe('isPictoFamily', () => {
  it('isPictoFamily_picto', () => expect(isPictoFamily('PICTO')).toBe(true))
  it('isPictoFamily_rafale_picto', () => expect(isPictoFamily('RAFALE_PICTO')).toBe(true))
  it('isPictoFamily_rafale', () => expect(isPictoFamily('RAFALE')).toBe(false))
  it('isPictoFamily_speedy', () => expect(isPictoFamily('SPEEDY')).toBe(false))
  it('isPictoFamily_unknown', () => expect(isPictoFamily('NIMPORTE')).toBe(false))
  it('isPictoFamily_empty', () => { expect(isPictoFamily('')).toBe(false); expect(isPictoFamily(undefined)).toBe(false); expect(isPictoFamily(null)).toBe(false) })
  it('isPictoFamily_case_sensitive', () => expect(isPictoFamily('picto')).toBe(false))
})
describe('isRafalePicto', () => {
  it('seul RAFALE_PICTO', () => {
    expect(isRafalePicto('RAFALE_PICTO')).toBe(true)
    expect(isRafalePicto('PICTO')).toBe(false)
    expect(isRafalePicto('RAFALE')).toBe(false)
  })
})
