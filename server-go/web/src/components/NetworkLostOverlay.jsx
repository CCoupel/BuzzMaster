// #267 (v11.2.2) — alerte admin « le serveur n'a pas de réseau ».
// Maquette validée : admin_network_overlay__no_network (3 états : overlay / bandeau replié / toast).
// Contrat : contracts/game-state.md §NETWORK ; décisions Q1–Q3 (2026-10-13).
//
//   NETWORK.ONLY_LOCALHOST = true  → overlay plein écran (role="alertdialog", portail sur <body>)
//   « Continuer quand même » / Échap → repli en bandeau rouge persistant (« Détails » rouvre l'overlay)
//   nouvelle transition false → true → l'overlay se rouvre, même si l'opérateur l'avait replié
//   true → false → overlay et bandeau disparaissent, toast « Réseau rétabli — <IP> » 6 s
//
// Monté par App.jsx pour les routes /admin/* uniquement (jamais /tv, /anim, /player).
import { useCallback, useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useGame } from '../hooks/GameContext'
import NetworkWarningBanner, { formatCheckedAgo } from './NetworkWarningBanner'
import './NetworkLostOverlay.css'

export const NETWORK_TOAST_MS = 6000
const TICK_MS = 1000

const FOCUSABLE = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'

export default function NetworkLostOverlay() {
  const { gameState } = useGame()
  const network = gameState?.NETWORK
  const onlyLocalhost = network?.ONLY_LOCALHOST === true
  const ips = Array.isArray(network?.IPS) ? network.IPS : []
  const ignored = Array.isArray(network?.IGNORED_INTERFACES) ? network.IGNORED_INTERFACES : []
  const checkedAt = network?.CHECKED_AT ?? 0

  const [collapsed, setCollapsed] = useState(false)
  const [toast, setToast] = useState(null)
  const [now, setNow] = useState(() => Date.now())

  // Transitions : le ref garde la valeur du rendu précédent (initialisée à la valeur de montage,
  // donc un admin qui se connecte pendant la panne voit l'overlay, sans toast ni transition).
  const prevOnlyRef = useRef(onlyLocalhost)
  const lastIpsRef = useRef(ips)
  lastIpsRef.current = ips

  useEffect(() => {
    const prev = prevOnlyRef.current
    prevOnlyRef.current = onlyLocalhost
    if (prev === onlyLocalhost) return
    if (onlyLocalhost) {
      // false → true : nouvelle perte → l'overlay revient toujours (Q2)
      setCollapsed(false)
      setToast(null)
    } else {
      // true → false : retour du réseau
      setCollapsed(false)
      const ip = lastIpsRef.current[0]
      setToast(ip ? `Réseau rétabli — ${ip}` : 'Réseau rétabli')
    }
  }, [onlyLocalhost])

  // Toast de retour : 6 s.
  useEffect(() => {
    if (!toast) return undefined
    const id = setTimeout(() => setToast(null), NETWORK_TOAST_MS)
    return () => clearTimeout(id)
  }, [toast])

  // « Vérifié il y a N s » : horloge locale tant que l'alerte est affichée.
  useEffect(() => {
    if (!onlyLocalhost) return undefined
    setNow(Date.now())
    const id = setInterval(() => setNow(Date.now()), TICK_MS)
    return () => clearInterval(id)
  }, [onlyLocalhost])

  // Focus après repli : « Détails » (bouton d'origine) est démonté quand l'overlay s'ouvre, donc la
  // restitution du focus par le dialogue ne trouve plus son élément. Une fois le bandeau remonté on
  // redonne le focus à son bouton « Détails » — seulement si le focus est tombé sur <body>
  // (sinon l'élément précédent, encore présent, l'a déjà récupéré).
  const detailsRef = useRef(null)
  useEffect(() => {
    if (!(onlyLocalhost && collapsed)) return
    const active = document.activeElement
    if (!active || active === document.body) detailsRef.current?.focus()
  }, [onlyLocalhost, collapsed])

  const collapse = useCallback(() => setCollapsed(true), [])
  const reopen = useCallback(() => setCollapsed(false), [])

  const overlayOpen = onlyLocalhost && !collapsed

  return (
    <>
      {onlyLocalhost && collapsed && (
        <NetworkWarningBanner checkedAt={checkedAt} now={now} onDetails={reopen} detailsRef={detailsRef} />
      )}
      {overlayOpen && createPortal(
        <NetworkLostDialog ips={ips} ignored={ignored} checkedAt={checkedAt} now={now} onContinue={collapse} />,
        document.body
      )}
      {toast && (
        <div className="network-toast" role="status">
          <span aria-hidden="true">✅</span> {toast}
        </div>
      )}
    </>
  )
}

