import './AnimRafaleActions.css'

// Sous-titre des boutons VALIDE/INVALIDE, par RAFALE_MODE (contrat rafale.md
// §3.4/§6.1, table des 4 modes) — revue code-reviewer (2026-08-28,
// _work/reports/code-review-20260828-183037.md) : "équipe suivante" était
// affiché tel quel quel que soit le mode, faux pour SOLO (aucune rotation,
// une seule équipe) et TANT_QUE_JE_GAGNE (une bonne réponse garde la main,
// pas de changement d'équipe). `null` = pas de sous-titre (bouton monté sans
// second enfant), même discipline que buttonSubLabel côté L1
// (AnimConductPanel.jsx) — jamais une chaîne vide affichée à la place.
const RAFALE_MODE_SUBLABELS = {
  SOLO: { valid: null, invalid: null },
  CHACUN_SON_TOUR: { valid: 'équipe suivante', invalid: 'équipe suivante' },
  TANT_QUE_JE_GAGNE: { valid: 'garde la main', invalid: 'équipe suivante' },
  MAILLON_FAIBLE: { valid: 'équipe suivante', invalid: 'compteur à 0' },
}

/**
 * AnimRafaleActions — ligne L2 (« gestes propres au mode ») pendant une
 * manche RAFALE, `/anim` (contrat rafale.md §5.1, §8.1 : seuls `admin` et
 * `anim` jugent la réponse — VPlayer/TV/buzzers restent strictement
 * passifs).
 *
 * Deux actions client→serveur, sans payload : `RAFALE_VALIDATE` /
 * `RAFALE_INVALIDATE` (contrat §5.1). Le contrat ne définit AUCUNE action
 * d'attribution de points ici — l'attribution de fin de manche réutilise
 * `TEAM_POINTS` (clic équipe), hors du périmètre de ce composant (§6.2).
 *
 * Palette `anim-conduct-btn-{go|danger}` réutilisée TELLE QUELLE
 * (AnimConductPanel.css, chargé par AnimConductPanel qui monte toujours ce
 * composant) — aucune nouvelle couleur pour RAFALE, même discipline que
 * `AnimMotionActions` (#160/F6). Seule la mise en page (grille 2 colonnes)
 * est propre à cette ligne.
 *
 * @param {Object} props
 * @param {boolean} [props.disabled] - désactive les deux boutons (ex. hors
 *   sous-phase QUESTION, ou question déjà jugée) — émet `disabled` natif,
 *   jamais de clic possible sur un bouton `off` (même discipline que L1).
 * @param {string} [props.rafaleMode] - question.RAFALE_MODE (SOLO par
 *   défaut si absent/vide, contrat §3.4) — pilote le sous-titre des deux
 *   boutons, voir RAFALE_MODE_SUBLABELS ci-dessus.
 * @param {() => void} props.onValidate - émet RAFALE_VALIDATE
 * @param {() => void} props.onInvalidate - émet RAFALE_INVALIDATE
 */
export default function AnimRafaleActions({
  disabled = false,
  rafaleMode = 'SOLO',
  onValidate,
  onInvalidate,
}) {
  const sublabels = RAFALE_MODE_SUBLABELS[rafaleMode] || RAFALE_MODE_SUBLABELS.SOLO
  return (
    <div className="anim-rafale-actions">
      <button
        type="button"
        className="anim-conduct-btn anim-conduct-btn-go anim-rafale-action-btn"
        disabled={disabled}
        onClick={disabled ? undefined : onValidate}
      >
        RÉPONSE VALIDE
        {sublabels.valid && <span className="anim-conduct-btn-sub">{sublabels.valid}</span>}
      </button>
      <button
        type="button"
        className="anim-conduct-btn anim-conduct-btn-danger anim-rafale-action-btn"
        disabled={disabled}
        onClick={disabled ? undefined : onInvalidate}
      >
        RÉPONSE INVALIDE
        {sublabels.invalid && <span className="anim-conduct-btn-sub">{sublabels.invalid}</span>}
      </button>
    </div>
  )
}
