import { describe, it, expect } from 'vitest'
import * as colors from './colors'

// #285 — la table CATEGORIES de constants/colors.js (doublon de categoryUtils.js) est SUPPRIMÉE au profit d'un import
// unique ; getCategoryColor(key, categories) prend la liste API en second argument.
describe('constants/colors — source unique (#285)', () => {
  it('le doublon CATEGORIES n\'est plus exporté ici', () => {
    expect(colors.CATEGORIES).toBeUndefined()
  })
  it('getCategoryColor reste exporté et DEFAULT_NEON_COLOR inchangé', () => {
    expect(typeof colors.getCategoryColor).toBe('function')
    expect(colors.DEFAULT_NEON_COLOR).toBe('#6b7280')
  })
})
