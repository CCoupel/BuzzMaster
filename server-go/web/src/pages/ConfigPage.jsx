import { useState, useRef, useEffect, useMemo } from 'react'
import { useGame } from '../hooks/GameContext'
import Button from '../components/Button'
import Card from '../components/Card'
import USBConfigModal from '../components/USBConfigModal'
import ApiKeyHelpModal from '../components/ApiKeyHelpModal'
import ApiKeyValidationDialog from '../components/ApiKeyValidationDialog'
import { OtaAllModal } from '../components/TeamCard'
import './ConfigPage.css'

// Libellés fournisseur pour les messages de validation de clé API
// (contracts/ai-key-validation.md §9.1) — noms utilisateur, pas les identifiants
// techniques 'anthropic'/'groq'.
const PROVIDER_LABELS = { anthropic: 'Claude', groq: 'Groq' }

export default function ConfigPage() {
  const { teams, bumpers, gameState, updateConfig, sendMessage, version, firmwareInfo: wsFirmwareInfo } = useGame()
  const dualRangeTrackRef = useRef(null)

  // Count outdated physical WebSocket buzzers only (those with FIRMWARE_VERSION set).
  // Excludes TCP-only/simulated bumpers (no FIRMWARE_VERSION) and VJoueurs.
  const outdatedCount = useMemo(() =>
    Object.values(bumpers).filter(b => b.IS_OUTDATED === true && b.FIRMWARE_VERSION).length
  , [bumpers])

  // Neon effect configuration
  const [neonConfig, setNeonConfig] = useState({
    enabled: false,
    mode: 'bar',
    arc_width: 60,
    intensity_gap: 80,
    rotation_speed: 4,
    glow_pulse_speed: 2,
    glow_pulse_min: 30,
    glow_pulse_max: 50,
    bar_offset: 20,
    bar_thickness: 4,
    arc_blur: 100
  })
  const [savingNeon, setSavingNeon] = useState(false)

  const [loadingDemo, setLoadingDemo] = useState(false)
  const [showUSBModal, setShowUSBModal] = useState(false)

  // Server parameters
  const [serverParams, setServerParams] = useState({
    auto_open_browsers: false,
    debug: false
  })
  const [savingParams, setSavingParams] = useState(false)

  // WiFi config
  const [wifiSsid, setWifiSsid] = useState('')
  const [wifiPassword, setWifiPassword] = useState('')
  const [wifiSsid2, setWifiSsid2] = useState('')
  const [wifiPassword2, setWifiPassword2] = useState('')
  const [wifiServerIp, setWifiServerIp] = useState('')
  const [wifiServerPort, setWifiServerPort] = useState(80)
  const [savingWifi, setSavingWifi] = useState(false)
  const [wifiToast, setWifiToast] = useState(null)
  const [broadcastingWifi, setBroadcastingWifi] = useState(false)

  // Firmware section
  const [firmwareInfo, setFirmwareInfo] = useState(null) // { VERSION, FILENAME, SIZE, EXISTS, EMBEDDED_VERSION }
  const [uploadingFirmware, setUploadingFirmware] = useState(false)
  const [restoringEmbedded, setRestoringEmbedded] = useState(false)
  const [showOtaAllModal, setShowOtaAllModal] = useState(false)
  const [firmwareToast, setFirmwareToast] = useState(null) // { message, type }
  const firmwareFileRef = useRef(null)

  // Default question image
  const [defaultImageIsCustom, setDefaultImageIsCustom] = useState(false) // true = custom uploaded, false = embedded fallback
  const [defaultImageCacheBuster, setDefaultImageCacheBuster] = useState(() => Date.now())
  const [uploadingDefaultImage, setUploadingDefaultImage] = useState(false)
  const [deletingDefaultImage, setDeletingDefaultImage] = useState(false)
  const [defaultImageToast, setDefaultImageToast] = useState(null)
  const defaultImageFileRef = useRef(null)

  // AI generation section (v6.0.0, #8) — la clé n'est jamais reçue du serveur
  // (contract ai-generation.md §2) : seul `api_key_configured` est lu.
  const [aiApiKeyInput, setAiApiKeyInput] = useState('')
  const [aiKeyConfigured, setAiKeyConfigured] = useState(false)
  const [savingAiKey, setSavingAiKey] = useState(false)
  const [clearingAiKey, setClearingAiKey] = useState(false)
  const [aiToast, setAiToast] = useState(null)
  // Popup d'aide "comment obtenir une clé API" par fournisseur (bugfix/config-api-key-help)
  const [apiKeyHelpProvider, setApiKeyHelpProvider] = useState(null) // null | 'anthropic' | 'groq'
  // Validation de clé API par appel réel au fournisseur (contracts/ai-key-validation.md,
  // tâche #13 du plan) — état "vérifiée" tri-état (persisté côté serveur, jamais
  // dérivé) + dialogue bloquant en cas de refus/injoignable + message d'erreur
  // inline sur le champ après "Corriger".
  const [aiKeyVerified, setAiKeyVerified] = useState(false)
  const [aiKeyFieldError, setAiKeyFieldError] = useState(null)
  const [groqKeyVerified, setGroqKeyVerified] = useState(false)
  const [groqKeyFieldError, setGroqKeyFieldError] = useState(null)
  // null | { provider, trimmedKey, result: 'invalid_key'|'unreachable', httpStatus,
  // detail, isConfigured, setConfigured, setVerified, setInput, setSaving, setFieldError }
  const [keyValidation, setKeyValidation] = useState(null)
  const [retryingValidation, setRetryingValidation] = useState(false)
  const [forcingValidationSave, setForcingValidationSave] = useState(false)
  // #137 — second provider BYOK (Groq, tier gratuit). Mêmes règles de secret
  // que la clé Anthropic (contract ai-multi-provider.md §8) : jamais
  // renvoyée, vide en POST = préservée, effacement explicite dédié.
  const [aiProvider, setAiProvider] = useState('anthropic') // 'anthropic' | 'groq'
  const [savingProvider, setSavingProvider] = useState(false)
  const [groqApiKeyInput, setGroqApiKeyInput] = useState('')
  const [groqKeyConfigured, setGroqKeyConfigured] = useState(false)
  const [savingGroqKey, setSavingGroqKey] = useState(false)
  const [clearingGroqKey, setClearingGroqKey] = useState(false)
  // Fix bloquant (_work/handoff/task-dev-frontend-20260806-103759.md) — la
  // section `ai` d'un POST /config.json est remplacée INTÉGRALEMENT, sauf les
  // 2 clés API résolues individuellement (contract ai-generation.md §0, même
  // règle que neon_effect — confirmé par dev-backend, pas une régression
  // backend). Poster un payload partiel ({ai: {groq_api_key: "..."}} seul)
  // remettait provider/batch_size/groq_model/etc à leurs défauts à chaque
  // sauvegarde de clé. `aiSettings` porte donc l'état COMPLET de la section
  // (hors clés, jamais reçues du serveur) pour être ré-émis en entier à
  // chaque POST, exactement comme `neonConfig` ci-dessus.
  const [aiSettings, setAiSettings] = useState({
    provider: 'anthropic',
    model: 'claude-opus-5',
    timeout_seconds: 300,
    max_questions: 200,
    batch_size: 20,
    inter_batch_delay_ms: 60000,
    context_token_budget: 1500,
    max_consecutive_failures: 2,
    groq_model: 'openai/gpt-oss-120b',
  })

  // WiFi toast auto-hide
  useEffect(() => {
    if (wifiToast) {
      const timer = setTimeout(() => setWifiToast(null), 3000)
      return () => clearTimeout(timer)
    }
  }, [wifiToast])

  // Firmware toast auto-hide
  useEffect(() => {
    if (firmwareToast) {
      const timer = setTimeout(() => setFirmwareToast(null), 4000)
      return () => clearTimeout(timer)
    }
  }, [firmwareToast])

  // Default image toast auto-hide
  useEffect(() => {
    if (defaultImageToast) {
      const timer = setTimeout(() => setDefaultImageToast(null), 3000)
      return () => clearTimeout(timer)
    }
  }, [defaultImageToast])

  // AI key toast auto-hide
  useEffect(() => {
    if (aiToast) {
      const timer = setTimeout(() => setAiToast(null), 3000)
      return () => clearTimeout(timer)
    }
  }, [aiToast])

  // Load neon config and server parameters from server on mount
  //
  // #136 (tâche 15) — garde `cancelled` : si le composant est démonté avant la
  // résolution du fetch (navigation rapide, remount en test), on n'appelle plus
  // setState sur un composant démonté. Pas d'AbortController ici : il changerait
  // la signature de l'appel `fetch()` (2e argument `{ signal }`), ce qui casse
  // les assertions existantes `toHaveBeenCalledWith('/config.json')`.
  useEffect(() => {
    let cancelled = false
    const fetchConfig = async () => {
      try {
        const response = await fetch('/config.json')
        if (cancelled) return
        if (response.ok) {
          const data = await response.json()
          if (cancelled) return
          if (data.neon_effect) {
            setNeonConfig(data.neon_effect)
          }
          if (data.server) {
            setServerParams({
              auto_open_browsers: data.server.auto_open_browsers || false,
              debug: data.server.debug || false
            })
          }
          // v6.0.0 (#8) — la clé elle-même n'est jamais renvoyée par le
          // serveur, seul ce booléen dérivé l'est (contract ai-generation.md §2).
          if (data.ai) {
            const claudeOk = !!data.ai.api_key_configured
            const groqOk = !!data.ai.groq_api_key_configured
            const savedProvider = data.ai.provider || 'anthropic'
            setAiKeyConfigured(claudeOk)
            // #137 — provider sélectionné + état de la clé Groq (même règle de secret).
            // Sélection strictement manuelle (bugfix/config-api-key-help, simplification
            // #7) — on respecte tel quel ce qui est persisté côté serveur, plus d'auto-pick.
            setAiProvider(savedProvider)
            setGroqKeyConfigured(groqOk)
            // Validation de clé API (contracts/ai-key-validation.md §7) — booléens
            // simples, jamais masqués, persistés (pas dérivés : cf. contrat D2).
            setAiKeyVerified(!!data.ai.anthropic_api_key_verified)
            setGroqKeyVerified(!!data.ai.groq_api_key_verified)
            // Fix bloquant — capture la section complète pour la ré-émettre
            // entière à chaque POST (cf. commentaire sur aiSettings ci-dessus).
            setAiSettings({
              provider: savedProvider,
              model: data.ai.model || 'claude-opus-5',
              timeout_seconds: data.ai.timeout_seconds || 300,
              max_questions: data.ai.max_questions || 200,
              batch_size: data.ai.batch_size || 20,
              inter_batch_delay_ms: data.ai.inter_batch_delay_ms || 60000,
              context_token_budget: data.ai.context_token_budget || 1500,
              max_consecutive_failures: data.ai.max_consecutive_failures || 2,
              groq_model: data.ai.groq_model || 'openai/gpt-oss-120b',
            })
          }
        }
      } catch (error) {
        if (!cancelled) console.error('Failed to fetch config:', error)
      }
    }
    fetchConfig()
    return () => { cancelled = true }
  }, [])

  // Load WiFi defaults on mount
  useEffect(() => {
    let cancelled = false
    const fetchWifiDefaults = async () => {
      try {
        const res = await fetch('/api/wifi/defaults')
        if (cancelled) return
        if (res.ok) {
          const data = await res.json()
          if (cancelled) return
          if (data.ssid) setWifiSsid(data.ssid)
          if (data.password) setWifiPassword(data.password)
          if (data.ssid2) setWifiSsid2(data.ssid2)
          if (data.password2) setWifiPassword2(data.password2)
          if (data.server_ip) setWifiServerIp(data.server_ip)
          if (data.server_port) setWifiServerPort(data.server_port)
        }
      } catch (err) {
        // Defaults not available
      }
    }
    fetchWifiDefaults()
    return () => { cancelled = true }
  }, [])

  // Fetch firmware info on mount
  useEffect(() => {
    let cancelled = false
    const fetchFirmwareInfo = async () => {
      try {
        const res = await fetch('/api/firmware/buzzclick/version')
        if (cancelled) return
        if (res.ok) {
          const data = await res.json()
          if (cancelled) return
          setFirmwareInfo(data)
        }
      } catch {
        // Firmware endpoint not available (ignore)
      }
    }
    fetchFirmwareInfo()
    return () => { cancelled = true }
  }, [])

  // Sync defaultImageIsCustom from gameState (CONFIG_UPDATE broadcasts it)
  useEffect(() => {
    if (gameState?.defaultQuestionImageIsCustom !== undefined) {
      setDefaultImageIsCustom(gameState.defaultQuestionImageIsCustom)
    }
  }, [gameState?.defaultQuestionImageIsCustom])

  // Update firmware info from WebSocket broadcast (after upload)
  //
  // #136/#111 — cet effet dépend d'un OBJET (wsFirmwareInfo), pas d'une valeur
  // primitive. En production, useWebSocket.js expose un useState d'identité
  // stable entre deux rendus sans changement réel, donc l'effet est inoffensif.
  // Mais toute source qui recrée cet objet à chaque appel (un hook composé, un
  // mock de test littéral, un futur refactor) déclenche une boucle : l'effet se
  // relance à chaque rendu -> setFirmwareInfo -> nouveau rendu -> nouvel objet
  // -> effet relancé. On se protège en comparant le CONTENU avant d'écrire
  // l'état, pas seulement la référence : si le contenu n'a pas changé, le
  // setState fonctionnel renvoie la référence précédente et React ignore le
  // rendu (bail-out), ce qui casse la boucle indépendamment de la stabilité de
  // wsFirmwareInfo en amont.
  useEffect(() => {
    if (!wsFirmwareInfo) return
    setFirmwareInfo(prev => (
      prev && JSON.stringify(prev) === JSON.stringify(wsFirmwareInfo) ? prev : wsFirmwareInfo
    ))
  }, [wsFirmwareInfo])

  // Update local state when gameState.neonEffect changes (from WebSocket)
  // Même défaut de classe que ci-dessus (dépendance objet + setState de ce
  // même objet) — dormant aujourd'hui car aucune fixture de test ne peuple
  // gameState.neonEffect, mais corrigé par cohérence (#136 tâche 13).
  useEffect(() => {
    if (!gameState?.neonEffect) return
    setNeonConfig(prev => (
      prev && JSON.stringify(prev) === JSON.stringify(gameState.neonEffect) ? prev : gameState.neonEffect
    ))
  }, [gameState?.neonEffect])

  const handleSaveNeonConfig = async () => {
    setSavingNeon(true)
    try {
      const response = await fetch('/config.json', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ neon_effect: neonConfig })
      })
      if (!response.ok) {
        const text = await response.text()
        alert('Erreur: ' + text)
      }
    } catch (error) {
      console.error('Save neon config failed:', error)
      alert('Erreur: ' + error.message)
    } finally {
      setSavingNeon(false)
    }
  }

  const handleSaveServerParams = async () => {
    setSavingParams(true)
    try {
      const response = await fetch('/config.json', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          server: {
            auto_open_browsers: serverParams.auto_open_browsers,
            debug: serverParams.debug
          }
        })
      })
      if (!response.ok) {
        const text = await response.text()
        alert('Erreur: ' + text)
      }
    } catch (error) {
      console.error('Save server params failed:', error)
      alert('Erreur: ' + error.message)
    } finally {
      setSavingParams(false)
    }
  }

  // Validation de clé API par appel réel au fournisseur (contracts/ai-key-validation.md
  // §9, tâche #13 du plan) — factorisé pour les 2 fournisseurs, appelé uniquement par
  // handleSaveAiKey/handleSaveGroqKey (handleClearAiKey/handleClearGroqKey/
  // handleProviderChange restent hors scope, inchangés).
  // Persiste la clé (si fournie) + le flag *_verified. `trimmedKey` vide préserve la
  // clé existante côté serveur (règle de secret inchangée) ; seul le flag est mis à jour.
  const persistApiKey = async (cfg, trimmedKey, verified, message) => {
    const { provider, setConfigured, setVerified, setInput } = cfg
    const keyField = provider === 'anthropic' ? 'anthropic_api_key' : 'groq_api_key'
    const verifiedField = provider === 'anthropic' ? 'anthropic_api_key_verified' : 'groq_api_key_verified'
    const payload = {
      ai: {
        ...aiSettings,
        ...(trimmedKey ? { [keyField]: trimmedKey } : {}),
        [verifiedField]: verified,
      }
    }
    try {
      const response = await fetch('/config.json', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      })
      if (response.ok) {
        if (trimmedKey) {
          setConfigured(true)
          setInput('')
        }
        setVerified(verified)
        setAiToast({ message, type: verified ? 'success' : 'warning' })
      } else {
        const text = await response.text()
        setAiToast({ message: 'Erreur: ' + text, type: 'error' })
      }
    } catch (error) {
      console.error(`Save ${provider} key failed:`, error)
      setAiToast({ message: 'Erreur: ' + error.message, type: 'error' })
    }
  }

  // Appelle POST /api/ai/validate-key (contrat §5) puis agit selon le verdict :
  // valid -> enregistre directement ; invalid_key/unreachable -> ouvre le dialogue
  // bloquant (contrat §9). `keyToValidate` vide = valide la clé effective déjà stockée
  // côté serveur (§9 D3 — c'est aussi le seul chemin pour vérifier une clé fournie par
  // variable d'environnement, qui ne transite jamais par le champ de saisie).
  const validateAndProceed = async (cfg, keyToValidate) => {
    const { provider } = cfg
    let verdict
    try {
      const res = await fetch('/api/ai/validate-key', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ provider, api_key: keyToValidate })
      })
      if (!res.ok) {
        // Erreur de NOTRE serveur (400 préfixe invalide, 429 cooldown, ...) — pas un
        // verdict fournisseur au sens du contrat, donc pas de dialogue : toast classique.
        const text = await res.text()
        setAiToast({ message: 'Erreur: ' + text, type: 'error' })
        return
      }
      verdict = await res.json()
    } catch (error) {
      console.error(`Validate ${provider} key failed:`, error)
      setAiToast({ message: 'Erreur: ' + error.message, type: 'error' })
      return
    }

    if (verdict.result === 'valid') {
      await persistApiKey(cfg, keyToValidate, true, `Clé vérifiée auprès de ${PROVIDER_LABELS[provider]} et enregistrée.`)
      return
    }

    // invalid_key | unreachable — dialogue bloquant (contrat §9), rien n'est écrit.
    setKeyValidation({ ...cfg, trimmedKey: keyToValidate, result: verdict.result, httpStatus: verdict.http_status, detail: verdict.detail })
  }

  // Point d'entrée des boutons "Enregistrer" (contrat §9, séquence normative) :
  //   1. champ vide ET aucune clé stockée -> comportement actuel inchangé (pas de
  //      validation, enregistre les autres réglages ai.*)
  //   2. sinon -> validation réelle, cf. validateAndProceed
  const runKeyValidationFlow = async (cfg) => {
    const { trimmedKey, isConfigured, setSaving, setFieldError } = cfg
    setFieldError(null)
    setSaving(true)
    try {
      if (!trimmedKey && !isConfigured) {
        const response = await fetch('/config.json', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ ai: aiSettings })
        })
        if (response.ok) {
          setAiToast({ message: 'Clé API enregistrée', type: 'success' })
        } else {
          const text = await response.text()
          setAiToast({ message: 'Erreur: ' + text, type: 'error' })
        }
        return
      }
      await validateAndProceed(cfg, trimmedKey)
    } catch (error) {
      console.error(`Save ${cfg.provider} key failed:`, error)
      setAiToast({ message: 'Erreur: ' + error.message, type: 'error' })
    } finally {
      setSaving(false)
    }
  }

  // Dialogue de validation — "Corriger" (§9, ESC) : ne persiste rien, laisse le champ
  // tel quel (l'utilisateur peut le corriger) et affiche l'erreur inline sur le champ.
  const handleValidationCorrect = () => {
    if (!keyValidation) return
    const { provider, result, setFieldError } = keyValidation
    const label = PROVIDER_LABELS[provider]
    setFieldError(
      result === 'invalid_key'
        ? `Clé refusée par ${label}. Rien n'a été enregistré.`
        : `Impossible de vérifier la clé (${label} injoignable). Rien n'a été enregistré.`
    )
    setKeyValidation(null)
  }

  // "Enregistrer quand même" — décision consciente de l'opérateur (§9) : persiste la
  // clé avec *_verified: false, quel que soit le verdict (refus ou injoignable).
  const handleValidationForceSave = async () => {
    if (!keyValidation) return
    const cfg = keyValidation
    setKeyValidation(null)
    setForcingValidationSave(true)
    try {
      await persistApiKey(
        cfg,
        cfg.trimmedKey,
        false,
        `Clé enregistrée sans vérification. Elle n'a pas été confirmée par ${PROVIDER_LABELS[cfg.provider]}.`
      )
    } finally {
      setForcingValidationSave(false)
    }
  }

  // "Réessayer" — uniquement pour `unreachable` (le dialogue ne l'affiche pas pour
  // `invalid_key`, cf. ApiKeyValidationDialog) : relance la même validation.
  const handleValidationRetry = async () => {
    if (!keyValidation) return
    const cfg = keyValidation
    setKeyValidation(null)
    setRetryingValidation(true)
    try {
      await validateAndProceed(cfg, cfg.trimmedKey)
    } finally {
      setRetryingValidation(false)
    }
  }

  // AI: enregistrer la clé — un champ laissé vide NE modifie PAS la clé
  // existante côté serveur (contract ai-generation.md §2, maquette §9).
  // Le payload ré-émet TOUJOURS la section `ai` complète (aiSettings) — un
  // payload partiel ({ai: {anthropic_api_key: "..."}} seul) remettrait
  // provider/batch_size/etc à leurs défauts (fix bloquant, cf. commentaire
  // sur aiSettings plus haut).
  const handleSaveAiKey = () => runKeyValidationFlow({
    provider: 'anthropic',
    trimmedKey: aiApiKeyInput.trim(),
    isConfigured: aiKeyConfigured,
    setConfigured: setAiKeyConfigured,
    setVerified: setAiKeyVerified,
    setInput: setAiApiKeyInput,
    setSaving: setSavingAiKey,
    setFieldError: setAiKeyFieldError,
  })

  // AI: suppression explicite, via le bouton dédié (maquette §9) — distincte
  // d'un simple "Enregistrer" avec champ vide, qui préserve la clé.
  const handleClearAiKey = async () => {
    if (!window.confirm('Supprimer la clé API Claude enregistrée ?')) return
    setClearingAiKey(true)
    try {
      const response = await fetch('/config.json', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ai: { ...aiSettings, clear_api_key: true } })
      })
      if (response.ok) {
        setAiKeyConfigured(false)
        setAiApiKeyInput('')
        setAiToast({ message: 'Clé API supprimée', type: 'success' })
      } else {
        const text = await response.text()
        setAiToast({ message: 'Erreur: ' + text, type: 'error' })
      }
    } catch (error) {
      console.error('Clear AI key failed:', error)
      setAiToast({ message: 'Erreur: ' + error.message, type: 'error' })
    } finally {
      setClearingAiKey(false)
    }
  }

  // #137 — bascule le fournisseur actif. Persisté immédiatement (pas de
  // bouton "Enregistrer" séparé pour ce champ) — mise à jour optimiste avec
  // rollback si le POST échoue.
  const handleProviderChange = async (nextProvider) => {
    if (nextProvider === aiProvider || savingProvider) return
    const previous = aiProvider
    setAiProvider(nextProvider)
    setAiSettings(prev => ({ ...prev, provider: nextProvider }))
    setSavingProvider(true)
    try {
      const response = await fetch('/config.json', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ai: { ...aiSettings, provider: nextProvider } })
      })
      if (!response.ok) {
        setAiProvider(previous)
        setAiSettings(prev => ({ ...prev, provider: previous }))
        const text = await response.text()
        setAiToast({ message: 'Erreur: ' + text, type: 'error' })
      }
    } catch (error) {
      setAiProvider(previous)
      setAiSettings(prev => ({ ...prev, provider: previous }))
      console.error('Save AI provider failed:', error)
      setAiToast({ message: 'Erreur: ' + error.message, type: 'error' })
    } finally {
      setSavingProvider(false)
    }
  }

  // Groq: mêmes règles de secret que la clé Claude (contract §8) — champ vide
  // préserve la clé existante, suppression via bouton dédié.
  const handleSaveGroqKey = () => runKeyValidationFlow({
    provider: 'groq',
    trimmedKey: groqApiKeyInput.trim(),
    isConfigured: groqKeyConfigured,
    setConfigured: setGroqKeyConfigured,
    setVerified: setGroqKeyVerified,
    setInput: setGroqApiKeyInput,
    setSaving: setSavingGroqKey,
    setFieldError: setGroqKeyFieldError,
  })

  const handleClearGroqKey = async () => {
    if (!window.confirm('Supprimer la clé API Groq enregistrée ?')) return
    setClearingGroqKey(true)
    try {
      const response = await fetch('/config.json', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ai: { ...aiSettings, clear_groq_api_key: true } })
      })
      if (response.ok) {
        setGroqKeyConfigured(false)
        setGroqApiKeyInput('')
        setAiToast({ message: 'Clé API supprimée', type: 'success' })
      } else {
        const text = await response.text()
        setAiToast({ message: 'Erreur: ' + text, type: 'error' })
      }
    } catch (error) {
      console.error('Clear Groq key failed:', error)
      setAiToast({ message: 'Erreur: ' + error.message, type: 'error' })
    } finally {
      setClearingGroqKey(false)
    }
  }

  const handleResetScores = () => {
    if (!window.confirm('Remettre tous les scores a zero ?')) return
    sendMessage('RAZ', {})
  }

  const handleSaveWifiDefaults = async () => {
    setSavingWifi(true)
    try {
      const res = await fetch('/api/wifi/defaults', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          ssid: wifiSsid,
          password: wifiPassword,
          ssid2: wifiSsid2,
          password2: wifiPassword2,
          server_ip: wifiServerIp,
          server_port: wifiServerPort
        })
      })
      if (res.ok) {
        setWifiToast({ message: 'Configuration WiFi sauvegardee', type: 'success' })
      } else {
        const text = await res.text()
        setWifiToast({ message: 'Erreur: ' + text, type: 'error' })
      }
    } catch (err) {
      setWifiToast({ message: 'Erreur: ' + err.message, type: 'error' })
    } finally {
      setSavingWifi(false)
    }
  }

  const handleBroadcastWifi = async () => {
    setBroadcastingWifi(true)
    try {
      const res = await fetch('/api/buzzer/wifi-config', {
        method: 'POST'
      })
      if (res.ok) {
        const data = await res.json()
        setWifiToast({ message: `Config WiFi envoyee a ${data.count} buzzer(s)`, type: 'success' })
      } else {
        const text = await res.text()
        setWifiToast({ message: 'Erreur: ' + text, type: 'error' })
      }
    } catch (err) {
      setWifiToast({ message: 'Erreur: ' + err.message, type: 'error' })
    } finally {
      setBroadcastingWifi(false)
    }
  }

  const handleFirmwareUpload = async () => {
    const file = firmwareFileRef.current?.files?.[0]
    if (!file) {
      setFirmwareToast({ message: 'Veuillez selectionner un fichier .bin', type: 'error' })
      return
    }
    // Client-side validation
    if (!file.name.endsWith('.bin')) {
      setFirmwareToast({ message: 'Le fichier doit etre au format .bin', type: 'error' })
      return
    }
    const sizeMB = file.size / (1024 * 1024)
    if (file.size < 200 * 1024) {
      setFirmwareToast({ message: 'Fichier trop petit (minimum 200 Ko)', type: 'error' })
      return
    }
    if (sizeMB > 2) {
      setFirmwareToast({ message: 'Fichier trop grand (maximum 2 Mo)', type: 'error' })
      return
    }

    setUploadingFirmware(true)
    try {
      const formData = new FormData()
      formData.append('file', file)
      const res = await fetch('/api/firmware/buzzclick/upload', {
        method: 'POST',
        body: formData,
      })
      const data = await res.json()
      if (res.ok && data.status === 'ok') {
        setFirmwareInfo(prev => ({ ...prev, VERSION: data.version, SIZE: data.size, EXISTS: true, FILENAME: file.name, IS_MERGED: data.is_merged === true }))
        setFirmwareToast({ message: `Firmware ${data.version} uploade avec succes`, type: 'success' })
        if (firmwareFileRef.current) firmwareFileRef.current.value = ''
      } else {
        setFirmwareToast({ message: 'Erreur: ' + (data.message || 'Upload echoue'), type: 'error' })
      }
    } catch (err) {
      setFirmwareToast({ message: 'Erreur reseau: ' + err.message, type: 'error' })
    } finally {
      setUploadingFirmware(false)
    }
  }

  const handleRestoreEmbedded = async () => {
    setRestoringEmbedded(true)
    try {
      const res = await fetch('/api/firmware/buzzclick/restore-embedded', { method: 'POST' })
      const data = await res.json()
      if (res.ok && data.status === 'ok') {
        setFirmwareInfo(prev => ({ ...prev, VERSION: data.version, SIZE: data.size, EXISTS: true, FILENAME: data.filename }))
        setFirmwareToast({ message: `Firmware v${data.version} restaure (firmware embarque)`, type: 'success' })
      } else {
        setFirmwareToast({ message: 'Erreur: ' + (data.message || 'Restauration echouee'), type: 'error' })
      }
    } catch (err) {
      setFirmwareToast({ message: 'Erreur reseau: ' + err.message, type: 'error' })
    } finally {
      setRestoringEmbedded(false)
    }
  }

  const handleUpdateAll = () => {
    setShowOtaAllModal(true)
  }

  const handleLoadDemo = async () => {
    if (!window.confirm('Charger les donnees de demonstration ? Les donnees actuelles seront remplacees.')) return

    setLoadingDemo(true)
    try {
      const response = await fetch('/load-demo', { method: 'POST' })
      if (response.ok) {
        window.location.reload()
      } else {
        const data = await response.json()
        alert('Erreur: ' + (data.message || 'Echec du chargement'))
      }
    } catch (error) {
      console.error('Load demo failed:', error)
      alert('Erreur: ' + error.message)
    } finally {
      setLoadingDemo(false)
    }
  }

  const handleDefaultImageUpload = async () => {
    const file = defaultImageFileRef.current?.files?.[0]
    if (!file) {
      setDefaultImageToast({ message: 'Veuillez selectionner une image', type: 'error' })
      return
    }
    const allowed = ['.jpg', '.jpeg', '.png', '.gif', '.webp', '.svg']
    const ext = '.' + file.name.split('.').pop().toLowerCase()
    if (!allowed.includes(ext)) {
      setDefaultImageToast({ message: 'Format non supporte. Utilisez jpg, png, gif, webp ou svg', type: 'error' })
      return
    }
    setUploadingDefaultImage(true)
    try {
      const formData = new FormData()
      formData.append('file', file)
      const res = await fetch('/api/config/default-image', { method: 'POST', body: formData })
      const data = await res.json()
      if (res.ok && data.is_custom) {
        setDefaultImageIsCustom(true)
        setDefaultImageCacheBuster(Date.now())
        setDefaultImageToast({ message: 'Image par defaut enregistree', type: 'success' })
        if (defaultImageFileRef.current) defaultImageFileRef.current.value = ''
      } else {
        setDefaultImageToast({ message: 'Erreur lors de l\'upload', type: 'error' })
      }
    } catch (err) {
      setDefaultImageToast({ message: 'Erreur reseau: ' + err.message, type: 'error' })
    } finally {
      setUploadingDefaultImage(false)
    }
  }

  const handleDefaultImageDelete = async () => {
    if (!window.confirm('Supprimer l\'image personnalisee ? L\'image par defaut embarquee sera utilisee.')) return
    setDeletingDefaultImage(true)
    try {
      const res = await fetch('/api/config/default-image', { method: 'DELETE' })
      if (res.ok) {
        setDefaultImageIsCustom(false)
        setDefaultImageCacheBuster(Date.now())
        setDefaultImageToast({ message: 'Image personnalisee supprimee', type: 'success' })
      } else {
        setDefaultImageToast({ message: 'Erreur lors de la suppression', type: 'error' })
      }
    } catch (err) {
      setDefaultImageToast({ message: 'Erreur reseau: ' + err.message, type: 'error' })
    } finally {
      setDeletingDefaultImage(false)
    }
  }


  return (
    <div className="config-page page">
      <header className="page-header">
        <h1 className="page-title">Configuration</h1>
        <p className="page-subtitle">Parametres du systeme</p>
      </header>

      <div className="config-layout">
        {/* System Section */}
        <section className="system-section">
          <h2 className="section-title">Systeme</h2>

          <Card padding="lg" className="system-card">
            <div className="system-info">
              {version && (
                <div className="info-item">
                  <span className="info-label">Version serveur</span>
                  <span className="info-value">{version}</span>
                </div>
              )}
              <div className="info-item">
                <span className="info-label">Equipes</span>
                <span className="info-value">{Object.keys(teams).length}</span>
              </div>
              <div className="info-item">
                <span className="info-label">Buzzers</span>
                <span className="info-value">{Object.keys(bumpers).length}</span>
              </div>
            </div>

            <div className="system-actions">
              <Button variant="secondary" onClick={handleResetScores}>
                Remettre les scores a zero
              </Button>
            </div>

            {/* Server Parameters Section */}
            <div className="config-section">
              <h3 className="config-section-title">Parametres serveur</h3>
              <p className="config-section-hint">
                Configuration du comportement du serveur au demarrage et mode de fonctionnement.
              </p>

              <label className="checkbox-item">
                <input
                  type="checkbox"
                  checked={serverParams.auto_open_browsers}
                  onChange={(e) => setServerParams(prev => ({ ...prev, auto_open_browsers: e.target.checked }))}
                />
                <span>Ouvrir les navigateurs automatiquement</span>
              </label>

              <label className="checkbox-item">
                <input
                  type="checkbox"
                  checked={serverParams.debug}
                  onChange={(e) => setServerParams(prev => ({ ...prev, debug: e.target.checked }))}
                />
                <span>Mode debug</span>
              </label>

              <div className="config-section-actions">
                <Button variant="primary" onClick={handleSaveServerParams} loading={savingParams}>
                  Enregistrer
                </Button>
              </div>
            </div>

            {/* WiFi Config Section */}
            <div className="config-section">
              <h3 className="config-section-title">Configuration du WiFi</h3>
              <p className="config-section-hint">
                Parametres WiFi par defaut pour les buzzers. Ces valeurs seront envoyees aux buzzers lors de la configuration USB.
              </p>

              <div className="udp-discovery-info">
                <span className="udp-discovery-icon">📡</span>
                <span>L'adresse IP du serveur est decouverte automatiquement par les buzzers via UDP broadcast.</span>
              </div>

              <div className="wifi-form">
                <label className="wifi-field">
                  <span>SSID WiFi</span>
                  <input
                    type="text"
                    value={wifiSsid}
                    onChange={(e) => setWifiSsid(e.target.value)}
                    placeholder="Nom du reseau WiFi"
                    maxLength={32}
                  />
                </label>
                <label className="wifi-field">
                  <span>Mot de passe WiFi</span>
                  <input
                    type="password"
                    value={wifiPassword}
                    onChange={(e) => setWifiPassword(e.target.value)}
                    placeholder="Mot de passe (min 8 car.)"
                    maxLength={63}
                  />
                </label>
              </div>

              <div className="wifi-fallback-section">
                <h4 className="wifi-fallback-title">WiFi de secours (optionnel)</h4>
                <div className="wifi-form">
                  <label className="wifi-field">
                    <span>SSID WiFi 2 (optionnel)</span>
                    <input
                      type="text"
                      value={wifiSsid2}
                      onChange={(e) => setWifiSsid2(e.target.value)}
                      placeholder="Nom du reseau WiFi secondaire"
                      maxLength={32}
                    />
                  </label>
                  <label className="wifi-field">
                    <span>Mot de passe WiFi 2</span>
                    <input
                      type="password"
                      value={wifiPassword2}
                      onChange={(e) => setWifiPassword2(e.target.value)}
                      placeholder="Mot de passe (min 8 car.)"
                      maxLength={63}
                    />
                  </label>
                </div>
              </div>

              <div className="config-section-actions">
                <Button variant="primary" onClick={handleSaveWifiDefaults} loading={savingWifi}>
                  Sauvegarder
                </Button>
                <Button variant="secondary" onClick={() => setShowUSBModal(true)}>
                  Configuration via USB
                </Button>
              </div>

              <div className="wifi-broadcast-section">
                <div className="wifi-broadcast-warning">
                  <span className="warning-icon">⚠️</span>
                  <span>Les buzzers qui changent de reseau WiFi vont redemarrer automatiquement.</span>
                </div>
                <Button variant="secondary" onClick={handleBroadcastWifi} loading={broadcastingWifi}>
                  Appliquer a tous les buzzers connectes
                </Button>
              </div>
            </div>

            {/* Firmware Buzzers Section */}
            <div className="config-section">
              <h3 className="config-section-title">Firmware Buzzers</h3>
              <p className="config-section-hint">
                Gerez le firmware des buzzers BuzzClick. Uploadez un fichier .bin de reference
                et lancez les mises a jour OTA sur les buzzers connectes.
              </p>

              {/* Current firmware info */}
              <div className="firmware-info-grid">
                <div className="firmware-info-item">
                  <span className="firmware-info-label">Version de reference</span>
                  <span className="firmware-info-value">
                    {firmwareInfo?.VERSION || <span className="firmware-none">aucune</span>}
                  </span>
                </div>
                <div className="firmware-info-item">
                  <span className="firmware-info-label">Fichier</span>
                  <span className="firmware-info-value firmware-filename">
                    {firmwareInfo?.FILENAME || '-'}
                  </span>
                </div>
                <div className="firmware-info-item">
                  <span className="firmware-info-label">Taille</span>
                  <span className="firmware-info-value">
                    {firmwareInfo?.EXISTS ? `${Math.round(firmwareInfo.SIZE / 1024)} Ko` : '—'}
                  </span>
                </div>
                <div className="firmware-info-item">
                  <span className="firmware-info-label">Etat</span>
                  <div className="firmware-status-row">
                    <span className={`firmware-status-badge ${firmwareInfo?.EXISTS ? 'exists' : firmwareInfo?.VERSION ? 'pending' : 'missing'}`}>
                      {firmwareInfo?.EXISTS ? 'Disponible' : firmwareInfo?.VERSION ? 'Non uploade' : 'Absent'}
                    </span>
                    {firmwareInfo?.EXISTS && (
                      <span className={`firmware-type-badge ${firmwareInfo.IS_MERGED ? 'merged' : 'app-only'}`}>
                        {firmwareInfo.IS_MERGED ? 'Full (merged)' : 'App only'}
                      </span>
                    )}
                  </div>
                </div>
                {firmwareInfo?.EMBEDDED_VERSION && (
                  <div className="firmware-info-item">
                    <span className="firmware-info-label">Version embarquee</span>
                    <span className="firmware-info-value">{firmwareInfo.EMBEDDED_VERSION}</span>
                  </div>
                )}
              </div>

              {/* File upload */}
              <div className="firmware-upload-row">
                <input
                  ref={firmwareFileRef}
                  type="file"
                  accept=".bin"
                  className="firmware-file-input"
                  id="firmware-file-input"
                />
                <label htmlFor="firmware-file-input" className="firmware-file-label">
                  Choisir un fichier .bin (200 Ko - 2 Mo)
                </label>
              </div>

              <div className="config-section-actions">
                <Button
                  variant="primary"
                  onClick={handleFirmwareUpload}
                  loading={uploadingFirmware}
                >
                  Uploader le firmware
                </Button>
                {firmwareInfo?.EMBEDDED_VERSION && firmwareInfo?.VERSION !== firmwareInfo?.EMBEDDED_VERSION && (
                  <Button
                    variant="secondary"
                    onClick={handleRestoreEmbedded}
                    loading={restoringEmbedded}
                  >
                    Restaurer v{firmwareInfo.EMBEDDED_VERSION}
                  </Button>
                )}
                <Button
                  variant="secondary"
                  onClick={handleUpdateAll}
                  disabled={!firmwareInfo?.EXISTS || outdatedCount === 0}
                >
                  {outdatedCount > 0
                    ? `Mettre a jour les ${outdatedCount} buzzer${outdatedCount > 1 ? 's' : ''} obsoletes`
                    : 'Mettre a jour les buzzers obsoletes'}
                </Button>
                <Button
                  variant="secondary"
                  onClick={() => setShowUSBModal(true)}
                  disabled={!firmwareInfo?.IS_MERGED}
                >
                  Flash via USB
                </Button>
              </div>

            </div>

            {/* Default Question Image Section */}
            <div className="config-section">
              <h3 className="config-section-title">Image par defaut</h3>
              <p className="config-section-hint">
                Image affichee sur l'ecran TV pour les questions sans image. Une image SVG est embarquee par defaut ; vous pouvez la remplacer par votre propre image.
              </p>

              <div className="default-image-preview">
                <img
                  src={`/api/config/default-image?t=${defaultImageCacheBuster}`}
                  alt="Image par defaut"
                  className="default-image-thumbnail"
                />
                <span className="default-image-filename">
                  {defaultImageIsCustom ? 'Image personnalisee' : 'Image embarquee (defaut)'}
                </span>
              </div>

              <div className="firmware-upload-row">
                <input
                  ref={defaultImageFileRef}
                  type="file"
                  accept=".jpg,.jpeg,.png,.gif,.webp,.svg"
                  className="firmware-file-input"
                  id="default-image-file-input"
                />
                <label htmlFor="default-image-file-input" className="firmware-file-label">
                  Choisir une image (jpg, png, gif, webp, svg)
                </label>
              </div>

              <div className="config-section-actions">
                <Button variant="primary" onClick={handleDefaultImageUpload} loading={uploadingDefaultImage}>
                  Enregistrer
                </Button>
                {defaultImageIsCustom && (
                  <Button variant="secondary" onClick={handleDefaultImageDelete} loading={deletingDefaultImage}>
                    Restaurer l'image embarquee
                  </Button>
                )}
              </div>
            </div>

            {/* Demo Section */}
            <div className="config-section">
              <h3 className="config-section-title">Mode Demo</h3>
              <p className="config-section-hint">
                Charge des donnees de demonstration: equipes, joueurs, questions (QCM, Memory, Normal) et historique.
              </p>
              <div className="config-section-actions">
                <Button variant="primary" onClick={handleLoadDemo} loading={loadingDemo}>
                  Charger la demo
                </Button>
              </div>
            </div>

            {/* AI Generation Section (v6.0.0 #8, fournisseurs multiples v6.1.0 #137) */}
            <div className="config-section">
              <h3 className="config-section-title">IA</h3>
              <p className="config-section-hint">
                Génération automatique de questions depuis QuestionsPage. Choisissez un fournisseur
                et renseignez sa clé API — le bouton « ✨ Générer via IA » s'active selon la clé du
                fournisseur sélectionné ci-dessous.
              </p>

              {/* #137 — sélecteur de fournisseur (maquette 137-generation-tache-de-fond.md §7) */}
              <div className="ai-provider-row">
                <span className="ai-provider-label">Fournisseur</span>
                <div className="mode-selector">
                  <button
                    type="button"
                    className={`mode-btn ${aiProvider !== 'groq' ? 'active' : ''}`}
                    onClick={() => handleProviderChange('anthropic')}
                    disabled={savingProvider}
                  >
                    Claude (Anthropic)
                  </button>
                  <button
                    type="button"
                    className={`mode-btn ${aiProvider === 'groq' ? 'active' : ''}`}
                    onClick={() => handleProviderChange('groq')}
                    disabled={savingProvider}
                  >
                    Groq
                  </button>
                </div>
              </div>

              {/* Simplification #7 (bugfix/config-api-key-help) — sélection strictement
                  manuelle via le sélecteur ci-dessus : seule la carte du fournisseur
                  actif est affichée, l'autre est masquée (pas de logique d'auto-pick). */}
              {aiProvider !== 'groq' ? (
                <div className="ai-provider-block">
                  <div className="ai-key-status">
                    <span className="ai-provider-block-title">Claude (Anthropic)</span>
                    {/* Badge tri-état (contracts/ai-key-validation.md §7, tâche #13) —
                        pas de clé / clé enregistrée mais non vérifiée / clé vérifiée. */}
                    <span className={`ai-key-badge ${aiKeyConfigured ? (aiKeyVerified ? 'configured' : 'unverified') : 'missing'}`}>
                      {aiKeyConfigured ? (aiKeyVerified ? '✅ Clé vérifiée' : '⚠️ Clé non vérifiée') : '⚠️ Aucune clé'}
                    </span>
                  </div>
                  <p className="ai-provider-caveat">Payant, rapide (1 à 3 min pour 200 questions).</p>
                  <label className="wifi-field">
                    <span className="wifi-field-label-row">
                      Clé API Claude
                      <button
                        type="button"
                        className="api-key-help-btn"
                        onClick={() => setApiKeyHelpProvider('anthropic')}
                        aria-label="Comment obtenir une clé API Anthropic ?"
                        title="Comment obtenir une clé API ?"
                      >
                        ?
                      </button>
                    </span>
                    <input
                      type="password"
                      className={aiKeyFieldError ? 'invalid' : ''}
                      value={aiApiKeyInput}
                      onChange={(e) => { setAiApiKeyInput(e.target.value); setAiKeyFieldError(null) }}
                      placeholder={aiKeyConfigured ? '••••••••' : 'sk-ant-...'}
                      autoComplete="off"
                    />
                    {aiKeyFieldError && <span className="wifi-field-error">{aiKeyFieldError}</span>}
                  </label>
                  <div className="config-section-actions">
                    <Button variant="primary" size="sm" onClick={handleSaveAiKey} loading={savingAiKey}>
                      Enregistrer
                    </Button>
                    {aiKeyConfigured && (
                      <Button variant="secondary" size="sm" onClick={handleClearAiKey} loading={clearingAiKey}>
                        Supprimer la clé
                      </Button>
                    )}
                  </div>
                </div>
              ) : (
                <div className="ai-provider-block">
                  <div className="ai-key-status">
                    <span className="ai-provider-block-title">Groq</span>
                    {/* Badge tri-état (contracts/ai-key-validation.md §7, tâche #13) —
                        pas de clé / clé enregistrée mais non vérifiée / clé vérifiée. */}
                    <span className={`ai-key-badge ${groqKeyConfigured ? (groqKeyVerified ? 'configured' : 'unverified') : 'missing'}`}>
                      {groqKeyConfigured ? (groqKeyVerified ? '✅ Clé vérifiée' : '⚠️ Clé non vérifiée') : '⚠️ Aucune clé'}
                    </span>
                  </div>
                  <p className="ai-provider-caveat">
                    Gratuit, mais limité en débit : comptez ~10 minutes pour 200 questions.
                  </p>
                  <label className="wifi-field">
                    <span className="wifi-field-label-row">
                      Clé API Groq
                      <button
                        type="button"
                        className="api-key-help-btn"
                        onClick={() => setApiKeyHelpProvider('groq')}
                        aria-label="Comment obtenir une clé API Groq ?"
                        title="Comment obtenir une clé API ?"
                      >
                        ?
                      </button>
                    </span>
                    <input
                      type="password"
                      className={groqKeyFieldError ? 'invalid' : ''}
                      value={groqApiKeyInput}
                      onChange={(e) => { setGroqApiKeyInput(e.target.value); setGroqKeyFieldError(null) }}
                      placeholder={groqKeyConfigured ? '••••••••' : 'gsk_...'}
                      autoComplete="off"
                    />
                    {groqKeyFieldError && <span className="wifi-field-error">{groqKeyFieldError}</span>}
                  </label>
                  <div className="config-section-actions">
                    <Button variant="primary" size="sm" onClick={handleSaveGroqKey} loading={savingGroqKey}>
                      Enregistrer
                    </Button>
                    {groqKeyConfigured && (
                      <Button variant="secondary" size="sm" onClick={handleClearGroqKey} loading={clearingGroqKey}>
                        Supprimer la clé
                      </Button>
                    )}
                  </div>
                </div>
              )}

              <p className="config-section-hint">
                Les clés sont stockées localement sur le serveur, jamais renvoyées au navigateur.
                Laisser un champ vide et Enregistrer conserve la clé existante.
              </p>
            </div>

            {/* Neon Effect Section */}
            <div className="config-section">
              <h3 className="config-section-title">Effet Neon</h3>
              <p className="config-section-hint">
                Bordure lumineuse animee autour de l'ecran TV et VJoueur, avec la couleur de la categorie.
              </p>

              <label className="checkbox-item neon-toggle">
                <input
                  type="checkbox"
                  checked={neonConfig.enabled}
                  onChange={(e) => setNeonConfig(prev => ({ ...prev, enabled: e.target.checked }))}
                />
                <span>Activer l'effet neon</span>
              </label>

              {neonConfig.enabled && (
                <div className="neon-sliders">
                  {/* Mode selector */}
                  <div className="slider-row">
                    <label>Mode d'affichage</label>
                    <div className="mode-selector">
                      <button
                        className={`mode-btn ${neonConfig.mode !== 'halo' ? 'active' : ''}`}
                        onClick={() => setNeonConfig(prev => ({ ...prev, mode: 'bar' }))}
                      >
                        Neon
                      </button>
                      <button
                        className={`mode-btn ${neonConfig.mode === 'halo' ? 'active' : ''}`}
                        onClick={() => setNeonConfig(prev => ({ ...prev, mode: 'halo' }))}
                      >
                        Halo
                      </button>
                    </div>
                  </div>

                  {/* Bar mode specific settings */}
                  {neonConfig.mode !== 'halo' && (
                    <>
                      <div className="slider-row">
                        <label>Distance du bord</label>
                        <div className="slider-control">
                          <input
                            type="range"
                            min="10"
                            max="100"
                            value={neonConfig.bar_offset || 20}
                            onChange={(e) => setNeonConfig(prev => ({ ...prev, bar_offset: parseInt(e.target.value) }))}
                          />
                          <span className="slider-value">{neonConfig.bar_offset || 20}px</span>
                        </div>
                      </div>

                      <div className="slider-row">
                        <label>Epaisseur de la barre</label>
                        <div className="slider-control">
                          <input
                            type="range"
                            min="2"
                            max="20"
                            value={neonConfig.bar_thickness || 4}
                            onChange={(e) => setNeonConfig(prev => ({ ...prev, bar_thickness: parseInt(e.target.value) }))}
                          />
                          <span className="slider-value">{neonConfig.bar_thickness || 4}px</span>
                        </div>
                      </div>
                    </>
                  )}

                  {/* Glow section - grouped */}
                  <div className="neon-glow-section">
                    <h4 className="neon-subsection-title">Glow</h4>

                    <div className="slider-row">
                      <label>Vitesse pulsation</label>
                      <div className="slider-control">
                        <input
                          type="range"
                          min="0.5"
                          max="5"
                          step="0.1"
                          value={neonConfig.glow_pulse_speed || 2}
                          onChange={(e) => setNeonConfig(prev => ({ ...prev, glow_pulse_speed: parseFloat(e.target.value) }))}
                        />
                        <span className="slider-value">{neonConfig.glow_pulse_speed || 2}s</span>
                      </div>
                    </div>

                    <div className="slider-row">
                      <label>Amplitude pulsation</label>
                      <div className="dual-range-container">
                        <div className="dual-range-track" ref={dualRangeTrackRef}>
                          <div
                            className="dual-range-fill"
                            style={{
                              left: `${neonConfig.glow_pulse_min || 30}%`,
                              width: `${(neonConfig.glow_pulse_max || 50) - (neonConfig.glow_pulse_min || 30)}%`,
                              background: `linear-gradient(to right,
                                rgba(0, 200, 200, ${(neonConfig.glow_pulse_min || 30) / 100}),
                                rgba(0, 200, 200, ${(neonConfig.glow_pulse_max || 50) / 100}))`
                            }}
                            onMouseDown={(e) => {
                              e.preventDefault()
                              const track = dualRangeTrackRef.current
                              if (!track) return
                              const trackRect = track.getBoundingClientRect()
                              const startX = e.clientX
                              const startMin = neonConfig.glow_pulse_min || 30
                              const startMax = neonConfig.glow_pulse_max || 50
                              const gap = startMax - startMin

                              const onMouseMove = (moveEvent) => {
                                const deltaX = moveEvent.clientX - startX
                                const deltaPercent = (deltaX / trackRect.width) * 100
                                let newMin = Math.round(startMin + deltaPercent)
                                let newMax = Math.round(startMax + deltaPercent)

                                // Clamp to boundaries - min cannot go below 1%
                                if (newMin < 1) {
                                  newMin = 1
                                  newMax = 1 + gap
                                }
                                // max cannot go above 100
                                if (newMax > 100) {
                                  newMax = 100
                                  newMin = 100 - gap
                                }

                                // Final safety clamp (min at least 1%)
                                newMin = Math.max(1, Math.min(100, newMin))
                                newMax = Math.max(1, Math.min(100, newMax))

                                setNeonConfig(prev => ({
                                  ...prev,
                                  glow_pulse_min: newMin,
                                  glow_pulse_max: newMax
                                }))
                              }

                              const onMouseUp = () => {
                                document.removeEventListener('mousemove', onMouseMove)
                                document.removeEventListener('mouseup', onMouseUp)
                              }

                              document.addEventListener('mousemove', onMouseMove)
                              document.addEventListener('mouseup', onMouseUp)
                            }}
                          />
                          <input
                            type="range"
                            className="dual-range-input dual-range-min"
                            min="1"
                            max="100"
                            value={neonConfig.glow_pulse_min || 30}
                            onChange={(e) => {
                              const val = parseInt(e.target.value)
                              const max = neonConfig.glow_pulse_max || 50
                              setNeonConfig(prev => ({
                                ...prev,
                                glow_pulse_min: Math.max(1, Math.min(val, max - 5))
                              }))
                            }}
                          />
                          <input
                            type="range"
                            className="dual-range-input dual-range-max"
                            min="0"
                            max="100"
                            value={neonConfig.glow_pulse_max || 50}
                            onChange={(e) => {
                              const val = parseInt(e.target.value)
                              const min = neonConfig.glow_pulse_min || 30
                              setNeonConfig(prev => ({
                                ...prev,
                                glow_pulse_max: Math.max(val, min + 5)
                              }))
                            }}
                          />
                        </div>
                        <span className="slider-value">{neonConfig.glow_pulse_min || 30}% - {neonConfig.glow_pulse_max || 50}%</span>
                      </div>
                    </div>
                  </div>

                  {/* Arc section - grouped */}
                  <div className="neon-arc-section">
                    <h4 className="neon-subsection-title">Arc lumineux</h4>

                    <div className="slider-row">
                      <label>Intensite</label>
                      <div className="slider-control">
                        <input
                          type="range"
                          min="0"
                          max="100"
                          value={neonConfig.intensity_gap}
                          onChange={(e) => setNeonConfig(prev => ({ ...prev, intensity_gap: parseInt(e.target.value) }))}
                        />
                        <span className="slider-value">{neonConfig.intensity_gap}%</span>
                      </div>
                    </div>

                    <div className="slider-row">
                      <label>Largeur</label>
                      <div className="slider-control">
                        <input
                          type="range"
                          min="30"
                          max="180"
                          value={neonConfig.arc_width}
                          onChange={(e) => setNeonConfig(prev => ({ ...prev, arc_width: parseInt(e.target.value) }))}
                        />
                        <span className="slider-value">{neonConfig.arc_width}°</span>
                      </div>
                    </div>

                    <div className="slider-row">
                      <label>Epaisseur</label>
                      <div className="slider-control">
                        <input
                          type="range"
                          min="0"
                          max="200"
                          step="10"
                          value={neonConfig.arc_blur !== undefined ? neonConfig.arc_blur : 100}
                          onChange={(e) => setNeonConfig(prev => ({ ...prev, arc_blur: parseInt(e.target.value) }))}
                        />
                        <span className="slider-value">{neonConfig.arc_blur !== undefined ? neonConfig.arc_blur : 100}%</span>
                      </div>
                    </div>

                    <div className="slider-row">
                      <label>Vitesse</label>
                      <div className="slider-control">
                        <input
                          type="range"
                          min="1"
                          max="10"
                          step="0.5"
                          value={neonConfig.rotation_speed}
                          onChange={(e) => setNeonConfig(prev => ({ ...prev, rotation_speed: parseFloat(e.target.value) }))}
                        />
                        <span className="slider-value">{neonConfig.rotation_speed}s</span>
                      </div>
                    </div>
                  </div>
                </div>
              )}

              <div className="config-section-actions">
                <Button variant="primary" onClick={handleSaveNeonConfig} loading={savingNeon}>
                  Enregistrer
                </Button>
              </div>
            </div>

          </Card>
        </section>
      </div>

      {showUSBModal && (
        <USBConfigModal
          onClose={() => setShowUSBModal(false)}
          wifiConfig={{ ssid: wifiSsid, password: wifiPassword, serverIp: wifiServerIp, serverPort: wifiServerPort }}
          firmwareInfo={firmwareInfo}
        />
      )}

      {apiKeyHelpProvider && (
        <ApiKeyHelpModal
          provider={apiKeyHelpProvider}
          onClose={() => setApiKeyHelpProvider(null)}
        />
      )}

      {keyValidation && (
        <ApiKeyValidationDialog
          provider={keyValidation.provider}
          result={keyValidation.result}
          httpStatus={keyValidation.httpStatus}
          detail={keyValidation.detail}
          onCorrect={handleValidationCorrect}
          onForceSave={handleValidationForceSave}
          onRetry={handleValidationRetry}
          retrying={retryingValidation}
          forcing={forcingValidationSave}
        />
      )}

      {defaultImageToast && (
        <div className={`wifi-toast wifi-toast-${defaultImageToast.type}`}>
          {defaultImageToast.message}
        </div>
      )}

      {wifiToast && (
        <div className={`wifi-toast wifi-toast-${wifiToast.type}`}>
          {wifiToast.message}
        </div>
      )}

      {aiToast && (
        <div className={`wifi-toast wifi-toast-${aiToast.type}`}>
          {aiToast.message}
        </div>
      )}

      {firmwareToast && (
        <div className={`wifi-toast wifi-toast-${firmwareToast.type}`}>
          {firmwareToast.message}
        </div>
      )}

      {showOtaAllModal && (
        <OtaAllModal
          bumpers={bumpers}
          onClose={() => setShowOtaAllModal(false)}
        />
      )}
    </div>
  )
}
