/**
 * AnimRoue — commandes de l'animateur pour une manche ROUE (v12.0.0, #144, maquette M7, décision GATE 1.5 :
 * l'animateur COMMANDE la manche). Mêmes gestes que l'admin (contrat roue.md §8.2) : ROUE_SPIN, ROUE_STOP,
 * ROUE_LETTER, ROUE_FLIP, ROUE_ANSWER, ROUE_NEXT. **Jamais** ROUE_SET_TEAMS, ROUE_PASS ni ROUE_REVEAL (admin seulement).
 * Le premier geste valable est pris, l'autre poste reçoit `WRONG_SUBPHASE` (sans effet).
 * Aucun texte explicatif : la phrase (lettres trouvées à la couleur de l'équipe), les 7 boutons d'étape, les
 * consonnes jouées par équipe, la valeur de la case et les cagnottes.
 */
import { useEffect, useState } from 'react'
import { useGame } from '../../hooks/GameContext'
import { RoueErrorLine, RoueKeyboard, useSpinCountdown } from './RoueControls'
import { currentSegment, fmtPts, isBonusEvent, isVowel, makeRoueActions, roueGates, segmentLabel, teamCss, vowelInfo } from './roueState'
import './AnimRoue.css'

const fold = (c) => c.normalize('NFD').replace(/[̀-ͯ]/g, '').toUpperCase()

function Phrase({ gs, colorOf }) {
  const phrase = gs.question?.ROUE_PHRASE || ''
  if (!phrase) return null
  const found = {}
  ;(gs.ROUE_LETTERS || []).forEach(l => { if (l.HIT) found[l.LETTER] = l.TEAM })
  const last = (gs.ROUE_LETTERS || []).slice(-1)[0]?.LETTER
  const sub = gs.ROUE_SUBPHASE
  return (
    <div className="anim-roue-sol" aria-label="Phrase solution">
      {[...phrase].map((ch, i) => {
        if (ch === ' ') return <span key={i}>&nbsp;&nbsp;</span>
        const f = fold(ch)
        if ((sub === 'LIT' || sub === 'VOWEL_LIT') && f === last) return <span key={i} className="lit">{ch}</span>
        const team = found[f]
        return <span key={i} className={team ? 'f' : ''} style={team ? { '--c': colorOf(team) } : undefined}>{ch}</span>
      })}
    </div>
  )
}

