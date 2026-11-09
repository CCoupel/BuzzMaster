import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'

// ---------------------------------------------------------------------------
// #231 (v11.6.0, LOT-3) — composant mutualisé AmbianceSettingsPanel, MODE GLOBAL.
// Contrat : contracts/ambiance-settings.md §6 (props mode/value/onChange/cues/renderCatalogue/sections ;
// onglets Global / QCM / MEMORY / MEMOTION / RAFALE avec compteur) ; maquette
// docs/mockup/v11.6.0/ui/ambiance_settings_panel__global_per_type_tabs.html.
// Seul le mode global est livré en LOT-3 (le mode override est testé en LOT-4). Écrit depuis le contrat.
// ---------------------------------------------------------------------------

vi.mock('./AmbianceSettingsPanel.css', () => ({}))
import AmbianceSettingsPanel from './AmbianceSettingsPanel'

const C = (cue, type, label, sub = '') => ({ cue, type, label, sub })
const CUES = [
  C('depart', '', 'Départ', 'Le chronomètre démarre'),
  C('temps-ecoule', '', 'Temps écoulé'),
  C('gagne', '', 'Points gagnés'),
  C('perdu', '', 'Erreur'),
  C('reveal', '', 'Révélation'),
  C('entracte-debut', '', "Début d'entracte"),
  C('entracte-fin', '', "Fin d'entracte"),
  C('qcm-indice', 'QCM', 'Indice révélé', 'Une réponse est barrée'),
  C('memory-carte', 'MEMORY', 'Carte retournée'),
  C('memory-paire', 'MEMORY', 'Paire trouvée'),
  C('memotion-selection', 'MEMOTION', 'Carte choisie'),
  C('memotion-fin-carte', 'MEMOTION', 'Fin de carte'),
  C('rafale-bonne', 'RAFALE', 'Bonne réponse'),
  C('rafale-mauvaise', 'RAFALE', 'Mauvaise réponse'),
  C('rafale-fin-lot', 'RAFALE', 'Fin de lot'),
]
const VALUE = { sound: { muted: false, countdown: { enabled: true, loop: true }, cues: {} }, light: { mode: 'AUTO' } }

const setup = (props = {}) => {
  const onChange = vi.fn()
  const utils = render(<AmbianceSettingsPanel mode="global" value={VALUE} cues={CUES} onChange={onChange} {...props} />)
  return { onChange, ...utils }
}
const tab = (name) => screen.getByRole('tab', { name: new RegExp(`^${name}`) })

describe('AmbianceSettingsPanel (mode global) — onglets de bruitages (#231)', () => {
  it('cinq onglets Global / QCM / MEMORY / MEMOTION / RAFALE avec leur compteur (7 / 1 / 2 / 2 / 3)', () => {
    setup()
    expect(tab('Global')).toHaveTextContent('7')
    expect(tab('QCM')).toHaveTextContent('1')
    expect(tab('MEMORY')).toHaveTextContent('2')
    expect(tab('MEMOTION')).toHaveTextContent('2')
    expect(tab('RAFALE')).toHaveTextContent('3')
    expect(screen.getAllByRole('tab')).toHaveLength(5)
  })

  it("l'onglet Global est actif par défaut et n'affiche que les 7 cues historiques", () => {
    setup()
    expect(tab('Global')).toHaveAttribute('aria-selected', 'true')
    for (const label of ['Départ', 'Temps écoulé', 'Points gagnés', 'Erreur', 'Révélation', "Début d'entracte", "Fin d'entracte"]) {
      expect(screen.getByText(label)).toBeInTheDocument()
    }
    expect(screen.queryByText('Indice révélé')).not.toBeInTheDocument()
    expect(screen.queryByText('Carte retournée')).not.toBeInTheDocument()
  })

  it.each([
    ['QCM', ['Indice révélé'], ['Départ', 'Carte retournée']],
    ['MEMORY', ['Carte retournée', 'Paire trouvée'], ['Départ', 'Indice révélé']],
    ['MEMOTION', ['Carte choisie', 'Fin de carte'], ['Départ', 'Carte retournée']],
    ['RAFALE', ['Bonne réponse', 'Mauvaise réponse', 'Fin de lot'], ['Départ', 'Fin de carte']],
  ])('onglet %s : seulement ses événements propres', (name, shown, hidden) => {
    setup()
    fireEvent.click(tab(name))
    expect(tab(name)).toHaveAttribute('aria-selected', 'true')
    expect(tab('Global')).toHaveAttribute('aria-selected', 'false')
    shown.forEach((l) => expect(screen.getByText(l)).toBeInTheDocument())
    hidden.forEach((l) => expect(screen.queryByText(l)).not.toBeInTheDocument())
  })

  it('aucun point « surchargé » en mode global', () => {
    const { container } = setup()
    expect(container.querySelector('.asp-dot')).toBeNull()
    expect(screen.queryByLabelText(/surchargé/i)).toBeNull()
  })

  it('aucun état « Hérité » en mode global', () => {
    setup()
    expect(screen.queryByRole('radio', { name: /hérité/i })).toBeNull()
  })

  it('sans cue typée (serveur antérieur) : pas de barre d\'onglets, les 7 cues s\'affichent comme en v11.4.0', () => {
    setup({ cues: CUES.slice(0, 7) })
    expect(screen.queryByRole('tab')).toBeNull()
    expect(screen.getByText('Départ')).toBeInTheDocument()
  })
})

