/**
 * motionGrid — règle de disposition de la grille MEMOTION, PARTAGÉE entre
 * `PlayerDisplay.jsx` (`/tv`, vue joueur, aperçu TV de la régie),
 * `GamePage.jsx` (`/admin`, panneau MEMOTION en mode Secret) et
 * `AnimMotionGrid` (`/anim`) (#160/F0).
 *
 * Extraction PURE depuis `PlayerDisplay.jsx:1985-1996` (dupliquée à
 * l'identique dans `GamePage.jsx:814-837`) — même formules, zéro changement
 * de comportement pour les clients existants.
 *
 * MOTIF (même leçon que `memoryGrid.js`, #159/F0 — la neuvième mutualisation
 * de la série #149/#155/#158/#159/#165/#166/#170/#171) : un joueur peut
 * annoncer verbalement « la carte A2 » (mode Secret) — l'animateur doit voir
 * EXACTEMENT la même carte à cette position que `/tv`. Si cette règle était
 * réimplémentée « à l'identique » au lieu d'être extraite, la moindre
 * évolution future d'une des copies casserait silencieusement cette
 * correspondance : aucune erreur, aucun test rouge — juste un animateur et
 * un joueur qui ne parlent plus de la même carte en pleine partie.
 *
 * ⚠️ La formule de colonnes MEMOTION DIFFÈRE de celle de MEMORY
 * (`memoryGrid.js` : 2/3/4/5/6 sur 4/6/16/20 cartes). Ne pas fusionner les
 * deux utilitaires, ne pas réutiliser l'un pour l'autre.
 *
 * Le nombre de colonnes ne dépend JAMAIS de la largeur de l'écran — seule
 * la TAILLE des cartes s'adapte (CSS, container queries).
 */

import { getQuestionTypeMeta } from './questionTypeMeta'

/**
 * Nombre de colonnes — formule FIXE sur le nombre de cartes, jamais sur la
 * largeur de l'écran (c'est ce qui rend la correspondance possible).
 *
 * Source : `PlayerDisplay.jsx:2000` (et copie identique `GamePage.jsx:836`).
 *
 * @param {number} count
 * @returns {number}
 */
export function getMotionGridCols(count) {
  if (count <= 4) return 2
  if (count <= 6) return 3
  if (count <= 12) return 4
  return 5
}

/**
 * Nombre de rangées, déduit du nombre de cartes et de colonnes.
 *
 * Source : `PlayerDisplay.jsx:2001`.
 *
 * @param {number} count
 * @returns {number}
 */
export function getMotionGridRows(count) {
  const cols = getMotionGridCols(count)
  return Math.ceil(count / cols)
}

/**
 * Coordonnée d'une carte (mode Secret) — lettre de ligne (A, B, C...) +
 * numéro de colonne (1-based), dérivée de la position dans le tableau
 * `MOTION_CARDS` tel qu'envoyé par le serveur (aucun mélange côté client,
 * contrairement à MEMORY).
 *
 * Source : `PlayerDisplay.jsx:1996` (et copie identique `GamePage.jsx:837`).
 *
 * @param {number} index - position de la carte dans `MOTION_CARDS`
 * @param {number} cols - `getMotionGridCols(count)`
 * @returns {string}
 */
export function getMotionCardCoord(index, cols) {
  return `${String.fromCharCode(65 + Math.floor(index / cols))}${(index % cols) + 1}`
}

/**
 * Barème de points d'une carte selon sa difficulté (1-3 étoiles), avec repli
 * 1/3/5 si `MOTION_CONFIG` est absent (question ancienne, pas encore migrée).
 *
 * Source : `PlayerDisplay.jsx:1985-1992` (et copie identique
 * `GamePage.jsx:814-821`).
 *
 * @param {number} difficulty - `card.DIFFICULTY` (1, 2 ou 3)
 * @param {{POINTS_1_STAR?: number, POINTS_2_STAR?: number, POINTS_3_STAR?: number}|null|undefined} motionConfig - `question.MOTION_CONFIG`
 * @returns {number}
 */
