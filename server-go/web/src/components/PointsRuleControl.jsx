import { allowedPointsRuleModes, defaultPointsRuleMode, describePointsRule } from '../utils/motionGrid'
import { isValidPointsRuleValue, pointsRuleModeLabel, POINTS_RULE_VALUE_MAX, POINTS_RULE_VALUE_MIN } from '../utils/pointsRuleEditor'
import './PointsRuleControl.css'

function needsValue(mode) {
  return mode === 'FIXED' || mode === 'PER_UNIT'
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
  const mode = value?.mode || ''
  const amount = value?.value ?? ''
  const showInput = needsValue(mode)
  const invalid = showInput && !isValidPointsRuleValue(amount)

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
              {pointsRuleModeLabel(isDefault ? '' : m, cardType)}
            </option>
          )
        })}
      </select>
      {showInput && (
        <input
          type="number"
          min={POINTS_RULE_VALUE_MIN}
          max={POINTS_RULE_VALUE_MAX}
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
