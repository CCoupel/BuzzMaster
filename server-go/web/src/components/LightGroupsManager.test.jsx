import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'

// ---------------------------------------------------------------------------
// #225 (v11.7.0, LOT-1) — page « Groupes » de l'onglet Lumière.
// Écrit depuis contracts/light-effects.md §2.1, §2.4, §2.5 et la maquette
// docs/mockup/v11.7.0/ui/ambiance_page__light_groups.html (sections 02 à 04).
// Interface testée = props du composant (generalLights, teams, groups, enabled,
// onSave, onTest, onChooseEffects, onNotify) et ce que voit l'utilisateur.
// ---------------------------------------------------------------------------

vi.mock('./LightGroupsManager.css', () => ({}))

import LightGroupsManager from './LightGroupsManager'

const LIGHTS = ['BuzzHue1', 'BuzzHue2', 'BuzzHue3', 'BuzzHue4']
const TEAMS = ['Rouges', 'Bleus']

const salle = (name, lights = [], near_teams = []) => ({ name, role: 'salle', lights, near_teams })
const host = (name, lights = []) => ({ name, role: 'animateur', lights, near_teams: [] })

function setup(props = {}) {
  const onSave = vi.fn().mockResolvedValue({ ok: true })
  const onTest = vi.fn().mockResolvedValue({ ok: true })
  const onNotify = vi.fn()
  const utils = render(
    <LightGroupsManager
      generalLights={LIGHTS}
      teams={TEAMS}
      groups={[]}
      enabled
      onSave={onSave}
      onTest={onTest}
      onNotify={onNotify}
      {...props}
    />
  )
  return { onSave, onTest, onNotify, ...utils }
}

const group = (n) => screen.getByTestId(`lgm-group-${n}`)
const pool = () => screen.getByTestId('lgm-pool')
const saveBtn = () => screen.getByRole('button', { name: /Enregistrer les groupes/ })
const moveTo = (lightName, value) =>
  fireEvent.change(screen.getByLabelText(`Déplacer ${lightName} vers…`), { target: { value } })

