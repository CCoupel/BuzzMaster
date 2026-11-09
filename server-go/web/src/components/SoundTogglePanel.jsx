import { useState } from 'react'
import { notifySoundChanged } from '../hooks/useSoundStatus'
import { isRoundScope, ROUND_SCOPE_MESSAGE } from '../utils/ambianceScope'

// #286 — section « Son » du popover « Ambiance » : sélecteur ON / OFF à DEUX
// positions (le son n'a pas d'état automatique). Écrit `sound.muted` par
// POST /config.json {"sound":{"muted":bool}} (contracts/sound.md §11.2) —
// aucun endpoint dédié. Jamais de mise à jour optimiste : la position
// affichée est celle que le serveur confirme (refresh() après réponse).
// `muted` = on stocke ce qui est coupé : ON ⇔ !muted.

export default function SoundTogglePanel({ muted, refresh, source = 'global', roundActive = false }) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState(null)
  const [notice, setNotice] = useState(null)

  const setMuted = async (next) => {
    if (busy || muted === next) return
    setBusy(true)
    setError(null)
    setNotice(null)
    try {
      const res = await fetch('/config.json', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ sound: { muted: next } }),
      })
      if (!res.ok) throw new Error(await res.text())
      if (await isRoundScope(res)) setNotice(ROUND_SCOPE_MESSAGE)
      notifySoundChanged()
      if (refresh) await refresh()
    } catch (e) {
      console.error('Set sound muted failed:', e)
      setError('Erreur : ' + e.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="sound-toggle-panel">
      <p className="navbar-popover-section-title">
        Son{source === 'round' && <span className="ambiance-source-tag"> manche</span>}
      </p>
      <div className="sound-toggle-row">
        <div className="lighting-mode-tristate" role="radiogroup" aria-label="Son">
          <button
            type="button"
            role="radio"
            aria-checked={!muted}
            className={`lighting-mode-btn is-sound-on ${!muted ? 'is-selected' : ''}`}
            disabled={busy || !muted}
            onClick={() => setMuted(false)}
          >
            ON
          </button>
          <button
            type="button"
            role="radio"
            aria-checked={!!muted}
            className={`lighting-mode-btn is-off ${muted ? 'is-selected' : ''}`}
            disabled={busy || !!muted}
            onClick={() => setMuted(true)}
          >
            OFF
          </button>
        </div>
        <span className="navbar-popover-hint">
          {source === 'direct' && roundActive
            ? 'Réglé par un geste direct — vaut jusqu\'au prochain changement de manche (la manche ne fixe pas le son : réglage global mis à jour).'
            : source === 'round'
              ? 'Imposé par la manche courante — un geste ici l\'emporte jusqu\'au prochain changement de manche (le réglage global n\'est pas modifié).'
              : 'Bruitages, média de question et son de décompte.'}
        </span>
      </div>
      {muted && (
        <p className="lighting-mode-warning" role="status">
          Son <strong>coupé</strong> — toutes les sorties audio du serveur restent muettes, même pendant une
          partie <strong>et après un redémarrage</strong>, jusqu'à un retour manuel sur ON. Un média de question
          en cours est arrêté ; le chronomètre différé démarre immédiatement.
        </p>
      )}
      {notice && <p className="lighting-mode-warning" role="status">{notice}</p>}
      {error && <p className="lighting-mode-warning is-error" role="alert">{error}</p>}
    </div>
  )
}
