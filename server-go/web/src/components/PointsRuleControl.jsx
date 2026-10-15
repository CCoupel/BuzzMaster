import { allowedPointsRuleModes, defaultPointsRuleMode, describePointsRule } from '../utils/motionGrid'
import './PointsRuleControl.css'

const VALUE_MIN = 1
const VALUE_MAX = 999

/** Libellé d'une option du `select` selon le type (maquette admin_question_editor §4). */
function optionLabel(mode, isDefault, isProgression) {
  if (isDefault) return isProgression ? 'Prorata des étoiles (défaut)' : 'Étoiles (défaut)'
  switch (mode) {
    case 'STARS': return 'Étoiles pleines (tout ou rien)'
    case 'FIXED': return 'Montant fixe'
    case 'PER_UNIT': return 'Par unité'
    default: return mode
  }
}

function needsValue(mode) {
  return mode === 'FIXED' || mode === 'PER_UNIT'
}

function isValidValue(v) {
  return Number.isInteger(v) && v >= VALUE_MIN && v <= VALUE_MAX
}

/**
 * PointsRuleControl — ligne « Points : » de l'éditeur de carte MEMOTION (#190,
 * contrat question-types.md §6.2) : choix du barème `POINTS_RULE` d'une carte.
 *
 * - Options filtrées par type (binaire SPEEDY/QCM : Étoiles (défaut) / Montant
 *   fixe ; progression MEMORY/RAFALE : Prorata (défaut) / Étoiles pleines /
 *   Montant fixe / Par unité), le défaut en premier.
 * - `mode: ''` = défaut du type = `POINTS_RULE` ABSENT à l'envoi (jamais écrit).
 * - Montant (entier 1–999) uniquement pour `FIXED` / `PER_UNIT`.
 * - Aide « → … » calculée par `describePointsRule` (même fonction que les
 *   affichages anim/admin).
 *
 * @param {Object} props
 * @param {'SPEEDY'|'QCM'|'MEMORY'|'RAFALE'} props.cardType
 * @param {{mode: string, value: number|''}} props.value - `mode: ''` = défaut du type
 * @param {(next: {mode: string, value: number|''}) => void} props.onChange
 * @param {number} [props.difficulty] - valeur de la carte (1-3) pour l'aide
 * @param {Object|null} [props.motionConfig] - `question.MOTION_CONFIG`
 * @param {number} [props.unitsTotal] - MEMORY : nombre de paires de la carte
 * @param {string|null} [props.notice] - avertissement de remise au défaut
 * @param {string|null} [props.error] - erreur de saisie au submit
 * @param {string} props.idPrefix - préfixe d'id (ex. `motion-card-mc-1`)
 */
export default function PointsRuleControl({
  cardType,
  value,
  onChange,
  difficulty = 1,
  motionConfig,
  unitsTotal,
  notice = null,
  error = null,
  idPrefix,
}) {
  const modes = allowedPointsRuleModes(cardType)
  const defaultMode = defaultPointsRuleMode(cardType)
  const isProgression = defaultMode === 'STARS_PRORATA'
  const mode = value?.mode || ''
  const amount = value?.value ?? ''
  const showInput = needsValue(mode)
  const invalid = showInput && !isValidValue(amount)

  const hint = describePointsRule(
    {
      TYPE: cardType,
      DIFFICULTY: difficulty,
      POINTS_RULE: mode ? { MODE: mode, VALUE: amount } : undefined,
    },
    motionConfig,
    { unitsTotal },
  )

  const handleModeChange = (e) => {
    const next = e.target.value
    onChange({ mode: next, value: needsValue(next) ? amount : '' })
  }

  const handleAmountChange = (e) => {
    const raw = e.target.value
    onChange({ mode, value: raw === '' ? '' : Number(raw) })
  }

  const errorId = `${idPrefix}-points-rule-error`

  return (
    <div className="points-rule-row" role="group" aria-label="Barème de points de la carte">
      <span className="lbl">Points :</span>
      <select
        id={`${idPrefix}-points-rule-mode`}
        aria-label="Mode de barème"
        value={mode}
        onChange={handleModeChange}
      >
        {modes.map(m => {
          const isDefault = m === defaultMode
          return (
            <option key={m} value={isDefault ? '' : m}>
              {optionLabel(m, isDefault, isProgression)}
            </option>
          )
        })}
      </select>
      {showInput && (
        <input
          type="number"
          min={VALUE_MIN}
          max={VALUE_MAX}
          step={1}
          placeholder="1–999"
          aria-label={mode === 'PER_UNIT' ? 'Points par unité' : 'Montant'}
          aria-invalid={invalid ? 'true' : undefined}
          aria-describedby={error ? errorId : undefined}
          value={amount}
          onChange={handleAmountChange}
        />
      )}
      <span className="hint">→ {hint}</span>
      {notice && <span className="notice" role="status">{notice}</span>}
      {error && <span className="err" id={errorId} role="alert">{error}</span>}
    </div>
  )
}
