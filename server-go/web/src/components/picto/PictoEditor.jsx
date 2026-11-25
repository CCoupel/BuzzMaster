/**
 * PictoEditor — section d'édition d'une question de type PICTO (v13.0.0, #191,
 * maquette M5 retenue C « grille compacte + résumé », contrat picto.md §3.1/§4/§11).
 *
 * Composant contrôlé : l'état vit dans le formulaire de QuestionsPage
 * (`values`, `onChange(champ, valeur)`). La catégorie unique reste le
 * CategoryPicker du formulaire. Les règles d'un mot (format, doublon replié)
 * n'existent que côté serveur : l'éditeur n'en copie aucune et affiche le refus
 * renvoyé (`error`). Seules les bornes de saisie (liste 1 à 20) servent à
 * désactiver l'ajout / l'enregistrement.
 */
import { useEffect, useRef, useState } from 'react'
import { categoryMeta } from '../../utils/categoryUtils'
import PictoWordsSource, { SOURCE_POOL, isPictoFilterValid, PICTO_KEYS } from './PictoWordsSource'
import './PictoEditor.css'

export const PICTO_WORDS_MAX = 20

// Jeux de champs du formulaire par type de la famille Picto (picto.md §17.2) : `Picto` (un mot) et `Rafale Picto`
// (préfixe RAFALE_PICTO_*, plus le bloc Passes). Le sous-éditeur PictoWordsSource est paramétré par ces noms.
export const PICTO_FIELDS = { ...PICTO_KEYS, words: 'pictoWords', rafale: false }
export const RAFALE_PICTO_FIELDS = {
  source: 'rafalePictoSource', categories: 'rafalePictoCategories', difficulties: 'rafalePictoDifficulties',
  words: 'rafalePictoWords', passCost: 'rafalePictoPassCost', passLimit: 'rafalePictoPassLimit', poolEnd: 'rafalePictoPoolEnd',
  rafale: true,
}
export const PASS_COST_MAX = 100
export const validPassCost = (v) => /^\d+$/.test(String(v ?? '').trim()) && Number(v) >= 0 && Number(v) <= PASS_COST_MAX
export const PICTO_TIME_DEFAULT = '90'
export const PICTO_TIME_MIN = 30
export const PICTO_TIME_MAX = 300

// Espaces de début/fin retirés, espaces internes compactés (la validation reste serveur).
export const normalizePictoWord = (w) => String(w ?? '').trim().replace(/\s+/g, ' ')

// Texte d'un refus PICTO_* : le message du serveur est prioritaire, sinon un libellé lisible.
const FALLBACK_ERRORS = {
  PICTO_TIME_INVALID: `Durée invalide : ${PICTO_TIME_MIN} à ${PICTO_TIME_MAX} secondes.`,
  PICTO_WORDS_INVALID: 'Mots refusés : 1 à 20 mots valables, sans doublon.',
}
export function pictoRefusalText(raw) {
  const text = String(raw ?? '').trim()
  if (!text) return ''
  const code = Object.keys(FALLBACK_ERRORS).find((c) => text.includes(c))
  if (!code) return text
  const rest = text.replace(code, '').replace(/^[\s:—-]+/, '')
  return rest || FALLBACK_ERRORS[code]
}

export const validPictoTime = (t) => /^\d+$/.test(String(t ?? '').trim()) && Number(t) >= PICTO_TIME_MIN && Number(t) <= PICTO_TIME_MAX

export function isPictoReady(values, fields = PICTO_FIELDS) {
  if (!validPictoTime(values.time)) return false
  // Rafale Picto : coût de passe 0 à 100 (picto.md §17.2)
  if (fields.rafale && !validPassCost(values[fields.passCost] ?? 0)) return false
  // Source « Pool Picto » (v13.1.0) : un filtre catégories × difficultés valide remplace la liste de mots.
  if (values[fields.source] === SOURCE_POOL) return isPictoFilterValid(values[fields.categories], values[fields.difficulties])
  const n = (values[fields.words] || []).length
  return n >= 1 && n <= PICTO_WORDS_MAX
}

