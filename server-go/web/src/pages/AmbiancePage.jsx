import { useState, useEffect, useCallback, useMemo, useRef } from 'react'
import Button from '../components/Button'
import Card from '../components/Card'
import { useLightingStatus, notifyLightingChanged } from '../hooks/useLightingStatus'
import { lightingStateLabel, normalizeLightingState } from '../utils/lightingState'
import './AmbiancePage.css'

// #207 — /admin/ambiance : connecter BuzzMaster à un pont Philips Hue et
// choisir les ampoules pilotées. Maquette validée (rév. 4) :
// docs/mockups/lighting-hue-config-207.html. Contrat : contracts/hue-bridge.md
// §6 (schéma `lighting` de config.json), §7 (endpoints), §5.6 (taxonomie).
//
// Trois étapes, UNE SEULE visible à la fois (maquette §08-1) :
//   1. trouver le pont      (POST /api/lighting/discover, repli saisie IP)
//   2. l'associer           (POST /api/lighting/register en boucle, appui bouton)
//   3. choisir et tester    (GET /api/lighting/lights, POST /api/lighting/test)
//
// Points non négociables (handoff) :
//   - l'attente « appuyez sur le bouton » est EN LIGNE, pas en modale ;
//   - AUCUN champ de saisie de clé : elle s'obtient par l'appui bouton ;
//   - « bouton non pressé » (Hue 101 -> 409) est un cas NOMINAL, pas une panne ;
//   - toutes les ampoules cochées par défaut mais toutes AFFICHÉES ;
//   - badge à QUATRE valeurs, « injoignable » et « refusée » jamais fondues.
//
// La section de config se nomme `lighting`, jamais `ambiance` (mot déjà pris
// par la catégorie de sauvegarde de game-config.json, BackupPage.jsx/#152).

export const REGISTER_RETRY_MS = 2000
export const REGISTER_TIMEOUT_S = 45
const TOAST_MS = 3000

const EMPTY_LIGHTING = Object.freeze({
  enabled: false,
  bridge_ip: '',
  bridge_id: '',
  api_key_configured: false,
  lights: [],
})

const postJson = (url, body) =>
  fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body ?? {}),
  })

const saveLighting = (lighting) => postJson('/config.json', { lighting })

const readJsonSafe = async (res) => {
  try { return await res.json() } catch { return {} }
}

// Taxonomie §5.6 appliquée à une réponse d'inventaire/test : trois issues.
function classifyFailure(res, body) {
  if (res.status === 503 || body?.result === 'unreachable') return 'unreachable'
  if (res.status === 401 || res.status === 409 || body?.result === 'refused') return 'refused'
  return 'error'
}

