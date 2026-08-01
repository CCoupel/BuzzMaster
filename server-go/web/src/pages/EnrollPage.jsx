import { useState, useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { useGame } from '../hooks/GameContext'
import { REJECTION_MESSAGES, DEFAULT_REJECTION_MESSAGE, REDIRECT_MESSAGES, DEFAULT_REDIRECT_MESSAGE } from '../utils/playerConnectMessages'
import './EnrollPage.css'

// #120 (F3) — présentation du bandeau de motif selon la catégorie du renvoi.
// Le texte affiché vient de REDIRECT_MESSAGES/DEFAULT_REDIRECT_MESSAGE
// (validé au GATE 2) ; ceci ne pilote que l'icône/la couleur.
const REDIRECT_BANNER_META = {
  PLAYER_REMOVED: { kind: 'warn', icon: '⚠️' },
  GAME_RESET: { kind: 'info', icon: '🔄' },
  SESSION_EXPIRED: { kind: 'warn', icon: '⚠️' },
}
const DEFAULT_REDIRECT_BANNER_META = { kind: 'warn', icon: '⚠️' }

// Le serveur ne répond jamais (perte réseau, etc.) — ne pas rester bloqué sur "Connexion..."
const CONNECT_TIMEOUT_MS = 5000

export default function EnrollPage() {
  const navigate = useNavigate()
  const { connectVirtualPlayer, gameState, status, bumpers, playerConnectStatus, clearPlayerConnectStatus } = useGame()

  const [playerName, setPlayerName] = useState('')
  const [error, setError] = useState('')
  const [isConnecting, setIsConnecting] = useState(false)
  const [validationError, setValidationError] = useState('')
  const [checkingSession, setCheckingSession] = useState(true)
  // #120 (F3) — motif de renvoi relayé par VPlayerPage (PLAYER_EVICTED ou le
  // filet de sécurité local), lu une seule fois au montage puis consommé.
  const [redirectReason, setRedirectReason] = useState(null)
  const redirectReasonConsumedRef = useRef(false)
  // Pseudo soumis en attente de confirmation serveur — stocké en localStorage
  // seulement après PLAYER_CONNECTED (pas avant : un pseudo rejeté NAME_TAKEN
  // ne doit jamais être persisté comme si c'était le nôtre).
  const pendingNameRef = useRef('')
  const connectTimeoutRef = useRef(null)

  // #120 (F3) — lire le motif de renvoi une seule fois au montage. Relais en
  // sessionStorage (déposé par VPlayerPage avant `navigate('/')`) : cette
  // seule voie couvre à la fois la navigation SPA immédiate (lu dès le
  // montage suivant) et un rechargement complet de cet onglet (sessionStorage
  // survit au reload, contrairement à un état de navigation React Router).
  // Retiré immédiatement après lecture pour qu'un rechargement ultérieur ne
  // réaffiche pas le bandeau.
  useEffect(() => {
    if (redirectReasonConsumedRef.current) return
    redirectReasonConsumedRef.current = true
    let reason = null
    try {
      reason = sessionStorage.getItem('vplayer_redirect_reason')
      if (reason !== null) sessionStorage.removeItem('vplayer_redirect_reason')
    } catch {
      reason = null
    }
    if (reason !== null) setRedirectReason(reason)
  }, [])

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
      // #120 (F2) — identité par ID, repli sur le nom uniquement pour une
      // session antérieure à cette version (pas de vplayer_id stocké) —
      // repli transitoire, appelé à disparaître avec ces anciennes sessions.
      const savedId = localStorage.getItem('vplayer_id')
      const existingBumper = savedId
        ? (bumpers[savedId]?.IS_VIRTUAL ? bumpers[savedId] : null)
        : Object.values(bumpers).find(b => b.IS_VIRTUAL && b.NAME === savedName)

      if (existingBumper) {
        // Player exists on server, go directly to game
        console.log('[EnrollPage] Found existing session for:', savedName)
        navigate('/player')
      }
      // #120 : plus de branche "else" — l'absence du bumper n'est jamais, à
      // elle seule, la preuve que la session est morte (c'est exactement la
      // course qui causait #120). On continue d'attendre : le timeout de
      // vérification ci-dessus fera apparaître le formulaire si rien
      // n'arrive. L'éviction est désormais décidée par le serveur
      // (PLAYER_EVICTED, traité sur VPlayerPage), jamais déduite ici.
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

  // Fix R1 (#109) : attendre PLAYER_CONNECTED (succès → /player) ou
  // PLAYER_REJECTED (erreur affichée, notamment NAME_TAKEN — redemander un
  // pseudo) au lieu de naviguer en aveugle après un délai fixe.
  useEffect(() => {
    if (!playerConnectStatus) return

    if (connectTimeoutRef.current) {
      clearTimeout(connectTimeoutRef.current)
      connectTimeoutRef.current = null
    }

    if (playerConnectStatus.status === 'connected') {
      // Persister la session seulement une fois le pseudo confirmé accepté
      // par le serveur (jamais avant : un pseudo rejeté ne doit pas polluer
      // localStorage comme si la connexion avait réussi).
      localStorage.setItem('vplayer_name', pendingNameRef.current)
      localStorage.setItem('vplayer_session', Date.now().toString())
      navigate('/player')
    } else if (playerConnectStatus.status === 'rejected') {
      setIsConnecting(false)
      setError(REJECTION_MESSAGES[playerConnectStatus.reason] || DEFAULT_REJECTION_MESSAGE)
      // Redemander un pseudo : vider le champ pour forcer une nouvelle saisie.
      setPlayerName('')
    }

    clearPlayerConnectStatus()
  }, [playerConnectStatus, navigate, clearPlayerConnectStatus])

  // Nettoyage du timeout de garde au démontage
  useEffect(() => {
    return () => {
      if (connectTimeoutRef.current) clearTimeout(connectTimeoutRef.current)
    }
  }, [])

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
    pendingNameRef.current = trimmedName

    // Client type already set on mount, just send connection request
    connectVirtualPlayer(trimmedName)

    // Garde-fou : si le serveur ne répond jamais (perte réseau...), ne pas
    // rester bloqué indéfiniment sur "Connexion...".
    if (connectTimeoutRef.current) clearTimeout(connectTimeoutRef.current)
    connectTimeoutRef.current = setTimeout(() => {
      setIsConnecting(false)
      setError('Le serveur ne répond pas, réessaie.')
    }, CONNECT_TIMEOUT_MS)
  }

  const isValid = playerName.trim().length >= 2 && playerName.trim().length <= 20
  const enrollmentOpen = gameState.enrollmentActive

  // #120 (F3) — bandeau de motif, affiché au-dessus du formulaire (et de
  // l'écran d'attente d'ouverture des inscriptions) quand le joueur arrive à
  // la suite d'un renvoi. Jamais pendant la vérification de session : cet
  // écran est transitoire (≤ 2 s) et précède la décision d'affichage.
  const redirectBanner = redirectReason !== null && (
    <div
      className={`enroll-redirect-banner ${(REDIRECT_BANNER_META[redirectReason] || DEFAULT_REDIRECT_BANNER_META).kind}`}
      role="status"
    >
      <span className="enroll-redirect-banner-icon">
        {(REDIRECT_BANNER_META[redirectReason] || DEFAULT_REDIRECT_BANNER_META).icon}
      </span>
      <span>{REDIRECT_MESSAGES[redirectReason] || DEFAULT_REDIRECT_MESSAGE}</span>
    </div>
  )

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
          {redirectBanner}
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

        {redirectBanner}

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
