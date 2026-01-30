import { motion } from 'framer-motion'
import { useState, useMemo } from 'react'
import './TeamCard.css'

// QCM answer colors - same as TeamsPage
const ANSWER_COLORS = {
  RED: { label: 'A', color: '#ef4444' },
  GREEN: { label: 'B', color: '#22c55e' },
  YELLOW: { label: 'C', color: '#eab308' },
  BLUE: { label: 'D', color: '#3b82f6' },
}

// Calculate penalty percentage based on hints at buzz time
const getPenaltyPercent = (hintsAtBuzz, penaltyConfig) => {
  if (!penaltyConfig || hintsAtBuzz === 0) return 100
  if (hintsAtBuzz === 1) return Math.round(penaltyConfig.penalty1 * 100)
  if (hintsAtBuzz >= 2) return Math.round(penaltyConfig.penalty2 * 100)
  return 100
}

export default function TeamCard({
  name,
  color,
  score = 0,
  teamPoints = 0,
  ready = false,
  active = false,
  timestamp,
  gameTime,
  gamePhase,
  rank,
  showResponseTime,
  buzzers = [],
  onClick,
  onTeamClick,
  onPlayerClick,
  className = '',
  waitingForReady = false,
  waitingForBuzz = false,
  pointsTarget = null,  // PLAYER or TEAM - from current question
  qcmPenaltyConfig = null, // { penalty1: 0.67, penalty2: 0.33 } - for QCM penalty display
}) {
  const [showTooltip, setShowTooltip] = useState(false)
  const rgbColor = color ? `rgb(${color.join(',')})` : 'var(--primary-500)'
  const reactionTime = timestamp && gameTime
    ? ((timestamp - gameTime) / 1000000).toFixed(3)
    : null

  // Calcul du temps de réponse en ms (feature tri-rapidite)
  const responseTime = timestamp && gameTime
    ? Math.round((timestamp - gameTime) / 1000)
    : null

  // Badge de classement (🏆 🥈 🥉)
  const getRankBadge = (r) => {
    if (r === 1) return '🏆'
    if (r === 2) return '🥈'
    if (r === 3) return '🥉'
    return null
  }

  const rankBadge = rank && showResponseTime ? getRankBadge(rank) : null

  // Tri des joueurs au sein de l'équipe (feature tri-rapidite)
  // Le tri persiste jusqu'à PREPARE (nouvelle question)
  const sortedBuzzers = useMemo(() => {
    if (!['STARTED', 'PAUSED', 'REVEALED', 'STOPPED'].includes(gamePhase)) {
      return buzzers || []
    }

    const buzzed = (buzzers || []).filter(b => (b.timestamp ?? 0) > 0)
    const notBuzzed = (buzzers || []).filter(b => (b.timestamp ?? 0) === 0)

    // Tri stable : trier par timestamp croissant (plus rapide en haut)
    buzzed.sort((a, b) => a.timestamp - b.timestamp)

    return [...buzzed, ...notBuzzed]
  }, [buzzers, gamePhase])

  // Team is waiting when in PREPARE/READY phase but hasn't responded PONG yet
  const isWaiting = waitingForReady && !ready

  // Count ready buzzers for waiting badge
  const readyBuzzersCount = buzzers.filter(b => b.ready).length
  const totalBuzzersCount = buzzers.length

  // Team is waiting for buzz when in STARTED/PAUSED phase and hasn't buzzed yet
  const isWaitingForBuzz = waitingForBuzz && !active

  // Calculate bumper total for tooltip
  const bumperTotal = buzzers.reduce((sum, b) => sum + (b.score || 0), 0)

  // Handle team header click (for team points)
  const handleTeamClick = (e) => {
    e.stopPropagation()
    if (onTeamClick) {
      onTeamClick(name)
    } else if (onClick) {
      onClick()
    }
  }

  return (
    <motion.div
      layoutId={`team-${name}`}
      layout
      className={`team-card ${active ? 'active' : ''} ${ready ? 'ready' : ''} ${isWaiting ? 'waiting' : ''} ${isWaitingForBuzz ? 'waiting-buzz' : ''} ${className}`}
      style={{ '--team-color': rgbColor, zIndex: active ? 10 : 1 }}
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ type: 'spring', stiffness: 300, damping: 30 }}
    >
      <div
        className={`team-card-header ${onTeamClick ? 'clickable' : ''}`}
        onClick={handleTeamClick}
        onMouseEnter={() => setShowTooltip(true)}
        onMouseLeave={() => setShowTooltip(false)}
      >
        <div className="team-color-indicator" />
        <div className="team-header-content">
          {rankBadge && <span className="rank-badge">{rankBadge}</span>}
          <h3 className="team-name">{name}</h3>
        </div>
        {ready && (
          <motion.span
            className="ready-badge"
            initial={{ scale: 0 }}
            animate={{ scale: 1 }}
          >
            PRET
          </motion.span>
        )}
        {isWaiting && (
          <motion.span
            className="waiting-badge"
            initial={{ scale: 0 }}
            animate={{ scale: 1 }}
          >
            {readyBuzzersCount}/{totalBuzzersCount}
          </motion.span>
        )}
        <motion.span
          className="score-value header-score"
          key={score}
          initial={{ scale: 1.5, color: 'var(--accent-green)' }}
          animate={{ scale: 1, color: 'var(--gray-800)' }}
          transition={{ duration: 0.3 }}
        >
          {score} Pts
        </motion.span>

        {/* Score decomposition tooltip */}
        {showTooltip && (teamPoints > 0 || bumperTotal > 0) && (
          <div className="score-tooltip">
            <div className="tooltip-row">
              <span>Equipe:</span>
              <span>{teamPoints} pts</span>
            </div>
            {buzzers.map((b, i) => (
              <div key={b.mac || i} className="tooltip-row">
                <span>+ {b.name}:</span>
                <span>{b.score || 0} pts</span>
              </div>
            ))}
            <div className="tooltip-row tooltip-total">
              <span>= Total:</span>
              <span>{score} pts</span>
            </div>
          </div>
        )}
      </div>

      {reactionTime && (
        <div className="team-card-body">
          <motion.div
            className="reaction-time"
            initial={{ opacity: 0, x: 20 }}
            animate={{ opacity: 1, x: 0 }}
          >
            <span className="time-label">Temps</span>
            <span className="time-value">{reactionTime}s</span>
          </motion.div>
        </div>
      )}

      {sortedBuzzers.length > 0 && (
        <div className="team-buzzers">
          {sortedBuzzers.map((buzzer, index) => {
            const answerColorData = buzzer.answerColor && ANSWER_COLORS[buzzer.answerColor]
            const buzzerWaitingBuzz = waitingForBuzz && !buzzer.active
            const buzzerWaitingPong = waitingForReady  // In PREPARE phase, show PONG status
            const handleBuzzerClick = (e) => {
              e.stopPropagation()
              if (onPlayerClick) {
                onPlayerClick(buzzer.mac, e?.ctrlKey)
              } else if (buzzer.onClick) {
                buzzer.onClick(e)
              }
            }
            // Calculate penalty for this buzzer based on hints at buzz time
            const penaltyPercent = buzzer.active && qcmPenaltyConfig
              ? getPenaltyPercent(buzzer.hintsAtBuzz, qcmPenaltyConfig)
              : 100
            const hasPenalty = penaltyPercent < 100
            return (
              <motion.div
                key={`${buzzer.mac}-${buzzer.timestamp}`}
                layoutId={`buzzer-${buzzer.mac}`}
                layout
                className={`buzzer-mini ${buzzer.active ? 'active' : ''} ${buzzer.ready ? 'ready' : ''} ${answerColorData ? 'has-answer-color' : ''} ${buzzerWaitingBuzz ? 'waiting-buzz' : ''} ${buzzerWaitingPong ? 'waiting-pong' : ''} ${onPlayerClick ? 'clickable' : ''}`}
                style={answerColorData ? { '--answer-color': answerColorData.color } : undefined}
                initial={{ scale: 0.95, opacity: 0.8 }}
                animate={{ scale: 1, opacity: 1 }}
                transition={{ type: 'spring', stiffness: 300, damping: 30 }}
                onClick={handleBuzzerClick}
              >
                <div className="buzzer-info">
                  {answerColorData && (
                    <div className={`buzzer-answer-color-wrapper ${hasPenalty ? 'has-penalty' : ''}`}>
                      <span
                        className="buzzer-answer-color"
                        style={{ backgroundColor: answerColorData.color }}
                      >
                        {answerColorData.label}
                      </span>
                      {hasPenalty && (
                        <svg className="penalty-ring" viewBox="0 0 36 36">
                          <circle
                            className="penalty-ring-bg"
                            cx="18" cy="18" r="16"
                            fill="none"
                            strokeWidth="3"
                          />
                          <circle
                            className="penalty-ring-fill"
                            cx="18" cy="18" r="16"
                            fill="none"
                            strokeWidth="3"
                            strokeDasharray={`${penaltyPercent} ${100 - penaltyPercent}`}
                            strokeDashoffset="25"
                            transform="rotate(-90 18 18)"
                          />
                        </svg>
                      )}
                      {hasPenalty && (
                        <span className="penalty-badge">{penaltyPercent}%</span>
                      )}
                    </div>
                  )}
                  <span className="buzzer-name">{buzzer.name}</span>
                </div>
                <div className="buzzer-right">
                  {buzzer.timestamp && gameTime && (
                    <span className="buzzer-time">
                      {((buzzer.timestamp - gameTime) / 1000000).toFixed(3)}s
                    </span>
                  )}
                  <span className="buzzer-score">{buzzer.score || 0} pts</span>
                </div>
              </motion.div>
            )
          })}
        </div>
      )}

      {active && (
        <motion.div
          className="active-indicator"
          layoutId="activeTeam"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
        />
      )}
    </motion.div>
  )
}
