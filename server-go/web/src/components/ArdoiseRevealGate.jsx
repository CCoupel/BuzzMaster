import { useState } from 'react'
import './ArdoiseRevealGate.css'

/**
 * ArdoiseRevealGate — motif d'attente + « Révéler quand même… » + confirmation
 * (#246 v11.2, maquettes validées admin_game_ardoise_zone__reveal_gate et
 * anim_ardoise_list__reveal_gate). Partagé par /admin et /anim : une seule
 * règle, deux présentations (`variant` 'admin' | 'anim').
 *
 * Rendu seulement si `pending` est non vide. Le forçage n'est jamais direct :
 * « Révéler quand même… » ouvre une confirmation qui liste les équipes.
 *
 * @param {Array<{team: string, offline: boolean}>} props.pending
 * @param {'admin'|'anim'} props.variant
 * @param {() => void} props.onForce - émet REVEAL {FORCE: true}
 */
export default function ArdoiseRevealGate({ pending, variant = 'admin', onForce }) {
  const [confirming, setConfirming] = useState(false)
  if (!pending || pending.length === 0) return null

  const n = pending.length
  const plural = n > 1 ? 's' : ''
  const isAnim = variant === 'anim'

  const reason = isAnim ? (
    <div className="ardoise-gate-reason ardoise-gate-reason-anim" data-testid="ardoise-gate-reason">
      En attente de {n} tablette{plural} :{' '}
      {pending.map((p, i) => (
        <span key={p.team}>
          {i > 0 && ', '}
          {p.offline ? <b>{p.team} (hors ligne)</b> : p.team}
        </span>
      ))}
    </div>
  ) : (
    <div className="start-wait-reason ardoise-gate-reason" data-testid="ardoise-gate-reason">
      En attente des réponses de {n} tablette{plural} :
      <ul>
        {pending.map((p) => (
          <li key={p.team}>
            {p.team}
            {p.offline && <> — <b>hors ligne</b></>}
          </li>
        ))}
      </ul>
    </div>
  )

  return (
    <>
      {reason}
      <button
        type="button"
        className={`ardoise-gate-force${isAnim ? ' ardoise-gate-force-anim' : ''}`}
        onClick={() => setConfirming(true)}
      >
        Révéler quand même…
      </button>
      {confirming && (
        <div
          className={`ardoise-gate-backdrop ardoise-gate-${variant}`}
          onClick={() => setConfirming(false)}
        >
          <div
            className="ardoise-gate-dialog"
            role="dialog"
            aria-labelledby="ardoise-gate-title"
            onClick={(e) => e.stopPropagation()}
          >
            <h4 id="ardoise-gate-title">Révéler sans toutes les réponses ?</h4>
            <p>
              {isAnim
                ? 'Réponse finale non reçue pour :'
                : 'Ces équipes n’ont pas encore transmis leur réponse finale :'}
            </p>
            <ul>
              {pending.map((p) => (
                <li key={p.team}>
                  {p.team}
                  {p.offline && <> — <b>hors ligne</b></>}
                </li>
              ))}
            </ul>
            <p>
              {isAnim
                ? 'Elles seront révélées avec le dernier texte reçu, marqué « non confirmée ».'
                : 'Elles seront révélées avec le dernier texte reçu pendant la question, marqué « non confirmée ».'}
            </p>
            <div className="ardoise-gate-actions">
              <button type="button" className="ardoise-gate-cancel" onClick={() => setConfirming(false)}>
                Attendre
              </button>
              <button
                type="button"
                className="ardoise-gate-confirm"
                onClick={() => { setConfirming(false); onForce() }}
              >
                Révéler quand même
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  )
}

/** Pastille de réception d'une équipe (✓ reçue / ⏳ en attente / ⚠ hors ligne / non confirmée). */
export function ArdoiseReceiptChip({ status, revealed, className = '' }) {
  let kind, label
  if (revealed && status !== 'ok') { kind = 'unconf'; label = 'non confirmée' }
  else if (status === 'ok') { kind = 'ok'; label = '✓ reçue' }
  else if (status === 'off') { kind = 'off'; label = '⚠ hors ligne' }
  else { kind = 'wait'; label = '⏳ en attente' }
  return <span className={`ardoise-rx ardoise-rx-${kind} ${className}`.trim()}>{label}</span>
}

/** Compteur « n/N reçues » (gris « Saisie en cours » pendant STARTED). */
export function ArdoiseReceiptCounter({ receipt, live, className = '' }) {
  if (live) {
    return <span className={`ardoise-rcounter live ${className}`.trim()}>Saisie en cours</span>
  }
  return (
    <span className={`ardoise-rcounter ${receipt.complete ? 'complete' : ''} ${className}`.trim()}>
      {receipt.receivedCount}/{receipt.total} reçues
    </span>
  )
}
