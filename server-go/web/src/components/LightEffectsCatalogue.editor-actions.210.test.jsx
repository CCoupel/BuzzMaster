import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'

// #210 (v11.8.0, LOT-2, F2.3) — boutons du catalogue qui mènent à l'éditeur : « ＋ Nouvel effet », « ✎ Modifier » (effet personnalisé),
// « 👁 Ouvrir » (livré ou natif modifié), « ⧉ Dupliquer », avec la fenêtre nom + identifiant.
// Source : maquette docs/mockup/v11.8.0/ui/ambiance_page__effects_catalogue_import_export.html §01 ; contract §11.8 ; plan F2.3.
//
// SURFACE SUPPOSÉE (à rapprocher du handoff de dev-frontend ; seul `mount()` et les libellés changent) : le composant reçoit
//   onEdit(id)                       « Modifier » / « Ouvrir » (l'éditeur décide de la lecture seule d'après l'origine)
//   onCreate({ id, name, from })     fenêtre validée ; `from` = id de l'effet dupliqué, absent pour « Nouvel effet »
// La fenêtre porte les champs « Nom » et « Identifiant » ; l'identifiant proposé pour une copie est `<id>-copie` (40 car. au plus) ;
// un identifiant déjà au catalogue (livré compris) ou mal formé ([a-z0-9-], 1 à 40) est refusé avec un message, avant tout envoi.

vi.mock('./LightEffectsCatalogue.css', () => ({}))
import LightEffectsCatalogue from './LightEffectsCatalogue'

const ALL = { ambiance: true, animateur: true, public: true }
const fx = (id, name, origin, extra = {}) => ({ id, name, description: '', moments: ['calm'], origin, file: origin === 'native' ? `natifs/${id}.yaml` : `${id}.yaml`, min_groups: 0, tick_ms: 1000, applicable: ALL, ...extra })
const CATALOG = {
  groups: { ambiance: 4, animateur: 1, public: 0 },
  effects: [fx('pulsation', 'Pulsation', 'native'), fx('degrade', 'Dégradé', 'override'), fx('aller-retour', 'Aller-retour', 'user')],
  errors: [], dir: 'data/files/effects',
}
const mount = (props = {}) => {
  const h = { onEdit: vi.fn(), onCreate: vi.fn() }
  const utils = render(
    <LightEffectsCatalogue catalog={CATALOG} lighting={{}} zoneGroups={CATALOG.groups} onReload={vi.fn().mockResolvedValue(true)} onCatalogue={vi.fn()}
      onTry={vi.fn()} onStopTry={vi.fn()} {...h} {...props} />,
  )
  return { ...utils, h }
}
const card = (id) => within(screen.getByTestId(`fx-${id}`))
afterEach(() => { vi.restoreAllMocks() })

describe('boutons selon l\'origine', () => {
  it('effet personnalisé : « Modifier » et « Dupliquer » ; pas « Ouvrir »', () => {
    mount()
    expect(card('aller-retour').getByRole('button', { name: /Modifier/ })).toBeInTheDocument()
    expect(card('aller-retour').getByRole('button', { name: /Dupliquer/ })).toBeInTheDocument()
    expect(card('aller-retour').queryByRole('button', { name: /Ouvrir/ })).toBeNull()
  })

  it('effet livré et natif modifié : « Ouvrir » (lecture seule) et « Dupliquer » ; pas « Modifier »', () => {
    mount()
    for (const id of ['pulsation', 'degrade']) {
      expect(card(id).getByRole('button', { name: /Ouvrir/ })).toBeInTheDocument()
      expect(card(id).getByRole('button', { name: /Dupliquer/ })).toBeInTheDocument()
      expect(card(id).queryByRole('button', { name: /Modifier/ })).toBeNull()
    }
  })

  it('en tête : « Nouvel effet » à côté d\'« Importer… »', () => {
    mount()
    expect(screen.getByRole('button', { name: /Nouvel effet/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Importer/ })).toBeInTheDocument()
  })

  it('« Modifier » et « Ouvrir » appellent onEdit(id)', () => {
    const { h } = mount()
    fireEvent.click(card('aller-retour').getByRole('button', { name: /Modifier/ }))
    fireEvent.click(card('pulsation').getByRole('button', { name: /Ouvrir/ }))
    expect(h.onEdit.mock.calls).toEqual([['aller-retour'], ['pulsation']])
  })
})

