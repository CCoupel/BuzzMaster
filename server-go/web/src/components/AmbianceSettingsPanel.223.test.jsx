import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'

vi.mock('./AmbianceSettingsPanel.css', () => ({}))
import AmbianceSettingsPanel from './AmbianceSettingsPanel'

const CATALOG = {
  groups: 2,
  effects: [
    { id: 'pulsation', name: 'Pulsation', moments: ['chrono'], origin: 'native', min_groups: 1, applicable: true },
    { id: 'chenillard', name: 'Chenillard', moments: ['chrono'], origin: 'native', min_groups: 3, applicable: false, reason: '' },
    { id: 'couleur-equipe', name: "Couleur de l'équipe", moments: ['score'], origin: 'native', min_groups: 1, applicable: true },
  ],
  errors: [], unresolved: [],
}
const GLOBAL = { light: { mode: 'AUTO', effects: { chrono: 'pulsation', calm: '', score: '' }, host_effects: { chrono: '', calm: '', score: '' } } }
const IDS = { 'Pendant le chronomètre': 'chrono', 'Phases calmes': 'calm', 'Quand une équipe marque': 'score' }
const sel = (label) => document.querySelector('#asp-eff-room-' + IDS[label])

describe('AmbianceSettingsPanel › effets lumière par moment (#223)', () => {
  it('mode global : trois sélecteurs salle, valeur courante, « (actuel) » sur le défaut', () => {
    render(<AmbianceSettingsPanel mode="global" value={GLOBAL} sections={['lightEffects']} catalog={CATALOG} onChange={() => {}} />)
    expect(sel('Pendant le chronomètre').value).toBe('pulsation')
    expect(within(sel('Pendant le chronomètre')).getByText('Pulsation (actuel)')).toBeInTheDocument()
    expect(sel('Phases calmes').value).toBe('')
    expect(sel('Quand une équipe marque')).toBeInTheDocument()
  })

  it('un effet inapplicable est grisé avec la raison', () => {
    render(<AmbianceSettingsPanel mode="global" value={GLOBAL} sections={['lightEffects']} catalog={CATALOG} onChange={() => {}} />)
    const opt = within(sel('Pendant le chronomètre')).getByText('Chenillard — demande 3 groupes, 2 configurés')
    expect(opt).toBeDisabled()
  })

  it('changer un sélecteur appelle onChange(light.effects.<moment>, id)', () => {
    const onChange = vi.fn()
    render(<AmbianceSettingsPanel mode="global" value={GLOBAL} sections={['lightEffects']} catalog={CATALOG} onChange={onChange} />)
    fireEvent.change(sel('Pendant le chronomètre'), { target: { value: '' } })
    expect(onChange).toHaveBeenCalledWith('light.effects.chrono', '')
  })

  it('sans groupe animateur : ligne « Aucun groupe animateur — en créer un » ; avec : bloc dédié', () => {
    const onCreateHost = vi.fn()
    const { rerender } = render(<AmbianceSettingsPanel mode="global" value={GLOBAL} sections={['lightEffects']} catalog={CATALOG} onChange={() => {}} onCreateHost={onCreateHost} />)
    expect(screen.getByText(/Aucun groupe animateur/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'en créer un' }))
    expect(onCreateHost).toHaveBeenCalled()
    rerender(<AmbianceSettingsPanel mode="global" value={GLOBAL} sections={['lightEffects']} catalog={CATALOG} hostGroup={{ name: 'Pupitre' }} onChange={() => {}} />)
    expect(screen.getByText(/Groupe animateur « Pupitre »/)).toBeInTheDocument()
    const host = document.querySelector('#asp-eff-host-chrono')
    expect(within(host).getByText('Blanc fixe (actuel)')).toBeInTheDocument()
    expect(within(host).getByText('Chenillard — demande plusieurs groupes')).toBeDisabled()
    fireEvent.change(document.querySelector('#asp-eff-host-score'), { target: { value: 'couleur-equipe' } })
  })

  it('avertissement salle : effet inapplicable (lien « Gérer les groupes ») et fichier introuvable', () => {
    const onManageGroups = vi.fn()
    const value = { light: { ...GLOBAL.light, effects: { chrono: 'chenillard', calm: 'mon-effet', score: '' } } }
    render(<AmbianceSettingsPanel mode="global" value={value} sections={['lightEffects']} catalog={CATALOG} onChange={() => {}} onManageGroups={onManageGroups} onOpenCatalogue={() => {}} />)
    expect(screen.getByText(/Cet effet demande 3 groupes et 2 sont configurés : la pulsation est jouée à la place/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Gérer les groupes' }))
    expect(onManageGroups).toHaveBeenCalled()
    expect(screen.getByText(/absent ou invalide/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Voir le catalogue' })).toBeInTheDocument()
  })

  it('mode surcharge : « Hérité (valeur globale) », badge « manche », null retire la surcharge', () => {
    const onChange = vi.fn()
    const { rerender } = render(<AmbianceSettingsPanel mode="override" value={{}} base={GLOBAL} sections={['lightEffects']} catalog={CATALOG} onChange={onChange} />)
    expect(sel('Pendant le chronomètre').value).toBe('__inherit__')
    expect(screen.getByText('Hérité (Pulsation)')).toBeInTheDocument()
    fireEvent.change(sel('Pendant le chronomètre'), { target: { value: '' } })
    expect(onChange).toHaveBeenCalledWith('light.effects.chrono', '')
    rerender(<AmbianceSettingsPanel mode="override" value={{ light: { effects: { chrono: '' } } }} base={GLOBAL} sections={['lightEffects']} catalog={CATALOG} onChange={onChange} />)
    expect(screen.getByText('manche')).toBeInTheDocument()
    fireEvent.change(sel('Pendant le chronomètre'), { target: { value: '__inherit__' } })
    expect(onChange).toHaveBeenLastCalledWith('light.effects.chrono', null)
  })

  it('catalogue absent (indéfini) : section masquée', () => {
    render(<AmbianceSettingsPanel mode="global" value={GLOBAL} sections={['lightEffects']} onChange={() => {}} />)
    expect(screen.queryByTestId('asp-light-effects')).toBeNull()
  })
})
