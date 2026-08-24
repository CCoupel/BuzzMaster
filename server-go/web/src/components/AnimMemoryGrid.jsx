import { useMemo } from 'react'
import { buildMemoryCards, getMemoryGridCols, getMemoryGridRows } from '../utils/memoryGrid'
import { getRgbColor } from '../utils/colorUtils'
import './AnimMemoryGrid.css'

/**
 * AnimMemoryGrid — grille MEMORY tactile de `/anim` (#159/F1+F2).
 *
 * Composant CRÉÉ, PAS repris du rendu de `PlayerDisplay.jsx` (dense, pensé
 * souris, gardé par `isAdminPreview`) — mais consommant OBLIGATOIREMENT
 * `utils/memoryGrid.js` (#159/F0) pour l'ordre des cartes ET le nombre de
 * colonnes : ni l'un ni l'autre n'est recalculé ici. Le nombre de colonnes
 * ne dépend JAMAIS de la largeur de la tablette — seule la taille des
 * cartes s'adapte (CSS). Si la hauteur manque, c'est le bloc central de la
 * conduite qui défile (#171) — la disposition ne bouge pas.
 *
 * AUCUNE LOGIQUE DE JEU CÔTÉ CLIENT : pas de détection de paire, pas de
 * minuterie de retour automatique après échec. `canClick` ne fait que
 * gater le GESTE (phase STARTED, carte pas déjà retournée/trouvée) — le
 * serveur décide de tout le reste (retournement forcé après échec, bascule
 * de tour...), qui arrive comme un UPDATE ordinaire. La tablette est une
 * télécommande, pas un second moteur de jeu.
 *
 * Quatre états de carte : face cachée (lettre A/B/C..., même design que
 * PlayerDisplay.css `.memory-card-back`, cliquable si canClick) · retournée
 * (`MEMORY_FLIPPED_CARDS`, ou TOUTE carte non trouvée une fois en phase
 * `REVEALED` — sinon les paires jamais trouvées restent invisibles, bug
 * remonté en QUALIF v6.2.0.27) · paire trouvée (couleur du propriétaire,
 * `MEMORY_PAIR_OWNERS`) · inerte hors phase `STARTED`.
 *
 * #184/B-F2 — reçoit `playable`/`revealed` (contexte d'hôte normalisé,
 * `utils/hostContext.js`) au lieu de `phase` : ce composant ne lit plus
 * jamais `gameState.phase` ni `MEMOTION_SUBPHASE` directement. Neutre pour
 * l'hôte question (`playable` = phase === 'STARTED'`, `revealed` = phase ===
 * 'REVEALED'`, mêmes valeurs qu'avant) — c'est ce qui rend ce composant
 * montable dans l'hôte carte MEMOTION sans variante, quand MEMORY deviendra
 * nestable (#187, v7.1.0).
 *
 * @param {Object} props
 * @param {Object|null} props.question - gameState.question (question MEMORY)
 * @param {boolean} props.playable - hostContext.playable (ex `phase === 'STARTED'`)
 * @param {boolean} props.revealed - hostContext.revealed (ex `phase === 'REVEALED'`)
 * @param {Object} props.teams - teams (useGame()), pour la couleur du propriétaire
 * @param {string[]} [props.flippedCards] - gameState.memoryFlippedCards
 * @param {string[]} [props.matchedPairs] - gameState.memoryMatchedPairs (pairId[])
 * @param {Object} [props.pairOwners] - gameState.MEMORY_PAIR_OWNERS ({pairId: teamName})
 * @param {string|null} [props.currentTeam] - gameState.MEMORY_CURRENT_TEAM
 * @param {Object} [props.teamPairs] - gameState.MEMORY_TEAM_PAIRS ({teamName: count})
 * @param {Object} [props.teamErrors] - gameState.MEMORY_TEAM_ERRORS ({teamName: count})
 * @param {number} [props.globalErrors] - gameState.memoryErrors (mode SOLO, pas d'équipes)
 * @param {(cardId: string) => void} props.onFlip - flipMemoryCard (useGame())
 */
