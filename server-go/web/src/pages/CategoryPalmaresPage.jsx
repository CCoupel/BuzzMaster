import { useState, useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import Card, { CardHeader, CardBody } from '../components/Card'
import Podium from '../components/Podium'
import { CATEGORIES } from '../components/QuestionCard'
import './CategoryPalmaresPage.css'

export default function CategoryPalmaresPage() {
  // v5.7.10 : GET /palmares retourne tout pré-assemblé (name/imageURL/color/teams/players)
  const [palmares, setPalmares] = useState([])
  const [loading, setLoading] = useState(true)
  const [expandedCategories, setExpandedCategories] = useState({})

  // Fetch pre-assembled palmares from server (v5.7.10)
  useEffect(() => {
    const fetchPalmares = async () => {
      try {
        const response = await fetch('/palmares')
        if (response.ok) {
          const data = await response.json()
          setPalmares(data || [])
        }
      } catch (error) {
        console.error('Failed to fetch palmares:', error)
      } finally {
        setLoading(false)
      }
    }

    fetchPalmares()
    // Refresh every 5 seconds
    const interval = setInterval(fetchPalmares, 5000)
    return () => clearInterval(interval)
  }, [])

  // Add ranks to a sorted list (ties get the same rank)
  const addRanks = (items) => items.reduce((acc, item, idx) => {
    const rank = idx === 0 ? 1 : (acc[idx - 1].points === item.points ? acc[idx - 1].rank : idx + 1)
    acc.push({ ...item, rank })
    return acc
  }, [])

  const toggleCategory = (category) => {
    setExpandedCategories(prev => ({
      ...prev,
      [category]: !prev[category]
    }))
  }

  const expandAll = () => {
    const allExpanded = {}
    palmares.forEach(entry => allExpanded[entry.category] = true)
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
  const transformForPodium = (items, teamColorMap = {}) => {
    return items.map(item => ({
      name: item.name,
      score: item.points,
      color: item.color || teamColorMap[item.team] || null,
      rank: item.rank,
    }))
  }

  const totalCategories = palmares.length
  const totalPoints = palmares.reduce((s, e) => s + e.totalPoints, 0)

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
              {totalCategories} categorie{totalCategories !== 1 ? 's' : ''} &bull; {totalPoints} point{totalPoints !== 1 ? 's' : ''}
            </p>
          </div>
          {palmares.length > 0 && (
            <div className="header-actions">
              <button className="action-btn" onClick={expandAll}>Tout ouvrir</button>
              <button className="action-btn" onClick={collapseAll}>Tout fermer</button>
            </div>
          )}
        </div>
      </header>

      {palmares.length === 0 ? (
        <Card className="empty-state">
          <CardBody>
            <p className="empty-message">Aucun evenement enregistre</p>
            <p className="empty-hint">Les points attribues pendant le jeu apparaitront ici par categorie</p>
          </CardBody>
        </Card>
      ) : (
        <div className="categories-grid">
          <AnimatePresence>
            {/* v5.7.10 : PalmaresEntry pré-assemblé — name/imageURL/color/teams/players directs */}
            {palmares.map((entry, index) => {
              const isExpanded = expandedCategories[entry.category]
              // v5.7.10 : name et color viennent directement de l'entry (backend résout)
              // Plus de fallback ❓ — si catégorie inconnue, backend retourne name vide et on titre-case la clé
              const catLabel = entry.name
                || (entry.category === 'UNKNOWN' ? 'Inconnue' : entry.category.replace(/_/g, ' ').toLowerCase().replace(/\b\w/g, c => c.toUpperCase()))
              const catIcon  = entry.imageURL
                ? <img src={entry.imageURL} alt={catLabel} style={{ width: '1.25rem', height: '1.25rem', objectFit: 'cover', borderRadius: '0.125rem' }} />
                : (CATEGORIES[entry.category]?.icon ?? '📷')
              const catColor = entry.color || CATEGORIES[entry.category]?.color || 'var(--gray-400)'
              const hasTeams   = entry.teams.length > 0
              const hasPlayers = entry.players.length > 0

              // ranks (items sorted desc by backend)
              const rankedTeams   = addRanks(entry.teams)
              const rankedPlayers = addRanks(entry.players)

              // team color lookup for player rows
              const teamColorMap = Object.fromEntries(entry.teams.map(t => [t.name, t.color]))

              const totalTeamPoints   = entry.teams.reduce((s, t) => s + t.points, 0)
              const totalPlayerPoints = entry.players.reduce((s, p) => s + p.points, 0)

              return (
                <motion.div
                  key={entry.category}
                  initial={{ opacity: 0, y: 20 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -20 }}
                  transition={{ delay: index * 0.05 }}
                  className="category-card-wrapper"
                >
                  <Card className={`category-card ${isExpanded ? 'expanded' : 'collapsed'}`}>
                    <CardHeader onClick={() => toggleCategory(entry.category)} className="clickable category-header">
                      <div className="category-header-content">
                        <span
                          className="category-icon-badge"
                          style={{ backgroundColor: catColor }}
                        >
                          {catIcon}
                        </span>
                        <span className="category-name">{catLabel}</span>
                        <span className={`collapse-icon ${isExpanded ? 'open' : ''}`}>▶</span>
                      </div>
                      <div className="category-stats">
                        {hasTeams && (
                          <span className="stat-badge team-stat" title="Points equipes">
                            <span className="stat-icon">👥</span>
                            <span className="stat-value">{totalTeamPoints} pts</span>
                          </span>
                        )}
                        {hasPlayers && (
                          <span className="stat-badge player-stat" title="Points joueurs">
                            <span className="stat-icon">👤</span>
                            <span className="stat-value">{totalPlayerPoints} pts</span>
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
                                      teams={transformForPodium(rankedTeams.slice(0, 3))}
                                      variant="compact"
                                    />
                                  </div>
                                  {rankedTeams.length > 3 && (
                                    <div className="ranking-list">
                                      {rankedTeams.slice(3).map((team) => (
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
                                      teams={transformForPodium(rankedPlayers.slice(0, 3), teamColorMap)}
                                      variant="compact"
                                    />
                                  </div>
                                  {rankedPlayers.length > 3 && (
                                    <div className="ranking-list">
                                      {rankedPlayers.slice(3).map((player) => (
                                        <div key={`${player.team}|${player.name}`} className="ranking-item">
                                          <span className="ranking-position">{player.rank}</span>
                                          <span
                                            className="ranking-dot"
                                            style={{ backgroundColor: getRgbColor(teamColorMap[player.team]) }}
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
                                {rankedTeams.slice(0, 3).map((team, idx) => (
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
                                {rankedPlayers.slice(0, 3).map((player, idx) => (
                                  <span
                                    key={`${player.team}|${player.name}`}
                                    className="summary-badge"
                                    style={{
                                      backgroundColor: `${getRgbColor(teamColorMap[player.team])}20`,
                                      borderColor: getRgbColor(teamColorMap[player.team]),
                                      color: getRgbColor(teamColorMap[player.team]),
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
