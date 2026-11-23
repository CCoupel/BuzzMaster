/**
 * pictoGeometry — contrat de normalisation de la feuille PICTO (contrat picto.md §8.2).
 *
 * Implémenté une fois en Go (`picto_geometry.go`), une fois ici ; mêmes valeurs attendues des deux côtés
 * (cas de test nommés : normalize_center, normalize_corners, clamp_outside, round_half, single_point_dot,
 * line_width_units). La feuille est carrée ; un point est `[x, y]` en entiers de 0 à 10 000, origine en haut à gauche.
 */

export const PICTO_COORD_MAX = 10000
export const PICTO_LINE_WIDTH = 80 // unités (0,8 % du côté), extrémités et jonctions arrondies
export const PICTO_MIN_STEP = 15 // un point n'est gardé que s'il est à au moins 15 unités du précédent
export const PICTO_MAX_POINTS_PER_FRAME = 200
export const PICTO_MAX_STROKES_PER_FRAME = 20
export const PICTO_FRAME_MS = 60

const clamp = (v) => Math.min(PICTO_COORD_MAX, Math.max(0, v))

/** Une coordonnée en pixels → unités : `round(px / taille × 10000)`, bornée à [0, 10000]. */
export function normalizeCoord(px, size) {
  if (!(size > 0)) return 0
  // px × 10000 / taille (multiplier d'abord) : 1,5 / 10000 × 10000 vaut 1,4999… en flottant et casserait round_half
  return clamp(Math.round((px * PICTO_COORD_MAX) / size)) || 0
}

/** Un point en pixels → `[x, y]` en unités. */
export function normalizePoint(px, py, width, height) {
  return [normalizeCoord(px, width), normalizeCoord(py, height)]
}

/** Unités → pixels pour une feuille de côté `size`. */
export function denormalizeCoord(u, size) {
  return (u / PICTO_COORD_MAX) * size
}

/** Épaisseur du trait en pixels pour une feuille de côté `size`. */
export function lineWidthPx(size) {
  return denormalizeCoord(PICTO_LINE_WIDTH, size)
}

/** Distance euclidienne entre deux points en unités. */
export function distance(a, b) {
  return Math.hypot(a[0] - b[0], a[1] - b[1])
}

/** Garde-t-on `p` après `last` (simplification à 15 unités minimum) ? */
export function keepPoint(last, p) {
  return !last || distance(last, p) >= PICTO_MIN_STEP
}
