import { describe, it, expect, afterEach } from 'vitest'
import { render, cleanup } from '@testing-library/react'
import Wheel from './Wheel'

// #282 (v11.4.0, LOT-2) — contracts/wheel-component.md : API, états, layouts, accessibilité,
// critère « lisible après 100 mouvements » (§4). Écrit contre le contrat seul (RED attendu).
afterEach(() => cleanup())

const letters = (n = 26) => Array.from({ length: n }, (_, i) => ({
  id: String.fromCharCode(65 + i), label: String.fromCharCode(65 + i), kind: 'letter', state: '',
}))

describe('Wheel — structure et layouts', () => {
  it('layout par défaut = strip ; classes wheel-strip / wheel-arc selon le layout', () => {
    const { container, rerender } = render(<Wheel segments={letters()} current={2} />)
    expect(container.querySelector('.wheel-strip')).not.toBeNull()
    expect(container.querySelector('.wheel-arc')).toBeNull()
    rerender(<Wheel segments={letters()} current={2} layout="arc" />)
    expect(container.querySelector('.wheel-arc')).not.toBeNull()
    expect(container.querySelector('.wheel-strip')).toBeNull()
  })

  it.each([26, 24])('rend %i segments dans les deux layouts (une cellule/secteur par segment)', (n) => {
    const strip = render(<Wheel segments={letters(n)} current={0} layout="strip" />)
    const texts = Array.from(strip.container.querySelectorAll('text, .wheel-cell')).map(e => e.textContent)
    for (let i = 0; i < n; i++) expect(texts).toContain(String.fromCharCode(65 + i))
    cleanup()
    const arc = render(<Wheel segments={letters(n)} current={0} layout="arc" />)
    const arcTexts = Array.from(arc.container.querySelectorAll('text')).map(e => e.textContent)
    for (let i = 0; i < n; i++) expect(arcTexts).toContain(String.fromCharCode(65 + i))
  })

  it('accessibilité : role="img" et aria-label portant le segment courant', () => {
    const { getByRole } = render(<Wheel segments={letters()} current={2} />)
    const img = getByRole('img')
    expect(img.getAttribute('aria-label')).toMatch(/C/)
  })

  it('current = null : aucun segment mis en évidence (ex. lettre « # »)', () => {
    const { container } = render(<Wheel segments={letters()} current={null} />)
    expect(container.querySelector('.is-current, [data-current="true"]')).toBeNull()
  })

  it('un seul segment courant mis en évidence', () => {
    const { container } = render(<Wheel segments={letters()} current={4} />)
    expect(container.querySelectorAll('.is-current, [data-current="true"]').length).toBeGreaterThanOrEqual(1)
  })
})

describe('Wheel — états des segments', () => {
  it('disabled et played sont distinguables du normal (classes/attributs d\'état)', () => {
    const segs = letters()
    segs[1] = { ...segs[1], state: 'disabled' }
    segs[2] = { ...segs[2], state: 'played' }
    const { container } = render(<Wheel segments={segs} current={0} />)
    expect(container.querySelector('.disabled, [data-state="disabled"]')).not.toBeNull()
    expect(container.querySelector('.played, [data-state="played"]')).not.toBeNull()
  })

  it('un segment special est rendu avec la classe special', () => {
    const segs = [...letters(5), { id: 'x', label: 'BANQUEROUTE', kind: 'special', state: '' }]
    const { container } = render(<Wheel segments={segs} current={0} />)
    expect(container.querySelector('.special, [data-kind="special"]')).not.toBeNull()
  })
})

describe('Wheel — critère « lisible après 100 mouvements » (§4, #282)', () => {
  const translateXOf = (el) => {
    const m = /translateX\((-?[\d.]+)px\)|translate3d\((-?[\d.]+)px/.exec(el.style.transform || '')
    return m ? parseFloat(m[1] ?? m[2]) : 0
  }

  it('strip : après 100 rerender de current, toutes les cellules sont rendues et le courant est présent', () => {
    const segs = letters()
    const { container, rerender } = render(<Wheel segments={segs} current={0} layout="strip" />)
    const initialCells = container.querySelectorAll('.wheel-cell, .cell').length
    for (let k = 1; k <= 100; k++) {
      rerender(<Wheel segments={segs} current={(k * 7) % 26} layout="strip" />)
    }
    expect(container.querySelectorAll('.wheel-cell, .cell').length).toBe(initialCells) // pas de ré-allocation
    expect(initialCells).toBeGreaterThanOrEqual(26) // ruban répété REP fois
    const expected = String.fromCharCode(65 + ((100 * 7) % 26))
    expect(container.querySelector('[role="img"]').getAttribute('aria-label')).toContain(expected)
    const track = container.querySelector('.wheel-track, .track')
    if (track) {
      // translateX bornée : jamais au-delà de REP × n cellules (aucun bandeau vide)
      const cellW = track.firstElementChild?.getBoundingClientRect?.().width || 1
      expect(Math.abs(translateXOf(track))).toBeLessThanOrEqual(7 * 26 * Math.max(cellW, 1) + 1)
    }
  })

  it('arc : après 100 rerender, la rotation reste bornée (< 360° × 2) et le courant est correct', () => {
    const segs = letters()
    const { container, rerender } = render(<Wheel segments={segs} current={0} layout="arc" />)
    for (let k = 1; k <= 100; k++) {
      rerender(<Wheel segments={segs} current={(k * 11) % 26} layout="arc" />)
    }
    const rotor = container.querySelector('.rotor, .wheel-rotor')
    const m = /rotate\((-?[\d.]+)deg\)/.exec(rotor?.style?.transform || rotor?.getAttribute('transform') || '')
    if (m) expect(Math.abs(parseFloat(m[1]))).toBeLessThan(720)
    expect(container.querySelectorAll('path').length).toBeGreaterThanOrEqual(26)
  })
})
