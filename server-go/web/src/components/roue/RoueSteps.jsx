/** Bandeau des 7 étapes d'un tour, piloté par `ROUE_STEP` (contrat roue.md §5, maquette M2). */
import { ROUE_STEPS } from './roueState'
import './RoueControls.css'

export default function RoueSteps({ step = 0, before = false, after = false }) {
  // Libellés adaptés au moment d'achat des voyelles (v12.1.0, maquette admin retenue)
  const label = (s) => (s.n === 1 && before ? 'Lancer la roue\nou voyelle' : s.n === 4 && after ? 'Consonne\nou voyelle' : s.name)
  return (
    <ol className="roue-steps" aria-label="Étapes du tour">
      {ROUE_STEPS.map(s => {
        const state = s.n < step ? 'done' : s.n === step ? 'cur' : ''
        return (
          <li key={s.n} className={`roue-step ${state}`} aria-current={state === 'cur' ? 'step' : undefined}>
            <b>{state === 'done' ? '✓' : s.n}</b>
            {label(s).split('\n').map((l, i) => <span key={i}>{i > 0 && <br />}{l}</span>)}
          </li>
        )
      })}
    </ol>
  )
}
