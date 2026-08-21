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
 * #184/B-F2 — reçoit `playable`/`revealed` (contexte d'hôte normalisé,
 * `utils/hostContext.js`) au lieu de `subphase` : ce composant ne lit plus
 * `MEMOTION_SUBPHASE` directement. Il fait confiance à son appelant
 * (`AnimConductPanel`, qui ne le monte jamais pour `GRID`/`MEMORIZE` — ces
 * deux sous-phases restent affichées par `AnimMotionGrid`) plutôt que de
 * revalider lui-même la sous-phase — même philosophie que `AnimQcmOptions`/
 * `AnimAnswerZone`, qui ne se protègent pas non plus contre un hôte
 * incohérent. Correspondance : `revealed` ⇒ face REVEAL, sinon `playable` ⇒
 * face QUESTION, sinon (ni l'un ni l'autre — seul cas restant en pratique :
 * SELECTED) ⇒ face RECTO/SELECTED.
 *
 * @param {Object} props
 * @param {boolean} [props.playable] - hostContext.playable (ex `subphase === 'QUESTION'`)
 * @param {boolean} [props.revealed] - hostContext.revealed (ex `subphase === 'REVEAL'`)
 * @param {Object|null} props.card - carte MEMOTION sélectionnée
 *   (`question.MOTION_CARDS.find(c => c.ID === MEMOTION_SELECTED)`)
 * @param {{POINTS_1_STAR?: number, POINTS_2_STAR?: number, POINTS_3_STAR?: number}|null} [props.motionConfig] - question.MOTION_CONFIG
 */
export default function AnimMotionCard({ playable, revealed, card, motionConfig }) {
  if (!card) return null

  const diff = card.DIFFICULTY || 1
  const points = getMotionCardPoints(diff, motionConfig)

  if (!revealed && !playable) {
    // SELECTED — aucun hôte de carte actif, mais le seul cas qui atteint
    // réellement ce composant (AnimConductPanel ne le monte jamais pour
    // GRID/MEMORIZE) : la carte vient d'être choisie, face de grille.
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

  if (playable) {
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
