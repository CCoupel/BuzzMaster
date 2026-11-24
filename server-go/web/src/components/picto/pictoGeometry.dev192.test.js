import { describe, it, expect } from 'vitest'
import { normalizePoint, normalizeCoord, lineWidthPx, keepPoint, PICTO_LINE_WIDTH } from './pictoGeometry'

// Cas nommés à l'identique du côté Go (contrat picto.md §8.2) : mêmes valeurs attendues.
describe('pictoGeometry — cas nommés du contrat §8.2', () => {
  it('normalize_center', () => expect(normalizePoint(150, 150, 300, 300)).toEqual([5000, 5000]))
  it('normalize_corners', () => {
    expect(normalizePoint(0, 0, 300, 300)).toEqual([0, 0])
    expect(normalizePoint(300, 300, 300, 300)).toEqual([10000, 10000])
  })
  it('clamp_outside', () => {
    expect(normalizePoint(-20, 400, 300, 300)).toEqual([0, 10000])
  })
  it('round_half', () => {
    // 1 / 20000 de la largeur = 0,5 unité → arrondi vers le haut ; 3 / 20000 = 1,5 → 2
    expect(normalizeCoord(0.5, 10000)).toBe(1)
    expect(normalizeCoord(1.5, 10000)).toBe(2)
  })
  it('line_width_units', () => {
    expect(PICTO_LINE_WIDTH).toBe(80)
    expect(lineWidthPx(1000)).toBe(8)
  })
  it('single_point_dot : un point seul est gardé (rien avant lui)', () => {
    expect(keepPoint(null, [10, 10])).toBe(true)
  })
  it('simplification : un point à moins de 15 unités du précédent est ignoré', () => {
    expect(keepPoint([100, 100], [110, 100])).toBe(false)
    expect(keepPoint([100, 100], [115, 100])).toBe(true)
  })
})
