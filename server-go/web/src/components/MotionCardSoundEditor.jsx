import { useMemo, useEffect, useRef } from 'react'

// #235 (T13) — bloc « Son de la carte » d'une carte MEMOTION (maquette
// admin_question_editor__sound_memory_memotion_entracte.html §03).
// Champs de carte : sound (File) | existingSound (chemin serveur) |
// soundTimerDelayed (chrono DE LA CARTE) | soundLoop | soundCleared.
// Contrat sound.md §14.4 : « En boucle » est exclusif de « À la fin ».
// Aucune préservation ici : le serveur fusionne par ID de carte.

export default function MotionCardSoundEditor({ card, onChange }) {
  const inputRef = useRef(null)
  const objectUrl = useMemo(
    () => (card.sound instanceof File ? URL.createObjectURL(card.sound) : null),
    [card.sound],
  )
  useEffect(() => () => { if (objectUrl) URL.revokeObjectURL(objectUrl) }, [objectUrl])

  const hasSound = !!(card.sound || card.existingSound)
  const delayed = !!card.soundTimerDelayed
  const loop = !!card.soundLoop

  const setDelayed = (next) => {
    onChange('soundTimerDelayed', next)
    if (next) onChange('soundLoop', false)
  }
  const setLoop = (next) => {
    onChange('soundLoop', next)
    if (next) onChange('soundTimerDelayed', false)
  }

  return (
    <div className="form-group motion-card-sound" data-testid={`motion-card-sound-${card.id}`}>
      <label htmlFor={`motion-card-sound-input-${card.id}`}>
        Son de la carte{' '}
        <span className="section-hint">(optionnel — WAV, 30 s maximum)</span>
      </label>
      <input
        id={`motion-card-sound-input-${card.id}`}
        type="file"
        ref={inputRef}
        accept=".wav,audio/wav"
        onChange={(e) => {
          const f = e.target.files?.[0]
          if (f) {
            onChange('sound', f)
            onChange('existingSound', null)
            onChange('soundCleared', false)
          }
        }}
      />
      {hasSound && (
        <div className="sound-preview">
          <div className="sound-preview-info">
            <span className="sound-preview-name">
              {card.sound ? card.sound.name : decodeURIComponent(card.existingSound.split('/').pop())}
            </span>
          </div>
          <audio
            key={objectUrl || card.existingSound}
            controls
            preload="metadata"
            src={card.sound ? objectUrl : card.existingSound}
          />
          <button
            type="button"
            className="btn-ghost"
            onClick={() => {
              onChange('sound', null)
              onChange('existingSound', null)
              onChange('soundCleared', true)
              onChange('soundTimerDelayed', false)
              onChange('soundLoop', false)
              if (inputRef.current) inputRef.current.value = ''
            }}
          >
            Supprimer
          </button>
        </div>
      )}
      <div className={`sound-timer-toggle ${!hasSound ? 'disabled' : ''}`}>
        <span className="sound-timer-toggle-label">Chrono de la carte démarre</span>
        <span className="sound-timer-switch-group">
          <span className={`sound-timer-switch-side ${!delayed ? 'active' : ''}`}>Début</span>
          <button
            type="button"
            role="switch"
            aria-checked={delayed}
            aria-label="Démarrer le chrono de la carte à la fin du son"
            className={`quiz-switch ${delayed ? 'on' : ''}`}
            disabled={!hasSound}
            onClick={() => setDelayed(!delayed)}
          />
          <span className={`sound-timer-switch-side ${delayed ? 'active' : ''}`}>À la fin</span>
        </span>
      </div>
      <div className={`sound-timer-toggle ${!hasSound ? 'disabled' : ''}`}>
        <span className="sound-timer-toggle-label">Lecture</span>
        <span className="sound-timer-switch-group">
          <span className={`sound-timer-switch-side ${!loop ? 'active' : ''}`}>Une fois</span>
          <button
            type="button"
            role="switch"
            aria-checked={loop}
            aria-label="Jouer le son de la carte en boucle"
            className={`quiz-switch ${loop ? 'on' : ''}`}
            disabled={!hasSound}
            onClick={() => setLoop(!loop)}
          />
          <span className={`sound-timer-switch-side ${loop ? 'active' : ''}`}>En boucle</span>
        </span>
      </div>
      <p className="sound-loop-hint">
        {delayed
          ? 'Indisponible en boucle : une boucle n\'a pas de fin. Le chrono de la carte attend la fin du son — ou le Stop de l\'animateur.'
          : 'Le son démarre au retournement de la carte et remplace le son de la manche ; il s\'arrête à la fermeture de la carte, à la révélation ou au STOP.'}
      </p>
    </div>
  )
}
