import { useEffect, useRef, useState } from 'react'
import { useGame } from '../hooks/GameContext'
import useElementHeightVar from '../hooks/useElementHeightVar'
import './RegieMessageBar.css'

// #167 (F2b) — pause de frappe : 2000ms, pas 1500 (voir plan
// _work/reports/plan-20260818-121500.md, tâche F2b pour la justification).
const DEBOUNCE_MS = 2000
// #167 (F2) — commodité d'interface uniquement : la troncature fait
// autorité côté serveur (contrat §REGIE_MESSAGE_SEND, règle 3, en runes).
const MAX_LENGTH = 140
// #176 (F3) — durée d'affichage de l'indicateur fugace "Vu par l'animateur".
const ACK_INDICATOR_MS = 4000

/**
 * RegieMessageBar — bandeau d'envoi régie → animateurs (`/admin/*`, #167,
 * révisé #176).
 *
 * Composant sœur de `Navbar`, monté par `App.jsx` sous la même condition
 * `isAdminRoute`. Pleine largeur du bas de l'écran (voir RegieMessageBar.css).
 *
 * **Aucun bouton « Envoyer ».** L'envoi part de lui-même sur trois
 * déclencheurs simultanés (F2b) : touche Entrée, perte de focus, pause de
 * frappe de 2s.
 *
 * #176 — le champ de saisie est désormais **toujours visible et éditable**,
 * quel que soit l'état du message (repos, actif, juste acquitté) — plus
 * d'état bloquant « acquitté » ni de bouton « Nouveau message » (décision ①
 * du plan #176 : le seul retour d'acquittement du modèle devient un
 * indicateur EN LIGNE fugace, ~4s, à côté du champ qui reste éditable).
 * Le seul bouton restant est « Effacer », visible tant qu'un message est
 * actif.
 *
 * L'état affiché (texte du champ quand ACTIVE, indicateur fugace) est
 * dérivé de `regieMessage` (état WebSocket, useWebSocket.js) — SAUF la
 * saisie en cours d'un champ FOCALISÉ, qui n'est jamais écrasée par l'écho
 * serveur (F2, garde de focus — la course écho/saisie est le piège
 * principal de cette révision).
 */
