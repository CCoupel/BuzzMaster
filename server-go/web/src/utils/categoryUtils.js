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

/** Couleur d'affichage quand une catégorie n'en a aucune (`color: ""` côté API). */
export const NO_CATEGORY_COLOR = '#6b7280'

// Contournement du cache navigateur après remplacement d'une icône (#285) :
// `?v=<TS>` (TS serveur de CATEGORIES_CHANGED) ajouté à l'imageURL de la clé concernée.
const iconBust = {}
export function bustCategoryIcon(key, ts = Date.now()) {
  iconBust[key] = ts
}

/**
 * Returns display metadata for a category key — API-first (#285).
 * Nom, couleur et image viennent de GET /api/categories ; la table JS ne fournit
 * que l'emoji des intégrées et un repli hors ligne. `color: ""` (= aucune) ⇒ gris.
 *
 * @param {string} key
 * @param {Array}  categories - from GET /api/categories (liste brute acceptée)
 * @returns {{ label, icon, color, isCustom, imageURL } | null}
 */
export function categoryMeta(key, categories = []) {
  const base = CATEGORIES[key]
  const api = categories.find(c => c.key === key)
  if (!base && !api) return null
  if (base) {
    // Intégrée : jamais d'image (emoji), nom/couleur surchargés par l'API si présents.
    return {
      label: api ? api.name : base.label,
      icon: base.icon,
      color: api ? (api.color || NO_CATEGORY_COLOR) : base.color,
      isCustom: false,
      imageURL: null,
    }
  }
  const bust = iconBust[key]
  const imageURL = api.imageURL && bust ? `${api.imageURL}?v=${bust}` : api.imageURL
  return { label: api.name, icon: null, color: api.color || NO_CATEGORY_COLOR, isCustom: true, imageURL }
}

/**
 * Niveaux de difficulté (étoiles) du réservoir RAFALE et des cartes MEMOTION.
 * Source unique (#221) — remplace RAFALE_DIFFICULTIES / DIFFICULTIES locales
 * et les `[1, 2, 3]` inline des pages.
 */
export const DIFFICULTY_LEVELS = [1, 2, 3]

/**
 * Palette de 12 teintes proposée pour la couleur d'une catégorie (#285, maquette
 * admin_category_manager §01/§02). La couleur pilote badges, néon TV **et** ambiance
 * (contracts/lighting.md §8) ; le serveur n'impose pas la palette, seule l'interface la borne.
 */
export const CATEGORY_PALETTE = [
  '#ef4444', '#f97316', '#eab308', '#22c55e', '#14b8a6', '#0ea5e9',
  '#3b82f6', '#6366f1', '#a855f7', '#ec4899', '#f43f5e', '#a3e635',
]

/** Première teinte de la palette non utilisée par une catégorie existante (défaut de création). */
export function firstFreePaletteColor(categories = []) {
  const used = new Set(categories.map(c => (c.color || '').toLowerCase()))
  return CATEGORY_PALETTE.find(c => !used.has(c)) ?? CATEGORY_PALETTE[0]
}
