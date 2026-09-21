// #230 — état de la sortie audio (contracts/http-endpoints.md §Sound,
// GET /api/sound/status). Jumeau réduit de lightingState.js (#207) : le
// contrat n'y définit que DEUX états, délibérément fusionnés (maquette
// sound-config-230.html rév. 4, §06 « deux états, et c'est tout ») — les
// causes « bruitages désactivés » et « sortie indisponible » sont
// indiscernables pour qui regarde la pastille (aucun son ne sortira dans les
// deux cas). Il n'y a donc ni normalizeLightingState à quatre valeurs, ni
// glyphe de menu : ce badge ne vit que dans l'onglet Son (le bandeau reste
// dédié à l'éclairage, cf. AmbiancePage.jsx).

/** Toute valeur non strictement `true` est traitée comme « inactif ». */
export function normalizeSoundActive(active) {
  return active === true
}

/** Libellé de la pastille (maquette rév. 4, §06). */
export function soundStateLabel(active) {
  return normalizeSoundActive(active) ? 'Son actif' : 'Son inactif'
}
