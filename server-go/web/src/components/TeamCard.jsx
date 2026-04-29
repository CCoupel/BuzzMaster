import { motion } from 'framer-motion'
import { useState, useMemo, useEffect } from 'react'
import { boostTeamColor } from '../utils/colorUtils'
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

// OTA Update Modal - shown when clicking a red (outdated) firmware version
export function OtaModal({ buzzer, onClose }) {
  const [targetVersion, setTargetVersion] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)

  // Fetch the reference firmware version when modal opens
  useEffect(() => {
    fetch('/api/firmware/buzzclick/version')
      .then(r => r.json())
      .then(data => setTargetVersion(data))
      .catch(() => setError('Impossible de recuperer la version de reference'))
  }, [])

  const handleLaunchOta = async () => {
    setLoading(true)
    setError(null)
    try {
      const mac = encodeURIComponent(buzzer.mac)
      const res = await fetch(`/api/buzzer/${mac}/update`, { method: 'POST' })
      const data = await res.json()
      if (data.status !== 'ok') {
        setError(data.message || 'Erreur lors du lancement de la mise a jour')
        setLoading(false)
      }
      // Modal stays open to show live OTA_STATUS progress via props
    } catch (err) {
      setError('Erreur reseau: ' + err.message)
      setLoading(false)
    }
  }


  const isInProgress = buzzer.otaStatus === 'downloading' || buzzer.otaStatus === 'flashing'
  const isDone = buzzer.otaStatus === 'done'
  const isError = buzzer.otaStatus === 'error'
  const hasProgress = isInProgress || isDone

  // Determine progress bar percentage and phase
  const percent = isDone ? 100 : (buzzer.otaPercent || 0)
  const phaseClass = isDone ? 'done' : (buzzer.otaStatus || 'downloading')
  // Use indeterminate animation if server sends 0% (buzzer doesn't report percent yet)
  const indeterminate = isInProgress && percent === 0

  const phaseLabel = {
    downloading: 'Téléchargement',
    flashing: 'Flashage',
    done: 'Terminé',
    error: 'Erreur OTA',
  }[buzzer.otaStatus] || ''

  return (
    <div className="ota-modal-overlay" onClick={onClose}>
      <div className="ota-modal" onClick={e => e.stopPropagation()}>
        <div className="ota-modal-header">
          <h3>Mise a jour firmware</h3>
          <button className="ota-modal-close" onClick={onClose} aria-label="Fermer">x</button>
        </div>

        <div className="ota-modal-body">
          <div className="ota-info-row">
            <span className="ota-info-label">Buzzer</span>
            <span className="ota-info-value">{buzzer.name}</span>
          </div>
          <div className="ota-info-row">
            <span className="ota-info-label">MAC</span>
            <span className="ota-info-value ota-mac">{buzzer.mac}</span>
          </div>
          <div className="ota-info-row">
            <span className="ota-info-label">Version actuelle</span>
            <span className="ota-info-value ota-version-current">{buzzer.firmwareVersion || '?'}</span>
          </div>
          <div className="ota-info-row">
            <span className="ota-info-label">Version cible</span>
            <span className="ota-info-value ota-version-target">
              {targetVersion ? targetVersion.VERSION : '...'}
            </span>
          </div>

          {/* Progress bar - shown during and after OTA */}
          {hasProgress && (
            <div className="ota-progress-container">
              <div className="ota-progress-label">
                <span className={`ota-progress-phase ${phaseClass}`}>{phaseLabel}</span>
                {!indeterminate && <span>{percent}%</span>}
              </div>
              <div className="ota-progress-track">
                <div
                  className={`ota-progress-fill ${phaseClass}${indeterminate ? ' indeterminate' : ''}`}
                  style={indeterminate ? undefined : { width: `${percent}%` }}
                />
              </div>
            </div>
          )}

          {/* Error states */}
          {isError && (
            <div className="ota-status ota-status-error">Erreur OTA</div>
          )}
          {error && (
            <div className="ota-status ota-status-error">{error}</div>
          )}
        </div>

        <div className="ota-modal-footer">
          <button
            className="ota-btn ota-btn-secondary"
            onClick={onClose}
            disabled={isInProgress}
          >
            Annuler
          </button>
          <button
            className="ota-btn ota-btn-primary"
            onClick={handleLaunchOta}
            disabled={loading || isInProgress || isDone || !targetVersion?.EXISTS}
          >
            {isInProgress ? (
              <><span className="ota-spinner" /> En cours...</>
            ) : isDone ? (
              'Termine'
            ) : (
              'Lancer la mise a jour OTA'
            )}
          </button>
        </div>
      </div>
    </div>
  )
}