describe('AmbianceSettingsPanel (mode global) — édition (#231)', () => {
  it("l'interrupteur d'une nouvelle cue émet onChange('sound.cues.<cue>', bool)", () => {
    const { onChange } = setup()
    fireEvent.click(tab('QCM'))
    fireEvent.click(screen.getByRole('checkbox', { name: /Indice révélé/ }))
    expect(onChange).toHaveBeenCalledWith('sound.cues.qcm-indice', false)
  })

  it('une cue désactivée (value.sound.cues[cue] = false) est décochée', () => {
    setup({ value: { ...VALUE, sound: { ...VALUE.sound, cues: { 'memory-paire': false } } } })
    fireEvent.click(tab('MEMORY'))
    expect(screen.getByRole('checkbox', { name: /Paire trouvée/ })).not.toBeChecked()
    expect(screen.getByRole('checkbox', { name: /Carte retournée/ })).toBeChecked()
  })

  it("Son : « Coupé » émet onChange('sound.muted', true)", () => {
    const { onChange } = setup()
    fireEvent.click(screen.getByRole('radio', { name: 'Coupé' }))
    expect(onChange).toHaveBeenCalledWith('sound.muted', true)
  })

  it("Décompte : « Inactif » → ('sound.countdown.enabled', false) ; « Une fois » → ('sound.countdown.loop', false)", () => {
    const { onChange } = setup()
    fireEvent.click(screen.getByRole('radio', { name: 'Inactif' }))
    expect(onChange).toHaveBeenCalledWith('sound.countdown.enabled', false)
    fireEvent.click(screen.getByRole('radio', { name: 'Une fois' }))
    expect(onChange).toHaveBeenCalledWith('sound.countdown.loop', false)
  })

  it('l\'état courant est reflété : muted=true ⇒ « Coupé » sélectionné ; boucle ⇒ « En boucle » sélectionné', () => {
    setup({ value: { ...VALUE, sound: { ...VALUE.sound, muted: true } } })
    expect(screen.getByRole('radio', { name: 'Coupé' })).toHaveAttribute('aria-checked', 'true')
    expect(screen.getByRole('radio', { name: 'En boucle' })).toHaveAttribute('aria-checked', 'true')
  })
})

describe('AmbianceSettingsPanel (mode global) — slot catalogue et sections (#231)', () => {
  it('renderCatalogue(cue) est appelé pour chaque ligne visible et son contenu est rendu dans la ligne', () => {
    const renderCatalogue = vi.fn((cue) => <td data-testid={`cat-${cue}`}>fichier-{cue}</td>)
    setup({ renderCatalogue, catalogueColumns: [<th key="f">Fichier</th>] })
    expect(renderCatalogue).toHaveBeenCalledWith('depart')
    const row = screen.getByTestId('cat-depart').closest('tr')
    expect(within(row).getByText('Départ')).toBeInTheDocument()
    expect(screen.getByText('Fichier')).toBeInTheDocument()
    fireEvent.click(tab('RAFALE'))
    expect(renderCatalogue).toHaveBeenCalledWith('rafale-fin-lot')
    expect(screen.getByTestId('cat-rafale-bonne')).toBeInTheDocument()
  })

  it('sans renderCatalogue : le panneau s\'affiche quand même (aucune colonne catalogue)', () => {
    setup()
    expect(screen.getByText('Départ')).toBeInTheDocument()
  })

  it("sections=['sound','countdown','cues'] (LOT-3) : la section Lumière est masquée", () => {
    setup({ sections: ['sound', 'countdown', 'cues'] })
    expect(screen.queryByText(/Mode d'éclairage/i)).toBeNull()
    expect(screen.getByText('Départ')).toBeInTheDocument()
  })

  it("sections=['light'] : seule la section Lumière est rendue (mode ON / AUTO / OFF)", () => {
    const { onChange } = setup({ sections: ['light'] })
    expect(screen.getByText(/Mode d'éclairage/i)).toBeInTheDocument()
    expect(screen.queryByText('Départ')).toBeNull()
    expect(screen.queryByRole('radio', { name: 'Coupé' })).toBeNull()
    fireEvent.click(screen.getByRole('radio', { name: 'OFF' }))
    expect(onChange).toHaveBeenCalledWith('light.mode', 'OFF')
  })

  it('disabled fige tous les contrôles', () => {
    setup({ disabled: true })
    expect(screen.getByRole('radio', { name: 'Coupé' })).toBeDisabled()
    expect(screen.getByRole('checkbox', { name: /Départ/ })).toBeDisabled()
  })
})
