import { useEffect, useState } from 'react'
import '../styles/entracte.css'

// ENTRACTE (v6.5.2, #119) — panneau affiché au-dessus du contenu estompé sur
// TV et VJoueur. Rendu STRICTEMENT identique sur les deux surfaces (D7 du
// plan) : même taille, même centrage — aucune branche isVPlayer ici. C'est
// l'appelant (PlayerDisplay / VPlayerPage) qui décide où le monter, toujours
// en FRÈRE du contenu filtré, jamais en enfant (voir styles/entracte.css).
//
// config attendu : le sous-objet GameState.ENTRACTE_CONFIG tel que reçu par
// useWebSocket (voir hooks/useWebSocket.js) — TITLE, SUBTITLE, IMAGE_IS_CUSTOM,
// PANEL_SIZE, ANIM_PERIOD, ANIM_INTENSITY.
export default function EntractePanel({ config }) {
  const {
    TITLE = 'ENTRACTE',
    SUBTITLE = '',
    IMAGE_IS_CUSTOM = false,
    PANEL_SIZE = 65,
    ANIM_PERIOD = 10,
    ANIM_INTENSITY = 20,
  } = config || {}

  // Cache-buster local à ce montage : l'URL de l'image est stable
  // (/api/config/entracte-image, cf. contrats http-endpoints.md), donc un
  // navigateur qui l'a déjà résolue (ex. en 404 avant upload) doit être forcé
  // à revalider dès que IMAGE_IS_CUSTOM change de valeur. Même principe que
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

  const panelStyle = {
    '--ep-size': `${PANEL_SIZE}%`,
    '--ep-anim-duration': `${ANIM_PERIOD}s`,
    '--ep-anim-intensity': ANIM_INTENSITY,
  }

  return (
    <div className="entracte-panel-stage">
      <div
        className={`entracte-panel${animated ? ' entracte-panel--animated' : ''}`}
        style={panelStyle}
      >
        {IMAGE_IS_CUSTOM && (
          <div
            className="entracte-panel-bg"
            style={{ backgroundImage: `url(/api/config/entracte-image?t=${cacheBuster})` }}
          />
        )}
        <div className="entracte-panel-content">
          <p className="entracte-panel-title">{TITLE}</p>
          {SUBTITLE && <p className="entracte-panel-subtitle">{SUBTITLE}</p>}
        </div>
      </div>
    </div>
  )
}
