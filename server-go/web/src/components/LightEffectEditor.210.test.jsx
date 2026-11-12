import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'

// #210 (v11.8.0, LOT-2, F2.2) — éditeur d'effet intégré : ouverture, onglets Grille / Texte, avertissements, lecture seule, ajout / retrait de
// groupe avec confirmation, enregistrement et garde « modifications non enregistrées ». Le serveur est simulé (source, draft, PUT).
// Source : contracts/light-effects.md §11.2 à §11.4, §11.8 ; maquette effect_editor_r4. On attend l'état final (fin de la vérification
// par le serveur) avant d'agir : l'éditeur affiche d'abord un rendu provisoire (leçon groups225).

vi.mock('./LightEffectEditor.css', () => ({}))
import LightEffectEditor from './LightEffectEditor'

const A = (target, color = 'theme', intensity = 100) => ({ target, color, intensity })
const DOC = {
  id: 'mon-effet', name: 'Mon effet', description: 'D', moments: ['chrono'], min_groups: 0, tick_ms: 1000, loop: true,
  variables: [{ name: 'gA', kind: 'progression', start: '1', step: '+1' }], pre: null, steps: [[A('g0'), A('gA', 'urgence')], [A('g*', '#00FF00', 60)]], last: null, post: null,
}
const SOURCE = (over = {}) => ({
  id: 'mon-effet', origin: 'user', file: 'mon-effet.yaml', read_only: false, yaml: '# n\nid: mon-effet\n', effect: DOC, has_comments: false, custom_order: false,
  grid: { editable: true, effect: DOC, converted: [], blocked: [] }, ...over,
})
const DRAFT = (over = {}) => ({ ok: true, errors: [], effect: DOC, yaml: 'id: mon-effet\n', has_comments: false, custom_order: false, grid: { editable: true, effect: DOC, converted: [], blocked: [] }, moment: 'chrono', previews: [], ...over })
const json = (status, data) => ({ ok: status < 400, status, json: async () => data })

let puts, drafts, answer
const install = (source = SOURCE(), put = () => json(200, { ok: true, file: 'mon-effet.yaml', catalogue: { effects: [] } })) => {
  puts = []; drafts = []; answer = () => json(200, DRAFT())
  global.fetch = vi.fn(async (url, opts = {}) => {
    const u = String(url)
    if (/\/source$/.test(u)) return json(200, source)
    if (/\/draft$/.test(u)) { const b = JSON.parse(opts.body); drafts.push(b); return answer(b) }
    if (opts.method === 'PUT') { puts.push(JSON.parse(opts.body)); return put() }
    return json(404, {})
  })
}
const mount = async (props = {}, source) => {
  const { keep, ...rest } = props // keep : le test a déjà installé son serveur simulé
  if (!keep) install(source)
  const p = { id: 'mon-effet', onClose: vi.fn(), onSaved: vi.fn(), ...rest }
  const utils = render(<LightEffectEditor {...p} />)
  await screen.findByTestId('effect-editor')
  await waitFor(() => expect(screen.queryByText('Vérification par le serveur…')).toBeNull())
  return { ...utils, p }
}
afterEach(() => { vi.restoreAllMocks() })

