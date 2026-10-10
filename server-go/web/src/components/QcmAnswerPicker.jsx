import './QcmAnswerPicker.css'

const ANSWERS = [
  { key: 'RED', color: '#dc2626' },
  { key: 'GREEN', color: '#16a34a' },
  { key: 'YELLOW', color: '#ca8a04' },
  { key: 'BLUE', color: '#2563eb' },
]

/**
 * QcmAnswerPicker — les 4 réponses QCM cliquables après le REVEAL (#244/#251) : un clic
 * sélectionne les équipes qui l'ont donnée et règle le montant (décidé par l'appelant).
 * `teamCounts[key]` = nombre d'équipes ayant donné cette réponse.
 */
export default function QcmAnswerPicker({ question, teamCounts, picked, onPick }) {
  const answers = question?.QCM_ANSWERS || {}
  return (
    <div>
      <div className="qcm-picker" role="group" aria-label="Réponses QCM">
        {ANSWERS.filter((a) => answers[a.key] !== undefined).map((a) => {
          const n = teamCounts?.[a.key] || 0
          const correct = a.key === question?.QCM_CORRECT
          return (
            <button
              key={a.key}
              type="button"
              className={`qcm-picker-answer${correct ? ' correct' : ''}${picked === a.key ? ' picked' : ''}`}
              title={`Sélectionner les ${n} équipe(s) ayant répondu « ${answers[a.key]} »`}
              aria-pressed={picked === a.key}
              onClick={() => onPick(a.key)}
            >
              <span className="qcm-picker-sw" style={{ background: a.color }} aria-hidden="true" />
              <span className="qcm-picker-txt">{answers[a.key]}{correct && <span className="qcm-picker-ok"> ✓ bonne réponse</span>}</span>
              <span className="qcm-picker-n">{n} équipe{n > 1 ? 's' : ''}</span>
            </button>
          )
        })}
      </div>
      <p className="qcm-picker-hint">Cliquez une réponse pour sélectionner les équipes qui l'ont donnée.</p>
    </div>
  )
}
