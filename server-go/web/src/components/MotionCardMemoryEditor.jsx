/**
 * MotionCardMemoryEditor — sous-éditeur MEMORY d'une carte MEMOTION (#187,
 * v7.1.0).
 *
 * Pas de CSS propre : réutilise EXACTEMENT les classes de l'éditeur MEMORY
 * question-scopé (`memory-pairs-list`, `memory-pair-item`,
 * `memory-config-section`…), définies dans `pages/QuestionsPage.css` — même
 * convention que `QcmAnswersEditor.jsx` (voir sa doc de tête). Aucune
 * nouvelle classe visuelle, seule la source des données change (`card.*` au
 * lieu de `formData.*`).
 *
 * Différences délibérées avec l'éditeur MEMORY question-scopé :
 * - **Pas de sélecteur de mode** (SOLO/CHACUN_SON_TOUR/TANT_QUE_JE_GAGNE) :
 *   `MEMORY_MODE` est **ignoré** par le moteur en contexte carte (contrat
 *   §6.3 — une seule équipe, celle de la manche MEMOTION en cours). Ne pas
 *   l'exposer évite un contrôle qui ne ferait jamais rien, et garantit que le
 *   champ reste à sa valeur de création "SOLO" (jamais verrouillant,
 *   `utils/motionCardLock.js`).
 * - **Aucun champ `VALUE`** : le barème d'une carte MEMORY est
 *   `STARS_PRORATA` (contrat §6.2/§6.3), calculé par le serveur à partir des
 *   étoiles de la carte et des paires trouvées — rien à saisir ici.
 * - **Les trois réglages de points de `MEMORY_CONFIG`
 *   (`POINTS_PER_PAIR`/`ERROR_PENALTY`/`COMPLETION_BONUS`) sont neutralisés
 *   visiblement** (désactivés, note explicite) : sans aucune autorité en
 *   contexte carte (contrat §6.1/§6.3), les laisser actifs laisserait
 *   l'utilisateur régler des valeurs sans effet. Les cinq autres réglages
 *   (`FLIP_DELAY`/`MEMORIZE_TIME`/`SHOW_DURING_MEMORIZE`/`REVEAL_DELAY`/
 *   `USE_TIMER`) restent pleinement actifs.
 *
 * @param {Object} props
 * @param {Object} props.card - item de `formData.motionCards[i]` (TYPE=MEMORY)
 *   — lit `card.memoryPairs`/`card.memoryConfig`
 * @param {() => void} props.onAddPair
 * @param {(pairId: number) => void} props.onRemovePair
 * @param {(pairId: number, cardKey: 'card1'|'card2', field: 'type'|'text'|'image', value: any) => void} props.onCardChange
 * @param {(field: string, value: any) => void} props.onConfigChange
 */

// Un côté de paire (card1 ou card2) — factorisé plutôt que dupliqué deux
// fois inline (l'éditeur MEMORY question-scopé de QuestionsPage.jsx, lui,
// duplique — écart assumé : ce composant est neuf, autant éviter la
// duplication dès l'écriture plutôt que la reproduire).
function renderCardSide(pair, sideNum, cardKey, side, onCardChange) {
  return (
    <div className="memory-card-input">
      <div className="memory-card-type-toggle">
        <button
          type="button"
          className={`toggle-btn ${!side.isImage ? 'active' : ''}`}
          onClick={() => onCardChange(pair.id, cardKey, 'type', 'text')}
        >
          Texte
        </button>
        <button
          type="button"
          className={`toggle-btn ${side.isImage ? 'active' : ''}`}
          onClick={() => onCardChange(pair.id, cardKey, 'type', 'image')}
        >
          Image
        </button>
      </div>
      {side.isImage ? (
        <div className="memory-card-image-input">
          {side.image ? (
            <div className="memory-card-image-preview">
              <img
                src={side.image instanceof File ? URL.createObjectURL(side.image) : side.image}
                alt={`Carte ${sideNum}`}
              />
              <button
                type="button"
                className="memory-card-remove-img"
                onClick={() => onCardChange(pair.id, cardKey, 'image', null)}
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
                  if (file) onCardChange(pair.id, cardKey, 'image', file)
                }}
              />
              <span>+ Image</span>
            </label>
          )}
        </div>
      ) : (
        <input
          type="text"
          value={side.text}
          onChange={(e) => onCardChange(pair.id, cardKey, 'text', e.target.value)}
          placeholder={`Texte carte ${sideNum}...`}
          className="memory-card-text-input"
        />
      )}
    </div>
  )
}

