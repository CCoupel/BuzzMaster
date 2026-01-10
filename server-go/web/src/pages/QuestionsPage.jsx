import { useState, useRef, useMemo, useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { useGame } from '../hooks/GameContext'
import Button from '../components/Button'
import Card, { CardHeader, CardBody } from '../components/Card'
import './QuestionsPage.css'

// QCM answer colors
const QCM_COLORS = {
  RED: { label: 'Rouge', color: '#ef4444', letter: 'A' },
  GREEN: { label: 'Vert', color: '#22c55e', letter: 'B' },
  YELLOW: { label: 'Jaune', color: '#eab308', letter: 'C' },
  BLUE: { label: 'Bleu', color: '#3b82f6', letter: 'D' },
}

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
    qcmAnswers: { RED: '', GREEN: '', YELLOW: '', BLUE: '' },
    qcmCorrect: '',
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
    setFormData(prev => ({ ...prev, [field]: value }))
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
    setFormData({
      question: question.QUESTION || '',
      answer: question.ANSWER || '',
      type: question.TYPE || 'NORMAL',
      qcmAnswers: question.QCM_ANSWERS || { RED: '', GREEN: '', YELLOW: '', BLUE: '' },
      qcmCorrect: question.QCM_CORRECT || '',
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
      qcmAnswers: { RED: '', GREEN: '', YELLOW: '', BLUE: '' },
      qcmCorrect: '',
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

  const handleSubmit = async (e) => {
    e.preventDefault()
    // For normal questions, need question and answer
    // For QCM, need question and at least the correct answer filled
    if (!formData.question) return
    if (formData.type === 'NORMAL' && !formData.answer) return
    if (formData.type === 'QCM' && (!formData.qcmCorrect || !formData.qcmAnswers[formData.qcmCorrect])) return

    setIsUploading(true)

    const data = new FormData()
    if (editingId) {
      data.append('number', editingId)
    }
    data.append('question', formData.question)
    data.append('type', formData.type)
    data.append('points', formData.points)
    data.append('time', formData.time)

    if (formData.type === 'NORMAL') {
      data.append('answer', formData.answer)
    } else {
      // QCM mode - send QCM answers and correct answer
      data.append('qcm_answers', JSON.stringify(formData.qcmAnswers))
      data.append('qcm_correct', formData.qcmCorrect)
      // The answer field contains the correct answer text for display
      data.append('answer', formData.qcmAnswers[formData.qcmCorrect])
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

  const storagePercent = fsInfo?.P_USED ? parseFloat(fsInfo.P_USED) : 0

  return (
    <div className="questions-page page">
      <header className="page-header">
        <h1 className="page-title">Gestion des Questions</h1>
        <p className="page-subtitle">{sortedQuestions.length} questions disponibles</p>
      </header>

      <div className="questions-layout">
        {/* Questions List */}
        <section className="questions-list-section">
          <div className="questions-grid">
            <AnimatePresence>
              {sortedQuestions.map((question, index) => (
                <motion.div
                  key={question.ID}
                  initial={{ opacity: 0, scale: 0.9 }}
                  animate={{ opacity: 1, scale: 1 }}
                  exit={{ opacity: 0, scale: 0.9 }}
                  transition={{ delay: index * 0.05 }}
                  draggable
                  onDragStart={(e) => handleDragStart(e, question.ID)}
                  onDragOver={(e) => handleDragOver(e, question.ID)}
                  onDragLeave={handleDragLeave}
                  onDragEnd={handleDragEnd}
                  onDrop={(e) => handleDrop(e, question.ID)}
                  className={`question-drag-wrapper ${draggedId === question.ID ? 'dragging' : ''} ${dragOverId === question.ID ? 'drag-over' : ''}`}
                >
                  <Card
                    hover
                    padding="md"
                    className={`question-card ${editingId === question.ID ? 'selected' : ''}`}
                    onClick={() => handleQuestionClick(question)}
                  >
                    <div className="question-card-header">
                      <span className="drag-handle">⋮⋮</span>
                      <span className="question-id">#{question.ID}</span>
                      {question.TYPE === 'QCM' && (
                        <span className="qcm-badge">QCM</span>
                      )}
                      <Button
                        variant="danger"
                        size="xs"
                        onClick={(e) => handleDelete(e, question.ID)}
                      >
                        X
                      </Button>
                    </div>
                    {(question.MEDIA || question.MEDIA_ANSWER) && (
                      <div className="question-thumbnails">
                        {question.MEDIA && (
                          <img
                            src={question.MEDIA}
                            alt=""
                            className="question-thumbnail"
                          />
                        )}
                        {question.MEDIA_ANSWER && (
                          <img
                            src={question.MEDIA_ANSWER}
                            alt=""
                            className="question-thumbnail answer-thumbnail"
                            title="Image réponse"
                          />
                        )}
                      </div>
                    )}
                    <p className="question-preview">{question.QUESTION}</p>
                    <div className="question-meta">
                      <span>{question.POINTS} pts</span>
                      <span>{question.TIME}s</span>
                    </div>
                  </Card>
                </motion.div>
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
                            <span className="qcm-label">{label}</span>
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

                <div className="form-row">
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
