import { useState, useMemo, useRef } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { useGame } from '../hooks/GameContext'
import Button from '../components/Button'
import Card, { CardHeader, CardBody } from '../components/Card'
import './ConfigPage.css'

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

export default function ConfigPage() {
  const { teams, bumpers, gameState, updateConfig, version } = useGame()
  const [editingTeam, setEditingTeam] = useState(null)
  const [editingBumper, setEditingBumper] = useState(null)
  const [newTeamName, setNewTeamName] = useState('')
  const [uploadingBg, setUploadingBg] = useState(false)
  const bgInputRef = useRef(null)

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

  const handleTeamScoreChange = (teamName, score) => {
    updateConfig({
      teams: {
        ...teams,
        [teamName]: { ...teams[teamName], SCORE: parseInt(score) || 0 }
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

  const handleResetScores = () => {
    if (!window.confirm('Remettre tous les scores a zero ?')) return
    const newTeams = { ...teams }
    Object.keys(newTeams).forEach(name => {
      newTeams[name] = { ...newTeams[name], SCORE: 0 }
    })
    const newBumpers = { ...bumpers }
    Object.keys(newBumpers).forEach(mac => {
      newBumpers[mac] = { ...newBumpers[mac], SCORE: 0 }
    })
    updateConfig({ teams: newTeams, bumpers: newBumpers })
  }

  const handleBackup = async () => {
    try {
      const response = await fetch('/backup')
      const blob = await response.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `buzzcontrol-backup-${new Date().toISOString().slice(0, 10)}.tar`
      a.click()
      URL.revokeObjectURL(url)
    } catch (error) {
      console.error('Backup failed:', error)
    }
  }

  const handleRestore = async (e) => {
    const file = e.target.files?.[0]
    if (!file) return

    if (!window.confirm('Restaurer les donnees depuis ce fichier ? Cette action est irreversible.')) {
      e.target.value = ''
      return
    }

    const formData = new FormData()
    formData.append('file', file)

    try {
      await fetch('/restore', { method: 'POST', body: formData })
      window.location.reload()
    } catch (error) {
      console.error('Restore failed:', error)
    }
    e.target.value = ''
  }

  const handleBackgroundUpload = async (e) => {
    const file = e.target.files?.[0]
    console.log('[Background] File selected:', file)
    if (!file) return

    setUploadingBg(true)
    const formData = new FormData()
    formData.append('file', file)

    try {
      console.log('[Background] Uploading...')
      const response = await fetch('/background', { method: 'POST', body: formData })
      console.log('[Background] Response:', response.status, response.statusText)
      if (response.ok) {
        window.location.reload()
      } else {
        const text = await response.text()
        console.error('[Background] Server error:', text)
        alert('Erreur: ' + text)
      }
    } catch (error) {
      console.error('Background upload failed:', error)
      alert('Erreur: ' + error.message)
    } finally {
      setUploadingBg(false)
      if (bgInputRef.current) bgInputRef.current.value = ''
    }
  }

  const handleRemoveBackground = async (bgPath) => {
    if (!window.confirm('Supprimer cette image de fond ?')) return
    try {
      const filename = bgPath.split('/').pop()
      await fetch(`/background?file=${encodeURIComponent(filename)}`, { method: 'DELETE' })
    } catch (error) {
      console.error('Remove background failed:', error)
    }
  }

  const handleRemoveAllBackgrounds = async () => {
    if (!window.confirm('Supprimer toutes les images de fond ?')) return
    try {
      await fetch('/background', { method: 'DELETE' })
    } catch (error) {
      console.error('Remove all backgrounds failed:', error)
    }
  }

  const handleDurationChange = async (index, newDuration) => {
    const backgrounds = [...(gameState?.backgrounds || [])]
    backgrounds[index] = { ...backgrounds[index], duration: parseInt(newDuration) || 10 }
    await saveBackgrounds(backgrounds)
  }

  const handleOpacityChange = async (index, newOpacity) => {
    const backgrounds = [...(gameState?.backgrounds || [])]
    backgrounds[index] = { ...backgrounds[index], opacity: Math.max(0, Math.min(100, parseInt(newOpacity) || 100)) }
    await saveBackgrounds(backgrounds)
  }

  const handleMoveBackground = async (fromIndex, toIndex) => {
    if (fromIndex === toIndex) return
    const backgrounds = [...(gameState?.backgrounds || [])]
    const [moved] = backgrounds.splice(fromIndex, 1)
    backgrounds.splice(toIndex, 0, moved)
    await saveBackgrounds(backgrounds)
  }

  const saveBackgrounds = async (backgrounds) => {
    try {
      await fetch('/background', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(backgrounds)
      })
    } catch (error) {
      console.error('Save backgrounds failed:', error)
    }
  }

  const [draggedIndex, setDraggedIndex] = useState(null)

  return (
    <div className="config-page page">
      <header className="page-header">
        <h1 className="page-title">Configuration</h1>
        <p className="page-subtitle">Gerez vos equipes et buzzers</p>
      </header>

      <div className="config-layout">
        {/* Teams Section */}
        <section className="teams-config-section">
          <div className="section-header">
            <h2 className="section-title">Equipes</h2>
            <div className="section-actions">
              <form onSubmit={(e) => { e.preventDefault(); handleAddTeam(); }} className="add-team-form">
                <input
                  type="text"
                  value={newTeamName}
                  onChange={(e) => setNewTeamName(e.target.value)}
                  placeholder="Nom de l'equipe..."
                  className="add-team-input"
                />
                <Button type="submit" variant="primary" size="sm">Ajouter</Button>
              </form>
            </div>
          </div>

          <div className="teams-list">
            <AnimatePresence>
              {Object.entries(teams).map(([name, data], index) => {
                const teamBumpers = bumpersByTeam[name] || []
                const rgbColor = data.COLOR ? `rgb(${data.COLOR.join(',')})` : 'var(--gray-400)'

                return (
                  <motion.div
                    key={name}
                    initial={{ opacity: 0, x: -20 }}
                    animate={{ opacity: 1, x: 0 }}
                    exit={{ opacity: 0, x: 20 }}
                    transition={{ delay: index * 0.1 }}
                  >
                    <Card padding="lg" className="team-config-card" style={{ '--team-color': rgbColor }}>
                      <div className="team-config-header">
                        <div className="team-color-picker">
                          {PRESET_COLORS.map((color, i) => (
                            <button
                              key={i}
                              className={`color-swatch ${JSON.stringify(color) === JSON.stringify(data.COLOR) ? 'active' : ''}`}
                              style={{ backgroundColor: `rgb(${color.join(',')})` }}
                              onClick={() => handleTeamColorChange(name, color)}
                            />
                          ))}
                        </div>
                        <h3 className="team-config-name">{name}</h3>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => handleDeleteTeam(name)}
                        >
                          Supprimer
                        </Button>
                      </div>

                      <div className="team-config-body">
                        <div className="score-editor">
                          <label>Score</label>
                          <input
                            type="number"
                            value={data.SCORE || 0}
                            onChange={(e) => handleTeamScoreChange(name, e.target.value)}
                          />
                        </div>

                        <div className="team-bumpers-list">
                          <span className="bumpers-label">{teamBumpers.length} buzzer(s)</span>
                          {teamBumpers.map(bumper => (
                            <span key={bumper.mac} className="bumper-tag">
                              {bumper.NAME}
                            </span>
                          ))}
                        </div>
                      </div>
                    </Card>
                  </motion.div>
                )
              })}
            </AnimatePresence>
          </div>
        </section>

        {/* Bumpers Section */}
        <section className="bumpers-config-section">
          <div className="section-header">
            <h2 className="section-title">Buzzers</h2>
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
                    initial={{ opacity: 0, y: 20 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -20 }}
                    transition={{ delay: index * 0.05 }}
                  >
                    <Card
                      padding="md"
                      className={`bumper-config-card ${data.READY === 'TRUE' ? 'ready' : ''}`}
                      style={{ borderLeftColor: teamColor }}
                    >
                      <div className="bumper-config-info">
                        <input
                          type="text"
                          value={data.NAME || mac.slice(-6)}
                          onChange={(e) => handleBumperNameChange(mac, e.target.value)}
                          className="bumper-name-input"
                        />
                        <span className="bumper-mac">{mac}</span>
                        {data.VERSION && (
                          <span className="bumper-version">v{data.VERSION}</span>
                        )}
                      </div>

                      <div className="bumper-config-controls">
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
                          <span className="ready-indicator">PRET</span>
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

        {/* Background Section */}
        <section className="background-section">
          <div className="section-header">
            <h2 className="section-title">Fonds d'ecran</h2>
            <div className="section-actions">
              <label className="upload-bg-btn">
                <input
                  type="file"
                  ref={bgInputRef}
                  accept="image/*"
                  onChange={handleBackgroundUpload}
                  style={{ display: 'none' }}
                />
                <Button variant="primary" size="sm" as="span" loading={uploadingBg}>
                  Ajouter une image
                </Button>
              </label>
              {gameState?.backgrounds?.length > 0 && (
                <Button variant="ghost" size="sm" onClick={handleRemoveAllBackgrounds}>
                  Tout supprimer
                </Button>
              )}
            </div>
          </div>
          <p className="section-hint">Glissez-deposez pour changer l'ordre. Duree en secondes.</p>
          <div className="backgrounds-grid">
            {gameState?.backgrounds?.length > 0 ? (
              gameState.backgrounds.map((bg, index) => (
                <motion.div
                  key={bg.path}
                  className={`background-item ${draggedIndex === index ? 'dragging' : ''}`}
                  initial={{ opacity: 0, scale: 0.9 }}
                  animate={{ opacity: 1, scale: 1 }}
                  transition={{ delay: index * 0.05 }}
                  draggable
                  onDragStart={() => setDraggedIndex(index)}
                  onDragEnd={() => setDraggedIndex(null)}
                  onDragOver={(e) => e.preventDefault()}
                  onDrop={() => {
                    if (draggedIndex !== null) {
                      handleMoveBackground(draggedIndex, index)
                    }
                  }}
                >
                  <img src={bg.path} alt={`Background ${index + 1}`} className="bg-thumb" />
                  <button
                    className="bg-delete-btn"
                    onClick={() => handleRemoveBackground(bg.path)}
                    title="Supprimer"
                  >
                    ×
                  </button>
                  <span className="bg-index">{index + 1}</span>
                  <div className="bg-controls">
                    <div className="bg-duration">
                      <input
                        type="number"
                        min="1"
                        max="300"
                        value={bg.duration || 10}
                        onChange={(e) => handleDurationChange(index, e.target.value)}
                        className="duration-input"
                      />
                      <span className="duration-label">s</span>
                    </div>
                    <div className="bg-opacity">
                      <input
                        type="range"
                        min="0"
                        max="100"
                        value={bg.opacity ?? 100}
                        onChange={(e) => handleOpacityChange(index, e.target.value)}
                        className="opacity-slider"
                      />
                      <span className="opacity-value">{bg.opacity ?? 100}%</span>
                    </div>
                  </div>
                </motion.div>
              ))
            ) : (
              <Card variant="outlined" padding="lg" className="empty-state backgrounds-empty">
                <p>Aucune image de fond</p>
                <p className="empty-hint">Ajoutez des images pour les afficher en cycle sur l'ecran TV</p>
              </Card>
            )}
          </div>
        </section>

        {/* System Section */}
        <section className="system-section">
          <h2 className="section-title">Systeme</h2>

          <Card padding="lg" className="system-card">
            <div className="system-info">
              {version && (
                <div className="info-item">
                  <span className="info-label">Version serveur</span>
                  <span className="info-value">{version}</span>
                </div>
              )}
              <div className="info-item">
                <span className="info-label">Equipes</span>
                <span className="info-value">{Object.keys(teams).length}</span>
              </div>
              <div className="info-item">
                <span className="info-label">Buzzers</span>
                <span className="info-value">{Object.keys(bumpers).length}</span>
              </div>
            </div>

            <div className="system-actions">
              <Button variant="secondary" onClick={handleResetScores}>
                Remettre les scores a zero
              </Button>
              <Button variant="secondary" onClick={handleBackup}>
                Sauvegarder les donnees
              </Button>
              <label className="restore-btn">
                <input
                  type="file"
                  accept=".tar"
                  onChange={handleRestore}
                  style={{ display: 'none' }}
                />
                <Button variant="secondary" as="span">
                  Restaurer une sauvegarde
                </Button>
              </label>
            </div>
          </Card>
        </section>
      </div>
    </div>
  )
}
