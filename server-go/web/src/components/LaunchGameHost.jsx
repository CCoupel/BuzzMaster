import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useGame } from '../hooks/GameContext'
import { LaunchGamePopup } from './LaunchGameDialog'
import './LaunchGameHost.css'

// #253 (v11.2) — « Lancer la partie » : entrée du menu Préparation + popup rendue en PORTAIL sur
// <body> (plein écran, plus cloisonnée dans une zone de page) + verrou serveur LAUNCH_LOCK.
// Contrat : websocket-actions.md §LAUNCH_LOCK. Maquettes : admin_navbar__launch_game_entry,
// admin_new_game__launch_overlay.

const RENEW_INTERVAL_MS = 15000 // renouvellement du verrou (expiration serveur : 45 s)
const OPEN_WAIT_MS = 1000 // attente max de l'état verrouillé à notre nom
const TOAST_MS = 6000

export const ENTRACTE_TOAST = "Impossible pendant une entracte : sortez d'abord de l'entracte."
export const LOCK_BANNER_TEXT = "🔒 Lancement d'une nouvelle partie en cours sur un autre poste : actions de jeu suspendues."
export const LOCKED_BY_OTHER_TITLE = 'Popup « Lancer la partie » déjà ouverte sur un autre poste'
export const LAUNCH_TITLE = "Choisir ce qu'il faut effacer et relancer la partie"

const LaunchGameContext = createContext({ openLaunch: () => {}, lockedByOther: false })

export function useLaunchGame() {
  return useContext(LaunchGameContext)
}

/**
 * État du verrou vu depuis CE poste : `active` (une popup « Lancer la partie » est ouverte quelque
 * part), `lockedByOther` (ouverte sur UN AUTRE poste → actions de jeu suspendues ici).
 */
export function useLaunchLock() {
  const { gameState, connectionId } = useGame()
  const lock = gameState?.LAUNCH_LOCK
  const active = !!lock?.ACTIVE
  const isOwner = active && !!connectionId && lock.OWNER === connectionId
  return { active, isOwner, lockedByOther: active && !isOwner }
}

/** Bandeau « verrou actif » des postes non propriétaires (admin et anim). */
export function LaunchLockBanner({ className = '', children }) {
  const { lockedByOther } = useLaunchLock()
  if (!lockedByOther) return null
  return (
    <div className={`launch-lock-banner ${className}`.trim()} role="status">
      {children || LOCK_BANNER_TEXT}
    </div>
  )
}

