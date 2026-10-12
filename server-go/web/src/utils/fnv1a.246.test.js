import { describe, it, expect } from 'vitest'
import * as mod from './fnv1a'

// #246 (v11.2) — empreinte FNV-1a 32 bits du texte encodé en UTF-8, 8 hexadécimaux minuscules.
// Contrat : contracts/websocket-actions.md §ARDOISE_FINAL (vecteurs de test — Go et JS identiques,
// vérifiés côté Go par TestArdoiseFinal246_HashVectorsMatchTheContract).
const fnv1a = mod.fnv1a ?? mod.default ?? mod.fnv1a32

const VECTORS = [
  ['', '811c9dc5'],
  ['a', 'e40c292c'],
  ['Paris', '246f9d40'],
  ['PARIS EN FRANCE', '066a01e6'],
  ['Équipe à côté', 'c3ff86c2'],
  ['  espaces  ', '1e009bcf'],
]

describe('fnv1a (#246)', () => {
  it('exporte une fonction', () => {
    expect(typeof fnv1a).toBe('function')
  })
  for (const [text, hash] of VECTORS) {
    it(`${JSON.stringify(text)} → ${hash}`, () => {
      expect(fnv1a(text)).toBe(hash)
    })
  }
  it('hache les octets UTF-8, pas les unités UTF-16 (émoji hors BMP)', () => {
    // 😀 = F0 9F 98 80 ; FNV-1a 32 bits de ces 4 octets
    let h = 0x811c9dc5
    for (const b of [0xf0, 0x9f, 0x98, 0x80]) { h ^= b; h = Math.imul(h, 0x01000193) >>> 0 }
    expect(fnv1a('😀')).toBe(h.toString(16).padStart(8, '0'))
  })
  it('toujours 8 caractères hexadécimaux minuscules', () => {
    expect(fnv1a('x')).toMatch(/^[0-9a-f]{8}$/)
  })
})
