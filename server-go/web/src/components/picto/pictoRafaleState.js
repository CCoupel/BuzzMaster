/**
 * pictoRafaleState — helpers purs de la manche Rafale Picto (v13.1.0, #318, picto.md §17). Aucune règle de jeu :
 * le serveur reste seul juge (compteurs, passes restantes, crédit suggéré) ; ces fonctions ne servent qu'à l'affichage.
 */

/** Durée d'affichage du mot trouvé sur le dessin, TV seulement (picto.md §17.5). */
export const PICTO_FOUND_FLASH_MS = 1000

/** « 3 mots trouvés », « 1 mot trouvé », « 0 mot trouvé ». */
export const foundLabel = (n) => `${n} mot${n > 1 ? 's' : ''} trouvé${n > 1 ? 's' : ''}`

/** Passes utilisées : « 0 passe », « 1 passe », « 2 passes » (accord au pluriel dès 2). */
export const passUsedLabel = (n) => `${n} passe${n > 1 ? 's' : ''}`

/** Réglage des passes d'après `PICTO_PASSES_LEFT` (−1 = illimité) : « illimitées » / « N restante(s) ». */
export function passesLeftLabel(gs) {
  const left = gs?.PICTO_PASSES_LEFT
  if (left === undefined || left === null || left < 0) return 'illimitées'
  return `${left} restante${left > 1 ? 's' : ''}`
}

/** Réglage de la question : « 1 passe » ou « illimitées » (RAFALE_PICTO_PASS_LIMIT). */
export const passLimitLabel = (q) => (Number(q?.RAFALE_PICTO_PASS_LIMIT) === 1 ? '1 passe' : 'illimitées')

/** « passer » proposé : manche en cours seulement (§17.5, `STARTED`) et il reste une passe (−1 = illimité). */
export const canPictoPass = (gs) => gs?.phase === 'STARTED' && gs?.PICTO_PASSES_LEFT !== 0

/** « Trouvé » proposé en Rafale Picto : manche en cours, en pause, ou temps écoulé sur le gong (§17.7). */
export const canRafaleFound = (gs) => gs?.phase === 'STARTED' || gs?.phase === 'PAUSED' || (gs?.phase === 'STOPPED' && gs?.PICTO_OUTCOME === 'TIMEOUT')

/** Texte de la formule du crédit : « 3 × 10 − 1 × 5 » (informatif ; le montant vient du serveur). */
export function roundPointsFormula(gs, q) {
  const found = gs?.PICTO_FOUND_COUNT || 0
  const passes = gs?.PICTO_PASS_COUNT || 0
  const per = Number(q?.POINTS) || 0
  const cost = Number(q?.RAFALE_PICTO_PASS_COST) || 0
  return `${found} × ${per}${cost > 0 || passes > 0 ? ` − ${passes} × ${cost}` : ''}`
}

/**
 * Fil de la manche (admin et anim seulement) : mots dans l'ordre où ils ont été montrés, avec leur issue —
 * 'found' ✅, 'passed' ⏭, 'current' ✏️ (en cours), 'revealed' ⌛ (mot en cours à la fin de la manche).
 * Les mots passés ne sont jamais dans le GameState : le fil est construit côté client, au fil des `PICTO_WORD`.
 * Pas de mot à venir (jamais tiré d'avance).
 *
 * @param {Array<{word:string, epoch:number, status:string}>} thread - fil précédent
 * @param {{word:string, epoch:number, found:number, passes:number}} ev - mot affiché maintenant et compteurs du serveur
 * @param {{found:number, passes:number}} seen - compteurs déjà pris en compte
 * @returns {{thread: Array, seen: {found:number, passes:number}}}
 */
export function advanceThread(thread, ev, seen = { found: 0, passes: 0 }) {
  const list = thread.slice()
  const last = list[list.length - 1]
  if (last && last.epoch === ev.epoch) {
    // Même mot : seulement le mot visible peut avoir changé (rechargement de page)
    if (ev.word && !last.word) list[list.length - 1] = { ...last, word: ev.word }
    return { thread: list, seen: { found: ev.found, passes: ev.passes } }
  }
  if (last && last.status === 'current') {
    // compteur de trouvés monté = trouvé ; sinon le mot a été passé (le seul autre moyen d'obtenir un nouveau mot)
    const status = ev.found > seen.found ? 'found' : 'passed'
    list[list.length - 1] = { ...last, status }
  }
  if (ev.word) list.push({ word: ev.word, epoch: ev.epoch, status: 'current' })
  return { thread: list, seen: { found: ev.found, passes: ev.passes } }
}

/** Fin de manche : le mot en cours devient « révélé » (⌛), ou « trouvé » si le dernier compteur a monté. */
export function closeThread(thread, ev, seen) {
  const list = thread.slice()
  const last = list[list.length - 1]
  if (last && last.status === 'current') {
    list[list.length - 1] = { ...last, status: ev.found > seen.found ? 'found' : 'revealed' }
  }
  return list
}

export const THREAD_ICON = { found: '✅', passed: '⏭', current: '✏️', revealed: '⌛' }

/** Mots nécessaires estimés pour la manche : plafond(TIME / 10) (picto.md §17.3). */
export const neededWords = (q) => (Number(q?.TIME) > 0 ? Math.ceil(Number(q.TIME) / 10) : 0)
