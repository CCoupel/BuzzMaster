/**
 * RoueTv — écran TV (statique) et tablette joueur d'une manche ROUE (v12.0.0, #144, maquettes M1 et M5).
 *
 * TV : équipes en haut avec leur cagnotte, catégorie + présentation, plateau plein écran, annonce, lettres
 * proposées (selon `ROUE_SHOW_LETTERS`), roue toujours affichée (bandeau ou arc selon la question).
 * Tablette (`isVPlayer`) : plateau pleine largeur et bouton STOP pour l'équipe qui a la main ; BUTTON est envoyé
 * par `onStop` (le serveur ignore tout appui hors SPINNING ou d'une autre équipe).
 * Aucune phrase en entrée : seulement `ROUE_BOARD` (jamais une lettre non révélée, contrat §9).
 */
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
  const gs = gameState || {}
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
    return (
      <div className="roue-vp" data-testid="roue-vp" style={{ '--roue-accent': accent }}>
        <div className="roue-vp-top">
          {teamName && <span className="roue-vp-me" style={{ background: colorOf(teamName) }}>{teamName}</span>}
          {catMeta && <span className="roue-vp-cat">{catMeta.icon} {catMeta.label}</span>}
          {q.ROUE_PRESENTATION && <span className="roue-vp-pres">{q.ROUE_PRESENTATION}</span>}
          {myBank !== undefined && <span className="roue-vp-bank">{fmtPts(myBank)} <small>cagnotte</small></span>}
        </div>
        <RoueGameBoard board={gs.ROUE_BOARD} flipIntervalMs={flipMs} size="tablet" />
        {solved ? (
          <div className="roue-vp-win">
            {gs.ROUE_WINNER === teamName ? `🎉 Votre équipe a trouvé la phrase${aw ? ` : +${fmtPts(aw)} points` : ''}`
              : gs.ROUE_WINNER ? `Les ${gs.ROUE_WINNER} ont trouvé la phrase` : 'Phrase dévoilée'}
          </div>
        ) : (
          <>
            <div className="roue-vp-msg"><Announcement parts={roueAnnouncement(gs, { mine })} /></div>
            {showLetters && <Letters letters={gs.ROUE_LETTERS} />}
            {canStop ? (
              <button type="button" className="roue-vp-stop" style={{ background: accent }} onClick={onStop}>
                STOP<small>appuyez pour arrêter la roue</small>
              </button>
            ) : (
              <div className="roue-vp-stop off">{mine ? 'Bouton actif quand la roue tourne' : "Ce n'est pas votre tour"}</div>
            )}
          </>
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
