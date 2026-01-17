import { useState, useEffect, useMemo } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import Card, { CardHeader, CardBody } from '../components/Card'
import Podium from '../components/Podium'
import { CATEGORIES } from '../components/QuestionCard'
import './CategoryPalmaresPage.css'

export default function CategoryPalmaresPage() {
  const [history, setHistory] = useState([])
  const [loading, setLoading] = useState(true)
  const [expandedCategories, setExpandedCategories] = useState({})

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

  // Aggregate stats by category
  const categoryStats = useMemo(() => {
    const stats = {}

    history.forEach(event => {
      const category = event.QUESTION_CATEGORY || 'UNKNOWN'

      if (!stats[category]) {
        stats[category] = {
          teams: {},
          players: {},
          totalTeamPoints: 0,
          totalPlayerPoints: 0,
          eventCount: 0,
        }
      }

      stats[category].eventCount++

      if (event.WINNER_TYPE === 'TEAM' && event.TEAM_NAME) {
        // Points awarded to team
        if (!stats[category].teams[event.TEAM_NAME]) {
          stats[category].teams[event.TEAM_NAME] = {
            name: event.TEAM_NAME,
            color: event.TEAM_COLOR,
            points: 0,
          }
        }
        stats[category].teams[event.TEAM_NAME].points += event.POINTS
        stats[category].totalTeamPoints += event.POINTS
      } else if (event.WINNER_TYPE === 'PLAYER' && event.PLAYER_NAME) {
        // Points awarded to player
        const key = `${event.TEAM_NAME}|${event.PLAYER_NAME}`
        if (!stats[category].players[key]) {
          stats[category].players[key] = {
            name: event.PLAYER_NAME,
            team: event.TEAM_NAME,
            teamColor: event.TEAM_COLOR,
            playerColor: event.PLAYER_COLOR,
            points: 0,
          }
        }
        stats[category].players[key].points += event.POINTS
        stats[category].totalPlayerPoints += event.POINTS
      }
    })

    // Convert to arrays, sort and calculate ranks
    Object.keys(stats).forEach(category => {
      // Sort teams by points
      const sortedTeams = Object.values(stats[category].teams)
        .sort((a, b) => b.points - a.points)

      // Calculate team ranks with ties
      let currentRank = 1
      stats[category].sortedTeams = sortedTeams.map((team, index) => {
        if (index > 0 && sortedTeams[index - 1].points === team.points) {
          return { ...team, rank: currentRank }
        }
        currentRank = index + 1
        return { ...team, rank: currentRank }
      })

      // Sort players by points
      const sortedPlayers = Object.values(stats[category].players)
        .sort((a, b) => b.points - a.points)

      // Calculate player ranks with ties
      currentRank = 1
      stats[category].sortedPlayers = sortedPlayers.map((player, index) => {
        if (index > 0 && sortedPlayers[index - 1].points === player.points) {
          return { ...player, rank: currentRank }
        }
        currentRank = index + 1
        return { ...player, rank: currentRank }
      })
    })

    // Filter out categories with no events and sort by total points
    return Object.entries(stats)
      .filter(([_, data]) => data.eventCount > 0)
      .sort((a, b) => {
        const totalA = a[1].totalTeamPoints + a[1].totalPlayerPoints
        const totalB = b[1].totalTeamPoints + b[1].totalPlayerPoints
        return totalB - totalA
      })
  }, [history])

  const toggleCategory = (category) => {
    setExpandedCategories(prev => ({
      ...prev,
      [category]: !prev[category]
    }))
  }

  const expandAll = () => {
    const allExpanded = {}
    categoryStats.forEach(([cat]) => allExpanded[cat] = true)
    setExpandedCategories(allExpanded)
  }

  const collapseAll = () => {
    setExpandedCategories({})
  }

  const getRgbColor = (color) => {
    if (!color) return 'var(--gray-400)'
    if (Array.isArray(color)) return `rgb(${color.join(',')})`
    return color
  }

  // Transform for Podium component (expects { name, score, color })
  const transformForPodium = (items, isTeam = true) => {
    return items.map(item => ({
      name: item.name,
      score: item.points,
      color: isTeam ? item.color : item.teamColor,
      rank: item.rank,
    }))
  }

  const totalEvents = history.length
  const totalCategories = categoryStats.length

  if (loading) {
    return (
      <div className="palmares-page page">
        <header className="page-header">
          <h1 className="page-title">Palmares par Categorie</h1>
          <p className="page-subtitle">Chargement...</p>
        </header>
      </div>
    )
  }

  return (
    <div className="palmares-page page">
      <header className="page-header">
        <div className="page-header-row">
          <div>
            <h1 className="page-title">Palmares par Categorie</h1>
            <p className="page-subtitle">
              {totalCategories} categorie{totalCategories !== 1 ? 's' : ''} &bull; {totalEvents} evenement{totalEvents !== 1 ? 's' : ''}
            </p>
          </div>
          {categoryStats.length > 0 && (
            <div className="header-actions">
              <button className="action-btn" onClick={expandAll}>Tout ouvrir</button>
              <button className="action-btn" onClick={collapseAll}>Tout fermer</button>
            </div>
          )}
        </div>
      </header>

      {categoryStats.length === 0 ? (
        <Card className="empty-state">
          <CardBody>
            <p className="empty-message">Aucun evenement enregistre</p>
            <p className="empty-hint">Les points attribues pendant le jeu apparaitront ici par categorie</p>
          </CardBody>
        </Card>
      ) : (
        <div className="categories-grid">
          <AnimatePresence>
            {categoryStats.map(([category, data], index) => {
              const isExpanded = expandedCategories[category]
              const catInfo = CATEGORIES[category] || { label: category, icon: '❓', color: 'var(--gray-400)' }
              const hasTeams = data.sortedTeams.length > 0
              const hasPlayers = data.sortedPlayers.length > 0

              return (
                <motion.div
                  key={category}
                  initial={{ opacity: 0, y: 20 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -20 }}
                  transition={{ delay: index * 0.05 }}
                  className="category-card-wrapper"
                >
                  <Card className={`category-card ${isExpanded ? 'expanded' : 'collapsed'}`}>
                    <CardHeader onClick={() => toggleCategory(category)} className="clickable category-header">
                      <div className="category-header-content">
                        <span
                          className="category-icon-badge"
                          style={{ backgroundColor: catInfo.color }}
                        >
                          {catInfo.icon}
                        </span>
                        <span className="category-name">{catInfo.label}</span>
                        <span className={`collapse-icon ${isExpanded ? 'open' : ''}`}>▶</span>
                      </div>
                      <div className="category-stats">
                        {hasTeams && (
                          <span className="stat-badge team-stat" title="Points equipes">
                            <span className="stat-icon">👥</span>
                            <span className="stat-value">{data.totalTeamPoints} pts</span>
                          </span>
                        )}
                        {hasPlayers && (
                          <span className="stat-badge player-stat" title="Points joueurs">
                            <span className="stat-icon">👤</span>
                            <span className="stat-value">{data.totalPlayerPoints} pts</span>
                          </span>
                        )}
                      </div>
                    </CardHeader>

                    <AnimatePresence>
                      {isExpanded && (
                        <motion.div
                          initial={{ height: 0, opacity: 0 }}
                          animate={{ height: 'auto', opacity: 1 }}
                          exit={{ height: 0, opacity: 0 }}
                          transition={{ duration: 0.2 }}
                        >
                          <CardBody className="category-body">
                            <div className="category-columns">
                              {/* Teams Column */}
                              {hasTeams && (
                                <div className="ranking-column">
                                  <h3 className="column-title">
                                    <span className="column-icon">👥</span>
                                    Equipes
                                  </h3>
                                  <div className="podium-container">
                                    <Podium
                                      teams={transformForPodium(data.sortedTeams.slice(0, 3), true)}
                                      variant="compact"
                                    />
                                  </div>
                                  {data.sortedTeams.length > 3 && (
                                    <div className="ranking-list">
                                      {data.sortedTeams.slice(3).map((team, idx) => (
                                        <div key={team.name} className="ranking-item">
                                          <span className="ranking-position">{team.rank}</span>
                                          <span
                                            className="ranking-dot"
                                            style={{ backgroundColor: getRgbColor(team.color) }}
                                          />
                                          <span className="ranking-name">{team.name}</span>
                                          <span className="ranking-points">{team.points} pts</span>
                                        </div>
                                      ))}
                                    </div>
                                  )}
                                </div>
                              )}

                              {/* Players Column */}
                              {hasPlayers && (
                                <div className="ranking-column">
                                  <h3 className="column-title">
                                    <span className="column-icon">👤</span>
                                    Joueurs
                                  </h3>
                                  <div className="podium-container">
                                    <Podium
                                      teams={transformForPodium(data.sortedPlayers.slice(0, 3), false)}
                                      variant="compact"
                                    />
                                  </div>
                                  {data.sortedPlayers.length > 3 && (
                                    <div className="ranking-list">
                                      {data.sortedPlayers.slice(3).map((player, idx) => (
                                        <div key={`${player.team}|${player.name}`} className="ranking-item">
                                          <span className="ranking-position">{player.rank}</span>
                                          <span
                                            className="ranking-dot"
                                            style={{ backgroundColor: getRgbColor(player.teamColor) }}
                                          />
                                          <span className="ranking-name">{player.name}</span>
                                          <span className="ranking-team">({player.team})</span>
                                          <span className="ranking-points">{player.points} pts</span>
                                        </div>
                                      ))}
                                    </div>
                                  )}
                                </div>
                              )}

                              {/* Show message if no data */}
                              {!hasTeams && !hasPlayers && (
                                <div className="no-data">
                                  <p>Aucune donnee pour cette categorie</p>
                                </div>
                              )}
                            </div>
                          </CardBody>
                        </motion.div>
                      )}
                    </AnimatePresence>

                    {/* Collapsed summary */}
                    {!isExpanded && (hasTeams || hasPlayers) && (
                      <CardBody className="summary-body">
                        <div className="summary-section">
                          {hasTeams && (
                            <div className="summary-row">
                              <span className="summary-label">Top equipe:</span>
                              <div className="summary-badges">
                                {data.sortedTeams.slice(0, 3).map((team, idx) => (
                                  <span
                                    key={team.name}
                                    className="summary-badge"
                                    style={{
                                      backgroundColor: `${getRgbColor(team.color)}20`,
                                      borderColor: getRgbColor(team.color),
                                      color: getRgbColor(team.color),
                                    }}
                                  >
                                    <span className="badge-rank">{idx + 1}</span>
                                    <span className="badge-name">{team.name}</span>
                                    <span className="badge-points">{team.points}</span>
                                  </span>
                                ))}
                              </div>
                            </div>
                          )}
                          {hasPlayers && (
                            <div className="summary-row">
                              <span className="summary-label">Top joueur:</span>
                              <div className="summary-badges">
                                {data.sortedPlayers.slice(0, 3).map((player, idx) => (
                                  <span
                                    key={`${player.team}|${player.name}`}
                                    className="summary-badge"
                                    style={{
                                      backgroundColor: `${getRgbColor(player.teamColor)}20`,
                                      borderColor: getRgbColor(player.teamColor),
                                      color: getRgbColor(player.teamColor),
                                    }}
                                  >
                                    <span className="badge-rank">{idx + 1}</span>
                                    <span className="badge-name">{player.name}</span>
                                    <span className="badge-points">{player.points}</span>
                                  </span>
                                ))}
                              </div>
                            </div>
                          )}
                        </div>
                      </CardBody>
                    )}
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
