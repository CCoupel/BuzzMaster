import { useState, useEffect, useCallback, useRef } from 'react'

const RECONNECT_INTERVAL = 5000
// #118 (F5) — dispersion : ±30% autour de RECONNECT_INTERVAL, pour éviter que
// tous les VJoueurs ne se reconnectent en cadence après une coupure
// collective (redémarrage de point d'accès).
const RECONNECT_JITTER_RATIO = 0.3

// #118 (F1, R6) — surveillance de liaison passive. Le client n'émet AUCUN
// battement périodique : il surveille seulement l'arrivée de messages
// (n'importe lequel, HEARTBEAT compris — voir handleMessage). Le serveur émet
// HEARTBEAT { INTERVAL_MS, DEAD_LINK_TIMEOUT_MS } sur son ticker par client
// existant (writePump).
//
// #130 — chemin nominal : DEAD_LINK_TIMEOUT_MS est désormais transmis en
// valeur ABSOLUE par le serveur (contrat liveness-timing.md §3) — c'est lui
// qui fixe le seuil de liaison morte quand il est présent et valide (voir
// deadLinkTimeoutMsRef, case 'HEARTBEAT'). Les deux constantes ci-dessous ne
// sont plus le chemin nominal : elles restent le REPLI contractuel, utilisé
// tant qu'aucun DEAD_LINK_TIMEOUT_MS valide n'a été reçu (serveur antérieur à
// #130, ou valeur aberrante ignorée par la garde de robustesse) — dans ce
// cas le seuil se déduit de la cadence INTERVAL_MS annoncée (repli
// intermédiaire), ou de FALLBACK_HEARTBEAT_INTERVAL_MS si même celle-ci n'est
// encore jamais arrivée (repli total, cadence supposée au moment de #118).
const FALLBACK_HEARTBEAT_INTERVAL_MS = 3000
const HEARTBEAT_MISS_THRESHOLD = 3
// Fréquence à laquelle on VÉRIFIE le délai écoulé — indépendante de la
// cadence du battement lui-même. #130 : 1000 → 500ms — au seuil nominal de
// 4000ms (contrat), une granularité de 1000ms aurait étalé la détection sur
// 4,0-5,0s au lieu de 4,0-4,5s.
const LIVENESS_CHECK_INTERVAL_MS = 500

function nextReconnectDelay() {
  const jitter = RECONNECT_INTERVAL * RECONNECT_JITTER_RATIO
  return RECONNECT_INTERVAL + (Math.random() * 2 - 1) * jitter
}

// v6.1.0 (#137, contract game-state.md risque R2) — un GameState diffusé porte
// toujours QUIZ_POPULATIONS/QUIZ_DIFFICULTIES/QUIZ_HIDDEN_FIELDS comme
// tableaux (jamais null côté serveur), mais un serveur partiellement déployé
// ou un message malformé pourrait envoyer autre chose : une valeur absente ou
// d'un type inattendu devient [] plutôt que de casser l'itération côté TV.
function normalizeQuizArray(value) {
  return Array.isArray(value) ? value : []
}

