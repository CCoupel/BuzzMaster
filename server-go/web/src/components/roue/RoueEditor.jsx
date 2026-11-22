/**
 * RoueEditor — section d'édition d'une question de type ROUE (v12.0.0, #145,
 * maquette M3 « A — Formulaire puis aperçu », contrat roue.md §3 / §11).
 *
 * Composant contrôlé : l'état vit dans le formulaire de QuestionsPage
 * (`values`, `onChange(champ, valeur)`). La catégorie, le son et les boutons
 * restent ceux du formulaire existant. L'aperçu du plateau vient du serveur
 * (`POST /api/roue/preview`) : aucune règle de mise en lignes n'est copiée ici.
 * `onValidityChange(bool)` signale si la phrase est enregistrable.
 */
import { useEffect, useRef, useState } from 'react'
import RoueBoard from './RoueBoard'
import FortuneWheel from './FortuneWheel'
import './RoueEditor.css'

export const ROUE_PRESENTATION_MAX = 80
const PREVIEW_DELAY_MS = 300
export const ROUE_BONUS_MAX = 4 // contrat roue.md §15.2.1 : 0 à 4 cases BONUS

const MODES = [
  { value: 'SOLO', label: 'SOLO', desc: 'Une équipe joue seule' },
  { value: 'CHACUN_SON_TOUR', label: 'CHACUN SON TOUR', desc: 'La main passe à la fin de chaque tour' },
  { value: 'TANT_QUE_JE_GAGNE', label: 'TANT QUE JE GAGNE', desc: 'Garde la main tant que la lettre est trouvée' },
]
const GAINS = [
  { value: 'WINNER_ONLY', label: 'Seul le gagnant encaisse', desc: "L'équipe qui trouve la phrase reçoit sa cagnotte ; les autres perdent la leur" },
  { value: 'ALL_TEAMS', label: 'Tout le monde encaisse', desc: 'Chaque équipe garde la cagnotte gagnée pendant la manche' },
]
const TIMINGS = [
  { value: 'BEFORE', label: 'Avant de tourner', desc: "À l'étape 1, à côté de « Lancer la roue »" },
  { value: 'AFTER', label: "Après l'arrêt de la roue", desc: "À la place de la consonne : la case tirée n'est pas jouée" },
  { value: 'BOTH', label: 'Les deux', desc: "Avant de tourner et après l'arrêt" },
]
const COUNTS = [
  { value: 'ONE', label: '1 voyelle', desc: 'Une seule voyelle à ce moment' },
  { value: 'MANY', label: 'Plusieurs voyelles', desc: "D'affilée, tant que la cagnotte le permet et que chaque voyelle est présente" },
]
const VGAINS = [
  { value: 'NONE', label: 'Sans gain', desc: 'La voyelle présente ne rapporte rien' },
  { value: 'REFUND', label: 'Mise remboursée', desc: 'La voyelle présente rend son prix' },
  { value: 'PER_OCCURRENCE', label: 'Prix × nombre de lettres', desc: 'Ex. 3 E à 250 → +750' },
]
export const VOWEL_PRICE_MAX = 1000000
export const validVowelPrice = (v) => /^\d+$/.test(String(v ?? '').trim()) && Number(v) >= 1 && Number(v) <= VOWEL_PRICE_MAX
const LAYOUTS = [
  { value: 'strip', label: 'Bandeau', desc: "Bande horizontale en bas de l'écran ; plateau le plus grand" },
  { value: 'arc', label: 'Arc', desc: "Haut d'une grande roue qui émerge du bas de l'écran" },
]

