import { useState, useEffect, useCallback, useMemo, useRef } from 'react'
import Button from '../components/Button'
import Card from '../components/Card'
import { useGame } from '../hooks/GameContext'
import { useLightingStatus, notifyLightingChanged } from '../hooks/useLightingStatus'
import { lightingStateLabel, normalizeLightingState } from '../utils/lightingState'
import { findTeamColor } from '../constants/colors'
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
//   - toutes les ampoules AFFICHÉES, cochées ou non (voir revirement 2026-09-07
//     ci-dessous — ce n'est PLUS "cochées par défaut") ;
//   - badge à QUATRE valeurs, « injoignable » et « refusée » jamais fondues.
//
// ⚠️ Revirement (retour QUALIF v10.0.0.13, 2026-09-07) — le point ci-dessus
// disait à l'origine « toutes les ampoules cochées par défaut mais toutes
// affichées » (pont dédié à BuzzMaster, présomption assumée). L'utilisateur
// ne veut plus de cette présomption : une association fraîche ne coche plus
// rien, chaque ampoule est affectée explicitement (voir defaultSelectionFor
// ci-dessous). Ne pas restaurer l'ancien comportement en lisant seulement ce
// bloc de tête — c'est le code, pas ce commentaire, qui a été corrigé en
// premier ; ce commentaire reflète l'état actuel.
//
// La section de config se nomme `lighting`, jamais `ambiance` (mot déjà pris
// par la catégorie de sauvegarde de game-config.json, BackupPage.jsx/#152).
//
// #213 (v10.0.0, Batch 3) — ajout à l'étape 3 ci-dessus, sur cet écran
// uniquement (jamais /anim) : colonne « Rôle » par ampoule (menu déroulant
// Éclairage général / Équipe X), même schéma role/team que #207 fige déjà
// (config.go). La liste des équipes vient de l'état de jeu courant
// (useGame().teams), pas d'un endpoint dédié — c'est la même source que
// TeamsPage/GamePage. Maquette de référence : rev6 de
// docs/mockups/lighting-team-assignment-213.html. Contrats :
// contracts/lighting.md §10.1 (SHA df448318), contracts/hue-bridge.md
// §5.2/§5.7.
//
// #208 — le panneau de conduite en direct ON/AUTO/OFF + Flash a d'abord
// vécu ici (Batch 3), puis a été déplacé sur GamePage.jsx (correction
// utilisateur du 2026-09-07) : ce sont des outils utilisés par la régie
// PENDANT une partie, donc à portée de main sur l'écran qu'elle a ouvert en
// séance — pas sur cet écran de configuration séparé. Composant réutilisable
// : components/LightingModePanel.jsx (+ .css).

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

// Taxonomie §5.6 appliquée à une réponse d'inventaire/test : trois issues
// (+ `busy` : une opération est déjà en cours, ce n'est pas une panne).
function classifyFailure(res, body) {
  if (res.status === 429 || body?.result === 'busy') return 'busy'
  if (res.status === 503 || body?.result === 'unreachable') return 'unreachable'
  if (res.status === 401 || res.status === 409 || body?.result === 'refused') return 'refused'
  return 'error'
}

// Issues de POST /api/lighting/register hors succès (http_lighting.go). Le
// serveur ne renvoie JAMAIS de `message` : les textes sont fixés ici.
//   409 link_button_not_pressed   → nominal, on relance dans 2 s
//   429 busy (register_in_progress) → une association tourne déjà, on relance
//   503 unreachable               → « Pont injoignable » (rebrancher)
//   400 bridge_ip_not_private     → adresse hors réseau local : changer d'adresse
//   409 api_key_refused, 502 invalid_key_from_bridge, 500 {result:error}
//                                 → « Association impossible », texte fixe
function registerOutcome(res, body) {
  const reason = body?.reason || ''
  if (res.status === 409 && (reason === '' || reason === 'link_button_not_pressed')) return { kind: 'retry' }
  if (res.status === 429 || body?.result === 'busy') return { kind: 'retry' }
  if (res.status === 503 || body?.result === 'unreachable') return { kind: 'unreachable' }
  if (res.status === 400 && reason === 'bridge_ip_not_private') return { kind: 'rejected' }
  if (res.status === 502 && reason === 'invalid_key_from_bridge') {
    return { kind: 'error', detail: 'Le pont a renvoyé une clé inutilisable.' }
  }
  if (res.status === 409) return { kind: 'error', detail: "Le pont a refusé l'association." }
  return { kind: 'error', detail: `Réponse inattendue du serveur (HTTP ${res.status}).` }
}

