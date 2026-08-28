import { useState, useMemo, useEffect, useRef } from 'react'
// AnimatePresence removed - layout animations handled by motion.div in TeamCard
import { useGame } from '../hooks/GameContext'
import { useCategoryFilter } from '../hooks/useCategoryFilter'
import { useCategories } from '../hooks/useCategories'
import { categoryMeta } from '../utils/categoryUtils'
import { getRgbColor } from '../utils/colorUtils'
import { sortQuestionsByOrder } from '../utils/questionOrder'
import { sortTeamsByBuzzOrder } from '../utils/buzzOrder'
import { formatArdoiseDelay, sortArdoiseEntries } from '../utils/ardoiseOrder'
import { getMotionGridCols, getMotionCardCoord, getMotionCardPoints, isMotionSecretMode } from '../utils/motionGrid'
import {
  canSelectQuestion as canSelectQuestionRule,
  canStart as canStartRule,
  isPlaying as isPlayingRule,
  canReveal as canRevealRule,
} from '../utils/phaseRules'
import { isTeamReady, prepareWaitReason } from '../utils/prepareWaitReason'
import {
  calcQcmPenalty,
  calcQcmTeamAward,
  calcMemoryScore,
  calcArdoiseDefaultPoints,
  resolvePointsAward,
} from '../utils/pointsAward'
import Button from '../components/Button'
import Card from '../components/Card'
import Timer from '../components/Timer'
import TeamCard from '../components/TeamCard'
import QuestionPreview from '../components/QuestionPreview'
import CategoryBadge from '../components/CategoryBadge'
import QuestionCard from '../components/QuestionCard'
import NetworkWarningBanner from '../components/NetworkWarningBanner'
import RafalePoolAlert from '../components/RafalePoolAlert'
import './GamePage.css'
import '../styles/entracte.css'

