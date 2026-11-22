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
  if (seg.KIND === 'bonus') return '🎁 BONUS'
  return String(seg.VALUE)
}

/** Arrêt sur une case BONUS (v12.1.0, #308, contrat §15.2.3) : bandeau tant que `ROUE_LAST_EVENT.KIND = BONUS`. */
export const isBonusEvent = (gs) => gs?.ROUE_LAST_EVENT?.KIND === 'BONUS'

/** Dernière lettre proposée (entrée de ROUE_LETTERS) ou null. */
export function lastLetter(gs) {
  const l = gs?.ROUE_LETTERS
  return Array.isArray(l) && l.length ? l[l.length - 1] : null
}

export const isVowel = (l) => VOWELS.includes(String(l || '').toUpperCase())

/**
 * Achat de voyelles (v12.1.0, #147, contrat §15.1) — lecture d'AFFICHAGE des réglages effectifs de GameState
 * (le serveur reste seul juge). Un GameState sans `ROUE_BUY_VOWELS` vrai = comportement v12.0 (aucun achat).
 */
export function vowelInfo(gs) {
  const enabled = gs?.ROUE_BUY_VOWELS === true
  const timing = gs?.ROUE_VOWEL_TIMING || 'BEFORE'
  const price = Number(gs?.ROUE_VOWEL_PRICE) || 0
  const bank = Number((gs?.ROUE_TEAM_BANK || {})[gs?.ROUE_CURRENT_TEAM]) || 0
  const left = gs?.ROUE_VOWELS_LEFT !== false
  const before = enabled && timing !== 'AFTER'
  const after = enabled && timing !== 'BEFORE'
  const manyBefore = (gs?.ROUE_VOWELS_BEFORE || 'MANY') !== 'ONE'
  const manyAfter = (gs?.ROUE_VOWELS_AFTER || 'MANY') !== 'ONE'
  const sub = gs?.ROUE_SUBPHASE || ''
  const rich = bank >= price
  const afterPossible = after && left && rich
  return {
    enabled, timing, price, bank, left, before, after, manyBefore, manyAfter, rich, afterPossible,
    gain: gs?.ROUE_VOWEL_GAIN || 'NONE',
    // Points d'achat affichés (bouton visible) et possibles (bouton actif).
    showBefore: before && sub === 'TURN',
    canBuyBefore: before && sub === 'TURN' && left && rich && !(!manyBefore && gs?.ROUE_TURN_VOWEL === true),
    limitBefore: before && sub === 'TURN' && !manyBefore && gs?.ROUE_TURN_VOWEL === true,
    showAfter: after && (sub === 'LETTER' || (sub === 'ANSWER' && gs?.ROUE_STOP_VOWEL === true)),
    canBuyAfter: afterPossible && (sub === 'LETTER' || (sub === 'ANSWER' && gs?.ROUE_STOP_VOWEL === true && manyAfter)),
    limitAfter: after && sub === 'ANSWER' && gs?.ROUE_STOP_VOWEL === true && !manyAfter,
  }
}

export const fmtPts = (n) => Number(n || 0).toLocaleString('fr-FR')

