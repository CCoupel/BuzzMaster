import { motion } from 'framer-motion'
import './Timer.css'

export default function Timer({
  currentTime,
  totalTime,
  phase = 'STOP',
  size = 'md',
  showBar = true,
  showPhase = true,
  className = '',
}) {
  const percentage = totalTime > 0 ? (currentTime / totalTime) * 100 : 100
  const minutes = Math.floor(currentTime / 60)
  const seconds = currentTime % 60
  const timeDisplay = `${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`

  const getBarColor = () => {
    if (percentage > 50) return 'var(--success)'
    if (percentage > 25) return 'var(--warning)'
    return 'var(--error)'
  }

  const isUrgent = currentTime <= 10 && currentTime > 5
  const isCritical = currentTime <= 5
  const isPaused = phase === 'PAUSED'
  const isRunning = phase === 'STARTED'

  // Animation intensity increases as time decreases
  const getAnimation = () => {
    if (isPaused) {
      return { opacity: [1, 0.4, 1] }
    }
    if (isCritical && isRunning) {
      // Very intense pulsation at 5s or less
      return {
        scale: [1, 1.25, 1],
        textShadow: [
          '0 0 10px rgba(239, 68, 68, 0.5)',
          '0 0 40px rgba(239, 68, 68, 1)',
          '0 0 10px rgba(239, 68, 68, 0.5)'
        ]
      }
    }
    if (isUrgent && isRunning) {
      // Moderate pulsation at 10s
      return {
        scale: [1, 1.15, 1],
        textShadow: [
          '0 0 5px rgba(234, 179, 8, 0.3)',
          '0 0 25px rgba(234, 179, 8, 0.8)',
          '0 0 5px rgba(234, 179, 8, 0.3)'
        ]
      }
    }
    return {}
  }

  const getTransition = () => {
    if (isPaused) {
      return { duration: 1, repeat: Infinity, ease: 'easeInOut' }
    }
    if (isCritical && isRunning) {
      return { duration: 0.3, repeat: Infinity, ease: 'easeInOut' }
    }
    if (isUrgent && isRunning) {
      return { duration: 0.5, repeat: Infinity, ease: 'easeInOut' }
    }
    return {}
  }

  return (
    <div className={`timer timer-${size} ${className}`}>
      <motion.div
        className={`timer-display ${isUrgent ? 'urgent' : ''} ${isCritical ? 'critical' : ''} ${isPaused ? 'paused' : ''}`}
        animate={getAnimation()}
        transition={getTransition()}
      >
        {timeDisplay}
      </motion.div>

      {showBar && (
        <div className="timer-bar-container">
          <motion.div
            className={`timer-bar ${isUrgent ? 'urgent' : ''} ${isPaused ? 'paused' : ''}`}
            initial={false}
            animate={{ width: `${percentage}%` }}
            transition={{ duration: 0.2 }}
            style={{ backgroundColor: getBarColor() }}
          />
        </div>
      )}

      {showPhase && (
        <div className="timer-phase">
          {phase === 'STOPPED' && (
            <motion.span
              className="phase-badge phase-stopped"
              initial={{ scale: 0 }}
              animate={{ scale: 1 }}
            >
              ARRET
            </motion.span>
          )}
          {phase === 'PAUSED' && (
            <motion.span
              className="phase-badge phase-paused"
              initial={{ scale: 0 }}
              animate={{ scale: 1 }}
            >
              PAUSE
            </motion.span>
          )}
          {phase === 'STARTED' && (
            <motion.span
              className="phase-badge phase-running"
              initial={{ scale: 0 }}
              animate={{ scale: 1 }}
            >
              EN COURS
            </motion.span>
          )}
          {phase === 'PREPARE' && (
            <motion.span
              className="phase-badge phase-prepare"
              initial={{ scale: 0 }}
              animate={{ scale: 1 }}
            >
              PREPARATION
            </motion.span>
          )}
          {phase === 'READY' && (
            <motion.span
              className="phase-badge phase-ready"
              initial={{ scale: 0 }}
              animate={{ scale: 1 }}
            >
              PRET
            </motion.span>
          )}
          {phase === 'REVEALED' && (
            <motion.span
              className="phase-badge phase-revealed"
              initial={{ scale: 0 }}
              animate={{ scale: 1 }}
            >
              REPONSE
            </motion.span>
          )}
        </div>
      )}
    </div>
  )
}
