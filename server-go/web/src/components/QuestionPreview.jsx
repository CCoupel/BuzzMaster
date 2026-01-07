import { useMemo, useState, useEffect, useRef } from 'react'
import Timer from './Timer'
import Podium from './Podium'
import './QuestionPreview.css'

const DEFAULT_DURATION = 10

export default function QuestionPreview({ question, gameState, backgrounds = [], teams = {}, bumpers = {} }) {
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
  const isShowingScores = gameState?.remote === 'SCORE'
  const isShowingPlayers = gameState?.remote === 'PLAYERS'

  // Sort teams by score for podium (with rank calculation for ties)
  const sortedTeams = useMemo(() => {
    const sorted = Object.entries(teams)
      .map(([name, data]) => ({
        name,
        score: data.SCORE || 0,
        color: data.COLOR,
      }))
      .sort((a, b) => b.score - a.score)

    // Calculate ranks with tie handling
    let currentRank = 1
    return sorted.map((team, index) => {
      if (index > 0 && sorted[index - 1].score === team.score) {
        return { ...team, rank: currentRank }
      }
      currentRank = index + 1
      return { ...team, rank: currentRank }
    })
  }, [teams])

  // Sort players by score for podium (with rank calculation for ties)
  const sortedPlayers = useMemo(() => {
    const sorted = Object.entries(bumpers)
      .map(([mac, data]) => ({
        mac,
        name: data.NAME || mac.slice(-6),
        score: data.SCORE || 0,
        color: teams[data.TEAM]?.COLOR,
      }))
      .sort((a, b) => b.score - a.score)

    // Calculate ranks with tie handling
    let currentRank = 1
    return sorted.map((player, index) => {
      if (index > 0 && sorted[index - 1].score === player.score) {
        return { ...player, rank: currentRank }
      }
      currentRank = index + 1
      return { ...player, rank: currentRank }
    })
  }, [bumpers, teams])

  const getRgbColor = (color) => {
    if (!color) return 'var(--gray-400)'
    if (Array.isArray(color)) return `rgb(${color.join(',')})`
    return color
  }

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

        {/* Teams ranking with Podium component */}
        {isShowingScores && (
          <div className="tv-preview-scores">
            <h3 className="tv-preview-scores-title">Classement Equipes</h3>
            <div className="tv-preview-scores-layout">
              <Podium teams={sortedTeams} variant="compact" />
              {/* Ranking List */}
              <div className="tv-preview-ranking-list">
                {sortedTeams.map((team) => (
                  <div
                    key={team.name}
                    className="tv-preview-ranking-item"
                    style={{ '--team-color': getRgbColor(team.color) }}
                  >
                    <span className="tv-preview-ranking-rank">
                      {team.rank === 1 ? '🥇' : team.rank === 2 ? '🥈' : team.rank === 3 ? '🥉' : `#${team.rank}`}
                    </span>
                    <div
                      className="tv-preview-ranking-badge"
                      style={{ backgroundColor: getRgbColor(team.color) }}
                    />
                    <span className="tv-preview-ranking-name">{team.name}</span>
                    <span className="tv-preview-ranking-score">{team.score}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}

        {/* Players ranking with Podium component */}
        {isShowingPlayers && (
          <div className="tv-preview-scores">
            <h3 className="tv-preview-scores-title">Classement Joueurs</h3>
            <div className="tv-preview-scores-layout">
              <Podium teams={sortedPlayers} variant="compact" />
              {/* Ranking List */}
              <div className="tv-preview-ranking-list">
                {sortedPlayers.map((player) => (
                  <div
                    key={player.mac}
                    className="tv-preview-ranking-item"
                    style={{ '--team-color': getRgbColor(player.color) }}
                  >
                    <span className="tv-preview-ranking-rank">
                      {player.rank === 1 ? '🥇' : player.rank === 2 ? '🥈' : player.rank === 3 ? '🥉' : `#${player.rank}`}
                    </span>
                    <div
                      className="tv-preview-ranking-badge"
                      style={{ backgroundColor: getRgbColor(player.color) }}
                    />
                    <span className="tv-preview-ranking-name">{player.name}</span>
                    <span className="tv-preview-ranking-score">{player.score}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}
      </div>

      {/* TV Frame indicator */}
      <div className="tv-preview-frame-label">Apercu TV</div>
    </div>
  )
}
