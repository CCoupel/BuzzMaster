import { useState, useMemo } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { useGame } from '../hooks/GameContext'
import Button from '../components/Button'
import Card from '../components/Card'
import './TeamsPage.css'

const PRESET_COLORS = [
  [239, 68, 68],    // Red
  [249, 115, 22],   // Orange
  [234, 179, 8],    // Yellow
  [34, 197, 94],    // Green
  [6, 182, 212],    // Cyan
  [99, 102, 241],   // Indigo
  [168, 85, 247],   // Purple
  [236, 72, 153],   // Pink
]

export default function TeamsPage() {
  const { teams, bumpers, updateConfig } = useGame()
  const [newTeamName, setNewTeamName] = useState('')

  // Group bumpers by team
  const bumpersByTeam = useMemo(() => {
    const grouped = { unassigned: [] }
    Object.entries(bumpers).forEach(([mac, bumper]) => {
      const teamName = bumper.TEAM || 'unassigned'
      if (!grouped[teamName]) grouped[teamName] = []
      grouped[teamName].push({ mac, ...bumper })
    })
    return grouped
  }, [bumpers])

  const handleTeamColorChange = (teamName, color) => {
    updateConfig({
      teams: {
        ...teams,
        [teamName]: { ...teams[teamName], COLOR: color }
      }
    })
  }

  const handleBumperNameChange = (mac, name) => {
    updateConfig({
      bumpers: {
        ...bumpers,
        [mac]: { ...bumpers[mac], NAME: name }
      }
    })
  }

  const handleBumperTeamChange = (mac, teamName) => {
    updateConfig({
      bumpers: {
        ...bumpers,
        [mac]: { ...bumpers[mac], TEAM: teamName }
      }
    })
  }

  const handleAddTeam = () => {
    if (!newTeamName.trim()) return
    const randomColor = PRESET_COLORS[Math.floor(Math.random() * PRESET_COLORS.length)]
    updateConfig({
      teams: {
        ...teams,
        [newTeamName.trim()]: { COLOR: randomColor, SCORE: 0 }
      }
    })
    setNewTeamName('')
  }

  const handleDeleteTeam = (teamName) => {
    if (!window.confirm(`Supprimer l'equipe "${teamName}" ?`)) return
    const newTeams = { ...teams }
    delete newTeams[teamName]
    // Unassign bumpers from this team
    const newBumpers = { ...bumpers }
    Object.entries(newBumpers).forEach(([mac, bumper]) => {
      if (bumper.TEAM === teamName) {
        newBumpers[mac] = { ...bumper, TEAM: '' }
      }
    })
    updateConfig({ teams: newTeams, bumpers: newBumpers })
  }

  const handleRenameTeam = (oldName, newName) => {
    if (!newName.trim() || newName === oldName) return
    if (teams[newName]) {
      alert('Une equipe avec ce nom existe deja')
      return
    }
    const newTeams = { ...teams }
    newTeams[newName] = newTeams[oldName]
    delete newTeams[oldName]
    // Update bumpers with new team name
    const newBumpers = { ...bumpers }
    Object.entries(newBumpers).forEach(([mac, bumper]) => {
      if (bumper.TEAM === oldName) {
        newBumpers[mac] = { ...bumper, TEAM: newName }
      }
    })
    updateConfig({ teams: newTeams, bumpers: newBumpers })
  }

  return (
    <div className="teams-page page">
      <header className="page-header">
        <h1 className="page-title">Equipes & Joueurs</h1>
        <p className="page-subtitle">Gerez vos equipes et assignez les buzzers</p>
      </header>

      <div className="teams-layout">
        {/* Teams Section */}
        <section className="teams-section">
          <div className="section-header">
            <h2 className="section-title">Equipes</h2>
            <form onSubmit={(e) => { e.preventDefault(); handleAddTeam(); }} className="add-team-form">
              <input
                type="text"
                value={newTeamName}
                onChange={(e) => setNewTeamName(e.target.value)}
                placeholder="Nouvelle equipe..."
                className="add-team-input"
              />
              <Button type="submit" variant="primary" size="sm">Ajouter</Button>
            </form>
          </div>

          <div className="teams-grid">
            <AnimatePresence>
              {Object.entries(teams).map(([name, data], index) => {
                const teamBumpers = bumpersByTeam[name] || []
                const rgbColor = data.COLOR ? `rgb(${data.COLOR.join(',')})` : 'var(--gray-400)'

                return (
                  <motion.div
                    key={name}
                    initial={{ opacity: 0, scale: 0.9 }}
                    animate={{ opacity: 1, scale: 1 }}
                    exit={{ opacity: 0, scale: 0.9 }}
                    transition={{ delay: index * 0.05 }}
                    className="team-card-wrapper"
                  >
                    <Card padding="lg" className="team-card" style={{ '--team-color': rgbColor }}>
                      <div className="team-header">
                        <div
                          className="team-color-badge"
                          style={{ backgroundColor: rgbColor }}
                        >
                          {name.charAt(0).toUpperCase()}
                        </div>
                        <div className="team-info">
                          <input
                            type="text"
                            defaultValue={name}
                            className="team-name-input"
                            onBlur={(e) => handleRenameTeam(name, e.target.value)}
                            onKeyDown={(e) => {
                              if (e.key === 'Enter') {
                                e.target.blur()
                              }
                            }}
                          />
                          <span className="team-members-count">
                            {teamBumpers.length} joueur{teamBumpers.length !== 1 ? 's' : ''}
                          </span>
                        </div>
                        <button
                          className="team-delete-btn"
                          onClick={() => handleDeleteTeam(name)}
                          title="Supprimer l'equipe"
                        >
                          ×
                        </button>
                      </div>

                      <div className="team-colors">
                        {PRESET_COLORS.map((color, i) => (
                          <button
                            key={i}
                            className={`color-swatch ${JSON.stringify(color) === JSON.stringify(data.COLOR) ? 'active' : ''}`}
                            style={{ backgroundColor: `rgb(${color.join(',')})` }}
                            onClick={() => handleTeamColorChange(name, color)}
                            title="Changer la couleur"
                          />
                        ))}
                      </div>

                      {teamBumpers.length > 0 && (
                        <div className="team-members">
                          {teamBumpers.map(bumper => (
                            <span key={bumper.mac} className="member-tag">
                              {bumper.NAME || bumper.mac.slice(-6)}
                            </span>
                          ))}
                        </div>
                      )}
                    </Card>
                  </motion.div>
                )
              })}
            </AnimatePresence>

            {Object.keys(teams).length === 0 && (
              <Card variant="outlined" padding="lg" className="empty-state">
                <p>Aucune equipe</p>
                <p className="empty-hint">Creez des equipes pour y assigner des joueurs</p>
              </Card>
            )}
          </div>
        </section>

        {/* Bumpers Section */}
        <section className="bumpers-section">
          <div className="section-header">
            <h2 className="section-title">Joueurs / Buzzers</h2>
            <span className="bumper-count">{Object.keys(bumpers).length} connecte(s)</span>
          </div>

          <div className="bumpers-list">
            <AnimatePresence>
              {Object.entries(bumpers).map(([mac, data], index) => {
                const teamColor = data.TEAM && teams[data.TEAM]?.COLOR
                  ? `rgb(${teams[data.TEAM].COLOR.join(',')})`
                  : 'var(--gray-300)'

                return (
                  <motion.div
                    key={mac}
                    initial={{ opacity: 0, x: -20 }}
                    animate={{ opacity: 1, x: 0 }}
                    exit={{ opacity: 0, x: 20 }}
                    transition={{ delay: index * 0.03 }}
                  >
                    <Card
                      padding="md"
                      className={`bumper-card ${data.READY === 'TRUE' ? 'ready' : ''}`}
                      style={{ '--bumper-team-color': teamColor }}
                    >
                      <div className="bumper-avatar" style={{ backgroundColor: teamColor }}>
                        {(data.NAME || mac.slice(-6)).charAt(0).toUpperCase()}
                      </div>

                      <div className="bumper-info">
                        <input
                          type="text"
                          value={data.NAME || ''}
                          placeholder={mac.slice(-6)}
                          onChange={(e) => handleBumperNameChange(mac, e.target.value)}
                          className="bumper-name-input"
                        />
                        <div className="bumper-meta">
                          <span className="bumper-mac">{mac}</span>
                          {data.VERSION && <span className="bumper-version">v{data.VERSION}</span>}
                        </div>
                      </div>

                      <div className="bumper-controls">
                        <select
                          value={data.TEAM || ''}
                          onChange={(e) => handleBumperTeamChange(mac, e.target.value)}
                          className="team-select"
                        >
                          <option value="">Sans equipe</option>
                          {Object.keys(teams).map(teamName => (
                            <option key={teamName} value={teamName}>{teamName}</option>
                          ))}
                        </select>

                        {data.READY === 'TRUE' && (
                          <span className="ready-badge">PRET</span>
                        )}
                      </div>
                    </Card>
                  </motion.div>
                )
              })}
            </AnimatePresence>

            {Object.keys(bumpers).length === 0 && (
              <Card variant="outlined" padding="lg" className="empty-state">
                <p>Aucun buzzer connecte</p>
                <p className="empty-hint">Allumez vos buzzers pour les voir apparaitre ici</p>
              </Card>
            )}
          </div>
        </section>
      </div>
    </div>
  )
}
