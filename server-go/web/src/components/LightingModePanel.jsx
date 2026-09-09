import { useState, useEffect } from 'react'
import { useLightingStatus } from '../hooks/useLightingStatus'
import { normalizeLightingState } from '../utils/lightingState'
import './LightingModePanel.css'

// #208 (v10.0.0, Batch 3, déplacé de AmbiancePage.jsx vers GamePage.jsx sur
// demande utilisateur — 2026-09-07) — panneau de conduite en direct pour la
// régie : sélecteur tri-état ON/AUTO/OFF + bascule Flash, tous deux scopés
// à la zone `general` UNIQUEMENT (jamais les ampoules d'équipe, contrat
// §10.1 encart normatif). Contrat : contracts/lighting.md §10.1
// (SHA df448318). Maquettes : docs/mockups/lighting-team-assignment-213.html
// (rev6), docs/mockups/lighting-priority-208.md (rev3).
//
// Composant AUTONOME et réutilisable (aucune prop requise) : gère son
// propre `useLightingStatus()` — même principe que Navbar/AmbiancePage,
// chacune avec sa propre instance du hook (voir le commentaire de ce hook).
// Se rend invisible tant que l'éclairage n'est pas configuré (state
// `disabled`, dérivé de `h.Lighting == nil` côté serveur — même garde que
// l'étape 3 de AmbiancePage.jsx avant ce déplacement) : aucune trace sur une
// installation sans pont Hue, même sur l'écran de jeu.
//
// Historique : d'abord affiché sur /admin/ambiance (Batch 3), déplacé ici
// car ce sont des outils de conduite EN DIRECT utilisés par la régie
// PENDANT une partie — l'écran qu'elle a ouvert en séance, pas un écran de
// configuration séparé qu'elle n'a aucune raison de rouvrir en cours de jeu.
//
// Le bandeau d'avertissement permanent (garde-fou "mode oublié", R8 —
// _work/reports/planner-v10-etat-courant-20260907.md §3) reste local à ce
// panneau : il est donc visible tant que l'admin a GamePage ouvert, ce qui
// couvre l'usage réel (c'est l'écran ouvert pendant une partie). Ce n'est
// PAS un bandeau global multi-pages — si l'admin navigue vers un autre
// écran admin pendant qu'un mode reste engagé, il ne le voit plus tant
// qu'il ne revient pas sur GamePage ou Ambiance. Signalé au CDP comme point
// à trancher si une garde-fou cross-page est souhaitée (précédent existant :
// le bouton ENTRACTE manuel vit dans Navbar.jsx, visible sur tout /admin/*).

const postJson = (url, body) =>
  fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body ?? {}),
  })

const TOAST_MS = 3000

export default function LightingModePanel({ className = '' }) {
  const { status, refresh: refreshStatus } = useLightingStatus()
  const [modeBusy, setModeBusy] = useState(false)
  const [flashBusy, setFlashBusy] = useState(false)
  const [toast, setToast] = useState(null)

  useEffect(() => {
    if (!toast) return undefined
    const t = setTimeout(() => setToast(null), TOAST_MS)
    return () => clearTimeout(t)
  }, [toast])

  // Aucun état optimiste : après la réponse serveur, on relit le statut
  // (refreshStatus) — la position affichée est TOUJOURS celle que le
  // serveur vient de confirmer, jamais une supposition côté client
  // (contrat §10.1.1 pt.6, « propriété serveur »).
  const handleSetMode = async (mode) => {
    if (modeBusy || status.mode === mode) return
    setModeBusy(true)
    try {
      const res = await postJson('/api/lighting/mode', { mode })
      if (!res.ok) throw new Error(await res.text())
      await refreshStatus()
    } catch (error) {
      console.error('Set lighting mode failed:', error)
      setToast({ message: 'Erreur : ' + error.message, type: 'error' })
    } finally {
      setModeBusy(false)
    }
  }

  const handleToggleFlash = async () => {
    if (flashBusy) return
    const next = !status.flash
    setFlashBusy(true)
    try {
      const res = await postJson('/api/lighting/flash', { on: next })
      if (!res.ok) throw new Error(await res.text())
      await refreshStatus()
    } catch (error) {
      console.error('Set lighting flash failed:', error)
      setToast({ message: 'Erreur : ' + error.message, type: 'error' })
    } finally {
      setFlashBusy(false)
    }
  }

  // Invisible tant que l'éclairage n'est pas configuré — aucun appel réseau
  // supplémentaire au-delà du GET /api/lighting/status déjà interrogé par
  // le hook (même ligne de conduite que #207 : « aucune goroutine, aucun
  // appel réseau » quand non configuré, ici transposée à « aucun élément
  // d'interface »).
  if (normalizeLightingState(status.state) === 'disabled') return null

  return (
    <div className={`lighting-mode-panel ${className}`}>
      <h3 className="lighting-mode-title">Éclairage général — conduite en direct</h3>
      <div className="lighting-mode-row">
        <div className="lighting-mode-tristate" role="radiogroup" aria-label="Éclairage général">
          {['ON', 'AUTO', 'OFF'].map(m => (
            <button
              key={m}
              type="button"
              role="radio"
              aria-checked={status.mode === m}
              className={`lighting-mode-btn is-${m.toLowerCase()} ${status.mode === m ? 'is-selected' : ''}`}
              disabled={modeBusy || status.mode === m}
              onClick={() => handleSetMode(m)}
            >
              {m}
            </button>
          ))}
        </div>
        <button
          type="button"
          className={`lighting-flash-toggle ${status.flash ? 'is-on' : ''}`}
          aria-pressed={status.flash}
          disabled={flashBusy}
          onClick={handleToggleFlash}
        >
          <span className="lighting-flash-switch" aria-hidden="true" />
          Flash
        </button>
      </div>
      {status.mode !== 'AUTO' && (
        <p className="lighting-mode-warning" role="status">
          Mode <strong>{status.mode}</strong> engagé — l'éclairage général restera
          {status.mode === 'ON' ? ' allumé' : ' éteint'} indéfiniment, même pendant une partie,
          jusqu'à un retour manuel sur AUTO.
        </p>
      )}
      {toast && (
        <div className={`wifi-toast wifi-toast-${toast.type}`} role="status">
          {toast.message}
        </div>
      )}
    </div>
  )
}
