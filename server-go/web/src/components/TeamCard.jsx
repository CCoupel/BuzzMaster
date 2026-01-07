import { motion } from 'framer-motion'
import './TeamCard.css'

export default function TeamCard({
  name,
  color,
  score = 0,
  ready = false,
  active = false,
  timestamp,
  gameTime,
  buzzers = [],
  onClick,
  className = '',
}) {
  const rgbColor = color ? `rgb(${color.join(',')})` : 'var(--primary-500)'
  const reactionTime = timestamp && gameTime
    ? ((timestamp - gameTime) / 1000000).toFixed(3)
    : null

  return (
    <motion.div
      className={`team-card ${active ? 'active' : ''} ${ready ? 'ready' : ''} ${className}`}
      style={{ '--team-color': rgbColor }}
      onClick={onClick}
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      whileHover={onClick ? { scale: 1.02 } : undefined}
      whileTap={onClick ? { scale: 0.98 } : undefined}
    >
      <div className="team-card-header">
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
      </div>

      <div className="team-card-body">
        <div className="team-score">
          <span className="score-label">Score</span>
          <motion.span
            className="score-value"
            key={score}
            initial={{ scale: 1.5, color: 'var(--accent-green)' }}
            animate={{ scale: 1, color: 'var(--gray-800)' }}
            transition={{ duration: 0.3 }}
          >
            {score}
          </motion.span>
        </div>

        {reactionTime && (
          <motion.div
            className="reaction-time"
            initial={{ opacity: 0, x: 20 }}
            animate={{ opacity: 1, x: 0 }}
          >
            <span className="time-label">Temps</span>
            <span className="time-value">{reactionTime}s</span>
          </motion.div>
        )}
      </div>

      {buzzers.length > 0 && (
        <div className="team-buzzers">
          {buzzers.map((buzzer, index) => (
            <motion.div
              key={buzzer.mac || index}
              className={`buzzer-mini ${buzzer.active ? 'active' : ''} ${buzzer.ready ? 'ready' : ''}`}
              initial={{ opacity: 0, scale: 0.8 }}
              animate={{ opacity: 1, scale: 1 }}
              transition={{ delay: index * 0.1 }}
            >
              <span className="buzzer-name">{buzzer.name}</span>
              {buzzer.timestamp && gameTime && (
                <span className="buzzer-time">
                  {((buzzer.timestamp - gameTime) / 1000000).toFixed(3)}s
                </span>
              )}
            </motion.div>
          ))}
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