export default function GamePage() {
  const {
    gameState,
    teams,
    bumpers,
    questions,
    startGame,
    stopGame,
    pauseGame,
    continueGame,
    revealAnswer,
    selectQuestion,
    setRemoteDisplay,
    setBumperPoints,
    setTeamPoints,
    forceReady,
    simulateButton,
    simulatePong,
    sendMessage,
  } = useGame()

  const [timeInput, setTimeInput] = useState(30)
  const [pointsInput, setPointsInput] = useState(1)

  // MAJEUR-1 (revue de code #155/#156) — pointsInput est un état React
  // local, dont le serveur n'avait connaissance d'aucune façon avant ce
  // mécanisme : /anim créditait question.POINTS brut pendant que /admin
  // créditait pointsInput potentiellement ajusté (ex. manche bonus), deux
  // montants différents pour la même question selon l'interface utilisée.
  // SET_CREDIT_POINTS pousse la valeur ajustée au serveur, qui la rediffuse
  // à /anim via CREDIT_POINTS (contrats/websocket-actions.md §SET_CREDIT_POINTS).
  // Debounce 400ms : ce champ change à chaque frappe/incrément, pas question
  // d'envoyer un message WS par frappe. Ne se déclenche pas au montage (rien
  // n'a encore été ajusté par l'utilisateur à ce moment).
  const isFirstPointsRender = useRef(true)
  useEffect(() => {
    if (isFirstPointsRender.current) {
      isFirstPointsRender.current = false
      return
    }
    const timeoutId = setTimeout(() => {
      sendMessage('SET_CREDIT_POINTS', { POINTS: pointsInput })
    }, 400)
    return () => clearTimeout(timeoutId)
  }, [pointsInput, sendMessage])

  // Memory team selection - always use backend state as source of truth
  const selectedTeams = gameState.MEMORY_PARTICIPATING_TEAMS || []
  // MEMOTION team selection
  const selectedMotionTeams = gameState.MEMOTION_PARTICIPATING_TEAMS || []

  // Group bumpers by team and sort by timestamp
  const teamBumpers = useMemo(() => {
    const grouped = {}
    Object.entries(bumpers).forEach(([mac, bumper]) => {
      const teamName = bumper.TEAM || 'Sans equipe'
      if (!grouped[teamName]) grouped[teamName] = []
      grouped[teamName].push({
        mac,
        name: bumper.NAME,
        score: bumper.SCORE || 0,
        timestamp: bumper.TIME,
        button: bumper.BUTTON,
        ready: bumper.READY === true || bumper.READY === 'TRUE',
        active: bumper.TIME !== undefined && bumper.TIME > 0,
        answerColor: bumper.ANSWER_COLOR,
        hintsAtBuzz: bumper.HINTS_AT_BUZZ || 0, // QCM hints count when player buzzed
        isVPlayer: bumper.IS_VPLAYER === true, // VPlayer flag for multicolor QCM
        firmwareVersion: bumper.FIRMWARE_VERSION || '',
        isOutdated: bumper.IS_OUTDATED === true,
        otaStatus: bumper.OTA_STATUS || '',
        connState: bumper.CONN_STATE || '', // 4-état ("" / orange / red / green) — voir ConnectionBadge
        isVirtual: bumper.IS_VIRTUAL === true, // virtual bumper flag (VJoueurs now show disconnected badge like physical buzzers, see #109)
        ackPending: bumper.ACK_PENDING === true, // strict — undefined does NOT trigger ACK badge
      })
    })
    // Sort bumpers by timestamp within each team
    Object.values(grouped).forEach(bumperList => {
      bumperList.sort((a, b) => {
        const timeA = a.timestamp ?? Infinity
        const timeB = b.timestamp ?? Infinity
        return timeA - timeB
      })
    })
    return grouped
  }, [bumpers])

  // Sort teams by total score (descending), then by timestamp if tied
  // During STARTED/PAUSED/REVEALED phases, sort by buzz time instead (faster first)
  const sortedTeams = useMemo(() => {
    const teamsList = Object.entries(teams)
      .map(([name, data]) => ({
        name,
        ...data,
        buzzers: teamBumpers[name] || [],
      }))

    // Tri par temps de réponse si en STARTED/PAUSED/REVEALED/STOPPED (feature tri-rapidite)
    // Le tri persiste jusqu'à PREPARE (nouvelle question) — règle mutualisée
    // dans utils/buzzOrder.js (sortTeamsByBuzzOrder), consommée aussi par
    // AnimPage.jsx (#156/F6) pour un ordre strictement identique entre les
    // deux interfaces pendant la même partie.
    if (['STARTED', 'PAUSED', 'REVEALED', 'STOPPED'].includes(gameState.phase)) {
      return sortTeamsByBuzzOrder(teamsList, gameState.phase)
    } else {
      // Tri par score hors phases de jeu actif (STOP, PREPARE, READY)
      teamsList.sort((a, b) => {
        const scoreA = a.SCORE ?? 0
        const scoreB = b.SCORE ?? 0
        if (scoreB !== scoreA) return scoreB - scoreA  // Higher score first
        // If tied, sort by timestamp (earlier first)
        const timeA = a.TIME ?? Infinity
        const timeB = b.TIME ?? Infinity
        return timeA - timeB
      })
      return teamsList
    }
  }, [teams, teamBumpers, gameState.phase])

  // Display-only sorting for Memory multi-team mode (by pairs found)
  // This doesn't affect game logic, only the visual order in the Equipes column
  const displayTeams = useMemo(() => {
    // Only show teams that have at least one player (bumper) assigned — empty teams are hidden (#45)
    const teamsWithPlayers = sortedTeams.filter(team => team.buzzers && team.buzzers.length > 0)

    // Only apply Memory sorting in multi-team mode during active phases
    if (gameState.question?.TYPE === 'MEMORY' &&
        gameState.question?.MEMORY_MODE &&
        gameState.question.MEMORY_MODE !== 'SOLO' &&
        ['STARTED', 'PAUSED', 'REVEALED', 'STOPPED'].includes(gameState.phase)) {
      // Create a copy to avoid mutating sortedTeams
      return [...teamsWithPlayers].sort((a, b) => {
        const pairsA = gameState.MEMORY_TEAM_PAIRS?.[a.name] || 0
        const pairsB = gameState.MEMORY_TEAM_PAIRS?.[b.name] || 0
        if (pairsB !== pairsA) return pairsB - pairsA  // More pairs first
        // Tie-breaker: fewer errors first
        const errorsA = gameState.MEMORY_TEAM_ERRORS?.[a.name] || 0
        const errorsB = gameState.MEMORY_TEAM_ERRORS?.[b.name] || 0
        return errorsA - errorsB
      })
    }
    // For all other cases, use the standard sortedTeams order (filtered)
    return teamsWithPlayers
  }, [sortedTeams, gameState.question, gameState.phase, gameState.MEMORY_TEAM_PAIRS, gameState.MEMORY_TEAM_ERRORS])

  // Teams that have at least one VJoueur (ARDOISE panel filter #93)
  const vplayerTeamNames = useMemo(() =>
    new Set(
      Object.values(bumpers)
        .filter(b => b.IS_VPLAYER)
        .map(b => b.TEAM)
        .filter(Boolean)
    ),
    [bumpers]
  )

  // Full team objects for VJoueur teams (tv-preview overlay)
  const vplayerTeams = useMemo(() =>
    displayTeams.filter(team => vplayerTeamNames.has(team.name)),
    [displayTeams, vplayerTeamNames]
  )

  // ARDOISE panel: teams with an answer first, ordered by first-keystroke arrival (#117).
  // Falls back to SUBMITTED_AT when STARTED_AT is 0 (answers recorded before this fix).
  // Teams without an answer keep the current team-list order, appended at the end.
  // #158/F1 — règle extraite dans utils/ardoiseOrder.js (consommée aussi par /anim,
  // AnimArdoiseList.jsx) ; extraction pure, comportement inchangé.
  const sortedArdoiseEntries = useMemo(
    () => sortArdoiseEntries(vplayerTeams, gameState.ARDOISE_ANSWERS),
    [vplayerTeams, gameState.ARDOISE_ANSWERS]
  )

  // Sort questions by ORDER if available, otherwise by ID
  // #149 — mutualisé avec QuestionsPage.jsx (utils/questionOrder.js) : ce
  // tri était dupliqué à l'identique dans les deux pages ; toute évolution
  // (dont "Mélanger les questions") devait d'abord passer par cette
  // mutualisation pour ne pas diverger entre les deux.
  const sortedQuestions = useMemo(() => sortQuestionsByOrder(questions), [questions])

  // Custom categories from API (#95)
  const { categories: apiCategories } = useCategories()
  const customCategories = useMemo(() => apiCategories.filter(c => c.isCustom), [apiCategories])

  // Category filter (shared hook) — passes custom categories for filter support
  const { selectedCategories, availableCategories, filteredQuestions, toggleCategoryFilter, clearCategoryFilters } = useCategoryFilter(sortedQuestions, customCategories)

  // Next unplayed question after current one (for "Question suivante" button)
  const nextUnplayedQuestion = useMemo(() => {
    if (!['STOPPED', 'REVEALED', 'PREPARE', 'READY', 'STARTED', 'PAUSED', 'COUNTDOWN', 'ENROLL', 'NEW_GAME'].includes(gameState.phase)) return null
    const currentId = gameState.question?.ID
    if (!currentId) return null
    const currentIndex = sortedQuestions.findIndex(q => q.ID === currentId)
    for (let i = currentIndex + 1; i < sortedQuestions.length; i++) {
      const q = sortedQuestions[i]
      if (!['STOPPED', 'REVEALED', 'PLAYED'].includes(q.STATUS)) return q
    }
    return null
  }, [sortedQuestions, gameState.phase, gameState.question])

  // MEMOTION: selected card data (from MOTION_CARDS array matching MEMOTION_SELECTED)
  const selectedMotionCard = useMemo(() => {
    const selectedId = gameState.MEMOTION_SELECTED
    if (!selectedId) return null
    const cards = gameState.question?.MOTION_CARDS
    if (!cards || !Array.isArray(cards)) return null
    return cards.find(c => c.ID === selectedId) || null
  }, [gameState.MEMOTION_SELECTED, gameState.question])

  // Calculate Memory score based on matched pairs, errors, and config
  // Rule mutualized in utils/pointsAward.js (calcMemoryScore) — consumed by
  // /admin here and by the animateur page (#155/#156/F1).
  const memoryScore = useMemo(() => {
    return calcMemoryScore(gameState.question, gameState.memoryMatchedPairs?.length || 0, gameState.memoryErrors || 0)
  }, [gameState.question, gameState.memoryMatchedPairs, gameState.memoryErrors])

  // Calculate QCM penalty based on current invalidated count (used for UI display in score input)
  // Rule mutualized in utils/pointsAward.js (calcQcmPenalty).
  const qcmPenalty = useMemo(() => {
    return calcQcmPenalty(gameState.question, pointsInput, gameState.qcmInvalidated?.length || 0)
  }, [gameState.question, gameState.qcmInvalidated, pointsInput])

  // Calculate per-team QCM acquired points for the REVEALED badge
  // Only for teams with a buzzer that answered correctly — rule mutualized
  // in utils/pointsAward.js (calcQcmTeamAward, #157/T1). Only the
  // hasCorrectAnswer branch feeds this map: teams that fall back to the
  // current-invalidated-count penalty (no correct buzzer) are intentionally
  // NOT included here, same as before — see handleBumperClick/onTeamClick
  // below, which fall back to `qcmPenalty` for those.
  const qcmTeamAcquiredPoints = useMemo(() => {
    if (gameState.question?.TYPE !== 'QCM' || gameState.phase !== 'REVEALED') return {}
    const bumpersByTeam = {}
    Object.values(bumpers).forEach(bumper => {
      if (!bumper.TEAM || !bumper.TIME || bumper.TIME === 0) return
      if (!bumpersByTeam[bumper.TEAM]) bumpersByTeam[bumper.TEAM] = []
      bumpersByTeam[bumper.TEAM].push(bumper)
    })
    const result = {}
    Object.entries(bumpersByTeam).forEach(([teamName, teamBumperList]) => {
      const award = calcQcmTeamAward(gameState.question, pointsInput, teamBumperList, gameState.qcmInvalidated?.length || 0)
      if (award.hasCorrectAnswer) result[teamName] = award.amount
    })
    return result
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [gameState.question, gameState.phase, bumpers, pointsInput])

  const handleStartStop = () => {
    if (gameState.phase === 'READY') {
      startGame(timeInput, pointsInput)
    } else if (gameState.phase === 'STARTED' || gameState.phase === 'PAUSED') {
      stopGame()
    }
  }

  const handlePauseContinue = () => {
    if (gameState.phase === 'PAUSED') {
      continueGame()
    } else if (gameState.phase === 'STARTED') {
      pauseGame()
    }
  }

  const handleQuestionSelect = (question, ctrlKey = false) => {
    if (['STOPPED', 'REVEALED', 'PREPARE', 'READY', 'NEW_GAME'].includes(gameState.phase)) {
      if (ctrlKey) {
        // Ctrl+click: select and force to READY (debug)
        selectQuestion(question.ID)
        setTimeInput(parseInt(question.TIME) || 30)
        setPointsInput(parseInt(question.POINTS) || 1)
        // Team selection is reset by backend in Ready()
        forceReady()
        return
      }
      selectQuestion(question.ID)
      setTimeInput(parseInt(question.TIME) || 30)
      setPointsInput(parseInt(question.POINTS) || 1)
      // Team selection is reset by backend in Ready()
    }
  }

  const toggleTeam = (teamName) => {
    const memoryMode = gameState.question?.MEMORY_MODE
    const isSolo = !memoryMode || memoryMode === 'SOLO'
    let newSelection
    if (isSolo) {
      // SOLO: replace current selection with the clicked team
      // If already selected, deselect (empty)
      newSelection = selectedTeams.includes(teamName) ? [] : [teamName]
    } else {
      // Multi: toggle add/remove
      newSelection = selectedTeams.includes(teamName)
        ? selectedTeams.filter(t => t !== teamName)
        : [...selectedTeams, teamName]
    }
    sendMessage('MEMORY_SET_TEAMS', { TEAMS: newSelection })
  }

  const toggleMotionTeam = (teamName) => {
    const motionMode = gameState.question?.MOTION_MODE
    const isSolo = !motionMode || motionMode === 'SOLO'
    let newSelection
    if (isSolo) {
      newSelection = selectedMotionTeams.includes(teamName) ? [] : [teamName]
    } else {
      newSelection = selectedMotionTeams.includes(teamName)
        ? selectedMotionTeams.filter(t => t !== teamName)
        : [...selectedMotionTeams, teamName]
    }
    sendMessage('MEMOTION_SET_TEAMS', { TEAMS: newSelection })
  }

  const handleBumperClick = (bumperMac, ctrlKey = false) => {
    if (ctrlKey && gameState.phase === 'PREPARE') {
      // Ctrl+click in PREPARE: simulate PONG response (debug)
      simulatePong(bumperMac)
      return
    }
    if (ctrlKey && ['STARTED', 'PAUSED'].includes(gameState.phase)) {
      // Ctrl+click: simulate buzzer button press (debug)
      simulateButton(bumperMac, 'A')
      return
    }
    if (gameState.phase === 'STOPPED' || gameState.phase === 'REVEALED') {
      // Only allow points for players who have buzzed
      const bumper = bumpers[bumperMac]
      if (bumper?.TIME && bumper.TIME > 0) {
        // Rule mutualized in utils/pointsAward.js (resolvePointsAward) — Memory
        // score, QCM per-player hint penalty (buzz-time, not current), and
        // POINTS_TARGET (team vs player) all applied there.
        const { amount, target } = resolvePointsAward(gameState.question, pointsInput, {
          hintsAtBuzz: bumper.HINTS_AT_BUZZ || 0,
          memory: { matchedPairs: gameState.memoryMatchedPairs?.length || 0, errors: gameState.memoryErrors || 0 },
        })
        if (target === 'TEAM') {
          const teamName = bumper.TEAM
          if (teamName) {
            setTeamPoints(teamName, amount)
          }
        } else {
          setBumperPoints(bumperMac, amount)
        }
      }
    }
  }

  // #166/F4b — règles d'activation extraites vers utils/phaseRules.js (source
  // unique partagée avec /anim) ; extraction pure, comportement inchangé.
  const isPlaying = isPlayingRule(gameState.phase)
  const canSelectQuestion = canSelectQuestionRule(gameState.phase)
  // REPONSE button active only in STOPPED phase after a question was played
  const canReveal = canRevealRule(gameState.phase, gameState.question)
  // START button only active in READY phase
  const canStart = canStartRule(gameState.phase)

  // RAFALE (v8.0.0, #16/#107, contrat rafale.md §7.2, tâche 26) — pool
  // vide pour le filtre de la manche sélectionnée = démarrage REFUSÉ.
  // `rafalePoolLevel` vient de RafalePoolAlert (onLevelChange), même appel
  // GET /api/rafale/pool que celui déjà utilisé pour l'affichage de
  // l'alerte ci-dessous — pas de second calcul dupliqué ici.
  const isRafaleSelected = gameState.question?.TYPE === 'RAFALE'
  const [rafalePoolLevel, setRafalePoolLevel] = useState(null)
  const rafaleBlocked = isRafaleSelected && rafalePoolLevel === 'blocking'

  // ENTRACTE (#119, C2) — le bouton a déménagé dans Navbar.jsx (visible sur
  // toutes les pages admin, pas seulement ici) ; seul l'estompage de
  // l'interface reste porté par GamePage. .game-page (GamePage.css:23-30) n'a
  // AUCUN descendant position:fixed (RegieMessageBar/Navbar vivent au niveau
  // App.jsx, hors de cet arbre) — filtrer .game-page directement est donc
  // sûr : `filter` ne modifie pas la disposition de la grille qui le porte,
  // seulement son rendu et le containing block de ses éventuels descendants
  // fixed (aucun ici).
  const entracteDim = gameState.entracte ? ' entracte-dim' : ''
  // Transition progressive (#119, C3) — depuis entracteConfig (diffusé, gelé
  // pendant une pause active) — jamais entracteConfigSaved.
  const entracteTransitionStyle = { '--ep-transition': `${gameState.entracteConfig?.TRANSITION_MS ?? 2000}ms` }

  return (
    <>
      {gameState.NETWORK_ONLY_LOCALHOST && <NetworkWarningBanner />}

      <div className={`game-page page${entracteDim}`} style={entracteTransitionStyle}>
      {/* Timer + Display Section (stacked vertically) */}
      <div className="timer-display-section">
        <Card variant="elevated" padding="md" className="timer-card">
          <Timer
            currentTime={gameState.timer}
            totalTime={gameState.totalTime}
            phase={gameState.phase}
            size="lg"
            showPhase={false}
          />
        </Card>
        <Card variant="elevated" padding="sm" className="display-card">
          <span className="toggle-label-vertical">TV</span>
          <div className="toggle-buttons">
            <Button
              variant={gameState.remote === 'GAME' ? 'primary' : 'ghost'}
              size="sm"
              onClick={() => setRemoteDisplay('GAME')}
            >
              Jeu
            </Button>
            <Button
              variant={gameState.remote === 'SCORE' ? 'primary' : 'ghost'}
              size="sm"
              onClick={() => setRemoteDisplay('SCORE')}
            >
              Equipes
            </Button>
            <Button
              variant={gameState.remote === 'PLAYERS' ? 'primary' : 'ghost'}
              size="sm"
              onClick={() => setRemoteDisplay('PLAYERS')}
            >
              Joueurs
            </Button>
            <Button
              variant={gameState.remote === 'PALMARES' ? 'primary' : 'ghost'}
              size="sm"
              onClick={() => setRemoteDisplay('PALMARES')}
            >
              Palmares
            </Button>
          </div>
          <div className="phase-badge-container">
            {gameState.phase === 'STOPPED' && <span className="phase-badge phase-stopped">ARRET</span>}
            {gameState.phase === 'PAUSED' && <span className="phase-badge phase-paused">PAUSE</span>}
            {gameState.phase === 'STARTED' && <span className="phase-badge phase-running">EN COURS</span>}
            {gameState.phase === 'PREPARE' && <span className="phase-badge phase-prepare">PREPARATION</span>}
            {gameState.phase === 'READY' && <span className="phase-badge phase-ready">PRET</span>}
            {gameState.phase === 'REVEALED' && <span className="phase-badge phase-revealed">REPONSE</span>}
            {gameState.phase === 'COUNTDOWN' && <span className="phase-badge phase-countdown">COMPTE A REBOURS</span>}
            {gameState.phase === 'ENROLL' && <span className="phase-badge phase-enroll">INSCRIPTION</span>}
            {gameState.phase === 'NEW_GAME' && <span className="phase-badge phase-new-game">NOUVELLE PARTIE</span>}
          </div>
          {nextUnplayedQuestion && (
            <button
              className="next-question-btn"
              onClick={() => handleQuestionSelect(nextUnplayedQuestion)}
              title={`Aller à la question #${nextUnplayedQuestion.ID}`}
              style={!canSelectQuestion ? { opacity: 0.5, pointerEvents: 'none' } : undefined}
            >
              <span className="nq-label">à suivre : #{nextUnplayedQuestion.ID}</span>
              {nextUnplayedQuestion.CATEGORY && (() => {
                const catMeta = categoryMeta(nextUnplayedQuestion.CATEGORY, customCategories)
                if (!catMeta) return null
                return (
                  <span className="nq-badge nq-badge-cat" style={{ backgroundColor: catMeta.color }}>
                    <CategoryBadge catKey={nextUnplayedQuestion.CATEGORY} customCategories={customCategories} size="sm" chip={false} />
                  </span>
                )
              })()}
              <span className="nq-badge nq-badge-type">{nextUnplayedQuestion.TYPE || 'SPEEDY'}</span>
              <span className="nq-title">
                {(nextUnplayedQuestion.QUESTION || '').substring(0, 30)}{(nextUnplayedQuestion.QUESTION || '').length > 30 ? '…' : ''}
              </span>
            </button>
          )}
          <div className="question-indicators">
            {gameState.question?.CATEGORY && (() => {
              const catMeta = categoryMeta(gameState.question.CATEGORY, customCategories)
              if (!catMeta) return null
              return (
                <div className="category-indicator" style={{ backgroundColor: catMeta.color }} title={catMeta.label}>
                  <CategoryBadge catKey={gameState.question.CATEGORY} customCategories={customCategories} size="sm" chip={false} />
                </div>
              )
            })()}
            {gameState.question?.POINTS_TARGET && (
              <div className={`points-target-indicator ${gameState.question.POINTS_TARGET.toLowerCase()}`} title={gameState.question.POINTS_TARGET === 'TEAM' ? 'Points à l\'équipe' : 'Points au joueur'}>
                {gameState.question.POINTS_TARGET === 'TEAM' ? (
                  <>
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                      <circle cx="9" cy="7" r="4"/>
                      <path d="M17 11c1.66 0 2.99-1.34 2.99-3S18.66 5 17 5c-.32 0-.63.05-.91.14.57.81.9 1.79.9 2.86s-.34 2.04-.9 2.86c.28.09.59.14.91.14z"/>
                      <path d="M3 18v-1c0-2.66 5.33-4 8-4s8 1.34 8 4v1H3z"/>
                      <path d="M17 13c2.05.26 5 1.22 5 3v1h-3v-1.5c0-1.19-.68-2.14-2-2.5z"/>
                    </svg>
                    <span>Equipe</span>
                  </>
                ) : (
                  <>
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                      <circle cx="12" cy="7" r="4"/>
                      <path d="M12 14c-4 0-8 2-8 4v2h16v-2c0-2-4-4-8-4z"/>
                    </svg>
                    <span>Joueur</span>
                  </>
                )}
              </div>
            )}
          </div>
          {/* Memory Stats - inline in display bar (Paires & Erreurs only, Points in input) */}
          {memoryScore && (
            <div className="memory-admin-stats">
              <div className="memory-admin-stat">
                <span className="memory-admin-stat-label">Paires</span>
                <span className={`memory-admin-stat-value ${memoryScore.isComplete ? 'success' : ''}`}>
                  {memoryScore.matchedPairs}/{memoryScore.totalPairs}
                  {memoryScore.isComplete && ' ✓'}
                </span>
              </div>
              <div className="memory-admin-stat">
                <span className="memory-admin-stat-label">Erreurs</span>
                <span className={`memory-admin-stat-value ${memoryScore.errors > 0 ? 'error' : ''}`}>
                  {memoryScore.errors}
                </span>
              </div>
            </div>
          )}
        </Card>

        {/* Memory Team Selection - row layout */}
        {(gameState.phase === 'PREPARE' || gameState.phase === 'READY') &&
         gameState.question?.TYPE === 'MEMORY' && (() => {
          // MEMORY_MODE is empty string for default SOLO (omitempty), treat empty as SOLO
          const isSolo = !gameState.question.MEMORY_MODE || gameState.question.MEMORY_MODE === 'SOLO'
          // Only show teams with at least one player (consistent with main display filter, #45)
          const teamsWithBuzzers = sortedTeams.filter(t => t.buzzers && t.buzzers.length > 0)
          const selected = teamsWithBuzzers.filter(t => selectedTeams.includes(t.name))
          const available = teamsWithBuzzers.filter(t => !selectedTeams.includes(t.name))
          // #172/C2 — motif d'attente PREPARE (buzzers, ou sélection non
          // conforme à la règle du mode) : miroir client-side en lecture
          // seule de participantsConform (#172/B1) — n'influence aucune
          // action, seulement le texte affiché.
          const waitReason = prepareWaitReason(gameState.phase, gameState.question, teamsWithBuzzers, gameState)
          return (
            <div className={`memory-team-selector ${isSolo ? 'solo-mode' : 'multi-mode'}`}>
              <div className="memory-selector-label">
                {isSolo ? 'Mode SOLO' : gameState.question.MEMORY_MODE === 'CHACUN_SON_TOUR' ? 'Chacun son tour' : 'Tant que je gagne'}
                {waitReason && ` · ${waitReason}`}
              </div>
              <div className="memory-chips-row">
                {selected.map((team, idx) => {
                  const teamColor = getRgbColor(team.COLOR)
                  return (
                    <div
                      key={team.name}
                      className={`memory-team-chip selected${isSolo ? ' solo-active' : ''}`}
                      style={{ backgroundColor: teamColor, '--team-color': teamColor }}
                      onClick={!isSolo ? () => toggleTeam(team.name) : undefined}
                      title={!isSolo ? 'Cliquer pour retirer' : undefined}
                    >
                      {!isSolo && <span className="chip-order">{idx + 1}</span>}
                      <span className="chip-name">{team.name}</span>
                      {!isSolo && <span className="chip-action">×</span>}
                    </div>
                  )
                })}
                {selected.length > 0 && available.length > 0 && (
                  <span className="memory-chips-divider">|</span>
                )}
                {available.map(team => {
                  const teamColor = getRgbColor(team.COLOR)
                  // #172/C1 (arbitrage H) — équipe non prête : reste dans la
                  // liste, en retrait visuel, non sélectionnable.
                  const notReady = !isTeamReady(team)
                  return (
                    <div
                      key={team.name}
                      className={`memory-team-chip available${notReady ? ' not-ready' : ''}`}
                      style={{ backgroundColor: teamColor, '--team-color': teamColor }}
                      onClick={notReady ? undefined : () => toggleTeam(team.name)}
                      title={notReady ? 'Buzzer(s) non prêt(s)' : 'Cliquer pour ajouter'}
                    >
                      <span className="chip-name">{team.name}</span>
                      {!notReady && <span className="chip-action">+</span>}
                    </div>
                  )
                })}
              </div>
            </div>
          )
        })()}
        {/* MEMOTION Team Selection - same pattern as Memory */}
        {(gameState.phase === 'PREPARE' || gameState.phase === 'READY') &&
         gameState.question?.TYPE === 'MEMOTION' && (() => {
          const motionMode = gameState.question?.MOTION_MODE
          const isSolo = !motionMode || motionMode === 'SOLO'
          const teamsWithBuzzers = sortedTeams.filter(t => t.buzzers && t.buzzers.length > 0)
          const selected = teamsWithBuzzers.filter(t => selectedMotionTeams.includes(t.name))
          const available = teamsWithBuzzers.filter(t => !selectedMotionTeams.includes(t.name))
          // #172/C2 — même motif d'attente que MEMORY (voir bloc ci-dessus).
          const waitReason = prepareWaitReason(gameState.phase, gameState.question, teamsWithBuzzers, gameState)
          return (
            <div className={`memory-team-selector ${isSolo ? 'solo-mode' : 'multi-mode'}`}>
              <div className="memory-selector-label">
                🃏 MEMOTION · {isSolo ? 'Mode SOLO' : motionMode === 'CHACUN_SON_TOUR' ? 'Chacun son tour' : 'Tant que je gagne'}
                {waitReason && ` · ${waitReason}`}
              </div>
              <div className="memory-chips-row">
                {selected.map((team, idx) => {
                  const teamColor = getRgbColor(team.COLOR)
                  return (
                    <div
                      key={team.name}
                      className={`memory-team-chip selected${isSolo ? ' solo-active' : ''}`}
                      style={{ backgroundColor: teamColor, '--team-color': teamColor }}
                      onClick={!isSolo ? () => toggleMotionTeam(team.name) : undefined}
                      title={!isSolo ? 'Cliquer pour retirer' : undefined}
                    >
                      {!isSolo && <span className="chip-order">{idx + 1}</span>}
                      <span className="chip-name">{team.name}</span>
                      {!isSolo && <span className="chip-action">×</span>}
                    </div>
                  )
                })}
                {selected.length > 0 && available.length > 0 && (
                  <span className="memory-chips-divider">|</span>
                )}
                {available.map(team => {
                  const teamColor = getRgbColor(team.COLOR)
                  // #172/C1 (arbitrage H) — même traitement que MEMORY.
                  const notReady = !isTeamReady(team)
                  return (
                    <div
                      key={team.name}
                      className={`memory-team-chip available${notReady ? ' not-ready' : ''}`}
                      style={{ backgroundColor: teamColor, '--team-color': teamColor }}
                      onClick={notReady ? undefined : () => toggleMotionTeam(team.name)}
                      title={notReady ? 'Buzzer(s) non prêt(s)' : 'Cliquer pour ajouter'}
                    >
                      <span className="chip-name">{team.name}</span>
                      {!notReady && <span className="chip-action">+</span>}
                    </div>
                  )
                })}
              </div>
            </div>
          )
        })()}

        {/* ARDOISE — zone réponses équipes (pattern Memory team zone) */}
        {gameState.question?.TYPE === 'ARDOISE' &&
         ['STARTED', 'STOPPED', 'REVEALED'].includes(gameState.phase) && (
          <div className="ardoise-team-zone">
            <div className="memory-selector-label">Réponses ARDOISE</div>
            <div className="ardoise-answers-list">
              {sortedArdoiseEntries.map(({ team, teamName, answer }, rank) => {
                const teamColor = getRgbColor(team.COLOR)
                const defaultPts = calcArdoiseDefaultPoints(gameState.question, pointsInput)
                const delayLabel = answer ? formatArdoiseDelay(answer, gameState.gameTime) : null
                return (
                  <div
                    key={teamName}
                    className={`ardoise-answer-row ${answer ? 'has-answer' : 'no-answer'} ${rank === 0 && answer ? 'rank-first' : ''}`}
                  >
                    <div className="ardoise-answer-team-name">
                      {answer && <span className="ardoise-answer-rank">{rank + 1}</span>}
                      <span className="ardoise-team-dot" style={{ background: teamColor }} />
                      <span style={{ color: teamColor }}>{teamName}</span>
                      {delayLabel && <span className="ardoise-answer-delay">{delayLabel}</span>}
                    </div>
                    <div className="ardoise-answer-text-row">
                      <span className={`ardoise-answer-text ${answer ? 'has-answer' : 'no-answer'}`}>
                        {answer?.TEXT || '—'}
                      </span>
                      {gameState.phase === 'REVEALED' && (
                        <button
                          className="ardoise-points-btn"
                          onClick={() => setTeamPoints(teamName, defaultPts)}
                          title={`Attribuer ${defaultPts} pts à ${teamName}`}
                        >
                          +{defaultPts} pts
                        </button>
                      )}
                    </div>
                  </div>
                )
              })}
            </div>
          </div>
        )}
      </div>

      {/* Questions Panel - Left */}
      <div className="questions-panel">
        <h2 className="panel-title">Questions</h2>

        {/* Category filter bar */}
        {availableCategories.length > 0 && (
          <div className="category-filter-bar">
            {availableCategories.map(catKey => {
              const meta = categoryMeta(catKey, customCategories)
              if (!meta) return null
              const isActive = selectedCategories.has(catKey)
              return (
                <button
                  key={catKey}
                  className={`category-filter-pill${isActive ? ' active' : ''}`}
                  style={{ '--cat-color': meta.color }}
                  onClick={() => toggleCategoryFilter(catKey)}
                  title={meta.label}
                >
                  <CategoryBadge catKey={catKey} customCategories={customCategories} size="md" chip={false} />
                </button>
              )
            })}
            {selectedCategories.size > 0 && (
              <button
                className="category-filter-reset"
                onClick={clearCategoryFilters}
                title="Reinitialiser les filtres"
              >
                ×
              </button>
            )}
          </div>
        )}

        <div className="questions-list">
          {filteredQuestions.map((question) => {
            const isCurrentQuestion = gameState.question?.ID === question.ID
            const isUnplayed = !['STOPPED', 'REVEALED', 'PLAYED'].includes(question.STATUS)
            const dimmed = !isCurrentQuestion && isUnplayed && !canSelectQuestion
            return (
              <div key={question.ID} style={dimmed ? { opacity: 0.5 } : undefined}>
                <QuestionCard
                  question={question}
                  selected={isCurrentQuestion}
                  compact
                  showStatus
                  showTarget
                  customCategories={customCategories}
                  canSelect={canSelectQuestion}
                  onClick={handleQuestionSelect}
                />
              </div>
            )
          })}
          {filteredQuestions.length === 0 && selectedCategories.size > 0 && (
            <div className="category-filter-empty">
              Aucune question dans cette categorie.
            </div>
          )}
        </div>
      </div>

      {/* Control Panel - Center */}
      <div className="control-panel">
        <Card variant="elevated" padding="lg" className="controls-card">
          <div className="control-inputs">
            <div className="input-group">
              <label htmlFor="time-input">Temps (sec)</label>
              <input
                id="time-input"
                type="number"
                value={timeInput}
                onChange={(e) => setTimeInput(parseInt(e.target.value) || 30)}
                min="1"
                max="300"
                disabled={isPlaying}
              />
            </div>
            <div className="input-group">
              <label htmlFor="points-input">
                Points
                {qcmPenalty && (
                  <span className="qcm-penalty-badge" title={`${qcmPenalty.invalidatedCount} indice(s) donne(s) - penalite ${100 - qcmPenalty.penaltyPercent}%`}>
                    {qcmPenalty.penaltyPercent}%
                  </span>
                )}
              </label>
              {memoryScore ? (
                <input
                  id="points-input"
                  type="number"
                  value={memoryScore.score}
                  readOnly
                  className="memory-score-input"
                  title={`${memoryScore.matchedPairs}×${memoryScore.pointsPerPair}${memoryScore.isComplete ? ` +${memoryScore.completionBonus}` : ''}${memoryScore.errors > 0 ? ` -${memoryScore.errors}×${memoryScore.errorPenalty}` : ''}`}
                />
              ) : qcmPenalty ? (
                <input
                  id="points-input"
                  type="number"
                  value={qcmPenalty.effectivePoints}
                  readOnly
                  className="qcm-penalty-input"
                  title={`Base: ${pointsInput} pts × ${qcmPenalty.penaltyPercent}% = ${qcmPenalty.effectivePoints} pts`}
                />
              ) : (
                <input
                  id="points-input"
                  type="number"
                  value={pointsInput}
                  onChange={(e) => setPointsInput(parseInt(e.target.value) || 1)}
                  min="1"
                  max="100"
                />
              )}
            </div>
          </div>

          {/* RAFALE — panneau de configuration de manche + alerte de pool
              (v8.0.0, #16/#107, contrat rafale.md §7.2, tâche 26 du plan).
              Affiché AVANT le lancement uniquement — pendant la manche
              (STARTED), la conduite se fait depuis /anim (AnimConductPanel,
              boutons VALIDE/INVALIDE) : ce panneau ne duplique pas cette
              zone, il informe et bloque le démarrage si besoin. */}
          {isRafaleSelected && !isPlaying && (
            <div className="rafale-admin-panel">
              <div className="rafale-admin-config">
                <span className="rafale-admin-chip">
                  {(gameState.question.RAFALE_CATEGORIES || []).length} categorie(s)
                </span>
                <span className="rafale-admin-chip">
                  {'★'.repeat(gameState.question.RAFALE_DIFFICULTY || 1)}
                </span>
                <span className="rafale-admin-chip">
                  {gameState.question.RAFALE_MODE || 'SOLO'}
                </span>
                <span className="rafale-admin-chip">
                  {gameState.question.RAFALE_QUESTION_TIME || 3}s/question
                </span>
              </div>
              <RafalePoolAlert
                categories={gameState.question.RAFALE_CATEGORIES || []}
                difficulty={gameState.question.RAFALE_DIFFICULTY}
                roundTime={parseInt(timeInput) || 0}
                questionTime={gameState.question.RAFALE_QUESTION_TIME || 3}
                onLevelChange={setRafalePoolLevel}
              />
            </div>
          )}

          {/* MEMOTION subphase controls — shown when MEMOTION question is STARTED */}
          {gameState.question?.TYPE === 'MEMOTION' && gameState.phase === 'STARTED' && (() => {
            const subphase = gameState.MEMOTION_SUBPHASE
            const cardStates = gameState.MEMOTION_CARD_STATES || {}
            const cardTeams = gameState.MEMOTION_CARD_TEAMS || {}
            const currentTeamColor = gameState.MEMOTION_CURRENT_TEAM_COLOR
            const currentTeam = gameState.MEMOTION_CURRENT_TEAM
            const motionCards = gameState.question?.MOTION_CARDS || []
            const motionCfg = gameState.question?.MOTION_CONFIG
            const diffPts = d => getMotionCardPoints(d, motionCfg)

            if (subphase === 'MEMORIZE') {
              return (
                <div className="memotion-admin-panel">
                  <div className="memotion-memorize-status">
                    <span>Phase mémorisation en cours...</span>
                    <span className="memotion-memorize-timer">{gameState.timer}s</span>
                  </div>
                </div>
              )
            }

            const isSecretMode = isMotionSecretMode(gameState.question)
            const motionCount = motionCards.length
            const motionCols = getMotionGridCols(motionCount)
            const getCoord = getMotionCardCoord

            if (subphase === 'GRID') {
              return (
                <div className="memotion-admin-panel">
                  <div className="memotion-admin-label">
                    Sélectionner une carte sur la preview TV
                    {isSecretMode && <span className="memotion-admin-secret-label"> (mode SECRET)</span>}
                    {currentTeam && (
                      <span
                        className="memotion-admin-current-team"
                        style={{ color: currentTeamColor?.length ? `rgb(${currentTeamColor.join(',')})` : undefined }}
                      > · {currentTeam}</span>
                    )}
                  </div>
                  {isSecretMode && (
                    <div
                      className="memotion-admin-recto-grid"
                      style={{ '--motion-cols': motionCols }}
                    >
                      {motionCards.map((card, idx) => {
                        const isDone = (cardStates[card.ID] || 'UNPLAYED') === 'DONE'
                        return (
                          <div
                            key={card.ID}
                            className={`memotion-admin-recto-card${isDone ? ' done' : ''}`}
                          >
                            <div className="memotion-admin-recto-coord">{getCoord(idx, motionCols)}</div>
                            {card.RECTO_IMAGE && (
                              <img
                                src={card.RECTO_IMAGE}
                                alt={card.RECTO_THEME}
                                className="memotion-admin-recto-img"
                              />
                            )}
                            <div className="memotion-admin-recto-theme">{card.RECTO_THEME}</div>
                          </div>
                        )
                      })}
                    </div>
                  )}
                </div>
              )
            }

            if (subphase === 'SELECTED' && selectedMotionCard) {
              const diff = selectedMotionCard.DIFFICULTY || 1
              return (
                <div className="memotion-admin-panel">
                  <div className="memotion-admin-label">Carte sélectionnée</div>
                  <div className="memotion-admin-card-info">
                    <span className="memotion-admin-theme">{selectedMotionCard.RECTO_THEME}</span>
                    <span className="memotion-admin-diff">{'★'.repeat(diff)} · {diffPts(diff)}pt</span>
                  </div>
                  <div className="memotion-admin-actions">
                    <Button variant="primary" size="md" onClick={() => sendMessage('MEMOTION_FLIP', {})}>
                      ▶ DÉMARRER
                    </Button>
                    <Button variant="ghost" size="sm" onClick={() => sendMessage('MEMOTION_DONE', { CARD_ID: gameState.MEMOTION_SELECTED, WINNER_TEAM: '' })}>
                      ANNULER
                    </Button>
                  </div>
                </div>
              )
            }

            if (subphase === 'QUESTION' && selectedMotionCard) {
              const diff = selectedMotionCard.DIFFICULTY || 1
              return (
                <div className="memotion-admin-panel">
                  <div className="memotion-admin-label">Question</div>
                  <div className="memotion-admin-card-info">
                    <span className="memotion-admin-theme">{selectedMotionCard.RECTO_THEME}</span>
                    <span className="memotion-admin-diff">{'★'.repeat(diff)} · {diffPts(diff)}pt</span>
                  </div>
                  {selectedMotionCard.QUESTION_TEXT && (
                    <p className="memotion-admin-qtext">{selectedMotionCard.QUESTION_TEXT}</p>
                  )}
                  {selectedMotionCard.QUESTION_IMAGE && (
                    <img src={selectedMotionCard.QUESTION_IMAGE} alt="Question" className="memotion-admin-img" />
                  )}
                  <div className="memotion-admin-actions">
                    {gameState.timer > 0 && (
                      <Button variant="warning" size="sm" onClick={() => sendMessage('MEMOTION_STOP_TIMER', {})}>
                        STOP TIMER
                      </Button>
                    )}
                    {!(gameState.timer > 0) && (
                      <Button variant="primary" size="md" onClick={() => sendMessage('MEMOTION_REVEAL', {})}>
                        RÉVÉLER
                      </Button>
                    )}
                    <Button variant="ghost" size="sm" onClick={() => sendMessage('MEMOTION_DONE', { CARD_ID: gameState.MEMOTION_SELECTED, WINNER_TEAM: '' })}>
                      SANS VAINQUEUR
                    </Button>
                  </div>
                </div>
              )
            }

            if (subphase === 'REVEAL' && selectedMotionCard) {
              const diff = selectedMotionCard.DIFFICULTY || 1
              const currentTeamData = sortedTeams.find(t => t.name === currentTeam)
              const currentTeamChipColor = currentTeamData ? getRgbColor(currentTeamData.COLOR) : undefined
              return (
                <div className="memotion-admin-panel">
                  <div className="memotion-admin-label">Réponse</div>
                  <div className="memotion-admin-card-info">
                    <span className="memotion-admin-theme">{selectedMotionCard.RECTO_THEME}</span>
                    <span className="memotion-admin-diff">{'★'.repeat(diff)} · {diffPts(diff)}pt</span>
                  </div>
                  {selectedMotionCard.ANSWER_TEXT && (
                    <p className="memotion-admin-qtext">{selectedMotionCard.ANSWER_TEXT}</p>
                  )}
                  {selectedMotionCard.ANSWER_IMAGE && (
                    <img src={selectedMotionCard.ANSWER_IMAGE} alt="Reponse" className="memotion-admin-img" />
                  )}
                  <p className="memotion-admin-winner-label">Qui a bien répondu ?</p>
                  <div className="memotion-winner-chips">
                    {currentTeam && (
                      <button
                        className="memotion-winner-chip"
                        style={{ backgroundColor: currentTeamChipColor }}
                        onClick={() => sendMessage('MEMOTION_DONE', { CARD_ID: gameState.MEMOTION_SELECTED, WINNER_TEAM: currentTeam })}
                      >
                        {currentTeam}
                      </button>
                    )}
                    <button
                      className="memotion-winner-chip none"
                      onClick={() => sendMessage('MEMOTION_DONE', { CARD_ID: gameState.MEMOTION_SELECTED, WINNER_TEAM: '' })}
                    >
                      Perdu
                    </button>
                  </div>
                </div>
              )
            }

            return null
          })()}

          <div className="control-buttons">
            <div className="control-buttons-row">
              <Button
                variant={isPlaying ? 'danger' : 'success'}
                size="lg"
                onClick={handleStartStop}
                title={rafaleBlocked && !isPlaying ? 'Pool RAFALE vide pour ce filtre — démarrage refusé (contrat §7.2)' : undefined}
                disabled={!isPlaying && (!canStart || rafaleBlocked)}
              >
                {isPlaying ? 'STOP' : 'START'}
              </Button>

              <Button
                variant={gameState.phase === 'PAUSED' ? 'primary' : 'warning'}
                size="lg"
                onClick={handlePauseContinue}
                disabled={!isPlaying}
              >
                {gameState.phase === 'PAUSED' ? 'CONTINUER' : 'PAUSE'}
              </Button>
            </div>

            <Button
              variant="secondary"
              size="md"
              onClick={revealAnswer}
              disabled={!canReveal}
            >
              REPONSE
            </Button>

          </div>
        </Card>
      </div>

      {/* TV Preview (Row 2 Col 2) */}
      <div className="tv-preview-container">
        <QuestionPreview />
      </div>

      {/* Right Panel - Teams */}
      <div className="right-panel">
        <div className="teams-section">
          <h2 className="section-title">Equipes</h2>
          <div className="teams-grid">
              {displayTeams.map((team, index) => (
                <TeamCard
                  key={team.name}
                  name={team.name}
                  color={team.COLOR}
                  score={team.SCORE || 0}
                  teamPoints={team.TEAM_POINTS || 0}
                  ready={team.READY === true || team.READY === 'TRUE'}
                  active={team.TIME !== undefined && team.TIME > 0}
                  timestamp={team.TIME}
                  gameTime={gameState.gameTime}
                  gamePhase={gameState.phase}
                  rank={index + 1}
                  showResponseTime={['STARTED', 'PAUSED', 'REVEALED'].includes(gameState.phase)}
                  waitingForReady={['PREPARE', 'READY'].includes(gameState.phase)}
                  waitingForBuzz={['STARTED', 'PAUSED'].includes(gameState.phase)}
                  questionType={gameState.question?.TYPE || null}
                  qcmPenaltyConfig={gameState.question?.TYPE === 'QCM' && gameState.question?.QCM_HINTS_ENABLED ? {
                    penalty1: gameState.question?.QCM_PENALTY_1 || 0.67,
                    penalty2: gameState.question?.QCM_PENALTY_2 || 0.33,
                  } : null}
                  qcmAcquiredPoints={qcmTeamAcquiredPoints[team.name] !== undefined ? qcmTeamAcquiredPoints[team.name] : null}
                  memoryStats={gameState.question?.TYPE === 'MEMORY' &&
                               gameState.question?.MEMORY_MODE !== 'SOLO' &&
                               gameState.MEMORY_TEAM_PAIRS?.[team.name] !== undefined ? {
                    pairs: gameState.MEMORY_TEAM_PAIRS[team.name] || 0,
                    errors: gameState.MEMORY_TEAM_ERRORS?.[team.name] || 0,
                    totalPairs: gameState.question?.MEMORY_PAIRS?.length || 0,
                    pointsPerPair: gameState.question?.MEMORY_CONFIG?.POINTS_PER_PAIR || 10,
                    errorPenalty: gameState.question?.MEMORY_CONFIG?.ERROR_PENALTY || 0,
                    completionBonus: gameState.question?.MEMORY_CONFIG?.COMPLETION_BONUS || 0,
                  } : null}
                  onTeamClick={(teamName) => {
                    if (['STOPPED', 'REVEALED'].includes(gameState.phase)) {
                      // For Memory multi-team, calculate team-specific points
                      // For QCM with hints, use penalty-adjusted points
                      // Otherwise use pointsInput
                      let pointsToAward = pointsInput
                      if (gameState.question?.TYPE === 'MEMORY' &&
                          gameState.question?.MEMORY_MODE !== 'SOLO' &&
                          gameState.MEMORY_TEAM_PAIRS?.[teamName] !== undefined) {
                        // Calculate team-specific Memory points — same rule as
                        // solo (utils/pointsAward.js calcMemoryScore), only the
                        // pairs/errors source differs (per-team vs global).
                        const pairs = gameState.MEMORY_TEAM_PAIRS[teamName] || 0
                        const errors = gameState.MEMORY_TEAM_ERRORS?.[teamName] || 0
                        const teamMemoryScore = calcMemoryScore(gameState.question, pairs, errors)
                        if (teamMemoryScore) pointsToAward = teamMemoryScore.score
                      } else if (memoryScore) {
                        // Solo Memory mode - use global score
                        pointsToAward = memoryScore.score
                      } else if (gameState.question?.TYPE === 'QCM' && gameState.question?.QCM_HINTS_ENABLED) {
                        // QCM: rule mutualized in utils/pointsAward.js
                        // (calcQcmTeamAward, #157/T1) — per-player penalty if
                        // a correct buzzer exists for this team, else falls
                        // back to the current-invalidated-count penalty.
                        // #157 note: this now applies the same way in STOPPED
                        // and REVEALED. Before this refactor, a STOPPED-phase
                        // click always used the fallback, because
                        // qcmTeamAcquiredPoints was gated to REVEALED only
                        // (built for the acquired-points badge) and this
                        // handler read it as a proxy for "team answered
                        // correctly" — an accidental coupling between a
                        // display gate and a credit decision, with no test
                        // coverage either way. Unifying both consumers onto
                        // calcQcmTeamAward removes that coupling.
                        const teamBumperList = Object.values(bumpers).filter(b => b.TEAM === teamName)
                        const award = calcQcmTeamAward(gameState.question, pointsInput, teamBumperList, gameState.qcmInvalidated?.length || 0)
                        pointsToAward = award.amount
                      }
                      setTeamPoints(teamName, pointsToAward)
                    }
                  }}
                  onPlayerClick={(bumperMac, ctrlKey) => handleBumperClick(bumperMac, ctrlKey)}
                  buzzers={team.buzzers.map(b => ({
                    ...b,
                    onClick: (e) => handleBumperClick(b.mac, e?.ctrlKey)
                  }))}
                />
              ))}
            </div>
        </div>
      </div>
    </div>
    </>
  )
}

