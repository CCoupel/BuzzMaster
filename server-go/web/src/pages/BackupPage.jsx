import { useState, useEffect, useRef } from 'react'
import { motion } from 'framer-motion'
import { useGame } from '../hooks/GameContext'
import Button from '../components/Button'
import Card from '../components/Card'
import './BackupPage.css'

// Cases simples (clé → libellé). Les pools sont rendus à part : case parente « Pools » à trois états (M2 B).
const PLAIN_OPTIONS = {
  questions: 'Questions',
  teams: 'Equipes',
  bumpers: 'Joueurs',
  history: 'Historique',
  medias: 'Médias & Catégories',
  ambiance: 'Réglages de jeu',
}
// Pools (v13.1.0, #324) : clés serveur `rafale` (inchangée, archives existantes) et `picto`.
const POOL_OPTIONS = { rafale: 'Pool Speedy', picto: 'Pool Picto' }
const OPTION_LABELS = { ...PLAIN_OPTIONS, ...POOL_OPTIONS }

/** Case à trois états : cochée si tous les enfants le sont, indéterminée si une partie seulement. */
function ParentCheckbox({ label, checked, indeterminate, onChange }) {
  const ref = useRef(null)
  useEffect(() => { if (ref.current) ref.current.indeterminate = indeterminate }, [indeterminate])
  return (
    <label className="checkbox-item">
      <input ref={ref} type="checkbox" checked={checked} onChange={(e) => onChange(e.target.checked)} />
      <span>{label}</span>
    </label>
  )
}

/** Groupe de cases : options simples, puis « Pools » (parent à trois états) qui déplie Pool Speedy et Pool Picto. */
function OptionGroup({ options, setOptions }) {
  const poolKeys = Object.keys(POOL_OPTIONS)
  const poolsOn = poolKeys.filter(k => options[k]).length
  return (
    <div className="checkbox-group">
      {Object.entries(PLAIN_OPTIONS).map(([key, label]) => (
        <label key={key} className="checkbox-item">
          <input
            type="checkbox"
            checked={options[key]}
            onChange={(e) => setOptions(prev => ({ ...prev, [key]: e.target.checked }))}
          />
          <span>{label}</span>
        </label>
      ))}
      <ParentCheckbox
        label="Pools"
        checked={poolsOn === poolKeys.length}
        indeterminate={poolsOn > 0 && poolsOn < poolKeys.length}
        onChange={(checked) => setOptions(prev => ({ ...prev, ...Object.fromEntries(poolKeys.map(k => [k, checked])) }))}
      />
      {Object.entries(POOL_OPTIONS).map(([key, label]) => (
        <label key={key} className="checkbox-item checkbox-item-child">
          <input
            type="checkbox"
            checked={options[key]}
            onChange={(e) => setOptions(prev => ({ ...prev, [key]: e.target.checked }))}
          />
          <span>{label}</span>
        </label>
      ))}
    </div>
  )
}

export default function BackupPage() {
  const { sendMessage } = useGame()

  // Backup options
  // "ambiance" (#152) : case dédiée pour game-config.json (délai par défaut,
  // effet néon — la config ENTRACTE a rejoint game_state.json avec #119, donc
  // le flag "history" existant). Auparavant rattaché à "history", sans
  // rapport sémantique (relevé par code-reviewer lors de #150) — flag serveur
  // dédié coordonné avec dev-backend, nom convenu "ambiance".
  const [backupOptions, setBackupOptions] = useState({
    questions: true,
    teams: true,
    bumpers: true,
    history: true,
    medias: true,
    ambiance: true,
    rafale: true,
    picto: true,
  })

  // Reset options
  const [resetOptions, setResetOptions] = useState({
    questions: false,
    teams: false,
    bumpers: false,
    history: false,
    medias: false,
    ambiance: false,
    rafale: false,
    picto: false,
  })

  const handleBackup = async () => {
    try {
      // Build URL with selected options
      const params = new URLSearchParams()
      if (backupOptions.questions) params.append('questions', 'true')
      if (backupOptions.teams) params.append('teams', 'true')
      if (backupOptions.bumpers) params.append('bumpers', 'true')
      if (backupOptions.history) params.append('history', 'true')
      if (backupOptions.medias) params.append('medias', 'true')
      if (backupOptions.ambiance) params.append('ambiance', 'true')
      if (backupOptions.rafale) params.append('rafale', 'true')
      if (backupOptions.picto) params.append('picto', 'true')

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

    const labels = OPTION_LABELS
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

            <OptionGroup options={backupOptions} setOptions={setBackupOptions} />

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

            <OptionGroup options={resetOptions} setOptions={setResetOptions} />

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
