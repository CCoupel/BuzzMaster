import { useState } from 'react'
import { motion } from 'framer-motion'
import { useGame } from '../hooks/GameContext'
import Button from '../components/Button'
import Card from '../components/Card'
import './BackupPage.css'

export default function BackupPage() {
  const { sendMessage } = useGame()

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

  return (
    <div className="backup-page page">
      <header className="page-header">
        <h1 className="page-title">Sauvegarde et Restauration</h1>
        <p className="page-subtitle">Gestion des sauvegardes et reinitialisations</p>
      </header>

      <div className="backup-layout">
        {/* Backup Section */}
        <motion.section
          className="backup-section"
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3 }}
        >
          <Card padding="lg" className="backup-card">
            <div className="section-header">
              <h2 className="section-title">Sauvegarde</h2>
              <span className="section-icon">💾</span>
            </div>
            <p className="section-description">
              Selectionnez les elements a sauvegarder et generez un fichier archive.
            </p>

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

            <div className="section-actions">
              <Button variant="primary" onClick={handleBackup}>
                Sauvegarder
              </Button>
            </div>
          </Card>
        </motion.section>

        {/* Restore Section */}
        <motion.section
          className="restore-section"
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3, delay: 0.1 }}
        >
          <Card padding="lg" className="restore-card">
            <div className="section-header">
              <h2 className="section-title">Restauration</h2>
              <span className="section-icon">📂</span>
            </div>
            <p className="section-description">
              Restaurez vos donnees a partir d'un fichier archive anterieur.
            </p>

            <div className="restore-action">
              <label className="restore-btn">
                <input
                  type="file"
                  accept=".tar"
                  onChange={handleRestore}
                  style={{ display: 'none' }}
                />
                <Button variant="primary" as="span">
                  Selectionner un fichier
                </Button>
              </label>
              <p className="restore-hint">Format accepte: .tar</p>
            </div>
          </Card>
        </motion.section>

        {/* Reset Section */}
        <motion.section
          className="reset-section"
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3, delay: 0.2 }}
        >
          <Card padding="lg" className="reset-card">
            <div className="section-header">
              <h2 className="section-title">Reinitialisation</h2>
              <span className="section-icon">🔄</span>
            </div>
            <p className="section-description">
              Reinitialiser selectivement les donnees du systeme.
            </p>

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

            <div className="section-actions">
              <Button variant="danger" onClick={handleSelectiveReset}>
                Reinitialiser
              </Button>
            </div>
          </Card>
        </motion.section>
      </div>
    </div>
  )
}
