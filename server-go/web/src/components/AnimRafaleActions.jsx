import './AnimRafaleActions.css'

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
 * @param {() => void} props.onValidate - émet RAFALE_VALIDATE
 * @param {() => void} props.onInvalidate - émet RAFALE_INVALIDATE
 */
export default function AnimRafaleActions({
  disabled = false,
  onValidate,
  onInvalidate,
}) {
  return (
    <div className="anim-rafale-actions">
      <button
        type="button"
        className="anim-conduct-btn anim-conduct-btn-go anim-rafale-action-btn"
        disabled={disabled}
        onClick={disabled ? undefined : onValidate}
      >
        RÉPONSE VALIDE
        <span className="anim-conduct-btn-sub">équipe suivante</span>
      </button>
      <button
        type="button"
        className="anim-conduct-btn anim-conduct-btn-danger anim-rafale-action-btn"
        disabled={disabled}
        onClick={disabled ? undefined : onInvalidate}
      >
        RÉPONSE INVALIDE
        <span className="anim-conduct-btn-sub">équipe suivante</span>
      </button>
    </div>
  )
}
