import { QCM_COLORS } from '../constants/colors'
import './AnimQcmOptions.css'

// Ordre d'affichage fixe (grille 2x2), imposé par la spec F3 — pas l'ordre
// d'insertion d'un objet JS, qui n'est pas garanti stable entre payloads.
const ORDER = ['RED', 'GREEN', 'YELLOW', 'BLUE']

/**
 * AnimQcmOptions — grille 2x2 des propositions QCM en zone contexte de
 * `/anim` (#163/F3, plan `_work/reports/plan-20260814-101626.md`).
 *
 * Réutilise IMPÉRATIVEMENT `QCM_COLORS` (`constants/colors.js`) pour le
 * mapping couleur -> lettre : `AnimPage.jsx` l'utilise déjà pour la zone
 * équipes (#157) — une table dupliquée ici serait exactement le piège déjà
 * évité par #149/#155 (risque R2 du plan).
 *
 * Décision GATE 2 D3 : les propositions sont visibles dès que la question
 * est chargée, sans garde de phase sur les propositions elles-mêmes — seule
 * la bonne réponse (`correct`) est gardée par `revealed`
 * (`phase === 'REVEALED'`), même règle que le marqueur ✓/✗ par équipe (#157).
 *
 * Guard double (type ET answers) volontaire : le composant reste correct
 * s'il est monté isolément (tests T3), pas seulement derrière la garde de
 * l'appelant (`AnimPage.jsx`).
 *
 * @param {Object} props
 * @param {string} [props.type] - question.TYPE ; ne rend rien si != 'QCM'
 * @param {Object} [props.answers] - question.QCM_ANSWERS ({RED,GREEN,YELLOW,BLUE})
 * @param {string} [props.correct] - question.QCM_CORRECT (clé couleur)
 * @param {string[]} [props.invalidated] - gameState.qcmInvalidated
 * @param {boolean} [props.revealed] - phase === 'REVEALED'
 */
export default function AnimQcmOptions({ type, answers, correct, invalidated = [], revealed }) {
  if (type !== 'QCM' || !answers) return null

  return (
    <div className="anim-qcm-options">
      {ORDER.map((colorKey) => {
        const colorInfo = QCM_COLORS[colorKey]
        const isInvalidated = invalidated.includes(colorKey)
        const isCorrect = revealed && correct === colorKey
        return (
          <div
            key={colorKey}
            className={`anim-qcm-option ${isInvalidated ? 'invalidated' : ''} ${isCorrect ? 'correct' : ''}`}
          >
            <span className="anim-qcm-option-letter" style={{ backgroundColor: colorInfo.color }}>
              {colorInfo.letter}
            </span>
            <span className="anim-qcm-option-text">{answers[colorKey]}</span>
            {isCorrect && <span className="anim-qcm-option-mark">✓</span>}
          </div>
        )
      })}
    </div>
  )
}
