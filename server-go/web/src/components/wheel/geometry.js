// Géométrie pure de la roue générique (contracts/wheel-component.md §3/§4).
// Aucune dépendance DOM : testée seule (geometry.test.js).

/** Nombre de répétitions du ruban en layout `strip` (≥ 2 + ⌈amplitudeMax/n⌉ + 1, amplitude ≤ 4n). */
export const REP = 7
/** Amplitude maximale d'un mouvement, en multiples de n (contrat §4). */
export const MAX_AMPLITUDE_N = 4

/** Position continue (en « segments ») d'un index entier. */
export function indexToOffset(index, n) {
  if (!(n > 0)) return 0
  return offsetToIndex(index, n)
}

/** Ramène toute position continue dans [0, n) (modulo sûr pour les négatifs et les très grandes valeurs). */
export function offsetToIndex(offset, n) {
  if (!(n > 0) || !Number.isFinite(offset)) return 0
  const r = ((offset % n) + n) % n
  return r >= n ? 0 : r
}

/** Distance vers l'avant (sens de lecture, index croissant) de `from` à `to`, dans [0, n). */
export function forwardDistance(from, to, n) {
  return offsetToIndex(to - from, n)
}

/** Angle (degrés) du centre du secteur `index` en layout `arc` : -90° (sommet) + index × 360/n. */
export function angleFor(index, n) {
  return -90 + (index * 360) / n
}

/** Ease-out cubique, identique à la bascule courte CSS (cubic-bezier(.215,.61,.355,1) ≈ easeOutCubic). */
export function easeOutCubic(p) {
  const c = Math.min(1, Math.max(0, p))
  return 1 - Math.pow(1 - c, 3)
}

/**
 * Position à l'instant `t` (horloge serveur, ms) d'un mouvement long {from, to, turns, durationMs, startedAt}.
 * Réservé v12.0 ; `n` (optionnel) convertit `turns` en segments.
 */
export function positionAt(t, motion, n = 0) {
  const { from, to, turns = 0, durationMs, startedAt } = motion
  const p = durationMs > 0 ? (t - startedAt) / durationMs : 1
  const total = (to - from) + turns * n
  return from + total * easeOutCubic(p)
}
