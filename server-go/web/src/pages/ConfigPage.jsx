import { useState, useRef } from 'react'
import { motion } from 'framer-motion'
import { useGame } from '../hooks/GameContext'
import Button from '../components/Button'
import Card from '../components/Card'
import './ConfigPage.css'

export default function ConfigPage() {
  const { teams, bumpers, gameState, updateConfig, sendMessage, version } = useGame()
  const [uploadingBg, setUploadingBg] = useState(false)
  const bgInputRef = useRef(null)
  const [draggedIndex, setDraggedIndex] = useState(null)

  // Backup options
  const [backupOptions, setBackupOptions] = useState({
    questions: true,
    teams: true,
    bumpers: true,
    history: true,
    backgrounds: true,
  })

  // Reset options
  const [resetOptions, setResetOptions] = useState({
    questions: false,
    teams: false,
    bumpers: false,
    history: false,
    backgrounds: false,
  })

  const [loadingDemo, setLoadingDemo] = useState(false)

  const handleResetScores = () => {
    if (!window.confirm('Remettre tous les scores a zero ?')) return
    sendMessage('RAZ', {})
  }

  const handleLoadDemo = async () => {
    if (!window.confirm('Charger les donnees de demonstration ? Les donnees actuelles seront remplacees.')) return

    setLoadingDemo(true)
    try {
      const response = await fetch('/load-demo', { method: 'POST' })
      if (response.ok) {
        window.location.reload()
      } else {
        const data = await response.json()
        alert('Erreur: ' + (data.message || 'Echec du chargement'))
      }
    } catch (error) {
      console.error('Load demo failed:', error)
      alert('Erreur: ' + error.message)
    } finally {
      setLoadingDemo(false)
    }
  }

  const handleBackup = async () => {
    try {
      // Build URL with selected options
      const params = new URLSearchParams()
      if (backupOptions.questions) params.append('questions', 'true')
      if (backupOptions.teams) params.append('teams', 'true')
      if (backupOptions.bumpers) params.append('bumpers', 'true')
      if (backupOptions.history) params.append('history', 'true')
      if (backupOptions.backgrounds) params.append('backgrounds', 'true')

      const response = await fetch(`/backup-select?${params.toString()}`)
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

  const handleSelectiveReset = async () => {
    const selected = Object.entries(resetOptions)
      .filter(([, v]) => v)
      .map(([k]) => k)

    if (selected.length === 0) {
      alert('Selectionnez au moins un element a reinitialiser')
      return
    }

    const labels = {
      questions: 'Questions',
      teams: 'Equipes',
      bumpers: 'Joueurs',
      history: 'Historique',
      backgrounds: 'Fonds'
    }
    const selectedLabels = selected.map(k => labels[k]).join(', ')

    if (!window.confirm(`Reinitialiser: ${selectedLabels} ?`)) return

    try {
      const params = new URLSearchParams()
      selected.forEach(k => params.append(k, 'true'))

      await fetch(`/reset-select?${params.toString()}`, { method: 'POST' })
      window.location.reload()
    } catch (error) {
      console.error('Reset failed:', error)
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
    if (!file) return

    setUploadingBg(true)
    const formData = new FormData()
    formData.append('file', file)

    try {
      const response = await fetch('/background', { method: 'POST', body: formData })
      if (response.ok) {
        window.location.reload()
      } else {
        const text = await response.text()
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

  return (
    <div className="config-page page">
      <header className="page-header">
        <h1 className="page-title">Configuration</h1>
        <p className="page-subtitle">Parametres du systeme</p>
      </header>

      <div className="config-layout">
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
            </div>

            {/* Demo Section */}
            <div className="config-section">
              <h3 className="config-section-title">Mode Demo</h3>
              <p className="config-section-hint">
                Charge des donnees de demonstration: equipes, joueurs, questions (QCM, Memory, Normal) et historique.
              </p>
              <div className="config-section-actions">
                <Button variant="primary" onClick={handleLoadDemo} loading={loadingDemo}>
                  Charger la demo
                </Button>
              </div>
            </div>

            {/* Backup Section */}
            <div className="config-section">
              <h3 className="config-section-title">Sauvegarde</h3>
              <div className="checkbox-group">
                {Object.entries({
                  questions: 'Questions',
                  teams: 'Equipes',
                  bumpers: 'Joueurs',
                  history: 'Historique',
                  backgrounds: 'Fonds'
                }).map(([key, label]) => (
                  <label key={key} className="checkbox-item">
                    <input
                      type="checkbox"
                      checked={backupOptions[key]}
                      onChange={(e) => setBackupOptions(prev => ({ ...prev, [key]: e.target.checked }))}
                    />
                    <span>{label}</span>
                  </label>
                ))}
              </div>
              <div className="config-section-actions">
                <Button variant="secondary" onClick={handleBackup}>
                  Sauvegarder
                </Button>
                <label className="restore-btn">
                  <input
                    type="file"
                    accept=".tar"
                    onChange={handleRestore}
                    style={{ display: 'none' }}
                  />
                  <Button variant="secondary" as="span">
                    Restaurer
                  </Button>
                </label>
              </div>
            </div>

            {/* Reset Section */}
            <div className="config-section">
              <h3 className="config-section-title">Reinitialisation</h3>
              <div className="checkbox-group">
                {Object.entries({
                  questions: 'Questions',
                  teams: 'Equipes',
                  bumpers: 'Joueurs',
                  history: 'Historique',
                  backgrounds: 'Fonds'
                }).map(([key, label]) => (
                  <label key={key} className="checkbox-item">
                    <input
                      type="checkbox"
                      checked={resetOptions[key]}
                      onChange={(e) => setResetOptions(prev => ({ ...prev, [key]: e.target.checked }))}
                    />
                    <span>{label}</span>
                  </label>
                ))}
              </div>
              <div className="config-section-actions">
                <Button variant="danger" onClick={handleSelectiveReset}>
                  Reinitialiser
                </Button>
              </div>
            </div>
          </Card>
        </section>
      </div>
    </div>
  )
}
