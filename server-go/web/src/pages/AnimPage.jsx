import { useEffect, useMemo, useRef } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import NoSleep from 'nosleep.js'
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'
import useDoubleTap from '../hooks/useDoubleTap'
import { categoryMeta } from '../utils/categoryUtils'
import { sortTeamsByBuzzOrder, sortTeamsByRafaleCounter, getRankBadge, formatReactionTime } from '../utils/buzzOrder'
import { sortArdoiseEntries } from '../utils/ardoiseOrder'
import { resolvePointsAward, resolvePointsTarget, calcQcmTeamAward, rafaleCounterForTeam, calcRafaleTeamAward } from '../utils/pointsAward'
import { isRevealed } from '../utils/phaseRules'
import { prepareWaitReason } from '../utils/prepareWaitReason'
import { getQuestionTypeMeta } from '../utils/questionTypeMeta'
import { resolveHostContext } from '../utils/hostContext'
import { getTypeState } from '../utils/typeState'
import { getMotionCardPoints } from '../utils/motionGrid'
import { getPhaseBadge } from '../utils/phaseBadge'
import { canAwardPoints } from '../utils/canAwardPoints'
import { QCM_COLORS } from '../constants/colors'
import Timer from '../components/Timer'
import AnimTeamCard from '../components/AnimTeamCard'
import AnimConductPanel from '../components/AnimConductPanel'
import AnimAnswerZone from '../components/AnimAnswerZone'
import AnimCreditControl from '../components/AnimCreditControl'
import AnimArdoiseList from '../components/AnimArdoiseList'
import RafaleTimers from '../components/RafaleTimers'
import './AnimPage.css'
import '../styles/entracte.css'

// #158/F3 — phases où la liste ARDOISE remplace les cartes équipe. Mêmes
// phases que les critères d'acceptation du plan (STARTED/PAUSED/STOPPED/
// REVEALED) — les copies arrivent en direct dès STARTED, PAUSED inclus
// (l'animateur peut suspendre le chrono sans perdre la vue des réponses).
const ARDOISE_LIST_PHASES = ['STARTED', 'PAUSED', 'STOPPED', 'REVEALED']

// Phases pendant lesquelles l'ordre de buzz (rang, réordonnancement) est
// actif — même règle que GamePage.jsx (utils/buzzOrder.js).
const BUZZ_ORDER_PHASES = ['STARTED', 'PAUSED', 'REVEALED', 'STOPPED']
// Rang affiché uniquement pendant ces phases (pas en STOPPED, même choix
// que TeamCard.jsx — le badge médaille perd son sens une fois arrêté).
const RANK_BADGE_PHASES = ['STARTED', 'PAUSED', 'REVEALED']
// Crédit actif uniquement une fois la question arrêtée (GamePage.jsx:1077).
const CREDIT_PHASES = ['STOPPED', 'REVEALED']

// #166/F3 — libellés des modes de tour MEMORY/MEMOTION (D5, "puce mode de
// tour"). Affichée même si ces modes ne sont pas encore conduits depuis
// /anim — cf. modèle Go (models.go:224-230, MemoryMode).
const MEMORY_MODE_LABEL = {
  SOLO: 'Solo',
  CHACUN_SON_TOUR: 'Chacun son tour',
  TANT_QUE_JE_GAGNE: 'Tant que je gagne',
}

const STATUS_LABEL = {
  connected: 'Connecté',
  connecting: 'Connexion...',
  disconnected: 'Déconnecté',
}

// #184/B-F3 — champs `TypedContent` (question-types.md §2), partagés SOUS LE
// MÊME NOM entre `Question` et `MotionCard` (embarquement à plat des deux
// côtés). Source unique de la liste : `internal/game/models.go` (`TypedContent`,
// dev-backend B-B1). Ne PAS y ajouter `ANSWER` : ce champ est bien dans
// `TypedContent`, mais une `MotionCard` SPEEDY le porte sous un nom différent
// (`ANSWER_TEXT`, champ historique propre à la carte, contrat §3) — c'est
// pourquoi `cardToSyntheticQuestion` le traite à part, jamais via cette liste.
const TYPED_CONTENT_FIELDS = [
  'QCM_ANSWERS', 'QCM_CORRECT', 'QCM_HINTS_ENABLED',
  'QCM_HINT_THRESHOLD_1', 'QCM_HINT_THRESHOLD_2', 'QCM_PENALTY_1', 'QCM_PENALTY_2',
  'ARDOISE_KEYBOARD_TYPE',
  'MEMORY_PAIRS', 'MEMORY_CONFIG', 'MEMORY_MODE',
]

/**
 * cardToSyntheticQuestion — adaptateur généralisé « carte → question
 * synthétique » (#184/B-F3, question-types.md §2/§4). Remplace le mapping ad
 * hoc par champ (`{...question, ANSWER: card?.ANSWER_TEXT}`, avant #184) par
 * une copie des champs `TypedContent` présents sur la carte, sous le même
 * nom : c'est ce qui permet à un composant de type (`AnimAnswerZone`,
 * `AnimQcmOptions`…) de traiter l'hôte carte exactement comme l'hôte
 * question, sans variante — la même généralisation que doivent exploiter
 * #185/#186/#187 sans reformer cet adaptateur.
 *
 * @param {Object|null} question - gameState.question (hôte MEMOTION)
 * @param {Object|null} card - carte MEMOTION active (peut être `null` hors
 *   SELECTED/QUESTION/REVEAL — l'objet `question` est alors renvoyé tel quel)
 * @returns {Object|null}
 */
function cardToSyntheticQuestion(question, card) {
  if (!card) return question
  const typedContent = {}
  TYPED_CONTENT_FIELDS.forEach(field => {
    if (card[field] !== undefined) typedContent[field] = card[field]
  })
  return {
    ...question,
    TYPE: card.TYPE || 'SPEEDY',
    ANSWER: card.ANSWER_TEXT || '',
    ...typedContent,
  }
}

/**
 * AnimPage — interface animateur (`/anim`, #155/#156/#157/#163/#165/#166).
 *
 * Gabarit à quatre zones (#166/F6, plan
 * `_work/reports/plan-20260815-144925.md`) :
 *   - Zone contexte (bandeau) : ligne méta (statut, catégorie, type,
 *     progression n/total, #ID, options conditionnelles, points — F3),
 *     énoncé, zone réponse permanente (AnimAnswerZone, F10), chronomètre en
 *     colonne dédiée (F12).
 *   - Zone conduite (AnimConductPanel, #166/F5) : cinq lignes permanentes
 *     (L1 cinq gestes globaux, "à suivre", L2 grille QCM/réservé, L3/L4
 *     réservées) — le composant calcule lui-même l'état de chaque geste
 *     depuis `phase`/`question` (utils/phaseRules.js), AnimPage ne lui
 *     passe plus de booléens précalculés.
 *   - Zone équipes (AnimTeamCard enrichie, #156/#157) — INCHANGÉE par
 *     #166 : seule sa place dans la grille change (AnimPage.css).
 *   - Bande régie réservée (#166/F8, #167) — pleine largeur, vide.
 *
 * Tablette paysage, pas de Navbar régie (App.jsx ne l'affiche que sur
 * /admin/* désormais — #155/F2). Connecté sur /ws/anim (ClientTypeAnim,
 * capacité réduite) via GameProvider (App.jsx).
 */