export default function AmbiancePage() {
  const { status, refresh: refreshStatus } = useLightingStatus()

  // Section `lighting` de config.json (clé API jamais présente : masquée).
  const [lighting, setLighting] = useState(EMPTY_LIGHTING)
  const [configLoaded, setConfigLoaded] = useState(false)

  // Étape 1 — découverte.
  const [discovery, setDiscovery] = useState({ phase: 'idle', bridges: [] })
  const [selectedBridge, setSelectedBridge] = useState(null)
  const [manualOpen, setManualOpen] = useState(false)
  const [manualIp, setManualIp] = useState('')

  // Étape 2 — association : null | { bridge, phase: waiting|timeout|unreachable|error, remaining, detail }
  const [pairing, setPairing] = useState(null)

  // Étape 3 — inventaire et sélection.
  const [inventory, setInventory] = useState({ phase: 'idle', lights: [], failure: null })
  const [selectedNames, setSelectedNames] = useState([])
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(null) // nom en cours de test, ou '*' pour toutes

  const [toast, setToast] = useState(null)

  const configured = !!(lighting.api_key_configured && lighting.bridge_ip)

  // ---- Toast (auto-fermeture, même mécanisme que ConfigPage) --------------
  useEffect(() => {
    if (!toast) return undefined
    const t = setTimeout(() => setToast(null), TOAST_MS)
    return () => clearTimeout(t)
  }, [toast])

  // ---- Chargement de la configuration --------------------------------------
  const loadConfig = useCallback(async () => {
    try {
      const res = await fetch('/config.json')
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const data = await res.json()
      const section = data?.lighting && typeof data.lighting === 'object' ? data.lighting : {}
      const next = { ...EMPTY_LIGHTING, ...section, lights: Array.isArray(section.lights) ? section.lights : [] }
      setLighting(next)
      return next
    } catch (error) {
      console.error('Load lighting config failed:', error)
      setToast({ message: 'Erreur de chargement : ' + error.message, type: 'error' })
      return null
    } finally {
      setConfigLoaded(true)
    }
  }, [])

  useEffect(() => { loadConfig() }, [loadConfig])

  // Après tout enregistrement : recharger la config, prévenir la Navbar
  // (ampoule du menu) et rafraîchir le badge — contrat §7.1.
  const afterSave = useCallback(async () => {
    await loadConfig()
    notifyLightingChanged()
    refreshStatus()
  }, [loadConfig, refreshStatus])

  // ---- Étape 1 : découverte ------------------------------------------------
  const handleDiscover = async () => {
    setDiscovery({ phase: 'searching', bridges: [] })
    setSelectedBridge(null)
    setManualOpen(false)
    try {
      const res = await postJson('/api/lighting/discover')
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const data = await readJsonSafe(res)
      const bridges = Array.isArray(data.bridges) ? data.bridges : []
      setDiscovery({ phase: 'done', bridges })
      // Un seul pont : pas de choix à faire. Plusieurs : aucun présélectionné (maquette §03).
      if (bridges.length === 1) setSelectedBridge(bridges[0])
    } catch (error) {
      console.error('Bridge discovery failed:', error)
      setDiscovery({ phase: 'done', bridges: [], error: error.message })
    }
  }

  const handleUseManualIp = () => {
    const ip = manualIp.trim()
    if (!ip) return
    setSelectedBridge({ ip, id: '', model: '', manual: true })
  }

  // ---- Étape 2 : association (appui bouton) --------------------------------
  const startPairing = (bridge) => {
    setPairing({ bridge, phase: 'waiting', remaining: REGISTER_TIMEOUT_S })
  }

  // Association réussie : la clé est enregistrée côté serveur, jamais renvoyée.
  // On persiste ici le pont (ip + id) et on active la section. `api_key`
  // absente => préservée (§6.1). Les ampoules déjà choisies (cas « ré-associer »
  // depuis l'état refusé) sont conservées telles quelles.
  const onPairedRef = useRef(null)
  onPairedRef.current = async (bridge) => {
    try {
      const res = await saveLighting({
        enabled: true,
        bridge_ip: bridge.ip,
        bridge_id: bridge.id || lighting.bridge_id || '',
        lights: lighting.lights,
      })
      if (!res.ok) throw new Error(await res.text())
      setPairing(null)
      setToast({ message: 'Pont associé.', type: 'success' })
      await afterSave()
    } catch (error) {
      console.error('Save bridge failed:', error)
      setPairing(null)
      setToast({ message: 'Erreur : ' + error.message, type: 'error' })
    }
  }

  useEffect(() => {
    if (!pairing || pairing.phase !== 'waiting') return undefined
    let cancelled = false
    let retry = null
    const deadline = Date.now() + REGISTER_TIMEOUT_S * 1000
    const bridge = pairing.bridge

    const countdown = setInterval(() => {
      const left = Math.max(0, Math.ceil((deadline - Date.now()) / 1000))
      setPairing(p => (p && p.phase === 'waiting' ? { ...p, remaining: left } : p))
    }, 1000)

    const attempt = async () => {
      if (cancelled) return
      let res
      try {
        res = await postJson('/api/lighting/register', { bridge_ip: bridge.ip })
      } catch (error) {
        if (!cancelled) setPairing(p => (p ? { ...p, phase: 'error', detail: error.message } : p))
        return
      }
      if (cancelled) return
      if (res.ok) {
        onPairedRef.current?.(bridge)
        return
      }
      const body = await readJsonSafe(res)
      if (cancelled) return
      if (res.status === 409 || body.result === 'refused') {
        // Cas NOMINAL : personne n'a encore appuyé. On attend, on réessaie.
        if (Date.now() >= deadline) {
          setPairing(p => (p ? { ...p, phase: 'timeout', remaining: 0 } : p))
          return
        }
        retry = setTimeout(attempt, REGISTER_RETRY_MS)
        return
      }
      if (res.status === 503 || body.result === 'unreachable') {
        setPairing(p => (p ? { ...p, phase: 'unreachable' } : p))
        return
      }
      setPairing(p => (p ? { ...p, phase: 'error', detail: `HTTP ${res.status}` } : p))
    }

    attempt()
    return () => {
      cancelled = true
      clearInterval(countdown)
      if (retry) clearTimeout(retry)
    }
    // `pairing.bridge` est figé pour toute la durée d'une attente ; seul le
    // passage à `waiting` (démarrage, « Réessayer ») relance la boucle.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pairing?.phase, pairing?.bridge?.ip])

  const handleCancelPairing = () => setPairing(null)
  const handleRetryPairing = () =>
    setPairing(p => (p ? { ...p, phase: 'waiting', remaining: REGISTER_TIMEOUT_S } : p))

  // ---- Étape 3 : inventaire ------------------------------------------------
  const loadInventory = useCallback(async () => {
    setInventory(prev => ({ ...prev, phase: 'loading' }))
    try {
      const res = await fetch('/api/lighting/lights')
      if (!res.ok) {
        const body = await readJsonSafe(res)
        setInventory({ phase: 'done', lights: [], failure: classifyFailure(res, body) })
        return
      }
      const data = await readJsonSafe(res)
      setInventory({ phase: 'done', lights: Array.isArray(data.lights) ? data.lights : [], failure: null })
    } catch (error) {
      console.error('Load lights failed:', error)
      setInventory({ phase: 'done', lights: [], failure: 'error' })
    }
  }, [])

  useEffect(() => {
    if (configured) loadInventory()
    else setInventory({ phase: 'idle', lights: [], failure: null })
  }, [configured, lighting.bridge_ip, loadInventory])

  // Sélection initiale : la config fait foi ; si elle est vide (association
  // toute fraîche) TOUT est coché — le pont est dédié à BuzzMaster. Mais tout
  // reste affiché et décochable (maquette §05).
  const inventoryKey = inventory.lights.map(l => l.name).join(' ')
  useEffect(() => {
    if (inventory.phase !== 'done') return
    const fromConfig = lighting.lights.map(l => l.name)
    if (fromConfig.length > 0) setSelectedNames(fromConfig)
    else setSelectedNames(inventory.lights.map(l => l.name))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [inventory.phase, inventoryKey, lighting.lights])

  // Lignes affichées : inventaire + noms configurés introuvables (§4.2 — on
  // signale, on ne remplace jamais par une voisine). Doublon de nom => refus
  // explicite (§4.2-3). Pont injoignable => dernière sélection connue, gelée.
  const rows = useMemo(() => {
    const counts = new Map()
    inventory.lights.forEach(l => counts.set(l.name, (counts.get(l.name) || 0) + 1))
    const fromInventory = inventory.lights.map(l => ({
      key: `inv-${l.id}`,
      name: l.name,
      id: l.id,
      reachable: !!l.reachable,
      duplicate: counts.get(l.name) > 1,
      missing: false,
    }))
    const known = new Set(inventory.lights.map(l => l.name))
    const missing = lighting.lights
      .filter(l => !known.has(l.name))
      .map(l => ({ key: `cfg-${l.name}`, name: l.name, id: null, reachable: false, duplicate: false, missing: true }))
    return [...fromInventory, ...missing]
  }, [inventory.lights, lighting.lights])

  const frozen = inventory.failure === 'unreachable' || inventory.failure === 'refused'

  const toggleName = (name, checked) => {
    setSelectedNames(prev => (checked ? [...new Set([...prev, name])] : prev.filter(n => n !== name)))
  }

  const handleSaveLights = async () => {
    setSaving(true)
    try {
      // Les entrées existantes sont reprises telles quelles (rôle/équipe #213
      // préservés) ; les nouvelles sont `general` — seul rôle actif en #207.
      const existing = new Map(lighting.lights.map(l => [l.name, l]))
      const lights = selectedNames.map(name => existing.get(name) || { name, role: 'general' })
      const res = await saveLighting({
        enabled: true,
        bridge_ip: lighting.bridge_ip,
        bridge_id: lighting.bridge_id,
        lights,
      })
      if (!res.ok) throw new Error(await res.text())
      setToast({ message: 'Ampoules enregistrées.', type: 'success' })
      await afterSave()
    } catch (error) {
      console.error('Save lights failed:', error)
      setToast({ message: 'Erreur : ' + error.message, type: 'error' })
    } finally {
      setSaving(false)
    }
  }

  const handleTest = async (name) => {
    setTesting(name || '*')
    try {
      const res = await postJson('/api/lighting/test', name ? { name } : {})
      if (!res.ok) {
        const body = await readJsonSafe(res)
        const failure = classifyFailure(res, body)
        setToast({
          message: failure === 'unreachable' ? 'Pont injoignable — test impossible.'
            : failure === 'refused' ? 'Association refusée — ré-associez le pont.'
            : `Erreur : HTTP ${res.status}`,
          type: failure === 'error' ? 'error' : 'warning',
        })
      }
    } catch (error) {
      setToast({ message: 'Erreur : ' + error.message, type: 'error' })
    } finally {
      setTesting(null)
    }
  }

  const handleUnpair = async () => {
    if (!window.confirm("Dissocier ce pont ? La clé d'association sera effacée et l'éclairage ne sera plus piloté.")) return
    try {
      const res = await saveLighting({
        enabled: false,
        bridge_ip: '',
        bridge_id: '',
        clear_api_key: true,
        lights: [],
      })
      if (!res.ok) throw new Error(await res.text())
      setDiscovery({ phase: 'idle', bridges: [] })
      setSelectedBridge(null)
      setToast({ message: 'Pont dissocié.', type: 'success' })
      await afterSave()
    } catch (error) {
      console.error('Unpair failed:', error)
      setToast({ message: 'Erreur : ' + error.message, type: 'error' })
    }
  }

  const handleReassociate = () => startPairing({ ip: lighting.bridge_ip, id: lighting.bridge_id })

  // ---- Badge d'état (4 valeurs, maquette §02) ------------------------------
  // Non configuré tant qu'aucun pont n'est associé. Sinon l'état du pilote
  // (status) fait foi ; s'il n'est pas encore démarré (« disabled » juste
  // après l'association), le résultat de l'inventaire — preuve directe que le
  // pont répond ou non — le remplace.
  const badgeState = !configured
    ? 'disabled'
    : normalizeLightingState(status.state) !== 'disabled'
      ? status.state
      : inventory.failure === 'unreachable' || inventory.failure === 'refused'
        ? inventory.failure
        : 'ok'

  const lightsSummary = badgeState === 'ok' && status.lights_total > 0
    ? ` · ${status.lights_ok}/${status.lights_total} ampoules`
    : ''

  // ---- Rendu ---------------------------------------------------------------
  const step = pairing ? 2 : configured ? 3 : 1

  return (
    <div className="ambiance-page page">
      <header className="page-header ambiance-header">
        <div className="ambiance-title-row">
          <h1 className="page-title">Ambiance</h1>
          <span
            className={`ambiance-status-badge is-${badgeState}`}
            data-state={badgeState}
            role="status"
          >
            <span className="ambiance-status-dot" aria-hidden="true" />
            {lightingStateLabel(badgeState)}{lightsSummary}
          </span>
        </div>
        <p className="page-subtitle">Éclairage de la salle piloté par le jeu</p>
      </header>

      <Card padding="lg" className="ambiance-card">
        {!configLoaded && <p className="ambiance-hint">Chargement…</p>}

        {/* ---------------- Étape 1 : trouver le pont ---------------- */}
        {configLoaded && step === 1 && (
          <section className="ambiance-step ambiance-step-discover" aria-labelledby="ambiance-step1-title">
            <h2 id="ambiance-step1-title" className="ambiance-step-title">Trouver le pont</h2>
            <p className="ambiance-hint">
              Pilote l'éclairage de la salle en réaction au jeu — buzz, révélation, points.
              Nécessite un pont Philips Hue sur le même réseau. Fonctionnalité facultative :
              sans pont, rien ne change.
            </p>

            {discovery.phase === 'done' && discovery.bridges.length === 0 && !selectedBridge && (
              <div className="ambiance-notice">
                <strong>Aucun pont trouvé.</strong>
                <span>
                  Le pont est-il allumé, et sur le même réseau que ce serveur ?
                  {discovery.error ? ` (${discovery.error})` : ''}
                </span>
                {!manualOpen && (
                  <button type="button" className="ambiance-link" onClick={() => setManualOpen(true)}>
                    Saisir l'adresse manuellement
                  </button>
                )}
                {manualOpen && (
                  <div className="ambiance-manual">
                    <label className="ambiance-label" htmlFor="ambiance-manual-ip">Adresse du pont</label>
                    <div className="ambiance-manual-row">
                      <input
                        id="ambiance-manual-ip"
                        type="text"
                        inputMode="decimal"
                        placeholder="192.168.1.101"
                        value={manualIp}
                        onChange={e => setManualIp(e.target.value)}
                        onKeyDown={e => e.key === 'Enter' && handleUseManualIp()}
                      />
                      <Button variant="secondary" size="sm" onClick={handleUseManualIp} disabled={!manualIp.trim()}>
                        Utiliser cette adresse
                      </Button>
                    </div>
                  </div>
                )}
              </div>
            )}

            {discovery.phase === 'done' && discovery.bridges.length > 1 && (
              <fieldset className="ambiance-fieldset">
                <legend className="ambiance-fieldset-title">Plusieurs ponts trouvés — choisissez</legend>
                {discovery.bridges.map(b => (
                  <label key={b.id || b.ip} className="ambiance-choice">
                    <input
                      type="radio"
                      name="ambiance-bridge"
                      checked={selectedBridge?.ip === b.ip}
                      onChange={() => setSelectedBridge(b)}
                    />
                    <span className="ambiance-mono">{b.ip}</span>
                    <span className="ambiance-meta">{b.id}{b.model ? ` · ${b.model}` : ''}</span>
                  </label>
                ))}
              </fieldset>
            )}

            {selectedBridge && discovery.bridges.length <= 1 && (
              <fieldset className="ambiance-fieldset">
                <legend className="ambiance-fieldset-title">{selectedBridge.manual ? 'Pont saisi' : 'Pont détecté'}</legend>
                <div className="ambiance-fields">
                  <div className="ambiance-field">
                    <span className="ambiance-label">Adresse</span>
                    <span className="ambiance-value ambiance-mono">{selectedBridge.ip}</span>
                  </div>
                  {selectedBridge.id && (
                    <div className="ambiance-field">
                      <span className="ambiance-label">Identifiant</span>
                      <span className="ambiance-value ambiance-mono">{selectedBridge.id}</span>
                    </div>
                  )}
                </div>
                {selectedBridge.model && (
                  <p className="ambiance-meta">Modèle {selectedBridge.model}</p>
                )}
              </fieldset>
            )}

            <div className="ambiance-actions">
              {selectedBridge ? (
                <>
                  <Button variant="primary" onClick={() => startPairing(selectedBridge)}>Associer ce pont</Button>
                  <Button variant="ghost" onClick={handleDiscover} loading={discovery.phase === 'searching'}>
                    Rechercher à nouveau
                  </Button>
                </>
              ) : discovery.phase === 'done' && discovery.bridges.length > 1 ? (
                <>
                  <Button variant="primary" disabled>Associer ce pont</Button>
                  <Button variant="ghost" onClick={handleDiscover}>Rechercher à nouveau</Button>
                </>
              ) : (
                <Button variant="primary" onClick={handleDiscover} loading={discovery.phase === 'searching'}>
                  {discovery.phase === 'done' ? 'Rechercher à nouveau' : 'Rechercher un pont'}
                </Button>
              )}
            </div>
          </section>
        )}

        {/* ---------------- Étape 2 : associer (appui bouton) ---------------- */}
        {step === 2 && (
          <section className="ambiance-step ambiance-step-pair" aria-labelledby="ambiance-step2-title">
            <h2 id="ambiance-step2-title" className="ambiance-step-title">Associer le pont</h2>
            <p className="ambiance-meta">Pont <span className="ambiance-mono">{pairing.bridge.ip}</span></p>

            {pairing.phase === 'waiting' && (
              <>
                <div className="ambiance-wait is-waiting" role="status" aria-live="polite">
                  <span className="ambiance-wait-ring" aria-hidden="true" />
                  <div className="ambiance-wait-text">
                    <div className="ambiance-wait-title">Appuyez sur le bouton rond au centre du pont</div>
                    <div className="ambiance-wait-sub">
                      BuzzMaster réessaie automatiquement toutes les {REGISTER_RETRY_MS / 1000} secondes.
                    </div>
                  </div>
                  <span className="ambiance-wait-countdown">{pairing.remaining} s</span>
                </div>
                <div className="ambiance-actions">
                  <Button variant="ghost" onClick={handleCancelPairing}>Annuler</Button>
                </div>
              </>
            )}

            {pairing.phase === 'timeout' && (
              <>
                <div className="ambiance-wait is-timeout" role="status">
                  <div className="ambiance-wait-text">
                    <div className="ambiance-wait-title">Bouton non pressé</div>
                    <div className="ambiance-wait-sub">
                      Le délai de {REGISTER_TIMEOUT_S} secondes est écoulé. Rien n'a été enregistré.
                    </div>
                  </div>
                </div>
                <div className="ambiance-actions">
                  <Button variant="primary" onClick={handleRetryPairing}>Réessayer</Button>
                  <Button variant="ghost" onClick={handleCancelPairing}>Annuler</Button>
                </div>
              </>
            )}

            {(pairing.phase === 'unreachable' || pairing.phase === 'error') && (
              <>
                <div className={`ambiance-wait is-${pairing.phase}`} role="alert">
                  <div className="ambiance-wait-text">
                    <div className="ambiance-wait-title">
                      {pairing.phase === 'unreachable' ? 'Pont injoignable' : 'Association impossible'}
                    </div>
                    <div className="ambiance-wait-sub">
                      {pairing.phase === 'unreachable'
                        ? "Le pont ne répond pas à cette adresse. Vérifiez qu'il est allumé et branché au réseau."
                        : `Réponse inattendue du serveur${pairing.detail ? ` (${pairing.detail})` : ''}.`}
                    </div>
                  </div>
                </div>
                <div className="ambiance-actions">
                  <Button variant="primary" onClick={handleRetryPairing}>Réessayer</Button>
                  <Button variant="ghost" onClick={handleCancelPairing}>Annuler</Button>
                </div>
              </>
            )}
          </section>
        )}

        {/* ---------------- Étape 3 : choisir et tester les ampoules ---------------- */}
        {configLoaded && step === 3 && (
          <section className="ambiance-step ambiance-step-lights" aria-labelledby="ambiance-step3-title">
            <div className="ambiance-bridge-row">
              <div>
                <h2 id="ambiance-step3-title" className="ambiance-step-title">Ampoules du pont</h2>
                <p className="ambiance-meta">
                  <span className="ambiance-mono">{lighting.bridge_ip}</span>
                  {lighting.bridge_id && <> · <span className="ambiance-mono">{lighting.bridge_id}</span></>}
                </p>
              </div>
              <div className="ambiance-bridge-actions">
                {badgeState === 'refused' && (
                  <Button variant="primary" size="sm" onClick={handleReassociate}>Ré-associer</Button>
                )}
                <Button variant="secondary" size="sm" onClick={handleUnpair}>Dissocier ce pont</Button>
              </div>
            </div>

            {badgeState === 'unreachable' && (
              <div className="ambiance-notice is-unreachable" role="status">
                <strong>Pont injoignable.</strong>
                <span>Réseau, pont éteint ou débranché. La partie continue normalement. Rien n'est perdu : la dernière sélection connue est affichée.</span>
              </div>
            )}
            {badgeState === 'refused' && (
              <div className="ambiance-notice is-refused" role="status">
                <strong>Association refusée.</strong>
                <span>La clé a été refusée par le pont (absente, invalide ou révoquée). Ré-associez le pont pour reprendre la main.</span>
              </div>
            )}

            {inventory.phase === 'loading' && rows.length === 0 && (
              <p className="ambiance-hint">Lecture des ampoules…</p>
            )}

            {inventory.phase === 'done' && rows.length === 0 && !frozen && (
              <p className="ambiance-hint">Aucune ampoule sur ce pont.</p>
            )}

            {rows.length > 0 && (
              <ul className={`ambiance-lights ${frozen ? 'is-frozen' : ''}`} aria-label="Ampoules">
                {rows.map(row => {
                  const checked = selectedNames.includes(row.name)
                  const blocked = frozen || row.duplicate
                  return (
                    <li
                      key={row.key}
                      className={`ambiance-light ${row.missing ? 'is-missing' : ''} ${!row.missing && !row.reachable ? 'is-unreachable' : ''} ${row.duplicate ? 'is-duplicate' : ''}`}
                    >
                      <label className="ambiance-light-main">
                        <input
                          type="checkbox"
                          checked={checked}
                          disabled={blocked}
                          onChange={e => toggleName(row.name, e.target.checked)}
                          aria-label={row.name}
                        />
                        <span className="ambiance-light-text">
                          <span className="ambiance-light-name">{row.name}</span>
                          <span className="ambiance-light-meta">
                            {row.missing
                              ? 'introuvable sur le pont — renommée ou supprimée ?'
                              : row.duplicate
                                ? `id ${row.id} · nom en double — renommez-la dans l'application Hue`
                                : row.reachable
                                  ? `id ${row.id} · joignable`
                                  : `id ${row.id} · éteinte au mur`}
                          </span>
                        </span>
                      </label>
                      <Button
                        variant="secondary"
                        size="sm"
                        disabled={blocked || row.missing || !row.reachable || testing !== null}
                        loading={testing === row.name}
                        onClick={() => handleTest(row.name)}
                      >
                        Tester
                      </Button>
                    </li>
                  )
                })}
              </ul>
            )}

            <div className="ambiance-actions">
              <Button variant="primary" onClick={handleSaveLights} loading={saving} disabled={frozen}>
                Enregistrer
              </Button>
              <Button
                variant="secondary"
                onClick={() => handleTest(null)}
                loading={testing === '*'}
                disabled={frozen || testing !== null || selectedNames.length === 0}
              >
                Tester toutes les ampoules
              </Button>
              <Button variant="ghost" onClick={loadInventory} loading={inventory.phase === 'loading'}>
                Actualiser la liste
              </Button>
            </div>
            <p className="ambiance-hint">« Tester » produit un bref flash puis rend l'ampoule à son état précédent.</p>
          </section>
        )}
      </Card>

      {toast && (
        <div className={`wifi-toast wifi-toast-${toast.type}`} role="status">
          {toast.message}
        </div>
      )}
    </div>
  )
}
