import { useEffect, useMemo, useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import confetti from 'canvas-confetti'
import { useGame } from '../hooks/GameContext'
import Timer from '../components/Timer'
import Podium from '../components/Podium'
import QRCodeOverlay from '../components/QRCodeOverlay'
import QRCodeDisplay from '../components/QRCodeDisplay'
import { CATEGORIES } from './QuestionsPage'
import { getCategoryColor } from '../constants/colors'
import './PlayerDisplay.css'
import '../styles/neon.css'

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

export default function PlayerDisplay({ playerName = null, playerNameColor = null, teamName = null, teamColor = null, isVPlayer = false, onMediaClick = null }) {
  const { gameState, teams, bumpers, flipMemoryCard, showQRCode } = useGame()
  const [previousRanking, setPreviousRanking] = useState({})
  const [changedTeams, setChangedTeams] = useState({})
  const [history, setHistory] = useState([]) // For PALMARES view
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
  const [localCountdown, setLocalCountdown] = useState(null) // Local countdown that starts after cascade reveal is done

  // Check if admin mode (for pair hints visibility)
  const isAdminPreview = useMemo(() => {
    const params = new URLSearchParams(window.location.search)
    return params.get('admin') === 'true'
  }, [])

  // Fetch history for PALMARES view
  useEffect(() => {
    if (gameState.remote !== 'PALMARES') return

    const fetchHistory = async () => {
      try {
        const response = await fetch('/history')
        if (response.ok) {
          const data = await response.json()
          setHistory(data || [])
        }
      } catch (error) {
        console.error('Failed to fetch history:', error)
      }
    }

    fetchHistory()
    const interval = setInterval(fetchHistory, 5000)
    return () => clearInterval(interval)
  }, [gameState.remote])

  // Aggregate history by category for PALMARES view
  const categoryStats = useMemo(() => {
    const stats = {}

    history.forEach(event => {
      const category = event.QUESTION_CATEGORY || 'UNKNOWN'

      if (!stats[category]) {
        stats[category] = {
          teams: {},
          players: {},
          totalTeamPoints: 0,
          totalPlayerPoints: 0,
        }
      }

      if (event.WINNER_TYPE === 'TEAM' && event.TEAM_NAME) {
        if (!stats[category].teams[event.TEAM_NAME]) {
          stats[category].teams[event.TEAM_NAME] = {
            name: event.TEAM_NAME,
            color: event.TEAM_COLOR,
            points: 0,
          }
        }
        stats[category].teams[event.TEAM_NAME].points += event.POINTS
        stats[category].totalTeamPoints += event.POINTS
      } else if (event.WINNER_TYPE === 'PLAYER' && event.PLAYER_NAME) {
        const key = `${event.TEAM_NAME}|${event.PLAYER_NAME}`
        if (!stats[category].players[key]) {
          stats[category].players[key] = {
            name: event.PLAYER_NAME,
            team: event.TEAM_NAME,
            teamColor: event.TEAM_COLOR,
            points: 0,
          }
        }
        stats[category].players[key].points += event.POINTS
        stats[category].totalPlayerPoints += event.POINTS
      }
    })

    // Convert to arrays and sort
    Object.keys(stats).forEach(category => {
      const sortedTeams = Object.values(stats[category].teams)
        .sort((a, b) => b.points - a.points)
      let currentRank = 1
      stats[category].sortedTeams = sortedTeams.map((team, index) => {
        if (index > 0 && sortedTeams[index - 1].points === team.points) {
          return { ...team, rank: currentRank }
        }
        currentRank = index + 1
        return { ...team, rank: currentRank }
      })

      const sortedPlayers = Object.values(stats[category].players)
        .sort((a, b) => b.points - a.points)
      currentRank = 1
      stats[category].sortedPlayers = sortedPlayers.map((player, index) => {
        if (index > 0 && sortedPlayers[index - 1].points === player.points) {
          return { ...player, rank: currentRank }
        }
        currentRank = index + 1
        return { ...player, rank: currentRank }
      })
    })

    return Object.entries(stats)
      .filter(([_, data]) => data.sortedTeams.length > 0 || data.sortedPlayers.length > 0)
      .sort((a, b) => {
        const totalA = a[1].totalTeamPoints + a[1].totalPlayerPoints
        const totalB = b[1].totalTeamPoints + b[1].totalPlayerPoints
        return totalB - totalA
      })
  }, [history])

  // Sort teams by score for scoreboard with rank calculation
  const sortedTeams = useMemo(() => {
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
  }, [teams])

  // Sort players (bumpers) by score
  const sortedPlayers = useMemo(() => {
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
    return sorted.map((player, index) => {
      if (index > 0 && player.score === lastScore) {
        return { ...player, rank: lastRank }
      }
      currentRank = index + 1
      lastScore = player.score
      lastRank = currentRank
      return { ...player, rank: currentRank }
    })
  }, [bumpers, teams])

  // Detect team ranking changes
  useEffect(() => {
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
  }, [sortedTeams])

  // Detect player score changes
  useEffect(() => {
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
  }, [sortedPlayers])

  // Detect score changes and trigger celebration animation in GAME view
  useEffect(() => {
    // Check for team score changes
    sortedTeams.forEach((team) => {
      const prev = previousRanking[team.name]
      if (prev && prev.score !== team.score && team.score > prev.score) {
        const pointsAdded = team.score - prev.score
        setPointsAnimation({
          name: team.name,
          points: pointsAdded,
          color: team.color
        })
        triggerCelebration(team.color)
        setTimeout(() => setPointsAnimation(null), 2500)
      }
    })
  }, [sortedTeams, previousRanking])

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
  const isMemory = gameState.question?.TYPE === 'MEMORY'
  // QCM answers visible from READY through REVEALED (no re-render on transition)
  const showQcmAnswers = ['READY', 'COUNTDOWN', 'STARTED', 'PAUSED', 'STOPPED', 'REVEALED'].includes(gameState.phase)
  // Memory grid visible from READY (cards face down during countdown) through REVEALED
  const showMemoryGrid = ['READY', 'COUNTDOWN', 'STARTED', 'PAUSED', 'STOPPED', 'REVEALED'].includes(gameState.phase)
  // QCM answer TEXT visible from COUNTDOWN (READY shows only colored zones with letters)
  const showQcmAnswerText = ['COUNTDOWN', 'STARTED', 'PAUSED', 'STOPPED', 'REVEALED'].includes(gameState.phase)

  // Top 3 players for podium
  const topPlayers = useMemo(() => {
    return sortedPlayers.slice(0, 3).map(player => ({
      ...player,
      name: player.name,
      color: player.teamColor,
    }))
  }, [sortedPlayers])

  // Prepare Memory cards - all cards shuffled (Card1 and Card2 from each pair)
  const memoryCards = useMemo(() => {
    const pairs = gameState.question?.MEMORY_PAIRS || []
    if (pairs.length === 0) return []

    // Create array with all cards, each linked to its pair ID
    const allCards = []
    pairs.forEach((pair) => {
      allCards.push({
        id: `${pair.ID}-1`,
        pairId: pair.ID,
        card: pair.CARD1,
        cardIndex: 1,
      })
      allCards.push({
        id: `${pair.ID}-2`,
        pairId: pair.ID,
        card: pair.CARD2,
        cardIndex: 2,
      })
    })

    // Shuffle using Fisher-Yates with seeded randomness based on question ID
    // This ensures consistent shuffle for same question across all clients
    const questionId = gameState.question?.ID || '0'
    let seed = parseInt(questionId, 10) || 1
    const shuffled = [...allCards]
    for (let i = shuffled.length - 1; i > 0; i--) {
      seed = (seed * 1103515245 + 12345) & 0x7fffffff
      const j = seed % (i + 1)
      ;[shuffled[i], shuffled[j]] = [shuffled[j], shuffled[i]]
    }
    return shuffled
  }, [gameState.question?.MEMORY_PAIRS, gameState.question?.ID])

  // Calculate grid columns and rows based on card count
  // Optimized for common configurations: 2x2, 2x3, 3x4, 4x4, 4x5, 4x6
  const memoryGridCols = useMemo(() => {
    const cardCount = memoryCards.length
    if (cardCount <= 4) return 2   // 2x2
    if (cardCount <= 6) return 3   // 2x3
    if (cardCount <= 16) return 4  // 4x4 max (changed from 12)
    if (cardCount <= 20) return 5  // 4x5
    return 6                       // 4x6
  }, [memoryCards.length])

  const memoryGridRows = useMemo(() => {
    const cardCount = memoryCards.length
    return Math.ceil(cardCount / memoryGridCols)
  }, [memoryCards.length, memoryGridCols])

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

  const getRgbColor = (color) => {
    if (!color) return 'var(--gray-400)'
    if (Array.isArray(color)) return `rgb(${color.join(',')})`
    return color
  }

  // Neon effect configuration
  const neonConfig = useMemo(() => {
    return gameState?.neonEffect || {
      enabled: false,
      arc_width: 60,
      intensity_gap: 80,
      rotation_speed: 4
    }
  }, [gameState?.neonEffect])

  // Show neon effect during game phases
  const showNeon = useMemo(() => {
    return neonConfig.enabled &&
      ['READY', 'COUNTDOWN', 'STARTED', 'PAUSED'].includes(gameState?.phase)
  }, [neonConfig.enabled, gameState?.phase])

  // Get category color for neon effect
  const neonCategoryColor = useMemo(() => {
    return getCategoryColor(gameState?.question?.CATEGORY)
  }, [gameState?.question?.CATEGORY])

  // Neon style variables
  const neonStyle = useMemo(() => {
    if (!showNeon) return {}
    const barThickness = neonConfig.bar_thickness || 4
    const arcBlurPercent = neonConfig.arc_blur !== undefined ? neonConfig.arc_blur : 100
    // arc_blur is 0-200% of bar thickness
    const arcBlurPx = (barThickness * arcBlurPercent) / 100
    return {
      '--neon-color': neonCategoryColor,
      '--neon-arc-width': `${neonConfig.arc_width}deg`,
      '--neon-intensity-gap': neonConfig.intensity_gap / 100,
      '--neon-rotation-speed': `${neonConfig.rotation_speed}s`,
      '--neon-bar-offset': `${neonConfig.bar_offset || 20}px`,
      '--neon-bar-thickness': `${barThickness}px`,
      '--neon-arc-blur': `${arcBlurPx}px`,
    }
  }, [showNeon, neonCategoryColor, neonConfig])

  // Neon mode class
  const neonModeClass = neonConfig.mode === 'halo' ? 'neon-mode-halo' : 'neon-mode-bar'

  return (
    <div className={`player-display ${showNeon ? `neon-border ${neonModeClass}` : ''}`} style={neonStyle}>
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

            <div className="enroll-qr-zone">
              <QRCodeDisplay url={`http://${window.location.hostname}/player`} size={400} />
              <p className="enroll-instruction">Scannez ce code avec votre smartphone</p>
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
              {categoryStats.length === 0 ? (
                <div className="palmares-empty">
                  <p>Aucun evenement enregistre</p>
                </div>
              ) : (
                categoryStats.slice(0, 6).map(([category, data], index) => { // Max 6 categories for TV (3x2 grid)
                  const catInfo = CATEGORIES[category] || { label: category, icon: '❓', color: '#6b7280' }
                  const hasTeams = data.sortedTeams.length > 0
                  const hasPlayers = data.sortedPlayers.length > 0

                  return (
                    <motion.div
                      key={category}
                      className="palmares-category"
                      initial={{ opacity: 0, y: 20 }}
                      animate={{ opacity: 1, y: 0 }}
                      transition={{ delay: index * 0.1 }}
                    >
                      <div className="palmares-category-header" style={{ backgroundColor: catInfo.color }}>
                        <span className="palmares-category-icon">{catInfo.icon}</span>
                        <span className="palmares-category-name">{catInfo.label}</span>
                        <div className="palmares-category-stats">
                          {hasTeams && <span className="palmares-stat">👥 {data.totalTeamPoints} pts</span>}
                          {hasPlayers && <span className="palmares-stat">👤 {data.totalPlayerPoints} pts</span>}
                        </div>
                      </div>

                      <div className="palmares-category-content">
                        {hasTeams && (
                          <div className="palmares-ranking">
                            <h3 className="palmares-ranking-title">Equipes</h3>
                            <div className="palmares-ranking-list">
                              {data.sortedTeams.slice(0, 3).map((team, idx) => (
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
                              {data.sortedPlayers.slice(0, 3).map((player, idx) => (
                                <div
                                  key={`${player.team}|${player.name}`}
                                  className={`palmares-ranking-item rank-${player.rank}`}
                                  style={{ '--team-color': player.teamColor ? `rgb(${player.teamColor.join(',')})` : '#6b7280' }}
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
            {showCountdown && !isQcm && !isMemory && (
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
                  {gameState.question?.CATEGORY && CATEGORIES[gameState.question.CATEGORY] && (
                    <motion.div
                      className="category-badge-inline"
                      style={{ backgroundColor: CATEGORIES[gameState.question.CATEGORY].color }}
                      initial={{ opacity: 0, y: 150, scale: 1.5 }}
                      animate={{ opacity: 1, y: 0, scale: 1 }}
                      transition={{ duration: 0.5, ease: 'easeOut' }}
                    >
                      <span className="category-badge-icon">{CATEGORIES[gameState.question.CATEGORY].icon}</span>
                      <span className="category-badge-label">{CATEGORIES[gameState.question.CATEGORY].label}</span>
                    </motion.div>
                  )}
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
            {showReady && !isQcm && !isMemory && (
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
                    {gameState.question?.CATEGORY && CATEGORIES[gameState.question.CATEGORY] ? (
                      <>
                        <motion.span
                          className="ready-category-icon"
                          animate={{ scale: [1, 1.3, 1] }}
                          transition={{ duration: 0.4, repeat: Infinity }}
                        >
                          {CATEGORIES[gameState.question.CATEGORY].icon}
                        </motion.span>
                        <motion.span
                          className="ready-category-name"
                          style={{ backgroundColor: CATEGORIES[gameState.question.CATEGORY].color }}
                          animate={{ opacity: [1, 0.7, 1] }}
                          transition={{ duration: 0.6, repeat: Infinity }}
                        >
                          {CATEGORIES[gameState.question.CATEGORY].label}
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
                          animate={{ opacity: [1, 0.2, 1] }}
                          transition={{ duration: 0.6, repeat: Infinity }}
                        >
                          PREPAREZ-VOUS
                        </motion.span>
                      </>
                    )}
                  </motion.div>
                </div>

                {/* Zone 4: Empty answers placeholder */}
                <div className="zone-answers" />
              </div>
            )}

            {/* QCM Game Content - unified block for READY through REVEALED (no flash on transition) */}
            {isQcm && showQcmAnswers && gameState.question && (
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
                  ) : showCountdown && gameState.question?.CATEGORY && CATEGORIES[gameState.question.CATEGORY] ? (
                    <motion.div
                      className="category-badge-inline"
                      style={{ backgroundColor: CATEGORIES[gameState.question.CATEGORY].color }}
                      initial={{ opacity: 0, y: 150, scale: 1.5 }}
                      animate={{ opacity: 1, y: 0, scale: 1 }}
                      transition={{ duration: 0.5, ease: 'easeOut' }}
                    >
                      <span className="category-badge-icon">{CATEGORIES[gameState.question.CATEGORY].icon}</span>
                      <span className="category-badge-label">{CATEGORIES[gameState.question.CATEGORY].label}</span>
                    </motion.div>
                  ) : (
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
                      {gameState.question?.CATEGORY && CATEGORIES[gameState.question.CATEGORY] ? (
                        <>
                          <motion.span
                            className="ready-category-icon"
                            animate={{ scale: [1, 1.3, 1] }}
                            transition={{ duration: 0.4, repeat: Infinity }}
                          >
                            {CATEGORIES[gameState.question.CATEGORY].icon}
                          </motion.span>
                          <motion.span
                            className="ready-category-name"
                            style={{ backgroundColor: CATEGORIES[gameState.question.CATEGORY].color }}
                            animate={{ opacity: [1, 0.7, 1] }}
                            transition={{ duration: 0.6, repeat: Infinity }}
                          >
                            {CATEGORIES[gameState.question.CATEGORY].label}
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
                            animate={{ opacity: [1, 0.2, 1] }}
                            transition={{ duration: 0.6, repeat: Infinity }}
                          >
                            PREPAREZ-VOUS
                          </motion.span>
                        </>
                      )}
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
                  ) : gameState.question.MEDIA ? (
                    <motion.img
                      key="question-media"
                      src={gameState.question.MEDIA}
                      alt=""
                      className="question-media"
                      initial={{ opacity: 0, y: 20 }}
                      animate={{ opacity: 1, y: 0 }}
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

                      return (
                        <motion.div
                          key={colorKey}
                          className={`qcm-answer-item ${showAnswer ? (isCorrect ? 'correct' : 'wrong') : ''} ${isInvalidated ? 'invalidated' : ''}`}
                          style={{
                            backgroundColor: isInvalidated ? '#374151' : (showAnswer && !isCorrect ? '#4b5563' : colorData.color),
                            opacity: isInvalidated ? 0.35 : (showAnswer && !isCorrect ? 0.4 : 1)
                          }}
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
            {isMemory && showMemoryGrid && gameState.question && (
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
                      {gameState.question?.CATEGORY && CATEGORIES[gameState.question.CATEGORY] ? (
                        <motion.div
                          className="category-badge-inline category-badge-large"
                          style={{ backgroundColor: CATEGORIES[gameState.question.CATEGORY].color }}
                          animate={{ scale: [1, 1.05, 1] }}
                          transition={{ duration: 0.6, repeat: Infinity }}
                        >
                          <span className="category-badge-icon">{CATEGORIES[gameState.question.CATEGORY].icon}</span>
                          <span className="category-badge-label">{CATEGORIES[gameState.question.CATEGORY].label}</span>
                        </motion.div>
                      ) : (
                        <>
                          <span className="prepare-emoji">🔔</span>
                          <span className="prepare-text">PREPAREZ-VOUS</span>
                        </>
                      )}
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
                      // VPlayer cannot flip cards - only admin can
                      const canClick = gameState.phase === 'STARTED' && !isMatched && !isFlipped && !isVPlayer
                      // Only show matched styling during gameplay phases (not before game starts)
                      const showMatchedStyle = isGameplayPhase && isMatched
                      return (
                        <motion.div
                          key={cardData.id}
                          className={`memory-card ${isFlipped ? 'flipped' : ''} ${showMatchedStyle ? 'matched' : ''} ${isJustMatched ? 'just-matched' : ''}`}
                          initial={{ opacity: 0, scale: 0.8 }}
                          animate={{ opacity: 1, scale: 1 }}
                          transition={{ delay: index * 0.05, duration: 0.3 }}
                          onClick={() => canClick && flipMemoryCard(cardData.id)}
                          style={{ cursor: canClick ? 'pointer' : 'default' }}
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

                {/* Zone 4: Memory stats during gameplay, answer during reveal */}
                <div className="zone-answers">
                  {/* Memory stats during STARTED/PAUSED */}
                  {(gameState.phase === 'STARTED' || gameState.phase === 'PAUSED') && (
                    <div className="memory-stats">
                      <div className="memory-stat">
                        <span className="memory-stat-label">Paires</span>
                        <span className="memory-stat-value">
                          {gameState.memoryMatchedPairs?.length || 0} / {gameState.question?.MEMORY_PAIRS?.length || 0}
                        </span>
                      </div>
                      {(gameState.memoryErrors > 0 || gameState.question?.MEMORY_CONFIG?.ERROR_PENALTY > 0) && (
                        <div className="memory-stat errors">
                          <span className="memory-stat-label">Erreurs</span>
                          <span className="memory-stat-value">{gameState.memoryErrors || 0}</span>
                        </div>
                      )}
                    </div>
                  )}
                  {/* Answer during REVEALED */}
                  {showAnswer && (
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
            )}

            {/* Non-QCM/Non-Memory Game Content - 4 vertical zones: Timer, Question, Media, Answers */}
            {!isQcm && !isMemory && showGameContent && gameState.question && (
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
                  ) : gameState.question.MEDIA ? (
                    <motion.img
                      key="question-media"
                      src={gameState.question.MEDIA}
                      alt=""
                      className="question-media"
                      initial={{ opacity: 0, y: 20 }}
                      animate={{ opacity: 1, y: 0 }}
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

      {/* QR Code Overlay for player enrollment */}
      <QRCodeOverlay show={gameState.showQRCode || false} />
    </div>
  )
}
