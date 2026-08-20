import { useEffect, useState } from 'react'
import { motion } from 'framer-motion'
import '../styles/entracte.css'

// ENTRACTE (v6.5.2, #119) — panneau affiché au-dessus du contenu estompé sur
// TV et VJoueur. Rendu STRICTEMENT identique sur les deux surfaces (D7 du
// plan) : même taille, même centrage — aucune branche isVPlayer ici. C'est
// l'appelant (PlayerDisplay / VPlayerPage) qui décide où le monter, toujours
// en FRÈRE du contenu filtré, jamais en enfant (voir styles/entracte.css).
//
// config attendu : le sous-objet GameState.ENTRACTE_CONFIG tel que reçu par
// useWebSocket (voir hooks/useWebSocket.js) — TITLE, SUBTITLE, IMAGE_IS_CUSTOM,
// PANEL_SIZE, ANIM_PERIOD, ANIM_INTENSITY, TRANSITION_MS. C'est la config
// DIFFUSÉE (gelée pendant une pause active, corrections C4) — jamais
// entracteConfigSaved, qui n'est consommé que par le formulaire d'édition.
//
// Transition (corrections C3) : ce composant DOIT être monté par l'appelant
// à l'intérieur d'un <AnimatePresence> entourant son rendu conditionnel
// ({entracteActive && <EntractePanel .../>}) — sans quoi le panneau disparaît
// instantanément à la sortie, quelle que soit TRANSITION_MS, faute d'exister
// encore pendant le fondu. Le composant expose sa propre clé implicite via
// motion.div ; l'appelant doit lui passer une prop `key` stable.
export default function EntractePanel({ config }) {
  const {
    TITLE = 'ENTRACTE',
    SUBTITLE = '',
    IMAGE_IS_CUSTOM = false,
    PANEL_SIZE = 65,
    ANIM_PERIOD = 10,
    ANIM_INTENSITY = 20,
    TRANSITION_MS = 2000,
  } = config || {}

  // Cache-buster local à ce montage : l'URL de l'image est stable
  // (/api/game/entracte-image, cf. contrats http-endpoints.md — renommé
  // depuis /api/config/entracte-image par les corrections C1, l'image
  // appartient désormais à la partie), donc un navigateur qui l'a déjà
  // résolue (ex. en 404 avant upload) doit être forcé à revalider dès que
  // IMAGE_IS_CUSTOM change de valeur. Même principe que
  // defaultImageCacheBuster dans ConfigPage.jsx, mais réagissant ici à l'état
  // reçu du serveur plutôt qu'à une action d'upload locale.
  const [cacheBuster, setCacheBuster] = useState(() => Date.now())
  useEffect(() => {
    setCacheBuster(Date.now())
  }, [IMAGE_IS_CUSTOM])

  // ANIM_INTENSITY = 0 => aucune classe/animation appliquée (pas une
  // animation d'amplitude nulle qui tournerait quand même en boucle pendant
  // toute la pause, D8).
  const animated = Number(ANIM_INTENSITY) > 0

  // Fondu d'entrée/sortie (C3) — porté par .entracte-panel-stage (le
  // conteneur), jamais par .entracte-panel lui-même : celui-ci porte déjà
  // l'animation CSS de respiration (transform) + will-change:transform,
  // propriétés disjointes de l'opacité, aucun conflit à éviter en les
  // séparant. Durée bornée à >= 0 (TRANSITION_MS = 0 → fondu instantané,
  // l'échappatoire documentée par le contrat).
  const transitionSec = Math.max(0, Number(TRANSITION_MS) || 0) / 1000

  const panelStyle = {
    '--ep-size': `${PANEL_SIZE}%`,
    '--ep-anim-duration': `${ANIM_PERIOD}s`,
    '--ep-anim-intensity': ANIM_INTENSITY,
  }
  // --ep-transition exposée séparément sur .entracte-panel-stage (le
  // conteneur du fondu) : ne PAS la fusionner dans panelStyle ci-dessus,
  // qui reste réservé aux variables de taille/animation lues directement sur
  // .entracte-panel (tests, et cohérence avec le composant avant C3).
  const stageStyle = { '--ep-transition': `${TRANSITION_MS}ms` }

  return (
    <motion.div
      className="entracte-panel-stage"
      style={stageStyle}
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: transitionSec }}
    >
      <div className={`entracte-panel${animated ? ' entracte-panel--animated' : ''}`} style={panelStyle}>
        {IMAGE_IS_CUSTOM && (
          <div
            className="entracte-panel-bg"
            style={{ backgroundImage: `url(/api/game/entracte-image?t=${cacheBuster})` }}
          />
        )}
        <div className="entracte-panel-content">
          <p className="entracte-panel-title">{TITLE}</p>
          {SUBTITLE && <p className="entracte-panel-subtitle">{SUBTITLE}</p>}
        </div>
      </div>
    </motion.div>
  )
}
