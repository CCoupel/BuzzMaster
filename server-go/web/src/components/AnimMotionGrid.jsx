import { useMemo } from 'react'
import { getMotionGridCols, getMotionCardCoord, isMotionSecretMode } from '../utils/motionGrid'
import { getRgbColor } from '../utils/colorUtils'
import './AnimMotionGrid.css'

/**
 * AnimMotionGrid — grille MEMOTION tactile de `/anim` (#160/F4).
 *
 * **N'est PAS une réutilisation de `AnimMemoryGrid`** (#159) : modèle de
 * données différent (`MOTION_CARDS` vs paires `MEMORY_PAIRS` mélangées),
 * formule de colonnes différente (`utils/motionGrid.js`, ⚠️ ≠
 * `utils/memoryGrid.js`), états de carte différents (pas de « retournée »
 * intermédiaire — une carte MEMOTION est soit `UNPLAYED`, soit `DONE`, le
 * jeu de la carte elle-même se joue en plein écran via `AnimMotionCard`).
 * La réutilisation porte sur le VOCABULAIRE VISUEL et les CONVENTIONS
 * d'interaction (cibles tactiles ≥ 62px, dégradé violet `#6366f1 →
 * #8b5cf6 → #a855f7`, identique à `/tv`), pas sur le composant — décision
 * de plan signalée au GATE 2.
 *
 * Colonnes et coordonnées **exclusivement** via `utils/motionGrid.js`
 * (#160/F0) — jamais recalculées ici, pour garantir la correspondance
 * positionnelle avec `/tv` (R1, le risque central du lot #160). Aucun
 * mélange local (contrairement à MEMORY) : les cartes sont rendues dans le
 * MÊME ordre que `question.MOTION_CARDS`.
 *
 * AUCUNE LOGIQUE DE JEU CÔTÉ CLIENT : `onSelect` ne fait qu'émettre
 * `MEMOTION_SELECT` (via le hook) — le serveur décide de tout le reste
 * (passage en `SELECTED`, etc.), qui arrive comme un UPDATE ordinaire.
 *
 * @param {Object} props
 * @param {Object|null} props.question - gameState.question (question MEMOTION)
 * @param {string} props.phase - gameState.phase
 * @param {string} props.subphase - gameState.MEMOTION_SUBPHASE
 * @param {Object} [props.cardStates] - gameState.MEMOTION_CARD_STATES ({cardID: 'UNPLAYED'|'QUESTION'|'REVEALED'|'DONE'})
 * @param {Object} [props.cardTeams] - gameState.MEMOTION_CARD_TEAMS ({cardID: teamName})
 * @param {string} [props.currentTeam] - gameState.MEMOTION_CURRENT_TEAM
 * @param {string} [props.selectedId] - gameState.MEMOTION_SELECTED
 * @param {Object} [props.teams] - teams (useGame()), pour la couleur de l'équipe gagnante
 * @param {(cardId: string) => void} props.onSelect - selectMotionCard (useGame())
 */
export default function AnimMotionGrid({
  question,
  phase,
  subphase,
  cardStates,
  cardTeams,
  currentTeam,
  selectedId,
  teams,
  onSelect,
}) {
  const motionCards = question?.MOTION_CARDS || []
  const cols = useMemo(() => getMotionGridCols(motionCards.length), [motionCards.length])
  const isSecretMode = isMotionSecretMode(question)
  // Coordonnée-seule (sans thème ni étoiles, la difficulté trahirait la
  // carte) UNIQUEMENT en GRID — en MEMORIZE, les joueurs mémorisent encore
  // le thème, la coordonnée n'a pas de sens tant que la manche n'y est pas.
  // Verbatim PlayerDisplay.jsx:2106 (`isSecretMode && subphase === 'GRID'`).
  const showCoordOnly = isSecretMode && subphase === 'GRID'

  if (motionCards.length === 0) return null

  const doneCount = motionCards.filter(c => (cardStates?.[c.ID] || 'UNPLAYED') === 'DONE').length

  return (
    <div className="anim-motion">
      {/* F4 — bandeau de compteurs : équipe au tour + cartes jouées/total. */}
      <div className="anim-motion-hud">
        {currentTeam && phase === 'STARTED' && (
          <span className="anim-motion-hud-chip anim-motion-hud-turn">
            <span className="anim-motion-hud-key">au tour de</span>
            <span className="anim-motion-hud-value">{currentTeam}</span>
          </span>
        )}
        <span className="anim-motion-hud-chip">
          <span className="anim-motion-hud-key">cartes</span>
          <span className="anim-motion-hud-value">{doneCount}/{motionCards.length}</span>
        </span>
      </div>

      <div
        className="anim-motion-grid"
        style={{ '--anim-motion-cols': cols }}
      >
        {motionCards.map((card, index) => {
          const state = cardStates?.[card.ID] || 'UNPLAYED'
          const isDone = state === 'DONE'
          const winnerTeam = isDone ? (cardTeams?.[card.ID] || '') : ''
          const winnerColor = winnerTeam && teams?.[winnerTeam]?.COLOR
            ? getRgbColor(teams[winnerTeam].COLOR)
            : null
          const canClick = !isDone && subphase === 'GRID' && phase === 'STARTED'
          const diff = card.DIFFICULTY || 1
          const coord = getMotionCardCoord(index, cols)

          const stateClass = isDone
            ? 'anim-motion-card-done'
            : (canClick ? 'anim-motion-card-active' : 'anim-motion-card-inert')

          return (
            <button
              key={card.ID}
              type="button"
              className={`anim-motion-card ${stateClass}`}
              disabled={!canClick}
              onClick={() => canClick && onSelect(card.ID)}
              // Toujours renseignée sur DONE, même sans gagnant : repli sur
              // un neutre plutôt qu'une couleur d'équipe (aucune équipe à
              // colorer si personne n'a gagné).
              style={isDone ? { '--anim-motion-winner-color': winnerColor || '#4b4368' } : undefined}
              aria-label={showCoordOnly ? `Carte ${coord}` : (card.RECTO_THEME || `Carte ${index + 1}`)}
            >
              {isDone ? (
                <>
                  <span className="anim-motion-card-title">{card.RECTO_THEME}</span>
                  <span className="anim-motion-card-team">{winnerTeam ? `${winnerTeam} ✓` : 'PERSONNE –'}</span>
                </>
              ) : showCoordOnly ? (
                <span className="anim-motion-card-coord">{coord}</span>
              ) : (
                <>
                  <span className="anim-motion-card-title">{card.RECTO_THEME}</span>
                  <span className="anim-motion-card-stars">{'★'.repeat(diff)}</span>
                </>
              )}
            </button>
          )
        })}
      </div>
    </div>
  )
}