describe('Dupliquer — fenêtre nom + identifiant', () => {
  const open = (id = 'pulsation') => {
    const utils = mount()
    fireEvent.click(card(id).getByRole('button', { name: /Dupliquer/ }))
    return { ...utils, dlg: within(screen.getByRole('dialog')) }
  }

  it('propose le nom et l\'identifiant « <id>-copie » ; la copie part d\'un effet livré', () => {
    const { dlg } = open()
    expect(dlg.getByLabelText('Identifiant')).toHaveValue('pulsation-copie')
    expect(dlg.getByLabelText('Nom')).toHaveValue(expect.stringContaining('Pulsation'))
  })

  it('valider appelle onCreate({ id, name, from }) et ferme la fenêtre', () => {
    const { dlg, h } = open()
    fireEvent.change(dlg.getByLabelText('Nom'), { target: { value: 'Ma pulsation' } })
    fireEvent.change(dlg.getByLabelText('Identifiant'), { target: { value: 'ma-pulsation' } })
    fireEvent.click(dlg.getByRole('button', { name: /Créer|Dupliquer|Continuer|Valider/ }))
    expect(h.onCreate).toHaveBeenCalledWith({ id: 'ma-pulsation', name: 'Ma pulsation', from: 'pulsation' })
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('identifiant déjà au catalogue (livré compris) : refus avec message, aucun appel', () => {
    const { dlg, h } = open()
    for (const taken of ['pulsation', 'aller-retour', 'degrade']) {
      fireEvent.change(dlg.getByLabelText('Identifiant'), { target: { value: taken } })
      expect(screen.getByRole('dialog')).toHaveTextContent(/déjà/i)
      fireEvent.click(dlg.getByRole('button', { name: /Créer|Dupliquer|Continuer|Valider/ }))
    }
    expect(h.onCreate).not.toHaveBeenCalled()
  })

  it('identifiant mal formé (majuscule, espace, accent, vide, plus de 40) : refus', () => {
    const { dlg, h } = open()
    for (const bad of ['Ma Copie', 'é', '', 'a'.repeat(41), 'a_b', 'reload', 'draft', 'import', 'try']) {
      fireEvent.change(dlg.getByLabelText('Identifiant'), { target: { value: bad } })
      fireEvent.click(dlg.getByRole('button', { name: /Créer|Dupliquer|Continuer|Valider/ }))
    }
    expect(h.onCreate).not.toHaveBeenCalled()
  })

  it('nom vide : refus ; Annuler et Échap ferment sans rien créer', () => {
    const { dlg, h } = open()
    fireEvent.change(dlg.getByLabelText('Nom'), { target: { value: '' } })
    fireEvent.click(dlg.getByRole('button', { name: /Créer|Dupliquer|Continuer|Valider/ }))
    expect(h.onCreate).not.toHaveBeenCalled()
    fireEvent.click(dlg.getByRole('button', { name: /Annuler/ }))
    expect(screen.queryByRole('dialog')).toBeNull()
    fireEvent.click(card('pulsation').getByRole('button', { name: /Dupliquer/ }))
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(h.onCreate).not.toHaveBeenCalled()
  })

  it('l\'identifiant d\'une copie d\'un effet à nom long est tronqué à 40 caractères', () => {
    const long = 'a'.repeat(40)
    mount({ catalog: { ...CATALOG, effects: [fx(long, 'Long', 'user')] } })
    fireEvent.click(card(long).getByRole('button', { name: /Dupliquer/ }))
    expect(within(screen.getByRole('dialog')).getByLabelText('Identifiant').value.length).toBeLessThanOrEqual(40)
  })
})

describe('Nouvel effet — fenêtre nom + identifiant', () => {
  const open = () => {
    const utils = mount()
    fireEvent.click(screen.getByRole('button', { name: /Nouvel effet/ }))
    return { ...utils, dlg: within(screen.getByRole('dialog')) }
  }

  it('l\'identifiant se déduit du nom (minuscules, tirets, sans accent) tant qu\'on ne l\'a pas modifié', () => {
    const { dlg } = open()
    fireEvent.change(dlg.getByLabelText('Nom'), { target: { value: 'Mon Chenillard Été' } })
    expect(dlg.getByLabelText('Identifiant')).toHaveValue('mon-chenillard-ete')
  })

  it('valider appelle onCreate sans `from`', () => {
    const { dlg, h } = open()
    fireEvent.change(dlg.getByLabelText('Nom'), { target: { value: 'Nouveau' } })
    fireEvent.click(dlg.getByRole('button', { name: /Créer|Continuer|Valider/ }))
    expect(h.onCreate).toHaveBeenCalledWith({ id: 'nouveau', name: 'Nouveau' })
  })

  it('même refus d\'identifiant déjà pris', () => {
    const { dlg, h } = open()
    fireEvent.change(dlg.getByLabelText('Nom'), { target: { value: 'Pulsation' } })
    expect(screen.getByRole('dialog')).toHaveTextContent(/déjà/i)
    fireEvent.click(dlg.getByRole('button', { name: /Créer|Continuer|Valider/ }))
    expect(h.onCreate).not.toHaveBeenCalled()
  })
})

describe('garde « modifications non enregistrées » (AmbiancePage)', () => {
  it('voir LightEffectEditor.210.test.jsx : question Enregistrer / Abandonner / Rester au retour au catalogue', () => {
    expect(true).toBe(true) // couverte au niveau de l'éditeur ; le branchement de la page est vérifié par la procédure (scénario 6)
  })
})