export default function LaunchGameHost({ children }) {
  const {
    gameState, teams, bumpers, questions, newGame, newGameRefused, clearNewGameRefused,
    launchLock, launchLockRefused, clearLaunchLockRefused, actionRefused, clearActionRefused,
  } = useGame()
  const { isOwner, lockedByOther } = useLaunchLock()
  const [open, setOpen] = useState(false)
  const [waiting, setWaiting] = useState(false)
  const [expired, setExpired] = useState(false)
  const [toast, setToast] = useState(null)
  const openerRef = useRef(null)
  const closingRef = useRef(false) // CLOSE déjà envoyé pour ce verrou (évite un doublon)
  const openRef = useRef(false)
  const launchLockRef = useRef(launchLock)
  openRef.current = open
  launchLockRef.current = launchLock

  // ── Toast (haut droite) ────────────────────────────────────────────────
  useEffect(() => {
    if (!toast) return undefined
    const id = setTimeout(() => setToast(null), TOAST_MS)
    return () => clearTimeout(id)
  }, [toast])

  // ── Ouverture : OPEN → attente de l'état verrouillé à NOTRE nom (≤ 1 s) → popup ──────────
  const openLaunch = useCallback(() => {
    if (open || waiting) return
    if (gameState?.entracte) { setToast(ENTRACTE_TOAST); return }
    if (lockedByOther) return
    openerRef.current = document.activeElement
    clearLaunchLockRefused?.()
    if (launchLock?.('OPEN') === false) {
      setToast('Connexion perdue — réessayez.')
      return
    }
    setWaiting(true)
  }, [open, waiting, gameState?.entracte, lockedByOther, launchLock, clearLaunchLockRefused])

  useEffect(() => {
    if (waiting && isOwner) {
      setWaiting(false)
      setExpired(false)
      setOpen(true)
    }
  }, [waiting, isOwner])

  useEffect(() => {
    if (!waiting) return undefined
    const id = setTimeout(() => {
      setWaiting(false)
      setToast("Impossible d'ouvrir « Lancer la partie » : le serveur n'a pas répondu.")
    }, OPEN_WAIT_MS)
    return () => clearTimeout(id)
  }, [waiting])

  // ── Renouvellement toutes les 15 s tant que la popup est ouverte ─────────────────────────
  useEffect(() => {
    if (!open || expired) return undefined
    const id = setInterval(() => launchLock?.('OPEN'), RENEW_INTERVAL_MS)
    return () => clearInterval(id)
  }, [open, expired, launchLock])

  // ── Verrou perdu pendant que la popup est ouverte (expiration de sécurité) ──────────────
  useEffect(() => {
    if (open && !isOwner) setExpired(true)
  }, [open, isOwner])

  // ── Fermeture : CLOSE (Annuler, Échap, succès) et pagehide ──────────────────────────────
  const close = useCallback(() => {
    closingRef.current = true
    launchLock?.('CLOSE')
    setOpen(false)
    setWaiting(false)
    openerRef.current?.focus?.()
  }, [launchLock])

  useEffect(() => {
    if (!open) return undefined
    const onHide = () => { launchLock?.('CLOSE') }
    window.addEventListener('pagehide', onHide)
    return () => window.removeEventListener('pagehide', onHide)
  }, [open, launchLock])

  // ── Verrou orphelin : pris à notre nom après l'expiration de l'attente d'ouverture (1 s) ──
  // → aucune popup à afficher, on le libère tout de suite au lieu d'attendre l'expiration serveur.
  useEffect(() => {
    if (!isOwner) { closingRef.current = false; return }
    if (!open && !waiting && !closingRef.current) {
      closingRef.current = true
      launchLock?.('CLOSE')
    }
  }, [isOwner, open, waiting, launchLock])

  // ── Démontage de l'hôte (changement de route SPA) popup ouverte → CLOSE ─────────────────
  useEffect(() => () => {
    if (openRef.current) launchLockRef.current?.('CLOSE')
  }, [])

  // ── Refus serveur ──────────────────────────────────────────────────────────────────────
  useEffect(() => {
    if (!launchLockRefused) return
    setWaiting(false)
    setToast(launchLockRefused.reason === 'ENTRACTE_ACTIVE'
      ? ENTRACTE_TOAST
      : 'Une fenêtre « Lancer la partie » est déjà ouverte sur un autre poste.')
    clearLaunchLockRefused?.()
  }, [launchLockRefused, clearLaunchLockRefused])

  useEffect(() => {
    if (!actionRefused) return
    setToast('Action suspendue : lancement d’une nouvelle partie en cours.')
    clearActionRefused?.()
  }, [actionRefused, clearActionRefused])

  const value = useMemo(() => ({ openLaunch, lockedByOther }), [openLaunch, lockedByOther])

  return (
    <LaunchGameContext.Provider value={value}>
      {children}
      {toast && <div className="launch-toast" role="alert">{toast}</div>}
      {open && createPortal(
        <LaunchGamePopup
          onLaunch={newGame}
          onClose={close}
          teams={teams}
          bumpers={bumpers}
          questions={questions}
          phase={gameState?.phase}
          refused={newGameRefused}
          onClearRefused={clearNewGameRefused}
          expired={expired}
        />,
        document.body
      )}
    </LaunchGameContext.Provider>
  )
}
