import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { useEffectDraft, DRAFT_DELAY_MS } from './useEffectDraft'

// ---------------------------------------------------------------------------
// #210 (v11.8.0, LOT-2, F2.1) — brouillon d'un effet : une source (grille ou texte) validée par le serveur après chaque modification,
// temporisation 300 ms, réponses périmées ignorées, grille ⇄ texte, enregistrement. Source : contracts/light-effects.md §11.2 à §11.4, §11.8.
// Le navigateur n'analyse jamais de YAML : tout vient des réponses mockées du serveur.
// ---------------------------------------------------------------------------

const A = (target, color = 'theme', intensity = 100) => ({ target, color, intensity })
const DOC = { id: 'mon-effet', name: 'Mon effet', description: '', moments: ['calm'], min_groups: 0, tick_ms: 1000, loop: true, variables: [], pre: null, steps: [[A('g1')]], last: null, post: null }
const SOURCE = (over = {}) => ({
  id: 'mon-effet', origin: 'user', file: 'mon-effet.yaml', read_only: false, yaml: '# note\nid: mon-effet\n', effect: DOC,
  has_comments: true, custom_order: false, grid: { editable: true, effect: DOC, converted: [], blocked: [] }, ...over,
})
const DRAFT_OK = (over = {}) => ({ ok: true, errors: [], effect: DOC, yaml: 'id: mon-effet\n# régénéré\n', has_comments: false, custom_order: false, grid: { editable: true, effect: DOC, converted: [], blocked: [] }, moment: 'calm', previews: [{ groups: 1, frames: [] }], ...over })
const json = (status, data) => ({ ok: status < 400, status, json: async () => data })

let drafts // corps reçus par POST …/draft
let puts
let draftAnswer
const install = ({ source = SOURCE(), put = () => json(200, { ok: true, file: 'mon-effet.yaml', catalogue: { effects: [] } }) } = {}) => {
  drafts = []; puts = []
  draftAnswer = () => json(200, DRAFT_OK())
  global.fetch = vi.fn(async (url, opts = {}) => {
    const u = String(url)
    if (/\/source$/.test(u)) return typeof source === 'function' ? source() : json(200, source)
    if (/\/draft$/.test(u)) { drafts.push(JSON.parse(opts.body)); return draftAnswer(JSON.parse(opts.body)) }
    if (opts.method === 'PUT') { puts.push({ url: u, body: JSON.parse(opts.body) }); return put() }
    return json(404, {})
  })
}
const open = async (args = { id: 'mon-effet' }) => {
  const r = renderHook((p) => useEffectDraft(p), { initialProps: args })
  await waitFor(() => expect(r.result.current.status).toBe('ready'))
  await waitFor(() => expect(r.result.current.draft).toBeTruthy())
  return r
}
beforeEach(() => { vi.useRealTimers() })
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks() })

