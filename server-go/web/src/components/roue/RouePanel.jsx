/**
 * RouePanel — commandes admin d'une manche ROUE (v12.0.0, #144, maquette M2 « B »).
 *
 * Avant le départ (PREPARE / READY) : choix et ordre des équipes (`ROUE_SET_TEAMS`).
 * En manche : phrase solution (admin seulement), cagnottes, bandeau des 7 étapes piloté par `ROUE_STEP`, action de
 * l'étape courante, « Passer la main » et « Dévoiler la phrase » (admin seulement, §8.2).
 * Aucune règle de jeu ici : `ROUE_SUBPHASE` / `ROUE_STEP` viennent du serveur, qui reste seul juge (`ROUE_ERROR`).
 */
import { useEffect, useState } from 'react'
import RoueSteps from './RoueSteps'
import { RoueErrorLine, RoueKeyboard, RoueNextButtons, useSpinCountdown } from './RoueControls'
import { currentSegment, fmtPts, isBonusEvent, isVowel, roueGates, roueSolo, segmentLabel, teamCss, vowelInfo, VOWELS } from './roueState'
import './RouePanel.css'

const MODE_LABEL = { SOLO: 'Solo', CHACUN_SON_TOUR: 'Chacun son tour', TANT_QUE_JE_GAGNE: 'Tant que je gagne' }
const GAINS_LABEL = { WINNER_ONLY: 'Seul le gagnant encaisse', ALL_TEAMS: 'Tout le monde encaisse' }
const TIMING_LABEL = { BEFORE: 'avant de tourner', AFTER: "après l'arrêt", BOTH: 'avant et après' }
const VGAIN_LABEL = { REFUND: 'Mise remboursée', PER_OCCURRENCE: 'Prix × nombre de lettres' }
const fold = (c) => c.normalize('NFD').replace(/[̀-ͯ]/g, '').toUpperCase()

/** Phrase solution (visible ici seulement) : noir = affichée sur la TV, jaune = éclairée, vert = en retournement. */
function Solution({ gs }) {
  const phrase = gs.question?.ROUE_PHRASE || ''
  if (!phrase) return null
  const hits = new Set((gs.ROUE_LETTERS || []).filter(l => l.HIT).map(l => l.LETTER))
  const last = (gs.ROUE_LETTERS || []).slice(-1)[0]?.LETTER
  const sub = gs.ROUE_SUBPHASE
  return (
    <div className="roue-sol" aria-label="Phrase solution">
      {[...phrase].map((ch, i) => {
        if (ch === ' ') return <span key={i}>&nbsp;&nbsp;</span>
        const f = fold(ch)
        const isLetter = /^[A-Z]$/.test(f)
        let cls = ''
        if (sub === 'SOLVED' || !isLetter) cls = 'rv'
        else if (f === last && (sub === 'LIT' || sub === 'VOWEL_LIT')) cls = 'lit'
        else if (f === last && (sub === 'REVEALING' || sub === 'VOWEL_REVEALING')) cls = 'new'
        else if (hits.has(f)) cls = 'rv'
        return <span key={i} className={cls}>{ch}</span>
      })}
    </div>
  )
}

