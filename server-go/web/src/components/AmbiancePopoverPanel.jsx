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

// #290 (LOT-4, T22) — `ambiance` = lecture de GameState.AMBIANCE_EFFECTIVE (réglages RÉSOLUS global ← manche ←
// direct) : { settings, sources, roundId, roundName, roundCount }. Absent (serveur ancien / pas encore reçu) ⇒
// rendu v11.4.0 à l'identique. Le popover n'embarque pas le panneau mutualisé : il n'en lit que le modèle.
export default function AmbiancePopoverPanel({
  showLighting, lightingSwitchedOff, showSound, soundMuted, refreshSound, ambianceHref, ambiance = null,
}) {
  const sources = ambiance?.sources || null
  const roundActive = !!ambiance?.roundId
  const cd = ambiance?.settings?.sound?.countdown
  const tag = (src) => (src === 'round' ? <span className="ambiance-source-tag"> manche</span> : null)
  return (
    <div className="ambiance-popover-panel">
      <h3 className="lighting-mode-title">Ambiance — conduite en direct</h3>
      {roundActive && ambiance.roundCount > 0 && (
        <div className="ambiance-round-line" role="status">
          Manche « {ambiance.roundName} » : {ambiance.roundCount} réglage{ambiance.roundCount > 1 ? 's' : ''} imposé{ambiance.roundCount > 1 ? 's' : ''}
        </div>
      )}
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
            <p className="navbar-popover-section-title">Éclairage général{tag(sources?.light?.mode)}</p>
            <LightingModePanel bare />
          </div>
        )
      )}
      {showSound && (
        <SoundTogglePanel
          muted={soundMuted}
          refresh={refreshSound}
          source={sources?.sound?.muted || 'global'}
          roundActive={roundActive}
        />
      )}
      {showSound && cd && (
        <p className="navbar-popover-hint ambiance-countdown-line">
          Décompte : <b>{cd.enabled === false ? 'inactif' : 'actif'}{cd.enabled === false ? '' : (cd.loop === false ? ', une fois' : ', en boucle')}</b>
          {tag(sources?.sound?.countdown?.enabled === 'round' || sources?.sound?.countdown?.loop === 'round' ? 'round' : null)}
        </p>
      )}
      {ambiance && (
        <p className="navbar-popover-hint">
          {roundActive && ambiance.roundCount > 0
            ? 'Un geste ici l\'emporte sur la manche jusqu\'au prochain changement de manche. Réglages de la manche : éditeur de manches › 🎛.'
            : 'Réglages détaillés : Ambiance › Son / Lumière.'}
        </p>
      )}
    </div>
  )
}
