/**
 * PictoRafaleConduct — conduite d'une manche Rafale Picto pour l'admin (M7) et l'animateur (M6), mise en page
 * COMPACTE de la maquette M9 (variante A) : un bandeau d'une ligne (état, trouvés, passes, restants, crédit ⓘ), le mot et
 * les boutons, puis le fil regroupé (passés dédoublonnés « ×n »). Aucune répétition : un seul montant de crédit (formule
 * en info-bulle), pas de ligne « Pool · catégorie », aide en info-bulle.
 *
 * « Passer » / « Trouvé » envoient l'ÉPOQUE du mot affiché (`PICTO_CANVAS_EPOCH`) : une action d'une autre époque est
 * refusée par le serveur (`STALE_WORD`). Mots montrés : le mot en cours et le fil — jamais de mot à venir (pas de tirage
 * d'avance, picto.md §17.6). Le dessinateur n'a aucun bouton : il annonce la passe à voix haute.
 */
import { getRgbColor, getContrastColor } from '../../utils/colorUtils'
import { canPictoPass, canRafaleFound, groupThread, presentationCount, roundPointsFormula, stateChipLabel } from './pictoRafaleState'

const HELP = "L'équipe de la manche est présélectionnée dans la colonne Équipes avec ce montant, ajustable ; « Attribuer les points » déclenche l'animation commune sur la TV."

export default function PictoRafaleConduct({ gs, word, thread = [], actions, variant = 'admin', onReveal = null }) {
  const q = gs.question || {}
  const epoch = gs.PICTO_CANVAS_EPOCH
  const found = gs.PICTO_FOUND_COUNT || 0
  const passes = gs.PICTO_PASS_COUNT || 0
  const left = gs.PICTO_POOL_AVAILABLE || 0
  const cost = Number(q.RAFALE_PICTO_PASS_COST) || 0
  const per = Number(q.POINTS) || 0
  const credit = gs.PICTO_ROUND_POINTS || 0
  const revealed = gs.phase === 'REVEALED'
  const gong = gs.phase === 'STOPPED' && gs.PICTO_OUTCOME === 'TIMEOUT'
  const poolEnd = gs.PICTO_OUTCOME === 'POOL_END'
  const canPass = canPictoPass(gs)
  const canFound = canRafaleFound(gs)
  const live = !revealed && gs.phase !== 'PREPARE' && gs.phase !== 'READY'
  const grouped = groupThread(thread)
  const nth = presentationCount(thread, word)

  // Fil sur fond de la couleur de l'équipe qui joue (PICTO_TEAM_COLOR) ; repli neutre si aucune équipe
  const teamColor = Array.isArray(gs.PICTO_TEAM_COLOR) && gs.PICTO_TEAM_COLOR.length === 3 ? gs.PICTO_TEAM_COLOR : null
  const threadStyle = teamColor
    ? { '--thread-bg': getRgbColor(teamColor), '--thread-fg': getContrastColor(teamColor) }
    : { '--thread-bg': '#475569', '--thread-fg': '#ffffff' }

  return (
    <div className={`picto-rafale picto-rafale-${variant}`} data-testid="picto-rafale">
      {/* Bandeau d'une ligne — placé AU-DESSUS de la zone de jeu (aperçu TV) */}
      <div className="picto-rafale-banner" data-testid="picto-rafale-stats">
        <b>🖍️ Rafale Picto</b>
        <span className="picto-chip" data-testid="picto-rafale-state">{stateChipLabel(gs)}</span>
        <span className="picto-chip" title="Mots trouvés">✅ <b>{found}</b></span>
        <span className="picto-chip" title={`Passes utilisées${cost > 0 ? ` (−${cost} pts chacune)` : ''}`}>⏭ {passes}</span>
        <span className="picto-chip">🗂 {left} restant{left > 1 ? 's' : ''}</span>
        <span className="picto-chip picto-credit" data-testid="picto-rafale-credit" title={`Crédit suggéré : ${roundPointsFormula(gs, q)} = ${credit}`}>
          Crédit <b>{credit} pts</b> ⓘ
        </span>
      </div>

      {live && (
        <div className="picto-rafale-line">
          <span className="picto-muted">Mot :</span>
          <b className="picto-rafale-word" data-testid="picto-rafale-word">{word || '—'}</b>
          {nth > 1 && <span className="picto-muted" data-testid="picto-rafale-nth">({nth}ᵉ présentation)</span>}
          <span className="picto-rafale-actions">
            <button type="button" className="picto-btn" data-testid="picto-rafale-pass" disabled={!canPass}
              title={gs.PICTO_PASSES_LEFT === 0 ? 'Plus de passe disponible' : `Passer ce mot (le dessinateur l'a annoncé)${cost > 0 ? ` : −${cost} pts` : ''}`}
              aria-label="Passer" onClick={() => actions.pass(epoch)}>
              ⏭ Passer{cost > 0 && <span className="picto-btn-sub">−{cost} pts</span>}
            </button>
            <button type="button" className="picto-btn ok" data-testid="picto-rafale-found" disabled={!canFound}
              aria-label={gong ? 'Trouvé sur le gong' : 'Trouvé'} onClick={() => actions.found(epoch)}>
              ✅ {gong ? 'Trouvé sur le gong' : 'Trouvé'}{per > 0 && <span className="picto-btn-sub">+{per} pts</span>}
            </button>
          </span>
        </div>
      )}

      {revealed && (
        <div className="picto-rafale-line">
          {poolEnd || !gs.PICTO_REVEALED_WORD ? (
            <span className="picto-muted" data-testid="picto-rafale-recap">
              {poolEnd ? 'Fin de pool' : 'Aucun mot révélé'}{credit === 0 ? " · crédit nul : rien n'est coché" : ''}
            </span>
          ) : (
            <>
              <span className="picto-muted">Mot révélé :</span>
              <span className="picto-chip picto-revealed-chip"><b data-testid="picto-rafale-word">{gs.PICTO_REVEALED_WORD}</b></span>
            </>
          )}
          <span className="picto-muted picto-rafale-help" data-testid="picto-rafale-recap-help" title={HELP}>→ crédit dans la colonne Équipes ⓘ</span>
        </div>
      )}

      {gong && (
        <p className="picto-muted" data-testid="picto-rafale-gong">
          ⏰ « {word || '…'} » a-t-il été donné sur le gong ? « Trouvé sur le gong » compte le mot et révèle ; sinon{' '}
          {onReveal ? <button type="button" className="picto-btn" data-testid="picto-rafale-reveal" onClick={onReveal}>👁 Révéler</button> : '« Révéler »'}.
        </p>
      )}
      {gs.phase === 'STOPPED' && poolEnd && (
        <p className="picto-muted" data-testid="picto-rafale-poolend">
          Plus aucun mot dans le pool : la manche est terminée.
          {onReveal && <> <button type="button" className="picto-btn" data-testid="picto-rafale-reveal" onClick={onReveal}>👁 Révéler</button></>}
        </p>
      )}

      {grouped.length > 0 && (
        <div className="picto-thread" data-testid="picto-thread" style={threadStyle}
          title={q.RAFALE_PICTO_POOL_END === 'LOOP' ? 'Les mots passés reviennent en fin de pool (rebouclage).' : undefined}>
          <ol aria-label="Mots de la manche">
            {grouped.map(g => (
              <li key={g.key} className={`picto-thread-${g.status}`} data-status={g.status}>
                {g.status === 'found' ? '✅' : '⏭'} {g.word}{g.count > 1 ? ` ×${g.count}` : ''}
              </li>
            ))}
          </ol>
        </div>
      )}
    </div>
  )
}
