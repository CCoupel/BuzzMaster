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
 * #185/C-F1 — auto-garde relâchée : ce composant ne reçoit plus de prop
 * `type` et ne se garde plus lui-même sur une valeur de type. Il est
 * maintenant monté dans DEUX hôtes (question standalone `TYPE=QCM`, et
 * carte MEMOTION active `TYPE=QCM` — `AnimConductPanel`/`AnimMotionCard`,
 * #184/B-F3) : dupliquer un contrôle `type === 'QCM'` ici obligerait chaque
 * host à connaître et transmettre correctement cette valeur, alors que le
 * dispatch par type est désormais **entièrement** la responsabilité de
 * l'appelant (host-aware depuis #184). La garde utile ici est `!answers` :
 * `QCM_ANSWERS` n'est jamais peuplé pour un autre type, question ou carte
 * (contrat `question-types.md` §2/§7) — suffisant pour rester correct monté
 * isolément (tests), sans dupliquer une décision de dispatch qui ne lui
 * appartient pas.
 *
 * @param {Object} props
 * @param {Object} [props.answers] - question.QCM_ANSWERS ou selectedMotionCard.QCM_ANSWERS ({RED,GREEN,YELLOW,BLUE})
 * @param {string} [props.correct] - question.QCM_CORRECT ou selectedMotionCard.QCM_CORRECT (clé couleur)
 * @param {string[]} [props.invalidated] - gameState.qcmInvalidated (hôte question) ou
 *   getTypeState(gameState, hostContext).qcmInvalidated (hôte carte, #184/B-F1)
 * @param {boolean} [props.revealed] - hostContext.revealed (hôte courant, #184/B-F2)
 */
export default function AnimQcmOptions({ answers, correct, invalidated = [], revealed }) {
  if (!answers) return null

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
