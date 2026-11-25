/**
 * pictoState — helpers purs des écrans PICTO (contrat picto.md §5, §6, §9). Aucune règle de jeu ici : le serveur reste
 * seul juge (`PICTO_ERROR`), ces fonctions ne servent qu'à l'affichage et à griser les boutons.
 */
import { isPictoFamily } from '../../utils/pictoFamily'


export const isPictoQuestion = (gs) => isPictoFamily(gs?.question?.TYPE)

/** Fabrique des actions de préparation (§9.1) au-dessus d'un `sendMessage(action, payload)`. */
export function makePictoActions(sendMessage, hooks = {}) {
  return {
    setTeam: (team) => sendMessage('PICTO_SET_TEAM', { TEAM: team || '' }),
    setDrawer: (player) => sendMessage('PICTO_SET_DRAWER', { PLAYER: player || '' }),
    drawDrawer: () => sendMessage('PICTO_DRAW_DRAWER', {}),
    // « Trouvé ! » (§6.3/§6.4) : STARTED, PAUSED, ou STOPPED après TIMEOUT (réponse donnée sur le gong)
    // `epoch` : obligatoire en Rafale Picto (STALE_WORD sinon), facultatif en Picto un mot (§17.5)
    // Avec une époque, passe par le callback du contexte (`useGame().pictoFound`) quand il est fourni.
    found: (epoch) => (epoch === undefined || epoch === null
      ? sendMessage('PICTO_FOUND', {})
      : (hooks.found ? hooks.found(epoch) : sendMessage('PICTO_FOUND', { EPOCH: epoch }))),
    // « Passer » (Rafale Picto seulement, §17.5) : admin et anim ; le dessinateur l'annonce à l'oral
    pass: (epoch) => (hooks.pass ? hooks.pass(epoch) : sendMessage('PICTO_PASS', { EPOCH: epoch })),
  }
}

/** Phases de préparation : équipe et dessinateur se choisissent seulement ici (§6.1). */
export const isPreparing = (phase) => phase === 'PREPARE' || phase === 'READY'
/** Phases où la manche est jouée (mot en main du dessinateur) avant la révélation. */
export const isDrawing = (phase) => phase === 'STARTED' || phase === 'PAUSED' || phase === 'STOPPED'

/** « START » est refusé sans dessinateur désigné et connecté (§6.1) : sert à griser « Lancer » (affichage seulement). */
export const pictoLaunchBlocked = (gs) => isPictoQuestion(gs) && (!gs.PICTO_DRAWER || gs.PICTO_DRAWER_CONNECTED !== true)

/** Raison courte affichée sous « Lancer » quand il est grisé faute de dessinateur. */
export function pictoLaunchReason(gs) {
  if (!isPictoQuestion(gs)) return null
  if (!gs.PICTO_TEAM) return "choisir l'équipe"
  if (!gs.PICTO_DRAWER) return 'choisir le dessinateur'
  if (gs.PICTO_DRAWER_CONNECTED !== true) return 'dessinateur hors ligne'
  return null
}

/**
 * Mot de la manche à afficher (admin, anim, dessinateur seulement) : `PICTO_WORD` de l'époque courante tant que la
 * manche se joue, `PICTO_REVEALED_WORD` à la révélation (seul moment où le mot entre dans le GameState, §6.5).
 */
export function pictoShownWord(gs, pictoWord) {
  if (!gs) return ''
  if (gs.phase === 'REVEALED') return gs.PICTO_REVEALED_WORD || ''
  if (!isDrawing(gs.phase)) return ''
  if (pictoWord && pictoWord.EPOCH === gs.PICTO_CANVAS_EPOCH) return pictoWord.WORD || ''
  return ''
}

/** « Trouvé ! » proposé : manche en cours, ou temps écoulé tant que le mot n'est pas révélé (I5, §6.4). */
export const canPictoFound = (gs) => gs?.phase === 'STARTED' || gs?.phase === 'PAUSED' || (gs?.phase === 'STOPPED' && gs?.PICTO_OUTCOME === 'TIMEOUT')

/** Libellé court de l'issue (admin / anim). */
export function outcomeLabel(gs) {
  switch (gs?.PICTO_OUTCOME) {
    case 'FOUND': return '✅ Trouvé'
    case 'TIMEOUT': return '⏰ Temps écoulé'
    case 'STOPPED': return '⏹ Manche arrêtée'
    case 'NO_WORD': return 'Aucun mot dans la question'
    case 'POOL_END': return '🗂 Fin du pool'
    default: return ''
  }
}

export const ERROR_TEXT = {
  INVALID_TEAM: "Équipe inconnue ou non choisie.",
  NOT_A_CANDIDATE: "Ce joueur n'est pas un candidat de l'équipe.",
  DRAWER_OFFLINE: 'Ce joueur est hors ligne.',
  NO_CONNECTED_PLAYER: "Aucun joueur connecté dans cette équipe : il faut un écran pour dessiner.",
  NO_DRAWER: 'Choisissez un dessinateur connecté avant de lancer.',
  WRONG_PHASE: "Action impossible dans cette phase.",
  NOT_DRAWER: "Vous n'êtes pas le dessinateur.",
  INVALID_STROKES: 'Trait refusé.',
  CANVAS_FULL: 'Feuille pleine',
  RATE_LIMITED: 'Trop de traits à la fois.',
}
export const errorText = (e) => (e ? (ERROR_TEXT[e.CODE] || e.MESSAGE || e.CODE || '') : '')

/** Couleur d'équipe (tableau [r,g,b] ou chaîne CSS). */
export function teamCss(color, fallback = '#6b7280') {
  return Array.isArray(color) && color.length === 3 ? `rgb(${color.join(',')})` : (typeof color === 'string' && color ? color : fallback)
}

/**
 * Origine des mots d'une question Picto (v13.1.0, picto.md §17.2) : `PICTO_SOURCE = RESERVOIR` → Pool Picto, mots
 * encore tirables = `PICTO_POOL_AVAILABLE` ; sinon (absent = mots saisis) → nombre de mots de la question.
 */
export function pictoWordsInfo(gs) {
  const q = gs?.question || {}
  const rafale = q.TYPE === 'RAFALE_PICTO' // champs propres du type (picto.md §17.2)
  const fromPool = (rafale ? q.RAFALE_PICTO_SOURCE : q.PICTO_SOURCE) === 'RESERVOIR'
  return {
    fromPool,
    available: gs?.PICTO_POOL_AVAILABLE || 0,
    words: gs?.PICTO_WORDS_COUNT || ((rafale ? q.RAFALE_PICTO_WORDS : q.PICTO_WORDS) || []).length || 0,
  }
}
