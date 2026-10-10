import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { getArdoiseTvLayout } from './ardoiseTvLayout'

// #250 — règle de la maquette tv_ardoise_reveal__adaptive_grid.

describe('getArdoiseTvLayout (#250)', () => {
  it.each([
    [1, 1, 1, 2], [4, 1, 4, 2], [5, 1, 5, 2],
    [6, 2, 3, 2], [8, 2, 4, 2], [10, 2, 5, 2], [12, 2, 6, 2],
    [13, 2, 7, 1], [16, 2, 8, 1],
    [17, 3, 6, 1], [20, 3, 7, 1],
  ])('N=%i → %i lignes, %i colonnes, %i ligne(s) de texte', (n, rows, cols, lines) => {
    expect(getArdoiseTvLayout(n)).toMatchObject({ rows, cols, lines })
  })

  it('assez de cellules pour toutes les équipes, jamais de défilement nécessaire (jusqu\'à 30)', () => {
    for (let n = 1; n <= 30; n++) {
      const L = getArdoiseTvLayout(n)
      expect(L.rows * L.cols).toBeGreaterThanOrEqual(n)
    }
  })

  it('le texte rétrécit quand les équipes sont nombreuses', () => {
    expect(getArdoiseTvLayout(16).answerFont).toBeLessThan(getArdoiseTvLayout(4).answerFont)
    expect(getArdoiseTvLayout(4).answerFont).toBeLessThanOrEqual(0.026)
  })

  it('0 ou valeur invalide : repli sur une équipe', () => {
    expect(getArdoiseTvLayout(0)).toMatchObject({ rows: 1, cols: 1 })
    expect(getArdoiseTvLayout(NaN)).toMatchObject({ rows: 1, cols: 1 })
  })
})

describe('CSS TV ARDOISE statique (#250)', () => {
  // vitest s'exécute depuis server-go/web
  const css = readFileSync(resolve(process.cwd(), 'src/pages/PlayerDisplay.css'), 'utf8')
  const block = (sel) => {
    const i = css.indexOf(`\n${sel} {`)
    expect(i, `${sel} introuvable`).toBeGreaterThan(-1)
    return css.slice(i, css.indexOf('}', i))
  }

  it.each(['.ardoise-footer-bottom', '.ardoise-teams-grid', '.ardoise-team-card', '.ardoise-team-card-answer'])(
    '%s : overflow hidden, jamais auto/scroll',
    (sel) => {
      const b = block(sel)
      expect(b).toMatch(/overflow:\s*hidden/)
      expect(b).not.toMatch(/overflow(-y|-x)?:\s*(auto|scroll)/)
    }
  )

  it('grille pilotée par --cols/--rows avec minmax(0, 1fr) (pas de débordement) et texte coupé par --lines', () => {
    const g = block('.ardoise-teams-grid')
    expect(g).toMatch(/repeat\(var\(--cols[^)]*\),\s*minmax\(0,\s*1fr\)\)/)
    expect(g).toMatch(/repeat\(var\(--rows[^)]*\),\s*minmax\(0,\s*1fr\)\)/)
    expect(block('.ardoise-team-card-answer')).toMatch(/-webkit-line-clamp:\s*var\(--lines/)
    expect(g).toMatch(/min-height:\s*0/)
  })
})
