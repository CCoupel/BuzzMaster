import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'

// ---------------------------------------------------------------------------
// #305 (v11.8.0, LOT-1) — catalogue des effets : Importer…, Télécharger, Supprimer / Revenir à l'effet d'origine, choix de la zone d'essai.
// Écrit depuis contracts/light-effects.md §11.5 à §11.7 et la maquette docs/mockup/v11.8.0/ui/ambiance_page__effects_catalogue_import_export.html
// (§01 à §04, SANS « Nouvel effet », « Modifier », « Ouvrir », « Dupliquer » : LOT-2). TDD : peut échouer tant que F1.1 n'est pas livré.
// Contrat d'interface supposé : le composant garde ses props de v11.7.0 (catalog, lighting, zoneGroups, onReload, onTry, onStopTry,
// tryUnavailable) ; les appels réseau sont `fetch` aux routes du contrat (mockées ici) ; après import ou suppression, le catalogue
// affiché vient de `catalogue` dans la réponse — le composant le remonte par `onCatalogue(catalogue)` (à défaut : onReload()).
// Si dev-frontend nomme autrement ce rappel, il n'adapte que `mount()`.
// ---------------------------------------------------------------------------

vi.mock('./LightEffectsCatalogue.css', () => ({}))
import LightEffectsCatalogue from './LightEffectsCatalogue'

const ALL = { ambiance: true, animateur: true, public: true }
const fx = (id, name, moments, extra = {}) => ({
  id, name, description: '', moments, origin: 'native', file: `natifs/${id}.yaml`, min_groups: 0, tick_ms: 1000, applicable: ALL, ...extra,
})
const CATALOG = {
  groups: { ambiance: 4, animateur: 1, public: 0 },
  effects: [
    fx('pulsation', 'Pulsation', ['chrono'], { used_by: [{ zone: 'ambiance', moment: 'chrono' }] }),
    fx('degrade', 'Dégradé', ['calm'], { origin: 'override', file: 'degrade.yaml', min_groups: 2 }),
    fx('aller-retour', 'Aller-retour', ['calm', 'chrono'], { origin: 'user', file: 'aller-retour.yaml', used_by: [{ zone: 'public', moment: 'calm' }] }),
    fx('vague', 'Vague', ['calm'], { origin: 'user', file: 'vague.yaml' }),
  ],
  errors: [],
  dir: 'data/files/effects',
}
const LIGHTING = { effects: { public: { calm: 'aller-retour' } } }
const GROUPS = { ambiance: 4, animateur: 1, public: 0 }

const mount = (props = {}) => {
  const onCatalogue = vi.fn()
  const onReload = vi.fn().mockResolvedValue(true)
  const utils = render(
    <LightEffectsCatalogue
      catalog={CATALOG} lighting={LIGHTING} zoneGroups={GROUPS}
      onReload={onReload} onCatalogue={onCatalogue}
      onTry={vi.fn().mockResolvedValue({ ok: true, duration_ms: 10000 })} onStopTry={vi.fn()}
      {...props}
    />,
  )
  return { ...utils, onCatalogue, onReload }
}
const card = (id) => within(screen.getByTestId(`fx-${id}`))
const json = (status, data) => ({ ok: status < 400, status, json: async () => data, text: async () => JSON.stringify(data) })
const fetchMock = (handler) => { global.fetch = vi.fn(async (url, opts) => handler(String(url), opts || {})) }
const calls = (re, method) => global.fetch.mock.calls.filter(([u, o]) => re.test(String(u)) && (!method || (o?.method || 'GET') === method))
const yamlFile = (name, body = 'id: x\n') => new File([body], name, { type: 'application/yaml' })
const pick = (container, files) => {
  const input = container.querySelector('input[type="file"]')
  expect(input, 'un champ fichier (Importer…)').toBeTruthy()
  fireEvent.change(input, { target: { files } })
}
const NEW_CAT = { ...CATALOG, effects: [...CATALOG.effects, fx('vague-douce', 'Vague douce', ['calm'], { origin: 'user', file: 'vague-douce.yaml' })] }
afterEach(() => { vi.restoreAllMocks() })