function NetworkLostDialog({ ips, ignored, checkedAt, now, onContinue }) {
  const dialogRef = useRef(null)
  const continueRef = useRef(null)
  const onContinueRef = useRef(onContinue)
  onContinueRef.current = onContinue

  // Focus piégé dans le dialogue, restitué à l'élément précédent à la fermeture.
  useEffect(() => {
    const previouslyFocused = document.activeElement
    continueRef.current?.focus()

    // Échap = « Continuer quand même ». Phase de capture + stopPropagation : l'overlay est au-dessus
    // de tout (dont la popup « Lancer la partie » #253) et consomme la touche.
    const onKeyDown = (e) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        e.stopPropagation()
        onContinueRef.current()
        return
      }
      if (e.key !== 'Tab' || !dialogRef.current) return
      const focusables = Array.from(dialogRef.current.querySelectorAll(FOCUSABLE))
      if (focusables.length === 0) { e.preventDefault(); return }
      const first = focusables[0]
      const last = focusables[focusables.length - 1]
      const active = document.activeElement
      const inside = dialogRef.current.contains(active)
      if (e.shiftKey && (active === first || !inside)) { e.preventDefault(); last.focus() }
      else if (!e.shiftKey && (active === last || !inside)) { e.preventDefault(); first.focus() }
    }
    document.addEventListener('keydown', onKeyDown, true)
    return () => {
      document.removeEventListener('keydown', onKeyDown, true)
      if (previouslyFocused && typeof previouslyFocused.focus === 'function' && document.contains(previouslyFocused)) {
        previouslyFocused.focus()
      }
    }
  }, [])

  const ago = formatCheckedAgo(checkedAt, now)

  return (
    <div className="network-overlay">
      <div
        ref={dialogRef}
        className="network-dialog"
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="no-net-title"
        aria-describedby="no-net-lead"
      >
        <div className="network-dialog-icon" aria-hidden="true">📡</div>
        <h3 id="no-net-title">Le serveur n'a pas de réseau disponible</h3>
        <p className="network-dialog-lead" id="no-net-lead">
          BuzzControl n'est joignable que depuis ce poste (<code>localhost</code>). Tant que cela dure,{' '}
          <strong>aucun buzzer, aucune tablette, ni la TV ni l'animateur</strong> ne peuvent se connecter :
          une partie lancée maintenant ne pourrait pas être jouée.
        </p>
        <dl className="network-facts">
          <dt>Adresses détectées</dt>
          <dd>
            {ips.length === 0
              ? <><strong>aucune</strong> — seulement <code>127.0.0.1</code></>
              : ips.map((ip, i) => <span key={ip}>{i > 0 && ', '}<code>{ip}</code></span>)}
          </dd>
          {ignored.length > 0 && (
            <>
              <dt>Interfaces ignorées</dt>
              <dd>
                {ignored.map((it, i) => (
                  <div key={`${it.NAME}-${it.IP}-${i}`}>
                    <code>{it.NAME}</code> {it.IP}{' '}
                    <small>(virtuelle — non joignable par les appareils)</small>
                  </div>
                ))}
              </dd>
            </>
          )}
          <dt>Dernière vérification</dt>
          <dd>{ago ? `${ago} — ` : ''}contrôle automatique toutes les 30 s</dd>
        </dl>
        <ul className="network-advice">
          <li>Branchez le câble réseau ou reconnectez le WiFi de ce poste.</li>
          <li>Cet écran disparaîtra de lui-même dès qu'une adresse réseau sera détectée.</li>
        </ul>
        <div className="network-actions">
          <span className="network-actions-note">
            Vous pouvez continuer à préparer le quiz : l'alerte restera visible en bandeau rouge jusqu'au retour du réseau.
          </span>
          <button ref={continueRef} type="button" className="network-btn-secondary" onClick={onContinue}>
            Continuer quand même
          </button>
        </div>
      </div>
    </div>
  )
}
