import { useState, useMemo } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { useGame } from '../hooks/GameContext'
import Button from '../components/Button'
import Card, { CardHeader, CardBody } from '../components/Card'
import Timer from '../components/Timer'
import TeamCard from '../components/TeamCard'
import QuestionPreview from '../components/QuestionPreview'
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

  // Sort questions by ID
  const sortedQuestions = useMemo(() => {
    return Object.values(questions)
      .filter(q => q && q.ID)
      .sort((a, b) => parseInt(a.ID) - parseInt(b.ID))
  }, [questions])

  const handleStartStop = () => {
    if (gameState.phase === 'STOP' || gameState.phase === 'READY') {
      startGame(timeInput, pointsInput)
    } else {
      stopGame()
    }
  }

  const handlePauseContinue = () => {
    if (gameState.phase === 'PAUSE') {
      continueGame()
    } else {
      pauseGame()
    }
  }

  const handleQuestionSelect = (question) => {
    if (['STOP', 'PREPARE', 'READY'].includes(gameState.phase)) {
      selectQuestion(question.ID)
      setTimeInput(parseInt(question.TIME) || 30)
      setPointsInput(parseInt(question.POINTS) || 1)
    }
  }

  const handleBumperClick = (bumperMac) => {
    if (gameState.phase === 'STOP') {
      setBumperPoints(bumperMac, pointsInput)
    }
  }

  const isPlaying = gameState.phase === 'START' || gameState.phase === 'PAUSE'
  const canSelectQuestion = ['STOP', 'PREPARE', 'READY'].includes(gameState.phase)

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
              onClick={() => handleQuestionSelect(question)}
              whileHover={canSelectQuestion ? { scale: 1.01 } : undefined}
              whileTap={canSelectQuestion ? { scale: 0.99 } : undefined}
              style={{ cursor: canSelectQuestion ? 'pointer' : 'not-allowed' }}
            >
              <div className="preview-header">
                <span className="preview-id">#{question.ID}</span>
                <span className="preview-status">{question.STATUS || 'AVAILABLE'}</span>
              </div>
              {question.MEDIA && (
                <div className="preview-image">
                  <img src={question.MEDIA} alt="" />
                </div>
              )}
              <div className="preview-content">
                <p className="preview-question">{question.QUESTION}</p>
                <p className="preview-answer">{question.ANSWER}</p>
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
                disabled={gameState.phase === 'PREPARE'}
              >
                {isPlaying ? 'STOP' : 'START'}
              </Button>

              <Button
                variant={gameState.phase === 'PAUSE' ? 'primary' : 'warning'}
                size="lg"
                onClick={handlePauseContinue}
                disabled={!isPlaying}
              >
                {gameState.phase === 'PAUSE' ? 'CONTINUER' : 'PAUSE'}
              </Button>
            </div>

            <Button
              variant="secondary"
              size="md"
              onClick={revealAnswer}
              disabled={gameState.phase !== 'STOP'}
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

        {/* TV Preview */}
        <div className="tv-preview-container">
          <QuestionPreview
            question={gameState.question}
            gameState={gameState}
            backgrounds={gameState.backgrounds || []}
          />
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
