import { useState, useEffect, useRef } from 'react'
import './BuzzButton.css'

export default function BuzzButton({ phase, isAssigned, teamColor, hasBuzzed, onClick }) {
  const [isPressed, setIsPressed] = useState(false)
  const [showConfetti, setShowConfetti] = useState(false)
  const buttonRef = useRef(null)

  // Determine button state
  const isDisabled = phase !== 'STARTED' || !isAssigned || hasBuzzed
  const showWaiting = !isAssigned
  const showBuzzed = hasBuzzed

  const handleClick = () => {
    if (isDisabled) return

    setIsPressed(true)
    const isFirst = onClick?.()

    if (isFirst) {
      setShowConfetti(true)
      setTimeout(() => setShowConfetti(false), 2000)
    }

    setTimeout(() => setIsPressed(false), 200)
  }

  const handleTouchStart = (e) => {
    e.preventDefault()
    handleClick()
  }

  // Get status text
  const getStatusText = () => {
    if (showWaiting) return "En attente d'attribution..."
    if (showBuzzed) return "Buzzed!"
    if (phase === 'STARTED') return "BUZZ!"
    if (phase === 'PAUSE') return "Pause"
    if (phase === 'READY' || phase === 'PREPARE') return "Prêt..."
    return "En attente..."
  }

  return (
    <div className="buzz-button-container">
      {showConfetti && (
        <div className="confetti-overlay">
          {[...Array(20)].map((_, i) => (
            <div
              key={i}
              className="confetti-piece"
              style={{
                '--x': `${Math.random() * 100}%`,
                '--delay': `${Math.random() * 0.5}s`,
                backgroundColor: teamColor || '#6366f1'
              }}
            />
          ))}
        </div>
      )}

      <button
        ref={buttonRef}
        className={`buzz-button ${isPressed ? 'pressed' : ''} ${isDisabled ? 'disabled' : ''} ${showBuzzed ? 'buzzed' : ''}`}
        onClick={handleClick}
        onTouchStart={handleTouchStart}
        disabled={isDisabled}
        style={{
          '--team-color': teamColor || '#6366f1',
          '--team-color-light': `${teamColor || '#6366f1'}40`
        }}
      >
        <span className="buzz-text">{getStatusText()}</span>
      </button>
    </div>
  )
}
