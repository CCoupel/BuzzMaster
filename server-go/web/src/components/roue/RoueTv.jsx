/**
 * RoueTv — écran TV (statique) et tablette joueur d'une manche ROUE (v12.0.0, #144, maquettes M1 et M5).
 *
 * TV : équipes en haut avec leur cagnotte, catégorie + présentation, plateau plein écran, annonce, lettres
 * proposées (selon `ROUE_SHOW_LETTERS`), roue toujours affichée (bandeau ou arc selon la question).
 * Tablette (`isVPlayer`) : plateau pleine largeur, indice sur sa ligne, et la roue (mêmes props que la TV) à la place de l'ancien
 * bouton STOP : toute la zone est le bouton d'arrêt pour l'équipe qui a la main en SPINNING, à pleine vitesse seulement
 * (`ROUE_WHEEL.FULL_SPEED` ; pendant la montée : « La roue prend de la vitesse… », roue non tapable) ; BUTTON est envoyé par `onStop`
 * (le serveur ignore tout appui hors SPINNING ou d'une autre équipe).
 * Aucune phrase en entrée : seulement `ROUE_BOARD` (jamais une lettre non révélée, contrat §9).
 */
import { useEffect, useRef, useState } from 'react'
import FortuneWheel from './FortuneWheel'
import RoueGameBoard from './RoueGameBoard'
import { bonusValue, fmtPts, isBonusEvent, isVowel, roueAnnouncement, teamCss, vowelInfo } from './roueState'
import './RoueTv.css'

const FADE_MS = 600
const reducedMotion = () => { try { return !!window.matchMedia?.('(prefers-reduced-motion: reduce)').matches } catch { return false } }

/**
 * Bandeau « 🎁 BONUS ! case 800 » (v12.2.0, #316/#317). Retiré au choix de la lettre ; maintenu monté pendant le fondu
 * de 600 ms de la case (BONUS éphémère seulement, même durée que la roue) ; sans fondu en permanent et en `prefers-reduced-motion`.
 */
export function BonusBanner({ on, value, fade = true }) {
  const [fading, setFading] = useState(false)
  const last = useRef({ value: 0, was: false })
  if (on) last.current = { value, was: true }
  useEffect(() => {
    if (on) { setFading(false); return undefined }
    if (!last.current.was) return undefined
    last.current.was = false
    if (!fade || reducedMotion()) return undefined // permanent : retiré sans fondu (§17.5)
    setFading(true)
    const t = setTimeout(() => setFading(false), FADE_MS)
    return () => clearTimeout(t)
  }, [on]) // eslint-disable-line react-hooks/exhaustive-deps
  if (!on && !fading) return null
  const v = on ? value : last.current.value
  return (
    <div className={`roue-bonus-ribbon${on ? '' : ' fading'}`} role="status" data-testid="roue-bonus">
      🎁 BONUS !{v > 0 ? ` case ${v}` : ''}
    </div>
  )
}

function Announcement({ parts }) {
  return <>{parts.map((p, i) => (p.br ? <br key={i} /> : p.strong !== undefined ? <em key={i}>{p.strong}</em> : <span key={i}>{p.text}</span>))}</>
}

const ALPHABET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'.split('')

/** Achat de voyelles actif (v12.1.0, #147, maquette TV C) : alphabet complet sur une ligne (statique), prix rappelé. */
function Alphabet({ letters, price }) {
  const played = {}
  ;(letters || []).forEach(l => { played[l.LETTER] = l })
  return (
    <div className="roue-letters roue-alpha" data-testid="roue-alphabet">
      <span className="lab">🔤 {fmtPts(price)}</span>
      {ALPHABET.map(l => {
        const p = played[l]
        return <b key={l} className={`${p ? (p.HIT ? 'hit' : 'miss') : 'idle'}${isVowel(l) ? ' vu' : ''}`}>{l}</b>
      })}
    </div>
  )
}

