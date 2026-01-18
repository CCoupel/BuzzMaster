import { useState, useMemo } from 'react'
import { useGame } from '../hooks/GameContext'
import Button from '../components/Button'
import Card from '../components/Card'
import Timer from '../components/Timer'
import TeamCard from '../components/TeamCard'
import QuestionPreview from '../components/QuestionPreview'
import QuestionCard, { CATEGORIES } from '../components/QuestionCard'
import './GamePage.css'

export default function GamePage() {
  const {
    gameState,
    teams,
    bumpers,
    questions,
    startGame,
    stopGame,
    pauseGame,
    continueGame,
    revealAnswer,
    selectQuestion,
    setRemoteDisplay,
    setBumperPoints,
    setTeamPoints,
    forceReady,
    simulateButton,
    simulatePong,
  } = useGame()

  const [timeInput, setTimeInput] = useState(30)
  const [pointsInput, setPointsInput] = useState(1)

  // Group bumpers by team and sort by timestamp
  const teamBumpers = useMemo(() => {
    const grouped = {}
    Object.entries(bumpers).forEach(([mac, bumper]) => {
      const teamName = bumper.TEAM || 'Sans equipe'
      if (!grouped[teamName]) grouped[teamName] = []
      grouped[teamName].push({
        mac,
        name: bumper.NAME,
        score: bumper.SCORE || 0,
        timestamp: bumper.TIME,
        button: bumper.BUTTON,
        ready: bumper.READY === true || bumper.READY === 'TRUE',
        active: bumper.TIME !== undefined && bumper.TIME > 0,
        answerColor: bumper.ANSWER_COLOR,
      })
    })
    // Sort bumpers by timestamp within each team
    Object.values(grouped).forEach(bumperList => {
      bumperList.sort((a, b) => {
        const timeA = a.timestamp ?? Infinity
        const timeB = b.timestamp ?? Infinity
        return timeA - timeB
      })
    })
    return grouped
  }, [bumpers])

  // Sort teams by total score (descending), then by timestamp if tied
  const sortedTeams = useMemo(() => {
    return Object.entries(teams)
      .map(([name, data]) => ({
        name,
        ...data,
        buzzers: teamBumpers[name] || [],
      }))
      .sort((a, b) => {
        const scoreA = a.SCORE ?? 0
        const scoreB = b.SCORE ?? 0
        if (scoreB !== scoreA) return scoreB - scoreA  // Higher score first
        // If tied, sort by timestamp (earlier first)
        const timeA = a.TIME ?? Infinity
        const timeB = b.TIME ?? Infinity
        return timeA - timeB
      })
  }, [teams, teamBumpers])

  // Sort questions by ORDER if available, otherwise by ID
  const sortedQuestions = useMemo(() => {
    return Object.values(questions)
      .filter(q => q && q.ID)
      .sort((a, b) => { const orderA = a.ORDER !== undefined ? parseInt(a.ORDER) : parseInt(a.ID); const orderB = b.ORDER !== undefined ? parseInt(b.ORDER) : parseInt(b.ID); return orderA - orderB })
  }, [questions])

  // Calculate Memory score based on matched pairs, errors, and config
  const memoryScore = useMemo(() => {
    if (gameState.question?.TYPE !== 'MEMORY') return null

    const config = gameState.question.MEMORY_CONFIG || {}
    const pointsPerPair = config.POINTS_PER_PAIR || 10
    const errorPenalty = config.ERROR_PENALTY || 0
    const completionBonus = config.COMPLETION_BONUS || 0

    const matchedPairs = gameState.memoryMatchedPairs?.length || 0
    const totalPairs = gameState.question.MEMORY_PAIRS?.length || 0
    const errors = gameState.memoryErrors || 0
    const isComplete = matchedPairs === totalPairs && totalPairs > 0

    let score = matchedPairs * pointsPerPair
    if (isComplete) score += completionBonus
    score -= errors * errorPenalty
    if (score < 0) score = 0

    return { score, matchedPairs, totalPairs, errors, isComplete, pointsPerPair, errorPenalty, completionBonus }
  }, [gameState.question, gameState.memoryMatchedPairs, gameState.memoryErrors])

  // Calculate QCM penalty based on invalidated answers (hints given)
  const qcmPenalty = useMemo(() => {
    // Only apply for QCM questions with hints enabled
    if (gameState.question?.TYPE !== 'QCM' || !gameState.question?.QCM_HINTS_ENABLED) return null

    const invalidatedCount = gameState.qcmInvalidated?.length || 0
    if (invalidatedCount === 0) return null

    // Use configurable penalties from question, with defaults
    const penalty1 = gameState.question?.QCM_PENALTY_1 || 0.67
    const penalty2 = gameState.question?.QCM_PENALTY_2 || 0.33

    let multiplier = 1
    if (invalidatedCount === 1) multiplier = penalty1
    else if (invalidatedCount >= 2) multiplier = penalty2

    // Ensure effective points is at least 1 (never 0)
    const effectivePoints = Math.max(1, Math.round(pointsInput * multiplier))
    const penaltyPercent = Math.round(multiplier * 100)

    return { invalidatedCount, multiplier, effectivePoints, penaltyPercent }
  }, [gameState.question, gameState.qcmInvalidated, pointsInput])

  const handleStartStop = () => {
    if (gameState.phase === 'READY') {
      startGame(timeInput, pointsInput)
    } else if (gameState.phase === 'STARTED' || gameState.phase === 'PAUSED') {
      stopGame()
    }
  }

  const handlePauseContinue = () => {
    if (gameState.phase === 'PAUSED') {
      continueGame()
    } else if (gameState.phase === 'STARTED') {
      pauseGame()
    }
  }

  const handleQuestionSelect = (question, ctrlKey = false) => {
    if (['STOPPED', 'REVEALED', 'PREPARE', 'READY'].includes(gameState.phase)) {
      if (ctrlKey) {
        // Ctrl+click: select and force to READY (debug)
        selectQuestion(question.ID)
        setTimeInput(parseInt(question.TIME) || 30)
        setPointsInput(parseInt(question.POINTS) || 1)
        forceReady()
        return
      }
      selectQuestion(question.ID)
      setTimeInput(parseInt(question.TIME) || 30)
      setPointsInput(parseInt(question.POINTS) || 1)
    }
  }

  const handleBumperClick = (bumperMac, ctrlKey = false) => {
    if (ctrlKey && gameState.phase === 'PREPARE') {
      // Ctrl+click in PREPARE: simulate PONG response (debug)
      simulatePong(bumperMac)
      return
    }
    if (ctrlKey && ['STARTED', 'PAUSED'].includes(gameState.phase)) {
      // Ctrl+click: simulate buzzer button press (debug)
      simulateButton(bumperMac, 'A')
      return
    }
    if (gameState.phase === 'STOPPED' || gameState.phase === 'REVEALED') {
      // Only allow points for players who have buzzed
      const bumper = bumpers[bumperMac]
      if (bumper?.TIME && bumper.TIME > 0) {
        // For Memory questions, use calculated score
        // For QCM with hints, use penalty-adjusted points
        // Otherwise use pointsInput
        let pointsToAward = pointsInput
        if (memoryScore) {
          pointsToAward = memoryScore.score
        } else if (qcmPenalty) {
          pointsToAward = qcmPenalty.effectivePoints
        }
        // Check POINTS_TARGET: if TEAM, give points to team instead of player
        if (gameState.question?.POINTS_TARGET === 'TEAM') {
          const teamName = bumper.TEAM
          if (teamName) {
            setTeamPoints(teamName, pointsToAward)
          }
        } else {
          setBumperPoints(bumperMac, pointsToAward)
        }
      }
    }
  }

  const isPlaying = gameState.phase === 'STARTED' || gameState.phase === 'PAUSED'
  const canSelectQuestion = ['STOPPED', 'REVEALED', 'PREPARE', 'READY'].includes(gameState.phase)
  // REPONSE button active only in STOPPED phase after a question was played
  const canReveal = gameState.phase === 'STOPPED' && gameState.question?.STATUS === 'STOPPED'
  // START button only active in READY phase
  const canStart = gameState.phase === 'READY'

  return (
    <div className="game-page page">
      {/* Timer + Display Section (stacked vertically) */}
      <div className="timer-display-section">
        <Card variant="elevated" padding="md" className="timer-card">
          <Timer
            currentTime={gameState.timer}
            totalTime={gameState.totalTime}
            phase={gameState.phase}
            size="lg"
            showPhase={false}
          />
        </Card>
        <Card variant="elevated" padding="sm" className="display-card">
          <span className="toggle-label">Affichage TV:</span>
          <div className="toggle-buttons">
            <Button
              variant={gameState.remote === 'GAME' ? 'primary' : 'ghost'}
              size="sm"
              onClick={() => setRemoteDisplay('GAME')}
            >
              Jeu
            </Button>
            <Button
              variant={gameState.remote === 'SCORE' ? 'primary' : 'ghost'}
              size="sm"
              onClick={() => setRemoteDisplay('SCORE')}
            >
              Equipes
            </Button>
            <Button
              variant={gameState.remote === 'PLAYERS' ? 'primary' : 'ghost'}
              size="sm"
              onClick={() => setRemoteDisplay('PLAYERS')}
            >
              Joueurs
            </Button>
            <Button
              variant={gameState.remote === 'PALMARES' ? 'primary' : 'ghost'}
              size="sm"
              onClick={() => setRemoteDisplay('PALMARES')}
            >
              Palmares
            </Button>
          </div>
          <div className="phase-badge-container">
            {gameState.phase === 'STOPPED' && <span className="phase-badge phase-stopped">ARRET</span>}
            {gameState.phase === 'PAUSED' && <span className="phase-badge phase-paused">PAUSE</span>}
            {gameState.phase === 'STARTED' && <span className="phase-badge phase-running">EN COURS</span>}
            {gameState.phase === 'PREPARE' && <span className="phase-badge phase-prepare">PREPARATION</span>}
            {gameState.phase === 'READY' && <span className="phase-badge phase-ready">PRET</span>}
            {gameState.phase === 'REVEALED' && <span className="phase-badge phase-revealed">REPONSE</span>}
          </div>
          <div className="question-indicators">
            {gameState.question?.CATEGORY && CATEGORIES[gameState.question.CATEGORY] && (
              <div
                className="category-indicator"
                style={{ backgroundColor: CATEGORIES[gameState.question.CATEGORY].color }}
                title={CATEGORIES[gameState.question.CATEGORY].label}
              >
                <span>{CATEGORIES[gameState.question.CATEGORY].icon}</span>
              </div>
            )}
            {gameState.question?.POINTS_TARGET && (
              <div className={`points-target-indicator ${gameState.question.POINTS_TARGET.toLowerCase()}`} title={gameState.question.POINTS_TARGET === 'TEAM' ? 'Points à l\'équipe' : 'Points au joueur'}>
                {gameState.question.POINTS_TARGET === 'TEAM' ? (
                  <>
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                      <circle cx="9" cy="7" r="4"/>
                      <path d="M17 11c1.66 0 2.99-1.34 2.99-3S18.66 5 17 5c-.32 0-.63.05-.91.14.57.81.9 1.79.9 2.86s-.34 2.04-.9 2.86c.28.09.59.14.91.14z"/>
                      <path d="M3 18v-1c0-2.66 5.33-4 8-4s8 1.34 8 4v1H3z"/>
                      <path d="M17 13c2.05.26 5 1.22 5 3v1h-3v-1.5c0-1.19-.68-2.14-2-2.5z"/>
                    </svg>
                    <span>Equipe</span>
                  </>
                ) : (
                  <>
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                      <circle cx="12" cy="7" r="4"/>
                      <path d="M12 14c-4 0-8 2-8 4v2h16v-2c0-2-4-4-8-4z"/>
                    </svg>
                    <span>Joueur</span>
                  </>
                )}
              </div>
            )}
          </div>
          {/* Memory Stats - inline in display bar (Paires & Erreurs only, Points in input) */}
          {memoryScore && (
            <div className="memory-admin-stats">
              <div className="memory-admin-stat">
                <span className="memory-admin-stat-label">Paires</span>
                <span className={`memory-admin-stat-value ${memoryScore.isComplete ? 'success' : ''}`}>
                  {memoryScore.matchedPairs}/{memoryScore.totalPairs}
                  {memoryScore.isComplete && ' ✓'}
                </span>
              </div>
              <div className="memory-admin-stat">
                <span className="memory-admin-stat-label">Erreurs</span>
                <span className={`memory-admin-stat-value ${memoryScore.errors > 0 ? 'error' : ''}`}>
                  {memoryScore.errors}
                </span>
              </div>
            </div>
          )}
        </Card>
      </div>

      {/* Questions Panel - Left */}
      <div className="questions-panel">
        <h2 className="panel-title">Questions</h2>
        <div className="questions-list">
          {sortedQuestions.map((question) => (
            <QuestionCard
              key={question.ID}
              question={question}
              selected={gameState.question?.ID === question.ID}
              compact
              showStatus
              showTarget
              canSelect={canSelectQuestion}
              onClick={handleQuestionSelect}
            />
          ))}
        </div>
      </div>

      {/* Control Panel - Center */}
      <div className="control-panel">
        <Card variant="elevated" padding="lg" className="controls-card">
          <div className="control-inputs">
            <div className="input-group">
              <label htmlFor="time-input">Temps (sec)</label>
              <input
                id="time-input"
                type="number"
                value={timeInput}
                onChange={(e) => setTimeInput(parseInt(e.target.value) || 30)}
                min="1"
                max="300"
                disabled={isPlaying}
              />
            </div>
            <div className="input-group">
              <label htmlFor="points-input">
                Points
                {qcmPenalty && (
                  <span className="qcm-penalty-badge" title={`${qcmPenalty.invalidatedCount} indice(s) donne(s) - penalite ${100 - qcmPenalty.penaltyPercent}%`}>
                    {qcmPenalty.penaltyPercent}%
                  </span>
                )}
              </label>
              {memoryScore ? (
                <input
                  id="points-input"
                  type="number"
                  value={memoryScore.score}
                  readOnly
                  className="memory-score-input"
                  title={`${memoryScore.matchedPairs}×${memoryScore.pointsPerPair}${memoryScore.isComplete ? ` +${memoryScore.completionBonus}` : ''}${memoryScore.errors > 0 ? ` -${memoryScore.errors}×${memoryScore.errorPenalty}` : ''}`}
                />
              ) : qcmPenalty ? (
                <input
                  id="points-input"
                  type="number"
                  value={qcmPenalty.effectivePoints}
                  readOnly
                  className="qcm-penalty-input"
                  title={`Base: ${pointsInput} pts × ${qcmPenalty.penaltyPercent}% = ${qcmPenalty.effectivePoints} pts`}
                />
              ) : (
                <input
                  id="points-input"
                  type="number"
                  value={pointsInput}
                  onChange={(e) => setPointsInput(parseInt(e.target.value) || 1)}
                  min="1"
                  max="100"
                />
              )}
            </div>
          </div>

          <div className="control-buttons">
            <div className="control-buttons-row">
              <Button
                variant={isPlaying ? 'danger' : 'success'}
                size="lg"
                onClick={handleStartStop}
                disabled={!canStart && !isPlaying}
              >
                {isPlaying ? 'STOP' : 'START'}
              </Button>

              <Button
                variant={gameState.phase === 'PAUSED' ? 'primary' : 'warning'}
                size="lg"
                onClick={handlePauseContinue}
                disabled={!isPlaying}
              >
                {gameState.phase === 'PAUSED' ? 'CONTINUER' : 'PAUSE'}
              </Button>
            </div>

            <Button
              variant="secondary"
              size="md"
              onClick={revealAnswer}
              disabled={!canReveal}
            >
              REPONSE
            </Button>
          </div>
        </Card>
      </div>

      {/* TV Preview (Row 2 Col 2) */}
      <div className="tv-preview-container">
        <QuestionPreview />
      </div>

      {/* Right Panel - Teams */}
      <div className="right-panel">
        <div className="teams-section">
          <h2 className="section-title">Equipes</h2>
          <div className="teams-grid">
            {sortedTeams.map((team) => (
              <TeamCard
                key={team.name}
                name={team.name}
                color={team.COLOR}
                score={team.SCORE || 0}
                teamPoints={team.TEAM_POINTS || 0}
                ready={team.READY === true || team.READY === 'TRUE'}
                active={team.TIME !== undefined && team.TIME > 0}
                timestamp={team.TIME}
                gameTime={gameState.gameTime}
                waitingForReady={['PREPARE', 'READY'].includes(gameState.phase)}
                waitingForBuzz={['STARTED', 'PAUSED'].includes(gameState.phase)}
                onTeamClick={(teamName) => {
                  if (['STOPPED', 'REVEALED'].includes(gameState.phase)) {
                    // For Memory questions, use calculated score
                    // For QCM with hints, use penalty-adjusted points
                    // Otherwise use pointsInput
                    let pointsToAward = pointsInput
                    if (memoryScore) {
                      pointsToAward = memoryScore.score
                    } else if (qcmPenalty) {
                      pointsToAward = qcmPenalty.effectivePoints
                    }
                    setTeamPoints(teamName, pointsToAward)
                  }
                }}
                onPlayerClick={(bumperMac, ctrlKey) => handleBumperClick(bumperMac, ctrlKey)}
                buzzers={team.buzzers.map(b => ({
                  ...b,
                  onClick: (e) => handleBumperClick(b.mac, e?.ctrlKey)
                }))}
              />
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}

