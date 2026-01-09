import { useState, useMemo } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { useGame } from '../hooks/GameContext'
import Button from '../components/Button'
import Card, { CardHeader, CardBody } from '../components/Card'
import Timer from '../components/Timer'
import TeamCard from '../components/TeamCard'
import QuestionPreview from '../components/QuestionPreview'
import './GamePage.css'

// QCM answer colors
const QCM_COLORS = {
  RED: { label: 'Rouge', color: '#ef4444', letter: 'A' },
  GREEN: { label: 'Vert', color: '#22c55e', letter: 'B' },
  YELLOW: { label: 'Jaune', color: '#eab308', letter: 'C' },
  BLUE: { label: 'Bleu', color: '#3b82f6', letter: 'D' },
}

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
    forceReady,
  } = useGame()

  const [timeInput, setTimeInput] = useState(30)
  const [pointsInput, setPointsInput] = useState(1)

  // Group bumpers by team
  const teamBumpers = useMemo(() => {
    const grouped = {}
    Object.entries(bumpers).forEach(([mac, bumper]) => {
      const teamName = bumper.TEAM || 'Sans equipe'
      if (!grouped[teamName]) grouped[teamName] = []
      grouped[teamName].push({
        mac,
        name: bumper.NAME,
        timestamp: bumper.TIMESTAMP,
        button: bumper.BUTTON,
        ready: bumper.READY === 'TRUE',
        active: bumper.TIMESTAMP !== undefined || bumper.BUTTON !== undefined,
        answerColor: bumper.ANSWER_COLOR,
      })
    })
    return grouped
  }, [bumpers])

  // Sort teams by timestamp (first to buzz)
  const sortedTeams = useMemo(() => {
    return Object.entries(teams)
      .map(([name, data]) => ({
        name,
        ...data,
        buzzers: teamBumpers[name] || [],
      }))
      .sort((a, b) => {
        const timeA = a.TIMESTAMP ?? Infinity
        const timeB = b.TIMESTAMP ?? Infinity
        return timeA - timeB
      })
  }, [teams, teamBumpers])

  // Sort questions by ORDER if available, otherwise by ID
  const sortedQuestions = useMemo(() => {
    return Object.values(questions)
      .filter(q => q && q.ID)
      .sort((a, b) => { const orderA = a.ORDER !== undefined ? parseInt(a.ORDER) : parseInt(a.ID); const orderB = b.ORDER !== undefined ? parseInt(b.ORDER) : parseInt(b.ID); return orderA - orderB })
  }, [questions])

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

  const handleBumperClick = (bumperMac) => {
    if (gameState.phase === 'STOPPED' || gameState.phase === 'REVEALED') {
      setBumperPoints(bumperMac, pointsInput)
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
      {/* Timer - Full Width Top */}
      <div className="timer-section">
        <Card variant="elevated" padding="lg" className="timer-card">
          <Timer
            currentTime={gameState.timer}
            totalTime={gameState.totalTime}
            phase={gameState.phase}
            size="lg"
          />
        </Card>
      </div>

      {/* Questions Panel - Left */}
      <div className="questions-panel">
        <h2 className="panel-title">Questions</h2>
        <div className="questions-list">
          {sortedQuestions.map((question) => (
            <motion.div
              key={question.ID}
              className={`question-preview ${question.STATUS?.toLowerCase() || 'available'} ${
                gameState.question?.ID === question.ID ? 'selected' : ''
              }`}
              onClick={(e) => handleQuestionSelect(question, e.ctrlKey)}
              whileHover={canSelectQuestion ? { scale: 1.01 } : undefined}
              whileTap={canSelectQuestion ? { scale: 0.99 } : undefined}
              style={{ cursor: canSelectQuestion ? 'pointer' : 'not-allowed' }}
            >
              <div className="preview-header">
                <span className="preview-id">#{question.ID}</span>
                {question.TYPE === 'QCM' && (
                  <span className="preview-qcm-badge">QCM</span>
                )}
                <span className="preview-status">{question.STATUS || 'AVAILABLE'}</span>
              </div>
              {question.MEDIA && (
                <div className="preview-image">
                  <img src={question.MEDIA} alt="" />
                </div>
              )}
              <div className="preview-content">
                <p className="preview-question">{question.QUESTION}</p>
                {question.TYPE === 'QCM' && question.QCM_CORRECT && QCM_COLORS[question.QCM_CORRECT] ? (
                  <p
                    className="preview-answer preview-answer-qcm"
                    style={{ backgroundColor: QCM_COLORS[question.QCM_CORRECT].color }}
                  >
                    <span className="qcm-letter">{QCM_COLORS[question.QCM_CORRECT].letter}</span>
                    {question.ANSWER}
                  </p>
                ) : (
                  <p className="preview-answer">{question.ANSWER}</p>
                )}
              </div>
              <div className="preview-meta">
                <span className="preview-time">{question.TIME}s</span>
                <span className="preview-points">{question.POINTS} pts</span>
              </div>
            </motion.div>
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
              <label htmlFor="points-input">Points</label>
              <input
                id="points-input"
                type="number"
                value={pointsInput}
                onChange={(e) => setPointsInput(parseInt(e.target.value) || 1)}
                min="1"
                max="100"
              />
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

          <div className="display-toggle">
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
            </div>
          </div>
        </Card>

        {/* TV Preview (iframe to /tv) */}
        <div className="tv-preview-container">
          <QuestionPreview />
        </div>
      </div>

      {/* Right Panel - Teams */}
      <div className="right-panel">
        <div className="teams-section">
          <h2 className="section-title">Equipes</h2>
          <div className="teams-grid">
            <AnimatePresence>
              {sortedTeams.map((team, index) => (
                <motion.div
                  key={team.name}
                  className="team-card-wrapper"
                  initial={{ opacity: 0, scale: 0.9 }}
                  animate={{ opacity: 1, scale: 1 }}
                  transition={{ delay: index * 0.1 }}
                >
                  <TeamCard
                    name={team.name}
                    color={team.COLOR}
                    score={team.SCORE || 0}
                    ready={team.READY === 'TRUE'}
                    active={team.TIMESTAMP !== undefined}
                    timestamp={team.TIMESTAMP}
                    gameTime={gameState.gameTime}
                    waitingForReady={['PREPARE', 'READY'].includes(gameState.phase)}
                    buzzers={team.buzzers.map(b => ({
                      ...b,
                      onClick: () => handleBumperClick(b.mac)
                    }))}
                  />
                </motion.div>
              ))}
            </AnimatePresence>
          </div>
        </div>
      </div>
    </div>
  )
}
