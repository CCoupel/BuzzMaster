import { useState, useEffect } from 'react'
import Button from './Button'
import Card, { CardHeader, CardBody } from './Card'

// Énumérations partagées avec la modale IA et le contrat backend
// (contracts/ai-generation.md §6) — source de vérité pour les selects
// Population/Langue et le multi-select Difficulté, ici comme dans la modale.
export const QUIZ_POPULATIONS = ['Junior (6-12 ans)', 'Ado (13-17 ans)', 'Adulte (18-64 ans)', 'Senior (65+ ans)', 'Famille']
export const QUIZ_DIFFICULTIES = ['Facile', 'Moyen', 'Difficile', 'Expert']
export const QUIZ_LANGUAGES = ['Français', 'Anglais', 'Espagnol']

/**
 * QuizMetaForm — onglet "Quiz" de BackstagePage (#215, extrait de
 * QuestionsPage.jsx où il vivait historiquement dans la "Zone 1").
 *
 * Formulaire des métadonnées de la partie (nom/thème/publics/difficultés/
 * langue/objectif/notes) + bouton NOUVELLE PARTIE — action dédiée
 * UPDATE_QUIZ_META (contract game-state.md "Métadonnées Quiz"), aucun champ
 * partagé avec EntracteConfigForm/BackgroundsManager.
 *
 * `onNewGame` reste optionnel : GamePage (#215 AC9) rend aussi son propre
 * déclencheur NOUVELLE PARTIE directement depuis `useGame().newGame`, sans
 * passer par ce composant.
 */
export default function QuizMetaForm({ gameState, sendMessage, onNewGame }) {
  const [quizName, setQuizName] = useState(gameState.quizName || '')
  const [quizTheme, setQuizTheme] = useState(gameState.quizTheme || '')
  const [quizNotes, setQuizNotes] = useState(gameState.quizNotes || '')
  // v6.1.0 (#137) — publics/difficultés multiples, objectif de partie,
  // visibilité TV par champ. Contract game-state.md §"Métadonnées Quiz".
  const [quizPopulations, setQuizPopulations] = useState(gameState.quizPopulations || [])
  const [quizDifficulties, setQuizDifficulties] = useState(gameState.quizDifficulties || [])
  const [quizLanguage, setQuizLanguage] = useState(gameState.quizLanguage || 'Français')
  const [quizObjectives, setQuizObjectives] = useState(gameState.quizObjectives || '')
  const [quizHiddenFields, setQuizHiddenFields] = useState(gameState.quizHiddenFields || [])
  const [quizSaved, setQuizSaved] = useState(false)

  // Sync du formulaire avec gameState (peuplé depuis le WS après montage)
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

  return (
    <section className="quiz-meta-section">
      <Card padding="lg">
        <CardHeader>
          <div className="section-header">
            <h3 className="section-title">Quiz</h3>
            {onNewGame && (
              <Button variant="fun" size="sm" onClick={onNewGame} title="Réinitialiser le jeu et préparer une nouvelle partie">
                NOUVELLE PARTIE
              </Button>
            )}
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
        </CardBody>
      </Card>
    </section>
  )
}
