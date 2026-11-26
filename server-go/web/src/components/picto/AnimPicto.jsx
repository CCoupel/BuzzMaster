/**
 * AnimPicto — panneau de l'animateur pour une manche PICTO (v13.0.0, #192/#193, maquette M1, retenue A).
 * Préparation : équipe puis dessinateur (touche un joueur connecté, ou « Au hasard »). En manche : le dessin en direct
 * (relayé par le serveur, jamais par un UPDATE), le mot à faire deviner (secret : animateur, admin, dessinateur) et le
 * gain. Pause / arrêt restent les gestes globaux de la conduite. Aucun remplacement du dessinateur en cours de manche
 * (le recours est la relance de la question). « Trouvé » et la révélation arrivent avec la fin de manche (#194).
 */
import PictoCanvasView from './PictoCanvasView'
import PictoPrepare from './PictoPrepare'
import { PictoDisconnectAlert, PictoErrorLine } from './PictoPanel'
import PictoRafaleConduct from './PictoRafaleConduct'
import { neededWords } from './pictoRafaleState'
import { usePictoThread } from './usePictoThread'
import { isRafalePicto } from '../../utils/pictoFamily'
import { canPictoFound, isDrawing, isPreparing, makePictoActions, outcomeLabel, pictoShownWord, pictoWordsInfo } from './pictoState'
import { categoryMeta } from '../../utils/categoryUtils'
import { useGame } from '../../hooks/GameContext'
import { useCategories } from '../../hooks/useCategories'
import './PictoCanvas.css'
import './PictoPanel.css'

/** Vue pure (testable) ; `AnimPicto` (défaut) la branche sur le contexte de jeu. */
export function AnimPictoView({ gameState: gs, teams = {}, bumpers = {}, actions, error, pictoWord = null, store = null, customCategories = [], onReveal = null }) {
  const q = gs.question || {}
  // v13.1.0 : indice = catégorie du mot en cours (pool Picto), sinon celle de la question (crédit)
  const cat = categoryMeta(gs.PICTO_WORD_CATEGORY || q.CATEGORY, customCategories)
  const { fromPool, available, words } = pictoWordsInfo(gs)
  const rafale = isRafalePicto(q.TYPE)
  const thread = usePictoThread(gs, pictoWord)
  const word = pictoShownWord(gs, pictoWord)
  // Mêmes équipes que la zone équipes de l'anim : celles qui ont au moins un joueur
  const withPlayers = new Set(Object.values(bumpers || {}).filter(b => b.TEAM).map(b => b.TEAM))
  const teamList = Object.keys(teams || {}).filter(name => withPlayers.has(name)).map(name => ({ name, color: teams[name]?.COLOR }))
  const points = q.POINTS
  const outcomeShown = !!gs.PICTO_OUTCOME && (gs.phase === 'STOPPED' || gs.phase === 'REVEALED')
  const outcomeSuffix = gs.phase === 'REVEALED' ? ' — le mot est affiché sur la TV' : (gs.PICTO_OUTCOME === 'TIMEOUT' ? ' — le mot n\'est pas encore affiché' : '')
  return (
    <div className="anim-picto" data-testid="anim-picto">
      <div className="anim-picto-bar">
        <span className="picto-chip">{rafale ? '🖍️' : '✏️'} <b>{rafale ? 'Rafale Picto' : 'Picto'}</b></span>
        {cat && <span className="picto-chip">{cat.icon ? `${cat.icon} ` : ''}{cat.label}</span>}
        {fromPool
          ? <span className="picto-chip">🗂 <b>Pool Picto</b> · {available} mot{available > 1 ? 's' : ''} disponible{available > 1 ? 's' : ''}</span>
          : <span className="picto-chip">✍️ <b>{words}</b> mot{words > 1 ? 's' : ''} dans la question</span>}
        {q.TIME ? <span className="picto-chip">⏱ {q.TIME} s</span> : null}
        {points ? <span className="picto-chip">🏆 {points} pts</span> : null}
      </div>
      {isPreparing(gs.phase) && <PictoPrepare gs={gs} teamList={teamList} actions={actions} compact />}
      {rafale && isPreparing(gs.phase) && (
        <p className={`picto-muted${available < neededWords(q) ? ' picto-err' : ''}`} data-testid="picto-rafale-prep-hint">
          🗂 {available} mot{available > 1 ? 's' : ''} disponible{available > 1 ? 's' : ''}, environ {neededWords(q)} nécessaire{neededWords(q) > 1 ? 's' : ''} pour {q.TIME} s
        </p>
      )}
      {(isDrawing(gs.phase) || gs.phase === 'REVEALED') && (
        <>
          <PictoDisconnectAlert gs={gs} />
          <div className="anim-picto-body">
            <div className="anim-picto-sheet">
              <span className="picto-lbl">Dessin en direct</span>
              <div className="picto-sheet picto-sheet-fit">{store && <PictoCanvasView store={store} />}</div>
              <p className="picto-muted">✏️ {gs.PICTO_DRAWER_NAME || '—'} dessine pour <b>{gs.PICTO_TEAM || '—'}</b></p>
            </div>
            <div className="anim-picto-side">
              {rafale ? (
                <PictoRafaleConduct gs={gs} word={word} thread={thread} actions={actions} variant="anim" onReveal={onReveal} />
              ) : (<>
              <div className="picto-wordbox">
                {/* Libellé de la carte du mot : texte exact des maquettes validées (anim_picto__found_credit) */}
                {outcomeShown ? (
                  <div className="picto-lbl" data-testid="anim-picto-outcome"><b>{outcomeLabel(gs)}</b>{outcomeSuffix}</div>
                ) : (
                  <span className="picto-lbl">Mot à faire deviner</span>
                )}
                <div className="picto-word" data-testid="anim-picto-word">{word || '—'}</div>
                {cat && <p className="picto-muted">{cat.label} · {fromPool ? 'tiré dans le Pool Picto' : <>tiré parmi les {words} mot{words > 1 ? 's' : ''} de la question</>}</p>}
              </div>
              {gs.phase === 'STOPPED' && gs.PICTO_OUTCOME === 'TIMEOUT' && (
                <p className="picto-muted">Réponse donnée sur le gong ? « Trouvé » crédite encore l'équipe. Sinon, « Révéler » montre le mot à tous, sans points.</p>
              )}
              <div className="anim-picto-actions">
                {canPictoFound(gs) && (
                  <button type="button" className="picto-btn ok" data-testid="anim-picto-found" onClick={() => actions.found()}>
                    ✅ {gs.phase === 'STOPPED' ? 'Trouvé (sur le gong)' : 'Trouvé !'}
                  </button>
                )}
                {gs.phase === 'STOPPED' && onReveal && (
                  <button type="button" className="picto-btn" data-testid="anim-picto-reveal" onClick={onReveal}>👁 Révéler le mot</button>
                )}
              </div>
              </>)}
            </div>
          </div>
        </>
      )}
      <PictoErrorLine error={error} />
    </div>
  )
}


export default function AnimPicto() {
  const { gameState, teams, bumpers, sendMessage, revealAnswer, pictoError, pictoWord, pictoCanvasStore, pictoFound, pictoPass } = useGame()
  const { categories } = useCategories()
  return (
    <AnimPictoView
      gameState={gameState}
      teams={teams}
      bumpers={bumpers}
      actions={makePictoActions(sendMessage, { found: pictoFound, pass: pictoPass })}
      error={pictoError}
      pictoWord={pictoWord}
      store={pictoCanvasStore}
      customCategories={categories}
      onReveal={() => revealAnswer()}
    />
  )
}
