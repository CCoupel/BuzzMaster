import { useState, useRef, useEffect } from 'react'
import Button from './Button'

// #230 (Lot C, delta rév. 4) — modale de test d'un son, deux écoutes
// distinctes, deux verdicts manuels. Styles : `.sound-test-*` dans
// AmbiancePage.css (aucun CSS propre à ce fichier — même discipline que
// BackgroundsManager/ConfigPage, mais centralisé ici plutôt que sur une
// page tierce, cf. plan §3.1). Contrat : contracts/http-endpoints.md §Sound
// (POST /api/sounds/{cue}/test) ; `path` (écoute locale) vient de
// GET /api/sounds, déjà servi par handleFiles sans restriction de
// sous-dossier — aucun endpoint dédié.
//
// QUATRE règles non négociables (handoff #230 §4, plan-delta-lotC) :
//   1. Rien ne se joue à l'ouverture — chaque écoute est un clic explicite.
//      /admin/ambiance reste atteignable PENDANT une partie : un
//      déclenchement automatique enverrait un bruitage dans la salle.
//   2. Le verdict n'est JAMAIS dérivé de la réponse serveur. `result:
//      "played"` signifie « confiée au moteur », jamais « entendue » — seul
//      l'utilisateur, après avoir écouté, pose le verdict.
//   3. Les verdicts sont éphémères (état React porté par SoundsManager),
//      remis à zéro au rechargement — ni serveur, ni localStorage.
//   4. Remplacer/restaurer un son efface ses deux verdicts (fait par
//      l'appelant, SoundsManager — pas cette modale, qui ne fait qu'afficher
//      et modifier le verdict de la cue actuellement ouverte).

const VERDICT_OPTIONS = [
  { key: 'untested', label: 'Pas testé' },
  { key: 'ok', label: 'Ok' },
  { key: 'ko', label: 'Ko' },
]

// Taxonomie normative à trois issues (contract §Sound, POST /.../test) —
// aucune n'est une erreur HTTP, le corps de la réponse porte la distinction.
const SERVER_RESULT_LABEL = {
  played: 'Son envoyé à l’enceinte',
  disabled: 'Bruitages désactivés — rien n’a été joué',
  unavailable: 'Enceinte indisponible — rien n’a été joué',
}

const postAction = (url) => fetch(url, { method: 'POST' })
const readJsonSafe = async (res) => { try { return await res.json() } catch { return {} } }

const formatDuration = (seconds) =>
  typeof seconds === 'number' ? `${seconds.toFixed(2).replace('.', ',')} s` : '—'

function VerdictSeg({ value, onChange, ariaLabel }) {
  return (
    <span className="sound-test-seg" role="group" aria-label={ariaLabel}>
      {VERDICT_OPTIONS.map(opt => (
        <button
          key={opt.key}
          type="button"
          className={`sound-test-seg-opt ${value === opt.key ? `sel is-${opt.key}` : ''}`}
          aria-pressed={value === opt.key}
          onClick={() => onChange(opt.key)}
        >
          {opt.label}
        </button>
      ))}
    </span>
  )
}

