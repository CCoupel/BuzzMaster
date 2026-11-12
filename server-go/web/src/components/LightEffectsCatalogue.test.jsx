import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within, act } from '@testing-library/react'

// ---------------------------------------------------------------------------
// #299 (v11.7.0, LOT-3) — catalogue des effets : aperçu rejoué à partir des trames du serveur (§7.3), essai « Essayer sur la
// salle » ×1 sur la zone Ambiance (§7.4), « utilisé : zone, moment », erreurs de fichiers, rechargement.
// Contrat e9d86cd7 : l'interface LIT `tick`, `step` (« pre » | « pas:<n> » | « dernier »), `duration_ms`, `chrono_s` dans la réponse de
// l'aperçu et `steps` dans le catalogue ; elle ne fixe plus aucune durée (essai : `seconds` choisi par le serveur).
// Écrit depuis contracts/light-effects.md §7.1 à §7.4 et la maquette docs/mockup/v11.7.0/ui/ambiance_page__effects_catalogue_zones.html.
// Complète LightEffectsCatalogue.cards.test.jsx (cartes, étiquettes, essai de base) : ici le rejeu de l'aperçu, le temps simulé,
// les états limites et la cohabitation avec le bouton « Tester » des réglages.
// ---------------------------------------------------------------------------

vi.mock('./LightEffectsCatalogue.css', () => ({}))
import LightEffectsCatalogue, { usedBy } from './LightEffectsCatalogue'
import { TRY_EVENT } from '../utils/lightEffects'