// Messages lisibles des refus ROUE_* (le serveur envoie aussi un MESSAGE, prioritaire).
const FALLBACK_ERRORS = {
  ROUE_PHRASE_EMPTY: 'La phrase est vide.',
  ROUE_PHRASE_INVALID_CHAR: 'La phrase contient un caractère non admis.',
  ROUE_PHRASE_WORD_TOO_LONG: 'Un mot dépasse 14 caractères.',
  ROUE_PHRASE_TOO_LONG: 'Phrase trop longue : 4 lignes au plus (14 caractères par ligne, mots non coupés).',
  ROUE_VOWEL_TIMING_INVALID: "Moment d'achat des voyelles invalide.",
  ROUE_VOWELS_BEFORE_INVALID: 'Nombre de voyelles avant de tourner invalide.',
  ROUE_VOWELS_AFTER_INVALID: "Nombre de voyelles après l'arrêt invalide.",
  ROUE_VOWEL_PRICE_INVALID: "Prix d'une voyelle invalide (entier de 1 à 1 000 000).",
  ROUE_VOWEL_GAIN_INVALID: "Gain d'une voyelle invalide.",
  ROUE_WHEEL_INVALID: 'Composition de la roue invalide.',
  ROUE_BONUS_COUNT_INVALID: 'Trop de BONUS pour cette roue (0 à 4 ; un BONUS ne touche jamais une Banqueroute, un Perdu ni un autre BONUS).',
  ROUE_BONUS_MODE_INVALID: 'Mode du BONUS invalide (éphémère ou permanent).',
}

/** Refus 400 de POST /questions (texte « ROUE_CODE: détail ») → message lisible ; autre texte inchangé. */
export function roueRefusalText(text) {
  const m = /^(ROUE_[A-Z_]+)\s*:?\s*(.*)$/s.exec(String(text || '').trim())
  if (!m) return text
  const friendly = {
    ROUE_PRESENTATION_CONTAINS_PHRASE: 'La phrase de présentation contient la phrase à deviner : elle serait lisible sur la TV.',
    ROUE_PRESENTATION_TOO_LONG: `La phrase de présentation dépasse ${ROUE_PRESENTATION_MAX} caractères.`,
    ...FALLBACK_ERRORS,
  }[m[1]]
  // Composition de la roue (#309) : le message du serveur (cases, pas, maximum) est plus précis que le texte générique.
  if (m[1] === 'ROUE_WHEEL_INVALID' && m[2]) return m[2]
  return friendly || m[2] || text
}

function RadioCards({ name, options, value, onChange, two, disabled = false }) {
  return (
    <div className={`memory-mode-options roue-options${two ? ' roue-options-two' : ''}${disabled ? ' roue-dim' : ''}`}>
      {options.map(o => (
        <label key={o.value} className="memory-mode-option">
          <input type="radio" name={name} value={o.value} checked={value === o.value} disabled={disabled} onChange={() => onChange(o.value)} />
          <span className="memory-mode-label">
            <strong>{o.label}</strong>
            <small>{o.desc}</small>
          </span>
        </label>
      ))}
    </div>
  )
}

/** Section Voyelles (v12.1.0, #147, maquette éditeur retenue) : interrupteur, moment, deux réglages « 1 / plusieurs »
 * (grisés quand le moment ne les utilise pas), prix, gain avec exemple. Valeurs par défaut d'une question v12.0 : voyelles
 * autorisées, avant de tourner, plusieurs / plusieurs, 250, sans gain. */
