/**
 * motionGrid — règle de disposition de la grille MEMOTION, PARTAGÉE entre
 * `PlayerDisplay.jsx` (`/tv`, vue joueur, aperçu TV de la régie),
 * `GamePage.jsx` (`/admin`, panneau MEMOTION en mode Secret) et
 * `AnimMotionGrid` (`/anim`) (#160/F0).
 *
 * Extraction PURE depuis `PlayerDisplay.jsx:1985-1996` (dupliquée à
 * l'identique dans `GamePage.jsx:814-837`) — même formules, zéro changement
 * de comportement pour les clients existants.
 *
 * MOTIF (même leçon que `memoryGrid.js`, #159/F0 — la neuvième mutualisation
 * de la série #149/#155/#158/#159/#165/#166/#170/#171) : un joueur peut
 * annoncer verbalement « la carte A2 » (mode Secret) — l'animateur doit voir
 * EXACTEMENT la même carte à cette position que `/tv`. Si cette règle était
 * réimplémentée « à l'identique » au lieu d'être extraite, la moindre
 * évolution future d'une des copies casserait silencieusement cette
 * correspondance : aucune erreur, aucun test rouge — juste un animateur et
 * un joueur qui ne parlent plus de la même carte en pleine partie.
 *
 * ⚠️ La formule de colonnes MEMOTION DIFFÈRE de celle de MEMORY
 * (`memoryGrid.js` : 2/3/4/5/6 sur 4/6/16/20 cartes). Ne pas fusionner les
 * deux utilitaires, ne pas réutiliser l'un pour l'autre.
 *
 * Le nombre de colonnes ne dépend JAMAIS de la largeur de l'écran — seule
 * la TAILLE des cartes s'adapte (CSS, container queries).
 */

/**
 * Nombre de colonnes — formule FIXE sur le nombre de cartes, jamais sur la
 * largeur de l'écran (c'est ce qui rend la correspondance possible).
 *
 * Source : `PlayerDisplay.jsx:2000` (et copie identique `GamePage.jsx:836`).
 *
 * @param {number} count
 * @returns {number}
 */
export function getMotionGridCols(count) {
  if (count <= 4) return 2
  if (count <= 6) return 3
  if (count <= 12) return 4
  return 5
}

/**
 * Nombre de rangées, déduit du nombre de cartes et de colonnes.
 *
 * Source : `PlayerDisplay.jsx:2001`.
 *
 * @param {number} count
 * @returns {number}
 */
export function getMotionGridRows(count) {
  const cols = getMotionGridCols(count)
  return Math.ceil(count / cols)
}

/**
 * Coordonnée d'une carte (mode Secret) — lettre de ligne (A, B, C...) +
 * numéro de colonne (1-based), dérivée de la position dans le tableau
 * `MOTION_CARDS` tel qu'envoyé par le serveur (aucun mélange côté client,
 * contrairement à MEMORY).
 *
 * Source : `PlayerDisplay.jsx:1996` (et copie identique `GamePage.jsx:837`).
 *
 * @param {number} index - position de la carte dans `MOTION_CARDS`
 * @param {number} cols - `getMotionGridCols(count)`
 * @returns {string}
 */
export function getMotionCardCoord(index, cols) {
  return `${String.fromCharCode(65 + Math.floor(index / cols))}${(index % cols) + 1}`
}

/**
 * Barème de points d'une carte selon sa difficulté (1-3 étoiles), avec repli
 * 1/3/5 si `MOTION_CONFIG` est absent (question ancienne, pas encore migrée).
 *
 * Source : `PlayerDisplay.jsx:1985-1992` (et copie identique
 * `GamePage.jsx:814-821`).
 *
 * @param {number} difficulty - `card.DIFFICULTY` (1, 2 ou 3)
 * @param {{POINTS_1_STAR?: number, POINTS_2_STAR?: number, POINTS_3_STAR?: number}|null|undefined} motionConfig - `question.MOTION_CONFIG`
 * @returns {number}
 */
export function getMotionCardPoints(difficulty, motionConfig) {
  if (motionConfig) {
    if (difficulty === 1) return motionConfig.POINTS_1_STAR ?? 1
    if (difficulty === 2) return motionConfig.POINTS_2_STAR ?? 3
    if (difficulty === 3) return motionConfig.POINTS_3_STAR ?? 5
  }
  return difficulty === 3 ? 5 : difficulty === 2 ? 3 : 1
}

/**
 * Mode Secret — la grille affiche des coordonnées plutôt que les thèmes tant
 * que la manche n'est pas terminée pour une carte (aucun étoile visible non
 * plus, la difficulté trahirait la carte).
 *
 * Source : `PlayerDisplay.jsx:1995` (et copie identique `GamePage.jsx:834`).
 *
 * @param {{MOTION_MEMORIZE_DURATION?: number}|null|undefined} question - gameState.question
 * @returns {boolean}
 */
export function isMotionSecretMode(question) {
  return (question?.MOTION_MEMORIZE_DURATION || 0) > 0
}