export default function MotionCardMemoryEditor({ card, onAddPair, onRemovePair, onCardChange, onConfigChange }) {
  const pairs = card.memoryPairs || []
  const config = card.memoryConfig || {}

  return (
    <div className="memotion-card-memory-section">
      <label>Paires de cartes * ({pairs.length} paires)</label>

      <div className="memory-pairs-list">
        {pairs.map((pair, index) => (
          <div key={pair.id} className="memory-pair-item">
            <div className="memory-pair-header">
              <span className="memory-pair-number">Paire {index + 1}</span>
              {pairs.length > 2 && (
                <button
                  type="button"
                  className="memory-remove-btn"
                  onClick={() => onRemovePair(pair.id)}
                  title="Supprimer cette paire"
                >
                  ×
                </button>
              )}
            </div>
            <div className="memory-pair-cards">
              {renderCardSide(pair, 1, 'card1', pair.card1, onCardChange)}
              <span className="memory-pair-arrow">↔</span>
              {renderCardSide(pair, 2, 'card2', pair.card2, onCardChange)}
            </div>
          </div>
        ))}
      </div>

      {pairs.length < 12 && (
        <button type="button" className="memory-add-btn" onClick={onAddPair}>
          + Ajouter une paire
        </button>
      )}

      {pairs.filter(p => {
        const c1 = p.card1.isImage ? p.card1.image : p.card1.text
        const c2 = p.card2.isImage ? p.card2.image : p.card2.text
        return c1 && c2
      }).length < 2 && (
        <p className="memory-hint">Remplissez au moins 2 paires completes</p>
      )}

      {/* Barème (#187) — aucun champ VALUE : STARS_PRORATA est calculé par le
          serveur, rien à saisir ici. */}
      <p className="motion-card-memory-scoring-note">
        Barème : points de la carte (étoiles) répartis au prorata des paires trouvées.
      </p>

      <div className="memory-config-section">
        <label>Configuration</label>
        <div className="memory-config-grid">
          <div className="memory-config-item">
            <label>Delai retournement (s)</label>
            <input
              type="number"
              value={config.flipDelay}
              onChange={(e) => onConfigChange('flipDelay', parseFloat(e.target.value) || 3)}
              min="1"
              max="10"
              step="0.5"
            />
          </div>
          {/* #187 — les trois réglages de points suivants sont neutralisés :
              sans aucune autorité en contexte carte (contrat §6.1/§6.3), le
              moteur ne les lit jamais au moment de créditer une carte MEMORY. */}
          <div className="memory-config-item motion-card-memory-config-disabled" title="Sans effet en carte MEMOTION — le barème est STARS_PRORATA (étoiles au prorata des paires trouvées)">
            <label>Points par paire</label>
            <input type="number" value={config.pointsPerPair} disabled />
          </div>
          <div className="memory-config-item motion-card-memory-config-disabled" title="Sans effet en carte MEMOTION — le barème est STARS_PRORATA (étoiles au prorata des paires trouvées)">
            <label>Penalite erreur</label>
            <input type="number" value={config.errorPenalty} disabled />
          </div>
          <div className="memory-config-item motion-card-memory-config-disabled" title="Sans effet en carte MEMOTION — le barème est STARS_PRORATA (étoiles au prorata des paires trouvées)">
            <label>Bonus completion</label>
            <input type="number" value={config.completionBonus} disabled />
          </div>
          <div className="memory-config-item">
            <label>Temps memorisation (s)</label>
            <input
              type="number"
              value={config.memorizeTime}
              onChange={(e) => onConfigChange('memorizeTime', parseInt(e.target.value) || 5)}
              min="1"
              max="30"
            />
          </div>
          <div className="memory-config-item">
            <label>Delai reveal (s)</label>
            <input
              type="number"
              value={config.revealDelay}
              onChange={(e) => onConfigChange('revealDelay', parseFloat(e.target.value) || 0.5)}
              min="0.1"
              max="2"
              step="0.1"
            />
          </div>
        </div>
        <p className="motion-card-memory-config-hint">
          Points par paire / Penalite erreur / Bonus completion : sans effet sur une carte MEMOTION (barème STARS_PRORATA).
        </p>
        <div className="memory-config-toggle">
          <label>
            <input
              type="checkbox"
              checked={config.useTimer}
              onChange={(e) => onConfigChange('useTimer', e.target.checked)}
            />
            Utiliser le timer global
          </label>
          <span className="memory-config-hint">
            {config.useTimer
              ? 'Le jeu s\'arrete quand le temps est ecoule'
              : 'Pas de limite de temps, jeu jusqu\'a toutes les paires trouvees'}
          </span>
        </div>
        <div className="memory-config-toggle">
          <label>
            <input
              type="checkbox"
              checked={config.showDuringMemorize}
              onChange={(e) => onConfigChange('showDuringMemorize', e.target.checked)}
            />
            Afficher les cartes pendant la memorisation
          </label>
          <span className="memory-config-hint">
            {config.showDuringMemorize
              ? 'Les cartes sont visibles pendant le decompte'
              : 'Les cartes restent cachees jusqu\'au debut du jeu'}
          </span>
        </div>
      </div>
    </div>
  )
}