function VowelsSection({ values, onChange }) {
  const on = values.roueBuyVowels !== false
  const timing = values.roueVowelTiming || 'BEFORE'
  const before = values.roueVowelsBefore || 'MANY'
  const after = values.roueVowelsAfter || 'MANY'
  const gain = values.roueVowelGain || 'NONE'
  const priceRaw = values.roueVowelPrice ?? 250
  const priceOk = validVowelPrice(priceRaw)
  const p = priceOk ? Number(priceRaw) : 250
  const ex = gain === 'NONE' ? 'aucun gain' : gain === 'REFUND' ? `remboursé +${p}` : `gagné +${3 * p}`
  return (
    <div className="roue-vowels-section" data-testid="roue-vowels-section">
      <div className="rafale-alpha-option">
        <button type="button" role="switch" aria-checked={on ? 'true' : 'false'} aria-label="Autoriser l'achat de voyelles"
          className={`rafale-alpha-switch${on ? ' is-on' : ''}`} onClick={() => onChange('roueBuyVowels', !on)}>
          <span className="rafale-alpha-switch-track"><span className="rafale-alpha-switch-thumb" /></span>
          <span className="rafale-alpha-switch-label">🔤 Autoriser l'achat de voyelles</span>
        </button>
        <small className="rafale-alpha-hint">L'équipe paie une voyelle avec sa cagnotte de manche. Voyelle absente : payée, tour perdu.</small>
      </div>
      <div className={on ? '' : 'roue-dim'} data-testid="roue-vowels-body">
        <div className="memory-mode-selector">
          <label>Moment de l'achat</label>
          <RadioCards name="roueVowelTiming" options={TIMINGS} value={timing} disabled={!on} onChange={(v) => onChange('roueVowelTiming', v)} />
        </div>
        <div className="roue-vowels-counts">
          <div className={timing === 'AFTER' ? 'roue-dim' : ''} data-testid="roue-vowels-before">
            <label>Avant de tourner : combien de voyelles par tour ?</label>
            <RadioCards name="roueVowelsBefore" options={COUNTS} value={before} two disabled={!on || timing === 'AFTER'} onChange={(v) => onChange('roueVowelsBefore', v)} />
          </div>
          <div className={timing === 'BEFORE' ? 'roue-dim' : ''} data-testid="roue-vowels-after">
            <label>Après l'arrêt : combien de voyelles par arrêt ?</label>
            <RadioCards name="roueVowelsAfter" options={COUNTS} value={after} two disabled={!on || timing === 'BEFORE'} onChange={(v) => onChange('roueVowelsAfter', v)} />
          </div>
        </div>
        <div className="form-group">
          <label htmlFor="roue-vowel-price">Prix d'une voyelle</label>
          <input id="roue-vowel-price" type="number" min="1" max={VOWEL_PRICE_MAX} step="1" value={priceRaw} disabled={!on}
            aria-invalid={priceOk ? 'false' : 'true'} onChange={(e) => onChange('roueVowelPrice', e.target.value)} />
          <p className={`roue-hint${priceOk ? '' : ' err'}`} role={priceOk ? undefined : 'alert'}>
            {priceOk ? 'Points de cagnotte (1 à 1 000 000).' : '✗ Un entier de 1 à 1 000 000.'}
          </p>
        </div>
        <div className="memory-mode-selector">
          <label>Gain d'une voyelle présente</label>
          <RadioCards name="roueVowelGain" options={VGAINS} value={gain} disabled={!on} onChange={(v) => onChange('roueVowelGain', v)} />
          <p className="roue-hint" data-testid="roue-vowel-example">Exemple : 3 E à {p} → payé −{p}, {ex}{timing === 'AFTER' ? " ; la case tirée n'est pas jouée" : ''}</p>
        </div>
      </div>
    </div>
  )
}

// Composition de la roue par question (v12.1.0, #309, contrat roue.md §16, maquette retenue B « champ lié au global, à délier »).
// Règle : valeur de la question = paramètre personnalisé ; `null` (ou vide) = suit le réglage global (champ retiré à l'enregistrement).
const WHEEL_PARAMS = [
  { id: 'max', field: 'roueWheelMaxPoints', global: 'max_points', label: 'Points maximum', hint: 'meilleure case', min: 2, group: 'Points' },
  { id: 'step', field: 'roueWheelStep', global: 'step', label: 'Pas', hint: 'écart et plus petite valeur', min: 1, group: 'Points' },
  { id: 'B', field: 'roueWheelBankruptCount', global: 'bankrupt_count', label: '💸 Banqueroute', hint: 'vident la cagnotte', min: 0, group: 'Cases spéciales' },
  { id: 'P', field: 'roueWheelLoseTurnCount', global: 'lose_turn_count', label: '⏭ Perdu', hint: 'font passer le tour', min: 0, group: 'Cases spéciales' },
  { id: 'X', field: 'roueBonusCount', global: 'bonus_count', label: '🎁 BONUS', hint: "0 à 4, posés sur des cases à points", min: 0, max: ROUE_BONUS_MAX, group: 'Cases spéciales' },
]
const WHEEL_FIELDS = WHEEL_PARAMS.map(p => p.field)
const isCustom = (v) => v !== null && v !== undefined && v !== ''
const globalOf = (g, p) => (p.id === 'X' ? (Number.isInteger(g?.[p.global]) ? g[p.global] : 0) : g?.[p.global])

