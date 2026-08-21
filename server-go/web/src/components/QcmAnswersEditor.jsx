import { QCM_COLORS } from '../constants/colors'

/**
 * QcmAnswersEditor — sous-éditeur QCM réutilisable (#184/B-F4, commit 1
 * d'extraction — strictement neutre).
 *
 * Pas de CSS propre : les classes (`qcm-answers-section`, `qcm-form-answers`,
 * `qcm-answer-item`…) restent définies dans `pages/QuestionsPage.css`, seul
 * lieu de montage de ce composant — les déplacer serait un risque de
 * régression visuelle pour un gain nul, hors du périmètre « extraction
 * neutre » de ce commit.
 *
 * Extrait à l'identique du bloc `qcm-answers-section` de `QuestionsPage.jsx`
 * (question standalone), pour être monté aussi bien pour une question `TYPE
 * = QCM` que pour une carte MEMOTION `TYPE = QCM` (#184/B-F4, commit 2) —
 * mêmes 4 réponses colorées, même bouton de désignation, mêmes indices
 * progressifs. Ni la forme des données (`values`) ni les callbacks
 * (`onFieldChange`/`onAnswerChange`) ne présument de leur origine
 * (`formData` top-level ou item de `formData.motionCards`) : c'est
 * l'appelant qui sait où écrire.
 *
 * @param {Object} props
 * @param {Object} props.values - forme QCM : {qcmAnswers, qcmCorrect,
 *   qcmHintsEnabled, qcmHintThreshold1, qcmHintThreshold2, qcmPenalty1, qcmPenalty2}
 * @param {(field: string, value: any) => void} props.onFieldChange - écrit un
 *   champ scalaire (qcmCorrect, qcmHintsEnabled, qcmHintThreshold1/2, qcmPenalty1/2)
 * @param {(color: string, value: string) => void} props.onAnswerChange - écrit
 *   le texte d'une réponse (qcmAnswers[color])
 */
export default function QcmAnswersEditor({ values, onFieldChange, onAnswerChange }) {
  const {
    qcmAnswers,
    qcmCorrect,
    qcmHintsEnabled,
    qcmHintThreshold1,
    qcmHintThreshold2,
    qcmPenalty1,
    qcmPenalty2,
  } = values

  return (
    <div className="qcm-answers-section">
      <label>Reponses QCM *</label>
      <div className="qcm-form-answers">
        {Object.entries(QCM_COLORS).map(([colorKey, { color, letter }]) => (
          <div
            key={colorKey}
            className={`qcm-answer-item ${qcmCorrect === colorKey ? 'correct' : ''}`}
            style={{ '--qcm-color': color }}
          >
            <div className="qcm-answer-header">
              <span className="qcm-letter" style={{ backgroundColor: color }}>{letter}</span>
              <button
                type="button"
                className={`qcm-correct-btn ${qcmCorrect === colorKey ? 'active' : ''}`}
                onClick={() => onFieldChange('qcmCorrect', colorKey)}
                title="Marquer comme bonne reponse"
              >
                {qcmCorrect === colorKey ? '✓' : '○'}
              </button>
            </div>
            <input
              type="text"
              value={qcmAnswers[colorKey]}
              onChange={(e) => onAnswerChange(colorKey, e.target.value)}
              placeholder={`Reponse ${letter}...`}
              className="qcm-answer-input"
            />
          </div>
        ))}
      </div>
      {!qcmCorrect && (
        <p className="qcm-hint">Cliquez sur ○ pour indiquer la bonne reponse</p>
      )}

      {/* QCM Hints Toggle */}
      <div className="qcm-hints-toggle">
        <label>
          <input
            type="checkbox"
            checked={qcmHintsEnabled}
            onChange={(e) => onFieldChange('qcmHintsEnabled', e.target.checked)}
          />
          Activer les indices progressifs
        </label>
        <span className="qcm-hints-description">
          {qcmHintsEnabled
            ? `Les mauvaises reponses seront invalidees progressivement (penalites: ${Math.round((qcmPenalty1 || 0.67) * 100)}% puis ${Math.round((qcmPenalty2 || 0.33) * 100)}%)`
            : 'Pas d\'indices automatiques'}
        </span>
        {qcmHintsEnabled && (
          <div className="qcm-hints-config">
            <div className="hint-row">
              <span className="hint-label">Indice 1 :</span>
              <span className="hint-at">a</span>
              <input
                type="number"
                min="1"
                max="99"
                value={Math.round((qcmHintThreshold1 || 0.25) * 100)}
                onChange={(e) => onFieldChange('qcmHintThreshold1', parseInt(e.target.value) / 100)}
              />
              <span className="hint-unit">%</span>
              <span className="hint-arrow">→</span>
              <input
                type="number"
                min="1"
                max="99"
                value={Math.round((qcmPenalty1 || 0.67) * 100)}
                onChange={(e) => onFieldChange('qcmPenalty1', parseInt(e.target.value) / 100)}
              />
              <span className="hint-unit">% pts</span>
            </div>
            <div className="hint-row">
              <span className="hint-label">Indice 2 :</span>
              <span className="hint-at">a</span>
              <input
                type="number"
                min="1"
                max="99"
                value={Math.round((qcmHintThreshold2 || 0.125) * 100)}
                onChange={(e) => onFieldChange('qcmHintThreshold2', parseInt(e.target.value) / 100)}
              />
              <span className="hint-unit">%</span>
              <span className="hint-arrow">→</span>
              <input
                type="number"
                min="1"
                max="99"
                value={Math.round((qcmPenalty2 || 0.33) * 100)}
                onChange={(e) => onFieldChange('qcmPenalty2', parseInt(e.target.value) / 100)}
              />
              <span className="hint-unit">% pts</span>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