export function getMotionCardPoints(difficulty, motionConfig) {
  if (motionConfig) {
    if (difficulty === 1) return motionConfig.POINTS_1_STAR ?? 1
    if (difficulty === 2) return motionConfig.POINTS_2_STAR ?? 3
    if (difficulty === 3) return motionConfig.POINTS_3_STAR ?? 5
  }
  return difficulty === 3 ? 5 : difficulty === 2 ? 3 : 1
}

/**
 * Gain prévisionnel d'une carte en barème `STARS_PRORATA` (#187, v7.1.0,
 * contrat §6.2) — carte MEMORY : les points de la carte (barème étoiles,
 * `getMotionCardPoints`) sont répartis au prorata des unités réalisées
 * (paires trouvées / total de paires). Affichage seul côté client (préviu
 * de `AnimMotionActions`, bouton "Terminer" de la ligne L2) — le serveur
 * reste **seule autorité** sur le calcul réellement crédité (`Units` et
 * `UnitsTotal` dérivés de son propre état, contrat §9.3) : cette fonction ne
 * fait jamais foi, elle anticipe seulement ce que verra l'animateur.
 *
 * 🔴 **Ordre des opérations normatif — multiplier AVANT de diviser**
 * (contrat §6.2) : `Math.floor(starsPoints * units / unitsTotal)`, jamais
 * `Math.floor(starsPoints / unitsTotal) * units` (qui vaut 0 dès que
 * `unitsTotal > starsPoints`, ex. une carte à 5 points / 8 paires — voir
 * `motionGrid.test.js`, cas nommé à l'identique côté Go,
 * `STARS_PRORATA_5points_8pairs`).
 *
 * @param {number} starsPoints - points de la carte au barème étoiles (`getMotionCardPoints`)
 * @param {number} units - unités réalisées (ex. paires trouvées)
 * @param {number} unitsTotal - unités réalisables au total (ex. nombre de paires de la carte)
 * @returns {number} points entiers, `0` si `unitsTotal <= 0` (garde contrat §6.2)
 */
export function computeStarsProrataPoints(starsPoints, units, unitsTotal) {
  if (!unitsTotal || unitsTotal <= 0) return 0
  return Math.floor((starsPoints * units) / unitsTotal)
}

/**
 * Mode Secret — la grille affiche des coordonnées plutôt que les thèmes tant
 * que la manche n'est pas terminée pour une carte (aucun étoile visible non
 * plus, la difficulté trahirait la carte).
 *
 * Source : `PlayerDisplay.jsx:1995` (et copie identique `GamePage.jsx:834`).
 *
 * @param {{MOTION_MEMORIZE_DURATION?: number}|null|undefined} question - gameState.question
 * @returns {boolean}
 */
export function isMotionSecretMode(question) {
  return (question?.MOTION_MEMORIZE_DURATION || 0) > 0
}

// ─────────────────────────────────────────────────────────────────────────────
// Barème par carte — POINTS_RULE (#190, v11.2.2, contrat question-types.md §6.2)
//
// Miroir JS de `Engine.motionCardPointsForOutcome` (engine.go) : UNE seule
// fonction de calcul pour tous les affichages (éditeur, anim, admin) — classe
// de bug R5 (divergence Go/JS du montant). Les cas de test sont NOMMÉS à
// l'identique côté Go (`engine_points_rule_190_test.go`) : FIXED_10_won,
// FIXED_10_lost, PER_UNIT_2x3, DEFAULT_SPEEDY_is_STARS,
// DEFAULT_MEMORY_is_STARS_PRORATA, DEFAULT_RAFALE_is_STARS_PRORATA,
// EXPLICIT_STARS_on_MEMORY_full_value, STARS_PRORATA_5points_8pairs.
//
// `card` = carte telle que sérialisée par le serveur (`TYPE`, `DIFFICULTY`,
// `POINTS_RULE: { MODE, VALUE }`) — jamais l'état camelCase de l'éditeur.
// ─────────────────────────────────────────────────────────────────────────────