describe('ouverture (§11.2)', () => {
  it('GET …/source puis un brouillon initial ; la grille travaille sur grid.effect', async () => {
    install()
    const { result } = await open()
    expect(global.fetch.mock.calls[0][0]).toBe('/api/lighting/effects/mon-effet/source')
    expect(result.current.doc).toEqual(DOC)
    expect(result.current.text).toBe('# note\nid: mon-effet\n')
    expect(result.current.hasComments).toBe(true)
    expect(result.current.dirty).toBe(false)
    expect(drafts[0]).toMatchObject({ effect: DOC, groups: [1, 3, 6, 12] })
  })

  it('effet inconnu : erreur lisible, pas de plantage', async () => {
    install({ source: () => json(404, {}) })
    const { result } = renderHook(() => useEffectDraft({ id: 'inconnu' }))
    await waitFor(() => expect(result.current.status).toBe('error'))
    expect(result.current.loadError).toMatch(/inconnu/i)
  })

  it('grille non éditable (voisin non convertible) : ouverture en texte, grille en lecture seule', async () => {
    install({ source: SOURCE({ grid: { editable: false, effect: null, converted: [], blocked: ['gA+1'] } }) })
    const { result } = await open()
    expect(result.current.gridEditable).toBe(false)
    expect(result.current.blocked).toEqual(['gA+1'])
    expect(drafts[0]).toHaveProperty('yaml')
  })

  it('voisins convertis : la liste converted est exposée pour le message', async () => {
    install({ source: SOURCE({ grid: { editable: true, effect: DOC, converted: [{ from: 'gA-1', to: 'gB' }], blocked: [] } }) })
    const { result } = await open()
    expect(result.current.converted).toEqual([{ from: 'gA-1', to: 'gB' }])
  })

  it('effet livré : lecture seule, setDoc / setText sans effet', async () => {
    install({ source: SOURCE({ origin: 'native', read_only: true }) })
    const { result } = await open()
    expect(result.current.readOnly).toBe(true)
    const n = drafts.length
    act(() => result.current.setDoc({ ...DOC, name: 'X' }))
    act(() => result.current.setText('id: x'))
    await new Promise(r => setTimeout(r, DRAFT_DELAY_MS + 80))
    expect(drafts.length).toBe(n)
    expect(result.current.doc).toEqual(DOC)
  })

  it('nouvel effet (initialDoc) : pas de GET source, brouillon aussitôt, canSave pour la création', async () => {
    install()
    const init = { ...DOC, id: 'nouveau' }
    const { result } = await open({ id: 'nouveau', create: true, initialDoc: init })
    expect(global.fetch.mock.calls.some(([u]) => /\/source$/.test(String(u)))).toBe(false)
    expect(result.current.persisted).toBe(false)
    await waitFor(() => expect(result.current.canSave).toBe(true))
  })
})

describe('temporisation et réponses périmées (§11.8)', () => {
  it('plusieurs modifications rapprochées = un seul envoi, 300 ms après la dernière', async () => {
    install()
    const { result } = await open()
    vi.useFakeTimers()
    const n = drafts.length
    act(() => result.current.setDoc({ ...DOC, name: 'A' }))
    await act(async () => { await vi.advanceTimersByTimeAsync(DRAFT_DELAY_MS - 50) })
    act(() => result.current.setDoc({ ...DOC, name: 'AB' }))
    await act(async () => { await vi.advanceTimersByTimeAsync(DRAFT_DELAY_MS - 50) })
    expect(drafts.length).toBe(n)
    await act(async () => { await vi.advanceTimersByTimeAsync(80) })
    expect(drafts.length).toBe(n + 1)
    expect(drafts.at(-1).effect.name).toBe('AB')
  })

  it('une réponse périmée est ignorée : seule la dernière modification compte', async () => {
    install()
    const { result } = await open()
    const resolvers = []
    draftAnswer = () => new Promise(res => resolvers.push(res))
    act(() => result.current.setDoc({ ...DOC, name: 'premier' }))
    await waitFor(() => expect(resolvers.length).toBe(1), { timeout: 1500 })
    act(() => result.current.setDoc({ ...DOC, name: 'second' }))
    await waitFor(() => expect(resolvers.length).toBe(2), { timeout: 1500 })
    await act(async () => { resolvers[1](json(200, DRAFT_OK({ previews: [{ groups: 3, frames: [], tag: 'second' }] }))) })
    await act(async () => { resolvers[0](json(200, DRAFT_OK({ ok: false, errors: [{ line: 0, message: 'périmé' }], previews: [] }))) })
    expect(result.current.draft.ok).toBe(true)
    expect(result.current.previews[0].tag).toBe('second')
    expect(result.current.errors).toEqual([])
  })

  it('erreurs et aperçus viennent de la réponse du serveur', async () => {
    install()
    const { result } = await open()
    draftAnswer = () => json(200, DRAFT_OK({ ok: false, errors: [{ line: 0, path: 'variables[0].start', message: 'l\'expression « N//2 » est mal écrite' }], previews: [], grid: null }))
    act(() => result.current.setDoc({ ...DOC, name: 'x' }))
    await waitFor(() => expect(result.current.errors).toHaveLength(1), { timeout: 1500 })
    expect(result.current.errors[0].path).toBe('variables[0].start')
    expect(result.current.canSave).toBe(false)
  })

  it('serveur injoignable : message « Vérification impossible », jamais enregistrable', async () => {
    install()
    const { result } = await open()
    draftAnswer = () => { throw new Error('réseau') }
    act(() => result.current.setDoc({ ...DOC, name: 'x' }))
    await waitFor(() => expect(result.current.errors[0]?.message).toMatch(/Vérification impossible/), { timeout: 1500 })
    expect(result.current.canSave).toBe(false)
  })

  it('tailles d\'aperçu modifiées (zones réelles) : même brouillon, nouvelles tailles', async () => {
    install()
    const { result, rerender } = await open({ id: 'mon-effet', groups: [1, 3, 6, 12] })
    rerender({ id: 'mon-effet', groups: [1, 3, 6, 12, 4] })
    await waitFor(() => expect(drafts.at(-1).groups).toEqual([1, 3, 6, 12, 4]), { timeout: 1500 })
    expect(result.current.status).toBe('ready')
  })
})

