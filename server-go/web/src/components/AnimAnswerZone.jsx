import { QCM_COLORS } from '../constants/colors'
import useHoldToPeek from '../hooks/useHoldToPeek'
import './AnimAnswerZone.css'

/**
 * AnimAnswerZone — zone réponse permanente de `/anim` (#166/F10, #169).
 *
 * Remplace le bloc conditionnel #163/F4 (ANSWER affiché uniquement en
 * REVEALED) et unifie l'affichage de la réponse pour tous les modes :
 * hors QCM → `question.ANSWER` ; QCM → pastille de couleur (`QCM_CORRECT`
 * via `QCM_COLORS`, même mapping que `AnimQcmOptions`/zone équipes — jamais
 * de table dupliquée) + libellé (`QCM_ANSWERS[QCM_CORRECT]`).
 *
 * Toujours rendue dès qu'une question est chargée. **Dimensions et
 * position identiques dans tous les états** (masqué / révélé par pression
 * / révélé par phase) — seuls `filter: blur()`, l'opacité et le style de
 * bordure changent (AnimAnswerZone.css), pour qu'aucune transition ne
 * décale rien à l'écran (risque R5 du plan #166).
 *
 * #169 — révélation par pression tactile (remplace le flou passif permanent,
 * qui ne protégeait rien de réel) :
 *   - Avant `REVEALED` : masquée par défaut, révélée tant qu'un pointeur
 *     (doigt ou souris — Pointer Events, pas d'événements souris/touch
 *     séparés) est MAINTENU sur la zone ; relâché ou sorti de la zone ->
 *     remasquée. État interne `peeking`, réinitialisé au changement de
 *     question (garde-fou si un `pointerup` est manqué, ex. changement de
 *     question pendant une pression).
 *   - En `REVEALED` : reste visible en PERMANENCE, sans interaction —
 *     l'animateur ne doit pas garder le doigt appuyé pendant qu'il crédite
 *     les équipes. Les handlers sont des no-op dans cet état.
 *
 * Précautions d'usage — flou franc, `user-select: none` sur le texte
 * masqué : ce N'EST PAS un mécanisme de confidentialité. La réponse
 * transite déjà sur `/ws/anim` dès le chargement de la question (constat
 * #163, inchangé) ; le flou/la pression évitent la lecture involontaire
 * par l'animateur, ils ne résistent ni à un regard appuyé ni aux outils de
 * développement. Voir plan §"Le flou n'est pas un masque" — aucune
 * prétention contraire ici.
 *
 * La grille QCM (AnimConductPanel, L2) garde SON PROPRE marquage de bonne
 * réponse en REVEALED (#163/F3, liseré vert + ✓) : les deux lectures se
 * confirment, cette zone ne le remplace pas.
 *
 * #168/F6 — le geste de révélation par pression (état `peeking`, les 4
 * handlers pointeur, la réinitialisation sur changement de question) est
 * extrait dans `useHoldToPeek` (hooks/useHoldToPeek.js), partagé avec
 * `AnimExplanationNote.jsx` (note d'explication, #168) — même geste EXACT,
 * pas une seconde implémentation. Comportement inchangé par cette
 * extraction (garde-fou : `AnimAnswerZone.test.jsx` reste vert sans
 * modification).
 *
 * @param {Object} props
 * @param {Object|null} [props.question] - gameState.question ; zone absente si null
 * @param {boolean} [props.revealed] - phase === 'REVEALED' (phaseRules.isRevealed)
 */
export default function AnimAnswerZone({ question, revealed }) {
  const { visible, handlers } = useHoldToPeek(revealed, question?.ID)

  if (!question) return null

  const isQcm = question.TYPE === 'QCM'
  const colorInfo = isQcm ? QCM_COLORS[question.QCM_CORRECT] : null
  const value = isQcm
    ? (question.QCM_ANSWERS?.[question.QCM_CORRECT] || '—')
    : (question.ANSWER || '—')

  return (
    <div
      className={`anim-answer-zone ${visible ? 'revealed' : 'masked'} ${!revealed ? 'anim-answer-zone-peekable' : ''}`}
      {...handlers}
    >
      <span className="anim-answer-zone-label">Réponse</span>
      {isQcm && colorInfo && (
        <span className="anim-answer-zone-badge" style={{ backgroundColor: colorInfo.color }}>
          {colorInfo.letter}
        </span>
      )}
      <span className="anim-answer-zone-value">{value}</span>
    </div>
  )
}
