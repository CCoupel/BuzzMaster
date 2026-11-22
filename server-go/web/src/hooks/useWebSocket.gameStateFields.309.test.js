// Garde étendue (backlog LOT-1, #312 → tous les champs) : tout champ JSON du `GameState` serveur (struct Go) doit être lu par
// le hook `useWebSocket` (référence `MSG.GAME.<CHAMP>` ou `GAME?.<CHAMP>`). Un champ ajouté côté serveur sans le hook fait échouer
// ce test — cause du retour QUALIF n°4 (achat de voyelles jamais proposé : champs ROUE_* jamais recopiés). La liste vient du source Go.
// Les champs ROUE_* ont en plus la garde stricte de useWebSocket.roueFields.312.test.js (état initial ET mapping `?? prev`).
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const here = (rel) => fileURLToPath(new URL(rel, import.meta.url))
const goSrc = readFileSync(here('../../../internal/game/models.go'), 'utf8')
const hookSrc = readFileSync(here('./useWebSocket.js'), 'utf8')

function gameStateFields() {
  const start = goSrc.indexOf('type GameState struct')
  expect(start).toBeGreaterThan(-1)
  const body = goSrc.slice(start, goSrc.indexOf('\n}\n', start))
  return [...body.matchAll(/json:"([A-Za-z0-9_]+)(?:,[^"]*)?"/g)].map(m => m[1])
}

const isRead = (f) => new RegExp(`GAME\\??\\.${f}\\b`).test(hookSrc)

// Exceptions justifiées : champs du GameState que le hook ne lit pas dans le message UPDATE.
const NOT_READ_BY_HOOK = {
  new_game_backgrounds: "lu dans un autre message du hook (`MSG.new_game_backgrounds`, ligne « NEW_GAME_BACKGROUNDS »), pas dans MSG.GAME",
  RAFALE_TEAM_CATEGORY_COUNTERS: "compteurs de RAFALE exploités côté serveur / autres clients, aucune page ne les lit (grep web/src : aucun usage)",
  QUESTION_SOUND_OWNER: "propriétaire du son de question, usage serveur uniquement (aucun usage dans web/src)",
}

describe('useWebSocket — tous les champs du GameState serveur', () => {
  const fields = gameStateFields()

  it('lit bien le GameState Go (plusieurs dizaines de champs, dont ROUE_* et voyelles)', () => {
    expect(fields.length).toBeGreaterThanOrEqual(90)
    for (const f of ['PHASE', 'QUESTION', 'ROUE_SUBPHASE', 'ROUE_BUY_VOWELS', 'ROUE_STOP_VOWEL']) expect(fields).toContain(f)
  })

  it.each(fields.filter(f => !(f in NOT_READ_BY_HOOK)))('%s : lu par le hook', (f) => {
    expect(isRead(f), `${f} (GameState Go) n'est jamais lu dans useWebSocket.js : l'écran ne le verra pas`).toBe(true)
  })

  it('la liste d\'exceptions ne pourrit pas : chaque exception existe dans le GameState et n\'est toujours pas lue', () => {
    for (const f of Object.keys(NOT_READ_BY_HOOK)) {
      expect(fields, `${f} n'est plus un champ du GameState : retirer l'exception`).toContain(f)
      expect(isRead(f), `${f} est maintenant lu par le hook : retirer l'exception`).toBe(false)
    }
  })

  it('aucun champ du GameState n\'apparaît en double dans le source Go (clés JSON uniques)', () => {
    expect(new Set(fields).size).toBe(fields.length)
  })
})