export default function AnimPage() {
  const {
    status,
    gameState,
    teams,
    bumpers,
    nextQuestion,
    questionPosition,
    awardedTeams,
    creditPoints,
    // RAFALE (v8.0.0, #16/#107, contrat rafale.md §5.1/§5.2)
    rafaleAnswer,
    rafaleValidate,
    rafaleInvalidate,
    // Défauts défensifs (repos, jamais actif) — le hook réel (useWebSocket.js)
    // fournit toujours ces valeurs, mais certains mocks de test n'ont pas
    // encore été mis à jour avec les champs #167 (test-writer, en parallèle).
    regieMessage = { ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: '' },
    clearRegieMessage = () => {},
    startGame,
    stopGame,
    pauseGame,
    continueGame,
    revealAnswer,
    selectQuestion,
    setTeamPoints,
    setBumperPoints,
    flipMemoryCard,
    selectMotionCard,
    flipMotionCard,
    stopMotionTimer,
    revealMotionCard,
    doneMotionCard,
  } = useGame()

  // #176 (F5) — acquittement de la consigne régie par double-tap sur toute
  // la zone du message (remplace le bouton « Vu », trop petit pour un usage
  // tactile debout). Un tap unique n'a aucun effet (AC13).
  const { handlers: regieDoubleTapHandlers } = useDoubleTap(clearRegieMessage)

  // Veille écran — reprend le motif PlayerDisplay.jsx:912-921 : wake lock
  // natif (HTTPS) si disponible, repli NoSleep.js sinon. Le serveur n'expose
  // aucun TLS, donc ce repli est le chemin NOMINAL sur une tablette
  // animateur, pas une option de secours.
  const wakeLockRef = useRef(null)
  const noSleepRef = useRef(null)
  useEffect(() => {
    const startWakeLock = async () => {
      if ('wakeLock' in navigator) {
        try {
          wakeLockRef.current = await navigator.wakeLock.request('screen')
          return
        } catch (_) {
          // Repli NoSleep.js ci-dessous.
        }
      }
      if (!noSleepRef.current) noSleepRef.current = new NoSleep()
      if (!noSleepRef.current.isEnabled) noSleepRef.current.enable()
    }
    startWakeLock()

    return () => {
      if (wakeLockRef.current) {
        wakeLockRef.current.release()
        wakeLockRef.current = null
      }
      if (noSleepRef.current?.isEnabled) {
        noSleepRef.current.disable()
        noSleepRef.current = null
      }
    }
  }, [])

  const { categories: apiCategories } = useCategories()
  const customCategories = useMemo(() => apiCategories.filter(c => c.isCustom), [apiCategories])

  const question = gameState.question
  const categoryInfo = question?.CATEGORY ? categoryMeta(question.CATEGORY, customCategories) : null
  // #166/F3 — icône + libellé de type (D4), repli SPEEDY géré par
  // getQuestionTypeMeta lui-même (même convention que #163).
  const typeMeta = getQuestionTypeMeta(question?.TYPE)
  // #166 — verrou de la zone réponse (F10) ET du marquage QCM en L2 (F9),
  // seule source : phaseRules.isRevealed. Remplace l'inline
  // `gameState.phase === 'REVEALED'` répété à trois endroits avant #166.
  const revealed = isRevealed(gameState.phase)
  // #171/F2 — badge de phase déplacé de la colonne chrono vers la ligne
  // réponse (voir utils/phaseBadge.js).
  const phaseBadge = getPhaseBadge(gameState.phase)
  // #184/B-F2, B-F3 — contexte d'hôte normalisé (utils/hostContext.js,
  // question-types.md §4), calculé UNE FOIS ici et transmis à
  // AnimConductPanel : c'est ce qui lui permet de nourrir les composants de
  // type (AnimMemoryGrid) en `playable`/`revealed` plutôt qu'en `phase`, et de
  // résoudre le type de l'hôte courant (question, ou carte MEMOTION active en
  // QUESTION/REVEAL) sans jamais relire `gameState.phase`/`MEMOTION_SUBPHASE`
  // lui-même.
  const hostContext = resolveHostContext(gameState)
  // #185/C-F1 — état d'indices QCM de l'hôte COURANT (question ou carte
  // MEMOTION active), résolu une seule fois via l'accesseur unique posé en
  // B-F1 (question-types.md §5.3) — jusqu'ici calculé mais jamais consommé.
  // Remplace la lecture directe de `gameState.qcmInvalidated` (qui ne
  // couvrait que l'hôte question) dans la prop transmise à
  // `AnimConductPanel` ci-dessous — neutre pour l'hôte question (même
  // valeur), correct pour l'hôte carte QCM.
  const typeState = getTypeState(gameState, hostContext)
  // #187 (v7.1.0) — flipMemoryCard porte désormais la portée de carte
  // (`CardScope`, contrat question-types.md §9) quand une carte MEMOTION est
  // active : `hostContext.cardId` vaut "" hors manche MEMOTION (repli neutre,
  // comportement inchangé) et l'ID de la carte active en SELECTED/QUESTION/
  // REVEAL. Lié UNE FOIS ici — `AnimConductPanel`/`AnimMemoryGrid` ne
  // connaissent jamais `MOTION_CARD_ID` eux-mêmes (même discipline que
  // `hostContext`/`typeState` ci-dessus).
  const handleFlipMemoryCard = (cardId) =>
    hostContext.cardId ? flipMemoryCard(cardId, hostContext.cardId) : flipMemoryCard(cardId)

  // #160/F8 — MEMOTION, sur le modèle exact de isMemoryQuestion (#159, plus
  // bas dans le fichier, zone équipes). La manche entière se joue en phase
  // STARTED (les 5 sous-phases sont dans MEMOTION_SUBPHASE, pas dans
  // gameState.phase) — aucun impact sur CREDIT_PHASES ci-dessus : AC7 tient
  // par construction, pas par garde ajoutée ici.
  const isMemotionQuestion = question?.TYPE === 'MEMOTION'
  const motionSubphase = gameState.MEMOTION_SUBPHASE
  const motionCards = question?.MOTION_CARDS || []
  const selectedMotionCard = isMemotionQuestion
    ? motionCards.find(c => c.ID === gameState.MEMOTION_SELECTED) || null
    : null
  // #187 cycle 4 (F2) — délibérément la valeur ÉTOILES PLEINE, PAS le
  // prorata STARS_PRORATA (contrairement à `AnimConductPanel.jsx`'s propre
  // `motionCardPoints`, calcul interne et local à ce composant, non partagé
  // avec celui-ci). Motif : ce libellé n'apparaît QUE pendant la sous-phase
  // SELECTED (voir motionStatement ci-dessous — en QUESTION/REVEAL il est
  // remplacé par le texte de la carte), c'est-à-dire AVANT toute paire
  // trouvée, quand la question posée est « combien vaut cette carte au
  // maximum ? », pas « combien vais-je créditer maintenant ? ». Appliquer le
  // prorata ici afficherait 0pt sur une carte MEMORY tout juste sélectionnée
  // (`matchedPairs` encore vide), ce qui serait FAUX — la carte reste
  // intégralement gagnable. Les deux affichages (ce libellé en SELECTED, le
  // bouton de crédit d'AnimConductPanel en REVEAL) ne se chevauchent jamais
  // dans le temps, donc pas de risque de lire deux montants contradictoires
  // pour la même carte au même instant.
  const motionCardPoints = selectedMotionCard
    ? getMotionCardPoints(selectedMotionCard.DIFFICULTY || 1, question?.MOTION_CONFIG)
    : 0
  // Zone contexte — l'énoncé suit la carte en cours plutôt que rester figé
  // sur l'énoncé de la question MEMOTION (qui n'a pas de sens ici, une
  // question MEMOTION porte plusieurs cartes) : thème + points en SELECTED,
  // texte de la carte en QUESTION/REVEAL, rappel générique en MEMORIZE/GRID.
  const motionStatement = isMemotionQuestion
    ? (motionSubphase === 'SELECTED' && selectedMotionCard
        ? `Carte « ${selectedMotionCard.RECTO_THEME} » — ${'★'.repeat(selectedMotionCard.DIFFICULTY || 1)} · ${motionCardPoints}pt${motionCardPoints > 1 ? 's' : ''}`
        : (motionSubphase === 'QUESTION' || motionSubphase === 'REVEAL') && selectedMotionCard?.QUESTION_TEXT
          ? selectedMotionCard.QUESTION_TEXT
          : motionSubphase === 'MEMORIZE'
            ? 'Mémorisez la grille'
            : 'Choisissez une carte')
    : null
  // Zone réponse (arbitrage n°1 du GATE 2, #160) — AnimAnswerZone lit
  // `question.ANSWER`/`question.TYPE`/`question.QCM_*`, inexistants en
  // MEMOTION (vide toute la manche sans ce contournement) : on lui passe un
  // objet question dérivé où ces champs pointent la CARTE en cours. Réutilise
  // la mécanique AnimAnswerZone EXISTANTE (flou + révélation par pression)
  // sans y toucher.
  // #184/B-F3 — adaptateur généralisé (question-types.md §2/§4) : plus un
  // mapping ad hoc par champ (avant #184 : seul `ANSWER` était recopié), mais
  // une copie de tous les champs `TypedContent` présents sur la carte, SOUS
  // LE MÊME NOM qu'ils portent déjà sur `Question` — c'est précisément ce que
  // permet l'embarquement à plat partagé du contrat. `TYPE` est également
  // recopié (la carte peut être QCM, plus seulement SPEEDY) : c'est ce qui
  // débloquera #185/#186/#187 sans reformer cet adaptateur. `ANSWER_TEXT` →
  // `ANSWER` reste un mapping dédié : ce sont des champs SPEEDY historiques
  // propres à `MotionCard` (contrat §3), pas des champs `TypedContent`
  // partagés sous le même nom entre `Question` et `MotionCard`.
  // RAFALE (v8.0.0, #16/#107, contrat rafale.md §2.1/§2.3) — `question`
  // (TYPE RAFALE) ne porte ni QUESTION ni ANSWER : l'énoncé vient de
  // `RAFALE_CURRENT_QUESTION` (GameState, sans réponse — §4), la réponse de
  // l'action dédiée `RAFALE_ANSWER` (`rafaleAnswer`, admin+anim
  // uniquement — §2.3, jamais dans GameState, fuite ardoise_leak_128).
  const isRafaleQuestion = question?.TYPE === 'RAFALE'
  const rafaleCurrentQuestion = gameState.RAFALE_CURRENT_QUESTION || {}
  // Garde anti-obsolescence — même discipline que AWARDED_TEAMS
  // (useWebSocket.js) : un RAFALE_ANSWER dont l'ID ne correspond plus à la
  // question RAFALE actuellement affichée est ignoré plutôt qu'appliqué,
  // pour ne jamais laisser transparaître, même un instant, la réponse
  // d'UNE question sous l'énoncé de la SUIVANTE.
  const rafaleAnswerValue = (rafaleAnswer && rafaleAnswer.ID === rafaleCurrentQuestion.ID)
    ? rafaleAnswer.ANSWER
    : ''
  // NEXT (#202, contrat §13.3) — pré-tirage de la question suivante,
  // transporté par le MÊME `RAFALE_ANSWER` que la réponse ci-dessus (pas
  // de canal séparé, §13.3). Réutilise EXACTEMENT la garde anti-obsolescence
  // déjà écrite pour `rafaleAnswerValue` juste au-dessus — surtout ne pas en
  // recréer une seconde (rappel explicite de la tâche 13/du contrat) :
  // un `NEXT` dont l'ID ne correspond plus à la question RAFALE actuellement
  // affichée ne doit jamais apparaître sous une question à laquelle il
  // n'appartient pas. `NEXT === null` est une information à part entière
  // (§13.5, "aucune question suivante" / fin de réservoir) — distincte de
  // "pas encore reçu" (rafaleAnswer absent ou périmé), d'où `undefined` en
  // repli plutôt que `null` : `AnimRafaleQuestion` doit pouvoir distinguer
  // "rien à afficher pour l'instant" de "dernière question du réservoir".
  const rafaleNext = (rafaleAnswer && rafaleAnswer.ID === rafaleCurrentQuestion.ID)
    ? rafaleAnswer.NEXT
    : undefined
  const rafaleNextCatMeta = rafaleNext ? categoryMeta(rafaleNext.CATEGORY, customCategories) : null
  // RÉVISION 2026-08-28 (maquette rafale-v8.html §9.2) — RAFALE affiche
  // désormais question ET réponse ensemble dans un encart coloré équipe
  // dédié, sans passer par AnimAnswerZone (masquage hold-to-peek retiré
  // pour ce type — cf. rendu dans .anim-zone-context ci-dessous).
  const rafaleCatMeta = categoryMeta(rafaleCurrentQuestion.CATEGORY, customCategories)
  const rafaleCurrentTeamColorArr = gameState.RAFALE_CURRENT_TEAM_COLOR
  const rafaleCurrentTeamCss = Array.isArray(rafaleCurrentTeamColorArr) && rafaleCurrentTeamColorArr.length === 3
    ? `rgb(${rafaleCurrentTeamColorArr.join(',')})`
    : 'var(--error)'
  // Zone réponse — réutilise AnimAnswerZone TELLE QUELLE (adaptateur en
  // objet "question" synthétique, même patron que MEMOTION ci-dessus) :
  // ANSWER pointe la valeur ci-dessus, jamais gameState.question.ANSWER
  // (inexistant pour ce type). RAFALE ne passe plus par ce chemin (voir
  // ci-dessus) mais answerZoneQuestion reste utilisé par les autres types.
  const answerZoneQuestion = isMemotionQuestion
    ? cardToSyntheticQuestion(question, selectedMotionCard)
    : question

  // #166/F3 — options conditionnelles de la ligne méta (D5) : cible des
  // points, indices QCM activés, mode de tour MEMORY/MEMOTION (affiché même
  // si ces modes ne sont pas encore conduits depuis /anim — la donnée
  // existe côté modèle, cf. models.go:281).
  const optionChips = useMemo(() => {
    if (!question) return []
    const chips = []
    if (question.POINTS_TARGET === 'TEAM') chips.push({ key: 'target', label: 'Équipe' })
    else if (question.POINTS_TARGET === 'PLAYER') chips.push({ key: 'target', label: 'Individuel' })
    if (question.TYPE === 'QCM' && question.QCM_HINTS_ENABLED) chips.push({ key: 'hints', label: 'Indices' })
    if ((question.TYPE === 'MEMORY' || question.TYPE === 'MEMOTION') && question.MEMORY_MODE) {
      chips.push({ key: 'turn', label: MEMORY_MODE_LABEL[question.MEMORY_MODE] || question.MEMORY_MODE })
    }
    return chips
  }, [question])

  const handleStart = () => {
    const time = parseInt(gameState.question?.TIME) || 30
    // MAJEUR-1 — creditPoints (CREDIT_POINTS) est l'équivalent serveur de
    // pointsInput sur /admin, potentiellement ajusté depuis question.POINTS
    // (ex. manche bonus) : c'est la valeur à jour, pas la valeur brute de la
    // question. En pratique ce paramètre n'a aujourd'hui aucun effet côté
    // serveur (StartPayload.POINTS n'est pas décodé, cf. rapport backend
    // MAJEUR-1) mais autant transmettre la bonne valeur plutôt que réintroduire
    // le même écart que celui corrigé pour le crédit.
    startGame(time, creditPoints || 1)
  }

  // Zone équipes — mêmes équipes que /admin (au moins un joueur assigné —
  // règle de base #45, GamePage.jsx:135), triées par ordre de buzz pendant
  // les phases actives (utils/buzzOrder.js — même règle que GamePage.jsx,
  // #156/F6). INCHANGÉ par #166 (seule la grid-area change, AnimPage.css).
  const displayTeams = useMemo(() => {
    const teamsWithPlayers = new Set(
      Object.values(bumpers)
        .filter(b => b.TEAM)
        .map(b => b.TEAM)
    )
    const list = Object.entries(teams)
      .filter(([name]) => teamsWithPlayers.has(name))
      .map(([name, data]) => ({ name, ...data }))
    // RAFALE (v8.0.0, #16/#199, contrat rafale.md §6.1, tâche 34) — même
    // classement par compteur que GamePage.jsx (utils/buzzOrder.js,
    // mutualisé). sortTeamsByRafaleCounter est un no-op hors type RAFALE.
    return sortTeamsByRafaleCounter(
      sortTeamsByBuzzOrder(list, gameState.phase),
      question,
      gameState.RAFALE_TEAM_COUNTERS,
      gameState.RAFALE_TEAM_BEST
    )
  }, [teams, bumpers, gameState.phase, question, gameState.RAFALE_TEAM_COUNTERS, gameState.RAFALE_TEAM_BEST])

  // #172/C2 — motif d'attente PREPARE, passé à AnimConductPanel (repli du
  // bouton LANCER, style "à suivre" #166 déjà en place, aucun nouveau
  // badge/CSS). `short: true` — sub-label du bouton, place limitée (F7).
  const waitReason = useMemo(
    () => prepareWaitReason(gameState.phase, question, displayTeams, gameState, { short: true }),
    [gameState, question, displayTeams]
  )

  // #158/F3 — mode ARDOISE : liste des copies à la place des cartes équipe.
  // Filtre équipes à joueur virtuel, parité #93 (même règle que
  // GamePage.jsx vplayerTeamNames) — pas "au moins un joueur" comme
  // displayTeams ci-dessus, une équipe SPEEDY sans VJoueur n'a pas de
  // copie possible.
  const isArdoise = question?.TYPE === 'ARDOISE'
  const showArdoiseList = isArdoise && ARDOISE_LIST_PHASES.includes(gameState.phase)
  const ardoiseTeams = useMemo(() => {
    if (!showArdoiseList) return []
    const vplayerTeamNames = new Set(
      Object.values(bumpers)
        .filter(b => b.IS_VPLAYER)
        .map(b => b.TEAM)
        .filter(Boolean)
    )
    return Object.entries(teams)
      .filter(([name]) => vplayerTeamNames.has(name))
      .map(([name, data]) => ({ name, ...data }))
  }, [teams, bumpers, showArdoiseList])
  const ardoiseEntries = useMemo(
    () => sortArdoiseEntries(ardoiseTeams, gameState.ARDOISE_ANSWERS),
    [ardoiseTeams, gameState.ARDOISE_ANSWERS]
  )
  // #158/F3 — cible TOUJOURS l'équipe pour ARDOISE, mirror exact du bouton
  // ARDOISE de /admin (GamePage.jsx : setTeamPoints(teamName, defaultPts)
  // direct, sans résolution PLAYER/bumper via POINTS_TARGET).
  const handleArdoiseCredit = (teamName, points) => setTeamPoints(teamName, points)

  const showRankBadge = RANK_BADGE_PHASES.includes(gameState.phase)
  const showBuzzOrder = BUZZ_ORDER_PHASES.includes(gameState.phase)
  const creditEnabled = CREDIT_PHASES.includes(gameState.phase)
  const isQcmWithHints = gameState.question?.TYPE === 'QCM' && gameState.question?.QCM_HINTS_ENABLED

  // Bumpers groupés par équipe — évite un filtre O(bumpers) répété par
  // équipe à chaque rendu ; consommé par le crédit QCM (T3, #157) et
  // l'affichage de la réponse QCM (T4, #157).
  const bumpersByTeam = useMemo(() => {
    const grouped = {}
    Object.entries(bumpers).forEach(([mac, bumper]) => {
      if (!bumper.TEAM) return
      if (!grouped[bumper.TEAM]) grouped[bumper.TEAM] = []
      // `mac` ajouté au bumper brut : garde ANSWER_COLOR/HINTS_AT_BUZZ/TIME
      // directement accessibles (forme attendue par calcQcmTeamAward) tout
      // en gardant l'identifiant nécessaire à setBumperPoints.
      grouped[bumper.TEAM].push({ ...bumper, mac })
    })
    return grouped
  }, [bumpers])

  // Crédit — cible (équipe/joueur) globale, POINTS_TARGET ne dépend pas de
  // l'équipe (GamePage.jsx:404-411). Base = creditPoints (CREDIT_POINTS,
  // MAJEUR-1) — l'équivalent serveur de pointsInput sur /admin, PAS
  // question.POINTS brut : /admin crédite pointsInput, potentiellement
  // ajusté après sélection (ex. manche bonus), et SET_CREDIT_POINTS/
  // CREDIT_POINTS existent précisément pour que /anim voie cet ajustement.
  const creditTarget = resolvePointsTarget(gameState.question)

  // #157/T3 — le MONTANT, lui, est par équipe en QCM avec indices activés
  // (chaque équipe a sa propre pénalité, celle de SON buzzer) : calcul
  // mutualisé par calcQcmTeamAward (#157/T1). Hors QCM (ou QCM sans
  // indices), resolvePointsAward retombe sur le montant de base pour
  // toutes les équipes — comportement inchangé par rapport à avant T3.
  // #159/F5 — MEMORY : le montant vient de calcMemoryScore (via
  // resolvePointsAward, ctx.memory), JAMAIS recalculé ici. matchedPairs/
  // errors par équipe si MEMORY_TEAM_PAIRS est renseigné (multi-équipes) ;
  // repli sur les compteurs globaux (memoryMatchedPairs/memoryErrors) en
  // mode SOLO, où la carte de MEMORY_TEAM_PAIRS n'existe pas pour ce nom
  // d'équipe. basePoints n'a aucun effet pour MEMORY (calcMemoryScore
  // l'ignore, cf. pointsAward.js) — transmis pour l'uniformité de l'appel.
  const getTeamAward = (teamName) => {
    const basePoints = creditPoints || 1
    if (question?.TYPE === 'MEMORY') {
      const matchedPairs = gameState.MEMORY_TEAM_PAIRS?.[teamName] ?? gameState.memoryMatchedPairs?.length ?? 0
      const errors = gameState.MEMORY_TEAM_ERRORS?.[teamName] ?? gameState.memoryErrors ?? 0
      return {
        amount: resolvePointsAward(gameState.question, basePoints, { memory: { matchedPairs, errors } }).amount,
        hasCorrectAnswer: null,
      }
    }
    if (isQcmWithHints) {
      return calcQcmTeamAward(gameState.question, basePoints, bumpersByTeam[teamName] || [], gameState.qcmInvalidated?.length || 0)
    }
    if (question?.TYPE === 'RAFALE') {
      // RAFALE (v8.0.0, #16/#199, contrat rafale.md §6.2) — même règle que
      // GamePage.jsx (mutualisée, utils/pointsAward.js) : suggestion =
      // compteur retenu × basePoints (creditPoints, ajustable via
      // pointsInput /admin puis rediffusé — MAJEUR-1). AnimCreditControl
      // (crédit générique, déjà monté pour tous les types) affiche ce
      // montant tel quel, verrouillé après crédit via awardedTeams comme
      // n'importe quel autre type — aucune UI dédiée nécessaire ici.
      const counter = rafaleCounterForTeam(gameState.question, teamName, gameState.RAFALE_TEAM_COUNTERS, gameState.RAFALE_TEAM_BEST)
      const award = calcRafaleTeamAward(gameState.question, basePoints, counter)
      return { amount: award?.amount ?? basePoints, hasCorrectAnswer: null }
    }
    return { amount: resolvePointsAward(gameState.question, basePoints, {}).amount, hasCorrectAnswer: null }
  }

  // En PLAYER, crédite le bumper le plus rapide de l'équipe. Correction
  // #157/T2 : le verrou de buzz n'est PAS global, il est PAR ÉQUIPE
  // (engine.go:1404-1409, "only ONE player per team can buzz") — et
  // s'applique à tous les types de question SAUF MEMORY/MEMOTION, donc à
  // QCM et ARDOISE aussi, pas seulement SPEEDY. Conséquence : une équipe a
  // au plus UN bumper avec TIME > 0, quel que soit le type — "le plus
  // rapide de l'équipe" est donc sans ambiguïté le même joueur que /admin
  // créditerait en cliquant sur son buzzer, pas une approximation propre à
  // SPEEDY.
  //
  // #170/F4 — le montant n'est plus recalculé ici : il est fourni par
  // l'appelant (AnimCreditControl, "+N pts" via getTeamAward INCHANGÉ, ou
  // "0 pt" via un simple 0 littéral). Même chemin de crédit pour les deux
  // gestes — c'est ce qui donne au refus l'enregistrement d'historique, la
  // diffusion AWARDED_TEAMS et le verrouillage d'un crédit ordinaire, sans
  // aucun état local à construire. Cible (équipe/bumper) inchangée.
  const handleCredit = (teamName, points) => {
    if (!creditEnabled) return
    if (creditTarget === 'TEAM') {
      setTeamPoints(teamName, points)
      return
    }
    const fastestBumper = (bumpersByTeam[teamName] || [])
      .filter(b => (b.TIME ?? 0) > 0)
      .sort((a, b) => a.TIME - b.TIME)[0]
    if (fastestBumper) setBumperPoints(fastestBumper.mac, points)
  }

  // #157/T4 — réponse QCM de l'équipe (zone équipes) : couleur choisie +
  // joueur, dès que l'équipe a buzzé (pas de garde de phase — contrairement
  // au crédit). Marqueur de justesse séparé, gardé côté rendu à REVEALED
  // uniquement. Rien de tout cela hors QCM.
  const getTeamQcmAnswer = (teamName) => {
    if (gameState.question?.TYPE !== 'QCM') return null
    const buzzedBumper = (bumpersByTeam[teamName] || []).find(b => (b.TIME ?? 0) > 0)
    if (!buzzedBumper?.ANSWER_COLOR) return null
    const colorInfo = QCM_COLORS[buzzedBumper.ANSWER_COLOR] || null
    return {
      colorInfo,
      playerName: buzzedBumper.NAME || '',
      isCorrect: buzzedBumper.ANSWER_COLOR === gameState.question?.QCM_CORRECT,
    }
  }

  // ENTRACTE (#119) — anim n'a AUCUN bouton de contrôle (admin uniquement,
  // contract websocket-actions.md §ENTRACTE_SET) : filtre seul + indicateur
  // net. .anim-page reste elle-même non filtrée (grille à placement
  // explicite, F7) — la classe est appliquée à chacune de ses 4 zones.
  const entracteActive = !!gameState.entracte
  const entracteDim = entracteActive ? ' entracte-dim' : ''
  // Transition progressive (#119, C3) — posée une seule fois sur .anim-page,
  // héritée par les 4 zones (.anim-zone la consomme, styles/entracte.css) —
  // pas besoin de la répéter sur chacune. Depuis entracteConfig (diffusé,
  // gelé pendant une pause active) — jamais entracteConfigSaved.
  const entracteTransitionMs = gameState.entracteConfig?.TRANSITION_MS ?? 2000

  return (
    <div className="anim-page" style={{ '--ep-transition': `${entracteTransitionMs}ms` }}>
      <AnimatePresence>
        {entracteActive && (
          <motion.div
            key="anim-entracte-indicator"
            className="anim-entracte-indicator"
            role="status"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: Math.max(0, Number(entracteTransitionMs)) / 1000 }}
          >
            ⏸ Entracte en cours — contrôle réservé à l'admin
          </motion.div>
        )}
      </AnimatePresence>

      {/* Zone contexte — bandeau (#166/F3 méta, F10 réponse permanente,
          F12 chrono en colonne). Grille 2 colonnes : lignes à gauche,
          chronomètre pleine hauteur à droite (E6 = option i, recommandée). */}
      <div className={`anim-zone anim-zone-context${entracteDim}`}>
        {isRafaleQuestion ? (
          <>
            {/* RÉVISION #198 (retour QUALIF 8.0.0.13) — l'encart
                question+réponse (équipe/catégorie/difficulté/texte/réponse,
                maquette §9.2) a DÉMÉNAGÉ vers la zone centrale
                (AnimConductPanel L3, AnimRafaleQuestion.jsx) : "je veux que
                la question et la réponse soient dans la zone centrale, pas
                dans la zone question actuelle". Ce bandeau ne garde plus
                qu'une ligne de méta compacte (équipe/catégorie/difficulté/
                progression) — mêmes classes génériques .anim-meta-row/
                .anim-chip que les autres types, aucune nouvelle règle CSS. */}
            <div className="anim-context-lines">
              <div className="anim-meta-row">
                <span className={`anim-connection-status ${status}`}>
                  <span className="anim-status-dot" />
                  {STATUS_LABEL[status] || status}
                </span>
                <div className="anim-meta-chips">
                  {gameState.RAFALE_CURRENT_TEAM && (
                    <span className="anim-chip">{gameState.RAFALE_CURRENT_TEAM}</span>
                  )}
                  {rafaleCatMeta && (
                    <span className="anim-chip">
                      {rafaleCatMeta.icon && <span className="anim-chip-glyph">{rafaleCatMeta.icon}</span>}
                      {rafaleCatMeta.label}
                    </span>
                  )}
                  {rafaleCurrentQuestion.DIFFICULTY > 0 && (
                    <span className="anim-chip">{'★'.repeat(rafaleCurrentQuestion.DIFFICULTY)}</span>
                  )}
                  {gameState.RAFALE_ASKED_COUNT > 0 && (
                    <span className="anim-chip anim-chip-count">question {gameState.RAFALE_ASKED_COUNT}</span>
                  )}
                </div>
              </div>
            </div>
            <div className="anim-chrono-col">
              <RafaleTimers
                roundTime={gameState.timer}
                roundTotal={gameState.totalTime}
                questionTime={gameState.RAFALE_QUESTION_TIME || 0}
                questionTotal={question?.RAFALE_QUESTION_TIME || 3}
                phase={gameState.phase}
                size="md"
              />
            </div>
          </>
        ) : (
        <>
        <div className="anim-context-lines">
          <div className="anim-meta-row">
            <span className={`anim-connection-status ${status}`}>
              <span className="anim-status-dot" />
              {STATUS_LABEL[status] || status}
            </span>

            {/* Groupe flex:1 — pousse .anim-question-points contre le bord
                droit de la ligne (même mécanique que #163 retouche, portée
                depuis .anim-question-info vers ce groupe englobant tous les
                chips désormais côte à côte). */}
            <div className="anim-meta-chips">
              {question ? (
                <>
                  {/* #171/F1 — ordre : avancement · catégorie · type · #ID
                      (le "titre") · options. #ID promu au même style que les
                      autres chips (auparavant en retrait, seule occurrence
                      sur la page désormais — l'ancien affichage subdued a
                      disparu, pas de doublon). */}
                  {questionPosition.total > 0 && (
                    <span className="anim-chip anim-chip-count">
                      {questionPosition.position}
                      <span className="anim-question-counter-total">/{questionPosition.total}</span>
                    </span>
                  )}
                  {categoryInfo && (
                    <span className="anim-chip">
                      {categoryInfo.icon && <span className="anim-chip-glyph">{categoryInfo.icon}</span>}
                      {categoryInfo.label}
                    </span>
                  )}
                  <span className="anim-chip">
                    <span className="anim-chip-glyph">{typeMeta.icon}</span>
                    {typeMeta.label}
                  </span>
                  <span className="anim-chip anim-chip-title">#{question.ID}</span>
                  {optionChips.map(chip => (
                    <span key={chip.key} className="anim-opt-chip">{chip.label}</span>
                  ))}
                </>
              ) : (
                <span className="anim-question-empty">Aucune question en cours</span>
              )}
            </div>

            {/* Points totaux de la question en cours, alignés à droite de
                la ligne méta (#163 retouche). */}
            {question?.POINTS != null && (
              <span className="anim-question-points">{question.POINTS}pt</span>
            )}
          </div>

          {/* Énoncé de la question en cours. Affiché dès que la question
              est chargée, SANS garde de phase (l'animateur lit la question
              avant de lancer, écart assumé avec la TV qui attend STARTED).
              #160/F8 — MEMOTION : suit la carte en cours (motionStatement)
              plutôt que question.QUESTION, qui n'a pas de sens pour une
              question portant plusieurs cartes. */}
          {isMemotionQuestion ? (
            <p className="anim-question-statement">{motionStatement}</p>
          ) : question?.QUESTION && (
            <p className="anim-question-statement">{question.QUESTION}</p>
          )}

          {/* #171/F2 — pastille de statut de phase déplacée de la colonne
              chrono (Timer, showPhase=false ci-dessous) vers cette ligne,
              juste avant la zone réponse. Mêmes classes/libellés que Timer
              (utils/phaseBadge.js — Timer.jsx lui-même n'est pas modifié). */}
          <div className="anim-answer-row">
            {phaseBadge && (
              <span className={`phase-badge ${phaseBadge.className}`}>{phaseBadge.label}</span>
            )}
            {/* #166/F10 — zone réponse permanente : remplace le bloc
                conditionnel #163/F4. Absente si aucune question chargée
                (AnimAnswerZone rend null). */}
            <AnimAnswerZone question={answerZoneQuestion} revealed={revealed} />
          </div>
        </div>

        {/* #166/F12 (E6 = i) — chronomètre en colonne dédiée, pleine hauteur
            sur les trois lignes du bandeau. Même composant Timer que
            précédemment, simplement repositionné et agrandi (md -> lg). */}
        <div className="anim-chrono-col">
          <Timer
            currentTime={gameState.timer}
            totalTime={gameState.totalTime}
            phase={gameState.phase}
            size="lg"
            showPhase={false}
          />
        </div>
        </>
        )}
      </div>

      {/* Zone conduite (#166/F5) — cinq lignes permanentes, l'état de
          chaque geste est calculé PAR AnimConductPanel depuis phase/question
          (utils/phaseRules.js) : AnimPage ne précalcule plus isPlaying/
          canStart/canReveal. */}
      <div className={`anim-zone anim-zone-conduct${entracteDim}`}>
        <AnimConductPanel
          phase={gameState.phase}
          question={question}
          qcmInvalidated={typeState.qcmInvalidated}
          revealed={revealed}
          hostContext={hostContext}
          nextQuestion={nextQuestion}
          onStart={handleStart}
          onPause={pauseGame}
          onContinue={continueGame}
          onStop={stopGame}
          onReveal={revealAnswer}
          onSelectNext={selectQuestion}
          teams={teams}
          memory={{
            flippedCards: gameState.memoryFlippedCards,
            matchedPairs: gameState.memoryMatchedPairs,
            pairOwners: gameState.MEMORY_PAIR_OWNERS,
            currentTeam: gameState.MEMORY_CURRENT_TEAM,
            teamPairs: gameState.MEMORY_TEAM_PAIRS,
            teamErrors: gameState.MEMORY_TEAM_ERRORS,
            errors: gameState.memoryErrors,
          }}
          cardMemory={typeState.memory}
          onFlipMemoryCard={handleFlipMemoryCard}
          motion={{
            subphase: motionSubphase,
            timerRunning: gameState.timer > 0,
            cardStates: gameState.MEMOTION_CARD_STATES,
            cardTeams: gameState.MEMOTION_CARD_TEAMS,
            currentTeam: gameState.MEMOTION_CURRENT_TEAM,
            currentTeamColor: gameState.MEMOTION_CURRENT_TEAM_COLOR,
            selectedId: gameState.MEMOTION_SELECTED,
            participatingTeams: gameState.MEMOTION_PARTICIPATING_TEAMS,
          }}
          onSelectMotionCard={selectMotionCard}
          onFlipMotionCard={flipMotionCard}
          onStopMotionTimer={stopMotionTimer}
          onRevealMotionCard={revealMotionCard}
          onDoneMotionCard={doneMotionCard}
          waitReason={waitReason}
          // RAFALE (contrat rafale.md §5.1, milestone v8.0.0, #107 phase 2)
          // — câblage réel des boutons VALIDE/INVALIDE (AnimRafaleActions,
          // montés par AnimConductPanel). Désactivés hors sous-phase
          // QUESTION (ex. ROUND_END, ou question non-RAFALE) : rafaleValidate/
          // rafaleInvalidate (useWebSocket.js) n'ont de sens que pendant le
          // tirage courant.
          rafaleDisabled={!isRafaleQuestion || gameState.RAFALE_SUBPHASE !== 'QUESTION'}
          onRafaleValidate={rafaleValidate}
          onRafaleInvalidate={rafaleInvalidate}
          // #198 (retour QUALIF 8.0.0.13) — question+réponse rendues en
          // zone centrale (AnimRafaleQuestion, L3) plutôt que dans
          // .anim-zone-context (voir ce bandeau plus haut, réduit à une
          // ligne de méta compacte). Données déjà résolues ci-dessus.
          rafale={isRafaleQuestion ? {
            current: rafaleCurrentQuestion,
            teamName: gameState.RAFALE_CURRENT_TEAM,
            teamColorCss: rafaleCurrentTeamCss,
            answerValue: rafaleAnswerValue,
            catMeta: rafaleCatMeta,
            askedCount: gameState.RAFALE_ASKED_COUNT,
            // NEXT (#202, contrat §13.3/§13.5/§13.6) — zone "SUIVANTE" de
            // AnimRafaleQuestion. `showNext` reprend TELLE QUELLE la
            // condition déjà posée sur `rafaleDisabled` ci-dessus (sous-phase
            // QUESTION uniquement — masquée en ROUND_END, §13.5 dernière
            // ligne) : rien à préparer hors tirage courant.
            next: rafaleNext,
            nextCatMeta: rafaleNextCatMeta,
            showNext: gameState.RAFALE_SUBPHASE === 'QUESTION',
          } : null}
        />
      </div>

      {/* Zone équipes (#156/F6 : ordre de buzz, rang, temps, crédit ;
          #157/T3-T4 : montant par équipe en QCM, couleur de réponse).
          INCHANGÉE par #166 — seule sa grid-area change (AnimPage.css).
          #158/F3 — en ARDOISE (phases STARTED/PAUSED/STOPPED/REVEALED),
          AnimArdoiseList remplace les cartes équipe À LA MÊME PLACE ; hors
          ARDOISE, ce bloc reste strictement celui de #156/#157/#170. */}
      <div className={`anim-zone anim-zone-teams${entracteDim}`}>
        {showArdoiseList ? (
          <AnimArdoiseList
            entries={ardoiseEntries}
            question={question}
            gameTime={gameState.gameTime}
            creditPoints={creditPoints}
            revealed={revealed}
            awardedTeams={awardedTeams}
            onCredit={handleArdoiseCredit}
          />
        ) : displayTeams.map((team, index) => {
          const rank = index + 1
          const rankBadge = showRankBadge ? getRankBadge(rank) : null
          const reactionTime = showBuzzOrder ? formatReactionTime(team.TIME, gameState.gameTime) : null
          const qcmAnswer = getTeamQcmAnswer(team.name)
          // #159/F4 — MEMORY : participation (MEMORY_PARTICIPATING_TEAMS
          // vide/absent = pas de restriction, toutes les équipes du roster
          // participent — repli permissif, même philosophie que
          // canAwardPoints #171/F4), compteurs par équipe (repli sur les
          // compteurs globaux memoryMatchedPairs/memoryErrors quand
          // MEMORY_TEAM_PAIRS/TEAM_ERRORS n'a pas d'entrée pour cette
          // équipe — mode SOLO) et équipe active (MEMORY_CURRENT_TEAM). La
          // ligne de stat s'affiche pour TOUTE question MEMORY, jamais
          // conditionnée à la présence de MEMORY_TEAM_PAIRS : la
          // participation est une notion à part (MEMORY_PARTICIPATING_TEAMS),
          // indépendante de la ventilation par équipe des compteurs.
          const isMemoryQuestion = question?.TYPE === 'MEMORY'
          const memoryParticipating = isMemoryQuestion
            ? (!gameState.MEMORY_PARTICIPATING_TEAMS?.length || gameState.MEMORY_PARTICIPATING_TEAMS.includes(team.name))
            : true
          const memoryStat = isMemoryQuestion
            ? (() => {
                if (!memoryParticipating) return { participating: false, label: 'ne participe pas' }
                const pairs = gameState.MEMORY_TEAM_PAIRS?.[team.name] ?? gameState.memoryMatchedPairs?.length ?? 0
                const errors = gameState.MEMORY_TEAM_ERRORS?.[team.name] ?? gameState.memoryErrors ?? 0
                const label = `${pairs} paire${pairs > 1 ? 's' : ''} · ${errors} erreur${errors > 1 ? 's' : ''}`
                return { participating: true, label }
              })()
            : null
          const isActiveMemoryTeam = isMemoryQuestion && gameState.MEMORY_CURRENT_TEAM === team.name
          // #160/F8 — MEMOTION : même traitement que MEMORY ci-dessus
          // (liseré équipe active, atténuation + mention "ne participe pas"
          // hors participation) — SANS ligne de compteur paires/erreurs (pas
          // de notion équivalente en MEMOTION : les points sont attribués
          // carte par carte par le moteur, cf. AC7, pas de compteur à
          // afficher côté équipe).
          const motionParticipating = isMemotionQuestion
            ? (!gameState.MEMOTION_PARTICIPATING_TEAMS?.length || gameState.MEMOTION_PARTICIPATING_TEAMS.includes(team.name))
            : true
          const motionStat = isMemotionQuestion && !motionParticipating
            ? { participating: false, label: 'ne participe pas' }
            : null
          const isActiveMotionTeam = isMemotionQuestion && gameState.MEMOTION_CURRENT_TEAM === team.name
          // RAFALE (v8.0.0, #16/#199, contrat rafale.md §6.1/§8.2, tâche 34)
          // — même traitement que MEMORY/MEMOTION ci-dessus : compteur "live"
          // (pas un score réel, §6.1), équipe active en surbrillance (§8.2).
          const isRafaleQuestion = question?.TYPE === 'RAFALE'
          const rafaleParticipating = isRafaleQuestion
            ? (!gameState.RAFALE_PARTICIPATING_TEAMS?.length || gameState.RAFALE_PARTICIPATING_TEAMS.includes(team.name))
            : true
          // RÉVISION 2026-08-28 (maquette rafale-v8.html §3, section
          // conservée par la refonte §9) — panneau enrichi : 3 compteurs par
          // équipe au lieu du seul total de bonnes réponses. RAFALE_TEAM_
          // ERRORS/STREAK sont nouveaux (dev-backend, contrat rafale.md
          // §4/redéfinition 2026-08-30) ; RAFALE_TEAM_COUNTERS inchangé.
          const rafaleStat = isRafaleQuestion
            ? (() => {
                if (!rafaleParticipating) return { participating: false, label: 'ne participe pas' }
                const correct = gameState.RAFALE_TEAM_COUNTERS?.[team.name] || 0
                const errors = gameState.RAFALE_TEAM_ERRORS?.[team.name] || 0
                const streak = gameState.RAFALE_TEAM_STREAK?.[team.name] || 0
                return { participating: true, correct, errors, streak }
              })()
            : null
          const isActiveRafaleTeam = isRafaleQuestion && gameState.RAFALE_CURRENT_TEAM === team.name
          // #171/F4/F6 — "tenté" ne conditionne plus si le geste de crédit
          // est monté (creditEnabled, phase seule, inchangé) : seulement si
          // un montant positif est proposé en plus de "0 pt". Une équipe
          // jamais buzzée mais DÉJÀ créditée par la régie doit rester
          // verrouillée avec son montant — attempted n'intervient qu'AVANT
          // verrouillage (awardedTeams reste l'unique source du lock, #170).
          // #159/F5/F6 — pour MEMORY, "tenté" = participe à la question
          // (memoryParticipating) plutôt que canAwardPoints (basé sur les
          // bumpers, sans objet ici) : une équipe hors
          // MEMORY_PARTICIPATING_TEAMS ne se voit proposer que "0 pt".
          const attempted = isMemoryQuestion ? memoryParticipating : isRafaleQuestion ? rafaleParticipating : canAwardPoints(question, bumpersByTeam[team.name])
          const teamCreditAmount = attempted ? getTeamAward(team.name).amount : null
          const noAttemptLabel = question?.TYPE === 'QCM' ? 'pas de réponse' : 'pas de buzz'
          const hasExtra = reactionTime || qcmAnswer || creditEnabled || memoryStat || motionStat || rafaleStat
          return (
            <AnimTeamCard
              key={team.name}
              name={team.name}
              color={team.COLOR}
              score={team.SCORE || 0}
              medal={rankBadge}
              active={isActiveMemoryTeam || isActiveMotionTeam || isActiveRafaleTeam}
              dimmed={(isMemoryQuestion && !memoryParticipating) || (isMemotionQuestion && !motionParticipating) || (isRafaleQuestion && !rafaleParticipating)}
            >
              {hasExtra && (
                <>
                  {reactionTime && (
                    <span className="anim-team-buzz-info">
                      <span className="anim-team-reaction-time">{reactionTime}</span>
                    </span>
                  )}
                  {memoryStat && (
                    <span className="anim-team-memory-stat">{memoryStat.label}</span>
                  )}
                  {motionStat && (
                    <span className="anim-team-memory-stat">{motionStat.label}</span>
                  )}
                  {rafaleStat && (
                    rafaleStat.participating ? (
                      <span className="rafale-anim-team-stats">
                        <span className="rafale-anim-team-stat rafale-anim-team-stat-good">
                          {rafaleStat.correct}<small>bonnes</small>
                        </span>
                        <span className="rafale-anim-team-stat rafale-anim-team-stat-bad">
                          {rafaleStat.errors}<small>mauvaises</small>
                        </span>
                        <span className="rafale-anim-team-stat rafale-anim-team-stat-streak">
                          {rafaleStat.streak}<small>d'affilée</small>
                        </span>
                      </span>
                    ) : (
                      <span className="anim-team-memory-stat">{rafaleStat.label}</span>
                    )
                  )}
                  {/* #157/T4 — couleur choisie dès le buzz ; justesse (✓/✗)
                      uniquement en REVEALED — rien de tout cela hors QCM
                      (qcmAnswer est null hors QCM ou tant que l'équipe n'a
                      pas buzzé). */}
                  {qcmAnswer && (
                    <span className="anim-team-qcm-answer">
                      {qcmAnswer.colorInfo && (
                        <span
                          className="anim-team-qcm-color"
                          style={{ backgroundColor: qcmAnswer.colorInfo.color }}
                          title={qcmAnswer.colorInfo.label}
                        >
                          {qcmAnswer.colorInfo.letter}
                        </span>
                      )}
                      {qcmAnswer.playerName && (
                        <span className="anim-team-qcm-player">{qcmAnswer.playerName}</span>
                      )}
                      {revealed && (
                        <span className={`anim-team-qcm-correct ${qcmAnswer.isCorrect ? 'correct' : 'incorrect'}`}>
                          {qcmAnswer.isCorrect ? '✓' : '✗'}
                        </span>
                      )}
                    </span>
                  )}
                  {/* #170/F3 — composant de crédit unique : décide et rend
                      les deux gestes ou l'état verrouillé, à partir de
                      awardedTeams (F1). Cible et montant ("+N pts")
                      inchangés (getTeamAward, #157) — seuls l'habillage et
                      le verrou sont nouveaux. #171/F6 — monté pour TOUTES
                      les équipes dès creditEnabled (plus de gate sur
                      rankBadge/reactionTime/qcmAnswer) ; motif "pas de
                      buzz"/"pas de réponse" à côté, jamais à la place. */}
                  {creditEnabled && (
                    <span className="anim-team-credit-group">
                      {/* #159/F6 — pas de motif "pas de buzz/réponse" pour
                          MEMORY : memoryStat ("ne participe pas") le dit
                          déjà, pas la peine de le répéter à côté du crédit. */}
                      {!attempted && !isMemoryQuestion && <span className="anim-team-no-attempt">{noAttemptLabel}</span>}
                      <AnimCreditControl
                        team={team.name}
                        amount={teamCreditAmount}
                        awarded={awardedTeams[team.name]}
                        onCredit={(points) => handleCredit(team.name, points)}
                      />
                    </span>
                  )}
                </>
              )}
            </AnimTeamCard>
          )
        })}
      </div>

      {/* #166/F8 — bande régie (#167, câblée ; geste révisé #176). État
          dérivé exclusivement de regieMessage (F3b, même règle que
          RegieMessageBar) : repos si aucun message actif, sinon consigne —
          toute la zone est la cible du double-tap qui acquitte pour toutes
          les tablettes ET la régie (REGIE_MESSAGE_CLEAR, F1). Plus de
          bouton « Vu » (AC11) : role="button"/tabIndex/onKeyDown préservent
          l'accès clavier (AC18, un seul appui suffit au clavier — le
          double-tap protège du doigt, pas du clavier). */}
      <div className={`anim-zone anim-zone-regie${entracteDim}`}>
        {regieMessage.ACTIVE ? (
          <div
            className="anim-regie-bar active"
            role="button"
            tabIndex={0}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault()
                clearRegieMessage()
              }
            }}
            {...regieDoubleTapHandlers}
          >
            <span className="msg">{regieMessage.TEXT}</span>
            <span className="anim-regie-hint">Double-tap pour marquer comme vu</span>
          </div>
        ) : (
          <div className="anim-regie-bar idle">Aucun message de la régie</div>
        )}
      </div>
    </div>
  )
}
