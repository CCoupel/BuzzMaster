import PoolPage from './PoolPage'

// Plafonds de longueur partagés Go ↔ JS (`rafaleMaxQuestionRunes` / `rafaleMaxAnswerRunes`,
// ai_generator_rafale.go) : le test d'alignement rafale_go_js_alignment_203_test.go les lit ICI. Valeurs identiques
// à POOL_INSTANCES.rafale (utils/poolInstances.js, vérifié par poolInstances.324.test.js).
export const RAFALE_MAX_QUESTION_RUNES = 100
export const RAFALE_MAX_ANSWER_RUNES = 40

/**
 * RafalePage — « Pool Speedy » (`/admin/rafale`). Enveloppe mince de `PoolPage` sur l'instance `rafale` (v13.1.0,
 * contrat picto-pool.md §6) : écran, libellés et chemins inchangés. Le contenu historique (contrat rafale.md §9,
 * #197, #203) vit désormais dans PoolPage.jsx.
 */
export default function RafalePage() {
  return <PoolPage instance="rafale" />
}
