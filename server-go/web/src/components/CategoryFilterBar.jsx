import { categoryMeta } from '../utils/categoryUtils'
import CategoryBadge from './CategoryBadge'

// Pas de CategoryFilterBar.css dédié : `.category-filter-bar`/
// `.category-filter-pill`/`.category-filter-reset` (base) vivent dans
// GamePage.css, `.questions-page-filter-bar .category-filter-pill`/
// `.cat-pill-label` (modificateur Quiz repris ici) dans QuestionsPage.css —
// les deux fichiers restent chargés globalement tant que GamePage.jsx et
// QuestionsPage.jsx existent, même discipline que AnimRafaleActions.jsx
// vis-à-vis d'AnimConductPanel.css. Pas de règle dupliquée ici.

/**
 * CategoryFilterBar — barre de filtre par catégorie, extraite de
 * QuestionsPage.jsx (#40) pour être partagée avec RafalePage.jsx (v8.0.0,
 * #16/#197, bugfix : "le pool RAFALE liste 2 lignes identiques de
 * catégorie... l'affichage des catégories devrait être mutualisé entre les
 * questions Quiz et RAFALE").
 *
 * RafalePage.jsx portait sa propre variante (`.rafale-chip`), divergente
 * sur deux points par rapport à celle-ci (source du rendu incohérent
 * signalé) :
 *   1. Pas de garde `if (!meta) return null` — une clé sans meta connue
 *      (categoryMeta renvoie null) rendait quand même un bouton, avec une
 *      icône vide (CategoryBadge renvoie null en interne) à côté du texte
 *      brut de la clé — visuellement un doublon de la ligne portant la
 *      même catégorie déjà rendue correctement par ailleurs.
 *   2. `customCategories` passé NON FILTRÉ (`apiCategories` brut, incluant
 *      les 8 catégories codées en dur en miroir, `isCustom:false`) au lieu
 *      de la liste déjà filtrée `isCustom:true` que QuestionsPage.jsx
 *      calcule avant de l'utiliser — `categoryMeta`/`CategoryBadge` s'en
 *      sortent (la clé codée en dur est prioritaire dans `categoryMeta`),
 *      mais c'est une source de données différente de celle du Quiz,
 *      exactement ce que le retour utilisateur demande de mutualiser.
 *
 * Un seul composant désormais — jamais une seconde variante de la même
 * fonctionnalité.
 *
 * @param {Object} props
 * @param {string[]} props.availableCategories - useCategoryFilter().availableCategories
 * @param {Set<string>} props.selectedCategories - useCategoryFilter().selectedCategories
 * @param {Array} props.customCategories - GET /api/categories (useCategories()).
 *   Idéalement déjà filtré isCustom:true par l'appelant (même discipline que
 *   QuestionsPage.jsx/GamePage.jsx), mais peut aussi être passé BRUT — le
 *   composant filtre lui-même avant tout usage (symétrie avec
 *   CategorySelector.jsx, code-review 20260829-131404, point mineur) :
 *   `availableCategories` (déjà dédupliqué par useCategoryFilter) protège
 *   du doublon de PILLS ici, mais un `customCategories` brut resterait une
 *   source de données divergente de celle du Quiz si un futur appelant
 *   oubliait de pré-filtrer — jamais dépendre de cette discipline côté
 *   appelant.
 * @param {(key: string) => void} props.onToggle - useCategoryFilter().toggleCategoryFilter
 * @param {() => void} props.onClear - useCategoryFilter().clearCategoryFilters
 * @param {'sm'|'md'|'lg'} [props.size] - taille des icônes (CategoryBadge), défaut 'md'
 * @param {boolean} [props.showLabel] - affiche le libellé texte à côté de
 *   l'icône (comme QuestionsPage.jsx) — défaut true
 */
export default function CategoryFilterBar({
  availableCategories,
  selectedCategories,
  customCategories = [],
  onToggle,
  onClear,
  size = 'md',
  showLabel = true,
}) {
  if (availableCategories.length === 0) return null

  // Filtre défensif isCustom (symétrie avec CategorySelector.jsx) — voir la
  // doc du prop customCategories ci-dessus.
  const trueCustomCategories = customCategories.filter(c => c.isCustom)

  // `questions-page-filter-bar` — modificateur visuel du Quiz (taille des
  // pastilles + libellé, QuestionsPage.css), appliqué systématiquement ici
  // : c'est précisément le rendu "Quiz" que RAFALE doit reprendre à
  // l'identique (retour utilisateur). `.category-filter-bar` (base,
  // GamePage.css) reste la classe partagée sous-jacente.
  return (
    <div className="category-filter-bar questions-page-filter-bar">
      {availableCategories.map(catKey => {
        const meta = categoryMeta(catKey, trueCustomCategories)
        if (!meta) return null
        const isActive = selectedCategories.has(catKey)
        return (
          <button
            key={catKey}
            type="button"
            className={`category-filter-pill${isActive ? ' active' : ''}`}
            style={{ '--cat-color': meta.color }}
            onClick={() => onToggle(catKey)}
            title={meta.label}
          >
            <CategoryBadge catKey={catKey} customCategories={trueCustomCategories} size={size} chip={false} />
            {showLabel && <span className="cat-pill-label">{meta.label}</span>}
          </button>
        )
      })}
      {selectedCategories.size > 0 && (
        <button type="button" className="category-filter-reset" onClick={onClear} title="Réinitialiser les filtres">
          ×
        </button>
      )}
    </div>
  )
}
