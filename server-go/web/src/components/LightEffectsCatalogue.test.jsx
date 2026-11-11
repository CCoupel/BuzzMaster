import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within, act } from '@testing-library/react'

// ---------------------------------------------------------------------------
// #299 (v11.7.0, LOT-3) — catalogue des effets : aperçu rejoué à partir des trames du serveur (§7.3), essai « Essayer sur la
// salle » ×1 sur la zone Ambiance (§7.4), « utilisé : zone, moment », erreurs de fichiers, rechargement.
// Écrit depuis contracts/light-effects.md §7.1 à §7.4 et la maquette docs/mockup/v11.7.0/ui/ambiance_page__effects_catalogue_zones.html.
// Complète LightEffectsCatalogue.cards.test.jsx (cartes, étiquettes, essai de base) : ici le rejeu de l'aperçu, le temps simulé,
// les états limites et la cohabitation avec le bouton « Tester » des réglages.
// ---------------------------------------------------------------------------

vi.mock('./LightEffectsCatalogue.css', () => ({}))
import LightEffectsCatalogue, { usedBy, TRY_EVENT } from './LightEffectsCatalogue'

const ALL = { ambiance: true, animateur: true, public: true }
const fx = (id, name, moments, extra = {}) => ({
  id, name, description: '', moments, origin: 'native', file: `natifs/${id}.yaml`, min_groups: 0, tick_ms: 1000, applicable: ALL, ...extra,
})
const CATALOG = {
  groups: { ambiance: 4, animateur: 1, public: 0 },
  effects: [
    fx('pulsation', 'Pulsation', ['chrono'], { tick_ms: 500 }),
    fx('chenillard', 'Chenillard', ['chrono'], { min_groups: 3, description: 'Un point lumineux fait le tour.' }),
    fx('couleur-equipe', "Couleur de l'équipe", ['score']),
    fx('degrade', 'Dégradé', ['calm'], { min_groups: 2, tick_ms: 60000 }),
    fx('mon-effet', 'Mon effet', ['calm', 'chrono'], { origin: 'user', file: 'mon-effet.yaml' }),
  ],
  errors: [],
  dir: 'data/files/effects',
}
const GROUPS = { ambiance: 4, animateur: 1, public: 0 }
const L = (c, i = 255) => ({ color: c, intensity: i, fade_ms: 0 })
const RED = [255, 0, 0], BLUE = [0, 0, 255]
const PREVIEW = { id: 'chenillard', groups: 4, duration_ms: 4000, loop: true, frames: [
  { at_ms: 0, groups: [L(RED), L(BLUE), L(BLUE), L(BLUE)] },
  { at_ms: 1000, groups: [L(BLUE), L(RED), L(BLUE), L(BLUE)] },
  { at_ms: 2000, groups: [L(BLUE), L(BLUE), L(RED), L(BLUE)] },
] }

const mount = (props = {}) => render(
  <LightEffectsCatalogue
    catalog={CATALOG} lighting={{}} zoneGroups={GROUPS}
    onReload={vi.fn().mockResolvedValue(true)} onTry={vi.fn().mockResolvedValue({ ok: true, duration_ms: 10000 })} onStopTry={vi.fn()}
    {...props}
  />
)
const card = (id) => within(screen.getByTestId(`fx-${id}`))
const okFetch = (data) => { global.fetch = vi.fn(async () => ({ ok: true, status: 200, json: async () => data })) }
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks() })

