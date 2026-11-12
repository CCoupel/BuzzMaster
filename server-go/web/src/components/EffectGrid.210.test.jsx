import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import EffectGrid from './EffectGrid'
import { gridRows } from '../utils/effectDocument'

// #210 (v11.8.0, LOT-2, F2.2) — grille pas × cibles. Source : contracts/light-effects.md §11.8 (révision 7), maquette effect_editor_r4 §01.
// Lignes : variables en haut, puis g0, g1…g<n>, et TOUJOURS g* en dernier ; colonnes : PRE, pas 0…n, DERNIER, POST.

const A = (target, color = 'theme', intensity = 100) => ({ target, color, intensity })
const VARS = [{ name: 'gA', kind: 'progression', start: 'N/2', step: '+1' }, { name: 'gB', kind: 'progression', start: 'gA+1', step: null }]
const doc = (over = {}) => ({
  id: 'e', name: 'E', description: '', moments: ['chrono'], min_groups: 0, tick_ms: 1000, loop: true,
  variables: VARS, pre: [A('g0', 'scene', 20)], steps: [[A('g0'), A('gA', 'urgence')], [A('g*', '#00FF00', 60)]], last: null, post: null, ...over,
})
const handlers = () => ({
  onCell: vi.fn(), onAddStep: vi.fn(), onDuplicateStep: vi.fn(), onInsertStep: vi.fn(), onMoveStep: vi.fn(), onRemoveStep: vi.fn(),
  onAddGroup: vi.fn(), onRemoveGroup: vi.fn(),
})
const mount = (props = {}) => {
  const h = handlers()
  const utils = render(<EffectGrid doc={doc()} numbered={5} {...h} {...props} />)
  return { ...utils, h }
}
const rowLabels = (container) => [...container.querySelectorAll('tbody tr th[scope="row"]')].map(th => th.firstChild.textContent)

describe('lignes de la grille', () => {
  it('variables en haut (ordre de variables), puis g0, g1…g5, et g* en dernier', () => {
    const { container } = mount()
    expect(rowLabels(container)).toEqual(['gA', 'gB', 'g0', 'g1', 'g2', 'g3', 'g4', 'g5', 'g*'])
  })

  it('la dernière ligne est toujours g*, quel que soit le nombre de lignes', () => {
    for (const n of [1, 5, 12]) {
      const { container, unmount } = mount({ numbered: n })
      expect(rowLabels(container).at(-1)).toBe('g*')
      expect(rowLabels(container).filter(l => /^g[1-9]\d*$/.test(l))).toHaveLength(n)
      unmount()
    }
  })

  it('bandeaux « Variables » et « Fond et groupes fixes » ; la ligne g* est annotée « dernier groupe »', () => {
    mount()
    expect(screen.getByText(/Variables — repeignent/)).toBeInTheDocument()
    expect(screen.getByText(/Fond et groupes fixes/)).toBeInTheDocument()
    expect(screen.getByText('dernier groupe')).toBeInTheDocument()
  })

  it('pas de ligne « voisin » : aucune ligne gA+1 / gA-1 n\'est créée', () => {
    expect(gridRows(doc(), 5).some(r => /[+-]\d/.test(r.target))).toBe(false)
  })
})

describe('colonnes et cases', () => {
  it('colonnes PRE, un pas par tick, DERNIER, POST', () => {
    mount()
    const heads = [...document.querySelectorAll('thead th')].map(t => t.textContent)
    expect(heads.join('|')).toMatch(/PRE.*pas 0.*pas 1.*DERNIER.*POST/)
  })

  it('une case vide n\'écrit rien (aria « vide »), une case remplie dit sa couleur et son intensité', () => {
    mount()
    expect(screen.getByTestId('cell-0-g3')).toHaveAccessibleName(/vide/)
    expect(screen.getByTestId('cell-1-g*')).toHaveAccessibleName(/60 %/)
    expect(screen.getByTestId('cell-pre-g0')).toHaveAccessibleName(/scène 20 %/)
  })

  it('un clic sur une case appelle onCell(colonne, cible)', () => {
    const { h } = mount()
    fireEvent.click(screen.getByTestId('cell-0-g2'))
    fireEvent.click(screen.getByTestId('cell-pre-gA'))
    fireEvent.click(screen.getByTestId('cell-post-g*'))
    expect(h.onCell.mock.calls).toEqual([[0, 'g2'], ['pre', 'gA'], ['post', 'g*']])
  })

  it('lecture seule : cases désactivées, aucun bouton d\'ajout, ni menu de pas', () => {
    const { h } = mount({ readOnly: true })
    expect(screen.getByTestId('cell-0-g2')).toBeDisabled()
    fireEvent.click(screen.getByTestId('cell-0-g2'))
    expect(h.onCell).not.toHaveBeenCalled()
    expect(screen.queryByRole('button', { name: /Ajouter un groupe/ })).toBeNull()
    expect(screen.queryByRole('button', { name: /Ajouter un pas/ })).toBeNull()
    expect(screen.queryByRole('button', { name: /Menu du/ })).toBeNull()
  })

  it('colonne en erreur et colonne jouée sont marquées', () => {
    mount({ errorColumns: new Set([1]), playingColumn: 0 })
    expect(screen.getByTestId('cell-1-g1').closest('td')).toHaveClass('is-err')
    expect(screen.getByTestId('cell-0-g1').closest('td')).toHaveClass('is-playing')
  })

  it('clic sur l\'en-tête d\'un pas : place l\'aperçu (onGoStep)', () => {
    const onGoStep = vi.fn()
    mount({ onGoStep })
    const heads = screen.getAllByTitle("Placer l'aperçu sur ce pas")
    fireEvent.click(heads[1]) // PRE, pas 0, pas 1, DERNIER, POST
    expect(onGoStep).toHaveBeenCalledWith(0)
  })
})