export default function RegieMessageBar() {
  const { regieMessage, sendRegieMessage, clearRegieMessage } = useGame()
  const [localText, setLocalText] = useState('')
  // #176 (F3) — indicateur fugace "Vu par l'animateur", visible ~4s après un
  // acquittement animateur (CLEARED_BY === 'ANIM'). Un retrait régie
  // (CLEARED_BY === 'REGIE') n'affiche rien : la régie sait ce qu'elle vient
  // de faire.
  const [ackVisible, setAckVisible] = useState(false)

  // #177 (F1) — élément racine mesuré par useElementHeightVar ci-dessous.
  const barRef = useRef(null)

  const debounceRef = useRef(null)
  const ackTimerRef = useRef(null)
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
  // #176 (F2) — la garde qui rend la synchronisation entrante sûre : un
  // champ FOCALISÉ n'est jamais écrasé par l'écho serveur. Sans elle : la
  // régie tape "abcd", la pause de frappe envoie "abc" (frappe antérieure),
  // l'écho serveur revient et réécrit le champ à "abc" en pleine saisie —
  // course classique entre un champ contrôlé et son écho serveur.
  const isFocusedRef = useRef(false)

  // Nettoyage des timers (debounce + indicateur fugace) au démontage — le
  // bandeau est monté sur tout /admin/*, vit longtemps (F2b, obligatoire).
  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
      if (ackTimerRef.current) clearTimeout(ackTimerRef.current)
    }
  }, [])

  // #177 (F1, extrait en hook partagé #179/F2) — mesure la hauteur RÉELLE du
  // bandeau et la partage via une variable CSS (--regie-bar-h, App.css F2),
  // source unique consommée par .main-content ET par les huit pages admin
  // (F3) — remplace la constante 44px en dur qui provoquait soit un
  // débordement (bandeau plus petit que prévu... jamais le cas ici) soit un
  // recouvrement (bandeau plus grand, #176 : message + Effacer + indicateur
  // d'acquittement ensemble sur fenêtre étroite). Cleanup (disconnect +
  // remise à 0px au démontage) géré par le hook — AC8 inchangé.
  //
  // ⚠️ Pas de boucle ResizeObserver possible : --regie-bar-h n'influence
  // jamais CE composant (position: fixed, dimensionné par son propre
  // contenu et la largeur du viewport) — seulement .main-content et les
  // pages, qui n'affectent pas le bandeau en retour.
  useElementHeightVar(barRef, '--regie-bar-h')

  useEffect(() => {
    if (regieMessage.ACTIVE) {
      lastActiveTextRef.current = regieMessage.TEXT
    }
  }, [regieMessage.ACTIVE, regieMessage.TEXT])

  // #176 (F2) — synchronisation entrante : quand un message devient actif
  // ou est remplacé (SENT_AT réarmé) ET que le champ n'a PAS le focus,
  // aligne la saisie locale sur le texte serveur (AC2/AC3 — pré-remplissage,
  // visible par un second poste régie qui ne tape pas). `lastSentRef` est
  // posé en même temps pour qu'un blur ultérieur sur ce champ pré-rempli ne
  // renvoie pas le même texte (le serveur le dédupliquerait de toute façon —
  // ceci évite simplement du trafic inutile).
  useEffect(() => {
    if (regieMessage.ACTIVE && !isFocusedRef.current) {
      setLocalText(regieMessage.TEXT)
      lastSentRef.current = regieMessage.TEXT
    }
  }, [regieMessage.SENT_AT, regieMessage.ACTIVE, regieMessage.TEXT])

  // Vidage à l'effacement (F2b/#176 décision ②) + indicateur fugace
  // (#176/F3), sur la même transition ACTIVE true -> false.
  useEffect(() => {
    const wasActive = prevActiveRef.current
    if (wasActive && !regieMessage.ACTIVE) {
      // Vidage — UNIQUEMENT si le contenu local trimmé est encore égal au
      // message qui vient d'être effacé. Si la régie a commencé à composer
      // autre chose entre-temps, on ne détruit pas sa frappe (AC6).
      setLocalText(prev => {
        if (prev.trim() === lastActiveTextRef.current) {
          lastSentRef.current = ''
          return ''
        }
        return prev
      })

      if (regieMessage.CLEARED_BY === 'ANIM') {
        setAckVisible(true)
        if (ackTimerRef.current) clearTimeout(ackTimerRef.current)
        ackTimerRef.current = setTimeout(() => {
          ackTimerRef.current = null
          setAckVisible(false)
        }, ACK_INDICATOR_MS)
      }
    }
    prevActiveRef.current = regieMessage.ACTIVE
  }, [regieMessage.ACTIVE, regieMessage.CLEARED_BY])

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

  const handleFocus = () => {
    isFocusedRef.current = true
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
    isFocusedRef.current = false
    if (debounceRef.current) {
      clearTimeout(debounceRef.current)
      debounceRef.current = null
    }
    doSend(localText)
  }

  const handleClear = () => clearRegieMessage()

  const remaining = MAX_LENGTH - localText.length

  return (
    <div className="regie-message-bar" ref={barRef}>
      <span className="regie-message-tag">Animateur</span>
      <input
        type="text"
        className="regie-message-input"
        value={localText}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        onFocus={handleFocus}
        onBlur={handleBlur}
        maxLength={MAX_LENGTH}
        placeholder="Consigne à envoyer aux tablettes animateur…"
        aria-label="Consigne à envoyer aux tablettes animateur"
      />
      <span className={`regie-message-counter ${remaining <= 20 ? 'near' : ''}`}>{remaining}</span>
      {regieMessage.ACTIVE && (
        <button type="button" className="regie-message-btn regie-message-btn-ghost" onClick={handleClear}>
          Effacer
        </button>
      )}
      {ackVisible && (
        <span className="regie-message-ack-indicator">Vu par l'animateur</span>
      )}
    </div>
  )
}
