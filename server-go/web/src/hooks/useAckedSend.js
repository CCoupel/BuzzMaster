import { useCallback, useEffect, useRef, useState } from 'react'

// #248 — envoi d'une action joueur (réponse QCM, buzz SPEEDY) qui attend l'ACCUSÉ
// du serveur et se renvoie tant qu'il n'est pas venu. Maquette validée :
// docs/mockup/v11.2.1/ui/vplayer_qcm_answer__ack.html.
//
// États (`status`) :
//   idle     aucune action en cours
//   sending  envoyée (ou à envoyer) mais pas d'accusé ; `resending` = renvoi lié à
//            un événement (reliaison, premier plan, reprise après pause)
//   acked    accusé reçu (écho de son propre bumper avec TIME > 0)
//   served   un coéquipier a déjà été servi (TIME d'équipe > 0), pas ce joueur : fin
//            des renvois, ce n'est pas une erreur
//   failed   question terminée / changée, ou plafond de 30 envois atteint, sans accusé
//
// Un seul appui par question (le premier) ; un échec ne propose jamais de réessayer.
export const ACKED_SEND_INTERVAL_MS = 4000
export const ACKED_SEND_MAX = 30

/**
 * @param {Object} o
 * @param {Function} o.sendBound   (action, payload) => boolean — n'émet que si la connexion est liée
 * @param {string}   o.connStatus  état WebSocket ('connected' …)
 * @param {boolean}  o.bound       connexion liée à notre bumper
 * @param {string}   o.phase       phase de jeu
 * @param {string|null} o.questionId question courante
 * @param {boolean}  o.acked       accusé reçu pour ce joueur
 * @param {boolean}  o.teamServed  l'équipe a déjà été servie
 */
export function useAckedSend({ sendBound, connStatus, bound, phase, questionId, acked, teamServed }) {
  const [status, setStatus] = useState('idle')
  const [resending, setResending] = useState(false)
  const recRef = useRef(null) // { action, payload, qid, sends }
  const statusRef = useRef('idle')
  const timerRef = useRef(null)
  const live = useRef({})
  live.current = { sendBound, connStatus, bound, phase, questionId, acked, teamServed }

  const setSt = useCallback((s) => { statusRef.current = s; setStatus(s) }, [])

  const stopTimer = () => {
    if (timerRef.current) clearInterval(timerRef.current)
    timerRef.current = null
  }

  // Tente un envoi ; `evt` = déclenché par un événement (marque un « renvoi »).
  const attempt = useCallback((evt) => {
    const rec = recRef.current
    if (!rec || statusRef.current !== 'sending') return
    const l = live.current
    if (rec.sends >= ACKED_SEND_MAX) { stopTimer(); setSt('failed'); return }
    if (l.connStatus !== 'connected' || !l.bound || l.phase !== 'STARTED') return
    if (!l.sendBound(rec.action, rec.payload)) return
    if (evt && rec.sends > 0) setResending(true)
    rec.sends += 1
    // la cadence repart de cet envoi (pas de double envoi collé à un événement)
    stopTimer()
    timerRef.current = setInterval(() => attempt(false), ACKED_SEND_INTERVAL_MS)
  }, [setSt])

  const start = useCallback((action, payload) => {
    const l = live.current
    if (recRef.current && recRef.current.qid === l.questionId) return false // un seul appui par question
    stopTimer()
    recRef.current = { action, payload, qid: l.questionId, sends: 0 }
    setResending(false)
    setSt('sending')
    timerRef.current = setInterval(() => attempt(false), ACKED_SEND_INTERVAL_MS)
    attempt(false)
    return true
  }, [attempt, setSt])

  const clear = useCallback(() => {
    stopTimer()
    recRef.current = null
    setResending(false)
    setSt('idle')
  }, [setSt])

  // Accusé / équipe déjà servie / contexte périmé.
  useEffect(() => {
    const rec = recRef.current
    if (!rec) return
    const s = statusRef.current
    if (s === 'sending') {
      if (acked) { stopTimer(); setSt('acked'); return }
      if (teamServed) { stopTimer(); setSt('served'); return }
      const stale = rec.qid !== questionId || !(phase === 'STARTED' || phase === 'PAUSED')
      if (stale) { stopTimer(); setSt('failed') }
    } else if (s === 'acked' || s === 'served') {
      if (rec.qid !== questionId) clear()
    }
    if (phase === 'PREPARE' && recRef.current) clear()
  }, [acked, teamServed, questionId, phase, clear, setSt])

  // Renvoi : reliaison, reprise en STARTED.
  useEffect(() => { attempt(true) }, [bound, connStatus, phase, attempt])

  // Renvoi : retour au premier plan.
  useEffect(() => {
    const onVisible = () => {
      if (document.visibilityState === 'visible') attempt(true)
    }
    document.addEventListener('visibilitychange', onVisible)
    return () => document.removeEventListener('visibilitychange', onVisible)
  }, [attempt])

  useEffect(() => stopTimer, [])

  return { status, resending, start, clear }
}
