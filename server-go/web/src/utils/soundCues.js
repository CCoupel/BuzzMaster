// #231 — registre front des cues sonores : libellés (maquette
// ambiance_settings_panel__global_per_type_tabs) et onglets par type.
// Source de vérité de l'ordre et du `type` : GET /api/sounds (sound.md §2.1) ;
// ce module n'ajoute que les textes d'affichage. Une cue inconnue (serveur plus
// récent) s'affiche sous son nom brut.

export const CUE_LABELS = {
  depart: { label: 'Départ', sub: 'Le chronomètre démarre' },
  'temps-ecoule': { label: 'Temps écoulé', sub: 'Le chronomètre arrive à zéro' },
  gagne: { label: 'Points gagnés', sub: 'La régie crédite une équipe' },
  perdu: { label: 'Erreur', sub: 'Paire ratée en MEMORY, réponse invalidée en RAFALE' },
  reveal: { label: 'Révélation', sub: 'La réponse est dévoilée' },
  'entracte-debut': { label: "Début d'entracte", sub: 'La pause commence' },
  'entracte-fin': { label: "Fin d'entracte", sub: 'La partie reprend' },
  // #231 — cues par type de jeu
  'qcm-indice': { label: 'Indice révélé', sub: "Une réponse est barrée par l'indice (QCM)" },
  'memory-carte': { label: 'Carte retournée', sub: 'Une carte MEMORY est révélée' },
  'memory-paire': { label: 'Paire trouvée', sub: 'Les deux cartes correspondent (la paire ratée joue « Erreur », onglet Global)' },
  'memotion-selection': { label: 'Carte choisie', sub: "L'équipe sélectionne une carte de la grille" },
  'memotion-fin-carte': { label: 'Fin de carte', sub: 'Carte fermée sans gagnant (avec gagnant : « Points gagnés », onglet Global)' },
  'rafale-bonne': { label: 'Bonne réponse', sub: "L'animateur valide la réponse RAFALE" },
  'rafale-mauvaise': { label: 'Mauvaise réponse', sub: "L'animateur invalide (ou passe) la réponse" },
  'rafale-fin-lot': { label: 'Fin de lot', sub: 'Le réservoir est épuisé ou le temps du lot est écoulé' },
}

/** Onglets de bruitages : '' = Global (cues sans type). */
export const CUE_TABS = [
  { key: '', label: 'Global' },
  { key: 'QCM', label: 'QCM' },
  { key: 'MEMORY', label: 'MEMORY' },
  { key: 'MEMOTION', label: 'MEMOTION' },
  { key: 'RAFALE', label: 'RAFALE' },
]

export const cueMeta = (cue) => CUE_LABELS[cue] || { label: cue, sub: '' }
