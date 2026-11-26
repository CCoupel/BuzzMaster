// #192/#193 (v13.0.0, LOT-2.1) — garde de raccord Go → hook (leçon #312, ROUE_BONUS_MODE) : tout champ PICTO_* du
// GameState serveur (struct Go `GameState`, tags JSON) doit figurer dans l'état initial ET dans le mapping
// `MSG.GAME.PICTO_X ?? prev.PICTO_X` du hook. La liste vient du source Go, pas d'une copie à la main : un champ
// ajouté côté serveur sans le hook fait échouer ce test. Contrat picto.md §5 (11 champs).
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const here = (rel) => fileURLToPath(new URL(rel, import.meta.url))
const goSrc = readFileSync(here('../../../internal/game/models.go'), 'utf8')
const hookSrc = readFileSync(here('./useWebSocket.js'), 'utf8')

function gameStatePictoFields() {
  const start = goSrc.indexOf('type GameState struct')
  expect(start).toBeGreaterThan(-1)
  const end = goSrc.indexOf('\n}\n', start)
  const body = goSrc.slice(start, end)
  return [...body.matchAll(/json:"(PICTO_[A-Z_]+)(?:,[^"]*)?"/g)].map(m => m[1])
}

const CONTRACT_FIELDS = [
  'PICTO_TEAM', 'PICTO_TEAM_COLOR', 'PICTO_DRAWER', 'PICTO_DRAWER_NAME', 'PICTO_DRAWER_CONNECTED', 'PICTO_CANDIDATES',
  'PICTO_WORDS_COUNT', 'PICTO_CANVAS_EPOCH', 'PICTO_OUTCOME', 'PICTO_REVEALED_WORD', 'PICTO_REVEALED_CATEGORY',
]

describe('useWebSocket — champs PICTO_* du GameState (#192/#193, contrat §5)', () => {
  const fields = gameStatePictoFields()

  it('le GameState Go porte les 11 champs du contrat, sans omitempty', () => {
    for (const f of CONTRACT_FIELDS) expect(fields, `${f} absent du GameState Go`).toContain(f)
    const start = goSrc.indexOf('type GameState struct')
    const body = goSrc.slice(start, goSrc.indexOf('\n}\n', start))
    expect(body).not.toMatch(/json:"PICTO_[A-Z_]+,omitempty"/) // règle projet : jamais omitempty sur GameState
  })

  it.each(fields)('%s : état initial et mapping du message GAME', (f) => {
    expect(hookSrc, `${f} absent de l'état initial`).toMatch(new RegExp(`^\\s+${f}: [^M]`, 'm'))
    expect(hookSrc, `${f} absent du mapping MSG.GAME`).toContain(`${f}: MSG.GAME.${f} ?? prev.${f}`)
  })

  it('le mot de la manche n\'a jamais de champ GameState (seul PICTO_REVEALED_WORD existe, §10)', () => {
    // PICTO_WORD_CATEGORY (v13.1.0) est l'indice public — une catégorie, jamais le mot ; seul PICTO_REVEALED_WORD porte un mot (après révélation)
    // PICTO_FOUND_WORDS (v13.1.0, picto.md §17.4) = mots TROUVÉS seulement, déjà dits à voix haute (H2) ; jamais le mot en cours ni un mot passé
    expect(fields.filter(f => /WORD/.test(f)).sort()).toEqual(['PICTO_FOUND_WORDS', 'PICTO_REVEALED_WORD', 'PICTO_WORDS_COUNT', 'PICTO_WORD_CATEGORY'])
  })
})
