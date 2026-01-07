import { useMemo, useState, useEffect, useRef } from 'react'
import Timer from './Timer'
import './QuestionPreview.css'

const DEFAULT_DURATION = 10

export default function QuestionPreview({ question, gameState, backgrounds = [] }) {
  const [currentBgIndex, setCurrentBgIndex] = useState(0)
  const timerRef = useRef(null)

  // Background cycling
  useEffect(() => {
    if (backgrounds.length <= 1) return

    if (timerRef.current) {
      clearTimeout(timerRef.current)
    }

    const currentBg = backgrounds[currentBgIndex]
    const duration = (currentBg?.duration || DEFAULT_DURATION) * 1000

    timerRef.current = setTimeout(() => {
      setCurrentBgIndex(prev => (prev + 1) % backgrounds.length)
    }, duration)

    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current)
      }
    }
  }, [backgrounds, currentBgIndex])

  // Reset index when backgrounds change
  useEffect(() => {
    if (currentBgIndex >= backgrounds.length) {
      setCurrentBgIndex(0)
    }
  }, [backgrounds, currentBgIndex])

  const currentBg = backgrounds.length > 0 ? backgrounds[currentBgIndex] : null
  const currentBackground = currentBg?.path || null
  const currentOpacity = (currentBg?.opacity ?? 100) / 100

  const phase = gameState?.phase || 'STOP'
  const isShowingGame = gameState?.remote !== 'SCORE' && gameState?.remote !== 'PLAYERS'

  return (
    <div className="tv-preview">
      {/* Background */}
      <div className="tv-preview-background">
        {currentBackground && (
          <div
            className="tv-preview-bg-image"
            style={{
              backgroundImage: `url(${currentBackground})`,
              opacity: currentOpacity
            }}
          />
        )}
        <div className="tv-preview-bg-overlay" />
      </div>

      {/* Timer Bar */}
      {isShowingGame && (
        <div className="tv-preview-timer-bar">
          <Timer
            currentTime={gameState?.timer || 0}
            totalTime={gameState?.totalTime || 30}
            phase={phase}
            size="sm"
          />
        </div>
      )}

      {/* Content */}
      <div className="tv-preview-content">
        {/* Question Display */}
        {question && (
          <div className="tv-preview-question">
            {question.MEDIA && (
              <div className="tv-preview-media-container">
                <img
                  src={question.MEDIA}
                  alt=""
                  className="tv-preview-media"
                />
              </div>
            )}
            <div className="tv-preview-text-container">
              <p className="tv-preview-question-text">{question.QUESTION}</p>
            </div>
          </div>
        )}

        {/* States */}
        {!question && phase === 'STOP' && (
          <div className="tv-preview-state waiting">
            <span className="tv-preview-state-emoji">🎮</span>
            <span className="tv-preview-state-text">En attente...</span>
          </div>
        )}

        {phase === 'PREPARE' && (
          <div className="tv-preview-state prepare">
            <span className="tv-preview-state-emoji">🔔</span>
            <span className="tv-preview-state-text">Preparez-vous !</span>
          </div>
        )}

        {phase === 'READY' && (
          <div className="tv-preview-state ready">
            <span className="tv-preview-state-emoji">✋</span>
            <span className="tv-preview-state-text">PRETS ?</span>
          </div>
        )}

        {/* Scores/Players mode indicator */}
        {gameState?.remote === 'SCORE' && (
          <div className="tv-preview-state scores">
            <span className="tv-preview-state-emoji">🏆</span>
            <span className="tv-preview-state-text">Classement Equipes</span>
          </div>
        )}

        {gameState?.remote === 'PLAYERS' && (
          <div className="tv-preview-state players">
            <span className="tv-preview-state-emoji">👥</span>
            <span className="tv-preview-state-text">Classement Joueurs</span>
          </div>
        )}
      </div>

      {/* TV Frame indicator */}
      <div className="tv-preview-frame-label">Apercu TV</div>
    </div>
  )
}
