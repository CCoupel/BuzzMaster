/**
 * typeState — accesseur unique de l'état vivant par type de contenu
 * (#184/B-F1), en fonction de l'hôte courant (`resolveHostContext`).
 *
 * Les champs question-scopés existants (`qcmInvalidated`, `ARDOISE_ANSWERS`,
 * `MEMORY_FLIPPED_CARDS`…) restent **inchangés** pour l'hôte question — ce
 * fichier ne les renomme ni ne les déplace. `contracts/question-types.md`
 * §5.3 : la même notion (ex. les réponses QCM invalidées) existe à deux
 * emplacements selon l'hôte — champs plats de `gameState` pour l'hôte
 * question, `gameState.MEMOTION_ACTIVE.STATE` pour l'hôte carte MEMOTION.
 * **C'est le seul point du code qui doit connaître cette duplication** :
 * tout composant de type appelle `getTypeState`, jamais `gameState.qcmInvalidated`
 * ni `gameState.MEMOTION_ACTIVE` directement.
 *
 * ⚠️ **Périmètre v7.1.0 (#187)** : `qcmInvalidated` (v7.0.0) et `memory`
 * (v7.1.0) sont exposés ici — les deux seuls champs d'état par type
 * consommés aujourd'hui (`AnimQcmOptions`/`AnimMemoryGrid`, `PlayerDisplay`).
 * `ARDOISE_ANSWERS` reste lu directement à son emplacement question-scopé
 * existant : ARDOISE n'est **pas** nestable en carte (#186 fermée « not
 * planned », contrat §7.2) — l'y ajouter serait l'abstraction spéculative que
 * #184 écarte explicitement.
 *
 * `gameState.MEMOTION_ACTIVE` est câblé côté `useWebSocket.js` depuis #184/
 * #185 (v7.0.0) — la branche « hôte carte » ci-dessous est donc active dès
 * qu'une carte MEMOTION QCM ou MEMORY est en jeu.
 */

/**
 * @typedef {Object} MemoryTypeState
 * @property {string[]} flippedCards - IDs des cartes actuellement retournées (max 2)
 * @property {number[]} matchedPairs - IDs des paires trouvées (permanent)
 * @property {number} errors - nombre d'erreurs (tentatives ratées)
 */

/**
 * @param {Object} gameState
 * @param {import('./hostContext').HostContext} hostContext
 * @returns {{qcmInvalidated: string[], memory: MemoryTypeState}}
 */
export function getTypeState(gameState, hostContext) {
  if (hostContext?.cardId) {
    const active = gameState?.MEMOTION_ACTIVE
    // Garde-fou : ne servir l'état carte que s'il correspond bien à la carte
    // active courante (évite une frame où STATE porterait encore les
    // données de la carte précédente pendant une transition).
    const state = (active && active.CARD_ID === hostContext.cardId) ? (active.STATE || {}) : {}
    return {
      qcmInvalidated: state.QCM_INVALIDATED || [],
      memory: {
        flippedCards: state.MEMORY_FLIPPED_CARDS || [],
        matchedPairs: state.MEMORY_MATCHED_PAIRS || [],
        errors: state.MEMORY_ERRORS || 0,
      },
    }
  }

  return {
    qcmInvalidated: gameState?.qcmInvalidated || [],
    memory: {
      flippedCards: gameState?.memoryFlippedCards || [],
      matchedPairs: gameState?.memoryMatchedPairs || [],
      errors: gameState?.memoryErrors || 0,
    },
  }
}
