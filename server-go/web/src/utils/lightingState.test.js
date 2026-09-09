import { describe, it, expect } from 'vitest'
import {
  LIGHTING_STATES,
  normalizeLightingState,
  lightingStateLabel,
  lightingStateTitle,
  lightingStateGlyph,
} from './lightingState'

// ---------------------------------------------------------------------------
// #207 — table de correspondance état -> libellé/title/glyphe (contrat
// hue-bridge.md §5.6 + §7.1, maquette §01 rév. 4). C'est la SEULE source des
// libellés pour Navbar et AmbiancePage : verrouillée ici ligne par ligne pour
// que les deux ne puissent jamais diverger sans faire échouer un test.
// ---------------------------------------------------------------------------

describe('lightingState — table normative (contrat §5.6/§7.1)', () => {
  const table = [
    { state: 'ok', label: 'Pont connecté', title: 'Éclairage : pont connecté', glyph: 'lit' },
    { state: 'unreachable', label: 'Pont injoignable', title: 'Éclairage : pont injoignable', glyph: 'alert' },
    { state: 'refused', label: 'Association refusée', title: 'Éclairage : association refusée', glyph: 'alert' },
    { state: 'disabled', label: 'Non configuré', title: 'Éclairage : non configuré', glyph: 'off' },
  ]

  it.each(table)('$state -> label=$label title=$title glyph=$glyph', ({ state, label, title, glyph }) => {
    expect(lightingStateLabel(state)).toBe(label)
    expect(lightingStateTitle(state)).toBe(title)
    expect(lightingStateGlyph(state)).toBe(glyph)
  })

  it("« refused » et « unreachable » partagent le glyphe d'alerte MAIS restent des libellés distincts (contrat §5.6 : jamais fondus)", () => {
    expect(lightingStateGlyph('refused')).toBe(lightingStateGlyph('unreachable'))
    expect(lightingStateLabel('refused')).not.toBe(lightingStateLabel('unreachable'))
    expect(lightingStateTitle('refused')).not.toBe(lightingStateTitle('unreachable'))
  })

  it("LIGHTING_STATES énumère exactement les 4 valeurs, dans l'ordre du contrat", () => {
    expect(LIGHTING_STATES).toEqual(['ok', 'unreachable', 'refused', 'disabled'])
  })
})

describe('normalizeLightingState — robustesse (endpoint absent, valeur inconnue)', () => {
  it.each(['ok', 'unreachable', 'refused', 'disabled'])('%s reste inchangé', (s) => {
    expect(normalizeLightingState(s)).toBe(s)
  })

  it.each([undefined, null, '', 'weird', 'OK', 42, {}])('%s (valeur inconnue) -> disabled', (v) => {
    expect(normalizeLightingState(v)).toBe('disabled')
  })
})
