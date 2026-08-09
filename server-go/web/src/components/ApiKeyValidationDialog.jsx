import { useEffect, useRef } from 'react'
import Button from './Button'
import './ApiKeyValidationDialog.css'

// Dialogue de validation de clé API (contracts/ai-key-validation.md §9, tâche #13
// du plan). Deux variantes normatives — jamais fusionnées en une "erreur de
// validation" générique, la distinction porte tout le sens de la fonctionnalité :
//   - invalid_key  : le fournisseur a explicitement REFUSÉ la clé -> corriger.
//   - unreachable  : la clé n'a PAS PU être vérifiée (réseau, timeout, 5xx, 429)
//                     -> peut-être bonne, réessayer plus tard.
//
// Modal bloquant (pas de fermeture par clic sur l'overlay) : forcer l'enregistrement
// est une décision consciente, pas une bannière qu'on ignore (contrat §9). Focus
// piégé + Échap = "Corriger" (l'option non destructive), demandé par le plan (tâche 8).
const PROVIDER_LABELS = { anthropic: 'Claude', groq: 'Groq' }

export default function ApiKeyValidationDialog({
  provider,
  result, // 'invalid_key' | 'unreachable'
  httpStatus,
  detail,
  onCorrect,
  onForceSave,
  onRetry, // uniquement pour 'unreachable'
  retrying = false,
  forcing = false,
}) {
  const dialogRef = useRef(null)
  const providerLabel = PROVIDER_LABELS[provider] || provider

  const title = result === 'invalid_key'
    ? `${providerLabel} a refusé cette clé`
    : `Impossible de joindre ${providerLabel}`

  const body = result === 'invalid_key'
    ? "La clé a bien été transmise, mais " + providerLabel + " ne la reconnaît pas. Vérifiez que vous l'avez copiée en entier et qu'elle n'a pas été révoquée."
    : "La clé n'a pas pu être vérifiée — elle n'est ni confirmée ni refusée. Si ce serveur est hors ligne, vous pouvez l'enregistrer telle quelle et la vérifier plus tard."

  const detailLine = httpStatus
    ? `${httpStatus}${detail ? ' · ' + detail : ''}`
    : (detail || (result === 'unreachable' ? 'Délai de 10 s dépassé — aucune réponse' : ''))

  // Focus piégé : au montage, focus le premier bouton ; Tab/Shift+Tab cyclent
  // uniquement parmi les éléments focusables du dialogue. Échap = Corriger
  // (option non destructive, jamais "Enregistrer quand même").
  useEffect(() => {
    const dialogEl = dialogRef.current
    const getFocusable = () =>
      Array.from(dialogEl?.querySelectorAll('button:not(:disabled)') || [])

    const focusable = getFocusable()
    focusable[0]?.focus()

    const handleKeyDown = (e) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        onCorrect()
        return
      }
      if (e.key !== 'Tab') return
      const items = getFocusable()
      if (items.length === 0) return
      const first = items[0]
      const last = items[items.length - 1]
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault()
        last.focus()
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault()
        first.focus()
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [onCorrect])

  return (
    <div className="apikey-validation-overlay">
      <div
        ref={dialogRef}
        className={`apikey-validation-modal ${result === 'invalid_key' ? 'refus' : 'injoignable'}`}
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="apikey-validation-title"
        aria-describedby="apikey-validation-body"
      >
        <h3 id="apikey-validation-title" className="apikey-validation-title">{title}</h3>
        <p id="apikey-validation-body" className="apikey-validation-body">{body}</p>
        {detailLine && <div className="apikey-validation-detail">{detailLine}</div>}
        <div className="apikey-validation-actions">
          <Button variant="ghost" size="sm" onClick={onCorrect}>
            Corriger la clé
          </Button>
          {result === 'unreachable' && (
            <Button variant="secondary" size="sm" onClick={onRetry} loading={retrying}>
              Réessayer
            </Button>
          )}
          <Button variant="danger" size="sm" onClick={onForceSave} loading={forcing}>
            Enregistrer quand même
          </Button>
        </div>
      </div>
    </div>
  )
}
