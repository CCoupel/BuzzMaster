import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within, act } from '@testing-library/react'

vi.mock('./LightEffectsCatalogue.css', () => ({}))
import LightEffectsCatalogue, { usedBy } from './LightEffectsCatalogue'
import { frameAt, paintStyle } from './LightEffectPreview'

const CATALOG = {
  groups: { ambiance: 3, animateur: 2, public: 0 },
  effects: [
    { id: 'pulsation', name: 'Pulsation', description: 'Toute la zone respire.', moments: ['chrono'], origin: 'native', file: 'natifs/pulsation.yaml', min_groups: 0, tick_ms: 500, applicable: { ambiance: true, animateur: true, public: false } },
    { id: 'chenillard-traine', name: 'Chenillard avec traînée', description: '', moments: ['chrono'], origin: 'native', file: 'natifs/chenillard-traine.yaml', min_groups: 5, tick_ms: 1000, applicable: { ambiance: false, animateur: false, public: false } },
    { id: 'couleur-equipe', name: "Couleur de l'équipe", description: '', moments: ['score'], origin: 'native', file: 'natifs/couleur-equipe.yaml', min_groups: 0, tick_ms: 1000, applicable: { ambiance: true, animateur: true, public: false } },
    { id: 'degrade', name: 'Dégradé', description: '', moments: ['calm'], origin: 'override', file: 'degrade.yaml', min_groups: 2, tick_ms: 1000, applicable: { ambiance: true, animateur: true, public: false } },
    { id: 'aller-retour', name: 'Aller-retour', description: '', moments: ['chrono', 'calm'], origin: 'user', file: 'aller-retour.yaml', min_groups: 0, tick_ms: 500, applicable: { ambiance: true, animateur: true, public: false } },
  ],
  errors: [{ file: 'mon-chenillard.yaml', line: 12, message: "la variable « gB » n'est pas déclarée dans variables" }, { file: 'ancien-effet.yaml', message: 'format à réécrire en pas à pas' }],
  dir: 'data/files/effects',
}
const LIGHTING = { effects: { animateur: { chrono: 'pulsation', score: 'couleur-equipe' }, public: { score: 'couleur-equipe' } }, speeds: { animateur: { chrono: 1.5 } } }
const GROUPS = { ambiance: 3, animateur: 2, public: 0 }

const mount = (props = {}) => render(
  <LightEffectsCatalogue
    catalog={CATALOG} lighting={LIGHTING} zoneGroups={GROUPS}
    onReload={vi.fn().mockResolvedValue(true)} onTry={vi.fn().mockResolvedValue({ ok: true, duration_ms: 10000 })} onStopTry={vi.fn()}
    {...props}
  />
)
const card = (id) => within(screen.getByTestId(`fx-${id}`))
afterEach(() => { vi.restoreAllMocks() })

