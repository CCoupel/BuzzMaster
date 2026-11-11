import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor, within, act } from '@testing-library/react'

// ---------------------------------------------------------------------------
// #225 (v11.7.0, LOT-1) — page « Groupes » de l'onglet Lumière, révision 4 : TROIS ZONES (Ambiance, Animateur, Public),
// groupes SANS NOM (un groupe est son numéro dans sa zone), 12 groupes au plus par zone, essai en séquence (§2.4).
// Écrit depuis contracts/light-effects.md §2.1, §2.4, §2.5 et la maquette docs/mockup/v11.7.0/ui/ambiance_page__light_groups_zones.html.
// Interface testée = props du composant (generalLights, groups, enabled, testUnavailableReason, onSave, onTest, onStopTest,
// onChooseEffects, onNotify) et ce que voit l'utilisateur. Complète LightGroupsManager.zones225.test.jsx.
// ---------------------------------------------------------------------------

vi.mock('./LightGroupsManager.css', () => ({}))

import LightGroupsManager from './LightGroupsManager'

const LIGHTS = ['BuzzHue1', 'BuzzHue2', 'BuzzHue3', 'BuzzHue4']

const withId = (role, lights, id) => ({ ...(id ? { id } : {}), role, lights })
const salle = (lights = [], id = '') => withId('salle', lights, id)
const host = (lights = [], id = '') => withId('animateur', lights, id)
const pub = (lights = [], id = '') => withId('public', lights, id)

function setup(props = {}) {
  const onSave = vi.fn().mockResolvedValue({ ok: true })
  const onTest = vi.fn().mockResolvedValue({ ok: true })
  const onNotify = vi.fn()
  const utils = render(
    <LightGroupsManager generalLights={LIGHTS} groups={[]} enabled onSave={onSave} onTest={onTest} onNotify={onNotify} {...props} />
  )
  return { onSave, onTest, onNotify, ...utils }
}

const grp = (zone, n) => screen.getByTestId(`lgm-${zone}-${n}`)
const group = (n) => grp('ambiance', n)
const hostBlock = (k) => grp('animateur', k)
const pubBlock = (k) => grp('public', k)
const zone = (z) => within(screen.getByTestId(`lgm-zone-${z}`))
const pool = () => screen.getByTestId('lgm-pool')
const saveBtn = () => screen.getByRole('button', { name: /Enregistrer les groupes/ })
const moveTo = (lightName, value) => fireEvent.change(screen.getByLabelText(`Déplacer ${lightName} vers…`), { target: { value } })
const NEW_ROOM = '+ Nouveau groupe'
const NEW_HOST = '🎤 + Nouveau groupe animateur'
const NEW_PUBLIC = '👥 + Nouveau groupe public'

