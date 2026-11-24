// @vitest-environment node
/**
 * #193 (v13.0.0, LOT-2.1) — pictoGeometry : contrat de normalisation de la feuille (contracts/picto.md §8.2).
 * Les 6 cas nommés ci-dessous portent le MÊME nom et les MÊMES valeurs attendues que dans
 * `internal/server/picto_geometry_test.go` (ou `internal/game/…`) : c'est l'antidote à R5 (dérive Go/JS).
 *   normalize_center · normalize_corners · clamp_outside · round_half · single_point_dot · line_width_units
 */
import { describe, it, expect, vi } from 'vitest'
import {
  PICTO_COORD_MAX, PICTO_LINE_WIDTH, PICTO_MIN_STEP, PICTO_MAX_POINTS_PER_FRAME, PICTO_MAX_STROKES_PER_FRAME, PICTO_FRAME_MS,
  normalizePoint, normalizeCoord, lineWidthPx, denormalizeCoord, keepPoint,
} from './pictoGeometry'
import { drawStrokes } from './PictoCanvasView'

describe('pictoGeometry — cas nommés du contrat §8.2 (mêmes valeurs qu\'en Go)', () => {
  it('normalize_center : le centre d\'une feuille 1000×1000 vaut [5000, 5000]', () => {
    expect(normalizePoint(500, 500, 1000, 1000)).toEqual([5000, 5000])
  })

  it('normalize_corners : les quatre coins valent 0 et 10 000', () => {
    expect(normalizePoint(0, 0, 1000, 1000)).toEqual([0, 0])
    expect(normalizePoint(1000, 0, 1000, 1000)).toEqual([10000, 0])
    expect(normalizePoint(0, 1000, 1000, 1000)).toEqual([0, 10000])
    expect(normalizePoint(1000, 1000, 1000, 1000)).toEqual([10000, 10000])
  })

  it('clamp_outside : un point hors feuille est borné à [0, 10 000]', () => {
    expect(normalizePoint(-50, 1200, 1000, 1000)).toEqual([0, 10000])
    expect(normalizePoint(5000, -1, 1000, 1000)).toEqual([10000, 0])
  })

  it('round_half : arrondi à l\'entier le plus proche, la moitié vers le haut', () => {
    expect(normalizeCoord(1, 20000)).toBe(1) // 0,5 → 1
    expect(normalizeCoord(3, 20000)).toBe(2) // 1,5 → 2
    expect(normalizeCoord(1, 3)).toBe(3333) // 3333,33 → 3333
    expect(normalizeCoord(2, 3)).toBe(6667) // 6666,67 → 6667
  })

  it('single_point_dot : un trait d\'un seul point est un disque du diamètre du trait', () => {
    const ctx = {
      clearRect: vi.fn(), fillRect: vi.fn(), beginPath: vi.fn(), moveTo: vi.fn(), lineTo: vi.fn(), stroke: vi.fn(),
      fill: vi.fn(), arc: vi.fn(),
    }
    drawStrokes(ctx, [{ ID: 1, PTS: [[5000, 2500]], END: true }], 1000)
    expect(ctx.arc).toHaveBeenCalledTimes(1)
    const [cx, cy, r] = ctx.arc.mock.calls[0]
    expect([cx, cy]).toEqual([500, 250])
    expect(r * 2).toBeCloseTo(8) // diamètre = 80 unités sur 1000 px = 8 px
    expect(ctx.stroke).not.toHaveBeenCalled() // pas de segment pour un point seul
  })

  it('line_width_units : le trait fait 80 unités (0,8 % du côté), fixe', () => {
    expect(PICTO_LINE_WIDTH).toBe(80)
    expect(lineWidthPx(1000)).toBe(8)
    expect(lineWidthPx(500)).toBe(4)
    expect(denormalizeCoord(PICTO_COORD_MAX, 640)).toBe(640)
  })
})

describe('pictoGeometry — constantes de cadence et simplification (§8.5)', () => {
  it('une trame : au plus une toutes les 60 ms, 200 points, 20 traits', () => {
    expect(PICTO_FRAME_MS).toBe(60)
    expect(PICTO_MAX_POINTS_PER_FRAME).toBe(200)
    expect(PICTO_MAX_STROKES_PER_FRAME).toBe(20)
  })

  it('un point n\'est gardé que s\'il est à au moins 15 unités du précédent', () => {
    expect(PICTO_MIN_STEP).toBe(15)
    expect(keepPoint([0, 0], [14, 0])).toBe(false)
    expect(keepPoint([0, 0], [15, 0])).toBe(true)
    expect(keepPoint([0, 0], [10, 10])).toBe(false) // 14,14 < 15
    expect(keepPoint([0, 0], [11, 11])).toBe(true) // 15,55 ≥ 15
    expect(keepPoint(null, [0, 0])).toBe(true) // premier point
  })

  it('une feuille de taille nulle ne produit ni NaN ni valeur hors bornes', () => {
    expect(normalizePoint(10, 10, 0, 0)).toEqual([0, 0])
  })
})
