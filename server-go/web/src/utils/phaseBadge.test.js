import { describe, it, expect } from 'vitest'
import { getPhaseBadge, PHASE_BADGE } from './phaseBadge'

// ---------------------------------------------------------------------------
// phaseBadge — #171/F2, table phase -> {className, label} pour le badge de
// statut rendu sur la ligne réponse d'AnimPage.jsx. Duplication ASSUMÉE de
// la table interne à Timer.jsx (contrainte du plan : Timer.jsx n'est touché
// que par AJOUT d'une option, jamais retouché en interne — Timer est
// partagé /admin+/tv). Ce test fixe le contrat de cette table ET vérifie
// qu'elle reste synchronisée avec Timer.jsx (mêmes classes, mêmes libellés)
// — Timer.test.jsx (#171/T2) est la source de vérité côté Timer.jsx ; les
// deux fichiers doivent s'accorder, sinon la pastille "ligne réponse" et
// la pastille Timer (admin/tv) divergeraient silencieusement.
// ---------------------------------------------------------------------------

describe('getPhaseBadge', () => {
  it.each([
    ['STOPPED', 'phase-stopped', 'ARRET'],
    ['PAUSED', 'phase-paused', 'PAUSE'],
    ['STARTED', 'phase-running', 'EN COURS'],
    ['PREPARE', 'phase-prepare', 'PREPARATION'],
    ['READY', 'phase-ready', 'PRET'],
    ['REVEALED', 'phase-revealed', 'REPONSE'],
    // #159 (bugfix, commit 9f77bff) — trou de couverture #171 : la table
    // avait été écrite avant l'arrivée de MEMORY, qui rend ce badge visible
    // en pratique pendant la mémorisation. Timer.jsx n'a aucune branche pour
    // COUNTDOWN (rien à en reprendre côté "identique à Timer.jsx" comme les
    // autres lignes) — classe/libellé repris de GamePage.jsx:441 (admin), pas
    // une invention. Voir phaseBadge.js pour le commentaire complet.
    ['COUNTDOWN', 'phase-countdown', 'COMPTE A REBOURS'],
  ])('phase %s -> { className: %s, label: %s }', (phase, className, label) => {
    expect(getPhaseBadge(phase)).toEqual({ className, label })
  })

  it('retourne null pour une phase sans badge dédié (NEW_GAME, ENROLL)', () => {
    expect(getPhaseBadge('NEW_GAME')).toBeNull()
    expect(getPhaseBadge('ENROLL')).toBeNull()
  })

  it('retourne null pour une phase absente/inconnue, sans planter', () => {
    expect(getPhaseBadge(undefined)).toBeNull()
    expect(getPhaseBadge('')).toBeNull()
    expect(getPhaseBadge('PHASE_INCONNUE')).toBeNull()
  })

  // #159 — 6 -> 7 entrées (ajout de COUNTDOWN, cf. it.each ci-dessus).
  it('exactement 7 entrées dans la table (une par phase avec badge, pas plus)', () => {
    expect(Object.keys(PHASE_BADGE)).toHaveLength(7)
  })
})