describe('LightEffectsCatalogue — aperçu rejoué (§7.3)', () => {
  it('une ampoule dessinée par groupe, étiquetée g1, g2… (groupes de la zone Ambiance)', async () => {
    okFetch(PREVIEW)
    mount()
    fireEvent.click(card('chenillard').getByRole('button', { name: /Aperçu/ }))
    await waitFor(() => expect(global.fetch).toHaveBeenCalled())
    const stage = within(screen.getByTestId('preview-chenillard'))
    for (const n of [1, 2, 3, 4]) expect(stage.getByText(`g${n}`)).toBeInTheDocument()
    expect(stage.queryByText('g5')).toBeNull()
  })

  it('sans groupe dans la zone Ambiance : 6 ampoules par défaut, et l\'aperçu est demandé avec groups=6', async () => {
    okFetch({ ...PREVIEW, groups: 6 })
    mount({ zoneGroups: { ambiance: 0, animateur: 0, public: 0 }, catalog: { ...CATALOG, groups: { ambiance: 0, animateur: 0, public: 0 } } })
    fireEvent.click(card('pulsation').getByRole('button', { name: /Aperçu/ }))
    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/api/lighting/effects/pulsation/preview?groups=6&moment=chrono'))
  })

  it('le moment demandé est le chronomètre si l\'effet le déclare, sinon son premier moment', async () => {
    okFetch(PREVIEW)
    mount()
    fireEvent.click(card('degrade').getByRole('button', { name: /Aperçu/ }))
    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/api/lighting/effects/degrade/preview?groups=4&moment=calm'))
    fireEvent.click(card('mon-effet').getByRole('button', { name: /Aperçu/ }))
    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/api/lighting/effects/mon-effet/preview?groups=4&moment=chrono'))
  })

  it('rejoue les trames du serveur dans le temps : tick et chronomètre simulé de 20 s (temps simulé)', async () => {
    vi.useFakeTimers()
    okFetch(PREVIEW)
    mount()
    await act(async () => { fireEvent.click(card('chenillard').getByRole('button', { name: /Aperçu/ })) })
    await act(async () => { await vi.advanceTimersByTimeAsync(100) })
    const stage = screen.getByTestId('preview-chenillard')
    expect(stage).toHaveTextContent(/tick 0 · chronomètre simulé : 20 s/)
    await act(async () => { await vi.advanceTimersByTimeAsync(1000) })
    expect(stage).toHaveTextContent(/tick 1 · chronomètre simulé : 19 s/)
    await act(async () => { await vi.advanceTimersByTimeAsync(1000) })
    expect(stage).toHaveTextContent(/tick 2/)
  })

  it('l\'aperçu reboucle sur duration_ms quand l\'effet boucle', async () => {
    vi.useFakeTimers()
    okFetch(PREVIEW)
    mount()
    await act(async () => { fireEvent.click(card('chenillard').getByRole('button', { name: /Aperçu/ })) })
    await act(async () => { await vi.advanceTimersByTimeAsync(4100) })
    expect(screen.getByTestId('preview-chenillard')).toHaveTextContent(/tick 0/)
  })

  it('boucle: non (loop=false) : l\'aperçu garde la dernière trame', async () => {
    vi.useFakeTimers()
    okFetch({ ...PREVIEW, loop: false })
    mount()
    await act(async () => { fireEvent.click(card('chenillard').getByRole('button', { name: /Aperçu/ })) })
    await act(async () => { await vi.advanceTimersByTimeAsync(9000) })
    expect(screen.getByTestId('preview-chenillard')).toHaveTextContent(/tick 2|tick 3|tick 4/)
    expect(screen.getByTestId('preview-chenillard')).not.toHaveTextContent(/tick 0/)
  })

  it('un effet qui ne s\'applique pas à la zone reste prévisualisable : « simulation avec 6 groupes »', async () => {
    okFetch({ ...PREVIEW, groups: 6 })
    mount({ catalog: { ...CATALOG, groups: { ambiance: 2, animateur: 0, public: 0 }, effects: CATALOG.effects.map(e => (e.id === 'chenillard' ? { ...e, min_groups: 5, applicable: { ambiance: false, animateur: false, public: false } } : e)) }, zoneGroups: { ambiance: 2, animateur: 0, public: 0 } })
    fireEvent.click(card('chenillard').getByRole('button', { name: /Aperçu/ }))
    await waitFor(() => expect(global.fetch).toHaveBeenCalledWith('/api/lighting/effects/chenillard/preview?groups=6&moment=chrono'))
    expect(await card('chenillard').findByText('simulation avec 6 groupes')).toBeInTheDocument()
  })

  it('« ■ Arrêter » arrête l\'aperçu et rend le bouton « ▶ Aperçu »', async () => {
    okFetch(PREVIEW)
    mount()
    fireEvent.click(card('chenillard').getByRole('button', { name: /▶ Aperçu/ }))
    fireEvent.click(await card('chenillard').findByRole('button', { name: /■ Arrêter$/ }))
    expect(card('chenillard').getByRole('button', { name: /▶ Aperçu/ })).toBeInTheDocument()
  })

  it('un seul aperçu à la fois : en lancer un second arrête le premier', async () => {
    okFetch(PREVIEW)
    mount()
    fireEvent.click(card('chenillard').getByRole('button', { name: /▶ Aperçu/ }))
    await card('chenillard').findByRole('button', { name: /■ Arrêter$/ })
    fireEvent.click(card('pulsation').getByRole('button', { name: /▶ Aperçu/ }))
    await card('pulsation').findByRole('button', { name: /■ Arrêter$/ })
    expect(card('chenillard').getByRole('button', { name: /▶ Aperçu/ })).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: /■ Arrêter$/ })).toHaveLength(1)
  })

  it('aperçu indisponible (404 / 400 / réseau) : le bouton est rendu, aucune exception', async () => {
    for (const impl of [async () => ({ ok: false, status: 404, json: async () => ({}) }), async () => { throw new Error('réseau') }]) {
      global.fetch = vi.fn(impl)
      const { unmount } = mount()
      fireEvent.click(card('pulsation').getByRole('button', { name: /▶ Aperçu/ }))
      await waitFor(() => expect(card('pulsation').getByRole('button', { name: /▶ Aperçu/ })).toBeInTheDocument())
      unmount()
    }
  })

  it('l\'aperçu est à la vitesse d\'origine : aucun paramètre de vitesse dans la requête', async () => {
    okFetch(PREVIEW)
    mount({ lighting: { speeds: { ambiance: { chrono: 2 } } } })
    fireEvent.click(card('chenillard').getByRole('button', { name: /▶ Aperçu/ }))
    await waitFor(() => expect(global.fetch).toHaveBeenCalled())
    expect(global.fetch.mock.calls[0][0]).not.toMatch(/speed/)
  })
})

