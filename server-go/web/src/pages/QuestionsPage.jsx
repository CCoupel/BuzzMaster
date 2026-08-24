import { useState, useRef, useMemo, useEffect } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { useGame } from '../hooks/GameContext'
import { useCategoryFilter } from '../hooks/useCategoryFilter'
import { useCategories } from '../hooks/useCategories'
import { CATEGORIES, categoryMeta } from '../utils/categoryUtils'
import { sortQuestionsByOrder, shuffleArray } from '../utils/questionOrder'
import { QUESTION_TYPES } from '../utils/questionTypeMeta'
import { isMotionCardTypeLocked, motionCardLockReason } from '../utils/motionCardLock'
import Button from '../components/Button'
import Card, { CardHeader, CardBody } from '../components/Card'
import CategoryBalance from '../components/CategoryBalance'
import CategoryBadge from '../components/CategoryBadge'
import QuestionCard from '../components/QuestionCard'
import AIGenerateModal from '../components/AIGenerateModal'
import QcmAnswersEditor from '../components/QcmAnswersEditor'
import './QuestionsPage.css'
import './ConfigPage.css'
import '../styles/sliders.css'

// Énumérations partagées avec la modale IA et le contrat backend
// (contracts/ai-generation.md §6) — source de vérité pour les selects
// Population/Langue et le multi-select Difficulté, ici comme dans la modale.
export const QUIZ_POPULATIONS = ['Junior (6-12 ans)', 'Ado (13-17 ans)', 'Adulte (18-64 ans)', 'Senior (65+ ans)', 'Famille']
export const QUIZ_DIFFICULTIES = ['Facile', 'Moyen', 'Difficile', 'Expert']
export const QUIZ_LANGUAGES = ['Français', 'Anglais', 'Espagnol']

// Re-export CATEGORIES for backward compatibility
export { CATEGORIES }

