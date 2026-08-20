import { Fragment, useEffect, useMemo, useState, useCallback, useRef } from 'react'
import NoSleep from 'nosleep.js'
import { motion, AnimatePresence } from 'framer-motion'
import confetti from 'canvas-confetti'
import { useGame } from '../hooks/GameContext'
import { useCategories } from '../hooks/useCategories'
import { CATEGORIES, categoryMeta } from '../utils/categoryUtils'
import Timer from '../components/Timer'
import Podium from '../components/Podium'
import QRCodeOverlay from '../components/QRCodeOverlay'
import QRCodeDisplay from '../components/QRCodeDisplay'
import EntractePanel from '../components/EntractePanel'
import { getCategoryColor } from '../constants/colors'
import { getRgbColor } from '../utils/colorUtils'
import { escapeWifiString } from '../utils/wifiUtils'
import { buildMemoryCards, getMemoryGridCols, getMemoryGridRows } from '../utils/memoryGrid'
import { getMotionGridCols, getMotionGridRows, getMotionCardCoord, getMotionCardPoints, isMotionSecretMode } from '../utils/motionGrid'
import './PlayerDisplay.css'
import '../styles/neon.css'
import '../styles/entracte.css'

// QCM answer colors
const QCM_COLORS = {
  RED: { label: 'Rouge', color: '#ef4444', letter: 'A' },
  GREEN: { label: 'Vert', color: '#22c55e', letter: 'B' },
  YELLOW: { label: 'Jaune', color: '#eab308', letter: 'C' },
  BLUE: { label: 'Bleu', color: '#3b82f6', letter: 'D' },
}

// Mapping from button press (A, B, C, D) to QCM color
const BUTTON_TO_QCM_COLOR = {
  'A': 'RED',
  'B': 'GREEN',
  'C': 'YELLOW',
  'D': 'BLUE',
}

// v6.1.0 (#137, T2.4) — écran NEW_GAME statique (contrainte TV, aucun scroll) :
// au plus 2 badges par famille (publics/difficultés), le surplus rendu en
// "+N" (maquette 137-batch2b-globaux-multiples.html §vue 3, règle 2).
const QUIZ_BADGE_CAP_PER_FAMILY = 2

// Renders category badge (icon+label or image+label) OR ✋ PRÉPAREZ-VOUS fallback.
// Used by all game types in READY phase — single source of truth for READY display.
const GAME_TYPE_COLORS = {
  SPEEDY:   { color: '#60a5fa', shadow: 'rgba(96,165,250,0.6)' },
  ARDOISE:  { color: '#fcd34d', shadow: 'rgba(252,211,77,0.6)' },
  QCM:      { color: '#34d399', shadow: 'rgba(52,211,153,0.6)' },
  MEMOTION: { color: '#f472b6', shadow: 'rgba(244,114,182,0.6)' },
}

function ReadyCategoryDisplay({ catKey, customCategories, variant, gameType }) {
  const catMeta = categoryMeta(catKey, customCategories)
  const typeStyle = GAME_TYPE_COLORS[gameType] ?? { color: '#fff', shadow: 'rgba(255,255,255,0.5)' }
  // #114 — contour sombre (text-shadow multi-directionnel, fallback pour navigateurs sans
  // -webkit-text-stroke comme Firefox) combiné au glow coloré existant : une valeur inline
  // écrase toute règle CSS text-shadow, donc les deux doivent être fusionnés ici plutôt que
  // séparés entre JSX et .ready-game-type. Voir maquette docs/mockups/tv-memory-contrast-108-114.html
  const typeOutline = '-1.5px -1.5px 0 rgba(0,0,0,0.85), 1.5px -1.5px 0 rgba(0,0,0,0.85), -1.5px 1.5px 0 rgba(0,0,0,0.85), 1.5px 1.5px 0 rgba(0,0,0,0.85)'
  const typeTextShadow = `${typeOutline}, 0 0 20px ${typeStyle.shadow}, 0 0 50px ${typeStyle.shadow}`

  // Icône unifiée : image ou emoji
  const iconInner = catMeta ? (
    catMeta.imageURL
      ? <img src={catMeta.imageURL} alt={catMeta.label} className="ready-category-img" />
      : <span className={variant === 'memory' ? 'category-badge-icon' : 'ready-category-icon'}>{catMeta.icon}</span>
  ) : null

  // Deux motion.div imbriqués : entrée spring (une fois) + idle wobble (boucle)
  const iconWrapper = iconInner ? (
    <motion.div
      initial={{ scale: 0, rotate: -15 }}
      animate={{ scale: 1, rotate: 0 }}
      transition={{ type: 'spring', stiffness: 250, damping: 12 }}
    >
      <motion.div
        className="ready-category-icon-wrapper"
        animate={{ scale: [1, 1.25, 1], rotate: [0, 5, -5, 0] }}
        transition={{ duration: 1.8, repeat: Infinity, ease: 'easeInOut' }}
      >
        {iconInner}
      </motion.div>
    </motion.div>
  ) : null

  if (variant === 'memory') {
    return (
      <div className="ready-category-display">
        {gameType && (
        <motion.span
          className="ready-game-type"
          style={{ color: typeStyle.color, textShadow: typeTextShadow }}
          initial={{ opacity: 0, y: -24, scale: 0.7 }}
          animate={{ opacity: [null, 1, 0.85, 1], y: 0, scale: [null, 1.2, 1, 1.06, 1] }}
          transition={{ duration: 1.6, times: [0, 0.2, 0.6, 0.8, 1], repeat: Infinity, repeatDelay: 0.8 }}
        >
          {gameType}
        </motion.span>
      )}
        {catMeta ? (
          <motion.div
            className="category-badge-inline category-badge-large"
            style={{ backgroundColor: catMeta.color }}
            animate={{ scale: [1, 1.05, 1] }}
            transition={{ duration: 0.6, repeat: Infinity }}
          >
            {iconWrapper}
            <span className="category-badge-label">{catMeta.label}</span>
          </motion.div>
        ) : (
          <>
            <span className="prepare-emoji">🔔</span>
            <span className="prepare-text">PREPAREZ-VOUS</span>
          </>
        )}
      </div>
    )
  }

  return (
    <div className="ready-category-display">
      {gameType && (
        <motion.span
          className="ready-game-type"
          style={{ color: typeStyle.color, textShadow: typeTextShadow }}
          initial={{ opacity: 0, y: -24, scale: 0.7 }}
          animate={{ opacity: [null, 1, 0.85, 1], y: 0, scale: [null, 1.2, 1, 1.06, 1] }}
          transition={{ duration: 1.6, times: [0, 0.2, 0.6, 0.8, 1], repeat: Infinity, repeatDelay: 0.8 }}
        >
          {gameType}
        </motion.span>
      )}
      {catMeta ? (
        <>
          {iconWrapper}
          <motion.span
            className="ready-category-name"
            style={{ backgroundColor: catMeta.color }}
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: [null, 1, 0.75, 1], y: 0 }}
            transition={{ duration: 1.4, repeat: Infinity, repeatDelay: 0.5 }}
          >
            {catMeta.label}
          </motion.span>
        </>
      ) : (
        <>
          <motion.span
            className="ready-emoji"
            animate={{ scale: [1, 1.3, 1] }}
            transition={{ duration: 0.4, repeat: Infinity }}
          >
            ✋
          </motion.span>
          <motion.span
            className="ready-text"
            animate={{ opacity: [1, 0.7, 1] }}
            transition={{ duration: 0.6, repeat: Infinity }}
          >
            PRÉPAREZ-VOUS
          </motion.span>
        </>
      )}
    </div>
  )
}

