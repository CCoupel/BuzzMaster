// #310 (v12.1.0) — phrase centrée verticalement ET horizontalement : même règle pour l'aperçu de l'éditeur et la TV / tablette.
import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import RoueBoard from './RoueBoard'
import RoueGameBoard from './RoueGameBoard'
import { boardTopOffset } from './boardLayout'

const rowsOf = (cells, cols = 14) => Array.from({ length: 4 }, (_, r) => cells.slice(r * cols, (r + 1) * cols).map(c => c.textContent.trim()).join(''))

describe('boardTopOffset', () => {
  it('1 ou 2 lignes → décalage 1 ; 3 ou 4 lignes → 0 ; vide → 2', () => {
    expect([0, 1, 2, 3, 4].map(n => boardTopOffset(n))).toEqual([2, 1, 1, 0, 0])
  })
})

describe('aperçu de l\'éditeur (RoueBoard)', () => {
  it('2 lignes → lignes 2 et 3, chaque ligne centrée', () => {
    const { container } = render(<RoueBoard rows={['LA', 'VITRINE']} />)
    const lines = rowsOf(Array.from(container.querySelectorAll('.roue-cell')))
    expect(lines).toEqual(['', 'LA', 'VITRINE', ''])
    const row2 = Array.from(container.querySelectorAll('.roue-cell')).slice(28, 42)
    expect(row2.findIndex(c => c.textContent === 'V')).toBe(3) // (14-7)/2 = 3
  })
})

describe('TV / tablette (RoueGameBoard)', () => {
  it('2 lignes → lignes 2 et 3 (même placement que l\'aperçu)', () => {
    const { container } = render(<RoueGameBoard board={{ COLS: 14, ROWS: ['__', '_______'] }} />)
    const cells = Array.from(container.querySelectorAll('.rgb-cell'))
    expect(cells).toHaveLength(56)
    const tiles = (r) => cells.slice(r * 14, (r + 1) * 14).filter(c => c.classList.contains('tile')).length
    expect([0, 1, 2, 3].map(tiles)).toEqual([0, 2, 7, 0])
  })
  it('3 lignes → lignes 1 à 3 (inchangé)', () => {
    const { container } = render(<RoueGameBoard board={{ COLS: 14, ROWS: ['_', '_', '_'] }} />)
    const cells = Array.from(container.querySelectorAll('.rgb-cell'))
    const tiles = (r) => cells.slice(r * 14, (r + 1) * 14).filter(c => c.classList.contains('tile')).length
    expect([0, 1, 2, 3].map(tiles)).toEqual([1, 1, 1, 0])
  })
})
