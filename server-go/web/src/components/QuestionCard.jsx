import { motion } from 'framer-motion'
import './QuestionCard.css'

// Export motion for pages that need AnimatePresence
export { motion }

// QCM answer colors
const QCM_COLORS = {
  RED: { label: 'Rouge', color: '#ef4444', letter: 'A' },
  GREEN: { label: 'Vert', color: '#22c55e', letter: 'B' },
  YELLOW: { label: 'Jaune', color: '#eab308', letter: 'C' },
  BLUE: { label: 'Bleu', color: '#3b82f6', letter: 'D' },
}

// Question categories
export const CATEGORIES = {
  GEOGRAPHY: { label: 'Geographie', icon: '🌍', color: '#3b82f6' },
  ENTERTAINMENT: { label: 'Divertissement', icon: '🎭', color: '#ec4899' },
  HISTORY: { label: 'Histoire', icon: '📜', color: '#eab308' },
  ARTS: { label: 'Arts & Litterature', icon: '🎨', color: '#a855f7' },
  SCIENCE: { label: 'Sciences & Nature', icon: '🔬', color: '#22c55e' },
  SPORTS: { label: 'Sports & Loisirs', icon: '⚽', color: '#f97316' },
  FOOD: { label: 'Gastronomie', icon: '🍽️', color: '#991b1b' },
  ANIMALS: { label: 'Animaux', icon: '🐾', color: '#78716c' },
}

/**
 * QuestionCard - Shared component for displaying question cards
 *
 * @param {Object} question - Question data
 * @param {boolean} selected - Is this card selected
 * @param {boolean} draggable - Show drag handle
 * @param {boolean} showDelete - Show delete button
 * @param {boolean} showStatus - Show status badge
 * @param {boolean} showTarget - Show PLAYER/TEAM target badge
 * @param {boolean} compact - Use compact layout (smaller)
 * @param {boolean} canSelect - Can this card be clicked
 * @param {function} onClick - Click handler
 * @param {function} onDelete - Delete handler
 * @param {Object} dragHandlers - Drag event handlers (onDragStart, onDragOver, etc.)
 */