function TeamPicker({ gs, teamList, actions, waitReasons }) {
  const solo = roueSolo(gs)
  const selected = gs.ROUE_TEAMS || []
  const toggle = (name) => {
    if (solo) return actions.setTeams(selected.includes(name) ? [] : [name])
    actions.setTeams(selected.includes(name) ? selected.filter(t => t !== name) : [...selected, name])
  }
  return (
    <div className="roue-picker">
      <div className="roue-status">Équipes participantes — {MODE_LABEL[gs.question?.ROUE_MODE] || MODE_LABEL.TANT_QUE_JE_GAGNE}</div>
      {(waitReasons || []).map((r, i) => <p key={i} className="roue-wait">⏳ {r}</p>)}
      <div className="roue-pick">
        {teamList.map(t => (
          <label key={t.name} className={t.ready === false ? 'not-ready' : ''} style={{ '--c': t.color }}>
            <input type="checkbox" checked={selected.includes(t.name)} disabled={t.ready === false && !selected.includes(t.name)} onChange={() => toggle(t.name)} />
            {selected.includes(t.name) && !solo && <span className="roue-order">{selected.indexOf(t.name) + 1}</span>}
            {t.name}
          </label>
        ))}
      </div>
      <p className="roue-hint">Ordre de passage = ordre des équipes cochées. {solo ? 'Mode Solo : une équipe.' : '2 équipes au moins.'}</p>
    </div>
  )
}
/** Clavier des voyelles + bouton « Acheter le X — prix » (étape 1 après « Acheter une voyelle », étape 6 « autre voyelle »). */
function VowelBuy({ gs, actions, selected, setSelected, label, onDone }) {
  const v = vowelInfo(gs)
  const picked = VOWELS.includes(selected) ? selected : ''
  return (
    <div className="roue-vowbuy" data-testid="roue-vowel-buy">
      <div className="roue-vow roue-vow-buy" role="group" aria-label="Voyelles">
        <b className="roue-vow-label">{label || `Voyelle — ${fmtPts(v.price)} :`}</b>
        {VOWELS.map(l => {
          const played = (gs.ROUE_LETTERS || []).find(x => x.LETTER === l)
          return (
            <button key={l} type="button" className={`roue-key ${played ? (played.HIT ? 'hit' : 'miss') : (picked === l ? 'sel' : '')}`}
              disabled={!!played} aria-pressed={picked === l} onClick={() => setSelected(l)}><b>{l}</b></button>
          )
        })}
      </div>
      <div className="roue-row">
        <button type="button" className="roue-btn vow solid big" disabled={!picked}
          onClick={() => { if (picked) { actions.buyVowel(picked); setSelected(''); onDone?.() } }}>
          {picked ? `Acheter le ${picked} — ${fmtPts(v.price)}` : 'Choisissez une voyelle'}
        </button>
        {onDone && <button type="button" className="roue-btn ghost" onClick={() => { setSelected(''); onDone() }}>Annuler</button>}
      </div>
    </div>
  )
}

/** Libellé du bouton d'achat à l'étape 1 selon les réglages et la cagnotte. */
function buyLabel(v) {
  if (!v.left) return 'Plus de voyelle à trouver'
  if (v.limitBefore) return '1 voyelle par tour : déjà achetée'
  if (!v.rich) return `Acheter une voyelle — ${fmtPts(v.price)} (cagnotte : ${fmtPts(v.bank)})`
  return `🔤 Acheter une voyelle — ${fmtPts(v.price)}`
}