/** Modes de barème possibles (contrat §6.1). */
export const POINTS_RULE_MODES = ['STARS', 'STARS_PRORATA', 'FIXED', 'PER_UNIT']

/**
 * Barème par défaut d'un type de carte (registre, `defaultPointsRule`) :
 * `STARS_PRORATA` pour les types à progression (MEMORY, RAFALE), `STARS`
 * sinon (SPEEDY, QCM — tout-ou-rien).
 *
 * @param {string} [type]
 * @returns {'STARS'|'STARS_PRORATA'}
 */
export function defaultPointsRuleMode(type) {
  return getQuestionTypeMeta(type).defaultPointsRule === 'STARS_PRORATA' ? 'STARS_PRORATA' : 'STARS'
}

/**
 * Modes proposés/acceptés pour un type de carte, le défaut en premier.
 * Binaire (SPEEDY, QCM) : `['STARS', 'FIXED']`. Progression (MEMORY, RAFALE) :
 * `['STARS_PRORATA', 'STARS', 'FIXED', 'PER_UNIT']`. Miroir de
 * `AllowedPointsRuleModes` (Go) — classification dérivée du registre.
 *
 * @param {string} [type]
 * @returns {string[]}
 */
export function allowedPointsRuleModes(type) {
  return defaultPointsRuleMode(type) === 'STARS_PRORATA'
    ? ['STARS_PRORATA', 'STARS', 'FIXED', 'PER_UNIT']
    : ['STARS', 'FIXED']
}

/**
 * Mode effectif d'une carte : `POINTS_RULE.MODE` explicite, sinon le défaut
 * de son TYPE courant (résolu à chaque appel, jamais figé sur la carte —
 * miroir d'`engine.go` `motionCardPointsForOutcome`).
 *
 * @param {{TYPE?: string, POINTS_RULE?: {MODE?: string}|null}|null|undefined} card
 * @returns {'STARS'|'STARS_PRORATA'|'FIXED'|'PER_UNIT'}
 */
export function resolvePointsRuleMode(card) {
  const explicit = card?.POINTS_RULE?.MODE
  if (explicit) return explicit
  return defaultPointsRuleMode(card?.TYPE)
}

/**
 * Montant crédité pour une carte selon sa règle — miroir de
 * `motionCardPointsForOutcome(card, units, unitsTotal)` (engine.go).
 *
 * - `FIXED`          : `VALUE` si `units > 0`, sinon 0
 * - `PER_UNIT`       : `VALUE × units`
 * - `STARS_PRORATA`  : `computeStarsProrataPoints(étoiles, units, unitsTotal)`
 * - `STARS` (défaut) : points du barème étoiles
 *
 * Affichage seul — le serveur reste seule autorité sur le crédit réel.
 *
 * @param {Object} card - carte serveur (`TYPE`, `DIFFICULTY`, `POINTS_RULE`)
 * @param {Object|null|undefined} motionConfig - `question.MOTION_CONFIG`
 * @param {{units?: number, unitsTotal?: number}} [outcome]
 * @returns {number}
 */
export function getMotionCardPointsForRule(card, motionConfig, { units = 1, unitsTotal = 0 } = {}) {
  const mode = resolvePointsRuleMode(card)
  const value = card?.POINTS_RULE?.VALUE || 0
  const stars = getMotionCardPoints(card?.DIFFICULTY || 1, motionConfig)
  switch (mode) {
    case 'FIXED':
      return units > 0 ? value : 0
    case 'PER_UNIT':
      return value * units
    case 'STARS_PRORATA':
      return computeStarsProrataPoints(stars, units, unitsTotal)
    default:
      return stars
  }
}

