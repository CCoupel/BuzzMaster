import { useEffect, useRef, useState } from 'react'
import { useGame } from '../hooks/GameContext'
import './RegieMessageBar.css'

// #167 (F2b) — pause de frappe : 2000ms, pas 1500 (voir plan
// _work/reports/plan-20260818-121500.md, tâche F2b pour la justification).
const DEBOUNCE_MS = 2000
// #167 (F2) — commodité d'interface uniquement : la troncature fait
// autorité côté serveur (contrat §REGIE_MESSAGE_SEND, règle 3, en runes).
const MAX_LENGTH = 140

/**
 * RegieMessageBar — bandeau d'envoi régie → animateurs (`/admin/*`, #167).
 *
 * Composant neuf, sœur de `Navbar`, monté par `App.jsx` sous la même
 * condition `isAdminRoute`. Pleine largeur du bas de l'écran (voir
 * RegieMessageBar.css).
 *
 * **Aucun bouton « Envoyer ».** L'envoi part de lui-même sur trois
 * déclencheurs simultanés (F2b) : touche Entrée, perte de focus, pause de
 * frappe de 2s. Les deux seuls boutons sont « Effacer » (retirer son propre
 * message) et « Nouveau message » (bascule locale d'affichage après
 * acquittement, F3b — ne décide jamais de l'état actif/inactif lui-même).
 *
 * F3b — l'état affiché (actif / acquitté / repos) est dérivé EXCLUSIVEMENT
 * de `regieMessage` (état WebSocket, useWebSocket.js). Seule la saisie en
 * cours (`localText`) est un état local légitime : c'est un champ texte.
 */
export default function RegieMessageBar() {
  const { regieMessage, sendRegieMessage, clearRegieMessage } = useGame()
  const [localText, setLocalText] = useState('')
  // Bascule purement locale d'affichage après acquittement (« Nouveau
  // message ») — ne décide PAS de l'état actif/inactif du message
  // (regieMessage seul en décide, F3b), seulement si on montre le résumé
  // "Vu par l'animateur" ou l'input d'édition pendant que l'état serveur
  // reste "inactif, dernier effacement = ANIM".
  const [ackDismissed, setAckDismissed] = useState(false)

  const debounceRef = useRef(null)
  // Dernier texte (trimmé) effectivement envoyé par CE composant — garde
  // anti-doublon côté client (F2b). Le serveur pose la même garde et fait
  // autorité (contrat §REGIE_MESSAGE_SEND règle 4) ; celle-ci évite
  // simplement du trafic inutile.
  const lastSentRef = useRef('')
  // Texte du message actif le plus récent — nécessaire pour la règle de
  // vidage à l'acquittement : le REGIE_MESSAGE d'effacement porte TEXT:''
  // (contrat), pas le texte qui vient d'être effacé.
  const lastActiveTextRef = useRef('')
  const prevActiveRef = useRef(regieMessage.ACTIVE)

  // Nettoyage du timer de debounce au démontage — le bandeau est monté sur
  // tout /admin/*, vit longtemps (F2b, obligatoire).
  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
    }
  }, [])

  useEffect(() => {
    if (regieMessage.ACTIVE) {
      lastActiveTextRef.current = regieMessage.TEXT
    }
  }, [regieMessage.ACTIVE, regieMessage.TEXT])

  // F2b — vidage du champ à l'acquittement, UNIQUEMENT si son contenu
  // trimmé est encore égal au message qui vient d'être effacé. Si la régie
  // a commencé à composer autre chose entre-temps, on ne détruit pas sa
  // frappe.
  useEffect(() => {
    if (prevActiveRef.current && !regieMessage.ACTIVE) {
      setLocalText(prev => {
        if (prev.trim() === lastActiveTextRef.current) {
          lastSentRef.current = ''
          return ''
        }
        return prev
      })
    }
    prevActiveRef.current = regieMessage.ACTIVE
  }, [regieMessage.ACTIVE])

  // Réinitialise la bascule locale « Nouveau message » à chaque nouvel
  // événement serveur (nouvel envoi ou nouvel acquittement) — sans quoi un
  // second message acquitté resterait affiché en input à cause d'un clic
  // précédent sur "Nouveau message".
  useEffect(() => {
    setAckDismissed(false)
  }, [regieMessage.SENT_AT, regieMessage.CLEARED_BY])

  const doSend = (text) => {
    const trimmed = text.trim()
    if (!trimmed) return
    if (trimmed === lastSentRef.current) return
    lastSentRef.current = trimmed
    sendRegieMessage(trimmed)
  }

  const handleChange = (e) => {
    const value = e.target.value
    setLocalText(value)
    if (debounceRef.current) clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => {
      debounceRef.current = null
      doSend(value)
    }, DEBOUNCE_MS)
  }

  const handleKeyDown = (e) => {
    if (e.key !== 'Enter') return
    e.preventDefault()
    if (debounceRef.current) {
      clearTimeout(debounceRef.current)
      debounceRef.current = null
    }
    doSend(localText)
  }

  const handleBlur = () => {
    if (debounceRef.current) {
      clearTimeout(debounceRef.current)
      debounceRef.current = null
    }
    doSend(localText)
  }

  const handleClear = () => clearRegieMessage()
  const handleNewMessage = () => setAckDismissed(true)

  const isAcked = !regieMessage.ACTIVE && regieMessage.CLEARED_BY === 'ANIM' && !ackDismissed
  const remaining = MAX_LENGTH - localText.length

  return (
    <div className="regie-message-bar">
      <span className="regie-message-tag">Animateur</span>

      {regieMessage.ACTIVE ? (
        <>
          <span className="regie-message-pending">« {regieMessage.TEXT} »</span>
          <button type="button" className="regie-message-btn regie-message-btn-ghost" onClick={handleClear}>
            Effacer
          </button>
        </>
      ) : isAcked ? (
        <>
          <span className="regie-message-acked">Vu par l'animateur</span>
          <button type="button" className="regie-message-btn regie-message-btn-ghost" onClick={handleNewMessage}>
            Nouveau message
          </button>
        </>
      ) : (
        <>
          <input
            type="text"
            className="regie-message-input"
            value={localText}
            onChange={handleChange}
            onKeyDown={handleKeyDown}
            onBlur={handleBlur}
            maxLength={MAX_LENGTH}
            placeholder="Consigne à envoyer aux tablettes animateur…"
            aria-label="Consigne à envoyer aux tablettes animateur"
          />
          <span className={`regie-message-counter ${remaining <= 20 ? 'near' : ''}`}>{remaining}</span>
        </>
      )}
    </div>
  )
}