describe('LightGroupsManager — groupes sans nom, trois zones', () => {
  it('aucun champ de nom : un groupe est son numéro dans sa zone (« Groupe 1 » dans chacune des trois)', () => {
    setup({ groups: [salle(['BuzzHue1']), salle(), host(), pub()] })
    expect(screen.queryByLabelText(/Nom du groupe/)).toBeNull()
    expect(screen.queryAllByRole('textbox')).toHaveLength(0)
    expect(within(group(1)).getByText('Groupe 1', { selector: 'strong' })).toBeInTheDocument()
    expect(within(group(2)).getByText('Groupe 2', { selector: 'strong' })).toBeInTheDocument()
    expect(within(hostBlock(1)).getByText('Groupe 1', { selector: 'strong' })).toBeInTheDocument()
    expect(within(pubBlock(1)).getByText('Groupe 1', { selector: 'strong' })).toBeInTheDocument()
  })

  it('trois zones annoncées avec leur nombre de groupes, même vides', () => {
    setup({ groups: [salle(), salle()] })
    expect(screen.getByTestId('lgm-zone-ambiance')).toHaveTextContent('Zone Ambiance — 2 groupes')
    expect(screen.getByTestId('lgm-zone-animateur')).toHaveTextContent('Zone Animateur — 0 groupe')
    expect(screen.getByTestId('lgm-zone-public')).toHaveTextContent('Zone Public — 0 groupe')
  })

  it('« Équipes à proximité » n\'existe plus (ni menu, ni aide, ni libellé)', () => {
    setup({ groups: [salle(['BuzzHue1']), host()], teams: ['Rouges', 'Bleus'] })
    expect(screen.queryByText(/Équipes à proximité/i)).toBeNull()
    expect(screen.queryByLabelText(/Ajouter une équipe/)).toBeNull()
    expect(screen.queryByLabelText(/Aide : équipes à proximité/i)).toBeNull()
    expect(screen.queryByText(/Vague/)).toBeNull()
  })

  it('l\'ordre de parcours de chaque zone est écrit en numéros (g1 → g2 → … → retour à g1), par zone', () => {
    setup({ groups: [salle(), host(), salle(), salle(), pub(), pub()] })
    const room = screen.getByLabelText('Ordre de parcours, zone Ambiance')
    expect(room).toHaveTextContent('g1')
    expect(room).toHaveTextContent('g3')
    expect(room).toHaveTextContent(/retour à g1/)
    const h = screen.getByLabelText('Ordre de parcours, zone Animateur')
    expect(h).toHaveTextContent('g1')
    expect(h).not.toHaveTextContent(/g2|retour/)
    const p = screen.getByLabelText('Ordre de parcours, zone Public')
    expect(p).toHaveTextContent(/g1.*g2.*retour à g1/)
  })

  it('l\'aide explique que les effets désignent les groupes par leur numéro (g1, g0 = toute la zone)', () => {
    setup()
    expect(screen.getByText(/g1/, { selector: 'code' })).toBeInTheDocument()
    expect(screen.getByText(/g0/, { selector: 'code' })).toBeInTheDocument()
  })

  it('les zones Animateur et Public expliquent leur comportement sans effet (blanc fixe / rien d\'envoyé)', () => {
    setup({ groups: [host(['BuzzHue1']), pub(['BuzzHue2'])] })
    expect(screen.getByTestId('lgm-zone-animateur')).toHaveTextContent(/blanc fixe/)
    expect(screen.getByTestId('lgm-zone-public')).toHaveTextContent(/non pilotée/)
    expect(screen.getByTestId('lgm-zone-public')).toHaveTextContent(/ni par ON \/ OFF, ni par le Flash/)
  })

  it('une ancienne configuration avec name et near_teams s\'affiche sans erreur, sans aucun nom', () => {
    setup({ groups: [{ id: 'ga1', name: 'Scène gauche', role: 'salle', lights: ['BuzzHue1'], near_teams: ['Rouges'] }] })
    expect(screen.queryByText('Scène gauche')).toBeNull()
    expect(screen.queryByText('Rouges')).toBeNull()
    expect(within(group(1)).getByText('Groupe 1', { selector: 'strong' })).toBeInTheDocument()
    expect(within(group(1)).getByText('BuzzHue1')).toBeInTheDocument()
  })

  it('les numéros suivent le rang dans la zone après réordonnancement (Monter / Descendre)', () => {
    setup({ groups: [salle(['BuzzHue1']), salle(['BuzzHue2']), salle(['BuzzHue3'])] })
    fireEvent.click(screen.getByRole('button', { name: 'Descendre le groupe 1 de la zone Ambiance' }))
    expect(within(group(1)).getByText('BuzzHue2')).toBeInTheDocument()
    expect(within(group(2)).getByText('BuzzHue1')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Monter le groupe 3 de la zone Ambiance' }))
    expect(within(group(2)).getByText('BuzzHue3')).toBeInTheDocument()
    expect(within(group(3)).getByText('BuzzHue1')).toBeInTheDocument()
  })

  it('un réordonnancement de groupes déjà enregistrés (avec id) avertit que g1, g2… désignent maintenant d\'autres groupes ; l\'annulation lève l\'avertissement', () => {
    setup({ groups: [salle(['BuzzHue1'], 'ga1'), salle(['BuzzHue2'], 'gb2')] })
    expect(document.body).not.toHaveTextContent(/désignent maintenant d'autres groupes/)
    fireEvent.click(screen.getByRole('button', { name: 'Descendre le groupe 1 de la zone Ambiance' }))
    expect(document.body).toHaveTextContent(/désignent maintenant d'autres groupes/)
    fireEvent.click(screen.getByRole('button', { name: 'Annuler les modifications' }))
    expect(document.body).not.toHaveTextContent(/désignent maintenant d'autres groupes/)
  })

  it('ajouter un groupe n\'avertit pas : seul un changement d\'ordre des groupes enregistrés le fait', () => {
    setup({ groups: [salle(['BuzzHue1'], 'ga1')] })
    fireEvent.click(screen.getByRole('button', { name: NEW_ROOM }))
    expect(document.body).not.toHaveTextContent(/désignent maintenant d'autres groupes/)
  })

  it('« Déplacer vers… » désigne chaque groupe par sa zone et son numéro', () => {
    setup({ groups: [salle(), salle(), host(), pub()] })
    const opts = within(screen.getByLabelText('Déplacer BuzzHue1 vers…')).getAllByRole('option').map(o => o.textContent)
    expect(opts).toEqual(['Hors groupe', '💡 Ambiance · Groupe 1', '💡 Ambiance · Groupe 2', '🎤 Animateur · Groupe 1', '👥 Public · Groupe 1'])
  })
})

describe('LightGroupsManager — états vides', () => {
  it('sans ampoule générale ni groupe : message qui renvoie à « Pont et ampoules »', () => {
    setup({ generalLights: [] })
    expect(screen.getByText(/Aucune ampoule n'a le rôle « Éclairage général »/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Nouveau groupe/ })).toBeNull()
  })

  it('sans groupe : propose de créer un groupe dans chacune des trois zones — jamais « un groupe par ampoule »', () => {
    setup()
    expect(screen.getByRole('button', { name: NEW_ROOM })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: NEW_HOST })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: NEW_PUBLIC })).toBeInTheDocument()
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

describe('LightGroupsManager — création, numéros = rang dans la zone', () => {
  it('« + Nouveau groupe » ajoute un groupe vide numéroté à la suite', () => {
    setup({ groups: [salle(['BuzzHue1'])] })
    fireEvent.click(screen.getByRole('button', { name: NEW_ROOM }))
    expect(screen.getByLabelText('Groupe 2, zone Ambiance')).toBeInTheDocument()
    expect(within(group(2)).getByText('Groupe 2', { selector: 'strong' })).toBeInTheDocument()
    expect(screen.getByText(/Aucune ampoule — glissez-en une ici/)).toBeInTheDocument()
    expect(saveBtn()).toBeEnabled()
  })

  it('chaque zone numérote ses groupes depuis 1, indépendamment des autres', () => {
    setup()
    fireEvent.click(screen.getByRole('button', { name: NEW_PUBLIC }))
    fireEvent.click(screen.getByRole('button', { name: NEW_PUBLIC }))
    fireEvent.click(screen.getByRole('button', { name: NEW_HOST }))
    expect(screen.getByLabelText('Groupe 2, zone Public')).toBeInTheDocument()
    expect(screen.getByLabelText('Groupe 1, zone Animateur')).toBeInTheDocument()
    expect(screen.queryByLabelText('Groupe 1, zone Ambiance')).toBeNull()
  })

  it('le premier groupe ne peut pas monter, le dernier ne peut pas descendre (dans sa zone)', () => {
    setup({ groups: [salle(), salle()] })
    expect(screen.getByRole('button', { name: 'Monter le groupe 1 de la zone Ambiance' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Descendre le groupe 2 de la zone Ambiance' })).toBeDisabled()
  })

  it('supprimer un groupe renvoie ses ampoules dans le bac et renumérote', () => {
    setup({ groups: [salle(['BuzzHue1']), salle(['BuzzHue2'])] })
    fireEvent.click(screen.getByRole('button', { name: 'Supprimer le groupe 1 de la zone Ambiance' }))
    expect(within(group(1)).getByText('BuzzHue2')).toBeInTheDocument()
    expect(screen.queryByTestId('lgm-ambiance-2')).toBeNull()
    expect(within(pool()).getByText('BuzzHue1')).toBeInTheDocument()
  })

  it('12 groupes au plus PAR ZONE : la création est désactivée dans la zone pleine seulement', () => {
    setup({ groups: [...Array.from({ length: 12 }, () => salle()), host()] })
    expect(screen.getByRole('button', { name: NEW_ROOM })).toBeDisabled()
    expect(screen.getByText(/12 groupes au plus dans la zone Ambiance/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: NEW_HOST })).toBeEnabled()
    expect(screen.getByRole('button', { name: NEW_PUBLIC })).toBeEnabled()
  })

  it('douze groupes dans chacune des trois zones : aucune zone ne compte dans le plafond d\'une autre', () => {
    const all = [...Array.from({ length: 12 }, () => salle()), ...Array.from({ length: 12 }, () => host()), ...Array.from({ length: 12 }, () => pub())]
    setup({ groups: all })
    for (const b of [NEW_ROOM, NEW_HOST, NEW_PUBLIC]) expect(screen.getByRole('button', { name: b })).toBeDisabled()
    expect(screen.getByLabelText('Groupe 12, zone Public')).toBeInTheDocument()
  })
})

describe('LightGroupsManager — ranger les ampoules (alternative clavier)', () => {
  it('« Déplacer vers… » range une ampoule dans un groupe, l\'enlève du bac et du groupe précédent', () => {
    setup({ groups: [salle(['BuzzHue1']), salle()] })
    moveTo('BuzzHue1', 'ambiance:1')
    expect(within(group(2)).getByText('BuzzHue1')).toBeInTheDocument()
    expect(within(group(1)).queryByText('BuzzHue1')).toBeNull()
    moveTo('BuzzHue3', 'ambiance:0')
    expect(within(group(1)).getByText('BuzzHue3')).toBeInTheDocument()
    expect(within(pool()).queryByText('BuzzHue3')).toBeNull()
  })

  it('une ampoule n\'est jamais que dans un seul groupe, toutes zones confondues', () => {
    setup({ groups: [salle(['BuzzHue1']), host(), pub()] })
    moveTo('BuzzHue1', 'animateur:0')
    moveTo('BuzzHue1', 'public:0')
    expect(screen.getAllByText('BuzzHue1')).toHaveLength(1)
    expect(within(pubBlock(1)).getByText('BuzzHue1')).toBeInTheDocument()
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

  it('glisser une ampoule vers un groupe de la zone Animateur, puis de la zone Public (d\'une zone à l\'autre)', () => {
    setup({ groups: [host(), pub()] })
    const data = dt()
    fireEvent.dragStart(screen.getByTestId('chip-BuzzHue3'), { dataTransfer: data })
    fireEvent.drop(hostBlock(1), { dataTransfer: data })
    expect(within(hostBlock(1)).getByText('BuzzHue3')).toBeInTheDocument()
    const data2 = dt()
    fireEvent.dragStart(screen.getByTestId('chip-BuzzHue3'), { dataTransfer: data2 })
    fireEvent.drop(pubBlock(1), { dataTransfer: data2 })
    expect(within(pubBlock(1)).getByText('BuzzHue3')).toBeInTheDocument()
    expect(within(hostBlock(1)).queryByText('BuzzHue3')).toBeNull()
  })

  it('rien n\'est déplaçable quand la page est désactivée', () => {
    setup({ groups: [salle(['BuzzHue1'])], disabled: true })
    expect(screen.getByTestId('chip-BuzzHue1')).toHaveAttribute('draggable', 'false')
    expect(screen.getByLabelText('Déplacer BuzzHue1 vers…')).toBeDisabled()
  })
})

describe('LightGroupsManager — enregistrement', () => {
  it('envoie le remplacement complet SANS name ni near_teams : ambiance (salle), animateur, public dans l\'ordre, `id` conservés, absents pour les nouveaux', async () => {
    const { onSave, onNotify } = setup({ groups: [salle(['BuzzHue1'], 'ga1'), host(['BuzzHue4'], 'hp1'), pub(['BuzzHue3'], 'pq1')] })
    fireEvent.click(screen.getByRole('button', { name: NEW_ROOM }))
    moveTo('BuzzHue2', 'ambiance:1')
    fireEvent.click(screen.getByRole('button', { name: NEW_HOST }))
    fireEvent.click(saveBtn())
    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1))
    expect(onSave.mock.calls[0][0]).toEqual([
      { id: 'ga1', role: 'salle', lights: ['BuzzHue1'] },
      { role: 'salle', lights: ['BuzzHue2'] },
      { id: 'hp1', role: 'animateur', lights: ['BuzzHue4'] },
      { role: 'animateur', lights: [] },
      { id: 'pq1', role: 'public', lights: ['BuzzHue3'] },
    ])
    for (const g of onSave.mock.calls[0][0]) {
      expect('name' in g || 'near_teams' in g).toBe(false)
    }
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith(expect.objectContaining({ type: 'success' })))
    expect(saveBtn()).toBeDisabled()
  })

  it('une ancienne configuration (name, near_teams) est réécrite sans ces clés à l\'enregistrement suivant', async () => {
    const { onSave } = setup({ groups: [{ id: 'ga1', name: 'Scène', role: 'salle', lights: ['BuzzHue1'], near_teams: ['Rouges'] }] })
    fireEvent.click(screen.getByRole('button', { name: NEW_ROOM }))
    fireEvent.click(saveBtn())
    await waitFor(() => expect(onSave).toHaveBeenCalled())
    expect(onSave.mock.calls[0][0][0]).toEqual({ id: 'ga1', role: 'salle', lights: ['BuzzHue1'] })
  })

  it('réordonner conserve l\'`id` de chaque groupe dans la charge utile', async () => {
    const { onSave } = setup({ groups: [salle([], 'ga1'), salle([], 'gb2'), host([], 'hp1'), host([], 'hq2')] })
    fireEvent.click(screen.getByRole('button', { name: 'Descendre le groupe 1 de la zone Ambiance' }))
    fireEvent.click(screen.getByRole('button', { name: 'Monter le groupe 2 de la zone Animateur' }))
    fireEvent.click(saveBtn())
    await waitFor(() => expect(onSave).toHaveBeenCalled())
    expect(onSave.mock.calls[0][0].map(g => g.id)).toEqual(['gb2', 'ga1', 'hq2', 'hp1'])
  })

  it('un refus du serveur affiche son message et conserve les modifications', async () => {
    const onSave = vi.fn().mockResolvedValue({ ok: false, message: 'ampoule inconnue' })
    const { onNotify } = setup({ groups: [salle()], onSave })
    fireEvent.click(screen.getByRole('button', { name: NEW_ROOM }))
    fireEvent.click(saveBtn())
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith({ message: 'ampoule inconnue', type: 'error' }))
    expect(screen.getByLabelText('Groupe 2, zone Ambiance')).toBeInTheDocument()
    expect(saveBtn()).toBeEnabled()
  })

  it('« Annuler les modifications » revient à l\'enregistré', () => {
    setup({ groups: [salle()] })
    fireEvent.click(screen.getByRole('button', { name: NEW_ROOM }))
    expect(screen.getByText(/Modifications non enregistrées/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Annuler les modifications' }))
    expect(screen.queryByTestId('lgm-ambiance-2')).toBeNull()
    expect(saveBtn()).toBeDisabled()
  })

  it('désactivé : aucune modification possible', () => {
    setup({ groups: [salle()], disabled: true })
    expect(screen.getByRole('button', { name: NEW_ROOM })).toBeDisabled()
    expect(screen.getByRole('button', { name: NEW_PUBLIC })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Supprimer le groupe 1 de la zone Ambiance' })).toBeDisabled()
  })

  it('sans nom, plus aucune règle de nom : l\'enregistrement n\'est jamais bloqué pour un nom', () => {
    setup({ groups: [salle(), salle(), host(), host(), pub()] })
    fireEvent.click(screen.getByRole('button', { name: NEW_ROOM }))
    expect(screen.queryByRole('alert')).toBeNull()
    expect(saveBtn()).toBeEnabled()
  })
})

describe('LightGroupsManager — essai en séquence d\'un groupe (§2.4)', () => {
  it('« Tester le groupe » appelle onTest avec le numéro du groupe (zone Ambiance)', async () => {
    const { onTest } = setup({ groups: [salle(['BuzzHue1']), salle(['BuzzHue2'])] })
    fireEvent.click(within(group(2)).getByRole('button', { name: 'Tester le groupe' }))
    await waitFor(() => expect(onTest).toHaveBeenCalledWith(2))
  })

  it('un groupe animateur se teste avec "host<k>" et un groupe public avec "public<k>" (rang dans la zone)', async () => {
    const { onTest } = setup({ groups: [host(['BuzzHue1']), host(['BuzzHue2']), pub(['BuzzHue3']), pub(['BuzzHue4'])] })
    fireEvent.click(within(hostBlock(2)).getByRole('button', { name: 'Tester le groupe' }))
    await waitFor(() => expect(onTest).toHaveBeenLastCalledWith('host2'))
    fireEvent.click(within(pubBlock(2)).getByRole('button', { name: 'Tester le groupe' }))
    await waitFor(() => expect(onTest).toHaveBeenLastCalledWith('public2'))
  })

  it('désactivé pour un groupe vide', () => {
    setup({ groups: [salle()] })
    expect(within(group(1)).getByRole('button', { name: 'Tester le groupe' })).toBeDisabled()
  })

  it('désactivé tant que des modifications ne sont pas enregistrées, avec l\'infobulle « Enregistrez d\'abord »', () => {
    setup({ groups: [salle(['BuzzHue1'])] })
    fireEvent.click(screen.getByRole('button', { name: NEW_ROOM }))
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

  it('un essai sur un autre groupe — même d\'une autre zone — remplace celui en cours (un seul « Arrêter l\'essai »)', async () => {
    const onTest = vi.fn().mockResolvedValue({ ok: true, duration_ms: 6000 })
    setup({ groups: [salle(['BuzzHue1']), salle(['BuzzHue2']), pub(['BuzzHue3'])], onTest })
    fireEvent.click(within(group(1)).getByRole('button', { name: 'Tester le groupe' }))
    await within(group(1)).findByRole('button', { name: /Arrêter l'essai/ })
    fireEvent.click(within(group(2)).getByRole('button', { name: 'Tester le groupe' }))
    await waitFor(() => expect(onTest).toHaveBeenLastCalledWith(2))
    await within(group(2)).findByRole('button', { name: /Arrêter l'essai/ })
    expect(within(group(1)).getByRole('button', { name: 'Tester le groupe' })).toBeInTheDocument()
    fireEvent.click(within(pubBlock(1)).getByRole('button', { name: 'Tester le groupe' }))
    await waitFor(() => expect(onTest).toHaveBeenLastCalledWith('public1'))
    await within(pubBlock(1)).findByRole('button', { name: /Arrêter l'essai/ })
    expect(screen.getAllByRole('button', { name: /Arrêter l'essai/ })).toHaveLength(1)
  })
})

describe('LightGroupsManager — zones Animateur et Public (§2.5)', () => {
  it('« 🎤 + Nouveau groupe animateur » et « 👥 + Nouveau groupe public » créent des groupes jusqu\'à 12', () => {
    setup()
    for (let k = 1; k <= 12; k++) {
      expect(screen.getByRole('button', { name: NEW_HOST })).toBeEnabled()
      fireEvent.click(screen.getByRole('button', { name: NEW_HOST }))
      expect(hostBlock(k)).toBeInTheDocument()
    }
    expect(screen.getByRole('button', { name: NEW_HOST })).toBeDisabled()
    expect(screen.getByText(/12 groupes au plus dans la zone Animateur/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: NEW_PUBLIC }))
    expect(pubBlock(1)).toBeInTheDocument()
  })

  it('les groupes des zones Animateur et Public n\'entrent ni dans la numérotation de l\'ambiance ni dans son plafond', () => {
    setup({ groups: [salle(), host(), ...Array.from({ length: 10 }, () => salle()), host(), pub()] })
    expect(screen.getByLabelText('Groupe 11, zone Ambiance')).toBeInTheDocument()
    expect(screen.getByLabelText('Groupe 2, zone Animateur')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: NEW_ROOM })).toBeEnabled()
  })

  it('on range une ampoule dans la zone Animateur par « Déplacer vers… »', () => {
    setup({ groups: [host(), host()] })
    moveTo('BuzzHue2', 'animateur:1')
    expect(within(hostBlock(2)).getByText('BuzzHue2')).toBeInTheDocument()
    expect(within(hostBlock(1)).queryByText('BuzzHue2')).toBeNull()
  })

  it('« Supprimer » renvoie ses ampoules dans le bac et renumérote les suivants (zone Animateur)', () => {
    setup({ groups: [host(['BuzzHue1']), host(['BuzzHue2'])] })
    fireEvent.click(screen.getByRole('button', { name: 'Supprimer le groupe 1 de la zone Animateur' }))
    expect(within(pool()).getByText('BuzzHue1')).toBeInTheDocument()
    expect(within(hostBlock(1)).getByText('BuzzHue2')).toBeInTheDocument()
    expect(screen.queryByTestId('lgm-animateur-2')).toBeNull()
  })

  it('les groupes d\'une zone se réordonnent entre eux (Monter / Descendre) sans toucher les autres zones', () => {
    setup({ groups: [host(['BuzzHue1']), host(['BuzzHue2']), pub(['BuzzHue3'])] })
    fireEvent.click(screen.getByRole('button', { name: 'Descendre le groupe 1 de la zone Animateur' }))
    expect(within(hostBlock(1)).getByText('BuzzHue2')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Monter le groupe 1 de la zone Animateur' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Descendre le groupe 2 de la zone Animateur' })).toBeDisabled()
    expect(within(pubBlock(1)).getByText('BuzzHue3')).toBeInTheDocument()
  })

  it('« Choisir les effets de la zone » : actif seulement quand tous les groupes sont enregistrés (id) et sans modification, appelle onChooseEffects(zone)', () => {
    const onChooseEffects = vi.fn()
    setup({ groups: [host([], 'hp1'), pub([], 'pq1')], onChooseEffects })
    fireEvent.click(zone('animateur').getByRole('button', { name: 'Choisir les effets de la zone' }))
    expect(onChooseEffects).toHaveBeenLastCalledWith('animateur')
    fireEvent.click(zone('public').getByRole('button', { name: 'Choisir les effets de la zone' }))
    expect(onChooseEffects).toHaveBeenLastCalledWith('public')
    fireEvent.click(screen.getByRole('button', { name: NEW_HOST }))
    expect(zone('animateur').queryByRole('button', { name: 'Choisir les effets de la zone' })).toBeNull()
    expect(zone('public').queryByRole('button', { name: 'Choisir les effets de la zone' })).toBeNull()
  })

  it('sans onChooseEffects, le lien est inactif ; la zone Ambiance n\'en a jamais', () => {
    setup({ groups: [salle([], 'ga1'), host([], 'hp1')] })
    expect(screen.queryByRole('button', { name: 'Choisir les effets de la zone' })).toBeNull()
  })

  it('une zone sans groupe ne propose pas « Choisir les effets de la zone »', () => {
    setup({ groups: [host([], 'hp1')], onChooseEffects: vi.fn() })
    expect(zone('public').queryByText('Choisir les effets de la zone')).toBeNull()
  })
})

describe('LightGroupsManager — résumé sous la liste', () => {
  it('signale ampoules hors groupe et groupes vides, et le nombre de groupes animés (zone Ambiance)', () => {
    setup({ groups: [salle(['BuzzHue1']), salle(['BuzzHue2']), salle()] })
    const warn = screen.getByRole('status')
    expect(warn).toHaveTextContent(/2 ampoules générales sont hors groupe/)
    expect(warn).toHaveTextContent(/1 groupe d'ambiance est vide/)
    expect(warn).toHaveTextContent(/animeront 2 groupes dans la zone Ambiance/)
  })

  it('aucun avertissement quand tout est rangé', () => {
    setup({ generalLights: ['BuzzHue1'], groups: [salle(['BuzzHue1'])] })
    expect(screen.queryByRole('status')).toBeNull()
  })
})
