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

const RECEIPT_ICONS = {
  ok: { glyph: '✓', label: 'Réponse reçue' },
  wait: { glyph: '⏳', label: 'En attente de la réponse finale' },
  off: { glyph: '⚠', label: 'Tablette hors ligne — réponse finale non reçue' },
}
const UNCONFIRMED_LABEL = 'Non confirmée — révélée sans réponse finale'

/**
 * Icône de réception d'une équipe, à GAUCHE du texte de la réponse
 * (maquettes validées `*__receipt_icon`) : ✓ vert reçue · ⏳ en attente · ⚠ orange
 * hors ligne ; après un REVEAL forcé, le glyphe d'origine grisé (« non confirmée »).
 * `title` + `aria-label` : l'information ne passe pas seulement par la couleur.
 * Sans `status`, la colonne reste réservée (même hauteur de carte quel que soit l'état).
 */
export function ArdoiseReceiptIcon({ status, revealed }) {
  const icon = RECEIPT_ICONS[status]
  if (!icon) return <span className="ardoise-receipt-icon" aria-hidden="true" />
  if (revealed && status !== 'ok') {
    return (
      <span className="ardoise-receipt-icon unconfirmed" role="img" aria-label={UNCONFIRMED_LABEL} title={UNCONFIRMED_LABEL}>
        {icon.glyph}
      </span>
    )
  }
  return (
    <span className={`ardoise-receipt-icon ${status}`} role="img" aria-label={icon.label} title={icon.label}>
      {icon.glyph}
    </span>
  )
}

/** Légende de l'en-tête (inchangée côté admin ; ajoutée à côté du compteur côté anim). */
export function ArdoiseReceiptLegend({ className = '' }) {
  return (
    <div className={`ardoise-legend ${className}`.trim()}>
      <span>✓ reçue</span><span>⏳ en attente</span><span>⚠ hors ligne</span>
    </div>
  )
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
