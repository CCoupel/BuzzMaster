import { motion } from 'framer-motion'
import './Timer.css'

export default function Timer({
  currentTime,
  totalTime,
  phase = 'STOP',
  size = 'md',
  showBar = true,
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

  const isUrgent = currentTime <= 10
  const isCritical = currentTime <= 5
  const isPaused = phase === 'PAUSE'
  const isRunning = phase === 'START'

  return (
    <div className={`timer timer-${size} ${className}`}>
      <motion.div
        className={`timer-display ${isUrgent ? 'urgent' : ''} ${isCritical ? 'critical' : ''} ${isPaused ? 'paused' : ''}`}
        animate={
          isCritical && isRunning
            ? { scale: [1, 1.1, 1] }
            : isPaused
            ? { opacity: [1, 0.5, 1] }
            : {}
        }
        transition={
          isCritical && isRunning
            ? { duration: 0.5, repeat: Infinity }
            : isPaused
            ? { duration: 1, repeat: Infinity }
            : {}
        }
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

      {phase !== 'STOP' && (
        <div className="timer-phase">
          {phase === 'PAUSE' && (
            <motion.span
              className="phase-badge phase-paused"
              initial={{ scale: 0 }}
              animate={{ scale: 1 }}
            >
              PAUSE
            </motion.span>
          )}
          {phase === 'START' && (
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
        </div>
      )}
    </div>
  )
}
