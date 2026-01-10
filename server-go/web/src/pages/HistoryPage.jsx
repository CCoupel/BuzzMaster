import { useState, useEffect, useMemo } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import Card, { CardHeader, CardBody } from '../components/Card'
import './HistoryPage.css'

export default function HistoryPage() {
  const [history, setHistory] = useState([])
  const [loading, setLoading] = useState(true)

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
          events: []
        }
      }
      groups[qId].events.push(event)
    })
    // Sort events within each group by timestamp (newest first)
    Object.values(groups).forEach(group => {
      group.events.sort((a, b) => b.TIMESTAMP - a.TIMESTAMP)
    })
    return Object.values(groups)
  }, [history])

  const formatTime = (timestamp) => {
    if (!timestamp) return '-'
    const date = new Date(timestamp / 1000) // Convert microseconds to milliseconds
    return date.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  }

  const formatReactionTime = (reactionTime) => {
    if (!reactionTime) return '-'
    return (reactionTime / 1000000).toFixed(3) + 's'
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
        <h1 className="page-title">Historique</h1>
        <p className="page-subtitle">
          {history.length} evenement{history.length !== 1 ? 's' : ''} enregistre{history.length !== 1 ? 's' : ''}
        </p>
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
            {groupedHistory.map((group, groupIndex) => (
              <motion.div
                key={group.questionId}
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -20 }}
                transition={{ delay: groupIndex * 0.1 }}
              >
                <Card className="history-group">
                  <CardHeader>
                    <div className="group-header">
                      <span className="group-id">#{group.questionId}</span>
                      <span className="group-question">{group.questionText}</span>
                      <span className="group-count">{group.events.length} evt</span>
                    </div>
                  </CardHeader>
                  <CardBody>
                    <table className="history-table">
                      <thead>
                        <tr>
                          <th>Heure</th>
                          <th>Gagnant</th>
                          <th>Type</th>
                          <th>Temps</th>
                          <th>Points</th>
                        </tr>
                      </thead>
                      <tbody>
                        {group.events.map((event, eventIndex) => (
                          <motion.tr
                            key={`${event.TIMESTAMP}-${eventIndex}`}
                            initial={{ opacity: 0, x: -10 }}
                            animate={{ opacity: 1, x: 0 }}
                            transition={{ delay: eventIndex * 0.05 }}
                            className={`event-row ${event.WINNER_TYPE?.toLowerCase()}`}
                          >
                            <td className="time-cell">{formatTime(event.TIMESTAMP)}</td>
                            <td className="winner-cell">
                              <span className="winner-name">{event.WINNER_NAME || '-'}</span>
                            </td>
                            <td className="type-cell">
                              <span className={`type-badge ${event.WINNER_TYPE?.toLowerCase()}`}>
                                {event.WINNER_TYPE === 'TEAM' ? 'Equipe' : 'Joueur'}
                              </span>
                            </td>
                            <td className="reaction-cell">{formatReactionTime(event.REACTION_TIME)}</td>
                            <td className="points-cell">
                              <span className={`points-badge ${event.POINTS >= 0 ? 'positive' : 'negative'}`}>
                                {event.POINTS >= 0 ? '+' : ''}{event.POINTS} pts
                              </span>
                            </td>
                          </motion.tr>
                        ))}
                      </tbody>
                    </table>
                  </CardBody>
                </Card>
              </motion.div>
            ))}
          </AnimatePresence>
        </div>
      )}
    </div>
  )
}
