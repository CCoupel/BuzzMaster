/**
 * Color utility functions for BuzzControl
 */

/**
 * Convert color to RGB string
 * Handles multiple input formats:
 * - Array [r, g, b] → "rgb(r,g,b)"
 * - String "#RRGGBB" → "#RRGGBB" (passthrough)
 * - null/undefined → fallback color
 *
 * @param {Array|string|null} color - Color in various formats
 * @param {string} fallback - Fallback color (default: 'var(--gray-400)')
 * @returns {string} CSS color value
 */
export function getRgbColor(color, fallback = 'var(--gray-400)') {
  if (!color) return fallback
  if (Array.isArray(color)) return `rgb(${color.join(',')})`
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
