export const CATEGORIES = {
  GEOGRAPHY:     { label: 'Geographie',       icon: '🌍', color: '#3b82f6' },
  ENTERTAINMENT: { label: 'Divertissement',   icon: '🎭', color: '#ec4899' },
  HISTORY:       { label: 'Histoire',         icon: '📜', color: '#eab308' },
  ARTS:          { label: 'Arts & Litterature', icon: '🎨', color: '#a855f7' },
  SCIENCE:       { label: 'Sciences & Nature', icon: '🔬', color: '#22c55e' },
  SPORTS:        { label: 'Sports & Loisirs', icon: '⚽', color: '#f97316' },
  FOOD:          { label: 'Gastronomie',      icon: '🍽️', color: '#991b1b' },
  ANIMALS:       { label: 'Animaux',          icon: '🐾', color: '#78716c' },
}

/**
 * Returns display metadata for a category key.
 * Hardcoded categories take priority over custom ones.
 *
 * @param {string} key
 * @param {Array}  customCategories - from GET /api/categories
 * @returns {{ label, icon, color, isCustom, imageURL } | null}
 */
export function categoryMeta(key, customCategories = []) {
  if (CATEGORIES[key]) {
    return { ...CATEGORIES[key], isCustom: false, imageURL: null }
  }
  const custom = customCategories.find(c => c.key === key)
  if (custom) {
    return { label: custom.name, icon: null, color: '#6b7280', isCustom: true, imageURL: custom.imageURL }
  }
  return null
}
