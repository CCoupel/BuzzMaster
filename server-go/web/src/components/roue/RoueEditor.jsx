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
  ROUE_BONUS_COUNT_INVALID: 'Nombre de cases BONUS invalide (0 à 4, et la roue ne dépasse pas 64 cases).',
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
  return friendly || m[2] || text
}

function RadioCards({ name, options, value, onChange, two }) {
  return (
    <div className={`memory-mode-options roue-options${two ? ' roue-options-two' : ''}`}>
      {options.map(o => (
        <label key={o.value} className="memory-mode-option">
          <input type="radio" name={name} value={o.value} checked={value === o.value} onChange={() => onChange(o.value)} />
          <span className={`roue-layout-thumb ${o.value}`} hidden={name !== 'roueWheelLayout'} aria-hidden="true" />
          <span className="memory-mode-label">
            <strong>{o.label}</strong>
            <small>{o.desc}</small>
          </span>
        </label>
      ))}
    </div>
  )
}

/** Cases BONUS (v12.1.0, #308, maquette C) : compteur 0 à 4 + aperçu de la roue générée par le serveur (`POST /api/roue/wheel-preview`
 * avec `bonus_count`, section `wheel` lue sur GET /game-config.json). Aucune règle de placement n'est copiée ici.
 * L'aperçu n'est demandé que si le compteur est > 0 ou a été touché (aucun appel réseau pour une question sans BONUS). */
function BonusSection({ count, onChange }) {
  const [active, setActive] = useState(count > 0)
  const [wheel, setWheel] = useState(null) // { SEGMENTS, COUNT }
  const [error, setError] = useState('')
  const seq = useRef(0)
  const show = active || count > 0
  // Composant resté monté sur une autre question à 0 BONUS : pas d'aperçu inutile.
  useEffect(() => { if (count === 0) setActive(false) }, [count])
  useEffect(() => {
    if (!show) return undefined
    const id = ++seq.current
    const ctrl = typeof AbortController !== 'undefined' ? new AbortController() : null
    const timer = setTimeout(async () => {
      try {
        const cfgRes = await fetch('/game-config.json', { signal: ctrl?.signal })
        const cfg = cfgRes.ok ? await cfgRes.json() : null
        const w = cfg?.wheel
        if (!w) throw new Error('config')
        const res = await fetch('/api/roue/wheel-preview', {
          method: 'POST', headers: { 'Content-Type': 'application/json' }, signal: ctrl?.signal,
          body: JSON.stringify({ max_points: w.max_points, step: w.step, bankrupt_count: w.bankrupt_count, lose_turn_count: w.lose_turn_count, bonus_count: count }),
        })
        if (id !== seq.current) return
        if (res.ok) { setWheel(await res.json()); setError('') } else {
          let body = null
          try { body = await res.json() } catch { /* corps non JSON */ }
          setWheel(null)
          setError(body?.MESSAGE || FALLBACK_ERRORS[body?.CODE] || 'Aperçu de la roue refusé.')
        }
      } catch (e) {
        if (e?.name === 'AbortError' || id !== seq.current) return
        setWheel(null)
        setError('Aperçu de la roue indisponible.')
      }
    }, PREVIEW_DELAY_MS)
    return () => { clearTimeout(timer); ctrl?.abort() }
  }, [show, count])
  const set = (v) => { setActive(true); onChange(Math.max(0, Math.min(ROUE_BONUS_MAX, v))) }
  return (
    <div className="memory-mode-selector roue-bonus-section">
      <div className="roue-bonus-line">
        <label htmlFor="roue-bonus-count">🎁 Cases BONUS</label>
        <span className="roue-stepper" role="group" aria-label="Nombre de cases BONUS">
          <button type="button" aria-label="Moins de cases BONUS" disabled={count <= 0} onClick={() => set(count - 1)}>−</button>
          <output id="roue-bonus-count" data-testid="roue-bonus-count">{count}</output>
          <button type="button" aria-label="Plus de cases BONUS" disabled={count >= ROUE_BONUS_MAX} onClick={() => set(count + 1)}>+</button>
        </span>
        <small className="roue-hint">0 à 4 — ajoutées aux cases des Paramètres. Arrêt sur BONUS : aucun point, l'équipe rejoue ; le cadeau est géré par l'animateur.</small>
      </div>
      {show && (error
        ? <p className="roue-hint err" role="alert">✗ {error}</p>
        : wheel && (
          <>
            <div className="roue-bonus-preview" data-testid="roue-bonus-preview"><FortuneWheel wheel={{ SEGMENTS: wheel.SEGMENTS, LAYOUT: 'strip', CURRENT: -1 }} subphase="" size="compact" /></div>
            <p className="roue-hint">Aperçu de la roue générée ({wheel.COUNT ?? wheel.SEGMENTS?.length} cases) — calculé par le serveur.</p>
          </>
        ))}
    </div>
  )
}

export default function RoueEditor({ values, onChange, onValidityChange }) {
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
        <p className="roue-hint">Présentation de la roue sur la TV pour cette question. Les cases de la roue se règlent dans les Paramètres.</p>
      </div>

      <BonusSection count={Number.isInteger(values.roueBonusCount) ? values.roueBonusCount : 0} onChange={(v) => onChange('roueBonusCount', v)} />

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
