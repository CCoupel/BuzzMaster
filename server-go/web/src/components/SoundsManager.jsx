import { useState, useEffect, useCallback } from 'react'
import Button from './Button'
import SoundTestModal from './SoundTestModal'

// #230 — tableau des sept sons d'événement, onglet Son de /admin/ambiance.
// Maquette : docs/mockups/sound-config-230.html (rév. 4, §02/§03/§04/§05).
// Contrat : contracts/http-endpoints.md §Sound + contracts/sound.md §6.3.
//
// ⚠️ N'EST PAS une généralisation de BackgroundsManager.jsx : le modèle de
// données est différent (FileBank résout chaque son par `<cue>.wav`, le nom
// du fichier EST la cue — sept emplacements FIXES, pas une galerie). « Ajouter »
// n'existe pas ; « supprimer » n'a pas de sens (ça rendrait la cue muette) —
// l'action est « Restaurer ». Voir plan-dev-230 §3.1 pour la décision
// complète. Composant autonome : il s'auto-alimente (GET /api/sounds), il
// n'a besoin d'aucune prop.
//
// Styles : `.sound-*` dans AmbiancePage.css (aucun CSS propre à ce fichier —
// la seule page qui monte ce composant importe déjà AmbiancePage.css ;
// éviter de faire entrer ConfigPage.css comme le ferait BackgroundsManager).

const CUE_LABELS = {
  depart: { label: 'Départ', sub: 'Le chronomètre démarre' },
  'temps-ecoule': { label: 'Temps écoulé', sub: 'Le chronomètre arrive à zéro' },
  gagne: { label: 'Points gagnés', sub: 'La régie crédite une équipe' },
  perdu: { label: 'Erreur', sub: 'Paire ratée en MEMORY, réponse invalidée en RAFALE' },
  reveal: { label: 'Révélation', sub: 'La réponse est dévoilée' },
  'entracte-debut': { label: "Début d'entracte", sub: 'La pause commence' },
  'entracte-fin': { label: "Fin d'entracte", sub: 'La partie reprend' },
}

const EMPTY_VERDICT = Object.freeze({ local: 'untested', server: 'untested' })
const TOAST_MS = 3000

const postJson = (url, body) =>
  fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body ?? {}),
  })

const readJsonSafe = async (res) => { try { return await res.json() } catch { return {} } }

// Les endpoints Sound (B.4/B.5/POST .../restore-defaults) renvoient tous
// leurs erreurs via writeSoundError (http_sound.go) : un JSON
// `{status:"error", message:"..."}`, jamais du texte brut — lire `message`
// plutôt que `res.text()` (qui afficherait le JSON tel quel dans un toast).
const readErrorMessage = async (res, fallback) => {
  const body = await readJsonSafe(res)
  return body?.message || fallback
}

const formatDuration = (seconds) =>
  typeof seconds === 'number' ? `${seconds.toFixed(2).replace('.', ',')} s` : '—'

const labelFor = (cue) => CUE_LABELS[cue]?.label || cue

