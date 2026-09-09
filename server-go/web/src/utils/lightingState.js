// #207 — état de l'éclairage d'ambiance (contracts/hue-bridge.md §5.6 + §7.1).
//
// Quatre valeurs, jamais fondues : `refused` et `unreachable` commandent des
// gestes correctifs opposés (ré-associer vs rebrancher). Ce module est la
// seule source des libellés pour que le menu (Navbar) et la page Ambiance
// disent la même chose.

export const LIGHTING_STATES = ['ok', 'unreachable', 'refused', 'disabled']

/** Toute valeur inconnue (endpoint absent, `{}`) est traitée comme « non configuré ». */
export function normalizeLightingState(state) {
  return LIGHTING_STATES.includes(state) ? state : 'disabled'
}

/** Libellé du badge de la page (maquette §02). */
export function lightingStateLabel(state) {
  switch (normalizeLightingState(state)) {
    case 'ok': return 'Pont connecté'
    case 'unreachable': return 'Pont injoignable'
    case 'refused': return 'Association refusée'
    default: return 'Non configuré'
  }
}

/** `title` en toutes lettres de l'entrée de menu (contrat §7.1, accessibilité). */
export function lightingStateTitle(state) {
  switch (normalizeLightingState(state)) {
    case 'ok': return 'Éclairage : pont connecté'
    case 'unreachable': return 'Éclairage : pont injoignable'
    case 'refused': return 'Éclairage : association refusée'
    default: return 'Éclairage : non configuré'
  }
}

/**
 * Glyphe du menu (contrat §7.1, maquette §01 rév. 4) — c'est la FORME qui porte
 * l'état, la couleur ne fait que renforcer :
 *   lit   = ampoule pleine avec rayons   (ok)
 *   alert = contour + pastille d'alerte  (unreachable, refused)
 *   off   = contour nu                   (disabled)
 */
export function lightingStateGlyph(state) {
  switch (normalizeLightingState(state)) {
    case 'ok': return 'lit'
    case 'unreachable':
    case 'refused': return 'alert'
    default: return 'off'
  }
}
