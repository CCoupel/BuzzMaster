import { useState, useRef, useMemo, useEffect } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { AnimatePresence, motion } from 'framer-motion'
import { useGame } from '../hooks/GameContext'
import { useCategoryFilter } from '../hooks/useCategoryFilter'
import { useCategories } from '../hooks/useCategories'
import { CATEGORIES } from '../utils/categoryUtils'
import { sortQuestionsByOrder, shuffleArray } from '../utils/questionOrder'
import { QUESTION_TYPES } from '../utils/questionTypeMeta'
import { isMotionCardTypeLocked, motionCardLockReason } from '../utils/motionCardLock'
import Button from '../components/Button'
import Card, { CardHeader, CardBody } from '../components/Card'
import CategoryBalance from '../components/CategoryBalance'
import CategorySelector from '../components/CategorySelector'
import CategoryFilterBar from '../components/CategoryFilterBar'
import QuestionCard from '../components/QuestionCard'
import AIGenerateModal from '../components/AIGenerateModal'
import QcmAnswersEditor from '../components/QcmAnswersEditor'
import MotionCardMemoryEditor from '../components/MotionCardMemoryEditor'
import RafalePoolAlert from '../components/RafalePoolAlert'
import RafalePage from './RafalePage'
import './QuestionsPage.css'
import './ConfigPage.css'
import '../styles/sliders.css'
import '../styles/tabs.css'

// Re-export CATEGORIES for backward compatibility
export { CATEGORIES }

