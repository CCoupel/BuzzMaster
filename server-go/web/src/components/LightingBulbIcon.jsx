import { lightingStateGlyph } from '../utils/lightingState'
import './LightingBulbIcon.css'

// #207 — ampoule de l'entrée « Ambiance » du menu abeille.
//
// Trois SVG en ligne tracés en `currentColor` (contrat hue-bridge.md §7.1,
// maquette §01 rév. 4). Un emoji 💡 ne permettrait ni de changer la couleur
// (fixée par la police système) ni la forme. Tracés repris de la maquette.
//
// La forme seule distingue les états — lisible en niveaux de gris et pour un
// daltonien : rayons → contour + pastille → contour nu. Le contour nu ne porte
// JAMAIS de pastille : une fonctionnalité facultative non configurée ne doit
// pas ressembler à une alerte.
//
// `aria-hidden` : le libellé « Ambiance » et le `title` de l'entrée portent le
// sens (Navbar.jsx).

const BULB_PATH = 'M12 4a6 6 0 0 0-3.5 10.9V17a1 1 0 0 0 1 1h5a1 1 0 0 0 1-1v-2.1A6 6 0 0 0 12 4Z'

export default function LightingBulbIcon({ state, className = '' }) {
  const glyph = lightingStateGlyph(state)

  return (
    <svg
      className={`lighting-bulb-icon lighting-bulb-${glyph} ${className}`.trim()}
      viewBox="0 0 24 24"
      aria-hidden="true"
      focusable="false"
      data-glyph={glyph}
    >
      {glyph === 'lit' ? (
        <>
          <g fill="currentColor" className="lighting-bulb-rays">
            <rect x="11.2" y="0" width="1.6" height="3.2" rx=".8" />
            <rect x="1.6" y="10.6" width="3.2" height="1.6" rx=".8" />
            <rect x="19.2" y="10.6" width="3.2" height="1.6" rx=".8" />
            <rect x="3.4" y="3.1" width="1.6" height="3.2" rx=".8" transform="rotate(-45 4.2 4.7)" />
            <rect x="19" y="3.1" width="1.6" height="3.2" rx=".8" transform="rotate(45 19.8 4.7)" />
          </g>
          <path fill="currentColor" d={BULB_PATH} />
          <rect fill="currentColor" x="9.6" y="19.3" width="4.8" height="1.5" rx=".75" />
          <rect fill="currentColor" x="10.3" y="21.5" width="3.4" height="1.5" rx=".75" />
        </>
      ) : (
        <>
          <path fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinejoin="round" d={BULB_PATH} />
          <rect fill="currentColor" x="9.6" y="19.3" width="4.8" height="1.5" rx=".75" />
          {glyph === 'alert' && (
            <circle className="lighting-bulb-alert-dot" fill="currentColor" cx="19.3" cy="4.7" r="3.4" />
          )}
        </>
      )}
    </svg>
  )
}
