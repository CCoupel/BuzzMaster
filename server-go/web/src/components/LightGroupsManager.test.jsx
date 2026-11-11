import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor, within, act } from '@testing-library/react'

// ---------------------------------------------------------------------------
// #225 (v11.7.0, LOT-1) — page « Groupes » de l'onglet Lumière, révision 3.
// Écrit depuis contracts/light-effects.md §2.1 (groupes SANS NOM, ni « équipes à proximité » : un groupe est son numéro),
// §2.4 (essai en séquence), §2.5 (groupes animateur, 4 au plus) et la maquette
// docs/mockup/v11.7.0/ui/ambiance_page__light_groups_numbered.html.
// Interface testée = props du composant (generalLights, groups, enabled, testUnavailableReason, onSave, onTest, onStopTest,
// onChooseEffects, onNotify) et ce que voit l'utilisateur.
// ---------------------------------------------------------------------------

vi.mock('./LightGroupsManager.css', () => ({}))

import LightGroupsManager from './LightGroupsManager'

const LIGHTS = ['BuzzHue1', 'BuzzHue2', 'BuzzHue3', 'BuzzHue4']

const salle = (lights = [], id = '') => ({ ...(id ? { id } : {}), role: 'salle', lights })
const host = (lights = [], id = '') => ({ ...(id ? { id } : {}), role: 'animateur', lights })

function setup(props = {}) {
  const onSave = vi.fn().mockResolvedValue({ ok: true })
  const onTest = vi.fn().mockResolvedValue({ ok: true })
  const onNotify = vi.fn()
  const utils = render(
    <LightGroupsManager generalLights={LIGHTS} groups={[]} enabled onSave={onSave} onTest={onTest} onNotify={onNotify} {...props} />
  )
  return { onSave, onTest, onNotify, ...utils }
}

const group = (n) => screen.getByTestId(`lgm-group-${n}`)
const hostBlock = (k) => screen.getByTestId(`lgm-host-${k}`)
const pool = () => screen.getByTestId('lgm-pool')
const saveBtn = () => screen.getByRole('button', { name: /Enregistrer les groupes/ })
const moveTo = (lightName, value) => fireEvent.change(screen.getByLabelText(`Déplacer ${lightName} vers…`), { target: { value } })