function WheelCompositionSection({ values, onChange, onValidityChange }) {
  const [globalWheel, setGlobalWheel] = useState(null) // section `wheel` de GET /game-config.json
  const [preview, setPreview] = useState(null) // réponse de wheel-preview
  const [error, setError] = useState('')
  const touched = useRef(false)
  const seq = useRef(0)
  const [draw, setDraw] = useState(0) // « 🎲 Autre tirage » : nouvel aperçu, marques BONUS placées ailleurs (le serveur tire)
  const validityRef = useRef(onValidityChange)
  validityRef.current = onValidityChange

  useEffect(() => {
    const ctrl = typeof AbortController !== 'undefined' ? new AbortController() : null
    ;(async () => {
      try {
        const res = await fetch('/game-config.json', { signal: ctrl?.signal })
        const cfg = res.ok ? await res.json() : null
        if (cfg?.wheel) setGlobalWheel(cfg.wheel)
        else setError('Réglages globaux de la roue indisponibles.')
      } catch (e) {
        if (e?.name !== 'AbortError') setError('Réglages globaux de la roue indisponibles.')
      }
    })()
    return () => ctrl?.abort()
  }, [])

  const eff = {}
  WHEEL_PARAMS.forEach(p => { eff[p.id] = isCustom(values[p.field]) ? Number(values[p.field]) : globalOf(globalWheel, p) })
  const globalMode = globalWheel?.bonus_mode === 'PERMANENT' ? 'PERMANENT' : 'EPHEMERAL'
  const modeCustom = values.roueBonusMode === 'PERMANENT' || values.roueBonusMode === 'EPHEMERAL'
  const effMode = modeCustom ? values.roueBonusMode : globalMode
  const effKey = globalWheel ? JSON.stringify({ ...eff, draw }) : ''

  useEffect(() => {
    if (!globalWheel) { validityRef.current?.(true); return undefined }
    const id = ++seq.current
    const ctrl = typeof AbortController !== 'undefined' ? new AbortController() : null
    const timer = setTimeout(async () => {
      try {
        const res = await fetch('/api/roue/wheel-preview', {
          method: 'POST', headers: { 'Content-Type': 'application/json' }, signal: ctrl?.signal,
          body: JSON.stringify({ max_points: eff.max, step: eff.step, bankrupt_count: eff.B, lose_turn_count: eff.P, bonus_count: eff.X }),
        })
        if (id !== seq.current) return
        if (res.ok) { setPreview(await res.json()); setError(''); validityRef.current?.(true) } else {
          let body = null
          try { body = await res.json() } catch { /* corps non JSON */ }
          setPreview(null)
          setError(body?.MESSAGE || FALLBACK_ERRORS[body?.CODE] || 'Roue refusée.')
          validityRef.current?.(false)
        }
      } catch (e) {
        if (e?.name === 'AbortError' || id !== seq.current) return
        setPreview(null)
        setError('Aperçu de la roue indisponible.')
        validityRef.current?.(false)
      }
    }, PREVIEW_DELAY_MS)
    return () => { clearTimeout(timer); ctrl?.abort() }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [effKey])

  const toggleMode = () => { touched.current = true; onChange('roueBonusMode', modeCustom ? null : globalMode) }
  const editMode = (once) => { touched.current = true; onChange('roueBonusMode', once ? 'EPHEMERAL' : 'PERMANENT') }
  const toggle = (p) => { touched.current = true; onChange(p.field, isCustom(values[p.field]) ? null : globalOf(globalWheel, p)) }
  const edit = (p, raw) => {
    touched.current = true
    if (raw === '') { onChange(p.field, p.min); return }
    const n = Math.max(p.min, Math.trunc(Number(raw)))
    onChange(p.field, Number.isFinite(n) ? (p.max != null ? Math.min(p.max, n) : n) : p.min)
  }
  const custom = WHEEL_PARAMS.filter(p => isCustom(values[p.field]))
  // Question v12.1 : ROUE_BONUS_COUNT sans mode, à l'ouverture (§17.7) — ses BONUS sont désormais posés sur des cases à points.
  const migrated = !touched.current && isCustom(values.roueBonusCount) && Number(values.roueBonusCount) > 0 && !modeCustom
  const stale = !!error && !touched.current && custom.length > 0
  const segs = preview?.SEGMENTS || []
  const vals = segs.filter(x => x.KIND === 'value').map(x => x.VALUE)

  const param = (p) => {
    const c = isCustom(values[p.field])
    const g = globalOf(globalWheel, p)
    return (
      <div key={p.id} className={`roue-wparam${c ? ' c' : ''}`} data-testid={`roue-wparam-${p.id}`}>
        <label htmlFor={`roue-w-${p.id}`}>{p.label}</label>
        <div className="roue-wlock">
          <input id={`roue-w-${p.id}`} type="number" min={p.min} max={p.max} step="1" disabled={!c || !globalWheel}
            value={c ? values[p.field] : (g ?? '')} onChange={(e) => edit(p, e.target.value)} />
          <button type="button" disabled={!globalWheel} aria-label={c ? `Revenir au global : ${p.label}` : `Personnaliser : ${p.label}`}
            title={c ? 'Revenir au global' : 'Personnaliser'} onClick={() => toggle(p)}>{c ? '🔗' : '✏️'}</button>
        </div>
        <span className="roue-wtag">{c ? 'personnalisé' : '🔗 global'}</span>
        <small>{c ? `global : ${g ?? '…'} — 🔗 pour y revenir` : p.hint}</small>
        {p.id === 'X' && (
          <div className={`roue-wmode${modeCustom ? ' c' : ''}${(eff.X ?? 0) === 0 ? ' off' : ''}`} data-testid="roue-wmode">
            <label htmlFor="roue-w-mode">
              <input id="roue-w-mode" type="checkbox" role="switch" checked={effMode !== 'PERMANENT'} disabled={!modeCustom || !globalWheel || (eff.X ?? 0) === 0}
                onChange={(e) => editMode(e.target.checked)} />
              <span>une seule fois par manche</span>
            </label>
            <button type="button" disabled={!globalWheel || (eff.X ?? 0) === 0} aria-label={modeCustom ? 'Revenir au global : mode du BONUS' : 'Personnaliser : mode du BONUS'}
              title={modeCustom ? 'Revenir au global' : 'Personnaliser'} onClick={toggleMode}>{modeCustom ? '🔗' : '✏️'}</button>
            <span className="roue-wtag">{modeCustom ? 'personnalisé' : '🔗 global'}</span>
          </div>
        )}
      </div>
    )
  }

  return (
    <div className="memory-mode-selector roue-wheel-section" data-testid="roue-wheel-section">
      <h4 className="roue-wheel-title">🎡 Roue — composition</h4>
      <p className="roue-hint">Valeurs des Paramètres par défaut ; personnalisez seulement ce qui change pour cette question. Les durées de la roue restent celles des Paramètres.</p>
      <div className="roue-wgroups">
        {['Points', 'Cases spéciales'].map(g => (
          <fieldset key={g} className="roue-wgroup"><legend>{g}</legend>
            <div className={`roue-wrow roue-wrow-${g === 'Points' ? 2 : 3}`}>{WHEEL_PARAMS.filter(p => p.group === g).map(param)}</div>
          </fieldset>
        ))}
      </div>
      {migrated && <div className="roue-wfix" role="status" data-testid="roue-bonus-migration">ℹ Question créée avant la v12.2 : ses {values.roueBonusCount} BONUS sont désormais posés sur des cases à points — la roue compte {values.roueBonusCount} case{values.roueBonusCount > 1 ? 's' : ''} de moins. Mode : {globalMode === 'PERMANENT' ? 'permanent' : 'éphémère'} (réglage global).</div>}
      <div className="roue-wstatus" data-testid="roue-wheel-status">
        {custom.length
          ? <>Personnalisé pour cette question : <b>{custom.map(p => p.label.replace(/^\S+ /, '')).join(', ')}</b> — le reste suit les Paramètres.</>
          : 'Tous les paramètres suivent les Paramètres (roue globale).'}
      </div>
      {error
        ? <p className="roue-hint err" role="alert">✗ {stale ? "Cette roue n'est plus valable avec les réglages globaux actuels : " : 'Roue refusée : '}{error}{stale ? " En jeu, la roue globale sera utilisée tant que la question n'est pas corrigée." : ' La question ne peut pas être enregistrée.'}</p>
        : preview && (
          <>
            {preview.step_adjusted_from != null && preview.step !== preview.step_adjusted_from && (
              <div className="roue-wfix" role="status">ℹ Pas ajusté à <b>{preview.step}</b> : {preview.step_adjusted_from} ne divise pas {eff.max} ; {preview.step} est le diviseur le plus proche.</div>
            )}
            <div className="roue-hint" data-testid="roue-wheel-summary">
              <b>{preview.COUNT ?? segs.length} cases</b>{vals.length ? <> : {vals.length} gagnantes de {Math.min(...vals)} à {Math.max(...vals)} par pas de {preview.step}</> : null} · {eff.B} Banqueroute · {eff.P} Perdu · {eff.X} BONUS
            </div>
            <div className="roue-bonus-preview" data-testid="roue-bonus-preview"><FortuneWheel wheel={{ SEGMENTS: segs, LAYOUT: 'strip', CURRENT: -1 }} subphase="" size="compact" /></div>
            <p className="roue-hint">Aperçu de la roue de cette question — un tirage calculé par le serveur ; les BONUS changent de place à chaque manche.
              {(eff.X ?? 0) > 0 && <button type="button" className="roue-redraw" data-testid="roue-redraw" onClick={() => setDraw(d => d + 1)}>🎲 Autre tirage</button>}</p>
          </>
        )}
    </div>
  )
}

export default function RoueEditor({ values, onChange, onValidityChange, onWheelValidityChange }) {
  const { roueMode, roueGainsRule, roueWheelLayout, roueShowLetters } = values
  const phrase = values.rouePhrase || ''
  const presentation = values.rouePresentation || ''
  const [preview, setPreview] = useState(null) // { ROWS, CONSONANTS, LETTERS, COLS, MAX_ROWS }
  const [error, setError] = useState(null) // { CODE, MESSAGE }
  const [loading, setLoading] = useState(false)
  const validityRef = useRef(onValidityChange)
  validityRef.current = onValidityChange

  useEffect(() => {
    const trimmed = phrase.trim()
    if (!trimmed) {
      setPreview(null)
      setError(null)
      setLoading(false)
      validityRef.current?.(false)
      return undefined
    }
    validityRef.current?.(false) // non enregistrable tant que le serveur n'a pas validé
    setLoading(true)
    const ctrl = typeof AbortController !== 'undefined' ? new AbortController() : null
    const timer = setTimeout(async () => {
      try {
        const res = await fetch('/api/roue/preview', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ PHRASE: phrase }),
          signal: ctrl?.signal,
        })
        if (res.ok) {
          const body = await res.json()
          setPreview(body)
          setError(null)
          validityRef.current?.(true)
        } else {
          let body = null
          try { body = await res.json() } catch { /* corps non JSON */ }
          setPreview(null)
          setError({ CODE: body?.CODE || 'ROUE_PHRASE_INVALID', MESSAGE: body?.MESSAGE || '' })
          validityRef.current?.(false)
        }
      } catch (e) {
        if (e?.name === 'AbortError') return
        setPreview(null)
        setError({ CODE: 'ROUE_PREVIEW_UNAVAILABLE', MESSAGE: "Aperçu indisponible (serveur injoignable)." })
        validityRef.current?.(false)
      } finally {
        if (!ctrl?.signal.aborted) setLoading(false)
      }
    }, PREVIEW_DELAY_MS)
    return () => {
      clearTimeout(timer)
      ctrl?.abort()
    }
  }, [phrase])

  const errText = error ? (error.MESSAGE || FALLBACK_ERRORS[error.CODE] || error.CODE) : ''
  const rowsCount = preview?.ROWS?.length ?? 0

  return (
    <div className="roue-editor rafale-section">
      <div className="form-group">
        <label htmlFor="roue-phrase-input">Phrase à deviner *</label>
        <input
          id="roue-phrase-input"
          type="text"
          value={phrase}
          className={error ? 'roue-input-error' : ''}
          aria-invalid={error ? 'true' : 'false'}
          aria-describedby="roue-phrase-hint"
          onChange={(e) => onChange('rouePhrase', e.target.value)}
          placeholder="Ex. Combien pour ce chien dans la vitrine"
          autoComplete="off"
        />
        <p id="roue-phrase-hint" className={`roue-hint${error ? ' err' : preview ? ' ok' : ''}`} role={error ? 'alert' : undefined}>
          {error
            ? `✗ ${errText}`
            : preview
              ? `✓ ${rowsCount} ligne${rowsCount > 1 ? 's' : ''} sur ${preview.MAX_ROWS} · ${preview.CONSONANTS} consonnes différentes · ${preview.LETTERS} lettres différentes`
              : loading ? 'Vérification…' : '14 caractères par ligne, 4 lignes au plus, mots non coupés.'}
        </p>
        <RoueBoard rows={preview?.ROWS || []} cols={preview?.COLS || 14} maxRows={preview?.MAX_ROWS || 4} />
      </div>

      <div className="form-group">
        <label htmlFor="roue-presentation-input">Phrase de présentation</label>
        <input
          id="roue-presentation-input"
          type="text"
          value={presentation}
          maxLength={ROUE_PRESENTATION_MAX}
          onChange={(e) => onChange('rouePresentation', e.target.value)}
        />
        <p className="roue-hint">
          Facultative, {ROUE_PRESENTATION_MAX} caractères au plus ({presentation.length} / {ROUE_PRESENTATION_MAX}). Indice affiché sur la TV à côté de la catégorie.
        </p>
      </div>

      <div className="memory-mode-selector">
        <label>Mode de jeu</label>
        <RadioCards name="roueMode" options={MODES} value={roueMode} onChange={(v) => onChange('roueMode', v)} />
      </div>

      <div className="memory-mode-selector">
        <label>Gains de la manche</label>
        <RadioCards name="roueGainsRule" options={GAINS} value={roueGainsRule} onChange={(v) => onChange('roueGainsRule', v)} two />
      </div>

      <div className="memory-mode-selector">
        <label>Forme de la roue</label>
        <RadioCards name="roueWheelLayout" options={LAYOUTS} value={roueWheelLayout} onChange={(v) => onChange('roueWheelLayout', v)} two />
        <p className="roue-hint">Présentation de la roue sur la TV pour cette question. Les cases de la roue se règlent plus bas, dans « Roue — composition » (valeurs des Paramètres par défaut).</p>
      </div>

      <VowelsSection values={values} onChange={onChange} />

      <WheelCompositionSection values={values} onChange={onChange} onValidityChange={onWheelValidityChange} />

      <div className="rafale-alpha-option">
        <button
          type="button"
          role="switch"
          aria-checked={roueShowLetters ? 'true' : 'false'}
          aria-label="Afficher les lettres déjà proposées"
          className={`rafale-alpha-switch${roueShowLetters ? ' is-on' : ''}`}
          onClick={() => onChange('roueShowLetters', !roueShowLetters)}
        >
          <span className="rafale-alpha-switch-track"><span className="rafale-alpha-switch-thumb" /></span>
          <span className="rafale-alpha-switch-label">Afficher sur la TV les lettres déjà proposées</span>
        </button>
        <small className="rafale-alpha-hint">
          Activé : une ligne « Lettres proposées » sous le plateau, lettres absentes barrées. Désactivé : aux joueurs de s'en souvenir.
        </small>
      </div>
    </div>
  )
}
