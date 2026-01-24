import { useState, useEffect, useRef } from 'react'
import { useGame } from '../hooks/GameContext'
import BuzzButton from '../components/BuzzButton'
import PlayerDisplay from './PlayerDisplay'
import './PlayerPage.css'

const STORAGE_KEY = 'buzzcontrol_player'
const STORAGE_TTL = 2 * 60 * 60 * 1000 // 2 hours

export default function PlayerPage() {
  const { gameState, teams, bumpers, connectAsPlayer, playerStatus, sendMessage, status: wsStatus } = useGame()
  const [name, setName] = useState('')
  const [status, setStatus] = useState('idle') // idle, connecting, connected, rejected
  const [error, setError] = useState('')
  const [reconnecting, setReconnecting] = useState(false)
  const hasTriedReconnect = useRef(false)

  // Try to restore from localStorage once WebSocket is connected
  useEffect(() => {
    // Wait for WebSocket connection before attempting reconnection
    if (wsStatus !== 'connected') return
    if (hasTriedReconnect.current) return

    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved) {
      try {
        const { name: savedName, timestamp } = JSON.parse(saved)
        // Check if not expired and name is valid
        if (Date.now() - timestamp < STORAGE_TTL && savedName && savedName.trim()) {
          hasTriedReconnect.current = true
          console.log('[PlayerPage] Attempting reconnection for saved player:', savedName)
          setName(savedName)
          setReconnecting(true)
          setStatus('connecting')
          connectAsPlayer(savedName)
        } else {
          // Expired or invalid, clear storage
          console.log('[PlayerPage] Saved session expired or invalid, clearing localStorage')
          localStorage.removeItem(STORAGE_KEY)
        }
      } catch (err) {
        console.error('[PlayerPage] Failed to parse stored player data:', err)
        localStorage.removeItem(STORAGE_KEY)
      }
    }
  }, [wsStatus, connectAsPlayer])

  const handleSubmit = (e) => {
    e.preventDefault()
    if (!name.trim()) return
    setStatus('connecting')
    setError('')
    setReconnecting(false)
    connectAsPlayer(name.trim())
  }

  useEffect(() => {
    console.log('[PlayerPage] playerStatus changed:', playerStatus)
    if (playerStatus?.status === 'connected') {
      console.log('[PlayerPage] Player connected, ID:', playerStatus.id)
      setStatus('connected')
      setError('')
      setReconnecting(false)
      // Save to localStorage
      localStorage.setItem(STORAGE_KEY, JSON.stringify({
        name: playerStatus.name || name,
        id: playerStatus.id,
        timestamp: Date.now()
      }))
    } else if (playerStatus?.status === 'rejected') {
      console.log('[PlayerPage] Player rejected, reason:', playerStatus.reason)
      setStatus('rejected')
      const reason = playerStatus.reason
      if (reason === 'LIMIT_REACHED') {
        setError('Nombre maximum de joueurs atteint')
      } else if (reason === 'INVALID_NAME') {
        setError('Nom invalide (2-20 caractères)')
      } else {
        setError('Les inscriptions sont fermées')
      }
      // Allow retry after rejection
      setTimeout(() => {
        setStatus('idle')
      }, 3000)
    }
  }, [playerStatus, name])

  // Phase non ENROLL = inscriptions fermées
  if (gameState.phase !== 'ENROLL' && status !== 'connected') {
    return (
      <div className="player-page-enrollment">
        <div className="player-card">
          <h1>📱 BuzzControl</h1>
          <p className="closed-message">Les inscriptions ne sont pas ouvertes</p>
          <p className="help-text">Attendez que l'organisateur ouvre les inscriptions</p>
        </div>
      </div>
    )
  }

  // Connecté : Interface de jeu complète
  if (status === 'connected') {
    const playerId = playerStatus?.id
    const playerData = playerId ? bumpers[playerId] : null
    const playerName = playerData?.NAME || playerStatus?.name || name
    const teamId = playerData?.TEAM
    const teamData = teamId ? teams[teamId] : null
    const teamName = teamData?.NAME || ''
    const teamColor = teamData?.COLOR ? `rgb(${teamData.COLOR.join(',')})` : '#6b7280'
    const score = playerData?.SCORE || 0
    const answerColor = playerData?.ANSWER_COLOR
    const isAssigned = !!(teamId && answerColor)
    const hasBuzzed = (playerData?.TIME || 0) > 0
    const connected = wsStatus === 'connected'

    // Couleurs de réponse QCM
    const ANSWER_COLORS = {
      RED: '#ef4444',
      GREEN: '#22c55e',
      YELLOW: '#eab308',
      BLUE: '#3b82f6'
    }

    const playerNameColor = answerColor ? ANSWER_COLORS[answerColor] : '#6b7280'

    const handleBuzz = () => {
      if (gameState.phase !== 'STARTED' || !isAssigned) return false

      // Use player's assigned ANSWER_COLOR as button press
      const button = answerColor || 'A'
      sendMessage('BUTTON', { ID: playerId, button })

      // Vérifier si c'est le premier buzzer (aucune équipe n'a encore buzzé)
      const anyTeamBuzzed = Object.values(teams).some(t => t.TIME > 0)
      return !anyTeamBuzzed
    }

    // Phase ENROLL : afficher un écran d'attente
    if (gameState.phase === 'ENROLL') {
      return (
        <div className="player-page-enrollment">
          <div className="player-card waiting-card">
            <h1>Bienvenue {playerName} !</h1>
            <div className="success-icon">✓</div>
            <p className="help-text">Inscription réussie</p>
            <p className="waiting-message">En attente du début de la partie...</p>
          </div>
        </div>
      )
    }

    return (
      <div className="player-page-game">
        <div className="player-display-zone">
          <PlayerDisplay
            playerName={playerName}
            playerNameColor={isAssigned && answerColor ? playerNameColor : '#6b7280'}
            teamName={isAssigned ? teamName : null}
            teamColor={teamColor}
          />
        </div>
        <div className="buzz-button-zone">
          <BuzzButton
            phase={gameState.phase}
            isAssigned={isAssigned}
            teamColor={teamColor}
            hasBuzzed={hasBuzzed}
            onClick={handleBuzz}
          />
        </div>
      </div>
    )
  }

  // Formulaire d'inscription (phase ENROLL)
  return (
    <div className="player-page-enrollment">
      <div className="player-card">
        <h1>📱 Rejoindre la partie</h1>

        {error && (
          <div className="error-message">
            <span className="error-icon">⚠️</span>
            {error}
          </div>
        )}

        {status === 'connecting' ? (
          <div className="reconnection-info">
            <div className="spinner-large"></div>
            <p>Connexion en cours...</p>
            <button
              onClick={() => {
                localStorage.removeItem(STORAGE_KEY)
                setStatus('idle')
                setName('')
                setReconnecting(false)
                hasTriedReconnect.current = false
              }}
              className="forget-btn"
            >
              Oublier et rejoindre
            </button>
          </div>
        ) : (
          <form onSubmit={handleSubmit}>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Votre nom"
              disabled={status === 'connecting'}
              autoFocus
              maxLength={20}
            />
            <button
              type="submit"
              disabled={status === 'connecting' || !name.trim()}
            >
              Rejoindre
            </button>
          </form>
        )}

        {status !== 'connecting' && (
          <p className="help-text">
            Entrez votre nom pour vous inscrire à la partie
          </p>
        )}
      </div>
    </div>
  )
}
