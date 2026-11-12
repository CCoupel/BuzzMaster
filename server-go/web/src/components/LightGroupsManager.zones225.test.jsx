import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'

vi.mock('./LightGroupsManager.css', () => ({}))
import LightGroupsManager from './LightGroupsManager'

const CFG = [
  { id: 'g1', role: 'salle', lights: ['A'] },
  { id: 'h1', role: 'animateur', lights: ['B'] },
  { id: 'p1', role: 'public', lights: ['C'] },
]
const base = (props = {}) => ({
  generalLights: ['A', 'B', 'C', 'D'], groups: CFG, enabled: true,
  onSave: vi.fn().mockResolvedValue({ ok: true }),
  onTest: vi.fn().mockResolvedValue({ ok: true, duration_ms: 6000 }),
  onStopTest: vi.fn().mockResolvedValue({ ok: true }),
  ...props,
})
const zone = (z) => within(screen.getByTestId(`lgm-zone-${z}`))

describe('LightGroupsManager — trois zones (#225, révision 4)', () => {
  it('trois zones avec leur ordre de parcours ; pas de nom, pas de « Équipes à proximité », pas de « un groupe par ampoule »', () => {
    render(<LightGroupsManager {...base()} />)
    expect(screen.getByTestId('lgm-zone-ambiance')).toHaveTextContent('Zone Ambiance — 1 groupe')
    expect(screen.getByTestId('lgm-zone-animateur')).toHaveTextContent('Zone Animateur — 1 groupe')
    expect(screen.getByTestId('lgm-zone-public')).toHaveTextContent('Zone Public — 1 groupe')
    expect(screen.getByTestId('lgm-zone-public')).toHaveTextContent('sans effet : non pilotée')
    expect(screen.queryByRole('textbox')).toBeNull()
    expect(screen.queryByText(/Équipes à proximité|Créer un groupe par ampoule/)).toBeNull()
  })

  it('une zone sans groupe garde son bouton de création ; créer un groupe public l\'ajoute sans id à la charge utile', async () => {
    const props = base({ groups: [] })
    render(<LightGroupsManager {...props} />)
    fireEvent.click(screen.getByRole('button', { name: '👥 + Nouveau groupe public' }))
    fireEvent.click(screen.getByRole('button', { name: /Enregistrer les groupes/ }))
    await waitFor(() => expect(props.onSave).toHaveBeenCalledWith([{ role: 'public', lights: [] }]))
  })

  it('12 groupes au plus par zone : la création est désactivée dans la zone pleine seulement', () => {
    const full = Array.from({ length: 12 }, (_, i) => ({ id: `p${i}`, role: 'public', lights: [] }))
    render(<LightGroupsManager {...base({ groups: full })} />)
    expect(screen.getByRole('button', { name: '👥 + Nouveau groupe public' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '🎤 + Nouveau groupe animateur' })).not.toBeDisabled()
  })

  it('« Déplacer vers… » range une ampoule dans un groupe d\'une autre zone', async () => {
    const props = base()
    render(<LightGroupsManager {...props} />)
    fireEvent.change(screen.getByLabelText('Déplacer D vers…'), { target: { value: 'public:0' } })
    fireEvent.click(screen.getByRole('button', { name: /Enregistrer les groupes/ }))
    await waitFor(() => expect(props.onSave).toHaveBeenCalled())
    expect(props.onSave.mock.calls[0][0].at(-1)).toEqual({ id: 'p1', role: 'public', lights: ['C', 'D'] })
  })

  it('essai : poste n / host<k> / public<k>, « ■ Arrêter l\'essai » pendant la séquence, puis stop', async () => {
    const props = base()
    render(<LightGroupsManager {...props} />)
    fireEvent.click(zone('public').getByRole('button', { name: 'Tester le groupe' }))
    await waitFor(() => expect(props.onTest).toHaveBeenCalledWith('public1'))
    const stop = await screen.findByRole('button', { name: /Arrêter l'essai/ })
    expect(stop.title).toMatch(/Étape 1\/6/)
    fireEvent.click(stop)
    await waitFor(() => expect(props.onStopTest).toHaveBeenCalled())
    fireEvent.click(zone('animateur').getByRole('button', { name: 'Tester le groupe' }))
    await waitFor(() => expect(props.onTest).toHaveBeenLastCalledWith('host1'))
  })

  it('infobulles de désactivation : groupe vide, éclairage désactivé, pont injoignable, enregistrer d\'abord', () => {
    const { rerender } = render(<LightGroupsManager {...base({ groups: [{ id: 'g1', role: 'salle', lights: [] }] })} />)
    expect(screen.getByRole('button', { name: 'Tester le groupe' })).toHaveAttribute('title', 'Groupe vide')
    rerender(<LightGroupsManager {...base({ enabled: false })} />)
    expect(screen.getAllByRole('button', { name: 'Tester le groupe' })[0]).toHaveAttribute('title', "Éclairage d'ambiance désactivé")
    rerender(<LightGroupsManager {...base({ testUnavailableReason: 'Pont injoignable' })} />)
    expect(screen.getAllByRole('button', { name: 'Tester le groupe' })[0]).toHaveAttribute('title', 'Pont injoignable')
    fireEvent.click(screen.getByRole('button', { name: '+ Nouveau groupe' }))
    expect(screen.getAllByRole('button', { name: 'Tester le groupe' })[0]).toHaveAttribute('title', "Enregistrez d'abord")
  })

  it('« Choisir les effets de la zone » (animateur et public seulement) transmet la zone', () => {
    const onChooseEffects = vi.fn()
    render(<LightGroupsManager {...base({ onChooseEffects })} />)
    expect(zone('ambiance').queryByText('Choisir les effets de la zone')).toBeNull()
    fireEvent.click(zone('public').getByRole('button', { name: 'Choisir les effets de la zone' }))
    expect(onChooseEffects).toHaveBeenCalledWith('public')
  })

  it('changer l\'ordre des groupes affiche le message sur g1, g2…', () => {
    const two = [{ id: 'g1', role: 'salle', lights: ['A'] }, { id: 'g2', role: 'salle', lights: ['B'] }]
    render(<LightGroupsManager {...base({ groups: two })} />)
    expect(screen.queryByText(/L'ordre a changé/)).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Descendre le groupe 1 de la zone Ambiance' }))
    expect(screen.getByText(/L'ordre a changé/)).toBeInTheDocument()
  })
})
