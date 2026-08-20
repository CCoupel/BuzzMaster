import { describe, it, expect } from 'vitest'
import { canToggleEntracte } from './phaseRules'

// ---------------------------------------------------------------------------
// canToggleEntracte — mode ENTRACTE (#119), tâche F6/T4 du plan
// _work/reports/plan-entracte-119-20260820-140825.md.
//
// Source UNIQUE de la règle d'activation du bouton ENTRACTE sur /admin
// (GamePage.jsx), miroir exact de la garde moteur `SetEntracte` (D4/B2) :
// entrer en entracte n'est autorisé que si aucune question n'est en cours.
// Signature réelle : canToggleEntracte(phase, entracteActive) — le second
// paramètre encode directement "la sortie est toujours permise" (si déjà
// actif, retourne true sans même regarder la phase), pas une logique
// séparée à recomposer côté appelant.
// ---------------------------------------------------------------------------

describe('canToggleEntracte — table exhaustive des phases, ENTRÉE (D4, entracteActive=false)', () => {
  it.each([
    ['STOPPED', true],
    ['PREPARE', true],
    ['READY', true],
    ['NEW_GAME', true],
    ['REVEALED', true],
    ['COUNTDOWN', false],
    ['STARTED', false],
    ['PAUSED', false],
    ['ENROLL', false],
  ])('phase %s → autorisée=%s', (phase, expected) => {
    expect(canToggleEntracte(phase, false)).toBe(expected)
  })

  it('une phase inconnue/absente est refusée par défaut (fermé, pas ouvert)', () => {
    expect(canToggleEntracte(undefined, false)).toBe(false)
    expect(canToggleEntracte('', false)).toBe(false)
    expect(canToggleEntracte('UNE_PHASE_FUTURE_INCONNUE', false)).toBe(false)
  })
})

describe('canToggleEntracte — la SORTIE est toujours permise (entracteActive=true), quelle que soit la phase', () => {
  it.each(['STOPPED', 'PREPARE', 'READY', 'NEW_GAME', 'REVEALED', 'COUNTDOWN', 'STARTED', 'PAUSED', 'ENROLL', 'UNE_PHASE_FUTURE_INCONNUE'])(
    'phase %s, entracteActive=true → toujours autorisée (jamais de blocage définitif)',
    (phase) => {
      expect(canToggleEntracte(phase, true)).toBe(true)
    }
  )
})
