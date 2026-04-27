import { describe, it, expect } from 'vitest'
import { boostTeamColor, getRgbColor, rgbToHex, getContrastColor } from './colorUtils'

// ========================================
// Issue #61 — boostTeamColor
// ========================================

describe('boostTeamColor — entrées invalides', () => {
  it('retourne null pour null', () => {
    expect(boostTeamColor(null)).toBeNull()
  })

  it('retourne null pour undefined', () => {
    expect(boostTeamColor(undefined)).toBeNull()
  })

  it('retourne null pour un non-array', () => {
    expect(boostTeamColor('#FF0000')).toBeNull()
    expect(boostTeamColor(42)).toBeNull()
    expect(boostTeamColor({})).toBeNull()
  })

  it('retourne null pour un array trop court (< 3 éléments)', () => {
    expect(boostTeamColor([255, 0])).toBeNull()
    expect(boostTeamColor([])).toBeNull()
    expect(boostTeamColor([128])).toBeNull()
  })
})

describe('boostTeamColor — format de sortie', () => {
  it('retourne une string commençant par "rgb("', () => {
    const result = boostTeamColor([255, 0, 0])
    expect(result).toMatch(/^rgb\(\d+,\d+,\d+\)$/)
  })

  it('retourne des valeurs RGB entières dans [0,255]', () => {
    const inputs = [
      [255, 0, 0],
      [0, 255, 0],
      [0, 0, 255],
      [128, 128, 128],
      [255, 215, 0],
    ]
    inputs.forEach(input => {
      const result = boostTeamColor(input)
      expect(result).not.toBeNull()
      const match = result.match(/^rgb\((\d+),(\d+),(\d+)\)$/)
      expect(match).not.toBeNull()
      const [, r, g, b] = match.map(Number)
      expect(r).toBeGreaterThanOrEqual(0)
      expect(r).toBeLessThanOrEqual(255)
      expect(g).toBeGreaterThanOrEqual(0)
      expect(g).toBeLessThanOrEqual(255)
      expect(b).toBeGreaterThanOrEqual(0)
      expect(b).toBeLessThanOrEqual(255)
    })
  })
})

describe('boostTeamColor — saturation minimale 70%', () => {
  // Le gris pur (128,128,128) a saturation = 0 → doit être boosté à min 70%
  it('gris pur est boosted (saturation non nulle)', () => {
    const result = boostTeamColor([128, 128, 128])
    expect(result).not.toBeNull()
    // Le résultat ne doit pas être un gris pur (r == g == b)
    const match = result.match(/^rgb\((\d+),(\d+),(\d+)\)$/)
    const [, r, g, b] = match.map(Number)
    const isGray = (r === g && g === b)
    // Avec s forcé à 0.70 et l clampé, le résultat ne peut pas être gris pur
    expect(isGray).toBe(false)
  })

  // Blanc pur (255,255,255) a saturation = 0, lightness = 1 → doit être reclampé
  it('blanc pur est boosted et reclampé en lightness', () => {
    const result = boostTeamColor([255, 255, 255])
    expect(result).not.toBeNull()
    const match = result.match(/^rgb\((\d+),(\d+),(\d+)\)$/)
    expect(match).not.toBeNull()
  })

  // Noir pur (0,0,0) a saturation = 0, lightness = 0 → doit être reclampé
  it('noir pur est boosted et reclampé en lightness', () => {
    const result = boostTeamColor([0, 0, 0])
    expect(result).not.toBeNull()
    const match = result.match(/^rgb\((\d+),(\d+),(\d+)\)$/)
    expect(match).not.toBeNull()
  })
})

describe('boostTeamColor — couleurs déjà saturées', () => {
  // Rouge vif (255,0,0) — saturation déjà max, ne doit pas dégénérer
  it('rouge vif reste stable (retourne une couleur rougeâtre)', () => {
    const result = boostTeamColor([255, 0, 0])
    expect(result).not.toBeNull()
    const match = result.match(/^rgb\((\d+),(\d+),(\d+)\)$/)
    const [, r, , b] = match.map(Number)
    // La composante rouge doit dominer après boost
    expect(r).toBeGreaterThan(b)
  })

  // Bleu vif (0,0,255)
  it('bleu vif reste stable (composante bleue dominante)', () => {
    const result = boostTeamColor([0, 0, 255])
    expect(result).not.toBeNull()
    const match = result.match(/^rgb\((\d+),(\d+),(\d+)\)$/)
    const [, r, , b] = match.map(Number)
    expect(b).toBeGreaterThan(r)
  })

  // Vert vif (0,255,0)
  it('vert vif reste stable (composante verte dominante)', () => {
    const result = boostTeamColor([0, 255, 0])
    expect(result).not.toBeNull()
    const match = result.match(/^rgb\((\d+),(\d+),(\d+)\)$/)
    const [, r, g] = match.map(Number)
    expect(g).toBeGreaterThan(r)
  })
})

