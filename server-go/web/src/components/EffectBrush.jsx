import ColorWheel from './ColorWheel'

// #210 (v11.8.0, LOT-2) — pinceau de la grille : couleur (nommées, libre, éteint, gomme) et intensité, avec info-bulles
// (définitions du §4.2, maquette effect_editor §02). Les info-bulles s'ouvrent au survol ET au focus clavier.

export const BRUSH_TIPS = [
  { key: 'theme', label: 'Thème', tip: "La couleur du thème de la question en cours. Blanc si la question n'a pas de thème." },
  { key: 'equipe', label: 'Équipe', tip: "La couleur de l'équipe qui vient de marquer. Ne sert qu'au moment « Équipe qui marque » ; blanc aux autres moments." },
  { key: 'urgence', label: 'Urgence', tip: 'Suit le chronomètre : blanc, puis orange quand il reste 10 secondes, puis rouge quand il en reste 5. Blanc hors chronomètre. La couleur change toute seule, sans nouveau pas.' },
  { key: 'scene', label: 'Scène', tip: "La couleur que la salle aurait sans effet à ce moment du jeu. Blanc pour les zones Animateur et Public." },
  { key: 'free', label: 'Couleur libre', tip: 'Une couleur fixe que vous choisissez sur la roue. Elle ne dépend ni de la question ni du chronomètre.' },
  { key: 'off', label: 'Éteint', tip: "Éteint le groupe. Il reste éteint jusqu'à ce qu'un pas lui redonne une couleur." },
  { key: 'erase', label: 'Gomme', tip: "Vide la case : ce pas n'écrit plus rien sur ce groupe, qui garde l'état qu'il avait au pas d'avant." },
]
export const INTENSITY_TIP = "Puissance de l'éclairage du groupe, de 0 à 100 %. 0 % revient à éteindre. L'intensité s'applique à toutes les couleurs."
export const INTENSITIES = [25, 50, 75, 100]

// Couleurs d'affichage des couleurs nommées (indicatif : le serveur les résout vivantes à chaque rendu).
export const NAMED_RGB = { theme: '#4F46E5', equipe: '#E53935', urgence: '#FFFFFF', scene: '#E2E8F0' }

export const DEFAULT_BRUSH = Object.freeze({ color: 'urgence', intensity: 100, free: '#14B8A6' })

// Action écrite par le pinceau sur une cible (null = gomme).
export function brushToAction(target, brush) {
  if (brush.color === 'erase') return null
  if (brush.color === 'off') return { target, off: true }
  const color = brush.color === 'free' ? brush.free : brush.color
  return { target, color, intensity: Math.max(0, Math.min(100, Math.round(Number(brush.intensity) || 0))) }
}

export default function EffectBrush({ brush, onChange, disabled = false }) {
  const set = (patch) => onChange({ ...brush, ...patch })
  const swatch = (key) => (key === 'off' ? '#0f172a' : key === 'free' ? brush.free : key === 'erase' ? null : NAMED_RGB[key])

  return (
    <div className="efx-brush-panel" data-testid="effect-brush">
      <h3 className="efx-h3">Pinceau</h3>
      <div className="efx-brush" role="group" aria-label="Couleur du pinceau">
        {BRUSH_TIPS.map(b => (
          <button
            key={b.key} type="button" className={`efx-swl${brush.color === b.key ? ' is-sel' : ''}`} data-tip={b.tip}
            aria-pressed={brush.color === b.key} aria-label={`${b.label} — ${b.tip}`} disabled={disabled}
            onClick={() => set({ color: b.key })}
          >
            <span className={`efx-sw${b.key === 'erase' ? ' is-erase' : ''}`} style={swatch(b.key) ? { background: swatch(b.key) } : undefined} />
            {b.label.toLowerCase()}
          </button>
        ))}
      </div>
      <div className="efx-brush efx-brush-i">
        <span className="efx-hint-i" tabIndex={0} data-tip={INTENSITY_TIP} aria-label={`Intensité — ${INTENSITY_TIP}`}>Intensité ⓘ</span>
        {INTENSITIES.map(i => (
          <button
            key={i} type="button" className={`efx-chip${brush.intensity === i ? ' is-on' : ''}`} aria-pressed={brush.intensity === i}
            disabled={disabled} onClick={() => set({ intensity: i })}
          >{i} %</button>
        ))}
        <input
          className="efx-ifree" type="number" min="0" max="100" aria-label="Autre intensité, de 0 à 100" value={brush.intensity} disabled={disabled}
          onChange={(e) => set({ intensity: Math.max(0, Math.min(100, parseInt(e.target.value, 10) || 0)) })}
        /> %
      </div>
      {brush.color === 'free' && !disabled && <ColorWheel value={brush.free} onChange={(hex) => set({ free: hex })} />}
      <p className="efx-hint">
        Choisissez une couleur et une intensité, puis cliquez les cases. La roue apparaît quand le pinceau « couleur libre » est choisi :
        cliquez dans la roue (teinte et saturation), ou saisissez le code de la couleur.
      </p>
    </div>
  )
}
