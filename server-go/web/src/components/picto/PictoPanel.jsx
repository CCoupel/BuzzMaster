/**
 * PictoPanel — commandes admin d'une manche PICTO (v13.0.0, #192/#193, maquette M2). Même organisation que
 * `RouePanel` (centre de la page Jeu) : préparation (équipe + dessinateur) en PREPARE / READY, puis mot secret,
 * équipe et dessinateur, alerte de déconnexion en STARTED / PAUSED. La feuille en direct est dans l'aperçu TV.
 * Le mot n'apparaît ici qu'en manche (admin, animateur et dessinateur seulement, contrat §10).
 */
import { useEffect, useState } from 'react'
import PictoPrepare from './PictoPrepare'
import { canPictoFound, errorText, isDrawing, isPreparing, outcomeLabel, pictoShownWord, teamCss } from './pictoState'
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
  const words = gs.PICTO_WORDS_COUNT || (q.PICTO_WORDS || []).length || 0
  const word = pictoShownWord(gs, pictoWord)
  const team = teamList.find(t => t.name === gs.PICTO_TEAM)

  return (
    <div className="picto-panel" data-testid="picto-panel">
      <div className="picto-title">✏️ Picto — {isPreparing(gs.phase) ? 'préparation' : gs.phase === 'REVEALED' ? 'révélé' : 'manche en cours'}</div>
      <p className="picto-muted">
        ✍️ <b>{words} mot{words > 1 ? 's' : ''}</b> dans la question
        {cat ? ` · ${cat.icon ? `${cat.icon} ` : ''}${cat.label}` : ''} · un mot tiré au lancement
      </p>
      {isPreparing(gs.phase) && <PictoPrepare gs={gs} teamList={teamList} actions={actions} />}
      {(isDrawing(gs.phase) || gs.phase === 'REVEALED') && (
        <div className="picto-live">
          <div className="picto-wordbox">
            <span className="picto-lbl">{gs.phase === 'REVEALED' ? 'Mot révélé' : 'Mot · secret (admin, animateur, dessinateur)'}</span>
            <div className="picto-word" data-testid="picto-word">{word || '—'}</div>
          </div>
          <div className="picto-side">
            <div className="picto-muted" style={team ? { '--c': teamCss(team.color) } : undefined}>
              ✏️ {gs.PICTO_DRAWER_NAME || '—'} · {gs.PICTO_TEAM || '—'}
            </div>
            {canPictoFound(gs) && (
              <button type="button" className="picto-btn ok" data-testid="picto-found" onClick={actions.found}>
                ✅ {gs.phase === 'STOPPED' ? 'Trouvé (sur le gong)' : 'Trouvé !'}
              </button>
            )}
          </div>
        </div>
      )}
      {(gs.phase === 'STOPPED' || gs.phase === 'REVEALED') && gs.PICTO_OUTCOME && (
        <p className="picto-outcome" data-testid="picto-outcome">
          {outcomeLabel(gs)}
          {gs.phase === 'STOPPED' && gs.PICTO_OUTCOME === 'TIMEOUT' ? ' — le mot n\'est pas encore affiché : « Trouvé » crédite encore l\'équipe, « Reveal » montre le mot à tous, sans points.' : ''}
          {gs.phase === 'REVEALED' ? ' — crédit par la colonne Équipes (champ Points pré-rempli aux points de la question) ; l\'animation de gain commune s\'affiche sur la TV.' : ''}
        </p>
      )}
      <PictoDisconnectAlert gs={gs} />
      <PictoErrorLine error={error} />
    </div>
  )
}
