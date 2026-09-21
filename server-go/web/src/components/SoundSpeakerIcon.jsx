import { soundStateGlyph } from '../utils/soundState'
import './SoundSpeakerIcon.css'

// #234 — haut-parleur de l'entrée « Ambiance » du menu abeille, à côté de
// l'ampoule Hue (LightingBulbIcon.jsx, #207). Même patron exactement :
// SVG en ligne tracé en `currentColor`, `aria-hidden`, `data-glyph`.
//
// DEUX glyphes, pas trois : la forme seule distingue les états (héritage de
// #207, lisible en niveaux de gris et pour un daltonien).
//   on  = haut-parleur AVEC ondes  → le son qui sort
//   off = haut-parleur NU, sans ondes, **JAMAIS barré** — une barre oblique
//         signale une anomalie, or « inactif » recouvre aussi le cas
//         parfaitement normal où l'utilisateur a éteint les bruitages.
//         C'est exactement le piège que #207 avait identifié pour le
//         contour nu de l'ampoule (jamais de pastille dessus).
//
// `aria-hidden` : le libellé « Ambiance » et le `title` de l'entrée portent
// le sens (Navbar.jsx).

// Corps du haut-parleur (boîtier + cône), identique dans les deux états —
// seules les ondes apparaissent/disparaissent.
const SPEAKER_PATH = 'M4 9v6h3l5 4V5L7 9H4Z'

export default function SoundSpeakerIcon({ active, className = '' }) {
  const glyph = soundStateGlyph(active)

  return (
    <svg
      className={`sound-speaker-icon sound-speaker-${glyph} ${className}`.trim()}
      viewBox="0 0 24 24"
      aria-hidden="true"
      focusable="false"
      data-glyph={glyph}
    >
      <path fill="currentColor" d={SPEAKER_PATH} />
      {glyph === 'on' && (
        <g className="sound-speaker-waves" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round">
          <path d="M14.5 7.5a5 5 0 0 1 0 9" />
          <path d="M17.2 5a8.5 8.5 0 0 1 0 14" />
        </g>
      )}
    </svg>
  )
}
