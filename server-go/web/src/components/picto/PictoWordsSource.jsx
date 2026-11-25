/**
 * PictoWordsSource — sous-éditeur « Mots à dessiner » PARTAGÉ par les types Picto et Rafale Picto (v13.1.0, #324,
 * contrat picto.md §17.2, maquette M3) : source des mots (Pool Picto ou mots saisis), filtre catégories ×
 * difficultés du pool, alerte de disponibilité. Les mots saisis eux-mêmes sont le `children` (éditeur de liste du type).
 *
 * Paramétré par `keys` (noms des champs du formulaire) : `PICTO_KEYS` ici ; le type Rafale Picto (LOT-3) passera
 * les siens (`RAFALE_PICTO_*`). Composant contrôlé : l'état vit dans le formulaire de QuestionsPage.
 */
import { useEffect, useMemo } from 'react'
import CategoryPicker from '../CategoryPicker'
import DifficultyPicker from '../DifficultyPicker'
import PoolAlert from '../PoolAlert'

export const PICTO_KEYS = { source: 'pictoSource', categories: 'pictoCategories', difficulties: 'pictoDifficulties' }

export const SOURCE_POOL = 'RESERVOIR'
export const SOURCE_WORDS = 'WORDS'

/** Filtre du pool valide : au moins une catégorie et une difficulté de 1 à 3 (contrat §17.2). */
export function isPictoFilterValid(categories, difficulties) {
  return Array.isArray(categories) && categories.length > 0
    && Array.isArray(difficulties) && difficulties.length > 0 && difficulties.every(d => d >= 1 && d <= 3)
}

/**
 * @param {object} props
 * @param {object} props.values - formulaire (`category` = catégorie de la question, repli du filtre)
 * @param {(field: string, value: any) => void} props.onChange
 * @param {object} [props.keys] - noms des champs (défaut `PICTO_KEYS`)
 * @param {Array} [props.customCategories] - GET /api/categories
 * @param {number} [props.roundTime] - durée à couvrir (Rafale Picto : TIME) ; défaut 1 (un mot suffit)
 * @param {number} [props.questionTime] - secondes par mot pour estimer le besoin (Rafale Picto : 10) ; défaut 1
 * @param {React.ReactNode} props.children - éditeur des mots saisis (affiché quand la source est « WORDS »)
 */
export default function PictoWordsSource({
  values, onChange, keys = PICTO_KEYS, customCategories = [], roundTime = 1, questionTime = 1, children,
}) {
  const source = values[keys.source] === SOURCE_POOL ? SOURCE_POOL : SOURCE_WORDS
  const categories = useMemo(() => values[keys.categories] || [], [values, keys.categories])
  const difficulties = useMemo(() => values[keys.difficulties] || [], [values, keys.difficulties])

  // Défaut du contrat §17.2 : filtre = catégorie de la question tant qu'aucune catégorie n'est choisie.
  useEffect(() => {
    if (source === SOURCE_POOL && categories.length === 0 && values.category) {
      onChange(keys.categories, [values.category])
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [source, values.category])

  const toggle = (field, current, v) => {
    const next = current.includes(v) ? current.filter(x => x !== v) : [...current, v]
    onChange(field, field === keys.difficulties ? next.sort((a, b) => a - b) : next)
  }

  return (
    <div className="picto-source" data-testid="picto-words-source">
      <div className="picto-source-toggle" role="group" aria-label="Source des mots">
        <button type="button" className={`picto-source-btn ${source === SOURCE_POOL ? 'on' : ''}`} aria-pressed={source === SOURCE_POOL}
          onClick={() => onChange(keys.source, SOURCE_POOL)}>🗂 Pool Picto</button>
        <button type="button" className={`picto-source-btn ${source === SOURCE_WORDS ? 'on' : ''}`} aria-pressed={source === SOURCE_WORDS}
          onClick={() => onChange(keys.source, SOURCE_WORDS)}>✍️ Saisis dans la question</button>
      </div>

      {source === SOURCE_POOL ? (
        <div className="picto-source-pool">
          <CategoryPicker
            mode="multi"
            label="Catégories du pool"
            value={new Set(categories)}
            onToggle={(k) => toggle(keys.categories, categories, k)}
            customCategories={customCategories}
          />
          <DifficultyPicker
            mode="multi"
            ghost
            value={new Set(difficulties)}
            onToggle={(d) => toggle(keys.difficulties, difficulties, d)}
          />
          <PoolAlert instance="picto" categories={categories} difficulties={difficulties} roundTime={roundTime} questionTime={questionTime} />
          <p className="picto-hint">
            Pioche stratifiée par catégorie × difficulté, comme le Pool Speedy. Chaque mot tiré est marqué « utilisé » et
            ne ressort plus avant une remise à disposition (onglet ✏️ Pool Picto ou nouvelle partie).
          </p>
        </div>
      ) : children}
    </div>
  )
}