// OTA Update All Modal - shown when clicking "Mettre à jour tous les buzzers obsolètes"
// bumpers: full bumpers object from useGame() (keyed by MAC)
export function OtaAllModal({ bumpers: allBumpers, onClose }) {
  const [targetVersion, setTargetVersion] = useState(null)
  const [launched, setLaunched] = useState(false)
  const [error, setError] = useState(null)
  // Local snapshot: once a buzzer reaches done/error, keep its final status/percent
  // even if the server resets OTA_STATUS after the buzzer reboots.
  const [doneSnapshot, setDoneSnapshot] = useState({}) // mac → { status, percent }
  // Frozen list of buzzers at launch time: prevents bars from disappearing when
  // IS_OUTDATED flips to false after a successful OTA + reboot.
  const [frozenEntries, setFrozenEntries] = useState(null) // [[mac, {NAME, FIRMWARE_VERSION}]]

  // Fetch reference firmware version
  useEffect(() => {
    fetch('/api/firmware/buzzclick/version')
      .then(r => r.json())
      .then(data => setTargetVersion(data))
      .catch(() => setError('Impossible de recuperer la version de reference'))
  }, [])

  // Pre-launch: live list of outdated physical buzzers.
  const outdatedEntries = useMemo(
    () => Object.entries(allBumpers).filter(([, b]) => b.IS_OUTDATED && b.FIRMWARE_VERSION),
    [allBumpers]
  )

  // Post-launch: frozen list (name/version from launch time) + live OTA_STATUS/OTA_PERCENT.
  // Using frozenEntries ensures bars stay visible even when IS_OUTDATED → false after reboot.
  const workingEntries = useMemo(() => {
    if (!frozenEntries) return outdatedEntries
    return frozenEntries.map(([mac, frozen]) => {
      const live = allBumpers[mac] || {}
      return [mac, { ...frozen, OTA_STATUS: live.OTA_STATUS, OTA_PERCENT: live.OTA_PERCENT }]
    })
  }, [frozenEntries, allBumpers, outdatedEntries])

  // Snapshot buzzer progress the moment it reaches done/error.
  // After the buzzer reboots, the server clears OTA_STATUS — the snapshot
  // preserves the completed state so the progress bar stays at 100%.
  useEffect(() => {
    if (!launched) return
    workingEntries.forEach(([mac, b]) => {
      const terminal = b.OTA_STATUS === 'done' || b.OTA_STATUS === 'error'
      if (terminal && !doneSnapshot[mac]) {
        setDoneSnapshot(prev => ({
          ...prev,
          [mac]: { status: b.OTA_STATUS, percent: b.OTA_STATUS === 'done' ? 100 : (b.OTA_PERCENT || 0) },
        }))
      }
    })
  }, [launched, workingEntries, doneSnapshot])

  const handleLaunch = async () => {
    setError(null)
    // Freeze the current outdated list before launching so bars persist after reboot
    setFrozenEntries(
      Object.entries(allBumpers)
        .filter(([, b]) => b.IS_OUTDATED && b.FIRMWARE_VERSION)
        .map(([mac, b]) => [mac, { NAME: b.NAME, FIRMWARE_VERSION: b.FIRMWARE_VERSION }])
    )
    setLaunched(true)
    try {
      const res = await fetch('/api/buzzer/update-all', { method: 'POST' })
      const data = await res.json()
      if (!res.ok || data.status !== 'ok') {
        setError(data.message || 'Erreur lors du lancement')
      }
    } catch (err) {
      setError('Erreur reseau: ' + err.message)
    }
  }

  // Global progress: average OTA_PERCENT across all buzzers (each capped at 100 when done/error)
  // Uses doneSnapshot so buzzers that rebooted (clearing OTA_STATUS) still count as 100%.
  const totalCount = workingEntries.length
  const finishedCount = workingEntries.filter(([mac, b]) => {
    const status = doneSnapshot[mac]?.status || b.OTA_STATUS
    return status === 'done' || status === 'error'
  }).length
  const globalPercent = totalCount > 0
    ? Math.round(
        workingEntries.reduce((sum, [mac, b]) => {
          const snap = doneSnapshot[mac]
          const status = snap ? snap.status : b.OTA_STATUS
          const pct = status === 'done' || status === 'error'
            ? 100
            : (b.OTA_PERCENT || 0)
          return sum + pct
        }, 0) / totalCount
      )
    : 0
  const allFinished = launched && totalCount > 0 && finishedCount === totalCount
  const allSuccess = allFinished && workingEntries.every(([mac, b]) => {
    const status = doneSnapshot[mac]?.status || b.OTA_STATUS
    return status === 'done'
  })
  // allConfirmed: all transfers done AND all buzzers rebooted with correct version
  const allConfirmed = allSuccess && workingEntries.every(([mac]) =>
    allBumpers[mac] && !allBumpers[mac].IS_OUTDATED
  )


  return (
    <div className="ota-modal-overlay" onClick={onClose}>
      <div className="ota-modal ota-all-modal" onClick={e => e.stopPropagation()}>
        <div className="ota-modal-header">
          <h3>Mise a jour de tous les buzzers</h3>
          <button className="ota-modal-close" onClick={onClose} aria-label="Fermer">×</button>
        </div>

        <div className="ota-modal-body">
          {/* Target version info */}
          <div className="ota-info-row">
            <span className="ota-info-label">Version cible</span>
            <span className="ota-info-value ota-version-target">
              {targetVersion ? targetVersion.VERSION : '...'}
            </span>
          </div>

          {/* Global progress bar */}
          {launched && (
            <div className="ota-progress-container ota-global-progress">
              <div className="ota-progress-label">
                <span className={`ota-progress-phase ${allConfirmed ? 'confirmed' : 'downloading'}`}>
                  {allConfirmed ? 'Tous confirmés' : 'Progression globale'}
                </span>
                <span>{globalPercent}%</span>
              </div>
              <div className="ota-progress-track">
                <div
                  className={`ota-progress-fill ${allConfirmed ? 'confirmed' : 'downloading'}`}
                  style={{ width: `${globalPercent}%` }}
                />
              </div>
            </div>
          )}

          {/* Per-buzzer list */}
          <div className="ota-all-list">
            {workingEntries.map(([mac, b]) => {
              // Use snapshot if buzzer already completed (server may have reset OTA_STATUS after reboot)
              const snap = doneSnapshot[mac]
              const otaStatus = snap ? snap.status : (b.OTA_STATUS || '')
              const otaPercent = snap ? snap.percent : (b.OTA_PERCENT || 0)
              const isDone = otaStatus === 'done'
              const isError = otaStatus === 'error'
              // confirmed: transfer done AND buzzer rebooted with correct version (IS_OUTDATED=false)
              const isConfirmed = isDone && allBumpers[mac] && !allBumpers[mac].IS_OUTDATED
              const isInProgress = otaStatus === 'downloading' || otaStatus === 'flashing'
              const percent = isDone ? 100 : otaPercent
              const phaseClass = isConfirmed ? 'confirmed' : isDone ? 'done' : isError ? 'error' : (otaStatus || '')
              const indeterminate = isInProgress && percent === 0

              return (
                <div key={mac} className="ota-all-row">
                  <div className="ota-all-row-header">
                    <span className="ota-all-name">{b.NAME || mac}</span>
                    <div className="ota-all-row-right">
                      <span className="ota-all-version">{b.FIRMWARE_VERSION}</span>
                      {launched && (
                        <span className={`ota-all-status-badge ${phaseClass}`}>
                          {isDone ? '✓' : isError ? '✗' : isInProgress ? `${percent > 0 ? percent + '%' : '…'}` : '—'}
                        </span>
                      )}
                    </div>
                  </div>
                  {launched && (
                    <div className="ota-progress-track ota-progress-track-small">
                      <div
                        className={`ota-progress-fill ${phaseClass}${indeterminate ? ' indeterminate' : ''}`}
                        style={indeterminate ? undefined : { width: `${percent}%` }}
                      />
                    </div>
                  )}
                </div>
              )
            })}
          </div>

          {error && <div className="ota-status ota-status-error">{error}</div>}
        </div>

        <div className="ota-modal-footer">
          {!launched ? (
            <>
              <button className="ota-btn ota-btn-secondary" onClick={onClose}>Annuler</button>
              <button
                className="ota-btn ota-btn-primary"
                onClick={handleLaunch}
                disabled={totalCount === 0 || !targetVersion?.EXISTS}
              >
                Lancer ({totalCount} buzzer{totalCount > 1 ? 's' : ''})
              </button>
            </>
          ) : allFinished ? (
            <>
              <span className={`ota-finished-label ${allSuccess ? 'success' : 'error'}`}>
                {allSuccess ? '✓ Mises à jour terminées' : '⚠ Terminé avec erreurs'}
              </span>
              <button className="ota-btn ota-btn-primary" onClick={onClose}>Fermer</button>
            </>
          ) : (
            <span className="ota-in-progress-label">
              <span className="ota-spinner" /> Mise à jour en cours...
            </span>
          )}
        </div>
      </div>
    </div>
  )
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
  qcmAcquiredPoints = null, // Points earned by this team on a QCM question (null = did not answer correctly)
  memoryStats = null, // { pairs, errors, totalPairs, pointsPerPair, errorPenalty, completionBonus }
  questionType = null, // Question type (QCM, NORMAL, MEMORY)
}) {
  const [showTooltip, setShowTooltip] = useState(false)
  const [otaBuzzer, setOtaBuzzer] = useState(null) // buzzer object for OTA modal
  // Use boosted color for display intensity (#61); falls back to raw rgb if boost unavailable
  const rgbColor = (color && boostTeamColor(color)) || (color ? `rgb(${color.join(',')})` : 'var(--primary-500)')
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

  // Calculate Memory acquired points (with completion bonus)
  const memoryAcquiredPoints = useMemo(() => {
    if (!memoryStats || memoryStats.pairs === undefined) return null
    const isComplete = memoryStats.pairs === memoryStats.totalPairs && memoryStats.totalPairs > 0
    let points = memoryStats.pairs * (memoryStats.pointsPerPair || 10) -
                 (memoryStats.errors || 0) * (memoryStats.errorPenalty || 0)
    if (isComplete) points += (memoryStats.completionBonus || 0)
    return Math.max(0, points)
  }, [memoryStats])

  // Detect if team has an active VPlayer
  const teamHasVPlayer = useMemo(() => {
    return buzzers.some(b => b.isVPlayer === true)
  }, [buzzers])


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
    <>
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
        {/* Memory acquired points badge replaces PRET badge */}
        {/* QCM acquired points badge (correct answer in REVEALED phase) */}
        {memoryAcquiredPoints !== null && gamePhase === 'REVEALED' ? (
          <motion.span
            className="memory-acquired-badge"
            initial={{ scale: 0 }}
            animate={{ scale: 1 }}
          >
            +{memoryAcquiredPoints} pts
          </motion.span>
        ) : qcmAcquiredPoints !== null && gamePhase === 'REVEALED' ? (
          <motion.span
            className="memory-acquired-badge qcm-acquired-badge"
            initial={{ scale: 0 }}
            animate={{ scale: 1 }}
          >
            +{qcmAcquiredPoints} pts
          </motion.span>
        ) : ready && (
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
            // Check if buzzer is invalidated by VPlayer during QCM
            const isInvalidatedByVPlayer = questionType === 'QCM' && !buzzer.isVPlayer && teamHasVPlayer
            return (
              <motion.div
                key={`${buzzer.mac}-${buzzer.timestamp}`}
                layoutId={`buzzer-${buzzer.mac}`}
                layout
                className={`buzzer-mini ${buzzer.active ? 'active' : ''} ${buzzer.ready ? 'ready' : ''} ${answerColorData ? 'has-answer-color' : ''} ${buzzerWaitingBuzz ? 'waiting-buzz' : ''} ${buzzerWaitingPong ? 'waiting-pong' : ''} ${onPlayerClick ? 'clickable' : ''} ${buzzer.isVPlayer ? 'is-vplayer' : ''} ${isInvalidatedByVPlayer ? 'invalidated-by-vplayer' : ''} ${!buzzer.isVPlayer && buzzer.connected === false ? 'disconnected' : ''}`}
                style={answerColorData ? { '--answer-color': answerColorData.color } : undefined}
                initial={{ scale: 0.95, opacity: 0.8 }}
                animate={{ scale: 1, opacity: 1 }}
                transition={{ type: 'spring', stiffness: 300, damping: 30 }}
                onClick={handleBuzzerClick}
              >
                <div className="buzzer-info" style={{ flexWrap: 'wrap' }}>
                  {buzzer.isVPlayer ? (
                    <div className="buzzer-vplayer-multicolor">
                      <svg className="vplayer-multicolor-badge" viewBox="0 0 24 24">
                        <path d="M 12,12 L 12,0 A 12,12 0 0,1 24,12 Z" fill={ANSWER_COLORS.RED.color} />
                        <path d="M 12,12 L 24,12 A 12,12 0 0,1 12,24 Z" fill={ANSWER_COLORS.GREEN.color} />
                        <path d="M 12,12 L 12,24 A 12,12 0 0,1 0,12 Z" fill={ANSWER_COLORS.YELLOW.color} />
                        <path d="M 12,12 L 0,12 A 12,12 0 0,1 12,0 Z" fill={ANSWER_COLORS.BLUE.color} />
                      </svg>
                      <span className="vplayer-initial">{(buzzer.name || '?').charAt(0).toUpperCase()}</span>
                    </div>
                  ) : answerColorData ? (
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
                  ) : null}
                  <span className="buzzer-name">{buzzer.name}</span>
                  {isInvalidatedByVPlayer && (
                    <span className="vplayer-invalidation-badge" title="Buzzer physique invalidé : VJoueur actif">
                      🚫
                    </span>
                  )}
                  {!buzzer.isVPlayer && !buzzer.isVirtual && buzzer.connected === false && (
                    <span style={{display:'inline-flex',alignItems:'center',justifyContent:'center',width:'18px',height:'18px',borderRadius:'50%',background:'#f59e0b',flexShrink:0,boxShadow:'0 1px 4px rgba(245,158,11,0.5)'}} title="Buzzer déconnecté"><svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="white" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"><line x1="1" y1="1" x2="23" y2="23"/><path d="M16.72 11.06A10.94 10.94 0 0 1 19 12.55"/><path d="M5 12.55a10.94 10.94 0 0 1 5.17-2.39"/><path d="M10.71 5.05A16 16 0 0 1 22.56 9"/><path d="M1.42 9a15.91 15.91 0 0 1 4.7-2.88"/><path d="M8.53 16.11a6 6 0 0 1 6.95 0"/><line x1="12" y1="20" x2="12.01" y2="20"/></svg></span>
                  )}
                  {!buzzer.isVPlayer && !buzzer.isVirtual && buzzer.ackPending === true && (
                    <span style={{display:'inline-flex',alignItems:'center',justifyContent:'center',width:'18px',height:'18px',borderRadius:'50%',background:'#f59e0b',flexShrink:0,boxShadow:'0 1px 4px rgba(245,158,11,0.5)'}} title="En attente de confirmation"><svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg></span>
                  )}
                  {/* Firmware version indicator */}
                  {buzzer.firmwareVersion && !buzzer.isVPlayer && (
                    <span
                      className={`buzzer-fw-version ${buzzer.isOutdated ? 'outdated' : ''}`}
                      title={buzzer.isOutdated ? 'Cliquer pour mettre a jour' : `Firmware ${buzzer.firmwareVersion} — Ctrl+clic pour forcer la mise a jour`}
                      onClick={(e) => { if (buzzer.isOutdated || e.ctrlKey) { e.stopPropagation(); setOtaBuzzer(buzzer) } }}
                    >
                      fw: {buzzer.firmwareVersion}
                    </span>
                  )}
                  {/* OTA inline status (downloading / flashing / done / error) */}
                  {buzzer.otaStatus && buzzer.otaStatus !== '' && !buzzer.isVPlayer && (
                    <span className={`buzzer-ota-status buzzer-ota-status-${buzzer.otaStatus}`}>
                      {(buzzer.otaStatus === 'downloading' || buzzer.otaStatus === 'flashing') && (
                        <span className="ota-spinner-inline" />
                      )}
                      {buzzer.otaStatus}
                    </span>
                  )}
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
    {/* OTA Modal - rendered outside the card to avoid overflow clipping */}
    {otaBuzzer && (
      <OtaModal
        buzzer={sortedBuzzers.find(b => b.mac === otaBuzzer.mac) || otaBuzzer}
        onClose={() => setOtaBuzzer(null)}
      />
    )}
    </>
  )
}
