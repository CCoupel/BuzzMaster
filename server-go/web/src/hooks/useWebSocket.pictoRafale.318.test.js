import { describe, it, expect } from 'vitest'
import { makePictoActions } from '../components/picto/pictoState'
import fs from 'node:fs'
import path from 'node:path'

// #318 (v13.1.0, LOT-3 T28) — actions PICTO_FOUND / PICTO_PASS (picto.md §17.5, §17.10).
describe('makePictoActions — Rafale Picto', () => {
  const calls = []
  const a = makePictoActions((action, payload) => calls.push([action, payload]))
  it('found(epoch) envoie EPOCH ; sans époque, payload vide (Picto un mot, EPOCH facultatif)', () => {
    calls.length = 0
    a.found(7); a.found()
    expect(calls).toEqual([['PICTO_FOUND', { EPOCH: 7 }], ['PICTO_FOUND', {}]])
  })
  it('found(0) envoie bien EPOCH 0', () => {
    calls.length = 0
    a.found(0)
    expect(calls).toEqual([['PICTO_FOUND', { EPOCH: 0 }]])
  })
  it('pass(epoch) envoie PICTO_PASS {EPOCH}', () => {
    calls.length = 0
    a.pass(3)
    expect(calls).toEqual([['PICTO_PASS', { EPOCH: 3 }]])
  })
})

describe('useWebSocket — pictoFound / pictoPass exposés ; aucune action PICTO_PASS côté vplayer', () => {
  const src = fs.readFileSync(path.resolve(__dirname, 'useWebSocket.js'), 'utf8')
  it('callbacks exposés', () => {
    expect(src).toMatch(/const pictoFound = useCallback/)
    expect(src).toMatch(/const pictoPass = useCallback/)
    expect(src).toMatch(/sendMessage\('PICTO_PASS', \{ EPOCH: epoch \}\)/)
  })
  it('état initial de manche : compteurs à 0, PICTO_PASSES_LEFT = −1 (illimité), listes jamais null', () => {
    expect(src).toMatch(/PICTO_FOUND_WORDS: \[\]/)
    expect(src).toMatch(/PICTO_PASSES_LEFT: -1/)
    expect(src).toMatch(/PICTO_ROUND_POINTS: 0/)
  })
})