export default function SoundsManager() {
  const [sounds, setSounds] = useState({ phase: 'idle', cues: [] })
  const [busyCue, setBusyCue] = useState(null) // remplacement/restauration unitaire en vol
  const [restoringAll, setRestoringAll] = useState(false)
  const [testCue, setTestCue] = useState(null) // nom de la cue ouverte dans la modale de test
  // Verdicts manuels — ÉPHÉMÈRES (état React, jamais persistés — plan-delta
  // lotC §2) : { [cue]: { local, server } }, remis à zéro au rechargement
  // de la page ET effacés par un remplacement/une restauration (règle 4).
  const [verdicts, setVerdicts] = useState({})
  const [toast, setToast] = useState(null)

  useEffect(() => {
    if (!toast) return undefined
    const t = setTimeout(() => setToast(null), TOAST_MS)
    return () => clearTimeout(t)
  }, [toast])

  const loadSounds = useCallback(async () => {
    setSounds(prev => ({ ...prev, phase: 'loading' }))
    try {
      const res = await fetch('/api/sounds')
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const data = await readJsonSafe(res)
      setSounds({ phase: 'done', cues: Array.isArray(data.cues) ? data.cues : [] })
    } catch (error) {
      console.error('Load sounds failed:', error)
      setSounds({ phase: 'done', cues: [] })
      setToast({ message: 'Erreur de chargement des sons : ' + error.message, type: 'error' })
    }
  }, [])

  useEffect(() => { loadSounds() }, [loadSounds])

  const clearVerdicts = (cue) => {
    setVerdicts(prev => {
      if (!(cue in prev)) return prev
      const next = { ...prev }
      delete next[cue]
      return next
    })
  }

  // Interrupteur par cue (contract sound.md §6.3) : on stocke ce qui est
  // ÉTEINT, et le champ `cues_disabled` se REMPLACE en entier à chaque
  // écriture (pas de fusion interne au champ) — on reconstruit donc la carte
  // complète à partir de l'état actuellement affiché, où chaque ligne connaît
  // déjà son propre `enabled` (GET /api/sounds). Optimiste, comme
  // AmbiancePage/handlePreviewToggle pour l'éclairage : la case réagit tout
  // de suite, sans dépendre du résultat réseau ; rollback par rechargement
  // en cas d'échec.
  const handleToggleCue = async (cue, checked) => {
    const nextDisabled = {}
    sounds.cues.forEach(c => {
      const isChecked = c.cue === cue ? checked : c.enabled
      if (!isChecked) nextDisabled[c.cue] = true
    })
    setSounds(prev => ({
      ...prev,
      cues: prev.cues.map(c => (c.cue === cue ? { ...c, enabled: checked } : c)),
    }))
    try {
      const res = await postJson('/config.json', { sound: { cues_disabled: nextDisabled } })
      if (!res.ok) throw new Error(await res.text())
    } catch (error) {
      console.error('Toggle cue failed:', error)
      setToast({ message: 'Erreur : ' + error.message, type: 'error' })
      await loadSounds() // la config fait foi
    }
  }

  const handleReplace = async (cue, file) => {
    setBusyCue(cue)
    const fd = new FormData()
    fd.append('file', file)
    try {
      const res = await fetch(`/api/sounds/${cue}`, { method: 'POST', body: fd })
      if (!res.ok) {
        const message = await readErrorMessage(res, `Fichier refusé (HTTP ${res.status}).`)
        setToast({ message, type: 'error' })
        return
      }
      const body = await readJsonSafe(res)
      // Règle 4 (delta lotC §4) — un remplacement efface les deux verdicts :
      // ils portaient sur un fichier qui n'existe plus.
      clearVerdicts(cue)
      await loadSounds()
      setToast({
        message: body.warning ? `Son remplacé — ${body.warning}` : 'Son remplacé.',
        type: body.warning ? 'warning' : 'success',
      })
    } catch (error) {
      console.error('Replace sound failed:', error)
      setToast({ message: 'Erreur : ' + error.message, type: 'error' })
    } finally {
      setBusyCue(null)
    }
  }

  const handleRestore = async (cue) => {
    setBusyCue(cue)
    try {
      const res = await postJson(`/api/sounds/${cue}/restore`)
      if (!res.ok) throw new Error(await readErrorMessage(res, `HTTP ${res.status}`))
      clearVerdicts(cue)
      await loadSounds()
      setToast({ message: 'Son restauré.', type: 'success' })
    } catch (error) {
      console.error('Restore sound failed:', error)
      setToast({ message: 'Erreur : ' + error.message, type: 'error' })
    } finally {
      setBusyCue(null)
    }
  }

  // Restauration globale — endpoint déjà livré en #229. Confirmation
  // nommant les sons personnalisés qui seront écrasés (maquette §05),
  // `window.confirm` comme le reste du projet (AmbiancePage.handleUnpair,
  // handleResetToLibre) plutôt qu'une modale dédiée.
  const handleRestoreAll = async () => {
    const customNames = sounds.cues.filter(c => c.custom).map(c => labelFor(c.cue))
    const message = customNames.length > 0
      ? `Restaurer les sept sons par défaut ? ${customNames.length} son${customNames.length > 1 ? 's' : ''} ` +
        `personnalisé${customNames.length > 1 ? 's' : ''} seront remplacés par les sons d'origine : ` +
        `${customNames.join(', ')}. Cette action ne peut pas être annulée.`
      : 'Restaurer les sept sons par défaut ? Cette action ne peut pas être annulée.'
    if (!window.confirm(message)) return
    setRestoringAll(true)
    try {
      const res = await postJson('/api/sounds/restore-defaults')
      if (!res.ok) throw new Error(await readErrorMessage(res, `HTTP ${res.status}`))
      setVerdicts({})
      await loadSounds()
      setToast({ message: 'Les sept sons ont été restaurés.', type: 'success' })
    } catch (error) {
      console.error('Restore all sounds failed:', error)
      setToast({ message: 'Erreur : ' + error.message, type: 'error' })
    } finally {
      setRestoringAll(false)
    }
  }

  const testingRow = sounds.cues.find(c => c.cue === testCue) || null

  return (
    <div className="sound-manager">
      <div className="sound-table-scroll">
        <table className="sound-table">
          <thead>
            <tr>
              <th>Moment du jeu</th>
              <th className="sound-col-center">Actif</th>
              <th>Son</th>
              <th className="sound-col-right">Durée</th>
              <th className="sound-col-center">Test</th>
              <th className="sound-col-right">Actions</th>
            </tr>
          </thead>
          <tbody>
            {sounds.phase === 'loading' && sounds.cues.length === 0 && (
              <tr><td colSpan={6} className="sound-hint-row">Chargement…</td></tr>
            )}
            {sounds.phase === 'done' && sounds.cues.length === 0 && (
              <tr><td colSpan={6} className="sound-hint-row">Aucun son disponible.</td></tr>
            )}
            {sounds.cues.map(row => {
              const meta = CUE_LABELS[row.cue] || { label: row.cue, sub: '' }
              const v = verdicts[row.cue] || EMPTY_VERDICT
              return (
                <tr key={row.cue} className={!row.enabled ? 'is-off' : ''}>
                  <td>
                    <b>{meta.label}</b>
                    <span className="sound-sub">{meta.sub}</span>
                  </td>
                  <td className="sound-col-center">
                    <label className="sound-switch sound-switch-sm">
                      <input
                        type="checkbox"
                        checked={row.enabled}
                        onChange={e => handleToggleCue(row.cue, e.target.checked)}
                        aria-label={`Activer ${meta.label}`}
                      />
                      <span className="sound-switch-track" aria-hidden="true" />
                    </label>
                  </td>
                  <td>
                    <span className={`sound-tag ${row.custom ? 'is-custom' : ''}`}>
                      {row.custom ? 'Personnalisé' : 'Défaut'}
                    </span>
                  </td>
                  <td className="sound-col-right sound-mono">{formatDuration(row.duration_seconds)}</td>
                  <td className="sound-col-center">
                    <span className="sound-verdict-dots">
                      <span className={`sound-verdict-dot is-${v.local}`} title="Écoute locale" />
                      <span className={`sound-verdict-dot is-${v.server}`} title="Écoute serveur" />
                    </span>
                  </td>
                  <td>
                    <div className="sound-row-actions">
                      {/* Un moment éteint reste testable — tester est un
                          geste explicite, seule LA PARTIE est muette pour
                          cette cue (maquette §02, "Un moment éteint peut
                          quand même être testé"). */}
                      <Button variant="secondary" size="sm" onClick={() => setTestCue(row.cue)}>
                        ▶ Tester
                      </Button>
                      <label className="sound-replace-btn">
                        <input
                          type="file"
                          accept=".wav,audio/wav,audio/x-wav"
                          onChange={e => {
                            const file = e.target.files?.[0]
                            e.target.value = ''
                            if (file) handleReplace(row.cue, file)
                          }}
                          style={{ display: 'none' }}
                        />
                        <Button variant="secondary" size="sm" as="span" loading={busyCue === row.cue}>
                          ⬆ Remplacer
                        </Button>
                      </label>
                      {/* « Restaurer » n'apparaît que sur les sons
                          personnalisés (maquette §02) — un son déjà par
                          défaut n'a rien à restaurer. */}
                      {row.custom && (
                        <Button
                          variant="ghost"
                          size="sm"
                          loading={busyCue === row.cue}
                          onClick={() => handleRestore(row.cue)}
                        >
                          ↺ Restaurer
                        </Button>
                      )}
                    </div>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>

      <div className="ambiance-actions sound-actions">
        <Button variant="ghost" onClick={handleRestoreAll} loading={restoringAll}>
          ↺ Restaurer tous les sons par défaut
        </Button>
      </div>
      <p className="ambiance-hint">
        Colonne <b>Test</b> : deux pastilles, écoute locale puis écoute serveur — grise « pas
        testé », verte « ok », rouge « ko ». Elles font de ce tableau la liste de contrôle d&rsquo;une
        séance de réglage.
        <br />Format accepté : WAV, 44 100 Hz, 16 bits, stéréo — 5 secondes au maximum.
      </p>

      {testingRow && (
        <SoundTestModal
          cue={testingRow}
          label={labelFor(testingRow.cue)}
          verdict={verdicts[testingRow.cue] || EMPTY_VERDICT}
          onSetVerdict={(section, value) =>
            setVerdicts(prev => ({
              ...prev,
              [testingRow.cue]: { ...(prev[testingRow.cue] || EMPTY_VERDICT), [section]: value },
            }))
          }
          onClose={() => setTestCue(null)}
        />
      )}

      {toast && (
        <div className={`wifi-toast wifi-toast-${toast.type}`} role="status">
          {toast.message}
        </div>
      )}
    </div>
  )
}