describe('grille ⇄ texte (§11.8)', () => {
  it('Grille → Texte : après une modification de la grille, le texte devient le yaml régénéré', async () => {
    install()
    const { result } = await open()
    act(() => result.current.setDoc({ ...DOC, name: 'modifié' }))
    await waitFor(() => expect(result.current.text).toBe('id: mon-effet\n# régénéré\n'), { timeout: 1500 })
  })

  it('texte inchangé tant que la grille n\'a pas été modifiée (commentaires gardés)', async () => {
    install()
    const { result } = await open()
    expect(result.current.text).toBe('# note\nid: mon-effet\n')
  })

  it('Texte → Grille refusé tant que le texte est invalide, puis accepté : la grille prend grid.effect', async () => {
    install()
    const { result } = await open()
    draftAnswer = () => json(200, DRAFT_OK({ ok: false, errors: [{ line: 3, message: 'x' }], grid: null }))
    act(() => result.current.setText('id: casse'))
    await waitFor(() => expect(result.current.errors).toHaveLength(1), { timeout: 1500 })
    act(() => { result.current.setTab('text') })
    let r
    act(() => { r = result.current.setTab('grid') })
    expect(r).toEqual({ ok: false, reason: 'invalid' })
    const NEW = { ...DOC, name: 'depuis le texte' }
    draftAnswer = () => json(200, DRAFT_OK({ grid: { editable: true, effect: NEW, converted: [], blocked: [] } }))
    act(() => result.current.setText('id: mon-effet\nnom: depuis le texte\n'))
    await waitFor(() => expect(result.current.draft.ok).toBe(true), { timeout: 1500 })
    act(() => { r = result.current.setTab('grid') })
    expect(r).toEqual({ ok: true })
    expect(result.current.doc.name).toBe('depuis le texte')
  })

  it('Texte → Grille refusé quand grid.editable est faux', async () => {
    install()
    const { result } = await open()
    draftAnswer = () => json(200, DRAFT_OK({ grid: { editable: false, effect: null, converted: [], blocked: ['gA+1'] } }))
    act(() => result.current.setText('id: mon-effet\n'))
    await waitFor(() => expect(result.current.draft.grid.editable).toBe(false), { timeout: 1500 })
    act(() => { result.current.setTab('text') })
    let r
    act(() => { r = result.current.setTab('grid') })
    expect(r).toEqual({ ok: false, reason: 'not-editable' })
  })
})