describe('ouverture', () => {
  it('fiche (nom, identifiant fixe, moments, boucle), onglet Grille actif, grille affichée', async () => {
    await mount({}, SOURCE())
    expect(screen.getByDisplayValue('Mon effet')).toBeInTheDocument()
    expect(screen.getByDisplayValue('mon-effet')).toHaveAttribute('readonly')
    expect(screen.getByRole('tab', { name: 'Grille' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByTestId('effect-grid')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Chronomètre/ })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: /Recommencer/ })).toHaveAttribute('aria-pressed', 'true')
  })

  it('« Ouverture de l\'effet… » puis erreur lisible si l\'effet est inconnu', async () => {
    global.fetch = vi.fn(async () => json(404, {}))
    render(<LightEffectEditor id="x" onClose={vi.fn()} />)
    expect(await screen.findByRole('alert')).toHaveTextContent(/inconnu/i)
    expect(screen.getByRole('button', { name: /Retour au catalogue/ })).toBeInTheDocument()
  })

  it('nombre de lignes à l\'ouverture : 5 numérotées + g* ; un effet qui écrit g12 : 12 + g*, « Ajouter un groupe » indisponible', async () => {
    const { unmount } = await mount({}, SOURCE())
    expect(screen.getByTestId('cell-0-g5')).toBeInTheDocument()
    expect(screen.queryByTestId('cell-0-g6')).toBeNull()
    unmount()
    const big = { ...DOC, steps: [[A('g0'), A('g12')]] }
    install(SOURCE({ effect: big, grid: { editable: true, effect: big, converted: [], blocked: [] } }))
    await mount({ keep: true })
    expect(screen.getByTestId('cell-0-g12')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Ajouter un groupe/ })).toBeDisabled()
  })

  it('voisins convertis : message « remplacés par les variables … : le rendu est le même »', async () => {
    await mount({}, SOURCE({ grid: { editable: true, effect: DOC, converted: [{ from: 'gA-1', to: 'gB' }, { from: 'gA+1', to: 'gC' }], blocked: [] } }))
    const info = screen.getByText(/utilisait des voisins/).parentElement
    expect(info).toHaveTextContent('gB')
    expect(info).toHaveTextContent('gC')
    expect(info).toHaveTextContent(/le rendu est le même/)
    expect(info).toHaveTextContent(/réécrit qu'à l'enregistrement/)
  })

  it('commentaires : avertissement sur la grille ; ordre non canonique : avertissement dédié', async () => {
    await mount({}, SOURCE({ has_comments: true, custom_order: true }))
    expect(screen.getByText(/contient des commentaires/)).toBeInTheDocument()
    expect(screen.getByText(/pas dans l'ordre de la grille/)).toBeInTheDocument()
  })

  it('grille non éditable : lecture seule annoncée, cibles en cause, modification par l\'onglet Texte', async () => {
    await mount({}, SOURCE({ grid: { editable: false, effect: null, converted: [], blocked: ['gA+1'] } }))
    expect(screen.getByText(/grille est en lecture seule/)).toBeInTheDocument()
    expect(screen.getByText('gA+1', { exact: false })).toBeInTheDocument()
    expect(screen.getByTestId('cell-0-g1')).toBeDisabled()
  })

  it('effet livré : bandeau « ne se modifie pas », pas d\'Enregistrer, cases et champs verrouillés, Dupliquer proposé', async () => {
    const onDuplicate = vi.fn()
    await mount({ onDuplicate }, SOURCE({ origin: 'native', read_only: true }))
    expect(screen.getByText(/est un effet livré/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Enregistrer' })).toBeNull()
    expect(screen.getByTestId('cell-0-g1')).toBeDisabled()
    expect(screen.getByDisplayValue('Mon effet')).toHaveAttribute('readonly')
    fireEvent.click(screen.getAllByRole('button', { name: /Dupliquer/ })[0])
    expect(onDuplicate).toHaveBeenCalled()
  })

  it('Télécharger : lien vers /api/lighting/effects/{id}/download dès que l\'effet existe sur le serveur', async () => {
    await mount({}, SOURCE())
    expect(screen.getByRole('link', { name: /Télécharger/ })).toHaveAttribute('href', '/api/lighting/effects/mon-effet/download')
  })
})

describe('édition de la grille', () => {
  it('un clic de case écrit l\'action du pinceau, la modification est signalée et envoyée au serveur', async () => {
    await mount()
    expect(screen.queryByText('modifications non enregistrées')).toBeNull()
    fireEvent.click(screen.getByTestId('cell-0-g3'))
    expect(screen.getByText('modifications non enregistrées')).toBeInTheDocument()
    expect(screen.getByTestId('cell-0-g3')).toHaveAccessibleName(/urgence 100 %/) // pinceau par défaut
    await waitFor(() => expect(drafts.at(-1)?.effect?.steps[0].some(a => a.target === 'g3')).toBe(true), { timeout: 1500 })
  })

  it('pinceau « gomme » : vide la case', async () => {
    await mount()
    fireEvent.click(screen.getByRole('button', { name: /^Gomme/ }))
    fireEvent.click(screen.getByTestId('cell-0-g0'))
    expect(screen.getByTestId('cell-0-g0')).toHaveAccessibleName(/vide/)
  })

  it('Ajouter un groupe : la ligne g* devient g6 (contenu gardé), une nouvelle ligne g* vide apparaît', async () => {
    await mount()
    expect(screen.getByTestId('cell-1-g*')).toHaveAccessibleName(/60 %/)
    fireEvent.click(screen.getByRole('button', { name: /Ajouter un groupe/ }))
    expect(screen.getByTestId('cell-1-g6')).toHaveAccessibleName(/60 %/)
    expect(screen.getByTestId('cell-1-g*')).toHaveAccessibleName(/vide/)
  })

  it('Retirer un groupe avec g* non vide : confirmation ; Annuler ne change rien', async () => {
    await mount()
    fireEvent.click(screen.getByRole('button', { name: /Retirer un groupe/ }))
    const dlg = within(screen.getByRole('dialog'))
    expect(screen.getByRole('dialog')).toHaveTextContent(/n'est pas vide/)
    fireEvent.click(dlg.getByRole('button', { name: /Annuler/ }))
    expect(screen.getByTestId('cell-1-g*')).toHaveAccessibleName(/60 %/)
    expect(screen.getByTestId('cell-0-g5')).toBeInTheDocument()
  })

  it('Retirer un groupe confirmé : g* supprimée, g5 devient g*, une ligne de moins', async () => {
    await mount()
    fireEvent.click(screen.getByTestId('cell-0-g5')) // écrit g5 (urgence 100 %)
    fireEvent.click(screen.getByRole('button', { name: /Retirer un groupe/ }))
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: /Retirer le groupe/ }))
    expect(screen.queryByTestId('cell-0-g5')).toBeNull()
    expect(screen.getByTestId('cell-0-g4')).toBeInTheDocument()
    expect(screen.getByTestId('cell-0-g*')).toHaveAccessibleName(/urgence 100 %/)
    expect(screen.getByTestId('cell-1-g*')).toHaveAccessibleName(/vide/)
  })

  it('Retirer un groupe sans confirmation quand g* est vide', async () => {
    const d = { ...DOC, steps: [[A('g0'), A('g1')]] }
    install(SOURCE({ effect: d, grid: { editable: true, effect: d, converted: [], blocked: [] } }))
    await mount({ keep: true })
    fireEvent.click(screen.getByRole('button', { name: /Retirer un groupe/ }))
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(screen.queryByTestId('cell-0-g5')).toBeNull()
  })

  it('erreur du serveur placée par path : variable (sous sa ligne) et colonne (marquée)', async () => {
    await mount()
    answer = () => json(200, DRAFT({ ok: false, grid: null, errors: [
      { line: 0, path: 'variables[0].start', message: 'l\'expression « N//2 » est mal écrite' },
      { line: 0, path: 'steps[1]', message: 'un pas compte de 1 à 40 actions' },
    ] }))
    fireEvent.click(screen.getByTestId('cell-0-g2'))
    expect(await screen.findByTestId('effect-errors', {}, { timeout: 1500 })).toHaveTextContent('un pas compte de 1 à 40 actions')
    expect(within(screen.getByTestId('var-gA')).getByRole('alert')).toHaveTextContent('N//2')
    expect(screen.getByTestId('cell-1-g1').closest('td')).toHaveClass('is-err')
    expect(screen.getByRole('button', { name: 'Enregistrer' })).toBeDisabled()
  })
})

