import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import EffectVariablesPanel from './EffectVariablesPanel'

// #210 (v11.8.0, LOT-2, F2.2) — panneau « Variables de groupe ». Source : contracts/light-effects.md §11.8 (variables, bulle « ? »,
// suite lue dans `vars` des aperçus), §12.2 (expressions vérifiées par le serveur), maquette effect_editor_r4 §01.

const VARS = [
  { name: 'gA', kind: 'progression', start: 'N/2', step: '+1' },
  { name: 'gB', kind: 'progression', start: 'gA+1', step: null },
  { name: 'gC', kind: 'list', items: ['g1', 'g4', 'g3'] },
]
const fr = (tick, vars) => ({ at_ms: tick * 1000, tick, step: 'pas:0', groups: [], vars })
const PREVIEWS = [1, 3, 6, 12].map(n => ({ groups: n, frames: [0, 1, 2].map(k => fr(k, { gA: ((n >> 1) + k) % n + 1, gB: 99 })) }))

const mount = (over = {}) => {
  const h = { onAdd: vi.fn(), onKind: vi.fn(), onPatch: vi.fn(), onMove: vi.fn(), onRemove: vi.fn() }
  render(<EffectVariablesPanel variables={VARS} previews={PREVIEWS} {...h} {...over} />)
  return h
}

describe('lignes de variables', () => {
  it('une ligne par variable, dans l\'ordre, avec son type', () => {
    mount()
    for (const v of VARS) expect(screen.getByTestId(`var-${v.name}`)).toBeInTheDocument()
    expect(screen.getByLabelText('Type de gA')).toHaveValue('progression')
    expect(screen.getByLabelText('Type de gC')).toHaveValue('list')
    expect(within(screen.getByLabelText('Type de gA')).getByText('Avance régulière')).toBeInTheDocument()
    expect(within(screen.getByLabelText('Type de gA')).getByText('Suite de groupes')).toBeInTheDocument()
  })

  it('départ et pas sont des champs texte d\'expression ; pas vide = « suit »', () => {
    const h = mount()
    expect(screen.getByLabelText('Départ de gA')).toHaveValue('N/2')
    expect(screen.getByLabelText('Pas de gA')).toHaveValue('+1')
    expect(screen.getByLabelText('Pas de gB')).toHaveValue('')
    expect(screen.getByLabelText('Pas de gB')).toHaveAttribute('placeholder', 'suit')
    fireEvent.change(screen.getByLabelText('Départ de gA'), { target: { value: '(N+1)/2' } })
    expect(h.onPatch).toHaveBeenCalledWith(0, { start: '(N+1)/2' })
    fireEvent.change(screen.getByLabelText('Pas de gA'), { target: { value: '-N/3' } })
    expect(h.onPatch).toHaveBeenCalledWith(0, { step: '-N/3' })
    fireEvent.change(screen.getByLabelText('Pas de gA'), { target: { value: '  ' } })
    expect(h.onPatch).toHaveBeenLastCalledWith(0, { step: null })
  })

  it('propositions de départ : 1, N/2, N, gA+1', () => {
    mount()
    const opts = [...document.querySelectorAll('datalist option')].map(o => o.value)
    for (const s of ['1', 'N/2', 'N', 'gA+1']) expect(opts).toContain(s)
  })

  it('suite de groupes : champ « g1, g4, g3 » validé à la sortie du champ', () => {
    const h = mount()
    const input = screen.getByLabelText('Groupes suivis par gC')
    expect(input).toHaveValue('g1, g4, g3')
    fireEvent.change(input, { target: { value: '2, 5 ,13' } })
    fireEvent.blur(input)
    expect(h.onPatch).toHaveBeenCalledWith(2, { items: ['g2', 'g5'] })
  })

  it('changement de type, ordre et suppression appellent leurs rappels', () => {
    const h = mount()
    fireEvent.change(screen.getByLabelText('Type de gB'), { target: { value: 'list' } })
    expect(h.onKind).toHaveBeenCalledWith(1, 'list')
    fireEvent.click(screen.getByLabelText('Monter gB'))
    fireEvent.click(screen.getByLabelText('Descendre gB'))
    expect(h.onMove.mock.calls).toEqual([[1, -1], [1, 1]])
    fireEvent.click(screen.getByLabelText('Supprimer gC'))
    expect(h.onRemove).toHaveBeenCalledWith(2)
  })

  it('la première variable ne monte pas, la dernière ne descend pas', () => {
    mount()
    expect(screen.getByLabelText('Monter gA')).toBeDisabled()
    expect(screen.getByLabelText('Descendre gC')).toBeDisabled()
  })

  it('+ Ajouter : désactivé à 24 variables', () => {
    const h = mount()
    fireEvent.click(screen.getByRole('button', { name: '+ Ajouter' }))
    expect(h.onAdd).toHaveBeenCalled()
    const many = Array.from({ length: 24 }, (_, i) => ({ name: `g${String.fromCharCode(65 + i)}`, kind: 'progression', start: '1', step: '+1' }))
    render(<EffectVariablesPanel variables={many} previews={[]} onAdd={vi.fn()} onKind={vi.fn()} onPatch={vi.fn()} onMove={vi.fn()} onRemove={vi.fn()} />)
    expect(screen.getAllByRole('button', { name: '+ Ajouter' }).at(-1)).toBeDisabled()
  })

  it('sans variable : message d\'aide ; au-delà de 8 lignes le bloc défile', () => {
    const { container } = render(<EffectVariablesPanel variables={[]} previews={[]} onAdd={vi.fn()} onKind={vi.fn()} onPatch={vi.fn()} onMove={vi.fn()} onRemove={vi.fn()} />)
    expect(container.textContent).toMatch(/Aucune variable/)
    const nine = Array.from({ length: 9 }, (_, i) => ({ name: `g${String.fromCharCode(65 + i)}`, kind: 'progression', start: '1', step: '+1' }))
    const r = render(<EffectVariablesPanel variables={nine} previews={[]} onAdd={vi.fn()} onKind={vi.fn()} onPatch={vi.fn()} onMove={vi.fn()} onRemove={vi.fn()} />)
    expect(r.container.querySelector('.efx-vars')).toHaveClass('is-scroll')
  })

  it('désactivé (effet livré) : aucun champ modifiable', () => {
    mount({ disabled: true })
    expect(screen.getByLabelText('Départ de gA')).toBeDisabled()
    expect(screen.getByLabelText('Type de gA')).toBeDisabled()
    expect(screen.getByRole('button', { name: '+ Ajouter' })).toBeDisabled()
  })

  it('erreur d\'expression du serveur placée sous la variable (path variables[i].start)', () => {
    mount({ errors: { 0: ['l\'expression « N//2 » est mal écrite'] } })
    expect(within(screen.getByTestId('var-gA')).getByRole('alert')).toHaveTextContent(/N\/\/2/)
    expect(within(screen.getByTestId('var-gB')).queryByRole('alert')).toBeNull()
  })
})

