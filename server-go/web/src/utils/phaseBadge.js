// #171/F2 — table phase -> {classe, libellé} pour le badge de statut affiché
// sur la ligne d'AnimAnswerZone (AnimPage.jsx). Reprend EXACTEMENT les
// classes CSS et libellés déjà définis dans Timer.jsx/Timer.css
// (`phase-badge phase-*`) — ces styles sont globaux (Timer.css est déjà
// importé sur `/anim` via <Timer>, donc disponibles ici sans nouvel import.
//
// Duplication assumée : Timer.jsx ne doit être touché QUE par ajout d'une
// option de rendu (contrainte explicite du plan #171, Timer est partagé
// avec /admin et /tv) — extraire cette table hors de Timer.jsx aurait été
// une retouche interne, pas un ajout. Le jeu de phases est petit et stable
// (6 entrées, alignées sur engine.go), le risque de divergence est faible.
export const PHASE_BADGE = {
  STOPPED: { className: 'phase-stopped', label: 'ARRET' },
  PAUSED: { className: 'phase-paused', label: 'PAUSE' },
  STARTED: { className: 'phase-running', label: 'EN COURS' },
  PREPARE: { className: 'phase-prepare', label: 'PREPARATION' },
  READY: { className: 'phase-ready', label: 'PRET' },
  REVEALED: { className: 'phase-revealed', label: 'REPONSE' },
  // COUNTDOWN — trou de couverture de #171 (table écrite avant l'arrivée
  // de MEMORY en #159, qui rend ce badge visible en pratique pendant la
  // mémorisation) : Timer.jsx n'a lui-même AUCUNE branche pour cette phase
  // (rien à en reprendre), mais GamePage.jsx (admin) a déjà son propre
  // badge — classe et libellé repris ici tels quels, pas une invention.
  // Voir AnimPage.css pour la règle `.phase-countdown` correspondante
  // (absente de Timer.css, ajoutée là par le même motif que GamePage.css:174).
  COUNTDOWN: { className: 'phase-countdown', label: 'COMPTE A REBOURS' },
}

export function getPhaseBadge(phase) {
  return PHASE_BADGE[phase] || null
}
