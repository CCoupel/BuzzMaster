/**
 * pointsAward — règles de calcul et d'attribution des points, mutualisées
 * (#155/#156, plan _work/reports/plan-20260813-094321.md §4 F1).
 *
 * Extrait de GamePage.jsx (calcQcmPenaltyForHints 255-265, qcmPenalty
 * 268-288, memoryScore 232-251, défaut ARDOISE 672) — même piège que
 * questionOrder.js (#149, GamePage.jsx:196-199) : une règle métier ne doit
 * jamais être dupliquée entre deux pages. La page animateur (#155/#156)
 * consomme ce module au même titre que /admin, pour garantir un montant
 * crédité strictement identique dans les deux interfaces (voir plan §7 R11).
 *
 * IMPORTANT — le montant de base n'est JAMAIS décidé ici. C'est toujours un
 * paramètre fourni par l'appelant (`pointsInput` sur /admin, `creditPoints`
 * — rediffusé par CREDIT_POINTS, PAS `question.POINTS` brut, voir MAJEUR-1
 * ci-dessous — sur /anim) : ce module applique les règles de pénalité/bonus
 * par-dessus, il ne choisit pas la source.
 */

/**
 * Pénalité QCM par joueur, pour un nombre d'indices donné au moment du buzz
 * (HINTS_AT_BUZZ). Utilisé pour calculer le montant réellement dû à un
 * joueur qui a buzzé avec 0, 1 ou 2+ indices déjà révélés — indépendant du
 * nombre d'indices révélés *maintenant* (voir calcQcmPenalty ci-dessous).
 *
 * @param {object} question - question courante (TYPE, QCM_HINTS_ENABLED, QCM_PENALTY_1/2)
 * @param {number} basePoints - montant de base (paramètre, non décidé ici)
 * @param {number} hintsAtBuzz - HINTS_AT_BUZZ enregistré sur le bumper au moment du buzz
 * @returns {{multiplier:number, effectivePoints:number, penaltyPercent:number}|null}
 *   null si la question n'est pas un QCM avec indices activés
 */
export function calcQcmPenaltyForHints(question, basePoints, hintsAtBuzz) {
  if (question?.TYPE !== 'QCM' || !question?.QCM_HINTS_ENABLED) return null
  const penalty1 = question?.QCM_PENALTY_1 || 0.67
  const penalty2 = question?.QCM_PENALTY_2 || 0.33
  let multiplier = 1
  if (hintsAtBuzz === 1) multiplier = penalty1
  else if (hintsAtBuzz >= 2) multiplier = penalty2
  const effectivePoints = Math.max(1, Math.round(basePoints * multiplier))
  const penaltyPercent = Math.round(multiplier * 100)
  return { multiplier, effectivePoints, penaltyPercent }
}

/**
 * Pénalité QCM basée sur le nombre d'indices *actuellement* invalidés
 * (gameState.qcmInvalidated) — utilisé pour l'affichage courant (badge de
 * saisie sur /admin, indicateur live sur /anim), par opposition à la
 * pénalité figée au moment du buzz (calcQcmPenaltyForHints).
 *
 * @param {object} question
 * @param {number} basePoints - montant de base (paramètre)
 * @param {number} invalidatedCount - gameState.qcmInvalidated?.length || 0
 * @returns {{invalidatedCount:number, multiplier:number, effectivePoints:number, penaltyPercent:number}|null}
 */
export function calcQcmPenalty(question, basePoints, invalidatedCount) {
  if (question?.TYPE !== 'QCM' || !question?.QCM_HINTS_ENABLED) return null
  if (!invalidatedCount) return null
  const penalty1 = question?.QCM_PENALTY_1 || 0.67
  const penalty2 = question?.QCM_PENALTY_2 || 0.33
  let multiplier = 1
  if (invalidatedCount === 1) multiplier = penalty1
  else if (invalidatedCount >= 2) multiplier = penalty2
  const effectivePoints = Math.max(1, Math.round(basePoints * multiplier))
  const penaltyPercent = Math.round(multiplier * 100)
  return { invalidatedCount, multiplier, effectivePoints, penaltyPercent }
}

