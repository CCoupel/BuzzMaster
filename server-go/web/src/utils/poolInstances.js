/**
 * poolInstances — paramétrage par instance des composants de pool (contrat picto-pool.md §1, §4, §6, v13.1.0).
 *
 * Un seul jeu de composants (PoolPage, PoolAIGenerateModal, PoolAlert), instancié par une clé :
 *   - `rafale` : « Pool Speedy » (type RAFALE). Valeurs strictement identiques à l'ancien code en dur : l'écran,
 *     les libellés (y compris leur orthographe sans accent d'origine) et les chemins ne changent pas.
 *   - `picto`  : « Pool Picto » (types PICTO et RAFALE_PICTO). Préparé ici ; branché sur l'onglet de la page Questions
 *     au batch suivant (T13 onglets).
 *
 * Les routes Picto diffèrent de RAFALE (`words` au lieu de `questions`), d'où `listPath` / `itemPath` explicites.
 */
export const POOL_INSTANCES = {
  rafale: {
    key: 'rafale',
    target: 'RAFALE', // TARGET de AI_GENERATION_PROGRESS
    apiBase: '/api/rafale',
    listPath: '/questions',
    poolPath: '/pool',
    generatePath: '/generate-questions',
    resetAllPath: '/questions/reset-all',
    hasAnswer: true, // colonne et champ « Réponse »
    pasteList: false, // collage d'une liste (Pool Picto seulement)
    maxEntryRunes: 100, // énoncé (RAFALE) / mot (PICTO : 40, contrat §2)
    maxAnswerRunes: 40,
    generationPresets: [10, 20, 50, 100, 200],
    labels: {
      pageTitle: 'Pool Speedy',
      pageSubtitle: 'Questions texte seul, independantes du Quiz/MEMOTION',
      cardTitle: 'Questions du pool',
      entryColumn: 'Question',
      answerColumn: 'Reponse',
      entryField: 'Enonce',
      answerField: 'Reponse',
      formTitleNew: 'Nouvelle question',
      formTitleEdit: 'Modifier la question',
      noun: 'question', // « 3 questions · 1 utilisee · 2 disponibles »
      emptyFilter: 'Aucune question pour ce filtre.',
      required: 'Enonce et reponse sont obligatoires.',
      entryTooLong: (max) => `Enonce trop long (${max} caracteres max).`,
      answerTooLong: (max) => `Reponse trop longue (${max} caracteres max).`,
      confirmDelete: 'Supprimer cette question du Pool Speedy ?',
      confirmResetAll: (n) => `Remettre les ${n} question(s) utilisee(s) du Pool Speedy disponibles ? Le contenu des questions n'est pas modifie, seul le flag "deja utilisee" est efface.`,
      resetAllTitle: "Remet toutes les questions utilisees du Pool Speedy a l'etat disponible (le contenu des questions n'est pas modifie)",
      aiButtonTitle: 'Générer des questions via IA',
      aiModalTitle: '✨ Générer des questions pour le Pool Speedy',
      aiModalAria: 'Générer des questions pour le Pool Speedy',
      aiAddsTo: "Ce qu'on ajoute au réservoir",
      aiBatchLabel: 'Taille du lot',
      aiPoolName: 'réservoir',
      alertUnit: 'question',
    },
  },
  picto: {
    key: 'picto',
    target: 'PICTO',
    apiBase: '/api/picto',
    listPath: '/words',
    poolPath: '/pool',
    generatePath: '/generate-words',
    resetAllPath: '/words/reset-all',
    hasAnswer: false, // pas de colonne réponse (contrat §6)
    pasteList: true, // « Un mot » / « Une liste » (POST /words/batch, 1 à 200 mots)
    maxEntryRunes: 40,
    maxAnswerRunes: 0,
    generationPresets: [10, 20, 50],
    labels: {
      pageTitle: 'Pool Picto',
      pageSubtitle: 'Mots à dessiner, indépendants des questions',
      cardTitle: 'Mots du pool',
      entryColumn: 'Mot',
      answerColumn: '',
      entryField: 'Mot',
      answerField: '',
      formTitleNew: 'Nouveau mot',
      formTitleEdit: 'Modifier le mot',
      noun: 'mot',
      emptyFilter: 'Aucun mot pour ce filtre.',
      required: 'Le mot est obligatoire.',
      entryTooLong: (max) => `Mot trop long (${max} caractères max).`,
      answerTooLong: () => '',
      confirmDelete: 'Supprimer ce mot du Pool Picto ?',
      confirmResetAll: (n) => `Remettre les ${n} mot(s) utilisé(s) du Pool Picto disponibles ? Le contenu des mots n'est pas modifié, seul le marquage « utilisé » est effacé.`,
      resetAllTitle: "Remet tous les mots utilisés du Pool Picto à l'état disponible (le contenu des mots n'est pas modifié)",
      aiButtonTitle: 'Générer des mots via IA',
      aiModalTitle: '✨ Générer des mots pour le Pool Picto',
      aiModalAria: 'Générer des mots pour le Pool Picto',
      aiAddsTo: "Ce qu'on ajoute au Pool Picto",
      aiBatchLabel: 'Taille du lot',
      aiPoolName: 'Pool Picto',
      alertUnit: 'mot',
    },
  },
}

/** Instance par clé ; clé absente ou inconnue → `rafale` (comportement historique). */
export function getPoolInstance(key) {
  return POOL_INSTANCES[key] || POOL_INSTANCES.rafale
}

/** Chemins HTTP d'une instance (aucun chemin en dur dans les composants). */
export function poolUrls(inst) {
  const base = `${inst.apiBase}${inst.listPath}`
  return {
    list: base,
    item: (id) => `${base}/${encodeURIComponent(id)}`,
    reset: (id) => `${base}/${encodeURIComponent(id)}/reset`,
    resetAll: `${inst.apiBase}${inst.resetAllPath}`,
    batch: `${base}/batch`,
    pool: `${inst.apiBase}${inst.poolPath}`,
    generate: `${inst.apiBase}${inst.generatePath}`,
  }
}
