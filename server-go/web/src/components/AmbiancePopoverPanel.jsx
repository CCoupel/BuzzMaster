import { Link } from 'react-router-dom'
import LightingModePanel from './LightingModePanel'
import SoundTogglePanel from './SoundTogglePanel'
import './LightingModePanel.css'

// #286 / #284 — contenu du popover « Ambiance » de l'en-tête (ex-« Éclairage »),
// contracts/sound.md §11.3. Deux sections, chacune absente si sa fonction
// n'existe pas :
//  - « Éclairage général » : LightingModePanel (ON/AUTO/OFF + Flash) tant qu'un
//    pont est configuré et l'éclairage d'ambiance activé ; grisé + renvoi vers
//    Ambiance › Lumière quand `lighting.enabled` a été coupé (§11.4) ;
//  - « Son » : ON/OFF (sound.muted), présent dès que le son est actif ou coupé.

export default function AmbiancePopoverPanel({
  showLighting, lightingSwitchedOff, showSound, soundMuted, refreshSound, ambianceHref,
}) {
  return (
    <div className="ambiance-popover-panel">
      <h3 className="lighting-mode-title">Ambiance — conduite en direct</h3>
      {showLighting && (
        lightingSwitchedOff ? (
          <div className="ambiance-popover-lighting is-muted">
            <p className="navbar-popover-section-title">Éclairage général</p>
            <p className="navbar-popover-hint" role="status">
              Éclairage d'ambiance <strong>désactivé</strong> — réactivez-le dans{' '}
              <Link to={ambianceHref}>Ambiance › Lumière</Link>. Les commandes ON / AUTO / OFF et Flash n'ont aucun effet.
            </p>
          </div>
        ) : (
          <div className="ambiance-popover-lighting">
            <p className="navbar-popover-section-title">Éclairage général</p>
            <LightingModePanel bare />
          </div>
        )
      )}
      {showSound && <SoundTogglePanel muted={soundMuted} refresh={refreshSound} />}
    </div>
  )
}