export default function SoundTestModal({ cue, label, verdict, onSetVerdict, onClose }) {
  const [localMessage, setLocalMessage] = useState(null)
  const [serverMessage, setServerMessage] = useState(null)
  const [serverTesting, setServerTesting] = useState(false)
  const audioRef = useRef(null)

  // Fermeture via Échap, cohérent avec les autres modales du projet
  // (ApiKeyHelpModal, USBConfigModal).
  useEffect(() => {
    const handleKey = (e) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [onClose])

  // Règle 1 — AUCUNE lecture automatique : ce handler n'est jamais appelé
  // ailleurs qu'au clic explicite du bouton ci-dessous.
  const handlePlayLocal = () => {
    setLocalMessage(null)
    const audio = audioRef.current
    if (!audio) return
    audio.currentTime = 0
    audio.play().catch(error => {
      console.error('Local sound playback failed:', error)
      setLocalMessage("Impossible de lire ce fichier dans le navigateur.")
    })
  }

  // Règle 2 — la réponse alimente UNIQUEMENT `serverMessage` (informatif) :
  // jamais `onSetVerdict`. Le verdict reste un geste exclusivement manuel.
  const handlePlayServer = async () => {
    setServerTesting(true)
    setServerMessage(null)
    try {
      const res = await postAction(`/api/sounds/${cue.cue}/test`)
      const body = await readJsonSafe(res)
      if (!res.ok) {
        setServerMessage(`Test impossible (HTTP ${res.status}).`)
        return
      }
      setServerMessage(SERVER_RESULT_LABEL[body.result] || `Réponse inattendue (${body.result}).`)
    } catch (error) {
      console.error('Server sound test failed:', error)
      setServerMessage('Erreur : ' + error.message)
    } finally {
      setServerTesting(false)
    }
  }

  return (
    <div className="sound-test-overlay" onClick={(e) => e.target === e.currentTarget && onClose()}>
      <div className="sound-test-modal" role="dialog" aria-modal="true" aria-labelledby="sound-test-title">
        <div className="sound-test-header">
          <b id="sound-test-title">Tester le son</b>
          <span className="sound-test-cue ambiance-mono">
            {cue.cue}.wav · {formatDuration(cue.duration_seconds)} · {cue.custom ? 'personnalisé' : 'défaut'}
          </span>
        </div>

        <div className="sound-test-body">
          <section className="sound-test-section">
            <div className="sound-test-section-hd">
              <b>1 · Écoute locale</b>
              <Button variant="secondary" size="sm" onClick={handlePlayLocal}>▶ Jouer dans ce navigateur</Button>
            </div>
            <p>
              Vérifie <b>le fichier</b> : est-il audible, au bon volume, sans coupure ni
              saturation ? Le son sort des haut-parleurs de cet appareil.
            </p>
            {/* preload="none" + aucun autoPlay : rien ne charge ni ne joue
                avant le clic ci-dessus (règle 1). */}
            <audio
              ref={audioRef}
              src={cue.path}
              preload="none"
              onError={() => setLocalMessage("Impossible de lire ce fichier dans le navigateur.")}
            />
            {localMessage && <p className="sound-test-message is-error">{localMessage}</p>}
            <div className="sound-test-section-ft">
              <span className="sound-test-verdict-label">Ce que j&rsquo;ai entendu&nbsp;:</span>
              <VerdictSeg
                value={verdict.local}
                onChange={v => onSetVerdict('local', v)}
                ariaLabel={`Verdict écoute locale, ${label}`}
              />
            </div>
          </section>

          <section className="sound-test-section">
            <div className="sound-test-section-hd">
              <b>2 · Écoute serveur</b>
              <Button variant="secondary" size="sm" onClick={handlePlayServer} loading={serverTesting}>
                🔊 Jouer sur l&rsquo;enceinte
              </Button>
            </div>
            <p>
              Vérifie <b>toute la chaîne</b> : le serveur joue le son sur l&rsquo;enceinte de la
              salle, par le même chemin qu&rsquo;en partie. Il faut être à portée d&rsquo;oreille
              pour l&rsquo;entendre.
            </p>
            {serverMessage && <p className="sound-test-message">{serverMessage}</p>}
            <div className="sound-test-section-ft">
              <span className="sound-test-verdict-label">Ce que j&rsquo;ai entendu&nbsp;:</span>
              <VerdictSeg
                value={verdict.server}
                onChange={v => onSetVerdict('server', v)}
                ariaLabel={`Verdict écoute serveur, ${label}`}
              />
            </div>
          </section>
        </div>

        <div className="sound-test-footer">
          <Button variant="ghost" onClick={onClose}>Fermer</Button>
        </div>
      </div>
    </div>
  )
}
