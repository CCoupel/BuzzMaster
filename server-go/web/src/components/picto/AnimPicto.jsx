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
import { isDrawing, isPreparing, makePictoActions, pictoShownWord } from './pictoState'
import { categoryMeta } from '../../utils/categoryUtils'
import { useGame } from '../../hooks/GameContext'
import { useCategories } from '../../hooks/useCategories'
import './PictoCanvas.css'
import './PictoPanel.css'

/** Vue pure (testable) ; `AnimPicto` (défaut) la branche sur le contexte de jeu. */
export function AnimPictoView({ gameState: gs, teams = {}, bumpers = {}, actions, error, pictoWord = null, store = null, customCategories = [] }) {
  const q = gs.question || {}
  const cat = categoryMeta(q.CATEGORY, customCategories)
  const words = gs.PICTO_WORDS_COUNT || (q.PICTO_WORDS || []).length || 0
  const word = pictoShownWord(gs, pictoWord)
  // Mêmes équipes que la zone équipes de l'anim : celles qui ont au moins un joueur
  const withPlayers = new Set(Object.values(bumpers || {}).filter(b => b.TEAM).map(b => b.TEAM))
  const teamList = Object.keys(teams || {}).filter(name => withPlayers.has(name)).map(name => ({ name, color: teams[name]?.COLOR }))
  const points = q.POINTS
  return (
    <div className="anim-picto" data-testid="anim-picto">
      <div className="anim-picto-bar">
        <span className="picto-chip">✏️ <b>Picto</b></span>
        {cat && <span className="picto-chip">{cat.icon ? `${cat.icon} ` : ''}{cat.label}</span>}
        <span className="picto-chip">✍️ <b>{words}</b> mot{words > 1 ? 's' : ''} dans la question</span>
        {q.TIME ? <span className="picto-chip">⏱ {q.TIME} s</span> : null}
        {points ? <span className="picto-chip">🏆 {points} pts</span> : null}
      </div>
      {isPreparing(gs.phase) && <PictoPrepare gs={gs} teamList={teamList} actions={actions} compact />}
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
              <div className="picto-wordbox">
                <span className="picto-lbl">{gs.phase === 'REVEALED' ? 'Mot révélé' : 'Mot à faire deviner'}</span>
                <div className="picto-word" data-testid="anim-picto-word">{word || '—'}</div>
                {cat && <p className="picto-muted">{cat.label} · tiré parmi les {words} mot{words > 1 ? 's' : ''} de la question</p>}
              </div>
              {points ? <div className="picto-chips"><span className="picto-chip">Gain <b>+{points} pts</b></span></div> : null}
            </div>
          </div>
        </>
      )}
      <PictoErrorLine error={error} />
    </div>
  )
}


export default function AnimPicto() {
  const { gameState, teams, bumpers, sendMessage, pictoError, pictoWord, pictoCanvasStore } = useGame()
  const { categories } = useCategories()
  return (
    <AnimPictoView
      gameState={gameState}
      teams={teams}
      bumpers={bumpers}
      actions={makePictoActions(sendMessage)}
      error={pictoError}
      pictoWord={pictoWord}
      store={pictoCanvasStore}
      customCategories={categories}
    />
  )
}