export default function QuestionCard({
  question,
  selected = false,
  draggable = false,
  showDelete = false,
  showStatus = false,
  showTarget = false,
  compact = false,
  canSelect = true,
  onClick,
  onDelete,
  dragHandlers = {},
}) {
  const status = question.STATUS?.toLowerCase() || 'available'
  const isQCM = question.TYPE === 'QCM'
  const isMemory = question.TYPE === 'MEMORY'
  const qcmColor = isQCM && question.QCM_CORRECT ? QCM_COLORS[question.QCM_CORRECT] : null
  const memoryPairCount = isMemory && question.MEMORY_PAIRS ? question.MEMORY_PAIRS.length : 0

  // Calculate max points for Memory questions
  const memoryConfig = isMemory ? (question.MEMORY_CONFIG || {}) : null
  const memoryMaxPoints = isMemory ? (
    memoryPairCount * (memoryConfig.POINTS_PER_PAIR || 10) + (memoryConfig.COMPLETION_BONUS || 0)
  ) : 0
  const memoryPointsPerPair = memoryConfig?.POINTS_PER_PAIR || 10
  const memoryErrorPenalty = memoryConfig?.ERROR_PENALTY || 0

  const handleClick = (e) => {
    if (onClick && canSelect) {
      onClick(question, e.ctrlKey)
    }
  }

  const handleDelete = (e) => {
    e.stopPropagation()
    if (onDelete) {
      onDelete(question.ID)
    }
  }

  const cardContent = (
    <div
      className={`question-card ${status} ${selected ? 'selected' : ''} ${compact ? 'compact' : ''}`}
      onClick={handleClick}
      style={{ cursor: canSelect ? 'pointer' : 'not-allowed' }}
    >
      {/* Header row */}
      <div className="qcard-header">
        {draggable && <span className="qcard-drag-handle">⋮⋮</span>}
        <span className="qcard-id">#{question.ID}</span>

        {question.CATEGORY && CATEGORIES[question.CATEGORY] && (
          <span
            className="qcard-category-badge"
            style={{ backgroundColor: CATEGORIES[question.CATEGORY].color }}
            title={CATEGORIES[question.CATEGORY].label}
          >
            {CATEGORIES[question.CATEGORY].icon}
          </span>
        )}

        {isQCM && <span className="qcard-qcm-badge">QCM</span>}
        {isMemory && <span className="qcard-memory-badge">MEMORY</span>}

        {showTarget && question.POINTS_TARGET && (
          <span
            className={`qcard-target-badge ${question.POINTS_TARGET.toLowerCase()}`}
            title={question.POINTS_TARGET === 'TEAM' ? 'Points equipe' : 'Points joueur'}
          >
            {question.POINTS_TARGET === 'TEAM' ? (
              <svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor">
                <circle cx="9" cy="7" r="4"/>
                <path d="M17 11c1.66 0 2.99-1.34 2.99-3S18.66 5 17 5c-.32 0-.63.05-.91.14.57.81.9 1.79.9 2.86s-.34 2.04-.9 2.86c.28.09.59.14.91.14z"/>
                <path d="M3 18v-1c0-2.66 5.33-4 8-4s8 1.34 8 4v1H3z"/>
                <path d="M17 13c2.05.26 5 1.22 5 3v1h-3v-1.5c0-1.19-.68-2.14-2-2.5z"/>
              </svg>
            ) : (
              <svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor">
                <circle cx="12" cy="7" r="4"/>
                <path d="M12 14c-4 0-8 2-8 4v2h16v-2c0-2-4-4-8-4z"/>
              </svg>
            )}
          </span>
        )}

        <span className="qcard-meta">
          <span className="qcard-time">{question.TIME}s</span>
          <span className="qcard-points">{isMemory ? memoryMaxPoints : question.POINTS}pt</span>
        </span>

        {showStatus && (
          <span className="qcard-status">{question.STATUS || 'AVAILABLE'}</span>
        )}

        {showDelete && (
          <button className="qcard-delete-btn" onClick={handleDelete}>X</button>
        )}
      </div>

      {/* Fixed media zone - for Memory shows config, for others shows images */}
      <div className="qcard-media-zone">
        {isMemory ? (
          <>
            <div className="qcard-memory-config-slot">
              <span className="qcard-memory-config-value">+{memoryPointsPerPair}</span>
              <span className="qcard-memory-config-label">/ paire</span>
            </div>
            <div className={`qcard-memory-config-slot ${memoryErrorPenalty > 0 ? 'penalty' : 'no-penalty'}`}>
              <span className="qcard-memory-config-value">{memoryErrorPenalty > 0 ? `-${memoryErrorPenalty}` : '0'}</span>
              <span className="qcard-memory-config-label">/ erreur</span>
            </div>
          </>
        ) : (
          <>
            <div className={`qcard-media-slot ${question.MEDIA ? 'has-media' : 'empty'}`}>
              {question.MEDIA ? (
                <img src={question.MEDIA} alt="" />
              ) : (
                <span className="qcard-media-placeholder">📷</span>
              )}
            </div>
            <div className={`qcard-media-slot answer-slot ${question.MEDIA_ANSWER ? 'has-media' : 'empty'}`}>
              {question.MEDIA_ANSWER ? (
                <img src={question.MEDIA_ANSWER} alt="" />
              ) : (
                <span className="qcard-media-placeholder">✓</span>
              )}
            </div>
          </>
        )}
      </div>

      {/* Fixed question zone */}
      <div className="qcard-question-zone">
        <p className="qcard-question">{question.QUESTION}</p>
      </div>

      {/* Fixed answer zone */}
      <div className="qcard-answer-zone">
        {isMemory ? (
          <p className="qcard-answer qcard-answer-memory">
            <span className="qcard-memory-icon">🎴</span>
            {memoryPairCount} paires
          </p>
        ) : qcmColor ? (
          <p className="qcard-answer qcard-answer-qcm" style={{ backgroundColor: qcmColor.color }}>
            <span className="qcard-qcm-letter">{qcmColor.letter}</span>
            {question.ANSWER}
          </p>
        ) : (
          <p className="qcard-answer">{question.ANSWER}</p>
        )}
      </div>
    </div>
  )

  // Wrap with motion.div if we have drag handlers or need animation
  if (draggable && Object.keys(dragHandlers).length > 0) {
    return (
      <motion.div
        className={`qcard-wrapper ${dragHandlers.isDragging ? 'dragging' : ''} ${dragHandlers.isDragOver ? 'drag-over' : ''}`}
        draggable
        onDragStart={dragHandlers.onDragStart}
        onDragOver={dragHandlers.onDragOver}
        onDragLeave={dragHandlers.onDragLeave}
        onDragEnd={dragHandlers.onDragEnd}
        onDrop={dragHandlers.onDrop}
        initial={{ opacity: 0, scale: 0.9 }}
        animate={{ opacity: 1, scale: 1 }}
        exit={{ opacity: 0, scale: 0.9 }}
        transition={{ delay: dragHandlers.index * 0.05 }}
      >
        {cardContent}
      </motion.div>
    )
  }

  // Simple motion wrapper for GamePage
  return (
    <motion.div
      className="qcard-wrapper"
      whileHover={canSelect ? { scale: 1.01 } : undefined}
      whileTap={canSelect ? { scale: 0.99 } : undefined}
    >
      {cardContent}
    </motion.div>
  )
}
