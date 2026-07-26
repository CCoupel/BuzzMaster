import { describe, it, expect } from 'vitest'
import { TEAM_COLORS, getNextTeamColor, findTeamColor } from './colors'

// ---------------------------------------------------------------------------
// Tests : palette de 16 couleurs d'équipe (#113)
//
// Table normative : contracts/models.md § "Palette d'équipes (#113)". TEAM_COLORS
// est la source de vérité frontend — ce fichier vérifie sa conformité à la table,
// et le comportement de getNextTeamColor/findTeamColor qui s'appuient dessus.
// ---------------------------------------------------------------------------

// Copie de contrôle de la table contractuelle (contracts/models.md), volontairement
// indépendante de constants/colors.js — détecte toute dérive de TEAM_COLORS.
const CONTRACT_TABLE = [
  { rank: 1, key: 'rouge', rgb: [255, 26, 26] },
  { rank: 2, key: 'orange', rgb: [255, 133, 26] },
  { rank: 3, key: 'jaune', rgb: [255, 217, 26] },
  { rank: 4, key: 'vert', rgb: [26, 255, 83] },
  { rank: 5, key: 'cyan', rgb: [26, 236, 255] },
  { rank: 6, key: 'bleu', rgb: [26, 94, 255] },
  { rank: 7, key: 'violet', rgb: [159, 26, 255] },
  { rank: 8, key: 'rose', rgb: [255, 26, 159] },
  { rank: 9, key: 'rouge-profond', rgb: [179, 0, 0] },
  { rank: 10, key: 'orange-profond', rgb: [179, 83, 0] },
  { rank: 11, key: 'jaune-profond', rgb: [179, 149, 0] },
  { rank: 12, key: 'vert-profond', rgb: [0, 179, 45] },
  { rank: 13, key: 'cyan-profond', rgb: [0, 164, 179] },
  { rank: 14, key: 'bleu-profond', rgb: [0, 54, 179] },
  { rank: 15, key: 'violet-profond', rgb: [104, 0, 179] },
  { rank: 16, key: 'rose-profond', rgb: [179, 0, 104] },
]

describe('TEAM_COLORS — conformité à la table contractuelle (#113)', () => {
  it('contient exactement 16 entrées', () => {
    expect(TEAM_COLORS).toHaveLength(16)
  })

  it("respecte l'ordre d'attribution (rang 1→16) et les valeurs RGB de contracts/models.md", () => {
    CONTRACT_TABLE.forEach((expected, index) => {
      const actual = TEAM_COLORS[index]
      expect(actual, `rang ${expected.rank} (index ${index})`).toBeDefined()
      expect(actual.key, `rang ${expected.rank}`).toBe(expected.key)
      expect(actual.rgb, `rang ${expected.rank} (${expected.key})`).toEqual(expected.rgb)
    })
  })

  it('a des clés deux à deux distinctes', () => {
    const keys = TEAM_COLORS.map(c => c.key)
    expect(new Set(keys).size).toBe(keys.length)
  })

  it('a des valeurs RGB deux à deux distinctes', () => {
    const rgbStrings = TEAM_COLORS.map(c => c.rgb.join(','))
    expect(new Set(rgbStrings).size).toBe(rgbStrings.length)
  })

  it('marque les 8 premières entrées comme vives (deep=false) et les 8 suivantes comme profondes (deep=true)', () => {
    TEAM_COLORS.slice(0, 8).forEach(c => expect(c.deep, c.key).toBe(false))
    TEAM_COLORS.slice(8).forEach(c => expect(c.deep, c.key).toBe(true))
  })
})

describe('getNextTeamColor — attribution déterministe (#113)', () => {
  it('attribue le rang 1 quand aucune équipe n\'existe', () => {
    expect(getNextTeamColor({}).key).toBe('rouge')
  })

  it('16 créations successives attribuent les rangs 1→16, sans doublon', () => {
    let teams = {}
    const attributed = []
    for (let i = 0; i < 16; i++) {
      const next = getNextTeamColor(teams)
      attributed.push(next.key)
      teams = { ...teams, [`Team${i}`]: { COLOR_NAME: next.key } }
    }
    expect(attributed).toEqual(TEAM_COLORS.map(c => c.key))
    expect(new Set(attributed).size).toBe(16)
  })

  it('la 17ᵉ équipe recycle le rang 1', () => {
    let teams = {}
    for (let i = 0; i < 16; i++) {
      const next = getNextTeamColor(teams)
      teams = { ...teams, [`Team${i}`]: { COLOR_NAME: next.key } }
    }
    // All 16 taken — the 17th creation must recycle rank 1.
    expect(getNextTeamColor(teams).key).toBe('rouge')
  })

  it('comble un trou laissé par une équipe supprimée', () => {
    // 3 équipes occupent rangs 1, 2, 3 — l'équipe du rang 2 ("orange") est supprimée.
    const teams = {
      TeamA: { COLOR_NAME: 'rouge' },
      TeamC: { COLOR_NAME: 'jaune' },
    }
    // Rang 2 ("orange") est libre — doit être attribué avant de continuer au rang 4.
    expect(getNextTeamColor(teams).key).toBe('orange')
  })
})

describe('findTeamColor — résolution par clé ou repli par RGB (#113)', () => {
  it('résout par COLOR_NAME quand présent', () => {
    const found = findTeamColor('bleu-profond', [0, 0, 0] /* RGB volontairement différent */)
    expect(found?.key).toBe('bleu-profond')
    expect(found?.rgb).toEqual([0, 54, 179])
  })

  it("retombe sur une résolution par RGB pour une équipe antérieure à la feature (pas de COLOR_NAME)", () => {
    const found = findTeamColor(undefined, [26, 94, 255]) // bleu vif exact
    expect(found?.key).toBe('bleu')
  })

  it('retourne null si ni la clé ni le RGB ne correspondent à une entrée de la palette', () => {
    expect(findTeamColor('couleur-inconnue', [1, 2, 3])).toBeNull()
    expect(findTeamColor(undefined, undefined)).toBeNull()
  })
})