/** Variation de cagnotte d'une voyelle (débit à l'achat, gain au retournement), lue sur le dernier événement. */
function bankDelta(gs, name) {
  const ev = gs.ROUE_LAST_EVENT || {}
  if (ev.TEAM !== name && name !== gs.ROUE_CURRENT_TEAM) return null
  if ((ev.KIND === 'VOWEL_HIT' || ev.KIND === 'VOWEL_MISS') && ev.TEAM === name) return { dir: 'down', text: `−${fmtPts(ev.VALUE)}` }
  if (ev.KIND === 'FLIP' && isVowel(ev.LETTER) && ev.GAIN > 0 && name === gs.ROUE_CURRENT_TEAM) return { dir: 'up', text: `+${fmtPts(ev.GAIN)}` }
  return null
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
  const bonus = isBonusEvent(gs) // 🎁 BONUS (v12.1.0, #308) : bandeau sur le plateau
  const bonusBanner = <BonusBanner on={bonus} value={bonusValue(gs)} fade={gs.ROUE_BONUS_MODE !== 'PERMANENT'} />

  if (isVPlayer) {
    const mine = !!teamName && teamName === current
    const myBank = bank[teamName]
    const canStop = mine && sub === 'SPINNING' && wheel?.FULL_SPEED === true && typeof onStop === 'function'
    const aw = solved && gs.ROUE_WINNER ? gs.ROUE_AWARDS?.[teamName] : 0
    // Zone de la roue (M5 révisée) : à la place de l'ancien bouton STOP, affichée dans toutes les sous-phases (SOLVED
    // comprise) ; un bouton (calque sur toute la zone) n'existe que quand l'équipe a la main en SPINNING à pleine vitesse (le serveur reste seul juge).
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
        <div className="roue-bonus-wrap">
          <RoueGameBoard board={gs.ROUE_BOARD} flipIntervalMs={flipMs} size="tablet" />
          {bonusBanner}
        </div>
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
          {!solved && showLetters && (vowelInfo(gs).enabled ? <Alphabet letters={gs.ROUE_LETTERS} price={vowelInfo(gs).price} /> : <Letters letters={gs.ROUE_LETTERS} />)}
        </div>
        {/* Structure STABLE : la roue ne change jamais de parent (sinon React la démonte au passage tapable → non tapable, et
            `Wheel` remonté directement en `land` se placerait d'un coup sur la case : plus de freinage). Le bouton d'arrêt est un
            calque transparent posé sur toute la zone, seulement quand `canStop`. */}
        <div className={`roue-vp-zone${canStop ? ' tap' : ''}`}>
          {wheelEl}
          {canStop && <span className="roue-vp-glyph" aria-hidden="true">👆 Touchez la roue</span>}
          {canStop && <button type="button" className="roue-vp-tap" aria-label="Arrêter la roue" onClick={onStop} />}
        </div>
      </div>
    )
  }

  return (
    <div className={`roue-tv roue-tv-${layout}`} data-testid="roue-tv" style={{ '--roue-accent': accent }}>
      <div className={`roue-banks${roster.length >= 7 ? ' dense' : ''}`}>
        {roster.map(name => {
          const win = solved && gs.ROUE_WINNER === name
          const delta = vowelInfo(gs).enabled && !solved ? bankDelta(gs, name) : null
          return (
            <div key={name} className={`roue-bank${name === current && !solved ? ' active' : ''}${win ? ' win' : ''}`} style={{ '--c': colorOf(name) }}>
              <span className="n">{name}</span>
              <span className="v">{win && gs.ROUE_AWARDS?.[name] ? `+${fmtPts(gs.ROUE_AWARDS[name])}` : fmtPts(bank[name])}</span>
              {delta && <span className={`delta ${delta.dir}`} data-testid="roue-bank-delta">{delta.text}</span>}
            </div>
          )
        })}
      </div>
      <div className="roue-head">
        {catMeta && <span className="roue-cat">{catMeta.icon} {catMeta.label}</span>}
        {q.ROUE_PRESENTATION && <span className="roue-pres">{q.ROUE_PRESENTATION}</span>}
      </div>
      <div className="roue-mid roue-bonus-wrap">
        <RoueGameBoard board={gs.ROUE_BOARD} flipIntervalMs={flipMs} size="tv" />
        {bonusBanner}
      </div>
      <div className="roue-event"><div><Announcement parts={roueAnnouncement(gs)} /></div></div>
      {showLetters ? (vowelInfo(gs).enabled ? <Alphabet letters={gs.ROUE_LETTERS} price={vowelInfo(gs).price} /> : <Letters letters={gs.ROUE_LETTERS} />) : <div className="roue-letters" />}
      <div className={`roue-wheel-box roue-wheel-${layout}`}>
        <FortuneWheel wheel={wheel} subphase={sub} teamColor={gs.ROUE_CURRENT_TEAM_COLOR} />
      </div>
    </div>
  )
}
