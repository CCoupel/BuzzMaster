/**
 * #145 (v12.0.0, LOT-1.1) — RoueBoard : plateau 4 × 14 de l'aperçu (contracts/roue.md §3.1, maquette M3).
 * Affichage pur : les lignes viennent du serveur, aucune règle de mise en lignes n'est recopiée ici.
 */
import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import RoueBoard from './RoueBoard'

const cells = (c) => Array.from(c.querySelectorAll('.roue-cell'))
const letters = (c) => cells(c).filter(e => e.textContent.trim() !== '')

describe('RoueBoard (#145)', () => {
  it('affiche toujours 4 × 14 = 56 cases, même vide', () => {
    const { container } = render(<RoueBoard rows={[]} />)
    expect(cells(container)).toHaveLength(56)
    expect(letters(container)).toHaveLength(0)
  })

  it('place les lettres des lignes fournies (exemple de référence)', () => {
    const rows = ['COMBIEN POUR', 'CE CHIEN DANS', 'LA VITRINE']
    const { container } = render(<RoueBoard rows={rows} />)
    expect(cells(container)).toHaveLength(56)
    // 10 + 11 + 9 lettres (espaces exclus) : COMBIEN(7) POUR(4) CE(2) CHIEN(5) DANS(4) LA(2) VITRINE(7) = 31
    expect(letters(container)).toHaveLength(31)
  })

  it('lit les lettres ligne par ligne dans l\'ordre', () => {
    const { container } = render(<RoueBoard rows={['AB CD']} />)
    expect(letters(container).map(e => e.textContent).join('')).toBe('ABCD')
  })

  it('centre chaque ligne sur 14 colonnes', () => {
    // « LA » (2) : décalage (14-2)/2 = 6 → première lettre en colonne 7 (index 6) de la ligne
    const { container } = render(<RoueBoard rows={['LA']} />)
    // #310 (v12.1.0) : la phrase est aussi centrée verticalement — 1 ligne → ligne 2 sur 4 (index 1)
    const first = cells(container).slice(14, 28)
    expect(first.findIndex(e => e.textContent === 'L')).toBe(6)
    expect(first.findIndex(e => e.textContent === 'A')).toBe(7)
  })

  it('distingue voyelles (dont Y) et consonnes', () => {
    const { container } = render(<RoueBoard rows={['AEIOUY BCD']} />)
    const byLetter = Object.fromEntries(letters(container).map(e => [e.textContent, e.className]))
    for (const v of 'AEIOUY') expect(byLetter[v]).toMatch(/vowel/)
    for (const k of 'BCD') expect(byLetter[k]).not.toMatch(/vowel/)
  })

  it('ne dépasse pas 4 lignes même si on lui en donne davantage (garde d\'affichage)', () => {
    const { container } = render(<RoueBoard rows={['A', 'B', 'C', 'D', 'E']} />)
    expect(cells(container)).toHaveLength(56)
    expect(letters(container).map(e => e.textContent).join('')).toBe('ABCD')
  })

  it('apostrophe, tiret, chiffres et ponctuation sont des cases visibles', () => {
    const { container } = render(<RoueBoard rows={["L'AVOIR 21 !"]} />)
    expect(letters(container).map(e => e.textContent).join('')).toBe("L'AVOIR21!")
  })

  it('expose un libellé accessible du plateau', () => {
    const { container } = render(<RoueBoard rows={['LA VITRINE']} />)
    expect(container.querySelector('[role="img"]').getAttribute('aria-label')).toMatch(/LA VITRINE/)
  })
})
