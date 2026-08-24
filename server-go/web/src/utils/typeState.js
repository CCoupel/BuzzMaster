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
 * ⚠️ **Périmètre v7.0.0** : seul `qcmInvalidated` est exposé ici, seul champ
 * d'état par type consommé aujourd'hui (`AnimQcmOptions`, `PlayerDisplay`).
 * `ARDOISE_ANSWERS`/`MEMORY_FLIPPED_CARDS` restent lus directement à leur
 * emplacement question-scopé existant tant qu'ARDOISE et MEMORY ne sont pas
 * nestables en carte (`NestableInMotionCard`, v7.1.0, #186/#187, contrat §7)
 * — les ajouter ici maintenant serait l'abstraction spéculative que #184
 * écarte explicitement.
 *
 * ⚠️ **Dépendance non bouclée signalée** : `gameState.MEMOTION_ACTIVE` est un
 * nouveau champ `GameState` (contrat §5.2, tâche `dev-backend` B-B4) — au
 * moment d'écrire ce fichier, il n'est pas encore émis par le serveur ni
 * répercuté dans `useWebSocket.js` (aucune des deux tâches n'apparaît dans
 * B-F1-B-F5 ni dans B-B1-B-B8). La branche « hôte carte » ci-dessous est
 * donc correcte par construction mais **inerte** tant que ce champ n'est pas
 * câblé côté `useWebSocket.js` — sans conséquence en v7.0.0 puisqu'aucune
 * carte QCM n'est encore jouable (#185, Phase 3). À câbler avant C-F1/C-F2.
 */

/**
 * @param {Object} gameState
 * @param {import('./hostContext').HostContext} hostContext
 * @returns {{qcmInvalidated: string[]}}
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
    }
  }

  return {
    qcmInvalidated: gameState?.qcmInvalidated || [],
  }
}
