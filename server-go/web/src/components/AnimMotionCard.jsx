import { getMotionCardPoints } from '../utils/motionGrid'
import './AnimMotionCard.css'

/**
 * AnimMotionCard — carte MEMOTION au premier plan de `/anim` (#160/F5),
 * montée en L3 à la place de `AnimMotionGrid` pendant `SELECTED` /
 * `QUESTION` / `REVEAL`.
 *
 * Trois faces, une seule à la fois (pas de composant par face — un seul
 * rendu conditionnel sur `subphase`, comme `AnimMotionActions`/F6) :
 *   - `SELECTED` → thème + image recto + étoiles + points
 *   - `QUESTION` → texte de la question + image question
 *   - `REVEAL`   → rappel du texte de question (atténué) + image réponse +
 *                  texte réponse (en évidence)
 *
 * `motionConfig` (`question.MOTION_CONFIG`) est transmis TEL QUEL — le
 * composant dérive lui-même les points via `utils/motionGrid.js`
 * (`getMotionCardPoints`), jamais un montant précalculé par l'appelant :
 * même philosophie que #160/F0, la formule de barème ne doit exister qu'à
 * un seul endroit.
 *
 * **Pas d'animation de flip** : le spectacle (rotation 3D, clip-path) est
 * sur `/tv` (`PlayerDisplay.jsx`) — la tablette est un outil de conduite,
 * pas un second écran de jeu. Changement de face = changement de contenu,
 * point.
 *
 * @param {Object} props
 * @param {string} props.subphase - gameState.MEMOTION_SUBPHASE ('SELECTED'|'QUESTION'|'REVEAL')
 * @param {Object|null} props.card - carte MEMOTION sélectionnée
 *   (`question.MOTION_CARDS.find(c => c.ID === MEMOTION_SELECTED)`)
 * @param {{POINTS_1_STAR?: number, POINTS_2_STAR?: number, POINTS_3_STAR?: number}|null} [props.motionConfig] - question.MOTION_CONFIG
 */
export default function AnimMotionCard({ subphase, card, motionConfig }) {
  if (!card) return null
  if (!['SELECTED', 'QUESTION', 'REVEAL'].includes(subphase)) return null

  const diff = card.DIFFICULTY || 1
  const points = getMotionCardPoints(diff, motionConfig)

  if (subphase === 'SELECTED') {
    return (
      <div className="anim-motion-card-focus anim-motion-card-focus-recto">
        <span className="anim-motion-card-focus-theme">{card.RECTO_THEME}</span>
        {card.RECTO_IMAGE && (
          <img src={card.RECTO_IMAGE} alt="" className="anim-motion-card-focus-img" />
        )}
        <span className="anim-motion-card-focus-foot">
          <span className="anim-motion-card-focus-stars">{'★'.repeat(diff)}</span>
          {' · '}
          <span className="anim-motion-card-focus-points">{points} pt{points > 1 ? 's' : ''}</span>
        </span>
      </div>
    )
  }

  if (subphase === 'QUESTION') {
    return (
      <div className="anim-motion-card-focus anim-motion-card-focus-verso">
        {card.QUESTION_TEXT && (
          <p className="anim-motion-card-focus-theme">{card.QUESTION_TEXT}</p>
        )}
        {card.QUESTION_IMAGE && (
          <img src={card.QUESTION_IMAGE} alt="" className="anim-motion-card-focus-img" />
        )}
      </div>
    )
  }

  // REVEAL
  return (
    <div className="anim-motion-card-focus anim-motion-card-focus-verso">
      {card.QUESTION_TEXT && (
        <p className="anim-motion-card-focus-question">{card.QUESTION_TEXT}</p>
      )}
      {card.ANSWER_IMAGE && (
        <img src={card.ANSWER_IMAGE} alt="" className="anim-motion-card-focus-img" />
      )}
      {card.ANSWER_TEXT && (
        <span className="anim-motion-card-focus-answer">{card.ANSWER_TEXT}</span>
      )}
    </div>
  )
}
