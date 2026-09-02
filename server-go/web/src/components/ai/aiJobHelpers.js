// aiJobHelpers.js — helpers purs partagés par AIJobModalShell et les deux
// modales de génération IA (Quiz : AIGenerateModal.jsx, Rafale :
// RafaleAIGenerateModal.jsx).
//
// #203 (v8.1.0, tâche 10, GATE 2 §6bis) — extraits tels quels de
// AIGenerateModal.jsx (#8 v6.0.0, #137 v6.1.0), AUCUNE réécriture : ce
// fichier ne fait que déplacer 4 fonctions déjà entièrement génériques
// (aucune ne référence quoi que ce soit de propre au Quiz) hors du composant
// pour qu'elles servent aussi la modale Rafale — critère bloquant R10 du
// plan (_work/reports/plan-20260901-162105.md).

// #137 — messages ERROR_CODE (contract ai-multi-provider.md §10, réutilise
// ai-generation.md §3 + provider_quota). Utilisés pour l'état ARRÊTÉ/ÉCHEC du
// job asynchrone (pas de upstream_status disponible dans ce payload : le
// backend a déjà résolu 401/429 vers un code stable unique).
const JOB_ERROR_MESSAGES = {
  no_api_key: 'Clé API invalide ou absente. Vérifiez-la dans Paramètres.',
  invalid_request: 'Requête invalide.',
  upstream_error: "Le serveur n'a pas pu joindre le provider IA.",
  timeout: 'Un lot a dépassé le temps imparti.',
  id_exhausted: "Plus d'identifiant disponible pour de nouvelles questions.",
  provider_quota: 'Quota quotidien du provider atteint. Réessayez demain.',
}

export function jobErrorMessage(errorCode) {
  return JOB_ERROR_MESSAGES[errorCode] || 'Erreur pendant la génération.'
}

export function providerLabel(provider) {
  if (provider === 'groq') return 'Groq (gratuit)'
  if (provider === 'anthropic') return 'Claude (Anthropic)'
  return provider || '—'
}

// Traduit le rejet **synchrone** de la soumission (avant même qu'un job
// existe : 400/405/409/507, contract ai-multi-provider.md §9) en message
// lisible. Distinct de jobErrorMessage ci-dessus, qui couvre l'échec
// **asynchrone** d'un job déjà démarré (ERROR_CODE de AI_GENERATION_PROGRESS).
//
// #203 — `rafale_round_in_progress` (contract rafale-ai-generation.md §2,
// §7) ajouté ici plutôt que dupliqué dans une variante Rafale : cette
// fonction est déjà partagée par construction (aucun code Quiz-only), et un
// futur 3e chemin de génération n'aurait besoin que d'y ajouter son propre
// code d'erreur.
export function mapSubmitError({ networkFailure, data }) {
  if (networkFailure) {
    return { message: "Le serveur n'a pas pu être joint. Vérifiez l'accès réseau.", showConfigLink: false, detail: null }
  }
  const code = data?.code
  if (code === 'no_api_key') {
    return { message: 'Clé API invalide ou absente. Vérifiez-la dans Paramètres.', showConfigLink: true, detail: null }
  }
  if (code === 'generation_in_progress') {
    // Ne devrait pas arriver en pratique : le bouton ré-attache déjà à
    // EnCours si un job tourne. Filet de sécurité si l'état a changé entre
    // l'ouverture de la modale et la soumission (second onglet/admin).
    return { message: 'Une génération est déjà en cours. Fermez et rouvrez la fenêtre pour suivre sa progression.', showConfigLink: false, detail: null }
  }
  if (code === 'rafale_round_in_progress') {
    return { message: 'Génération refusée : une manche RAFALE est en cours. Réessayez une fois la manche terminée.', showConfigLink: false, detail: null }
  }
  return { message: 'Erreur pendant la génération.', showConfigLink: false, detail: data?.message || null }
}

export function clampInt(raw, min, max, fallback) {
  const n = parseInt(raw, 10)
  if (Number.isNaN(n)) return fallback
  return Math.max(min, Math.min(max, n))
}

// Un `aiJob` (singleton global, useWebSocket.js) est pertinent pour une
// modale donnée seulement si son `target` correspond à la cible de CETTE
// modale — absent (serveur antérieur à #203) équivaut à "QUIZ" (contract
// rafale-ai-generation.md §6, additif/rétrocompatible).
export function matchesTarget(aiJob, target) {
  return !!aiJob && (aiJob.target || 'QUIZ') === target
}
