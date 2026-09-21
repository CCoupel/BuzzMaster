// #230 — état de la sortie audio (contracts/http-endpoints.md §Sound,
// GET /api/sound/status). Jumeau réduit de lightingState.js (#207) : le
// contrat n'y définit que DEUX états, délibérément fusionnés (maquette
// sound-config-230.html rév. 4, §06 « deux états, et c'est tout ») — les
// causes « bruitages désactivés » et « sortie indisponible » sont
// indiscernables pour qui regarde la pastille (aucun son ne sortira dans les
// deux cas). Il n'y a donc pas de normalizeLightingState à quatre valeurs.
//
// #234 — CE module PORTE désormais aussi un glyphe de menu (soundStateGlyph
// ci-dessous, consommé par SoundSpeakerIcon.jsx/Navbar.jsx), à côté de
// l'ampoule Hue de l'entrée « Ambiance ». (Une version antérieure de ce
// commentaire affirmait le contraire — l'entrée « Ambiance » mène désormais
// à une page à deux onglets, Lumière ET Son : cohérent que son icône porte
// les deux états.) La pastille de la §06 ci-dessus, elle, reste où elle
// était : dans l'onglet Son d'AmbiancePage.jsx, jamais dans le bandeau de
// cette page (qui reste dédié à l'éclairage).

/** Toute valeur non strictement `true` est traitée comme « inactif ». */
export function normalizeSoundActive(active) {
  return active === true
}

/** Libellé de la pastille (maquette rév. 4, §06). */
export function soundStateLabel(active) {
  return normalizeSoundActive(active) ? 'Son actif' : 'Son inactif'
}

/** `title` en toutes lettres (#234, accessibilité — même patron que
 * lightingStateTitle). */
export function soundStateTitle(active) {
  return normalizeSoundActive(active) ? 'Son : actif' : 'Son : inactif'
}

/**
 * Glyphe de l'icône de menu (#234, SoundSpeakerIcon.jsx) — DEUX formes
 * seulement, jamais une troisième « alerte » : contrairement à l'éclairage,
 * le son n'a pas d'état de panne distinct à signaler ici (contrat §Sound,
 * GET /api/sound/status fusionne déjà « désactivé » et « indisponible »).
 *   on  = haut-parleur AVEC ondes  (actif)
 *   off = haut-parleur NU, sans ondes, JAMAIS barré (inactif — un choix
 *         normal de l'utilisateur, pas une panne ; même piège que le
 *         contour nu de l'ampoule en #207, voir LightingBulbIcon.jsx)
 */
export function soundStateGlyph(active) {
  return normalizeSoundActive(active) ? 'on' : 'off'
}
