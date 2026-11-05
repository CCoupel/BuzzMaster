import './TeamAwardBar.css'

// Bornes du montant par entrée (contrat TEAM_POINTS_BATCH : entier non nul de −999 à 999).
export const AWARD_LIMIT = 999

const ICON_ALL = (
  <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
    <rect x="2.5" y="2.5" width="15" height="15" rx="3" /><path d="M6 10.5l3 3 5-6" />
  </svg>
)
const ICON_NONE = (
  <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
    <rect x="2.5" y="2.5" width="15" height="15" rx="3" />
  </svg>
)

/**
 * TeamAwardBar — bloc d'attribution sur 3 lignes (#289, maquette admin_game_award__selection_lines ;
 * remplace le bloc compact 2 lignes de #254) :
 *   ligne 1 : source du montant + commutateur « Indices » (si visible) ;
 *   ligne 2 : compteur de sélection (`role="status"`) — « x joueurs · y équipes » (niveau joueur),
 *             « y équipes » (niveau équipe), « Aucune sélection » — + icônes Tout (dé)sélectionner ;
 *   ligne 3 : bouton pleine largeur, SANS compteur dans son libellé, sur une seule ligne
 *             (nowrap + ellipse : plus de débordement).
 * `teamCount` = équipes distinctes des joueurs sélectionnés (niveau joueur uniquement ; calculé par GamePage).
 * Aucun champ de montant : le montant est le champ « Points » de la page. Le bouton est désactivé à
 * N = 0, pendant l'envoi ou si `blockedReason` ; la raison est affichée EN TEXTE sous le bloc
 * (une infobulle sur un bouton désactivé est invisible au toucher).
 */
export default function TeamAwardBar({
  count, unit = 'équipe', teamCount, source, penaltyMode, sending, blockedReason, hints,
  onSelectAll, onSelectNone, onGive,
}) {
  const plural = (n, w) => `${n} ${w}${n > 1 ? 's' : ''}`
  const countLabel = count === 0
    ? 'Aucune sélection'
    : unit === 'joueur'
      ? `${plural(count, 'joueur')} · ${plural(teamCount ?? 0, 'équipe')}`
      : plural(count, unit)
  return (
    <div className={`award${penaltyMode ? ' neg' : ''}`} role="group" aria-label="Attribution des points">
      <div className="award-l1">
        <span className="award-l1t">
          {source}
        </span>
        {hints?.visible && (
          <label
            className="award-hints-switch"
            title="Tenir compte des indices : chaque équipe ou joueur reçoit la valeur du champ × le coefficient des indices révélés à son buzz (100 % / 67 % / 33 %)"
          >
            <input
              type="checkbox"
              role="switch"
              checked={hints.on}
              onChange={(e) => hints.onChange(e.target.checked)}
              aria-label="Tenir compte des indices"
            />
            <span className="award-hints-track" aria-hidden="true" />
            <span>Indices</span>
          </label>
        )}
      </div>
      <div className="award-l2">
        <span className={`award-count${count === 0 ? ' empty' : ''}`} role="status" aria-live="polite">{countLabel}</span>
        <button type="button" className="award-icon-btn" onClick={onSelectAll} aria-label="Tout sélectionner" title="Tout sélectionner">{ICON_ALL}</button>
        <button type="button" className="award-icon-btn" onClick={onSelectNone} aria-label="Tout désélectionner" title="Tout désélectionner">{ICON_NONE}</button>
      </div>
      <div className="award-l3">
        <button
          type="button"
          className={`award-give${penaltyMode ? ' neg' : ''}`}
          disabled={count === 0 || sending || !!blockedReason}
          onClick={onGive}
        >
          {penaltyMode ? 'Appliquer les pénalités' : 'Attribuer les points'}
        </button>
      </div>
      {blockedReason && <p className="award-reason" role="status">{blockedReason}</p>}
    </div>
  )
}
