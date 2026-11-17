// Helpers partagés ROUE (v12.0.0, #144, contrat roue.md) — TV, tablette, admin, animateur.
// Aucune règle de jeu ici : uniquement la lecture de GameState et la fabrique d'actions WebSocket.

export const CONSONANTS = 'BCDFGHJKLMNPQRSTVWXZ'.split('')
export const VOWELS = 'AEIOUY'.split('')

/** Étapes affichées par le bandeau (`ROUE_STEP` 1 à 7, contrat §5). */
export const ROUE_STEPS = [
  { n: 1, name: 'Lancer\nla roue', short: 'Lancer la roue' },
  { n: 2, name: 'Arrêt\nde la roue', short: 'Arrêt de la roue' },
  { n: 3, name: 'Affichage\nde la case', short: 'Affichage de la case' },
  { n: 4, name: 'Sélection de\nla consonne', short: 'Sélection de la consonne' },
  { n: 5, name: 'Affichage\ndes cases', short: 'Affichage des cases' },
  { n: 6, name: 'Réponse', short: 'Réponse' },
  { n: 7, name: 'Fin\ndu tour', short: 'Fin du tour' },
]

/** [r,g,b] → couleur CSS ; sinon repli. */
export function teamCss(color, fallback = '#6b7280') {
  return Array.isArray(color) && color.length === 3 ? `rgb(${color.join(',')})` : (typeof color === 'string' && color ? color : fallback)
}

export const isRoueQuestion = (gs) => gs?.question?.TYPE === 'ROUE'
export const roueSolo = (gs) => gs?.question?.ROUE_MODE === 'SOLO'

/** Case actuellement sous le pointeur ({KIND, VALUE}) ou null. */
export function currentSegment(gs) {
  const w = gs?.ROUE_WHEEL
  const i = w?.CURRENT
  return w && Number.isInteger(i) && i >= 0 ? (w.SEGMENTS?.[i] ?? null) : null
}

export function segmentLabel(seg) {
  if (!seg) return ''
  if (seg.KIND === 'bankrupt') return 'BANQUEROUTE'
  if (seg.KIND === 'lose_turn') return 'TOUR PERDU'
  return String(seg.VALUE)
}

/** Dernière lettre proposée (entrée de ROUE_LETTERS) ou null. */
export function lastLetter(gs) {
  const l = gs?.ROUE_LETTERS
  return Array.isArray(l) && l.length ? l[l.length - 1] : null
}

export const fmtPts = (n) => Number(n || 0).toLocaleString('fr-FR')

/**
 * Phrase d'annonce (TV / tablette) selon la sous-phase — texte brut ; `team` = équipe qui a la main.
 * @returns {{ text: string, strong?: string }[]} segments (`strong` = mis en évidence)
 */
