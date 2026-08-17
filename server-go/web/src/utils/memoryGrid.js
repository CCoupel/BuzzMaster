/**
 * memoryGrid — règle de disposition de la grille MEMORY, PARTAGÉE entre
 * `PlayerDisplay.jsx` (`/tv`, vue joueur, aperçu TV de la régie) et
 * `AnimMemoryGrid` (`/anim`) (#159/F0).
 *
 * Extraction PURE depuis `PlayerDisplay.jsx:709-758` — même mélange, même
 * formule, zéro changement de comportement pour les clients existants.
 *
 * MOTIF (huitième mutualisation de la série #149/#155/#158/#165/#166/#170/
 * #171) : un joueur peut annoncer verbalement « la 2e carte en haut à
 * gauche » — l'animateur doit voir EXACTEMENT la même carte à cette
 * position que `/tv` et l'écran du joueur. Si cette règle était réimplémentée
 * « à l'identique » au lieu d'être extraite, la moindre évolution future
 * d'une des deux copies casserait silencieusement cette correspondance :
 * aucune erreur, aucun test rouge — juste un animateur et un joueur qui ne
 * parlent plus de la même carte en pleine partie.
 *
 * Le nombre de colonnes ne dépend JAMAIS de la largeur de l'écran — seule
 * la TAILLE des cartes s'adapte (CSS, container queries). Si la hauteur
 * manque côté `/anim`, c'est le bloc central de la conduite qui défile
 * (#171) — la disposition, elle, ne change jamais.
 */

/**
 * Construit la liste mélangée des cartes MEMORY d'une question, ensemencée
 * par l'identifiant de la question (déterministe — même ordre sur tous les
 * clients, sans que le serveur ait à le transmettre).
 *
 * @param {Object|null} question - gameState.question (ou équivalent), lit
 *   `.MEMORY_PAIRS` et `.ID` — extraction verbatim de PlayerDisplay.jsx, qui
 *   lisait déjà `gameState.question?.MEMORY_PAIRS`/`?.ID` de cette façon.
 * @returns {Array<{id: string, pairId: string|number, card: Object, cardIndex: 1|2}>}
 */
export function buildMemoryCards(question) {
  const pairs = question?.MEMORY_PAIRS || []
  if (pairs.length === 0) return []

  const allCards = []
  pairs.forEach((pair) => {
    allCards.push({
      id: `${pair.ID}-1`,
      pairId: pair.ID,
      card: pair.CARD1,
      cardIndex: 1,
    })
    allCards.push({
      id: `${pair.ID}-2`,
      pairId: pair.ID,
      card: pair.CARD2,
      cardIndex: 2,
    })
  })

  // Fisher-Yates ensemencé par l'identifiant de question — garantit un
  // mélange identique pour la même question sur tous les clients.
  const questionId = question?.ID || '0'
  let seed = parseInt(questionId, 10) || 1
  const shuffled = [...allCards]
  for (let i = shuffled.length - 1; i > 0; i--) {
    seed = (seed * 1103515245 + 12345) & 0x7fffffff
    const j = seed % (i + 1)
    ;[shuffled[i], shuffled[j]] = [shuffled[j], shuffled[i]]
  }
  return shuffled
}

/**
 * Nombre de colonnes — formule FIXE sur le nombre de cartes, jamais sur la
 * largeur de l'écran (c'est ce qui rend la correspondance possible).
 *
 * @param {number} cardCount
 * @returns {number}
 */
export function getMemoryGridCols(cardCount) {
  if (cardCount <= 4) return 2   // 2x2
  if (cardCount <= 6) return 3   // 2x3
  if (cardCount <= 16) return 4  // 4x4 max
  if (cardCount <= 20) return 5  // 4x5
  return 6                       // 4x6
}

/**
 * Nombre de rangées, déduit du nombre de cartes et de colonnes.
 *
 * @param {number} cardCount
 * @param {number} cols
 * @returns {number}
 */
export function getMemoryGridRows(cardCount, cols) {
  return Math.ceil(cardCount / cols)
}
