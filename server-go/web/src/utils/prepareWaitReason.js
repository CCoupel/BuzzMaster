/**
 * prepareWaitReasons — motif(s) d'attente en phase `PREPARE` (#172/C2, plan
 * `_work/reports/plan-20260817-122307.md` §7 Bloc C). Renommée du singulier
 * `prepareWaitReason` (v11.1, retour QUALIF v11.1.0.5, #219/#236/#237) :
 * renvoie désormais TOUS les motifs actifs, pas seulement le premier trouvé.
 *
 * MIROIR client-side, en LECTURE SEULE, du prédicat serveur
 * `participantsConform(question, state)` (#172/B1, `engine.go`) et de la
 * condition de sortie de `PREPARE` (`AreAllTeamsReady() &&
 * participantsConform(...)`, #172/B2, `main.go:1456`). Ce fichier ne décide
 * RIEN — il n'est jamais consulté pour bloquer une action (le moteur reste
 * l'unique source de vérité, verrouillée côté serveur par `Engine.Start`,
 * #172/B4) : il sert uniquement à EXPLIQUER pourquoi la question reste en
 * `PREPARE`, sur `/admin` (GamePage.jsx) et `/anim` (AnimConductPanel.jsx).
 *
 * Table de règles identique à B1 (aucune branche par type en dehors de
 * cette table) :
 *   - SPEEDY, QCM, ARDOISE : déjà couvert par `AreAllTeamsReady` (≥1 équipe
 *     active) — toujours conforme ici, aucun changement de comportement.
 *   - MEMORY SOLO           : exactement une équipe sélectionnée.
 *   - MEMORY multi          : au moins deux équipes sélectionnées.
 *   - MEMOTION SOLO         : exactement une équipe sélectionnée (v8.0.0,
 *     #201 suivi — dev-backend SHA d3c6fb20, engine.go participantsConform
 *     durci pour être symétrique à MEMORY SOLO ; avant #201 : au moins une
 *     équipe, sans distinction SOLO/multi).
 *   - MEMOTION multi        : au moins deux équipes sélectionnées (#201
 *     suivi, mêmes SHA ci-dessus).
 *   - RAFALE SOLO           : exactement une équipe sélectionnée (v8.0.0,
 *     #201, retour QUALIF — dev-backend SHA e2917395/d3c6fb20, engine.go
 *     participantsConform durci pour être symétrique à MEMORY SOLO ;
 *     avant #201 : toujours conforme, aucune restriction).
 *   - RAFALE multi          : au moins deux équipes sélectionnées (#201,
 *     durci depuis "au moins une" — #199, dev-backend SHA 393c6dc7 —
 *     mêmes SHA #201 ci-dessus. `question.Category`/`RafaleDifficulty`
 *     absents/invalides restent volontairement HORS PÉRIMÈTRE ici, déjà
 *     couverts côté admin par `rafaleBlocked`/RafalePoolAlert, contrat
 *     §7.2 — dupliquer cette partie créerait deux messages concurrents
 *     pour la même cause).
 *   - Type inconnu          : permissif par défaut.
 */

/** @param {{READY?: boolean|string}} team */
export function isTeamReady(team) {
  return team?.READY === true || team?.READY === 'TRUE'
}

/**
 * @param {{TYPE?: string, MEMORY_MODE?: string, MOTION_MODE?: string, RAFALE_MODE?: string}|null} question
 * @param {string[]} participating - sélection courante (MEMORY_PARTICIPATING_TEAMS,
 *   MEMOTION_PARTICIPATING_TEAMS ou RAFALE_PARTICIPATING_TEAMS selon le type)
 */
export function participantsConform(question, participating) {
  const type = question?.TYPE
  const count = (participating || []).length
  if (type === 'MEMORY') {
    const isSolo = !question.MEMORY_MODE || question.MEMORY_MODE === 'SOLO'
    return isSolo ? count === 1 : count >= 2
  }
  if (type === 'MEMOTION') {
    const isSolo = !question.MOTION_MODE || question.MOTION_MODE === 'SOLO'
    return isSolo ? count === 1 : count >= 2
  }
  if (type === 'RAFALE') {
    const isSolo = !question.RAFALE_MODE || question.RAFALE_MODE === 'SOLO'
    return isSolo ? count === 1 : count >= 2
  }
  // SPEEDY, QCM, ARDOISE, type inconnu — déjà couvert par AreAllTeamsReady
  // (≥1 équipe active) ou sans règle : permissif.
  return true
}

// Addendum média indisponible (v11.1, #219/#236/#237, contrat sound.md
// §10.8) — motifs de la branche son de `participantsConform` côté serveur,
// diffusés par `GAME.QUESTION_SOUND_UNAVAILABLE` ("" | "DISABLED" | "OUTPUT"
// | "FILE"). Ne concerne que les questions portant un son (le serveur sort
// immédiatement de sa branche sinon) — SPEEDY/QCM/ARDOISE dans le périmètre
// actuel, mais aucune condition de type ici : le champ est structurellement
// commun à tous les types (plan §0.3 du lot v11.1 initial), une éventuelle
// question MEMORY/MEMOTION à son futur serait couverte sans modification.
// Chaque motif NOMME LE REMÈDE réel — jamais une étiquette générique — pour
// que l'utilisateur sache quoi faire, pas seulement que quelque chose bloque.
const SOUND_UNAVAILABLE_REASON_LABELS = {
  DISABLED: {
    short: 'son désactivé',
    long: 'Les sons sont désactivés dans la configuration. Les réactiver demande un redémarrage du serveur.',
  },
  OUTPUT: {
    short: 'pas de sortie audio',
    long: "Aucune sortie audio n'a pu être ouverte au démarrage du serveur. Brancher l'enceinte puis redémarrer le serveur.",
  },
  FILE: {
    short: 'son introuvable',
    long: 'Le fichier son de cette question est introuvable ou invalide sur le serveur.',
  },
}