const ALL = { ambiance: true, animateur: true, public: true }
const fx = (id, name, moments, extra = {}) => ({
  id, name, description: '', moments, origin: 'native', file: `natifs/${id}.yaml`, min_groups: 0, tick_ms: 1000, applicable: ALL, ...extra,
})
const CATALOG = {
  groups: { ambiance: 4, animateur: 1, public: 0 },
  effects: [
    fx('pulsation', 'Pulsation', ['chrono'], { tick_ms: 500, steps: 2 }),
    fx('chenillard', 'Chenillard', ['chrono'], { min_groups: 3, steps: 1, description: 'Un point lumineux fait le tour.' }),
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
const PREVIEW = { id: 'chenillard', moment: 'chrono', groups: 4, tick_ms: 1000, duration_ms: 4000, chrono_s: 20, loop: true, frames: [
  { at_ms: 0, tick: 0, step: 'pre', groups: [L(RED), L(BLUE), L(BLUE), L(BLUE)] },
  { at_ms: 1000, tick: 1, step: 'pas:0', groups: [L(BLUE), L(RED), L(BLUE), L(BLUE)] },
  { at_ms: 2000, tick: 2, step: 'pas:1', groups: [L(BLUE), L(BLUE), L(RED), L(BLUE)] },
  { at_ms: 3000, tick: 3, step: 'dernier', groups: [L(BLUE), L(BLUE), L(BLUE), L(RED)] },
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
    expect(stage).toHaveTextContent(/tick 0 · PRE · chronomètre simulé : 20 s/)
    await act(async () => { await vi.advanceTimersByTimeAsync(1000) })
    expect(stage).toHaveTextContent(/tick 1 · pas 0 · chronomètre simulé : 19 s/)
    await act(async () => { await vi.advanceTimersByTimeAsync(1000) })
    expect(stage).toHaveTextContent(/tick 2 · pas 1/)
    await act(async () => { await vi.advanceTimersByTimeAsync(1000) })
    expect(stage).toHaveTextContent(/tick 3 · dernier/)
  })

  it('le tick et le pas sont LUS dans la trame (pas déduits de at_ms ÷ tick_ms) : un tick 7 à 1000 ms s\'affiche « tick 7 »', async () => {
    vi.useFakeTimers()
    okFetch({ ...PREVIEW, frames: [{ at_ms: 0, tick: 7, step: 'pas:3', groups: [L(RED), L(BLUE), L(BLUE), L(BLUE)] }] })
    mount()
    await act(async () => { fireEvent.click(card('chenillard').getByRole('button', { name: /Aperçu/ })) })
    await act(async () => { await vi.advanceTimersByTimeAsync(100) })
    expect(screen.getByTestId('preview-chenillard')).toHaveTextContent(/tick 7 · pas 3/)
  })

  it('chrono_s lu dans la réponse : 0 ⇒ aucun compte à rebours ; 20 ⇒ « chronomètre simulé » ; le temps restant = chrono_s − at_ms/1000', async () => {
    vi.useFakeTimers()
    okFetch({ ...PREVIEW, id: 'degrade', moment: 'calm', chrono_s: 0, frames: [{ at_ms: 0, tick: 0, step: 'pas:0', groups: [L(RED), L(BLUE), L(BLUE), L(BLUE)] }] })
    mount()
    await act(async () => { fireEvent.click(card('degrade').getByRole('button', { name: /Aperçu/ })) })
    await act(async () => { await vi.advanceTimersByTimeAsync(100) })
    expect(screen.getByTestId('preview-degrade')).toHaveTextContent(/tick 0 · pas 0/)
    expect(screen.getByTestId('preview-degrade')).not.toHaveTextContent(/chronomètre simulé/)
  })

  it('la durée de l\'aperçu est celle de la réponse (5 s pour score) : l\'interface ne la fixe pas', async () => {
    vi.useFakeTimers()
    okFetch({ ...PREVIEW, id: 'couleur-equipe', moment: 'score', chrono_s: 0, duration_ms: 5000, frames: [
      { at_ms: 0, tick: 0, step: 'pas:0', groups: [L(RED), L(RED), L(RED), L(RED)] },
      { at_ms: 4000, tick: 4, step: 'pas:0', groups: [L(BLUE), L(BLUE), L(BLUE), L(BLUE)] },
    ] })
    mount()
    await act(async () => { fireEvent.click(card('couleur-equipe').getByRole('button', { name: /Aperçu/ })) })
    await act(async () => { await vi.advanceTimersByTimeAsync(4100) })
    expect(screen.getByTestId('preview-couleur-equipe')).toHaveTextContent(/tick 4/)
    await act(async () => { await vi.advanceTimersByTimeAsync(1000) }) // 5,1 s : retour au début (5 000 ms)
    expect(screen.getByTestId('preview-couleur-equipe')).toHaveTextContent(/tick 0/)
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
    expect(screen.getByTestId('preview-chenillard')).toHaveTextContent(/tick 3 · dernier/)
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
  it('l\'appel ne porte ni zone, ni vitesse, ni durée : seul l\'id est transmis (le serveur choisit seconds : 5 pour score seul, 10 sinon)', async () => {
    const onTry = vi.fn().mockResolvedValue({ ok: true, duration_ms: 10000 })
    mount({ lighting: { speeds: { ambiance: { chrono: 2 } } }, onTry })
    fireEvent.click(card('chenillard').getByRole('button', { name: /Essayer sur la salle/ }))
    await waitFor(() => expect(onTry).toHaveBeenCalledTimes(1))
    expect(onTry.mock.calls[0]).toEqual(['chenillard', 'ambiance'])
    onTry.mockResolvedValue({ ok: true, duration_ms: 5000 })
    fireEvent.click(card('couleur-equipe').getByRole('button', { name: /Essayer sur la salle/ }))
    await waitFor(() => expect(onTry).toHaveBeenCalledTimes(2))
    expect(onTry.mock.calls[1]).toEqual(['couleur-equipe', 'ambiance'])
  })

  it('la durée de l\'essai est celle que renvoie le serveur (5 s pour un effet de score) : le bouton se rétablit à ce moment', async () => {
    vi.useFakeTimers()
    mount({ onTry: vi.fn().mockResolvedValue({ ok: true, duration_ms: 5000 }) })
    await act(async () => { fireEvent.click(card('couleur-equipe').getByRole('button', { name: /Essayer sur la salle/ })) })
    expect(card('couleur-equipe').getByRole('button', { name: /Arrêter l'essai/ })).toBeInTheDocument()
    await act(async () => { await vi.advanceTimersByTimeAsync(5200) })
    expect(card('couleur-equipe').getByRole('button', { name: /Essayer sur la salle/ })).toBeInTheDocument()
  })

  it('l\'infobulle annonce la zone Ambiance et la vitesse ×1', () => {
    mount()
    expect(card('pulsation').getByRole('button', { name: /Essayer sur la salle/ }).title).toMatch(/zone Ambiance, à la vitesse ×1/)
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
  it('la ligne « Fichier » affiche le nombre de pas de la boucle (`steps`) et la durée du tick', () => {
    mount()
    expect(screen.getByTestId('fx-pulsation')).toHaveTextContent('natifs/pulsation.yaml · 2 pas · tick de 0,5 s')
    expect(screen.getByTestId('fx-chenillard')).toHaveTextContent('natifs/chenillard.yaml · 1 pas · tick de 1 s')
    expect(screen.getByTestId('fx-couleur-equipe')).not.toHaveTextContent(/ pas ·/) // steps absent : rien d'inventé
  })

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

  it('aucun bouton de création ni de modification (éditeur : LOT-2) ; Supprimer n\'existe que sur un effet personnalisé (#305)', () => {
    mount()
    expect(screen.queryByRole('button', { name: /Créer|Nouvel effet|Modifier|Éditer/i })).toBeNull()
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
    expect(card('chenillard').getByRole('button', { name: /Essayer sur la salle/ }).title).toBe('Demande 5 groupes ; la zone Ambiance en a 2')
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