export default function PlayerDisplay({ playerName = null, playerNameColor = null, teamName = null, teamColor = null, isVPlayer = false, onMediaClick = null, onQCMAnswer = null, vplayerHasBuzzed = false }) {
  const { gameState, teams, bumpers, flipMemoryCard, showQRCode, selectMotionCard } = useGame()
  const { categories: apiCategories } = useCategories()
  const [previousRanking, setPreviousRanking] = useState({})
  const [changedTeams, setChangedTeams] = useState({})
  const [palmares, setPalmares] = useState([]) // For PALMARES view (v5.7.10 — GET /palmares)
  const [previousPlayerScores, setPreviousPlayerScores] = useState({})
  const [changedPlayers, setChangedPlayers] = useState({})
  const [pointsAnimation, setPointsAnimation] = useState(null) // {name, points, color}
  const [justMatchedPairs, setJustMatchedPairs] = useState([]) // Track newly matched pairs for animation
  const [prevMatchedPairs, setPrevMatchedPairs] = useState([]) // Previous matched pairs
  const [revealedPairs, setRevealedPairs] = useState([]) // Pairs revealed progressively during REVEAL phase
  const [prevPhase, setPrevPhase] = useState(null) // Track phase changes
  const [countdownVisibleCards, setCountdownVisibleCards] = useState([]) // Cards progressively revealed during countdown
  const [cascadeRevealDone, setCascadeRevealDone] = useState(false) // True when all cards are revealed in cascade
  const [cascadeHideDone, setCascadeHideDone] = useState(false) // True when all cards are hidden after cascade
  const [cascadeHideStarted, setCascadeHideStarted] = useState(false) // True when cascade hide has been triggered
  const [isFullscreen, setIsFullscreen] = useState(false)
  const tvWakeLockRef = useRef(null)
  const tvNoSleepRef = useRef(null)
  const motionCardRefs = useRef({})
  const [localCountdown, setLocalCountdown] = useState(null) // Local countdown that starts after cascade reveal is done
  const [wifiConfig, setWifiConfig] = useState(null) // { ssid, password } for enrollment WiFi QR code
  const [ngBgIndex, setNgBgIndex] = useState(0) // Current index for NEW_GAME background rotation

  // Fetch WiFi config for enrollment QR code (#51) — always set, even if SSID empty
  useEffect(() => {
    if (gameState.phase !== 'ENROLL' || isVPlayer) return
    fetch('/api/wifi/defaults')
      .then(r => r.ok ? r.json() : null)
      .then(data => {
        setWifiConfig({ ssid: data?.ssid || '', password: data?.password || '' })
      })
      .catch(() => { setWifiConfig({ ssid: '', password: '' }) })
  }, [gameState.phase, isVPlayer])

  // NEW_GAME backgrounds rotation (client-side, based on each image's duration)
  const newGameBgs = gameState.newGameBackgrounds || []

  // v6.1.0 (#137, T2.4) — familles de badges de l'écran NEW_GAME : on masque
  // D'ABORD (QUIZ_HIDDEN_FIELDS, préférence d'affichage — les valeurs restent
  // dans le payload TV, cf. game-state.md "diffusion — préférence d'affichage
  // ≠ confidentialité"), puis on plafonne CE QUI RESTE (maquette §vue 3,
  // règle 3 : "les deux règles se composent dans cet ordre"). THEME est géré
  // séparément (bloc .new-game-quiz-theme, pas une famille de badges).
  const quizHiddenFields = gameState.quizHiddenFields || []
  const themeHiddenFromTV = quizHiddenFields.includes('THEME')
  const quizBadgeFamilies = useMemo(() => {
    const families = []
    if (!quizHiddenFields.includes('POPULATIONS') && gameState.quizPopulations?.length > 0) {
      families.push({ key: 'populations', values: gameState.quizPopulations })
    }
    if (!quizHiddenFields.includes('DIFFICULTIES') && gameState.quizDifficulties?.length > 0) {
      families.push({ key: 'difficulties', values: gameState.quizDifficulties })
    }
    if (!quizHiddenFields.includes('LANGUAGE') && gameState.quizLanguage) {
      families.push({ key: 'language', values: [gameState.quizLanguage] })
    }
    return families
  }, [quizHiddenFields, gameState.quizPopulations, gameState.quizDifficulties, gameState.quizLanguage])
  useEffect(() => {
    if (newGameBgs.length <= 1) return
    const current = newGameBgs[ngBgIndex] || newGameBgs[0]
    const duration = (current?.duration || 10) * 1000
    const timer = setTimeout(() => {
      setNgBgIndex(i => (i + 1) % newGameBgs.length)
    }, duration)
    return () => clearTimeout(timer)
  }, [ngBgIndex, newGameBgs])

  // DEBUG: Log gameState
  useEffect(() => {
    console.log('[PlayerDisplay] gameState.question:', gameState.question)
    console.log('[PlayerDisplay] gameState.phase:', gameState.phase)
    console.log('[PlayerDisplay] gameState.remote:', gameState.remote)
  }, [gameState.question, gameState.phase, gameState.remote])

  // Default question image path - always valid (backend serves custom or embedded SVG fallback)
  const defaultQuestionImage = '/api/config/default-image'

  // Check if admin mode (for pair hints visibility)
  const isAdminPreview = useMemo(() => {
    const params = new URLSearchParams(window.location.search)
    return params.get('admin') === 'true'
  }, [])

  // Fetch palmares for PALMARES view (v5.7.10 — single endpoint, no client-side join)
  useEffect(() => {
    if (gameState.remote !== 'PALMARES') return

    const fetchPalmares = async () => {
      try {
        const response = await fetch('/palmares')
        if (response.ok) {
          const data = await response.json()
          setPalmares(data || [])
        }
      } catch (error) {
        console.error('Failed to fetch palmares:', error)
      }
    }

    fetchPalmares()
    const interval = setInterval(fetchPalmares, 5000)
    return () => clearInterval(interval)
  }, [gameState.remote])

  // Sort teams by score for scoreboard with rank calculation
  // #127 — pour un VJoueur, ce classement n'est JAMAIS rendu sauf quand l'animateur
  // bascule le remote sur 'SCORE' (vue "Classement Equipes", l.1012 — pas de garde
  // !isVPlayer sur cette branche, elle s'affiche aussi côté VJoueur : casser ce cas
  // serait une régression visible). Repli sûr : ne trier que quand ce sera
  // effectivement affiché — élimine le tri O(n log n) exécuté à chaque UPDATE reçu
  // pendant la rafale PREPARE→READY (cause racine #127), sans toucher à la vue SCORE.
  const sortedTeams = useMemo(() => {
    if (isVPlayer && gameState.remote !== 'SCORE') return []
    const sorted = Object.entries(teams)
      .map(([name, data]) => ({
        name,
        score: data.SCORE || 0,
        color: data.COLOR,
      }))
      .sort((a, b) => b.score - a.score)

    let currentRank = 1
    let lastScore = null
    let lastRank = 1
    return sorted.map((team, index) => {
      if (index > 0 && team.score === lastScore) {
        return { ...team, rank: lastRank }
      }
      currentRank = index + 1
      lastScore = team.score
      lastRank = currentRank
      return { ...team, rank: currentRank }
    })
  }, [teams, isVPlayer, gameState.remote])

  // Sort players (bumpers) by score
  // #127 — même raisonnement que sortedTeams ci-dessus, pour la vue "Classement
  // Joueurs" (remote='PLAYERS', l.1085, également non gatée sur !isVPlayer).
  //
  // Fix post-review (code-reviewer #127, Problème Majeur #2) : pendant PREPARE/READY,
  // le serveur réduit `bumpers` au seul bumper du VJoueur destinataire (contrat
  // vplayer-payload-filter.md §2) — rien côté admin n'empêche de basculer sur la vue
  // "Joueurs" pendant ces deux phases (GamePage.jsx, bouton sans garde de phase). Sans
  // précaution, un VJoueur y verrait un classement à 1 seule entrée (lui-même),
  // trompeur. `teams` n'est jamais réduit (contrat), donc sortedTeams n'est pas
  // concerné — uniquement sortedPlayers. Pendant ces deux phases, on réaffiche donc le
  // dernier classement complet connu (figé) plutôt que de recalculer sur le payload
  // réduit : le classement réel ne bouge de toute façon pas pendant PREPARE/READY
  // (aucun buzz/score possible), donc rejouer la dernière valeur est fidèle, pas périmé.
  const lastFullSortedPlayersRef = useRef([])
  const sortedPlayers = useMemo(() => {
    if (isVPlayer && gameState.remote !== 'PLAYERS') return []
    if (isVPlayer && ['PREPARE', 'READY'].includes(gameState.phase)) {
      return lastFullSortedPlayersRef.current
    }
    const sorted = Object.entries(bumpers)
      .map(([mac, data]) => ({
        mac,
        name: data.NAME || mac.slice(-6),
        score: data.SCORE || 0,
        team: data.TEAM,
        teamColor: teams[data.TEAM]?.COLOR,
      }))
      .sort((a, b) => b.score - a.score)

    let currentRank = 1
    let lastScore = null
    let lastRank = 1
    const ranked = sorted.map((player, index) => {
      if (index > 0 && player.score === lastScore) {
        return { ...player, rank: lastRank }
      }
      currentRank = index + 1
      lastScore = player.score
      lastRank = currentRank
      return { ...player, rank: currentRank }
    })
    lastFullSortedPlayersRef.current = ranked
    return ranked
  }, [bumpers, teams, isVPlayer, gameState.remote, gameState.phase])

  // Detect team ranking changes
  // #127 — gardé aligné sur le gating de sortedTeams ci-dessus : sans ce retour
  // anticipé, un VJoueur hors vue SCORE ré-exécuterait quand même setPreviousRanking
  // avec un nouvel objet {} à chaque UPDATE (sortedTeams=[] fait toujours tourner ce
  // forEach à vide), déclenchant un re-render inutile à chaque message de la rafale.
  useEffect(() => {
    if (isVPlayer && gameState.remote !== 'SCORE') return
    const currentRanking = {}
    const changes = {}

    sortedTeams.forEach((team, index) => {
      currentRanking[team.name] = { position: index, score: team.score }

      const prev = previousRanking[team.name]
      if (prev) {
        if (prev.position > index) {
          changes[team.name] = 'up'
        } else if (prev.score !== team.score) {
          changes[team.name] = 'score'
        }
      }
    })

    if (Object.keys(changes).length > 0) {
      setChangedTeams(changes)
      setTimeout(() => setChangedTeams({}), 2000)
    }

    setPreviousRanking(currentRanking)
  }, [sortedTeams, isVPlayer, gameState.remote])

  // Detect player score changes — #127, même raisonnement que ci-dessus.
  useEffect(() => {
    if (isVPlayer && gameState.remote !== 'PLAYERS') return
    const currentScores = {}
    const changes = {}

    sortedPlayers.forEach((player) => {
      currentScores[player.mac] = player.score

      const prevScore = previousPlayerScores[player.mac]
      if (prevScore !== undefined && prevScore !== player.score) {
        changes[player.mac] = player.score > prevScore ? 'up' : 'down'
      }
    })

    if (Object.keys(changes).length > 0) {
      setChangedPlayers(changes)
      setTimeout(() => setChangedPlayers({}), 2000)
    }

    setPreviousPlayerScores(currentScores)
  }, [sortedPlayers, isVPlayer, gameState.remote])

  // Detect score changes and trigger celebration animation in GAME view
  // #127 — DÉLIBÉRÉMENT INDÉPENDANT de sortedTeams/previousRanking (ci-dessus) :
  // cette célébration est rendue sans garde !isVPlayer (JSX plus bas, vue GAME) et
  // doit se déclencher précisément quand remote==='GAME' — soit l'exact opposé de la
  // condition qui vide sortedTeams pour un VJoueur. La coupler à sortedTeams aurait
  // soit cassé la célébration pendant le jeu (si gardée alignée sur 'SCORE'), soit
  // réintroduit le tri complet à chaque UPDATE pendant la rafale PREPARE→READY (si
  // laissée inconditionnelle). Calcul dédié en O(n), sans tri, sur `teams` brut.
  const previousTeamScoresRef = useRef({})
  useEffect(() => {
    const previous = previousTeamScoresRef.current
    const currentScores = {}

    Object.entries(teams).forEach(([name, data]) => {
      const score = data.SCORE || 0
      currentScores[name] = score

      const prevScore = previous[name]
      if (prevScore !== undefined && score > prevScore) {
        setPointsAnimation({
          name,
          points: score - prevScore,
          color: data.COLOR,
        })
        triggerCelebration(data.COLOR)
        setTimeout(() => setPointsAnimation(null), 2500)
      }
    })

    previousTeamScoresRef.current = currentScores
  }, [teams])

  // Background index is now server-synchronized via gameState.currentBackgroundIndex
  // No local cycling needed - server broadcasts BACKGROUND_CHANGE to all clients

  // Detect newly matched Memory pairs for celebration animation
  useEffect(() => {
    const currentMatched = gameState.memoryMatchedPairs || []

    // Find newly matched pairs (in current but not in previous)
    const newlyMatched = currentMatched.filter(pairId => !prevMatchedPairs.includes(pairId))

    if (newlyMatched.length > 0) {
      // Wait for flip animation to complete (0.6s) before starting celebration
      setTimeout(() => {
        // Add to justMatched for celebration animation
        setJustMatchedPairs(prev => [...prev, ...newlyMatched])

        // Remove from justMatched after celebration animation completes (0.8s)
        setTimeout(() => {
          setJustMatchedPairs(prev => prev.filter(id => !newlyMatched.includes(id)))
        }, 800)
      }, 600) // Delay = flip animation duration
    }

    setPrevMatchedPairs(currentMatched)
  }, [gameState.memoryMatchedPairs])

  // Reset all Memory local state when entering PREPARE (new game)
  useEffect(() => {
    if (gameState.phase === 'PREPARE') {
      setJustMatchedPairs([])
      setPrevMatchedPairs([])
      setRevealedPairs([])
      setCountdownVisibleCards([])
      setCascadeRevealDone(false)
      setCascadeHideDone(false)
      setCascadeHideStarted(false)
      setLocalCountdown(null)
    }
  }, [gameState.phase])

  // Local countdown timer - starts when cascade reveal is done
  // Uses MEMORIZE_TIME as the visual countdown (backend accounts for cascade durations)
  useEffect(() => {
    const memoryConfig = gameState.question?.MEMORY_CONFIG || {}
    const memorizeTime = memoryConfig.MEMORIZE_TIME || 5

    // Start local countdown when cascade reveal is done
    // The countdown is simply MEMORIZE_TIME (backend phase duration includes cascade times)
    if (cascadeRevealDone && localCountdown === null && gameState.phase === 'COUNTDOWN') {
      setLocalCountdown(memorizeTime)
    }

    // Reset when leaving COUNTDOWN
    if (gameState.phase !== 'COUNTDOWN') {
      setLocalCountdown(null)
    }
  }, [cascadeRevealDone, gameState.phase, gameState.question?.MEMORY_CONFIG, localCountdown])

  // Decrement local countdown every second
  useEffect(() => {
    if (localCountdown !== null && localCountdown > 0) {
      const timer = setTimeout(() => {
        setLocalCountdown(prev => prev - 1)
      }, 1000)
      return () => clearTimeout(timer)
    }
  }, [localCountdown])

  // Trigger cascade hide when localCountdown reaches 0
  useEffect(() => {
    if (localCountdown === 0 && !cascadeHideStarted && gameState.phase === 'COUNTDOWN') {
      const isMemoryQuestion = gameState.question?.TYPE === 'MEMORY'
      const memoryConfig = gameState.question?.MEMORY_CONFIG || {}
      const showDuringMemorize = memoryConfig.SHOW_DURING_MEMORIZE === undefined || memoryConfig.SHOW_DURING_MEMORIZE === true
      const pairs = gameState.question?.MEMORY_PAIRS || []
      const cardCount = pairs.length * 2
      const STAGGER_DELAY = 200

      if (isMemoryQuestion && showDuringMemorize && cardCount > 0) {
        setCascadeHideStarted(true)
        setCascadeHideDone(false)
        // Hide cards one by one starting from index 0 (remove from beginning)
        for (let i = 0; i < cardCount; i++) {
          setTimeout(() => {
            setCountdownVisibleCards(prev => {
              const newArr = [...prev]
              newArr.shift() // Remove first element
              return newArr
            })
            // Mark hide as done when last card starts hiding
            if (i === cardCount - 1) {
              setTimeout(() => {
                setCascadeHideDone(true)
              }, 600)
            }
          }, i * STAGGER_DELAY)
        }
      }
    }
  }, [localCountdown, cascadeHideStarted, gameState.phase, gameState.question])

  // Progressive reveal of pairs when entering REVEALED phase
  useEffect(() => {
    const currentPhase = gameState.phase
    const pairs = gameState.question?.MEMORY_PAIRS || []
    const matchedPairs = gameState.memoryMatchedPairs || []
    const memoryConfig = gameState.question?.MEMORY_CONFIG || {}
    // Default reveal delay is 0.5s, convert to ms
    const revealDelayMs = (memoryConfig.REVEAL_DELAY || 0.5) * 1000

    // Detect transition to REVEALED
    if (currentPhase === 'REVEALED' && prevPhase !== 'REVEALED' && pairs.length > 0) {
      // Reset revealed pairs
      setRevealedPairs([])

      // Filter out already matched pairs - they are already visible
      const pairsToReveal = pairs.filter(pair => !matchedPairs.includes(pair.ID))

      // Reveal unmatched pairs one by one with configurable delay
      pairsToReveal.forEach((pair, index) => {
        setTimeout(() => {
          setRevealedPairs(prev => [...prev, pair.ID])
        }, index * revealDelayMs)
      })
    }

    // Reset when leaving REVEALED
    if (currentPhase !== 'REVEALED' && prevPhase === 'REVEALED') {
      setRevealedPairs([])
    }

    setPrevPhase(currentPhase)
  }, [gameState.phase, gameState.question?.MEMORY_PAIRS, gameState.question?.MEMORY_CONFIG, gameState.memoryMatchedPairs, prevPhase])

  // Cascading flip animation during COUNTDOWN phase (Memory only)
  // Cards flip one after another with 200ms stagger (0.33 of 0.6s flip animation)
  useEffect(() => {
    const isMemoryQuestion = gameState.question?.TYPE === 'MEMORY'
    const memoryConfig = gameState.question?.MEMORY_CONFIG || {}
    const showDuringMemorize = memoryConfig.SHOW_DURING_MEMORIZE === undefined || memoryConfig.SHOW_DURING_MEMORIZE === true
    const pairs = gameState.question?.MEMORY_PAIRS || []
    const cardCount = pairs.length * 2
    const STAGGER_DELAY = 200 // 0.33 of flip animation (0.6s)

    // Entering COUNTDOWN - progressively reveal cards starting from card 1
    if (gameState.phase === 'COUNTDOWN' && prevPhase !== 'COUNTDOWN' && isMemoryQuestion && showDuringMemorize && cardCount > 0) {
      setCountdownVisibleCards([])
      setCascadeRevealDone(false)
      setCascadeHideDone(false)
      setCascadeHideStarted(false)
      // Reveal cards one by one starting from index 0
      for (let i = 0; i < cardCount; i++) {
        setTimeout(() => {
          setCountdownVisibleCards(prev => [...prev, i])
          // Mark reveal as done when last card starts flipping
          if (i === cardCount - 1) {
            // Wait for flip animation to complete (0.6s) before marking as done
            setTimeout(() => {
              setCascadeRevealDone(true)
            }, 600)
          }
        }, i * STAGGER_DELAY)
      }
    }

    // Note: Cascade hide is now triggered by localCountdown === 0 (see earlier useEffect)

    // Direct exit (not to STARTED) - clear immediately
    if (prevPhase === 'COUNTDOWN' && gameState.phase !== 'STARTED' && gameState.phase !== 'COUNTDOWN') {
      setCountdownVisibleCards([])
      setCascadeRevealDone(false)
      setCascadeHideDone(false)
      setCascadeHideStarted(false)
    }
  }, [gameState.phase, gameState.question?.TYPE, gameState.question?.MEMORY_CONFIG, gameState.question?.MEMORY_PAIRS, prevPhase])

  const triggerCelebration = (color) => {
    const rgb = color || [99, 102, 241]
    const hex = `#${rgb.map(c => c.toString(16).padStart(2, '0')).join('')}`

    confetti({
      particleCount: 100,
      spread: 70,
      origin: { y: 0.6 },
      colors: [hex, '#ffffff', '#ffd700'],
    })
  }

  const isShowingScores = gameState.remote === 'SCORE'
  const isShowingPlayers = gameState.remote === 'PLAYERS'
  const isShowingPalmares = gameState.remote === 'PALMARES'
  const isShowingGame = !isShowingScores && !isShowingPlayers && !isShowingPalmares
  const maxTeamScore = Math.max(...sortedTeams.map(t => t.score), 1)
  const maxPlayerScore = Math.max(...sortedPlayers.map(p => p.score), 1)

  // Display logic for game content
  const showPrepare = gameState.phase === 'PREPARE'
  const showReady = gameState.phase === 'READY'
  const showCountdown = gameState.phase === 'COUNTDOWN'
  const showGameContent = ['STARTED', 'PAUSED', 'STOPPED', 'REVEALED'].includes(gameState.phase)
  const showAnswer = gameState.phase === 'REVEALED'
  const isQcm = gameState.question?.TYPE === 'QCM'
  // isMemory: question.TYPE primary source; fallback on MEMORY_PARTICIPATING_TEAMS when question
  // is temporarily null (e.g. TV connects between PREPARE→READY transition before UPDATE arrives)
  const isMemory = gameState.question?.TYPE === 'MEMORY' ||
    (!gameState.question && (gameState.MEMORY_PARTICIPATING_TEAMS?.length ?? 0) > 0 && showMemoryGrid)
  const isMemotion = gameState.question?.TYPE === 'MEMOTION'
  const isArdoise = gameState.question?.TYPE === 'ARDOISE'
  // QCM answers visible from READY through REVEALED (no re-render on transition)
  const showQcmAnswers = ['READY', 'COUNTDOWN', 'STARTED', 'PAUSED', 'STOPPED', 'REVEALED'].includes(gameState.phase)
  // Memory grid visible from READY (cards face down during countdown) through REVEALED
  const showMemoryGrid = ['READY', 'COUNTDOWN', 'STARTED', 'PAUSED', 'STOPPED', 'REVEALED'].includes(gameState.phase)
  // QCM answer TEXT visible from COUNTDOWN (READY shows only colored zones with letters)
  const showQcmAnswerText = ['COUNTDOWN', 'STARTED', 'PAUSED', 'STOPPED', 'REVEALED'].includes(gameState.phase)

  // Calculate QCM hint markers for the timer bar
  const qcmHintMarkers = useMemo(() => {
    if (!isQcm || !gameState.question?.QCM_HINTS_ENABLED) return null

    const t1 = gameState.question.QCM_HINT_THRESHOLD_1 || 0.25
    const t2 = gameState.question.QCM_HINT_THRESHOLD_2 || 0.125
    const totalTime = gameState.totalTime || 0
    const currentTime = gameState.timer || 0
    const invalidated = gameState.qcmInvalidated || []

    if (totalTime <= 0) return null

    const markers = []

    // Threshold values are fractions (0.25 = 25% of time remaining)
    // The timer bar width = (currentTime/totalTime)*100%, shrinking from right
    // Marker position from left = threshold * 100%
    // When the bar edge reaches the marker, the hint triggers

    // Check safety constraints (same as backend)
    const t1Seconds = t1 * totalTime
    const t2Seconds = t2 * totalTime
    const canShowT1 = t1Seconds >= 2
    const canShowT2 = canShowT1 && t2Seconds >= 1 && (t1Seconds - t2Seconds) >= 1

    if (canShowT1) {
      const t1Triggered = invalidated.length >= 1
      const t1RemainingPercent = t1 * 100
      const currentRemainingPercent = totalTime > 0 ? (currentTime / totalTime) * 100 : 100
      // Pulsing when approaching the threshold (within 15% on the bar)
      const t1Pulsing = !t1Triggered && currentRemainingPercent <= (t1RemainingPercent + 15) && currentRemainingPercent > t1RemainingPercent

      markers.push({
        id: 1,
        position: `${t1RemainingPercent}%`,
        triggered: t1Triggered,
        pulsing: t1Pulsing,
      })
    }

    if (canShowT2) {
      const t2Triggered = invalidated.length >= 2
      const t2RemainingPercent = t2 * 100
      const currentRemainingPercent = totalTime > 0 ? (currentTime / totalTime) * 100 : 100
      const t2Pulsing = !t2Triggered && invalidated.length >= 1 && currentRemainingPercent <= (t2RemainingPercent + 15) && currentRemainingPercent > t2RemainingPercent

      markers.push({
        id: 2,
        position: `${t2RemainingPercent}%`,
        triggered: t2Triggered,
        pulsing: t2Pulsing,
      })
    }

    return markers.length > 0 ? markers : null
  }, [isQcm, gameState.question?.QCM_HINTS_ENABLED, gameState.question?.QCM_HINT_THRESHOLD_1, gameState.question?.QCM_HINT_THRESHOLD_2, gameState.totalTime, gameState.timer, gameState.qcmInvalidated])

  // Top 3 players for podium
  const topPlayers = useMemo(() => {
    return sortedPlayers.slice(0, 3).map(player => ({
      ...player,
      name: player.name,
      color: player.teamColor,
    }))
  }, [sortedPlayers])

  // Règle de disposition MEMORY — EXTRAITE dans utils/memoryGrid.js (#159/F0),
  // partagée avec la grille animateur (AnimMemoryGrid). Extraction pure :
  // même mélange ensemencé, même formule de colonnes/rangées, mêmes
  // dépendances de useMemo qu'avant l'extraction. Ne PAS réimplémenter
  // cette logique ailleurs — voir le commentaire de tête de memoryGrid.js
  // pour le motif (correspondance positionnelle entre /tv, la vue joueur et
  // /anim).
  const memoryCards = useMemo(
    () => buildMemoryCards(gameState.question),
    [gameState.question?.MEMORY_PAIRS, gameState.question?.ID]
  )

  const memoryGridCols = useMemo(
    () => getMemoryGridCols(memoryCards.length),
    [memoryCards.length]
  )

  const memoryGridRows = useMemo(
    () => getMemoryGridRows(memoryCards.length, memoryGridCols),
    [memoryCards.length, memoryGridCols]
  )

  // Calculate if we need 2 columns for players (if more than 6 players)
  const useTwoColumns = sortedPlayers.length > 6

  // Calculate teams that buzzed each QCM answer (for STOPPED/REVEALED phases)
  const teamsByQcmAnswer = useMemo(() => {
    const result = { RED: [], GREEN: [], YELLOW: [], BLUE: [] }

    // Only compute during STOPPED or REVEALED phases for QCM questions
    if (!['STOPPED', 'REVEALED'].includes(gameState.phase) || gameState.question?.TYPE !== 'QCM') {
      return result
    }

    // Group bumpers by team, keeping only those who buzzed
    // Use ANSWER_COLOR (the player's assigned QCM color) to determine their answer
    const teamBuzzes = {}
    Object.entries(bumpers).forEach(([mac, bumper]) => {
      if (bumper.TIME && bumper.TIME > 0 && bumper.ANSWER_COLOR && bumper.TEAM) {
        const qcmColor = bumper.ANSWER_COLOR // Already in RED/GREEN/YELLOW/BLUE format
        if (qcmColor && !teamBuzzes[bumper.TEAM]) {
          // Use first buzzer from each team
          teamBuzzes[bumper.TEAM] = {
            team: bumper.TEAM,
            color: teams[bumper.TEAM]?.COLOR,
            qcmAnswer: qcmColor,
            time: bumper.TIME,
            hintsAtBuzz: bumper.HINTS_AT_BUZZ || 0, // Store hints count at buzz time for penalty display
          }
        }
      }
    })

    // Distribute teams to their QCM answer, sorted by response time
    Object.values(teamBuzzes)
      .sort((a, b) => a.time - b.time) // Sort by time (fastest first)
      .forEach(buzz => {
        if (result[buzz.qcmAnswer]) {
          result[buzz.qcmAnswer].push({
            name: buzz.team,
            color: buzz.color,
            time: buzz.time,
            hintsAtBuzz: buzz.hintsAtBuzz,
          })
        }
      })

    return result
  }, [gameState.phase, gameState.question?.TYPE, bumpers, teams])

  // Current background - index is server-synchronized
  const backgrounds = gameState.backgrounds || []
  const bgIndex = gameState.currentBackgroundIndex ?? 0
  const currentBg = backgrounds.length > 0 ? backgrounds[bgIndex % backgrounds.length] : null
  const currentBackground = currentBg?.path || null
  const currentOpacity = (currentBg?.opacity ?? 100) / 100

  // Get team color by team name (for Memory matched pairs)
  const getTeamColorByName = (teamName) => {
    const team = Object.values(teams).find(t => t.NAME === teamName)
    if (team && team.COLOR) {
      return getRgbColor(team.COLOR)
    }
    return '#4CAF50' // Green by default (SOLO mode)
  }

  // Neon effect configuration
  const neonConfig = useMemo(() => {
    return gameState?.neonEffect || {
      enabled: false,
      mode: 'bar',
      arc_width: 60,
      intensity_gap: 80,
      rotation_speed: 4,
      bar_offset: 20,
      bar_thickness: 4,
      arc_blur: 100,
      glow_pulse_speed: 2,
      glow_pulse_min: 30,
      glow_pulse_max: 50
    }
  }, [gameState?.neonEffect])

  // Show neon effect during game phases
  const showNeon = useMemo(() => {
    return neonConfig.enabled &&
      ['PREPARE', 'READY', 'COUNTDOWN', 'STARTED', 'PAUSED'].includes(gameState?.phase)
  }, [neonConfig.enabled, gameState?.phase])

  // Get category color for neon effect
  // Priority: MEMOTION team color > Memory team color > Category color > default
  const neonCategoryColor = useMemo(() => {
    // MEMOTION: use current team color
    if (Array.isArray(gameState?.MEMOTION_CURRENT_TEAM_COLOR) && gameState.MEMOTION_CURRENT_TEAM_COLOR.length === 3) {
      return `rgb(${gameState.MEMOTION_CURRENT_TEAM_COLOR.join(',')})`
    }
    // Memory multi-teams mode: use team color
    if (Array.isArray(gameState?.MEMORY_CURRENT_TEAM_COLOR) && gameState.MEMORY_CURRENT_TEAM_COLOR.length === 3) {
      return `rgb(${gameState.MEMORY_CURRENT_TEAM_COLOR.join(',')})`
    }
    // Otherwise, use category color
    return getCategoryColor(gameState?.question?.CATEGORY)
  }, [gameState?.question?.CATEGORY, gameState?.MEMORY_CURRENT_TEAM_COLOR, gameState?.MEMOTION_CURRENT_TEAM_COLOR])

  // Neon style variables
  const neonStyle = useMemo(() => {
    if (!showNeon) return {}
    const barThickness = neonConfig.bar_thickness
    const arcBlurPercent = neonConfig.arc_blur
    // arc_blur is 0-200% of bar thickness
    const arcBlurPx = (barThickness * arcBlurPercent) / 100
    return {
      '--neon-color': neonCategoryColor,
      '--neon-arc-width': `${neonConfig.arc_width}deg`,
      '--neon-intensity-gap': neonConfig.intensity_gap / 100,
      '--neon-rotation-speed': `${neonConfig.rotation_speed}s`,
      '--neon-glow-pulse-speed': `${neonConfig.glow_pulse_speed}s`,
      '--neon-glow-pulse-min': neonConfig.glow_pulse_min / 100,
      '--neon-glow-pulse-max': neonConfig.glow_pulse_max / 100,
      '--neon-bar-offset': `${neonConfig.bar_offset}px`,
      '--neon-bar-thickness': `${barThickness}px`,
      '--neon-arc-blur': `${arcBlurPx}px`,
    }
  }, [showNeon, neonCategoryColor, neonConfig])

  // Neon mode class
  const neonModeClass = neonConfig.mode === 'halo' ? 'neon-mode-halo' : 'neon-mode-bar'

  // TV page: auto-fullscreen + wake lock on mount (skip for admin preview and vplayer)
  const toggleTvFullscreen = useCallback(() => {
    const isFull = !!(document.fullscreenElement || document.webkitFullscreenElement)
    if (!isFull) {
      const el = document.documentElement
      const fn = el.requestFullscreen || el.webkitRequestFullscreen
      if (fn) fn.call(el).catch(() => {})
    } else {
      const fn = document.exitFullscreen || document.webkitExitFullscreen
      if (fn) fn.call(document).catch(() => {})
    }
  }, [])

  useEffect(() => {
    if (isVPlayer || isAdminPreview) return

    // Auto-fullscreen on mount
    const el = document.documentElement
    const fn = el.requestFullscreen || el.webkitRequestFullscreen
    if (fn) fn.call(el).catch(() => {})

    // Track fullscreen state
    const onChange = () => setIsFullscreen(!!(document.fullscreenElement || document.webkitFullscreenElement))
    document.addEventListener('fullscreenchange', onChange)
    document.addEventListener('webkitfullscreenchange', onChange)

    // Wake lock: native (HTTPS) or NoSleep.js (HTTP, uses user gesture from click)
    const startWakeLock = async () => {
      if ('wakeLock' in navigator) {
        try { tvWakeLockRef.current = await navigator.wakeLock.request('screen'); return } catch (_) {}
      }
      if (!tvNoSleepRef.current) tvNoSleepRef.current = new NoSleep()
      if (!tvNoSleepRef.current.isEnabled) tvNoSleepRef.current.enable()
    }
    startWakeLock()

    return () => {
      document.removeEventListener('fullscreenchange', onChange)
      document.removeEventListener('webkitfullscreenchange', onChange)
      if (tvWakeLockRef.current) { tvWakeLockRef.current.release(); tvWakeLockRef.current = null }
      if (tvNoSleepRef.current?.isEnabled) { tvNoSleepRef.current.disable(); tvNoSleepRef.current = null }
    }
  }, [isVPlayer, isAdminPreview])

  // ENTRACTE (v6.5.2, #119) — conditionné par !isVPlayer : VPlayerPage monte
  // <PlayerDisplay isVPlayer> et gère son propre filtre/panneau (F5) ; sans
  // cette garde les deux filtres se composeraient et l'écran deviendrait
  // presque noir (piège documenté au plan, "double filtrage").
  const entracteActiveHere = !!gameState.entracte && !isVPlayer
  // Transition progressive (#119, C3) — --ep-transition posée ici, héritée
  // par .entracte-dim et le fondu du panneau (même durée pour les deux
  // mécanismes). Depuis entracteConfig (diffusé, gelé pendant une pause
  // active) — jamais entracteConfigSaved.
  const entracteTransitionStyle = { '--ep-transition': `${gameState.entracteConfig?.TRANSITION_MS ?? 2000}ms` }

  return (
    <div className={`player-display ${showNeon ? `neon-border ${neonModeClass}` : ''}`} style={neonStyle}>
      {/* TV fullscreen fallback button — only if auto-fullscreen failed and not admin preview.
          Kept OUTSIDE .entracte-content: `filter` on an ancestor breaks `position: fixed`
          descendants (they'd reposition relative to the filtered box and lose their z-index). */}
      {!isVPlayer && !isAdminPreview && !isFullscreen && (
        <button className="tv-fullscreen-btn" onClick={toggleTvFullscreen} title="Plein écran">⛶</button>
      )}

      {/* ENTRACTE — tout le contenu de jeu existant reste visible mais estompé
          derrière le panneau ; wrapper toujours monté (jamais démonté/remonté
          au bascule, pour ne pas perturber AnimatePresence/timers), la classe
          entracte-dim est seule conditionnelle. */}
      <div
        className={`entracte-content${entracteActiveHere ? ' entracte-dim' : ''}`}
        style={entracteTransitionStyle}
      >
      {/* Background Images with Crossfade */}
      <div className="background-container">
        <AnimatePresence mode="sync">
          {currentBackground && (
            <motion.div
              key={currentBackground}
              className="background-image"
              style={{ backgroundImage: `url(${currentBackground})` }}
              initial={{ opacity: 0 }}
              animate={{ opacity: currentOpacity }}
              exit={{ opacity: 0 }}
              transition={{ duration: 1 }}
            />
          )}
        </AnimatePresence>
        <div className="background-overlay" />
      </div>

      <AnimatePresence mode="wait">
        {gameState.phase === 'ENROLL' && !isVPlayer ? (
          /* Enrollment Phase - QR Code Display (only for TV, not for VPlayers) */
          <motion.div
            key="enroll"
            className="enroll-phase"
            initial={{ opacity: 0, scale: 0.9 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.9 }}
          >
            <div className="enroll-header">
              <h1>📱 INSCRIPTION DES JOUEURS</h1>
            </div>

            {/* Two QR codes side by side: WiFi (left, blue) + VJoueur URL (right, green) (#51) */}
            <div className="enroll-qr-row">
              <div className="enroll-qr-card">
                <div className="enroll-qr-title">1. Rejoindre le WiFi</div>
                <QRCodeDisplay
                  url={wifiConfig?.ssid
                    ? `WIFI:T:${wifiConfig.password ? 'WPA' : 'nopass'};S:${escapeWifiString(wifiConfig.ssid)};P:${escapeWifiString(wifiConfig.password || '')};;`
                    : 'https://buzzcontrol.local/no-wifi-configured'}
                  size={260}
                  fgColor="#1d4ed8"
                  logo="📶"
                  label={null}
                />
                <div className="enroll-qr-subtitle">
                  {wifiConfig?.ssid ? `Réseau : ${wifiConfig.ssid}` : 'Non configuré'}
                </div>
              </div>
              <div className="enroll-qr-card enroll-qr-card-main">
                <div className="enroll-qr-title">2. Rejoindre le jeu</div>
                <QRCodeDisplay
                  url={`http://${window.location.host}/`}
                  size={260}
                  fgColor="#15803d"
                  logo="👤"
                />
                <div className="enroll-qr-subtitle">{window.location.host}/</div>
              </div>
            </div>

            <div className="enroll-progress">
              <div className="enroll-progress-bar">
                <div
                  className="enroll-progress-fill"
                  style={{ width: `${Math.min(((gameState.virtualPlayerCount || 0) / (gameState.virtualPlayerLimit || 20)) * 100, 100)}%` }}
                />
              </div>
              <span className="enroll-progress-text">
                {gameState.virtualPlayerCount || 0} / {gameState.virtualPlayerLimit || 20} joueurs
              </span>
            </div>
          </motion.div>
        ) : gameState.phase === 'NEW_GAME' ? (
          /* New Game Phase — full-screen announcement */
          <motion.div
            key="new-game"
            className="new-game-phase"
            initial={{ opacity: 0, scale: 0.8 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.8 }}
            transition={{ duration: 0.6, ease: 'easeOut' }}
          >
            {/* Background image overlay (shown over gradient when images available) */}
            {newGameBgs.length > 0 && (
              <div
                className="new-game-bg-overlay"
                style={{
                  backgroundImage: `url('${newGameBgs[ngBgIndex]?.path}')`,
                  backgroundSize: 'cover',
                  backgroundPosition: 'center',
                  opacity: (newGameBgs[ngBgIndex]?.opacity ?? 100) / 100,
                }}
              />
            )}
            {/* Étoiles décoratives */}
            <div className="new-game-stars" aria-hidden="true">
              {[...Array(12)].map((_, i) => (
                <div key={i} className={`new-game-star star-${i + 1}`} />
              ))}
            </div>

            <motion.h1
              className="new-game-title"
              animate={{ textShadow: [
                '0 0 30px rgba(255,255,255,0.9), 0 0 80px #a855f7, 0 0 120px #a855f7, 0 2px 4px rgba(0,0,0,0.9)',
                '0 0 30px rgba(255,255,255,0.9), 0 0 80px #06b6d4, 0 0 120px #06b6d4, 0 2px 4px rgba(0,0,0,0.9)',
                '0 0 30px rgba(255,255,255,0.9), 0 0 80px #a855f7, 0 0 120px #a855f7, 0 2px 4px rgba(0,0,0,0.9)',
              ] }}
              transition={{ duration: 3, repeat: Infinity, ease: 'easeInOut' }}
            >
              NOUVELLE PARTIE À VENIR
            </motion.h1>

            {/* Sous-titre accroche */}
            <motion.p
              className="new-game-subtitle"
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: 0.2 }}
            >
              Préparez-vous !
            </motion.p>

            {(gameState.quizName || (!themeHiddenFromTV && gameState.quizTheme)) && (
              <motion.div
                className="new-game-meta"
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: 0.35 }}
              >
                {gameState.quizName && (
                  <div className="new-game-quiz-name">{gameState.quizName}</div>
                )}
                {/* QUIZ_NAME n'est pas maskable (H4, contract game-state.md) —
                    seul le thème l'est via QUIZ_HIDDEN_FIELDS. */}
                {!themeHiddenFromTV && gameState.quizTheme && (
                  <div className="new-game-quiz-theme">{gameState.quizTheme}</div>
                )}
              </motion.div>
            )}
            {gameState.quizNotes && (
              <motion.div
                className="new-game-notes"
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: 0.5 }}
              >
                {gameState.quizNotes}
              </motion.div>
            )}
            {/* v6.1.0 (#137, T2.4) — Publics/Difficultés/Langue : une ligne
                compacte de badges, plafonnée à 2 par famille ("+N" pour le
                surplus) et respectant QUIZ_HIDDEN_FIELDS (masquer d'abord,
                plafonner ensuite ce qui reste — quizBadgeFamilies ci-dessus).
                Contrainte TV STATIQUE (CLAUDE.md) : overflow hidden hérité de
                .new-game-phase, jamais de nouvelle ligne par champ. Les
                quatre champs masqués -> quizBadgeFamilies est vide -> la
                rangée entière disparaît, sans bloc vide résiduel. */}
            {quizBadgeFamilies.length > 0 && (
              <motion.div
                className="new-game-badges"
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: 0.6 }}
              >
                {quizBadgeFamilies.map((family, index) => (
                  <Fragment key={family.key}>
                    {index > 0 && <span className="new-game-badge-sep" aria-hidden="true" />}
                    {family.values.slice(0, QUIZ_BADGE_CAP_PER_FAMILY).map(value => (
                      <span key={value} className="new-game-badge">{value}</span>
                    ))}
                    {family.values.length > QUIZ_BADGE_CAP_PER_FAMILY && (
                      <span className="new-game-badge new-game-badge-more">
                        +{family.values.length - QUIZ_BADGE_CAP_PER_FAMILY}
                      </span>
                    )}
                  </Fragment>
                ))}
              </motion.div>
            )}
          </motion.div>
        ) : isShowingScores ? (
          /* Team Scores View */
          <motion.div
            key="scores"
            className="scores-display"
            initial={{ opacity: 0, scale: 0.9 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.9 }}
          >
            <h1 className="scores-title">Classement Equipes</h1>

            <div className="scores-layout">
              {sortedTeams.length >= 1 && (
                <div className="scores-podium-section">
                  <Podium teams={sortedTeams} changedTeams={changedTeams} />
                </div>
              )}

              {sortedTeams.length > 0 && (
                <div className="scores-list-section">
                  <div className="scores-list compact">
                    <AnimatePresence mode="popLayout">
                      {sortedTeams.map((team) => {
                        const rgbColor = getRgbColor(team.color)
                        const isChanged = changedTeams[team.name]
                        const isTied = sortedTeams.filter(t => t.rank === team.rank).length > 1
                        const barWidth = (team.score / maxTeamScore) * 100

                        return (
                          <motion.div
                            key={team.name}
                            className={`score-row ${isChanged ? 'score-changed' : ''} ${isChanged === 'up' ? 'rank-up' : ''}`}
                            initial={{ opacity: 0, x: 50 }}
                            animate={{ opacity: 1, x: 0 }}
                            exit={{ opacity: 0, x: -50 }}
                            transition={{ type: 'spring', stiffness: 300, damping: 30 }}
                            style={{ '--team-color': rgbColor }}
                            layout
                          >
                            <div className="score-rank-medal">
                              {team.rank === 1 ? '🥇' : team.rank === 2 ? '🥈' : team.rank === 3 ? '🥉' : `#${team.rank}`}
                              {isTied && <span className="tied-indicator">ex</span>}
                            </div>
                            <div className="score-team-info">
                              <div className="score-team-badge-name" style={{ backgroundColor: rgbColor }}>
                                {team.name}
                              </div>
                              <div className="score-team-bar">
                                <motion.div
                                  className="score-team-bar-fill"
                                  initial={{ width: 0 }}
                                  animate={{ width: `${barWidth}%` }}
                                  transition={{ duration: 0.5 }}
                                />
                              </div>
                            </div>
                            <motion.span
                              className="score-points"
                              key={team.score}
                              animate={isChanged ? { scale: [1, 1.3, 1], color: ['#fff', '#22c55e', '#fff'] } : {}}
                              transition={{ duration: 0.5 }}
                            >
                              {team.score}
                            </motion.span>
                          </motion.div>
                        )
                      })}
                    </AnimatePresence>
                  </div>
                </div>
              )}
            </div>
          </motion.div>
        ) : isShowingPlayers ? (
          /* Players Ranking View */
          <motion.div
            key="players"
            className="players-display"
            initial={{ opacity: 0, scale: 0.9 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.9 }}
          >
            <h1 className="scores-title">Classement Joueurs</h1>

            <div className="players-layout">
              {topPlayers.length >= 1 && (
                <div className="players-podium-section">
                  <Podium teams={topPlayers} changedTeams={{}} />
                </div>
              )}

              <div className="players-list-section">
                <div className={`players-list ${useTwoColumns ? 'two-columns' : ''}`}>
                  <AnimatePresence mode="popLayout">
                    {sortedPlayers.map((player, index) => {
                      const rgbColor = getRgbColor(player.teamColor)
                      const barWidth = (player.score / maxPlayerScore) * 100
                      const isTied = sortedPlayers.filter(p => p.rank === player.rank).length > 1
                      const isPlayerChanged = changedPlayers[player.mac]

                      return (
                        <motion.div
                          key={player.mac}
                          className={`player-row ${isPlayerChanged ? 'score-changed' : ''}`}
                          initial={{ opacity: 0, y: 20 }}
                          animate={{ opacity: 1, y: 0 }}
                          exit={{ opacity: 0, y: -20 }}
                          transition={{ delay: index * 0.03, type: 'spring', stiffness: 300, damping: 30 }}
                          style={{ '--team-color': rgbColor }}
                          layout
                        >
                          <div className="player-rank">
                            {player.rank <= 3 ? (
                              <span className="player-medal">
                                {player.rank === 1 ? '🥇' : player.rank === 2 ? '🥈' : '🥉'}
                              </span>
                            ) : (
                              <span className="player-rank-number">#{player.rank}</span>
                            )}
                          </div>
                          <div className="player-avatar" style={{ backgroundColor: rgbColor }}>
                            {player.name.charAt(0).toUpperCase()}
                          </div>
                          <div className="player-info">
                            <span className="player-name">{player.name}</span>
                            <span className="player-team">{player.team || 'Sans equipe'}</span>
                            <div className="player-bar-outer">
                              <motion.div
                                className="player-bar-inner"
                                initial={{ width: 0 }}
                                animate={{ width: `${barWidth}%` }}
                                transition={{ delay: 0.2, duration: 0.5 }}
                              />
                            </div>
                          </div>
                          <div className="player-score-section">
                            {isTied && <span className="player-tied">ex</span>}
                            <motion.span
                              className="player-score"
                              key={player.score}
                              animate={isPlayerChanged ? {
                                scale: [1, 1.4, 1],
                                color: ['#fff', '#22c55e', '#fff']
                              } : {}}
                              transition={{ duration: 0.5 }}
                            >
                              {player.score} pts
                            </motion.span>
                          </div>
                        </motion.div>
                      )
                    })}
                  </AnimatePresence>
                </div>
              </div>
            </div>
          </motion.div>
        ) : isShowingPalmares ? (
          /* Palmares by Category View */
          <motion.div
            key="palmares"
            className="palmares-display"
            initial={{ opacity: 0, scale: 0.9 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.9 }}
          >
            <h1 className="scores-title">Palmares par Categorie</h1>

            <div className="palmares-categories">
              {palmares.length === 0 ? (
                <div className="palmares-empty">
                  <p>Aucun evenement enregistre</p>
                </div>
              ) : (
                // v5.7.10: PalmaresEntry pré-assemblé par le backend — accès direct, zéro lookup
                palmares.slice(0, 6).map((entry, index) => {
                  const catLabel    = entry.name
                    || (entry.category === 'UNKNOWN'
                      ? 'Inconnue'
                      : entry.category.replace(/_/g, ' ').toLowerCase().replace(/\b\w/g, c => c.toUpperCase()))
                  const catImageURL = entry.imageURL || null
                  const catColor    = entry.color || '#6b7280'
                  const hasTeams    = entry.teams.length > 0
                  const hasPlayers  = entry.players.length > 0

                  // Rank calculation with tie handling (items sorted desc by backend)
                  const addRanks = (items) => items.slice(0, 3).reduce((acc, item, idx) => {
                    const rank = idx === 0 ? 1 : (acc[idx - 1].points === item.points ? acc[idx - 1].rank : idx + 1)
                    acc.push({ ...item, rank })
                    return acc
                  }, [])
                  const rankedTeams   = addRanks(entry.teams)
                  const rankedPlayers = addRanks(entry.players)

                  // Team color lookup for player rows
                  const teamColorMap = Object.fromEntries(entry.teams.map(t => [t.name, t.color]))

                  const totalTeamPoints   = entry.teams.reduce((s, t) => s + t.points, 0)
                  const totalPlayerPoints = entry.players.reduce((s, p) => s + p.points, 0)

                  return (
                    <motion.div
                      key={entry.category}
                      className="palmares-category"
                      initial={{ opacity: 0, y: 20 }}
                      animate={{ opacity: 1, y: 0 }}
                      transition={{ delay: index * 0.1 }}
                    >
                      <div className="palmares-category-header" style={{ backgroundColor: catColor }}>
                        {catImageURL
                          ? <img
                              src={catImageURL}
                              alt={catLabel}
                              style={{ width: '2rem', height: '2rem', objectFit: 'cover', borderRadius: '0.25rem', flexShrink: 0 }}
                            />
                          : <span className="palmares-category-icon" style={{ color: catColor }}>
                              {CATEGORIES[entry.category]?.icon ?? '📷'}
                            </span>
                        }
                        <span className="palmares-category-name">{catLabel}</span>
                        <div className="palmares-category-stats">
                          {hasTeams && <span className="palmares-stat">👥 {totalTeamPoints} pts</span>}
                          {hasPlayers && <span className="palmares-stat">👤 {totalPlayerPoints} pts</span>}
                        </div>
                      </div>

                      <div className="palmares-category-content">
                        {hasTeams && (
                          <div className="palmares-ranking">
                            <h3 className="palmares-ranking-title">Equipes</h3>
                            <div className="palmares-ranking-list">
                              {rankedTeams.map((team) => (
                                <div
                                  key={team.name}
                                  className={`palmares-ranking-item rank-${team.rank}`}
                                  style={{ '--team-color': team.color ? `rgb(${team.color.join(',')})` : '#6b7280' }}
                                >
                                  <span className="palmares-rank-medal">
                                    {team.rank === 1 ? '🥇' : team.rank === 2 ? '🥈' : '🥉'}
                                  </span>
                                  <span className="palmares-rank-name">{team.name}</span>
                                  <span className="palmares-rank-points">{team.points} pts</span>
                                </div>
                              ))}
                            </div>
                          </div>
                        )}

                        {hasPlayers && (
                          <div className="palmares-ranking">
                            <h3 className="palmares-ranking-title">Joueurs</h3>
                            <div className="palmares-ranking-list">
                              {rankedPlayers.map((player) => (
                                <div
                                  key={`${player.team}|${player.name}`}
                                  className={`palmares-ranking-item rank-${player.rank}`}
                                  style={{ '--team-color': teamColorMap[player.team] ? `rgb(${teamColorMap[player.team].join(',')})` : '#6b7280' }}
                                >
                                  <span className="palmares-rank-medal">
                                    {player.rank === 1 ? '🥇' : player.rank === 2 ? '🥈' : '🥉'}
                                  </span>
                                  <span className="palmares-rank-name">{player.name}</span>
                                  <span className="palmares-rank-points">{player.points} pts</span>
                                </div>
                              ))}
                            </div>
                          </div>
                        )}
                      </div>
                    </motion.div>
                  )
                })
              )}
            </div>
          </motion.div>
        ) : (
          /* Game View - 4 vertical zones */
          <motion.div
            key="game"
            className="game-display"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
          >
            {/* VPlayer permanent info - always shown regardless of phase */}
            {playerName && isVPlayer && (
              <div className="vplayer-permanent-info">
                <div className="player-name-badge-mobile" style={{ backgroundColor: playerNameColor }}>
                  {playerName}
                </div>
                {teamName && (
                  <div className="player-team-badge-mobile" style={{ backgroundColor: teamColor }}>
                    {teamName}
                  </div>
                )}
              </div>
            )}

            {/* PREPARE State - Fixed "Preparez-vous" centered, no category (all question types) */}
            {showPrepare && (
              <motion.div
                className="prepare-state"
                initial={{ opacity: 0, scale: 0.8 }}
                animate={{ opacity: 1, scale: 1 }}
              >
                <motion.span
                  className="prepare-emoji"
                  animate={{ rotate: [0, 10, -10, 0] }}
                  transition={{ duration: 1, repeat: Infinity }}
                >
                  🔔
                </motion.span>
                <span className="prepare-text">NOUVELLE QUESTION</span>
              </motion.div>
            )}

            {/* COUNTDOWN State - Timer + Category animates to question zone + Big countdown number */}
            {showCountdown && !isQcm && !isMemory && !isMemotion && (
              <div className="game-content-zones">
                {/* Zone 1: Timer */}
                <div className="zone-timer">
                  <Timer
                    currentTime={gameState.timer}
                    totalTime={gameState.totalTime}
                    phase={gameState.phase}
                    size="xl"
                    showPhase={false}
                  />
                </div>

                {/* Zone 2: Category badge animates from center to question zone */}
                <div className="zone-question">
                  {(() => {
                    const catMeta = categoryMeta(gameState.question?.CATEGORY, apiCategories)
                    if (!catMeta) return null
                    return (
                      <motion.div
                        className="category-badge-inline"
                        style={{ backgroundColor: catMeta.color }}
                        initial={{ opacity: 0, y: 150, scale: 1.5 }}
                        animate={{ opacity: 1, y: 0, scale: 1 }}
                        transition={{ duration: 0.5, ease: 'easeOut' }}
                      >
                        {catMeta.imageURL
                          ? <img src={catMeta.imageURL} alt={catMeta.label} className="ready-category-img" />
                          : <span className="category-badge-icon">{catMeta.icon}</span>
                        }
                        <span className="category-badge-label">{catMeta.label}</span>
                      </motion.div>
                    )
                  })()}
                </div>

                {/* Zone 3: Big countdown number */}
                <div className="zone-media">
                  <motion.div
                    className="countdown-state"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                  >
                    <AnimatePresence mode="wait">
                      <motion.div
                        key={gameState.countdownTime}
                        className="countdown-number"
                        initial={{ scale: 0.5, opacity: 0 }}
                        animate={{ scale: 1.2, opacity: 1 }}
                        exit={{ scale: 1.8, opacity: 0 }}
                        transition={{ duration: 0.35, ease: 'easeOut' }}
                      >
                        {gameState.countdownTime > 0 ? gameState.countdownTime : 'GO!'}
                      </motion.div>
                    </AnimatePresence>
                  </motion.div>
                </div>

                {/* Zone 4: Empty answers placeholder */}
                <div className="zone-answers" />
              </div>
            )}

            {/* READY State - Non-QCM, Non-Memory: Timer + centered message (same layout as QCM) */}
            {showReady && !isQcm && !isMemory && !isMemotion && (
              <div className="game-content-zones">
                {/* Zone 1: Timer */}
                <div className="zone-timer">
                  <Timer
                    currentTime={gameState.timer}
                    totalTime={gameState.totalTime}
                    phase={gameState.phase}
                    size="xl"
                    showPhase={false}
                  />
                </div>

                {/* Zone 2: Empty question placeholder */}
                <div className="zone-question">
                  <div className="zone-question-placeholder" />
                </div>

                {/* Zone 3: Category icon + name with colored background */}
                <div className="zone-media">
                  <motion.div
                    className="ready-state"
                    initial={{ opacity: 0, scale: 0.8 }}
                    animate={{ opacity: 1, scale: 1 }}
                  >
                    <ReadyCategoryDisplay catKey={gameState.question?.CATEGORY} customCategories={apiCategories} gameType={gameState.question?.TYPE} />
                  </motion.div>
                </div>

                {/* Zone 4: Empty answers placeholder */}
                <div className="zone-answers" />
              </div>
            )}

            {/* QCM Game Content - unified block for READY through REVEALED (no flash on transition) */}
            {isQcm && showQcmAnswers && gameState.question && (
              <div className="game-content-zones">
                {/* Zone 1: Timer with hint markers */}
                <div className="zone-timer">
                  <Timer
                    currentTime={gameState.timer}
                    totalTime={gameState.totalTime}
                    phase={gameState.phase}
                    size="xl"
                    showPhase={false}
                    hintMarkers={qcmHintMarkers}
                  />
                </div>

                {/* Zone 2: Question (visible from STARTED) or Category badge (during COUNTDOWN) */}
                <div className="zone-question">
                  {showGameContent ? (
                    <motion.p
                      className="question-text"
                      initial={{ opacity: 0, y: 20 }}
                      animate={{ opacity: 1, y: 0 }}
                      transition={{ delay: 0.1 }}
                    >
                      {gameState.question.QUESTION}
                    </motion.p>
                  ) : showCountdown ? (() => {
                    const catMeta = categoryMeta(gameState.question?.CATEGORY, apiCategories)
                    if (!catMeta) return <div className="zone-question-placeholder" />
                    return (
                      <motion.div
                        className="category-badge-inline"
                        style={{ backgroundColor: catMeta.color }}
                        initial={{ opacity: 0, y: 150, scale: 1.5 }}
                        animate={{ opacity: 1, y: 0, scale: 1 }}
                        transition={{ duration: 0.5, ease: 'easeOut' }}
                      >
                        {catMeta.imageURL
                          ? <img src={catMeta.imageURL} alt={catMeta.label} className="ready-category-img" />
                          : <span className="category-badge-icon">{catMeta.icon}</span>
                        }
                        <span className="category-badge-label">{catMeta.label}</span>
                      </motion.div>
                    )
                  })() : (
                    <div className="zone-question-placeholder" />
                  )}
                </div>

                {/* Zone 3: Media or "PREPAREZ-VOUS" message or COUNTDOWN */}
                <div
                  className="zone-media"
                  onClick={onMediaClick && isVPlayer && (gameState.phase === 'STARTED' || gameState.phase === 'PAUSED') ? onMediaClick : undefined}
                  style={{ cursor: onMediaClick && isVPlayer && (gameState.phase === 'STARTED' || gameState.phase === 'PAUSED') ? 'pointer' : 'default' }}
                >
                  {showCountdown ? (
                    <motion.div
                      className="countdown-state"
                      initial={{ opacity: 0 }}
                      animate={{ opacity: 1 }}
                    >
                      <AnimatePresence mode="wait">
                        <motion.div
                          key={gameState.countdownTime}
                          className="countdown-number"
                          initial={{ scale: 0.5, opacity: 0 }}
                          animate={{ scale: 1.2, opacity: 1 }}
                          exit={{ scale: 1.8, opacity: 0 }}
                          transition={{ duration: 0.35, ease: 'easeOut' }}
                        >
                          {gameState.countdownTime > 0 ? gameState.countdownTime : 'GO!'}
                        </motion.div>
                      </AnimatePresence>
                    </motion.div>
                  ) : showReady ? (
                    <motion.div
                      className="ready-state"
                      initial={{ opacity: 0, scale: 0.8 }}
                      animate={{ opacity: 1, scale: 1 }}
                      exit={{ opacity: 0, scale: 0.8 }}
                    >
                      <ReadyCategoryDisplay catKey={gameState.question?.CATEGORY} customCategories={apiCategories} gameType="QCM" />
                    </motion.div>
                  ) : (showAnswer && gameState.question.MEDIA_ANSWER) ? (
                    <motion.img
                      key="answer-media"
                      src={gameState.question.MEDIA_ANSWER}
                      alt=""
                      className="question-media answer-media-highlight"
                      initial={{ opacity: 0, scale: 0.9 }}
                      animate={{ opacity: 1, scale: 1 }}
                      transition={{ delay: 0.2 }}
                    />
                  ) : (gameState.question.MEDIA || defaultQuestionImage) ? (
                    <motion.img
                      key="question-media"
                      src={gameState.question.MEDIA || defaultQuestionImage}
                      alt=""
                      className={`question-media${!gameState.question.MEDIA && defaultQuestionImage ? ' default-question-image' : ''}`}
                      initial={{ opacity: 0 }}
                      animate={{ opacity: 1 }}
                      transition={{ delay: 0.2 }}
                    />
                  ) : null}
                </div>

                {/* Zone 4: QCM Answers - persistent across READY→STARTED transition */}
                <div className="zone-answers">
                  <div className="qcm-answers-grid">
                    {Object.entries(QCM_COLORS).map(([colorKey, colorData]) => {
                      const answer = gameState.question.QCM_ANSWERS?.[colorKey]
                      if (!answer) return null
                      const isCorrect = gameState.question.QCM_CORRECT === colorKey
                      const isInvalidated = gameState.qcmInvalidated?.includes(colorKey)
                      const teamsOnThisAnswer = teamsByQcmAnswer[colorKey] || []
                      const showTeamBadges = ['STOPPED', 'REVEALED'].includes(gameState.phase) && teamsOnThisAnswer.length > 0

                      const isClickable = onQCMAnswer && gameState.phase === 'STARTED' && !isInvalidated && !vplayerHasBuzzed

                      return (
                        <motion.div
                          key={colorKey}
                          className={`qcm-answer-item ${showAnswer ? (isCorrect ? 'correct' : 'wrong') : ''} ${isInvalidated ? 'invalidated' : ''} ${isClickable ? 'clickable' : ''}`}
                          style={{
                            backgroundColor: isInvalidated ? '#374151' : (showAnswer && !isCorrect ? '#4b5563' : colorData.color),
                            opacity: isInvalidated ? 0.35 : (showAnswer && !isCorrect ? 0.4 : 1),
                            cursor: isClickable ? 'pointer' : undefined
                          }}
                          onClick={isClickable ? () => onQCMAnswer(colorKey) : undefined}
                          animate={showAnswer && isCorrect ? {
                            scale: [1, 1.08, 1],
                            boxShadow: [
                              `0 0 0px ${colorData.color}`,
                              `0 0 30px ${colorData.color}`,
                              `0 0 0px ${colorData.color}`
                            ]
                          } : {}}
                          transition={{ duration: 0.5, repeat: showAnswer && isCorrect ? 3 : 0 }}
                        >
                          <span className="qcm-answer-letter">{colorData.letter}</span>
                          <motion.span
                            className="qcm-answer-text"
                            initial={showQcmAnswerText ? { opacity: 0, y: 10 } : false}
                            animate={{ opacity: showQcmAnswerText ? 1 : 0, y: 0 }}
                            transition={{ duration: 0.3 }}
                          >
                            {answer}
                          </motion.span>
                          {/* Team badges - show which teams buzzed this answer */}
                          {showTeamBadges && (
                            <div className="qcm-team-badges">
                              {teamsOnThisAnswer.map((team, idx) => {
                                const totalBadges = teamsOnThisAnswer.length
                                // Size gradient: 70% (first) to 40% (last) of base size (60px)
                                const maxSize = 60
                                const minSize = 20
                                const sizeRatio = totalBadges > 1
                                  ? 0.70 - (idx / (totalBadges - 1)) * 0.30
                                  : 0.70
                                const badgeSize = Math.round(maxSize * sizeRatio)
                                const finalSize = Math.max(badgeSize, minSize)
                                const ringSize = finalSize + 8

                                // Calculate penalty percentage based on hintsAtBuzz
                                const hintsAtBuzz = team.hintsAtBuzz || 0
                                const penalty1 = gameState.question?.QCM_PENALTY_1 || 0.67
                                const penalty2 = gameState.question?.QCM_PENALTY_2 || 0.33
                                const penaltyPercent = hintsAtBuzz === 0 ? 100
                                  : hintsAtBuzz === 1 ? Math.round(penalty1 * 100)
                                  : Math.round(penalty2 * 100)

                                return (
                                  <motion.div
                                    key={team.name}
                                    className="qcm-team-badge-wrapper"
                                    style={{ width: `${ringSize}px`, height: `${ringSize}px` }}
                                    initial={{ scale: 0, opacity: 0 }}
                                    animate={{ scale: 1, opacity: 1 }}
                                    transition={{ delay: idx * 0.1 }}
                                    title={team.name}
                                  >
                                    {/* Penalty ring around badge - always shown */}
                                    <svg className="qcm-penalty-ring" viewBox="0 0 36 36">
                                      <circle
                                        className={penaltyPercent < 100 ? 'qcm-penalty-ring-fill' : 'qcm-penalty-ring-full'}
                                        cx="18" cy="18" r="16"
                                        fill="none"
                                        strokeWidth="3"
                                        strokeDasharray={`${penaltyPercent} ${100 - penaltyPercent}`}
                                        strokeDashoffset="25"
                                        style={{ transform: 'rotate(-90deg)', transformOrigin: 'center' }}
                                      />
                                    </svg>
                                    {/* Team color badge */}
                                    <div
                                      className="qcm-team-badge"
                                      style={{
                                        backgroundColor: team.color
                                          ? (Array.isArray(team.color) ? `rgb(${team.color.join(',')})` : team.color)
                                          : 'var(--gray-400)',
                                        width: `${finalSize}px`,
                                        height: `${finalSize}px`,
                                      }}
                                    />
                                  </motion.div>
                                )
                              })}
                            </div>
                          )}
                        </motion.div>
                      )
                    })}
                  </div>
                </div>

              </div>
            )}

            {/* MEMORY Game Content - unified block for READY through REVEALED */}
            {isMemory && showMemoryGrid && (() => {
              // DEBUG logs
              console.log('[PlayerDisplay] DEBUG:', {
                QUESTION: gameState.question?.QUESTION,
                CATEGORY: gameState.question?.CATEGORY,
                MEMORY_CURRENT_TEAM: gameState.MEMORY_CURRENT_TEAM,
                MEMORY_PAIR_OWNERS: gameState.MEMORY_PAIR_OWNERS,
                MEMORY_TEAM_PAIRS: gameState.MEMORY_TEAM_PAIRS,
                MEMORY_PARTICIPATING_TEAMS: gameState.MEMORY_PARTICIPATING_TEAMS,
                phase: gameState.phase
              })
              return (
              <div className="game-content-zones memory-game">
                {/* Zone 1: Timer - only show during STARTED when cascade hide is done */}
                <div className="zone-timer">
                  {/* During STARTED: only show timer after cascade hide is complete */}
                  {gameState.phase === 'STARTED' ? (
                    cascadeHideDone ? (
                      <Timer
                        currentTime={gameState.timer}
                        totalTime={gameState.totalTime}
                        phase={gameState.phase}
                        size="xl"
                        showPhase={false}
                      />
                    ) : (
                      <motion.div
                        className="memory-hiding-message"
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                      >
                        <span className="hiding-text">C'EST PARTI !</span>
                      </motion.div>
                    )
                  ) : (
                    <Timer
                      currentTime={gameState.timer}
                      totalTime={gameState.totalTime}
                      phase={gameState.phase}
                      size="xl"
                      showPhase={false}
                    />
                  )}
                </div>

                {/* Zone 2: Question, MEMORISEZ during COUNTDOWN, or PREPAREZ-VOUS during READY */}
                <div className="zone-question">
                  {showGameContent ? (
                    /* During STARTED: only show question after cascade hide is done */
                    (gameState.phase === 'STARTED' && !cascadeHideDone) ? (
                      <div className="zone-question-placeholder" />
                    ) : (
                      <motion.p
                        className="question-text"
                        initial={{ opacity: 0, y: 20 }}
                        animate={{ opacity: 1, y: 0 }}
                        transition={{ delay: 0.1 }}
                      >
                        {gameState.question.QUESTION}
                      </motion.p>
                    )
                  ) : showCountdown ? (
                    <motion.div
                      className="memory-memorize-message"
                      initial={{ opacity: 0, scale: 0.9 }}
                      animate={{ opacity: 1, scale: 1 }}
                    >
                      {/* Only show countdown number after cascade reveal is done, use local countdown */}
                      {cascadeRevealDone && localCountdown !== null ? (
                        <>
                          <span className="memorize-countdown">{localCountdown > 0 ? localCountdown : 'GO!'}</span>
                          <span className="memorize-text">MÉMORISEZ !</span>
                        </>
                      ) : (
                        <span className="memorize-text">MÉMORISEZ !</span>
                      )}
                    </motion.div>
                  ) : (
                    <motion.div
                      className="memory-prepare-message"
                      initial={{ opacity: 0, scale: 0.9 }}
                      animate={{ opacity: 1, scale: 1 }}
                    >
                      <ReadyCategoryDisplay catKey={gameState.question?.CATEGORY} customCategories={apiCategories} variant="memory" />
                    </motion.div>
                  )}
                </div>

                {/* Zone 3: Memory Grid - cards always visible, no overlay */}
                <div className="zone-media zone-memory-grid">
                  <div
                    className="memory-grid"
                    style={{ '--memory-cols': memoryGridCols, '--memory-rows': memoryGridRows }}
                  >
                    {memoryCards.map((cardData, index) => {
                      const cardLetter = String.fromCharCode(65 + index)
                      const memoryConfig = gameState.question?.MEMORY_CONFIG || {}
                      // Default to true if SHOW_DURING_MEMORIZE is not set (undefined) or explicitly true
                      const showDuringMemorize = memoryConfig.SHOW_DURING_MEMORIZE === undefined || memoryConfig.SHOW_DURING_MEMORIZE === true

                      // Phases before gameplay starts - hide all previous game state
                      const isPreGamePhase = ['PREPARE', 'READY', 'COUNTDOWN'].includes(gameState.phase)
                      // Phases during/after gameplay - show matched pairs
                      const isGameplayPhase = ['STARTED', 'PAUSED', 'STOPPED', 'REVEALED'].includes(gameState.phase)

                      // Cascading memorization: card visible during COUNTDOWN if its index is in countdownVisibleCards
                      const isInCountdownCascade = gameState.phase === 'COUNTDOWN' && showDuringMemorize && countdownVisibleCards.includes(index)
                      // Cards still hiding in cascade after COUNTDOWN ends (going to STARTED)
                      const isStillVisibleInCascade = gameState.phase === 'STARTED' && countdownVisibleCards.includes(index)
                      // REVEALED: progressively reveal pairs (using revealedPairs state)
                      const isProgressivelyRevealed = gameState.phase === 'REVEALED' && revealedPairs.includes(cardData.pairId)
                      const isMatched = gameState.memoryMatchedPairs?.includes(cardData.pairId)
                      const isPlayerFlipped = gameState.memoryFlippedCards?.includes(cardData.id)

                      // Cards are flipped (face up) during: cascading COUNTDOWN reveal, still hiding after countdown, progressive REVEAL, matched pairs, or player clicked
                      const isFlipped = isInCountdownCascade || isStillVisibleInCascade || (isGameplayPhase && (isProgressivelyRevealed || isMatched || isPlayerFlipped))
                      const isJustMatched = justMatchedPairs.includes(cardData.pairId)
                      // VPlayer can flip cards only if in active team (Memory multi-team mode)
                      // Admin TV can always flip, VPlayer can flip only if in MEMORY_CURRENT_TEAM
                      const isVPlayerInActiveTeam = isVPlayer && teamName && gameState.MEMORY_CURRENT_TEAM && teamName === gameState.MEMORY_CURRENT_TEAM
                      const canClick = gameState.phase === 'STARTED' && !isMatched && !isFlipped && (!isVPlayer || isVPlayerInActiveTeam)
                      // Only show matched styling during gameplay phases (not before game starts)
                      const showMatchedStyle = isGameplayPhase && isMatched
                      // Get team color for matched pairs (convert pairId to string for JSON key lookup)
                      const ownerTeam = gameState.MEMORY_PAIR_OWNERS?.[String(cardData.pairId)]
                      const teamColor = ownerTeam ? getTeamColorByName(ownerTeam) : '#4CAF50'

                      // DEBUG: Log team color lookup for matched pairs
                      if (isMatched && index === 0) {
                        console.log('[PlayerDisplay] Pair color lookup:', {
                          pairId: cardData.pairId,
                          pairIdString: String(cardData.pairId),
                          MEMORY_PAIR_OWNERS: gameState.MEMORY_PAIR_OWNERS,
                          ownerTeam,
                          teamColor
                        })
                      }
                      return (
                        <motion.div
                          key={cardData.id}
                          className={`memory-card ${isFlipped ? 'flipped' : ''} ${showMatchedStyle ? 'matched' : ''} ${isJustMatched ? 'just-matched' : ''}`}
                          initial={{ opacity: 0, scale: 0.8 }}
                          animate={{ opacity: 1, scale: 1 }}
                          transition={{ delay: index * 0.05, duration: 0.3 }}
                          onClick={() => canClick && flipMemoryCard(cardData.id)}
                          style={{
                            cursor: canClick ? 'pointer' : 'default',
                            '--matched-team-color': teamColor
                          }}
                        >
                          <div className="memory-card-inner">
                            <div className="memory-card-front">
                              {cardData.card.IS_IMAGE && cardData.card.IMAGE ? (
                                <img src={cardData.card.IMAGE} alt="" className="memory-card-image" />
                              ) : (
                                <span className="memory-card-text">{cardData.card.TEXT}</span>
                              )}
                            </div>
                            <div className="memory-card-back">
                              <span className="memory-card-letter">{cardLetter}</span>
                              {isAdminPreview && <span className="memory-card-pair-hint">{cardData.pairId}</span>}
                            </div>
                          </div>
                        </motion.div>
                      )
                    })}
                  </div>
                </div>

                {/* Zone 4: Team indicators during gameplay, results during reveal */}
                <div className="zone-answers">
                  {/* During COUNTDOWN/STARTED/PAUSED: Show all participating teams, highlight current */}
                  {['COUNTDOWN', 'STARTED', 'PAUSED'].includes(gameState.phase) &&
                   gameState.MEMORY_PARTICIPATING_TEAMS && gameState.MEMORY_PARTICIPATING_TEAMS.length > 0 && (
                    <div className="memory-team-bar">
                      {gameState.MEMORY_PARTICIPATING_TEAMS.map((teamName) => {
                        const teamData = teams[teamName]
                        const teamColor = teamData?.COLOR
                          ? (Array.isArray(teamData.COLOR) ? `rgb(${teamData.COLOR.join(',')})` : teamData.COLOR)
                          : 'var(--gray-400)'
                        // Highlight current team during COUNTDOWN/STARTED/PAUSED
                        const isActive = ['COUNTDOWN', 'STARTED', 'PAUSED'].includes(gameState.phase) && teamName === gameState.MEMORY_CURRENT_TEAM
                        return (
                          <div
                            key={teamName}
                            className={`memory-team-chip ${isActive ? 'active' : 'inactive'}`}
                            style={{
                              backgroundColor: teamColor,
                              '--team-color': teamColor
                            }}
                          >
                            {isActive && <span className="team-play-icon">🎮</span>}
                            <span className="team-name">{teamName}</span>
                          </div>
                        )
                      })}
                    </div>
                  )}

                  {/* During REVEALED: Show all teams with their results (pairs + errors) */}
                  {/* Note: Don't show the "X paires" answer text for multi-team Memory - it's redundant */}
                  {showAnswer && gameState.MEMORY_TEAM_PAIRS && Object.keys(gameState.MEMORY_TEAM_PAIRS).length > 0 && (
                    <div className="memory-team-results">
                      {Object.entries(gameState.MEMORY_TEAM_PAIRS)
                        .sort(([,a], [,b]) => b - a) // Sort by pairs descending
                        .map(([teamName, pairs], index) => {
                          const teamData = teams[teamName]
                          const teamColor = teamData?.COLOR
                            ? (Array.isArray(teamData.COLOR) ? `rgb(${teamData.COLOR.join(',')})` : teamData.COLOR)
                            : 'var(--gray-400)'
                          const errors = gameState.MEMORY_TEAM_ERRORS?.[teamName] || 0
                          return (
                            <motion.div
                              key={teamName}
                              className={`memory-team-result ${index === 0 ? 'winner' : ''}`}
                              style={{ backgroundColor: teamColor }}
                              initial={{ opacity: 0, y: 20 }}
                              animate={{ opacity: 1, y: 0 }}
                              transition={{ delay: index * 0.1 }}
                            >
                              {index === 0 && <span className="winner-icon">🏆</span>}
                              <span className="result-team-name">{teamName}</span>
                              <div className="result-stats">
                                <span className="result-pairs">✓ {pairs}</span>
                                <span className="result-errors">✗ {errors}</span>
                              </div>
                            </motion.div>
                          )
                        })}
                    </div>
                  )}

                  {/* Solo mode reveal (no teams) */}
                  {showAnswer && (!gameState.MEMORY_TEAM_PAIRS || Object.keys(gameState.MEMORY_TEAM_PAIRS).length === 0) && (
                    <motion.div
                      className="answer-container memory-answer"
                      initial={{ opacity: 0, scale: 0.8, y: 20 }}
                      animate={{ opacity: 1, scale: 1, y: 0 }}
                    >
                      <p className="answer-text">{gameState.question.ANSWER}</p>
                    </motion.div>
                  )}
                </div>
              </div>
              )
            })()}

            {/* MEMOTION Game Content — TV display, 4 subphases: GRID / SELECTED / QUESTION / REVEAL */}
            {isMemotion && (showGameContent || showReady) && gameState.question && (() => {
              const subphase = gameState.MEMOTION_SUBPHASE
              const cardStates = gameState.MEMOTION_CARD_STATES || {}
              const cardTeams = gameState.MEMOTION_CARD_TEAMS || {}
              const currentTeam = gameState.MEMOTION_CURRENT_TEAM
              const currentTeamColor = gameState.MEMOTION_CURRENT_TEAM_COLOR
              const currentTeamCss = Array.isArray(currentTeamColor) && currentTeamColor.length === 3
                ? `rgb(${currentTeamColor.join(',')})`
                : undefined
              const motionCards = gameState.question?.MOTION_CARDS || []
              const selectedId = gameState.MEMOTION_SELECTED
              const selectedCard = motionCards.find(c => c.ID === selectedId) || null
              const motionCfg = gameState.question?.MOTION_CONFIG
              const diffPts = d => getMotionCardPoints(d, motionCfg)

              /* ---- Secret mode ---- */
              const isSecretMode = isMotionSecretMode(gameState.question)
              const getCoord = getMotionCardCoord

              /* ---- Calcul grille (utils/motionGrid.js, #160/F0) ---- */
              const count = motionCards.length
              const motionCols = getMotionGridCols(count)
              const motionRows = getMotionGridRows(count)
              const participatingTeams = gameState.MEMOTION_PARTICIPATING_TEAMS || []

              /* ---- isMotionFullscreen : grille visible mais dimmed ---- */
              const isMotionFullscreen = subphase === 'SELECTED' || subphase === 'QUESTION' || subphase === 'REVEAL'

              /* ---- Grille (toujours rendue pour que layoutId fonctionne) ---- */
              const gridView = (
                <div className={`game-content-zones memory-game memotion-game${subphase === 'MEMORIZE' ? ' memotion-memorize-active' : ''}`}>
                  {/* Zone 1: Timer — reste visible (non dimmed) quand fullscreen overlay actif */}
                  <div className="zone-timer">
                    <Timer
                      currentTime={gameState.timer}
                      totalTime={gameState.totalTime}
                      phase={gameState.phase}
                      size="xl"
                      showPhase={false}
                    />
                  </div>

                  {/* Zones 2-4: contenu grille — dimmed séparément quand overlay actif */}
                  <div className={`memotion-grid-body${isMotionFullscreen ? ' memotion-grid-dimmed' : ''}`}>

                  {/* Zone 2: Current team or READY message or MEMORIZE banner */}
                  <div className="zone-question">
                    {subphase === 'MEMORIZE' ? (
                      <motion.div
                        className="memotion-memorize-banner"
                        initial={{ opacity: 0, y: -10 }}
                        animate={{ opacity: 1, y: 0 }}
                      >
                        🧠 MÉMORISEZ !
                      </motion.div>
                    ) : currentTeam ? (
                      <motion.div
                        className="memotion-current-team-label"
                        style={{ color: currentTeamCss, borderColor: currentTeamCss }}
                        initial={{ opacity: 0, y: -10 }}
                        animate={{ opacity: 1, y: 0 }}
                      >
                        {currentTeam}
                      </motion.div>
                    ) : showReady ? (
                      <motion.div
                        className="ready-state"
                        initial={{ opacity: 0, scale: 0.8 }}
                        animate={{ opacity: 1, scale: 1 }}
                      >
                        <ReadyCategoryDisplay catKey={gameState.question?.CATEGORY} customCategories={apiCategories} gameType="MEMOTION" />
                      </motion.div>
                    ) : (
                      <div className="zone-question-placeholder" />
                    )}
                  </div>

                  {/* Zone 3: Card grid — reuses memory-grid + memory-card system */}
                  <div className="zone-media zone-memory-grid">
                    <div
                      className="memory-grid"
                      style={{ '--memory-cols': motionCols, '--memory-rows': motionRows }}
                    >
                      {motionCards.map((card, index) => {
                        const state = cardStates[card.ID] || 'UNPLAYED'
                        const isDone = state === 'DONE'
                        const isActiveCard = !isDone && card.ID === selectedId
                        const winnerTeam = isDone ? (cardTeams[card.ID] || null) : null
                        const matchedColor = winnerTeam
                          ? getTeamColorByName(winnerTeam)
                          : 'rgba(255,255,255,0.4)'
                        const diff = card.DIFFICULTY || 1
                        const canSelectCard = isAdminPreview && subphase === 'GRID' && state === 'UNPLAYED'
                        return (
                          <motion.div
                            key={card.ID}
                            ref={el => { if (el) motionCardRefs.current[card.ID] = el; else delete motionCardRefs.current[card.ID] }}
                            className={`memory-card memotion-card${isDone ? ' matched' : ''}${isActiveCard ? ' selected' : ''}`}
                            style={{
                              ...(isDone ? { '--matched-team-color': matchedColor } : undefined),
                              visibility: isActiveCard && isMotionFullscreen ? 'hidden' : 'visible',
                              cursor: canSelectCard ? 'pointer' : 'default',
                            }}
                            initial={{ opacity: 0, scale: 1 }}
                            animate={{ opacity: 1, scale: isDone ? 0.8 : 1 }}
                            transition={{ delay: index * 0.05, duration: isDone ? 0.4 : 0.3 }}
                            onClick={() => canSelectCard && selectMotionCard(card.ID)}
                          >
                            {/* Use motion.div on inner so framer-motion drives the 3D flip
                                (CSS .flipped class is not used — framer-motion owns rotateY) */}
                            <motion.div
                              className="memory-card-inner"
                              animate={{ rotateY: 0 }}
                              transition={{ duration: 0 }}
                              style={{ transformStyle: 'preserve-3d' }}
                            >
                              {/* Front: shown when DONE (rotated 180° by framer-motion) */}
                              <div className="memory-card-front">
                                <span className="memotion-card-check">✓</span>
                                {winnerTeam ? (
                                  <span className="memotion-card-winner">{winnerTeam}</span>
                                ) : (
                                  <span className="memotion-card-nowinner">–</span>
                                )}
                              </div>
                              {/* Back: shown by default (UNPLAYED / in-progress) */}
                              <div className="memory-card-back">
                                {isSecretMode && subphase === 'GRID' ? (
                                  /* Secret mode GRID: coordinate only — no stars (reveals difficulty) */
                                  <>
                                    <div className="memotion-card-header" />
                                    <div className="memotion-card-body">
                                      <span className="memotion-card-coord">{getCoord(index, motionCols)}</span>
                                    </div>
                                    <div className="memotion-card-footer" />
                                  </>
                                ) : (
                                  /* Standard / MEMORIZE: show RECTO_THEME + image */
                                  <>
                                    <div className="memotion-card-header">
                                      <span className="memotion-card-title">{card.RECTO_THEME}</span>
                                    </div>
                                    <div className="memotion-card-body">
                                      {card.RECTO_IMAGE && (
                                        <img src={card.RECTO_IMAGE} alt="" className="memotion-card-img" />
                                      )}
                                    </div>
                                    <div className="memotion-card-footer">
                                      <span className="memotion-card-stars">{'★'.repeat(diff)}</span>
                                      {isDone && winnerTeam && (
                                        <span className="memotion-card-done-team">{winnerTeam}</span>
                                      )}
                                    </div>
                                  </>
                                )}
                              </div>
                            </motion.div>
                          </motion.div>
                        )
                      })}
                    </div>
                  </div>

                  {/* Zone 4: Participating teams bar — reuses memory-team-bar */}
                  <div className="zone-answers">
                    {participatingTeams.length > 0 && (
                      <div className="memory-team-bar">
                        {participatingTeams.map((tName) => {
                          const teamData = teams[tName]
                          const tColor = teamData?.COLOR
                            ? (Array.isArray(teamData.COLOR) ? `rgb(${teamData.COLOR.join(',')})` : teamData.COLOR)
                            : 'var(--gray-400)'
                          const isActive = tName === currentTeam
                          return (
                            <div
                              key={tName}
                              className={`memory-team-chip ${isActive ? 'active' : 'inactive'}`}
                              style={{ backgroundColor: tColor, '--team-color': tColor }}
                            >
                              {isActive && <span className="team-play-icon">🎮</span>}
                              <span className="team-name">{tName}</span>
                            </div>
                          )
                        })}
                      </div>
                    )}
                  </div>

                  </div>{/* /memotion-grid-body */}
                </div>
              )

              /* ---- READY: message PRÉPAREZ-VOUS sans la grille de cartes ---- */
              if (showReady) {
                return (
                  <div className="game-content-zones memory-game memotion-game">
                    <div className="zone-timer">
                      <Timer currentTime={gameState.timer} totalTime={gameState.totalTime} phase={gameState.phase} size="xl" showPhase={false} />
                    </div>
                    <div className="zone-question"><div className="zone-question-placeholder" /></div>
                    <div className="zone-media">
                      <motion.div className="ready-state" initial={{ opacity: 0, scale: 0.8 }} animate={{ opacity: 1, scale: 1 }}>
                        <ReadyCategoryDisplay catKey={gameState.question?.CATEGORY} customCategories={apiCategories} gameType="MEMOTION" />
                      </motion.div>
                    </div>
                    <div className="zone-answers">
                      {participatingTeams.length > 0 && (
                        <div className="memory-team-bar">
                          {participatingTeams.map((tName) => {
                            const teamData = teams[tName]
                            const tColor = teamData?.COLOR
                              ? (Array.isArray(teamData.COLOR) ? `rgb(${teamData.COLOR.join(',')})` : teamData.COLOR)
                              : 'var(--gray-400)'
                            return (
                              <div key={tName} className="memory-team-chip inactive" style={{ backgroundColor: tColor, '--team-color': tColor }}>
                                <span className="team-name">{tName}</span>
                              </div>
                            )
                          })}
                        </div>
                      )}
                    </div>
                  </div>
                )
              }

              /* Build fullscreen overlay — shared AnimatePresence enables SELECTED→QUESTION exit animation */
              let motionOverlay = null

              /* ---- SELECTED: carte sélectionnée zoome vers fullscreen via clip-path ---- */
              if (subphase === 'SELECTED' && selectedCard) {
                const diff = selectedCard.DIFFICULTY || 1
                const selectedEl = motionCardRefs.current[selectedId]
                const rect = selectedEl ? selectedEl.getBoundingClientRect() : null
                const W = window.innerWidth || 1920
                const H = window.innerHeight || 1080
                // overlay starts at top: 10vh → inset top is relative to overlay top edge
                const timerH = H * 0.1
                const clipStart = rect
                  ? `inset(${Math.max(0, rect.top - timerH)}px ${W - rect.right}px ${H - rect.bottom}px ${rect.left}px round 8px)`
                  : 'inset(40% 30% 40% 30% round 8px)'
                const clipEnd = 'inset(0px 0px 0px 0px round 0px)'
                motionOverlay = (
                  <motion.div
                    key={`memotion-selected-${selectedId}`}
                    className="memotion-tv-fullscreen memotion-tv-selected"
                    style={{ position: 'absolute', top: '10vh', left: 0, right: 0, bottom: 0, zIndex: 10 }}
                    initial={{ clipPath: clipStart }}
                    animate={{ clipPath: clipEnd }}
                    transition={{ duration: 0.45, ease: [0.4, 0, 0.2, 1] }}
                  >
                    {/* Row 1 : Titre (RECTO_THEME) */}
                    <div className="memotion-tv-fs-header memotion-tv-fs-recto-zone">
                      <span className="memotion-tv-fs-theme">{selectedCard.RECTO_THEME}</span>
                    </div>
                    {/* Row 2 : Image RECTO (ou texte thème si pas d'image) */}
                    <div className="memotion-tv-fs-body">
                      {selectedCard.RECTO_IMAGE ? (
                        <motion.img
                          src={selectedCard.RECTO_IMAGE}
                          alt=""
                          className="memotion-tv-fs-img"
                          initial={{ opacity: 0, scale: 0.9 }}
                          animate={{ opacity: 1, scale: 1 }}
                          transition={{ delay: 0.25 }}
                        />
                      ) : (
                        <motion.p
                          className="memotion-tv-fs-text"
                          initial={{ opacity: 0 }}
                          animate={{ opacity: 1 }}
                          transition={{ delay: 0.25 }}
                        >
                          {selectedCard.RECTO_THEME}
                        </motion.p>
                      )}
                    </div>
                    {/* Row 3 : Étoiles */}
                    <div className="memotion-tv-fs-footer memotion-tv-fs-recto-zone">
                      <span className="memotion-tv-fs-diff">{'★'.repeat(diff)}</span>
                    </div>
                  </motion.div>
                )

              /* ---- QUESTION: flip depuis SELECTED vers question ---- */
              } else if (subphase === 'QUESTION' && selectedCard) {
                const diff = selectedCard.DIFFICULTY || 1
                motionOverlay = (
                  <motion.div
                    key="memotion-question"
                    className="memotion-tv-fullscreen"
                    initial={{ rotateY: -90 }}
                    animate={{ rotateY: 0 }}
                    exit={{ rotateY: 90, transition: { duration: 0.35 } }}
                    transition={{ duration: 0.5, ease: [0.4, 0, 0.2, 1] }}
                    style={{ position: 'absolute', top: '10vh', left: 0, right: 0, bottom: 0, zIndex: 10, perspective: '1200px' }}
                  >
                    {/* Row 1 : Texte de la question */}
                    <div className="memotion-tv-fs-header memotion-tv-fs-recto-zone">
                      {selectedCard.QUESTION_TEXT && (
                        <motion.p
                          className="memotion-tv-fs-question-text"
                          initial={{ opacity: 0, y: -10 }}
                          animate={{ opacity: 1, y: 0 }}
                          transition={{ delay: 0.3 }}
                        >
                          {selectedCard.QUESTION_TEXT}
                        </motion.p>
                      )}
                    </div>
                    {/* Row 2 : Image de la question */}
                    <div className="memotion-tv-fs-body">
                      {selectedCard.QUESTION_IMAGE && (
                        <motion.img
                          src={selectedCard.QUESTION_IMAGE}
                          alt=""
                          className="memotion-tv-fs-img"
                          initial={{ opacity: 0, scale: 0.9 }}
                          animate={{ opacity: 1, scale: 1 }}
                          transition={{ delay: 0.3 }}
                        />
                      )}
                    </div>
                    {/* Row 3 : Vide (zone réponses — libre) */}
                    <div className="memotion-tv-fs-footer" />
                  </motion.div>
                )

              /* ---- REVEAL: flip depuis QUESTION vers réponse ---- */
              } else if (subphase === 'REVEAL' && selectedCard) {
                const diff = selectedCard.DIFFICULTY || 1
                const selectedEl = motionCardRefs.current[selectedId]
                const rect = selectedEl ? selectedEl.getBoundingClientRect() : null
                const W = window.innerWidth || 1920
                const H = window.innerHeight || 1080
                // overlay starts at top: 10vh → inset top is relative to overlay top edge
                const timerH = H * 0.1
                const clipStart = rect
                  ? `inset(${Math.max(0, rect.top - timerH)}px ${W - rect.right}px ${H - rect.bottom}px ${rect.left}px round 8px)`
                  : 'inset(40% 30% 40% 30% round 8px)'
                motionOverlay = (
                  <motion.div
                    key="memotion-reveal-overlay"
                    className="memotion-tv-fullscreen memotion-tv-reveal"
                    initial={{ clipPath: 'inset(0px 0px 0px 0px round 0px)', rotateY: -90 }}
                    animate={{ clipPath: 'inset(0px 0px 0px 0px round 0px)', rotateY: 0 }}
                    exit={{ clipPath: clipStart, transition: { duration: 0.35, ease: 'easeIn' } }}
                    transition={{ duration: 0.5, ease: [0.4, 0, 0.2, 1] }}
                    style={{ position: 'absolute', top: '10vh', left: 0, right: 0, bottom: 0, zIndex: 10, perspective: '1200px' }}
                  >
                    {/* Row 1 : Rappel de la question */}
                    <div className="memotion-tv-fs-header memotion-tv-fs-recto-zone">
                      {selectedCard.QUESTION_TEXT && (
                        <p className="memotion-tv-fs-question-text memotion-tv-fs-recall">
                          {selectedCard.QUESTION_TEXT}
                        </p>
                      )}
                    </div>
                    {/* Row 2 : Image réponse (ou texte si pas d'image) */}
                    <div className="memotion-tv-fs-body">
                      {selectedCard.ANSWER_IMAGE ? (
                        <motion.img
                          src={selectedCard.ANSWER_IMAGE}
                          alt=""
                          className="memotion-tv-fs-img"
                          initial={{ opacity: 0, scale: 0.9 }}
                          animate={{ opacity: 1, scale: 1 }}
                          transition={{ delay: 0.3 }}
                        />
                      ) : selectedCard.ANSWER_TEXT ? (
                        <motion.p
                          className="memotion-tv-fs-text memotion-tv-answer-text"
                          initial={{ opacity: 0, y: 20 }}
                          animate={{ opacity: 1, y: 0 }}
                          transition={{ delay: 0.35 }}
                        >
                          {selectedCard.ANSWER_TEXT}
                        </motion.p>
                      ) : null}
                    </div>
                    {/* Row 3 : Texte réponse si image + texte coexistent */}
                    {selectedCard.ANSWER_IMAGE && selectedCard.ANSWER_TEXT ? (
                      <motion.div
                        className="memotion-tv-fs-footer"
                        initial={{ opacity: 0, y: 10 }}
                        animate={{ opacity: 1, y: 0 }}
                        transition={{ delay: 0.45 }}
                      >
                        {selectedCard.ANSWER_TEXT}
                      </motion.div>
                    ) : (
                      <div className="memotion-tv-fs-footer" />
                    )}
                  </motion.div>
                )
              }

              /* GRID (subphase null, "GRID", or unknown): motionOverlay stays null */
              return (
                <>
                  {gridView}
                  <AnimatePresence mode="wait">
                    {motionOverlay}
                  </AnimatePresence>
                </>
              )
            })()}

            {/* Non-QCM/Non-Memory/Non-Memotion Game Content - 4 vertical zones: Timer, Question, Media, Answers */}
            {/* ARDOISE TV reveal handled separately below — exclude from this block during REVEALED non-VPlayer */}
            {!isQcm && !isMemory && !isMemotion && !(isArdoise && showAnswer && !isVPlayer) && showGameContent && gameState.question && (
              <div className="game-content-zones">
                {/* Zone 1: Timer */}
                <div className="zone-timer">
                  <Timer
                    currentTime={gameState.timer}
                    totalTime={gameState.totalTime}
                    phase={gameState.phase}
                    size="xl"
                    showPhase={false}
                  />
                </div>

                {/* Zone 2: Question */}
                <motion.div
                  className="zone-question"
                  initial={{ opacity: 0, y: 20 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: 0.1 }}
                >
                  <p className="question-text">{gameState.question.QUESTION}</p>
                </motion.div>

                {/* Zone 3: Media - shows MEDIA_ANSWER during REVEAL if available - ALWAYS clickable for VPlayer buzz */}
                <div
                  className="zone-media"
                  onClick={onMediaClick && isVPlayer && (gameState.phase === 'STARTED' || gameState.phase === 'PAUSED') ? onMediaClick : undefined}
                  style={{ cursor: onMediaClick && isVPlayer && (gameState.phase === 'STARTED' || gameState.phase === 'PAUSED') ? 'pointer' : 'default' }}
                >
                  {(showAnswer && gameState.question.MEDIA_ANSWER) ? (
                    <motion.img
                      key="answer-media"
                      src={gameState.question.MEDIA_ANSWER}
                      alt=""
                      className="question-media answer-media-highlight"
                      initial={{ opacity: 0, scale: 0.9 }}
                      animate={{ opacity: 1, scale: 1 }}
                      transition={{ delay: 0.2 }}
                    />
                  ) : (gameState.question.MEDIA || defaultQuestionImage) ? (
                    <motion.img
                      key="question-media"
                      src={gameState.question.MEDIA || defaultQuestionImage}
                      alt=""
                      className={`question-media${!gameState.question.MEDIA && defaultQuestionImage ? ' default-question-image' : ''}`}
                      initial={{ opacity: 0 }}
                      animate={{ opacity: 1 }}
                      transition={{ delay: 0.2 }}
                    />
                  ) : null}
                </div>

                {/* Zone 4: Answer - only in REVEALED phase */}
                <div className="zone-answers">
                  {showAnswer && (
                    <motion.div
                      className="answer-container"
                      initial={{ opacity: 0, scale: 0.8, y: 20 }}
                      animate={{ opacity: 1, scale: 1, y: 0 }}
                    >
                      <p className="answer-text">{gameState.question.ANSWER}</p>
                    </motion.div>
                  )}
                </div>

              </div>
            )}

            {/* ARDOISE TV Reveal — standard zone layout (#90 #92 #93 #94) */}
            {isArdoise && showAnswer && !isVPlayer && gameState.question && (
              <div className="game-content-zones">

                {/* Zone 1: Question text */}
                <motion.div
                  className="zone-question"
                  initial={{ opacity: 0, y: 20 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: 0.1 }}
                >
                  <p className="question-text">{gameState.question.QUESTION}</p>
                </motion.div>

                {/* Zone 2: Media — MEDIA_ANSWER in REVEALED (highlighted), question MEDIA otherwise (#94) */}
                {(gameState.question.MEDIA_ANSWER || gameState.question.MEDIA) && (
                  <div className="zone-media">
                    {gameState.question.MEDIA_ANSWER ? (
                      <motion.img
                        key="ardoise-answer-media"
                        src={gameState.question.MEDIA_ANSWER}
                        alt=""
                        className="question-media answer-media-highlight"
                        initial={{ opacity: 0, scale: 0.9 }}
                        animate={{ opacity: 1, scale: 1 }}
                        transition={{ delay: 0.15 }}
                      />
                    ) : (
                      <motion.img
                        key="ardoise-question-media"
                        src={gameState.question.MEDIA}
                        alt=""
                        className="question-media"
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        transition={{ delay: 0.15 }}
                      />
                    )}
                  </div>
                )}

                {/* Zone 3: 2 equal sub-zones — correct answer (top) + team cards (bottom) */}
                <motion.div
                  className="zone-answers"
                  initial={{ opacity: 0, scale: 0.9 }}
                  animate={{ opacity: 1, scale: 1 }}
                  transition={{ delay: 0.2 }}
                >
                  {/* Sub-zone 1/2 — correct answer (QUIZZ style) */}
                  <div className="ardoise-footer-top">
                    <motion.div
                      className="answer-container"
                      initial={{ opacity: 0, scale: 0.8, y: 20 }}
                      animate={{ opacity: 1, scale: 1, y: 0 }}
                    >
                      <p className="answer-text ardoise-correct-answer">✓ {gameState.question.ANSWER}</p>
                    </motion.div>
                  </div>

                  {/* Sub-zone 2/2 — team cards grid (only VJoueur teams, max 8, static) */}
                  <div className="ardoise-footer-bottom">
                    <div className="ardoise-teams-grid">
                      {Object.values(teams)
                        .filter(team => Object.values(bumpers).some(b => b.IS_VPLAYER && b.TEAM === team.NAME))
                        .slice(0, 8)
                        .map((team, idx) => {
                          const answer = (gameState.ARDOISE_ANSWERS || {})[team.NAME]
                          const teamColor = getRgbColor(team.COLOR)
                          return (
                            <motion.div
                              key={team.NAME}
                              className={`ardoise-team-card ${answer ? 'has-answer' : 'no-answer'}`}
                              initial={{ opacity: 0, y: 15 }}
                              animate={{ opacity: 1, y: 0 }}
                              transition={{ delay: 0.3 + idx * 0.05 }}
                            >
                              <div
                                className="ardoise-team-card-header"
                                style={{ backgroundColor: teamColor }}
                              >
                                {team.NAME}
                              </div>
                              <div className="ardoise-team-card-answer">
                                {answer?.TEXT || '—'}
                              </div>
                            </motion.div>
                          )
                        })}
                    </div>
                  </div>
                </motion.div>
              </div>
            )}

            {/* Waiting State - no question selected (NOT shown for VPlayer) */}
            {!isVPlayer && !gameState.question && ['STOPPED', 'REVEALED'].includes(gameState.phase) && (
              <motion.div
                className="waiting-state"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
              >
                <span className="waiting-emoji">🎮</span>
                <span className="waiting-text">En attente de la prochaine question...</span>
              </motion.div>
            )}

            {/* Points Animation - floating +X pts when score changes */}
            <AnimatePresence>
              {pointsAnimation && (
                <motion.div
                  className="points-animation"
                  initial={{ opacity: 0, scale: 0.5, y: 100 }}
                  animate={{ opacity: 1, scale: 1, y: 0 }}
                  exit={{ opacity: 0, scale: 1.5, y: -100 }}
                  transition={{ duration: 0.5 }}
                  style={{
                    '--team-color': pointsAnimation.color
                      ? `rgb(${pointsAnimation.color.join(',')})`
                      : 'var(--success)'
                  }}
                >
                  <span className="points-team">{pointsAnimation.name}</span>
                  <span className="points-value">+{pointsAnimation.points} pts</span>
                </motion.div>
              )}
            </AnimatePresence>
          </motion.div>
        )}
      </AnimatePresence>
      </div>
      {/* /.entracte-content */}

      {/* ENTRACTE panel — frère du contenu filtré, jamais enfant (même piège
          position:fixed / z-index que QRCodeOverlay et le bouton plein écran).
          AnimatePresence (#119, C3) : sans elle, le panneau serait démonté
          instantanément à la sortie, quelle que soit TRANSITION_MS — il
          n'existerait plus pour jouer son fondu de sortie. */}
      <AnimatePresence>
        {entracteActiveHere && <EntractePanel key="entracte-panel" config={gameState.entracteConfig} />}
      </AnimatePresence>

      {/* QR Code Overlay — suppressed during ENROLL (dedicated two-QR view handles it) */}
      <QRCodeOverlay show={(gameState.showQRCode || false) && gameState.phase !== 'ENROLL'} />
    </div>
  )
}