// Libellés courts (#166, style "à suivre"/"attendu"/"optionnel" —
// AnimConductPanel.anim-conduct-btn-sub, tablette, place limitée) et
// libellés complets (régie, memory-selector-label, plus de place).
function participantsReasonLabel(question, { short } = {}) {
  const type = question?.TYPE
  if (type === 'MEMORY') {
    const isSolo = !question.MEMORY_MODE || question.MEMORY_MODE === 'SOLO'
    if (short) return isSolo ? '1 équipe' : '2 équipes'
    return isSolo ? 'sélectionnez une équipe' : 'sélectionnez au moins deux équipes'
  }
  if (type === 'MEMOTION') {
    const isSolo = !question.MOTION_MODE || question.MOTION_MODE === 'SOLO'
    if (short) return isSolo ? '1 équipe' : '2 équipes'
    return isSolo ? 'sélectionnez une équipe' : 'sélectionnez au moins deux équipes'
  }
  if (type === 'RAFALE') {
    const isSolo = !question.RAFALE_MODE || question.RAFALE_MODE === 'SOLO'
    if (short) return isSolo ? '1 équipe' : '2 équipes'
    return isSolo ? 'sélectionnez une équipe' : 'sélectionnez au moins deux équipes participantes'
  }
  return null
}

/**
 * prepareWaitReasons — TOUS les motifs de blocage actifs en phase `PREPARE`,
 * simultanément (évolution UX, retour QUALIF v11.1.0.5, #219/#236/#237).
 *
 * Remplace l'ancienne `prepareWaitReason` (singulier), qui ne retournait que
 * le motif de plus haute priorité (buzzers, PUIS participants, PUIS son) —
 * un utilisateur avec deux problèmes en même temps (ex. buzzers pas prêts
 * ET son indisponible) ne voyait que le premier, et découvrait le second
 * seulement après avoir corrigé le premier. Ici les trois branches
 * s'accumulent dans un tableau au lieu de `return` dès la première trouvée.
 *
 * Choix de présentation (pas de nouveau composant de liste) : chaque appelant
 * assemble le tableau avec le séparateur `' · '` déjà utilisé partout où ce
 * motif s'affichait (GamePage.jsx memory-selector-label, sous-libellé
 * AnimConductPanel) — un tableau vide donne `[].join(' · ') === ''`, donc le
 * même test `reason &&`/`reasons.length &&` continue de fonctionner sans
 * changer la mise en page existante (toujours une seule ligne de texte, pas
 * une liste à puces qui aurait demandé un nouveau composant et plus
 * d'espace vertical que la tablette `/anim` ou les encarts `/admin`
 * n'en ont).
 *
 * @param {string} phase - gameState.phase
 * @param {{TYPE?: string, MEMORY_MODE?: string}|null} question - gameState.question
 * @param {Array<{READY?: boolean|string}>} activeTeams - équipes ayant ≥1 buzzer
 *   assigné (même filtre que l'affichage, cf. `AreAllTeamsReady` — "Empty
 *   teams are ignored, matching the frontend display filter")
 * @param {{MEMORY_PARTICIPATING_TEAMS?: string[], MEMOTION_PARTICIPATING_TEAMS?: string[], QUESTION_SOUND_UNAVAILABLE?: string}} gameState
 * @param {{short?: boolean}} [opts] - `short: true` pour les libellés tablette
 *   (AnimConductPanel, espace contraint) ; sinon libellés complets (régie).
 * @returns {string[]} tous les motifs actifs, dans l'ordre buzzers → participants
 *   → son ; tableau vide (jamais `null`) hors `PREPARE` ou si rien à expliquer.
 */
export function prepareWaitReasons(phase, question, activeTeams, gameState, opts = {}) {
  if (phase !== 'PREPARE') return []

  const reasons = []

  const buzzersWaiting = (activeTeams || []).some(t => !isTeamReady(t))
  if (buzzersWaiting) reasons.push(opts.short ? 'buzzers' : 'Buzzers en attente')

  const participating = question?.TYPE === 'MEMOTION'
    ? (gameState?.MEMOTION_PARTICIPATING_TEAMS || [])
    : question?.TYPE === 'RAFALE'
      ? (gameState?.RAFALE_PARTICIPATING_TEAMS || [])
      : (gameState?.MEMORY_PARTICIPATING_TEAMS || [])

  if (!participantsConform(question, participating)) {
    const label = participantsReasonLabel(question, opts)
    if (label) reasons.push(label)
  }

  // Addendum média indisponible (v11.1, #219/#236/#237) — dernière branche :
  // `GAME.QUESTION_SOUND_UNAVAILABLE` n'est jamais renseigné par le serveur
  // pour une question sans son (sortie immédiate de sa propre branche,
  // contrat §10.8), donc aucune garde de type n'est nécessaire ici non plus.
  const soundReason = SOUND_UNAVAILABLE_REASON_LABELS[gameState?.QUESTION_SOUND_UNAVAILABLE]
  if (soundReason) {
    reasons.push(opts.short ? soundReason.short : soundReason.long)
  }

  return reasons
}
