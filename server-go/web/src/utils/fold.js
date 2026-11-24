/**
 * fold — repli de comparaison ARDOISE (contrat award-preselect.md §3, #321).
 *
 * Décomposition NFD, suppression des diacritiques, minuscules, espaces de début et de fin retirés,
 * espaces internes compactés en un seul. Même repli que `rafaleAlphaKey` (Go), sans le filtrage des
 * caractères : « Éléphant » = « elephant » = « ÉLÉPHANT » ; « Le Louvre » ≠ « Louvre ».
 *
 * @param {string|null|undefined} s
 * @returns {string}
 */
export function fold(s) {
  if (s === null || s === undefined) return ''
  return String(s)
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .trim()
    .replace(/\s+/g, ' ')
}