describe('onglet Texte', () => {
  it('le texte du fichier est affiché tel quel (commentaires compris) ; saisie envoyée en brouillon yaml', async () => {
    await mount({}, SOURCE({ has_comments: true }))
    fireEvent.click(screen.getByRole('tab', { name: 'Texte' }))
    const area = screen.getByLabelText("Fichier de l'effet (YAML)")
    expect(area).toHaveValue('# n\nid: mon-effet\n')
    fireEvent.change(area, { target: { value: '# n\nid: mon-effet\nnom: Z\n' } })
    await waitFor(() => expect(drafts.at(-1)).toHaveProperty('yaml', '# n\nid: mon-effet\nnom: Z\n'), { timeout: 1500 })
  })

  it('Texte → Grille refusé tant que le texte est invalide (message), accepté ensuite', async () => {
    await mount()
    fireEvent.click(screen.getByRole('tab', { name: 'Texte' }))
    answer = () => json(200, DRAFT({ ok: false, grid: null, errors: [{ line: 4, message: 'cassé' }] }))
    fireEvent.change(screen.getByLabelText("Fichier de l'effet (YAML)"), { target: { value: 'id: [' } })
    expect(await screen.findByText(/Ligne 4/, {}, { timeout: 1500 })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('tab', { name: 'Grille' }))
    expect(screen.getAllByRole('alert').some(a => /Corrigez les erreurs du texte/.test(a.textContent))).toBe(true)
    expect(screen.getByRole('tab', { name: 'Texte' })).toHaveAttribute('aria-selected', 'true')
  })

  it('avertissement « commentaires » dans l\'onglet Texte : conservés si on enregistre depuis le texte', async () => {
    await mount({}, SOURCE({ has_comments: true }))
    fireEvent.click(screen.getByRole('tab', { name: 'Texte' }))
    expect(screen.getByText(/conservés tant que vous enregistrez depuis l'onglet Texte/)).toBeInTheDocument()
  })
})

describe('enregistrer et quitter', () => {
  it('Enregistrer : PUT { effect } puis onSaved(catalogue) ; le bouton se désactive, la modification n\'est plus signalée', async () => {
    const { p } = await mount()
    expect(screen.getByRole('button', { name: 'Enregistrer' })).toBeDisabled()
    fireEvent.click(screen.getByTestId('cell-0-g4'))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Enregistrer' })).toBeEnabled(), { timeout: 2000 })
    fireEvent.click(screen.getByRole('button', { name: 'Enregistrer' }))
    await waitFor(() => expect(p.onSaved).toHaveBeenCalledWith({ effects: [] }))
    expect(puts[0]).toHaveProperty('effect')
    expect(puts[0]).not.toHaveProperty('create')
    await waitFor(() => expect(screen.queryByText('modifications non enregistrées')).toBeNull())
  })

  it('refus du serveur : message affiché, la modification reste signalée', async () => {
    install(SOURCE(), () => json(409, { reason: 'native' }))
    await mount({ keep: true })
    fireEvent.click(screen.getByTestId('cell-0-g4'))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Enregistrer' })).toBeEnabled(), { timeout: 2000 })
    fireEvent.click(screen.getByRole('button', { name: 'Enregistrer' }))
    expect(await screen.findByText(/dupliquez-le/)).toBeInTheDocument()
    expect(screen.getByText('modifications non enregistrées')).toBeInTheDocument()
  })

  it('retour au catalogue sans modification : onClose aussitôt, pas de question', async () => {
    const { p } = await mount()
    fireEvent.click(screen.getByRole('button', { name: /Retour au catalogue/ }))
    expect(p.onClose).toHaveBeenCalled()
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('retour avec modifications : question « Modifications non enregistrées » ; Rester / Abandonner', async () => {
    const { p } = await mount()
    fireEvent.click(screen.getByTestId('cell-0-g4'))
    fireEvent.click(screen.getByRole('button', { name: /Retour au catalogue/ }))
    const dlg = within(screen.getByRole('dialog'))
    expect(dlg.getByText('Modifications non enregistrées')).toBeInTheDocument()
    fireEvent.click(dlg.getByRole('button', { name: 'Rester' }))
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(p.onClose).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: /Retour au catalogue/ }))
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: /Abandonner/ }))
    expect(p.onClose).toHaveBeenCalled()
  })
})

describe('nouvel effet et copie', () => {
  it('initialDoc + create : pas de GET source ; le premier enregistrement crée (create: true)', async () => {
    install()
    const init = { ...DOC, id: 'nouveau', name: 'Nouveau' }
    const { p } = await mount({ id: 'nouveau', initialDoc: init, create: true, keep: true })
    expect(global.fetch.mock.calls.some(([u]) => /\/source$/.test(String(u)))).toBe(false)
    expect(screen.queryByRole('link', { name: /Télécharger/ })).toBeNull() // pas encore de fichier
    await waitFor(() => expect(screen.getByRole('button', { name: 'Enregistrer' })).toBeEnabled(), { timeout: 2000 })
    fireEvent.click(screen.getByRole('button', { name: 'Enregistrer' }))
    await waitFor(() => expect(p.onSaved).toHaveBeenCalled())
    expect(puts[0].create).toBe(true)
    expect(await screen.findByRole('link', { name: /Télécharger/ })).toBeInTheDocument()
  })
})