export function roueAnnouncement(gs, { mine = null } = {}) {
  const sub = gs?.ROUE_SUBPHASE || ''
  const team = gs?.ROUE_CURRENT_TEAM || ''
  const seg = currentSegment(gs)
  const ev = gs?.ROUE_LAST_EVENT || {}
  const letter = lastLetter(gs)
  const T = (strong) => ({ strong })
  const t = (text) => ({ text })
  switch (sub) {
    case 'TURN':
      if (!gs.ROUE_CONSONANTS_LEFT) return [t('Plus de consonne : proposez la phrase')]
      return mine === true ? [t('À votre équipe de jouer')] : [t('Aux '), T(team.toUpperCase()), t(' de jouer')]
    case 'SPINNING':
      // Montée (FULL_SPEED = false, roue.md §6) : personne ne peut encore arrêter la roue.
      if (gs?.ROUE_WHEEL && gs.ROUE_WHEEL.FULL_SPEED !== true) return [t('La roue prend de la vitesse…')]
      return mine === true ? [t('La roue tourne — arrêtez-la !')]
        : mine === false ? [t('Les '), T(team), t(' vont arrêter la roue')]
          : [T(team.toUpperCase()), t(' : buzzez pour arrêter la roue !')]
    case 'STOPPING':
      return [t('La roue ralentit…')]
    case 'LETTER':
      return [t('Case '), T(segmentLabel(seg)), t(' — proposez une consonne')]
    case 'LIT':
      return [T(ev.LETTER || letter?.LETTER || ''), t(` : ${ev.COUNT || ''} case${(ev.COUNT || 0) > 1 ? 's' : ''}`)]
    case 'REVEALING':
      return [T(ev.LETTER || ''), t(` : ${ev.COUNT || 0} × ${ev.VALUE || 0} = `), T(`+${fmtPts(ev.GAIN ?? gs.ROUE_TURN_GAIN)}`)]
    case 'ANSWER':
      return [t('Une réponse ? À voix haute')]
    case 'TURN_END': {
      if (seg?.KIND === 'bankrupt') return [t('💸 '), T('BANQUEROUTE'), t(` — la cagnotte des ${team} retombe à 0`)]
      if (seg?.KIND === 'lose_turn') return [t('⏭ '), T('TOUR PERDU')]
      if (letter && letter.HIT === false) return [T(letter.LETTER), t(' : absente')]
      if (ev.GAIN > 0) return [T(`+${fmtPts(ev.GAIN)}`), t(` pour les ${team}`)]
      return [t('Fin du tour')]
    }
    case 'SOLVED': {
      const w = gs.ROUE_WINNER
      if (!w) return [t('Phrase dévoilée')]
      const aw = gs.ROUE_AWARDS?.[w]
      return [t('🎉 Les '), T(w.toUpperCase()), t(' trouvent la phrase'), ...(aw ? [t(' : '), T(`+${fmtPts(aw)} points`)] : [])]
    }
    default:
      return []
  }
}

/** Fabrique des 9 actions ROUE_* (contrat §8.1) au-dessus d'un `sendMessage(action, payload)`. */
export function makeRoueActions(sendMessage) {
  const send = (action, payload = {}) => sendMessage(action, payload)
  return {
    setTeams: (teams) => send('ROUE_SET_TEAMS', { TEAMS: teams }),
    spin: () => send('ROUE_SPIN'),
    stop: () => send('ROUE_STOP'),
    letter: (letter) => send('ROUE_LETTER', { LETTER: letter }),
    flip: () => send('ROUE_FLIP'),
    answer: () => send('ROUE_ANSWER'),
    next: (hand) => send('ROUE_NEXT', { HAND: hand === 'NEXT' ? 'NEXT' : 'SAME' }),
    pass: () => send('ROUE_PASS'),
    reveal: () => send('ROUE_REVEAL'),
  }
}

/**
 * Ce que la sous-phase courante autorise (réplique d'AFFICHAGE du contrat §5/§8, pas une règle : le serveur
 * reste seul juge et répond `ROUE_ERROR`). Sert à griser/activer les boutons.
 */
export function roueGates(gs) {
  const sub = gs?.ROUE_SUBPHASE || ''
  const left = gs?.ROUE_CONSONANTS_LEFT !== false
  const solo = roueSolo(gs)
  const noLetters = !left && sub === 'TURN'
  return {
    sub,
    canSpin: sub === 'TURN' && left,
    canStop: sub === 'SPINNING',
    // « Forcer l'arrêt » (admin comme animateur) : actif seulement à pleine vitesse ; avant, le serveur refuse (NOT_FULL_SPEED) et les boutons sont grisés
    fullSpeed: gs?.ROUE_WHEEL?.FULL_SPEED === true,
    canLetter: sub === 'LETTER',
    canFlip: sub === 'LIT',
    canAnswer: sub === 'ANSWER' || noLetters,
    canNext: sub === 'ANSWER' || sub === 'TURN_END' || noLetters,
    canSame: sub === 'ANSWER' || sub === 'TURN_END' || noLetters,
    canNextTeam: (sub === 'ANSWER' || sub === 'TURN_END' || noLetters) && !solo,
    canPass: sub === 'TURN' && left && !solo,
    canReveal: !!sub && sub !== 'SOLVED',
    solo,
  }
}
