import useHoldToPeek from '../hooks/useHoldToPeek'
import './AnimExplanationNote.css'

/**
 * AnimExplanationNote — note d'explication d'une question (`/anim`, L4 de
 * `AnimConductPanel`, #168).
 *
 * Occupe la ligne L4 réservée depuis #166/F11. Suit EXACTEMENT le même
 * geste que la zone réponse (`AnimAnswerZone`, #169) — mutualisé via
 * `useHoldToPeek` (#168/F6) plutôt que réécrit ici : avant `REVEALED`, la
 * note est masquée (floutée) et révélée tant qu'un pointeur est maintenu ;
 * en `REVEALED`, elle reste visible en permanence, sans interaction.
 *
 * Une question sans note affiche l'emplacement au repos (jamais un blanc,
 * même traitement que les replis `.anim-conduct-reserved` de L2/L3) — la
 * ligne L4 est TOUJOURS rendue, `question` puisse être `null` ou sans
 * `EXPLANATION`.
 *
 * Comme `AnimAnswerZone`, ceci n'est PAS un mécanisme de confidentialité :
 * `EXPLANATION` transite déjà en clair sur `/ws/anim` dès le chargement de
 * la question (contrats/models.md §EXPLANATION). Le flou évite seulement la
 * lecture involontaire.
 *
 * @param {Object} props
 * @param {Object|null} [props.question] - gameState.question
 * @param {boolean} [props.revealed] - phase === 'REVEALED' (phaseRules.isRevealed)
 */
export default function AnimExplanationNote({ question, revealed }) {
  const explanation = question?.EXPLANATION || ''
  const hasNote = explanation.trim().length > 0
  const { visible, handlers } = useHoldToPeek(revealed, question?.ID)

  if (!hasNote) {
    return (
      <div className="anim-explanation-note empty">Aucune note pour cette question</div>
    )
  }

  return (
    <div
      className={`anim-explanation-note ${visible ? 'shown' : 'masked'} ${!revealed ? 'anim-explanation-note-peekable' : ''}`}
      {...handlers}
    >
      <span className="anim-explanation-note-label">
        {visible ? 'Note' : 'Note — maintenir pour lire'}
      </span>
      <span className="anim-explanation-note-body">{explanation}</span>
    </div>
  )
}