describe('LightEffectsCatalogue — cartes (#299)', () => {
  it('une carte par effet : étiquettes natif / personnalisé / natif modifié, moments, fichier, tick, groupes demandés', () => {
    mount()
    expect(card('pulsation').getByText('natif')).toBeInTheDocument()
    expect(card('aller-retour').getByText('personnalisé')).toBeInTheDocument()
    expect(card('aller-retour').getByText('Chronomètre')).toBeInTheDocument()
    expect(card('aller-retour').getByText('Phases calmes')).toBeInTheDocument()
    expect(card('degrade').getByText('natif modifié')).toBeInTheDocument()
    expect(screen.getByTestId('fx-degrade')).toHaveTextContent('remplace natifs/degrade.yaml')
    expect(screen.getByTestId('fx-pulsation')).toHaveTextContent('natifs/pulsation.yaml · tick de 0,5 s')
    expect(screen.getByTestId('fx-chenillard-traine')).toHaveTextContent('demande 5 groupes dans la zone')
  })

  it('« utilisé : zone, moment (×vitesse) » d\'après les réglages globaux, y compris le défaut (Ambiance, chronomètre)', () => {
    expect(usedBy(LIGHTING, GROUPS, 'pulsation')).toEqual(['Ambiance, chronomètre', 'Animateur, chronomètre (×1,5)'])
    expect(usedBy(LIGHTING, GROUPS, 'couleur-equipe')).toEqual(['Animateur, équipe qui marque']) // zone Public sans groupe : absente
    mount()
    expect(card('pulsation').getByText(/utilisé : Ambiance, chronomètre · Animateur, chronomètre \(×1,5\)/)).toBeInTheDocument()
  })

  it('bandeau des fichiers en erreur : nom, ligne, cause ; le reste du catalogue est affiché', () => {
    mount()
    const alert = screen.getByRole('alert')
    expect(alert).toHaveTextContent('2 fichiers n\'ont pas pu être chargés.')
    expect(alert).toHaveTextContent("mon-chenillard.yaml, ligne 12 : la variable « gB » n'est pas déclarée dans variables")
    expect(alert).toHaveTextContent('ancien-effet.yaml : format à réécrire en pas à pas')
    expect(screen.getAllByRole('article')).toHaveLength(5)
  })

  it('catalogue vide : « Aucun effet disponible »', () => {
    mount({ catalog: { groups: GROUPS, effects: [], errors: [], dir: '' } })
    expect(screen.getByText('Aucun effet disponible.')).toBeInTheDocument()
  })

  it('« Recharger les fichiers » appelle onReload', async () => {
    const onReload = vi.fn().mockResolvedValue(true)
    mount({ onReload })
    fireEvent.click(screen.getByRole('button', { name: /Recharger les fichiers/ }))
    await waitFor(() => expect(onReload).toHaveBeenCalled())
  })
})

