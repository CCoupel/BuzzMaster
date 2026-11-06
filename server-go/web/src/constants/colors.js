/**
 * Shared color constants for BuzzControl
 */

import { categoryMeta } from '../utils/categoryUtils'

// QCM answer colors - used for multiple choice questions
export const QCM_COLORS = {
  RED: { label: 'Rouge', color: '#ef4444', letter: 'A' },
  GREEN: { label: 'Vert', color: '#22c55e', letter: 'B' },
  YELLOW: { label: 'Jaune', color: '#eab308', letter: 'C' },
  BLUE: { label: 'Bleu', color: '#3b82f6', letter: 'D' },
}

// Mapping from button press (A, B, C, D) to QCM color
export const BUTTON_TO_QCM_COLOR = {
  'A': 'RED',
  'B': 'GREEN',
  'C': 'YELLOW',
  'D': 'BLUE',
}

// Answer colors for unassigned players (TeamsPage)
export const ANSWER_COLORS = {
  RED: { label: 'Rouge', color: '#ef4444', letter: 'A' },
  GREEN: { label: 'Vert', color: '#22c55e', letter: 'B' },
  YELLOW: { label: 'Jaune', color: '#eab308', letter: 'C' },
  BLUE: { label: 'Bleu', color: '#3b82f6', letter: 'D' },
}

// Default color for neon effect when no category
export const DEFAULT_NEON_COLOR = '#6b7280';

/**
 * Get the color for a category (for neon effect) — API-first (#285).
 * @param {string} category - Category key (e.g., 'GEOGRAPHY', 'CINEMA')
 * @param {Array} [categories] - GET /api/categories ; sans liste, repli sur la table
 * @returns {string} Hex color code
 */
export const getCategoryColor = (category, categories = []) => {
  if (!category) return DEFAULT_NEON_COLOR;
  const meta = categoryMeta(category, categories);
  return meta ? meta.color : DEFAULT_NEON_COLOR;
}

// ---------------------------------------------------------------------------
// Team color palette (#113) — 16 colors = 8 hues × 2 tones (vif / profond).
//
// Normative source: contracts/models.md § "Palette d'équipes". The backend
// mirrors this exact table in server-go/cmd/server/main.go (`teamColorPalette`)
// — the two MUST stay value-for-value identical, or the physical buzzer LED
// (resolved server-side from COLOR_NAME) will diverge from what's on screen.
//
// Order = attribution rank (index 0 = rank 1): the 8 vivid tones first, then
// the 8 deep tones. Each RGB is built at S=100%, L=55% (vif) / L=35%
// (profond) — exactly the bounds of the widened boostTeamColor() (#113) — so
// these values pass through the display boost unchanged (invariance
// property: stored RGB === displayed RGB).
// ---------------------------------------------------------------------------
export const TEAM_COLORS = [
  { key: 'rouge', label: 'Rouge', rgb: [255, 26, 26], deep: false },
  { key: 'orange', label: 'Orange', rgb: [255, 133, 26], deep: false },
  { key: 'jaune', label: 'Jaune', rgb: [255, 217, 26], deep: false },
  { key: 'vert', label: 'Vert', rgb: [26, 255, 83], deep: false },
  { key: 'cyan', label: 'Cyan', rgb: [26, 236, 255], deep: false },
  { key: 'bleu', label: 'Bleu', rgb: [26, 94, 255], deep: false },
  { key: 'violet', label: 'Violet', rgb: [159, 26, 255], deep: false },
  { key: 'rose', label: 'Rose', rgb: [255, 26, 159], deep: false },
  { key: 'rouge-profond', label: 'Grenat', rgb: [179, 0, 0], deep: true },
  { key: 'orange-profond', label: 'Ambre', rgb: [179, 83, 0], deep: true },
  { key: 'jaune-profond', label: 'Or', rgb: [179, 149, 0], deep: true },
  { key: 'vert-profond', label: 'Émeraude', rgb: [0, 179, 45], deep: true },
  { key: 'cyan-profond', label: 'Turquoise', rgb: [0, 164, 179], deep: true },
  { key: 'bleu-profond', label: 'Marine', rgb: [0, 54, 179], deep: true },
  { key: 'violet-profond', label: 'Indigo', rgb: [104, 0, 179], deep: true },
  { key: 'rose-profond', label: 'Magenta', rgb: [179, 0, 104], deep: true },
]

/**
 * Pick the next color to attribute to a newly created team.
 * Returns the first palette entry whose key isn't already carried by an
 * existing team (COLOR_NAME). If all 16 are taken, recycles from rank 1.
 *
 * @param {Object} teams - Current teams map (name -> {COLOR_NAME, ...})
 * @returns {{key: string, label: string, rgb: number[], deep: boolean}}
 */
export function getNextTeamColor(teams = {}) {
  const usedKeys = new Set(
    Object.values(teams || {})
      .map(t => t?.COLOR_NAME)
      .filter(Boolean)
  )
  return TEAM_COLORS.find(c => !usedKeys.has(c.key)) || TEAM_COLORS[0]
}

/**
 * Resolve a palette entry from a team's stored color data.
 * Primary resolution by COLOR_NAME key; falls back to an exact RGB match
 * for teams created before the feature (no COLOR_NAME stored).
 *
 * @param {string} [colorName] - Team.COLOR_NAME
 * @param {number[]} [rgb] - Team.COLOR ([r, g, b])
 * @returns {{key: string, label: string, rgb: number[], deep: boolean}|null}
 */
export function findTeamColor(colorName, rgb) {
  if (colorName) {
    const byKey = TEAM_COLORS.find(c => c.key === colorName)
    if (byKey) return byKey
  }
  if (Array.isArray(rgb) && rgb.length >= 3) {
    return TEAM_COLORS.find(c => c.rgb[0] === rgb[0] && c.rgb[1] === rgb[1] && c.rgb[2] === rgb[2]) || null
  }
  return null
}