export default function useWebSocket(endpoint = '/ws/admin') {
  const [status, setStatus] = useState('disconnected')
  const [gameState, setGameState] = useState({
    phase: 'STOPPED',
    timer: 30,
    totalTime: 30,
    countdownTime: 0,
    gameTime: 0,
    question: null,
    remote: 'GAME',
    backgrounds: [],
    currentBackgroundIndex: 0, // Server-synchronized
    memoryFlippedCards: [], // Server-synchronized flipped Memory cards (max 2)
    memoryMatchedPairs: [], // Server-synchronized matched pair IDs (permanent)
    memoryErrors: 0, // Server-synchronized error count (non-matches)
    MEMORY_CURRENT_TEAM: null, // Current team playing in multi-team Memory
    MEMORY_CURRENT_TEAM_COLOR: null, // RGB color array of current team
    MEMORY_PAIR_OWNERS: {}, // Map of pairId -> teamName for matched pairs
    MEMORY_TEAM_PAIRS: {}, // Map of teamName -> pairCount
    MEMORY_TEAM_ERRORS: {}, // Map of teamName -> errorCount
    MEMORY_PARTICIPATING_TEAMS: [], // List of participating team names
    // MEMOTION fields (v5.0.0)
    MEMOTION_SUBPHASE: '', // 'MEMORIZE' | 'GRID' | 'SELECTED' | 'QUESTION' | 'REVEAL' | ''
    MEMOTION_SELECTED: '', // ID of active card
    MEMOTION_CARD_STATES: {}, // Map of cardID -> 'UNPLAYED'|'QUESTION'|'REVEALED'|'DONE'
    MEMOTION_CARD_TEAMS: {}, // Map of cardID -> teamName (winner)
    // #184/B-F6 — emplacement actif MEMOTION (contrat question-types.md §5.2) :
    // {CARD_ID, TYPE, STATE}, jamais omitempty côté serveur. Consommé par
    // utils/typeState.js::getTypeState (hôte carte, cardId !== "").
    MEMOTION_ACTIVE: { CARD_ID: '', TYPE: '', STATE: {} },
    MEMOTION_CURRENT_TEAM: '', // Currently playing team
    MEMOTION_CURRENT_TEAM_COLOR: [], // RGB array of current team
    MEMOTION_PARTICIPATING_TEAMS: [], // Selected participating teams
    qcmInvalidated: [], // Server-synchronized invalidated QCM answers (e.g., ["RED", "YELLOW"])
    virtualPlayerCount: 0, // Server-synchronized virtual player count (ENROLL phase)
    virtualPlayerLimit: 20, // Server-synchronized virtual player limit
    enrollmentActive: false, // Whether enrollment is currently open
    showQRCode: false, // Whether QR code should be displayed on TV
    // ENTRACTE (v6.5.2, #119) — pause globale, même nature éphémère que showQRCode.
    entracte: false, // true for the whole duration of the pause
    // ENTRACTE_CONFIG — configuration DIFFUSÉE au panneau (gelée pendant une
    // pause active, corrections C4). entracteConfigSaved (ci-dessous) est la
    // configuration ENREGISTRÉE, toujours à jour, admin-only — c'est elle que
    // lit le formulaire d'édition (QuestionsPage.jsx), jamais entracteConfig.
    entracteConfig: {
      TITLE: 'ENTRACTE',
      SUBTITLE: 'Retour dans 20mn',
      IMAGE_IS_CUSTOM: false,
      PANEL_SIZE: 65,
      ANIM_PERIOD: 10,
      ANIM_INTENSITY: 20,
      TRANSITION_MS: 2000,
    },
    entracteConfigSaved: {
      TITLE: 'ENTRACTE',
      SUBTITLE: 'Retour dans 20mn',
      IMAGE_IS_CUSTOM: false,
      PANEL_SIZE: 65,
      ANIM_PERIOD: 10,
      ANIM_INTENSITY: 20,
      TRANSITION_MS: 2000,
    },
    quizName: '', // Quiz name (v4.0.0)
    quizTheme: '', // Quiz theme (v4.0.0)
    quizNotes: '', // Quiz free-text notes (v4.0.0)
    quizPopulations: [], // Quiz target populations, multi-value (v6.1.0, #137 — replaces quizPopulation string)
    quizDifficulties: [], // Quiz difficulties, multi-value (v6.1.0, #137 — replaces quizDifficulty string)
    quizLanguage: '', // Quiz language (v6.0.0, #8)
    quizObjectives: '', // Quiz global objectives (v6.1.0, #137) — admin-only, never sent to /ws/tv or /ws/player
    quizHiddenFields: [], // Fields hidden from the TV NEW_GAME screen (v6.1.0, #137), subset of THEME/POPULATIONS/DIFFICULTIES/LANGUAGE
    newGameBackgrounds: [], // Multi-image backgrounds for NEW_GAME screen (v4.0.4)
    // ARDOISE fields (v5.6.0)
    ARDOISE_ANSWERS: {}, // Map of teamName -> { TEXT, SUBMITTED_AT }
    // RAFALE fields (v8.0.0, #16/#107 — contrat rafale.md §4). Jamais nil
    // côté serveur (règle projet "pas d'omitempty") — initialisés ici en
    // conséquence : maps/tableaux vides, jamais null. RAFALE_ANSWER n'est
    // PAS un champ GameState (contrat §2.3 — fuite ardoise_leak_128) : voir
    // l'état `rafaleAnswer` séparé plus bas, alimenté par l'action dédiée.
    RAFALE_SUBPHASE: '', // '' | 'QUESTION' | 'ROUND_END'
    RAFALE_CURRENT_QUESTION: { ID: '', QUESTION: '', CATEGORY: '', DIFFICULTY: 0 },
    RAFALE_QUESTION_TIME: 0, // décompte question (~3s), alimenté par RAFALE_TICK
    RAFALE_TEAM_COUNTERS: {}, // Map teamName -> compteur de manche (PAS un score réel)
    RAFALE_TEAM_BEST: {}, // Map teamName -> meilleur compteur atteint (MAILLON_FAIBLE uniquement)
    RAFALE_CURRENT_TEAM: '',
    RAFALE_PARTICIPATING_TEAMS: [],
    RAFALE_CURRENT_TEAM_COLOR: [],
    RAFALE_ASKED_COUNT: 0,
    RAFALE_POOL_REMAINING: 0,
    RAFALE_EXHAUSTED: false,
  })
  const [teams, setTeams] = useState({})
  const [bumpers, setBumpers] = useState({})
  const [questions, setQuestions] = useState({})
  const [fsInfo, setFsInfo] = useState(null)
  const [version, setVersion] = useState(null)
  const [clientCounts, setClientCounts] = useState({ admin: 0, tv: 0, vplayer: 0, buzzerWs: 0, anim: 0 })
  // #155/#156 (F4) — dernier NEXT_QUESTION reçu : la question suivante jouable,
  // pour l'enchaînement animateur sans consulter /admin. `null` tant qu'aucun
  // NEXT_QUESTION n'est encore arrivé, ou si le serveur a explicitement
  // annoncé qu'il n'y en a plus (payload vide — contracts/websocket-actions.md
  // §NEXT_QUESTION : absence de champ ID). Exclusif à /ws/anim (ClientTypeAnim),
  // reste toujours null sur les autres endpoints.
  const [nextQuestion, setNextQuestion] = useState(null)
  // #166 (F1) — progression dans le quiz (question COURANTE, pas la
  // suivante) : contracts/websocket-actions.md §NEXT_QUESTION,
  // CURRENT_POSITION/TOTAL_QUESTIONS. Volontairement INDÉPENDANT de
  // `nextQuestion` — le payload n'est plus "tout ou rien" depuis #166 :
  // ces deux champs restent renseignés même en fin de quiz (plus de
  // question suivante), pour que "12/12" reste affiché sur la dernière
  // question plutôt que de disparaître. `position: 0` = pas encore reçu /
  // aucune question courante.
  const [questionPosition, setQuestionPosition] = useState({ position: 0, total: 0 })
  // #170 (F1) — équipes déjà créditées pour la question COURANTE, indexé
  // par nom d'équipe pour accès direct (contracts/websocket-actions.md
  // §AWARDED_TEAMS). Exclusif à /ws/anim, comme nextQuestion/creditPoints —
  // reste vide ({}) sur les autres endpoints (jamais diffusé par le
  // serveur ailleurs que ClientTypeAnim). Source de vérité du verrouillage
  // du crédit (AnimCreditControl, F2) : JAMAIS d'anticipation locale du
  // clic ici, uniquement ce que le serveur confirme.
  const [awardedTeams, setAwardedTeams] = useState({})
  // #155/#156 (MAJEUR-1, revue de code) — montant de base courant que
  // l'animateur créditera, rediffusé par le serveur via CREDIT_POINTS
  // (contrepartie de SET_CREDIT_POINTS émis par /admin). Remplace la lecture
  // directe de `question.POINTS` côté AnimPage — sans ce champ, /anim et
  // /admin pouvaient créditer deux montants différents pour la même question
  // dès que l'admin ajustait pointsInput sans resélectionner. 0 tant qu'aucun
  // CREDIT_POINTS n'est encore arrivé (le serveur envoie une valeur ciblée
  // dès le HELLO, donc la fenêtre réelle est très courte) ou après NEW_GAME
  // (aucune question courante — contrat §CREDIT_POINTS). Exclusif à
  // /ws/anim, reste 0 sur les autres endpoints.
  const [creditPoints, setCreditPoints] = useState(0)
  // RAFALE (v8.0.0, #16/#107, contrat rafale.md §2.3/§5.2) — réponse
  // attendue de la question courante, JAMAIS dans gameState (fuite
  // ardoise_leak_128 : /tv et /anim reçoivent le même payload GameState,
  // aucune liste d'exclusion ne peut les séparer). Diffusée par l'action
  // dédiée RAFALE_ANSWER, à `admin`+`anim` uniquement — reste `null` sur
  // /ws/tv et /ws/player, qui ne la reçoivent jamais. `null` tant qu'aucune
  // question RAFALE n'a encore été tirée.
  const [rafaleAnswer, setRafaleAnswer] = useState(null)
  // Résultat du dernier PLAYER_CONNECT en attente de réponse serveur — consommé
  // par EnrollPage (attend PLAYER_CONNECTED/PLAYER_REJECTED au lieu de naviguer
  // en aveugle, fix R1 #109). { status: 'connected', id, name } | { status: 'rejected', reason } | null
  const [playerConnectStatus, setPlayerConnectStatus] = useState(null)
  // #120 — dernier PLAYER_EVICTED reçu (ciblé, ce client uniquement), en
  // attente de consommation par VPlayerPage : { reason } | null. Remplace la
  // détection par balayage de roster, qui ne pouvait pas distinguer "bumper
  // supprimé" de "bumper pas encore reçu" pendant la fenêtre d'enrôlement.
  const [playerEvictedStatus, setPlayerEvictedStatus] = useState(null)
  const [logs, setLogs] = useState([])
  const [firmwareInfo, setFirmwareInfo] = useState(null) // { VERSION, FILENAME, SIZE, EXISTS }
  // #137 — état du job de génération IA en tâche de fond (un seul job global,
  // tout admin confondu, contract ai-multi-provider.md §10/§12). null = aucun
  // job connu. Alimenté par AI_GENERATION_PROGRESS, émis par le serveur après
  // chaque lot ET immédiatement à la connexion d'un client admin si un job
  // est en cours (permet à AIGenerateModal de se ré-attacher après un
  // rechargement de page sans état à reconstruire côté client).
  const [aiJob, setAiJob] = useState(null) // { jobId, state, batchesDone, batchesTotal, createdCount, skippedCount, errorCode, errorMessage, provider } | null
  // #167 (F1) — messagerie régie → animateurs, un seul emplacement, jamais
  // optimiste côté client (contrats/websocket-actions.md §"Messagerie
  // régie"). Diffusé aux clients admin ET anim uniquement ; reste à sa
  // valeur par défaut sur tv/vplayer, qui ne reçoivent jamais REGIE_MESSAGE.
  // ACTIVE:false / TEXT:'' / SENT_AT:0 / CLEARED_BY:'' = repos initial, avant
  // tout REGIE_MESSAGE reçu (le serveur rejoue l'état courant au HELLO).
  const [regieMessage, setRegieMessage] = useState({ ACTIVE: false, TEXT: '', SENT_AT: 0, CLEARED_BY: '' })

  const wsRef = useRef(null)
  const logCallbackRef = useRef(null)
  const reconnectTimeoutRef = useRef(null)
  // #118 (F1, R6) — instant du dernier message reçu (n'importe lequel,
  // HEARTBEAT compris) et cadence de battement annoncée par le serveur.
  const lastMessageAtRef = useRef(Date.now())
  const heartbeatIntervalMsRef = useRef(null)
  // #130 — dernier DEAD_LINK_TIMEOUT_MS valide reçu (voir case 'HEARTBEAT').
  // `null` tant qu'aucune valeur valide n'est jamais arrivée (repli sur la
  // règle dérivée). N'est jamais réinitialisé sur reconnexion, même logique
  // que heartbeatIntervalMsRef ci-dessus : c'est une constante serveur, pas
  // un état par connexion — la garder au chaud évite d'élargir inutilement
  // la fenêtre de repli juste après une reconnexion.
  const deadLinkTimeoutMsRef = useRef(null)

  const connect = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}${endpoint}`

    setStatus('connecting')
    const ws = new WebSocket(wsUrl)
    wsRef.current = ws
    // Reset the liveness clock on every fresh socket — avoids a false
    // positive in the window between creation and the first message.
    lastMessageAtRef.current = Date.now()

    ws.onopen = () => {
      lastMessageAtRef.current = Date.now()
      setStatus('connected')
      sendMessage('HELLO', {})
    }

    ws.onclose = () => {
      setStatus('disconnected')
      wsRef.current = null
      // #118 (F5) — dispersion : évite que tous les VJoueurs ne se
      // reconnectent en cadence après une coupure collective.
      reconnectTimeoutRef.current = setTimeout(connect, nextReconnectDelay())
    }

    ws.onerror = (error) => {
      console.error('WebSocket error:', error)
    }

    ws.onmessage = (event) => {
      lastMessageAtRef.current = Date.now()
      try {
        const data = JSON.parse(event.data)
        handleMessage(data)
      } catch (error) {
        console.error('Failed to parse message:', error)
      }
    }
  }, [endpoint])

  // #118 (F1, R6) — ferme un socket zombie (readyState toujours OPEN, mais
  // plus aucun message reçu depuis le seuil de liaison morte) et programme une
  // reconnexion. Neutralise D'ABORD les handlers du socket périmé, puis le
  // ferme, puis SEULEMENT ENSUITE remet la référence à null : cet ordre est
  // ce qui empêche la garde de connect() (`readyState === OPEN`) de rester
  // verrouillée indéfiniment sur ce socket. Neutraliser les handlers avant la
  // fermeture évite en plus qu'un `onclose` tardif sur CE socket (le
  // navigateur peut mettre du temps à finaliser une fermeture sur un lien
  // déjà mort) ne s'exécute après qu'une reconnexion ait déjà eu lieu, et
  // n'efface par erreur la référence du NOUVEAU socket.
  const closeZombieSocket = useCallback((reason) => {
    const zombie = wsRef.current
    if (!zombie) return
    console.warn(`[WS] Liaison morte détectée (${reason}) — fermeture forcée et reconnexion`)
    zombie.onopen = null
    zombie.onclose = null
    zombie.onerror = null
    zombie.onmessage = null
    try {
      zombie.close()
    } catch {
      // Déjà fermé ou fermeture en cours — sans effet sur la suite.
    }
    wsRef.current = null
    setStatus('disconnected')
    if (reconnectTimeoutRef.current) clearTimeout(reconnectTimeoutRef.current)
    reconnectTimeoutRef.current = setTimeout(connect, nextReconnectDelay())
  }, [connect])

  // #118 (F1, R6) — surveillance passive de la liaison : aucun émetteur
  // périodique côté client (voir _work/reports/plan-analysis-20260729-201500-
  // heartbeat.md) — seule l'arrivée de messages est surveillée. Seuil dérivé
  // de la cadence HEARTBEAT annoncée par le serveur (repli tant qu'aucun
  // HEARTBEAT n'est encore arrivé), pour rester synchronisé si le ticker
  // serveur change un jour. Un seul minuteur pour toute la durée de vie du
  // hook, nettoyé au démontage.
  useEffect(() => {
    const checkId = setInterval(() => {
      if (!wsRef.current) return // pas de socket à surveiller actuellement
      // #130 — cascade de seuil (contrat liveness-timing.md §3) : valeur
      // absolue transmise par le serveur (chemin nominal) → sinon dérivée de
      // la cadence INTERVAL_MS annoncée (repli intermédiaire, serveur
      // antérieur à #130) → sinon repli total (aucun HEARTBEAT encore reçu).
      const intervalMs = heartbeatIntervalMsRef.current || FALLBACK_HEARTBEAT_INTERVAL_MS
      const thresholdMs = deadLinkTimeoutMsRef.current ?? (intervalMs * HEARTBEAT_MISS_THRESHOLD)
      const elapsed = Date.now() - lastMessageAtRef.current
      if (elapsed > thresholdMs) {
        closeZombieSocket(`silence de ${elapsed}ms > seuil ${thresholdMs}ms`)
      }
    }, LIVENESS_CHECK_INTERVAL_MS)
    return () => clearInterval(checkId)
  }, [closeZombieSocket])

  // #170 (F1) — miroir de gameState.question.ID toujours à jour, lu par le
  // handler AWARDED_TEAMS pour rejeter un payload obsolète. `handleMessage`
  // est mémoïsé une seule fois (useCallback([]) ci-dessous) : il fermerait
  // sinon sur le `gameState` du tout premier rendu (closure périmée), même
  // piège que celui déjà évité par les mises à jour fonctionnelles
  // `setGameState(prev => ...)` utilisées partout ailleurs dans ce fichier.
  const currentQuestionIdRef = useRef(null)
  useEffect(() => {
    currentQuestionIdRef.current = gameState.question?.ID ?? null
  }, [gameState.question?.ID])

  const handleMessage = useCallback((data) => {
    const { ACTION, MSG, FSINFO, VERSION } = data
    console.log('[WS] Received:', ACTION, MSG)
    // DEBUG: Log GAME state if present
    if (MSG?.GAME) {
      console.log('[WS DEBUG] GAME state:', {
        QUESTION: MSG.GAME.QUESTION,
        MEMORY_CURRENT_TEAM: MSG.GAME.MEMORY_CURRENT_TEAM,
        MEMORY_CURRENT_TEAM_COLOR: MSG.GAME.MEMORY_CURRENT_TEAM_COLOR,
        MEMORY_PAIR_OWNERS: MSG.GAME.MEMORY_PAIR_OWNERS,
        MEMORY_TEAM_PAIRS: MSG.GAME.MEMORY_TEAM_PAIRS,
        MEMORY_PARTICIPATING_TEAMS: MSG.GAME.MEMORY_PARTICIPATING_TEAMS,
      })
    }

    switch (ACTION) {
      case 'UPDATE':
        if (MSG?.GAME) {
          setGameState(prev => ({
            ...prev,
            phase: MSG.GAME.PHASE || prev.phase,
            timer: MSG.GAME.CURRENT_TIME ?? MSG.GAME.DELAY ?? prev.timer,
            totalTime: MSG.GAME.DELAY ?? prev.totalTime,
            countdownTime: MSG.GAME.COUNTDOWN_TIME ?? prev.countdownTime,
            gameTime: MSG.GAME.TIME ?? prev.gameTime,
            question: MSG.GAME.PHASE === 'NEW_GAME' ? null : (MSG.GAME.QUESTION || prev.question),
            remote: MSG.GAME.REMOTE || prev.remote,
            backgrounds: MSG.GAME.backgrounds || prev.backgrounds,
            currentBackgroundIndex: MSG.GAME.CURRENT_BACKGROUND_INDEX ?? prev.currentBackgroundIndex,
            memoryFlippedCards: MSG.GAME.MEMORY_FLIPPED_CARDS || [],
            memoryMatchedPairs: MSG.GAME.MEMORY_MATCHED_PAIRS || [],
            memoryErrors: MSG.GAME.MEMORY_ERRORS || 0,
            MEMORY_CURRENT_TEAM: MSG.GAME.MEMORY_CURRENT_TEAM ?? prev.MEMORY_CURRENT_TEAM,
            MEMORY_CURRENT_TEAM_COLOR: MSG.GAME.MEMORY_CURRENT_TEAM_COLOR ?? prev.MEMORY_CURRENT_TEAM_COLOR,
            MEMORY_PAIR_OWNERS: MSG.GAME.MEMORY_PAIR_OWNERS ?? prev.MEMORY_PAIR_OWNERS,
            MEMORY_TEAM_PAIRS: MSG.GAME.MEMORY_TEAM_PAIRS ?? prev.MEMORY_TEAM_PAIRS,
            MEMORY_TEAM_ERRORS: MSG.GAME.MEMORY_TEAM_ERRORS ?? prev.MEMORY_TEAM_ERRORS,
            MEMORY_PARTICIPATING_TEAMS: MSG.GAME.MEMORY_PARTICIPATING_TEAMS ?? prev.MEMORY_PARTICIPATING_TEAMS,
            // MEMOTION fields (v5.0.0)
            MEMOTION_SUBPHASE: MSG.GAME.MEMOTION_SUBPHASE !== undefined ? MSG.GAME.MEMOTION_SUBPHASE : prev.MEMOTION_SUBPHASE,
            MEMOTION_SELECTED: MSG.GAME.MEMOTION_SELECTED !== undefined ? MSG.GAME.MEMOTION_SELECTED : prev.MEMOTION_SELECTED,
            MEMOTION_CARD_STATES: MSG.GAME.MEMOTION_CARD_STATES ?? prev.MEMOTION_CARD_STATES,
            MEMOTION_CARD_TEAMS: MSG.GAME.MEMOTION_CARD_TEAMS ?? prev.MEMOTION_CARD_TEAMS,
            // #184/B-F6 — voir défaut ci-dessus (state initial du hook).
            MEMOTION_ACTIVE: MSG.GAME.MEMOTION_ACTIVE ?? prev.MEMOTION_ACTIVE,
            MEMOTION_CURRENT_TEAM: MSG.GAME.MEMOTION_CURRENT_TEAM !== undefined ? MSG.GAME.MEMOTION_CURRENT_TEAM : prev.MEMOTION_CURRENT_TEAM,
            MEMOTION_CURRENT_TEAM_COLOR: MSG.GAME.MEMOTION_CURRENT_TEAM_COLOR ?? prev.MEMOTION_CURRENT_TEAM_COLOR,
            MEMOTION_PARTICIPATING_TEAMS: MSG.GAME.MEMOTION_PARTICIPATING_TEAMS ?? prev.MEMOTION_PARTICIPATING_TEAMS,
            // ARDOISE fields (v5.6.0)
            ARDOISE_ANSWERS: MSG.GAME.ARDOISE_ANSWERS !== undefined ? MSG.GAME.ARDOISE_ANSWERS : prev.ARDOISE_ANSWERS,
            // RAFALE fields (v8.0.0, #16/#107 — contrat rafale.md §4). `??`
            // préserve les valeurs falsy légitimes (0, '', [], {}) — seul
            // `undefined` (champ absent, ex. serveur pas encore à jour côté
            // dev-backend pendant #107) retombe sur `prev`.
            RAFALE_SUBPHASE: MSG.GAME.RAFALE_SUBPHASE ?? prev.RAFALE_SUBPHASE,
            RAFALE_CURRENT_QUESTION: MSG.GAME.RAFALE_CURRENT_QUESTION ?? prev.RAFALE_CURRENT_QUESTION,
            RAFALE_QUESTION_TIME: MSG.GAME.RAFALE_QUESTION_TIME ?? prev.RAFALE_QUESTION_TIME,
            RAFALE_TEAM_COUNTERS: MSG.GAME.RAFALE_TEAM_COUNTERS ?? prev.RAFALE_TEAM_COUNTERS,
            RAFALE_TEAM_BEST: MSG.GAME.RAFALE_TEAM_BEST ?? prev.RAFALE_TEAM_BEST,
            RAFALE_CURRENT_TEAM: MSG.GAME.RAFALE_CURRENT_TEAM ?? prev.RAFALE_CURRENT_TEAM,
            RAFALE_PARTICIPATING_TEAMS: MSG.GAME.RAFALE_PARTICIPATING_TEAMS ?? prev.RAFALE_PARTICIPATING_TEAMS,
            RAFALE_CURRENT_TEAM_COLOR: MSG.GAME.RAFALE_CURRENT_TEAM_COLOR ?? prev.RAFALE_CURRENT_TEAM_COLOR,
            RAFALE_ASKED_COUNT: MSG.GAME.RAFALE_ASKED_COUNT ?? prev.RAFALE_ASKED_COUNT,
            RAFALE_POOL_REMAINING: MSG.GAME.RAFALE_POOL_REMAINING ?? prev.RAFALE_POOL_REMAINING,
            RAFALE_EXHAUSTED: MSG.GAME.RAFALE_EXHAUSTED ?? prev.RAFALE_EXHAUSTED,
            qcmInvalidated: MSG.GAME.QCM_INVALIDATED || [],
            virtualPlayerCount: MSG.GAME.VIRTUAL_PLAYER_COUNT ?? prev.virtualPlayerCount,
            virtualPlayerLimit: MSG.GAME.VIRTUAL_PLAYER_LIMIT ?? prev.virtualPlayerLimit,
            enrollmentActive: MSG.GAME.ENROLLMENT_ACTIVE ?? prev.enrollmentActive,
            showQRCode: MSG.GAME.SHOW_QR_CODE ?? prev.showQRCode,
            // ENTRACTE (v6.5.2, #119) — jamais omitempty côté serveur (contrat game-state.md),
            // mais on garde le repli sur prev pour un client plus ancien / un message partiel.
            entracte: MSG.GAME.ENTRACTE ?? prev.entracte,
            entracteConfig: MSG.GAME.ENTRACTE_CONFIG ?? prev.entracteConfig,
            // ENTRACTE_CONFIG_SAVED (corrections C4) — diffusion restreinte à
            // l'admin (AdminOnlyGameFields), même repli que QUIZ_OBJECTIVES :
            // absent sur /ws/tv, /ws/player et /ws/anim, jamais écrasé (reste
            // au défaut initial sur ces clients, qui n'en ont de toute façon
            // aucun usage — seul le formulaire QuestionsPage le consomme).
            entracteConfigSaved: MSG.GAME.ENTRACTE_CONFIG_SAVED ?? prev.entracteConfigSaved,
            quizName: MSG.GAME.QUIZ_NAME !== undefined ? MSG.GAME.QUIZ_NAME : prev.quizName,
            quizTheme: MSG.GAME.QUIZ_THEME !== undefined ? MSG.GAME.QUIZ_THEME : prev.quizTheme,
            quizNotes: MSG.GAME.QUIZ_NOTES !== undefined ? MSG.GAME.QUIZ_NOTES : prev.quizNotes,
            // v6.1.0 (#137) — publics/difficultés multiples, objectif global,
            // visibilité TV par champ. QUIZ_OBJECTIVES est diffusion restreinte
            // (game-state.md) : absent sur /ws/tv et /ws/player, d'où le repli
            // sur prev (reste '' par défaut sur ces clients, jamais écrasé).
            quizPopulations: normalizeQuizArray(MSG.GAME.QUIZ_POPULATIONS),
            quizDifficulties: normalizeQuizArray(MSG.GAME.QUIZ_DIFFICULTIES),
            quizLanguage: MSG.GAME.QUIZ_LANGUAGE !== undefined ? MSG.GAME.QUIZ_LANGUAGE : prev.quizLanguage,
            quizObjectives: MSG.GAME.QUIZ_OBJECTIVES !== undefined ? MSG.GAME.QUIZ_OBJECTIVES : prev.quizObjectives,
            quizHiddenFields: normalizeQuizArray(MSG.GAME.QUIZ_HIDDEN_FIELDS),
          }))
        }
        if (MSG?.teams !== undefined) setTeams(MSG.teams ?? {})
        if (MSG?.bumpers !== undefined) setBumpers(MSG.bumpers ?? {})
        if (VERSION) setVersion(VERSION)
        break

      case 'UPDATE_TIMER':
        if (MSG?.GAME) {
          setGameState(prev => ({
            ...prev,
            phase: MSG.GAME.PHASE || prev.phase,
            timer: MSG.GAME.CURRENT_TIME ?? prev.timer,
            countdownTime: MSG.GAME.COUNTDOWN_TIME ?? prev.countdownTime,
            gameTime: MSG.GAME.TIME ?? prev.gameTime,
          }))
        }
        break

      case 'START':
        if (MSG?.GAME) {
          setGameState(prev => ({
            ...prev,
            phase: MSG.GAME.PHASE || 'STARTED', // Use server phase (could be COUNTDOWN)
            timer: MSG.GAME.CURRENT_TIME ?? MSG.GAME.DELAY ?? prev.timer,
            totalTime: MSG.GAME.DELAY ?? prev.totalTime,
            countdownTime: MSG.GAME.COUNTDOWN_TIME ?? prev.countdownTime,
            gameTime: MSG.GAME.TIME ?? prev.gameTime,
            question: MSG.GAME.QUESTION || prev.question,
          }))
        }
        break

      case 'STOP':
        if (MSG?.GAME) {
          setGameState(prev => ({
            ...prev,
            phase: 'STOPPED',
            question: MSG.GAME.QUESTION || prev.question,
          }))
        } else {
          setGameState(prev => ({ ...prev, phase: 'STOPPED' }))
        }
        break

      case 'PAUSE':
        setGameState(prev => ({ ...prev, phase: 'PAUSED' }))
        break

      case 'CONTINUE':
        setGameState(prev => ({ ...prev, phase: 'STARTED' }))
        break

      case 'BUMPER':
        if (MSG?.teams !== undefined) setTeams(MSG.teams ?? {})
        if (MSG?.bumpers !== undefined) setBumpers(MSG.bumpers ?? {})
        break

      case 'QUESTIONS':
        console.log('[WS] QUESTIONS handler - MSG:', MSG, 'FSINFO:', FSINFO)
        if (MSG) {
          const questionsMap = {}
          Object.entries(MSG).forEach(([key, value]) => {
            if (key !== 'FSINFO' && value?.ID) {
              questionsMap[value.ID] = value
            }
          })
          console.log('[WS] Parsed questions:', questionsMap)
          setQuestions(questionsMap)
        }
        if (FSINFO) setFsInfo(FSINFO)
        if (VERSION) setVersion(VERSION)
        break

      case 'READY':
        setGameState(prev => ({
          ...prev,
          phase: 'READY',
          question: MSG?.QUESTION || prev.question,
        }))
        break

      case 'REVEAL':
        setGameState(prev => ({ ...prev, phase: 'REVEALED' }))
        break

      case 'REMOTE':
        console.log('[WS] REMOTE handler - MSG.GAME:', MSG?.GAME)
        if (MSG?.GAME?.REMOTE) {
          console.log('[WS] Setting remote to:', MSG.GAME.REMOTE)
          setGameState(prev => ({ ...prev, remote: MSG.GAME.REMOTE }))
        }
        if (MSG?.teams !== undefined) setTeams(MSG.teams ?? {})
        if (MSG?.bumpers !== undefined) setBumpers(MSG.bumpers ?? {})
        break

      case 'CLIENTS':
        if (MSG) {
          setClientCounts({
            admin: MSG.ADMIN_COUNT ?? 0,
            tv: MSG.TV_COUNT ?? 0,
            vplayer: MSG.VPLAYER_COUNT ?? 0,
            // Compteur brut de sockets buzzer WS (informationnel — le X/Y participants
            // affiché en Navbar se calcule côté client depuis `bumpers`, pas ce champ).
            buzzerWs: MSG.BUZZER_WS_COUNT ?? 0,
            // #155 (F3/B2) — interfaces animateur connectées.
            anim: MSG.ANIM_COUNT ?? 0,
          })
        }
        break

      case 'NEXT_QUESTION':
        // #155/#156 (B5/F4) — payload vide (pas d'ID) = plus aucune question
        // jouable, contrat §NEXT_QUESTION. `null` dans les deux cas (jamais
        // reçu / explicitement vide) : AnimPage n'a pas besoin de distinguer.
        setNextQuestion(MSG?.ID ? MSG : null)
        // #166 (F1) — CURRENT_POSITION/TOTAL_QUESTIONS décrivent la question
        // COURANTE et le quiz, pas la suivante : mis à jour indépendamment de
        // la ligne ci-dessus, même quand MSG.ID est absent (fin de quiz).
        setQuestionPosition({
          position: MSG?.CURRENT_POSITION ?? 0,
          total: MSG?.TOTAL_QUESTIONS ?? 0,
        })
        break

      case 'AWARDED_TEAMS': {
        // #170 (F1) — projection serveur des équipes déjà créditées pour la
        // question courante, contrat §AWARDED_TEAMS. Garde anti-obsolescence :
        // un payload qui ne correspond plus à la question affichée localement
        // (changement de question entre l'émission et la réception) est
        // ignoré plutôt qu'appliqué — sinon une réponse de la question
        // PRÉCÉDENTE pourrait verrouiller (ou, pire, réinitialiser) le
        // verrouillage de la question qui vient de démarrer. `""` et
        // `null`/`undefined` sont équivalents ("aucune question").
        const msgQuestionId = MSG?.QUESTION_ID || null
        if (msgQuestionId !== (currentQuestionIdRef.current || null)) break
        const map = {}
        ;(MSG?.TEAMS || []).forEach((entry) => {
          if (!entry?.TEAM) return
          map[entry.TEAM] = { POINTS: entry.POINTS, TIMESTAMP: entry.TIMESTAMP }
        })
        // Tableau vide ([]) -> map vide ({}) : c'est la réinitialisation
        // "payload vide" demandée par F1, sans cas particulier à écrire.
        setAwardedTeams(map)
        break
      }

      case 'CREDIT_POINTS':
        // MAJEUR-1 — contrepartie serveur→client de SET_CREDIT_POINTS,
        // contrat §CREDIT_POINTS. Toujours un entier (0 = rien à créditer).
        setCreditPoints(MSG?.POINTS ?? 0)
        break

      case 'RAFALE_ANSWER':
        // RAFALE (v8.0.0, #16/#107, contrat rafale.md §2.3/§5.2) — réponse
        // attendue de la question courante. Diffusée à admin+anim
        // uniquement (BroadcastToTypes côté serveur) : ce case ne s'exécute
        // donc jamais côté /ws/tv ou /ws/player en pratique, mais reste
        // inoffensif si un serveur mal configuré l'envoyait quand même —
        // rafaleAnswer n'est JAMAIS lu par PlayerDisplay.jsx.
        if (MSG?.ID) {
          setRafaleAnswer({ ID: MSG.ID, ANSWER: MSG.ANSWER || '' })
        }
        break

      case 'RAFALE_TICK':
        // RAFALE (v8.0.0, #16/#107, contrat rafale.md §5.2) — décompte
        // léger du timer de QUESTION (~3s), distinct du timer de MANCHE
        // (UPDATE_TIMER/CURRENT_TIME, inchangé). Ne réémet jamais tout
        // GameState — un seul champ mis à jour ici.
        setGameState(prev => ({
          ...prev,
          RAFALE_QUESTION_TIME: MSG?.QUESTION_TIME ?? prev.RAFALE_QUESTION_TIME,
        }))
        break

      case 'REGIE_MESSAGE':
        // #167 (F1) — état dérivé EXCLUSIVEMENT du serveur (F3b) : aucun
        // champ n'a omitempty côté Go (contrat), donc MSG porte toujours les
        // 4 champs — pas de repli sur `prev` ici, contrairement à la plupart
        // des autres handlers de ce switch.
        if (MSG) {
          setRegieMessage({
            ACTIVE: MSG.ACTIVE === true,
            TEXT: MSG.TEXT || '',
            SENT_AT: MSG.SENT_AT || 0,
            CLEARED_BY: MSG.CLEARED_BY || '',
          })
        }
        break

      case 'BACKGROUND_CHANGE':
        if (MSG?.INDEX !== undefined) {
          setGameState(prev => ({
            ...prev,
            currentBackgroundIndex: MSG.INDEX,
          }))
        }
        break

      case 'QCM_HINT':
        // A QCM answer was invalidated - append to the list
        if (MSG?.COLOR) {
          console.log('[WS] QCM_HINT: invalidated color:', MSG.COLOR, 'remaining:', MSG.REMAINING)
          setGameState(prev => ({
            ...prev,
            qcmInvalidated: [...prev.qcmInvalidated, MSG.COLOR],
          }))
        }
        break

      case 'SHOW_QR_CODE':
        console.log('[WS] SHOW_QR_CODE received')
        setGameState(prev => ({
          ...prev,
          showQRCode: true,
          enrollmentActive: true,
        }))
        break

      case 'HIDE_QR_CODE':
        console.log('[WS] HIDE_QR_CODE received')
        setGameState(prev => ({
          ...prev,
          showQRCode: false,
          enrollmentActive: false,
        }))
        break

      case 'PLAYER_CONNECTED':
        console.log('[WS] PLAYER_CONNECTED:', MSG)
        // Fix R1 (#109) : capturer l'ID renvoyé par le serveur pour le
        // renvoyer à la reconnexion (lookup par ID côté backend, plus par nom).
        if (MSG?.ID) {
          localStorage.setItem('vplayer_id', MSG.ID)
        }
        setPlayerConnectStatus({ status: 'connected', id: MSG?.ID, name: MSG?.NAME })
        // Player successfully enrolled - state will be updated via UPDATE message
        break

      case 'PLAYER_REJECTED':
        console.log('[WS] PLAYER_REJECTED:', MSG?.REASON)
        setPlayerConnectStatus({ status: 'rejected', reason: MSG?.REASON })
        break

      case 'PLAYER_EVICTED':
        // #120 — notification ciblée : ce bumper vient d'être supprimé côté
        // serveur (animateur, ou purge NEW_GAME). Contrairement à l'absence
        // d'un bumper dans un roster, cette action fait autorité à elle seule.
        console.log('[WS] PLAYER_EVICTED:', MSG?.REASON)
        setPlayerEvictedStatus({ reason: MSG?.REASON })
        break

      case 'PLAYER_ASSIGNED':
        console.log('[WS] PLAYER_ASSIGNED:', MSG)
        // Player assigned to team - state will be updated via UPDATE message
        break

      case 'ENROLLMENT_UPDATE':
        console.log('[WS] ENROLLMENT_UPDATE:', MSG)
        if (MSG) {
          setGameState(prev => ({
            ...prev,
            virtualPlayerCount: MSG.VIRTUAL_PLAYER_COUNT ?? prev.virtualPlayerCount,
            virtualPlayerLimit: MSG.VIRTUAL_PLAYER_LIMIT ?? prev.virtualPlayerLimit,
            enrollmentActive: MSG.ENROLLMENT_ACTIVE ?? prev.enrollmentActive,
          }))
        }
        break

      case 'CONFIG_UPDATE':
        console.log('[WS] CONFIG_UPDATE:', MSG)
        setGameState(prev => {
          const updates = {}
          if (MSG?.neon_effect !== undefined) updates.neonEffect = MSG.neon_effect
          if (MSG?.default_question_image_is_custom !== undefined) updates.defaultQuestionImageIsCustom = MSG.default_question_image_is_custom
          if (MSG?.new_game_backgrounds !== undefined) updates.newGameBackgrounds = MSG.new_game_backgrounds ?? []
          return Object.keys(updates).length > 0 ? { ...prev, ...updates } : prev
        })
        break

      case 'LOG_HISTORY':
        // Receive full log history on subscription
        if (MSG?.entries) {
          setLogs(MSG.entries)
          if (logCallbackRef.current) {
            MSG.entries.forEach(entry => logCallbackRef.current(entry))
          }
        }
        break

      case 'LOG_ENTRY':
        // Receive a single new log entry
        if (MSG) {
          setLogs(prev => [...prev, MSG])
          if (logCallbackRef.current) {
            logCallbackRef.current(MSG)
          }
        }
        break

      case 'FIRMWARE_VERSION':
        // Received after firmware upload: update firmware info for all clients
        if (MSG) {
          console.log('[WS] FIRMWARE_VERSION:', MSG)
          setFirmwareInfo({
            VERSION: MSG.VERSION,
            FILENAME: MSG.FILENAME,
            SIZE: MSG.SIZE,
            EXISTS: MSG.EXISTS,
            IS_MERGED: MSG.IS_MERGED === true,
            EMBEDDED_VERSION: MSG.EMBEDDED_VERSION || '',
          })
        }
        break

      case 'HEARTBEAT':
        // #118 (F1, R6) — pas d'état de jeu à mettre à jour : l'arrivée du
        // message elle-même a déjà réarmé lastMessageAtRef (dans onmessage,
        // avant ce switch). La cadence annoncée est retenue ici, pour que le
        // repli intermédiaire reste dérivé de la valeur réelle du ticker
        // serveur plutôt que d'une constante indépendante.
        if (MSG?.INTERVAL_MS) heartbeatIntervalMsRef.current = MSG.INTERVAL_MS
        // #130 — seuil de liaison morte transmis en valeur ABSOLUE (chemin
        // nominal, contrat liveness-timing.md §3). Garde de robustesse
        // obligatoire (R3, plan #130) : une valeur non numérique, nulle,
        // négative, ou strictement inférieure à la cadence INTERVAL_MS de CE
        // MÊME message est ignorée — un seuil sous la cadence de battement
        // provoquerait une boucle de reconnexion permanente (le socket
        // serait fermé avant même que le prochain battement normal ait une
        // chance d'arriver). Sur valeur invalide/absente, on NE RÉINITIALISE
        // PAS deadLinkTimeoutMsRef : on conserve la dernière valeur valide
        // connue (ou `null`, jamais reçue) plutôt que de faire flapper le
        // seuil sur un message isolé aberrant — voir la boucle de
        // surveillance ci-dessus pour la cascade de repli complète.
        {
          const deadLinkTimeoutMs = MSG?.DEAD_LINK_TIMEOUT_MS
          const currentIntervalMs = heartbeatIntervalMsRef.current || FALLBACK_HEARTBEAT_INTERVAL_MS
          if (
            typeof deadLinkTimeoutMs === 'number' &&
            Number.isFinite(deadLinkTimeoutMs) &&
            deadLinkTimeoutMs > 0 &&
            deadLinkTimeoutMs >= currentIntervalMs
          ) {
            deadLinkTimeoutMsRef.current = deadLinkTimeoutMs
          }
        }
        break

      case 'AI_GENERATION_PROGRESS':
        // #137 — progression du job de génération IA en tâche de fond
        // (contract ai-multi-provider.md §10). Un seul job global : ce
        // message écrase l'état précédent sans le fusionner (STATE fait
        // autorité). Émis après chaque lot ET à la connexion si un job est
        // déjà en cours — c'est ce second cas qui permet le ré-attachement
        // après un rechargement de page (AIGenerateModal lit `aiJob` au
        // montage, pas de reconstruction d'état côté client).
        if (MSG) {
          setAiJob({
            jobId: MSG.JOB_ID,
            state: MSG.STATE,
            batchesDone: MSG.BATCHES_DONE ?? 0,
            batchesTotal: MSG.BATCHES_TOTAL ?? 0,
            createdCount: MSG.CREATED_COUNT ?? 0,
            skippedCount: MSG.SKIPPED_COUNT ?? 0,
            errorCode: MSG.ERROR_CODE || '',
            // issue #142 — détail assaini du message d'erreur provider,
            // présent uniquement quand STATE=FAILED (omitempty côté serveur,
            // absent et non '' sur les autres états — || '' couvre les deux).
            errorMessage: MSG.ERROR_MESSAGE || '',
            provider: MSG.PROVIDER || '',
          })
        }
        break

      default:
        console.log('Unknown action:', ACTION)
    }
  }, [])

  // #149 — renvoie désormais un booléen (true = effectivement envoyé sur un
  // socket ouvert) pour permettre aux appelants qui font une mise à jour
  // optimiste locale (ex: QuestionsPage.jsx handleShuffleQuestions) de
  // détecter synchroniquement l'échec réseau et revenir à l'état antérieur.
  // Tous les appelants existants ignoraient déjà la valeur de retour (void) —
  // changement additif, non cassant.
  const sendMessage = useCallback((action, msg = {}) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      const message = JSON.stringify({ ACTION: action, MSG: msg })
      console.log('[WS] Sending:', action, msg)
      wsRef.current.send(message)
      return true
    } else {
      console.error('WebSocket is not connected')
      return false
    }
  }, [])

  // Game actions
  const startGame = useCallback((delay, points) => {
    sendMessage('START', { DELAY: delay, POINTS: points })
  }, [sendMessage])

  const stopGame = useCallback(() => {
    sendMessage('STOP', {})
  }, [sendMessage])

  const pauseGame = useCallback(() => {
    sendMessage('PAUSE', {})
  }, [sendMessage])

  const continueGame = useCallback(() => {
    sendMessage('CONTINUE', {})
  }, [sendMessage])

  const revealAnswer = useCallback(() => {
    sendMessage('REVEAL', {})
  }, [sendMessage])

  const selectQuestion = useCallback((questionId) => {
    sendMessage('READY', { QUESTION: questionId })
  }, [sendMessage])

  const setRemoteDisplay = useCallback((display) => {
    sendMessage('REMOTE', { REMOTE: display })
  }, [sendMessage])

  const updateConfig = useCallback((config) => {
    sendMessage('UPDATE', config)
  }, [sendMessage])

  const deleteQuestion = useCallback((questionId) => {
    sendMessage('DELETE', { ID: questionId })
  }, [sendMessage])

  const setBumperPoints = useCallback((bumperMac, points) => {
    sendMessage('BUMPER_POINTS', { ID: bumperMac, POINTS: points })
  }, [sendMessage])

  const setTeamPoints = useCallback((teamName, points) => {
    sendMessage('TEAM_POINTS', { TEAM: teamName, POINTS: points })
  }, [sendMessage])

  // #167 (F1) — envoi/effacement de la consigne régie. Validation (trim,
  // troncature 140 runes, garde d'idempotence) intégralement côté serveur
  // (contrat §REGIE_MESSAGE_SEND règles 1-4) : ces wrappers transmettent le
  // texte brut tel quel, sans logique dupliquée ici.
  const sendRegieMessage = useCallback((text) => {
    sendMessage('REGIE_MESSAGE_SEND', { TEXT: text })
  }, [sendMessage])

  const clearRegieMessage = useCallback(() => {
    sendMessage('REGIE_MESSAGE_CLEAR', {})
  }, [sendMessage])

  // #123 (F1) — action dédiée pour supprimer un joueur/buzzer. Avant ce fix,
  // TeamsPage.jsx supprimait un bumper via `updateConfig({ bumpers })` — un
  // simple UPDATE de configuration, sans notification. `DELETE_BUMPER` est le
  // seul chemin qui déclenche `PLAYER_EVICTED { PLAYER_REMOVED }` côté
  // serveur pour le VJoueur concerné (handleDeleteBumper, #120).
  const deleteBumper = useCallback((bumperId) => {
    sendMessage('DELETE_BUMPER', { ID: bumperId })
  }, [sendMessage])

  // #122 (F1) — reprise de place assistée : autorise une reprise à usage
  // unique du bumper (score, équipe et historique conservés) au prochain
  // PLAYER_CONNECT sans ID portant ce nom. N'agit que sur une fiche marquée
  // RECLAIM_REQUESTED — voir TeamsPage.jsx.
  const releaseBumperName = useCallback((bumperId) => {
    sendMessage('RELEASE_BUMPER_NAME', { ID: bumperId })
  }, [sendMessage])

  const setClientType = useCallback((type) => {
    sendMessage('SET_CLIENT_TYPE', { TYPE: type })
  }, [sendMessage])

  // Debug: Force transition to READY state (skips PREPARE/PONG wait)
  const forceReady = useCallback(() => {
    sendMessage('FORCE_READY', {})
  }, [sendMessage])

  // Debug: Simulate a button press from a buzzer (for testing)
  const simulateButton = useCallback((bumperMac, button = 'A') => {
    sendMessage('BUTTON', { ID: bumperMac, button })
  }, [sendMessage])

  // Debug: Simulate a PONG response from a buzzer (for testing in PREPARE state)
  const simulatePong = useCallback((bumperMac) => {
    sendMessage('PONG', { ID: bumperMac })
  }, [sendMessage])

  // Memory game: Flip a card. `motionCardId` (#187, v7.1.0) porte la portée
  // de carte (`CardScope`, contrat question-types.md §9) quand le flip
  // s'applique à une carte MEMOTION active — absent (undefined) hors manche
  // MEMOTION, comportement inchangé (contrat websocket-actions.md,
  // FLIP_MEMORY_CARD : "absent hors manche MEMOTION ⇒ comportement actuel").
  //
  // 🔴 FIX bug QUALIF cycle 7 (#187) — `playerId` (payload.ID) était omis
  // depuis la toute première version de cette fonction. Sans conséquence
  // tant que le serveur n'exécutait aucune vérification de tour (avant le
  // cycle 4), mais devenu bloquant depuis : `handleFlipMemoryCard`
  // (cmd/server/main.go) résout l'émetteur vplayer par un motif 3 passes
  // (payload.ID → msg.ID → clientID, même motif qu'ARDOISE_INPUT/
  // VPLAYER_QCM_ANSWER) — le 3e repli (clientID direct) est documenté côté
  // serveur comme NE correspondant PAS à la clé du bumper pour un VJoueur
  // ("clientID fallback (IP:port — may not match bumper key for VPlayers)").
  // Sans `ID` explicite, la résolution échouait TOUJOURS pour un VJoueur —
  // `bumper == nil` → tout flip VJoueur était ignoré silencieusement, quelle
  // que soit l'équipe. `playerId` doit venir de `bumper.id` côté appelant
  // (VPlayerPage.jsx via PlayerDisplay.jsx, prop `playerId`) — absent pour
  // tv/anim, qui n'ont pas de bumper et pour qui le serveur ne vérifie de
  // toute façon aucun tour (clientType != vplayer).
  const flipMemoryCard = useCallback((cardId, motionCardId, playerId) => {
    const msg = { CARD_ID: cardId }
    if (motionCardId) msg.MOTION_CARD_ID = motionCardId
    if (playerId) msg.ID = playerId
    sendMessage('FLIP_MEMORY_CARD', msg)
  }, [sendMessage])

  // MEMOTION: Select a card from the grid (admin preview mode)
  const selectMotionCard = useCallback((cardId) => {
    sendMessage('MEMOTION_SELECT', { CARD_ID: cardId })
  }, [sendMessage])

  // MEMOTION (#160/F2) — flip la carte sélectionnée (SELECTED -> QUESTION),
  // sur le modèle exact de selectMotionCard ci-dessus.
  const flipMotionCard = useCallback(() => {
    sendMessage('MEMOTION_FLIP', {})
  }, [sendMessage])

  // MEMOTION (#160/F2) — coupe le chrono de la carte en cours (reste en
  // QUESTION).
  const stopMotionTimer = useCallback(() => {
    sendMessage('MEMOTION_STOP_TIMER', {})
  }, [sendMessage])

  // MEMOTION (#160/F2) — révèle la réponse de la carte en cours
  // (QUESTION -> REVEAL).
  const revealMotionCard = useCallback(() => {
    sendMessage('MEMOTION_REVEAL', {})
  }, [sendMessage])

  // MEMOTION (#160/F2) — clôt la carte en cours (retour à GRID), avec ou
  // sans équipe gagnante. CARD_ID doit toujours être renseigné (même pour
  // "annuler"/"sans vainqueur") — le moteur le compare à MEMOTION_SELECTED.
  const doneMotionCard = useCallback((cardId, winnerTeam = '') => {
    sendMessage('MEMOTION_DONE', { CARD_ID: cardId, WINNER_TEAM: winnerTeam })
  }, [sendMessage])

  // RAFALE (v8.0.0, #16/#107, contrat rafale.md §5.1) — juge la réponse de
  // la question courante. Sans payload (§5.1) : le serveur connaît déjà la
  // question active (RAFALE_CURRENT_QUESTION) et l'équipe qui répond
  // (RAFALE_CURRENT_TEAM), même discipline que MEMOTION_FLIP/REVEAL
  // ci-dessus. `admin`+`anim` uniquement (contrat §5.1) — sans effet
  // ailleurs si appelé (le serveur rejetterait via l'allow-list entrante).
  const rafaleValidate = useCallback(() => {
    sendMessage('RAFALE_VALIDATE', {})
  }, [sendMessage])

  const rafaleInvalidate = useCallback(() => {
    sendMessage('RAFALE_INVALIDATE', {})
  }, [sendMessage])

  // RAFALE (v8.0.0, #16/#199, contrat rafale.md §5.1) — équipes
  // participantes et ordre de passage (modes multi-équipes, Phase 3).
  // Câblé ici par anticipation du contrat (même patron que les autres
  // actions RAFALE) ; aucun appelant en Phase 2 (#107, mode SOLO).
  const rafaleSetTeams = useCallback((teamNames) => {
    sendMessage('RAFALE_SET_TEAMS', { TEAMS: teamNames })
  }, [sendMessage])

  // ENTRACTE (v6.5.2, #119) — commande explicite portant l'état voulu (pas un
  // toggle, D3 du plan) : admin uniquement, le libellé du bouton se dérive
  // côté client de gameState.entracte, aucun état de bouton côté serveur.
  const setEntracte = useCallback((active) => {
    sendMessage('ENTRACTE_SET', { ACTIVE: active })
  }, [sendMessage])

  // ENTRACTE (#119, corrections C1/C4) — enregistre la configuration du
  // panneau (propriété de la partie, éditée depuis QuestionsPage.jsx).
  // Action dédiée UPDATE_ENTRACTE_CONFIG, distincte d'UPDATE_QUIZ_META — deux
  // formulaires séparés, chacun propriétaire de ses champs (contrat
  // websocket-actions.md). Toujours le formulaire complet (pas de champs
  // optionnels à omettre comme updateQuizMeta) : cfg porte les 6 champs.
  // Accepté par le serveur même pendant un entracte actif (C4) — l'action
  // écrit ENTRACTE_CONFIG_SAVED, sans rafraîchir le panneau déjà diffusé.
  const updateEntracteConfig = useCallback((cfg) => {
    sendMessage('UPDATE_ENTRACTE_CONFIG', {
      TITLE: cfg.TITLE,
      SUBTITLE: cfg.SUBTITLE,
      PANEL_SIZE: cfg.PANEL_SIZE,
      ANIM_PERIOD: cfg.ANIM_PERIOD,
      ANIM_INTENSITY: cfg.ANIM_INTENSITY,
      TRANSITION_MS: cfg.TRANSITION_MS,
    })
  }, [sendMessage])

  // VPlayer enrollment: Show QR code
  const showQRCode = useCallback(() => {
    sendMessage('SHOW_QR_CODE', {})
  }, [sendMessage])

  // VPlayer enrollment: Hide QR code
  const hideQRCode = useCallback(() => {
    sendMessage('HIDE_QR_CODE', {})
  }, [sendMessage])

  // VPlayer enrollment: Connect as virtual player
  const connectVirtualPlayer = useCallback((name) => {
    sendMessage('PLAYER_CONNECT', { NAME: name })
  }, [sendMessage])

  // VPlayer enrollment: Set virtual player limit
  const setVirtualPlayerLimit = useCallback((limit) => {
    sendMessage('SET_VIRTUAL_PLAYER_LIMIT', { LIMIT: limit })
  }, [sendMessage])

  // VPlayer enrollment: consume the last PLAYER_CONNECTED/PLAYER_REJECTED
  // result once handled by the caller (EnrollPage) — avoids re-triggering
  // the same navigation/error on every re-render (fix R1 #109).
  const clearPlayerConnectStatus = useCallback(() => {
    setPlayerConnectStatus(null)
  }, [])

  // #120 — consommer le dernier PLAYER_EVICTED une fois traité par
  // VPlayerPage, même logique que clearPlayerConnectStatus ci-dessus.
  const clearPlayerEvictedStatus = useCallback(() => {
    setPlayerEvictedStatus(null)
  }, [])

  // New game: trigger full reset and transition to NEW_GAME phase
  const newGame = useCallback(() => {
    sendMessage('NEW_GAME', {})
  }, [sendMessage])

  // Quiz metadata: update name, theme, notes, populations, difficulties,
  // language, objectives, hiddenFields (v6.1.0, #137 — payload étendu à 8
  // champs, contract §7). Les 5 derniers paramètres sont optionnels pour
  // compat arrière des appelants existants ; QuestionsPage envoie désormais
  // toujours les 8.
  // Fix M-1 (code-review 20260805-163118) : des défauts `= ''`/`= []`
  // enverraient explicitement ces champs vides sur un appel partiel — le
  // backend distingue "absent" (conservé) de "présent vide" (effacé,
  // contract §7). N'inclure ces clés dans le message que si explicitement
  // fournies, pour que la compat arrière promise par le commentaire
  // ci-dessus soit réelle et pas seulement documentée.
  const updateQuizMeta = useCallback((name, theme, notes, populations, difficulties, language, objectives, hiddenFields) => {
    const msg = { NAME: name, THEME: theme, NOTES: notes }
    if (populations !== undefined) msg.POPULATIONS = populations
    if (difficulties !== undefined) msg.DIFFICULTIES = difficulties
    if (language !== undefined) msg.LANGUAGE = language
    if (objectives !== undefined) msg.OBJECTIVES = objectives
    if (hiddenFields !== undefined) msg.HIDDEN_FIELDS = hiddenFields
    sendMessage('UPDATE_QUIZ_META', msg)
  }, [sendMessage])

  // #137 — arrête le job de génération IA en cours. Prend effet entre deux
  // lots (contract §11) ; les questions déjà écrites sont conservées. Le
  // passage à l'état CANCELLED arrive de façon asynchrone via
  // AI_GENERATION_PROGRESS, pas en retour de cet appel.
  const cancelAiGeneration = useCallback((jobId) => {
    sendMessage('CANCEL_AI_GENERATION', { JOB_ID: jobId })
  }, [sendMessage])

  // Logs: Subscribe to log updates
  const subscribeLogs = useCallback((callback = null) => {
    logCallbackRef.current = callback
    sendMessage('SUBSCRIBE_LOGS', {})
  }, [sendMessage])

  // Logs: Unsubscribe from log updates
  const unsubscribeLogs = useCallback(() => {
    logCallbackRef.current = null
    sendMessage('UNSUBSCRIBE_LOGS', {})
  }, [sendMessage])

  // Logs: Clear local logs state
  const clearLogs = useCallback(() => {
    setLogs([])
  }, [])

  useEffect(() => {
    connect()

    return () => {
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current)
      }
      if (wsRef.current) {
        // #118: neutralize handlers before closing on unmount too — a
        // delayed close event firing after unmount must not call setState
        // on a component that's gone.
        wsRef.current.onopen = null
        wsRef.current.onclose = null
        wsRef.current.onerror = null
        wsRef.current.onmessage = null
        wsRef.current.close()
      }
    }
  }, [connect])

  return {
    status,
    gameState,
    teams,
    bumpers,
    questions,
    fsInfo,
    version,
    clientCounts,
    firmwareInfo,
    aiJob,
    nextQuestion,
    questionPosition,
    awardedTeams,
    creditPoints,
    regieMessage,
    // RAFALE (v8.0.0, #16/#107)
    rafaleAnswer,
    rafaleValidate,
    rafaleInvalidate,
    rafaleSetTeams,
    // Actions
    sendMessage,
    startGame,
    stopGame,
    pauseGame,
    continueGame,
    revealAnswer,
    selectQuestion,
    setRemoteDisplay,
    updateConfig,
    deleteQuestion,
    setBumperPoints,
    setTeamPoints,
    deleteBumper,
    releaseBumperName,
    sendRegieMessage,
    clearRegieMessage,
    setClientType,
    forceReady,
    simulateButton,
    simulatePong,
    flipMemoryCard,
    selectMotionCard,
    flipMotionCard,
    stopMotionTimer,
    revealMotionCard,
    doneMotionCard,
    // New game / Quiz meta
    newGame,
    updateQuizMeta,
    // AI generation (#137)
    cancelAiGeneration,
    // ENTRACTE (#119)
    setEntracte,
    updateEntracteConfig,
    // VPlayer enrollment
    showQRCode,
    hideQRCode,
    connectVirtualPlayer,
    setVirtualPlayerLimit,
    playerConnectStatus,
    clearPlayerConnectStatus,
    playerEvictedStatus,
    clearPlayerEvictedStatus,
    // Logs
    logs,
    subscribeLogs,
    unsubscribeLogs,
    clearLogs,
  }
}