describe('Catalogue — boutons selon l\'origine (§01)', () => {
  it('en-tête : « Importer… » et « Recharger les fichiers » ; pas de « Nouvel effet » (LOT-2)', () => {
    mount()
    expect(screen.getByRole('button', { name: /Importer/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Recharger/ })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Nouvel effet/ })).toBeNull()
  })

  it('effet livré : Télécharger, sans Supprimer ni Revenir', () => {
    mount()
    const c = card('pulsation')
    expect(c.getByRole('link', { name: /Télécharger/ }) || c.getByRole('button', { name: /Télécharger/ })).toBeTruthy()
    expect(c.queryByRole('button', { name: /Supprimer/ })).toBeNull()
    expect(c.queryByRole('button', { name: /Revenir/ })).toBeNull()
  })

  it('natif modifié : Télécharger et « Revenir à l\'effet d\'origine », sans Supprimer', () => {
    mount()
    const c = card('degrade')
    expect(c.getByRole('button', { name: /Revenir à l'effet d'origine/ })).toBeInTheDocument()
    expect(c.queryByRole('button', { name: /^🗑?\s*Supprimer/ })).toBeNull()
  })

  it('effet personnalisé : Télécharger et Supprimer, sans Revenir', () => {
    mount()
    const c = card('aller-retour')
    expect(c.getByRole('button', { name: /Supprimer/ })).toBeInTheDocument()
    expect(c.queryByRole('button', { name: /Revenir/ })).toBeNull()
  })

  it('Télécharger = lien vers /api/lighting/effects/{id}/download (§11.6), pour chaque effet y compris livré', () => {
    mount()
    for (const id of ['pulsation', 'degrade', 'aller-retour']) {
      const el = card(id).getByText(/Télécharger/).closest('a')
      expect(el, `lien Télécharger de ${id}`).toBeTruthy()
      expect(el.getAttribute('href')).toBe(`/api/lighting/effects/${id}/download`)
    }
  })
})