function Action({ gs, gates, actions, selected, setSelected, picking, setPicking }) {
  const vi = vowelInfo(gs)
  const seg = currentSegment(gs)
  const cd = useSpinCountdown(gs)
  const team = gs.ROUE_CURRENT_TEAM
  const ev = gs.ROUE_LAST_EVENT || {}
  const gain = gs.ROUE_TURN_GAIN || 0
  const submitLetter = (l) => { if (!l) return; if (isVowel(l)) { if (vi.canBuyAfter) actions.buyVowel(l) } else actions.letter(l) }
  switch (gs.ROUE_SUBPHASE) {
    case 'TURN':
      if (!gs.ROUE_CONSONANTS_LEFT && !gates.canSpin) {
        return (<>
          <h5>Plus aucune consonne à trouver</h5>
          <div className="roue-row">
            <button type="button" className="roue-btn ok big" onClick={actions.answer}>✓ Réponse validée</button>
            <RoueNextButtons gs={gs} gates={gates} onNext={actions.next} />
            {vi.showBefore && <button type="button" className="roue-btn vow big" disabled={!vi.canBuyBefore} onClick={() => setPicking(true)}>{buyLabel(vi)}</button>}
          </div>
          {picking && vi.canBuyBefore && <VowelBuy gs={gs} actions={actions} selected={selected} setSelected={setSelected} onDone={() => setPicking(false)} />}
        </>)
      }
      return (<>
        {isBonusEvent(gs) && <div className="roue-bonus" role="status" data-testid="roue-bonus">🎁 BONUS ! — les {team} gardent la main. Remettez le cadeau, puis relancez la roue (aucun point compté).</div>}
        <h5>1 — {vi.before ? 'Lancer la roue ou acheter une voyelle' : 'Lancer la roue'}</h5>
        <div className="roue-row">
          <button type="button" className="roue-btn primary big" onClick={actions.spin} disabled={!gates.canSpin}>🎡 Lancer la roue</button>
          {vi.showBefore && <button type="button" className="roue-btn vow big" disabled={!vi.canBuyBefore} onClick={() => setPicking(true)}>{buyLabel(vi)}</button>}
          {gs.ROUE_TURN_VOWEL === true && <button type="button" className="roue-btn ok big" onClick={actions.answer}>✓ Réponse validée</button>}
        </div>
        {picking && vi.canBuyBefore && <VowelBuy gs={gs} actions={actions} selected={selected} setSelected={setSelected} onDone={() => setPicking(false)} />}
        {vi.before && vi.canBuyBefore === false && vi.rich === false && vi.left && <p className="roue-hint">Cagnotte des {team} : {fmtPts(vi.bank)} — il faut {fmtPts(vi.price)} pour acheter une voyelle.</p>}
        {!vi.before && vi.after && <p className="roue-hint">Voyelles : achat après l'arrêt de la roue seulement (étape 4), pour cette question.</p>}
        <p className="roue-hint">La roue se met à tourner sur la TV ; l'équipe qui a la main buzze pour l'arrêter.</p>
      </>)
    case 'VOWEL_LIT':
      return (<>
        <h5>1 — Voyelle {ev.LETTER} : {ev.COUNT} case{ev.COUNT > 1 ? 's' : ''} éclairée{ev.COUNT > 1 ? 's' : ''} sur le plateau</h5>
        <div className="roue-row"><span>Payé : <span className="roue-cost">−{fmtPts(ev.VALUE)}</span></span></div>
        <div className="roue-row roue-gap"><button type="button" className="roue-btn primary big" disabled={!gates.canFlip} onClick={actions.flip}>🔄 Retourner les cases</button></div>
      </>)
    case 'VOWEL_REVEALING':
      return (<>
        <h5>1 — Voyelle {ev.LETTER} : retournement</h5>
        <div className="roue-row"><span>Les {ev.COUNT} case{ev.COUNT > 1 ? 's' : ''} se retournent une à une sur la TV…</span>{ev.GAIN > 0 && <span className="roue-gain">{ev.LETTER} : +{fmtPts(ev.GAIN)}</span>}</div>
        <p className="roue-hint">Aucun geste : retour à l'étape 1 à la fin de l'animation, les {team} gardent la main.</p>
      </>)
    case 'SPINNING':
      return (<>
        <h5>2 — Arrêt de la roue</h5>
        {gs.ROUE_WHEEL?.FULL_SPEED === true ? (
          <div className="roue-row">
            <span>En attente du buzz des {team}…</span>
            <span className="roue-bar"><i style={{ width: `${cd.ratio * 100}%` }} /></span>
            <span className="roue-hint">arrêt automatique dans {Math.ceil(cd.remainingMs / 1000)} s</span>
          </div>
        ) : (
          <div className="roue-row"><span>La roue prend de la vitesse… personne ne peut encore l'arrêter, ni les {team}, ni vous.</span></div>
        )}
        <div className="roue-row roue-gap"><button type="button" className="roue-btn big" disabled={gs.ROUE_WHEEL?.FULL_SPEED !== true} onClick={actions.stop}>✋ Forcer l'arrêt de la roue</button></div>
      </>)
    case 'STOPPING': {
      const to = gs.ROUE_WHEEL?.STOP?.TO
      const target = Number.isInteger(to) ? gs.ROUE_WHEEL.SEGMENTS?.[to] : null
      return (<>
        <h5>3 — Affichage de la case</h5>
        <div className="roue-row"><span>La roue ralentit et s'arrête sur</span>{target && <span className="roue-seg">{segmentLabel(target)}</span>}</div>
        <p className="roue-hint">Aucun geste : le clavier des consonnes s'ouvre dès l'arrêt.</p>
      </>)
    }
    case 'LETTER':
      return (<>
        <h5>4 — {vi.after ? 'Consonne ou voyelle' : 'Sélection de la consonne'} {seg && <span className="roue-seg">{segmentLabel(seg)}</span>}</h5>
        <RoueKeyboard letters={gs.ROUE_LETTERS} enabled selected={selected} onSelect={setSelected} onSubmit={() => submitLetter(selected)}
          vowelsLabel={vi.after ? `ou une voyelle — ${fmtPts(vi.price)} :` : null} vowelsEnabled={vi.canBuyAfter} />
        {vi.after && <p className="roue-hint">Une voyelle remplace la consonne : la case {segmentLabel(seg)} n'est pas jouée.{vi.rich ? '' : ' Cagnotte insuffisante.'}</p>}
        <div className="roue-row roue-gap">
          {selected && isVowel(selected) ? (
            <button type="button" className="roue-btn vow solid big" disabled={!vi.canBuyAfter} onClick={() => submitLetter(selected)}>Acheter le {selected} — {fmtPts(vi.price)}</button>
          ) : (
            <button type="button" className="roue-btn primary big" disabled={!selected} onClick={() => submitLetter(selected)}>
              {selected ? `Valider la lettre ${selected}` : `Choisissez une consonne${vi.after ? ' ou une voyelle' : ''}`}
            </button>
          )}
          <span className="roue-hint">ou tapez la lettre au clavier puis Entrée</span>
        </div>
      </>)
    case 'LIT':
      return (<>
        <h5>5 — {isVowel(ev.LETTER) ? 'Voyelle' : 'Consonne'} {ev.LETTER} : {ev.COUNT} case{ev.COUNT > 1 ? 's' : ''} éclairée{ev.COUNT > 1 ? 's' : ''} sur le plateau</h5>
        <div className="roue-row"><span>Les cases du <b>{ev.LETTER}</b> sont éclairées sur la TV.</span></div>
        <div className="roue-row roue-gap"><button type="button" className="roue-btn primary big" onClick={actions.flip}>🔄 Retourner les cases</button></div>
      </>)
    case 'REVEALING':
      return (<>
        <h5>5 — Affichage des cases</h5>
        <div className="roue-row"><span>Les {ev.COUNT} case{ev.COUNT > 1 ? 's' : ''} se retournent une à une sur la TV…</span><span className="roue-gain">{ev.LETTER} : {ev.COUNT} × {fmtPts(ev.VALUE)} = +{fmtPts(gain)}</span></div>
        <p className="roue-hint">Aucun geste : l'étape 6 s'ouvre à la fin de l'animation.</p>
      </>)
    case 'ANSWER':
      return (<>
        <h5>6 — Réponse (bonus)</h5>
        <div className="roue-row">
          <button type="button" className="roue-btn ok big" onClick={actions.answer}>✓ Réponse validée</button>
          <span className="roue-hint">sinon, étape 7 :</span>
          <RoueNextButtons gs={gs} gates={gates} onNext={actions.next} />
        </div>
        {vi.showAfter && (vi.limitAfter
          ? <div className="roue-row"><button type="button" className="roue-btn vow big" disabled>1 voyelle par arrêt : déjà achetée</button></div>
          : <VowelBuy gs={gs} actions={actions} selected={selected} setSelected={setSelected} label={`ou une autre voyelle — ${fmtPts(vi.price)} :`} />)}
        <p className="roue-hint">La réponse est un bonus : une réponse fausse ne fait pas perdre la main. Gain du tour en attente : <span className="roue-gain">+{fmtPts(gain)}</span>.</p>
      </>)
    case 'TURN_END': {
      const letter = (gs.ROUE_LETTERS || []).slice(-1)[0]
      const why = seg?.KIND === 'bankrupt' ? 'Banqueroute' : seg?.KIND === 'lose_turn' ? 'Tour perdu'
        : letter && letter.HIT === false ? (isVowel(letter.LETTER) ? `Voyelle ${letter.LETTER} absente — payée −${fmtPts(vi.price)}, tour perdu` : `Lettre ${letter.LETTER} absente`) : null
      return (<>
        <h5>7 — Fin du tour</h5>
        <div className="roue-row">
          {ev.GAIN > 0 ? <><span className="roue-gain">+{fmtPts(ev.GAIN)}</span><span>ajoutés à la cagnotte des {team} : <b>{fmtPts((gs.ROUE_TEAM_BANK || {})[team])}</b></span></>
            : <span>{why ? `${why} — aucun gain pour les ${team}` : `Aucun gain pour les ${team}`}</span>}
        </div>
        <div className="roue-row roue-gap"><RoueNextButtons gs={gs} gates={gates} onNext={actions.next} /></div>
      </>)
    }
    case 'SOLVED': {
      const w = gs.ROUE_WINNER
      return w
        ? <div className="roue-won">🎉 Les {w} ont trouvé la phrase{gs.ROUE_AWARDS?.[w] ? ` — +${fmtPts(gs.ROUE_AWARDS[w])} points crédités` : ''}. Cliquez « Stop » pour terminer la manche.</div>
        : <div className="roue-won">Phrase dévoilée sans gagnant. Cliquez « Stop » pour terminer la manche.</div>
    }
    default:
      return null
  }
}

export default function RouePanel({ gameState, actions, error, teamList = [], waitReasons = [] }) {
  const gs = gameState
  const [selected, setSelected] = useState('')
  const [picking, setPicking] = useState(false)
  const [confirmReveal, setConfirmReveal] = useState(false)
  const sub = gs.ROUE_SUBPHASE || ''
  useEffect(() => { setSelected(''); setPicking(false) }, [sub]) // choix remis à zéro à chaque changement de sous-phase
  useEffect(() => { setConfirmReveal(false) }, [sub])
  const q = gs.question || {}

  if (gs.phase === 'PREPARE' || gs.phase === 'READY') {
    return (
      <div className="roue-panel" data-testid="roue-panel">
        <TeamPicker gs={gs} teamList={teamList} actions={actions} waitReasons={waitReasons} />
        <RoueErrorLine error={error} />
      </div>
    )
  }
  if (gs.phase !== 'STARTED' && gs.phase !== 'PAUSED') return null

  const gates = roueGates(gs)
  const current = gs.ROUE_CURRENT_TEAM
  const bank = gs.ROUE_TEAM_BANK || {}
  const paused = gs.phase === 'PAUSED'
  return (
    <div className="roue-panel" data-testid="roue-panel">
      <div className="roue-meta">
        <span className="roue-chip">Mode : <b>{MODE_LABEL[q.ROUE_MODE] || MODE_LABEL.TANT_QUE_JE_GAGNE}</b></span>
        <span className="roue-chip">Gains : <b>{GAINS_LABEL[q.ROUE_GAINS_RULE] || GAINS_LABEL.WINNER_ONLY}</b></span>
        {vowelInfo(gs).enabled && <span className="roue-chip" data-testid="roue-vowel-chip">Voyelles : <b>{fmtPts(vowelInfo(gs).price)}</b> · {TIMING_LABEL[vowelInfo(gs).timing] || TIMING_LABEL.BEFORE}{vowelInfo(gs).gain !== 'NONE' ? ` · ${VGAIN_LABEL[vowelInfo(gs).gain] || ''}` : ''}</span>}
        {q.ROUE_BONUS_COUNT > 0 && <span className="roue-chip">🎁 BONUS : <b>{q.ROUE_BONUS_COUNT} case{q.ROUE_BONUS_COUNT > 1 ? 's' : ''}</b></span>}
        {q.ROUE_PRESENTATION && <span className="roue-chip">Présentation : <b>{q.ROUE_PRESENTATION}</b></span>}
      </div>
      <Solution gs={gs} />
      <p className="roue-hint">Phrase solution (visible ici seulement) — noir : lettres affichées sur la TV ; jaune : cases éclairées ; vert : en cours de retournement.</p>
      <div className="roue-banks-admin" aria-label="Cagnottes de la manche">
        {(gs.ROUE_TEAMS || []).map(n => (
          <span key={n} className={`roue-bank-chip${n === current && sub !== 'SOLVED' ? ' active' : ''}`} style={{ '--c': teamCss(gs.ROUE_CURRENT_TEAM_COLOR && n === current ? gs.ROUE_CURRENT_TEAM_COLOR : null, '#6b7280') }}>
            {n}{n === current && sub !== 'SOLVED' ? ' · a la main' : ''} <b>{fmtPts(bank[n])}</b>
          </span>
        ))}
      </div>
      {sub !== 'SOLVED' && current && <div className="roue-status"><span className="roue-dot" /> Les {current} ont la main{paused ? ' — manche en pause' : ''}</div>}
      {sub !== 'SOLVED' && <RoueSteps step={gs.ROUE_STEP} before={vowelInfo(gs).before} after={vowelInfo(gs).after} />}
      <fieldset className="roue-act" disabled={paused}>
        <Action gs={gs} gates={gates} actions={actions} selected={selected} setSelected={setSelected} picking={picking} setPicking={setPicking} />
      </fieldset>
      <RoueErrorLine error={error} />
      <div className="roue-foot">
        <div className="roue-row">
          <button type="button" className="roue-btn ghost" disabled={paused || !gates.canPass} onClick={actions.pass}>⏭ Passer la main</button>
          {confirmReveal ? (
            <>
              <button type="button" className="roue-btn ghost" onClick={() => { actions.reveal(); setConfirmReveal(false) }}>Confirmer : dévoiler sans gagnant</button>
              <button type="button" className="roue-btn ghost" onClick={() => setConfirmReveal(false)}>Annuler</button>
            </>
          ) : (
            <button type="button" className="roue-btn ghost" disabled={paused || !gates.canReveal} onClick={() => setConfirmReveal(true)}>👁 Dévoiler la phrase (sans gagnant)…</button>
          )}
        </div>
        <span className="roue-hint">Lettres proposées : {(gs.ROUE_LETTERS || []).map(l => (l.HIT ? <span key={l.LETTER}>{l.LETTER} </span> : <s key={l.LETTER}>{l.LETTER} </s>))}</span>
      </div>
    </div>
  )
}