describe('LightEffectsCatalogue — « Essayer sur la salle » ×1 sur la zone Ambiance (§7.4)', () => {
  it('l\'appel ne porte ni zone ni vitesse : seuls l\'id (et une durée différente de 10 s) sont transmis', async () => {
    const onTry = vi.fn().mockResolvedValue({ ok: true, duration_ms: 10000 })
    mount({ lighting: { speeds: { ambiance: { chrono: 2 } } }, onTry })
    fireEvent.click(card('chenillard').getByRole('button', { name: /Essayer sur la salle \(10 s\)/ }))
    await waitFor(() => expect(onTry).toHaveBeenCalledTimes(1))
    expect(onTry.mock.calls[0]).toEqual(['chenillard', undefined])
  })

  it('l\'infobulle annonce la zone Ambiance et la vitesse ×1', () => {
    mount()
    expect(card('pulsation').getByRole('button', { name: /Essayer sur la salle/ }).title).toMatch(/zone Ambiance, à la vitesse ×1, pendant 10 s/)
  })

  it('un essai refusé (ok:false) ne laisse pas le bouton en « Arrêter l\'essai »', async () => {
    const onTry = vi.fn().mockResolvedValue({ ok: false })
    mount({ onTry })
    fireEvent.click(card('pulsation').getByRole('button', { name: /Essayer sur la salle/ }))
    await waitFor(() => expect(onTry).toHaveBeenCalled())
    expect(card('pulsation').queryByRole('button', { name: /Arrêter l'essai/ })).toBeNull()
  })

  it('un essai se termine de lui-même après sa durée (temps simulé)', async () => {
    vi.useFakeTimers()
    mount({ onTry: vi.fn().mockResolvedValue({ ok: true, duration_ms: 10000 }) })
    await act(async () => { fireEvent.click(card('pulsation').getByRole('button', { name: /Essayer sur la salle/ })) })
    expect(card('pulsation').getByRole('button', { name: /Arrêter l'essai/ })).toBeInTheDocument()
    await act(async () => { await vi.advanceTimersByTimeAsync(10500) })
    expect(card('pulsation').getByRole('button', { name: /Essayer sur la salle/ })).toBeInTheDocument()
  })

  it('un seul essai à la fois : lancer l\'essai d\'un autre effet remplace le premier (la durée porte celle du serveur)', async () => {
    const onTry = vi.fn().mockResolvedValue({ ok: true, duration_ms: 10000 })
    mount({ onTry })
    fireEvent.click(card('pulsation').getByRole('button', { name: /Essayer sur la salle/ }))
    await card('pulsation').findByRole('button', { name: /Arrêter l'essai/ })
    fireEvent.click(card('chenillard').getByRole('button', { name: /Essayer sur la salle/ }))
    await card('chenillard').findByRole('button', { name: /Arrêter l'essai/ })
    expect(card('pulsation').getByRole('button', { name: /Essayer sur la salle/ })).toBeInTheDocument() // le premier essai n'est plus affiché
    expect(onTry).toHaveBeenCalledTimes(2)
  })

  it('raisons de grisage : éclairage désactivé, pont absent / injoignable, forçage ON/OFF/Flash, question en cours', () => {
    for (const reason of [
      "L'éclairage d'ambiance est désactivé.", 'Aucun pont associé', 'Pont injoignable',
      "L'éclairage est forcé sur ON ou OFF, ou le Flash est actif : repassez en AUTO.", "Une question est en cours : l'essai est possible hors question.",
    ]) {
      const { unmount } = mount({ tryUnavailable: () => reason })
      const btn = card('pulsation').getByRole('button', { name: /Essayer sur la salle/ })
      expect(btn).toBeDisabled()
      expect(btn.title).toBe(reason)
      unmount()
    }
  })

  it('l\'aperçu reste disponible quand l\'essai est grisé', () => {
    mount({ tryUnavailable: () => 'Pont injoignable' })
    expect(card('pulsation').getByRole('button', { name: /▶ Aperçu/ })).toBeEnabled()
  })

  it('le bouton « Tester » des réglages qui démarre remplace l\'essai du catalogue (événement), mais pas l\'inverse de la même source', async () => {
    mount()
    fireEvent.click(card('pulsation').getByRole('button', { name: /Essayer sur la salle/ }))
    await card('pulsation').findByRole('button', { name: /Arrêter l'essai/ })
    act(() => { window.dispatchEvent(new CustomEvent(TRY_EVENT, { detail: { source: 'catalogue' } })) })
    expect(card('pulsation').getByRole('button', { name: /Arrêter l'essai/ })).toBeInTheDocument()
    act(() => { window.dispatchEvent(new CustomEvent(TRY_EVENT, { detail: { source: 'panel' } })) })
    await waitFor(() => expect(card('pulsation').queryByRole('button', { name: /Arrêter l'essai/ })).toBeNull())
  })
})

describe('LightEffectsCatalogue — « utilisé : zone, moment » et états du catalogue', () => {
  const LIGHTING = { effects: { ambiance: { chrono: 'pulsation' }, animateur: { chrono: 'pulsation', score: 'couleur-equipe' }, public: { score: 'couleur-equipe' } }, speeds: { animateur: { chrono: 1.5 }, ambiance: { chrono: 1 } } }

  it('cite la zone, le moment et la vitesse seulement si elle n\'est pas ×1', () => {
    expect(usedBy(LIGHTING, GROUPS, 'pulsation')).toEqual(['Ambiance, chronomètre', 'Animateur, chronomètre (×1,5)'])
  })

  it('une zone sans groupe non vide n\'est pas citée (elle n\'existe pas)', () => {
    expect(usedBy(LIGHTING, GROUPS, 'couleur-equipe')).toEqual(['Animateur, équipe qui marque'])
  })

  it('un effet réglé nulle part : aucune mention « utilisé »', () => {
    mount({ lighting: LIGHTING })
    expect(card('chenillard').queryByText(/utilisé/)).toBeNull()
  })

  it('le défaut global (pulsation au chronomètre de l\'ambiance) est cité sans configuration', () => {
    mount({ lighting: {} })
    expect(card('pulsation').getByText(/utilisé : Ambiance, chronomètre/)).toBeInTheDocument()
  })

  it('les surcharges de manche ne sont pas listées : seules les valeurs globales comptent', () => {
    expect(usedBy({ effects: { ambiance: { chrono: 'chenillard' } } }, GROUPS, 'pulsation')).toEqual([])
  })

  it('aucun bouton de création, de modification ni de suppression (éditeur prévu en v11.8.0)', () => {
    mount()
    expect(screen.queryByRole('button', { name: /Créer|Nouvel effet|Modifier|Supprimer|Éditer/i })).toBeNull()
  })

  it('un seul fichier en erreur : message au singulier ; sans erreur, pas de bandeau', () => {
    const { rerender } = mount({ catalog: { ...CATALOG, errors: [{ file: 'a.yaml', line: 3, message: 'clé inconnue' }] } })
    expect(screen.getByRole('alert')).toHaveTextContent("1 fichier n'a pas pu être chargé.")
    expect(screen.getByRole('alert')).toHaveTextContent('a.yaml, ligne 3 : clé inconnue')
    rerender(<LightEffectsCatalogue catalog={CATALOG} lighting={{}} zoneGroups={GROUPS} onTry={vi.fn()} />)
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('un effet inapplicable propose « Gérer les groupes » et son essai est grisé « Demande n groupes »', () => {
    const onManageGroups = vi.fn()
    mount({ onManageGroups, catalog: { ...CATALOG, groups: { ambiance: 2, animateur: 0, public: 0 }, effects: CATALOG.effects.map(e => (e.id === 'chenillard' ? { ...e, min_groups: 5, applicable: { ambiance: false, animateur: false, public: false } } : e)) }, zoneGroups: { ambiance: 2, animateur: 0, public: 0 } })
    fireEvent.click(card('chenillard').getByRole('button', { name: 'Gérer les groupes' }))
    expect(onManageGroups).toHaveBeenCalled()
    expect(card('chenillard').getByRole('button', { name: /Essayer sur la salle/ }).title).toBe('Demande 5 groupes')
  })

  it('« Recharger les fichiers » : un seul appel par clic, bouton occupé pendant l\'appel', async () => {
    let done
    const onReload = vi.fn(() => new Promise(r => { done = r }))
    mount({ onReload })
    fireEvent.click(screen.getByRole('button', { name: /Recharger les fichiers/ }))
    expect(onReload).toHaveBeenCalledTimes(1)
    await act(async () => { done(true) })
  })

  it('le lien « Voir l\'aperçu » d\'un sélecteur met la carte en vue (focus)', () => {
    const scroll = vi.fn()
    const { rerender } = mount()
    document.getElementById('fx-card-chenillard').scrollIntoView = scroll
    rerender(<LightEffectsCatalogue catalog={CATALOG} lighting={{}} zoneGroups={GROUPS} onTry={vi.fn()} focus={{ id: 'chenillard', n: 1 }} />)
    expect(scroll).toHaveBeenCalled()
  })
})
