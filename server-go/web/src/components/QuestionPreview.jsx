import { useMemo, useState, useEffect, useRef } from 'react'
import Timer from './Timer'
import Podium from './Podium'
import './QuestionPreview.css'

const DEFAULT_DURATION = 10

// QCM answer colors
const QCM_COLORS = {
  RED: { label: 'Rouge', color: '#ef4444', letter: 'A' },
  GREEN: { label: 'Vert', color: '#22c55e', letter: 'B' },
  YELLOW: { label: 'Jaune', color: '#eab308', letter: 'C' },
  BLUE: { label: 'Bleu', color: '#3b82f6', letter: 'D' },
}

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

  const phase = gameState?.phase || 'STOPPED'
  const isShowingGame = gameState?.remote !== 'SCORE' && gameState?.remote !== 'PLAYERS'
  const isShowingScores = gameState?.remote === 'SCORE'
  const isShowingPlayers = gameState?.remote === 'PLAYERS'
  
  // Question should only be visible during STARTED, PAUSED, STOPPED, or REVEALED
  const showQuestionContent = ['STARTED', 'PAUSED', 'STOPPED', 'REVEALED'].includes(phase)
  // QCM answers visible from READY phase
  const showQcmAnswers = ['READY', 'STARTED', 'PAUSED', 'STOPPED', 'REVEALED'].includes(phase)
  const isQcm = question?.TYPE === 'QCM'
  const isRevealed = phase === 'REVEALED'

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

      {/* 4-Zone Layout Content */}
      <div className="tv-preview-zones">
        {/* Zone 1: Timer - at top, compact */}
        {isShowingGame && (
          <div className="tv-zone-timer">
            <Timer
              currentTime={gameState?.timer || 0}
              totalTime={gameState?.totalTime || 30}
              phase={phase}
              size="sm"
            />
          </div>
        )}

        {/* Zone 2: Question - close to timer, bigger text */}
        {question && showQuestionContent && (
          <div className="tv-zone-question">
            <p className="tv-zone-question-text">{question.QUESTION}</p>
          </div>
        )}

        {/* Zone 3: Media (or spacer if no media) */}
        {question && showQuestionContent && (
          question.MEDIA ? (
            <div className="tv-zone-media">
              <img
                src={question.MEDIA}
                alt=""
                className="tv-zone-media-img"
              />
            </div>
          ) : (
            <div className="tv-zone-media-spacer" />
          )
        )}

        {/* Zone 4: Answers - fixed at bottom */}
        {question && showQuestionContent && (
          <div className="tv-zone-answers">
            {/* QCM Answers */}
            {isQcm && question.QCM_ANSWERS && (
              <div className="tv-zone-qcm-grid">
                {Object.entries(QCM_COLORS).map(([colorKey, colorData]) => {
                  const answer = question.QCM_ANSWERS[colorKey]
                  if (!answer) return null
                  const isCorrect = question.QCM_CORRECT === colorKey

                  return (
                    <div
                      key={colorKey}
                      className={`tv-zone-qcm-item ${isRevealed ? (isCorrect ? 'correct' : 'wrong') : ''}`}
                      style={{
                        backgroundColor: isRevealed && !isCorrect ? '#6b7280' : colorData.color,
                        opacity: isRevealed && !isCorrect ? 0.5 : 1
                      }}
                    >
                      <span className="tv-zone-qcm-letter">{colorData.letter}</span>
                      <span className="tv-zone-qcm-text">{answer}</span>
                    </div>
                  )
                })}
              </div>
            )}

            {/* Answer for non-QCM in REVEALED phase */}
            {!isQcm && isRevealed && question.ANSWER && (
              <div className="tv-zone-answer">
                <p className="tv-zone-answer-text">{question.ANSWER}</p>
              </div>
            )}
          </div>
        )}

        {/* QCM Answers in READY phase (without question) - uses zone layout */}
        {question && isQcm && showQcmAnswers && !showQuestionContent && question.QCM_ANSWERS && (
          <div className="tv-zone-qcm-ready">
            {Object.entries(QCM_COLORS).map(([colorKey, colorData]) => {
              const answer = question.QCM_ANSWERS[colorKey]
              if (!answer) return null

              return (
                <div
                  key={colorKey}
                  className="tv-zone-qcm-item"
                  style={{ backgroundColor: colorData.color }}
                >
                  <span className="tv-zone-qcm-letter">{colorData.letter}</span>
                  <span className="tv-zone-qcm-text">{answer}</span>
                </div>
              )
            })}
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