function unitWord(card, { plural }) {
  if (card?.TYPE === 'MEMORY') return plural ? 'paires trouvées' : 'paire'
  return plural ? 'bonnes réponses' : 'bonne rép.'
}

/**
 * Montant AFFICHABLE d'une carte avant jeu (sélection, bandeau, panneau
 * admin) : ce que la carte vaut « au maximum » / par unité, sans présumer
 * d'un résultat. `STARS` et `STARS_PRORATA` → points du barème étoiles
 * (prorata : préfixe « jusqu'à ») ; `FIXED` → `VALUE` ; `PER_UNIT` →
 * `VALUE` avec le suffixe d'unité (« / bonne rép. », « / paire »).
 *
 * @param {Object} card - carte serveur
 * @param {Object|null|undefined} motionConfig
 * @returns {{ points: number, prefix: string, suffix: string }} `prefix` : « jusqu'à  » en prorata
 */
export function getMotionCardPointsDisplay(card, motionConfig) {
  const mode = resolvePointsRuleMode(card)
  if (mode === 'FIXED') return { points: card?.POINTS_RULE?.VALUE || 0, prefix: '', suffix: '' }
  if (mode === 'PER_UNIT') {
    return { points: card?.POINTS_RULE?.VALUE || 0, prefix: '', suffix: ` / ${unitWord(card, { plural: false })}` }
  }
  const stars = getMotionCardPoints(card?.DIFFICULTY || 1, motionConfig)
  // Prorata : le montant plein est un maximum (maquette admin_question_editor §3 : « jusqu'à 5 pts »).
  return { points: stars, prefix: mode === 'STARS_PRORATA' ? "jusqu'à " : '', suffix: '' }
}

const pts = (n) => `${n} pt${n > 1 ? 's' : ''}`

/**
 * Libellé d'aide « → … » de l'éditeur (sans la flèche) : décrit la règle de
 * la carte en langage clair.
 *
 * - STARS         → « 3 pts (★★) »
 * - FIXED         → « 10 pts, quelle que soit la valeur ★ »
 * - STARS_PRORATA → « jusqu'à 5 pts, au prorata des paires trouvées (8 paires) »
 *                   / « jusqu'à 5 pts, au prorata des bonnes réponses »
 * - PER_UNIT      → « 2 pts × bonnes réponses » / « 2 pts × paires trouvées »
 *
 * `FIXED` / `PER_UNIT` sans `VALUE` valide : « N pts … ».
 *
 * @param {Object} card - carte (forme serveur) ; `POINTS_RULE.VALUE` peut être '' / absent
 * @param {Object|null|undefined} motionConfig
 * @param {{unitsTotal?: number, unitLabel?: string}} [opts]
 * @returns {string}
 */
export function describePointsRule(card, motionConfig, { unitsTotal, unitLabel } = {}) {
  const mode = resolvePointsRuleMode(card)
  const diff = card?.DIFFICULTY || 1
  const stars = getMotionCardPoints(diff, motionConfig)
  const rawValue = Number(card?.POINTS_RULE?.VALUE)
  const valueText = Number.isInteger(rawValue) && rawValue >= 1 && rawValue <= 999 ? pts(rawValue) : 'N pts'
  const unit = unitLabel || unitWord(card, { plural: true })
  switch (mode) {
    case 'FIXED':
      return `${valueText}, quelle que soit la valeur ★`
    case 'PER_UNIT':
      return `${valueText} × ${unit}`
    case 'STARS_PRORATA':
      return `jusqu'à ${pts(stars)}, au prorata des ${unit}${
        unitsTotal > 0 && card?.TYPE === 'MEMORY' ? ` (${unitsTotal} paires)` : ''
      }`
    default:
      return `${pts(stars)} (${'★'.repeat(diff)})`
  }
}
