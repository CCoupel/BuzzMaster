/**
 * questionOrder — shared question-ordering logic (#149, plan tâche 25.1).
 *
 * Mutualise le tri auparavant dupliqué à l'identique dans
 * QuestionsPage.jsx et GamePage.jsx (tri sur ORDER, repli sur ID) — toute
 * option de #149 devait d'abord mutualiser ce tri, faute de quoi un
 * correctif divergerait entre les deux pages.
 */

/**
 * Sorts questions by their ORDER field, falling back to ID when ORDER is
 * absent (questions créées avant l'introduction du champ, ou jamais
 * réordonnées).
 *
 * @param {Object.<string, object>} questionsById - `questions` map from useGame()
 * @returns {Array} Questions triées, uniquement celles avec un ID valide
 */
export function sortQuestionsByOrder(questionsById) {
  return Object.values(questionsById || {})
    .filter(q => q && q.ID)
    .sort((a, b) => {
      const orderA = a.ORDER !== undefined ? parseInt(a.ORDER) : parseInt(a.ID)
      const orderB = b.ORDER !== undefined ? parseInt(b.ORDER) : parseInt(b.ID)
      return orderA - orderB
    })
}

/**
 * Unbiased Fisher-Yates shuffle — returns a NEW array, never mutates the
 * input. Contrairement au mélange seedé de PlayerDisplay.jsx:731-741 (choisi
 * là-bas pour être reproductible entre clients à partir du même ID de
 * question), ce mélange n'a pas besoin de seed : la reproductibilité entre
 * écrans vient ici de la PERSISTANCE de l'ordre choisi (REORDER_QUESTIONS),
 * pas du générateur — chaque appel doit donc être un tirage réellement
 * aléatoire, pas déterministe.
 *
 * @param {Array} array
 * @returns {Array} A shuffled copy of `array`
 */
export function shuffleArray(array) {
  const result = [...array]
  for (let i = result.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1))
    ;[result[i], result[j]] = [result[j], result[i]]
  }
  return result
}

/**
 * #245 (v11.2) — ordre de jeu et d'affichage à DEUX niveaux, règle unique du contrat (models.md §Manches) écrite aussi
 * côté Go (`getNextQuestionPayload`) : (1) le groupe « Sans manche » EN PREMIER ; (2) puis les manches dans l'ordre de
 * `rounds` ; (3) dans chaque groupe, par ORDER croissant puis par ID numérique. Une question dont `ROUND_ID` est absent,
 * vide ou inconnu est « Sans manche ». Sans aucune manche, l'ordre est identique à `sortQuestionsByOrder` (v11.1).
 */
/** Nombre fini si `v` est un nombre (les ORDER non entiers sont tronqués plus bas), ou une chaîne en entier décimal ("7") ; sinon null. */
function strictNumber(v) {
  if (typeof v === 'number') return Number.isFinite(v) ? v : null
  // Chaîne : entier décimal seulement (parité Go) — "7" oui ; "7.9", "1e2", "0x10", "5abc" non.
  if (typeof v === 'string' && /^\s*[+-]?\d+\s*$/.test(v)) {
    const n = Number(v)
    return Number.isFinite(n) ? n : null
  }
  return null
}

/**
 * ORDER effectif (parité avec le Go) : un ORDER entier ou chaîne entièrement numérique est pris tel quel ; absent,
 * null ou invalide ("5abc") = repli sur l'ID numérique ; un ID non numérique n'a pas de repli (sans ORDER valide il
 * passe après les autres du groupe).
 */
function effectiveOrder(q) {
  const o = strictNumber(q.ORDER)
  if (o !== null) return Math.trunc(o) // ORDER non entier (1.5, 1.9) : tronqué, comme le Go
  const id = strictNumber(q.ID)
  return id !== null ? id : Infinity
}

function compareInGroup(a, b) {
  const oa = effectiveOrder(a)
  const ob = effectiveOrder(b)
  if (oa !== ob) return oa < ob ? -1 : 1
  const ia = strictNumber(a.ID)
  const ib = strictNumber(b.ID)
  if (ia !== null && ib !== null) return ia - ib
  if (ia !== null) return -1
  if (ib !== null) return 1
  return String(a.ID) < String(b.ID) ? -1 : String(a.ID) > String(b.ID) ? 1 : 0
}

/**
 * Répartit les questions en groupes ordonnés.
 * @returns {{ unassigned: object[], rounds: Array<{ID: string, NAME: string, questions: object[]}> }}
 */
export function groupQuestionsByRound(questionsById, rounds) {
  const list = Array.isArray(rounds) ? rounds : []
  const known = new Set(list.map(r => r.ID))
  const unassigned = []
  const byRound = new Map(list.map(r => [r.ID, []]))
  Object.values(questionsById || {})
    .filter(q => q && q.ID)
    .forEach(q => {
      if (q.ROUND_ID && known.has(q.ROUND_ID)) byRound.get(q.ROUND_ID).push(q)
      else unassigned.push(q)
    })
  return {
    unassigned: unassigned.sort(compareInGroup),
    rounds: list.map(r => ({ ID: r.ID, NAME: r.NAME, questions: byRound.get(r.ID).sort(compareInGroup) })),
  }
}

/** Liste à plat dans l'ordre de jeu : « Sans manche », puis chaque manche. */
export function sortQuestionsByRounds(questionsById, rounds) {
  const g = groupQuestionsByRound(questionsById, rounds)
  return [...g.unassigned, ...g.rounds.flatMap(r => r.questions)]
}

/** Nombre de questions réelles et d'entractes d'une liste (les ENTRACTE ne comptent pas comme questions). */
export function countEntries(questions) {
  const entractes = questions.filter(q => q.TYPE === 'ENTRACTE').length
  return { questions: questions.length - entractes, entractes }
}

/**
 * #245 — rang 0-based de la manche d'une question dans `rounds`, ou -1 (« Sans manche » : ROUND_ID absent, vide
 * ou inconnu).
 */
export function roundIndexOf(question, rounds) {
  if (!question?.ROUND_ID || !Array.isArray(rounds)) return -1
  return rounds.findIndex(r => r.ID === question.ROUND_ID)
}
