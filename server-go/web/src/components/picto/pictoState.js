/**
 * pictoState — helpers purs des écrans PICTO (contrat picto.md §5, §6, §9). Aucune règle de jeu ici : le serveur reste
 * seul juge (`PICTO_ERROR`), ces fonctions ne servent qu'à l'affichage et à griser les boutons.
 */

export const isPictoQuestion = (gs) => gs?.question?.TYPE === 'PICTO'

/** Fabrique des actions de préparation (§9.1) au-dessus d'un `sendMessage(action, payload)`. */
export function makePictoActions(sendMessage) {
  return {
    setTeam: (team) => sendMessage('PICTO_SET_TEAM', { TEAM: team || '' }),
    setDrawer: (player) => sendMessage('PICTO_SET_DRAWER', { PLAYER: player || '' }),
    drawDrawer: () => sendMessage('PICTO_DRAW_DRAWER', {}),
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