export default function QuestionsPage() {
  const { questions, fsInfo, deleteQuestion, sendMessage, gameState, newGame, aiJob, cancelAiGeneration } = useGame()
  const [isUploading, setIsUploading] = useState(false)
  const [editingId, setEditingId] = useState(null)
  const fileInputRef = useRef(null)
  const fileAnswerInputRef = useRef(null)
  const [draggedId, setDraggedId] = useState(null)
  const [dragOverId, setDragOverId] = useState(null)

  // NEW_GAME backgrounds management state (multi-images, mirrors Zone Ambiance)
  const [uploadingNgBg, setUploadingNgBg] = useState(false)
  const ngBgInputRef = useRef(null)
  const [draggedNgBgIndex, setDraggedNgBgIndex] = useState(null)

  // Background management state
  const [uploadingBg, setUploadingBg] = useState(false)
  const bgInputRef = useRef(null)
  const [draggedBgIndex, setDraggedBgIndex] = useState(null)

  // Add category inline form state (#97 + #100)
  const [showAddCategory, setShowAddCategory] = useState(false)
  const [newCategoryName, setNewCategoryName] = useState('')
  const [newCategoryFile, setNewCategoryFile] = useState(null)
  const [addCategoryError, setAddCategoryError] = useState('')

  // Form state
  const [formData, setFormData] = useState({
    question: '',
    answer: '',
    type: 'SPEEDY',
    category: '', // Question category
    pointsTarget: 'PLAYER', // PLAYER or TEAM
    qcmAnswers: { RED: '', GREEN: '', YELLOW: '', BLUE: '' },
    qcmCorrect: '',
    qcmHintsEnabled: false, // Enable automatic hint invalidation for QCM
    qcmHintThreshold1: 0.25, // First hint at 25% of time remaining
    qcmHintThreshold2: 0.125, // Second hint at 12.5% of time remaining
    qcmPenalty1: 0.67, // Points multiplier after 1 hint (67%)
    qcmPenalty2: 0.33, // Points multiplier after 2 hints (33%)
    // Memory game fields
    memoryMode: 'SOLO', // SOLO, CHACUN_SON_TOUR, TANT_QUE_JE_GAGNE
    memoryPairs: [
      { id: 1, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } },
      { id: 2, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } },
    ],
    // ARDOISE fields (v5.6.0)
    ardoiseKeyboardType: 'AZERTY',
    // MEMOTION fields (v5.0.0)
    motionMode: 'SOLO',
    // #184/B-F4 — chaque carte porte désormais `type` + les valeurs de
    // création de ses OwnedFields QCM (7 occurrences de ce littéral dans le
    // fichier : useState initial, resetForm, chargement d'une question
    // existante, handleAddMotionCard — voir grep `rectoTheme: ''` pour les
    // retrouver toutes). Ces valeurs DOIVENT rester synchronisées avec
    // `utils/motionCardLock.js` (`QCM_CREATION_VALUES`) — un écart ferait
    // apparaître une carte neuve comme déjà verrouillée, exactement le piège
    // documenté par `contracts/question-types.md` §3.2 (QCM_HINT_THRESHOLD_1
    // etc. naissent non vides, PAS `undefined`/`0`).
    motionCards: [
      { id: 'mc-1', rectoTheme: '', rectoImage: null, difficulty: 1, questionText: '', questionImage: null, answerText: '', answerImage: null,
        type: 'SPEEDY',
        qcmAnswers: { RED: '', GREEN: '', YELLOW: '', BLUE: '' },
        qcmCorrect: '',
        qcmHintsEnabled: false,
        qcmHintThreshold1: 0.25,
        qcmHintThreshold2: 0.125,
        qcmPenalty1: 0.67,
        qcmPenalty2: 0.33,
      },
      { id: 'mc-2', rectoTheme: '', rectoImage: null, difficulty: 1, questionText: '', questionImage: null, answerText: '', answerImage: null,
        type: 'SPEEDY',
        qcmAnswers: { RED: '', GREEN: '', YELLOW: '', BLUE: '' },
        qcmCorrect: '',
        qcmHintsEnabled: false,
        qcmHintThreshold1: 0.25,
        qcmHintThreshold2: 0.125,
        qcmPenalty1: 0.67,
        qcmPenalty2: 0.33,
      },
    ],
    motionConfig: { points1: 1, points2: 3, points3: 5 },
    motionMemorizeDuration: 0,
    memoryConfig: {
      flipDelay: 3,
      pointsPerPair: 10,
      errorPenalty: 0,
      completionBonus: 0,
      useTimer: true,
      memorizeTime: 5,
      showDuringMemorize: true,
      revealDelay: 0.5,
    },
    points: '1',
    time: '30',
    media: null,
    existingMedia: null,
    mediaAnswer: null,
    existingMediaAnswer: null,
    // Note d'explication — animateur seul (v6.4.x, #168)
    explanation: '',
  })

  // Quiz metadata form state
  const [quizName, setQuizName] = useState(gameState.quizName || '')
  const [quizTheme, setQuizTheme] = useState(gameState.quizTheme || '')
  const [quizNotes, setQuizNotes] = useState(gameState.quizNotes || '')
  // v6.1.0 (#137) — publics/difficultés multiples (remplacent les valeurs
  // uniques v6.0.0), objectif de partie, visibilité TV par champ. Contract
  // game-state.md §"Métadonnées Quiz".
  const [quizPopulations, setQuizPopulations] = useState(gameState.quizPopulations || [])
  const [quizDifficulties, setQuizDifficulties] = useState(gameState.quizDifficulties || [])
  const [quizLanguage, setQuizLanguage] = useState(gameState.quizLanguage || 'Français')
  const [quizObjectives, setQuizObjectives] = useState(gameState.quizObjectives || '')
  const [quizHiddenFields, setQuizHiddenFields] = useState(gameState.quizHiddenFields || [])
  const [quizSaved, setQuizSaved] = useState(false)

  // ENTRACTE (#119, corrections C1/C4) — configuration du panneau de pause
  // globale, propriété de la partie (comme les métadonnées Quiz ci-dessus),
  // éditée depuis cette page via l'action dédiée UPDATE_ENTRACTE_CONFIG.
  // Alimenté depuis gameState.entracteConfigSaved (la config ENREGISTRÉE,
  // toujours à jour) — JAMAIS gameState.entracteConfig (la config DIFFUSÉE
  // au panneau, gelée pendant une pause active) : sinon un enregistrement
  // fait pendant l'entracte semblerait perdu au retour sur cette page (C4).
  const savedEntracteCfg = gameState.entracteConfigSaved || {}
  const [entracteTitle, setEntracteTitle] = useState(savedEntracteCfg.TITLE || 'ENTRACTE')
  const [entracteSubtitle, setEntracteSubtitle] = useState(savedEntracteCfg.SUBTITLE || 'Retour dans 20mn')
  const [entracteImageIsCustom, setEntracteImageIsCustom] = useState(savedEntracteCfg.IMAGE_IS_CUSTOM || false)
  const [entractePanelSize, setEntractePanelSize] = useState(savedEntracteCfg.PANEL_SIZE ?? 65)
  const [entracteAnimPeriod, setEntracteAnimPeriod] = useState(savedEntracteCfg.ANIM_PERIOD ?? 10)
  const [entracteAnimIntensity, setEntracteAnimIntensity] = useState(savedEntracteCfg.ANIM_INTENSITY ?? 20)
  const [entracteTransitionMs, setEntracteTransitionMs] = useState(savedEntracteCfg.TRANSITION_MS ?? 2000)
  const [entracteSaved, setEntracteSaved] = useState(false)
  const [entracteImageCacheBuster, setEntracteImageCacheBuster] = useState(() => Date.now())
  const [uploadingEntracteImage, setUploadingEntracteImage] = useState(false)
  const [deletingEntracteImage, setDeletingEntracteImage] = useState(false)
  const [entracteImageToast, setEntracteImageToast] = useState(null)
  const entracteImageFileRef = useRef(null)

  // AI generation modal (v6.0.0 #8, multi-provider + tâche de fond v6.1.0 #137)
  const [showAIModal, setShowAIModal] = useState(false)
  const [aiConfig, setAiConfig] = useState({
    provider: 'anthropic',
    apiKeyConfigured: false,
    groqApiKeyConfigured: false,
    interBatchDelayMs: 60000,
    maxConsecutiveFailures: 2,
  })
  const [aiToast, setAiToast] = useState(null)
  // #149 — toast de confirmation après "Mélanger les questions" (même motif
  // que aiToast/wifiToast : auto-masqué, cf. effet plus bas).
  const [shuffleToast, setShuffleToast] = useState(null)
  // #137 — jobId déjà signalé par toast, pour ne pas re-toaster à chaque
  // re-render tant que le job reste dans le même état terminal, et pour ne
  // pas toaster un job dont la fin a été vue en direct dans la modale.
  const toastedAiJobIdRef = useRef(null)
  const quizMetaSectionRef = useRef(null)

  // #137 — le bouton "✨ Générer via IA" s'active selon la clé du provider
  // ACTUELLEMENT sélectionné (maquette 137 §7), pas "si n'importe lequel en
  // a une". Reste cliquable si un job tourne déjà, pour permettre le
  // ré-attachement/l'annulation même si la clé a depuis été retirée.
  const providerConfigured = aiConfig.provider === 'groq' ? aiConfig.groqApiKeyConfigured : aiConfig.apiKeyConfigured
  const canOpenAiModal = providerConfigured || aiJob?.state === 'RUNNING'

  // Sync quiz form with gameState (populated from WS after mount)
  useEffect(() => {
    setQuizName(gameState.quizName || '')
    setQuizTheme(gameState.quizTheme || '')
    setQuizNotes(gameState.quizNotes || '')
    setQuizPopulations(gameState.quizPopulations || [])
    setQuizDifficulties(gameState.quizDifficulties || [])
    setQuizLanguage(gameState.quizLanguage || 'Français')
    setQuizObjectives(gameState.quizObjectives || '')
    setQuizHiddenFields(gameState.quizHiddenFields || [])
  }, [gameState.quizName, gameState.quizTheme, gameState.quizNotes, gameState.quizPopulations, gameState.quizDifficulties, gameState.quizLanguage, gameState.quizObjectives, gameState.quizHiddenFields])

  // ENTRACTE (#119, C1/C4) — sync depuis gameState.entracteConfigSaved
  // (jamais entracteConfig, voir commentaire de déclaration d'état ci-dessus).
  useEffect(() => {
    const cfg = gameState.entracteConfigSaved
    if (!cfg) return
    setEntracteTitle(cfg.TITLE ?? 'ENTRACTE')
    setEntracteSubtitle(cfg.SUBTITLE ?? 'Retour dans 20mn')
    setEntracteImageIsCustom(cfg.IMAGE_IS_CUSTOM ?? false)
    setEntractePanelSize(cfg.PANEL_SIZE ?? 65)
    setEntracteAnimPeriod(cfg.ANIM_PERIOD ?? 10)
    setEntracteAnimIntensity(cfg.ANIM_INTENSITY ?? 20)
    setEntracteTransitionMs(cfg.TRANSITION_MS ?? 2000)
  }, [gameState.entracteConfigSaved])

  // ENTRACTE image toast auto-hide (même patron que les autres toasts de la page)
  useEffect(() => {
    if (entracteImageToast) {
      const timer = setTimeout(() => setEntracteImageToast(null), 3000)
      return () => clearTimeout(timer)
    }
  }, [entracteImageToast])

  // v6.1.0 (#137, T2.5) — comparaison indépendante de l'ordre pour les deux
  // tableaux : "publics/difficultés dans un ordre différent" ne doit pas être
  // traité comme une divergence.
  const arraysEqualUnordered = (a, b) => {
    if (a.length !== b.length) return false
    const sa = [...a].sort()
    const sb = [...b].sort()
    return sa.every((v, i) => v === sb[i])
  }

  // T2.5 — écart entre le formulaire de la section Quiz et le GameState
  // réellement diffusé, restreint aux 5 champs qui alimentent la génération
  // (thème/publics/difficultés/langue/objectif) — QUIZ_NAME et QUIZ_NOTES
  // partagent le même bouton Enregistrer mais n'affectent pas la génération,
  // les inclure ferait apparaître le bandeau de la modale IA à tort.
  const quizFormDiverged =
    quizTheme !== (gameState.quizTheme || '') ||
    quizLanguage !== (gameState.quizLanguage || 'Français') ||
    quizObjectives !== (gameState.quizObjectives || '') ||
    !arraysEqualUnordered(quizPopulations, gameState.quizPopulations || []) ||
    !arraysEqualUnordered(quizDifficulties, gameState.quizDifficulties || [])

  // AI: état de la clé API + provider sélectionné — source de vérité pour
  // activer/désactiver le bouton "✨ Générer via IA" (contract
  // ai-generation.md §2, ai-multi-provider.md §8, maquette 137 §7). La clé
  // elle-même n'est jamais transmise au frontend. async/await + try/catch
  // (motif ConfigPage.jsx) plutôt qu'une chaîne .then() : protège aussi
  // contre un environnement où fetch() ne retournerait pas une Promise
  // valide (ex. mock de test partiel), pas seulement contre un rejet réseau.
  useEffect(() => {
    let cancelled = false
    const fetchAiStatus = async () => {
      try {
        const res = await fetch('/config.json')
        if (!res.ok) return
        const data = await res.json()
        if (!cancelled && data?.ai) {
          setAiConfig({
            provider: data.ai.provider || 'anthropic',
            apiKeyConfigured: !!data.ai.api_key_configured,
            groqApiKeyConfigured: !!data.ai.groq_api_key_configured,
            interBatchDelayMs: data.ai.inter_batch_delay_ms || 60000,
            maxConsecutiveFailures: data.ai.max_consecutive_failures || 2,
          })
        }
      } catch {
        // Génération indisponible — le bouton reste désactivé, le modal
        // (si ouvert malgré tout) retombera sur l'état "Indisponible".
      }
    }
    fetchAiStatus()
    return () => { cancelled = true }
  }, [])

  // #137 — toast de fin de job (motif wifiToast) quand la modale a été
  // fermée pendant que la génération continuait en fond (maquette §4).
  // `toastedAiJobIdRef` marque le job comme "vu" dès qu'il atteint un état
  // terminal, que ce soit via ce toast ou parce que la modale l'affichait
  // déjà à l'écran — évite un doublon si l'admin ferme juste après.
  useEffect(() => {
    if (aiToast) {
      const timer = setTimeout(() => setAiToast(null), 4000)
      return () => clearTimeout(timer)
    }
  }, [aiToast])

  // #149 — shuffleToast auto-hide
  useEffect(() => {
    if (shuffleToast) {
      const timer = setTimeout(() => setShuffleToast(null), 3000)
      return () => clearTimeout(timer)
    }
  }, [shuffleToast])

  useEffect(() => {
    if (!aiJob) return
    const terminal = aiJob.state === 'DONE' || aiJob.state === 'CANCELLED' || aiJob.state === 'FAILED'
    if (!terminal || toastedAiJobIdRef.current === aiJob.jobId) return
    toastedAiJobIdRef.current = aiJob.jobId
    if (showAIModal) return // déjà visible dans la modale elle-même
    const n = aiJob.createdCount || 0
    const plural = n > 1 ? 's' : ''
    if (aiJob.state === 'DONE') {
      setAiToast({ message: `✅ Génération terminée — ${n} question${plural} créée${plural}`, type: 'success' })
    } else if (aiJob.state === 'CANCELLED') {
      setAiToast({ message: `⏹ Génération arrêtée — ${n} question${plural} conservée${plural}`, type: 'success' })
    } else {
      setAiToast({ message: `⚠️ Génération interrompue — ${n} question${plural} conservée${plural}`, type: 'error' })
    }
  }, [aiJob, showAIModal])

  const handleSaveQuizMeta = (e) => {
    e.preventDefault()
    sendMessage('UPDATE_QUIZ_META', {
      NAME: quizName,
      THEME: quizTheme,
      NOTES: quizNotes,
      POPULATIONS: quizPopulations,
      DIFFICULTIES: quizDifficulties,
      LANGUAGE: quizLanguage,
      OBJECTIVES: quizObjectives,
      HIDDEN_FIELDS: quizHiddenFields,
    })
    setQuizSaved(true)
    setTimeout(() => setQuizSaved(false), 2000)
  }

  // ENTRACTE (#119, C1) — action dédiée, distincte d'UPDATE_QUIZ_META
  // (plan : deux formulaires séparés, chacun propriétaire de ses champs,
  // pour ne pas risquer qu'un enregistrement du bloc Quiz efface les
  // réglages d'entracte ou réciproquement). Acceptée par le serveur même
  // pendant un entracte actif (C4) — écrit ENTRACTE_CONFIG_SAVED sans
  // rafraîchir le panneau déjà diffusé.
  const handleSaveEntracteConfig = (e) => {
    e.preventDefault()
    sendMessage('UPDATE_ENTRACTE_CONFIG', {
      TITLE: entracteTitle,
      SUBTITLE: entracteSubtitle,
      PANEL_SIZE: entractePanelSize,
      ANIM_PERIOD: entracteAnimPeriod,
      ANIM_INTENSITY: entracteAnimIntensity,
      TRANSITION_MS: entracteTransitionMs,
    })
    setEntracteSaved(true)
    setTimeout(() => setEntracteSaved(false), 2000)
  }

  // Image de fond du panneau — copie du patron "Image par défaut"
  // (anciennement dans ConfigPage.jsx, déplacé ici avec la configuration,
  // C1). Endpoint renommé /api/game/entracte-image (C1-B6, l'image
  // appartient désormais à la partie, pas à la config serveur).
  const handleEntracteImageUpload = async () => {
    const file = entracteImageFileRef.current?.files?.[0]
    if (!file) {
      setEntracteImageToast({ message: 'Veuillez selectionner une image', type: 'error' })
      return
    }
    const allowed = ['.jpg', '.jpeg', '.png', '.gif', '.webp', '.svg']
    const ext = '.' + file.name.split('.').pop().toLowerCase()
    if (!allowed.includes(ext)) {
      setEntracteImageToast({ message: 'Format non supporte. Utilisez jpg, png, gif, webp ou svg', type: 'error' })
      return
    }
    setUploadingEntracteImage(true)
    try {
      const formData = new FormData()
      formData.append('file', file)
      const res = await fetch('/api/game/entracte-image', { method: 'POST', body: formData })
      const data = await res.json()
      if (res.ok && (data.is_custom || data.image_is_custom)) {
        setEntracteImageIsCustom(true)
        setEntracteImageCacheBuster(Date.now())
        setEntracteImageToast({ message: 'Image d\'entracte enregistree', type: 'success' })
        if (entracteImageFileRef.current) entracteImageFileRef.current.value = ''
      } else {
        setEntracteImageToast({ message: 'Erreur lors de l\'upload', type: 'error' })
      }
    } catch (err) {
      setEntracteImageToast({ message: 'Erreur reseau: ' + err.message, type: 'error' })
    } finally {
      setUploadingEntracteImage(false)
    }
  }

  const handleEntracteImageDelete = async () => {
    if (!window.confirm('Retirer l\'image d\'entracte ? Le panneau restera lisible sans image.')) return
    setDeletingEntracteImage(true)
    try {
      const res = await fetch('/api/game/entracte-image', { method: 'DELETE' })
      if (res.ok) {
        setEntracteImageIsCustom(false)
        setEntracteImageCacheBuster(Date.now())
        setEntracteImageToast({ message: 'Image d\'entracte supprimee', type: 'success' })
      } else {
        setEntracteImageToast({ message: 'Erreur lors de la suppression', type: 'error' })
      }
    } catch (err) {
      setEntracteImageToast({ message: 'Erreur reseau: ' + err.message, type: 'error' })
    } finally {
      setDeletingEntracteImage(false)
    }
  }

  // Chips multi-sélection (motif AIGenerateModal.jsx — catégories)
  const toggleQuizPopulation = (p) => {
    setQuizPopulations(prev => prev.includes(p) ? prev.filter(x => x !== p) : [...prev, p])
  }
  const toggleQuizDifficulty = (d) => {
    setQuizDifficulties(prev => prev.includes(d) ? prev.filter(x => x !== d) : [...prev, d])
  }

  // Interrupteur "Afficher sur la TV" — case cochée = champ ABSENT de
  // QUIZ_HIDDEN_FIELDS. L'inversion (liste des champs masqués ↔ case "visible")
  // est absorbée ici uniquement (plan §5, T2.2).
  const isQuizFieldVisibleOnTV = (field) => !quizHiddenFields.includes(field)
  const toggleQuizFieldVisibility = (field) => {
    setQuizHiddenFields(prev => prev.includes(field) ? prev.filter(f => f !== field) : [...prev, field])
  }

  // AI: après une génération (terminée/arrêtée/en échec partiel) et la
  // fermeture de la modale, défiler jusqu'à la première question créée
  // (maquette §4). AIGenerateModal calcule déjà cet ID (delta sur
  // `questions`, cf. son commentaire sur startingQuestionIdsRef) et le passe
  // directement — un court délai supplémentaire laisse le DOM se stabiliser.
  const handleAIGenerated = (firstId) => {
    if (!firstId) return
    setTimeout(() => {
      document.getElementById(`qcard-${firstId}`)?.scrollIntoView({ behavior: 'smooth', block: 'center' })
    }, 300)
  }

  const handleNavigateToQuizSettings = () => {
    setShowAIModal(false)
    quizMetaSectionRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  // NEW_GAME backgrounds handlers (mirror Zone Ambiance, endpoint: /new-game-backgrounds)
  const handleNgBackgroundUpload = async (e) => {
    const file = e.target.files?.[0]
    if (!file) return
    setUploadingNgBg(true)
    const fd = new FormData()
    fd.append('file', file)
    try {
      const response = await fetch('/new-game-backgrounds', { method: 'POST', body: fd })
      if (response.ok) {
        window.location.reload()
      } else {
        const text = await response.text()
        alert('Erreur: ' + text)
      }
    } catch (error) {
      console.error('NG background upload failed:', error)
      alert('Erreur: ' + error.message)
    } finally {
      setUploadingNgBg(false)
      if (ngBgInputRef.current) ngBgInputRef.current.value = ''
    }
  }

  const handleRemoveNgBackground = async (bgPath) => {
    if (!window.confirm('Supprimer cette image de fond Nouvelle Partie ?')) return
    try {
      const filename = bgPath.split('/').pop()
      await fetch(`/new-game-backgrounds?file=${encodeURIComponent(filename)}`, { method: 'DELETE' })
    } catch (error) {
      console.error('Remove NG background failed:', error)
    }
  }

  const handleRemoveAllNgBackgrounds = async () => {
    if (!window.confirm('Supprimer toutes les images de fond Nouvelle Partie ?')) return
    try {
      await fetch('/new-game-backgrounds', { method: 'DELETE' })
    } catch (error) {
      console.error('Remove all NG backgrounds failed:', error)
    }
  }

  const handleNgDurationChange = async (index, newDuration) => {
    const backgrounds = [...(gameState?.newGameBackgrounds || [])]
    backgrounds[index] = { ...backgrounds[index], duration: parseInt(newDuration) || 10 }
    await saveNgBackgrounds(backgrounds)
  }

  const handleNgOpacityChange = async (index, newOpacity) => {
    const backgrounds = [...(gameState?.newGameBackgrounds || [])]
    backgrounds[index] = { ...backgrounds[index], opacity: Math.max(0, Math.min(100, parseInt(newOpacity) || 100)) }
    await saveNgBackgrounds(backgrounds)
  }

  const handleNgMoveBackground = async (fromIndex, toIndex) => {
    if (fromIndex === toIndex) return
    const backgrounds = [...(gameState?.newGameBackgrounds || [])]
    const [moved] = backgrounds.splice(fromIndex, 1)
    backgrounds.splice(toIndex, 0, moved)
    await saveNgBackgrounds(backgrounds)
  }

  const saveNgBackgrounds = async (backgrounds) => {
    try {
      await fetch('/new-game-backgrounds', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(backgrounds)
      })
    } catch (error) {
      console.error('Save NG backgrounds failed:', error)
    }
  }

  // #149 — mise à jour optimiste de l'ordre après "Mélanger les questions" :
  // REORDER_QUESTIONS (comme handleDrop plus bas) est fire-and-forget côté
  // WebSocket, sans accusé de réception. `shuffleOverrideOrder` affiche donc
  // le nouvel ordre immédiatement, avant même la diffusion serveur, et se
  // referme tout seul dès que `questions` reflète réellement ce même ordre
  // (ou après un délai de sécurité si la diffusion tarde/normalise l'ORDER
  // différemment — cf. handleShuffleQuestions et l'effet de reconciliation
  // plus bas).
  const [shuffleOverrideOrder, setShuffleOverrideOrder] = useState(null) // string[] | null

  const sortedQuestions = useMemo(() => {
    const naturalOrder = sortQuestionsByOrder(questions)
    if (!shuffleOverrideOrder) return naturalOrder
    const byId = new Map(naturalOrder.map(q => [q.ID, q]))
    const ordered = shuffleOverrideOrder.map(id => byId.get(id)).filter(Boolean)
    // Sécurité : toute question absente du snapshot mélangé (créée entre
    // temps par exemple) reste visible, ajoutée à la suite.
    const missing = naturalOrder.filter(q => !shuffleOverrideOrder.includes(q.ID))
    return [...ordered, ...missing]
  }, [questions, shuffleOverrideOrder])

  // Referme l'override optimiste dès que l'ordre réel (issu de la diffusion
  // REORDER_QUESTIONS) coïncide avec celui qu'on affichait déjà — ou après un
  // délai de sécurité, pour ne jamais rester bloqué sur un ordre obsolète si
  // la diffusion n'arrive jamais exactement à l'identique (le backend
  // renormalise ORDER en 0-based, mais la SÉQUENCE d'ID reste la même
  // permutation, donc la comparaison ci-dessous reste fiable).
  useEffect(() => {
    if (!shuffleOverrideOrder) return
    const naturalIds = sortQuestionsByOrder(questions).map(q => q.ID)
    if (JSON.stringify(naturalIds) === JSON.stringify(shuffleOverrideOrder)) {
      setShuffleOverrideOrder(null)
      return
    }
    const timer = setTimeout(() => setShuffleOverrideOrder(null), 5000)
    return () => clearTimeout(timer)
  }, [questions, shuffleOverrideOrder])

  // Custom categories from API (#95)
  const { categories: apiCategories, refetch: refetchCategories } = useCategories()
  const customCategories = useMemo(() => apiCategories.filter(c => c.isCustom), [apiCategories])

  // Category filter (shared hook) — passes custom categories for filter support
  const { selectedCategories, availableCategories, filteredQuestions, toggleCategoryFilter, clearCategoryFilters } = useCategoryFilter(sortedQuestions, customCategories)

  // Background handlers
  const handleBackgroundUpload = async (e) => {
    const file = e.target.files?.[0]
    if (!file) return

    setUploadingBg(true)
    const formData = new FormData()
    formData.append('file', file)

    try {
      const response = await fetch('/background', { method: 'POST', body: formData })
      if (response.ok) {
        window.location.reload()
      } else {
        const text = await response.text()
        alert('Erreur: ' + text)
      }
    } catch (error) {
      console.error('Background upload failed:', error)
      alert('Erreur: ' + error.message)
    } finally {
      setUploadingBg(false)
      if (bgInputRef.current) bgInputRef.current.value = ''
    }
  }

  const handleRemoveBackground = async (bgPath) => {
    if (!window.confirm('Supprimer cette image de fond ?')) return
    try {
      const filename = bgPath.split('/').pop()
      await fetch(`/background?file=${encodeURIComponent(filename)}`, { method: 'DELETE' })
    } catch (error) {
      console.error('Remove background failed:', error)
    }
  }

  const handleRemoveAllBackgrounds = async () => {
    if (!window.confirm('Supprimer toutes les images de fond ?')) return
    try {
      await fetch('/background', { method: 'DELETE' })
    } catch (error) {
      console.error('Remove all backgrounds failed:', error)
    }
  }

  const handleDurationChange = async (index, newDuration) => {
    const backgrounds = [...(gameState?.backgrounds || [])]
    backgrounds[index] = { ...backgrounds[index], duration: parseInt(newDuration) || 10 }
    await saveBackgrounds(backgrounds)
  }

  const handleOpacityChange = async (index, newOpacity) => {
    const backgrounds = [...(gameState?.backgrounds || [])]
    backgrounds[index] = { ...backgrounds[index], opacity: Math.max(0, Math.min(100, parseInt(newOpacity) || 100)) }
    await saveBackgrounds(backgrounds)
  }

  const handleMoveBackground = async (fromIndex, toIndex) => {
    if (fromIndex === toIndex) return
    const backgrounds = [...(gameState?.backgrounds || [])]
    const [moved] = backgrounds.splice(fromIndex, 1)
    backgrounds.splice(toIndex, 0, moved)
    await saveBackgrounds(backgrounds)
  }

  const saveBackgrounds = async (backgrounds) => {
    try {
      await fetch('/background', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(backgrounds)
      })
    } catch (error) {
      console.error('Save backgrounds failed:', error)
    }
  }

  // Drag and drop handlers
  const handleDragStart = (e, questionId) => {
    setDraggedId(questionId)
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', questionId)
  }

  const handleDragOver = (e, questionId) => {
    e.preventDefault()
    e.dataTransfer.dropEffect = 'move'
    if (questionId !== draggedId) {
      setDragOverId(questionId)
    }
  }

  const handleDragLeave = () => {
    setDragOverId(null)
  }

  const handleDragEnd = () => {
    setDraggedId(null)
    setDragOverId(null)
  }

  const handleDrop = async (e, targetId) => {
    e.preventDefault()
    const sourceId = draggedId

    if (!sourceId || sourceId === targetId) {
      setDraggedId(null)
      setDragOverId(null)
      return
    }

    // Calculate new order
    const currentOrder = sortedQuestions.map(q => q.ID)
    const sourceIndex = currentOrder.indexOf(sourceId)
    const targetIndex = currentOrder.indexOf(targetId)

    if (sourceIndex === -1 || targetIndex === -1) return

    // Remove source and insert at target position
    currentOrder.splice(sourceIndex, 1)
    currentOrder.splice(targetIndex, 0, sourceId)

    // Send new order to server
    sendMessage('REORDER_QUESTIONS', { ORDER: currentOrder })

    setDraggedId(null)
    setDragOverId(null)
  }

  // #149 — "Mélanger les questions" : réordonne aléatoirement TOUTES les
  // questions (sortedQuestions, jamais filteredQuestions — le mélange porte
  // sur l'ensemble même quand un filtre de catégorie est actif, cf. maquette
  // validée §5), via le même message REORDER_QUESTIONS que le glisser-déposer
  // (handleDrop ci-dessus). Action irréversible -> confirmation, renforcée si
  // une partie est en cours (décision utilisateur, plan Batch 5 point 3).
  const handleShuffleQuestions = () => {
    if (sortedQuestions.length < 2) return

    const count = sortedQuestions.length
    const gameInProgress = !!gameState?.phase && gameState.phase !== 'STOPPED'
    const filterActive = selectedCategories.size > 0

    const messageParts = [
      gameInProgress
        ? `⚠️ Une partie est en cours : mélanger les ${count} questions va changer l'ordre de jeu immédiatement, pour tout le monde.`
        : `Mélanger les ${count} questions ?`,
      `L'ordre actuel sera remplacé par un ordre aléatoire, pour tout le monde. ` +
        `Vous pourrez le réajuster ensuite au glisser-déposer, mais l'ordre actuel ne pourra pas être restauré.`,
    ]
    if (filterActive) {
      messageParts.push('Le mélange porte sur toutes les questions, pas seulement celles affichées par le filtre actif.')
    }
    if (!window.confirm(messageParts.join('\n\n'))) return

    const previousOrder = sortedQuestions.map(q => q.ID)
    const newOrder = shuffleArray(previousOrder)

    // Mise à jour optimiste — cf. commentaire sur shuffleOverrideOrder plus
    // haut. sendMessage (hooks/useWebSocket.js) renvoie désormais un booléen
    // (true si effectivement envoyé sur le socket ouvert) pour permettre ce
    // retour à l'état antérieur en cas d'échec réseau, ce qu'aucun appel
    // WebSocket fire-and-forget de cette page ne faisait jusqu'ici.
    setShuffleOverrideOrder(newOrder)
    const sent = sendMessage('REORDER_QUESTIONS', { ORDER: newOrder })
    if (!sent) {
      setShuffleOverrideOrder(null)
      setShuffleToast({ message: "Erreur : connexion perdue, le mélange n'a pas été envoyé.", type: 'error' })
      return
    }
    setShuffleToast({ message: `✓ Ordre mélangé — ${count} question${count > 1 ? 's' : ''}`, type: 'success' })
  }

  const handleInputChange = (field, value) => {
    setFormData(prev => {
      const updates = { [field]: value }
      // Auto-set pointsTarget when type changes
      if (field === 'type') {
        // QCM, MEMORY, MEMOTION and ARDOISE default to TEAM, SPEEDY defaults to PLAYER
        updates.pointsTarget = (value === 'QCM' || value === 'MEMORY' || value === 'MEMOTION' || value === 'ARDOISE') ? 'TEAM' : 'PLAYER'
      }
      return { ...prev, ...updates }
    })
  }

  const handleFileChange = (e) => {
    const file = e.target.files?.[0]
    if (file) {
      setFormData(prev => ({ ...prev, media: file, existingMedia: null }))
    }
  }

  const handleFileAnswerChange = (e) => {
    const file = e.target.files?.[0]
    if (file) {
      setFormData(prev => ({ ...prev, mediaAnswer: file, existingMediaAnswer: null }))
    }
  }

  const handleQuestionClick = (question) => {
    setEditingId(question.ID)
    const qType = question.TYPE || 'SPEEDY'
    // Default pointsTarget based on type if not set
    const defaultTarget = (qType === 'QCM' || qType === 'MEMORY' || qType === 'ARDOISE') ? 'TEAM' : 'PLAYER'

    // Load memory pairs from question data
    let memoryPairs = [
      { id: 1, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } },
      { id: 2, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } },
    ]
    if (question.MEMORY_PAIRS && Array.isArray(question.MEMORY_PAIRS)) {
      memoryPairs = question.MEMORY_PAIRS.map(pair => ({
        id: pair.ID,
        card1: {
          text: pair.CARD1?.TEXT || '',
          image: pair.CARD1?.IMAGE || null,
          isImage: pair.CARD1?.IS_IMAGE || false,
        },
        card2: {
          text: pair.CARD2?.TEXT || '',
          image: pair.CARD2?.IMAGE || null,
          isImage: pair.CARD2?.IS_IMAGE || false,
        },
      }))
    }

    // Load memory config from question data
    let memoryConfig = {
      flipDelay: 3,
      pointsPerPair: 10,
      errorPenalty: 0,
      completionBonus: 0,
      useTimer: true,
      memorizeTime: 5,
      showDuringMemorize: true,
      revealDelay: 0.5,
    }
    if (question.MEMORY_CONFIG) {
      memoryConfig = {
        flipDelay: question.MEMORY_CONFIG.FLIP_DELAY || 3,
        pointsPerPair: question.MEMORY_CONFIG.POINTS_PER_PAIR || 10,
        errorPenalty: question.MEMORY_CONFIG.ERROR_PENALTY || 0,
        completionBonus: question.MEMORY_CONFIG.COMPLETION_BONUS || 0,
        useTimer: question.MEMORY_CONFIG.USE_TIMER !== false,
        memorizeTime: question.MEMORY_CONFIG.MEMORIZE_TIME || 5,
        showDuringMemorize: question.MEMORY_CONFIG.SHOW_DURING_MEMORIZE !== false,
        revealDelay: question.MEMORY_CONFIG.REVEAL_DELAY || 0.5,
      }
    }

    // Load MEMOTION cards from question data
    let motionCards = [
      { id: 'mc-1', rectoTheme: '', rectoImage: null, difficulty: 1, questionText: '', questionImage: null, answerText: '', answerImage: null,
        type: 'SPEEDY',
        qcmAnswers: { RED: '', GREEN: '', YELLOW: '', BLUE: '' },
        qcmCorrect: '',
        qcmHintsEnabled: false,
        qcmHintThreshold1: 0.25,
        qcmHintThreshold2: 0.125,
        qcmPenalty1: 0.67,
        qcmPenalty2: 0.33,
      },
      { id: 'mc-2', rectoTheme: '', rectoImage: null, difficulty: 1, questionText: '', questionImage: null, answerText: '', answerImage: null,
        type: 'SPEEDY',
        qcmAnswers: { RED: '', GREEN: '', YELLOW: '', BLUE: '' },
        qcmCorrect: '',
        qcmHintsEnabled: false,
        qcmHintThreshold1: 0.25,
        qcmHintThreshold2: 0.125,
        qcmPenalty1: 0.67,
        qcmPenalty2: 0.33,
      },
    ]
    if (question.MOTION_CARDS && Array.isArray(question.MOTION_CARDS)) {
      motionCards = question.MOTION_CARDS.map(card => ({
        id: card.ID,
        rectoTheme: card.RECTO_THEME || '',
        rectoImage: card.RECTO_IMAGE || null,
        difficulty: card.DIFFICULTY || 1,
        questionText: card.QUESTION_TEXT || '',
        questionImage: card.QUESTION_IMAGE || null,
        answerText: card.ANSWER_TEXT || '',
        answerImage: card.ANSWER_IMAGE || null,
        // #184/B-F4 — TYPE absent/vide ⇒ SPEEDY (contrat §3, rétrocompatibilité
        // stricte : les 9 questions MEMOTION existantes n'ont pas ce champ).
        // Défauts QCM synchronisés avec motionCardLock.js (QCM_CREATION_VALUES).
        type: card.TYPE || 'SPEEDY',
        qcmAnswers: card.QCM_ANSWERS || { RED: '', GREEN: '', YELLOW: '', BLUE: '' },
        qcmCorrect: card.QCM_CORRECT || '',
        qcmHintsEnabled: card.QCM_HINTS_ENABLED || false,
        qcmHintThreshold1: card.QCM_HINT_THRESHOLD_1 ?? 0.25,
        qcmHintThreshold2: card.QCM_HINT_THRESHOLD_2 ?? 0.125,
        qcmPenalty1: card.QCM_PENALTY_1 ?? 0.67,
        qcmPenalty2: card.QCM_PENALTY_2 ?? 0.33,
      }))
    }

    // Load MEMOTION points config from question data
    const mc = question.MOTION_CONFIG
    const motionConfig = {
      points1: mc?.POINTS_1_STAR ?? 1,
      points2: mc?.POINTS_2_STAR ?? 3,
      points3: mc?.POINTS_3_STAR ?? 5,
    }

    setFormData({
      question: question.QUESTION || '',
      answer: question.ANSWER || '',
      type: qType,
      category: question.CATEGORY || '',
      pointsTarget: question.POINTS_TARGET || defaultTarget,
      qcmAnswers: question.QCM_ANSWERS || { RED: '', GREEN: '', YELLOW: '', BLUE: '' },
      qcmCorrect: question.QCM_CORRECT || '',
      qcmHintsEnabled: question.QCM_HINTS_ENABLED || false,
      qcmHintThreshold1: question.QCM_HINT_THRESHOLD_1 || 0.25,
      qcmHintThreshold2: question.QCM_HINT_THRESHOLD_2 || 0.125,
      qcmPenalty1: question.QCM_PENALTY_1 || 0.67,
      qcmPenalty2: question.QCM_PENALTY_2 || 0.33,
      memoryMode: question.MEMORY_MODE || 'SOLO',
      memoryPairs,
      memoryConfig,
      motionMode: question.MOTION_MODE || 'SOLO',
      motionCards,
      motionConfig,
      motionMemorizeDuration: question.MOTION_MEMORIZE_DURATION || 0,
      // ARDOISE fields
      ardoiseKeyboardType: question.ARDOISE_KEYBOARD_TYPE || 'AZERTY',
      points: question.POINTS || '1',
      time: question.TIME || '30',
      media: null,
      existingMedia: question.MEDIA || null,
      mediaAnswer: null,
      existingMediaAnswer: question.MEDIA_ANSWER || null,
      // Note d'explication — animateur seul (v6.4.x, #168)
      explanation: question.EXPLANATION || '',
    })
    if (fileInputRef.current) {
      fileInputRef.current.value = ''
    }
    if (fileAnswerInputRef.current) {
      fileAnswerInputRef.current.value = ''
    }
  }

  const handleNewQuestion = () => {
    setEditingId(null)
    setFormData({
      question: '',
      answer: '',
      type: 'SPEEDY',
      category: '',
      pointsTarget: 'PLAYER',
      qcmAnswers: { RED: '', GREEN: '', YELLOW: '', BLUE: '' },
      qcmCorrect: '',
      qcmHintsEnabled: false,
      qcmHintThreshold1: 0.25,
      qcmHintThreshold2: 0.125,
      qcmPenalty1: 0.67,
      qcmPenalty2: 0.33,
      memoryMode: 'SOLO',
      memoryPairs: [
        { id: 1, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } },
        { id: 2, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } },
      ],
      memoryConfig: {
        flipDelay: 3,
        pointsPerPair: 10,
        errorPenalty: 0,
        completionBonus: 0,
        useTimer: true,
        memorizeTime: 5,
        showDuringMemorize: true,
        revealDelay: 0.5,
      },
      motionMode: 'SOLO',
      motionCards: [
        { id: 'mc-1', rectoTheme: '', rectoImage: null, difficulty: 1, questionText: '', questionImage: null, answerText: '', answerImage: null,
        type: 'SPEEDY',
        qcmAnswers: { RED: '', GREEN: '', YELLOW: '', BLUE: '' },
        qcmCorrect: '',
        qcmHintsEnabled: false,
        qcmHintThreshold1: 0.25,
        qcmHintThreshold2: 0.125,
        qcmPenalty1: 0.67,
        qcmPenalty2: 0.33,
      },
        { id: 'mc-2', rectoTheme: '', rectoImage: null, difficulty: 1, questionText: '', questionImage: null, answerText: '', answerImage: null,
        type: 'SPEEDY',
        qcmAnswers: { RED: '', GREEN: '', YELLOW: '', BLUE: '' },
        qcmCorrect: '',
        qcmHintsEnabled: false,
        qcmHintThreshold1: 0.25,
        qcmHintThreshold2: 0.125,
        qcmPenalty1: 0.67,
        qcmPenalty2: 0.33,
      },
      ],
      motionConfig: { points1: 1, points2: 3, points3: 5 },
      motionMemorizeDuration: 0,
      // ARDOISE fields
      ardoiseKeyboardType: 'AZERTY',
      points: '1',
      time: '30',
      media: null,
      existingMedia: null,
      mediaAnswer: null,
      existingMediaAnswer: null,
      // Note d'explication — animateur seul (v6.4.x, #168)
      explanation: '',
    })
    if (fileInputRef.current) {
      fileInputRef.current.value = ''
    }
    if (fileAnswerInputRef.current) {
      fileAnswerInputRef.current.value = ''
    }
  }

  const handleQcmAnswerChange = (color, value) => {
    setFormData(prev => ({
      ...prev,
      qcmAnswers: { ...prev.qcmAnswers, [color]: value }
    }))
  }

  // Memory handlers
  const handleAddMemoryPair = () => {
    setFormData(prev => {
      if (prev.memoryPairs.length >= 12) return prev // Max 12 pairs
      const maxId = Math.max(...prev.memoryPairs.map(p => p.id), 0)
      return {
        ...prev,
        memoryPairs: [
          ...prev.memoryPairs,
          { id: maxId + 1, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } }
        ]
      }
    })
  }

  const handleRemoveMemoryPair = (pairId) => {
    setFormData(prev => {
      if (prev.memoryPairs.length <= 2) return prev // Min 2 pairs
      return {
        ...prev,
        memoryPairs: prev.memoryPairs.filter(p => p.id !== pairId)
      }
    })
  }

  const handleMemoryCardChange = (pairId, cardKey, field, value) => {
    setFormData(prev => ({
      ...prev,
      memoryPairs: prev.memoryPairs.map(pair => {
        if (pair.id !== pairId) return pair
        const card = { ...pair[cardKey] }
        if (field === 'type') {
          // Toggle between text and image
          card.isImage = value === 'image'
          if (card.isImage) {
            card.text = ''
          } else {
            card.image = null
          }
        } else if (field === 'text') {
          card.text = value
        } else if (field === 'image') {
          card.image = value
        }
        return { ...pair, [cardKey]: card }
      })
    }))
  }

  const handleMemoryConfigChange = (field, value) => {
    setFormData(prev => ({
      ...prev,
      memoryConfig: { ...prev.memoryConfig, [field]: value }
    }))
  }

  const getMemoryGridColumns = (pairCount) => {
    const cardCount = pairCount * 2
    if (cardCount <= 4) return 2
    if (cardCount <= 8) return 4
    if (cardCount <= 12) return 4
    if (cardCount <= 16) return 4
    if (cardCount <= 20) return 5
    return 6
  }

  // MEMOTION handlers
  const handleAddMotionCard = () => {
    setFormData(prev => {
      if (prev.motionCards.length >= 12) return prev // Max 12 cards
      const idx = prev.motionCards.length + 1
      const newId = `mc-${Date.now()}`
      return {
        ...prev,
        motionCards: [
          ...prev.motionCards,
          { id: newId, rectoTheme: '', rectoImage: null, difficulty: 1, questionText: '', questionImage: null, answerText: '', answerImage: null,
        type: 'SPEEDY',
        qcmAnswers: { RED: '', GREEN: '', YELLOW: '', BLUE: '' },
        qcmCorrect: '',
        qcmHintsEnabled: false,
        qcmHintThreshold1: 0.25,
        qcmHintThreshold2: 0.125,
        qcmPenalty1: 0.67,
        qcmPenalty2: 0.33,
      },
        ]
      }
    })
  }

  const handleRemoveMotionCard = (cardId) => {
    setFormData(prev => {
      if (prev.motionCards.length <= 2) return prev // Min 2 cards
      return { ...prev, motionCards: prev.motionCards.filter(c => c.id !== cardId) }
    })
  }

  const handleMotionCardChange = (cardId, field, value) => {
    setFormData(prev => ({
      ...prev,
      motionCards: prev.motionCards.map(c => c.id !== cardId ? c : { ...c, [field]: value })
    }))
  }

  const handleSubmit = async (e) => {
    e.preventDefault()
    // For normal questions, need question and answer
    // For QCM, need question and at least the correct answer filled
    // For Memory, need question and at least 2 valid pairs
    if (!formData.question) return
    if (formData.type === 'SPEEDY' && !formData.answer) return
    if (formData.type === 'ARDOISE' && !formData.answer) return
    if (formData.type === 'QCM' && (!formData.qcmCorrect || !formData.qcmAnswers[formData.qcmCorrect])) return
    if (formData.type === 'MEMORY') {
      // Validate at least 2 pairs with all cards filled
      const validPairs = formData.memoryPairs.filter(pair => {
        const card1Valid = pair.card1.isImage ? pair.card1.image : pair.card1.text
        const card2Valid = pair.card2.isImage ? pair.card2.image : pair.card2.text
        return card1Valid && card2Valid
      })
      if (validPairs.length < 2) return
    }
    if (formData.type === 'MEMOTION') {
      // Validate at least 2 cards with theme
      const validCards = formData.motionCards.filter(c => c.rectoTheme.trim())
      if (validCards.length < 2) return
    }

    setIsUploading(true)

    const data = new FormData()
    if (editingId) {
      data.append('number', editingId)
    }
    data.append('question', formData.question)
    data.append('type', formData.type)
    if (formData.category) {
      data.append('category', formData.category)
    }
    data.append('points_target', formData.pointsTarget)
    data.append('points', formData.points)
    data.append('time', formData.time)
    // Note d'explication — animateur seul (v6.4.x, #168). Piège
    // handleUploadQuestion (backend) : sans cette lecture explicite, la note
    // serait détruite à chaque édition. Vidé si le champ est vide — c'est le
    // mécanisme d'effacement, sans code dédié côté serveur.
    data.append('explanation', formData.explanation || '')

    if (formData.type === 'SPEEDY') {
      data.append('answer', formData.answer)
    } else if (formData.type === 'ARDOISE') {
      // ARDOISE mode — store answer (correct response, shown to admin only) and keyboard type
      data.append('answer', formData.answer)
      data.append('ardoise_keyboard_type', formData.ardoiseKeyboardType || 'AZERTY')
    } else if (formData.type === 'QCM') {
      // QCM mode - send QCM answers and correct answer
      data.append('qcm_answers', JSON.stringify(formData.qcmAnswers))
      data.append('qcm_correct', formData.qcmCorrect)
      // The answer field contains the correct answer text for display
      data.append('answer', formData.qcmAnswers[formData.qcmCorrect])
      // QCM hints enabled flag and thresholds
      data.append('qcm_hints_enabled', formData.qcmHintsEnabled ? 'true' : 'false')
      if (formData.qcmHintsEnabled) {
        data.append('qcm_hint_threshold_1', formData.qcmHintThreshold1.toString())
        data.append('qcm_hint_threshold_2', formData.qcmHintThreshold2.toString())
        data.append('qcm_penalty_1', formData.qcmPenalty1.toString())
        data.append('qcm_penalty_2', formData.qcmPenalty2.toString())
      }
    } else if (formData.type === 'MEMORY') {
      // Memory mode - send pairs, config, and mode
      data.append('MEMORY_MODE', formData.memoryMode)

      // Serialize pairs (convert File objects to flags, actual files uploaded separately)
      const serializedPairs = formData.memoryPairs.map(pair => ({
        ID: pair.id,
        CARD1: {
          TEXT: pair.card1.isImage ? '' : pair.card1.text,
          IMAGE: typeof pair.card1.image === 'string' ? pair.card1.image : '',
          IS_IMAGE: pair.card1.isImage,
        },
        CARD2: {
          TEXT: pair.card2.isImage ? '' : pair.card2.text,
          IMAGE: typeof pair.card2.image === 'string' ? pair.card2.image : '',
          IS_IMAGE: pair.card2.isImage,
        },
      }))
      data.append('memory_pairs', JSON.stringify(serializedPairs))

      // Send config
      const config = {
        FLIP_DELAY: formData.memoryConfig.flipDelay,
        POINTS_PER_PAIR: formData.memoryConfig.pointsPerPair,
        ERROR_PENALTY: formData.memoryConfig.errorPenalty,
        COMPLETION_BONUS: formData.memoryConfig.completionBonus,
        USE_TIMER: formData.memoryConfig.useTimer,
        MEMORIZE_TIME: formData.memoryConfig.memorizeTime,
        SHOW_DURING_MEMORIZE: formData.memoryConfig.showDuringMemorize,
        REVEAL_DELAY: formData.memoryConfig.revealDelay,
      }
      data.append('memory_config', JSON.stringify(config))

      // Append image files
      formData.memoryPairs.forEach(pair => {
        if (pair.card1.image instanceof File) {
          data.append(`memory_card_${pair.id}_1`, pair.card1.image)
        }
        if (pair.card2.image instanceof File) {
          data.append(`memory_card_${pair.id}_2`, pair.card2.image)
        }
      })

      // Set answer to number of pairs for display
      data.append('answer', `${formData.memoryPairs.length} paires`)
    } else if (formData.type === 'MEMOTION') {
      // MEMOTION mode — serialize cards and mode
      data.append('MOTION_MODE', formData.motionMode)

      // #184/B-F4 — TYPE explicite + contenu propre au type de la carte
      // (contrat §3.1/§7) : SPEEDY porte ANSWER_TEXT/ANSWER_IMAGE (historique,
      // inchangé), QCM porte les champs TypedContent partagés avec Question
      // (mêmes noms JSON, §2). Jamais les deux à la fois — c'est exactement
      // ce que vérifie le serveur (CARD_TYPE_CONTENT_MISMATCH, B-B2).
      const serializedCards = formData.motionCards.map(card => {
        const cardType = card.type || 'SPEEDY'
        const base = {
          ID: card.id,
          TYPE: cardType,
          RECTO_THEME: card.rectoTheme,
          RECTO_IMAGE: typeof card.rectoImage === 'string' ? card.rectoImage : '',
          DIFFICULTY: card.difficulty,
          QUESTION_TEXT: card.questionText,
          QUESTION_IMAGE: typeof card.questionImage === 'string' ? card.questionImage : '',
        }
        if (cardType === 'QCM') {
          return {
            ...base,
            QCM_ANSWERS: card.qcmAnswers,
            QCM_CORRECT: card.qcmCorrect,
            QCM_HINTS_ENABLED: card.qcmHintsEnabled,
            QCM_HINT_THRESHOLD_1: card.qcmHintThreshold1,
            QCM_HINT_THRESHOLD_2: card.qcmHintThreshold2,
            QCM_PENALTY_1: card.qcmPenalty1,
            QCM_PENALTY_2: card.qcmPenalty2,
          }
        }
        return {
          ...base,
          ANSWER_TEXT: card.answerText,
          ANSWER_IMAGE: typeof card.answerImage === 'string' ? card.answerImage : '',
        }
      })
      data.append('motion_cards', JSON.stringify(serializedCards))
      data.append('motion_config', JSON.stringify({
        POINTS_1_STAR: parseInt(formData.motionConfig?.points1) || 1,
        POINTS_2_STAR: parseInt(formData.motionConfig?.points2) || 3,
        POINTS_3_STAR: parseInt(formData.motionConfig?.points3) || 5,
      }))
      data.append('MOTION_MEMORIZE_DURATION', String(formData.motionMemorizeDuration || 0))

      // Append image files (per face, per card)
      formData.motionCards.forEach(card => {
        if (card.rectoImage instanceof File) data.append(`motion_card_${card.id}_recto`, card.rectoImage)
        if (card.questionImage instanceof File) data.append(`motion_card_${card.id}_question`, card.questionImage)
        if (card.answerImage instanceof File) data.append(`motion_card_${card.id}_answer`, card.answerImage)
      })

      // Set answer to number of cards for display
      data.append('answer', `${formData.motionCards.length} cartes`)
    }

    if (formData.media) {
      data.append('file', formData.media)
    }

    if (formData.mediaAnswer) {
      data.append('file_answer', formData.mediaAnswer)
    }

    try {
      const response = await fetch('/questions', {
        method: 'POST',
        body: data,
      })

      if (response.ok) {
        handleNewQuestion()
      }
    } catch (error) {
      console.error('Upload failed:', error)
    } finally {
      setIsUploading(false)
    }
  }

  const handleDelete = (e, questionId) => {
    e.stopPropagation()
    if (window.confirm(`Supprimer la question #${questionId} ?`)) {
      deleteQuestion(questionId)
      if (editingId === questionId) {
        handleNewQuestion()
      }
    }
  }

  // Handler for QuestionCard onDelete prop (receives ID directly)
  const handleDeleteQuestion = (questionId) => {
    if (window.confirm(`Supprimer la question #${questionId} ?`)) {
      deleteQuestion(questionId)
      if (editingId === questionId) {
        handleNewQuestion()
      }
    }
  }

  const storagePercent = fsInfo?.P_USED ? parseFloat(fsInfo.P_USED) : 0

  return (
    <div className="questions-page page">
      <header className="page-header">
        <h1 className="page-title">Gestion des Questions</h1>
        <p className="page-subtitle">
          {filteredQuestions.length !== sortedQuestions.length
            ? `${filteredQuestions.length} / ${sortedQuestions.length} questions`
            : `${sortedQuestions.length} questions disponibles`}
        </p>
      </header>

      {/* Zone 1 — Quiz */}
      <section className="quiz-meta-section" ref={quizMetaSectionRef}>
        <Card padding="lg">
          <CardHeader>
            <div className="section-header">
              <h3 className="section-title">Quiz</h3>
              <Button variant="fun" size="sm" onClick={newGame} title="Réinitialiser le jeu et préparer une nouvelle partie">
                NOUVELLE PARTIE
              </Button>
            </div>
          </CardHeader>
          <CardBody>
            <form onSubmit={handleSaveQuizMeta} className="quiz-meta-form">
              <div className="quiz-meta-grid">
                <div className="form-group">
                  <label htmlFor="quiz-name">Nom du quiz</label>
                  <input
                    id="quiz-name"
                    type="text"
                    value={quizName}
                    onChange={e => setQuizName(e.target.value)}
                    placeholder="Ex : Quiz Science et Nature"
                  />
                </div>
                <div className="form-group">
                  <label htmlFor="quiz-theme">
                    Thème général
                    <span className="quiz-tv-toggle">
                      <button
                        type="button"
                        role="switch"
                        aria-checked={isQuizFieldVisibleOnTV('THEME')}
                        aria-label="Afficher le thème sur la TV"
                        className={`quiz-switch ${isQuizFieldVisibleOnTV('THEME') ? 'on' : ''}`}
                        onClick={() => toggleQuizFieldVisibility('THEME')}
                      />
                      TV
                    </span>
                  </label>
                  <input
                    id="quiz-theme"
                    type="text"
                    value={quizTheme}
                    onChange={e => setQuizTheme(e.target.value)}
                    placeholder="Ex : Culture générale"
                  />
                </div>
                {/* v6.1.0 (#137) — publics/difficultés multiples (chips, motif
                    AIGenerateModal.jsx catégories) + interrupteur "Afficher sur
                    la TV" par champ (contract game-state.md, QUIZ_HIDDEN_FIELDS) */}
                <div className="form-group quiz-meta-wide">
                  <span className="quiz-field-label">
                    Publics cibles <span className="quiz-field-hint">— au moins un</span>
                    <span className="quiz-tv-toggle">
                      <button
                        type="button"
                        role="switch"
                        aria-checked={isQuizFieldVisibleOnTV('POPULATIONS')}
                        aria-label="Afficher les publics sur la TV"
                        className={`quiz-switch ${isQuizFieldVisibleOnTV('POPULATIONS') ? 'on' : ''}`}
                        onClick={() => toggleQuizFieldVisibility('POPULATIONS')}
                      />
                      TV
                    </span>
                  </span>
                  <div className="quiz-chip-row">
                    {QUIZ_POPULATIONS.map(p => (
                      <button
                        type="button"
                        key={p}
                        className={`quiz-chip ${quizPopulations.includes(p) ? 'active' : ''}`}
                        onClick={() => toggleQuizPopulation(p)}
                      >
                        {quizPopulations.includes(p) && <span className="quiz-chip-check" aria-hidden="true">✓</span>}
                        {p}
                      </button>
                    ))}
                  </div>
                </div>
                <div className="form-group quiz-meta-wide">
                  <span className="quiz-field-label">
                    Difficultés visées <span className="quiz-field-hint">— au moins une</span>
                    <span className="quiz-tv-toggle">
                      <button
                        type="button"
                        role="switch"
                        aria-checked={isQuizFieldVisibleOnTV('DIFFICULTIES')}
                        aria-label="Afficher les difficultés sur la TV"
                        className={`quiz-switch ${isQuizFieldVisibleOnTV('DIFFICULTIES') ? 'on' : ''}`}
                        onClick={() => toggleQuizFieldVisibility('DIFFICULTIES')}
                      />
                      TV
                    </span>
                  </span>
                  <div className="quiz-chip-row">
                    {QUIZ_DIFFICULTIES.map(d => (
                      <button
                        type="button"
                        key={d}
                        className={`quiz-chip ${quizDifficulties.includes(d) ? 'active' : ''}`}
                        onClick={() => toggleQuizDifficulty(d)}
                      >
                        {quizDifficulties.includes(d) && <span className="quiz-chip-check" aria-hidden="true">✓</span>}
                        {d}
                      </button>
                    ))}
                  </div>
                  <span className="quiz-field-hint">
                    Interrupteur éteint : sélection prise en compte pour la génération, mais non annoncée aux joueurs.
                  </span>
                </div>
                <div className="form-group">
                  <label htmlFor="quiz-language">
                    Langue
                    <span className="quiz-tv-toggle">
                      <button
                        type="button"
                        role="switch"
                        aria-checked={isQuizFieldVisibleOnTV('LANGUAGE')}
                        aria-label="Afficher la langue sur la TV"
                        className={`quiz-switch ${isQuizFieldVisibleOnTV('LANGUAGE') ? 'on' : ''}`}
                        onClick={() => toggleQuizFieldVisibility('LANGUAGE')}
                      />
                      TV
                    </span>
                  </label>
                  <select
                    id="quiz-language"
                    value={quizLanguage}
                    onChange={e => setQuizLanguage(e.target.value)}
                  >
                    {QUIZ_LANGUAGES.map(l => <option key={l} value={l}>{l}</option>)}
                  </select>
                </div>
                <div className="form-group quiz-meta-wide">
                  <span className="quiz-field-label">
                    Objectif de la partie
                    <span className="quiz-field-visibility private">🔒 Non affiché aux joueurs</span>
                  </span>
                  <textarea
                    id="quiz-objectives"
                    value={quizObjectives}
                    onChange={e => setQuizObjectives(e.target.value)}
                    placeholder="Ex : révision du chapitre 3, team building, faire marquer les timides..."
                    rows={2}
                    maxLength={2000}
                  />
                  <span className="quiz-field-hint">Sert de consigne au générateur IA et de rappel pour l'animateur.</span>
                </div>
                <div className="form-group quiz-meta-wide">
                  <span className="quiz-field-label">
                    Texte libre
                    <span className="quiz-field-visibility public">📺 Affiché aux joueurs</span>
                  </span>
                  <textarea
                    id="quiz-notes"
                    value={quizNotes}
                    onChange={e => setQuizNotes(e.target.value)}
                    placeholder="Notes, règles, anecdotes..."
                    rows={3}
                  />
                </div>
              </div>
              <div className="quiz-meta-actions">
                <Button type="submit" variant="primary" size="sm">
                  {quizSaved ? 'Enregistré ✓' : 'Enregistrer'}
                </Button>
              </div>
            </form>

            {/* Image(s) de fond — écran "Nouvelle Partie" */}
            <div className="new-game-bg-section">
              <div className="new-game-bg-header">
                <h4 className="new-game-bg-title">Image(s) de fond — Nouvelle Partie</h4>
                <div style={{ display: 'flex', gap: 'var(--space-2)' }}>
                  <label className="upload-bg-btn">
                    <input
                      type="file"
                      ref={ngBgInputRef}
                      accept="image/*"
                      onChange={handleNgBackgroundUpload}
                      style={{ display: 'none' }}
                    />
                    <Button variant="primary" size="sm" as="span" loading={uploadingNgBg}>
                      + Image
                    </Button>
                  </label>
                  {gameState?.newGameBackgrounds?.length > 0 && (
                    <Button variant="ghost" size="sm" onClick={handleRemoveAllNgBackgrounds}>
                      Tout supprimer
                    </Button>
                  )}
                </div>
              </div>
              <p className="new-game-bg-hint">
                Images affichees en rotation sur l'ecran TV lors de la phase "Nouvelle Partie". Par defaut, un degrade anime est utilise.
              </p>
              <p className="section-hint">Glissez-deposez pour changer l'ordre.</p>
              <div className="backgrounds-grid">
                {gameState?.newGameBackgrounds?.length > 0 ? (
                  gameState.newGameBackgrounds.map((bg, index) => (
                    <motion.div
                      key={bg.path}
                      className={`background-item ${draggedNgBgIndex === index ? 'dragging' : ''}`}
                      initial={{ opacity: 0, scale: 0.9 }}
                      animate={{ opacity: 1, scale: 1 }}
                      transition={{ delay: index * 0.05 }}
                      draggable
                      onDragStart={() => setDraggedNgBgIndex(index)}
                      onDragEnd={() => setDraggedNgBgIndex(null)}
                      onDragOver={(e) => e.preventDefault()}
                      onDrop={() => {
                        if (draggedNgBgIndex !== null) {
                          handleNgMoveBackground(draggedNgBgIndex, index)
                        }
                      }}
                    >
                      <img src={bg.path} alt={`Nouvelle Partie fond ${index + 1}`} className="bg-thumb" />
                      <button
                        className="bg-delete-btn"
                        onClick={() => handleRemoveNgBackground(bg.path)}
                        title="Supprimer"
                      >
                        ×
                      </button>
                      <span className="bg-index">{index + 1}</span>
                      <div className="bg-controls">
                        <div className="bg-duration">
                          <input
                            type="number"
                            min="1"
                            max="300"
                            value={bg.duration || 10}
                            onChange={(e) => handleNgDurationChange(index, e.target.value)}
                            className="duration-input"
                          />
                          <span className="duration-label">s</span>
                        </div>
                        <div className="bg-opacity">
                          <input
                            type="range"
                            min="0"
                            max="100"
                            value={bg.opacity ?? 100}
                            onChange={(e) => handleNgOpacityChange(index, e.target.value)}
                            className="opacity-slider"
                          />
                          <span className="opacity-value">{bg.opacity ?? 100}%</span>
                        </div>
                      </div>
                    </motion.div>
                  ))
                ) : (
                  <div className="backgrounds-empty">
                    <p className="empty-state">Aucune image (degrade anime par defaut)</p>
                  </div>
                )}
              </div>
            </div>

            {/* ENTRACTE (#119, corrections C1) — configuration du panneau de
                pause globale, propriété de la partie (comme le reste de cette
                page). Le déclenchement lui-même se fait depuis la barre de
                navigation (bouton ENTRACTE / FIN D'ENTRACTE, C2). */}
            <form onSubmit={handleSaveEntracteConfig} className="new-game-bg-section">
              <div className="new-game-bg-header">
                <h4 className="new-game-bg-title">Entracte (pause globale)</h4>
              </div>
              <p className="new-game-bg-hint">
                Panneau affiche sur l'ecran TV et VJoueur pendant une pause globale (repas, changement de salle...), declenchee depuis la barre de navigation. Le reste de l'interface (TV, VJoueur, admin, animateur) est estompe pendant toute la duree de la pause.
              </p>

              <div className="wifi-form">
                <label className="wifi-field">
                  <span>Titre</span>
                  <input
                    type="text"
                    value={entracteTitle}
                    onChange={(e) => setEntracteTitle(e.target.value)}
                    placeholder="ENTRACTE"
                    maxLength={40}
                  />
                </label>
                <label className="wifi-field">
                  <span>Sous-titre</span>
                  <input
                    type="text"
                    value={entracteSubtitle}
                    onChange={(e) => setEntracteSubtitle(e.target.value)}
                    placeholder="Retour dans 20mn"
                    maxLength={80}
                  />
                </label>
              </div>

              <div className="default-image-preview">
                {entracteImageIsCustom ? (
                  <img
                    src={`/api/game/entracte-image?t=${entracteImageCacheBuster}`}
                    alt="Image d'entracte"
                    className="default-image-thumbnail"
                  />
                ) : (
                  <span className="default-image-filename">Aucune image (panneau sans fond)</span>
                )}
                {entracteImageIsCustom && (
                  <span className="default-image-filename">Image personnalisée</span>
                )}
              </div>

              <div className="firmware-upload-row">
                <input
                  ref={entracteImageFileRef}
                  type="file"
                  accept=".jpg,.jpeg,.png,.gif,.webp,.svg"
                  className="firmware-file-input"
                  id="entracte-image-file-input"
                />
                <label htmlFor="entracte-image-file-input" className="firmware-file-label">
                  Choisir une image (jpg, png, gif, webp, svg)
                </label>
              </div>

              <div className="config-section-actions">
                <Button type="button" variant="primary" onClick={handleEntracteImageUpload} loading={uploadingEntracteImage}>
                  Enregistrer l'image
                </Button>
                {entracteImageIsCustom && (
                  <Button type="button" variant="secondary" onClick={handleEntracteImageDelete} loading={deletingEntracteImage}>
                    Retirer l'image
                  </Button>
                )}
              </div>

              <div className="slider-group">
                <div className="slider-row">
                  <label>Taille du panneau</label>
                  <div className="slider-control">
                    <input
                      type="range"
                      min="20"
                      max="100"
                      value={entractePanelSize}
                      onChange={(e) => setEntractePanelSize(parseInt(e.target.value))}
                    />
                    <span className="slider-value">{entractePanelSize}%</span>
                  </div>
                  <p className="section-hint">
                    Même réglage, même rendu sur TV et VJoueur — pas de taille séparée par écran.
                  </p>
                </div>

                <div className="slider-row">
                  <label>Vitesse du mouvement</label>
                  <div className="slider-control">
                    <input
                      type="range"
                      min="2"
                      max="30"
                      value={entracteAnimPeriod}
                      onChange={(e) => setEntracteAnimPeriod(parseInt(e.target.value))}
                    />
                    <span className="slider-value">{entracteAnimPeriod}s</span>
                  </div>
                  <p className="section-hint">Durée d'un cycle complet — plus court = plus rapide.</p>
                </div>

                <div className="slider-row">
                  <label>Intensité du mouvement</label>
                  <div className="slider-control">
                    <input
                      type="range"
                      min="0"
                      max="100"
                      value={entracteAnimIntensity}
                      onChange={(e) => setEntracteAnimIntensity(parseInt(e.target.value))}
                    />
                    <span className="slider-value">
                      {entracteAnimIntensity === 0 ? 'animation désactivée' : entracteAnimIntensity}
                    </span>
                  </div>
                </div>

                <div className="slider-row">
                  <label>Vitesse de transition</label>
                  <div className="slider-control">
                    <input
                      type="range"
                      min="0"
                      max="10000"
                      step="100"
                      value={entracteTransitionMs}
                      onChange={(e) => setEntracteTransitionMs(parseInt(e.target.value))}
                    />
                    <span className="slider-value">
                      {entracteTransitionMs === 0 ? 'transition instantanée' : `${(entracteTransitionMs / 1000).toFixed(1)}s`}
                    </span>
                  </div>
                  <p className="section-hint">Durée du fondu à l'entrée et à la sortie de la pause.</p>
                </div>
              </div>

              <div className="config-section-actions">
                <Button type="submit" variant="primary">
                  {entracteSaved ? 'Enregistré ✓' : 'Enregistrer'}
                </Button>
                {gameState.entracte && (
                  <span className="section-hint" role="status">
                    Un entracte est en cours — prendra effet au prochain entracte.
                  </span>
                )}
              </div>
            </form>
          </CardBody>
        </Card>
      </section>

      {/* Zone 2 — Ambiance (fonds d'écran) */}
      <section className="background-section">
        <Card padding="lg">
          <CardHeader>
            <div className="section-header">
              <h3 className="section-title">Fonds d'ecran</h3>
              <div style={{ display: 'flex', gap: 'var(--space-2)' }}>
                <label className="upload-bg-btn">
                  <input
                    type="file"
                    ref={bgInputRef}
                    accept="image/*"
                    onChange={handleBackgroundUpload}
                    style={{ display: 'none' }}
                  />
                  <Button variant="primary" size="sm" as="span" loading={uploadingBg}>
                    + Image
                  </Button>
                </label>
                {gameState?.backgrounds?.length > 0 && (
                  <Button variant="ghost" size="sm" onClick={handleRemoveAllBackgrounds}>
                    Tout supprimer
                  </Button>
                )}
              </div>
            </div>
          </CardHeader>
          <CardBody>
            <p className="section-hint">Glissez-deposez pour changer l'ordre.</p>
            <div className="backgrounds-grid">
              {gameState?.backgrounds?.length > 0 ? (
                gameState.backgrounds.map((bg, index) => (
                  <motion.div
                    key={bg.path}
                    className={`background-item ${draggedBgIndex === index ? 'dragging' : ''}`}
                    initial={{ opacity: 0, scale: 0.9 }}
                    animate={{ opacity: 1, scale: 1 }}
                    transition={{ delay: index * 0.05 }}
                    draggable
                    onDragStart={() => setDraggedBgIndex(index)}
                    onDragEnd={() => setDraggedBgIndex(null)}
                    onDragOver={(e) => e.preventDefault()}
                    onDrop={() => {
                      if (draggedBgIndex !== null) {
                        handleMoveBackground(draggedBgIndex, index)
                      }
                    }}
                  >
                    <img src={bg.path} alt={`Background ${index + 1}`} className="bg-thumb" />
                    <button
                      className="bg-delete-btn"
                      onClick={() => handleRemoveBackground(bg.path)}
                      title="Supprimer"
                    >
                      ×
                    </button>
                    <span className="bg-index">{index + 1}</span>
                    <div className="bg-controls">
                      <div className="bg-duration">
                        <input
                          type="number"
                          min="1"
                          max="300"
                          value={bg.duration || 10}
                          onChange={(e) => handleDurationChange(index, e.target.value)}
                          className="duration-input"
                        />
                        <span className="duration-label">s</span>
                      </div>
                      <div className="bg-opacity">
                        <input
                          type="range"
                          min="0"
                          max="100"
                          value={bg.opacity ?? 100}
                          onChange={(e) => handleOpacityChange(index, e.target.value)}
                          className="opacity-slider"
                        />
                        <span className="opacity-value">{bg.opacity ?? 100}%</span>
                      </div>
                    </div>
                  </motion.div>
                ))
              ) : (
                <div className="backgrounds-empty">
                  <p className="empty-state">Aucune image de fond</p>
                </div>
              )}
            </div>
          </CardBody>
        </Card>
      </section>

      {/* Category Balance + Filters — div unique pour alignement parfait */}
      <div className="category-filter-group">
        <CategoryBalance questions={sortedQuestions} />

        {/* Category filter bar (#40) — supports custom categories */}
        {availableCategories.length > 0 && (
          <div className="category-filter-bar questions-page-filter-bar">
            {availableCategories.map(catKey => {
              const meta = categoryMeta(catKey, customCategories)
              if (!meta) return null
              const isActive = selectedCategories.has(catKey)
              return (
                <button
                  key={catKey}
                  className={`category-filter-pill${isActive ? ' active' : ''}`}
                  style={{ '--cat-color': meta.color }}
                  onClick={() => toggleCategoryFilter(catKey)}
                  title={meta.label}
                >
                  <CategoryBadge catKey={catKey} customCategories={customCategories} size="md" chip={false} />
                  <span className="cat-pill-label">{meta.label}</span>
                </button>
              )
            })}
            {selectedCategories.size > 0 && (
              <button
                className="category-filter-reset"
                onClick={clearCategoryFilters}
                title="Réinitialiser les filtres"
              >
                ×
              </button>
            )}
          </div>
        )}
      </div>

      {/* #149 — barre de mélange, entre les filtres et la grille (maquette validée §1) */}
      <div className="questions-shuffle-toolbar">
        <span className="questions-shuffle-toolbar-count">
          {sortedQuestions.length} question{sortedQuestions.length > 1 ? 's' : ''}
        </span>
        <Button
          variant="secondary"
          size="sm"
          icon="⇄"
          onClick={handleShuffleQuestions}
          disabled={sortedQuestions.length < 2}
          title={sortedQuestions.length < 2 ? 'Au moins 2 questions sont necessaires pour melanger' : undefined}
        >
          Mélanger les questions
        </Button>
      </div>

      <div className="questions-layout">
        {/* Questions List */}
        <section className="questions-list-section">
          <div className="questions-grid">
            <AnimatePresence>
              {filteredQuestions.map((question, index) => (
                <QuestionCard
                  key={question.ID}
                  question={question}
                  selected={editingId === question.ID}
                  draggable
                  showDelete
                  customCategories={customCategories}
                  onClick={() => handleQuestionClick(question)}
                  onDelete={handleDeleteQuestion}
                  dragHandlers={{
                    index,
                    isDragging: draggedId === question.ID,
                    isDragOver: dragOverId === question.ID,
                    onDragStart: (e) => handleDragStart(e, question.ID),
                    onDragOver: (e) => handleDragOver(e, question.ID),
                    onDragLeave: handleDragLeave,
                    onDragEnd: handleDragEnd,
                    onDrop: (e) => handleDrop(e, question.ID),
                  }}
                />
              ))}
            </AnimatePresence>
            {filteredQuestions.length === 0 && selectedCategories.size > 0 && (
              <div className="category-filter-empty">
                Aucune question dans cette catégorie.
              </div>
            )}
          </div>
        </section>

        {/* Sidebar */}
        <aside className="questions-sidebar">

          {/* Question Form */}
          <Card padding="lg" className="add-form-card">
            <CardHeader>
              <div className="form-header">
                <h3 className="sidebar-title">
                  {editingId ? `Modifier Question #${editingId}` : 'Nouvelle Question'}
                </h3>
                <div className="form-header-actions">
                  {/* v6.0.0 (#8) — désactivé tant qu'aucune clé n'est configurée pour le
                      provider sélectionné (maquette 137 §7, contract ai-multi-provider.md §8).
                      Reste actif si un job tourne déjà (ré-attachement, #137). */}
                  <Button
                    variant="primary"
                    size="sm"
                    onClick={() => setShowAIModal(true)}
                    disabled={!canOpenAiModal}
                    title={canOpenAiModal ? 'Générer des questions via IA' : 'Configurer une clé API dans Paramètres pour activer la génération IA'}
                  >
                    ✨ Générer via IA
                  </Button>
                  {editingId && (
                    <Button variant="ghost" size="sm" onClick={handleNewQuestion}>
                      + Nouveau
                    </Button>
                  )}
                </div>
              </div>
              {!canOpenAiModal && (
                <p className="ai-generate-hint">
                  <span className="ai-generate-hint-dot" aria-hidden="true">●</span>
                  Configurer une clé API dans Paramètres pour activer la génération IA
                </p>
              )}
            </CardHeader>
            <CardBody>
              <form onSubmit={handleSubmit} className="add-form">
                <div className="form-group">
                  <label htmlFor="question-input">Question *</label>
                  <textarea
                    id="question-input"
                    value={formData.question}
                    onChange={(e) => handleInputChange('question', e.target.value)}
                    placeholder="Entrez la question..."
                    rows={3}
                    required
                  />
                </div>

                {/* Question Type Selector — généré depuis la table unique
                    utils/questionTypeMeta.js (#183/A-F2), regroupement visuel
                    3+2 conservé à l'identique (rows CSS .type-filter-row) */}
                <div className="form-group">
                  <label>Type de question</label>
                  <div className="type-filter-grid">
                    {[QUESTION_TYPES.slice(0, 3), QUESTION_TYPES.slice(3)].map((row, rowIdx) => (
                      <div className="type-filter-row" key={rowIdx}>
                        {row.map(t => (
                          <button
                            key={t.key}
                            type="button"
                            className={`type-btn ${t.key.toLowerCase()} ${formData.type === t.key ? 'active' : ''}`}
                            onClick={() => handleInputChange('type', t.key)}
                          >
                            {t.label}
                          </button>
                        ))}
                      </div>
                    ))}
                  </div>
                </div>

                {/* Category Selector — boucle unifiée hardcoded + custom (#95) + bouton + (#97) */}
                <div className="form-group">
                  <label>Categorie</label>
                  <div className="category-selector">
                    {[...Object.keys(CATEGORIES), ...customCategories.map(c => c.key)].map(key => {
                      const meta = categoryMeta(key, customCategories)
                      if (!meta) return null
                      return (
                        <button
                          key={key}
                          type="button"
                          className={`category-btn ${formData.category === key ? 'active' : ''}`}
                          style={{ '--cat-color': meta.color }}
                          onClick={() => handleInputChange('category', formData.category === key ? '' : key)}
                          title={meta.label}
                        >
                          <CategoryBadge catKey={key} customCategories={customCategories} size="lg" chip={false} />
                        </button>
                      )
                    })}
                    {/* Bouton + pour créer une catégorie (#97) */}
                    {!showAddCategory && (
                      <button
                        type="button"
                        className="category-btn category-btn--add"
                        title="Créer une catégorie"
                        onClick={() => { setShowAddCategory(true); setAddCategoryError('') }}
                      >
                        +
                      </button>
                    )}
                  </div>
                  {/* Formulaire inline ajout catégorie (#97) */}
                  {showAddCategory && (
                    <div className="add-category-inline">
                      <input
                        type="text"
                        className="add-category-input"
                        placeholder="Nom de la catégorie..."
                        value={newCategoryName}
                        maxLength={50}
                        onChange={(e) => { setNewCategoryName(e.target.value); setAddCategoryError('') }}
                        onKeyDown={(e) => { if (e.key === 'Escape') { setShowAddCategory(false); setNewCategoryName(''); setNewCategoryFile(null) } }}
                        autoFocus
                      />
                      <label className="add-category-file-label">
                        <input
                          type="file"
                          accept=".png,.jpg,.jpeg,.webp"
                          style={{ display: 'none' }}
                          onChange={(e) => { setNewCategoryFile(e.target.files[0] || null); setAddCategoryError('') }}
                        />
                        <span className="add-category-file-btn">
                          {newCategoryFile ? '✓ ' + newCategoryFile.name : '📁 Choisir une image…'}
                        </span>
                      </label>
                      <div className="add-category-actions">
                        <button
                          type="button"
                          className="add-category-validate"
                          onClick={async () => {
                            if (!newCategoryName.trim()) { setAddCategoryError('Nom invalide'); return }
                            if (!newCategoryFile) { setAddCategoryError('Image requise'); return }
                            try {
                              const fd = new FormData()
                              fd.append('name', newCategoryName.trim())
                              fd.append('file', newCategoryFile)
                              // Ne PAS définir Content-Type — le browser gère le boundary
                              const res = await fetch('/api/categories', { method: 'POST', body: fd })
                              if (res.ok) {
                                const created = await res.json()
                                setShowAddCategory(false)
                                setNewCategoryName('')
                                setNewCategoryFile(null)
                                setAddCategoryError('')
                                await refetchCategories()
                                handleInputChange('category', created.key)
                              } else if (res.status === 409) {
                                setAddCategoryError('Cette catégorie existe déjà')
                              } else {
                                setAddCategoryError('Nom invalide ou image non supportée')
                              }
                            } catch {
                              setAddCategoryError('Erreur réseau')
                            }
                          }}
                        >
                          Valider
                        </button>
                        <button
                          type="button"
                          className="add-category-cancel"
                          onClick={() => { setShowAddCategory(false); setNewCategoryName(''); setNewCategoryFile(null); setAddCategoryError('') }}
                        >
                          Annuler
                        </button>
                      </div>
                      {addCategoryError && <p className="add-category-error">{addCategoryError}</p>}
                    </div>
                  )}
                  {formData.category && (() => {
                    const meta = categoryMeta(formData.category, customCategories)
                    if (!meta) return null
                    return <span className="category-label" style={{ color: meta.color }}>{meta.label}</span>
                  })()}
                </div>

                {/* Points Target Selector */}
                <div className="form-group">
                  <label>Attribution des points</label>
                  <div className="type-selector points-target-selector">
                    <button
                      type="button"
                      className={`type-btn target-btn ${formData.pointsTarget === 'PLAYER' ? 'active' : ''}`}
                      onClick={() => handleInputChange('pointsTarget', 'PLAYER')}
                      title="Points attribues au joueur"
                    >
                      <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
                        <circle cx="12" cy="8" r="4"/>
                        <path d="M12 14c-4 0-8 2-8 4v2h16v-2c0-2-4-4-8-4z"/>
                      </svg>
                      <span>Individuel</span>
                    </button>
                    <button
                      type="button"
                      className={`type-btn target-btn ${formData.pointsTarget === 'TEAM' ? 'active' : ''}`}
                      onClick={() => handleInputChange('pointsTarget', 'TEAM')}
                      title="Points attribues a l'equipe"
                    >
                      <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
                        <circle cx="9" cy="7" r="3"/>
                        <circle cx="15" cy="7" r="3"/>
                        <path d="M9 12c-3 0-6 1.5-6 3v2h12v-2c0-1.5-3-3-6-3z"/>
                        <path d="M15 12c-.5 0-1 .1-1.5.2.8.6 1.5 1.4 1.5 2.3v2.5h6v-2c0-1.5-3-3-6-3z"/>
                      </svg>
                      <span>Equipe</span>
                    </button>
                  </div>
                </div>

                {/* Normal Answer */}
                {(formData.type === 'SPEEDY' || formData.type === 'ARDOISE') && (
                  <div className="form-group">
                    <label htmlFor="answer-input">
                      {formData.type === 'ARDOISE' ? 'Bonne réponse * (animateur)' : 'Reponse *'}
                    </label>
                    <input
                      id="answer-input"
                      type="text"
                      value={formData.answer}
                      onChange={(e) => handleInputChange('answer', e.target.value)}
                      placeholder={formData.type === 'ARDOISE' ? 'Réponse attendue (visible animateur seulement)...' : 'Entrez la reponse...'}
                      required
                    />
                  </div>
                )}

                {/* Note d'explication — animateur seul (v6.4.x, #168).
                    Toujours affichée, quel que soit le type de question :
                    pas de maxLength (longueur non bornée, contrat
                    §EXPLANATION), jamais rendue ailleurs que /anim. */}
                <div className="form-group">
                  <label htmlFor="explanation-input">Note d'explication (animateur seul)</label>
                  <textarea
                    id="explanation-input"
                    value={formData.explanation}
                    onChange={(e) => handleInputChange('explanation', e.target.value)}
                    placeholder="Contexte, anecdote, source... visible uniquement par l'animateur sur /anim"
                    rows={3}
                  />
                </div>

                {/* QCM Answers — extrait dans QcmAnswersEditor (#184/B-F4,
                    commit 1), réutilisé tel quel pour la carte MEMOTION QCM
                    (commit 2) : ni la forme des données ni les callbacks ne
                    changent, comportement strictement identique. */}
                {formData.type === 'QCM' && (
                  <QcmAnswersEditor
                    values={{
                      qcmAnswers: formData.qcmAnswers,
                      qcmCorrect: formData.qcmCorrect,
                      qcmHintsEnabled: formData.qcmHintsEnabled,
                      qcmHintThreshold1: formData.qcmHintThreshold1,
                      qcmHintThreshold2: formData.qcmHintThreshold2,
                      qcmPenalty1: formData.qcmPenalty1,
                      qcmPenalty2: formData.qcmPenalty2,
                    }}
                    onFieldChange={handleInputChange}
                    onAnswerChange={handleQcmAnswerChange}
                  />
                )}

                {/* Memory Pairs Editor */}
                {formData.type === 'MEMORY' && (
                  <div className="memory-section">
                    {/* Memory Mode Selector */}
                    <div className="memory-mode-selector">
                      <label>Mode de jeu</label>
                      <div className="memory-mode-options">
                        <label className="memory-mode-option">
                          <input
                            type="radio"
                            name="memoryMode"
                            value="SOLO"
                            checked={formData.memoryMode === 'SOLO'}
                            onChange={(e) => handleInputChange('memoryMode', e.target.value)}
                          />
                          <span className="memory-mode-label">
                            <strong>SOLO</strong>
                            <small>Une équipe joue seule</small>
                          </span>
                        </label>
                        <label className="memory-mode-option">
                          <input
                            type="radio"
                            name="memoryMode"
                            value="CHACUN_SON_TOUR"
                            checked={formData.memoryMode === 'CHACUN_SON_TOUR'}
                            onChange={(e) => handleInputChange('memoryMode', e.target.value)}
                          />
                          <span className="memory-mode-label">
                            <strong>CHACUN SON TOUR</strong>
                            <small>Rotation après chaque paire (match ou erreur)</small>
                          </span>
                        </label>
                        <label className="memory-mode-option">
                          <input
                            type="radio"
                            name="memoryMode"
                            value="TANT_QUE_JE_GAGNE"
                            checked={formData.memoryMode === 'TANT_QUE_JE_GAGNE'}
                            onChange={(e) => handleInputChange('memoryMode', e.target.value)}
                          />
                          <span className="memory-mode-label">
                            <strong>TANT QUE JE GAGNE</strong>
                            <small>Garde la main si match, passe au suivant si erreur</small>
                          </span>
                        </label>
                      </div>
                    </div>

                    <label>Paires de cartes * ({formData.memoryPairs.length} paires)</label>

                    {/* Pairs List */}
                    <div className="memory-pairs-list">
                      {formData.memoryPairs.map((pair, index) => (
                        <div key={pair.id} className="memory-pair-item">
                          <div className="memory-pair-header">
                            <span className="memory-pair-number">Paire {index + 1}</span>
                            {formData.memoryPairs.length > 2 && (
                              <button
                                type="button"
                                className="memory-remove-btn"
                                onClick={() => handleRemoveMemoryPair(pair.id)}
                                title="Supprimer cette paire"
                              >
                                ×
                              </button>
                            )}
                          </div>
                          <div className="memory-pair-cards">
                            {/* Card 1 */}
                            <div className="memory-card-input">
                              <div className="memory-card-type-toggle">
                                <button
                                  type="button"
                                  className={`toggle-btn ${!pair.card1.isImage ? 'active' : ''}`}
                                  onClick={() => handleMemoryCardChange(pair.id, 'card1', 'type', 'text')}
                                >
                                  Texte
                                </button>
                                <button
                                  type="button"
                                  className={`toggle-btn ${pair.card1.isImage ? 'active' : ''}`}
                                  onClick={() => handleMemoryCardChange(pair.id, 'card1', 'type', 'image')}
                                >
                                  Image
                                </button>
                              </div>
                              {pair.card1.isImage ? (
                                <div className="memory-card-image-input">
                                  {pair.card1.image ? (
                                    <div className="memory-card-image-preview">
                                      <img
                                        src={pair.card1.image instanceof File
                                          ? URL.createObjectURL(pair.card1.image)
                                          : pair.card1.image}
                                        alt="Carte 1"
                                      />
                                      <button
                                        type="button"
                                        className="memory-card-remove-img"
                                        onClick={() => handleMemoryCardChange(pair.id, 'card1', 'image', null)}
                                      >
                                        ×
                                      </button>
                                    </div>
                                  ) : (
                                    <label className="memory-card-upload">
                                      <input
                                        type="file"
                                        accept="image/*"
                                        onChange={(e) => {
                                          const file = e.target.files?.[0]
                                          if (file) handleMemoryCardChange(pair.id, 'card1', 'image', file)
                                        }}
                                      />
                                      <span>+ Image</span>
                                    </label>
                                  )}
                                </div>
                              ) : (
                                <input
                                  type="text"
                                  value={pair.card1.text}
                                  onChange={(e) => handleMemoryCardChange(pair.id, 'card1', 'text', e.target.value)}
                                  placeholder="Texte carte 1..."
                                  className="memory-card-text-input"
                                />
                              )}
                            </div>

                            <span className="memory-pair-arrow">↔</span>

                            {/* Card 2 */}
                            <div className="memory-card-input">
                              <div className="memory-card-type-toggle">
                                <button
                                  type="button"
                                  className={`toggle-btn ${!pair.card2.isImage ? 'active' : ''}`}
                                  onClick={() => handleMemoryCardChange(pair.id, 'card2', 'type', 'text')}
                                >
                                  Texte
                                </button>
                                <button
                                  type="button"
                                  className={`toggle-btn ${pair.card2.isImage ? 'active' : ''}`}
                                  onClick={() => handleMemoryCardChange(pair.id, 'card2', 'type', 'image')}
                                >
                                  Image
                                </button>
                              </div>
                              {pair.card2.isImage ? (
                                <div className="memory-card-image-input">
                                  {pair.card2.image ? (
                                    <div className="memory-card-image-preview">
                                      <img
                                        src={pair.card2.image instanceof File
                                          ? URL.createObjectURL(pair.card2.image)
                                          : pair.card2.image}
                                        alt="Carte 2"
                                      />
                                      <button
                                        type="button"
                                        className="memory-card-remove-img"
                                        onClick={() => handleMemoryCardChange(pair.id, 'card2', 'image', null)}
                                      >
                                        ×
                                      </button>
                                    </div>
                                  ) : (
                                    <label className="memory-card-upload">
                                      <input
                                        type="file"
                                        accept="image/*"
                                        onChange={(e) => {
                                          const file = e.target.files?.[0]
                                          if (file) handleMemoryCardChange(pair.id, 'card2', 'image', file)
                                        }}
                                      />
                                      <span>+ Image</span>
                                    </label>
                                  )}
                                </div>
                              ) : (
                                <input
                                  type="text"
                                  value={pair.card2.text}
                                  onChange={(e) => handleMemoryCardChange(pair.id, 'card2', 'text', e.target.value)}
                                  placeholder="Texte carte 2..."
                                  className="memory-card-text-input"
                                />
                              )}
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>

                    {/* Add Pair Button */}
                    {formData.memoryPairs.length < 12 && (
                      <button
                        type="button"
                        className="memory-add-btn"
                        onClick={handleAddMemoryPair}
                      >
                        + Ajouter une paire
                      </button>
                    )}

                    {/* Memory Preview Grid */}
                    <div className="memory-preview-section">
                      <label>Apercu grille ({formData.memoryPairs.length * 2} cartes)</label>
                      <div
                        className="memory-preview-grid"
                        style={{ '--grid-cols': getMemoryGridColumns(formData.memoryPairs.length) }}
                      >
                        {shuffleArray(formData.memoryPairs.flatMap(pair => [
                          { ...pair.card1, pairId: pair.id, cardNum: 1 },
                          { ...pair.card2, pairId: pair.id, cardNum: 2 },
                        ])).map((card, idx) => (
                          <div key={`${card.pairId}-${card.cardNum}-${idx}`} className="memory-preview-card">
                            {card.isImage && card.image ? (
                              <img
                                src={card.image instanceof File ? URL.createObjectURL(card.image) : card.image}
                                alt=""
                              />
                            ) : card.text ? (
                              <span className="memory-preview-text">{card.text}</span>
                            ) : (
                              <span className="memory-preview-empty">?</span>
                            )}
                          </div>
                        ))}
                      </div>
                    </div>

                    {/* Memory Config */}
                    <div className="memory-config-section">
                      <label>Configuration</label>
                      <div className="memory-config-grid">
                        <div className="memory-config-item">
                          <label>Delai retournement (s)</label>
                          <input
                            type="number"
                            value={formData.memoryConfig.flipDelay}
                            onChange={(e) => handleMemoryConfigChange('flipDelay', parseFloat(e.target.value) || 3)}
                            min="1"
                            max="10"
                            step="0.5"
                          />
                        </div>
                        <div className="memory-config-item">
                          <label>Points par paire</label>
                          <input
                            type="number"
                            value={formData.memoryConfig.pointsPerPair}
                            onChange={(e) => handleMemoryConfigChange('pointsPerPair', parseInt(e.target.value) || 10)}
                            min="1"
                            max="100"
                          />
                        </div>
                        <div className="memory-config-item">
                          <label>Penalite erreur</label>
                          <input
                            type="number"
                            value={formData.memoryConfig.errorPenalty}
                            onChange={(e) => handleMemoryConfigChange('errorPenalty', parseInt(e.target.value) || 0)}
                            min="0"
                            max="50"
                          />
                        </div>
                        <div className="memory-config-item">
                          <label>Bonus completion</label>
                          <input
                            type="number"
                            value={formData.memoryConfig.completionBonus}
                            onChange={(e) => handleMemoryConfigChange('completionBonus', parseInt(e.target.value) || 0)}
                            min="0"
                            max="100"
                          />
                        </div>
                        <div className="memory-config-item">
                          <label>Temps memorisation (s)</label>
                          <input
                            type="number"
                            value={formData.memoryConfig.memorizeTime}
                            onChange={(e) => handleMemoryConfigChange('memorizeTime', parseInt(e.target.value) || 5)}
                            min="1"
                            max="30"
                          />
                        </div>
                        <div className="memory-config-item">
                          <label>Delai reveal (s)</label>
                          <input
                            type="number"
                            value={formData.memoryConfig.revealDelay}
                            onChange={(e) => handleMemoryConfigChange('revealDelay', parseFloat(e.target.value) || 0.5)}
                            min="0.1"
                            max="2"
                            step="0.1"
                          />
                        </div>
                      </div>
                      <div className="memory-config-toggle">
                        <label>
                          <input
                            type="checkbox"
                            checked={formData.memoryConfig.useTimer}
                            onChange={(e) => handleMemoryConfigChange('useTimer', e.target.checked)}
                          />
                          Utiliser le timer global
                        </label>
                        <span className="memory-config-hint">
                          {formData.memoryConfig.useTimer
                            ? 'Le jeu s\'arrete quand le temps est ecoule'
                            : 'Pas de limite de temps, jeu jusqu\'a toutes les paires trouvees'}
                        </span>
                      </div>
                      <div className="memory-config-toggle">
                        <label>
                          <input
                            type="checkbox"
                            checked={formData.memoryConfig.showDuringMemorize}
                            onChange={(e) => handleMemoryConfigChange('showDuringMemorize', e.target.checked)}
                          />
                          Afficher les cartes pendant la memorisation
                        </label>
                        <span className="memory-config-hint">
                          {formData.memoryConfig.showDuringMemorize
                            ? 'Les cartes sont visibles pendant le decompte'
                            : 'Les cartes restent cachees jusqu\'au debut du jeu'}
                        </span>
                      </div>
                    </div>

                    {/* Validation Hint */}
                    {formData.memoryPairs.filter(p => {
                      const c1 = p.card1.isImage ? p.card1.image : p.card1.text
                      const c2 = p.card2.isImage ? p.card2.image : p.card2.text
                      return c1 && c2
                    }).length < 2 && (
                      <p className="memory-hint">Remplissez au moins 2 paires completes</p>
                    )}
                  </div>
                )}

                {/* MEMOTION Cards Editor */}
                {formData.type === 'MEMOTION' && (
                  <div className="memotion-section">
                    {/* Mode Selector */}
                    <div className="memory-mode-selector">
                      <label>Mode de jeu</label>
                      <div className="memory-mode-options">
                        {[
                          { value: 'SOLO', label: 'SOLO', desc: 'Une equipe joue seule' },
                          { value: 'CHACUN_SON_TOUR', label: 'CHACUN SON TOUR', desc: 'Rotation apres chaque carte' },
                          { value: 'TANT_QUE_JE_GAGNE', label: 'TANT QUE JE GAGNE', desc: 'Garde la main si bonne reponse' },
                        ].map(mode => (
                          <label key={mode.value} className="memory-mode-option">
                            <input
                              type="radio"
                              name="motionMode"
                              value={mode.value}
                              checked={formData.motionMode === mode.value}
                              onChange={(e) => handleInputChange('motionMode', e.target.value)}
                            />
                            <span className="memory-mode-label">
                              <strong>{mode.label}</strong>
                              <small>{mode.desc}</small>
                            </span>
                          </label>
                        ))}
                      </div>
                    </div>

                    {/* MEMOTION — Temps + Points par difficulté */}
                    <div className="memotion-points-config">
                      <div className="memotion-time-row">
                        <label htmlFor="memotion-time-input">Temps (s)</label>
                        <input
                          id="memotion-time-input"
                          type="number"
                          value={formData.time}
                          onChange={e => handleInputChange('time', e.target.value)}
                          min="5"
                          max="300"
                        />
                      </div>
                      <label>Points par difficulté</label>
                      <div className="memotion-points-row">
                        <span className="memotion-points-star">★</span>
                        <input
                          type="number"
                          min="1"
                          className="memotion-points-input"
                          value={formData.motionConfig?.points1 ?? 1}
                          onChange={e => setFormData(p => ({ ...p, motionConfig: { ...p.motionConfig, points1: e.target.value } }))}
                        />
                        <span className="memotion-points-star">★★</span>
                        <input
                          type="number"
                          min="1"
                          className="memotion-points-input"
                          value={formData.motionConfig?.points2 ?? 3}
                          onChange={e => setFormData(p => ({ ...p, motionConfig: { ...p.motionConfig, points2: e.target.value } }))}
                        />
                        <span className="memotion-points-star">★★★</span>
                        <input
                          type="number"
                          min="1"
                          className="memotion-points-input"
                          value={formData.motionConfig?.points3 ?? 5}
                          onChange={e => setFormData(p => ({ ...p, motionConfig: { ...p.motionConfig, points3: e.target.value } }))}
                        />
                        <span className="memotion-points-unit">pts</span>
                      </div>
                    </div>

                    {/* MEMOTION — Mode SECRET : durée mémorisation */}
                    <div className="memotion-memorize-row">
                      <label htmlFor="memotion-memorize-input">Mode SECRET — Durée mémorisation (s)</label>
                      <input
                        id="memotion-memorize-input"
                        type="number" min="0" max="120" step="5"
                        value={formData.motionMemorizeDuration ?? 0}
                        onChange={e => handleInputChange('motionMemorizeDuration', parseInt(e.target.value) || 0)}
                      />
                      <span className="memotion-memorize-hint">0 = mode standard</span>
                    </div>

                    <label>Cartes MEMOTION * ({formData.motionCards.length} cartes)</label>

                    {/* Cards List */}
                    <div className="memotion-cards-list">
                      {formData.motionCards.map((card, index) => {
                        const cardType = card.type || 'SPEEDY'
                        const cardLocked = isMotionCardTypeLocked(card)
                        return (
                        <div key={card.id} className="memotion-card-item">
                          <div className="memory-pair-header">
                            <span className="memory-pair-number">Carte {index + 1}</span>
                            {formData.motionCards.length > 2 && (
                              <button
                                type="button"
                                className="memory-remove-btn"
                                onClick={() => handleRemoveMotionCard(card.id)}
                                title="Supprimer cette carte"
                              >
                                ×
                              </button>
                            )}
                          </div>

                          {/* Type de jeu de la carte (#184/B-F4) — sélecteur
                              filtré sur les types nestable (SPEEDY/QCM en
                              v7.0.0, questionTypeMeta.js). Désactivé + raison
                              affichée dès que la carte porte du contenu propre
                              à son type (verrou réactif, contrat §3.2). */}
                          <div className="form-group memotion-card-type-selector">
                            <label>Type de jeu de la carte</label>
                            <div className="type-filter-row">
                              {QUESTION_TYPES.filter(t => t.nestable).map(t => (
                                <button
                                  key={t.key}
                                  type="button"
                                  className={`type-btn ${t.key.toLowerCase()} ${cardType === t.key ? 'active' : ''}`}
                                  disabled={cardLocked}
                                  onClick={() => handleMotionCardChange(card.id, 'type', t.key)}
                                >
                                  {t.label}
                                </button>
                              ))}
                            </div>
                            {cardLocked && (
                              <div className="motion-card-lock-reason">
                                <span>🔒</span>
                                <span>Type verrouillé — {motionCardLockReason(card)}</span>
                              </div>
                            )}
                          </div>

                          {/* RECTO face — commune à tous les types (contrat §3.1),
                              ne verrouille jamais. */}
                          <div className="memotion-face-section">
                            <div className="memotion-face-label memotion-face-recto">RECTO</div>
                            <div className="form-group" style={{ marginBottom: '0.5rem' }}>
                              <input
                                type="text"
                                value={card.rectoTheme}
                                onChange={(e) => handleMotionCardChange(card.id, 'rectoTheme', e.target.value)}
                                placeholder="Theme / Titre..."
                                className="memory-card-text-input"
                                required
                              />
                            </div>
                            {/* Difficulty selector */}
                            <div className="memotion-difficulty-row">
                              <span className="memotion-diff-label">Difficulte :</span>
                              {[1, 2, 3].map(d => (
                                <button
                                  key={d}
                                  type="button"
                                  className={`memotion-diff-btn ${card.difficulty === d ? 'active' : ''}`}
                                  onClick={() => handleMotionCardChange(card.id, 'difficulty', d)}
                                >
                                  {'★'.repeat(d)}
                                </button>
                              ))}
                            </div>
                            {/* Recto image (optional) */}
                            <div className="memotion-img-row">
                              {card.rectoImage ? (
                                <div className="memory-card-image-preview">
                                  <img
                                    src={card.rectoImage instanceof File ? URL.createObjectURL(card.rectoImage) : card.rectoImage}
                                    alt="Recto"
                                  />
                                  <button type="button" className="memory-card-remove-img" onClick={() => handleMotionCardChange(card.id, 'rectoImage', null)}>×</button>
                                </div>
                              ) : (
                                <label className="memory-card-upload">
                                  <input type="file" accept="image/*" onChange={(e) => { const f = e.target.files?.[0]; if (f) handleMotionCardChange(card.id, 'rectoImage', f) }} />
                                  <span>+ Image recto</span>
                                </label>
                              )}
                            </div>
                          </div>

                          {/* VERSO face — énoncé commun à tous les types
                              (contrat §3.1), suivi du sous-éditeur propre au
                              type le cas échéant (emplacement §7.1 — QCM
                              n'ajoute aucune image, seulement ses 4 réponses). */}
                          <div className="memotion-face-section">
                            <div className="memotion-face-label memotion-face-verso">VERSO (Question)</div>
                            <div className="form-group" style={{ marginBottom: '0.5rem' }}>
                              <textarea
                                value={card.questionText}
                                onChange={(e) => handleMotionCardChange(card.id, 'questionText', e.target.value)}
                                placeholder="Texte de la question..."
                                rows={2}
                                className="memory-card-text-input"
                              />
                            </div>
                            <div className="memotion-img-row">
                              {card.questionImage ? (
                                <div className="memory-card-image-preview">
                                  <img
                                    src={card.questionImage instanceof File ? URL.createObjectURL(card.questionImage) : card.questionImage}
                                    alt="Question"
                                  />
                                  <button type="button" className="memory-card-remove-img" onClick={() => handleMotionCardChange(card.id, 'questionImage', null)}>×</button>
                                </div>
                              ) : (
                                <label className="memory-card-upload">
                                  <input type="file" accept="image/*" onChange={(e) => { const f = e.target.files?.[0]; if (f) handleMotionCardChange(card.id, 'questionImage', f) }} />
                                  <span>+ Image question</span>
                                </label>
                              )}
                            </div>
                            {cardType === 'QCM' && (
                              <QcmAnswersEditor
                                values={{
                                  qcmAnswers: card.qcmAnswers,
                                  qcmCorrect: card.qcmCorrect,
                                  qcmHintsEnabled: card.qcmHintsEnabled,
                                  qcmHintThreshold1: card.qcmHintThreshold1,
                                  qcmHintThreshold2: card.qcmHintThreshold2,
                                  qcmPenalty1: card.qcmPenalty1,
                                  qcmPenalty2: card.qcmPenalty2,
                                }}
                                onFieldChange={(field, value) => handleMotionCardChange(card.id, field, value)}
                                onAnswerChange={(color, value) => handleMotionCardChange(
                                  card.id, 'qcmAnswers', { ...card.qcmAnswers, [color]: value }
                                )}
                              />
                            )}
                          </div>

                          {/* REVEAL face — propre à SPEEDY uniquement (contrat
                              §7 : MediaSlots de QCM = recto/question, aucune
                              face reveal). QCM n'a rien à saisir ici : la
                              bonne réponse est déjà désignée ci-dessus. */}
                          {cardType === 'SPEEDY' ? (
                            <div className="memotion-face-section">
                              <div className="memotion-face-label memotion-face-reveal">REVEAL (Reponse)</div>
                              <div className="form-group" style={{ marginBottom: '0.5rem' }}>
                                <textarea
                                  value={card.answerText}
                                  onChange={(e) => handleMotionCardChange(card.id, 'answerText', e.target.value)}
                                  placeholder="Texte de la reponse..."
                                  rows={2}
                                  className="memory-card-text-input"
                                />
                              </div>
                              <div className="memotion-img-row">
                                {card.answerImage ? (
                                  <div className="memory-card-image-preview">
                                    <img
                                      src={card.answerImage instanceof File ? URL.createObjectURL(card.answerImage) : card.answerImage}
                                      alt="Reponse"
                                    />
                                    <button type="button" className="memory-card-remove-img" onClick={() => handleMotionCardChange(card.id, 'answerImage', null)}>×</button>
                                  </div>
                                ) : (
                                  <label className="memory-card-upload">
                                    <input type="file" accept="image/*" onChange={(e) => { const f = e.target.files?.[0]; if (f) handleMotionCardChange(card.id, 'answerImage', f) }} />
                                    <span>+ Image reponse</span>
                                  </label>
                                )}
                              </div>
                            </div>
                          ) : (
                            <div className="memotion-face-section">
                              <div className="memotion-face-label memotion-face-reveal">REVEAL (Reponse)</div>
                              <p className="memotion-card-no-reveal-hint">
                                Pas de face de réponse à saisir : la bonne réponse est la proposition cochée ci-dessus.
                              </p>
                            </div>
                          )}
                        </div>
                        )
                      })}
                    </div>

                    {formData.motionCards.length < 12 && (
                      <button type="button" className="memory-add-btn" onClick={handleAddMotionCard}>
                        + Ajouter une carte
                      </button>
                    )}

                    {formData.motionCards.filter(c => c.rectoTheme.trim()).length < 2 && (
                      <p className="memory-hint">Remplissez le theme d'au moins 2 cartes</p>
                    )}
                  </div>
                )}

                {/* ARDOISE Keyboard Selector */}
                {formData.type === 'ARDOISE' && (
                  <div className="ardoise-section">
                    <div className="form-group">
                      <label>Type de clavier</label>
                      <div className="ardoise-keyboard-selector">
                        <button
                          type="button"
                          className={`ardoise-keyboard-btn ${formData.ardoiseKeyboardType === 'AZERTY' ? 'active' : ''}`}
                          onClick={() => handleInputChange('ardoiseKeyboardType', 'AZERTY')}
                        >
                          <span className="ardoise-keyboard-icon">⌨️</span>
                          <span className="ardoise-keyboard-label">AZERTY</span>
                          <span className="ardoise-keyboard-desc">Clavier texte complet</span>
                        </button>
                        <button
                          type="button"
                          className={`ardoise-keyboard-btn ${formData.ardoiseKeyboardType === 'NUMPAD' ? 'active' : ''}`}
                          onClick={() => handleInputChange('ardoiseKeyboardType', 'NUMPAD')}
                        >
                          <span className="ardoise-keyboard-icon">🔢</span>
                          <span className="ardoise-keyboard-label">Pavé numérique</span>
                          <span className="ardoise-keyboard-desc">Chiffres uniquement</span>
                        </button>
                      </div>
                    </div>
                    <p className="ardoise-info">
                      💡 La bonne réponse est stockée et visible uniquement par l'animateur lors de la révélation.
                    </p>
                  </div>
                )}

                <div className="form-row">
                  {/* Hide Points for MEMORY and MEMOTION - calculated per pair/card */}
                  {formData.type !== 'MEMORY' && formData.type !== 'MEMOTION' && (
                    <div className="form-group">
                      <label htmlFor="points-input">Points</label>
                      <input
                        id="points-input"
                        type="number"
                        value={formData.points}
                        onChange={(e) => handleInputChange('points', e.target.value)}
                        min="1"
                        max="100"
                      />
                    </div>
                  )}
                  {formData.type !== 'MEMOTION' && (
                    <div className="form-group">
                      <label htmlFor="time-input">Temps (s)</label>
                      <input
                        id="time-input"
                        type="number"
                        value={formData.time}
                        onChange={(e) => handleInputChange('time', e.target.value)}
                        min="5"
                        max="300"
                      />
                    </div>
                  )}
                </div>

                {/* Hide Image question/answer for MEMORY/MEMOTION only — ARDOISE supports images (#94) */}
                {formData.type !== 'MEMORY' && formData.type !== 'MEMOTION' && (
                  <>
                    <div className="form-group">
                      <label htmlFor="media-input">Image question (optionnel)</label>
                      <input
                        id="media-input"
                        type="file"
                        ref={fileInputRef}
                        onChange={handleFileChange}
                        accept="image/*"
                      />
                      {(formData.media || formData.existingMedia) && (
                        <div className="media-preview">
                          <img
                            src={formData.media ? URL.createObjectURL(formData.media) : formData.existingMedia}
                            alt="Preview"
                          />
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => {
                              setFormData(prev => ({ ...prev, media: null, existingMedia: null }))
                              if (fileInputRef.current) fileInputRef.current.value = ''
                            }}
                          >
                            Supprimer
                          </Button>
                        </div>
                      )}
                    </div>

                    <div className="form-group">
                      <label htmlFor="media-answer-input">Image reponse (optionnel)</label>
                      <input
                        id="media-answer-input"
                        type="file"
                        ref={fileAnswerInputRef}
                        onChange={handleFileAnswerChange}
                        accept="image/*"
                      />
                      {(formData.mediaAnswer || formData.existingMediaAnswer) && (
                        <div className="media-preview media-preview-answer">
                          <img
                            src={formData.mediaAnswer ? URL.createObjectURL(formData.mediaAnswer) : formData.existingMediaAnswer}
                            alt="Preview reponse"
                          />
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => {
                              setFormData(prev => ({ ...prev, mediaAnswer: null, existingMediaAnswer: null }))
                              if (fileAnswerInputRef.current) fileAnswerInputRef.current.value = ''
                            }}
                          >
                            Supprimer
                          </Button>
                        </div>
                      )}
                    </div>
                  </>
                )}

                <Button
                  type="submit"
                  variant={editingId ? 'primary' : 'fun'}
                  size="lg"
                  loading={isUploading}
                  className="submit-btn"
                >
                  {editingId ? 'Modifier' : 'Ajouter'}
                </Button>

                {/* Storage indicator */}
                <div className="storage-mini">
                  <div className="storage-bar-mini">
                    <motion.div
                      className="storage-fill-mini"
                      initial={{ width: 0 }}
                      animate={{ width: `${storagePercent}%` }}
                      style={{
                        backgroundColor: storagePercent > 80 ? 'var(--error)' :
                          storagePercent > 60 ? 'var(--warning)' : 'var(--success)'
                      }}
                    />
                  </div>
                  <span className="storage-text">
                    {storagePercent.toFixed(1)}% - {fsInfo ? `${(parseInt(fsInfo.FREE) / 1024 / 1024).toFixed(1)} MB libres` : ''}
                  </span>
                </div>
              </form>
            </CardBody>
          </Card>
        </aside>
      </div>

      {showAIModal && (
        <AIGenerateModal
          onClose={() => setShowAIModal(false)}
          apiKeyConfigured={providerConfigured}
          provider={aiConfig.provider}
          categories={apiCategories}
          // T2.5 — la modale lit gameState.quiz*, jamais l'état local du
          // formulaire ci-dessus : la génération doit utiliser ce qui est
          // réellement enregistré et diffusé (cohérent avec l'écran TV).
          quizTheme={gameState.quizTheme}
          quizPopulations={gameState.quizPopulations}
          quizDifficulties={gameState.quizDifficulties}
          quizLanguage={gameState.quizLanguage}
          quizObjectives={gameState.quizObjectives}
          hasUnsavedQuizChanges={quizFormDiverged}
          questions={questions}
          aiJob={aiJob}
          onCancelGeneration={cancelAiGeneration}
          interBatchDelayMs={aiConfig.interBatchDelayMs}
          maxConsecutiveFailures={aiConfig.maxConsecutiveFailures}
          onGenerated={handleAIGenerated}
          onNavigateToQuizSettings={handleNavigateToQuizSettings}
        />
      )}

      {aiToast && (
        <div className={`wifi-toast wifi-toast-${aiToast.type}`}>
          {aiToast.message}
        </div>
      )}

      {shuffleToast && (
        <div className={`wifi-toast wifi-toast-${shuffleToast.type}`}>
          {shuffleToast.message}
        </div>
      )}

      {entracteImageToast && (
        <div className={`wifi-toast wifi-toast-${entracteImageToast.type}`}>
          {entracteImageToast.message}
        </div>
      )}
    </div>
  )
}
