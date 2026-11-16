/**
 * Briques de commande ROUE partagées par l'admin (RouePanel, M2) et l'animateur (AnimRoue, M7) —
 * contrat roue.md §5, §5.2, §8. Aucune règle de jeu : le serveur reste seul juge (réponse `ROUE_ERROR`).
 */
import { useEffect, useRef, useState } from 'react'
import { CONSONANTS, VOWELS, teamCss } from './roueState'
import './RoueControls.css'

const ERROR_TEXT = {
  VOWEL_NOT_AVAILABLE: 'Voyelle non disponible',
  NOT_A_LETTER: "Ce n'est pas une lettre",
  WRONG_SUBPHASE: "Action déjà prise en compte ou indisponible à cette étape",
  NOT_ALLOWED: 'Action non autorisée',
  INVALID_TEAMS: "Sélection d'équipes non conforme au mode",
}

/** Message du dernier refus serveur (`ROUE_ERROR`), masqué après 4 s. `error` = { CODE, MESSAGE, SEQ }. */
export function RoueErrorLine({ error }) {
  const [shown, setShown] = useState(null)
  useEffect(() => {
    if (!error?.SEQ) return undefined
    setShown(error)
    const t = setTimeout(() => setShown(null), 4000)
    return () => clearTimeout(t)
  }, [error])
  if (!shown) return null
  return <p className="roue-error" role="alert">{ERROR_TEXT[shown.CODE] || shown.MESSAGE || shown.CODE}</p>
}

/** Temps restant avant l'arrêt automatique, décompté localement depuis l'entrée en SPINNING (§6). */
export function useSpinCountdown(gs) {
  const spinning = gs?.ROUE_SUBPHASE === 'SPINNING'
  const maxMs = gs?.ROUE_WHEEL?.SPIN_MAX_MS || 0
  const [elapsed, setElapsed] = useState(0)
  useEffect(() => {
    if (!spinning || !(maxMs > 0)) { setElapsed(0); return undefined }
    const t0 = Date.now()
    setElapsed(0)
    const id = setInterval(() => setElapsed(Math.min(maxMs, Date.now() - t0)), 200)
    return () => clearInterval(id)
  }, [spinning, maxMs])
  return { spinning, maxMs, remainingMs: Math.max(0, maxMs - elapsed), ratio: maxMs > 0 ? Math.max(0, 1 - elapsed / maxMs) : 0 }
}

/**
 * Clavier des consonnes. `letters` = ROUE_LETTERS ; `enabled` = saisie possible (étape 4) ; `selected` / `onSelect`.
 * `teamColor(team)` (optionnel) : colore les consonnes jouées à la couleur de l'équipe qui les a proposées (M7).
 * `onSubmit` : Entrée. Le clavier physique sélectionne n'importe quelle lettre (le serveur refuse une voyelle).
 */
export function RoueKeyboard({ letters = [], enabled = false, selected = '', onSelect, onSubmit, teamColor, showVowels = true }) {
  const played = {}
  letters.forEach(l => { played[l.LETTER] = l })
  const latest = useRef({ onSelect, onSubmit })
  latest.current = { onSelect, onSubmit }
  useEffect(() => {
    if (!enabled) return undefined
    const onKey = (e) => {
      const tag = e.target?.tagName
      if (tag === 'INPUT' || tag === 'TEXTAREA' || e.ctrlKey || e.metaKey || e.altKey) return
      if (e.key === 'Enter') { latest.current.onSubmit?.(); return }
      if (/^[a-zA-Z]$/.test(e.key)) latest.current.onSelect?.(e.key.toUpperCase())
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [enabled])
  return (
    <>
      <div className="roue-kbd" role="group" aria-label="Consonnes">
        {CONSONANTS.map(l => {
          const p = played[l]
          const cls = p ? (p.HIT ? 'hit' : 'miss') : (selected === l ? 'sel' : '')
          return (
            <button key={l} type="button" className={`roue-key ${cls}${!enabled && !p ? ' off' : ''}`}
              style={p && teamColor ? { '--c': teamCss(teamColor(p.TEAM)) } : undefined}
              disabled={!!p || !enabled} aria-pressed={selected === l} onClick={() => onSelect?.(l)}>
              <b>{l}</b>
              {teamColor && <small>{p ? p.TEAM : (selected === l ? 'choisie' : ' ')}</small>}
            </button>
          )
        })}
      </div>
      {showVowels && (
        <div className="roue-vow">Voyelles non disponibles :
          {VOWELS.map(v => <button key={v} type="button" className="roue-key" disabled>{v}</button>)}
        </div>
      )}
    </>
  )
}

/**
 * Les deux boutons de l'étape 7 : « Tour suivant » (HAND SAME) / « Équipe suivante » (HAND NEXT, absent en SOLO).
 * Le bouton conseillé (`ROUE_NEXT_DEFAULT`) est mis en avant (`primary`), l'autre reste utilisable (§5.2).
 */
export function RoueNextButtons({ gs, gates, onNext, size = 'big', withTeams = true }) {
  const rec = gs?.ROUE_NEXT_DEFAULT || ''
  const same = withTeams && gs?.ROUE_CURRENT_TEAM ? ` — même équipe ▶ ${gs.ROUE_CURRENT_TEAM}` : ''
  const next = withTeams && gs?.ROUE_NEXT_TEAM ? ` ▶ ${gs.ROUE_NEXT_TEAM}` : ''
  return (
    <>
      <button type="button" className={`roue-btn ${size} ${rec === 'SAME' || gates.solo ? 'primary' : ''}`} disabled={!gates.canSame} onClick={() => onNext('SAME')}>
        Tour suivant{same}
      </button>
      {!gates.solo && (
        <button type="button" className={`roue-btn ${size} ${rec === 'NEXT' ? 'primary' : ''}`} disabled={!gates.canNextTeam} onClick={() => onNext('NEXT')}>
          Équipe suivante{next}
        </button>
      )}
    </>
  )
}