/** Vue pure (testable) ; `AnimRoue` (défaut) la branche sur le contexte de jeu. */
export function AnimRoueView({ gameState: gs, teams = {}, actions, error }) {
  const [selected, setSelected] = useState('')
  const sub = gs.ROUE_SUBPHASE || ''
  useEffect(() => { setSelected('') }, [sub])
  const cd = useSpinCountdown(gs)
  const gates = roueGates(gs)
  const colorOf = (name) => teamCss(teams?.[name]?.COLOR)
  const step = gs.ROUE_STEP || 0
  const q = gs.question || {}
  const seg = currentSegment(gs)
  const current = gs.ROUE_CURRENT_TEAM
  const live = gs.phase === 'STARTED'
  const rec = gs.ROUE_NEXT_DEFAULT
  const ev = gs.ROUE_LAST_EVENT || {}
  const gain = gs.ROUE_TURN_GAIN || 0
  const bank = gs.ROUE_TEAM_BANK || {}
  const vi = vowelInfo(gs)
  const vowelOpen = live && (vi.canBuyBefore || vi.canBuyAfter)
  const pickedVowel = isVowel(selected) ? selected : ''
  const buyShown = live && (vi.showBefore || vi.showAfter)

  const cls = (n, extra = '') => `anim-roue-sb ${step > n ? 'done' : step === n ? 'cur' : ''} ${extra}`.trim()
  const lastLetter = (gs.ROUE_LETTERS || []).slice(-1)[0]
  const gainText = (sub === 'VOWEL_LIT' || (sub === 'LIT' && isVowel(ev.LETTER))) ? `${ev.LETTER} · ${ev.COUNT} case${ev.COUNT > 1 ? 's' : ''} · −${fmtPts(ev.VALUE)}`
    : (sub === 'VOWEL_REVEALING' || (sub === 'REVEALING' && isVowel(ev.LETTER))) ? `${ev.LETTER} : ${ev.COUNT} case${ev.COUNT > 1 ? 's' : ''}${ev.GAIN > 0 ? ` = +${fmtPts(ev.GAIN)}` : ''}`
    : sub === 'REVEALING' ? `${ev.LETTER} : ${ev.COUNT} cases × ${fmtPts(ev.VALUE)}`
    : sub === 'ANSWER' ? `Gain du tour : +${fmtPts(gain)}`
      : sub === 'TURN_END' ? (ev.GAIN > 0 ? `+${fmtPts(ev.GAIN)} → cagnotte ${fmtPts(bank[current])}` : (lastLetter && lastLetter.HIT === false ? (isVowel(lastLetter.LETTER) ? `${lastLetter.LETTER} absente · −${fmtPts(vi.price)}` : `${lastLetter.LETTER} absente — aucun gain`) : 'Aucun gain'))
        : ''

  return (
    <div className="anim-roue" data-testid="anim-roue">
      <div className="anim-roue-bar">
        <span className="anim-roue-chip">🎡 <b>ROUE</b></span>
        {vi.enabled && <span className="anim-roue-chip anim-roue-vchip">🔤 {fmtPts(vi.price)} · {({ BEFORE: 'avant', AFTER: 'après', BOTH: 'avant / après' })[vi.timing] || 'avant'}</span>}
        {q.ROUE_PRESENTATION && <span className="anim-roue-chip">« {q.ROUE_PRESENTATION} »</span>}
        {current && sub !== 'SOLVED' && <span className="anim-roue-hand" style={{ background: colorOf(current) }}>▶ {current}</span>}
      </div>
      {isBonusEvent(gs) && <div className="anim-roue-bonus" role="status" data-testid="anim-roue-bonus">🎁 BONUS !</div>}
      <Phrase gs={gs} colorOf={colorOf} />

      {live && (
        <div className="anim-roue-steps">
          <button type="button" className={cls(1)} disabled={!gates.canSpin} onClick={actions.spin}><b>1</b>Lancer<br />la roue</button>
          <button type="button" className={cls(2)} disabled={!(gates.canStop && gates.fullSpeed)} onClick={actions.stop}><b>2</b>Forcer<br />l'arrêt</button>
          <button type="button" className={`${cls(3)} ${step === 3 ? 'wait' : ''}`} disabled><b>3</b>{step === 3 ? <>La roue<br />ralentit…</> : 'Case'}</button>
          <button type="button" className={cls(4)} disabled={!gates.canLetter || !selected || !!pickedVowel} onClick={() => selected && !pickedVowel && actions.letter(selected)}>
            <b>4</b>{selected && !pickedVowel && gates.canLetter ? <>Valider la<br />lettre {selected}</> : <>Valider la<br />consonne</>}
          </button>
          <button type="button" className={cls(5)} disabled={!gates.canFlip} onClick={actions.flip}><b>5</b>Retourner<br />les cases</button>
          <button type="button" className={`${cls(6)} ${step === 6 ? 'ok' : ''}`} disabled={!gates.canAnswer} onClick={actions.answer}><b>✓</b>Réponse<br />validée</button>
          <button type="button" className={`anim-roue-sb ${gates.canSame ? (rec === 'SAME' || gates.solo ? 'cur' : 'alt') : ''}`} disabled={!gates.canSame} onClick={() => actions.next('SAME')}>
            <b>7</b>Tour suivant<br />▶ {current}
          </button>
          {!gates.solo && (
            <button type="button" className={`anim-roue-sb ${gates.canNextTeam ? (rec === 'NEXT' ? 'cur' : 'alt') : ''}`} disabled={!gates.canNextTeam} onClick={() => actions.next('NEXT')}>
              <b>7</b>Équipe suivante<br />▶ {gs.ROUE_NEXT_TEAM}
            </button>
          )}
        </div>
      )}
      <RoueErrorLine error={error} />

      <div className="anim-roue-body">
        <div className="anim-roue-panel">
          <RoueKeyboard letters={gs.ROUE_LETTERS} enabled={live && gates.canLetter} selected={selected} onSelect={setSelected}
            onSubmit={() => (pickedVowel ? (vowelOpen && actions.buyVowel(pickedVowel)) : selected && actions.letter(selected))} teamColor={colorOf}
            showVowels={buyShown} vowelsEnabled={vowelOpen} consonantsEnabled={live && gates.canLetter}
            vowelsLabel={buyShown ? `🔤 ${fmtPts(vi.price)}` : null} />
          {buyShown && (
            <button type="button" className="anim-roue-buy" disabled={!vowelOpen || !pickedVowel}
              onClick={() => pickedVowel && (actions.buyVowel(pickedVowel), setSelected(''))}>
              {pickedVowel ? `🔤 Acheter le ${pickedVowel} — ${fmtPts(vi.price)}`
                : !vi.left ? '🔤 Plus de voyelle'
                  : !vi.rich ? `🔤 Cagnotte < ${fmtPts(vi.price)}`
                    : (vi.limitBefore || vi.limitAfter) ? '🔤 1 voyelle à ce moment'
                      : sub === 'ANSWER' ? `🔤 Autre voyelle · ${fmtPts(vi.price)}` : `🔤 Voyelle · ${fmtPts(vi.price)}`}
            </button>
          )}
          <div className="anim-roue-legend">
            {(gs.ROUE_TEAMS || []).map(n => <span key={n}><i style={{ background: colorOf(n) }} />{n}</span>)}
            <span>barrée = absente</span>
          </div>
        </div>
        <div className="anim-roue-panel">
          <div className={`anim-roue-case${!seg && !cd.spinning ? ' none' : ''}${cd.spinning ? ' spin' : ''}${seg?.KIND === 'bonus' && !cd.spinning ? ' gold' : ''}`}>
            <small>Case de la roue</small>
            {cd.spinning ? (cd.fullSpeed ? `la roue tourne · ${Math.ceil(cd.remainingMs / 1000)} s` : 'la roue prend de la vitesse…') : seg ? segmentLabel(seg) : '—'}
          </div>
          <div className="anim-roue-gain">{gainText}</div>
          <h4>Cagnottes de la manche</h4>
          {(gs.ROUE_TEAMS || []).map(n => (
            <div key={n} className={`anim-roue-team${n === current && sub !== 'SOLVED' ? ' active' : ''}`} style={{ '--c': colorOf(n) }}>
              <span>{n}</span><span className="v">{fmtPts(bank[n])}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

export default function AnimRoue() {
  const { gameState, teams, sendMessage, roueError } = useGame()
  return <AnimRoueView gameState={gameState} teams={teams} actions={makeRoueActions(sendMessage)} error={roueError} />
}
