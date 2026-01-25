import { useState } from 'react'
import './BuzzButton.css'

export default function BuzzButton({ bumper, phase, onBuzz, disabled }) {
  const [isPressing, setIsPressing] = useState(false)

  const handlePress = () => {
    if (disabled) return

    // Trigger vibration if available
    if (navigator.vibrate) {
      navigator.vibrate(100)
    }

    setIsPressing(true)
    onBuzz()

    // Reset visual state after animation
    setTimeout(() => {
      setIsPressing(false)
    }, 300)
  }

  // Determine button state
  const getButtonState = () => {
    if (!bumper?.TEAM) return 'NOT_ASSIGNED'

    switch (phase) {
      case 'STOPPED':
      case 'REVEALED':
        return 'STOPPED'
      case 'PREPARE':
        return 'PREPARE'
      case 'READY':
      case 'COUNTDOWN':
        return 'READY'
      case 'STARTED':
        return 'STARTED'
      case 'PAUSED':
        return 'PAUSED'
      default:
        return 'STOPPED'
    }
  }

  const getButtonText = () => {
    const state = getButtonState()

    switch (state) {
      case 'NOT_ASSIGNED':
        return 'En attente...'
      case 'STOPPED':
        return 'En attente de question'
      case 'PREPARE':
        return 'Préparation...'
      case 'READY':
        return 'Prêt !'
      case 'STARTED':
        return 'BUZZ !'
      case 'PAUSED':
        return 'Déjà buzzé'
      case 'REVEALED':
        return 'Réponse révélée'
      default:
        return 'BUZZ'
    }
  }

  const buttonState = getButtonState()
  const isActive = buttonState === 'STARTED' && !disabled
  const isDisabled = disabled || buttonState === 'NOT_ASSIGNED' || buttonState === 'PAUSED' || buttonState === 'STOPPED'

  return (
    <div className="buzz-button-container">
      <button
        className={`buzz-button ${buttonState.toLowerCase()} ${isPressing ? 'pressing' : ''} ${isActive ? 'active' : ''}`}
        onClick={handlePress}
        disabled={isDisabled}
      >
        <span className="buzz-text">{getButtonText()}</span>
      </button>
    </div>
  )
}
