import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'

vi.mock('./LightEffectEditor.css', () => ({}))
vi.mock('./LightEffectsImport', async (orig) => ({ ...(await orig()) }))
import LightEffectEditor from './LightEffectEditor'

const A = (target, color = 'theme', intensity = 100) => ({ target, color, intensity })
const DOC = {
  id: 'mon', name: 'Mon effet', description: 'd', moments: ['chrono'], min_groups: 0, tick_ms: 1000, loop: true,
  variables: [{ name: 'gA', kind: 'progression', start: 'N/2', step: '+1' }],
  pre: null, steps: [[A('g0'), A('gA', 'urgence')]], last: null, post: null,
}
const SOURCE = (extra = {}) => ({
  id: 'mon', origin: 'user', file: 'mon.yaml', read_only: false, yaml: '# note\nid: mon\n', effect: DOC, has_comments: true, custom_order: false,
  grid: { editable: true, effect: DOC, converted: [], blocked: [] }, ...extra,
})
const json = (status, data) => ({ ok: status < 400, status, json: async () => data, text: async () => JSON.stringify(data) })
const preview = (n) => ({ id: 'mon', groups: n, frames: [0, 1, 2].map(t => ({ tick: t, step: 'pas:0', groups: [], vars: { gA: ((t + 1) % n) + 1 } })) })

function server({ source = SOURCE(), draft } = {}) {
  const calls = []
  global.fetch = vi.fn(async (url, opts = {}) => {
    const method = opts.method || 'GET'
    const body = opts.body ? JSON.parse(opts.body) : null
    calls.push({ url, method, body })
    if (/\/source$/.test(url)) return json(200, source)
    if (/\/draft$/.test(url)) return json(200, draft ? draft(body) : { ok: true, errors: [], effect: body.effect || DOC, yaml: 'id: mon\nregen\n', has_comments: false, custom_order: false, grid: { editable: true, effect: body.effect || DOC, converted: [], blocked: [] }, previews: (body.groups || []).map(preview) })
    if (method === 'PUT') return json(200, { ok: true, catalogue: { effects: [] } })
    return json(404, {})
  })
  return calls
}
const open = async (props = {}) => {
  const utils = render(<LightEffectEditor id="mon" {...props} />)
  await screen.findByTestId('effect-grid')
  return utils
}
afterEach(() => { vi.restoreAllMocks() })

