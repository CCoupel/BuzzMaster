import { motion } from 'framer-motion'
import { useState } from 'react'
import './TeamCard.css'

// QCM answer colors - same as TeamsPage
const ANSWER_COLORS = {
  RED: { label: 'A', color: '#ef4444' },
  GREEN: { label: 'B', color: '#22c55e' },
  YELLOW: { label: 'C', color: '#eab308' },
  BLUE: { label: 'D', color: '#3b82f6' },
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
  buzzers = [],
  onClick,
  onTeamClick,
  onPlayerClick,
  className = '',
  waitingForReady = false,
  waitingForBuzz = false,
  pointsTarget = null,  // PLAYER or TEAM - from current question
}) {
  const [showTooltip, setShowTooltip] = useState(false)
  const rgbColor = color ? `rgb(${color.join(',')})` : 'var(--primary-500)'
  const reactionTime = timestamp && gameTime
    ? ((timestamp - gameTime) / 1000000).toFixed(3)
    : null

  // Team is waiting when in PREPARE/READY phase but hasn't responded PONG yet
  const isWaiting = waitingForReady && !ready

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
      className={`team-card ${active ? 'active' : ''} ${ready ? 'ready' : ''} ${isWaiting ? 'waiting' : ''} ${isWaitingForBuzz ? 'waiting-buzz' : ''} ${className}`}
      style={{ '--team-color': rgbColor }}
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
    >
      <div
        className={`team-card-header ${onTeamClick ? 'clickable' : ''}`}
        onClick={handleTeamClick}
        onMouseEnter={() => setShowTooltip(true)}
        onMouseLeave={() => setShowTooltip(false)}
      >
        <div className="team-color-indicator" />
        <h3 className="team-name">{name}</h3>
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
            ...
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

      {buzzers.length > 0 && (
        <div className="team-buzzers">
          {buzzers.map((buzzer, index) => {
            const answerColorData = buzzer.answerColor && ANSWER_COLORS[buzzer.answerColor]
            const buzzerWaitingBuzz = waitingForBuzz && !buzzer.active
            const handleBuzzerClick = (e) => {
              e.stopPropagation()
              if (onPlayerClick) {
                onPlayerClick(buzzer.mac, e?.ctrlKey)
              } else if (buzzer.onClick) {
                buzzer.onClick(e)
              }
            }
            return (
              <motion.div
                key={buzzer.mac || index}
                className={`buzzer-mini ${buzzer.active ? 'active' : ''} ${buzzer.ready ? 'ready' : ''} ${answerColorData ? 'has-answer-color' : ''} ${buzzerWaitingBuzz ? 'waiting-buzz' : ''} ${onPlayerClick ? 'clickable' : ''}`}
                style={answerColorData ? { '--answer-color': answerColorData.color } : undefined}
                initial={{ opacity: 0, scale: 0.8 }}
                animate={{ opacity: 1, scale: 1 }}
                transition={{ delay: index * 0.1 }}
                onClick={handleBuzzerClick}
              >
                <div className="buzzer-info">
                  {answerColorData && (
                    <span
                      className="buzzer-answer-color"
                      style={{ backgroundColor: answerColorData.color }}
                    >
                      {answerColorData.label}
                    </span>
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
