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
  INVALID_TEAMS: 'Équipe inconnue ou en double',
  NOT_A_VOWEL: "Ce n'est pas une voyelle",
  VOWELS_DISABLED: 'Achat de voyelles désactivé pour cette question',
  VOWEL_NOT_NOW: "Pas d'achat de voyelle à ce moment pour cette question",
  VOWEL_LIMIT_REACHED: 'Une seule voyelle à ce moment pour cette question',
  INSUFFICIENT_BANK: 'Cagnotte insuffisante',
  LETTER_ALREADY_PROPOSED: 'Voyelle déjà proposée',
  NO_VOWEL_LEFT: 'Plus de voyelle à trouver',
  NOT_FULL_SPEED: 'La roue prend de la vitesse : attendez la pleine vitesse pour l\'arrêter',
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
  // Durée maximale décomptée depuis la pleine vitesse (FULL_SPEED = true), pas pendant la montée (roue.md §6).
  const spinning = gs?.ROUE_SUBPHASE === 'SPINNING'
  const fullSpeed = spinning && gs?.ROUE_WHEEL?.FULL_SPEED === true
  const maxMs = gs?.ROUE_WHEEL?.SPIN_MAX_MS || 0
  const [elapsed, setElapsed] = useState(0)
  useEffect(() => {
    if (!fullSpeed || !(maxMs > 0)) { setElapsed(0); return undefined }
    const t0 = Date.now()
    setElapsed(0)
    const id = setInterval(() => setElapsed(Math.min(maxMs, Date.now() - t0)), 200)
    return () => clearInterval(id)
  }, [fullSpeed, maxMs])
  return { spinning, fullSpeed, maxMs, remainingMs: Math.max(0, maxMs - elapsed), ratio: maxMs > 0 ? Math.max(0, 1 - elapsed / maxMs) : 0 }
}

/**
 * Clavier des consonnes. `letters` = ROUE_LETTERS ; `enabled` = saisie possible (étape 4) ; `selected` / `onSelect`.
 * `teamColor(team)` (optionnel) : colore les consonnes jouées à la couleur de l'équipe qui les a proposées (M7).
 * `onSubmit` : Entrée. Le clavier physique sélectionne n'importe quelle lettre (le serveur refuse une voyelle).
 */
export function RoueKeyboard({ letters = [], enabled = false, selected = '', onSelect, onSubmit, teamColor, showVowels = true, vowelsEnabled = false, vowelsLabel = null, consonantsEnabled = enabled }) {
  const played = {}
  letters.forEach(l => { played[l.LETTER] = l })
  const latest = useRef({ onSelect, onSubmit })
  latest.current = { onSelect, onSubmit }
  useEffect(() => {
    if (!enabled && !vowelsEnabled) return undefined
    const onKey = (e) => {
      const tag = e.target?.tagName
      if (tag === 'INPUT' || tag === 'TEXTAREA' || e.ctrlKey || e.metaKey || e.altKey) return
      if (e.key === 'Enter') { latest.current.onSubmit?.(); return }
      if (/^[a-zA-Z]$/.test(e.key)) latest.current.onSelect?.(e.key.toUpperCase())
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [enabled, vowelsEnabled])
  return (
    <>
      <div className="roue-kbd" role="group" aria-label="Consonnes">
        {CONSONANTS.map(l => {
          const p = played[l]
          const cls = p ? (p.HIT ? 'hit' : 'miss') : (selected === l ? 'sel' : '')
          return (
            <button key={l} type="button" className={`roue-key ${cls}${!consonantsEnabled && !p ? ' off' : ''}`}
              style={p && teamColor ? { '--c': teamCss(teamColor(p.TEAM)) } : undefined}
              disabled={!!p || !consonantsEnabled} aria-pressed={selected === l} onClick={() => onSelect?.(l)}>
              <b>{l}</b>
              {teamColor && <small>{p ? p.TEAM : (selected === l ? 'choisie' : ' ')}</small>}
            </button>
          )
        })}
      </div>
      {showVowels && !vowelsEnabled && !vowelsLabel && (
        <div className="roue-vow">Voyelles non disponibles :
          {VOWELS.map(v => <button key={v} type="button" className="roue-key" disabled>{v}</button>)}
        </div>
      )}
      {showVowels && (vowelsEnabled || vowelsLabel) && (
        <div className="roue-vow roue-vow-buy" role="group" aria-label="Voyelles">
          {vowelsLabel && <b className="roue-vow-label">{vowelsLabel}</b>}
          {VOWELS.map(v => {
            const p = played[v]
            const cls = p ? (p.HIT ? 'hit' : 'miss') : (selected === v ? 'sel' : '')
            return (
              <button key={v} type="button" className={`roue-key ${cls}${!vowelsEnabled && !p ? ' off' : ''}`}
                style={p && teamColor ? { '--c': teamCss(teamColor(p.TEAM)) } : undefined}
                disabled={!!p || !vowelsEnabled} aria-pressed={selected === v} onClick={() => onSelect?.(v)}>
                <b>{v}</b>
                {teamColor && <small>{p ? p.TEAM : (selected === v ? 'choisie' : ' ')}</small>}
              </button>
            )
          })}
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
