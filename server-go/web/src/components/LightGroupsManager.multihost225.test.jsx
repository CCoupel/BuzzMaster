import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

vi.mock('./LightGroupsManager.css', () => ({}))
import LightGroupsManager from './LightGroupsManager'

const CFG = [
  { id: 'g1', role: 'salle', lights: ['A'] },
  { id: 'h1', role: 'animateur', lights: ['B'] },
  { id: 'h2', role: 'animateur', lights: ['C'] },
]
const base = (props = {}) => ({
  generalLights: ['A', 'B', 'C', 'D'], groups: CFG, enabled: true,
  onSave: vi.fn().mockResolvedValue({ ok: true }),
  onTest: vi.fn().mockResolvedValue({ ok: true, duration_ms: 6000 }),
  onStopTest: vi.fn().mockResolvedValue({ ok: true }),
  ...props,
})

describe('LightGroupsManager — groupes animateur multiples, essai en séquence (#225, GATE LOT-1)', () => {
  it('plusieurs groupes animateur numérotés 🎤1, 🎤2 ; plus de « Créer un groupe par ampoule »', () => {
    render(<LightGroupsManager {...base({ groups: [] })} />)
    expect(screen.queryByText(/Créer un groupe par ampoule/)).toBeNull()
    cleanupAndRender(base())
    expect(screen.getByTestId('lgm-host-1')).toBeInTheDocument()
    expect(screen.getByLabelText('Groupe animateur 2')).toHaveTextContent('🎤2')
  })

  it('4 groupes animateur au plus : la création est désactivée', () => {
    const four = [1, 2, 3, 4].map(n => ({ id: `h${n}`, role: 'animateur', lights: [] }))
    render(<LightGroupsManager {...base({ groups: four })} />)
    expect(screen.getByRole('button', { name: /Nouveau groupe animateur/ })).toBeDisabled()
  })

  it('enregistrement : les id sont renvoyés tels quels, un nouveau groupe n\'en a pas', async () => {
    const props = base()
    render(<LightGroupsManager {...props} />)
    fireEvent.click(screen.getByRole('button', { name: /Nouveau groupe animateur/ }))
    fireEvent.click(screen.getByRole('button', { name: /Enregistrer les groupes/ }))
    await waitFor(() => expect(props.onSave).toHaveBeenCalled())
    const payload = props.onSave.mock.calls[0][0]
    expect(payload.map(g => g.id)).toEqual(['g1', 'h1', 'h2', undefined])
    expect(payload[3]).toEqual({ role: 'animateur', lights: [] })
  })

  it('essai : poste host<k>, bouton « Arrêter l\'essai » pendant la séquence, puis stop', async () => {
    const props = base()
    render(<LightGroupsManager {...props} />)
    fireEvent.click(screen.getAllByRole('button', { name: 'Tester le groupe' })[2]) // salle 1, 🎤1, 🎤2 -> 🎤2
    await waitFor(() => expect(props.onTest).toHaveBeenCalledWith('host2'))
    const stop = await screen.findByRole('button', { name: /Arrêter l'essai/ })
    expect(stop.title).toMatch(/Étape 1\/6/)
    fireEvent.click(stop)
    await waitFor(() => expect(props.onStopTest).toHaveBeenCalled())
    await waitFor(() => expect(screen.queryByRole('button', { name: /Arrêter l'essai/ })).toBeNull())
  })

  it('infobulles de désactivation : groupe vide, modifications non enregistrées, éclairage désactivé, pont injoignable', () => {
    const { rerender } = render(<LightGroupsManager {...base({ groups: [{ id: 'g1', role: 'salle', lights: [] }] })} />)
    expect(screen.getByRole('button', { name: 'Tester le groupe' })).toHaveAttribute('title', 'Groupe vide')
    rerender(<LightGroupsManager {...base({ enabled: false })} />)
    expect(screen.getAllByRole('button', { name: 'Tester le groupe' })[0]).toHaveAttribute('title', "Éclairage d'ambiance désactivé")
    rerender(<LightGroupsManager {...base({ testUnavailableReason: 'Pont injoignable' })} />)
    expect(screen.getAllByRole('button', { name: 'Tester le groupe' })[0]).toHaveAttribute('title', 'Pont injoignable')
    fireEvent.click(screen.getByRole('button', { name: /Nouveau groupe$/ }))
    expect(screen.getAllByRole('button', { name: 'Tester le groupe' })[0]).toHaveAttribute('title', "Enregistrez d'abord")
  })

  it('« Choisir ses effets » transmet l\'id du groupe animateur', () => {
    const onChooseEffects = vi.fn()
    render(<LightGroupsManager {...base({ onChooseEffects })} />)
    fireEvent.click(screen.getAllByRole('button', { name: 'Choisir ses effets' })[1])
    expect(onChooseEffects).toHaveBeenCalledWith('h2')
  })

  it('révision 3 : libellés d\'après le rang, ni champ de nom ni « Équipes à proximité »', () => {
    render(<LightGroupsManager {...base()} />)
    expect(screen.getByTestId('lgm-group-1')).toHaveTextContent('Groupe 1')
    expect(screen.getByTestId('lgm-host-2')).toHaveTextContent('Groupe animateur 2')
    expect(screen.queryByRole('textbox')).toBeNull()
    expect(screen.queryByText(/Équipes à proximité/)).toBeNull()
    expect(screen.getByText(/g0/)).toBeInTheDocument()
  })

  it('changer l\'ordre des groupes affiche le message sur g1, g2…', () => {
    const two = [{ id: 'g1', role: 'salle', lights: ['A'] }, { id: 'g2', role: 'salle', lights: ['B'] }]
    render(<LightGroupsManager {...base({ groups: two })} />)
    expect(screen.queryByText(/L'ordre a changé/)).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Descendre le groupe 1' }))
    expect(screen.getByText(/L'ordre a changé/)).toBeInTheDocument()
  })
})

function cleanupAndRender(props) {
  document.body.innerHTML = ''
  render(<LightGroupsManager {...props} />)
}