describe('Importer… (§02, §11.7)', () => {
  it('envoie un multipart au serveur : champ « files » répété, sans on_conflict au premier envoi (= ask)', async () => {
    fetchMock((u) => json(200, { results: [{ file: 'vague-douce.yaml', status: 'imported', id: 'vague-douce', name: 'Vague douce', stored: 'vague-douce.yaml' }], catalogue: NEW_CAT }))
    const { container, onCatalogue } = mount()
    pick(container, [yamlFile('vague-douce.yaml'), yamlFile('b.yml')])
    await waitFor(() => expect(calls(/\/api\/lighting\/effects\/import$/, 'POST')).toHaveLength(1))
    const body = calls(/import$/, 'POST')[0][1].body
    expect(body).toBeInstanceOf(FormData)
    expect(body.getAll('files').map((f) => f.name)).toEqual(['vague-douce.yaml', 'b.yml'])
    expect(['ask', null]).toContain(body.get('on_conflict'))
    await waitFor(() => expect(onCatalogue).toHaveBeenCalledWith(NEW_CAT))
  })

  it('panneau de résultat : une ligne par fichier — importé, refusé avec ligne et message ; il reste jusqu\'à sa fermeture', async () => {
    fetchMock(() => json(200, { results: [
      { file: 'vague-douce.yaml', status: 'imported', id: 'vague-douce', name: 'Vague douce', stored: 'vague-douce.yaml' },
      { file: 'casse.yaml', status: 'invalid', errors: [{ line: 12, message: 'la variable « gB » n\'est pas déclarée dans variables' }] },
      { file: 'pulsation.yaml', status: 'native', id: 'pulsation', errors: [{ line: 2, message: '« pulsation » est l\'identifiant d\'un effet livré : changez la clé id' }] },
      { file: 'gros.yaml', status: 'too_big' },
      { file: 'plein.yaml', status: 'limit' },
    ], catalogue: NEW_CAT }))
    const { container } = mount()
    pick(container, [yamlFile('vague-douce.yaml')])
    const panel = await screen.findByTestId('import-result')
    const p = within(panel)
    expect(p.getByText(/vague-douce\.yaml/)).toBeInTheDocument()
    expect(p.getByText(/Vague douce/)).toBeInTheDocument()
    expect(p.getByText(/Ligne 12/)).toBeInTheDocument()
    expect(p.getByText(/gB/)).toBeInTheDocument()
    expect(p.getByText(/identifiant d'un effet livré/)).toBeInTheDocument()
    expect(p.getByText(/Fichier trop gros \(64 Ko au plus\)/)).toBeInTheDocument()
    expect(p.getByText(/100 effets/)).toBeInTheDocument()
    fireEvent.click(p.getByRole('button', { name: /Fermer/ }))
    expect(screen.queryByTestId('import-result')).toBeNull()
  })

  it('« replaced » et « renamed » sont lisibles (remplacé / nouvel identifiant)', async () => {
    fetchMock(() => json(200, { results: [
      { file: 'a.yaml', status: 'replaced', id: 'aller-retour', name: 'Aller-retour', stored: 'aller-retour.yaml' },
      { file: 'b.yaml', status: 'renamed', id: 'vague', new_id: 'vague-2', name: 'Vague', stored: 'vague-2.yaml' },
    ], catalogue: NEW_CAT }))
    const { container } = mount()
    pick(container, [yamlFile('a.yaml'), yamlFile('b.yaml')])
    const p = within(await screen.findByTestId('import-result'))
    expect(p.getByText(/remplacé/i)).toBeInTheDocument()
    expect(p.getByText(/vague-2/)).toBeInTheDocument()
  })

  it('fichiers déposés sur le catalogue : même envoi que le choix de fichiers', async () => {
    fetchMock(() => json(200, { results: [{ file: 'd.yaml', status: 'imported', id: 'd', name: 'D', stored: 'd.yaml' }], catalogue: NEW_CAT }))
    mount()
    const zone = screen.getByTestId('catalogue-dropzone')
    fireEvent.drop(zone, { dataTransfer: { files: [yamlFile('d.yaml')], types: ['Files'] } })
    await waitFor(() => expect(calls(/import$/, 'POST')).toHaveLength(1))
    expect(calls(/import$/, 'POST')[0][1].body.getAll('files')[0].name).toBe('d.yaml')
  })

  it('envoi refusé (400) : message du serveur affiché, catalogue inchangé', async () => {
    fetchMock(() => json(400, { error: 'Ce fichier n\'est pas un fichier d\'effet (.yaml).' }))
    const { container, onCatalogue } = mount()
    pick(container, [yamlFile('notes.txt')])
    expect(await screen.findByText(/pas un fichier d'effet/)).toBeInTheDocument()
    expect(onCatalogue).not.toHaveBeenCalled()
  })

  it('erreur réseau : message, pas de plantage', async () => {
    global.fetch = vi.fn(async () => { throw new Error('réseau') })
    const { container } = mount()
    pick(container, [yamlFile('a.yaml')])
    expect(await screen.findByRole('alert')).toBeInTheDocument()
  })
})

describe('Conflit d\'import (§02, §11.7 « conflict »)', () => {
  const CONFLICT = { results: [
    { file: 'aller-retour (1).yaml', status: 'conflict', id: 'aller-retour', name: 'Aller-retour', existing: { name: 'Aller-retour', file: 'aller-retour.yaml' } },
    { file: 'ok.yaml', status: 'imported', id: 'ok', name: 'Ok', stored: 'ok.yaml' },
  ], catalogue: NEW_CAT }

  const open = async () => {
    fetchMock((u, o) => {
      if (/import$/.test(u) && o.body.get('on_conflict') === 'replace') return json(200, { results: [{ file: 'aller-retour (1).yaml', status: 'replaced', id: 'aller-retour', name: 'Aller-retour', stored: 'aller-retour.yaml' }], catalogue: NEW_CAT })
      if (/import$/.test(u) && o.body.get('on_conflict') === 'keep_both') return json(200, { results: [{ file: 'aller-retour (1).yaml', status: 'renamed', id: 'aller-retour', new_id: 'aller-retour-2', name: 'Aller-retour', stored: 'aller-retour-2.yaml' }], catalogue: NEW_CAT })
      return json(200, CONFLICT)
    })
    const utils = mount()
    pick(utils.container, [yamlFile('aller-retour (1).yaml'), yamlFile('ok.yaml')])
    const dlg = await screen.findByRole('dialog')
    return { ...utils, dlg: within(dlg) }
  }

  it('fenêtre : nom, fichier, « utilisé : … », Remplacer / Garder les deux / Annuler ; posée après le traitement des autres', async () => {
    const { dlg } = await open()
    expect(dlg.getByText(/« Aller-retour » existe déjà/)).toBeInTheDocument()
    expect(dlg.getByText(/aller-retour \(1\)\.yaml/)).toBeInTheDocument()
    expect(dlg.getByText(/utilisé : Public/)).toBeInTheDocument()
    for (const n of [/Remplacer/, /Garder les deux/, /Annuler/]) expect(dlg.getByRole('button', { name: n })).toBeInTheDocument()
    expect(screen.getByTestId('import-result')).toHaveTextContent(/ok\.yaml/)
  })

  it('Remplacer : renvoie ce seul fichier avec on_conflict=replace', async () => {
    const { dlg } = await open()
    fireEvent.click(dlg.getByRole('button', { name: /Remplacer/ }))
    await waitFor(() => expect(calls(/import$/, 'POST')).toHaveLength(2))
    const body = calls(/import$/, 'POST')[1][1].body
    expect(body.get('on_conflict')).toBe('replace')
    expect(body.getAll('files').map((f) => f.name)).toEqual(['aller-retour (1).yaml'])
  })

  it('Garder les deux : renvoie ce seul fichier avec on_conflict=keep_both', async () => {
    const { dlg } = await open()
    fireEvent.click(dlg.getByRole('button', { name: /Garder les deux/ }))
    await waitFor(() => expect(calls(/import$/, 'POST')).toHaveLength(2))
    const body = calls(/import$/, 'POST')[1][1].body
    expect(body.get('on_conflict')).toBe('keep_both')
    expect(body.getAll('files')).toHaveLength(1)
  })

  it('Annuler : aucun nouvel envoi, la fenêtre se ferme', async () => {
    const { dlg } = await open()
    fireEvent.click(dlg.getByRole('button', { name: /Annuler/ }))
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(calls(/import$/, 'POST')).toHaveLength(1)
  })

  it('Échap ferme la fenêtre comme Annuler', async () => {
    await open()
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(calls(/import$/, 'POST')).toHaveLength(1)
  })

  it('jamais de question pour un effet livré : statut native = ligne refusée, pas de fenêtre', async () => {
    fetchMock(() => json(200, { results: [{ file: 'pulsation.yaml', status: 'native', id: 'pulsation', errors: [{ line: 2, message: 'identifiant d\'un effet livré' }] }], catalogue: CATALOG }))
    const { container } = mount()
    pick(container, [yamlFile('pulsation.yaml')])
    await screen.findByTestId('import-result')
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})

describe('Supprimer / Revenir à l\'effet d\'origine (§03, §11.5)', () => {
  it('Supprimer un effet utilisé : confirmation citant « utilisé : … » et le fichier ; rien n\'est envoyé avant confirmation', () => {
    fetchMock(() => json(200, {}))
    mount()
    fireEvent.click(card('aller-retour').getByRole('button', { name: /Supprimer/ }))
    const dlg = within(screen.getByRole('dialog'))
    expect(dlg.getByText(/Supprimer « Aller-retour » ?/)).toBeInTheDocument()
    expect(dlg.getByText(/utilisé : Public/)).toBeInTheDocument()
    expect(dlg.getByText(/aller-retour\.yaml/)).toBeInTheDocument()
    expect(dlg.getByRole('button', { name: /Télécharger/ }) || dlg.getByRole('link', { name: /Télécharger/ })).toBeTruthy()
    expect(calls(/effects\/aller-retour$/, 'DELETE')).toHaveLength(0)
  })

  it('effet non utilisé : même fenêtre sans le paragraphe « utilisé »', () => {
    mount()
    fireEvent.click(card('vague').getByRole('button', { name: /Supprimer/ }))
    expect(within(screen.getByRole('dialog')).queryByText(/utilisé :/)).toBeNull()
  })

  it('confirmer envoie DELETE /api/lighting/effects/{id} et remonte le catalogue de la réponse', async () => {
    const after = { ...CATALOG, effects: CATALOG.effects.filter((e) => e.id !== 'vague') }
    fetchMock(() => json(200, { ok: true, catalogue: after }))
    const { onCatalogue } = mount()
    fireEvent.click(card('vague').getByRole('button', { name: /Supprimer/ }))
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: /^Supprimer$/ }))
    await waitFor(() => expect(calls(/effects\/vague$/, 'DELETE')).toHaveLength(1))
    await waitFor(() => expect(onCatalogue).toHaveBeenCalledWith(after))
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('Annuler : aucune requête', () => {
    fetchMock(() => json(200, {}))
    mount()
    fireEvent.click(card('vague').getByRole('button', { name: /Supprimer/ }))
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: /Annuler/ }))
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(calls(/effects\/vague/, 'DELETE')).toHaveLength(0)
  })

  it('Revenir à l\'effet d\'origine : fenêtre dédiée, puis DELETE du même effet', async () => {
    fetchMock(() => json(200, { ok: true, catalogue: CATALOG }))
    mount()
    fireEvent.click(card('degrade').getByRole('button', { name: /Revenir à l'effet d'origine/ }))
    const dlg = within(screen.getByRole('dialog'))
    expect(dlg.getByText(/Revenir au « Dégradé » d'origine ?/)).toBeInTheDocument()
    expect(dlg.getByText(/degrade\.yaml/)).toBeInTheDocument()
    fireEvent.click(dlg.getByRole('button', { name: /Revenir à l'effet d'origine/ }))
    await waitFor(() => expect(calls(/effects\/degrade$/, 'DELETE')).toHaveLength(1))
  })

  it('échec 409 / 404 : message du serveur, la fenêtre ne ment pas (catalogue inchangé)', async () => {
    fetchMock(() => json(409, { reason: 'native' }))
    const { onCatalogue } = mount()
    fireEvent.click(card('vague').getByRole('button', { name: /Supprimer/ }))
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: /^Supprimer$/ }))
    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(onCatalogue).not.toHaveBeenCalled()
  })
})

describe('Essayer sur la salle — choix de la zone (§01, E7)', () => {
  it('la liste ne propose que les zones qui ont au moins un groupe (Ambiance, Animateur ; pas Public)', () => {
    mount()
    fireEvent.click(card('pulsation').getByRole('button', { name: /zone|▾|Essayer/ }))
    const menu = screen.getByRole('menu')
    expect(within(menu).getByText(/Ambiance/)).toBeInTheDocument()
    expect(within(menu).getByText(/Animateur/)).toBeInTheDocument()
    expect(within(menu).queryByText(/^Public/)).toBeNull()
  })

  it('choisir une zone lance l\'essai sur cette zone (onTry reçoit l\'id et la zone)', async () => {
    const onTry = vi.fn().mockResolvedValue({ ok: true, duration_ms: 10000 })
    mount({ onTry })
    fireEvent.click(card('pulsation').getByRole('button', { name: /zone|▾|Essayer/ }))
    fireEvent.click(within(screen.getByRole('menu')).getByText(/Animateur/))
    await waitFor(() => expect(onTry).toHaveBeenCalled())
    expect(onTry.mock.calls[0][0]).toBe('pulsation')
    expect(JSON.stringify(onTry.mock.calls[0].slice(1))).toContain('animateur')
  })

  it('zone grisée avec la raison quand l\'effet demande plus de groupes que la zone n\'en a', () => {
    mount()
    fireEvent.click(card('degrade').getByRole('button', { name: /zone|▾|Essayer/ }))
    const item = within(screen.getByRole('menu')).getByText(/Animateur/).closest('[role="menuitem"], button')
    expect(item).toBeDisabled()
    expect(item.getAttribute('title') || item.textContent).toMatch(/demande 2 groupes/)
  })

  it('la dernière zone choisie est retenue pour les autres cartes (par navigateur)', async () => {
    localStorage.clear()
    mount()
    fireEvent.click(card('pulsation').getByRole('button', { name: /zone|▾|Essayer/ }))
    fireEvent.click(within(screen.getByRole('menu')).getByText(/Animateur/))
    await waitFor(() => expect(JSON.stringify(Object.entries(localStorage))).toMatch(/animateur/))
  })

  it('tryUnavailable(zone) grise la zone concernée avec sa raison', () => {
    mount({ tryUnavailable: (zone) => (zone === 'ambiance' ? 'éclairage forcé sur ON' : '') })
    fireEvent.click(card('pulsation').getByRole('button', { name: /zone|▾|Essayer/ }))
    const amb = within(screen.getByRole('menu')).getByText(/Ambiance/).closest('[role="menuitem"], button')
    expect(amb).toBeDisabled()
    expect(amb.getAttribute('title') || amb.textContent).toMatch(/forcé sur ON/)
  })
})

describe('Non-régression v11.7.0', () => {
  it('« Recharger les fichiers » appelle onReload', () => {
    const { onReload } = mount()
    fireEvent.click(screen.getByRole('button', { name: /Recharger/ }))
    expect(onReload).toHaveBeenCalled()
  })

  it('les cartes gardent « Aperçu », les étiquettes d\'origine et « utilisé : … »', () => {
    mount()
    expect(card('pulsation').getByRole('button', { name: /Aperçu/ })).toBeInTheDocument()
    expect(card('pulsation').getByText(/utilisé : Ambiance/)).toBeInTheDocument()
    expect(card('degrade').getByText(/natif modifié/)).toBeInTheDocument()
    expect(card('aller-retour').getByText(/personnalisé/)).toBeInTheDocument()
  })
})
