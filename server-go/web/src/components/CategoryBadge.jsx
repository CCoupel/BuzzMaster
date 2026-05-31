import { categoryMeta } from '../utils/categoryUtils'
import './CategoryBadge.css'

const IMG_SIZES = { sm: 16, md: 20, lg: 30 }

/**
 * CategoryBadge — source de vérité unique pour l'affichage d'une catégorie.
 *
 * @param {string}  catKey          - Clé de la catégorie
 * @param {Array}   customCategories - Catégories custom depuis GET /api/categories
 * @param {'sm'|'md'|'lg'} size     - Taille (sm=16px, md=20px, lg=30px)
 * @param {boolean} chip            - Si true, rend un chip coloré (fond + border-radius).
 *                                    Si false, rend uniquement l'icône/image brute.
 */
export default function CategoryBadge({ catKey, customCategories = [], size = 'sm', chip = true }) {
  const meta = categoryMeta(catKey, customCategories)
  if (!meta) return null

  const imgPx = IMG_SIZES[size] ?? 16

  const inner = meta.imageURL
    ? <img src={meta.imageURL} alt={meta.label} className="category-badge__img" style={{ width: imgPx, height: imgPx }} />
    : <span className="category-badge__icon">{meta.icon}</span>

  if (!chip) return inner

  return (
    <span
      className={`category-badge category-badge--${size}`}
      style={{ backgroundColor: meta.color }}
      title={meta.label}
    >
      {inner}
    </span>
  )
}
