import { useEffect, useMemo, useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import confetti from 'canvas-confetti'
import { useGame } from '../hooks/GameContext'
import Timer from '../components/Timer'
import Podium from '../components/Podium'
import { CATEGORIES } from './QuestionsPage'
import './PlayerDisplay.css'

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

export default function PlayerDisplay() {
  const { gameState, teams, bumpers } = useGame()
  const [previousRanking, setPreviousRanking] = useState({})
  const [changedTeams, setChangedTeams] = useState({})
  const [previousPlayerScores, setPreviousPlayerScores] = useState({})
  const [changedPlayers, setChangedPlayers] = useState({})
  const [pointsAnimation, setPointsAnimation] = useState(null) // {name, points, color}

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
  const isShowingGame = !isShowingScores && !isShowingPlayers
  const maxTeamScore = Math.max(...sortedTeams.map(t => t.score), 1)
  const maxPlayerScore = Math.max(...sortedPlayers.map(p => p.score), 1)

  // Display logic for game content
  const showPrepare = gameState.phase === 'PREPARE'
  const showReady = gameState.phase === 'READY'
  const showCountdown = gameState.phase === 'COUNTDOWN'
  const showGameContent = ['STARTED', 'PAUSED', 'STOPPED', 'REVEALED'].includes(gameState.phase)
  const showAnswer = gameState.phase === 'REVEALED'
  const isQcm = gameState.question?.TYPE === 'QCM'
  // QCM answers visible from READY through REVEALED (no re-render on transition)
  const showQcmAnswers = ['READY', 'COUNTDOWN', 'STARTED', 'PAUSED', 'STOPPED', 'REVEALED'].includes(gameState.phase)
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

  return (
    <div className="player-display">
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
        {isShowingScores ? (
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
        ) : (
          /* Game View - 4 vertical zones */
          <motion.div
            key="game"
            className="game-display"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
          >
            {/* PREPARE State - Fixed "Preparez-vous" */}
            {showPrepare && (
              <motion.div
                className="prepare-state"
                initial={{ opacity: 0, scale: 0.8 }}
                animate={{ opacity: 1, scale: 1 }}
              >
                {gameState.question?.CATEGORY && CATEGORIES[gameState.question.CATEGORY] && (
                  <motion.div
                    className="category-display"
                    style={{ backgroundColor: CATEGORIES[gameState.question.CATEGORY].color }}
                    initial={{ opacity: 0, y: -20 }}
                    animate={{ opacity: 1, y: 0 }}
                  >
                    <span className="category-icon">{CATEGORIES[gameState.question.CATEGORY].icon}</span>
                    <span className="category-label">{CATEGORIES[gameState.question.CATEGORY].label}</span>
                  </motion.div>
                )}
                <motion.span
                  className="prepare-emoji"
                  animate={{ rotate: [0, 10, -10, 0] }}
                  transition={{ duration: 1, repeat: Infinity }}
                >
                  🔔
                </motion.span>
                <span className="prepare-text">PREPAREZ-VOUS</span>
              </motion.div>
            )}

            {/* COUNTDOWN State - Big 3... 2... 1... display (non-QCM) */}
            {showCountdown && !isQcm && (
              <motion.div
                className="countdown-state"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
              >
                {gameState.question?.CATEGORY && CATEGORIES[gameState.question.CATEGORY] && (
                  <motion.div
                    className="category-display"
                    style={{ backgroundColor: CATEGORIES[gameState.question.CATEGORY].color }}
                    initial={{ opacity: 0, y: -20 }}
                    animate={{ opacity: 1, y: 0 }}
                  >
                    <span className="category-icon">{CATEGORIES[gameState.question.CATEGORY].icon}</span>
                    <span className="category-label">{CATEGORIES[gameState.question.CATEGORY].label}</span>
                  </motion.div>
                )}
                <motion.div
                  key={gameState.countdownTime}
                  className="countdown-number"
                  initial={{ scale: 2, opacity: 0 }}
                  animate={{ scale: 1, opacity: 1 }}
                  exit={{ scale: 0.5, opacity: 0 }}
                  transition={{ duration: 0.3, ease: 'easeOut' }}
                >
                  {gameState.countdownTime > 0 ? gameState.countdownTime : 'GO!'}
                </motion.div>
              </motion.div>
            )}

            {/* READY State - Non-QCM: centered message */}
            {showReady && !isQcm && (
              <motion.div
                className="ready-state-container"
                initial={{ opacity: 0, scale: 0.8 }}
                animate={{ opacity: 1, scale: 1 }}
              >
                {gameState.question?.CATEGORY && CATEGORIES[gameState.question.CATEGORY] && (
                  <motion.div
                    className="category-display"
                    style={{ backgroundColor: CATEGORIES[gameState.question.CATEGORY].color }}
                    initial={{ opacity: 0, y: -20 }}
                    animate={{ opacity: 1, y: 0 }}
                  >
                    <span className="category-icon">{CATEGORIES[gameState.question.CATEGORY].icon}</span>
                    <span className="category-label">{CATEGORIES[gameState.question.CATEGORY].label}</span>
                  </motion.div>
                )}
                <div className="ready-state">
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
                </div>
              </motion.div>
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

                {/* Zone 2: Question (only visible from STARTED) */}
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
                  ) : (
                    <div className="zone-question-placeholder" />
                  )}
                </div>

                {/* Zone 3: Media or "PREPAREZ-VOUS" message or COUNTDOWN */}
                <div className="zone-media">
                  {showCountdown ? (
                    <motion.div
                      className="countdown-state"
                      initial={{ opacity: 0 }}
                      animate={{ opacity: 1 }}
                    >
                      <motion.div
                        key={gameState.countdownTime}
                        className="countdown-number"
                        initial={{ scale: 2, opacity: 0 }}
                        animate={{ scale: 1, opacity: 1 }}
                        exit={{ scale: 0.5, opacity: 0 }}
                        transition={{ duration: 0.3, ease: 'easeOut' }}
                      >
                        {gameState.countdownTime > 0 ? gameState.countdownTime : 'GO!'}
                      </motion.div>
                    </motion.div>
                  ) : showReady ? (
                    <motion.div
                      className="ready-state"
                      initial={{ opacity: 0, scale: 0.8 }}
                      animate={{ opacity: 1, scale: 1 }}
                      exit={{ opacity: 0, scale: 0.8 }}
                    >
                      {gameState.question?.CATEGORY && CATEGORIES[gameState.question.CATEGORY] && (
                        <motion.div
                          className="category-display"
                          style={{ backgroundColor: CATEGORIES[gameState.question.CATEGORY].color }}
                          initial={{ opacity: 0, y: -20 }}
                          animate={{ opacity: 1, y: 0 }}
                        >
                          <span className="category-icon">{CATEGORIES[gameState.question.CATEGORY].icon}</span>
                          <span className="category-label">{CATEGORIES[gameState.question.CATEGORY].label}</span>
                        </motion.div>
                      )}
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
                      const teamsOnThisAnswer = teamsByQcmAnswer[colorKey] || []
                      const showTeamBadges = ['STOPPED', 'REVEALED'].includes(gameState.phase) && teamsOnThisAnswer.length > 0

                      return (
                        <motion.div
                          key={colorKey}
                          className={`qcm-answer-item ${showAnswer ? (isCorrect ? 'correct' : 'wrong') : ''}`}
                          style={{
                            backgroundColor: showAnswer && !isCorrect ? '#4b5563' : colorData.color,
                            opacity: showAnswer && !isCorrect ? 0.4 : 1
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
                                const isFirst = idx === 0
                                const isLast = idx === totalBadges - 1
                                // Size gradient: 70% (first) to 40% (last) of base size (60px)
                                const maxSize = 60
                                const minSize = 20
                                const sizeRatio = totalBadges > 1
                                  ? 0.70 - (idx / (totalBadges - 1)) * 0.30
                                  : 0.70
                                const badgeSize = Math.round(maxSize * sizeRatio)
                                const finalSize = Math.max(badgeSize, minSize)

                                return (
                                  <motion.div
                                    key={team.name}
                                    className={`qcm-team-badge ${isFirst ? 'first-badge' : ''} ${isLast && totalBadges > 1 ? 'last-badge' : ''}`}
                                    style={{
                                      backgroundColor: team.color
                                        ? (Array.isArray(team.color) ? `rgb(${team.color.join(',')})` : team.color)
                                        : 'var(--gray-400)',
                                      width: `${finalSize}px`,
                                      height: `${finalSize}px`,
                                    }}
                                    initial={{ scale: 0, opacity: 0 }}
                                    animate={{ scale: 1, opacity: 1 }}
                                    transition={{ delay: idx * 0.1 }}
                                    title={team.name}
                                  />
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

            {/* Non-QCM Game Content - 4 vertical zones: Timer, Question, Media, Answers */}
            {!isQcm && showGameContent && gameState.question && (
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

                {/* Zone 3: Media - shows MEDIA_ANSWER during REVEAL if available */}
                {(showAnswer && gameState.question.MEDIA_ANSWER) ? (
                  <motion.div
                    className="zone-media"
                    key="answer-media"
                    initial={{ opacity: 0, scale: 0.9 }}
                    animate={{ opacity: 1, scale: 1 }}
                    transition={{ delay: 0.2 }}
                  >
                    <img
                      src={gameState.question.MEDIA_ANSWER}
                      alt=""
                      className="question-media answer-media-highlight"
                    />
                  </motion.div>
                ) : gameState.question.MEDIA ? (
                  <motion.div
                    className="zone-media"
                    key="question-media"
                    initial={{ opacity: 0, y: 20 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: 0.2 }}
                  >
                    <img
                      src={gameState.question.MEDIA}
                      alt=""
                      className="question-media"
                    />
                  </motion.div>
                ) : (
                  <div className="zone-media-spacer" />
                )}

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

            {/* Waiting State - no question selected */}
            {!gameState.question && ['STOPPED', 'REVEALED'].includes(gameState.phase) && (
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
  )
}
