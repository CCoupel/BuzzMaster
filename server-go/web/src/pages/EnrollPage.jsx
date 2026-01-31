import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useGame } from '../hooks/GameContext'
import './EnrollPage.css'

export default function EnrollPage() {
  const navigate = useNavigate()
  const { connectVirtualPlayer, gameState, status, bumpers, setClientType } = useGame()

  const [playerName, setPlayerName] = useState('')
  const [error, setError] = useState('')
  const [isConnecting, setIsConnecting] = useState(false)
  const [validationError, setValidationError] = useState('')
  const [checkingSession, setCheckingSession] = useState(true)

  // Identify as vplayer IMMEDIATELY on connection (before any other action)
  useEffect(() => {
    if (status === 'connected') {
      console.log('[EnrollPage] Setting client type to vplayer')
      setClientType('vplayer')
    }
  }, [status, setClientType])

  // Timeout to stop checking after 2 seconds
  useEffect(() => {
    const timeout = setTimeout(() => {
      if (checkingSession) {
        console.log('[EnrollPage] Session check timeout')
        setCheckingSession(false)
      }
    }, 2000)
    return () => clearTimeout(timeout)
  }, [checkingSession])

  // Check for existing session on mount and when bumpers are loaded
  useEffect(() => {
    // Wait for WebSocket connection
    if (status !== 'connected') {
      return
    }

    const savedName = localStorage.getItem('vplayer_name')

    if (savedName) {
      // Check if this player exists on the server
      const existingBumper = Object.values(bumpers).find(
        b => b.IS_VIRTUAL && b.NAME === savedName
      )

      if (existingBumper) {
        // Player exists on server, go directly to game
        console.log('[EnrollPage] Found existing session for:', savedName)
        navigate('/player')
        return
      } else if (Object.keys(bumpers).length > 0) {
        // Bumpers loaded but player not found - clear session
        console.log('[EnrollPage] Session expired for:', savedName)
        localStorage.removeItem('vplayer_name')
        localStorage.removeItem('vplayer_session')
        setCheckingSession(false)
      }
      // If bumpers is empty, keep waiting (timeout will handle)
    } else {
      // No saved session
      setCheckingSession(false)
    }
  }, [status, bumpers, navigate])

  // Real-time validation
  useEffect(() => {
    if (!playerName) {
      setValidationError('')
      return
    }

    if (playerName.length < 2) {
      setValidationError('Minimum 2 caractères')
    } else if (playerName.length > 20) {
      setValidationError('Maximum 20 caractères')
    } else {
      setValidationError('')
    }
  }, [playerName])

  const handleSubmit = (e) => {
    e.preventDefault()

    const trimmedName = playerName.trim()

    // Validate
    if (trimmedName.length < 2 || trimmedName.length > 20) {
      setError('Le pseudo doit contenir entre 2 et 20 caractères')
      return
    }

    // Check if enrollment is open
    if (!gameState.enrollmentActive) {
      setError('Les inscriptions ne sont pas ouvertes')
      return
    }

    // Clear previous errors
    setError('')
    setIsConnecting(true)

    // Client type already set on mount, just send connection request
    connectVirtualPlayer(trimmedName)

    // Store in localStorage for reconnection
    localStorage.setItem('vplayer_name', trimmedName)
    localStorage.setItem('vplayer_session', Date.now().toString())

    // Navigate to player page
    setTimeout(() => {
      navigate('/player')
    }, 500)
  }

  const isValid = playerName.trim().length >= 2 && playerName.trim().length <= 20
  const enrollmentOpen = gameState.enrollmentActive

  // Show loading while checking session
  if (checkingSession) {
    return (
      <div className="enroll-page">
        <div className="enroll-container">
          <div className="enroll-waiting">
            <span className="waiting-spinner">⏳</span>
            <p>Vérification...</p>
          </div>
        </div>
      </div>
    )
  }

  // Show waiting state if enrollment is not open
  if (!enrollmentOpen) {
    return (
      <div className="enroll-page">
        <div className="enroll-container">
          <div className="enroll-header">
            <h1>🎮 BuzzMaster</h1>
          </div>
          <div className="enroll-waiting">
            <span className="waiting-spinner">⏳</span>
            <p>En attente de l'ouverture des inscriptions...</p>
            <p className="waiting-hint">L'animateur doit démarrer les inscriptions</p>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="enroll-page">
      <div className="enroll-container">
        <div className="enroll-header">
          <h1>🎮 BuzzMaster</h1>
          <p>Rejoins la partie en tant que joueur virtuel</p>
        </div>

        <form className="enroll-form" onSubmit={handleSubmit}>
          <div className="form-group">
            <label htmlFor="playerName">Choisis ton pseudo</label>
            <input
              type="text"
              id="playerName"
              value={playerName}
              onChange={(e) => setPlayerName(e.target.value)}
              placeholder="Entre ton pseudo..."
              maxLength={20}
              autoComplete="off"
              autoFocus
              disabled={isConnecting}
            />
            <div className="input-feedback">
              {validationError && (
                <span className="validation-error">{validationError}</span>
              )}
              <span className="char-count">
                {playerName.length}/20
              </span>
            </div>
          </div>

          {error && (
            <div className="error-message">
              {error}
            </div>
          )}

          <button
            type="submit"
            className="btn-join"
            disabled={!isValid || isConnecting}
          >
            {isConnecting ? 'Connexion...' : 'Rejoindre la partie'}
          </button>
        </form>

        <div className="enroll-footer">
          <p>Places disponibles: {gameState.virtualPlayerCount || 0}/{gameState.virtualPlayerLimit || 20}</p>
        </div>
      </div>
    </div>
  )
}