describe('LightEffectEditor (dev)', () => {
  it('ouvre l\'effet : fiche, variables en haut, g0…g5 puis g*, avertissement commentaires, aperçus demandés (1, 3, 6, 12)', async () => {
    const calls = server()
    await open()
    expect(screen.getByDisplayValue('Mon effet')).toBeInTheDocument()
    const rows = [...document.querySelectorAll('.efx-grid tbody th[scope=row]')].map(t => t.firstChild.textContent)
    expect(rows).toEqual(['gA', 'g0', 'g1', 'g2', 'g3', 'g4', 'g5', 'g*'])
    expect(screen.getByText(/Ce fichier contient des commentaires/)).toBeInTheDocument()
    await waitFor(() => expect(calls.some(c => /draft$/.test(c.url))).toBe(true))
    expect(calls.find(c => /draft$/.test(c.url)).body.groups).toEqual([1, 3, 6, 12])
  })

  it('un clic de case envoie le brouillon (effect) après 300 ms ; Enregistrer envoie effect', async () => {
    const calls = server()
    const onSaved = vi.fn()
    await open({ onSaved })
    fireEvent.click(screen.getByRole('button', { name: /^Éteint/ }))
    fireEvent.click(screen.getByTestId('cell-0-g2'))
    expect(screen.getByText('modifications non enregistrées')).toBeInTheDocument()
    await waitFor(() => expect(calls.filter(c => /draft$/.test(c.url)).length).toBeGreaterThanOrEqual(2))
    const last = calls.filter(c => /draft$/.test(c.url)).at(-1).body
    expect(last.effect.steps[0].find(a => a.target === 'g2')).toEqual({ target: 'g2', off: true })
    const save = screen.getByRole('button', { name: 'Enregistrer' })
    await waitFor(() => expect(save).not.toBeDisabled())
    fireEvent.click(save)
    await waitFor(() => expect(onSaved).toHaveBeenCalled())
    const put = calls.find(c => c.method === 'PUT')
    expect(put.url).toBe('/api/lighting/effects/mon')
    expect(put.body.effect).toBeTruthy()
    expect(put.body.yaml).toBeUndefined()
  })

  it('erreur de variable (path) : affichée sous la variable ; Enregistrer grisé', async () => {
    server({ draft: (b) => ({ ok: false, errors: [{ line: 0, path: 'variables[0].start', message: "l'expression « gA+ » est incomplète" }], effect: b.effect, yaml: '', previews: [] }) })
    await open()
    fireEvent.click(screen.getByTestId('cell-0-g1'))
    const v = await screen.findByTestId('var-gA')
    expect(await within(v).findByText(/incomplète/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Enregistrer' })).toBeDisabled()
  })

  it('bulle « ? » : suite lue dans vars des aperçus du serveur', async () => {
    server()
    await open()
    await waitFor(() => expect(screen.getByRole('button', { name: 'Ce que fait gA' })).toBeInTheDocument())
    await new Promise(r => setTimeout(r, 50))
    fireEvent.click(screen.getByRole('button', { name: 'Ce que fait gA' }))
    const pop = await screen.findByRole('dialog', { name: 'Ce que fait gA' })
    await waitFor(() => expect(within(pop).getByText(/6 groupes/)).toBeInTheDocument())
    expect(pop.textContent).toMatch(/3 groupes : g2 g3 g1/)
    fireEvent.keyDown(document, { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Ce que fait gA' })).toBeNull())
  })

  it('Texte : texte tel que le fichier ; Texte → Grille refusé tant que le texte est en erreur', async () => {
    server({ draft: (b) => (b.yaml ? { ok: false, errors: [{ line: 3, message: 'clé inconnue' }], effect: null, yaml: '', previews: [] } : { ok: true, errors: [], effect: b.effect, yaml: 'id: mon\nregen\n', grid: { editable: true, effect: b.effect }, previews: [] }) })
    await open()
    fireEvent.click(screen.getByRole('tab', { name: 'Texte' }))
    const ta = screen.getByLabelText(/Fichier de l'effet/)
    expect(ta.value).toBe('# note\nid: mon\n')
    fireEvent.change(ta, { target: { value: 'oups' } })
    expect(await screen.findByText(/clé inconnue/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('tab', { name: 'Grille' }))
    expect(await screen.findByText(/Corrigez les erreurs du texte/)).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Texte' })).toHaveAttribute('aria-selected', 'true')
  })

  it('Grille → Texte : le texte devient le fichier régénéré ; Enregistrer depuis le texte envoie yaml', async () => {
    const calls = server()
    await open()
    fireEvent.click(screen.getByTestId('cell-0-g3'))
    await waitFor(() => expect(calls.filter(c => /draft$/.test(c.url)).length).toBeGreaterThanOrEqual(2))
    fireEvent.click(screen.getByRole('tab', { name: 'Texte' }))
    await waitFor(() => expect(screen.getByLabelText(/Fichier de l'effet/).value).toBe('id: mon\nregen\n'))
    fireEvent.change(screen.getByLabelText(/Fichier de l'effet/), { target: { value: 'id: mon\nnouveau\n' } })
    const save = screen.getByRole('button', { name: 'Enregistrer' })
    await waitFor(() => expect(save).not.toBeDisabled())
    fireEvent.click(save)
    await waitFor(() => expect(calls.some(c => c.method === 'PUT')).toBe(true))
    expect(calls.find(c => c.method === 'PUT').body).toEqual({ yaml: 'id: mon\nnouveau\n' })
  })

  it('effet livré : lecture seule, pas de pinceau ni d\'Enregistrer, message « Dupliquez-le »', async () => {
    server({ source: SOURCE({ origin: 'native', read_only: true }) })
    await open({ onDuplicate: vi.fn() })
    expect(screen.getByText(/est un effet livré avec BuzzControl/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Enregistrer' })).toBeNull()
    expect(screen.queryByTestId('effect-brush')).toBeNull()
    expect(screen.getByTestId('cell-0-g1')).toBeDisabled()
  })

  it('Retirer un groupe : confirmation quand g* n\'est pas vide ; g<n> devient g*', async () => {
    server()
    await open()
    fireEvent.click(screen.getByRole('button', { name: /^Urgence/ }))
    fireEvent.click(screen.getByTestId('cell-0-g*'))
    fireEvent.click(screen.getByRole('button', { name: /Retirer un groupe/ }))
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Retirer le groupe' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect([...document.querySelectorAll('.efx-grid tbody th[scope=row]')].map(t => t.firstChild.textContent)).toEqual(['gA', 'g0', 'g1', 'g2', 'g3', 'g4', 'g*'])
  })

  it('Retour avec modifications : fenêtre Enregistrer / Abandonner / Rester', async () => {
    server()
    const onClose = vi.fn()
    await open({ onClose })
    fireEvent.click(screen.getByTestId('cell-0-g1'))
    fireEvent.click(screen.getByRole('button', { name: /Retour au catalogue/ }))
    const dlg = within(screen.getByRole('dialog'))
    fireEvent.click(dlg.getByRole('button', { name: 'Rester' }))
    expect(onClose).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: /Retour au catalogue/ }))
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: /Abandonner/ }))
    expect(onClose).toHaveBeenCalled()
  })
})
