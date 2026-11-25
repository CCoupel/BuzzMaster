/**
 * PictoRafaleConduct — conduite d'une manche Rafale Picto pour l'admin (M7) et l'animateur (M6) : compteurs, mot en
 * cours, « Passer » / « Trouvé » et fil des mots (picto.md §17.5). Les deux boutons envoient l'ÉPOQUE du mot affiché
 * (`PICTO_CANVAS_EPOCH`) : une action d'une autre époque est refusée par le serveur (`STALE_WORD`).
 *
 * Mots montrés : le mot en cours et le fil (trouvés ✅, passés ⏭, en cours ✏️) — jamais de mot à venir (pas de tirage
 * d'avance, picto.md §17.6). Variante `anim` : boutons larges, « Passer » sous le pouce gauche, « Trouvé » sous le
 * pouce droit. Le dessinateur n'a aucun bouton : il annonce la passe à voix haute.
 */
import { categoryMeta } from '../../utils/categoryUtils'
import { getRgbColor, getContrastColor } from '../../utils/colorUtils'
import { canPictoPass, canRafaleFound, passLimitLabel, roundPointsFormula, THREAD_ICON } from './pictoRafaleState'

export default function PictoRafaleConduct({ gs, word, thread = [], actions, variant = 'admin', customCategories = [], onReveal = null }) {
  const q = gs.question || {}
  const epoch = gs.PICTO_CANVAS_EPOCH
  const found = gs.PICTO_FOUND_COUNT || 0
  const cost = Number(q.RAFALE_PICTO_PASS_COST) || 0
  const per = Number(q.POINTS) || 0
  const revealed = gs.phase === 'REVEALED'
  const gong = gs.phase === 'STOPPED' && gs.PICTO_OUTCOME === 'TIMEOUT'
  const cat = categoryMeta(gs.PICTO_WORD_CATEGORY || q.CATEGORY, customCategories)
  const wordNo = thread.length
  const canPass = canPictoPass(gs)
  const canFound = canRafaleFound(gs)
  // Fil en ligne, fond = couleur de l'équipe qui joue (PICTO_TEAM_COLOR) ; repli neutre si aucune équipe
  const teamColor = Array.isArray(gs.PICTO_TEAM_COLOR) && gs.PICTO_TEAM_COLOR.length === 3 ? gs.PICTO_TEAM_COLOR : null
  const threadStyle = teamColor
    ? { '--thread-bg': getRgbColor(teamColor), '--thread-fg': getContrastColor(teamColor) }
    : { '--thread-bg': '#475569', '--thread-fg': '#ffffff' }
  const showWord = !revealed && gs.phase !== 'PREPARE' && gs.phase !== 'READY'

  return (
    <div className={`picto-rafale picto-rafale-${variant}`} data-testid="picto-rafale">
      <div className="picto-rafale-stats" data-testid="picto-rafale-stats">
        <span className="picto-chip">✅ <b>{found}</b> trouvé{found > 1 ? 's' : ''}</span>
        <span className="picto-chip">⏭ {gs.PICTO_PASS_COUNT || 0} passe{(gs.PICTO_PASS_COUNT || 0) > 1 ? 's' : ''} · {passLimitLabel(q)}</span>
        <span className="picto-chip">−{cost} pts / passe</span>
        <span className="picto-chip">🗂 {gs.PICTO_POOL_AVAILABLE || 0} restant{(gs.PICTO_POOL_AVAILABLE || 0) > 1 ? 's' : ''}</span>
        <span className="picto-chip picto-credit" data-testid="picto-rafale-credit">Crédit {revealed ? '' : 'en cours '}: <b>{gs.PICTO_ROUND_POINTS || 0} pts</b></span>
      </div>

      {showWord && (
        <div className="picto-wordbox">
          <span className="picto-lbl">
            {wordNo > 0 ? `Mot ${wordNo}` : 'Mot'}{cat ? ` · ${cat.icon ? `${cat.icon} ` : ''}${cat.label}` : ''}
          </span>
          <div className="picto-word" data-testid="picto-rafale-word">{word || '—'}</div>
          <p className="picto-muted">visible par l'admin, l'animateur et le dessinateur</p>
        </div>
      )}
      {revealed && (
        <div className="picto-wordbox">
          <span className="picto-lbl">Mot révélé</span>
          <div className="picto-word" data-testid="picto-rafale-word">{gs.PICTO_REVEALED_WORD || '—'}</div>
          <p className="picto-muted" data-testid="picto-rafale-recap">
            {gs.PICTO_OUTCOME === 'POOL_END' ? 'Fin du pool · ' : ''}{found} trouvé{found > 1 ? 's' : ''} · {gs.PICTO_PASS_COUNT || 0} passe{(gs.PICTO_PASS_COUNT || 0) > 1 ? 's' : ''} · crédit suggéré {roundPointsFormula(gs, q)} = {gs.PICTO_ROUND_POINTS || 0}
          </p>
        </div>
      )}

      {!revealed && (
        <div className="picto-rafale-actions">
          <button type="button" className="picto-btn" data-testid="picto-rafale-pass" disabled={!canPass}
            title={gs.PICTO_PASSES_LEFT === 0 ? 'Plus de passe disponible' : 'Passer ce mot (le dessinateur l\'a annoncé)'}
            aria-label="Passer"
            onClick={() => actions.pass(epoch)}>
            ⏭ Passer<span className="picto-btn-sub">−{cost} pts</span>
          </button>
          <button type="button" className="picto-btn ok" data-testid="picto-rafale-found" disabled={!canFound}
            aria-label={gong ? 'Trouvé sur le gong' : 'Trouvé'}
            onClick={() => actions.found(epoch)}>
            ✅ {gong ? 'Trouvé sur le gong' : 'Trouvé'}<span className="picto-btn-sub">+{per} pts</span>
          </button>
        </div>
      )}
      {gong && (
        <p className="picto-muted" data-testid="picto-rafale-gong">
          ⏰ Temps écoulé — « {word || '…'} » a-t-il été donné sur le gong ? « Trouvé sur le gong » compte le mot et révèle ; sinon « Révéler ».
          {onReveal && <> <button type="button" className="picto-btn" data-testid="picto-rafale-reveal" onClick={onReveal}>👁 Révéler</button></>}
        </p>
      )}
      {gs.phase === 'STOPPED' && gs.PICTO_OUTCOME === 'POOL_END' && (
        <p className="picto-muted" data-testid="picto-rafale-poolend">🗂 Plus aucun mot dans le pool : la manche est terminée.
          {onReveal && <> <button type="button" className="picto-btn" data-testid="picto-rafale-reveal" onClick={onReveal}>👁 Révéler</button></>}
        </p>
      )}

      {thread.length > 0 && (
        <div className="picto-thread" data-testid="picto-thread" style={threadStyle}>
          <span className="picto-lbl">Mots de la manche</span>
          <ol>
            {thread.map((t, i) => (
              <li key={`${t.epoch}-${i}`} className={`picto-thread-${t.status}`} data-status={t.status}>
                {i + 1} · {THREAD_ICON[t.status]} {t.word}{t.status === 'current' ? '…' : ''}
              </li>
            ))}
          </ol>
          {(gs.question?.RAFALE_PICTO_POOL_END === 'LOOP') && <p className="picto-muted">Les mots passés reviennent en fin de pool (rebouclage).</p>}
        </div>
      )}
    </div>
  )
}