export default function AnimMemoryGrid({
  question,
  playable,
  revealed,
  teams,
  flippedCards,
  matchedPairs,
  pairOwners,
  currentTeam,
  teamPairs,
  teamErrors,
  globalErrors,
  onFlip,
}) {
  // #159/F0 — ordre et colonnes viennent EXCLUSIVEMENT de l'utilitaire
  // partagé, jamais recalculés ici (correspondance positionnelle avec
  // /tv et la vue joueur).
  const cards = useMemo(
    () => buildMemoryCards(question),
    [question?.MEMORY_PAIRS, question?.ID]
  )
  const cols = useMemo(() => getMemoryGridCols(cards.length), [cards.length])
  const rows = useMemo(() => getMemoryGridRows(cards.length, cols), [cards.length, cols])

  if (cards.length === 0) return null

  const totalPairs = question?.MEMORY_PAIRS?.length || 0
  const matchedCount = matchedPairs?.length || 0
  const isMultiTeam = teamPairs && Object.keys(teamPairs).length > 0
  const hudErrors = isMultiTeam
    ? Object.values(teamErrors || {}).reduce((sum, n) => sum + (n || 0), 0)
    : (globalErrors || 0)
  const complete = totalPairs > 0 && matchedCount >= totalPairs

  return (
    <div className="anim-memory">
      {/* F2 — bandeau de compteurs, lu depuis l'état, jamais recalculé
          (équipe active, paires trouvées/total, erreurs). */}
      <div className="anim-memory-hud">
        {isMultiTeam && currentTeam && playable && (
          <span className="anim-memory-hud-chip anim-memory-hud-turn">
            <span className="anim-memory-hud-key">au tour de</span>
            <span className="anim-memory-hud-value">{currentTeam}</span>
          </span>
        )}
        <span className="anim-memory-hud-chip">
          <span className="anim-memory-hud-key">paires</span>
          <span className="anim-memory-hud-value">{complete ? 'complète' : `${matchedCount}/${totalPairs}`}</span>
        </span>
        <span className="anim-memory-hud-chip">
          <span className="anim-memory-hud-key">erreurs</span>
          <span className="anim-memory-hud-value">{hudErrors}</span>
        </span>
      </div>

      <div
        className="anim-memory-grid"
        style={{ '--anim-memory-cols': cols, '--anim-memory-rows': rows }}
      >
        {cards.map((cardData, index) => {
          // Bug remonté après #159 (issue #159, commentaire du
          // 2026-08-17) : le dos de carte affichait une icône générique
          // (🃏), différente de celle de `/tv`/la vue joueur
          // (PlayerDisplay.jsx:1819-1884) — une lettre (A, B, C...) dérivée
          // de la position dans l'ordre mélangé. Repris ICI à l'identique,
          // sur le MÊME ordre `cards` (issu de utils/memoryGrid.js, #159/F0)
          // — la lettre d'une carte est donc la même sur /anim et /tv,
          // cohérent avec la correspondance positionnelle du lot (« la
          // carte B », pas seulement « la 2e en haut à gauche »). Ce n'est
          // pas une invention : `cardLetter = String.fromCharCode(65 + index)`
          // est repris verbatim de PlayerDisplay.jsx:1819, PAS un placeholder
          // deviné.
          const cardLetter = String.fromCharCode(65 + index)
          const isMatched = matchedPairs?.includes(cardData.pairId)
          const isFlipped = flippedCards?.includes(cardData.id)
          // Bug remonté en QUALIF v6.2.0.27 (#159) : `revealed` ne
          // couvrait que "trouvée" et "retournée en direct" — en phase
          // REVEALED, une paire jamais trouvée (délai écoulé, erreur non
          // récupérée) n'était ni l'un ni l'autre : TOUTES les cartes
          // restaient face cachée, aucun nom visible pour arbitrer.
          // PlayerDisplay.jsx a le même besoin (révélation progressive
          // via un état `revealedPairs` animé, ligne ~1834) ; ici on
          // couvre le même cas SANS minuterie/animation côté client (pas
          // de logique de jeu à réinventer) — dès que le serveur annonce
          // REVEALED, tout le contenu s'affiche d'un coup.
          // Aucune restriction par équipe côté /anim (contrairement au
          // VPlayer) : l'animateur joue pour la table, comme la TV/l'aperçu
          // admin — même geste disponible quelle que soit l'équipe active.
          const canClick = playable && !isMatched && !isFlipped
          const ownerTeam = pairOwners?.[String(cardData.pairId)]
          const ownerColor = ownerTeam && teams?.[ownerTeam]?.COLOR
            ? getRgbColor(teams[ownerTeam].COLOR)
            : null
          const cardRevealed = isMatched || isFlipped || revealed
          const state = isMatched
            ? 'matched'
            : (isFlipped || revealed)
              ? 'up'
              : !playable
                ? 'inert'
                : 'down'

          return (
            <button
              key={cardData.id}
              type="button"
              className={`anim-memory-card anim-memory-card-${state}`}
              disabled={!canClick}
              onClick={() => canClick && onFlip(cardData.id)}
              style={ownerColor ? { '--anim-memory-owner-color': ownerColor } : undefined}
              aria-label={cardRevealed ? undefined : `Carte ${cardLetter}, face cachée`}
            >
              {cardRevealed ? (
                cardData.card?.IS_IMAGE && cardData.card?.IMAGE ? (
                  <img src={cardData.card.IMAGE} alt="" className="anim-memory-card-image" />
                ) : (
                  <span className="anim-memory-card-text">{cardData.card?.TEXT}</span>
                )
              ) : (
                <span className="anim-memory-card-letter">{cardLetter}</span>
              )}
            </button>
          )
        })}
      </div>
    </div>
  )
}