describe('LightGroupsManager — groupes sans nom (révision 3)', () => {
  it('aucun champ de nom : un groupe est son numéro (« Groupe 1 », « Groupe animateur 1 »)', () => {
    setup({ groups: [salle(['BuzzHue1']), salle(), host()] })
    expect(screen.queryByLabelText(/Nom du groupe/)).toBeNull()
    expect(screen.queryAllByRole('textbox')).toHaveLength(0)
    expect(within(group(1)).getByText('Groupe 1', { selector: 'strong' })).toBeInTheDocument()
    expect(within(group(2)).getByText('Groupe 2', { selector: 'strong' })).toBeInTheDocument()
    expect(within(hostBlock(1)).getByText('Groupe animateur 1', { selector: 'strong' })).toBeInTheDocument()
  })

  it('« Équipes à proximité » n\'existe plus (ni menu, ni aide, ni libellé)', () => {
    setup({ groups: [salle(['BuzzHue1']), host()], teams: ['Rouges', 'Bleus'] })
    expect(screen.queryByText(/Équipes à proximité/i)).toBeNull()
    expect(screen.queryByLabelText(/Ajouter une équipe/)).toBeNull()
    expect(screen.queryByLabelText(/Aide : équipes à proximité/i)).toBeNull()
    expect(screen.queryByText(/Vague/)).toBeNull()
  })

  it('l\'ordre de parcours est écrit en numéros (g1 → g2 → … → retour à g1), sans les animateurs', () => {
    setup({ groups: [salle(), host(), salle(), salle()] })
    const order = screen.getByLabelText('Ordre de parcours')
    expect(order).toHaveTextContent('g1')
    expect(order).toHaveTextContent('g3')
    expect(order).toHaveTextContent(/retour à g1/)
    expect(order).not.toHaveTextContent(/g4/)
  })

  it('l\'aide explique que les effets désignent les groupes par leur numéro (g1, g0 = toute la salle)', () => {
    setup()
    expect(screen.getByText(/g1/, { selector: 'code' })).toBeInTheDocument()
    expect(screen.getByText(/g0/, { selector: 'code' })).toBeInTheDocument()
  })

  it('une ancienne configuration avec name et near_teams s\'affiche sans erreur, sans aucun nom', () => {
    setup({ groups: [{ id: 'ga1', name: 'Scène gauche', role: 'salle', lights: ['BuzzHue1'], near_teams: ['Rouges'] }] })
    expect(screen.queryByText('Scène gauche')).toBeNull()
    expect(screen.queryByText('Rouges')).toBeNull()
    expect(within(group(1)).getByText('Groupe 1', { selector: 'strong' })).toBeInTheDocument()
    expect(within(group(1)).getByText('BuzzHue1')).toBeInTheDocument()
  })

  it('les numéros suivent le rang après réordonnancement (Monter / Descendre) : les ampoules changent de numéro', () => {
    setup({ groups: [salle(['BuzzHue1']), salle(['BuzzHue2']), salle(['BuzzHue3'])] })
    fireEvent.click(screen.getByRole('button', { name: 'Descendre le groupe 1' }))
    expect(within(group(1)).getByText('BuzzHue2')).toBeInTheDocument()
    expect(within(group(2)).getByText('BuzzHue1')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Monter le groupe 3' }))
    expect(within(group(2)).getByText('BuzzHue3')).toBeInTheDocument()
    expect(within(group(3)).getByText('BuzzHue1')).toBeInTheDocument()
  })

  it('un réordonnancement de groupes déjà enregistrés (avec id) avertit que g1, g2… désignent maintenant d\'autres groupes dans les effets ; l\'annulation lève l\'avertissement', () => {
    setup({ groups: [salle(['BuzzHue1'], 'ga1'), salle(['BuzzHue2'], 'gb2')] })
    expect(document.body).not.toHaveTextContent(/désignent maintenant d'autres groupes/)
    fireEvent.click(screen.getByRole('button', { name: 'Descendre le groupe 1' }))
    expect(document.body).toHaveTextContent(/désignent maintenant d'autres groupes/)
    fireEvent.click(screen.getByRole('button', { name: 'Annuler les modifications' }))
    expect(document.body).not.toHaveTextContent(/désignent maintenant d'autres groupes/)
  })

  it('ajouter un groupe n\'avertit pas : seul un changement d\'ordre des groupes enregistrés le fait', () => {
    setup({ groups: [salle(['BuzzHue1'], 'ga1')] })
    fireEvent.click(screen.getByRole('button', { name: '+ Nouveau groupe' }))
    expect(document.body).not.toHaveTextContent(/désignent maintenant d'autres groupes/)
  })

  it('« Déplacer vers… » désigne les groupes par leur numéro (Groupe n, 🎤 Groupe animateur k)', () => {
    setup({ groups: [salle(), salle(), host()] })
    const opts = within(screen.getByLabelText('Déplacer BuzzHue1 vers…')).getAllByRole('option').map(o => o.textContent)
    expect(opts).toEqual(['Hors groupe', 'Groupe 1', 'Groupe 2', '🎤 Groupe animateur 1'])
  })
})

describe('LightGroupsManager — états vides', () => {
  it('sans ampoule générale ni groupe : message qui renvoie à « Pont et ampoules »', () => {
    setup({ generalLights: [] })
    expect(screen.getByText(/Aucune ampoule n'a le rôle « Éclairage général »/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Nouveau groupe/ })).toBeNull()
  })

  it('sans groupe : propose de créer un groupe de salle ou un groupe animateur — jamais « un groupe par ampoule »', () => {
    setup()
    expect(screen.getByRole('button', { name: '+ Nouveau groupe' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Nouveau groupe animateur/ })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Créer un groupe par ampoule/ })).toBeNull()
  })

  it('toutes les ampoules générales sont d\'abord dans le bac « hors groupe »', () => {
    setup()
    const p = within(pool())
    for (const n of LIGHTS) expect(p.getByText(n)).toBeInTheDocument()
    expect(p.getByText(/hors groupe \(4\)/)).toBeInTheDocument()
  })

  it('« Enregistrer » est désactivé tant que rien n\'est modifié', () => {
    setup()
    expect(saveBtn()).toBeDisabled()
  })
})

describe('LightGroupsManager — création, numéros = rang', () => {
  it('« + Nouveau groupe » ajoute un groupe vide numéré à la suite', () => {
    setup({ groups: [salle(['BuzzHue1'])] })
    fireEvent.click(screen.getByRole('button', { name: '+ Nouveau groupe' }))
    expect(screen.getByLabelText('Groupe 2')).toBeInTheDocument()
    expect(within(group(2)).getByText('Groupe 2', { selector: 'strong' })).toBeInTheDocument()
    expect(screen.getByText(/Aucune ampoule — glissez-en une ici/)).toBeInTheDocument()
    expect(saveBtn()).toBeEnabled()
  })

  it('le premier groupe ne peut pas monter, le dernier ne peut pas descendre', () => {
    setup({ groups: [salle(), salle()] })
    expect(screen.getByRole('button', { name: 'Monter le groupe 1' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Descendre le groupe 2' })).toBeDisabled()
  })

  it('supprimer un groupe renvoie ses ampoules dans le bac et renumérote', () => {
    setup({ groups: [salle(['BuzzHue1']), salle(['BuzzHue2'])] })
    fireEvent.click(screen.getByRole('button', { name: 'Supprimer le groupe 1' }))
    expect(within(group(1)).getByText('BuzzHue2')).toBeInTheDocument()
    expect(screen.queryByTestId('lgm-group-2')).toBeNull()
    expect(within(pool()).getByText('BuzzHue1')).toBeInTheDocument()
  })

  it('12 groupes de salle au plus : la création est désactivée au plafond', () => {
    setup({ groups: Array.from({ length: 12 }, () => salle()) })
    expect(screen.getByRole('button', { name: '+ Nouveau groupe' })).toBeDisabled()
    expect(screen.getByText(/12 groupes de salle au plus/)).toBeInTheDocument()
  })
})

describe('LightGroupsManager — ranger les ampoules (alternative clavier)', () => {
  it('« Déplacer vers… » range une ampoule dans un groupe, l\'enlève du bac et du groupe précédent', () => {
    setup({ groups: [salle(['BuzzHue1']), salle()] })
    moveTo('BuzzHue1', 'g1')
    expect(within(group(2)).getByText('BuzzHue1')).toBeInTheDocument()
    expect(within(group(1)).queryByText('BuzzHue1')).toBeNull()
    moveTo('BuzzHue3', 'g0')
    expect(within(group(1)).getByText('BuzzHue3')).toBeInTheDocument()
    expect(within(pool()).queryByText('BuzzHue3')).toBeNull()
  })

  it('une ampoule n\'est jamais que dans un seul groupe', () => {
    setup({ groups: [salle(['BuzzHue1']), salle()] })
    moveTo('BuzzHue1', 'g1')
    expect(screen.getAllByText('BuzzHue1')).toHaveLength(1)
  })

  it('« Retirer du groupe » (✕) remet l\'ampoule dans le bac', () => {
    setup({ groups: [salle(['BuzzHue1'])] })
    fireEvent.click(screen.getByRole('button', { name: 'Retirer BuzzHue1 du groupe' }))
    expect(within(pool()).getByText('BuzzHue1')).toBeInTheDocument()
  })

  it('une ampoule enregistrée qui n\'est plus de rôle général est ignorée (retrait automatique)', () => {
    setup({ generalLights: ['BuzzHue2'], groups: [salle(['BuzzHue1', 'BuzzHue2'])] })
    expect(screen.queryByText('BuzzHue1')).toBeNull()
    expect(within(group(1)).getByText('BuzzHue2')).toBeInTheDocument()
  })
})

describe('LightGroupsManager — glisser-déposer natif', () => {
  const dt = () => {
    const store = {}
    return { setData: (k, v) => { store[k] = v }, getData: (k) => store[k] || '', effectAllowed: '', dropEffect: '' }
  }

  it('glisser une ampoule du bac vers un groupe', () => {
    setup({ groups: [salle(), salle()] })
    const data = dt()
    fireEvent.dragStart(screen.getByTestId('chip-BuzzHue2'), { dataTransfer: data })
    fireEvent.dragOver(group(2), { dataTransfer: data })
    fireEvent.drop(group(2), { dataTransfer: data })
    expect(within(group(2)).getByText('BuzzHue2')).toBeInTheDocument()
    expect(within(pool()).queryByText('BuzzHue2')).toBeNull()
  })

  it('glisser une ampoule d\'un groupe vers le bac la retire du groupe', () => {
    setup({ groups: [salle(['BuzzHue1'])] })
    const data = dt()
    fireEvent.dragStart(screen.getByTestId('chip-BuzzHue1'), { dataTransfer: data })
    fireEvent.drop(pool(), { dataTransfer: data })
    expect(within(pool()).getByText('BuzzHue1')).toBeInTheDocument()
  })

  it('glisser une ampoule vers un groupe animateur', () => {
    setup({ groups: [host()] })
    const data = dt()
    fireEvent.dragStart(screen.getByTestId('chip-BuzzHue3'), { dataTransfer: data })
    fireEvent.drop(hostBlock(1), { dataTransfer: data })
    expect(within(hostBlock(1)).getByText('BuzzHue3')).toBeInTheDocument()
  })

  it('rien n\'est déplaçable quand la page est désactivée', () => {
    setup({ groups: [salle(['BuzzHue1'])], disabled: true })
    expect(screen.getByTestId('chip-BuzzHue1')).toHaveAttribute('draggable', 'false')
    expect(screen.getByLabelText('Déplacer BuzzHue1 vers…')).toBeDisabled()
  })
})

describe('LightGroupsManager — enregistrement', () => {
  it('envoie le remplacement complet SANS name ni near_teams : salle dans l\'ordre puis animateurs, `id` conservés, absents pour les nouveaux', async () => {
    const { onSave, onNotify } = setup({ groups: [salle(['BuzzHue1'], 'ga1'), host(['BuzzHue4'], 'hp1')] })
    fireEvent.click(screen.getByRole('button', { name: '+ Nouveau groupe' }))
    moveTo('BuzzHue2', 'g1')
    fireEvent.click(screen.getByRole('button', { name: /Nouveau groupe animateur/ }))
    fireEvent.click(saveBtn())
    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1))
    expect(onSave.mock.calls[0][0]).toEqual([
      { id: 'ga1', role: 'salle', lights: ['BuzzHue1'] },
      { role: 'salle', lights: ['BuzzHue2'] },
      { id: 'hp1', role: 'animateur', lights: ['BuzzHue4'] },
      { role: 'animateur', lights: [] },
    ])
    for (const g of onSave.mock.calls[0][0]) {
      expect('name' in g || 'near_teams' in g).toBe(false)
    }
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith(expect.objectContaining({ type: 'success' })))
    expect(saveBtn()).toBeDisabled()
  })

  it('une ancienne configuration (name, near_teams) est réécrite sans ces clés à l\'enregistrement suivant', async () => {
    const { onSave } = setup({ groups: [{ id: 'ga1', name: 'Scène', role: 'salle', lights: ['BuzzHue1'], near_teams: ['Rouges'] }] })
    fireEvent.click(screen.getByRole('button', { name: '+ Nouveau groupe' }))
    fireEvent.click(saveBtn())
    await waitFor(() => expect(onSave).toHaveBeenCalled())
    expect(onSave.mock.calls[0][0][0]).toEqual({ id: 'ga1', role: 'salle', lights: ['BuzzHue1'] })
  })

  it('réordonner conserve l\'`id` de chaque groupe dans la charge utile', async () => {
    const { onSave } = setup({ groups: [salle([], 'ga1'), salle([], 'gb2'), host([], 'hp1'), host([], 'hq2')] })
    fireEvent.click(screen.getByRole('button', { name: 'Descendre le groupe 1' }))
    fireEvent.click(screen.getByRole('button', { name: 'Monter le groupe animateur 2' }))
    fireEvent.click(saveBtn())
    await waitFor(() => expect(onSave).toHaveBeenCalled())
    expect(onSave.mock.calls[0][0].map(g => g.id)).toEqual(['gb2', 'ga1', 'hq2', 'hp1'])
  })

  it('un refus du serveur affiche son message et conserve les modifications', async () => {
    const onSave = vi.fn().mockResolvedValue({ ok: false, message: 'ampoule inconnue' })
    const { onNotify } = setup({ groups: [salle()], onSave })
    fireEvent.click(screen.getByRole('button', { name: '+ Nouveau groupe' }))
    fireEvent.click(saveBtn())
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith({ message: 'ampoule inconnue', type: 'error' }))
    expect(screen.getByLabelText('Groupe 2')).toBeInTheDocument()
    expect(saveBtn()).toBeEnabled()
  })

  it('« Annuler les modifications » revient à l\'enregistré', () => {
    setup({ groups: [salle()] })
    fireEvent.click(screen.getByRole('button', { name: '+ Nouveau groupe' }))
    expect(screen.getByText(/Modifications non enregistrées/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Annuler les modifications' }))
    expect(screen.queryByTestId('lgm-group-2')).toBeNull()
    expect(saveBtn()).toBeDisabled()
  })

  it('désactivé : aucune modification possible', () => {
    setup({ groups: [salle()], disabled: true })
    expect(screen.getByRole('button', { name: '+ Nouveau groupe' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Supprimer le groupe 1' })).toBeDisabled()
  })

  it('sans nom, plus aucune règle de nom : l\'enregistrement n\'est jamais bloqué pour un nom', () => {
    setup({ groups: [salle(), salle(), host(), host()] })
    fireEvent.click(screen.getByRole('button', { name: '+ Nouveau groupe' }))
    expect(screen.queryByRole('alert')).toBeNull()
    expect(saveBtn()).toBeEnabled()
  })
})

describe('LightGroupsManager — essai en séquence d\'un groupe (§2.4)', () => {
  it('« Tester le groupe » appelle onTest avec le numéro du groupe', async () => {
    const { onTest } = setup({ groups: [salle(['BuzzHue1']), salle(['BuzzHue2'])] })
    fireEvent.click(within(group(2)).getByRole('button', { name: 'Tester le groupe' }))
    await waitFor(() => expect(onTest).toHaveBeenCalledWith(2))
  })

  it('désactivé pour un groupe vide', () => {
    setup({ groups: [salle()] })
    expect(within(group(1)).getByRole('button', { name: 'Tester le groupe' })).toBeDisabled()
  })

  it('désactivé tant que des modifications ne sont pas enregistrées, avec l\'infobulle « Enregistrez d\'abord »', () => {
    setup({ groups: [salle(['BuzzHue1'])] })
    fireEvent.click(screen.getByRole('button', { name: '+ Nouveau groupe' }))
    const btn = within(group(1)).getByRole('button', { name: 'Tester le groupe' })
    expect(btn).toBeDisabled()
    expect(btn).toHaveAttribute('title', "Enregistrez d'abord")
  })

  it('désactivé quand l\'éclairage d\'ambiance est désactivé', () => {
    setup({ groups: [salle(['BuzzHue1'])], enabled: false })
    const btn = within(group(1)).getByRole('button', { name: 'Tester le groupe' })
    expect(btn).toBeDisabled()
    expect(btn).toHaveAttribute('title', expect.stringMatching(/désactivé/))
  })

  it('désactivé avec la raison fournie quand le pont est injoignable', () => {
    setup({ groups: [salle(['BuzzHue1'])], testUnavailableReason: 'Pont injoignable' })
    const btn = within(group(1)).getByRole('button', { name: 'Tester le groupe' })
    expect(btn).toBeDisabled()
    expect(btn).toHaveAttribute('title', 'Pont injoignable')
  })

  it('un essai refusé est signalé par un avertissement', async () => {
    const onTest = vi.fn().mockResolvedValue({ ok: false, message: 'Pont injoignable' })
    const { onNotify } = setup({ groups: [salle(['BuzzHue1'])], onTest })
    fireEvent.click(within(group(1)).getByRole('button', { name: 'Tester le groupe' }))
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith({ message: 'Pont injoignable', type: 'warning' }))
  })

  it('pendant la séquence, le bouton devient « ■ Arrêter l\'essai » et indique l\'étape (1/6 éteint)', async () => {
    const onTest = vi.fn().mockResolvedValue({ ok: true, duration_ms: 6000 })
    setup({ groups: [salle(['BuzzHue1'])], onTest })
    fireEvent.click(within(group(1)).getByRole('button', { name: 'Tester le groupe' }))
    const stop = await within(group(1)).findByRole('button', { name: /Arrêter l'essai/ })
    expect(stop).toHaveAttribute('title', expect.stringMatching(/Étape 1\/6/))
    expect(stop.getAttribute('title')).toMatch(/éteint/)
  })

  it('« Arrêter l\'essai » appelle onStopTest et rend le bouton « Tester le groupe »', async () => {
    const onTest = vi.fn().mockResolvedValue({ ok: true, duration_ms: 6000 })
    const onStopTest = vi.fn().mockResolvedValue({ ok: true })
    setup({ groups: [salle(['BuzzHue1'])], onTest, onStopTest })
    fireEvent.click(within(group(1)).getByRole('button', { name: 'Tester le groupe' }))
    fireEvent.click(await within(group(1)).findByRole('button', { name: /Arrêter l'essai/ }))
    await waitFor(() => expect(onStopTest).toHaveBeenCalledTimes(1))
    expect(await within(group(1)).findByRole('button', { name: 'Tester le groupe' })).toBeInTheDocument()
  })

  it('la séquence se termine d\'elle-même après duration_ms (temps simulé)', async () => {
    vi.useFakeTimers()
    try {
      const onTest = vi.fn().mockResolvedValue({ ok: true, duration_ms: 6000 })
      setup({ groups: [salle(['BuzzHue1'])], onTest })
      await act(async () => { fireEvent.click(within(group(1)).getByRole('button', { name: 'Tester le groupe' })) })
      expect(within(group(1)).getByRole('button', { name: /Arrêter l'essai/ })).toBeInTheDocument()
      await act(async () => { vi.advanceTimersByTime(7000) })
      expect(within(group(1)).getByRole('button', { name: 'Tester le groupe' })).toBeInTheDocument()
    } finally { vi.useRealTimers() }
  })

  it('un essai sur un autre groupe remplace celui en cours (un seul « Arrêter l\'essai »)', async () => {
    const onTest = vi.fn().mockResolvedValue({ ok: true, duration_ms: 6000 })
    setup({ groups: [salle(['BuzzHue1']), salle(['BuzzHue2'])], onTest })
    fireEvent.click(within(group(1)).getByRole('button', { name: 'Tester le groupe' }))
    await within(group(1)).findByRole('button', { name: /Arrêter l'essai/ })
    fireEvent.click(within(group(2)).getByRole('button', { name: 'Tester le groupe' }))
    await waitFor(() => expect(onTest).toHaveBeenLastCalledWith(2))
    await within(group(2)).findByRole('button', { name: /Arrêter l'essai/ })
    expect(within(group(1)).getByRole('button', { name: 'Tester le groupe' })).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: /Arrêter l'essai/ })).toHaveLength(1)
  })

  it('un groupe animateur se teste avec la forme "host<k>" (rang propre)', async () => {
    const { onTest } = setup({ groups: [host(['BuzzHue1']), host(['BuzzHue2'])] })
    fireEvent.click(within(hostBlock(2)).getByRole('button', { name: 'Tester le groupe' }))
    await waitFor(() => expect(onTest).toHaveBeenCalledWith('host2'))
  })
})

describe('LightGroupsManager — groupes animateur multiples (§2.5)', () => {
  it('« 🎤 + Nouveau groupe animateur » en crée un, jusqu\'à 4', () => {
    setup()
    for (let k = 1; k <= 4; k++) {
      expect(screen.getByRole('button', { name: /Nouveau groupe animateur/ })).toBeEnabled()
      fireEvent.click(screen.getByRole('button', { name: /Nouveau groupe animateur/ }))
      expect(hostBlock(k)).toBeInTheDocument()
    }
    expect(screen.getByRole('button', { name: /Nouveau groupe animateur/ })).toBeDisabled()
    expect(screen.getAllByText(/4 groupes animateur au plus/).length).toBeGreaterThan(0)
  })

  it('numérotation propre 🎤1, 🎤2…, sans « équipes à proximité », avec la mention « blanc fixe par défaut »', () => {
    setup({ groups: [host(['BuzzHue1']), host()] })
    expect(within(hostBlock(1)).getByLabelText('Groupe animateur 1')).toHaveTextContent('🎤1')
    expect(within(hostBlock(2)).getByLabelText('Groupe animateur 2')).toHaveTextContent('🎤2')
    expect(within(hostBlock(1)).getByText(/Blanc fixe par défaut/)).toBeInTheDocument()
    expect(screen.getByText(/ni par ON \/ OFF, ni par le Flash/)).toBeInTheDocument()
  })

  it('les groupes animateur n\'entrent ni dans la numérotation de salle ni dans son plafond de 12', () => {
    const rooms = Array.from({ length: 11 }, () => salle())
    setup({ groups: [salle(), host(), ...rooms.slice(1), host()] })
    expect(screen.getByLabelText('Groupe 11')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '+ Nouveau groupe' })).toBeEnabled()
  })

  it('on y range une ampoule par « Déplacer vers… » (🎤 Groupe animateur k)', () => {
    setup({ groups: [host(), host()] })
    moveTo('BuzzHue2', 'h1')
    expect(within(hostBlock(2)).getByText('BuzzHue2')).toBeInTheDocument()
    expect(within(hostBlock(1)).queryByText('BuzzHue2')).toBeNull()
  })

  it('« Supprimer » renvoie ses ampoules dans le bac et renumérote les suivants', () => {
    setup({ groups: [host(['BuzzHue1']), host(['BuzzHue2'])] })
    fireEvent.click(screen.getByRole('button', { name: 'Supprimer le groupe animateur 1' }))
    expect(within(pool()).getByText('BuzzHue1')).toBeInTheDocument()
    expect(within(hostBlock(1)).getByText('BuzzHue2')).toBeInTheDocument()
    expect(screen.queryByTestId('lgm-host-2')).toBeNull()
  })

  it('les groupes animateur se réordonnent entre eux (Monter / Descendre)', () => {
    setup({ groups: [host(['BuzzHue1']), host(['BuzzHue2'])] })
    fireEvent.click(screen.getByRole('button', { name: 'Descendre le groupe animateur 1' }))
    expect(within(hostBlock(1)).getByText('BuzzHue2')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Monter le groupe animateur 1' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Descendre le groupe animateur 2' })).toBeDisabled()
  })

  it('« Choisir ses effets » : actif seulement pour un groupe enregistré (avec id) et sans modification en cours, appelle onChooseEffects(id)', () => {
    const onChooseEffects = vi.fn()
    setup({ groups: [host([], 'hp1')], onChooseEffects })
    fireEvent.click(within(hostBlock(1)).getByRole('button', { name: 'Choisir ses effets' }))
    expect(onChooseEffects).toHaveBeenCalledWith('hp1')
    fireEvent.click(screen.getByRole('button', { name: /Nouveau groupe animateur/ }))
    expect(within(hostBlock(1)).queryByRole('button', { name: 'Choisir ses effets' })).toBeNull()
    expect(within(hostBlock(2)).queryByRole('button', { name: 'Choisir ses effets' })).toBeNull()
  })

  it('sans onChooseEffects, le lien est inactif', () => {
    setup({ groups: [host([], 'hp1')] })
    expect(within(hostBlock(1)).queryByRole('button', { name: 'Choisir ses effets' })).toBeNull()
  })
})

describe('LightGroupsManager — résumé sous la liste', () => {
  it('signale ampoules hors groupe et groupes vides, et le nombre de groupes animés', () => {
    setup({ groups: [salle(['BuzzHue1']), salle(['BuzzHue2']), salle()] })
    const warn = screen.getByRole('status')
    expect(warn).toHaveTextContent(/2 ampoules générales sont hors groupe/)
    expect(warn).toHaveTextContent(/1 groupe est vide/)
    expect(warn).toHaveTextContent(/animeront 2 groupes/)
  })

  it('aucun avertissement quand tout est rangé', () => {
    setup({ generalLights: ['BuzzHue1'], groups: [salle(['BuzzHue1'])] })
    expect(screen.queryByRole('status')).toBeNull()
  })
})
