import { useState, useEffect, useMemo } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import Card, { CardHeader, CardBody } from '../components/Card'
import './HistoryPage.css'

// Player answer colors
const PLAYER_COLORS = {
  RED: '#ef4444',
  GREEN: '#22c55e',
  YELLOW: '#eab308',
  BLUE: '#3b82f6',
}

export default function HistoryPage() {
  const [history, setHistory] = useState([])
  const [loading, setLoading] = useState(true)
  const [expandedGroups, setExpandedGroups] = useState({})

  // Fetch history from server
  useEffect(() => {
    const fetchHistory = async () => {
      try {
        const response = await fetch('/history')
        if (response.ok) {
          const data = await response.json()
          setHistory(data || [])
        }
      } catch (error) {
        console.error('Failed to fetch history:', error)
      } finally {
        setLoading(false)
      }
    }

    fetchHistory()
    // Refresh every 5 seconds
    const interval = setInterval(fetchHistory, 5000)
    return () => clearInterval(interval)
  }, [])

  // Group events by question
  const groupedHistory = useMemo(() => {
    const groups = {}
    history.forEach(event => {
      const qId = event.QUESTION_ID || 'unknown'
      if (!groups[qId]) {
        groups[qId] = {
          questionId: qId,
          questionText: event.QUESTION_TEXT || `Question #${qId}`,
          events: [],
          firstTimestamp: event.TIMESTAMP
        }
      }
      groups[qId].events.push(event)
      // Track earliest timestamp for sorting
      if (event.TIMESTAMP < groups[qId].firstTimestamp) {
        groups[qId].firstTimestamp = event.TIMESTAMP
      }
    })

    // Calculate summaries for each group
    Object.values(groups).forEach(group => {
      // Sort events by timestamp (newest first within group)
      group.events.sort((a, b) => b.TIMESTAMP - a.TIMESTAMP)

      // Calculate team totals (only TEAM points) and player totals (only PLAYER points)
      const teamTotals = {}
      const playerTotals = {}

      group.events.forEach(event => {
        if (event.WINNER_TYPE === 'TEAM' && event.TEAM_NAME) {
          // Points awarded directly to team
          if (!teamTotals[event.TEAM_NAME]) {
            teamTotals[event.TEAM_NAME] = { points: 0, color: event.TEAM_COLOR }
          }
          teamTotals[event.TEAM_NAME].points += event.POINTS
        } else if (event.WINNER_TYPE === 'PLAYER' && event.PLAYER_NAME) {
          // Points awarded to player
          const key = `${event.TEAM_NAME}|${event.PLAYER_NAME}`
          if (!playerTotals[key]) {
            playerTotals[key] = {
              name: event.PLAYER_NAME,
              team: event.TEAM_NAME,
              points: 0,
              teamColor: event.TEAM_COLOR,
              playerColor: event.PLAYER_COLOR
            }
          }
          playerTotals[key].points += event.POINTS
        }
      })

      group.teamTotals = Object.entries(teamTotals).map(([name, data]) => ({
        name,
        ...data
      })).sort((a, b) => b.points - a.points)

      group.playerTotals = Object.values(playerTotals).sort((a, b) => b.points - a.points)

      group.totalPoints = group.events.reduce((sum, e) => sum + e.POINTS, 0)
    })

    // Sort groups by first timestamp (chronological order)
    return Object.values(groups).sort((a, b) => a.firstTimestamp - b.firstTimestamp)
  }, [history])

  const toggleGroup = (questionId) => {
    setExpandedGroups(prev => ({
      ...prev,
      [questionId]: !prev[questionId]
    }))
  }

  const expandAll = () => {
    const allExpanded = {}
    groupedHistory.forEach(g => allExpanded[g.questionId] = true)
    setExpandedGroups(allExpanded)
  }

  const collapseAll = () => {
    setExpandedGroups({})
  }

  const formatTime = (timestamp) => {
    if (!timestamp) return '-'
    const date = new Date(timestamp / 1000) // Convert microseconds to milliseconds
    return date.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  }

  const formatReactionTime = (reactionTime) => {
    if (!reactionTime) return '-'
    return (reactionTime / 1000000).toFixed(3) + 's'
  }

  const rgbToString = (color) => {
    if (!color || color.length < 3) return null
    return `rgb(${color[0]}, ${color[1]}, ${color[2]})`
  }

  if (loading) {
    return (
      <div className="history-page page">
        <header className="page-header">
          <h1 className="page-title">Historique</h1>
          <p className="page-subtitle">Chargement...</p>
        </header>
      </div>
    )
  }

  return (
    <div className="history-page page">
      <header className="page-header">
        <div className="page-header-row">
          <div>
            <h1 className="page-title">Historique</h1>
            <p className="page-subtitle">
              {history.length} evenement{history.length !== 1 ? 's' : ''} enregistre{history.length !== 1 ? 's' : ''}
            </p>
          </div>
          {groupedHistory.length > 0 && (
            <div className="header-actions">
              <button className="action-btn" onClick={expandAll}>Tout ouvrir</button>
              <button className="action-btn" onClick={collapseAll}>Tout fermer</button>
            </div>
          )}
        </div>
      </header>

      {groupedHistory.length === 0 ? (
        <Card className="empty-state">
          <CardBody>
            <p className="empty-message">Aucun evenement enregistre</p>
            <p className="empty-hint">Les points attribues pendant le jeu apparaitront ici</p>
          </CardBody>
        </Card>
      ) : (
        <div className="history-groups">
          <AnimatePresence>
            {groupedHistory.map((group, groupIndex) => {
              const isExpanded = expandedGroups[group.questionId]

              return (
                <motion.div
                  key={group.questionId}
                  initial={{ opacity: 0, y: 20 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -20 }}
                  transition={{ delay: groupIndex * 0.05 }}
                >
                  <Card className={`history-group ${isExpanded ? 'expanded' : 'collapsed'}`}>
                    <CardHeader onClick={() => toggleGroup(group.questionId)} className="clickable">
                      <div className="group-header">
                        <span className={`collapse-icon ${isExpanded ? 'open' : ''}`}>▶</span>
                        <span className="group-id">#{group.questionId}</span>
                        <span className="group-question">{group.questionText}</span>
                        <span className="group-count">{group.events.length} evt</span>
                        <span className={`group-total ${group.totalPoints >= 0 ? 'positive' : 'negative'}`}>
                          {group.totalPoints >= 0 ? '+' : ''}{group.totalPoints} pts
                        </span>
                      </div>
                    </CardHeader>

                    <AnimatePresence>
                      {!isExpanded && (
                        <motion.div
                          initial={{ height: 0, opacity: 0 }}
                          animate={{ height: 'auto', opacity: 1 }}
                          exit={{ height: 0, opacity: 0 }}
                          transition={{ duration: 0.2 }}
                        >
                          <CardBody className="summary-body">
                            <div className="summary-section">
                              {group.teamTotals.length > 0 && (
                                <div className="summary-row">
                                  <span className="summary-label">Équipes:</span>
                                  <div className="summary-badges">
                                    {group.teamTotals.map(team => {
                                      const color = rgbToString(team.color)
                                      return (
                                        <span
                                          key={team.name}
                                          className="summary-badge team"
                                          style={{
                                            backgroundColor: color ? `${color}20` : undefined,
                                            borderColor: color,
                                            color: color
                                          }}
                                        >
                                          <span className="badge-dot" style={{ backgroundColor: color }} />
                                          {team.name}
                                          <span className="badge-points">
                                            {team.points >= 0 ? '+' : ''}{team.points}
                                          </span>
                                        </span>
                                      )
                                    })}
                                  </div>
                                </div>
                              )}
                              {group.playerTotals.length > 0 && (
                                <div className="summary-row">
                                  <span className="summary-label">Joueurs:</span>
                                  <div className="summary-badges">
                                    {group.playerTotals.map(player => {
                                      const teamColor = rgbToString(player.teamColor)
                                      const playerColor = player.playerColor ? PLAYER_COLORS[player.playerColor] : null
                                      const displayColor = playerColor || teamColor
                                      return (
                                        <span
                                          key={`${player.team}|${player.name}`}
                                          className="summary-badge player"
                                          style={{
                                            backgroundColor: displayColor ? `${displayColor}20` : undefined,
                                            borderColor: displayColor,
                                            color: displayColor
                                          }}
                                        >
                                          {playerColor && <span className="badge-dot" style={{ backgroundColor: playerColor }} />}
                                          {player.name}
                                          <span className="badge-points">
                                            {player.points >= 0 ? '+' : ''}{player.points}
                                          </span>
                                        </span>
                                      )
                                    })}
                                  </div>
                                </div>
                              )}
                            </div>
                          </CardBody>
                        </motion.div>
                      )}
                    </AnimatePresence>

                    <AnimatePresence>
                      {isExpanded && (
                        <motion.div
                          initial={{ height: 0, opacity: 0 }}
                          animate={{ height: 'auto', opacity: 1 }}
                          exit={{ height: 0, opacity: 0 }}
                          transition={{ duration: 0.2 }}
                        >
                          <CardBody>
                            <table className="history-table">
                              <thead>
                                <tr>
                                  <th>Heure</th>
                                  <th>Equipe</th>
                                  <th>Joueur</th>
                                  <th>Temps</th>
                                  <th>Points</th>
                                </tr>
                              </thead>
                              <tbody>
                                {group.events.map((event, eventIndex) => {
                                  const teamColor = rgbToString(event.TEAM_COLOR)
                                  const playerColor = event.PLAYER_COLOR ? PLAYER_COLORS[event.PLAYER_COLOR] : null

                                  return (
                                    <motion.tr
                                      key={`${event.TIMESTAMP}-${eventIndex}`}
                                      initial={{ opacity: 0, x: -10 }}
                                      animate={{ opacity: 1, x: 0 }}
                                      transition={{ delay: eventIndex * 0.03 }}
                                      className={`event-row ${event.WINNER_TYPE?.toLowerCase()}`}
                                    >
                                      <td className="time-cell">{formatTime(event.TIMESTAMP)}</td>
                                      <td className="team-cell">
                                        {event.TEAM_NAME && (
                                          <span
                                            className="team-badge"
                                            style={{
                                              backgroundColor: teamColor ? `${teamColor}20` : undefined,
                                              borderColor: teamColor,
                                              color: teamColor
                                            }}
                                          >
                                            <span
                                              className="team-dot"
                                              style={{ backgroundColor: teamColor }}
                                            />
                                            {event.TEAM_NAME}
                                          </span>
                                        )}
                                      </td>
                                      <td className="player-cell">
                                        {event.WINNER_TYPE === 'PLAYER' && event.PLAYER_NAME && (
                                          <span
                                            className="player-badge"
                                            style={{
                                              backgroundColor: playerColor ? `${playerColor}20` : undefined,
                                              borderColor: playerColor || teamColor,
                                              color: playerColor || teamColor
                                            }}
                                          >
                                            {playerColor && (
                                              <span
                                                className="player-dot"
                                                style={{ backgroundColor: playerColor }}
                                              />
                                            )}
                                            {event.PLAYER_NAME}
                                          </span>
                                        )}
                                      </td>
                                      <td className="reaction-cell">{formatReactionTime(event.REACTION_TIME)}</td>
                                      <td className="points-cell">
                                        <span className={`points-badge ${event.POINTS >= 0 ? 'positive' : 'negative'}`}>
                                          {event.POINTS >= 0 ? '+' : ''}{event.POINTS} pts
                                        </span>
                                      </td>
                                    </motion.tr>
                                  )
                                })}
                              </tbody>
                            </table>
                          </CardBody>
                        </motion.div>
                      )}
                    </AnimatePresence>
                  </Card>
                </motion.div>
              )
            })}
          </AnimatePresence>
        </div>
      )}
    </div>
  )
}
