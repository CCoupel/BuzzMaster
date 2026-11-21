// #312 — garde : tout champ ROUE_* du GameState serveur (struct Go `GameState`, tags JSON) doit figurer dans l'état initial
// ET dans le mapping `MSG.GAME.ROUE_X ?? prev.ROUE_X` du hook. La liste vient du source Go (pas d'une copie à la main) :
// un champ ajouté côté serveur sans le hook fait échouer ce test (cause du retour QUALIF n°4 : achat de voyelles jamais proposé).
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const here = (rel) => fileURLToPath(new URL(rel, import.meta.url))
const goSrc = readFileSync(here('../../../internal/game/models.go'), 'utf8')
const hookSrc = readFileSync(here('./useWebSocket.js'), 'utf8')

function gameStateRoueFields() {
  const start = goSrc.indexOf('type GameState struct')
  expect(start).toBeGreaterThan(-1)
  const end = goSrc.indexOf('\n}\n', start)
  const body = goSrc.slice(start, end)
  return [...body.matchAll(/json:"(ROUE_[A-Z_]+)(?:,[^"]*)?"/g)].map(m => m[1])
}

describe('useWebSocket — champs ROUE_* du GameState (#312)', () => {
  const fields = gameStateRoueFields()
  it('lit bien les champs du GameState Go (v12.0 + 9 champs voyelles v12.1.0)', () => {
    expect(fields.length).toBeGreaterThanOrEqual(26)
    for (const f of ['ROUE_SUBPHASE', 'ROUE_BUY_VOWELS', 'ROUE_VOWEL_TIMING', 'ROUE_VOWELS_LEFT', 'ROUE_TURN_VOWEL', 'ROUE_STOP_VOWEL']) expect(fields).toContain(f)
  })
  it.each(fields)('%s : état initial et mapping du message GAME', (f) => {
    expect(hookSrc, `${f} absent de l'état initial`).toMatch(new RegExp(`^\\s+${f}: [^M]`, 'm'))
    expect(hookSrc, `${f} absent du mapping MSG.GAME`).toContain(`${f}: MSG.GAME.${f} ?? prev.${f}`)
  })
})
