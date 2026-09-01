/**
 * buzzOrder — ordre de buzz mutualisé entre `/admin` (GamePage.jsx) et
 * `/anim` (AnimPage.jsx, #156/F6).
 *
 * Règle reprise de GamePage.jsx (tri équipes par temps de buzz, feature
 * tri-rapidite) et components/TeamCard.jsx (badge de rang, temps de
 * réaction) — mutualisée ici pour que les deux interfaces affichent
 * STRICTEMENT le même ordre et les mêmes temps pendant la même partie,
 * même piège déjà rencontré avec utils/questionOrder.js (#149) et
 * utils/pointsAward.js (#155/F1).
 */

const BUZZ_ORDER_PHASES = ['STARTED', 'PAUSED', 'REVEALED', 'STOPPED']

/**
 * Trie une liste d'équipes par ordre de buzz pendant les phases actives —
 * équipes ayant buzzé (TIME > 0) triées par TIME croissant (plus rapide en
 * haut), équipes non-buzzées à la suite, dans leur ordre courant. Hors de
 * ces phases (STOP idle/PREPARE/READY), la liste est retournée inchangée —
 * le tri persiste jusqu'à la phase PREPARE de la question suivante.
 *
 * @param {Array<{TIME?: number}>} teamsList
 * @param {string} phase - gameState.phase
 * @returns {Array} nouvelle liste (ne mute pas `teamsList`)
 */
export function sortTeamsByBuzzOrder(teamsList, phase) {
  if (!BUZZ_ORDER_PHASES.includes(phase)) return teamsList

  const buzzed = teamsList.filter(t => (t.TIME ?? 0) > 0)
  const nonBuzzed = teamsList.filter(t => (t.TIME ?? 0) === 0)
  buzzed.sort((a, b) => a.TIME - b.TIME)
  return [...buzzed, ...nonBuzzed]
}

/**
 * sortTeamsByRafaleCounter — classement RAFALE (v8.0.0, #16/#199, contrat
 * rafale.md §6.1) : trié par COMPTEUR de manche décroissant, pas par score
 * réel (aucun point n'est attribué avant la fin de manche, §6.1). En mode
 * `MAILLON_FAIBLE`, le compteur courant retombe à 0 sur mauvaise réponse
 * (§3.4) — le classement suit donc le MEILLEUR compteur atteint
 * (RAFALE_TEAM_BEST) plutôt que le compteur courant, seul mode où les deux
 * divergent (contrat §6.2 : "compteur_retenu = RAFALE_TEAM_BEST en
 * MAILLON_FAIBLE, RAFALE_TEAM_COUNTERS sinon" — même règle que l'attribution
 * de fin de manche, réutilisée ici pour l'affichage).
 *
 * Mutualisé entre `/admin` (GamePage.jsx) et `/anim` (AnimPage.jsx, tâche 34)
 * — même discipline que `sortTeamsByBuzzOrder` ci-dessus : un seul classement,
 * jamais deux règles qui pourraient diverger silencieusement.
 *
 * @param {Array<{name:string}>} teamsList
 * @param {object} question - question courante (TYPE RAFALE, RAFALE_MODE)
 * @param {Object<string,number>} teamCounters - gameState.RAFALE_TEAM_COUNTERS
 * @param {Object<string,number>} teamBest - gameState.RAFALE_TEAM_BEST
 * @returns {Array} nouvelle liste (ne mute pas `teamsList`) — inchangée si
 *   `question` n'est pas de type RAFALE
 */
export function sortTeamsByRafaleCounter(teamsList, question, teamCounters, teamBest) {
  if (question?.TYPE !== 'RAFALE') return teamsList
  const useBest = question.RAFALE_MODE === 'MAILLON_FAIBLE'
  const source = useBest ? (teamBest || {}) : (teamCounters || {})
  return [...teamsList].sort((a, b) => (source[b.name] || 0) - (source[a.name] || 0))
}

/**
 * Badge de classement (🏆 🥈 🥉) pour un rang donné — null au-delà de la 3e place.
 * @param {number} rank - 1-based
 * @returns {string|null}
 */
export function getRankBadge(rank) {
  if (rank === 1) return '🏆'
  if (rank === 2) return '🥈'
  if (rank === 3) return '🥉'
  return null
}

/**
 * Temps de réaction formaté ("X.XXXs") depuis un TIME de buzz (bumper ou
 * équipe, microsecondes) et l'horloge de partie courante — même formule que
 * `TeamCard.jsx` (reactionTime).
 *
 * @param {number} [timestamp] - TIME du bumper/équipe (microsecondes)
 * @param {number} [gameTime] - gameState.gameTime (microsecondes)
 * @returns {string|null}
 */
export function formatReactionTime(timestamp, gameTime) {
  if (!timestamp || !gameTime) return null
  return `${((timestamp - gameTime) / 1000000).toFixed(3)}s`
}
