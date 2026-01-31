import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import useWebSocket from '../hooks/useWebSocket'
import PlayerDisplay from './PlayerDisplay'
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
  const { sendMessage, gameState, bumpers, teams, status, setClientType } = useWebSocket()

  const [playerSession, setPlayerSession] = useState(null)
  const [bumper, setBumper] = useState(null)
  const [team, setTeam] = useState(null)

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
      setBumper({ id: bumperId, ...bumperData })

      // Find team if assigned
      if (bumperData.TEAM && teams[bumperData.TEAM]) {
        setTeam(teams[bumperData.TEAM])
      } else {
        setTeam(null)
      }
    }
  }, [playerSession, bumpers, teams])

  // Identify as vplayer IMMEDIATELY on connection (before any other action)
  // This must happen first, before checking session or bumper
  useEffect(() => {
    if (status === 'connected') {
      console.log('[VPlayer] Setting client type to vplayer')
      setClientType('vplayer')
    }
  }, [status, setClientType])

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

  // Auto-reconnect if session exists but bumper not found
  useEffect(() => {
    if (!playerSession || bumper || status !== 'connected') return

    // Wait a bit for initial state sync
    const timeoutId = setTimeout(() => {
      if (!bumper) {
        // Try reconnecting by sending PLAYER_CONNECT again
        console.log('[VPlayer] Reconnecting with name:', playerSession.name)
        sendMessage('PLAYER_CONNECT', { NAME: playerSession.name })
      }
    }, 2000)

    return () => clearTimeout(timeoutId)
  }, [playerSession, bumper, status, sendMessage])

  // Auto-respond to PREPARE phase with PONG
  useEffect(() => {
    if (!bumper || !bumper.id) return
    if (gameState.phase !== 'PREPARE') return

    console.log('[VPlayer] Auto-sending PONG in PREPARE phase, bumper ID:', bumper.id)
    // Send PONG - ID in payload for web clients
    sendMessage('PONG', { ID: bumper.id })
  }, [gameState.phase, bumper, sendMessage])

  const handleBuzz = () => {
    if (!bumper || !bumper.id) return

    // Only allow buzz during STARTED or PAUSED phases
    if (gameState.phase !== 'STARTED' && gameState.phase !== 'PAUSED') return

    // Block buzz for MEMORY questions (admin controls the game)
    if (gameState.question?.TYPE === 'MEMORY') {
      console.log('[VPlayer] Buzz blocked for MEMORY question')
      return
    }

    console.log('[VPlayer] Buzzing:', bumper.id)

    // Send BUTTON message with correct format
    sendMessage('BUTTON', { ID: bumper.id, button: 'A' })
  }

  // Get player name color from ANSWER_COLOR
  const getPlayerNameColor = () => {
    if (!bumper || !bumper.ANSWER_COLOR) return null
    return ANSWER_COLORS[bumper.ANSWER_COLOR] || null
  }

  // Get team color in rgb format
  const getTeamColor = () => {
    if (!team || !team.COLOR) return null
    return `rgb(${team.COLOR.join(',')})`
  }

  // Check if player has buzzed (TIME > 0)
  const hasBuzzed = bumper && bumper.TIME > 0

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
      {/* Buzz confirmation overlay */}
      {hasBuzzed && (
        <div className="vplayer-buzz-overlay">
          {/* Show checkmark and text only during STARTED and PAUSED */}
          {(gameState.phase === 'STARTED' || gameState.phase === 'PAUSED') && (
            <>
              <div className="vplayer-buzz-checkmark">✓</div>
              <div className="vplayer-buzz-text">BUZZÉ !</div>
            </>
          )}
        </div>
      )}

      <PlayerDisplay
        playerName={bumper?.NAME}
        playerNameColor={getPlayerNameColor()}
        teamName={team?.NAME}
        teamColor={getTeamColor()}
        isVPlayer={true}
        onMediaClick={handleBuzz}
      />
    </div>
  )
}
