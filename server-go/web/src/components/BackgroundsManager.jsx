import { useState, useRef } from 'react'
import { motion } from 'framer-motion'
import Button from './Button'
import Card, { CardHeader, CardBody } from './Card'

/**
 * BackgroundsManager — gestion d'un jeu d'images de fond, mutualisée entre
 * les deux zones historiques de QuestionsPage.jsx (#215, onglet "Fonds
 * d'écran" de BackstagePage) :
 *   - Ambiance pendant le jeu       → destination="background"
 *   - Écran d'accueil Nouvelle Partie → destination="new-game-backgrounds"
 *
 * Les deux blocs étaient quasi identiques dans QuestionsPage.jsx (le second
 * portait même un commentaire indiquant qu'il reproduisait le premier) — ce
 * composant unique, paramétré par `destination`, supprime la duplication
 * (maquette #215 §06). Aucun contrat modifié : mêmes endpoints
 * `GET/POST/PUT/DELETE /background` et `/new-game-backgrounds` qu'avant.
 */
export default function BackgroundsManager({ destination, backgrounds, title, hint, emptyLabel }) {
  const endpoint = `/${destination}`
  const [uploading, setUploading] = useState(false)
  const [draggedIndex, setDraggedIndex] = useState(null)
  const inputRef = useRef(null)

  const handleUpload = async (e) => {
    const file = e.target.files?.[0]
    if (!file) return
    setUploading(true)
    const fd = new FormData()
    fd.append('file', file)
    try {
      const response = await fetch(endpoint, { method: 'POST', body: fd })
      if (response.ok) {
        window.location.reload()
      } else {
        const text = await response.text()
        alert('Erreur: ' + text)
      }
    } catch (error) {
      console.error(`Background upload failed (${endpoint}):`, error)
      alert('Erreur: ' + error.message)
    } finally {
      setUploading(false)
      if (inputRef.current) inputRef.current.value = ''
    }
  }

  const handleRemove = async (bgPath) => {
    if (!window.confirm('Supprimer cette image de fond ?')) return
    try {
      const filename = bgPath.split('/').pop()
      await fetch(`${endpoint}?file=${encodeURIComponent(filename)}`, { method: 'DELETE' })
    } catch (error) {
      console.error(`Remove background failed (${endpoint}):`, error)
    }
  }

  const handleRemoveAll = async () => {
    if (!window.confirm('Supprimer toutes les images de fond ?')) return
    try {
      await fetch(endpoint, { method: 'DELETE' })
    } catch (error) {
      console.error(`Remove all backgrounds failed (${endpoint}):`, error)
    }
  }

  const save = async (next) => {
    try {
      await fetch(endpoint, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(next),
      })
    } catch (error) {
      console.error(`Save backgrounds failed (${endpoint}):`, error)
    }
  }

  const handleDurationChange = async (index, newDuration) => {
    const next = [...(backgrounds || [])]
    next[index] = { ...next[index], duration: parseInt(newDuration) || 10 }
    await save(next)
  }

  const handleOpacityChange = async (index, newOpacity) => {
    const next = [...(backgrounds || [])]
    next[index] = { ...next[index], opacity: Math.max(0, Math.min(100, parseInt(newOpacity) || 100)) }
    await save(next)
  }

  const handleMove = async (fromIndex, toIndex) => {
    if (fromIndex === toIndex) return
    const next = [...(backgrounds || [])]
    const [moved] = next.splice(fromIndex, 1)
    next.splice(toIndex, 0, moved)
    await save(next)
  }

  return (
    <section className="background-section">
      <Card padding="lg">
        <CardHeader>
          <div className="section-header">
            <h3 className="section-title">{title}</h3>
            <div style={{ display: 'flex', gap: 'var(--space-2)' }}>
              <label className="upload-bg-btn">
                <input
                  type="file"
                  ref={inputRef}
                  accept="image/*"
                  onChange={handleUpload}
                  style={{ display: 'none' }}
                />
                <Button variant="primary" size="sm" as="span" loading={uploading}>
                  + Image
                </Button>
              </label>
              {backgrounds?.length > 0 && (
                <Button variant="ghost" size="sm" onClick={handleRemoveAll}>
                  Tout supprimer
                </Button>
              )}
            </div>
          </div>
        </CardHeader>
        <CardBody>
          {hint && <p className="new-game-bg-hint">{hint}</p>}
          <p className="section-hint">Glissez-deposez pour changer l'ordre.</p>
          <div className="backgrounds-grid">
            {backgrounds?.length > 0 ? (
              backgrounds.map((bg, index) => (
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
                      handleMove(draggedIndex, index)
                    }
                  }}
                >
                  <img src={bg.path} alt={`${title} ${index + 1}`} className="bg-thumb" />
                  <button
                    className="bg-delete-btn"
                    onClick={() => handleRemove(bg.path)}
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
              <div className="backgrounds-empty">
                <p className="empty-state">{emptyLabel}</p>
              </div>
            )}
          </div>
        </CardBody>
      </Card>
    </section>
  )
}