/**
 * Score MEMORY à partir des paires trouvées, des erreurs et de la config de
 * la question (POINTS_PER_PAIR, ERROR_PENALTY, COMPLETION_BONUS). Couvre
 * aussi bien le mode SOLO (paires/erreurs globales de la partie) que le
 * calcul par équipe en mode multi-équipes (MEMORY_TEAM_PAIRS/ERRORS) — la
 * formule est identique, seule la source des paires/erreurs change côté
 * appelant.
 *
 * @param {object} question - question courante (TYPE, MEMORY_CONFIG, MEMORY_PAIRS)
 * @param {number} matchedPairs
 * @param {number} errors
 * @returns {{score:number, matchedPairs:number, totalPairs:number, errors:number,
 *   isComplete:boolean, pointsPerPair:number, errorPenalty:number, completionBonus:number}|null}
 *   null si la question n'est pas de type MEMORY
 */
export function calcMemoryScore(question, matchedPairs, errors) {
  if (question?.TYPE !== 'MEMORY') return null

  const config = question.MEMORY_CONFIG || {}
  const pointsPerPair = config.POINTS_PER_PAIR || 10
  const errorPenalty = config.ERROR_PENALTY || 0
  const completionBonus = config.COMPLETION_BONUS || 0

  const totalPairs = question.MEMORY_PAIRS?.length || 0
  const isComplete = matchedPairs === totalPairs && totalPairs > 0

  let score = matchedPairs * pointsPerPair
  if (isComplete) score += completionBonus
  score -= errors * errorPenalty
  if (score < 0) score = 0

  return { score, matchedPairs, totalPairs, errors, isComplete, pointsPerPair, errorPenalty, completionBonus }
}

/**
 * Montant par défaut attribué pour une réponse ARDOISE : POINTS de la
 * question si défini, sinon le montant de base fourni par l'appelant.
 *
 * @param {object} question
 * @param {number} fallbackPoints - montant de repli (ex: pointsInput sur /admin)
 * @returns {number}
 */
export function calcArdoiseDefaultPoints(question, fallbackPoints) {
  return parseInt(question?.POINTS) || fallbackPoints
}

/**
 * Détermine qui reçoit les points d'une attribution : l'équipe ou le joueur
 * — discriminant POINTS_TARGET (GamePage.jsx:404-411). S'applique à tous
 * les types de question, y compris SPEEDY (le cas nouvellement couvert pour
 * #156/F6).
 *
 * @param {object} question
 * @returns {'TEAM'|'PLAYER'}
 */
export function resolvePointsTarget(question) {
  return question?.POINTS_TARGET === 'TEAM' ? 'TEAM' : 'PLAYER'
}

/**
 * Résout le montant et la cible d'une attribution de points suite au buzz
 * d'un joueur — mutualise la règle appliquée par GamePage.jsx:396-411
 * (handleBumperClick) : QCM avec indices → pénalité par joueur, MEMORY →
 * score calculé, tout le reste (dont SPEEDY, le cas par défaut) → montant de
 * base tel quel. La cible (équipe/joueur) est toujours résolue par
 * POINTS_TARGET, quel que soit le type.
 *
 * Consommé par /admin (handleBumperClick) et par le bouton de crédit de la
 * page animateur (#156/F6) — un seul calcul, donc un montant garanti
 * identique pour un même `(question, basePoints, ctx)` dans les deux
 * interfaces (plan §7 R11). Cette garantie ne tenait PAS avant le mécanisme
 * SET_CREDIT_POINTS/CREDIT_POINTS (MAJEUR-1, revue de code #155/#156) :
 * `basePoints` valait `pointsInput` sur /admin (état React local, ajustable
 * après sélection — ex. manche bonus) mais `question.POINTS` brut sur
 * /anim, deux valeurs pouvant diverger silencieusement pour la même
 * question. Elle est vraie maintenant que /admin pousse tout ajustement de
 * `pointsInput` au serveur (SET_CREDIT_POINTS, debounced) qui le rediffuse
 * à /anim (CREDIT_POINTS) — voir `useWebSocket.js` (`creditPoints`) et
 * `AnimPage.jsx`, qui appelle cette fonction avec `creditPoints`, jamais
 * `question.POINTS` directement.
 *
 * @param {object} question - question courante
 * @param {number} basePoints - montant de base (paramètre : pointsInput sur
 *   /admin, creditPoints — issu de CREDIT_POINTS, PAS question.POINTS brut
 *   — sur /anim)
 * @param {object} [ctx]
 * @param {number} [ctx.hintsAtBuzz] - QCM : HINTS_AT_BUZZ du bumper au moment du buzz
 * @param {{matchedPairs:number, errors:number}} [ctx.memory] - MEMORY : paires/erreurs à créditer
 * @returns {{amount:number, target:'TEAM'|'PLAYER'}}
 */