describe('LightGroupsManager — états vides', () => {
  it('sans ampoule générale ni groupe : message qui renvoie à « Pont et ampoules »', () => {
    setup({ generalLights: [] })
    expect(screen.getByText(/Aucune ampoule n'a le rôle « Éclairage général »/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Nouveau groupe/ })).toBeNull()
  })

  it('sans groupe : explique que l\'éclairage se comporte comme avant et propose les trois créations', () => {
    setup()
    expect(screen.getByText(/l'éclairage se comporte comme avant/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Créer un groupe par ampoule' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '+ Nouveau groupe' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Créer le groupe animateur/ })).toBeInTheDocument()
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
  it('« Créer un groupe par ampoule » : un groupe par ampoule, nommé d\'après elle, dans l\'ordre', () => {
    setup()
    fireEvent.click(screen.getByRole('button', { name: 'Créer un groupe par ampoule' }))
    expect(screen.getByLabelText('Nom du groupe 1')).toHaveValue('BuzzHue1')
    expect(screen.getByLabelText('Nom du groupe 4')).toHaveValue('BuzzHue4')
    expect(within(group(2)).getByText('BuzzHue2')).toBeInTheDocument()
    expect(saveBtn()).toBeEnabled()
  })

  it('« + Nouveau groupe » ajoute un groupe vide numéroté à la suite', () => {
    setup({ groups: [salle('Scène', ['BuzzHue1'])] })
    fireEvent.click(screen.getByRole('button', { name: '+ Nouveau groupe' }))
    expect(screen.getByLabelText('Groupe 2')).toBeInTheDocument()
    expect(screen.getByLabelText('Nom du groupe 2').value).not.toBe('')
    expect(screen.getByText(/Aucune ampoule — glissez-en une ici/)).toBeInTheDocument()
  })

  it('le numéro est le rang : monter / descendre un groupe recalcule les numéros', () => {
    setup({ groups: [salle('A'), salle('B'), salle('C')] })
    fireEvent.click(screen.getByRole('button', { name: 'Descendre le groupe 1' }))
    expect(screen.getByLabelText('Nom du groupe 1')).toHaveValue('B')
    expect(screen.getByLabelText('Nom du groupe 2')).toHaveValue('A')
    fireEvent.click(screen.getByRole('button', { name: 'Monter le groupe 3' }))
    expect(screen.getByLabelText('Nom du groupe 2')).toHaveValue('C')
    expect(screen.getByLabelText('Nom du groupe 3')).toHaveValue('A')
  })

  it('le premier groupe ne peut pas monter, le dernier ne peut pas descendre', () => {
    setup({ groups: [salle('A'), salle('B')] })
    expect(screen.getByRole('button', { name: 'Monter le groupe 1' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Descendre le groupe 2' })).toBeDisabled()
  })

  it('supprimer un groupe renvoie ses ampoules dans le bac et renumérote', () => {
    setup({ groups: [salle('A', ['BuzzHue1']), salle('B', ['BuzzHue2'])] })
    fireEvent.click(screen.getByRole('button', { name: 'Supprimer le groupe 1' }))
    expect(screen.getByLabelText('Nom du groupe 1')).toHaveValue('B')
    expect(screen.queryByLabelText('Nom du groupe 2')).toBeNull()
    expect(within(pool()).getByText('BuzzHue1')).toBeInTheDocument()
  })

  it('12 groupes au plus : les créations sont désactivées au plafond', () => {
    const many = Array.from({ length: 12 }, (_, i) => salle(`G${i + 1}`))
    setup({ groups: many })
    expect(screen.getByRole('button', { name: '+ Nouveau groupe' })).toBeDisabled()
    expect(screen.getByText(/12 groupes au plus/)).toBeInTheDocument()
  })
})

describe('LightGroupsManager — ranger les ampoules (alternative clavier)', () => {
  it('« Déplacer vers… » range une ampoule dans un groupe, l\'enlève du bac et du groupe précédent', () => {
    setup({ groups: [salle('A', ['BuzzHue1']), salle('B')] })
    moveTo('BuzzHue1', 'g1')
    expect(within(group(2)).getByText('BuzzHue1')).toBeInTheDocument()
    expect(within(group(1)).queryByText('BuzzHue1')).toBeNull()
    moveTo('BuzzHue3', 'g0')
    expect(within(group(1)).getByText('BuzzHue3')).toBeInTheDocument()
    expect(within(pool()).queryByText('BuzzHue3')).toBeNull()
  })

  it('une ampoule n\'est jamais que dans un seul groupe', () => {
    setup({ groups: [salle('A', ['BuzzHue1']), salle('B')] })
    moveTo('BuzzHue1', 'g1')
    expect(screen.getAllByText('BuzzHue1')).toHaveLength(1)
  })

  it('« Retirer du groupe » (✕) remet l\'ampoule dans le bac', () => {
    setup({ groups: [salle('A', ['BuzzHue1'])] })
    fireEvent.click(screen.getByRole('button', { name: 'Retirer BuzzHue1 du groupe' }))
    expect(within(pool()).getByText('BuzzHue1')).toBeInTheDocument()
  })

  it('une ampoule enregistrée qui n\'est plus de rôle général est ignorée (retrait automatique)', () => {
    setup({ generalLights: ['BuzzHue2'], groups: [salle('A', ['BuzzHue1', 'BuzzHue2'])] })
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
    setup({ groups: [salle('A'), salle('B')] })
    const data = dt()
    fireEvent.dragStart(screen.getByTestId('chip-BuzzHue2'), { dataTransfer: data })
    fireEvent.dragOver(group(2), { dataTransfer: data })
    fireEvent.drop(group(2), { dataTransfer: data })
    expect(within(group(2)).getByText('BuzzHue2')).toBeInTheDocument()
    expect(within(pool()).queryByText('BuzzHue2')).toBeNull()
  })

  it('glisser une ampoule d\'un groupe vers le bac la retire du groupe', () => {
    setup({ groups: [salle('A', ['BuzzHue1'])] })
    const data = dt()
    fireEvent.dragStart(screen.getByTestId('chip-BuzzHue1'), { dataTransfer: data })
    fireEvent.drop(pool(), { dataTransfer: data })
    expect(within(pool()).getByText('BuzzHue1')).toBeInTheDocument()
  })

  it('glisser une ampoule vers le groupe animateur', () => {
    setup({ groups: [host('Pupitre')] })
    const data = dt()
    fireEvent.dragStart(screen.getByTestId('chip-BuzzHue3'), { dataTransfer: data })
    fireEvent.drop(screen.getByTestId('lgm-host'), { dataTransfer: data })
    expect(within(screen.getByTestId('lgm-host')).getByText('BuzzHue3')).toBeInTheDocument()
  })

  it('rien n\'est déplaçable quand la page est désactivée', () => {
    setup({ groups: [salle('A', ['BuzzHue1'])], disabled: true })
    expect(screen.getByTestId('chip-BuzzHue1')).toHaveAttribute('draggable', 'false')
    expect(screen.getByLabelText('Déplacer BuzzHue1 vers…')).toBeDisabled()
  })
})

describe('LightGroupsManager — équipes à proximité', () => {
  it('ajouter une équipe à un groupe ; elle n\'est plus proposée aux autres groupes', () => {
    setup({ groups: [salle('A'), salle('B')] })
    fireEvent.change(screen.getByLabelText('Ajouter une équipe au groupe 1'), { target: { value: 'Rouges' } })
    expect(within(group(1)).getByText('Rouges')).toBeInTheDocument()
    const optionsB = within(screen.getByLabelText('Ajouter une équipe au groupe 2')).getAllByRole('option').map(o => o.textContent)
    expect(optionsB).not.toContain('Rouges')
    expect(optionsB).toContain('Bleus')
  })

  it('retirer une équipe la remet à disposition', () => {
    setup({ groups: [salle('A', [], ['Rouges']), salle('B')] })
    fireEvent.click(screen.getByRole('button', { name: "Retirer l'équipe Rouges du groupe 1" }))
    const optionsB = within(screen.getByLabelText('Ajouter une équipe au groupe 2')).getAllByRole('option').map(o => o.textContent)
    expect(optionsB).toContain('Rouges')
  })

  it('une équipe qui n\'existe plus est retirée du brouillon', () => {
    setup({ groups: [salle('A', [], ['Fantômes'])] })
    expect(screen.queryByText('Fantômes')).toBeNull()
  })
})

describe('LightGroupsManager — validation', () => {
  it('nom vide : message, « Enregistrer » désactivé', () => {
    setup({ groups: [salle('A')] })
    fireEvent.change(screen.getByLabelText('Nom du groupe 1'), { target: { value: '   ' } })
    expect(screen.getByRole('alert')).toHaveTextContent(/nom est obligatoire/i)
    expect(saveBtn()).toBeDisabled()
  })

  it('nom en double (casse ignorée) : message sur le groupe, enregistrement bloqué', () => {
    setup({ groups: [salle('Bar'), salle('Scène')] })
    fireEvent.change(screen.getByLabelText('Nom du groupe 2'), { target: { value: 'BAR' } })
    expect(screen.getByRole('alert')).toHaveTextContent(/déjà utilisé/i)
    expect(screen.getByLabelText('Nom du groupe 2')).toHaveAttribute('aria-invalid', 'true')
    expect(saveBtn()).toBeDisabled()
  })

  it('nom de plus de 40 caractères : refusé', () => {
    setup({ groups: [salle('A')] })
    fireEvent.change(screen.getByLabelText('Nom du groupe 1'), { target: { value: 'x'.repeat(41) } })
    expect(screen.getByRole('alert')).toHaveTextContent(/40 caractères/)
    expect(saveBtn()).toBeDisabled()
  })

  it('le nom du groupe animateur suit les mêmes règles et compte dans l\'unicité', () => {
    setup({ groups: [salle('Pupitre'), host('Pupitre')] })
    expect(screen.getByRole('alert')).toHaveTextContent(/déjà utilisé/i)
  })
})

describe('LightGroupsManager — enregistrement', () => {
  it('envoie le remplacement complet : groupes de salle dans l\'ordre (rôle salle) puis groupe animateur', async () => {
    const { onSave, onNotify } = setup({ groups: [salle('A', ['BuzzHue1'], ['Rouges']), host('Pupitre', ['BuzzHue4'])] })
    fireEvent.click(screen.getByRole('button', { name: '+ Nouveau groupe' }))
    fireEvent.change(screen.getByLabelText('Nom du groupe 2'), { target: { value: ' Bar ' } })
    moveTo('BuzzHue2', 'g1')
    fireEvent.click(saveBtn())
    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1))
    expect(onSave.mock.calls[0][0]).toEqual([
      { name: 'A', role: 'salle', lights: ['BuzzHue1'], near_teams: ['Rouges'] },
      { name: 'Bar', role: 'salle', lights: ['BuzzHue2'], near_teams: [] },
      { name: 'Pupitre', role: 'animateur', lights: ['BuzzHue4'], near_teams: [] },
    ])
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith(expect.objectContaining({ type: 'success' })))
    expect(saveBtn()).toBeDisabled()
  })

  it('un refus du serveur affiche son message et conserve les modifications', async () => {
    const onSave = vi.fn().mockResolvedValue({ ok: false, message: 'ampoule inconnue' })
    const { onNotify } = setup({ groups: [salle('A')], onSave })
    fireEvent.click(screen.getByRole('button', { name: '+ Nouveau groupe' }))
    fireEvent.click(saveBtn())
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith({ message: 'ampoule inconnue', type: 'error' }))
    expect(screen.getByLabelText('Nom du groupe 2')).toBeInTheDocument()
    expect(saveBtn()).toBeEnabled()
  })

  it('« Annuler les modifications » revient à l\'enregistré', () => {
    setup({ groups: [salle('A')] })
    fireEvent.click(screen.getByRole('button', { name: '+ Nouveau groupe' }))
    expect(screen.getByText(/Modifications non enregistrées/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Annuler les modifications' }))
    expect(screen.queryByLabelText('Nom du groupe 2')).toBeNull()
    expect(saveBtn()).toBeDisabled()
  })

  it('désactivé : aucune modification possible, rien à enregistrer', () => {
    setup({ groups: [salle('A')], disabled: true })
    expect(screen.getByRole('button', { name: '+ Nouveau groupe' })).toBeDisabled()
    expect(screen.getByLabelText('Nom du groupe 1')).toBeDisabled()
  })
})

describe('LightGroupsManager — essai d\'un groupe (§2.4)', () => {
  it('« Tester le groupe » appelle onTest avec le numéro du groupe', async () => {
    const { onTest } = setup({ groups: [salle('A', ['BuzzHue1']), salle('B', ['BuzzHue2'])] })
    fireEvent.click(within(group(2)).getByRole('button', { name: 'Tester le groupe' }))
    await waitFor(() => expect(onTest).toHaveBeenCalledWith(2))
  })

  it('désactivé pour un groupe vide', () => {
    setup({ groups: [salle('A')] })
    expect(within(group(1)).getByRole('button', { name: 'Tester le groupe' })).toBeDisabled()
  })

  it('désactivé tant que des modifications ne sont pas enregistrées, avec l\'infobulle « Enregistrez d\'abord »', () => {
    setup({ groups: [salle('A', ['BuzzHue1'])] })
    fireEvent.click(screen.getByRole('button', { name: '+ Nouveau groupe' }))
    const btn = within(group(1)).getByRole('button', { name: 'Tester le groupe' })
    expect(btn).toBeDisabled()
    expect(btn).toHaveAttribute('title', "Enregistrez d'abord")
  })

  it('désactivé quand l\'éclairage d\'ambiance est désactivé', () => {
    setup({ groups: [salle('A', ['BuzzHue1'])], enabled: false })
    expect(within(group(1)).getByRole('button', { name: 'Tester le groupe' })).toBeDisabled()
  })

  it('un essai refusé est signalé par un avertissement', async () => {
    const onTest = vi.fn().mockResolvedValue({ ok: false, message: 'Pont injoignable' })
    const { onNotify } = setup({ groups: [salle('A', ['BuzzHue1'])], onTest })
    fireEvent.click(within(group(1)).getByRole('button', { name: 'Tester le groupe' }))
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith({ message: 'Pont injoignable', type: 'warning' }))
  })
})

describe('LightGroupsManager — bloc « Groupe animateur » (§2.5)', () => {
  it('sans groupe animateur : un bouton le crée, un seul groupe animateur possible', () => {
    setup()
    expect(screen.queryByTestId('lgm-host')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: /Créer le groupe animateur/ }))
    expect(screen.getByTestId('lgm-host')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Créer le groupe animateur/ })).toBeNull()
  })

  it('le bloc n\'a ni numéro ni « équipes à proximité », et précise qu\'il reste hors des effets de salle', () => {
    setup({ groups: [host('Pupitre', ['BuzzHue1'])] })
    const h = screen.getByTestId('lgm-host')
    expect(within(h).queryByText(/Équipes à proximité/)).toBeNull()
    expect(within(h).getByText(/blanc fixe par défaut/)).toBeInTheDocument()
    expect(within(h).getByText(/Hors des effets de salle/)).toBeInTheDocument()
    expect(screen.getByText(/ni par ON \/ OFF, ni par le Flash/)).toBeInTheDocument()
  })

  it('le groupe animateur n\'entre pas dans l\'ordre de parcours ni dans la numérotation', () => {
    setup({ groups: [salle('A'), host('Pupitre'), salle('B')] })
    expect(screen.getByLabelText('Nom du groupe 2')).toHaveValue('B')
    expect(screen.queryByLabelText('Nom du groupe 3')).toBeNull()
    const order = screen.getByLabelText('Ordre de parcours')
    expect(order).toHaveTextContent(/1 A/)
    expect(order).toHaveTextContent(/2 B/)
    expect(order).not.toHaveTextContent(/Pupitre/)
  })

  it('on y range une ampoule générale par « Déplacer vers… »', () => {
    setup({ groups: [host('Pupitre')] })
    moveTo('BuzzHue2', 'host')
    expect(within(screen.getByTestId('lgm-host')).getByText('BuzzHue2')).toBeInTheDocument()
  })

  it('« Tester le groupe » du bloc appelle onTest("host")', async () => {
    const { onTest } = setup({ groups: [host('Pupitre', ['BuzzHue1'])] })
    fireEvent.click(within(screen.getByTestId('lgm-host')).getByRole('button', { name: 'Tester le groupe' }))
    await waitFor(() => expect(onTest).toHaveBeenCalledWith('host'))
  })

  it('« Supprimer » renvoie ses ampoules dans le bac', () => {
    setup({ groups: [host('Pupitre', ['BuzzHue1'])] })
    fireEvent.click(screen.getByRole('button', { name: 'Supprimer le groupe animateur' }))
    expect(screen.queryByTestId('lgm-host')).toBeNull()
    expect(within(pool()).getByText('BuzzHue1')).toBeInTheDocument()
  })

  it('« Choisir ses effets » est inactif (« bientôt ») tant que onChooseEffects est absent, actif ensuite', () => {
    const { unmount } = setup({ groups: [host('Pupitre')] })
    expect(screen.getByText(/Choisir ses effets \(bientôt\)/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Choisir ses effets' })).toBeNull()
    unmount()
    const onChooseEffects = vi.fn()
    setup({ groups: [host('Pupitre')], onChooseEffects })
    fireEvent.click(screen.getByRole('button', { name: 'Choisir ses effets' }))
    expect(onChooseEffects).toHaveBeenCalled()
  })

  it('le groupe animateur compte dans la limite de 12 groupes', () => {
    const many = [...Array.from({ length: 11 }, (_, i) => salle(`G${i + 1}`)), host('Pupitre')]
    setup({ groups: many })
    expect(screen.getByRole('button', { name: '+ Nouveau groupe' })).toBeDisabled()
  })
})

describe('LightGroupsManager — résumé sous la liste', () => {
  it('signale ampoules hors groupe et groupes vides, et le nombre de groupes animés', () => {
    setup({ groups: [salle('A', ['BuzzHue1']), salle('B', ['BuzzHue2']), salle('Vide')] })
    const warn = screen.getByRole('status')
    expect(warn).toHaveTextContent(/2 ampoules générales sont hors groupe/)
    expect(warn).toHaveTextContent(/1 groupe est vide/)
    expect(warn).toHaveTextContent(/animeront 2 groupes/)
  })

  it('aucun avertissement quand tout est rangé', () => {
    setup({ generalLights: ['BuzzHue1'], groups: [salle('A', ['BuzzHue1'])] })
    expect(screen.queryByRole('status')).toBeNull()
  })
})