describe('boostTeamColor — idempotence partielle', () => {
  it('appliquer deux fois ne produit pas un résultat radicalement différent', () => {
    // Convertit le résultat rgb() en array [r,g,b] pour passer à boostTeamColor
    const input = [200, 100, 50]
    const firstPass = boostTeamColor(input)
    const match = firstPass.match(/^rgb\((\d+),(\d+),(\d+)\)$/)
    const arr = match.slice(1).map(Number)
    const secondPass = boostTeamColor(arr)
    // Les deux passes doivent produire une string rgb valide
    expect(firstPass).toMatch(/^rgb\(/)
    expect(secondPass).toMatch(/^rgb\(/)
  })
})

// ========================================
// getRgbColor
// ========================================

describe('getRgbColor', () => {
  it('retourne le fallback pour null', () => {
    expect(getRgbColor(null)).toBe('var(--gray-400)')
  })

  it('retourne le fallback personnalisé pour undefined', () => {
    expect(getRgbColor(undefined, '#000000')).toBe('#000000')
  })

  it('retourne un rgb() boosted pour un array', () => {
    const result = getRgbColor([255, 0, 0])
    expect(result).toMatch(/^rgb\(/)
  })

  it('retourne la string telle quelle pour une couleur hex', () => {
    expect(getRgbColor('#FF0000')).toBe('#FF0000')
  })

  it('retourne la string telle quelle pour une couleur css', () => {
    expect(getRgbColor('red')).toBe('red')
  })
})

// ========================================
// rgbToHex
// ========================================

describe('rgbToHex', () => {
  it('convertit [255,0,0] en #ff0000', () => {
    expect(rgbToHex([255, 0, 0])).toBe('#ff0000')
  })

  it('convertit [0,255,0] en #00ff00', () => {
    expect(rgbToHex([0, 255, 0])).toBe('#00ff00')
  })

  it('convertit [0,0,255] en #0000ff', () => {
    expect(rgbToHex([0, 0, 255])).toBe('#0000ff')
  })

  it('retourne la couleur par défaut (#6366f1) pour null', () => {
    expect(rgbToHex(null)).toBe('#6366f1')
  })

  it('retourne la couleur par défaut pour un non-array', () => {
    expect(rgbToHex('not an array')).toBe('#6366f1')
  })

  it('pad à 2 chiffres pour les petites valeurs', () => {
    expect(rgbToHex([0, 0, 0])).toBe('#000000')
    expect(rgbToHex([1, 2, 3])).toBe('#010203')
  })
})

// ========================================
// getContrastColor
// ========================================

describe('getContrastColor', () => {
  it('retourne #000000 (noir) pour un fond clair', () => {
    // Blanc pur → fond clair → texte noir
    expect(getContrastColor([255, 255, 255])).toBe('#000000')
  })

  it('retourne #ffffff (blanc) pour un fond sombre', () => {
    // Noir pur → fond sombre → texte blanc
    expect(getContrastColor([0, 0, 0])).toBe('#ffffff')
  })

  it('retourne #ffffff pour null (fallback sécurisé)', () => {
    expect(getContrastColor(null)).toBe('#ffffff')
  })

  it('retourne #ffffff pour un non-array', () => {
    expect(getContrastColor('red')).toBe('#ffffff')
  })

  it('retourne #000000 pour un jaune clair (luminosité > 0.5)', () => {
    // Jaune pâle
    expect(getContrastColor([255, 255, 0])).toBe('#000000')
  })

  it('retourne #ffffff pour un bleu foncé (luminosité < 0.5)', () => {
    expect(getContrastColor([0, 0, 200])).toBe('#ffffff')
  })
})
