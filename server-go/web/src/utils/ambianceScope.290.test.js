// @vitest-environment node
import { describe, it, expect } from 'vitest'
import { isRoundScope, ROUND_SCOPE_MESSAGE } from './ambianceScope'

// #290 (v11.6.0, LOT-4) — réponse d'un geste direct (POST /config.json, POST /api/lighting/mode) : `scope: "round"` ⇒ le
// global n'a PAS été modifié (la manche impose le réglage). Contrat http-endpoints.md (écritures de réglages existantes,
// `scope` additif), ambiance-settings.md §4.2. Le helper ne doit JAMAIS lever d'exception (corps non JSON, clone absent).
const res = (body, { throws = false } = {}) => ({
  ok: true,
  clone() { return { json: async () => { if (throws) throw new SyntaxError('Unexpected token'); return body } } },
})

describe('isRoundScope (#290)', () => {
  it('scope "round" ⇒ vrai', async () => { expect(await isRoundScope(res({ status: 'ok', scope: 'round' }))).toBe(true) })
  it('scope "global" ⇒ faux', async () => { expect(await isRoundScope(res({ status: 'ok', scope: 'global' }))).toBe(false) })
  it('champ absent (serveur antérieur) ⇒ faux', async () => { expect(await isRoundScope(res({ status: 'ok' }))).toBe(false) })
  it('corps non JSON ⇒ faux, sans exception', async () => { expect(await isRoundScope(res(null, { throws: true }))).toBe(false) })
  it('corps JSON null / tableau / chaîne ⇒ faux', async () => {
    for (const b of [null, [], 'round', 3]) expect(await isRoundScope(res(b))).toBe(false)
  })
  it('réponse sans clone() ⇒ faux, sans exception', async () => { expect(await isRoundScope({ ok: true })).toBe(false) })
  it('la réponse d\'origine reste consommable (clone, pas lecture directe)', async () => {
    let cloned = false
    const r = { clone() { cloned = true; return { json: async () => ({ scope: 'round' }) } }, json: async () => { throw new Error('ne doit pas être lu') } }
    expect(await isRoundScope(r)).toBe(true)
    expect(cloned).toBe(true)
  })
  it('message : « Appliqué à la manche en cours seulement (la manche impose ce réglage). »', () => {
    expect(ROUND_SCOPE_MESSAGE).toBe('Appliqué à la manche en cours seulement (la manche impose ce réglage).')
  })
})
