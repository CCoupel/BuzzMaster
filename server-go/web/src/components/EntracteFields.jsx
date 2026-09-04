/**
 * EntracteFields — champs partagés de configuration d'un panneau ENTRACTE
 * (#214, milestone v9.0.0), factorisés entre :
 *   - `EntracteConfigForm.jsx` (BackstagePage, config globale — #215)
 *   - `QuestionsPage.jsx` (éditeur d'une entrée ENTRACTE du déroulé — #214,
 *     config PAR OCCURRENCE, même structure de champs, contrat backend
 *     `TypedContent.EntracteConfig *EntracteConfig` — réutilise TEL QUEL le
 *     type Go `EntracteConfig` déjà porté par la config globale, précédent
 *     `TypedContent.MemoryConfig`/`MotionConfig`)
 *
 * Titre/Sous-titre + les 3 sliders (taille du panneau, vitesse/intensité du
 * mouvement, vitesse de transition) — l'image et l'action d'enregistrement
 * restent propres à chaque appelant (mécanismes d'upload et de sauvegarde
 * différents : endpoint dédié + WS global d'un côté, MEDIA générique +
 * multipart `POST /questions` per-occurrence de l'autre).
 *
 * @param {Object} props
 * @param {{title: string, subtitle: string, panelSize: number, animPeriod: number, animIntensity: number, transitionMs: number}} props.values
 * @param {(field: 'title'|'subtitle'|'panelSize'|'animPeriod'|'animIntensity'|'transitionMs', value: string|number) => void} props.onChange
 * @param {string} [props.titleId] - id du champ titre (accessibilité, un seul par page)
 * @param {string} [props.subtitleId] - id du champ sous-titre
 */
export default function EntracteFields({ values, onChange, titleId = 'entracte-title', subtitleId = 'entracte-subtitle' }) {
  return (
    <>
      <div className="wifi-form">
        <label className="wifi-field">
          <span>Titre</span>
          <input
            id={titleId}
            type="text"
            value={values.title}
            onChange={(e) => onChange('title', e.target.value)}
            placeholder="ENTRACTE"
            maxLength={40}
          />
        </label>
        <label className="wifi-field">
          <span>Sous-titre</span>
          <input
            id={subtitleId}
            type="text"
            value={values.subtitle}
            onChange={(e) => onChange('subtitle', e.target.value)}
            placeholder="Retour dans 20mn"
            maxLength={80}
          />
        </label>
      </div>

      <div className="slider-group">
        <div className="slider-row">
          <label>Taille du panneau</label>
          <div className="slider-control">
            <input
              type="range"
              min="20"
              max="100"
              value={values.panelSize}
              onChange={(e) => onChange('panelSize', parseInt(e.target.value))}
            />
            <span className="slider-value">{values.panelSize}%</span>
          </div>
          <p className="section-hint">
            Même réglage, même rendu sur TV et VJoueur — pas de taille séparée par écran.
          </p>
        </div>

        <div className="slider-row">
          <label>Vitesse du mouvement</label>
          <div className="slider-control">
            <input
              type="range"
              min="2"
              max="30"
              value={values.animPeriod}
              onChange={(e) => onChange('animPeriod', parseInt(e.target.value))}
            />
            <span className="slider-value">{values.animPeriod}s</span>
          </div>
          <p className="section-hint">Durée d'un cycle complet — plus court = plus rapide.</p>
        </div>

        <div className="slider-row">
          <label>Intensité du mouvement</label>
          <div className="slider-control">
            <input
              type="range"
              min="0"
              max="100"
              value={values.animIntensity}
              onChange={(e) => onChange('animIntensity', parseInt(e.target.value))}
            />
            <span className="slider-value">
              {values.animIntensity === 0 ? 'animation désactivée' : values.animIntensity}
            </span>
          </div>
        </div>

        <div className="slider-row">
          <label>Vitesse de transition</label>
          <div className="slider-control">
            <input
              type="range"
              min="0"
              max="10000"
              step="100"
              value={values.transitionMs}
              onChange={(e) => onChange('transitionMs', parseInt(e.target.value))}
            />
            <span className="slider-value">
              {values.transitionMs === 0 ? 'transition instantanée' : `${(values.transitionMs / 1000).toFixed(1)}s`}
            </span>
          </div>
          <p className="section-hint">Durée du fondu à l'entrée et à la sortie de la pause.</p>
        </div>
      </div>
    </>
  )
}
