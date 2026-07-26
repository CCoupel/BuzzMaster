/**
 * Color utility functions for BuzzControl
 */

/**
 * Boost team color saturation/lightness for display (#61, widened #113).
 * Converts an RGB array to HSL, enforces a minimum saturation of 95%
 * and clamps lightness to [35%, 65%] for vivid, non-pastel colors.
 * Does NOT modify the stored values — display only.
 *
 * The 16-color team palette (constants/colors.js, TEAM_COLORS) is built at
 * S=100% and L=55% (vif) / L=35% (profond) — exactly within these bounds —
 * so it passes through unchanged (invariance property, contracts/models.md
 * "Palette d'équipes"). Widening from [45%,55%] to [35%,65%] is what lets
 * the deep tones (L=35%) survive the boost instead of being clamped up to
 * 45% and drifting away from their contractual RGB value.
 *
 * @param {Array} rgbArray - [r, g, b] array (0-255)
 * @returns {string|null} Boosted "rgb(r,g,b)" string, or null on invalid input
 */
export function boostTeamColor(rgbArray) {
  if (!rgbArray || !Array.isArray(rgbArray) || rgbArray.length < 3) return null
  const [r, g, b] = rgbArray.map(v => v / 255)
  const max = Math.max(r, g, b)
  const min = Math.min(r, g, b)
  const delta = max - min
  let h = 0, s = 0, l = (max + min) / 2

  if (delta !== 0) {
    s = delta / (1 - Math.abs(2 * l - 1))
    if (max === r) h = ((g - b) / delta) % 6
    else if (max === g) h = (b - r) / delta + 2
    else h = (r - g) / delta + 4
    // Fix (#113) : NE PAS arrondir h à un degré entier ici — cet arrondi
    // intermédiaire cassait l'invariance RGB→HSL→RGB pour toute teinte non
    // multiple de 60° (ex. 222°, 275°, 325°, 28°), décalant le résultat de
    // ±1 sur un canal (bleu/violet/rose/orange-profond de la palette #113
    // divergeaient chacun de 1 du RGB stocké). h ne sert qu'en interne pour
    // reconstruire r1/g1/b1 ci-dessous — seul le résultat final est arrondi
    // (toInt), donc garder h en flottant ne change rien au format de sortie.
    h = h * 60
    if (h < 0) h += 360
  }

  s = Math.max(s, 0.95)        // was 0.70 — enforce vivid, non-pastel saturation
  l = Math.min(Math.max(l, 0.35), 0.65) // was [0.45, 0.55] (#113) — lets deep tones (L=35%) through unclamped

  const c = (1 - Math.abs(2 * l - 1)) * s
  const x = c * (1 - Math.abs((h / 60) % 2 - 1))
  const m = l - c / 2
  let r1 = 0, g1 = 0, b1 = 0
  if (h < 60)       { r1 = c; g1 = x }
  else if (h < 120) { r1 = x; g1 = c }
  else if (h < 180) { g1 = c; b1 = x }
  else if (h < 240) { g1 = x; b1 = c }
  else if (h < 300) { r1 = x; b1 = c }
  else              { r1 = c; b1 = x }

  const toInt = v => Math.round((v + m) * 255)
  return `rgb(${toInt(r1)},${toInt(g1)},${toInt(b1)})`
}

/**
 * Convert color to RGB string
 * Handles multiple input formats:
 * - Array [r, g, b] → boosted "rgb(r,g,b)" for display (#61)
 * - String "#RRGGBB" → "#RRGGBB" (passthrough)
 * - null/undefined → fallback color
 *
 * @param {Array|string|null} color - Color in various formats
 * @param {string} fallback - Fallback color (default: 'var(--gray-400)')
 * @returns {string} CSS color value
 */
export function getRgbColor(color, fallback = 'var(--gray-400)') {
  if (!color) return fallback
  if (Array.isArray(color)) return boostTeamColor(color) || `rgb(${color.join(',')})`
  return color
}

/**
 * Convert RGB array to hex string
 * @param {Array} rgb - [r, g, b] array
 * @returns {string} Hex color string "#RRGGBB"
 */
export function rgbToHex(rgb) {
  if (!rgb || !Array.isArray(rgb)) return '#6366f1' // Default indigo
  return `#${rgb.map(c => c.toString(16).padStart(2, '0')).join('')}`
}

/**
 * Get contrasting text color (black or white) for a background
 * @param {Array} rgb - [r, g, b] array
 * @returns {string} "#000000" or "#ffffff"
 */
export function getContrastColor(rgb) {
  if (!rgb || !Array.isArray(rgb)) return '#ffffff'
  // Calculate relative luminance
  const luminance = (0.299 * rgb[0] + 0.587 * rgb[1] + 0.114 * rgb[2]) / 255
  return luminance > 0.5 ? '#000000' : '#ffffff'
}
