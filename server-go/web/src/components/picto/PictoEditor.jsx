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
import { useState } from 'react'
import { categoryMeta } from '../../utils/categoryUtils'
import './PictoEditor.css'

export const PICTO_WORDS_MAX = 20
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

export function isPictoReady(values) {
  const n = (values.pictoWords || []).length
  return n >= 1 && n <= PICTO_WORDS_MAX && validPictoTime(values.time)
}

export default function PictoEditor({ values, onChange, error, customCategories = [] }) {
  const [draft, setDraft] = useState('')
  const words = values.pictoWords || []
  const full = words.length >= PICTO_WORDS_MAX
  const timeOk = validPictoTime(values.time)
  const cat = values.category ? categoryMeta(values.category, customCategories) : null

  // Ajoute un ou plusieurs mots (un par ligne ou séparés par virgule/point-virgule) ; doublons : refus serveur.
  const addWords = (raw) => {
    const incoming = String(raw ?? '').split(/[\n,;]+/).map(normalizePictoWord).filter(Boolean)
    if (!incoming.length) return
    onChange('pictoWords', [...words, ...incoming].slice(0, PICTO_WORDS_MAX))
    setDraft('')
  }
  const removeWord = (i) => onChange('pictoWords', words.filter((_, k) => k !== i))

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
        <span className="picto-pill">✏️ Picto</span>
        <span className="picto-muted">Un mot à faire deviner par une équipe ; un joueur le dessine.</span>
      </div>

      <div className="picto-summary" aria-label="Résumé de la question">
        <span className="picto-chip">⏱ {timeOk ? `${values.time} s` : '— s'}</span>
        <span className="picto-chip">🏆 {values.points || '—'} pts</span>
        <span className="picto-chip">✍️ {words.length} mot{words.length > 1 ? 's' : ''}</span>
        {cat && <span className="picto-chip">{cat.icon ? `${cat.icon} ` : ''}{cat.label}</span>}
      </div>

      <div className="picto-grid">
        <div className="picto-card">
          <div className="picto-lbl">Mot à dessiner</div>
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
          {words.length === 0 && <p className="picto-hint err">Saisissez au moins un mot.</p>}
          {error && <div className="picto-error" role="alert">{pictoRefusalText(error)}</div>}
          <p className="picto-hint">La catégorie (une seule) ci-dessus est l'indice affiché sur la TV.</p>
        </div>

        <div className="picto-card">
          <div className="picto-lbl">Temps et points</div>
          <div className="picto-numbers">
            <div className="form-group">
              <label htmlFor="picto-time-input">Durée de dessin</label>
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
              <label htmlFor="picto-points-input">Points du mot trouvé</label>
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
