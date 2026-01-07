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

// Answer colors for QCM mode
const ANSWER_COLORS = {
  RED: { label: 'Rouge', color: '#ef4444', letter: 'A' },
  GREEN: { label: 'Vert', color: '#22c55e', letter: 'B' },
  YELLOW: { label: 'Jaune', color: '#eab308', letter: 'C' },
  BLUE: { label: 'Bleu', color: '#3b82f6', letter: 'D' },
}

export default function TeamsPage() {
  const { teams, bumpers, updateConfig } = useGame()
  const [newTeamName, setNewTeamName] = useState('')
  const [draggedBumper, setDraggedBumper] = useState(null)
  const [dragOverTarget, setDragOverTarget] = useState(null)

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

  // Unassigned bumpers
  const unassignedBumpers = bumpersByTeam.unassigned || []

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

  const handleBumperAnswerColorChange = (mac, answerColor) => {
    updateConfig({
      bumpers: {
        ...bumpers,
        [mac]: { ...bumpers[mac], ANSWER_COLOR: answerColor }
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

  // Drag and Drop handlers
  const handleDragStart = (e, mac) => {
    setDraggedBumper(mac)
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', mac)
    // Add dragging class after a small delay for visual feedback
    setTimeout(() => {
      e.target.classList.add('dragging')
    }, 0)
  }

  const handleDragEnd = (e) => {
    e.target.classList.remove('dragging')
    setDraggedBumper(null)
    setDragOverTarget(null)
  }

  const handleDragOver = (e, targetTeam) => {
    e.preventDefault()
    e.dataTransfer.dropEffect = 'move'
    setDragOverTarget(targetTeam)
  }

  const handleDragLeave = (e) => {
    // Only clear if leaving the drop zone entirely
    if (!e.currentTarget.contains(e.relatedTarget)) {
      setDragOverTarget(null)
    }
  }

  const handleDrop = (e, targetTeam) => {
    e.preventDefault()
    const mac = e.dataTransfer.getData('text/plain')
    if (mac) {
      handleBumperTeamChange(mac, targetTeam === 'unassigned' ? '' : targetTeam)
    }
    setDragOverTarget(null)
    setDraggedBumper(null)
  }

  const getRgbColor = (color) => {
    if (!color) return 'var(--gray-400)'
    if (Array.isArray(color)) return `rgb(${color.join(',')})`
    return color
  }

  return (
    <div className="teams-page page">
      <header className="page-header">
        <h1 className="page-title">Equipes & Joueurs</h1>
        <p className="page-subtitle">Glissez-deposez les joueurs pour les assigner aux equipes</p>
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
                const rgbColor = getRgbColor(data.COLOR)
                const isDropTarget = dragOverTarget === name

                return (
                  <motion.div
                    key={name}
                    initial={{ opacity: 0, scale: 0.9 }}
                    animate={{ opacity: 1, scale: 1 }}
                    exit={{ opacity: 0, scale: 0.9 }}
                    transition={{ delay: index * 0.05 }}
                    className="team-card-wrapper"
                  >
                    <Card
                      padding="lg"
                      className={`team-card ${isDropTarget ? 'drop-target' : ''} ${draggedBumper ? 'can-drop' : ''}`}
                      style={{ '--team-color': rgbColor }}
                      onDragOver={(e) => handleDragOver(e, name)}
                      onDragLeave={handleDragLeave}
                      onDrop={(e) => handleDrop(e, name)}
                    >
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

                      {/* Team Members - Draggable */}
                      <div className="team-members-zone">
                        {teamBumpers.length > 0 ? (
                          <div className="team-members-list">
                            {teamBumpers.map(bumper => {
                              // Use answer color if set, otherwise gray
                              const avatarColor = bumper.ANSWER_COLOR && ANSWER_COLORS[bumper.ANSWER_COLOR]
                                ? ANSWER_COLORS[bumper.ANSWER_COLOR].color
                                : 'var(--gray-400)'
                              return (
                                <div
                                  key={bumper.mac}
                                  className="draggable-member"
                                  draggable
                                  onDragStart={(e) => handleDragStart(e, bumper.mac)}
                                  onDragEnd={handleDragEnd}
                                >
                                  <div
                                    className="member-avatar"
                                    style={{ backgroundColor: avatarColor }}
                                  >
                                    {(bumper.NAME || bumper.mac.slice(-6)).charAt(0).toUpperCase()}
                                  </div>
                                  <div className="member-info">
                                    <input
                                      type="text"
                                      value={bumper.NAME || ''}
                                      placeholder={bumper.mac.slice(-6)}
                                      onChange={(e) => handleBumperNameChange(bumper.mac, e.target.value)}
                                      className="member-name-input"
                                      onClick={(e) => e.stopPropagation()}
                                    />
                                    <span className="member-mac">{bumper.mac}</span>
                                  </div>
                                  {bumper.ANSWER_COLOR && ANSWER_COLORS[bumper.ANSWER_COLOR] && (
                                    <span className="answer-color-badge" style={{ backgroundColor: ANSWER_COLORS[bumper.ANSWER_COLOR].color }}>
                                      {ANSWER_COLORS[bumper.ANSWER_COLOR].letter}
                                    </span>
                                  )}
                                  <span className="drag-handle">⋮⋮</span>
                                </div>
                              )
                            })}
                          </div>
                        ) : (
                          <div className="team-drop-hint">
                            Deposez des joueurs ici
                          </div>
                        )}
                      </div>
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

        {/* Unassigned Bumpers Section */}
        <section className="bumpers-section">
          <div className="section-header">
            <h2 className="section-title">Joueurs non assignes</h2>
            <span className="bumper-count">{Object.keys(bumpers).length} connecte(s)</span>
          </div>

          <div
            className={`unassigned-zone ${dragOverTarget === 'unassigned' ? 'drop-target' : ''} ${draggedBumper ? 'can-drop' : ''}`}
            onDragOver={(e) => handleDragOver(e, 'unassigned')}
            onDragLeave={handleDragLeave}
            onDrop={(e) => handleDrop(e, 'unassigned')}
          >
            <AnimatePresence>
              {unassignedBumpers.map((bumper, index) => (
                <motion.div
                  key={bumper.mac}
                  initial={{ opacity: 0, x: -20 }}
                  animate={{ opacity: 1, x: 0 }}
                  exit={{ opacity: 0, x: 20 }}
                  transition={{ delay: index * 0.03 }}
                  className="draggable-bumper"
                  draggable
                  onDragStart={(e) => handleDragStart(e, bumper.mac)}
                  onDragEnd={handleDragEnd}
                >
                  <Card
                    padding="md"
                    className={`bumper-card ${bumper.READY === 'TRUE' ? 'ready' : ''}`}
                  >
                    <div
                      className={`bumper-avatar ${bumper.ANSWER_COLOR ? 'has-color' : ''}`}
                      style={bumper.ANSWER_COLOR && ANSWER_COLORS[bumper.ANSWER_COLOR]
                        ? { backgroundColor: ANSWER_COLORS[bumper.ANSWER_COLOR].color }
                        : {}
                      }
                    >
                      {(bumper.NAME || bumper.mac.slice(-6)).charAt(0).toUpperCase()}
                    </div>

                    <div className="bumper-info">
                      <input
                        type="text"
                        value={bumper.NAME || ''}
                        placeholder={bumper.mac.slice(-6)}
                        onChange={(e) => handleBumperNameChange(bumper.mac, e.target.value)}
                        className="bumper-name-input"
                      />
                      <div className="bumper-meta">
                        <span className="bumper-mac">{bumper.mac}</span>
                        {bumper.VERSION && <span className="bumper-version">v{bumper.VERSION}</span>}
                      </div>
                    </div>

                    <div className="answer-color-selector">
                      {Object.entries(ANSWER_COLORS).map(([key, { color, letter }]) => (
                        <button
                          key={key}
                          className={`answer-color-btn ${bumper.ANSWER_COLOR === key ? 'active' : ''}`}
                          style={{ backgroundColor: color }}
                          onClick={(e) => {
                            e.stopPropagation()
                            handleBumperAnswerColorChange(bumper.mac, bumper.ANSWER_COLOR === key ? '' : key)
                          }}
                          title={ANSWER_COLORS[key].label}
                        >
                          {letter}
                        </button>
                      ))}
                    </div>

                    <div className="bumper-controls">
                      {bumper.READY === 'TRUE' && (
                        <span className="ready-badge">PRET</span>
                      )}
                      <span className="drag-handle">⋮⋮</span>
                    </div>
                  </Card>
                </motion.div>
              ))}
            </AnimatePresence>

            {unassignedBumpers.length === 0 && !draggedBumper && (
              <div className="empty-unassigned">
                <p>Tous les joueurs sont assignes</p>
                <p className="empty-hint">Deposez des joueurs ici pour les retirer de leur equipe</p>
              </div>
            )}

            {unassignedBumpers.length === 0 && draggedBumper && (
              <div className="drop-hint-large">
                Deposez ici pour retirer de l'equipe
              </div>
            )}
          </div>
        </section>
      </div>
    </div>
  )
}
