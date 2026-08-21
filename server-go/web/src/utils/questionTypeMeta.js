/**
 * questionTypeMeta — table unique des types de question (#183, A-F2).
 *
 * Source unique et faisant autorité pour tout le front — remplace les trois
 * tables locales qui divergaient silencieusement (`QuestionCard.jsx`
 * `TYPE_LABELS`, `AIGenerateModal.jsx` `TYPES`, boutons codés en dur de
 * `QuestionsPage.jsx`). Toute nouvelle propriété d'affichage partagée entre
 * plusieurs composants (icône, libellé, couleur…) se déclare ici, jamais en
 * copie locale — voir `contracts/question-types.md` §10 (test d'exhaustivité).
 *
 * Icônes validées au GATE 2 (issue #166) : ⚡ SPEEDY, 🔠 QCM, 🖊️ ARDOISE,
 * 🃏 MEMORY, 🎞️ MEMOTION. Couleurs reprises de `AIGenerateModal.jsx`
 * (ARDOISE : cohérente avec `.type-btn.ardoise` de `QuestionsPage.css`,
 * #10b981).
 *
 * Ordre = ordre d'affichage (grille éditeur `QuestionsPage.jsx` : ligne 1
 * SPEEDY/QCM/MEMORY, ligne 2 MEMOTION/ARDOISE ; même ordre que le sélecteur
 * `AIGenerateModal.jsx`).
 *
 * `nestable` (#184/B-F4) — miroir JS de `TypeDescriptor.NestableInMotionCard`
 * (registre Go, `internal/game/question_types.go`, contrat §7) : les seuls
 * types qu'une carte MEMOTION peut porter en `TYPE`. SPEEDY et QCM en v7.0.0 ;
 * ARDOISE/MEMORY rejoignent en v7.1.0 (#186/#187) ; MEMOTION jamais
 * (profondeur d'imbrication plafonnée à 1, contrat §1). Consommé par le
 * sélecteur de type de carte (`QuestionsPage.jsx`) pour filtrer les boutons
 * proposés — pas de registre séparé côté JS, cette table reste la source
 * unique.
 */

export const QUESTION_TYPES = [
  { key: 'SPEEDY', label: 'Speedy', icon: '⚡', color: '#3b7fc4', nestable: true },
  { key: 'QCM', label: 'QCM', icon: '🔠', color: '#8a5cc4', nestable: true },
  { key: 'MEMORY', label: 'Memory', icon: '🃏', color: '#2e9e6d', nestable: false },
  { key: 'MEMOTION', label: 'Memotion', icon: '🎞️', color: '#c8568f', nestable: false },
  { key: 'ARDOISE', label: 'Ardoise', icon: '🖊️', color: '#10b981', nestable: false },
]

export const QUESTION_TYPE_META = QUESTION_TYPES.reduce((acc, t) => {
  acc[t.key] = t
  return acc
}, {})

// Repli explicite pour un type réellement inconnu (chaîne non vide, absente
// du registre ci-dessus) — distinct de SPEEDY à dessein : voir
// `getQuestionTypeMeta` ci-dessous.
const UNKNOWN_TYPE_META = { key: null, label: 'Type inconnu', icon: '❓', color: '#6b7280' }

/**
 * @param {string} [type] - question.TYPE (ou nextQuestion.TYPE, ou MotionCard.TYPE)
 * @returns {{key: string|null, icon: string, label: string, color: string}}
 *
 * - `type` absent/vide ⇒ repli **SPEEDY**, comportement documenté et
 *   volontaire (`contracts/question-types.md` §3 : une carte/question sans
 *   `TYPE` vaut `SPEEDY`).
 * - `type` renseigné mais absent du registre ⇒ **ne retombe plus
 *   silencieusement sur SPEEDY** (#183) : un signalement explicite est loggé
 *   et un objet distinct (`UNKNOWN_TYPE_META`) est renvoyé, détectable par
 *   test — voir `questionTypeMeta.test.js`.
 */
export function getQuestionTypeMeta(type) {
  if (!type) return QUESTION_TYPE_META.SPEEDY
  const meta = QUESTION_TYPE_META[type]
  if (meta) return meta
  console.error(`[questionTypeMeta] Type de question inconnu : "${type}" — aucune entrée dans le registre, pas de repli silencieux sur SPEEDY (#183)`)
  return UNKNOWN_TYPE_META
}