// Sélection par défaut des ampoules — la config fait foi, POINT FINAL.
//
// Retour utilisateur QUALIF v10.0.0.13 (2026-09-07) — REVIRE la décision de
// #207 : une association fraîche ne pré-coche plus rien. Auparavant, config
// vide ⇒ tout l'inventaire coché (« le pont est dédié à BuzzMaster ») ;
// l'utilisateur ne veut plus de cette présomption, même sur un pont dédié —
// chaque ampoule nouvellement détectée démarre NON affectée, à cocher (et
// affecter un rôle, #213) explicitement une par une. Reste TOUJOURS affichée
// même non cochée (#207, « toutes les ampoules affichées, cochées ou non »
// — seule la moitié « cochées » de cette règle change ici).
function defaultSelectionFor(configLights) {
  return configLights.map(l => l.name)
}

// #213 — normalise une entrée `lighting.lights` en {role, team}. Toute
// valeur de `role` autre que "team" est traitée comme `general` (repli
// prudent, même principe que normalizeLightingState).
function roleOf(light) {
  if (light && light.role === 'team' && light.team) return { role: 'team', team: light.team }
  return { role: 'general' }
}

// #213 (revue code-reviewer, v10.0.0 Batch 3) — la `value` du <select> de
// rôle ne peut PAS être le nom d'équipe brut : une équipe littéralement
// nommée "general" produirait deux <option value="general"> indiscernables
// pour `onChange` (e.target.value vaudrait "general" quelle que soit celle
// cliquée, rendant cette équipe impossible à choisir). Les valeurs d'équipe
// sont donc préfixées — pendant côté React du garde-fou déjà posé côté
// backend (`ambiance.go`, `seen[lighting.ZoneGeneral]` : « a team literally
// named "general" must never shadow it »). Le préfixe ne fuit jamais dans
// {role, team} envoyé au serveur — seule la value DOM en porte trace.
const ROLE_SELECT_GENERAL = 'general'
const TEAM_SELECT_PREFIX = 'team:'
const roleSelectValue = (role) => (role.role === 'team' ? TEAM_SELECT_PREFIX + role.team : ROLE_SELECT_GENERAL)
const roleFromSelectValue = (value) =>
  value.startsWith(TEAM_SELECT_PREFIX) ? { role: 'team', team: value.slice(TEAM_SELECT_PREFIX.length) } : { role: 'general' }