export default function PictoEditor({ values, onChange, error, customCategories = [], fields = PICTO_FIELDS }) {
  const rafale = fields.rafale === true
  const [draft, setDraft] = useState('')
  const [ignored, setIgnored] = useState(0) // mots non ajoutés car au-delà de 20 (M3)
  const words = values[fields.words] || []
  // Liste émise par cet éditeur : toute autre liste (autre question chargée, formulaire remis à zéro) efface le message.
  const emitted = useRef(words)
  useEffect(() => { if (words !== emitted.current) setIgnored(0) }, [words])
  const full = words.length >= PICTO_WORDS_MAX
  const fromPool = values[fields.source] === SOURCE_POOL
  const timeOk = validPictoTime(values.time)
  const cat = values.category ? categoryMeta(values.category, customCategories) : null

  // Ajoute un ou plusieurs mots (un par ligne ou séparés par virgule/point-virgule) ; doublons : refus serveur.
  const addWords = (raw) => {
    const incoming = String(raw ?? '').split(/[\n,;]+/).map(normalizePictoWord).filter(Boolean)
    if (!incoming.length) return
    const all = [...words, ...incoming]
    const next = all.slice(0, PICTO_WORDS_MAX)
    emitted.current = next
    onChange(fields.words, next)
    setIgnored(Math.max(0, all.length - PICTO_WORDS_MAX))
    setDraft('')
  }
  const removeWord = (i) => { setIgnored(0); const next = words.filter((_, k) => k !== i); emitted.current = next; onChange(fields.words, next) }

  const onKeyDown = (e) => {
    if (e.key === 'Enter' || e.key === ',' || e.key === ';') {
      e.preventDefault()
      addWords(draft)
    } else if (e.key === 'Backspace' && !draft && words.length) {
      removeWord(words.length - 1)
    }
  }

  return (
    <div className="picto-editor">
      <div className="picto-intro">
        <span className="picto-pill">{rafale ? '🖍️ Rafale Picto' : '✏️ Picto'}</span>
        <span className="picto-muted">{rafale
          ? 'Une équipe fait deviner le plus de mots possible dans le temps imparti ; un joueur les dessine.'
          : 'Un mot à faire deviner par une équipe ; un joueur le dessine.'}</span>
      </div>

      <div className="picto-summary" aria-label="Résumé de la question">
        <span className="picto-chip">⏱ {timeOk ? `${values.time} s` : '— s'}</span>
        <span className="picto-chip">🏆 {values.points || '—'} pts{rafale ? ' / mot' : ''}</span>
        {rafale && (
          <span className="picto-chip" data-testid="picto-pass-summary">
            ⏭ passe −{values[fields.passCost] || 0} pts · {Number(values[fields.passLimit]) === 1 ? '1 passe' : 'illimitées'}
          </span>
        )}
        {rafale && <span className="picto-chip">{values[fields.poolEnd] === 'LOOP' ? '🔁 reboucle' : '⏹ fin à la dernière'}</span>}
        {fromPool
          ? <span className="picto-chip">🗂 Pool Picto</span>
          : <span className="picto-chip">✍️ {words.length} mot{words.length > 1 ? 's' : ''}</span>}
        {cat && <span className="picto-chip">{cat.icon ? `${cat.icon} ` : ''}{cat.label}</span>}
      </div>

      <div className={`picto-grid${rafale ? ' picto-grid-stacked' : ''}`}>
        <div className="picto-card">
          <div className="picto-lbl">Mots à dessiner</div>
          <PictoWordsSource
            values={values}
            onChange={onChange}
            keys={{ source: fields.source, categories: fields.categories, difficulties: fields.difficulties }}
            customCategories={customCategories}
            roundTime={rafale ? Number(values.time) || 0 : 1}
            questionTime={rafale ? 10 : 1}
          >
            <div className="form-group">
              <label htmlFor="picto-word-input">
                Mots de la question <span className="picto-muted">— 1 à {PICTO_WORDS_MAX}, un par ligne</span>
              </label>
              <div className="picto-words">
                {words.map((w, i) => (
                  <span className="picto-chip" key={`${w}-${i}`}>
                    {w}
                    <button type="button" className="picto-chip-x" aria-label={`Retirer « ${w} »`} onClick={() => removeWord(i)}>✕</button>
                  </span>
                ))}
                <input
                  id="picto-word-input"
                  type="text"
                  className="picto-word-input"
                  value={draft}
                  maxLength={200}
                  disabled={full}
                  placeholder={full ? `${PICTO_WORDS_MAX} mots au maximum` : '＋ mot…'}
                  onChange={(e) => setDraft(e.target.value)}
                  onKeyDown={onKeyDown}
                  onBlur={() => addWords(draft)}
                  onPaste={(e) => {
                    const t = e.clipboardData?.getData('text') || ''
                    if (/[\n,;]/.test(t)) { e.preventDefault(); addWords(draft + t) }
                  }}
                />
              </div>
              <p className="picto-hint">
                Un mot est tiré au hasard à chaque lancement ou relance, parmi tous les mots ; le même peut ressortir.
                Avec un seul mot, c'est toujours ce mot. Ces mots ne sont jamais montrés sur la TV ni sur les tablettes.
              </p>
            </div>
            {ignored > 0 && (
              <p className="picto-hint err" role="alert">
                {ignored} mot{ignored > 1 ? 's' : ''} ignoré{ignored > 1 ? 's' : ''} : {PICTO_WORDS_MAX} mots au maximum par question.
              </p>
            )}
            {words.length === 0 && <p className="picto-hint err">Saisissez au moins un mot.</p>}
          </PictoWordsSource>
          {error && <div className="picto-error" role="alert">{pictoRefusalText(error)}</div>}
          <p className="picto-hint">La catégorie (une seule) ci-dessus {fromPool ? 'est le crédit de la question ; l\'indice TV est la catégorie du mot tiré.' : 'est l\'indice affiché sur la TV.'}</p>
        </div>

        {rafale && (
          <div className="picto-card" data-testid="picto-passes">
            <div className="picto-lbl">Passes</div>
            <div className="picto-numbers">
              <div className="form-group">
                <label htmlFor="picto-pass-cost">Coût d'une passe</label>
                <div className="picto-inline">
                  <input
                    id="picto-pass-cost"
                    type="number"
                    className={validPassCost(values[fields.passCost] ?? 0) ? '' : 'picto-input-error'}
                    value={values[fields.passCost] ?? 0}
                    min="0"
                    max={PASS_COST_MAX}
                    onChange={(e) => onChange(fields.passCost, e.target.value)}
                  />
                  <span>pts</span>
                </div>
                <span className="picto-muted">0 = gratuite</span>
              </div>
              <div className="form-group">
                <span className="picto-lbl-inline">Passes par manche</span>
                <div className="picto-source-toggle" role="group" aria-label="Passes par manche">
                  <button type="button" className={`picto-source-btn ${Number(values[fields.passLimit]) === 1 ? 'on' : ''}`} aria-pressed={Number(values[fields.passLimit]) === 1}
                    onClick={() => onChange(fields.passLimit, 1)}>1 passe</button>
                  <button type="button" className={`picto-source-btn ${Number(values[fields.passLimit]) !== 1 ? 'on' : ''}`} aria-pressed={Number(values[fields.passLimit]) !== 1}
                    onClick={() => onChange(fields.passLimit, 0)}>Illimitées</button>
                </div>
              </div>
            </div>
            <div className="form-group">
              <span className="picto-lbl-inline">Quand il n'y a plus de mot</span>
              <div className="picto-source-toggle" role="group" aria-label="Fin du pool">
                <button type="button" className={`picto-source-btn ${values[fields.poolEnd] === 'LOOP' ? 'on' : ''}`} aria-pressed={values[fields.poolEnd] === 'LOOP'}
                  onClick={() => onChange(fields.poolEnd, 'LOOP')}>🔁 Reboucler sur les passés</button>
                <button type="button" className={`picto-source-btn ${values[fields.poolEnd] !== 'LOOP' ? 'on' : ''}`} aria-pressed={values[fields.poolEnd] !== 'LOOP'}
                  onClick={() => onChange(fields.poolEnd, 'END')}>⏹ Finir la manche</button>
              </div>
            </div>
            <p className="picto-hint">Crédit suggéré en fin de manche : mots trouvés × {values.points || 0} pts − passes × {values[fields.passCost] || 0} pts (jamais sous 0), ajustable.</p>
          </div>
        )}

        <div className="picto-card">
          <div className="picto-lbl">Temps et points</div>
          <div className="picto-numbers">
            <div className="form-group">
              <label htmlFor="picto-time-input">{rafale ? 'Durée de la manche' : 'Durée de dessin'}</label>
              <div className="picto-inline">
                <input
                  id="picto-time-input"
                  type="number"
                  className={timeOk ? '' : 'picto-input-error'}
                  value={values.time}
                  min={PICTO_TIME_MIN}
                  max={PICTO_TIME_MAX}
                  onChange={(e) => onChange('time', e.target.value)}
                />
                <span>secondes</span>
              </div>
              <span className={`picto-muted ${timeOk ? '' : 'picto-err'}`}>{PICTO_TIME_MIN} à {PICTO_TIME_MAX} s</span>
            </div>
            <div className="form-group">
              <label htmlFor="picto-points-input">{rafale ? 'Points par mot trouvé' : 'Points du mot trouvé'}</label>
              <input
                id="picto-points-input"
                type="number"
                value={values.points}
                min="1"
                max="100"
                onChange={(e) => onChange('points', e.target.value)}
              />
              <span className="picto-muted">ajustables au crédit</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
