import { motion } from 'framer-motion'
import CategoryBadge from './CategoryBadge'
import { getQuestionTypeMeta } from '../utils/questionTypeMeta'
import { effectiveRafaleCategories, effectiveRafaleDifficulties } from '../utils/rafaleEffective'
import './QuestionCard.css'

// Export motion for pages that need AnimatePresence
export { motion }

// Re-export from utils for backward compat
export { CATEGORIES, categoryMeta } from '../utils/categoryUtils'

// QCM answer colors
const QCM_COLORS = {
  RED: { label: 'Rouge', color: '#ef4444', letter: 'A' },
  GREEN: { label: 'Vert', color: '#22c55e', letter: 'B' },
  YELLOW: { label: 'Jaune', color: '#eab308', letter: 'C' },
  BLUE: { label: 'Bleu', color: '#3b82f6', letter: 'D' },
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
  customCategories = [],
  onClick,
  onDelete,
  dragHandlers = {},
}) {
  const status = question.STATUS?.toLowerCase() || 'available'
  const isQCM = question.TYPE === 'QCM'
  const isMemory = question.TYPE === 'MEMORY'
  const isMemotion = question.TYPE === 'MEMOTION'
  const isArdoise = question.TYPE === 'ARDOISE'
  const isRafale = question.TYPE === 'RAFALE'
  // #216 (réouverture assumée de #107, maquette rafale-multi-216.html §01) —
  // le bug d'affichage historique venait d'une carte lisant un champ CATEGORY
  // unique alors que RAFALE en admet plusieurs : résolu ici par des chips
  // multiples (une par catégorie ET par difficulté), jamais par une branche
  // qui suppose une seule valeur. utils/rafaleEffective.js applique la même
  // rétro-compatibilité mono → liste que le moteur (contrat §3.3).
  const rafaleCategories = isRafale ? effectiveRafaleCategories(question) : []
  const rafaleDifficulties = isRafale ? effectiveRafaleDifficulties(question) : []
  // Source unique des couleurs de badge de type (questionTypeMeta.js,
  // #183/A-F2) — remplace la liste CSS `.qcard-type-badge.type-*`
  // (QuestionCard.css) qui divergeait silencieusement du sélecteur de type
  // (QuestionsPage.jsx `.type-btn.*`) : RAFALE avait une entrée dans
  // questionTypeMeta.js/le sélecteur mais AUCUNE règle CSS ici, laissant le
  // badge sans fond (texte blanc flottant sur le fond ambiant de la carte).
  // Un seul style calculé depuis la même source pour les deux surfaces —
  // ne peut plus diverger pour ce type ni pour un futur type.
  const typeMeta = getQuestionTypeMeta(question.TYPE)
  const qcmColor = isQCM && question.QCM_CORRECT ? QCM_COLORS[question.QCM_CORRECT] : null
  const memoryPairCount = isMemory && question.MEMORY_PAIRS ? question.MEMORY_PAIRS.length : 0

  // Calculate max points for Memory questions
  const memoryConfig = isMemory ? (question.MEMORY_CONFIG || {}) : null
  const memoryMaxPoints = isMemory ? (
    memoryPairCount * (memoryConfig.POINTS_PER_PAIR || 10) + (memoryConfig.COMPLETION_BONUS || 0)
  ) : 0
  const memoryPointsPerPair = memoryConfig?.POINTS_PER_PAIR || 10
  const memoryErrorPenalty = memoryConfig?.ERROR_PENALTY || 0

  // Calculate max points for MEMOTION questions (difficulty: 1→1pt, 2→3pt, 3→5pt)
  const motionCards = isMemotion && question.MOTION_CARDS ? question.MOTION_CARDS : []
  const motionCardCount = motionCards.length
  const motionMaxPoints = motionCards.reduce((sum, c) => {
    const d = c.DIFFICULTY || 1
    return sum + (d === 3 ? 5 : d === 2 ? 3 : 1)
  }, 0)
  const motionMode = isMemotion ? (question.MOTION_MODE || 'SOLO') : null

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
      {/* Header row 1: Question name + status + delete button */}
      <div className="qcard-header-row1">
        {draggable && <span className="qcard-drag-handle">⋮⋮</span>}
        <span className="qcard-name">#{question.ID} - {question.QUESTION?.substring(0, 40)}{question.QUESTION?.length > 40 ? '...' : ''}</span>
        {showStatus && (
          <span className="qcard-status">{question.STATUS || 'AVAILABLE'}</span>
        )}
        {showDelete && (
          <button className="qcard-delete-btn" onClick={handleDelete}>X</button>
        )}
      </div>

      {/* Header row 2: Category, type, target, time, points */}
      <div className="qcard-header-row2">
        {/* #216 — RAFALE affiche autant de chips que de catégories ET de
            difficultés sélectionnées (pas un badge unique) ; tous les autres
            types gardent le badge de catégorie générique unique, inchangé. */}
        {isRafale ? (
          <>
            {rafaleCategories.map(cat => (
              <CategoryBadge key={cat} catKey={cat} customCategories={customCategories} size="sm" />
            ))}
            {rafaleDifficulties.map(d => (
              <span key={d} className="qcard-rafale-diff-chip">{'★'.repeat(d)}</span>
            ))}
          </>
        ) : (
          question.CATEGORY && (
            <CategoryBadge catKey={question.CATEGORY} customCategories={customCategories} size="sm" />
          )
        )}

        <span className="qcard-type-badge" style={{ backgroundColor: typeMeta.color }}>
          {typeMeta.label}
        </span>

        <span
          className={`qcard-target-badge ${(question.POINTS_TARGET || 'PLAYER').toLowerCase()}`}
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

        <span className="qcard-meta">
          <span className="qcard-time">{question.TIME}s</span>
          <span className="qcard-points">
            {isMemory ? memoryMaxPoints : isMemotion ? motionMaxPoints : question.POINTS}pt
          </span>
        </span>
      </div>

      {/* Fixed media zone - for Memory shows config, for Memotion shows card count, for others shows images */}
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
        ) : isMemotion ? (
          <>
            <div className="qcard-memory-config-slot">
              <span className="qcard-memory-config-value">{motionCardCount}</span>
              <span className="qcard-memory-config-label">cartes</span>
            </div>
            <div className="qcard-memory-config-slot">
              <span className="qcard-memory-config-value" style={{ fontSize: '0.65rem' }}>
                {motionMode === 'TANT_QUE_JE_GAGNE' ? 'TQJG' : motionMode === 'CHACUN_SON_TOUR' ? 'CST' : 'SOLO'}
              </span>
              <span className="qcard-memory-config-label">mode</span>
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
        ) : isMemotion ? (
          <p className="qcard-answer qcard-answer-memory">
            <span className="qcard-memory-icon">🃏</span>
            {motionCardCount} cartes · {motionMaxPoints}pts max
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
        id={`qcard-${question.ID}`}
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
