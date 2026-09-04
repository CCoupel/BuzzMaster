import { useState, useEffect, useRef } from 'react'
import Button from './Button'
import Card, { CardHeader, CardBody } from './Card'
import EntracteFields from './EntracteFields'

/**
 * EntracteConfigForm — onglet "Entracte" de BackstagePage (#215, extrait de
 * QuestionsPage.jsx où il vivait historiquement dans la "Zone 1").
 *
 * Configuration du panneau de pause globale (#119, corrections C1/C4),
 * propriété de la partie — le déclenchement lui-même reste dans la barre de
 * navigation (bouton ENTRACTE / FIN D'ENTRACTE, C2, Navbar.jsx).
 *
 * ⚠️ C4 — alimenté depuis `gameState.entracteConfigSaved` (la config
 * ENREGISTRÉE, toujours à jour), JAMAIS `gameState.entracteConfig` (la config
 * DIFFUSÉE au panneau, gelée pendant une pause active) : sinon un
 * enregistrement fait pendant l'entracte semblerait perdu au retour sur cette
 * page. Piège documenté à nouveau ici — voir la maquette #215 §05.
 */
export default function EntracteConfigForm({ gameState, sendMessage }) {
  const savedEntracteCfg = gameState.entracteConfigSaved || {}
  const [entracteTitle, setEntracteTitle] = useState(savedEntracteCfg.TITLE || 'ENTRACTE')
  const [entracteSubtitle, setEntracteSubtitle] = useState(savedEntracteCfg.SUBTITLE || 'Retour dans 20mn')
  const [entracteImageIsCustom, setEntracteImageIsCustom] = useState(savedEntracteCfg.IMAGE_IS_CUSTOM || false)
  const [entractePanelSize, setEntractePanelSize] = useState(savedEntracteCfg.PANEL_SIZE ?? 65)
  const [entracteAnimPeriod, setEntracteAnimPeriod] = useState(savedEntracteCfg.ANIM_PERIOD ?? 10)
  const [entracteAnimIntensity, setEntracteAnimIntensity] = useState(savedEntracteCfg.ANIM_INTENSITY ?? 20)
  const [entracteTransitionMs, setEntracteTransitionMs] = useState(savedEntracteCfg.TRANSITION_MS ?? 2000)
  const [entracteSaved, setEntracteSaved] = useState(false)
  const [entracteImageCacheBuster, setEntracteImageCacheBuster] = useState(() => Date.now())
  const [uploadingEntracteImage, setUploadingEntracteImage] = useState(false)
  const [deletingEntracteImage, setDeletingEntracteImage] = useState(false)
  const [entracteImageToast, setEntracteImageToast] = useState(null)
  const entracteImageFileRef = useRef(null)

  // C1/C4 — sync depuis gameState.entracteConfigSaved (jamais entracteConfig,
  // voir commentaire de déclaration d'état ci-dessus).
  useEffect(() => {
    const cfg = gameState.entracteConfigSaved
    if (!cfg) return
    setEntracteTitle(cfg.TITLE ?? 'ENTRACTE')
    setEntracteSubtitle(cfg.SUBTITLE ?? 'Retour dans 20mn')
    setEntracteImageIsCustom(cfg.IMAGE_IS_CUSTOM ?? false)
    setEntractePanelSize(cfg.PANEL_SIZE ?? 65)
    setEntracteAnimPeriod(cfg.ANIM_PERIOD ?? 10)
    setEntracteAnimIntensity(cfg.ANIM_INTENSITY ?? 20)
    setEntracteTransitionMs(cfg.TRANSITION_MS ?? 2000)
  }, [gameState.entracteConfigSaved])

  // Toast auto-hide (même patron que les autres toasts du projet)
  useEffect(() => {
    if (entracteImageToast) {
      const timer = setTimeout(() => setEntracteImageToast(null), 3000)
      return () => clearTimeout(timer)
    }
  }, [entracteImageToast])

  // ENTRACTE (#119, C1) — action dédiée, distincte d'UPDATE_QUIZ_META.
  // Acceptée par le serveur même pendant un entracte actif (C4) — écrit
  // ENTRACTE_CONFIG_SAVED sans rafraîchir le panneau déjà diffusé.
  const handleSaveEntracteConfig = (e) => {
    e.preventDefault()
    sendMessage('UPDATE_ENTRACTE_CONFIG', {
      TITLE: entracteTitle,
      SUBTITLE: entracteSubtitle,
      PANEL_SIZE: entractePanelSize,
      ANIM_PERIOD: entracteAnimPeriod,
      ANIM_INTENSITY: entracteAnimIntensity,
      TRANSITION_MS: entracteTransitionMs,
    })
    setEntracteSaved(true)
    setTimeout(() => setEntracteSaved(false), 2000)
  }

  // Image de fond du panneau — endpoint /api/game/entracte-image (C1-B6,
  // l'image appartient à la partie, pas à la config serveur).
  const handleEntracteImageUpload = async () => {
    const file = entracteImageFileRef.current?.files?.[0]
    if (!file) {
      setEntracteImageToast({ message: 'Veuillez selectionner une image', type: 'error' })
      return
    }
    const allowed = ['.jpg', '.jpeg', '.png', '.gif', '.webp', '.svg']
    const ext = '.' + file.name.split('.').pop().toLowerCase()
    if (!allowed.includes(ext)) {
      setEntracteImageToast({ message: 'Format non supporte. Utilisez jpg, png, gif, webp ou svg', type: 'error' })
      return
    }
    setUploadingEntracteImage(true)
    try {
      const formData = new FormData()
      formData.append('file', file)
      const res = await fetch('/api/game/entracte-image', { method: 'POST', body: formData })
      const data = await res.json()
      if (res.ok && (data.is_custom || data.image_is_custom)) {
        setEntracteImageIsCustom(true)
        setEntracteImageCacheBuster(Date.now())
        setEntracteImageToast({ message: 'Image d\'entracte enregistree', type: 'success' })
        if (entracteImageFileRef.current) entracteImageFileRef.current.value = ''
      } else {
        setEntracteImageToast({ message: 'Erreur lors de l\'upload', type: 'error' })
      }
    } catch (err) {
      setEntracteImageToast({ message: 'Erreur reseau: ' + err.message, type: 'error' })
    } finally {
      setUploadingEntracteImage(false)
    }
  }

  const handleEntracteImageDelete = async () => {
    if (!window.confirm('Retirer l\'image d\'entracte ? Le panneau restera lisible sans image.')) return
    setDeletingEntracteImage(true)
    try {
      const res = await fetch('/api/game/entracte-image', { method: 'DELETE' })
      if (res.ok) {
        setEntracteImageIsCustom(false)
        setEntracteImageCacheBuster(Date.now())
        setEntracteImageToast({ message: 'Image d\'entracte supprimee', type: 'success' })
      } else {
        setEntracteImageToast({ message: 'Erreur lors de la suppression', type: 'error' })
      }
    } catch (err) {
      setEntracteImageToast({ message: 'Erreur reseau: ' + err.message, type: 'error' })
    } finally {
      setDeletingEntracteImage(false)
    }
  }

  return (
    <section className="entracte-section">
      <Card padding="lg">
        <CardBody>
          {/* ENTRACTE (#119, corrections C1) — configuration du panneau de
              pause globale, propriété de la partie. Le déclenchement lui-même
              se fait depuis la barre de navigation (bouton ENTRACTE / FIN
              D'ENTRACTE, C2). */}
          <form onSubmit={handleSaveEntracteConfig} className="new-game-bg-section">
            <div className="new-game-bg-header">
              <h4 className="new-game-bg-title">Entracte (pause globale)</h4>
            </div>
            <p className="new-game-bg-hint">
              Panneau affiche sur l'ecran TV et VJoueur pendant une pause globale (repas, changement de salle...), declenchee depuis la barre de navigation. Le reste de l'interface (TV, VJoueur, admin, animateur) est estompe pendant toute la duree de la pause.
            </p>

            <EntracteFields
              values={{
                title: entracteTitle,
                subtitle: entracteSubtitle,
                panelSize: entractePanelSize,
                animPeriod: entracteAnimPeriod,
                animIntensity: entracteAnimIntensity,
                transitionMs: entracteTransitionMs,
              }}
              onChange={(field, value) => {
                if (field === 'title') setEntracteTitle(value)
                else if (field === 'subtitle') setEntracteSubtitle(value)
                else if (field === 'panelSize') setEntractePanelSize(value)
                else if (field === 'animPeriod') setEntracteAnimPeriod(value)
                else if (field === 'animIntensity') setEntracteAnimIntensity(value)
                else if (field === 'transitionMs') setEntracteTransitionMs(value)
              }}
            />

            <div className="default-image-preview">
              {entracteImageIsCustom ? (
                <img
                  src={`/api/game/entracte-image?t=${entracteImageCacheBuster}`}
                  alt="Image d'entracte"
                  className="default-image-thumbnail"
                />
              ) : (
                <span className="default-image-filename">Aucune image (panneau sans fond)</span>
              )}
              {entracteImageIsCustom && (
                <span className="default-image-filename">Image personnalisée</span>
              )}
            </div>

            <div className="firmware-upload-row">
              <input
                ref={entracteImageFileRef}
                type="file"
                accept=".jpg,.jpeg,.png,.gif,.webp,.svg"
                className="firmware-file-input"
                id="entracte-image-file-input"
              />
              <label htmlFor="entracte-image-file-input" className="firmware-file-label">
                Choisir une image (jpg, png, gif, webp, svg)
              </label>
            </div>

            <div className="config-section-actions">
              <Button type="button" variant="primary" onClick={handleEntracteImageUpload} loading={uploadingEntracteImage}>
                Enregistrer l'image
              </Button>
              {entracteImageIsCustom && (
                <Button type="button" variant="secondary" onClick={handleEntracteImageDelete} loading={deletingEntracteImage}>
                  Retirer l'image
                </Button>
              )}
            </div>

            <div className="config-section-actions">
              <Button type="submit" variant="primary">
                {entracteSaved ? 'Enregistré ✓' : 'Enregistrer'}
              </Button>
              {gameState.entracte && (
                <span className="section-hint" role="status">
                  Un entracte est en cours — prendra effet au prochain entracte.
                </span>
              )}
            </div>
          </form>
        </CardBody>
      </Card>

      {entracteImageToast && (
        <div className={`wifi-toast wifi-toast-${entracteImageToast.type}`}>
          {entracteImageToast.message}
        </div>
      )}
    </section>
  )
}
