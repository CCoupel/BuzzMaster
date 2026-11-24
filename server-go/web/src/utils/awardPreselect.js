/**
 * awardPreselect — présélection des gagnants au crédit (contrat award-preselect.md, #321, v13.1.0).
 *
 * Fonction pure, testée en table. À l'entrée en `REVEALED`, la page Jeu admin coche d'office les
 * unités (équipes ou joueurs) que la question identifie ; le montant reste celui de la règle existante
 * (`amountOf`), le crédit reste manuel.
 *
 * Règles communes : jamais d'unité déjà créditée, jamais d'unité dont le montant par défaut est ≤ 0.
 */
import { calcMemoryScore, rafaleCounterForTeam, answeredColor, hasAnswered } from './pointsAward'
import { fold } from './fold'

function toSet(awarded) {
  if (!awarded) return new Set()
  if (awarded instanceof Set) return awarded
  if (Array.isArray(awarded)) return new Set(awarded)
  return new Set(Object.keys(awarded))
}

/** Équipes (noms) participantes d'une liste GameState, dédoublonnées. */
function participants(list) {
  return [...new Set(Array.isArray(list) ? list : [])]
}

/** Bumper de plus petit TIME > 0 → [mac, bumper] ou null (ordre de utils/buzzOrder.js). */
function firstBuzzer(bumpers) {
  let best = null
  Object.entries(bumpers || {}).forEach(([mac, b]) => {
    const t = b?.TIME ?? 0
    if (t > 0 && b.TEAM && (best === null || t < best[1].TIME)) best = [mac, b]
  })
  return best
}

/** Clés candidates (avant filtres communs) selon le type. */
function candidateKeys({ question, gameState, bumpers, awardLevel }) {
  const gs = gameState || {}
  const player = awardLevel === 'PLAYER'
  switch (question?.TYPE) {
    case 'PICTO':
      return gs.PICTO_OUTCOME === 'FOUND' && gs.PICTO_TEAM ? [gs.PICTO_TEAM] : []
    case 'RAFALE_PICTO':
      return (gs.PICTO_ROUND_POINTS ?? 0) > 0 && gs.PICTO_TEAM ? [gs.PICTO_TEAM] : []
    case 'MEMORY': {
      const teams = participants(gs.MEMORY_PARTICIPATING_TEAMS)
      if (question.MEMORY_MODE === 'SOLO') return teams.slice(0, 1)
      return teams.filter(t => {
        const s = calcMemoryScore(question, gs.MEMORY_TEAM_PAIRS?.[t] || 0, gs.MEMORY_TEAM_ERRORS?.[t] || 0)
        return (s?.score ?? 0) > 0
      })
    }
    case 'RAFALE':
      // SOLO et multi-équipes : même règle — équipes participantes de compteur retenu > 0.
      return participants(gs.RAFALE_PARTICIPATING_TEAMS)
        .filter(t => rafaleCounterForTeam(question, t, gs.RAFALE_TEAM_COUNTERS, gs.RAFALE_TEAM_BEST) > 0)
    case 'QCM': {
      if (!question.QCM_CORRECT) return []
      const keys = new Set()
      Object.entries(bumpers || {}).forEach(([mac, b]) => {
        if (!hasAnswered(b) || answeredColor(b) !== question.QCM_CORRECT) return
        keys.add(player ? mac : b.TEAM)
      })
      return [...keys]
    }
    case 'SPEEDY': {
      const first = firstBuzzer(bumpers)
      if (!first) return []
      return [player ? first[0] : first[1].TEAM]
    }
    case 'ARDOISE': {
      const expected = fold(question.ANSWER)
      if (!expected) return []
      return Object.entries(gs.ARDOISE_ANSWERS || {})
        .filter(([, a]) => fold(a?.TEXT) === expected)
        .map(([team]) => team)
    }
    // MEMOTION : gagnant de carte (award-preselect.md §4), pas la colonne Équipes ; ROUE : crédit automatique ;
    // ENTRACTE : aucun point ; type inconnu : rien.
    default:
      return []
  }
}

/**
 * @param {object} p
 * @param {object} p.question - question courante
 * @param {object} p.gameState
 * @param {Object<string,object>} p.bumpers - bumpers par MAC
 * @param {'TEAM'|'PLAYER'} p.awardLevel - resolveAwardLevel
 * @param {Set<string>|string[]|object} [p.awarded] - clés déjà créditées (équipe créditée au niveau équipe comprise)
 * @param {(key:string)=>number} p.amountOf - montant par défaut de la page
 * @param {string[]} [p.knownKeys] - clés existantes (équipes/joueurs affichés) ; une clé absente (équipe supprimée) est ignorée
 * @returns {string[]} clés (noms d'équipe ou MAC)
 */
export function preselectAwardKeys({ question, gameState, bumpers, awardLevel, awarded, amountOf, knownKeys }) {
  const done = toSet(awarded)
  const known = knownKeys ? new Set(knownKeys) : null
  const amount = typeof amountOf === 'function' ? amountOf : () => 1
  const seen = new Set()
  return candidateKeys({ question, gameState, bumpers, awardLevel }).filter(key => {
    if (!key || seen.has(key)) return false
    seen.add(key)
    if (done.has(key)) return false
    if (known && !known.has(key)) return false
    return amount(key) > 0
  })
}