describe('bulle « ? » (§11.8 révision 6)', () => {
  it('fermée par défaut ; la suite pour 1, 3, 6 et 12 groupes n\'apparaît que dans la bulle', () => {
    mount()
    expect(screen.queryByText(/12 groupes :/)).toBeNull()
    fireEvent.click(screen.getByLabelText('Ce que fait gA'))
    const pop = within(screen.getByRole('dialog', { name: 'Ce que fait gA' }))
    for (const n of ['1 groupe', '3 groupes', '6 groupes', '12 groupes']) expect(pop.getByText(new RegExp(`${n} :`))).toBeInTheDocument()
  })

  it('la suite est LUE dans vars des aperçus, jamais calculée', () => {
    mount({ previews: [{ groups: 3, frames: [fr(0, { gA: 2 }), fr(1, { gA: 3 }), fr(2, { gA: 1 })] }] })
    fireEvent.click(screen.getByLabelText('Ce que fait gA'))
    expect(screen.getByText(/3 groupes :/).parentElement).toHaveTextContent('g2 g3 g1')
    expect(screen.getByText(/6 groupes :/).parentElement).toHaveTextContent('…') // pas d'aperçu : rien n'est inventé
  })

  it('valeur 0 dans vars (aucun groupe) affichée « — »', () => {
    mount({ previews: [{ groups: 1, frames: [fr(0, { gA: 0 })] }] })
    fireEvent.click(screen.getByLabelText('Ce que fait gA'))
    expect(screen.getByText(/1 groupe :/).parentElement).toHaveTextContent('—')
  })

  it('se ferme à Échap et au clic suivant ailleurs', () => {
    mount()
    fireEvent.click(screen.getByLabelText('Ce que fait gA'))
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(screen.queryByRole('dialog', { name: 'Ce que fait gA' })).toBeNull()
    fireEvent.click(screen.getByLabelText('Ce que fait gB'))
    fireEvent.mouseDown(document.body)
    expect(screen.queryByRole('dialog', { name: 'Ce que fait gB' })).toBeNull()
  })

  it('une seule bulle ouverte à la fois ; le bouton « ? » porte aria-expanded', () => {
    mount()
    const qa = screen.getByLabelText('Ce que fait gA')
    expect(qa).toHaveAttribute('aria-expanded', 'false')
    fireEvent.click(qa)
    expect(qa).toHaveAttribute('aria-expanded', 'true')
    fireEvent.click(screen.getByLabelText('Ce que fait gB'))
    expect(screen.getAllByRole('dialog')).toHaveLength(1)
  })

  it('bulle générale : explique N, la division entière, le rebouclage et « suit »', () => {
    mount()
    fireEvent.click(screen.getByLabelText('À quoi servent les variables ?'))
    const t = screen.getByRole('dialog', { name: 'À quoi servent les variables ?' }).textContent
    expect(t).toMatch(/nombre de groupes/)
    expect(t).toMatch(/partie entière/)
    expect(t).toMatch(/reboucle/)
    expect(t).toMatch(/sans pas/)
  })

  it('la phrase d\'une variable qui suit mentionne la variable citée', () => {
    mount()
    fireEvent.click(screen.getByLabelText('Ce que fait gB'))
    expect(screen.getByRole('dialog', { name: 'Ce que fait gB' })).toHaveTextContent(/gA\+1/)
  })
})
