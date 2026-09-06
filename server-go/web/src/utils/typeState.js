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
 * @typedef {Object} RafaleTypeState
 * @property {string} subphase - RAFALE_SUBPHASE de la carte ("" | "QUESTION" | "ROUND_END", contrat rafale.md §14.2)
 * @property {Object} currentQuestion - RAFALE_CURRENT_QUESTION (sans réponse, contrat §4/§14.2) —
 *   `.POINTS` vaut toujours 0 en carte (barème par question sans objet, §14.2) : ne jamais l'afficher pour cet hôte
 * @property {number} questionTime - RAFALE_QUESTION_TIME (décompte live de la question en cours, §14.2)
 * @property {number} askedCount - RAFALE_ASKED_COUNT
 * @property {number} correctCount - RAFALE_CORRECT_COUNT ("Units" du barème STARS_PRORATA, §14.4)
 * @property {number} poolRemaining - RAFALE_POOL_REMAINING
 * @property {boolean} exhausted - RAFALE_EXHAUSTED
 */

/**
 * @param {Object} gameState
 * @param {import('./hostContext').HostContext} hostContext
 * @returns {{qcmInvalidated: string[], memory: MemoryTypeState, rafale: RafaleTypeState}}
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
      // RAFALE (#217, milestone v9.0.0, contrat rafale.md §14.2) — mini-manche
      // jouable comme carte MEMOTION, mode SOLO forcé (217-Q3). Mêmes clés que
      // les champs GameState globaux de la manche RAFALE classique
      // (RAFALE_SUBPHASE, RAFALE_CURRENT_QUESTION, RAFALE_QUESTION_TIME,
      // RAFALE_ASKED_COUNT, RAFALE_POOL_REMAINING, RAFALE_EXHAUSTED),
      // désormais scopées à la carte via MOTION_ACTIVE.STATE plutôt que
      // globales, PLUS RAFALE_CORRECT_COUNT (nouveau, "Units" du barème
      // STARS_PRORATA, §14.4 — sans équivalent question-scopé, la manche
      // classique n'a pas de barème dérivé). PAS de champs équipe
      // (RAFALE_MODE/TEAM_*/CURRENT_TEAM/PARTICIPATING_TEAMS/
      // CURRENT_TEAM_COLOR) : l'équipe qui joue la carte est déjà portée par
      // MEMOTION_CURRENT_TEAM/MEMOTION_CARD_TEAMS (mode SOLO, une seule
      // équipe par carte, pas de rotation à suivre séparément). PAS de
      // pré-tirage NEXT (#202, §13) : une carte tire à la demande, §14.2.
      rafale: {
        subphase: state.RAFALE_SUBPHASE || '',
        currentQuestion: state.RAFALE_CURRENT_QUESTION || {},
        questionTime: state.RAFALE_QUESTION_TIME || 0,
        askedCount: state.RAFALE_ASKED_COUNT || 0,
        correctCount: state.RAFALE_CORRECT_COUNT || 0,
        poolRemaining: state.RAFALE_POOL_REMAINING ?? 0,
        exhausted: !!state.RAFALE_EXHAUSTED,
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
    // Hôte question — sans objet, une manche RAFALE classique (hôte
    // question, jamais carte) est déjà lue directement depuis les champs
    // GameState globaux existants (GamePage.jsx/PlayerDisplay.jsx/AnimPage.jsx,
    // inchangés par #217) ; ce repli n'est ici que pour la forme du type de
    // retour, jamais consommé côté hôte question.
    rafale: { subphase: '', currentQuestion: {}, questionTime: 0, askedCount: 0, correctCount: 0, poolRemaining: 0, exhausted: false },
  }
}
