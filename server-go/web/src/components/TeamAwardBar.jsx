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
 * TeamAwardBar — bloc d'attribution COMPACT sur 2 lignes (#254, maquettes admin_game_award__compact_block_hints_toggle
 * et admin_game_award__team_or_player_level_hints_toggle) :
 *   ligne 1 : « N équipes/joueurs · source du montant » + commutateur « Indices » (si visible) ;
 *   ligne 2 : icônes « Tout sélectionner / Tout désélectionner » (nom accessible) + bouton
 *             « Attribuer les points (N) » / « Appliquer les pénalités (N) » (rouge).
 * Aucun champ de montant : le montant est le champ « Points » de la page. Le bouton est désactivé à
 * N = 0, pendant l'envoi ou si `blockedReason` ; la raison est affichée EN TEXTE sous le bloc
 * (une infobulle sur un bouton désactivé est invisible au toucher).
 */
export default function TeamAwardBar({
  count, unit = 'équipe', source, penaltyMode, sending, blockedReason, hints,
  onSelectAll, onSelectNone, onGive,
}) {
  return (
    <div className={`award${penaltyMode ? ' neg' : ''}`} role="group" aria-label="Attribution des points">
      <div className="award-l1">
        <span className="award-l1t">
          <b>{count} {unit}{count > 1 ? 's' : ''}</b> · {source}
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
        <button type="button" className="award-icon-btn" onClick={onSelectAll} aria-label="Tout sélectionner" title="Tout sélectionner">{ICON_ALL}</button>
        <button type="button" className="award-icon-btn" onClick={onSelectNone} aria-label="Tout désélectionner" title="Tout désélectionner">{ICON_NONE}</button>
        <button
          type="button"
          className={`award-give${penaltyMode ? ' neg' : ''}`}
          disabled={count === 0 || sending || !!blockedReason}
          onClick={onGive}
        >
          {penaltyMode ? 'Appliquer les pénalités' : 'Attribuer les points'} ({count})
        </button>
      </div>
      {blockedReason && <p className="award-reason" role="status">{blockedReason}</p>}
    </div>
  )
}
