import { useState, useEffect, useRef, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { useGame } from '../hooks/GameContext'
import PlayerDisplay from './PlayerDisplay'
import NoSleep from 'nosleep.js'
import './VPlayerPage.css'

// QCM answer colors mapping
const ANSWER_COLORS = {
  RED: '#ef4444',
  GREEN: '#22c55e',
  YELLOW: '#eab308',
  BLUE: '#3b82f6',
}

export default function VPlayerPage() {
  const navigate = useNavigate()
  const { sendMessage, gameState, bumpers, teams, status } = useGame()

  const [playerSession, setPlayerSession] = useState(null)
  const [bumper, setBumper] = useState(null)
  const [team, setTeam] = useState(null)
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [showFullscreenHint, setShowFullscreenHint] = useState(false)
  const noSleepRef = useRef(null)
  // Ref to always have the latest bumper value in async callbacks (avoids stale closure)
  const bumperRef = useRef(null)

  // Load session from localStorage on mount
  useEffect(() => {
    const savedName = localStorage.getItem('vplayer_name')
    const savedSession = localStorage.getItem('vplayer_session')

    if (!savedName || !savedSession) {
      // No session, redirect to enroll page
      navigate('/')
      return
    }

    setPlayerSession({
      name: savedName,
      sessionId: savedSession,
    })
  }, [navigate])

  // Find bumper by name in bumpers list
  useEffect(() => {
    if (!playerSession || !bumpers) return

    // Find bumper matching our name
    const foundBumper = Object.entries(bumpers).find(([_, b]) =>
      b.IS_VIRTUAL && b.NAME === playerSession.name
    )

    if (foundBumper) {
      const [bumperId, bumperData] = foundBumper
      const newBumper = { id: bumperId, ...bumperData }
      setBumper(newBumper)
      bumperRef.current = newBumper

      // Find team if assigned
      if (bumperData.TEAM && teams[bumperData.TEAM]) {
        setTeam(teams[bumperData.TEAM])
      } else {
        setTeam(null)
      }
    } else {
      // Bumper not found in current state — reset ref so reconnect logic can fire
      bumperRef.current = null
    }
  }, [playerSession, bumpers, teams])

  // Detect if bumper was deleted by admin - redirect to enrollment page
  useEffect(() => {
    if (!playerSession || status !== 'connected') return

    // Wait for initial state sync (at least one bumper loaded or 3 seconds timeout)
    if (Object.keys(bumpers).length === 0) return

    // Check if bumper still exists in the server's bumpers list
    const stillExists = Object.values(bumpers).some(
      b => b.IS_VIRTUAL && b.NAME === playerSession.name
    )

    // If bumper doesn't exist anymore, clear session and redirect
    if (!stillExists) {
      console.log('[VPlayer] Bumper deleted by admin, redirecting to enrollment')
      localStorage.removeItem('vplayer_name')
      localStorage.removeItem('vplayer_session')
      navigate('/')
    }
  }, [bumpers, playerSession, status, navigate])

  // Auto-reconnect if session exists but bumper not found after initial state sync
  // Uses bumperRef (not bumper state) to read the latest value inside the timeout callback,
  // preventing the stale closure bug that caused PLAYER_CONNECT to be sent even when the
  // bumper was already found (which could create duplicate VJoueur entries on the server).
  useEffect(() => {
    if (!playerSession || status !== 'connected') return
    // If we already found the bumper, no need to reconnect
    if (bumperRef.current) return

    // Wait for initial state sync before attempting reconnect
    const timeoutId = setTimeout(() => {
      // Check ref (latest value) rather than closure-captured bumper
      if (!bumperRef.current) {
        console.log('[VPlayer] Reconnecting with name:', playerSession.name)
        sendMessage('PLAYER_CONNECT', { NAME: playerSession.name })
      }
    }, 2000)

    return () => clearTimeout(timeoutId)
  }, [playerSession, status, sendMessage])

  // Auto-respond to PREPARE phase with PONG
  useEffect(() => {
    if (!bumper || !bumper.id) return
    if (gameState.phase !== 'PREPARE') return

    console.log('[VPlayer] Auto-sending PONG in PREPARE phase, bumper ID:', bumper.id)
    // Send PONG - ID in payload for web clients
    sendMessage('PONG', { ID: bumper.id })
  }, [gameState.phase, bumper, sendMessage])

  // NoSleep — requires user gesture to enable (called on fullscreen tap or first buzz)
  const activateNoSleep = useCallback(() => {
    if (!noSleepRef.current) noSleepRef.current = new NoSleep()
    if (!noSleepRef.current.isEnabled) noSleepRef.current.enable()
  }, [])

  useEffect(() => {
    return () => { if (noSleepRef.current?.isEnabled) noSleepRef.current.disable() }
  }, [])

  // Auto-fullscreen on mount; show hint if browser rejects (e.g. reconnect without user gesture)
  useEffect(() => {
    const el = document.documentElement
    const fn = el.requestFullscreen || el.webkitRequestFullscreen || el.mozRequestFullScreen
    if (fn) {
      fn.call(el).catch(() => setShowFullscreenHint(true))
    } else {
      setShowFullscreenHint(true)
    }
    const onChange = () => setIsFullscreen(!!(document.fullscreenElement || document.webkitFullscreenElement))
    document.addEventListener('fullscreenchange', onChange)
    document.addEventListener('webkitfullscreenchange', onChange)
    return () => {
      document.removeEventListener('fullscreenchange', onChange)
      document.removeEventListener('webkitfullscreenchange', onChange)
    }
  }, [])

  // Tap on fullscreen hint: enter fullscreen + activate NoSleep (user gesture)
  const handleFullscreenTap = useCallback(() => {
    setShowFullscreenHint(false)
    const el = document.documentElement
    const fn = el.requestFullscreen || el.webkitRequestFullscreen
    if (fn) fn.call(el).catch(() => {})
    activateNoSleep()
  }, [activateNoSleep])

  const handleBuzz = () => {
    if (!bumper || !bumper.id) return

    // Only allow buzz during STARTED or PAUSED phases
    if (gameState.phase !== 'STARTED' && gameState.phase !== 'PAUSED') return

    // Block buzz for MEMORY questions (admin controls the game)
    if (gameState.question?.TYPE === 'MEMORY') {
      console.log('[VPlayer] Buzz blocked for MEMORY question')
      return
    }

    // Block buzz for QCM questions (use QCM buttons instead)
    if (gameState.question?.TYPE === 'QCM') {
      console.log('[VPlayer] Buzz blocked for QCM question - use QCM buttons')
      return
    }

    console.log('[VPlayer] Buzzing:', bumper.id)
    activateNoSleep()
    sendMessage('BUTTON', { ID: bumper.id, button: 'A' })
  }

  const handleQCMAnswer = (color) => {
    if (!bumper || !bumper.id) return
    if (gameState.phase !== 'STARTED') return
    if (gameState.question?.TYPE !== 'QCM') return

    console.log('[VPlayer] QCM answer:', color)

    // Send VPLAYER_QCM_ANSWER message - wait for server confirmation
    sendMessage('VPLAYER_QCM_ANSWER', { ID: bumper.id, ANSWER_COLOR: color })
  }

  // Get player name color - use team color (VPlayers have no dedicated ANSWER_COLOR)
  const getPlayerNameColor = () => {
    if (bumper?.ANSWER_COLOR && ANSWER_COLORS[bumper.ANSWER_COLOR]) {
      return ANSWER_COLORS[bumper.ANSWER_COLOR]
    }
    // Fallback to team color for VPlayers
    if (team?.COLOR) {
      return `rgb(${team.COLOR.join(',')})`
    }
    return null
  }

  // Get team color in rgb format
  const getTeamColor = () => {
    if (!team || !team.COLOR) return null
    return `rgb(${team.COLOR.join(',')})`
  }

  // Check if player has buzzed (TIME > 0)
  const hasBuzzed = bumper && bumper.TIME > 0

  // Check if current question is QCM
  const isQcmQuestion = gameState.question?.TYPE === 'QCM'

  // Show loading if no session or bumper not found yet
  if (!playerSession) {
    return (
      <div className="vplayer-page loading">
        <div className="loading-spinner">Chargement...</div>
      </div>
    )
  }

  return (
    <div className="vplayer-page">
      {/* Fullscreen hint — shown when auto-fullscreen failed (reconnect without user gesture) */}
      {showFullscreenHint && (
        <div className="vplayer-fullscreen-hint" onClick={handleFullscreenTap}>
          <span className="vplayer-fullscreen-hint-icon">⛶</span>
          <span className="vplayer-fullscreen-hint-text">Appuyez pour le plein écran</span>
        </div>
      )}

      {/* Buzz confirmation overlay */}
      {hasBuzzed && (() => {
        const answerColor = isQcmQuestion && bumper.ANSWER_COLOR ? ANSWER_COLORS[bumper.ANSWER_COLOR] : null
        const answerLetter = isQcmQuestion && bumper.ANSWER_COLOR
          ? ['A', 'B', 'C', 'D'][['RED', 'GREEN', 'YELLOW', 'BLUE'].indexOf(bumper.ANSWER_COLOR)]
          : null
        const answerText = isQcmQuestion && bumper.ANSWER_COLOR && gameState.question?.QCM_ANSWERS
          ? gameState.question.QCM_ANSWERS[bumper.ANSWER_COLOR]
          : null
        return (
          <div
            className="vplayer-buzz-overlay"
            style={answerColor ? { '--buzz-color': answerColor } : undefined}
          >
            {(gameState.phase === 'STARTED' || gameState.phase === 'PAUSED' || gameState.phase === 'REVEALED') && (
              <>
                <div className="vplayer-buzz-checkmark">✓</div>
                <div className="vplayer-buzz-text">
                  {answerLetter && answerText ? `${answerLetter} : ${answerText}` : answerLetter ? `${answerLetter} !` : 'BUZZÉ !'}
                </div>
              </>
            )}
          </div>
        )
      })()}

      <PlayerDisplay
        playerName={bumper?.NAME}
        playerNameColor={getPlayerNameColor()}
        teamName={team?.NAME}
        teamColor={getTeamColor()}
        isVPlayer={true}
        onMediaClick={handleBuzz}
        onQCMAnswer={handleQCMAnswer}
        vplayerHasBuzzed={hasBuzzed}
      />
    </div>
  )
}
