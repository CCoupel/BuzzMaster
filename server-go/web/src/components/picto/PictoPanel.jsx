/**
 * PictoPanel — commandes admin d'une manche PICTO (v13.0.0, #192/#193, maquette M2). Même organisation que
 * `RouePanel` (centre de la page Jeu) : préparation (équipe + dessinateur) en PREPARE / READY, puis mot secret,
 * équipe et dessinateur, alerte de déconnexion en STARTED / PAUSED. La feuille en direct est dans l'aperçu TV.
 * Le mot n'apparaît ici qu'en manche (admin, animateur et dessinateur seulement, contrat §10).
 */
import { useEffect, useState } from 'react'
import PictoPrepare from './PictoPrepare'
import PictoRafaleConduct from './PictoRafaleConduct'
import { neededWords } from './pictoRafaleState'
import { usePictoThread } from './usePictoThread'
import { isRafalePicto } from '../../utils/pictoFamily'
import { canPictoFound, errorText, isDrawing, isPreparing, outcomeLabel, pictoShownWord, pictoWordsInfo, teamCss } from './pictoState'
import { categoryMeta } from '../../utils/categoryUtils'
import './PictoPanel.css'

export function PictoErrorLine({ error }) {
  const [shown, setShown] = useState(null)
  useEffect(() => {
    if (!error?.SEQ) return undefined
    setShown(error)
    const t = setTimeout(() => setShown(null), 4000)
    return () => clearTimeout(t)
  }, [error])
  if (!shown) return null
  return <p className="picto-error" role="alert">{errorText(shown)}</p>
}

/** Alerte « Alice est déconnectée » (manche en cours, §6.7) — admin et anim. */
export function PictoDisconnectAlert({ gs }) {
  if (!(gs.phase === 'STARTED' || gs.phase === 'PAUSED') || !gs.PICTO_DRAWER || gs.PICTO_DRAWER_CONNECTED !== false) return null
  return (
    <div className="picto-alert" role="alert" data-testid="picto-disconnected">
      <b>⚠ {gs.PICTO_DRAWER_NAME || 'Le dessinateur'} est déconnecté(e)</b>
      {' '}— la manche continue ; il retrouve sa feuille en revenant. Sinon, relancez la question (nouveau tirage, feuille vierge).
    </div>
  )
}

export default function PictoPanel({ gameState: gs, actions, error, teamList = [], pictoWord = null, customCategories = [] }) {
  const q = gs.question || {}
  const cat = categoryMeta(q.CATEGORY, customCategories)
  const { fromPool, available, words } = pictoWordsInfo(gs)
  const rafale = isRafalePicto(q.TYPE)
  const thread = usePictoThread(gs, pictoWord)
  const word = pictoShownWord(gs, pictoWord)
  const team = teamList.find(t => t.name === gs.PICTO_TEAM)
  const revealed = gs.phase === 'REVEALED'

  return (
    <div className="picto-panel" data-testid="picto-panel">
      {/* Titre exact de la maquette validée admin_game_picto__found_credit : « ✏️ Picto — révélé · ✅ Trouvé » en REVEALED */}
      <div className="picto-title" data-testid={revealed && gs.PICTO_OUTCOME ? 'picto-outcome' : undefined}>
        {rafale ? '🖍️ Rafale Picto' : '✏️ Picto'} — {isPreparing(gs.phase) ? 'préparation' : revealed ? `révélé${gs.PICTO_OUTCOME ? ` · ${outcomeLabel(gs)}` : ''}` : 'manche en cours'}
      </div>
      <p className="picto-muted">
        {fromPool
          ? <>🗂 <b>Pool Picto</b> · {available} mot{available > 1 ? 's' : ''} disponible{available > 1 ? 's' : ''}</>
          : <>✍️ <b>{words} mot{words > 1 ? 's' : ''}</b> dans la question</>}
        {cat ? ` · ${cat.icon ? `${cat.icon} ` : ''}${cat.label}` : ''} · {rafale ? 'mots tirés au fil de la manche' : 'un mot tiré au lancement'}
      </p>
      {isPreparing(gs.phase) && <PictoPrepare gs={gs} teamList={teamList} actions={actions} />}
      {rafale && isPreparing(gs.phase) && (
        <p className={`picto-muted${available < neededWords(q) ? ' picto-err' : ''}`} data-testid="picto-rafale-prep-hint">
          🗂 {available} mot{available > 1 ? 's' : ''} disponible{available > 1 ? 's' : ''}, environ {neededWords(q)} nécessaire{neededWords(q) > 1 ? 's' : ''} pour {q.TIME} s
        </p>
      )}
      {rafale && (isDrawing(gs.phase) || revealed) && (
        <PictoRafaleConduct gs={gs} word={word} thread={thread} actions={actions} variant="admin" customCategories={customCategories} />
      )}
      {!rafale && (isDrawing(gs.phase) || gs.phase === 'REVEALED') && (
        <div className="picto-live">
          <div className="picto-wordbox">
            {!revealed && <span className="picto-lbl">Mot · secret (admin, animateur, dessinateur)</span>}
            <div className="picto-word" data-testid="picto-word">{word || '—'}</div>
          </div>
          {!revealed && (
          <div className="picto-side">
            <div className="picto-muted" style={team ? { '--c': teamCss(team.color) } : undefined}>
              ✏️ {gs.PICTO_DRAWER_NAME || '—'} · {gs.PICTO_TEAM || '—'}
            </div>
            {canPictoFound(gs) && (
              <button type="button" className="picto-btn ok" data-testid="picto-found" onClick={() => actions.found()}>
                ✅ {gs.phase === 'STOPPED' ? 'Trouvé (sur le gong)' : 'Trouvé !'}
              </button>
            )}
          </div>
          )}
        </div>
      )}
      {!rafale && gs.phase === 'STOPPED' && gs.PICTO_OUTCOME && (
        <p className="picto-outcome" data-testid="picto-outcome">
          {outcomeLabel(gs)}
          {gs.PICTO_OUTCOME === 'TIMEOUT' ? ' — le mot n\'est pas encore affiché : « Trouvé » crédite encore l\'équipe, « REPONSE » montre le mot à tous, sans points.' : ''}
        </p>
      )}
      {revealed && (
        <p className="picto-muted">
          {rafale
            ? 'Crédit manuel par la colonne Équipes : l\'équipe de la manche est présélectionnée avec le montant suggéré par le serveur (ajustable) ; il déclenche l\'animation commune des points sur la TV.'
            : <>Crédit par la colonne Équipes (champ Points pré-rempli à {q.POINTS ?? '—'}), comme pour les autres types ; il déclenche l'animation commune des points sur la TV.</>}
        </p>
      )}
      <PictoDisconnectAlert gs={gs} />
      <PictoErrorLine error={error} />
    </div>
  )
}
