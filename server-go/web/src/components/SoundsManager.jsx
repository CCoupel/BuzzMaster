import { useState, useEffect, useCallback } from 'react'
import Button from './Button'
import { soundUploadMessage } from '../utils/soundState'
import SoundTestModal from './SoundTestModal'
import AmbianceSettingsPanel from './AmbianceSettingsPanel'
import { CUE_LABELS } from '../utils/soundCues'
import { useSoundStatus, notifySoundChanged } from '../hooks/useSoundStatus'

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


// #287/#288 (v11.4.0, LOT-3) — sons LONGS (≤ 30 s), table à part : le décompte du chronomètre n'est PAS
// une cue (CUE_LABELS reste inchangé, soundCueFromString('decompte') est faux côté serveur). Contrat :
// sound.md §12/§13, http-endpoints.md §Sound ; maquette admin_sounds_manager__countdown_loop §01.
const LONG_LABELS = {
  decompte: { label: 'Décompte du chronomètre', sub: 'Pendant que le chronomètre décompte, en boucle ou une seule fois' },
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

const labelFor = (cue) => CUE_LABELS[cue]?.label || LONG_LABELS[cue]?.label || cue

export default function SoundsManager() {
  const [sounds, setSounds] = useState({ phase: 'idle', cues: [], long: [] })
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
      setSounds({
        phase: 'done',
        cues: Array.isArray(data.cues) ? data.cues : [],
        long: Array.isArray(data.long) ? data.long : [], // absent sur un serveur antérieur à v11.4.0
      })
    } catch (error) {
      console.error('Load sounds failed:', error)
      setSounds({ phase: 'done', cues: [], long: [] })
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

  // Décompte (#287) — optimiste puis rechargement en cas d'échec, patch champ par champ
  // (clé présente ⇒ écrite côté serveur : on n'envoie JAMAIS l'autre clé).
  const patchLong = async (name, patch, sound) => {
    setSounds(prev => ({ ...prev, long: prev.long.map(l => (l.name === name ? { ...l, ...patch } : l)) }))
    try {
      const res = await postJson('/config.json', { sound })
      if (!res.ok) throw new Error(await res.text())
    } catch (error) {
      console.error('Update long sound config failed:', error)
      setToast({ message: 'Erreur : ' + error.message, type: 'error' })
      await loadSounds() // la config fait foi
    }
  }
  const handleToggleLong = (name, checked) => patchLong(name, { enabled: checked }, { countdown_disabled: !checked })
  const handleSetLoop = (name, loop) => patchLong(name, { loop }, { countdown_once: !loop })

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
        message: soundUploadMessage('Son remplacé', body),
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
    const nCues = sounds.cues.length || 7
    const customNames = [
      ...sounds.cues.filter(c => c.custom).map(c => labelFor(c.cue)),
      ...sounds.long.filter(l => l.custom).map(l => labelFor(l.name)),
    ]
    const message = customNames.length > 0
      ? `Restaurer les ${nCues} bruitages et le décompte par défaut ? ${customNames.length} son${customNames.length > 1 ? 's' : ''} ` +
        `personnalisé${customNames.length > 1 ? 's' : ''} seront remplacés par les sons d'origine : ` +
        `${customNames.join(', ')}. Cette action ne peut pas être annulée.`
      : `Restaurer les ${nCues} bruitages et le décompte par défaut ? Cette action ne peut pas être annulée.`
    if (!window.confirm(message)) return
    setRestoringAll(true)
    try {
      const res = await postJson('/api/sounds/restore-defaults')
      if (!res.ok) throw new Error(await readErrorMessage(res, `HTTP ${res.status}`))
      setVerdicts({})
      await loadSounds()
      setToast({ message: `Les ${nCues} bruitages et le décompte ont été restaurés.`, type: 'success' })
    } catch (error) {
      console.error('Restore all sounds failed:', error)
      setToast({ message: 'Erreur : ' + error.message, type: 'error' })
    } finally {
      setRestoringAll(false)
    }
  }

  // Les sons longs sont adaptés à la forme `cue` attendue par SoundTestModal (cue.cue = nom du fichier,
  // d'où POST /api/sounds/decompte/test et `<audio src=path>` sans autre changement de la modale).
  const longAsCue = sounds.long.map(l => ({ ...l, cue: l.name }))
  const testingRow = sounds.cues.find(c => c.cue === testCue) || longAsCue.find(c => c.cue === testCue) || null

  // #231/#290 — l'écran Son est hébergé par le composant mutualisé (mode global).
  // Les réglages (Son coupé, interrupteurs de cues) passent par `value`/`onChange` ;
  // les colonnes catalogue (fichier, durée, Remplacer/Restaurer/Tester) restent ICI.
  // Section « Lumière » masquée (backend lighting.default_mode : LOT-4) ; le décompte
  // garde ses commandes dans sa ligne « Sons longs » (comportement v11.4.0 inchangé).
  const { status: soundStatus } = useSoundStatus()
  const panelValue = {
    sound: {
      muted: soundStatus.muted === true,
      cues: Object.fromEntries(sounds.cues.map(c => [c.cue, c.enabled])),
    },
  }
  const handlePanelChange = async (field, value) => {
    if (field === 'sound.muted') {
      try {
        const res = await postJson('/config.json', { sound: { muted: value } })
        if (!res.ok) throw new Error(await res.text())
        notifySoundChanged()
      } catch (error) {
        console.error('Set sound muted failed:', error)
        setToast({ message: 'Erreur : ' + error.message, type: 'error' })
      }
    } else if (field.startsWith('sound.cues.')) {
      await handleToggleCue(field.slice('sound.cues.'.length), value)
    }
  }
  const emptyRow = sounds.phase === 'loading'
    ? <tr><td colSpan={7} className="sound-hint-row">Chargement…</td></tr>
    : sounds.phase === 'done'
      ? <tr><td colSpan={7} className="sound-hint-row">Aucun son disponible.</td></tr>
      : null
  const CATALOGUE_COLUMNS = [
    <th key="lecture">Lecture</th>,
    <th key="son">Son</th>,
    <th key="duree" className="sound-col-right">Durée</th>,
    <th key="test" className="sound-col-center">Test</th>,
    <th key="actions" className="sound-col-right">Actions</th>,
  ]
  const renderCatalogue = (cue) => {
    const row = sounds.cues.find(c => c.cue === cue)
    if (!row) return null
    const v = verdicts[row.cue] || EMPTY_VERDICT
    return (
      <>
              <td className="sound-mono">—</td>
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

      </>
    )
  }
  const renderLongRows = (tabKey) => {
    if (tabKey !== '') return null
    return (
      <>
        {sounds.long.length > 0 && (
          <tr className="sound-section-row"><th colSpan={7} scope="colgroup">Sons longs (≤ 30 s)</th></tr>
        )}
        {sounds.long.map(row => {
          const meta = LONG_LABELS[row.name] || { label: row.name, sub: '' }
          const v = verdicts[row.name] || EMPTY_VERDICT
          return (
            <tr key={row.name} className={!row.enabled ? 'is-off' : ''}>
              <td>
                <b>{meta.label}</b>
                <span className="sound-sub">{meta.sub}</span>
              </td>
              <td className="sound-col-center">
                <label className="sound-switch sound-switch-sm">
                  <input
                    type="checkbox"
                    checked={row.enabled}
                    onChange={e => handleToggleLong(row.name, e.target.checked)}
                    aria-label={`Activer ${meta.label}`}
                  />
                  <span className="sound-switch-track" aria-hidden="true" />
                </label>
              </td>
              <td>
                <span className="sound-loop-seg" role="group" aria-label="Lecture du décompte">
                  {[{ loop: false, label: 'Une fois' }, { loop: true, label: 'En boucle' }].map(opt => (
                    <button
                      key={opt.label}
                      type="button"
                      className={`sound-loop-seg-opt ${row.loop === opt.loop ? 'sel' : ''}`}
                      aria-pressed={row.loop === opt.loop}
                      onClick={() => row.loop !== opt.loop && handleSetLoop(row.name, opt.loop)}
                    >
                      {opt.label}
                    </button>
                  ))}
                </span>
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
                  <Button variant="secondary" size="sm" onClick={() => setTestCue(row.name)}>
                    ▶ Tester
                  </Button>
                  <label className="sound-replace-btn">
                    <input
                      type="file"
                      accept=".wav,audio/wav,audio/x-wav"
                      onChange={e => {
                        const file = e.target.files?.[0]
                        e.target.value = ''
                        if (file) handleReplace(row.name, file)
                      }}
                      style={{ display: 'none' }}
                    />
                    <Button variant="secondary" size="sm" as="span" loading={busyCue === row.name}>
                      ⬆ Remplacer
                    </Button>
                  </label>
                  {row.custom && (
                    <Button
                      variant="ghost"
                      size="sm"
                      loading={busyCue === row.name}
                      onClick={() => handleRestore(row.name)}
                    >
                      ↺ Restaurer
                    </Button>
                  )}
                </div>
              </td>
            </tr>
          )
        })}

      </>
    )
  }

  return (
    <div className="sound-manager">
      <AmbianceSettingsPanel
        mode="global"
        value={panelValue}
        onChange={handlePanelChange}
        cues={sounds.cues}
        sections={['sound', 'cues']}
        catalogueColumns={CATALOGUE_COLUMNS}
        renderCatalogue={renderCatalogue}
        renderExtraRows={renderLongRows}
        emptyRow={emptyRow}
      />

      <div className="ambiance-actions sound-actions">
        <Button variant="ghost" onClick={handleRestoreAll} loading={restoringAll}>
          ↺ Restaurer tous les sons par défaut
        </Button>
      </div>
      <p className="ambiance-hint">
        Colonne <b>Test</b> : deux pastilles, écoute locale puis écoute serveur — grise « pas
        testé », verte « ok », rouge « ko ». Elles font de ce tableau la liste de contrôle d&rsquo;une
        séance de réglage.
        <br />Format accepté : WAV PCM ou flottant 32 bits, mono ou stéréo, 8 000 à 192 000 Hz — ramené automatiquement à 44 100 Hz, stéréo, 16 bits à l'envoi — 5 secondes au maximum (30 secondes pour les sons longs).
        <br />« Tester » joue le son une fois, même s&rsquo;il est désactivé. Restaurer tout ({sounds.cues.length || 7} bruitages + décompte) remet les sons d&rsquo;origine.
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