describe('enregistrement (§11.4, §11.8)', () => {
  it('rien à enregistrer tant que rien n\'a changé', async () => {
    install()
    const { result } = await open()
    expect(result.current.canSave).toBe(false)
    let r
    await act(async () => { r = await result.current.save() })
    expect(r.ok).toBe(false)
    expect(puts).toHaveLength(0)
  })

  it('modification de la grille : PUT /api/lighting/effects/{id} avec { effect }, sans create', async () => {
    install()
    const { result } = await open()
    act(() => result.current.setDoc({ ...DOC, name: 'M' }))
    await waitFor(() => expect(result.current.canSave).toBe(true), { timeout: 1500 })
    let r
    await act(async () => { r = await result.current.save() })
    expect(r.ok).toBe(true)
    expect(puts[0].url).toBe('/api/lighting/effects/mon-effet')
    expect(puts[0].body).toEqual({ effect: expect.objectContaining({ name: 'M' }) })
    expect(result.current.dirty).toBe(false)
  })

  it('texte = dernière source modifiée : PUT { yaml } tel quel', async () => {
    install()
    const { result } = await open()
    act(() => result.current.setText('# mon commentaire\nid: mon-effet\n'))
    await waitFor(() => expect(result.current.canSave).toBe(true), { timeout: 1500 })
    await act(async () => { await result.current.save() })
    expect(puts[0].body).toEqual({ yaml: '# mon commentaire\nid: mon-effet\n' })
  })

  it('création : le premier PUT porte create: true, les suivants non', async () => {
    install()
    const { result } = await open({ id: 'nouveau', create: true, initialDoc: { ...DOC, id: 'nouveau' } })
    await waitFor(() => expect(result.current.canSave).toBe(true), { timeout: 1500 })
    await act(async () => { await result.current.save() })
    expect(puts[0].body.create).toBe(true)
    act(() => result.current.setDoc({ ...DOC, id: 'nouveau', name: 'B' }))
    await waitFor(() => expect(result.current.canSave).toBe(true), { timeout: 1500 })
    await act(async () => { await result.current.save() })
    expect(puts[1].body.create).toBeUndefined()
    expect(result.current.persisted).toBe(true)
  })

  it('refus 409 exists / native, 404, 413, 400 avec errors : message lisible, rien n\'est marqué enregistré', async () => {
    for (const [status, data, re] of [
      [409, { reason: 'exists' }, /déjà cet identifiant/],
      [409, { reason: 'native' }, /dupliquez-le/],
      [404, {}, /inconnu/i],
      [413, {}, /trop gros/],
      [400, { ok: false, errors: [{ line: 0, path: 'steps[0]', message: 'x' }] }, /refuse/],
    ]) {
      install({ put: () => json(status, data) })
      const { result, unmount } = await open()
      act(() => result.current.setDoc({ ...DOC, name: `M${status}` }))
      await waitFor(() => expect(result.current.canSave).toBe(true), { timeout: 1500 })
      let r
      await act(async () => { r = await result.current.save() })
      expect(r.ok).toBe(false)
      expect(result.current.saveError).toMatch(re)
      expect(result.current.dirty).toBe(true)
      unmount()
    }
  })

  it('erreur réseau à l\'enregistrement : message, brouillon conservé', async () => {
    install({ put: () => { throw new Error('réseau') } })
    const { result } = await open()
    act(() => result.current.setDoc({ ...DOC, name: 'M' }))
    await waitFor(() => expect(result.current.canSave).toBe(true), { timeout: 1500 })
    await act(async () => { await result.current.save() })
    expect(result.current.saveError).toMatch(/impossible/i)
    expect(result.current.doc.name).toBe('M')
  })

  it('un brouillon invalide n\'est pas enregistrable', async () => {
    install()
    const { result } = await open()
    draftAnswer = () => json(200, DRAFT_OK({ ok: false, errors: [{ line: 0, message: 'x' }], grid: null }))
    act(() => result.current.setDoc({ ...DOC, name: 'x' }))
    await waitFor(() => expect(result.current.errors).toHaveLength(1), { timeout: 1500 })
    expect(result.current.canSave).toBe(false)
  })
})