export function resolvePointsAward(question, basePoints, ctx = {}) {
  let amount = basePoints

  if (question?.TYPE === 'MEMORY' && ctx.memory) {
    const memory = calcMemoryScore(question, ctx.memory.matchedPairs || 0, ctx.memory.errors || 0)
    if (memory) amount = memory.score
  } else if (question?.TYPE === 'QCM' && question?.QCM_HINTS_ENABLED) {
    const penalty = calcQcmPenaltyForHints(question, basePoints, ctx.hintsAtBuzz || 0)
    if (penalty) amount = penalty.effectivePoints
  }

  return { amount, target: resolvePointsTarget(question) }
}

/**
 * Montant de crédit QCM au niveau ÉQUIPE — mutualise la règle restée locale
 * à GamePage.jsx (`qcmTeamAcquiredPoints` 272-290 et le repli du clic sur
 * carte d'équipe 1072-1078) avant #157 : une équipe = un buzzer retenu par
 * le moteur (`engine.go:1404-1409`, verrou par équipe), mais la fonction
 * reste correcte si plusieurs bumpers sont fournis (garde le meilleur
 * montant), l'invariant moteur n'étant pas une hypothèse à coder en dur ici.
 *
 * Trois branches :
 *  1. Chercher parmi `teamBumpers` celui dont `ANSWER_COLOR === QCM_CORRECT`.
 *  2. Trouvé → pénalité PAR JOUEUR selon SON `HINTS_AT_BUZZ` au moment du
 *     buzz (calcQcmPenaltyForHints) — figée, indépendante des indices
 *     révélés depuis.
 *  3. Aucun bumper correct pour cette équipe → repli sur la pénalité des
 *     indices COURANTS (calcQcmPenalty, `invalidatedCount`), PAS celle du
 *     buzz — c'est délibérément différent de la branche 2 (aucune réponse
 *     correcte à figer, donc pas d'autre repère que l'état actuel).
 *
 * Consommé par /admin (`GamePage.jsx`) et par le bouton de crédit de la
 * page animateur (#156/F6, #157) pour garantir le même montant dans les
 * deux interfaces, y compris ce repli — l'oublier romprait la parité
 * exactement dans le cas où une équipe n'a pas la bonne réponse.
 *
 * @param {object} question - question courante (TYPE QCM, QCM_CORRECT, QCM_HINTS_ENABLED, ...)
 * @param {number} basePoints - montant de base (paramètre : pointsInput sur /admin, creditPoints sur /anim)
 * @param {Array<{TIME?: number, ANSWER_COLOR?: string, HINTS_AT_BUZZ?: number}>} teamBumpers - bumpers de
 *   l'équipe (objets bumper bruts) — seuls ceux ayant buzzé (`TIME > 0`) sont éligibles à la branche 1
 * @param {number} invalidatedCount - gameState.qcmInvalidated?.length || 0
 * @returns {{amount: number, hasCorrectAnswer: boolean}} hasCorrectAnswer distingue
 *   la branche 2 (réponse correcte trouvée) de la branche 3 (repli) — sert à
 *   l'affichage (ex: n'ajouter une équipe à une liste "a la bonne réponse"
 *   que si `hasCorrectAnswer` est vrai)
 */
export function calcQcmTeamAward(question, basePoints, teamBumpers, invalidatedCount) {
  const correctColor = question?.QCM_CORRECT
  let best = null

  if (correctColor) {
    ;(teamBumpers || []).forEach(bumper => {
      if (!bumper || !bumper.TIME || bumper.ANSWER_COLOR !== correctColor) return
      const hints = bumper.HINTS_AT_BUZZ || 0
      const penalty = calcQcmPenaltyForHints(question, basePoints, hints)
      const pts = penalty ? penalty.effectivePoints : basePoints
      if (best === null || pts > best) best = pts
    })
  }

  if (best !== null) return { amount: best, hasCorrectAnswer: true }

  const fallbackPenalty = calcQcmPenalty(question, basePoints, invalidatedCount)
  return { amount: fallbackPenalty ? fallbackPenalty.effectivePoints : basePoints, hasCorrectAnswer: false }
}
