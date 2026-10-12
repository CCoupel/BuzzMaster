import { useMemo, useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { useGame } from '../hooks/GameContext'
import Podium from '../components/Podium'
import './ScoresPage.css'

export default function ScoresPage() {
  const { teams, bumpers, setBumperPoints, setTeamPoints } = useGame()
  const [pointsAmount, setPointsAmount] = useState(1)

  // Sort teams by score (descending) with rank calculation
  const sortedTeams = useMemo(() => {
    const sorted = Object.entries(teams)
      .map(([name, data]) => ({
        name,
        score: data.SCORE || 0,
        color: data.COLOR,
      }))
      .sort((a, b) => b.score - a.score)

    let currentRank = 1
    let lastScore = null
    let lastRank = 1
    return sorted.map((team, index) => {
      if (index > 0 && team.score === lastScore) {
        return { ...team, rank: lastRank }
      }
      currentRank = index + 1
      lastScore = team.score
      lastRank = currentRank
      return { ...team, rank: currentRank }
    })
  }, [teams])

  // Calculate max scores for progress bars
  const maxTeamScore = useMemo(() => Math.max(...sortedTeams.map(t => t.score), 1), [sortedTeams])

  // Sort players by score with rank calculation
  const sortedPlayers = useMemo(() => {
    const sorted = Object.entries(bumpers)
      .map(([mac, data]) => ({
        mac,
        name: data.NAME || mac.slice(-6),
        team: data.TEAM,
        score: data.SCORE || 0,
        teamColor: teams[data.TEAM]?.COLOR,
      }))
      .sort((a, b) => b.score - a.score)

    let currentRank = 1
    let lastScore = null
    let lastRank = 1
    return sorted.map((player, index) => {
      if (index > 0 && player.score === lastScore) {
        return { ...player, rank: lastRank }
      }
      currentRank = index + 1
      lastScore = player.score
      lastRank = currentRank
      return { ...player, rank: currentRank }
    })
  }, [bumpers, teams])

  const maxPlayerScore = useMemo(() => Math.max(...sortedPlayers.map(p => p.score), 1), [sortedPlayers])

  // Top 3 players for podium
  const topPlayers = useMemo(() => {
    return sortedPlayers.slice(0, 3).map(player => ({
      ...player,
      name: player.name,
      color: player.teamColor,
    }))
  }, [sortedPlayers])

  const getRgbColor = (color) => {
    if (!color) return 'var(--gray-400)'
    if (Array.isArray(color)) return `rgb(${color.join(',')})`
    return color
  }

  const getMedalEmoji = (rank) => {
    if (rank === 1) return '🥇'
    if (rank === 2) return '🥈'
    if (rank === 3) return '🥉'
    return `#${rank}`
  }

  const handleTeamPoints = (teamName, amount) => {
    setTeamPoints(teamName, amount)
  }

  const handlePlayerPoints = (playerMac, amount) => {
    setBumperPoints(playerMac, amount)
  }

  return (
    <div className="scores-page page">
      <header className="page-header">
        <h1 className="page-title">Gestion des Scores</h1>
        <div className="points-selector">
          <span className="points-label">Points:</span>
          <div className="points-buttons">
            {[1, 2, 5, 10].map(val => (
              <button
                key={val}
                className={`points-btn ${pointsAmount === val ? 'active' : ''}`}
                onClick={() => setPointsAmount(val)}
              >
                {val}
              </button>
            ))}
          </div>
        </div>
      </header>

      <div className="scores-grid">
        {/* Teams Section */}
        <section className="scores-section teams-section">
          <h2 className="section-title">Equipes</h2>

          <div className="section-content">
            {/* Teams Podium */}
            <div className="podium-container">
              {sortedTeams.length >= 1 && (
                <Podium teams={sortedTeams} variant="compact" />
              )}
            </div>

            {/* Teams List */}
            <div className="ranking-list">
              <AnimatePresence mode="popLayout">
                {sortedTeams.map((team) => {
                  const rgbColor = getRgbColor(team.color)
                  const isTied = sortedTeams.filter(t => t.rank === team.rank).length > 1

                  const barWidth = (team.score / maxTeamScore) * 100

                  return (
                    <motion.div
                      key={team.name}
                      className="ranking-item"
                      initial={{ opacity: 0, y: 20 }}
                      animate={{ opacity: 1, y: 0 }}
                      exit={{ opacity: 0, y: -20 }}
                      layout
                      style={{ '--team-color': rgbColor }}
                    >
                      <div className="ranking-rank">
                        <span className="rank-medal">{getMedalEmoji(team.rank)}</span>
                        {isTied && <span className="tied-badge">ex</span>}
                      </div>
                      <div className="ranking-badge" style={{ backgroundColor: rgbColor }}>
                        {team.name.charAt(0).toUpperCase()}
                      </div>
                      <div className="ranking-info">
                        <span className="ranking-name">{team.name}</span>
                        <div className="ranking-bar">
                          <motion.div
                            className="ranking-bar-fill"
                            initial={{ width: 0 }}
                            animate={{ width: `${barWidth}%` }}
                            transition={{ duration: 0.5 }}
                          />
                        </div>
                      </div>
                      <div className="ranking-score">{team.score}</div>
                      <div className="ranking-controls">
                        <button
                          className="control-btn remove"
                          onClick={() => handleTeamPoints(team.name, -pointsAmount)}
                          title={`Retirer ${pointsAmount} point(s)`}
                        >
                          -
                        </button>
                        <button
                          className="control-btn add"
                          onClick={() => handleTeamPoints(team.name, pointsAmount)}
                          title={`Ajouter ${pointsAmount} point(s)`}
                        >
                          +
                        </button>
                      </div>
                    </motion.div>
                  )
                })}
              </AnimatePresence>
            </div>
          </div>
        </section>

        {/* Players Section */}
        <section className="scores-section players-section">
          <h2 className="section-title">Joueurs</h2>

          <div className="section-content">
            {/* Players Podium */}
            <div className="podium-container">
              {topPlayers.length >= 1 && (
                <Podium teams={topPlayers} variant="compact" />
              )}
            </div>

            {/* Players List */}
            <div className="ranking-list">
              <AnimatePresence mode="popLayout">
                {sortedPlayers.map((player) => {
                  const rgbColor = getRgbColor(player.teamColor)
                  const isTied = sortedPlayers.filter(p => p.rank === player.rank).length > 1
                  const barWidth = (player.score / maxPlayerScore) * 100

                  return (
                    <motion.div
                      key={player.mac}
                      className="ranking-item player-item"
                      initial={{ opacity: 0, y: 20 }}
                      animate={{ opacity: 1, y: 0 }}
                      exit={{ opacity: 0, y: -20 }}
                      layout
                      style={{ '--team-color': rgbColor }}
                    >
                      <div className="ranking-rank">
                        <span className="rank-medal">{getMedalEmoji(player.rank)}</span>
                        {isTied && <span className="tied-badge">ex</span>}
                      </div>
                      <div className="ranking-badge" style={{ backgroundColor: rgbColor }}>
                        {player.name.charAt(0).toUpperCase()}
                      </div>
                      <div className="ranking-info">
                        <span className="ranking-name">{player.name}</span>
                        <span className="ranking-team">{player.team || 'Sans equipe'}</span>
                        <div className="ranking-bar">
                          <motion.div
                            className="ranking-bar-fill"
                            initial={{ width: 0 }}
                            animate={{ width: `${barWidth}%` }}
                            transition={{ duration: 0.5 }}
                          />
                        </div>
                      </div>
                      <div className="ranking-score">{player.score}</div>
                      <div className="ranking-controls">
                        <button
                          className="control-btn remove"
                          onClick={() => handlePlayerPoints(player.mac, -pointsAmount)}
                          title={`Retirer ${pointsAmount} point(s)`}
                        >
                          -
                        </button>
                        <button
                          className="control-btn add"
                          onClick={() => handlePlayerPoints(player.mac, pointsAmount)}
                          title={`Ajouter ${pointsAmount} point(s)`}
                        >
                          +
                        </button>
                      </div>
                    </motion.div>
                  )
                })}
              </AnimatePresence>
            </div>
          </div>
        </section>
      </div>
    </div>
  )
}
