import { useState, useRef, useMemo } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { useGame } from '../hooks/GameContext'
import Button from '../components/Button'
import Card, { CardHeader, CardBody } from '../components/Card'
import CategoryBalance from '../components/CategoryBalance'
import QuestionCard, { CATEGORIES } from '../components/QuestionCard'
import './QuestionsPage.css'

// QCM answer colors (for form only)
const QCM_COLORS = {
  RED: { label: 'Rouge', color: '#ef4444', letter: 'A' },
  GREEN: { label: 'Vert', color: '#22c55e', letter: 'B' },
  YELLOW: { label: 'Jaune', color: '#eab308', letter: 'C' },
  BLUE: { label: 'Bleu', color: '#3b82f6', letter: 'D' },
}

// Re-export CATEGORIES for backward compatibility
export { CATEGORIES }

export default function QuestionsPage() {
  const { questions, fsInfo, deleteQuestion, sendMessage } = useGame()
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
    type: 'NORMAL',
    category: '', // Question category
    pointsTarget: 'PLAYER', // PLAYER or TEAM
    qcmAnswers: { RED: '', GREEN: '', YELLOW: '', BLUE: '' },
    qcmCorrect: '',
    // Memory game fields
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
    points: '1',
    time: '30',
    media: null,
    existingMedia: null,
    mediaAnswer: null,
    existingMediaAnswer: null,
  })

  const sortedQuestions = useMemo(() => {
    return Object.values(questions)
      .filter(q => q && q.ID)
      .sort((a, b) => {
        // Sort by ORDER if available, otherwise by ID
        const orderA = a.ORDER !== undefined ? parseInt(a.ORDER) : parseInt(a.ID)
        const orderB = b.ORDER !== undefined ? parseInt(b.ORDER) : parseInt(b.ID)
        return orderA - orderB
      })
  }, [questions])

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

  const handleInputChange = (field, value) => {
    setFormData(prev => {
      const updates = { [field]: value }
      // Auto-set pointsTarget when type changes
      if (field === 'type') {
        // QCM and MEMORY default to TEAM, NORMAL defaults to PLAYER
        updates.pointsTarget = (value === 'QCM' || value === 'MEMORY') ? 'TEAM' : 'PLAYER'
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
    const qType = question.TYPE || 'NORMAL'
    // Default pointsTarget based on type if not set
    const defaultTarget = (qType === 'QCM' || qType === 'MEMORY') ? 'TEAM' : 'PLAYER'

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

    setFormData({
      question: question.QUESTION || '',
      answer: question.ANSWER || '',
      type: qType,
      category: question.CATEGORY || '',
      pointsTarget: question.POINTS_TARGET || defaultTarget,
      qcmAnswers: question.QCM_ANSWERS || { RED: '', GREEN: '', YELLOW: '', BLUE: '' },
      qcmCorrect: question.QCM_CORRECT || '',
      memoryPairs,
      memoryConfig,
      points: question.POINTS || '1',
      time: question.TIME || '30',
      media: null,
      existingMedia: question.MEDIA || null,
      mediaAnswer: null,
      existingMediaAnswer: question.MEDIA_ANSWER || null,
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
      type: 'NORMAL',
      category: '',
      pointsTarget: 'PLAYER',
      qcmAnswers: { RED: '', GREEN: '', YELLOW: '', BLUE: '' },
      qcmCorrect: '',
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
      points: '1',
      time: '30',
      media: null,
      existingMedia: null,
      mediaAnswer: null,
      existingMediaAnswer: null,
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

  const handleSubmit = async (e) => {
    e.preventDefault()
    // For normal questions, need question and answer
    // For QCM, need question and at least the correct answer filled
    // For Memory, need question and at least 2 valid pairs
    if (!formData.question) return
    if (formData.type === 'NORMAL' && !formData.answer) return
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

    if (formData.type === 'NORMAL') {
      data.append('answer', formData.answer)
    } else if (formData.type === 'QCM') {
      // QCM mode - send QCM answers and correct answer
      data.append('qcm_answers', JSON.stringify(formData.qcmAnswers))
      data.append('qcm_correct', formData.qcmCorrect)
      // The answer field contains the correct answer text for display
      data.append('answer', formData.qcmAnswers[formData.qcmCorrect])
    } else if (formData.type === 'MEMORY') {
      // Memory mode - send pairs and config
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
        <p className="page-subtitle">{sortedQuestions.length} questions disponibles</p>
      </header>

      {/* Category Balance Visualization */}
      <CategoryBalance questions={sortedQuestions} />

      <div className="questions-layout">
        {/* Questions List */}
        <section className="questions-list-section">
          <div className="questions-grid">
            <AnimatePresence>
              {sortedQuestions.map((question, index) => (
                <QuestionCard
                  key={question.ID}
                  question={question}
                  selected={editingId === question.ID}
                  draggable
                  showDelete
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
                {editingId && (
                  <Button variant="ghost" size="sm" onClick={handleNewQuestion}>
                    + Nouveau
                  </Button>
                )}
              </div>
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

                {/* Question Type Selector */}
                <div className="form-group">
                  <label>Type de question</label>
                  <div className="type-selector">
                    <button
                      type="button"
                      className={`type-btn ${formData.type === 'NORMAL' ? 'active' : ''}`}
                      onClick={() => handleInputChange('type', 'NORMAL')}
                    >
                      Normal
                    </button>
                    <button
                      type="button"
                      className={`type-btn qcm ${formData.type === 'QCM' ? 'active' : ''}`}
                      onClick={() => handleInputChange('type', 'QCM')}
                    >
                      QCM
                    </button>
                    <button
                      type="button"
                      className={`type-btn memory ${formData.type === 'MEMORY' ? 'active' : ''}`}
                      onClick={() => handleInputChange('type', 'MEMORY')}
                    >
                      Memory
                    </button>
                  </div>
                </div>

                {/* Category Selector */}
                <div className="form-group">
                  <label>Categorie</label>
                  <div className="category-selector">
                    {Object.entries(CATEGORIES).map(([key, { label, icon, color }]) => (
                      <button
                        key={key}
                        type="button"
                        className={`category-btn ${formData.category === key ? 'active' : ''}`}
                        style={{ '--cat-color': color }}
                        onClick={() => handleInputChange('category', formData.category === key ? '' : key)}
                        title={label}
                      >
                        <span className="category-icon">{icon}</span>
                      </button>
                    ))}
                  </div>
                  {formData.category && (
                    <span className="category-label" style={{ color: CATEGORIES[formData.category]?.color }}>
                      {CATEGORIES[formData.category]?.label}
                    </span>
                  )}
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
                {formData.type === 'NORMAL' && (
                  <div className="form-group">
                    <label htmlFor="answer-input">Reponse *</label>
                    <input
                      id="answer-input"
                      type="text"
                      value={formData.answer}
                      onChange={(e) => handleInputChange('answer', e.target.value)}
                      placeholder="Entrez la reponse..."
                      required
                    />
                  </div>
                )}

                {/* QCM Answers */}
                {formData.type === 'QCM' && (
                  <div className="qcm-answers-section">
                    <label>Reponses QCM *</label>
                    <div className="qcm-answers-grid">
                      {Object.entries(QCM_COLORS).map(([colorKey, { label, color, letter }]) => (
                        <div
                          key={colorKey}
                          className={`qcm-answer-item ${formData.qcmCorrect === colorKey ? 'correct' : ''}`}
                          style={{ '--qcm-color': color }}
                        >
                          <div className="qcm-answer-header">
                            <span className="qcm-letter" style={{ backgroundColor: color }}>{letter}</span>
                            <button
                              type="button"
                              className={`qcm-correct-btn ${formData.qcmCorrect === colorKey ? 'active' : ''}`}
                              onClick={() => handleInputChange('qcmCorrect', colorKey)}
                              title="Marquer comme bonne reponse"
                            >
                              {formData.qcmCorrect === colorKey ? '✓' : '○'}
                            </button>
                          </div>
                          <input
                            type="text"
                            value={formData.qcmAnswers[colorKey]}
                            onChange={(e) => handleQcmAnswerChange(colorKey, e.target.value)}
                            placeholder={`Reponse ${letter}...`}
                            className="qcm-answer-input"
                          />
                        </div>
                      ))}
                    </div>
                    {!formData.qcmCorrect && (
                      <p className="qcm-hint">Cliquez sur ○ pour indiquer la bonne reponse</p>
                    )}
                  </div>
                )}

                {/* Memory Pairs Editor */}
                {formData.type === 'MEMORY' && (
                  <div className="memory-section">
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
                        {formData.memoryPairs.flatMap(pair => [
                          { ...pair.card1, pairId: pair.id, cardNum: 1 },
                          { ...pair.card2, pairId: pair.id, cardNum: 2 },
                        ]).sort(() => Math.random() - 0.5).map((card, idx) => (
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

                <div className="form-row">
                  {/* Hide Points for MEMORY - calculated per pair */}
                  {formData.type !== 'MEMORY' && (
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
                </div>

                {/* Hide Image question/answer for MEMORY - images are in pairs */}
                {formData.type !== 'MEMORY' && (
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
    </div>
  )
}