/** « R : 3 × 500 = +1 500 » — le FLIP du moteur porte VALUE 0 : valeur de la case courante, gain du tour en attente. */
function consonantPoints(gs, ev, seg) {
  const value = ev.VALUE || seg?.VALUE || 0
  const gain = ev.GAIN > 0 ? ev.GAIN : (gs.ROUE_TURN_GAIN || 0)
  return [{ strong: ev.LETTER || '' }, { text: ` : ${ev.COUNT || 0} × ${value} = ` }, { strong: `+${fmtPts(gain)}` }]
}

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
  // BONUS (#308) : la main est rendue à la même équipe (nouveau tour en TURN) tant que l'événement dure.
  if (sub === 'TURN' && ev.KIND === 'BONUS') {
    return [T('🎁 BONUS'), t(mine === true ? ' — vous rejouez' : ' — les '), ...(mine === true ? [] : [T(team.toUpperCase()), t(' rejouent')])]
  }
  switch (sub) {
    case 'TURN': {
      const v = vowelInfo(gs)
      if (!gs.ROUE_CONSONANTS_LEFT) {
        return [t(v.afterPossible || v.canBuyBefore ? 'Plus de consonne : proposez la phrase ou achetez une voyelle' : 'Plus de consonne : proposez la phrase')]
      }
      const hint = v.before ? [t(` — roue ou voyelle (${fmtPts(v.price)})`)] : []
      return mine === true ? [t('À votre équipe de jouer'), ...hint] : [t('Aux '), T(team.toUpperCase()), t(' de jouer'), ...hint]
    }
    case 'SPINNING':
      // Montée (FULL_SPEED = false, roue.md §6) : personne ne peut encore arrêter la roue.
      if (gs?.ROUE_WHEEL && gs.ROUE_WHEEL.FULL_SPEED !== true) return [t('La roue prend de la vitesse…')]
      return mine === true ? [t('La roue tourne — arrêtez-la !')]
        : mine === false ? [t('Les '), T(team), t(' vont arrêter la roue')]
          : [T(team.toUpperCase()), t(' : buzzez pour arrêter la roue !')]
    case 'STOPPING':
      return [t('La roue ralentit…')]
    case 'LETTER': {
      const v = vowelInfo(gs)
      return [t('Case '), T(segmentLabel(seg)), t(v.after ? ` — consonne, ou voyelle à ${fmtPts(v.price)}` : ' — proposez une consonne')]
    }
    case 'VOWEL_LIT':
    case 'LIT':
      return [T(ev.LETTER || letter?.LETTER || ''), t(` : ${ev.COUNT || ''} case${(ev.COUNT || 0) > 1 ? 's' : ''}`)]
    case 'VOWEL_REVEALING':
    case 'REVEALING': {
      const cnt = ev.COUNT || 0
      // Le FLIP du moteur porte VALUE = 0 : voyelle → gain de l'événement ; consonne → valeur de la case × cases, gain du tour en attente.
      if (sub === 'VOWEL_REVEALING' || isVowel(ev.LETTER)) {
        return [T(ev.LETTER || ''), t(` : ${cnt} case${cnt > 1 ? 's' : ''}${ev.GAIN > 0 ? ' = ' : ''}`), ...(ev.GAIN > 0 ? [T(`+${fmtPts(ev.GAIN)}`)] : [])]
      }
      return consonantPoints(gs, ev, seg)
    }
    case 'ANSWER': {
      const say = t('Une réponse ? À voix haute')
      // Les points de la consonne restent affichés (le FLIP reste le dernier événement), le texte vient en dessous (QUALIF 12.1.0.2).
      // Voyelle : inchangé (gain déjà crédité au retournement).
      if (ev.KIND === 'FLIP' && ev.LETTER && !isVowel(ev.LETTER) && (ev.COUNT || 0) > 0) return [...consonantPoints(gs, ev, seg), { br: true }, say]
      return [say]
    }
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

/** Fabrique des 10 actions ROUE_* (contrat §8.1) au-dessus d'un `sendMessage(action, payload)`. */
export function makeRoueActions(sendMessage) {
  const send = (action, payload = {}) => sendMessage(action, payload)
  return {
    setTeams: (teams) => send('ROUE_SET_TEAMS', { TEAMS: teams }),
    spin: () => send('ROUE_SPIN'),
    buyVowel: (letter) => send('ROUE_BUY_VOWEL', { LETTER: letter }),
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
  const v = vowelInfo(gs)
  return {
    sub,
    // Sans consonne restante, la roue reste lançable si un achat après l'arrêt est possible (§15.1.4).
    canSpin: sub === 'TURN' && (left || v.afterPossible),
    canStop: sub === 'SPINNING',
    // « Forcer l'arrêt » (admin comme animateur) : actif seulement à pleine vitesse ; avant, le serveur refuse (NOT_FULL_SPEED) et les boutons sont grisés
    fullSpeed: gs?.ROUE_WHEEL?.FULL_SPEED === true,
    canLetter: sub === 'LETTER',
    canFlip: sub === 'LIT' || sub === 'VOWEL_LIT',
    canAnswer: sub === 'ANSWER' || noLetters || (sub === 'TURN' && gs?.ROUE_TURN_VOWEL === true),
    canNext: sub === 'ANSWER' || sub === 'TURN_END' || noLetters,
    canSame: sub === 'ANSWER' || sub === 'TURN_END' || noLetters,
    canNextTeam: (sub === 'ANSWER' || sub === 'TURN_END' || noLetters) && !solo,
    canPass: sub === 'TURN' && left && !solo,
    canReveal: !!sub && sub !== 'SOLVED',
    solo,
    canBuyBefore: v.canBuyBefore,
    canBuyAfter: v.canBuyAfter,
  }
}
