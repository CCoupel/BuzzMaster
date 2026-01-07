import { useState, useRef, useMemo, useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { useGame } from '../hooks/GameContext'
import Button from '../components/Button'
import Card, { CardHeader, CardBody } from '../components/Card'
import './QuestionsPage.css'

export default function QuestionsPage() {
  const { questions, fsInfo, deleteQuestion } = useGame()
  const [isUploading, setIsUploading] = useState(false)
  const [editingId, setEditingId] = useState(null)
  const fileInputRef = useRef(null)

  // Form state
  const [formData, setFormData] = useState({
    question: '',
    answer: '',
    points: '1',
    time: '30',
    media: null,
    existingMedia: null,
  })

  const sortedQuestions = useMemo(() => {
    return Object.values(questions)
      .filter(q => q && q.ID)
      .sort((a, b) => parseInt(a.ID) - parseInt(b.ID))
  }, [questions])

  const handleInputChange = (field, value) => {
    setFormData(prev => ({ ...prev, [field]: value }))
  }

  const handleFileChange = (e) => {
    const file = e.target.files?.[0]
    if (file) {
      setFormData(prev => ({ ...prev, media: file, existingMedia: null }))
    }
  }

  const handleQuestionClick = (question) => {
    setEditingId(question.ID)
    setFormData({
      question: question.QUESTION || '',
      answer: question.ANSWER || '',
      points: question.POINTS || '1',
      time: question.TIME || '30',
      media: null,
      existingMedia: question.MEDIA || null,
    })
    if (fileInputRef.current) {
      fileInputRef.current.value = ''
    }
  }

  const handleNewQuestion = () => {
    setEditingId(null)
    setFormData({
      question: '',
      answer: '',
      points: '1',
      time: '30',
      media: null,
      existingMedia: null,
    })
    if (fileInputRef.current) {
      fileInputRef.current.value = ''
    }
  }

  const handleSubmit = async (e) => {
    e.preventDefault()
    if (!formData.question || !formData.answer) return

    setIsUploading(true)

    const data = new FormData()
    if (editingId) {
      data.append('number', editingId)
    }
    data.append('question', formData.question)
    data.append('answer', formData.answer)
    data.append('points', formData.points)
    data.append('time', formData.time)
    if (formData.media) {
      data.append('file', formData.media)
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
                >
                  <Card
                    hover
                    padding="md"
                    className={`question-card ${editingId === question.ID ? 'selected' : ''}`}
                    onClick={() => handleQuestionClick(question)}
                  >
                    <div className="question-card-header">
                      <span className="question-id">#{question.ID}</span>
                      <Button
                        variant="danger"
                        size="xs"
                        onClick={(e) => handleDelete(e, question.ID)}
                      >
                        X
                      </Button>
                    </div>
                    {question.MEDIA && (
                      <img
                        src={question.MEDIA}
                        alt=""
                        className="question-thumbnail"
                      />
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
                  <label htmlFor="media-input">Image (optionnel)</label>
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
                        Supprimer image
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
