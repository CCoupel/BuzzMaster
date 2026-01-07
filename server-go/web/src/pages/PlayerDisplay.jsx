import { useEffect, useMemo, useState, useRef } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import confetti from 'canvas-confetti'
import { useGame } from '../hooks/GameContext'
import Timer from '../components/Timer'
import Podium from '../components/Podium'
import './PlayerDisplay.css'

const DEFAULT_DURATION = 10 // seconds

export default function PlayerDisplay() {
  const { gameState, teams, bumpers } = useGame()
  const [showAnswer, setShowAnswer] = useState(false)
  const [lastWinner, setLastWinner] = useState(null)
  const [currentBgIndex, setCurrentBgIndex] = useState(0)
  const timerRef = useRef(null)
  const [previousRanking, setPreviousRanking] = useState({})
  const [changedTeams, setChangedTeams] = useState({})

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
        // Same score as previous team, use same rank
        return { ...team, rank: lastRank }
      }
      // New score, assign new rank
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
        // Same score as previous player, use same rank
        return { ...player, rank: lastRank }
      }
      // New score, assign new rank
      currentRank = index + 1
      lastScore = player.score
      lastRank = currentRank
      return { ...player, rank: currentRank }
    })
  }, [bumpers, teams])

  // Detect ranking changes
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

  // Find winning bumper (first to press)
  const winningBumper = useMemo(() => {
    let earliest = { timestamp: Infinity, data: null }
    Object.entries(bumpers).forEach(([mac, bumper]) => {
      if (bumper.TIMESTAMP && bumper.TIMESTAMP < earliest.timestamp) {
        earliest = {
          timestamp: bumper.TIMESTAMP,
          data: { mac, ...bumper, teamColor: teams[bumper.TEAM]?.COLOR }
        }
      }
    })
    return earliest.data
  }, [bumpers, teams])

  // Show answer on STOP phase after START
  useEffect(() => {
    if (gameState.phase === 'STOP') {
      // Could trigger answer reveal here
    } else {
      setShowAnswer(false)
    }
  }, [gameState.phase])

  // Celebration when points are scored
  useEffect(() => {
    if (winningBumper && gameState.phase === 'STOP' && winningBumper !== lastWinner) {
      setLastWinner(winningBumper)
      triggerCelebration(winningBumper.teamColor)
    }
  }, [winningBumper, gameState.phase])

  // Background cycling with per-image duration
  useEffect(() => {
    const backgrounds = gameState.backgrounds || []
    if (backgrounds.length <= 1) return

    if (timerRef.current) {
      clearTimeout(timerRef.current)
    }

    const currentBg = backgrounds[currentBgIndex]
    const duration = (currentBg?.duration || DEFAULT_DURATION) * 1000

    timerRef.current = setTimeout(() => {
      setCurrentBgIndex(prev => (prev + 1) % backgrounds.length)
    }, duration)

    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current)
      }
    }
  }, [gameState.backgrounds, currentBgIndex])

  // Reset index when backgrounds change
  useEffect(() => {
    const backgrounds = gameState.backgrounds || []
    if (currentBgIndex >= backgrounds.length) {
      setCurrentBgIndex(0)
    }
  }, [gameState.backgrounds, currentBgIndex])

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

  // Current background
  const backgrounds = gameState.backgrounds || []
  const currentBg = backgrounds.length > 0 ? backgrounds[currentBgIndex] : null
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

      {/* Timer Bar - only show during game */}
      {isShowingGame && (
        <div className="display-timer-bar">
          <Timer
            currentTime={gameState.timer}
            totalTime={gameState.totalTime}
            phase={gameState.phase}
            size="xl"
          />
        </div>
      )}

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
              {/* Podium - main focus */}
              {sortedTeams.length >= 1 && (
                <div className="scores-podium-section">
                  <Podium teams={sortedTeams} changedTeams={changedTeams} />
                </div>
              )}

              {/* Compact scores list */}
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
              {/* Podium - main focus */}
              {topPlayers.length >= 1 && (
                <div className="players-podium-section">
                  <Podium teams={topPlayers} changedTeams={{}} />
                </div>
              )}

              {/* Players list */}
              <div className="players-list-section">
                <div className={`players-list ${useTwoColumns ? 'two-columns' : ''}`}>
                  <AnimatePresence mode="popLayout">
                    {sortedPlayers.map((player, index) => {
                      const rgbColor = getRgbColor(player.teamColor)
                      const barWidth = (player.score / maxPlayerScore) * 100
                      const isTied = sortedPlayers.filter(p => p.rank === player.rank).length > 1

                      return (
                        <motion.div
                          key={player.mac}
                          className="player-row"
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
                            <span className="player-score">{player.score} pts</span>
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
          /* Game View */
          <motion.div
            key="game"
            className="game-display"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
          >
            {/* Question Content */}
            {gameState.question && (
              <div className="question-display">
                {gameState.question.MEDIA && (
                  <motion.div
                    className="question-media-container"
                    initial={{ opacity: 0, y: 20 }}
                    animate={{ opacity: 1, y: 0 }}
                  >
                    <img
                      src={gameState.question.MEDIA}
                      alt=""
                      className="question-media"
                    />
                  </motion.div>
                )}

                <motion.div
                  className="question-text-container"
                  initial={{ opacity: 0, y: 20 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: 0.2 }}
                >
                  <p className="question-text">{gameState.question.QUESTION}</p>
                </motion.div>

                <AnimatePresence>
                  {showAnswer && (
                    <motion.div
                      className="answer-container"
                      initial={{ opacity: 0, scale: 0.8, y: 20 }}
                      animate={{ opacity: 1, scale: 1, y: 0 }}
                      exit={{ opacity: 0, scale: 0.8 }}
                    >
                      <p className="answer-text">{gameState.question.ANSWER}</p>
                    </motion.div>
                  )}
                </AnimatePresence>
              </div>
            )}

            {/* Winner Display */}
            <AnimatePresence>
              {winningBumper && gameState.phase === 'STOP' && (
                <motion.div
                  className="winner-display"
                  initial={{ opacity: 0, scale: 0.5, y: 100 }}
                  animate={{ opacity: 1, scale: 1, y: 0 }}
                  exit={{ opacity: 0, scale: 0.5 }}
                  style={{
                    '--team-color': winningBumper.teamColor
                      ? `rgb(${winningBumper.teamColor.join(',')})`
                      : 'var(--primary-500)'
                  }}
                >
                  <span className="winner-emoji">🎉</span>
                  <span className="winner-name">{winningBumper.NAME}</span>
                  <span className="winner-team">{winningBumper.TEAM}</span>
                  <span className="winner-points">+{gameState.question?.POINTS || 1} pts</span>
                </motion.div>
              )}
            </AnimatePresence>

            {/* Waiting State */}
            {!gameState.question && gameState.phase === 'STOP' && (
              <motion.div
                className="waiting-state"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
              >
                <span className="waiting-emoji">🎮</span>
                <span className="waiting-text">En attente de la prochaine question...</span>
              </motion.div>
            )}

            {/* Prepare State */}
            {gameState.phase === 'PREPARE' && (
              <motion.div
                className="prepare-state"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
              >
                <motion.span
                  className="prepare-emoji"
                  animate={{ rotate: [0, 10, -10, 0] }}
                  transition={{ duration: 1, repeat: Infinity }}
                >
                  🔔
                </motion.span>
                <span className="prepare-text">Preparez vos buzzers !</span>
              </motion.div>
            )}

            {/* Ready State */}
            {gameState.phase === 'READY' && (
              <motion.div
                className="ready-state"
                initial={{ opacity: 0, scale: 0.8 }}
                animate={{ opacity: 1, scale: 1 }}
              >
                <motion.span
                  className="ready-emoji"
                  animate={{ scale: [1, 1.2, 1] }}
                  transition={{ duration: 0.5, repeat: Infinity }}
                >
                  ✋
                </motion.span>
                <span className="ready-text">PRETS ?</span>
              </motion.div>
            )}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}
