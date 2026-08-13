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
