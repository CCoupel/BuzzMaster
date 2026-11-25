import { describe, it, expect } from 'vitest'
import { mapSubmitError } from './aiJobHelpers'

// #324 — 409 picto_round_in_progress à la soumission de la génération (picto-pool.md §4) : message dédié.
describe('mapSubmitError — manche Picto en cours', () => {
  it('picto_round_in_progress : message explicite, pas le générique', () => {
    const r = mapSubmitError({ data: { code: 'picto_round_in_progress' } })
    expect(r.message).toMatch(/manche Picto est en cours/)
    expect(r.message).not.toBe('Erreur pendant la génération.')
    expect(r.showConfigLink).toBe(false)
  })
  it('rafale_round_in_progress inchangé', () => {
    expect(mapSubmitError({ data: { code: 'rafale_round_in_progress' } }).message).toMatch(/Rafale Speedy est en cours/)
  })
})