describe('menu d\'un pas', () => {
  it('Dupliquer / Insérer avant / Déplacer / Supprimer, avec le bon pas', () => {
    const { h } = mount()
    fireEvent.click(screen.getByRole('button', { name: 'Menu du pas 1' }))
    const menu = within(screen.getByRole('menu'))
    fireEvent.click(menu.getByRole('menuitem', { name: /Dupliquer/ }))
    expect(h.onDuplicateStep).toHaveBeenCalledWith(1)
    fireEvent.click(screen.getByRole('button', { name: 'Menu du pas 1' }))
    fireEvent.click(within(screen.getByRole('menu')).getByRole('menuitem', { name: /Déplacer vers la gauche/ }))
    expect(h.onMoveStep).toHaveBeenCalledWith(1, -1)
    fireEvent.click(screen.getByRole('button', { name: 'Menu du pas 0' }))
    fireEvent.click(within(screen.getByRole('menu')).getByRole('menuitem', { name: /Supprimer/ }))
    expect(h.onRemoveStep).toHaveBeenCalledWith(0)
  })

  it('premier pas : pas de déplacement à gauche ; dernier : pas à droite ; pas unique : pas de suppression', () => {
    mount()
    fireEvent.click(screen.getByRole('button', { name: 'Menu du pas 0' }))
    expect(screen.getByRole('menuitem', { name: /vers la gauche/ })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: 'Menu du pas 1' }))
    expect(screen.getByRole('menuitem', { name: /vers la droite/ })).toBeDisabled()
  })

  it('le menu se ferme à Échap', () => {
    mount()
    fireEvent.click(screen.getByRole('button', { name: 'Menu du pas 0' }))
    expect(screen.getByRole('menu')).toBeInTheDocument()
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(screen.queryByRole('menu')).toBeNull()
  })

  it('« ＋ » ajoute un pas', () => {
    const { h } = mount()
    fireEvent.click(screen.getByRole('button', { name: 'Ajouter un pas' }))
    expect(h.onAddStep).toHaveBeenCalled()
  })
})

describe('Ajouter / Retirer un groupe', () => {
  it('les deux boutons appellent leurs rappels ; l\'indication compte les lignes (n + g*)', () => {
    const { h } = mount()
    fireEvent.click(screen.getByRole('button', { name: /Ajouter un groupe/ }))
    fireEvent.click(screen.getByRole('button', { name: /Retirer un groupe/ }))
    expect(h.onAddGroup).toHaveBeenCalledTimes(1)
    expect(h.onRemoveGroup).toHaveBeenCalledTimes(1)
    expect(screen.getByText(/6 lignes de groupes.*dernière ligne est toujours g\*/)).toBeInTheDocument()
  })

  it('12 lignes numérotées (degrade) : « Ajouter un groupe » indisponible', () => {
    mount({ numbered: 12 })
    expect(screen.getByRole('button', { name: /Ajouter un groupe/ })).toBeDisabled()
  })

  it('une seule ligne numérotée restante : on peut encore retirer (jusqu\'à 1 ligne au moins) ; zéro : non', () => {
    mount({ numbered: 0 })
    expect(screen.getByRole('button', { name: /Retirer un groupe/ })).toBeDisabled()
  })
})
