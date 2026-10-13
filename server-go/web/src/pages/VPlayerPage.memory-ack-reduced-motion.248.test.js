import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

// #248 lot 5b (revue, mineur) — l'échec d'un retournement fait trembler la carte (animation vplayer-card-shake).
// Sous prefers-reduced-motion, cette animation ne doit pas jouer (comme le spinner du lot 5a). jsdom ne calcule
// pas les styles : on lit le CSS. Deux écritures acceptées : un bloc `@media (prefers-reduced-motion: reduce)` qui
// neutralise l'animation de la carte en échec, OU la déclaration d'animation elle-même placée sous
// `@media (prefers-reduced-motion: no-preference)`.

const here = dirname(fileURLToPath(import.meta.url))
const css = readFileSync(resolve(here, './PlayerDisplay.css'), 'utf8')

function mediaBlocks(src, feature) {
  const blocks = []
  const re = new RegExp(`@media\\s*\\(\\s*prefers-reduced-motion:\\s*${feature}\\s*\\)\\s*\\{`, 'g')
  let m
  while ((m = re.exec(src))) {
    let depth = 1
    let i = re.lastIndex
    while (i < src.length && depth > 0) {
      if (src[i] === '{') depth++
      else if (src[i] === '}') depth--
      i++
    }
    blocks.push(src.slice(re.lastIndex, i - 1))
  }
  return blocks
}

describe('retournement refusé — secousse de la carte et prefers-reduced-motion (#248 lot 5b)', () => {
  it('Critical — la secousse (vplayer-card-shake) est neutralisée sous prefers-reduced-motion', () => {
    const noMotion = /animation(-name)?\s*:\s*none/
    const reduce = mediaBlocks(css, 'reduce').some((b) => /vplayer-pending-failed|vplayer-card-shake/.test(b) && noMotion.test(b))
    const gated = mediaBlocks(css, 'no-preference').some((b) => /animation\s*:[^;}]*vplayer-card-shake/.test(b))
    expect(reduce || gated, 'aucune garde prefers-reduced-motion sur vplayer-card-shake').toBe(true)
  })

  it('la secousse existe bien (garde : le test ne passe pas à vide si l\'animation est renommée)', () => {
    expect(css).toMatch(/@keyframes\s+vplayer-card-shake/)
  })
})