export default function QuestionsPage() {
  const { questions, fsInfo, deleteQuestion, sendMessage, gameState, aiJob, cancelAiGeneration } = useGame()
  const navigate = useNavigate()
  // #215 — 2 onglets (Questions/Rafale). `/admin/rafale` redirige désormais
  // vers `/admin/quiz?tab=rafale` (App.jsx) pour préserver les favoris —
  // lu ici en état initial uniquement (pas de synchronisation continue avec
  // l'URL au clic sur un onglet : même patron que BackstagePage.jsx).
  const [searchParams] = useSearchParams()
  const [activeTab, setActiveTab] = useState(searchParams.get('tab') === 'rafale' ? 'rafale' : 'questions')
  const [isUploading, setIsUploading] = useState(false)
  const [editingId, setEditingId] = useState(null)
  const fileInputRef = useRef(null)
  const fileAnswerInputRef = useRef(null)
  const [draggedId, setDraggedId] = useState(null)
  const [dragOverId, setDragOverId] = useState(null)

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
    // RAFALE fields (v8.0.0, #16/#107, contrat rafale.md §3.3) — pas un
    // énoncé/réponse, une CONFIGURATION de manche : difficulté unique
    // (RAFALE_DIFFICULTY), mode (RAFALE_MODE), temps par question
    // (RAFALE_QUESTION_TIME, défaut 3s), plafond dur (RAFALE_MAX_QUESTIONS,
    // défaut 100, max 100). TIME/POINTS/CATEGORY réutilisent les champs
    // génériques déjà présents (durée de manche / barème d'une bonne
    // réponse / catégorie du filtre de pioche) — bugfix 2026-08-29 :
    // CATEGORY est désormais unique, comme tous les autres types (l'ancien
    // RAFALE_CATEGORIES multi est retiré, contrat §3.3).
    rafaleDifficulty: 1,
    rafaleMode: 'SOLO',
    rafaleQuestionTime: 3,
    rafaleMaxQuestions: 100,
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
        // #187 — OwnedFields MEMORY, valeurs de création (contrat §3.2,
        // DOIVENT rester synchronisées avec utils/motionCardLock.js
        // MEMORY_MODE_CREATION_VALUE/MEMORY_CONFIG_CREATION_VALUES).
        memoryMode: 'SOLO',
        memoryPairs: [
          { id: 1, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } },
          { id: 2, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } },
        ],
        memoryConfig: {
          flipDelay: 3, pointsPerPair: 10, errorPenalty: 0, completionBonus: 0,
          useTimer: true, memorizeTime: 5, showDuringMemorize: true, revealDelay: 0.5,
        },
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
        // #187 — voir commentaire OwnedFields MEMORY sur la carte mc-1 ci-dessus.
        memoryMode: 'SOLO',
        memoryPairs: [
          { id: 1, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } },
          { id: 2, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } },
        ],
        memoryConfig: {
          flipDelay: 3, pointsPerPair: 10, errorPenalty: 0, completionBonus: 0,
          useTimer: true, memorizeTime: 5, showDuringMemorize: true, revealDelay: 0.5,
        },
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

  // #137 — le bouton "✨ Générer via IA" s'active selon la clé du provider
  // ACTUELLEMENT sélectionné (maquette 137 §7), pas "si n'importe lequel en
  // a une". Reste cliquable si un job tourne déjà, pour permettre le
  // ré-attachement/l'annulation même si la clé a depuis été retirée.
  const providerConfigured = aiConfig.provider === 'groq' ? aiConfig.groqApiKeyConfigured : aiConfig.apiKeyConfigured
  const canOpenAiModal = providerConfigured || aiJob?.state === 'RUNNING'

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

  // #215 — le lien "configurer le quiz" de la modale IA naviguait auparavant
  // par scrollIntoView vers la section Quiz (autrefois sur cette même page).
  // Cette section vit désormais sur /admin/backstage (BackstagePage.jsx) :
  // une vraie navigation inter-page est donc nécessaire, pas un défilement.
  const handleNavigateToQuizSettings = () => {
    setShowAIModal(false)
    navigate('/admin/backstage')
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
        // QCM, MEMORY, MEMOTION, ARDOISE and RAFALE default to TEAM, SPEEDY defaults to PLAYER
        updates.pointsTarget = (value === 'QCM' || value === 'MEMORY' || value === 'MEMOTION' || value === 'ARDOISE' || value === 'RAFALE') ? 'TEAM' : 'PLAYER'
        // RAFALE (v8.0.0, #16, contrat rafale.md §3.3) — défauts distincts
        // du reste des types : durée de MANCHE (120s, pas 30s) et barème
        // de 2pts/bonne réponse (maquette rafale-v8.html §2). Uniquement
        // appliqué en ENTRANT dans RAFALE, jamais en sortant (une valeur
        // déjà saisie par l'admin pour un autre type n'est pas écrasée par
        // erreur si l'admin revient sur RAFALE après l'avoir quitté —
        // repris à l'identique à chaque sélection, cohérent avec le fait
        // que ces deux champs sont remis à zéro par handleNewQuestion/
        // chargés depuis la question par handleQuestionClick de toute façon).
        if (value === 'RAFALE') {
          updates.time = '120'
          updates.points = '2'
        }
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
    const defaultTarget = (qType === 'QCM' || qType === 'MEMORY' || qType === 'ARDOISE' || qType === 'RAFALE') ? 'TEAM' : 'PLAYER'

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
        // #187 — OwnedFields MEMORY, valeurs de création (contrat §3.2,
        // DOIVENT rester synchronisées avec utils/motionCardLock.js
        // MEMORY_MODE_CREATION_VALUE/MEMORY_CONFIG_CREATION_VALUES).
        memoryMode: 'SOLO',
        memoryPairs: [
          { id: 1, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } },
          { id: 2, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } },
        ],
        memoryConfig: {
          flipDelay: 3, pointsPerPair: 10, errorPenalty: 0, completionBonus: 0,
          useTimer: true, memorizeTime: 5, showDuringMemorize: true, revealDelay: 0.5,
        },
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
        // #187 — voir commentaire OwnedFields MEMORY sur la carte mc-1 ci-dessus.
        memoryMode: 'SOLO',
        memoryPairs: [
          { id: 1, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } },
          { id: 2, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } },
        ],
        memoryConfig: {
          flipDelay: 3, pointsPerPair: 10, errorPenalty: 0, completionBonus: 0,
          useTimer: true, memorizeTime: 5, showDuringMemorize: true, revealDelay: 0.5,
        },
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
        // #187 — MEMORY : mêmes défauts de création que les autres sites
        // (synchronisés avec motionCardLock.js, MEMORY_MODE_CREATION_VALUE/
        // MEMORY_CONFIG_CREATION_VALUES). MEMORY_PAIRS d'une carte suit le
        // même format que MEMORY_PAIRS d'une question (contrat §2, TypedContent
        // partagé) — même mapping que `memoryPairs` ci-dessus (question-scopé).
        memoryMode: card.MEMORY_MODE || 'SOLO',
        memoryPairs: (card.MEMORY_PAIRS && Array.isArray(card.MEMORY_PAIRS))
          ? card.MEMORY_PAIRS.map(pair => ({
              id: pair.ID,
              card1: { text: pair.CARD1?.TEXT || '', image: pair.CARD1?.IMAGE || null, isImage: pair.CARD1?.IS_IMAGE || false },
              card2: { text: pair.CARD2?.TEXT || '', image: pair.CARD2?.IMAGE || null, isImage: pair.CARD2?.IS_IMAGE || false },
            }))
          : [
              { id: 1, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } },
              { id: 2, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } },
            ],
        memoryConfig: card.MEMORY_CONFIG
          ? {
              flipDelay: card.MEMORY_CONFIG.FLIP_DELAY ?? 3,
              pointsPerPair: card.MEMORY_CONFIG.POINTS_PER_PAIR ?? 10,
              errorPenalty: card.MEMORY_CONFIG.ERROR_PENALTY ?? 0,
              completionBonus: card.MEMORY_CONFIG.COMPLETION_BONUS ?? 0,
              useTimer: card.MEMORY_CONFIG.USE_TIMER !== false,
              memorizeTime: card.MEMORY_CONFIG.MEMORIZE_TIME ?? 5,
              showDuringMemorize: card.MEMORY_CONFIG.SHOW_DURING_MEMORIZE !== false,
              revealDelay: card.MEMORY_CONFIG.REVEAL_DELAY ?? 0.5,
            }
          : { flipDelay: 3, pointsPerPair: 10, errorPenalty: 0, completionBonus: 0, useTimer: true, memorizeTime: 5, showDuringMemorize: true, revealDelay: 0.5 },
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
      // RAFALE fields (v8.0.0, #16/#107, contrat rafale.md §3.3) — CATEGORY
      // (générique, ligne `category:` ci-dessus) porte désormais le filtre
      // de manche, comme tous les autres types (bugfix 2026-08-29).
      rafaleDifficulty: question.RAFALE_DIFFICULTY || 1,
      rafaleMode: question.RAFALE_MODE || 'SOLO',
      rafaleQuestionTime: question.RAFALE_QUESTION_TIME || 3,
      rafaleMaxQuestions: question.RAFALE_MAX_QUESTIONS || 100,
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
        // #187 — OwnedFields MEMORY, valeurs de création (contrat §3.2,
        // DOIVENT rester synchronisées avec utils/motionCardLock.js
        // MEMORY_MODE_CREATION_VALUE/MEMORY_CONFIG_CREATION_VALUES).
        memoryMode: 'SOLO',
        memoryPairs: [
          { id: 1, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } },
          { id: 2, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } },
        ],
        memoryConfig: {
          flipDelay: 3, pointsPerPair: 10, errorPenalty: 0, completionBonus: 0,
          useTimer: true, memorizeTime: 5, showDuringMemorize: true, revealDelay: 0.5,
        },
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
        // #187 — voir commentaire OwnedFields MEMORY sur la carte mc-1 ci-dessus.
        memoryMode: 'SOLO',
        memoryPairs: [
          { id: 1, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } },
          { id: 2, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } },
        ],
        memoryConfig: {
          flipDelay: 3, pointsPerPair: 10, errorPenalty: 0, completionBonus: 0,
          useTimer: true, memorizeTime: 5, showDuringMemorize: true, revealDelay: 0.5,
        },
      },
      ],
      motionConfig: { points1: 1, points2: 3, points3: 5 },
      motionMemorizeDuration: 0,
      // ARDOISE fields
      ardoiseKeyboardType: 'AZERTY',
      // RAFALE fields (v8.0.0, #16/#107) — voir commentaire de l'état
      // initial (useState ci-dessus) pour le détail des champs.
      rafaleDifficulty: 1,
      rafaleMode: 'SOLO',
      rafaleQuestionTime: 3,
      rafaleMaxQuestions: 100,
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
        // #187 — voir commentaire OwnedFields MEMORY, useState initial (mc-1).
        memoryMode: 'SOLO',
        memoryPairs: [
          { id: 1, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } },
          { id: 2, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } },
        ],
        memoryConfig: {
          flipDelay: 3, pointsPerPair: 10, errorPenalty: 0, completionBonus: 0,
          useTimer: true, memorizeTime: 5, showDuringMemorize: true, revealDelay: 0.5,
        },
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

  // #187 — sous-éditeur MEMORY d'une carte MEMOTION. Même mécanique que les
  // handlers `handleAddMemoryPair`/`handleRemoveMemoryPair`/
  // `handleMemoryCardChange` (hôte question, ci-dessus), mais scopée à la
  // carte `cardId` — pas de handler générique partagé (le state de la carte
  // vit sous `motionCards[i].memoryPairs`, pas au niveau racine de `formData`).
  const handleAddMotionCardMemoryPair = (cardId) => {
    setFormData(prev => ({
      ...prev,
      motionCards: prev.motionCards.map(c => {
        if (c.id !== cardId) return c
        const pairs = c.memoryPairs || []
        if (pairs.length >= 12) return c // Max 12 paires, même borne que l'hôte question
        const maxId = Math.max(...pairs.map(p => p.id), 0)
        return {
          ...c,
          memoryPairs: [
            ...pairs,
            { id: maxId + 1, card1: { text: '', image: null, isImage: false }, card2: { text: '', image: null, isImage: false } },
          ],
        }
      }),
    }))
  }

  const handleRemoveMotionCardMemoryPair = (cardId, pairId) => {
    setFormData(prev => ({
      ...prev,
      motionCards: prev.motionCards.map(c => {
        if (c.id !== cardId) return c
        const pairs = c.memoryPairs || []
        if (pairs.length <= 2) return c // Min 2 paires, même borne que l'hôte question
        return { ...c, memoryPairs: pairs.filter(p => p.id !== pairId) }
      }),
    }))
  }

  const handleMotionCardMemoryCardChange = (cardId, pairId, cardKey, field, value) => {
    setFormData(prev => ({
      ...prev,
      motionCards: prev.motionCards.map(c => {
        if (c.id !== cardId) return c
        return {
          ...c,
          memoryPairs: (c.memoryPairs || []).map(pair => {
            if (pair.id !== pairId) return pair
            const cardSide = { ...pair[cardKey] }
            if (field === 'type') {
              cardSide.isImage = value === 'image'
              if (cardSide.isImage) cardSide.text = ''
              else cardSide.image = null
            } else if (field === 'text') {
              cardSide.text = value
            } else if (field === 'image') {
              cardSide.image = value
            }
            return { ...pair, [cardKey]: cardSide }
          }),
        }
      }),
    }))
  }

  const handleMotionCardMemoryConfigChange = (cardId, field, value) => {
    setFormData(prev => ({
      ...prev,
      motionCards: prev.motionCards.map(c => c.id !== cardId ? c : {
        ...c,
        memoryConfig: { ...c.memoryConfig, [field]: value },
      }),
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
    if (formData.type === 'RAFALE' && !formData.category) {
      // code-review-20260829-163049.md [MAJEUR] — CATEGORY est OPTIONNEL
      // pour tous les autres types (purement cosmétique, ignoré du moteur),
      // mais FONCTIONNELLEMENT REQUIS pour RAFALE : c'est le filtre de
      // pioche du réservoir (contrat §7, `q.CATEGORY == CATEGORY`).
      // L'enregistrer vide garantirait un pool vide (ErrRafalePoolEmpty)
      // dès le lancement — défense en profondeur, en plus du blocage côté
      // GamePage.jsx (rafaleBlocked). Même style que la validation
      // MEMORY/MEMOTION ci-dessus (retour silencieux, aucun POST envoyé).
      return
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
        if (cardType === 'MEMORY') {
          // #187 — MEMORY_MODE toujours "SOLO" : jamais exposé dans
          // l'éditeur de carte (ignoré par le moteur en contexte carte,
          // contrat §6.3 — une seule équipe, celle de la manche MEMOTION en
          // cours), donc jamais écarté de sa valeur de création côté client.
          // Pas de POINTS_RULE ici : le serveur applique STARS_PRORATA par
          // défaut pour une carte MEMORY sans réglage explicite (contrat
          // §6.3) — cette structure n'est pas encore modélisée côté JS
          // (même état que pour SPEEDY/QCM, voir motionCardLock.js).
          return {
            ...base,
            MEMORY_MODE: 'SOLO',
            MEMORY_PAIRS: (card.memoryPairs || []).map(pair => ({
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
            })),
            // Les trois réglages de points (POINTS_PER_PAIR/ERROR_PENALTY/
            // COMPLETION_BONUS) sont envoyés pour la cohérence du format
            // (même structure MEMORY_CONFIG qu'en hôte question) mais SANS
            // AUCUNE AUTORITÉ en contexte carte (contrat §6.1/§6.3) — le
            // moteur ne les lit jamais au moment de créditer une carte.
            MEMORY_CONFIG: {
              FLIP_DELAY: card.memoryConfig?.flipDelay ?? 3,
              POINTS_PER_PAIR: card.memoryConfig?.pointsPerPair ?? 10,
              ERROR_PENALTY: card.memoryConfig?.errorPenalty ?? 0,
              COMPLETION_BONUS: card.memoryConfig?.completionBonus ?? 0,
              USE_TIMER: card.memoryConfig?.useTimer ?? true,
              MEMORIZE_TIME: card.memoryConfig?.memorizeTime ?? 5,
              SHOW_DURING_MEMORIZE: card.memoryConfig?.showDuringMemorize ?? true,
              REVEAL_DELAY: card.memoryConfig?.revealDelay ?? 0.5,
            },
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
        // #187 — images de paires MEMORY. MediaSlots d'une carte MEMORY =
        // "recto + N paires" (contrat §7, aucun slot `question`/`answer`) :
        // convention retenue pour le nom de champ, coordonnée avec
        // dev-backend (`handleUploadQuestion` doit lire les mêmes clés) —
        // motion_card_<cardID>_pair_<pairID>_1 / _2, extension du patron
        // `motion_card_<cardID>_<slot>` du contrat avec un slot dynamique
        // par paire (même esprit que `memory_card_<pairID>_1/2` question-scopé).
        if ((card.type || 'SPEEDY') === 'MEMORY') {
          ;(card.memoryPairs || []).forEach(pair => {
            if (pair.card1.image instanceof File) data.append(`motion_card_${card.id}_pair_${pair.id}_1`, pair.card1.image)
            if (pair.card2.image instanceof File) data.append(`motion_card_${card.id}_pair_${pair.id}_2`, pair.card2.image)
          })
        }
      })

      // Set answer to number of cards for display
      data.append('answer', `${formData.motionCards.length} cartes`)
    } else if (formData.type === 'RAFALE') {
      // RAFALE mode — configuration de manche (contrat rafale.md §3.3),
      // aucun énoncé/réponse propre (les questions viennent du réservoir,
      // /admin/rafale). `category` (singulier) EST utilisé par ce type
      // depuis le bugfix 2026-08-29 — comme tous les autres types, déjà
      // envoyé ci-dessus (`if (formData.category) { data.append('category', ...) }`),
      // jamais ici. L'ancien RAFALE_CATEGORIES (multi) est retiré.
      data.append('RAFALE_DIFFICULTY', String(formData.rafaleDifficulty))
      data.append('RAFALE_MODE', formData.rafaleMode)
      data.append('RAFALE_QUESTION_TIME', String(formData.rafaleQuestionTime))
      data.append('RAFALE_MAX_QUESTIONS', String(formData.rafaleMaxQuestions))
      // Réponse d'affichage dans la liste des questions (patron MEMORY/
      // MEMOTION ci-dessus — `answer` reste un champ purement informatif
      // pour QuestionCard.jsx, jamais lu par le moteur RAFALE).
      data.append('answer', `${formData.category || '?'} - ${'★'.repeat(formData.rafaleDifficulty)}`)
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
        {activeTab === 'questions' && (
          <p className="page-subtitle">
            {filteredQuestions.length !== sortedQuestions.length
              ? `${filteredQuestions.length} / ${sortedQuestions.length} questions`
              : `${sortedQuestions.length} questions disponibles`}
          </p>
        )}
      </header>

      {/* #215 — page en 2 onglets : Questions (définition du contenu du jeu,
          inchangée) et Rafale (accueille le réservoir de RafalePage.jsx, qui
          avait sa propre route /admin/rafale — désormais conservée en
          redirection vers cet onglet, App.jsx). Les zones Quiz/Entracte/Fonds
          d'écran ont déménagé vers /admin/backstage (BackstagePage.jsx). */}
      <div className="page-tabs" role="tablist">
        <button
          type="button"
          role="tab"
          aria-selected={activeTab === 'questions'}
          className={`page-tab ${activeTab === 'questions' ? 'active' : ''}`}
          onClick={() => setActiveTab('questions')}
        >
          Questions
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={activeTab === 'rafale'}
          className={`page-tab ${activeTab === 'rafale' ? 'active' : ''}`}
          onClick={() => setActiveTab('rafale')}
        >
          Rafale
        </button>
      </div>

      {activeTab === 'rafale' ? (
        <RafalePage />
      ) : (
        <>

      {/* Category Balance + Filters — div unique pour alignement parfait */}
      <div className="category-filter-group">
        <CategoryBalance questions={sortedQuestions} />

        {/* Category filter bar (#40) — CategoryFilterBar.jsx (v8.0.0,
            #16/#197, bugfix cohérence UI/mutualisation, code-review
            20260829-131404 MAJEUR 2) : composant partagé avec
            RafalePage.jsx, plus de copie inline divergente ici. */}
        <CategoryFilterBar
          availableCategories={availableCategories}
          selectedCategories={selectedCategories}
          customCategories={customCategories}
          onToggle={toggleCategoryFilter}
          onClear={clearCategoryFilters}
        />
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

                {/* Category Selector — CategorySelector.jsx (v8.0.0, #16/#197,
                    bugfix cohérence UI), extrait ici (#95/#97/#100), aussi
                    utilisé par RafalePage.jsx — un seul composant, plus de
                    variante dupliquée. RAFALE utilise désormais ce MEME
                    sélecteur (bugfix 2026-08-29, contrat §3.3) : CATEGORY
                    est une catégorie unique pour ce type comme pour tous
                    les autres, l'ancien multi-sélecteur RAFALE_CATEGORIES
                    est retiré (plus de branche dédiée). */}
                <div className="form-group">
                  <label>Categorie</label>
                  <CategorySelector
                    value={formData.category}
                    onChange={(key) => handleInputChange('category', key)}
                    customCategories={customCategories}
                    onRefetchCategories={refetchCategories}
                  />
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
                              n'ajoute aucune image, seulement ses 4 réponses).
                              #187 — MEMORY n'a PAS de MediaSlot `question`
                              (contrat §7 : "recto + N paires") : l'upload
                              d'image question est masqué pour ce type, le
                              texte reste disponible (champ commun à la carte,
                              §3.1) comme consigne optionnelle au-dessus de la
                              grille. */}
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
                            {cardType !== 'MEMORY' && (
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
                            )}
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
                            {cardType === 'MEMORY' && (
                              <MotionCardMemoryEditor
                                card={card}
                                onAddPair={() => handleAddMotionCardMemoryPair(card.id)}
                                onRemovePair={(pairId) => handleRemoveMotionCardMemoryPair(card.id, pairId)}
                                onCardChange={(pairId, cardKey, field, value) => handleMotionCardMemoryCardChange(card.id, pairId, cardKey, field, value)}
                                onConfigChange={(field, value) => handleMotionCardMemoryConfigChange(card.id, field, value)}
                              />
                            )}
                          </div>

                          {/* REVEAL face — propre à SPEEDY uniquement (contrat
                              §7 : MediaSlots de QCM/MEMORY n'en déclarent
                              aucune, aucune face reveal). QCM n'a rien à
                              saisir ici : la bonne réponse est déjà désignée
                              ci-dessus. MEMORY (#187) : la grille EST la
                              carte, il n'y a rien à révéler séparément — le
                              serveur affiche simplement toutes les paires. */}
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
                                {cardType === 'MEMORY'
                                  ? 'Pas de face de réponse à saisir : la grille de paires ci-dessus est déjà la carte.'
                                  : 'Pas de face de réponse à saisir : la bonne réponse est la proposition cochée ci-dessus.'}
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

                {/* RAFALE — configuration de manche (v8.0.0, #16/#107,
                    contrat rafale.md §3.3, tâche 26 du plan). "Points"/
                    "Temps (s)" ci-dessous (form-row générique) portent ici
                    le BARÈME (points par bonne réponse) et la DURÉE DE
                    MANCHE — mêmes champs génériques que les autres types
                    (TIME/POINTS réutilisés tel quel, contrat §3.3). */}
                {formData.type === 'RAFALE' && (
                  <div className="rafale-section">
                    {/* Categorie — bugfix 2026-08-29 (contrat §3.3) : retire
                        le multi-selecteur RAFALE_CATEGORIES, RAFALE utilise
                        desormais le CategorySelector generique ci-dessus
                        (formData.category, comme tous les autres types). */}
                    <div className="form-group">
                      <label>Difficulte (une seule par manche)</label>
                      <div className="memotion-difficulty-row">
                        {[1, 2, 3].map(d => (
                          <button
                            key={d}
                            type="button"
                            className={`memotion-diff-btn ${formData.rafaleDifficulty === d ? 'active' : ''}`}
                            onClick={() => handleInputChange('rafaleDifficulty', d)}
                          >
                            {'★'.repeat(d)}
                          </button>
                        ))}
                      </div>
                    </div>

                    {/* Mode Selector — meme patron que MEMORY/MEMOTION
                        ci-dessus (v8.0.0, #16/#199, bugfix cohérence UI) :
                        .memory-mode-selector/-options/-option, radios,
                        reutilises tels quels (aucune nouvelle classe). */}
                    <div className="memory-mode-selector">
                      <label>Mode de jeu</label>
                      <div className="memory-mode-options">
                        {[
                          { value: 'SOLO', label: 'SOLO', desc: 'Une equipe joue seule' },
                          { value: 'CHACUN_SON_TOUR', label: 'CHACUN SON TOUR', desc: 'Rotation apres chaque reponse (juste ou fausse)' },
                          { value: 'TANT_QUE_JE_GAGNE', label: 'TANT QUE JE GAGNE', desc: 'Garde la main si bonne reponse' },
                          { value: 'MAILLON_FAIBLE', label: 'MAILLON FAIBLE', desc: 'Compteur remis a 0 sur erreur, meilleur score memorise' },
                        ].map(mode => (
                          <label key={mode.value} className="memory-mode-option">
                            <input
                              type="radio"
                              name="rafaleMode"
                              value={mode.value}
                              checked={formData.rafaleMode === mode.value}
                              onChange={(e) => handleInputChange('rafaleMode', e.target.value)}
                            />
                            <span className="memory-mode-label">
                              <strong>{mode.label}</strong>
                              <small>{mode.desc}</small>
                            </span>
                          </label>
                        ))}
                      </div>
                    </div>

                    <div className="form-row">
                      <div className="form-group">
                        <label htmlFor="rafale-question-time-input">Temps par question (s)</label>
                        <input
                          id="rafale-question-time-input"
                          type="number"
                          value={formData.rafaleQuestionTime}
                          onChange={(e) => handleInputChange('rafaleQuestionTime', parseInt(e.target.value) || 1)}
                          min="1"
                          max="30"
                        />
                      </div>
                      <div className="form-group">
                        <label htmlFor="rafale-max-questions-input">Plafond de questions</label>
                        <input
                          id="rafale-max-questions-input"
                          type="number"
                          value={formData.rafaleMaxQuestions}
                          onChange={(e) => handleInputChange('rafaleMaxQuestions', Math.min(100, parseInt(e.target.value) || 1))}
                          min="1"
                          max="100"
                        />
                      </div>
                    </div>

                    {/* Alerte de pool (contrat §7.2) — mêmes 3 états
                        qu'avant le lancement (GamePage.jsx), calculés ici
                        depuis les catégories/difficulté en cours d'édition
                        pour guider l'admin AVANT même de sauvegarder la
                        manche. */}
                    <RafalePoolAlert
                      category={formData.category}
                      difficulty={formData.rafaleDifficulty}
                      roundTime={parseInt(formData.time) || 0}
                      questionTime={formData.rafaleQuestionTime}
                    />
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

                {/* Hide Image question/answer for MEMORY/MEMOTION/RAFALE — ARDOISE supports images (#94).
                    RAFALE (v8.0.0, #16) — aucun média, contrat §3.3/D3 (texte seul, réservoir). */}
                {formData.type !== 'MEMORY' && formData.type !== 'MEMOTION' && formData.type !== 'RAFALE' && (
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
        </>
      )}

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
          // #215 — hasUnsavedQuizChanges (T2.5) omis délibérément : le
          // formulaire Quiz vit désormais sur une page séparée
          // (BackstagePage.jsx), donc plus jamais "non enregistré ET visible
          // en même temps" que cette modale — le prop retombe sur son défaut
          // (false), structurellement toujours correct depuis cette page.
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
    </div>
  )
}