describe('LightEffectsCatalogue — « Essayer sur la salle » (#299)', () => {
  it('essai sans zone ni vitesse (10 s par défaut) ; « ■ Arrêter l\'essai » appelle onStopTry', async () => {
    const props = { onTry: vi.fn().mockResolvedValue({ ok: true, duration_ms: 10000 }), onStopTry: vi.fn() }
    mount(props)
    fireEvent.click(card('pulsation').getByRole('button', { name: /Essayer sur la salle \(10 s\)/ }))
    await waitFor(() => expect(props.onTry).toHaveBeenCalledWith('pulsation', undefined))
    fireEvent.click(await card('pulsation').findByRole('button', { name: /Arrêter l'essai/ }))
    expect(props.onStopTry).toHaveBeenCalled()
  })

  it('effet de score seul : 5 s', async () => {
    const onTry = vi.fn().mockResolvedValue({ ok: true, duration_ms: 5000 })
    mount({ onTry })
    fireEvent.click(card('couleur-equipe').getByRole('button', { name: /Essayer sur la salle \(5 s\)/ }))
    await waitFor(() => expect(onTry).toHaveBeenCalledWith('couleur-equipe', 5))
  })

  it('grisé avec la raison : effet qui demande plus de groupes, éclairage / pont / forçage / question', () => {
    const { rerender } = mount()
    const btn = card('chenillard-traine').getByRole('button', { name: /Essayer sur la salle/ })
    expect(btn).toBeDisabled()
    expect(btn.title).toBe('Demande 5 groupes')
    expect(card('chenillard-traine').getByText(/Demande 5 groupes ; la zone Ambiance en a 3/)).toBeInTheDocument()
    rerender(<LightEffectsCatalogue catalog={CATALOG} lighting={LIGHTING} zoneGroups={GROUPS} onTry={vi.fn()} tryUnavailable={() => "L'éclairage d'ambiance est désactivé."} />)
    const b2 = card('pulsation').getByRole('button', { name: /Essayer sur la salle/ })
    expect(b2).toBeDisabled()
    expect(b2.title).toBe("L'éclairage d'ambiance est désactivé.")
  })

  it('un test des réglages qui démarre (événement) remplace l\'essai du catalogue', async () => {
    mount()
    fireEvent.click(card('pulsation').getByRole('button', { name: /Essayer sur la salle/ }))
    await card('pulsation').findByRole('button', { name: /Arrêter l'essai/ })
    act(() => { window.dispatchEvent(new CustomEvent('buzzcontrol:effect-try', { detail: { source: 'panel' } })) })
    await waitFor(() => expect(card('pulsation').queryByRole('button', { name: /Arrêter l'essai/ })).toBeNull())
  })
})

describe('LightEffectsCatalogue — aperçu simulé (#299)', () => {
  const PREVIEW = { id: 'pulsation', groups: 3, duration_ms: 2000, loop: true, frames: [
    { at_ms: 0, step: 'PRE', groups: [{ color: [79, 70, 229], intensity: 255, fade_ms: 0 }, { color: [79, 70, 229], intensity: 128, fade_ms: 0 }, { color: [0, 0, 0], intensity: 0, fade_ms: 0 }] },
    { at_ms: 1000, step: 'pas 1', groups: [{ color: [255, 0, 0], intensity: 255, fade_ms: 0 }, { color: [255, 0, 0], intensity: 255, fade_ms: 0 }, { color: [255, 0, 0], intensity: 255, fade_ms: 0 }] },
  ] }

  it('« Aperçu » demande les trames au serveur (groupes de la zone Ambiance), un seul aperçu à la fois', async () => {
    global.fetch = vi.fn(async () => ({ ok: true, status: 200, json: async () => PREVIEW }))
    mount()
    fireEvent.click(card('pulsation').getByRole('button', { name: /Aperçu/ }))
    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/api/lighting/effects/pulsation/preview?groups=3&moment=chrono'))
    expect(await card('pulsation').findByText(/tick 0 · PRE/)).toBeInTheDocument()
    expect(card('pulsation').getByRole('button', { name: /■ Arrêter/ })).toBeInTheDocument()
    fireEvent.click(card('aller-retour').getByRole('button', { name: /Aperçu/ }))
    await waitFor(() => expect(card('pulsation').getByRole('button', { name: /Aperçu/ })).toBeInTheDocument())
  })

  it('effet inapplicable : aperçu simulé avec 6 groupes, mention « simulation avec 6 groupes »', async () => {
    global.fetch = vi.fn(async () => ({ ok: true, status: 200, json: async () => ({ ...PREVIEW, id: 'chenillard-traine', groups: 6 }) }))
    mount()
    fireEvent.click(card('chenillard-traine').getByRole('button', { name: /Aperçu/ }))
    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/api/lighting/effects/chenillard-traine/preview?groups=6&moment=chrono'))
    expect(await card('chenillard-traine').findByText('simulation avec 6 groupes')).toBeInTheDocument()
  })

  it('aperçu indisponible (erreur serveur) : message, bouton rendu', async () => {
    global.fetch = vi.fn(async () => ({ ok: false, status: 404, json: async () => ({}) }))
    mount()
    fireEvent.click(card('pulsation').getByRole('button', { name: /Aperçu/ }))
    await waitFor(() => expect(card('pulsation').getByRole('button', { name: /▶ Aperçu/ })).toBeInTheDocument())
  })

  it('frameAt : dernière trame ≤ t, rebouclage sur duration_ms ; paintStyle assombrit selon l\'intensité', () => {
    expect(frameAt(PREVIEW, 0).frame.step).toBe('PRE')
    expect(frameAt(PREVIEW, 999).frame.step).toBe('PRE')
    expect(frameAt(PREVIEW, 1500).frame.step).toBe('pas 1')
    expect(frameAt(PREVIEW, 2100).frame.step).toBe('PRE') // 2100 mod 2000 = 100
    expect(frameAt({ ...PREVIEW, loop: false }, 9000).frame.step).toBe('pas 1')
    expect(frameAt({ frames: [] }, 0)).toBeNull()
    expect(paintStyle([255, 0, 0], 255).backgroundColor).toBe('rgb(255,0,0)')
    expect(paintStyle([255, 0, 0], 0).boxShadow).toBe('none')
  })
})