export default function AmbiancePage() {
  const { teams } = useGame()
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
  // null = « par défaut » : la sélection effective est DÉRIVÉE (useMemo) de la
  // config et de l'inventaire dans le même rendu que les lignes — jamais fixée
  // après coup par un effet, qui laisserait un rendu intermédiaire tout
  // décoché (course relevée en revue). Un tableau = choix explicite de
  // l'utilisateur, non encore enregistré.
  const [selectedNames, setSelectedNames] = useState(null)
  const [saving, setSaving] = useState(false)
  // #213 — remise groupée à l'état LIBRE : null (repos) | 'all' | 'general' |
  // 'team', identifie quel bouton est en vol (chacun sa propre portée, voir
  // handleResetToLibre).
  const [resettingScope, setResettingScope] = useState(null)
  const [testing, setTesting] = useState(null) // nom en cours de test, ou '*' pour toutes

  // #213 — rôle par ampoule : { [name]: {role, team} }, seulement les
  // entrées modifiées cette session (même patron que selectedNames/prev ci-
  // dessus) — la config chargée fait foi tant que l'utilisateur n'a rien
  // changé, jamais fixé après coup par un effet.
  const [roleOverrides, setRoleOverrides] = useState({})

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
      if (res.status === 429) {
        // Une recherche tourne déjà (autre onglet) : pas une panne.
        setDiscovery({ phase: 'done', bridges: [], busy: true })
        return
      }
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

  // Association réussie : le serveur a DÉJÀ persisté la clé, `bridge_ip` et le
  // `bridge_id` qu'il vient de lire sur le pont (http_lighting.go). La page ne
  // renvoie donc que `enabled: true` — le merge de `handleConfig` est champ par
  // champ, toute clé absente est laissée intacte. Renvoyer un `bridge_id`
  // calculé côté client écrasait à vide celui du serveur après une saisie
  // manuelle de l'IP (bug relevé en revue). Les ampoules déjà choisies sont
  // préservées pour la même raison (clé absente).
  // L'étape 2 reste affichée jusqu'à ce que la config rechargée dise
  // « configuré » : pas d'aller-retour visuel par l'étape 1.
  const onPairedRef = useRef(null)
  onPairedRef.current = async () => {
    try {
      const res = await saveLighting({ enabled: true })
      if (!res.ok) throw new Error(await res.text())
      await afterSave()
      setToast({ message: 'Pont associé.', type: 'success' })
    } catch (error) {
      console.error('Enable lighting failed:', error)
      setToast({ message: 'Erreur : ' + error.message, type: 'error' })
    } finally {
      setPairing(null)
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
        // 200 {"result":"ok","bridge_id":"…"} — l'identité est déjà persistée
        // côté serveur, rien à renvoyer.
        onPairedRef.current?.()
        return
      }
      const body = await readJsonSafe(res)
      if (cancelled) return
      const outcome = registerOutcome(res, body)
      if (outcome.kind === 'retry') {
        // Cas NOMINAL : personne n'a encore appuyé (ou une association tourne
        // déjà). On attend, on réessaie.
        if (Date.now() >= deadline) {
          setPairing(p => (p ? { ...p, phase: 'timeout', remaining: 0 } : p))
          return
        }
        retry = setTimeout(attempt, REGISTER_RETRY_MS)
        return
      }
      setPairing(p => (p ? { ...p, phase: outcome.kind, detail: outcome.detail } : p))
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
  // Round 4, Bug 1 (2026-09-07) — une réponse VIDE sans erreur juste après
  // association a été observée en pratique (29 ampoules réelles sur le
  // pont), alors que #213/#207 traitaient déjà ce cas comme un simple "rien
  // à afficher" plutôt qu'un état transitoire à corriger. Deux filets
  // indépendants, aucun ne remplace l'autre :
  //   1. Auto-guérison ci-dessous — un seul nouvel essai après un court
  //      délai si la première réponse est vide SANS erreur, jamais en
  //      boucle (`isRetry` empêche un second essai).
  //   2. "Enregistrer" reste désactivé tant que la liste est vide (plus bas,
  //      `rows.length === 0`) — filet de sécurité qui NE DÉPEND PAS de la
  //      guérison ci-dessus : même si l'auto-essai échouait aussi, un clic
  //      prématuré ne peut plus écraser une configuration existante par
  //      `lights: []`.
  const loadInventory = useCallback(async (isRetry = false) => {
    setInventory(prev => ({ ...prev, phase: 'loading' }))
    let next
    try {
      const res = await fetch('/api/lighting/lights')
      if (!res.ok) {
        const body = await readJsonSafe(res)
        next = { phase: 'done', lights: [], failure: classifyFailure(res, body) }
      } else {
        const data = await readJsonSafe(res)
        next = { phase: 'done', lights: Array.isArray(data.lights) ? data.lights : [], failure: null }
      }
    } catch (error) {
      console.error('Load lights failed:', error)
      next = { phase: 'done', lights: [], failure: 'error' }
    }
    // Un nouvel inventaire repart de la sélection par défaut (dérivée dans le
    // même rendu que les lignes — voir selectedNames) et des rôles tels
    // qu'enregistrés (#213 — voir roleOverrides).
    setSelectedNames(null)
    setRoleOverrides({})
    setInventory(next)
    if (!isRetry && next.phase === 'done' && !next.failure && next.lights.length === 0) {
      setTimeout(() => { loadInventory(true) }, 1500)
    }
  }, [])

  useEffect(() => {
    if (configured) loadInventory()
    else setInventory({ phase: 'idle', lights: [], failure: null })
  }, [configured, lighting.bridge_ip, loadInventory])

  // Sélection effective : choix explicite de l'utilisateur s'il existe, sinon
  // la sélection par défaut (uniquement la config — voir defaultSelectionFor)
  // — dérivée de façon SYNCHRONE, donc correcte dès le premier rendu des
  // lignes, jamais un flash intermédiaire incorrect.
  const defaultSelection = useMemo(
    () => defaultSelectionFor(lighting.lights),
    [lighting.lights]
  )
  const effectiveSelected = selectedNames ?? defaultSelection

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

  // Round 4, Bug 1 (2026-09-07) — vrai quand AUCUNE ligne affichée ne
  // provient de l'inventaire réel du pont : soit `rows` est entièrement
  // vide (rien n'a jamais été configuré ET le pont ne répond rien pour
  // l'instant), soit toutes les lignes sont des placeholders "introuvable"
  // dérivés de la config existante (le pont ne confirme aucune des
  // ampoules déjà enregistrées). `Array.prototype.every` sur un tableau
  // vide vaut `true` : ce test couvre donc aussi rows.length === 0 sans
  // condition séparée. Distinct du cas légitime "une partie des ampoules
  // configurées est introuvable, les autres sont bien là" (#207) — celui-là
  // NE bloque PAS l'enregistrement, seule l'absence TOTALE de confirmation
  // du pont le fait.
  const allMissing = rows.every(r => r.missing)

  // ---- #213 : rôle par ampoule ---------------------------------------------
  // Équipes de la partie courante — même source que TeamsPage/GamePage
  // (useGame().teams), pas un endpoint dédié : la liste proposée dans chaque
  // menu Rôle est TOUJOURS celle de la config de partie en vigueur.
  const teamNames = useMemo(() => Object.keys(teams || {}), [teams])
  const existingLights = useMemo(() => new Map(lighting.lights.map(l => [l.name, l])), [lighting.lights])

  const roleFor = useCallback(
    (name) => roleOverrides[name] ?? roleOf(existingLights.get(name)),
    [roleOverrides, existingLights]
  )

  const teamSwatchColor = useCallback((teamName) => {
    const data = teams?.[teamName]
    const c = findTeamColor(data?.COLOR_NAME, data?.COLOR)
    return c ? `rgb(${c.rgb.join(',')})` : null
  }, [teams])

  // Portée de chacun des 3 boutons « Remettre à Libre » — sert uniquement à
  // les désactiver quand ils n'auraient aucun effet (rien à réinitialiser
  // dans leur catégorie) : pas une garde de sécurité comme `allMissing`,
  // juste une désactivation de confort. « Toutes » n'a pas besoin de son
  // propre calcul : `effectiveSelected.length === 0` suffit (utilisé
  // directement au point d'usage).
  const anyGeneralAssigned = effectiveSelected.some(name => roleFor(name).role === 'general')
  const anyTeamAssigned = effectiveSelected.some(name => roleFor(name).role === 'team')

  const toggleName = (name, checked) => {
    setSelectedNames(prev => {
      const base = prev ?? defaultSelection
      return checked ? [...new Set([...base, name])] : base.filter(n => n !== name)
    })
  }

  const handleSaveLights = async () => {
    setSaving(true)
    try {
      // Rôle/équipe (#213) : l'override de cette session fait foi, sinon la
      // valeur déjà enregistrée, sinon `general` — même hiérarchie que
      // `roleFor`. Seules les clés possédées par cette étape sont envoyées
      // (merge champ par champ côté serveur) : pont et clé restent intacts.
      const lights = effectiveSelected.map(name => ({ name, ...roleFor(name) }))
      const res = await saveLighting({ enabled: true, lights })
      if (!res.ok) throw new Error(await res.text())
      setSelectedNames(null) // la config rechargée devient la référence
      setRoleOverrides({})
      setToast({ message: 'Ampoules enregistrées.', type: 'success' })
      await afterSave()
    } catch (error) {
      console.error('Save lights failed:', error)
      setToast({ message: 'Erreur : ' + error.message, type: 'error' })
    } finally {
      setSaving(false)
    }
  }

  // #213 — remise GROUPÉE à l'état LIBRE (retour utilisateur QUALIF
  // v10.0.0.13, précision du 2026-09-07 sur SHA b35fddbf) : le modèle a
  // TROIS états, pas deux — Libre (l'ampoule n'a AUCUNE entrée dans
  // `lighting.lights[]`, jamais configurée ou explicitement réinitialisée),
  // Général (entrée avec `role: "general"`), Équipe (entrée avec
  // `role: "team"`). Une première version de ce bouton remettait à
  // `role: "general"` — INCORRECT, ça change de rôle explicite, pas de
  // « libère » l'ampoule. Libre = retirer l'entrée du tableau envoyé, pas
  // lui donner un rôle. Vérifié cohérent avec le comportement déjà en place
  // pour une ampoule jamais cochée (#213/#207 revirement du même jour,
  // handleSaveLights) : une ampoule absente de `lights[]` est justement
  // affichée décochée avec le rôle de repli « Éclairage général » dans le
  // sélecteur — sans que cela persiste quoi que ce soit.
  //
  // Trois portées, un seul handler paramétré :
  //   'all'     → toutes les ampoules SÉLECTIONNÉES repassent libres.
  //   'general' → seules celles actuellement en rôle "general" (les
  //               ampoules d'équipe ne sont PAS touchées).
  //   'team'    → seules celles actuellement en rôle "team", quelle que
  //               soit l'équipe (les ampoules générales ne sont PAS touchées).
  // Distinct de `handleUnpair` ci-dessous (qui dissocie le PONT entier,
  // efface la clé) : celui-ci ne touche que les rôles, jamais l'association
  // du pont. Persiste immédiatement (même patron que handleUnpair, pas un
  // état local en attente d'un second clic sur « Enregistrer ») : chaque
  // bouton est sa propre action, avec sa propre confirmation.
  const RESET_CONFIRM = {
    all: "Remettre TOUTES les ampoules à l'état libre ? Elles ne seront plus pilotées tant qu'elles ne seront pas ré-affectées (rôle général ou équipe).",
    general: "Remettre à l'état libre les ampoules actuellement en « Éclairage général » ? Les ampoules affectées à une équipe ne sont pas concernées.",
    team: "Remettre à l'état libre les ampoules actuellement affectées à une équipe ? Les ampoules en « Éclairage général » ne sont pas concernées.",
  }
  const handleResetToLibre = async (scope) => {
    if (!window.confirm(RESET_CONFIRM[scope])) return
    setResettingScope(scope)
    try {
      // 'all' : rien ne survit — [] retire toutes les entrées sélectionnées.
      // 'general'/'team' : on RECONSTRUIT le tableau en excluant seulement
      // la catégorie ciblée — celles de l'autre catégorie gardent EXACTEMENT
      // leur rôle actuel, elles ne sont pas "re-sauvées à l'identique" par
      // accident avec une valeur différente.
      const lights = scope === 'all'
        ? []
        : effectiveSelected
            .filter(name => roleFor(name).role !== scope)
            .map(name => ({ name, ...roleFor(name) }))
      const res = await saveLighting({ enabled: true, lights })
      if (!res.ok) throw new Error(await res.text())
      setSelectedNames(null)
      setRoleOverrides({})
      setToast({ message: 'Ampoules remises à l\'état libre.', type: 'success' })
      await afterSave()
    } catch (error) {
      console.error('Reset to libre failed:', error)
      setToast({ message: 'Erreur : ' + error.message, type: 'error' })
    } finally {
      setResettingScope(null)
    }
  }

  const handleTest = async (name) => {
    setTesting(name || '*')
    try {
      const res = await postJson('/api/lighting/test', name ? { name } : {})
      if (!res.ok) {
        const body = await readJsonSafe(res)
        const failure = classifyFailure(res, body)
        // Aucun `message` serveur n'est supposé : textes fixés ici.
        const message = failure === 'busy' ? 'Un test est déjà en cours — patientez un instant.'
          : failure === 'unreachable' ? 'Pont injoignable — test impossible.'
          : failure === 'refused' && body?.reason === 'not_configured' ? 'Enregistrez d\'abord la configuration.'
          : failure === 'refused' ? 'Association refusée — ré-associez le pont.'
          : `Test impossible (HTTP ${res.status}).`
        setToast({ message, type: failure === 'error' ? 'error' : 'warning' })
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
                <strong>{discovery.busy ? 'Une recherche est déjà en cours.' : 'Aucun pont trouvé.'}</strong>
                <span>
                  {discovery.busy
                    ? 'Réessayez dans quelques secondes.'
                    : 'Le pont est-il allumé, et sur le même réseau que ce serveur ?'}
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

            {pairing.phase === 'rejected' && (
              <>
                <div className="ambiance-wait is-rejected" role="alert">
                  <div className="ambiance-wait-text">
                    <div className="ambiance-wait-title">Adresse hors du réseau local</div>
                    <div className="ambiance-wait-sub">
                      Le pont doit être sur une adresse privée du réseau (par exemple 192.168.x.x ou 10.x.x.x).
                      Rien n'a été enregistré — corrigez l'adresse.
                    </div>
                  </div>
                </div>
                <div className="ambiance-actions">
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
                        : `${pairing.detail || 'Réponse inattendue du serveur.'} Rien n'a été enregistré.`}
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

            {inventory.phase === 'done' && (inventory.failure === 'error' || inventory.failure === 'busy') && (
              <div className="ambiance-notice" role="status">
                <strong>{inventory.failure === 'busy' ? 'Le pont est occupé.' : 'Lecture des ampoules impossible.'}</strong>
                <span>Réessayez avec « Actualiser la liste ».</span>
              </div>
            )}

            {inventory.phase === 'done' && rows.length === 0 && !frozen && !inventory.failure && (
              <p className="ambiance-hint">Aucune ampoule sur ce pont.</p>
            )}

            {/* Round 4, Bug 1 (2026-09-07) — au moins une ligne à afficher,
                mais AUCUNE ne provient de l'inventaire réel (rows.length > 0
                uniquement grâce aux placeholders "introuvable" dérivés de la
                config déjà enregistrée) : le pont ne confirme rien pour
                l'instant. Distinct du cas légitime "une partie seulement est
                introuvable" (#207, signalé ligne par ligne, n'empêche rien) :
                ici c'est la totalité, donc "Enregistrer" est désactivé
                (§ plus bas) plutôt que de risquer un enregistrement qui
                efface silencieusement ce qui existe déjà. */}
            {inventory.phase === 'done' && rows.length > 0 && allMissing && !frozen && !inventory.failure && (
              <div className="ambiance-notice is-refused" role="status">
                <strong>Aucune ampoule reconnue par le pont pour l'instant.</strong>
                <span>
                  La configuration enregistrée n'est pas perdue, mais « Enregistrer » est
                  désactivé tant que le pont ne confirme aucune de ces ampoules — un nouvel essai
                  automatique est en cours ; sinon, cliquez « Actualiser la liste ».
                </span>
              </div>
            )}

            {rows.length > 0 && (
              <ul className={`ambiance-lights ${frozen ? 'is-frozen' : ''}`} aria-label="Ampoules">
                {rows.map(row => {
                  const checked = effectiveSelected.includes(row.name)
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
                      {/* #213 — rôle : Éclairage général (défaut) ou Équipe X. Même
                          disponibilité que la case à cocher : figée si le pont est
                          injoignable/refusé, ou si le nom est en double. */}
                      <span className="ambiance-light-role-group">
                        {roleFor(row.name).role === 'team' && (
                          <span
                            className="ambiance-light-swatch"
                            style={{ backgroundColor: teamSwatchColor(roleFor(row.name).team) || 'var(--gray-300)' }}
                            aria-hidden="true"
                          />
                        )}
                        <select
                          className="ambiance-light-role"
                          value={roleSelectValue(roleFor(row.name))}
                          disabled={blocked}
                          onChange={e => {
                            setRoleOverrides(prev => ({
                              ...prev,
                              [row.name]: roleFromSelectValue(e.target.value),
                            }))
                          }}
                          aria-label={`Rôle de ${row.name}`}
                        >
                          <option value={ROLE_SELECT_GENERAL}>Éclairage général</option>
                          {teamNames.map(name => (
                            <option key={name} value={TEAM_SELECT_PREFIX + name}>Équipe — {name}</option>
                          ))}
                        </select>
                      </span>
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
              {/* Round 4, Bug 1 (2026-09-07) — "Enregistrer" n'était désactivé
                  que sur `frozen` (pont injoignable/refusé), jamais quand le
                  pont ne confirmait ENCORE AUCUNE ampoule (`allMissing`,
                  vrai aussi pour rows.length === 0 — voir sa définition).
                  Sans ce garde-fou, un clic dans cette fenêtre pouvait
                  envoyer `lights: []` lors d'une toute première association
                  (rien à préserver, mais rien à sauver non plus), ou
                  ré-enregistrer une config existante sans jamais avoir été
                  confirmée par le pont (le backend persiste fidèlement ce
                  qu'on lui envoie, vérifié par dev-backend) —
                  indépendamment de la cause exacte de l'absence de
                  confirmation, et indépendamment de l'auto-essai de
                  loadInventory ci-dessus, qui peut lui-même échouer. */}
              <Button variant="primary" onClick={handleSaveLights} loading={saving} disabled={frozen || allMissing}>
                Enregistrer
              </Button>
              <Button
                variant="secondary"
                onClick={() => handleTest(null)}
                loading={testing === '*'}
                disabled={frozen || testing !== null || effectiveSelected.length === 0}
              >
                Tester toutes les ampoules
              </Button>
              <Button variant="ghost" onClick={() => loadInventory()} loading={inventory.phase === 'loading'}>
                Actualiser la liste
              </Button>
              {/* #213 — remise groupée à l'état LIBRE (2026-09-07, précision
                  3 états sur SHA b35fddbf), distincte de « Dissocier ce pont »
                  plus haut (celle-ci efface la clé et le pont entier ; les
                  trois boutons ci-dessous ne touchent que les rôles — pas
                  l'association du pont). Trois portées disjointes : "toutes",
                  seulement "général", seulement "équipe" — chacune sa propre
                  confirmation, désactivée si rien à faire dans sa catégorie. */}
              <Button
                variant="ghost"
                onClick={() => handleResetToLibre('all')}
                loading={resettingScope === 'all'}
                disabled={frozen || allMissing || resettingScope !== null || effectiveSelected.length === 0}
              >
                Remettre à Libre toutes les ampoules
              </Button>
              <Button
                variant="ghost"
                onClick={() => handleResetToLibre('general')}
                loading={resettingScope === 'general'}
                disabled={frozen || allMissing || resettingScope !== null || !anyGeneralAssigned}
              >
                Remettre à Libre les ampoules de rôle Général
              </Button>
              <Button
                variant="ghost"
                onClick={() => handleResetToLibre('team')}
                loading={resettingScope === 'team'}
                disabled={frozen || allMissing || resettingScope !== null || !anyTeamAssigned}
              >
                Remettre à Libre les ampoules de rôle Équipe
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
