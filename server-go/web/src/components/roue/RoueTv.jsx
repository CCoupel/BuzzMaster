/**
 * RoueTv — écran TV (statique) et tablette joueur d'une manche ROUE (v12.0.0, #144, maquettes M1 et M5).
 *
 * TV : équipes en haut avec leur cagnotte, catégorie + présentation, plateau plein écran, annonce, lettres
 * proposées (selon `ROUE_SHOW_LETTERS`), roue toujours affichée (bandeau ou arc selon la question).
 * Tablette (`isVPlayer`) : plateau pleine largeur, indice sur sa ligne, et la roue (mêmes props que la TV) à la place de l'ancien
 * bouton STOP : toute la zone est le bouton d'arrêt pour l'équipe qui a la main en SPINNING ; BUTTON est envoyé par `onStop`
 * (le serveur ignore tout appui hors SPINNING ou d'une autre équipe).
 * Aucune phrase en entrée : seulement `ROUE_BOARD` (jamais une lettre non révélée, contrat §9).
 */
import { useRef } from 'react'
import FortuneWheel from './FortuneWheel'
import RoueGameBoard from './RoueGameBoard'
import { fmtPts, roueAnnouncement, teamCss } from './roueState'
import './RoueTv.css'

function Announcement({ parts }) {
  return <>{parts.map((p, i) => (p.strong !== undefined ? <em key={i}>{p.strong}</em> : <span key={i}>{p.text}</span>))}</>
}

function Letters({ letters }) {
  if (!letters?.length) return null
  return (
    <div className="roue-letters">
      Lettres proposées :
      {letters.map((l, i) => <b key={`${l.LETTER}${i}`} className={l.HIT ? '' : 'miss'}>{l.LETTER}</b>)}
    </div>
  )
}

export default function RoueTv({ gameState, teams = {}, catMeta = null, isVPlayer = false, teamName = null, onStop = null }) {
  // Fin de manche : le serveur remet tout l'état ROUE à vide au STOP (stopUnsafe). L'écran garde alors le dernier
  // état de la manche (plateau résolu, cagnottes) tant que la question reste affichée.
  const lastRef = useRef(null)
  if (gameState?.ROUE_SUBPHASE) lastRef.current = gameState
  const ended = !gameState?.ROUE_SUBPHASE && ['STOPPED', 'REVEALED'].includes(gameState?.phase) && lastRef.current
  const gs = (ended ? { ...lastRef.current, phase: gameState.phase } : gameState) || {}
  const q = gs.question || {}
  const wheel = gs.ROUE_WHEEL
  const roster = gs.ROUE_TEAMS || []
  const bank = gs.ROUE_TEAM_BANK || {}
  const colorOf = (name) => teamCss(teams?.[name]?.COLOR)
  const current = gs.ROUE_CURRENT_TEAM || ''
  const accent = teamCss(gs.ROUE_CURRENT_TEAM_COLOR, current ? colorOf(current) : undefined)
  const sub = gs.ROUE_SUBPHASE || ''
  const layout = wheel?.LAYOUT === 'arc' ? 'arc' : 'strip'
  const showLetters = gs.ROUE_SHOW_LETTERS !== false
  const solved = sub === 'SOLVED'
  const flipMs = wheel?.FLIP_INTERVAL_MS || 600

  if (isVPlayer) {
    const mine = !!teamName && teamName === current
    const myBank = bank[teamName]
    const canStop = mine && sub === 'SPINNING' && typeof onStop === 'function'
    const aw = solved && gs.ROUE_WINNER ? gs.ROUE_AWARDS?.[teamName] : 0
    // Zone de la roue (M5 révisée) : à la place de l'ancien bouton STOP, affichée dans toutes les sous-phases (SOLVED
    // comprise) ; c'est un bouton seulement quand l'équipe a la main en SPINNING (le serveur reste seul juge de l'appui).
    const wheelEl = <FortuneWheel wheel={wheel} subphase={sub} teamColor={gs.ROUE_CURRENT_TEAM_COLOR} />
    return (
      <div className={`roue-vp roue-vp-${layout}`} data-testid="roue-vp" style={{ '--roue-accent': accent }}>
        <div className="roue-vp-top">
          {teamName && <span className="roue-vp-me" style={{ background: colorOf(teamName) }}>{teamName}</span>}
          {myBank !== undefined && <span className="roue-vp-bank">{fmtPts(myBank)} <small>cagnotte</small></span>}
        </div>
        {(catMeta || q.ROUE_PRESENTATION) && (
          <div className="roue-vp-hint">
            {catMeta && <span className="roue-vp-cat">{catMeta.icon} {catMeta.label}</span>}
            {q.ROUE_PRESENTATION && <span className="roue-vp-pres">{q.ROUE_PRESENTATION}</span>}
          </div>
        )}
        <RoueGameBoard board={gs.ROUE_BOARD} flipIntervalMs={flipMs} size="tablet" />
        <div className="roue-vp-info">
          {solved ? (
            <div className="roue-vp-win">
              {gs.ROUE_WINNER === teamName ? `🎉 Votre équipe a trouvé la phrase${aw ? ` : +${fmtPts(aw)} points` : ''}`
                : gs.ROUE_WINNER ? `Les ${gs.ROUE_WINNER} ont trouvé la phrase` : 'Phrase dévoilée'}
            </div>
          ) : (
            <div className="roue-vp-msg">
              {canStop ? <span>👆 Touchez la roue pour l'arrêter</span> : <Announcement parts={roueAnnouncement(gs, { mine })} />}
            </div>
          )}
          {!solved && showLetters && <Letters letters={gs.ROUE_LETTERS} />}
        </div>
        {canStop ? (
          <button type="button" className="roue-vp-zone tap" aria-label="Arrêter la roue" onClick={onStop}>
            {wheelEl}
            <span className="roue-vp-glyph" aria-hidden="true">👆 Touchez la roue</span>
          </button>
        ) : (
          <div className="roue-vp-zone">{wheelEl}</div>
        )}
      </div>
    )
  }

  return (
    <div className={`roue-tv roue-tv-${layout}`} data-testid="roue-tv" style={{ '--roue-accent': accent }}>
      <div className={`roue-banks${roster.length >= 7 ? ' dense' : ''}`}>
        {roster.map(name => {
          const win = solved && gs.ROUE_WINNER === name
          return (
            <div key={name} className={`roue-bank${name === current && !solved ? ' active' : ''}${win ? ' win' : ''}`} style={{ '--c': colorOf(name) }}>
              <span className="n">{name}</span>
              <span className="v">{win && gs.ROUE_AWARDS?.[name] ? `+${fmtPts(gs.ROUE_AWARDS[name])}` : fmtPts(bank[name])}</span>
            </div>
          )
        })}
      </div>
      <div className="roue-head">
        {catMeta && <span className="roue-cat">{catMeta.icon} {catMeta.label}</span>}
        {q.ROUE_PRESENTATION && <span className="roue-pres">{q.ROUE_PRESENTATION}</span>}
      </div>
      <div className="roue-mid"><RoueGameBoard board={gs.ROUE_BOARD} flipIntervalMs={flipMs} size="tv" /></div>
      <div className="roue-event"><Announcement parts={roueAnnouncement(gs)} /></div>
      {showLetters ? <Letters letters={gs.ROUE_LETTERS} /> : <div className="roue-letters" />}
      <div className={`roue-wheel-box roue-wheel-${layout}`}>
        <FortuneWheel wheel={wheel} subphase={sub} teamColor={gs.ROUE_CURRENT_TEAM_COLOR} />
      </div>
    </div>
  )
}
